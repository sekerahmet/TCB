package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Bu dosya dosya düzeyindeki tests-for'u kurar: bir Go dosyasını koruyan testler, dosyadaki bildirimlerin testlerinin
// birleşimidir (changed'in toplamasıyla aynı).

// testDistances n'yi sınayan testlerin en kısa uzaklığıdır (0: doğrudan ya da bütün listeyi gezen; >0: zincir ya da ad kodu
// üzerinden adım sayısı). tests-for'un gösterdiği testlerin aynısıdır (ölçümler hariç); `changed` bunu kullanır, böylece
// "bu değişikliği hangi testler koşar" ile "bu işlevi hangi testler sınar" aynı cevabı verir (bağımsız denetim Y3).
func (ix *index) testDistances(n *Node) map[string]int {
	best := map[string]int{}
	note := func(id string, d int) {
		if t := ix.byID[id]; t == nil || (!isTestNode(t) && !isNegativeFixture(t)) || isMeasurement(t) {
			return
		}
		if old, ok := best[id]; !ok || d < old {
			best[id] = d
		}
	}
	for _, e := range ix.in[n.ID] {
		if e.Kind == edgeTests {
			note(e.From, 0)
		}
	}
	for _, f := range ix.fixtureTests(n) {
		note(f.ID, 0)
	}
	for _, e := range ix.walkEdges(n) {
		note(e.From, 0)
	}
	for _, it := range append(ix.valueTests(n), ix.indirectTests(n)...) {
		note(it.test.ID, it.steps)
	}
	for _, t := range ix.testsThroughNames(n) {
		note(t.test.ID, t.steps)
	}
	return best
}

// goFileTestsBody bir Go dosyasını koruyan testlerdir: dosyadaki bildirimlerin (işlev, metot, tip, değer) testlerinin
// birleşimi, `changed`'in değişen düğümler için yaptığı toplamanın aynısı (testsToRun; alt süreç testleri dahil).
// "Bir dosyaya dokunmadan önce tests-for <dosya>" kuralı Go dosyasında da cevap alır. Liste eksiltilmez, bildirim
// düzeyinin gruplarıyla ve sırasıyla yazılır (fileTests). Testler bir kez toplanır, dönen işlev onları verilen sınırla
// yazar (tests-for --all iki sınırla yazar; hız işi, tekrar 1).
func (ix *index) goFileTestsBody(file *Node) func(io.Writer, int) {
	ts := ix.fileDecls(file)
	decls := len(ts.nodes)
	if sibling := ix.byID[fileID(strings.TrimSuffix(file.File, ".go")+"_test.go")]; !isTestFile(file.File) && sibling != nil {
		// Adaş test dosyasının (aynı ya da dış `_test` paketteki x_test.go) testleri dosyanın testleridir: ilk grupta.
		for _, t := range ownTests(ix.fileDecls(sibling)) {
			ts.nodes[t.ID] = t
		}
	}
	s := ix.fileTests(file, ts)
	return func(w io.Writer, limit int) { ix.printGoFileTests(w, file, ts, decls, s, limit) }
}

// printGoFileTests bir Go dosyasının toplanmış testlerini (goFileTestsBody) yazar.
func (ix *index) printGoFileTests(w io.Writer, file *Node, ts *touchSet, decls int, s fileTestSet, limit int) {
	if isUntracked(ix.root, file.File) {
		printUntrackedWarning(w, file.File)
	}
	writeRunPlan(w, func(l listedTests) runPlan { return ix.runPlanWith(file, ts, s, l) })
	ix.printSizes(w, file)
	fmt.Fprintf(w, "  declarations in this file: %d\n", decls)
	ix.printFileTests(w, file, s, limit)
	ix.printGoFileExams(w, file, ts, s)
	ix.printScans(w, file.File, limit)
	ix.printTypeQueries(w, file.File, limit)
	tests := map[string]bool{}
	for _, t := range s.own {
		tests[t.ID] = true
	}
	for id := range s.all {
		tests[id] = true
	}
	for _, h := range append(ix.scansCovering(file.File), ix.typeQueriesCovering(file.File)...) {
		for _, t := range h.tests {
			tests[t.ID] = true
		}
	}
	ix.printRulesGuardedBy(w, tests, limit)
}

