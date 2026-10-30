package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// SingleReturn yerleşiğin tek tipli dönüşüdür (1.2, 13.9): tanım dosyasında dönüşü tek bir tip olan
// fonksiyon, bütün alıcılarında aynı tek tipi döndüren metot ve tip çağrısı (3.12, dönüşü tipin
// kendisi). Birleşim (TR_Int|TR_Float), TR_Any, yokluk (değer döndürmeyen yerleşik) ve kitaplık kodu
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
	if len(t.Types) != 1 || t.Optional || t.Types[0] == codes.TypeAny || t.Types[0] == codes.TypeNoneType {
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
var ReturnsNotVerified = map[codes.Code]string{
	codes.FnJsonParse: "13.8: the engine gives another type than the definition",
}

// builtinReturnType yerleşik çağrısının tipidir (1.2, 13.9): tek tipli dönüş (SingleReturn) denetleyicinin
// tanıdığı çekirdek tipse o tip (TR_BuiltinFunction TR_Function'dır, 13.9); kapsa öğe tipsiz (TR_List: imzalarda öğe tipi henüz yok). Dönüşü
// motorla doğrulanmamış yerleşik (ReturnsNotVerified), birleşim ve tanınmayan tip "" (TR_Any).
func (c *Checker) builtinReturnType(e *ast.CallExpression, objType string) string {
	k := c.builtinCallCode(e, objType)
	if _, unverified := ReturnsNotVerified[k]; unverified || k == codes.None {
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
