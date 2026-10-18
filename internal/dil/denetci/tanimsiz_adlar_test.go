package denetci

import (
	"strings"
	"testing"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
)

// 5.14: tanımsız ad çalışmadan önce hatadır; ipucu çalışma anındakiyle aynıdır.
func TestTanimsizAdHatadir(t *testing.T) {
	tekHata(t, "yazdir(1)", "Satır 1, Sütun 1: Tanımsız ad", "'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?")
	tekHata(t, "değişken öğrenci_sayısı = 1\nyazdır(öğrenci_sayis)", "Satır 2, Sütun 8", "'öğrenci_sayis' adında bir değişken ya da fonksiyon yok", "Bunu mu demek istediniz: 'öğrenci_sayısı'?")
	tekHata(t, "tanımsız = 3", "Satır 1, Sütun 1: Tanımsız ad — 'tanımsız'")
	tekHata(t, "değişken l = [1]\nfiltrele(l, (x) => x > 0)", "'filtrele(...)' TR'de bir fonksiyon değil; 'liste.filtrele(...)' yazın")
	tekHata(t, "matris([[1]])", "'matris' TR'de yok; 'tensör' yazmak mı istediniz?")
	tekHata(t, "eğer doğru { değişken iç = 1 }\nyazdır(iç)", "Satır 2", "'iç'")
	tekHata(t, "fonksiyon f() { sonuç g() }", "'g'")
	tekHata(t, "test \"a\" { değişken yerel = 1 }\ntest \"b\" { yazdır(yerel) }", "Satır 2", "'yerel'")
	tekHata(t, "yazdır(\"{bilinmeyen}\")", "'bilinmeyen'")
	tekHata(t, "değişken l = [1, 2]\nyazdır([x * 2 döngü x içinde l eğer y > 0])", "'y'")
	tekHata(t, "kendi.n = 1", "'kendi'")
	// Aynı ad tek kez bildirilir.
	tekHata(t, "yazdır(a)\nyazdır(a + 1)", "Satır 1", "'a'")
}

// Dinamik dilde yanlış alarm yok: aşağıdakilerin hepsi tanımlıdır.
func TestTanimsizAdYanlisAlarmVermez(t *testing.T) {
	durumlar := map[string]string{
		"sonra tanımlanan fonksiyon":  "fonksiyon a() { sonuç b() }\nfonksiyon b() { sonuç 1 }\nyazdır(a())",
		"özyineleme":                  "fonksiyon f(n) { sonuç eğer n < 1 { 0 } değilse { f(n - 1) } }",
		"sondaki blok o":              "yazdır([1, 2].haritala { o * 2 }.filtrele { o > 2 })",
		"ok fonksiyonu":               "değişken k = (x) => x * x\nyazdır(k(3), [1].haritala(y => y + 1))",
		"yakala hata":                 "dene { fırlat \"x\" } yakala { yazdır(hata) }",
		"yakala adlı":                 "dene { fırlat \"x\" } yakala h { yazdır(h) }",
		"döngü değişkenleri":          "döngü i, x içinde [5, 6] { yazdır(i, x) }",
		"liste üreteci":               "yazdır([x * x döngü x içinde 0..3 eğer x > 0])",
		"yapı ve kendi":               "yapı Sayaç { n = 0; fonksiyon artır(k) { kendi.n += k } }\ndeğişken s = Sayaç()\ns.artır(2)\nyazdır(s.n)",
		"üye ve adlı argüman":         "değişken l = [3, 1]\nyazdır(l.sırala(ters: doğru).uzunluk)",
		"sabit ve çoklu":              "sabit PI2 = 6.28\ndeğişken a, b = 1, 2\na, b = b, a\nyazdır(PI2, a, b)",
		"dış değişkene atama":         "değişken n = 0\nfonksiyon art() { n += 1 }\nart()",
		"eşle":                        "değişken p = 5\ndeğişken s = 3\nyazdır(eşle p { s => \"eşit\"\n 1..10 => \"aralık\"\n varsayılan => \"diğer\" })",
		"ertele ve eşzamanlı":         "fonksiyon f() { değişken iz = []\n ertele iz.ekle(1)\n eşzamanlı { yazdır(iz) }\n sonuç iz }",
		"takma adsız aktar":           "aktar \"yardimci_modul\"\nyazdır(kare(PI))",
		"takma adlı aktar":            "aktar \"yardimci_modul\" olarak ym\nyazdır(ym.kare(2))",
		"ifade içinde adlı fonksiyon": "değişken l = [1].haritala(fonksiyon ikiye(x) { sonuç x * 2 })\nyazdır(ikiye(3))",
		"test bloğu genel ad":         "değişken g = 1\ntest \"t\" { eşit_mi(1, g) }",
		"kesme noktası koşulu":        "değişken x = 1\nkesme_noktası eğer x > 10",
		"varsayılan parametre":        "değişken taban = 2\nfonksiyon üs(x, k = taban) { sonuç x ** k }",
	}
	for ad, kaynak := range durumlar {
		if errs := denetle(t, kaynak); len(errs) > 0 {
			t.Errorf("%s: hata beklenmiyordu: %v\n%s", ad, errs, kaynak)
		}
	}
}

