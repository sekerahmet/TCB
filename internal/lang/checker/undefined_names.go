package checker

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tanımsız ad denetimi (docs/spec/05_deyimler.md §5.14): bir ad, okunduğu ya da atandığı yerde
// görünen hiçbir kapsamda tanımlı değilse ve bir yerleşik değilse program çalışmadan önce
// hata verir; çalışma anındaki tanımsız ad hatasıyla aynı mesaj ve ipucu kullanılır
// ("'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?").
//
// Dil dinamiktir; denetim yanlış alarm vermemek için yalnızca kesin durumları yakalar:
//   - Bir kapsamda tanımlanan ad, o kapsamın her yerinden görünür sayılır (tanım sırası
//     aranmaz): fonksiyon gövdesi, kendisinden sonra tanımlanan bir genel adı çağrıldığı
//     anda görebilir.
//   - Takma adsız 'aktar' adlarını çalışma anında belli olur; böyle bir aktarımın bulunduğu
//     kapsamda ve altında tanımsız ad aranmaz.
//   - Üye adları (x.ad), adlı argümanlar, yapı alanları denetlenmez; sondaki bloğun 'o'su,
//     yakala'nın 'hata'sı, döngü ve üreteç değişkenleri, parametreler ve yapı metotlarındaki
//     'kendi' kendi kapsamlarında tanımlıdır.

// nameScope tanımsız ad denetiminin sözcüksel kapsamıdır.
type nameScope struct {
	parent  *nameScope
	names   map[string]bool
	dynamic bool // takma adsız 'aktar': adlar çalışma anında belli olur
	// structs kapsamda tanımlanan yapılardır (kurucuya verilen adlı argümanlar alan adıdır);
	// ortuk, kapsamın örtük adlarının bağlamsal kodlarıdır (sondaki bloğun 'o'su ve
	// 'birikim'i, adı yazılmamış yakala'nın 'hata'sı). İkisi de ad bağlarında kullanılır
	// (bindings.go).
	structs  map[string]*ast.StructStatement
	implicit [2]codes.Code
}

func newNameScope(parent *nameScope) *nameScope {
	return &nameScope{parent: parent, names: map[string]bool{}}
}

func (k *nameScope) define(name string) {
	if name == "" {
		k.dynamic = true
		return
	}
	k.names[name] = true
}

// isKnown ad bu kapsamdan görünüyor mu (ya da adları belirsiz bir kapsamın altında mı)?
func (k *nameScope) isKnown(name string) bool {
	for c := k; c != nil; c = c.parent {
		if c.names[name] || c.dynamic {
			return true
		}
	}
	return builtinNames()[name]
}

