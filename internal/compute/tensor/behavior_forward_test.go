package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// İleri yön (forward) sonuç testleri.

func TestConstructorsAndAccess(t *testing.T) {
	x := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	shapeEqual(t, "şekil", x.Shape(), []int{2, 3})
	if x.Numel() != 6 || x.Dim() != 2 || x.Size(-1) != 3 {
		t.Fatal("boyut bilgileri yanlış")
	}
	if x.At(1, 2) != 6 || x.At(-1, 0) != 4 {
		t.Fatal("At yanlış")
	}
	x.Set(9, 0, 1)
	if x.At(0, 1) != 9 {
		t.Fatal("Set yanlış")
	}
	if Scalar(3).Item() != 3 || Scalar(3).Dim() != 0 {
		t.Fatal("skaler yanlış")
	}
	near(t, "ones", Ones(2, 2).Data(), []float32{1, 1, 1, 1}, 0)
	near(t, "full", Full(2.5, 3).Data(), []float32{2.5, 2.5, 2.5}, 0)
	near(t, "arange", Arange(4).Data(), []float32{0, 1, 2, 3}, 0)
	near(t, "f64", FromFloat64([]float64{1.5, -2}, 2).Data(), []float32{1.5, -2}, 0)
	expectPanic(t, "yanlış uzunluk", func() { New([]float32{1, 2, 3}, 2, 2) })
	expectPanic(t, "Item çok eleman", func() { x.Item() })
	expectPanic(t, "indeks dışı", func() { x.At(2, 0) })
	if x.String() == "" {
		t.Fatal("String boş")
	}
}

func TestViewsWithoutCopy(t *testing.T) {
	x := sorted(2, 3, 4)
	r := x.Reshape(6, -1)
	shapeEqual(t, "reshape", r.Shape(), []int{6, 4})
	if r.storage != x.storage {
		t.Fatal("reshape veri kopyaladı")
	}
	p := x.Permute(2, 0, 1)
	shapeEqual(t, "permute", p.Shape(), []int{4, 2, 3})
	if p.storage != x.storage || p.IsContiguous() {
		t.Fatal("permute kopyasız ve bitişik olmayan görünüm olmalı")
	}
	// p[k, i, j] = x[i, j, k]
	if p.At(3, 1, 2) != x.At(1, 2, 3) {
		t.Fatal("permute değerleri yanlış")
	}
	tr := x.Transpose(0, 2)
	shapeEqual(t, "transpose", tr.Shape(), []int{4, 3, 2})
	if tr.At(1, 2, 0) != x.At(0, 2, 1) {
		t.Fatal("transpose değerleri yanlış")
	}
	s := x.Slice(1, 1, 3)
	shapeEqual(t, "slice", s.Shape(), []int{2, 2, 4})
	if s.storage != x.storage || s.At(1, 0, 2) != x.At(1, 1, 2) {
		t.Fatal("slice yanlış")
	}
	ss := x.SliceStep(2, 0, 4, 2)
	near(t, "slice adım", ss.Slice(0, 0, 1).Slice(1, 0, 1).Data(), []float32{1, 3}, 0)
	near(t, "negatif slice", Arange(5).Slice(0, -2, 5).Data(), []float32{3, 4}, 0)
	// Bitişik olmayan görünümün Data'sı mantıksal sırayı verir.
	m := sorted(2, 3).T()
	near(t, "devrik veri", m.Data(), []float32{1, 4, 2, 5, 3, 6}, 0)
	// Bitişik olmayan tensörün Reshape'i kopyalar ama doğru değer verir.
	near(t, "devrik reshape", m.Reshape(6).Data(), []float32{1, 4, 2, 5, 3, 6}, 0)
	c := m.Contiguous()
	if !c.IsContiguous() || c.Contiguous() != c {
		t.Fatal("Contiguous yanlış")
	}
	u := x.Unsqueeze(1)
	shapeEqual(t, "unsqueeze", u.Shape(), []int{2, 1, 3, 4})
	shapeEqual(t, "squeeze", u.Squeeze(1).Shape(), []int{2, 3, 4})
	shapeEqual(t, "flatten", x.Flatten().Shape(), []int{24})
	// Görünüm üzerinden görünüm.
	v := x.Permute(1, 0, 2).Slice(2, 1, 3).Transpose(0, 1)
	shapeEqual(t, "zincir", v.Shape(), []int{2, 3, 2})
	if v.At(1, 2, 1) != x.At(1, 2, 2) {
		t.Fatal("görünüm zinciri yanlış")
	}
	expectPanic(t, "kötü reshape", func() { x.Reshape(5, -1) })
	expectPanic(t, "kötü permute", func() { x.Permute(0, 0, 1) })
}

