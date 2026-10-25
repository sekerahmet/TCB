package e2e

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Görüntü sınavı (madde 15, görüntü kuralı): kişiye görünen her yüzey okuyanın dilindedir.
// İngilizce ve Türkçe kodlu küçük programlar TR_DIL=en ve TR_DIL=tr ile çalışır; çıktıda okuyanın
// dili dışındaki bir dilin sözcüğü geçmez (iki dilde aynı yazılanlar, kullanıcı adları ve
// programdaki metinler hariç), hata çerçevesinin satırı okuyanın görünümündeki satırdır,
// çerçevedeki '^' ve iletideki sütun işaretlenen adın o görünümdeki yeridir. Okuyan, görüntüde
// TR_DIL'dir; değerin metninde ve yakalanan hatada dosyanın dilidir (17.7, 8.3).
//
// Bugün kalan durumlar testdata/display_known.txt'dedir ("yüzey F R tür ayrıntı"); liste
// yalnızca küçülür. Yeniden yazmak için: go test ./cmd/tr/e2e -run TestDisplayExam -update

var updateDisplayKnown = flag.Bool("update", false, "testdata/display_known.txt'i bugünkü sonuçla yeniden yaz")

// displaySurface sınavın bir yüzeyidir. Programlar Türkçe görünümle yazılır, dosyanın diline (F)
// çevrilip kodlanır; ad_N kullanıcı adlarıdır.
type displaySurface struct {
	name string
	// files kodlu yazılan dosyalardır (Türkçe görünüm); raw olduğu gibi yazılanlardır.
	files map[string]string
	raw   func(r *codes.Language) map[string]string
	// typed: girdi okuyanın dilinde yazılır (kabuk, kodla -); dosyanın dili yoktur, F = R.
	typed bool
	// readerIsFile: okuyan dosyanın dilidir (değer metni, yakalanan hata).
	readerIsFile bool
	args         func(r *codes.Language) []string
	input        func(r *codes.Language) string
	// build: önce 'tr derle', sonra üretilen uygulama çalışır.
	build bool
	// frameFile çerçevelerin okuyanın görünümüyle karşılaştırılacağı dosyadır ("<input>": girdi);
	// iletisi başka bir dosyayı anan çerçeve o dosyanın görünümüyle karşılaştırılır (modül hatası).
	frameFile string
	// mark iletideki sütunun ve '^'nin göstermesi gereken kullanıcı adıdır; markFile onun
	// dosyasıdır (boşsa frameFile).
	mark     string
	markFile string
	// rawFrame: çerçeve dosyayı yazıldığı gibi gösterir, sızıntı denetimine girmez.
	rawFrame bool
	json     bool
}

func cmd(k codes.Code) func(r *codes.Language) []string {
	return func(r *codes.Language) []string { return []string{r.Name(k), "p.tr"} }
}

