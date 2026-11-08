package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// quantDotShapes ağırlık satırı uzunluklarıdır: en kısa satır, Qwen3-0.6B ve SmolLM2'nin boyları.
var quantDotShapes = map[DType][]int{
	Q8:  {32, 64, 576, 1024, 3072},
	Q4K: {256, 512, 1024, 2560, 3072},
	Q6K: {256, 512, 1024, 2560, 3072},
}

// TestQuantDotMatchesOpenedDot: çekirdek "aç + dotF32" ile bit bit aynıdır, Go eşi de; ölçeklerin
// bütün fp16 aralığı, alt normaller ve eksiler dahil. Sonsuz ve NaN toplamlar da aynıdır.
func TestQuantDotMatchesOpenedDot(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	r := rand.New(rand.NewSource(1))
	for _, path := range []struct {
		name      string
		avx, f16c bool
	}{{"çekirdek", oldAVX, oldF16C}, {"F16C yok", oldAVX, false}, {"Go", false, false}} {
		for dt, shapes := range quantDotShapes {
			for _, K := range shapes {
				hasAVX2, hasF16C = path.avx, path.f16c
				elems, size := dt.QuantBlock()
				rows := 8
				w := randomQuant(dt, uint32(K), rows*K/elems)
				x := randomData(r, K)
				s := newQuantScratch(dt, K)
				open := make([]float32, K)
				got, want := make([]float32, rows), make([]float32, rows)
				for n := range rows {
					row := w[n*K/elems*size : (n+1)*K/elems*size]
					got[n] = quantDot(dt, x, row, s)
					dequantBlocks(dt, open, row)
					want[n] = dotF32(x, open)
				}
				sameFloatBits(t, path.name+" "+dt.Name(), got, want)
			}
		}
	}
}

// openedWeight w'nin açılmış float32 kopyasıdır.
func openedWeight(w *Tensor) *Tensor { return w.Open(Float32) }

// withinSumBound 15.41'in sınırı: toplama sırası farkı, satırın büyüklüğüne göre.
func withinSumBound(t *testing.T, what string, got, want []float32, K int) {
	t.Helper()
	bound := 0.0
	for _, v := range want {
		bound = math.Max(bound, math.Abs(float64(v)))
	}
	bound *= float64(K) * 1.2e-7
	for i := range want {
		if math.Abs(float64(got[i])-float64(want[i])) > bound {
			t.Fatalf("%s: [%d] = %v, beklenen %v (sınır %v)", what, i, got[i], want[i], bound)
		}
	}
}

// smallScaleQuant ölçekleri küçük bloklardır: çarpım sonlu kalsın, sınır anlamlı olsun.
func smallScaleQuant(dt DType, seed uint32, blocks int) []byte {
	raw := randomQuant(dt, seed, blocks)
	_, size := dt.QuantBlock()
	for b := range blocks {
		block := raw[b*size : (b+1)*size]
		switch dt {
		case Q8:
			block[1] = 0x1C | block[1]&0x83
		case Q4K:
			block[1], block[3] = 0x1C|block[1]&0x83, 0x1C|block[3]&0x83
		case Q6K:
			block[209] = 0x1C | block[209]&0x83
		}
	}
	return raw
}

// TestQuantMatMulMatchesOpened: nicem çarpımı açılmış ağırlıkla float32 çarpımın 15.41 sınırındadır.
// Tek satır, kısa son parça, parçadan büyük N, satır dilimi; çıktı c'ye yazılır, bütün ağırlık açılmaz.
func TestQuantMatMulMatchesOpened(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for dt := range quantDotShapes {
		elems, _ := dt.QuantBlock()
		for _, c := range [][3]int{{1, 96, 512}, {1, 1, 512}, {3, 70, 256}, {2, quantTileRows + 17, 512}, {17, 2*quantTileRows + 1, 256}} {
			M, N, K := c[0], c[1], c[2]
			w := FromQuantized(dt, smallScaleQuant(dt, uint32(N), N*K/elems), N, K)
			x := New(randomData(r, M*K), M, K)
			got := MatMul(x, w.T())
			if s := got.Shape(); s[0] != M || s[1] != N || got.DType() != Float32 {
				t.Fatalf("%s: şekil %v, tür %v", dt.Name(), s, got.DType())
			}
			withinSumBound(t, dt.Name(), got.Data(), MatMul(x, openedWeight(w).T()).Data(), K)
		}
	}
}

