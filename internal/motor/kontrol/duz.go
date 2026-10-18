package kontrol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
)

// Düz değerlerin (sayaçlar, en iyi kayıp, okuyucu durumu) kayıpsız JSON yazımı. json_metin
// yetmez: Ondalık 99.0 geri okununca Tam olur, NaN yazılamaz, Sözlük'ün Tam anahtarı metne
// döner. Bu yazımda değer tipiyle birlikte saklanır:
//
//	yok → null · Mantık → true/false · Metin → "..." · Tam → 123 (her zaman tam sayı)
//	Ondalık → {"ondalık":"0.1"} (en kısa kayıpsız yazım; "NaN", "+Inf", "-Inf")
//	Liste → [...] · Sözlük → {"sözlük":[[anahtar, değer], ...]} (sıra korunur)
//
// Nesne biçimleri yalnızca bu iki etiketlidir; başka her şey hatadır.

var duzEtiketleri = strings.Fields("ondalık sözlük")

var etiketOndalik, etiketSozluk = duzEtiketleri[0], duzEtiketleri[1]

// enFazlaDuzDerinlik düz değerlerin iç içe derinliğidir (kendini içeren değerler için de sınır).
const enFazlaDuzDerinlik = 100

// duzYaz düz bir değeri yazar; yazılamayan bir değer yol ile (izlenen.en_iyi) bildirilir.
func duzYaz(v yurut.Value) (json.RawMessage, error) { return duzYazSuzgecli(v, nil) }

// duzYazSuzgecli duzYaz gibidir; metinler (anahtarlar dahil) önce suzgec'ten geçer (gizli
// değer süzgeci). suzgec nil olabilir.
func duzYazSuzgecli(v yurut.Value, suzgec func(string) string) (json.RawMessage, error) {
	y := duzYazici{suzgec: suzgec}
	if err := y.yaz(v, "", 0); err != nil {
		return nil, err
	}
	return y.b.Bytes(), nil
}

type duzYazici struct {
	b      bytes.Buffer
	suzgec func(string) string
}

