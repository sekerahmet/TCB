package e2e

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Dosya kimliklerle yazılır (dil tanımı 2.1): ilk satırı '# TR 4' başlığıdır, dili yoktur. Bir yazım
// dilindeki görünüm 'tr göster' ile alınır ve başlığında dilini taşır ('# TR 4 view=tr'). Görünüm yalnız
// okunur: çalışmaz, koda geri çevrilmez (17.7). Görünüm satırları tutar; konumlar dosyanın satırlarıdır.

// progressLine ilerleme çubuğunun bir satırıdır ("adım 3/10 ▕██░▏ ...").
var progressLine = regexp.MustCompile(`(?m)^.*▕.*▏.*\n?`)

// tr göster: görünüm satırları tutar, başlığa görünümün dilini yazar ('view=tr'), dosyayı değiştirmez.
// Görünüm çalışmaz ve denetlenmez: ileti kodlu dosyayı çalıştırmayı söyler (17.7). Görünümden kodlama
// komutu yoktur; yardım yazılabilen komutları gösterir.
func TestShowCommand(t *testing.T) {
	k := t.TempDir()
	coded := source.Header + "\r\nTR_var x = 1\r\nTR_print(\"x: \\(x)\") # yorum\r\n"
	filePath := testkit.WriteFile(t, k, "a.tr", coded)
	testkit.Wait(t, tr(t, k, "a.tr"), 0, []string{"x: 1"}, nil)
	shown := tr(t, k, "göster", "a.tr", "--dil", "tr")
	if want := source.Header + " view=tr\r\ndeğişken x = 1\r\nyazdır(\"x: \\(x)\") # yorum\r\n"; shown.Code != 0 || shown.Stdout != want {
		t.Errorf("tr göster: %q (çıkış %d, hata %q), beklenen %q", shown.Stdout, shown.Code, shown.Stderr, want)
	}
	if s := tr(t, k, "göster", "a.tr", "--dil", "en"); s.Code != 0 || s.Stdout != source.Header+" view=en\r\nvar x = 1\r\nprint(\"x: \\(x)\") # yorum\r\n" {
		t.Errorf("tr göster --dil en: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	if readFile(t, filePath) != coded {
		t.Errorf("tr göster dosyayı değiştirdi: %q", readFile(t, filePath))
	}
	// Görünüm çalışmaz ve denetlenmez; ileti başlığın satırındadır ve kodlu dosyayı çalıştırmayı söyler.
	testkit.WriteFile(t, k, "g.tr", shown.Stdout)
	for _, args := range [][]string{{"g.tr"}, {"kontrol", "g.tr"}} {
		testkit.Wait(t, tr(t, k, args...), 1, nil, []string{"Satır 1", "bu dosya bir 'tr' görünümü", "kodlu dosyayı çalıştırın", "tr show"})
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, k, "", []string{"TR_DIL=en"}, "g.tr"), 1, nil, []string{"this file is a 'tr' view", "run the coded file"})
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok; var olanlar: en, tr"})
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil"), 2, nil, []string{"'--lang' bir yazım dili bekler"})
	testkit.Wait(t, tr(t, k, "goster", "a.tr"), 2, nil, []string{"'tr show' yazmak mı istediniz?"})
	// Görünümden kodlama yoktur: 'kodla' bir komut değil, okunamayan bir dosya adıdır; yardımda da geçmez.
	for _, name := range []string{"kodla", "çöz"} {
		testkit.Wait(t, tr(t, k, name, "a.tr"), 1, nil, []string{"\"" + name + "\" okunamadı"})
	}
	if readFile(t, filePath) != coded {
		t.Errorf("dosya değişti: %q", readFile(t, filePath))
	}
	help := tr(t, k, "yardım")
	testkit.Wait(t, help, 0, []string{"tr show <dosya.tr> [--lang tr]", "tr show - [--lang tr]"}, nil)
	for _, gone := range []string{"kodla", "çöz"} {
		if strings.Contains(help.Stdout, gone) {
			t.Errorf("yardımda %q geçiyor:\n%s", gone, help.Stdout)
		}
	}
}

