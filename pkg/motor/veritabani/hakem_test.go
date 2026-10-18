package veritabani

// Hakem denetiminin (veri_web.md 1, 9, 10, 13, 14, 18, 19) bulduğu kusurların regresyon testleri.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// geciciVT geçici klasörde bir veritabanı yolu verir; test bitince açık nesne ve kilidi
// bırakılır (Windows açık dosyayı sildirmez).
func geciciVT(t *testing.T, ad string) string {
	yol := filepath.Join(t.TempDir(), ad)
	t.Cleanup(func() { unutVeritabani(yol) })
	return yol
}

// trYol test dosyasının yolunu TR metnine gömülecek biçimde verir.
func trYol(p string) string { return filepath.ToSlash(p) }

// diskSatirlari veritabanı dosyasını bu süreçteki kayıttan bağımsız okur (tablo → satır sayısı).
func diskSatirlari(t *testing.T, yol string) map[string]int {
	t.Helper()
	b, err := os.ReadFile(yol)
	if err != nil {
		t.Fatalf("dosya okunamadı: %v", err)
	}
	var tablolar map[string]struct {
		Rows []map[string]interface{} `json:"rows"`
	}
	if err := json.Unmarshal(b, &tablolar); err != nil {
		t.Fatalf("dosya JSON değil: %v\n%s", err, b)
	}
	sonuc := map[string]int{}
	for k, v := range tablolar {
		sonuc[k] = len(v.Rows)
	}
	return sonuc
}

