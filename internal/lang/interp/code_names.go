package interp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Yorumlayıcının kaydettiği adlar kod listesinden gelir (internal/lang/codes). Adlar kayıt sırasında
// (VM kurulurken) bir kez okunur; çalışma anında sözlüğe bakılmaz.

// RegisterBuiltinCode registers a builtin code from the dictionary (G/T/V/W) in the global
// scope. The name comes from the dictionary; every operation has one name. A code that is not
// a builtin code or is missing from the dictionary is a developer error and panics at
// registration.
func (v *VM) RegisterBuiltinCode(kd codes.Code, fn BuiltinFunc) *BuiltinVal {
	return v.DefineBuiltin(kd, &BuiltinVal{Name: codes.Name(kd), Fn: fn})
}

// RegisterNoNamed registers a builtin that takes no named arguments: a call with named
// arguments is rejected before the body runs (see BuiltinVal.NoNamedArgs).
func (v *VM) RegisterNoNamed(kd codes.Code, fn BuiltinFunc) *BuiltinVal {
	b := v.RegisterBuiltinCode(kd, fn)
	b.NoNamedArgs = true
	return b
}

// NoNamedArgsError reports that named arguments were given to a builtin or method that takes
// none; the format matches the unknown-name error of builtins that do take named arguments.
func NoNamedArgsError(ad string, named map[string]Value) error {
	adlar := make([]string, 0, len(named))
	for a := range named {
		adlar = append(adlar, a)
	}
	slices.Sort(adlar)
	return fmt.Errorf("%s() '%s' adında bir parametre almaz (adlı argüman almaz)", ad, adlar[0])
}

// DefineBuiltin binds a ready builtin value (its name must be the code's name) in the global
// scope under the code's accepted spellings (codes.Spellings: just the name), e.g. the value
// with which the tensor engine wraps an older builtin.
func (v *VM) DefineBuiltin(kd codes.Code, b *BuiltinVal) *BuiltinVal {
	if !kd.IsBuiltin() {
		panic(fmt.Sprintf("yerleşik kaydı: %s bir yerleşik kodu (G/T/V/W, x9xx dışı) değil", kd))
	}
	for _, yazim := range codes.Spellings(kd) {
		v.GlobalEnv.Define(yazim, b, true)
	}
	return b
}

// DefineConstant binds a builtin constant (kind "sabit" in definitions.json; e.g. pi) with its
// value in the global scope. A constant is not called; it is read as a value and cannot be
// reassigned.
func (v *VM) DefineConstant(kd codes.Code, deger Value) {
	if !kd.IsBuiltin() {
		panic(fmt.Sprintf("sabit kaydı: %s bir yerleşik kodu (G/T/V/W, x9xx dışı) değil", kd))
	}
	if v.yerlesikSabitler == nil {
		v.yerlesikSabitler = map[string]Value{}
	}
	for _, yazim := range codes.Spellings(kd) {
		v.GlobalEnv.Define(yazim, deger, true)
		v.yerlesikSabitler[yazim] = deger
	}
}

// IsBuiltinConstant reports whether ad is a builtin constant still holding val in scope (if
// the user defined a variable with the same name, the value differs and it returns false).
func (v *VM) IsBuiltinConstant(ad string, val Value) bool {
	s, ok := v.yerlesikSabitler[ad]
	return ok && s == val
}

// Builtin returns the builtin registered in the global scope for a code, or nil.
func (v *VM) Builtin(kd codes.Code) *BuiltinVal {
	b, _ := v.GlobalEnv.yerelDeger(codes.Name(kd)).(*BuiltinVal)
	return b
}

// kaldirilanUyeIpucu Metin, Liste ve Sözlük üyelerinden birinin ASCII biçimi ya da
// kaldırılmış adı için ipucunu döndürür.
func kaldirilanUyeIpucu(ad string) string {
	k, ok := codes.Suggest(codes.NamespaceMember, ad, cekirdekUyesi)
	if !ok {
		return ""
	}
	return hint.NotInTRHint(ad, codes.Name(k))
}

// uyeOneriHatasi motor tiplerinde (Tensör, Parçalayıcı, VeriKümesi ...) bulunamayan bir üye
// adı bu tipin bir üyesinin ASCII biçimi ya da kaldırılmış adıysa, hatayı asıl adı öneren
// bir hatayla değiştirir; değilse hatayı olduğu gibi döndürür. Önerinin bu tipte gerçekten
// geçerli olduğu asıl adla yeniden okunarak doğrulanır (üye okumak yan etkisizdir:
// metotlar bağlı değer olarak döner, çağrılmaz). Yalnızca hata yolunda çalışır.
func uyeOneriHatasi(pg PropertyGetter, obj Value, prop string, err error) error {
	k, ok := codes.Suggest(codes.NamespaceMember, prop, nil)
	if !ok {
		return err
	}
	if strings.Contains(err.Error(), "'"+codes.Name(k)) {
		return err // tipin kendi hata mesajı asıl adı zaten öneriyor (ör. 'devrik()')
	}
	if _, e := pg.GetProperty(k, codes.Name(k)); e != nil {
		return err
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", obj.Type(), prop, hint.NotInTRHint(prop, codes.Name(k)))
}
