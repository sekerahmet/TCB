package tensor

import (
	"math"
	"math/bits"
)

// ============================================================================
// CPU arka ucu: cinsli işlemler (TypedBackend; typed.go)
// ============================================================================
//
// Girdiler sonucun şekline yayınlanır; her girdinin öğe konumları bir kez çıkarılır, sonra
// öğeler tam sayıda int64, ondalıkta float64 olarak (kayıpsız) okunur. Sonuç hedef türe
// yazılır; tam sayıda sığmayan değer ilk konumuyla hata olur. Hız bu dilimin ölçütü değildir:
// yol sade ve doğru tutuldu (hızlı çekirdekler 2b'de).

// elems bir girdi görünümünün sonucun şekline yayınlanmış öğe okuyucusu.
type elems struct {
	c    *cpuStorage
	offs []int
}

func newElems(v View, outShape []int) elems {
	st := BroadcastStrides(v.Strides, v.Shape, outShape)
	offs := make([]int, NumelOf(outShape))
	idx := make([]int, len(outShape))
	for i := range offs {
		o := v.Offset
		for d, k := range idx {
			o += k * st[d]
		}
		offs[i] = o
		for d := len(idx) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < outShape[d] {
				break
			}
			idx[d] = 0
		}
	}
	return elems{c: cpuStore(v.S), offs: offs}
}

// i int64 değer (tam sayı ve mantık türlerinde).
func (e elems) i(k int) int64 {
	eb := elemBytes(e.c.dtyp)
	o := e.offs[k] * eb
	return RawInt(e.c.dtyp, e.c.raw[o:o+eb])
}

// f float64 değer (bütün ondalık türlerde, kayıpsız).
func (e elems) f(k int) float64 {
	o := e.offs[k]
	switch e.c.dtyp {
	case Float32:
		return float64(e.c.d[o])
	case BFloat16:
		return float64(bf16ToF32(e.c.h[o]))
	case Float16:
		return float64(f16ToF32(e.c.h[o]))
	case Float64:
		return RawFloat64(e.c.raw[o*8:])
	}
	return float64(e.i(k))
}

// isFloat girdi ondalık mı.
func (e elems) isFloat() bool { return e.c.dtyp.Elem() == ElemFloat }

// putInt sonucun k. öğesine v'yi yazar; sığmazsa yanlış döner.
func putInt(o *cpuStorage, k int, v int64) bool {
	if !o.dtyp.Fits(v) && o.dtyp != Bool {
		return false
	}
	eb := elemBytes(o.dtyp)
	PutRawInt(o.dtyp, o.raw[k*eb:(k+1)*eb], v)
	return true
}

// putFloat sonucun k. öğesine v'yi ondalık türde (en yakına yuvarlayarak) yazar.
func putFloat(o *cpuStorage, k int, v float64) {
	switch o.dtyp {
	case Float32:
		o.d[k] = float32(v)
	case BFloat16, Float16:
		o.h[k] = narrowOne(f64ToF32Odd(v), o.dtyp)
	case Float64:
		PutRawFloat64(o.raw[k*8:], v)
	}
}

// f64ToF32Odd float64'ü float32'ye tek sayıya yuvarlar (round-to-odd): sonraki 16 bitlik
// yuvarlama böylece çift yuvarlama yanlışı yapmaz.
func f64ToF32Odd(v float64) float32 {
	f := float32(v)
	if float64(f) == v || math.IsNaN(v) || math.IsInf(float64(f), 0) {
		return f
	}
	if math.Abs(float64(f)) > math.Abs(v) {
		f = math.Nextafter32(f, 0)
	}
	return math.Float32frombits(math.Float32bits(f) | 1)
}

