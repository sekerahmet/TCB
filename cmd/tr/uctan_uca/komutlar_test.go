package uctanuca

import (
	"debug/buildinfo"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Komut satırının temel komutları: yardım ve sürüm, komut adları, dosya çalıştırma, kontrol,
// test, biçimle, dap, etkileşimli kabuk ve derle. Hepsi derlenmiş 'tr' ikilisiyle sınanır.

func TestYardimVeSurum(t *testing.T) {
	for _, a := range []string{"yardım", "-h"} {
		s := tr(t, "", a)
		// Yardım her komutu listeler (hakem mimari 22: dap eksikti)
		bekle(t, s, 0, []string{"Kullanım:", "tr derle <dosya.tr>", "tr test", "tr çalıştır", "tr hata_ayıkla",
			"tr kontrol", "tr biçimle", "tr dap", "tr yardım", "tr sürüm"}, nil)
	}
	for _, a := range []string{"sürüm", "-v"} {
		s := tr(t, "", a)
		bekle(t, s, 0, []string{"v" + trSurumu(t)}, nil)
	}
}

// Her komutun tek Türkçe adı vardır: Türkçe karaktersiz ve İngilizce yazımlar dosya adı
// sanılmaz, asıl komutu öneren bir hatayla 1 koduyla çıkar.
func TestKomutlarTekAdli(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "yazdır(\"çalıştı\")\n")
	for ad, asil := range map[string]string{
		"calistir": "çalıştır", "bicimle": "biçimle", "hata_ayikla": "hata_ayıkla", "debug": "hata_ayıkla",
		"yardim": "yardım", "--help": "yardım", "surum": "sürüm", "--version": "sürüm",
	} {
		s := tr(t, klasor, ad, "a.tr")
		bekle(t, s, 1, nil, []string{"'" + ad + "' TR'de bir komut değil; 'tr " + asil + "' yazmak mı istediniz?"})
		if strings.Contains(s.cikti, "çalıştı") {
			t.Errorf("%s: program çalışmamalıydı", ad)
		}
	}
	bekle(t, tr(t, klasor, "biçimle", "--check", "a.tr"), 1, nil, []string{"'--check' TR'de bir bayrak değil; '--kontrol'"})
}

func TestKaldirilmisKomutlar(t *testing.T) {
	for _, a := range []string{"bytecode", "baytkod"} {
		s := tr(t, "", a)
		bekle(t, s, 1, nil, []string{"'tr bytecode' komutu kaldırıldı"})
	}
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "yazdır(1)\n")
	for _, a := range []string{"--web", "-w"} {
		s := tr(t, klasor, "hata_ayıkla", "a.tr", a)
		bekle(t, s, 1, nil, []string{"Tarayıcıdaki hata ayıklayıcı kaldırıldı"})
		if strings.Contains(s.cikti, "1") {
			t.Errorf("program çalışmamalıydı: %q", s.cikti)
		}
	}
}

func TestEksikArguman(t *testing.T) {
	for _, k := range []string{"kontrol", "derle", "hata_ayıkla", "çalıştır", "biçimle"} {
		s := tr(t, "", k)
		bekle(t, s, 1, nil, []string{"Hata: Lütfen"})
	}
}

// Yerleşik ad gölgeleme uyarısı standart hataya, hatalardan ayrı başlıkla yazılır; program
// çalışır ve çıkış kodu 0 kalır.
func TestGolgelemeUyarisi(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "değişken toplam = 5\nyazdır(toplam)\n")
	for _, komut := range [][]string{{"a.tr"}, {"kontrol", "a.tr"}} {
		s := tr(t, klasor, komut...)
		bekle(t, s, 0, nil, []string{"⚠️ Uyarılar:\n  • Satır 1: 'toplam' yerleşik bir ad; bu kapsamda yerleşik 'toplam' artık kullanılamaz", "1 | değişken toplam = 5"})
		if strings.Contains(s.hata, "❌") || strings.Contains(s.cikti, "Uyarı") {
			t.Errorf("%v: uyarı hata başlığıyla ya da standart çıktıya yazılmamalı:\n%s\n%s", komut, s.cikti, s.hata)
		}
	}
	bekle(t, tr(t, klasor, "a.tr"), 0, []string{"5"}, nil)
}

func TestBilinmeyenKomut(t *testing.T) {
	klasor := t.TempDir()
	s := tr(t, klasor, "olmayan_komut")
	bekle(t, s, 1, nil, []string{"Dosya okuma hatası", "olmayan_komut", "böyle bir dosya ya da klasör yok"})
}

