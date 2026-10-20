package interp

import "github.com/sekerahmet/TCB/internal/lang/ast"

// Bu dosyadaki yöntemler hata ayıklayıcının (github.com/sekerahmet/TCB/internal/devtools/debugger) VM'nin iç
// durumunu okuyup ifade değerlendirebilmesi için dışa açılmıştır. Hata ayıklayıcının
// verdiği ortamın düzeni derleme anında bilinmediği için adlar adla çözülür.

// CurrentSource returns the file of the running statement: a module brought in with 'aktar'
// (or code of it called from another file), nil for the main program (SourceFile, SourceCode).
// Statement lines refer to this file.
func (v *VM) CurrentSource() *Source { return v.source }

// EvalExpression evaluates an expression in the given environment.
func (v *VM) EvalExpression(expr ast.Expression, env *Environment) (Value, error) {
	return v.dynamicExpr(expr, env)
}

// ExecStatements runs statements in the given environment and returns the last value;
// return/break/continue signals are ignored.
func (v *VM) ExecStatements(stmts []ast.Statement, env *Environment) (Value, error) {
	val, _, err := v.dynamicStatements(stmts, env)
	return val, err
}
