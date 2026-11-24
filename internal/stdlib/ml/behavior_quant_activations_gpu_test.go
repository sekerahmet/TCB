package ml

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// TestQuantActIgnoresVendorBlock 15.53a'nın çarpımı GPU'da da tamsayı yoludur (A1, A3): TR_quantized_activations
// bloğunda nicemli ağırlıkla `@` kartta hata vermez, CPU'nun bitlerini verir ve TR_vendor_matmul bloğu onu
// değiştirmez (bloklar birlikte, iki sırayla; 15.53b'nin "her zaman TR'nin kendi çekirdeği" cümlesi). Blok dışında
// kartta float yolu kalır (farklı bitler). q4, q4_k; tek ve çok satır.
func TestQuantActIgnoresVendorBlock(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, tc := range []struct {
		dt string
		M  int
		w  func(seed int64, N, K int) *tensor.Tensor
	}{{"q4_k", 1, q4kWeight}, {"q4_k", 5, q4kWeight}, {"q4", 1, q4Weight}, {"q4", 5, q4Weight}} {
		name := fmt.Sprintf("%s M=%d", tc.dt, tc.M)
		x := tensor.Randn(rand.New(rand.NewSource(int64(60+tc.M))), tc.M, 512)
		w := tc.w(int64(70+tc.M), 7, 512)
		xg, wg := x.To(tensor.GPU(0)), w.To(tensor.GPU(0))
		run := func(xx, ww *tensor.Tensor, act, vendor bool) []float32 {
			m := interp.New()
			m.SetTaskValue(actQuantKey{}, act)
			m.SetTaskValue(vendorMatmulKey{}, vendor)
			r, err := matmulDType(m, xx, ww.T(), tensor.Float32)
			if err != nil {
				t.Fatalf("%s (blok %v, kitaplık %v): %v", name, act, vendor, err)
			}
			return r.Data()
		}
		want := run(x, w, true, false)
		for _, vendor := range []bool{false, true} {
			got := run(xg, wg, true, vendor)
			for i := range want {
				if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
					t.Fatalf("%s kitaplık %v [%d]: kartta %v, CPU %v", name, vendor, i, got[i], want[i])
				}
			}
		}
		outside := run(xg, wg, false, false)
		same := true
		for i := range want {
			same = same && math.Float32bits(outside[i]) == math.Float32bits(want[i])
		}
		if same {
			t.Fatalf("%s: blok dışında da tamsayı yolu kullanıldı", name)
		}
	}
}
