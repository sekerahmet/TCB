package interp

import "github.com/sekerahmet/TCB/internal/lang/ast"

// Eşitliğin sonucu (4.5, 7.6): iki değer, açılmış ağaçları her düğümde 4.5'in kuralıyla eşitse eşittir;
// iç içelik sınırı yoktur, kendine bağlanan değerde de karşılaştırma biter. Yol Adams ve Dybvig'in
// (ICFP 2008) ve Chez Scheme'in equal?'inindir (s/5_1.ss): önce bütçeli özyineleme (treeEqual), bütçe
// biterse açık yığın ve kap düğümleri üstünde birleşim-bul (graphEqual). Chez'in eq? kısa yolu yoktur:
// aynı değer de öğe öğe karşılaştırılır (TR_nan tutan değer kendisine eşit değil). valuesEqual yalnız
// sonuçtur (ValuesEqual, Equaler); dilin '=='si hatayı da arayan EqualValues'tur (equality.go).

// equalBudget hızlı kipte gezilen en çok kap çifti sayısıdır (yaprak çiftleri sayılmaz): kendine bağlanan
// değer, çok derin değer ve aynı alt nesneyi birçok yoldan tutan değer bu bütçeyi bitirir, birleşim-bula
// geçer. Derinlik değil adım sayısıdır: aynı alt nesneyi iki yerde tutan 20 katlı değer derinlik sınırının
// altında 2²⁰ adım ederdi.
const equalBudget = 10000

func ValuesEqual(a, b Value) bool {
	return valuesEqual(a, b)
}

func valuesEqual(a, b Value) bool {
	budget := equalBudget
	if eq, ok := treeEqual(a, b, &budget); ok {
		return eq
	}
	return graphEqual(a, b)
}

// treeEqual karşılaştırmanın özyinelemeli yoludur; ok, bütçe bitmeden sonuca varıldığıdır. Bulunan fark
// açılmış ağaçların gerçek bir farkıdır; doğru sonuç ancak bütün ağaç gezilince verilir.
func treeEqual(a, b Value, budget *int) (eq, ok bool) {
	eq, descend := shallowEqual(a, b)
	if !descend {
		return eq, true
	}
	*budget--
	if *budget < 0 {
		return false, false
	}
	for i := 0; ; i++ {
		x, y, st := children(a, b, i, false)
		switch st {
		case childEnd:
			return true, true
		case childSkip:
			continue
		case childDiff:
			return false, true
		}
		if eq, ok := treeEqual(x, y, budget); !ok || !eq {
			return eq, ok
		}
	}
}

// eqFrame açık yığında bir kap çiftidir; i sıradaki öğe çiftidir, diff eşi olmayan öğe görüldüğüdür
// (EqualValues'un gezintisi onları sonra tarar).
type eqFrame struct {
	a, b Value
	i    int
	diff bool
}

// graphEqual karşılaştırmanın genel yoludur: kap çiftleri açık yığında (Go yığını taşmaz), kap düğümleri
// birleşim-bulda. İki düğüm de daha önce bir çifte girmişse ve aynı sınıftaysa çift eşit sayılır (yolda
// ya da denetlenmiş bir varsayım); değilse sınıfları birleşir, öğe çiftleri gezilir. Bellek iki değerin
// kap düğümü sayısıyla doğrusaldır.
func graphEqual(a, b Value) bool {
	eq, descend := shallowEqual(a, b)
	if !descend {
		return eq
	}
	u := unionFind{index: map[Value]int32{}}
	u.sameOrUnite(a, b)
	stack := []eqFrame{{a: a, b: b}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		x, y, st := children(top.a, top.b, top.i, false)
		top.i++
		switch st {
		case childEnd:
			stack = stack[:len(stack)-1]
			continue
		case childSkip:
			continue
		case childDiff:
			return false
		}
		eq, descend := shallowEqual(x, y)
		if !descend {
			if !eq {
				return false
			}
			continue
		}
		if !u.sameOrUnite(x, y) {
			stack = append(stack, eqFrame{a: x, b: y})
		}
	}
	return true
}

