package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
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
	pkg     string   // dosyanın paketi, bütün olarak (-run'sız): grafın göremediği aynı paket testleri de koşar
	others  []string // dosyayı tarayan depo kurallarının ve konu edinen testlerin paketleri (pkg hariç)
	names   []string // onların adları (-run)
	golden  []string // seçilen altın dosya testleri (-update ile yeniden yazılır)
	whole   bool     // -run listesi commandLineLimit'i aşıyor: others'ın ağır olmayanları bütün olarak koşar
	heavy   []string // whole iken others'ın ağır paketleri (heavyTestPackages): yalnız sığan adlarla koşar
	fit     []string // onların satıra sığan adları
	cut     int      // satıra sığmayan ad sayısı
	pkgFit  []string // pkg ağırsa (heavyTestPackages) bütün yerine koşan adları: kendi testleri ve cevapta adı geçenler
	pkgCut  int      // onlardan satıra sığmayanların sayısı
	trFiles []string // .tr dosyasında cevapta adı geçen TR test dosyaları (`go run ./cmd/trlang test ...`)
	rest    []goTest // ön denetimin seçiminden komutların koşmadıkları
}

// runPlanFor koşu satırlarını ön denetimin seçiminden (goTestSelection, `changed --go-tests`) kurar: dosyanın paketi
// bütün olarak; dosyayı tarayan bütün depo kuralları (scansCovering); dosyanın bir bildirimini konu edinen (about) ve
// tip denetleyicisi dosyalarında denetleyiciden program geçiren (checkerRunnerTests) Go testleri. Seçimin geri kalanı rest'tir.
func (ix *index) runPlanFor(file *Node, ts *touchSet, s fileTestSet) runPlan {
	return ix.runPlanWith(file, ts, s, listedTests{})
}

// runPlanWith runPlanFor'dur; cevapta adı geçen testler (listed) de adlandırılır (query_run_listed.go).
func (ix *index) runPlanWith(file *Node, ts *touchSet, s fileTestSet, listed listedTests) runPlan {
	var p runPlan
	switch {
	case file.Kind == kindFile && strings.HasSuffix(file.File, ".go"):
		p.pkg = path.Dir(file.File)
	case file.Kind == kindFile && strings.HasSuffix(file.File, ".tr"):
		p.trFiles = listed.trFiles // .tr modülünün cevapta adı geçen TR test dosyaları: tek `trlang test` satırı
	}
	extra, golden := map[goTest]bool{}, map[string]bool{}
	add := func(n *Node) {
		if n == nil {
			return
		}
		if t, ok := goTestOf(n); ok && t.dir != p.pkg {
			extra[t] = true
			if strings.HasSuffix(t.name, "Golden") {
				golden[t.name] = true
			}
		}
		for _, r := range ix.fixtureRunners(n) {
			if t, ok := goTestOf(r); ok && t.dir != p.pkg {
				extra[t] = true
			}
		}
	}
	for _, h := range ix.scansCovering(file.File) {
		for _, t := range h.tests {
			add(t)
		}
	}
	for _, t := range s.own {
		add(t)
	}
	for _, a := range s.about {
		add(a.test)
	}
	for _, t := range ix.checkerRunnerTests(file, s.all) {
		add(t)
	}
	own := map[string]bool{} // ağır pakette dosyanın paketinin adlandırılan testleri: kendi testleri ve cevapta adı geçenler
	for _, n := range s.own {
		if t, ok := goTestOf(n); ok && t.dir == p.pkg {
			own[t.name] = true
		}
	}
	for _, t := range listed.goTests {
		if t.dir != p.pkg {
			extra[t] = true
		} else {
			own[t.name] = true
		}
	}
	if isHeavyPackage(p.pkg) {
		p.pkgFit, p.pkgCut = p.fitNames([]string{p.pkg}, sortedKeys(own))
	}
	pkgs, names, heavyNames := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for t := range extra {
		pkgs[t.dir], names[t.name] = true, true
		if isHeavyPackage(t.dir) {
			heavyNames[t.name] = true
		}
	}
	p.others, p.names, p.golden = sortedKeys(pkgs), sortedKeys(names), sortedKeys(golden)
	if p.whole = len(p.command(p.others, p.names)) > commandLineLimit; p.whole {
		p.heavy = slices.DeleteFunc(slices.Clone(p.others), func(d string) bool { return !isHeavyPackage(d) })
		p.fit, p.cut = p.fitNames(p.heavy, sortedKeys(heavyNames))
	}
	sel := newTouchSet()
	sel.nodes, sel.files = ts.nodes, map[string]bool{file.File: true}
	for _, t := range ix.goTestSelection(sel) {
		if !p.runs(t) {
			p.rest = append(p.rest, t)
		}
	}
	return p
}

// runs koşu satırlarından biri t'yi koşuyor mu.
func (p runPlan) runs(t goTest) bool {
	switch {
	case t.dir == p.pkg:
		return !isHeavyPackage(p.pkg) || slices.Contains(p.pkgFit, t.name)
	case !slices.Contains(p.others, t.dir):
		return false
	case !p.whole:
		return slices.Contains(p.names, t.name)
	case isHeavyPackage(t.dir):
		return slices.Contains(p.fit, t.name)
	}
	return true
}

