package interp

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dilin eşitliğinin sessiz yanlış verebileceği karşılaştırmaları (3.7a, 4.5, 5.6, 7.6) çalışma hatası yapan
// denetim: değerlerin kendisi valuesEqual'dadır (value_equality.go). Hatalar sonuçtan önce aranır: sonuç
// ilk farkta durur, hata ise değerin neresinde olursa olsun hatadır ([1, f] == [2, f] hatadır, 4.5).

// EqualValues is the language's equality (4.5) where a wrong answer would be silent: in '==', in
// .contains and .unique, in a match pattern and in assert_equal. A JSON value compared with a value that
// is not JSON (3.7a) and a function (4.5) are errors there, also inside a list, a dictionary, a tuple or
// an object; a value that reaches itself is compared too (7.6).
func EqualValues(a, b Value) (bool, error) {
	if err := comparisonError(a, b); err != nil {
		return false, err
	}
	// Eşitlenemeyen tipin isteğe bağlısı (fonksiyon ve onu içeren değer) TR_none'la karşılaştırılabilir
	// (4.5); öbür durumda iki değerin içi de aranır, karşısındaki öğeye bakılmaz.
	if a.Type() != ValNull && b.Type() != ValNull {
		jsonA, err := contentError(a)
		if err != nil {
			return false, err
		}
		jsonB := false
		if b != a {
			if jsonB, err = contentError(b); err != nil {
				return false, err
			}
		}
		if jsonA || jsonB {
			if err := jsonPairsError(a, b); err != nil {
				return false, err
			}
		}
	}
	return valuesEqual(a, b), nil
}

