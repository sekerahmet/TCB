package ml

import (
	"encoding/binary"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bayt ve sayı (22.1–22.3): baytlardan tensör ve sayı okumak, tensörü ve sayıyı bayta yazmak.

var nameByteOrder = bind.NamedArg(codes.ArgByteOrder)

// byteFormat bir çağrının bayt biçimidir.
type byteFormat struct {
	dt   tensor.DType
	name string
	size int
	big  bool
}

func (f byteFormat) order() binary.ByteOrder {
	if f.big {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// registerBytes yerleşikleri ve TR_Bytes üyelerini bağlar.
func registerBytes(v *interp.VM) {
	wrap(v, codes.FnFromBytes, newBuiltin(func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
		return nil, messages.New(messages.H7051, codes.NameRef(codes.FnFromBytes), codes.Spelling(codes.FnFromBytes))
	}))
	v.RegisterTypeArgs(codes.FnFromBytes, fromBytesType)
	wrap(v, codes.FnIntToBytes, newBuiltin(intToBytes))
	wrap(v, codes.FnFloatToBytes, newBuiltin(floatToBytes))
	interp.RegisterBytesMember(codes.MemReadInt, readNumber(codes.MemReadInt, codes.TypeInt))
	interp.RegisterBytesMember(codes.MemReadFloat, readNumber(codes.MemReadFloat, codes.TypeFloat))
}

// fromBytesType TR_from_bytes[E]'nin çağrılabilir değeridir.
func fromBytesType(_ *interp.VM, args []*interp.TypeVal) (*interp.BuiltinVal, error) {
	k := codes.FnFromBytes
	fn := codes.NameRef(k)
	kind := args[0].Code
	if _, ok := kindDTypes[kind]; !ok || args[0].Struct != nil || args[0].Elems != nil {
		return nil, messages.New(messages.H6971, fn, interp.Shown(args[0]), codes.Spelling(k))
	}
	call := func(_ *interp.VM, a []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if err := bind.CheckNamed(fn.Called(), named, nameDType, nameByteOrder); err != nil {
			return nil, err
		}
		if len(a) != 1 {
			return nil, interp.ArgCountError(k, 1, len(a))
		}
		b, ok := a[0].(*interp.BytesVal)
		if !ok {
			return nil, interp.ArgTypeError(k, 1, a[0], codes.TypeBytes)
		}
		f, err := tensorFormat(fn, named, kind)
		if err != nil {
			return nil, err
		}
		return decodeTensor(fn, b.Data(), f)
	}
	return &interp.BuiltinVal{Name: codes.Name(k), Fn: guarded(call), Pure: true}, nil
}

// tensorFormat TR_from_bytes'ın biçimidir: hassasiyet E'nin cinsindendir.
func tensorFormat(fn codes.Ref, named map[string]interp.Value, kind codes.Code) (byteFormat, error) {
	f, err := precision(fn, named)
	if err != nil {
		return f, err
	}
	if k := kindOf(f.dt); k != kind {
		return f, messages.New(messages.H6976, fn, f.name, codes.NameRef(k), codes.NameRef(kind), "type_arg", codes.Spelling(fn.Code), codes.Spelling(k))
	}
	return f, orderOf(fn, named, &f)
}

// precision TR_dtype:'ı okur: zorunludur, nicemlenmiş hassasiyet baytlardan kurulmaz.
func precision(fn any, named map[string]interp.Value) (byteFormat, error) {
	v, given := bind.GetNamed(named, nameDType)
	if !given {
		return byteFormat{}, messages.New(messages.H7052, fn)
	}
	dt, name, err := anyDType(fn, v)
	if err != nil {
		return byteFormat{}, err
	}
	if dt.IsQuantized() {
		return byteFormat{}, messages.New(messages.H7053, fn, name)
	}
	return byteFormat{dt: dt, name: name, size: tensor.ElemSize(dt)}, nil
}

// orderOf TR_byte_order:'ı okur: çok baytlı hassasiyette zorunludur; makinenin sırası yoktur.
func orderOf(fn any, named map[string]interp.Value, f *byteFormat) error {
	v, given := bind.GetNamed(named, nameByteOrder)
	if !given {
		if f.size > 1 {
			return messages.New(messages.H7055, fn, f.name)
		}
		return nil
	}
	s, ok := v.(*interp.StringVal)
	if !ok || s.Val != "little" && s.Val != "big" {
		return messages.New(messages.H7056, fn, interp.Shown(v))
	}
	f.big = s.Val == "big"
	return nil
}

// decodeTensor baytları denetleyip tensör kurar.
func decodeTensor(fn any, data []byte, f byteFormat) (interp.Value, error) {
	if rest := len(data) % f.size; rest != 0 {
		return nil, messages.New(messages.H7057, fn, len(data), f.size, rest, len(data)-rest)
	}
	if f.dt == tensor.Bool {
		for i, x := range data {
			if x > 1 {
				return nil, messages.New(messages.H7058, fn, i, int(x))
			}
		}
	}
	return safe(func() interp.Value { return New(tensor.FromBytesOn(tensor.DefaultDevice(), f.dt, data, f.big)) })
}

// toBytes t.TR_to_bytes(TR_byte_order: sıra) metodudur (22.2): sonuç türevlenmez.
func (d *TensorValue) toBytes(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberToBytes)
	if err := bind.CheckNamed(fn.Called(), named, nameByteOrder); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, interp.ArgCountError(memberToBytes, 0, len(args))
	}
	dt := d.t.DType()
	if dt.IsQuantized() {
		return nil, messages.New(messages.H6687, dt.Name())
	}
	f := byteFormat{dt: dt, name: dt.Name(), size: tensor.ElemSize(dt)}
	if err := orderOf(fn, named, &f); err != nil {
		return nil, err
	}
	return safe(func() interp.Value { return interp.NewBytes(d.t.Bytes(f.big)) })
}

