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

// turevIzle türev_izle adlı argümanını (ya da verilen konumlu değeri) okur.
func turevIzle(named map[string]interp.Value, konumlu interp.Value, varsayilan bool) bool {
	if konumlu != nil {
		varsayilan = konumlu.IsTruthy()
	}
	if v, ok := bind.GetNamed(named, adTurevIzle); ok {
		return v.IsTruthy()
	}
	return varsayilan
}

// tensorYerlesik tensör(veri, türev_izle: yanlış, tür: "float32") yerleşiğidir. Veri tek
// sayı (0 boyutlu tensör), iç içe sayı listesi ya da tensör olabilir. tür verilmezse
// listeden float32, tensörden kaynağın türünde tensör oluşur.
func tensorYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T100)
	if len(args) < 1 {
		return nil, fmt.Errorf("%s en az 1 parametre bekler", turevIzleyenOrnek("veri"))
	}
	if len(args) > 2 {
		return nil, fmt.Errorf("%s en fazla 2 parametre alır, %d verildi", turevIzleyenOrnek("veri"), len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, adTurevIzle, adTur); err != nil {
		return nil, err
	}
	var konumlu interp.Value
	if len(args) == 2 {
		konumlu = args[1]
	}
	izle := turevIzle(named, konumlu, false)
	var t *tensor.Tensor
	if d, ok := args[0].(*Deger); ok {
		// Türü değiştirmenin tek yazımı t.türe(...)'dir; tür: yalnızca listeden ya da
		// sayıdan oluştururken verilir (karar 14).
		if _, verildi := bind.GetNamed(named, adTur); verildi {
			return nil, fmt.Errorf("%s: girdi zaten bir tensör; türünü değiştirmek için t.%s(...) yazın (%s: yalnızca listeden ya da sayıdan oluştururken verilir)", fn, codes.Name(codes.Y415), adTur.TR)
		}
		// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak (aynı tür).
		dt := d.t.DType()
		var err error
		t, err = guvenli(func() *tensor.Tensor {
			if dt == tensor.Float32 {
				return tensor.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
			}
			return tensor.NewOf(dt, d.t.Data(), d.t.Shape()...).To(d.t.Device())
		})
		if err != nil {
			return nil, err
		}
	} else {
		dt, err := adliTur(fn, named, tensor.Float32)
		if err != nil {
			return nil, err
		}
		t, err = tensoreCevir(fn, args[0])
		if err != nil {
			return nil, err
		}
		if t, err = tureCevir(t, dt); err != nil {
			return nil, err
		}
	}
	t.SetRequiresGrad(izle)
	return Yeni(t), nil
}

// sekilliOlusturucu şekli liste olarak alan oluşturucular (sıfırlar([2, 3]) ...) içindir.
// Şeklin tek yazımı listedir: sıfırlar(2, 3) biçimi kaldırıldı, yazılınca liste biçimini
// önerir.
func sekilliOlusturucu(kd codes.Code, olustur func(sh []int, named map[string]interp.Value) (*tensor.Tensor, error)) interp.BuiltinFunc {
	ad := codes.Name(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
		}
		izinli := []bind.Named{adTurevIzle, adTur}
		if kd == codes.T103 { // rastgele_normal
			izinli = append(izinli, adTohum)
		}
		if _, liste := args[0].(*interp.ListVal); !liste {
			if _, tam := args[0].(*interp.IntVal); tam {
				return nil, fmt.Errorf("%s(%s): şekil liste olarak verilir; %s([%s]) yazın", ad, argMetni(args), ad, argMetni(args))
			}
			return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, tipAdi(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
		}
		if err := bind.CheckNamed(ad+"()", named, izinli...); err != nil {
			return nil, err
		}
		sh, err := sekilListesi(ad, args[0])
		if err != nil {
			return nil, err
		}
		dt, err := adliTur(ad, named, tensor.Float32)
		if err != nil {
			return nil, err
		}
		t, err := olustur(sh, named)
		if err != nil {
			return nil, err
		}
		// Yeni tensör türev izlemediği için dönüşüm çizgeye kaydedilmez; sonuç yapraktır.
		if t, err = tureCevir(t, dt); err != nil {
			return nil, err
		}
		t.SetRequiresGrad(turevIzle(named, nil, false))
		return Yeni(t), nil
	}
}

var (
	rastgeleMu  sync.Mutex
	rastgeleKay = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// tohumsuzKanca tohumsuz rastgelelik (paylaşılan, saate göre tohumlanmış üreteç) her
// kullanıldığında çağrılır. Kontrol noktası, sürdürülen eğitimin kesintisizle aynı
// olmayacağını bir kez bildirmek için kurar; kanca yoksa maliyet tek bir atomik okumadır.
var tohumsuzKanca atomic.Pointer[func()]

// TohumsuzKancasiAyarla tohumsuz rastgelelik kancasını kurar (nil: kaldırır). Kanca
// yorumlayıcı kilidi altında çağrılır; kısa olmalı ve TR kodu çalıştırmamalıdır.
func TohumsuzKancasiAyarla(f func()) {
	if f == nil {
		tohumsuzKanca.Store(nil)
		return
	}
	tohumsuzKanca.Store(&f)
}

// tohumsuzBildir paylaşılan üreteç kullanılmadan önce çağrılır.
func tohumsuzBildir() {
	if f := tohumsuzKanca.Load(); f != nil {
		(*f)()
	}
}

func rastgeleNormal(sh []int, named map[string]interp.Value) (*tensor.Tensor, error) {
	if s, ok := bind.GetNamed(named, adTohum); ok {
		tohum, ok := s.(*interp.IntVal)
		if !ok {
			return nil, fmt.Errorf("%s: %s tam sayı olmalıdır, %s verildi", codes.Name(codes.T103), adTohum.TR, tipAdi(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(tohum.Val)), sh...), nil
	}
	tohumsuzBildir()
	rastgeleMu.Lock()
	defer rastgeleMu.Unlock()
	return tensor.Randn(rastgeleKay, sh...), nil
}
