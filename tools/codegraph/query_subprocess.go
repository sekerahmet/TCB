package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Bu dosya tests-for'un alt süreç testlerini ve test edilen ikiliye bağlanan paketleri yazar (subprocess.go'nun
// builds kenarlarından; bağımsız denetim Y5).

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

const (
	branchTested     = "tested"
	branchSubprocess = "subprocess"
	branchRule       = "rule"
	branchMeasured   = "measured"
	branchWeak       = "weak"
	branchExam       = "exam"
	branchUntested   = "untested"
)

type nodeTests struct {
	branch                 string
	own                    []*Node
	about                  []aboutTest
	walks, tests, allTests []*Edge
	asValue, indirect      []indirectTest
	allIndirect            []indirectTest
	viaNames               []nameTest
	listed                 []*Node
	measured               []*Edge
	indirectMeasured       []indirectTest
	subprocess             map[string][]*Node
	cites                  []*Edge
	exam                   *Edge
	rules                  []*Node
	scans                  []scanHit
	linked                 []string
	linkedTotal            int
	beyond                 beyondTests // sınır ötesi testler (query_beyond.go); beyondDone değilse kurulmadı
	beyondDone             bool
}

// testsForNode n'yi sınayan test gruplarını toplar ve yedi koldan hangisine düştüğünü belirler.
func (ix *index) testsForNode(n *Node) nodeTests {
	own := ix.trFileOwnTests(n)
	tests, measured := ix.directTestEdges(n)
	walks := ix.walkEdges(n)
	asValue, _ := splitMeasurements(ix.valueTests(n), func(t indirectTest) *Node { return t.test })
	indirect, indirectMeasured := splitMeasurements(ix.indirectTests(n), func(t indirectTest) *Node { return t.test })
	viaNames, _ := splitMeasurements(ix.testsThroughNames(n), func(t nameTest) *Node { return t.test })
	subprocess := ix.subprocessTests(n)
	allTests, allIndirect := tests, indirect // koruduğu kurallar bütün testlerden
	// Tek sıralama (deneme 8 S1 ve M): önce onu konu edinen testler, sonra doğrudan kullananlar, sonra kısa ve aynı paket
	// zincirleri, en sonda zayıf kanıt; her test bir kez, en güçlü kanıtının grubunda. Adaylar bütün gruplardır.
	about := ix.aboutTests(n, append(ix.testsListed(nil, nil, tests, walks, asValue, indirect, viaNames), ix.fixtureTests(n)...))
	walks, tests, asValue, indirect, viaNames = keepStrongest(own, about, walks, tests, asValue, indirect, viaNames)
	listed := ix.testsListed(own, about, allTests, walks, asValue, indirect, viaNames)
	cites := ix.citingTests(n, listed)
	exam := ix.examEdge(listed, nil, n)
	rules := ix.docRules(n)
	var scans []scanHit
	if n.Kind == kindFile || n.Kind == kindDoc {
		scans = ix.scansCovering(n.File)
	}
	t := nodeTests{
		own: own, about: about, walks: walks, tests: tests, allTests: allTests,
		asValue: asValue, indirect: indirect, allIndirect: allIndirect, viaNames: viaNames,
		listed: listed, measured: measured, indirectMeasured: indirectMeasured,
		subprocess: subprocess, cites: cites, exam: exam, rules: rules, scans: scans,
	}
	found := len(own) + len(about) + len(tests) + len(walks) + len(asValue) + len(indirect) + len(viaNames)
	if found == 0 { // kademe 1: sınırın ötesinde klasör içi zincirle ulaşan test de "testli" demektir
		t.beyond, t.beyondDone = ix.beyondLimit(n, listed), true
		found = t.beyond.total()
	}
	switch {
	case found > 0:
		t.branch = branchTested
	case len(subprocess) > 0:
		t.branch = branchSubprocess
	case len(cites)+len(rules)+len(scans) > 0 && len(measured)+len(indirectMeasured) == 0:
		t.branch = branchRule
	case len(measured)+len(indirectMeasured) > 0:
		t.branch = branchMeasured
	default:
		if t.linked, t.linkedTotal = ix.linkedBinaries(n); len(t.linked) > 0 {
			t.branch = branchWeak
		} else if exam != nil {
			t.branch = branchExam
		} else {
			t.branch = branchUntested
		}
	}
	return t
}

// linkedBinaries n'nin paketi (geçişli içe aktarmayla) bir testin derleyip koştuğu ana pakete bağlıysa bağlandığı
// paketlerin özetlerini ve toplam test sayısını verir (DENETİM doğrulaması Y5: codeindex.LoadCached, e2e'deki --no-index
// testine rağmen "no tests found" diyordu). Paketin kendisi derleniyorsa satır zaten "covered by subprocess tests"tir;
// burada yalnızca bağlanan paketler.
func (ix *index) linkedBinaries(n *Node) ([]string, int) {
	pkg := ix.goPackageOf(n)
	if pkg == "" {
		return nil, 0
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
	sort.Strings(parts)
	return parts, total
}

func printLinkedBinaries(w io.Writer, parts []string, total int) {
	fmt.Fprintln(w, "  no tests traced to it")
	fmt.Fprintf(w, "  <- weak evidence (%d): linked into the binary of %s; a subprocess test may reach this code, which one is not traced\n", total, strings.Join(parts, ", "))
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

var untestedBranches = []string{branchTested, branchSubprocess, branchRule, branchMeasured, branchWeak, branchExam, branchUntested}

type untestedEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// runUntested test dışı her Go işlevini ve metodunu testsForNode'un yedi koluna göre gruplar.
func (ix *index) runUntested(o *queryOpts) int {
	counts := map[string]int{}
	for _, b := range untestedBranches {
		counts[b] = 0
	}
	groups := map[string][]*Node{}
	total := 0
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		if (n.Kind != kindFunc && n.Kind != kindMethod) || inTestFile(n) || !underIn(n.File, o.in) {
			continue
		}
		total++
		b := ix.testsForNode(n).branch
		counts[b]++
		if b != branchTested {
			groups[b] = append(groups[b], n)
		}
	}
	if o.asJSON {
		jGroups := map[string][]untestedEntry{}
		for _, b := range untestedBranches[1:] {
			for _, n := range groups[b] {
				jGroups[b] = append(jGroups[b], untestedEntry{Name: n.Name, Kind: n.Kind, File: n.File, Line: n.Line})
			}
		}
		return writeJSON(o.stdout, o.stderr, map[string]any{
			"schema": "codegraph-untested/1", "total": total, "counts": counts, "groups": jGroups,
		})
	}
	fmt.Fprintf(o.stdout, "%d non-test Go functions and methods: %d tested, %d subprocess, %d rule, %d measured, %d weak, %d exam, %d untested\n",
		total, counts[branchTested], counts[branchSubprocess], counts[branchRule], counts[branchMeasured], counts[branchWeak], counts[branchExam], counts[branchUntested])
	for _, b := range append([]string{branchUntested}, untestedBranches[1:len(untestedBranches)-1]...) {
		if len(groups[b]) > 0 {
			fmt.Fprintf(o.stdout, "\n%s (%d):\n", b, len(groups[b]))
			forEachLimited(o.stdout, groups[b], o.limit, "  ", moreHint, describe)
		}
	}
	return exitOK
}
