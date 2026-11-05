package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_JSON (3.7a) denetleyicinin tanıdığı bir çekirdek tiptir: TR_json_parse(m) ve TR_JSON(x) bir
// TR_JSON verir; j["k"] ve j[i] yine TR_JSON; TR_List(j) TR_List[TR_JSON], TR_Dict(j)
// TR_Dict[TR_String, TR_JSON]; .TR_get(k) TR_JSON?, .TR_keys() TR_List[TR_String], .TR_length TR_Int.
// Motorun TR_Type'ı JSON değerinin türünü verir (TR_Int, TR_List[TR_JSON] ...), TR_JSON'u hiç vermez: bu yüzden TR_JSON veren yerleşikler
// TestSingleReturnsMatchEngine'de değil, TestJSONReturnsAreJSONValues'ta (stdlib) doğrulanır.

// JSONReturns are builtins whose value is a TR_JSON value (3.7a): the engine's TR_Type gives the JSON
// kind, never TR_JSON, so stdlib.TestJSONReturnsAreJSONValues checks that the value is a JSON value.
var JSONReturns = map[codes.Code]string{
	codes.FnJsonParse:    "3.7a, 13.8: parsing gives a structured value",
	codes.FnLoadMetadata: "15.55: GGUF metadata is structured",
	codes.TypeJSON:       "3.7a: the type call converts its argument",
}

// jsonCallType k yerleşiğinin çağrısının TR_JSON'a bağlı tipidir: TR_JSON veren yerleşikte TR_JSON,
// TR_JSON argümanla TR_List(j)'de TR_List[TR_JSON], TR_Dict(j)'de TR_Dict[TR_String, TR_JSON]
// (3.7a "Açmak"); öbüründe "".
func (c *Checker) jsonCallType(e *ast.CallExpression, k codes.Code) string {
	if _, ok := JSONReturns[k]; ok {
		return typeJSON
	}
	if len(e.Arguments) != 1 || len(e.NamedArgs) != 0 || c.itemTypes[e.Arguments[0]] != typeJSON {
		return ""
	}
	switch k {
	case codes.TypeList:
		return listType(typeJSON)
	case codes.TypeDict:
		return containerType(typeDict, []string{typeString, typeJSON})
	}
	return ""
}

// jsonMemberType TR_JSON alıcıdaki üyenin değerinin tipidir (3.7a "Okumak"): .TR_get(k) TR_JSON?,
// .TR_keys() TR_List[TR_String], .TR_length TR_Int; öbür üye ve çağrısız metot "".
func jsonMemberType(m *ast.MemberExpression, called bool) string {
	if m.Safe {
		return ""
	}
	switch k := m.NameToken.Code; {
	case k == codes.MemGet && called:
		return optionalType(typeJSON)
	case k == codes.MemKeys && called:
		return listType(typeString)
	case k == codes.MemLength && !called:
		return typeInt
	}
	return ""
}

// checkJSONConversion TR_JSON(x)'in argümanını denetler (3.7a "Kurmak"): çevrilen değer ve öğeleri
// yalnız metin, sayı, mantık değeri, TR_none, liste, metin anahtarlı sözlük ve TR_JSON olabilir; tipi
// bilinen öbürü (demet, tensör, yapı, fonksiyon, aralık, tip, bayt dizisi, metin olmayan anahtarlı
// sözlük) çalışma anındaki hatanın kendisidir (H2654). Tipi bilinmeyen değer ve öğe çalışma anında
// denetlenir; TR_inf ve TR_nan (H2653) denetimde bilinmez.
func (c *Checker) checkJSONConversion(arg ast.Expression, argType string) {
	if bad, t := c.notJSON(arg, argType); bad != nil {
		c.addErrorAt(bad, messages.New(messages.H2654, c.typeArg(t)))
	}
}

