package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkUnusedValue değeri kullanılmayan, çağrı içermeyen ifade deyimini hata sayar (2.8): satır
// başındaki ikili işleç önceki satırı sürdürmediği için 'a = 1' ardından gelen '- b' ya da '+ c'
// bölünmüş bir formülün kopan parçasıdır; uyarı geçilebileceği için hatadır (Go da yalnız çağrıyı
// deyim olarak kabul eder). Değeri kullanılan bir bloğun son ifadesi (blockValues) bloğun
// değeridir; deyim olarak yazılan eğer ve eşle, fonksiyon tanımı ve kabuk satırı (interactive)
// sayılmaz.
func (c *Checker) checkUnusedValue(s *ast.ExpressionStatement) {
	if c.interactive || c.blockValues[s] || s.Expression == nil {
		return
	}
	switch s.Expression.(type) {
	case *ast.IfExpression, *ast.MatchExpression, *ast.FunctionLiteral:
		return
	}
	if containsCall(s.Expression) {
		return
	}
	// İkili biçimi de olan işleçle başlayan satır kopan bir formül parçasıdır; ileti işleci önceki
	// satırın sonuna taşımayı söyler. Öbürlerinde değeri bir değişkene atamak ya da satırı silmek.
	if pe, ok := s.Expression.(*ast.PrefixExpression); ok && (pe.Operator == lexer.TokMinus || pe.Operator == lexer.TokPlus) {
		c.addErrorAt(s, messages.New(messages.H1017, string(pe.Operator)))
		return
	}
	c.addErrorAt(s, messages.New(messages.H1023, c.word(codes.KwVar)))
}

// containsCall ifadenin (metin yerleştirmeleri dahil) bir çağrı içerdiğidir; '|>' sağındaki
// fonksiyonun çağrısıdır (x |> f, f(x)).
func containsCall(e ast.Expression) bool {
	found := false
	ast.Walk(e, func(n any) bool {
		switch n := n.(type) {
		case *ast.CallExpression:
			found = true
		case *ast.InfixExpression:
			found = n.Operator == lexer.TokPipe
		}
		return !found
	})
	return found
}

// markBlockValue değeri kullanılan bloğun son deyimini, ifade deyimiyse, bloğun değeri olarak işaretler.
func (c *Checker) markBlockValue(b *ast.BlockStatement, used bool) {
	if !used || b == nil || len(b.Statements) == 0 {
		return
	}
	if s, ok := b.Statements[len(b.Statements)-1].(*ast.ExpressionStatement); ok {
		if c.blockValues == nil {
			c.blockValues = map[*ast.ExpressionStatement]bool{}
		}
		c.blockValues[s] = true
	}
}

// inferIf eğer ifadesinin koşulunu ve dallarını denetler; used, değerinin kullanıldığıdır (deyim olarak
// yazılmadı): o zaman dalların son ifadesi değerdir.
// Değer veren eğer ve eşlenin tipi (5.5, 5.6) bütün dallarının ortak tipidir: dalın değeri son
// ifadesidir; TR_return ya da TR_throw ile biten dal değer vermez, sayılmaz. Bir dalın tipi
// bilinmiyorsa ya da dallar farklıysa TR_Any.
func (c *Checker) inferIf(e *ast.IfExpression, used bool) string {
	var b branchTypes
	c.inferExpression(e.Condition)
	c.markBlockValue(e.Consequence, used)
	c.checkStatement(e.Consequence)
	b.block(c, e.Consequence)
	for _, elif := range e.ElseIfs {
		c.inferExpression(elif.Condition)
		c.markBlockValue(elif.Consequence, used)
		c.checkStatement(elif.Consequence)
		b.block(c, elif.Consequence)
	}
	if e.Alternative == nil {
		return typeAny
	}
	c.markBlockValue(e.Alternative, used)
	c.checkStatement(e.Alternative)
	b.block(c, e.Alternative)
	if !used {
		return typeAny
	}
	return b.common(c)
}

// branchTypes dalların değerleridir; unknown, değeri bilinmeyen bir dal vardır.
type branchTypes struct {
	types   []string
	exprs   []ast.Expression
	unknown bool
}

func (b *branchTypes) add(e ast.Expression, t string) {
	b.types, b.exprs = append(b.types, t), append(b.exprs, e)
}

// block bloğun değerini ekler: son ifade deyiminin tipi; TR_return ve TR_throw ile biten blok değer vermez.
func (b *branchTypes) block(c *Checker, blk *ast.BlockStatement) {
	if blk == nil || len(blk.Statements) == 0 {
		b.unknown = true
		return
	}
	switch s := blk.Statements[len(blk.Statements)-1].(type) {
	case *ast.ExpressionStatement:
		if t, ok := c.blockTypes[s]; ok {
			b.add(s.Expression, t)
			return
		}
	case *ast.ReturnStatement, *ast.ThrowStatement:
		return
	}
	b.unknown = true
}

func (b *branchTypes) common(c *Checker) string {
	if b.unknown || len(b.types) == 0 {
		return typeAny
	}
	if t := c.commonType(b.types, b.exprs); t != "" {
		return t
	}
	return typeAny
}

// noteBlockValue değeri kullanılan bloğun son ifadesinin tipini kaydeder.
func (c *Checker) noteBlockValue(s *ast.ExpressionStatement, t string) {
	if !c.blockValues[s] {
		return
	}
	if c.blockTypes == nil {
		c.blockTypes = map[*ast.ExpressionStatement]string{}
	}
	c.blockTypes[s] = t
}

// inferMatch eşle ifadesinin konusunu, desenlerini ve kollarını denetler; used inferIf'teki gibidir.
func (c *Checker) inferMatch(e *ast.MatchExpression, used bool) string {
	var b branchTypes
	c.inferExpression(e.Subject)
	for _, mc := range e.Cases {
		for _, p := range mc.Patterns {
			c.inferExpression(p)
		}
		if mc.Body != nil {
			c.markBlockValue(mc.Body, used)
			c.checkStatement(mc.Body)
			b.block(c, mc.Body)
		}
		if mc.ValueExpr != nil {
			b.add(mc.ValueExpr, c.inferExpression(mc.ValueExpr))
		}
	}
	if !used {
		return typeAny
	}
	return b.common(c)
}
