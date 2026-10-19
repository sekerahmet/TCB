package tensor

import (
	"math/rand"
	"strings"
	"testing"
)

// kareOzel x² işleminin dış düğümle yazılmış biçimi: ileri türevsiz, geri 2·x·g.
func kareOzel(x *Tensor) *Tensor {
	var y *Tensor
	NoGrad(func() { y = x.Mul(x) })
	return CustomOp("kare", []*Tensor{x}, []*Tensor{y}, []*Tensor{x}, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{g[0].Mul(x).MulScalar(2)}
	})[0]
}

func TestCustomOpKareBilesikleAyni(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x1 := Randn(rng, 3, 4).SetRequiresGrad(true)
	x2 := x1.Detach().Clone().SetRequiresGrad(true)
	kareOzel(x1).Mul(x1).SumAll().Backward() // x³'ün türevi: 3x²
	x2.Mul(x2).Mul(x2).SumAll().Backward()
	yakin(t, "kare gradyanı", x1.Grad().Data(), x2.Grad().Data(), 1e-5)
	gradDenetle(t, "kare (sayısal)", func(xs []*Tensor) *Tensor { return kareOzel(xs[0]) }, Randn(rng, 2, 3))
}

func TestCustomOpTekDugum(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	y := kareOzel(x)
	if y.IsLeaf() || !y.RequiresGrad() {
		t.Fatal("dış işlemin sonucu çizgeye bağlanmalı")
	}
	if n := len(topoOrder(y)); n != 2 { // x ve y
		t.Fatalf("çizgede %d düğüm, 2 bekleniyordu", n)
	}
	// ileri hesap girdiyi olduğu gibi döndürse bile girdi yaprak kalır.
	z := CustomOp("özdeş", []*Tensor{x}, []*Tensor{x}, nil, func(_ any, g []*Tensor) []*Tensor { return g })[0]
	if !x.IsLeaf() || z == x || z.IsLeaf() {
		t.Fatal("çıktı girdiyle aynı tensör olmamalı; girdi yaprak kalmalı")
	}
	z.SumAll().Backward()
	yakin(t, "özdeş gradyanı", x.Grad().Data(), []float32{1, 1}, 0)
}

func TestCustomOpIzlenmeyenGirdi(t *testing.T) {
	a := New([]float32{1, 2, 3}, 3).SetRequiresGrad(true)
	b := New([]float32{4, 5, 6}, 3)
	var y *Tensor
	NoGrad(func() { y = a.Mul(b) })
	cagrildi := false
	out := CustomOp("çarp", []*Tensor{a, b, nil}, []*Tensor{y}, []*Tensor{a, b}, func(_ any, g []*Tensor) []*Tensor {
		cagrildi = true
		if GradEnabled() {
			t.Error("geri çağrı türevsiz kipte çalışmalı")
		}
		// b ve tensör olmayan üçüncü girdi için verilen gradyanlar yok sayılır.
		return []*Tensor{g[0].Mul(b), g[0].Mul(a), Ones(7)}
	})[0]
	out.SumAll().Backward()
	if !cagrildi {
		t.Fatal("geri çağrı çalışmadı")
	}
	yakin(t, "a gradyanı", a.Grad().Data(), []float32{4, 5, 6}, 0)
	if b.Grad() != nil {
		t.Fatal("türev izlemeyen girdiye gradyan yazılmamalı")
	}

	// Hiçbir girdi gradyan istemiyorsa ya da türevsiz kipte çıktı olduğu gibi döner.
	if r := CustomOp("x", []*Tensor{b}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return nil })[0]; r != y {
		t.Fatal("gradyan istemeyen girdide çıktı değişmemeli")
	}
	NoGrad(func() {
		if r := CustomOp("x", []*Tensor{a}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return nil })[0]; r != y {
			t.Fatal("türevsiz kipte çıktı değişmemeli")
		}
	})
}

// toplaFark (a+b, a-b) iki çıktılı bir işlem; geri hesap kaç kez çağrıldığını sayar.
func toplaFark(a, b *Tensor, sayac *int) (*Tensor, *Tensor) {
	var s, f *Tensor
	NoGrad(func() { s, f = a.Add(b), a.Sub(b) })
	outs := CustomOp("topla_fark", []*Tensor{a, b}, []*Tensor{s, f}, nil, func(_ any, g []*Tensor) []*Tensor {
		*sayac++
		return []*Tensor{g[0].Add(g[1]), g[0].Sub(g[1])}
	})
	return outs[0], outs[1]
}

func TestCustomOpCokCikti(t *testing.T) {
	a := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	b := New([]float32{3, 5}, 2).SetRequiresGrad(true)
	sayac := 0
	s, f := toplaFark(a, b, &sayac)
	// İki çıktı da kayba bağlı: L = Σ 2s + Σ 3f → dL/da = 5, dL/db = -1.
	s.MulScalar(2).SumAll().Add(f.MulScalar(3).SumAll()).Backward()
	if sayac != 1 {
		t.Fatalf("geri hesap %d kez çağrıldı, bir kez bekleniyordu", sayac)
	}
	yakin(t, "a", a.Grad().Data(), []float32{5, 5}, 0)
	yakin(t, "b", b.Grad().Data(), []float32{-1, -1}, 0)

	// Yalnızca ikinci çıktı kayba bağlı: birinci çıktının gradyanı sıfır tensördür.
	a.ZeroGrad()
	b.ZeroGrad()
	sayac = 0
	_, f = toplaFark(a, b, &sayac)
	f.Mul(f).SumAll().Backward() // dL/df = 2f = [-4, -6]
	yakin(t, "a (tek çıktı)", a.Grad().Data(), []float32{-4, -6}, 0)
	yakin(t, "b (tek çıktı)", b.Grad().Data(), []float32{4, 6}, 0)

	// Aynı çizgeden ikinci geri yayılım (diğer çıktı üzerinden) açık hatadır.
	s, f = toplaFark(a, b, &sayac)
	f.SumAll().Backward()
	panikBekle(t, "çok çıktıda ikinci geri yayılım", func() { s.SumAll().Backward() })
}

