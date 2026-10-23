package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
)

// changeOpts changed komutunun ayarlarıdır.
type changeOpts struct {
	root  string
	base  string // dal adı verildiğinde karşılaştırılan dal (git diff base...dal)
	limit int
}

// runChanged ref'in (dal ya da a..b / a...b aralığı) değişikliklerini grafiğin düğümlerine eşler:
// değişen düğümler, onları sınayan testler ve aynı düğümlere dokunan öbür yerel claude/ dalları.
//
// Eşleme: grafik bugünkü çalışma ağacının grafiğidir. ref bugünkü HEAD ise yeni dosyadaki
// satırlar eşlenir (yeni işlevler dahil); değilse hunk'ların eski tarafı (birleşme tabanındaki
// satırlar) eşlenir, bu ana dal henüz kımıldamadıysa doğrudur. Yalnızca eklenen kod (eski
// tarafta karşılığı yok) "mapped to no node" altında dosya olarak sayılır.
func (ix *index) runChanged(o changeOpts, ref string, stdout, stderr io.Writer) int {
	rng, right := o.base+"..."+ref, ref
	if _, b, ok := splitRange(ref); ok {
		rng, right = ref, b
	}
	hs, err := diffHunks(o.root, rng)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	oldSide := ix.touched(hs, false)
	main, mapping := oldSide, "old-side lines (the merge base); the graph is of the checked-out tree, so this is exact only while it equals the merge base"
	if sameCommit(o.root, right, "HEAD") {
		main, mapping = ix.touched(hs, true), "new-side lines (the graph is of this very tree)"
	}
	fmt.Fprintf(stdout, "changed %s: %d hunks in %d files\n  mapping: %s\n", rng, len(hs), len(main.files), mapping)
	ix.printTouched(stdout, main, o.limit)
	if main == oldSide { // grafik dalın ağacı değil: dalda yeni olan bildirimleri dalın kendi dosyalarından adlandır
		printBranchDecls(stdout, branchDecls(o.root, right, main.unmappedHunks), o.limit)
	}
	ix.printTestsToRun(stdout, main, o.limit)
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

// diffHunks `git diff` parçalarını verir (yollar root'a göre).
func diffHunks(root, rng string) ([]hunk, error) {
	out, err := gitOut(root, "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--relative", rng)
	if err != nil {
		return nil, err
	}
	return parseDiff(string(out)), nil
}

// sameCommit iki revizyon aynı commit'e çözülüyor mu.
func sameCommit(root, a, b string) bool {
	x, err1 := gitOut(root, "rev-parse", a+"^{commit}")
	y, err2 := gitOut(root, "rev-parse", b+"^{commit}")
	return err1 == nil && err2 == nil && string(x) == string(y)
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
func (ix *index) printTestsToRun(w io.Writer, ts *touchSet, limit int) {
	best := map[string]int{} // test kimliği -> en kısa uzaklık (0: doğrudan)
	for _, n := range ts.nodes {
		for id, d := range ix.testDistances(n) { // tests-for ile aynı toplama: doğrudan, gezen, zincir, ad kodu
			if old, ok := best[id]; !ok || d < old {
				best[id] = d
			}
		}
	}
	ids := make([]string, 0, len(best))
	for id := range best {
		if isTestNode(ix.byID[id]) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		fmt.Fprintln(w, "  tests: none found for the changed code")
		return
	}
	sort.Slice(ids, func(i, j int) bool {
		if best[ids[i]] != best[ids[j]] {
			return best[ids[i]] < best[ids[j]]
		}
		return ix.byID[ids[i]].Name < ix.byID[ids[j]].Name
	})
	fmt.Fprintf(w, "  tests that exercise the changed code (%d, direct first):\n", len(ids))
	lines := make([]string, len(ids))
	for i, id := range ids {
		how := "direct"
		if best[id] > 0 {
			how = fmt.Sprintf("%d steps [INFERRED]", best[id])
		}
		lines[i] = fmt.Sprintf("%s  %s", describe(ix.byID[id]), how)
	}
	printLimited(w, lines, limit, "    ")
}

// printOverlaps öbür yerel claude/ dallarının aynı düğümlere ya da dosyalara dokunup
// dokunmadığını yazar; aynı dosyaya dokunanlar için `git merge-tree` çakışma önizlemesi verir.
func (ix *index) printOverlaps(w io.Writer, o changeOpts, ref string, mine *touchSet) {
	out, err := gitOut(o.root, "for-each-ref", "--format=%(refname:short)", "refs/heads/claude/")
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
