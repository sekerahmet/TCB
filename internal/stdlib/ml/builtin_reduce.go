package ml

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// İndirgemeler (toplam, ortalama, max, min) ve en büyüğün indeksi
// ----------------------------------------------------------------------------

// reduction toplam/ortalama/max/min'i kaydeder. Eksen verilmezse tüm elemanlar
// indirgenir ve 0 boyutlu (skaler) tensör döner; eksen verilirse o eksen kaldırılır.
// boyutu_koru: doğru indirgenen ekseni (eksen verilmezse bütün eksenleri) 1 uzunlukta
// korur: [B, T, D] → [B, T, 1].
func reduction(v *interp.VM, kd codes.Code, fn func(t *tensor.Tensor, axis int, whole, keep bool) *tensor.Tensor) {
	ad := codes.NameRef(kd)
	maxMin := kd == codes.FnMax || kd == codes.FnMin // max, min
	wrap(v, kd, func(old interp.BuiltinFunc) interp.BuiltinFunc {
		return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) == 0 {
				if err := bind.CheckNamed(ad.Called(), named, nameAxis, nameKeepDim); err != nil {
					return nil, err
				}
				return oldCall(ad, old, m, args, named)
			}
			if _, ok := args[0].(*TensorValue); !ok {
				return nonTensorReduce(kd, old, m, args, named)
			}
			if maxMin && len(args) >= 2 {
				// max(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				limit := map[bool]bind.Named{true: nameMin, false: nameMax}[kd == codes.FnMax]
				return nil, messages.New(messages.H6205, ad, interp.Shown(args[1]), nameAxis, codes.NameRef(codes.FnClamp), limit)
			}
			if len(args) > 2 {
				return nil, messages.New(messages.H6206, ad, nameAxis, len(args))
			}
			if err := bind.CheckNamed(ad.Called(), named, nameAxis, nameKeepDim); err != nil {
				return nil, err
			}
			keep, err := bind.OptionalBool(ad, named, nameKeepDim, false)
			if err != nil {
				return nil, err
			}
			t := args[0].(*TensorValue).t
			if t.DType().IsTyped() {
				return typedReduceCall(kd, ad, t, args, named, keep)
			}
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
					return nil, messages.New(messages.H6209, ad, tensor.ShapeText(sh), e)
				}
			} else if maxMin && t.Numel() == 0 {
				return nil, messages.New(messages.H6211, ad, tensor.ShapeText(sh))
			}
			if kd == codes.FnMean { // ortalama: boş kümenin ortalaması tanımsızdır
				if !given && t.Numel() == 0 {
					return nil, messages.New(messages.H6212, ad, tensor.ShapeText(sh))
				}
				if given && sh[e] == 0 {
					exit := 1
					for i, d := range sh {
						if i != e {
							exit *= d
						}
					}
					if exit > 0 {
						return nil, messages.New(messages.H6213, ad, tensor.ShapeText(sh), e)
					}
				}
			}
			// Kilit bırakılarak (released): toplam, ortalama ve max tek çekirdektir; min'de
			// girdiyi yalnız ilk çekirdek (-x) okur, sonrakiler ara sonucu okur.
			r, err := safe(released(m, func() *tensor.Tensor {
				r := fn(t, e, !given, keep)
				if keep && !given {
					ones := make([]int, len(sh))
					for i := range ones {
						ones[i] = 1
					}
					r = r.Reshape(ones...)
				}
				return r
			}))
			if err != nil {
				return nil, err
			}
			return New(r), nil
		}
	})
}

// typedReduceCall cinsli tensörde indirgemenin eksenini çözer ve typedReduce'u çağırır.
func typedReduceCall(kd codes.Code, ad any, t *tensor.Tensor, args []interp.Value, named map[string]interp.Value, keep bool) (interp.Value, error) {
	e, given, err := axisArg(ad, args, named)
	if err != nil {
		return nil, err
	}
	if given {
		if e, err = parseAxis(ad, e, t.Shape()); err != nil {
			return nil, err
		}
	}
	return typedReduce(kd, ad, t, e, given, keep)
}

