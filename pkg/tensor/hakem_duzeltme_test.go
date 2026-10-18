package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
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

// nanAyni iki dilimin NaN'lar dahil bit düzeyinde değil değer düzeyinde aynı olduğunu
// (NaN yerinde NaN) doğrular.
func nanAyni(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		gn, wn := got[i] != got[i], want[i] != want[i]
		if gn != wn || (!gn && got[i] != want[i]) {
			t.Fatalf("%s: %v, beklenen %v", ad, got, want)
		}
	}
}

// Bulgu 4: maks/min/relu ve maks_indeks NaN'ı yayar (indirgemede NaN varsa sonuç NaN,
// maks_indeks ilk NaN'ın yeri); 16 bitlik türlerde de aynı.
func TestNaNYayilir(t *testing.T) {
	nan := float32(math.NaN())
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		x := New([]float32{1, nan, 3, nan, nan, nan, 0, 5, 2}, 3, 3).Cast(dt)
		ad := dt.String()
		nanAyni(t, ad+" Max(1)", x.Max(1, false).Data(), []float32{nan, nan, 5})
		nanAyni(t, ad+" Max(0)", x.Max(0, false).Data(), []float32{nan, nan, nan})
		nanAyni(t, ad+" -Max(-x)", x.Neg().Max(1, false).Neg().Data(), []float32{nan, nan, 0})
		if got := x.ArgMax(1); got[0] != 1 || got[1] != 0 || got[2] != 1 {
			t.Fatalf("%s ArgMax(1) = %v, beklenen [1 0 1]", ad, got)
		}
		if got := x.ArgMax(0); got[0] != 1 || got[1] != 0 || got[2] != 1 {
			t.Fatalf("%s ArgMax(0) = %v, beklenen [1 0 1]", ad, got)
		}
		nanAyni(t, ad+" Relu", New([]float32{-1, nan, 2}, 3).Cast(dt).Relu().Data(), []float32{0, nan, 2})
	}
	// Gradyan ilk NaN'ın yerine gider.
	x := New([]float32{1, nan, 3}, 3).SetRequiresGrad(true)
	x.Max(0, false).Backward()
	nanAyni(t, "Max gradyanı", x.Grad().Data(), []float32{0, 1, 0})
}

// Bulgu 12: GELU'nun çok büyük |x|'te ve x**0'ın x=0'da gradyanı NaN değildir.
func TestGeluVeSifirinciKuvvetGradyani(t *testing.T) {
	x := New([]float32{1e20, -1e20, 3e38, -3e38, 0, 2}, 6).SetRequiresGrad(true)
	x.Gelu().SumAll().Backward()
	yakin(t, "GELU' uçlarda", x.Grad().Data()[:4], []float32{1, 0, 1, 0}, 0)
	z := New([]float32{0, 2, -1}, 3).SetRequiresGrad(true)
	z.Pow(0).SumAll().Backward()
	yakin(t, "d/dx x^0", z.Grad().Data(), []float32{0, 0, 0}, 0)
	yakin(t, "x^0", z.Detach().Pow(0).Data(), []float32{1, 1, 1}, 0)
}

// Bulgu 5: skalere bölme tensörün türünü korur ve gerçek bölmedir (float32'de bölünüp bir
// kez yuvarlanır).
func TestDivScalarTuruKorur(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3)
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		y := x.Cast(dt).DivScalar(3)
		if y.DType() != dt {
			t.Fatalf("%s / 3 türü %s", dt, y.DType())
		}
		yakin(t, dt.String()+" / 3", y.Data(), Div(x, Scalar(3)).Cast(dt).Data(), 0)
	}
	// 1/s ile çarpmadan farklı olarak tam bölme: 3/3 = 1.
	yakin(t, "3/3", New([]float32{3}, 1).DivScalar(3).Data(), []float32{1}, 0)
	gradDenetle(t, "DivScalar gradyanı", func(xs []*Tensor) *Tensor { return xs[0].DivScalar(4) }, New([]float32{1, -2, 3}, 3))
}

