// Package jsonwrite TR değerlerini JSON metnine yazan tek yerdir: json_metin, web sunucusunun
// bunu kullanır. Go'nun map tabanlı yazımından farkları:
//   - Sözlük anahtarları sözlükteki sırayla yazılır (json_metin(json_çöz('{"b":1,"a":2}'))
//     '{"b":1,"a":2}' verir).
//   - Döngüsel ya da çok derin değerler yığın taşması yerine Türkçe hata verir.
//   - JSON'da karşılığı olmayan Tanımsız (NaN) ve sonsuz sayılar Türkçe hata verir.
//   - Metin olmayan anahtar görünen hâliyle yazılır (1 → "1"); bu, aynı sözlükteki "1"
//     metin anahtarıyla çakışırsa sessizce biri atılmaz, hata verilir.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package jsonwrite

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Write değeri boşluksuz JSON olarak yazar.
func Write(val interp.Value) ([]byte, error) { return WriteIndent(val, "") }

// WriteIndent değeri her seviyede girinti kadar içeri alarak yazar; girinti boşsa
// boşluksuz yazar.
func WriteIndent(val interp.Value, indent string) ([]byte, error) {
	return WriteOptions(val, Options{Indent: indent})
}

// Options yazımı ayarlar; sıfır değeri Yaz ile aynıdır.
type Options struct {
	// Indent her seviyede eklenen girintidir; boşsa boşluksuz yazılır.
	Indent string
	// NonFiniteAsText Tanımsız (NaN) ve sonsuz sayıları hata yerine veri biçiminin sabit
	// metniyle yazar ("NaN", "Infinity", "-Infinity"; nonFiniteData); eğitim günlüğünde kayıpta beliren NaN
	// gizlenmesin diye.
	NonFiniteAsText bool
	// TextFilter yazılan her metne (sözlük anahtarları dahil) uygulanır, ör. gizli değer süzgeci;
	// nil ise metinler olduğu gibi yazılır.
	TextFilter func(string) string
}

// WriteOptions değeri verilen seçeneklerle yazar.
func WriteOptions(val interp.Value, s Options) ([]byte, error) {
	y := writer{indent: s.Indent, nonFiniteAsText: s.NonFiniteAsText, textFilter: s.TextFilter}
	if err := y.value(val, 0); err != nil {
		return nil, err
	}
	return y.b, nil
}

type writer struct {
	b               []byte
	indent          string
	nonFiniteAsText bool
	textFilter      func(string) string
	walker          interp.ContainerWalker
}

func (y *writer) newline(depth int) {
	if y.indent == "" {
		return
	}
	y.b = append(y.b, '\n')
	for i := 0; i < depth; i++ {
		y.b = append(y.b, y.indent...)
	}
}

func (y *writer) text(s string) {
	if y.textFilter != nil {
		s = y.textFilter(s)
	}
	b, _ := json.Marshal(s) // metin her zaman yazılabilir
	y.b = append(y.b, b...)
}

// nonFiniteData sonlu olmayan sayının veri biçimindeki metnidir (NonFiniteAsText: günlük,
// durum dosyası; 19.5). Veri biçimi sözleşmedir: dilden ve değer adlarından (A110, A111)
// bağımsız, sabit metin. Python'un json.dumps'ı aynı sözcükleri yazar (NaN, Infinity,
// -Infinity) ve float("Infinity") onları geri okur: günlük Python araçlarıyla okunabilir.
func nonFiniteData(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	}
	return "-Infinity"
}

func (y *writer) number(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if y.nonFiniteAsText {
			y.text(nonFiniteData(f))
			return nil
		}
		return messages.New(messages.H7008, (&interp.FloatVal{Val: f}).Text(codes.Display()))
	}
	// encoding/json ile aynı biçim: küçük ve çok büyük sayılarda bilimsel yazım
	format := byte('f')
	if a := math.Abs(f); a != 0 && (a < 1e-6 || a >= 1e21) {
		format = 'e'
	}
	y.b = strconv.AppendFloat(y.b, f, format, -1, 64)
	if format == 'e' {
		// 1e-07 → 1e-7
		n := len(y.b)
		if n >= 4 && y.b[n-4] == 'e' && y.b[n-3] == '-' && y.b[n-2] == '0' {
			y.b[n-2] = y.b[n-1]
			y.b = y.b[:n-1]
		}
	}
	return nil
}

func (y *writer) value(val interp.Value, depth int) error {
	switch v := val.(type) {
	case *interp.NullVal:
		y.b = append(y.b, "null"...)
	case *interp.BoolVal:
		y.b = strconv.AppendBool(y.b, v.Val)
	case *interp.IntVal:
		y.b = strconv.AppendInt(y.b, v.Val, 10)
	case *interp.FloatVal:
		return y.number(v.Val)
	case *interp.StringVal:
		y.text(v.Val)
	case *interp.ListVal:
		if err := y.walker.Enter(v, depth); err != nil {
			return err
		}
		defer y.walker.Leave(v)
		y.b = append(y.b, '[')
		for i, elem := range v.Elements {
			if i > 0 {
				y.b = append(y.b, ',')
			}
			y.newline(depth + 1)
			if err := y.value(elem, depth+1); err != nil {
				return err
			}
		}
		if len(v.Elements) > 0 {
			y.newline(depth)
		}
		y.b = append(y.b, ']')
	case *interp.DictVal:
		if err := y.walker.Enter(v, depth); err != nil {
			return err
		}
		defer y.walker.Leave(v)
		y.b = append(y.b, '{')
		var seen map[string]bool
		if len(v.KeyVals) > 0 {
			seen = make(map[string]bool, len(v.Order))
		}
		first := true
		for _, k := range v.Order {
			elem, ok := v.Pairs[k]
			if !ok {
				continue
			}
			name := v.DisplayKey(k)
			if seen != nil {
				if seen[name] {
					return messages.New(messages.H7009, name)
				}
				seen[name] = true
			}
			if !first {
				y.b = append(y.b, ',')
			}
			first = false
			y.newline(depth + 1)
			y.text(name)
			y.b = append(y.b, ':')
			if y.indent != "" {
				y.b = append(y.b, ' ')
			}
			if err := y.value(elem, depth+1); err != nil {
				return err
			}
		}
		if !first {
			y.newline(depth)
		}
		y.b = append(y.b, '}')
	case interp.PlainValuer:
		// Dış değerler (ör. tensör → iç içe sayı listesi)
		return y.value(v.PlainValue(), depth+1)
	default:
		y.text(val.Inspect())
	}
	return nil
}
