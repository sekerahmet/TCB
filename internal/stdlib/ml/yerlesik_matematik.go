package ml

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Genişletilmiş matematik yerleşikleri: etkinleştirme (silu), ters trigonometri,
// hiperbolik, kararlı logaritma/üstel (birli ile: sayıda Ondalık, tensörde türevli
// tensör), taban/tavan/işaret/mutlak (sayı yolu internal/lang/interp'ta; tensörde tam sayılı
// işaret korunur), sınırla ve arktanjant2. Koşullu seçim yerlesik_secim.go'da, birikim ve sıralama
// yerlesik_siralama.go'dadır.

func silu64(x float64) float64 {
	if math.IsInf(x, -1) {
		return 0 // x·σ(x) → 0 (tensör yoluyla aynı)
	}
	return x / (1 + math.Exp(-x))
}

// matematigiGenisKaydet genişletilmiş matematik yerleşiklerini kaydeder.
func matematigiGenisKaydet(v *interp.VM) {
	birli(v, codes.T204, (*tensor.Tensor).Silu, silu64)
	birli(v, codes.T224, (*tensor.Tensor).Tan, math.Tan)
	birli(v, codes.T225, (*tensor.Tensor).Asin, math.Asin)
	birli(v, codes.T226, (*tensor.Tensor).Acos, math.Acos)
	birli(v, codes.T227, (*tensor.Tensor).Atan, math.Atan)
	birli(v, codes.T240, (*tensor.Tensor).Sinh, math.Sinh)
	birli(v, codes.T241, (*tensor.Tensor).Cosh, math.Cosh)
	birli(v, codes.T242, (*tensor.Tensor).Log1p, math.Log1p)
	birli(v, codes.T243, (*tensor.Tensor).Log2, math.Log2)
	birli(v, codes.T244, (*tensor.Tensor).Log10, math.Log10)
	birli(v, codes.T245, (*tensor.Tensor).Expm1, math.Expm1)
	yuvarlamaKaydet(v, codes.G406, (*tensor.Tensor).Floor)
	yuvarlamaKaydet(v, codes.G407, (*tensor.Tensor).Ceil)
	yuvarlamaKaydet(v, codes.G408, (*tensor.Tensor).Sign)
	yuvarlamaKaydet(v, codes.G403, (*tensor.Tensor).Abs) // mutlak: türevli (işaret(x), 0'da 0)
	sar(v, codes.G409, yeniYerlesik(sinirlaYerlesik))
	sar(v, codes.T228, yeniYerlesik(arktanjant2Yerlesik))
	sar(v, codes.T234, yeniYerlesik(kosulluYerlesik))         // yerlesik_secim.go
	sar(v, codes.T231, yeniYerlesik(birikimliToplamYerlesik)) // yerlesik_siralama.go
	sar(v, codes.T232, yeniYerlesik(enBuyukKYerlesik))
	sar(v, codes.T233, yeniYerlesik(siralamaIndeksiYerlesik))
}

// yuvarlamaKaydet taban, tavan, işaret ve mutlak'ı sarar: tensörde eleman bazlı (türevi tensör
// işleminindir: taban, tavan ve işarette 0, mutlak'ta işaret(x); tam sayılı işaretli tensörün
// sonucu da tam sayılıdır), sayıda çekirdek yerleşiği (Tam → Tam), listede yol gösteren hata.
func yuvarlamaKaydet(v *interp.VM, kd codes.Code, fn func(*tensor.Tensor) *tensor.Tensor) {
	ad := codes.Name(kd)
	sar(v, kd, func(eski interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if err := bind.CheckNamed(ad+"()", named); err != nil {
				return nil, err
			}
			if len(args) == 1 {
				if err := listeHatasi(ad, args[0], "tensörde ve sayıda", ad+"("+codes.Name(codes.T100)+"(...))"); err != nil {
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
func tekTensor(args []interp.Value) (*Deger, bool) {
	if len(args) != 1 {
		return nil, false
	}
	d, ok := args[0].(*Deger)
	return d, ok
}

// sinirSayisi sınırla'nın bir sınırını okur: sayı olmalı, NaN olamaz.
func sinirSayisi(ad string, adli bind.Named, v interp.Value) (float64, bool, error) {
	f, ok := interp.ToFloat(v)
	if !ok || f != f {
		return 0, false, fmt.Errorf("%s: %s bir sayı olmalıdır, %s verildi", ad, adli.TR, v.Inspect())
	}
	_, tam := v.(*interp.IntVal)
	return f, tam, nil
}

// sinirlaYerlesik sınırla(x, en_az: ..., en_fazla: ...).
func sinirlaYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.G409)
	kullanim := fmt.Sprintf("%s(x, %s: ..., %s: ...)", ad, adEnAz.TR, adEnFazla.TR)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s 1 konumlu parametre bekler, %d verildi", kullanim, len(args))
	}
	if err := bind.CheckNamed(ad+"()", named, adEnAz, adEnFazla); err != nil {
		return nil, err
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	tamSinir := true
	var verildi bool
	for _, s := range []struct {
		adli  bind.Named
		hedef *float64
	}{{adEnAz, &lo}, {adEnFazla, &hi}} {
		v, ok := bind.GetNamed(named, s.adli)
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
	case *interp.IntVal:
		if tamSinir {
			n := x.Val
			if f := float64(n); f < lo {
				n = int64(lo)
			} else if f > hi {
				n = int64(hi)
			}
			return interp.NewInt(n), nil
		}
		return &interp.FloatVal{Val: sinirla64(float64(x.Val), lo, hi)}, nil
	case *interp.FloatVal:
		return &interp.FloatVal{Val: sinirla64(x.Val, lo, hi)}, nil
	}
	if err := listeHatasi(ad, args[0], "tensörde ve sayıda", ad+"("+codes.Name(codes.T100)+"(...), "+adEnAz.TR+": 0)"); err != nil {
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
func arktanjant2Yerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T228)
	if err := bind.CheckNamed(ad+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(y, x) 2 parametre bekler, %d verildi", ad, len(args))
	}
	for _, a := range args {
		if err := listeHatasi(ad, a, "tensörde ve sayıda", ad+"("+codes.Name(codes.T100)+"(...), x)"); err != nil {
			return nil, err
		}
	}
	if !tensorMu(args) {
		y, ok1 := interp.ToFloat(args[0])
		x, ok2 := interp.ToFloat(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("%s: sayı ya da tensör bekleniyor, %s ve %s verildi", ad, tipAdi(args[0]), tipAdi(args[1]))
		}
		return &interp.FloatVal{Val: math.Atan2(y, x)}, nil
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
func tensorlereCevir(ad string, args []interp.Value) ([]*tensor.Tensor, error) {
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