// docRunPlan bir belge dosyasını (.md) adıyla okuyan ve tarayan testlerden koşu satırını kurar.
func (ix *index) docRunPlan(doc *Node, t nodeTests, byName []*Edge, listed listedTests) runPlan {
	own := make([]*Node, 0, len(byName))
	for _, e := range byName {
		own = append(own, ix.byID[e.From])
	}
	return ix.runPlanWith(doc, newTouchSet(), fileTestSet{own: own, about: t.about}, listed)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkerRunnerTests denetleyici paketindeki bir dosyanın seçiminde (all) yer alan ve TR programlarını
// Checker.CheckProgram üzerinden geçiren Go testlerini verir.
func (ix *index) checkerRunnerTests(file *Node, all map[string]int) []*Node {
	if path.Dir(file.File) != checkerRelPkg {
		return nil
	}
	checkID := memberID(ix.g.Module+"/"+checkerRelPkg, "Checker", "CheckProgram")
	var out []*Node
	for _, e := range ix.in[checkID] {
		if e.Kind == edgeTests && e.Confidence == inferred {
			if _, ok := all[e.From]; ok {
				out = append(out, ix.byID[e.From])
			}
		}
	}
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
	var out []string
	switch {
	case p.pkg == "":
	case !isHeavyPackage(p.pkg):
		out = append(out, p.command([]string{p.pkg}, nil))
	case len(p.pkgFit) > 0: // ağır paket -run'sız önerilmez: kanca durdurur, paketin bütününü tam denetim koşar
		out = append(out, p.command([]string{p.pkg}, p.pkgFit))
	}
	switch {
	case len(p.others) == 0:
	case p.whole:
		if light := slices.DeleteFunc(slices.Clone(p.others), isHeavyPackage); len(light) > 0 {
			out = append(out, p.command(light, nil))
		}
		if len(p.heavy) > 0 && len(p.fit) > 0 {
			out = append(out, p.command(p.heavy, p.fit))
		}
	default:
		out = append(out, p.command(p.others, p.names))
	}
	if len(p.trFiles) > 0 { // trlang test birden çok hedef alır (cmd/trlang/test_command.go)
		out = append(out, "go run ./cmd/trlang test "+strings.Join(p.trFiles, " "))
	}
	return out
}

// printRunPlan koşu satırlarını girintisiz `run:` başlığıyla, dosya satırının hemen altına yazar (öğe satırı değildir:
// planlama sınavının first-item sorusu en az 4 boşlukla girintili ilk satırı okur); altında koşulmayanların sayısı.
func printRunPlan(w io.Writer, p runPlan) {
	cmds := p.commands()
	if len(cmds) == 0 && !isHeavyPackage(p.pkg) {
		return
	}
	for _, c := range cmds {
		fmt.Fprintln(w, "run: "+c)
	}
	if isHeavyPackage(p.pkg) {
		fmt.Fprintf(w, "  ./%s/ is a heavy package: it does not run whole here, the run line names %s of it (the full check runs it whole)\n",
			p.pkg, countOf(len(p.pkgFit), "listed test"))
	}
	if p.pkgCut > 0 {
		fmt.Fprintf(w, "  %s of ./%s/ did not fit the command line; they are in the groups below\n", countOf(p.pkgCut, "more listed test"), p.pkg)
	}
	if len(p.golden) > 0 {
		fmt.Fprintf(w, "  golden files (%s): if they change, rewrite with -update and read the diff\n", strings.Join(p.golden, ", "))
	}
	if p.whole {
		fmt.Fprintf(w, "  the second command runs its packages whole: its -run list is longer than %d characters (the Windows command line)\n", commandLineLimit)
	}
	if p.cut > 0 {
		fmt.Fprintf(w, "  %s of %s did not fit the command line (heavy packages do not run whole here); they are in the groups below\n",
			countOf(p.cut, "more listed test"), strings.Join(p.heavy, ", "))
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

// isUntracked dosya bir git deposunda olduğu halde git tarafından izlenmiyor mu (git ls-files --others).
// Depo taramaları (TestHeaderRulesLiveOnlyInSourceAndLexer, TestLayoutFileSize vb.) yalnız izlenen dosyaları
// gördüğü için izlenmeyen dosyada tests-for ve changed --go-tests uyarı verir.
func isUntracked(root, rel string) bool {
	if rel == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return false
	}
	out, err := gitOut(root, "--literal-pathspecs", "ls-files", "--others", "--exclude-standard", "--", rel)
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// printUntrackedWarning izlenmeyen dosya için depo taramalarının onu görmediğini ve `git add` gerektiğini yazar.
func printUntrackedWarning(w io.Writer, file string) {
	fmt.Fprintf(w, "untracked: %s is not tracked by git; repository-wide checks do not see it (run `git add %s`)\n", file, file)
}

// untrackedWorkingFiles çalışma ağacında git'in izlemediği kaynak dosyalarını (.go, .tr, go.mod) sıralı verir.
// Yalnızca grafiğe giren ve depo taramalarının denetlediği dosyalar alınır (testdata, .claude, .codegraph, eski atlanır).
func untrackedWorkingFiles(root string) []string {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}
	out, err := gitOut(root, "ls-files", "-z", "--others", "--exclude-standard", "--", ".")
	if err != nil {
		return nil
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" || (!untrackedSource(p) && !strings.EqualFold(filepath.Ext(p), ".tr")) || skippedUntrackedDir(p) {
			continue
		}
		files = append(files, p)
	}
	sort.Strings(files)
	return files
}

func skippedUntrackedDir(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts[:len(parts)-1] {
		if codeindex.DefaultSkipDirs[part] || part == "eski" {
			return true
		}
	}
	return false
}
