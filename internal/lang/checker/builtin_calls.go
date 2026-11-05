package checker

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// countedReceivers argüman sayısı denetlenen alıcı tipleridir: çekirdek metotların (Metin,
// Liste, Sözlük, Aralık) sayısının tanım dosyasıyla aynı olduğu sınanır
// (interp.TestMethodCountsMatchDefinitions); öbür tiplerin metotları henüz sınanmadı.
var countedReceivers = map[codes.Code]bool{codes.TypeString: true, codes.TypeList: true, codes.TypeDict: true, codes.TypeRange: true, codes.TypeTask: true}

// MembersBeyondDefinitions are members whose "types" in definitions.json leave out types the engine
// supports them on (member → the types left out) (stdlib.TestMemberTypesMatchDefinitions, 2026-10-30); the missing-member check
// (13.11) does not trust their types until the definitions are fixed.
var MembersBeyondDefinitions = map[codes.Code][]codes.Code{
	codes.MemGet:    {codes.TypeJSON, codes.TypeBytes},
	codes.MemKeys:   {codes.TypeJSON},
	codes.MemLength: {codes.TypeJSON, codes.TypeBytes},
}

// memberOf tipi bilinen alıcıda çağrılan yerleşik metodun ya da özelliğin kodu, alıcının tip kodu ve
// tanımıdır (13.9): alıcı isteğe bağlı değil, tipi bir çekirdek tip (denetleyicinin tanıdığı; kullanıcı
// yapısı değil) ve üye kodla yazılmış, kitaplığın olmayan bir üyedir. ok false ise kural uygulanmaz.
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
	return k, recv, def, found && (def.Kind == codes.KindMethod || def.Kind == codes.KindProperty)
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
	if def.Kind == codes.KindProperty {
		if found {
			c.checkPropertyCall(e, sig)
		}
		return
	}
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
		return
	}
	if !slices.Contains(ParamsNotVerified[k], recv) {
		c.checkMemberArgTypes(e, k, sig, argTypes)
	}
}

// checkMemberArgTypes çekirdek metodun konumlu argümanlarının tipini imzaya göre denetler (13.11):
// tek tipli parametreye (ParamType) uymayan, tipi bilinen argüman çalışma anındaki hatanın kendisidir
// (H2423). Tipi bilinmeyen, TR_JSON (çalışma anında H2665, açma önerisiyle) ve fonksiyon beklenen yere
// verilen tip (13.9'un ok önerisi, sonraki iş) denetlenmez.
func (c *Checker) checkMemberArgTypes(e *ast.CallExpression, k codes.Code, sig codes.Signature, argTypes []string) {
	for i, arg := range e.Arguments {
		p, ok := paramAt(sig, i)
		want, single := ParamType(p)
		given := argTypes[i]
		if ok && k == codes.MemAppend && p.Text[0] == "T" {
			c.checkElementArg(e, i, given)
			continue
		}
		if k == codes.MemContains && shapeOf(c.receivers[e]).base == typeList {
			c.checkContainsArg(e, given) // 3.6, 13.10 (element_types.go)
			continue
		}
		if !ok || !single || given == typeAny || given == codes.Name(codes.TypeJSON) || want == codes.TypeFunction && given == typeType {
			continue
		}
		if !c.assignable(codes.Name(want), given, arg) {
			c.addErrorAt(arg, hint.MemberArgType(k, i+1, c.typeArg(codes.Name(want)), c.typeArg(given), c.memberArgHint(k, codes.Name(want), given, arg)))
		}
	}
}

// memberArgHint openHint'tir; .TR_split'in ayırıcısının varsayılanı boş metin olamaz (H2635).
func (c *Checker) memberArgHint(k codes.Code, expected, actual string, value ast.Expression) any {
	if k == codes.MemSplit {
		return c.openHintAs("separator", expected, actual, value)
	}
	return c.openHint(expected, actual, value)
}

