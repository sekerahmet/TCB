package tensor

import (
	"math/rand"
	"testing"
)

// Her türevlenebilir işlem için sayısal gradyan denetimi (merkezi sonlu
// farklar) ve otomatik türev davranış testleri.

type gcDurum struct {
	ad string
	f  func(xs []*Tensor) *Tensor
	xs func(rng *rand.Rand) []*Tensor
}

func girdiler(ts ...*Tensor) []*Tensor { return ts }

func TestGradyanDenetimi(t *testing.T) {
	n := func(rng *rand.Rand, shape ...int) *Tensor { return rastgele(rng, -1, 1, 0, shape...) }
	pozitif := func(rng *rand.Rand, shape ...int) *Tensor { return rastgele(rng, 0.5, 2, 0, shape...) }
	durumlar := []gcDurum{
		// İkili işlemler (aynı şekil ve yayınlama kenar durumları)
		{"add", func(x []*Tensor) *Tensor { return Add(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3), n(r, 2, 3)) }},
		{"add satır yayını", func(x []*Tensor) *Tensor { return Add(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 4)) }},
		{"add dış yayın", func(x []*Tensor) *Tensor { return Add(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 1), n(r, 1, 4)) }},
		{"add skaler tensör", func(x []*Tensor) *Tensor { return Add(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r), n(r, 2, 3)) }},
		{"sub orta eksen yayını", func(x []*Tensor) *Tensor { return Sub(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 1, 3), n(r, 4, 1)) }},
		{"mul", func(x []*Tensor) *Tensor { return Mul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 4), n(r, 3, 4)) }},
		{"mul yayın", func(x []*Tensor) *Tensor { return Mul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 3, 1)) }},
		{"mul kendisiyle", func(x []*Tensor) *Tensor { return Mul(x[0], x[0]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 5)) }},
		{"div", func(x []*Tensor) *Tensor { return Div(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3), pozitif(r, 2, 3)) }},
		{"div yayın", func(x []*Tensor) *Tensor { return Div(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3), pozitif(r, 2, 1)) }},
		{"mul devrik görünüm", func(x []*Tensor) *Tensor { return Mul(x[0].T(), x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 2), n(r, 2, 3)) }},
		// Skalerle işlemler
		{"add_scalar", func(x []*Tensor) *Tensor { return x[0].AddScalar(1.5) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		{"sub_scalar", func(x []*Tensor) *Tensor { return x[0].SubScalar(1.5) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		{"mul_scalar", func(x []*Tensor) *Tensor { return x[0].MulScalar(-2.5) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		{"div_scalar", func(x []*Tensor) *Tensor { return x[0].DivScalar(4) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		{"rsub_scalar", func(x []*Tensor) *Tensor { return x[0].RSubScalar(3) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		{"rdiv_scalar", func(x []*Tensor) *Tensor { return x[0].RDivScalar(2) }, func(r *rand.Rand) []*Tensor { return girdiler(pozitif(r, 4)) }},
		{"neg", func(x []*Tensor) *Tensor { return x[0].Neg() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4)) }},
		// Tekli işlemler
		{"exp", func(x []*Tensor) *Tensor { return x[0].Exp() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"log", func(x []*Tensor) *Tensor { return x[0].Log() }, func(r *rand.Rand) []*Tensor { return girdiler(pozitif(r, 2, 3)) }},
		{"sqrt", func(x []*Tensor) *Tensor { return x[0].Sqrt() }, func(r *rand.Rand) []*Tensor { return girdiler(pozitif(r, 2, 3)) }},
		{"pow 2", func(x []*Tensor) *Tensor { return x[0].Pow(2) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 5)) }},
		{"pow 3", func(x []*Tensor) *Tensor { return x[0].Pow(3) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 5)) }},
		{"pow -1.5", func(x []*Tensor) *Tensor { return x[0].Pow(-1.5) }, func(r *rand.Rand) []*Tensor { return girdiler(pozitif(r, 5)) }},
		{"square", func(x []*Tensor) *Tensor { return x[0].Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 5)) }},
		{"tanh", func(x []*Tensor) *Tensor { return x[0].Tanh() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"relu", func(x []*Tensor) *Tensor { return x[0].Relu() }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -1, 1, 0.05, 2, 3)) }},
		{"gelu", func(x []*Tensor) *Tensor { return x[0].Gelu() }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -3, 3, 0, 2, 4)) }},
		{"sigmoid", func(x []*Tensor) *Tensor { return x[0].Sigmoid() }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -3, 3, 0, 2, 4)) }},
		{"tekli devrik girdi", func(x []*Tensor) *Tensor { return x[0].T().Tanh() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 2)) }},
		// İndirgemeler
		{"sum eksen 0", func(x []*Tensor) *Tensor { return x[0].Sum(0, false) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 4)) }},
		{"sum orta eksen keepdim", func(x []*Tensor) *Tensor { return x[0].Sum(1, true) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4)) }},
		{"sum son eksen", func(x []*Tensor) *Tensor { return x[0].Sum(-1, false) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4)) }},
		{"mean", func(x []*Tensor) *Tensor { return x[0].Mean(1, false) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4)) }},
		{"mean keepdim", func(x []*Tensor) *Tensor { return x[0].Mean(0, true) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 2)) }},
		{"sum_all", func(x []*Tensor) *Tensor { return x[0].SumAll() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"mean_all", func(x []*Tensor) *Tensor { return x[0].MeanAll() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"max son eksen", func(x []*Tensor) *Tensor { return x[0].Max(-1, false) }, func(r *rand.Rand) []*Tensor { return girdiler(ayrik(r, 3, 4)) }},
		{"max eksen 0 keepdim", func(x []*Tensor) *Tensor { return x[0].Max(0, true) }, func(r *rand.Rand) []*Tensor { return girdiler(ayrik(r, 3, 4)) }},
		{"max orta eksen", func(x []*Tensor) *Tensor { return x[0].Max(1, false) }, func(r *rand.Rand) []*Tensor { return girdiler(ayrik(r, 2, 3, 2)) }},
		// Softmax
		{"softmax son", func(x []*Tensor) *Tensor { return x[0].Softmax(-1) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 5)) }},
		{"softmax eksen 0", func(x []*Tensor) *Tensor { return x[0].Softmax(0) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 2, 2)) }},
		{"log_softmax son", func(x []*Tensor) *Tensor { return x[0].LogSoftmax(-1) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 5)) }},
		{"log_softmax orta", func(x []*Tensor) *Tensor { return x[0].LogSoftmax(1) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 4, 3)) }},
		// Matris çarpımı
		{"matmul 2B", func(x []*Tensor) *Tensor { return MatMul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 5), n(r, 5, 4)) }},
		{"matmul kenar boyutlar", func(x []*Tensor) *Tensor { return MatMul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 7, 9), n(r, 9, 6)) }},
		{"matmul ND@2B", func(x []*Tensor) *Tensor { return MatMul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 4, 5)) }},
		{"matmul toplu 3B", func(x []*Tensor) *Tensor { return MatMul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 2, 4, 5)) }},
		{"matmul devrik görünümler", func(x []*Tensor) *Tensor { return MatMul(x[0].T(), x[1].T()) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4, 3), n(r, 5, 4)) }},
		{"matmul toplu 4B", func(x []*Tensor) *Tensor { return MatMul(x[0], x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 2, 3, 4), n(r, 2, 2, 4, 3)) }},
		// Görünümler
		{"reshape", func(x []*Tensor) *Tensor { return x[0].Reshape(3, -1).Mul(x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 6), n(r, 3, 4)) }},
		{"permute", func(x []*Tensor) *Tensor { return x[0].Permute(2, 0, 1).Mul(x[1]) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 4, 2, 3)) }},
		{"transpose", func(x []*Tensor) *Tensor { return x[0].Transpose(0, 1).Exp() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"slice", func(x []*Tensor) *Tensor { return x[0].Slice(1, 1, 3).Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 4, 3)) }},
		{"slice adımlı", func(x []*Tensor) *Tensor { return x[0].SliceStep(0, 0, 5, 2).Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 5, 2)) }},
		{"görünüm zinciri", func(x []*Tensor) *Tensor {
			return x[0].Permute(1, 0, 2).Slice(2, 1, 3).Transpose(0, 1).Reshape(-1).Square()
		}, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4)) }},
		{"contiguous", func(x []*Tensor) *Tensor { return x[0].T().Contiguous().Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"devrik reshape", func(x []*Tensor) *Tensor { return x[0].T().Reshape(6).Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3)) }},
		{"clone", func(x []*Tensor) *Tensor { return x[0].Clone().Square() }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3)) }},
		{"unsqueeze squeeze", func(x []*Tensor) *Tensor { return x[0].Unsqueeze(0).Mul(x[1]).Squeeze(0) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3), n(r, 1, 3)) }},
		// Sinir ağı işlemleri
		{"embedding", func(x []*Tensor) *Tensor { return Embedding(x[0], []int{1, 3, 1, 0, 3, 3}, 2, 3) }, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 4, 3)) }},
		{"cross_entropy", func(x []*Tensor) *Tensor { return CrossEntropy(x[0], []int{2, 0, 4, 1}) }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -2, 2, 0, 4, 5)) }},
		{"cross_entropy 3B yok sayma", func(x []*Tensor) *Tensor { return CrossEntropy(x[0], []int{2, -1, 0, 3, -1, 1}) }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -2, 2, 0, 2, 3, 4)) }},
		{"layer_norm γβ", func(x []*Tensor) *Tensor { return LayerNorm(x[0], x[1], x[2], 1e-5) }, func(r *rand.Rand) []*Tensor {
			return girdiler(rastgele(r, -2, 2, 0, 3, 5), n(r, 5), n(r, 5))
		}},
		{"layer_norm yalın", func(x []*Tensor) *Tensor { return LayerNorm(x[0], nil, nil, 1e-5) }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -2, 2, 0, 2, 2, 4)) }},
		{"rms_norm γ", func(x []*Tensor) *Tensor { return RMSNorm(x[0], x[1], 1e-6) }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -2, 2, 0.2, 3, 5), n(r, 5)) }},
		{"rms_norm yalın", func(x []*Tensor) *Tensor { return RMSNorm(x[0], nil, 1e-6) }, func(r *rand.Rand) []*Tensor { return girdiler(rastgele(r, -2, 2, 0.2, 2, 4)) }},
		// Bileşik: küçük bir dikkat bloğu (softmax(QKᵀ/√d)·V) ve iki katmanlı ağ.
		{"dikkat bloğu", func(x []*Tensor) *Tensor {
			q, k, v := x[0], x[1], x[2]
			s := MatMul(q, k.Transpose(-2, -1)).MulScalar(0.5).Softmax(-1)
			return MatMul(s, v)
		}, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 2, 3, 4), n(r, 2, 3, 4), n(r, 2, 3, 4)) }},
		{"iki katmanlı ağ", func(x []*Tensor) *Tensor {
			h := Add(MatMul(x[0], x[1]), x[2]).Gelu()
			return CrossEntropy(MatMul(LayerNorm(h, nil, nil, 1e-5), x[3]), []int{0, 2, 1})
		}, func(r *rand.Rand) []*Tensor { return girdiler(n(r, 3, 4), n(r, 4, 5), n(r, 5), n(r, 5, 3)) }},
	}
	for i, d := range durumlar {
		d := d
		rng := rand.New(rand.NewSource(int64(100 + i)))
		t.Run(d.ad, func(t *testing.T) {
			gradDenetle(t, d.ad, d.f, d.xs(rng)...)
		})
	}
}