func TestElementwiseAndBroadcast(t *testing.T) {
	a := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	b := New([]float32{10, 20, 30}, 3)
	near(t, "add satır", Add(a, b).Data(), []float32{11, 22, 33, 14, 25, 36}, 0)
	near(t, "sub", Sub(a, a).Data(), []float32{0, 0, 0, 0, 0, 0}, 0)
	near(t, "mul", Mul(a, a).Data(), []float32{1, 4, 9, 16, 25, 36}, 0)
	near(t, "div", Div(a, b).Data(), []float32{0.1, 0.1, 0.1, 0.4, 0.25, 0.2}, 1e-6)
	// (2,1) ile (1,3) -> (2,3)
	c := New([]float32{1, 2}, 2, 1)
	d := New([]float32{10, 20, 30}, 1, 3)
	r := Add(c, d)
	shapeEqual(t, "dış yayın", r.Shape(), []int{2, 3})
	near(t, "dış yayın", r.Data(), []float32{11, 21, 31, 12, 22, 32}, 0)
	// Skaler tensör ile.
	near(t, "skaler", Mul(a, Scalar(2)).Data(), []float32{2, 4, 6, 8, 10, 12}, 0)
	near(t, "skaler sol", Sub(Scalar(1), b).Data(), []float32{-9, -19, -29}, 0)
	// Farklı boyut sayısı: (2,1,3) - (4,1) -> (2,4,3)
	e := sorted(2, 1, 3)
	f := sorted(4, 1)
	g := Sub(e, f)
	shapeEqual(t, "3B yayın", g.Shape(), []int{2, 4, 3})
	if g.At(1, 2, 0) != e.At(1, 0, 0)-f.At(2, 0) {
		t.Fatal("3B yayın değeri yanlış")
	}
	// Bitişik olmayan işlenen.
	at := a.T() // (3,2)
	near(t, "devrik ile", Add(at, New([]float32{100, 200}, 2)).Data(), []float32{101, 204, 102, 205, 103, 206}, 0)
	// Sıfır uzunluklu eksen.
	z := Add(Zeros(0, 3), b)
	shapeEqual(t, "boş", z.Shape(), []int{0, 3})
	expectPanic(t, "uyumsuz", func() { Add(Zeros(2, 3), Zeros(2)) })
	expectPanic(t, "uyumsuz 2", func() { Add(Zeros(2, 3), Zeros(3, 3)) })
	// Büyük tensör (paralel yol) ve büyük satır yayını.
	rng := rand.New(rand.NewSource(1))
	big := Randn(rng, 300, 400)
	bias := Randn(rng, 400)
	s := Add(big, bias)
	if s.At(123, 321) != big.At(123, 321)+bias.At(321) {
		t.Fatal("büyük satır yayını yanlış")
	}
	m := Mul(big, big)
	if m.At(299, 399) != big.At(299, 399)*big.At(299, 399) {
		t.Fatal("büyük çarpım yanlış")
	}
}

