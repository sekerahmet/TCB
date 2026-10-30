package interp

import (
	"fmt"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yorumlayıcının kaydettiği adlar kod listesinden gelir (internal/lang/codes). Adlar kayıt sırasında
// (VM kurulurken) bir kez okunur; çalışma anında sözlüğe bakılmaz.

// RegisterBuiltinCode registers a builtin code from the dictionary (fn) in the global
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
func NoNamedArgsError(name any, named map[string]Value) error {
	nameList := make([]string, 0, len(named))
	for a := range named {
		nameList = append(nameList, a)
	}
	slices.Sort(nameList)
	return messages.New(messages.H2427, name, nameList[0])
}

// DefineBuiltin binds a ready builtin value (its name must be the code's name) in the global
// scope under the code's accepted spellings (codes.Spellings: just the name), e.g. the value
// with which the tensor engine wraps an older builtin.
func (v *VM) DefineBuiltin(kd codes.Code, b *BuiltinVal) *BuiltinVal {
	if !kd.IsBuiltin() {
		panic(fmt.Sprintf("builtin registry: %s is not a builtin code (fn or const, not arg)", kd))
	}
	for _, spelling := range codes.Spellings(kd) {
		v.GlobalEnv.Define(spelling, b, true)
	}
	return b
}

// DefineConstant binds a builtin constant (kind "sabit" in definitions.json; e.g. pi) with its
// value in the global scope. A constant is not called; it is read as a value and cannot be
// reassigned.
func (v *VM) DefineConstant(kd codes.Code, value Value) {
	if !kd.IsBuiltin() {
		panic(fmt.Sprintf("constant registry: %s is not a builtin code (fn or const, not arg)", kd))
	}
	if v.builtinConstants == nil {
		v.builtinConstants = map[string]Value{}
	}
	for _, spelling := range codes.Spellings(kd) {
		v.GlobalEnv.Define(spelling, value, true)
		v.builtinConstants[spelling] = value
	}
}

// IsBuiltinConstant reports whether ad is a builtin constant still holding val in scope (if
// the user defined a variable with the same name, the value differs and it returns false).
func (v *VM) IsBuiltinConstant(name string, val Value) bool {
	s, ok := v.builtinConstants[name]
	return ok && s == val
}

// Builtin returns the builtin registered in the global scope for a code, or nil.
func (v *VM) Builtin(kd codes.Code) *BuiltinVal {
	b, _ := v.GlobalEnv.localValue(codes.Name(kd)).(*BuiltinVal)
	return b
}

// removedMemberHint Metin, Liste ve Sözlük üyelerinden birinin ASCII biçimi ya da
// yabancı adı için ipucunu döndürür.
func removedMemberHint(name string) *messages.Error {
	k, ok := codes.Suggest(codes.NamespaceMember, name, coreMember)
	if !ok {
		return nil
	}
	return hint.NotInTRHint(name, codes.NameRef(k))
}

// suggests err'in (ya da içindeki kodlu bir iletinin) değerlerinde k'nin adı (codes.Ref) var mı:
// tipin kendi iletisi o adı zaten öneriyor. İletinin metnine bakılmaz; değerlere bakılır.
func suggests(err error, k codes.Code) bool {
	m, ok := messages.As(err)
	if !ok {
		return false
	}
	for _, a := range m.Args {
		switch v := a.(type) {
		case codes.Ref:
			if v.Code == k {
				return true
			}
		case string:
			if v == codes.Spelling(k) || v == codes.Spelling(k)+"()" {
				return true
			}
		case error:
			if suggests(v, k) {
				return true
			}
		}
	}
	return false
}

// memberSuggestionError motor tiplerinde (Tensör, Parçalayıcı, VeriKümesi ...) bulunamayan bir üye
// adı bu tipin bir üyesinin ASCII biçimi ya da kaldırılmış adıysa, hatayı asıl adı öneren
// bir hatayla değiştirir; değilse hatayı olduğu gibi döndürür. Önerinin bu tipte gerçekten
// geçerli olduğu asıl adla yeniden okunarak doğrulanır (üye okumak yan etkisizdir:
// metotlar bağlı değer olarak döner, çağrılmaz). Yalnızca hata yolunda çalışır.
func memberSuggestionError(pg PropertyGetter, obj Value, prop string, err error) error {
	k, ok := codes.Suggest(codes.NamespaceMember, prop, nil)
	if !ok {
		return err
	}
	if suggests(err, k) {
		return err // tipin kendi hata mesajı asıl adı zaten öneriyor (ör. 'devrik()')
	}
	if _, e := pg.GetProperty(k, codes.Name(k)); e != nil {
		return err
	}
	return messages.New(messages.H2428, obj.Type(), prop, hint.NotInTRHint(prop, codes.NameRef(k)))
}