// Standart giriş kipi (düzenleyiciler için): 'tr göster -' metni girişten okur, çıkışa yalnızca
// görünümü yazar; BOM ve CRLF korunur, başlıksız metin değişmez. Ayrıştırılamayan metnin görünümü
// yazılmaz ve 2 koduyla (sözdizimi hatası) çıkar; başka hatalar 1 koduyla.
func TestShowStandardInput(t *testing.T) {
	coded := "\xef\xbb\xbf" + source.Header + "\r\nTR_var eğer_değil = 1\r\nTR_print(eğer_değil) # yorum\r\n"
	shown := "\xef\xbb\xbf" + source.Header + " view=tr\r\ndeğişken eğer_değil = 1\r\nyazdır(eğer_değil) # yorum\r\n"
	if s := testkit.RunBinary(t, trBinary, "", coded, nil, "göster", "-"); s.Code != 0 || s.Stdout != shown || s.Stderr != "" {
		t.Errorf("tr göster -: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	plain := "değişken x = 1\n"
	if s := testkit.RunBinary(t, trBinary, "", plain, nil, "göster", "-"); s.Code != 0 || s.Stdout != plain {
		t.Errorf("başlıksız metnin gösterimi değişti: %q (çıkış %d)", s.Stdout, s.Code)
	}
	corrupt := testkit.RunBinary(t, trBinary, "", source.Header+"\nTR_if {\n", nil, "göster", "-")
	testkit.Wait(t, corrupt, 2, nil, []string{"Sözdizimi Hataları"})
	if corrupt.Stdout != "" {
		t.Errorf("ayrıştırılamayan metinde çıktı boş olmalı: %q", corrupt.Stdout)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "# TR 3\nTR_var x = 1\n", nil, "göster", "-"), 1, nil, []string{"okunamayan bir başlık", source.Header})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "göster", "-", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "göster", "-", "a.tr"), 2, nil, []string{"fazladan argüman: a.tr"})
}

// İngilizce görünüm: kodlu dosya İngilizce gösterilir (başlık dilsizdir). Kullanıcının adları ve metin
// anahtarları ({"adım": 3}; 3.7) yazıldığı gibi kalır, sözlüğün üyesi (ayar.TR_length) çevrilir. Üye
// adıyla aynı kullanıcı adı (name: İngilizcede Yol'un üyesi) değişken yerinde işaretsiz yazılır: üye
// yalnız noktadan sonradır. Okunamayan kodlu dosyanın görünümü yoktur.
func TestEnglishView(t *testing.T) {
	coded := source.Header + "\n" +
		"TR_var ayar = {\"adım\": 3, \"renk\": \"mavi\"}\n" +
		"TR_for i TR_in 0..ayar[\"adım\"] {\n" +
		"    TR_if i % 2 == 0 { TR_print(\"\\(i) \\(ayar[\"renk\"])\") }\n" +
		"}\n" +
		"TR_var name = ayar.TR_length\n" +
		"TR_print(name)\n"
	englishText := source.Header + " view=en\n" +
		"var ayar = {\"adım\": 3, \"renk\": \"mavi\"}\n" +
		"for i in 0..ayar[\"adım\"] {\n" +
		"    if i % 2 == 0 { print(\"\\(i) \\(ayar[\"renk\"])\") }\n" +
		"}\n" +
		"var name = ayar.length\n" +
		"print(name)\n"
	english := testkit.RunBinary(t, trBinary, "", coded, nil, "göster", "-", "--dil", "en")
	if english.Code != 0 || english.Stdout != englishText {
		t.Fatalf("tr göster - --dil en:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", english.Stdout, english.Code, english.Stderr, englishText)
	}
	// İletideki komut adı kişinin dilindedir.
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", []string{"TR_DIL=en"}, "show", "-", "a.tr"), 2, nil, []string{"'tr show -' reads"})
	unreadable := source.Header + "\nTR_var l = [1]\nTR_yok(l)\n"
	if s := testkit.RunBinary(t, trBinary, "", unreadable, nil, "göster", "-", "--dil", "en"); s.Code == 0 {
		t.Errorf("okunamayan dosyanın gösterimi verildi: %q", s.Stdout)
	}
	k := t.TempDir()
	testkit.WriteFile(t, k, "k.tr", unreadable)
	if s := tr(t, k, "göster", "k.tr", "--dil", "en"); s.Code == 0 {
		t.Errorf("dosyadan okunamayan gösterim verildi: %q", s.Stdout)
	}
}

