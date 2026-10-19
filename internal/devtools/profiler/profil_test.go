package profiler

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// tamponYazici programın çıktısını toplar (birden çok goroutine yazabilir).
type tamponYazici struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *tamponYazici) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// kisaEgitim 200 adımlık küçük bir eğitimdir: her adımda bilinen sayıda tensör işlemi.
const kisaEgitim = `değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)
değişken op = AdamW([w], öğrenme_oranı: 0.01)
değişken ç = ilerleme(200)
döngü i içinde 0..200 {
    op.sıfırla()
    değişken kayıp = toplam(relu(x @ w))
    geri_yayılım(kayıp)
    op.adım()
    ç.güncelle({kayıp: kayıp})
}
yazdır("bitti")
`

// calistir kodu bir VM'de profille çalıştırır ve son raporu verir.
func calistir(t *testing.T, kod string, s Secenekler) (*Goruntu, *interp.VM) {
	t.Helper()
	return calistirSonra(t, kod, s, nil)
}

// calistirSonra calistir gibidir; sonra nil değilse program bittikten sonra, rapor
// alınmadan önce çağrılır.
func calistirSonra(t *testing.T, kod string, s Secenekler, sonra func(*Profil)) (*Goruntu, *interp.VM) {
	t.Helper()
	t.Cleanup(logging.CloseAll)
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	w := &tamponYazici{}
	m.Out, m.Err = w, w
	m.SourceFile = filepath.Join(t.TempDir(), "egit.tr")
	s.Program, s.Kaynak, s.Agac = "egit.tr", kod, prog
	pr := Baslat(m, s)
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hata verdi: %v", err)
	}
	if sonra != nil {
		sonra(pr)
	}
	return pr.Bitir(0), m
}

func islemBul(g *Goruntu, kod, yon string) *IslemBilgisi {
	for i := range g.Tensor.Islemler {
		if b := &g.Tensor.Islemler[i]; b.Kod == kod && b.Yon == yon {
			return b
		}
	}
	return nil
}

// Sayaçlar bilinen sayıda işlemi doğru sayar; adım sınırları sayılır.
func TestProfilBilinenSayidaIslem(t *testing.T) {
	g, m := calistir(t, kisaEgitim, Secenekler{})
	if m.Debugger != nil {
		t.Fatal("örnekleme istenmediği halde hata ayıklayıcı yuvası dolu")
	}
	if g.Ornekleme != nil {
		t.Fatal("örnekleme kapalıyken satır örnekleri var")
	}
	for _, d := range []struct {
		kod, yon string
		n        int64
	}{
		{"matris_carpimi", "ileri", 200}, {"matris_carpimi", "geri", 200}, {"relu", "ileri", 200},
		{"relu", "geri", 200}, {"adamw", "ileri", 200}, {"toplam", "ileri", 200},
	} {
		b := islemBul(g, d.kod, d.yon)
		if b == nil || b.Cagri != d.n {
			t.Errorf("%s (%s): %+v, beklenen %d çağrı", d.kod, d.yon, b, d.n)
		}
	}
	if g.Adim == nil || g.Adim.Sayi != 200 || g.Adim.Toplam != 200 {
		t.Fatalf("adım bilgisi %+v, beklenen 200/200", g.Adim)
	}
	if g.Tensor.SureSn <= 0 {
		t.Errorf("tensör süresi %v sn", g.Tensor.SureSn)
	}
	// Pay programın duvar süresine bölünür; Windows'ta duvar saati kabadır (0,5–15,6 ms), çok
	// kısa koşuda payın anlamı yoktur.
	if g.SureSn >= 0.05 && (g.Tensor.Pay <= 0 || g.Tensor.Pay > 1.05) {
		t.Errorf("tensör süresi %v sn, programın %v sn'si içinde payı %v", g.Tensor.SureSn, g.SureSn, g.Tensor.Pay)
	}
	if b := islemBul(g, "matris_carpimi", "ileri"); b.MsAdim == nil {
		t.Error("adım varken işlemin adım başına süresi yok")
	}
}

