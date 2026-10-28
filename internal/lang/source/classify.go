package source

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// langElement bir ad sözcüğünün (kodla yazılmamış, kaçışsız TANIMLAYICI) bulunduğu yerde
// c.dil'in hangi öğesi olduğudur; kullanıcının adıysa kod.Yok (17.3):
//
//   - tip açıklaması (değişken x: Tam): tip adı;
//   - üye adı (x.ad) ile yapı alanı ve metodu tanımı: üye kodu (Y5, tipten bağımsız; 'kurucu'
//     bağlamsal TR_ctx_init);
//   - sözlük değişmezinin çıplak ad anahtarı ({ad: değer}): adı bir üye koduysa o kod (anahtar
//     x.ad ile okunur; iki yer birlikte çevrilsin);
//   - test deyiminin ve aktar ... olarak'ın sözcükleri (TR_ctx_test, TR_ctx_as);
//   - değer olarak okunan ad: kullanıcının görünen bir tanımı yoksa yerleşik ya da tip adı,
//     kendi bloğunda örtük ad (o, birikim, hata);
//   - adlı argüman: çağrılan dilin öğesiyse parametre kodu (denetci.AdBaglari);
//   - kitaplığın bildirimi: kitaplığın yapı, fonksiyon ve sabit adı ile kitaplık fonksiyonunun
//     parametresi bildirildiği yerde de kodlanır (libraryDeclarations; kitaplığın dili yoktur).
//
// Tip adının tek yazımı vardır (3.14): kabul edilmeyen bir yazım (tam) tip adı değildir,
// yazıldığı gibi kalır; denetleyici onu hata olarak bildirir.
func (c *analysis) langElement(r lexer.Record) codes.Code {
	pos := [2]int{r.Line, r.Column}
	name := r.Literal
	if k, ok := c.declarations[pos]; ok {
		return k
	}
	switch c.roles[pos] {
	case parser.RoleType:
		return c.find(codes.NamespaceType, name)
	case parser.RoleMember, parser.RoleStructField, parser.RoleStructMethod:
		if name == c.lang.Name(codes.CtxInit) {
			return codes.CtxInit
		}
		return c.find(codes.NamespaceMember, name)
	case parser.RoleTest:
		return c.contextual(codes.CtxTest, name)
	case parser.RoleAs:
		return c.contextual(codes.CtxAs, name)
	}
	if bg, ok := c.bindings[pos]; ok && bg.Token.Literal == name {
		switch bg.Kind {
		case checker.BindBuiltin, checker.BindImplicit, checker.BindNamed:
			return bg.Code
		case checker.BindUser:
			// Kaçışla ($ad) tanımlanmış bir kullanıcı adı, kaçışsız geçtiği yerde dilin
			// sözcüğüdür: gösterim dilin sözcüğüyle çakışan kullanıcı adını her yerde kaçışla
			// yazar (2.11); kaçışsız biçim kullanıcının tanımına bağlanamaz.
			if c.escapedNames[name] {
				if k, ok := c.lang.Find(codes.NamespaceGlobal, name); ok && k.IsBuiltin() {
					return k
				}
			}
		}
	}
	return codes.None
}

// find adın c.dil'de verilen ad alanındaki canlı kodudur.
func (c *analysis) find(ns codes.Namespace, name string) codes.Code {
	if k, ok := c.lang.Find(ns, name); ok {
		return k
	}
	return codes.None
}

// contextual ad c.dil'de k bağlamsal sözcüğünün adıysa k'dir.
func (c *analysis) contextual(k codes.Code, name string) codes.Code {
	if c.lang.Name(k) == name {
		return k
	}
	return codes.None
}