// ayrik birbirinden en az 0.1 uzak değerlerle tensör (Max eşitliklerinden kaçınmak için).
func ayrik(rng *rand.Rand, shape ...int) *Tensor {
	n := numelOf(shape)
	p := rng.Perm(n)
	d := make([]float32, n)
	for i, v := range p {
		d[i] = float32(v)*0.1 - float32(n)*0.05
	}
	return New(d, shape...)
}

func TestGradyanBirikimiVeSifirlama(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3).SetRequiresGrad(true)
	// Elmas çizge: x iki yoldan kullanılır; gradyanlar toplanmalı.
	y := Add(x.MulScalar(2), x.Square()).SumAll()
	y.Backward()
	yakin(t, "elmas", x.Grad().Data(), []float32{4, 6, 8}, 0)
	// İkinci geri yayılım birikir.
	x.Square().SumAll().Backward()
	yakin(t, "birikim", x.Grad().Data(), []float32{6, 10, 14}, 0)
	x.ZeroGrad()
	yakin(t, "sıfırlama", x.Grad().Data(), []float32{0, 0, 0}, 0)
	ZeroGrads([]*Tensor{x})
	// Gradyan istemeyen girdinin gradyanı oluşmaz.
	c := New([]float32{1, 1, 1}, 3)
	Mul(x, c).SumAll().Backward()
	if c.Grad() != nil {
		t.Fatal("gradyan istemeyen tensörde gradyan oluştu")
	}
	if !x.IsLeaf() || Mul(x, c).IsLeaf() {
		t.Fatal("IsLeaf yanlış")
	}
}