func TestUnaryOps(t *testing.T) {
	x := New([]float32{-2, -0.5, 0.5, 2}, 4)
	f := func(fn func(float64) float64) []float32 {
		out := make([]float32, 4)
		for i, v := range x.Data() {
			out[i] = float32(fn(float64(v)))
		}
		return out
	}
	near(t, "exp", x.Exp().Data(), f(math.Exp), 1e-6)
	near(t, "tanh", x.Tanh().Data(), f(math.Tanh), 1e-6)
	near(t, "relu", x.Relu().Data(), []float32{0, 0, 0.5, 2}, 0)
	near(t, "sigmoid", x.Sigmoid().Data(), f(func(v float64) float64 { return 1 / (1 + math.Exp(-v)) }), 1e-6)
	near(t, "gelu", x.Gelu().Data(), f(func(v float64) float64 {
		return 0.5 * v * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(v+0.044715*v*v*v)))
	}), 1e-6)
	p := New([]float32{0.25, 1, 4}, 3)
	near(t, "log", p.Log().Data(), []float32{float32(math.Log(0.25)), 0, float32(math.Log(4))}, 1e-6)
	near(t, "sqrt", p.Sqrt().Data(), []float32{0.5, 1, 2}, 1e-6)
	near(t, "pow2", p.Pow(2).Data(), []float32{0.0625, 1, 16}, 1e-6)
	near(t, "pow-0.5", p.Pow(-0.5).Data(), []float32{2, 1, 0.5}, 1e-6)
	near(t, "add_scalar", p.AddScalar(1).Data(), []float32{1.25, 2, 5}, 0)
	near(t, "sub_scalar", p.SubScalar(1).Data(), []float32{-0.75, 0, 3}, 0)
	near(t, "mul_scalar", p.MulScalar(2).Data(), []float32{0.5, 2, 8}, 0)
	near(t, "div_scalar", p.DivScalar(2).Data(), []float32{0.125, 0.5, 2}, 0)
	near(t, "rsub", p.RSubScalar(1).Data(), []float32{0.75, 0, -3}, 0)
	near(t, "rdiv", p.RDivScalar(1).Data(), []float32{4, 1, 0.25}, 0)
	near(t, "neg", p.Neg().Data(), []float32{-0.25, -1, -4}, 0)
	// Aşırı değerlerde sigmoid kararlılığı.
	s := New([]float32{-1000, 1000}, 2).Sigmoid().Data()
	if s[0] != 0 || s[1] != 1 {
		t.Fatalf("sigmoid uç değerler: %v", s)
	}
}

func TestReductions(t *testing.T) {
	x := sorted(2, 3, 4)
	s := x.Sum(1, false)
	shapeEqual(t, "sum", s.Shape(), []int{2, 4})
	near(t, "sum", s.Data(), []float32{15, 18, 21, 24, 51, 54, 57, 60}, 0)
	sk := x.Sum(-1, true)
	shapeEqual(t, "sum keepdim", sk.Shape(), []int{2, 3, 1})
	near(t, "sum son", sk.Data(), []float32{10, 26, 42, 58, 74, 90}, 0)
	near(t, "sum 0", x.Sum(0, false).Slice(0, 0, 1).Data(), []float32{14, 16, 18, 20}, 0)
	near(t, "mean", x.Mean(2, false).Data(), []float32{2.5, 6.5, 10.5, 14.5, 18.5, 22.5}, 1e-6)
	m := New([]float32{3, 1, 4, 1, 5, 9, 2, 6}, 2, 4)
	near(t, "max son", m.Max(1, false).Data(), []float32{4, 9}, 0)
	near(t, "max 0", m.Max(0, true).Data(), []float32{5, 9, 4, 6}, 0)
	shapeEqual(t, "max keepdim", m.Max(0, true).Shape(), []int{1, 4})
	am := m.ArgMax(1)
	if am[0] != 2 || am[1] != 1 {
		t.Fatalf("argmax %v", am)
	}
	if x.SumAll().Item() != 300 || x.MeanAll().Item() != 12.5 {
		t.Fatal("sum_all / mean_all yanlış")
	}
	shapeEqual(t, "sum_all skaler", x.SumAll().Shape(), nil)
	// Bitişik olmayan girdi üzerinde indirgeme.
	near(t, "devrik sum", sorted(2, 3).T().Sum(1, false).Data(), []float32{5, 7, 9}, 0)
}

func TestSoftmax(t *testing.T) {
	x := New([]float32{1, 2, 3, 1000, 1001, 1002}, 2, 3)
	s := x.Softmax(-1).Data()
	e := []float64{math.Exp(-2), math.Exp(-1), 1}
	z := e[0] + e[1] + e[2]
	want := []float32{float32(e[0] / z), float32(e[1] / z), float32(e[2] / z)}
	near(t, "softmax", s[:3], want, 1e-6)
	near(t, "softmax büyük değer kararlı", s[3:], want, 1e-6)
	ls := x.LogSoftmax(-1).Data()
	for i := range ls {
		if math.Abs(float64(ls[i])-math.Log(float64(s[i]))) > 1e-5 {
			t.Fatalf("log_softmax [%d] %v", i, ls[i])
		}
	}
	// Eksen 0 boyunca: her sütun toplamı 1.
	c := x.Softmax(0).Sum(0, false).Data()
	near(t, "softmax eksen 0", c, []float32{1, 1, 1}, 1e-6)
	// -sonsuz maskeli satır.
	inf := float32(math.Inf(-1))
	mk := New([]float32{0, inf, inf, 1, 1, inf}, 2, 3).Softmax(-1).Data()
	near(t, "maskeli softmax", mk, []float32{1, 0, 0, 0.5, 0.5, 0}, 1e-6)
}

