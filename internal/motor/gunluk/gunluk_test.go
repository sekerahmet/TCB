package gunluk_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
	_ "tr_dili/internal/motor"
	"tr_dili/internal/motor/gunluk"
)

// esliYazici bir tamponu birden çok çıkışa (standart çıktı ve hata) paylaştırır.
type esliYazici struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (e *esliYazici) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

func (e *esliYazici) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.String()
}

// yeniVM çıktısı tampona giden bir VM kurar; program dosyası klasördedir.
func yeniVM(t *testing.T, klasor string, argumanlar ...string) (*yurut.VM, *esliYazici) {
	t.Helper()
	t.Cleanup(gunluk.TumunuKapat)
	m := yurut.New()
	w := &esliYazici{}
	m.Out, m.Err = w, w
	m.SourceFile = filepath.Join(klasor, "egit.tr")
	m.ArgumanlariAyarla(argumanlar)
	return m, w
}

func calistir(t *testing.T, m *yurut.VM, kod string) (yurut.Value, error) {
	t.Helper()
	p := ayristir.New(sozcuk.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kod)
	}
	return m.Run(prog)
}

// satirlar günlük dosyasının JSON satırlarını çözer.
func satirlar(t *testing.T, yol string) []map[string]any {
	t.Helper()
	veri, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, s := range strings.Split(strings.TrimSuffix(string(veri), "\n"), "\n") {
		var k map[string]any
		if err := json.Unmarshal([]byte(s), &k); err != nil {
			t.Fatalf("geçersiz JSON satırı %q: %v", s, err)
		}
		out = append(out, k)
	}
	return out
}

func yolMetni(yol string) string { return filepath.ToSlash(yol) }

// Günlük: başlangıç kaydı (program, komut satırı ayarları, gizli değer maskeli), ölçü ve
// mesaj kayıtları, NaN ve sonsuz metin olarak, tek elemanlı tensör sayı olarak.
func TestGunlukKayitlari(t *testing.T) {
	k := t.TempDir()
	yol := filepath.Join(k, "kayit", "egitim.jsonl")
	m, _ := yeniVM(t, k, "--hf_token", "hf_gizlideger1")
	_, err := calistir(t, m, `
değişken ayar = komut_satırı({adım: 10, hf_token: ""})
değişken g = günlük("`+yolMetni(yol)+`")
g.ekle({kayıp: tensör([2.5]), nan: ondalık("NaN"), sonsuz: ondalık("inf"), belirteç: ayar.hf_token})
g.ekle("merhaba")
g.ekle({zaman: "elle", x: [1, 2]})
`)
	if err != nil {
		t.Fatal(err)
	}
	s := satirlar(t, yol)
	if len(s) != 4 {
		t.Fatalf("4 satır bekleniyordu: %v", s)
	}
	b := s[0]
	if b["tür"] != "başlangıç" || b["sürüm"] != 1.0 || b["program"] != "egit.tr" {
		t.Errorf("başlangıç kaydı: %v", b)
	}
	if a, _ := b["ayarlar"].(map[string]any); a["adım"] != 10.0 || a["hf_token"] != yurut.GizliMaske {
		t.Errorf("ayarlar gizli değeri maskelenmiş olarak taşımalı: %v", b["ayarlar"])
	}
	o := s[1]
	if o["kayıp"] != 2.5 || o["nan"] != "NaN" || o["sonsuz"] != "+Sonsuz" || o["belirteç"] != yurut.GizliMaske {
		t.Errorf("ölçü kaydı: %v", o)
	}
	if _, ok := o["zaman"].(string); !ok {
		t.Errorf("zaman eklenmeli: %v", o)
	}
	if s[2]["mesaj"] != "merhaba" || s[3]["zaman"] != "elle" {
		t.Errorf("mesaj ve elle verilen zaman: %v %v", s[2], s[3])
	}
	if strings.Contains(string(mustOku(t, yol)), "hf_gizlideger1") {
		t.Error("gizli değer dosyaya düz yazılmamalı")
	}
}

