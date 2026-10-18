package vm

import (
	"errors"
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
)

// sabitKod her zaman aynı (değişmez) değeri veren kod üretir; sabitler bir kez kutulanır.
func sabitKod(d Value) ifadeKodu {
	return func(v *VM, env *Environment) (Value, error) { return d, nil }
}

var nullKod = sabitKod(NULL)

func (c *derleyici) ifadeler(k *derleyiciKapsam, exprs []ast.Expression) []ifadeKodu {
	kodlar := make([]ifadeKodu, len(exprs))
	for i, e := range exprs {
		kodlar[i] = c.ifade(k, e)
	}
	return kodlar
}

// ifadeSinyal deyim konumundaki bir ifadeyi derler. 'eğer' ve 'eşle' dallarında çalışan
// 'sonuç', 'kır', 'devam' sinyalleri yukarı iletilir; diğer ifadeler sinyal üretmez.
func (c *derleyici) ifadeSinyal(k *derleyiciKapsam, expr ast.Expression) deyimKodu {
	switch e := expr.(type) {
	case *ast.IfExpression:
		return c.eger(k, e)
	case *ast.MatchExpression:
		return c.esle(k, e)
	}
	kod := c.ifade(k, expr)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := kod(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalNone, nil
	}
}

// sinyalsiz deyim konumu dışında kullanılan 'eğer'/'eşle' için sinyali düşürür.
func sinyalsiz(d deyimKodu) ifadeKodu {
	return func(v *VM, env *Environment) (Value, error) {
		val, _, err := d(v, env)
		return val, err
	}
}

func (c *derleyici) ifade(k *derleyiciKapsam, expr ast.Expression) ifadeKodu {
	switch e := expr.(type) {
	case nil:
		return nullKod
	case *ast.IntegerLiteral:
		return sabitKod(NewInt(e.Value))
	case *ast.FloatLiteral:
		return sabitKod(&FloatVal{Val: e.Value})
	case *ast.StringLiteral:
		return sabitKod(&StringVal{Val: e.Value})
	case *ast.BooleanLiteral:
		if e.Value {
			return sabitKod(TRUE)
		}
		return sabitKod(FALSE)
	case *ast.NullLiteral:
		return nullKod

	case *ast.InterpolatedString:
		parcalar := c.ifadeler(k, e.Parts)
		return func(v *VM, env *Environment) (Value, error) {
			var sb strings.Builder
			for _, p := range parcalar {
				val, err := p(v, env)
				if err != nil {
					return nil, err
				}
				sb.WriteString(val.Inspect())
			}
			return &StringVal{Val: sb.String()}, nil
		}

	case *ast.Identifier:
		return c.adOku(k, e)

	case *ast.ListLiteral:
		elemanlar := c.ifadeler(k, e.Elements)
		return func(v *VM, env *Environment) (Value, error) {
			elems := make([]Value, 0, len(elemanlar))
			for _, el := range elemanlar {
				val, err := el(v, env)
				if err != nil {
					return nil, err
				}
				elems = append(elems, val)
			}
			return &ListVal{Elements: elems}, nil
		}

	case *ast.ListComprehension:
		return c.listeUreteci(k, e)

	case *ast.DictLiteral:
		anahtarlar := make([]ifadeKodu, len(e.Pairs))
		degerler := make([]ifadeKodu, len(e.Pairs))
		for i, p := range e.Pairs {
			anahtarlar[i] = c.ifade(k, p.Key)
			degerler[i] = c.ifade(k, p.Value)
		}
		return func(v *VM, env *Environment) (Value, error) {
			dict := NewDictVal()
			for i := range anahtarlar {
				kVal, err := anahtarlar[i](v, env)
				if err != nil {
					return nil, err
				}
				vVal, err := degerler[i](v, env)
				if err != nil {
					return nil, err
				}
				dict.SetValue(kVal, vVal)
			}
			return dict, nil
		}

	case *ast.RangeExpression:
		bas := c.ifade(k, e.Start)
		son := c.ifade(k, e.End)
		line := e.Line()
		dahil := e.Inclusive
		return func(v *VM, env *Environment) (Value, error) {
			startVal, err := bas(v, env)
			if err != nil {
				return nil, err
			}
			endVal, err := son(v, env)
			if err != nil {
				return nil, err
			}
			sInt, ok1 := startVal.(*IntVal)
			eInt, ok2 := endVal.(*IntVal)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("Satır %d: Aralık ('..') uçları Tam sayı olmalıdır", line)
			}
			return &RangeVal{Start: sInt.Val, End: eInt.Val, Inclusive: dahil}, nil
		}

	case *ast.PrefixExpression:
		return c.onEk(k, e)

	case *ast.InfixExpression:
		return c.ikili(k, e)

	case *ast.IfExpression:
		return sinyalsiz(c.eger(k, e))

	case *ast.MatchExpression:
		return sinyalsiz(c.esle(k, e))

	case *ast.FunctionLiteral:
		fk := c.fonksiyon(k, e.Parameters, e.Body, false)
		var tanim func(*Environment, Value, bool)
		if e.Name != "" {
			tanim = tanimlayici(k, e.Name)
		}
		return func(v *VM, env *Environment) (Value, error) {
			fn := &FuncVal{
				Name:       e.Name,
				Parameters: e.Parameters,
				Body:       e.Body,
				Env:        env,
				kod:        fk,
			}
			if tanim != nil {
				tanim(env, fn, false)
			}
			return fn, nil
		}

	case *ast.CallExpression:
		return c.cagri(k, e)

	case *ast.IndexExpression:
		sol := c.ifade(k, e.Left)
		indeks := c.ifade(k, e.Index)
		line := e.Line()
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			idx, err := indeks(v, env)
			if err != nil {
				return nil, err
			}
			return v.indeksDegeri(left, idx, line)
		}

	case *ast.MemberExpression:
		nesne := c.ifade(k, e.Object)
		return func(v *VM, env *Environment) (Value, error) {
			obj, err := nesne(v, env)
			if err != nil {
				return nil, err
			}
			return v.uyeDegeri(obj, e)
		}
	}
	return nullKod
}

