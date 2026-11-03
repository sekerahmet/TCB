package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dilin eşitliğinin sessiz yanlış verebileceği karşılaştırmaları (3.7a, 4.5, 5.6) çalışma hatası yapan
// denetim: değerlerin kendisi valuesEqual'dadır (operators.go).

// EqualValues is the language's equality (4.5) where a wrong answer would be silent: in '==', in
// .contains and .unique, in a match pattern and in assert_equal. A JSON value compared with a value that
// is not JSON (3.7a), a function (4.5) and a value that reaches itself (7.6) are errors there, also inside
// a list or a dictionary.
func EqualValues(a, b Value) (bool, error) {
	if err := (comparison{}).walk(a, b, 0); err != nil {
		return false, err
	}
	return valuesEqual(a, b, 0), nil
}

// comparison valuesEqual'ın karşılaştıracağı her öğe çiftini gezer: boyu aynı kapların her öğe çifti,
// aynı yapıdan nesnelerin her alan çifti (fields, struct_ops.go); aynı kap da gezilir (7.6). Boyu farklı
// kapların öğeleri karşılaştırılmaz, gezilmez. Kümede gezintinin yolundaki kap çiftleri vardır (bir çift
// bitince çıkar): karşılaştırılmakta olan bir liste, sözlük ya da nesne çiftine yeniden varmak (kendini
// içeren değer) çalışma hatasıdır (H2771); yoksa valuesEqual derinlik sınırında sessizce yanlış verirdi.
type comparison map[[2]Value]bool

// cycleDepth çiftlerin kümeye girdiği derinliktir: kendini içeren değerin gezintisi her derinliğe iner,
// döngüsü bu derinlikten sonra da yakalanır; sığ değerler (ör. küçük listelerin listesi) kümeye hiç girmez.
const cycleDepth = 100

func (w comparison) walk(a, b Value, nestDepth int) error {
	if nestDepth >= equalityDepth {
		return nil
	}
	if err := comparisonError(a, b); err != nil {
		return err
	}
	switch x := a.(type) {
	case *ListVal:
		if y, ok := b.(*ListVal); ok && len(x.Elements) == len(y.Elements) {
			return w.list(x, y, nestDepth)
		}
	case *TupleVal:
		return tupleComparisonError(x, b, nestDepth, w.walk) // tuple_ops.go
	case *DictVal:
		if y, ok := b.(*DictVal); ok && len(x.Pairs) == len(y.Pairs) {
			return w.dict(x, y, nestDepth)
		}
	case *StructInstanceVal:
		if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def && x.Variant == y.Variant {
			return w.fields(x, y, nestDepth) // struct_ops.go
		}
	}
	return nil
}

func (w comparison) list(x, y *ListVal, nestDepth int) error {
	if !w.enter(x, y, nestDepth) {
		return messages.New(messages.H2771, codes.NameRef(codes.TypeList), "")
	}
	defer w.leave(x, y, nestDepth)
	for i := range x.Elements {
		if err := w.walk(x.Elements[i], y.Elements[i], nestDepth+1); err != nil {
			return err
		}
	}
	return nil
}

func (w comparison) dict(x, y *DictVal, nestDepth int) error {
	if !w.enter(x, y, nestDepth) {
		return messages.New(messages.H2771, codes.NameRef(codes.TypeDict), "")
	}
	defer w.leave(x, y, nestDepth)
	for k, va := range x.Pairs {
		if vb, ok := y.Pairs[k]; ok {
			if err := w.walk(va, vb, nestDepth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// enter (x, y) çiftini gezintinin yoluna koyar (cycleDepth'ten itibaren); çift zaten yoldaysa (değer kendine
// varıyor) yanlış döner.
func (w comparison) enter(x, y Value, nestDepth int) bool {
	if nestDepth < cycleDepth {
		return true
	}
	pair := [2]Value{x, y}
	if w[pair] {
		return false
	}
	w[pair] = true
	return true
}

func (w comparison) leave(x, y Value, nestDepth int) {
	if nestDepth >= cycleDepth {
		delete(w, [2]Value{x, y})
	}
}

// comparisonError tek bir değer çiftinin hatasıdır: JSON değeri JSON olmayan bir değerle (yokluk dışında),
// bir fonksiyon ya da bir hata değeri karşılaştırılıyorsa.
func comparisonError(a, b Value) error {
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
	// Hata değerinin kendisi karşılaştırılmaz (8.3): kodu ya da fırlatılan değeri karşılaştırılır;
	// yoklukla karşılaştırma serbesttir (e.TR_cause == TR_none).
	if (a.Type() == ValError || b.Type() == ValError) && a.Type() != ValNull && b.Type() != ValNull {
		return messages.New(messages.H2913, "") // öneri programın yazımıyla '=='nın derlendiği yerde kurulur (errorComparison)
	}
	return nil
}
