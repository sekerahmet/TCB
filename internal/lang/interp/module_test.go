package interp

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// runWithModules klasör altına dosyaları yazar, çalışma klasörünü kok yapar ve ana.tr'yi çalıştırır.
func runWithModules(t *testing.T, root string, files map[string]string) (string, error) {
	t.Helper()
	for name, content := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	codeVal := files["ana.tr"]
	p := parser.New(lexer.New(codeVal))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("ana.tr sözdizimi: %v", p.Errors())
	}
	var out bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &out
	m.SourceFile = filepath.Join(root, "ana.tr")
	m.SourceCode = codeVal
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// Modülün üst düzeyindeki 'sonuç' aktaran programı sessizce bitiriyordu (çıkış kodu 0).
// Derleme hatasıdır: modülün hiçbir satırı çalışmaz, aktar hata verir.
func TestModuleTopLevelReturnError(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
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
func TestModulePassesTypeCheck(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
		"m5.tr":  "değişken t: Tam = \"metin\"\nfonksiyon f(a: Tam): Tam { sonuç \"yanlış tip\" }\nyazdır(\"m5 çalıştı\")\n",
		"ana.tr": "aktar \"m5\"\n",
	})
	if err == nil {
		t.Fatalf("tip hatalı modül çalıştı:\n%s", out)
	}
	message := err.Error()
	for _, wantSub := range []string{"Satır 1:", "\"m5\" modülü tip denetimi hataları nedeniyle yüklenemedi", "m5.tr, Satır 1: Tip uyuşmazlığı", "m5.tr, Satır 2: Dönüş tipi uyuşmazlığı"} {
		if !strings.Contains(message, wantSub) {
			t.Errorf("hatada %q yok: %s", wantSub, message)
		}
	}
	if strings.Contains(out, "m5 çalıştı") {
		t.Error("tip hatalı modülün hiçbir satırı çalışmamalı")
	}
	// Maddeler: aktar satırı ana dosyadan, her tip hatası modülün kaynağından çerçevelenir.
	entries := ErrorItems(err, "aktar \"m5\"\n")
	if len(entries) != 3 || !strings.Contains(entries[0].Frame, "1 | aktar \"m5\"") ||
		!strings.Contains(entries[1].Frame, "1 | değişken t: Tam = \"metin\"") ||
		!strings.Contains(entries[2].Frame, "2 | fonksiyon f(a: Tam)") {
		t.Fatalf("maddeler yanlış: %+v", entries)
	}
}