func TestDosyaCalistirma(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "iyi.tr", "değişken ad = \"dünya\"\nyazdır(\"merhaba {ad}\")\n")
	dosyaYaz(t, klasor, "sozdizimi.tr", "yazdır(\"a\"\n")
	dosyaYaz(t, klasor, "tip.tr", "değişken x: Tam = \"metin\"\nyazdır(\"çalışmamalı\")\n")
	dosyaYaz(t, klasor, "calisma.tr", "yazdır(\"önce\")\ndeğişken y = 1 / 0\nyazdır(\"sonra\")\n")

	for _, args := range [][]string{{"iyi.tr"}, {"çalıştır", "iyi.tr"}} {
		bekle(t, tr(t, klasor, args...), 0, []string{"merhaba dünya"}, nil)
	}

	s := tr(t, klasor, "sozdizimi.tr")
	bekle(t, s, 1, nil, []string{"❌ Sözdizimi Hataları:", "  • Satır 2, Sütun 1"})

	s = tr(t, klasor, "tip.tr")
	bekle(t, s, 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları:", "Tip uyuşmazlığı", "1 | değişken x: Tam"})
	if strings.Contains(s.cikti, "çalışmamalı") {
		t.Error("tip hatalı program çalıştırılmamalıydı")
	}

	s = tr(t, klasor, "calisma.tr")
	bekle(t, s, 1, []string{"önce"}, []string{"❌ Çalışma Zamanı Hatası:", "Satır 2: Sıfıra bölme hatası", "2 | değişken y = 1 / 0"})
	if strings.Contains(s.cikti, "sonra") {
		t.Error("çalışma hatasından sonra program durmalıydı")
	}
}

func TestKontrol(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "iyi.tr", "yazdır(\"çalışmamalı\")\n")
	dosyaYaz(t, klasor, "tip.tr", "değişken x: Tam = \"metin\"\n")
	dosyaYaz(t, klasor, "sozdizimi.tr", "fonksiyon f( {\n")

	s := tr(t, klasor, "kontrol", "iyi.tr")
	bekle(t, s, 0, []string{"iyi.tr başarıyla denetlendi"}, nil)
	if strings.Contains(s.cikti, "çalışmamalı") {
		t.Error("kontrol programı çalıştırmamalı")
	}
	bekle(t, tr(t, klasor, "kontrol", "tip.tr"), 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları:"})
	bekle(t, tr(t, klasor, "kontrol", "sozdizimi.tr"), 1, nil, []string{"❌ Sözdizimi Hataları:"})
	bekle(t, tr(t, klasor, "kontrol", "yok.tr"), 1, nil, []string{"Dosya okuma hatası", "yok.tr", "böyle bir dosya ya da klasör yok"})
}

func TestTestKomutu(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "gecen/a_test.tr", "test \"toplama\" {\n    eşit_mi(4, 2 + 2)\n}\n")
	dosyaYaz(t, klasor, "kalan/b_test.tr", "test \"yanlış\" {\n    eşit_mi(5, 2 + 2)\n}\n")

	bekle(t, tr(t, klasor, "test", "gecen"), 0, []string{"toplama"}, nil)
	bekle(t, tr(t, klasor, "test", "kalan"), 1, []string{"yanlış"}, nil)
	// Hedefsiz: bulunulan klasördeki tüm *_test.tr dosyaları (biri kalıyor).
	bekle(t, tr(t, klasor, "test"), 1, []string{"toplama", "yanlış"}, nil)
	bekle(t, tr(t, klasor, "test", "olmayan"), 1, nil, []string{"Hata:"})
}

func TestBicimle(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "değişken   x=1\nyazdır( x )\n")

	s := tr(t, klasor, "biçimle", "--kontrol", "a.tr")
	bekle(t, s, 1, []string{"biçimlenmeli: a.tr"}, nil)

	bekle(t, tr(t, klasor, "biçimle", "a.tr"), 0, []string{"biçimlendi: a.tr"}, nil)
	bekle(t, tr(t, klasor, "biçimle", "--kontrol", "."), 0, []string{"hepsi standart biçimde"}, nil)
	bekle(t, tr(t, klasor, "biçimle", "olmayan.tr"), 1, nil, []string{`"olmayan.tr" biçimlenemedi: böyle bir dosya ya da klasör yok`})
	// Sözdizimi hatalı dosya varken "hepsi standart biçimde" denmez
	dosyaYaz(t, klasor, "bozuk.tr", "yazdır(\n")
	s = tr(t, klasor, "biçimle", "--kontrol", ".")
	bekle(t, s, 1, []string{"1 tanesi biçimlenemedi"}, []string{"bozuk.tr"})
	if strings.Contains(s.cikti, "standart biçimde") {
		t.Errorf("hatalı dosya varken başarı yazılmamalı: %q", s.cikti)
	}
}

