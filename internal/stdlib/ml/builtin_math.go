package ml

import (
	"fmt"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/detmath"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Genişletilmiş matematik yerleşikleri: etkinleştirme (silu), ters trigonometri,
// hiperbolik, kararlı logaritma/üstel (birli ile: sayıda Ondalık, tensörde türevli
// tensör), taban/tavan/işaret/mutlak (sayı yolu internal/lang/interp'ta; tensörde tam sayılı
// işaret korunur), sınırla ve atan2. Koşullu seçim yerlesik_secim.go'da, birikim ve sıralama
// yerlesik_siralama.go'dadır.

func silu64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0 // x·σ(x) → 0 (tensör yoluyla aynı)
	}
	return x / (1 + detmath.Exp(-x))
}

// registerWideMath genişletilmiş matematik yerleşiklerini kaydeder.
func registerWideMath(v *interp.VM) {
	unit(v, codes.FnSilu, (*tensor.Tensor).Silu, silu64)
	unit(v, codes.FnTan, (*tensor.Tensor).Tan, detmath.Tan)
	unit(v, codes.FnAsin, (*tensor.Tensor).Asin, detmath.Asin)
	unit(v, codes.FnAcos, (*tensor.Tensor).Acos, detmath.Acos)
	unit(v, codes.FnAtan, (*tensor.Tensor).Atan, detmath.Atan)
	unit(v, codes.FnSinh, (*tensor.Tensor).Sinh, detmath.Sinh)
	unit(v, codes.FnCosh, (*tensor.Tensor).Cosh, detmath.Cosh)
	unit(v, codes.FnLog1p, (*tensor.Tensor).Log1p, detmath.Log1p)
	unit(v, codes.FnLog2, (*tensor.Tensor).Log2, detmath.Log2)
	unit(v, codes.FnLog10, (*tensor.Tensor).Log10, detmath.Log10)
	unit(v, codes.FnExpm1, (*tensor.Tensor).Expm1, detmath.Expm1)
	registerRounding(v, codes.FnFloor, (*tensor.Tensor).Floor)
	registerRounding(v, codes.FnCeil, (*tensor.Tensor).Ceil)
	registerRounding(v, codes.FnSign, (*tensor.Tensor).Sign)
	registerRounding(v, codes.FnAbs, (*tensor.Tensor).Abs) // mutlak: türevli (işaret(x), 0'da 0)
	registerIsFinite(v)
	wrap(v, codes.FnClamp, newBuiltin(clampBuiltin))
	wrap(v, codes.FnAtan2, newBuiltin(atan2Builtin))
	wrap(v, codes.FnWhere, newBuiltin(whereBuiltin))   // yerlesik_secim.go
	wrap(v, codes.FnCumsum, newBuiltin(cumSumBuiltin)) // yerlesik_siralama.go
	wrap(v, codes.FnTopk, newBuiltin(topKBuiltin))
	wrap(v, codes.FnArgsort, newBuiltin(argsortBuiltin))
}

// isFinite sonlu_mu'nun tensör yoludur (15.16a, 15.36): x − x sonlu öğede 0, NaN ve ±∞'da NaN'dır;
// NaN kendine eşit değildir. Sonuç TR_Tensor[TR_Bool]'dur ve türevsizdir (torch.isfinite de x == x
// ile |x| ≠ ∞'u birleştirir). TR_fuse'un izinde karşılaştırmalar gibi float32 0/1 kalır: izde mantık
// tensörü yoktur; "≥ 0" karşılaştırması NaN'da 0 verir. Ara fark başlığı hemen bırakılır.
func isFinite(t *tensor.Tensor) *tensor.Tensor {
	var r *tensor.Tensor
	tensor.NoGrad(func() {
		d := finiteGap(t)
		if t.IsSymbolic() {
			r = d.CompareScalar(0, tensor.OpGe, false)
		} else {
			r = tensor.CompareTyped(d, d, tensor.CmpEq)
		}
		d.Drop()
	})
	return r
}

