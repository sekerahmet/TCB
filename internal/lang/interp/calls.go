package interp

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// maxCallDepth iç içe kullanıcı fonksiyonu çağrılarının üst sınırıdır. Sınırsız özyineleme
// Go yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda 'dene/yakala' ile
// yakalanabilen bir hata döner.
const maxCallDepth = 10000

// selfName yapı metotlarında nesneyi gösteren 'kendi' anahtar kelimesinin sözlükteki
// adıdır (TR_kw_self).
var selfName = codes.Name(codes.KwSelf)

// ctorName nesne oluşturulurken çağrılan metodun adıdır (7.4; sözlükte TR_ctx_init).
var ctorName = codes.Name(codes.CtxInit)

// checkArity koddaki doğrudan çağrılarda (f(1, 2), x |> f) kullanıcı fonksiyonunun argüman
// sayısını ve adlandırılmış argümanlarını denetler. Yerleşiklerin geri çağırdığı fonksiyonlar
// (filtrele, haritala, indirge ...) bu denetime girmez; onlar fazladan argüman gönderebilir.
//
// owner, metot ya da kurucu çağrısında yapının tanımıdır (bağlı metotta BoundSelf'ten okunur): ileti
// öznesini o kurar (callLabel).
func checkArity(fnVal Value, owner *StructDefVal, nargs int, named map[string]Value, line int) error {
	fn, ok := fnVal.(*FuncVal)
	if !ok {
		return nil
	}
	// Hızlı yol: adlı argüman yoksa ve verilmeyen her parametrenin varsayılanı varsa sorun
	// yoktur. Hata mesajı için gereken metin ve haritalar yalnızca hata yolunda kurulur.
	if len(named) == 0 && nargs <= len(fn.Parameters) && defaultsSuffice(fn, nargs) {
		return nil
	}
	name := callLabel(fn, owner)
	known := make(map[string]bool, len(fn.Parameters))
	params := make([]string, len(fn.Parameters))
	for i, p := range fn.Parameters {
		known[p.Name] = true
		params[i] = p.Name
	}
	alignNamed(named, params)
	for k := range named {
		if !known[k] {
			return messages.At(line, 0, messages.New(messages.H2410, name, k, suggestParam(k, fn.Parameters)))
		}
	}
	if nargs > len(fn.Parameters) {
		return messages.At(line, 0, messages.New(messages.H2412, name, len(fn.Parameters), nargs))
	}
	// Konumlu argümanın doldurduğu bir parametre adıyla da verilirse adlı değer sessizce
	// yok sayılırdı; hangisinin kastedildiği belli olmadığından hatadır.
	for i, p := range fn.Parameters[:nargs] {
		if _, ok := named[p.Name]; ok {
			return messages.At(line, 0, messages.New(messages.H2413, name, p.Name, i+1))
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
			return messages.At(line, 0, messages.New(messages.H2414, name, required, nargs, strings.Join(missing, ", ")))
		}
		return messages.At(line, 0, messages.New(messages.H2415, name, strings.Join(missing, ", "), len(missing)))
	}
	return nil
}

// callLabel argüman hatasının öznesidir: kurucuda yapının kendisi ('Nokta' yapısı; kurucuyu yapının
// çağrısı çalıştırır), metotta Yapı.metot (Python: Point.go()), öbür fonksiyonda adı.
func callLabel(fn *FuncVal, owner *StructDefVal) *messages.Error {
	if owner == nil {
		if si, ok := fn.BoundSelf.(*StructInstanceVal); ok {
			owner = si.Def
		}
	}
	switch {
	case owner != nil && fn.Name == ctorName:
		return messages.New(messages.H2524, owner.Name)
	case owner != nil:
		return messages.New(messages.H2408, owner.Name+"."+fn.Name)
	case fn.Name == "":
		return messages.New(messages.H2409)
	}
	return messages.New(messages.H2408, fn.Name)
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
	alignNamed(named, nameList)
	for k := range named {
		if !slices.Contains(nameList, k) {
			return messages.At(line, 0, messages.New(messages.H2418, def.Name, k, hint.ClosestSuggestion(k, nameList, nil)))
		}
	}
	if nargs > len(def.Fields) {
		return messages.At(line, 0, messages.New(messages.H2419, def.Name, len(def.Fields), strings.Join(nameList, ", "), nargs))
	}
	for i, f := range nameList[:nargs] {
		if _, ok := named[f]; ok {
			return messages.At(line, 0, messages.New(messages.H2420, def.Name, f, i+1))
		}
	}
	return nil
}

