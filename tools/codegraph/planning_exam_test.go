package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
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
	"callees": true, "calls-into": true, "fields": true, "captures": true}

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
	root, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	records := parseExam(t, filepath.Join("testdata", "planning_questions.txt"))
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := codeindex.Build(root, codeindex.Options{Library: lib.Files})
	if err != nil {
		t.Fatal(err)
	}
	x := &examRunner{root: root, graph: g, reader: codeindex.NewReader(idx)}
	for _, r := range records {
		out, code := x.answer(t, r)
		for _, c := range r.checks {
			if why := checkExam(c, out, code); why != "" {
				t.Errorf("planning_questions.txt:%d (%s): %s %s\n  expectation %q: %s\n%s", r.line, r.source, r.tool,
					strings.Join(r.args, " "), c, why, indentOutput(out))
			}
		}
	}
}

func indentOutput(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 25 {
		lines = append(lines[:25], "...")
	}
	return "    | " + strings.Join(lines, "\n    | ")
}
