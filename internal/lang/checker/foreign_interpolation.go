package checker

import (
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkForeignInterpolation tırnaklı metinde başka dillerin yerleştirme yazımını uyarır (2.7):
// '{konu}' (Python, Rust) ya da '${konu}' (Kotlin, JavaScript, kabuk), konu o yerde bir ad ise; TR'de
// süslü parantez sıradan karakterdir ve metne olduğu gibi geçer, uyarı '\(konu)'ı önerir. Ham metinde
// (2.6a) yalnız '${konu}' uyarılır: istem şablonu gibi '{konu}'ı olduğu gibi tutan metin ham yazılır.
// Süslü parantezin içinde tek bir ad olmayan metin ('{"n": 1}', '{{konu}}') uyarı vermez. Emsal: Ruff
// RUF027 (yalnız kapsamdaki adlar), ESLint no-template-curly-in-string.
func (c *Checker) checkForeignInterpolation(s *ast.StringLiteral) {
	if s.Token.Type != lexer.TokString {
		return // sözlük değişmezinin çıplak ad anahtarı
	}
	r := []rune(s.Value)
	for i := 0; i < len(r); i++ {
		if r[i] != '{' || i > 0 && r[i-1] == '{' {
			continue
		}
		dollar := i > 0 && r[i-1] == '$'
		if s.Token.Raw && !dollar {
			continue
		}
		j := i + 1
		for j < len(r) && (unicode.IsLetter(r[j]) || r[j] == '_' || j > i+1 && unicode.IsDigit(r[j])) {
			j++
		}
		if j == i+1 || j >= len(r) || r[j] != '}' || j+1 < len(r) && r[j+1] == '}' {
			continue
		}
		name := string(r[i+1 : j])
		if _, ok := c.scope.Resolve(name); !ok {
			continue
		}
		start := i
		if dollar {
			start--
		}
		c.warnings = append(c.warnings, messages.At(s.Token.Line, 0, messages.New(messages.H1022, string(r[start:j+1]), name)))
	}
}
