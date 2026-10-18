package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// bilesikDikkat başvuru sürümüdür: softmax(q·kᵀ·ölçek + toplamsal maske)·v,
// GQA için k/v başları birleştirilerek çoğaltılır. Toplamsal maske nedensellik,
// pencere ve ek maskeden kurulur (bakılamayan yer -1e9).
func bilesikDikkat(q, k, v *Tensor, opt AttentionOptions) *Tensor {
	p := attentionParams(q, k, v, opt)
	nd := len(q.shape)
	if p.Hkv != p.H {
		grup := p.H / p.Hkv
		var ks, vs []*Tensor
		for h := 0; h < p.H; h++ {
			ks = append(ks, k.Slice(nd-3, h/grup, h/grup+1))
			vs = append(vs, v.Slice(nd-3, h/grup, h/grup+1))
		}
		k, v = Concat(ks, nd-3), Concat(vs, nd-3)
	}
	ek := make([]float32, p.Tq*p.Tk)
	for i := 0; i < p.Tq; i++ {
		lo, hi := p.Limits(i)
		for j := 0; j < p.Tk; j++ {
			if j < lo || j >= hi {
				ek[i*p.Tk+j] = -1e9
			}
		}
	}
	puan := MatMul(q, k.Transpose(-1, -2)).MulScalar(p.Scale).Add(New(ek, p.Tq, p.Tk))
	if opt.Mask != nil {
		// 0 → -1e9, 1 → 0
		puan = puan.Add(opt.Mask.AddScalar(-1).MulScalar(1e9))
	}
	return MatMul(puan.Softmax(-1), v)
}

type dikkatDurumu struct {
	ad              string
	B, H, Hkv       int
	Tq, Tk, Dh, Dv  int
	nedensel        bool
	pencere         int
	maske           bool
	maskeYayinlanan bool
}

// rastgeleMaske her satırın en az bir anahtara bakabildiği 0/1 maskesi.
func rastgeleMaske(rng *rand.Rand, d dikkatDurumu) *Tensor {
	sh := []int{d.B, d.H, d.Tq, d.Tk}
	if d.maskeYayinlanan {
		sh = []int{d.Tq, d.Tk}
	}
	m := make([]float32, numelOf(sh))
	for i := range m {
		if rng.Float32() < 0.7 {
			m[i] = 1
		}
	}
	return New(m, sh...)
}

// gecerliSatirlar her sorgu satırının (nedensellik, pencere ve maske ile)
// en az bir anahtara bakıp bakmadığı: bileşik sürüm tamamen kapalı satırda
// tanımsızdır, bu satırlar karşılaştırılmaz.
func gecerliSatirlar(d dikkatDurumu, p AttentionParams, maske *Tensor) []bool {
	var md []float32
	if maske != nil {
		md = maske.Data()
	}
	ok := make([]bool, d.B*d.H*d.Tq)
	for bh := 0; bh < d.B*d.H; bh++ {
		for i := 0; i < d.Tq; i++ {
			lo, hi := p.Limits(i)
			for j := lo; j < hi; j++ {
				if md != nil {
					o := i*d.Tk + j
					if !d.maskeYayinlanan {
						o += bh * d.Tq * d.Tk
					}
					if md[o] == 0 {
						continue
					}
				}
				ok[bh*d.Tq+i] = true
				break
			}
		}
	}
	return ok
}