// finiteGap sonlu öğede 0, öbürlerinde NaN olan tensördür; tam sayı ve mantık öğesi her zaman sonludur.
func finiteGap(t *tensor.Tensor) *tensor.Tensor {
	if t.Elem() != tensor.ElemFloat {
		return tensor.FullOn(t.Device(), 0, t.Shape()...)
	}
	return t.Sub(t)
}

// registerIsFinite sonlu_mu'nun tensör yolunu bağlar: sonuç TR_Tensor[TR_Bool]'dur (15.16a), TR_Int
// girdide de; kaynaştır izinde Ondalık 0/1 kalır (isFinite).
func registerIsFinite(v *interp.VM) {
	registerTensorWrap(v, codes.FnIsFinite, false, isFinite)
}

// registerRounding taban, tavan, işaret ve mutlak'ı sarar: tensörde eleman bazlı (türevi tensör
// işleminindir: taban, tavan ve işarette 0, mutlak'ta işaret(x); TR_Int tensörün sonucu da
// TR_Int'tir, typedRounding), sayıda çekirdek yerleşiği (Tam → Tam), listede yol gösteren hata.
func registerRounding(v *interp.VM, kd codes.Code, fn func(*tensor.Tensor) *tensor.Tensor) {
	registerTensorWrap(v, kd, true, fn)
}

