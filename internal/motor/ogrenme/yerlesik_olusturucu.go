package ogrenme

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// turevIzle türev_izle adlı argümanını (ya da verilen konumlu değeri) okur.
func turevIzle(named map[string]yurut.Value, konumlu yurut.Value, varsayilan bool) bool {
	if konumlu != nil {
		varsayilan = konumlu.IsTruthy()
	}
	if v, ok := baglama.AdliAl(named, adTurevIzle); ok {
		return v.IsTruthy()
	}
	return varsayilan
}

// tensorYerlesik tensör(veri, türev_izle: yanlış, tür: "float32") yerleşiğidir. Veri tek
// sayı (0 boyutlu tensör), iç içe sayı listesi ya da tensör olabilir. tür verilmezse
// listeden float32, tensörden kaynağın türünde tensör oluşur.
func tensorYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T100)
	if len(args) < 1 {
		return nil, fmt.Errorf("%s en az 1 parametre bekler", turevIzleyenOrnek("veri"))
	}
	if len(args) > 2 {
		return nil, fmt.Errorf("%s en fazla 2 parametre alır, %d verildi", turevIzleyenOrnek("veri"), len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adTurevIzle, adTur); err != nil {
		return nil, err
	}
	var konumlu yurut.Value
	if len(args) == 2 {
		konumlu = args[1]
	}
	izle := turevIzle(named, konumlu, false)
	var t *tensor.Tensor
	if d, ok := args[0].(*Deger); ok {
		// Türü değiştirmenin tek yazımı t.türe(...)'dir; tür: yalnızca listeden ya da
		// sayıdan oluştururken verilir (karar 14).
		if _, verildi := baglama.AdliAl(named, adTur); verildi {
			return nil, fmt.Errorf("%s: girdi zaten bir tensör; türünü değiştirmek için t.%s(...) yazın (%s: yalnızca listeden ya da sayıdan oluştururken verilir)", fn, kod.Ad(kod.Y415), adTur.TR)
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
func sekilliOlusturucu(kd kod.Kod, olustur func(sh []int, named map[string]yurut.Value) (*tensor.Tensor, error)) yurut.BuiltinFunc {
	ad := kod.Ad(kd)
	return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
		}
		izinli := []baglama.Ad{adTurevIzle, adTur}
		if kd == kod.T103 { // rastgele_normal
			izinli = append(izinli, adTohum)
		}
		if _, liste := args[0].(*yurut.ListVal); !liste {
			if _, tam := args[0].(*yurut.IntVal); tam {
				return nil, fmt.Errorf("%s(%s): şekil liste olarak verilir; %s([%s]) yazın", ad, argMetni(args), ad, argMetni(args))
			}
			return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, tipAdi(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
		}
		if err := baglama.AdliDenetle(ad+"()", named, izinli...); err != nil {
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

func rastgeleNormal(sh []int, named map[string]yurut.Value) (*tensor.Tensor, error) {
	if s, ok := baglama.AdliAl(named, adTohum); ok {
		tohum, ok := s.(*yurut.IntVal)
		if !ok {
			return nil, fmt.Errorf("%s: %s tam sayı olmalıdır, %s verildi", kod.Ad(kod.T103), adTohum.TR, tipAdi(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(tohum.Val)), sh...), nil
	}
	tohumsuzBildir()
	rastgeleMu.Lock()
	defer rastgeleMu.Unlock()
	return tensor.Randn(rastgeleKay, sh...), nil
}