func dikkatKarsilastir(t *testing.T, d dikkatDurumu, tol float64) {
	t.Helper()
	rng := rand.New(rand.NewSource(int64(d.Tq*131 + d.Tk*7 + d.H)))
	q := Randn(rng, d.B, d.H, d.Tq, d.Dh)
	k := Randn(rng, d.B, d.Hkv, d.Tk, d.Dh)
	v := Randn(rng, d.B, d.Hkv, d.Tk, d.Dv)
	opt := AttentionOptions{Causal: d.nedensel, Window: d.pencere}
	if d.maske {
		opt.Mask = rastgeleMaske(rng, d)
	}
	p := attentionParams(q, k, v, opt)
	gecerli := gecerliSatirlar(d, p, opt.Mask)
	g := Randn(rng, d.B, d.H, d.Tq, d.Dv)
	// Geçersiz satırların çıktı gradyanı sıfırlanır (bileşik sürüm orada tanımsız).
	gd := g.Data()
	for r, ok := range gecerli {
		if !ok {
			clear(gd[r*d.Dv : (r+1)*d.Dv])
		}
	}
	g = New(gd, g.shape...)

	calistir := func(f func(q, k, v *Tensor, opt AttentionOptions) *Tensor) (out []float32, gq, gk, gv []float32) {
		qq, kk, vv := q.Detach().SetRequiresGrad(true), k.Detach().SetRequiresGrad(true), v.Detach().SetRequiresGrad(true)
		qq.grad, kk.grad, vv.grad = nil, nil, nil
		o := f(qq, kk, vv, opt)
		o.BackwardWith(g)
		return o.Data(), qq.Grad().Data(), kk.Grad().Data(), vv.Grad().Data()
	}
	bo, bq, bk, bv := calistir(bilesikDikkat)
	fo, fq, fk, fv := calistir(Attention)
	for r, ok := range gecerli {
		if !ok {
			for _, x := range fo[r*d.Dv : (r+1)*d.Dv] {
				if x != 0 {
					t.Fatalf("%s: tamamen kapalı satır %d sıfır olmalı: %v", d.ad, r, fo[r*d.Dv:(r+1)*d.Dv])
				}
			}
			copy(fo[r*d.Dv:(r+1)*d.Dv], bo[r*d.Dv:(r+1)*d.Dv])
		}
	}
	yakin(t, d.ad+" ileri", fo, bo, tol)
	yakin(t, d.ad+" dq", fq, bq, tol)
	yakin(t, d.ad+" dk", fk, bk, tol)
	yakin(t, d.ad+" dv", fv, bv, tol)
}

func dikkatDurumlari() []dikkatDurumu {
	var ds []dikkatDurumu
	for _, T := range []int{1, 7, 33, 130} {
		ds = append(ds,
			dikkatDurumu{ad: "tam", B: 2, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 8},
			dikkatDurumu{ad: "nedensel", B: 1, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 16, Dv: 16, nedensel: true},
			dikkatDurumu{ad: "gqa", B: 2, H: 4, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 12, nedensel: true},
			dikkatDurumu{ad: "pencere", B: 1, H: 2, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, nedensel: true, pencere: 5},
			dikkatDurumu{ad: "iki yönlü pencere", B: 1, H: 1, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, pencere: 4},
			dikkatDurumu{ad: "önbellekli (Tq<Tk)", B: 1, H: 4, Hkv: 2, Tq: max(T/3, 1), Tk: T + 3, Dh: 8, Dv: 8, nedensel: true},
			dikkatDurumu{ad: "önbellekli pencere", B: 1, H: 2, Hkv: 2, Tq: 1, Tk: T + 2, Dh: 8, Dv: 8, nedensel: true, pencere: 6},
			dikkatDurumu{ad: "maske", B: 2, H: 2, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, maske: true},
			dikkatDurumu{ad: "yayınlanan maske", B: 2, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 8, nedensel: true, maske: true, maskeYayinlanan: true},
		)
	}
	return ds
}

// Birleşik dikkat ileri ve gradyanda bileşik sürümle aynı sonucu verir; farklı
// blok boyutlarıyla ve blok boyutuna bölünmeyen uzunluklarla denenir.
func TestDikkatBilesikleAyni(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, blok := range [][2]int{{0, 0}, {16, 16}, {7, 5}, {1, 3}, {128, 32}} {
		SetAttentionBlocks(CPU, blok[0], blok[1])
		for _, d := range dikkatDurumlari() {
			d.ad = fmt.Sprintf("%s T=%d/%d blok=%v", d.ad, d.Tq, d.Tk, blok)
			dikkatKarsilastir(t, d, 2e-4)
		}
	}
}

