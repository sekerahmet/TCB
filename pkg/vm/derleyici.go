package vm

import (
	"fmt"
	"tr_dili/pkg/ast"
)

// --- KAPANIŞ DERLEYİCİSİ ---
//
// AST, çalıştırılmadan önce bir kez Go kapanışlarına (closure) çevrilir; böylece ağaç her
// turda yeniden dolaşılmaz, işleç metinleri ve sabitler derleme anında çözülür. Değişkenler
// mümkün olduğunda derleme anında (derinlik, yuva) çiftine çözülür ve çalışma anında ad
// araması yapılmaz.
//
// Derleme anındaki her kapsam (derleyiciKapsam) çalışma anındaki tam olarak bir ortama
// karşılık gelir: fonksiyon çağrısı, ad tanımlayan blok, döngü turu, yakala bloğu,
// eşzamanlı/ertele gövdesi, liste üreteci öğesi, modül ve genel kapsam. Ad tanımlamayan
// bloklar ortam açmaz ve kapsam da oluşturmaz. Bu birebirlik sayesinde derinlik çalışma
// anında da geçerlidir.
//
// Bir yuva nil ise (ad henüz tanımlanmamış: tanım ilerideyse ya da hata ayıklayıcı
// 'zıpla' ile atlandıysa) arama adla, ağaç yorumlayıcısındaki gibi sürdürülür; bu yüzden
// anlam değişmez.

type ifadeKodu func(v *VM, env *Environment) (Value, error)
type deyimKodu func(v *VM, env *Environment) (Value, flowSignal, error)

// derleyiciKapsam derleme anındaki bir kapsamdır.
type derleyiciKapsam struct {
	// bilgi kapsamın ad tablosudur. nil ise kapsam, düzeni derleme anında bilinmeyen ve
	// çalışma anında verilen bir ortamdır (hata ayıklayıcının değerlendirdiği ifadeler):
	// buradan geçen adlar adla aranır, buradaki tanımlar adla yapılır.
	bilgi *kapsamBilgisi
	ust   *derleyiciKapsam
	// genel, bu kapsam genel kapsamsa onun ortamıdır; genel adlara derinlik yürümeden
	// doğrudan erişilir.
	genel *Environment
	// dinamik: kapsamda takma adsız bir 'aktar' var; adları çalışma anında eklenir ve dış
	// kapsamlardaki aynı adları gölgeleyebilir. Bu kapsamdan geçen adlar adla aranır.
	dinamik bool
}

// tanimla bir adı kapsamın tablosuna ekler (derleme anı).
func (k *derleyiciKapsam) tanimla(ad string) {
	if ad == "" {
		k.dinamik = true
		return
	}
	if k.bilgi != nil {
		k.bilgi.yuva(ad)
	}
}

// onTanimla deyimlerin bu kapsama tanımlayacağı tüm adlara, deyimler derlenmeden önce yuva
// ayırır. Böylece tanımdan önce gelen bir kullanım da (ör. hata ayıklayıcı geri atladığında)
// doğru yuvaya çözülür.
func (k *derleyiciKapsam) onTanimla(stmts []ast.Statement) {
	for _, st := range stmts {
		deyimAdlari(st, k.tanimla)
	}
}

const (
	cozDinamik = iota
	cozYerel
	cozGenel
)

type cozum struct {
	tur      int
	derinlik int
	yuva     int
	genel    *Environment
}

// coz bir adı derleme anında çözer.
func (k *derleyiciKapsam) coz(ad string) cozum {
	d := 0
	for c := k; c != nil; c = c.ust {
		if c.bilgi == nil {
			return cozum{}
		}
		if i, ok := c.bilgi.indeks[ad]; ok {
			if c.genel != nil {
				return cozum{tur: cozGenel, yuva: i, genel: c.genel}
			}
			return cozum{tur: cozYerel, derinlik: d, yuva: i}
		}
		if c.dinamik {
			return cozum{}
		}
		d++
	}
	return cozum{}
}

