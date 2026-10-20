package jsonwrite

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

func dict(pairs ...interface{}) *interp.DictVal {
	d := interp.NewDictVal()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i].(string), pairs[i+1].(interp.Value))
	}
	return d
}

func list(elem ...interp.Value) *interp.ListVal { return &interp.ListVal{Elements: elem} }

// Anahtarlar sözlükteki sırayla yazılır (eskiden Go map'i yüzünden alfabetik oluyordu).
func TestKeyOrderKept(t *testing.T) {
	d := dict("b", interp.NewInt(1), "a", interp.NewInt(2))
	b, err := Write(d)
	if err != nil || string(b) != `{"b":1,"a":2}` {
		t.Fatalf("%s %v", b, err)
	}
}

// Biçim, alfabetik sıralı sözlükte encoding/json ile bayt bayt aynıdır (girintili ve girintisiz).
func TestSameFormatAsEncodingJSON(t *testing.T) {
	d := dict("a", list(interp.NewInt(1), &interp.FloatVal{Val: 0.5}, &interp.FloatVal{Val: 1e-7}, &interp.FloatVal{Val: 1e21}, &interp.FloatVal{Val: 2}),
		"b", dict(), "c", list(), "d", interp.NULL, "e", interp.TRUE, "f", &interp.StringVal{Val: "<ş\"\n>"},
		"g", dict("x", list(dict("y", interp.NewInt(-3)))))
	goVal := goValue(d)
	for _, indent := range []string{"", "  "} {
		var want []byte
		if indent == "" {
			want, _ = json.Marshal(goVal)
		} else {
			want, _ = json.MarshalIndent(goVal, "", indent)
		}
		b, err := WriteIndent(d, indent)
		if err != nil || string(b) != string(want) {
			t.Fatalf("girinti %q:\n%s\nbeklenen:\n%s (%v)", indent, b, want, err)
		}
	}
}

func TestCycleAndDepthError(t *testing.T) {
	l := list(interp.NewInt(1))
	l.Elements = append(l.Elements, l)
	if _, err := Write(l); !errors.Is(err, interp.ErrCyclic) {
		t.Fatalf("döngü hatası bekleniyordu: %v", err)
	}
	d := interp.NewDictVal()
	d.Set("ben", list(d))
	if _, err := Write(d); !errors.Is(err, interp.ErrCyclic) {
		t.Fatalf("dolaylı döngü hatası bekleniyordu: %v", err)
	}
	// Paylaşılan (döngüsüz) alt değer serbesttir
	shared := list(interp.NewInt(7))
	if b, err := Write(list(shared, shared)); err != nil || string(b) != "[[7],[7]]" {
		t.Fatalf("%s %v", b, err)
	}
	var deep interp.Value = interp.NewInt(0)
	for i := 0; i < interp.MaxExternalDepth+5; i++ {
		deep = list(deep)
	}
	if _, err := Write(deep); err == nil || !strings.Contains(err.Error(), "çok derin") {
		t.Fatalf("derinlik hatası bekleniyordu: %v", err)
	}
}

func TestNaNAndInfError(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := Write(dict("d", &interp.FloatVal{Val: f}))
		if err == nil || !strings.Contains(err.Error(), "JSON'a yazılamaz") {
			t.Fatalf("%v için hata bekleniyordu: %v", f, err)
		}
	}
}

// Tam anahtar 1 ile metin anahtar "1" JSON'da aynı anahtardır: biri sessizce atılmaz.
func TestLookalikeKeyError(t *testing.T) {
	d := interp.NewDictVal()
	d.SetValue(interp.NewInt(1), interp.NewInt(10))
	d.Set("1", interp.NewInt(20))
	if _, err := Write(d); err == nil || !strings.Contains(err.Error(), "görünüşü aynı") {
		t.Fatalf("çakışma hatası bekleniyordu: %v", err)
	}
	single := interp.NewDictVal()
	single.SetValue(interp.NewInt(1), interp.NewInt(10))
	if b, err := Write(single); err != nil || string(b) != `{"1":10}` {
		t.Fatalf("%s %v", b, err)
	}
}

// goValue bir TR değerini encoding/json'un yazabileceği Go değerine çevirir (yalnızca bu
// testin karşılaştırması için; döngü denetimi yapmaz).
func goValue(v interp.Value) any {
	switch x := v.(type) {
	case *interp.NullVal:
		return nil
	case *interp.BoolVal:
		return x.Val
	case *interp.IntVal:
		return x.Val
	case *interp.FloatVal:
		return x.Val
	case *interp.StringVal:
		return x.Val
	case *interp.ListVal:
		out := make([]any, len(x.Elements))
		for i, e := range x.Elements {
			out[i] = goValue(e)
		}
		return out
	case *interp.DictVal:
		out := make(map[string]any, len(x.Pairs))
		for k, e := range x.Pairs {
			out[x.DisplayKey(k)] = goValue(e)
		}
		return out
	}
	return v.Inspect()
}

// Sayı dışı değerler yalnızca çağıran açıkça isterse (NonFiniteAsText) TR yazımıyla metin olarak
// yazılır; varsayılan yazım hatadır (TestNaNAndInfError), böylece geçersiz JSON çıkmaz.
func TestNonFiniteAsTextOnlyWhenAsked(t *testing.T) {
	d := dict("a", &interp.FloatVal{Val: math.NaN()}, "b", &interp.FloatVal{Val: math.Inf(1)},
		"c", &interp.FloatVal{Val: math.Inf(-1)}, "d", &interp.FloatVal{Val: 1.5})
	b, err := WriteOptions(d, Options{NonFiniteAsText: true})
	if err != nil || string(b) != `{"a":"NaN","b":"+Sonsuz","c":"-Sonsuz","d":1.5}` {
		t.Fatalf("%s %v", b, err)
	}
	if _, err := WriteOptions(d, Options{}); err == nil {
		t.Fatal("seçenek verilmeyince NaN ve sonsuz hata olmalı")
	}
}
