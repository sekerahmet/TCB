package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// TestMikroKiyas GPU mikro kıyasları (yalnızca gerçek GPU'da ve TR_GPU_MIKRO=1 iken): kıyas
// modelinin (B32 T256 D512 H8 FF2048 V40000) işlemleri tek tek ölçülür. Her satır
//
//	MIKRO|işlem|yol|ms|değer|birim
//
// biçimindedir; tools/gpu/mikro_kiyas.py aynı biçimi verir, tools/gpu/mikro_tablo.py ikisini
// karşılaştırma tablosuna çevirir. Süreler çekirdek süresidir (iki olay arası, ısınmadan
// sonra n tekrarın ortalaması). Matris çarpımı iki yolla ölçülür: kitaplık (varsa) ve kendi
// çekirdeğimiz.
func TestMikroKiyas(t *testing.T) {
	if os.Getenv("TR_GPU_MIKRO") != "1" || benzetimde() {
		t.Skip("yalnızca gerçek GPU'da ve TR_GPU_MIKRO=1 ile")
	}
	g := gpuArkaUc()
	cs, _ := g.sr.(*cudaSurucu)
	tepe, bant := 0.0, 1800.0
	if cs != nil {
		tepe = cs.tepeGFLOPS()
	}
	if v, err := strconv.ParseFloat(os.Getenv("TR_GPU_MIKRO_BANT"), 64); err == nil && v > 0 {
		bant = v
	}
	fmt.Printf("MIKRO|bilgi|%s|0|%.0f|tepe GFLOPS (bant %.0f GB/s)\n", g.carpimYolu(), tepe, bant)
	mikroHepsi(g, tepe, bant, 1)
	for _, b := range BellekOlcumleri() {
		fmt.Printf("MIKRO|bellek|gpu:%d|0|%.0f|MB tepe canlı (ayrılmış tepe %.0f MB)\n", b.Cihaz,
			float64(b.TepeCanli)/(1<<20), float64(b.TepeAyrilmis)/(1<<20))
	}
}

// TestMikroKiyasKucuk kıyas kodunun kendisini benzetimde küçük boylarla koşturur.
func TestMikroKiyasKucuk(t *testing.T) {
	if !benzetimde() || testing.Short() {
		t.Skip("benzetim sınaması")
	}
	mikroHepsi(gpuArkaUc(), 0, 1800, 64)
}

// mikroHepsi bütün mikro kıyasları boylar bol'e bölünmüş olarak koşturur (1: gerçek boylar).
func mikroHepsi(g *arkaUc, tepe, bant float64, bol int) {
	mikroCarpimlar(g, tepe, bol)
	mikroDikkat(g, bol)
	mikroCE(g, bol)
	mikroAdamW(g, bol)
	mikroEleman(g, bant, bol)
}

// mikroOlc f'yi bir kez ısındırıp n kez çalıştırır; tekrar başına GPU süresini (ms) verir.
func mikroOlc(g *arkaUc, n int, f func()) float64 {
	f()
	g.gir()
	g.esitle()
	a, _ := g.sr.olayOlustur()
	b, _ := g.sr.olayOlustur()
	g.sr.olayKaydet(a)
	g.cik()
	for i := 0; i < n; i++ {
		f()
	}
	g.gir()
	defer g.cik()
	g.sr.olayKaydet(b)
	g.esitle()
	ms, _ := g.sr.olaySure(a, b)
	return float64(ms) / float64(n)
}

func mikroYaz(islem, yol string, ms, deger float64, birim string) {
	fmt.Printf("MIKRO|%s|%s|%.4f|%.1f|%s\n", islem, yol, ms, deger, birim)
}

// rastgeleDepo n elemanlı rastgele float32 GPU depolaması.
func rastgeleDepo(g *arkaUc, rng *rand.Rand, n int, olcek float64) tensor.Storage {
	return g.FromHost(rastgele(rng, n, olcek), tensor.Float32)
}

