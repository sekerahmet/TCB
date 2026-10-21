package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
func requiresGrad(fn string, named map[string]interp.Value, positional interp.Value, defaultVal bool) (bool, error) {
	// Mantık olmalı: dolu bir metin ("yanlış") doğru sayılıp türevi sessizce açardı.
	if positional != nil {
		b, err := bind.Bool(fn, nameRequiresGrad.TR, positional)
		if err != nil {
			return false, err
		}
		defaultVal = b
	}
	if v, ok := bind.GetNamed(named, nameRequiresGrad); ok {
		return bind.Bool(fn, nameRequiresGrad.TR, v)
	}
	return defaultVal, nil
}

// tensorBuiltin tensör(veri, türev_izle: yanlış, tür: "float32") yerleşiğidir. Veri tek
// sayı (0 boyutlu tensör), iç içe sayı listesi ya da tensör olabilir. tür verilmezse
// listeden float32, tensörden kaynağın türünde tensör oluşur.
func tensorBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T100)
	if len(args) < 1 {
		return nil, messages.New(messages.H6001, gradExample("veri"))
	}
	if len(args) > 2 {
		return nil, messages.New(messages.H6002, gradExample("veri"), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameRequiresGrad, nameDType); err != nil {
		return nil, err
	}
	var positional interp.Value
	if len(args) == 2 {
		positional = args[1]
	}
	watch, err := requiresGrad(fn, named, positional, false)
	if err != nil {
		return nil, err
	}
	var t *tensor.Tensor
	if d, ok := args[0].(*TensorValue); ok {
		// Türü değiştirmenin tek yazımı t.türe(...)'dir; tür: yalnızca listeden ya da
		// sayıdan oluştururken verilir (karar 14).
		if _, given := bind.GetNamed(named, nameDType); given {
			return nil, messages.New(messages.H6008, fn, codes.DisplayName(codes.Y415), nameDType.TR)
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
	ad := codes.DisplayName(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) == 0 {
			return nil, messages.New(messages.H6017, ad)
		}
		allowed := []bind.Named{nameRequiresGrad, nameDType}
		if kd == codes.T103 { // rastgele_normal
			allowed = append(allowed, nameSeed)
		}
		if _, list := args[0].(*interp.ListVal); !list {
			if _, integer := args[0].(*interp.IntVal); integer {
				return nil, messages.New(messages.H6020, ad, argText(args))
			}
			return nil, messages.New(messages.H6021, ad, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, messages.New(messages.H6023, ad, len(args))
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
		watch, err := requiresGrad(ad, named, nil, false)
		if err != nil {
			return nil, err
		}
		t.SetRequiresGrad(watch)
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
			return nil, messages.New(messages.H6031, codes.DisplayName(codes.T103), nameSeed.TR, typeName(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(seed.Val)), sh...), nil
	}
	reportUnseeded()
	isRandom.Lock()
	defer isRandom.Unlock()
	return tensor.Randn(randomStream, sh...), nil
}
