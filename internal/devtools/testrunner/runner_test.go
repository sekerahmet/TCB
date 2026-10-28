package testrunner

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

const exampleTests = `TR_func topla(a, b) { TR_return a + b }
TR_var sayac = 0

TR_test "toplama doğru" {
    TR_assert(topla(2, 2) == 4)
}

TR_test "bölme hatası" {
    TR_var x = 10 / 0
}

TR_test "eşitlik kalır" {
    TR_assert_equal(5, topla(2, 2))
}

TR_test "yerel tanım sızmaz" {
    TR_var gizli = 1
    TR_assert(gizli == 1)
}

TR_test "önceki testin yereli görünmez" {
    TR_var görünür = TR_true; TR_try { TR_print(gizli) } TR_catch h { görünür = TR_false }
    TR_if görünür { TR_throw "gizli görünür olmamalıydı" }; TR_var gizli = 2 # sonra tanımlı: sızma çalışma anında sınanır (5.14)
}

TR_test "mesajlı doğrulama" {
    TR_assert(TR_false, "özel mesaj")
}
`

func TestRunFile(t *testing.T) {
	fpath := testkit.WriteFile(t, t.TempDir(), "ornek_test.tr", exampleTests)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()

	if summary.Total != 6 || summary.Passed != 3 || summary.Failed != 3 {
		t.Fatalf("özet yanlış: %+v\n%s", summary, text)
	}
	expected := []string{
		"✓ toplama doğru",
		"✗ bölme hatası — Satır 9",
		"Sıfıra bölme",
		"✗ eşitlik kalır — Satır 13: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)",
		" 13 |     eşit_mi(5, topla(2, 2))", // kod çerçevesi
		"✓ yerel tanım sızmaz",
		"✓ önceki testin yereli görünmez",
		"✗ mesajlı doğrulama — Satır 27: doğrulama başarısız: özel mesaj",
	}
	for _, b := range expected {
		if !strings.Contains(text, b) {
			t.Errorf("çıktıda %q yok:\n%s", b, text)
		}
	}
	if summary.OK() {
		t.Error("kalan test varken Basarili() TR_true döndü")
	}
}

// Testin başlattığı eşzamanlı görevdeki hata (başarısız doğrulama dahil) testi kaldırır (10.1);
// eskiden yalnızca rapora yazılıyor, test geçmiş görünüyordu (Go'da başka goroutine'den t.Error
// testi kaldırır). Hata, görevi başlatan testte kalır; sonraki teste taşmaz.
func TestTaskErrorFailsTest(t *testing.T) {
	fpath := testkit.WriteFile(t, t.TempDir(), "gorev_test.tr", `TR_test "görevde başarısız doğrulama" {
    TR_async {
        TR_assert(TR_false, "görevde kaldı")
    }
    TR_wait_all()
}

TR_test "sonraki test etkilenmez" {
    TR_assert(TR_true)
}
`)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()
	if summary.Total != 2 || summary.Failed != 1 || summary.Passed != 1 {
		t.Fatalf("özet yanlış: %+v\n%s", summary, text)
	}
	for _, b := range []string{"✗ görevde başarısız doğrulama", "görevde kaldı", "✓ sonraki test etkilenmez"} {
		if !strings.Contains(text, b) {
			t.Errorf("çıktıda %q yok:\n%s", b, text)
		}
	}
}

func TestSummaryLine(t *testing.T) {
	o := Summary{Total: 12, Passed: 11, Failed: 1}
	if o.String() != "12 test, 11 geçti, 1 kaldı" {
		t.Fatalf("özet satırı: %q", o.String())
	}
}