// --- DEĞİŞKEN OKUMA ---

// adOku bir adın değerini okuyan kodu üretir. Derleme anında çözülen adlar yuvadan okunur;
// yuva boşsa (ya da ad çözülemediyse) arama adla yapılır.
func (c *derleyici) adOku(k *derleyiciKapsam, e *ast.Identifier) ifadeKodu {
	r := k.coz(e.Value)
	i := r.yuva
	switch r.tur {
	case cozGenel:
		g := r.genel
		return func(v *VM, env *Environment) (Value, error) {
			if i < len(g.vals) {
				if x := g.vals[i]; x != nil {
					return x, nil
				}
			}
			return adlaOku(env, e)
		}
	case cozYerel:
		switch r.derinlik {
		case 0:
			return func(v *VM, env *Environment) (Value, error) {
				if i < len(env.vals) {
					if x := env.vals[i]; x != nil {
						return x, nil
					}
				}
				return adlaOku(env, e)
			}
		case 1:
			return func(v *VM, env *Environment) (Value, error) {
				o := env.outer
				if i < len(o.vals) {
					if x := o.vals[i]; x != nil {
						return x, nil
					}
				}
				return adlaOku(env, e)
			}
		default:
			d := r.derinlik
			return func(v *VM, env *Environment) (Value, error) {
				o := env
				for j := 0; j < d; j++ {
					o = o.outer
				}
				if i < len(o.vals) {
					if x := o.vals[i]; x != nil {
						return x, nil
					}
				}
				return adlaOku(env, e)
			}
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		return adlaOku(env, e)
	}
}

func adlaOku(env *Environment, e *ast.Identifier) (Value, error) {
	val, ok := env.Get(e.Value)
	if !ok {
		return nil, fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken ya da fonksiyon yok%s",
			e.Line(), e.Token.Column, e.Value, suggestName(e.Value, env))
	}
	return val, nil
}

// yuvaOrtami çözümün işaret ettiği ortamı döndürür (yalnızca cozYerel/cozGenel için).
func yuvaOrtami(r cozum, env *Environment) *Environment {
	if r.tur == cozGenel {
		return r.genel
	}
	o := env
	for j := 0; j < r.derinlik; j++ {
		o = o.outer
	}
	return o
}

// --- İŞLEÇLER ---

func (c *derleyici) onEk(k *derleyiciKapsam, e *ast.PrefixExpression) ifadeKodu {
	sag := c.ifade(k, e.Right)
	line := e.Line()
	switch e.Operator {
	case "!":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			if right.IsTruthy() {
				return FALSE, nil
			}
			return TRUE, nil
		}
	case "-":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			switch rv := right.(type) {
			case *IntVal:
				return NewInt(-rv.Val), nil
			case *FloatVal:
				return &FloatVal{Val: -rv.Val}, nil
			case Olumsuzlanabilir:
				val, err := rv.Olumsuz()
				return val, satirEki(line, err)
			}
			return nil, fmt.Errorf("Satır %d: '-' operatörü %s tipine uygulanamaz", line, right.Type())
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		if _, err := sag(v, env); err != nil {
			return nil, err
		}
		return NULL, nil
	}
}

