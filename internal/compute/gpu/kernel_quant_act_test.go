package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// actQuantTypes 15.53a'nın ağırlık türleridir.
var actQuantTypes = []tensor.DType{tensor.Q4, tensor.Q4K, tensor.Q6K}

// actPaths sınanacak yollardır (quantActPath): benzeticide hepsi; kartta yeteneğin izin verdikleri.
func actPaths() []string {
	cc, _ := gpuBackend().sr.capability()
	switch {
	case inSim() || cc >= 80:
		return []string{"", "mma", "dp4a", "nodp4a"}
	case cc >= 61:
		return []string{"", "dp4a", "nodp4a"}
	}
	return []string{""}
}

// actShapes türün şekilleridir (M, N, K): tek satır, karonun içi ve kenarı, birden çok blok; q4'te K 256'nın
// katı olmayabilir. TR_QUANT_ACT_SHAPES=model Qwen3-4B'nin boylarını ekler (yalnız kartta).
func actShapes(dt tensor.DType) [][3]int {
	s := [][3]int{{1, 8, 256}, {1, 13, 768}, {2, 8, 256}, {5, 70, 512}, {17, 64, 256}, {40, 33, 768}, {65, 65, 256}}
	if dt == tensor.Q4 {
		s = append(s, [3]int{1, 9, 32}, [3]int{3, 17, 96}, [3]int{66, 70, 288})
	}
	if raceEnabled { // ağır boylar yarış dedektöründe koşmaz (skipUnderRace'in gerekçesi); yol ve tür aynı
		return s[:3]
	}
	if os.Getenv("TR_QUANT_ACT_SHAPES") == "model" && !inSim() {
		s = append(s, [3]int{1, 4096, 2560}, [3]int{40, 1024, 2560}, [3]int{512, 2560, 4096}, [3]int{512, 9728, 2560},
			[3]int{512, 2560, 9728}, [3]int{1, 2560, 9728})
	}
	return s
}

// withActPath f'yi her yolla koşar.
func withActPath(t *testing.T, f func(path string)) {
	t.Helper()
	defer func() { quantActPath = "" }()
	for _, p := range actPaths() {
		quantActPath = p
		f(p)
	}
}

// actProduct GPU'da ve CPU'da nicem(x) · aç(w)'dir.
func actProduct(t *testing.T, x []float32, M int, host, card *tensor.Tensor) (got, want []float32) {
	t.Helper()
	K := len(x) / M
	xs := twin(x, false, M, K)
	want = tensor.MatMulActQuant(xs[0], host.T(), tensor.Float32).Data()
	onCard(t, func() { got = tensor.MatMulActQuant(xs[1], card.T(), tensor.Float32).Data() })
	return got, want
}

// TestQuantActMatchesCPUForEveryShape 15.53a'nın GPU çarpımı her yolda, türde ve şekilde CPU'nunkiyle bit bit
// aynıdır (A1).
func TestQuantActMatchesCPUForEveryShape(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	withActPath(t, func(path string) {
		for _, dt := range actQuantTypes {
			for _, c := range actShapes(dt) {
				M, N, K := c[0], c[1], c[2]
				host, card := quantWeights(t, rng, dt, N, K)
				got, want := actProduct(t, random(rng, M*K, 1), M, host, card)
				sameFloatBits(t, fmt.Sprintf("%q %s %dx%dx%d", path, dt.Name(), M, N, K), got, want)
			}
		}
	})
}

// TestQuantActIsDeterministic SM sayısı bitleri değiştirmez (15.22a): toplama sırası yalnız şekle bağlıdır.
func TestQuantActIsDeterministic(t *testing.T) {
	skipUnderRace(t)
	sd, ok := gpuBackend().sr.(*simDriver)
	if !ok {
		t.Skip("SM sayısı yalnız benzeticide değiştirilebilir")
	}
	rng := rand.New(rand.NewSource(12))
	withActPath(t, func(path string) {
		for _, dt := range actQuantTypes {
			M, N, K := 65, 65, 256
			host, card := quantWeights(t, rng, dt, N, K)
			x := random(rng, M*K, 1)
			old := sd.smCount
			var runs [][]float32
			for _, sm := range []int{1, 7} {
				sd.smCount = sm
				got, _ := actProduct(t, x, M, host, card)
				runs = append(runs, got)
			}
			sd.smCount = old
			for _, r := range runs[1:] {
				sameFloatBits(t, fmt.Sprintf("%q %s SM", path, dt.Name()), r, runs[0])
			}
		}
	})
}

