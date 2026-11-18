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
// Exit codes: 0 ok, 1 failure, 2 usage, 3 ambiguous name (candidates are listed), 4 not found, 5 the repo_scans lists of
// tools/full_check.sh disagree with the graph (changed --go-tests; repo_scans.go).
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
	exitScanList  = 5 // changed --go-tests: tam denetimin depo taramaları listeleri graftan ayrışıyor (repo_scans.go)
)

const usage = `usage: codegraph <command> [flags] [args]

  build                      build the graph (flags: --root DIR, --out FILE, --warnings)
  query <name>               a node and its neighbours; a name may be file:line[:col] (the declaration that contains it); query - reads grep -n output and writes each line's declaration
  path <from> <to> [--undirected|--through-hubs]   shortest directed path (calls, uses, dispatches ...); none is said plainly; --undirected also ignores direction but never goes through a type, package or file, --through-hubs allows those (marked)
  who-uses <name> [--mode read|write]   what uses a function, type, message code or name code; for a struct field its reads and writes (--mode: only those, and the name resolves only to a field)
  tests-for <name> [--list|--all]   tests that exercise a function, type, file or package: the run: and size: lines and how many tests are listed; --list (or --limit N) the first page of each group, --all every test
  untested [--in DIR] [--json]   non-test Go functions and methods grouped by the seven branches of tests-for
  fields <Type>              the table of a type's fields with read and write counts (one field: who-uses <Type.field> --mode read|write)
  callees <func> [--in PKG]  the functions it calls, by package
  calls-into <pkg> [--min N] [--in SRC] [--tests]  functions that call at least N distinct functions of a package
  deleted [--retired-json]   message and name codes that existed in the git history and are gone today, with the commit that removed them
  changed <branch|range> [--go-tests]   graph nodes touched by a branch (git diff main...branch; when the tip is the checked-out HEAD, as in "changed HEAD", the diff runs to the working tree: uncommitted and staged changes included), their tests, other claude/ branches on the same code; --go-tests prints only the Go tests to run, one "folder TestName" per line (the pre-check runs them)
  dups [--min N] [--in DIR] [--changed branch|range] [--json]   duplicate-code candidates in the Go sources (token-sequence clones of at least N syntax-tree nodes, default 100); --changed keeps those that touch what a branch changed; deliberate exceptions are listed by --list-exclusions
  similar <func> [--json]    the functions most like this one, with a similarity score
  dead-methods [--in DIR] [--json]   methods with no caller: unreferenced, kept alive only by an interface-satisfaction assertion (assertion-only), shadowed by an earlier interface check, or used only by tests
  spec-refs [--list] [--in DIR] [--json]   references to language-definition items (**N.M**, §N.M, the "dil tan…" form): a missing item is an error (exit 1), suffix vowel harmony after N.M is checked; --list prints each reference with the heading it points to

query flags: --graph FILE (default <root>/.codegraph/graph.json), --limit N (default 12), --all, --list, --by-package, --min N, --in X, --tests, --base B, --by-distance, --root DIR (default: the repository root above the current directory), --no-rebuild
A query rebuilds the graph first when the sources changed since it was built (see README).
exit codes: 0 ok, 1 failure, 2 usage, 3 ambiguous (candidates listed), 4 not found, 5 the repo_scans lists of tools/full_check.sh disagree with the graph (changed --go-tests)
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
	case "query", "path", "who-uses", "tests-for", "untested", "changed", "callees", "calls-into", "fields":
		return runQuery(args[0], args[1:], stdout, stderr)
	case "explain", "history", "package-state", "time-ceilings", "captures":
		return runRetired(args[0], args[1:], stderr)
	case "spec-refs":
		return runSpecRefs(args[1:], stdout, stderr)
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
// düğümlerine, kurallar test işlevlerine bağlanır. Çıkarıcılardan biri paniklerse (ör. yarım
// düzenlenmiş gömülü ileti dosyasında messages.load) panik tek satırlık hataya çevrilir: refresh eski
// grafiği korur, build ve boş grafik tek satırla çıkar.
func buildGraph(root string) (g *Graph, err error) {
	defer func() {
		if r := recover(); r != nil {
			g, err = nil, fmt.Errorf("%s", firstLine(fmt.Errorf("%v", r)))
		}
	}()
	stamp, _ := sourceStamp(root) // build başlamadan alınır: sürerken değişen dosya sonra yeniden kurdurur
	m, err := load(root)
	if err != nil {
		return nil, err
	}
	g = buildGraphFrom(root, m)
	g.Stamp = stamp
	return g, nil
}

// buildGraphFrom yüklenmiş modülden grafiği kurar (damgasız): depo testleri modülü bir kez yükleyip grafiği ve
// go/types karşılaştırmalarını aynı modülden yapar.
func buildGraphFrom(root string, m *module) *Graph {
	return buildGraphFromWithRules(root, m, repoTypeQueryRules)
}

// buildGraphFromWithRules yüklenmiş modülden verilen tip sorgusu kapsam listesiyle grafiği kurar; fikstür testleri
// kendi kapsam kurallarını parametreyle geçirir.
func buildGraphFromWithRules(root string, m *module, typeRules []typeQueryRule) *Graph {
	b := newBuilder(m.Path)
	idx := extractGo(m, b)
	extractOtherConfigs(m, b)
	extractCodes(root, b)
	addTestEdges(b, idx)
	extractTR(root, b, m)
	extractMarkdown(root, b, idx)
	extractSpecRefs(root, b)
	extractDataFiles(root, b)
	linkTRPaths(root, b)
	extractScriptEnv(root, b)
	extractJSLoads(root, b)
	reach := newTestReach(b, idx)
	linkTestPaths(b, idx, reach)
	linkCaseTests(b, idx, reach)
	linkScans(b, idx, reach)
	linkSnippets(b, idx, reach, os.DirFS(filepath.Join(root, "lib")))
	linkCheckerTests(b, idx, reach)
	linkTypeQueries(b, typeRules)
	linkExam(root, b)
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

// historyGone emekliye ayrılan `history`'nin yerini söyler (K6): adın dosyası query ile, geçmişi git'in bildirim
// satırına sabitlenmiş biçimleriyle. Sabitlenmemiş `-L :<ad>:<dosya>` adı içeren ilk satıra gider
// (`-L :Mask:internal/lang/interp/output.go` metodun değil SecretMask sabitinin geçmişini veriyordu).
const historyGone = `codegraph: "history" is gone; git answers it. The file of a name: go run ./tools/codegraph query <name>. Then: git log -L ':^func <name>(:<file>' -s --oneline for a function, git log -L ':^func (.*) <name>(:<file>' -s --oneline for a method, git log -L ':^type <name> :<file>' -s --oneline for a type, git log --follow --oneline -- <file> for a file, git log -S'<code>' --oneline -- internal/lang/messages/definitions.json internal/lang/codes/definitions.json for a code (renames are not followed)`

// retiredGone emekliye ayrılan komutların yerini söyleyen iletilerdir. package-state, time-ceilings ve captures
// codegraph 7 Dilim C'de (K3) gitti: package-state'in yazanlarını grafın writes_var kenarı aynı kuralla veriyor
// (writeTargets), time-ceilings'in 37 yerinin 36'sını tek git grep buluyor, captures'ın yazılma nedeni olan istek
// .tr içindi ve komut onu hiç cevaplamadı (tasarım codegraph-7c-tasarim.md 2cf336d6).
var retiredGone = map[string]string{
	"history":       historyGone,
	"package-state": `codegraph: "package-state" is gone; query answers it: go run ./tools/codegraph query <variable> lists who writes it (<- writes_var) and reads it, query <function> the package variables it writes and reads (-> writes_var, -> reads_var), query <file> --all the variables a file declares ([var])`,
	"time-ceilings": `codegraph: "time-ceilings" is gone; git answers it: git grep -n -E '(>|<|>=|<=) *([0-9_]+ *\* *)?time\.(Second|Millisecond|Minute)' -- '*_test.go' (a ceiling kept in a named constant is found by that constant's name). The rule for tests that measure time is in internal/README.md.`,
	"captures":      `codegraph: "captures" is gone: read the function; go run ./tools/codegraph query <function> gives its file and line`,
}

// runRetired emekliye ayrılan komutların yerine geçen yolu yazar; çıkış 2 (K6). explain query'nin öbür adıydı: aynı
// argümanlarla query satırı verilir; öbürleri retiredGone'daki iletiyi yazar.
func runRetired(cmd string, args []string, stderr io.Writer) int {
	if msg, ok := retiredGone[cmd]; ok {
		fmt.Fprintln(stderr, msg)
		return exitUsage
	}
	rest := strings.Join(args, " ")
	if rest == "" {
		rest = "<name>"
	}
	fmt.Fprintf(stderr, "codegraph: %q was another name for query and is gone: go run ./tools/codegraph query %s\n", cmd, rest)
	return exitUsage
}