// alignNamed kodlu adlı argümanı (TR_arg_lr) bildirimi metin olan parametreye ya da alana bağlar. Kullanıcının
// parametre adı metin kalır, çağrıdaki dilin sözcüğü kodlanır (öğrenme_oranı:, lr: → TR_arg_lr); kimlik
// hiçbir adla eşleşmiyorsa kodun herhangi bir yazım dilindeki adı olan parametre seçilir: hiçbir dile
// ayrıcalık yoktur, Türkçe yazılmış metot İngilizce çağrıyla da eşleşir.
func alignNamed(named map[string]Value, names []string) {
	for key, val := range named {
		if slices.Contains(names, key) || !codes.IsReservedName(key) {
			continue
		}
		k, err := codes.Parse(key)
		if err != nil {
			continue
		}
		for _, n := range codes.WritingNames(k) {
			if _, given := named[n]; slices.Contains(names, n) && !given {
				delete(named, key)
				named[n] = val
				break
			}
		}
	}
}

func suggestParam(name string, params []ast.Param) *messages.Error {
	var names []string
	for _, p := range params {
		names = append(names, p.Name)
	}
	return hint.ClosestSuggestion(name, names, nil)
}

func (v *VM) ApplyFunction(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
	res, err := v.apply(fnVal, args, named, line)
	if err == nil {
		// Sonuç Go koduna geçer; orada saklanabilir (ownership.go: kaçış).
		escape(res)
	}
	return res, err
}

// applyCompiled derlenmiş TR kodunun yaptığı çağrıdır: yerleşiğin taze sonucu çalışan
// çağrının olur (callBuiltin), kullanıcı fonksiyonunun sonucu çağırana geçer (callPrepared).
func (v *VM) applyCompiled(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
	if fn, ok := fnVal.(*BuiltinVal); ok {
		return v.callBuiltin(fn, args, named, line)
	}
	return v.apply(fnVal, args, named, line)
}

// callBuiltin derlenmiş koddan bir yerleşiği çağırır. Argümanlarını saklayabilecek (saf
// olmayan) yerleşiğin argümanları kaçar; saf yerleşiğin taze sonucu sahiplenilir ve
// argümanlarından ara sonuç olanlar hemen bırakılır (ownership.go).
func (v *VM) callBuiltin(fn *BuiltinVal, args []Value, named map[string]Value, line int) (Value, error) {
	if fn.NoNamedArgs && len(named) > 0 {
		return nil, lineSuffix(line, NoNamedArgsError(fn.Name, named))
	}
	if !fn.Pure {
		escapeAll(args, named)
	}
	newOp()
	res, err := fn.Fn(v, args, named)
	if err != nil {
		return nil, lineSuffix(line, err)
	}
	v.afterBuiltin(fn, res, args, named)
	return res, nil
}

// afterBuiltin yerleşik çağrısından sonra sonucu sahiplenir, saf yerleşiğin ara sonuç
// argümanlarını bırakır.
func (v *VM) afterBuiltin(fn *BuiltinVal, res Value, args []Value, named map[string]Value) {
	v.adopt(res)
	if !fn.Pure {
		return
	}
	for _, a := range args {
		v.releaseTemp(a, res)
	}
	for _, a := range named {
		v.releaseTemp(a, res)
	}
}

func (v *VM) apply(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
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
				inst.Fields[f.Name] = Null
			}
			escape(inst.Fields[f.Name]) // nesne alanına yazılan değer kaçar
		}
		if withCtor {
			// kurucu varsa argümanlar onun imzasına göre denetlenir
			if err := checkArity(initMethod, fn, len(args), named, line); err != nil {
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

	return nil, messages.At(line, 0, messages.New(messages.H2421, fnVal.Type()))
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
			if derr := v.runDefers(fnEnv); derr != nil {
				val, err = nil, addDeferError(err, derr)
			}
		}()
	}
	if len(v.CallStack) >= maxCallDepth {
		return nil, messages.At(line, 0, messages.New(messages.H2422, maxCallDepth))
	}
	fnName := fn.Name
	if fnName == "" {
		fnName = "<anonim>"
	}
	v.CallStack = append(v.CallStack, StackFrame{
		FuncName: fnName,
		Line:     line,
		Source:   v.source,
	})
	depth := len(v.CallStack)
	v.beginCall(depth)
	val, err = v.functionBody(fn, fk, fnEnv, self, args, nargs, named)
	fnEnv.closeEnv()
	if len(v.CallStack) > 0 {
		v.CallStack = v.CallStack[:len(v.CallStack)-1]
	}
	v.endCall(depth, val)
	if !fk.hasDefer {
		if derr := v.runDefers(fnEnv); derr != nil {
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
			fnEnv.store(fk.self, self)
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
				fnEnv.store(slot, args[i])
			}
		} else if named != nil && named[param.Name] != nil {
			fnEnv.store(slot, named[param.Name])
		} else if param.DefaultValue != nil && fk.defaultVal[i] != nil {
			defVal, err := fk.defaultVal[i](v, fnEnv)
			if err != nil {
				return nil, err
			}
			fnEnv.store(slot, defVal)
		} else {
			fnEnv.store(slot, Null)
		}
	}
	val, _, err := fk.bodyBlock.run(v, fnEnv)
	if err != nil {
		return nil, err
	}
	return val, nil
}
