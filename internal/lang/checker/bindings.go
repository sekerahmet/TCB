package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Ad bağları: kodlu saklama (docs/spec/17_kodlar.md, 17.3) bir adın dilin öğesi mi
// kullanıcının adı mı olduğunu sözcüğe bakarak değil, tanımsız ad denetiminin kapsam
// kuralıyla (5.12, 5.14) bulur. Aynı gezinti burada hata aramadan her adın neye bağlandığını
// verir: kullanıcı bir yerleşiği gölgelediyse (değişken yol = ...) o kapsamdaki 'yol'
// kullanıcının adıdır; sondaki bloğun 'o'su ve adı yazılmamış yakala'nın 'hata'sı yalnızca
// kendi bloklarında bağlamsal sözcüktür.

// BindingKind is the kind of thing a name is bound to.
type BindingKind uint8

const (
	// BindUser: the name is the user's definition in a visible scope.
	BindUser BindingKind = iota
	// BindBuiltin: the name is in no user scope; it is a builtin or type name of the
	// language (Code). Under an 'aktar' with no alias it also counts as builtin (the module's
	// names arrive at run time; whether a module shadows a builtin is known only then).
	BindBuiltin
	// BindImplicit: the name is a contextual implicit name (Code: TR_ctx_error hata, TR_ctx_it o, TR_ctx_acc
	// birikim).
	BindImplicit
	// BindNamed: a named argument's name is a name of the language (Code: the arg parameter
	// code, or the member code of a field name in a struct constructor).
	BindNamed
	// BindUndefined: the name belongs neither to the user nor to the language.
	BindUndefined
)

// NameBinding is the binding of a name token (a name read as a value, or a named-argument
// name).
type NameBinding struct {
	Token lexer.Token
	Kind  BindingKind
	Code  codes.Code
}

// NameBindings returns the binding of every name in the program using dil's names. Implicit
// names and builtins are looked up by dil's names: if dil is not the active language, the
// result is what the token would bind to had it been written in that language (this is how
// a coded file shown in another writing language finds out whether a '$name' escape is
// needed). cakisanlar holds the user names that, where they appear, would capture dil's name
// of a value name written as a code in coded source (TR_fn_print) with their own definition (a user
// variable named 'yazdır' in dil would shadow the builtin 'yazdır'); these names are written
// with an escape everywhere in the view. genelAdlar and dinamik are as in SetGlobalScope
// (modules). treeLang is the language the tree's names are written in (the parser's implicit
// names, 'kendi', the constructor's name): the active language for a coded body, Turkish for
// a file written in Turkish (lexer.Lexer.Language).
func NameBindings(prog *ast.Program, treeLang, lang *codes.Language, globalNames []string, dynamic bool) (bindings []NameBinding, conflicts map[string]bool) {
	d := &nameChecker{bindings: &bindings, treeLang: treeLang, lang: lang, conflicts: map[string]bool{},
		libraryFuncs: map[*ast.FunctionLiteral]bool{}}
	d.statements(prog.Statements, newGlobalNameScope(globalNames, dynamic))
	return bindings, d.conflicts
}

// resolve adın görünen kapsamlardaki bağıdır; yapı, ad bir kullanıcı yapısıysa onun tanımıdır.
func (d *nameChecker) resolve(name string, k *nameScope) (BindingKind, codes.Code, *ast.StructStatement) {
	ambiguous := false
	for c := k; c != nil; c = c.parent {
		for _, o := range c.implicit {
			if o != codes.None && d.lang.Name(o) == name {
				return BindImplicit, o, nil
			}
		}
		if c.names[name] {
			if code, ok := c.coded[name]; ok {
				return BindBuiltin, code, c.structs[name]
			}
			return BindUser, codes.None, c.structs[name]
		}
		ambiguous = ambiguous || c.dynamic
	}
	if k, ok := d.lang.Find(codes.NamespaceGlobal, name); ok && (k.IsBuiltin() || k.IsLibrary()) {
		return BindBuiltin, k, nil
	}
	if k, ok := d.lang.Find(codes.NamespaceType, name); ok {
		return BindBuiltin, k, nil
	}
	if ambiguous {
		return BindUser, codes.None, nil
	}
	return BindUndefined, codes.None, nil
}

// tokenResolution bir ad sözcüğünün bağıdır: kodlu kaynakta kodla yazılmış sözcük (Token.Kod)
// kodun kendisine bağlıdır, başka her ad cozum ile kapsamlarda aranır.
func (d *nameChecker) tokenResolution(tok lexer.Token, name string, k *nameScope) (BindingKind, codes.Code, *ast.StructStatement) {
	switch code := tok.Code; {
	case code == codes.None:
		return d.resolve(name, k)
	case code.Namespace() == codes.NamespaceGlobal && !code.IsBuiltin() && !code.IsLibrary():
		return BindImplicit, code, nil
	default:
		return BindBuiltin, code, nil
	}
}

