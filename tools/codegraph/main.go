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
  query <name>  (or explain)   a node and its neighbours
  path <from> <to>           shortest path between two nodes
  who-uses <name>            what uses a function, type, message code or name code
  tests-for <name>           tests that exercise a function, type or package
  history <name>             commits that touched a function, type, file, message code or name code (computed from git)

query flags: --graph FILE (default .codegraph/graph.json), --limit N (default 12), --all, --root DIR (default .), --no-rebuild
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
	case "query", "explain", "path", "who-uses", "tests-for", "history":
		return runQuery(args[0], args[1:], stdout, stderr)
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
	root := fs.String("root", ".", "repository root (the directory with go.mod)")
	out := fs.String("out", "", "output file (default <root>/.codegraph/graph.json)")
	warnings := fs.Bool("warnings", false, "print every warning")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	abs, err := filepath.Abs(*root)
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
	b := newBuilder(m.path)
	idx := extractGo(m, b)
	extractCodes(root, b)
	addTestEdges(b, idx)
	extractMarkdown(root, b, idx)
	g := b.graph()
	g.Stamp = stamp
	return g, nil
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