func TestCustomOpSaklananDegisirseHata(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	c := New([]float32{3, 4}, 2)
	var y *Tensor
	NoGrad(func() { y = x.Mul(c) })
	out := CustomOp("ölçekle", []*Tensor{x}, []*Tensor{y}, []*Tensor{c}, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{g[0].Mul(c)}
	})[0]
	c.Set(9, 0)
	mesaj := panikMesaji(func() { out.SumAll().Backward() })
	if !strings.Contains(mesaj, `"ölçekle"`) || !strings.Contains(mesaj, "yerinde değiştirildi") {
		t.Fatalf("sürüm hatası beklenirdi, alınan: %q", mesaj)
	}
}

func TestCustomOpYanlisGradyanlar(t *testing.T) {
	x := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	sabit := func(grads ...*Tensor) *Tensor {
		var y *Tensor
		NoGrad(func() { y = x.MulScalar(2) })
		return CustomOp("yanlış", []*Tensor{x}, []*Tensor{y}, nil, func(any, []*Tensor) []*Tensor { return grads })[0]
	}
	if m := panikMesaji(func() { sabit(Ones(3)).SumAll().Backward() }); !strings.Contains(m, "[3] şeklinde gradyan") || !strings.Contains(m, "[2 3]") {
		t.Fatalf("şekil hatası: %q", m)
	}
	if m := panikMesaji(func() { sabit(Ones(2, 3), nil).SumAll().Backward() }); !strings.Contains(m, "1 girdi için 2 gradyan") {
		t.Fatalf("uzunluk hatası: %q", m)
	}
}

func TestCustomOpBaglamIletilir(t *testing.T) {
	x := New([]float32{1}, 1).SetRequiresGrad(true)
	var gelen any
	var y *Tensor
	NoGrad(func() { y = x.Clone() })
	out := CustomOp("bağlam", []*Tensor{x}, []*Tensor{y}, nil, func(ctx any, g []*Tensor) []*Tensor {
		gelen = ctx
		return g
	})[0]
	out.SumAll().BackwardCtx(nil, "görev")
	if gelen != "görev" {
		t.Fatalf("bağlam iletilmedi: %v", gelen)
	}
}

// Karışık türler: bf16 çıktının gradyanı GradDType türündedir; bf16 girdinin gradyanı da.
func TestCustomOpBF16(t *testing.T) {
	x := NewOf(BFloat16, []float32{1, 2}, 2).SetRequiresGrad(true)
	var y *Tensor
	NoGrad(func() { y = x.Mul(x) })
	out := CustomOp("kare16", []*Tensor{x}, []*Tensor{y}, []*Tensor{x}, func(_ any, g []*Tensor) []*Tensor {
		if g[0].DType() != GradDType() {
			t.Errorf("çıktı gradyanı %s, %s bekleniyordu", g[0].DType(), GradDType())
		}
		return []*Tensor{g[0].Mul(x).MulScalar(2)}
	})[0]
	if out.DType() != BFloat16 {
		t.Fatalf("çıktı türü %s", out.DType())
	}
	out.Cast(Float32).SumAll().Backward()
	yakin(t, "bf16 gradyan", x.Grad().Data(), []float32{2, 4}, 0)
}

func panikMesaji(fn func()) (mesaj string) {
	defer func() {
		if r := recover(); r != nil {
			mesaj = r.(string)
		}
	}()
	fn()
	return ""
}

// Bir dış düğümün geri hesabı sürerken aynı çizgenin düğümlerinden ikinci bir geri yayılım
// açık hata verir; ilk geri yayılım bozulmadan biter.
func TestCustomOpGeriSirasindaIkinciGeriYayilim(t *testing.T) {
	x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	var k2 *Tensor
	var ikinciHata string
	o := CustomOp("özdeş", []*Tensor{x}, []*Tensor{x.Detach()}, nil, func(_ any, g []*Tensor) []*Tensor {
		ikinciHata = panikMesaji(func() { k2.Backward() })
		return g
	})[0]
	k1 := o.MulScalar(3).SumAll()
	k2 = o.MulScalar(2).SumAll()
	k1.Backward()
	if !strings.Contains(ikinciHata, "başka bir geri yayılım") {
		t.Fatalf("ikinci geri yayılım hata vermedi: %q", ikinciHata)
	}
	yakin(t, "ilk geri yayılım", x.Grad().Data(), []float32{3, 3}, 0)
	// Hata veren geri yayılımın ayırdığı düğümlerin işareti kaldırıldı; o artık serbest
	// olduğu için k2 açıkça "zaten yapıldı" hatası verir.
	if h := panikMesaji(func() { k2.Backward() }); !strings.Contains(h, "zaten yapıldı") {
		t.Fatalf("beklenen hata: %q", h)
	}
}
