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

// Kodlu dosya Türkçesiyle aynı programdır (docs/dil/17_kodlar.md, 17.4): testler/ ve
// ornekler/ iki kopyaya alınır, birinin bütün .tr dosyaları kodlu biçime çevrilir; iki
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
	for _, k := range []string{"testler", "ornekler", "docs"} {
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

	for _, klasor := range []string{"testler", "ornekler"} {
		s1, s2 := tr(t, tr1, "test", klasor), tr(t, tr2, "test", klasor)
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
	bekle(t, tr(t, k, "göster", "a.tr", "--dil", "en"), 1, nil, []string{"'en' yazım dili yok; bugün yalnızca 'tr' var"})
	bekle(t, tr(t, k, "göster", "a.tr", "--dil"), 1, nil, []string{"'--dil' bir yazım dili bekler"})
	bekle(t, tr(t, k, "çöz", "a.tr"), 0, []string{"çözüldü: a.tr"}, nil)
	if v, _ := os.ReadFile(yol); string(v) != tr1 {
		t.Errorf("çözülen dosya özgün değil: %q", v)
	}
	bekle(t, tr(t, k, "coz", "a.tr"), 1, nil, []string{"'tr çöz' yazmak mı istediniz?"})
	bekle(t, tr(t, k, "yardım"), 0, []string{"tr kodla <dosya.tr|klasör>", "tr çöz <dosya.tr|klasör>", "tr göster <dosya.tr> [--dil tr]"}, nil)
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
