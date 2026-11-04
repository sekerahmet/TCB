package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Fonksiyon ifadesinin blok gövdesi (6.2, 6.6): beklenen dönüş tipi bilinmiyorsa ve bir yol değer
// döndürüyorsa her yol değerle ya da 6.2'nin öbür bitişleriyle biter; değersiz TR_return, TR_return
// TR_none ve bitmeyen yol denetim hatasıdır (H1145): değer yerinde sessiz yokluk bırakırdı. Dönüş tipi ancak fonksiyon
// tipi yazılmış yerde bilinir (T? karışımı orada geçerlidir): tipli değişken ve atama, parametre ve
// alan varsayılanı, dönüşü fonksiyon tipi olan TR_return, kullanıcı fonksiyonunun, metodunun ve
// kurucusunun argümanı. Yerleşiğin imzası `TR_Function` der, dönüş tipi söylemez; modülün fonksiyonu
// bilinmez. T? dönen fonksiyon beklenen tipten yazılır.

// expect t bir fonksiyon tipiyse value'daki fonksiyon ifadesinin dönüş tipi bilinir; liste ve demet
// değişmezinin öğeleri öğe tipiyle.
func (c *Checker) expect(t *ast.TypeRef, value ast.Expression) {
	if t == nil {
		return
	}
	switch v := value.(type) {
	case *ast.FunctionLiteral:
		if t.Return != nil {
			c.expectedReturns[v] = true
		}
	case *ast.ListLiteral:
		if len(t.Args) == 1 {
			for _, el := range v.Elements {
				c.expect(&t.Args[0], el)
			}
		}
	case *ast.TupleExpression:
		for i, el := range v.Elements {
			if i < len(t.Elems) {
				c.expect(&t.Elems[i], el)
			}
		}
	}
}

// expectArguments çağrısı bilinen kullanıcı fonksiyonunun, metodunun ve kurucusunun argümanlarını
// parametrenin ya da alanın yazılan tipine bağlar.
func (c *Checker) expectArguments(e *ast.CallExpression, info TypeInfo, objType string) {
	var written func(i int, name string) *ast.TypeRef
	switch f := e.Function.(type) {
	case *ast.Identifier:
		if s := c.structs[f.Value]; s != nil {
			written = func(i int, name string) *ast.TypeRef { return fieldTypeRef(s, i, name) }
		} else if b, ok := c.scope.binding(f.Value); ok && b.Fn != nil {
			written = func(i int, name string) *ast.TypeRef { return paramTypeRef(b.Fn, i, name) }
		}
	case *ast.MemberExpression:
		if s := c.structs[objType]; s != nil && s.Methods[f.Property] != nil {
			written = func(i int, name string) *ast.TypeRef { return paramTypeRef(s.Methods[f.Property], i, name) }
		}
	}
	if written == nil {
		return
	}
	for i, a := range e.Arguments {
		c.expect(written(i, ""), a)
	}
	for _, a := range e.NamedArgs {
		c.expect(written(-1, a.Name), a.Value)
	}
}

// paramTypeRef f'nin i. ya da name adlı parametresinin yazılan tipidir.
func paramTypeRef(f *ast.FunctionLiteral, i int, name string) *ast.TypeRef {
	for j, p := range f.Parameters {
		if j == i || p.Name == name {
			return p.Type
		}
	}
	return nil
}

// fieldTypeRef s'nin i. ya da name adlı alanının yazılan tipidir.
func fieldTypeRef(s *ast.StructStatement, i int, name string) *ast.TypeRef {
	for j, f := range s.Fields {
		if j == i || f.Name == name {
			return f.Type
		}
	}
	return nil
}

// checkArrowReturns fonksiyon ifadesinin blok gövdesini denetler; ifade gövdesi ve sondaki blok
// TR_return'le değer vermez (3.5, 6.7).
func (c *Checker) checkArrowReturns(f *ast.FunctionLiteral) {
	if f.Body == nil || arrowBody(f) != nil || c.expectedReturns[f] {
		return
	}
	bare, value := false, false
	for _, r := range c.facts.returns[f] {
		none := r.expr == nil || r.typ == typeNull
		bare = bare || none
		value = value || !none
	}
	if value && (bare || !blockEnds(f.Body)) {
		c.addErrorAt(f, messages.New(messages.H1145))
	}
}

// blockEnds bloğun her yolu biter mi (6.2).
func blockEnds(b *ast.BlockStatement) bool {
	if b == nil {
		return false
	}
	for _, s := range b.Statements {
		if statementEnds(s) {
			return true
		}
	}
	return false
}

// statementEnds deyim bitiyor mu: TR_return, TR_throw, TR_exit, çıkışı olmayan sonsuz döngü, bütün
// dalları biten TR_if/TR_else, TR_try ve bütün kolları biten TR_match (eşleme her durumu kapsar, 5.6).
func statementEnds(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStatement, *ast.ThrowStatement:
		return true
	case *ast.BlockStatement:
		return blockEnds(s)
	case *ast.TryCatchStatement:
		return blockEnds(s.TryBody) && blockEnds(s.CatchBody)
	case *ast.WhileStatement:
		return endlessLoop(s)
	case *ast.ExpressionStatement:
		return exprEnds(s.Expression)
	}
	return false
}

func exprEnds(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.CallExpression:
		id, ok := e.Function.(*ast.Identifier)
		return ok && id.Token.Code == codes.FnExit
	case *ast.IfExpression:
		if !blockEnds(e.Consequence) || !blockEnds(e.Alternative) {
			return false
		}
		for _, elif := range e.ElseIfs {
			if !blockEnds(elif.Consequence) {
				return false
			}
		}
		return true
	case *ast.MatchExpression:
		for _, mc := range e.Cases {
			if !blockEnds(mc.Body) && !exprEnds(mc.ValueExpr) {
				return false
			}
		}
		return len(e.Cases) > 0
	}
	return false
}

// endlessLoop TR_for { ... } çıkışı olmayan sonsuz döngü mü (5.9): TR_break içteki döngünün ya da iç
// fonksiyonun değilse çıkıştır.
func endlessLoop(s *ast.WhileStatement) bool {
	if lit, ok := s.Condition.(*ast.BooleanLiteral); s.Condition != nil && !(ok && lit.Value) {
		return false
	}
	exits := false
	ast.Walk(s.Body, func(n any) bool {
		switch n.(type) {
		case *ast.WhileStatement, *ast.ForInStatement, *ast.FunctionLiteral:
			return false
		case *ast.BreakStatement:
			exits = true
		}
		return !exits
	})
	return !exits
}
