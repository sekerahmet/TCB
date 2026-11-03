package checker

import (
	"strings"

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
// ya da tip adı, ya da tipi bilinen kullanıcı yapısı olmayan alıcının kodla yazılmış üyesi.
func (c *Checker) builtinCallCode(e *ast.CallExpression, objType string) codes.Code {
	switch f := e.Function.(type) {
	case *ast.Identifier:
		if _, user := c.scope.Resolve(f.Value); !user {
			return f.Token.Code
		}
	case *ast.MemberExpression:
		if !f.Safe && !c.knownStruct(objType) && f.NameToken.Code.Namespace() == codes.NamespaceMember {
			return f.NameToken.Code
		}
	}
	return codes.None
}

// ReturnsNotVerified are single-return builtins whose return the engine does not give in the
// definition's example, or whose example does not let the test see it
// (stdlib.TestSingleReturnsMatchEngine): the checker does not use their return.
var ReturnsNotVerified = map[codes.Code]string{}

// ReturnsIgnored are builtins whose return the definition and the engine agree on but 13.10 says
// is meaningless ("no meaningless return"): the checker does not use it until the return is removed.
var ReturnsIgnored = map[codes.Code]string{
	codes.FnPrint: "13.10: TR_print returns no value; its TR_String return is to be removed",
}

// builtinReturnType yerleşik çağrısının tipidir (1.2, 13.9): tek tipli dönüş (SingleReturn) denetleyicinin
// tanıdığı çekirdek tipse o tip (TR_BuiltinFunction TR_Function'dır, 13.9); kapsa öğe tipsiz (TR_List: imzalarda öğe tipi henüz yok). Dönüşü
// motorla doğrulanmamış yerleşik (ReturnsNotVerified), birleşim ve tanınmayan tip "" (TR_Any).
func (c *Checker) builtinReturnType(e *ast.CallExpression, objType string) string {
	k := c.builtinCallCode(e, objType)
	if t := c.jsonCallType(e, k); t != "" {
		return t // TR_JSON'u veren yerleşik (json_values.go)
	}
	if t := c.waitAllType(e, k); t != "" {
		return t // görev listesini bekleyen TR_wait_all (task_types.go)
	}
	if t := c.extremeType(e, k); t != "" {
		return t
	}
	_, ignored := ReturnsIgnored[k]
	if _, unverified := ReturnsNotVerified[k]; unverified || ignored || k == codes.None {
		return ""
	}
	ret, ok := SingleReturn(k)
	if ok && ret == codes.TypeBuiltinFunction {
		return typeFunction // yerleşik fonksiyon bir fonksiyon değeridir (13.9)
	}
	if ok && coreTypes[codes.Name(ret)] {
		return codes.Name(ret)
	}
	return ""
}

// extremeType TR_max ve TR_min'in sayı çağrısının tipidir (13.1, işlecin tipi, 4.2): değerler (ya da
// tek liste argümanının öğeleri) hep TR_Int ise TR_Int, aralarında TR_Float varsa TR_Float
// (TR_max(2, 1.5) → 2.0). Adlı argüman (tensör), tek sayı, anahtar fonksiyonu, sayı olmayan ya da
// tipi bilinmeyen değer ""; çağrı TR_max ya da TR_min değilse de "".
func (c *Checker) extremeType(e *ast.CallExpression, k codes.Code) string {
	if k != codes.FnMax && k != codes.FnMin || len(e.NamedArgs) != 0 || len(e.Arguments) == 0 {
		return ""
	}
	types := make([]string, len(e.Arguments))
	for i, a := range e.Arguments {
		types[i] = c.itemTypes[a]
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
