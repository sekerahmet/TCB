package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Tek satırlık çarpım (M = 1) gemv çekirdeğine gider ve float64 başvuruyla aynıdır: iki B
// düzeni, ekleme ve üzerine yazma, vektörel (N ya da K 4'ün katı, hizalı) ve tekli okuma
// (4'ün katı olmayan boy ya da hizasız görünüm), son blokta eksik sütunlar, dörtlü açılımın
// kalan satırları, toplu çarpım ve elle verilen her ayar (L, T).
func TestGemvMatchesReference(t *testing.T) {
	g := gpuBackend()
	rng := rand.New(rand.NewSource(31))
	type shape struct{ batch, N, K, offset int }
	shapes := []shape{{1, 64, 48, 0}, {1, 70, 37, 0}, {2, 36, 129, 0}, {1, 8, 5, 0}, {3, 100, 64, 0}, {1, 64, 48, 1}}
	configs := []*gemvConfig{nil, {L: 2, T: 32}, {L: 8, T: 64}, {L: 32, T: 128}, {L: 1, T: 32}}
	if !inSim() {
		shapes = append(shapes, shape{1, 1536, 576, 0}, shape{1, 576, 1536, 0}, shape{2, 1000, 333, 0})
		configs = append(configs, &gemvConfig{L: 4, T: 1024})
	}
	defer func() { g.gemvConst = nil }()
	for _, s := range shapes {
		for _, transB := range []bool{false, true} {
			for _, over := range []bool{false, true} {
				for _, cfg := range configs {
					if transB && cfg != nil {
						continue // devrik çekirdeğin ayarı yok
					}
					g.gemvConst = cfg
					ad := fmt.Sprintf("b=%d N=%d K=%d ofset=%d devrik=%v üzerine=%v ayar=%+v", s.batch, s.N, s.K, s.offset, transB, over, cfg)
					gemvCase(t, g, rng, ad, s.batch, s.N, s.K, s.offset, transB, over)
				}
			}
		}
	}
}

func gemvCase(t *testing.T, g *backend, rng *rand.Rand, ad string, batch, N, K, offset int, transB, over bool) {
	t.Helper()
	av := random(rng, offset+batch*K, 1)
	bv := random(rng, offset+batch*K*N, 1)
	cv := random(rng, batch*N, 1)
	want := make([]float32, batch*N)
	for z := 0; z < batch; z++ {
		for n := 0; n < N; n++ {
			s := 0.0
			if !over {
				s = float64(cv[z*N+n])
			}
			for k := 0; k < K; k++ {
				bk := bv[offset+z*K*N+k*N+n]
				if transB {
					bk = bv[offset+z*N*K+n*K+k]
				}
				s += float64(av[offset+z*K+k]) * float64(bk)
			}
			want[z*N+n] = float32(s)
		}
	}
	as, bs, cs := g.FromHost(av, tensor.Float32), g.FromHost(bv, tensor.Float32), g.FromHost(cv, tensor.Float32)
	defer func() {
		for _, s := range []tensor.Storage{as, bs, cs} {
			g.Release(s)
		}
	}()
	a := tensor.View{S: as, Offset: offset, Shape: []int{batch * K}, Strides: []int{1}}
	b := tensor.View{S: bs, Offset: offset, Shape: []int{batch * K * N}, Strides: []int{1}}
	if over {
		g.GemmSet(batch, 1, N, K, a, false, b, transB, cs)
	} else {
		g.Gemm(batch, 1, N, K, a, false, b, transB, cs)
	}
	if !strings.HasPrefix(g.lastKernel, "gemv_") {
		t.Fatalf("%s: tek satırlık çarpım gemv çekirdeğine gitmedi (son çekirdek %s)", ad, g.lastKernel)
	}
	checkDiff(t, ad, fromDevice(g, cs), want, 1e-5*math.Sqrt(float64(K)))
}

// Doğrusal katmanın ileri ve geri yönü (dX = dY @ Wᵀ devrik gemv'dir) CPU ile aynıdır.
func TestGemvLinearGrad(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	K, N := 40, 52
	xv, wv := random(rng, K, 1), random(rng, K*N, 1)
	compare(t, "x @ W (M = 1)", 2e-5, func(c int) (*tensor.Tensor, []*tensor.Tensor) {
		x, w := twin(xv, true, 1, K)[c], twin(wv, true, K, N)[c]
		return tensor.MatMul(x, w), []*tensor.Tensor{x, w}
	})
}

// TR_GPU_GEMV=0 karşılığı (gemvOff) M = 1 çarpımını döşemeli yola bırakır.
func TestGemvOff(t *testing.T) {
	g := gpuBackend()
	g.gemvOff = true
	defer func() { g.gemvOff = false }()
	if inSim() {
		g.gemmTile(&gemmConfig{1, 1, 1, 1, 1})
		defer g.gemmTile(nil)
	}
	gemvOffCase(t, g)
}

func gemvOffCase(t *testing.T, g *backend) {
	t.Helper()
	as := g.FromHost([]float32{1, 2}, tensor.Float32)
	bs := g.FromHost([]float32{3, 4, 5, 6}, tensor.Float32)
	cs := g.FromHost([]float32{0, 0}, tensor.Float32)
	g.GemmSet(1, 1, 2, 2, tensor.View{S: as, Shape: []int{2}, Strides: []int{1}}, false,
		tensor.View{S: bs, Shape: []int{4}, Strides: []int{1}}, false, cs)
	if strings.HasPrefix(g.lastKernel, "gemv_") {
		t.Fatalf("gemv kapalıyken gemv çekirdeği çalıştı")
	}
	checkDiff(t, "gemv kapalı", fromDevice(g, cs), []float32{13, 16}, 1e-6)
}
