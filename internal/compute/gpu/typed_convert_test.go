package gpu

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

var convIntTypes = []tensor.DType{tensor.Int8, tensor.Int16, tensor.Int32, tensor.Int64, tensor.UInt8, tensor.UInt16, tensor.UInt32}

// convertBoth aynı girdiyi (src işlemcide kurulur, karta kopyalanır) iki cihazda dt'ye çevirir.
func convertBoth(t *testing.T, src *tensor.Tensor, dt tensor.DType) (cv, gv []int64, cf, gf *tensor.ElemFault) {
	t.Helper()
	g := src.To(gpuDevice)
	defer g.Drop()
	cr, cf := tensor.Convert(src, dt)
	gr, gf := tensor.Convert(g, dt)
	if cr != nil {
		cv = cr.Int64s()
	}
	if gr != nil {
		if gr.Device() != gpuDevice {
			t.Fatalf("%s → %s: sonuç %v'de, kartta olmalı", src.DType(), dt, gr.Device())
		}
		gv = gr.Int64s()
	}
	return cv, gv, cf, gf
}

// sameFault iki hatanın çeşidi, konumu ve değerleri aynı mı.
func sameFault(a, b *tensor.ElemFault) bool {
	if a == nil || b == nil {
		return a == b
	}
	nan := math.IsNaN(a.Float) && math.IsNaN(b.Float)
	return a.Kind == b.Kind && a.Index == b.Index && a.Int == b.Int && (a.Float == b.Float || nan) &&
		a.A == b.A && a.B == b.B && a.DType == b.DType && slices.Equal(a.Shape, b.Shape)
}

// floatSource işlemcide tek öğeli dt ondalık tensörü (float64'ten; hesap ondalıklarında
// yuvarlanarak).
func floatSource(dt tensor.DType, v float64) *tensor.Tensor {
	if dt == tensor.Float64 {
		return tensor.Float64sOn(tensor.CPU, []float64{v}, 1)
	}
	f := tensor.NewOn(tensor.CPU, []float32{float32(v)}, 1)
	if dt == tensor.Float32 {
		return f
	}
	return f.Cast(dt)
}

// Kural (15.3, 15.6, 15.41; T1 dilim 2a): kartta ondalıktan tam sayıya dönüşüm işlemcininkiyle
// aynı değeri ve aynı hatayı verir: sıfıra doğru keser; NaN, sonsuz ve sığmayan değer hatadır
// (bu sırayla); sınırların iki yanı, ±0 ve −0,5'in işaretsize dönüşümü dahil.
func TestConvertFloatToIntMatchesCPU(t *testing.T) {
	base := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 1.9, -1.9, 100.75, math.NaN(), math.Inf(1), math.Inf(-1),
		math.Ldexp(1, 63), -math.Ldexp(1, 63), math.Ldexp(1, 63) - 1024, math.Ldexp(1, 64), 1e30}
	for _, out := range convIntTypes {
		lo, hi := intBounds(out)
		vals := append(slices.Clone(base), float64(lo), float64(lo)-1, float64(lo)-0.5, float64(hi), float64(hi)+1, float64(hi)+0.5)
		for _, in := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16, tensor.Float64} {
			for _, v := range vals {
				src := floatSource(in, v)
				cv, gv, cf, gf := convertBoth(t, src, out)
				if !slices.Equal(cv, gv) || !sameFault(cf, gf) {
					t.Errorf("%s %v → %s: kart %v %+v, işlemci %v %+v", in, v, out, gv, gf, cv, cf)
				}
			}
		}
	}
}

// Kural (15.3, 15.41; T1 dilim 2a): kartta tam sayıdan tam sayıya dönüşüm işlemcininkiyle aynı:
// değer sığmalıdır, sığmayan FaultNotFit; genişleten dönüşüm hata veremez.
func TestConvertIntToIntMatchesCPU(t *testing.T) {
	for _, in := range convIntTypes {
		ilo, ihi := intBounds(in)
		for _, out := range convIntTypes {
			if in == out {
				continue
			}
			lo, hi := intBounds(out)
			for _, v := range []int64{ilo, ihi, -1, 0, 1, lo, lo - 1, hi, hi + 1} {
				if v < ilo || v > ihi {
					continue
				}
				src := tensor.Int64sOn(tensor.CPU, in, []int64{v}, 1)
				cv, gv, cf, gf := convertBoth(t, src, out)
				if !slices.Equal(cv, gv) || !sameFault(cf, gf) {
					t.Errorf("%s %d → %s: kart %v %+v, işlemci %v %+v", in, v, out, gv, gf, cv, cf)
				}
			}
		}
	}
}

