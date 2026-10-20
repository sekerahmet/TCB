package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// dirOf düğümün paket klasörüdür (modül köküne göre; kök için "."): paket düğümünde içe
// aktarma yolundan, öbürlerinde dosyasından.
func (ix *index) dirOf(n *Node) string {
	if n.Kind == kindPackage {
		if n.Detail == ix.g.Module {
			return "."
		}
		return strings.TrimPrefix(n.Detail, ix.g.Module+"/")
	}
	if n.File == "" {
		return ""
	}
	return path.Dir(n.File)
}

func isFuncNode(n *Node) bool { return n.Kind == kindFunc || n.Kind == kindMethod }

func inTestFile(n *Node) bool { return strings.HasSuffix(n.File, "_test.go") }

// packageDirs --in değerini klasörlere çevirir: ad bir paketse onun klasörü, değilse değer
// kendisi klasör önekidir. Boş değer: kısıt yok.
func (ix *index) packageDirs(in string) []string {
	if in == "" {
		return nil
	}
	var dirs []string
	for _, n := range ix.lookup(in) {
		if n.Kind == kindPackage {
			dirs = append(dirs, ix.dirOf(n))
		}
	}
	if len(dirs) == 0 {
		dirs = []string{strings.TrimSuffix(in, "/")}
	}
	return dirs
}

func dirMatches(dir string, dirs []string) bool {
	if dirs == nil {
		return true
	}
	for _, d := range dirs {
		if dir == d || strings.HasPrefix(dir, d+"/") {
			return true
		}
	}
	return false
}

// printCallees bir işlevin çağırdığı işlevleri paketlere göre gruplayıp yazar (--in: yalnızca o paket).
func (ix *index) printCallees(n *Node, o *queryOpts) int {
	if !isFuncNode(n) {
		fmt.Fprintf(o.stderr, "callees needs a function or method, %s is a %s\n", n.Name, n.Kind)
		return exitUsage
	}
	dirs := ix.packageDirs(o.in)
	byDir := map[string][]*Edge{}
	for _, e := range ix.out[n.ID] {
		callee := ix.byID[e.To]
		if e.Kind != edgeCalls || !isFuncNode(callee) {
			continue
		}
		if d := ix.dirOf(callee); dirMatches(d, dirs) {
			byDir[d] = append(byDir[d], e)
		}
	}
	fmt.Fprintln(o.stdout, describe(n))
	if len(byDir) == 0 {
		fmt.Fprintln(o.stdout, "  calls no function"+inClause(o.in))
		return exitOK
	}
	var names []string
	total := 0
	for d, list := range byDir {
		names = append(names, d)
		total += len(list)
	}
	sort.Strings(names)
	fmt.Fprintf(o.stdout, "  calls %d distinct functions in %d packages%s:\n", total, len(names), inClause(o.in))
	for _, d := range names {
		list := byDir[d]
		sort.Slice(list, func(i, j int) bool { return ix.byID[list[i].To].Name < ix.byID[list[j].To].Name })
		fmt.Fprintf(o.stdout, "    %s (%d):\n", d, len(list))
		lines := make([]string, len(list))
		for i, e := range list {
			lines[i] = ix.edgeLine(e, e.To, true)
		}
		printLimited(o.stdout, lines, o.limit, "      ")
	}
	return exitOK
}

func inClause(in string) string {
	if in == "" {
		return ""
	}
	return " (in " + in + ")"
}

// caller bir çağıranın pakete yaptığı ayrı çağrılardır.
type caller struct {
	node    *Node
	callees []*Node
}

// printCallsInto pkg'deki işlevlerden en az --min tanesini çağıran işlevleri yazar ("hangi
// işlev hangi tensör işlevlerini çağırıyor"). Yalnızca EXTRACTED çağrılar sayılır; paketin
// kendi içindeki çağıranlar ve (--tests yoksa) test dosyaları dışarıdadır; --in çağıranları
// bir pakete ya da klasör önekine daraltır.
func (ix *index) printCallsInto(pkg *Node, o *queryOpts) int {
	target := ix.dirOf(pkg)
	srcDirs := ix.packageDirs(o.in)
	byCaller := map[string]*caller{}
	for i := range ix.g.Edges {
		e := &ix.g.Edges[i]
		if e.Kind != edgeCalls || e.Confidence != extracted {
			continue
		}
		callee, src := ix.byID[e.To], ix.byID[e.From]
		if !isFuncNode(callee) || !isFuncNode(src) || ix.dirOf(callee) != target {
			continue
		}
		if d := ix.dirOf(src); d == target || !dirMatches(d, srcDirs) || (!o.tests && inTestFile(src)) {
			continue
		}
		c := byCaller[src.ID]
		if c == nil {
			c = &caller{node: src}
			byCaller[src.ID] = c
		}
		c.callees = append(c.callees, callee)
	}
	var found []*caller
	for _, c := range byCaller {
		if len(c.callees) >= o.min {
			found = append(found, c)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if len(found[i].callees) != len(found[j].callees) {
			return len(found[i].callees) > len(found[j].callees)
		}
		return found[i].node.Name < found[j].node.Name
	})
	fmt.Fprintf(o.stdout, "functions calling at least %d distinct functions of %s%s: %d\n", o.min, target, inClause(o.in), len(found))
	printCallers(o.stdout, found, o.limit)
	return exitOK
}

func printCallers(w io.Writer, found []*caller, limit int) {
	lines := make([]string, len(found))
	for i, c := range found {
		names := make([]string, len(c.callees))
		for j, n := range c.callees {
			names[j] = n.Name
		}
		sort.Strings(names)
		lines[i] = fmt.Sprintf("%s  (%d): %s", describe(c.node), len(names), strings.Join(names, ", "))
	}
	printLimited(w, lines, limit, "  ")
}