// Başlıksız metin dosya değildir (2.1): çalışmaz, ileti başlığı söyler ve kodlamayı önermez. Kabuğa
// yazılan satır koddur, kimlik yazımıyla okunur (17.7): bir yazım dilinin adı (yazdır) kullanıcının
// adıdır. Değerlerin metni dilden bağımsızdır (3.14 ilk yanı).
func TestHeaderlessFileDoesNotRun(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "ana.tr", "yazdır(1)\n")
	for _, env := range [][]string{nil, {"TR_DIL=en"}} {
		s := testkit.RunBinary(t, trBinary, dir, "", env, "ana.tr")
		testkit.Wait(t, s, 1, nil, []string{source.Header, "TR_print"})
		if strings.Contains(s.Stdout+s.Stderr, "kodla") || strings.Contains(s.Stdout+s.Stderr, "encode") {
			t.Errorf("%v: ileti kodlamayı önermemeli:\n%s%s", env, s.Stdout, s.Stderr)
		}
	}
	repl := testkit.RunBinary(t, trBinary, dir, "TR_var x = [1, 2].TR_map((y) => y * 10)\nTR_print(x, TR_Type(x), TR_true)\nexit\n", []string{"TR_DIL=en"})
	testkit.Wait(t, repl, 0, []string{"[10, 20] List true"}, nil)
	typed := testkit.RunBinary(t, trBinary, dir, "yazdır(1)\nprint(2)\nçıkış\n", nil)
	testkit.Wait(t, typed, 0, nil, []string{"'yazdır'", "'print'"})
	if strings.Contains(typed.Stdout, "1\n") || strings.Contains(typed.Stdout, "2\n") {
		t.Errorf("kişinin dilinde yazılan satır çalışmamalı:\n%s", typed.Stdout)
	}
}

// readFile dosyanın metnidir.
func readFile(t *testing.T, filePath string) string {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Dosyanın dili yoktur: kişinin dili (TR_DIL) programın anlamını değiştirmez. Bir dilin bağlamsal
// sözcüğüyle aynı adlı kullanıcı adı (it, acc, init, print) kimliklerden ayrıdır: 'it' ve 'acc' sıradan
// adlardır (sondaki bloğun örtük parametresi yoktur, 6.7), 'init' adlı metot kurucu sayılmaz.
func TestUserNamesNeverBecomeLanguageWords(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"it.tr":    "TR_var it = 4\nTR_print([1, 2].TR_map((x) => x + it))\n",
		"acc.tr":   "TR_var acc = 1\nTR_print([1, 2].TR_reduce(0, (a, x) => a + x + acc))\n",
		"init.tr":  "TR_struct K {\n    a\n    TR_func init() { TR_return 5 }\n}\nTR_print(K(1).init())\n",
		"print.tr": "TR_var print = 5\nTR_print(print)\n",
	}
	want := map[string]string{"it.tr": "[5, 6]\n", "acc.tr": "5\n", "init.tr": "5\n", "print.tr": "5\n"}
	for name, program := range cases {
		testkit.WriteFile(t, dir, name, source.Header+"\n"+program)
		for _, lang := range []string{"TR_DIL=tr", "TR_DIL=en"} {
			if s := testkit.RunBinary(t, trBinary, dir, "", []string{lang}, name); s.Code != 0 || s.Stdout != want[name] {
				t.Errorf("%s %s: çıktı (kod %d) %q, beklenen %q\nhata:\n%s", lang, name, s.Code, s.Stdout, want[name], s.Stderr)
			}
		}
	}
}

// Sözlük anahtarı bir metindir ve köşeli parantezle okunur ve yazılır (3.7): s["adım"] metin anahtarı
// bulur ve günceller; "step" anahtarı çevrilmez. Sonuç çalıştıranın diline (TR_DIL) bağlı değildir.
func TestDictMemberKeysDoNotDependOnTheLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "d.tr", source.Header+"\nTR_var s = {\"tür\": 1, \"adım\": 2}\nTR_print(s[\"tür\"], s[\"adım\"])\ns[\"adım\"] = 5\n"+
		"TR_var k = {\"step\": 3}\nTR_print(s, k.TR_keys())\n")
	want := "1 2\n{\"tür\": 1, \"adım\": 5} [\"step\"]\n"
	for _, lang := range []string{"TR_DIL=tr", "TR_DIL=en"} {
		if s := testkit.RunBinary(t, trBinary, dir, "", []string{lang}, "d.tr"); s.Code != 0 || s.Stdout != want {
			t.Errorf("%s: çıktı (kod %d) %q, beklenen %q\nhata:\n%s", lang, s.Code, s.Stdout, want, s.Stderr)
		}
	}
}

// Tanı iletilerindeki dil adları kişinin yazım dilinde gösterilir: TR_DIL=en ile bir yerleşiğe
// yanlış adlı argüman veren program İngilizce adla (rms_norm) bildirilir; TR_DIL=tr ile Türkçe adla
// (rms_normu).
func TestDiagnosticsUseDisplayNames(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "yanlis.tr", source.Header+"\nTR_rms_norm(1, yanlis: 2)\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "yanlis.tr"), 1, nil, []string{"rms_norm()", "eps, dtype, weight"})
	if s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "yanlis.tr"); strings.Contains(s.Stderr+s.Stdout, "rms_normu()") {
		t.Errorf("TR_DIL=en iletisinde Türkçe ad var:\n%s%s", s.Stdout, s.Stderr)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "yanlis.tr"), 1, nil, []string{"rms_normu()"})
}
