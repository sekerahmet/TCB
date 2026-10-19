package ogrenme

import (
	"fmt"
	"math"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// Genişletilmiş matematik yerleşikleri: etkinleştirme (silu), ters trigonometri,
// hiperbolik, kararlı logaritma/üstel (birli ile: sayıda Ondalık, tensörde türevli
// tensör), taban/tavan/işaret/mutlak (sayı yolu internal/dil/yurut'ta; tensörde tam sayılı
// işaret korunur), sınırla ve arktanjant2. Koşullu seçim yerlesik_secim.go'da, birikim ve sıralama
// yerlesik_siralama.go'dadır.

func silu64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0 // x·σ(x) → 0 (tensör yoluyla aynı)
	}
	return x / (1 + math.Exp(-x))
}

// matematigiGenisKaydet genişletilmiş matematik yerleşiklerini kaydeder.
func matematigiGenisKaydet(v *yurut.VM) {
	birli(v, kod.T204, (*tensor.Tensor).Silu, silu64)
	birli(v, kod.T224, (*tensor.Tensor).Tan, math.Tan)
	birli(v, kod.T225, (*tensor.Tensor).Asin, math.Asin)
	birli(v, kod.T226, (*tensor.Tensor).Acos, math.Acos)
	birli(v, kod.T227, (*tensor.Tensor).Atan, math.Atan)
	birli(v, kod.T240, (*tensor.Tensor).Sinh, math.Sinh)
	birli(v, kod.T241, (*tensor.Tensor).Cosh, math.Cosh)
	birli(v, kod.T242, (*tensor.Tensor).Log1p, math.Log1p)
	birli(v, kod.T243, (*tensor.Tensor).Log2, math.Log2)
	birli(v, kod.T244, (*tensor.Tensor).Log10, math.Log10)
	birli(v, kod.T245, (*tensor.Tensor).Expm1, math.Expm1)
	yuvarlamaKaydet(v, kod.G406, (*tensor.Tensor).Floor)
	yuvarlamaKaydet(v, kod.G407, (*tensor.Tensor).Ceil)
	yuvarlamaKaydet(v, kod.G408, (*tensor.Tensor).Sign)
	yuvarlamaKaydet(v, kod.G403, (*tensor.Tensor).Abs) // mutlak: türevli (işaret(x), 0'da 0)
	sar(v, kod.G409, yeniYerlesik(sinirlaYerlesik))
	sar(v, kod.T228, yeniYerlesik(arktanjant2Yerlesik))
	sar(v, kod.T234, yeniYerlesik(kosulluYerlesik))         // yerlesik_secim.go
	sar(v, kod.T231, yeniYerlesik(birikimliToplamYerlesik)) // yerlesik_siralama.go
	sar(v, kod.T232, yeniYerlesik(enBuyukKYerlesik))
	sar(v, kod.T233, yeniYerlesik(siralamaIndeksiYerlesik))
}

// yuvarlamaKaydet taban, tavan, işaret ve mutlak'ı sarar: tensörde eleman bazlı (türevi tensör
// işleminindir: taban, tavan ve işarette 0, mutlak'ta işaret(x); tam sayılı işaretli tensörün
// sonucu da tam sayılıdır), sayıda çekirdek yerleşiği (Tam → Tam), listede yol gösteren hata.
func yuvarlamaKaydet(v *yurut.VM, kd kod.Kod, fn func(*tensor.Tensor) *tensor.Tensor) {
	ad := kod.Ad(kd)
	sar(v, kd, func(eski yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if err := baglama.AdliDenetle(ad+"()", named); err != nil {
				return nil, err
			}
			if len(args) == 1 {
				if err := listeHatasi(ad, args[0], "tensörde ve sayıda", ad+"("+kod.Ad(kod.T100)+"(...))"); err != nil {
					return nil, err
				}
			}
			d, ok := tekTensor(args)
			if !ok {
				return eskiCagir(ad, eski, m, args, named)
			}
			r, err := guvenli(func() *tensor.Tensor { return fn(d.t) })
			if err != nil {
				return nil, err
			}
			s := Yeni(r)
			s.tamSayili = d.tamSayili
			return s, nil
		}
	})
}

// tekTensor argümanlar tek bir tensörse onu verir.
func tekTensor(args []yurut.Value) (*Deger, bool) {
	if len(args) != 1 {
		return nil, false
	}
	d, ok := args[0].(*Deger)
	return d, ok
}

// sinirSayisi sınırla'nın bir sınırını okur: sayı olmalı, NaN olamaz.
func sinirSayisi(ad string, adli baglama.Ad, v yurut.Value) (float64, bool, error) {
	f, ok := yurut.ToFloat(v)
	if !ok || f != f {
		return 0, false, fmt.Errorf("%s: %s bir sayı olmalıdır, %s verildi", ad, adli.TR, v.Inspect())
	}
	_, tam := v.(*yurut.IntVal)
	return f, tam, nil
}

