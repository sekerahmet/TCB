package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/kaynak"
)

// Kodlu dosya Türkçesiyle aynı programdır (docs/dil/17_kodlar.md, 17.4): testler/, ornekler/ ve
// kitaplik/ iki kopyaya alınır, birinin bütün .tr dosyaları kodlu biçime çevrilir; iki
// kopyada 'tr test' aynı raporu, örnekler aynı çıktıyı verir. (TR → kod → TR bayt bayt
// kimliği internal/dil/kaynak'ta sınanır.)

// kopyala kaynak klasörü (alt klasörleriyle) hedefe kopyalar.
func kopyala(t *testing.T, kaynakKlasor, hedef string) {
	t.Helper()
	err := filepath.WalkDir(kaynakKlasor, func(yol string, g fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		goreli, _ := filepath.Rel(kaynakKlasor, yol)
		h := filepath.Join(hedef, goreli)
		if g.IsDir() {
			return os.MkdirAll(h, 0o755)
		}
		veri, err := os.ReadFile(yol)
		if err != nil {
			return err
		}
		return os.WriteFile(h, veri, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// veriOlanTRDosyalari testlerin veri olarak (metnini) okuduğu .tr dosyalarıdır; kodlu kopyada
// da Türkçe kalırlar (testler/dosya_ve_yol_test.tr, 18.11: BOM'lu dosyanın ilk satırı).
var veriOlanTRDosyalari = map[string]bool{"bom_ornegi.tr": true}

// kodluKopya klasörün altındaki bütün .tr dosyalarını yerinde kodlar; ayrıştırılamayan
// (bilerek hatalı) dosyalar Türkçe kalır. Kodlanan dosya sayısını verir.
func kodluKopya(t *testing.T, klasor string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(klasor, func(yol string, g fs.DirEntry, err error) error {
		if err != nil || g.IsDir() || !strings.HasSuffix(yol, ".tr") || veriOlanTRDosyalari[g.Name()] {
			return err
		}
		veri, err := os.ReadFile(yol)
		if err != nil {
			return err
		}
		kodlu, err := kaynak.Kodla(string(veri))
		var sh *kaynak.SozdizimiHatasi
		var bh *kaynak.BaslikHatasi
		if errors.As(err, &sh) || errors.As(err, &bh) {
			return nil
		}
		if err != nil {
			t.Errorf("%s kodlanamadı: %v", yol, err)
			return nil
		}
		if kodlu != string(veri) {
			n++
		}
		return os.WriteFile(yol, []byte(kodlu), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// rakamlar örnek çıktılarındaki süreleri (ms/adım) karşılaştırmadan çıkarmak içindir.
var rakamlar = regexp.MustCompile(`[0-9]+`)

func TestKodluDosyalarAyniCiktiyiVerir(t *testing.T) {
	tr1, tr2 := t.TempDir(), t.TempDir()
	// docs/ karakter modelinin eğitim verisidir.
	for _, k := range []string{"testler", "ornekler", "docs", "kitaplik"} {
		kopyala(t, depo.Yol(k), filepath.Join(tr1, k))
		kopyala(t, depo.Yol(k), filepath.Join(tr2, k))
	}
	// Karakter modelinin eğitimi kısaltılır (iki kopyada da aynı biçimde).
	for _, kok := range []string{tr1, tr2} {
		yol := filepath.Join(kok, "ornekler", "07_karakter_modeli.tr")
		veri, err := os.ReadFile(yol)
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(yol, []byte(strings.Replace(string(veri), "değişken ADIM = 300", "değişken ADIM = 10", 1)), 0o644)
	}
	n := kodluKopya(t, tr2)
	if n < 35 {
		t.Fatalf("yalnızca %d dosya kodlandı", n)
	}

	// Çıktıda geçen kopya klasörlerinin yolları (ör. kontrol noktasının "Kaldığı yerden
	// devam: ..." satırı) karşılaştırmadan çıkarılır.
	kokuSoy := func(s sonuc, kok string) sonuc {
		for _, k := range []string{filepath.ToSlash(kok), kok} {
			s.cikti, s.hata = strings.ReplaceAll(s.cikti, k, "<kök>"), strings.ReplaceAll(s.hata, k, "<kök>")
		}
		return s
	}
	// Kitaplığın testleri: kodlanan test dosyalarıdır (aktarılan kitaplık ikiliye gömülüdür).
	// Kontrol noktasının "Kaldığı yerden devam: ..." satırı geçici klasörün yolunu içerir.
	devamSatiri := regexp.MustCompile(`(?m)^Kaldığı yerden devam: .*$`)
	for _, klasor := range []string{"testler", "ornekler", "kitaplik"} {
		s1, s2 := kokuSoy(tr(t, tr1, "test", klasor), tr1), kokuSoy(tr(t, tr2, "test", klasor), tr2)
		for _, s := range []*sonuc{&s1, &s2} {
			s.cikti, s.hata = devamSatiri.ReplaceAllString(s.cikti, ""), devamSatiri.ReplaceAllString(s.hata, "")
		}
		if s1.kod != 0 || s1.kod != s2.kod || s1.cikti != s2.cikti || s1.hata != s2.hata {
			t.Errorf("tr test %s: Türkçe ve kodlu kopyalar farklı (çıkış %d / %d)\n%s", klasor, s1.kod, s2.kod, satirFarki(s1.cikti+s1.hata, s2.cikti+s2.hata))
		}
	}
	ornekler, _ := filepath.Glob(filepath.Join(tr1, "ornekler", "*.tr"))
	if len(ornekler) < 5 {
		t.Fatalf("yalnızca %d örnek bulundu", len(ornekler))
	}
	for _, o := range ornekler {
		ad := filepath.ToSlash(filepath.Join("ornekler", filepath.Base(o)))
		s1, s2 := tr(t, tr1, ad), tr(t, tr2, ad)
		c1 := rakamlar.ReplaceAllString(s1.cikti+s1.hata, "#")
		c2 := rakamlar.ReplaceAllString(s2.cikti+s2.hata, "#")
		if s1.kod != 0 || s1.kod != s2.kod || c1 != c2 {
			t.Errorf("tr %s: Türkçe ve kodlu kopyalar farklı (çıkış %d / %d)\n%s", ad, s1.kod, s2.kod, satirFarki(c1, c2))
		}
	}
}

// tr kodla, tr çöz ve tr göster komutları (17): yerinde çeviri, gösterim, hatalı dosyaya
// dokunmama ve komut adlarının tek yazımı.
func TestKodlaCozGosterKomutlari(t *testing.T) {
	k := t.TempDir()
	tr1 := "değişken x = 1\r\nyazdır(\"x: {x}\") // yorum\r\n"
	yol := dosyaYaz(t, k, "a.tr", tr1)
	bozuk := dosyaYaz(t, k, "alt/bozuk.tr", "eğer {\n")

	s := tr(t, k, "kodla", k)
	bekle(t, s, 1, []string{"kodlandı: ", "2 dosya incelendi, 1 dosya kodlandı."}, []string{"bozuk.tr: sözdizimi hatası olan kaynak çevrilmez"})
	veri, _ := os.ReadFile(yol)
	if want := kaynak.Baslik + "\r\nA030 x = 1\r\nG100(\"x: {x}\") // yorum\r\n"; string(veri) != want {
		t.Errorf("kodlu dosya: %q, beklenen %q", veri, want)
	}
	if v, _ := os.ReadFile(bozuk); string(v) != "eğer {\n" {
		t.Errorf("hatalı dosyaya dokunuldu: %q", v)
	}
	// Kodlu dosya çevrilmeden çalışır; zaten kodlu dosya yeniden kodlanmaz.
	bekle(t, tr(t, k, "a.tr"), 0, []string{"x: 1"}, nil)
	bekle(t, tr(t, k, "kodla", "a.tr"), 0, []string{"1 dosya incelendi, 0 dosya kodlandı."}, nil)
	// Gösterim dosyayı değiştirmez.
	if s := tr(t, k, "göster", "a.tr", "--dil", "tr"); s.kod != 0 || s.cikti != tr1 {
		t.Errorf("tr göster: %q (çıkış %d, hata %q)", s.cikti, s.kod, s.hata)
	}
	if s := tr(t, k, "göster", "a.tr", "--dil", "en"); s.kod != 0 || s.cikti != "var x = 1\r\nprint(\"x: {x}\") // yorum\r\n" {
		t.Errorf("tr göster --dil en: %q (çıkış %d, hata %q)", s.cikti, s.kod, s.hata)
	}
	bekle(t, tr(t, k, "göster", "a.tr", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok; var olanlar: en, tr"})
	bekle(t, tr(t, k, "göster", "a.tr", "--dil"), 1, nil, []string{"'--dil' bir yazım dili bekler"})
	bekle(t, tr(t, k, "çöz", "a.tr"), 0, []string{"çözüldü: a.tr"}, nil)
	if v, _ := os.ReadFile(yol); string(v) != tr1 {
		t.Errorf("çözülen dosya özgün değil: %q", v)
	}
	bekle(t, tr(t, k, "coz", "a.tr"), 1, nil, []string{"'tr çöz' yazmak mı istediniz?"})
	bekle(t, tr(t, k, "yardım"), 0, []string{"tr kodla <dosya.tr|klasör>", "tr çöz <dosya.tr|klasör>", "tr göster <dosya.tr> [--dil tr]"}, nil)
}

// Standart giriş kipi (düzenleyiciler için; 17_kodlar.md, "Düzenleyicide"): 'tr kodla -' ve 'tr göster -' metni
// girişten okur, çıkışa yalnızca çevrilmiş metni yazar; BOM ve CRLF korunur, iki yön birbirinin
// tersidir. Ayrıştırılamayan metin çevrilmez ve 2 koduyla (sözdizimi hatası) çıkar; başka
// hatalar 1 koduyla.
func TestKodlaGosterStandartGiris(t *testing.T) {
	turkce := "\xef\xbb\xbfdeğişken eğer_değil = 1\r\nyazdır(eğer_değil) // yorum\r\n"
	kodla := calistir(t, trIkilisi, "", turkce, nil, "kodla", "-", "--dil", "tr")
	if want := "\xef\xbb\xbf" + kaynak.Baslik + "\r\nA030 eğer_değil = 1\r\nG100(eğer_değil) // yorum\r\n"; kodla.kod != 0 || kodla.cikti != want || kodla.hata != "" {
		t.Fatalf("tr kodla -: %q (çıkış %d, hata %q), beklenen %q", kodla.cikti, kodla.kod, kodla.hata, want)
	}
	if s := calistir(t, trIkilisi, "", kodla.cikti, nil, "göster", "-"); s.kod != 0 || s.cikti != turkce {
		t.Errorf("tr göster -: %q (çıkış %d, hata %q)", s.cikti, s.kod, s.hata)
	}
	// Zaten kodlu metin kodlanınca, Türkçe metin gösterilince değişmez.
	if s := calistir(t, trIkilisi, "", kodla.cikti, nil, "kodla", "-"); s.kod != 0 || s.cikti != kodla.cikti {
		t.Errorf("kodlu metin yeniden kodlandı: %q (çıkış %d)", s.cikti, s.kod)
	}
	if s := calistir(t, trIkilisi, "", turkce, nil, "göster", "-"); s.kod != 0 || s.cikti != turkce {
		t.Errorf("Türkçe metnin gösterimi değişti: %q (çıkış %d)", s.cikti, s.kod)
	}
	bozuk := calistir(t, trIkilisi, "", "eğer {\n", nil, "kodla", "-")
	bekle(t, bozuk, 2, nil, []string{"Sözdizimi Hataları", "Satır 2, Sütun 1"})
	if bozuk.cikti != "" {
		t.Errorf("ayrıştırılamayan metinde çıktı boş olmalı: %q", bozuk.cikti)
	}
	bekle(t, calistir(t, trIkilisi, "", kaynak.Baslik+"\nA001 {\n", nil, "göster", "-"), 2, nil, []string{"Sözdizimi Hataları"})
	bekle(t, calistir(t, trIkilisi, "", "//! tr kodlu 2\n", nil, "kodla", "-"), 1, nil, []string{"tanımadığı bir kodlu biçim başlığı"})
	bekle(t, calistir(t, trIkilisi, "", "x\n", nil, "göster", "-", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok"})
	bekle(t, calistir(t, trIkilisi, "", "x\n", nil, "kodla", "-", "--dil", "xx"), 1, nil, []string{"'xx' yazım dili yok"})
	bekle(t, calistir(t, trIkilisi, "", "x\n", nil, "kodla", "-", "a.tr"), 1, nil, []string{"fazladan argüman: a.tr"})
	bekle(t, tr(t, "", "yardım"), 0, []string{"tr göster - [--dil tr]", "tr kodla - [--dil tr]"}, nil)
}

// İngilizce yazım dili (17.6): kodlu dosya İngilizce gösterilir, İngilizce yazılmış metin aynı
// kodlu biçime kodlanır. Kullanıcının adları yazıldığı gibi kalır; üye adıyla aynı sözlük
// anahtarı ({adım: 3}) üyeyle (ayar.adım) birlikte çevrilir. İngilizcede bir sözcük olarak
// okunacak kullanıcı adı '$ad' kaçışıyla yazılır (name: İngilizcede Yol'un üyesi); kullanıcının
// görünen tanımı olan ad (len) kaçışsızdır.
func TestIngilizceYazimDili(t *testing.T) {
	turkce := "değişken ayar = {adım: 3, renk: \"mavi\"}\n" +
		"döngü i içinde 0..ayar.adım {\n" +
		"    eğer i % 2 == 0 { yazdır(\"{i} {ayar.renk}\") }\n" +
		"}\n" +
		"değişken len = {name: ayar.uzunluk}\n" +
		"yazdır(len.name)\n"
	ingilizce := "var ayar = {step: 3, renk: \"mavi\"}\n" +
		"for i in 0..ayar.step {\n" +
		"    if i % 2 == 0 { print(\"{i} {ayar.renk}\") }\n" +
		"}\n" +
		"var len = {$name: ayar.length}\n" +
		"print(len.$name)\n"
	kodlu := calistir(t, trIkilisi, "", turkce, nil, "kodla", "-")
	if kodlu.kod != 0 {
		t.Fatalf("tr kodla -: çıkış %d, hata %q", kodlu.kod, kodlu.hata)
	}
	en := calistir(t, trIkilisi, "", kodlu.cikti, nil, "göster", "-", "--dil", "en")
	if en.kod != 0 || en.cikti != ingilizce {
		t.Fatalf("tr göster - --dil en:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", en.cikti, en.kod, en.hata, ingilizce)
	}
	geri := calistir(t, trIkilisi, "", en.cikti, nil, "kodla", "-", "--dil", "en")
	if geri.kod != 0 || geri.cikti != kodlu.cikti {
		t.Fatalf("İngilizce metin aynı kodlu biçime kodlanmadı:\n%s(çıkış %d, hata %q)\nbeklenen:\n%s", geri.cikti, geri.kod, geri.hata, kodlu.cikti)
	}
	if s := calistir(t, trIkilisi, "", geri.cikti, nil, "göster", "-"); s.kod != 0 || s.cikti != turkce {
		t.Errorf("İngilizceden kodlanan metnin Türkçesi özgün değil: %q", s.cikti)
	}
	bekle(t, calistir(t, trIkilisi, "", "if {\n", nil, "kodla", "-", "--dil", "en"), 2, nil, []string{"Sözdizimi Hataları"})
}

// Depodaki .tr dosyalarının İngilizce gidiş dönüşü: kodlu → İngilizce → kodlu bayt bayt aynıdır
// (İngilizcenin sözcükleriyle çakışan kullanıcı adları kaçışla korunur).
func TestDepodakiDosyalarIngilizceKayipsiz(t *testing.T) {
	if testing.Short() {
		t.Skip("her dosya için üç süreç çalıştırır")
	}
	var dosyalar []string
	for _, k := range []string{"testler", "ornekler", "kitaplik", "tools"} {
		filepath.WalkDir(depo.Yol(k), func(yol string, g fs.DirEntry, err error) error {
			if err == nil && g.IsDir() && g.Name() == "olumsuz" {
				return filepath.SkipDir // bilerek hatalı dosyalar (ölü kodlar, bozuk başlıklar)
			}
			if err == nil && !g.IsDir() && strings.HasSuffix(yol, ".tr") && !veriOlanTRDosyalari[g.Name()] {
				dosyalar = append(dosyalar, yol)
			}
			return err
		})
	}
	sinanan := 0
	for _, yol := range dosyalar {
		veri, err := os.ReadFile(yol)
		if err != nil {
			t.Fatal(err)
		}
		kodlu := calistir(t, trIkilisi, "", string(veri), nil, "kodla", "-")
		if kodlu.kod == cikisSozdizimi {
			continue // bilerek hatalı olumsuz test dosyaları
		}
		en := calistir(t, trIkilisi, "", kodlu.cikti, nil, "göster", "-", "--dil", "en")
		geri := calistir(t, trIkilisi, "", en.cikti, nil, "kodla", "-", "--dil", "en")
		if kodlu.kod != 0 || en.kod != 0 || geri.kod != 0 || geri.cikti != kodlu.cikti {
			t.Errorf("%s: İngilizce gidiş dönüş kayıplı (çıkış %d/%d/%d, hata %q%q)\n%s", yol, kodlu.kod, en.kod, geri.kod, en.hata, geri.hata, satirFarki(kodlu.cikti, geri.cikti))
			continue
		}
		sinanan++
	}
	if sinanan < 40 {
		t.Fatalf("yalnızca %d dosya sınandı", sinanan)
	}
}

// satirFarki iki çıktının ayrıldığı ilk satırları gösterir.
func satirFarki(a, b string) string {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x != y {
			return "Türkçe: " + x + "\nkodlu:  " + y
		}
	}
	return "(çıktılar aynı)"
}
