package interp

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// maxJSONDepth iç içe JSON nesne/dizi derinliğinin üst sınırıdır (3.7a): en çok 1000 kap iç içe
// açılır, içteki değer ne olursa olsun. JSON dışarıdan (dosyadan, kullanıcıdan) gelebildiği için
// sınırsız derinlik yığın taşmasına yol açabilirdi.
const maxJSONDepth = 1000

// jsonDepthError, depth kabın içinde yeni bir kap açılırken sınırı aşıyorsa hatadır; depth açılan
// kabı saran kapların sayısıdır.
func jsonDepthError(depth int) error {
	if depth >= maxJSONDepth {
		return messages.New(messages.H2500, maxJSONDepth)
	}
	return nil
}

// JSONToTR converts JSON text to a TR value made of core values (JSONVal holds it). Unlike Go's
// map-based decoding it keeps the key order of objects: json_çöz("{\"b\": 1, \"a\": 2}") gives b, a.
// A number keeps its spelling (3.7a): an integer written without '.' or exponent that fits Int is an
// Int, every other number a Float (2.0 stays a Float, a larger integer is a Float). A key written
// twice in one object is an error that names the key and both places (3.7a, 13.8).
func JSONToTR(data []byte) (Value, error) {
	p := jsonParser{dec: json.NewDecoder(bytes.NewReader(data)), data: data}
	p.dec.UseNumber()
	val, err := p.value(0)
	if err != nil {
		return nil, err
	}
	if _, err := p.dec.Token(); err != io.EOF {
		return nil, messages.New(messages.H2499)
	}
	return val, nil
}

// jsonParser JSON metnini belirteç belirteç okur; data yinelenen anahtarın yerini söylemek içindir.
type jsonParser struct {
	dec  *json.Decoder
	data []byte
}

// value sıradaki değeri okur; depth onu saran kapların sayısıdır.
func (p *jsonParser) value(depth int) (Value, error) {
	tok, err := p.dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if err := jsonDepthError(depth); err != nil {
			return nil, err
		}
		switch t {
		case '{':
			return p.object(depth)
		case '[':
			l := &ListVal{Elements: []Value{}}
			for p.dec.More() {
				val, err := p.value(depth + 1)
				if err != nil {
					return nil, err
				}
				l.Elements = append(l.Elements, val)
			}
			if _, err := p.dec.Token(); err != nil { // ']'
				return nil, err
			}
			return l, nil
		}
		return nil, messages.New(messages.H2502, t)
	case string:
		return &StringVal{Val: t}, nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return NewInt(i), nil
		}
		f, err := t.Float64()
		if err != nil || math.IsInf(f, 0) {
			return nil, messages.New(messages.H2503, t)
		}
		return &FloatVal{Val: f}, nil
	case bool:
		return BoolToVal(t), nil
	case nil:
		return Null, nil
	}
	return nil, messages.New(messages.H2504, tok)
}

// object bir nesnenin anahtarlarını ve değerlerini okur ('{' okunmuştur). Aynı nesnede ikinci kez
// gelen anahtar hatadır (3.7a): ileti anahtarı ve iki yerini söyler. Anahtarların başladığı yer
// (bayt) sözlüğün sırasıyla tutulur; satır ve sütun ancak hatada hesaplanır.
func (p *jsonParser) object(depth int) (Value, error) {
	d := NewDictVal()
	var starts []int64
	for p.dec.More() {
		start := p.dec.InputOffset()
		kt, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, messages.New(messages.H2501)
		}
		if _, twice := d.Pairs[key]; twice {
			line, column := p.position(starts[slices.Index(d.Order, key)])
			line2, column2 := p.position(start)
			return nil, messages.New(messages.H7011, key, line, column, line2, column2)
		}
		starts = append(starts, start)
		val, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		d.Set(key, val)
	}
	if _, err := p.dec.Token(); err != nil { // '}'
		return nil, err
	}
	return d, nil
}

// position, offset'ten sonra boşluk ve ',' atlanınca gelen ilk karakterin (anahtarın tırnağının)
// 1'den başlayan satırı ve sütunudur; sütun karakter sayar.
func (p *jsonParser) position(offset int64) (line, column int) {
	i := int(offset)
	for i < len(p.data) && bytes.IndexByte([]byte(" \t\r\n,"), p.data[i]) >= 0 {
		i++
	}
	before := p.data[:i]
	line = bytes.Count(before, []byte("\n")) + 1
	return line, utf8.RuneCount(before[bytes.LastIndexByte(before, '\n')+1:]) + 1
}
