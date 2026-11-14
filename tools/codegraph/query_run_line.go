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

// Bu dosya tests-for'un dar koşu satırlarını kurar: yazarken koşulacak kısa komut (codegraph 7, Dilim A). Satır yalnız
// yakın testleri koşar: dosyanın paketi (ağır pakette adla), dosyanın kendi testleri, onu konu edinenler, Go dışı dosyada
// bütün listeyi gezenler ve başka paketlerdeki doğrudan testler (en çok narrowHubLimit). Taramalar, denetleyici
// koşucuları, dolaylı zincirler, zayıf kanıt ve alt süreç testleri satıra girmez; ön denetimin seçiminden kalanların
// sayısı tek satırla söylenir, onları commit öncesi denetim ve G4 koşar. Neden: satır ön denetimin payı olunca her G4
// düşüşüyle büyüdü (`checker.go`'da 203 ad), ajan ya yüzlerce uzak testi bekledi ya da satırı atladı.

// commandLineLimit tools/full_check.sh'in -run listesi için kullandığı sınırdır: Windows'ta komut satırı ~32 000
// karakterdir, ön denetim 20 000'i aşan listede paketleri bütün olarak koşar; koşu satırı da öyle yapar.
const commandLineLimit = 20000

// narrowHubLimit başka paketlerdeki doğrudan testlerin dar satıra girebileceği en çok sayıdır (A2, K6): `vm.go`'nun 291,
// `checker.go`'nun 175 doğrudan testi var; örnekte 11 ile 31 arasındaki her sınır aynı sonucu veriyor.
const narrowHubLimit = 20

// runPlan dosyanın koşu satırlarıdır. İki komut gerekir: -run bir `go test`'in bütün paketlerine uygulanır, tek komut
// dosyanın kendi paketini de süzerdi.
type runPlan struct {
	pkg     string   // dosyanın paketi, bütün olarak (-run'sız): grafın göremediği aynı paket testleri de koşar
	others  []string // dar satırın başka paketlerdeki testlerinin paketleri (pkg hariç)
	names   []string // onların adları (-run)
	hub     int      // narrowHubLimit'i aştığı için satıra girmeyen, başka paketlerdeki doğrudan test sayısı
	golden  []string // seçilen altın dosya testleri (-update ile yeniden yazılır)
	whole   bool     // -run listesi commandLineLimit'i aşıyor: others'ın ağır olmayanları bütün olarak koşar
	heavy   []string // whole iken others'ın ağır paketleri (heavyTestPackages): yalnız sığan adlarla koşar
	fit     []string // onların satıra sığan adları
	cut     int      // satıra sığmayan ad sayısı
	pkgFit  []string // pkg ağırsa (heavyTestPackages) bütün yerine koşan adları: o paketteki yakın testler
	pkgCut  int      // onlardan satıra sığmayanların sayısı
	trFiles []string // kendi ve konu edinen TR test dosyaları (`go run ./cmd/trlang test ...`)
	rest    []goTest // ön denetimin seçiminden komutların koşmadıkları
	// selfTests iki klasörün dışındaki dosyada komutlardan ve rest'ten çıkan codegraph testleridir (self_tests.go, S3).
	selfTests []goTest
}

// runPlanFor bir Go dosyasının dar koşu satırlarıdır: doğrudan testleri, fileTests'in 0 adımlı testleridir ("tests" grubu).
func (ix *index) runPlanFor(file *Node, ts *touchSet, s fileTestSet) runPlan {
	var direct []*Node
	for _, id := range s.others {
		if s.all[id] == 0 {
			direct = append(direct, ix.byID[id])
		}
	}
	return ix.runPlanWith(file, ts, s, direct)
}

