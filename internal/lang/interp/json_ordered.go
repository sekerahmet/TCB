package interp

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// maxJSONDepth iç içe JSON nesne/dizi derinliğinin üst sınırıdır. JSON dışarıdan (dosyadan,
// kullanıcıdan) gelebildiği için sınırsız derinlik yığın taşmasına yol açabilirdi.
const maxJSONDepth = 1000

// JSONToTR converts JSON text to a TR value. Unlike Go's map-based decoding it keeps the key
// order of objects: json_çöz("{\"b\": 1, \"a\": 2}") returns a dict ordered b, a.
func JSONToTR(data []byte) (Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	val, err := jsonValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, messages.New(messages.H2499)
	}
	return val, nil
}

func jsonValue(dec *json.Decoder, depth int) (Value, error) {
	if depth > maxJSONDepth {
		return nil, messages.New(messages.H2500, maxJSONDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			d := NewDictVal()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, messages.New(messages.H2501)
				}
				val, err := jsonValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				d.Set(key, val)
			}
			if _, err := dec.Token(); err != nil { // '}'
				return nil, err
			}
			return d, nil
		case '[':
			l := &ListVal{Elements: []Value{}}
			for dec.More() {
				val, err := jsonValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				l.Elements = append(l.Elements, val)
			}
			if _, err := dec.Token(); err != nil { // ']'
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
		if err != nil {
			return nil, messages.New(messages.H2503, t)
		}
		// Önceki davranışla uyumlu: tam sayı değerli ondalıklar Tam olur (GoToTRValue gibi)
		return GoToTRValue(f), nil
	case bool:
		return BoolToVal(t), nil
	case nil:
		return NULL, nil
	}
	return nil, messages.New(messages.H2504, tok)
}
