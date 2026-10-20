package ml

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Genişletilmiş matematik yerleşikleri: etkinleştirme (silu), ters trigonometri,
// hiperbolik, kararlı logaritma/üstel (birli ile: sayıda Ondalık, tensörde türevli
// tensör), taban/tavan/işaret/mutlak (sayı yolu internal/lang/interp'ta; tensörde tam sayılı
// işaret korunur), sınırla ve arktanjant2. Koşullu seçim yerlesik_secim.go'da, birikim ve sıralama
// yerlesik_siralama.go'dadır.

func silu64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0 // x·σ(x) → 0 (tensör yoluyla aynı)
	}
	return x / (1 + math.Exp(-x))
}

// registerWideMath genişletilmiş matematik yerleşiklerini kaydeder.
func registerWideMath(v *interp.VM) {
	unit(v, codes.T204, (*tensor.Tensor).Silu, silu64)
	unit(v, codes.T224, (*tensor.Tensor).Tan, math.Tan)
	unit(v, codes.T225, (*tensor.Tensor).Asin, math.Asin)
	unit(v, codes.T226, (*tensor.Tensor).Acos, math.Acos)
	unit(v, codes.T227, (*tensor.Tensor).Atan, math.Atan)
	unit(v, codes.T240, (*tensor.Tensor).Sinh, math.Sinh)
	unit(v, codes.T241, (*tensor.Tensor).Cosh, math.Cosh)
	unit(v, codes.T242, (*tensor.Tensor).Log1p, math.Log1p)
	unit(v, codes.T243, (*tensor.Tensor).Log2, math.Log2)
	unit(v, codes.T244, (*tensor.Tensor).Log10, math.Log10)
	unit(v, codes.T245, (*tensor.Tensor).Expm1, math.Expm1)
	registerRounding(v, codes.G406, (*tensor.Tensor).Floor)
	registerRounding(v, codes.G407, (*tensor.Tensor).Ceil)
	registerRounding(v, codes.G408, (*tensor.Tensor).Sign)
	registerRounding(v, codes.G403, (*tensor.Tensor).Abs) // mutlak: türevli (işaret(x), 0'da 0)
	wrap(v, codes.G409, newBuiltin(clampBuiltin))
	wrap(v, codes.T228, newBuiltin(atan2Builtin))
	wrap(v, codes.T234, newBuiltin(whereBuiltin))  // yerlesik_secim.go
	wrap(v, codes.T231, newBuiltin(cumSumBuiltin)) // yerlesik_siralama.go
	wrap(v, codes.T232, newBuiltin(topKBuiltin))
	wrap(v, codes.T233, newBuiltin(argsortBuiltin))
}

// registerRounding taban, tavan, işaret ve mutlak'ı sarar: tensörde eleman bazlı (türevi tensör
// işleminindir: taban, tavan ve işarette 0, mutlak'ta işaret(x); tam sayılı işaretli tensörün
// sonucu da tam sayılıdır), sayıda çekirdek yerleşiği (Tam → Tam), listede yol gösteren hata.
func registerRounding(v *interp.VM, kd codes.Code, fn func(*tensor.Tensor) *tensor.Tensor) {
	ad := codes.Name(kd)
	wrap(v, kd, func(old interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if err := bind.CheckNamed(ad+"()", named); err != nil {
				return nil, err
			}
			if len(args) == 1 {
				if err := listError(ad, args[0], messages.Text(messages.H6131), ad+"("+codes.Name(codes.T100)+"(...))"); err != nil {
					return nil, err
				}
			}
			d, ok := singleTensor(args)
			if !ok {
				return oldCall(ad, old, m, args, named)
			}
			r, err := safe(func() *tensor.Tensor { return fn(d.t) })
			if err != nil {
				return nil, err
			}
			s := New(r)
			s.hasInts = d.hasInts
			return s, nil
		}
	})
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
func limitCount(ad string, isNamed bind.Named, v interp.Value) (float64, bool, error) {
	f, ok := interp.ToFloat(v)
	if !ok || f != f {
		return 0, false, messages.New(messages.H6107, ad, isNamed.TR, v.Inspect())
	}
	_, integer := v.(*interp.IntVal)
	return f, integer, nil
}

