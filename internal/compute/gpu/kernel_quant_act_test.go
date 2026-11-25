package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
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

// actShapes türün şekilleridir (M, N, K): tek satır (K = 2 560 gerçek model boyudur: satır çekirdeğinin döngülerinin
// ikinci turu), karonun içi ve kenarı, birden çok blok; q4'te K 256'nın katı olmayabilir. TR_QUANT_ACT_SHAPES=model
// Qwen3-4B'nin boylarını ekler (yalnız kartta).
func actShapes(dt tensor.DType) [][3]int {
	s := [][3]int{{1, 8, 256}, {1, 13, 768}, {2, 8, 256}, {5, 70, 512}, {17, 64, 256}, {40, 33, 768}, {65, 65, 256}, {1, 9, 2560}}
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
			for _, c := range [][3]int{{65, 65, 256}, {1, 13, 2560}} {
				M, N, K := c[0], c[1], c[2]
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
					sameFloatBits(t, fmt.Sprintf("%q %s M=%d SM", path, dt.Name(), M), r, runs[0])
				}
			}
		}
	})
}

// TestQuantActGemvMatchesRows tek satırlı çarpım (A2) çok satırlının aynı satırıyla bit bit aynıdır; hizasız etkinleştirme
// görünümünde ve dilimlenmiş ağırlıkta (kaydırma) da.
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
			shiftedX := append([]float32{0}, x[4*K:5*K]...)
			onCard(t, func() {
				row := tensor.NewOn(gpuDevice, shiftedX, K+1).Slice(0, 1, K+1).Reshape(1, K) // 4 bayt kayık etkinleştirme
				sameFloatBits(t, fmt.Sprintf("%q %s satır 4, hizasız", path, dt.Name()),
					tensor.MatMulActQuant(row, card.T(), tensor.Float32).Data(), all[4*N:5*N])
				plain := tensor.NewOn(gpuDevice, x[4*K:5*K], 1, K)
				sameFloatBits(t, fmt.Sprintf("%q %s satır 4, dilimlenmiş ağırlık", path, dt.Name()),
					tensor.MatMulActQuant(plain, card.Slice(0, 3, 20).T(), tensor.Float32).Data(), all[4*N+3:4*N+20])
			})
		}
	})
}

// TestQuantActEdges sıfır blok, K'nın son bloğu, ölçeği taşan küçük blok; sonlu olmayan satırda (NaN, +∞, −∞)
// sonucun NaN olması ve sonsuzun işareti CPU'nunkiyle aynıdır (NaN'ın bitleri karşılaştırılmaz; ara sonucu taşan
// satır söz dışıdır), komşu sonlu satırlar bit bit aynıdır. Satırlar birlikte (M = 7) ve tek tek (M = 1) koşar.
func TestQuantActEdges(t *testing.T) {
	skipUnderRace(t)
	rng := rand.New(rand.NewSource(14))
	withActPath(t, func(path string) {
		for _, dt := range actQuantTypes {
			M, N, K := 8, 40, 768
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
			clear(x[7*K : 8*K]) // satır 7: bütünü sıfır
			got, want := actProduct(t, x, M, host, card)
			name := fmt.Sprintf("%q %s", path, dt.Name())
			for m := range M {
				sameActRow(t, fmt.Sprintf("%s satır %d", name, m), got[m*N:(m+1)*N], want[m*N:(m+1)*N], m >= 3 && m <= 5)
				one, oneWant := actProduct(t, x[m*K:(m+1)*K], 1, host, card)
				sameActRow(t, fmt.Sprintf("%s tek satır %d", name, m), one, oneWant, m >= 3 && m <= 5)
			}
			// Tek başlatmada K = 2 560: sonlu olmayan işaret turlar arasında birikir (M = 1, ilk blokta).
			K = 2560
			host, card = quantWeights(t, rng, dt, 9, K)
			for i, bad := range []float32{float32(math.Inf(-1)), float32(math.Inf(1)), float32(math.NaN())} {
				y := random(rng, K, 1)
				y[7] = bad
				one, oneWant := actProduct(t, y, 1, host, card)
				sameActRow(t, fmt.Sprintf("%s K = 2560, ilk blokta %v (%d)", name, bad, i), one, oneWant, true)
			}
		}
	})
}

