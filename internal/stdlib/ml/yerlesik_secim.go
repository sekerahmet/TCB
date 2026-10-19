package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// koşullu(koşul, a, b): eleman bazlı seçim (bkz. tensor.Where).

// kosulluYerlesik koşullu(koşul, a, b). Hiçbiri tensör değilse koşul Mantık ya da sayı
// olmalıdır ve a ya da b olduğu gibi döner. Değilse sayılar ve Mantık tensöre çevrilir
// (a ile b'deki sayılar ilk tensör işlenenin türünü alır), üç şekil yayınlanır.
func kosulluYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T234)
	if err := bind.AdliDenetle(ad+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 3 {
		return nil, fmt.Errorf("%s(koşul, a, b) 3 parametre bekler, %d verildi", ad, len(args))
	}
	for _, a := range args {
		if err := listeHatasi(ad, a, "tensörde, sayıda ve "+codes.Name(codes.A203)+" değerinde", ad+"("+codes.Name(codes.T100)+"(...), a, b)"); err != nil {
			return nil, err
		}
	}
	if !tensorMu(args) {
		d, err := kosulDegeri(ad, args[0])
		if err != nil {
			return nil, err
		}
		for _, a := range args[1:] {
			if _, ok := interp.ToFloat(a); !ok {
				return nil, fmt.Errorf("%s: a ve b sayı ya da tensör olmalıdır, %s verildi", ad, tipAdi(a))
			}
		}
		if d {
			return args[1], nil
		}
		return args[2], nil
	}
	ts, err := kosulTensorleri(ad, args)
	if err != nil {
		return nil, err
	}
	if err := ucluYayinDenetle(ad, ts); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.Where(ts[0], ts[1], ts[2]) })
	if err != nil {
		return nil, err
	}
	s := Yeni(r)
	s.tamSayili = tamSayiliMi(args[1]) && tamSayiliMi(args[2])
	return s, nil
}

// kosulDegeri tensör olmayan koşulun doğruluğu: Mantık ya da sayı (0 dışı doğru).
func kosulDegeri(ad string, v interp.Value) (bool, error) {
	if b, ok := v.(*interp.BoolVal); ok {
		return b.Val, nil
	}
	if f, ok := interp.ToFloat(v); ok {
		return f != 0, nil
	}
	return false, fmt.Errorf("%s: koşul %s, sayı ya da tensör olmalıdır, %s verildi", ad, codes.Name(codes.A203), tipAdi(v))
}

// tamSayiliMi değer tam sayılı bir tensör ya da Tam mı (sonucun tam sayılı kalması için).
func tamSayiliMi(v interp.Value) bool {
	switch x := v.(type) {
	case *Deger:
		return x.tamSayili
	case *interp.IntVal:
		return true
	}
	return false
}

// kosulTensorleri koşul, a ve b'yi aynı cihazda tensöre çevirir. Mantık koşul 1 ya da 0
// olur; a ve b'deki sayılar ilk tensör işlenenin (yoksa float32) türünü alır.
func kosulTensorleri(ad string, args []interp.Value) ([]*tensor.Tensor, error) {
	kosul := args[0]
	if b, ok := kosul.(*interp.BoolVal); ok {
		kosul = interp.NewInt(0)
		if b.Val {
			kosul = interp.NewInt(1)
		}
	} else if _, ok := kosul.(*Deger); !ok {
		if _, sayi := interp.ToFloat(kosul); !sayi {
			return nil, fmt.Errorf("%s: koşul %s, sayı ya da tensör olmalıdır, %s verildi", ad, codes.Name(codes.A203), tipAdi(kosul))
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*Deger); ok {
			continue
		}
		if _, ok := interp.ToFloat(a); !ok {
			return nil, fmt.Errorf("%s: a ve b sayı ya da tensör olmalıdır, %s verildi", ad, tipAdi(a))
		}
	}
	degerler := []interp.Value{args[1], args[2], kosul}
	if !tensorMu(degerler[:2]) {
		// a ve b sayı: koşul tensördür; a ve b onun cihazında float32 tensörler olur.
		c := kosul.(*Deger).t
		ts := []*tensor.Tensor{c, nil, nil}
		for i, a := range args[1:] {
			t, err := tensoreCevirCihaz(ad, a, c)
			if err != nil {
				return nil, err
			}
			ts[i+1] = t
		}
		return ts, nil
	}
	ts, err := tensorlereCevir(ad, degerler)
	if err != nil {
		return nil, err
	}
	return []*tensor.Tensor{ts[2], ts[0], ts[1]}, nil
}

// ucluYayinDenetle koşul, a ve b şekillerinin birlikte yayınlanabilirliğini denetler.
func ucluYayinDenetle(ad string, ts []*tensor.Tensor) error {
	nd := 0
	for _, t := range ts {
		nd = max(nd, t.Dim())
	}
	for i := 1; i <= nd; i++ {
		boy := 1
		for _, t := range ts {
			d := sondanEksen(t, i)
			if d != 1 && boy != 1 && d != boy {
				return fmt.Errorf("%s: koşul %s, a %s ve b %s şekilleri yayınlanamaz; sondan %d. eksende uzunluklar uyuşmuyor (eksenler sağdan hizalanır, eşit ya da 1 olmalı)",
					ad, sekilMetni(ts[0].Shape()), sekilMetni(ts[1].Shape()), sekilMetni(ts[2].Shape()), i)
			}
			if d != 1 {
				boy = d
			}
		}
	}
	return nil
}
