package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// splitCase dikkatin boyları: toplu, sorgu ve anahtar bakışı, sorgu, anahtar, bakış boyu.
type splitCase struct {
	B, H, Hkv, Tq, Tk, D int
	opt                  tensor.AttentionOptions
}

func (d splitCase) String() string {
	return fmt.Sprintf("toplu %d H %d/%d Tq %d Tk %d D %d nedensel %v pencere %d", d.B, d.H, d.Hkv, d.Tq, d.Tk, d.D, d.opt.Causal, d.opt.Window)
}

// withAttnPath f'yi dikkatin yolunu zorlayarak koşar.
func withAttnPath(path string, f func()) {
	g := gpuBackend()
	old := g.attnPath
	g.attnPath = path
	defer func() { g.attnPath = old }()
	f()
}

// splitAttention kartta dikkat; yol "bolme" ise bölünmüş çekirdeğin koştuğunu denetler.
func splitAttention(t *testing.T, path string, q, k, v *tensor.Tensor, opt tensor.AttentionOptions) *tensor.Tensor {
	t.Helper()
	g := gpuBackend()
	var out *tensor.Tensor
	withAttnPath(path, func() { out = tensor.Attention(q, k, v, opt) })
	if split := strings.HasPrefix(g.lastKernel, "dikkat_bolme"); split != (path == "bolme") {
		t.Fatalf("yol %q: son çekirdek %s", path, g.lastKernel)
	}
	return out
}

// splitInputs rastgele q, k, v değerleri (q'nun ölçeği ayrı).
func splitInputs(rng *rand.Rand, d splitCase, qScale, scale float64) (q, k, v []float32) {
	return random(rng, d.B*d.H*d.Tq*d.D, qScale), random(rng, d.B*d.Hkv*d.Tk*d.D, scale), random(rng, d.B*d.Hkv*d.Tk*d.D, scale)
}

// splitTensors c = 0 CPU, 1 kart.
func splitTensors(qv, kv, vv []float32, grad bool, d splitCase, c int) (q, k, v *tensor.Tensor) {
	return twin(qv, grad, d.B, d.Tq, d.H, d.D)[c], twin(kv, grad, d.B, d.Tk, d.Hkv, d.D)[c], twin(vv, grad, d.B, d.Tk, d.Hkv, d.D)[c]
}

// splitCases bölünmüş yolun şekilleri: R = 4, 8, 16; tek ve çok parça; G = 1, 2, 4, 7; D'nin dört boyu; toplu 1 ve 2.
func splitCases() []splitCase {
	causal, all := tensor.AttentionOptions{Causal: true}, tensor.AttentionOptions{}
	return []splitCase{
		{1, 4, 1, 1, 40, 64, all}, {1, 8, 2, 1, 80, 32, causal}, {1, 4, 1, 1, 1100, 32, causal},
		{1, 2, 1, 2, 70, 96, causal}, {1, 2, 1, 4, 40, 128, causal}, {2, 2, 1, 4, 270, 32, causal},
		{1, 7, 1, 2, 40, 64, all}, {1, 7, 1, 1, 270, 32, all}, {1, 2, 2, 8, 40, 64, tensor.AttentionOptions{Causal: true, Window: 9}},
		{1, 2, 1, 4, 270, 64, causal}, {1, 2, 1, 4, 270, 96, causal},
		{1, 2, 1, 16, 270, 32, causal}, {1, 2, 1, 40, 45, 32, causal}, {1, 2, 2, 1, 1, 32, all}, {1, 2, 1, 6, 3, 32, causal},
	}
}

