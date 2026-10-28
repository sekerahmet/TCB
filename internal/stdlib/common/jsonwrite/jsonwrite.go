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
	// metniyle yazar ("NaN", "Infinity", "-Infinity"; NonFiniteData); eğitim günlüğünde kayıpta beliren NaN
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

// NonFiniteData is the data-format text of a non-finite number: logs, state files and
// checkpoints (19.5, 20). Data formats are a contract: the text is fixed, independent of the
// language and of the value names (TR_val_inf, TR_val_nan). Python's json.dumps writes the same words
// (NaN, Infinity, -Infinity) and float("Infinity") reads them back.
func NonFiniteData(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	}
	return "-Infinity"
}

// ParseNonFiniteData reads NonFiniteData's text back; ok is false for any other text (other
// spellings of non-finite numbers are not part of the format).
func ParseNonFiniteData(s string) (float64, bool) {
	switch s {
	case "NaN":
		return math.NaN(), true
	case "Infinity":
		return math.Inf(1), true
	case "-Infinity":
		return math.Inf(-1), true
	}
	return 0, false
}

func (y *writer) number(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if y.nonFiniteAsText {
			y.text(NonFiniteData(f))
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
	case *interp.JSONVal:
		// TR_JSON (3.7a): sayının tam ya da ondalık yazımı korunur.
		y.b = append(y.b, v.JSONText()...)
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
		return y.dict(v, depth)
	case interp.PlainValuer:
		// Dış değerler (ör. tensör → iç içe sayı listesi)
		return y.value(v.PlainValue(), depth+1)
	default:
		return notData(val)
	}
	return nil
}

// dict sözlüğü JSON nesnesi olarak yazar: anahtarlar sözlükteki sırayla, dilden bağımsız metinleriyle.
func (y *writer) dict(v *interp.DictVal, depth int) error {
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
		name := v.KeyText(k) // dilden bağımsız (17.3)
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
	return nil
}

// notData veri olmayan değerin (fonksiyon, yerleşik, iyileştirici, yapı nesnesi ...) hatasıdır:
// JSON'a yazılmaz, çünkü metni görüntü ya da dosyanın diline bağlıdır, veri biçimi dilden
// bağımsızdır. Python'un json.dumps'ı da TypeError verir.
func notData(val interp.Value) error {
	return messages.New(messages.H7010, val.Type(), interp.TypeNames(codes.TypeInt, codes.TypeFloat, codes.TypeString, codes.TypeBool,
		codes.TypeNoneType, codes.TypeList, codes.TypeDict, codes.TypeTensor, codes.TypePath))
}
