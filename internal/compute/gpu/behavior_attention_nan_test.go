package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (dil tanımı 15.24): puanı NaN olan bir sorgu satırı yalnızca bakmadığı anahtarlara NaN
// sızdırmaz; hem CPU hem GPU çekirdekleri (skaler ve tensör çekirdekli) aynı gradyanı verir:
// satırın görmediği anahtarın dK ve dV'si sonludur.
func TestAttentionNaNRowDoesNotLeakToKeysItDoesNotSee(t *testing.T) {
	g := gpuBackend()
	old := g.attnPath
	defer func() { g.attnPath = old }()
	for _, path := range []string{"", "tensor"} {
		if path == "tensor" {
			if !inSim() {
				if cc, _ := g.sr.capability(); cc < 80 {
					continue
				}
			}
			g.attnPath = "tensor"
		} else {
			g.attnPath = old
		}
		for _, d := range []struct {
			B, H, Hkv, Tq, Tk, Dh, Dv int
			opt                       tensor.AttentionOptions
		}{
			{1, 1, 1, 6, 6, 8, 8, tensor.AttentionOptions{Causal: true}},
			{1, 2, 1, 40, 40, 16, 16, tensor.AttentionOptions{Causal: true, Window: 5}},
		} {
			for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
				if path == "tensor" && dt == tensor.Float32 {
					continue
				}
				ad := fmt.Sprintf("TR_attention NaN satırı TR_path=%q %v %+v", path, dt, d)
				rng := rand.New(rand.NewSource(5))
				qv := random(rng, d.B*d.Tq*d.H*d.Dh, 1)
				kv := random(rng, d.B*d.Tk*d.Hkv*d.Dh, 1)
				vv := random(rng, d.B*d.Tk*d.Hkv*d.Dv, 1)
				for c := 0; c < d.Dh; c++ {
					qv[(1*d.H)*d.Dh+c] = float32(math.NaN()) // 1. satır, 0. baş
				}
				compare(t, ad, 3e-2, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
					q := twin(qv, true, d.B, d.Tq, d.H, d.Dh)[c]
					k := twin(kv, true, d.B, d.Tk, d.Hkv, d.Dh)[c]
					v := twin(vv, true, d.B, d.Tk, d.Hkv, d.Dv)[c]
					out := tensor.Attention(q.Cast(dt), k.Cast(dt), v.Cast(dt), d.opt).Cast(tensor.Float32)
					return out, []*tensor.Tensor{q, k, v}
				})
			}
		}
	}
}
