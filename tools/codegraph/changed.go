package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// changeOpts changed komutunun ayarlarıdır.
type changeOpts struct {
	root  string
	base  string // dal adı verildiğinde karşılaştırılan dal (git diff base...dal)
	limit int
	// goTests: yalnız koşulacak Go testleri (printGoTests); ön denetim okur.
	goTests bool
}

// runChanged ref'in (dal ya da a..b / a...b aralığı) değişikliklerini grafiğin düğümlerine eşler:
// değişen düğümler, onları sınayan testler ve aynı düğümlere dokunan öbür yerel claude/ dalları.
//
// Eşleme: grafik bugünkü çalışma ağacının grafiğidir. ref bugünkü HEAD ise fark çalışma ağacına kadardır
// (changeHunks) ve yeni dosyadaki satırlar eşlenir (yeni işlevler, commit'lenmemiş düzenlemeler dahil); değilse hunk'ların eski tarafı (birleşme tabanındaki
// satırlar) eşlenir, bu ana dal henüz kımıldamadıysa doğrudur. Yalnızca eklenen kod (eski
// tarafta karşılığı yok) "mapped to no node" altında dosya olarak sayılır.
func (ix *index) runChanged(o changeOpts, ref string, stdout, stderr io.Writer) int {
	rng, right, flag := o.base+"..."+ref, ref, "--base"
	if _, b, ok := splitRange(ref); ok {
		rng, right, flag = ref, b, ""
	} else if ref == "main" && !sameCommit(o.root, ref, "HEAD") {
		rng, right = o.base+"...HEAD", "HEAD"
	}
	hs, live, err := changeHunks(o.root, rng, flag)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	oldSide := ix.touched(hs, false)
	main, mapping := oldSide, "old-side lines (the merge base); the graph is of the checked-out tree, so this is exact only while it equals the merge base"
	var untracked []string
	if live {
		untracked = untrackedWorkingFiles(o.root)
		main, mapping = ix.touched(hs, true), "new-side lines up to the working tree, uncommitted edits included (the graph is of this very tree)"
		ix.touchUntracked(main, untracked)
	}
	for _, f := range untracked {
		printUntrackedWarning(stdout, f)
	}
	if o.goTests {
		if len(hs) == 0 && len(untracked) == 0 && flag != "" && ref != "HEAD" && ref != o.base && isAncestor(o.root, ref, o.base) {
			fmt.Fprintf(stdout, "%s is an ancestor of %s (git diff %s is empty); for a single commit's change, use `%s~1..%s`\n", ref, o.base, rng, ref, ref)
		}
		ix.printGoTests(stdout, main)
		return exitOK
	}
	fmt.Fprintf(stdout, "changed %s: %d hunks in %d files\n  mapping: %s\n", rng, len(hs), len(main.files), mapping)
	ix.printTouched(stdout, main, o.limit)
	if main == oldSide { // grafik dalın ağacı değil: dalda yeni olan bildirimleri dalın kendi dosyalarından adlandır
		printBranchDecls(stdout, branchDecls(o.root, right, main.unmappedHunks), o.limit)
	}
	ix.printTestsToRun(stdout, main, o.limit, "the changed code", "changed tests (they run themselves)")
	ix.printOverlaps(stdout, o, ref, oldSide)
	return exitOK
}

// splitRange "A..B" ve "A...B" biçimini ayırır (changed ve dups --changed aynı ayrıştırmayı kullanır); ref bir
// aralık değilse ok=false. A ve B boş olabilir.
func splitRange(ref string) (a, b string, ok bool) {
	i := strings.Index(ref, "..")
	if i < 0 {
		return "", "", false
	}
	return ref[:i], strings.TrimLeft(ref[i:], "."), true
}

