package main

// repo_scans.go: tam denetim betiğinin (tools/full_check.sh) depo taramaları listeleri, repo_scans (testler) ve
// repo_scan_pkgs (paketler), graftaki tarama testleriyle karşılaştırılır. `changed --go-tests` seçimi yazdıktan sonra
// karşılaştırır: listeye yazılmayan yeni bir tarama testi commit kipinde commit'ten önce adıyla ve eklenecek satırla
// söylenir, kip 5'te (exitScanList) durur. Neden: 2026-11-14 – 2026-11-18'de G4'ü dört kez aynı eksik düşürdü
// (codegraph tarama listesi tasarımı). TestFullCheckRepoScansMatchesScanNodes aynı işlevlerle G4'te korur.

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// repoScanScript listelerin dosyasıdır (kökten).
const repoScanScript = "tools/full_check.sh"

// repoScanLists betikten okunan listelerdir.
type repoScanLists struct {
	tests, pkgs []string // repo_scans ve repo_scan_pkgs, betikteki sırayla
	layout      []string // düzen kuralları adımının bütün koşan paketleri (./x, ./x/...): onların taramaları listeye girmez
}

var (
	// layoutLine düzen kuralları adımının go test satırıdır.
	layoutLine = regexp.MustCompile(`(?m)^\s*go test [^\n]*-short(?: -count=1)? ((?:\./\S+\s*)+)[^\n]*\n\s*\[ "\$\{PIPESTATUS\[0\]\}" -eq 0 \] \|\| fail "düzen kuralları"`)
	// repoScanStep depo taramaları adımının satırıdır: listedeki testleri listedeki paketlerde koşar.
	repoScanStep = regexp.MustCompile(`(?s)repo_scans\[@\].*"\$\{repo_scan_pkgs\[@\]\}"[^\n]*\n\s*\[ "\$\{PIPESTATUS\[0\]\}" -eq 0 \] \|\| fail "depo taramaları"`)
)

// staleRepoScans bayat grafta karşılaştırmanın atlandığını söyler (söz S6): liste diskteki güncel hâldir, graf eskidir;
// karşılaştırılsa doğru yazılmış yeni test fazla, silinen test eksik görünürdü.
const staleRepoScans = "codegraph: warning: the graph is older than the tree, so the repo_scans list of tools/full_check.sh was not checked; G4's TestFullCheckRepoScansMatchesScanNodes still checks it"

// readRepoScanLists root'taki betiğin listelerini okur. Betik yoksa nil, nil (fikstür depoları). Betik var ama
// okunamıyorsa ya da bir parçası bulunamıyorsa hata söz S5'in nedenidir.
func readRepoScanLists(root string) (*repoScanLists, error) {
	data, err := os.ReadFile(filepath.Join(root, repoScanScript))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read: %v", err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	l := &repoScanLists{}
	var ok bool
	if l.tests, ok = bashArray(script, "repo_scans"); !ok {
		return nil, errors.New("array repo_scans not found")
	}
	if l.pkgs, ok = bashArray(script, "repo_scan_pkgs"); !ok {
		return nil, errors.New("array repo_scan_pkgs not found")
	}
	m := layoutLine.FindStringSubmatch(script)
	if m == nil {
		return nil, errors.New(`the layout-rules go test line (… || fail "düzen kuralları") not found`)
	}
	l.layout = strings.Fields(m[1])
	if !repoScanStep.MatchString(script) {
		return nil, errors.New(`the depo taramaları step does not run ${repo_scans[@]} across "${repo_scan_pkgs[@]}"`)
	}
	return l, nil
}

// bashArray `ad=( ... )` dizisinin sözcükleridir (# yorumları atılır).
func bashArray(script, name string) ([]string, bool) {
	m := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `=\(\s*(.*?)\s*\)`).FindStringSubmatch(script)
	if m == nil {
		return nil, false
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		out = append(out, strings.Fields(line)...)
	}
	return out, true
}

// layoutRuns dir paketini düzen kuralları adımı bütünüyle koşuyor mu.
func (l *repoScanLists) layoutRuns(dir string) bool {
	for _, p := range l.layout {
		clean := strings.TrimPrefix(p, "./")
		if prefix, sub := strings.CutSuffix(clean, "..."); sub {
			if strings.HasPrefix(dir+"/", prefix) {
				return true
			}
		} else if strings.TrimSuffix(clean, "/") == dir {
			return true
		}
	}
	return false
}

