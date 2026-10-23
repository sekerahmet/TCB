package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// queryOpts bir sorgu komutunun ayarları ve çıktı akışlarıdır.
type queryOpts struct {
	stdout, stderr io.Writer
	limit          int    // gruptaki en çok satır (0: sınırsız)
	root, base     string // depo kökü; changed için karşılaştırılan dal
	in             string // callees / calls-into: paket sınırı
	min            int    // calls-into: en az kaç ayrı işlev
	tests          bool   // calls-into: test dosyalarındaki çağıranlar da sayılır
	byPackage      bool   // who-uses: kullanıcıları pakete göre özetle
	mode           string // who-uses, fields: yalnızca "read" ya da "write" erişimleri
	typeFilter     string // captures: yakalanan değişkenin tipinde geçen metin
	undirected     bool   // path: yön yok sayılır (tür, paket ve dosyalardan geçmeden)
	throughHubs    bool   // path: yön yok sayılır, çok bağlı düğümlerden de geçer
}

// runQuery grafiği okuyan komutları çalıştırır: query, explain, path, who-uses, tests-for,
// history, changed, callees, calls-into.
func runQuery(cmd string, args []string, stdout, stderr io.Writer) int {
	return runQueryOn(nil, cmd, args, stdout, stderr)
}

// runQueryOn runQuery'dir; loaded nil değilse grafik dosyadan okunmaz, o kullanılır (aynı süreçte çok soru soran planlama
// sınavı grafiği bir kez yükler).
func runQueryOn(loaded *Graph, cmd string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	graphPath := fs.String("graph", "", "graph file (default <root>/.codegraph/graph.json)")
	all := fs.Bool("all", false, "no line limit")
	noRebuild := fs.Bool("no-rebuild", false, "use the graph as it is, even when the sources changed")
	o := &queryOpts{stdout: stdout, stderr: stderr}
	fs.IntVar(&o.limit, "limit", 12, "lines per group")
	fs.StringVar(&o.root, "root", "", "repository root (default: found from the current directory, see README)")
	fs.StringVar(&o.base, "base", "main", "changed: the branch a branch name is compared with (git diff base...branch)")
	fs.StringVar(&o.in, "in", "", "callees, calls-into: restrict to a package")
	fs.IntVar(&o.min, "min", 2, "calls-into: at least this many distinct functions")
	fs.BoolVar(&o.tests, "tests", false, "calls-into: count callers in test files too")
	fs.StringVar(&o.mode, "mode", "", "fields: only read or write accesses")
	fs.StringVar(&o.typeFilter, "type", "", "captures: only variables whose type contains this text")
	fs.BoolVar(&o.undirected, "undirected", false, "path: also look for a connection ignoring edge direction (never through a type, package or file)")
	fs.BoolVar(&o.throughHubs, "through-hubs", false, "path: ignore direction and allow types, packages and files on the way (marked [hub])")
	fs.BoolVar(&o.byPackage, "by-package", false, "who-uses: summarise the users per package")
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
	abs, err := resolveRoot(o.root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	o.root = abs // load ve git yolları mutlak ister
	if *graphPath == "" {
		*graphPath = filepath.Join(abs, ".codegraph", "graph.json")
	}
	if *all {
		o.limit = 0
	}
	g := loaded
	if g == nil {
		var code int
		if g, code = loadGraph(*graphPath, o, *noRebuild); g == nil {
			return code
		}
	}
	ix := newIndex(g)
	ix.root = o.root
	return ix.dispatch(cmd, fs.Args(), o)
}

// loadGraph grafiği okur ve gerekirse tazeler; okunamazsa nil ve çıkış kodunu verir.
func loadGraph(path string, o *queryOpts, noRebuild bool) (*Graph, int) {
	g, err := readGraph(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(o.stderr, "codegraph: %s not found; run `go run ./tools/codegraph build` first\n", path)
		} else {
			fmt.Fprintln(o.stderr, "codegraph:", err)
		}
		return nil, exitFail
	}
	switch {
	case !noRebuild:
		g = refresh(g, o.root, path, o.stderr)
	case g.Extractor != extractorRevision:
		fmt.Fprintf(o.stderr, "codegraph: warning: the graph was built by an older codegraph (extractor %d, now %d); results may be incomplete; run build\n", g.Extractor, extractorRevision)
	}
	return g, exitOK
}