// changeHunks `changed` ve `dups --changed`'in değişen satırlarının tek kaynağıdır. Aralığın sağ ucu çıkış yapılmış commit
// ise fark çalışma ağacına kadar alınır (live): commit'lenmiş, sahnelenmiş ve sahnelenmemiş değişiklik, izlenen yeni dosyalar
// dahil; `A...B` birleşme tabanından, `A..B` A'dan. Grafik ve tekrar derlemi çalışma ağacından kurulduğu için satırlar o
// zaman onlarınkiyle aynıdır; commit'ten önce koşan denetim de commit'lenmemiş kopyayı görür. Sağ uç başka bir commit ise
// fark o commit'e kadardır (live=false). Boş uç git'teki gibi HEAD'dir. flag, sol ucun geldiği bayraktır (--changed ya da --base).
func changeHunks(root, rng, flag string) (hs []hunk, live bool, err error) {
	a, b, _ := splitRange(rng)
	if b == "" {
		b = "HEAD"
	}
	from := a
	if from == "" {
		from = "HEAD"
	}
	if !sameCommit(root, b, "HEAD") {
		hs, err = diffHunks(root, rng)
		if err != nil && strings.Contains(rng, "...") {
			if _, mbErr := mergeBase(root, from, b, flag); mbErr != nil {
				return nil, false, mbErr
			}
		}
		return followFileRenames(root, b, hs), false, err
	}
	if strings.Contains(rng, "...") {
		from, err = mergeBase(root, from, "HEAD", flag)
		if err != nil {
			return nil, false, err
		}
	}
	hs, err = diffHunks(root, from)
	return hs, true, err
}

// mergeBase iki revizyonun ortak atasını verir; iki revizyon da geçerli birer commit iken ortak ata
// bulunamazsa (sığ klon ya da eski yerel dal) ortak atası bulunamayan revizyonu hangi bayrağın (flag:
// --changed ya da --base) verdiğini ve o bayrağın düzeltilmiş hâlini (bayrak yoksa konumsal aralığın
// düzeltilmiş hâlini: `changed origin/main...HEAD`) açıklar.
func mergeBase(root, from, to, flag string) (string, error) {
	mb, err := gitOut(root, "merge-base", from, to)
	if err != nil {
		if _, revErr := gitOut(root, "rev-parse", from+"^{commit}", to+"^{commit}"); revErr == nil {
			remote := "origin/" + strings.TrimPrefix(from, "origin/")
			src, fix := from, "changed "+remote+"..."+to
			if flag != "" {
				src, fix = from+" (from "+flag+")", flag+" "+remote
			}
			return "", fmt.Errorf("no common ancestor (merge base) between %s and %s (shallow clone or outdated local %s): run `git fetch` or use `%s`", src, to, from, fix)
		}
		return "", err
	}
	return strings.TrimSpace(string(mb)), nil
}

// diffHunks `git diff` parçalarını verir (yollar root'a göre); rng tek bir commit ise o commit'ten çalışma ağacına.
func diffHunks(root, rng string) ([]hunk, error) {
	out, err := gitOut(root, "diff", "-U0", "--no-color", "--no-ext-diff", "-M", "--relative", rng)
	if err != nil {
		return nil, err
	}
	return parseDiff(string(out)), nil
}

// followFileRenames geçmişteki bir commit'ten (from) bugünkü HEAD'e kadar yeniden adlandırılan dosyaların
// hunk yollarını bugünkü yola taşır (99f04289~1..99f04289'daki lib/egitim.tr -> lib/training.tr).
func followFileRenames(root, from string, hs []hunk) []hunk {
	out, err := gitOut(root, "log", "--diff-filter=R", "-M", "--name-status", "--format=", "--reverse", from+"..HEAD")
	if err != nil {
		return hs
	}
	renames := map[string]string{}
	for _, line := range nonEmptyLines(string(out)) {
		if f := strings.Split(line, "\t"); len(f) == 3 && strings.HasPrefix(f[0], "R") {
			renames[f[1]] = f[2]
		}
	}
	if len(renames) == 0 {
		return hs
	}
	for i := range hs {
		hs[i].file = finalKey(renames, hs[i].file)
	}
	return hs
}

// sameCommit iki revizyon aynı commit'e çözülüyor mu.
func sameCommit(root, a, b string) bool {
	x, err1 := gitOut(root, "rev-parse", a+"^{commit}")
	y, err2 := gitOut(root, "rev-parse", b+"^{commit}")
	return err1 == nil && err2 == nil && string(x) == string(y)
}

func isAncestor(root, ancestor, commit string) bool {
	_, err := gitOut(root, "merge-base", "--is-ancestor", ancestor, commit)
	return err == nil
}