// TestQuantActGemvMatchesRows tek satırlı çarpım (A2) çok satırlının aynı satırıyla bit bit aynıdır.
func TestQuantActGemvMatchesRows(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(13))
	withActPath(t, func(path string) {
		for _, dt := range actQuantTypes {
			M, N, K := 9, 70, 512
			_, card := quantWeights(t, rng, dt, N, K)
			x := random(rng, M*K, 1)
			var all, one []float32
			onCard(t, func() {
				all = tensor.MatMulActQuant(tensor.NewOn(gpuDevice, x, M, K), card.T(), tensor.Float32).Data()
				one = tensor.MatMulActQuant(tensor.NewOn(gpuDevice, x[4*K:5*K], 1, K), card.T(), tensor.Float32).Data()
			})
			sameFloatBits(t, fmt.Sprintf("%q %s satır 4", path, dt.Name()), one, all[4*N:5*N])
		}
	})
}

// TestQuantActEdges sıfır blok, K'nın son bloğu, ölçeği taşan küçük blok; sonlu olmayan satırda (NaN, +∞, −∞)
// sonucun NaN olması ve sonsuzun işareti CPU'nunkiyle aynıdır (NaN'ın bitleri karşılaştırılmaz; ara sonucu taşan
// satır söz dışıdır), komşu sonlu satırlar bit bit aynıdır.
func TestQuantActEdges(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(14))
	withActPath(t, func(path string) {
		for _, dt := range actQuantTypes {
			M, N, K := 7, 40, 768
			host, card := quantWeights(t, rng, dt, N, K)
			x := random(rng, M*K, 1)
			clear(x[0:256]) // satır 0: sıfır bloklar
			x[K+K-1] = 3    // satır 1: son bloğun son öğesi en büyük
			for i := 2 * K; i < 2*K+256; i++ {
				x[i] = 1e-38 // satır 2: 127 / amax taşar, blok sıfıra iner
			}
			x[3*K+5] = float32(math.NaN())
			x[4*K+300] = float32(math.Inf(1))
			x[5*K+17] = float32(math.Inf(-1))
			x[5*K+600] = float32(math.Inf(-1))
			got, want := actProduct(t, x, M, host, card)
			name := fmt.Sprintf("%q %s", path, dt.Name())
			for m := range M {
				g, w := got[m*N:(m+1)*N], want[m*N:(m+1)*N]
				if m < 3 || m == 6 {
					sameFloatBits(t, fmt.Sprintf("%s satır %d", name, m), g, w)
					continue
				}
				for n := range g {
					gn, wn := math.IsNaN(float64(g[n])), math.IsNaN(float64(w[n]))
					if gn != wn || (!gn && g[n] != w[n]) {
						t.Fatalf("%s satır %d sütun %d: %v, CPU %v", name, m, n, g[n], w[n])
					}
				}
			}
		}
	})
}

// TestQuantActSpeed model boyunda yolların süresini yazar (gerçek kartta; benzeticide atlanır). Ölçü değildir.
func TestQuantActSpeed(t *testing.T) {
	if inSim() {
		t.Skip("benzetici")
	}
	rng := rand.New(rand.NewSource(15))
	for _, dt := range []tensor.DType{tensor.Q4K, tensor.Q6K} {
		for _, c := range [][3]int{{512, 9728, 2560}, {512, 2560, 9728}, {40, 9728, 2560}, {1, 9728, 2560}} {
			M, N, K := c[0], c[1], c[2]
			_, card := quantWeights(t, rng, dt, N, K)
			x := twin(random(rng, M*K, 1), false, M, K)[1]
			run := func(f func() *tensor.Tensor) time.Duration {
				f().Data()
				t0 := time.Now()
				for range 10 {
					f().Data()
				}
				return time.Since(t0) / 10
			}
			ops := 2 * float64(M) * float64(N) * float64(K)
			var tf time.Duration
			onCard(t, func() { tf = run(func() *tensor.Tensor { return tensor.MatMul(x, card.T()) }) })
			t.Logf("%s M=%d N=%d K=%d: float %v (%.1f TFLOPS)", dt.Name(), M, N, K, tf, ops/tf.Seconds()/1e12)
			withActPath(t, func(path string) {
				var tq time.Duration
				onCard(t, func() {
					tq = run(func() *tensor.Tensor { return tensor.MatMulActQuant(x, card.T(), tensor.Float32) })
				})
				t.Logf("  int8 %q %v (%.1f TOPS)", path, tq, ops/tq.Seconds()/1e12)
			})
		}
	}
}