// fileTestSet bir Go dosyasını koruyan testlerin grupları: koşulan küme changed'inkidir (testsToRun ve dosyanın
// kendi testleri), gruplar bildirim düzeyinin gruplarıdır.
type fileTestSet struct {
	own        []*Node            // dosyanın kendi ve adaş test dosyasının testleri
	all        map[string]int     // testsToRun ve dosyanın kendi testleri (testDistances): test -> en kısa uzaklık
	extracted  map[string]bool    // d == 0 kanıtı çıkarım olmayan testler ("direct" yazılanlar)
	about      []aboutTest        // bir bildirimini konu edinenler (ilk grup)
	others     []string           // kalanlar, kanıt sırasıyla (testRank)
	subprocess map[string][]*Node // ikiliyi derleyip koşan testler
}

// extractedDirectTests verilen düğümlerden en az birini çıkarım olmayan doğrudan kanıtla (EXTRACTED `tests` kenarı,
// negatif fikstür ya da aynı paketteki ad kodu uygulaması) sınayan testlerin kümesidir; INFERRED `tests` ve `walks`
// kenarları buraya girmez, böylece `direct` yazılmazlar.
func (ix *index) extractedDirectTests(nodes []*Node) map[string]bool {
	out := map[string]bool{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		for _, e := range ix.in[n.ID] {
			if e.Kind == edgeTests && e.Confidence == extracted {
				out[e.From] = true
			}
		}
		for _, f := range ix.fixtureTests(n) {
			out[f.ID] = true
		}
		for _, t := range ix.testsThroughNames(n) {
			if t.steps == 0 {
				out[t.test.ID] = true
			}
		}
	}
	return out
}

// fileTests dosyanın testlerini toplar ve sıralar. Konu edinen testler her bildirim için bildirim düzeyindeki gibi
// seçilir (aboutTests, adaylar o bildirimin testleri); her test bir kez, ilk grubunda.
func (ix *index) fileTests(file *Node, ts *touchSet) fileTestSet {
	all, subprocess := ix.testsToRun(ts)
	for id, d := range ix.testDistances(file) {
		if old, ok := all[id]; !ok || d < old {
			all[id] = d
		}
	}
	targets := []*Node{file}
	decls := make([]*Node, 0, len(ts.nodes))
	for _, d := range ts.nodes {
		targets = append(targets, d)
		if !isTestCode(d) {
			decls = append(decls, d)
		}
	}
	s := fileTestSet{own: ownTests(ts), all: all, extracted: ix.extractedDirectTests(targets), subprocess: subprocess}
	sort.Slice(decls, func(i, j int) bool { return decls[i].ID < decls[j].ID })
	shown := map[string]bool{}
	for _, d := range decls {
		var cands []*Node
		for id := range ix.testDistances(d) {
			cands = append(cands, ix.byID[id])
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].ID < cands[j].ID })
		for _, a := range ix.aboutTests(d, cands) {
			if !shown[a.test.ID] && ts.nodes[a.test.ID] == nil {
				shown[a.test.ID] = true
				s.about = append(s.about, a)
			}
		}
	}
	rank := func(id string) testRank { return newTestRank(file, ix.byID[id], all[id], false) }
	sort.SliceStable(s.about, func(i, j int) bool { return rank(s.about[i].test.ID).before(rank(s.about[j].test.ID)) })
	for id := range all {
		if isTestNode(ix.byID[id]) && ts.nodes[id] == nil && !shown[id] {
			s.others = append(s.others, id)
		}
	}
	sort.Slice(s.others, func(i, j int) bool {
		a, b := s.others[i], s.others[j]
		if all[a] == 0 && all[b] == 0 && s.extracted[a] != s.extracted[b] {
			return s.extracted[a]
		}
		return rank(a).before(rank(b))
	})
	return s
}

