package e2e

import (
	"debug/buildinfo"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Komut satırının temel komutları: yardım ve sürüm, komut adları, dosya çalıştırma, kontrol,
// test, biçimle, dap, etkileşimli kabuk ve derle. Hepsi derlenmiş 'tr' ikilisiyle sınanır.

func TestHelpAndVersion(t *testing.T) {
	for _, a := range []string{"yardım", "-h"} {
		s := tr(t, "", a)
		// Yardım her komutu listeler (hakem mimari 22: dap eksikti)
		testkit.Wait(t, s, 0, []string{"Kullanım:", "tr derle <dosya.tr>", "tr test", "tr çalıştır", "tr hata_ayıkla",
			"tr kontrol", "tr biçimle", "tr dap", "tr yardım", "tr sürüm"}, nil)
	}
	for _, a := range []string{"sürüm", "-v"} {
		s := tr(t, "", a)
		testkit.Wait(t, s, 0, []string{"v" + trVersion(t)}, nil)
	}
}

// Komut adları ad kodlarıdır: her yazım dilinin adı kabul edilir (tr çalıştır da tr run da her
// makinede çalışır; betikler TR_DIL'den bağımsızdır). Türkçe karaktersiz yazımlar ve --help,
// --version dosya adı sanılmaz, asıl komutu öneren bir hatayla 1 koduyla çıkar.
func TestCommandsHaveOneName(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "a.tr", "TR_print(\"çalıştı\")\n")
	for ad, actual := range map[string]string{
		"calistir": "çalıştır", "bicimle": "biçimle", "hata_ayikla": "hata_ayıkla",
		"yardim": "yardım", "--help": "yardım", "surum": "sürüm", "--version": "sürüm",
	} {
		s := tr(t, dir, ad, "a.tr")
		testkit.Wait(t, s, 2, nil, []string{"'" + ad + "' TR'de bir komut değil; 'tr " + actual + "' yazmak mı istediniz?"})
		if strings.Contains(s.Stdout, "çalıştı") {
			t.Errorf("%s: program çalışmamalıydı", ad)
		}
	}
	// Seçenekler de ad kodudur (opt): her dilin adı kabul edilir, sözcükleri tireyle yazılır.
	testkit.Wait(t, tr(t, dir, "biçimle", "--check", "a.tr"), 0, nil, nil)
	testkit.Wait(t, tr(t, dir, "format", "--kontrol", "a.tr"), 0, nil, nil)
	testkit.Wait(t, tr(t, dir, "profil", "--go_profili", "x.prof", "a.tr"), 2, nil, []string{"bilinmeyen seçenek \"--go_profili\"", "--go-profili"})
	// İngilizce yazım dilinin adları da her makinede komuttur.
	testkit.Wait(t, tr(t, dir, "run", "a.tr"), 0, []string{"çalıştı"}, nil)
	testkit.Wait(t, tr(t, dir, "version"), 0, []string{"v" + trVersion(t)}, nil)
	testkit.Wait(t, tr(t, dir, "help"), 0, []string{"Kullanım:"}, nil)
}

func TestMissingArgument(t *testing.T) {
	for _, k := range []string{"kontrol", "derle", "hata_ayıkla", "çalıştır", "biçimle"} {
		s := tr(t, "", k)
		testkit.Wait(t, s, 2, nil, []string{"Hata: Lütfen"}) // eksik argüman bir kullanım hatasıdır (1.9)
	}
}

// Kullanıcının adı dosyada yerleşiği gölgelemez (2.1): 'toplam' bir kullanıcı adıdır, TR_sum
// değildir; uyarı yoktur, program çalışır. Gölgeleme uyarısı yalnız kabukta.
func TestShadowingWarning(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "a.tr", "TR_var toplam = 5\nTR_print(toplam)\n")
	for _, command := range [][]string{{"a.tr"}, {"kontrol", "a.tr"}} {
		s := tr(t, dir, command...)
		testkit.Wait(t, s, 0, nil, nil)
		if strings.Contains(s.Stderr, "Uyarı") || strings.Contains(s.Stderr, "❌") {
			t.Errorf("%v: dosyada gölgeleme uyarısı olmamalı:\n%s", command, s.Stderr)
		}
	}
	testkit.Wait(t, tr(t, dir, "a.tr"), 0, []string{"5"}, nil)
}