func TestMatMulForward(t *testing.T) {
	a := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	b := New([]float32{7, 8, 9, 10, 11, 12}, 3, 2)
	near(t, "2B", MatMul(a, b).Data(), []float32{58, 64, 139, 154}, 0)
	rng := rand.New(rand.NewSource(3))
	// Kenar durumları dahil çeşitli boyutlar (mr/nr/kc katı olmayanlar,
	// paralel eşiğin üstü ve altı).
	for _, d := range [][3]int{{1, 1, 1}, {3, 5, 7}, {4, 4, 4}, {17, 33, 9}, {65, 130, 300}, {200, 150, 517}, {1, 300, 2}} {
		M, N, K := d[0], d[1], d[2]
		x := Randn(rng, M, K)
		y := Randn(rng, K, N)
		near(t, "rastgele", MatMul(x, y).Data(), naiveMatMul(x.Data(), y.Data(), M, N, K), 1e-4)
	}
	// Toplu 3B.
	x := Randn(rng, 3, 5, 6)
	y := Randn(rng, 3, 6, 7)
	z := MatMul(x, y)
	shapeEqual(t, "toplu", z.Shape(), []int{3, 5, 7})
	for bi := 0; bi < 3; bi++ {
		want := naiveMatMul(x.Data()[bi*30:(bi+1)*30], y.Data()[bi*42:(bi+1)*42], 5, 7, 6)
		near(t, "toplu", z.Data()[bi*35:(bi+1)*35], want, 1e-4)
	}
	// (B,T,C) @ (C,D)
	w := Randn(rng, 6, 4)
	l := MatMul(x, w)
	shapeEqual(t, "nd@2b", l.Shape(), []int{3, 5, 4})
	near(t, "nd@2b", l.Data(), naiveMatMul(x.Data(), w.Data(), 15, 4, 6), 1e-4)
	// Devrik görünüm girdisi.
	q := Randn(rng, 7, 5)
	near(t, "devrik girdi", MatMul(q.T(), Randn(rand.New(rand.NewSource(9)), 7, 3)).Data(),
		naiveMatMul(q.T().Data(), Randn(rand.New(rand.NewSource(9)), 7, 3).Data(), 5, 3, 7), 1e-4)
	// 4B toplu (dikkat başlıkları gibi).
	h1 := Randn(rng, 2, 3, 4, 5)
	h2 := Randn(rng, 2, 3, 5, 4)
	shapeEqual(t, "4B", MatMul(h1, h2).Shape(), []int{2, 3, 4, 4})
	expectPanic(t, "iç boyut", func() { MatMul(Zeros(2, 3), Zeros(2, 3)) })
	expectPanic(t, "toplu uyumsuz", func() { MatMul(Zeros(2, 2, 3), Zeros(3, 3, 2)) })
	expectPanic(t, "1B", func() { MatMul(Zeros(3), Zeros(3, 2)) })
}

func TestMatMulLarge(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	M, N, K := 257, 263, 520
	x := Randn(rng, M, K)
	y := Randn(rng, K, N)
	near(t, "büyük", MatMul(x, y).Data(), naiveMatMul(x.Data(), y.Data(), M, N, K), 2e-4)
}

func TestEmbeddingForward(t *testing.T) {
	w := sorted(4, 3)
	e := Embedding(w, []int{2, 0, 2, 3}, 2, 2)
	shapeEqual(t, "embedding", e.Shape(), []int{2, 2, 3})
	near(t, "embedding", e.Data(), []float32{7, 8, 9, 1, 2, 3, 7, 8, 9, 10, 11, 12}, 0)
	expectPanic(t, "indeks dışı", func() { Embedding(w, []int{4}) })
}