func TestDAPBaslangici(t *testing.T) {
	govde := `{"seq":1,"type":"request","command":"initialize","arguments":{"adapterID":"tr"}}`
	girdi := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(govde), govde)
	s := calistir(t, trIkilisi, "", girdi, nil, "dap")
	bekle(t, s, 0, []string{"Content-Length:", `"command":"initialize"`, `"success":true`, `"event":"initialized"`}, nil)

	// Bozuk çerçeve: sunucu hatayı Türkçe bildirip 1 koduyla çıkar.
	s = calistir(t, trIkilisi, "", "Content-Length: abc\r\n\r\n", nil, "dap")
	bekle(t, s, 1, nil, []string{"❌ Hata ayıklama sunucusu:"})
}

func TestREPL(t *testing.T) {
	girdi := strings.Join([]string{
		`yazdır("merhaba")`,
		`1 +`,
		`değişken x: Tam = "a"`,
		`1 / 0`,
		`2 + 3`,
		`exit`,
		`çıkış`,
	}, "\n") + "\n"
	s := calistir(t, trIkilisi, "", girdi, nil)
	bekle(t, s, 0, []string{"merhaba", "=> 5", "Görüşmek üzere!"}, []string{
		"'exit' TR'de bir komut değil; çıkmak için 'çıkış' yazın.",
		"❌ Sözdizimi Hataları:\n  • Satır 1, Sütun 4",
		"❌ Tip ve Güvenlik Denetimi Hataları:\n  • Satır 1: Tip uyuşmazlığı",
		"❌ Çalışma Zamanı Hatası:\n  • Satır 1: Sıfıra bölme hatası",
		"1 | 1 / 0",
	})
	for _, p := range []string{"Hata", "❌"} {
		if strings.Contains(s.cikti, p) {
			t.Errorf("hatalar standart çıktıya yazılmamalı (%q bulundu):\n%s", p, s.cikti)
		}
	}
	// Denetleyici durumunu korur; eski tip hatası sonraki satırlarda yinelenmemeli.
	if n := strings.Count(s.hata, "Tip uyuşmazlığı"); n != 1 {
		t.Errorf("tip hatası %d kez yazıldı, beklenen 1:\n%s", n, s.hata)
	}
}

// girdi() boru ile gelen satırları sırayla okur; kabuk da aynı okuyucuyu kullandığı için
// kabukta girdi() bir sonraki satırı alır, kabuk satırları yutulmaz.
func TestGirdiBoruVeKabuk(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "g.tr", "değişken a = girdi()\ndeğişken b = girdi()\nyazdır(\"a=[{a}] b=[{b}]\")\n")
	s := calistir(t, trIkilisi, klasor, "bir\niki\n", nil, "g.tr")
	bekle(t, s, 0, []string{"a=[bir] b=[iki]"}, nil)

	girdi := "değişken x = girdi()\nveri\nyazdır(\"x=\" + x)\nçıkış\n"
	s = calistir(t, trIkilisi, "", girdi, nil)
	bekle(t, s, 0, []string{"x=veri", "Görüşmek üzere!"}, nil)
}

// avxBekleniyor derlenmiş yorumlayıcının GOAMD64=v3 ile derlenip derlenmediğini okur.
func avxBekleniyor(t *testing.T) bool {
	t.Helper()
	bilgi, err := buildinfo.ReadFile(trIkilisi)
	if err != nil {
		t.Fatalf("derleme bilgisi okunamadı: %v", err)
	}
	var mimari, duzey string
	for _, s := range bilgi.Settings {
		switch s.Key {
		case "GOARCH":
			mimari = s.Value
		case "GOAMD64":
			duzey = s.Value
		}
	}
	return mimari == "amd64" && duzey == "v3"
}

