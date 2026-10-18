package tensor

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Sert hakem denetiminin bulgularının regresyon testleri.

// panikIcerir fn'in, mesajında parca geçen bir panik verdiğini doğrular.
func panikIcerir(t *testing.T, ad, parca string, fn func()) {
	t.Helper()
	if m := panikMesaji(fn); !strings.Contains(m, parca) {
		t.Fatalf("%s: %q içeren panik bekleniyordu, alınan: %q", ad, parca, m)
	}
}

// Bulgu 2: Grad() gradyan deposunun canlı bir takma adıdır. Gradyanı değiştiren her işlem
// (geri yayılım birikimi, ZeroGrad, ClipGradNorm) deponun sürümünü artırır; saklanmış bir
// gradyanla kurulmuş çizge, gradyan değişince açık "yerinde değiştirildi" hatası verir.
func TestGradyanTakmaAdiSurumu(t *testing.T) {
	yeni := func() (*Tensor, *Tensor, *Tensor) {
		x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
		w := New([]float32{1, 1}, 2).SetRequiresGrad(true)
		x.MulScalar(3).SumAll().Backward()
		g := x.Grad()
		return x, w, g.Mul(w).SumAll() // dk/dw = g = [3, 3]
	}
	// Değişiklik yoksa geri yayılım doğru.
	_, w, k := yeni()
	k.Backward()
	yakin(t, "değişmeyen gradyan", w.Grad().Data(), []float32{3, 3}, 0)

	x, _, k := yeni()
	x.MulScalar(5).SumAll().Backward() // x'in gradyanı yerinde birikir
	panikIcerir(t, "birikim", "yerinde değiştirildi", func() { k.Backward() })

	x, _, k = yeni()
	x.ZeroGrad()
	panikIcerir(t, "ZeroGrad", "yerinde değiştirildi", func() { k.Backward() })

	x, _, k = yeni()
	ClipGradNorm([]*Tensor{x}, 0.1)
	panikIcerir(t, "ClipGradNorm", "yerinde değiştirildi", func() { k.Backward() })

	// Aynı geri yayılımda hem gradyanı birikecek x hem de x'in saklanmış gradyanı
	// kullanılıyorsa da açık hata (sıraya bağlı sessiz yanlış yok).
	x, w, _ = yeni()
	g := x.Grad()
	k = Add(x.MulScalar(2).SumAll(), g.Mul(w).SumAll())
	panikIcerir(t, "aynı geri yayılım", "yerinde değiştirildi", func() { k.Backward() })
}

// Bulgu 3: bitişik olmayan tensörde Squeeze/Unsqueeze ve uygun Reshape'ler veriyi
// kopyalamayan görünümlerdir (yazma kaynağa yansır); gradyanları doğrudur.
func TestBitisikOlmayanGorunumler(t *testing.T) {
	ayniDepo := func(ad string, a, b *Tensor) {
		t.Helper()
		if a.storage != b.storage {
			t.Fatalf("%s: görünüm değil, kopya", ad)
		}
	}
	x := sirali(2, 3)
	d := x.T() // [3, 2], bitişik değil
	satir := d.Slice(0, 0, 1).Squeeze(0)
	ayniDepo("Squeeze", x, satir)
	satir.Set(9, 1) // d[0][1] = x[1][0]
	if x.At(1, 0) != 9 {
		t.Fatalf("Squeeze görünümüne yazma kaynağa yansımadı: %v", x.Data())
	}
	u := d.Unsqueeze(1) // [3, 1, 2]
	ayniDepo("Unsqueeze", x, u)
	sekilEsit(t, "Unsqueeze şekli", u.shape, []int{3, 1, 2})
	yakin(t, "Unsqueeze verisi", u.Data(), d.Data(), 0)
	ayniDepo("Unsqueeze(-1)", x, d.Unsqueeze(-1))

	// [2, 3, 4]'ün son iki ekseni yer değiştirince [2, 4, 3]; ilk eksen birleştirilebilir
	// (adımı diğerlerinin çarpımı), son ikisi birleştirilemez.
	y := sirali(2, 3, 4).Permute(0, 2, 1)
	r := y.Reshape(1, 2, 4, 3, 1)
	ayniDepo("Reshape (boyut 1 ekleme)", y, r)
	yakin(t, "Reshape verisi", r.Data(), y.Data(), 0)
	z := sirali(4, 6).SliceStep(1, 0, 6, 2) // [4, 3], adım (6, 2)
	r2 := z.Reshape(2, 2, 3)
	ayniDepo("Reshape (ilk ekseni bölme)", z, r2)
	yakin(t, "bölünmüş veri", r2.Data(), z.Data(), 0)
	if r3 := y.Reshape(2, 12); r3.storage == y.storage {
		t.Fatal("birleştirilemeyen eksenler kopyalanmalı")
	} else {
		yakin(t, "kopya verisi", r3.Data(), y.Data(), 0)
	}

	rng := rand.New(rand.NewSource(11))
	gradDenetle(t, "Squeeze/Unsqueeze görünüm gradyanı", func(xs []*Tensor) *Tensor {
		return xs[0].T().Slice(0, 1, 2).Squeeze(0).Unsqueeze(0).Mul(xs[0].T().Slice(0, 0, 1))
	}, Randn(rng, 2, 3))
	gradDenetle(t, "adımlı Reshape gradyanı", func(xs []*Tensor) *Tensor {
		return xs[0].SliceStep(1, 0, 6, 2).Reshape(2, 2, 3).Square()
	}, Randn(rng, 4, 6))
}

// Görünümle yeniden şekillendirme, rastgele eksen sırası/dilim ve hedef şekillerde
// kopyalayarak yeniden şekillendirmeyle aynı elemanları verir.
func TestGorunumReshapeRastgele(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	bolenler := func(n int) []int {
		var b []int
		for d := 1; d <= n; d++ {
			if n%d == 0 {
				b = append(b, d)
			}
		}
		return b
	}
	gorunum := 0
	for deneme := 0; deneme < 2000; deneme++ {
		nd := 1 + rng.Intn(4)
		sh := make([]int, nd)
		for i := range sh {
			sh[i] = 1 + rng.Intn(4)
		}
		x := sirali(sh...)
		// Rastgele eksen sırası ve adımlı dilim.
		x = x.Permute(rng.Perm(nd)...)
		e := rng.Intn(nd)
		if n := x.shape[e]; n > 1 {
			x = x.SliceStep(e, rng.Intn(2), n, 1+rng.Intn(2))
		}
		// Rastgele hedef şekil (boyut 1 eksenler dahil).
		kalan := x.Numel()
		var yeni []int
		for kalan > 1 || rng.Intn(3) == 0 {
			b := bolenler(kalan)
			d := b[rng.Intn(len(b))]
			yeni = append(yeni, d)
			kalan /= d
			if len(yeni) > 6 {
				break
			}
		}
		if kalan != 1 {
			yeni = append(yeni, kalan)
		}
		r := x.Reshape(yeni...)
		if r.storage == x.storage {
			gorunum++
		}
		yakin(t, fmt.Sprintf("%v→%v", x.shape, yeni), r.Data(), x.Contiguous().Reshape(yeni...).Data(), 0)
	}
	if gorunum < 200 {
		t.Fatalf("yalnızca %d görünüm; algoritma fazla kopyalıyor olabilir", gorunum)
	}
}
