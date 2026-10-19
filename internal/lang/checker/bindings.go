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
	// BindImplicit: the name is a contextual implicit name (Code: A103 hata, A104 o, A105
	// birikim).
	BindImplicit
	// BindNamed: a named argument's name is a name of the language (Code: the x9xx parameter
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
// of a value name written as a code in coded source (G100) with their own definition (a user
// variable named 'yazdır' in dil would shadow the builtin 'yazdır'); these names are written
// with an escape everywhere in the view. genelAdlar and dinamik are as in SetGlobalScope
// (modules).
func NameBindings(prog *ast.Program, dil *codes.Language, genelAdlar []string, dinamik bool) (baglar []NameBinding, cakisanlar map[string]bool) {
	d := &adDenetleyici{baglar: &baglar, dil: dil, cakisanlar: map[string]bool{}}
	ust := yeniAdKapsami(nil)
	for _, a := range genelAdlar {
		ust.tanimla(a)
	}
	ust.dinamik = dinamik
	k := yeniAdKapsami(ust)
	d.tanimlar(prog.Statements, k)
	for _, st := range prog.Statements {
		d.deyim(st, k)
	}
	return baglar, d.cakisanlar
}

// cozum adın görünen kapsamlardaki bağıdır; yapı, ad bir kullanıcı yapısıysa onun tanımıdır.
func (d *adDenetleyici) cozum(ad string, k *adKapsami) (BindingKind, codes.Code, *ast.StructStatement) {
	belirsiz := false
	for c := k; c != nil; c = c.ust {
		for _, o := range c.ortuk {
			if o != codes.None && d.dil.Name(o) == ad {
				return BindImplicit, o, nil
			}
		}
		if c.adlar[ad] {
			return BindUser, codes.None, c.yapilar[ad]
		}
		belirsiz = belirsiz || c.dinamik
	}
	if k, ok := d.dil.Find(codes.NamespaceGlobal, ad); ok && k.IsBuiltin() {
		return BindBuiltin, k, nil
	}
	if k, ok := d.dil.Find(codes.NamespaceType, ad); ok {
		return BindBuiltin, k, nil
	}
	if belirsiz {
		return BindUser, codes.None, nil
	}
	return BindUndefined, codes.None, nil
}

// sozcukCozumu bir ad sözcüğünün bağıdır: kodlu kaynakta kodla yazılmış sözcük (Token.Kod)
// kodun kendisine bağlıdır, başka her ad cozum ile kapsamlarda aranır.
func (d *adDenetleyici) sozcukCozumu(tok lexer.Token, ad string, k *adKapsami) (BindingKind, codes.Code, *ast.StructStatement) {
	switch kd := tok.Code; {
	case kd == codes.None:
		return d.cozum(ad, k)
	case kd.Namespace() == codes.NamespaceGlobal && !kd.IsBuiltin():
		return BindImplicit, kd, nil
	default:
		return BindBuiltin, kd, nil
	}
}

// bagla değer olarak okunan (ya da atanan) bir adın bağını kaydeder.
func (d *adDenetleyici) bagla(id *ast.Identifier, k *adKapsami) {
	tur, kd, _ := d.sozcukCozumu(id.Token, id.Value, k)
	*d.baglar = append(*d.baglar, NameBinding{Token: id.Token, Kind: tur, Code: kd})
	if kd := id.Token.Code; kd.IsBuiltin() || kd.Namespace() == codes.NamespaceType {
		if g, ok := d.dil.Entry(kd); ok {
			if t, _, _ := d.cozum(g.Name, k); t == BindUser {
				d.cakisanlar[g.Name] = true
			}
		}
	}
}

// adliBaglar bir çağrının adlı argümanlarının bağlarını kaydeder. Adlı argüman adı yalnızca
// çağrılan dilin bir öğesiyse dilin adıdır: yerleşik bir fonksiyon (gölgelenmemiş) ya da
// dilin bir üyesi (x.ad, 17.3); kullanıcının fonksiyonlarına verilen adlı argümanlar
// parametre adlarıdır, kullanıcının adıdır. Kurucusu olmayan bir yapının oluşturulmasında
// adlı argümanlar alan adlarıdır; alan adı bir üye koduyla saklanıyorsa (Y5) argüman da öyle.
func (d *adDenetleyici) adliBaglar(c *ast.CallExpression, k *adKapsami) {
	if len(c.NamedArgs) == 0 {
		return
	}
	alan := codes.NamespaceParam
	switch f := c.Function.(type) {
	case *ast.Identifier:
		tur, _, yapi := d.sozcukCozumu(f.Token, f.Value, k)
		switch {
		case tur == BindBuiltin:
		case yapi != nil && yapi.Methods[codes.Name(codes.A102)] == nil:
			alan = codes.NamespaceMember
		default:
			return
		}
	case *ast.MemberExpression:
		if _, ok := d.dil.Find(codes.NamespaceMember, f.Property); !ok && f.NameToken.Code == codes.None {
			return
		}
	default:
		return
	}
	for _, na := range c.NamedArgs {
		if kd, ok := d.dil.Find(alan, na.Name); ok {
			*d.baglar = append(*d.baglar, NameBinding{Token: na.Token, Kind: BindNamed, Code: kd})
		}
	}
}

// sondakiBlokOrtukleri sondaki bloğun (liste.filtrele { o > 2 }) örtük parametrelerinin
// bağlamsal kodlarıdır: 'o' (A104) ve indirge bloğunda 'birikim' (A105).
func sondakiBlokOrtukleri(f *ast.FunctionLiteral) [2]codes.Code {
	var o [2]codes.Code
	for i, p := range f.Parameters {
		if i >= len(o) {
			break
		}
		switch p.Name {
		case codes.Name(codes.A104):
			o[i] = codes.A104
		case codes.Name(codes.A105):
			o[i] = codes.A105
		}
	}
	return o
}
