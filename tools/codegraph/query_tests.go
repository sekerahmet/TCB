package main

import (
	"fmt"
	"io"
	"path"
	"slices"
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
	seen := map[string]int{}
	var out []indirectTest
	note := func(t, via *Node, steps int) {
		if direct[t.ID] || steps > testMaxSteps {
			return
		}
		if idx, ok := seen[t.ID]; ok {
			if steps < out[idx].steps {
				out[idx] = indirectTest{t, steps, via}
			}
			return
		}
		seen[t.ID] = len(out)
		out = append(out, indirectTest{t, steps, via})
	}
	walkCallers(target.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		n := ix.byID[r.ID]
		if !isTestNode(n) && !isTRMeasurement(n) {
			if ix.pkgOf(r.ID) == ix.pkgOf(target.ID) && r.Steps < testMaxSteps {
				for _, e := range ix.in[r.ID] {
					if tn := ix.byID[e.From]; e.Kind == edgeTests && e.Confidence == inferred && isTestNode(tn) && ix.pkgOf(tn.ID) != ix.pkgOf(r.ID) {
						note(tn, n, r.Steps+1)
					}
				}
			}
			return true
		}
		note(n, ix.byID[r.Via], r.Steps)
		return false
	})
	out = append(out, ix.loaderTests(target, direct)...)
	// Aynı uzaklıkta dosyanın kendi test dosyasındakiler önce (lib/layers.tr için lib/layers_test.tr).
	own := func(t *Node) bool {
		stem := strings.TrimSuffix(target.File, path.Ext(target.File))
		return target.Kind == kindFile && strings.TrimSuffix(t.File, path.Ext(t.File)) == stem+"_test"
	}
	rank := func(it indirectTest) testRank { return newTestRank(target, it.test, it.steps, own(it.test)) }
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]).before(rank(out[j])) })
	return out
}

// testRank bir testin bir hedefe göre sıralama anahtarıdır. Bildirim düzeyi (indirectTests) ve dosya düzeyi
// (printGoFileTests) aynı kuralla sıralar: kısa zincir önce; eşit uzaklıkta hedefin kendi test dosyasındaki, sonra
// hedefle aynı klasördeki (düzenlenen paketin testi), sonra ada göre (DENETİM 2026-10-29: `tests-for assign.go`'da
// interp'in 2 adımdaki ilk testi 140. satırdaydı, önünde başka paketlerin testleri alfabetik sırayla duruyordu).
type testRank struct {
	steps      int
	own, local bool
	name       string
}

func newTestRank(target, test *Node, steps int, own bool) testRank {
	return testRank{steps, own, path.Dir(test.File) == path.Dir(target.File), test.Name}
}

func (a testRank) before(b testRank) bool {
	switch {
	case a.steps != b.steps:
		return a.steps < b.steps
	case a.own != b.own:
		return a.own
	case a.local != b.local:
		return a.local
	}
	return a.name < b.name
}