func TestDirScan(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	testkit.WriteFile(t, root, "alt/b_test.tr", `TR_test "b" { TR_assert_equal("x", "x") }`)
	testkit.WriteFile(t, root, "yardimci.tr", `TR_test "çalışmamalı" { TR_assert(TR_false) }`)
	testkit.WriteFile(t, root, ".gizli/c_test.tr", `TR_test "gizli" { TR_assert(TR_false) }`)

	var out bytes.Buffer
	summary, err := Run(root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Passed != 2 || summary.File != 2 || !summary.OK() {
		t.Fatalf("özet yanlış: %+v\n%s", summary, out.String())
	}
	if !strings.Contains(out.String(), "✅ 2 test, 2 geçti, 0 kaldı") {
		t.Errorf("özet satırı yok:\n%s", out.String())
	}
}

func TestEmptyDirErrors(t *testing.T) {
	if _, err := Run(t.TempDir(), &bytes.Buffer{}); !messages.Has(err, messages.H8503) {
		t.Fatalf("test dosyası olmayan klasör H8503 vermeli: %v", err)
	}
	if _, err := Run(filepath.Join(t.TempDir(), "yok"), &bytes.Buffer{}); !messages.Has(err, messages.H8502) {
		t.Fatalf("olmayan hedef H8502 vermeli: %v", err)
	}
}

func TestUnloadableFile(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "bozuk_test.tr", "test \"a\" { doğrula(doğru) }\ndeğişken = 5\n")
	testkit.WriteFile(t, root, "ustduzey_test.tr", "TR_throw \"yükleme hatası\"\nTR_test \"b\" { TR_assert(TR_true) }\n")
	testkit.WriteFile(t, root, "iyi_test.tr", "TR_test \"c\" { TR_assert(TR_true) }\n")
	var out bytes.Buffer
	summary, err := Run(root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Unloadable != 2 || summary.Passed != 1 || summary.OK() {
		t.Fatalf("özet yanlış: %+v\n%s", summary, out.String())
	}
	if !strings.Contains(out.String(), "2 dosya yüklenemedi") {
		t.Errorf("yükleme hatası özetlenmedi:\n%s", out.String())
	}
	// Aşama adı (source.StageSyntax) iletiye ham girmez; etkin dilde yazılır.
	if !strings.Contains(out.String(), "Sözdizimi hataları nedeniyle testler çalıştırılamadı") {
		t.Errorf("aşama adı Türkçe yazılmadı:\n%s", out.String())
	}
}

// Test içindeki 'ertele' test bitince çalışmalı; testin kendisi kalsa bile.
func TestDeferAndOutput(t *testing.T) {
	fpath := testkit.WriteFile(t, t.TempDir(), "e_test.tr", `TR_test "ertele" {
    TR_defer TR_print("temizlik")
    TR_print("gövde")
    TR_assert(TR_false)
}
`)
	var out bytes.Buffer
	RunFile(fpath, &out)
	m := out.String()
	g, tz := strings.Index(m, "gövde"), strings.Index(m, "temizlik")
	if g < 0 || tz < 0 || tz < g {
		t.Fatalf("ertele sırası yanlış:\n%s", m)
	}
}

// Testin başlattığı eşzamanlı görevin hatası raporda testin hatası olarak görünür (standart
// hataya kaçmaz; 10.1).
func TestTaskErrorShownInReport(t *testing.T) {
	fpath := testkit.WriteFile(t, t.TempDir(), "g_test.tr", `TR_test "görev" {
    TR_async { TR_var x = 1 / 0 }
}
`)
	var out bytes.Buffer
	RunFile(fpath, &out)
	if !strings.Contains(out.String(), "✗ görev") || !strings.Contains(out.String(), "Sıfıra bölme") {
		t.Fatalf("görev hatası testin hatası olarak raporda yok:\n%s", out.String())
	}
}

// Testin 'ertele' blokları, programın üst düzeyinde olduğu gibi, testin başlattığı görevler
// bittikten sonra çalışır.
func TestDeferRunsAfterTasks(t *testing.T) {
	f := testkit.WriteFile(t, t.TempDir(), "ertele_test.tr", `TR_test "ertele görevden sonra" {
    TR_var l = [1]
    TR_defer { TR_assert(l.TR_length == 2, "görev erteleden önce bitmeli") }
    TR_async {
        TR_sleep(0.05)
        l.TR_append(2)
    }
}
`)
	var b bytes.Buffer
	if s := RunFile(f, &b); s.Passed != 1 || s.Failed != 0 {
		t.Fatalf("test geçmeliydi: %+v\n%s", s, b.String())
	}
}

// Kullanıcı iletisinde geçen "Satır " sözcüğü konum sayılmaz: konum yapıdan okunur (fırlat'ın
// kendi satırı, 8.3) ve çerçevesi çizilir.
func TestUserMessageWithSatirGetsLine(t *testing.T) {
	f := testkit.WriteFile(t, t.TempDir(), "satır_test.tr", "TR_test \"satır kelimesi\" {\n    TR_throw \"Satır sayısı yanlış\"\n}\n")
	var b bytes.Buffer
	RunFile(f, &b)
	if out := b.String(); !strings.Contains(out, "Satır 2: Satır sayısı yanlış") || !strings.Contains(out, `2 |     fırlat "Satır sayısı yanlış"`) {
		t.Fatalf("testin satırı eklenmeli ve çerçevesi çizilmeli:\n%s", out)
	}
}

// TR_cmd_test altında TR_exit süreci bitirmez (1.12): çağrıldığı test (çağırdığı fonksiyon ya da
// başlattığı görev dahil) başarısız sayılır, öbür testler sürer ve rapor çıkış çağrısını anlatır.
func TestExitInATestFailsOnlyThatTest(t *testing.T) {
	fpath := testkit.WriteFile(t, t.TempDir(), "cikis_test.tr", `# TR 4
TR_func bitir() {
    TR_exit(3)
}
TR_test "doğrudan çıkış" {
    TR_exit(3)
}
TR_test "fonksiyondan çıkış" {
    bitir()
}
TR_test "görevden çıkış" {
    TR_async { TR_exit(4) }
}
TR_test "sonra gelen test sürer" {
    TR_assert(TR_true)
}
`)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()
	if summary.Total != 4 || summary.Passed != 1 || summary.Failed != 3 {
		t.Fatalf("özet yanlış: %+v\n%s", summary, text)
	}
	for _, want := range []string{"✗ doğrudan çıkış", "✗ fonksiyondan çıkış", "✗ görevden çıkış", "✓ sonra gelen test sürer"} {
		if !strings.Contains(text, want) {
			t.Errorf("çıktıda %q yok:\n%s", want, text)
		}
	}
}
