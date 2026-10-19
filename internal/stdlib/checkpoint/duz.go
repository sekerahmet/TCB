package checkpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
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
func duzYaz(v interp.Value) (json.RawMessage, error) { return duzYazSuzgecli(v, nil) }

// duzYazSuzgecli duzYaz gibidir; metinler (anahtarlar dahil) önce suzgec'ten geçer (gizli
// değer süzgeci). suzgec nil olabilir.
func duzYazSuzgecli(v interp.Value, suzgec func(string) string) (json.RawMessage, error) {
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

func (y *duzYazici) yaz(v interp.Value, yol string, derinlik int) error {
	b := &y.b
	if derinlik > enFazlaDuzDerinlik {
		return fmt.Errorf("%s: değer çok derin iç içe ya da kendini içeriyor (en çok %d düzey)", yolAdi(yol), enFazlaDuzDerinlik)
	}
	switch x := v.(type) {
	case *interp.NullVal:
		b.WriteString("null")
	case *interp.BoolVal:
		b.WriteString(strconv.FormatBool(x.Val))
	case *interp.IntVal:
		b.WriteString(strconv.FormatInt(x.Val, 10))
	case *interp.FloatVal:
		fmt.Fprintf(b, `{%q:%q}`, etiketOndalik, ondalikYazimi(x.Val))
	case *interp.StringVal:
		m := x.Val
		if y.suzgec != nil {
			m = y.suzgec(m)
		}
		s, _ := json.Marshal(m)
		b.Write(s)
	case *interp.ListVal:
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
	case *interp.DictVal:
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
			yolAdi(yol), interp.TipVeOzet(v), codes.Ad(codes.A200), codes.Ad(codes.A201), codes.Ad(codes.A202), codes.Ad(codes.A203), codes.Ad(codes.A042),
			codes.Ad(codes.A205), codes.Ad(codes.A206), codes.Ad(codes.Y404))
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
func duzOku(b []byte) (interp.Value, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var ham any
	if err := d.Decode(&ham); err != nil {
		return nil, err
	}
	return duzCevir(ham, 0)
}

func duzCevir(ham any, derinlik int) (interp.Value, error) {
	if derinlik > enFazlaDuzDerinlik {
		return nil, fmt.Errorf("değer çok derin iç içe (en çok %d düzey)", enFazlaDuzDerinlik)
	}
	switch x := ham.(type) {
	case nil:
		return interp.NULL, nil
	case bool:
		return interp.BoolToVal(x), nil
	case string:
		return &interp.StringVal{Val: x}, nil
	case json.Number:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q bir tam sayı değil", string(x))
		}
		return interp.NewInt(n), nil
	case []any:
		l := &interp.ListVal{Elements: make([]interp.Value, 0, len(x))}
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
func etiketliCevir(x map[string]any, derinlik int) (interp.Value, error) {
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
		return &interp.FloatVal{Val: f}, nil
	}
	ciftler, ok := x[etiketSozluk].([]any)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen etiketli değer")
	}
	s := interp.NewDictVal()
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
		case *interp.ListVal, *interp.DictVal:
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
func yerindeYenile(ad string, hedef, kayit interp.Value) error {
	switch h := hedef.(type) {
	case *interp.DictVal:
		k, ok := kayit.(*interp.DictVal)
		if !ok {
			return fmt.Errorf("%q kayıtta %s, şimdi %s", ad, interp.TipVeOzet(kayit), codes.Ad(codes.A206))
		}
		*h = *k
	case *interp.ListVal:
		k, ok := kayit.(*interp.ListVal)
		if !ok {
			return fmt.Errorf("%q kayıtta %s, şimdi %s", ad, interp.TipVeOzet(kayit), codes.Ad(codes.A205))
		}
		h.Elements = k.Elements
	default:
		return fmt.Errorf("%q yerinde yenilenemez (%s)", ad, interp.TipVeOzet(hedef))
	}
	return nil
}
