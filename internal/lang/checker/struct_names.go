package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// methodNames denetlenmekte olan metodun yapısıdır (7.3): metotta düz ad hiçbir zaman alan ya da
// metot değildir; alan ve metot TR_self ile okunur, atanır ve çağrılır.
type methodNames struct {
	def *ast.StructStatement
	// outer metodun tanımlandığı kapsamdır: ad bundan önceki (metodun içindeki) bir kapsamda
	// tanımlıysa yereldir, alanla karışmaz.
	outer *nameScope
}

// structMethods yapının metotlarını, düz alan adını yakalayan bağlamla denetler.
func (d *nameChecker) structMethods(s *ast.StructStatement, k *nameScope) {
	prev := d.method
	d.method = &methodNames{def: s, outer: k}
	for _, name := range sortedMethodNames(s) {
		d.function(s.Methods[name], k, d.treeLang.Name(codes.KwSelf))
	}
	d.method = prev
}

// memberName id metotta yapının bir alanının ya da metodunun adıyla yazılmış ve metodun içinde
// tanımlanmamış düz bir adsa hatasını kaydeder ve true döner (7.3). Ad dıştaki bir ada çözülüyorsa
// H1024, hiçbir ada çözülmüyorsa tanımsız ad (H1025); ikisi de TR_self ile yazımı önerir.
func (d *nameChecker) memberName(id *ast.Identifier, k *nameScope) bool {
	m := d.method
	if m == nil || d.bindings != nil {
		return false
	}
	kind := ""
	if _, ok := m.def.Methods[id.Value]; ok {
		kind = "method"
	}
	for _, f := range m.def.Fields {
		if f.Name == id.Value {
			kind = "field"
		}
	}
	if kind == "" {
		return false
	}
	for c := k; c != nil && c != m.outer; c = c.parent {
		if c.names[id.Value] {
			return false // metodun yereli ya da parametresi
		}
	}
	// düzeltme dosyanın yazımıdır: kodlu dosyada TR_self, bir yazım dilindeki metinde o dilin sözcüğü
	self := codes.Spelling(codes.KwSelf)
	if lang := d.c.language(); lang != nil {
		self = lang.Name(codes.KwSelf)
	}
	var err error = messages.New(messages.H1024, kind, id.Value, m.def.Name, d.c.word(codes.KwSelf))
	if !k.isKnown(id.Value) {
		err = messages.New(messages.H1010, messages.New(messages.H1025, kind, id.Value, m.def.Name, d.c.word(codes.KwSelf)))
	}
	name := id.Value
	if id.Token.Escaped {
		name = "$" + name
	}
	fix := messages.Fix{Range: id.Token.Range(), Replacement: self + "." + name, Applicable: true}
	d.c.nameErrors = append(d.c.nameErrors, messages.AtRange(id.Token.Range(), err).WithFix(fix))
	return true
}