// Typed cinsli işlemi yapar (TypedBackend).
func (cpuBackend) Typed(op TypedOp, out Storage, outShape []int, ins []View) (*ElemFault, bool) {
	o := cpuStore(out)
	n := NumelOf(outShape)
	es := make([]elems, len(ins))
	for i, v := range ins {
		es[i] = newElems(v, outShape)
	}
	switch op.Kind {
	case TCompare:
		for k := 0; k < n; k++ {
			putInt(o, k, b2i(compareAt(CmpOp(op.Sub), es[0], es[1], k)))
		}
	case TLogic:
		return logicAll(LogicOp(op.Sub), o, es[0], es[1], n), true
	case TNot:
		for k := 0; k < n; k++ {
			if o.dtyp == Bool {
				putInt(o, k, 1-es[0].i(k))
			} else {
				putInt(o, k, ^es[0].i(k))
			}
		}
	case TArith:
		return arithAll(ArithOp(op.Sub), o, es[0], es[1], n), true
	case TNeg, TAbs, TSign:
		return unaryAll(op.Kind, o, es[0], n), true
	case TConvert:
		return convertAll(o, es[0], n), true
	case TWhere:
		for k := 0; k < n; k++ {
			pick := es[2]
			if es[0].i(k) != 0 {
				pick = es[1]
			}
			copyElem(o, k, pick, k)
		}
	default:
		return nil, false
	}
	return nil, true
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// compareAt k. öğelerin karşılaştırması: ondalıkta IEEE 754 (NaN'lı karşılaştırma yanlış, !=
// doğru), tam sayıda ve mantıkta kesin.
func compareAt(op CmpOp, a, b elems, k int) bool {
	if a.isFloat() || b.isFloat() {
		x, y := a.f(k), b.f(k)
		switch op {
		case CmpEq:
			return x == y
		case CmpNe:
			return x != y
		case CmpLt:
			return x < y
		case CmpGt:
			return x > y
		case CmpLe:
			return x <= y
		}
		return x >= y
	}
	x, y := a.i(k), b.i(k)
	switch op {
	case CmpEq:
		return x == y
	case CmpNe:
		return x != y
	case CmpLt:
		return x < y
	case CmpGt:
		return x > y
	case CmpLe:
		return x <= y
	}
	return x >= y
}

// copyElem kaynağın k. öğesini sonucun j. öğesine aynı türde yazar.
func copyElem(o *cpuStorage, j int, e elems, k int) {
	off := e.offs[k]
	switch o.dtyp {
	case Float32:
		o.d[j] = e.c.d[off]
	case BFloat16, Float16:
		o.h[j] = e.c.h[off]
	default:
		eb := elemBytes(o.dtyp)
		copy(o.raw[j*eb:(j+1)*eb], e.c.raw[off*eb:(off+1)*eb])
	}
}

// logicAll mantık ve bit işleçleri; kaydırmada miktar 0 ile sol işlenenin bit genişliği eksi
// 1 arasında olmalı, dışarı taşan bitler atılır (4.12).
func logicAll(op LogicOp, o *cpuStorage, a, b elems, n int) *ElemFault {
	bits := uint(o.dtyp.Bits())
	for k := 0; k < n; k++ {
		x, y := a.i(k), b.i(k)
		var r int64
		switch op {
		case LogicAnd:
			r = x & y
		case LogicOr:
			r = x | y
		case LogicXor:
			r = x ^ y
		case LogicShl, LogicShr:
			if y < 0 || y >= int64(bits) {
				return &ElemFault{Kind: FaultShift, Index: k, Int: y, DType: o.dtyp}
			}
			if op == LogicShr {
				r = x >> uint(y)
			} else {
				r = wrapBits(x<<uint(y), o.dtyp)
			}
		}
		putInt(o, k, r)
	}
	return nil
}

// wrapBits v'yi dt'nin bit genişliğine keser (işaretlide işaret genişletilir).
func wrapBits(v int64, dt DType) int64 {
	bits := uint(dt.Bits())
	if bits >= 64 {
		return v
	}
	v &= 1<<bits - 1
	if !dt.IsUnsigned() && v>>(bits-1) == 1 {
		v -= 1 << bits
	}
	return v
}

// arithAll Int ve float64 aritmetiği; Int'te taşma, sıfıra bölme ve eksi üs ilk konumla hata.
func arithAll(op ArithOp, o *cpuStorage, a, b elems, n int) *ElemFault {
	if o.dtyp.Elem() == ElemFloat {
		for k := 0; k < n; k++ {
			putFloat(o, k, floatArith(op, a.f(k), b.f(k)))
		}
		return nil
	}
	for k := 0; k < n; k++ {
		x, y := a.i(k), b.i(k)
		r, kind, ok := intArith(op, x, y)
		if ok && !o.dtyp.Fits(r) {
			kind, ok = FaultOverflow, false
		}
		if !ok {
			return &ElemFault{Kind: kind, Index: k, A: x, B: y, DType: o.dtyp}
		}
		putInt(o, k, r)
	}
	return nil
}

// floatArith float64 aritmetiği (IEEE 754; // aşağı yuvarlar, % bölenin işaretini alır, 4.2a).
func floatArith(op ArithOp, x, y float64) float64 {
	switch op {
	case ArithAdd:
		return x + y
	case ArithSub:
		return x - y
	case ArithMul:
		return x * y
	case ArithDiv:
		return x / y
	case ArithFloorDiv:
		return math.Floor(x / y)
	case ArithMod:
		r := math.Mod(x, y)
		if r != 0 && (r < 0) != (y < 0) {
			r += y
		}
		return r
	}
	return math.Pow(x, y)
}

// intArith int64 aritmetiği; ok yanlışsa kind hatanın çeşididir (int64 taşması dahil).
func intArith(op ArithOp, x, y int64) (int64, FaultKind, bool) {
	switch op {
	case ArithAdd:
		r := x + y
		return r, FaultOverflow, (x >= 0) != (y >= 0) || (r >= 0) == (x >= 0)
	case ArithSub:
		r := x - y
		return r, FaultOverflow, (x >= 0) == (y >= 0) || (r >= 0) == (x >= 0)
	case ArithMul:
		return mulInt(x, y)
	case ArithFloorDiv, ArithMod:
		if y == 0 {
			return 0, FaultDivZero, false
		}
		if x == math.MinInt64 && y == -1 {
			return 0, FaultOverflow, op == ArithMod
		}
		q, r := x/y, x%y
		if r != 0 && (r < 0) != (y < 0) {
			q, r = q-1, r+y
		}
		if op == ArithMod {
			return r, 0, true
		}
		return q, 0, true
	}
	return powInt(x, y)
}

func mulInt(x, y int64) (int64, FaultKind, bool) {
	if x == 0 || y == 0 {
		return 0, 0, true
	}
	r := x * y
	if r/y != x || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64) {
		return 0, FaultOverflow, false
	}
	return r, 0, true
}

