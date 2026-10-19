package interp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// maxCallDepth iç içe kullanıcı fonksiyonu çağrılarının üst sınırıdır. Sınırsız özyineleme
// Go yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda 'dene/yakala' ile
// yakalanabilen bir hata döner.
const maxCallDepth = 10000

// selfName yapı metotlarında nesneyi gösteren 'kendi' anahtar kelimesinin sözlükteki
// adıdır (A051).
var selfName = codes.Name(codes.A051)

// ctorName nesne oluşturulurken çağrılan metodun adıdır (7.4; sözlükte A102).
var ctorName = codes.Name(codes.A102)

// checkArity koddaki doğrudan çağrılarda (f(1, 2), x |> f) kullanıcı fonksiyonunun argüman
// sayısını ve adlandırılmış argümanlarını denetler. Yerleşiklerin geri çağırdığı fonksiyonlar
// (filtrele, haritala, indirge ...) bu denetime girmez; onlar fazladan argüman gönderebilir.
func checkArity(fnVal Value, nargs int, named map[string]Value, line int) error {
	fn, ok := fnVal.(*FuncVal)
	if !ok {
		return nil
	}
	// Hızlı yol: adlı argüman yoksa ve verilmeyen her parametrenin varsayılanı varsa sorun
	// yoktur. Hata mesajı için gereken metin ve haritalar yalnızca hata yolunda kurulur.
	if len(named) == 0 && nargs <= len(fn.Parameters) && defaultsSuffice(fn, nargs) {
		return nil
	}
	name := "'" + fn.Name + "' fonksiyonu"
	if fn.Name == "" {
		name = "İsimsiz fonksiyon"
	}
	known := make(map[string]bool, len(fn.Parameters))
	for _, p := range fn.Parameters {
		known[p.Name] = true
	}
	for k := range named {
		if !known[k] {
			return fmt.Errorf("Satır %d: %s '%s' adında bir parametre almaz%s", line, name, k, suggestParam(k, fn.Parameters))
		}
	}
	if nargs > len(fn.Parameters) {
		return fmt.Errorf("Satır %d: %s en fazla %d argüman alır, %d verildi", line, name, len(fn.Parameters), nargs)
	}
	// Konumlu argümanın doldurduğu bir parametre adıyla da verilirse adlı değer sessizce
	// yok sayılırdı; hangisinin kastedildiği belli olmadığından hatadır.
	for i, p := range fn.Parameters[:nargs] {
		if _, ok := named[p.Name]; ok {
			return fmt.Errorf("Satır %d: %s '%s' parametresi hem sırayla (%d. argüman) hem adıyla verildi", line, name, p.Name, i+1)
		}
	}
	var missing []string
	required := 0
	for i, p := range fn.Parameters {
		if p.DefaultValue == nil {
			required++
		}
		if i >= nargs && named[p.Name] == nil && p.DefaultValue == nil {
			missing = append(missing, "'"+p.Name+"'")
		}
	}
	if len(missing) > 0 {
		// Sayılı mesaj yalnızca sayılar eksikliği açıklıyorsa verilir. Adlı argümanlar ya da
		// zorunlu olandan önce gelen varsayılanlı parametreler işin içindeyse ("1 argüman
		// bekler, 1 verildi") kendisiyle çelişirdi; o zaman eksikler adlarıyla söylenir.
		if len(named) == 0 && nargs < required {
			return fmt.Errorf("Satır %d: %s %d argüman bekler, %d verildi (eksik: %s)", line, name, required, nargs, strings.Join(missing, ", "))
		}
		what := "argümanı"
		if len(missing) > 1 {
			what = "argümanları"
		}
		return fmt.Errorf("Satır %d: %s için zorunlu %s %s verilmedi", line, name, strings.Join(missing, ", "), what)
	}
	return nil
}

