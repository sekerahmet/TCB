package tensor

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Veri türü (bfloat16/float16) ve sürüm denetimi testleri.

func f32(b uint32) float32 { return math.Float32frombits(b) }

func TestBF16ConversionEdges(t *testing.T) {
	cases := []struct {
		x    float32
		want uint16
	}{
		{0, 0x0000},
		{float32(math.Copysign(0, -1)), 0x8000},
		{1, 0x3F80},
		{-2, 0xC000},
		{1 + 1.0/256, 0x3F80},               // tam ortada: çifte (aşağı)
		{1 + 3.0/256, 0x3F82},               // tam ortada: çifte (yukarı)
		{1 + 1.0/256 + 1.0/(1<<20), 0x3F81}, // ortanın biraz üstü
		{f32(0x7F7FFFFF), 0x7F80},           // float32 en büyük → +sonsuz
		{-f32(0x7F7FFFFF), 0xFF80},
		{f32(0x7F7F0000), 0x7F7F}, // bfloat16 en büyük sonlu
		{float32(math.Inf(1)), 0x7F80},
		{float32(math.Inf(-1)), 0xFF80},
		{f32(0x00010000), 0x0001}, // normal altı korunur
		{f32(0x00008000), 0x0000}, // en küçük normal altının yarısı: çifte (0)
		{f32(0x00018000), 0x0002}, // 1.5 normal altı: çifte (2)
	}
	for _, d := range cases {
		if got := f32ToBF16(d.x); got != d.want {
			t.Fatalf("bf16(%v) = %#04x, beklenen %#04x", d.x, got, d.want)
		}
	}
	for _, nan := range []float32{float32(math.NaN()), f32(0x7F800001), f32(0xFFC00000), f32(0x7F80FFFF)} {
		h := f32ToBF16(nan)
		if !math.IsNaN(float64(bf16ToF32(h))) || h&0x0040 == 0 {
			t.Fatalf("bf16(NaN %#08x) = %#04x, sessiz NaN beklenen", math.Float32bits(nan), h)
		}
	}
}

func TestF16ConversionEdges(t *testing.T) {
	cases := []struct {
		x    float32
		want uint16
	}{
		{0, 0x0000},
		{float32(math.Copysign(0, -1)), 0x8000},
		{1, 0x3C00},
		{-2, 0xC000},
		{65504, 0x7BFF},                           // en büyük sonlu
		{65519, 0x7BFF},                           // ortanın altı
		{65520, 0x7C00},                           // tam ortada: çifte (sonsuz)
		{1e6, 0x7C00},                             // taşma
		{-1e6, 0xFC00},                            //
		{1 + 1.0/2048, 0x3C00},                    // tam ortada: çifte (aşağı)
		{1 + 3.0/2048, 0x3C02},                    // tam ortada: çifte (yukarı)
		{6.103515625e-05, 0x0400},                 // 2^-14 en küçük normal
		{5.960464477539063e-08, 0x0001},           // 2^-24 en küçük normal altı
		{2.9802322387695312e-08, 0x0000},          // 2^-25: tam ortada, çifte (0)
		{2.9802322387695312e-08 * 1.0001, 0x0001}, // ortanın biraz üstü
		{4.470348358154297e-08, 0x0001},           // 1.5·2^-25 → 2^-24 (1.5 ulp değil: 0.75)
		{8.940696716308594e-08, 0x0002},           // 1.5·2^-24: tam ortada, çifte (2)
		{6.097555160522461e-05, 0x03FF},           // en büyük normal altı (1023·2^-24)
		{6.1005353927612305e-05, 0x0400},          // 1023.5·2^-24: tam ortada, çifte (en küçük normal)
		{1e-10, 0x0000},
		{-1e-10, 0x8000},
		{float32(math.Inf(1)), 0x7C00},
		{float32(math.Inf(-1)), 0xFC00},
	}
	for _, d := range cases {
		if got := f32ToF16(d.x); got != d.want {
			t.Fatalf("f16(%v) = %#04x, beklenen %#04x", d.x, got, d.want)
		}
	}
	for _, nan := range []float32{float32(math.NaN()), f32(0x7F800001), f32(0xFFC00000)} {
		if !math.IsNaN(float64(f16ToF32(f32ToF16(nan)))) {
			t.Fatalf("f16(NaN) NaN kalmalı")
		}
	}
}

