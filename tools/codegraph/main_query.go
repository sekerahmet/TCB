package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runQuery query, path, who-uses ve tests-for komutlarını çalıştırır.
func runQuery(cmd string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	graphPath := fs.String("graph", "", "graph file (default <root>/.codegraph/graph.json)")
	limit := fs.Int("limit", 12, "lines per group")
	all := fs.Bool("all", false, "no line limit")
	root := fs.String("root", "", "repository root (default: found from the current directory, see README)")
	base := fs.String("base", "main", "changed: the branch a branch name is compared with (git diff base...branch)")
	noRebuild := fs.Bool("no-rebuild", false, "use the graph as it is, even when the sources changed")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return exitUsage
	}
	want := 1
	if cmd == "path" {
		want = 2
	}
	if fs.NArg() != want {
		fmt.Fprintf(stderr, "codegraph %s: expected %d name(s)\n%s", cmd, want, usage)
		return exitUsage
	}
	abs, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	*root = abs // load ve git yolları mutlak ister
	if *graphPath == "" {
		*graphPath = filepath.Join(abs, ".codegraph", "graph.json")
	}
	if *all {
		*limit = 0
	}
	g, err := readGraph(*graphPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stderr, "codegraph: %s not found; run `go run ./tools/codegraph build` first\n", *graphPath)
		} else {
			fmt.Fprintln(stderr, "codegraph:", err)
		}
		return exitFail
	}
	if !*noRebuild {
		g = refresh(g, *root, *graphPath, stderr)
	} else if g.Extractor != extractorRevision {
		fmt.Fprintf(stderr, "codegraph: warning: the graph was built by an older codegraph (extractor %d, now %d); results may be incomplete; run build\n", g.Extractor, extractorRevision)
	}
	ix := newIndex(g)
	nodes := make([]*Node, 0, want)
	names := fs.Args()
	if cmd == "changed" {
		names = nil // changed takes a git revision, not a node name
	}
	for _, q := range names {
		n, code := ix.resolve(q, stdout, stderr, *limit)
		if code != exitOK {
			return code
		}
		nodes = append(nodes, n)
	}
	if cmd == "changed" {
		return ix.runChanged(changeOpts{root: *root, base: *base, limit: *limit}, fs.Arg(0), stdout, stderr)
	}
	if cmd == "history" {
		return ix.runHistory(*root, nodes[0], *limit, stdout, stderr)
	}
	return ix.answer(cmd, nodes, stdout, stderr, *limit)
}

func (ix *index) answer(cmd string, nodes []*Node, stdout, stderr io.Writer, limit int) int {
	switch cmd {
	case "query", "explain":
		ix.printQuery(stdout, nodes[0], limit)
	case "who-uses":
		ix.printWhoUses(stdout, nodes[0], limit)
	case "tests-for":
		ix.printTestsFor(stdout, nodes[0], limit)
	case "path":
		if !ix.printPath(stdout, nodes[0], nodes[1]) {
			fmt.Fprintf(stderr, "no path between %s and %s\n", nodes[0].Name, nodes[1].Name)
			return exitNotFound
		}
	}
	return exitOK
}

// resolve bir adı tek düğüme çözer; belirsizse adayları stdout'a yazar (3), yoksa stderr'e
// benzer adlarla birlikte bildirir (4).
func (ix *index) resolve(q string, stdout, stderr io.Writer, limit int) (*Node, int) {
	found := ix.lookup(q)
	switch len(found) {
	case 1:
		return found[0], exitOK
	case 0:
		fmt.Fprintf(stderr, "%q not found\n", q)
		if sug := ix.suggest(q, 8); len(sug) > 0 {
			fmt.Fprintln(stderr, "similar:")
			for _, n := range sug {
				fmt.Fprintf(stderr, "  %s\n", describe(n))
			}
		}
		return nil, exitNotFound
	}
	ix.printCandidates(stdout, q, found, limit)
	return nil, exitAmbiguous
}

// reorderFlags bayrakları adların önüne alır (flag paketi ilk addan sonrasını bayrak saymaz):
// "query New --limit 5" ile "query --limit 5 New" aynıdır.
func reorderFlags(args []string) []string {
	valued := map[string]bool{"graph": true, "limit": true, "root": true, "base": true}
	var flags, names []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			names = append(names, a)
			continue
		}
		flags = append(flags, a)
		if name := strings.TrimLeft(a, "-"); valued[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, names...)
}
