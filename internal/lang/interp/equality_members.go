package interp

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Karşılaştırılamayan öğenin aranması (4.5, 7.6, 15.5): hata değere bağlıdır, çifte değil. Kabın içinde
// nerede olursa olsun fonksiyon, hata değeri ve tensör (H2777; yapı alanında H2770) hatadır; karşısındaki
// öğeye bakılmaz. TR_assert_equal'ın kipinde (tensorByValue) tensör hata değildir, karşılaştırılır (12.4). EqualValues aynı biçimdeki çiftlerde bunu çiftin gezintisinde
// yapar (equality.go), biçimi tutmayan alt değerde contentError'la.

// memberError kabın kendi öğelerinin hatasıdır: fonksiyon, hata değeri ya da tensör tutan öğe ve alan.
// Öğelerin içine contentError ve çiftin gezintisi iner.
func memberError(v Value, tensors tensorRule) error {
	switch x := v.(type) {
	case *ListVal:
		for _, e := range x.Elements {
			if err := elementError(e, tensors); err != nil {
				return err
			}
		}
	case *DictVal:
		for _, k := range x.Order {
			if err := elementError(x.Pairs[k], tensors); err != nil {
				return err
			}
		}
	case *TupleVal:
		for _, e := range x.Elements {
			if err := elementError(e, tensors); err != nil {
				return err
			}
		}
	case *StructInstanceVal:
		return fieldError(x, tensors)
	}
	return nil
}

// fieldError fonksiyon ya da tensör tutan alanın hatasıdır (7.6, H2770); iletinin önerisi
// karşılaştırılabilen bir alandır (plainField). TR_assert_equal'ın kipinde tensör alanı karşılaştırılır.
func fieldError(x *StructInstanceVal, tensors tensorRule) error {
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
		} else if v.Type() == ValTensor && tensors == tensorIsError {
			kind = "tensor"
		}
		if kind != "" {
			return messages.New(messages.H2770, label, f.Name, kind, plainField(x, x, f.Name))
		}
		if err := elementError(v, tensors); err != nil {
			return err
		}
	}
	return nil
}

// elementError bir öğenin kendi hatasıdır: fonksiyon (4.5), hata değeri (8.3) ya da tensör (15.5;
// TR_assert_equal'ın kipinde değil).
func elementError(e Value, tensors tensorRule) error {
	switch e.(type) {
	case *IntVal, *FloatVal, *StringVal, *BoolVal, *NullVal:
		return nil
	}
	if isFunction(e) {
		return messages.New(messages.H2706)
	}
	if e.Type() == ValError {
		return messages.New(messages.H2913, "")
	}
	if tensors == tensorIsError && e.Type() == ValTensor {
		return messages.New(messages.H2777)
	}
	return nil
}

// member kabın i. öğesidir (liste ve demette sıra, sözlükte anahtar sırası, nesnede alan sırası); ok
// yanlışsa öğeler bitti. Nesnede olmayan alan yokluktur.
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

// contentError v'nin içindeki ilk hatadır (memberError), değerin neresinde olursa olsun. Önce bütçeli
// özyineleme (bütçe kap başına bir azalır), bütçe biterse açık yığın ve ziyaret kümesi: kendine bağlanan
// değerde de biter, bellek düğüm sayısıyla doğrusaldır.
func contentError(v Value, tensors tensorRule) error {
	if !isContainer(v) {
		return nil
	}
	budget := equalBudget
	if done, err := scanTree(v, &budget, tensors); done || err != nil {
		return err
	}
	return scanGraph(v, map[Value]bool{}, tensors)
}

// scanTree contentError'ın özyinelemeli yoludur; done yanlışsa bütçe bitti.
func scanTree(v Value, budget *int, tensors tensorRule) (bool, error) {
	if !isContainer(v) {
		return true, nil
	}
	*budget--
	if *budget < 0 {
		return false, nil
	}
	if err := memberError(v, tensors); err != nil {
		return true, err
	}
	for i := 0; ; i++ {
		e, ok := member(v, i)
		if !ok {
			return true, nil
		}
		if done, err := scanTree(e, budget, tensors); !done || err != nil {
			return done, err
		}
	}
}

// scanGraph contentError'ın genel yoludur; seen gezilmiş kaplardır (EqualValues'un genel yolu, çifte girmiş
// düğümleri de buraya koyar).
func scanGraph(root Value, seen map[Value]bool, tensors tensorRule) error {
	stack := []Value{root}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !isContainer(v) || seen[v] {
			continue
		}
		seen[v] = true
		if err := memberError(v, tensors); err != nil {
			return err
		}
		start := len(stack)
		for i := 0; ; i++ {
			e, ok := member(v, i)
			if !ok {
				break
			}
			if isContainer(e) {
				stack = append(stack, e)
			}
		}
		slices.Reverse(stack[start:]) // öğe sırasıyla gezilsin
	}
	return nil
}

// uncomparableError v'nin '==' ile karşılaştırılamamasının hatasıdır, eşine bakmadan: kendisi ya da içi
// fonksiyon, hata değeri ya da tensör (4.5, 15.5). .TR_contains ve .TR_index_of aranan değeri, .TR_unique her
// öğeyi karşılaştırmadan önce sınar: boş ya da tek öğeli listede de hata aynıdır. TR_none'la karşılaştırma
// serbesttir.
func uncomparableError(v Value) error {
	if err := elementError(v, tensorIsError); err != nil {
		return err
	}
	return contentError(v, tensorIsError)
}
