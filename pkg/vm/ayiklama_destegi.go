package vm

import "tr_dili/pkg/ast"

// Bu dosyadaki yöntemler hata ayıklayıcının (tr_dili/pkg/hataayiklama) VM'nin iç
// durumunu okuyup ifade değerlendirebilmesi için dışa açılmıştır.

// EvalExpression ifadeyi verilen ortamda değerlendirir.
func (v *VM) EvalExpression(expr ast.Expression, env *Environment) (Value, error) {
	return v.evalExpression(expr, env)
}

// ExecStatements deyimleri verilen ortamda çalıştırır ve son değeri döndürür;
// döndür/kır/devam sinyalleri yok sayılır.
func (v *VM) ExecStatements(stmts []ast.Statement, env *Environment) (Value, error) {
	val, _, err := v.execStatements(stmts, env)
	return val, err
}

// Outer bir dıştaki kapsamı döndürür (genel kapsamda nil).
func (e *Environment) Outer() *Environment {
	return e.outer
}

// EachLocal yalnızca bu kapsamda tanımlı adları fn'e verir.
func (e *Environment) EachLocal(fn func(name string, val Value)) {
	for k, v := range e.store {
		fn(k, v)
	}
}
