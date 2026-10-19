package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Eksen boyunca birikim ve sıralama yerleşikleri: birikimli_toplam, en_büyük_k,
// sıralama_indeksi. Üçü de yalnızca tensörde çalışır; eksen yalnızca adlı verilir
// (varsayılan son eksen).

// eksenliTensor ilk konumlu argümanın tensör olduğunu, konumlu argüman sayısını (konumlu
// sayısı n) ve eksen adlı argümanını denetleyip tensörü ve çözülmüş ekseni verir.
func eksenliTensor(ad, kullanim string, args []interp.Value, n int, named map[string]interp.Value) (*Deger, int, error) {
	if len(args) != n {
		if len(args) == n+1 {
			return nil, 0, fmt.Errorf("%s: eksen adlı verilir (%s: %s)", kullanim, adEksen.TR, args[n].Inspect())
		}
		return nil, 0, fmt.Errorf("%s %d konumlu parametre bekler, %d verildi", kullanim, n, len(args))
	}
	if err := bind.AdliDenetle(ad+"()", named, adEksen); err != nil {
		return nil, 0, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		if err := listeHatasi(ad, args[0], "tensörde", ad+"("+codes.Name(codes.T100)+"(...))"); err != nil {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("%s: bir tensör bekler, %s verildi", ad, tipAdi(args[0]))
	}
	e := -1
	if v, ok := bind.AdliAl(named, adEksen); ok {
		var tam bool
		if e, tam = tamSayi(v); !tam {
			return nil, 0, fmt.Errorf("%s: eksen tam sayı olmalıdır, %s verildi", ad, v.Inspect())
		}
	}
	e, err := eksenCoz(ad, e, d.t.Shape())
	if err != nil {
		return nil, 0, err
	}
	return d, e, nil
}

// birikimliToplamYerlesik birikimli_toplam(t, eksen: -1): şekil korunur, türev izler; tam
// sayılı tensörün sonucu da tam sayılıdır.
func birikimliToplamYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T231)
	d, e, err := eksenliTensor(ad, fmt.Sprintf("%s(t, %s: -1)", ad, adEksen.TR), args, 1, named)
	if err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.CumSum(e) })
	if err != nil {
		return nil, err
	}
	s := Yeni(r)
	s.tamSayili = d.tamSayili
	return s, nil
}

// enBuyukKYerlesik en_büyük_k(t, k, eksen: -1) → [değerler, indeksler].
func enBuyukKYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T232)
	d, e, err := eksenliTensor(ad, fmt.Sprintf("%s(t, k, %s: -1)", ad, adEksen.TR), args, 2, named)
	if err != nil {
		return nil, err
	}
	k, ok := args[1].(*interp.IntVal)
	L := d.t.Shape()[e]
	if !ok {
		return nil, fmt.Errorf("%s: k bir %s olmalıdır, %s verildi", ad, codes.Name(codes.A200), tipAdi(args[1]))
	}
	if k.Val < 0 || k.Val > int64(L) {
		return nil, fmt.Errorf("%s: k = %d, ama %s şeklinde %d. eksenin uzunluğu %d; k 0 ile %d arasında olmalı",
			ad, k.Val, sekilMetni(d.t.Shape()), e, L, L)
	}
	var vals, idx *tensor.Tensor
	if _, err := guvenli(func() *tensor.Tensor { vals, idx = d.t.TopK(int(k.Val), e); return vals }); err != nil {
		return nil, err
	}
	v := Yeni(vals)
	v.tamSayili = d.tamSayili
	return &interp.ListVal{Elements: []interp.Value{v, YeniTam(idx)}}, nil
}

// siralamaIndeksiYerlesik sıralama_indeksi(t, eksen: -1): kararlı, küçükten büyüğe.
func siralamaIndeksiYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T233)
	d, e, err := eksenliTensor(ad, fmt.Sprintf("%s(t, %s: -1)", ad, adEksen.TR), args, 1, named)
	if err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.ArgSort(e) })
	if err != nil {
		return nil, err
	}
	return YeniTam(r), nil
}