func TestUnknownCommand(t *testing.T) {
	dir := t.TempDir()
	s := tr(t, dir, "olmayan_komut")
	testkit.Wait(t, s, 1, nil, []string{"Dosya okuma hatası", "olmayan_komut", "böyle bir dosya ya da klasör yok"})
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "iyi.tr", "TR_var ad = \"dünya\"\nTR_print(\"merhaba \\(ad)\")\n")
	writeTR(t, dir, "sozdizimi.tr", "TR_print(\"a\"\n")
	writeTR(t, dir, "tip.tr", "TR_var x: TR_Int = \"metin\"\nTR_print(\"çalışmamalı\")\n")
	writeTR(t, dir, "calisma.tr", "TR_print(\"önce\")\nTR_var y = 1 / 0\nTR_print(\"sonra\")\n")

	for _, args := range [][]string{{"iyi.tr"}, {"çalıştır", "iyi.tr"}} {
		testkit.Wait(t, tr(t, dir, args...), 0, []string{"merhaba dünya"}, nil)
	}

	s := tr(t, dir, "sozdizimi.tr")
	testkit.Wait(t, s, 1, nil, []string{"❌ Sözdizimi Hataları:", "  • Satır 3, Sütun 1"})

	s = tr(t, dir, "tip.tr")
	testkit.Wait(t, s, 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları:", "Tip uyuşmazlığı", "2 | TR_var x: TR_Int"})
	if strings.Contains(s.Stdout, "çalışmamalı") {
		t.Error("tip hatalı program çalıştırılmamalıydı")
	}

	s = tr(t, dir, "calisma.tr")
	testkit.Wait(t, s, 1, []string{"önce"}, []string{"❌ Çalışma Zamanı Hatası:", "Satır 3: Sıfıra bölme hatası", "3 | TR_var y = 1 / 0"})
	if strings.Contains(s.Stdout, "sonra") {
		t.Error("çalışma hatasından sonra program durmalıydı")
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "iyi.tr", "TR_print(\"çalışmamalı\")\n")
	writeTR(t, dir, "TR_type.tr", "TR_var x: TR_Int = \"metin\"\n")
	writeTR(t, dir, "sozdizimi.tr", "fonksiyon f( {\n")

	s := tr(t, dir, "kontrol", "iyi.tr")
	testkit.Wait(t, s, 0, []string{"iyi.tr başarıyla denetlendi"}, nil)
	if strings.Contains(s.Stdout, "çalışmamalı") {
		t.Error("kontrol programı çalıştırmamalı")
	}
	testkit.Wait(t, tr(t, dir, "kontrol", "TR_type.tr"), 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları:"})
	testkit.Wait(t, tr(t, dir, "kontrol", "sozdizimi.tr"), 1, nil, []string{"❌ Sözdizimi Hataları:"})
	testkit.Wait(t, tr(t, dir, "kontrol", "TR_none.tr"), 1, nil, []string{"Dosya okuma hatası", "TR_none.tr", "böyle bir dosya ya da klasör yok"})
}

func TestTestCommand(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "gecen/a_test.tr", "TR_test \"toplama\" {\n    TR_assert_equal(4, 2 + 2)\n}\n")
	writeTR(t, dir, "kalan/b_test.tr", "TR_test \"yanlış\" {\n    TR_assert_equal(5, 2 + 2)\n}\n")

	testkit.Wait(t, tr(t, dir, "test", "gecen"), 0, []string{"toplama"}, nil)
	testkit.Wait(t, tr(t, dir, "test", "kalan"), 1, []string{"yanlış"}, nil)
	// Hedefsiz: bulunulan klasördeki tüm *_test.tr dosyaları (biri kalıyor).
	testkit.Wait(t, tr(t, dir, "test"), 1, []string{"toplama", "yanlış"}, nil)
	testkit.Wait(t, tr(t, dir, "test", "olmayan"), 1, nil, []string{"Hata:"})
}

