package e2e

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Dosya kimliklerle yazılır (dil tanımı 2.1): ilk satırı '# TR 4' başlığıdır, dili yoktur. Bir yazım
// dilindeki gösterim 'tr göster' ile alınır ve başlığında dilini taşır ('# TR 4 view=tr'), 'tr kodla'
// dili oradan okuyup dosyaya döndürür. Gösterim satırları tutar; konumlar dosyanın satırlarıdır.

// trFilesWithData testlerin veri olarak (metnini) okuduğu .tr dosyalarıdır.
var trFilesWithData = map[string]bool{"bom_ornegi.tr": true}

// digits örnek çıktılarındaki süreleri (ms/adım) karşılaştırmadan çıkarmak içindir.
var digits = regexp.MustCompile(`[0-9]+`)

// progressLine ilerleme çubuğunun bir satırıdır ("adım 3/10 ▕██░▏ ...").
var progressLine = regexp.MustCompile(`(?m)^.*▕.*▏.*\n?`)

// tr kodla ve tr göster komutları: başlıksız gösterim yerinde kodlanır, başlıklı (kodlu) dosyaya ve
// hatalı dosyaya dokunulmaz; gösterim dosyayı değiştirmez; komut adlarının tek yazımı.
func TestEncodeShowCommands(t *testing.T) {
	k := t.TempDir()
	view := "değişken x = 1\r\nyazdır(\"x: \\(x)\") # yorum\r\n"
	filePath := testkit.WriteFile(t, k, "a.tr", view)
	corrupt := testkit.WriteFile(t, k, "alt/bozuk.tr", "eğer {\n")
	// Başlıklı dosya zaten kodludur: kullanıcının 'eğer' adı Türkçe gösterimde okunup TR_if olmaz.
	coded := source.Header + "\nTR_var eğer = 1\nTR_print(eğer)\n"
	codedPath := testkit.WriteFile(t, k, "kodlu.tr", coded)

	s := tr(t, k, "kodla", k)
	testkit.Wait(t, s, 1, []string{"kodlandı: ", "kodlu.tr\" zaten kodlu, dokunulmadı", "3 dosya incelendi, 1 dosya kodlandı."}, []string{"bozuk.tr: sözdizimi hatası olan kaynak çevrilmez"})
	if want := source.Header + "\r\nTR_var x = 1\r\nTR_print(\"x: \\(x)\") # yorum\r\n"; readFile(t, filePath) != want {
		t.Errorf("kodlu dosya: %q, beklenen %q", readFile(t, filePath), want)
	}
	if v := readFile(t, corrupt); v != "eğer {\n" {
		t.Errorf("hatalı dosyaya dokunuldu: %q", v)
	}
	if v := readFile(t, codedPath); v != coded {
		t.Errorf("kodlu dosyaya dokunuldu: %q", v)
	}
	// Kodlu dosya çevrilmeden çalışır; yeniden kodlanmaz.
	testkit.Wait(t, tr(t, k, "a.tr"), 0, []string{"x: 1"}, nil)
	testkit.Wait(t, tr(t, k, "kodlu.tr"), 0, []string{"1"}, nil)
	testkit.Wait(t, tr(t, k, "kodla", "a.tr"), 0, []string{"1 dosya incelendi, 0 dosya kodlandı."}, nil)
	// Gösterim satırları tutar, başlığa görünümün dilini yazar ('view=tr'), dosyayı değiştirmez.
	shown := tr(t, k, "göster", "a.tr", "--dil", "tr")
	if shown.Code != 0 || shown.Stdout != source.Header+" view=tr\r\n"+view {
		t.Errorf("tr göster: %q (çıkış %d, hata %q)", shown.Stdout, shown.Code, shown.Stderr)
	}
	if s := tr(t, k, "göster", "a.tr", "--dil", "en"); s.Code != 0 || s.Stdout != source.Header+" view=en\r\nvar x = 1\r\nprint(\"x: \\(x)\") # yorum\r\n" {
		t.Errorf("tr göster --dil en: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	// Göster → düzenle → kodla: görünüm çalışmaz, kodlama komutunu söyler; 'tr kodla' dili başlıktan
	// okur ('--dil' gerekmez, başka bir dil hatadır) ve dosya çalışır.
	g := testkit.WriteFile(t, k, "g.tr", strings.Replace(shown.Stdout, "x: ", "x = ", 1))
	testkit.Wait(t, tr(t, k, "g.tr"), 1, nil, []string{"bu dosya bir 'tr' görünümü", "tr kodla <dosya>"})
	testkit.Wait(t, tr(t, k, "kodla", "g.tr", "--dil", "en"), 1, nil, []string{"metin bir 'tr' görünümü, ama --dil en verildi"})
	testkit.Wait(t, tr(t, k, "kodla", "g.tr"), 0, []string{"1 dosya incelendi, 1 dosya kodlandı."}, nil)
	if want := source.Header + "\r\nTR_var x = 1\r\nTR_print(\"x = \\(x)\") # yorum\r\n"; readFile(t, g) != want {
		t.Errorf("görünüm kodlanınca: %q, beklenen %q", readFile(t, g), want)
	}
	testkit.Wait(t, tr(t, k, "g.tr"), 0, []string{"x = 1"}, nil)
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok; var olanlar: en, tr"})
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil"), 2, nil, []string{"'--dil' bir yazım dili bekler"})
	testkit.Wait(t, tr(t, k, "goster", "a.tr"), 2, nil, []string{"'tr göster' yazmak mı istediniz?"})
	// tr çöz yoktur: ad bir komut değil, okunamayan bir dosya adıdır; yardımda da geçmez.
	testkit.Wait(t, tr(t, k, "çöz", "a.tr"), 1, nil, []string{"\"çöz\" okunamadı"})
	help := tr(t, k, "yardım")
	testkit.Wait(t, help, 0, []string{"tr kodla <dosya.tr|klasör> [--dil tr]", "tr göster <dosya.tr> [--dil tr]"}, nil)
	if strings.Contains(help.Stdout, "çöz") {
		t.Errorf("yardımda tr çöz geçiyor:\n%s", help.Stdout)
	}
}

// Standart giriş kipi (düzenleyiciler için): 'tr kodla -' ve 'tr göster -' metni girişten okur,
// çıkışa yalnızca çevrilmiş metni yazar; BOM ve CRLF korunur, iki yön birbirinin tersidir. 'tr kodla
// -' girişi gösterim olarak okur: görünüm başlığı ('view=tr') varsa dili oradan, yoksa '--dil'den ya
// da TR_DIL'den alır.
// Ayrıştırılamayan metin çevrilmez ve 2 koduyla (sözdizimi hatası) çıkar; başka hatalar 1 koduyla.
func TestEncodeShowStandardInput(t *testing.T) {
	view := "\xef\xbb\xbfdeğişken eğer_değil = 1\r\nyazdır(eğer_değil) # yorum\r\n"
	encode := testkit.RunBinary(t, trBinary, "", view, nil, "kodla", "-", "--dil", "tr")
	want := "\xef\xbb\xbf" + source.Header + "\r\nTR_var eğer_değil = 1\r\nTR_print(eğer_değil) # yorum\r\n"
	if encode.Code != 0 || encode.Stdout != want || encode.Stderr != "" {
		t.Fatalf("tr kodla -: %q (çıkış %d, hata %q), beklenen %q", encode.Stdout, encode.Code, encode.Stderr, want)
	}
	shown := "\xef\xbb\xbf" + source.Header + " view=tr\r\ndeğişken eğer_değil = 1\r\nyazdır(eğer_değil) # yorum\r\n"
	if s := testkit.RunBinary(t, trBinary, "", encode.Stdout, nil, "göster", "-"); s.Code != 0 || s.Stdout != shown {
		t.Errorf("tr göster -: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	// Görünüm (TR_DIL=en'de de, dili başlığındadır) ve kimliklerle yazılmış metin aynı kodlu biçime
	// kodlanır; başlıksız metin gösterilince değişmez.
	for _, in := range []string{shown, encode.Stdout} {
		if s := testkit.RunBinary(t, trBinary, "", in, nil, "kodla", "-"); s.Code != 0 || s.Stdout != encode.Stdout {
			t.Errorf("%q kodlandı: %q (çıkış %d)", in, s.Stdout, s.Code)
		}
	}
	if s := testkit.RunBinary(t, trBinary, "", shown, []string{"TR_DIL=en"}, "kodla", "-"); s.Code != 0 || s.Stdout != encode.Stdout {
		t.Errorf("TR_DIL=en'de Türkçe görünüm kodlandı: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", shown, nil, "kodla", "-", "--dil", "en"), 1, nil, []string{"metin bir 'tr' görünümü"})
	if s := testkit.RunBinary(t, trBinary, "", view, nil, "göster", "-"); s.Code != 0 || s.Stdout != view {
		t.Errorf("başlıksız metnin gösterimi değişti: %q (çıkış %d)", s.Stdout, s.Code)
	}
	corrupt := testkit.RunBinary(t, trBinary, "", "eğer {\n", nil, "kodla", "-")
	testkit.Wait(t, corrupt, 2, nil, []string{"Sözdizimi Hataları", "Satır 2, Sütun 1"})
	if corrupt.Stdout != "" {
		t.Errorf("ayrıştırılamayan metinde çıktı boş olmalı: %q", corrupt.Stdout)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", source.Header+"\nTR_if {\n", nil, "göster", "-"), 2, nil, []string{"Sözdizimi Hataları"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "# TR 3\nTR_var x = 1\n", nil, "kodla", "-"), 1, nil, []string{"okunamayan bir başlık", source.Header})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "göster", "-", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "kodla", "-", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "kodla", "-", "a.tr"), 2, nil, []string{"fazladan argüman: a.tr"})
	testkit.Wait(t, tr(t, "", "yardım"), 0, []string{"tr göster - [--dil tr]", "tr kodla - [--dil tr]"}, nil)
}

// İngilizce yazım dili: kodlu dosya İngilizce gösterilir, İngilizce yazılmış metin aynı kodlu
// biçime kodlanır (başlık dilsizdir). Kullanıcının adları yazıldığı gibi kalır; üye adıyla aynı
// sözlük anahtarı ({adım: 3}) üyeyle (ayar.adım) birlikte çevrilir. İngilizcede bir sözcük olarak
// okunacak kullanıcı adı '$ad' kaçışıyla yazılır (name: İngilizcede Yol'un üyesi).
func TestEnglishWritingLanguage(t *testing.T) {
	turkish := "değişken ayar = {adım: 3, renk: \"mavi\"}\n" +
		"döngü i içinde 0..ayar.adım {\n" +
		"    eğer i % 2 == 0 { yazdır(\"\\(i) \\(ayar.renk)\") }\n" +
		"}\n" +
		"değişken len = {name: ayar.uzunluk}\n" +
		"yazdır(len.name)\n"
	englishText := source.Header + " view=en\n" +
		"var ayar = {step: 3, renk: \"mavi\"}\n" +
		"for i in 0..ayar.step {\n" +
		"    if i % 2 == 0 { print(\"\\(i) \\(ayar.renk)\") }\n" +
		"}\n" +
		"var len = {$name: ayar.length}\n" +
		"print(len.$name)\n"
	coded := testkit.RunBinary(t, trBinary, "", turkish, nil, "kodla", "-")
	if coded.Code != 0 {
		t.Fatalf("tr kodla -: çıkış %d, hata %q", coded.Code, coded.Stderr)
	}
	english := testkit.RunBinary(t, trBinary, "", coded.Stdout, nil, "göster", "-", "--dil", "en")
	if english.Code != 0 || english.Stdout != englishText {
		t.Fatalf("tr göster - --dil en:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", english.Stdout, english.Code, english.Stderr, englishText)
	}
	back := testkit.RunBinary(t, trBinary, "", english.Stdout, nil, "kodla", "-", "--dil", "en")
	if back.Code != 0 || back.Stdout != coded.Stdout {
		t.Fatalf("İngilizce metin aynı kodlu biçime kodlanmadı:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", back.Stdout, back.Code, back.Stderr, coded.Stdout)
	}
	if s := testkit.RunBinary(t, trBinary, "", back.Stdout, nil, "göster", "-"); s.Code != 0 || s.Stdout != source.Header+" view=tr\n"+turkish {
		t.Errorf("İngilizceden kodlanan metnin Türkçesi özgün değil: %q", s.Stdout)
	}
	// Dosya ve klasör kipi de metni kişinin yazım dilinde (TR_DIL) okur, '--dil' ile başkasında;
	// başlık eklenir ve kodlu dosya çalışır.
	englishFile := "var x = 1\nprint(x)\n"
	wantCoded := source.Header + "\nTR_var x = 1\nTR_print(x)\n"
	for _, c := range []struct {
		env  []string
		args []string
	}{
		{[]string{"TR_DIL=en"}, []string{"kodla", "en.tr"}},
		{nil, []string{"kodla", "--dil", "en", "."}},
	} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "en.tr", englishFile)
		testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", c.env, c.args...), 0, nil, nil)
		if got := readFile(t, filepath.Join(dir, "en.tr")); got != wantCoded {
			t.Errorf("%v %v: kodlu dosya %q, beklenen %q", c.env, c.args, got, wantCoded)
		}
		testkit.Wait(t, tr(t, dir, "en.tr"), 0, []string{"1\n"}, nil)
	}
	testkit.Wait(t, tr(t, t.TempDir(), "kodla", "a.tr", "--dil"), 2, nil, []string{"'--dil' bir yazım dili bekler"})
	testkit.Wait(t, tr(t, t.TempDir(), "kodla", "a.tr", "--dil", "xx"), 2, nil, []string{"'xx' yazım dili yok"})
	// İletideki komut adı kişinin dilindedir.
	enEnv := []string{"TR_DIL=en"}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, t.TempDir(), "", enEnv, "encode", "a.tr", "--x"), 2, nil, []string{"for 'tr encode'"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", enEnv, "encode", "-", "a.tr"), 2, nil, []string{"'tr encode -' reads"})
	// Metin --dil'in dilinde okunur; iletiler kişinin yazım dilindedir (TR_DIL).
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "if {\n", []string{"TR_DIL=en"}, "kodla", "-", "--dil", "en"), 2, nil, []string{"Syntax Errors"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "if {\n", nil, "kodla", "-", "--dil", "en"), 2, nil, []string{"Sözdizimi"})
	// Okunamayan kodlu dosyanın (kod listesinde olmayan kimlik) gösterimi yoktur: komut bir gösterim
	// vermez (dosyayla da standart girişle de).
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

// Depodaki .tr dosyalarının İngilizce gidiş dönüşü: kodlu → İngilizce → kodlu bayt bayt aynıdır
// (İngilizcenin sözcükleriyle çakışan kullanıcı adları kaçışla korunur).
func TestRepoFilesEnglishLossless(t *testing.T) {
	if testing.Short() {
		t.Skip("her dosya için iki süreç çalıştırır")
	}
	var files []string
	for _, k := range []string{"tests", "examples", "lib", "tools"} {
		reporoot.WalkDir(reporoot.Path(k), func(filePath string, g fs.DirEntry, err error) error {
			if err == nil && g.IsDir() && (g.Name() == "negative" || g.Name() == errorsDir["tr"]) {
				return filepath.SkipDir // bilerek hatalı dosyalar (bozuk başlıklar, hata örnekleri)
			}
			if err == nil && !g.IsDir() && strings.HasSuffix(filePath, ".tr") && !trFilesWithData[g.Name()] && !brokenFixture(filePath) {
				files = append(files, filePath)
			}
			return err
		})
	}
	tested := 0
	for _, filePath := range files {
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}
		english := testkit.RunBinary(t, trBinary, "", string(data), nil, "göster", "-", "--dil", "en")
		back := testkit.RunBinary(t, trBinary, "", english.Stdout, nil, "kodla", "-", "--dil", "en")
		want, err := source.NFCNames(string(data)) // yalnız kullanıcı adları NFC yazılır (2.3)
		if err != nil {
			t.Fatalf("%s: %v", filePath, err)
		}
		if english.Code != 0 || back.Code != 0 || back.Stdout != want {
			t.Errorf("%s: İngilizce gidiş dönüş kayıplı (çıkış %d/%d, hata %q%q)\n%s", filePath, english.Code, back.Code, english.Stderr, back.Stderr, testkit.FirstLineDiff(string(data), back.Stdout, "dosya", "geri"))
			continue
		}
		tested++
	}
	if tested < 40 {
		t.Fatalf("yalnızca %d dosya sınandı", tested)
	}
}

// Başlıksız metin dosya değildir (2.1): çalışmaz, ileti başlığı ve kodlama komutunu (kişinin dili
// --dil ile) söyler. Kabuğa yazılan satır kişinin yazım dilindedir; değerlerin metni dilden
// bağımsızdır (3.14 ilk yanı).
func TestHeaderlessFileDoesNotRun(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "ana.tr", "yazdır(1)\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", nil, "ana.tr"), 1, nil, []string{source.Header, "tr kodla <dosya> --dil tr"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "ana.tr"), 1, nil, []string{source.Header, "tr encode <file> --lang en"})
	repl := testkit.RunBinary(t, trBinary, dir, "var x = [1, 2].map { it * 10 }\nprint(x, type(x), true)\nexit\n", []string{"TR_DIL=en"})
	testkit.Wait(t, repl, 0, []string{"[10, 20] List true"}, nil)
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
// sözcüğüyle aynı adlı kullanıcı adı (it, acc, init, print) kimliklerden ayrıdır: 'it' adlı değişken
// sondaki bloğun örtük parametresine (TR_it) düşmez, 'init' adlı metot kurucu sayılmaz.
func TestUserNamesNeverBecomeLanguageWords(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"it.tr":    "TR_var it = 4\nTR_print([1, 2].TR_map { TR_it + it })\n",
		"acc.tr":   "TR_var acc = 1\nTR_print([1, 2].TR_reduce(0) { TR_acc + TR_it + acc })\n",
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

// Sözlükte kullanıcı adıyla üye erişimi (s.tür, s.adım) metin anahtarı bulur ve günceller; kimlikle
// yazılan çıplak anahtar ({TR_step: 3}) dilden bağımsız adıyla saklanır ("step", 17.3). Sonuç
// çalıştıranın diline (TR_DIL) bağlı değildir.
func TestDictMemberKeysDoNotDependOnTheLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "d.tr", source.Header+"\nTR_var s = {\"tür\": 1, \"adım\": 2}\nTR_print(s.tür, s.adım)\ns.adım = 5\n"+
		"TR_var k = {TR_step: 3}\nTR_print(s, k.TR_keys())\n")
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

// brokenFixture bilerek sözdizimi hatalı yazılmış araç fikstürüdür (codegraph'ın atlanan dosya
// testi); gösterimi yoktur.
func brokenFixture(filePath string) bool {
	return filepath.ToSlash(filePath) == filepath.ToSlash(reporoot.Path("tools/codegraph/testdata/mod/examples/broken.tr"))
}