// Bulgu 6: tamamen maskeli (-sonsuz) softmax satırı 0 (log_softmax'ta -sonsuz) verir ve
// gradyanı 0'dır; birleşik dikkatteki boş satır davranışıyla tutarlı. Son eksen (vektörel)
// ve diğer eksen yolları ile 16 bitlik türler aynı.
func TestTamamenMaskeliSoftmax(t *testing.T) {
	ni := float32(math.Inf(-1))
	m := New([]float32{ni, ni, ni, 0, ni, 0}, 2, 3)
	for _, dt := range []DType{Float32, BFloat16} {
		for _, devrik := range []bool{false, true} {
			x := m.Cast(dt)
			eksen := 1
			if devrik {
				x = x.T().Contiguous() // [3, 2]; satırlar eksen 0'da
				eksen = 0
			}
			ad := fmt.Sprintf("%s eksen %d", dt, eksen)
			s := x.Softmax(eksen)
			ls := x.LogSoftmax(eksen)
			if devrik {
				s, ls = s.T(), ls.T()
			}
			nanAyni(t, ad+" softmax", s.Data(), []float32{0, 0, 0, 0.5, 0, 0.5})
			l := float32(math.Log(0.5))
			nanAyni(t, ad+" log_softmax", ls.Data(), New([]float32{ni, ni, ni, l, ni, l}, 6).Cast(dt).Data())
		}
	}
	// Gradyan: maskeli satırda 0, diğer satırda sonlu.
	for _, logKip := range []bool{false, true} {
		w := New([]float32{1, 2, 3, 1, 2, 3}, 2, 3).SetRequiresGrad(true)
		z := Add(w, m)
		var y *Tensor
		if logKip {
			y = z.LogSoftmax(1)
		} else {
			y = z.Softmax(1)
		}
		// Gelen gradyan maskeli satırda da sıfırdan farklı (ileri değer -sonsuz olsa da
		// geri yayılım tanımlıdır).
		y = Mul(y, New([]float32{1, 2, 3, 1, 2, 3}, 2, 3)).SumAll()
		y.Backward()
		g := w.Grad().Data()
		for i, v := range g {
			if v != v || (i < 3 && v != 0) {
				t.Fatalf("log=%v gradyan %v: maskeli satır 0, hepsi sonlu olmalı", logKip, g)
			}
		}
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

// Bulgu 18: büyük toplu çarpımda B paketlemesi toplu parçalara bölünür (tampon sınırlı ve
// havuzdan); sonuç her toplu elemanın tek başına çarpımıyla bit düzeyinde aynıdır.
func TestGemmTopluParca(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	durumlar := []struct{ batch, M, N, K int }{{3, 13, 17, 300}, {20, 64, 1100, 64}, {70, 9, 300, 64}}
	for _, d := range durumlar {
		if d.batch*((d.N+nr-1)/nr*d.K*nr) <= tamponEnBuyuk && d.batch > 3 {
			t.Fatalf("%v parçalanmıyor; durum yanlış seçilmiş", d)
		}
		for _, tr := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			a := Randn(rng, d.batch*d.M*d.K).Data()
			b := Randn(rng, d.batch*d.K*d.N).Data()
			c1 := make([]float32, d.batch*d.M*d.N)
			c2 := make([]float32, d.batch*d.M*d.N)
			gemm(d.batch, d.M, d.N, d.K, a, tr[0], b, tr[1], c1)
			for bi := 0; bi < d.batch; bi++ {
				gemm(1, d.M, d.N, d.K, a[bi*d.M*d.K:(bi+1)*d.M*d.K], tr[0], b[bi*d.K*d.N:(bi+1)*d.K*d.N], tr[1], c2[bi*d.M*d.N:(bi+1)*d.M*d.N])
			}
			for i := range c1 {
				if c1[i] != c2[i] {
					t.Fatalf("%v devrik=%v: [%d] %v != %v", d, tr, i, c1[i], c2[i])
				}
			}
		}
	}
}

// Bulgu 10: işçi goroutine'inde (ya da çağıranın parçasında) oluşan panik kurtarılıp
// çağırana taşınır; işçiler yarım işi sonraki çağrılarla eşzamanlı yürütmeye devam etmez ve
// havuz kullanılabilir kalır. İç içe (geçici goroutine'li) yolda da aynı.
func TestIsciPanigiCagiranaTasinir(t *testing.T) {
	if workers() < 2 {
		t.Skip("tek işlemci")
	}
	for _, icIce := range []bool{false, true} {
		calistir := func(fn func()) {
			if icIce {
				dagit(2, 2, func(k, c int) {
					if c == 0 {
						fn()
					}
				})
				return
			}
			fn()
		}
		var yurutulen atomic.Int64
		m := panikMesaji(func() {
			calistir(func() {
				dagit(4, 64, func(_, c int) {
					yurutulen.Add(1)
					if c == 5 {
						panic("tensor: işçide hata")
					}
				})
			})
		})
		if m != "tensor: işçide hata" {
			t.Fatalf("iç içe=%v: panik çağırana taşınmadı: %q", icIce, m)
		}
		// Havuz hâlâ çalışır ve bütün parçaları yürütür.
		var n atomic.Int64
		dagit(4, 100, func(_, c int) { n.Add(1) })
		if n.Load() != 100 {
			t.Fatalf("panik sonrası havuz %d/100 parça yürüttü", n.Load())
		}
	}
}

// Bulgu 11: üstelin sonucu elemanın konumuna (8'li gövde ya da kuyruk, blok sınırı) ve
// softmax'ın eksenine bağlı değildir.
func TestUstelKonumdanBagimsiz(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, n := range []int{1, 7, 9, 15, 17, 255, 257, 300, 1000} {
		x := make([]float32, n)
		for i := range x {
			x[i] = float32(rng.NormFloat64() * 30)
		}
		y := make([]float32, n)
		expVek(y, x)
		for i := range x {
			var tek [1]float32
			expVek(tek[:], x[i:i+1])
			if y[i] != tek[0] && !(y[i] != y[i] && tek[0] != tek[0]) {
				t.Fatalf("n=%d: e^x[%d] gövdede %v, tek başına %v", n, i, y[i], tek[0])
			}
		}
	}
	// Eşit elemanlı tensörde (hakem kanıtı: 9 eleman, e[0] != e[8]) hepsi aynı.
	e := Full(-48.528, 9).Exp().Data()
	for i := range e {
		if e[i] != e[0] {
			t.Fatalf("üstel(t)[%d] = %v, [0] = %v", i, e[i], e[0])
		}
	}
	// Softmax son eksende ve diğer eksende aynı değerleri verir.
	x := Randn(rng, 37, 11).MulScalar(5)
	for _, logKip := range []bool{false, true} {
		var a, b *Tensor
		if logKip {
			a, b = x.LogSoftmax(1), x.T().Contiguous().LogSoftmax(0).T()
		} else {
			a, b = x.Softmax(1), x.T().Contiguous().Softmax(0).T()
		}
		ad, bd := a.Data(), b.Data()
		for i := range ad {
			if ad[i] != bd[i] {
				t.Fatalf("log=%v: softmax son eksen %v, eksen 0 %v ([%d])", logKip, ad[i], bd[i], i)
			}
		}
	}
}

// Bulgu 13 ve 14: tanımsız sonuçlar sessizce 0 dönmez: boş tensörün/boş eksenin ortalaması
// ve bütün hedefleri yok sayılan çapraz entropi açık hata verir; 0 boyutlu girdide
// LayerNorm/RMSNorm biçimli bir hata verir (ham indeks paniği değil).
func TestTanimsizSonuclarHata(t *testing.T) {
	panikIcerir(t, "MeanAll(boş)", "ortalaması tanımsız", func() { Zeros(0).MeanAll() })
	panikIcerir(t, "Mean(boş eksen)", "ortalaması tanımsız", func() { Zeros(3, 0).Mean(1, false) })
	// Boş olmayan eksenin boş tensördeki ortalaması tanımlıdır (sonuç boş).
	if n := Zeros(3, 0).Mean(0, false).Numel(); n != 0 {
		t.Fatalf("[3,0] eksen 0 ortalaması %d elemanlı", n)
	}
	if v := Zeros(0).SumAll().Item(); v != 0 {
		t.Fatalf("boş toplam %v", v)
	}
	logit := New([]float32{1, 2, 3, 4}, 2, 2)
	panikIcerir(t, "CE yok sayılan", "geçerli hedef yok", func() { CrossEntropy(logit, []int{-1, -1}) })
	panikIcerir(t, "CE C=0", "geçerli hedef yok", func() { CrossEntropy(Zeros(2, 0), nil) })
	if v := CrossEntropy(logit, []int{-1, 1}).Item(); v != v || v <= 0 {
		t.Fatalf("tek geçerli hedefle kayıp %v", v)
	}
	panikIcerir(t, "LayerNorm 0 boyut", "en az 1 boyutlu", func() { LayerNorm(Scalar(1), nil, nil, 1e-5) })
	panikIcerir(t, "RMSNorm 0 boyut", "en az 1 boyutlu", func() { RMSNorm(Scalar(1), nil, 1e-5) })
}

// Bulgu 15a: AdamW'nin sapma düzeltmesi parametre başına adım sayısını kullanır: ilk
// adımlarda gradyanı olmayan bir parametrenin ilk güncellemesi, yeni bir iyileştiricinin
// ilk güncellemesiyle aynıdır.
func TestAdamWParametreBasinaAdim(t *testing.T) {
	a := New([]float32{1, 2}, 2).SetRequiresGrad(true)
	b := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op := NewAdamW([]*Tensor{a, b}, 0.1, 0)
	for i := 0; i < 3; i++ {
		op.ZeroGrad()
		a.Square().SumAll().Backward() // b'nin gradyanı yok
		op.Step()
	}
	op.ZeroGrad()
	b.Square().SumAll().Backward()
	op.Step()
	tek := New([]float32{3, 4}, 2).SetRequiresGrad(true)
	op2 := NewAdamW([]*Tensor{tek}, 0.1, 0)
	tek.Square().SumAll().Backward()
	op2.Step()
	yakin(t, "gecikmeli ilk adım", b.Data(), tek.Data(), 0)
	if op.StepCount() != 4 {
		t.Fatalf("StepCount %d", op.StepCount())
	}
}

// Bulgu 15b: iyileştirici durumu (momentler, adım sayıları, 16 bitlik parametrelerin
// float32 ana kopyası, SGD hızı) dışa alınıp yeni bir iyileştiriciye yüklenebilir;
// sürdürülen eğitim kesintisiz eğitimle bit düzeyinde aynıdır.
func TestIyilestiriciDurumuSurdurme(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	w0 := Randn(rng, 4, 3)
	b0 := Randn(rng, 3).Cast(BFloat16)
	x := Randn(rng, 5, 4)
	adim := func(params []*Tensor, op Optimizer) {
		op.ZeroGrad()
		Add(x.MatMul(params[0]), params[1]).Square().MeanAll().Backward()
		op.Step()
	}
	for _, tur := range []string{"AdamW", "SGD"} {
		yeni := func(params []*Tensor) interface {
			Optimizer
			State() map[string]*Tensor
			LoadState(map[string]*Tensor)
		} {
			if tur == "AdamW" {
				return NewAdamW(params, 0.01, 0.1)
			}
			return NewSGD(params, 0.05, 0.9)
		}
		kopya := func() []*Tensor {
			return []*Tensor{w0.Clone().Detach().SetRequiresGrad(true), b0.Clone().Detach().SetRequiresGrad(true)}
		}
		// Kesintisiz: 8 adım.
		pa := kopya()
		oa := yeni(pa)
		for i := 0; i < 8; i++ {
			adim(pa, oa)
		}
		// 5 adım, durumu al, yeni iyileştiriciyle 3 adım daha.
		pb := kopya()
		ob := yeni(pb)
		for i := 0; i < 5; i++ {
			adim(pb, ob)
		}
		durum := ob.State()
		pc := []*Tensor{pb[0].Clone().Detach().SetRequiresGrad(true), pb[1].Clone().Detach().SetRequiresGrad(true)}
		oc := yeni(pc)
		oc.LoadState(durum)
		for i := 0; i < 3; i++ {
			adim(pc, oc)
		}
		for k := range pa {
			yakin(t, fmt.Sprintf("%s param %d", tur, k), pc[k].Data(), pa[k].Data(), 0)
		}
		// Uyumsuz durum açık hata verir.
		panikIcerir(t, tur+" uyumsuz", "durum", func() {
			yeni([]*Tensor{Zeros(2).SetRequiresGrad(true)}).LoadState(durum)
		})
	}
}
