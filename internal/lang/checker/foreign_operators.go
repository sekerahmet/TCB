package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Başka dillerin sözcükle yazılan işleçleri TR'de işleç değildir, ad okunur: 'TR_var b = a and a'
// üç deyimdir (TR_var b = a, and, a), 'x not in l' dört. Sözdizimi hatası çıkmaz, sorun tanımsız ad
// olarak görünür. Burası o adların iletisini tamamlar: iki sözcüklü karşılaştırma bütün olarak
// yazımını alır (hint.ForeignOperator), satırın kalan deyimleri de etkisiz değer hatası vermez.

// noteOperator i'deki yalın ad ve aynı satırda ardından gelen deyimin ilk adı başka dilin
// karşılaştırmasıysa (not in, is not) ikisine de ipucunu bağlar. İkinci sözcük yalın olmayabilir:
// 'x not in [1]'de 'in[1]' indeks, 'x is not -1'de 'not - 1' işlemdir. Satırda yazılan dilin
// sözcükleri geçerlidir, ipucu yalnız dosyanındır.
func (d *nameChecker) noteOperator(stmts []ast.Statement, i int) {
	first := bareName(stmts[i])
	if first == nil || i+1 == len(stmts) || d.c.language() != nil {
		return
	}
	second := leadingName(stmts[i+1])
	if second == nil || second.Line() != first.Line() {
		return
	}
	if o := hint.ForeignOperator(first.Value, second.Value); o != nil {
		d.operators[first], d.operators[second] = o, o
	}
}

// noteLoop i'deki deyim yalın 'for' ise aynı satırdaki 'in' döngü başlığıdır, karşılaştırma değil:
// 'for x in l {' deyimlere ayrılır, 'in' üyelik değil döngünün sözcüğünü önerir.
func (d *nameChecker) noteLoop(stmts []ast.Statement, i int) {
	loop := bareName(stmts[i])
	if loop == nil || loop.Value != hint.ForeignFor || d.c.language() != nil {
		return
	}
	for _, st := range stmts[i+1:] {
		w := leadingName(st)
		if w == nil || w.Line() != loop.Line() {
			return
		}
		if hint.ForeignOperator(w.Value) != nil {
			d.operators[w] = hint.LoopInHint()
			return
		}
	}
}

// quietAfterUndefined i'deki deyimin değeri tanımsız bir adın kendisiyse deyimi ve aynı satırda
// ardından gelen ifade deyimini etkisiz değer denetiminden çıkarır (2.8): 'a and a'da hata 'and'in
// tanımsız olmasıdır, ondan kalan 'and' ve 'a' deyimleri, 'TR_var c = not a'da kalan 'a' ayrı bir hata
// değildir. Satırda tanımsız ad hatası durduğu için hiçbir yanlış program bu yolla geçmez.
func (d *nameChecker) quietAfterUndefined(stmts []ast.Statement, i int) {
	id := valueName(stmts[i])
	if id == nil || !d.undefined[id] {
		return
	}
	if s, ok := stmts[i].(*ast.ExpressionStatement); ok {
		d.c.undefinedLeft[s] = true
	}
	if i+1 < len(stmts) {
		if next, ok := stmts[i+1].(*ast.ExpressionStatement); ok && next.Line() == id.Line() {
			d.c.undefinedLeft[next] = true
		}
	}
}

// bareName deyim tek başına bir addan ibaretse o addır.
func bareName(st ast.Statement) *ast.Identifier {
	if s, ok := st.(*ast.ExpressionStatement); ok {
		id, _ := s.Expression.(*ast.Identifier)
		return id
	}
	return nil
}

// valueName deyimin değeri tek başına bir addan ibaretse o addır.
func valueName(st ast.Statement) *ast.Identifier {
	var value ast.Expression
	switch s := st.(type) {
	case *ast.ExpressionStatement:
		value = s.Expression
	case *ast.VarStatement:
		value = s.Value
	case *ast.AssignStatement:
		value = s.Value
	case *ast.ReturnStatement:
		value = s.ReturnValue
	}
	id, _ := value.(*ast.Identifier)
	return id
}

// leadingName ifade deyiminin ilk adıdır: 'in[1]' ve 'not - 1' içinde 'in' ve 'not'.
func leadingName(st ast.Statement) *ast.Identifier {
	s, ok := st.(*ast.ExpressionStatement)
	if !ok {
		return nil
	}
	var id *ast.Identifier
	ast.Walk(s.Expression, func(n any) bool {
		if i, isName := n.(*ast.Identifier); isName && id == nil {
			id = i
		}
		return id == nil
	})
	return id
}
