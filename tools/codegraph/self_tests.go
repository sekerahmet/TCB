package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// Bu dosya codegraph'ın kendi testlerinin öbür dosyaların seçiminden çıkma kuralıdır (codegraph dilim 6, D6-3, Karar 3).
// codegraph paketinin testleri ağır pakette koşar (tam koşusu G4'te 151 sn); çoğu codegraph'ı sınar (kendini gerçek
// depoyla ya da fikstürle), düşünce düzeltme codegraph'tadır. Depo kuralları (repoRule ile başlayan testler) ise
// codegraph'ın dışındaki dosyaları denetler. İki klasörün dışındaki bir dosyanın `tests-for` koşu satırına ve commit
// kipinin seçimine (`changed --go-tests --without-codegraph-self-tests`) yalnız kurallar girer; öbürleri sayılır (S3).
// Ön denetimin seçimi ve G4 aynı kalır: hiçbir test atlanmaz.

// selfTestFolders kendi testleri seçimde kalan klasörlerdir: codegraph'ın kendisi ve aynı çözümleyiciyi paylaşan
// codeindex.
var selfTestFolders = []string{"tools/codegraph/", "internal/devtools/codeindex/"}

// codegraphPkg codegraph'ın paket klasörüdür.
const codegraphPkg = "tools/codegraph"

// inSelfTestFolders dosya iki klasörden birinde mi.
func inSelfTestFolders(file string) bool {
	return slices.ContainsFunc(selfTestFolders, func(f string) bool { return strings.HasPrefix(file, f) })
}

// isCodegraphSelfTest t codegraph paketinin repoRule çağırmayan bir testi mi (S3'ün saydığı).
func (ix *index) isCodegraphSelfTest(t goTest) bool {
	if t.dir != codegraphPkg {
		return false
	}
	pkg := ix.g.Module + "/" + codegraphPkg
	rule := memberID(pkg, "", "repoRule")
	for _, e := range ix.out[memberID(pkg, "", t.name)] {
		if e.Kind == edgeCalls && e.To == rule {
			return false
		}
	}
	return true
}

// selfTestsLine S3'tür: koşu satırlarının ve commit kipinin dışında kalan codegraph testlerinin sayısı.
const selfTestsLine = "  not run by these commands: %d codegraph tests (the full check on Colab runs them)\n"

// printSelfTestsLine koşu planının dışarıda tuttuğu codegraph testleri varsa S3'ü yazar.
func printSelfTestsLine(w io.Writer, p runPlan) {
	if len(p.selfTests) > 0 {
		fmt.Fprintf(w, selfTestsLine, len(p.selfTests))
	}
}

// printGoTestsWithoutSelfTests `changed --go-tests --without-codegraph-self-tests`'tir (commit kipi): seçim printGoTests'inkidir,
// yalnız codegraph'ın kendi testleri iki klasörün içindeki bir değişiklik seçmediyse çıkar. Standart çıktının biçimi
// aynıdır; çıkanların sayısı S3 ile standart hataya yazılır (kip "G4'e kaldı" satırına ekler, S7).
func (ix *index) printGoTestsWithoutSelfTests(w, errw io.Writer, ts *touchSet) {
	inside := newTouchSet()
	for id, n := range ts.nodes {
		if inSelfTestFolders(n.File) {
			inside.nodes[id] = n
		}
	}
	for f := range ts.files {
		if inSelfTestFolders(f) {
			inside.files[f] = true
		}
	}
	keep := map[goTest]bool{}
	if len(inside.nodes)+len(inside.files) > 0 {
		for _, t := range ix.goTestSelection(inside) {
			keep[t] = true
		}
	}
	out := 0
	for _, t := range ix.goTestSelection(ts) {
		if !keep[t] && ix.isCodegraphSelfTest(t) {
			out++
			continue
		}
		fmt.Fprintln(w, t.dir+" "+t.name)
	}
	if out > 0 {
		fmt.Fprintf(errw, selfTestsLine, out)
	}
}

// withoutSelfTests listeden codegraph'ın kendi testlerini çıkarır; çıkanları da verir.
func (ix *index) withoutSelfTests(list []goTest) (kept, out []goTest) {
	for _, t := range list {
		if ix.isCodegraphSelfTest(t) {
			out = append(out, t)
		} else {
			kept = append(kept, t)
		}
	}
	return kept, out
}
