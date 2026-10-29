package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Bu dosya tests-for <Go dosyası>'nın koşu satırlarını kurar: düzenleme döngüsünde koşulacak kısa komut. Koşulan liste
// (ön denetimin seçimi) eksiltilmez; komutun koşmadığı her test sayısıyla ve onu koşan komutla söylenir (DENETİM
// 2026-10-29: 496 testlik listede düzenlenen paketin testi seçilemedi, boyut testi commit'ten sonra düştü).

// commandLineLimit tools/full_check.sh'in -run listesi için kullandığı sınırdır: Windows'ta komut satırı ~32 000
// karakterdir, ön denetim 20 000'i aşan listede paketleri bütün olarak koşar; koşu satırı da öyle yapar.
const commandLineLimit = 20000

// runPlan dosyanın koşu satırlarıdır. İki komut gerekir: -run bir `go test`'in bütün paketlerine uygulanır, tek komut
// dosyanın kendi paketini de süzerdi.
type runPlan struct {
	pkg    string   // dosyanın paketi, bütün olarak (-run'sız): grafın göremediği aynı paket testleri de koşar
	others []string // dosyayı tarayan depo kurallarının ve konu edinen testlerin paketleri (pkg hariç)
	names  []string // onların adları (-run)
	whole  bool     // -run listesi commandLineLimit'i aşıyor: others bütün olarak koşar
	rest   []goTest // ön denetimin seçiminden iki komutun koşmadıkları
}

// runPlanFor koşu satırlarını ön denetimin seçiminden (goTestSelection, `changed --go-tests`) kurar: dosyanın paketi
// bütün olarak; dosyayı tarayan bütün depo kuralları (scansCovering); dosyanın bir bildirimini konu edinen Go testleri
// (about). Seçimin geri kalanı rest'tir.
func (ix *index) runPlanFor(file *Node, ts *touchSet, about []aboutTest) runPlan {
	p := runPlan{pkg: path.Dir(file.File)}
	extra := map[goTest]bool{}
	add := func(n *Node) {
		if t, ok := goTestOf(n); ok && t.dir != p.pkg {
			extra[t] = true
		}
	}
	for _, h := range ix.scansCovering(file.File) {
		for _, t := range h.tests {
			add(t)
		}
	}
	for _, a := range about {
		add(a.test)
	}
	pkgs, names := map[string]bool{}, map[string]bool{}
	for t := range extra {
		pkgs[t.dir], names[t.name] = true, true
	}
	p.others, p.names = sortedKeys(pkgs), sortedKeys(names)
	p.whole = len(p.command(p.others, p.names)) > commandLineLimit
	sel := newTouchSet()
	sel.nodes, sel.files = ts.nodes, map[string]bool{file.File: true}
	for _, t := range ix.goTestSelection(sel) {
		if t.dir != p.pkg && !(pkgs[t.dir] && (p.whole || names[t.name])) {
			p.rest = append(p.rest, t)
		}
	}
	return p
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// command bir `go test` komutudur; names boşsa paketler bütün olarak koşar.
func (p runPlan) command(pkgs, names []string) string {
	args := make([]string, len(pkgs))
	for i, d := range pkgs {
		args[i] = "./" + d + "/"
	}
	cmd := "go test -short " + strings.Join(args, " ")
	if len(names) > 0 {
		cmd += " -run '^(" + strings.Join(names, "|") + ")$'"
	}
	return cmd
}

// commands koşu satırlarının komutlarıdır.
func (p runPlan) commands() []string {
	out := []string{p.command([]string{p.pkg}, nil)}
	switch {
	case len(p.others) == 0:
	case p.whole:
		out = append(out, p.command(p.others, nil))
	default:
		out = append(out, p.command(p.others, p.names))
	}
	return out
}

// printRunPlan koşu satırlarını girintisiz `run:` başlığıyla, dosya satırının hemen altına yazar (öğe satırı değildir:
// planlama sınavının first-item sorusu en az 4 boşlukla girintili ilk satırı okur); altında koşulmayanların sayısı.
func printRunPlan(w io.Writer, p runPlan) {
	for _, c := range p.commands() {
		fmt.Fprintln(w, "run: "+c)
	}
	if p.whole {
		fmt.Fprintf(w, "  the second command runs its packages whole: its -run list is longer than %d characters (the Windows command line)\n", commandLineLimit)
	}
	if len(p.rest) == 0 {
		fmt.Fprintln(w, "  these commands run every Go test the pre-check selects for this file")
		return
	}
	pkgs := map[string]bool{}
	for _, t := range p.rest {
		pkgs[t.dir] = true
	}
	// Koşulmayanlar ayrı listelenmez: aşağıdaki gruplardadırlar (DENETİM: "listed below" ayrı bir liste vaat ediyordu).
	fmt.Fprintf(w, "  not run by these commands: %s in %s (among the groups below); before committing, `bash tools/full_check.sh --kip=on` (in the background) runs them all\n",
		countOf(len(p.rest), "more Go test"), countOf(len(pkgs), "package"))
}

// countOf "1 package", "2 packages": sayıya göre tekil ya da çoğul İngilizce ad.
func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
