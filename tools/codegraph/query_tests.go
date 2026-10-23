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

// valueTests hedefi test dosyasında işlev değeri olarak (tablo öğesi, metot ifadesi, kayıt) kullanan testlerdir. Go'da
// değer olarak geçen işlev INFERRED `calls` kenarıdır ve zincire girmez (çağrı sonradan, başka yerde olabilir); ama
// kullanan test dosyasındaysa o dosyanın testleri hedefi sınar: kullanan bir testse kendisi (1 adım), bir tablo ya da
// yardımcıysa onu okuyan ve çağıran ilk testler (zincir kuralıyla). Doğrudan `tests` kenarı olanlar listeye girmez.
func (ix *index) valueTests(target *Node) []indirectTest {
	seen := map[string]bool{}
	for _, e := range ix.in[target.ID] {
		if e.Kind == edgeTests {
			seen[e.From] = true
		}
	}
	var out []indirectTest
	add := func(t, via *Node, steps int) {
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, indirectTest{t, steps, via})
		}
	}
	for _, e := range ix.in[target.ID] {
		from := ix.byID[e.From]
		if e.Kind != edgeCalls || e.Confidence != inferred || from == nil || !inTestFile(from) {
			continue
		}
		if isTestNode(from) {
			add(from, nil, 1) // testin kendisi kullanıyor
			continue
		}
		walkCallers(from.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
			if t := ix.byID[r.ID]; isTestNode(t) {
				add(t, from, r.Steps+1)
				return false
			}
			return true
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].steps != out[j].steps {
			return out[i].steps < out[j].steps
		}
		return out[i].test.Name < out[j].test.Name
	})
	return out
}

// printValueTests değer olarak sınayan testleri yazar.
func (ix *index) printValueTests(w io.Writer, list []indirectTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests that use it as a function value (%d; a table entry, a method expression, a registration in a test file):\n", len(list))
	forEachLimited(w, list, limit, "    ", moreHint, func(it indirectTest) string {
		if it.via == nil {
			return describe(it.test) + "  in the test itself  [INFERRED]"
		}
		return fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name)
	})
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
	if n.Kind == kindFile && strings.HasSuffix(n.File, ".go") {
		ix.printGoFileTests(w, n, limit)
		return
	}
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
	asValue, _ := splitMeasurements(ix.valueTests(n), func(t indirectTest) *Node { return t.test })
	indirect, indirectMeasured := splitMeasurements(ix.indirectTests(n), func(t indirectTest) *Node { return t.test })
	viaNames, _ := splitMeasurements(ix.testsThroughNames(n), func(t nameTest) *Node { return t.test })
	subprocess := ix.subprocessTests(n)
	found := len(tests) + len(walks) + len(asValue) + len(indirect) + len(viaNames)
	if found == 0 && len(measured) == 0 && len(indirectMeasured) == 0 && len(subprocess) == 0 {
		if !ix.printLinkedBinaries(w, n) {
			fmt.Fprintln(w, "  no tests found")
		}
		ix.printOtherConfigs(w, n, limit)
		return
	}
	switch {
	case found == 0 && len(subprocess) > 0:
		fmt.Fprintln(w, "  no tests traced to it (only subprocess tests, below)")
	case found == 0:
		fmt.Fprintln(w, "  no tests found (only measurements, below)")
	}
	ix.printGroups(w, "  <-", tests, fromOf, limit, true) // doğrudan testler önce: bu adı ya da dosyayı kullananlar
	ix.printWalks(w, walks, limit)
	ix.printValueTests(w, asValue, limit)
	ix.printInferred(w, n, indirect, viaNames, limit)
	ix.printSubprocessTests(w, subprocess)
	ix.printMeasurements(w, measured, indirectMeasured, limit)
	ix.printGuardedRules(w, tests, indirect, limit)
	ix.printOtherConfigs(w, n, limit)
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
	ix.printRulesGuardedBy(w, froms, limit)
}

// printRulesGuardedBy testlerin koruduğu kuralları yazar.
func (ix *index) printRulesGuardedBy(w io.Writer, froms map[string]bool, limit int) {
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
	for _, it := range append(ix.valueTests(n), ix.indirectTests(n)...) {
		note(it.test.ID, it.steps)
	}
	for _, t := range ix.testsThroughNames(n) {
		note(t.test.ID, t.steps)
	}
	return best
}

// subprocessTests n'nin paketini derleyip ikilisini koşan testlerdir, paket klasörüne göre gruplu. Derleyen TestMain ise
// onun paketindeki bütün testler (ikiliyi hepsi paylaşır); başka bir işlevse onu çağıran ilk testler (zincir kuralı).
func (ix *index) subprocessTests(n *Node) map[string][]*Node {
	return ix.subprocessTestsOf(ix.goPackageOf(n))
}

// subprocessTestsOf subprocessTests'tir, paket kimliğiyle.
func (ix *index) subprocessTestsOf(pkg string) map[string][]*Node {
	var builders []*Node
	for _, e := range ix.in[pkg] {
		if e.Kind == edgeBuilds && ix.byID[e.From] != nil {
			builders = append(builders, ix.byID[e.From])
		}
	}
	out := map[string][]*Node{}
	seen := map[string]bool{}
	add := func(t *Node) {
		if !seen[t.ID] && !isMeasurement(t) {
			seen[t.ID] = true
			out[path.Dir(t.File)] = append(out[path.Dir(t.File)], t)
		}
	}
	for _, b := range builders {
		if strings.HasSuffix(b.Name, ".TestMain") {
			for _, t := range ix.testsInDir(path.Dir(b.File)) {
				if t.ID != b.ID {
					add(t)
				}
			}
			continue
		}
		walkCallers(b.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
			if t := ix.byID[r.ID]; isTestNode(t) {
				add(t)
				return false
			}
			return true
		})
	}
	return out
}

