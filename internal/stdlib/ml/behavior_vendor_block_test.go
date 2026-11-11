package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.53b; DENETİM 467 N1): yorumlayıcının TR_vendor_matmul bloğunun durumunu çekirdeğe
// bağladığı üç yer bloğu kitaplık girişine ulaştırır, blok dışında ulaştırmaz: `@` (unlocked.go'nun
// matMulOn'u), TR_attention'ın çarpımlı yolu (model_tools.go'nun attentionBuiltin'i) ve
// TR_linear_cross_entropy (builtin_nn.go). Benzeticide kitaplık yoktur: blok kendi çekirdekle sürer,
// bitler blok dışındakiyle aynıdır; girişe ulaşmayı gpu paketinin sayacı (gpu.VendorCalls) gösterir.
// Kitaplığın kendisi kartta sınanır (gpu paketinin behavior_vendor_matmul_test.go'su, L4 koşusu).
func TestVendorMatmulBlockReachesTheLibraryEntry(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	if !gpu.Simulated() {
		t.Skip("giriş sayacı benzeticide sınanır; kartta kitaplık gerçekten çağrılır")
	}
	// Dikkatin bakış boyu 512: birleşik çekirdeğin paylaşımlı belleğine sığmaz, dikkat matris
	// çarpımı yoluna gider (gpu paketinin pickFlash'ı); Tq = 4 olduğu için çarpımlar M > 1'dir.
	const prelude = `TR_var x = (TR_ones([4, 8]) * 0.5).TR_to_device("gpu")
TR_var w = (TR_ones([8, 6]) * 0.25).TR_to_device("gpu")
TR_var q = (TR_ones([1, 4, 1, 512]) * 0.01).TR_to_device("gpu")
TR_var hedef = TR_Tensor([0, 1, 2, 3])
`
	for _, c := range []struct{ name, expr string }{
		{"@ (matMulOn)", "x @ w"},
		{"TR_attention'ın çarpımlı yolu", "TR_attention(q, q, q)"},
		{"TR_linear_cross_entropy", "TR_linear_cross_entropy(x, w, hedef)"},
	} {
		before := gpu.VendorCalls()
		outside := mustRun(t, prelude+"TR_print(TR_sum("+c.expr+").TR_item())")
		if n := gpu.VendorCalls() - before; n != 0 {
			t.Errorf("%s: blok dışında kitaplık girişine %d çarpım gitti", c.name, n)
		}
		before = gpu.VendorCalls()
		inside := mustRun(t, prelude+"TR_print(TR_vendor_matmul { TR_sum("+c.expr+") }.TR_item())")
		if n := gpu.VendorCalls() - before; n == 0 {
			t.Errorf("%s: blokta kitaplık girişine gitmedi", c.name)
		}
		if inside != outside {
			t.Errorf("%s: kitaplıksız blokta sonuç %q, blok dışında %q", c.name, inside, outside)
		}
	}
}
