package interp

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// modulluCalistir klasör altına dosyaları yazar, çalışma klasörünü kok yapar ve ana.tr'yi çalıştırır.
func modulluCalistir(t *testing.T, kok string, dosyalar map[string]string) (string, error) {
	t.Helper()
	for ad, icerik := range dosyalar {
		dosyaYaz(t, filepath.Join(kok, filepath.FromSlash(ad)), icerik)
	}
	kod := dosyalar["ana.tr"]
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("ana.tr sözdizimi: %v", p.Errors())
	}
	var out bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &out
	m.SourceFile = filepath.Join(kok, "ana.tr")
	m.SourceCode = kod
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// Modülün üst düzeyindeki 'sonuç' aktaran programı sessizce bitiriyordu (çıkış kodu 0).
// Derleme hatasıdır: modülün hiçbir satırı çalışmaz, aktar hata verir.
func TestModulUstDuzeySonucHata(t *testing.T) {
	kok := t.TempDir()
	klasordeCalis(t, kok)
	out, err := modulluCalistir(t, kok, map[string]string{
		"m1.tr":  "yazdır(\"modül çalıştı\")\nsonuç 5\n",
		"ana.tr": "yazdır(\"başla\")\naktar \"m1\"\nyazdır(\"aktardıktan sonra\")\n",
	})
	if err == nil || !strings.Contains(err.Error(), "'sonuç' modülün üst düzeyinde kullanılamaz") {
		t.Fatalf("modül hatası bekleniyordu: %v\n%s", err, out)
	}
	if !strings.Contains(err.Error(), "m1.tr, Satır 2") || strings.Contains(out, "modül çalıştı") || strings.Contains(out, "aktardıktan") {
		t.Fatalf("hata m1.tr'nin satırını göstermeli, modül çalışmamalı: %v\n%s", err, out)
	}
}

// Modüller tip denetiminden geçmeden çalışıyordu (docs/spec/01_program.md §1 ile çelişki).
func TestModulTipDenetimindenGecer(t *testing.T) {
	kok := t.TempDir()
	klasordeCalis(t, kok)
	out, err := modulluCalistir(t, kok, map[string]string{
		"m5.tr":  "değişken t: Tam = \"metin\"\nfonksiyon f(a: Tam): Tam { sonuç \"yanlış tip\" }\nyazdır(\"m5 çalıştı\")\n",
		"ana.tr": "aktar \"m5\"\n",
	})
	if err == nil {
		t.Fatalf("tip hatalı modül çalıştı:\n%s", out)
	}
	mesaj := err.Error()
	for _, ister := range []string{"Satır 1:", "\"m5\" modülü tip denetimi hataları nedeniyle yüklenemedi", "m5.tr, Satır 1: Tip uyuşmazlığı", "m5.tr, Satır 2: Dönüş tipi uyuşmazlığı"} {
		if !strings.Contains(mesaj, ister) {
			t.Errorf("hatada %q yok: %s", ister, mesaj)
		}
	}
	if strings.Contains(out, "m5 çalıştı") {
		t.Error("tip hatalı modülün hiçbir satırı çalışmamalı")
	}
	// Maddeler: aktar satırı ana dosyadan, her tip hatası modülün kaynağından çerçevelenir.
	maddeler := ErrorItems(err, "aktar \"m5\"\n")
	if len(maddeler) != 3 || !strings.Contains(maddeler[0].Frame, "1 | aktar \"m5\"") ||
		!strings.Contains(maddeler[1].Frame, "1 | değişken t: Tam = \"metin\"") ||
		!strings.Contains(maddeler[2].Frame, "2 | fonksiyon f(a: Tam)") {
		t.Fatalf("maddeler yanlış: %+v", maddeler)
	}
}

