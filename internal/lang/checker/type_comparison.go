package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tip değeri metinle karşılaştırılmaz (docs/spec/03_tipler.md 3.11): tip(x) bir tip değeri
// verir, tip adları tırnaksız yazılır (tip(x) == Tam). tip(x) == "Tam" çalışma anında da
// hatadır; denetleyici, yerleşik tip() çağrısının bir metin sabitiyle karşılaştırıldığını
// çalıştırmadan önce bulur.

// checkTypeTextComparison e, yerleşik tip()'in sonucunu bir metin sabitiyle (== ya da !=)
// karşılaştırıyorsa hata ekler.
func (c *Checker) checkTypeTextComparison(e *ast.InfixExpression) {
	text, call, ok := textComparison(e)
	if !ok || c.builtinCall(call, codes.FnType) == nil {
		return
	}
	c.addError(e.Line(), 0, hint.TypeTextComparisonError(string(e.Operator), text.Value, c.word(codes.TypeInt), c.language()))
}

// Mantık değerinin metni dilden bağımsızdır (codes.ValueNames): TR_str(b) her dosyada ve her
// TR_DIL'de "true" ya da "false" verir. Aynı aileden ikinci kural: TR_str(Mantık) bir yazım dilinin
// doğru/yanlış adıyla karşılaştırılıyorsa (TR_str(b) == "doğru") karşılaştırma hiç tutmaz;
// denetleyici uyarır (çalışmayı engellemez).

// checkBoolTextComparison e, Mantık bir değerin TR_str()'ini değer metninde olmayan bir doğru/
// yanlış adıyla karşılaştırıyorsa uyarı ekler.
func (c *Checker) checkBoolTextComparison(e *ast.InfixExpression) {
	text, call, ok := textComparison(e)
	if !ok {
		return
	}
	str := c.builtinCall(call, codes.FnStr)
	if str == nil || len(str.Arguments) != 1 || !c.isBool(str.Arguments[0]) {
		return
	}
	own := codes.ValueNames()
	if text.Value == own.Name(codes.KwTrue) || text.Value == own.Name(codes.KwFalse) {
		return
	}
	if boolName(text.Value) {
		c.warnings = append(c.warnings, messages.At(e.Line(), 0, messages.New(messages.H1016,
			c.word(codes.FnStr), own.Name(codes.KwTrue), own.Name(codes.KwFalse), text.Value)))
	}
}

// boolName metin bir doğru/yanlış adı mı: bir yazım dilinin adı (doğru, yanlış) ya da başka bir
// dilden tanıdık ad (Python'un True ve False'u, definitions.json "familiar").
func boolName(text string) bool {
	if k, ok := codes.FamiliarCode(text); ok && (k == codes.KwTrue || k == codes.KwFalse) {
		return true
	}
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil && (text == l.Name(codes.KwTrue) || text == l.Name(codes.KwFalse)) {
			return true
		}
	}
	return false
}

// isBool ifadenin tipi Mantık mı (yan etkisiz: tip çıkarımı ifadeyi yeniden denetlemez, hataları
// ikinci kez eklemez)? Mantık sabiti, tipi Mantık olan ad, karşılaştırma ve değil (!).
func (c *Checker) isBool(e ast.Expression) bool {
	switch x := e.(type) {
	case *ast.BooleanLiteral:
		return true
	case *ast.Identifier:
		info, ok := c.scope.Resolve(x.Value)
		return ok && info.TypeName == typeBool
	case *ast.PrefixExpression:
		return x.Operator == "!"
	case *ast.InfixExpression:
		switch x.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return true
		}
	}
	return false
}

// textComparison e bir metin sabitiyle == ya da != karşılaştırması mı; öyleyse sabit ve öbür taraf.
func textComparison(e *ast.InfixExpression) (*ast.StringLiteral, ast.Expression, bool) {
	if e.Operator != "==" && e.Operator != "!=" {
		return nil, nil, false
	}
	if text, ok := e.Right.(*ast.StringLiteral); ok {
		return text, e.Left, true
	}
	text, ok := e.Left.(*ast.StringLiteral)
	return text, e.Right, ok
}

// builtinCall ifade, kullanıcının gölgelemediği yerleşik k'nın bir çağrısıysa o çağrıdır.
func (c *Checker) builtinCall(expression ast.Expression, k codes.Code) *ast.CallExpression {
	call, ok := expression.(*ast.CallExpression)
	if !ok {
		return nil
	}
	name, ok := call.Function.(*ast.Identifier)
	if !ok || name.Value != codes.Name(k) {
		return nil
	}
	if _, shadowed := c.scope.Resolve(name.Value); shadowed {
		return nil
	}
	return call
}