// Paralel yollar: işçi havuzuna dağıtılan büyük işler ve dQ'nun anahtar
// dilimlerinde ayrı biriktirildiği az başlı durum.
func TestDikkatParalel(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, blok := range [][2]int{{0, 0}, {32, 48}} {
		SetAttentionBlocks(CPU, blok[0], blok[1])
		for _, d := range []dikkatDurumu{
			{ad: "paralel nedensel", B: 2, H: 4, Hkv: 4, Tq: 300, Tk: 300, Dh: 64, Dv: 64, nedensel: true},
			{ad: "paralel gqa tek toplu", B: 1, H: 8, Hkv: 2, Tq: 257, Tk: 257, Dh: 64, Dv: 64, nedensel: true},
			{ad: "paralel pencere", B: 1, H: 2, Hkv: 1, Tq: 400, Tk: 400, Dh: 64, Dv: 32, nedensel: true, pencere: 70},
			{ad: "paralel maske", B: 1, H: 2, Hkv: 2, Tq: 200, Tk: 200, Dh: 64, Dv: 64, maske: true},
		} {
			p := AttentionParams{Batch: d.B, H: d.H, Tq: d.Tq, Tk: d.Tk, Dh: d.Dh, Dv: d.Dv, Causal: d.nedensel, Window: d.pencere}
			if dikkatIsciSayisi(dikkatIs(&p)) < 2 && workers() > 1 {
				t.Fatalf("%s: paralel yol denenmiyor", d.ad)
			}
			d.ad = fmt.Sprintf("%s blok=%v", d.ad, blok)
			dikkatKarsilastir(t, d, 5e-4)
		}
	}
}

// Sonuç işçi sayısından bağımsızdır: tek geçiş ve iki geçiş kipleri (ve tek
// iş parçacığı) bit düzeyinde aynı ileri çıktıyı ve gradyanları verir.
func TestDikkatIsciSayisindanBagimsiz(t *testing.T) {
	onceki := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(onceki)
	rng := rand.New(rand.NewSource(11))
	q := Randn(rng, 1, 4, 200, 64)
	k := Randn(rng, 1, 2, 200, 64)
	v := Randn(rng, 1, 2, 200, 64)
	g := Randn(rng, 1, 4, 200, 64)
	calistir := func(n int) [][]float32 {
		runtime.GOMAXPROCS(n)
		qq, kk, vv := q.Detach().SetRequiresGrad(true), k.Detach().SetRequiresGrad(true), v.Detach().SetRequiresGrad(true)
		o := Attention(qq, kk, vv, AttentionOptions{Causal: true, Window: 90})
		o.BackwardWith(g)
		return [][]float32{o.Data(), qq.Grad().Data(), kk.Grad().Data(), vv.Grad().Data()}
	}
	a := calistir(1)
	for _, n := range []int{2, 3, 8, 32} {
		b := calistir(n)
		for i := range a {
			for j := range a[i] {
				if a[i][j] != b[i][j] {
					t.Fatalf("GOMAXPROCS=%d: çıktı %d eleman %d farklı: %v ≠ %v", n, i, j, b[i][j], a[i][j])
				}
			}
		}
	}
}

// Sayısal türev denetimi (küçük boyut, maske ve GQA ile).
func TestDikkatSayisalTurev(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	SetAttentionBlocks(CPU, 2, 3)
	rng := rand.New(rand.NewSource(3))
	maske := New([]float32{1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1}, 5, 5)
	for _, opt := range []AttentionOptions{{}, {Causal: true}, {Causal: true, Window: 2}, {Mask: maske}} {
		q := rastgele(rng, -1, 1, 0, 1, 4, 5, 3)
		k := rastgele(rng, -1, 1, 0, 1, 2, 5, 3)
		v := rastgele(rng, -1, 1, 0, 1, 2, 5, 2)
		gradDenetle(t, fmt.Sprintf("dikkat %+v", opt), func(xs []*Tensor) *Tensor {
			return Attention(xs[0], xs[1], xs[2], opt)
		}, q, k, v)
	}
}