// sameActRow çarpımın bir satırını CPU'nunkiyle karşılaştırır: bit bit, sonlu olmayan satırda NaN olma ve değer.
func sameActRow(t *testing.T, name string, got, want []float32, nonFinite bool) {
	t.Helper()
	if !nonFinite {
		sameFloatBits(t, name, got, want)
		return
	}
	for n := range got {
		gn, wn := math.IsNaN(float64(got[n])), math.IsNaN(float64(want[n]))
		if gn != wn || (!gn && got[n] != want[n]) {
			t.Fatalf("%s sütun %d: %v, CPU %v", name, n, got[n], want[n])
		}
	}
}

// TestQuantActOverwritesOutput çarpım çıktıyı ezer (tensor.ActQuantGemmer): kirli tamponla tek ve çok satırda, boş
// toplamda (K = 0) da. Çıktının 16 bitlik biçimi de sınanır.
func TestQuantActOverwritesOutput(t *testing.T) {
	be := gpuBackend()
	rng := rand.New(rand.NewSource(16))
	for _, dt := range actQuantTypes {
		for _, c := range [][3]int{{1, 5, 256}, {2, 5, 256}, {1, 3, 0}, {2, 3, 0}} {
			M, N, K := c[0], c[1], c[2]
			elems, _ := dt.QuantBlock()
			w := be.FromRaw(quantBlockBytes(rng, dt, N*K/elems), dt)
			a := be.FromHost(random(rng, M*K, 1), tensor.Float32)
			av := tensor.View{S: a, Shape: []int{M, K}, Strides: []int{K, 1}}
			bv := tensor.View{S: w, Shape: []int{K, N}, Strides: []int{1, K}}
			for _, ct := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
				var runs [][]float32
				for _, dirt := range []float32{0, 7, float32(math.NaN())} {
					out := be.Alloc(M*N, ct)
					be.Fill(out, dirt)
					be.GemmActQuant(M, N, K, av, bv, out)
					runs = append(runs, be.ToHost(tensor.View{S: out, Shape: []int{M * N}, Strides: []int{1}}))
				}
				name := fmt.Sprintf("%s %s %dx%dx%d", dt.Name(), ct.Name(), M, N, K)
				sameFloatBits(t, name+" kirli 7", runs[1], runs[0])
				sameFloatBits(t, name+" kirli NaN", runs[2], runs[0])
				if K == 0 {
					sameFloatBits(t, name+" boş toplam", runs[0], make([]float32, M*N))
				}
			}
		}
	}
}

// TestQuantActGemvOneLaunch tek satırlı çarpım kartta tek iştir: nicemleme, bayrak ve sıfırlama için ayrı iş yoktur
// (bu işler konağın payıydı; kart profili, 2026-11-25).
func TestQuantActGemvOneLaunch(t *testing.T) {
	rng := rand.New(rand.NewSource(18))
	g := gpuBackend()
	EnableProfile(true)
	defer EnableProfile(false)
	count := func(f func()) int64 {
		g.enter()
		g.equalize()
		before := g.profiler.callN
		g.exit()
		f()
		g.enter()
		defer g.exit()
		g.equalize()
		return g.profiler.callN - before
	}
	for _, dt := range actQuantTypes {
		_, card := quantWeights(t, rng, dt, 9, 256)
		x := twin(random(rng, 256, 1), false, 1, 256)[1]
		onCard(t, func() {
			var out *tensor.Tensor
			if n := count(func() { out = tensor.MatMulActQuant(x, card.T(), tensor.Float32) }); n != 1 {
				t.Errorf("%s: tek satırlı çarpım %d iş, beklenen 1", dt.Name(), n)
			}
			out.Drop()
		})
	}
}

