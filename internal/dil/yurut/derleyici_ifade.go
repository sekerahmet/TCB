package yurut

import (
	"fmt"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ipucu"
)

// sabitKod her zaman aynı (değişmez) değeri veren kod üretir; sabitler bir kez kutulanır.
func sabitKod(d Value) ifadeKodu {
	return func(v *VM, env *Environment) (Value, error) { return d, nil }
}

var nullKod = sabitKod(NULL)

func (c *derleyici) ifadeler(k *derleyiciKapsam, exprs []agac.Expression) []ifadeKodu {
	kodlar := make([]ifadeKodu, len(exprs))
	for i, e := range exprs {
		kodlar[i] = c.ifade(k, e)
	}
	return kodlar
}

// ifadeSinyal deyim konumundaki bir ifadeyi derler. 'eğer' ve 'eşle' dallarında çalışan
// 'sonuç', 'kır', 'devam' sinyalleri yukarı iletilir; diğer ifadeler sinyal üretmez.
func (c *derleyici) ifadeSinyal(k *derleyiciKapsam, expr agac.Expression) deyimKodu {
	switch e := expr.(type) {
	case *agac.IfExpression:
		return c.eger(k, e)
	case *agac.MatchExpression:
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

func (c *derleyici) ifade(k *derleyiciKapsam, expr agac.Expression) ifadeKodu {
	switch e := expr.(type) {
	case nil:
		return nullKod
	case *agac.IntegerLiteral:
		return sabitKod(NewInt(e.Value))
	case *agac.FloatLiteral:
		return sabitKod(&FloatVal{Val: e.Value})
	case *agac.StringLiteral:
		return sabitKod(&StringVal{Val: e.Value})
	case *agac.BooleanLiteral:
		if e.Value {
			return sabitKod(TRUE)
		}
		return sabitKod(FALSE)
	case *agac.NullLiteral:
		return nullKod

	case *agac.InterpolatedString:
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

	case *agac.Identifier:
		return c.adOku(k, e)

	case *agac.ListLiteral:
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

	case *agac.ListComprehension:
		return c.listeUreteci(k, e)

	case *agac.DictLiteral:
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

	case *agac.RangeExpression:
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
				return nil, fmt.Errorf("Satır %d: %s ('..') uçları %s sayı olmalıdır", line, VAL_RANGE, VAL_INT)
			}
			return &RangeVal{Start: sInt.Val, End: eInt.Val, Inclusive: dahil}, nil
		}

	case *agac.PrefixExpression:
		return c.onEk(k, e)

	case *agac.InfixExpression:
		return c.ikili(k, e)

	case *agac.IfExpression:
		return sinyalsiz(c.eger(k, e))

	case *agac.MatchExpression:
		return sinyalsiz(c.esle(k, e))

	case *agac.FunctionLiteral:
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

	case *agac.CallExpression:
		kod := c.cagri(k, e)
		// Gölgelenmiş bir yerleşiğin çağrısı: hata mesajı gölgelemeyi söyler (golge.go).
		if id, ok := e.Function.(*agac.Identifier); ok && yerlesikAdi(id.Value) {
			if g, ok := k.golgeBul(id.Value); ok {
				return golgeliCagri(kod, c.ifade(k, id), id.Value, g, e.Line())
			}
		}
		return kod

	case *agac.IndexExpression:
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

	case *agac.MemberExpression:
		nesne := c.ifade(k, e.Object)
		u := uyeCoz(e.Property) // ad → kod derleme anında bir kez çözülür
		return func(v *VM, env *Environment) (Value, error) {
			obj, err := nesne(v, env)
			if err != nil {
				return nil, err
			}
			return v.uyeDegeri(obj, e, u)
		}
	}
	return nullKod
}

// --- DEĞİŞKEN OKUMA ---

// adOku bir adın değerini okuyan kodu üretir. Derleme anında çözülen adlar yuvadan okunur;
// yuva boşsa (ya da ad çözülemediyse) arama adla yapılır.
func (c *derleyici) adOku(k *derleyiciKapsam, e *agac.Identifier) ifadeKodu {
	r := k.coz(e.Value)
	i := r.yuva
	switch r.tur {
	case cozHenuzYok:
		return func(v *VM, env *Environment) (Value, error) {
			return nil, fmt.Errorf("Satır %d, Sütun %d: '%s' parametresi henüz tanımlanmadı; varsayılan değer yalnızca kendisinden önceki parametreleri görür", e.Line(), e.Token.Column, e.Value)
		}
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

func adlaOku(env *Environment, e *agac.Identifier) (Value, error) {
	val, ok := env.Get(e.Value)
	if !ok {
		if h := ipucu.TasinanGenelAdHatasi(e.Value); h != "" {
			return nil, fmt.Errorf("Satır %d, Sütun %d: %s", e.Line(), e.Token.Column, h)
		}
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
