package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// indirectTest hedefi çağrı zinciriyle dolaylı sınayan bir testtir.
type indirectTest struct {
	test  *Node
	steps int   // test ile hedef arasındaki çağrı adımı sayısı
	via   *Node // zincirde hedefe en yakın çağıran
}

// indirectTests hedefi çağıranlara doğru (walkCallers: tek zincir kuralı, test_reach.go) izler ve yolda bulduğu testleri
// en yakından uzağa verir; bir testin ötesine geçilmez. Doğrudan `tests` kenarı olanlar listeye girmez.
func (ix *index) indirectTests(target *Node) []indirectTest {
	direct := map[string]bool{}
	for _, e := range ix.in[target.ID] {
		if e.Kind == edgeTests {
			direct[e.From] = true
		}
	}
	var out []indirectTest
	walkCallers(target.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		n := ix.byID[r.ID]
		if !isTestNode(n) {
			return true
		}
		if !direct[n.ID] {
			out = append(out, indirectTest{n, r.Steps, ix.byID[r.Via]})
		}
		return false
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].steps != out[j].steps {
			return out[i].steps < out[j].steps
		}
		return out[i].test.Name < out[j].test.Name
	})
	return out
}

// callersOf bir düğümü çağıran ya da kullananlardır (reachEdge kenarları), kimliğe göre sıralı.
func (ix *index) callersOf(id string) []string {
	var out []string
	for _, e := range ix.in[id] {
		if reachEdge(e.Kind, e.Confidence, e.From, e.To) {
			out = append(out, e.From)
		}
	}
	sort.Strings(out)
	return out
}

// pkgOf düğümün paket klasörüdür.
func (ix *index) pkgOf(id string) string {
	if n := ix.byID[id]; n != nil {
		return path.Dir(n.File)
	}
	return ""
}

// printTestsFor n'yi sınayan testleri yazar: önce doğrudan (EXTRACTED: çağrıdan ya da paketten),
// sonra adından çıkarılanlar, sonra çağrı zinciriyle dolaylı olanlar (INFERRED, kaç adım
// uzakta, en yakın önce); koruduğu kurallar en sonda.
func (ix *index) printTestsFor(w io.Writer, n *Node, limit int) {
	fmt.Fprintln(w, describe(n))
	var tests, measured []*Edge
	for _, e := range ix.in[n.ID] {
		switch {
		case e.Kind != edgeTests:
		case isMeasurement(ix.byID[e.From]):
			measured = append(measured, e)
		default:
			tests = append(tests, e)
		}
	}
	walks := ix.walkEdges(n)
	indirect, indirectMeasured := splitMeasurements(ix.indirectTests(n), func(t indirectTest) *Node { return t.test })
	viaNames, _ := splitMeasurements(ix.testsThroughNames(n), func(t nameTest) *Node { return t.test })
	if len(tests) == 0 && len(indirect) == 0 && len(walks) == 0 && len(viaNames) == 0 && len(measured) == 0 && len(indirectMeasured) == 0 {
		fmt.Fprintln(w, "  no tests found")
		return
	}
	if len(tests) == 0 && len(indirect) == 0 && len(walks) == 0 && len(viaNames) == 0 {
		fmt.Fprintln(w, "  no tests found (only measurements, below)")
	}
	ix.printGroups(w, "  <-", tests, fromOf, limit, true) // doğrudan testler önce: bu adı ya da dosyayı kullananlar
	ix.printWalks(w, walks, limit)
	ix.printInferred(w, n, indirect, viaNames, limit)
	ix.printMeasurements(w, measured, indirectMeasured, limit)
	ix.printGuardedRules(w, tests, indirect, limit)
}

// isMeasurement düğüm bir test değil ölçüm mü: Benchmark* işlevi ya da tools/bench altındaki kod. Ölçüm sonuç
// doğrulamaz, süre ölçer; "önce ölç" kuralı için testlerden ayrı gösterilir.
func isMeasurement(n *Node) bool {
	if n == nil {
		return false
	}
	if strings.HasPrefix(n.File, "tools/bench/") {
		return true
	}
	return n.Kind == kindFunc && strings.HasPrefix(n.Name[strings.LastIndex(n.Name, ".")+1:], "Benchmark")
}

// splitMeasurements listeyi testler ve ölçümler olarak ikiye böler.
func splitMeasurements[T any](list []T, node func(T) *Node) (tests, measured []T) {
	for _, it := range list {
		if isMeasurement(node(it)) {
			measured = append(measured, it)
		} else {
			tests = append(tests, it)
		}
	}
	return tests, measured
}

// printMeasurements ölçümleri (benchmark'lar, tools/bench) testlerden ayrı başlık altında yazar.
func (ix *index) printMeasurements(w io.Writer, direct []*Edge, indirect []indirectTest, limit int) {
	total := len(direct) + len(indirect)
	if total == 0 {
		return
	}
	fmt.Fprintf(w, "  <- measurements, not tests (%d; benchmarks and tools/bench: they time it, they do not check it):\n", total)
	var lines []string
	for _, e := range direct {
		lines = append(lines, ix.edgeLine(e, e.From, true))
	}
	for _, it := range indirect {
		lines = append(lines, fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name))
	}
	printLimited(w, lines, limit, "    ")
}

