package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bileşik atama tensörü yerinde değiştirir (dil tanımı 15.6d): t op= u'nun sonucu hedefin
// şeklinde ve hassasiyetinde t'nin deposuna yazılır; tensörü paylaşan her ad değişikliği görür ve
// deponun sürümü artar (15.3: geri yayılım için saklanmış tensörün değişmesi TR_backward'da
// yakalanır). Yayın yalnız sağ taraftadır. Türevin yazma kuralları checkWrite'tadır.

// inPlaceOps yerinde yapılan bileşik atamaların işleçleri; öbürleri (//, %, bit işleçleri)
// yorumlayıcının genel yoluna düşer (tensörde hatadır).
var inPlaceOps = map[string]bool{"+": true, "-": true, "*": true, "/": true, "**": true}

// InPlaceOp t op= u bileşik atamasıdır (interp.InPlaceOperator).
func (d *TensorValue) InPlaceOp(op string, rhs interp.Value) (bool, error) {
	if !inPlaceOps[op] {
		return false, nil
	}
	if err := checkWrite(d.t, valueTensor(rhs), true); err != nil {
		return true, err
	}
	return true, applyInPlace(d.t, op, rhs)
}

// AccumulateIndex t[i] op= u ve t[a..b] op= u bileşik atamasıdır (interp.IndexAccumulator):
// görünüm üstünde yerinde değişir. Türev kuralı indekslenen tensöre bakar (yaprağın görünümü
// yapraktır).
func (d *TensorValue) AccumulateIndex(op string, idx interp.Value, rhs interp.Value) (bool, error) {
	if !inPlaceOps[op] {
		return false, nil
	}
	if err := checkWrite(d.t, valueTensor(rhs), false); err != nil {
		return true, err
	}
	var view *TensorValue
	var err error
	tensor.NoGrad(func() {
		var v interp.Value
		if v, err = d.Index(idx); err == nil {
			view = v.(*TensorValue)
		}
	})
	if err != nil {
		return true, err
	}
	defer view.Release() // görünüm başlığı bu işlemin ara değeridir
	return true, applyInPlace(view.t, op, rhs)
}

// applyInPlace dst op rhs'yi türevsiz hesaplar ve dst'nin hassasiyetinde dst'ye yazar.
func applyInPlace(dst *tensor.Tensor, op string, rhs interp.Value) error {
	if err := checkInPlaceDType(dst.DType()); err != nil {
		return err
	}
	var r *tensor.Tensor
	var err error
	tensor.NoGrad(func() {
		var v interp.Value
		if v, _, err = (&TensorValue{t: dst}).operator(nil, op, rhs, false); err == nil {
			r = v.(*TensorValue).t
		}
	})
	if err != nil {
		return err
	}
	if rs, ds := r.Shape(), dst.Shape(); tensor.ShapeText(rs) != tensor.ShapeText(ds) {
		return messages.New(messages.H6958, op, tensor.ShapeText(rs), tensor.ShapeText(ds))
	}
	temps := []*tensor.Tensor{r}
	// Ara sonuçlar bu atamanındır (döngüde bellek birikmesin); hedef ve sağ taraf bırakılmaz.
	defer func() {
		for _, t := range temps {
			if t != dst && t != valueTensor(rhs) {
				t.Drop()
			}
		}
	}()
	if r.DType() != dst.DType() && (r.DType().IsTyped() || dst.DType().IsTyped()) {
		// Cinsli hedef: sonuç hedefin cinsinde kalmalı, hassasiyetine sığmalı (15.6d).
		if r.Elem() != dst.Elem() {
			return messages.New(messages.H6966, op+"=", dst.DType().Name(), r.DType().Name(), "compound")
		}
		if r, err = convertKind(op+"=", r, dst.DType()); err != nil {
			return err
		}
		temps = append(temps, r)
	}
	_, err = safe(func() bool {
		if r.DType() != dst.DType() {
			tensor.NoGrad(func() { r = r.Cast(dst.DType()) })
			temps = append(temps, r)
		}
		dst.CopyFrom(r)
		return true
	})
	return err
}

// checkInPlaceDType bileşik atamanın hedefi hesap ondalığı ya da cinsli (TR_Int, TR_Bool,
// float64) bir tensördür; nicemlenmiş tensöre (15.54) yerinde hesap yazılmaz.
func checkInPlaceDType(dt tensor.DType) error {
	switch dt {
	case tensor.Float32, tensor.BFloat16, tensor.Float16:
		return nil
	}
	if dt.IsTyped() {
		return nil
	}
	return messages.New(messages.H6957, dt.String())
}

var (
	_ interp.InPlaceOperator  = (*TensorValue)(nil)
	_ interp.IndexAccumulator = (*TensorValue)(nil)
)