func (ix *index) printTouched(w io.Writer, ts *touchSet, limit int) {
	groups := map[string][]string{}
	for _, n := range ts.nodes {
		g := "code"
		switch n.Kind {
		case kindMessage, kindName:
			g = "dictionary codes"
		case kindHeading:
			g = "doc sections"
		}
		groups[g] = append(groups[g], describe(n))
	}
	for id := range ts.codes {
		groups["dictionary codes"] = append(groups["dictionary codes"], id[strings.Index(id, ":")+1:]+" (added or removed; no node in the graph)")
	}
	for _, g := range []string{"code", "dictionary codes", "doc sections"} {
		list := groups[g]
		sort.Strings(list)
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s (%d):\n", g, len(list))
		printLimited(w, list, limit, "    ")
	}
	var files []string
	for f, n := range ts.unmapped {
		files = append(files, fmt.Sprintf("%s (%d hunks)", f, n))
	}
	sort.Strings(files)
	if len(files) > 0 {
		fmt.Fprintf(w, "  mapped to no node (%d files):\n", len(files))
		printLimited(w, files, limit, "    ")
	}
}

// moreHint kesilen listelerin sonundaki ipucudur.
const moreHint = " (--all lists them)"

// forEachLimited list'in ilk limit öğesini line ile yazar (limit 0: hepsi); kalanı "... N more" ile bildirir.
func forEachLimited[T any](w io.Writer, list []T, limit int, indent, hint string, line func(T) string) {
	for i, v := range list {
		if limit > 0 && i >= limit {
			fmt.Fprintf(w, "%s... %d more%s\n", indent, len(list)-limit, hint)
			return
		}
		fmt.Fprintf(w, "%s%s\n", indent, line(v))
	}
}

// printLimited satırları forEachLimited ile yazar.
func printLimited(w io.Writer, list []string, limit int, indent string) {
	forEachLimited(w, list, limit, indent, moreHint, func(s string) string { return s })
}

// printTestsToRun değişen düğümlerin (Go ve TR bildirimleri, ad kodları, dosyalar) testlerini yazar: tests-for'un topladığının
// aynısı (testDistances), doğrudan olanlar önce, sonra dolaylılar (kaç adım uzakta); her test bir kez.
// testsToRun değişen düğümleri sınayan testlerdir: test kimliği -> en kısa uzaklık (0: doğrudan; tests-for ile aynı
// toplama: doğrudan, gezen, zincir, ad kodu) ve ikiliyi derleyip koşan alt süreç testleri (klasör -> testler).
func (ix *index) testsToRun(ts *touchSet) (best map[string]int, subprocess map[string][]*Node) {
	best, subprocess = map[string]int{}, map[string][]*Node{}
	pkgs := map[string]bool{}
	for _, n := range ts.nodes {
		for id, d := range ix.testDistances(n) {
			if old, ok := best[id]; !ok || d < old {
				best[id] = d
			}
		}
		if p := ix.goPackageOf(n); p != "" && !pkgs[p] { // alt süreç testleri paketin tamamınındır: paket başına bir kez
			pkgs[p] = true
			for dir, list := range ix.subprocessTests(n) {
				subprocess[dir] = append(subprocess[dir], list...)
			}
		}
	}
	return best, subprocess
}

// printGoTests koşulacak Go testlerini satır başına "klasör TestAdı" olarak yazar (sıralı, tekrarsız): değişen
// testler, değişen kodu sınayanlar, alt süreç testleri ve değişen her dosyayı okuyan testler (grafiğin ayrıştırmadığı
// bir veri dosyası, planning_questions.txt gibi, hiçbir düğüme eşlenmez; onu okuyan test yine de seçilir). TR testleri
// (trlang test) ve ölçümler (Benchmark) girmez. Ön denetim (tools/full_check.sh --kip=on) bu listeyi koşar.
func (ix *index) printGoTests(w io.Writer, ts *touchSet) {
	for _, t := range ix.goTestSelection(ts) {
		fmt.Fprintln(w, t.dir+" "+t.name)
	}
}

// touchUntracked çalışma ağacındaki izlenmeyen kaynak dosyalarını ve bildirimlerini değişen kümeye ekler.
func (ix *index) touchUntracked(ts *touchSet, files []string) {
	for _, f := range files {
		ts.files[f] = true
		if file := ix.byID[fileID(f)]; file != nil {
			for id, d := range ix.fileDecls(file).nodes {
				ts.nodes[id] = d
			}
		}
		if tr := ix.byID[trFileID(f)]; tr != nil {
			ts.nodes[tr.ID] = tr
		}
	}
}

// goTest `go test`'in koşabildiği bir testtir: paketin klasörü ve Test/Fuzz adı.
type goTest struct{ dir, name string }

