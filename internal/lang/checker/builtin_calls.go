package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// countedReceivers argüman sayısı denetlenen alıcı tipleridir: çekirdek metotların (Metin,
// Liste, Sözlük, Aralık) sayısının tanım dosyasıyla aynı olduğu sınanır
// (interp.TestMethodCountsMatchDefinitions); öbür tiplerin metotları henüz sınanmadı.
var countedReceivers = map[codes.Code]bool{codes.TypeString: true, codes.TypeList: true, codes.TypeDict: true, codes.TypeRange: true}

// MembersBeyondDefinitions are members whose "types" in definitions.json leave out types the engine
// supports them on (stdlib.TestMemberTypesMatchDefinitions, 2026-10-30); the missing-member check
// (13.11) does not trust their types until the definitions are fixed.
var MembersBeyondDefinitions = map[codes.Code]string{
	codes.MemGet:    "the engine also supports it on JSON and Bytes",
	codes.MemKeys:   "the engine also supports it on JSON",
	codes.MemLength: "the engine also supports it on JSON and Bytes",
}

// memberOf tipi bilinen alıcıda çağrılan yerleşik metodun kodu, alıcının tip kodu ve tanımıdır
// (13.9): alıcı isteğe bağlı değil, tipi bir çekirdek tip (denetleyicinin tanıdığı; kullanıcı
// yapısı değil) ve metot kodla yazılmış, kitaplığın olmayan bir üyedir. ok false ise kural uygulanmaz.
func memberOf(m *ast.MemberExpression, objType string) (k, recv codes.Code, def codes.Definition, ok bool) {
	if m == nil || m.Safe {
		return
	}
	k = m.NameToken.Code
	s := shapeOf(objType)
	if k.Namespace() != codes.NamespaceMember || k.IsLibrary() || s.optional || !coreTypes[s.base] {
		return
	}
	recv, found := codes.Find(codes.NamespaceType, s.base)
	if !found {
		return
	}
	def, found = codes.DefinitionOf(k)
	return k, recv, def, found && def.Kind == codes.KindMethod
}

// memberSignature üyenin recv tipindeki imzasıdır; üye o tipte yoksa ok false.
func memberSignature(def codes.Definition, recv codes.Code) (codes.Signature, bool) {
	for _, u := range def.Members {
		if u.Type == recv {
			return u.Signature, true
		}
	}
	return codes.Signature{}, false
}

// checkMemberCall tipi bilinen alıcıdaki yerleşik metot çağrısını denetler (13.11): konumlu
// argümanların sayısı imzaya (tanım dosyası: zorunlu, '?' isteğe bağlı, '*' ve '+' sınırsız)
// uymuyorsa çalışma anındaki hatanın kendisi (H2011, hint.MemberArgCount): ad ve kullanım
// kimlikle; .TR_sort()'a verilen anahtar için bugünkü yol. Adlı argümanlar 13.5'in işidir.
func (c *Checker) checkMemberCall(e *ast.CallExpression, objType string) {
	m, _ := e.Function.(*ast.MemberExpression)
	k, recv, def, ok := memberOf(m, objType)
	if !ok {
		return
	}
	sig, found := memberSignature(def, recv)
	if !found || !countedReceivers[recv] {
		return
	}
	given := len(e.Arguments)
	if minN, maxN := sig.PositionalCounts(); given < minN || maxN >= 0 && given > maxN {
		c.addErrorAt(e, hint.MemberArgCount(recv, k, minN, maxN, given))
	}
}