// repoScanWant graftan listelerin olması gerekeni verir: scans kenarlarının Test* testleri (ad -> ilk düğümü) ve
// paketleri ("./klasör" -> o paketin ilk tarama testi); düzen adımının koştuğu paketler ve Fuzz* hedefleri dışarıda.
// "İlk" dosya ve satır sırasıdır (ileti her koşuda aynı yeri gösterir).
func repoScanWant(g *Graph, l *repoScanLists) (tests, pkgs map[string]*Node) {
	byID := map[string]*Node{}
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	tests, pkgs = map[string]*Node{}, map[string]*Node{}
	first := func(m map[string]*Node, k string, n *Node) {
		if o := m[k]; o == nil || n.File < o.File || n.File == o.File && n.Line < o.Line {
			m[k] = n
		}
	}
	for _, e := range g.Edges {
		sc, tn := byID[e.To], byID[e.From]
		if e.Kind != edgeScans || sc == nil || sc.Kind != kindScan || tn == nil {
			continue
		}
		name := tn.Name[strings.LastIndex(tn.Name, ".")+1:]
		if dir := path.Dir(tn.File); strings.HasPrefix(name, "Test") && !l.layoutRuns(dir) {
			first(tests, name, tn)
			first(pkgs, "./"+dir, tn)
		}
	}
	return tests, pkgs
}

// checkRepoScans `changed --go-tests`'in son adımıdır: listeler graftan ayrışırsa her eksik ve fazla ad için söz S1–S3,
// sonda S4 yazılır ve exitScanList döner. Betik yoksa sessizdir; bayat grafta S6 ile atlanır (seçimin kodu kalır);
// betik okunamazsa S5 ve exitFail.
func checkRepoScans(root string, g *Graph, stale bool, w io.Writer) int {
	if root == "" {
		return exitOK
	}
	l, err := readRepoScanLists(root)
	switch {
	case l == nil && err == nil:
		return exitOK
	case stale:
		fmt.Fprintln(w, staleRepoScans)
		return exitOK
	case err != nil:
		fmt.Fprintf(w, "codegraph: %s: cannot check the repo_scans list: %v\n", repoScanScript, err)
		return exitFail
	}
	tests, pkgs := repoScanWant(g, l)
	n := printListGaps(w, "repo_scans", l.tests, tests, func(name string, tn *Node) string {
		return fmt.Sprintf("%s, a test that scans the repository (%s:%d)", name, tn.File, tn.Line)
	})
	n += printListGaps(w, "repo_scan_pkgs", l.pkgs, pkgs, func(pkg string, tn *Node) string {
		return fmt.Sprintf("%s, the package of %s", pkg, tn.Name[strings.LastIndex(tn.Name, ".")+1:])
	})
	if n == 0 {
		return exitOK
	}
	fmt.Fprintln(w, "codegraph: the commit check stops here: G4's TestFullCheckRepoScansMatchesScanNodes fails on the same lists")
	return exitScanList
}

// printListGaps bir listenin eksiklerini (S1, S2: eklenecek satır ve yeri) ve fazlalarını (S3) yazar, sayısını verir.
// Yer listenin ad sırasıdır: eksiklerin hepsi eklenmiş sayılarak yeni addan önce gelen ad.
func printListGaps(w io.Writer, list string, got []string, want map[string]*Node, what func(string, *Node) string) int {
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
	}
	var missing []string
	for s := range want {
		if !have[s] {
			missing = append(missing, s)
		}
	}
	sort.Strings(missing)
	listed := slices.Sorted(maps.Keys(have))
	merged := slices.Sorted(slices.Values(append(slices.Clone(listed), missing...)))
	for _, s := range missing {
		where := "add this line as the first line of " + list + ":"
		if i := sort.SearchStrings(merged, s); i > 0 {
			where = fmt.Sprintf("add this line to %s after the line %s:", list, merged[i-1])
		}
		fmt.Fprintf(w, "codegraph: %s: the %s list lacks %s; %s\n\t%s\n", repoScanScript, list, what(s, want[s]), where, s)
	}
	extra := 0
	for _, s := range listed {
		if want[s] == nil {
			fmt.Fprintf(w, "codegraph: %s: the %s list names %s, which scans no repository files now; remove its line\n", repoScanScript, list, s)
			extra++
		}
	}
	return len(missing) + extra
}
