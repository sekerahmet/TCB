package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Eksen boyunca birikim ve sıralama yerleşikleri: birikimli_toplam, en_büyük_k,
// sıralama_indeksi. Üçü de yalnızca tensörde çalışır; eksen yalnızca adlı verilir
// (varsayılan son eksen).

// axisTensor ilk konumlu argümanın tensör olduğunu, konumlu argüman sayısını (konumlu
// sayısı n) ve eksen adlı argümanını denetleyip tensörü ve çözülmüş ekseni verir.
func axisTensor(ad, usage string, args []interp.Value, n int, named map[string]interp.Value) (*TensorValue, int, error) {
	if len(args) != n {
		if len(args) == n+1 {
			return nil, 0, messages.New(messages.H6200, usage, nameAxis, interp.Shown(args[n]))
		}
		return nil, 0, messages.New(messages.H6202, usage, n, len(args))
	}
	if err := bind.CheckNamed(ad+"()", named, nameAxis); err != nil {
		return nil, 0, err
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		if err := listError(ad, args[0], messages.Text(messages.H6132), ad+"("+codes.Name(codes.T100)+"(...))"); err != nil {
			return nil, 0, err
		}
		return nil, 0, messages.New(messages.H6203, ad, typeName(args[0]))
	}
	e := -1
	if v, ok := bind.GetNamed(named, nameAxis); ok {
		var integer bool
		if e, integer = intNumber(v); !integer {
			return nil, 0, messages.New(messages.H6106, ad, interp.Shown(v))
		}
	}
	e, err := parseAxis(ad, e, d.t.Shape())
	if err != nil {
		return nil, 0, err
	}
	return d, e, nil
}

// cumSumBuiltin birikimli_toplam(t, eksen: -1): şekil korunur, türev izler; tam
// sayılı tensörün sonucu da tam sayılıdır.
func cumSumBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.DisplayName(codes.T231)
	d, e, err := axisTensor(ad, fmt.Sprintf("%s(t, %s: -1)", ad, nameAxis.TR), args, 1, named)
	if err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return d.t.CumSum(e) })
	if err != nil {
		return nil, err
	}
	s := New(r)
	s.hasInts = d.hasInts
	return s, nil
}

// topKBuiltin en_büyük_k(t, k, eksen: -1) → [değerler, indeksler].
func topKBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.DisplayName(codes.T232)
	d, e, err := axisTensor(ad, fmt.Sprintf("%s(t, k, %s: -1)", ad, nameAxis.TR), args, 2, named)
	if err != nil {
		return nil, err
	}
	k, ok := args[1].(*interp.IntVal)
	L := d.t.Shape()[e]
	if !ok {
		return nil, messages.New(messages.H6208, ad, codes.NameRef(codes.A200), typeName(args[1]))
	}
	if k.Val < 0 || k.Val > int64(L) {
		return nil, messages.New(messages.H6210, ad, k.Val, tensor.ShapeText(d.t.Shape()), e, L)
	}
	var vals, idx *tensor.Tensor
	if _, err := safe(func() *tensor.Tensor { vals, idx = d.t.TopK(int(k.Val), e); return vals }); err != nil {
		return nil, err
	}
	v := New(vals)
	v.hasInts = d.hasInts
	return &interp.ListVal{Elements: []interp.Value{v, NewInt(idx)}}, nil
}

// argsortBuiltin sıralama_indeksi(t, eksen: -1): kararlı, küçükten büyüğe.
func argsortBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.DisplayName(codes.T233)
	d, e, err := axisTensor(ad, fmt.Sprintf("%s(t, %s: -1)", ad, nameAxis.TR), args, 1, named)
	if err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return d.t.ArgSort(e) })
	if err != nil {
		return nil, err
	}
	return NewInt(r), nil
}
