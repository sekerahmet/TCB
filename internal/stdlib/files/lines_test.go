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
		if got := mustRunTR(t, `[s döngü s içinde dosya_satırları("`+name+`")]`); got != wants {
			t.Errorf("%q: %s, %s bekleniyordu", inner, got, wants)
		}
	}
	// .sonraki() ve döngü aynı akışı tüketir; bitince yok verir.
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	got := mustRunTR(t, `değişken o = dosya_satırları(yol("s.txt"))
		değişken ilk = o.sonraki()
		değişken kalan = [s döngü s içinde o]
		[ilk, kalan, o.sonraki(), metin(o), tip(o) == SatırOkuyucu]`)
	if got != `["1", ["2", "3"], yok, "SatırOkuyucu(s.txt, 3 satır okundu, bitti)", doğru]` {
		t.Fatal(got)
	}
	errContains(t, `dosya_satırları("yok.txt")`, `dosya_satırları(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`, "bakılan yer")
	os.Mkdir(filepath.Join(k, "kl"), 0o755)
	errHas(t, `dosya_satırları("kl")`, messages.H7129)
	errHas(t, `dosya_satırları("s.txt").sonraki(1)`, messages.H2014)
	errContains(t, `dosya_satırları("s.txt").ilk`, "SatırOkuyucu üzerinde")
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
	got := mustRunTR(t, `değişken n = 0
		değişken top = 0
		döngü s içinde dosya_satırları("d.jsonl.gz") { n += 1; top += json_çöz(s)["no"] }
		[n, top]`)
	if got != "[50000, 1249975000]" {
		t.Fatalf("gz: %s", got)
	}
	old := lineLimit
	lineLimit = 100 << 10
	t.Cleanup(func() { lineLimit = old })
	write(t, filepath.Join(k, "uzun.txt"), "kısa\n"+strings.Repeat("x", 300<<10)+"\nson\n")
	errHas(t, `[s döngü s içinde dosya_satırları("uzun.txt")]`, messages.H7105)
	write(t, filepath.Join(k, "orta.txt"), strings.Repeat("y", 90<<10)+"\nson")
	if got := mustRunTR(t, `[s.uzunluk döngü s içinde dosya_satırları("orta.txt")]`); got != fmt.Sprintf("[%d, 3]", 90<<10) {
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
	got := mustRunTR(t, `değişken o = dosya_satırları("c.txt")
		değişken sayılar = [0, 0]
		eşzamanlı { döngü s içinde o { sayılar[0] += 1 } }
		eşzamanlı { döngü s içinde o { sayılar[1] += 1 } }
		bekle_hepsi()
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
	got := mustRunTR(t, `değişken o = dosya_satırları("s.txt")
		değişken ilk = o.sonraki()
		değişken birinci = o.kapat()
		değişken ikinci = o.kapat()
		[ilk, birinci, ikinci, metin(o), sil("s.txt"), yol("s.txt").var_mı]`)
	if got != `["1", doğru, yanlış, "SatırOkuyucu(s.txt, 1 satır okundu, kapatıldı)", 1, yanlış]` {
		t.Fatal(got)
	}
	write(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	errContains(t, `değişken o = dosya_satırları("s.txt"); o.kapat(); o.sonraki()`,
		`SatırOkuyucu: "s.txt" okuyucusu kapat() ile kapatıldı; yeniden okumak için dosya_satırları(...) ile yeni bir okuyucu açın`)
	errHas(t, `değişken o = dosya_satırları("s.txt"); o.kapat(); [s döngü s içinde o]`, messages.H7134)
	errContains(t, `dosya_satırları("s.txt").kapat(1)`, "kapat() parametre almaz")
	// Sonuna kadar okunan okuyucunun dosyası zaten kapalıdır: close() yanlış verir; close()
	// çağrıldıktan sonra satır istemek yine hatadır.
	if got := mustRunTR(t, `değişken o = dosya_satırları("s.txt")
		değişken l = [s döngü s içinde o]
		[l, o.sonraki(), o.kapat()]`); got != `[["1", "2", "3"], yok, yanlış]` {
		t.Fatal(got)
	}
	errHas(t, `değişken o = dosya_satırları("s.txt"); [s döngü s içinde o]; o.kapat(); o.sonraki()`, messages.H7134)
	// Açık okuyucu varken sil yol gösterir: kapat() önerilir.
	errHas(t, `değişken o = dosya_satırları("s.txt"); o.sonraki(); sil("s.txt")`, messages.H7125)
}

// Son satırı satır sonusuz biten dosyada okuyucu son satırla birlikte kapanır: dosya hemen
// silinebilir (eskiden okuyucu kapatılmayı ya da bir çağrı daha bekliyordu).
func TestLineReaderClosesAtEOFWithoutNewline(t *testing.T) {
	tmpWork(t)
	got := mustRunTR(t, `dosya_yaz("son.txt", "a\nb")
değişken o = dosya_satırları("son.txt")
değişken x = [o.sonraki(), o.sonraki()]
sil("son.txt")
[x, o.sonraki()]`)
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
	if got := mustRunTR(t, `fonksiyon bırak() { değişken o = dosya_satırları("s.txt"); o.sonraki() }
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
	errHas(t, `dosya_satırları("a.txt")[1]`, messages.H2525)
	errContains(t, `dosya_satırları("a.txt")[1]`, "döngü", ".sonraki()")
}
