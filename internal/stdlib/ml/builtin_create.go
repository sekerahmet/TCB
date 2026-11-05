package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"math/rand"
)

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// requiresGrad türev_izle adlı argümanını (ya da verilen konumlu değeri) okur.
func requiresGrad(fn any, named map[string]interp.Value, positional interp.Value, defaultVal bool) (bool, error) {
	// Mantık olmalı: dolu bir metin ("yanlış") doğru sayılıp türevi sessizce açardı.
	if positional != nil {
		b, err := bind.Bool(fn, nameRequiresGrad, positional)
		if err != nil {
			return false, err
		}
		defaultVal = b
	}
	return bind.OptionalBool(fn, named, nameRequiresGrad, defaultVal)
}

// tensorBuiltin TR_Tensor(veri, TR_requires_grad: TR_false, TR_dtype: ...) yerleşiğidir. Veri tek
// sayı ya da mantık değeri (0 boyutlu tensör), aralık, iç içe liste ya da tensör olabilir. Cins
// veriden çıkar (15.2, tensor_data.go); TR_dtype: cinsin içinde hassasiyeti seçer. Tensörden
// kaynağın türünde kopya oluşur.
func tensorBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.TypeTensor)
	if len(args) < 1 {
		return nil, messages.New(messages.H6001, gradExample)
	}
	if len(args) > 2 {
		return nil, messages.New(messages.H6002, gradExample, len(args))
	}
	if err := bind.CheckNamed(fn.Called(), named, nameRequiresGrad, nameDType); err != nil {
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
			return nil, messages.New(messages.H6008, fn, codes.NameRef(codes.MemToDtype), nameDType)
		}
		// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak (aynı tür).
		dt := d.t.DType()
		var err error
		t, err = safe(func() *tensor.Tensor {
			if dt.IsTyped() {
				return tensor.FromRawOn(d.t.Device(), dt, d.t.Raw(), d.t.Shape()...)
			}
			if dt == tensor.Float32 {
				return tensor.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
			}
			// 16 bitlik tür bitleriyle kopyalanır: float32'den geçseydi geçici bellek üç kat olur,
			// sinyal NaN'ı bozulurdu.
			return d.t.Detach().Clone()
		})
		if err != nil {
			return nil, err
		}
	} else {
		var kind codes.Code
		if t, kind, err = dataTensor(fn, args[0]); err != nil {
			return nil, err
		}
		dt, err := constructorDType(fn, named, kind)
		if err != nil {
			return nil, err
		}
		if dt == tensor.Float64 && kind == codes.TypeFloat {
			// float64 veri float32'den geçmeden yeniden kurulur (15.2: hassasiyet TR_dtype'tır).
			if t, err = kindData(fn, kind, dt, args[0]); err != nil {
				return nil, err
			}
		}
		if t, err = convertKind(fn, t, dt); err != nil {
			return nil, err
		}
	}
	if watch && t.Elem() != tensor.ElemFloat {
		return nil, messages.New(messages.H6973, fn, codes.NameRef(codes.ArgRequiresGrad), codes.NameRef(codes.KwTrue), t.DType().Name())
	}
	t.SetRequiresGrad(watch)
	return New(t), nil
}

// shapedConstructor şekli liste olarak alan oluşturucular (sıfırlar([2, 3]) ...) içindir.
// Şeklin tek yazımı listedir: sıfırlar(2, 3) biçimi kaldırıldı, yazılınca liste biçimini
// önerir.
func shapedConstructor(kd codes.Code, create func(m *interp.VM, sh []int, named map[string]interp.Value) (*tensor.Tensor, error)) interp.BuiltinFunc {
	ad := codes.NameRef(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) == 0 {
			return nil, messages.New(messages.H6017, ad)
		}
		allowed := []bind.Named{nameRequiresGrad, nameDType}
		if kd == codes.FnRandn { // rastgele_normal
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
		if err := bind.CheckNamed(ad.Called(), named, allowed...); err != nil {
			return nil, err
		}
		sh, err := shapeList(ad, args[0])
		if err != nil {
			return nil, err
		}
		dt, err := constructorDType(ad, named, codes.TypeFloat)
		if err != nil {
			return nil, err
		}
		var t *tensor.Tensor
		if kd == codes.FnZeros && (dt.IsFloat() || dt.IsQuantized()) {
			// Doğrudan kendi türünde: dönüşüm yok, bellek ilk yazılana dek dokunulmamış kalır.
			if t, err = safe(func() *tensor.Tensor { return tensor.ZerosOf(dt, sh...) }); err != nil {
				return nil, err
			}
		} else {
			if t, err = create(m, sh, named); err != nil {
				return nil, err
			}
			// Yeni tensör türev izlemediği için dönüşüm çizgeye kaydedilmez; sonuç yapraktır.
			if t, err = convertKind(ad, t, dt); err != nil {
				return nil, err
			}
		}
		watch, err := requiresGrad(ad, named, nil, false)
		if err != nil {
			return nil, err
		}
		t.SetRequiresGrad(watch)
		return New(t), nil
	}
}

// randomNormal tohumlu çağrıda kendi üretecini, tohumsuzda programın paylaşılan üretecini
// (shared_random.go, 20.17) kullanır.
func randomNormal(m *interp.VM, sh []int, named map[string]interp.Value) (*tensor.Tensor, error) {
	if s, ok := bind.GetNamed(named, nameSeed); ok {
		seed, ok := s.(*interp.IntVal)
		if !ok {
			return nil, messages.New(messages.H6031, codes.NameRef(codes.FnRandn), nameSeed, typeName(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(seed.Val)), sh...), nil
	}
	var t *tensor.Tensor
	sharedRandomOf(m).draw(func(stream *rand.Rand) { t = tensor.Randn(stream, sh...) })
	return t, nil
}