// goPackageOf düğümün dosyasını içeren Go paketi düğümünün kimliğidir (paket → dosya `contains` kenarı); yoksa "".
func (ix *index) goPackageOf(n *Node) string {
	for _, e := range ix.in[fileID(n.File)] {
		if e.Kind == edgeContains && strings.HasPrefix(e.From, "go:pkg:") {
			return e.From
		}
	}
	return ""
}

// testsInDir klasördeki Go testleridir.
func (ix *index) testsInDir(dir string) []*Node {
	var out []*Node
	for i := range ix.g.Nodes {
		if t := &ix.g.Nodes[i]; path.Dir(t.File) == dir && isTestNode(t) {
			out = append(out, t)
		}
	}
	return out
}

// printSubprocessTests alt süreç testlerini klasör başına tek satırda sayar.
func (ix *index) printSubprocessTests(w io.Writer, byDir map[string][]*Node) {
	if len(byDir) == 0 {
		return
	}
	dirs := make([]string, 0, len(byDir))
	total := 0
	for d, ts := range byDir {
		dirs = append(dirs, fmt.Sprintf("%s (%d)", d, len(ts)))
		total += len(ts)
	}
	sort.Strings(dirs)
	fmt.Fprintf(w, "  <- covered by subprocess tests (%d; they build this package and run the binary, not traced per function): %s\n", total, strings.Join(dirs, ", "))
}

// printLinkedBinaries n'nin paketi (geçişli içe aktarmayla) bir testin derleyip koştuğu ana pakete bağlıysa bunu zayıf
// kanıt olarak yazar ve true verir: ikiliyi koşan testler bu koda varabilir, ama hangisinin vardığı izlenmez (DENETİM
// doğrulaması Y5: codeindex.LoadCached, e2e'deki --dizin-yok testine rağmen "no tests found" diyordu). Paketin kendisi
// derleniyorsa satır zaten "covered by subprocess tests"tir; burada yalnızca bağlanan paketler.
func (ix *index) printLinkedBinaries(w io.Writer, n *Node) bool {
	pkg := ix.goPackageOf(n)
	if pkg == "" {
		return false
	}
	var parts []string
	total := 0
	for _, bin := range ix.builtPackages() {
		if bin == pkg || !ix.importsTransitively(bin, pkg) {
			continue
		}
		byDir := ix.subprocessTestsOf(bin)
		count := 0
		for _, ts := range byDir {
			count += len(ts)
		}
		total += count
		parts = append(parts, fmt.Sprintf("%s (%d tests run it)", ix.byID[bin].Name, count))
	}
	if len(parts) == 0 {
		return false
	}
	sort.Strings(parts)
	fmt.Fprintln(w, "  no tests traced to it")
	fmt.Fprintf(w, "  <- weak evidence (%d): linked into the binary of %s; a subprocess test may reach this code, which one is not traced\n", total, strings.Join(parts, ", "))
	return true
}

// builtPackages bir testin `go build` ile derlediği paketlerdir (builds kenarının hedefi), sıralı.
func (ix *index) builtPackages() []string {
	seen := map[string]bool{}
	var out []string
	for i := range ix.g.Edges {
		if e := &ix.g.Edges[i]; e.Kind == edgeBuilds && !seen[e.To] {
			seen[e.To] = true
			out = append(out, e.To)
		}
	}
	sort.Strings(out)
	return out
}

// importsTransitively from paketi to paketini (doğrudan ya da dolaylı) içe aktarıyor mu.
func (ix *index) importsTransitively(from, to string) bool {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range ix.out[cur] {
			if e.Kind != edgeImports || seen[e.To] {
				continue
			}
			if e.To == to {
				return true
			}
			seen[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return false
}

// printGoFileTests bir Go dosyasını koruyan testlerdir: dosyadaki bildirimlerin (işlev, metot, tip, değer) testlerinin
// birleşimi, `changed`'in değişen düğümler için yaptığı toplamanın aynısı (testDistances; alt süreç testleri dahil).
// "Bir dosyaya dokunmadan önce tests-for <dosya>" kuralı Go dosyasında da cevap alır.
func (ix *index) printGoFileTests(w io.Writer, file *Node, limit int) {
	ts := newTouchSet()
	for _, e := range ix.out[file.ID] {
		if d := ix.byID[e.To]; e.Kind == edgeContains && d != nil && strings.HasPrefix(d.ID, "go:") && d.Kind != kindFile {
			ts.nodes[d.ID] = d
		}
	}
	fmt.Fprintf(w, "  declarations in this file: %d\n", len(ts.nodes))
	ix.printTestsToRun(w, ts, limit, "the declarations in this file")
	tests := map[string]bool{}
	for _, d := range ts.nodes {
		for id := range ix.testDistances(d) {
			tests[id] = true
		}
	}
	ix.printRulesGuardedBy(w, tests, limit)
}