func TestDerleModulleriGomer(t *testing.T) {
	kaynak := t.TempDir()
	dosyaYaz(t, kaynak, "proje/ana.tr", strings.Join([]string{
		`aktar "lib/yardimci"`,
		`aktar "lib/yardimci.tr" olarak y`,
		`eğer doğru {`,
		`    aktar "alt/derin" olarak d`,
		`    yazdır("toplam {topla(2, 3)} {y.SELAM} {d.DERIN}")`,
		`}`,
	}, "\n")+"\n")
	// Modül içindeki aktarım, modülün klasörüne göre çözülür (yorumlayıcıdaki kural).
	dosyaYaz(t, kaynak, "proje/lib/yardimci.tr", "aktar \"ic.tr\"\ndeğişken SELAM = \"merhaba\"\nfonksiyon topla(a, b) {\n    sonuç a + b + IC\n}\n")
	dosyaYaz(t, kaynak, "proje/lib/ic.tr", "\xef\xbb\xbfdeğişken IC = 100\n")
	dosyaYaz(t, kaynak, "proje/alt/derin.tr", "aktar \"../lib/ic\"\ndeğişken DERIN = IC + 1\n")

	// Önce yorumlayıcıyla: beklenen çıktı.
	beklenen := "toplam 105 merhaba 101"
	bekle(t, tr(t, filepath.Join(kaynak, "proje"), "ana.tr"), 0, []string{beklenen}, nil)

	cikis := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, kaynak, "derle", filepath.Join("proje", "ana.tr"), cikis)
	bekle(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (3)", "proje/lib/yardimci.tr", "proje/lib/ic.tr", "proje/alt/derin.tr"}, nil)
	avxNotu := strings.Contains(s.cikti, "AVX2")
	if avxNotu != avxBekleniyor(t) {
		t.Errorf("AVX2 notu: çıktıda %v, beklenen %v\n%s", avxNotu, !avxNotu, s.cikti)
	}

	// Kaynaklar silinir; uygulama başka bir klasörde çalışır. Oradaki aynı adlı dosya
	// gömülü modülün önüne geçmemeli.
	if err := os.RemoveAll(kaynak); err != nil {
		t.Fatal(err)
	}
	baska := t.TempDir()
	dosyaYaz(t, baska, "lib/yardimci.tr", "değişken SELAM = \"SAHTE\"\nfonksiyon topla(a, b) { sonuç 0 }\n")
	gecici := t.TempDir()
	ortam := []string{"TMP=" + gecici, "TEMP=" + gecici, "TMPDIR=" + gecici}
	s = calistir(t, cikis, baska, "", ortam)
	bekle(t, s, 0, []string{beklenen}, nil)
	if strings.Contains(s.cikti, "SAHTE") {
		t.Error("bulunulan klasördeki dosya gömülü modülün yerine geçti")
	}
	if girdiler, _ := os.ReadDir(gecici); len(girdiler) != 0 {
		t.Errorf("gömülü modüllerin geçici klasörü silinmedi: %v", girdiler)
	}
}

func TestDerleCalismaHatasindaTemizler(t *testing.T) {
	kaynak := t.TempDir()
	dosyaYaz(t, kaynak, "ana.tr", "aktar \"m\"\nyazdır(bol(1))\n")
	dosyaYaz(t, kaynak, "m.tr", "fonksiyon bol(a) {\n    sonuç a / 0\n}\n")
	cikis := filepath.Join(t.TempDir(), "app.exe")
	bekle(t, tr(t, kaynak, "derle", "ana.tr", cikis), 0, nil, nil)

	gecici := t.TempDir()
	ortam := []string{"TMP=" + gecici, "TEMP=" + gecici, "TMPDIR=" + gecici}
	s := calistir(t, cikis, t.TempDir(), "", ortam)
	bekle(t, s, 1, nil, []string{"❌ Çalışma Zamanı Hatası:", "Sıfıra bölme hatası"})
	if girdiler, _ := os.ReadDir(gecici); len(girdiler) != 0 {
		t.Errorf("hata ile çıkışta geçici klasör silinmedi: %v", girdiler)
	}
}

