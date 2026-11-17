package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// q8Paths nicemli çekirdeklerin üç yoludur: AVX2 çekirdeği, F16C yok (Go eşine düşer), Go. Testler çekirdeği
// sarmalayıcıdan çağırır: amd64 dışında da derlenir, Go eşleri (dört satırlılar dahil) AVX2'li makinede de koşar.
func q8Paths() []struct {
	name      string
	avx, f16c bool
} {
	return []struct {
		name      string
		avx, f16c bool
	}{{"çekirdek", hasAVX2, hasF16C}, {"F16C yok", hasAVX2, false}, {"Go", false, false}}
}

// q8GoDot dt'nin tek satırlı Go eşidir.
func q8GoDot(dt DType) func(int, []byte, *q8kRow) float32 {
	switch dt {
	case Q4:
		return dotQ4Q8Go
	case Q6K:
		return dotQ6KQ8Go
	}
	return dotQ4KQ8Go
}

// q8Types etkinleştirmesi nicemlenen çarpımın ağırlık türleridir (QuantActSupported).
var q8Types = []DType{Q4, Q4K, Q6K}

// openedDot x·aç(w)'yi float64'te ve |x·aç(w)|'nin toplamını verir (nicemleme hatasının ölçeği).
func openedDot(dt DType, x []float32, w []byte) (dot, scale float64) {
	ref := make([]float32, len(x))
	dequantBlocks(dt, ref, w)
	for i := range x {
		dot += float64(x[i]) * float64(ref[i])
		scale += math.Abs(float64(x[i]) * float64(ref[i]))
	}
	return dot, scale
}

// TestQ8KernelsMatchGo: tek satırlı nicemli çekirdekler (Q4_0, Q4_K, Q6_K) her yolda Go eşiyle bit bit aynıdır ve
// açılmış float çarpımına yakındır (nicemleme hatası kadar); ×1000 ölçekli girdiler dahil. H1 hız prototipi.
func TestQ8KernelsMatchGo(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	for _, path := range q8Paths() {
		for _, dt := range q8Types {
			r := rand.New(rand.NewSource(31))
			blk := actBlock(dt)
			for _, K := range []int{32, 96, 256, 512, 2560, 9728} {
				if K%blk != 0 {
					continue
				}
				for trial := range 20 {
					hasAVX2, hasF16C = path.avx, path.f16c
					w := smallScaleQuant(dt, uint32(K+trial), K/blk)
					x := randomData(r, K)
					if trial%5 == 0 {
						for i := range x {
							x[i] *= 1000
						}
					}
					q := newQ8Row(K, blk)
					quantizeQ8(x, q)
					var tmp [q8Tmp]byte
					got := dotQ8(dt, K/blk, w, q, tmp[:])
					want := q8GoDot(dt)(K/blk, w, q)
					if math.Float32bits(got) != math.Float32bits(want) {
						t.Fatalf("%s %s K=%d deneme %d: çekirdek %v (%#x), Go %v (%#x)", path.name, dt.Name(), K, trial,
							got, math.Float32bits(got), want, math.Float32bits(want))
					}
					if f, n2 := openedDot(dt, x, w); math.Abs(float64(got)-f) > 0.02*n2+1e-6 {
						t.Fatalf("%s %s K=%d deneme %d: nicemli %v, float %v (ölçek %v)", path.name, dt.Name(), K, trial, got, f, n2)
					}
				}
			}
		}
	}
}

