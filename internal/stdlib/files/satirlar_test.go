package files

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDosyaSatirlari(t *testing.T) {
	k := geciciCalisma(t)
	durumlar := map[string]string{
		"a\nb\nc\n":                `["a", "b", "c"]`,
		"a\r\nb\r\n\r\nson":        `["a", "b", "", "son"]`,
		"\xef\xbb\xbfbaşlık\nveri": `["başlık", "veri"]`,
		"":                         `[]`,
		"\n":                       `[""]`,
		"tek":                      `["tek"]`,
		"a\rb\n":                   `["a\rb"]`,
	}
	i := 0
	for ic, ister := range durumlar {
		ad := fmt.Sprintf("d%d.txt", i)
		i++
		yaz(t, filepath.Join(k, ad), ic)
		if got := tamam(t, `[s döngü s içinde dosya_satırları("`+ad+`")]`); got != ister {
			t.Errorf("%q: %s, %s bekleniyordu", ic, got, ister)
		}
	}
	// .sonraki() ve döngü aynı akışı tüketir; bitince yok verir.
	yaz(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	got := tamam(t, `değişken o = dosya_satırları(yol("s.txt"))
		değişken ilk = o.sonraki()
		değişken kalan = [s döngü s içinde o]
		[ilk, kalan, o.sonraki(), metin(o), tip(o) == SatırOkuyucu]`)
	if got != `["1", ["2", "3"], yok, "SatırOkuyucu(s.txt, 3 satır okundu, bitti)", doğru]` {
		t.Fatal(got)
	}
	hataIcerir(t, `dosya_satırları("yok.txt")`, `dosya_satırları(): "yok.txt" okunamadı: böyle bir dosya ya da klasör yok`, "bakılan yer")
	os.Mkdir(filepath.Join(k, "kl"), 0o755)
	hataIcerir(t, `dosya_satırları("kl")`, "bir klasör")
	hataIcerir(t, `dosya_satırları("s.txt").sonraki(1)`, "parametre almaz")
	hataIcerir(t, `dosya_satırları("s.txt").ilk`, "SatırOkuyucu üzerinde")
}

func TestDosyaSatirlariGzipVeUzunSatir(t *testing.T) {
	k := geciciCalisma(t)
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	for i := 0; i < 50000; i++ {
		fmt.Fprintf(gz, `{"no": %d}`+"\n", i)
	}
	gz.Close()
	os.WriteFile(filepath.Join(k, "d.jsonl.gz"), b.Bytes(), 0o644)
	got := tamam(t, `değişken n = 0
		değişken top = 0
		döngü s içinde dosya_satırları("d.jsonl.gz") { n += 1; top += json_çöz(s)["no"] }
		[n, top]`)
	if got != "[50000, 1249975000]" {
		t.Fatalf("gz: %s", got)
	}
	eski := satirSiniri
	satirSiniri = 100 << 10
	t.Cleanup(func() { satirSiniri = eski })
	yaz(t, filepath.Join(k, "uzun.txt"), "kısa\n"+strings.Repeat("x", 300<<10)+"\nson\n")
	hataIcerir(t, `[s döngü s içinde dosya_satırları("uzun.txt")]`, "2. satır 64 MB'tan uzun")
	yaz(t, filepath.Join(k, "orta.txt"), strings.Repeat("y", 90<<10)+"\nson")
	if got := tamam(t, `[s.uzunluk döngü s içinde dosya_satırları("orta.txt")]`); got != fmt.Sprintf("[%d, 3]", 90<<10) {
		t.Fatalf("tampondan uzun satır: %s", got)
	}
}

// Aynı okuyucuyu gezen eşzamanlı görevler her satırı bir kez alır (kilit diskten okurken
// bırakılır, okuyucunun kendi kilidi satırları sıralar).
func TestDosyaSatirlariEszamanli(t *testing.T) {
	k := geciciCalisma(t)
	var sb strings.Builder
	for i := 0; i < 200000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	yaz(t, filepath.Join(k, "c.txt"), sb.String())
	got := tamam(t, `değişken o = dosya_satırları("c.txt")
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
func TestSatirOkuyucuKapat(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	got := tamam(t, `değişken o = dosya_satırları("s.txt")
		değişken ilk = o.sonraki()
		değişken birinci = o.kapat()
		değişken ikinci = o.kapat()
		[ilk, birinci, ikinci, metin(o), sil("s.txt"), yol("s.txt").var_mı]`)
	if got != `["1", doğru, yanlış, "SatırOkuyucu(s.txt, 1 satır okundu, kapatıldı)", 1, yanlış]` {
		t.Fatal(got)
	}
	yaz(t, filepath.Join(k, "s.txt"), "1\n2\n3\n")
	hataIcerir(t, `değişken o = dosya_satırları("s.txt"); o.kapat(); o.sonraki()`,
		`SatırOkuyucu: "s.txt" okuyucusu kapat() ile kapatıldı; yeniden okumak için dosya_satırları(...) ile yeni bir okuyucu açın`)
	hataIcerir(t, `değişken o = dosya_satırları("s.txt"); o.kapat(); [s döngü s içinde o]`, "kapat() ile kapatıldı")
	hataIcerir(t, `dosya_satırları("s.txt").kapat(1)`, "kapat() parametre almaz")
	// Sonuna kadar okunan okuyucunun dosyası zaten kapalıdır: kapat() yanlış verir; kapat()
	// çağrıldıktan sonra satır istemek yine hatadır.
	if got := tamam(t, `değişken o = dosya_satırları("s.txt")
		değişken l = [s döngü s içinde o]
		[l, o.sonraki(), o.kapat()]`); got != `[["1", "2", "3"], yok, yanlış]` {
		t.Fatal(got)
	}
	hataIcerir(t, `değişken o = dosya_satırları("s.txt"); [s döngü s içinde o]; o.kapat(); o.sonraki()`, "kapat() ile kapatıldı")
	// Açık okuyucu varken sil yol gösterir: kapat() önerilir.
	hataIcerir(t, `değişken o = dosya_satırları("s.txt"); o.sonraki(); sil("s.txt")`, "önce okuyucuyu kapatın (x.kapat())")
}