// ms/adım iki adım sınırı arasındaki sürenin adım sayısına bölümüdür.
func TestProfilAdimSinirindanMsAdim(t *testing.T) {
	p := &Profil{baslangic: time.Now()}
	t0 := time.Now()
	p.adimSiniri(logging.StepBoundary{Step: 1, Total: 100, Time: t0})
	p.adimSiniri(logging.StepBoundary{Step: 11, Total: 100, Time: t0.Add(250 * time.Millisecond)})
	b := adimBilgisi(p.adim)
	if b == nil || b.Sayi != 11 || b.Toplam != 100 {
		t.Fatalf("adım bilgisi %+v", b)
	}
	if b.MsAdim != 25 || b.AdimSn != 40 {
		t.Errorf("10 adım 250 ms: %v ms/adım, %v adım/sn; beklenen 25 ve 40", b.MsAdim, b.AdimSn)
	}
	// Pencere: önceki görüntüden bu yana 5 adım 50 ms.
	once := anlik{zaman: t0, adim: p.adim}
	p.adimSiniri(logging.StepBoundary{Step: 16, Total: 100, Time: t0.Add(300 * time.Millisecond)})
	w := p.pencere(anlik{zaman: t0.Add(time.Second), adim: p.adim}, &once)
	if w.Adim != 5 || w.MsAdim != 10 {
		t.Errorf("pencere: %d adım, %v ms/adım; beklenen 5 ve 10", w.Adim, w.MsAdim)
	}
	if adimBilgisi(adimDurumu{}) != nil {
		t.Error("adım sınırı yokken adım bilgisi nil olmalı")
	}
}

// Örnekleme yalnızca istenince açılır; sıcak döngünün satırını bulur.
func TestProfilOrneklemeSatiri(t *testing.T) {
	kod := `değişken s = 0
döngü i içinde 0..400000 {
    s += i % 7
}
yazdır(s)
`
	g, m := calistir(t, kod, Secenekler{Ornekle: true})
	if m.Debugger == nil {
		t.Fatal("örnekleme açıkken yuva boş")
	}
	o := g.Ornekleme
	if o == nil || len(o.Satirlar) == 0 {
		t.Fatalf("örnek yok: %+v", o)
	}
	if s := o.Satirlar[0]; s.Satir != 3 || s.Metin != "s += i % 7" || s.Dosya != "egit.tr" {
		t.Errorf("en sıcak satır %+v; beklenen 3. satır (döngünün gövdesi)", s)
	}
}

// Fonksiyon döndükten sonra çağıran satırın kalan işi çağıran satıra yazılır.
func TestProfilDonustenSonraCagiranSatir(t *testing.T) {
	if !derinlikVar {
		t.Skip("yarış denetimli derlemede örnekleyici çağrı derinliğini okumaz")
	}
	kod := `fonksiyon küçük() {
    sonuç 1
}
değişken a = rastgele_normal([256, 256], tohum: 1)
döngü i içinde 0..40 {
    değişken b = küçük() + toplam(a @ a @ a).değer
}
`
	g, _ := calistir(t, kod, Secenekler{Ornekle: true})
	s := g.Ornekleme.Satirlar[0]
	if s.Satir != 6 || s.Fonksiyon != "" {
		t.Errorf("en sıcak satır %+v; beklenen 6. satır (matris çarpımları küçük() döndükten sonra)", s)
	}
	// The share is a ratio of samples; on a fast machine (CI with AVX2) the whole program can
	// finish in a few samples, which makes the ratio noise. Check it only with enough samples.
	if n := s.Pay * float64(g.Ornekleme.Ornek); n < 20 {
		t.Logf("6. satırda yalnızca %.0f örnek var; tensör payı denetlenmedi", n)
	} else if s.TensorPayi < 0.5 {
		t.Errorf("6. satırın tensör payı %v; zaman matris çarpımında geçmeli", s.TensorPayi)
	}
}

// profil.json biçimi: sürüm, kimlik alanları, atomik yazma (geçici dosya kalmaz).
func TestProfilDurumDosyasi(t *testing.T) {
	klasor := t.TempDir()
	yol := filepath.Join(klasor, DurumDosyasiAdi)
	g, _ := calistir(t, kisaEgitim, Secenekler{DurumDosyasi: yol})
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(veri, &d); err != nil {
		t.Fatalf("profil.json geçerli JSON değil: %v", err)
	}
	if d["biçim"] != BicimAdi || d["sürüm"] != float64(BicimSurumu) || d["tür"] != "rapor" || d["bitti"] != true {
		t.Errorf("kimlik alanları: %v %v %v %v", d["biçim"], d["sürüm"], d["tür"], d["bitti"])
	}
	adim := d["adım"].(map[string]any)
	if adim["sayı"] != float64(200) {
		t.Errorf("adım.sayı %v", adim["sayı"])
	}
	if len(d["tensör"].(map[string]any)["işlemler"].([]any)) != len(g.Tensor.Islemler) {
		t.Error("dosyadaki işlem listesi görüntüyle aynı değil")
	}
	girdiler, _ := os.ReadDir(klasor)
	if len(girdiler) != 1 {
		t.Errorf("klasörde yalnızca profil.json kalmalı: %v", girdiler)
	}
}