// dispatch adları düğümlere çözer ve komutu çalıştırır.
func (ix *index) dispatch(cmd string, names []string, o *queryOpts) int {
	if cmd == "changed" { // adı değil, git revizyonu alır
		return ix.runChanged(changeOpts{root: o.root, base: o.base, limit: o.limit}, names[0], o.stdout, o.stderr)
	}
	kinds := map[string][]string{
		"calls-into": {kindPackage}, "captures": {kindFunc, kindMethod, kindClosure}, "fields": {kindField, kindType},
	}[cmd]
	nodes := make([]*Node, 0, len(names))
	for _, q := range names {
		n, code := ix.resolve(q, o, kinds...)
		if code != exitOK {
			return code
		}
		nodes = append(nodes, n)
	}
	return ix.answer(cmd, nodes, o)
}

func (ix *index) answer(cmd string, nodes []*Node, o *queryOpts) int {
	switch cmd {
	case "query", "explain":
		ix.printQuery(o.stdout, nodes[0], o.limit)
	case "who-uses":
		if nodes[0].Kind == kindField {
			ix.printFieldUses(nodes[0], o)
		} else if o.byPackage {
			ix.printWhoUsesByPackage(o.stdout, nodes[0], o.limit)
		} else {
			ix.printWhoUses(o.stdout, nodes[0], o.limit)
		}
	case "tests-for":
		ix.withTestNotes(o.root)
		ix.printTestsFor(o.stdout, nodes[0], o.limit)
	case "history":
		return ix.runHistory(o.root, nodes[0], o.limit, o.stdout, o.stderr)
	case "fields":
		if nodes[0].Kind == kindField {
			ix.printFieldUses(nodes[0], o)
		} else {
			ix.printFieldTable(nodes[0], o)
		}
	case "captures":
		return ix.printCaptures(nodes[0], o)
	case "callees":
		return ix.printCallees(nodes[0], o)
	case "calls-into":
		return ix.printCallsInto(nodes[0], o)
	case "path":
		mode := pathDirected
		switch {
		case o.throughHubs:
			mode = pathThroughHubs
		case o.undirected:
			mode = pathUndirected
		}
		if !ix.printPath(o.stdout, nodes[0], nodes[1], mode) {
			fmt.Fprintln(o.stderr, noPathMessage(nodes[0], nodes[1], mode))
			return exitNotFound
		}
	}
	return exitOK
}

// resolve bir adı tek düğüme çözer; belirsizse adayları stdout'a yazar (3), yoksa stderr'e
// yakın adlarla birlikte bildirir (4). kinds doluysa yalnızca o türdeki düğümler sayılır.
func (ix *index) resolve(q string, o *queryOpts, kinds ...string) (*Node, int) {
	found := ix.lookup(q, kinds...)
	switch len(found) {
	case 1:
		return found[0], exitOK
	case 0:
		fmt.Fprintf(o.stderr, "%q not found\n", q)
		ix.printSuggestions(o.stderr, q, kinds, o.limit == 0)
		return nil, exitNotFound
	}
	ix.printCandidates(o.stdout, q, found, o.limit)
	return nil, exitAmbiguous
}

// reorderFlags bayrakları adların önüne alır (flag paketi ilk addan sonrasını bayrak saymaz):
// "query New --limit 5" ile "query --limit 5 New" aynıdır.
func reorderFlags(args []string) []string {
	valued := map[string]bool{"graph": true, "limit": true, "root": true, "base": true, "in": true, "min": true, "mode": true, "type": true, "changed": true}
	var flags, names []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			names = append(names, a)
			continue
		}
		flags = append(flags, a)
		if name := strings.TrimLeft(a, "-"); valued[name] && !strings.Contains(a, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, names...)
}
