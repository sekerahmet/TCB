package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dondurma (dil tanımı 15.34): yaprağın .türev_izle'si okunur ve atanabilir. Yanlış yaprağı
// dondurur ve gradyanını siler; doğru türev izlemeyi açar, yalnız kendi deposu olan, görünüm
// olmayan, nicemlenmemiş ondalık yaprakta (görünümde ve gradyanın takma adında açmak, depoyu
// paylaşan tensörü de iyileştiriciye yazdırırdı). İyileştirici donmuş parametreyi atlar.

// SetMember t.türev_izle = değer atamasıdır (interp.MemberAssignable); tensörün öbür üyelerine
// atanmaz.
func (d *TensorValue) SetMember(kd codes.Code, name string, value interp.Value) error {
	if kd != memberTracking {
		return messages.New(messages.H2407, interp.ValTensor, name)
	}
	b, ok := value.(*interp.BoolVal)
	if !ok {
		return messages.New(messages.H6992, typeName(value))
	}
	t := d.t
	if d.intermediate || !t.IsLeaf() {
		return messages.New(messages.H6990, tensor.ShapeText(t.Shape()))
	}
	if !b.Val {
		t.Freeze()
		return nil
	}
	if t.RequiresGrad() {
		return nil
	}
	if dt := t.DType(); !(dt.IsFloat() || dt == tensor.Float64) {
		return messages.New(messages.H6991, "dtype", dt.Name())
	}
	if !t.OwnsStorage() {
		return messages.New(messages.H6991, "view", t.DType().Name())
	}
	t.SetRequiresGrad(true)
	return nil
}

var _ interp.MemberAssignable = (*TensorValue)(nil)
