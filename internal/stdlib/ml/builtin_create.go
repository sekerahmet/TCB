package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// requiresGrad türev_izle adlı argümanını (ya da verilen konumlu değeri) okur.
func requiresGrad(named map[string]interp.Value, positional interp.Value, defaultVal bool) bool {
	if positional != nil {
		defaultVal = positional.IsTruthy()
	}
	if v, ok := bind.GetNamed(named, nameRequiresGrad); ok {
		return v.IsTruthy()
	}
	return defaultVal
}

// tensorBuiltin tensör(veri, türev_izle: yanlış, tür: "float32") yerleşiğidir. Veri tek
// sayı (0 boyutlu tensör), iç içe sayı listesi ya da tensör olabilir. tür verilmezse
// listeden float32, tensörden kaynağın türünde tensör oluşur.
func tensorBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T100)
	if len(args) < 1 {
		return nil, fmt.Errorf("%s en az 1 parametre bekler", gradExample("veri"))
	}
	if len(args) > 2 {
		return nil, fmt.Errorf("%s en fazla 2 parametre alır, %d verildi", gradExample("veri"), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameRequiresGrad, nameDType); err != nil {
		return nil, err
	}
	var positional interp.Value
	if len(args) == 2 {
		positional = args[1]
	}
	watch := requiresGrad(named, positional, false)
	var t *tensor.Tensor
	if d, ok := args[0].(*TensorValue); ok {
		// Türü değiştirmenin tek yazımı t.türe(...)'dir; tür: yalnızca listeden ya da
		// sayıdan oluştururken verilir (karar 14).
		if _, given := bind.GetNamed(named, nameDType); given {
			return nil, fmt.Errorf("%s: girdi zaten bir tensör; türünü değiştirmek için t.%s(...) yazın (%s: yalnızca listeden ya da sayıdan oluştururken verilir)", fn, codes.Name(codes.Y415), nameDType.TR)
		}
		// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak (aynı tür).
		dt := d.t.DType()
		var err error
		t, err = safe(func() *tensor.Tensor {
			if dt == tensor.Float32 {
				return tensor.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
			}
			return tensor.NewOf(dt, d.t.Data(), d.t.Shape()...).To(d.t.Device())
		})
		if err != nil {
			return nil, err
		}
	} else {
		dt, err := namedDType(fn, named, tensor.Float32)
		if err != nil {
			return nil, err
		}
		t, err = toTensor(fn, args[0])
		if err != nil {
			return nil, err
		}
		if t, err = castTo(t, dt); err != nil {
			return nil, err
		}
	}
	t.SetRequiresGrad(watch)
	return New(t), nil
}

// shapedConstructor şekli liste olarak alan oluşturucular (sıfırlar([2, 3]) ...) içindir.
// Şeklin tek yazımı listedir: sıfırlar(2, 3) biçimi kaldırıldı, yazılınca liste biçimini
// önerir.
func shapedConstructor(kd codes.Code, create func(sh []int, named map[string]interp.Value) (*tensor.Tensor, error)) interp.BuiltinFunc {
	ad := codes.Name(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
		}
		allowed := []bind.Named{nameRequiresGrad, nameDType}
		if kd == codes.T103 { // rastgele_normal
			allowed = append(allowed, nameSeed)
		}
		if _, list := args[0].(*interp.ListVal); !list {
			if _, integer := args[0].(*interp.IntVal); integer {
				return nil, fmt.Errorf("%s(%s): şekil liste olarak verilir; %s([%s]) yazın", ad, argText(args), ad, argText(args))
			}
			return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
		}
		if err := bind.CheckNamed(ad+"()", named, allowed...); err != nil {
			return nil, err
		}
		sh, err := shapeList(ad, args[0])
		if err != nil {
			return nil, err
		}
		dt, err := namedDType(ad, named, tensor.Float32)
		if err != nil {
			return nil, err
		}
		t, err := create(sh, named)
		if err != nil {
			return nil, err
		}
		// Yeni tensör türev izlemediği için dönüşüm çizgeye kaydedilmez; sonuç yapraktır.
		if t, err = castTo(t, dt); err != nil {
			return nil, err
		}
		t.SetRequiresGrad(requiresGrad(named, nil, false))
		return New(t), nil
	}
}

var (
	isRandom     sync.Mutex
	randomStream = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// unseededHook tohumsuz rastgelelik (paylaşılan, saate göre tohumlanmış üreteç) her
// kullanıldığında çağrılır. Kontrol noktası, sürdürülen eğitimin kesintisizle aynı
// olmayacağını bir kez bildirmek için kurar; kanca yoksa maliyet tek bir atomik okumadır.
var unseededHook atomic.Pointer[func()]

// SetUnseededHook tohumsuz rastgelelik kancasını kurar (nil: kaldırır). Kanca
// yorumlayıcı kilidi altında çağrılır; kısa olmalı ve TR kodu çalıştırmamalıdır.
func SetUnseededHook(f func()) {
	if f == nil {
		unseededHook.Store(nil)
		return
	}
	unseededHook.Store(&f)
}

// reportUnseeded paylaşılan üreteç kullanılmadan önce çağrılır.
func reportUnseeded() {
	if f := unseededHook.Load(); f != nil {
		(*f)()
	}
}

func randomNormal(sh []int, named map[string]interp.Value) (*tensor.Tensor, error) {
	if s, ok := bind.GetNamed(named, nameSeed); ok {
		seed, ok := s.(*interp.IntVal)
		if !ok {
			return nil, fmt.Errorf("%s: %s tam sayı olmalıdır, %s verildi", codes.Name(codes.T103), nameSeed.TR, typeName(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(seed.Val)), sh...), nil
	}
	reportUnseeded()
	isRandom.Lock()
	defer isRandom.Unlock()
	return tensor.Randn(randomStream, sh...), nil
}