func mustOku(t *testing.T, yol string) []byte {
	t.Helper()
	b, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Hatalar: uzantı, ayrılmış anahtar, desteklenmeyen değer.
func TestGunlukHatalari(t *testing.T) {
	k := t.TempDir()
	m, _ := yeniVM(t, k)
	for _, d := range []struct{ kod, want string }{
		{`günlük("` + yolMetni(filepath.Join(k, "a.csv")) + `")`, "uzantılı olmalı"},
		{`günlük("` + yolMetni(filepath.Join(k, "a.jsonl")) + `").ekle({tür: "x"})`, "'tür' adı günlük biçimine ayrılmış"},
		{`günlük("` + yolMetni(filepath.Join(k, "b.jsonl")) + `").ekle({f: yazdır})`, "ölçüsünün değeri"},
		{`günlük("` + yolMetni(filepath.Join(k, "c.jsonl")) + `").ekle(5)`, "1. argüman olarak Sözlük ya da Metin bekler"},
	} {
		if _, err := calistir(t, m, d.kod); err == nil || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.kod, d.want, err)
		}
	}
}

// Var olan dosyaya eklenir; önceki koşu yarım satır bıraktıysa yeni kayıt yeni satırda başlar.
// Kontrol noktası arayüzü: Uzunluk satır sınırıdır, DevamYaz devam kaydını yazar.
func TestGunlukEklemeVeDevam(t *testing.T) {
	k := t.TempDir()
	yol := filepath.Join(k, "e.jsonl")
	if err := os.WriteFile(yol, []byte(`{"adım":1}`+"\n"+`{"adım":2,"ka`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ := yeniVM(t, k)
	v, err := calistir(t, m, `değişken g = günlük("`+yolMetni(yol)+`")
g.ekle({kayıp: 1})
g`)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := gunluk.GunlukMu(v)
	if !ok {
		t.Fatalf("Günlük bekleniyordu: %v", v)
	}
	n, err := g.Uzunluk()
	if err != nil || n != int64(len(mustOku(t, yol))) {
		t.Errorf("Uzunluk = %d, %v", n, err)
	}
	if err := g.DevamYaz(1500); err != nil {
		t.Fatal(err)
	}
	icerik := string(mustOku(t, yol))
	satir := strings.Split(strings.TrimSuffix(icerik, "\n"), "\n")
	if len(satir) != 5 || satir[1] != `{"adım":2,"ka` || !strings.HasPrefix(satir[2], `{"tür":"başlangıç"`) ||
		!strings.Contains(satir[4], `"tür":"devam"`) || !strings.HasSuffix(satir[4], `"adım":1500}`) {
		t.Errorf("beklenmeyen günlük:\n%s", icerik)
	}
	// Kapatılan günlüğe yazılınca dosya yeniden açılır.
	gunluk.TumunuKapat()
	if _, err := calistir(t, m, `g.ekle("yine")`); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(mustOku(t, yol)), `"mesaj":"yine"}`+"\n") {
		t.Errorf("kapatılan günlük yeniden açılmalı: %s", mustOku(t, yol))
	}
}

// Hiç kayıt yazılmayan günlük kapanırken başlangıç kaydını yazar.
func TestGunlukBosKapanis(t *testing.T) {
	k := t.TempDir()
	yol := filepath.Join(k, "bos.jsonl")
	m, _ := yeniVM(t, k)
	if _, err := calistir(t, m, `günlük("`+yolMetni(yol)+`")`); err != nil {
		t.Fatal(err)
	}
	gunluk.TumunuKapat()
	if s := satirlar(t, yol); len(s) != 1 || s[0]["tür"] != "başlangıç" {
		t.Errorf("tek başlangıç kaydı bekleniyordu: %v", s)
	}
}

// sahteSaat elle ilerletilen bir saattir.
type sahteSaat struct {
	mu sync.Mutex
	t  time.Time
}

func (s *sahteSaat) simdi() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.t
}

func (s *sahteSaat) ilerlet(d time.Duration) {
	s.mu.Lock()
	s.t = s.t.Add(d)
	s.mu.Unlock()
}

func saatKur(t *testing.T) *sahteSaat {
	s := &sahteSaat{t: time.Date(2026, 10, 18, 14, 0, 0, 0, time.Local)}
	t.Cleanup(gunluk.SaatTaklidi(s.simdi))
	return s
}
