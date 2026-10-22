package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestFlashMmaBackward tensör çekirdekli dikkatin geri yönünü (dQ, dK, dV) CPU ile karşılaştırır:
// döşeme kenarları, nedensel, pencere, Tq ≠ Tk, gruplu sorgu. Gradyanlar 16 bitlik dönüşümün
// önündeki float32 yapraklara akar. P, dS ve dO 16 bite yuvarlandığı için tolerans geniştir.
func TestFlashMmaBackward(t *testing.T) {
	skipUnderRace(t)
	g := gpuBackend()
	old := g.attnPath
	defer func() { g.attnPath = old }()
	if inSim() {
		g.attnPath = "tensor"
	} else if cc, _ := g.sr.capability(); cc < 80 {
		t.Skip("tensör çekirdeği sm_80 ister")
	}
	rng := rand.New(rand.NewSource(47))
	for _, d := range []struct {
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
	}{
		{1, 2, 2, 130, 130, 64, 64, tensor.AttentionOptions{Causal: true}},
		{1, 4, 2, 70, 200, 32, 48, tensor.AttentionOptions{}},
		{2, 2, 1, 150, 150, 16, 16, tensor.AttentionOptions{Causal: true, Window: 40}},
		// Bakış boyu 64'ün üstü: döşeme 32 (mmaBackTile); 32'nin katı olmayan boylar kenarları yoklar.
		{1, 4, 2, 70, 130, 128, 128, tensor.AttentionOptions{Causal: true}},
		{1, 2, 2, 50, 90, 96, 80, tensor.AttentionOptions{Window: 30}},
		{1, 2, 1, 40, 40, 128, 64, tensor.AttentionOptions{}},
	} {
		for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
			ad := fmt.Sprintf("dikkat_tc_geri %v %+v", dt, d)
			qv := random(rng, d.B*d.H*d.Tq*d.Dh, 1)
			kv := random(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
			vv := random(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
			compare(t, ad, 3e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				q := twin(qv, true, d.B, d.Tq, d.H, d.Dh)[c]
				k := twin(kv, true, d.B, d.Tk, d.Hkv, d.Dh)[c]
				v := twin(vv, true, d.B, d.Tk, d.Hkv, d.Dv)[c]
				out := tensor.AttentionOut(q.Cast(dt), k.Cast(dt), v.Cast(dt), d.opt, tensor.Float32)
				return out, []*tensor.Tensor{q, k, v}
			})
			if !strings.HasPrefix(g.lastKernel, "dikkat_geri_kv_tc") && !strings.HasPrefix(g.lastKernel, "kopya") {
				t.Fatalf("%s: tensör çekirdekli geri yol kullanılmadı (son çekirdek %s)", ad, g.lastKernel)
			}
		}
	}
}
