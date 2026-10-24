package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/testkit"
	"github.com/sekerahmet/TCB/lib"
)

// Planlama sınavı: codegraph ve tr dizin ile gerçek bir işi planlarken sorulmuş sorular, beklenen cevaplarıyla
// (testdata/planning_questions.txt). Her satır bir komut, yapısal beklentiler ve kaynaktır; beklentiler çıktının birebir
// metni değil, "şunu içerir / şu grubun ilk öğesi şudur / şu, şundan önce gelir" biçimindedir, böylece çıktı biçimi
// iyileşince sınav kırılmaz ama kapatılmış bir kör nokta geri gelirse düşer. Depo üzerinde koşar (fikstür değil):
// grafik ve TR dizini bir kez kurulur, her soru onlara sorulur. Biçim dosyanın başında anlatılır.

// examRecord dosyanın bir satırıdır.
type examRecord struct {
	line   int
	tool   string   // "codegraph" ya da "tr dizin"
	args   []string // komut ve argümanlar
	checks []string // "anahtar: değer" beklentileri
	source string
}

// parseExam soru dosyasını okur. Satır: `araç komut arg... | beklenti | beklenti | src: kaynak`; `#` ile başlayan ve boş
// satırlar atlanır. Argümanlar boşlukla ayrılır; boşluk içeren argüman çift tırnakla yazılır.
func parseExam(t *testing.T, path string) []examRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []examRecord
	for i, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, " | ")
		r := examRecord{line: i + 1}
		words := splitArgs(fields[0])
		switch {
		case len(words) >= 2 && words[0] == "codegraph":
			r.tool, r.args = "codegraph", words[1:]
		case len(words) >= 3 && words[0] == "tr" && words[1] == "dizin":
			r.tool, r.args = "tr dizin", words[2:]
		default:
			t.Fatalf("%s:%d: a record starts with `codegraph` or `tr dizin`: %q", path, i+1, fields[0])
		}
		for _, f := range fields[1:] {
			if src, ok := strings.CutPrefix(f, "src: "); ok {
				r.source = src
				continue
			}
			r.checks = append(r.checks, f)
		}
		if r.source == "" || len(r.checks) == 0 {
			t.Fatalf("%s:%d: a record needs at least one expectation and a `src:` field", path, i+1)
		}
		out = append(out, r)
	}
	return out
}

// splitArgs komut metnini boşluktan böler; çift tırnak içi tek argümandır.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	quoted, any := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, any = !quoted, true
		case r == ' ' && !quoted:
			if any {
				out = append(out, cur.String())
			}
			cur.Reset()
			any = false
		default:
			cur.WriteRune(r)
			any = true
		}
	}
	if any {
		out = append(out, cur.String())
	}
	return out
}

// queryCommands grafiği okuyan codegraph komutlarıdır; sınav onları yüklenmiş grafikle çalıştırır (runQueryOn).
var queryCommands = map[string]bool{"query": true, "explain": true, "path": true, "who-uses": true, "tests-for": true,
	"callees": true, "calls-into": true, "fields": true, "captures": true, "changed": true}

// historyCommands git geçmişini okuyan komutlardır; sığ klonda (CI) cevapları yoktur, sınav onları atlar.
var historyCommands = map[string]bool{"changed": true, "deleted": true, "history": true}

// examRunner sorulara cevap verir: grafik ve TR dizini bir kez kurulur.
type examRunner struct {
	root   string
	graph  *Graph
	reader *codeindex.Reader
}

// answer bir sorunun birleşik çıktısını (stdout ve stderr) ve çıkış kodunu verir.
func (x *examRunner) answer(t *testing.T, r examRecord) (string, int) {
	if r.tool == "tr dizin" {
		return x.trIndex(t, r)
	}
	var out, errb strings.Builder
	args := append(append([]string{}, r.args[1:]...), "--root", x.root)
	var code int
	if queryCommands[r.args[0]] {
		code = runQueryOn(x.graph, r.args[0], args, &out, &errb) // grafik bir kez kurulur, her soru ona sorulur
	} else {
		code = run(append([]string{r.args[0]}, args...), &out, &errb)
	}
	return out.String() + errb.String(), code
}

