package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// nicemli_etkinleştirme bloğu (15.53a)
// ----------------------------------------------------------------------------

// actQuantKey görev durumunda (interp.GorevDegeri) TR_quantized_activations bloğunun anahtarıdır.
type actQuantKey struct{}

// actQuantOn görevin TR_quantized_activations bloğunun içinde olup olmadığıdır.
func actQuantOn(m *interp.VM) bool { return taskFlag(m, actQuantKey{}) }

// quantizedActivations TR_quantized_activations { ... } yerleşiğidir: bloğu, nicemlenmiş ağırlıkla çarpımda
// etkinleştirmenin nicemlendiği kipte çalıştırır ve bloğun son ifadesinin değerini döndürür (15.53a).
func quantizedActivations(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return flagBlock(m, actQuantKey{}, codes.FnQuantizedActivations, args, named)
}

// quantizedActivationsOn TR_quantized_activations_on() yerleşiğidir: görev TR_quantized_activations bloğunun
// içindeyse doğru (kitaplık ve koşu kaydı hangi yolun koştuğunu bilsin; 15.53a).
func quantizedActivationsOn(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return flagQuery(m, actQuantKey{}, codes.FnQuantizedActivationsOn, args, named)
}

// taskFlag görev durumundaki bir blok bayrağıdır (15.53a, 15.53b).
func taskFlag(m *interp.VM, key any) bool {
	if m == nil {
		return false
	}
	on, _ := m.TaskValue(key).(bool)
	return on
}

// flagBlock görev bayrağı açan bir bloğun yerleşiğidir (15.53a, 15.53b): bloğu bayrak açıkken çalıştırır ve
// bloğun son ifadesinin değerini döndürür. İç içe bloklar tek durumdur; blok bitince (hata ile bitse de) önceki
// durum geri gelir; TR_async ile açılan görev açıldığı yerin durumunu alır (taskFork).
func flagBlock(m *interp.VM, key any, code codes.Code, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(code)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H6725, fn, len(args))
	}
	switch args[0].(type) {
	case *interp.FuncVal, *interp.BuiltinVal:
	default:
		return nil, messages.New(messages.H6726, fn, typeName(args[0]))
	}
	prev := m.TaskValue(key)
	m.SetTaskValue(key, true)
	defer m.SetTaskValue(key, prev)
	result, err := applyFunction(m, args[0], nil, nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = interp.Null
	}
	return result, nil
}

// flagQuery bir blok bayrağının argümansız sorgusudur: görev bloğun içindeyse doğru.
func flagQuery(m *interp.VM, key any, code codes.Code, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if err := bind.CheckNamed(codes.NameRef(code).Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, interp.ArgCountError(code, 0, len(args))
	}
	if taskFlag(m, key) {
		return interp.True, nil
	}
	return interp.False, nil
}

// checkActQuant bloktaki nicemlenmiş ağırlıklı çarpımın kapısıdır (15.53a): ağırlık CPU'da ve q4_k ya da q6_k
// olmalıdır. Desteklenmeyen tür ve GPU hatadır, sessizce float yoluna düşmez: tür ileride desteklenince aynı
// programın bitleri sürümle değişmesin.
func checkActQuant(w *tensor.Tensor) error {
	fn := codes.NameRef(codes.FnQuantizedActivations)
	if w.Device() != tensor.CPU {
		return messages.New(messages.H6724, fn, w.DType().Name())
	}
	if !tensor.QuantActSupported(w.DType()) {
		return messages.New(messages.H6723, fn, w.DType().Name())
	}
	return nil
}
