package gpu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestFlashMmaForward tensör çekirdekli dikkatin (16 bitlik girdiler) ileri yönünü CPU ile
// karşılaştırır: sorgu ve anahtar döşemesi kenarları (Tq, Tk 128'in ve 64'ün katı değil),
// nedensel, pencere, Tq ≠ Tk ve gruplu sorgu (Hkv < H). P 16 bite yuvarlandığı için tolerans
// geniştir. Benzetimde yol TR_GPU_DIKKAT=tensor ile zorlanır.
func TestFlashMmaForward(t *testing.T) {
	g := gpuBackend()
	old := g.attnPath
	defer func() { g.attnPath = old }()
	if inSim() {
		g.attnPath = "tensor"
	} else if cc, _ := g.sr.capability(); cc < 80 {
		t.Skip("tensör çekirdeği sm_80 ister")
	}
	rng := rand.New(rand.NewSource(43))
	for _, d := range []struct {
		B, H, Hkv, Tq, Tk, Dh, Dv int
		opt                       tensor.AttentionOptions
	}{
		{1, 2, 2, 130, 130, 64, 64, tensor.AttentionOptions{Causal: true}},
		{1, 4, 2, 70, 200, 32, 48, tensor.AttentionOptions{}},
		{2, 2, 1, 150, 150, 16, 16, tensor.AttentionOptions{Causal: true, Window: 40}},
	} {
		for _, dt := range []tensor.DType{tensor.BFloat16, tensor.Float16} {
			ad := fmt.Sprintf("dikkat_tc %v %+v", dt, d)
			qv := random(rng, d.B*d.H*d.Tq*d.Dh, 1)
			kv := random(rng, d.B*d.Hkv*d.Tk*d.Dh, 1)
			vv := random(rng, d.B*d.Hkv*d.Tk*d.Dv, 1)
			compare(t, ad, 1e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				q := twin(qv, false, d.B, d.H, d.Tq, d.Dh)[c].Cast(dt)
				k := twin(kv, false, d.B, d.Hkv, d.Tk, d.Dh)[c].Cast(dt)
				v := twin(vv, false, d.B, d.Hkv, d.Tk, d.Dv)[c].Cast(dt)
				out := tensor.AttentionOut(q, k, v, d.opt, tensor.Float32)
				if c == 1 {
					out.Data()
					if !strings.HasPrefix(g.lastKernel, "dikkat_ileri_tc") {
						t.Fatalf("%s: tensör çekirdekli yol kullanılmadı (%s)", ad, g.lastKernel)
					}
				}
				return out, nil
			})
		}
	}
}
