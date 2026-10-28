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
	// .sonraki() ve döngü aynı akışı tüketir; bitince yok verir.
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	got := mustRunTR(t, `TR_var o = TR_read_lines(TR_path("s.txt"))
		TR_var ilk = o.TR_next()
		TR_var kalan = [s TR_for s TR_in o]
		[ilk, kalan, o.TR_next(), TR_str(o), TR_type(o) == TR_LineReader]`)
	if got != `["1", ["2", "3"], yok, "SatırOkuyucu(s.txt, 3 satır okundu, bitti)", doğru]` {
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
		TR_for s TR_in TR_read_lines("d.jsonl.gz") { n += 1; top += TR_json_parse(s)["no"] }
		[n, top]`)
	if got != "[50000, 1249975000]" {
		t.Fatalf("gz: %s", got)
	}
	old := lineLimit
	lineLimit = 100 << 10
	t.Cleanup(func() { lineLimit = old })
	write(t, filepath.Join(k, "uzun.txt"), "kısa\n"+strings.Repeat("x", 300<<10)+"\nson\n")
	errHas(t, `[s TR_for s TR_in TR_read_lines("uzun.txt")]`, messages.H7105)
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
		TR_async { TR_for s TR_in o { sayılar[0] += 1 } }
		TR_async { TR_for s TR_in o { sayılar[1] += 1 } }
		TR_wait_all()
		sayılar[0] + sayılar[1]`)
	if got != "200000" {
		t.Fatalf("eşzamanlı okuma: %s", got)
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
		[ilk, birinci, ikinci, TR_str(o), TR_remove("s.txt"), TR_path("s.txt").TR_exists]`)
	if got != `["1", doğru, yanlış, "SatırOkuyucu(s.txt, 1 satır okundu, kapatıldı)", 1, yanlış]` {
		t.Fatal(got)
	}
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	errContains(t, `TR_var o = TR_read_lines("s.txt"); o.TR_close(); o.TR_next()`,
		`SatırOkuyucu: "s.txt" okuyucusu kapat() ile kapatıldı; yeniden okumak için dosya_satırları(...) ile yeni bir okuyucu açın`)
	errHas(t, `TR_var o = TR_read_lines("s.txt"); o.TR_close(); [s TR_for s TR_in o]`, messages.H7134)
	errContains(t, `TR_read_lines("s.txt").TR_close(1)`, "kapat() parametre almaz")
	// Sonuna kadar okunan okuyucunun dosyası zaten kapalıdır: close() yanlış verir; close()
	// çağrıldıktan sonra satır istemek yine hatadır.
	if got := mustRunTR(t, `TR_var o = TR_read_lines("s.txt")
		TR_var l = [s TR_for s TR_in o]
		[l, o.TR_next(), o.TR_close()]`); got != `[["1", "2", "3"], yok, yanlış]` {
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
[x, o.TR_next()]`)
	if got != `[["a", "b"], yok]` {
		t.Fatalf("sonuç: %s", got)
	}
}

// Erken bırakılan okuyucunun dosyası, okuyucuya erişilemez olunca çöp toplayıcıyla kapanır:
// kaynak program bitmesini beklemeden serbest kalır.
func TestLineReaderClosedWhenUnreachable(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	closeReaders()
	if got := mustRunTR(t, `TR_func bırak() { TR_var o = TR_read_lines("s.txt"); o.TR_next() }
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