// printFileTests dosyanın testlerini bildirim düzeyinin gruplarıyla yazar: kendi testleri, konu edinenler, 0 adımlı
// testler (EXTRACTED "direct" önce, sonra "[INFERRED]"), kısa zincirler (en çok weakSteps adım) ve "weak evidence";
// alt süreç testleri en sonda.
func (ix *index) printFileTests(w io.Writer, file *Node, s fileTestSet, limit int) {
	defer ix.printSubprocessTests(w, s.subprocess)
	printOwnTests(w, s.own, limit)
	if len(s.own)+len(s.about)+len(s.others)+len(s.subprocess) == 0 {
		fmt.Fprintln(w, "  tests: none found for the declarations in this file")
		return
	}
	ix.printAboutTests(w, s.about, limit)
	var direct, strong, weak []string
	for _, id := range s.others {
		switch d := s.all[id]; {
		case d == 0 && s.extracted[id]:
			direct = append(direct, describe(ix.byID[id])+"  direct")
		case d == 0:
			direct = append(direct, describe(ix.byID[id])+"  [INFERRED]")
		case d <= weakSteps:
			strong = append(strong, fmt.Sprintf("%s  %d steps [INFERRED]", describe(ix.byID[id]), d))
		default:
			weak = append(weak, fmt.Sprintf("%s  %d steps [INFERRED]", describe(ix.byID[id]), d))
		}
	}
	if len(direct) > 0 {
		fmt.Fprintf(w, "  <- tests (%d, they use, read or walk this file or a declaration in it):\n", len(direct))
		printLimited(w, direct, limit, "    ")
	}
	if len(strong) > 0 {
		fmt.Fprintf(w, "  <- tests, indirect (%d, through calls, loaded files or name codes, up to %d steps; same folder first at equal steps):\n", len(strong), weakSteps)
		printLimited(w, strong, limit, "    ")
	}
	printWeak(w, weak, limit)
}

func printOwnTests(w io.Writer, own []*Node, limit int) {
	if len(own) > 0 {
		fmt.Fprintf(w, "  this file's tests (%d):\n", len(own))
		forEachLimited(w, own, limit, "    ", moreHint, describe)
	}
}

// trFileOwnTests bir .tr dosyasının kendi bildirdiği testleri ve (dosya *_test.tr değilse) adaş *_test.tr dosyasının
// bildirdiği testlerdir (Go dosyasındaki adaş test kuralı gibi: ilk grupta).
func (ix *index) trFileOwnTests(n *Node) []*Node {
	if n.Kind != kindFile || !strings.HasSuffix(n.File, ".tr") {
		return nil
	}
	ts := newTouchSet()
	collect := func(f *Node) {
		if f == nil {
			return
		}
		for _, e := range ix.out[f.ID] {
			if t := ix.byID[e.To]; e.Kind == edgeContains && t != nil && t.Kind == kindTRTest {
				ts.nodes[t.ID] = t
			}
		}
	}
	collect(n)
	if !strings.HasSuffix(n.File, "_test.tr") {
		collect(ix.byID[trFileID(strings.TrimSuffix(n.File, ".tr")+"_test.tr")])
	}
	return ownTests(ts)
}

func (ix *index) directTestEdges(n *Node) (tests, measured []*Edge) {
	for _, e := range ix.in[n.ID] {
		switch {
		case e.Kind != edgeTests:
		case isMeasurement(ix.byID[e.From]):
			measured = append(measured, e)
		default:
			tests = append(tests, e)
		}
	}
	return tests, measured
}

// fileDecls bir Go dosyasının bildirimleridir (işlev, metot, tip, değer).
func (ix *index) fileDecls(file *Node) *touchSet {
	ts := newTouchSet()
	for _, e := range ix.out[file.ID] {
		if d := ix.byID[e.To]; e.Kind == edgeContains && d != nil && strings.HasPrefix(d.ID, "go:") && d.Kind != kindFile {
			ts.nodes[d.ID] = d
		}
	}
	return ts
}