// TestQ8FourRowsMatchOneRow: quantGemmQ8 (dört satırlı çekirdek ve kalan satırlar, q8BlockN'lik bloklar ve eksik
// son blok) her satırda quantGemvQ8 ile bit bit aynıdır; Q4_0, Q4_K ve Q6_K'da, her yolda. Yolların sonuçları da
// birbirinin aynısıdır (çekirdek = Go eşi).
func TestQ8FourRowsMatchOneRow(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	for _, dt := range q8Types {
		for _, c := range [][3]int{{5, 7, 96}, {4, 3, 256}, {7, 17, 512}, {9, 40, 2560}, {13, 5, 9728}, {21, 33, 512}} {
			M, N, K := c[0], c[1], c[2]
			if K%actBlock(dt) != 0 {
				continue
			}
			w := smallScaleQuant(dt, uint32(M+N+K), N*K/actBlock(dt))
			x := randomData(rand.New(rand.NewSource(int64(M*N+K))), M*K)
			var first []float32
			for _, path := range q8Paths() {
				hasAVX2, hasF16C = path.avx, path.f16c
				got := make([]float32, M*N)
				quantGemmQ8(M, N, K, x, w, dt, got)
				for m := range M {
					want := make([]float32, N)
					quantGemvQ8(N, K, x[m*K:(m+1)*K], w, dt, want)
					sameFloatBits(t, path.name+" "+dt.Name()+" satır", got[m*N:(m+1)*N], want)
				}
				if first == nil {
					first = got
				}
				sameFloatBits(t, path.name+" "+dt.Name()+" yollar", got, first)
			}
		}
	}
}

// TestQ8EightRowsMatchOneRow: Q4_K'da sekiz satırlı çekirdek (sekizli gruplar, ardından dörtlü ve tek satırlar)
// her satırda quantGemvQ8 ile ve Go yoluyla bit bit aynıdır; ×1000 ölçekli, sıfır bloklu ve ±1 satırları dahil.
func TestQ8EightRowsMatchOneRow(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	for _, c := range [][3]int{{8, 3, 256}, {12, 17, 512}, {16, 33, 2560}, {23, 5, 9728}, {40, 18, 512}} {
		M, N, K := c[0], c[1], c[2]
		w := smallScaleQuant(Q4K, uint32(7*M+N+K), N*K/kBlock)
		x := randomData(rand.New(rand.NewSource(int64(M+N*K))), M*K)
		for m := range M {
			row := x[m*K : (m+1)*K]
			switch m % 5 {
			case 1:
				for i := range row {
					row[i] *= 1000
				}
			case 2:
				clear(row[:kBlock])
			case 3:
				for i := range row {
					row[i] = float32(1 - 2*(i%2))
				}
			}
		}
		var first []float32
		for _, path := range q8Paths() {
			hasAVX2, hasF16C = path.avx, path.f16c
			got := make([]float32, M*N)
			quantGemmQ8(M, N, K, x, w, Q4K, got)
			for m := range M {
				want := make([]float32, N)
				quantGemvQ8(N, K, x[m*K:(m+1)*K], w, Q4K, want)
				sameFloatBits(t, path.name+" satır", got[m*N:(m+1)*N], want)
			}
			if first == nil {
				first = got
			}
			sameFloatBits(t, path.name+" yollar", got, first)
		}
	}
}

// TestQ8EightRowsChecksBounds: sekiz satırlı çekirdek çağıranın sözüne dayanmaz: r0'ın arkasında yedi satır yoksa
// ya da out sekiz öğeden kısaysa her yolda (çekirdek ve Go eşi) panik verir.
func TestQ8EightRowsChecksBounds(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	const K = 512
	nb := K / kBlock
	w := smallScaleQuant(Q4K, 9, nb)
	dT, ps := make([]float32, nb*8), make([]int16, nb*64)
	for _, path := range q8Paths() {
		hasAVX2, hasF16C = path.avx, path.f16c
		var tmp [q8Tmp]byte
		for _, c := range []struct {
			name string
			r0   *q8kRow
			out  []float32
		}{{"yedi satır", newQ8KRow(7 * K), make([]float32, 8)}, {"kısa out", newQ8KRow(8 * K), make([]float32, 7)}} {
			func() {
				defer func() {
					if recover() == nil {
						t.Fatalf("%s %s: panik yok", path.name, c.name)
					}
				}()
				dotQ4KQ8x8(nb, w, c.r0, K, dT, ps, tmp[:], c.out)
			}()
		}
	}
}

