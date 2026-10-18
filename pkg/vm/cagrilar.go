package vm

import (
	"fmt"
	"slices"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/kod"
	"tr_dili/pkg/turkce"
)

// maxCallDepth iç içe kullanıcı fonksiyonu çağrılarının üst sınırıdır. Sınırsız özyineleme
// Go yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda 'dene/yakala' ile
// yakalanabilen bir hata döner.
const maxCallDepth = 10000

// kendiAdi yapı metotlarında nesneyi gösteren 'kendi' anahtar kelimesinin sözlükteki
// adıdır (A051).
var kendiAdi = kod.Ad(kod.A051)

// kurucuAdi nesne oluşturulurken çağrılan metodun adıdır (7.4; sözlükte A102).
var kurucuAdi = kod.Ad(kod.A102)

// checkArity koddaki doğrudan çağrılarda (f(1, 2), x |> f) kullanıcı fonksiyonunun argüman
// sayısını ve adlandırılmış argümanlarını denetler. Yerleşiklerin geri çağırdığı fonksiyonlar
// (filtrele, web işleyicileri...) bu denetime girmez; onlar fazladan argüman gönderebilir.
func checkArity(fnVal Value, nargs int, named map[string]Value, line int) error {
	fn, ok := fnVal.(*FuncVal)
	if !ok {
		return nil
	}
	// Hızlı yol: adlı argüman yoksa ve verilmeyen her parametrenin varsayılanı varsa sorun
	// yoktur. Hata mesajı için gereken metin ve haritalar yalnızca hata yolunda kurulur.
	if len(named) == 0 && nargs <= len(fn.Parameters) && varsayilanlarYeter(fn, nargs) {
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

// yapiArgumanDenetle kurucusu olmayan bir yapının oluşturulmasındaki argümanları alanlara
// göre denetler: alan sayısından fazla konumlu argüman ya da alan olmayan bir adla verilen
// argüman hatadır.
func yapiArgumanDenetle(def *StructDefVal, nargs int, named map[string]Value, line int) error {
	if nargs <= len(def.Fields) && len(named) == 0 {
		return nil
	}
	adlar := make([]string, len(def.Fields))
	for i, f := range def.Fields {
		adlar[i] = f.Name
	}
	for k := range named {
		if !slices.Contains(adlar, k) {
			oneri := ""
			if o := turkce.EnYakin(k, adlar); o != "" {
				oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return fmt.Errorf("Satır %d: '%s' yapısının '%s' adında bir alanı yok%s", line, def.Name, k, oneri)
		}
	}
	if nargs > len(def.Fields) {
		return fmt.Errorf("Satır %d: '%s' yapısı en fazla %d argüman alır (alanlar: %s), %d verildi", line, def.Name, len(def.Fields), strings.Join(adlar, ", "), nargs)
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
		fk := fn.kod
		if fk == nil {
			fk = v.fonkKoduHazirla(fn)
		}
		return v.cagirHazir(fn, fk, fk.ortamAl(fn.Env), fn.BoundSelf, args, len(args), named, line)

	case *BuiltinVal:
		res, err := fn.Fn(v, args, named)
		if err != nil {
			return nil, satirEki(line, err)
		}
		return res, nil

	case *StructDefVal:
		inst := &StructInstanceVal{
			Def:    fn,
			Fields: make(map[string]Value),
		}
		if fn.alanKod == nil {
			// Derleyici dışında kurulmuş tanım: varsayılanlar genel kapsamda değerlendirilir
			c := v.yeniDerleyici()
			fn.alanKod = c.alanVarsayilanlari(c.kok, fn.Fields)
			fn.alanOrtam = v.GlobalEnv
		}
		alanOrtam := fn.alanOrtam
		if alanOrtam == nil {
			alanOrtam = v.GlobalEnv
		}
		initMethod, kuruculu := fn.Methods[kurucuAdi]
		if !kuruculu {
			// kurucu yoksa argümanlar alanlara gider: fazlası ya da bilinmeyen adlısı hatadır
			if err := yapiArgumanDenetle(fn, len(args), named, line); err != nil {
				return nil, err
			}
		}
		for i, f := range fn.Fields {
			if i < len(args) {
				inst.Fields[f.Name] = args[i]
			} else if named != nil && named[f.Name] != nil {
				inst.Fields[f.Name] = named[f.Name]
			} else if f.DefaultValue != nil && i < len(fn.alanKod) && fn.alanKod[i] != nil {
				defVal, err := fn.alanKod[i](v, alanOrtam)
				if err != nil {
					return nil, err
				}
				inst.Fields[f.Name] = defVal
			} else {
				inst.Fields[f.Name] = NULL
			}
		}
		if kuruculu {
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
				kod:        initMethod.kod,
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

// cagirHazir bir kullanıcı fonksiyonunu fnEnv ortamında çalıştırır. args nil değilse
// argümanlar sırayla bağlanır; nil ise ilk nargs argüman zaten yuvalara yazılmıştır
// (yalnızca fk.basit iken). Kalan parametreler adlı argümanlardan, varsayılanlardan ya da
// 'yok' ile doldurulur. kendi, metot çağrılarında nesnedir (bağlı fonksiyon değerinin
// BoundSelf'i ya da doğrudan derlenmiş 'nesne.metot(...)' çağrısının nesnesi).
func (v *VM) cagirHazir(fn *FuncVal, fk *fonkKodu, fnEnv *Environment, kendi Value, args []Value, nargs int, named map[string]Value, line int) (val Value, err error) {
	if fk.kaynak != v.kaynak {
		// Başka bir dosyada (modülde) tanımlı fonksiyon: o dosyanın bağlamında çalışır.
		return v.kaynaktaCagir(fn, fk, fnEnv, kendi, args, nargs, named, line)
	}
	fnEnv.IsFuncRoot = true
	v.devret() // özyinelemeli (döngüsüz) hesaplarda da kilit devredilebilsin
	if fk.erteleVar {
		// 'ertele' blokları bir çöküşte de (Go panic) çalışmalıdır; hataları yutulmaz (9.4)
		defer func() {
			if derr := fnEnv.RunDefers(); derr != nil {
				val, err = nil, erteleHatasiEkle(err, derr)
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
	val, err = v.fonksiyonGovdesi(fn, fk, fnEnv, kendi, args, nargs, named)
	if len(v.CallStack) > 0 {
		v.CallStack = v.CallStack[:len(v.CallStack)-1]
	}
	if !fk.erteleVar {
		if derr := fnEnv.RunDefers(); derr != nil {
			val, err = nil, erteleHatasiEkle(err, derr)
		}
		// Hata ayıklayıcı bağlıyken ortam onun elinde kalabilir; havuza konmaz.
		if fk.havuzlu && v.Debugger == nil {
			fk.birak(fnEnv)
		}
	}
	return val, err
}

func (v *VM) fonksiyonGovdesi(fn *FuncVal, fk *fonkKodu, fnEnv *Environment, kendi Value, args []Value, nargs int, named map[string]Value) (Value, error) {
	if kendi != nil {
		if fk.kendi >= 0 {
			fnEnv.vals[fk.kendi] = kendi
		} else {
			fnEnv.Define(kendiAdi, kendi, false)
		}
	}
	for i, param := range fn.Parameters {
		if i >= len(fk.param) {
			break
		}
		yuva := fk.param[i]
		if i < nargs {
			if args != nil {
				fnEnv.vals[yuva] = args[i]
			}
		} else if named != nil && named[param.Name] != nil {
			fnEnv.vals[yuva] = named[param.Name]
		} else if param.DefaultValue != nil && fk.varsayilan[i] != nil {
			defVal, err := fk.varsayilan[i](v, fnEnv)
			if err != nil {
				return nil, err
			}
			fnEnv.vals[yuva] = defVal
		} else {
			fnEnv.vals[yuva] = NULL
		}
	}
	val, _, err := fk.govde.calistir(v, fnEnv)
	if err != nil {
		return nil, err
	}
	return val, nil
}
