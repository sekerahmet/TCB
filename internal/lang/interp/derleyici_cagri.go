package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- ÇAĞRILAR ---

func (c *derleyici) cagri(k *derleyiciKapsam, e *ast.CallExpression) ifadeKodu {
	fonk := c.ifade(k, e.Function)
	argKod := c.ifadeler(k, e.Arguments)
	nargs := len(argKod)
	line := e.Line()

	if len(e.NamedArgs) == 0 {
		// x.metot(...): yapı metotları ile Metin/Liste/Sözlük yerleşik metotları, bağlı
		// fonksiyon/yerleşik değeri ayrılmadan doğrudan çağrılır (liste.ekle(x) argüman
		// dilimi de ayırmaz). Diğer durumlarda üye değeri okunup çağrılır.
		if mem, ok := e.Function.(*ast.MemberExpression); ok {
			nesne := c.ifade(k, mem.Object)
			u := uyeCoz(mem.Property) // ad → kod ve Metin/Liste/Sözlük metotları derleme anında
			ekle := u.cekirdek != nil && u.kod == uyeEkle
			uyeSatiri := mem.Line() // geri çağrıların hata satırı (bağlı yerleşikteki gibi)
			return func(v *VM, env *Environment) (Value, error) {
				obj, err := nesne(v, env)
				if err != nil {
					return nil, err
				}
				if inst, ok := obj.(*StructInstanceVal); ok {
					if _, alan := inst.Fields[mem.Property]; !alan {
						if metot, ok := inst.Def.Methods[mem.Property]; ok && metot.kod != nil {
							return v.metotCagir(metot, inst, argKod, env, line)
						}
					}
				} else if m := metotBul(obj, u.cekirdek); m != nil {
					if l, liste := obj.(*ListVal); liste && ekle && nargs == 1 {
						av, err := argKod[0](v, env)
						if err != nil {
							return nil, err
						}
						l.Elements = append(l.Elements, av)
						return l, nil
					}
					// Yerleşik çağrısıyla aynı sıra: önce tüm argümanlar, sonra gövde
					args, err := argumanlar(v, env, argKod)
					if err != nil {
						return nil, err
					}
					res, err := m.f(v, obj, args, uyeSatiri)
					if err != nil {
						return nil, satirEki(line, err)
					}
					return res, nil
				}
				fnVal, err := v.uyeDegeri(obj, mem, u)
				if err != nil {
					return nil, err
				}
				return v.konumluCagir(fnVal, argKod, env, line)
			}
		}
		return func(v *VM, env *Environment) (Value, error) {
			fnVal, err := fonk(v, env)
			if err != nil {
				return nil, err
			}
			return v.konumluCagir(fnVal, argKod, env, line)
		}
	}

	adlar := make([]string, len(e.NamedArgs))
	adliKod := make([]ifadeKodu, len(e.NamedArgs))
	for i, na := range e.NamedArgs {
		adlar[i] = na.Name
		adliKod[i] = c.ifade(k, na.Value)
	}
	return func(v *VM, env *Environment) (Value, error) {
		fnVal, err := fonk(v, env)
		if err != nil {
			return nil, err
		}
		args := make([]Value, 0, nargs)
		for _, a := range argKod {
			av, err := a(v, env)
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
		named := make(map[string]Value)
		for i, na := range adliKod {
			nv, err := na(v, env)
			if err != nil {
				return nil, err
			}
			named[adlar[i]] = nv
		}
		if err := checkArity(fnVal, len(args), named, line); err != nil {
			return nil, err
		}
		return v.ApplyFunction(fnVal, args, named, line)
	}
}

// argumanlar argüman kodlarını sırayla hesaplar.
func argumanlar(v *VM, env *Environment, argKod []ifadeKodu) ([]Value, error) {
	args := make([]Value, 0, len(argKod))
	for _, a := range argKod {
		av, err := a(v, env)
		if err != nil {
			return nil, err
		}
		args = append(args, av)
	}
	return args, nil
}

// metotCagir bir yapı metodunu, bağlı fonksiyon değeri ayırmadan, kendi = inst ile çağırır
// (bağlı değerle yapılan çağrıyla aynı denetim ve sırayla).
func (v *VM) metotCagir(metot *FuncVal, inst Value, argKod []ifadeKodu, env *Environment, line int) (Value, error) {
	nargs := len(argKod)
	fk := metot.kod
	if fk.basit && nargs <= len(metot.Parameters) && varsayilanlarYeter(metot, nargs) {
		fnEnv := fk.ortamAl(metot.Env)
		for i, a := range argKod {
			av, err := a(v, env)
			if err != nil {
				return nil, err
			}
			fnEnv.vals[fk.param[i]] = av
		}
		return v.cagirHazir(metot, fk, fnEnv, inst, nil, nargs, nil, line)
	}
	args, err := argumanlar(v, env, argKod)
	if err != nil {
		return nil, err
	}
	if err := checkArity(metot, len(args), nil, line); err != nil {
		return nil, err
	}
	return v.cagirHazir(metot, fk, fk.ortamAl(metot.Env), inst, args, len(args), nil, line)
}

// konumluCagir fnVal'ı (adlı argümansız) konumlu argümanlarla çağırır.
func (v *VM) konumluCagir(fnVal Value, argKod []ifadeKodu, env *Environment, line int) (Value, error) {
	nargs := len(argKod)
	switch fn := fnVal.(type) {
	case *FuncVal:
		// Hızlı yol: derlenmiş kullanıcı fonksiyonu. Argümanlar ara dilim kurulmadan
		// doğrudan yeni ortamın yuvalarına hesaplanır.
		if fk := fn.kod; fk != nil && fk.basit && nargs <= len(fn.Parameters) && varsayilanlarYeter(fn, nargs) {
			fnEnv := fk.ortamAl(fn.Env)
			for i, a := range argKod {
				av, err := a(v, env)
				if err != nil {
					return nil, err
				}
				fnEnv.vals[fk.param[i]] = av
			}
			return v.cagirHazir(fn, fk, fnEnv, fn.BoundSelf, nil, nargs, nil, line)
		}
	case *BuiltinVal:
		// Hızlı yol: tek argümanlı yerleşik (argüman dilimi ayrılmaz)
		if fn.Tek != nil && nargs == 1 {
			av, err := argKod[0](v, env)
			if err != nil {
				return nil, err
			}
			res, err := fn.Tek(v, av)
			if err != nil {
				return nil, satirEki(line, err)
			}
			return res, nil
		}
	}
	args, err := argumanlar(v, env, argKod)
	if err != nil {
		return nil, err
	}
	if err := checkArity(fnVal, len(args), nil, line); err != nil {
		return nil, err
	}
	return v.ApplyFunction(fnVal, args, nil, line)
}

// varsayilanlarYeter verilmeyen her parametrenin varsayılanı var mı (checkArity'nin hızlı yolu)?
func varsayilanlarYeter(fn *FuncVal, nargs int) bool {
	for _, p := range fn.Parameters[nargs:] {
		if p.DefaultValue == nil {
			return false
		}
	}
	return true
}