// TestQuantMatMulSameBitsAsOpened: çok satırlı nicem çarpımı açılmış ağırlıkla float32 çarpımın
// bitlerini verir; parça boyu ve satır bloğu toplama sırasını değiştirmez. Şekiller parça sınırını
// (quantTileRows), kcBlock'u, satır bloğunu ve bufMaxB'nin kestiği parçayı (K = 8192) geçer.
func TestQuantMatMulSameBitsAsOpened(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for dt := range quantDotShapes {
		elems, _ := dt.QuantBlock()
		for _, c := range [][3]int{{2, 40, 256}, {50, quantTileRows + 17, 512}, {quantRowBlock + 7, 300, 1024}, {60, 600, 8192}} {
			M, N, K := c[0], c[1], c[2]
			w := FromQuantized(dt, smallScaleQuant(dt, uint32(N+K), N*K/elems), N, K)
			x := New(randomData(r, M*K), M, K)
			sameFloatBits(t, dt.Name(), MatMul(x, w.T()).Data(), MatMul(x, openedWeight(w).T()).Data())
		}
	}
}

// TestQuantTileStaysInThePools: parça nr'nin katıdır, N'yi aşmaz; açılan parça ve ara sonuç havuzun
// sınırında kalır; çok uzun satırda da en az nr satır açılır.
func TestQuantTileStaysInThePools(t *testing.T) {
	for _, c := range [][3]int{{512, 2560, 2560}, {512, 2560, 9728}, {597, 9728, 2560}, {4096, 4096, 2560}, {2, 5, 256}, {2, 100, bufMaxB}, {bufMax, 64, 256}} {
		M, N, K := c[0], c[1], c[2]
		rows := quantTile(M, N, K)
		if rows < 1 || rows > N || (rows < N && rows%nr != 0) {
			t.Fatalf("M %d N %d K %d: %d satır", M, N, K, rows)
		}
		if rows > nr && (rows*K > bufMaxB || M*rows > bufMax) {
			t.Fatalf("M %d N %d K %d: %d satır havuzu aşıyor", M, N, K, rows)
		}
	}
	if got := quantTile(512, 9728, 2560); got != quantTileRows {
		t.Fatalf("K = 2560: %d satır, beklenen %d", got, quantTileRows)
	}
	if got := quantTile(512, 2560, 9728); got != 416 {
		t.Fatalf("K = 9728: %d satır, beklenen 416", got)
	}
}

// TestQuantMatMulDependsOnlyOnShape: aynı girdi her koşuda aynı bitleri verir; satır dilimi ağırlığın
// kendi satırlarını okur.
func TestQuantMatMulDependsOnlyOnShape(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	w := FromQuantized(Q6K, smallScaleQuant(Q6K, 5, 40*2), 40, 512)
	x := New(randomData(r, 4*512), 4, 512)
	first := MatMul(x, w.T()).Data()
	for range 3 {
		sameFloatBits(t, "yeniden", MatMul(x, w.T()).Data(), first)
	}
	row := New(randomData(r, 512), 1, 512)
	part := MatMul(row, w.Slice(0, 10, 20).T()).Data()
	sameFloatBits(t, "satır dilimi", part, MatMul(row, w.T()).Data()[10:20])
}

// TestQuantMatMulIndependentOfWorkers (15.22a): sonuç işçi sayısından bağımsızdır, tek ve çok satırda.
func TestQuantMatMulIndependentOfWorkers(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	r := rand.New(rand.NewSource(9))
	for dt := range quantDotShapes {
		elems, _ := dt.QuantBlock()
		for _, c := range [][3]int{{1, 700, 512}, {1, 2*quantTileRows + 3, 1024}, {7, quantTileRows + 40, 512}, {130, 300, 256}} {
			M, N, K := c[0], c[1], c[2]
			w := FromQuantized(dt, smallScaleQuant(dt, uint32(N), N*K/elems), N, K)
			x := New(randomData(r, M*K), M, K)
			runtime.GOMAXPROCS(1)
			want := MatMul(x, w.T()).Data()
			for _, n := range []int{2, 3, 8} {
				runtime.GOMAXPROCS(n)
				sameFloatBits(t, dt.Name(), MatMul(x, w.T()).Data(), want)
			}
		}
	}
}

// TestQuantMatMulPooledBuffersKeepNoData: çok satırlı çarpımın tamponları havuzdandır; büyük şekilden
// sonra küçüğü, küçükten sonra büyüğü ilk koşudaki bitleri verir, eski veri okunmaz.
func TestQuantMatMulPooledBuffersKeepNoData(t *testing.T) {
	r := rand.New(rand.NewSource(10))
	shapes := [][3]int{{3, 70, 256}, {40, quantTileRows + 9, 512}, {2, 5, 256}}
	run := func(c [3]int, x *Tensor) []float32 {
		w := FromQuantized(Q4K, smallScaleQuant(Q4K, uint32(c[1]), c[1]*c[2]/kBlock), c[1], c[2])
		return MatMul(x, w.T()).Data()
	}
	xs := make([]*Tensor, len(shapes))
	first := make([][]float32, len(shapes))
	for i, c := range shapes {
		xs[i] = New(randomData(r, c[0]*c[2]), c[0], c[2])
		first[i] = run(c, xs[i])
	}
	for _, i := range []int{1, 0, 2, 1, 2, 0} {
		sameFloatBits(t, "art arda", run(shapes[i], xs[i]), first[i])
	}
}

