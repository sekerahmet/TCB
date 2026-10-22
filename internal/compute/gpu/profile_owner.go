package gpu

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Çekirdek süresinin sahibi: ölçüm açıkken her çekirdek başlatması, onu başlatan tensör
// işlemini ve TR konumunu (tensor.CurrentOwner; geri yönde düğümü üreten ileri satır) taşır;
// çözülen cihaz süresi (işlem, yön, konum) üçlüsüne eklenir. Canlı profil bunu işlem ve satır
// tablolarına yazar: konakta ölçülen süre GPU'da yalnızca başlatma ve bekleme olduğundan,
// "hangi satır GPU'yu meşgul ediyor" sorusunun yanıtı budur. PyTorch'ta aynı bağı CUPTI'nin
// correlationId'si kurar (çekirdek kaydı başlatma çağrısının kimliğini taşır).

// OwnerSample bir (işlem, yön, konum) üçlüsünün GPU çekirdek süresidir.
type OwnerSample struct {
	Owner   tensor.Owner
	Calls   int64
	GPUTime time.Duration
}

// addOwner bir çekirdeğin süresini sahibine ekler (p.mu altında).
func (p *profile) addOwner(o tensor.Owner, ms float64) {
	st := p.owners[o]
	if st == nil {
		st = &pullStats{}
		p.owners[o] = st
	}
	st.num++
	st.ms += ms
}

// OwnerSamples bütün GPU'ların çekirdek süresini sahiplerine göre verir (süreye göre sıralı).
func OwnerSamples() []OwnerSample {
	sum := map[tensor.Owner]*OwnerSample{}
	for _, g := range registeredAll() {
		p := &g.profiler
		p.mu.Lock()
		for o, s := range p.owners {
			t := sum[o]
			if t == nil {
				t = &OwnerSample{Owner: o}
				sum[o] = t
			}
			t.Calls += s.num
			t.GPUTime += time.Duration(s.ms * float64(time.Millisecond))
		}
		p.mu.Unlock()
	}
	r := make([]OwnerSample, 0, len(sum))
	for _, o := range sum {
		r = append(r, *o)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].GPUTime > r[j].GPUTime })
	return r
}

// externalSteps adım sınırını profilin verdiğidir (MarkStep); açıkken iyileştiricinin kendi
// işaretleri yok sayılır.
var externalSteps atomic.Bool

// SetExternalSteps adım sınırını profile bırakır (true) ya da iyileştiriciye geri verir.
func SetExternalSteps(on bool) { externalSteps.Store(on) }

// MarkStep bütün GPU'larda yeni bir eğitim adımını başlatır (profilin adım sınırı).
func MarkStep() {
	for _, g := range registeredAll() {
		g.enter()
		g.markStepNow()
		g.exit()
	}
}
