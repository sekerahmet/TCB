package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// Türev
// ----------------------------------------------------------------------------

// backpropagation geri_yayılım(kayıp) yerleşiğidir: tek elemanlı kayıptan geri yayılım yapar;
// yaprak tensörlerin gradyanları birikir.
func backpropagation(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T300)
	if len(args) != 1 {
		return nil, messages.New(messages.H6400, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H6030, fn, typeName(args[0]))
	}
	t := d.t
	if t.Numel() != 1 {
		return nil, messages.New(messages.H6401, fn, shapeText(t.Shape()), codes.Name(codes.G400), codes.Name(codes.G410))
	}
	if !t.RequiresGrad() {
		return nil, messages.New(messages.H6402, fn, gradExample("..."), codes.Name(codes.T301))
	}
	// Dış düğümlerin (özel_türev) geri hesapları bu görevle çalışır.
	if err := backprop(m, t, nil); err != nil {
		return nil, err
	}
	return interp.TRUE, nil
}

// noGrad iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func noGrad(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T301)
	if len(args) != 1 {
		return nil, messages.New(messages.H6403, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *TensorValue:
		return New(x.t.Detach()), nil
	case *interp.FuncVal, *interp.BuiltinVal:
		var result interp.Value
		var err error
		tensor.NoGrad(func() {
			result, err = m.ApplyFunction(x, nil, nil, 0)
		})
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = interp.NULL
		}
		return result, nil
	}
	return nil, messages.New(messages.H6404, fn, typeName(args[0]))
}