// readNumber b.TR_read_int ve b.TR_read_float'tır (22.3).
func readNumber(kd, kind codes.Code) func(b *interp.BytesVal) *interp.BuiltinVal {
	fn := codes.NameRef(kd)
	return func(b *interp.BytesVal) *interp.BuiltinVal {
		call := func(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if err := bind.CheckNamed(fn.Called(), named, nameDType, nameByteOrder); err != nil {
				return nil, err
			}
			if len(args) != 1 {
				return nil, interp.ArgCountError(kd, 1, len(args))
			}
			pos, ok := args[0].(*interp.IntVal)
			if !ok {
				return nil, interp.ArgTypeError(kd, 1, args[0], codes.TypeInt)
			}
			f, err := numberFormat(fn, named, kind, otherReader[kd])
			if err != nil {
				return nil, err
			}
			raw, err := slot(fn, b.Data(), pos.Val, f.size)
			if err != nil {
				return nil, err
			}
			if kind == codes.TypeInt {
				return interp.NewInt(decodeInt(raw, f)), nil
			}
			return &interp.FloatVal{Val: decodeFloat(raw, f)}, nil
		}
		return &interp.BuiltinVal{Name: codes.Name(kd), Fn: call, Pure: true}
	}
}

// otherReader öbür cinsin çağrısıdır: iletinin önerisi.
var otherReader = map[codes.Code]codes.Code{
	codes.MemReadInt: codes.MemReadFloat, codes.MemReadFloat: codes.MemReadInt,
	codes.FnIntToBytes: codes.FnFloatToBytes, codes.FnFloatToBytes: codes.FnIntToBytes,
}

// numberFormat tek sayının biçimidir: hassasiyet kind cinsindendir.
func numberFormat(fn any, named map[string]interp.Value, kind, other codes.Code) (byteFormat, error) {
	f, err := precision(fn, named)
	if err != nil {
		return f, err
	}
	if k := kindOf(f.dt); k == codes.TypeBool {
		return f, messages.New(messages.H7060, fn)
	} else if k != kind {
		return f, messages.New(messages.H7054, fn, f.name, codes.NameRef(k), codes.NameRef(kind), codes.Spelling(other))
	}
	return f, orderOf(fn, named, &f)
}