// trIndex `tr dizin <alt komut> <ad> [--tümü]` sorusunu, komutun kendi kodu gibi codeindex'e sorar: alt komut adı
// kodundan çözülür (K016 tanım, K017 kullananlar, K018 sınayanlar, K019 etki), metin codeindex.Text'tir.
func (x *examRunner) trIndex(t *testing.T, r examRecord) (string, int) {
	sub, ok := codes.Find(codes.NamespaceSubcommand, r.args[0])
	if !ok || len(r.args) < 2 {
		t.Fatalf("planning_questions.txt:%d: unknown tr dizin subcommand %q", r.line, r.args[0])
	}
	o := codeindex.QueryOptions{Limit: 12}
	if len(r.args) > 2 && strings.HasPrefix(r.args[2], "--") {
		o.Limit = 0 // --tümü
	}
	var res *codeindex.Result
	switch sub {
	case codes.K016:
		res = x.reader.Definition(r.args[1], o)
	case codes.K017:
		res = x.reader.References(r.args[1], o)
	case codes.K018:
		res = x.reader.TestedBy(r.args[1], o)
	case codes.K019:
		res = x.reader.Impact(r.args[1], o)
	default:
		t.Fatalf("planning_questions.txt:%d: tr dizin %s is not a query", r.line, r.args[0])
	}
	return codeindex.Text(res), codeindex.ExitCode(res.Status)
}

// checkExam bir beklentiyi çıktıya karşı denetler; tutmuyorsa nedenini verir. Beklentiler:
//
//	contains: A            çıktıda A geçer (A1 || A2: biri yeter)
//	not: A                 çıktıda A geçmez
//	exit: N                çıkış kodu N
//	first-item: A          ilk öğe satırı (en az 4 boşlukla girintili ilk satır) A'yı içerir
//	group: H => A          H'yi içeren başlık satırının altındaki (daha girintili) satırlardan biri A'yı içerir
//	first-of: H => A       o grubun ilk satırı A'yı içerir
//	top: H => N => A       o grubun ilk N satırından biri A'yı içerir
//	before: A => B         A, B'den önce geçer (ikisi de geçer)
func checkExam(check, out string, code int) string {
	key, val, _ := strings.Cut(check, ": ")
	lines := strings.Split(out, "\n")
	switch key {
	case "contains":
		if !containsAny(out, val) {
			return "does not contain " + strconv.Quote(val)
		}
	case "not":
		if strings.Contains(out, val) {
			return "contains " + strconv.Quote(val)
		}
	case "exit":
		if want, _ := strconv.Atoi(val); code != want {
			return fmt.Sprintf("exit %d, want %d", code, want)
		}
	case "first-item":
		for _, l := range lines {
			if strings.HasPrefix(l, "    ") {
				if !containsAny(l, val) {
					return "first item is " + strconv.Quote(strings.TrimSpace(l)) + ", want " + strconv.Quote(val)
				}
				return ""
			}
		}
		return "no item lines"
	case "group", "first-of":
		head, want, ok := strings.Cut(val, " => ")
		if !ok {
			return "want `HEADING => TEXT`"
		}
		items, found := groupItems(lines, head)
		if !found {
			return "no group " + strconv.Quote(head)
		}
		if key == "first-of" {
			if len(items) == 0 || !containsAny(items[0], want) {
				return fmt.Sprintf("group %q does not start with %q: %q", head, want, firstOr(items))
			}
			return ""
		}
		for _, l := range items {
			if containsAny(l, want) {
				return ""
			}
		}
		return fmt.Sprintf("group %q has no %q", head, want)
	case "top":
		parts := strings.SplitN(val, " => ", 3)
		n, err := 0, error(nil)
		if len(parts) == 3 {
			n, err = strconv.Atoi(parts[1])
		}
		if len(parts) != 3 || err != nil {
			return "want `HEADING => N => TEXT`"
		}
		items, found := groupItems(lines, parts[0])
		if !found {
			return "no group " + strconv.Quote(parts[0])
		}
		for i, l := range items {
			if i < n && containsAny(l, parts[2]) {
				return ""
			}
		}
		return fmt.Sprintf("%q is not in the first %d of group %q", parts[2], n, parts[0])
	case "before":
		a, b, ok := strings.Cut(val, " => ")
		if !ok {
			return "want `A => B`"
		}
		ia, ib := strings.Index(out, a), strings.Index(out, b)
		if ia < 0 || ib < 0 || ia > ib {
			return fmt.Sprintf("%q is not before %q (at %d and %d)", a, b, ia, ib)
		}
	default:
		return "unknown expectation " + strconv.Quote(key)
	}
	return ""
}

