package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// TestQ8GemvW4AndQuantizeMatchGo: tek satırlı nicemli çarpım her yolda (VNNI, AVX2, F16C yok, Go; q8Paths) Go eşiyle bit
// bit aynıdır: VNNI'de dört ağırlık satırlı çekirdekler (dotQ4KQ8W4, dotQ6KQ8W4), AVX2'de tek satırlı çekirdek (dotQ8W4); AVX2 quantizeQ8 çifte yuvarlamanın yarılarında
// (±0,5, ±63,5, ±126,5) ve 127'de Go eşiyle aynıdır; q8Finite NaN ve ±∞'u her konumda Go eşi gibi bulur; kısa tamponda
// her yol assembly'ye girmeden panik verir. VNNI'siz makinede VNNI yolu atlandığını yazar (DENETİM 958 Ş1).
func TestQ8GemvW4AndQuantizeMatchGo(t *testing.T) {
	defer restoreQ8Path()
	r := rand.New(rand.NewSource(91))
	for _, blk := range []int{32, kBlock} {
		for _, K := range []int{32, 96, 256, 512, 2560} {
			if K%blk != 0 {
				continue
			}
			x := randomData(r, K)
			copy(x, []float32{127, 0.5, -0.5, 63.5, -63.5, 126.5, -126.5})
			got, want := newQ8Row(K, blk), newQ8Row(K, blk)
			hasAVX2 = q8MachineAVX2
			quantizeQ8(x, got)
			quantizeQ8Go(x, want)
			sameQ8Row(t, "quantizeQ8", got, want)
		}
	}
	for _, path := range q8Paths() {
		t.Run(path.name, func(t *testing.T) {
			path.set(t)
			for _, dt := range []DType{Q4K, Q6K} {
				for _, N := range []int{1, 3, 4, 5, 16, 19, 35} {
					for _, K := range []int{256, 512, 2560} {
						w := smallScaleQuant(dt, uint32(N+K), N*K/kBlock)
						x := randomData(r, K)
						q := newQ8KRow(K)
						quantizeQ8Go(x, q)
						got := make([]float32, N)
						quantGemvQ8(N, K, x, w, dt, got)
						want := make([]float32, N)
						rb := quantRowBytes(dt, K)
						for n := range N {
							want[n] = q8GoDot(dt)(K/kBlock, w[n*rb:(n+1)*rb], q)
						}
						sameFloatBits(t, path.name+" "+dt.Name()+" gemvW4", got, want)
					}
				}
				q8W4ShortBuffers(t, dt)
			}
		})
	}
	q8FiniteMatchesGo(t)
}

// q8W4ShortBuffers: dörtlü sarmalayıcı kısa ağırlık ya da çıkış tamponunda panik verir (o anki yolda). Sınır
// sığaya göre denetlenir (öbür çekirdeklerdeki gibi yeniden dilimleme, assembly ayrılmış belleğin dışını okumaz): kısa
// tampon sığasıyla birlikte kısadır.
func q8W4ShortBuffers(t *testing.T, dt DType) {
	const K = 512
	rb := quantRowBytes(dt, K)
	w := smallScaleQuant(dt, 7, 4*K/kBlock)
	q := newQ8KRow(K)
	var tmp [q8Tmp]byte
	for _, c := range []struct {
		name string
		w    []byte
		out  []float32
	}{{"kısa ağırlık", w[: len(w)-1 : len(w)-1], make([]float32, 4)}, {"kısa çıkış", w, make([]float32, 3)}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s %s: panik bekleniyordu", dt.Name(), c.name)
				}
			}()
			// Sarmalayıcı doğrudan: dotQ8W4 VNNI'siz yolda nil döner, nil çağrısının paniği sınırı sınamazdı.
			w4 := dotQ4KQ8W4
			if dt == Q6K {
				w4 = dotQ6KQ8W4
			}
			w4(K/kBlock, c.w, rb, q, tmp[:], c.out)
		}()
	}
}