var displaySurfaces = []displaySurface{
	{name: "çalışma_hatası", files: map[string]string{"p.tr": "değişken ad_1 = [1]\nyazdır(ad_1.ad_9)\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }, frameFile: "p.tr"},
	{name: "denetim_hatası", files: map[string]string{"p.tr": "değişken ad_1 = 1\nyazdır(ad_1 + ad_9)\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }, frameFile: "p.tr", mark: "ad_9"},
	{name: "kontrol", files: map[string]string{"p.tr": "değişken ad_1 = 1\nyazdır(ad_1 + ad_9)\n"},
		args: cmd(codes.K004), frameFile: "p.tr", mark: "ad_9"},
	{name: "uyarı", files: map[string]string{"p.tr": "değişken ad_1 = 1\n- mutlak(ad_1)\nyazdır(ad_1)\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }, frameFile: "p.tr"},
	{name: "test_raporu", files: map[string]string{"p_test.tr": "fonksiyon ad_1() {\n    sonuç 2\n}\ntest \"t\" {\n    eşit_mi(5, 2 + ad_1())\n}\n"},
		args: func(r *codes.Language) []string { return []string{r.Name(codes.K007), "p_test.tr"} }, frameFile: "p_test.tr"},
	{name: "kabuk", typed: true, input: func(r *codes.Language) string { return r.Name(codes.G100) + "(ad_9 + 1)\n" },
		args: func(*codes.Language) []string { return nil }, frameFile: "<input>", mark: "ad_9"},
	{name: "modül_hatası", files: map[string]string{"p.tr": "aktar \"m\"\nyazdır(ad_2)\n", "m.tr": "değişken ad_2 = 1\nyazdır(ad_2 + ad_9)\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }, frameFile: "p.tr", mark: "ad_9", markFile: "m.tr"},
	{name: "yakalanan_hata", readerIsFile: true, files: map[string]string{"p.tr": "dene {\n    değişken ad_1 = [1].ad_9\n} yakala ad_2 {\n    yazdır(ad_2)\n}\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }},
	{name: "yapı_değeri", readerIsFile: true, files: map[string]string{"p.tr": "yapı P {\n    uzunluk\n}\nyazdır(P(uzunluk: 2))\n"},
		args: func(*codes.Language) []string { return []string{"p.tr"} }},
	{name: "dizin", files: map[string]string{"p.tr": "fonksiyon ad_1(x) {\n    sonuç x\n}\nyazdır(ad_1(1))\n"},
		args: func(r *codes.Language) []string { return []string{r.Name(codes.K015), r.Name(codes.K016), "ad_1"} }, mark: "ad_1"},
	{name: "dizin_json", json: true, files: map[string]string{"p.tr": "fonksiyon ad_1(x) {\n    sonuç x\n}\nyazdır(ad_1(1))\n"},
		args: func(r *codes.Language) []string {
			return []string{r.Name(codes.K015), r.Name(codes.K016), "ad_1", "--" + r.Name(codes.K103)}
		}, mark: "ad_1"},
	{name: "kodla", typed: true, input: func(r *codes.Language) string { return r.Name(codes.G100) + "(ad_1 +)\n" },
		args: func(r *codes.Language) []string {
			return []string{r.Name(codes.K009), "-", "--" + r.Name(codes.K100), r.Code}
		},
		frameFile: "<input>"},
	{name: "göster", typed: true, rawFrame: true,
		raw: func(*codes.Language) map[string]string {
			return map[string]string{"p.tr": turkishLanguage.Name(codes.G100) + "(ad_1 +)\n"}
		},
		args: cmd(codes.K011)},
	{name: "derle", build: true, files: map[string]string{"p.tr": "değişken ad_1 = [1]\nyazdır(ad_1.ad_9)\n"}, frameFile: "p.tr"},
	{name: "hata_ayıklayıcı", files: map[string]string{"p.tr": "değişken ad_1 = [1]\nyazdır(ad_1.ad_9)\n"},
		args:  cmd(codes.K006),
		input: func(r *codes.Language) string { return strings.Repeat(r.Name(codes.K303)+"\n", 3) }, frameFile: "p.tr"},
}

var turkishLanguage = mustLanguage("tr")

func mustLanguage(code string) *codes.Language {
	l, err := codes.LoadLanguage(code)
	if err != nil {
		panic(err)
	}
	return l
}

var (
	wordPattern  = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)
	framePattern = regexp.MustCompile(`^\s*(?:=+>)?\s*(\d+) \| ?(.*)$`)
	caretPattern = regexp.MustCompile(`^\s*\| ?(.*\^.*)$`)
	// İletideki konum: "Line 2, column 12:" ya da "Satır 2, Sütun 12:" (sözcükler dilden okunmaz).
	columnPattern = regexp.MustCompile(`(\d+), \p{L}+ (\d+):`)
	indexPattern  = regexp.MustCompile(`p\.tr:(\d+):(\d+)`)
)

// foreignWords okuyanın dili dışındaki dillerin her ad alanındaki sözcükleridir; okuyanın dilinde
// aynı yazılanlar girmez.
func foreignWords(reader *codes.Language) map[string]bool {
	own := map[string]bool{}
	for _, k := range reader.Codes() {
		if k.Namespace() != codes.NamespaceNone {
			own[reader.Name(k)] = true
		}
	}
	words := map[string]bool{}
	for _, code := range codes.Languages() {
		if code == reader.Code {
			continue
		}
		l := mustLanguage(code)
		for _, k := range l.Codes() {
			if name := l.Name(k); k.Namespace() != codes.NamespaceNone && !own[name] {
				words[name] = true
			}
		}
	}
	return words
}

// markPosition adın görünümdeki ilk yeridir (satır ve sütun, 1'den; sütun harf olarak).
func markPosition(view, mark string) (line, col int) {
	for i, text := range strings.Split(view, "\n") {
		if j := strings.Index(text, mark); j >= 0 {
			return i + 1, utf8.RuneCountInString(text[:j]) + 1
		}
	}
	return 0, 0
}

func TestDisplayExam(t *testing.T) {
	var failing []string
	for _, s := range displaySurfaces {
		for _, fCode := range codes.Languages() {
			for _, rCode := range codes.Languages() {
				if s.typed && fCode != rCode {
					continue
				}
				f, r := mustLanguage(fCode), mustLanguage(rCode)
				for _, entry := range displayCase(t, s, f, r) {
					failing = append(failing, fmt.Sprintf("%s %s %s %s", s.name, fCode, rCode, entry))
				}
			}
		}
	}
	header := "# Görüntü sınavının bugün kalan durumları (TestDisplayExam): yüzey F R tür ayrıntı.\n" +
		"# Liste yalnızca küçülür; -update ile yeniden yazılır.\n"
	testkit.CheckKnownList(t, "testdata/display_known.txt", header, failing, *updateDisplayKnown)
}

// displayCase yüzeyi F'de yazılmış dosyalarla TR_DIL=R'de çalıştırır ve kalan durumları verir:
// "sızıntı <sözcük>", "çerçeve <satır>", "çerçeve_yok", "işaret", "sütun", "konum".
func displayCase(t *testing.T, s displaySurface, f, r *codes.Language) []string {
	t.Helper()
	dir := t.TempDir()
	coded := map[string]string{}
	for name, turkish := range s.files {
		view, err := source.Show(turkish, f)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if coded[name], err = source.EncodeIn(view, f); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		testkit.WriteFile(t, dir, name, coded[name])
	}
	if s.raw != nil {
		for name, text := range s.raw(r) {
			testkit.WriteFile(t, dir, name, text)
		}
	}
	input := ""
	if s.input != nil {
		input = s.input(r)
	}
	env := []string{"TR_DIL=" + r.Code}
	var res testkit.Result
	if s.build {
		app := filepath.Join(dir, "app.exe")
		if b := testkit.RunBinary(t, trBinary, dir, "", env, r.Name(codes.K005), "p.tr", app); b.Code != 0 {
			t.Fatalf("%s: derlenemedi: %s%s", s.name, b.Stdout, b.Stderr)
		}
		res = testkit.RunBinary(t, app, dir, input, env)
	} else {
		res = testkit.RunBinary(t, trBinary, dir, input, env, s.args(r)...)
	}
	out := res.Stdout + res.Stderr
	reader := r
	if s.readerIsFile {
		reader = f
	}
	var entries []string
	leaks := foreignWords(reader)
	lines := strings.Split(out, "\n")
	checked := out
	if s.json {
		checked = jsonProgramText(t, res.Stdout)
	}
	for _, line := range strings.Split(checked, "\n") {
		if s.rawFrame && (framePattern.MatchString(line) || caretPattern.MatchString(line)) {
			continue
		}
		for _, w := range wordPattern.FindAllString(line, -1) {
			if leaks[w] {
				entries = append(entries, "sızıntı "+w)
			}
		}
	}
	views := map[string]string{"<input>": input}
	for name, text := range coded {
		view, err := source.Show(text, reader)
		if err != nil {
			t.Fatal(err)
		}
		views[name] = view
	}
	markFile := cmp.Or(s.markFile, s.frameFile, "p.tr")
	markLine, markCol := markPosition(views[markFile], s.mark)
	if s.frameFile != "" {
		entries = append(entries, frameEntries(lines, views, s.frameFile, markFile, s.mark != "", markCol)...)
	}
	if s.mark != "" && !s.json && s.name != "dizin" {
		if m := columnPattern.FindStringSubmatch(out); m == nil {
			entries = append(entries, "sütun_yok")
		} else if col, _ := strconv.Atoi(m[2]); col != markCol {
			entries = append(entries, "sütun")
		}
	}
	if s.name == "dizin" || s.json {
		got := 0
		if s.json {
			got = jsonLine(t, res.Stdout)
		} else if m := indexPattern.FindStringSubmatch(out); m != nil {
			got, _ = strconv.Atoi(m[1])
		}
		if got != markLine {
			entries = append(entries, "konum")
		}
	}
	if testing.Verbose() {
		t.Logf("%s %s %s:\n%s", s.name, f.Code, r.Code, out)
	}
	return entries
}

// frameEntries çerçevenin her satırını, iletisinin andığı dosyanın (anmıyorsa main'in) okuyanın
// görünümündeki satırıyla, işaretli adın dosyasındaki '^'yi adın sütunuyla karşılaştırır.
func frameEntries(lines []string, views map[string]string, main, markFile string, withCaret bool, markCol int) []string {
	var entries []string
	frames := 0
	file := main
	for i, line := range lines {
		if strings.Contains(line, "•") {
			file = main
			for name := range views {
				if strings.Contains(line, name+",") {
					file = name
				}
			}
		}
		m := framePattern.FindStringSubmatch(line)
		if m == nil || caretPattern.MatchString(line) {
			continue
		}
		frames++
		view := strings.Split(views[file], "\n")
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > len(view) || strings.TrimRight(view[n-1], "\r") != m[2] {
			entries = append(entries, "çerçeve "+m[1])
		}
		if withCaret && file == markFile && i+1 < len(lines) {
			if c := caretPattern.FindStringSubmatch(lines[i+1]); c != nil {
				if utf8.RuneCountInString(c[1][:strings.Index(c[1], "^")])+1 != markCol {
					entries = append(entries, "işaret")
				}
			}
		}
	}
	if frames == 0 {
		entries = append(entries, "çerçeve_yok")
	}
	return entries
}

// jsonProgramText 'tr dizin --json' çıktısındaki program metinlerini (imza) verir: şemanın
// anahtarları ve sabit değerleri sızıntı sayılmaz.
func jsonProgramText(t *testing.T, out string) string {
	t.Helper()
	var doc struct {
		Target struct {
			Signature string `json:"signature"`
		} `json:"target"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("dizin --json: %v\n%s", err, out)
	}
	return doc.Target.Signature
}

func jsonLine(t *testing.T, out string) int {
	t.Helper()
	var doc struct {
		Target struct {
			Pos struct {
				Line int `json:"line"`
			} `json:"pos"`
		} `json:"target"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("dizin --json: %v\n%s", err, out)
	}
	return doc.Target.Pos.Line
}
