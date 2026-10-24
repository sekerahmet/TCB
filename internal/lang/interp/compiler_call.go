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
			u := c.member(mem.Property) // ad → kod ve Metin/Liste/Sözlük metotları derleme anında
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
					// Metin/Liste/Sözlük metotları alıcıyı ve argümanları başka listelere
					// koyabilir: sahip liste ve argümanlar kaçar (ownership.go).
					escape(obj)
					if l, listVal := obj.(*ListVal); listVal && add && nargs == 1 {
						av, err := argCode[0](v, env)
						if err != nil {
							return nil, err
						}
						escape(av)
						l.Elements = append(l.Elements, av)
						return l, nil
					}
					// Yerleşik çağrısıyla aynı sıra: önce tüm argümanlar, sonra gövde
					args, err := arguments(v, env, argCode)
					if err != nil {
						return nil, err
					}
					escapeAll(args, nil)
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

	fun = c.namedCallee(k, e, fun)
	namedCode := c.compileNamedArgs(k, e.NamedArgs)
	return func(v *VM, env *Environment) (Value, error) {
		fnVal, err := fun(v, env)
		if err != nil {
			return nil, err
		}
		args, err := arguments(v, env, argCode)
		if err != nil {
			return nil, err
		}
		named, err := namedCode.eval(v, env)
		if err != nil {
			return nil, err
		}
		if err := checkArity(fnVal, len(args), named, line); err != nil {
			return nil, err
		}
		return v.applyCompiled(fnVal, args, named, line)
	}
}

// namedCallee adlı argümanlı çağrının çağrılanını verir. x.metot(a, ad: b) için bağlı metot
// değeri bir değişkene verilmediğinden alıcı kaçmaz (konumlu çağrıdaki gibi); yalnızca üye
// değeri okunup çağrılır.
func (c *compiler) namedCallee(k *compilerScope, e *ast.CallExpression, fun exprCode) exprCode {
	mem, ok := e.Function.(*ast.MemberExpression)
	if !ok {
		return fun
	}
	object := c.expression(k, mem.Object)
	u := c.member(mem.Property)
	return func(v *VM, env *Environment) (Value, error) {
		obj, err := object(v, env)
		if err != nil {
			return nil, err
		}
		if u.core != nil && findMethod(obj, u.core) != nil {
			escape(obj) // Metin/Liste/Sözlük metotları alıcıyı başka yere koyabilir (ownership.go)
		}
		return v.memberValue(obj, mem, u)
	}
}

// namedArgs derlenmiş adlı argümanlardır (ad: değer).
type namedArgs struct {
	names []string
	code  []exprCode
}

// compileNamedArgs adlı argümanların değer kodlarını derler.
func (c *compiler) compileNamedArgs(k *compilerScope, list []ast.NamedArg) namedArgs {
	n := namedArgs{names: make([]string, len(list)), code: make([]exprCode, len(list))}
	for i, na := range list {
		n.names[i] = na.Name
		n.code[i] = c.expression(k, na.Value)
	}
	return n
}

// eval adlı argümanları yazım sırasıyla hesaplar.
func (n namedArgs) eval(v *VM, env *Environment) (map[string]Value, error) {
	named := make(map[string]Value)
	for i, code := range n.code {
		nv, err := code(v, env)
		if err != nil {
			return nil, err
		}
		named[n.names[i]] = nv
	}
	return named, nil
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
				v.abandonEnv(fk, fnEnv)
				return nil, err
			}
			fnEnv.store(fk.param[i], av)
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

// abandonEnv bir argümanı hata verdiği için hiç çalıştırılmayan çağrının ortamını kapatır:
// önceki argümanların bağları kalkar ve ortam (havuzlanabilirse) havuza döner.
func (v *VM) abandonEnv(fk *funcCode, fnEnv *Environment) {
	fnEnv.closeEnv()
	if fk.pooled && v.Debugger == nil {
		fk.release(fnEnv)
	}
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
					v.abandonEnv(fk, fnEnv)
					return nil, err
				}
				fnEnv.store(fk.param[i], av)
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
			if !fn.Pure {
				escape(av)
			}
			newOp()
			res, err := fn.Single(v, av)
			if err != nil {
				return nil, lineSuffix(line, err)
			}
			v.afterBuiltin(fn, res, []Value{av}, nil)
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
	return v.applyCompiled(fnVal, args, nil, line)
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