// derleyici bir derleme işinin durumudur.
type derleyici struct {
	// kok genel kapsamdır (yapı alanlarının varsayılanları burada değerlendirilir).
	kok *derleyiciKapsam
}

func (v *VM) yeniDerleyici() *derleyici {
	return &derleyici{kok: &derleyiciKapsam{bilgi: v.GlobalEnv.kapsam, genel: v.GlobalEnv}}
}

// altKapsam ust içinde yeni bir ortama karşılık gelen kapsam açar.
func altKapsam(ust *derleyiciKapsam) *derleyiciKapsam {
	return &derleyiciKapsam{bilgi: &kapsamBilgisi{}, ust: ust}
}

// sinirKapsami çalışma anında verilecek, düzeni bilinmeyen bir ortamı temsil eder.
func sinirKapsami() *derleyiciKapsam {
	return &derleyiciKapsam{}
}

// tanimlayici k kapsamına ad tanımlayan bir işlev döndürür (yuvalı ya da adla).
func tanimlayici(k *derleyiciKapsam, ad string) func(env *Environment, val Value, sabit bool) {
	if k.bilgi == nil {
		return func(env *Environment, val Value, sabit bool) { env.Define(ad, val, sabit) }
	}
	i := k.bilgi.yuva(ad)
	return func(env *Environment, val Value, sabit bool) { env.tanimla(i, val, sabit) }
}

// --- BLOKLAR ---

// blokKodu derlenmiş bir deyim listesidir. Deyimlerin kendisi hata ayıklayıcı (her
// deyimden önce danışılır) ve 'zıpla' (satıra atlama) için saklanır.
type blokKodu struct {
	deyimler []ast.Statement
	kodlar   []deyimKodu
}

func (c *derleyici) deyimler(k *derleyiciKapsam, stmts []ast.Statement) *blokKodu {
	b := &blokKodu{deyimler: stmts, kodlar: make([]deyimKodu, len(stmts))}
	for i, st := range stmts {
		b.kodlar[i] = c.deyim(k, st)
	}
	return b
}

func (b *blokKodu) calistir(v *VM, env *Environment) (Value, flowSignal, error) {
	var lastVal Value = NULL
	kodlar := b.kodlar
	for i := 0; i < len(kodlar); i++ {
		if v.Debugger != nil {
			if stmt := b.deyimler[i]; stmt != nil {
				// kesme_noktası satırının kendisinde değil, ya da kesme_noktası tetiklendikten sonra dur
				if _, isBp := stmt.(*ast.BreakpointStatement); !isBp {
					jumpLine := v.Debugger.BeforeStatement(v, stmt, env)
					if jumpLine > 0 {
						jumped := false
						for j, targetStmt := range b.deyimler {
							if targetStmt != nil && targetStmt.Line() >= jumpLine {
								i = j - 1 // döngü sonunda i++ olacağı için j-1
								jumped = true
								break
							}
						}
						if jumped {
							continue
						}
					}
				}
			}
		}
		val, sig, err := kodlar[i](v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if sig != signalNone {
			return val, sig, nil
		}
		lastVal = val
	}
	return lastVal, signalNone, nil
}

// blokDeyimi bir bloğu derler: blok ad tanımlıyorsa her çalışmada yeni bir ortam açar,
// tanımlamıyorsa çevredeki ortamda çalışır.
func (c *derleyici) blokDeyimi(k *derleyiciKapsam, b *ast.BlockStatement) deyimKodu {
	if b == nil {
		return bosDeyim
	}
	if blokBayraklari(b)&blokTanimlar == 0 {
		return c.deyimler(k, b.Statements).calistir
	}
	alt := altKapsam(k)
	alt.onTanimla(b.Statements)
	bk := c.deyimler(alt, b.Statements)
	bilgi := alt.bilgi
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return bk.calistir(v, yeniOrtam(env, bilgi))
	}
}