// TestQuantMatMulKeepsLeftPrecision: sonuç x'in hassasiyetindedir (15.53), ağırlık değişmez.
func TestQuantMatMulKeepsLeftPrecision(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	w := FromQuantized(Q8, smallScaleQuant(Q8, 6, 4*2), 4, 64)
	for _, dt := range []DType{BFloat16, Float16} {
		x := NewOf(dt, randomData(r, 128), 2, 64)
		got := MatMul(x, w.T())
		if got.DType() != dt {
			t.Fatalf("%s: sonuç %v", dt.Name(), got.DType())
		}
		want := MatMulOut(x.Cast(Float32), openedWeight(w).T(), Float32).Cast(dt)
		withinSumBound(t, dt.Name(), got.Data(), want.Data(), 2048)
	}
}

// TestQuantEmbedding: seçilen satırlar açılmış tablonun satırlarıdır, float32'dir.
func TestQuantEmbedding(t *testing.T) {
	for dt := range quantDotShapes {
		elems, _ := dt.QuantBlock()
		w := FromQuantized(dt, randomQuant(dt, 9, 5*512/elems), 5, 512)
		all := openedWeight(w).Data()
		got := Embedding(w, []int{4, 0, 4, 2}, 2, 2)
		if s := got.Shape(); len(s) != 3 || s[2] != 512 || got.DType() != Float32 {
			t.Fatalf("%s: %v %v", dt.Name(), s, got.DType())
		}
		data := got.Data()
		for k, r := range []int{4, 0, 4, 2} {
			sameFloatBits(t, dt.Name(), data[k*512:(k+1)*512], all[r*512:(r+1)*512])
		}
	}
}

// TestQuantMatMulOneRowWeight tek satır diliminin devriğini çarpar: görünüm yalnız sağ işlenen olarak
// okunur, satırın ofseti korunur.
func TestQuantMatMulOneRowWeight(t *testing.T) {
	q := FromQuantized(Q8, quantBlocks(Q8, 1)[:6*q8Bytes], 3, 64)
	x := make([]float32, 64)
	for i := range x {
		x[i] = float32(i%5) - 2
	}
	row := q.Slice(0, 1, 2)
	want := MatMul(New(x, 1, 64), row.Open(Float32).T()).Data()[0]
	if got := MatMul(New(x, 1, 64), row.T()).Data(); len(got) != 1 || got[0] != want {
		t.Errorf("sonuç %v; beklenen %v", got, want)
	}
}

// TestQuantMatMulEmptySum boş toplamdır (K = 0): float çarpımı gibi sıfır verir.
func TestQuantMatMulEmptySum(t *testing.T) {
	w := FromQuantized(Q8, nil, 3, 0)
	for _, m := range []int{1, 2} {
		got := MatMul(New(nil, m, 0), w.T())
		if s := got.Shape(); len(s) != 2 || s[0] != m || s[1] != 3 {
			t.Fatalf("M = %d: şekil %v", m, s)
		}
		for i, v := range got.Data() {
			if v != 0 {
				t.Errorf("M = %d: öğe %d = %v", m, i, v)
			}
		}
	}
}

func TestQuantMatMulRefuses(t *testing.T) {
	w := func() *Tensor { return ZerosOf(Q8, 2, 64) }
	x := func() *Tensor { return New(make([]float32, 64), 1, 64) }
	for _, c := range []struct {
		name string
		code messages.Code
		run  func()
	}{
		{"devriksiz", messages.H6695, func() { MatMul(New(make([]float32, 2), 1, 2), w()) }},
		{"türev izleyen girdi", messages.H6696, func() { MatMul(x().SetRequiresGrad(true), w().T()) }},
		{"float64 girdi", messages.H6960, func() { MatMul(ZerosOf(Float64, 1, 64), w().T()) }},
		{"nicemlenmiş sol", messages.H6687, func() { MatMul(w(), w().T()) }},
		{"devrik tablo", messages.H6688, func() { Embedding(w().T(), []int{0}) }},
		{"sınır dışı satır", messages.H3043, func() { Embedding(w(), []int{2}) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if err, _ := recover().(error); !messages.Has(err, c.code) {
					t.Errorf("hata %v; %s olmalı", err, c.code)
				}
			}()
			c.run()
		})
	}
	NoGrad(func() {
		if got := MatMul(x().SetRequiresGrad(true), w().T()); got.Shape()[1] != 2 {
			t.Errorf("türevsiz kip: %v", got.Shape())
		}
	})
}