// Bitişik olmayan girdiler (eksen sırası görünümü) ve 2/3 boyutlu girdiler.
func TestDikkatGorunumVeBoyutlar(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	x := Randn(rng, 2, 9, 3, 4) // [B, T, H, dh]
	q := x.Permute(0, 2, 1, 3)
	opt := AttentionOptions{Causal: true}
	yakin(t, "görünüm", Attention(q, q, q, opt).Data(), bilesikDikkat(q, q, q, opt).Data(), 1e-5)
	q3 := Randn(rng, 3, 6, 4)
	yakin(t, "3 boyut", Attention(q3, q3, q3, opt).Data(), bilesikDikkat(q3, q3, q3, opt).Data(), 1e-5)
	q2 := Randn(rng, 6, 4)
	yakin(t, "2 boyut", Attention(q2, q2, q2, opt).Data(), bilesikDikkat(q2, q2, q2, opt).Data(), 1e-5)
	sekilEsit(t, "şekil", Attention(q, q, q, opt).shape, []int{2, 3, 9, 4})
	panikBekle(t, "baş katı değil", func() { Attention(Randn(rng, 3, 4, 2), Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), opt) })
	panikBekle(t, "dh farklı", func() { Attention(Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 3), Randn(rng, 2, 4, 3), opt) })
	panikBekle(t, "maske yayınlanamaz", func() {
		Attention(Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), AttentionOptions{Mask: Ones(3, 4)})
	})
}

// bf16 girdiler float32 biriktirmeyle float32 çıktı verir: aynı (yuvarlanmış)
// değerlerin float32 sürümüyle aynı sonuç.
func TestDikkatBF16(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	q32 := Randn(rng, 1, 2, 33, 16).Cast(BFloat16).Cast(Float32)
	k32 := Randn(rng, 1, 2, 33, 16).Cast(BFloat16).Cast(Float32)
	v32 := Randn(rng, 1, 2, 33, 16).Cast(BFloat16).Cast(Float32)
	opt := AttentionOptions{Causal: true}
	q, k, v := q32.Cast(BFloat16).SetRequiresGrad(true), k32.Cast(BFloat16).SetRequiresGrad(true), v32.Cast(BFloat16).SetRequiresGrad(true)
	o := AttentionOut(q, k, v, opt, Float32)
	if o.DType() != Float32 {
		t.Fatalf("çıktı türü %s", o.DType())
	}
	q32.SetRequiresGrad(true)
	o32 := Attention(q32, k32, v32, opt)
	yakin(t, "bf16 ileri", o.Data(), o32.Data(), 1e-5)
	o.SumAll().Backward()
	o32.SumAll().Backward()
	yakin(t, "bf16 dq", q.Grad().Data(), q32.Grad().Data(), 1e-4)
	if q.Grad().DType() != Float32 {
		t.Fatalf("bf16 gradyanı float32 olmalı")
	}
	if Attention(q, k, v, opt).DType() != BFloat16 {
		t.Fatalf("bf16 girdilerin varsayılan çıktısı bf16 olmalı")
	}
}

// Tq×Tk puan matrisi tutulmaz: büyük T'de ayrılan bellek T ile doğrusal kalır.
func TestDikkatBellegiDogrusal(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	olc := func(T int) float64 {
		q := Randn(rng, 1, 2, T, 32).SetRequiresGrad(true)
		k := Randn(rng, 1, 2, T, 32).SetRequiresGrad(true)
		v := Randn(rng, 1, 2, T, 32).SetRequiresGrad(true)
		return float64(ayrilanBayt(func() {
			Attention(q, k, v, AttentionOptions{Causal: true}).SumAll().Backward()
		}))
	}
	a, b := olc(512), olc(2048)
	// Dört kat uzunlukta bellek dört kat civarı olmalı (T×T olsaydı 16 kat).
	if b > 6*a {
		t.Fatalf("bellek doğrusal değil: T=512 %.0f bayt, T=2048 %.0f bayt", a, b)
	}
}