func bosDeyim(v *VM, env *Environment) (Value, flowSignal, error) {
	return NULL, signalNone, nil
}

// donguOrtami bir döngünün her turunda gövdenin çalışacağı ortamı sağlar. Gövde ortamı
// yakalayamıyorsa (içinde fonksiyon, ertele, eşzamanlı, yapı yoksa) tek bir ortam turlar
// arasında temizlenip yeniden kullanılır; aksi hâlde her tur yeni ortam açılır.
type donguOrtami struct {
	dis     *Environment
	bilgi   *kapsamBilgisi
	env     *Environment
	yeniden bool
}

func (d *donguOrtami) yeni() *Environment {
	if d.env != nil && d.yeniden {
		d.env.temizle()
		return d.env
	}
	d.env = yeniOrtam(d.dis, d.bilgi)
	return d.env
}

// --- DEYİMLER ---

func (c *derleyici) deyim(k *derleyiciKapsam, stmt ast.Statement) deyimKodu {
	switch s := stmt.(type) {
	case nil:
		return bosDeyim
	case *ast.VarStatement:
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

	case *ast.MultiVarStatement:
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

	case *ast.AssignStatement:
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

	case *ast.MultiAssignStatement:
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

	case *ast.DeferStatement:
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
			env.AddDefer(func() {
				if govde != nil {
					_, _, _ = govde.calistir(v, yeniOrtam(capturedEnv, govdeBilgi))
				} else if cagri != nil {
					_, _ = cagri(v, capturedEnv)
				}
			})
			return NULL, signalNone, nil
		}

	case *ast.ReturnStatement:
		deger := c.ifade(k, s.ReturnValue)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			val, err := deger(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			return val, signalReturn, nil
		}

	case *ast.BreakStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return NULL, signalBreak, nil
		}

	case *ast.ContinueStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return NULL, signalContinue, nil
		}

	case *ast.BlockStatement:
		return c.blokDeyimi(k, s)

	case *ast.ForInStatement:
		return c.donguIcinde(k, s)

	case *ast.WhileStatement:
		return c.iken(k, s)

	case *ast.StructStatement:
		return c.yapiTanimi(k, s)

	case *ast.TryCatchStatement:
		dene := c.blokDeyimi(k, s.TryBody)
		alt := altKapsam(k)
		hataYuva := alt.bilgi.yuva(s.ErrorVar)
		var yakalaDeyimleri []ast.Statement
		if s.CatchBody != nil {
			yakalaDeyimleri = s.CatchBody.Statements
		}
		alt.onTanimla(yakalaDeyimleri)
		yakala := c.deyimler(alt, yakalaDeyimleri)
		bilgi := alt.bilgi
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			val, sig, err := dene(v, env)
			if err != nil {
				catchEnv := yeniOrtam(env, bilgi)
				catchEnv.tanimla(hataYuva, &StringVal{Val: err.Error()}, false)
				return yakala.calistir(v, catchEnv)
			}
			return val, sig, nil
		}

	case *ast.ThrowStatement:
		deger := c.ifade(k, s.Value)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			val, err := deger(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			return nil, signalNone, fmt.Errorf("%s", val.Inspect())
		}

	case *ast.AsyncStatement:
		return c.eszamanli(k, s)

	case *ast.ImportStatement:
		var takma func(*Environment, Value, bool)
		if s.Alias != "" {
			takma = tanimlayici(k, s.Alias)
		}
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return v.aktar(s, env, takma)
		}

	case *ast.TestStatement:
		// Test blokları normal çalıştırmada atlanır; 'tr test' bunları RunTest ile çalıştırır.
		return bosDeyim

	case *ast.BreakpointStatement:
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
				v.Debugger = YeniHataAyiklayici(v.SourceFile, v.SourceCode)
			}
			if v.Debugger != nil {
				if s.Condition != nil {
					v.Debugger.KesmeNoktasi(fmt.Sprintf("Koşullu kesme noktası (Satır %d)", s.Line()))
				} else {
					v.Debugger.KesmeNoktasi("Koddaki kesme noktası (kesme_noktası)")
				}
			}
			return NULL, signalNone, nil
		}

	case *ast.ExpressionStatement:
		return c.ifadeSinyal(k, s.Expression)
	}
	return bosDeyim
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