// TestQuantActGemvLargeK tek başlatmalı çekirdeğin sınırı: paylaşımlı belleğe sığan en büyük K tek başlatmayla, bir
// blok büyüğü karolu çekirdekle koşar; ikisi de CPU'nun bitleridir. Benzeticide karo, sınırı küçülterek küçük K'da
// sınanır (karonun 43 bin öğesi orada dakikalar sürer), büyük K yalnız sınırın hesabıyla.
func TestQuantActGemvLargeK(t *testing.T) {
	skipUnderRace(t)
	defer func(old int) { quantActGemvLimit, quantActPath = old, "" }(quantActGemvLimit)
	rng := rand.New(rand.NewSource(17))
	quantActPath = "mma" // dp4a'lı eş (benzeticide ucuz); M = 1'de mma.sync seçilmez
	g := gpuBackend()
	for _, dt := range actQuantTypes {
		elems, _ := dt.QuantBlock()
		fits := 43008
		if dt == tensor.Q4 {
			fits = 43520
		}
		if quantActGemvShared(dt, fits) > staticSharedMemLimit || quantActGemvShared(dt, fits+elems) <= staticSharedMemLimit {
			t.Fatalf("%s: sığan en büyük K %d değil", dt.Name(), fits)
		}
		check := func(K int, want string) {
			host, card := quantWeights(t, rng, dt, 1, K)
			x := twin(random(rng, K, 1), false, 1, K)
			cpu := tensor.MatMulActQuant(x[0], host.T(), tensor.Float32).Data()
			tile := fmt.Sprintf("nicem_etk_karo:%s:true", dt)
			delete(g.kernels, tile)
			var got *tensor.Tensor
			onCard(t, func() { got = tensor.MatMulActQuant(x[1], card.T(), tensor.Float32) })
			if _, ok := g.kernels[tile]; !strings.HasPrefix(g.lastKernel, want) || ok != (want == "nicem_etk_sonsuz_") {
				t.Errorf("%s K=%d: son çekirdek %s, beklenen %s…; karo %v", dt.Name(), K, g.lastKernel, want, ok)
			}
			sameFloatBits(t, fmt.Sprintf("%s K=%d", dt.Name(), K), got.Data(), cpu)
		}
		check(fits, "nicem_etk_gemv_")
		if !inSim() {
			check(fits+elems, "nicem_etk_sonsuz_")
			continue
		}
		quantActGemvLimit = quantActGemvShared(dt, 768)
		check(768, "nicem_etk_gemv_")
		check(768+elems, "nicem_etk_sonsuz_")
		quantActGemvLimit = staticSharedMemLimit
	}
}

// TestQuantActSpeed model boyunda yolların süresini yazar (gerçek kartta; benzeticide atlanır). Ölçü değildir. Süre
// yalnız çarpımındır: sonuç ana belleğe kopyalanmaz, çarpımlar art arda koşar, sonda eşitlenir; konak payı sıraya
// koymanın süresidir.
func TestQuantActSpeed(t *testing.T) {
	if inSim() {
		t.Skip("benzetici")
	}
	rng := rand.New(rand.NewSource(15))
	g := gpuBackend()
	for _, dt := range actQuantTypes {
		for _, c := range [][3]int{{512, 9728, 2560}, {512, 2560, 9728}, {40, 9728, 2560}, {1, 9728, 2560}, {1, 2560, 9728}, {1, 1024, 2560}} {
			M, N, K := c[0], c[1], c[2]
			reps := 200 / min(M, 20)
			_, card := quantWeights(t, rng, dt, N, K)
			x := twin(random(rng, M*K, 1), false, M, K)[1]
			run := func(f func() *tensor.Tensor) (total, host time.Duration) {
				f().Drop()
				g.equalize()
				t0 := time.Now()
				for range reps {
					f().Drop()
				}
				host = time.Since(t0) / time.Duration(reps)
				g.equalize()
				return time.Since(t0) / time.Duration(reps), host
			}
			ops := 2 * float64(M) * float64(N) * float64(K)
			show := func(name string, total, host time.Duration) {
				t.Logf("%s M=%d N=%d K=%d %s: %v (konak %v, %.1f T işlem/sn)", dt.Name(), M, N, K, name, total, host, ops/total.Seconds()/1e12)
			}
			onCard(t, func() {
				total, host := run(func() *tensor.Tensor { return tensor.MatMul(x, card.T()) })
				show("float", total, host)
			})
			withActPath(t, func(path string) {
				onCard(t, func() {
					total, host := run(func() *tensor.Tensor { return tensor.MatMulActQuant(x, card.T(), tensor.Float32) })
					show(fmt.Sprintf("int8 %q", path), total, host)
				})
			})
		}
	}
}