// q8FiniteMatchesGo: q8Finite (AVX2 yolu ve kuyruğu) NaN ve ±∞'u her konumda bulur, sonlu diziyi sonlu sayar;
// q8FiniteAsm 8'den kısa girdide hiç okumadan sonlu döner (N7).
func q8FiniteMatchesGo(t *testing.T) {
	hasAVX2 = q8MachineAVX2
	for _, n := range []int{7, 8, 9, 64, 65} {
		x := randomData(rand.New(rand.NewSource(int64(n))), n)
		if !q8Finite(x) || !q8FiniteGo(x) {
			t.Fatalf("n=%d: sonlu dizi sonlu sayılmadı", n)
		}
		for _, bad := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
			for _, pos := range []int{0, 7, 8, n - 1} {
				if pos >= n {
					continue
				}
				y := append([]float32(nil), x...)
				y[pos] = bad
				if q8Finite(y) || q8FiniteGo(y) {
					t.Fatalf("n=%d konum %d (%v): sonlu sayıldı", n, pos, bad)
				}
			}
		}
	}
	if q8MachineAVX2 {
		nan := []float32{float32(math.NaN()), 0, 0, 0, 0, 0, 0, 0}
		for n := range 8 {
			if !q8FiniteAsm(&nan[0], n) {
				t.Fatalf("q8FiniteAsm n=%d: okumadan sonlu dönmeliydi", n)
			}
		}
		if q8FiniteAsm(&nan[0], 8) {
			t.Fatal("q8FiniteAsm n=8: NaN bulunmadı")
		}
	}
}

// sameQ8Row iki nicemli satırın kodlarını, ölçeklerini ve alt blok toplamlarını karşılaştırır.
func sameQ8Row(t *testing.T, what string, got, want *q8kRow) {
	t.Helper()
	sameFloatBits(t, what+" d", got.d, want.d)
	for i := range want.q {
		if got.q[i] != want.q[i] {
			t.Fatalf("%s: kod[%d] = %d, Go %d", what, i, got.q[i], want.q[i])
		}
	}
	for i := range want.bs {
		if got.bs[i] != want.bs[i] {
			t.Fatalf("%s: bs[%d] = %d, Go %d", what, i, got.bs[i], want.bs[i])
		}
	}
}

// TestQuantizeQ8OverwritesDirtyRow: havuzdan (q8RowPool) gelen satır önceki çarpımın kodlarını taşır; nicemleme her
// yolda kodların, ölçeklerin ve alt blok toplamlarının hepsini yazar: sıfır blokta, 127/max'ı taşan alt normal blokta
// ve olağan blokta sonuç temiz satırdakiyle aynıdır.
func TestQuantizeQ8OverwritesDirtyRow(t *testing.T) {
	defer restoreQ8Path()
	tiny := math.Float32frombits(0x00400000) // 2^−127: 127 / max float32'de taşar
	for _, blk := range []int{32, kBlock} {
		K := 4 * kBlock
		x := randomData(rand.New(rand.NewSource(int64(blk))), K)
		for i := range blk {
			x[i], x[blk+i] = 0, tiny
		}
		want := newQ8Row(K, blk)
		quantizeQ8Go(x, want)
		for _, avx := range []bool{q8MachineAVX2, false} {
			hasAVX2 = avx
			got := newQ8Row(K, blk)
			for i := range got.q {
				got.q[i] = 0x55
			}
			for i := range got.d {
				got.d[i] = 7
			}
			for i := range got.bs {
				got.bs[i] = 1234
			}
			quantizeQ8(x, got)
			sameQ8Row(t, "kirli satır", got, want)
		}
	}
}

// TestQ8GemvSameBitsAnyWorkers: tek satırlı nicemli çarpımın (karo, işçi tavanı) bitleri işçi sayısından bağımsızdır.
func TestQ8GemvSameBitsAnyWorkers(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	for _, dt := range q8Types {
		const N, K = 2048, 2560
		w := smallScaleQuant(dt, 3, N*K/actBlock(dt))
		x := randomData(rand.New(rand.NewSource(8)), K)
		var first []float32
		for _, p := range []int{1, 2, 7, max(prev, 8)} {
			runtime.GOMAXPROCS(p)
			got := make([]float32, N)
			quantGemvQ8(N, K, x, w, dt, got)
			if first == nil {
				first = got
				continue
			}
			sameFloatBits(t, dt.Name(), got, first)
		}
	}
}
