package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// İndirgemeler (toplam, ortalama, maks, min) ve en büyüğün indeksi
// ----------------------------------------------------------------------------

// reduction toplam/ortalama/maks/min'i kaydeder. Eksen verilmezse tüm elemanlar
// indirgenir ve 0 boyutlu (skaler) tensör döner; eksen verilirse o eksen kaldırılır.
// boyutu_koru: doğru indirgenen ekseni (eksen verilmezse bütün eksenleri) 1 uzunlukta
// korur: [B, T, D] → [B, T, 1].
func reduction(v *interp.VM, kd codes.Code, fn func(t *tensor.Tensor, axis int, whole, keep bool) *tensor.Tensor) {
	ad := codes.Name(kd)
	maxMin := kd == codes.G402 || kd == codes.G401 // maks, min
	wrap(v, kd, func(old interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) == 0 {
				if err := bind.CheckNamed(ad+"()", named, nameAxis, nameKeepDim); err != nil {
					return nil, err
				}
				return oldCall(ad, old, m, args, named)
			}
			if _, ok := args[0].(*TensorValue); !ok {
				return nonTensorReduce(kd, old, m, args, named)
			}
			if maxMin && len(args) >= 2 {
				// maks(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				limit := map[bool]string{true: nameMin.TR, false: nameMax.TR}[kd == codes.G402]
				return nil, fmt.Errorf("%s(x, %s): eksen için %s(x, %s: %s) yazın; eleman bazlı sınır için %s(x, %s: %s) kullanın",
					ad, args[1].Inspect(), ad, nameAxis.TR, args[1].Inspect(), codes.Name(codes.G409), limit, args[1].Inspect())
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, %s: ...) en fazla 2 parametre alır, %d verildi", ad, nameAxis.TR, len(args))
			}
			if err := bind.CheckNamed(ad+"()", named, nameAxis, nameKeepDim); err != nil {
				return nil, err
			}
			keep, err := namedBool(ad, named, nameKeepDim, false)
			if err != nil {
				return nil, err
			}
			t := args[0].(*TensorValue).t
			if !maxMin {
				// Karışık hassasiyet bloğunda toplam/ortalama float32'de yapılır.
				if t, err = highPrecision(m, t); err != nil {
					return nil, err
				}
			}
			e, given, err := axisArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			sh := t.Shape()
			if given {
				if e, err = parseAxis(ad, e, sh); err != nil {
					return nil, err
				}
				if maxMin && sh[e] == 0 {
					return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, shapeText(sh), e)
				}
			} else if maxMin && t.Numel() == 0 {
				return nil, fmt.Errorf("%s: boş tensörün (%s) en büyük/küçük elemanı yoktur", ad, shapeText(sh))
			}
			if kd == codes.G410 { // ortalama: boş kümenin ortalaması tanımsızdır
				if !given && t.Numel() == 0 {
					return nil, fmt.Errorf("%s: boş tensörün ortalaması tanımsız (şekil %s)", ad, shapeText(sh))
				}
				if given && sh[e] == 0 {
					exit := 1
					for i, d := range sh {
						if i != e {
							exit *= d
						}
					}
					if exit > 0 {
						return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş; boş kümenin ortalaması tanımsız", ad, shapeText(sh), e)
					}
				}
			}
			r, err := safe(func() *tensor.Tensor {
				r := fn(t, e, !given, keep)
				if keep && !given {
					ones := make([]int, len(sh))
					for i := range ones {
						ones[i] = 1
					}
					r = r.Reshape(ones...)
				}
				return r
			})
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}

// argMax verilen eksende en büyük elemanın indeksini verir: maks_indeks(x, eksen: -1).
// Sonuç her zaman tam sayı değerli bir tensördür (o eksen kaldırılmış şekilde; 1 boyutlu
// girdide 0 boyutlu) ve türev izlemez. Tensör tam sayılı olarak işaretlenir: .değer Tam
// (ya da Tam listesi) verir, böylece liste indeksi olarak doğrudan kullanılabilir; sonuç
// gömme(...) ve çapraz_entropi(...) indeksi olarak da verilebilir.
// Eşitlikte ilk en büyük eleman seçilir.
func argMax(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T230)
	if len(args) != 1 {
		if len(args) == 2 {
			return nil, fmt.Errorf("%s(x, %s: %s): eksen adlı verilir", ad, nameAxis.TR, args[1].Inspect())
		}
		return nil, fmt.Errorf("%s(x, %s: -1) 1 konumlu parametre bekler, %d verildi", ad, nameAxis.TR, len(args))
	}
	if err := bind.CheckNamed(ad+"()", named, nameAxis); err != nil {
		return nil, err
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		return nil, fmt.Errorf("%s: bir tensör bekler, %s verildi; liste için %s(%s(liste)) yazın", ad, typeName(args[0]), ad, codes.Name(codes.T100))
	}
	t := d.t
	e, given, err := axisArg(ad, args, named)
	if err != nil {
		return nil, err
	}
	if !given {
		e = -1
	}
	sh := t.Shape()
	if e, err = parseAxis(ad, e, sh); err != nil {
		return nil, err
	}
	if sh[e] == 0 {
		return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, shapeText(sh), e)
	}
	r, err := safe(func() *tensor.Tensor {
		idx := t.ArgMax(e)
		data := make([]float32, len(idx))
		for i, x := range idx {
			data[i] = float32(x)
		}
		output := append(append([]int{}, sh[:e]...), sh[e+1:]...)
		return tensor.NewOn(t.Device(), data, output...)
	})
	if err != nil {
		return nil, err
	}
	result := New(r)
	result.hasInts = true
	return result, nil
}

