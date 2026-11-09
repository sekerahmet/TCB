package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Dilin eşitliğinin sessiz yanlış verebileceği karşılaştırmaları (3.7a, 4.5, 5.6, 7.6) çalışma hatası yapan
// denetim ve sonucu. Hata sonuçtan önce gelir: sonuç ilk farkta belli olur, ama karşılaştırılamayan öğe
// değerin neresinde olursa olsun hatadır ([1, f] == [2, f] hatadır, 4.5). Bu yüzden gezinti farkta
// durmaz; aynı biçimdeki çiftleri bir kez gezer, hatayı ve sonucu birlikte bulur. Biçimi tutmayan alt
// değerin hatası değere bağlıdır (contentError, equality_members.go). Sonucun kuralı shallowEqual'dadır
// (value_equality.go).

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
	if a.Type() == ValNull || b.Type() == ValNull {
		return valuesEqual(a, b), nil
	}
	eq, descend := shallowEqual(a, b)
	if !descend {
		if err := contentError(a); err != nil {
			return false, err
		}
		if b != a {
			if err := contentError(b); err != nil {
				return false, err
			}
		}
		return eq, nil
	}
	w := equalWalk{budget: equalBudget, eq: true}
	if done, err := w.tree(a, b); done || err != nil {
		return w.eq && err == nil, err
	}
	return w.graph(a, b)
}

// equalWalk EqualValues'un gezintisidir: eq şimdiye dek fark bulunmadığıdır. Önce bütçeli özyineleme (tree;
// bütçe kap çifti ya da kap başına bir azalır), bütçe biterse baştan açık yığın ve birleşim-bul (graph).
type equalWalk struct {
	budget int
	eq     bool
	u      unionFind
	seen   map[Value]bool // genel yolda çifte girmiş ya da taranmış kaplar
}

// tree a ile b'yi (aynı biçimde iki kap) gezer; done yanlışsa bütçe bitti.
func (w *equalWalk) tree(a, b Value) (bool, error) {
	w.budget--
	if w.budget < 0 {
		return false, nil
	}
	if xl, ok := a.(*ListVal); ok { // sıcak yol: listenin öğeleri doğrudan
		yl := b.(*ListVal)
		for i, x := range xl.Elements {
			if done, err := w.treeChild(x, yl.Elements[i]); !done || err != nil {
				return done, err
			}
		}
		return true, nil
	}
	if err := pairMembersError(a, b); err != nil {
		return true, err
	}
	diff := false
	for i := 0; ; i++ {
		x, y, st := children(a, b, i, false)
		switch st {
		case childEnd:
			if diff {
				return w.unpairedTree(a, b)
			}
			return true, nil
		case childSkip:
			continue
		case childDiff:
			diff, w.eq = true, false
			continue
		}
		if done, err := w.treeChild(x, y); !done || err != nil {
			return done, err
		}
	}
}

// treeChild bir öğe çiftidir: aynı biçimde iki kapsa çift gezilir, değilse kap olanın içi taranır.
func (w *equalWalk) treeChild(x, y Value) (bool, error) {
	descend, scan, err := w.child(x, y)
	if err != nil {
		return true, err
	}
	if descend {
		return w.tree(x, y)
	}
	if scan {
		for _, v := range [2]Value{x, y} {
			if done, err := scanTree(v, &w.budget); !done || err != nil {
				return done, err
			}
		}
	}
	return true, nil
}

func (w *equalWalk) unpairedTree(a, b Value) (bool, error) {
	for _, v := range unpaired(a, b) {
		if err := elementError(v); err != nil {
			return true, err
		}
		if done, err := scanTree(v, &w.budget); !done || err != nil {
			return done, err
		}
	}
	return true, nil
}