// Tip hatası, tanımsız ad ve gölgeleme uyarısı birlikte bildirilir; biri ötekini gizlemez.
// Hatalar satır sırasıyladır.
func TestTipHatasiVeTanimsizAdVeUyariBirlikte(t *testing.T) {
	p := ayristir.New(sozcuk.New("değişken toplam = [1, 2]\nyazdir(toplam)\ndeğişken x: Tam = \"metin\"\n"))
	prog := p.ParseProgram()
	c := New()
	errs := c.CheckProgram(prog)
	if len(errs) != 2 || !strings.HasPrefix(errs[0], "Satır 2, Sütun 1: Tanımsız ad — 'yazdir'") || !strings.HasPrefix(errs[1], "Satır 3: Tip uyuşmazlığı") {
		t.Fatalf("tanımsız ad ve tip hatası satır sırasıyla gelmeli: %q", errs)
	}
	if u := c.Warnings(); len(u) != 1 || !strings.Contains(u[0], "'toplam' yerleşik bir ad") {
		t.Fatalf("gölgeleme uyarısı da verilmeli: %q", u)
	}
}

// Etkileşimli kabukta denetleyici satırlar arasında kalır: önceki satırın tanımı görünür.
func TestTanimsizAdKabuktaSatirlarArasinda(t *testing.T) {
	c := New()
	satir := func(kaynak string) []string {
		p := ayristir.New(sozcuk.New(kaynak))
		return c.CheckProgram(p.ParseProgram())
	}
	if errs := satir("değişken a = 1"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := satir("yazdır(a)"); len(errs) != 0 {
		t.Fatalf("önceki satırın tanımı görünmeli: %v", errs)
	}
	if errs := satir("yazdır(b)"); len(errs) != 1 {
		t.Fatalf("tanımsız ad bildirilmeli: %v", errs)
	}
}

// Modül aktaranın genel kapsamının altında çalışır (11.1): genel adlar modülde tanımlıdır;
// adlar bilinmiyorsa (dinamik) tanımsız ad aranmaz.
func TestTanimsizAdModulGenelKapsami(t *testing.T) {
	modul := "yükleme_kaydı.ekle(1)\nyazdır(bilinmeyen)"
	denetleModul := func(adlar []string, dinamik bool) []string {
		c := New()
		c.GenelKapsam(adlar, dinamik)
		return c.CheckProgram(ayristir.New(sozcuk.New(modul)).ParseModule())
	}
	errs := denetleModul([]string{"yükleme_kaydı"}, false)
	if len(errs) != 1 || !strings.Contains(errs[0], "'bilinmeyen'") {
		t.Fatalf("yalnızca bilinmeyen ad bildirilmeli: %v", errs)
	}
	if errs := denetleModul(nil, true); len(errs) != 0 {
		t.Fatalf("dinamik genel kapsamda tanımsız ad aranmamalı: %v", errs)
	}
	adlar, dinamik := UstDuzeyAdlari(ayristir.New(sozcuk.New("değişken a = 1\nyapı Y { x }\nfonksiyon f() {}\naktar \"m\" olarak mm\neğer doğru { değişken iç = 2 }")).ParseProgram())
	if strings.Join(adlar, ",") != "Y,a,f,mm" || dinamik {
		t.Fatalf("üst düzey adlar: %v %v", adlar, dinamik)
	}
	if _, dinamik := UstDuzeyAdlari(ayristir.New(sozcuk.New("aktar \"m\"")).ParseProgram()); !dinamik {
		t.Fatal("takma adsız aktar adları belirsiz kılmalı")
	}
}
