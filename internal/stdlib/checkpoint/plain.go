package checkpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

var plainTags = strings.Fields("ondalık sözlük")

var tagFloat, tagDict = plainTags[0], plainTags[1]

// maxPlainDepth düz değerlerin iç içe derinliğidir (kendini içeren değerler için de sınır).
const maxPlainDepth = 100

// plainNodeLimit düz bir değerde yazılan en çok öğe sayısıdır (testler küçültür).
var plainNodeLimit = interp.MaxCollectionSize

// writePlain düz bir değeri yazar; yazılamayan bir değer yol ile (izlenen.en_iyi) bildirilir.
func writePlain(v interp.Value) (json.RawMessage, error) { return writePlainFiltered(v, nil) }

// writePlainFiltered writePlain gibidir; metinler (anahtarlar dahil) önce suzgec'ten geçer (gizli
// değer süzgeci). suzgec nil olabilir.
func writePlainFiltered(v interp.Value, filter func(string) string) (json.RawMessage, error) {
	y := plainWriter{filter: filter}
	if err := y.write(v, "", 0); err != nil {
		return nil, err
	}
	return y.b.Bytes(), nil
}

type plainWriter struct {
	b      bytes.Buffer
	filter func(string) string
	walker interp.ContainerWalker // kendini içeren liste/sözlük
	nodes  int                    // yazılan değer sayısı (paylaşılan alt listeler her kullanımda sayılır)
}