// 1 (KRİTİK): yazma hatası yutuluyordu; ekle "1" döndürüyor ama dosya hiç oluşmuyordu.
// Artık hata kullanıcıya döner ve bellekteki durum diske yazılmış durumla aynı kalır.
func TestYazmaHatasiKullaniciyaDoner(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç("olmayan_klasor_hakem/alt/x.trdb")
		değişken mesajlar = []
		dene { vt.ekle("t", {"ad": "önemli"}) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.tablo_oluştur("u") } yakala h { mesajlar.ekle("{h}") }
		sonuç [mesajlar, vt.tablolar()]`)
	if err != nil {
		t.Fatal(err)
	}
	if s := val.Inspect(); strings.Count(s, "kaydedilemedi") != 2 || !strings.HasSuffix(s, ", []]") {
		t.Fatalf("iki kaydetme hatası ve boş tablo listesi bekleniyordu: %s", s)
	}

	// Önce başarılı yazma; sonra dosyanın yerine bir klasör konunca güncelle/sil/SİL hata verir
	// ve bellekteki satırlar değişmez.
	yol := geciciVT(t, "v.trdb")
	val, err = runTR(t, fmt.Sprintf(`
		değişken vt = veritabanı_aç(%q)
		vt.ekle("t", [{"ad": "a"}, {"ad": "b"}])
		sonuç vt.say("t")`, trYol(yol)))
	if err != nil || val.Inspect() != "2" {
		t.Fatalf("%v %v", val, err)
	}
	if err := os.Remove(yol); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(yol, 0755); err != nil {
		t.Fatal(err)
	}
	val, err = runTR(t, fmt.Sprintf(`
		değişken vt = veritabanı_aç(%q)
		değişken mesajlar = []
		dene { vt.güncelle("t", {"ad": "z"}) } yakala h { mesajlar.ekle("güncelle") }
		dene { vt.sil("t", "ad == 'a'") } yakala h { mesajlar.ekle("sil") }
		dene { vt.sorgula("SİL TABLO t") } yakala h { mesajlar.ekle("SİL") }
		dene { vt.ekle("t", {"ad": "c"}) } yakala h { mesajlar.ekle("ekle") }
		dene { vt.kaydet() } yakala h { mesajlar.ekle("kaydet") }
		sonuç [mesajlar, vt.sorgula("SEÇ ad TABLO t")]`, trYol(yol)))
	if err != nil {
		t.Fatal(err)
	}
	if want := `[["güncelle", "sil", "SİL", "ekle", "kaydet"], [{"ad": "a"}, {"ad": "b"}]]`; val.Inspect() != want {
		t.Fatalf("\n%s\nbeklenen\n%s", val.Inspect(), want)
	}
}

// 1: NaN içeren tek satır sonraki her kaydetmeyi bozuyordu (bellekte 3, diskte 1 satır).
// Tanımsız, sonsuz ve döngüsel değerler artık eklenirken reddedilir.
func TestYazilamayanDegerlerReddedilir(t *testing.T) {
	yol := geciciVT(t, "nan.trdb")
	val, err := runTR(t, fmt.Sprintf(`
		değişken vt = veritabanı_aç(%q)
		vt.ekle("t", {"ad": "ilk"})
		değişken l = [1]
		l.ekle(l)
		değişken mesajlar = []
		dene { vt.ekle("t", {"x": karekök(-1.0)}) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.ekle("t", {"x": [1e308 * 10.0]}) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.ekle("t", {"x": l}) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.güncelle("t", {"x": karekök(-1.0)}) } yakala h { mesajlar.ekle("{h}") }
		vt.ekle("t", {"ad": "sonraki"})
		sonuç [mesajlar, vt.say("t")]`, trYol(yol)))
	if err != nil {
		t.Fatal(err)
	}
	s := val.Inspect()
	if strings.Count(s, "veritabanına yazılamaz") != 3 || strings.Count(s, "kendini içeriyor") != 1 || !strings.HasSuffix(s, ", 2]") {
		t.Fatalf("üç sayı ve bir döngü hatası, 2 satır bekleniyordu: %s", s)
	}
	if n := diskSatirlari(t, yol)["t"]; n != 2 {
		t.Fatalf("diskte 2 satır bekleniyordu, %d", n)
	}
}

// 10: kayıt atomik değildi (dosya doğrudan kısaltılıp yazılıyordu) ve aynı dosyayı açan iki
// nesne birbirinin kaydını siliyordu. Artık süreçte dosya başına tek nesne vardır ve kayıt
// geçici dosya + yeniden adlandırma ile yapılır.
func TestAyniDosyaTekNesneVeAtomikKayit(t *testing.T) {
	dir := t.TempDir()
	yol := filepath.Join(dir, "d3.trdb")
	t.Cleanup(func() { unutVeritabani(yol) })
	_, err := runTR(t, fmt.Sprintf(`
		değişken v1 = veritabanı_aç(%q)
		değişken v2 = veritabanı_aç(%q)
		v1.ekle("t", {"ad": "v1_kaydı"})
		v2.ekle("t", {"ad": "v2_kaydı"})`, trYol(yol), trYol(filepath.Join(dir, ".", "d3.trdb"))))
	if err != nil {
		t.Fatal(err)
	}
	if n := diskSatirlari(t, yol)["t"]; n != 2 {
		t.Fatalf("iki kayıt da diskte olmalı, %d", n)
	}
	girdiler, _ := os.ReadDir(dir)
	for _, g := range girdiler {
		if g.Name() != "d3.trdb" && g.Name() != "d3.trdb.kilit" {
			t.Errorf("geçici dosya kalmamalı: %s", g.Name())
		}
	}
}

// 10: süreçler arası kilit. Yardımcı süreç veritabanını açık tutarken bu süreç açamaz.
func TestSureclerArasiKilit(t *testing.T) {
	if os.Getenv("TR_VT_KILIT_YARDIMCI") != "" {
		return
	}
	yol := filepath.Join(t.TempDir(), "ortak.trdb")
	cmd := exec.Command(os.Args[0], "-test.run=^TestKilitYardimciSurec$", "-test.v")
	cmd.Env = append(os.Environ(), "TR_VT_KILIT_YARDIMCI="+yol)
	giris, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cikis, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		giris.Close()
		_ = cmd.Wait()
	}()
	hazir := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(cikis)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "KİLİT_HAZIR") {
				hazir <- true
			}
		}
		hazir <- false
	}()
	select {
	case ok := <-hazir:
		if !ok {
			t.Fatal("yardımcı süreç kilidi alamadı")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("yardımcı süreç yanıt vermedi")
	}
	_, err = runTR(t, fmt.Sprintf(`veritabanı_aç(%q)`, trYol(yol)))
	if !kilitDestekli {
		t.Skip("bu platformda dosya kilidi yok")
	}
	if err == nil || !strings.Contains(err.Error(), "başka bir program") {
		t.Fatalf("kilit hatası bekleniyordu: %v", err)
	}
}

// TestKilitYardimciSurec TestSureclerArasiKilit'in açtığı yardımcı süreçte çalışır.
func TestKilitYardimciSurec(t *testing.T) {
	yol := os.Getenv("TR_VT_KILIT_YARDIMCI")
	if yol == "" {
		t.Skip("yalnızca yardımcı süreç olarak çalışır")
	}
	if _, err := runTR(t, fmt.Sprintf(`değişken vt = veritabanı_aç(%q)
vt.ekle("t", {"a": 1})`, trYol(yol))); err != nil {
		t.Fatal(err)
	}
	fmt.Println("KİLİT_HAZIR")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n') // ana süreç girişi kapatana dek bekle
}

// 13: büyük Tam sayılar yeniden açınca bozuluyor (…993 → …992), 2.0 Tam'a dönüyordu.
func TestSayiTuruVeDuyarlikKorunur(t *testing.T) {
	yol := geciciVT(t, "sayi.trdb")
	kod := `
		değişken a = vt.sorgula("SEÇ * TABLO s")[0]
		sonuç [a["buyuk"], a["f"], tip(a["f"]), a["k"], tip(a["k"]), a["e"], vt.sorgula("SEÇ k TABLO s NEREDE buyuk == ?", [9007199254740993])]`
	val, err := runTR(t, fmt.Sprintf(`değişken vt = veritabanı_aç(%q)
vt.ekle("s", {"buyuk": 9007199254740993, "f": 2.0, "k": 5, "e": 1e300})`+kod, trYol(yol)))
	want := `[9007199254740993, 2.0, "Ondalık", 5, "Tam", 1e+300, [{"k": 5}]]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("aynı oturum: %v %v", val, err)
	}
	unutVeritabani(yol) // diskten yeniden okunsun
	val, err = runTR(t, fmt.Sprintf(`değişken vt = veritabanı_aç(%q)`+kod, trYol(yol)))
	if err != nil || val.Inspect() != want {
		t.Fatalf("yeniden açınca: %v %v", val, err)
	}
}