// sinirlaYerlesik sınırla(x, en_az: ..., en_fazla: ...).
func sinirlaYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	ad := kod.Ad(kod.G409)
	kullanim := fmt.Sprintf("%s(x, %s: ..., %s: ...)", ad, adEnAz.TR, adEnFazla.TR)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s 1 konumlu parametre bekler, %d verildi", kullanim, len(args))
	}
	if err := baglama.AdliDenetle(ad+"()", named, adEnAz, adEnFazla); err != nil {
		return nil, err
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	tamSinir := true
	var verildi bool
	for _, s := range []struct {
		adli  baglama.Ad
		hedef *float64
	}{{adEnAz, &lo}, {adEnFazla, &hi}} {
		v, ok := baglama.AdliAl(named, s.adli)
		if !ok {
			continue
		}
		f, tam, err := sinirSayisi(ad, s.adli, v)
		if err != nil {
			return nil, err
		}
		*s.hedef, verildi, tamSinir = f, true, tamSinir && tam
	}
	if !verildi {
		return nil, fmt.Errorf("%s: %s ya da %s verilmelidir (ikisi de verilebilir): %s", ad, adEnAz.TR, adEnFazla.TR, kullanim)
	}
	if lo > hi {
		return nil, fmt.Errorf("%s: %s (%v) %s'dan (%v) büyük olamaz", ad, adEnAz.TR, lo, adEnFazla.TR, hi)
	}
	switch x := args[0].(type) {
	case *Deger:
		r, err := guvenli(func() *tensor.Tensor { return x.t.Clamp(float32(lo), float32(hi)) })
		if err != nil {
			return nil, err
		}
		s := Yeni(r)
		s.tamSayili = x.tamSayili && tamDegerli(lo) && tamDegerli(hi)
		return s, nil
	case *yurut.IntVal:
		if tamSinir {
			n := x.Val
			if f := float64(n); f < lo {
				n = int64(lo)
			} else if f > hi {
				n = int64(hi)
			}
			return yurut.NewInt(n), nil
		}
		return &yurut.FloatVal{Val: sinirla64(float64(x.Val), lo, hi)}, nil
	case *yurut.FloatVal:
		return &yurut.FloatVal{Val: sinirla64(x.Val, lo, hi)}, nil
	}
	if err := listeHatasi(ad, args[0], "tensörde ve sayıda", ad+"("+kod.Ad(kod.T100)+"(...), "+adEnAz.TR+": 0)"); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s: sayı ya da tensör bekleniyor, %s verildi", ad, tipAdi(args[0]))
}

// tamDegerli f sonsuz ya da tam sayı değerli mi (sınırla sonucunun tam sayılı kalması için).
func tamDegerli(f float64) bool { return math.IsInf(f, 0) || f == math.Trunc(f) }

// sinirla64 x'i [lo, hi]'ye sınırlar; NaN korunur.
func sinirla64(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// arktanjant2Yerlesik arktanjant2(y, x): iki sayıda Ondalık; biri tensörse yayınlamalı,
// türevli tensör (sayı tensörün türüne çevrilir).
func arktanjant2Yerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	ad := kod.Ad(kod.T228)
	if err := baglama.AdliDenetle(ad+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(y, x) 2 parametre bekler, %d verildi", ad, len(args))
	}
	for _, a := range args {
		if err := listeHatasi(ad, a, "tensörde ve sayıda", ad+"("+kod.Ad(kod.T100)+"(...), x)"); err != nil {
			return nil, err
		}
	}
	if !tensorMu(args) {
		y, ok1 := yurut.ToFloat(args[0])
		x, ok2 := yurut.ToFloat(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("%s: sayı ya da tensör bekleniyor, %s ve %s verildi", ad, tipAdi(args[0]), tipAdi(args[1]))
		}
		return &yurut.FloatVal{Val: math.Atan2(y, x)}, nil
	}
	ts, err := tensorlereCevir(ad, args)
	if err != nil {
		return nil, err
	}
	if err := yayinDenetle(ad, ts[0], ts[1]); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.Atan2(ts[0], ts[1]) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// tensorlereCevir argümanlardan en az biri tensörken hepsini o tensörün cihazında tensöre
// çevirir; sayılar o tensörün türünü alır (t * 2 gibi). Farklı cihazlardaki tensörler hatadır.
func tensorlereCevir(ad string, args []yurut.Value) ([]*tensor.Tensor, error) {
	var ilk *tensor.Tensor
	for _, a := range args {
		if d, ok := a.(*Deger); ok {
			ilk = d.t
			break
		}
	}
	out := make([]*tensor.Tensor, len(args))
	for i, a := range args {
		if d, ok := a.(*Deger); ok {
			if err := cihazDenetle(ad, ilk, d.t); err != nil {
				return nil, err
			}
			out[i] = d.t
			continue
		}
		t, err := tensoreCevirCihaz(ad, a, ilk)
		if err != nil {
			return nil, err
		}
		if t.DType() != ilk.DType() {
			t = t.Cast(ilk.DType())
		}
		out[i] = t
	}
	return out, nil
}