// TestQ8FourRowsChecksBounds: dört satırlı çekirdekler çağıranın sözüne dayanmaz: r0'ın arkasında üç satır yoksa
// ya da out dört öğeden kısaysa her yolda panik verir (assembly tamponun dışını okumaz).
func TestQ8FourRowsChecksBounds(t *testing.T) {
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	const K = 512
	for _, path := range q8Paths() {
		hasAVX2, hasF16C = path.avx, path.f16c
		for _, dt := range q8Types {
			blk := actBlock(dt)
			w := smallScaleQuant(dt, 5, K/blk)
			var tmp [q8Tmp]byte
			for _, c := range []struct {
				name string
				r0   *q8kRow
				out  []float32
			}{{"tek satır", newQ8Row(K, blk), make([]float32, 4)}, {"kısa out", newQ8Row(4*K, blk), make([]float32, 3)}} {
				func() {
					defer func() {
						if recover() == nil {
							t.Fatalf("%s %s %s: panik yok", path.name, dt.Name(), c.name)
						}
					}()
					dotQ8x4(dt)(K/blk, w, c.r0, K, tmp[:], c.out)
				}()
			}
		}
	}
}

// TestQuantizeQ8KEdgeBlocks: nicemlemenin kenar blokları: tamamı sıfır (kod 0, ölçek 0), ±127'ye doyan (bütün
// öğeler ±max), tek büyük öğe (öbürleri 0'a iner), en büyüğü ~3,7e−37'nin altında olan blok (127 / max float32'de
// taşar: 15.53 taslağının formülüyle kodlar 0, ölçek 0, katkı tam 0). Her blokta çekirdek her yolda Go eşiyle
// aynıdır; sıfıra inmeyen blokta sonuç açılmış float çarpımına yakındır.
func TestQuantizeQ8KEdgeBlocks(t *testing.T) {
	tiny := math.Float32frombits(0x00400000) // 2^−127, alt normal: yarısı ve çeyreği tam
	cases := []struct {
		name  string
		fill  func(x []float32)
		codes map[int]int8 // gösterilmeyen öğenin kodu 0
		d     float32
	}{
		{"sıfır", func(x []float32) {}, nil, 0},
		{"doyan", func(x []float32) {
			for i := range x {
				x[i] = 3
				if i%2 == 1 {
					x[i] = -3
				}
			}
		}, nil, float32(3) / 127},
		{"tek büyük", func(x []float32) {
			for i := range x {
				x[i] = 1e-3
			}
			x[5] = 1000
		}, map[int]int8{5: 127}, float32(1000) / 127},
		{"küçük", func(x []float32) { x[0], x[1], x[2] = tiny, -tiny/2, tiny/4 }, nil, 0},
	}
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	for _, c := range cases {
		x := make([]float32, kBlock)
		c.fill(x)
		q := newQ8KRow(kBlock)
		quantizeQ8(x, q)
		for i, code := range q.q {
			want, ok := c.codes[i]
			switch {
			case c.name == "doyan":
				want = 127
				if i%2 == 1 {
					want = -127
				}
			case !ok:
				want = 0
			}
			if code != want {
				t.Fatalf("%s: kod[%d] = %d, beklenen %d", c.name, i, code, want)
			}
		}
		if q.d[0] != c.d {
			t.Fatalf("%s: ölçek %v, beklenen %v", c.name, q.d[0], c.d)
		}
		for s, sum := range q.bs {
			var want int16
			for _, v := range q.q[s*16 : s*16+16] {
				want += int16(v)
			}
			if sum != want {
				t.Fatalf("%s: alt toplam %d = %d, beklenen %d", c.name, s, sum, want)
			}
		}
		for _, dt := range []DType{Q4K, Q6K} {
			w := smallScaleQuant(dt, 77, 1)
			var first float32
			for i, path := range q8Paths() {
				hasAVX2, hasF16C = path.avx, path.f16c
				var tmp [q8Tmp]byte
				got := dotQ8(dt, 1, w, q, tmp[:])
				if want := q8GoDot(dt)(1, w, q); math.Float32bits(got) != math.Float32bits(want) {
					t.Fatalf("%s %s %s: çekirdek %v, Go %v", c.name, path.name, dt.Name(), got, want)
				}
				if i == 0 {
					first = got
				} else if math.Float32bits(got) != math.Float32bits(first) {
					t.Fatalf("%s %s %s: yol sonucu %v, ilk yol %v", c.name, path.name, dt.Name(), got, first)
				}
			}
			if c.name == "küçük" {
				if first != 0 {
					t.Fatalf("küçük %s: taşan ölçekli blok %v verdi, beklenen 0", dt.Name(), first)
				}
			} else if f, n2 := openedDot(dt, x, w); math.Abs(float64(first)-f) > 0.02*n2 {
				t.Fatalf("%s %s: nicemli %v, float %v (ölçek %v)", c.name, dt.Name(), first, f, n2)
			}
		}
	}
}