// containsAny s, `||` ile ayrılmış seçeneklerden birini içeriyor mu.
func containsAny(s, alternatives string) bool {
	for _, a := range strings.Split(alternatives, " || ") {
		if strings.Contains(s, a) {
			return true
		}
	}
	return false
}

// groupItems head'i içeren ilk satırın altındaki, ondan daha girintili ardışık satırları verir.
func groupItems(lines []string, head string) ([]string, bool) {
	for i, l := range lines {
		if !strings.Contains(l, head) {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		var items []string
		for _, m := range lines[i+1:] {
			if strings.TrimSpace(m) == "" || len(m)-len(strings.TrimLeft(m, " ")) <= indent {
				break
			}
			items = append(items, m)
		}
		return items, true
	}
	return nil, false
}

func firstOr(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0])
}

// Kural: planlama denemelerinde sorulan her soru bugün doğru cevaplanır (testdata/planning_questions.txt). Kapatılmış bir
// kör nokta (tür kullanımı, arayüz uygulaması, TR testleri, belge yorumu, madde atıfı ...) geri gelirse bu test düşer ve
// hangi denemenin hangi sorusu olduğunu söyler. -short ile de koşar (tam denetim -short'tur).
func TestPlanningQuestionsAreAnswered(t *testing.T) {
	root, _, g := repoGraph(t)
	records := parseExam(t, filepath.Join("testdata", "planning_questions.txt"))
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	x := &examRunner{root: root, graph: g, reader: codeindex.NewReader(idx)}
	if bad, checked := trTestSetMismatches(g, idx, x.reader); len(bad) > 0 || checked < 100 {
		t.Errorf("tr dizin sınayanlar and tests-for disagree on %d of %d TR functions:\n  %s", len(bad), checked, strings.Join(bad[:min(len(bad), 20)], "\n  "))
	}
	if bad, checked := trNameSetMismatches(g, idx, x.reader); len(bad) > 0 || checked < 20 {
		t.Errorf("tr dizin and codegraph resolve %d of %d bare TR names differently:"+"\n  %s", len(bad), checked, strings.Join(bad[:min(len(bad), 20)], "\n  "))
	}
	if bad, checked := testsListedTwice(g); len(bad) > 0 || checked < 100 {
		t.Errorf("tests-for lists a test in two groups for %d of %d targets:\n  %s", len(bad), checked, strings.Join(bad[:min(len(bad), 20)], "\n  "))
	}
	out, _ := gitStdout(root, "rev-parse", "--is-shallow-repository")
	shallow := strings.TrimSpace(string(out)) == "true" // CI kaynağı derinlik 1 ile alır: geçmiş yok
	for _, r := range records {
		if shallow && r.tool == "codegraph" && historyCommands[r.args[0]] {
			t.Logf("planning_questions.txt:%d: skipped, the clone has no history (%s)", r.line, r.args[0])
			continue
		}
		out, code := x.answer(t, r)
		for _, c := range r.checks {
			if why := checkExam(c, out, code); why != "" {
				t.Errorf("planning_questions.txt:%d (%s): %s %s\n  expectation %q: %s\n%s", r.line, r.source, r.tool,
					strings.Join(r.args, " "), c, why, indentOutput(out))
			}
		}
	}
}