// checkElementArg listeye eklenen değerin tipini listenin öğe tipine göre denetler (3.6: liste tek
// tiplidir; imzada T): öğe tipi bilinen listeye uymayan tipte değer H1123'tür. Çalışma anı öğe tipini
// taşımadığı için bu hatanın çalışma anında kodu yoktur.
func (c *Checker) checkElementArg(e *ast.CallExpression, i int, given string) {
	recvType := c.receivers[e]
	s := shapeOf(recvType)
	if s.base != typeList || len(s.args) != 1 || s.optional || given == typeAny || s.args[0] == typeAny {
		return
	}
	if !c.assignable(s.args[0], given, e.Arguments[i]) {
		c.addErrorAt(e.Arguments[i], messages.New(messages.H1123, i+1, c.typeArg(s.args[0]), c.typeArg(recvType), c.typeArg(given),
			c.openHint(s.args[0], given, e.Arguments[i])))
	}
}

// paramAt i. konumlu argümanın parametresidir; sondaki '*' ya da '+' parametresi kalan argümanları alır.
func paramAt(sig codes.Signature, i int) (codes.TypeExpr, bool) {
	if i < len(sig.Params) {
		return sig.Params[i], true
	}
	if n := len(sig.Params); n > 0 && (sig.Params[n-1].Many || sig.Params[n-1].AtLeastOne) {
		return sig.Params[n-1], true
	}
	return codes.TypeExpr{}, false
}

// checkPropertyCall özelliğin çağrılmasını denetler (13.11): özellik bir değerdir, çağrılmaz
// ([1].TR_length(1)). Değerinin tipi tek ve fonksiyon olmayan bir çekirdek tipse çalışma anındaki
// hatanın kendisidir (H2421, değerin tipiyle); tipi bilinmeyen ya da fonksiyon olabilen değer
// ([f].TR_first()) çağrılabilir, denetlenmez.
func (c *Checker) checkPropertyCall(e *ast.CallExpression, sig codes.Signature) {
	ret, ok := singleType(sig.Return)
	if !ok || ret == codes.TypeFunction || ret == codes.TypeBuiltinFunction || !coreTypes[codes.Name(ret)] {
		return
	}
	c.addErrorAt(e, messages.New(messages.H2421, c.typeArg(codes.Name(ret)), ""))
}

// reportMissingMember alıcının tipinde olmayan yerleşik metodu bildirir (13.10, 13.11): çalışma
// anındaki hatanın kendisi (H2474), ipucu metodun hangi tiplerde olduğunu söyler (H1117). Genel
// kural, yer değiştirme: metot tek argümanın tipinde varsa ve o tipte tek argüman alıyorsa ipucu
// alıcıyla argümanın yerini değiştirir: " ".TR_join(l) → l.TR_join(" ").
func (c *Checker) reportMissingMember(e *ast.CallExpression, m *ast.MemberExpression, k codes.Code, def codes.Definition, objType string, argTypes []string) {
	if k == codes.MemRemove && shapeOf(objType).base == typeList {
		c.addErrorAt(e, messages.New(messages.H2535, c.typeArg(objType), c.word(k))) // listede öğe konumuyla çıkar (13.3)
		return
	}
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
	c.addErrorAt(e, messages.New(messages.H2474, c.typeArg(objType), c.word(k),
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

// ParamsNotVerified are core members whose single-typed parameters in definitions.json the engine
// does not check with the same type on a receiver (member → the receivers;
// interp.TestParamsMatchEngineKinds): the checker does not check their argument types there.
var ParamsNotVerified = map[codes.Code][]codes.Code{
	// tanımın ilk parametresi isteğe bağlı (["TR_Any?", "TR_Function"]): tek argüman fonksiyondur, motor denetlemez
	codes.MemReduce: {codes.TypeList},
}

// ParamType is the type a positional parameter of a core member's signature asks for when the checker
// checks it (13.11): one type alternative, not TR_Any, not a type parameter, no element type.
func ParamType(p codes.TypeExpr) (codes.Code, bool) {
	if len(p.Types) != 1 || p.Types[0] == codes.TypeAny || p.Text[0] != codes.Spelling(p.Types[0]) {
		return codes.None, false
	}
	return p.Types[0], true
}