func mikroCarpimlar(g *arkaUc, tepe float64, bol int) {
	rng := rand.New(rand.NewSource(1))
	type carpim struct {
		ad      string
		M, N, K int
		ta, tb  bool
	}
	T, D, FF, V := 8192/bol, 512/bol, 2048/bol, 40000/bol
	liste := []carpim{
		{"çarpım qkv ileri", T, D, D, false, false}, {"çarpım ff1 ileri", T, FF, D, false, false},
		{"çarpım ff2 ileri", T, D, FF, false, false}, {"çarpım çıkış ileri", T, V, D, false, false},
		{"çarpım qkv dX", T, D, D, false, true}, {"çarpım ff1 dX", T, D, FF, false, true},
		{"çarpım çıkış dX", T, D, V, false, true}, {"çarpım qkv dW", D, D, T, true, false},
		{"çarpım ff1 dW", D, FF, T, true, false}, {"çarpım çıkış dW", V, D, T, true, false},
	}
	for _, c := range liste {
		a := rastgeleDepo(g, rng, c.M*c.K, 1)
		b := rastgeleDepo(g, rng, c.K*c.N, 1)
		out := g.AllocUninit(c.M*c.N, tensor.Float32)
		av := tensor.View{S: a, Shape: []int{c.M * c.K}, Strides: []int{1}}
		bv := tensor.View{S: b, Shape: []int{c.K * c.N}, Strides: []int{1}}
		f := func() { g.GemmSet(1, c.M, c.N, c.K, av, c.ta, bv, c.tb, out) }
		yollar := []string{"kendi"}
		if g.lt != nil {
			yollar = []string{"cublaslt", "kendi"}
		}
		lt := g.lt
		for _, yol := range yollar {
			if yol == "kendi" {
				g.lt = nil
			}
			ms := mikroOlc(g, 10, f)
			gf := 2 * float64(c.M) * float64(c.N) * float64(c.K) / (ms * 1e6)
			birim := "GFLOPS"
			if tepe > 0 {
				birim = fmt.Sprintf("GFLOPS (tepenin %%%.0f)", 100*gf/tepe)
			}
			mikroYaz(c.ad, yol, ms, gf, birim)
			g.lt = lt
		}
		g.Release(a)
		g.Release(b)
		g.Release(out)
	}
}

func mikroDikkat(g *arkaUc, bol int) {
	rng := rand.New(rand.NewSource(2))
	B, H, T, Dh := max(32/bol, 1), max(8/bol, 1), max(256/bol, 8), max(64/bol, 8)
	n := B * H * T * Dh
	q, k, v := rastgeleDepo(g, rng, n, 1), rastgeleDepo(g, rng, n, 1), rastgeleDepo(g, rng, n, 1)
	sh := []int{B, H, T, Dh}
	gor := func(s tensor.Storage) tensor.View { return tensor.View{S: s, Shape: sh, Strides: bitisik(sh)} }
	p := tensor.AttentionParams{Batch: B, H: H, Hkv: H, Tq: T, Tk: T, Dh: Dh, Dv: Dh, Causal: true,
		Scale: float32(1 / math.Sqrt(float64(Dh)))}
	out := g.AllocUninit(n, tensor.Float32)
	var lse tensor.Storage
	ileri := func() {
		if lse != nil {
			g.Release(lse)
		}
		lse = g.Attention(out, gor(q), gor(k), gor(v), nil, p)
	}
	// Nedensel FLOP: ileri 2 çarpım (QKᵀ, PV), yarısı görünür.
	flop := 2 * 2 * float64(B*H) * float64(T*T*Dh) / 2
	dO := rastgeleDepo(g, rng, n, 1)
	gq, gk, gv := g.AllocUninit(n, tensor.Float32), g.AllocUninit(n, tensor.Float32), g.AllocUninit(n, tensor.Float32)
	geri := func() {
		g.AttentionGradSet(gq, gk, gv, dO, gor(q), gor(k), gor(v), nil, out, lse, p, [3]bool{true, true, true})
	}
	// İki yol da ölçülür (TR_GPU_DIKKAT ile seçilen): birleşik çekirdek ve matris çarpımı yolu.
	eski := g.dikkatYolu
	for _, y := range []struct{ yol, ek, ad string }{{"birlesik", "", "birleşik"}, {"carpim", " (matris yolu)", "kendi"}} {
		g.dikkatYolu = y.yol
		ms := mikroOlc(g, 10, ileri)
		mikroYaz("dikkat ileri"+y.ek, y.ad, ms, flop/(ms*1e6), "GFLOPS")
		ms = mikroOlc(g, 10, geri)
		mikroYaz("dikkat geri"+y.ek, y.ad, ms, 2.5*flop/(ms*1e6), "GFLOPS")
	}
	g.dikkatYolu = eski
	for _, s := range []tensor.Storage{q, k, v, out, lse, dO, gq, gk, gv} {
		g.Release(s)
	}
}

