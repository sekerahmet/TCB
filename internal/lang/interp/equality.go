package interp

import "github.com/sekerahmet/TCB/internal/lang/messages"

// Dilin eşitliğinin sessiz yanlış verebileceği karşılaştırmaları (3.7a, 4.5, 5.6) çalışma hatası yapan
// denetim: değerlerin kendisi valuesEqual'dadır (operators.go).

// EqualValues is the language's equality (4.5) where a wrong answer would be silent: in '==', in
// .contains and .unique, in a match pattern and in assert_equal. A JSON value compared with a value that
// is not JSON (3.7a) and a function (4.5) are errors there, also inside a list or a dictionary.
func EqualValues(a, b Value) (bool, error) {
	if err := comparisonError(a, b, 0); err != nil {
		return false, err
	}
	return valuesEqual(a, b, 0), nil
}

// comparisonError valuesEqual'ın karşılaştıracağı her öğe çiftini gezer: JSON değeri JSON olmayan bir
// değerle (yokluk dışında) ya da bir fonksiyon karşılaştırılıyorsa hatadır. Boyu farklı kapların öğeleri
// karşılaştırılmaz, gezilmez; aynı kap kendisine eşittir. Nesnenin alanları structComparisonError'dadır.
func comparisonError(a, b Value, nestDepth int) error {
	if nestDepth >= equalityDepth {
		return nil
	}
	if jsonCompareError("==", a, b) != nil {
		j, other := a, b
		if _, ok := j.(*JSONVal); !ok {
			j, other = b, a
		}
		return messages.New(messages.H2705, j.(*JSONVal).kindName(), other.Type())
	}
	if (isFunction(a) || isFunction(b)) && a.Type() != ValNull && b.Type() != ValNull {
		return messages.New(messages.H2706)
	}
	switch x := a.(type) {
	case *ListVal:
		if y, ok := b.(*ListVal); ok && x != y && len(x.Elements) == len(y.Elements) {
			for i := range x.Elements {
				if err := comparisonError(x.Elements[i], y.Elements[i], nestDepth+1); err != nil {
					return err
				}
			}
		}
	case *DictVal:
		if y, ok := b.(*DictVal); ok && x != y && len(x.Pairs) == len(y.Pairs) {
			for k, va := range x.Pairs {
				if vb, ok := y.Pairs[k]; ok {
					if err := comparisonError(va, vb, nestDepth+1); err != nil {
						return err
					}
				}
			}
		}
	case *StructInstanceVal:
		return structComparisonError(x, b, nestDepth)
	}
	return nil
}