// repoIndex depo testlerinin TR dizinidir: kod grafiğiyle aynı dosya kümesi, git'in izlediği dosyalar (Tracked).
// Depo testleri geliştiricinin izlenmeyen dosyalarına bağlı olmaz: ana kopyada scratch/ altındaki .gitignore'lu .tr
// dosyaları tr dizin'e girer (AllFiles, T1 kararı) ama grafiğe girmez; iki aracı karşılaştıran sınav bu yüzden yalnız
// orada düşüyordu (TCB, 2026-10-24). TestToolComparisonsIgnoreUntrackedFiles korur.
func repoIndex(root string) (*codeindex.Index, error) {
	return codeindex.Build(root, codeindex.Options{Library: lib.Files, Tracked: true})
}

// trTestSetMismatches iki aracın her TR işlev ve metodu için verdiği TR test kümelerini karşılaştırır: `tr dizin
// sınayanlar` (codeindex.TestedBy) ile `tests-for`'un TR testleri (testDistances) aynı olmalı. Zincir kuralı tek
// (codeindex.WalkCallers); giren çağıran kümesi de aynı olmalı (DENETİM doğrulaması T3).
func trTestSetMismatches(g *Graph, idx *codeindex.Index, reader *codeindex.Reader) (bad []string, checked int) {
	ix := newIndex(g)
	for _, n := range idx.Nodes {
		if n.Kind != codeindex.KindFunction && n.Kind != codeindex.KindMethod {
			continue
		}
		gn := ix.byID[graphID(n)]
		if gn == nil {
			continue
		}
		checked++
		want := map[string]bool{}
		for _, u := range reader.TestedBy(n.ID, codeindex.QueryOptions{}).Tests {
			if u.In != nil {
				want["tr:test:"+u.In.Pos.File+":"+strconv.Itoa(u.In.Pos.Line)] = true
			}
		}
		got := map[string]bool{}
		for id := range ix.testDistances(gn) {
			if ix.byID[id].Kind == kindTRTest {
				got[id] = true
			}
		}
		if !maps.Equal(want, got) {
			bad = append(bad, fmt.Sprintf("%s: tr dizin %d, tests-for %d", gn.Name, len(want), len(got)))
		}
	}
	sort.Strings(bad)
	return bad, checked
}

// trNameSetMismatches her TR adını iki araca sorar: `tr dizin` (codeindex.Resolve) ile codegraph (trByName, kaynakta
// yazıldığı gibi sorulunca lookup'ın kendisi) çıplak ad için aynı TR aday kümesini vermeli (deneme 7 R1: tanım 3,
// yeniden_adlandır 7, query 4 aday veriyordu; bağımsız denetim grup 6 R1: büyük harfli sorgu, modül + yerleşik ve
// yerleşik + test çakışması farklıydı). Adlar: her bildirimin kısa adı, modülün adı ve takma adları, testin açıklaması,
// dizindeki her yerleşiğin bütün adları; her ad yazıldığı gibi, küçük harfle ve büyük harfle (Türkçe kuralı) sorulur.
// Karşılaştırma tr dizin'in bildiği düğümlerdedir: grafiğin ad kodlarından yalnızca dizinde yerleşik düğümü olanlar
// sayılır (anahtar sözcükler ve kullanılmayan yerleşikler yalnızca grafiktedir).
func trNameSetMismatches(g *Graph, idx *codeindex.Index, reader *codeindex.Reader) (bad []string, checked int) {
	ix := newIndex(g)
	known := map[string]bool{} // tr dizin'in düğümlerinin grafik kimlikleri
	names := map[string]bool{}
	for _, n := range idx.Nodes {
		id := graphID(n)
		known[id] = true
		switch n.Kind {
		case codeindex.KindBuiltin:
			if gn := ix.byID[id]; gn != nil {
				names[gn.Name] = true
				for _, a := range gn.Aliases {
					names[a] = true
				}
			}
		case codeindex.KindModule:
			names[n.Name] = true
			for _, a := range n.Aliases {
				names[a] = true
			}
		case codeindex.KindTest:
			names[n.Name] = true // açıklamada '.' olabilir: kısaltılmaz
		default:
			names[n.Name[strings.LastIndex(n.Name, ".")+1:]] = true
		}
	}
	for name := range names {
		for vi, q := range []string{name, codeindex.Fold(name), strings.ToUpperSpecial(unicode.TurkishCase, name)} {
			if vi > 0 && q == name {
				continue
			}
			checked++
			want := map[string]bool{}
			for _, n := range reader.Resolve(q, "") {
				want[graphID(*n)] = true
			}
			got := map[string]bool{}
			found, _ := ix.trByName(q, nil)
			if vi == 0 && !strings.ContainsAny(q, "./:") {
				found = ix.lookup(q) // yazıldığı gibi sorulan adda sorgunun kendisi
			}
			for _, n := range found {
				if known[n.ID] {
					got[n.ID] = true
				}
			}
			if !maps.Equal(want, got) {
				bad = append(bad, fmt.Sprintf("%q: tr dizin %d %v, codegraph %d %v", q, len(want), slices.Sorted(maps.Keys(want)), len(got), slices.Sorted(maps.Keys(got))))
			}
		}
	}
	sort.Strings(bad)
	return bad, checked
}