// clampBuiltin sınırla(x, en_az: ..., en_fazla: ...).
func clampBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.G409)
	usage := fmt.Sprintf("%s(x, %s: ..., %s: ...)", ad, nameMin.TR, nameMax.TR)
	if len(args) != 1 {
		return nil, messages.New(messages.H6108, usage, len(args))
	}
	if err := bind.CheckNamed(ad+"()", named, nameMin, nameMax); err != nil {
		return nil, err
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	// Tam sınırlar Tam'a ayrıca, float64'e çevrilmeden uygulanır (|x| > 2⁵³'te float64
	// yuvarlaması kırpmayı bozar, int64(2⁶³) taşardı).
	ilo, ihi := int64(math.MinInt64), int64(math.MaxInt64)
	intLimit := true
	var given bool
	for _, s := range []struct {
		isNamed bind.Named
		target  *float64
		exact   *int64
	}{{nameMin, &lo, &ilo}, {nameMax, &hi, &ihi}} {
		v, ok := bind.GetNamed(named, s.isNamed)
		if !ok {
			continue
		}
		f, integer, err := limitCount(ad, s.isNamed, v)
		if err != nil {
			return nil, err
		}
		*s.target, given, intLimit = f, true, intLimit && integer
		if n, ok := v.(*interp.IntVal); ok {
			*s.exact = n.Val
		}
	}
	if !given {
		return nil, messages.New(messages.H6112, ad, nameMin.TR, nameMax.TR, usage)
	}
	if intLimit && ilo > ihi {
		return nil, messages.New(messages.H6113, ad, nameMin.TR, ilo, nameMax.TR, ihi)
	}
	if lo > hi {
		return nil, messages.New(messages.H6113, ad, nameMin.TR, lo, nameMax.TR, hi)
	}
	switch x := args[0].(type) {
	case *TensorValue:
		r, err := safe(func() *tensor.Tensor { return x.t.Clamp(float32(lo), float32(hi)) })
		if err != nil {
			return nil, err
		}
		s := New(r)
		s.hasInts = x.hasInts && intValued(lo) && intValued(hi)
		return s, nil
	case *interp.IntVal:
		if intLimit {
			return interp.NewInt(min(max(x.Val, ilo), ihi)), nil
		}
		return &interp.FloatVal{Val: clamp64(float64(x.Val), lo, hi)}, nil
	case *interp.FloatVal:
		return &interp.FloatVal{Val: clamp64(x.Val, lo, hi)}, nil
	}
	if err := listError(ad, args[0], messages.Text(messages.H6131), ad+"("+codes.Name(codes.T100)+"(...), "+nameMin.TR+": 0)"); err != nil {
		return nil, err
	}
	return nil, messages.New(messages.H6103, ad, typeName(args[0]))
}

// intValued f sonsuz ya da tam sayı değerli mi (sınırla sonucunun tam sayılı kalması için).
func intValued(f float64) bool { return math.IsInf(f, 0) || f == math.Trunc(f) }

// clamp64 x'i [lo, hi]'ye sınırlar; NaN korunur.
func clamp64(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// atan2Builtin arktanjant2(y, x): iki sayıda Ondalık; biri tensörse yayınlamalı,
// türevli tensör (sayı tensörün türüne çevrilir).
func atan2Builtin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T228)
	if err := bind.CheckNamed(ad+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, messages.New(messages.H6116, ad, len(args))
	}
	for _, a := range args {
		if err := listError(ad, a, messages.Text(messages.H6131), ad+"("+codes.Name(codes.T100)+"(...), x)"); err != nil {
			return nil, err
		}
	}
	if !isTensor(args) {
		y, ok1 := interp.ToFloat(args[0])
		x, ok2 := interp.ToFloat(args[1])
		if !ok1 || !ok2 {
			return nil, messages.New(messages.H6118, ad, typeName(args[0]), typeName(args[1]))
		}
		return &interp.FloatVal{Val: math.Atan2(y, x)}, nil
	}
	ts, err := toTensors(ad, args)
	if err != nil {
		return nil, err
	}
	if err := checkBroadcast(ad, ts[0], ts[1]); err != nil {
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
func toTensors(ad string, args []interp.Value) ([]*tensor.Tensor, error) {
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
