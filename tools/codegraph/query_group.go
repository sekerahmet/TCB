package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// pkgUse bir paketin bir düğümü kullanışının özetidir.
type pkgUse struct {
	dir       string
	funcs     map[string]bool // test dışı kullanıcı işlevler
	testFuncs map[string]bool
	uses      int // kenarların toplam sayacı (kullanım yeri sayısı)
}

// printWhoUsesByPackage kullanıcıları paketlere göre özetler: pakette kaç işlev, kaç kullanım yeri
// (test dosyalarındakiler ayrı sayılır). Ayrıntı için aynı sorgu --by-package olmadan.
func (ix *index) printWhoUsesByPackage(w io.Writer, n *Node, limit int) {
	uses := ix.whoUsesHeader(w, n)
	if len(uses) == 0 {
		fmt.Fprintln(w, "  no uses")
		ix.printOtherConfigNote(w, n)
		return
	}
	byDir := map[string]*pkgUse{}
	for _, e := range uses {
		src := ix.byID[e.From]
		dir := ix.dirOf(src)
		p := byDir[dir]
		if p == nil {
			p = &pkgUse{dir: dir, funcs: map[string]bool{}, testFuncs: map[string]bool{}}
			byDir[dir] = p
		}
		p.uses += max(e.Count, 1)
		if inTestFile(src) {
			p.testFuncs[src.ID] = true
		} else {
			p.funcs[src.ID] = true
		}
	}
	list := make([]*pkgUse, 0, len(byDir))
	for _, p := range byDir {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		if len(list[i].funcs) != len(list[j].funcs) {
			return len(list[i].funcs) > len(list[j].funcs)
		}
		return list[i].dir < list[j].dir
	})
	fmt.Fprintf(w, "  users by package (%d packages):\n", len(list))
	lines := make([]string, len(list))
	for i, p := range list {
		lines[i] = fmt.Sprintf("%s: %d functions, %d use sites", p.dir, len(p.funcs), p.uses)
		if len(p.testFuncs) > 0 {
			lines[i] += fmt.Sprintf(" (users in tests: %d)", len(p.testFuncs))
		}
		if len(p.funcs) == 0 {
			lines[i] = strings.Replace(lines[i], "0 functions, ", "only in tests, ", 1)
		}
	}
	printLimited(w, lines, limit, "    ")
}