// notJSON value'nun (tipi t) ya da değişmezse bir öğesinin JSON'a giremeyen, tipi bilinen değer
// olduğunu söyler; döndürdüğü ifade ve tip o değerindir.
func (c *Checker) notJSON(value ast.Expression, t string) (ast.Expression, string) {
	if !c.jsonKind(t) {
		return value, t
	}
	switch v := value.(type) {
	case *ast.ListLiteral:
		for _, el := range v.Elements {
			if bad, bt := c.notJSON(el, c.itemTypes[el]); bad != nil {
				return bad, bt
			}
		}
	case *ast.DictLiteral:
		for _, p := range v.Pairs {
			if kt := c.itemTypes[p.Key]; kt != typeString && c.knownType(kt) {
				return p.Key, kt
			}
			if bad, bt := c.notJSON(p.Value, c.itemTypes[p.Value]); bad != nil {
				return bad, bt
			}
		}
	}
	return nil, ""
}

// checkJSONLiteral beklenen tipi TR_JSON olan liste ya da sözlük değişmezinin öğelerini denetler
// (3.7a "Kurmak"), iç içe değişmezlerinkini de, her değişmezi bir kez. Değişmez olmayan liste ya da
// sözlük (l, n.alan, f()) JSON değeri olarak kurulmaz: H1133 kopyasını çevirmeyi önerir
// ({"a": TR_JSON(l)}). JSON'a giremeyen, tipi bilinen öğe (tensör, demet, yapı; öğesi giremeyen
// liste) çalışma anındaki hatanın kendisidir (H2654); TR_JSON(t) de onu verdiği için önerisi yoktur.
func (c *Checker) checkJSONLiteral(value ast.Expression) {
	if c.jsonLiterals == nil {
		c.jsonLiterals = map[ast.Expression]bool{}
	}
	if c.jsonLiterals[value] {
		return
	}
	c.jsonLiterals[value] = true
	var items []ast.Expression
	switch v := value.(type) {
	case *ast.ListLiteral:
		items = v.Elements
	case *ast.DictLiteral:
		for _, p := range v.Pairs {
			items = append(items, p.Value)
		}
	}
	for _, el := range items {
		t := c.itemTypes[el]
		switch base := normalizeTypeName(baseType(requiredType(t))); {
		case literalValue(el) && shapeOf(t).elems == nil:
			c.checkJSONLiteral(el)
		case !c.jsonKind(t):
			c.addErrorAt(el, messages.New(messages.H2654, c.typeArg(t)))
		case base == typeList || base == typeDict:
			text, _ := c.exprText(el)
			c.addErrorAt(el, messages.New(messages.H1133, text, c.typeArg(t)))
		}
	}
}

// jsonKind t tipinde bir değer JSON'a girebilir mi: JSON'un türleri, onların isteğe bağlısı ve
// kabı; tipi bilinmeyen (TR_Any, öğe tipsiz kap, bilinmeyen ad) de girebilir sayılır.
func (c *Checker) jsonKind(t string) bool {
	s := shapeOf(t)
	if s.elems != nil {
		return false // demet (3.6a)
	}
	switch base := normalizeTypeName(s.base); {
	case base == typeInt, base == typeFloat, base == typeString, base == typeBool, base == typeNull, base == typeJSON, base == typeAny:
		return true
	case base == typeList:
		return len(s.args) != 1 || c.jsonKind(s.args[0])
	case base == typeDict:
		return len(s.args) != 2 || (s.args[0] == typeString || !c.knownType(s.args[0])) && c.jsonKind(s.args[1])
	case coreTypes[base] || c.knownStruct(base):
		return false
	}
	return true
}

// checkJSONIndexAssign TR_JSON değerine indeksle atamayı denetler (3.7a "Değişmez"): j["k"] = 2
// çalışma anının hatasıdır (H2403); değiştirmek için önce açılır.
func (c *Checker) checkJSONIndexAssign(s *ast.AssignStatement) {
	if ix, ok := s.Target.(*ast.IndexExpression); ok && c.itemTypes[ix.Left] == typeJSON {
		c.addErrorAt(s, messages.New(messages.H2403, c.typeArg(typeJSON)))
	}
}
