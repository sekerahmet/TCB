package testrunner

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

const exampleTests = `TR_func topla(a: TR_Int, b: TR_Int) -> TR_Int { TR_return a + b }
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
	fpath := writeTR(t, t.TempDir(), "ornek_test.tr", exampleTests)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()

	if summary.Total != 6 || summary.Passed != 3 || summary.Failed != 3 {
		t.Fatalf("özet yanlış: %+v\n%s", summary, text)
	}
	expected := []string{
		"✓ toplama doğru",
		"✗ bölme hatası — Satır 10",
		"Sıfıra bölme",
		"✗ eşitlik kalır — Satır 14: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)",
		" 14 |     TR_assert_equal(5, topla(2, 2))", // kod çerçevesi (dosyanın metni)
		"✓ yerel tanım sızmaz",
		"✓ önceki testin yereli görünmez",
		"✗ mesajlı doğrulama — Satır 28: doğrulama başarısız: özel mesaj",
	}
	for _, b := range expected {
		if !strings.Contains(text, b) {
			t.Errorf("çıktıda %q yok:\n%s", b, text)
		}
	}
	if summary.OK() {
		t.Error("kalan test varken Basarili() doğru döndü")
	}
}

// Testin başlattığı eşzamanlı görevdeki hata (başarısız doğrulama dahil) testi kaldırır (10.1);
// eskiden yalnızca rapora yazılıyor, test geçmiş görünüyordu (Go'da başka goroutine'den t.Error
// testi kaldırır). Hata, görevi başlatan testte kalır; sonraki teste taşmaz.
// Her test programın başlangıç durumunda başlar (12.5): iki test aynı gövdeyle yazılır, ikisi de
// geçmeli, ikisinin çektiği sayı aynı olmalı. Sonuç sıraya bağlı değildir.
func TestEveryTestRebuildsTheProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "durumlu.tr", `TR_var sayac = 0
TR_func artir() -> TR_Int {
    sayac = sayac + 1
    TR_return sayac
}
`)
	const body = ` {
    TR_assert_equal(1, sayac_artir())
    TR_assert_equal(1, durumlu.artir())
    liste.TR_append(2)
    TR_assert_equal(2, liste.TR_length)
    d["b"] = 2
    TR_assert_equal(2, d.TR_length)
    TR_print(TR_randn([2]))
}
`
	fpath := writeTR(t, dir, "yeniden_test.tr", `TR_import "durumlu" TR_as durumlu
