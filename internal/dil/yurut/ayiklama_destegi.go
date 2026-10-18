package yurut

import "tr_dili/internal/dil/agac"

// Bu dosyadaki yöntemler hata ayıklayıcının (tr_dili/internal/arac/ayikla) VM'nin iç
// durumunu okuyup ifade değerlendirebilmesi için dışa açılmıştır. Hata ayıklayıcının
// verdiği ortamın düzeni derleme anında bilinmediği için adlar adla çözülür.

// EvalExpression ifadeyi verilen ortamda değerlendirir.
func (v *VM) EvalExpression(expr agac.Expression, env *Environment) (Value, error) {
	return v.dinamikIfade(expr, env)
}

// ExecStatements deyimleri verilen ortamda çalıştırır ve son değeri döndürür;
// döndür/kır/devam sinyalleri yok sayılır.
func (v *VM) ExecStatements(stmts []agac.Statement, env *Environment) (Value, error) {
	val, _, err := v.dinamikDeyimler(stmts, env)
	return val, err
}
