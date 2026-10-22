package profiler

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// bufferedWriter programın çıktısını toplar (birden çok goroutine yazabilir).
type bufferedWriter struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *bufferedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// shortTraining 200 adımlık küçük bir eğitimdir: her adımda bilinen sayıda tensör işlemi.
const shortTraining = `değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
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

// run kodu bir VM'de profille çalıştırır ve son raporu verir.
func run(t *testing.T, code string, s Options) (*Snapshot, *interp.VM) {
	t.Helper()
	return runThen(t, code, s, nil)
}

// runThen calistir gibidir; sonra nil değilse program bittikten sonra, rapor
// alınmadan önce çağrılır.
func runThen(t *testing.T, code string, s Options, after func(*Profile)) (*Snapshot, *interp.VM) {
	t.Helper()
	t.Cleanup(logging.CloseAll)
	prog := trrun.Parse(t, code)
	m := interp.New()
	w := &bufferedWriter{}
	m.Out, m.Err = w, w
	m.SourceFile = filepath.Join(t.TempDir(), "egit.tr")
	s.Program, s.Source = "egit.tr", code
	pr := Start(m, s)
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hata verdi: %v", err)
	}
	if after != nil {
		after(pr)
	}
	return pr.Finish(0), m
}

func findOp(g *Snapshot, code, direction string) *OpInfo {
	for i := range g.Tensor.Ops {
		if b := &g.Tensor.Ops[i]; b.Code == code && b.Direction == direction {
			return b
		}
	}
	return nil
}

// Sayaçlar bilinen sayıda işlemi doğru sayar; adım sınırları sayılır.
func TestProfileKnownOpCount(t *testing.T) {
	g, m := run(t, shortTraining, Options{})
	if m.Debugger != nil {
		t.Fatal("örnekleme istenmediği halde hata ayıklayıcı yuvası dolu")
	}
	if g.Sampling != nil {
		t.Fatal("örnekleme kapalıyken satır örnekleri var")
	}
	for _, d := range []struct {
		code, direction string
		n               int64
	}{
		{"matris_carpimi", "ileri", 200}, {"matris_carpimi", "geri", 200}, {"relu", "ileri", 200},
		{"relu", "geri", 200}, {"adamw", "ileri", 200}, {"toplam", "ileri", 200},
	} {
		b := findOp(g, d.code, d.direction)
		if b == nil || b.Calls != d.n {
			t.Errorf("%s (%s): %+v, beklenen %d çağrı", d.code, d.direction, b, d.n)
		}
	}
	if g.Step == nil || g.Step.Count != 200 || g.Step.Total != 200 {
		t.Fatalf("adım bilgisi %+v, beklenen 200/200", g.Step)
	}
	if g.Tensor.Seconds <= 0 {
		t.Errorf("tensör süresi %v sn", g.Tensor.Seconds)
	}
	// Pay programın duvar süresine bölünür; Windows'ta duvar saati kabadır (0,5–15,6 ms), çok
	// kısa koşuda payın anlamı yoktur.
	if g.Seconds >= 0.05 && (g.Tensor.Share <= 0 || g.Tensor.Share > 1.05) {
		t.Errorf("tensör süresi %v sn, programın %v sn'si içinde payı %v", g.Tensor.Seconds, g.Seconds, g.Tensor.Share)
	}
	if b := findOp(g, "matris_carpimi", "ileri"); b.MsPerStep == nil {
		t.Error("adım varken işlemin adım başına süresi yok")
	}
}

// ms/adım iki adım sınırı arasındaki sürenin adım sayısına bölümüdür.
func TestProfileMsPerStepFromStepBoundary(t *testing.T) {
	p := &Profile{start: time.Now()}
	t0 := time.Now()
	p.stepBoundary(logging.StepBoundary{Step: 1, Total: 100, Time: t0})
	p.stepBoundary(logging.StepBoundary{Step: 11, Total: 100, Time: t0.Add(250 * time.Millisecond)})
	b := stepInfo(p.step)
	if b == nil || b.Count != 11 || b.Total != 100 {
		t.Fatalf("adım bilgisi %+v", b)
	}
	if b.MsPerStep != 25 || b.StepsPerSec != 40 {
		t.Errorf("10 adım 250 ms: %v ms/adım, %v adım/sn; beklenen 25 ve 40", b.MsPerStep, b.StepsPerSec)
	}
	// Pencere: önceki görüntüden bu yana 5 adım 50 ms.
	before := snap{time2: t0, step: p.step}
	p.stepBoundary(logging.StepBoundary{Step: 16, Total: 100, Time: t0.Add(300 * time.Millisecond)})
	w := p.window(snap{time2: t0.Add(time.Second), step: p.step}, &before)
	if w.Step != 5 || w.MsPerStep != 10 {
		t.Errorf("pencere: %d adım, %v ms/adım; beklenen 5 ve 10", w.Step, w.MsPerStep)
	}
	if stepInfo(stepState{}) != nil {
		t.Error("adım sınırı yokken adım bilgisi nil olmalı")
	}
}

// Örnekleme yalnızca istenince açılır; sıcak döngünün satırını bulur.
func TestProfileSamplingLine(t *testing.T) {
	code := `değişken s = 0