func TestFormat(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "a.tr", "TR_var   x=1\nTR_print( x )\n")

	s := tr(t, dir, "biçimle", "--kontrol", "a.tr")
	testkit.Wait(t, s, 1, []string{"biçimlenmeli: a.tr"}, nil)

	testkit.Wait(t, tr(t, dir, "biçimle", "a.tr"), 0, []string{"biçimlendi: a.tr"}, nil)
	testkit.Wait(t, tr(t, dir, "biçimle", "--kontrol", "."), 0, []string{"hepsi standart biçimde"}, nil)
	testkit.Wait(t, tr(t, dir, "biçimle", "olmayan.tr"), 1, nil, []string{`"olmayan.tr" biçimlenemedi: böyle bir dosya ya da klasör yok`})
	// Sözdizimi hatalı dosya varken "hepsi standart biçimde" denmez
	writeTR(t, dir, "bozuk.tr", "yazdır(\n")
	s = tr(t, dir, "biçimle", "--kontrol", ".")
	testkit.Wait(t, s, 1, []string{"1 tanesi biçimlenemedi"}, []string{"bozuk.tr"})
	if strings.Contains(s.Stdout, "standart biçimde") {
		t.Errorf("hatalı dosya varken başarı yazılmamalı: %q", s.Stdout)
	}
}

