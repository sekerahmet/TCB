package yurut

import (
	"fmt"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
)

// --- DEYİMLER ---

func (c *derleyici) deyim(k *derleyiciKapsam, stmt agac.Statement) deyimKodu {
	switch s := stmt.(type) {
	case nil:
		return bosDeyim
	case *agac.VarStatement:
		return c.tanimDeyimi(k, s)

	case *agac.MultiVarStatement:
		return c.cokluTanimDeyimi(k, s)

	case *agac.AssignStatement:
		return c.atamaDeyimi(k, s)

	case *agac.MultiAssignStatement:
		return c.cokluAtamaDeyimi(k, s)

	case *agac.DeferStatement:
		return c.erteleDeyimi(k, s)

	case *agac.ReturnStatement:
		return c.donDeyimi(k, s)

	case *agac.BreakStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return NULL, signalBreak, nil
		}

	case *agac.ContinueStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return NULL, signalContinue, nil
		}

	case *agac.BlockStatement:
		return c.blokDeyimi(k, s)

	case *agac.ForInStatement:
		return c.donguIcinde(k, s)

	case *agac.WhileStatement:
		return c.iken(k, s)

	case *agac.StructStatement:
		return c.yapiTanimi(k, s)

	case *agac.TryCatchStatement:
		return c.deneYakalaDeyimi(k, s)

	case *agac.ThrowStatement:
		return c.firlatDeyimi(k, s)

	case *agac.AsyncStatement:
		return c.eszamanli(k, s)

	case *agac.ImportStatement:
		return c.aktarDeyimi(k, s)

	case *agac.TestStatement:
		// Test blokları normal çalıştırmada atlanır; 'tr test' bunları RunTest ile çalıştırır.
		return bosDeyim

	case *agac.BreakpointStatement:
		return c.kesmeNoktasiDeyimi(k, s)

	case *agac.ExpressionStatement:
		return c.ifadeSinyal(k, s.Expression)
	}
	return bosDeyim
}

// tanimDeyimi 'değişken'/'sabit' tanımını derler.
func (c *derleyici) tanimDeyimi(k *derleyiciKapsam, s *agac.VarStatement) deyimKodu {
	deger := c.ifade(k, s.Value)
	tanim := tanimlayici(k, s.Name.Value)
	sabit := s.IsConst
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := deger(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		tanim(env, val, sabit)
		return val, signalNone, nil
	}
}

// cokluTanimDeyimi çoklu tanımı (değişken a, b = ...) derler.
func (c *derleyici) cokluTanimDeyimi(k *derleyiciKapsam, s *agac.MultiVarStatement) deyimKodu {
	degerler := c.ifadeler(k, s.Values)
	tanimlar := make([]func(*Environment, Value, bool), len(s.Names))
	for i, n := range s.Names {
		tanimlar[i] = tanimlayici(k, n.Value)
	}
	sabit := s.IsConst
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		evalVals, err := cokluDegerler(v, env, degerler, len(tanimlar))
		if err != nil {
			return nil, signalNone, err
		}
		var lastVal Value = NULL
		for i, tanim := range tanimlar {
			var val Value = NULL
			if i < len(evalVals) {
				val = evalVals[i]
			}
			tanim(env, val, sabit)
			lastVal = val
		}
		return lastVal, signalNone, nil
	}
}

// atamaDeyimi atamayı (bileşik atama dahil) derler.
func (c *derleyici) atamaDeyimi(k *derleyiciKapsam, s *agac.AssignStatement) deyimKodu {
	deger := c.ifade(k, s.Value)
	hedef := c.atamaHedefi(k, s.Target, islecKodu(s.Operator), s.Line())
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := deger(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if err := hedef(v, env, val); err != nil {
			return nil, signalNone, err
		}
		return val, signalNone, nil
	}
}

// cokluAtamaDeyimi çoklu atamayı (a, b = b, a) derler.
func (c *derleyici) cokluAtamaDeyimi(k *derleyiciKapsam, s *agac.MultiAssignStatement) deyimKodu {
	// Önce sağdaki tüm ifadeler hesaplanır (a, b = b, a takası için kritik!)
	degerler := c.ifadeler(k, s.Values)
	hedefler := make([]func(*VM, *Environment, Value) error, len(s.Targets))
	for i, t := range s.Targets {
		hedefler[i] = c.atamaHedefi(k, t, opAta, s.Line())
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		evalVals, err := cokluDegerler(v, env, degerler, len(hedefler))
		if err != nil {
			return nil, signalNone, err
		}
		var lastVal Value = NULL
		for i, hedef := range hedefler {
			var val Value = NULL
			if i < len(evalVals) {
				val = evalVals[i]
			}
			if err := hedef(v, env, val); err != nil {
				return nil, signalNone, err
			}
			lastVal = val
		}
		return lastVal, signalNone, nil
	}
}