// Her 16 bitlik değer float32'ye açılıp geri dönüştürüldüğünde aynı kalmalı
// (NaN hariç) ve rastgele float32 değerler en yakın (eşitlikte çift) 16 bitlik
// değere yuvarlanmalı.
func TestSixteenBitRounding(t *testing.T) {
	type format struct {
		ad     string
		narrow func(float32) uint16
		ac     func(uint16) float32
	}
	for _, b := range []format{{"bfloat16", f32ToBF16, bf16ToF32}, {"float16", f32ToF16, f16ToF32}} {
		for h := 0; h < 1<<16; h++ {
			v := b.ac(uint16(h))
			if math.IsNaN(float64(v)) {
				continue
			}
			if g := b.narrow(v); g != uint16(h) {
				t.Fatalf("%s: %#04x → %v → %#04x", b.ad, h, v, g)
			}
		}
		rng := rand.New(rand.NewSource(51))
		for i := 0; i < 300000; i++ {
			x := f32(rng.Uint32())
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				continue
			}
			h := b.narrow(x)
			r := float64(b.ac(h))
			if math.IsInf(r, 0) {
				continue // taşma: sınır testleri ayrıca
			}
			// Komşular (aynı işaret yönünde bir alt ve bir üst 16 bitlik değer).
			for _, k := range []uint16{h - 1, h + 1} {
				n := float64(b.ac(k))
				if math.IsNaN(n) || math.IsInf(n, 0) || (k&0x7FFF) == 0x7FFF || (h&0x7FFF) == 0 && k == h-1 {
					continue
				}
				dr, dn := math.Abs(float64(x)-r), math.Abs(float64(x)-n)
				if dn < dr || (dn == dr && h&1 == 1) {
					t.Fatalf("%s: %v → %v (%#04x), komşu %v daha yakın/çift", b.ad, x, r, h, n)
				}
			}
		}
	}
}

func TestTypedTensors(t *testing.T) {
	data := []float32{1, 2.5, -3.140625, 1e-3, 65504, 300}
	for _, dt := range []DType{BFloat16, Float16} {
		x := NewOf(dt, data, 2, 3)
		if x.DType() != dt || x.storage.Len() != 6 {
			t.Fatalf("%s: tür %s", dt, x.DType())
		}
		for i, v := range x.Data() {
			if math.Abs(float64(v-data[i])) > 1e-2*math.Abs(float64(data[i]))+1e-6 {
				t.Fatalf("%s: [%d] = %v, beklenen ~%v", dt, i, v, data[i])
			}
		}
		// Görünümler, dönüşüm ve tür kuralları.
		y := x.T().Contiguous()
		if y.DType() != dt || y.At(2, 1) != x.At(1, 2) {
			t.Fatalf("%s: devrik kopya", dt)
		}
		if Add(x, x).DType() != dt || Add(x, Ones(2, 3)).DType() != Float32 {
			t.Fatalf("%s: ikili işlem tür kuralı", dt)
		}
		if x.Cast(Float32).DType() != Float32 || x.Cast(dt) != x {
			t.Fatalf("%s: Cast", dt)
		}
		if !strings.Contains(x.String(), dt.String()) {
			t.Fatalf("%s: String türü göstermeli: %s", dt, x)
		}
		x.Set(7, 0, 0)
		if x.At(0, 0) != 7 {
			t.Fatalf("%s: Set", dt)
		}
	}
	expectPanic(t, "int8 CPU'da desteklenmez", func() { ZerosOf(Int8, 4) })
	if Int4.StorageBytes(5) != 3 || Int8.Bits() != 8 || BFloat16.String() != "bfloat16" {
		t.Fatal("tür yardımcıları")
	}
}

