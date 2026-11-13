package checker

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// SingleReturn yerleşiğin tek tipli dönüşüdür (1.2, 13.9): tanım dosyasında dönüşü tek bir tip olan
// fonksiyon, bütün alıcılarında aynı tek tipi döndüren metot ve tip çağrısı (3.12, dönüşü tipin
// kendisi). Birleşim (TR_Int|TR_Float), isteğe bağlı tip (T?), TR_Any, yokluk (değer döndürmeyen yerleşik) ve kitaplık kodu
// (imzası kaynaktadır) tek tipli değildir.
func SingleReturn(k codes.Code) (codes.Code, bool) {
	def, ok := codes.DefinitionOf(k)
	if !ok || k.IsLibrary() {
		return codes.None, false
	}
	var ret codes.Code
	switch def.Kind {
	case codes.KindFunction, codes.KindType:
		ret, ok = singleType(def.Signature.Return)
	case codes.KindMethod:
		for i, u := range def.Members {
			r, single := singleType(u.Signature.Return)
			if !single || i > 0 && r != ret {
				return codes.None, false
			}
			ret, ok = r, true
		}
	default:
		return codes.None, false
	}
	return ret, ok
}

func singleType(t codes.TypeExpr) (codes.Code, bool) {
	if len(t.Types) != 1 || t.Optional || t.Types[0] == codes.TypeAny || t.Types[0] == codes.TypeNoneType || strings.HasSuffix(t.Text[0], "?") {
		return codes.None, false
	}
	return t.Types[0], true
}

// builtinCallCode çağrılan yerleşiğin kodudur: kodla yazılmış, kullanıcının gölgelemediği fonksiyon
// ya da tip adı, ya da tipi bilinen kullanıcı yapısı olmayan alıcının kodla yazılmış üyesi. Alıcı
// yerleşik tipte olmayan bir yapıysa (kitaplık tipi ya da aktarılan modülün yapısı) üyesi yerleşik
// değildir: aynı adlı yerleşik üyenin imzası ona geçmez (D1 Gemma 3, Teslim D).
func (c *Checker) builtinCallCode(e *ast.CallExpression, objType string) codes.Code {
	switch f := e.Function.(type) {
	case *ast.Identifier:
		if _, user := c.scope.Resolve(f.Value); !user {
			return f.Token.Code
		}
	case *ast.MemberExpression:
		if !f.Safe && !c.knownStruct(objType) && !structReceiver(objType) && f.NameToken.Code.Namespace() == codes.NamespaceMember {
			return f.NameToken.Code
		}
	}
	return codes.None
}