// Yerine koyma çalışmayan sürücüde (ör. bulut sürücüsü) yazıcı bir kez uyarır, doğrudan
// yazmaya geçer; doğrudan yazma da çalışmazsa program durmaz.
func TestProfilYerineKoymaBasarisiz(t *testing.T) {
	eski := yerineKoy
	yerineKoy = func(string, string) error { return errors.New("benzetim: yeniden adlandırma desteklenmiyor") }
	defer func() { yerineKoy = eski }()
	klasor := t.TempDir()
	var uyarilar []string
	y := &durumYazici{yol: filepath.Join(klasor, DurumDosyasiAdi), uyari: func(m string) { uyarilar = append(uyarilar, m) }}
	for i := 0; i < 3; i++ {
		if err := y.yaz(&Goruntu{Bicim: BicimAdi, Surum: BicimSurumu, SureSn: float64(i)}); err != nil {
			t.Fatalf("doğrudan yazma başarısız: %v", err)
		}
	}
	var g Goruntu
	veri, _ := os.ReadFile(y.yol)
	if err := json.Unmarshal(veri, &g); err != nil || g.SureSn != 2 {
		t.Fatalf("son yazılan görüntü okunamadı: %v %+v", err, g)
	}
	if !y.dogrudan {
		t.Errorf("art arda %d başarısız yerine koymadan sonra doğrudan yazmaya geçilmeli", enCokYerineKoymaHatasi)
	}
	if len(uyarilar) != 1 || !strings.Contains(uyarilar[0], "doğrudan yazılacak") {
		t.Errorf("tek bir uyarı beklenirdi: %q", uyarilar)
	}
	girdiler, _ := os.ReadDir(klasor)
	if len(girdiler) != 1 {
		t.Errorf("geçici dosya kaldı: %v", girdiler)
	}
	// Klasör yoksa doğrudan yazma da başarısız: uyarı bir kez, panik yok.
	uyarilar = nil
	z := &durumYazici{yol: filepath.Join(klasor, "yok", DurumDosyasiAdi), uyari: func(m string) { uyarilar = append(uyarilar, m) }}
	for i := 0; i < 3; i++ {
		_ = z.yaz(&Goruntu{})
	}
	if len(uyarilar) != 2 { // yerine koyma (geçici dosya açılamadı) + doğrudan yazma
		t.Errorf("uyarılar %q; her sorun bir kez bildirilmeli", uyarilar)
	}
}

// Yazma hatası eğitimi durdurmaz: durum dosyası yazılamayan bir yerde program biter.
func TestProfilYazmaHatasiProgramiDurdurmaz(t *testing.T) {
	var uyari []string
	var mu sync.Mutex
	g, _ := calistir(t, kisaEgitim, Secenekler{
		DurumDosyasi: filepath.Join(t.TempDir(), "yok", "profil.json"),
		Uyari:        func(m string) { mu.Lock(); uyari = append(uyari, m); mu.Unlock() },
	})
	if g.Adim == nil || g.Adim.Sayi != 200 {
		t.Fatalf("program bitmedi: %+v", g.Adim)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(uyari) == 0 {
		t.Error("yazılamayan durum dosyası bildirilmedi")
	}
}

// Terminal değilken canlı gösterim düz özet satırları yazar.
func TestProfilTerminalOlmayanCikti(t *testing.T) {
	eski := duzAralik
	duzAralik = 0
	defer func() { duzAralik = eski }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	c := YeniCanli(w, false, nil)
	if c.Terminal() {
		t.Fatal("terminal istenmediği halde terminal kipi")
	}
	g := &Goruntu{Program: "egit.tr", SureSn: 12, Adim: &AdimBilgisi{Sayi: 120, Toplam: 500, MsAdim: 52.1},
		Tensor: TensorBilgisi{Pay: 0.71, Islemler: []IslemBilgisi{{Ad: "matris çarpımı (@)", Pay: 0.6}}}}
	c.Goster(g)
	c.Kapat()
	c.Goster(g) // kapandıktan sonra yazmaz
	w.Close()
	var b bytes.Buffer
	b.ReadFrom(r)
	satirlar := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(satirlar) != 1 {
		t.Fatalf("tek özet satırı beklenirdi: %q", b.String())
	}
	for _, p := range []string{"adım 120/500", "52,1 ms/adım", "tensör %71", "matris çarpımı (@) %60", "\x1b"} {
		if strings.Contains(satirlar[0], p) == (p == "\x1b") {
			t.Errorf("özet satırında %q: %q", p, satirlar[0])
		}
	}
}

// Ana programda tanımlı fonksiyon ve metotların adları satır metni için bilinir.
func TestProfilAnaFonksiyonlar(t *testing.T) {
	kod := "fonksiyon ileri(x) {\n    sonuç x\n}\nyapı M {\n    a: Tam\n    fonksiyon b() {\n        sonuç 1\n    }\n}\n"
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	m := anaFonksiyonlar(prog)
	if !m["ileri"] || !m["b"] {
		t.Errorf("ana fonksiyonlar %v", m)
	}
	if len(anaFonksiyonlar((*ast.Program)(nil))) != 0 {
		t.Error("boş program")
	}
}