func TestDAPStartup(t *testing.T) {
	body := `{"seq":1,"type":"request","command":"initialize","arguments":{"adapterID":"tr"}}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	s := testkit.RunBinary(t, trBinary, "", input, nil, "dap")
	testkit.Wait(t, s, 0, []string{"Content-Length:", `"command":"initialize"`, `"success":true`, `"event":"initialized"`}, nil)

	// Bozuk çerçeve: sunucu hatayı Türkçe bildirip 1 koduyla çıkar.
	s = testkit.RunBinary(t, trBinary, "", "Content-Length: abc\r\n\r\n", nil, "dap")
	testkit.Wait(t, s, 1, nil, []string{"❌ Hata ayıklama sunucusu:"})
}

func TestREPL(t *testing.T) {
	input := strings.Join([]string{
		`TR_print("merhaba")`,
		`1 +`,
		`TR_var x: TR_Int = "a"`,
		`1 / 0`,
		`2 + 3`,
		`cikis`,
		`çıkış`,
	}, "\n") + "\n"
	s := testkit.RunBinary(t, trBinary, "", input, nil)
	testkit.Wait(t, s, 0, []string{"merhaba", "=> 5", "Görüşmek üzere!"}, []string{
		"'cikis' TR'de bir komut değil; çıkmak için 'çıkış' yazın.",
		"❌ Sözdizimi Hataları:\n  • Satır 1, Sütun 4",
		"❌ Tip ve Güvenlik Denetimi Hataları:\n  • Satır 1: Tip uyuşmazlığı",
		"❌ Çalışma Zamanı Hatası:\n  • Satır 1: Sıfıra bölme hatası",
		"1 | 1 / 0",
	})
	for _, p := range []string{"Hata", "❌"} {
		if strings.Contains(s.Stdout, p) {
			t.Errorf("hatalar standart çıktıya yazılmamalı (%q bulundu):\n%s", p, s.Stdout)
		}
	}
	// Denetleyici durumunu korur; eski tip hatası sonraki satırlarda yinelenmemeli.
	if n := strings.Count(s.Stderr, "Tip uyuşmazlığı"); n != 1 {
		t.Errorf("tip hatası %d kez yazıldı, beklenen 1:\n%s", n, s.Stderr)
	}
	// Kabuğun çıkış komutu bir ad kodudur (TR_repl_exit): her yazım dilinin adı kabul edilir.
	s = testkit.RunBinary(t, trBinary, "", "exit\nTR_print(\"sürdü\")\n", nil)
	testkit.Wait(t, s, 0, []string{"Görüşmek üzere!"}, nil)
	if strings.Contains(s.Stdout, "sürdü") {
		t.Error("exit kabuğu kapatmalıydı")
	}
}

// Kabuk satırı önceki satırların adlarını bilerek kodlanır: kullanıcının önceki satırda tanımladığı ad
// (toplam, sum) dilin öğesiyle aynı yazılsa da kullanıcının adıdır, yerleşiğe dönmez. Her yazım dilinde.
func TestREPLKeepsTheEarlierLinesNames(t *testing.T) {
	for _, c := range []struct{ lang, input string }{
		{"tr", "TR_var toplam = 5\nTR_print(toplam + 1)\nçıkış\n"},
		{"en", "var sum = 5\nprint(sum + 1)\nexit\n"},
	} {
		s := testkit.RunBinary(t, trBinary, "", c.input, []string{"TR_DIL=" + c.lang})
		testkit.Wait(t, s, 0, []string{"6\n"}, nil)
		if strings.Contains(s.Stderr, "❌") {
			t.Errorf("%s: kabuk hata verdi:\n%s", c.lang, s.Stderr)
		}
	}
}

// input() boru ile gelen satırları sırayla okur; kabuk da aynı okuyucuyu kullandığı için
// kabukta girdi() bir sonraki satırı alır, kabuk satırları yutulmaz.
func TestInputPipeAndShell(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "g.tr", "TR_var a = TR_input()\nTR_var b = TR_input()\nTR_print(\"a=[\\(a)] b=[\\(b)]\")\n")
	s := testkit.RunBinary(t, trBinary, dir, "bir\niki\n", nil, "g.tr")
	testkit.Wait(t, s, 0, []string{"a=[bir] b=[iki]"}, nil)

	input := "TR_var x = TR_input()\nveri\nTR_print(\"x=\" + x)\nçıkış\n"
	s = testkit.RunBinary(t, trBinary, "", input, nil)
	testkit.Wait(t, s, 0, []string{"x=veri", "Görüşmek üzere!"}, nil)
}

// avxExpected derlenmiş yorumlayıcının GOAMD64=v3 ile derlenip derlenmediğini okur.
func avxExpected(t *testing.T) bool {
	t.Helper()
	info, err := buildinfo.ReadFile(trBinary)
	if err != nil {
		t.Fatalf("derleme bilgisi okunamadı: %v", err)
	}
	var arch, level string
	for _, s := range info.Settings {
		switch s.Key {
		case "GOARCH":
			arch = s.Value
		case "GOAMD64":
			level = s.Value
		}
	}
	return arch == "amd64" && level == "v3"
}

func TestBuildEmbedsModules(t *testing.T) {
	source := t.TempDir()
	writeTR(t, source, "proje/ana.tr", strings.Join([]string{
		`TR_import "lib/yardimci"`,
		`TR_import "lib/yardimci.tr" TR_as y`,
		`TR_import "alt/derin" TR_as d`,
		`TR_if TR_true {`,
		`    TR_print("toplam \(topla(2, 3)) \(y.SELAM) \(d.DERIN)")`,
		`}`,
	}, "\n")+"\n")
	// Modül içindeki aktarım, modülün klasörüne göre çözülür (yorumlayıcıdaki kural).
	writeTR(t, source, "proje/lib/yardimci.tr", "TR_import \"ic.tr\"\nTR_var SELAM = \"merhaba\"\nTR_func topla(a, b) {\n    TR_return a + b + IC\n}\n")
	writeTR(t, source, "proje/lib/ic.tr", "\xef\xbb\xbf# TR 4\nTR_var IC = 100\n")
	writeTR(t, source, "proje/alt/derin.tr", "TR_import \"../lib/ic\"\nTR_var DERIN = IC + 1\n")

	// Önce yorumlayıcıyla: beklenen çıktı.
	expected := "toplam 105 merhaba 101"
	testkit.Wait(t, tr(t, filepath.Join(source, "proje"), "ana.tr"), 0, []string{expected}, nil)

	exit := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, source, "derle", filepath.Join("proje", "ana.tr"), exit)
	testkit.Wait(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (3)", "proje/lib/yardimci.tr", "proje/lib/ic.tr", "proje/alt/derin.tr"}, nil)
	avxNote := strings.Contains(s.Stdout, "AVX2")
	if avxNote != avxExpected(t) {
		t.Errorf("AVX2 notu: çıktıda %v, beklenen %v\n%s", avxNote, !avxNote, s.Stdout)
	}

	// Kaynaklar silinir; uygulama başka bir klasörde çalışır. Oradaki aynı adlı dosya
	// gömülü modülün önüne geçmemeli.
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	writeTR(t, other, "lib/yardimci.tr", "TR_var SELAM = \"SAHTE\"\nTR_func topla(a, b) { TR_return 0 }\n")
	temp := t.TempDir()
	env := []string{"TMP=" + temp, "TEMP=" + temp, "TMPDIR=" + temp}
	s = testkit.RunBinary(t, exit, other, "", env)
	testkit.Wait(t, s, 0, []string{expected}, nil)
	if strings.Contains(s.Stdout, "SAHTE") {
		t.Error("bulunulan klasördeki dosya gömülü modülün yerine geçti")
	}
	if inputs, _ := reporoot.ReadDir(temp); len(inputs) != 0 {
		t.Errorf("gömülü modüllerin geçici klasörü silinmedi: %v", inputs)
	}
}

func TestBuildCleansUpOnRunError(t *testing.T) {
	source := t.TempDir()
	writeTR(t, source, "ana.tr", "TR_import \"m\"\nTR_print(bol(1))\n")
	writeTR(t, source, "m.tr", "TR_func bol(a) {\n    TR_return a / 0\n}\n")
	exit := filepath.Join(t.TempDir(), "app.exe")
	testkit.Wait(t, tr(t, source, "derle", "ana.tr", exit), 0, nil, nil)

	temp := t.TempDir()
	env := []string{"TMP=" + temp, "TEMP=" + temp, "TMPDIR=" + temp}
	s := testkit.RunBinary(t, exit, t.TempDir(), "", env)
	testkit.Wait(t, s, 1, nil, []string{"❌ Çalışma Zamanı Hatası:", "Sıfıra bölme hatası"})
	if inputs, _ := reporoot.ReadDir(temp); len(inputs) != 0 {
		t.Errorf("hata ile çıkışta geçici klasör silinmedi: %v", inputs)
	}
}

func TestBuildModuleErrors(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "eksik.tr", "TR_import \"yok_boyle_modul\"\nTR_print(1)\n")
	writeTR(t, dir, "bozuk.tr", "TR_import \"bozuk_modul\"\n")
	writeTR(t, dir, "bozuk_modul.tr", "TR_func f( {\n")

	exit := filepath.Join(dir, "x.exe")
	s := tr(t, dir, "derle", "eksik.tr", exit)
	testkit.Wait(t, s, 1, nil, []string{"❌ Derleme hatası:", "Satır 2", "modülü okunamadı", "yok_boyle_modul.tr"})
	if _, err := os.Stat(exit); err == nil {
		t.Error("hatalı derlemede çıktı dosyası oluşmamalı")
	}

	s = tr(t, dir, "derle", "bozuk.tr", exit)
	testkit.Wait(t, s, 1, nil, []string{"❌ Derleme Öncesi Sözdizimi Hataları (bozuk_modul.tr modülünde):"})

	writeTR(t, dir, "tip.tr", "TR_var x: TR_Int = \"a\"\n")
	testkit.Wait(t, tr(t, dir, "derle", "tip.tr", exit), 1, nil, []string{"❌ Derleme Öncesi Tip Güvenliği Hataları:"})

	// Modüller de tip denetiminden geçer; modülde üst düzey 'sonuç' hatadır.
	writeTR(t, dir, "tipli.tr", "TR_import \"tip_modul\"\n")
	writeTR(t, dir, "tip_modul.tr", "TR_print(1)\nTR_var y: TR_Int = \"b\"\n")
	testkit.Wait(t, tr(t, dir, "derle", "tipli.tr", exit), 1, nil, []string{"❌ Derleme Öncesi Tip Güvenliği Hataları (tip_modul.tr modülünde):", "Satır 3: Tip uyuşmazlığı", "3 | TR_var y: TR_Int"})
	writeTR(t, dir, "sonuclu.tr", "TR_import \"sonuc_modul\"\n")
	writeTR(t, dir, "sonuc_modul.tr", "TR_return 1\n")
	testkit.Wait(t, tr(t, dir, "derle", "sonuclu.tr", exit), 1, nil, []string{"(sonuc_modul.tr modülünde)", "'TR_return' modülün üst düzeyinde kullanılamaz"})
}

// tr derle kaynağın ya da başka bir .tr dosyasının üzerine yazmaz (veri kaybı).
func TestBuildDoesNotOverwriteSource(t *testing.T) {
	dir := t.TempDir()
	program := "TR_print(\"uygulama\")\n"
	writeTR(t, dir, "uyg.tr", program)
	writeTR(t, dir, "baska.tr", "TR_print(2)\n")
	for _, exit := range []string{"uyg.tr", "./uyg.tr", filepath.Join(dir, "uyg.tr"), "baska.tr", "yeni.TR"} {
		s := tr(t, dir, "derle", "uyg.tr", exit)
		testkit.Wait(t, s, 1, nil, []string{"❌ Derleme hatası:"})
	}
	for ad, content := range map[string]string{"uyg.tr": "# TR 4\n" + program, "baska.tr": "# TR 4\nTR_print(2)\n"} {
		if data, _ := os.ReadFile(filepath.Join(dir, ad)); string(data) != content {
			t.Errorf("%s değişti: %q", ad, data)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "yeni.TR")); err == nil {
		t.Error(".tr uzantılı çıktı oluşmamalı")
	}
}

// tr kontrol aktarılan modülleri de denetler; modül hatası modülün adı ve satırıyla yazılır.
func TestCheckModules(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "ana.tr", "TR_import \"lib/m\"\nTR_print(\"ana\")\n")
	writeTR(t, dir, "lib/m.tr", "TR_import \"ic\"\n")
	writeTR(t, dir, "lib/ic.tr", "TR_var a = 1\nTR_const S = 1\nS = 2\n")
	s := tr(t, dir, "kontrol", "ana.tr")
	testkit.Wait(t, s, 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):", "Satır 4: Sabit hatası", "4 | S = 2"})
	if strings.Contains(s.Stdout, "ana") {
		t.Errorf("kontrol programı çalıştırmamalı: %q", s.Stdout)
	}
	writeTR(t, dir, "eksik.tr", "TR_import \"olmayan\"\n")
	testkit.Wait(t, tr(t, dir, "kontrol", "eksik.tr"), 1, nil, []string{"❌ Modül hatası:", "olmayan.tr", "modülü okunamadı"})
	writeTR(t, dir, "lib/ic.tr", "TR_var a = 1\n")
	testkit.Wait(t, tr(t, dir, "kontrol", "ana.tr"), 0, []string{"başarıyla denetlendi"}, nil)
}

// Modüldeki çalışma hatası modülün adı, satırı ve kod çerçevesiyle yazılır.
func TestModuleRunErrorFrame(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "ana.tr", "TR_import \"m2\"\n\nTR_print(1)\n")
	writeTR(t, dir, "m2.tr", "TR_var x = 1\n\nTR_var z = x / 0\n")
	s := tr(t, dir, "ana.tr")
	testkit.Wait(t, s, 1, nil, []string{"• m2.tr, Satır 4: Sıfıra bölme hatası", "4 | TR_var z = x / 0"})
	if strings.Contains(s.Stderr, "TR_import \"m2\"") {
		t.Errorf("çerçeve ana programın satırını göstermemeli:\n%s", s.Stderr)
	}
}

// Gömülü Türkçe kod listesi kurulamayınca (bozuk ya da yarım değişmiş tr.json) tr paniklemez:
// başlarken okunur bir iç hatayla çıkar (codes.LanguageError). İkili bozuk listeyle derlenir.
func TestABrokenLanguageFileIsAnInternalError(t *testing.T) {
	if testing.Short() {
		t.Skip("tr'yi yeniden derler")
	}
	exe := testkit.BuildWithFile(t, "./cmd/tr", "internal/lang/codes/tr.json", `{ "TR_kw_if": `)
	dir := t.TempDir()
	writeTR(t, dir, "boş.tr", "")
	s := testkit.RunBinary(t, exe, dir, "", nil, "boş.tr")
	testkit.Wait(t, s, 2, nil, []string{"language list tr"})
	if strings.Contains(s.Stderr, "goroutine") {
		t.Errorf("panik yerine iç hata beklenirdi:\n%s", s.Stderr)
	}
}