// Cast türevlenebilir; 16 bitlik tensörlerin gradyanı varsayılan olarak float32'dir.
func TestCastGrad(t *testing.T) {
	x := New([]float32{0.5, -1.25, 2, 3}, 4).SetRequiresGrad(true)
	h := x.Cast(BFloat16)
	k := h.Square().Cast(Float32).SumAll()
	k.Backward()
	near(t, "cast gradyanı", x.Grad().Data(), []float32{1, -2.5, 4, 6}, 1e-2)

	w := NewOf(BFloat16, []float32{1, 2, 3, 4}, 2, 2).SetRequiresGrad(true)
	MatMul(w, w).Cast(Float32).SumAll().Backward()
	if w.n.grad.DType() != Float32 {
		t.Fatalf("bf16 parametrenin gradyanı float32 olmalı, %s", w.n.grad.DType())
	}
	near(t, "bf16 matmul gradyanı", w.Grad().Data(), []float32{7, 11, 9, 13}, 1e-2)

	SetGradDType(BFloat16)
	defer SetGradDType(Float32)
	w.n.grad = nil
	MatMul(w, w).Cast(Float32).SumAll().Backward()
	if w.n.grad.DType() != BFloat16 {
		t.Fatalf("SetGradDType(BFloat16) sonrası gradyan bf16 olmalı, %s", w.n.grad.DType())
	}
	near(t, "bf16 gradyan", w.Grad().Data(), []float32{7, 11, 9, 13}, 1e-2)
}

// Gradyan türü kuralı: 16 bitlik ara sonucun gradyanı kendi türünde; dönüşüm ve
// görünümler girdinin gradyan türünü izler; yaprakta GradDType.
func TestIntermediateGradDType(t *testing.T) {
	w := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	x := NewOf(BFloat16, []float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	wc := w.Cast(BFloat16)
	y := x.Mul(x)
	cases := []struct {
		name string
		t    *Tensor
		want DType
	}{
		{"float32 yaprak", w, Float32},
		{"bf16 yaprak", x, GradDType()},
		{"float32'nin dönüşümü", wc, Float32},
		{"dönüşümün devriği", wc.T(), Float32},
		{"dönüşümün kopyası", wc.T().Contiguous().Reshape(6).Slice(0, 1, 4), Float32},
		{"bf16 yaprağın görünümü", x.Reshape(3, 2), GradDType()},
		{"bf16 ara sonuç", y, BFloat16},
		{"bf16 ara sonucun görünümü", y.T().Clone(), BFloat16},
		{"bf16 ara sonucun float32 dönüşümü", y.Cast(Float32), Float32},
	}
	for _, c := range cases {
		if got := c.t.n.gradDType(); got != c.want {
			t.Errorf("%s: gradyan türü %s, %s bekleniyordu", c.name, got, c.want)
		}
	}
}

// float32 ağırlığın bf16 dönüşümüyle yapılan çarpımda ağırlık gradyanı bf16'ya
// yuvarlanmadan float32 yazılır: 1 + 2⁻⁸ bf16'da gösterilemez (1'e yuvarlanırdı).
func TestCastWeightGradStaysFloat32(t *testing.T) {
	x := NewOf(BFloat16, []float32{1, 1.0 / 256}, 2, 1)
	w := New([]float32{3}, 1, 1).SetRequiresGrad(true)
	out := MatMul(x, w.Cast(BFloat16))
	if out.DType() != BFloat16 {
		t.Fatalf("çıktı türü %s", out.DType())
	}
	out.Cast(Float32).SumAll().Backward()
	if g := w.Grad().Data()[0]; g != 1+1.0/256 {
		t.Fatalf("ağırlık gradyanı %v, %v bekleniyordu (bf16'ya yuvarlanmamalı)", g, 1+1.0/256)
	}
}

// CastShared bir ara sonucun aynı dönüşümünü yeniden kullanır; kaynak ya da sonuç
// yerinde değişince, yaprakta ve türevsiz hesapta yeniden dönüştürür. Paylaşılan
// dönüşümün gradyanı her kullanımdan birikir.
func TestCastShared(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3).SetRequiresGrad(true)
	h := x.Mul(x)
	a, b := h.CastShared(BFloat16), h.CastShared(BFloat16)
	if a != b {
		t.Fatal("aynı ara sonucun dönüşümü paylaşılmadı")
	}
	if c := h.CastShared(Float16); c == a || c.DType() != Float16 {
		t.Fatal("başka türe dönüşüm paylaşılmamalı")
	}
	if x.CastShared(BFloat16) == x.CastShared(BFloat16) {
		t.Fatal("yaprağın dönüşümü paylaşılmamalı")
	}
	var n1, n2 *Tensor
	NoGrad(func() { n1, n2 = h.CastShared(BFloat16), h.CastShared(BFloat16) })
	if n1 == n2 || n1.RequiresGrad() {
		t.Fatal("türevsiz hesapta dönüşüm paylaşılmamalı")
	}
	a2 := h.CastShared(BFloat16)
	a2.Set(7, 0)
	if h.CastShared(BFloat16) == a2 {
		t.Fatal("yerinde değişen dönüşüm yeniden kullanıldı")
	}
	h3 := h.CastShared(BFloat16)
	h.Set(5, 1)
	if h.CastShared(BFloat16) == h3 {
		t.Fatal("kaynak değişince dönüşüm yeniden yapılmalı")
	}

	y := x.Mul(x)
	p, q := y.CastShared(BFloat16), y.CastShared(BFloat16)
	p.Cast(Float32).SumAll().Add(q.Cast(Float32).SumAll()).Backward()
	near(t, "paylaşılan dönüşümün gradyanı", x.Grad().Data(), []float32{4, 8, 12}, 0)
	if y.n.castMemo.Load() != nil {
		t.Fatal("geri yayılımdan sonra saklanan dönüşüm bırakılmalı")
	}
}

