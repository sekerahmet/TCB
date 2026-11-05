package tensor

import "math"

// Öğe öğe en büyük ve en küçük (15.16a). Var olan cihaz işlemlerinin birleşimidir; arka uca yeni
// çekirdek eklemez, CPU ve GPU aynı yoldan geçer. Seçim sıralama sırasıyladır (15.22b): NaN
// yayılır, ±0'da −0.0 küçük sayılır; ±0 tensörde 1/x'in işaretiyle ayrılır. Türev tek düğümdür
// (CustomOp): seçilen tarafa gider, tam eşitlikte yarı yarıya, NaN olan konumda iki tarafa da NaN.

// Maximum a ile b'nin (yayınlamalı) öğe öğe en büyüğüdür; cinsleri aynıdır.
func Maximum(a, b *Tensor) *Tensor { return extreme("maximum", a, b, CmpGt, CmpGe) }

// Minimum a ile b'nin (yayınlamalı) öğe öğe en küçüğüdür; cinsleri aynıdır.
func Minimum(a, b *Tensor) *Tensor { return extreme("minimum", a, b, CmpLt, CmpLe) }

// extreme iki yönün ortak yoludur: strict sıkı, loose eşitliği de alan karşılaştırmadır.
func extreme(op string, a, b *Tensor, strict, loose CmpOp) *Tensor {
	if a.Elem() != ElemFloat {
		dt := MergeInt(a.dtyp, b.dtyp) // iki hassasiyet birleşir (15.6c)
		x, _ := Convert(a, dt)
		y, _ := Convert(b, dt)
		sel := CompareTyped(x, y, loose)
		defer sel.Drop()
		return WhereTyped(sel, x, y)
	}
	if a.dtyp == Float64 || b.dtyp == Float64 {
		return extreme64(a, b, strict, loose) // float64 cinsli yoldadır, türev izlemez (15.48)
	}
	var out *Tensor
	NoGrad(func() {
		sel := firstWins(a, b, strict, loose)
		out = selectBy(sel, a, b)
		sel.Drop()
	})
	if !GradEnabled() || !a.RequiresGrad() && !b.RequiresGrad() {
		return out
	}
	// Geri hesap başlık yakalamaz: yorumlayıcı geçici işleneni bu çağrıdan sonra bırakır. Kendi
	// başlıkları türev sorusunu (RequiresGrad) da taşır; verisi kaydedilen depolamadır.
	x, y := a.Detach(), b.Detach()
	wantA, wantB := a.RequiresGrad(), b.RequiresGrad()
	return CustomOp(op, []*Tensor{a, b}, []*Tensor{out}, []*Tensor{x, y}, func(_ any, grads []*Tensor) []*Tensor {
		defer dropAll(x, y)
		return extremeGrads(x, y, grads[0], wantA, wantB, strict, loose)
	})[0]
}

// extreme64 float64 işlenenli yoldur: iki taraf float64'e açılır, cinsli seçimle seçilir.
func extreme64(a, b *Tensor, strict, loose CmpOp) *Tensor {
	x, _ := Convert(a, Float64)
	y, _ := Convert(b, Float64)
	sel := firstWins(x, y, strict, loose)
	defer sel.Drop()
	return WhereTyped(sel, x, y)
}

// reciprocal 1/t'dir: ±0'ın işaretini büyüklüğe çevirir.
func reciprocal(t *Tensor) *Tensor {
	if t.dtyp != Float64 {
		return t.RDivScalar(1)
	}
	one := Float64sOn(t.Device(), []float64{1})
	defer one.Drop()
	r, _ := Arith(one, t, ArithDiv, Float64)
	return r
}

// firstWins a'nın seçildiği konumların mantık maskesidir: a kesin üstünse, a NaN ise (b NaN'sa a
// seçilmez, sonuç yine NaN'dır) ya da değerler eşitken işaret a'yı seçiyorsa.
func firstWins(a, b *Tensor, strict, loose CmpOp) *Tensor {
	over := CompareTyped(a, b, strict)
	nan := CompareTyped(a, a, CmpNe)
	eq := CompareTyped(a, b, CmpEq)
	ia, ib := reciprocal(a), reciprocal(b)
	sign := CompareTyped(ia, ib, loose)
	tie, _ := Logic(eq, sign, LogicAnd, Bool)
	either, _ := Logic(over, nan, LogicOr, Bool)
	r, _ := Logic(either, tie, LogicOr, Bool)
	dropAll(over, nan, eq, ia, ib, sign, tie, either)
	return r
}

// extremeGrads çıktının gradyanını a ile b'ye paylaştırır; yayınlanan eksenler toplanır.
func extremeGrads(a, b, g *Tensor, wantA, wantB bool, strict, loose CmpOp) []*Tensor {
	first := firstWins(a, b, strict, loose)
	second := Not(first)
	eq := CompareTyped(a, b, CmpEq)
	ia, ib := reciprocal(a), reciprocal(b)
	sameSign := CompareTyped(ia, ib, CmpEq)
	tie, _ := Logic(eq, sameSign, LogicAnd, Bool)
	na, nb := CompareTyped(a, a, CmpNe), CompareTyped(b, b, CmpNe)
	nan, _ := Logic(na, nb, LogicOr, Bool)
	half := g.MulScalar(0.5)
	share := func(want bool, t, mine *Tensor) *Tensor {
		if !want {
			return nil
		}
		zero := zerosOf(isGrad, g.be, nil, g.dtyp)
		lost := FullOn(g.Device(), float32(math.NaN()))
		w := selectBy(mine, g, zero)
		h := selectBy(tie, half, w)
		n := selectBy(nan, lost, h).Cast(g.dtyp)
		r := sumToShape(n, t.shape)
		dropAll(zero, lost, w, h)
		if r != n {
			n.Drop()
		}
		return r
	}
	res := []*Tensor{share(wantA, a, first), share(wantB, b, second)}
	dropAll(first, second, eq, ia, ib, sameSign, tie, na, nb, nan, half)
	return res
}

// selectBy mantık maskesiyle seçer. GPU'nun seçim çekirdeği koşulu float32 okur.
func selectBy(c, a, b *Tensor) *Tensor {
	if c.Device() == CPU {
		return Where(c, a, b)
	}
	f, _ := Convert(c, Float32)
	defer f.Drop()
	return Where(f, a, b)
}

// sumToShape yayınlanmış t'yi shape'e toplar; şekil aynıysa t'nin kendisini verir.
func sumToShape(t *Tensor, shape []int) *Tensor {
	r := t
	for r.Dim() > len(shape) {
		s := r.Sum(0, false)
		if r != t {
			r.Drop()
		}
		r = s
	}
	for i, n := range shape {
		if n == 1 && r.shape[i] != 1 {
			s := r.Sum(i, true)
			if r != t {
				r.Drop()
			}
			r = s
		}
	}
	return r
}

// dropAll ara tensörleri bırakır.
func dropAll(ts ...*Tensor) {
	for _, t := range ts {
		t.Drop()
	}
}