// structReceiver t'nin yerleşik tipte olmayan bir yapı olduğudur: kod listesinde modülü yazılı tip
// (TR_ChatTokens) ya da TR_ ile başlamayan düz ad (aktarılan modülün yapısı). TR_Any ve öbürleri değil.
func structReceiver(t string) bool {
	if k, err := codes.Parse(t); err == nil && k != codes.None {
		return k.IsLibrary()
	}
	return t != "" && !strings.HasPrefix(t, codes.IdentityPrefix) && strings.IndexFunc(t, func(r rune) bool {
		return r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) < 0
}

// libraryType t'nin kod listesinde modülü yazılı bir tip olduğudur (TR_type_ChatTokens).
func libraryType(t string) bool {
	k, err := codes.Parse(t)
	return err == nil && k.IsLibrary() && k.Namespace() == codes.NamespaceType
}

// libraryTypeCall kitaplık tipinin kurucusunun çağrısının tipidir: tipin kendisi (TR_ChatTokens(...)).
func libraryTypeCall(e *ast.CallExpression) string {
	if f, ok := e.Function.(*ast.Identifier); ok && f.Token.Code.IsLibrary() && f.Token.Code.Namespace() == codes.NamespaceType {
		return codes.Name(f.Token.Code)
	}
	return ""
}

// ReturnsNotVerified are single-return builtins whose return the engine does not give in the
// definition's example, or whose example does not let the test see it
// (stdlib.TestSingleReturnsMatchEngine): the checker does not use their return.
var ReturnsNotVerified = map[codes.Code]string{}

// builtinReturnType yerleşik çağrısının tipidir (1.2, 13.9): tek tipli dönüş (SingleReturn) denetleyicinin
// tanıdığı çekirdek tipse o tip (TR_BuiltinFunction TR_Function'dır, 13.9); fonksiyonun dönüşünde yazılı öğe tipi tipe girer. Dönüşü
// motorla doğrulanmamış yerleşik (ReturnsNotVerified), birleşim ve tanınmayan tip "" (TR_Any).
func (c *Checker) builtinReturnType(e *ast.CallExpression, objType string) string {
	k := c.builtinCallCode(e, objType)
	if t := libraryTypeCall(e); t != "" {
		return t
	}
	if t := c.jsonCallType(e, k); t != "" {
		return t // TR_JSON'u veren yerleşik (json_values.go)
	}
	if t := c.waitAllType(e, k); t != "" {
		return t // görev listesini bekleyen TR_wait_all (task_types.go)
	}
	if t := c.extremeType(e, k); t != "" {
		return t
	}
	if t := c.roundType(e, k); t != "" {
		return t
	}
	if _, unverified := ReturnsNotVerified[k]; unverified || k == codes.None {
		return ""
	}
	if t := optionalReturn(k); t != "" {
		return t
	}
	ret, ok := SingleReturn(k)
	if ok && ret == codes.TypeBuiltinFunction {
		return typeFunction // yerleşik fonksiyon bir fonksiyon değeridir (13.9)
	}
	if def, _ := codes.DefinitionOf(k); ok && def.Kind == codes.KindFunction {
		if t := MemberReturn(def.Signature.Return.Text[0], ""); t != "" {
			return t // öğe tipi yazılı dönüş: TR_List[TR_List[TR_String]]
		}
	}
	if ok && coreTypes[codes.Name(ret)] {
		return codes.Name(ret)
	}
	return ""
}

// extremeType TR_max ve TR_min'in sayı çağrısının tipidir (13.1, işlecin tipi, 4.2): değerler (ya da
// tek liste argümanının öğeleri) hep TR_Int ise TR_Int, aralarında TR_Float varsa TR_Float
// (TR_max(2, 1.5) → 2.0). Anahtar fonksiyonlu biçim (liste ve fonksiyon) listenin öğe tipini verir:
// sonuç listenin bir öğesidir. Adlı argüman (tensör), tek sayı, sayı olmayan ya da tipi bilinmeyen
// değer ""; çağrı TR_max ya da TR_min değilse de "".
func (c *Checker) extremeType(e *ast.CallExpression, k codes.Code) string {
	if k != codes.FnMax && k != codes.FnMin || len(e.NamedArgs) != 0 || len(e.Arguments) == 0 {
		return ""
	}
	types := make([]string, len(e.Arguments))
	for i, a := range e.Arguments {
		types[i] = c.itemTypes[a]
	}
	if list := shapeOf(types[0]); len(types) == 2 && list.base == typeList && !list.optional && len(list.args) == 1 &&
		shapeOf(types[1]).base == typeFunction {
		return list.args[0]
	}
	if len(types) == 1 {
		list := shapeOf(types[0])
		if list.base != typeList || list.optional || len(list.args) != 1 {
			return ""
		}
		types = list.args
	}
	result := typeInt
	for _, t := range types {
		switch t {
		case typeInt:
		case typeFloat:
			result = typeFloat
		default:
			return ""
		}
	}
	return result
}

// roundType TR_round'un sayı çağrısının tipidir (13.1): tek argümanla TR_Int, basamakla girdinin tipi
// (TR_round(5, 2) → TR_Int, TR_round(3.7, 0) → TR_Float). Girdi sayı değilse (tensör) ya da tipi
// bilinmiyorsa ""; çağrı TR_round değilse de "".
func (c *Checker) roundType(e *ast.CallExpression, k codes.Code) string {
	if k != codes.FnRound || len(e.NamedArgs) != 0 || len(e.Arguments) == 0 || len(e.Arguments) > 2 {
		return ""
	}
	t := c.itemTypes[e.Arguments[0]]
	switch {
	case !numericType(t):
		return ""
	case len(e.Arguments) == 1:
		return typeInt
	}
	return t
}

// optionalReturn dönüşü tek bir isteğe bağlı tip olan yerleşik fonksiyonun tipidir (TR_input
// TR_String? verir, 13.1): değer asıl tipin yerinde kullanılamaz, önce açılır (3.5). Başka dönüş "".
func optionalReturn(k codes.Code) string {
	def, ok := codes.DefinitionOf(k)
	if !ok || k.IsLibrary() || def.Kind != codes.KindFunction {
		return ""
	}
	r := def.Signature.Return
	if len(r.Types) != 1 || !(r.Optional || strings.HasSuffix(r.Text[0], "?")) {
		return ""
	}
	return MemberReturn(r.Text[0], "")
}