// Modüldeki çalışma hatası ana programın aynı numaralı satırını gösteriyordu.
func TestModuleRuntimeErrorWithOwnFile(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{
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
	root2 := t.TempDir()
	runInDir(t, root2)
	_, err = runWithModules(t, root2, map[string]string{
		"lib.tr": "fonksiyon böl(a) {\n    sonuç a / 0\n}\n",
		"ana.tr": "aktar \"lib\"\nyazdır(\"x\")\nböl(1)\n",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "lib.tr, Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("lib.tr'nin satırı bekleniyordu: %v", err)
	}
	// Modülün geri çağırdığı ana program fonksiyonunun hatası ana dosyaya ait kalır.
	_, err = runWithModules(t, root2, map[string]string{
		"uygula.tr": "fonksiyon uygula(f) {\n    sonuç f()\n}\n",
		"ana.tr":    "aktar \"uygula\"\nfonksiyon g() {\n    sonuç 1 / 0\n}\nuygula(g)\n",
	})
	if err == nil || !strings.HasSuffix(strings.SplitN(err.Error(), ",", 2)[0], "ana.tr") || !strings.Contains(err.Error(), "Satır 3: Sıfıra bölme") {
		t.Fatalf("hata ana.tr'nin 3. satırına ait olmalı: %v", err)
	}
}

// Eşzamanlı iki görev aynı modülü aktarınca ikincisi sahte "döngüsel aktarım" hatası
// alıyordu; ikincisi birincinin yüklemesini bekler, modül bir kez çalışır.
func TestConcurrentSameModule(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
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
	_, err = runWithModules(t, root, map[string]string{
		"a.tr":   "aktar \"b\"\n",
		"b.tr":   "aktar \"a\"\n",
		"ana.tr": "aktar \"a\"\n",
	})
	if !messages.Has(err, messages.H2481) && !messages.Has(err, messages.H2483) {
		t.Fatalf("döngüsel aktarım hatası bekleniyordu: %v", err)
	}
}

// Modül çalışırken paylaşılan SourceFile değişiyor, başka bir görevin göreli yolları modülün
// klasörüne göre çözülüyordu. Kaynak dosya göreve ve çağrı çerçevesine aittir.
func TestSourceFileBelongsToTask(t *testing.T) {
	root := t.TempDir()
	execution := filepath.Join(root, "C")
	writeFile(t, filepath.Join(execution, "bos.txt"), "")
	runInDir(t, execution)
	mainSrc := filepath.Join(root, "A")
	files := map[string]string{
		"veri.txt":             "ana veri",
		"alt/mod.tr":           "bekle(0.2)\nfonksiyon oku() {\n    sonuç dosya_oku(\"modul_verisi.txt\")\n}\n",
		"alt/modul_verisi.txt": "modül verisi",
		"ana.tr":               "eşzamanlı { bekle(0.05); yazdır(dosya_oku(\"veri.txt\")) }\naktar \"alt/mod\"\nbekle_hepsi()\nyazdır(oku())\n",
	}
	out, err := runWithModules(t, mainSrc, files)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "ana veri") || !strings.Contains(out, "modül verisi") || strings.Contains(out, "okunamadı") {
		t.Fatalf("görev ana programın, modül fonksiyonu modülün klasörünü kullanmalı:\n%s", out)
	}
}

// A cycle across two tasks: task A loads x, which imports y; task B loads y, which imports x.
// Each waited for the other forever; a single task already got a clear error. The second task
// to close the cycle now gets the same error instead of waiting.
func TestConcurrentImportCycle(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	done := make(chan string, 1)
	go func() {
		out, err := runWithModules(t, root, map[string]string{
			"x.tr":   "bekle(0.1)\naktar \"y\"\n",
			"y.tr":   "bekle(0.1)\naktar \"x\"\n",
			"ana.tr": "eşzamanlı { aktar \"x\" }\neşzamanlı { aktar \"y\" }\nbekle_hepsi()\n",
		})
		if err != nil {
			out += err.Error()
		}
		done <- out
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "döngüsel aktarım") {
			t.Fatalf("döngüsel aktarım hatası bekleniyordu:\n%s", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("iki görev arasındaki döngüsel aktarım sonsuza kadar bekliyor")
	}
}

// Fonksiyon gövdesindeki 'aktar' ve ortam havuzu: biten çağrının ortamı havuza konup
// sonraki çağrıda yeniden kullanılır. Aktarılan ad (takma adlı ya da takma adsız) bir
// çağrıdan sonrakine sızmamalı: her çağrıda 'aktar'dan önce okunan ad tanımsızdır.
func TestImportInFunctionDoesNotLeakBetweenCalls(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
		"m.tr": "değişken ad = \"m\"\n",
		"ana.tr": `
fonksiyon takma(i) {
    değişken önce = "yok"
    dene { önce = mod.ad } yakala { }
    aktar "m" olarak mod
    sonuç önce + "/" + mod.ad
}
fonksiyon takmasız(i) {
    değişken önce = "yok"
    dene { önce = ad } yakala { }
    aktar "m"
    sonuç önce + "/" + ad
}
fonksiyon özyinelemeli(n) {
    değişken önce = "yok"
    dene { önce = mod.ad } yakala { }
    aktar "m" olarak mod
    eğer n > 0 { sonuç önce + "," + özyinelemeli(n - 1) }
    sonuç önce
}
değişken farklı = []
döngü i içinde 1..=200 {
    döngü s içinde [takma(i), takmasız(i)] {
        eğer s != "yok/m" { farklı.ekle("{i}: {s}") }
    }
}
yazdır("farklı: {farklı}")
yazdır(özyinelemeli(5))
yazdır(özyinelemeli(3))
dene { yazdır(mod) } yakala h { yazdır("dışarıda mod yok") }
dene { yazdır(ad) } yakala h { yazdır("dışarıda ad yok") }
`,
	})
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\n%s", err, out)
	}
	want := "farklı: []\nyok,yok,yok,yok,yok,yok\nyok,yok,yok,yok\ndışarıda mod yok\ndışarıda ad yok\n"
	if out != want {
		t.Fatalf("aktarılan ad çağrılar arasında sızmamalı:\n%q bekleniyordu\n%q alındı", want, out)
	}
}
