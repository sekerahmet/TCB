package gpu

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// rowMap satırları (son eksen bitişik) en çok iki düzeyli adıma indirir; birleşemeyen üç düzey
// ve bitişik olmayan son eksen kopyaya düşer (ok yanlış). Adım 0 (yayın) yalnızca adım 0 ile
// birleşir.
func TestRowMap(t *testing.T) {
	for _, c := range []struct {
		shape, strides []int
		H, S0, S1      int
		ok             bool
	}{
		{[]int{5, 64}, []int{64, 1}, 1, 0, 64, true},
		{[]int{1, 64}, []int{64, 1}, 1, 0, 64, true},
		{[]int{1, 1, 64}, []int{7, 9, 1}, 1, 0, 64, true},
		{[]int{4, 3, 64}, []int{192, 64, 1}, 1, 0, 64, true},           // bitişik birleşir
		{[]int{4, 3, 64}, []int{384, 64, 1}, 3, 64, 384, true},         // K yarısı
		{[]int{4, 1, 3, 64}, []int{384, 192, 64, 1}, 3, 64, 384, true}, // boyu 1 eksen atlanır
		{[]int{4, 3, 64}, []int{64, 256, 1}, 3, 256, 64, true},         // yer değiştirmiş
		{[]int{2, 4, 3, 64}, []int{2000, 384, 64, 1}, 0, 0, 0, false},  // üç düzey
		{[]int{2, 4, 3, 64}, []int{1536, 384, 64, 1}, 3, 64, 384, true},
		{[]int{4, 3, 64}, []int{0, 64, 1}, 3, 64, 0, true}, // yayın (adım 0)
		{[]int{4, 3, 64}, []int{0, 0, 1}, 1, 0, 0, true},   // iki adım 0 birleşir
		{[]int{4, 64}, []int{64, 2}, 0, 0, 0, false},       // son eksen bitişik değil
		{[]int{4, 64}, []int{64, 0}, 0, 0, 0, false},
	} {
		H, S0, S1, ok := rowMap(tensor.View{Shape: c.shape, Strides: c.strides}, c.shape[len(c.shape)-1])
		if ok != c.ok || (ok && (H != c.H || S0 != c.S0 || S1 != c.S1)) {
			t.Errorf("%v/%v: (%d,%d,%d,%v), beklenen (%d,%d,%d,%v)", c.shape, c.strides, H, S0, S1, ok, c.H, c.S0, c.S1, c.ok)
		}
	}
}