// loaderTests bir dosyayı yükleyen dosyaları (`loads`, js_loads.go) geriye doğru izler ve onları koşan testleri verir:
// e2e.TestExtensionJSTests test/coded_view_test.js'yi koşar, o da coded_view.js'yi yükler (2 adım, via test dosyası).
// TR'de aktarma da yüklemedir: `.tr` dosyasının testleri onu (geçişli) aktaran `.tr` dosyalarının testleridir, onları
// koşan Go testleri (TestGPTTrainingScriptSmoke → tools/gpu/gpt_training.tr → lib/layers.tr) ve içlerindeki TR testleri
// (lib/layers_test.tr'nin testleri, 1 adım). Doğrudan `tests` kenarı olanlar (skip) listeye girmez; her test bir kez,
// en yakın yükleyiciyle.
func (ix *index) loaderTests(target *Node, skip map[string]bool) []indirectTest {
	if target.Kind != kindFile {
		return nil
	}
	seen := map[string]bool{target.ID: true}
	shown := map[string]bool{}
	var out []indirectTest
	level := []string{target.ID}
	for depth := 1; depth < testMaxSteps && len(level) > 0; depth++ {
		var next []string
		for _, id := range level {
			for _, e := range ix.in[id] {
				imports := e.Kind == edgeImports && strings.HasPrefix(e.From, "tr:file:")
				if e.Kind != edgeLoads && !imports || seen[e.From] {
					continue
				}
				seen[e.From] = true
				next = append(next, e.From)
				add := func(tn *Node, steps int) {
					if tn != nil && !skip[tn.ID] && !shown[tn.ID] {
						shown[tn.ID] = true
						out = append(out, indirectTest{tn, steps, ix.byID[e.From]})
					}
				}
				for _, t := range ix.in[e.From] {
					if t.Kind == edgeTests {
						add(ix.byID[t.From], depth+1)
					}
				}
				for _, c := range ix.out[e.From] {
					if tn := ix.byID[c.To]; imports && c.Kind == edgeContains && tn != nil && tn.Kind == kindTRTest {
						add(tn, depth)
					}
				}
			}
		}
		sort.Strings(next)
		level = next
	}
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
func (ix *index) callersOf(id string) []string { return ix.callers(id, true) }

// nameCallersOf ad kodu katmanının yürüyüşüdür: callersOf gibi, ama argüman olarak geçirilen işlevden onu alan
// işleve (viaArg) geçmez. Alan işlev çoğu kez bir kayıt işlevidir (`ml.Register`) ve paketinin bütün kodlarını
// kullanır; argüman olan işlev onları uygulamaz (DENETİM 279 Ş1: `interp.PathArg` 16 alakasız kodun 251 testini
// alıyordu). Test erişimi (callersOf) argümanı izlemeye devam eder.
func (ix *index) nameCallersOf(id string) []string { return ix.callers(id, false) }

func (ix *index) callers(id string, throughArgs bool) []string {
	var out []string
	for _, e := range ix.in[id] {
		if !throughArgs && (e.Via == viaArg || e.Via == viaVar) && e.Confidence != extracted {
			continue
		}
		if reachEdge(e.Kind, e.Confidence, e.From, e.To, e.Via) {
			out = append(out, e.From)
		}
	}
	for _, e := range ix.out[id] {
		if v, ok := initReader(e.Kind, e.From, e.To); ok {
			out = append(out, v)
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
	if n.Kind == kindFile && isUntracked(ix.root, n.File) {
		printUntrackedWarning(w, n.File)
	}
	t := ix.testsForNode(n)
	switch t.branch {
	case branchWeak, branchExam, branchUntested:
		if t.branch == branchWeak {
			printLinkedBinaries(w, t.linked, t.linkedTotal)
		} else if t.branch == branchExam {
			fmt.Fprintln(w, "  no tests found (only exam questions that name it, below)")
		} else {
			fmt.Fprintln(w, "  no tests found"+searchedFor(n))
		}
		ix.printExamEdge(w, t.exam)
		ix.printOtherConfigs(w, n, limit)
		return
	case branchSubprocess:
		fmt.Fprintln(w, "  no tests traced to it (only subprocess tests, below)")
	case branchRule:
		fmt.Fprintln(w, "  no tests read it by name (tests that guard its rules, cite it or scan its pattern, below)")
	case branchMeasured:
		fmt.Fprintln(w, "  no tests found (only measurements, below)")
	}
	printOwnTests(w, t.own, limit)
	ix.printAboutTests(w, t.about, limit)
	tests, withOthers := ix.splitSharedSites(t.tests)
	if n.Kind == kindFile {
		ix.printWalks(w, t.walks, limit) // veri dosyasında bütün listeyi gezenler önce (README)
	}
	ix.printGroups(w, "  <-", tests, fromOf, limit, true) // sonra doğrudan testler: bu adı ya da dosyayı kullananlar
	if n.Kind != kindFile {
		ix.printWalks(w, t.walks, limit)
	}
	ix.printValueTests(w, t.asValue, limit)
	ix.printInferred(w, n, t.indirect, t.viaNames, limit)
	ix.printSharedSites(w, withOthers, limit)
	ix.printDocRules(w, t.rules, limit)
	ix.printCites(w, t.cites, limit)
	ix.printExamEdge(w, t.exam)
	if len(t.scans) > 0 {
		ix.printScans(w, n.File, limit)
	}
	printAllSkipNote(w, t.listed)
	ix.printSubprocessTests(w, t.subprocess)
	ix.printMeasurements(w, t.measured, t.indirectMeasured, limit)
	ix.printGuardedRules(w, t.own, t.allTests, t.allIndirect, limit)
	ix.printOtherConfigsExcept(w, n, limit, t.listed)
}

// splitSharedSites dosyaya gelen test kenarlarını ayırır: testin aynı yerden (dosya:satır) başka dosyalara da giden
// kenarı varsa bir kalıp (lib/*.tr) ya da gömülü dosya kümesi (lib/embedded.go'nun go:embed *.tr'si) bu dosyayı
// ötekilerle birlikte okur (bir yol listesi de: TestDocLinks README.md'yi docs/*.md ile okur); adıyla ya da aktarmayla okuyan testten (TestReadyModelMatchesTransformers) zayıftır ve
// sonra yazılır (DENETİM, parti K2: gömülü kitaplık üzerinden gelen dört test davranış testinin önündeydi).
func (ix *index) splitSharedSites(tests []*Edge) (own, shared []*Edge) {
	for _, e := range tests {
		other := slices.ContainsFunc(ix.out[e.From], func(o *Edge) bool {
			t := ix.byID[o.To]
			return o.Kind == edgeTests && o.To != e.To && o.File == e.File && o.Line == e.Line && t != nil && (t.Kind == kindFile || t.Kind == kindDoc)
		})
		if other {
			shared = append(shared, e)
		} else {
			own = append(own, e)
		}
	}
	return own, shared
}

// printSharedSites dosyayı ötekilerle birlikte okuyan testleri (splitSharedSites) kendi grubunda yazar.
func (ix *index) printSharedSites(w io.Writer, list []*Edge, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests that read it among other files (%d; a list, a glob or an embedded file set: they read every file of it):\n", len(list))
	sort.SliceStable(list, func(i, j int) bool { return ix.byID[list[i].From].Name < ix.byID[list[j].From].Name })
	forEachLimited(w, list, limit, "    ", moreHint, func(e *Edge) string { return ix.edgeLine(e, e.From, true) })
}

// keepStrongest bir testi, en güçlü kanıtının grubundan başka gruplardan çıkarır (bağımsız denetim grup 6, S1:
// doğrudan test ad kodu grubunda bir kez daha yazılıyordu): önce onu konu edinenler, sonra bütün listeyi gezenler,
// doğrudan kullananlar, değer olarak kullananlar; zincir ve ad kodu grupları kanıt gücüne göre sıralanır
// (printInferred), ikisinde birden olan test daha kısa olanında kalır.
func keepStrongest(own []*Node, about []aboutTest, walks, tests []*Edge, asValue, indirect []indirectTest, viaNames []nameTest) ([]*Edge, []*Edge, []indirectTest, []indirectTest, []nameTest) {
	shown := map[string]bool{}
	for _, t := range own {
		shown[t.ID] = true
	}
	for _, a := range about {
		shown[a.test.ID] = true
	}
	walks = slices.DeleteFunc(slices.Clone(walks), func(e *Edge) bool { return shown[e.From] })
	for _, e := range walks {
		shown[e.From] = true // bütün listeyi gezen test her öğeyi sınar: doğrudan kullananlardan önce kendi grubunda
	}
	tests = slices.DeleteFunc(slices.Clone(tests), func(e *Edge) bool { return shown[e.From] })
	for _, e := range tests {
		shown[e.From] = true
	}
	asValue = slices.DeleteFunc(slices.Clone(asValue), func(it indirectTest) bool { return shown[it.test.ID] })
	for _, it := range asValue {
		shown[it.test.ID] = true
	}
	indirect = slices.DeleteFunc(slices.Clone(indirect), func(it indirectTest) bool { return shown[it.test.ID] })
	viaNames = slices.DeleteFunc(slices.Clone(viaNames), func(t nameTest) bool { return shown[t.test.ID] })
	chain := map[string]int{}
	for _, it := range indirect {
		chain[it.test.ID] = it.steps
	}
	byName := map[string]int{}
	viaNames = slices.DeleteFunc(viaNames, func(t nameTest) bool {
		s, ok := chain[t.test.ID]
		if !ok || t.steps < s {
			byName[t.test.ID] = t.steps
		}
		return ok && s <= t.steps
	})
	indirect = slices.DeleteFunc(indirect, func(it indirectTest) bool { _, ok := byName[it.test.ID]; return ok })
	return walks, tests, asValue, indirect, viaNames
}

// isTRMeasurement TR ölçüm programının bildirimi mi (tools/bench/tr/fib.tr): zincirde test gibi durak, ölçüm grubunda.
func isTRMeasurement(n *Node) bool {
	return n != nil && strings.HasPrefix(n.ID, "tr:") && isMeasurement(n)
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
	fmt.Fprintf(w, "  <- tests, indirect (%d, through calls or loaded files, up to %d steps and %d package boundary):\n", len(list), testMaxSteps, testMaxHops)
	forEachLimited(w, list, limit, "    ", moreHint, func(it indirectTest) string {
		return fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name)
	})
}

// printGuardedRules testlerin koruduğu kuralları (rule düğümleri) yazar.
func (ix *index) printGuardedRules(w io.Writer, own []*Node, tests []*Edge, indirect []indirectTest, limit int) {
	froms := map[string]bool{}
	for _, t := range own {
		froms[t.ID] = true
	}
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
	var lines []string
	for _, it := range weakInd {
		lines = append(lines, fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name))
	}
	for _, t := range weakNames {
		lines = append(lines, fmt.Sprintf("%s  via %s, %d steps  [INFERRED]", describe(t.test), t.name.Name, t.steps))
	}
	printWeak(w, lines, limit)
}

// printWeak "weak evidence" grubunu yazar; bildirim ve dosya düzeyi aynı başlığı kullanır.
func printWeak(w io.Writer, lines []string, limit int) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- weak evidence (%d; chains longer than %d steps, often through shared machinery: read with care):\n", len(lines), weakSteps)
	printLimited(w, lines, limit, "    ")
}