// typedReduce indirgemelerin cinsli tensör yolu (15.18'in tablosu): TR_Int ve TR_Bool toplamı
// "int64"te kesin (sonuç sığmalı), ortalaması "float32"ye bir kez yuvarlanır; en büyük ve en
// küçük TR_Int'te aynı hassasiyet, TR_Bool'da tanımsız. float64 float64'te biriktirilir.
func typedReduce(kd codes.Code, ad any, t *tensor.Tensor, e int, given, keep bool) (interp.Value, error) {
	sh := t.Shape()
	axis := -1
	if given {
		axis = e
	}
	if (kd == codes.FnMax || kd == codes.FnMin) && (given && sh[e] == 0 || !given && t.Numel() == 0) {
		return nil, messages.New(messages.H6211, ad, tensor.ShapeText(sh))
	}
	isInt := t.Elem() != tensor.ElemFloat
	var f *tensor.ElemFault
	r, err := safe(func() (r *tensor.Tensor) {
		switch {
		case kd == codes.FnSum:
			r, f = tensor.ReduceTyped(t, tensor.ReduceSum, axis)
		case (kd == codes.FnMax || kd == codes.FnMin) && t.Elem() != tensor.ElemBool:
			r, f = tensor.ReduceTyped(t, map[bool]tensor.ReduceKind{true: tensor.ReduceMax, false: tensor.ReduceMin}[kd == codes.FnMax], axis)
		case kd == codes.FnMean && isInt:
			r = tensor.MeanIntTyped(t, axis)
		case kd == codes.FnMean:
			s, _ := tensor.ReduceTyped(t, tensor.ReduceSum, axis)
			n := float64(t.Numel())
			if given {
				n = float64(sh[e])
			}
			r, _ = tensor.Arith(s, tensor.Float64sOn(t.Device(), []float64{n}), tensor.ArithDiv, tensor.Float64)
		case isInt:
			panic(messages.New(messages.H6967, ad, t.DType().Name(), "none"))
		default:
			panic(tensor.PrecisionError(t.DType(), t.Device()))
		}
		if keep && given {
			r = r.Unsqueeze(e)
		} else if keep {
			r = r.Reshape(slices.Repeat([]int{1}, len(sh))...)
		}
		return r
	})
	if err != nil {
		return nil, panicReasonError(err)
	}
	return typedResult(ad, "+", r, f)
}

// panicReasonError çekirdeğin panikle verdiği ileti hatasını (H6006'nın içindeki) kendisi yapar.
func panicReasonError(err error) error {
	if e, ok := panicReason(err).(*messages.Error); ok {
		return e
	}
	return err
}

