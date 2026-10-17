package vm

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/turkce"
)

func (v *VM) evalCall(e *ast.CallExpression, env *Environment) (Value, error) {
	fnVal, err := v.evalExpression(e.Function, env)
	if err != nil {
		return nil, err
	}

	args := make([]Value, 0, len(e.Arguments))
	for _, arg := range e.Arguments {
		av, err := v.evalExpression(arg, env)
		if err != nil {
			return nil, err
		}
		args = append(args, av)
	}

	var named map[string]Value
	if len(e.NamedArgs) > 0 {
		named = make(map[string]Value)
		for _, na := range e.NamedArgs {
			nv, err := v.evalExpression(na.Value, env)
			if err != nil {
				return nil, err
			}
			named[na.Name] = nv
		}
	}

	if err := checkArity(fnVal, len(args), named, e.Line()); err != nil {
		return nil, err
	}
	return v.ApplyFunction(fnVal, args, named, e.Line())
}

// maxCallDepth iç içe kullanıcı fonksiyonu çağrılarının üst sınırıdır. Sınırsız özyineleme
// Go yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda 'dene/yakala' ile
// yakalanabilen bir hata döner.
const maxCallDepth = 10000

// checkArity koddaki doğrudan çağrılarda (f(1, 2), x |> f) kullanıcı fonksiyonunun argüman
// sayısını ve adlandırılmış argümanlarını denetler. Yerleşiklerin geri çağırdığı fonksiyonlar
// (filtrele, web işleyicileri...) bu denetime girmez; onlar fazladan argüman gönderebilir.
func checkArity(fnVal Value, nargs int, named map[string]Value, line int) error {
	fn, ok := fnVal.(*FuncVal)
	if !ok {
		return nil
	}
	ad := "'" + fn.Name + "' fonksiyonu"
	if fn.Name == "" {
		ad = "İsimsiz fonksiyon"
	}
	known := make(map[string]bool, len(fn.Parameters))
	for _, p := range fn.Parameters {
		known[p.Name] = true
	}
	for k := range named {
		if !known[k] {
			return fmt.Errorf("Satır %d: %s '%s' adında bir parametre almaz%s", line, ad, k, suggestParam(k, fn.Parameters))
		}
	}
	if nargs > len(fn.Parameters) {
		return fmt.Errorf("Satır %d: %s en fazla %d argüman alır, %d verildi", line, ad, len(fn.Parameters), nargs)
	}
	var eksik []string
	gerekli := 0
	for i, p := range fn.Parameters {
		if p.DefaultValue == nil {
			gerekli++
		}
		if i >= nargs && named[p.Name] == nil && p.DefaultValue == nil {
			eksik = append(eksik, "'"+p.Name+"'")
		}
	}
	if len(eksik) > 0 {
		return fmt.Errorf("Satır %d: %s %d argüman bekler, %d verildi (eksik: %s)", line, ad, gerekli, nargs+len(named), strings.Join(eksik, ", "))
	}
	return nil
}

func suggestParam(name string, params []ast.Param) string {
	var names []string
	for _, p := range params {
		names = append(names, p.Name)
	}
	if o := turkce.EnYakin(name, names); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

func (v *VM) ApplyFunction(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
	switch fn := fnVal.(type) {
	case *FuncVal:
		fnEnv := NewEnvironment(fn.Env)
		fnEnv.IsFuncRoot = true
		defer fnEnv.RunDefers()
		fnName := fn.Name
		if fnName == "" {
			fnName = "<anonim>"
		}
		v.stackMu.Lock()
		if len(v.CallStack) >= maxCallDepth {
			v.stackMu.Unlock()
			return nil, fmt.Errorf("Satır %d: Özyineleme derinliği sınırı aşıldı (en fazla %d iç içe çağrı); fonksiyon kendini durmadan çağırıyor olabilir", line, maxCallDepth)
		}
		v.CallStack = append(v.CallStack, StackFrame{
			FuncName: fnName,
			Line:     line,
		})
		v.stackMu.Unlock()
		defer func() {
			v.stackMu.Lock()
			if len(v.CallStack) > 0 {
				v.CallStack = v.CallStack[:len(v.CallStack)-1]
			}
			v.stackMu.Unlock()
		}()
		if fn.BoundSelf != nil {
			fnEnv.Define("kendi", fn.BoundSelf, false)
		}
		for i, param := range fn.Parameters {
			if i < len(args) {
				fnEnv.Define(param.Name, args[i], false)
			} else if named != nil && named[param.Name] != nil {
				fnEnv.Define(param.Name, named[param.Name], false)
			} else if param.DefaultValue != nil {
				defVal, err := v.evalExpression(param.DefaultValue, fn.Env)
				if err != nil {
					return nil, err
				}
				fnEnv.Define(param.Name, defVal, false)
			} else {
				fnEnv.Define(param.Name, NULL, false)
			}
		}
		val, _, err := v.execStatements(fn.Body.Statements, fnEnv)
		if err != nil {
			return nil, err
		}
		return val, nil

	case *BuiltinVal:
		res, err := fn.Fn(v, args, named)
		if err != nil {
			return nil, fmt.Errorf("Satır %d: %v", line, err)
		}
		return res, nil

	case *StructDefVal:
		inst := &StructInstanceVal{
			Def:    fn,
			Fields: make(map[string]Value),
		}
		for i, f := range fn.Fields {
			if i < len(args) {
				inst.Fields[f.Name] = args[i]
			} else if named != nil && named[f.Name] != nil {
				inst.Fields[f.Name] = named[f.Name]
			} else if f.DefaultValue != nil {
				defVal, err := v.evalExpression(f.DefaultValue, v.GlobalEnv)
				if err != nil {
					return nil, err
				}
				inst.Fields[f.Name] = defVal
			} else {
				inst.Fields[f.Name] = NULL
			}
		}
		if initMethod, ok := fn.Methods["kurucu"]; ok {
			boundInit := &FuncVal{
				Name:       initMethod.Name,
				Parameters: initMethod.Parameters,
				Body:       initMethod.Body,
				Env:        initMethod.Env,
				BoundSelf:  inst,
			}
			_, err := v.ApplyFunction(boundInit, args, named, line)
			if err != nil {
				return nil, err
			}
		}
		return inst, nil
	}

	return nil, fmt.Errorf("Satır %d: %s tipi bir fonksiyon olarak çağrılamaz", line, fnVal.Type())
}