func TestRequiresGradYayilimi(t *testing.T) {
	a := Ones(2)
	b := Ones(2).SetRequiresGrad(true)
	if Add(a, a).RequiresGrad() {
		t.Fatal("gradyansız girdilerden gradyanlı çıktı")
	}
	if !Add(a, b).RequiresGrad() {
		t.Fatal("gradyan bayrağı yayılmadı")
	}
	if b.Detach().RequiresGrad() {
		t.Fatal("Detach gradyan bayrağını taşıdı")
	}
	panikBekle(t, "gradyansız geri yayılım", func() { Add(a, a).SumAll().Backward() })
	panikBekle(t, "skaler olmayan", func() { Add(a, b).Backward() })
	panikBekle(t, "ara tensörde kapatma", func() { Add(a, b).SetRequiresGrad(false) })
}

func TestNoGrad(t *testing.T) {
	w := Ones(3).SetRequiresGrad(true)
	var y *Tensor
	NoGrad(func() {
		if GradEnabled() {
			t.Fatal("NoGrad içinde türev açık")
		}
		NoGrad(func() {}) // iç içe
		if GradEnabled() {
			t.Fatal("iç içe NoGrad sonrası türev açıldı")
		}
		y = w.MulScalar(2).SumAll()
	})
	if !GradEnabled() {
		t.Fatal("NoGrad sonrası türev kapalı kaldı")
	}
	if y.RequiresGrad() || !y.IsLeaf() {
		t.Fatal("NoGrad içinde çizge kuruldu")
	}
	if y.Item() != 6 {
		t.Fatal("NoGrad sonucu yanlış")
	}
}

