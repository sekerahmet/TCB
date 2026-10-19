package logging_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Terminal değilken: ara satırlar en çok 10 saniyede bir, sonda özet satırı. Hız ve kalan
// süre ölçülür; günlüğe her güncelleme ve bitiş yazılır.
func TestIlerlemeDuzSatirlarVeGunluk(t *testing.T) {
	s := saatKur(t)
	k := t.TempDir()
	yol := filepath.Join(k, "i.jsonl")
	m, w := yeniVM(t, k)
	if _, err := calistir(t, m, `değişken g = günlük("`+yolMetni(yol)+`")
değişken ç = ilerleme(100, günlük: g)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 100; i++ {
		s.ilerlet(200 * time.Millisecond) // saniyede 5 adım
		if _, err := calistir(t, m, `ç.güncelle({kayıp: 4.0 / `+strconv.Itoa(i)+`, oran: 0.0003})`); err != nil {
			t.Fatal(err)
		}
	}
	cikti := w.String()
	satir := strings.Split(strings.TrimSuffix(cikti, "\n"), "\n")
	// 20 saniyede 100 adım: 10. ve 20. saniyede ara satır (20.'de çubuk bitti: yalnızca özet)
	if len(satir) != 2 {
		t.Fatalf("1 ara satır ve özet bekleniyordu:\n%s", cikti)
	}
	if want := "adım 50/100 ▕████████░░░░░░░░▏ %50 · kayıp 0,08 · oran 3e-4 · 5 adım/sn · kalan 10 sn"; satir[0] != want {
		t.Errorf("ara satır:\n%q\nbeklenen\n%q", satir[0], want)
	}
	if want := "adım 100/100 · kayıp 0,04 · oran 3e-4 · 20 sn sürdü"; satir[1] != want {
		t.Errorf("özet satırı:\n%q\nbeklenen\n%q", satir[1], want)
	}
	kayitlar := satirlar(t, yol)
	if len(kayitlar) != 102 || kayitlar[0]["toplam"] != 100.0 {
		t.Fatalf("başlangıç + 100 ölçü + bitiş bekleniyordu: %d kayıt, %v", len(kayitlar), kayitlar[0])
	}
	if o := kayitlar[50]; o["adım"] != 50.0 || o["kayıp"] != 0.08 || o["hız"] != 5.0 {
		t.Errorf("ölçü kaydı: %v", o)
	}
	if b := kayitlar[101]; b["tür"] != "bitiş" || b["adım"] != 100.0 || b["süre_sn"] != 20.0 {
		t.Errorf("bitiş kaydı: %v", b)
	}
	// Bitmiş çubuk güncellenemez; durdur yanlış döner.
	if _, err := calistir(t, m, `ç.güncelle()`); err == nil || !strings.Contains(err.Error(), "çubuk bitti (adım 100/100)") {
		t.Errorf("bitmiş çubukta hata bekleniyordu: %v", err)
	}
	if v, _ := calistir(t, m, `ç.durdur()`); v != interp.FALSE {
		t.Errorf("ikinci durdur yanlış dönmeli: %v", v)
	}
}

// adım: kaldığı yerden sürdürür; toplamdan büyük olamaz; ayrılmış ölçü adları reddedilir.
func TestIlerlemeAdimVeHatalar(t *testing.T) {
	saatKur(t)
	m, _ := yeniVM(t, t.TempDir())
	v, err := calistir(t, m, `değişken ç = ilerleme(10)
ç.güncelle(adım: 7)
ç.güncelle()
ç`)
	if err != nil || v.Inspect() != "İlerleme(adım 8/10)" {
		t.Fatalf("adım: ile sürdürme: %v %v", v, err)
	}
	for _, d := range []struct{ kod, want string }{
		{`ç.güncelle(adım: 11)`, "11 toplam adım sayısından (10) büyük olamaz"},
		{`ç.güncelle(adım: -1)`, "0 ya da pozitif"},
		{`ç.güncelle({adım: 3})`, "'adım' adı günlük biçimine ayrılmış"},
		{`ç.güncelle({hız: 3})`, "'hız' adı günlük biçimine ayrılmış"},
		{`ç.güncelle(5)`, "1. argüman olarak Sözlük bekler"},
		{`ilerleme(0)`, "pozitif olmalı"},
		{`ilerleme(10, günlük: 5)`, "bir Günlük olmalı"},
		{`ç.durdur(1)`, "0 argüman bekler"},
	} {
		if _, err := calistir(t, m, d.kod); err == nil || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.kod, d.want, err)
		}
	}
	// Toplamsız sayaç: kalan süre yerine geçen süre.
	if v, err := calistir(t, m, `değişken s = ilerleme()
s.güncelle(); s.güncelle(); s`); err != nil || v.Inspect() != "İlerleme(adım 2)" {
		t.Errorf("sayaç: %v %v", v, err)
	}
}

// Terminalde çubuk canlı satırdır: satır başından yeniden çizilir (en çok saniyede 10 kez),
// yazdır çıktısı çubuğu silip yazar ve çubuk yeniden çizilir; bitince satır özete dönüşür.
func TestIlerlemeTerminal(t *testing.T) {
	t.Cleanup(logging.TerminalTaklidi(100))
	s := saatKur(t)
	m, w := yeniVM(t, t.TempDir())
	if _, err := calistir(t, m, `değişken ç = ilerleme(4)`); err != nil {
		t.Fatal(err)
	}
	adim := func(kod string) {
		t.Helper()
		if _, err := calistir(t, m, kod); err != nil {
			t.Fatal(err)
		}
	}
	adim(`ç.güncelle({kayıp: 2.0})`) // çizilir
	adim(`ç.güncelle({kayıp: 1.5})`) // 100 ms dolmadı: çizilmez
	adim(`yazdır("ara")`)            // sil, yaz, tazelenmiş satırı yeniden çiz
	s.ilerlet(time.Second)
	adim(`ç.güncelle({kayıp: 1.0})`) // çizilir; hız 1 saniyede 3 adım
	adim(`ç.güncelle({kayıp: 0.5})`) // bitti: özet
	uzunluk := func(s string) int { return len([]rune(s)) }
	c1 := "adım 1/4 ▕████░░░░░░░░░░░░▏ %25 · kayıp 2"
	c2 := "adım 2/4 ▕████████░░░░░░░░▏ %50 · kayıp 1,5"
	c3 := "adım 3/4 ▕████████████░░░░▏ %75 · kayıp 1 · 3 adım/sn · kalan 1 sn"
	son := "adım 4/4 · kayıp 0,5 · 1 sn sürdü"
	want := "\r" + c1 + "\r" + strings.Repeat(" ", uzunluk(c1)) + "\r" + "ara\n" + "\r" + c2 +
		"\r" + c3 + "\r" + son + strings.Repeat(" ", uzunluk(c3)-uzunluk(son)) + "\n"
	if got := w.String(); got != want {
		t.Errorf("terminal çıktısı:\n%q\nbeklenen\n%q", got, want)
	}
	// Çubuk bitince canlı satır kalkar: yazdır doğrudan yazar.
	adim(`yazdır("son")`)
	if !strings.HasSuffix(w.String(), "\nson\n") {
		t.Errorf("çubuk bittikten sonra yazdır doğrudan yazmalı: %q", w.String())
	}
}

// Adım sınırı kancası her güncellemede tamamlanan adımla çağrılır.
func TestAdimKancasi(t *testing.T) {
	m, _ := yeniVM(t, t.TempDir())
	var adimlar []int64
	logging.AdimKancasiAyarla(func(a logging.AdimSiniri) {
		if a.VM == nil || a.Toplam != 3 {
			t.Errorf("kanca bilgisi: %+v", a)
		}
		adimlar = append(adimlar, a.Adim)
	})
	t.Cleanup(func() { logging.AdimKancasiAyarla(nil) })
	if _, err := calistir(t, m, `değişken ç = ilerleme(3)
döngü i içinde 0..3 { ç.güncelle() }`); err != nil {
		t.Fatal(err)
	}
	if len(adimlar) != 3 || adimlar[0] != 1 || adimlar[2] != 3 {
		t.Errorf("kanca adımları: %v", adimlar)
	}
}

// Arka plan işinde (TR_İŞ) çubuk iş klasörüne ilerleme.json yazar.
func TestIsDurumu(t *testing.T) {
	is := t.TempDir()
	t.Setenv("TR_İŞ", is)
	t.Cleanup(logging.TerminalTaklidi(60)) // iş kipinde terminal kullanılmaz
	m, w := yeniVM(t, t.TempDir())
	if _, err := calistir(t, m, `değişken ç = ilerleme(2)
ç.güncelle({kayıp: 0.25})`); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(mustOku(t, filepath.Join(is, "ilerleme.json")), &d); err != nil {
		t.Fatal(err)
	}
	if d["adım"] != 1.0 || d["toplam"] != 2.0 || d["bitti"] != false || d["ölçüler"].(map[string]any)["kayıp"] != 0.25 {
		t.Errorf("ilerleme.json: %v", d)
	}
	if _, err := calistir(t, m, `ç.güncelle()`); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustOku(t, filepath.Join(is, "ilerleme.json")), &d); err != nil || d["bitti"] != true || d["adım"] != 2.0 {
		t.Errorf("son ilerleme.json: %v %v", d, err)
	}
	if strings.Contains(w.String(), "\r") {
		t.Errorf("iş kipinde canlı satır olmamalı: %q", w.String())
	}
	if kalan, _ := filepath.Glob(filepath.Join(is, ".ilerleme.json.*")); len(kalan) > 0 {
		t.Errorf("geçici dosya kalmamalı: %v", kalan)
	}
	_ = os.Remove(filepath.Join(is, "ilerleme.json"))
}

// Program bitince bitmemiş çubuk özet satırını yazar.
func TestTumunuKapatCubuguBitirir(t *testing.T) {
	saatKur(t)
	m, w := yeniVM(t, t.TempDir())
	if _, err := calistir(t, m, `değişken ç = ilerleme(10)
ç.güncelle({kayıp: 3.25})`); err != nil {
		t.Fatal(err)
	}
	logging.TumunuKapat()
	if got := w.String(); got != "adım 1/10 · kayıp 3,25 · 0 sn sürdü\n" {
		t.Errorf("özet satırı: %q", got)
	}
}
