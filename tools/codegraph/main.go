// codegraph builds a local, deterministic knowledge graph of the repository and answers questions
// about it, so that finding "who calls this, which test guards that rule, where is this message
// used" does not need reading many files. No network, no model: the graph comes from go/types
// (calls, implements), the message and name dictionaries, the READMEs' "Denetim:" lines and the
// Markdown files.
//
// Usage (from the repository root):
//
//	go run ./tools/codegraph build                 // writes .codegraph/graph.json
//	go run ./tools/codegraph query messages.New    // a node and its neighbours
//	go run ./tools/codegraph path interp.VM H2100  // shortest path between two nodes
//	go run ./tools/codegraph who-uses H2100        // everything that uses a message/name code
//	go run ./tools/codegraph tests-for interp.VM.Run
//
// Exit codes: 0 ok, 1 failure, 2 usage, 3 ambiguous name (candidates are listed), 4 not found.
//
// Layer: tools (tools); not shipped to users.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Çıkış kodları.
const (
	exitOK        = 0
	exitFail      = 1
	exitUsage     = 2
	exitAmbiguous = 3
	exitNotFound  = 4
)

const usage = `usage: codegraph <command> [flags] [args]

  build                      build the graph (flags: --root DIR, --out FILE, --warnings)
  query <name>  (or explain)   a node and its neighbours; a name may be file:line[:col] (the declaration that contains it); query - reads grep -n output and writes each line's declaration
  path <from> <to> [--undirected|--through-hubs]   shortest directed path (calls, uses, dispatches ...); none is said plainly; --undirected also ignores direction but never goes through a type, package or file, --through-hubs allows those (marked)
  who-uses <name>            what uses a function, type, message code or name code
  tests-for <name>           tests that exercise a function, type or package
  history <name>             commits that touched a function, type, file, message code or name code (computed from git)
  fields <Type.field | Type> [--mode read|write]  where a struct field is read and written (typed; closures marked), or the table of a type's fields
  captures <func> [--type T]  the variables its func literals capture, with types
  callees <func> [--in PKG]  the functions it calls, by package
  calls-into <pkg> [--min N] [--in SRC] [--tests]  functions that call at least N distinct functions of a package
  deleted [--retired-json]   message and name codes that existed in the git history and are gone today, with the commit that removed them
  changed <branch|range> [--go-tests]   graph nodes touched by a branch (git diff main...branch), their tests, other claude/ branches on the same code; --go-tests prints only the Go tests to run, one "folder TestName" per line (the pre-check runs them)
  dups [--min N] [--in DIR] [--changed branch|range] [--json]   duplicate-code candidates in the Go sources (token-sequence clones of at least N syntax-tree nodes, default 100); --changed keeps those that touch what a branch changed; deliberate exceptions are listed by --list-exclusions
  similar <func> [--json]    the functions most like this one, with a similarity score
  dead-methods [--in DIR] [--json]   methods with no caller: unreferenced, kept alive only by an interface-satisfaction assertion (assertion-only), shadowed by an earlier interface check, or used only by tests
  package-state [--in DIR] [--json]  package variables written outside their declaration (process-wide mutable state): mutable, test-hook, address-taken, synchronized
  time-ceilings [--in DIR] [--json]  tests that compare an elapsed duration with a constant ceiling (file:line, constant, test name)
  spec-refs [--list] [--in DIR] [--json]   references to language-definition items (**N.M**, §N.M, the "dil tan…" form): a missing item is an error (exit 1), suffix vowel harmony after N.M is checked; --list prints each reference with the heading it points to

query flags: --graph FILE (default <root>/.codegraph/graph.json), --limit N (default 12), --all, --by-package, --min N, --in X, --tests, --base B, --root DIR (default: the repository root above the current directory), --no-rebuild
A query rebuilds the graph first when the sources changed since it was built (see README).
exit codes: 0 ok, 1 failure, 2 usage, 3 ambiguous (candidates listed), 4 not found
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "query", "explain", "path", "who-uses", "tests-for", "history", "changed", "callees", "calls-into", "fields", "captures":
		return runQuery(args[0], args[1:], stdout, stderr)
	case "spec-refs":
		return runSpecRefs(args[1:], stdout, stderr)
	case "time-ceilings":
		return runTimeCeilings(args[1:], stdout, stderr)
	case "package-state":
		return runPackageState(args[1:], stdout, stderr)
	case "dead-methods":
		return runDeadMethods(args[1:], stdout, stderr)
	case "deleted":
		return runDeleted(args[1:], stdout, stderr)
	case "dups":
		return runDups(args[1:], stdout, stderr)
	case "similar":
		return runSimilar(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	}
	fmt.Fprintf(stderr, "codegraph: unknown command %q\n%s", args[0], usage)
	return exitUsage
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repository root (default: found from the current directory, see README)")
	out := fs.String("out", "", "output file (default <root>/.codegraph/graph.json)")
	warnings := fs.Bool("warnings", false, "print every warning")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	abs, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	if *out == "" {
		*out = filepath.Join(abs, ".codegraph", "graph.json")
	}
	start := time.Now()
	g, err := buildGraph(abs)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	if err := writeGraph(*out, g); err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	printStats(stdout, g, *out, time.Since(start), *warnings)
	return exitOK
}

// buildGraph bütün çıkarıcıları sırayla çalıştırır. Sıra önemlidir: Markdown bağlantıları Go
// düğümlerine, kurallar test işlevlerine bağlanır.
func buildGraph(root string) (*Graph, error) {
	stamp, _ := sourceStamp(root) // build başlamadan alınır: sürerken değişen dosya sonra yeniden kurdurur
	m, err := load(root)
	if err != nil {
		return nil, err
	}
	g := buildGraphFrom(root, m)
	g.Stamp = stamp
	return g, nil
}

// buildGraphFrom yüklenmiş modülden grafiği kurar (damgasız): depo testleri modülü bir kez yükleyip grafiği ve
// go/types karşılaştırmalarını aynı modülden yapar.
func buildGraphFrom(root string, m *module) *Graph {
	b := newBuilder(m.Path)
	idx := extractGo(m, b)
	extractOtherConfigs(m, b)
	extractCodes(root, b)
	addTestEdges(b, idx)
	extractTR(root, b, m)
	extractMarkdown(root, b, idx)
	extractSpecRefs(root, b)
	extractDataFiles(root, b)
	extractJSLoads(root, b)
	reach := newTestReach(b, idx)
	linkTestPaths(b, idx, reach)
	linkCaseTests(b, idx, reach)
	linkScans(b, idx, reach)
	linkSnippets(b, idx, reach)
	return b.graph()
}

func printStats(w io.Writer, g *Graph, out string, took time.Duration, allWarnings bool) {
	nodes, edges := map[string]int{}, map[string]int{}
	conf := map[string]int{}
	for _, n := range g.Nodes {
		nodes[n.Kind]++
	}
	for _, e := range g.Edges {
		edges[e.Kind]++
		conf[e.Confidence]++
	}
	fmt.Fprintf(w, "codegraph: %d nodes, %d edges (%d EXTRACTED, %d INFERRED) in %.1fs -> %s\n",
		len(g.Nodes), len(g.Edges), conf[extracted], conf[inferred], took.Seconds(), out)
	fmt.Fprintf(w, "  nodes: %s\n  edges: %s\n", countList(nodes), countList(edges))
	if len(g.Warnings) == 0 {
		return
	}
	fmt.Fprintf(w, "  warnings: %d (also in graph.json)\n", len(g.Warnings))
	for i, s := range g.Warnings {
		if !allWarnings && i >= 5 {
			fmt.Fprintln(w, "    ... (--warnings lists all)")
			break
		}
		fmt.Fprintf(w, "    %s\n", s)
	}
}

func countList(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}
