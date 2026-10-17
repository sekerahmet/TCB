package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxJSONDepth iç içe JSON nesne/dizi derinliğinin üst sınırıdır. Web sunucusu dışarıdan
// JSON aldığı için sınırsız derinlik yığın taşmasına yol açabilirdi.
const maxJSONDepth = 1000

// jsonToTR JSON metnini TR değerine çevirir. Go'nun map tabanlı çözümlemesinden farklı olarak
// nesnelerdeki anahtar sırasını korur: json_çöz("{\"b\": 1, \"a\": 2}") sözlüğü b, a sırasıyla döner.
func jsonToTR(data []byte) (Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	val, err := jsonValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("JSON değerinden sonra beklenmeyen veri var")
	}
	return val, nil
}

func jsonValue(dec *json.Decoder, depth int) (Value, error) {
	if depth > maxJSONDepth {
		return nil, fmt.Errorf("JSON çok derin iç içe (en fazla %d seviye)", maxJSONDepth)
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
					return nil, errors.New("JSON nesne anahtarı metin olmalı")
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
		return nil, fmt.Errorf("beklenmeyen JSON işareti %q", t)
	case string:
		return &StringVal{Val: t}, nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return &IntVal{Val: i}, nil
		}
		f, err := t.Float64()
		if err != nil {
			return nil, fmt.Errorf("geçersiz JSON sayısı %q", t)
		}
		// Önceki davranışla uyumlu: tam sayı değerli ondalıklar Tam olur (goToTRValue gibi)
		return goToTRValue(f), nil
	case bool:
		return boolToVal(t), nil
	case nil:
		return NULL, nil
	}
	return nil, fmt.Errorf("beklenmeyen JSON değeri %v", tok)
}