func (ix *index) printIndirect(w io.Writer, list []indirectTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests, indirect (%d, through calls, up to %d steps and %d package boundary):\n", len(list), testMaxSteps, testMaxHops)
	forEachLimited(w, list, limit, "    ", moreHint, func(it indirectTest) string {
		return fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name)
	})
}

// printGuardedRules testlerin koruduğu kuralları (rule düğümleri) yazar.
func (ix *index) printGuardedRules(w io.Writer, tests []*Edge, indirect []indirectTest, limit int) {
	froms := map[string]bool{}
	for _, e := range tests {
		froms[e.From] = true
	}
	for _, it := range indirect {
		froms[it.test.ID] = true
	}
	rules := map[string]bool{}
	for id := range froms {
		for _, g := range ix.in[id] {
			if g.Kind == edgeGuardedBy {
				rules[g.From] = true
			}
		}
	}
	ids := make([]string, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return
	}
	fmt.Fprintf(w, "  rules guarded by these tests (%d):\n", len(ids))
	forEachLimited(w, ids, limit, "    ", moreHint, func(id string) string { return describe(ix.byID[id]) })
}

// weakSteps bundan uzun zincirle bulunan test zayıf kanıttır: zincir sınırının (testMaxSteps) yarısı. Sekiz adımlık bir
// yol genellikle ortak bir altyapıdan (yorumlayıcı, kayıt) geçer ve konuyla ilgisiz testleri de toplar; bir adımlık bir
// yol testin konuyu sınadığını söyler.
const weakSteps = testMaxSteps / 2

// evidence bir test grubunun en güçlü kanıtıdır: en az adım ve hedefle aynı paket. Gruplar buna göre sıralanır.
type evidence struct {
	steps int
	local bool
}

func (a evidence) stronger(b evidence) bool {
	if a.steps != b.steps {
		return a.steps < b.steps
	}
	return a.local && !b.local
}

// printInferred çağrı zinciriyle ve ad koduyla bulunan testleri kanıt gücüne göre yazar: kısa zincirler (en çok weakSteps
// adım) iki grupta, en güçlü kanıtı olan grup önce; uzun zincirler ayrı "weak evidence" başlığı altında en sonda.
func (ix *index) printInferred(w io.Writer, n *Node, indirect []indirectTest, viaNames []nameTest, limit int) {
	dir := path.Dir(n.File)
	var strongInd, weakInd []indirectTest
	bestInd := evidence{steps: testMaxSteps + 1}
	for _, it := range indirect {
		if it.steps > weakSteps {
			weakInd = append(weakInd, it)
			continue
		}
		strongInd = append(strongInd, it)
		if e := (evidence{it.steps, path.Dir(it.test.File) == dir}); e.stronger(bestInd) {
			bestInd = e
		}
	}
	var strongNames, weakNames []nameTest
	bestNames := evidence{steps: testMaxSteps + 1}
	for _, t := range viaNames {
		if t.steps > weakSteps {
			weakNames = append(weakNames, t)
			continue
		}
		strongNames = append(strongNames, t)
		if e := (evidence{t.steps, t.local}); e.stronger(bestNames) {
			bestNames = e
		}
	}
	if bestNames.stronger(bestInd) {
		ix.printViaNames(w, strongNames, limit)
		ix.printIndirect(w, strongInd, limit)
	} else {
		ix.printIndirect(w, strongInd, limit)
		ix.printViaNames(w, strongNames, limit)
	}
	if len(weakInd)+len(weakNames) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- weak evidence (%d; chains longer than %d steps, often through shared machinery: read with care):\n", len(weakInd)+len(weakNames), weakSteps)
	var lines []string
	for _, it := range weakInd {
		lines = append(lines, fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name))
	}
	for _, t := range weakNames {
		lines = append(lines, fmt.Sprintf("%s  via %s, %d steps  [INFERRED]", describe(t.test), t.name.Name, t.steps))
	}
	printLimited(w, lines, limit, "    ")
}

// testDistances n'yi sınayan testlerin en kısa uzaklığıdır (0: doğrudan ya da bütün listeyi gezen; >0: zincir ya da ad kodu
// üzerinden adım sayısı). tests-for'un gösterdiği testlerin aynısıdır (ölçümler hariç); `changed` bunu kullanır, böylece
// "bu değişikliği hangi testler koşar" ile "bu işlevi hangi testler sınar" aynı cevabı verir (bağımsız denetim Y3).
func (ix *index) testDistances(n *Node) map[string]int {
	best := map[string]int{}
	note := func(id string, d int) {
		if t := ix.byID[id]; t == nil || !isTestNode(t) || isMeasurement(t) {
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
	for _, e := range ix.walkEdges(n) {
		note(e.From, 0)
	}
	for _, it := range ix.indirectTests(n) {
		note(it.test.ID, it.steps)
	}
	for _, t := range ix.testsThroughNames(n) {
		note(t.test.ID, t.steps)
	}
	return best
}