// erteleDeyimi 'ertele' deyimini derler.
func (c *derleyici) erteleDeyimi(k *derleyiciKapsam, s *agac.DeferStatement) deyimKodu {
	var govde *blokKodu
	var govdeBilgi *kapsamBilgisi
	var cagri ifadeKodu
	if s.Body != nil {
		alt := altKapsam(k)
		alt.onTanimla(s.Body.Statements)
		govde = c.deyimler(alt, s.Body.Statements)
		govdeBilgi = alt.bilgi
	} else if s.Call != nil {
		cagri = c.ifade(k, s.Call)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		capturedEnv := env
		// Ertelenen işin hatası yutulmaz; fonksiyonun hatasına eklenir (9.4).
		env.AddDefer(func() error {
			if govde != nil {
				_, _, err := govde.calistir(v, yeniOrtam(capturedEnv, govdeBilgi))
				return err
			} else if cagri != nil {
				_, err := cagri(v, capturedEnv)
				return err
			}
			return nil
		})
		return NULL, signalNone, nil
	}
}

// donDeyimi 'dön' deyimini derler.
func (c *derleyici) donDeyimi(k *derleyiciKapsam, s *agac.ReturnStatement) deyimKodu {
	deger := c.ifade(k, s.ReturnValue)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := deger(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalReturn, nil
	}
}

// deneYakalaDeyimi 'dene'/'yakala' deyimini derler.
func (c *derleyici) deneYakalaDeyimi(k *derleyiciKapsam, s *agac.TryCatchStatement) deyimKodu {
	dene := c.blokDeyimi(k, s.TryBody)
	alt := altKapsam(k)
	hataYuva := alt.bilgi.yuva(s.ErrorVar)
	if s.CatchBody != nil {
		alt.golgeKaydet(s.ErrorVar, s.CatchBody.Line(), "yakalanan hataya")
	}
	var yakalaDeyimleri []agac.Statement
	if s.CatchBody != nil {
		yakalaDeyimleri = s.CatchBody.Statements
	}
	alt.onTanimla(yakalaDeyimleri)
	yakala := c.deyimler(alt, yakalaDeyimleri)
	bilgi := alt.bilgi
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, sig, err := dene(v, env)
		if err != nil {
			if yakalanamaz(err) {
				return nil, signalNone, err // kesilme (Ctrl+C) ve program çıkışı isteği yakalanmaz
			}
			catchEnv := yeniOrtam(env, bilgi)
			catchEnv.tanimla(hataYuva, &StringVal{Val: err.Error()}, false)
			return yakala.calistir(v, catchEnv)
		}
		return val, sig, nil
	}
}

// firlatDeyimi 'fırlat' deyimini derler.
func (c *derleyici) firlatDeyimi(k *derleyiciKapsam, s *agac.ThrowStatement) deyimKodu {
	deger := c.ifade(k, s.Value)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := deger(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return nil, signalNone, fmt.Errorf("%s", val.Inspect())
	}
}

// aktarDeyimi 'aktar' deyimini derler.
func (c *derleyici) aktarDeyimi(k *derleyiciKapsam, s *agac.ImportStatement) deyimKodu {
	var takma func(*Environment, Value, bool)
	if s.Alias != "" {
		takma = tanimlayici(k, s.Alias)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return v.aktar(s, env, takma)
	}
}

