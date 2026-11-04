package tensor

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestCheckLNEmptyOptional boş ama nil olmayan isteğe bağlı bir dilimin (ör. mean) "yok"
// sayılmadığını, kısa dilim gibi reddedildiğini denetler: AVX2 çekirdeği onu yok, Go eşi var
// sayardı.
func TestCheckLNEmptyOptional(t *testing.T) {
	x, a, rstd := make([]float32, 8), make([]float32, 8), make([]float32, 2)
	if R := checkLN(4, x, a, nil, nil, nil, rstd, nil, nil, nil); R != 2 {
		t.Fatalf("satır sayısı %d, 2 bekleniyordu", R)
	}
	for ad, f := range map[string]func(){
		"mean": func() { checkLN(4, x, a, nil, nil, []float32{}, rstd, nil, nil, nil) },
		"g":    func() { checkLN(4, x, a, []float32{}, nil, nil, rstd, nil, nil, nil) },
		"pg":   func() { checkLN(4, x, a, nil, nil, nil, rstd, nil, []float64{}, nil) },
	} {
		func() {
			defer func() {
				if r, _ := recover().(error); !messages.Has(r, messages.H9000) {
					t.Errorf("%s: boş dilim reddedilmedi (%v)", ad, r)
				}
			}()
			f()
		}()
	}
}

// 16 bitlik tablonun yalnız seçilen satırlarını açan gömme, bütün tablonun açıldığı eski yolla
// bit düzeyinde aynıdır: normal altı, sonsuz ve NaN değerlerde, yinelenen indekste, bitişik
// görünümün uzaklığında; bitişik olmayan görünüm bütün tabloyu açan yola gider.
func TestEmbedding16MatchesWholeTable(t *testing.T) {
	values := []float32{0, float32(math.Copysign(0, -1)), 1.5, -3.25, 1e-40, 6e-8, 70000, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	data := make([]float32, 7*12)
	for i := range data {
		data[i] = values[i%len(values)] * float32(1+i%3)
	}
	for _, dt := range []DType{BFloat16, Float16} {
		w := NewOf(dt, data, 7, 12)
		for _, c := range []struct {
			name string
			w    *Tensor
			idx  []int
		}{
			{"bitişik", w, []int{0, 6, 3, 3, 1}},
			{"tek satır", w, []int{5}},
			{"boş indeks", w, nil},
			{"uzaklıklı görünüm", w.Slice(0, 2, 6), []int{3, 0, 0}},
			{"bitişik olmayan görünüm", w.Transpose(0, 1), []int{11, 0, 4}},
		} {
			D := c.w.Shape()[1]
			got, want := cpuBackend{}.Alloc(len(c.idx)*D, dt), cpuBackend{}.Alloc(len(c.idx)*D, dt)
			cpuBackend{}.Embedding(got, c.w.view(), c.idx, D)
			var gt temp
			o, last := gt.write(want, false)
			cpuBackend{}.f32Embedding(o, gt.read(c.w.view()), c.idx, D)
			last()
			gt.finish()
			g, e := cpuStore(got).h, cpuStore(want).h
			for i := range e {
				if g[i] != e[i] {
					t.Fatalf("%s %s: [%d] = %#04x, eski yol %#04x", dt, c.name, i, g[i], e[i])
				}
			}
		}
	}
}
