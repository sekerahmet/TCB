package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Birden çok değer (6.3): 'TR_return a, b' fonksiyonun sonuçlarını birlikte döndürür. Çağrının sonucu tek
// bir değer değildir: yalnız açılır (TR_var a, b = f()), aynı sonuçlarla 'TR_return f()' ile iletilir ya
// da deyim olarak atılır. Tek değer ya da argüman olarak kullanımı çalışma hatasıdır (denetimi 4a'da).

// MultiVal bir çağrının birden çok sonucudur; programın değeri olmaz, çağrı yerinde açılır.
type MultiVal struct{ Values []Value }

func (m *MultiVal) Type() ValueType { return ValList }
func (m *MultiVal) IsTruthy() bool  { return true }
func (m *MultiVal) Inspect() string {
	parts := make([]string, len(m.Values))
	for i, x := range m.Values {
		parts[i] = x.Inspect()
	}
	return strings.Join(parts, ", ")
}

// tuple 'TR_return a, b'nin değerlerini hesaplar.
func (c *compiler) tuple(k *compilerScope, e *ast.TupleExpression) exprCode {
	parts := c.expressions(k, e.Elements)
	return func(v *VM, env *Environment) (Value, error) {
		vals := make([]Value, len(parts))
		for i, p := range parts {
			x, err := p(v, env)
			if err != nil {
				return nil, err
			}
			vals[i] = x
		}
		return &MultiVal{Values: vals}, nil
	}
}

// singleResult çağrının sonucunun tek değer olarak kullanıldığı yerin kodudur: birden çok sonuç orada
// hatadır, ileti açmayı önerir.
func singleResult(call exprCode, e *ast.CallExpression) exprCode {
	return func(v *VM, env *Environment) (Value, error) {
		x, err := call(v, env)
		if m, ok := x.(*MultiVal); ok && err == nil {
			return nil, messages.At(e.Line(), 0, messages.New(messages.H2708, len(m.Values)))
		}
		return x, err
	}
}

// resultsOf açılabilen yerin (çoklu tanım ve atamanın tek sağ yanı, TR_return'ün değeri, deyim) ifade
// kodudur: çağrının birden çok sonucu geçer.
func (c *compiler) resultsOf(k *compilerScope, e ast.Expression) exprCode {
	if call, ok := e.(*ast.CallExpression); ok {
		return c.callExpr(k, call)
	}
	return c.expression(k, e)
}