// libraryDeclarations kitaplık bildirimlerinin ad sözcüklerinin kodlarıdır: kitaplığın yapı,
// fonksiyon, fonksiyon değişkeni ve sabit adları ile kitaplık fonksiyonlarının (yapının metotları
// dahil) parametreleri (17.3). Bildirim adının ağaçta belirteci olmadığı için sözcük kayıtlardan
// bulunur: anahtar kelimeden sonraki ilk ad, parametre ayracının ilk düzeyindeki adlar. Takma adla
// aktarılan modülün adı da kodlanır (aktar "tr/katman" olarak k; k.GPT). Bu adlara giden
// kullanımları checker.NameBindings bağlar.
func (c *analysis) libraryDeclarations() map[[2]int]codes.Code {
	at := map[[2]int]int{}
	for i, r := range c.records {
		if _, ok := at[[2]int{r.Line, r.Column}]; !ok {
			at[[2]int{r.Line, r.Column}] = i
		}
	}
	out := map[[2]int]codes.Code{}
	mark := func(i int, k codes.Code) {
		if i >= 0 && k != codes.None {
			out[[2]int{c.records[i].Line, c.records[i].Column}] = k
		}
	}
	nextName := func(tok lexer.Token) int {
		if i, ok := at[[2]int{tok.Line, tok.Column}]; ok {
			for j := i + 1; j < len(c.records); j++ {
				if c.records[j].Type == lexer.TokIdent {
					return j
				}
			}
		}
		return -1
	}
	params := func(f *ast.FunctionLiteral) {
		if i, ok := at[[2]int{f.Token.Line, f.Token.Column}]; ok {
			for _, j := range c.parameterRecords(i) {
				if k, ok := c.lang.Find(codes.NamespaceParam, c.records[j].Literal); ok {
					mark(j, k)
				}
			}
		}
	}
	aliases := map[string]bool{}
	ast.Walk(c.prog, func(n any) bool {
		if x, ok := n.(*ast.ImportStatement); ok && x.Alias != "" {
			aliases[x.Alias] = true
		}
		return true
	})
	ast.Walk(c.prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.MemberExpression:
			if obj, ok := x.Object.(*ast.Identifier); ok && aliases[obj.Value] {
				if i, ok := at[[2]int{x.NameToken.Line, x.NameToken.Column}]; ok {
					mark(i, checker.LibraryCode(c.lang, x.Property))
				}
			}
		case *ast.StructStatement:
			if k := checker.LibraryCode(c.lang, x.Name); k != codes.None {
				mark(nextName(x.Token), k)
				for _, m := range x.Methods {
					params(m)
				}
			}
		case *ast.FunctionLiteral:
			if k := checker.LibraryCode(c.lang, x.Name); x.Name != "" && k != codes.None {
				mark(nextName(x.Token), k)
				params(x)
			}
		case *ast.VarStatement:
			if k := checker.LibraryCode(c.lang, x.Name.Value); k != codes.None {
				if i, ok := at[[2]int{x.Name.Token.Line, x.Name.Token.Column}]; ok {
					mark(i, k)
				}
				if f, ok := x.Value.(*ast.FunctionLiteral); ok {
					params(f)
				}
			}
		}
		return true
	})
	return out
}

// userParameterCodes kodlu programda kullanıcı fonksiyonlarının (kitaplık fonksiyonu, kitaplık
// yapısının metodu ve kitaplığın fonksiyon değişkeni dışındakiler) TR_ ile yazılmış
// parametrelerinin kodlarıdır (TR_func ölçekle(x, TR_lr)). Kitaplık parametresi gösterimde dilin
// sözcüğüyle yazılır ve geri kodlanır (libraryDeclarations); kullanıcı fonksiyonunda bu yapılamaz:
// düz yazılmış parametrelerin çoğu (x, n, adım) bir dilin parametre sözcüğüdür, kodlanırsa kaçış
// isterdi. O yüzden gösterim bu parametreleri, kullanımları ve adlı argümanlarıyla birlikte, kimlik
// yazımıyla bırakır. Yalnız adlı argüman kodları sayılır.
func userParameterCodes(prog *ast.Program) map[codes.Code]bool {
	isCode := func(name string) bool { _, err := codes.Parse(name); return err == nil }
	library := map[*ast.FunctionLiteral]bool{}
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.StructStatement:
			if isCode(x.Name) {
				for _, m := range x.Methods {
					library[m] = true
				}
			}
		case *ast.VarStatement:
			if f, ok := x.Value.(*ast.FunctionLiteral); ok && isCode(x.Name.Value) {
				library[f] = true
			}
		}
		return true
	})
	out := map[codes.Code]bool{}
	ast.Walk(prog, func(n any) bool {
		if f, ok := n.(*ast.FunctionLiteral); ok && !library[f] && !isCode(f.Name) {
			for _, p := range f.Parameters {
				if k, err := codes.Parse(p.Name); err == nil && k.Namespace() == codes.NamespaceParam {
					out[k] = true
				}
			}
		}
		return true
	})
	return out
}

// parameterRecords i'deki fonksiyon belirtecinden sonraki ilk parametre ayracının ilk düzeyindeki
// parametre adlarının kayıt sıralarıdır (ayraçtan ya da virgülden hemen sonraki adlar).
func (c *analysis) parameterRecords(i int) []int {
	for i < len(c.records) && c.records[i].Type != lexer.TokLParen {
		i++
	}
	var out []int
	depth := 0
	for j := i; j < len(c.records); j++ {
		r := c.records[j]
		switch {
		case r.Type.IsOpener():
			depth++
		case r.Type.IsCloser():
			depth--
		}
		if depth == 0 {
			return out
		}
		if prev := c.records[j-1].Type; r.Type == lexer.TokIdent && depth == 1 && (prev == lexer.TokLParen || prev == lexer.TokComma) {
			out = append(out, j)
		}
	}
	return out
}