// shallowEqual iki değerin kendi düzeyidir: descend doğruysa ikisi aynı biçimde kaptır (aynı boy liste,
// demet ve sözlük, aynı yapıdan aynı durumda nesne) ve eşitlikleri öğe çiftlerine bağlıdır (children);
// değilse eq sonuçtur.
func shallowEqual(a, b Value) (eq, descend bool) {
	if x, ok := a.(*TupleVal); ok {
		y, ok := b.(*TupleVal)
		return false, ok && len(x.Elements) == len(y.Elements)
	}
	if _, ok := b.(*TupleVal); ok {
		return false, false // demet yalnız demete eşittir; a demet değil
	}
	if a.Type() == ValNull && b.Type() == ValNull {
		return true, false
	}
	if ai, ok := a.(*IntVal); ok {
		if bi, ok := b.(*IntVal); ok {
			return ai.Val == bi.Val, false
		}
	}
	// A Tam and an Ondalık are compared exactly; converting the Tam to Ondalık would make
	// 9007199254740993 == 9007199254740992.0 true. NaN equals nothing.
	if c, ok := CompareNumbers(a, b); ok {
		return c == 0, false
	}
	if _, ok1 := ToFloat(a); ok1 {
		if _, ok2 := ToFloat(b); ok2 {
			return false, false
		}
	}
	// Değerin kendi eşitlik kuralı (ör. tensör: şekil ve tüm elemanlar)
	if e, ok := a.(Equaler); ok {
		return e.Equal(b), false
	}
	if e, ok := b.(Equaler); ok {
		return e.Equal(a), false
	}
	if a.Type() != b.Type() {
		return false, false
	}
	// Kaplar eleman eleman karşılaştırılır: görünen metinleri aynı olan farklı değerler (ör. yazdırırken
	// yuvarlanan elemanlar) eşit sayılmaz.
	switch x := a.(type) {
	case *StringVal:
		if y, ok := b.(*StringVal); ok {
			return x.Val == y.Val, false
		}
	case *ListVal:
		if y, ok := b.(*ListVal); ok {
			return false, len(x.Elements) == len(y.Elements)
		}
	case *DictVal:
		if y, ok := b.(*DictVal); ok {
			return false, len(x.Pairs) == len(y.Pairs)
		}
	case *StructInstanceVal:
		// Aynı yapıdan iki nesne alanlarıyla, tanım sırasıyla karşılaştırılır (7.6); başka yapının nesnesi
		// kimlikle.
		if y, ok := b.(*StructInstanceVal); ok && x.Def == y.Def && x.Variant == y.Variant {
			return false, true
		}
		return Value(x) == b, false
	case *BoolVal:
		return x.Val == b.(*BoolVal).Val, false
	case *RangeVal:
		y := b.(*RangeVal)
		return x.Start == y.Start && x.End == y.End && x.Inclusive == y.Inclusive, false
	case *FuncVal:
		// Kimlik: aynı değer ya da aynı tanımın aynı ortamdaki (ve metotlarda aynı nesneye
		// bağlı) kopyası. Görünüşü (Inspect) aynı olan farklı fonksiyonlar eşit değildir.
		y := b.(*FuncVal)
		return x == y || (x.codeVal != nil && x.codeVal == y.codeVal && x.Env == y.Env && x.Body == y.Body && sameValue(x.BoundSelf, y.BoundSelf)), false
	}
	// Diğer değerler (yerleşikler, yapı tanımları, motor nesneleri) kimlikle karşılaştırılır.
	return a == b, false
}

// childState children'ın bir sıradaki cevabıdır.
type childState uint8

const (
	childPair childState = iota // x, y bir öğe çifti
	childSkip                   // bu sırada çift yok (iki nesnede de olmayan alan)
	childDiff                   // bu sırada fark var (sözlükte eksik anahtar, yalnız bir nesnede olan alan)
	childEnd                    // öğe çiftleri bitti
)

