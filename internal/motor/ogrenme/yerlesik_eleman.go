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
// o çağrılır; yoksa sayı Ondalık verir (sayi64 ile). Liste kabul edilmez (karar 15): yol
// gösteren hata tensör(...) ile çevirmeyi önerir.
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
				return sayiyaUygula(ad, args[0], sayi64)
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

// sayiyaUygula eleman bazlı fonksiyonu tensör olmayan bir değere uygular: sayı Ondalık
// verir (float64 ile); liste ve başka değerler yol gösteren hatadır.
func sayiyaUygula(ad string, v yurut.Value, f func(float64) float64) (yurut.Value, error) {
	if s, ok := yurut.ToFloat(v); ok {
		return &yurut.FloatVal{Val: f(s)}, nil
	}
	if err := listeHatasi(ad, v, "tensörde ve sayıda", ad+"(tensör(...))"); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s: sayı ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
}

// listeHatasi değer bir liste ise eleman bazlı fonksiyonların listede çalışmadığını söyleyen,
// tensöre çevirmeyi öneren hatayı verir (karar 15); liste değilse nil.
func listeHatasi(ad string, v yurut.Value, kabul, oneri string) error {
	if _, ok := v.(*yurut.ListVal); !ok {
		return nil
	}
	return fmt.Errorf("%s listede çalışmaz, yalnızca %s; listeyi tensör(...) ile çevirin: %s", ad, kabul, oneri)
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
// (varsayılan son eksen) çalışır; liste ve başka değerler yol gösteren hatadır.
func softmaxKaydet(v *yurut.VM, kd kod.Kod, logKip bool) {
	ad := kod.Ad(kd)
	sar(v, kd, func(yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if len(args) == 0 || !tensorMu(args[:1]) {
				if len(args) == 0 {
					return nil, fmt.Errorf("%s(x, eksen: -1) en az 1 parametre bekler", ad)
				}
				if err := listeHatasi(ad, args[0], "tensörde", ad+"(tensör(...), eksen: -1)"); err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("%s: tensör bekleniyor, %s verildi", ad, tipAdi(args[0]))
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