// tamHizli iki Tam sayı arasındaki işlemin hızlı yoludur; ok=false ise (taşma, sıfıra
// bölme, desteklenmeyen işleç) genel yola (evalInfixKod) düşülür ve hata orada üretilir.
type tamHizli func(a, b int64) (Value, bool)

func tamHizliIslev(kod uint8) tamHizli {
	switch kod {
	case opTopla:
		return func(a, b int64) (Value, bool) {
			s := a + b
			if (s > a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opCikar:
		return func(a, b int64) (Value, bool) {
			s := a - b
			if (s < a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opCarp:
		return func(a, b int64) (Value, bool) {
			if !mulFits(a, b) {
				return nil, false
			}
			return NewInt(a * b), true
		}
	case opMod:
		return func(a, b int64) (Value, bool) {
			if b == 0 {
				return nil, false
			}
			return NewInt(a % b), true
		}
	case opBol:
		return func(a, b int64) (Value, bool) {
			if b == 0 {
				return nil, false
			}
			if a%b == 0 {
				return NewInt(a / b), true
			}
			return &FloatVal{Val: float64(a) / float64(b)}, true
		}
	case opKucuk:
		return func(a, b int64) (Value, bool) { return BoolToVal(a < b), true }
	case opBuyuk:
		return func(a, b int64) (Value, bool) { return BoolToVal(a > b), true }
	case opKucukEsit:
		return func(a, b int64) (Value, bool) { return BoolToVal(a <= b), true }
	case opBuyukEsit:
		return func(a, b int64) (Value, bool) { return BoolToVal(a >= b), true }
	case opEsit:
		// ValuesEqual ile aynı kural: sayılar Ondalık olarak karşılaştırılır
		return func(a, b int64) (Value, bool) { return BoolToVal(float64(a) == float64(b)), true }
	case opEsitDegil:
		return func(a, b int64) (Value, bool) { return BoolToVal(float64(a) != float64(b)), true }
	}
	return nil
}

func (c *derleyici) ikili(k *derleyiciKapsam, e *ast.InfixExpression) ifadeKodu {
	kod := islecKodu(e.Operator)
	sol := c.ifade(k, e.Left)
	sag := c.ifade(k, e.Right)
	op := e.Operator
	line := e.Line()

	// Kısa devre ve özel operatörler (&&, ||, ??, |>)
	switch kod {
	case opVe:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if !dogruMu(left) {
				return FALSE, nil
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(dogruMu(right)), nil
		}
	case opVeya:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if dogruMu(left) {
				return TRUE, nil
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(dogruMu(right)), nil
		}
	case opYoksa:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if left.Type() != VAL_NULL {
				return left, nil
			}
			return sag(v, env)
		}
	case opBoru:
		return c.boru(k, e.Left, e.Right)
	}

	hizli := tamHizliIslev(kod)
	if hizli != nil {
		// Sağ taraf Tam sayı sabitiyse (i < 1000, x % 7) sağ kod hiç çağrılmaz
		if lit, ok := e.Right.(*ast.IntegerLiteral); ok {
			b := lit.Value
			sagDeger := NewInt(b)
			// Sol taraf yerel (derinlik 0) ya da genel bir değişkense yuvası doğrudan okunur
			if id, ok := e.Left.(*ast.Identifier); ok {
				if r := k.coz(id.Value); r.tur == cozGenel || (r.tur == cozYerel && r.derinlik == 0) {
					i, g := r.yuva, r.genel
					return func(v *VM, env *Environment) (Value, error) {
						o := g
						if o == nil {
							o = env
						}
						var left Value
						if i < len(o.vals) {
							left = o.vals[i]
						}
						if li, ok := left.(*IntVal); ok {
							if x, ok := hizli(li.Val, b); ok {
								return x, nil
							}
						}
						if left == nil {
							var err error
							if left, err = sol(v, env); err != nil {
								return nil, err
							}
						}
						return v.evalInfixKod(kod, op, left, sagDeger, line)
					}
				}
			}
			return func(v *VM, env *Environment) (Value, error) {
				left, err := sol(v, env)
				if err != nil {
					return nil, err
				}
				if li, ok := left.(*IntVal); ok {
					if x, ok := hizli(li.Val, b); ok {
						return x, nil
					}
				}
				return v.evalInfixKod(kod, op, left, sagDeger, line)
			}
		}
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			if li, ok := left.(*IntVal); ok {
				if ri, ok := right.(*IntVal); ok {
					if x, ok := hizli(li.Val, ri.Val); ok {
						return x, nil
					}
				}
			}
			return v.evalInfixKod(kod, op, left, right, line)
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		left, err := sol(v, env)
		if err != nil {
			return nil, err
		}
		right, err := sag(v, env)
		if err != nil {
			return nil, err
		}
		return v.evalInfixKod(kod, op, left, right, line)
	}
}

// boru 'veri |> f(...)' ve 'veri |> f' ifadelerini derler.
func (c *derleyici) boru(k *derleyiciKapsam, left ast.Expression, right ast.Expression) ifadeKodu {
	sol := c.ifade(k, left)
	// Sağ taraf bir fonksiyon çağrısı ise (örn: veri |> filtrele((x) => x > 2))
	if callExpr, ok := right.(*ast.CallExpression); ok {
		fonk := c.ifade(k, callExpr.Function)
		argKod := c.ifadeler(k, callExpr.Arguments)
		adlar := make([]string, len(callExpr.NamedArgs))
		adliKod := make([]ifadeKodu, len(callExpr.NamedArgs))
		for i, na := range callExpr.NamedArgs {
			adlar[i] = na.Name
			adliKod[i] = c.ifade(k, na.Value)
		}
		line := callExpr.Line()
		return func(v *VM, env *Environment) (Value, error) {
			leftVal, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			fnVal, err := fonk(v, env)
			if err != nil {
				return nil, err
			}
			args := make([]Value, 1, len(argKod)+1)
			args[0] = leftVal
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

	// Sağ taraf doğrudan bir fonksiyon ise (örn: veri |> sırala)
	fonk := c.ifade(k, right)
	line := right.Line()
	return func(v *VM, env *Environment) (Value, error) {
		leftVal, err := sol(v, env)
		if err != nil {
			return nil, err
		}
		fnVal, err := fonk(v, env)
		if err != nil {
			return nil, err
		}
		if err := checkArity(fnVal, 1, nil, line); err != nil {
			return nil, err
		}
		return v.ApplyFunction(fnVal, []Value{leftVal}, nil, line)
	}
}

// --- EĞER / EŞLE ---

func (c *derleyici) eger(k *derleyiciKapsam, e *ast.IfExpression) deyimKodu {
	kosul := c.ifade(k, e.Condition)
	ana := c.blokDeyimi(k, e.Consequence)
	digerKosul := make([]ifadeKodu, len(e.ElseIfs))
	digerDal := make([]deyimKodu, len(e.ElseIfs))
	for i, ei := range e.ElseIfs {
		digerKosul[i] = c.ifade(k, ei.Condition)
		digerDal[i] = c.blokDeyimi(k, ei.Consequence)
	}
	var yoksa deyimKodu
	if e.Alternative != nil {
		yoksa = c.blokDeyimi(k, e.Alternative)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		cond, err := kosul(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if dogruMu(cond) {
			return ana(v, env)
		}
		for i, dk := range digerKosul {
			elifCond, err := dk(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if dogruMu(elifCond) {
				return digerDal[i](v, env)
			}
		}
		if yoksa != nil {
			return yoksa(v, env)
		}
		return NULL, signalNone, nil
	}
}

func (c *derleyici) esle(k *derleyiciKapsam, e *ast.MatchExpression) deyimKodu {
	type durumKodu struct {
		varsayilan bool
		desenler   []ifadeKodu
		govde      deyimKodu
		deger      ifadeKodu
	}
	konu := c.ifade(k, e.Subject)
	durumlar := make([]durumKodu, len(e.Cases))
	for i, mc := range e.Cases {
		d := durumKodu{varsayilan: mc.IsDefault, desenler: c.ifadeler(k, mc.Patterns)}
		if mc.Body != nil {
			d.govde = c.blokDeyimi(k, mc.Body)
		}
		if mc.ValueExpr != nil {
			d.deger = c.ifade(k, mc.ValueExpr)
		}
		durumlar[i] = d
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		subj, err := konu(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		for _, d := range durumlar {
			matched := d.varsayilan
			if !matched {
				for _, pat := range d.desenler {
					patVal, err := pat(v, env)
					if err != nil {
						return nil, signalNone, err
					}
					if desenUyar(subj, patVal) {
						matched = true
						break
					}
				}
			}
			if matched {
				if d.govde != nil {
					return d.govde(v, env)
				}
				if d.deger != nil {
					val, err := d.deger(v, env)
					return val, signalNone, err
				}
				return NULL, signalNone, nil
			}
		}
		return NULL, signalNone, nil
	}
}

// desenUyar 'eşle' deseninin konuya uyup uymadığını söyler (aralık desenleri dahil).
func desenUyar(subj, patVal Value) bool {
	if rng, ok := patVal.(*RangeVal); ok {
		if iv, ok := subj.(*IntVal); ok {
			if rng.Inclusive && iv.Val >= rng.Start && iv.Val <= rng.End {
				return true
			} else if !rng.Inclusive && iv.Val >= rng.Start && iv.Val < rng.End {
				return true
			}
		}
		return false
	}
	return ValuesEqual(subj, patVal)
}

// --- LİSTE ÜRETECİ ---

func (c *derleyici) listeUreteci(k *derleyiciKapsam, lc *ast.ListComprehension) ifadeKodu {
	kaynak := c.ifade(k, lc.Iterable)
	alt := altKapsam(k)
	degiskenYuva := alt.bilgi.yuva(lc.VarName)
	ifadeAdlari(lc.Condition, alt.tanimla)
	ifadeAdlari(lc.Output, alt.tanimla)
	var kosul ifadeKodu
	if lc.Condition != nil {
		kosul = c.ifade(alt, lc.Condition)
	}
	cikti := c.ifade(alt, lc.Output)
	bilgi := alt.bilgi
	// Çıktı ya da koşul fonksiyon oluşturmuyorsa ortam yakalanamaz; tek ortam yeterlidir.
	yeniden := !ifadeYakalar(lc.Output) && !ifadeYakalar(lc.Condition)
	line := lc.Line()
	return func(v *VM, env *Environment) (Value, error) {
		iterVal, err := kaynak(v, env)
		if err != nil {
			return nil, err
		}
		items, err := uretecOgeleri(iterVal, line)
		if err != nil {
			return nil, err
		}
		var results []Value
		tur := donguOrtami{dis: env, bilgi: bilgi, yeniden: yeniden}
		for _, item := range items {
			compEnv := tur.yeni()
			compEnv.vals[degiskenYuva] = item
			if kosul != nil {
				condVal, err := kosul(v, compEnv)
				if err != nil {
					return nil, err
				}
				if !condVal.IsTruthy() {
					continue
				}
			}
			outVal, err := cikti(v, compEnv)
			if err != nil {
				return nil, err
			}
			results = append(results, outVal)
		}
		return &ListVal{Elements: results}, nil
	}
}

// --- ÇAĞRILAR ---

func (c *derleyici) cagri(k *derleyiciKapsam, e *ast.CallExpression) ifadeKodu {
	fonk := c.ifade(k, e.Function)
	argKod := c.ifadeler(k, e.Arguments)
	nargs := len(argKod)
	line := e.Line()

	if len(e.NamedArgs) == 0 {
		return func(v *VM, env *Environment) (Value, error) {
			fnVal, err := fonk(v, env)
			if err != nil {
				return nil, err
			}
			// Hızlı yol: derlenmiş kullanıcı fonksiyonu. Argümanlar ara dilim kurulmadan
			// doğrudan yeni ortamın yuvalarına hesaplanır.
			if fn, ok := fnVal.(*FuncVal); ok && fn.kod != nil && fn.kod.basit && nargs <= len(fn.Parameters) && varsayilanlarYeter(fn, nargs) {
				fk := fn.kod
				fnEnv := fk.ortamAl(fn.Env)
				for i, a := range argKod {
					av, err := a(v, env)
					if err != nil {
						return nil, err
					}
					fnEnv.vals[fk.param[i]] = av
				}
				return v.cagirHazir(fn, fk, fnEnv, nil, nargs, nil, line)
			}
			args := make([]Value, 0, nargs)
			for _, a := range argKod {
				av, err := a(v, env)
				if err != nil {
					return nil, err
				}
				args = append(args, av)
			}
			if err := checkArity(fnVal, len(args), nil, line); err != nil {
				return nil, err
			}
			return v.ApplyFunction(fnVal, args, nil, line)
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

// varsayilanlarYeter verilmeyen her parametrenin varsayılanı var mı (checkArity'nin hızlı yolu)?
func varsayilanlarYeter(fn *FuncVal, nargs int) bool {
	for _, p := range fn.Parameters[nargs:] {
		if p.DefaultValue == nil {
			return false
		}
	}
	return true
}

// --- ATAMA ---

// atamaHedefi bir atama hedefini derler; dönen işlev sağ taraf değerini hedefe yazar
// (bileşik atamada önce mevcut değerle işlem yapar).
func (c *derleyici) atamaHedefi(k *derleyiciKapsam, target ast.Expression, kod uint8, line int) func(v *VM, env *Environment, rhs Value) error {
	bilesik := kod == opTopla || kod == opCikar || kod == opCarp || kod == opBol
	switch t := target.(type) {
	case *ast.Identifier:
		r := k.coz(t.Value)
		if r.tur == cozDinamik {
			return func(v *VM, env *Environment, rhs Value) error {
				return v.adlaAta(env, t, kod, rhs, line)
			}
		}
		i := r.yuva
		hizli := tamHizliIslev(kod)
		opMetni := atamaIslecMetni(kod)
		return func(v *VM, env *Environment, rhs Value) error {
			o := yuvaOrtami(r, env)
			if i < len(o.vals) {
				if cur := o.vals[i]; cur != nil && !o.sabitMi(i) {
					if !bilesik {
						o.vals[i] = rhs
						return nil
					}
					if li, ok := cur.(*IntVal); ok {
						if ri, ok := rhs.(*IntVal); ok {
							if x, ok := hizli(li.Val, ri.Val); ok {
								o.vals[i] = x
								return nil
							}
						}
					}
					yeni, err := v.evalInfixKod(kod, opMetni, cur, rhs, line)
					if err != nil {
						return err
					}
					o.vals[i] = yeni
					return nil
				}
			}
			return v.adlaAta(env, t, kod, rhs, line)
		}

	case *ast.IndexExpression:
		var icSol, icIndeks ifadeKodu
		innerIdx, is2D := t.Left.(*ast.IndexExpression)
		if is2D {
			icSol = c.ifade(k, innerIdx.Left)
			icIndeks = c.ifade(k, innerIdx.Index)
		}
		sol := c.ifade(k, t.Left)
		indeks := c.ifade(k, t.Index)
		return func(v *VM, env *Environment, rhs Value) error {
			// 2 boyutlu Matris hücre ataması kontrolü: A[satır][sütun] = değer
			if is2D {
				baseObj, err := icSol(v, env)
				if err != nil {
					return err
				}
				if mat, isMat := baseObj.(*MatrixVal); isMat {
					rIdxVal, err := icIndeks(v, env)
					if err != nil {
						return err
					}
					cIdxVal, err := indeks(v, env)
					if err != nil {
						return err
					}
					return v.matrisHucresineAta(mat, rIdxVal, cIdxVal, kod, rhs, line)
				}
			}
			left, err := sol(v, env)
			if err != nil {
				return err
			}
			idx, err := indeks(v, env)
			if err != nil {
				return err
			}
			return v.indekseAta(left, idx, kod, rhs, line)
		}

	case *ast.MemberExpression:
		nesne := c.ifade(k, t.Object)
		prop := t.Property
		return func(v *VM, env *Environment, rhs Value) error {
			obj, err := nesne(v, env)
			if err != nil {
				return err
			}
			return v.uyeyeAta(obj, prop, kod, rhs, line)
		}
	}
	return func(v *VM, env *Environment, rhs Value) error {
		return fmt.Errorf("Satır %d: Geçersiz atama hedefi", line)
	}
}

// adlaAta bir ada (yuvası bilinmeden) atama yapar.
func (v *VM) adlaAta(env *Environment, t *ast.Identifier, kod uint8, rightVal Value, line int) error {
	var finalVal Value = rightVal
	if kod != opAta {
		cur, ok := env.Get(t.Value)
		if !ok {
			return fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken yok%s",
				line, t.Token.Column, t.Value, suggestName(t.Value, env))
		}
		var err error
		finalVal, err = v.atamaIslemi(kod, cur, rightVal, line)
		if err != nil {
			return err
		}
	}
	if err := env.Assign(t.Value, finalVal); err != nil {
		var tanimsiz *TanimsizDegiskenHatasi
		if errors.As(err, &tanimsiz) {
			return fmt.Errorf("Satır %d, Sütun %d: %v%s", line, t.Token.Column, err, suggestName(t.Value, env))
		}
		return fmt.Errorf("Satır %d: %v", line, err)
	}
	return nil
}