// 14: ekle({ad:a}); ekle({id:3}); ekle({ad:c}) iki kayda id 3 veriyordu.
func TestYinelenenIdUretilmez(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("t", {"ad": "a"})
		vt.ekle("t", {"id": 3, "ad": "b"})
		vt.ekle("t", {"ad": "c"})
		değişken mesajlar = []
		dene { vt.ekle("t", {"id": 3, "ad": "d"}) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.ekle("t", [{"id": 9}, {"id": 9}]) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.güncelle("t", {"id": 1}, "ad == 'c'") } yakala h { mesajlar.ekle("{h}") }
		dene { vt.ekle("t", [{"ad": "e"}, 5]) } yakala h { mesajlar.ekle("{h}") }
		sonuç [[s["id"] döngü s içinde vt.hepsini_al("t")], mesajlar.uzunluk, mesajlar]`)
	if err != nil {
		t.Fatal(err)
	}
	s := val.Inspect()
	if !strings.HasPrefix(s, "[[1, 3, 4], 4, ") || strings.Count(s, "zaten var") != 3 || !strings.Contains(s, "Sözlük") {
		t.Fatalf("beklenmeyen: %s", s)
	}
}

// 9: koşul ayrıştırıcısında derinlik sınırı yoktu; 3 M '(' yakalanamaz yığın taşması veriyordu.
func TestKosulDerinlikSiniri(t *testing.T) {
	for _, kosul := range []string{
		strings.Repeat("(", 1_000_000) + "a > 0",
		strings.Repeat("değil ", 100_000) + "a > 0",
		"a > " + strings.Repeat("- ", 100_000) + "1",
	} {
		_, _, err := filterRows(testRows(), kosul, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "çok derin") {
			t.Fatalf("derinlik hatası bekleniyordu: %.80v", err)
		}
	}
	// Sınırın altındaki iç içe koşul çalışır
	if m, _, err := filterRows(testRows(), strings.Repeat("(", 150)+"id == 1"+strings.Repeat(")", 150), knownCols(), nil); err != nil || len(m) != 1 {
		t.Fatalf("150 seviye çalışmalı: %v", err)
	}
}

// 18: limit: -1 "İç hata (çökme önlendi): slice bounds" veriyordu ve dene yakalamıyordu;
// geçersiz LİMİT sessizce yok sayılıyordu.
func TestLimitDogrulanir(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("t", [{"a": 1}, {"a": 2}])
		değişken mesajlar = []
		dene { tablo_sorgula([{"a": 1}], "a > 0", limit: -1) } yakala h { mesajlar.ekle("{h}") }
		dene { tablo_sorgula([{"a": 1}], limit: 1.5) } yakala h { mesajlar.ekle("{h}") }
		dene { vt.sorgula("SEÇ * TABLO t LİMİT -1") } yakala h { mesajlar.ekle("{h}") }
		dene { vt.sorgula("SEÇ * TABLO t LİMİT iki") } yakala h { mesajlar.ekle("{h}") }
		sonuç [mesajlar, tablo_sorgula([{"a": 1}, {"a": 2}], limit: 0), vt.sorgula("SEÇ a TABLO t LİMİT 1")]`)
	if err != nil {
		t.Fatal(err)
	}
	s := val.Inspect()
	if strings.Count(s, "limit") < 2 || strings.Count(s, "LİMİT") < 2 || strings.Contains(s, "İç hata") || !strings.HasSuffix(s, `, [], [{"a": 1}]]`) {
		t.Fatalf("beklenmeyen: %s", s)
	}
}

