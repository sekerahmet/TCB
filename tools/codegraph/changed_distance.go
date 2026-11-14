package main

import (
	"sort"
	"strings"
)

// Bu dosya `changed --go-tests --by-distance`'tır (codegraph 7 Dilim B, B1): seçim bayraksızınkidir, her satırın sonuna
// bant yazılır (near ya da far) ve önce bütün yakınlar gelir. Commit öncesi denetim yakınları önce koşar; süre dolunca
// G4'e kalan uzak testlerdir, değişen dosyanın yakın testi değil.

// nearSteps yakın sayılan en uzun zincirdir (adım). Ötesi (3–8 adım), alt süreç testleri ve sınav uzaktır.
const nearSteps = 2

// goTestsInOrder `changed --go-tests`'in satırlarıdır. Bayraksız: goTestSelection, "klasör ad" sırasıyla, bant boş.
// byDistance: aynı küme, önce yakınlar; bant içinde sıra aynı; bant " near" ya da " far".
func (ix *index) goTestsInOrder(ts *touchSet, byDistance bool) (list []goTest, band func(goTest) string) {
	if !byDistance {
		return ix.goTestSelection(ts), func(goTest) string { return "" }
	}
	best, subprocess, each := ix.testsToRunEach(ts)
	list = ix.goTestSelectionFrom(ts, best, subprocess)
	near := ix.nearGoTests(ts, best, each)
	sort.SliceStable(list, func(i, j int) bool { return near[list[i]] && !near[list[j]] })
	return list, func(t goTest) string {
		if near[t] {
			return " near"
		}
		return " far"
	}
}

// nearGoTests değişikliğin yakın testleridir (B1): değişen testler; değişen Go dosyalarının adaş test dosyasındaki
// testler; değişen bir bildirimi konu edinenler; en çok nearSteps adımlık testler (doğrudan olanlar 0 adım); değişen
// dosyayı okuyan, gezen ve tarayan testler; yakın bir testin fikstür koşucusu. Küme seçimden geniş olabilir (adaş
// dosyanın seçimde olmayan testi); goTestsInOrder yalnız seçimdekileri yazar.
func (ix *index) nearGoTests(ts *touchSet, best map[string]int, each map[string]map[string]int) map[goTest]bool {
	nodes := ownTests(ts)
	for id, d := range best {
		if d <= nearSteps {
			nodes = append(nodes, ix.byID[id])
		}
	}
	for _, d := range ts.nodes {
		if isTestCode(d) {
			continue
		}
		var cands []*Node
		for id := range each[d.ID] {
			cands = append(cands, ix.byID[id])
		}
		for _, a := range ix.aboutTests(d, cands) {
			nodes = append(nodes, a.test)
		}
	}
	for f := range ts.files {
		if strings.HasSuffix(f, ".go") && !isTestFile(f) {
			if sibling := ix.byID[fileID(strings.TrimSuffix(f, ".go")+"_test.go")]; sibling != nil {
				nodes = append(nodes, ownTests(ix.fileDecls(sibling))...)
			}
		}
		for _, id := range []string{"file:" + f, trFileID(f), fileID(f), docID(f)} {
			if file := ix.byID[id]; file != nil {
				for id, d := range ix.testDistances(file) {
					if d <= nearSteps {
						nodes = append(nodes, ix.byID[id])
					}
				}
			}
		}
		for _, h := range append(ix.scansCovering(f), ix.typeQueriesCovering(f)...) {
			nodes = append(nodes, h.tests...)
		}
	}
	near := map[goTest]bool{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if t, ok := goTestOf(n); ok {
			near[t] = true
		}
		for _, r := range ix.fixtureRunners(n) {
			if t, ok := goTestOf(r); ok {
				near[t] = true
			}
		}
	}
	return near
}