// goTestOf düğüm `go test -run` ile koşulan bir Go testi mi (Test* ya da Fuzz* işlevi); TR testleri ve ölçümler değil.
func goTestOf(n *Node) (goTest, bool) {
	if !isTestNode(n) || n.Kind != kindFunc {
		return goTest{}, false
	}
	name := n.Name[strings.LastIndex(n.Name, ".")+1:]
	if !strings.HasPrefix(name, "Test") && !strings.HasPrefix(name, "Fuzz") {
		return goTest{}, false
	}
	return goTest{path.Dir(n.File), name}, true
}

// goTestSelection `changed --go-tests`'in seçimidir (printGoTests), "klasör ad" sırasıyla, tekrarsız. tests-for <Go dosyası>'nın
// koşu satırı (runPlanFor) aynı seçimden yazılır: koşmadığını sayar, ikinci bir seçim yolu yoktur.
func (ix *index) goTestSelection(ts *touchSet) []goTest {
	best, subprocess := ix.testsToRun(ts)
	return ix.goTestSelectionFrom(ts, best, subprocess)
}

// goTestSelectionFrom goTestSelection'dır, testsToRun(ts)'in sonucu verilmiş olarak (best'in yalnız anahtarları
// kullanılır). tests-for <Go dosyası> onu fileTests'ten alır, yeniden hesaplamaz (hız işi, tekrar 2).
func (ix *index) goTestSelectionFrom(ts *touchSet, best map[string]int, subprocess map[string][]*Node) []goTest {
	nodes := ownTests(ts)
	for id := range best {
		nodes = append(nodes, ix.byID[id])
	}
	for f := range ts.files {
		for _, id := range []string{"file:" + f, trFileID(f), fileID(f), docID(f)} { // dosyanın testleri: okuyan, koşan, aktaran ve bütün listeyi gezenler
			if file := ix.byID[id]; file != nil {
				for id := range ix.testDistances(file) {
					nodes = append(nodes, ix.byID[id])
				}
			}
		}
		for _, h := range ix.scansCovering(f) {
			nodes = append(nodes, h.tests...)
		}
		for _, h := range ix.typeQueriesCovering(f) {
			nodes = append(nodes, h.tests...)
		}
	}
	for _, list := range subprocess {
		nodes = append(nodes, list...)
	}
	nodes = append(nodes, ix.examTestsFor(ts)...)
	seen := map[goTest]bool{}
	var out []goTest
	add := func(n *Node) {
		if t, ok := goTestOf(n); ok && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, n := range nodes {
		add(n)
		for _, r := range ix.fixtureRunners(n) {
			add(r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dir+" "+out[i].name < out[j].dir+" "+out[j].name })
	return out
}

func (ix *index) printTestsToRun(w io.Writer, ts *touchSet, limit int, what, ownWhat string) {
	best, subprocess := ix.testsToRun(ts)
	defer ix.printSubprocessTests(w, subprocess)
	own := ownTests(ts)
	if len(own) > 0 {
		fmt.Fprintf(w, "  %s (%d):\n", ownWhat, len(own))
		forEachLimited(w, own, limit, "    ", moreHint, describe)
	}
	ids := make([]string, 0, len(best))
	for id := range best {
		if (isTestNode(ix.byID[id]) || isNegativeFixture(ix.byID[id])) && ts.nodes[id] == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		if len(subprocess) == 0 && len(own) == 0 {
			fmt.Fprintln(w, "  tests: none found for "+what)
		}
		return
	}
	targets := make([]*Node, 0, len(ts.nodes))
	for _, n := range ts.nodes {
		targets = append(targets, n)
	}
	extracted := ix.extractedDirectTests(targets)
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		if best[a] != best[b] {
			return best[a] < best[b]
		}
		if best[a] == 0 && extracted[a] != extracted[b] {
			return extracted[a]
		}
		return ix.byID[a].Name < ix.byID[b].Name
	})
	fmt.Fprintf(w, "  tests that exercise "+what+" (%d, direct first):\n", len(ids))
	lines := make([]string, len(ids))
	for i, id := range ids {
		how := "[INFERRED]"
		switch {
		case best[id] > 0:
			how = fmt.Sprintf("%d steps [INFERRED]", best[id])
		case extracted[id]:
			how = "direct"
		}
		lines[i] = fmt.Sprintf("%s  %s", describe(ix.byID[id]), how)
	}
	printLimited(w, lines, limit, "    ")
}