// Kural (15.3; T1 dilim 2a): kartta tam sayıdan float32'ye dönüşüm işlemcininki gibi en yakına
// (eşitlikte çifte) tek yuvarlamadır: 2²⁴'ün üstündeki eşitlikler ve int64'te float64'ten geçen
// çift yuvarlamanın ayrıldığı değer dahil.
func TestConvertIntToFloat32MatchesCPU(t *testing.T) {
	vals := map[tensor.DType][]int64{
		tensor.Int8:   {-128, -1, 0, 127},
		tensor.Int16:  {-32768, -1, 0, 32767},
		tensor.Int32:  {math.MinInt32, -16777217, 16777217, 16777219, 2147483583, math.MaxInt32},
		tensor.Int64:  {math.MinInt64, math.MaxInt64, 1<<60 + 1<<36 + 1, -(1<<60 + 1<<36 + 1), 1<<53 + 1, 16777217},
		tensor.UInt8:  {0, 255},
		tensor.UInt16: {0, 65535},
		tensor.UInt32: {0, 16777217, 4294967295, 4294967167},
		tensor.Bool:   {0, 1},
	}
	for in, vs := range vals {
		src := tensor.Int64sOn(tensor.CPU, in, vs, len(vs))
		g := src.To(gpuDevice)
		c, _ := tensor.Convert(src, tensor.Float32)
		r, f := tensor.Convert(g, tensor.Float32)
		if f != nil {
			t.Fatalf("%s → float32: hata %+v", in, f)
		}
		cb, gb := c.Data(), r.Data()
		for i := range cb {
			if math.Float32bits(cb[i]) != math.Float32bits(gb[i]) {
				t.Errorf("%s %d → float32: kart %v, işlemci %v", in, vs[i], gb[i], cb[i])
			}
		}
	}
}

// Kural (15.6, T1 dilim 2a): birden çok öğe hatalıysa ileti ilk hatalı öğeyi söyler: düz (satır
// öncelikli) konumu en küçük hatalı çıktı öğesi, iki cihazda aynı. Daha küçük konumdaki NaN
// büyükteki sığmamadan önce gelir; büyük tensörde rastgele konumlardaki hatalarda her koşuda en
// küçük konum (kartta atomların sırası sonucu değiştirmez); devrik görünümde konum çıktının
// şeklindedir.
func TestConvertFirstFaultIsSmallestFlatIndex(t *testing.T) {
	small := []float32{1, 2, 3e10, 4, float32(math.NaN()), 6}
	small[1] = float32(math.NaN())
	cv, gv, cf, gf := convertBoth(t, tensor.NewOn(tensor.CPU, small, 2, 3), tensor.Int32)
	if cv != nil || gv != nil || !sameFault(cf, gf) || gf.Kind != tensor.FaultNaN || gf.Index != 1 {
		t.Fatalf("küçük: kart %+v, işlemci %+v; beklenen NaN, konum 1", gf, cf)
	}
	n := 1 << 20
	rng := rand.New(rand.NewSource(7))
	for run := 0; run < 3; run++ {
		x := make([]float32, n)
		want := n
		for k := 0; k < 64; k++ {
			at := rng.Intn(n)
			x[at] = 1e20
			want = min(want, at)
		}
		_, _, cf, gf := convertBoth(t, tensor.NewOn(tensor.CPU, x, n), tensor.Int16)
		if !sameFault(cf, gf) || gf.Index != want || gf.Kind != tensor.FaultNotFit {
			t.Fatalf("koşu %d: kart %+v, işlemci %+v; beklenen konum %d", run, gf, cf, want)
		}
	}
	m := tensor.NewOn(tensor.CPU, []float32{1, 2, 3, 4, 5, float32(math.Inf(1))}, 2, 3).Transpose(0, 1)
	_, _, cf, gf = convertBoth(t, m, tensor.Int64)
	if !sameFault(cf, gf) || gf.Index != 5 || gf.Kind != tensor.FaultInf {
		t.Fatalf("devrik: kart %+v, işlemci %+v; beklenen sonsuz, konum 5", gf, cf)
	}
}