// visible öneri için bu kapsamdan görünen adlardır (yerleşikler dahil).
func (k *nameScope) visible() []string {
	var out []string
	for c := k; c != nil; c = c.parent {
		for a := range c.names {
			out = append(out, a)
		}
	}
	for a := range builtinNames() {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// nameChecker bir programın tanımsız ad denetimidir; bulunan hatalar Checker'a eklenir.
type nameChecker struct {
	c    *Checker
	decl map[string]bool // aynı ad için tek hata (ilk geçtiği yer)
	// bindings nil değilse gezinti hata aramaz, her adın neye bağlandığını dil'in adlarıyla
	// kaydeder (NameBindings, bindings.go).
	bindings *[]NameBinding
	// lang adların arandığı dil, treeLang ağacın adlarının yazıldığı dildir (NameBindings);
	// hata aramada ikisi de etkin dildir.
	lang, treeLang *codes.Language
	// conflicts kodla yazılmış bir sözcüğün dil'deki adını kendi yerinde yakalayan kullanıcı
	// adlarıdır (AdBaglari).
	conflicts map[string]bool
}

// SetGlobalScope is for checking modules: a module runs under the global scope of the
// program that imports it (11.1) and sees the names there. adlar are that global scope's
// names; dinamik says the names are not fully known (e.g. the global scope has an 'aktar'
// with no alias), in which case undefined names are not reported in the module. Call it
// before CheckProgram.
func (c *Checker) SetGlobalScope(names []string, dynamic bool) {
	c.globalNames = newGlobalNameScope(names, dynamic)
}

// newGlobalNameScope dil ve çalışma ortamının hazır adlarını (names) taşıyan bir üst kapsamın
// altında, programın kendi genel kapsamını açar; dynamic adların çalışma anında da eklenebildiğini söyler.
func newGlobalNameScope(names []string, dynamic bool) *nameScope {
	parent := newNameScope(nil)
	for _, a := range names {
		parent.define(a)
	}
	parent.dynamic = dynamic
	return newNameScope(parent)
}

// TopLevelNames returns the names the program defines directly in its global scope; dinamik
// says the names are not fully known because of an 'aktar' with no alias. They are passed to
// SetGlobalScope when the program's imported modules are checked.
func TopLevelNames(prog *ast.Program) (names []string, dynamic bool) {
	k := newNameScope(nil)
	declare(prog.Statements, k)
	for a := range k.names {
		names = append(names, a)
	}
	sort.Strings(names)
	return names, k.dynamic
}

// checkNames programı tanımsız adlar için denetler. Genel kapsam Checker'da kalıcıdır:
// etkileşimli kabukta önceki satırların tanımları sonraki satırlarda görünür.
func (c *Checker) checkNames(prog *ast.Program) {
	if c.globalNames == nil {
		c.globalNames = newNameScope(nil)
	}
	d := &nameChecker{c: c, decl: map[string]bool{}, lang: codes.Active(), treeLang: codes.Active()}
	d.statements(prog.Statements, c.globalNames)
}

// declare deyimlerin, çalıştıkları kapsama doğrudan tanımladığı adları kapsama ekler (yorumlayıcıyla
// aynı kural, ast.StatementDefinedNames: değişken, sabit, yapı, takma adlı aktar ve ifadelerin
// içindeki adlı fonksiyonlar; iç bloklar, eşle kolları, eşzamanlı görevler ve fonksiyon gövdeleri
// kendi kapsamıdır, koşul/konu/desen bulunulan kapsamdadır; 5.12).
func declare(stmts []ast.Statement, k *nameScope) {
	for _, st := range stmts {
		ast.StatementDefinedNames(st, k.define)
		if s, ok := st.(*ast.StructStatement); ok {
			if k.structs == nil {
				k.structs = map[string]*ast.StructStatement{}
			}
			k.structs[s.Name] = s
		}
	}
}

// walkTree ifade ağacını ön sırayla gezer; ziyaret false dönerse düğümün çocuklarına inmez.
func walkTree(e ast.Expression, visit func(ast.Expression) bool) {
	if e == nil || !visit(e) {
		return
	}
	ast.SubExpressions(e, func(child ast.Expression) bool {
		walkTree(child, visit)
		return false
	})
}

// block deyimleri yeni bir alt kapsamda denetler; once alt kapsama önceden tanımlanacak
// adları (döngü değişkeni, parametreler ...) ekler.
func (d *nameChecker) block(b *ast.BlockStatement, parent *nameScope, preset ...string) {
	if b == nil {
		return
	}
	k := newNameScope(parent)
	for _, a := range preset {
		k.define(a)
	}
	d.blockInScope(b, k)
}

// blockInScope bloğun deyimlerini hazırlanmış k kapsamında denetler.
func (d *nameChecker) blockInScope(b *ast.BlockStatement, k *nameScope) {
	if b == nil {
		return
	}
	d.statements(b.Statements, k)
}

// statements deyim listesini k kapsamında önce bildirir (ileri başvurular için), sonra gezer.
func (d *nameChecker) statements(stmts []ast.Statement, k *nameScope) {
	declare(stmts, k)
	for _, st := range stmts {
		d.statement(st, k)
	}
}

func (d *nameChecker) statement(st ast.Statement, k *nameScope) {
	switch s := st.(type) {
	case *ast.VarStatement:
		d.expression(s.Value, k)
	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			d.expression(v, k)
		}
	case *ast.AssignStatement:
		d.target(s.Target, k)
		d.expression(s.Value, k)
	case *ast.MultiAssignStatement:
		for _, t := range s.Targets {
			d.target(t, k)
		}
		for _, v := range s.Values {
			d.expression(v, k)
		}
	case *ast.ExpressionStatement:
		d.expression(s.Expression, k)
	case *ast.ReturnStatement:
		d.expression(s.ReturnValue, k)
	case *ast.ThrowStatement:
		d.expression(s.Value, k)
	case *ast.BlockStatement:
		d.block(s, k)
	case *ast.ForInStatement:
		d.expression(s.Iterable, k)
		var preset []string
		if s.KeyVar != "" {
			preset = append(preset, s.KeyVar)
		}
		d.block(s.Body, k, append(preset, s.ValueVar)...)
	case *ast.WhileStatement:
		d.expression(s.Condition, k)
		d.block(s.Body, k)
	case *ast.TryCatchStatement:
		d.block(s.TryBody, k)
		ck := newNameScope(k)
		ck.define(s.ErrorVar)
		if s.ImplicitErrorVar {
			ck.implicit[0] = codes.A103
		}
		// Örtük ad üst kapsamda kalır; gövdenin kendi 'hata' tanımı onu gölgeler.
		d.blockInScope(s.CatchBody, newNameScope(ck))
	case *ast.StructStatement:
		for _, f := range s.Fields {
			d.expression(f.DefaultValue, k)
		}
		for _, name := range sortedMethodNames(s) {
			d.function(s.Methods[name], k, d.treeLang.Name(codes.A051))
		}
	case *ast.DeferStatement:
		d.block(s.Body, k)
		d.expression(s.Call, k)
	case *ast.AsyncStatement:
		d.block(s.Body, k)
		d.expression(s.Call, k)
	case *ast.BreakpointStatement:
		d.expression(s.Condition, k)
	case *ast.TestStatement:
		d.block(s.Body, k)
	}
}

// function gövdeyi parametrelerin (ve metotta 'kendi'nin) tanımlı olduğu kapsamda denetler.
func (d *nameChecker) function(f *ast.FunctionLiteral, k *nameScope, extra ...string) {
	if f == nil {
		return
	}
	names := append([]string{}, extra...)
	for _, p := range f.Parameters {
		names = append(names, p.Name)
	}
	pk := newNameScope(k)
	for _, a := range names {
		pk.define(a)
	}
	if f.Token.Type == lexer.TokLBrace {
		pk.implicit = trailingBlockImplicits(f, d.treeLang)
	}
	for _, p := range f.Parameters {
		d.expression(p.DefaultValue, pk)
	}
	d.block(f.Body, pk)
}

// target atamanın sol tarafını denetler: ad tanımlı olmalı (çalışma anında tanımsız ada
// atama hatadır, bkz. 5.4); indeks ve alan hedeflerinde nesne okunur.
func (d *nameChecker) target(e ast.Expression, k *nameScope) {
	// Düz ada atama okumadan ayrı bir hatadır: çalışma anı gibi (H2448) 'değişken' ile tanımlamayı
	// söyler. İndeks ya da üye hedefi (l[0] = 1) adı okur.
	if id, ok := e.(*ast.Identifier); ok && d.bindings == nil {
		if !k.isKnown(id.Value) && !d.decl[id.Value] {
			d.decl[id.Value] = true
			d.c.nameErrors = append(d.c.nameErrors, messages.At(id.Line(), id.Token.Column,
				messages.New(messages.H1010, messages.New(messages.H2448, id.Value, codes.NameRef(codes.A030)))))
		}
		return
	}
	d.expression(e, k)
}

func (d *nameChecker) expression(e ast.Expression, k *nameScope) {
	walkTree(e, func(x ast.Expression) bool {
		switch y := x.(type) {
		case *ast.Identifier:
			if d.bindings != nil {
				d.bind(y, k)
			} else {
				d.name(y, k)
			}
		case *ast.CallExpression:
			if d.bindings != nil {
				d.namedArgBindings(y, k)
			}
		case *ast.FunctionLiteral:
			d.function(y, k)
			return false
		case *ast.IfExpression:
			d.expression(y.Condition, k)
			d.block(y.Consequence, k)
			for _, elif := range y.ElseIfs {
				d.expression(elif.Condition, k)
				d.block(elif.Consequence, k)
			}
			d.block(y.Alternative, k)
			return false
		case *ast.MatchExpression:
			d.expression(y.Subject, k)
			for _, mc := range y.Cases {
				for _, p := range mc.Patterns {
					d.expression(p, k)
				}
				d.block(mc.Body, k)
				d.expression(mc.ValueExpr, k)
			}
			return false
		case *ast.ListComprehension:
			d.expression(y.Iterable, k)
			ik := newNameScope(k)
			ik.define(y.VarName)
			d.expression(y.Output, ik)
			d.expression(y.Condition, ik)
			return false
		}
		return true
	})
}

// name, okunan ya da atanan bir adın görünür olduğunu denetler.
func (d *nameChecker) name(id *ast.Identifier, k *nameScope) {
	if k.isKnown(id.Value) || d.decl[id.Value] {
		return
	}
	d.decl[id.Value] = true
	d.c.nameErrors = append(d.c.nameErrors, messages.At(id.Line(), id.Token.Column,
		messages.New(messages.H1010, hint.UndefinedNameMessage(id.Value, k.visible()))))
}

// sortByLine hataları konumlarının satırına göre (eşitlikte ilk sırayı koruyarak) sıralar;
// konumsuz hatalar sona gider.
func sortByLine(errList []error) {
	line := func(h error) int {
		n, _, ok := messages.Pos(h)
		if !ok {
			return 1 << 30
		}
		return n
	}
	sort.SliceStable(errList, func(i, j int) bool { return line(errList[i]) < line(errList[j]) })
}