func TestBF16MatMul(t *testing.T) {
	rng := rand.New(rand.NewSource(52))
	a := Randn(rng, 3, 64, 96)
	b := Randn(rng, 3, 96, 48)
	ref := MatMul(a, b).Data()
	for _, dt := range []DType{BFloat16, Float16} {
		ab, bb := a.Cast(dt), b.Cast(dt)
		for _, out := range []*Tensor{MatMul(ab, bb), MatMulOut(ab, bb, Float32), MatMul(ab.T().Contiguous().T(), bb)} {
			got := out.Data()
			var failure, norm float64
			for i := range ref {
				d := float64(got[i] - ref[i])
				failure += d * d
				norm += float64(ref[i]) * float64(ref[i])
			}
			if r := math.Sqrt(failure / norm); r > 1e-2 {
				t.Fatalf("%s matmul (çıktı %s) göreli hata %v", dt, out.DType(), r)
			}
		}
	}
}

// Aynı küçük ağ bf16 (ve fp16) parametrelerle float32 ana kopyalı AdamW ile
// eğitildiğinde float32 eğitimine yakın kayba ulaşmalı.
func TestBF16Training(t *testing.T) {
	train := func(dt DType, master bool) float32 {
		rng := rand.New(rand.NewSource(53))
		X := Randn(rng, 128, 8)
		wActual := Randn(rng, 8, 4)
		target := make([]int, 128)
		for i, s := range MatMul(X, wActual).ArgMax(-1) {
			target[i] = s
		}
		W1 := Randn(rng, 8, 32).MulScalar(0.3).Cast(dt).Detach().SetRequiresGrad(true)
		b1 := ZerosOf(dt, 32).SetRequiresGrad(true)
		W2 := Randn(rng, 32, 4).MulScalar(0.3).Cast(dt).Detach().SetRequiresGrad(true)
		g := Ones(32).Cast(dt).Detach().SetRequiresGrad(true)
		Xd := X.Cast(dt)
		opt := NewAdamW([]*Tensor{W1, b1, W2, g}, 0.02, 0.01)
		opt.SetMasterWeights(master)
		var lossVal float32
		for step := 0; step < 200; step++ {
			opt.ZeroGrad()
			h := RMSNorm(Add(MatMul(Xd, W1), b1).Gelu(), g, 1e-5)
			l := CrossEntropy(MatMulOut(h, W2, Float32), target)
			lossVal = l.Item()
			l.Backward()
			opt.Step()
		}
		return lossVal
	}
	k32 := train(Float32, true)
	for _, dt := range []DType{BFloat16, Float16} {
		k := train(dt, true)
		if k32 > 0.2 || math.Abs(float64(k-k32)) > 0.05+0.5*float64(k32) {
			t.Fatalf("%s eğitim kaybı %v, float32 %v", dt, k, k32)
		}
	}
	if k := train(BFloat16, false); math.IsNaN(float64(k)) || k > 1 {
		t.Fatalf("ana kopyasız bf16 eğitim kaybı %v", k)
	}
}