func mikroCE(g *arkaUc, bol int) {
	rng := rand.New(rand.NewSource(3))
	R, C := 8192/bol, 40000/bol
	x := rastgeleDepo(g, rng, R*C, 2)
	hedef := make([]int, R)
	for i := range hedef {
		hedef[i] = rng.Intn(C)
	}
	xv := tensor.View{S: x, Shape: []int{R, C}, Strides: []int{C, 1}}
	var loss, lse tensor.Storage
	ileri := func() {
		for _, s := range []tensor.Storage{loss, lse} {
			if s != nil {
				g.Release(s)
			}
		}
		loss, lse = g.CrossEntropy(xv, hedef, C)
	}
	ms := mikroOlc(g, 10, ileri)
	gb := 4 * float64(R) * float64(C) / 1e9
	mikroYaz("çapraz entropi ileri", "birleşik", ms, gb/(ms/1e3), "GB/s")
	gx := g.AllocUninit(R*C, tensor.Float32)
	ms = mikroOlc(g, 10, func() { g.CrossEntropyGradSet(gx, loss, xv, lse, hedef, C) })
	mikroYaz("çapraz entropi geri", "birleşik", ms, 2*gb/(ms/1e3), "GB/s")
	for _, s := range []tensor.Storage{x, gx, loss, lse} {
		g.Release(s)
	}
}

func mikroAdamW(g *arkaUc, bol int) {
	rng := rand.New(rand.NewSource(4))
	V, D, FF := 40000/bol, 512/bol, 2048/bol
	boylar := []int{V * D}
	for i := 0; i < 6; i++ {
		boylar = append(boylar, D, D*D, D*D, D*D, D*D, D, D*FF, D*FF, FF*D)
	}
	boylar = append(boylar, D)
	var ps, gs, ms, vs []tensor.Storage
	var bc1, bc2 []float32
	toplam := 0
	for _, n := range boylar {
		ps = append(ps, rastgeleDepo(g, rng, n, 1))
		gs = append(gs, rastgeleDepo(g, rng, n, 1))
		ms = append(ms, g.Alloc(n, tensor.Float32))
		vs = append(vs, g.Alloc(n, tensor.Float32))
		bc1, bc2 = append(bc1, 0.1), append(bc2, 0.001)
		toplam += n
	}
	sure := mikroOlc(g, 10, func() { g.AdamWStepAll(ps, gs, ms, vs, 1e-3, 0.9, 0.999, 1e-8, 0, bc1, bc2) })
	mikroYaz(fmt.Sprintf("AdamW %.1f M parametre", float64(toplam)/1e6), "çok tensörlü", sure,
		7*4*float64(toplam)/1e9/(sure/1e3), "GB/s")
	for _, l := range [][]tensor.Storage{ps, gs, ms, vs} {
		for _, s := range l {
			g.Release(s)
		}
	}
}

func mikroEleman(g *arkaUc, bant float64, bol int) {
	rng := rand.New(rand.NewSource(5))
	kenar := 8192 / bol
	n := kenar * kenar
	a, b := rastgeleDepo(g, rng, n, 1), rastgeleDepo(g, rng, n, 1)
	out := g.AllocUninit(n, tensor.Float32)
	duz := tensor.View{Shape: []int{n}, Strides: []int{1}}
	av, bv := duz, duz
	av.S, bv.S = a, b
	yaz := func(ad string, bayt int, f func()) {
		ms := mikroOlc(g, 10, f)
		gbs := float64(bayt) / 1e9 / (ms / 1e3)
		mikroYaz(ad, "kendi", ms, gbs, fmt.Sprintf("GB/s (%%%.0f)", 100*gbs/bant))
	}
	yaz(fmt.Sprintf("eleman toplama %dM", n>>20), 12*n, func() { g.Binary(tensor.OpAdd, out, []int{n}, av, bv) })
	yaz(fmt.Sprintf("eleman üstel %dM", n>>20), 8*n, func() { g.Unary(tensor.OpExp, 0, out, av) })
	sh := []int{kenar, kenar}
	dev := tensor.View{S: a, Shape: sh, Strides: []int{1, kenar}}
	yaz(fmt.Sprintf("devrik kopya %d²", kenar), 8*n, func() { g.CopyStrided(tensor.View{S: out, Shape: sh, Strides: bitisik(sh)}, dev) })
	for _, s := range []tensor.Storage{a, b, out} {
		g.Release(s)
	}
}