// TestAttentionSplitMatchesFused bölünmüş yol CPU ve birleşik yolla 1e-5 içinde aynıdır (|a − b| ≤ 1e-5·(1 + |b|)), geri
// yönü de; kartta model boyları da koşar. Bitler aynı değildir: toplama sırası başkadır (15.22a aynı çekirdeğin
// sözüdür).
func TestAttentionSplitMatchesFused(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(31))
	cases := splitCases()
	if !inSim() {
		causal := tensor.AttentionOptions{Causal: true}
		cases = append(cases, splitCase{1, 32, 8, 1, 1100, 128, causal}, splitCase{1, 32, 8, 1, 4140, 128, causal}, splitCase{1, 32, 8, 40, 4140, 128, causal})
	}
	for _, d := range cases {
		grad := d.Tk <= 80
		qv, kv, vv := splitInputs(rng, d, 1, 1)
		var got []float32
		compare(t, fmt.Sprintf("%v bölünmüş ile CPU", d), 1e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q, k, v := splitTensors(qv, kv, vv, grad, d, c)
			var leaves []*tensor.Tensor
			if grad {
				leaves = []*tensor.Tensor{q, k, v}
			}
			if c == 0 {
				return tensor.Attention(q, k, v, d.opt), leaves
			}
			out := splitAttention(t, "bolme", q, k, v, d.opt)
			got = append([]float32{}, out.Data()...)
			return out, leaves
		})
		q, k, v := splitTensors(qv, kv, vv, false, d, 1)
		checkDiff(t, fmt.Sprintf("%v bölünmüş ile birleşik", d), got, splitAttention(t, "birlesik", q, k, v, d.opt).Data(), 1e-5)
	}
}

// TestAttentionSplitBackward bölünmüş ileri yolun geri yönü (lse'den) birleşik yolunkiyle 1e-5 içinde aynıdır;
// türev izlenen ve izlenmeyen çağrının ileri bitleri aynıdır (seçim türev kaydından bağımsızdır).
func TestAttentionSplitBackward(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	for _, d := range []splitCase{
		{1, 4, 2, 1, 40, 64, tensor.AttentionOptions{Causal: true}}, {1, 2, 1, 4, 270, 32, tensor.AttentionOptions{Causal: true}},
		{1, 2, 2, 8, 40, 96, tensor.AttentionOptions{Causal: true, Window: 11}},
	} {
		qv, kv, vv := splitInputs(rng, d, 1, 1)
		var grads [2][][]float32
		for i, path := range []string{"bolme", "birlesik"} {
			q, k, v := splitTensors(qv, kv, vv, true, d, 1)
			out := splitAttention(t, path, q, k, v, d.opt)
			bits := bitsOf(out)
			tensor.Mul(out, out).SumAll().Backward()
			for _, x := range []*tensor.Tensor{q, k, v} {
				grads[i] = append(grads[i], append([]float32{}, x.Grad().Data()...))
			}
			if path == "bolme" {
				q2, k2, v2 := splitTensors(qv, kv, vv, false, d, 1)
				sameBitsAt(t, fmt.Sprintf("%v türevsiz ile türevli", d), bitsOf(splitAttention(t, path, q2, k2, v2, d.opt)), bits)
			}
		}
		for i, name := range []string{"dq", "dk", "dv"} {
			checkDiff(t, fmt.Sprintf("%v %s", d, name), grads[0][i], grads[1][i], 1e-5)
		}
	}
}

// TestAttentionSplitOneLaunch tek parçada bölünmüş dikkat kartta tek iştir, çok parçada ikidir.
func TestAttentionSplitOneLaunch(t *testing.T) {
	rng := rand.New(rand.NewSource(33))
	g := gpuBackend()
	EnableProfile(true)
	defer EnableProfile(false)
	for _, c := range []struct {
		d    splitCase
		want int64
	}{
		{splitCase{1, 4, 2, 1, 40, 64, tensor.AttentionOptions{}}, 1}, {splitCase{1, 4, 1, 1, 600, 32, tensor.AttentionOptions{}}, 1},
		{splitCase{1, 4, 1, 4, 300, 32, tensor.AttentionOptions{}}, 2},
	} {
		qv, kv, vv := splitInputs(rng, c.d, 1, 1)
		q, k, v := splitTensors(qv, kv, vv, false, c.d, 1)
		g.enter()
		g.equalize()
		before := g.profiler.callN
		g.exit()
		out := splitAttention(t, "bolme", q, k, v, c.d.opt)
		g.enter()
		g.equalize()
		if n := g.profiler.callN - before; n != c.want {
			t.Errorf("%v: %d iş, beklenen %d", c.d, n, c.want)
		}
		g.exit()
		out.Drop()
	}
}