// nonTensorReduce ilk argümanı tensör olmayan indirgeme çağrılarını işler. Eski
// yerleşikler iç içe listeleri ve listedeki tensörleri sessizce atladığı için (toplam([[1, 2]])
// → 0) bu durumlar burada yakalanır: toplam/ortalama listedeki tensörleri hesap çizgesini
// koruyarak katlar, diğer durumlar açık hata verir.
func nonTensorReduce(kd codes.Code, old interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(kd)
	if err := bind.CheckNamed(ad+"()", named, nameAxis, nameKeepDim); err != nil {
		return nil, err
	}
	if _, ok := bind.GetNamed(named, nameKeepDim); ok {
		return nil, fmt.Errorf("%s: %s: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(%s(...), %s: 0, %s: %s)",
			ad, nameKeepDim.TR, typeName(args[0]), ad, codes.Name(codes.T100), nameAxis.TR, nameKeepDim.TR, codes.Name(codes.A040))
	}
	if _, ok := bind.GetNamed(named, nameAxis); ok {
		return nil, fmt.Errorf("%s: %s: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(%s(...), %s: 0)", ad, nameAxis.TR, typeName(args[0]), ad, codes.Name(codes.T100), nameAxis.TR)
	}
	if l, ok := args[0].(*interp.ListVal); ok && len(args) == 1 {
		var hasTensor bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *interp.ListVal:
				t := codes.Name(codes.T100)
				return nil, fmt.Errorf("%s: iç içe liste indirgenemez (eski liste davranışı yalnızca düz sayı listesi alır); tensöre çevirin: %s(%s(liste)) ya da eksen için %s(%s(liste), %s: 0)", ad, ad, t, ad, t, nameAxis.TR)
			case *TensorValue:
				hasTensor = true
			}
		}
		if hasTensor {
			if kd != codes.G400 && kd != codes.G410 { // toplam, ortalama
				return nil, fmt.Errorf("%s: tensör listesinin en %s elemanı tanımsız; tek bir tensör verin: %s(x) ya da %s(x, %s: 0)", ad, map[bool]string{true: "büyük", false: "küçük"}[kd == codes.G402], ad, ad, nameAxis.TR)
			}
			return sumTensorList(ad, kd == codes.G410, l.Elements)
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*TensorValue); ok {
			return nil, fmt.Errorf("%s(x, %s: ...) ilk parametre olarak tensör bekler, %s verildi", ad, nameAxis.TR, typeName(args[0]))
		}
	}
	return oldCall(ad, old, m, args, named)
}

// sumTensorList sayı ve tensörlerden oluşan bir listeyi yayınlamalı toplamla katlar;
// ortalama için eleman sayısına böler. Sonuç türev izlenen girdilere bağlı kalır.
func sumTensorList(ad string, mean bool, el []interp.Value) (interp.Value, error) {
	var acc *tensor.Tensor
	fixed := 0.0
	for i, e := range el {
		switch x := e.(type) {
		case *TensorValue:
			if acc == nil {
				acc = x.t
				continue
			}
			if err := checkBroadcast("+", acc, x.t); err != nil {
				return nil, fmt.Errorf("%s: %d. eleman eklenemiyor: %v", ad, i+1, err)
			}
			a := acc
			r, err := safe(func() *tensor.Tensor { return tensor.Add(a, x.t) })
			if err != nil {
				return nil, err
			}
			acc = r
		default:
			f, ok := interp.ToFloat(e)
			if !ok {
				return nil, fmt.Errorf("%s: listenin %d. elemanı sayı ya da tensör olmalıdır, %s verildi", ad, i+1, typeName(e))
			}
			fixed += f
		}
	}
	r, err := safe(func() *tensor.Tensor {
		out := acc
		if fixed != 0 {
			out = out.AddScalar(float32(fixed))
		}
		if mean {
			out = tensor.Div(out, tensor.Scalar(float32(len(el))).To(out.Device()))
		}
		return out
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
