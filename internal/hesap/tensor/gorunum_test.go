package tensor

import (
	"fmt"
	"math/rand"
	"testing"
)

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

// Bulgu 7: CopyFrom bir görünüme (bitişik olmayan dahil) tek kopyayla yazar ve sürümü bir
// kez artırır; kaynak yayınlanabilir (sayı ataması), türler dönüştürülür, örtüşen kaynak
// önce kopyalanmış gibi davranır.
func TestCopyFromGorunume(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		x := Zeros(3, 4).Cast(dt)
		hedef := x.T().Slice(0, 1, 3) // [2, 3]: x'in 1. ve 2. sütunları, bitişik değil
		v0 := x.Version()
		hedef.CopyFrom(New([]float32{1, 2, 3, 4, 5, 6}, 2, 3))
		if x.Version() != v0+1 {
			t.Fatalf("%s: sürüm %d → %d, tek artış bekleniyordu", dt, v0, x.Version())
		}
		yakin(t, dt.String()+" atama", x.Data(), []float32{0, 1, 4, 0, 0, 2, 5, 0, 0, 3, 6, 0}, 0)
		x.Slice(0, 2, 3).CopyFrom(Scalar(7)) // sayı ataması: yayın
		yakin(t, dt.String()+" sayı", x.Data()[8:], []float32{7, 7, 7, 7}, 0)
		panikIcerir(t, "şekil", "şekl", func() { hedef.CopyFrom(Zeros(3, 2)) })
	}
	// Örtüşme: y[0..2] = y[1..3] kopyalanmış kaynakla aynı sonucu verir.
	y := New([]float32{1, 2, 3, 4}, 4)
	y.Slice(0, 1, 4).CopyFrom(y.Slice(0, 0, 3))
	yakin(t, "örtüşen kopya", y.Data(), []float32{1, 1, 2, 3}, 0)
}

func BenchmarkCopyFromGorunum(b *testing.B) {
	x := Zeros(1000, 1000)
	k := Ones(1000, 1000)
	d := x.T()
	b.SetBytes(4 * 1000 * 1000)
	for i := 0; i < b.N; i++ {
		d.CopyFrom(k)
	}
}
