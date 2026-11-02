package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// koşullu(koşul, a, b): öğe öğe seçim (15.15; bkz. typed_where.go ve tensor.Where).

// whereBuiltin TR_where(koşul, a, b). Koşul TR_Tensor[TR_Bool]'dur; tek değere bağlı seçim
// TR_if'tir (sayı imzası yok). Dallar typedWhere'de kurulur.
func whereBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.NameRef(codes.FnWhere)
	if err := bind.CheckNamed(ad.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 3 {
		return nil, messages.New(messages.H6201, ad, len(args))
	}
	for _, a := range args {
		if err := listError(ad, a, messages.New(messages.H6133, codes.NameRef(codes.TypeBool)), tensorCallExample(ad, ", a, b")); err != nil {
			return nil, err
		}
	}
	return typedWhere(m, ad, args)
}

// checkBroadcast3 koşul, a ve b şekillerinin birlikte yayınlanabilirliğini denetler.
func checkBroadcast3(m *interp.VM, ad any, ts []*tensor.Tensor) error {
	nd := 0
	for _, t := range ts {
		nd = max(nd, t.Dim())
	}
	for i := 1; i <= nd; i++ {
		size := 1
		for _, t := range ts {
			d := axisFromEnd(t, i)
			if d != 1 && size != 1 && d != size {
				return messages.New(messages.H6214, ad, tensor.ShapeText(ts[0].Shape()), tensor.ShapeText(ts[1].Shape()), tensor.ShapeText(ts[2].Shape()), i)
			}
			if d != 1 {
				size = d
			}
		}
	}
	if m != nil {
		// Üç şekilden herhangi ikisi birden genişliyorsa tek uyarı (15.49).
		for _, pair := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
			if bothWiden(ts[pair[0]], ts[pair[1]]) {
				broadcastWarning(m, ad, ts[pair[0]], ts[pair[1]])
				break
			}
		}
	}
	return nil
}
