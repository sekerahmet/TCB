package tensor

import (
	"math"
	mbig "math/big"
	"math/bits"
)

// ============================================================================
// Cinsli indirgemeler (15.18, 15.20): TR_Int, TR_Bool ve float64, CPU'da
// ============================================================================
//
// Tam sayı ve mantık tensörünün biriktirmesi "int64"te yapılır ve "int64" verir; taşma ilk
// konumla hatadır (15.3). float64 float64'te biriktirilir. Bu dilimde yalnız CPU'dadır; GPU'da
// hassasiyet hatası (15.48, genel GPU çekirdekleri 2b'de).

// ReduceKind cinsli indirgemenin çeşidi.
type ReduceKind uint8

const (
	ReduceSum ReduceKind = iota
	ReduceMax
	ReduceMin
	ReduceCumSum
)

// cpuOnly cinsli konak yolunun yalnız CPU'da çalıştığını denetler.
func cpuOnly(t *Tensor) {
	if t.Device() != CPU {
		panic(PrecisionError(t.dtyp, t.Device()))
	}
}

// axisRuns bitişik değerlerde axis ekseni boyunca her satırın (dış, iç) başlangıcını ve adımını
// verir: satır r'nin k. öğesi base(r) + k·step'tedir.
func axisRuns(shape []int, axis int) (outer, length, inner int) {
	outer, inner = 1, 1
	for i, n := range shape {
		if i < axis {
			outer *= n
		} else if i > axis {
			inner *= n
		}
	}
	return outer, shape[axis], inner
}

// ReduceTyped cinsli tensörü axis ekseni boyunca indirger (axis < 0: bütün öğeler, 0 boyutlu
// sonuç). Toplamda sonuç int64 (tam sayı ve mantık) ya da float64'tür; en büyük ve en küçükte
// girdinin türü; birikimli toplamda şekil korunur. Boş eksende en büyük ve en küçüğü çağıran
// denetler.
func ReduceTyped(t *Tensor, kind ReduceKind, axis int) (*Tensor, *ElemFault) {
	cpuOnly(t)
	shape := copyInts(t.shape)
	if axis < 0 {
		shape, axis = []int{t.Numel()}, 0
	}
	outer, n, inner := axisRuns(shape, axis)
	outShape := append(copyInts(shape[:axis]), shape[axis+1:]...)
	if kind == ReduceCumSum {
		outShape = shape
	}
	if t.Elem() == ElemFloat {
		return reduceFloat64(t.Float64s(), kind, outer, n, inner, outShape, t.dtyp), nil
	}
	return reduceInt64(t.Int64s(), kind, outer, n, inner, outShape, t.dtyp)
}

func reduceInt64(vals []int64, kind ReduceKind, outer, n, inner int, outShape []int, dt DType) (*Tensor, *ElemFault) {
	out := make([]int64, NumelOf(outShape))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			if kind == ReduceSum {
				// Matematiksel toplam 128 bitte kesin; yalnız sonuç int64'e sığmalı (15.18).
				var w wide
				for k := 0; k < n; k++ {
					w.add(vals[(o*n+k)*inner+i])
				}
				r, ok := w.int64()
				if !ok {
					return nil, &ElemFault{Kind: FaultOverflow, Index: o*inner + i, A: w.sat(), Sum: w.text(), DType: Int64}
				}
				out[o*inner+i] = r
				continue
			}
			var acc int64
			for k := 0; k < n; k++ {
				v := vals[(o*n+k)*inner+i]
				switch {
				case k == 0 && kind != ReduceCumSum:
					acc = v
				case kind == ReduceMax:
					acc = max(acc, v)
				case kind == ReduceMin:
					acc = min(acc, v)
				default:
					r, _, ok := intArith(ArithAdd, acc, v)
					if !ok {
						return nil, &ElemFault{Kind: FaultOverflow, Index: (o*n+k)*inner + i, A: acc, B: v, DType: Int64}
					}
					acc = r
				}
				if kind == ReduceCumSum {
					out[(o*n+k)*inner+i] = acc
				}
			}
			if kind != ReduceCumSum {
				out[o*inner+i] = acc
			}
		}
	}
	rdt := Int64
	if kind == ReduceMax || kind == ReduceMin {
		rdt = dt
	}
	return Int64sOn(CPU, rdt, out, outShape...), nil
}

