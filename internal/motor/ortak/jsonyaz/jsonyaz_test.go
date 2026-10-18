package jsonyaz

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"tr_dili/internal/dil/yurut"
)

func sozluk(ciftler ...interface{}) *yurut.DictVal {
	d := yurut.NewDictVal()
	for i := 0; i < len(ciftler); i += 2 {
		d.Set(ciftler[i].(string), ciftler[i+1].(yurut.Value))
	}
	return d
}

func liste(el ...yurut.Value) *yurut.ListVal { return &yurut.ListVal{Elements: el} }

// Anahtarlar sözlükteki sırayla yazılır (eskiden Go map'i yüzünden alfabetik oluyordu).
func TestAnahtarSirasiKorunur(t *testing.T) {
	d := sozluk("b", yurut.NewInt(1), "a", yurut.NewInt(2))
	b, err := Yaz(d)
	if err != nil || string(b) != `{"b":1,"a":2}` {
		t.Fatalf("%s %v", b, err)
	}
}

// Biçim, alfabetik sıralı sözlükte encoding/json ile bayt bayt aynıdır (girintili ve girintisiz).
func TestEncodingJSONIleAyniBicim(t *testing.T) {
	d := sozluk("a", liste(yurut.NewInt(1), &yurut.FloatVal{Val: 0.5}, &yurut.FloatVal{Val: 1e-7}, &yurut.FloatVal{Val: 1e21}, &yurut.FloatVal{Val: 2}),
		"b", sozluk(), "c", liste(), "d", yurut.NULL, "e", yurut.TRUE, "f", &yurut.StringVal{Val: "<ş\"\n>"},
		"g", sozluk("x", liste(sozluk("y", yurut.NewInt(-3)))))
	goDeger := goDegeri(d)
	for _, girinti := range []string{"", "  "} {
		var bekle []byte
		if girinti == "" {
			bekle, _ = json.Marshal(goDeger)
		} else {
			bekle, _ = json.MarshalIndent(goDeger, "", girinti)
		}
		b, err := YazGirintili(d, girinti)
		if err != nil || string(b) != string(bekle) {
			t.Fatalf("girinti %q:\n%s\nbeklenen:\n%s (%v)", girinti, b, bekle, err)
		}
	}
}

func TestDongusVeDerinlikHata(t *testing.T) {
	l := liste(yurut.NewInt(1))
	l.Elements = append(l.Elements, l)
	if _, err := Yaz(l); !errors.Is(err, yurut.ErrDongusel) {
		t.Fatalf("döngü hatası bekleniyordu: %v", err)
	}
	d := yurut.NewDictVal()
	d.Set("ben", liste(d))
	if _, err := Yaz(d); !errors.Is(err, yurut.ErrDongusel) {
		t.Fatalf("dolaylı döngü hatası bekleniyordu: %v", err)
	}
	// Paylaşılan (döngüsüz) alt değer serbesttir
	ortak := liste(yurut.NewInt(7))
	if b, err := Yaz(liste(ortak, ortak)); err != nil || string(b) != "[[7],[7]]" {
		t.Fatalf("%s %v", b, err)
	}
	var derin yurut.Value = yurut.NewInt(0)
	for i := 0; i < yurut.DisDegerDerinligi+5; i++ {
		derin = liste(derin)
	}
	if _, err := Yaz(derin); err == nil || !strings.Contains(err.Error(), "çok derin") {
		t.Fatalf("derinlik hatası bekleniyordu: %v", err)
	}
}

func TestTanimsizVeSonsuzSayiHata(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := Yaz(sozluk("d", &yurut.FloatVal{Val: f}))
		if err == nil || !strings.Contains(err.Error(), "JSON'a yazılamaz") {
			t.Fatalf("%v için hata bekleniyordu: %v", f, err)
		}
	}
}

// Tam anahtar 1 ile metin anahtar "1" JSON'da aynı anahtardır: biri sessizce atılmaz.
func TestGorunusuAyniAnahtarHata(t *testing.T) {
	d := yurut.NewDictVal()
	d.SetValue(yurut.NewInt(1), yurut.NewInt(10))
	d.Set("1", yurut.NewInt(20))
	if _, err := Yaz(d); err == nil || !strings.Contains(err.Error(), "görünüşü aynı") {
		t.Fatalf("çakışma hatası bekleniyordu: %v", err)
	}
	tek := yurut.NewDictVal()
	tek.SetValue(yurut.NewInt(1), yurut.NewInt(10))
	if b, err := Yaz(tek); err != nil || string(b) != `{"1":10}` {
		t.Fatalf("%s %v", b, err)
	}
}

// goDegeri bir TR değerini encoding/json'un yazabileceği Go değerine çevirir (yalnızca bu
// testin karşılaştırması için; döngü denetimi yapmaz).
func goDegeri(v yurut.Value) any {
	switch x := v.(type) {
	case *yurut.NullVal:
		return nil
	case *yurut.BoolVal:
		return x.Val
	case *yurut.IntVal:
		return x.Val
	case *yurut.FloatVal:
		return x.Val
	case *yurut.StringVal:
		return x.Val
	case *yurut.ListVal:
		out := make([]any, len(x.Elements))
		for i, e := range x.Elements {
			out[i] = goDegeri(e)
		}
		return out
	case *yurut.DictVal:
		out := make(map[string]any, len(x.Pairs))
		for k, e := range x.Pairs {
			out[x.DisplayKey(k)] = goDegeri(e)
		}
		return out
	}
	return v.Inspect()
}
