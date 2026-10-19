package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- ÇAĞRILAR ---

func (c *compiler) call(k *compilerScope, e *ast.CallExpression) exprCode {
	fun := c.expression(k, e.Function)
	argCode := c.expressions(k, e.Arguments)
	nargs := len(argCode)
	line := e.Line()

	if len(e.NamedArgs) == 0 {
		// x.metot(...): yapı metotları ile Metin/Liste/Sözlük yerleşik metotları, bağlı
		// fonksiyon/yerleşik değeri ayrılmadan doğrudan çağrılır (liste.ekle(x) argüman
		// dilimi de ayırmaz). Diğer durumlarda üye değeri okunup çağrılır.
		if mem, ok := e.Function.(*ast.MemberExpression); ok {
			object := c.expression(k, mem.Object)
			u := resolveMember(mem.Property) // ad → kod ve Metin/Liste/Sözlük metotları derleme anında
			add := u.core != nil && u.codeVal == addMember
			memberLine := mem.Line() // geri çağrıların hata satırı (bağlı yerleşikteki gibi)
			return func(v *VM, env *Environment) (Value, error) {
				obj, err := object(v, env)
				if err != nil {
					return nil, err
				}
				if inst, ok := obj.(*StructInstanceVal); ok {
					if _, field := inst.Fields[mem.Property]; !field {
						if meth, ok := inst.Def.Methods[mem.Property]; ok && meth.codeVal != nil {
							return v.callMethod(meth, inst, argCode, env, line)
						}
					}
				} else if m := findMethod(obj, u.core); m != nil {
					if l, listVal := obj.(*ListVal); listVal && add && nargs == 1 {
						av, err := argCode[0](v, env)
						if err != nil {
							return nil, err
						}
						l.Elements = append(l.Elements, av)
						return l, nil
					}
					// Yerleşik çağrısıyla aynı sıra: önce tüm argümanlar, sonra gövde
					args, err := arguments(v, env, argCode)
					if err != nil {
						return nil, err
					}
					res, err := m.f(v, obj, args, memberLine)
					if err != nil {
						return nil, lineSuffix(line, err)
					}
					return res, nil
				}
				fnVal, err := v.memberValue(obj, mem, u)
				if err != nil {
					return nil, err
				}
				return v.callAt(fnVal, argCode, env, line)
			}
		}
		return func(v *VM, env *Environment) (Value, error) {
			fnVal, err := fun(v, env)
			if err != nil {
				return nil, err
			}
			return v.callAt(fnVal, argCode, env, line)
		}
	}

	nameList := make([]string, len(e.NamedArgs))
	namedCode := make([]exprCode, len(e.NamedArgs))
	for i, na := range e.NamedArgs {
		nameList[i] = na.Name
		namedCode[i] = c.expression(k, na.Value)
	}
	return func(v *VM, env *Environment) (Value, error) {
		fnVal, err := fun(v, env)
		if err != nil {
			return nil, err
		}
		args := make([]Value, 0, nargs)
		for _, a := range argCode {
			av, err := a(v, env)
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
		named := make(map[string]Value)
		for i, na := range namedCode {
			nv, err := na(v, env)
			if err != nil {
				return nil, err
			}
			named[nameList[i]] = nv
		}
		if err := checkArity(fnVal, len(args), named, line); err != nil {
			return nil, err
		}
		return v.ApplyFunction(fnVal, args, named, line)
	}
}

// arguments argüman kodlarını sırayla hesaplar.
func arguments(v *VM, env *Environment, argCode []exprCode) ([]Value, error) {
	args := make([]Value, 0, len(argCode))
	for _, a := range argCode {
		av, err := a(v, env)
		if err != nil {
			return nil, err
		}
		args = append(args, av)
	}
	return args, nil
}

// callMethod bir yapı metodunu, bağlı fonksiyon değeri ayırmadan, kendi = inst ile çağırır
// (bağlı değerle yapılan çağrıyla aynı denetim ve sırayla).
func (v *VM) callMethod(meth *FuncVal, inst Value, argCode []exprCode, env *Environment, line int) (Value, error) {
	nargs := len(argCode)
	fk := meth.codeVal
	if fk.simple && nargs <= len(meth.Parameters) && defaultsSuffice(meth, nargs) {
		fnEnv := fk.getEnv(meth.Env)
		for i, a := range argCode {
			av, err := a(v, env)
			if err != nil {
				return nil, err
			}
			fnEnv.vals[fk.param[i]] = av
		}
		return v.callPrepared(meth, fk, fnEnv, inst, nil, nargs, nil, line)
	}
	args, err := arguments(v, env, argCode)
	if err != nil {
		return nil, err
	}
	if err := checkArity(meth, len(args), nil, line); err != nil {
		return nil, err
	}
	return v.callPrepared(meth, fk, fk.getEnv(meth.Env), inst, args, len(args), nil, line)
}

// callAt fnVal'ı (adlı argümansız) konumlu argümanlarla çağırır.
func (v *VM) callAt(fnVal Value, argCode []exprCode, env *Environment, line int) (Value, error) {
	nargs := len(argCode)
	switch fn := fnVal.(type) {
	case *FuncVal:
		// Hızlı yol: derlenmiş kullanıcı fonksiyonu. Argümanlar ara dilim kurulmadan
		// doğrudan yeni ortamın yuvalarına hesaplanır.
		if fk := fn.codeVal; fk != nil && fk.simple && nargs <= len(fn.Parameters) && defaultsSuffice(fn, nargs) {
			fnEnv := fk.getEnv(fn.Env)
			for i, a := range argCode {
				av, err := a(v, env)
				if err != nil {
					return nil, err
				}
				fnEnv.vals[fk.param[i]] = av
			}
			return v.callPrepared(fn, fk, fnEnv, fn.BoundSelf, nil, nargs, nil, line)
		}
	case *BuiltinVal:
		// Hızlı yol: tek argümanlı yerleşik (argüman dilimi ayrılmaz)
		if fn.Single != nil && nargs == 1 {
			av, err := argCode[0](v, env)
			if err != nil {
				return nil, err
			}
			res, err := fn.Single(v, av)
			if err != nil {
				return nil, lineSuffix(line, err)
			}
			return res, nil
		}
	}
	args, err := arguments(v, env, argCode)
	if err != nil {
		return nil, err
	}
	if err := checkArity(fnVal, len(args), nil, line); err != nil {
		return nil, err
	}
	return v.ApplyFunction(fnVal, args, nil, line)
}

// defaultsSuffice verilmeyen her parametrenin varsayılanı var mı (checkArity'nin hızlı yolu)?
func defaultsSuffice(fn *FuncVal, nargs int) bool {
	for _, p := range fn.Parameters[nargs:] {
		if p.DefaultValue == nil {
			return false
		}
	}
	return true
}