// children aynı biçimde iki kabın i. öğe çiftidir: liste ve demette sıra, sözlükte a'nın anahtar sırası,
// nesnede alanların tanım sırası. lenient hata arayan gezintinindir (jsonPairsError): fark çift sayılmaz,
// atlanır; boyu ayrı iki demette ortak öğeler gezilir.
func children(a, b Value, i int, lenient bool) (Value, Value, childState) {
	switch x := a.(type) {
	case *ListVal:
		y := b.(*ListVal)
		if i >= len(x.Elements) {
			return nil, nil, childEnd
		}
		return x.Elements[i], y.Elements[i], childPair
	case *TupleVal:
		y := b.(*TupleVal)
		if i >= len(x.Elements) || i >= len(y.Elements) {
			return nil, nil, childEnd
		}
		return x.Elements[i], y.Elements[i], childPair
	case *DictVal:
		y := b.(*DictVal)
		if i >= len(x.Order) {
			return nil, nil, childEnd
		}
		k := x.Order[i]
		vb, ok := y.Pairs[k]
		if !ok {
			return nil, nil, missing(lenient)
		}
		return x.Pairs[k], vb, childPair
	case *StructInstanceVal:
		y := b.(*StructInstanceVal)
		fields := instanceFields(x)
		if i >= len(fields) {
			return nil, nil, childEnd
		}
		va, okA := x.Fields[fields[i].Name]
		vb, okB := y.Fields[fields[i].Name]
		if okA != okB {
			return nil, nil, missing(lenient)
		}
		if !okA {
			return nil, nil, childSkip
		}
		return va, vb, childPair
	}
	return nil, nil, childEnd
}

func missing(lenient bool) childState {
	if lenient {
		return childSkip
	}
	return childDiff
}

// instanceFields nesnenin alanlarıdır, tanım sırasıyla; durumlu tipte durumun alanları (7.7).
func instanceFields(x *StructInstanceVal) []ast.StructField {
	if x.Variant != nil {
		return x.Variant.Fields
	}
	return x.Def.Fields
}

// unionFind kap düğümlerinin denklik sınıflarıdır (yol yarılama, boyla birleştirme). Yalnız bir çifte
// girmiş düğüm kayıtlıdır.
type unionFind struct {
	index  map[Value]int32
	parent []int32
	size   []int32
}

// sameOrUnite a ile b'nin ikisi de kayıtlı ve aynı sınıftaysa doğrudur; değilse kaydeder, sınıflarını
// birleştirir ve yanlış döner. Kayıtsız düğümde (henüz hiçbir çifte girmemiş) kısa yol yoktur: a == a'da
// öğeler karşılaştırılır (TR_nan).
func (u *unionFind) sameOrUnite(a, b Value) bool {
	ia, okA := u.index[a]
	ib, okB := u.index[b]
	if okA && okB {
		ra, rb := u.find(ia), u.find(ib)
		if ra == rb {
			return true
		}
		u.link(ra, rb)
		return false
	}
	if !okA {
		ia = u.add(a)
	}
	if !okB {
		if a == b {
			ib = ia
		} else {
			ib = u.add(b)
		}
	}
	u.link(u.find(ia), u.find(ib))
	return false
}

func (u *unionFind) add(v Value) int32 {
	n := int32(len(u.parent))
	u.index[v] = n
	u.parent = append(u.parent, n)
	u.size = append(u.size, 1)
	return n
}

func (u *unionFind) find(n int32) int32 {
	for u.parent[n] != n {
		u.parent[n] = u.parent[u.parent[n]]
		n = u.parent[n]
	}
	return n
}

func (u *unionFind) link(ra, rb int32) {
	if ra == rb {
		return
	}
	if u.size[ra] < u.size[rb] {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	u.size[ra] += u.size[rb]
}