// donguIcinde 'döngü x içinde ...' deyimini derler. Döngü değişkenleri ve gövdenin
// tanımları her turun kendi ortamındadır.
func (c *derleyici) donguIcinde(k *derleyiciKapsam, s *ast.ForInStatement) deyimKodu {
	kaynak := c.ifade(k, s.Iterable)
	alt := altKapsam(k)
	anahtarYuva := -1
	if s.KeyVar != "" {
		anahtarYuva = alt.bilgi.yuva(s.KeyVar)
	}
	degerYuva := alt.bilgi.yuva(s.ValueVar)
	var govdeDeyimleri []ast.Statement
	yeniden := true
	if s.Body != nil {
		govdeDeyimleri = s.Body.Statements
		yeniden = blokBayraklari(s.Body)&blokYakalar == 0
	}
	alt.onTanimla(govdeDeyimleri)
	govde := c.deyimler(alt, govdeDeyimleri)
	bilgi := alt.bilgi
	line := s.Line()

	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		iterVal, err := kaynak(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		tur := donguOrtami{dis: env, bilgi: bilgi, yeniden: yeniden}
		var lastVal Value = NULL
		switch it := iterVal.(type) {
		case *RangeVal:
			end := it.End
			if it.Inclusive {
				end++
			}
			idx := int64(0)
			step := int64(1)
			if it.Start > it.End {
				step = -1
				if it.Inclusive {
					end = it.End - 1
				} else {
					end = it.End
				}
			}
			for i := it.Start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
				loopEnv := tur.yeni()
				if anahtarYuva >= 0 {
					loopEnv.vals[anahtarYuva] = NewInt(idx)
				}
				loopEnv.vals[degerYuva] = NewInt(i)
				val, sig, err := govde.calistir(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
				idx++
			}

		case *ListVal:
			for idx, item := range it.Elements {
				loopEnv := tur.yeni()
				if anahtarYuva >= 0 {
					loopEnv.vals[anahtarYuva] = NewInt(int64(idx))
				}
				loopEnv.vals[degerYuva] = item
				val, sig, err := govde.calistir(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case *DictVal:
			for _, key := range it.Order {
				loopEnv := tur.yeni()
				if anahtarYuva >= 0 {
					loopEnv.vals[anahtarYuva] = it.KeyValue(key)
					loopEnv.vals[degerYuva] = it.Pairs[key]
				} else {
					loopEnv.vals[degerYuva] = it.KeyValue(key)
				}
				val, sig, err := govde.calistir(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		case *StringVal:
			for idx, ch := range []rune(it.Val) {
				loopEnv := tur.yeni()
				if anahtarYuva >= 0 {
					loopEnv.vals[anahtarYuva] = NewInt(int64(idx))
				}
				loopEnv.vals[degerYuva] = &StringVal{Val: string(ch)}
				val, sig, err := govde.calistir(v, loopEnv)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}

		default:
			return nil, signalNone, fmt.Errorf("Satır %d: %s tipi üzerinde döngü kurulamaz", line, iterVal.Type())
		}
		return lastVal, signalNone, nil
	}
}

// iken 'iken koşul { ... }' deyimini derler. Gövde ad tanımlamıyorsa tur ortamı açılmaz.
func (c *derleyici) iken(k *derleyiciKapsam, s *ast.WhileStatement) deyimKodu {
	kosul := c.ifade(k, s.Condition)
	var govdeDeyimleri []ast.Statement
	if s.Body != nil {
		govdeDeyimleri = s.Body.Statements
	}
	if s.Body == nil || blokBayraklari(s.Body)&blokTanimlar == 0 {
		govde := c.deyimler(k, govdeDeyimleri)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			var lastVal Value = NULL
			for {
				cond, err := kosul(v, env)
				if err != nil {
					return nil, signalNone, err
				}
				if !dogruMu(cond) {
					break
				}
				val, sig, err := govde.calistir(v, env)
				if err != nil {
					return nil, signalNone, err
				}
				if sig == signalReturn {
					return val, signalReturn, nil
				}
				if sig == signalBreak {
					break
				}
				lastVal = val
			}
			return lastVal, signalNone, nil
		}
	}
	alt := altKapsam(k)
	alt.onTanimla(govdeDeyimleri)
	govde := c.deyimler(alt, govdeDeyimleri)
	bilgi := alt.bilgi
	yeniden := blokBayraklari(s.Body)&blokYakalar == 0
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		var lastVal Value = NULL
		tur := donguOrtami{dis: env, bilgi: bilgi, yeniden: yeniden}
		for {
			cond, err := kosul(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !dogruMu(cond) {
				break
			}
			val, sig, err := govde.calistir(v, tur.yeni())
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
		}
		return lastVal, signalNone, nil
	}
}

// dogruMu bir değerin doğruluğunu (Mantık değerleri için arayüz çağrısı yapmadan) verir.
func dogruMu(x Value) bool {
	if b, ok := x.(*BoolVal); ok {
		return b.Val
	}
	return x.IsTruthy()
}

// yapiTanimi 'yapı' deyimini derler: metotlar ve alan varsayılanları bir kez derlenir.
func (c *derleyici) yapiTanimi(k *derleyiciKapsam, s *ast.StructStatement) deyimKodu {
	type metotKodu struct {
		ad  string
		lit *ast.FunctionLiteral
		kod *fonkKodu
	}
	metotlar := make([]metotKodu, 0, len(s.Methods))
	for name, fnLit := range s.Methods {
		metotlar = append(metotlar, metotKodu{ad: name, lit: fnLit, kod: c.fonksiyon(k, fnLit.Parameters, fnLit.Body, true)})
	}
	alanKod := c.alanVarsayilanlari(s.Fields)
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
			Name:    s.Name,
			Fields:  s.Fields,
			Methods: methods,
			alanKod: alanKod,
		}
		tanim(env, def, true)
		return def, signalNone, nil
	}
}

// alanVarsayilanlari alan varsayılanlarını genel kapsamda derler (nesne oluşturulurken
// genel ortamda değerlendirilirler).
func (c *derleyici) alanVarsayilanlari(fields []ast.StructField) []ifadeKodu {
	kodlar := make([]ifadeKodu, len(fields))
	for i, f := range fields {
		if f.DefaultValue != nil {
			kodlar[i] = c.ifade(c.kok, f.DefaultValue)
		}
	}
	return kodlar
}

// eszamanli 'eşzamanlı' bloğunu derler: gövde, kendi kök ortamında ayrı bir görevde çalışır.
func (c *derleyici) eszamanli(k *derleyiciKapsam, s *ast.AsyncStatement) deyimKodu {
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
		task := v.NewTaskVM(true)
		go func(e *Environment) {
			defer v.wg.Done()
			task.acquireGIL()
			defer task.releaseGIL()
			defer func() {
				if r := recover(); r != nil {
					task.reportAsyncError(line, fmt.Errorf("İç hata (çökme önlendi): %v", r))
				}
			}()
			defer e.RunDefers()
			var err error
			if govde != nil {
				_, _, err = govde.calistir(task, e)
			} else if cagri != nil {
				_, err = cagri(task, e)
			}
			if err != nil {
				task.reportAsyncError(line, err)
			}
		}(asyncEnv)
		return TRUE, signalNone, nil
	}
}