TR_var sayac = 0
TR_var liste = [1]
TR_var d = {"a": 1}
TR_func sayac_artir() -> TR_Int {
    sayac = sayac + 1
    TR_return sayac
}
TR_test "birinci"`+body+`TR_test "ikinci"`+body)
	var out bytes.Buffer
	summary := RunFile(fpath, &out)
	text := out.String()
	if summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("testler birbirini etkiledi: %+v\n%s", summary, text)
	}
	var draws []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "[") {
			draws = append(draws, strings.TrimSpace(line))
		}
	}
	if len(draws) != 2 || draws[0] != draws[1] {
		t.Fatalf("iki test aynı sayıları çekmeli: %q\n%s", draws, text)
	}
}

func TestTaskErrorFailsTest(t *testing.T) {
	fpath := writeTR(t, t.TempDir(), "gorev_test.tr", `TR_test "görevde başarısız doğrulama" {
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
	writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	writeTR(t, root, "alt/b_test.tr", `TR_test "b" { TR_assert_equal("x", "x") }`)
	writeTR(t, root, "yardimci.tr", `TR_test "c" { TR_assert(TR_true) }`) // test dosyasını adı değil TR_test yapar (12.1)
	writeTR(t, root, "testsiz.tr", `TR_func f() -> TR_Int { TR_return 1 }`)
	writeTR(t, root, ".gizli/d_test.tr", `TR_test "gizli" { TR_assert(TR_false) }`)
	writeTR(t, root, "alt/testdata/e_test.tr", `TR_test "fikstür" { TR_assert(TR_false) }`)

	var out bytes.Buffer
	summary, err := Run([]string{root}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 3 || summary.Passed != 3 || summary.File != 3 || !summary.OK() {
		t.Fatalf("özet yanlış: %+v\n%s", summary, out.String())
	}
	if !strings.Contains(out.String(), "✅ 3 test, 3 geçti, 0 kaldı") {
		t.Errorf("özet satırı yok:\n%s", out.String())
	}
}

// Kural: verilen bütün hedefler koşar, sonuç hepsinindir.
// Neden: iki dosya verilince yalnız ilki koşuyor, ikincinin kalan testi geçmiş görünüyordu (DENETİM istek 125 N1).
func TestEveryTargetRuns(t *testing.T) {
	root := t.TempDir()
	passing := writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	failing := writeTR(t, root, "b_test.tr", `TR_test "b" { TR_assert(TR_false) }`)
	folder := filepath.Join(root, "alt")
	writeTR(t, root, "alt/c_test.tr", `TR_test "c" { TR_assert(TR_true) }`)

	for _, targets := range [][]string{{passing, failing}, {failing, passing}} {
		var out bytes.Buffer
		summary, err := Run(targets, &out)
		if err != nil {
			t.Fatal(err)
		}
		if summary.File != 2 || summary.Total != 2 || summary.Failed != 1 || summary.OK() {
			t.Fatalf("%v: ikinci dosya da koşmalı ve sonuç düşmeli: %+v\n%s", targets, summary, out.String())
		}
		if !strings.Contains(out.String(), "❌ 2 test, 1 geçti, 1 kaldı") {
			t.Errorf("%v: özet satırı yok:\n%s", targets, out.String())
		}
	}

	var out bytes.Buffer
	summary, err := Run([]string{passing, folder}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.File != 2 || summary.Passed != 2 || !summary.OK() {
		t.Fatalf("dosya ve klasör birlikte koşmalı: %+v\n%s", summary, out.String())
	}
}

// Kural: hedeflerden biri olmayan dosya ya da testsiz klasörse hatadır ve hiçbir test koşmaz.
// Neden: öbür hedefler geçince koşmayan hedef görünmeden çıkış 0 oluyordu.
func TestAMissingTargetStopsTheRun(t *testing.T) {
	root := t.TempDir()
	passing := writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	cases := []struct {
		target string
		code   messages.Code
	}{
		{filepath.Join(root, "yok.tr"), messages.H8502},
		{t.TempDir(), messages.H8503},
	}
	for _, c := range cases {
		for _, targets := range [][]string{{passing, c.target}, {c.target, passing}} {
			var out bytes.Buffer
			if _, err := Run(targets, &out); !messages.Has(err, c.code) {
				t.Fatalf("%v: %v vermeli: %v", targets, c.code, err)
			}
			if out.Len() != 0 {
				t.Fatalf("%v: hiçbir test koşmamalı:\n%s", targets, out.String())
			}
		}
	}
}

// Kural: adı ne olursa olsun TR_test içeren dosya test dosyasıdır; aktarılan modülün testleri de koşar,
// dolaylı aktarılanınki de, her dosya bir kez ve kendi makinesinde (12.1).
// Neden: adı _test.tr ile bitmeyen modülün ve aktarılan modülün kalan testi koşmadan geçmiş görünüyordu
// (DENETİM istek 81).
func TestImportedModuleTestsRun(t *testing.T) {
	root := t.TempDir()
	main := writeTR(t, root, "a_test.tr", "TR_import \"ara\"\nTR_import \"modul\"\nTR_test \"a\" { TR_assert_equal(0, n) }")
	writeTR(t, root, "ara.tr", "TR_import \"derin\"\nTR_func f() -> TR_Int { TR_return 1 }")
	writeTR(t, root, "derin.tr", `TR_test "c" { TR_assert(TR_true) }`)
	module := writeTR(t, root, "modul.tr", "TR_var n = 0\nTR_test \"b\" { n = 5; TR_assert_equal(5, n) }")
	for _, targets := range [][]string{{main}, {root}, {main, module}, {module, main}, {main, main}} {
		var out bytes.Buffer
		summary, err := Run(targets, &out)
		if err != nil {
			t.Fatal(err)
		}
		if summary.File != 3 || summary.Total != 3 || summary.Passed != 3 {
			t.Fatalf("%v: üç dosya, üç test bekleniyordu: %+v\n%s", targets, summary, out.String())
		}
	}
	writeTR(t, root, "derin.tr", `TR_test "kalan" { TR_assert(TR_false) }`)
	var out bytes.Buffer
	if summary, err := Run([]string{main}, &out); err != nil || summary.Failed != 1 || summary.OK() {
		t.Fatalf("aktarılan modülün kalan testi sonucu düşürmeli: %+v, %v\n%s", summary, err, out.String())
	}
}

// Kural: aynı dosya hedeflerde iki kez yazılsa da bir kez koşar (12.1).
func TestAFileRunsOnce(t *testing.T) {
	root := t.TempDir()
	file := writeTR(t, root, "alt/a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	var out bytes.Buffer
	other := filepath.Dir(file) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(file)
	summary, err := Run([]string{file, root, other}, &out)
	if err != nil || summary.File != 1 || summary.Total != 1 {
		t.Fatalf("bir kez koşmalı: %+v, %v\n%s", summary, err, out.String())
	}
}

// Kural: açıkça verilen dosyada test yoksa hatadır, yanında testli dosya olsa bile; klasörde testsiz
// dosya olağandır, klasörün toplamında hiç test yoksa hatadır (DENETİM istek 132 Ş1).
// Neden: test bloğu olmayan dosya "0 test" ile ya da yanındakinin sonucuyla geçmiş görünüyordu.
func TestATargetWithoutTestsIsAnError(t *testing.T) {
	root := t.TempDir()
	passing := writeTR(t, root, "a/a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	plain := writeTR(t, root, "b/duz.tr", `TR_func f() -> TR_Int { TR_return 1 } # TR_test "yorum"`)
	text := writeTR(t, root, "c/metin.tr", "TR_var s = \"TR_test \\\"x\\\"\"")
	for _, targets := range [][]string{{plain}, {text}, {passing, plain}, {plain, passing}, {filepath.Dir(plain)}, {passing, filepath.Dir(text)}} {
		var out bytes.Buffer
		if _, err := Run(targets, &out); !messages.Has(err, messages.H8503) {
			t.Fatalf("%v: H8503 vermeli: %v", targets, err)
		}
		if out.Len() != 0 {
			t.Fatalf("%v: hiçbir test koşmamalı:\n%s", targets, out.String())
		}
	}
	var out bytes.Buffer
	if summary, err := Run([]string{root}, &out); err != nil || summary.File != 1 || !summary.OK() {
		t.Fatalf("klasörde testsiz dosya olağandır: %+v, %v\n%s", summary, err, out.String())
	}
}

// Kural: adı _test.tr ile biten dosya hiç elenmez; testi yoksa koşu düşer (H8503), klasörde de, tek
// başına da, yanında geçen test olsa da (DENETİM istek 138 Ş1, Ş2).
func TestANamedTestFileWithoutTestsFailsTheRun(t *testing.T) {
	root := t.TempDir()
	passing := writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	empty := writeTR(t, root, "bos_test.tr", `TR_func f() -> TR_Int { TR_return 1 } # TR_test "yorum"`)
	for _, targets := range [][]string{{empty}, {passing, empty}, {empty, passing}, {root}} {
		var out bytes.Buffer
		summary, err := Run(targets, &out)
		if err != nil || summary.OK() || summary.Unloadable != 1 {
			t.Fatalf("%v: koşu düşmeliydi: %+v, %v\n%s", targets, summary, err, out.String())
		}
		if want := messages.Text(messages.H8503, empty); !strings.Contains(out.String(), want) {
			t.Fatalf("%v: %q yazılmalı:\n%s", targets, want, out.String())
		}
	}
}

// Kural: en dışta olmayan test koşmadan geçmiş görünmez: dosya denetimden geçemez, adı ne olursa olsun
// koşu düşer (12.2, DENETİM istek 138 Ş2).
func TestANestedTestFailsTheRun(t *testing.T) {
	for name, text := range map[string]string{
		"koşul":        "TR_if TR_true {\n    TR_test \"kalan\" { TR_assert(TR_false) }\n}\n",
		"fonksiyon":    "TR_func f() {\n    TR_test \"kalan\" { TR_assert(TR_false) }\n}\n",
		"test":         "TR_test \"dis\" {\n    TR_test \"ic\" { TR_assert(TR_false) }\n}\n",
		"yanında test": "TR_func f() {\n    TR_test \"kalan\" { TR_assert(TR_false) }\n}\nTR_test \"a\" { TR_assert(TR_true) }\n",
	} {
		for _, file := range []string{"ic_test.tr", "ic.tr"} {
			root := t.TempDir()
			writeTR(t, root, file, text)
			var out bytes.Buffer
			if summary, err := Run([]string{root}, &out); err != nil || summary.OK() || summary.Unloadable != 1 || summary.Total != 0 {
				t.Errorf("%s, %s: koşu düşmeliydi: %+v, %v\n%s", name, file, summary, err, out.String())
			}
		}
	}
}

// Kural: bozuk test dosyası klasörden kaybolmaz, "yüklenemedi" olur; yanındaki geçen test sonucu ✅ yapmaz
// (DENETİM istek 138 Ş1'in altı yazımı). Açıkça verilince de "test bulunamadı" değil yükleme hatası
// söylenir. TR_test yazan dosyanın adı önemsizdir (139 Ş2); kimliksiz yazımda dosyayı yalnız adı tutar.
func TestABrokenNamedTestFileIsNotDropped(t *testing.T) {
	kalan := "TR_test \"kalan\" { TR_assert(TR_false) }\n"
	for name, text := range map[string]string{
		"kapanmayan metin":       "TR_var s = \"kapanmayan\n" + kalan,
		"kapanmayan ters tırnak": "TR_var s = `kapanmayan\n" + kalan,
		"üç tırnak":              "\"\"\"\n" + kalan,
		"adsız test":             "TR_test { TR_assert(TR_false) }\n",
		"ad metin değil":         "TR_test nm { TR_assert(TR_false) }\n",
		"kimliksiz yazım":        "test \"kalan\" { TR_assert(TR_false) }\n",
	} {
		for _, file := range []string{"k_test.tr", "k.tr"} {
			if file == "k.tr" && name == "kimliksiz yazım" {
				continue
			}
			root := t.TempDir()
			writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
			broken := writeTR(t, root, file, text)
			for _, target := range []string{root, broken} {
				var out bytes.Buffer
				summary, err := Run([]string{target}, &out)
				if err != nil || summary.OK() || summary.Unloadable != 1 {
					t.Errorf("%s, %s: yüklenemedi olmalı: %+v, %v\n%s", name, target, summary, err, out.String())
				}
				if strings.Contains(out.String(), messages.Text(messages.H8503, broken)) {
					t.Errorf("%s, %s: bozuk dosya testsiz denmemeli:\n%s", name, target, out.String())
				}
			}
		}
	}
}

// Kural: tarama testdata adlı klasöre inmez; açıkça verilen testdata koşar (bilerek hatalı dosyalar orada).
func TestFixtureDirIsOnlyRunWhenGiven(t *testing.T) {
	root := t.TempDir()
	writeTR(t, root, "a_test.tr", `TR_test "a" { TR_assert(TR_true) }`)
	fixture := writeTR(t, root, "testdata/b_test.tr", `TR_test "b" { TR_assert(TR_false) }`)
	for target, failed := range map[string]int{root: 0, filepath.Dir(fixture): 1, fixture: 1} {
		var out bytes.Buffer
		if summary, err := Run([]string{target}, &out); err != nil || summary.Failed != failed {
			t.Fatalf("%s: %d kalan bekleniyordu: %+v, %v\n%s", target, failed, summary, err, out.String())
		}
	}
}

func TestEmptyDirErrors(t *testing.T) {
	if _, err := Run([]string{t.TempDir()}, &bytes.Buffer{}); !messages.Has(err, messages.H8503) {
		t.Fatalf("test dosyası olmayan klasör H8503 vermeli: %v", err)
	}
	if _, err := Run([]string{filepath.Join(t.TempDir(), "yok")}, &bytes.Buffer{}); !messages.Has(err, messages.H8502) {
		t.Fatalf("olmayan hedef H8502 vermeli: %v", err)
	}
}

func TestUnloadableFile(t *testing.T) {
	root := t.TempDir()
	writeTR(t, root, "bozuk_test.tr", "TR_test \"a\" { TR_assert(TR_true) }\nTR_var = 5\n")
	writeTR(t, root, "ustduzey_test.tr", "TR_throw \"yükleme hatası\"\nTR_test \"b\" { TR_assert(TR_true) }\n")
	writeTR(t, root, "iyi_test.tr", "TR_test \"c\" { TR_assert(TR_true) }\n")
	var out bytes.Buffer
	summary, err := Run([]string{root}, &out)
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
	fpath := writeTR(t, t.TempDir(), "e_test.tr", `TR_test "ertele" {
    TR_defer { TR_print("temizlik") }
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
	fpath := writeTR(t, t.TempDir(), "g_test.tr", `TR_test "görev" {
    TR_async { TR_var x = 1 / 0 }
}
`)
	var out bytes.Buffer
	RunFile(fpath, &out)
	if !strings.Contains(out.String(), "✗ görev") || !strings.Contains(out.String(), "Sıfıra bölme") {
		t.Fatalf("görev hatası testin hatası olarak raporda yok:\n%s", out.String())
	}
}

// 10.6: testte değeri tutulan ama beklenmeyen görevin hatası test biterken testin hatası olur;
// beklenip yakalanan görevin hatası testi düşürmez.
func TestUnwaitedTaskErrorShownInReport(t *testing.T) {
	fpath := writeTR(t, t.TempDir(), "g_test.tr", `TR_test "beklenmeyen" {
    TR_var g = TR_async { TR_var x = 1 / 0 }
}

TR_test "beklenen" {
    TR_var g = TR_async { TR_var x = 1 / 0 }
    TR_print(TR_error_code { g.TR_wait() })
}
`)
	var out bytes.Buffer
	s := RunFile(fpath, &out)
	if s.Passed != 1 || s.Failed != 1 || !strings.Contains(out.String(), "✗ beklenmeyen") || !strings.Contains(out.String(), "Sıfıra bölme") {
		t.Fatalf("beklenmeyen görevin hatası testin hatası olmalı, beklenen düşürmemeli: %+v\n%s", s, out.String())
	}
}

// Testin 'ertele' blokları, programın üst düzeyinde olduğu gibi, testin başlattığı görevler
// bittikten sonra çalışır.
func TestDeferRunsAfterTasks(t *testing.T) {
	f := writeTR(t, t.TempDir(), "ertele_test.tr", `TR_test "ertele görevden sonra" {
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
	f := writeTR(t, t.TempDir(), "satır_test.tr", "TR_test \"satır kelimesi\" {\n    TR_throw \"Satır sayısı yanlış\"\n}\n")
	var b bytes.Buffer
	RunFile(f, &b)
	if out := b.String(); !strings.Contains(out, "Satır 3: Satır sayısı yanlış") || !strings.Contains(out, `3 |     TR_throw "Satır sayısı yanlış"`) {
		t.Fatalf("testin satırı eklenmeli ve çerçevesi çizilmeli:\n%s", out)
	}
}

// TR_cmd_test altında TR_exit süreci bitirmez (1.12): çağrıldığı test (çağırdığı fonksiyon ya da
// başlattığı görev dahil) başarısız sayılır, öbür testler sürer ve rapor çıkış çağrısını anlatır.
func TestExitInATestFailsOnlyThatTest(t *testing.T) {
	fpath := writeTR(t, t.TempDir(), "cikis_test.tr", `# TR 4
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

// writeTR bir test dosyasını başlığıyla (2.1) yazar; satırlar dosyanınkidir, gövdenin ilk satırı 2.
func writeTR(t *testing.T, dir, name, body string) string {
	t.Helper()
	return testkit.WriteFile(t, dir, name, source.Header+"\n"+body)
}
