package checker

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Üyenin değerinin tipi (1.2, 13.9): tipi bilinen çekirdek alıcıda (TR_String, TR_List, TR_Dict,
// TR_Range) yerleşik özelliğin değeri ve metot çağrısının sonucu, tanım dosyasının o alıcıdaki dönüş
// yazımıdır; yazımın tip parametreleri alıcının öğe tipleriyle kurulur: TR_List[T]'de T, TR_Dict[K, V]'de
// K ve V. [1, 2].TR_length TR_Int, [1, 2].TR_first TR_Int?, {"a": 1}.TR_keys() TR_List[TR_String].

// MemberReturnsNotVerified are core members whose return on a receiver the engine does not give in
// the definition's example, or whose example does not call them on that receiver (member → the
// receivers; stdlib.TestMemberReturnsMatchEngine): the checker does not use their return there.
var MemberReturnsNotVerified = map[codes.Code][]codes.Code{}

// memberValueType tipi bilinen çekirdek alıcıdaki yerleşik üyenin değerinin tipidir: çağrıda (called)
// metodun sonucu, çağrısız yazımda özelliğin değeri. Çağrısız metot (6.9'un bağlı fonksiyonu),
// çağrılan özellik (H2421), motorla doğrulanmamış dönüş ve birden çok seçenekli dönüş "" verir.
func (c *Checker) memberValueType(m *ast.MemberExpression, objType string, called bool) string {
	if objType == typeJSON {
		return jsonMemberType(m, called) // 3.7a "Okumak" (json_values.go)
	}
	k, recv, def, ok := memberOf(m, objType)
	if !ok || !countedReceivers[recv] || called != (def.Kind == codes.KindMethod) {
		return ""
	}
	_, ignored := ReturnsIgnored[k]
	_, unverified := ReturnsNotVerified[k]
	for _, r := range MemberReturnsNotVerified[k] {
		unverified = unverified || r == recv
	}
	sig, found := memberSignature(def, recv)
	if ignored || unverified || !found || len(sig.Return.Text) != 1 {
		return ""
	}
	return MemberReturn(sig.Return.Text[0], objType)
}

// MemberReturn is the type a member's return text (definitions.json, 13.9) has on a receiver of type
// recvType: the receiver's type parameters (T of TR_List[T]; K, V of TR_Dict[K, V]) are its element
// types. A type parameter the receiver does not give is TR_Any; a container with an unknown element is
// written without it (TR_List). A type the checker does not know (a library type) is "".
func MemberReturn(text, recvType string) string {
	ref, err := typeTextRef(text)
	if err != nil {
		return ""
	}
	vars := map[string]string{}
	switch s := shapeOf(recvType); {
	case s.base == typeList && len(s.args) == 1:
		vars["T"] = s.args[0]
	case s.base == typeDict && len(s.args) == 2:
		vars["K"], vars["V"] = s.args[0], s.args[1]
	case s.base == typeTask && len(s.args) == 1:
		vars["T"] = s.args[0] // TR_Task[T].TR_wait() T verir (10.1)
	}
	t := boundType(ref, vars)
	if t == typeAny || !coreTypes[baseType(requiredType(t))] {
		return ""
	}
	return t
}

// boundType tip yazımının denetleyicideki metnidir; tip parametreleri vars'tan (yoksa TR_Any).
func boundType(t *ast.TypeRef, vars map[string]string) string {
	if t.Optional {
		inner := *t
		inner.Optional = false
		return optionalType(boundType(&inner, vars))
	}
	if t.Return != nil {
		return typeFunction
	}
	if len(t.Elems) >= 2 {
		elems := make([]string, len(t.Elems))
		for i := range t.Elems {
			elems[i] = boundType(&t.Elems[i], vars)
		}
		return tupleType(elems)
	}
	if codes.TypeParameter(t.Name) {
		if v, ok := vars[t.Name]; ok {
			return v
		}
		return typeAny
	}
	args := make([]string, len(t.Args))
	for i := range t.Args {
		if args[i] = boundType(&t.Args[i], vars); args[i] == typeAny {
			return normalizeTypeName(t.Name) // öğe tipi bilinmeyen kap (TR_List)
		}
	}
	return containerType(normalizeTypeName(t.Name), args)
}

var typeTextRefs sync.Map // tip metni → *ast.TypeRef ya da error

// typeTextRef tanım dosyasının tip metnini dilin tip ayrıştırıcısıyla okur (ast.ParseTypeText: ayrıştırıcının
// kaydettiği parser.ParseTypeText; denetleyici ayrıştırıcıyı içe aktarmaz), bir kez.
func typeTextRef(text string) (*ast.TypeRef, error) {
	if v, ok := typeTextRefs.Load(text); ok {
		if err, bad := v.(error); bad {
			return nil, err
		}
		return v.(*ast.TypeRef), nil
	}
	ref, err := ast.ParseTypeText(text)
	if err != nil {
		typeTextRefs.Store(text, err)
		return nil, err
	}
	typeTextRefs.Store(text, ref)
	return ref, nil
}