// --- FONKSİYONLAR ---

// fonkKodu bir fonksiyonun derlenmiş hâlidir; o fonksiyondan üretilen tüm değerler
// (her kapanış) paylaşır.
type fonkKodu struct {
	bilgi *kapsamBilgisi
	govde *blokKodu
	// param[i] i. parametrenin yuvası; varsayilan[i] varsayılan değer kodu (yoksa nil).
	// Varsayılanlar fonksiyonun tanımlandığı ortamda değerlendirilir.
	param      []int
	varsayilan []ifadeKodu
	kendi      int // metotlarda 'kendi'nin yuvası, değilse -1
	// basit: parametre yuvaları birbirinden ve 'kendi'den farklı; argümanlar doğrudan
	// yuvalara yazılabilir (bağlama sırası sonucu değiştirmez).
	basit     bool
	erteleVar bool
}

// fonksiyon bir fonksiyon gövdesini, tanımlandığı k kapsamının içinde derler.
func (c *derleyici) fonksiyon(k *derleyiciKapsam, params []ast.Param, body *ast.BlockStatement, metot bool) *fonkKodu {
	alt := altKapsam(k)
	fk := &fonkKodu{
		param:      make([]int, len(params)),
		varsayilan: make([]ifadeKodu, len(params)),
		kendi:      -1,
		basit:      true,
	}
	if metot {
		fk.kendi = alt.bilgi.yuva("kendi")
	}
	gorulen := map[int]bool{}
	if fk.kendi >= 0 {
		gorulen[fk.kendi] = true
	}
	for i, p := range params {
		fk.param[i] = alt.bilgi.yuva(p.Name)
		if gorulen[fk.param[i]] {
			fk.basit = false
		}
		gorulen[fk.param[i]] = true
		if p.DefaultValue != nil {
			fk.varsayilan[i] = c.ifade(k, p.DefaultValue)
		}
	}
	var stmts []ast.Statement
	if body != nil {
		stmts = body.Statements
	}
	alt.onTanimla(stmts)
	fk.govde = c.deyimler(alt, stmts)
	fk.bilgi = alt.bilgi
	fk.erteleVar = govdedeErteleVar(stmts)
	return fk
}