func TestDerinCizgeOzyinelemesiz(t *testing.T) {
	// Özyinelemeli topolojik sıralama bu derinlikte yığını zorlardı.
	x := Ones(1).SetRequiresGrad(true)
	y := x
	for i := 0; i < 100000; i++ {
		y = y.AddScalar(0)
	}
	y.MulScalar(3).SumAll().Backward()
	if x.Grad().Item() != 3 {
		t.Fatalf("derin çizge gradyanı %v", x.Grad().Item())
	}
}

func TestBackwardWithTohum(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	y := x.Square()
	y.BackwardWith(New([]float32{1, 10}, 2))
	yakin(t, "tohum", x.Grad().Data(), []float32{2, 40}, 0)
	panikBekle(t, "tohum şekli", func() { x.Square().BackwardWith(Ones(3)) })
}

func TestCihazAPI(t *testing.T) {
	x := Ones(2, 2)
	if x.Device() != CPU || DefaultDevice() != CPU || !HasDevice(CPU) {
		t.Fatal("varsayılan cihaz CPU olmalı")
	}
	if x.To(CPU) != x {
		t.Fatal("aynı cihaza taşıma kopyalamamalı")
	}
	if HasDevice(GPU(0)) {
		t.Fatal("GPU arka ucu kayıtlı olmamalı")
	}
	panikBekle(t, "kayıtsız cihaz", func() { x.To(GPU(0)) })
	if GPU(1).String() != "gpu:1" || CPU.String() != "cpu" {
		t.Fatal("cihaz adları yanlış")
	}
	if ZerosOn(CPU, 3).Numel() != 3 || NewOn(CPU, []float32{1}, 1).Item() != 1 {
		t.Fatal("cihazlı oluşturucular yanlış")
	}
}

// ikinciCPU, cihazlar arası taşıma yolunu sınamak için CPU arka ucunu farklı
// bir cihaz kimliğiyle sarar.
type ikinciCPU struct{ cpuBackend }

func (ikinciCPU) Device() Device { return GPU(99) }

func TestCihazlarArasiTasimaTurevi(t *testing.T) {
	RegisterBackend(ikinciCPU{})
	defer func() {
		backendMu.Lock()
		delete(backends, GPU(99))
		backendMu.Unlock()
	}()
	x := New([]float32{1, 2, 3}, 3).SetRequiresGrad(true)
	y := x.To(GPU(99))
	if y.Device() != GPU(99) {
		t.Fatal("taşıma cihazı yanlış")
	}
	panikBekle(t, "karışık cihaz", func() { Add(x, y) })
	y.Square().SumAll().Backward()
	yakin(t, "taşıma gradyanı", x.Grad().Data(), []float32{2, 4, 6}, 0)
}
