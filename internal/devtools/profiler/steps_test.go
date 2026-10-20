package profiler

import (
	"math"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

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