// graph gezintinin genel yoludur: kap çiftleri açık yığında, kap düğümleri birleşim-bulda (graphEqual gibi).
// Atlanan çiftin (iki düğüm de aynı sınıfta) iki düğümü de daha önce bir çiftte gezilmiştir: öğeleri
// aranmıştır.
func (w *equalWalk) graph(a, b Value) (bool, error) {
	w.eq = true
	w.u = unionFind{index: map[Value]int32{}}
	w.seen = map[Value]bool{}
	w.u.sameOrUnite(a, b)
	stack := []eqFrame{{a: a, b: b}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.i == 0 {
			w.seen[top.a], w.seen[top.b] = true, true
			if err := pairMembersError(top.a, top.b); err != nil {
				return false, err
			}
		}
		x, y, st := children(top.a, top.b, top.i, false)
		top.i++
		switch st {
		case childEnd:
			if top.diff {
				if err := w.unpairedGraph(top.a, top.b); err != nil {
					return false, err
				}
			}
			stack = stack[:len(stack)-1]
			continue
		case childSkip:
			continue
		case childDiff:
			top.diff, w.eq = true, false
			continue
		}
		descend, scan, err := w.child(x, y)
		if err != nil {
			return false, err
		}
		if descend {
			if !w.u.sameOrUnite(x, y) {
				stack = append(stack, eqFrame{a: x, b: y})
			}
			continue
		}
		if !scan {
			continue
		}
		for _, v := range [2]Value{x, y} {
			if err := scanGraph(v, w.seen); err != nil {
				return false, err
			}
		}
	}
	return w.eq, nil
}

func (w *equalWalk) unpairedGraph(a, b Value) error {
	for _, v := range unpaired(a, b) {
		if err := elementError(v); err != nil {
			return err
		}
		if err := scanGraph(v, w.seen); err != nil {
			return err
		}
	}
	return nil
}

// child bir öğe çiftinin kendi hatası ve sonucudur: iki öğenin kendi hatası (elementError), JSON ile JSON
// olmayan (3.7a). descend iki öğenin aynı biçimde kap olduğudur; scan ise birinin kap olup eşiyle aynı
// biçimde olmadığıdır: çağıran içini tarar.
func (w *equalWalk) child(x, y Value) (descend, scan bool, err error) {
	if xi, ok := x.(*IntVal); ok { // sıcak yol: iki Tam
		if yi, ok := y.(*IntVal); ok {
			if xi.Val != yi.Val {
				w.eq = false
			}
			return false, false, nil
		}
	}
	if err := elementError(x); err != nil {
		return false, false, err
	}
	if err := elementError(y); err != nil {
		return false, false, err
	}
	if err := jsonPairError(x, y); err != nil {
		return false, false, err
	}
	eq, descend := shallowEqual(x, y)
	if descend {
		return true, false, nil
	}
	if !eq {
		w.eq = false
	}
	return false, isContainer(x) || isContainer(y), nil
}

// pairMembersError demet ve nesne çiftinde öğelerin kapa bağlı hatasıdır (H2777, H2770); liste ve sözlükte
// öğelerin hatasına child bakar.
func pairMembersError(a, b Value) error {
	switch a.(type) {
	case *TupleVal, *StructInstanceVal:
		if err := memberError(a); err != nil {
			return err
		}
		if b != a {
			return memberError(b)
		}
	}
	return nil
}

// unpaired aynı biçimde iki kabın eşi olmayan öğeleridir: bir sözlükte olup öbüründe olmayan anahtarın
// değeri, yalnız bir nesnede olan alan.
func unpaired(a, b Value) []Value {
	var out []Value
	switch x := a.(type) {
	case *DictVal:
		y := b.(*DictVal)
		for _, k := range x.Order {
			if _, ok := y.Pairs[k]; !ok {
				out = append(out, x.Pairs[k])
			}
		}
		for _, k := range y.Order {
			if _, ok := x.Pairs[k]; !ok {
				out = append(out, y.Pairs[k])
			}
		}
	case *StructInstanceVal:
		y := b.(*StructInstanceVal)
		for _, f := range instanceFields(x) {
			va, okA := x.Fields[f.Name]
			vb, okB := y.Fields[f.Name]
			if okA && !okB {
				out = append(out, va)
			} else if okB && !okA {
				out = append(out, vb)
			}
		}
	}
	return out
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
// (3.7a). Kabın içinde çift çifte aranır, aynı biçimdeki çiftlerde. Sınırı: genel yolda birleşim-bul
// x ~ z ve z ~ y'yi birleştirince (x, y)'yi gezmez; z'nin aynı yerinde TR_none varsa o çiftin hatası kaçar.
// Bütçeyi aşan, paylaşılan ya da kendine bağlanan, genel tipli değerde olur: tipli kodda kabın içinde JSON
// ile JSON olmayan değer aynı yerde durmaz.
func jsonPairError(a, b Value) error {
	_, ja := a.(*JSONVal)
	_, jb := b.(*JSONVal)
	if !ja && !jb {
		return nil
	}
	if jsonCompareError("==", a, b) == nil {
		return nil
	}
	j, other := a, b
	if !ja {
		j, other = b, a
	}
	return messages.New(messages.H2705, j.(*JSONVal).kindName(), other.Type())
}