// registerTensorWrap tek tensör argümanlı yerleşiği sarar: adlı argüman yok, listede yol gösteren
// hata, tensör dışı girdide eski yerleşik; keepInts doğruysa (taban, tavan, işaret, mutlak) cinsli
// tensör (TR_Int, float64) cinsini korur (typedRounding).
func registerTensorWrap(v *interp.VM, kd codes.Code, keepInts bool, fn func(*tensor.Tensor) *tensor.Tensor) {
	ad := codes.NameRef(kd)
	wrap(v, kd, func(old interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if err := bind.CheckNamed(ad.Called(), named); err != nil {
				return nil, err
			}
			if len(args) == 1 {
				if err := listError(ad, args[0], messages.New(messages.H6131), tensorCallExample(ad, "")); err != nil {
					return nil, err
				}
			}
			d, ok := singleTensor(args)
			if !ok {
				return oldCall(ad, old, m, args, named)
			}
			if keepInts && d.t.DType().IsTyped() {
				return typedRounding(kd, d)
			}
			r, err := safe(func() *tensor.Tensor { return fn(d.t) })
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}

// typedRounding taban, tavan, işaret ve mutlak'ın cinsli tensör yolu (15.12, 15.16): TR_Int'te
// taban ve tavan girdinin kendisidir, işaret ve mutlak aynı cinsi verir (en küçük değerin mutlak
// değeri taşmadır); float64'te işaret ve mutlak, taban ve tavan bu sürümde hassasiyet hatası.
func typedRounding(kd codes.Code, d *TensorValue) (interp.Value, error) {
	ad := codes.NameRef(kd)
	switch kd {
	case codes.FnSign:
		r, err := safe(func() *tensor.Tensor { return tensor.SignTyped(d.t) })
		if err != nil {
			return nil, err
		}
		return New(r), nil
	case codes.FnAbs:
		var f *tensor.ElemFault
		r, err := safe(func() (r *tensor.Tensor) { r, f = tensor.AbsTyped(d.t); return })
		if err != nil {
			return nil, err
		}
		return typedResult(ad, "", r, f)
	}
	if d.t.Elem() == tensor.ElemInt {
		return d, nil
	}
	return nil, tensor.PrecisionError(d.t.DType(), d.t.Device())
}

// typedClamp TR_clamp'ın cinsli tensör yolu (15.13): TR_Int tensörde sınırlar TR_Int olmalıdır,
// cins korunur; float64'te sınırlar float64'tür. Sınır yalnız verilen yanda uygulanır.
func typedClamp(ad any, x *TensorValue, lo, hi float64, ilo, ihi int64, intLimit bool) (interp.Value, error) {
	t := x.t
	if t.Elem() == tensor.ElemBool || t.Elem() == tensor.ElemInt && !intLimit {
		return nil, messages.New(messages.H6968, ad, t.DType().Name(), clampLimitText(lo, hi), map[bool]string{true: "number", false: "float"}[t.Elem() == tensor.ElemBool])
	}
	return safe(func() interp.Value {
		r := t
		bound := func(v float64, iv int64, op tensor.CmpOp) {
			var b *tensor.Tensor
			if t.Elem() == tensor.ElemInt {
				iv = max(min(iv, hiOf(t.DType())), loOf(t.DType()))
				b = tensor.Int64sOn(t.Device(), t.DType(), []int64{iv})
			} else {
				b = tensor.Float64sOn(t.Device(), []float64{v})
			}
			r = tensor.WhereTyped(tensor.CompareTyped(r, b, op), b, r)
		}
		if !math.IsInf(lo, -1) {
			bound(lo, ilo, tensor.CmpLt)
		}
		if !math.IsInf(hi, 1) {
			bound(hi, ihi, tensor.CmpGt)
		}
		return New(r)
	})
}

func loOf(dt tensor.DType) int64 { lo, _ := dt.IntRange(); return lo }
func hiOf(dt tensor.DType) int64 { _, hi := dt.IntRange(); return hi }

// clampLimitText TR_clamp'ın sınırlarını iletide yazar.
func clampLimitText(lo, hi float64) string {
	return strconv.FormatFloat(lo, 'g', -1, 64) + ".." + strconv.FormatFloat(hi, 'g', -1, 64)
}

// singleTensor argümanlar tek bir tensörse onu verir.
func singleTensor(args []interp.Value) (*TensorValue, bool) {
	if len(args) != 1 {
		return nil, false
	}
	d, ok := args[0].(*TensorValue)
	return d, ok
}

// limitCount sınırla'nın bir sınırını okur: sayı olmalı, NaN olamaz.
func limitCount(ad any, isNamed bind.Named, v interp.Value) (float64, bool, error) {
	f, ok := interp.ToFloat(v)
	if !ok || f != f {
		return 0, false, messages.New(messages.H6107, ad, isNamed, interp.Shown(v))
	}
	_, integer := v.(*interp.IntVal)
	return f, integer, nil
}

// clampBuiltin sınırla(x, en_az: ..., en_fazla: ...).
func clampBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.NameRef(codes.FnClamp)
	usage := fmt.Sprintf("%s(x, %s: ..., %s: ...)", ad, nameMin.TR, nameMax.TR)
	if len(args) != 1 {
		return nil, messages.New(messages.H6108, usage, len(args))
	}
	if err := bind.CheckNamed(ad.Called(), named, nameMin, nameMax); err != nil {
		return nil, err
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	// Tam sınırlar Tam'a ayrıca, float64'e çevrilmeden uygulanır (|x| > 2⁵³'te float64
	// yuvarlaması kırpmayı bozar, int64(2⁶³) taşardı).
	ilo, ihi := int64(math.MinInt64), int64(math.MaxInt64)
	intLimit := true
	var given bool
	var low, high interp.Value
	for _, s := range []struct {
		isNamed bind.Named
		target  *float64
		exact   *int64
		value   *interp.Value
	}{{nameMin, &lo, &ilo, &low}, {nameMax, &hi, &ihi, &high}} {
		v, ok := bind.GetNamed(named, s.isNamed)
		if !ok {
			continue
		}
		f, integer, err := limitCount(ad, s.isNamed, v)
		if err != nil {
			return nil, err
		}
		*s.target, given, intLimit, *s.value = f, true, intLimit && integer, v
		if n, ok := v.(*interp.IntVal); ok {
			*s.exact = n.Val
		}
	}
	if !given {
		return nil, messages.New(messages.H6112, ad, nameMin, nameMax, usage)
	}
	if low != nil && high != nil {
		if c, _ := interp.CompareNumbers(low, high); c > 0 { // kesin (4.5): TR_Int yuvarlanmaz
			return nil, messages.New(messages.H6113, ad, nameMin, interp.Shown(low), nameMax, interp.Shown(high))
		}
	}
	switch x := args[0].(type) {
	case *TensorValue:
		if x.t.DType().IsTyped() {
			return typedClamp(ad, x, lo, hi, ilo, ihi, intLimit)
		}
		r, err := safe(func() *tensor.Tensor { return x.t.Clamp(float32(lo), float32(hi)) })
		if err != nil {
			return nil, err
		}
		return New(r), nil
	case *interp.IntVal, *interp.FloatVal:
		return clampNumber(x, low, high), nil
	}
	if err := listError(ad, args[0], messages.New(messages.H6131), tensorCallExample(ad, ", %s: 0", nameMin)); err != nil {
		return nil, err
	}
	return nil, messages.New(messages.H6103, ad, typeName(args[0]))
}

// clampNumber sınırla'nın sayı yoludur (13.7, 13.10): TR_max gibi kesin karşılaştırır, TR_Int yuvarlanmaz;
// üçünden biri TR_Float'sa dönen değer sonra TR_Float'a çevrilir. NaN x olduğu gibi kalır. Verilmeyen
// sınır nil'dir.
func clampNumber(x, low, high interp.Value) interp.Value {
	picked := x
	if c, ok := interp.CompareNumbers(x, low); ok && c < 0 {
		picked = low
	}
	if c, ok := interp.CompareNumbers(x, high); ok && c > 0 {
		picked = high
	}
	_, xFloat := x.(*interp.FloatVal)
	_, lowFloat := low.(*interp.FloatVal)
	_, highFloat := high.(*interp.FloatVal)
	if n, ok := picked.(*interp.IntVal); ok && (xFloat || lowFloat || highFloat) {
		return &interp.FloatVal{Val: float64(n.Val)}
	}
	return picked
}

// atan2Builtin atan2(y, x): iki sayıda Ondalık; biri tensörse yayınlamalı,
// türevli tensör (sayı tensörün türüne çevrilir).
func atan2Builtin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.NameRef(codes.FnAtan2)
	if err := bind.CheckNamed(ad.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, messages.New(messages.H6116, ad, len(args))
	}
	for _, a := range args {
		if err := listError(ad, a, messages.New(messages.H6131), tensorCallExample(ad, ", x")); err != nil {
			return nil, err
		}
	}
	if !isTensor(args) {
		y, ok1 := interp.ToFloat(args[0])
		x, ok2 := interp.ToFloat(args[1])
		if !ok1 || !ok2 {
			return nil, messages.New(messages.H6118, ad, typeName(args[0]), typeName(args[1]))
		}
		return &interp.FloatVal{Val: detmath.Atan2(y, x)}, nil
	}
	ts, err := toTensors(ad, args)
	if err != nil {
		return nil, err
	}
	if err := checkBroadcast(m, ad, ts[0], ts[1]); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.Atan2(ts[0], ts[1]) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// toTensors argümanlardan en az biri tensörken hepsini o tensörün cihazında tensöre
// çevirir; sayılar o tensörün türünü alır (t * 2 gibi). Farklı cihazlardaki tensörler hatadır.
func toTensors(ad any, args []interp.Value) ([]*tensor.Tensor, error) {
	var first *tensor.Tensor
	for _, a := range args {
		if d, ok := a.(*TensorValue); ok {
			first = d.t
			break
		}
	}
	out := make([]*tensor.Tensor, len(args))
	for i, a := range args {
		if d, ok := a.(*TensorValue); ok {
			if err := checkDevice(ad, first, d.t); err != nil {
				return nil, err
			}
			out[i] = d.t
			continue
		}
		t, err := toTensorOn(ad, a, first)
		if err != nil {
			return nil, err
		}
		if t.DType() != first.DType() {
			t = t.Cast(first.DType())
		}
		out[i] = t
	}
	return out, nil
}