// ayrilanBayt fn'in çalışırken ayırdığı toplam bayt.
func ayrilanBayt(fn func()) uint64 {
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	fn()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

func TestSinCos(t *testing.T) {
	var enKotu float64
	for _, x := range []float32{0, 1e-8, 0.5, -0.785, 1, 1.5707964, 3.1415927, -3, 10, 100, 1000.5, 12345.6, 65000, -131000, 2e5, 1e10, float32(math.Inf(1))} {
		for _, f := range []struct {
			ad string
			g  func(float32) float32
			b  func(float64) float64
		}{{"sin", sinf32, math.Sin}, {"cos", cosf32, math.Cos}} {
			got, want := float64(f.g(x)), f.b(float64(x))
			if math.IsNaN(want) {
				if !math.IsNaN(got) {
					t.Fatalf("%s(%v) = %v, NaN bekleniyordu", f.ad, x, got)
				}
				continue
			}
			ulp := float64(math.Nextafter32(float32(math.Abs(want)), float32(math.Inf(1)))) - math.Abs(float64(float32(want)))
			if e := math.Abs(got-want) / math.Max(ulp, 1e-45); e > enKotu {
				enKotu = e
			}
			if math.Abs(got-want) > 1.01*ulp+1e-12 {
				t.Fatalf("%s(%v) = %v, beklenen %v", f.ad, x, got, want)
			}
		}
	}
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 200000; i++ {
		x := (rng.Float32()*2 - 1) * 5000
		for _, f := range []struct {
			g func(float32) float32
			b func(float64) float64
		}{{sinf32, math.Sin}, {cosf32, math.Cos}} {
			got, want := f.g(x), float32(f.b(float64(x)))
			if got != want && math.Abs(float64(got-want)) > float64(math.Nextafter32(abs32(want), 2)-abs32(want)) {
				t.Fatalf("%v: %v, beklenen %v", x, got, want)
			}
		}
	}
	// Türev
	gradDenetle(t, "sin", func(xs []*Tensor) *Tensor { return xs[0].Sin() }, rastgele(rng, -4, 4, 0, 3, 4))
	gradDenetle(t, "cos", func(xs []*Tensor) *Tensor { return xs[0].Cos() }, rastgele(rng, -4, 4, 0, 3, 4))
}

func TestConcatVeSplit(t *testing.T) {
	a := sirali(2, 3)
	b := sirali(2, 1).MulScalar(10)
	c := Concat([]*Tensor{a, b}, -1)
	sekilEsit(t, "concat şekil", c.shape, []int{2, 4})
	yakin(t, "concat", c.Data(), []float32{1, 2, 3, 10, 4, 5, 6, 20}, 0)
	yakin(t, "concat 0", Concat([]*Tensor{a, a}, 0).Data(), []float32{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6}, 0)
	// Bitişik olmayan girdi ve bf16 çıktı.
	yakin(t, "concat devrik", Concat([]*Tensor{a.T(), a.T()}, 1).Data(), []float32{1, 4, 1, 4, 2, 5, 2, 5, 3, 6, 3, 6}, 0)
	h := Concat([]*Tensor{a.Cast(BFloat16), a.Cast(BFloat16)}, 0)
	if h.DType() != BFloat16 {
		t.Fatalf("bf16 birleştirme türü %s", h.DType())
	}
	if Concat([]*Tensor{a.Cast(BFloat16), a}, 0).DType() != Float32 {
		t.Fatal("karışık türlerin sonucu float32 olmalı")
	}
	panikBekle(t, "şekil uyumsuz", func() { Concat([]*Tensor{sirali(2, 3), sirali(3, 3)}, 1) })
	panikBekle(t, "boyut uyumsuz", func() { Concat([]*Tensor{sirali(2, 3), sirali(6)}, 0) })
	rng := rand.New(rand.NewSource(4))
	gradDenetle(t, "concat", func(xs []*Tensor) *Tensor {
		return Concat([]*Tensor{xs[0], xs[1], xs[0]}, 1).Mul(Concat([]*Tensor{xs[1], xs[0], xs[1]}, 1))
	}, rastgele(rng, -1, 1, 0, 2, 2, 3), rastgele(rng, -1, 1, 0, 2, 2, 3))

	p := sirali(2, 6).Split(3, -1)
	if len(p) != 3 {
		t.Fatalf("split parça sayısı %d", len(p))
	}
	yakin(t, "split", p[1].Data(), []float32{3, 4, 9, 10}, 0)
	panikBekle(t, "split bölünmez", func() { sirali(2, 5).Split(2, -1) })
	gradDenetle(t, "split", func(xs []*Tensor) *Tensor {
		s := xs[0].Split(2, 1)
		return s[0].Mul(s[1])
	}, rastgele(rng, -1, 1, 0, 3, 4))
}
