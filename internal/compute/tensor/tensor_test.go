package tensor

import "testing"

// Gradyan kapısı: cihaz aktarımı (To). CPU'da ikinci bir cihaz, yalnızca cihazı GPU olan
// bir sahte arka uçtur (device_registry_test.go); gerçek kartta aktarımın gradyanı
// internal/compute/gpu testlerindedir.
func init() {
	const fakeIndex = 9
	registerGrad("tensor", gradSpec{name: "to", ops: []string{"to"},
		setup: func(t *testing.T) {
			RegisterBackend(fakeGPU{i: fakeIndex})
			t.Cleanup(func() {
				backendMu.Lock()
				delete(backends, GPU(fakeIndex))
				backendMu.Unlock()
			})
		},
		build:  func(xs []*Tensor) *Tensor { return xs[0].To(GPU(fakeIndex)).Square().To(CPU) },
		inputs: genInputs(gen(-1, 1, 0, 3, 4)), ref: map64(func(x float64) float64 { return x * x })})
}

func TestGradTo(t *testing.T) { runGradGroup(t, "tensor") }

// Kural: SameShape iki şeklin boy ve eksen uzunluklarının aynı olduğunu söyler (boş şekil,
// 0 boyutlu tensörün şeklidir); gpu ve ml aynı yardımcıyı kullanır, kendi kopyalarını tutmaz.
func TestSameShapeComparesRankAndExtents(t *testing.T) {
	for _, c := range []struct {
		a, b []int
		want bool
	}{
		{nil, []int{}, true}, {[]int{2, 3}, []int{2, 3}, true}, {[]int{2, 3}, []int{3, 2}, false}, {[]int{2}, []int{2, 1}, false},
	} {
		if got := SameShape(c.a, c.b); got != c.want {
			t.Errorf("SameShape(%v, %v) = %v", c.a, c.b, got)
		}
	}
}
