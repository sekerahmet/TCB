package interp

// RegisterBuiltin testlerin kendi yardımcı yerleşiklerini verilen adla genel kapsama kaydeder.
// Kod listesini atlayan bir kapı olduğu için yalnızca bu paketin testlerinde vardır; dilin
// yerleşikleri RegisterBuiltinKod ile sözlük koduyla kaydedilir (adları internal/lang/codes'da).
func (v *VM) RegisterBuiltin(name string, fn BuiltinFunc) {
	v.GlobalEnv.Define(name, &BuiltinVal{Name: name, Fn: fn}, true)
}