// referenceError float64 başvurusuna göre en büyük |a − b| / (1 + |b|).
func referenceError(d splitCase, qv, kv, vv, out []float32) float64 {
	G := d.H / d.Hkv
	worst := 0.0
	for i := 0; i < d.Tq; i++ {
		for h := 0; h < d.H; h++ {
			x, mx := make([]float64, d.Tk), math.Inf(-1)
			for j := range x {
				for e := 0; e < d.D; e++ {
					x[j] += float64(qv[(i*d.H+h)*d.D+e]) * float64(kv[(j*d.Hkv+h/G)*d.D+e])
				}
				x[j] /= math.Sqrt(float64(d.D))
				mx = math.Max(mx, x[j])
			}
			total := 0.0
			for j := range x {
				x[j] = math.Exp(x[j] - mx)
				total += x[j]
			}
			for e := 0; e < d.D; e++ {
				y := 0.0
				for j := range x {
					y += x[j] * float64(vv[(j*d.Hkv+h/G)*d.D+e])
				}
				y /= total
				worst = math.Max(worst, math.Abs(float64(out[(i*d.H+h)*d.D+e])-y)/(1+math.Abs(y)))
			}
		}
	}
	return worst
}

// TestAttentionSplitMatchesReference bölünmüş yolun float64 başvurusuna hatası birleşik yolunkinin 1,5 katını (tabanı
// 1e-6) ve 1e-4'ü geçmez: girdi ölçeği 1 ve 3, büyük puanlı q × 20. Çok parçalı toplam daha kısadır, hata büyümez.
func TestAttentionSplitMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(34))
	for _, d := range []splitCase{
		{1, 4, 1, 1, 40, 64, tensor.AttentionOptions{}}, {1, 2, 1, 1, 640, 32, tensor.AttentionOptions{}},
		{1, 4, 2, 3, 70, 64, tensor.AttentionOptions{}}, {1, 2, 1, 10, 270, 32, tensor.AttentionOptions{}},
	} {
		for _, s := range []struct{ q, kv float64 }{{1, 1}, {3, 3}, {20, 1}} {
			qv, kv, vv := splitInputs(rng, d, s.q, s.kv)
			q, k, v := splitTensors(qv, kv, vv, false, d, 1)
			split := referenceError(d, qv, kv, vv, splitAttention(t, "bolme", q, k, v, d.opt).Data())
			fused := referenceError(d, qv, kv, vv, splitAttention(t, "birlesik", q, k, v, d.opt).Data())
			if split > math.Max(1.5*fused, 1e-6) || split > 1e-4 {
				t.Errorf("%v ölçek %v: bölünmüş %.2e, birleşik %.2e", d, s, split, fused)
			}
		}
	}
}

// visibleKeys satır i'nin gördüğü anahtar aralığı (tensor.AttentionParams.Limits'in kuralı).
func visibleKeys(d splitCase, i int) (lo, hi int) {
	c := i + d.Tk - d.Tq
	lo, hi = 0, d.Tk
	if d.opt.Causal {
		hi = min(hi, c+1)
	}
	if d.opt.Window > 0 {
		lo = max(lo, c+1-d.opt.Window)
		if !d.opt.Causal {
			hi = min(hi, c+d.opt.Window)
		}
	}
	return lo, max(hi, lo)
}

// TestAttentionSplitHiddenNaN bakılmayan anahtarın K ya da V'sindeki NaN satıra sızmaz; bakan satıra geçer (15.24).
// Beklenen kuraldır, CPU değil (CPU'nun karoları sızdırır, S-304).
func TestAttentionSplitHiddenNaN(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(35))
	nan := float32(math.NaN())
	for _, d := range []splitCase{
		{1, 4, 2, 4, 6, 32, tensor.AttentionOptions{Causal: true}}, {1, 4, 2, 4, 70, 64, tensor.AttentionOptions{Causal: true}},
		{1, 4, 2, 4, 40, 32, tensor.AttentionOptions{Causal: true, Window: 4}}, {1, 2, 1, 6, 45, 128, tensor.AttentionOptions{Window: 3}},
		{1, 4, 2, 1, 70, 96, tensor.AttentionOptions{Causal: true, Window: 9}}, {1, 4, 1, 4, 270, 32, tensor.AttentionOptions{Causal: true, Window: 60}},
		{1, 2, 1, 9, 5, 32, tensor.AttentionOptions{Causal: true}},
	} {
		for _, inV := range []bool{false, true} {
			for at := 0; at < d.Tk; at += 1 + d.Tk/24 {
				qv, kv, vv := splitInputs(rng, d, 1, 1)
				x := kv
				if inV {
					x = vv
				}
				x[at*d.Hkv*d.D+3] = nan // 0. anahtar bakışı
				q, k, v := splitTensors(qv, kv, vv, false, d, 1)
				out := splitAttention(t, "bolme", q, k, v, d.opt).Data()
				for i := 0; i < d.Tq; i++ {
					lo, hi := visibleKeys(d, i)
					for h := 0; h < d.H; h++ {
						got := false
						for _, y := range out[(i*d.H+h)*d.D : (i*d.H+h+1)*d.D] {
							got = got || math.IsNaN(float64(y))
						}
						if want := at >= lo && at < hi && h < d.H/d.Hkv; got != want {
							t.Fatalf("%v V %v, NaN anahtar %d, satır %d bakış %d: NaN %v, beklenen %v", d, inV, at, i, h, got, want)
						}
					}
				}
			}
		}
	}
}

