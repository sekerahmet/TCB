package process

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// MaskStreamType TR_MaskStream'in tip adıdır.
var MaskStreamType = interp.TypeName(codes.TypeMaskStream)

// MaskStream TR_mask_stream()'in değeridir (19.4a): parça parça gelen metni insanın göreceği çıktıya
// maskeli verir. Algoritma interp'tedir (interp.MaskStream), gizli değer listesinin yanında.
type MaskStream struct{ s *interp.MaskStream }

func (m *MaskStream) Type() interp.ValueType { return MaskStreamType }
func (m *MaskStream) IsTruthy() bool         { return true }

// Inspect değerin metnidir (3.14): tipin adı.
func (m *MaskStream) Inspect() string { return codes.ValueNames().Name(codes.TypeMaskStream) }

var maskStreamMembers = []codes.Code{codes.MemStep, codes.MemFinish}

// GetProperty m.TR_step(parça) ve m.TR_finish()'i verir.
func (m *MaskStream) GetProperty(kd codes.Code, name string) (interp.Value, error) {
	switch kd {
	case codes.MemStep:
		return bind.CodeMethod(kd, true, m.step), nil
	case codes.MemFinish:
		return bind.CodeMethod(kd, true, m.finish), nil
	}
	return nil, bind.MemberError(MaskStreamType, name, maskStreamMembers)
}

// step m.TR_step(parça)'dır: parça bir metindir, kararı kesinleşen baş maskeli döner; bitmiş akışta
// H7634.
func (m *MaskStream) step(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.MemStep, 1, len(args))
	}
	p, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.MemStep, 1, args[0], codes.TypeString)
	}
	out, open := m.s.Step(p.Val)
	if !open {
		return nil, messages.New(messages.H7634, interp.CallPrefix(codes.MemStep))
	}
	return &interp.StringVal{Val: out}, nil
}

// finish m.TR_finish()'tir: tutulan maskeli döner, akış biter; ikinci çağrı boş metin verir.
func (m *MaskStream) finish(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemFinish, 0, len(args))
	}
	return &interp.StringVal{Val: m.s.Finish()}, nil
}

// maskStream TR_mask_stream()'dir.
func maskStream(v *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.FnMaskStream, 0, len(args))
	}
	return &MaskStream{s: v.NewMaskStream()}, nil
}

var _ interp.PropertyGetter = (*MaskStream)(nil)