// ownTests kümenin kendi bildirdiği testlerdir (Benchmark* hariç, ada göre; tools/bench'teki bir Test de kendi aracını
// sınar, isMeasurement'ın klasör kuralı burada geçmez): bir test dosyasının ya da değişikliğin
// testi kendisini sınar. Eskiden yalnız kümedeki bir yardımcıyı çağıranlar listeleniyordu, test dosyasının yardımcısız
// testleri "none found" diye kayboluyordu (eksikler listesi, ince ayar: type_names_test.go, runner_test.go 10 testten 1'i).
func ownTests(ts *touchSet) []*Node {
	var own []*Node
	for _, n := range ts.nodes {
		if isTestNode(n) && !strings.HasPrefix(n.Name[strings.LastIndex(n.Name, ".")+1:], "Benchmark") {
			own = append(own, n)
		}
	}
	sort.Slice(own, func(i, j int) bool { return own[i].Name < own[j].Name })
	return own
}

// printOverlaps öbür yerel claude/ dallarının aynı düğümlere ya da dosyalara dokunup
// dokunmadığını yazar; aynı dosyaya dokunanlar için `git merge-tree` çakışma önizlemesi verir.
func (ix *index) printOverlaps(w io.Writer, o changeOpts, ref string, mine *touchSet) {
	// Tabana birleşmiş dalın base...dal farkı boştur; onları git tek çağrıda eler (83 dalda 9 sn yerine 0,3 sn).
	out, err := gitOut(o.root, "for-each-ref", "--no-merged="+o.base, "--format=%(refname:short)", "refs/heads/claude/")
	if err != nil {
		fmt.Fprintf(w, "  other branches: cannot list (%v)\n", firstLine(err))
		return
	}
	myKeys := map[string]bool{}
	for _, k := range mine.keys() {
		myKeys[k] = true
	}
	var lines []string
	for _, br := range nonEmptyLines(string(out)) {
		if br == ref || sameCommit(o.root, br, ref) {
			continue
		}
		hs, err := diffHunks(o.root, o.base+"..."+br)
		if err != nil || len(hs) == 0 {
			continue
		}
		theirs := ix.touched(hs, false)
		if l := overlapLine(o, ref, br, myKeys, mine, theirs); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		fmt.Fprintln(w, "  other claude/ branches: none touches the same nodes or files")
		return
	}
	fmt.Fprintf(w, "  other claude/ branches that touch the same code (%d):\n", len(lines))
	printLimited(w, lines, 0, "    ")
}

func overlapLine(o changeOpts, ref, br string, myKeys map[string]bool, mine, theirs *touchSet) string {
	var shared, files []string
	for _, k := range theirs.keys() {
		if myKeys[k] {
			shared = append(shared, k)
		}
	}
	for f := range theirs.files {
		if mine.files[f] {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return ""
	}
	desc := fmt.Sprintf("%s: %d shared nodes", br, len(shared))
	if len(shared) > 0 {
		desc += " (" + strings.Join(trimIDs(shared, 4), ", ") + ")"
	}
	desc += "; shared files: " + strings.Join(trimIDs(files, 4), ", ")
	return desc + "; merge: " + mergePreview(o.root, ref, br)
}

func trimIDs(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return append(append([]string{}, list[:n]...), fmt.Sprintf("+%d more", len(list)-n))
}

// mergePreview `git merge-tree --write-tree` ile iki ucu birleştirmeyi dener (çalışma ağacına
// dokunmaz) ve "clean" ya da çakışan dosyaları verir.
func mergePreview(root, a, b string) string {
	root = strings.TrimSpace(root)
	out, err := gitStdout(root, "merge-tree", "--write-tree", "--name-only", a, b)
	if err == nil {
		return "clean"
	}
	// Çakışmada git 1 koduyla çıkar ve çakışan dosyaları stdout'a yazar; başka hata önizleme yok demektir.
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return "unknown (git merge-tree unavailable)"
	}
	var names []string
	for i, l := range strings.Split(string(out), "\n") {
		if i == 0 {
			continue // ağaç kimliği
		}
		if strings.TrimSpace(l) == "" {
			break
		}
		names = append(names, strings.TrimSpace(l))
	}
	return "CONFLICT in " + strings.Join(trimIDs(names, 4), ", ")
}

// printBranchDecls grafikte düğümü olmayan, dalda eklenen ya da değişen bildirimleri yazar.
func printBranchDecls(w io.Writer, decls map[string][]string, limit int) {
	var lines []string
	for file, names := range decls {
		lines = append(lines, file+": "+strings.Join(names, ", "))
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "  declarations on the branch that the graph does not have (%d files; new functions, new tests):\n", len(lines))
	printLimited(w, lines, limit, "    ")
}
