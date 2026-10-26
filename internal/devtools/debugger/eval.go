package debugger

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// exprStatus bir metnin ilk deyimi olarak ayrıştırılma sonucudur.
type exprStatus int

const (
	exprParsed        exprStatus = iota // ilk deyim bir ifade
	exprSyntaxError                     // sözdizimi hatası ya da hiç deyim yok
	exprNotExpression                   // ilk deyim ifade değil
)

// parseFirstExpression metni env'in kapsamında ayrıştırır ve ilk deyiminin ifadesini verir (sonraki
// deyimler yok sayılır). Hangi kolun hangi iletiyi yazacağı çağıranındır.
func parseFirstExpression(text string, env *interp.Environment) (ast.Expression, exprStatus) {
	p := parser.New(source.TextLexerWithNames(text, env.Names()))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || len(prog.Statements) == 0 {
		return nil, exprSyntaxError
	}
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		return nil, exprNotExpression
	}
	return es.Expression, exprParsed
}

// evalQuiet ifadeyi hata ayıklayıcının kendi çıktısını bastırarak değerlendirir.
func (d *Debugger) evalQuiet(v *interp.VM, env *interp.Environment, e ast.Expression) (interp.Value, error) {
	return d.quietly(func() (interp.Value, error) { return v.EvalExpression(e, env) })
}

func (d *Debugger) checkWatchpoints(v *interp.VM, env *interp.Environment) string {
	for _, wp := range d.Watchpoints {
		if wp.IsCond {
			if e, status := parseFirstExpression(wp.Expression, env); status == exprParsed {
				val, err := d.evalQuiet(v, env, e)
				if err == nil && val.IsTruthy() {
					curStr := interp.True.Inspect()
					if wp.LastVal != curStr {
						wp.LastVal = curStr
						return "⚡ " + messages.Text(messages.H8032, wp.ID, wp.Expression)
					}
				} else {
					wp.LastVal = interp.False.Inspect()
				}
			}
		} else {
			if val, ok := env.Get(wp.Expression); ok {
				if old, changed := wp.observe(val); changed {
					return "⚡ " + messages.Text(messages.H8033, wp.ID, wp.Expression, v.Mask(old), v.Mask(wp.LastVal))
				}
			}
		}
	}
	return ""
}
