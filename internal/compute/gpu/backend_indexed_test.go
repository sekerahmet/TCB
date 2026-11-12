package gpu

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// indexCase bir gömme indeksi denemesi: konumdaki değer, beklenen çeşit ve hatalı ilk konum.
type indexCase struct {
	vals []float64
	kind tensor.FaultKind
	at   int
}

// indexTensor vals'ı dt türünde kartta 1 boyutlu indeks tensörü yapar.
func indexTensor(dt tensor.DType, vals []float64) *tensor.Tensor {
	switch dt {
	case tensor.Int64:
		iv := make([]int64, len(vals))
		for i, v := range vals {
			iv[i] = int64(v)
		}
		return tensor.Int64sOn(gpuDevice, dt, iv, len(vals))
	case tensor.Float64:
		return tensor.Float64sOn(gpuDevice, vals, len(vals))
	}
	fv := make([]float32, len(vals))
	for i, v := range vals {
		fv[i] = float32(v)
	}
	return tensor.NewOn(gpuDevice, fv, len(vals))
}

// Kural (15.45, 15.6; T1 dilim 2c): kayıtta gömmenin tensör indeksi kırpılmaz. Tam sayı olmayan
// (kesirli, NaN, ±∞, 2⁶³ ve üstü; float64 kendi türünde) ya da [0, V−1]'in dışındaki indeks
// cihaz içi bayrağa yazar, ilk hatalı öğe en küçük konumludur; değer ham bitlerden gelir.
// Hatalı oynatmadan sonraki hatasız oynatma doğrudur (yuva boşalır). Kayıt dışında aynı
// hata doğrudan döner.
func TestIndexedFaultsInsideRecording(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const V, D = 7, 3
	table := random(rand.New(rand.NewSource(3)), V*D, 1)
	w := tensor.NewOn(gpuDevice, table, V, D)
	two63 := math.Ldexp(1, 63)
	cases := map[tensor.DType][]indexCase{
		tensor.Float32: {
			{[]float64{1, 2.5, 3, 0}, tensor.FaultIndexNotInt, 1},
			{[]float64{1, 2, -1, 0}, tensor.FaultIndexRange, 2},
			{[]float64{V, 2, 3, 0}, tensor.FaultIndexRange, 0},
			{[]float64{0, 1, 2, math.NaN()}, tensor.FaultIndexNotInt, 3},
			{[]float64{0, math.Inf(1), 2, 1}, tensor.FaultIndexNotInt, 1},
			{[]float64{0, 1, math.Inf(-1), 1}, tensor.FaultIndexNotInt, 2},
			{[]float64{two63, 1, 2, 1}, tensor.FaultIndexNotInt, 0},
			{[]float64{-two63, 1, 2, 1}, tensor.FaultIndexRange, 0},
			{[]float64{5, 9, 2.5, -1}, tensor.FaultIndexRange, 1},
		},
		tensor.Float64: {
			{[]float64{1, 2.0000001, 3, 0}, tensor.FaultIndexNotInt, 1},
			{[]float64{1, 2, 3, V}, tensor.FaultIndexRange, 3},
		},
		tensor.Int64: {
			{[]float64{1, -1, 3, 0}, tensor.FaultIndexRange, 1},
			{[]float64{1, 2, V, 0}, tensor.FaultIndexRange, 2},
		},
	}
	for dt, cs := range cases {
		ids := indexTensor(dt, []float64{0, 1, 2, 3})
		if _, f := tensor.EmbeddingAt(w, ids); f != nil {
			t.Fatalf("%s ısınma: %+v", dt, f)
		}
		g.BeginCapture()
		out, rf := tensor.EmbeddingAt(w, ids)
		gr := g.EndCapture()
		if rf == nil || rf.Kind != tensor.FaultRecorded {
			t.Fatalf("%s kayıtta gömme: %+v, beklenen FaultRecorded", dt, rf)
		}
		fr := tensor.Recording(gr).(tensor.FaultRecording)
		for _, c := range cs {
			ids.CopyFrom(indexTensor(dt, c.vals))
			gr.Replay()
			site, f := fr.FirstFault()
			if f == nil {
				t.Fatalf("%s %v: hata yok; beklenen çeşit %d, konum %d", dt, c.vals, c.kind, c.at)
			}
			want := c.vals[c.at]
			got := f.Float
			if dt == tensor.Int64 {
				got = float64(f.Int)
			} else if dt == tensor.Float32 {
				want = float64(float32(want))
			}
			same := got == want || math.IsNaN(got) && math.IsNaN(want)
			if site != 0 || f.Kind != c.kind || f.Index != c.at || !same || f.DType != dt {
				t.Errorf("%s %v: yuva %d, %+v; beklenen çeşit %d, konum %d, değer %v", dt, c.vals, site, f, c.kind, c.at, want)
			}
			if _, df := tensor.EmbeddingAt(w, indexTensor(dt, c.vals)); df == nil || df.Kind != c.kind || df.Index != c.at {
				t.Errorf("%s %v kayıt dışında: %+v", dt, c.vals, df)
			}
			ids.CopyFrom(indexTensor(dt, []float64{6, 0, 5, 1}))
			gr.Replay()
			if site, f := fr.FirstFault(); site != -1 || f != nil {
				t.Fatalf("%s: hatalı oynatmadan sonra hatasız oynatma yuva %d, %+v", dt, site, f)
			}
			o := out.Data()
			for r, id := range []int{6, 0, 5, 1} {
				for j := 0; j < D; j++ {
					if o[r*D+j] != table[id*D+j] {
						t.Fatalf("%s: hatasız oynatmada [%d,%d] = %v, beklenen %v", dt, r, j, o[r*D+j], table[id*D+j])
					}
				}
			}
		}
		gr.Release()
	}
}