func TestDerleModulHatalari(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "eksik.tr", "yazdır(1)\naktar \"yok_boyle_modul\"\n")
	dosyaYaz(t, klasor, "bozuk.tr", "aktar \"bozuk_modul\"\n")
	dosyaYaz(t, klasor, "bozuk_modul.tr", "fonksiyon f( {\n")

	cikis := filepath.Join(klasor, "x.exe")
	s := tr(t, klasor, "derle", "eksik.tr", cikis)
	bekle(t, s, 1, nil, []string{"❌ Derleme hatası:", "satır 2", `"yok_boyle_modul" modülü bulunamadı`, "yok_boyle_modul.tr"})
	if _, err := os.Stat(cikis); err == nil {
		t.Error("hatalı derlemede çıktı dosyası oluşmamalı")
	}

	s = tr(t, klasor, "derle", "bozuk.tr", cikis)
	bekle(t, s, 1, nil, []string{"❌ Derleme Öncesi Sözdizimi Hataları (bozuk_modul.tr modülünde):"})

	dosyaYaz(t, klasor, "tip.tr", "değişken x: Tam = \"a\"\n")
	bekle(t, tr(t, klasor, "derle", "tip.tr", cikis), 1, nil, []string{"❌ Derleme Öncesi Tip Güvenliği Hataları:"})

	// Modüller de tip denetiminden geçer; modülde üst düzey 'sonuç' hatadır.
	dosyaYaz(t, klasor, "tipli.tr", "aktar \"tip_modul\"\n")
	dosyaYaz(t, klasor, "tip_modul.tr", "yazdır(1)\ndeğişken y: Tam = \"b\"\n")
	bekle(t, tr(t, klasor, "derle", "tipli.tr", cikis), 1, nil, []string{"❌ Derleme Öncesi Tip Güvenliği Hataları (tip_modul.tr modülünde):", "Satır 2: Tip uyuşmazlığı", "2 | değişken y: Tam"})
	dosyaYaz(t, klasor, "sonuclu.tr", "aktar \"sonuc_modul\"\n")
	dosyaYaz(t, klasor, "sonuc_modul.tr", "sonuç 1\n")
	bekle(t, tr(t, klasor, "derle", "sonuclu.tr", cikis), 1, nil, []string{"(sonuc_modul.tr modülünde)", "'sonuç' modülün üst düzeyinde kullanılamaz"})
}

// tr derle kaynağın ya da başka bir .tr dosyasının üzerine yazmaz (veri kaybı).
func TestDerleKaynagiEzmez(t *testing.T) {
	klasor := t.TempDir()
	kaynak := "yazdır(\"uygulama\")\n"
	dosyaYaz(t, klasor, "uyg.tr", kaynak)
	dosyaYaz(t, klasor, "baska.tr", "yazdır(2)\n")
	for _, cikis := range []string{"uyg.tr", "./uyg.tr", filepath.Join(klasor, "uyg.tr"), "baska.tr", "yeni.TR"} {
		s := tr(t, klasor, "derle", "uyg.tr", cikis)
		bekle(t, s, 1, nil, []string{"❌ Derleme hatası:"})
	}
	for ad, icerik := range map[string]string{"uyg.tr": kaynak, "baska.tr": "yazdır(2)\n"} {
		if veri, _ := os.ReadFile(filepath.Join(klasor, ad)); string(veri) != icerik {
			t.Errorf("%s değişti: %q", ad, veri)
		}
	}
	if _, err := os.Stat(filepath.Join(klasor, "yeni.TR")); err == nil {
		t.Error(".tr uzantılı çıktı oluşmamalı")
	}
}

// tr kontrol aktarılan modülleri de denetler; modül hatası modülün adı ve satırıyla yazılır.
func TestKontrolModulleri(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "ana.tr", "yazdır(\"ana\")\naktar \"lib/m\"\n")
	dosyaYaz(t, klasor, "lib/m.tr", "aktar \"ic\"\n")
	dosyaYaz(t, klasor, "lib/ic.tr", "değişken a = 1\nsabit S = 1\nS = 2\n")
	s := tr(t, klasor, "kontrol", "ana.tr")
	bekle(t, s, 1, nil, []string{"❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):", "Satır 3: Sabit hatası", "3 | S = 2"})
	if strings.Contains(s.cikti, "ana") {
		t.Errorf("kontrol programı çalıştırmamalı: %q", s.cikti)
	}
	dosyaYaz(t, klasor, "eksik.tr", "aktar \"olmayan\"\n")
	bekle(t, tr(t, klasor, "kontrol", "eksik.tr"), 1, nil, []string{"❌ Modül hatası:", `"olmayan" modülü bulunamadı`})
	dosyaYaz(t, klasor, "lib/ic.tr", "değişken a = 1\n")
	bekle(t, tr(t, klasor, "kontrol", "ana.tr"), 0, []string{"başarıyla denetlendi"}, nil)
}

// Modüldeki çalışma hatası modülün adı, satırı ve kod çerçevesiyle yazılır.
func TestModulCalismaHatasiCercevesi(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "ana.tr", "yazdır(1)\n\naktar \"m2\"\n")
	dosyaYaz(t, klasor, "m2.tr", "değişken x = 1\n\ndeğişken z = x / 0\n")
	s := tr(t, klasor, "ana.tr")
	bekle(t, s, 1, nil, []string{"• m2.tr, Satır 3: Sıfıra bölme hatası", "3 | değişken z = x / 0"})
	if strings.Contains(s.hata, "aktar \"m2\"") {
		t.Errorf("çerçeve ana programın satırını göstermemeli:\n%s", s.hata)
	}
}