// argMax verilen eksende en büyük elemanın indeksini verir: argmax(x, eksen: -1). Eksen
// verilmezse bütün öğeler üstünde çalışır ve satır sırasıyla düzleştirilmiş indeksi 0 boyutlu
// tensörde verir (15.19). Sonuç her zaman TR_Int ("int64") tensörüdür (o eksen kaldırılmış
// şekilde) ve türev izlemez: .değer Tam (ya da Tam listesi) verir, böylece liste indeksi
// olarak doğrudan kullanılabilir; sonuç gömme(...) ve
// çapraz_entropi(...) indeksi olarak da verilebilir.
// Eşitlikte ilk en büyük eleman seçilir; −0.0 0.0'dan küçüktür, NaN varsa ilk NaN (15.22b).
func argMax(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.NameRef(codes.FnArgmax)
	if len(args) != 1 {
		if len(args) == 2 {
			return nil, messages.New(messages.H6215, ad, nameAxis, interp.Shown(args[1]))
		}
		return nil, messages.New(messages.H6216, ad, nameAxis, len(args))
	}
	if err := bind.CheckNamed(ad.Called(), named, nameAxis); err != nil {
		return nil, err
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H6217, ad, typeName(args[0]), codes.NameRef(codes.TypeTensor))
	}
	t := d.t
	e, given, err := axisArg(ad, args, named)
	if err != nil {
		return nil, err
	}
	sh := t.Shape()
	if !given {
		if t.Numel() == 0 {
			return nil, messages.New(messages.H6211, ad, tensor.ShapeText(sh))
		}
		// Bütün öğeler: düzleştirilmiş tensörün tek ekseni, sonuç 0 boyutlu.
		tensor.NoGrad(func() { t = t.Reshape(t.Numel()) })
		e, sh = 0, t.Shape()
	} else if e, err = parseAxis(ad, e, sh); err != nil {
		return nil, err
	}
	if sh[e] == 0 {
		return nil, messages.New(messages.H6209, ad, tensor.ShapeText(sh), e)
	}
	r, err := safe(func() *tensor.Tensor {
		var idx []int
		if t.DType().IsTyped() {
			idx = tensor.ArgMaxTyped(t, e)
		} else {
			idx = t.ArgMax(e)
		}
		data := make([]int64, len(idx))
		for i, x := range idx {
			data[i] = int64(x)
		}
		output := append(append([]int{}, sh[:e]...), sh[e+1:]...)
		return tensor.Int64sOn(t.Device(), tensor.Int64, data, output...)
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// nonTensorReduce ilk argümanı tensör olmayan indirgeme çağrılarını işler. Eski
// yerleşikler iç içe listeleri ve listedeki tensörleri sessizce atladığı için (toplam([[1, 2]])
// → 0) bu durumlar burada yakalanır: toplam/ortalama listedeki tensörleri hesap çizgesini
// koruyarak katlar, diğer durumlar açık hata verir.
func nonTensorReduce(kd codes.Code, old interp.BuiltinFunc, m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.NameRef(kd)
	if err := bind.CheckNamed(ad.Called(), named, nameAxis, nameKeepDim); err != nil {
		return nil, err
	}
	if _, ok := bind.GetNamed(named, nameKeepDim); ok {
		return nil, messages.New(messages.H6218, ad, nameKeepDim, typeName(args[0]), codes.NameRef(codes.TypeTensor), nameAxis, codes.NameRef(codes.KwTrue))
	}
	if _, ok := bind.GetNamed(named, nameAxis); ok {
		return nil, messages.New(messages.H6219, ad, nameAxis, typeName(args[0]), codes.NameRef(codes.TypeTensor))
	}
	if l, ok := args[0].(*interp.ListVal); ok && len(args) == 1 {
		if kd == codes.FnSum && len(l.Elements) == 0 {
			// Boş listenin öğe tipi çağrıda denetleyiciden gelir (interp.knownListElem).
			if l.TensorList() {
				return nil, messages.New(messages.H2720, ad.Called()) // boş tensör listesinin şekli bilinmez (15.18)
			}
		}
		var hasTensor bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *interp.ListVal:
				t := codes.NameRef(codes.TypeTensor)
				return nil, messages.New(messages.H6220, ad, t, nameAxis)
			case *TensorValue:
				hasTensor = true
			}
		}
		if hasTensor {
			if kd != codes.FnSum && kd != codes.FnMean { // toplam, ortalama
				if kd == codes.FnMax {
					return nil, messages.New(messages.H6224, ad, nameAxis)
				}
				return nil, messages.New(messages.H6225, ad, nameAxis)
			}
			return sumTensorList(m, ad, kd == codes.FnMean, l.Elements)
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*TensorValue); ok {
			return nil, messages.New(messages.H6221, ad, nameAxis, typeName(args[0]))
		}
	}
	return oldCall(ad, old, m, args, named)
}

// sumTensorList sayı ve tensörlerden oluşan bir listeyi yayınlamalı toplamla katlar;
// ortalama için eleman sayısına böler. Sonuç türev izlenen girdilere bağlı kalır.
func sumTensorList(m *interp.VM, ad any, mean bool, el []interp.Value) (interp.Value, error) {
	var acc *tensor.Tensor
	fixed := 0.0
	for i, e := range el {
		switch x := e.(type) {
		case *TensorValue:
			if acc == nil {
				acc = x.t
				continue
			}
			if err := checkBroadcast(m, "+", acc, x.t); err != nil {
				return nil, messages.New(messages.H6222, ad, i+1, err)
			}
			if err := checkDevice(ad, acc, x.t); err != nil {
				return nil, err
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
				return nil, messages.New(messages.H6223, ad, i+1, typeName(e))
			}
			fixed += f
		}
	}
	r, err := safe(func() *tensor.Tensor {
		// Toplam +0'dan başlar (Python sum gibi): yalnızca −0'lardan oluşan toplam ve ortalama +0'dır
		// (15.50). Sayı toplamı yoksa da +0 eklenir; başka her değeri olduğu gibi bırakır.
		out := acc.AddScalar(float32(fixed))
		if mean {
			out = out.DivScalar(float32(len(el)))
		}
		return out
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
