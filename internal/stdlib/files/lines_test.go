package files

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Geçerli UTF-8 olmayan satır sıra ona gelince hatadır (18.11): ileti satırı söyler, TR_read_bytes'ı
// önerir, öneri çalışır; önceki satırlar verilmiş kalır, okuyucu sıradaki satırla sürer. TR_read_csv
// akış değildir, çağrı hata verir ve ilk bozuk satırı söyler (13.8). .gz'de öneri yoktur.
func TestInvalidUTF8LineIsAnError(t *testing.T) {
	k := tmpWork(t)
	for name, c := range map[string]struct {
		content string
		line    string
	}{
		"ilk.txt":  {"\xff\nb\n", "1"},
		"orta.txt": {"a\nb\xffc\nd\n", "2"},
		"son.txt":  {"a\nb\n\xc3", "3"},
		"uzun.txt": {strings.Repeat("a", 200_000) + "\n" + strings.Repeat("b", 200_000) + "\xff\n", "2"},
	} {
		write(t, filepath.Join(k, name), c.content)
		call := `TR_read_bytes("` + name + `")`
		for _, code := range []string{`[s TR_for s TR_in TR_read_lines("` + name + `")]`, `TR_read_csv("` + name + `")`} {
			err := runTR(t, k, code).failure
			if !messages.Has(err, messages.H2638) {
				t.Fatalf("%s: H2638 bekleniyordu: %v", code, err)
			}
			if tr := messages.ErrorIn(err, "tr"); !strings.Contains(tr, c.line+". satırı") || !strings.Contains(tr, "`"+call+"`") {
				t.Errorf("%s: ileti satırı ve öneriyi söylemeli: %q", code, tr)
			}
			if en := messages.ErrorIn(err, "en"); !strings.Contains(en, "line "+c.line+" of") || !strings.Contains(en, "`"+call+"`") {
				t.Errorf("%s: ileti satırı ve öneriyi söylemeli: %q", code, en)
			}
		}
		if got := mustRunTR(t, call+`.TR_length`); got != fmt.Sprint(len(c.content)) {
			t.Errorf("%s: önerinin sonucu %s", name, got)
		}
	}
	got := mustRunTR(t, `TR_var o = TR_read_lines("orta.txt")
		[o.TR_next(), TR_error_code { o.TR_next() }, o.TR_next(), o.TR_next()]`)
	if got != `["a", "H2638", "d", none]` {
		t.Errorf("önceki satır verilmiş kalmalı, okuyucu sürmeli: %s", got)
	}
	write(t, filepath.Join(k, "gecerli.txt"), "\xef\xbb\xbfçay\n😀\n")
	if got := mustRunTR(t, `[s TR_for s TR_in TR_read_lines("gecerli.txt")]`); got != `["çay", "😀"]` {
		t.Errorf("geçerli satırlar: %s", got)
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte("a\n\xff\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(k, "b.txt.gz"), gz.String())
	for _, code := range []string{`[s TR_for s TR_in TR_read_lines("b.txt.gz")]`, `TR_read_csv("b.txt.gz")`} {
		err := runTR(t, k, code).failure
		if !messages.Has(err, messages.H2639) {
			t.Fatalf("%s: H2639 bekleniyordu: %v", code, err)
		}
		if text := messages.ErrorIn(err, "en"); strings.Contains(text, "TR_read_bytes(") || !strings.Contains(text, "line 2 of") {
			t.Errorf("%s: .gz'de öneri olmamalı, satır olmalı: %q", code, text)
		}
	}
}

func TestFileLines(t *testing.T) {
	k := tmpWork(t)
	cases := map[string]string{
		"a\nb\nc\n":                `["a", "b", "c"]`,
		"a\r\nb\r\n\r\nson":        `["a", "b", "", "son"]`,
		"\xef\xbb\xbfbaşlık\nveri": `["başlık", "veri"]`,
		"":                         `[]`,
		"\n":                       `[""]`,
		"tek":                      `["tek"]`,
		"a\rb\n":                   `["a\rb"]`,
	}
	i := 0
	for inner, wants := range cases {
		name := fmt.Sprintf("d%d.txt", i)
		i++
		write(t, filepath.Join(k, name), inner)
		if got := mustRunTR(t, `[s TR_for s TR_in TR_read_lines("`+name+`")]`); got != wants {
			t.Errorf("%q: %s, %s bekleniyordu", inner, got, wants)
		}
	}
	// .sonraki() ve döngü aynı akışı tüketir; bitiş bir kez alınır, sonraki istek H7156 verir.
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	if got := mustRunTR(t, `TR_var u = TR_read_lines("s.txt")
		[u.TR_next(), u.TR_next(), u.TR_next(), u.TR_next(), TR_error_code { u.TR_next() }]`); got != `["1", "2", "3", none, "H7156"]` {
		t.Fatal(got)
	}
	got := mustRunTR(t, `TR_var o = TR_read_lines(TR_Path("s.txt"))
		TR_var ilk = o.TR_next()
		TR_var kalan = [s TR_for s TR_in o]
		[ilk, kalan, TR_error_code { o.TR_next() }, TR_error_code { [s TR_for s TR_in o] }, TR_String(o), TR_Type(o) == TR_LineReader]`)
	if got != `["1", ["2", "3"], "H7156", "H7156", "LineReader(s.txt, 3 lines read, finished)", true]` {
		t.Fatal(got)
	}
	errContains(t, `TR_read_lines("yok.txt")`, `dosya_satırları(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`, "bakılan yer")
	os.Mkdir(filepath.Join(k, "kl"), 0o755)
	errHas(t, `TR_read_lines("kl")`, messages.H7129)
	errHas(t, `TR_read_lines("s.txt").TR_next(1)`, messages.H2014)
	errContains(t, `TR_read_lines("s.txt").TR_first`, "SatırOkuyucu üzerinde")
}

func TestFileLinesGzipAndLongLine(t *testing.T) {
	k := tmpWork(t)
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	for i := 0; i < 50000; i++ {
		fmt.Fprintf(gz, `{"no": %d}`+"\n", i)
	}
	gz.Close()
	os.WriteFile(filepath.Join(k, "d.jsonl.gz"), b.Bytes(), 0o644)
	got := mustRunTR(t, `TR_var n = 0
		TR_var top = 0
		TR_for s TR_in TR_read_lines("d.jsonl.gz") { n += 1; top += TR_Int(TR_json_parse(s)["no"]) }
		[n, top]`)
	if got != "[50000, 1249975000]" {
		t.Fatalf("gz: %s", got)
	}
	old := lineLimit
	lineLimit = 100 << 10
	t.Cleanup(func() { lineLimit = old })
	write(t, filepath.Join(k, "uzun.txt"), "kısa\n"+strings.Repeat("x", 300<<10)+"\nson\n")
	errHas(t, `[s TR_for s TR_in TR_read_lines("uzun.txt")]`, messages.H7105)
	if got := mustRunTR(t, `TR_var o = TR_read_lines("uzun.txt")
		[o.TR_next(), TR_error_code { o.TR_next() }, TR_error_code { o.TR_next() }, TR_error_code { [s TR_for s TR_in o] }, TR_String(o), o.TR_close(), TR_error_code { o.TR_next() }]`); got != `["kısa", "H7105", "H7134", "H7134", "LineReader(uzun.txt, 1 line read, closed)", false, "H7134"]` {
		t.Fatalf("H7105 sonrası kapanış: %s", got)
	}
	longErr := runTR(t, "", `TR_var o = TR_read_lines("uzun.txt"); o.TR_next(); TR_try { o.TR_next() } TR_catch e {}; o.TR_next()`).failure
	if !strings.Contains(messages.ErrorIn(longErr, "tr"), "64 MB'ı aşan bir satırdan sonra kapandı") ||
		!strings.Contains(messages.ErrorIn(longErr, "en"), "was closed after a line longer than 64 MB") {
		t.Fatalf("H7134 long dalı: tr=%q en=%q", messages.ErrorIn(longErr, "tr"), messages.ErrorIn(longErr, "en"))
	}
	if err := os.WriteFile(filepath.Join(k, "yarim.jsonl.gz"), b.Bytes()[:len(b.Bytes())/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustRunTR(t, `TR_var o = TR_read_lines("yarim.jsonl.gz")
		TR_var kod = ""
		TR_try { TR_for s TR_in o {} } TR_catch e { kod = e.TR_code }
		[kod, TR_error_code { o.TR_next() }, TR_error_code { [s TR_for s TR_in o] }, o.TR_close()]`); got != `["H2043", "H7134", "H7134", false]` {
		t.Fatalf("okuma hatası sonrası kapanış: %s", got)
	}
	readErr := runTR(t, "", `TR_var o = TR_read_lines("yarim.jsonl.gz"); TR_try { TR_for s TR_in o {} } TR_catch e {}; o.TR_next()`).failure
	if !strings.Contains(messages.ErrorIn(readErr, "tr"), "okuma hatasıyla kapandı") ||
		!strings.Contains(messages.ErrorIn(readErr, "en"), "was closed after a read error") {
		t.Fatalf("H7134 error dalı: tr=%q en=%q", messages.ErrorIn(readErr, "tr"), messages.ErrorIn(readErr, "en"))
	}
	write(t, filepath.Join(k, "orta.txt"), strings.Repeat("y", 90<<10)+"\nson")
	if got := mustRunTR(t, `[s.TR_length TR_for s TR_in TR_read_lines("orta.txt")]`); got != fmt.Sprintf("[%d, 3]", 90<<10) {
		t.Fatalf("tampondan uzun satır: %s", got)
	}
}

// Aynı okuyucuyu gezen eşzamanlı görevler her satırı bir kez alır (kilit diskten okurken
// bırakılır, okuyucunun kendi kilidi satırları sıralar).
func TestFileLinesConcurrent(t *testing.T) {
	k := tmpWork(t)
	var sb strings.Builder
	for i := 0; i < 200000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	write(t, filepath.Join(k, "c.txt"), sb.String())
	got := mustRunTR(t, `TR_var o = TR_read_lines("c.txt")
		TR_var sayılar = [0, 0]
		TR_var kodlar = ["", ""]
		TR_async { TR_for s TR_in o { sayılar[0] += 1 }; kodlar[0] = TR_error_code { o.TR_next() } }
		TR_async { TR_for s TR_in o { sayılar[1] += 1 }; kodlar[1] = TR_error_code { [s TR_for s TR_in o] } }
		TR_wait_all()
		[sayılar[0] + sayılar[1], kodlar]`)
	if got != `[200000, ["H7156", "H7156"]]` {
		t.Fatalf("eşzamanlı okuma: %s", got)
	}
	for idx, tc := range []struct {
		content string
		want    string
	}{
		{"", `[0, ["H7156", "H7156"]]`},
		{"tek\n", `[1, ["H7156", "H7156"]]`},
		{"tek", `[1, ["H7156", "H7156"]]`},
	} {
		name := fmt.Sprintf("kucuk_%d.txt", idx)
		write(t, filepath.Join(k, name), tc.content)
		got := mustRunTR(t, fmt.Sprintf(`TR_var o = TR_read_lines(%q)
			TR_var sayılar = [0, 0]
			TR_var kodlar = ["", ""]
			TR_async { TR_for s TR_in o { sayılar[0] += 1 }; kodlar[0] = TR_error_code { o.TR_next() } }
			TR_async { TR_for s TR_in o { sayılar[1] += 1 }; kodlar[1] = TR_error_code { [s TR_for s TR_in o] } }
			TR_wait_all()
			[sayılar[0] + sayılar[1], kodlar]`, name))
		if got != tc.want {
			t.Fatalf("%s: %s, beklenen %s", name, got, tc.want)
		}
	}
	// Bitişi almış görevin sonradan açtığı görev H7156 alır; bitişi almamış kardeş görevin
	// sonradan açtığı görev ise bir kez bitiş alır (DENETİM 722 Ş1).
	write(t, filepath.Join(k, "miras.txt"), "a\nb\n")
	got = mustRunTR(t, `TR_var o = TR_read_lines("miras.txt")
		TR_var sal = TR_false
		TR_var kardes_alt_boy = -1
		TR_var kardes_alt_kod = ""
		TR_var kardes_none = TR_false
		TR_var kardes_kod = ""
		TR_var bitmis_alt_kodlar = ["", ""]
		TR_var kardes = TR_async {
			TR_for TR_not sal { TR_sleep(0.001) }
			TR_var kardes_alt = TR_async {
				kardes_alt_boy = [s TR_for s TR_in o].TR_length
				kardes_alt_kod = TR_error_code { o.TR_next() }
			}
			kardes_alt.TR_wait()
			kardes_none = o.TR_next() == TR_none
			kardes_kod = TR_error_code { o.TR_next() }
		}
		TR_var okunan = [s TR_for s TR_in o]
		TR_var bitmis_alt = TR_async {
			bitmis_alt_kodlar[0] = TR_error_code { [s TR_for s TR_in o] }
			bitmis_alt_kodlar[1] = TR_error_code { o.TR_next() }
		}
		sal = TR_true
		TR_wait_all([bitmis_alt, kardes])
		[okunan, bitmis_alt_kodlar, [[kardes_alt_boy, kardes_alt_kod], kardes_none, kardes_kod]]`)
	if got != `[["a", "b"], ["H7156", "H7156"], [[0, "H7156"], true, "H7156"]]` {
		t.Fatalf("görev mirası: %s", got)
	}
}

func TestLineReaderTwoByTwoAndNestedLoops(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "cift.txt"), "1\n2\n3\n4\n")
	write(t, filepath.Join(k, "tek.txt"), "1\n2\n3")
	got := mustRunTR(t, `TR_var o1 = TR_read_lines("cift.txt")
		TR_var ikililer1: TR_List[TR_List[TR_String?]] = []
		TR_for a TR_in o1 { ikililer1.TR_append([a, o1.TR_next()]) }
		TR_var o2 = TR_read_lines("tek.txt")
		TR_var ikililer2: TR_List[TR_List[TR_String?]] = []
		TR_for a TR_in o2 { ikililer2.TR_append([a, o2.TR_next()]) }
		[ikililer1, TR_error_code { o1.TR_next() }, ikililer2, TR_error_code { [s TR_for s TR_in o2] }]`)
	if got != `[[["1", "2"], ["3", "4"]], "H7156", [["1", "2"], ["3", none]], "H7156"]` {
		t.Fatalf("ikişer okuma: %s", got)
	}
	write(t, filepath.Join(k, "ic3.txt"), "1\n2\n3\n")
	write(t, filepath.Join(k, "ic1.txt"), "1")
	got = mustRunTR(t, `TR_var o3 = TR_read_lines("ic3.txt")
		TR_var iz3: TR_List[TR_String] = []
		TR_for a TR_in o3 {
			iz3.TR_append("d:" + a)
			TR_for b TR_in o3 { iz3.TR_append("i:" + b) }
		}
		TR_var o4 = TR_read_lines("ic1.txt")
		TR_var iz4: TR_List[TR_String] = []
		TR_for a TR_in o4 {
			iz4.TR_append("d:" + a)
			TR_for b TR_in o4 { iz4.TR_append("i:" + b) }
		}
		[iz3, TR_error_code { o3.TR_next() }, iz4, TR_error_code { [s TR_for s TR_in o4] }]`)
	if got != `[["d:1", "i:2", "i:3"], "H7156", ["d:1"], "H7156"]` {
		t.Fatalf("iç içe döngü: %s", got)
	}
}

// x.kapat() dosyayı sonuna kadar okumadan bırakır: dosya silinebilir (Windows'ta açık dosya
// silinemezdi), kapatılmış okuyucudan satır istemek açık bir hatadır, iki kez kapatmak değildir.
func TestLineReaderClose(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	got := mustRunTR(t, `TR_var o = TR_read_lines("s.txt")
		TR_var ilk = o.TR_next()
		TR_var birinci = o.TR_close()
		TR_var ikinci = o.TR_close()
		[ilk, birinci, ikinci, TR_String(o), TR_remove("s.txt"), TR_Path("s.txt").TR_exists]`)
	if got != `["1", true, false, "LineReader(s.txt, 1 line read, closed)", 1, false]` {
		t.Fatal(got)
	}
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	errContains(t, `TR_var o = TR_read_lines("s.txt"); o.TR_close(); o.TR_next()`,
		"SatırOkuyucu: \"s.txt\" okuyucusu kapat() ile kapatıldı; yeniden okumak için `TR_read_lines(\"s.txt\")` ile yeni bir okuyucu açın")
	errHas(t, `TR_var o = TR_read_lines("s.txt"); o.TR_close(); [s TR_for s TR_in o]`, messages.H7134)
	errContains(t, `TR_read_lines("s.txt").TR_close(1)`, "kapat() parametre almaz")
	// Sonuna kadar okunan okuyucunun dosyası zaten kapalıdır: close() yanlış verir; close()
	// öncesinde satır istemek H7156, sonrasında satır istemek H7134 verir.
	if got := mustRunTR(t, `TR_var o = TR_read_lines("s.txt")
		TR_var l = [s TR_for s TR_in o]
		[l, TR_error_code { o.TR_next() }, o.TR_close()]`); got != `[["1", "2", "3"], "H7156", false]` {
		t.Fatal(got)
	}
	errHas(t, `TR_var o = TR_read_lines("s.txt"); [s TR_for s TR_in o]; o.TR_close(); o.TR_next()`, messages.H7134)
	// Açık okuyucu varken sil yol gösterir: kapat() önerilir.
	errHas(t, `TR_var o = TR_read_lines("s.txt"); o.TR_next(); TR_remove("s.txt")`, messages.H7125)
}

// Son satırı satır sonusuz biten dosyada okuyucu son satırla birlikte kapanır: dosya hemen
// silinebilir (eskiden okuyucu kapatılmayı ya da bir çağrı daha bekliyordu).
func TestLineReaderClosesAtEOFWithoutNewline(t *testing.T) {
	tmpWork(t)
	got := mustRunTR(t, `TR_write_file("son.txt", "a\nb")
TR_var o = TR_read_lines("son.txt")
TR_var x = [o.TR_next(), o.TR_next()]
TR_remove("son.txt")
[x, o.TR_next(), TR_error_code { o.TR_next() }]`)
	if got != `[["a", "b"], none, "H7156"]` {
		t.Fatalf("sonuç: %s", got)
	}
}

// Erken bırakılan okuyucunun dosyası, okuyucuya erişilemez olunca çöp toplayıcıyla kapanır:
// kaynak program bitmesini beklemeden serbest kalır.
func TestLineReaderClosedWhenUnreachable(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	closeReaders()
	if got := mustRunTR(t, `TR_func bırak() -> TR_String? { TR_var o = TR_read_lines("s.txt"); TR_return o.TR_next() }
bırak()
bırak()`); got != `1` {
		t.Fatalf("sonuç: %s", got)
	}
	last := time.Now().Add(10 * time.Second)
	for {
		openReaders.Lock()
		n := len(openReaders.m)
		openReaders.Unlock()
		if n == 0 {
			return
		}
		if time.Now().After(last) {
			t.Fatalf("erişilemez okuyucular kapanmadı: %d açık", n)
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
}

// Kural (B5): sırayla okunan bir değer (SatırOkuyucu) indekslenince ileti nasıl okunacağını
// söyler: döngüyle ya da .sonraki() ile. Önceden yalnız "SatırOkuyucu tipi indekslenemez" diyordu.
func TestLineReaderIndexSaysHowToRead(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "a.txt"), "x\ny\n")
	errHas(t, `TR_read_lines("a.txt")[1]`, messages.H2525)
	errContains(t, `TR_read_lines("a.txt")[1]`, "döngü", ".sonraki()")
}

func extractBacktickCode(t *testing.T, msg string) string {
	t.Helper()
	parts := strings.Split(msg, "`")
	if len(parts) != 3 || parts[1] == "" {
		t.Fatalf("iletide tek ters tırnaklı kod bekleniyordu: %q", msg)
	}
	return parts[1]
}

type suggestionCase struct {
	name     string
	trigger  string
	wantCode messages.Code
	prepare  func()
	wrap     func(expr string) string
	want     string
}

func runSuggestionCase(t *testing.T, k string, tc suggestionCase) {
	t.Helper()
	err := runTR(t, k, tc.trigger).failure
	if !messages.Has(err, tc.wantCode) {
		t.Fatalf("%s: beklenen %s, gelen: %v", tc.name, tc.wantCode, err)
	}
	trSpell := extractBacktickCode(t, messages.ErrorIn(err, "tr"))
	enSpell := extractBacktickCode(t, messages.ErrorIn(err, "en"))
	caught := mustRunTR(t, `TR_var m: TR_List[TR_String] = []; TR_try { `+tc.trigger+` } TR_catch e { m = [e.TR_text("tr"), e.TR_text("en")] }; m.TR_join("\n")`)
	for _, line := range strings.Split(caught, "\n") {
		if gotSpell := extractBacktickCode(t, line); gotSpell != trSpell {
			t.Fatalf("%s: yakalanan hata önerisi farklı: %q != %q", tc.name, gotSpell, trSpell)
		}
	}
	if trSpell != enSpell {
		t.Fatalf("%s: öneriler aynı olmalı: tr=%q en=%q", tc.name, trSpell, enSpell)
	}
	if tc.prepare != nil {
		tc.prepare()
	}
	if got := mustRunTR(t, "# TR 4\n"+tc.wrap(trSpell)); got != tc.want {
		t.Fatalf("%s: öneri %q %q verdi, beklenen %s", tc.name, trSpell, got, tc.want)
	}
}

// H7105, H7134 (long, error ve other dalları) ve H7156 iletilerinin ters tırnaklı önerisi
// hem tr/en hata çıktısından hem yakalanan TR_Error metninden çıkarılıp programa konduğunda çalışır.
func TestLineReaderSuggestionsRun(t *testing.T) {
	k := tmpWork(t)
	old := lineLimit
	lineLimit = 100 << 10
	t.Cleanup(func() { lineLimit = old })

	rawRel, trArg := "alt/a\"b\\c.txt", `"alt/a\"b\\c.txt"`
	if runtime.GOOS == "windows" {
		rawRel, trArg = `alt\ozel.txt`, `"alt\\ozel.txt"`
	}
	longContent := "ilk\n" + strings.Repeat("z", 250<<10) + "\nson\n"
	write(t, filepath.Join(k, "alt", filepath.Base(rawRel)), longContent)
	write(t, filepath.Join(k, "kisa.txt"), "bir\niki\n")

	var gzBuf bytes.Buffer
	gzw := gzip.NewWriter(&gzBuf)
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(gzw, "satir_%d\n", i)
	}
	_ = gzw.Close()
	fullGz := gzBuf.Bytes()
	if err := os.WriteFile(filepath.Join(k, "bozuk.txt.gz"), fullGz[:len(fullGz)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []suggestionCase{
		{"H7105", `[s TR_for s TR_in TR_read_lines(` + trArg + `)]`, messages.H7105, nil, func(e string) string { return e + `.TR_length` }, fmt.Sprint(len(longContent))},
		{"H7134_long", `TR_var o = TR_read_lines(` + trArg + `); o.TR_next(); TR_try { o.TR_next() } TR_catch e {}; o.TR_next()`, messages.H7134, nil, func(e string) string { return e + `.TR_next()` }, `ilk`},
		{"H7134_error", `TR_var o = TR_read_lines("bozuk.txt.gz"); TR_try { TR_for s TR_in o {} } TR_catch e {}; o.TR_next()`, messages.H7134, func() { _ = os.WriteFile(filepath.Join(k, "bozuk.txt.gz"), fullGz, 0o644) }, func(e string) string { return e + `.TR_next()` }, `satir_0`},
		{"H7134_close", `TR_var o = TR_read_lines("kisa.txt"); o.TR_close(); o.TR_next()`, messages.H7134, nil, func(e string) string { return `[s TR_for s TR_in ` + e + `]` }, `["bir", "iki"]`},
		{"H7156", `TR_var o = TR_read_lines("kisa.txt"); [s TR_for s TR_in o]; o.TR_next()`, messages.H7156, nil, func(e string) string { return `[s TR_for s TR_in ` + e + `]` }, `["bir", "iki"]`},
	} {
		runSuggestionCase(t, k, tc)
	}
	verifyQuotedPathEscapeInMessages(t)
}

func verifyQuotedPathEscapeInMessages(t *testing.T) {
	t.Helper()
	tricky := `kl\a"b.txt`
	for _, err := range []error{
		messages.New(messages.H7105, 2, tricky),
		messages.New(messages.H7134, LineReaderType, tricky, "long"),
		messages.New(messages.H7134, LineReaderType, tricky, "error"),
		messages.New(messages.H7134, LineReaderType, tricky, "close"),
		messages.New(messages.H7156, LineReaderType, tricky),
	} {
		for _, lang := range []string{"tr", "en"} {
			spell := extractBacktickCode(t, messages.ErrorIn(err, lang))
			argExpr := spell[strings.IndexByte(spell, '(')+1 : len(spell)-1]
			if got := mustRunTR(t, "# TR 4\n"+argExpr); got != tricky {
				t.Fatalf("%s (%s): kaçışlı argüman %q -> %q, beklenen %q", spell, lang, argExpr, got, tricky)
			}
		}
	}
}
