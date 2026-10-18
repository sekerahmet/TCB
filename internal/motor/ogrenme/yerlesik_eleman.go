package ogrenme

import (
	"fmt"
	"math"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// ----------------------------------------------------------------------------
// Eleman bazlı fonksiyonlar
// ----------------------------------------------------------------------------

func gelu64(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(x+0.044715*x*x*x)))
}

func relu64(x float64) float64 {
	if x > 0 || x != x { // NaN yayılır (tensör relu'suyla aynı)
		return x
	}
	return 0
}

func sigmoid64(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }

// birli tek argümanlı eleman bazlı bir fonksiyonu kaydeder. Tensör (matris dahil)
// girdisinde tensör yolu çalışır. Tensör olmayan girdide eski yerleşiği varsa (karekök)
// o çağrılır; yoksa sonuç girdinin biçimindedir: sayı → Ondalık (sayi64 ile), liste → aynı
// yapıda liste.
func birli(v *yurut.VM, kd kod.Kod, fn func(*tensor.Tensor) *tensor.Tensor, sayi64 func(float64) float64) {
	ad := kod.Ad(kd)
	sar(v, kd, func(eski yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 1 || !tensorMu(args) {
				if err := baglama.AdliDenetle(ad+"()", named); err != nil {
					return nil, err
				}
				if eski != nil {
					return eski(m, args, named)
				}
				if len(args) != 1 {
					return nil, fmt.Errorf("%s(x) 1 parametre bekler, %d verildi", ad, len(args))
				}
				return eskiBicimdeUygula(ad, args[0], sayi64)
			}
			if err := baglama.AdliDenetle(ad+"()", named); err != nil {
				return nil, err
			}
			t, err := tensoreCevir(ad, args[0])
			if err != nil {
				return nil, err
			}
			r, err := guvenli(func() *tensor.Tensor { return fn(t) })
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// eskiBicimdeUygula eleman bazlı fonksiyonu tensör olmayan bir değere, değerin kendi
// biçimini koruyarak (float64 ile) uygular.
func eskiBicimdeUygula(ad string, v yurut.Value, f func(float64) float64) (yurut.Value, error) {
	if x, ok := v.(*yurut.ListVal); ok {
		el := make([]yurut.Value, len(x.Elements))
		for i, e := range x.Elements {
			r, err := eskiBicimdeUygula(ad, e, f)
			if err != nil {
				return nil, err
			}
			el[i] = r
		}
		return &yurut.ListVal{Elements: el}, nil
	}
	if s, ok := yurut.ToFloat(v); ok {
		return &yurut.FloatVal{Val: f(s)}, nil
	}
	return nil, fmt.Errorf("%s: sayı, liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
}

// softmaxListe softmax'ı (logKip ise log_softmax'ı) düz sayı listesine float64 ile uygular
// ve liste döndürür.
func softmaxListe(ad string, v yurut.Value, logKip bool) (yurut.Value, error) {
	x, ok := v.(*yurut.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
	}
	if len(x.Elements) == 0 {
		return nil, fmt.Errorf("%s: boş liste", ad)
	}
	sayilar := make([]float64, len(x.Elements))
	enb := math.Inf(-1)
	for i, e := range x.Elements {
		f, ok := yurut.ToFloat(e)
		if !ok {
			return nil, fmt.Errorf("%s: liste düz sayı listesi olmalıdır (%d. eleman %s); iç içe veri için %s(tensör(...), eksen: -1) yazın", ad, i+1, tipAdi(e), ad)
		}
		sayilar[i] = f
		enb = math.Max(enb, f)
	}
	ustel := make([]float64, len(sayilar))
	toplam := 0.0
	for i, f := range sayilar {
		ustel[i] = math.Exp(f - enb)
		toplam += ustel[i]
	}
	el := make([]yurut.Value, len(sayilar))
	for i := range sayilar {
		if logKip {
			el[i] = &yurut.FloatVal{Val: sayilar[i] - (enb + math.Log(toplam))}
		} else {
			el[i] = &yurut.FloatVal{Val: ustel[i] / toplam}
		}
	}
	return &yurut.ListVal{Elements: el}, nil
}

// eksenArg eksen adlı argümanını ya da konumlu ikinci argümanı okur.
func eksenArg(fn string, args []yurut.Value, named map[string]yurut.Value) (int, bool, error) {
	var ev yurut.Value
	if len(args) >= 2 {
		ev = args[1]
	}
	if v, ok := baglama.AdliAl(named, adEksen); ok {
		if ev != nil {
			return 0, false, fmt.Errorf("%s: eksen hem konumlu hem adlı verildi", fn)
		}
		ev = v
	}
	if ev == nil || ev == yurut.NULL {
		return 0, false, nil
	}
	e, ok := tamSayi(ev)
	if !ok {
		return 0, false, fmt.Errorf("%s: eksen tam sayı olmalıdır, %s verildi", fn, ev.Inspect())
	}
	return e, true, nil
}

// softmaxKaydet softmax/log_softmax'ı kaydeder: tensörde (matris dahil) verilen eksende
// (varsayılan son eksen) çalışır; düz sayı listesinde liste döndürür.
func softmaxKaydet(v *yurut.VM, kd kod.Kod, logKip bool) {
	ad := kod.Ad(kd)
	sar(v, kd, func(yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if len(args) == 0 || !tensorMu(args[:1]) {
				if len(args) == 0 {
					return nil, fmt.Errorf("%s(x, eksen: -1) en az 1 parametre bekler", ad)
				}
				if len(args) > 1 || len(named) > 0 {
					return nil, fmt.Errorf("%s: eksen yalnızca tensörlerde kullanılır; %s(tensör(...), eksen: -1) yazın", ad, ad)
				}
				return softmaxListe(ad, args[0], logKip)
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, eksen: -1) en fazla 2 parametre alır, %d verildi", ad, len(args))
			}
			if err := baglama.AdliDenetle(ad+"()", named, adEksen); err != nil {
				return nil, err
			}
			t, err := tensoreCevir(ad, args[0])
			if err != nil {
				return nil, err
			}
			if t, err = yuksekHassasiyet(m, t); err != nil {
				return nil, err
			}
			e, verildi, err := eksenArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			if !verildi {
				e = -1
			}
			e, err = eksenCoz(ad, e, t.Shape())
			if err != nil {
				return nil, err
			}
			r, err := guvenli(func() *tensor.Tensor {
				if logKip {
					return t.LogSoftmax(e)
				}
				return t.Softmax(e)
			})
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}