// 19: SQL sözcük tarayıcısı ile koşul ayrıştırıcısı ters bölü kaçışını farklı okuyordu
// ('a\\' LİMİT 1 → "beklenmeyen LİMİT"); olmayan tablo hata yerine [] veriyordu; tablo adı
// Türkçe kurala göre küçültülmüyordu (IŞIK eklenip ışık aranınca boş).
func TestSQLTutarliligiVeTurkceTabloAdi(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("t", [{"ad": "a\\"}, {"ad": "b"}, {"ad": "c'd"}])
		vt.ekle("IŞIK", {"ad": "b"})
		vt.ekle("İşler", {"ad": "a"})
		değişken mesajlar = []
		dene { vt.sorgula("SEÇ * TABLO tt") } yakala h { mesajlar.ekle("{h}") }
		dene { vt.sorgula("SİL TABLO ışk") } yakala h { mesajlar.ekle("{h}") }
		dene { vt.hepsini_al("yok_tablo") } yakala h { mesajlar.ekle("{h}") }
		dene { vt.say("yok_tablo") } yakala h { mesajlar.ekle("{h}") }
		sonuç [
			vt.sorgula("SEÇ ad TABLO t NEREDE ad == 'a\\\\' LİMİT 1"),
			vt.sorgula("SEÇ ad TABLO t NEREDE ad == 'c''d' LİMİT 5"),
			vt.hepsini_al("ışık").uzunluk, vt.say("IŞIK"), vt.sorgula("SEÇ ad TABLO işler"), vt.say("İŞLER"),
			mesajlar]`)
	if err != nil {
		t.Fatal(err)
	}
	s := val.Inspect()
	if !strings.HasPrefix(s, `[[{"ad": "a\\"}], [{"ad": "c'd"}], 1, 1, [{"ad": "a"}], 1, [`) {
		t.Fatalf("beklenmeyen: %s", s)
	}
	if strings.Count(s, "adında tablo yok") != 4 || !strings.Contains(s, "'t'") || !strings.Contains(s, "'IŞIK'") {
		t.Fatalf("olmayan tablo hataları öneriyle bekleniyordu: %s", s)
	}
}
