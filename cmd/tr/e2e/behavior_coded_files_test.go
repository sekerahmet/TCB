package e2e

import (
	"errors"
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

// Kodlu dosya Türkçesiyle aynı programdır (docs/spec/17_kodlar.md, 17.4): tests/, examples/ ve
// lib/ iki kopyaya alınır, birinin bütün .tr dosyaları kodlu biçime çevrilir; iki
// kopyada 'tr test' aynı raporu, örnekler aynı çıktıyı verir. (TR → kod → TR bayt bayt
// kimliği internal/lang/source'ta sınanır.)

// copyFile kaynak klasörü (alt klasörleriyle) hedefe kopyalar.

// trFilesWithData testlerin veri olarak (metnini) okuduğu .tr dosyalarıdır; kodlu kopyada
// da Türkçe kalırlar (tests/dosya_ve_yol_test.tr, 18.11: BOM'lu dosyanın ilk satırı).
var trFilesWithData = map[string]bool{"bom_ornegi.tr": true}

// codedCopy klasörün altındaki bütün .tr dosyalarını yerinde kodlar; ayrıştırılamayan
// (bilerek hatalı) dosyalar Türkçe kalır. Kodlanan dosya sayısını verir.
func codedCopy(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := reporoot.WalkDir(dir, func(filePath string, g fs.DirEntry, err error) error {
		if err != nil || g.IsDir() || !strings.HasSuffix(filePath, ".tr") || trFilesWithData[g.Name()] {
			return err
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		coded, err := source.Encode(string(data))
		var sh *source.SyntaxError
		var bh *source.HeaderError
		if errors.As(err, &sh) || errors.As(err, &bh) {
			return nil
		}
		if err != nil {
			t.Errorf("%s kodlanamadı: %v", filePath, err)
			return nil
		}
		if coded != string(data) {
			n++
		}
		return os.WriteFile(filePath, []byte(coded), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// digits örnek çıktılarındaki süreleri (ms/adım) karşılaştırmadan çıkarmak içindir.
var digits = regexp.MustCompile(`[0-9]+`)

// progressLine ilerleme çubuğunun bir satırıdır ("adım 3/10 ▕██░▏ ...").
var progressLine = regexp.MustCompile(`(?m)^.*▕.*▏.*\n?`)

func TestCodedFilesGiveSameOutput(t *testing.T) {
	tr1, tr2 := t.TempDir(), t.TempDir()
	// docs/ karakter modelinin eğitim verisidir.
	for _, k := range []string{"tests", "examples", "docs", "lib"} {
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(tr1, k))
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(tr2, k))
	}
	// Karakter modelinin eğitimi kısaltılır (iki kopyada da aynı biçimde).
	for _, root := range []string{tr1, tr2} {
		filePath := filepath.Join(root, "examples", "tr", "36_karakter_modeli.tr")
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "sabit ADIM = 300") {
			t.Fatal("36_karakter_modeli.tr: 'sabit ADIM = 300' yok; kısaltma eğitimi kısaltmıyor")
		}
		os.WriteFile(filePath, []byte(strings.Replace(string(data), "sabit ADIM = 300", "sabit ADIM = 10", 1)), 0o644)
	}
	n := codedCopy(t, tr2)
	if n < 35 {
		t.Fatalf("yalnızca %d dosya kodlandı", n)
	}

	// Çıktıda geçen kopya klasörlerinin yolları (ör. kontrol noktasının "Kaldığı yerden
	// devam: ..." satırı) karşılaştırmadan çıkarılır.
	stripRoot := func(s result, root string) result {
		for _, k := range []string{filepath.ToSlash(root), root} {
			s.Stdout, s.Stderr = strings.ReplaceAll(s.Stdout, k, "<kök>"), strings.ReplaceAll(s.Stderr, k, "<kök>")
		}
		return s
	}
	// Kitaplığın testleri: kodlanan test dosyalarıdır (aktarılan kitaplık ikiliye gömülüdür).
	// Kontrol noktasının "Kaldığı yerden devam: ..." satırı geçici klasörün yolunu içerir.
	resumeLine := regexp.MustCompile(`(?m)^Kaldığı yerden devam: .*$`)
	// geçici_klasör() her süreçte başka bir klasör verir (tr_<süreç>_<sayı>, 18.14); yolu
	// karşılaştırmadan çıkarılır.
	tempDir := regexp.MustCompile(`[^\s"']*tr_\d+_\d+`)
	// İki kopya ayrı klasörlerdedir: her karşılaştırmanın iki koşusu aynı anda yapılır.
	both := func(args ...string) (result, result) {
		return pair(t,
			func(t *testing.T) result { return tr(t, tr1, args...) },
			func(t *testing.T) result { return tr(t, tr2, args...) })
	}
	for _, dir := range []string{"tests", "examples", "lib"} {
		s1, s2 := both("test", dir)
		s1, s2 = stripRoot(s1, tr1), stripRoot(s2, tr2)
		for _, s := range []*result{&s1, &s2} {
			s.Stdout, s.Stderr = resumeLine.ReplaceAllString(s.Stdout, ""), resumeLine.ReplaceAllString(s.Stderr, "")
			s.Stdout, s.Stderr = tempDir.ReplaceAllString(s.Stdout, "<geçici>"), tempDir.ReplaceAllString(s.Stderr, "<geçici>")
		}
		if s1.Code != 0 || s1.Code != s2.Code || s1.Stdout != s2.Stdout || s1.Stderr != s2.Stderr {
			t.Errorf("tr test %s: Türkçe ve kodlu kopyalar farklı (çıkış %d / %d)\n%s", dir, s1.Code, s2.Code, testkit.FirstLineDiff(s1.Stdout+s1.Stderr, s2.Stdout+s2.Stderr, "Türkçe", "kodlu"))
		}
	}
	examples, _ := filepath.Glob(filepath.Join(tr1, "examples", "tr", "*.tr"))
	if len(examples) < 5 {
		t.Fatalf("yalnızca %d örnek bulundu", len(examples))
	}
	for _, o := range examples {
		ad := filepath.ToSlash(filepath.Join("examples", "tr", filepath.Base(o)))
		s1, s2 := both(ad)
		// İlerleme çubuğu terminal dışında zamanla ara satır yazar (19.7); yavaş makinede ara
		// satır iki koşuda farklı adıma düşer. Çubuk satırları karşılaştırılmaz.
		c1 := digits.ReplaceAllString(progressLine.ReplaceAllString(s1.Stdout+s1.Stderr, ""), "#")
		c2 := digits.ReplaceAllString(progressLine.ReplaceAllString(s2.Stdout+s2.Stderr, ""), "#")
		if s1.Code != 0 || s1.Code != s2.Code || c1 != c2 {
			t.Errorf("tr %s: Türkçe ve kodlu kopyalar farklı (çıkış %d / %d)\n%s", ad, s1.Code, s2.Code, testkit.FirstLineDiff(c1, c2, "Türkçe", "kodlu"))
		}
	}
}

// tr kodla, tr çöz ve tr göster komutları (17): yerinde çeviri, gösterim, hatalı dosyaya
// dokunmama ve komut adlarının tek yazımı.
func TestEncodeDecodeShowCommands(t *testing.T) {
	k := t.TempDir()
	tr1 := "değişken x = 1\r\nyazdır(\"x: {x}\") // yorum\r\n"
	filePath := testkit.WriteFile(t, k, "a.tr", tr1)
	corrupt := testkit.WriteFile(t, k, "alt/bozuk.tr", "eğer {\n")

	s := tr(t, k, "kodla", k)
	testkit.Wait(t, s, 1, []string{"kodlandı: ", "2 dosya incelendi, 1 dosya kodlandı."}, []string{"bozuk.tr: sözdizimi hatası olan kaynak çevrilmez"})
	data, _ := os.ReadFile(filePath)
	if want := source.Header + "tr\r\nA030 x = 1\r\nG100(\"x: {x}\") // yorum\r\n"; string(data) != want {
		t.Errorf("kodlu dosya: %q, beklenen %q", data, want)
	}
	if v, _ := os.ReadFile(corrupt); string(v) != "eğer {\n" {
		t.Errorf("hatalı dosyaya dokunuldu: %q", v)
	}
	// Kodlu dosya çevrilmeden çalışır; zaten kodlu dosya yeniden kodlanmaz.
	testkit.Wait(t, tr(t, k, "a.tr"), 0, []string{"x: 1"}, nil)
	testkit.Wait(t, tr(t, k, "kodla", "a.tr"), 0, []string{"1 dosya incelendi, 0 dosya kodlandı."}, nil)
	// Gösterim dosyayı değiştirmez.
	if s := tr(t, k, "göster", "a.tr", "--dil", "tr"); s.Code != 0 || s.Stdout != tr1 {
		t.Errorf("tr göster: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	if s := tr(t, k, "göster", "a.tr", "--dil", "en"); s.Code != 0 || s.Stdout != "var x = 1\r\nprint(\"x: {x}\") // yorum\r\n" {
		t.Errorf("tr göster --dil en: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok; var olanlar: en, tr"})
	testkit.Wait(t, tr(t, k, "göster", "a.tr", "--dil"), 1, nil, []string{"'--dil' bir yazım dili bekler"})
	testkit.Wait(t, tr(t, k, "çöz", "a.tr"), 0, []string{"çözüldü: a.tr"}, nil)
	if v, _ := os.ReadFile(filePath); string(v) != tr1 {
		t.Errorf("çözülen dosya özgün değil: %q", v)
	}
	testkit.Wait(t, tr(t, k, "coz", "a.tr"), 1, nil, []string{"'tr çöz' yazmak mı istediniz?"})
	testkit.Wait(t, tr(t, k, "yardım"), 0, []string{"tr kodla <dosya.tr|klasör>", "tr çöz <dosya.tr|klasör>", "tr göster <dosya.tr> [--dil tr]"}, nil)
}

// Standart giriş kipi (düzenleyiciler için; 17_kodlar.md, "Düzenleyicide"): 'tr kodla -' ve 'tr göster -' metni
// girişten okur, çıkışa yalnızca çevrilmiş metni yazar; BOM ve CRLF korunur, iki yön birbirinin
// tersidir. Ayrıştırılamayan metin çevrilmez ve 2 koduyla (sözdizimi hatası) çıkar; başka
// hatalar 1 koduyla.
func TestEncodeShowStandardInput(t *testing.T) {
	turkish := "\xef\xbb\xbfdeğişken eğer_değil = 1\r\nyazdır(eğer_değil) // yorum\r\n"
	encode := testkit.RunBinary(t, trBinary, "", turkish, nil, "kodla", "-", "--dil", "tr")
	if want := "\xef\xbb\xbf" + source.Header + "tr\r\nA030 eğer_değil = 1\r\nG100(eğer_değil) // yorum\r\n"; encode.Code != 0 || encode.Stdout != want || encode.Stderr != "" {
		t.Fatalf("tr kodla -: %q (çıkış %d, hata %q), beklenen %q", encode.Stdout, encode.Code, encode.Stderr, want)
	}
	if s := testkit.RunBinary(t, trBinary, "", encode.Stdout, nil, "göster", "-"); s.Code != 0 || s.Stdout != turkish {
		t.Errorf("tr göster -: %q (çıkış %d, hata %q)", s.Stdout, s.Code, s.Stderr)
	}
	// Zaten kodlu metin kodlanınca, Türkçe metin gösterilince değişmez.
	if s := testkit.RunBinary(t, trBinary, "", encode.Stdout, nil, "kodla", "-"); s.Code != 0 || s.Stdout != encode.Stdout {
		t.Errorf("kodlu metin yeniden kodlandı: %q (çıkış %d)", s.Stdout, s.Code)
	}
	if s := testkit.RunBinary(t, trBinary, "", turkish, nil, "göster", "-"); s.Code != 0 || s.Stdout != turkish {
		t.Errorf("Türkçe metnin gösterimi değişti: %q (çıkış %d)", s.Stdout, s.Code)
	}
	corrupt := testkit.RunBinary(t, trBinary, "", "eğer {\n", nil, "kodla", "-")
	testkit.Wait(t, corrupt, 2, nil, []string{"Sözdizimi Hataları", "Satır 2, Sütun 1"})
	if corrupt.Stdout != "" {
		t.Errorf("ayrıştırılamayan metinde çıktı boş olmalı: %q", corrupt.Stdout)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", source.Header+"tr\nA001 {\n", nil, "göster", "-"), 2, nil, []string{"Sözdizimi Hataları"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "//! tr kodlu 1\n", nil, "kodla", "-"), 1, nil, []string{"bu biçim sürümü okunamıyor"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "göster", "-", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "kodla", "-", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "x\n", nil, "kodla", "-", "a.tr"), 1, nil, []string{"fazladan argüman: a.tr"})
	testkit.Wait(t, tr(t, "", "yardım"), 0, []string{"tr göster - [--dil tr]", "tr kodla - [--dil tr]"}, nil)
}

// İngilizce yazım dili (17.6): kodlu dosya İngilizce gösterilir, İngilizce yazılmış metin aynı
// coded biçime kodlanır. Kullanıcının adları yazıldığı gibi kalır; üye adıyla aynı sözlük
// anahtarı ({adım: 3}) üyeyle (ayar.adım) birlikte çevrilir. İngilizcede bir sözcük olarak
// okunacak kullanıcı adı '$ad' kaçışıyla yazılır (name: İngilizcede Yol'un üyesi); kullanıcının
// görünen tanımı olan ad (len) kaçışsızdır.
func TestEnglishWritingLanguage(t *testing.T) {
	turkish := "değişken ayar = {adım: 3, renk: \"mavi\"}\n" +
		"döngü i içinde 0..ayar.adım {\n" +
		"    eğer i % 2 == 0 { yazdır(\"{i} {ayar.renk}\") }\n" +
		"}\n" +
		"değişken len = {name: ayar.uzunluk}\n" +
		"yazdır(len.name)\n"
	englishText := "var ayar = {step: 3, renk: \"mavi\"}\n" +
		"for i in 0..ayar.step {\n" +
		"    if i % 2 == 0 { print(\"{i} {ayar.renk}\") }\n" +
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
	// İngilizce yazılmış metin dil=en başlığıyla kodlanır (17.1); gövde aynı kodlu biçimdir.
	back := testkit.RunBinary(t, trBinary, "", english.Stdout, nil, "kodla", "-", "--dil", "en")
	if back.Code != 0 || back.Stdout != inLanguage(coded.Stdout, "en") {
		t.Fatalf("İngilizce metin aynı kodlu biçime kodlanmadı:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", back.Stdout, back.Code, back.Stderr, coded.Stdout)
	}
	if s := testkit.RunBinary(t, trBinary, "", back.Stdout, nil, "göster", "-"); s.Code != 0 || s.Stdout != turkish {
		t.Errorf("İngilizceden kodlanan metnin Türkçesi özgün değil: %q", s.Stdout)
	}
	// Metin --dil'in dilinde okunur; iletiler kişinin yazım dilindedir (TR_DIL): İngilizce
	// metnin sözdizimi hatası TR_DIL=en ile İngilizce başlıkla, yoksa Türkçe bildirilir.
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "if {\n", []string{"TR_DIL=en"}, "kodla", "-", "--dil", "en"), 2, nil, []string{"Syntax Errors"})
	testkit.Wait(t, testkit.RunBinary(t, trBinary, "", "if {\n", nil, "kodla", "-", "--dil", "en"), 2, nil, []string{"Sözdizimi"})
	// Gösterim de o dilde yeniden kodlanabilmeli: G200'ün İngilizce adı yok, "G200(l)" diye bir
	// gösterim geri okunamaz; komut onu vermez (dosyayla da standart girişle de).
	lossy := source.Header + "tr\nA030 l = [1]\nG200(l)\n"
	if s := testkit.RunBinary(t, trBinary, "", lossy, nil, "göster", "-", "--dil", "en"); s.Code == 0 {
		t.Errorf("geri okunamayan gösterim verildi: %q", s.Stdout)
	}
	k := t.TempDir()
	testkit.WriteFile(t, k, "k.tr", lossy)
	if s := tr(t, k, "göster", "k.tr", "--dil", "en"); s.Code == 0 {
		t.Errorf("dosyadan geri okunamayan gösterim verildi: %q", s.Stdout)
	}
}

// inLanguage kodlu metnin başlığındaki dili language yapar (gövde aynı kalır).
func inLanguage(coded, language string) string {
	b, err := source.Split(coded)
	if err != nil || !b.Coded {
		return coded
	}
	b.Language = language
	return b.Join()
}

// Depodaki .tr dosyalarının İngilizce gidiş dönüşü: kodlu → İngilizce → kodlu bayt bayt aynıdır
// (İngilizcenin sözcükleriyle çakışan kullanıcı adları kaçışla korunur).
func TestRepoFilesEnglishLossless(t *testing.T) {
	if testing.Short() {
		t.Skip("her dosya için üç süreç çalıştırır")
	}
	var files []string
	for _, k := range []string{"tests", "examples", "lib", "tools"} {
		reporoot.WalkDir(reporoot.Path(k), func(filePath string, g fs.DirEntry, err error) error {
			if err == nil && g.IsDir() && (g.Name() == "negative" || g.Name() == errorsDir["tr"] || g.Name() == errorsDir["en"]) {
				return filepath.SkipDir // bilerek hatalı dosyalar (ölü kodlar, bozuk başlıklar, hata örnekleri)
			}
			if err == nil && !g.IsDir() && strings.HasSuffix(filePath, ".tr") && !trFilesWithData[g.Name()] {
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
		coded := testkit.RunBinary(t, trBinary, "", string(data), nil, "kodla", "-")
		if coded.Code == exitSyntax {
			continue // bilerek hatalı olumsuz test dosyaları
		}
		english := testkit.RunBinary(t, trBinary, "", coded.Stdout, nil, "göster", "-", "--dil", "en")
		back := testkit.RunBinary(t, trBinary, "", english.Stdout, nil, "kodla", "-", "--dil", "en")
		if coded.Code != 0 || english.Code != 0 || back.Code != 0 || back.Stdout != inLanguage(coded.Stdout, "en") {
			t.Errorf("%s: İngilizce gidiş dönüş kayıplı (çıkış %d/%d/%d, hata %q%q)\n%s", filePath, coded.Code, english.Code, back.Code, english.Stderr, back.Stderr, testkit.FirstLineDiff(coded.Stdout, back.Stdout, "Türkçe", "kodlu"))
			continue
		}
		tested++
	}
	if tested < 40 {
		t.Fatalf("yalnızca %d dosya sınandı", tested)
	}
}

// Kodsuz (başlıksız) dosya etkin dil ne olursa olsun Türkçedir (17.7): TR_DIL=en ile de Türkçe
// okunur ve aynı sonucu verir. Bağlamsal sözcükler (olarak, kurucu, kendi, o, birikim, hata)
// ve kurucusuz yapının üye adlı alanları da Türkçe tanınır; aktarılan modül de Türkçedir.
// Sözdizimi hatası dosyanın kendi metnine göre gösterilir; 'tr göster --dil en' dosyayı
// İngilizce gösterir; dosyaya yazan 'tr kodla' ve 'tr biçimle' onu Türkçe okur.
func TestUncodedFileIsTurkishInEveryLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "m.tr", "fonksiyon iki_kat(x) { sonuç x * 2 }\n")
	program := "aktar \"m\" olarak m\n" +
		"değişken kat = 5\n" +
		"yapı Ayar {\n    öğrenme_oranı = 0.5\n    adım = 1\n}\n" +
		"yapı Kutu {\n    boy: Tam\n" +
		"    fonksiyon kurucu(kenar, çarpan = 1) { kendi.boy = kenar * çarpan }\n" +
		"    fonksiyon alan() { sonuç kendi.boy * kendi.boy }\n}\n" +
		"değişken a = Ayar(öğrenme_oranı: 0.25)\n" +
		"değişken k = Kutu(3, çarpan: 2)\n" +
		"değişken l = [1, 2, 3].haritala { o * kat }\n" +
		"dene { fırlat \"x\" } yakala { yazdır(\"yakalandı {hata}\") }\n" +
		"yazdır(k.alan(), l, l.indirge(0) { birikim + o }, m.iki_kat(4), a.öğrenme_oranı, a.adım)\n"
	testkit.WriteFile(t, dir, "ana.tr", program)
	want := "yakalandı x\n36 [5, 10, 15] 30 8 0.25 1\n"
	for _, env := range [][]string{nil, {"TR_DIL=en"}} {
		testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", env, "ana.tr"), 0, []string{want}, nil)
	}
	if s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "ana.tr"); s.Stdout != want {
		t.Errorf("TR_DIL=en çıktısı %q, beklenen %q", s.Stdout, want)
	}

	testkit.WriteFile(t, dir, "bozuk.tr", "değişken x = )\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "bozuk.tr"), 1, nil, []string{"1 | değişken x = )"})

	shown := tr(t, dir, "göster", "ana.tr", "--dil", "en")
	testkit.Wait(t, shown, 0, []string{"import \"m\" as m\n", "func init(kenar, çarpan = 1) { self.boy = kenar * çarpan }", "{ acc + it }"}, nil)

	testkit.WriteFile(t, dir, "bicim.tr", "değişken  y=1\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "biçimle", "bicim.tr"), 0, nil, nil)
	if got := readFile(t, filepath.Join(dir, "bicim.tr")); got != "değişken y = 1\n" {
		t.Errorf("TR_DIL=en ile biçimlenen Türkçe dosya %q", got)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "kodla", "ana.tr"), 0, nil, nil)
	if got := readFile(t, filepath.Join(dir, "ana.tr")); !strings.HasPrefix(got, source.Header) {
		t.Fatalf("TR_DIL=en ile 'tr kodla' Türkçe dosyayı kodlamadı:\n%s", got)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "çöz", "ana.tr"), 0, nil, nil)
	if got := readFile(t, filepath.Join(dir, "ana.tr")); got != program {
		t.Errorf("TR_DIL=en ile 'tr çöz' Türkçe yazmadı:\n%s", got)
	}
	// Kabuğa yazılan satır kişinin yazım dilindedir; kanonik biçime çevrilip çalışır. Değerler
	// satırın yazıldığı dilde metne çevrilir (3, 17.1): tip() adı İngilizcedir.
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

// Denetim (2026-10-21): kodsuz Türkçe dosya etkin dil ne olursa olsun aynı sonucu vermeli
// (17.7), ama TR_DIL=en ile dosya İngilizce görünümüyle çalışır ve görünümde kaçışla
// ($ad) ayrılan bir kullanıcı adı çalışma anında etkin dilin bağlamsal sözcüğüyle aynı ada
// düşer: 'it' adlı değişken sondaki bloğun örtük parametresine (o), 'acc' birikime yakalanır
// (sessizce yanlış sonuç), 'init' adlı metot kurucu sayılır, 'print' adlı değişken yazdır'ı
// gölgeler. Kodlu dosyada da aynıdır (tr kodla sonrası TR_DIL=en).
func TestAuditUncodedTurkishNamesCollideWithActiveLanguageWords(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"it.tr":    "değişken it = 4\nyazdır([1, 2].haritala { o + it })\n",
		"acc.tr":   "değişken acc = 1\nyazdır([1, 2].indirge(0) { birikim + o + acc })\n",
		"init.tr":  "yapı K {\n    a\n    fonksiyon init() { sonuç 5 }\n}\nyazdır(K(1).init())\n",
		"print.tr": "değişken print = 5\nyazdır(print)\n",
	}
	for name, program := range cases {
		testkit.WriteFile(t, dir, name, program)
		want := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, name)
		got := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, name)
		if got.Code != want.Code || got.Stdout != want.Stdout {
			t.Errorf("%s: TR_DIL=en sonucu (kod %d) %q, TR_DIL=tr (kod %d) %q\nhata:\n%s",
				name, got.Code, got.Stdout, want.Code, want.Stdout, got.Stderr)
		}
	}
}

// Denetim (2026-10-21): sözlükte üye erişimi (s.adım, s.tür) ve tırnaksız anahtar ({adım: 2})
// üye koduna (Y500, Y402) çevrilir ve TR_DIL=en ile İngilizce adıyla (step, dtype) okunur:
// Türkçe yazılmış dosyada sözlüğün anahtarı yazım diline göre değişir; okuma yok verir, yazma
// yeni bir "step" anahtarı ekler (JSON'a, kontrol noktasına giden veri değişir). Dil tanımı
// 17.7'nin testi de ("Kodlu dosyada üyenin adıyla aynı sözlük anahtarı Türkçe adıyla okunur",
// tests/kodlu_saklama_test.tr) TR_DIL=en ile kalır.
func TestAuditDictMemberKeysChangeWithWritingLanguage(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "d.tr", "değişken s = {\"tür\": 1, \"adım\": 2}\nyazdır(s.tür, s.adım)\ns.adım = 5\n"+
		"değişken k = {adım: 3}\nyazdır(s, k.anahtarlar())\n")
	want := "1 2\n{\"tür\": 1, \"adım\": 5} [\"adım\"]\n"
	for _, lang := range []string{"TR_DIL=tr", "TR_DIL=en"} {
		if s := testkit.RunBinary(t, trBinary, dir, "", []string{lang}, "d.tr"); s.Code != 0 || s.Stdout != want {
			t.Errorf("%s: çıktı (kod %d) %q, beklenen %q\nhata:\n%s", lang, s.Code, s.Stdout, want, s.Stderr)
		}
	}
}

// Tanı iletilerindeki dil adları kişinin yazım dilinde gösterilir (17.7): TR_DIL=en ile bir
// yerleşiğe yanlış adlı argüman veren program İngilizce adla (rms_norm) bildirilir; TR_DIL=tr
// ile Türkçe adla (rms_normu).
func TestDiagnosticsUseDisplayNames(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "yanlis.tr", "rms_normu(1, yanlis: 2)\n")
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "yanlis.tr"), 1, nil, []string{"rms_norm()", "eps, dtype, weight"})
	if s := testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "yanlis.tr"); strings.Contains(s.Stderr+s.Stdout, "rms_normu()") {
		t.Errorf("TR_DIL=en iletisinde Türkçe ad var:\n%s%s", s.Stdout, s.Stderr)
	}
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=tr"}, "yanlis.tr"), 1, nil, []string{"rms_normu()"})
}