// testsListedTwice her TR işlevi ve metodu, testi olan her ad kodu ve internal/stdlib'in her Go işlevi için tests-for'un
// test gruplarında bir testin iki kez yazılıp yazılmadığına bakar: her test bir kez, en güçlü kanıtının grubunda
// (bağımsız denetim grup 6, S1: files.TestDeleteWindowsJunction doğrudan grupta ve ad kodu grubunda yazılıyordu).
func testsListedTwice(g *Graph) (bad []string, checked int) {
	ix := newIndex(g)
	item := regexp.MustCompile(`^    (\S.*?) \[(func|tr_test)\]`)
	for i := range g.Nodes {
		n := &g.Nodes[i]
		switch {
		case n.Kind == kindTRFunc || n.Kind == kindTRMethod:
		case n.Kind == kindName && len(ix.in[n.ID]) > 0:
		case n.Kind == kindFunc && strings.HasPrefix(n.File, "internal/stdlib/") && !inTestFile(n):
		default:
			continue
		}
		checked++
		var buf strings.Builder
		ix.printTestsFor(&buf, n, 0)
		seen, inGroup := map[string]bool{}, false
		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				inGroup = strings.HasPrefix(line, "  <- tests") || strings.HasPrefix(line, "  <- weak evidence (") && !strings.Contains(line, "linked into the binary")
				continue
			}
			if m := item.FindStringSubmatch(line); inGroup && m != nil {
				if seen[m[1]] {
					bad = append(bad, n.Name+": "+m[1])
				}
				seen[m[1]] = true
			}
		}
	}
	sort.Strings(bad)
	return bad, checked
}

func indentOutput(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 25 {
		lines = append(lines[:25], "...")
	}
	return "    | " + strings.Join(lines, "\n    | ")
}

// Kural: iki aracı karşılaştıran depo denetimleri (sınayan kümesi, çıplak ad kümesi) aynı dosya kümesiyle çalışır;
// .gitignore'lu bir klasörde yazılmış .tr (geliştiricinin denemesi) karşılaştırmayı değiştirmez. Fikstürde `kenar`
// iki bildirimdir (alan ve kurucu parametresi); scratch/ altına aynı adlı bir işlev eklenir.
func TestToolComparisonsIgnoreUntrackedFiles(t *testing.T) {
	root := gitFixture(t)
	testkit.WriteFile(t, root, ".gitignore", "scratch/\n")
	testkit.WriteFile(t, root, "scratch/deneme.tr", "fonksiyon kenar(kenar) {\n    sonuç kenar\n}\n")
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	if bad, checked := trNameSetMismatches(g, idx, reader); len(bad) > 0 || checked == 0 {
		t.Errorf("bare names compared %d, differing: %v", checked, bad)
	}
	if bad, _ := trTestSetMismatches(g, idx, reader); len(bad) > 0 {
		t.Errorf("test sets differing: %v", bad)
	}
}