// checkStructArgs kurucusu olmayan bir yapının oluşturulmasındaki argümanları alanlara
// göre denetler: alan sayısından fazla konumlu argüman ya da alan olmayan bir adla verilen
// argüman hatadır.
func checkStructArgs(def *StructDefVal, nargs int, named map[string]Value, line int) error {
	if nargs <= len(def.Fields) && len(named) == 0 {
		return nil
	}
	nameList := make([]string, len(def.Fields))
	for i, f := range def.Fields {
		nameList[i] = f.Name
	}
	for k := range named {
		if !slices.Contains(nameList, k) {
			suggestion := ""
			if o := turkish.Closest(k, nameList); o != "" {
				suggestion = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return fmt.Errorf("Satır %d: '%s' yapısının '%s' adında bir alanı yok%s", line, def.Name, k, suggestion)
		}
	}
	if nargs > len(def.Fields) {
		return fmt.Errorf("Satır %d: '%s' yapısı en fazla %d argüman alır (alanlar: %s), %d verildi", line, def.Name, len(def.Fields), strings.Join(nameList, ", "), nargs)
	}
	for i, f := range nameList[:nargs] {
		if _, ok := named[f]; ok {
			return fmt.Errorf("Satır %d: '%s' yapısının '%s' alanı hem sırayla (%d. argüman) hem adıyla verildi", line, def.Name, f, i+1)
		}
	}
	return nil
}

func suggestParam(name string, params []ast.Param) string {
	var names []string
	for _, p := range params {
		names = append(names, p.Name)
	}
	if o := turkish.Closest(name, names); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

func (v *VM) ApplyFunction(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
	switch fn := fnVal.(type) {
	case *FuncVal:
		fk := fn.codeVal
		if fk == nil {
			fk = v.prepareFuncCode(fn)
		}
		return v.callPrepared(fn, fk, fk.getEnv(fn.Env), fn.BoundSelf, args, len(args), named, line)

	case *BuiltinVal:
		if fn.NoNamedArgs && len(named) > 0 {
			return nil, lineSuffix(line, NoNamedArgsError(fn.Name, named))
		}
		res, err := fn.Fn(v, args, named)
		if err != nil {
			return nil, lineSuffix(line, err)
		}
		return res, nil

	case *StructDefVal:
		inst := &StructInstanceVal{
			Def:    fn,
			Fields: make(map[string]Value),
		}
		if fn.fieldCode == nil {
			// Derleyici dışında kurulmuş tanım: varsayılanlar genel kapsamda değerlendirilir
			c := v.newCompiler()
			fn.fieldCode = c.fieldDefaults(c.root, fn.Fields)
			fn.fieldEnv = v.GlobalEnv
		}
		fieldEnv := fn.fieldEnv
		if fieldEnv == nil {
			fieldEnv = v.GlobalEnv
		}
		initMethod, withCtor := fn.Methods[ctorName]
		if !withCtor {
			// kurucu yoksa argümanlar alanlara gider: fazlası ya da bilinmeyen adlısı hatadır
			if err := checkStructArgs(fn, len(args), named, line); err != nil {
				return nil, err
			}
		}
		for i, f := range fn.Fields {
			if i < len(args) {
				inst.Fields[f.Name] = args[i]
			} else if named != nil && named[f.Name] != nil {
				inst.Fields[f.Name] = named[f.Name]
			} else if f.DefaultValue != nil && i < len(fn.fieldCode) && fn.fieldCode[i] != nil {
				defVal, err := fn.fieldCode[i](v, fieldEnv)
				if err != nil {
					return nil, err
				}
				inst.Fields[f.Name] = defVal
			} else {
				inst.Fields[f.Name] = NULL
			}
		}
		if withCtor {
			// kurucu varsa argümanlar onun imzasına göre denetlenir
			if err := checkArity(initMethod, len(args), named, line); err != nil {
				return nil, err
			}
			boundInit := &FuncVal{
				Name:       initMethod.Name,
				Parameters: initMethod.Parameters,
				Body:       initMethod.Body,
				Env:        initMethod.Env,
				BoundSelf:  inst,
				codeVal:    initMethod.codeVal,
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

// callPrepared bir kullanıcı fonksiyonunu fnEnv ortamında çalıştırır. args nil değilse
// argümanlar sırayla bağlanır; nil ise ilk nargs argüman zaten yuvalara yazılmıştır
// (yalnızca fk.basit iken). Kalan parametreler adlı argümanlardan, varsayılanlardan ya da
// 'yok' ile doldurulur. kendi, metot çağrılarında nesnedir (bağlı fonksiyon değerinin
// BoundSelf'i ya da doğrudan derlenmiş 'nesne.metot(...)' çağrısının nesnesi).
func (v *VM) callPrepared(fn *FuncVal, fk *funcCode, fnEnv *Environment, self Value, args []Value, nargs int, named map[string]Value, line int) (val Value, err error) {
	if fk.source != v.source {
		// Başka bir dosyada (modülde) tanımlı fonksiyon: o dosyanın bağlamında çalışır.
		return v.callInSource(fn, fk, fnEnv, self, args, nargs, named, line)
	}
	fnEnv.IsFuncRoot = true
	v.handOff() // özyinelemeli (döngüsüz) hesaplarda da kilit devredilebilsin
	if fk.hasDefer {
		// 'ertele' blokları bir çöküşte de (Go panic) çalışmalıdır; hataları yutulmaz (9.4)
		defer func() {
			if derr := fnEnv.RunDefers(); derr != nil {
				val, err = nil, addDeferError(err, derr)
			}
		}()
	}
	if len(v.CallStack) >= maxCallDepth {
		return nil, fmt.Errorf("Satır %d: Özyineleme derinliği sınırı aşıldı (en fazla %d iç içe çağrı); fonksiyon kendini durmadan çağırıyor olabilir", line, maxCallDepth)
	}
	fnName := fn.Name
	if fnName == "" {
		fnName = "<anonim>"
	}
	v.CallStack = append(v.CallStack, StackFrame{
		FuncName: fnName,
		Line:     line,
	})
	val, err = v.functionBody(fn, fk, fnEnv, self, args, nargs, named)
	if len(v.CallStack) > 0 {
		v.CallStack = v.CallStack[:len(v.CallStack)-1]
	}
	if !fk.hasDefer {
		if derr := fnEnv.RunDefers(); derr != nil {
			val, err = nil, addDeferError(err, derr)
		}
		// Hata ayıklayıcı bağlıyken ortam onun elinde kalabilir; havuza konmaz.
		if fk.pooled && v.Debugger == nil {
			fk.release(fnEnv)
		}
	}
	return val, err
}

func (v *VM) functionBody(fn *FuncVal, fk *funcCode, fnEnv *Environment, self Value, args []Value, nargs int, named map[string]Value) (Value, error) {
	if self != nil {
		if fk.self >= 0 {
			fnEnv.vals[fk.self] = self
		} else {
			fnEnv.Define(selfName, self, false)
		}
	}
	for i, param := range fn.Parameters {
		if i >= len(fk.param) {
			break
		}
		slot := fk.param[i]
		if i < nargs {
			if args != nil {
				fnEnv.vals[slot] = args[i]
			}
		} else if named != nil && named[param.Name] != nil {
			fnEnv.vals[slot] = named[param.Name]
		} else if param.DefaultValue != nil && fk.defaultVal[i] != nil {
			defVal, err := fk.defaultVal[i](v, fnEnv)
			if err != nil {
				return nil, err
			}
			fnEnv.vals[slot] = defVal
		} else {
			fnEnv.vals[slot] = NULL
		}
	}
	val, _, err := fk.bodyBlock.run(v, fnEnv)
	if err != nil {
		return nil, err
	}
	return val, nil
}
