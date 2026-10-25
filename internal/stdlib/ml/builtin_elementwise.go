package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"math"
)

// ----------------------------------------------------------------------------
// Eleman bazlı fonksiyonlar
// ----------------------------------------------------------------------------

func gelu64(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(x+0.044715*x*x*x)))
}

// geluExact64 tam (erf tabanlı) GELU: x·½·erfc(−x/√2); −∞'da 0 (tensör yoluyla aynı).
func geluExact64(x float64) float64 {
	phi := 0.5 * math.Erfc(-x/math.Sqrt2)
	if phi == 0 {
		return 0
	}
	return x * phi
}

// registerGelu gelu(x, yaklaşık: doğru) yerleşiğini kaydeder: yaklaşık: doğru (varsayılan)
// tanh yaklaşımlı, yanlış tam (erf tabanlı) GELU'dur. Öteki eleman bazlı fonksiyonlar gibi
// tensörde türev izleyen tensör, sayıda Ondalık verir, listede yol gösteren hata verir.
func registerGelu(v *interp.VM) {
	ad := codes.NameRef(codes.T203)
	wrap(v, codes.T203, func(interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if err := bind.CheckNamed(ad.Called(), named, nameApproximate); err != nil {
				return nil, err
			}
			if len(args) != 1 {
				return nil, messages.New(messages.H6101, ad, len(args))
			}
			approximate, err := bind.OptionalBool(ad, named, nameApproximate, true)
			if err != nil {
				return nil, err
			}
			if !isTensor(args) {
				if approximate {
					return applyToNumber(ad, args[0], gelu64)
				}
				return applyToNumber(ad, args[0], geluExact64)
			}
			t, err := toTensor(ad, args[0])
			if err != nil {
				return nil, err
			}
			fn := (*tensor.Tensor).Gelu
			if !approximate {
				fn = (*tensor.Tensor).GeluExact
			}
			r, err := safe(func() *tensor.Tensor { return fn(t) })
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}

func relu64(x float64) float64 {
	if x > 0 || x != x { // NaN yayılır (tensör relu'suyla aynı)
		return x
	}
	return 0
}

func sigmoid64(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }

// unit tek argümanlı eleman bazlı bir fonksiyonu kaydeder. Tensör (matris dahil)
// girdisinde tensör yolu çalışır. Tensör olmayan girdide eski yerleşiği varsa (karekök)
// o çağrılır; yoksa sayı Ondalık verir (sayi64 ile). Liste kabul edilmez (karar 15): yol
// gösteren hata tensör(...) ile çevirmeyi önerir.
func unit(v *interp.VM, kd codes.Code, fn func(*tensor.Tensor) *tensor.Tensor, number64 func(float64) float64) {
	ad := codes.NameRef(kd)
	wrap(v, kd, func(old interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) != 1 || !isTensor(args) {
				if err := bind.CheckNamed(ad.Called(), named); err != nil {
					return nil, err
				}
				if old != nil {
					return old(m, args, named)
				}
				if len(args) != 1 {
					return nil, messages.New(messages.H6101, ad, len(args))
				}
				return applyToNumber(ad, args[0], number64)
			}
			if err := bind.CheckNamed(ad.Called(), named); err != nil {
				return nil, err
			}
			t, err := toTensor(ad, args[0])
			if err != nil {
				return nil, err
			}
			r, err := safe(func() *tensor.Tensor { return fn(t) })
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}

// applyToNumber eleman bazlı fonksiyonu tensör olmayan bir değere uygular: sayı Ondalık
// verir (float64 ile); liste ve başka değerler yol gösteren hatadır.
func applyToNumber(ad codes.Ref, v interp.Value, f func(float64) float64) (interp.Value, error) {
	if s, ok := interp.ToFloat(v); ok {
		return &interp.FloatVal{Val: f(s)}, nil
	}
	if err := listError(ad, v, messages.New(messages.H6131), ad.String()+"("+codes.Name(codes.T100)+"(...))"); err != nil {
		return nil, err
	}
	return nil, messages.New(messages.H6103, ad, typeName(v))
}

// listError değer bir liste ise eleman bazlı fonksiyonların listede çalışmadığını söyleyen,
// tensöre çevirmeyi öneren hatayı verir (karar 15); liste değilse nil.
func listError(ad any, v interp.Value, accept any, example string) error {
	if _, ok := v.(*interp.ListVal); !ok {
		return nil
	}
	return messages.New(messages.H6104, ad, accept, codes.NameRef(codes.T100), example)
}

// axisArg eksen adlı argümanını ya da konumlu ikinci argümanı okur.
func axisArg(fn any, args []interp.Value, named map[string]interp.Value) (int, bool, error) {
	var ev interp.Value
	if len(args) >= 2 {
		ev = args[1]
	}
	if v, ok := bind.GetNamed(named, nameAxis); ok {
		if ev != nil {
			return 0, false, messages.New(messages.H6105, fn)
		}
		ev = v
	}
	if ev == nil || ev == interp.Null {
		return 0, false, nil
	}
	e, ok := intNumber(ev)
	if !ok {
		return 0, false, messages.New(messages.H6106, fn, interp.Shown(ev))
	}
	return e, true, nil
}

// registerSoftmax softmax/log_softmax'ı kaydeder: tensörde (matris dahil) verilen eksende
// (varsayılan son eksen) çalışır; liste ve başka değerler yol gösteren hatadır.
func registerSoftmax(v *interp.VM, kd codes.Code, logMode bool) {
	ad := codes.NameRef(kd)
	wrap(v, kd, func(interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) == 0 || !isTensor(args[:1]) {
				if len(args) == 0 {
					return nil, messages.New(messages.H6109, ad, nameAxis)
				}
				if err := listError(ad, args[0], messages.New(messages.H6132), ad.String()+"("+codes.Name(codes.T100)+"(...), "+nameAxis.TR+": -1)"); err != nil {
					return nil, err
				}
				return nil, messages.New(messages.H6110, ad, typeName(args[0]))
			}
			if len(args) > 2 {
				return nil, messages.New(messages.H6111, ad, nameAxis, len(args))
			}
			if err := bind.CheckNamed(ad.Called(), named, nameAxis); err != nil {
				return nil, err
			}
			t, err := toTensor(ad, args[0])
			if err != nil {
				return nil, err
			}
			if t, err = highPrecision(m, t); err != nil {
				return nil, err
			}
			e, given, err := axisArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			if !given {
				e = -1
			}
			e, err = parseAxis(ad, e, t.Shape())
			if err != nil {
				return nil, err
			}
			r, err := safe(func() *tensor.Tensor {
				if logMode {
					return t.LogSoftmax(e)
				}
				return t.Softmax(e)
			})
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}