// comparisonError tek bir değer çiftinin hatasıdır: JSON değeri JSON olmayan bir değerle (yokluk dışında),
// bir fonksiyon ya da bir hata değeri karşılaştırılıyorsa.
func comparisonError(a, b Value) error {
	if err := jsonPairError(a, b); err != nil {
		return err
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

// jsonPairError JSON değerinin JSON olmayan bir değerle (yokluk dışında) karşılaştırılmasının hatasıdır
// (3.7a).
func jsonPairError(a, b Value) error {
	if jsonCompareError("==", a, b) == nil {
		return nil
	}
	j, other := a, b
	if _, ok := j.(*JSONVal); !ok {
		j, other = b, a
	}
	return messages.New(messages.H2705, j.(*JSONVal).kindName(), other.Type())
}

// memberError kabın kendi öğelerinin hatasıdır (4.5, 7.6, 15.5): fonksiyon ya da hata değeri tutan öğe,
// fonksiyon ya da tensör tutan alan, tensör tutan demet öğesi. Öğelerin içine contentError iner.
func memberError(v Value) error {
	switch x := v.(type) {
	case *ListVal:
		for _, e := range x.Elements {
			if err := elementError(e); err != nil {
				return err
			}
		}
	case *DictVal:
		for _, k := range x.Order {
			if err := elementError(x.Pairs[k]); err != nil {
				return err
			}
		}
	case *TupleVal:
		for i, e := range x.Elements {
			if holdsTensor(e) {
				return messages.New(messages.H2777, i)
			}
			if err := elementError(e); err != nil {
				return err
			}
		}
	case *StructInstanceVal:
		return fieldError(x)
	}
	return nil
}

// fieldError fonksiyon ya da tensör tutan alanın hatasıdır (7.6, H2770); iletinin önerisi karşılaştırılabilen
// bir alandır (plainField).
func fieldError(x *StructInstanceVal) error {
	label := x.Def.Name
	if x.Variant != nil {
		label = variantLabel(x.Def, x.Variant)
	}
	for _, f := range instanceFields(x) {
		v, ok := x.Fields[f.Name]
		if !ok {
			continue
		}
		kind := ""
		if isFunction(v) {
			kind = "function"
		} else if v.Type() == ValTensor {
			kind = "tensor"
		}
		if kind != "" {
			return messages.New(messages.H2770, label, f.Name, kind, plainField(x, x, f.Name))
		}
		if err := elementError(v); err != nil {
			return err
		}
	}
	return nil
}

// elementError kabın içindeki bir öğenin kendi hatasıdır: fonksiyon (4.5) ya da hata değeri (8.3).
func elementError(e Value) error {
	if isFunction(e) {
		return messages.New(messages.H2706)
	}
	if e.Type() == ValError {
		return messages.New(messages.H2913, "")
	}
	return nil
}

// member kabın i. öğesidir (liste ve demette sıra, sözlükte anahtar sırası, nesnede alan sırası); ok yanlışsa
// öğeler bitti. Nesnede olmayan alan yokluktur.
func member(v Value, i int) (Value, bool) {
	switch x := v.(type) {
	case *ListVal:
		if i < len(x.Elements) {
			return x.Elements[i], true
		}
	case *TupleVal:
		if i < len(x.Elements) {
			return x.Elements[i], true
		}
	case *DictVal:
		if i < len(x.Order) {
			return x.Pairs[x.Order[i]], true
		}
	case *StructInstanceVal:
		fields := instanceFields(x)
		if i < len(fields) {
			if e, ok := x.Fields[fields[i].Name]; ok {
				return e, true
			}
			return Null, true
		}
	}
	return nil, false
}

func isContainer(v Value) bool {
	switch v.(type) {
	case *ListVal, *TupleVal, *DictVal, *StructInstanceVal:
		return true
	}
	return false
}

// contentError v'nin içindeki ilk hatadır (memberError), değerin neresinde olursa olsun; hasJSON v'nin
// içinde bir JSON değeri olduğudur (o zaman çiftler de aranır, jsonPairsError). Önce bütçeli özyineleme,
// bütçe biterse açık yığın ve ziyaret kümesi: kendine bağlanan değerde de biter, bellek düğüm sayısıyla
// doğrusaldır.
func contentError(v Value) (hasJSON bool, err error) {
	if !isContainer(v) {
		return false, nil
	}
	budget := equalBudget
	if done, err := scanTree(v, &budget, &hasJSON); done || err != nil {
		return hasJSON, err
	}
	hasJSON = false
	return scanGraph(v)
}

func scanTree(v Value, budget *int, hasJSON *bool) (bool, error) {
	*budget--
	if *budget < 0 {
		return false, nil
	}
	if _, ok := v.(*JSONVal); ok {
		*hasJSON = true
		return true, nil
	}
	if !isContainer(v) {
		return true, nil
	}
	if err := memberError(v); err != nil {
		return true, err
	}
	for i := 0; ; i++ {
		e, ok := member(v, i)
		if !ok {
			return true, nil
		}
		if done, err := scanTree(e, budget, hasJSON); !done || err != nil {
			return done, err
		}
	}
}

func scanGraph(root Value) (hasJSON bool, err error) {
	seen := map[Value]bool{}
	stack := []Value{root}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := v.(*JSONVal); ok {
			hasJSON = true
			continue
		}
		if !isContainer(v) || seen[v] {
			continue
		}
		seen[v] = true
		if err := memberError(v); err != nil {
			return hasJSON, err
		}
		start := len(stack)
		for i := 0; ; i++ {
			e, ok := member(v, i)
			if !ok {
				break
			}
			if isContainer(e) || e.Type() == ValJSON {
				stack = append(stack, e)
			}
		}
		slices.Reverse(stack[start:]) // öğe sırasıyla gezilsin
	}
	return hasJSON, nil
}

// jsonPairsError JSON değerinin aynı yerdeki JSON olmayan değerle karşılaştırılmasıdır (3.7a), iki değerin
// öğe çiftlerinde: aynı boy listeler, aynı boy sözlüklerin ortak anahtarları, aynı yapının ortak alanları,
// iki demetin ortak öğeleri. Önce bütçeli özyineleme, sonra birleşim-bul (valuesEqual'daki gibi).
//
// Sınırı: birleşim-bul x ~ z ve z ~ y'yi birleştirince (x, y)'yi gezmez; z'nin aynı yerinde TR_none varsa o
// çiftin hatası kaçar. Paylaşılan ya da kendine bağlanan, genel tipli değerde olur: tipli kodda kabın içinde
// JSON ile JSON olmayan değer aynı yerde durmaz.
func jsonPairsError(a, b Value) error {
	budget := equalBudget
	if done, err := jsonPairsTree(a, b, &budget); done || err != nil {
		return err
	}
	if !jsonShape(a, b) {
		return nil
	}
	u := unionFind{index: map[Value]int32{}}
	u.sameOrUnite(a, b)
	stack := []eqFrame{{a: a, b: b}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		x, y, st := children(top.a, top.b, top.i, true)
		top.i++
		switch st {
		case childEnd:
			stack = stack[:len(stack)-1]
			continue
		case childSkip:
			continue
		}
		if err := jsonPairError(x, y); err != nil {
			return err
		}
		if jsonShape(x, y) && !u.sameOrUnite(x, y) {
			stack = append(stack, eqFrame{a: x, b: y})
		}
	}
	return nil
}

func jsonPairsTree(a, b Value, budget *int) (bool, error) {
	*budget--
	if *budget < 0 {
		return false, nil
	}
	if err := jsonPairError(a, b); err != nil {
		return true, err
	}
	if !jsonShape(a, b) {
		return true, nil
	}
	for i := 0; ; i++ {
		x, y, st := children(a, b, i, true)
		switch st {
		case childEnd:
			return true, nil
		case childSkip:
			continue
		}
		if done, err := jsonPairsTree(x, y, budget); !done || err != nil {
			return done, err
		}
	}
}

// jsonShape iki değerin öğe çiftlerinin JSON için gezildiğidir: aynı boy listeler, aynı boy sözlükler, iki
// demet, aynı yapıdan aynı durumda iki nesne.
func jsonShape(a, b Value) bool {
	switch x := a.(type) {
	case *ListVal:
		y, ok := b.(*ListVal)
		return ok && len(x.Elements) == len(y.Elements)
	case *DictVal:
		y, ok := b.(*DictVal)
		return ok && len(x.Pairs) == len(y.Pairs)
	case *TupleVal:
		_, ok := b.(*TupleVal)
		return ok
	case *StructInstanceVal:
		y, ok := b.(*StructInstanceVal)
		return ok && x.Def == y.Def && x.Variant == y.Variant
	}
	return false
}

// holdsTensor v'nin bir tensör ya da tensör tutan bir liste veya sözlük olduğudur (iç içe de, sınırsız;
// kendini içeren kapta da biter). Demetin içindeki demeti ve nesneyi contentError ayrıca gezer.
func holdsTensor(v Value) bool {
	switch v.(type) {
	case *ListVal, *DictVal:
	default:
		return v.Type() == ValTensor
	}
	seen := map[Value]bool{}
	stack := []Value{v}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch c := x.(type) {
		case *ListVal:
			if seen[x] {
				continue
			}
			seen[x] = true
			stack = append(stack, c.Elements...)
		case *DictVal:
			if seen[x] {
				continue
			}
			seen[x] = true
			for _, k := range c.Order {
				stack = append(stack, c.Pairs[k])
			}
		default:
			if x.Type() == ValTensor {
				return true
			}
		}
	}
	return false
}