func TestCrossEntropyForward(t *testing.T) {
	l := New([]float32{1, 2, 3, 1, 1, 1}, 2, 3)
	ce := CrossEntropy(l, []int{2, 0}).Item()
	ls := l.LogSoftmax(-1).Data()
	want := -(float64(ls[2]) + float64(ls[3])) / 2
	if math.Abs(float64(ce)-want) > 1e-6 {
		t.Fatalf("cross entropy %v, beklenen %v", ce, want)
	}
	// Yok sayılan hedef.
	ce2 := CrossEntropy(l, []int{2, -1}).Item()
	if math.Abs(float64(ce2)+float64(ls[2])) > 1e-6 {
		t.Fatalf("yok sayma %v", ce2)
	}
	// Çok büyük logitlerde kararlılık.
	b := CrossEntropy(New([]float32{1e4, 0}, 1, 2), []int{1}).Item()
	if math.Abs(float64(b)-1e4) > 1 || math.IsInf(float64(b), 0) {
		t.Fatalf("büyük logit %v", b)
	}
	expectPanic(t, "hedef sayısı", func() { CrossEntropy(l, []int{1}) })
	expectPanic(t, "hedef aralığı", func() { CrossEntropy(l, []int{1, 3}) })
}

// Normun çıktı türü (RMSNormAs, LayerNormAs; 15.4): hesap float32'dir; çıktı float32 normun o
// türe dönüşümüyle bit düzeyinde aynıdır, girdinin gradyanı da dönüşüm yolununkiyle aynıdır:
// iki tüketicinin gradyanı float32'de toplanır (dönüşümün gradyanı gibi; node.wideGrad).
func TestNormOutputDType(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	xv, gv := Randn(rng, 3, 40), Randn(rng, 40)
	c1, c2 := Randn(rng, 40, 7), Randn(rng, 40, 5)
	for _, dt := range []DType{BFloat16, Float16} {
		for _, layer := range []bool{false, true} {
			norm := func(x, g *Tensor, out DType) *Tensor {
				if layer {
					return LayerNormAs(x, g, nil, 1e-5, out)
				}
				return RMSNormAs(x, g, 1e-5, out)
			}
			a, ga := xv.Detach().SetRequiresGrad(true), gv.Detach().SetRequiresGrad(true)
			b, gb := xv.Detach().SetRequiresGrad(true), gv.Detach().SetRequiresGrad(true)
			ya, yb := norm(a, ga, dt), norm(b, gb, Float32).Cast(dt)
			if ya.DType() != dt {
				t.Fatalf("çıktının türü %s, beklenen %s", ya.DType(), dt)
			}
			sameBits(t, fmt.Sprintf("%s norm (katman %v)", dt, layer), ya, yb)
			loss := func(y *Tensor) *Tensor {
				return y.MatMul(c1.Cast(dt)).Cast(Float32).SumAll().Add(y.MatMul(c2.Cast(dt)).Cast(Float32).SumAll())
			}
			loss(ya).Backward()
			loss(yb).Backward()
			sameBits(t, "girdinin gradyanı", a.Grad(), b.Grad())
			sameBits(t, "ağırlığın gradyanı", ga.Grad(), gb.Grad())
		}
	}
}

func TestNormsForward(t *testing.T) {
	x := New([]float32{1, 2, 3, 4, 2, 2, 2, 2}, 2, 4)
	ln := LayerNorm(x, nil, nil, 1e-6).Data()
	// [1,2,3,4]: ort 2.5, varyans 1.25
	s := float32(1 / math.Sqrt(1.25))
	near(t, "layernorm", ln[:4], []float32{-1.5 * s, -0.5 * s, 0.5 * s, 1.5 * s}, 1e-5)
	near(t, "layernorm sabit satır", ln[4:], []float32{0, 0, 0, 0}, 1e-5)
	g := New([]float32{1, 2, 1, 2}, 4)
	bt := New([]float32{0, 0, 1, 1}, 4)
	ln2 := LayerNorm(x, g, bt, 1e-6).Data()
	near(t, "layernorm γβ", ln2[:4], []float32{-1.5 * s, -1 * s, 0.5*s + 1, 3*s + 1}, 1e-5)
	rms := RMSNorm(x, nil, 0).Data()
	r := float32(1 / math.Sqrt(7.5))
	near(t, "rmsnorm", rms[:4], []float32{r, 2 * r, 3 * r, 4 * r}, 1e-5)
	near(t, "rmsnorm γ", RMSNorm(x, g, 0).Data()[4:], []float32{1, 2, 1, 2}, 1e-5)
	expectPanic(t, "γ şekli", func() { LayerNorm(x, Ones(3), nil, 1e-5) })
}
