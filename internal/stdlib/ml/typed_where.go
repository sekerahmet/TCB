package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_where'in cinsli yolu (15.15): koşul TR_Bool tensördür; a ve b aynı cinsten tensör ya da
// 15.6a'ya uyan sayıdır. İki sayıda cins onlardan çıkar: iki TR_Int → "int64", TR_Int ile
// TR_Float → "float32", iki mantık değeri → TR_Bool. Hesap ondalığı dallar türevi taşır
// (tensor.Where TR_Bool koşulu kabul eder); öbür cinsler türev izlemez.

// typedWhere koşul ve dalları kurar ve seçer.
func typedWhere(m *interp.VM, ad any, args []interp.Value) (interp.Value, error) {
	c, err := whereCond(ad, args[0])
	if err != nil {
		return nil, err
	}
	a, b, err := whereBranches(ad, c, args[1], args[2])
	if err != nil {
		return nil, err
	}
	ts := []*tensor.Tensor{c, a, b}
	if err := checkBroadcast3(m, ad, ts); err != nil {
		return nil, err
	}
	for _, t := range ts[1:] {
		if err := checkDevice(ad, c, t); err != nil {
			return nil, err
		}
	}
	return safe(func() interp.Value {
		if a.DType().IsFloat() {
			if c.Device() != tensor.CPU {
				// GPU'nun türevli seçim çekirdeği koşulu float32 okur (TR_Bool koşul 2b'de).
				c, _ = tensor.Convert(c, tensor.Float32)
			}
			return New(tensor.Where(c, a, b))
		}
		return New(tensor.WhereTyped(c, a, b))
	})
}

// whereCond koşulun TR_Tensor[TR_Bool] olduğunu denetler (15.15): ondalık ya da tam sayı tensör
// maskeye karşılaştırmayla çevrilir (`t != 0`); tensör olmayan koşulda seçim TR_if'tir.
func whereCond(ad any, v interp.Value) (*tensor.Tensor, error) {
	x, isTensor := v.(*TensorValue)
	switch {
	case isTensor && x.t.Elem() == tensor.ElemBool:
		return x.t, nil
	case isTensor:
		return nil, messages.New(messages.H6207, ad, codes.NameRef(kindOf(x.t.DType())), "number")
	}
	return nil, messages.New(messages.H6207, ad, typeName(v), "other")
}

// whereBranches a ve b'yi aynı türden tensörlere çevirir (koşulun cihazında).
func whereBranches(ad any, c *tensor.Tensor, av, bv interp.Value) (*tensor.Tensor, *tensor.Tensor, error) {
	ref := c
	for _, v := range []interp.Value{av, bv} {
		if t, ok := v.(*TensorValue); ok {
			ref = t.t
			break
		}
	}
	branch := func(v interp.Value) (*tensor.Tensor, error) {
		if t, ok := v.(*TensorValue); ok {
			return t.t, nil
		}
		if ref == c { // iki dal da sayı: cins sayılardan (15.15)
			return scalarBranch(ad, c.Device(), v, av, bv)
		}
		if ref.DType().IsFloat() {
			f, ok := interp.ToFloat(v)
			if !ok {
				return nil, messages.New(messages.H6204, ad, typeName(v))
			}
			return safe(func() *tensor.Tensor { return tensor.FullOn(ref.Device(), float32(f)) })
		}
		return (&TensorValue{t: ref}).typedOperandFor(ad, "", v, false)
	}
	a, err := branch(av)
	if err != nil {
		return nil, nil, err
	}
	b, err := branch(bv)
	if err != nil {
		return nil, nil, err
	}
	return sameKindBranches(ad, a, b)
}

// scalarBranch iki dal da sayıyken birini kurar: iki TR_Int → "int64", biri ondalıksa
// "float32", iki mantık değeri → TR_Bool.
func scalarBranch(ad any, dev tensor.Device, v, av, bv interp.Value) (*tensor.Tensor, error) {
	_, ai := av.(*interp.IntVal)
	_, bi := bv.(*interp.IntVal)
	switch x := v.(type) {
	case *interp.BoolVal:
		return safe(func() *tensor.Tensor { return tensor.Int64sOn(dev, tensor.Bool, []int64{b2i(x.Val)}) })
	case *interp.IntVal:
		if ai && bi {
			return safe(func() *tensor.Tensor { return tensor.Int64sOn(dev, tensor.Int64, []int64{x.Val}) })
		}
	}
	f, ok := interp.ToFloat(v)
	if !ok {
		return nil, messages.New(messages.H6204, ad, typeName(v))
	}
	return safe(func() *tensor.Tensor { return tensor.FullOn(dev, float32(f)) })
}

// sameKindBranches iki dalı aynı türe getirir: cinsler aynı olmalı; TR_Int'te hassasiyetler
// birleşir (15.6c), ondalıkta float64 baskındır, hesap ondalıkları Where'de birleşir.
func sameKindBranches(ad any, a, b *tensor.Tensor) (*tensor.Tensor, *tensor.Tensor, error) {
	if a.Elem() != b.Elem() {
		return nil, nil, kindMismatch(ad, a.DType(), b.DType())
	}
	if a.DType() == b.DType() || a.DType().IsFloat() && b.DType().IsFloat() {
		return a, b, nil
	}
	dt := tensor.Float64
	if a.Elem() == tensor.ElemInt {
		dt = tensor.MergeInt(a.DType(), b.DType())
	}
	var x, y *tensor.Tensor
	_, err := safe(func() *tensor.Tensor {
		x, _ = tensor.Convert(a, dt)
		y, _ = tensor.Convert(b, dt)
		return nil
	})
	return x, y, err
}