döngü i içinde 0..400000 {
    s += i % 7
}
yazdır(s)
`
	g, m := run(t, code, Options{SampleLines: true})
	if m.Debugger == nil {
		t.Fatal("örnekleme açıkken yuva boş")
	}
	o := g.Sampling
	if o == nil || len(o.Lines) == 0 {
		t.Fatalf("örnek yok: %+v", o)
	}
	if s := o.Lines[0]; s.Line != 3 || s.Text != "s += i % 7" || s.File != "egit.tr" {
		t.Errorf("en sıcak satır %+v; beklenen 3. satır (döngünün gövdesi)", s)
	}
}

// Fonksiyon döndükten sonra çağıran satırın kalan işi çağıran satıra yazılır.
func TestProfileCallerLineAfterReturn(t *testing.T) {
	if !hasDepth {
		t.Skip("yarış denetimli derlemede örnekleyici çağrı derinliğini okumaz")
	}
	code := `fonksiyon küçük() {
    sonuç 1
}
değişken a = rastgele_normal([256, 256], tohum: 1)
döngü i içinde 0..120 {
    değişken b = küçük() + toplam(a @ a @ a).değer
}
`
	g, _ := run(t, code, Options{SampleLines: true})
	if len(g.Sampling.Lines) == 0 {
		t.Skip("program örnek alınamadan bitti (çok hızlı makine); satır denetlenmedi")
	}
	s := g.Sampling.Lines[0]
	if s.Line != 6 || s.Function != "" {
		t.Errorf("en sıcak satır %+v; beklenen 6. satır (matris çarpımları küçük() döndükten sonra)", s)
	}
	// The share is a ratio of samples; on a fast machine (CI with AVX2) the whole program can
	// finish in a few samples, which makes the ratio noise. Check it only with enough samples.
	if n := s.Share * float64(g.Sampling.Samples); n < 20 {
		t.Logf("6. satırda yalnızca %.0f örnek var; tensör payı denetlenmedi", n)
	} else if s.TensorShare < 0.5 {
		t.Errorf("6. satırın tensör payı %v; zaman matris çarpımında geçmeli", s.TensorShare)
	}
}

// Geri yayılımın süresi geri_yayılım(...) satırına değil, düğümleri üreten ileri satıra yazılır.
func TestProfileBackwardTimeGoesToForwardLine(t *testing.T) {
	if !hasDepth {
		t.Skip("yarış denetimli derlemede örnekleyici çağrı derinliğini okumaz")
	}
	code := `değişken w = tensör(rastgele_normal([256, 256], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([256, 256], tohum: 2)
döngü i içinde 0..60 {
    değişken y = x @ w @ w @ w
    değişken kayıp = toplam(y)
    geri_yayılım(kayıp)
}
`
	g, _ := run(t, code, Options{SampleLines: true})
	var forward, backward *LineInfo
	for i, s := range g.Sampling.Lines {
		switch s.Line {
		case 4:
			forward = &g.Sampling.Lines[i]
		case 6:
			backward = &g.Sampling.Lines[i]
		}
	}
	if forward == nil || forward.Share*float64(g.Sampling.Samples) < 20 {
		t.Skip("4. satırda yeterli örnek yok (çok hızlı makine); denetlenmedi")
	}
	if forward.BackwardShare < 0.3 {
		t.Errorf("4. satırın geri payı %v; çarpımların geri hesabı bu satıra yazılmalı", forward.BackwardShare)
	}
	if backward != nil && backward.Seconds > forward.Seconds/2 {
		t.Errorf("geri_yayılım satırı %v sn, ileri satır %v sn; geri hesap ileri satıra yazılmalı", backward.Seconds, forward.Seconds)
	}
}

// profil.json biçimi: sürüm, kimlik alanları, atomik yazma (geçici dosya kalmaz).
func TestProfileStatusFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, StatusFileName)
	g, _ := run(t, shortTraining, Options{StatusFile: fpath})
	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("profil.json geçerli JSON değil: %v", err)
	}
	if d["biçim"] != FormatName || d["sürüm"] != float64(FormatVersion) || d["tür"] != "rapor" || d["bitti"] != true {
		t.Errorf("kimlik alanları: %v %v %v %v", d["biçim"], d["sürüm"], d["tür"], d["bitti"])
	}
	step := d["adım"].(map[string]any)
	if step["sayı"] != float64(200) {
		t.Errorf("adım.sayı %v", step["sayı"])
	}
	if len(d["tensör"].(map[string]any)["işlemler"].([]any)) != len(g.Tensor.Ops) {
		t.Error("dosyadaki işlem listesi görüntüyle aynı değil")
	}
	inputs, _ := os.ReadDir(dir)
	if len(inputs) != 1 {
		t.Errorf("klasörde yalnızca profil.json kalmalı: %v", inputs)
	}
}

// Yerine koyma çalışmayan sürücüde (ör. bulut sürücüsü) yazıcı bir kez uyarır, doğrudan
// yazmaya geçer; doğrudan yazma da çalışmazsa program durmaz.
func TestProfileReplaceFails(t *testing.T) {
	old := replace
	replace = func(string, string) error { return errors.New("benzetim: yeniden adlandırma desteklenmiyor") }
	defer func() { replace = old }()
	dir := t.TempDir()
	var warnings []string
	y := &statusWriter{fpath: filepath.Join(dir, StatusFileName), warning: func(m string) { warnings = append(warnings, m) }}
	for i := 0; i < 3; i++ {
		if err := y.write(&Snapshot{Format: FormatName, Version: FormatVersion, Seconds: float64(i)}); err != nil {
			t.Fatalf("doğrudan yazma başarısız: %v", err)
		}
	}
	var g Snapshot
	data, _ := os.ReadFile(y.fpath)
	if err := json.Unmarshal(data, &g); err != nil || g.Seconds != 2 {
		t.Fatalf("son yazılan görüntü okunamadı: %v %+v", err, g)
	}
	if !y.direct {
		t.Errorf("art arda %d başarısız yerine koymadan sonra doğrudan yazmaya geçilmeli", maxReplaceErrors)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "doğrudan yazılacak") {
		t.Errorf("tek bir uyarı beklenirdi: %q", warnings)
	}
	inputs, _ := os.ReadDir(dir)
	if len(inputs) != 1 {
		t.Errorf("geçici dosya kaldı: %v", inputs)
	}
	// Klasör yoksa doğrudan yazma da başarısız: uyarı bir kez, panik yok.
	warnings = nil
	z := &statusWriter{fpath: filepath.Join(dir, "yok", StatusFileName), warning: func(m string) { warnings = append(warnings, m) }}
	for i := 0; i < 3; i++ {
		_ = z.write(&Snapshot{})
	}
	if len(warnings) != 2 { // yerine koyma (geçici dosya açılamadı) + doğrudan yazma
		t.Errorf("uyarılar %q; her sorun bir kez bildirilmeli", warnings)
	}
}

// Yazma hatası eğitimi durdurmaz: durum dosyası yazılamayan bir yerde program biter.
func TestProfileWriteErrorDoesNotStopProgram(t *testing.T) {
	var warning []string
	var mu sync.Mutex
	g, _ := run(t, shortTraining, Options{
		StatusFile: filepath.Join(t.TempDir(), "yok", "profil.json"),
		Warning:    func(m string) { mu.Lock(); warning = append(warning, m); mu.Unlock() },
	})
	if g.Step == nil || g.Step.Count != 200 {
		t.Fatalf("program bitmedi: %+v", g.Step)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(warning) == 0 {
		t.Error("yazılamayan durum dosyası bildirilmedi")
	}
}

// Terminal değilken canlı gösterim düz özet satırları yazar.
func TestProfileNonTerminalOutput(t *testing.T) {
	old := plainInterval
	plainInterval = 0
	defer func() { plainInterval = old }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	c := NewLive(w, false, nil)
	if c.Terminal() {
		t.Fatal("terminal istenmediği halde terminal kipi")
	}
	g := &Snapshot{Program: "egit.tr", Seconds: 12, Step: &StepInfo{Count: 120, Total: 500, MsPerStep: 52.1},
		Tensor: TensorInfo{Share: 0.71, Ops: []OpInfo{{Name: "matris çarpımı (@)", Share: 0.6}}}}
	c.Show(g)
	c.Close()
	c.Show(g) // kapandıktan sonra yazmaz
	w.Close()
	var b bytes.Buffer
	b.ReadFrom(r)
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("tek özet satırı beklenirdi: %q", b.String())
	}
	for _, p := range []string{"adım 120/500", "52,1 ms/adım", "tensör %71", "matris çarpımı (@) %60", "\x1b"} {
		if strings.Contains(lines[0], p) == (p == "\x1b") {
			t.Errorf("özet satırında %q: %q", p, lines[0])
		}
	}
}

// 'aktar' ile yüklenen modülün satırları (fonksiyonları ve üst düzey kodu) modülün dosyasıyla
// ve metniyle bildirilir: ana dosyada aynı adlı bir fonksiyon olsa da ana dosyaya yazılmaz.
func TestProfileModuleLines(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "yardımcı.tr")
	code := "değişken ısınma = 0\ndöngü j içinde 0..300000 {\n    ısınma += j % 3\n}\nfonksiyon ileri(x) {\n    değişken s = 0\n    döngü i içinde 0..300000 {\n        s += i % 7\n    }\n    sonuç s + x\n}\n"
	if err := os.WriteFile(module, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	main := "aktar \"" + filepath.ToSlash(module) + "\" olarak m\nfonksiyon ileri(x) {\n    sonuç x\n}\nyazdır(m.ileri(ileri(1)))\n"
	g, _ := run(t, main, Options{SampleLines: true})
	want := map[int]string{3: "ısınma += j % 3", 8: "s += i % 7"}
	found := 0
	for _, s := range g.Sampling.Lines {
		text, ok := want[s.Line]
		if !ok || s.Share < 0.05 {
			continue
		}
		found++
		if s.File == "egit.tr" || s.Text != text || filepath.Base(filepath.FromSlash(s.File)) != "yardımcı.tr" {
			t.Errorf("modülün %d. satırı modülün dosyası ve metniyle bildirilmeli: %+v", s.Line, s)
		}
	}
	if found == 0 {
		t.Fatalf("modülün sıcak satırları örneklenmedi: %+v", g.Sampling.Lines)
	}
}

// Sayaçlar profil başından beri bütün adımları içerir: işlemin adım başına süresi tamamlanan
// adım sayısına bölünür (GPU raporundaki gibi), adım sınırları arasındaki aralık sayısına değil.
func TestProfileOpMsPerStepUsesAllSteps(t *testing.T) {
	g, _ := run(t, shortTraining, Options{})
	b := findOp(g, "matris_carpimi", "ileri")
	if b == nil || b.MsPerStep == nil {
		t.Fatalf("işlem ya da adım başına süresi yok: %+v", b)
	}
	if want := b.Seconds * 1000 / 200; math.Abs(*b.MsPerStep-want) > 1e-9*math.Max(1, want) {
		t.Fatalf("ms/adım %v, beklenen %v (200 adım)", *b.MsPerStep, want)
	}
}

// Her epoch'ta yeni bir ilerleme çubuğu (adım 1'den yeniden başlar) adımları sıfırlamaz:
// tamamlanan adımlar çubuklar boyunca birikir (Count son çubuğun adımıdır).
const epochTraining = `değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)
döngü e içinde 0..2 {
    değişken ç = ilerleme(100)
    döngü i içinde 0..100 {
        değişken kayıp = toplam(relu(x @ w))
        ç.güncelle({kayıp: kayıp})
    }
}
`

func TestProfileStepsAcrossProgressBars(t *testing.T) {
	g, _ := run(t, epochTraining, Options{})
	if g.Step == nil || g.Step.All != 200 || g.Step.Count != 100 {
		t.Fatalf("iki çubukta toplam 200 adım, son çubukta 100 bekleniyordu: %+v", g.Step)
	}
	b := findOp(g, "matris_carpimi", "ileri")
	if b == nil || b.MsPerStep == nil {
		t.Fatalf("işlemin adım başına süresi yok: %+v", b)
	}
	if want := b.Seconds * 1000 / 200; math.Abs(*b.MsPerStep-want) > 1e-9*math.Max(1, want) {
		t.Fatalf("ms/adım %v, beklenen %v (200 adım)", *b.MsPerStep, want)
	}

	// Pencere çubuk değişimini aşar: 100. adımdan sonra yeni çubukta 5 adım daha.
	p := &Profile{start: time.Now()}
	t0 := time.Now()
	p.stepBoundary(logging.StepBoundary{Step: 99, Total: 100, Time: t0})
	before := snap{time2: t0, step: p.step}
	p.stepBoundary(logging.StepBoundary{Step: 100, Total: 100, Time: t0.Add(10 * time.Millisecond)})
	p.stepBoundary(logging.StepBoundary{Step: 5, Total: 100, Time: t0.Add(60 * time.Millisecond)})
	w := p.window(snap{time2: t0.Add(time.Second), step: p.step}, &before)
	if w.Step != 6 || w.MsPerStep != 10 {
		t.Errorf("pencere: %d adım, %v ms/adım; beklenen 6 ve 10", w.Step, w.MsPerStep)
	}
}