// Modüldeki çalışma hatası ana programın aynı numaralı satırını gösteriyordu.
func TestModulCalismaHatasiKendiDosyasiyla(t *testing.T) {
	kok := t.TempDir()
	klasordeCalis(t, kok)
	_, err := modulluCalistir(t, kok, map[string]string{
		"m2.tr":  "değişken x = 1\n\ndeğişken z = x / 0\n",
		"ana.tr": "yazdır(1)\n\naktar \"m2\"\n",
	})
	var mh *ModuleError
	if err == nil || !errors.As(err, &mh) || !strings.HasPrefix(err.Error(), "m2.tr, Satır 3: Sıfıra bölme hatası") {
		t.Fatalf("m2.tr'nin satırı bekleniyordu: %v", err)
	}
	m := ErrorItems(err, "yazdır(1)\n\naktar \"m2\"\n")
	if len(m) != 1 || !strings.Contains(m[0].Frame, "3 | değişken z = x / 0") {
		t.Fatalf("çerçeve modülden çizilmeli: %+v", m)
	}

	// Modülde tanımlı fonksiyonun sonradan (ana programdan) çağrısındaki hata da öyle.
	kok2 := t.TempDir()
	klasordeCalis(t, kok2)
	_, err = modulluCalistir(t, kok2, map[string]string{
		"lib.tr": "fonksiyon böl(a) {\n    sonuç a / 0\n}\n",
		"ana.tr": "aktar \"lib\"\nyazdır(\"x\")\nböl(1)\n",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "lib.tr, Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("lib.tr'nin satırı bekleniyordu: %v", err)
	}
	// Modülün geri çağırdığı ana program fonksiyonunun hatası ana dosyaya ait kalır.
	_, err = modulluCalistir(t, kok2, map[string]string{
		"uygula.tr": "fonksiyon uygula(f) {\n    sonuç f()\n}\n",
		"ana.tr":    "aktar \"uygula\"\nfonksiyon g() {\n    sonuç 1 / 0\n}\nuygula(g)\n",
	})
	if err == nil || !strings.HasSuffix(strings.SplitN(err.Error(), ",", 2)[0], "ana.tr") || !strings.Contains(err.Error(), "Satır 3: Sıfıra bölme") {
		t.Fatalf("hata ana.tr'nin 3. satırına ait olmalı: %v", err)
	}
}

// Eşzamanlı iki görev aynı modülü aktarınca ikincisi sahte "döngüsel aktarım" hatası
// alıyordu; ikincisi birincinin yüklemesini bekler, modül bir kez çalışır.
func TestEsZamanliAyniModul(t *testing.T) {
	kok := t.TempDir()
	klasordeCalis(t, kok)
	out, err := modulluCalistir(t, kok, map[string]string{
		"m3.tr":  "yazdır(\"m3 yükleniyor\")\nbekle(0.1)\ndeğişken hazır = doğru\n",
		"ana.tr": "eşzamanlı { aktar \"m3\"; yazdır(\"A\", hazır) }\neşzamanlı { aktar \"m3\"; yazdır(\"B\", hazır) }\nbekle_hepsi()\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "döngüsel") || !strings.Contains(out, "A doğru") || !strings.Contains(out, "B doğru") || strings.Count(out, "m3 yükleniyor") != 1 {
		t.Fatalf("iki görev de modülü görmeli, modül bir kez çalışmalı:\n%s", out)
	}
	// Gerçek döngü yine hatadır.
	_, err = modulluCalistir(t, kok, map[string]string{
		"a.tr":   "aktar \"b\"\n",
		"b.tr":   "aktar \"a\"\n",
		"ana.tr": "aktar \"a\"\n",
	})
	if err == nil || !strings.Contains(err.Error(), "döngüsel aktarım") {
		t.Fatalf("döngüsel aktarım hatası bekleniyordu: %v", err)
	}
}

// Modül çalışırken paylaşılan SourceFile değişiyor, başka bir görevin göreli yolları modülün
// klasörüne göre çözülüyordu. Kaynak dosya göreve ve çağrı çerçevesine aittir.
func TestKaynakDosyaGoreveAit(t *testing.T) {
	kok := t.TempDir()
	calisma := filepath.Join(kok, "C")
	dosyaYaz(t, filepath.Join(calisma, "bos.txt"), "")
	klasordeCalis(t, calisma)
	ana := filepath.Join(kok, "A")
	dosyalar := map[string]string{
		"veri.txt":             "ana veri",
		"alt/mod.tr":           "bekle(0.2)\nfonksiyon oku() {\n    sonuç dosya_oku(\"modul_verisi.txt\")\n}\n",
		"alt/modul_verisi.txt": "modül verisi",
		"ana.tr":               "eşzamanlı { bekle(0.05); yazdır(dosya_oku(\"veri.txt\")) }\naktar \"alt/mod\"\nbekle_hepsi()\nyazdır(oku())\n",
	}
	out, err := modulluCalistir(t, ana, dosyalar)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "ana veri") || !strings.Contains(out, "modül verisi") || strings.Contains(out, "okunamadı") {
		t.Fatalf("görev ana programın, modül fonksiyonu modülün klasörünü kullanmalı:\n%s", out)
	}
}
