package tensor

import (
	"math/rand"
	"testing"
)

var q8MachineAVXVNNI = hasAVXVNNI

func q8PanelPaths() []struct {
	name            string
	avx, f16c, vnni bool
} {
	return []struct {
		name            string
		avx, f16c, vnni bool
	}{
		{"VNNI", q8MachineAVX2, q8MachineF16C, q8MachineAVXVNNI},
		{"AVX2 panel", q8MachineAVX2, q8MachineF16C, false},
		{"Go", false, false, false},
	}
}

// TestQ4KAndQ6KPanelMatchesGo: sekiz satırlı Q4_K ve Q6_K ağırlık panelleri (VNNI yolu, AVX2 panel yolu ve Go yolu)
// her M, N, K (tam sekizliler ve artık satırlar, tam dörtlüler ve artık etkinleştirmeler) boyutunda dotQ4KQ8Go ve
// dotQ6KQ8Go ile bit bit aynı sonucu verir.
func TestQ4KAndQ6KPanelMatchesGo(t *testing.T) {
	oldAVX, oldF16C, oldVNNI := hasAVX2, hasF16C, hasAVXVNNI
	defer func() { hasAVX2, hasF16C, hasAVXVNNI = oldAVX, oldF16C, oldVNNI }()
	for _, dt := range []DType{Q4K, Q6K} {
		for _, sh := range [][3]int{
			{4, 8, 256},
			{5, 8, 512},
			{8, 16, 2560},
			{9, 13, 512},
			{12, 29, 512},
			{15, 35, 2560},
			{7, 3, 256},
		} {
			M, N, K := sh[0], sh[1], sh[2]
			w := smallScaleQuant(dt, uint32(11*M+3*N+K), N*K/kBlock)
			x := randomData(rand.New(rand.NewSource(int64(13*M+N*K))), M*K)
			rb := quantRowBytes(dt, K)
			want := make([]float32, M*N)
			for m := range M {
				q := newQ8KRow(K)
				quantizeQ8Go(x[m*K:(m+1)*K], q)
				for n := range N {
					want[m*N+n] = q8GoDot(dt)(K/kBlock, w[n*rb:(n+1)*rb], q)
				}
			}
			for _, p := range q8PanelPaths() {
				hasAVX2, hasF16C, hasAVXVNNI = p.avx, p.f16c, p.vnni
				got := make([]float32, M*N)
				quantGemmQ8(M, N, K, x, w, dt, got)
				sameFloatBits(t, p.name+" "+dt.Name()+" panel", got, want)
				gotV := make([]float32, N)
				quantGemvQ8(N, K, x[:K], w, dt, gotV)
				sameFloatBits(t, p.name+" "+dt.Name()+" gemv", gotV, want[:N])
			}
		}
	}
}

// TestQ4KAndQ6KPanelChecksBounds: Q4_K ve Q6_K panel sarmalayıcıları kısa tamponda assembly'ye girmeden panik verir.
func TestQ4KAndQ6KPanelChecksBounds(t *testing.T) {
	if !q8MachineAVX2 || !q8MachineF16C {
		t.Skip("AVX2 ya da F16C yok")
	}
	const K, N = 512, 8
	nb := K / kBlock
	all := newQ8Row(4*K, kBlock)
	rows := make([]*q8kRow, 4)
	for i := range rows {
		rows[i] = &q8kRow{q: all.q[i*K : (i+1)*K], d: all.d[i*nb : (i+1)*nb], bs: all.bs[i*nb*16 : (i+1)*nb*16], blk: kBlock}
	}
	idx, c := []int{0, 1, 2, 3}, make([]float32, 4*N)
	w4 := smallScaleQuant(Q4K, 17, N*nb)
	rb4 := quantRowBytes(Q4K, K)
	bs32 := make([]int16, 4*nb*8)
	for _, fn := range []func(){
		func() {
			q4kPanelGemm(0, N, N, K, w4, rb4, all, rows, bs32, idx, c, make([]byte, nb*q4kPanelCodeBytes-1), make([]byte, nb*q4kPanelMetaBytes))
		},
		func() {
			q4kPanelGemm(0, N, N, K, w4, rb4, all, rows, bs32, idx, c, make([]byte, nb*q4kPanelCodeBytes), make([]byte, nb*q4kPanelMetaBytes-1))
		},
		func() {
			q4kPanelGemm(0, N, N, K, w4, rb4, all, rows, bs32[:len(bs32)-1], idx, c, make([]byte, nb*q4kPanelCodeBytes), make([]byte, nb*q4kPanelMetaBytes))
		},
		func() {
			w6 := smallScaleQuant(Q6K, 19, N*nb)
			rb6 := quantRowBytes(Q6K, K)
			q6kPanelGemm(0, N, N, K, w6, rb6, all, rows, idx, c, make([]byte, nb*q6kPanelCodeBytes), make([]byte, nb*q6kPanelMetaBytes-1))
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("kısa tamponda panik bekleniyordu")
				}
			}()
			fn()
		}()
	}
}
