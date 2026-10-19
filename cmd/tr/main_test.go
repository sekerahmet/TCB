package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Bu paketteki testler 'main' paketinin iç işlevlerini (gömülü paket biçimi, aktar
// yönlendirmesi, işlemci notu) doğrudan çağıran beyaz kutu testleridir. Yalnızca derlenmiş
// 'tr' ikilisini çalıştıran uçtan uca testler cmd/tr/e2e'dadır.

// Gömülü paket biçiminin iki testi, paketi gerçek bir yorumlayıcı ikilisinin sonuna ekleyip
// çalıştırır. İkili yalnızca gerektiğinde, bir kez derlenir (ikiliKlasoru TestMain'de silinir).
var (
	ikiliBir     sync.Once
	ikiliKlasoru string
	trIkilisi    string
	ikiliHatasi  error
)

func TestMain(m *testing.M) {
	kod := m.Run()
	if ikiliKlasoru != "" {
		os.RemoveAll(ikiliKlasoru)
	}
	os.Exit(kod)
}

// trDerle yorumlayıcıyı geçici bir klasöre bir kez derler ve yolunu verir.
func trDerle(t *testing.T) string {
	t.Helper()
	ikiliBir.Do(func() {
		ikiliKlasoru, ikiliHatasi = os.MkdirTemp("", "tr-cmd-test-*")
		if ikiliHatasi != nil {
			return
		}
		ad := "tr"
		if runtime.GOOS == "windows" {
			ad += ".exe"
		}
		trIkilisi = filepath.Join(ikiliKlasoru, ad)
		var cikti bytes.Buffer
		derle := exec.Command("go", "build", "-o", trIkilisi, ".")
		derle.Stdout, derle.Stderr = &cikti, &cikti
		if err := derle.Run(); err != nil {
			ikiliHatasi = fmt.Errorf("%v\n%s", err, cikti.String())
		}
	})
	if ikiliHatasi != nil {
		t.Fatalf("tr derlenemedi: %v", ikiliHatasi)
	}
	return trIkilisi
}

// sonuc bir komutun çalıştırılmasının sonucudur.
type sonuc struct {
	cikti, hata string
	kod         int
}

// calistir ikiliyi verilen klasörde ve standart girişle çalıştırır (e2e'deki aynı adlı
// yardımcının kısası).
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

// sablon gömülü paketi olmayan yorumlayıcı ikilisinin baytlarıdır.
func sablon(t *testing.T) []byte {
	t.Helper()
	veri, err := os.ReadFile(trDerle(t))
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
