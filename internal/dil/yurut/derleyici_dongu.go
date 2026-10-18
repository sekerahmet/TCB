package yurut

import (
	"fmt"
	"tr_dili/internal/dil/agac"
)

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

// donguIcinde 'döngü x içinde ...' deyimini derler. Döngü değişkenleri ve gövdenin
// tanımları her turun kendi ortamındadır.
func (c *derleyici) donguIcinde(k *derleyiciKapsam, s *agac.ForInStatement) deyimKodu {
	kaynak := c.ifade(k, s.Iterable)
	alt := altKapsam(k)
	anahtarYuva := -1
	if s.KeyVar != "" {
		anahtarYuva = alt.bilgi.yuva(s.KeyVar)
		alt.golgeKaydet(s.KeyVar, s.Line(), "bir döngü değişkenine")
	}
	degerYuva := alt.bilgi.yuva(s.ValueVar)
	alt.golgeKaydet(s.ValueVar, s.Line(), "bir döngü değişkenine")
	var govdeDeyimleri []agac.Statement
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
		var devir uint32
		switch it := iterVal.(type) {
		case *RangeVal:
			ilk, son, adim, bos := aralikAdimlari(it)
			if bos {
				break
			}
			idx := int64(0)
			// Son değer dahil gezilir ve döngü onda biter: int64 sınırında taşma olmaz.
			for i := ilk; ; i += adim {
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
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
				if i == son {
					break
				}
				idx++
			}

		case *ListVal:
			for idx, item := range it.Elements {
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
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
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
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
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
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

		case Yinelenebilir:
			// Dış değerler (ör. tensör: ilk eksen boyunca öğeler)
			n, err := it.YinelemeUzunlugu()
			if err != nil {
				return nil, signalNone, satirEki(line, err)
			}
			for idx := 0; idx < n; idx++ {
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
				item, err := it.YinelemeOgesi(idx)
				if err != nil {
					return nil, signalNone, satirEki(line, err)
				}
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

		default:
			return nil, signalNone, fmt.Errorf("Satır %d: %s tipi üzerinde döngü kurulamaz", line, iterVal.Type())
		}
		return lastVal, signalNone, nil
	}
}

// iken 'iken koşul { ... }' deyimini derler. Gövde ad tanımlamıyorsa tur ortamı açılmaz.
func (c *derleyici) iken(k *derleyiciKapsam, s *agac.WhileStatement) deyimKodu {
	kosul := c.ifade(k, s.Condition)
	var govdeDeyimleri []agac.Statement
	if s.Body != nil {
		govdeDeyimleri = s.Body.Statements
	}
	if s.Body == nil || blokBayraklari(s.Body)&blokTanimlar == 0 {
		govde := c.deyimler(k, govdeDeyimleri)
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			var lastVal Value = NULL
			var devir uint32
			for {
				if devir++; devir&255 == 0 {
					v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
				}
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
		var devir uint32
		for {
			if devir++; devir&255 == 0 {
				v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
			}
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

// --- LİSTE ÜRETECİ ---

func (c *derleyici) listeUreteci(k *derleyiciKapsam, lc *agac.ListComprehension) ifadeKodu {
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
		var devir uint32
		for _, item := range items {
			if devir++; devir&255 == 0 {
				v.devret() // her 256 turda bir: bekleyen göreve kilidi devret
			}
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