// powInt tam sayı üssü (üs eksi olamaz, 4.3).
func powInt(x, y int64) (int64, FaultKind, bool) {
	if y < 0 {
		return 0, FaultNegExp, false
	}
	r := int64(1)
	for ; y > 0; y-- {
		var ok bool
		if r, _, ok = mulInt(r, x); !ok {
			return 0, FaultOverflow, false
		}
		if r == 0 || r == 1 && x == 1 {
			return r, 0, true
		}
	}
	return r, 0, true
}

// unaryAll tekli eksi, mutlak değer ve işaret (Int ve float64).
func unaryAll(kind TypedKind, o *cpuStorage, a elems, n int) *ElemFault {
	for k := 0; k < n; k++ {
		if o.dtyp.Elem() == ElemFloat {
			x := a.f(k)
			switch kind {
			case TNeg:
				x = -x
			case TAbs:
				x = math.Abs(x)
			default:
				if x > 0 {
					x = 1
				} else if x < 0 {
					x = -1
				}
			}
			putFloat(o, k, x)
			continue
		}
		x := a.i(k)
		r := x
		switch kind {
		case TNeg:
			r = -x
		case TAbs:
			if x < 0 {
				r = -x
			}
		default:
			r = max(min(x, 1), -1)
		}
		overflow := kind == TNeg && x != 0 && (r < 0) == (x < 0) || kind == TAbs && r < 0
		if overflow || !putInt(o, k, r) {
			return &ElemFault{Kind: FaultOverflow, Index: k, A: x, DType: o.dtyp}
		}
	}
	return nil
}

// convertAll cins ve hassasiyet dönüşümü (15.3).
func convertAll(o *cpuStorage, a elems, n int) *ElemFault {
	for k := 0; k < n; k++ {
		switch {
		case o.dtyp.Elem() == ElemFloat && a.isFloat():
			putFloat(o, k, a.f(k))
		case o.dtyp.Elem() == ElemFloat:
			putFloat(o, k, intToFloat(a.i(k), o.dtyp))
		case a.isFloat():
			x := a.f(k)
			switch {
			case math.IsNaN(x):
				return &ElemFault{Kind: FaultNaN, Index: k, Float: x, DType: o.dtyp}
			case math.IsInf(x, 0):
				return &ElemFault{Kind: FaultInf, Index: k, Float: x, DType: o.dtyp}
			}
			t := math.Trunc(x)
			if t < -(1<<63) || t >= 1<<63 || !putInt(o, k, int64(t)) {
				return &ElemFault{Kind: FaultNotFit, Index: k, Float: x, DType: o.dtyp}
			}
		default:
			x := a.i(k)
			if !putInt(o, k, x) {
				return &ElemFault{Kind: FaultNotFit, Index: k, Int: x, Float: float64(x), DType: o.dtyp}
			}
		}
	}
	return nil
}

// intToFloat tam sayıyı dt ondalığına en yakına (eşitlikte çifte) yuvarlar; 16 bitlik ve float32
// hedefte değer önce hedefin anlamlı bit sayısına tam sayı olarak yuvarlanır (çift yuvarlama
// yok), sonra kayıpsız çevrilir.
func intToFloat(v int64, dt DType) float64 {
	var mant uint
	switch dt {
	case Float32:
		mant = 24
	case BFloat16:
		mant = 8
	case Float16:
		mant = 11
	default:
		return float64(v)
	}
	neg := v < 0
	u := uint64(v)
	if neg {
		u = -u
	}
	if w := uint(64 - bits.LeadingZeros64(u)); w > mant {
		drop := w - mant
		half, mask := uint64(1)<<(drop-1), uint64(1)<<drop-1
		rest := u & mask
		u &^= mask
		if rest > half || rest == half && u>>drop&1 == 1 {
			u += 1 << drop
		}
	}
	f := float64(u) // en çok 24 anlamlı bit: kayıpsız
	if neg {
		f = -f
	}
	return f
}