// Kural (15.3, 15.6; DENETİM 490 N3): boş ve 0 boyutlu tensörün kartta dönüşümü işlemcininkiyle
// aynıdır: boşta hata yok, sonuç boş ve aynı şekilde; 0 boyutluda değer de hata da aynı (konum 0,
// şekil boş).
func TestConvertEmptyAndScalarTensors(t *testing.T) {
	for _, shape := range [][]int{{0}, {2, 0}} {
		for _, c := range []struct {
			src *tensor.Tensor
			out tensor.DType
		}{
			{tensor.NewOn(tensor.CPU, []float32{}, shape...), tensor.Int32},
			{tensor.Int64sOn(tensor.CPU, tensor.Int64, []int64{}, shape...), tensor.Int8},
			{tensor.Int64sOn(tensor.CPU, tensor.Int16, []int64{}, shape...), tensor.Float32},
		} {
			g := c.src.To(gpuDevice)
			r, f := tensor.Convert(g, c.out)
			if f != nil || r == nil || r.Numel() != 0 || !slices.Equal(r.Shape(), shape) || r.DType() != c.out {
				t.Errorf("%v %s → %s: sonuç %v, hata %+v", shape, c.src.DType(), c.out, r, f)
			}
			g.Drop()
		}
	}
	for _, v := range []float32{2.5, -7.9, float32(math.NaN()), 1e10} {
		src := tensor.NewOn(tensor.CPU, []float32{v})
		cv, gv, cf, gf := convertBoth(t, src, tensor.Int32)
		if !slices.Equal(cv, gv) || !sameFault(cf, gf) {
			t.Errorf("0 boyutlu %v → int32: kart %v %+v, işlemci %v %+v", v, gv, gf, cv, cf)
		}
		if gf != nil && (gf.Index != 0 || len(gf.Shape) != 0) {
			t.Errorf("0 boyutlu %v: hatanın yeri %d %v; konum 0, boş şekil olmalı", v, gf.Index, gf.Shape)
		}
	}
	c, _ := tensor.Convert(tensor.Int64sOn(tensor.CPU, tensor.Int32, []int64{-3}), tensor.Float32)
	r, f := tensor.Convert(tensor.Int64sOn(gpuDevice, tensor.Int32, []int64{-3}), tensor.Float32)
	if f != nil || r.Dim() != 0 || r.Item() != c.Item() {
		t.Errorf("0 boyutlu int32 → float32: kart %v %+v, işlemci %v", r, f, c)
	}
}

// Kural (15.6; T1 dilim 2a): genişleten tam sayı dönüşümü (MergeInt'in hassasiyetine) hata
// veremez; kartta bayrağın yuvasını almaz, beklemez.
func TestWideningConversionHasNoFaultSlot(t *testing.T) {
	for _, c := range [][2]tensor.DType{{tensor.Int8, tensor.Int16}, {tensor.UInt8, tensor.Int16}, {tensor.UInt32, tensor.Int64}, {tensor.Int32, tensor.Int64}} {
		if typedFaults(tensor.TypedOp{Kind: tensor.TConvert}, c[1], c[:1]) {
			t.Errorf("%s → %s genişletir, yuva almamalı", c[0], c[1])
		}
	}
	for _, c := range [][2]tensor.DType{{tensor.Int16, tensor.Int8}, {tensor.Int8, tensor.UInt8}, {tensor.UInt32, tensor.Int32}, {tensor.Float32, tensor.Int64}} {
		if !typedFaults(tensor.TypedOp{Kind: tensor.TConvert}, c[1], c[:1]) {
			t.Errorf("%s → %s daraltır, yuva almalı", c[0], c[1])
		}
	}
}