// TestQuantizeQ8BlockEdges: "q4" ağırlıkta etkinleştirme 32'lik bloklarla nicemlenir (15.53a): kenar bloklar
// (sıfır, ±127'ye doyan, tek büyük öğe, 127 / max'ı taşan alt normal) 256'lıktakiyle aynı kuralda; her bloğun
// ölçeği yalnız kendi öğelerinden gelir (yanındaki bloğun 1000'i küçük bloğun kodlarını ezmez). Çekirdek her
// yolda Go eşiyle aynıdır; sıfıra inmeyen blokta sonuç açılmış float çarpımına yakındır.
func TestQuantizeQ8BlockEdges(t *testing.T) {
	tiny := math.Float32frombits(0x00400000) // 2^−127, alt normal
	cases := []struct {
		name  string
		fill  func(x []float32)
		codes func(i int) int8
		d     float32
	}{
		{"sıfır", func(x []float32) {}, func(int) int8 { return 0 }, 0},
		{"doyan", func(x []float32) {
			for i := range x {
				x[i] = float32(3 - 6*(i%2))
			}
		}, func(i int) int8 { return int8(127 - 254*(i%2)) }, float32(3) / 127},
		{"tek büyük", func(x []float32) {
			for i := range x {
				x[i] = 1e-3
			}
			x[31] = -1000
		}, func(i int) int8 { return map[bool]int8{true: -127}[i == 31] }, float32(1000) / 127},
		{"taşan oran", func(x []float32) { x[0], x[7], x[31] = tiny, -tiny/2, tiny/4 }, func(int) int8 { return 0 }, 0},
	}
	oldAVX, oldF16C := hasAVX2, hasF16C
	defer func() { hasAVX2, hasF16C = oldAVX, oldF16C }()
	w := smallScaleQuant(Q4, 78, 1)
	for _, c := range cases {
		x := make([]float32, 32)
		c.fill(x)
		q := newQ8Row(32, actBlock(Q4))
		quantizeQ8(x, q)
		for i, code := range q.q {
			if want := c.codes(i); code != want {
				t.Fatalf("%s: kod[%d] = %d, beklenen %d", c.name, i, code, want)
			}
		}
		if len(q.d) != 1 || q.d[0] != c.d {
			t.Fatalf("%s: ölçekler %v, beklenen [%v]", c.name, q.d, c.d)
		}
		var first float32
		for i, path := range q8Paths() {
			hasAVX2, hasF16C = path.avx, path.f16c
			got := dotQ4Q8(1, w, q)
			if want := dotQ4Q8Go(1, w, q); math.Float32bits(got) != math.Float32bits(want) {
				t.Fatalf("%s %s: çekirdek %v, Go %v", c.name, path.name, got, want)
			}
			if i == 0 {
				first = got
			} else if math.Float32bits(got) != math.Float32bits(first) {
				t.Fatalf("%s %s: yol sonucu %v, ilk yol %v", c.name, path.name, got, first)
			}
		}
		if c.d == 0 {
			if math.Float32bits(first) != 0 {
				t.Fatalf("%s: sıfıra inen blok %v verdi, beklenen +0", c.name, first)
			}
		} else if f, n2 := openedDot(Q4, x, w); math.Abs(float64(first)-f) > 0.02*n2 {
			t.Fatalf("%s: nicemli %v, float %v (ölçek %v)", c.name, first, f, n2)
		}
	}
	// İki blok: ilki 0,5'ler (ölçek 1/127, kod 63,5 → çifte 64), ikincisi 1000'ler.
	x := make([]float32, 64)
	for i := range x {
		x[i] = 0.5
		if i >= 32 {
			x[i] = 1000
		}
	}
	x[0] = 1
	q := newQ8Row(64, actBlock(Q4))
	quantizeQ8(x, q)
	if q.d[0] != float32(1)/127 || q.d[1] != float32(1000)/127 || q.q[0] != 127 || q.q[1] != 64 || q.q[32] != 127 {
		t.Fatalf("iki blok: ölçekler %v, kodlar %d %d %d", q.d, q.q[0], q.q[1], q.q[32])
	}
}