func TestVersionCheck(t *testing.T) {
	rng := rand.New(rand.NewSource(54))
	X := Randn(rng, 8, 4)
	W := Randn(rng, 4, 3).SetRequiresGrad(true)
	opt := NewSGD([]*Tensor{W}, 0.1, 0)
	target := []int{0, 1, 2, 0, 1, 2, 0, 1}

	// Doğru sıra: ileri → geri → adım.
	for i := 0; i < 3; i++ {
		opt.ZeroGrad()
		CrossEntropy(MatMul(X, W), target).Backward()
		opt.Step()
	}
	// Yanlış sıra: ileri → adım → geri.
	opt.ZeroGrad()
	CrossEntropy(MatMul(X, W), target).Backward()
	l := CrossEntropy(MatMul(X, W), target)
	v := W.Version()
	opt.Step()
	if W.Version() == v {
		t.Fatal("iyileştirici adımı sürümü artırmalı")
	}
	panicCode(t, "sürüm hatası", messages.H3012, func() { l.Backward() })

	// Set de yerinde değişikliktir; görünümler depolamayı (ve sürümü) paylaşır.
	a := Randn(rng, 3).SetRequiresGrad(true)
	y := a.Exp() // exp çıktıyı kaydeder; girdi a'yı da
	a.Detach().Set(5, 1)
	expectPanic(t, "Set sonrası geri yayılım", func() { y.SumAll().Backward() })

	// Geri yayılımın okumadığı girdi (toplama) değişirse hata yok.
	b := Randn(rng, 3).SetRequiresGrad(true)
	z := Add(b, Ones(3))
	b.Set(1, 0)
	z.SumAll().Backward()
	near(t, "toplama gradyanı", b.Grad().Data(), []float32{1, 1, 1}, 0)

	// AdamW ve 16 bitlik ana kopyalı parametreler de sürümü artırır.
	p := NewOf(BFloat16, []float32{1, 2}, 2).SetRequiresGrad(true)
	q := Mul(p, p)
	opt2 := NewAdamW([]*Tensor{p}, 0.1, 0)
	p.n.accGrad()
	opt2.Step()
	expectPanic(t, "AdamW sonrası geri yayılım", func() { q.Cast(Float32).SumAll().Backward() })
}

// Bits16/FromBits16: 16 bitlik tensör değerleri bit desenine kayıpsız gidip gelir.
func TestBits16RoundTrip(t *testing.T) {
	for _, dt := range []DType{BFloat16, Float16} {
		x := NewOf(dt, []float32{0.1, -2.5, 65504, 1e-5, 0, 3.14159}, 6)
		for i, v := range x.Data() {
			if g := FromBits16(dt, Bits16(dt, v)); g != v {
				t.Errorf("%s[%d]: %v → %v", dt, i, v, g)
			}
		}
	}
	if Bits16(BFloat16, 1) != 0x3F80 || Bits16(Float16, 1) != 0x3C00 {
		t.Errorf("1.0 bit deseni: %#x %#x", Bits16(BFloat16, 1), Bits16(Float16, 1))
	}
	defer func() {
		if recover() == nil {
			t.Error("float32 için panik bekleniyordu")
		}
	}()
	Bits16(Float32, 1)
}

// ============================================================================
// Gradyan kapısı: tür dönüşümü
// ============================================================================

func init() {
	// Dönüşüm gradyanı olduğu gibi geçirir; ileri değer hedef türe yuvarlanır (bf16: 2⁻⁹,
	// f16: 2⁻¹² göreli), bu yüzden ileri tolerans 1e-2'dir.
	cast := func(name string, dt DType) gradSpec {
		return gradSpec{name: name, ops: []string{"cast"}, fwdTol: 1e-2,
			build:  func(xs []*Tensor) *Tensor { return xs[0].Cast(dt).Cast(Float32) },
			inputs: genInputs(gen(-1, 1, 0, 3, 4)), ref: identity64}
	}
	registerGrad("dtype", cast("cast bf16", BFloat16), cast("cast f16", Float16))
}

func TestGradCast(t *testing.T) { runGradGroup(t, "dtype") }

// TestTensorStringIsDeveloperText: Tensor.String geliştirici gösterimidir ve İngilizcedir;
// bilinmeyen tür numarası da İngilizce yedek adla yazılır.
func TestTensorStringIsDeveloperText(t *testing.T) {
	x := NewOf(BFloat16, []float32{1, 2, 3, 4}, 2, 2)
	s := x.String()
	if !strings.HasPrefix(s, "Tensor(shape=[2 2], device=") || !strings.Contains(s, "dtype=bfloat16") {
		t.Errorf("beklenmeyen gösterim: %q", s)
	}
	if got := DType(200).String(); got != "dtype(200)" {
		t.Errorf("bilinmeyen tür adı = %q", got)
	}
}
