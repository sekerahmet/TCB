package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// calledBuiltin çağrının yerleşiğidir: kodla yazılmış yerleşik fonksiyon adı (TR_floor(x); kullanıcı
// adı gölgelemiyorsa) ya da kodla yazılmış üye (l.TR_sort()); receiver üyenin nesnesidir. Kod
// sözcüğün kodundan (lexer.Token.Code) gelir: kullanıcının adı hiçbir kodu taşımaz.
func (c *Checker) calledBuiltin(e *ast.CallExpression) (k codes.Code, receiver ast.Expression) {
	switch f := e.Function.(type) {
	case *ast.Identifier:
		if _, user := c.scope.Resolve(f.Value); !user && f.Token.Code.IsBuiltin() {
			return f.Token.Code, nil
		}
	case *ast.MemberExpression:
		if f.NameToken.Code.Namespace() == codes.NamespaceMember && !c.knownStruct(c.receivers[e]) {
			return f.NameToken.Code, f.Object
		}
	}
	return codes.None, nil
}

// checkPureCall değeri kullanılmayan yalnız-değer çağrısını hata sayar (13.10): tanım dosyasında
// "pure" işaretli yerleşik (giriş/çıkış, dosya, saat, rastgelelik kullanmayan; alıcısını ve
// argümanlarını değiştirmeyen, beklemeyen, yalnız yeni değer veren) tek başına deyim olunca
// sonucu atılır ve satır hiçbir iş yapmaz: Python'un yerinde sıralama alışkanlığı (l.sort())
// listeyi sıralanmamış bırakır. İleti atamayı önerir; alıcı atanabilir bir adsa (l) düzeltme
// uygulanabilir (l = l.TR_sort()), değilse ileti yalnız biçimi gösterir. Bloğun değeri olan son
// ifade ve kabuk satırı sayılmaz.
func (c *Checker) checkPureCall(s *ast.ExpressionStatement) {
	call, ok := s.Expression.(*ast.CallExpression)
	if !ok || c.interactive || c.blockValues[s] {
		return
	}
	k, receiver := c.calledBuiltin(call)
	if def, ok := codes.DefinitionOf(k); !ok || !def.Pure {
		return
	}
	kind := "function"
	if receiver != nil {
		kind = "member"
	}
	text := c.callText(call, k, receiver)
	target, tok := "", lexer.Token{}
	if id, ok := receiver.(*ast.Identifier); ok {
		if info, found := c.scope.Resolve(id.Value); found && !info.IsConst && info.TypeName != typeFunction {
			target, tok = id.Value, id.Token
		}
	}
	suggestion := text
	if target != "" {
		suggestion = target + " = " + text
	}
	e := c.addErrorAt(s, messages.New(messages.H1115, kind, c.word(k), target, suggestion))
	if target != "" {
		again := target
		if tok.Escaped {
			again = "$" + target
		}
		e.WithFix(replaceToken(tok, target+" = "+again, true))
	}
}

// callText çağrının dosyadaki yazımıdır (exprText); satır bilinmiyorsa kodun yazımıyla
// kısaltılmış biçimi.
func (c *Checker) callText(call *ast.CallExpression, k codes.Code, receiver ast.Expression) string {
	if text, ok := c.exprText(call); ok {
		return text
	}
	args := "()"
	if len(call.Arguments)+len(call.NamedArgs) > 0 {
		args = "(...)"
	}
	if id, ok := receiver.(*ast.Identifier); ok {
		return id.Value + "." + codes.Spelling(k) + args
	}
	return codes.Spelling(k) + args
}

// exprText ifadenin dosyadaki yazımıdır; ok false ise satır bilinmiyor ya da ifade tek satıra
// sığmıyor (dosyası verilmemiş denetim).
func (c *Checker) exprText(n ast.Expression) (string, bool) {
	r := ast.RangeOf(n)
	if r.Line < 1 || r.Line > len(c.fileLines) || r.EndLine != r.Line || r.Col < 1 {
		return "", false
	}
	line := []rune(c.fileLines[r.Line-1])
	if r.EndCol-1 > len(line) || r.Col >= r.EndCol {
		return "", false
	}
	return closeParens(line[r.Col-1:r.EndCol-1], line[r.EndCol-1:])
}

// closeParens çağrının ağaçtaki aralığına (kapanan ')' ağaçta tutulmaz) satırın devamından eksik
// kapanan parantezleri ekler; metin içindeki parantez sayılmaz.
func closeParens(text, rest []rune) (string, bool) {
	depth, quote := 0, rune(0)
	for _, r := range text {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '(':
			depth++
		case r == ')':
			depth--
		}
	}
	out := string(text)
	for _, r := range rest {
		if depth == 0 {
			break
		}
		if r != ')' {
			return "", false
		}
		out += ")"
		depth--
	}
	return out, depth == 0
}