func (y *duzYazici) yaz(v yurut.Value, yol string, derinlik int) error {
	b := &y.b
	if derinlik > enFazlaDuzDerinlik {
		return fmt.Errorf("%s: değer çok derin iç içe ya da kendini içeriyor (en çok %d düzey)", yolAdi(yol), enFazlaDuzDerinlik)
	}
	switch x := v.(type) {
	case *yurut.NullVal:
		b.WriteString("null")
	case *yurut.BoolVal:
		b.WriteString(strconv.FormatBool(x.Val))
	case *yurut.IntVal:
		b.WriteString(strconv.FormatInt(x.Val, 10))
	case *yurut.FloatVal:
		fmt.Fprintf(b, `{%q:%q}`, etiketOndalik, ondalikYazimi(x.Val))
	case *yurut.StringVal:
		m := x.Val
		if y.suzgec != nil {
			m = y.suzgec(m)
		}
		s, _ := json.Marshal(m)
		b.Write(s)
	case *yurut.ListVal:
		b.WriteByte('[')
		for i, el := range x.Elements {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := y.yaz(el, yol+"."+strconv.Itoa(i), derinlik+1); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case *yurut.DictVal:
		fmt.Fprintf(b, `{%q:[`, etiketSozluk)
		ilk := true
		for _, k := range x.Order {
			el, ok := x.Pairs[k]
			if !ok {
				continue
			}
			if !ilk {
				b.WriteByte(',')
			}
			ilk = false
			b.WriteByte('[')
			if err := y.yaz(x.KeyValue(k), yol, derinlik+1); err != nil {
				return err
			}
			b.WriteByte(',')
			if err := y.yaz(el, yol+"."+x.DisplayKey(k), derinlik+1); err != nil {
				return err
			}
			b.WriteByte(']')
		}
		b.WriteString("]}")
	default:
		return fmt.Errorf("%s: %s kaydedilemez; kontrol noktasındaki sözlük ve listeler yalnızca %s, %s, %s, %s, %s, %s ve %s içerebilir (tensörü .%s ile sayıya çevirin)",
			yolAdi(yol), yurut.TipVeOzet(v), kod.Ad(kod.A200), kod.Ad(kod.A201), kod.Ad(kod.A202), kod.Ad(kod.A203), kod.Ad(kod.A042),
			kod.Ad(kod.A205), kod.Ad(kod.A206), kod.Ad(kod.Y404))
	}
	return nil
}

func yolAdi(yol string) string {
	if yol == "" {
		return "değer"
	}
	return strings.TrimPrefix(yol, ".")
}

// ondalikYazimi kayıpsız en kısa yazımdır (strconv.ParseFloat geri okur).
func ondalikYazimi(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// duzOku duzYaz ile yazılmış bir değeri okur.
func duzOku(b []byte) (yurut.Value, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var ham any
	if err := d.Decode(&ham); err != nil {
		return nil, err
	}
	return duzCevir(ham, 0)
}

func duzCevir(ham any, derinlik int) (yurut.Value, error) {
	if derinlik > enFazlaDuzDerinlik {
		return nil, fmt.Errorf("değer çok derin iç içe (en çok %d düzey)", enFazlaDuzDerinlik)
	}
	switch x := ham.(type) {
	case nil:
		return yurut.NULL, nil
	case bool:
		return yurut.BoolToVal(x), nil
	case string:
		return &yurut.StringVal{Val: x}, nil
	case json.Number:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q bir tam sayı değil", string(x))
		}
		return yurut.NewInt(n), nil
	case []any:
		l := &yurut.ListVal{Elements: make([]yurut.Value, 0, len(x))}
		for _, el := range x {
			v, err := duzCevir(el, derinlik+1)
			if err != nil {
				return nil, err
			}
			l.Elements = append(l.Elements, v)
		}
		return l, nil
	case map[string]any:
		return etiketliCevir(x, derinlik)
	}
	return nil, fmt.Errorf("beklenmeyen değer %v", ham)
}

// etiketliCevir {"ondalık": ...} ve {"sözlük": ...} biçimlerini çevirir.
func etiketliCevir(x map[string]any, derinlik int) (yurut.Value, error) {
	if len(x) != 1 {
		return nil, fmt.Errorf("etiketli değer tek alanlı olmalı, %d alan var", len(x))
	}
	if s, ok := x[etiketOndalik]; ok {
		m, ok := s.(string)
		if !ok {
			return nil, fmt.Errorf("%s değeri metin olmalı", etiketOndalik)
		}
		f, err := strconv.ParseFloat(m, 64)
		if err != nil && !strings.Contains(err.Error(), "out of range") {
			return nil, fmt.Errorf("%q bir ondalık sayı değil", m)
		}
		return &yurut.FloatVal{Val: f}, nil
	}
	ciftler, ok := x[etiketSozluk].([]any)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen etiketli değer")
	}
	s := yurut.NewDictVal()
	for _, c := range ciftler {
		kv, ok := c.([]any)
		if !ok || len(kv) != 2 {
			return nil, fmt.Errorf("sözlük girdisi [anahtar, değer] olmalı")
		}
		k, err := duzCevir(kv[0], derinlik+1)
		if err != nil {
			return nil, err
		}
		switch k.(type) {
		case *yurut.ListVal, *yurut.DictVal:
			return nil, fmt.Errorf("sözlük anahtarı liste ya da sözlük olamaz")
		}
		v, err := duzCevir(kv[1], derinlik+1)
		if err != nil {
			return nil, err
		}
		s.SetValue(k, v)
	}
	return s, nil
}

// yerindeYenile kaydedilen değeri kullanıcının değerine yerinde yazar: Sözlük'ün ve Liste'nin
// içi değişir, değerin kendisi (kullanıcının değişkeni) aynı kalır.
func yerindeYenile(ad string, hedef, kayit yurut.Value) error {
	switch h := hedef.(type) {
	case *yurut.DictVal:
		k, ok := kayit.(*yurut.DictVal)
		if !ok {
			return fmt.Errorf("%q kayıtta %s, şimdi %s", ad, yurut.TipVeOzet(kayit), kod.Ad(kod.A206))
		}
		*h = *k
	case *yurut.ListVal:
		k, ok := kayit.(*yurut.ListVal)
		if !ok {
			return fmt.Errorf("%q kayıtta %s, şimdi %s", ad, yurut.TipVeOzet(kayit), kod.Ad(kod.A205))
		}
		h.Elements = k.Elements
	default:
		return fmt.Errorf("%q yerinde yenilenemez (%s)", ad, yurut.TipVeOzet(hedef))
	}
	return nil
}