// runPlanWith dar koşu satırlarını s'nin kendi ve konu edinen testlerinden ve direct'ten (doğrudan testler: Go dosyasında
// kullananlar, Go dışı dosyada okuyanlar) kurar. Kalan sayısı ön denetimin seçiminden (goTestSelection) gelir.
func (ix *index) runPlanWith(file *Node, ts *touchSet, s fileTestSet, direct []*Node) runPlan {
	var p runPlan
	if file.Kind == kindFile && strings.HasSuffix(file.File, ".go") {
		p.pkg = path.Dir(file.File)
	}
	p.trFiles = trRunFiles(s, direct)
	extra, golden, own, hub := ix.collectPlanTests(p.pkg, s, direct)
	p.hub = hub
	selfOut := !inSelfTestFolders(file.File)
	if selfOut {
		for t := range extra {
			if ix.isCodegraphSelfTest(t) {
				delete(extra, t)
				delete(golden, t.name)
				p.selfTests = append(p.selfTests, t)
			}
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
	p.rest = ix.unrunSelection(file, ts, s, p)
	if selfOut {
		var out []goTest
		p.rest, out = ix.withoutSelfTests(p.rest)
		for _, t := range out {
			if !slices.Contains(p.selfTests, t) {
				p.selfTests = append(p.selfTests, t)
			}
		}
	}
	return p
}

// trRunFiles koşu satırının `trlang test` hedefleridir: s.own, s.about ve direct içindeki kindTRTest dosyaları, o sırayla.
func trRunFiles(s fileTestSet, direct []*Node) []string {
	var out []string
	add := func(n *Node) {
		if n != nil && n.Kind == kindTRTest && n.File != "" && !slices.Contains(out, n.File) {
			out = append(out, n.File)
		}
	}
	for _, n := range s.own {
		add(n)
	}
	for _, a := range s.about {
		add(a.test)
	}
	for _, n := range direct {
		add(n)
	}
	return out
}

// collectPlanTests dar satırın testlerini toplar: dosyanın paketi dışındakiler (extra), altın dosya testleri (golden),
// dosyanın kendi paketindekiler (own; paket ağırsa adla koşar). Kendi ve konu edinen testler hep girer; başka paketlerdeki
// doğrudan testler narrowHubLimit'i aşarsa girmez, sayıları hub'dır. Her testin fikstür koşucusu da girer.
func (ix *index) collectPlanTests(pkg string, s fileTestSet, direct []*Node) (extra map[goTest]bool, golden, own map[string]bool, hub int) {
	extra, golden, own = map[goTest]bool{}, map[string]bool{}, map[string]bool{}
	add := func(n *Node) {
		if n == nil {
			return
		}
		for _, r := range append([]*Node{n}, ix.fixtureRunners(n)...) {
			t, ok := goTestOf(r)
			switch {
			case !ok:
			case t.dir == pkg:
				own[t.name] = true
			default:
				extra[t] = true
				if strings.HasSuffix(t.name, "Golden") {
					golden[t.name] = true
				}
			}
		}
	}
	for _, t := range s.own {
		add(t)
	}
	for _, a := range s.about {
		add(a.test)
	}
	far := map[goTest]bool{}
	for _, n := range direct {
		if t, ok := goTestOf(n); ok && t.dir != pkg && !extra[t] {
			far[t] = true
		}
	}
	if hub = len(far); hub > narrowHubLimit {
		direct = slices.DeleteFunc(slices.Clone(direct), func(n *Node) bool { t, ok := goTestOf(n); return !ok || t.dir != pkg })
	} else {
		hub = 0
	}
	for _, n := range direct {
		add(n)
	}
	return extra, golden, own, hub
}

// unrunSelection ön denetimin seçiminden (goTestSelection) koşu planının çalıştırmadığı testleri sırasıyla verir.
func (ix *index) unrunSelection(file *Node, ts *touchSet, s fileTestSet, p runPlan) []goTest {
	sel := newTouchSet()
	sel.nodes, sel.files = ts.nodes, map[string]bool{file.File: true}
	selection := ix.goTestSelection
	if s.all != nil { // fileTests'ten: all, testsToRun(ts)'in anahtarlarıyla dosyanın kendi testlerinin birleşimi; dosyanınkileri seçim zaten ekler
		selection = func(sel *touchSet) []goTest { return ix.goTestSelectionFrom(sel, s.all, s.subprocess) }
	}
	var rest []goTest
	for _, t := range selection(sel) {
		if !p.runs(t) {
			rest = append(rest, t)
		}
	}
	return rest
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

// nodeRunPlan Go dışı bir dosya ya da belge düğümünün (.tr, .md, veri, betik) dar koşu satırlarını kendi testlerinden,
// bütün listeyi gezenlerden ve konu edinenlerden kurar; doğrudan okuyanlar (readers) narrowHubLimit'e kadar girer.
func (ix *index) nodeRunPlan(n *Node, t nodeTests, readers []*Edge) runPlan {
	own := slices.Clone(t.own)
	for _, e := range t.walks {
		own = append(own, ix.byID[e.From])
	}
	var direct []*Node
	for _, e := range readers {
		switch {
		case n.Kind == kindDoc:
			direct = append(direct, ix.byID[e.From])
		case n.Kind == kindFile && isTestCode(ix.byID[e.From]) && (isTestFile(e.File) || strings.HasSuffix(e.File, "_test.tr")):
			direct = append(direct, ix.byID[e.From])
		}
	}
	// Modülü yükleyen TR test dosyası (1 adım, zincir testin kendi dosyasından): modülün test dosyası gibidir
	// (`lib/whisper.tr` → `tests/whisper_model_test.tr`); satıra tek `trlang test` hedefi olarak girer.
	for _, it := range t.indirect {
		if it.steps == 1 && it.test.Kind == kindTRTest && it.via != nil && it.via.File == it.test.File {
			direct = append(direct, it.test)
		}
	}
	return ix.runPlanWith(n, newTouchSet(), fileTestSet{own: own, about: t.about}, direct)
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
func printRunPlan(w io.Writer, p runPlan) { printRunPlanText(w, p, false) }

// printRunPlanText printRunPlan'dır; short tests-for'un bayraksız cevabıdır: orada grup yoktur, gruplara gönderen
// satırlar --list'e gönderir (codegraph dilim 6, S8).
func printRunPlanText(w io.Writer, p runPlan, short bool) {
	inGroups, amongGroups := "they are in the groups below", "(among the groups below)"
	if short {
		inGroups, amongGroups = "--list names them", "(--list names them)"
	}
	cmds := p.commands()
	for _, c := range cmds {
		fmt.Fprintln(w, "run: "+c)
	}
	if isHeavyPackage(p.pkg) && len(cmds) > 0 { // komut yoksa paketin neden bütün koşmadığını söylemeye gerek yok
		fmt.Fprintf(w, "  ./%s/ is a heavy package: it does not run whole here, the run line names %s of it (the full check runs it whole)\n",
			p.pkg, countOf(len(p.pkgFit), "near test"))
	}
	if p.pkgCut > 0 {
		fmt.Fprintf(w, "  %s of ./%s/ did not fit the command line; %s\n", countOf(p.pkgCut, "more near test"), p.pkg, inGroups)
	}
	if len(p.golden) > 0 {
		fmt.Fprintf(w, "  golden files (%s): if they change, rewrite with -update and read the diff\n", strings.Join(p.golden, ", "))
	}
	if p.whole {
		fmt.Fprintf(w, "  the second command runs its packages whole: its -run list is longer than %d characters (the Windows command line)\n", commandLineLimit)
	}
	if p.cut > 0 {
		fmt.Fprintf(w, "  %s of %s did not fit the command line (heavy packages do not run whole here); %s\n",
			countOf(p.cut, "more near test"), strings.Join(p.heavy, ", "), inGroups)
	}
	// Tek sayı satırı (A3, S2): kalanlar ayrı listelenmez, gruplardadırlar; codegraph'ın kendi testleri de sayıya girer.
	switch left := len(p.rest) + len(p.selfTests); {
	case left > 0:
		fmt.Fprintf(w, "  narrow: these lines run the near tests; %d more are left to the commit check and G4 %s\n", left, amongGroups)
	case len(cmds) > 0:
		fmt.Fprintln(w, "  these commands run every Go test the pre-check selects for this file")
	}
	// Hub satırı (A2, S3) sayı satırının içini söyler, ondan sonra gelir: önce ve ayrı bir sayı gibi yazılınca iki satır
	// toplanabiliyordu (`lib/layers.tr`'de 22 ve 119; 22, 119'un içinde; DENETİM 750 N1, codegraph 7 B6).
	if p.hub > 0 {
		fmt.Fprintf(w, "  narrow: %d of them are direct tests in other packages (more than %d stay off the run line)\n", p.hub, narrowHubLimit)
	}
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
