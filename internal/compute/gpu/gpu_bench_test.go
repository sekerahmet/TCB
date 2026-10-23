package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestMicroBench GPU mikro kıyasları (yalnızca gerçek GPU'da ve TR_GPU_MICRO=1 iken): kıyas
// modelinin (B32 T256 D512 H8 FF2048 V40000) işlemleri tek tek ölçülür. Her satır
//
//	MIKRO|işlem|yol|ms|değer|birim
//
// biçimindedir; tools/gpu/micro_compare.py aynı biçimi verir, tools/gpu/micro_table.py ikisini
// karşılaştırma tablosuna çevirir. Süreler çekirdek süresidir (iki olay arası, ısınmadan
// sonra n tekrarın ortalaması). Matris çarpımı iki yolla ölçülür: kitaplık (varsa) ve kendi
// çekirdeğimiz.
func TestMicroBench(t *testing.T) {
	if os.Getenv("TR_GPU_MICRO") != "1" || inSim() {
		t.Skip("yalnızca gerçek GPU'da ve TR_GPU_MICRO=1 ile")
	}
	g := gpuBackend()
	cs, _ := g.sr.(*cudaDriver)
	peak, band := 0.0, 1800.0
	if cs != nil {
		peak = cs.peakGFLOPS()
	}
	if v, err := strconv.ParseFloat(os.Getenv("TR_GPU_MICRO_BAND"), 64); err == nil && v > 0 {
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
	microGemv(g, band, divide)
	microAttn(g, divide)
	microCE(g, divide)
	microAdamW(g, divide)
	microElem(g, band, divide)
	microNorm(g, band, divide)
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

// microGemv tek satırlık çarpımları (M = 1: çıkarımda bir parça) SmolLM2-135M ve Qwen3-0.6B
// boylarında ölçer: gemv çekirdeği (otomatik ve elle verilen ayarlar), kitaplık ve döşemeli
// çekirdek. Değer okunan ağırlık bandıdır (GB/s); konak satırı n çağrının sıraya konma
// süresidir (çağrı başına µs): GPU süresine yakınsa çağrı konakla sınırlıdır.
func microGemv(g *backend, band float64, divide int) {
	rng := rand.New(rand.NewSource(4))
	type shape struct {
		ad   string
		N, K int
		tb   bool
	}
	list := []shape{
		{"smol q/o", 576, 576, false}, {"smol k/v", 192, 576, false}, {"smol kapı", 1536, 576, false},
		{"smol aşağı", 576, 1536, false}, {"smol logit", 49152, 576, true},
		{"qwen q", 2048, 1024, false}, {"qwen kapı", 3072, 1024, false}, {"qwen aşağı", 1024, 3072, false},
		{"qwen logit", 151936, 1024, true},
	}
	lt, off := g.lt, g.gemvOff
	defer func() { g.lt, g.gemvOff, g.gemvConst = lt, off, nil }()
	for _, c := range list {
		N, K := max(c.N/divide, 1), max(c.K/divide, 1)
		a := randomStorage(g, rng, K, 1)
		b := randomStorage(g, rng, K*N, 1)
		out := g.AllocUninit(N, tensor.Float32)
		av := tensor.View{S: a, Shape: []int{K}, Strides: []int{1}}
		bv := tensor.View{S: b, Shape: []int{K * N}, Strides: []int{1}}
		f := func() { g.GemmSet(1, 1, N, K, av, false, bv, c.tb, out) }
		gb := 4 * float64(N) * float64(K) / 1e9
		ad := fmt.Sprintf("gemv %s %dx%d", c.ad, K, N)
		run := func(path string) {
			n := 200 / divide
			ms := microMeasure(g, n, f)
			begin := time.Now()
			for i := 0; i < n; i++ {
				f()
			}
			host := time.Since(begin).Seconds() * 1e6 / float64(n)
			microWrite(ad, path, ms, gb/(ms/1e3), fmt.Sprintf("GB/s (bant %%%.0f; konak %.1f µs/çağrı)", 100*gb/(ms/1e3)/band, host))
		}
		g.gemvOff, g.lt = false, lt
		run("gemv")
		if !c.tb {
			for _, cfg := range []gemvConfig{{2, 256}, {4, 256}, {8, 256}, {2, 512}, {4, 512}, {8, 512}, {4, 128}} {
				g.gemvConst = &cfg
				run(fmt.Sprintf("gemv L%d T%d", cfg.L, cfg.T))
			}
			g.gemvConst = nil
		}
		g.gemvOff = true
		if lt != nil {
			run("cublaslt")
		}
		g.lt = nil
		run("kendi")
		for _, s := range []tensor.Storage{a, b, out} {
			g.Release(s)
		}
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
	// İki yol da ölçülür (TR_GPU_ATTENTION ile seçilen): birleşik çekirdek ve matris çarpımı yolu.
	old := g.attnPath
	for _, y := range []struct{ path, ek, ad string }{{"birlesik", "", "birleşik"}, {"carpim", " (matris yolu)", "kendi"}} {
		g.attnPath = y.path
		p.SeqMajor = y.path != "carpim" // birleşik çekirdekler [B, T, H, D], matris yolu bakış önde okur
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

// microNorm hedef modelin (tools/gpu/rgn512.tr) RMS normlarıdır, bf16, ileri ve geri: blok
// normu [T, 512]; q_norm [T, 8, 64] (bitişik); k_norm [T, 4, 64] (K ile V tek çarpımdan
// gelir ve bölünür: satırlar bitişik, token adımı 512). T bir mikro toplu iştir (32 × 1024).
// Karşılığı tools/gpu/micro_compare.py normlar(). Bant: okunan ve yazılan bayt / süre.
func microNorm(g *backend, band float64, divide int) {
	rng := rand.New(rand.NewSource(7))
	T := 32768 / divide
	type normCase struct {
		ad             string
		shape, strides []int
		n              int // girdinin depolamasının eleman sayısı
	}
	cases := []normCase{
		{fmt.Sprintf("norm blok %d×512", T), []int{T, 512}, []int{512, 1}, T * 512},
		{fmt.Sprintf("norm q %d×8×64", T), []int{T, 8, 64}, []int{512, 64, 1}, T * 512},
		{fmt.Sprintf("norm k adımlı %d×4×64", T), []int{T, 4, 64}, []int{512, 64, 1}, T * 512},
	}
	for _, c := range cases {
		D := c.shape[len(c.shape)-1]
		rows := numel(c.shape) / D
		x := g.FromHost(random(rng, c.n, 1), tensor.BFloat16)
		gamma := g.FromHost(random(rng, D, 1), tensor.Float32)
		gs := g.FromHost(random(rng, rows*D, 1), tensor.BFloat16)
		out := g.AllocUninit(rows*D, tensor.BFloat16)
		gx := g.AllocUninit(rows*D, tensor.BFloat16)
		ggamma := g.AllocUninit(D, tensor.Float32)
		xv := tensor.View{S: x, Shape: c.shape, Strides: c.strides}
		gv := tensor.View{S: gamma, Shape: []int{D}, Strides: []int{1}}
		var rinv tensor.Storage
		e := rows * D
		ms := microMeasure(g, 10, func() {
			if rinv != nil {
				g.Release(rinv)
			}
			rinv = g.RMSNorm(out, xv, &gv, D, 1e-6)
		})
		microWrite(c.ad+" ileri", "kendi", ms, float64(4*e)/1e9/(ms/1e3), fmt.Sprintf("GB/s (%%%.0f)", 100*float64(4*e)/1e9/(ms/1e3)/band))
		// Taze gx (ilk gradyan; üzerine yazılır, NormGradSetter) ve ekleyerek (gx okunur).
		ms = microMeasure(g, 10, func() { g.RMSNormGradSet(gx, ggamma, gs, xv, &gv, rinv, D) })
		microWrite(c.ad+" geri", "kendi", ms, float64(6*e)/1e9/(ms/1e3), fmt.Sprintf("GB/s (%%%.0f)", 100*float64(6*e)/1e9/(ms/1e3)/band))
		ms = microMeasure(g, 10, func() { g.RMSNormGrad(gx, ggamma, gs, xv, &gv, rinv, D) })
		microWrite(c.ad+" geri ekleyerek", "kendi", ms, float64(8*e)/1e9/(ms/1e3), fmt.Sprintf("GB/s (%%%.0f)", 100*float64(8*e)/1e9/(ms/1e3)/band))
		for _, s := range []tensor.Storage{x, gamma, gs, out, gx, ggamma, rinv} {
			g.Release(s)
		}
	}
}
