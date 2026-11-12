package gpu

import (
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Cinsli dönüşüm (15.3): tam sayıya ve tam sayıdan float32'ye, kartta
// ============================================================================
//
// İşlemcinin convertAll'u (tensor/cpu_typed_ops.go) ile aynı anlam: ondalıktan tam sayıya
// sıfıra doğru keser; NaN, sonsuz ve hedefe sığmayan değer hatadır (bu sırayla); tam sayıdan
// tam sayıya değer sığmalıdır; tam sayıdan float32'ye en yakına (eşitlikte çifte) tek
// yuvarlama. Hatalı öğe yazılmaz; anahtarı (düz konum << 4 | çeşit) bayrağın yuvasına tek
// atom.global.min.u64 ile yazılır (fault_slot.go): en küçük düz konum kazanır (15.6).

// convertOK dönüşümün kartta olup olmadığı: tam sayı hedefine ondalıktan (hesap ondalıkları ve
// float64) ya da tam sayıdan; float32 hedefine tam sayıdan ve mantıktan.
func convertOK(out, in tensor.DType) bool {
	switch {
	case out == tensor.Float32:
		return in.Elem() == tensor.ElemInt || in == tensor.Bool
	case out.Elem() == tensor.ElemInt:
		return in.IsFloat() || in == tensor.Float64 || in.Elem() == tensor.ElemInt
	}
	return false
}

// typedFaults işlemin öğeye bağlı hatası olabilir mi (bayrağın yuvasını ve beklemeyi ister):
// "int64" aritmetiği (typed_arith.go), ondalıktan tam sayıya ve daraltan tam sayı dönüşümü.
// Genişleten dönüşüm (MergeInt'in hassasiyetine, 15.6c) ve değerleri tam sayı olduğu bilinen
// kaynağın dönüşümü (tensor.ConvertIntegral: sıralamanın indeksi, T1 dilim 2b, A15) hata
// veremez: yuvası ve beklemesi yoktur.
func typedFaults(op tensor.TypedOp, out tensor.DType, ins []tensor.DType) bool {
	switch op.Kind {
	case tensor.TArith, tensor.TNeg, tensor.TAbs:
		return true
	}
	if op.Kind != tensor.TConvert || out.Elem() != tensor.ElemInt || op.Sub == tensor.ConvertIntegral {
		return false
	}
	in := ins[0]
	if in.Elem() != tensor.ElemInt {
		return true
	}
	lo, hi := intBounds(out)
	ilo, ihi := intBounds(in)
	return ilo < lo || ihi > hi
}

// convertElem bir öğeyi in'den out'a çevirir; tam sayı hedefinde hata bayrağa (flag) gider.
func (k *emitter) convertElem(in, out tensor.DType, from, to, i, flag reg) {
	if out == tensor.Float32 {
		k.store(tensor.Float32, to, k.intToF32(in, from))
		return
	}
	v, bad, kind := k.toInt64(in, out, from)
	if flag == "" { // genişleten dönüşüm: hata yok
		k.storeInt(out, to, v)
		return
	}
	ok, done := k.label(), k.label()
	k.jumpUnless(bad, ok)
	k.writeFault(flag, i, kind)
	k.jump(done)
	k.put(ok)
	k.storeInt(out, to, v)
	k.put(done)
}

// storeInt int64 v'yi out hassasiyetinde yazar (alt baytları; değerin sığdığı denetlendi).
func (k *emitter) storeInt(out tensor.DType, to, v reg) {
	if dtypeSize(out) == 8 {
		k.e("st.global.u64", mem(to, 0), v)
		return
	}
	r := k.r()
	k.e("cvt.u32.u64", r, v)
	k.e("st.global."+rawSuffix(out), mem(to, 0), r)
}

// intToF32 tam sayı ya da mantık öğesini float32'ye en yakına yuvarlar (tek yuvarlama).
func (k *emitter) intToF32(in tensor.DType, from reg) reg {
	v := k.f()
	switch {
	case in == tensor.Int64:
		k.e("cvt.rn.f32.s64", v, k.rawLoad(in, from))
	case in == tensor.Bool || in.IsUnsigned():
		k.e("cvt.rn.f32.u32", v, k.int32Of(in, from))
	default:
		k.e("cvt.rn.f32.s32", v, k.int32Of(in, from))
	}
	return v
}

// int32Of en çok 32 bitlik tam sayı öğesini 32 bitlik yazmaca yükler: işaretli dar türde işaret
// genişletilir, işaretsizde sıfırla doldurulur.
func (k *emitter) int32Of(in tensor.DType, from reg) reg {
	raw := k.rawLoad(in, from)
	r := raw
	size := dtypeSize(in)
	if size == 2 {
		r = k.r()
		k.e("cvt.u32.u16", r, raw)
	}
	if size < 4 && !in.IsUnsigned() && in != tensor.Bool {
		sh := 32 - 8*size
		k.e("shl.b32", r, r, sh)
		k.e("shr.s32", r, r, sh)
	}
	return r
}

// toInt64 öğeyi int64'e çevirir ve out'a sığmazlığını söyler: v değer (hatasızsa), bad hata
// yüklemi, kind hatanın çeşidi (tensor.FaultKind).
func (k *emitter) toInt64(in, out tensor.DType, from reg) (v, bad, kind reg) {
	if in.Elem() == tensor.ElemInt {
		return k.intToInt64(in, out, from)
	}
	x := k.fd()
	if in == tensor.Float64 {
		k.e("mov.b64", x, k.rawLoad(in, from))
	} else {
		k.e("cvt.f64.f32", x, k.load(in, from))
	}
	lo, hi := floatBounds(out)
	nan, pinf, ninf, low, high := k.p(), k.p(), k.p(), k.p(), k.p()
	k.e("setp.nan.f64", nan, x, x)
	k.e("setp.eq.f64", pinf, x, math.Inf(1))
	k.e("setp.eq.f64", ninf, x, math.Inf(-1))
	t := k.fd()
	k.e("cvt.rzi.f64.f64", t, x)
	k.e("setp.lt.f64", low, t, lo)
	k.e("setp.ge.f64", high, t, hi)
	inf, bad := k.p(), k.p()
	k.e("or.pred", inf, pinf, ninf)
	k.e("or.pred", bad, low, high)
	k.e("or.pred", bad, bad, inf)
	k.e("or.pred", bad, bad, nan)
	kind = k.r()
	k.e("selp.u32", kind, int(tensor.FaultInf), int(tensor.FaultNotFit), inf)
	k.e("selp.u32", kind, int(tensor.FaultNaN), kind, nan)
	v = k.rd()
	k.e("cvt.rzi.s64.f64", v, x)
	return v, bad, kind
}

// intToInt64 tam sayı öğesini int64'e genişletir; out'un aralığı dışı FaultNotFit.
func (k *emitter) intToInt64(in, out tensor.DType, from reg) (v, bad, kind reg) {
	v = k.rd()
	switch {
	case dtypeSize(in) == 8:
		k.e("mov.b64", v, k.rawLoad(in, from))
	case in.IsUnsigned():
		k.e("cvt.u64.u32", v, k.int32Of(in, from))
	default:
		k.e("cvt.s64.s32", v, k.int32Of(in, from))
	}
	lo, hi := intBounds(out)
	low, high, bad := k.p(), k.p(), k.p()
	k.e("setp.lt.s64", low, v, lo)
	k.e("setp.gt.s64", high, v, hi)
	k.e("or.pred", bad, low, high)
	kind = k.r()
	k.e("mov.u32", kind, int(tensor.FaultNotFit))
	return v, bad, kind
}

// intBounds tam sayı hassasiyetinin en küçük ve en büyük değeri.
func intBounds(dt tensor.DType) (int64, int64) {
	bits := dt.Bits()
	if dt.IsUnsigned() {
		return 0, int64(1)<<bits - 1
	}
	if bits == 64 {
		return math.MinInt64, math.MaxInt64
	}
	return -(int64(1) << (bits - 1)), int64(1)<<(bits-1) - 1
}

// floatBounds kesilmiş değerin hassasiyete sığma aralığı [lo, hi): ikinin kuvvetleri, float64'te
// tam.
func floatBounds(dt tensor.DType) (float64, float64) {
	bits := dt.Bits()
	if dt.IsUnsigned() {
		return 0, math.Ldexp(1, bits)
	}
	return -math.Ldexp(1, bits-1), math.Ldexp(1, bits-1)
}
