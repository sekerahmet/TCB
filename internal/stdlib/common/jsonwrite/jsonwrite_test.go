package jsonwrite

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// Biçim, alfabetik sıralı sözlükte encoding/json ile bayt bayt aynıdır (girintili ve girintisiz);
// yalnız ondalık sayı 3.2'nin yazımıyladır (TestFloatsInTheLanguagesSpelling).
func TestSameFormatAsEncodingJSON(t *testing.T) {
	d := dict("a", list(interp.NewInt(1), &interp.FloatVal{Val: 0.5}, &interp.FloatVal{Val: 1e-7}, &interp.FloatVal{Val: 1e21}, &interp.FloatVal{Val: 2}),
		"b", dict(), "c", list(), "d", interp.Null, "e", interp.True, "f", &interp.StringVal{Val: "<ş\"\n>"},
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

// Ondalık sayı 3.2'nin en kısa yazımıyla yazılır (13.8) ve geri okununca yine ondalıktır; tam
// sayı ve tam sayı cinsli tensörün öğesi '.0'sızdır. Eskiden encoding/json'un yazımıyla 2.0 "2",
// 1e16 "10000000000000000" yazılıyor, geri okununca tam sayı oluyordu.
func TestFloatsInTheLanguagesSpelling(t *testing.T) {
	floats := []float64{2, 0.5, 1e-7, 1e-5, 1e-4, 1e16, 1e21, 123456789012345680, math.Copysign(0, -1), 1.5e300}
	var elems []interp.Value
	for _, f := range floats {
		elems = append(elems, &interp.FloatVal{Val: f})
	}
	b, err := Write(list(append(elems, interp.NewInt(2))...))
	want := `[2.0,0.5,1e-07,1e-05,0.0001,1e+16,1e+21,1.2345678901234568e+17,-0.0,1.5e+300,2]`
	if err != nil || string(b) != want {
		t.Fatalf("%s %v\nbeklenen %s", b, err, want)
	}
	back, err := interp.JSONToTR(b)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range back.(*interp.ListVal).Elements[:len(floats)] {
		if f, ok := e.(*interp.FloatVal); !ok || f.Val != floats[i] {
			t.Errorf("%d. öğe geri okununca ondalık %v olmalı: %s", i, floats[i], e.Inspect())
		}
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

// Anahtarı metin olmayan sözlük hatadır (13.8): JSON'da anahtar metindir, geri okununca tipi
// değişirdi (eskiden 1 sessizce "1" yazılıyordu). İleti anahtarı, tipini ve çeviren yazımı söyler.
func TestNonTextKeyIsAnError(t *testing.T) {
	for _, key := range []interp.Value{interp.NewInt(1), interp.True, &interp.TupleVal{Elements: []interp.Value{interp.NewInt(1), interp.NewInt(2)}}} {
		d := interp.NewDictVal()
		d.Set("a", interp.NewInt(0))
		d.SetValue(key, interp.NewInt(10))
		_, err := Write(list(d))
		if !messages.Has(err, messages.H7012) {
			t.Fatalf("%s: H7012 bekleniyordu: %v", key.Inspect(), err)
		}
		if !strings.Contains(err.Error(), key.Inspect()) || !strings.Contains(err.Error(), "`TR_for k, v TR_in d { m[TR_String(k)] = v }`") {
			t.Errorf("ileti anahtarı ve çeviren yazımı söylemeli: %v", err)
		}
	}
	// NUL ile başlayan anahtar metindir, sözlükte kodlanmış dursa da (interp.DictKey): özgün
	// metniyle yazılır, geri okununca aynı anahtardır.
	d := interp.NewDictVal()
	d.Set("\x00a", interp.NewInt(1))
	got, err := Write(d)
	if want := `{"\u0000a":1}`; err != nil || string(got) != want {
		t.Fatalf("Write = %s, %v; %s bekleniyordu", got, err, want)
	}
	back, err := interp.JSONToTR(got)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := back.(*interp.DictVal).GetValue(&interp.StringVal{Val: "\x00a"}); !ok {
		t.Errorf("anahtar geri okunamadı: %s", back.Inspect())
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
		return json.RawMessage(interp.FloatText(x.Val))
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
			out[x.KeyText(k)] = goValue(e)
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
	if err != nil || string(b) != `{"a":"NaN","b":"Infinity","c":"-Infinity","d":1.5}` {
		t.Fatalf("%s %v", b, err)
	}
	if _, err := WriteOptions(d, Options{}); err == nil {
		t.Fatal("seçenek verilmeyince NaN ve sonsuz hata olmalı")
	}
}

// Veri olmayan değer (fonksiyon, yerleşik, aralık, yapı nesnesi) JSON'a yazılmaz, tipini söyleyen
// hata verir (H7010; Python'un json.dumps'ı TypeError verir). Eskiden görüntü metniyle yazılıyor,
// JSON baytları çalıştıranın diline (TR_LANG) bağlı oluyordu.
func TestNonDataValueIsAnError(t *testing.T) {
	for _, v := range []interp.Value{&interp.FuncVal{Name: "f"}, &interp.BuiltinVal{Name: "yazdır"}, &interp.RangeVal{Start: 0, End: 3}} {
		d := interp.NewDictVal()
		d.Set("a", v)
		if _, err := Write(d); !messages.Has(err, messages.H7010) {
			t.Errorf("%s: H7010 bekleniyordu: %v", v.Type(), err)
		}
	}
}

// Süzgeç (TextFilter) TR_JSON değerinin içindeki metinlere ve anahtarlara da uygulanır (19.4: olayın
// ve kaydın içindeki gizli değer TR_JSON'da da ***); sayının tam ya da ondalık yazımı aynı kalır.
// Süzgeçsiz yazım TR_JSON'un kendi metnidir.
func TestTextFilterReachesInsideJSON(t *testing.T) {
	inner, err := interp.JSONToTR([]byte(`{"gizli": ["gizli", 2, 2.0, null, true]}`))
	if err != nil {
		t.Fatal(err)
	}
	v := dict("j", interp.NewJSON(inner))
	mask := func(s string) string { return strings.ReplaceAll(s, "gizli", "***") }
	b, err := WriteOptions(v, Options{TextFilter: mask})
	if want := `{"j":{"***":["***",2,2.0,null,true]}}`; err != nil || string(b) != want {
		t.Fatalf("süzgeçle: %s %v, beklenen %s", b, err, want)
	}
	b, err = Write(v)
	if want := `{"j":{"gizli":["gizli",2,2.0,null,true]}}`; err != nil || string(b) != want {
		t.Fatalf("süzgeçsiz: %s %v, beklenen %s", b, err, want)
	}
}
