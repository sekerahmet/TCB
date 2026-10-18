package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// Paralel çekirdek yollarının doğruluk ve belirlenimcilik testleri. Boyutlar,
// paralelleştirme eşiklerini aşacak şekilde seçilmiştir.

// naifSum float64 birikimli başvuru eksen toplamı.
func naifSum(xd []float32, shape []int, axis int) []float32 {
	outer, dim, inner := eksenDuzeni(shape, axis)
	out := make([]float32, outer*inner)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			var s float64
			for d := 0; d < dim; d++ {
				s += float64(xd[(o*dim+d)*inner+i])
			}
			out[o*inner+i] = float32(s)
		}
	}
	return out
}

func TestSumParalelYollar(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	durumlar := []struct {
		shape []int
		axis  int
	}{
		{[]int{3, 100000}, 1},    // son eksen, birkaç satır
		{[]int{1, 1 << 17}, 1},   // son eksen, tek satır (parçalı toplam)
		{[]int{100, 7, 300}, 1},  // çok dış satır
		{[]int{2, 5, 4096}, 1},   // az dış satır, geniş iç eksen (sütun blokları)
		{[]int{1, 5000, 10}, 1},  // az dış satır, dar iç eksen (eksen parçaları)
		{[]int{5000, 10}, 0},     // eksen 0
		{[]int{2048, 512}, 0},    // eksen 0, orta genişlik
		{[]int{3, 4}, 0},         // küçük
		{[]int{0, 4}, 0},         // boş
		{[]int{4, 0, 3}, 1},      // boş eksen
		{[]int{7, 33, 1, 5}, -2}, // uzunluğu 1 olan eksen
	}
	for _, d := range durumlar {
		x := Randn(rng, d.shape...)
		got := x.Sum(d.axis, false).Data()
		want := naifSum(x.Data(), x.shape, normAxis(d.axis, len(d.shape)))
		yakin(t, "sum", got, want, 1e-5)
		// Belirlenimcilik: tekrar ve farklı işçi sayısıyla bit düzeyinde aynı.
		eski := runtime.GOMAXPROCS(3)
		tekrar := x.Sum(d.axis, false).Data()
		runtime.GOMAXPROCS(eski)
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(tekrar[i]) {
				t.Fatalf("sum %v eksen %d belirlenimci değil: [%d] %v != %v", d.shape, d.axis, i, got[i], tekrar[i])
			}
		}
	}
}

func TestSumAllParalel(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	x := Randn(rng, 3, 100003)
	var s float64
	for _, v := range x.Data() {
		s += float64(v)
	}
	got := x.SumAll().Item()
	if math.Abs(float64(got)-s) > 1e-3 {
		t.Fatalf("SumAll %v, beklenen %v", got, s)
	}
	eski := runtime.GOMAXPROCS(2)
	tekrar := x.SumAll().Item()
	runtime.GOMAXPROCS(eski)
	if got != tekrar {
		t.Fatalf("SumAll belirlenimci değil: %v != %v", got, tekrar)
	}
	// Devrik (bitişik olmayan) görünüm.
	yakin(t, "devrik SumAll", []float32{x.T().SumAll().Item()}, []float32{float32(s)}, 1e-5)
}

// tekVeCok fn'i tek işçiyle ve çok işçiyle çalıştırıp sonuçların bit düzeyinde
// aynı olduğunu denetler (paralel yollar, sıralı yolla aynı toplama sırasını
// korumalıdır).
func tekVeCok(t *testing.T, ad string, fn func() []float32) {
	t.Helper()
	eski := runtime.GOMAXPROCS(1)
	tek := fn()
	runtime.GOMAXPROCS(max(eski, 8))
	cok := fn()
	runtime.GOMAXPROCS(eski)
	if len(tek) != len(cok) {
		t.Fatalf("%s: uzunluk %d != %d", ad, len(tek), len(cok))
	}
	for i := range tek {
		if math.Float32bits(tek[i]) != math.Float32bits(cok[i]) {
			t.Fatalf("%s: [%d] tek işçi %v, çok işçi %v", ad, i, tek[i], cok[i])
		}
	}
}