// kesmeNoktasiDeyimi 'kesme_noktası' deyimini derler.
func (c *derleyici) kesmeNoktasiDeyimi(k *derleyiciKapsam, s *agac.BreakpointStatement) deyimKodu {
	var kosul ifadeKodu
	if s.Condition != nil {
		kosul = c.ifade(k, s.Condition)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		if kosul != nil {
			condVal, err := kosul(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !condVal.IsTruthy() {
				return NULL, signalNone, nil
			}
		}
		if v.Debugger == nil && YeniHataAyiklayici != nil {
			k := v.kaynak
			if k == nil {
				k = &Kaynak{Yol: v.SourceFile, Kod: v.SourceCode}
			}
			v.Debugger = YeniHataAyiklayici(k.Yol, k.Kod)
		}
		if v.Debugger != nil {
			if s.Condition != nil {
				v.Debugger.KesmeNoktasi(fmt.Sprintf("Koşullu kesme noktası (Satır %d)", s.Line()))
			} else {
				v.Debugger.KesmeNoktasi("Koddaki kesme noktası (" + kod.Ad(kod.A090) + ")")
			}
		}
		return NULL, signalNone, nil
	}
}

// cokluDegerler çoklu tanım/atamanın sağ tarafını hesaplar; tek bir Liste birden çok ada
// açılır.
func cokluDegerler(v *VM, env *Environment, degerler []ifadeKodu, adSayisi int) ([]Value, error) {
	evalVals := make([]Value, 0, len(degerler))
	for _, d := range degerler {
		val, err := d(v, env)
		if err != nil {
			return nil, err
		}
		evalVals = append(evalVals, val)
	}
	if len(evalVals) == 1 && adSayisi > 1 {
		if lv, ok := evalVals[0].(*ListVal); ok {
			evalVals = lv.Elements
		}
	}
	return evalVals, nil
}

// yapiTanimi 'yapı' deyimini derler: metotlar ve alan varsayılanları bir kez derlenir.
func (c *derleyici) yapiTanimi(k *derleyiciKapsam, s *agac.StructStatement) deyimKodu {
	type metotKodu struct {
		ad  string
		lit *agac.FunctionLiteral
		kod *fonkKodu
	}
	metotlar := make([]metotKodu, 0, len(s.Methods))
	for name, fnLit := range s.Methods {
		metotlar = append(metotlar, metotKodu{ad: name, lit: fnLit, kod: c.fonksiyon(k, fnLit.Parameters, fnLit.Body, true)})
	}
	// Alan varsayılanları yapının tanımlandığı kapsamda derlenir ve o ortamda
	// değerlendirilir (modüldeki bir yapı modülün adlarını görür).
	alanKod := c.alanVarsayilanlari(k, s.Fields)
	tanim := tanimlayici(k, s.Name)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		methods := make(map[string]*FuncVal, len(metotlar))
		for _, m := range metotlar {
			methods[m.ad] = &FuncVal{
				Name:       m.ad,
				Parameters: m.lit.Parameters,
				Body:       m.lit.Body,
				Env:        env,
				kod:        m.kod,
			}
		}
		def := &StructDefVal{
			Name:      s.Name,
			Fields:    s.Fields,
			Methods:   methods,
			alanKod:   alanKod,
			alanOrtam: env,
		}
		tanim(env, def, true)
		return def, signalNone, nil
	}
}

// alanVarsayilanlari alan varsayılanlarını k kapsamında derler (nesne oluşturulurken
// yapının tanımlandığı ortamda değerlendirilirler).
func (c *derleyici) alanVarsayilanlari(k *derleyiciKapsam, fields []agac.StructField) []ifadeKodu {
	kodlar := make([]ifadeKodu, len(fields))
	for i, f := range fields {
		if f.DefaultValue != nil {
			kodlar[i] = c.ifade(k, f.DefaultValue)
		}
	}
	return kodlar
}

// eszamanli 'eşzamanlı' bloğunu derler: gövde, kendi kök ortamında ayrı bir görevde çalışır.
func (c *derleyici) eszamanli(k *derleyiciKapsam, s *agac.AsyncStatement) deyimKodu {
	alt := altKapsam(k)
	var govde *blokKodu
	var cagri ifadeKodu
	if s.Body != nil {
		alt.onTanimla(s.Body.Statements)
		govde = c.deyimler(alt, s.Body.Statements)
	} else if s.Call != nil {
		ifadeAdlari(s.Call, alt.tanimla)
		cagri = c.ifade(alt, s.Call)
	}
	bilgi := alt.bilgi
	line := s.Line()
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		v.wg.Add(1)
		asyncEnv := yeniOrtam(env, bilgi)
		asyncEnv.IsFuncRoot = true // görev içindeki 'ertele' blokları görev bitince çalışır
		task := v.NewTaskVM()
		go func(e *Environment) {
			defer v.wg.Done()
			task.acquireGIL()
			defer task.releaseGIL()
			defer func() {
				if r := recover(); r != nil && !KesilmeMi(r) {
					task.reportAsyncError(line, fmt.Errorf("İç hata (çökme önlendi): %v", r))
				}
			}()
			var err error
			defer func() {
				// Görevin ertelenen işleri görev bitince çalışır; hataları görev hatasına eklenir.
				if err = erteleHatasiEkle(err, e.RunDefers()); err != nil {
					task.reportAsyncError(line, err)
				}
			}()
			if govde != nil {
				_, _, err = govde.calistir(task, e)
			} else if cagri != nil {
				_, err = cagri(task, e)
			}
		}(asyncEnv)
		return TRUE, signalNone, nil
	}
}
