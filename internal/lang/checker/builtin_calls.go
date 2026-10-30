package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// countedReceivers argüman sayısı denetlenen alıcı tipleridir: çekirdek metotların (Metin,
// Liste, Sözlük, Aralık) sayısının tanım dosyasıyla aynı olduğu sınanır
// (interp.TestMethodCountsMatchDefinitions); öbür tiplerin metotları henüz sınanmadı.
var countedReceivers = map[codes.Code]bool{codes.TypeString: true, codes.TypeList: true, codes.TypeDict: true, codes.TypeRange: true}

// MembersBeyondDefinitions are members whose "types" in definitions.json leave out types the engine
// supports them on (member → the types left out) (stdlib.TestMemberTypesMatchDefinitions, 2026-10-30); the missing-member check
// (13.11) does not trust their types until the definitions are fixed.
var MembersBeyondDefinitions = map[codes.Code][]codes.Code{
	codes.MemGet:    {codes.TypeJSON, codes.TypeBytes},
	codes.MemKeys:   {codes.TypeJSON},
	codes.MemLength: {codes.TypeJSON, codes.TypeBytes},
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

// checkMemberCall tipi bilinen alıcıdaki yerleşik metot çağrısını denetler (13.11): metot alıcının
// tipinde yoksa (tanımın "types"ı; reportMissingMember) ya da konumlu
// argümanların sayısı imzaya (tanım dosyası: zorunlu, '?' isteğe bağlı, '*' ve '+' sınırsız)
// uymuyorsa çalışma anındaki hatanın kendisi (H2011, hint.MemberArgCount): ad ve kullanım
// kimlikle; .TR_sort()'a verilen anahtar için bugünkü yol. Adlı argümanlar 13.5'in işidir.
func (c *Checker) checkMemberCall(e *ast.CallExpression, objType string, argTypes []string) {
	m, _ := e.Function.(*ast.MemberExpression)
	k, recv, def, ok := memberOf(m, objType)
	if !ok {
		return
	}
	sig, found := memberSignature(def, recv)
	if !found {
		if _, untrusted := MembersBeyondDefinitions[k]; !untrusted {
			c.reportMissingMember(e, m, k, def, objType, argTypes)
		}
		return
	}
	if !countedReceivers[recv] {
		return
	}
	given := len(e.Arguments)
	if minN, maxN := sig.PositionalCounts(); given < minN || maxN >= 0 && given > maxN {
		c.addErrorAt(e, hint.MemberArgCount(recv, k, minN, maxN, given))
	}
}

// reportMissingMember alıcının tipinde olmayan yerleşik metodu bildirir (13.10, 13.11): çalışma
// anındaki hatanın kendisi (H2474), ipucu metodun hangi tiplerde olduğunu söyler (H1117). Genel
// kural, yer değiştirme: metot tek argümanın tipinde varsa ve o tipte tek argüman alıyorsa ipucu
// alıcıyla argümanın yerini değiştirir: " ".TR_join(l) → l.TR_join(" ").
func (c *Checker) reportMissingMember(e *ast.CallExpression, m *ast.MemberExpression, k codes.Code, def codes.Definition, objType string, argTypes []string) {
	swap := ""
	if len(e.Arguments) == 1 && len(e.NamedArgs) == 0 {
		if t, ok := codes.Find(codes.NamespaceType, baseType(argTypes[0])); ok && coreTypes[baseType(argTypes[0])] && !shapeOf(argTypes[0]).optional {
			sig, found := memberSignature(def, t)
			minN, maxN := sig.PositionalCounts()
			recvText, ok1 := c.exprText(m.Object)
			argText, ok2 := c.exprText(e.Arguments[0])
			if found && minN <= 1 && (maxN < 0 || maxN >= 1) && ok1 && ok2 {
				swap = argText + "." + codes.Spelling(k) + "(" + recvText + ")"
			}
		}
	}
	var types typeChoices
	for _, t := range def.Types() {
		types = append(types, c.typeArg(codes.Name(t)))
	}
	c.addErrorAt(e, messages.New(messages.H2474, c.typeArg(objType), m.Property,
		messages.New(messages.H1117, c.word(k), types, swap)))
}

// typeChoices tiplerin iletideki adlarıdır (typeArg), "ya da" ile (messages.Localized).
type typeChoices []messages.Localized

func (t typeChoices) MessageText(lang string) string {
	names := make([]string, len(t))
	for i, k := range t {
		names[i] = k.MessageText(lang)
	}
	return messages.OrListIn(lang, names...)
}