// slot konumdaki size baytı verir. Karşılaştırma toplama yapmaz: en büyük TR_Int taşmaz.
func slot(fn any, data []byte, pos int64, size int) ([]byte, error) {
	if pos < 0 {
		return nil, messages.New(messages.H7059, fn, pos, size, "negative", len(data))
	}
	if len(data) < size || pos > int64(len(data)-size) {
		return nil, messages.New(messages.H7059, fn, pos, size, "other", len(data))
	}
	return data[pos : pos+int64(size)], nil
}

// decodeInt tam sayı hassasiyetini okur.
func decodeInt(raw []byte, f byteFormat) int64 {
	o := f.order()
	switch f.dt {
	case tensor.Int8:
		return int64(int8(raw[0]))
	case tensor.UInt8:
		return int64(raw[0])
	case tensor.Int16:
		return int64(int16(o.Uint16(raw)))
	case tensor.UInt16:
		return int64(o.Uint16(raw))
	case tensor.Int32:
		return int64(int32(o.Uint32(raw)))
	case tensor.UInt32:
		return int64(o.Uint32(raw))
	}
	return int64(o.Uint64(raw))
}

// decodeFloat ondalık hassasiyeti okur.
func decodeFloat(raw []byte, f byteFormat) float64 {
	o := f.order()
	switch f.dt {
	case tensor.Float32:
		return float64(math.Float32frombits(o.Uint32(raw)))
	case tensor.Float64:
		return math.Float64frombits(o.Uint64(raw))
	}
	return tensor.Bits16ToFloat(f.dt, o.Uint16(raw))
}

// intToBytes TR_int_to_bytes(n, ...): hassasiyete sığmayan değer hatadır (15.2).
func intToBytes(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn, kd := codes.NameRef(codes.FnIntToBytes), codes.FnIntToBytes
	n, err := oneArg[*interp.IntVal](kd, args, named, codes.TypeInt)
	if err != nil {
		return nil, err
	}
	f, err := numberFormat(fn, named, codes.TypeInt, otherReader[kd])
	if err != nil {
		return nil, err
	}
	if !f.dt.Fits(n.Val) {
		return nil, messages.New(messages.H6964, fn, "empty", n.Val, f.name, "other")
	}
	out := make([]byte, 8)
	if f.big {
		binary.BigEndian.PutUint64(out, uint64(n.Val))
		return interp.NewBytes(out[8-f.size:]), nil
	}
	binary.LittleEndian.PutUint64(out, uint64(n.Val))
	return interp.NewBytes(out[:f.size]), nil
}

// floatToBytes TR_float_to_bytes(x, ...): hassasiyetin en yakın değerine yuvarlanır (15.2).
func floatToBytes(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn, kd := codes.NameRef(codes.FnFloatToBytes), codes.FnFloatToBytes
	if err := bind.CheckNamed(fn.Called(), named, nameDType, nameByteOrder); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(kd, 1, len(args))
	}
	x, ok := interp.ToFloat(args[0])
	if !ok {
		return nil, interp.ArgTypeError(kd, 1, args[0], codes.TypeFloat)
	}
	f, err := numberFormat(fn, named, codes.TypeFloat, otherReader[kd])
	if err != nil {
		return nil, err
	}
	out, o := make([]byte, f.size), f.order()
	switch f.dt {
	case tensor.Float32:
		o.PutUint32(out, math.Float32bits(float32(x)))
	case tensor.Float64:
		o.PutUint64(out, math.Float64bits(x))
	default:
		o.PutUint16(out, tensor.FloatToBits16(f.dt, x))
	}
	return interp.NewBytes(out), nil
}

// oneArg adlı argümanları ve tek konumlu argümanı denetler.
func oneArg[T interp.Value](kd codes.Code, args []interp.Value, named map[string]interp.Value, want codes.Code) (T, error) {
	var zero T
	if err := bind.CheckNamed(codes.NameRef(kd).Called(), named, nameDType, nameByteOrder); err != nil {
		return zero, err
	}
	if len(args) != 1 {
		return zero, interp.ArgCountError(kd, 1, len(args))
	}
	v, ok := args[0].(T)
	if !ok {
		return zero, interp.ArgTypeError(kd, 1, args[0], want)
	}
	return v, nil
}
