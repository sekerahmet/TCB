package ml

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosya kaynaştır(f)'i (dil tanımı 15.44) TR'ye bağlar: f eleman bazlı tensör
// işlemlerinden oluşan bir fonksiyondur; kaynaştır aynı şeyi hesaplayan bir fonksiyon
// döndürür. İlk çağrıda (ve her yeni şekil, tür ya da türev isteği bileşiminde) f iz
// tensörleriyle bir kez çalışır ve programı kaydedilir (tensor.TraceFused); sonraki çağrılar
// programı doğrudan yürütür (tensor.FuseProgram.Run): GPU'da her çıktı tek çekirdektir.

// registerFuse kaynaştır yerleşiğini kaydeder.
func registerFuse(v *interp.VM) {
	wrap(v, codes.FnFuse, newBuiltin(fuseBuiltin))
}

// fusedFn kaynaştır'ın döndürdüğü fonksiyonun durumu: kaynaştırılan fonksiyon ve girdi
// bileşimine göre programları.
type fusedFn struct {
	f     interp.Value
	cache map[string]*tensor.FuseProgram
}

// fuseBuiltin kaynaştır(f) yerleşiğidir.
func fuseBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnFuse)
	if len(named) > 0 {
		return nil, interp.NoNamedArgsError(fn, named)
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnFuse, 1, len(args))
	}
	if !isCallable(args[0]) {
		return nil, messages.New(messages.H6433, fn, typeName(args[0]))
	}
	f := &fusedFn{f: args[0], cache: map[string]*tensor.FuseProgram{}}
	return &interp.BuiltinVal{Name: codes.Name(codes.FnFuse), Fn: guarded(f.call), NoNamedArgs: true, Pure: true}, nil
}

// call kaynaşmış fonksiyonu çalıştırır. Argümanlar tensör ya da sayıdır; sayılar programa
// sabit olarak girer (önbellek anahtarının parçasıdır), tensörler programın girdileridir.
func (f *fusedFn) call(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnFuse)
	var inputs []*tensor.Tensor
	var info []tensor.FuseInputInfo
	var key strings.Builder
	fmt.Fprintf(&key, "%s|", mixedDType(m)) // hesap_türü() izde sabitlenir (15.44)
	for i, a := range args {
		if x, ok := interp.ToFloat(a); ok {
			fmt.Fprintf(&key, "#%T%v;", a, x)
			continue
		}
		d, ok := a.(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6434, fn, i+1, typeName(a))
		}
		in := tensor.FuseInputInfo{Shape: d.t.Shape(), DType: d.t.DType(), RequiresGrad: d.t.RequiresGrad() && tensor.GradEnabled()}
		inputs, info = append(inputs, d.t), append(info, in)
		fmt.Fprintf(&key, "%v:%s:%v:%v;", in.Shape, in.DType, in.RequiresGrad, d.t.Device().Kind)
	}
	if len(inputs) == 0 {
		if len(args) == 0 {
			return nil, interp.ArgCountError(codes.FnFuse, interp.AtLeast(1), 0)
		}
		return nil, messages.New(messages.H6434, fn, 1, typeName(args[0]))
	}
	p := f.cache[key.String()]
	if p == nil {
		var err error
		if p, err = f.trace(m, tensor.BackendFor(inputs[0].Device()), info, args); err != nil {
			return nil, err
		}
		f.cache[key.String()] = p
	}
	outs := p.Run(fn.String(), inputs) // hesap katmanı adı görüntü metni olarak alır
	return packTensorValues(outs, len(outs) == 1 && p.Single), nil
}

// trace f'i iz tensörleriyle çalıştırıp programını kurar. f'in hatası olduğu gibi döner.
func (f *fusedFn) trace(m *interp.VM, be tensor.Backend, info []tensor.FuseInputInfo, args []interp.Value) (*tensor.FuseProgram, error) {
	fn := codes.NameRef(codes.FnFuse)
	var callErr error
	single := false
	p := tensor.TraceFused(be, info, func(syms []*tensor.Tensor) []*tensor.Tensor {
		vals := make([]interp.Value, len(args))
		k := 0
		for i, a := range args {
			if _, tensorArg := a.(*TensorValue); tensorArg {
				vals[i] = New(syms[k])
				k++
			} else {
				vals[i] = a // sayı: programa sabit olarak girer
			}
		}
		res, err := m.ApplyFunction(f.f, vals, nil, 0)
		if err != nil {
			callErr = err
			return nil
		}
		switch r := res.(type) {
		case *TensorValue:
			single = true
			return []*tensor.Tensor{r.t}
		case *interp.ListVal:
			if len(r.Elements) == 0 {
				callErr = messages.New(messages.H6436, fn)
				return nil
			}
			out := make([]*tensor.Tensor, len(r.Elements))
			for i, e := range r.Elements {
				d, ok := e.(*TensorValue)
				if !ok {
					callErr = messages.New(messages.H6435, fn, typeName(res))
					return nil
				}
				out[i] = d.t
			}
			return out
		}
		callErr = messages.New(messages.H6435, fn, typeName(res))
		return nil
	})
	if callErr != nil {
		return nil, callErr
	}
	p.Single = single
	return p, nil
}