func TestParalelCekirdeklerSiraliylaAyni(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	x := Randn(rng, 256, 3, 200)
	w := Randn(rng, 200)
	k := Randn(rng, 256, 3, 1)
	tablo := Randn(rng, 300, 128)
	idx := make([]int, 1000)
	for i := range idx {
		idx[i] = rng.Intn(40) * 7 // yinelenen indeksler
	}
	gradlar := func(f func() *Tensor, ps ...*Tensor) []float32 {
		for _, p := range ps {
			p.SetRequiresGrad(true)
			p.grad = nil
		}
		out := f()
		out.BackwardWith(Ones(out.shape...))
		var r []float32
		r = append(r, out.Data()...)
		for _, p := range ps {
			r = append(r, p.Grad().Data()...)
			p.SetRequiresGrad(false)
			p.grad = nil
		}
		return r
	}
	durumlar := []struct {
		ad string
		fn func() []float32
	}{
		{"devrik kopya", func() []float32 { return x.Permute(2, 0, 1).Contiguous().Data() }},
		{"dilim geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return x.Slice(2, 10, 150).Mul(x.Slice(2, 50, 190)) }, x)
		}},
		{"eksen toplamı geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return x.Sum(-1, true).Mul(k) }, x, k)
		}},
		{"sapma gradyanı", func() []float32 { return gradlar(func() *Tensor { return Add(x, w) }, x, w) }},
		{"orta eksen yayını", func() []float32 { return gradlar(func() *Tensor { return Mul(x, k) }, x, k) }},
		{"gömme geri yayılımı", func() []float32 {
			return gradlar(func() *Tensor { return Embedding(tablo, idx).Square() }, tablo)
		}},
		{"kare toplamı", func() []float32 {
			p := x.Detach().SetRequiresGrad(true)
			p.grad = p.be.Alloc(p.Numel())
			p.be.Copy(p.grad, 0, x.view())
			n := ClipGradNorm([]*Tensor{p}, 1)
			return append([]float32{float32(n)}, p.Grad().Data()...)
		}},
		{"doldurma", func() []float32 { return Full(2.5, 300, 300).Data() }},
	}
	for _, d := range durumlar {
		tekVeCok(t, d.ad, d.fn)
	}
	// Başvuru değerleriyle karşılaştırma (sıralı yol da doğru olmalı).
	gw := gradlar(func() *Tensor { return Embedding(tablo, idx) }, tablo)[len(idx)*128:]
	beklenen := make([]float32, 300*128)
	for _, ix := range idx {
		for j := 0; j < 128; j++ {
			beklenen[ix*128+j]++
		}
	}
	yakin(t, "gömme gradyanı", gw, beklenen, 0)
	dev := x.Permute(2, 0, 1).Contiguous().Data()
	xd := x.Data()
	for c := 0; c < 200; c += 37 {
		for a := 0; a < 256; a += 51 {
			if dev[(c*256+a)*3+1] != xd[(a*3+1)*200+c] {
				t.Fatalf("devrik kopya yanlış (%d, %d)", c, a)
			}
		}
	}
}

// LayerNorm/RMSNorm ölçek ve sapma gradyanları belirlenimci olmalı: tekrar eden
// çalıştırmalarda ve farklı işçi sayılarında bit düzeyinde aynı.
func TestNormGradyanBelirlenimci(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	x := Randn(rng, 3000, 48)
	gamma := Randn(rng, 48)
	beta := Randn(rng, 48)
	w := Randn(rng, 3000, 48)
	for _, ad := range []string{"layer_norm", "rms_norm"} {
		fn := func() []float32 {
			x.SetRequiresGrad(true)
			gamma.SetRequiresGrad(true)
			beta.SetRequiresGrad(true)
			x.grad, gamma.grad, beta.grad = nil, nil, nil
			var y *Tensor
			if ad == "layer_norm" {
				y = LayerNorm(x, gamma, beta, 1e-5)
			} else {
				y = RMSNorm(x, gamma, 1e-5)
			}
			Mul(y, w).SumAll().Backward()
			r := append(gamma.Grad().Data(), x.Grad().Data()...)
			if beta.grad != nil {
				r = append(r, beta.Grad().Data()...)
			}
			return r
		}
		ilk := fn()
		for k := 0; k < 3; k++ {
			tekVeCok(t, ad, fn)
			tekrar := fn()
			for i := range ilk {
				if math.Float32bits(ilk[i]) != math.Float32bits(tekrar[i]) {
					t.Fatalf("%s gradyanı çalıştırmadan çalıştırmaya değişti: [%d] %v != %v", ad, i, ilk[i], tekrar[i])
				}
			}
		}
	}
}