// actMatMul etkinleştirmesi nicemlenen çarpımdır (TR_quantized_activations bloğunun yolu).
func actMatMul(x, w *Tensor) *Tensor {
	r, k := MatMulActQuantKernel(x, w, MatMulType(x, w))
	k.RunLocked()
	return r
}

// TestQ8PathKeepsNonFinite: nicemli yolda sonlu olmayan değer (NaN, ±∞) içeren satır float yoluyla
// hesaplanır: sonucu o satırın tek başına float yolundaki sonucuyla bit bit aynıdır. Karar satır satır: öbür
// satırların sonucu, yanlarındaki satır sonlu olsa da olmasa da bit bit aynıdır. Sonlu girdide nicemli yol
// gerçekten kullanılır (sonuç farklıdır).
func TestQ8PathKeepsNonFinite(t *testing.T) {
	r := rand.New(rand.NewSource(34))
	for _, dt := range q8Types {
		for _, M := range []int{1, 5, 9} {
			N, K := 9, 512
			w := FromQuantized(dt, smallScaleQuant(dt, uint32(M+N), N*K/actBlock(dt)), N, K)
			for _, bad := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
				clean := randomData(r, M*K)
				data := append([]float32(nil), clean...)
				badRow := M / 2
				data[badRow*K+K/2] = bad
				got := actMatMul(New(data, M, K), w.T()).Data()
				wantClean := actMatMul(New(clean, M, K), w.T()).Data()
				wantBad := MatMul(New(data[badRow*K:(badRow+1)*K], 1, K), w.T()).Data()
				for m := range M {
					if m == badRow {
						sameFloatBits(t, dt.Name()+" sonlu olmayan satır", got[m*N:(m+1)*N], wantBad)
					} else {
						sameFloatBits(t, dt.Name()+" komşusu sonlu olmayan satır", got[m*N:(m+1)*N], wantClean[m*N:(m+1)*N])
					}
				}
			}
			x := New(randomData(r, M*K), M, K)
			float := MatMul(x, w.T()).Data()
			quant := actMatMul(x, w.T()).Data()
			same := true
			for i := range float {
				if math.Float32bits(float[i]) != math.Float32bits(quant[i]) {
					same = false
				}
			}
			if same {
				t.Fatalf("%s M=%d: nicemli yol kullanılmadı (sonuç float yolunun aynısı)", dt.Name(), M)
			}
		}
	}
}
