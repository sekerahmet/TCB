package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// fusedAttention başvuru sürümüdür: softmax(q·kᵀ·ölçek + toplamsal maske)·v,
// GQA için k/v başları birleştirilerek çoğaltılır. Toplamsal maske nedensellik,
// pencere ve ek maskeden kurulur (bakılamayan yer -1e9).
func fusedAttention(q, k, v *Tensor, opt AttentionOptions) *Tensor {
	p := attentionParams(q, k, v, opt)
	nd := len(q.shape)
	if p.Hkv != p.H {
		group := p.H / p.Hkv
		var ks, vs []*Tensor
		for h := 0; h < p.H; h++ {
			ks = append(ks, k.Slice(nd-3, h/group, h/group+1))
			vs = append(vs, v.Slice(nd-3, h/group, h/group+1))
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
	score := MatMul(q, k.Transpose(-1, -2)).MulScalar(p.Scale).Add(New(ek, p.Tq, p.Tk))
	if opt.Mask != nil {
		// 0 → -1e9, 1 → 0
		score = score.Add(opt.Mask.AddScalar(-1).MulScalar(1e9))
	}
	return MatMul(score.Softmax(-1), v)
}

type attnState struct {
	ad             string
	B, H, Hkv      int
	Tq, Tk, Dh, Dv int
	causalFlag     bool
	window         int
	maskVal        bool
	maskBroadcast  bool
}

// randomMask her satırın en az bir anahtara bakabildiği 0/1 maskesi.
func randomMask(rng *rand.Rand, d attnState) *Tensor {
	sh := []int{d.B, d.H, d.Tq, d.Tk}
	if d.maskBroadcast {
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

// validRows her sorgu satırının (nedensellik, pencere ve maske ile)
// en az bir anahtara bakıp bakmadığı: bileşik sürüm tamamen kapalı satırda
// tanımsızdır, bu satırlar karşılaştırılmaz.
func validRows(d attnState, p AttentionParams, maskVal *Tensor) []bool {
	var md []float32
	if maskVal != nil {
		md = maskVal.Data()
	}
	ok := make([]bool, d.B*d.H*d.Tq)
	for bh := 0; bh < d.B*d.H; bh++ {
		for i := 0; i < d.Tq; i++ {
			lo, hi := p.Limits(i)
			for j := lo; j < hi; j++ {
				if md != nil {
					o := i*d.Tk + j
					if !d.maskBroadcast {
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

func compareAttention(t *testing.T, d attnState, tol float64) {
	t.Helper()
	rng := rand.New(rand.NewSource(int64(d.Tq*131 + d.Tk*7 + d.H)))
	q := Randn(rng, d.B, d.H, d.Tq, d.Dh)
	k := Randn(rng, d.B, d.Hkv, d.Tk, d.Dh)
	v := Randn(rng, d.B, d.Hkv, d.Tk, d.Dv)
	opt := AttentionOptions{Causal: d.causalFlag, Window: d.window}
	if d.maskVal {
		opt.Mask = randomMask(rng, d)
	}
	p := attentionParams(q, k, v, opt)
	valid := validRows(d, p, opt.Mask)
	g := Randn(rng, d.B, d.H, d.Tq, d.Dv)
	// Geçersiz satırların çıktı gradyanı sıfırlanır (bileşik sürüm orada tanımsız).
	gd := g.Data()
	for r, ok := range valid {
		if !ok {
			clear(gd[r*d.Dv : (r+1)*d.Dv])
		}
	}
	g = New(gd, g.shape...)

	execute := func(f func(q, k, v *Tensor, opt AttentionOptions) *Tensor) (out []float32, gq, gk, gv []float32) {
		qq, kk, vv := q.Detach().SetRequiresGrad(true), k.Detach().SetRequiresGrad(true), v.Detach().SetRequiresGrad(true)
		qq.n.grad, kk.n.grad, vv.n.grad = nil, nil, nil
		o := f(qq, kk, vv, opt)
		o.BackwardWith(g)
		return o.Data(), qq.Grad().Data(), kk.Grad().Data(), vv.Grad().Data()
	}
	bo, bq, bk, bv := execute(fusedAttention)
	fo, fq, fk, fv := execute(Attention)
	for r, ok := range valid {
		if !ok {
			for _, x := range fo[r*d.Dv : (r+1)*d.Dv] {
				if x != 0 {
					t.Fatalf("%s: tamamen kapalı satır %d sıfır olmalı: %v", d.ad, r, fo[r*d.Dv:(r+1)*d.Dv])
				}
			}
			copy(fo[r*d.Dv:(r+1)*d.Dv], bo[r*d.Dv:(r+1)*d.Dv])
		}
	}
	near(t, d.ad+" ileri", fo, bo, tol)
	near(t, d.ad+" dq", fq, bq, tol)
	near(t, d.ad+" dk", fk, bk, tol)
	near(t, d.ad+" dv", fv, bv, tol)
}

func attnStates() []attnState {
	var ds []attnState
	for _, T := range []int{1, 7, 33, 130} {
		ds = append(ds,
			attnState{ad: "tam", B: 2, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 8},
			attnState{ad: "nedensel", B: 1, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 16, Dv: 16, causalFlag: true},
			attnState{ad: "gqa", B: 2, H: 4, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 12, causalFlag: true},
			attnState{ad: "pencere", B: 1, H: 2, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, causalFlag: true, window: 5},
			attnState{ad: "iki yönlü pencere", B: 1, H: 1, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, window: 4},
			attnState{ad: "önbellekli (Tq<Tk)", B: 1, H: 4, Hkv: 2, Tq: max(T/3, 1), Tk: T + 3, Dh: 8, Dv: 8, causalFlag: true},
			attnState{ad: "önbellekli pencere", B: 1, H: 2, Hkv: 2, Tq: 1, Tk: T + 2, Dh: 8, Dv: 8, causalFlag: true, window: 6},
			attnState{ad: "maske", B: 2, H: 2, Hkv: 1, Tq: T, Tk: T, Dh: 8, Dv: 8, maskVal: true},
			attnState{ad: "yayınlanan maske", B: 2, H: 2, Hkv: 2, Tq: T, Tk: T, Dh: 8, Dv: 8, causalFlag: true, maskVal: true, maskBroadcast: true},
		)
	}
	return ds
}

// Birleşik dikkat ileri ve gradyanda bileşik sürümle aynı sonucu verir; farklı
// blok boyutlarıyla ve blok boyutuna bölünmeyen uzunluklarla denenir.
func TestAttentionMatchesFused(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, blk := range [][2]int{{0, 0}, {16, 16}, {7, 5}, {1, 3}, {128, 32}} {
		SetAttentionBlocks(CPU, blk[0], blk[1])
		for _, d := range attnStates() {
			d.ad = fmt.Sprintf("%s T=%d/%d blok=%v", d.ad, d.Tq, d.Tk, blk)
			compareAttention(t, d, 2e-4)
		}
	}
}

// Paralel yollar: işçi havuzuna dağıtılan büyük işler ve dQ'nun anahtar
// dilimlerinde ayrı biriktirildiği az başlı durum.
func TestAttentionParallel(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, blk := range [][2]int{{0, 0}, {32, 48}} {
		SetAttentionBlocks(CPU, blk[0], blk[1])
		for _, d := range []attnState{
			{ad: "paralel nedensel", B: 2, H: 4, Hkv: 4, Tq: 300, Tk: 300, Dh: 64, Dv: 64, causalFlag: true},
			{ad: "paralel gqa tek toplu", B: 1, H: 8, Hkv: 2, Tq: 257, Tk: 257, Dh: 64, Dv: 64, causalFlag: true},
			{ad: "paralel pencere", B: 1, H: 2, Hkv: 1, Tq: 400, Tk: 400, Dh: 64, Dv: 32, causalFlag: true, window: 70},
			{ad: "paralel maske", B: 1, H: 2, Hkv: 2, Tq: 200, Tk: 200, Dh: 64, Dv: 64, maskVal: true},
		} {
			p := AttentionParams{Batch: d.B, H: d.H, Tq: d.Tq, Tk: d.Tk, Dh: d.Dh, Dv: d.Dv, Causal: d.causalFlag, Window: d.window}
			if attnWorkerCount(attnJob(&p)) < 2 && workers() > 1 {
				t.Fatalf("%s: paralel yol denenmiyor", d.ad)
			}
			d.ad = fmt.Sprintf("%s blok=%v", d.ad, blk)
			compareAttention(t, d, 5e-4)
		}
	}
}

// Sonuç işçi sayısından bağımsızdır: tek geçiş ve iki geçiş kipleri (ve tek
// iş parçacığı) bit düzeyinde aynı ileri çıktıyı ve gradyanları verir.
func TestAttentionIndependentOfWorkers(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	rng := rand.New(rand.NewSource(11))
	q := Randn(rng, 1, 4, 200, 64)
	k := Randn(rng, 1, 2, 200, 64)
	v := Randn(rng, 1, 2, 200, 64)
	g := Randn(rng, 1, 4, 200, 64)
	execute := func(n int) [][]float32 {
		runtime.GOMAXPROCS(n)
		qq, kk, vv := q.Detach().SetRequiresGrad(true), k.Detach().SetRequiresGrad(true), v.Detach().SetRequiresGrad(true)
		o := Attention(qq, kk, vv, AttentionOptions{Causal: true, Window: 90})
		o.BackwardWith(g)
		return [][]float32{o.Data(), qq.Grad().Data(), kk.Grad().Data(), vv.Grad().Data()}
	}
	a := execute(1)
	for _, n := range []int{2, 3, 8, 32} {
		b := execute(n)
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
func TestAttentionNumericalGrad(t *testing.T) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	SetAttentionBlocks(CPU, 2, 3)
	rng := rand.New(rand.NewSource(3))
	maskVal := New([]float32{1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1}, 5, 5)
	for _, opt := range []AttentionOptions{{}, {Causal: true}, {Causal: true, Window: 2}, {Mask: maskVal}} {
		q := random(rng, -1, 1, 0, 1, 4, 5, 3)
		k := random(rng, -1, 1, 0, 1, 2, 5, 3)
		v := random(rng, -1, 1, 0, 1, 2, 5, 2)
		checkGrad(t, fmt.Sprintf("dikkat %+v", opt), func(xs []*Tensor) *Tensor {
			return Attention(xs[0], xs[1], xs[2], opt)
		}, q, k, v)
	}
}

// Bitişik olmayan girdiler (eksen sırası görünümü) ve 2/3 boyutlu girdiler.
func TestAttentionViewsAndShapes(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	x := Randn(rng, 2, 9, 3, 4) // [B, T, H, dh]
	q := x.Permute(0, 2, 1, 3)
	opt := AttentionOptions{Causal: true}
	near(t, "görünüm", Attention(q, q, q, opt).Data(), fusedAttention(q, q, q, opt).Data(), 1e-5)
	q3 := Randn(rng, 3, 6, 4)
	near(t, "3 boyut", Attention(q3, q3, q3, opt).Data(), fusedAttention(q3, q3, q3, opt).Data(), 1e-5)
	q2 := Randn(rng, 6, 4)
	near(t, "2 boyut", Attention(q2, q2, q2, opt).Data(), fusedAttention(q2, q2, q2, opt).Data(), 1e-5)
	shapeEqual(t, "şekil", Attention(q, q, q, opt).shape, []int{2, 3, 9, 4})
	expectPanic(t, "baş katı değil", func() { Attention(Randn(rng, 3, 4, 2), Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), opt) })
	expectPanic(t, "dh farklı", func() { Attention(Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 3), Randn(rng, 2, 4, 3), opt) })
	expectPanic(t, "maske yayınlanamaz", func() {
		Attention(Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), Randn(rng, 2, 4, 2), AttentionOptions{Mask: Ones(3, 4)})
	})
}

// bf16 girdiler float32 biriktirmeyle float32 çıktı verir: aynı (yuvarlanmış)
// değerlerin float32 sürümüyle aynı sonuç.
func TestAttentionBF16(t *testing.T) {
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
	near(t, "bf16 ileri", o.Data(), o32.Data(), 1e-5)
	o.SumAll().Backward()
	o32.SumAll().Backward()
	near(t, "bf16 dq", q.Grad().Data(), q32.Grad().Data(), 1e-4)
	if q.Grad().DType() != Float32 {
		t.Fatalf("bf16 gradyanı float32 olmalı")
	}
	if Attention(q, k, v, opt).DType() != BFloat16 {
		t.Fatalf("bf16 girdilerin varsayılan çıktısı bf16 olmalı")
	}
}

// Tq×Tk puan matrisi tutulmaz: büyük T'de ileri ve geri yayılımın ayırdığı bellek, tek bir
// T×T puan matrisinin küçük bir kesridir (doğrusal yol ~10 MB, puan matrisi 128 MB).
// Sınır üstten konur, çünkü tampon havuzu önceki testlerden dolu olabilir ve ayrılan
// belleği yalnızca azaltır. İki uzunluğun ayırdığı belleğin oranı havuzun durumuna
// bağlıydı: küçük T'nin tamponları havuzdan gelince oran 6'yı aşıyor, test paketin
// tamamında kalıp tek başına geçiyordu.
func TestAttentionMemoryLinear(t *testing.T) {
	const T, headCount = 4096, 2
	rng := rand.New(rand.NewSource(1))
	q := Randn(rng, 1, headCount, T, 32).SetRequiresGrad(true)
	k := Randn(rng, 1, headCount, T, 32).SetRequiresGrad(true)
	v := Randn(rng, 1, headCount, T, 32).SetRequiresGrad(true)
	allocated := float64(allocatedBytes(func() {
		Attention(q, k, v, AttentionOptions{Causal: true}).SumAll().Backward()
	}))
	scoreMatrix := float64(headCount * T * T * 4) // float32 puanlar
	if allocated > scoreMatrix/4 {
		t.Fatalf("bellek doğrusal değil: T=%d'de %.0f bayt ayrıldı; tek bir T×T puan matrisi %.0f bayt", T, allocated, scoreMatrix)
	}
}

// allocatedBytes fn'in çalışırken ayırdığı toplam bayt.
func allocatedBytes(fn func()) uint64 {
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	fn()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

func TestSinCos(t *testing.T) {
	var worst float64
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
			if e := math.Abs(got-want) / math.Max(ulp, 1e-45); e > worst {
				worst = e
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
	checkGrad(t, "sin", func(xs []*Tensor) *Tensor { return xs[0].Sin() }, random(rng, -4, 4, 0, 3, 4))
	checkGrad(t, "cos", func(xs []*Tensor) *Tensor { return xs[0].Cos() }, random(rng, -4, 4, 0, 3, 4))
}

func TestConcatAndSplit(t *testing.T) {
	a := sorted(2, 3)
	b := sorted(2, 1).MulScalar(10)
	c := Concat([]*Tensor{a, b}, -1)
	shapeEqual(t, "concat şekil", c.shape, []int{2, 4})
	near(t, "concat", c.Data(), []float32{1, 2, 3, 10, 4, 5, 6, 20}, 0)
	near(t, "concat 0", Concat([]*Tensor{a, a}, 0).Data(), []float32{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6}, 0)
	// Bitişik olmayan girdi ve bf16 çıktı.
	near(t, "concat devrik", Concat([]*Tensor{a.T(), a.T()}, 1).Data(), []float32{1, 4, 1, 4, 2, 5, 2, 5, 3, 6, 3, 6}, 0)
	h := Concat([]*Tensor{a.Cast(BFloat16), a.Cast(BFloat16)}, 0)
	if h.DType() != BFloat16 {
		t.Fatalf("bf16 birleştirme türü %s", h.DType())
	}
	if Concat([]*Tensor{a.Cast(BFloat16), a}, 0).DType() != Float32 {
		t.Fatal("karışık türlerin sonucu float32 olmalı")
	}
	expectPanic(t, "şekil uyumsuz", func() { Concat([]*Tensor{sorted(2, 3), sorted(3, 3)}, 1) })
	expectPanic(t, "boyut uyumsuz", func() { Concat([]*Tensor{sorted(2, 3), sorted(6)}, 0) })
	rng := rand.New(rand.NewSource(4))
	checkGrad(t, "concat", func(xs []*Tensor) *Tensor {
		return Concat([]*Tensor{xs[0], xs[1], xs[0]}, 1).Mul(Concat([]*Tensor{xs[1], xs[0], xs[1]}, 1))
	}, random(rng, -1, 1, 0, 2, 2, 3), random(rng, -1, 1, 0, 2, 2, 3))

	p := sorted(2, 6).Split(3, -1)
	if len(p) != 3 {
		t.Fatalf("split parça sayısı %d", len(p))
	}
	near(t, "split", p[1].Data(), []float32{3, 4, 9, 10}, 0)
	expectPanic(t, "split bölünmez", func() { sorted(2, 5).Split(2, -1) })
	checkGrad(t, "split", func(xs []*Tensor) *Tensor {
		s := xs[0].Split(2, 1)
		return s[0].Mul(s[1])
	}, random(rng, -1, 1, 0, 3, 4))
}

// TestAttentionAliasGrad q, k, v aynı tensör olduğunda gradyanın, aynı veriden kurulmuş üç
// ayrı tensörün gradyanlarının toplamına eşit olduğunu denetler (16 bitlik gradyanda ayrı
// geçici tamponlar birbirinin sonucunu ezmemeli).
func TestAttentionAliasGrad(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		old := GradDType()
		SetGradDType(dt)
		x0 := random(rand.New(rand.NewSource(5)), -1, 1, 0, 1, 2, 9, 4)
		x := x0.Clone().SetRequiresGrad(true)
		Attention(x, x, x, AttentionOptions{Causal: true}).SumAll().Backward()
		a, b, c := x0.Clone().SetRequiresGrad(true), x0.Clone().SetRequiresGrad(true), x0.Clone().SetRequiresGrad(true)
		Attention(a, b, c, AttentionOptions{Causal: true}).SumAll().Backward()
		total := Add(Add(a.Grad().Cast(Float32), b.Grad().Cast(Float32)), c.Grad().Cast(Float32))
		tol := 1e-5
		if dt != Float32 {
			tol = 2e-2
		}
		near(t, "q=k=v gradyanı "+dt.String(), x.Grad().Cast(Float32).Data(), total.Data(), tol)
		SetGradDType(old)
	}
}

// Puanları NaN olan sorgu satırı NaN verir (bakılacak anahtarı olmayan satırın 0'ıyla
// karışmaz); geri yönde de gradyanı NaN'dır. Öbür satırlar etkilenmez.
func TestAttentionNaNRow(t *testing.T) {
	nan := float32(math.NaN())
	q := New([]float32{nan, nan, 1, 0}, 2, 2).SetRequiresGrad(true)
	k := New([]float32{1, 0, 0, 1, 1, 1}, 3, 2)
	v := New([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := Attention(q, k, v, AttentionOptions{})
	d := y.Data()
	if d[0] == d[0] || d[1] == d[1] {
		t.Fatalf("NaN satırı NaN vermeli: %v", d)
	}
	if d[2] != d[2] || d[3] != d[3] {
		t.Fatalf("öbür satır etkilenmemeli: %v", d)
	}
	y.SumAll().Backward()
	if g := q.Grad().Data(); g[0] == g[0] || g[2] != g[2] {
		t.Fatalf("gradyan: NaN satırda NaN, öbüründe sayı olmalı: %v", g)
	}
}

// Dikkatin geri yönü ileri çıktıyı da okur: çıktı geri yayılımdan önce yerinde değişirse
// sessizce yanlış gradyan yerine "yerinde değiştirildi" hatası verilir (softmax gibi).
func TestAttentionOutputChangedInPlace(t *testing.T) {
	q := New([]float32{1, 0, 0, 1}, 2, 2).SetRequiresGrad(true)
	k := New([]float32{1, 0, 0, 1, 1, 1}, 3, 2)
	v := New([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := Attention(q, k, v, AttentionOptions{})
	y.Set(100, 0, 0)
	panicCode(t, "dikkat çıktısı", messages.H3012, func() { y.SumAll().Backward() })
}

// ============================================================================
// Gradyan kapısı: birleşik dikkat (float64 başvuru + bf16)
// ============================================================================

// attention64 q [B,H,Tq,Dh], k [B,Hkv,Tk,Dh], v [B,Hkv,Tk,Dv] için softmax(q·kᵀ·ölçek)·v
// başvurusudur; bakılabilen aralık belgedeki kurala göre (sondan hizalı nedensellik,
// pencere, 0/1 maske) bulunur. Hiçbir anahtara bakamayan satırın çıktısı 0'dır.
func attention64(p AttentionParams, mask []float64) refFn {
	return func(in [][]float64) []float64 {
		q, k, v := in[0], in[1], in[2]
		group := p.H / p.Hkv
		res := make([]float64, p.Batch*p.H*p.Tq*p.Dv)
		for b := 0; b < p.Batch; b++ {
			for h := 0; h < p.H; h++ {
				kv := b*p.Hkv + h/group
				for i := 0; i < p.Tq; i++ {
					pos := p.Tk - p.Tq + i
					lo, hi := 0, p.Tk
					if p.Causal {
						hi = pos + 1
					} else if p.Window > 0 {
						hi = min(p.Tk, pos+p.Window)
					}
					if p.Window > 0 {
						lo = max(0, pos-p.Window+1)
					}
					var sc []float64
					for j := lo; j < hi; j++ {
						if mask != nil && mask[i*p.Tk+j] == 0 {
							sc = append(sc, math.Inf(-1))
							continue
						}
						var s float64
						for d := 0; d < p.Dh; d++ {
							s += q[((b*p.H+h)*p.Tq+i)*p.Dh+d] * k[(kv*p.Tk+j)*p.Dh+d]
						}
						sc = append(sc, s*float64(p.Scale))
					}
					attendRow64(res[((b*p.H+h)*p.Tq+i)*p.Dv:][:p.Dv], sc, lo, v[kv*p.Tk*p.Dv:], p.Dv)
				}
			}
		}
		return res
	}
}

// attendRow64 puanların softmax'ını alıp değer satırlarının ağırlıklı toplamını out'a yazar.
func attendRow64(out, sc []float64, lo int, v []float64, dv int) {
	m := math.Inf(-1)
	for _, s := range sc {
		m = math.Max(m, s)
	}
	if math.IsInf(m, -1) {
		return
	}
	var z float64
	for _, s := range sc {
		z += math.Exp(s - m)
	}
	for jj, s := range sc {
		w := math.Exp(s-m) / z
		for d := 0; d < dv; d++ {
			out[d] += w * v[(lo+jj)*dv+d]
		}
	}
}

func init() {
	att := func(name string, p AttentionParams, opt AttentionOptions, mask []float64) gradSpec {
		sq := []int{p.Batch, p.H, p.Tq, p.Dh}
		sk := []int{p.Batch, p.Hkv, p.Tk, p.Dh}
		sv := []int{p.Batch, p.Hkv, p.Tk, p.Dv}
		if p.Batch == 1 && p.H == 1 { // tek başlı 2 boyutlu biçim
			sq, sk, sv = sq[2:], sk[2:], sv[2:]
		}
		p.Causal, p.Window = opt.Causal, opt.Window
		p.Scale = opt.Scale
		if p.Scale == 0 {
			p.Scale = float32(1 / math.Sqrt(float64(p.Dh)))
		}
		return gradSpec{name: name, ops: []string{"attention"},
			build:  func(xs []*Tensor) *Tensor { return Attention(xs[0], xs[1], xs[2], opt) },
			inputs: genInputs(gen(-1, 1, 0, sq...), gen(-1, 1, 0, sk...), gen(-1, 1, 0, sv...)),
			ref:    attention64(p, mask)}
	}
	mask := []float64{1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1}
	maskT := New([]float32{1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1}, 3, 5)
	maskSpec := att("dikkat maske", AttentionParams{Batch: 1, H: 2, Hkv: 2, Tq: 3, Tk: 5, Dh: 4, Dv: 3}, AttentionOptions{Mask: maskT}, mask)
	registerGrad("attention",
		att("dikkat 2B", AttentionParams{Batch: 1, H: 1, Hkv: 1, Tq: 3, Tk: 5, Dh: 4, Dv: 3}, AttentionOptions{}, nil),
		att("dikkat nedensel GQA", AttentionParams{Batch: 2, H: 2, Hkv: 1, Tq: 3, Tk: 5, Dh: 4, Dv: 3}, AttentionOptions{Causal: true}, nil),
		att("dikkat pencere", AttentionParams{Batch: 1, H: 2, Hkv: 2, Tq: 4, Tk: 4, Dh: 4, Dv: 3}, AttentionOptions{Window: 2}, nil),
		att("dikkat nedensel pencere ölçek", AttentionParams{Batch: 1, H: 2, Hkv: 2, Tq: 4, Tk: 4, Dh: 4, Dv: 3}, AttentionOptions{Causal: true, Window: 3, Scale: 0.7}, nil),
		maskSpec)
}

func TestGradAttention(t *testing.T) { runGradGroup(t, "attention") }