func (y *plainWriter) write(v interp.Value, fpath string, depth int) error {
	b := &y.b
	if depth > maxPlainDepth {
		return messages.New(messages.H7454, pathName(fpath), maxPlainDepth)
	}
	// Aynı alt listeyi birçok kez içeren değer (l = [l, l] 60 kez) kısa olsa da yazılırken
	// üstel büyür: yazılan değer sayısı sınırlıdır.
	if y.nodes++; y.nodes > plainNodeLimit {
		return messages.New(messages.H7455, pathName(fpath), plainNodeLimit)
	}
	switch v.(type) {
	case *interp.ListVal, *interp.DictVal:
		if err := y.walker.Enter(v, depth); err != nil && errors.Is(err, interp.ErrCyclic) {
			return messages.New(messages.H7456, pathName(fpath))
		}
		defer y.walker.Leave(v)
	}
	switch x := v.(type) {
	case *interp.NullVal:
		b.WriteString("null")
	case *interp.BoolVal:
		b.WriteString(strconv.FormatBool(x.Val))
	case *interp.IntVal:
		b.WriteString(strconv.FormatInt(x.Val, 10))
	case *interp.FloatVal:
		fmt.Fprintf(b, `{%q:%q}`, tagFloat, floatText(x.Val))
	case *interp.StringVal:
		m := x.Val
		if y.filter != nil {
			m = y.filter(m)
		}
		s, _ := json.Marshal(m)
		b.Write(s)
	case *interp.ListVal:
		b.WriteByte('[')
		for i, el := range x.Elements {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := y.write(el, fpath+"."+strconv.Itoa(i), depth+1); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case *interp.DictVal:
		fmt.Fprintf(b, `{%q:[`, tagDict)
		first := true
		for _, k := range x.Order {
			el, ok := x.Pairs[k]
			if !ok {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.WriteByte('[')
			if err := y.write(x.KeyValue(k), fpath, depth+1); err != nil {
				return err
			}
			b.WriteByte(',')
			if err := y.write(el, fpath+"."+x.DisplayKey(k), depth+1); err != nil {
				return err
			}
			b.WriteByte(']')
		}
		b.WriteString("]}")
	default:
		return messages.New(messages.H7457,
			pathName(fpath), interp.TypeAndSummary(v), codes.Name(codes.A200), codes.Name(codes.A201), codes.Name(codes.A202), codes.Name(codes.A203), codes.Name(codes.A042),
			codes.Name(codes.A205), codes.Name(codes.A206), codes.Name(codes.Y404))
	}
	return nil
}

func pathName(fpath string) string {
	if fpath == "" {
		return messages.Text(messages.H7484)
	}
	return strings.TrimPrefix(fpath, ".")
}

// floatText kayıpsız en kısa yazımdır (strconv.ParseFloat geri okur).
func floatText(f float64) string {
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

// readPlain writePlain ile yazılmış bir değeri okur.
func readPlain(b []byte) (interp.Value, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var raw any
	if err := d.Decode(&raw); err != nil {
		return nil, err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, messages.New(messages.H7420) // fazladan veri (decodeJSON'daki gibi)
	}
	return toPlain(raw, 0)
}

func toPlain(raw any, depth int) (interp.Value, error) {
	if depth > maxPlainDepth {
		return nil, messages.New(messages.H7458, maxPlainDepth)
	}
	switch x := raw.(type) {
	case nil:
		return interp.Null, nil
	case bool:
		return interp.BoolToVal(x), nil
	case string:
		return &interp.StringVal{Val: x}, nil
	case json.Number:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil {
			return nil, messages.New(messages.H7459, string(x))
		}
		return interp.NewInt(n), nil
	case []any:
		l := &interp.ListVal{Elements: make([]interp.Value, 0, len(x))}
		for _, el := range x {
			v, err := toPlain(el, depth+1)
			if err != nil {
				return nil, err
			}
			l.Elements = append(l.Elements, v)
		}
		return l, nil
	case map[string]any:
		return toTagged(x, depth)
	}
	return nil, messages.New(messages.H7460, raw)
}

// toTagged {"ondalık": ...} ve {"sözlük": ...} biçimlerini çevirir.
func toTagged(x map[string]any, depth int) (interp.Value, error) {
	if len(x) != 1 {
		return nil, messages.New(messages.H7461, len(x))
	}
	if s, ok := x[tagFloat]; ok {
		m, ok := s.(string)
		if !ok {
			return nil, messages.New(messages.H7462, tagFloat)
		}
		f, err := strconv.ParseFloat(m, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, messages.New(messages.H7463, m)
		}
		return &interp.FloatVal{Val: f}, nil
	}
	pairs, ok := x[tagDict].([]any)
	if !ok {
		return nil, messages.New(messages.H7464)
	}
	s := interp.NewDictVal()
	for _, c := range pairs {
		kv, ok := c.([]any)
		if !ok || len(kv) != 2 {
			return nil, messages.New(messages.H7465)
		}
		k, err := toPlain(kv[0], depth+1)
		if err != nil {
			return nil, err
		}
		switch k.(type) {
		case *interp.ListVal, *interp.DictVal:
			return nil, messages.New(messages.H7466)
		}
		v, err := toPlain(kv[1], depth+1)
		if err != nil {
			return nil, err
		}
		s.SetValue(k, v)
	}
	return s, nil
}

// refreshInPlace kaydedilen değeri kullanıcının değerine yerinde ve iç içe yazar (20.2):
// Sözlük'ün ve Liste'nin içi değişir, değerin kendisi (kullanıcının değişkeni) aynı kalır; iç
// içe kaplar da aynı yerde aynı türdense yerinde yenilenir, başka değişkenlerin onlara tuttuğu
// bağlar kopmaz.
func refreshInPlace(name string, target, record interp.Value) error {
	switch h := target.(type) {
	case *interp.DictVal:
		k, ok := record.(*interp.DictVal)
		if !ok {
			return messages.New(messages.H7467, name, interp.TypeAndSummary(record), codes.DisplayName(codes.A206))
		}
		refreshDict(h, k)
	case *interp.ListVal:
		k, ok := record.(*interp.ListVal)
		if !ok {
			return messages.New(messages.H7467, name, interp.TypeAndSummary(record), codes.DisplayName(codes.A205))
		}
		refreshList(h, k)
	default:
		return messages.New(messages.H7468, name, interp.TypeAndSummary(target))
	}
	return nil
}

// refreshDict h'nin içini k'ninkiyle değiştirir; aynı anahtarda iki tarafta da aynı türden kap
// varsa h'deki nesne korunup yerinde yenilenir.
func refreshDict(h, k *interp.DictVal) {
	for key, v := range k.Pairs {
		if old, ok := h.Pairs[key]; ok {
			k.Pairs[key] = refreshNested(old, v)
		}
	}
	*h = *k
}

// refreshList h'nin elemanlarını k'ninkilerle değiştirir; aynı sırada aynı türden kaplar
// yerinde yenilenir.
func refreshList(h, k *interp.ListVal) {
	for i, v := range k.Elements {
		if i < len(h.Elements) {
			k.Elements[i] = refreshNested(h.Elements[i], v)
		}
	}
	h.Elements = k.Elements
}

// refreshNested eski değer yeniyle aynı türden bir kapsa onu yerinde yenileyip döndürür, değilse
// yeni değeri döndürür (düz değerler döngüsüzdür ve derinliği sınırlıdır, 20.2).
func refreshNested(old, fresh interp.Value) interp.Value {
	switch o := old.(type) {
	case *interp.DictVal:
		if f, ok := fresh.(*interp.DictVal); ok {
			refreshDict(o, f)
			return o
		}
	case *interp.ListVal:
		if f, ok := fresh.(*interp.ListVal); ok {
			refreshList(o, f)
			return o
		}
	}
	return fresh
}