func reduceFloat64(vals []float64, kind ReduceKind, outer, n, inner int, outShape []int, dt DType) *Tensor {
	out := make([]float64, NumelOf(outShape))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			acc := 0.0
			for k := 0; k < n; k++ {
				v := vals[(o*n+k)*inner+i]
				switch {
				case k == 0 && kind != ReduceSum && kind != ReduceCumSum:
					acc = v
				case kind == ReduceMax && (v > acc || math.IsNaN(v)) && !math.IsNaN(acc):
					acc = v
				case kind == ReduceMin && (v < acc || math.IsNaN(v)) && !math.IsNaN(acc):
					acc = v
				case kind == ReduceSum || kind == ReduceCumSum:
					acc += v
				}
				if kind == ReduceCumSum {
					out[(o*n+k)*inner+i] = acc
				}
			}
			if kind != ReduceCumSum {
				out[o*inner+i] = acc
			}
		}
	}
	return Float64sOn(CPU, out, outShape...)
}

// ArgMaxTyped cinsli tensörde axis boyunca en büyük öğenin indeksleri (ilk en büyük; float64'te
// ilk NaN, 15.19).
func ArgMaxTyped(t *Tensor, axis int) []int {
	cpuOnly(t)
	outer, n, inner := axisRuns(t.shape, axis)
	out := make([]int, outer*inner)
	var ints []int64
	var floats []float64
	if t.Elem() == ElemFloat {
		floats = t.Float64s()
	} else {
		ints = t.Int64s()
	}
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			best := 0
			for k := 1; k < n; k++ {
				p, q := (o*n+k)*inner+i, (o*n+best)*inner+i
				if floats != nil {
					if !math.IsNaN(floats[q]) && (floats[p] > floats[q] || math.IsNaN(floats[p])) {
						best = k
					}
				} else if ints[p] > ints[q] {
					best = k
				}
			}
			out[o*inner+i] = best
		}
	}
	return out
}

// wide 128 bitlik işaretli tam sayı biriktirici (int64 toplamının kesin ara değeri).
type wide struct {
	hi int64
	lo uint64
}

func (w *wide) add(v int64) {
	lo, carry := bits.Add64(w.lo, uint64(v), 0)
	w.lo = lo
	w.hi += int64(carry)
	if v < 0 {
		w.hi--
	}
}

// int64 değer int64'e sığıyorsa onu verir.
func (w wide) int64() (int64, bool) {
	r := int64(w.lo)
	return r, w.hi == r>>63
}

// text değerin onluk yazımı (sığmayan toplamın iletisi için).
func (w wide) text() string {
	return w.big().String()
}

func (w wide) big() *mbig.Int {
	sum := new(mbig.Int).Lsh(mbig.NewInt(w.hi), 64)
	return sum.Add(sum, new(mbig.Int).SetUint64(w.lo))
}

// sat sığmayan değerin iletideki yaklaşığı (int64'ün ucu).
func (w wide) sat() int64 {
	if w.hi < 0 {
		return math.MinInt64
	}
	return math.MaxInt64
}

// MeanIntTyped TR_Int ve TR_Bool tensörün ortalaması: "int64"te kesin toplam, sonuç "float32"ye
// bir kez yuvarlanır (15.18). axis < 0 bütün öğeler.
func MeanIntTyped(t *Tensor, axis int) *Tensor {
	cpuOnly(t)
	shape := copyInts(t.shape)
	if axis < 0 {
		shape, axis = []int{t.Numel()}, 0
	}
	outer, n, inner := axisRuns(shape, axis)
	outShape := append(copyInts(shape[:axis]), shape[axis+1:]...)
	vals := t.Int64s()
	out := make([]float32, NumelOf(outShape))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			var w wide
			for k := 0; k < n; k++ {
				w.add(vals[(o*n+k)*inner+i])
			}
			f, _ := new(mbig.Rat).SetFrac(w.big(), mbig.NewInt(int64(n))).Float32()
			out[o*inner+i] = f
		}
	}
	return NewOn(CPU, out, outShape...)
}
