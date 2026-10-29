package interp

import (
	"fmt"
	"math"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Demetin işleçleri (3.6a): '==' öğe öğedir (4.5), '<' ve öbür üçü sözlük sırasıdır (4.4), sıralama
// 13.10'un sırasıdır, sözlük anahtarı öğelerin anahtarlarıdır (3.7). Demetin '+'sı, '*'ı, uzunluğu
// ve dolaşılması dil tanımında yoktur: hata verir (H2469, bilinmeyen üye).

// tuplesEqual demetin '=='sidir: aynı sayıda öğe, öğe öğe valuesEqual. Demet yalnız demete eşittir.
func tuplesEqual(x *TupleVal, b Value, nestDepth int) bool {
	y, ok := b.(*TupleVal)
	if !ok || len(x.Elements) != len(y.Elements) {
		return false
	}
	if x == y {
		return true
	}
	if nestDepth >= equalityDepth {
		return false
	}
	for i := range x.Elements {
		if !valuesEqual(x.Elements[i], y.Elements[i], nestDepth+1) {
			return false
		}
	}
	return true
}

// tupleComparisonError iki demetin '=='sinin hatasıdır: tensör tutan öğe (15.5; tipi bilinen yerde
// denetimi 4a2'de) ve walk'ın bulduğu öğe hatası (JSON, fonksiyon; nesnede alanlar). Demet olmayan
// karşı yan (TR_none) hatasızdır.
func tupleComparisonError(x *TupleVal, b Value, nestDepth int, walk func(a, b Value, nestDepth int) error) error {
	y, ok := b.(*TupleVal)
	if !ok {
		return nil
	}
	for i := 0; i < min(len(x.Elements), len(y.Elements)); i++ {
		if x.Elements[i].Type() == ValTensor || y.Elements[i].Type() == ValTensor {
			return messages.New(messages.H2777, i)
		}
		if x != y {
			if err := walk(x.Elements[i], y.Elements[i], nestDepth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// tupleOrderOp demetlerin '<', '>', '<=', '>='sidir (4.4): öğeler sırayla karşılaştırılır, ilk eşit
// olmayan öğe çifti sonucu verir; ortak öğeler eşitse kısa demet küçüktür. Öğeler 4.4'ün
// karşılaştırmasıyladır (IEEE 754: -0.0 == 0.0, NaN'lı karşılaştırma yanlış). Her öğe çiftinin
// sıralanabilir olması önce denetlenir, sonuç değerlere bağlı hata vermez. ok yalnız bu dört işleçte
// ve iki yan da demetse doğrudur.
func tupleOrderOp(codeVal uint8, op string, left, right Value, line int) (Value, bool, error) {
	switch codeVal {
	case opLt, opGt, opLe, opGe:
	default:
		return nil, false, nil
	}
	x, okX := left.(*TupleVal)
	y, okY := right.(*TupleVal)
	if !okX || !okY {
		return nil, false, nil
	}
	if err := orderedPair(op, x, y); err != nil {
		return nil, true, messages.At(line, 0, err)
	}
	return BoolToVal(tupleOrder(codeVal, x, y)), true, nil
}

// orderedPair iki demetin öğe çiftlerinin 4.4'ün karşılaştırabildiği çiftler olduğunu denetler:
// iki sayı, iki metin ya da (iç içe) iki demet.
func orderedPair(op string, x, y *TupleVal) error {
	for i := 0; i < min(len(x.Elements), len(y.Elements)); i++ {
		a, b := x.Elements[i], y.Elements[i]
		switch {
		case isNumber(a) && isNumber(b):
		case isString(a) && isString(b):
		case isTuple(a) && isTuple(b):
			if err := orderedPair(op, a.(*TupleVal), b.(*TupleVal)); err != nil {
				return err
			}
		default:
			return messages.New(messages.H2772, op, i, a.Type(), b.Type())
		}
	}
	return nil
}

func isString(v Value) bool { _, ok := v.(*StringVal); return ok }
func isTuple(v Value) bool  { _, ok := v.(*TupleVal); return ok }

// tupleOrder sıralanabilir öğeli iki demetin karşılaştırmasıdır (orderedPair'den sonra).
func tupleOrder(codeVal uint8, x, y *TupleVal) bool {
	for i := 0; i < min(len(x.Elements), len(y.Elements)); i++ {
		a, b := x.Elements[i], y.Elements[i]
		if orderedEqual(a, b) {
			continue
		}
		switch a := a.(type) {
		case *TupleVal:
			return tupleOrder(codeVal, a, b.(*TupleVal))
		case *StringVal:
			return orderResult(codeVal, strings.Compare(a.Val, b.(*StringVal).Val))
		}
		c, ok := compareNumbers(a, b)
		return ok && orderResult(codeVal, c) // NaN'lı karşılaştırma yanlış
	}
	return orderResult(codeVal, len(x.Elements)-len(y.Elements))
}

// orderedEqual 4.4'ün eşitliğidir: sayılar kesin ve IEEE ile (NaN hiçbir şeye eşit değil).
func orderedEqual(a, b Value) bool {
	switch x := a.(type) {
	case *StringVal:
		return x.Val == b.(*StringVal).Val
	case *TupleVal:
		y := b.(*TupleVal)
		if len(x.Elements) != len(y.Elements) {
			return false
		}
		for i := range x.Elements {
			if !orderedEqual(x.Elements[i], y.Elements[i]) {
				return false
			}
		}
		return true
	}
	c, ok := compareNumbers(a, b)
	return ok && c == 0
}

// orderResult karşılaştırmanın işaretini (c < 0, 0, > 0) işlecin sonucuna çevirir.
func orderResult(codeVal uint8, c int) bool {
	switch codeVal {
	case opLt:
		return c < 0
	case opGt:
		return c > 0
	case opLe:
		return c <= 0
	}
	return c >= 0
}

// sortOrder 13.10'un sıralama sırasıdır, demette öğe öğe: sayılarda -0.0 < 0.0, NaN her sayıdan
// büyük, NaN'lar birbirine eşit; metinler kod noktası sırasıyla; ortak öğeleri eşit demetlerde kısa
// olan önce. Öğe çiftleri sıralanabilir olmalıdır (orderedPair).
func sortOrder(a, b Value) int {
	switch x := a.(type) {
	case *StringVal:
		return strings.Compare(x.Val, b.(*StringVal).Val)
	case *TupleVal:
		y := b.(*TupleVal)
		for i := 0; i < min(len(x.Elements), len(y.Elements)); i++ {
			if c := sortOrder(x.Elements[i], y.Elements[i]); c != 0 {
				return c
			}
		}
		return len(x.Elements) - len(y.Elements)
	}
	if c, ok := compareNumbers(a, b); ok {
		if c == 0 {
			return zeroSign(a) - zeroSign(b) // -0.0 < 0.0
		}
		return c
	}
	switch an, bn := isNaN(a), isNaN(b); {
	case an && bn:
		return 0
	case an:
		return 1
	}
	return -1
}

// zeroSign sıfırın işaretidir: -0.0'da -1, öbür sayılarda 0.
func zeroSign(v Value) int {
	if f, ok := v.(*FloatVal); ok && f.Val == 0 && math.Signbit(f.Val) {
		return -1
	}
	return 0
}

// tupleListError demetlerden oluşması gereken listenin (sıralama, TR_min, TR_max) hatasıdır: her
// öğe demet olmalı ve öğe çiftleri ilk demetinkilerle sıralanabilir olmalıdır.
func tupleListError(name any, d []Value) error {
	first := d[0].(*TupleVal)
	for _, x := range d {
		t, ok := x.(*TupleVal)
		if !ok || orderedPair("<", first, t) != nil {
			return sortTypeError(name, first, x)
		}
	}
	return nil
}

// tupleKey demetin sözlükteki anahtarıdır: öğelerin anahtarları, uzunluklarıyla yazılır (enumKey
// gibi); eşitliği '=='nin eşitliğidir.
func tupleKey(t *TupleVal) string {
	var sb strings.Builder
	sb.WriteString(nonStringKeyPrefix + "tuple")
	for _, e := range t.Elements {
		k := DictKey(e)
		fmt.Fprintf(&sb, ":%d:%s", len(k), k)
	}
	return sb.String()
}

// tupleKeyError demet sözlük anahtarı olabilir mi (3.7): öğelerinin hepsi anahtar olabiliyorsa.
func tupleKeyError(t *TupleVal) error {
	for i, e := range t.Elements {
		if err := DictKeyError(e); err != nil {
			return messages.New(messages.H2773, i, err)
		}
	}
	return nil
}

// extremeTuple TR_min ve TR_max'ın demet listesindeki sonucudur: 13.10'un sırasıyla (NaN'lı demet en
// büyük); eşitlerden ilki döner.
func extremeTuple(kd codes.Code, elems []Value, upper bool) (Value, error) {
	if err := tupleListError(CallPrefix(kd), elems); err != nil {
		return nil, err
	}
	picked := elems[0]
	for _, el := range elems[1:] {
		if c := sortOrder(el, picked); (upper && c > 0) || (!upper && c < 0) {
			picked = el
		}
	}
	return picked, nil
}