// testsListed test gruplarında yazılan testlerdir (atlama notu ve başka yapılandırma grubu bunlara bakar).
func (ix *index) testsListed(own []*Node, about []aboutTest, tests, walks []*Edge, asValue, indirect []indirectTest, viaNames []nameTest) []*Node {
	all := slices.Clone(own)
	for _, a := range about {
		all = append(all, a.test)
	}
	for _, e := range append(append([]*Edge(nil), tests...), walks...) {
		all = append(all, ix.byID[e.From])
	}
	for _, it := range append(append([]indirectTest(nil), asValue...), indirect...) {
		all = append(all, it.test)
	}
	for _, t := range viaNames {
		all = append(all, t.test)
	}
	return all
}

// printAllSkipNote listelenen testlerin hepsi bir atlama koşuluna bağlıysa bunu tek satırla söyler: CI'da hiçbiri
// koşmayabilir (deneme 9 H2: gpu.backend.ltMul'un testleri kitaplık yoksa atlanıyordu, okuyan CI'ın koruduğunu sanıyordu).
func printAllSkipNote(w io.Writer, all []*Node) {
	if len(all) == 0 || slices.ContainsFunc(all, func(t *Node) bool { return t == nil || t.Skip == "" || ciRunsIt(t.Skip) }) {
		return // koşulsuz ya da yalnızca CI'ın bir kipinde zaten koşan (testing.Short(), raceEnabled) bir test var
	}
	fmt.Fprintln(w, "  note: every test above skips under a condition ([skips when: ...]); CI may run none of them: check where the condition holds (real GPU, library, OS)")
}
