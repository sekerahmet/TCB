package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestMicroBench GPU mikro kıyasları (yalnızca gerçek GPU'da ve TR_GPU_MIKRO=1 iken): kıyas
// modelinin (B32 T256 D512 H8 FF2048 V40000) işlemleri tek tek ölçülür. Her satır
//
//	MIKRO|işlem|yol|ms|değer|birim
//
// biçimindedir; tools/gpu/mikro_kiyas.py aynı biçimi verir, tools/gpu/mikro_tablo.py ikisini
// karşılaştırma tablosuna çevirir. Süreler çekirdek süresidir (iki olay arası, ısınmadan
// sonra n tekrarın ortalaması). Matris çarpımı iki yolla ölçülür: kitaplık (varsa) ve kendi
// çekirdeğimiz.
func TestMicroBench(t *testing.T) {
	if os.Getenv("TR_GPU_MIKRO") != "1" || inSim() {
		t.Skip("yalnızca gerçek GPU'da ve TR_GPU_MIKRO=1 ile")
	}
	g := gpuBackend()
	cs, _ := g.sr.(*cudaDriver)
	peak, band := 0.0, 1800.0
	if cs != nil {
		peak = cs.peakGFLOPS()
	}
	if v, err := strconv.ParseFloat(os.Getenv("TR_GPU_MIKRO_BANT"), 64); err == nil && v > 0 {
		band = v
	}
	fmt.Printf("MIKRO|bilgi|%s|0|%.0f|tepe GFLOPS (bant %.0f GB/s)\n", g.gemmPath(), peak, band)
	microAll(g, peak, band, 1)
	for _, b := range MemorySamples() {
		fmt.Printf("MIKRO|bellek|gpu:%d|0|%.0f|MB tepe canlı (ayrılmış tepe %.0f MB)\n", b.Device,
			float64(b.PeakLive)/(1<<20), float64(b.PeakReserved)/(1<<20))
	}
}

// TestMicroBenchSmall kıyas kodunun kendisini benzetimde küçük boylarla koşturur.
func TestMicroBenchSmall(t *testing.T) {
	if !inSim() || testing.Short() {
		t.Skip("benzetim sınaması")
	}
	microAll(gpuBackend(), 0, 1800, 64)
}

// microAll bütün mikro kıyasları boylar bol'e bölünmüş olarak koşturur (1: gerçek boylar).
func microAll(g *backend, peak, band float64, divide int) {
	microGemms(g, peak, divide)
	microAttn(g, divide)
	microCE(g, divide)
	microAdamW(g, divide)
	microElem(g, band, divide)
}

// microMeasure f'yi bir kez ısındırıp n kez çalıştırır; tekrar başına GPU süresini (ms) verir.
func microMeasure(g *backend, n int, f func()) float64 {
	f()
	g.enter()
	g.equalize()
	a, _ := g.sr.createEvent()
	b, _ := g.sr.createEvent()
	g.sr.recordEvent(a)
	g.exit()
	for i := 0; i < n; i++ {
		f()
	}
	g.enter()
	defer g.exit()
	g.sr.recordEvent(b)
	g.equalize()
	ms, _ := g.sr.eventTime(a, b)
	return float64(ms) / float64(n)
}

func microWrite(op, path string, ms, value float64, unit string) {
	fmt.Printf("MIKRO|%s|%s|%.4f|%.1f|%s\n", op, path, ms, value, unit)
}

// randomStorage n elemanlı rastgele float32 GPU depolaması.
func randomStorage(g *backend, rng *rand.Rand, n int, scaleF float64) tensor.Storage {
	return g.FromHost(random(rng, n, scaleF), tensor.Float32)
}

func microGemms(g *backend, peak float64, divide int) {
	rng := rand.New(rand.NewSource(1))
	type gemm2 struct {
		ad      string
		M, N, K int
		ta, tb  bool
	}
	T, D, FF, V := 8192/divide, 512/divide, 2048/divide, 40000/divide
	list := []gemm2{
		{"çarpım qkv ileri", T, D, D, false, false}, {"çarpım ff1 ileri", T, FF, D, false, false},
		{"çarpım ff2 ileri", T, D, FF, false, false}, {"çarpım çıkış ileri", T, V, D, false, false},
		{"çarpım qkv dX", T, D, D, false, true}, {"çarpım ff1 dX", T, D, FF, false, true},
		{"çarpım çıkış dX", T, D, V, false, true}, {"çarpım qkv dW", D, D, T, true, false},
		{"çarpım ff1 dW", D, FF, T, true, false}, {"çarpım çıkış dW", V, D, T, true, false},
	}
	for _, c := range list {
		a := randomStorage(g, rng, c.M*c.K, 1)
		b := randomStorage(g, rng, c.K*c.N, 1)
		out := g.AllocUninit(c.M*c.N, tensor.Float32)
		av := tensor.View{S: a, Shape: []int{c.M * c.K}, Strides: []int{1}}
		bv := tensor.View{S: b, Shape: []int{c.K * c.N}, Strides: []int{1}}
		f := func() { g.GemmSet(1, c.M, c.N, c.K, av, c.ta, bv, c.tb, out) }
		paths := []string{"kendi"}
		if g.lt != nil {
			paths = []string{"cublaslt", "kendi"}
		}
		lt := g.lt
		for _, path := range paths {
			if path == "kendi" {
				g.lt = nil
			}
			ms := microMeasure(g, 10, f)
			gf := 2 * float64(c.M) * float64(c.N) * float64(c.K) / (ms * 1e6)
			unit := "GFLOPS"
			if peak > 0 {
				unit = fmt.Sprintf("GFLOPS (tepenin %%%.0f)", 100*gf/peak)
			}
			microWrite(c.ad, path, ms, gf, unit)
			g.lt = lt
		}
		g.Release(a)
		g.Release(b)
		g.Release(out)
	}
}

func microAttn(g *backend, divide int) {
	rng := rand.New(rand.NewSource(2))
	B, H, T, Dh := max(32/divide, 1), max(8/divide, 1), max(256/divide, 8), max(64/divide, 8)
	n := B * H * T * Dh
	q, k, v := randomStorage(g, rng, n, 1), randomStorage(g, rng, n, 1), randomStorage(g, rng, n, 1)
	sh := []int{B, H, T, Dh}
	see := func(s tensor.Storage) tensor.View { return tensor.View{S: s, Shape: sh, Strides: contiguous(sh)} }
	p := tensor.AttentionParams{Batch: B, H: H, Hkv: H, Tq: T, Tk: T, Dh: Dh, Dv: Dh, Causal: true,
		Scale: float32(1 / math.Sqrt(float64(Dh)))}
	out := g.AllocUninit(n, tensor.Float32)
	var lse tensor.Storage
	forward := func() {
		if lse != nil {
			g.Release(lse)
		}
		lse = g.Attention(out, see(q), see(k), see(v), nil, p)
	}
	// Nedensel FLOP: ileri 2 çarpım (QKᵀ, PV), yarısı görünür.
	flop := 2 * 2 * float64(B*H) * float64(T*T*Dh) / 2
	dO := randomStorage(g, rng, n, 1)
	gq, gk, gv := g.AllocUninit(n, tensor.Float32), g.AllocUninit(n, tensor.Float32), g.AllocUninit(n, tensor.Float32)
	backward := func() {
		g.AttentionGradSet(gq, gk, gv, dO, see(q), see(k), see(v), nil, out, lse, p, [3]bool{true, true, true})
	}
	// İki yol da ölçülür (TR_GPU_DIKKAT ile seçilen): birleşik çekirdek ve matris çarpımı yolu.
	old := g.attnPath
	for _, y := range []struct{ path, ek, ad string }{{"birlesik", "", "birleşik"}, {"carpim", " (matris yolu)", "kendi"}} {
		g.attnPath = y.path
		ms := microMeasure(g, 10, forward)
		microWrite("dikkat ileri"+y.ek, y.ad, ms, flop/(ms*1e6), "GFLOPS")
		ms = microMeasure(g, 10, backward)
		microWrite("dikkat geri"+y.ek, y.ad, ms, 2.5*flop/(ms*1e6), "GFLOPS")
	}
	g.attnPath = old
	for _, s := range []tensor.Storage{q, k, v, out, lse, dO, gq, gk, gv} {
		g.Release(s)
	}
}

func microCE(g *backend, divide int) {
	rng := rand.New(rand.NewSource(3))
	R, C := 8192/divide, 40000/divide
	x := randomStorage(g, rng, R*C, 2)
	target := make([]int, R)
	for i := range target {
		target[i] = rng.Intn(C)
	}
	xv := tensor.View{S: x, Shape: []int{R, C}, Strides: []int{C, 1}}
	var loss, lse tensor.Storage
	forward := func() {
		for _, s := range []tensor.Storage{loss, lse} {
			if s != nil {
				g.Release(s)
			}
		}
		loss, lse = g.CrossEntropy(xv, target, C)
	}
	ms := microMeasure(g, 10, forward)
	gb := 4 * float64(R) * float64(C) / 1e9
	microWrite("çapraz entropi ileri", "birleşik", ms, gb/(ms/1e3), "GB/s")
	gx := g.AllocUninit(R*C, tensor.Float32)
	ms = microMeasure(g, 10, func() { g.CrossEntropyGradSet(gx, loss, xv, lse, target, C) })
	microWrite("çapraz entropi geri", "birleşik", ms, 2*gb/(ms/1e3), "GB/s")
	for _, s := range []tensor.Storage{x, gx, loss, lse} {
		g.Release(s)
	}
}

func microAdamW(g *backend, divide int) {
	rng := rand.New(rand.NewSource(4))
	V, D, FF := 40000/divide, 512/divide, 2048/divide
	sizes := []int{V * D}
	for i := 0; i < 6; i++ {
		sizes = append(sizes, D, D*D, D*D, D*D, D*D, D, D*FF, D*FF, FF*D)
	}
	sizes = append(sizes, D)
	var ps, gs, ms, vs []tensor.Storage
	var bc1, bc2 []float32
	total := 0
	for _, n := range sizes {
		ps = append(ps, randomStorage(g, rng, n, 1))
		gs = append(gs, randomStorage(g, rng, n, 1))
		ms = append(ms, g.Alloc(n, tensor.Float32))
		vs = append(vs, g.Alloc(n, tensor.Float32))
		bc1, bc2 = append(bc1, 0.1), append(bc2, 0.001)
		total += n
	}
	duration := microMeasure(g, 10, func() { g.AdamWStepAll(ps, gs, ms, vs, 1e-3, 0.9, 0.999, 1e-8, 0, bc1, bc2) })
	microWrite(fmt.Sprintf("AdamW %.1f M parametre", float64(total)/1e6), "çok tensörlü", duration,
		7*4*float64(total)/1e9/(duration/1e3), "GB/s")
	for _, l := range [][]tensor.Storage{ps, gs, ms, vs} {
		for _, s := range l {
			g.Release(s)
		}
	}
}

func microElem(g *backend, band float64, divide int) {
	rng := rand.New(rand.NewSource(5))
	edge := 8192 / divide
	n := edge * edge
	a, b := randomStorage(g, rng, n, 1), randomStorage(g, rng, n, 1)
	out := g.AllocUninit(n, tensor.Float32)
	plain := tensor.View{Shape: []int{n}, Strides: []int{1}}
	av, bv := plain, plain
	av.S, bv.S = a, b
	write := func(ad string, byteN int, f func()) {
		ms := microMeasure(g, 10, f)
		gbs := float64(byteN) / 1e9 / (ms / 1e3)
		microWrite(ad, "kendi", ms, gbs, fmt.Sprintf("GB/s (%%%.0f)", 100*gbs/band))
	}
	write(fmt.Sprintf("eleman toplama %dM", n>>20), 12*n, func() { g.Binary(tensor.OpAdd, out, []int{n}, av, bv) })
	write(fmt.Sprintf("eleman üstel %dM", n>>20), 8*n, func() { g.Unary(tensor.OpExp, 0, out, av) })
	write(fmt.Sprintf("eleman silu %dM", n>>20), 8*n, func() { g.Unary(tensor.OpSilu, 0, out, av) })
	write(fmt.Sprintf("eleman sınırla %dM", n>>20), 8*n, func() { g.Clamp(out, av, -0.5, 0.5) })
	sh := []int{edge, edge}
	dev := tensor.View{S: a, Shape: sh, Strides: []int{1, edge}}
	write(fmt.Sprintf("devrik kopya %d²", edge), 8*n, func() { g.CopyStrided(tensor.View{S: out, Shape: sh, Strides: contiguous(sh)}, dev) })
	for _, s := range []tensor.Storage{a, b, out} {
		g.Release(s)
	}
}
