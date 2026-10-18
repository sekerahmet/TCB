package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// trIkilisi TestMain'in derlediği 'tr' yorumlayıcısının yoludur. Komut satırı testleri
// gerçek ikiliyi os/exec ile çalıştırır; böylece çıkış kodları ve standart çıktı/hata
// ayrımı da sınanır.
var trIkilisi string

func TestMain(m *testing.M) {
	klasor, err := os.MkdirTemp("", "tr-cmd-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "geçici klasör açılamadı:", err)
		os.Exit(1)
	}
	ad := "tr"
	if runtime.GOOS == "windows" {
		ad += ".exe"
	}
	trIkilisi = filepath.Join(klasor, ad)
	derle := exec.Command("go", "build", "-o", trIkilisi, ".")
	derle.Stdout, derle.Stderr = os.Stderr, os.Stderr
	if err := derle.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tr derlenemedi:", err)
		os.RemoveAll(klasor)
		os.Exit(1)
	}
	kod := m.Run()
	os.RemoveAll(klasor)
	os.Exit(kod)
}

// sonuc bir komutun çalıştırılmasının sonucudur.
type sonuc struct {
	cikti, hata string
	kod         int
}

// calistir ikiliyi verilen klasörde (boşsa bulunulan klasörde) ve standart girişle çalıştırır.
func calistir(t *testing.T, ikili, klasor, girdi string, ortam []string, args ...string) sonuc {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 2*time.Minute)
	defer iptal()
	cmd := exec.CommandContext(ctx, ikili, args...)
	cmd.Dir = klasor
	cmd.Stdin = strings.NewReader(girdi)
	if ortam != nil {
		cmd.Env = append(os.Environ(), ortam...)
	}
	var cikti, hata bytes.Buffer
	cmd.Stdout, cmd.Stderr = &cikti, &hata
	err := cmd.Run()
	kod := 0
	if err != nil {
		var cikisHatasi *exec.ExitError
		if !errors.As(err, &cikisHatasi) {
			t.Fatalf("%v çalıştırılamadı: %v", args, err)
		}
		kod = cikisHatasi.ExitCode()
	}
	return sonuc{cikti: cikti.String(), hata: hata.String(), kod: kod}
}

// tr yorumlayıcıyı çalıştırır.
func tr(t *testing.T, klasor string, args ...string) sonuc {
	t.Helper()
	return calistir(t, trIkilisi, klasor, "", nil, args...)
}

// dosyaYaz klasör altına (ara klasörleri açarak) bir dosya yazar.
func dosyaYaz(t *testing.T, klasor, ad, icerik string) string {
	t.Helper()
	yol := filepath.Join(klasor, filepath.FromSlash(ad))
	if err := os.MkdirAll(filepath.Dir(yol), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yol, []byte(icerik), 0o644); err != nil {
		t.Fatal(err)
	}
	return yol
}

// bekle sonucun çıkış kodunu ve çıktılarda geçmesi gereken parçaları denetler.
func bekle(t *testing.T, s sonuc, kod int, ciktida, hatada []string) {
	t.Helper()
	if s.kod != kod {
		t.Errorf("çıkış kodu %d, beklenen %d\nçıktı:\n%s\nhata:\n%s", s.kod, kod, s.cikti, s.hata)
	}
	for _, p := range ciktida {
		if !strings.Contains(s.cikti, p) {
			t.Errorf("standart çıktıda %q yok:\n%s", p, s.cikti)
		}
	}
	for _, p := range hatada {
		if !strings.Contains(s.hata, p) {
			t.Errorf("standart hatada %q yok:\n%s", p, s.hata)
		}
	}
}

func TestYardimVeSurum(t *testing.T) {
	for _, a := range []string{"yardım", "yardim", "--help", "-h"} {
		s := tr(t, "", a)
		bekle(t, s, 0, []string{"Kullanım:", "tr derle <dosya.tr>", "tr test"}, nil)
	}
	for _, a := range []string{"sürüm", "surum", "--version", "-v"} {
		s := tr(t, "", a)
		bekle(t, s, 0, []string{"v" + VERSION}, nil)
	}
}

func TestKaldirilmisKomutlar(t *testing.T) {
	for _, a := range []string{"bytecode", "baytkod"} {
		s := tr(t, "", a)
		bekle(t, s, 1, nil, []string{"'tr bytecode' komutu kaldırıldı"})
	}
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "yazdır(1)\n")
	for _, a := range []string{"--web", "-w"} {
		s := tr(t, klasor, "debug", "a.tr", a)
		bekle(t, s, 1, nil, []string{"Tarayıcıdaki hata ayıklayıcı kaldırıldı"})
		if strings.Contains(s.cikti, "1") {
			t.Errorf("program çalışmamalıydı: %q", s.cikti)
		}
	}
}