// TestAttentionSplitEdges kenarlar CPU ile 5e-5 içinde aynıdır: tek anahtar, warp'ın KC'sinden az anahtar, Tq > Tk (bakan
// anahtarı olmayan satır 0), pencere, puanı NaN olan satır, puanı −∞ olan satır.
func TestAttentionSplitEdges(t *testing.T) {
	rng := rand.New(rand.NewSource(36))
	for _, d := range []splitCase{
		{1, 2, 2, 1, 1, 32, tensor.AttentionOptions{}}, {1, 2, 1, 1, 7, 64, tensor.AttentionOptions{}}, {1, 2, 1, 6, 3, 32, tensor.AttentionOptions{Causal: true}},
		{1, 4, 2, 3, 50, 64, tensor.AttentionOptions{Causal: true, Window: 4}}, {2, 2, 1, 3, 50, 32, tensor.AttentionOptions{Window: 5}},
	} {
		for _, nan := range []bool{false, true} {
			qv, kv, vv := splitInputs(rng, d, 1, 1)
			if nan {
				qv[0] = float32(math.NaN())
			}
			compare(t, fmt.Sprintf("%v NaN satır %v", d, nan), 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
				q, k, v := splitTensors(qv, kv, vv, true, d, c)
				if c == 0 {
					return tensor.Attention(q, k, v, d.opt), []*tensor.Tensor{q, k, v}
				}
				return splitAttention(t, "bolme", q, k, v, d.opt), []*tensor.Tensor{q, k, v}
			})
		}
	}
	// Puanı −∞ olan satır (bütün anahtarları, ya da tek anahtar): K'nin 0. öğesi artı, q'nunki −∞; K'nin bir satırı −∞.
	for _, row := range []bool{true, false} {
		d := splitCase{1, 4, 2, 3, 50, 64, tensor.AttentionOptions{Causal: true}}
		qv, kv, vv := splitInputs(rng, d, 1, 1)
		for i := 0; i < len(kv); i += d.D {
			kv[i] = 1 + kv[i]*kv[i]
		}
		for i := 0; i < len(qv); i += d.D {
			qv[i] = 1 + qv[i]*qv[i]
		}
		if row {
			qv[d.H*d.D] = float32(math.Inf(-1)) // 1. sorgu, 0. bakış
		} else {
			kv[7*d.Hkv*d.D] = float32(math.Inf(-1)) // 7. anahtar, 0. anahtar bakışı
		}
		compare(t, fmt.Sprintf("%v −∞ satır %v", d, row), 5e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
			q, k, v := splitTensors(qv, kv, vv, false, d, c)
			if c == 0 {
				return tensor.Attention(q, k, v, d.opt), nil
			}
			return splitAttention(t, "bolme", q, k, v, d.opt), nil
		})
	}
	// Bakan anahtarı olmayan satır: O = 0.
	d := splitCase{1, 2, 1, 6, 3, 32, tensor.AttentionOptions{Causal: true}}
	qv, kv, vv := splitInputs(rng, d, 1, 1)
	q, k, v := splitTensors(qv, kv, vv, false, d, 1)
	for i, y := range splitAttention(t, "bolme", q, k, v, d.opt).Data()[:3*d.H*d.D] {
		if y != 0 {
			t.Fatalf("görünür anahtarı olmayan satır, öğe %d: %v", i, y)
		}
	}
}