// fonkKoduHazirla derleyici dışında oluşturulmuş bir fonksiyon değerini (kod alanı boş)
// adla çözümlemeyle derler.
func (v *VM) fonkKoduHazirla(fn *FuncVal) *fonkKodu {
	fk := v.yeniDerleyici().fonksiyon(sinirKapsami(), fn.Parameters, fn.Body, fn.BoundSelf != nil)
	fn.kod = fk
	return fk
}

// --- GİRİŞ NOKTALARI ---

// programCalistir deyimleri genel kapsamda derleyip genel ortamda çalıştırır.
func (v *VM) programCalistir(stmts []ast.Statement) (Value, flowSignal, error) {
	c := v.yeniDerleyici()
	c.kok.onTanimla(stmts)
	return c.deyimler(c.kok, stmts).calistir(v, v.GlobalEnv)
}

// genelAltindaCalistir deyimleri genel kapsamın altında yeni bir ortamda çalıştırır
// (testler); ortam çağırana verilir ki 'ertele' blokları çalıştırılabilsin.
func (v *VM) genelAltindaDerle(stmts []ast.Statement) (*blokKodu, *kapsamBilgisi) {
	c := v.yeniDerleyici()
	alt := altKapsam(c.kok)
	alt.onTanimla(stmts)
	return c.deyimler(alt, stmts), alt.bilgi
}

// dinamikIfade ifadeyi, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) değerlendirir.
func (v *VM) dinamikIfade(expr ast.Expression, env *Environment) (Value, error) {
	return v.yeniDerleyici().ifade(sinirKapsami(), expr)(v, env)
}

// dinamikDeyimler deyimleri, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) çalıştırır.
func (v *VM) dinamikDeyimler(stmts []ast.Statement, env *Environment) (Value, flowSignal, error) {
	return v.yeniDerleyici().deyimler(sinirKapsami(), stmts).calistir(v, env)
}
