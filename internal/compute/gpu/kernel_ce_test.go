package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestCrossEntropyNegInf −∞ logitli satırlar (başta, ortada, tamamı): −∞ katkı vermez;
// tamamı −∞ olan satır CPU'daki gibi NaN verir. Tekli ve 4'lü vektör yolu.
func TestCrossEntropyNegInf(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	minus := float32(math.Inf(-1))
	for _, C := range []int{7, 300} {
		R := 4
		v := random(rng, R*C, 1)
		v[0*C] = minus                  // satır 0: ilk eleman (her iş parçacığının ilk gördüğü)
		v[1*C], v[1*C+1] = minus, minus // satır 1: ilk iki
		v[2*C+C/2] = minus              // satır 2: ortada
		target := []int{1, 2, 3, 0}
		compare(t, fmt.Sprintf("ce −∞ C=%d", C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(v, true, R, C)[c]
			return tensor.CrossEntropy(x, target), []*tensor.Tensor{x}
		})
		all := append([]float32(nil), v...)
		for j := 0; j < C; j++ {
			all[3*C+j] = minus // satır 3: tamamı −∞ (CPU: NaN)
		}
		compare(t, fmt.Sprintf("ce tamamı −∞ satır C=%d", C), 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(all, true, R, C)[c]
			return tensor.CrossEntropy(x, target), []*tensor.Tensor{x}
		})
	}
}