func TestEksikArguman(t *testing.T) {
	for _, k := range []string{"kontrol", "derle", "debug", "hata_ayıkla", "çalıştır", "biçimle"} {
		s := tr(t, "", k)
		bekle(t, s, 1, nil, []string{"Hata: Lütfen"})
	}
}

func TestBilinmeyenKomut(t *testing.T) {
	klasor := t.TempDir()
	s := tr(t, klasor, "olmayan_komut")
	bekle(t, s, 1, nil, []string{"Dosya okuma hatası (olmayan_komut)"})
}

func TestDosyaCalistirma(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "iyi.tr", "değişken ad = \"dünya\"\nyazdır(\"merhaba {ad}\")\n")
	dosyaYaz(t, klasor, "sozdizimi.tr", "yazdır(\"a\"\n")
	dosyaYaz(t, klasor, "tip.tr", "değişken x: Tam = \"metin\"\nyazdır(\"çalışmamalı\")\n")
	dosyaYaz(t, klasor, "calisma.tr", "yazdır(\"önce\")\ndeğişken y = 1 / 0\nyazdır(\"sonra\")\n")

	for _, args := range [][]string{{"iyi.tr"}, {"çalıştır", "iyi.tr"}, {"calistir", "iyi.tr"}} {
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
	bekle(t, tr(t, klasor, "kontrol", "yok.tr"), 1, nil, []string{"Dosya okuma hatası (yok.tr)"})
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

	bekle(t, tr(t, klasor, "bicimle", "a.tr"), 0, []string{"biçimlendi: a.tr"}, nil)
	bekle(t, tr(t, klasor, "biçimle", "--kontrol", "."), 0, []string{"hepsi standart biçimde"}, nil)
	bekle(t, tr(t, klasor, "biçimle", "olmayan.tr"), 1, nil, []string{"olmayan.tr bulunamadı"})
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
		`çıkış`,
	}, "\n") + "\n"
	s := calistir(t, trIkilisi, "", girdi, nil)
	bekle(t, s, 0, []string{"merhaba", "=> 5", "Görüşmek üzere!"}, []string{
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
}

// sablon gömülü paketi olmayan yorumlayıcı ikilisinin baytlarıdır.
func sablon(t *testing.T) []byte {
	t.Helper()
	veri, err := os.ReadFile(trIkilisi)
	if err != nil {
		t.Fatal(err)
	}
	return veri
}

func TestEskiPaketBiciminiOkur(t *testing.T) {
	kod := []byte("yazdır(\"eski biçim çalışıyor\")\n")
	veri := append(sablon(t), kod...)
	altbilgi := make([]byte, 16)
	binary.LittleEndian.PutUint64(altbilgi[0:8], uint64(len(kod)))
	copy(altbilgi[8:], eskiPaketImzasi)
	veri = append(veri, altbilgi...)

	yol := filepath.Join(t.TempDir(), "eski.exe")
	if err := os.WriteFile(yol, veri, 0o755); err != nil {
		t.Fatal(err)
	}
	bekle(t, calistir(t, yol, t.TempDir(), "", nil), 0, []string{"eski biçim çalışıyor"}, nil)

	// Derlemede şablonun sonundaki eski paket de doğru boyda atılır.
	pk, sablonBoyu, bulundu, err := paketiAyikla(veri)
	if err != nil || !bulundu || sablonBoyu != len(sablon(t)) || pk.Ana.Kod != string(kod) {
		t.Fatalf("eski paket ayıklanamadı: bulundu=%v boy=%d hata=%v", bulundu, sablonBoyu, err)
	}
}

func TestDesteklenmeyenPaketSurumu(t *testing.T) {
	govde := []byte(`{"ana":{"kod":"yazdır(1)"}}`)
	veri := append(sablon(t), govde...)
	altbilgi := make([]byte, altbilgiBoyu)
	binary.LittleEndian.PutUint64(altbilgi[0:8], uint64(len(govde)))
	binary.LittleEndian.PutUint32(altbilgi[8:12], 99)
	copy(altbilgi[12:], paketImzasi)
	veri = append(veri, altbilgi...)
	yol := filepath.Join(t.TempDir(), "gelecek.exe")
	if err := os.WriteFile(yol, veri, 0o755); err != nil {
		t.Fatal(err)
	}
	bekle(t, calistir(t, yol, t.TempDir(), "", nil), 1, nil, []string{"❌ Gömülü uygulama okunamadı:", "sürümü 99 desteklenmiyor"})
}

// Birim testleri (süreç içinde).

func TestPaketGidisDonus(t *testing.T) {
	pk := &paket{
		Ana:      paketDosyasi{Ad: "ana.tr", Kod: "aktar \"m\"\n", Aktarimlar: []int{0}},
		Moduller: []paketDosyasi{{Ad: "m.tr", Kod: "değişken a = 1\n"}},
	}
	kodlu, err := paketiKodla(pk)
	if err != nil {
		t.Fatal(err)
	}
	ikili := append([]byte("YORUMLAYICI"), kodlu...)
	geri, sablonBoyu, bulundu, err := paketiAyikla(ikili)
	if err != nil || !bulundu || sablonBoyu != len("YORUMLAYICI") {
		t.Fatalf("bulundu=%v boy=%d hata=%v", bulundu, sablonBoyu, err)
	}
	if geri.Ana.Kod != pk.Ana.Kod || len(geri.Moduller) != 1 || geri.Moduller[0].Kod != pk.Moduller[0].Kod || geri.Ana.Aktarimlar[0] != 0 {
		t.Fatalf("paket bozuldu: %+v", geri)
	}
	if _, _, bulundu, _ := paketiAyikla([]byte("sıradan bir ikili dosya")); bulundu {
		t.Error("paketsiz ikilide paket bulunmamalı")
	}
	bozuk := append([]byte{}, kodlu...)
	binary.LittleEndian.PutUint64(bozuk[len(bozuk)-20:], 1<<40)
	if _, _, bulundu, err := paketiAyikla(bozuk); !bulundu || err == nil {
		t.Error("bozuk uzunluk alanı hata vermeli")
	}
}

func TestAktarYerleriVeYonlendirme(t *testing.T) {
	kod := "// aktar \"yorumdaki\" sayılmaz\naktar \"a\"\nyazdır(\"aktar b\")\neğer doğru {\n    aktar \"alt/ç.tr\" olarak c\n}\n"
	yerler := aktarYerleri(kod)
	if len(yerler) != 2 || yerler[0].yol != "a" || yerler[0].satir != 2 || yerler[1].yol != "alt/ç.tr" || yerler[1].satir != 5 {
		t.Fatalf("aktar yerleri yanlış: %+v", yerler)
	}
	d := paketDosyasi{Ad: "x.tr", Kod: kod, Aktarimlar: []int{1, 0}}
	yeni, err := aktarlariYonlendir(d, []string{"/g/0/ç.tr", "/g/1/a.tr"})
	if err != nil {
		t.Fatal(err)
	}
	beklenen := strings.Replace(strings.Replace(kod, `aktar "a"`, `aktar "/g/1/a.tr"`, 1), `aktar "alt/ç.tr"`, `aktar "/g/0/ç.tr"`, 1)
	if yeni != beklenen {
		t.Fatalf("yönlendirilmiş kod:\n%s\nbeklenen:\n%s", yeni, beklenen)
	}
	if strings.Count(yeni, "\n") != strings.Count(kod, "\n") {
		t.Error("satır sayısı değişmemeli")
	}
	if _, err := aktarlariYonlendir(paketDosyasi{Kod: kod, Aktarimlar: []int{0}}, []string{"x"}); err == nil {
		t.Error("aktarım sayısı uyuşmazlığı hata vermeli")
	}
}

func TestIslemciNotu(t *testing.T) {
	if n := islemciNotuMetni("amd64", "v3"); !strings.Contains(n, "AVX2") || strings.Contains(n, "\n") {
		t.Errorf("v3 notu tek satır ve AVX2 içermeli: %q", n)
	}
	if n := islemciNotuMetni("amd64", "v4"); !strings.Contains(n, "AVX-512") {
		t.Errorf("v4 notu AVX-512 içermeli: %q", n)
	}
	for _, d := range [][2]string{{"amd64", "v1"}, {"amd64", "v2"}, {"amd64", ""}, {"arm64", "v3"}} {
		if n := islemciNotuMetni(d[0], d[1]); n != "" {
			t.Errorf("%v için not beklenmiyordu: %q", d, n)
		}
	}
}