// bind değer olarak okunan (ya da atanan) bir adın bağını kaydeder.
func (d *nameChecker) bind(id *ast.Identifier, k *nameScope) {
	kind, code, _ := d.tokenResolution(id.Token, id.Value, k)
	*d.bindings = append(*d.bindings, NameBinding{Token: id.Token, Kind: kind, Code: code})
	if code := id.Token.Code; code.IsBuiltin() || code.IsLibrary() || code.Namespace() == codes.NamespaceType {
		if g, ok := d.lang.Entry(code); ok {
			if t, _, _ := d.resolve(g.Name, k); t == BindUser {
				d.conflicts[g.Name] = true
			}
		}
	}
}

// namedArgBindings bir çağrının adlı argümanlarının bağlarını kaydeder. Adlı argüman adı yalnızca
// çağrılan dilin bir öğesiyse dilin adıdır: yerleşik bir fonksiyon (gölgelenmemiş) ya da
// dilin bir üyesi (x.ad, 17.3); kullanıcının fonksiyonlarına verilen adlı argümanlar
// parametre adlarıdır, kullanıcının adıdır. Kurucusu olmayan bir yapının oluşturulmasında
// adlı argümanlar alan adlarıdır; alan adı bir üye koduyla saklanıyorsa (Y5) argüman da öyle.
func (d *nameChecker) namedArgBindings(c *ast.CallExpression, k *nameScope) {
	if len(c.NamedArgs) == 0 {
		return
	}
	field := codes.NamespaceParam
	switch f := c.Function.(type) {
	case *ast.Identifier:
		kind, _, structDef := d.tokenResolution(f.Token, f.Value, k)
		switch {
		case structDef != nil && structDef.Methods[d.treeLang.Name(codes.CtxInit)] == nil:
			field = codes.NamespaceMember
		case kind == BindBuiltin:
		default:
			return
		}
	case *ast.MemberExpression:
		// Üye metodu ya da takma adlı modülün kitaplık adı (k.GPT(...)).
		if _, ok := d.lang.Find(codes.NamespaceMember, f.Property); !ok && f.NameToken.Code == codes.None && LibraryCode(d.lang, f.Property) == codes.None {
			return
		}
	default:
		return
	}
	for _, na := range c.NamedArgs {
		if code, ok := d.lang.Find(field, na.Name); ok {
			*d.bindings = append(*d.bindings, NameBinding{Token: na.Token, Kind: BindNamed, Code: code})
		}
	}
}

// LibraryCode returns the code of name when it is a top-level name of the standard library (a
// struct, function or constant; definitions.json "modül") in lang, otherwise codes.None. The
// library has no language: its names are words of the language, coded wherever they are written,
// declaration included (17.3).
func LibraryCode(lang *codes.Language, name string) codes.Code {
	for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType} {
		if k, ok := lang.Find(ns, name); ok && k.IsLibrary() {
			return k
		}
	}
	return codes.None
}

// codeDeclarations kapsama tanımlanan kitaplık adlarını kodlarıyla işaretler: kitaplık dosyasının
// kendi bildirimleri ve onlara giden her kullanım kodlanır.
func (d *nameChecker) codeDeclarations(stmts []ast.Statement, k *nameScope) {
	for _, st := range stmts {
		ast.StatementDefinedNames(st, func(name string) {
			if code := LibraryCode(d.lang, name); code != codes.None {
				k.code(name, code)
			}
		})
		switch s := st.(type) {
		case *ast.StructStatement:
			if LibraryCode(d.lang, s.Name) != codes.None {
				for _, m := range s.Methods {
					d.libraryFuncs[m] = true
				}
			}
		case *ast.VarStatement:
			if fl, ok := s.Value.(*ast.FunctionLiteral); ok && LibraryCode(d.lang, s.Name.Value) != codes.None {
				d.libraryFuncs[fl] = true
			}
		}
	}
}

// codeParameters kitaplık fonksiyonunun (kitaplığın fonksiyonu, fonksiyon değişkeni ya da
// yapısının metodu) parametrelerini adlı argüman kodlarıyla işaretler (17.3).
func (d *nameChecker) codeParameters(f *ast.FunctionLiteral, k *nameScope) {
	if !d.libraryFuncs[f] && (f.Name == "" || LibraryCode(d.lang, f.Name) == codes.None) {
		return
	}
	for _, p := range f.Parameters {
		if code, ok := d.lang.Find(codes.NamespaceParam, p.Name); ok {
			k.code(p.Name, code)
		}
	}
}

// trailingBlockImplicits sondaki bloğun (liste.filtrele { o > 2 }) örtük parametrelerinin
// bağlamsal kodlarıdır: 'o' (TR_ctx_it) ve indirge bloğunda 'birikim' (TR_ctx_acc). Adlar ağacın
// yazıldığı dildedir (treeLang).
func trailingBlockImplicits(f *ast.FunctionLiteral, lang *codes.Language) [2]codes.Code {
	var o [2]codes.Code
	for i, p := range f.Parameters {
		if i >= len(o) {
			break
		}
		switch p.Name {
		case lang.Name(codes.CtxIt):
			o[i] = codes.CtxIt
		case lang.Name(codes.CtxAcc):
			o[i] = codes.CtxAcc
		}
	}
	return o
}