// Kural (15.45, 15.6; T1 dilim 2c): çok sayıda indekste rastgele konumlardaki hatalardan her
// oynatmada en küçük konumlusu söylenir (kartta atomların sırası sonucu değiştirmez).
// Benzeticide 2¹² indeks (satır başına bir blok), gerçek kartta 2²⁰.
func TestIndexedFirstFaultIsSmallestPosition(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	n := 1 << 20
	if inSim() {
		n = 1 << 12
	}
	w := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4}, 4, 1)
	ids := tensor.NewOn(gpuDevice, make([]float32, n), n)
	tensor.EmbeddingAt(w, ids) // ısınma
	g.BeginCapture()
	tensor.EmbeddingAt(w, ids)
	gr := g.EndCapture()
	defer gr.Release()
	rng := rand.New(rand.NewSource(11))
	for run := 0; run < 3; run++ {
		x := make([]float32, n)
		want := n
		for k := 0; k < 64; k++ {
			at := rng.Intn(n)
			x[at] = 4
			want = min(want, at)
		}
		ids.CopyFrom(tensor.NewOn(gpuDevice, x, n))
		gr.Replay()
		if _, f := tensor.Recording(gr).(tensor.FaultRecording).FirstFault(); f == nil || f.Index != want || f.Kind != tensor.FaultIndexRange {
			t.Fatalf("koşu %d: %+v; beklenen konum %d", run, f, want)
		}
	}
}

// Kural (15.45, 15.8; T1 dilim 2c): kayıtta dilim yazmanın tensör başlangıcı kırpılmaz: −1,
// Td−Ts+1 ve 1,5 bayrağa yazar (konum 0), hedef bit bit değişmez; int64 başlangıç da.
func TestUpdateSliceFaultLeavesTargetUntouched(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const Td, Ts = 6, 2
	for _, dt := range []tensor.DType{tensor.Float32, tensor.Int64} {
		dst := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4, 5, 6}, Td)
		src := tensor.NewOn(gpuDevice, []float32{9, 9}, Ts)
		start := tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{0})
		if dt == tensor.Float32 {
			start = tensor.NewOn(gpuDevice, []float32{0})
		}
		tensor.UpdateSliceAt(dst, src, start, 0)
		g.BeginCapture()
		tensor.UpdateSliceAt(dst, src, start, 0)
		gr := g.EndCapture()
		fr := tensor.Recording(gr).(tensor.FaultRecording)
		bad := []float64{-1, Td - Ts + 1}
		if dt == tensor.Float32 {
			bad = append(bad, 1.5)
		}
		for _, v := range bad {
			dst.CopyFrom(tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4, 5, 6}, Td))
			start.CopyFrom(indexTensor(dt, []float64{v}).Reshape())
			gr.Replay()
			_, f := fr.FirstFault()
			kind := tensor.FaultIndexRange
			if v == 1.5 {
				kind = tensor.FaultIndexNotInt
			}
			if f == nil || f.Kind != kind || f.Index != 0 {
				t.Errorf("%s başlangıç %v: %+v; beklenen çeşit %d", dt, v, f, kind)
			}
			checkDiff(t, "hatalı başlangıçta hedef", dst.Data(), []float32{1, 2, 3, 4, 5, 6}, 0)
		}
		start.CopyFrom(indexTensor(dt, []float64{Td - Ts}).Reshape())
		gr.Replay()
		if _, f := fr.FirstFault(); f != nil {
			t.Fatalf("%s: son geçerli başlangıç %+v", dt, f)
		}
		checkDiff(t, "son geçerli başlangıç", dst.Data(), []float32{1, 2, 3, 4, 9, 9}, 0)
		gr.Release()
	}
}
