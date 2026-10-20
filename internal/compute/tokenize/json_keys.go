package tokenize

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// encoding/json nesne anahtarlarını alan adlarıyla büyük/küçük harf duyarsız eşler ("post_proCessor"
// post_processor sayılır) ve yinelenen anahtarda sonuncusunu alır. Başvuru uygulaması ikisinde de
// farklı davranır (yanlış harfli anahtarı tanımaz, yinelenen alanı reddeder); aynı dosya iki yerde
// farklı parçalayıcı kurmasın diye yapıya çözülen her nesne önce burada denetlenir: yapı alanına
// yalnızca harfi harfine aynı anahtar karşılık gelir, aynı alan iki kez yazılamaz.

// exactUnmarshal checkKeys'ten geçen JSON'u target'a çözer.
func exactUnmarshal(raw []byte, target any) error {
	if err := checkKeys(raw, reflect.TypeOf(target)); err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// checkKeys raw'daki, t'nin yapı alanlarına denk gelen nesneleri denetler. json.RawMessage alanları
// atlanır: onlar çözüldükleri yerde ayrıca denetlenir.
func checkKeys(raw []byte, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == rawMessageType || !hasStruct(t) {
		return nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		if raw[0] != '{' {
			return nil // tür uyuşmazlığını json.Unmarshal bildirir
		}
		fields := jsonFields(t)
		seen := map[string]bool{}
		return eachMember(raw, func(key string, value []byte) error {
			f, ok := fields[key]
			if !ok {
				for name := range fields {
					if strings.EqualFold(name, key) {
						return messages.New(messages.H5214, key, name)
					}
				}
				return nil // bilinmeyen alanlara izin verilir
			}
			if seen[key] {
				return messages.New(messages.H5215, key)
			}
			seen[key] = true
			return checkKeys(value, f)
		})
	case reflect.Map:
		if raw[0] != '{' {
			return nil
		}
		return eachMember(raw, func(_ string, value []byte) error { return checkKeys(value, t.Elem()) })
	case reflect.Slice, reflect.Array:
		if raw[0] != '[' {
			return nil
		}
		return eachElement(raw, func(value []byte) error { return checkKeys(value, t.Elem()) })
	}
	return nil
}

// eachMember bir JSON nesnesinin anahtarlarını sırayla, değerleriyle (kopyalanmadan) f'ye verir.
// Bozuk JSON'da sessizce durur (hatayı json.Unmarshal bildirir).
func eachMember(raw []byte, f func(key string, value []byte) error) error {
	i := skipSpace(raw, 1) // '{' sonrası
	for i < len(raw) && raw[i] == '"' {
		end := skipString(raw, i)
		if end < 0 {
			return nil
		}
		key := string(raw[i+1 : end-1])
		if bytes.IndexByte(raw[i:end], '\\') >= 0 && json.Unmarshal(raw[i:end], &key) != nil {
			return nil
		}
		i = skipSpace(raw, end)
		if i >= len(raw) || raw[i] != ':' {
			return nil
		}
		vs := skipSpace(raw, i+1)
		ve := skipValue(raw, vs)
		if ve < 0 {
			return nil
		}
		if err := f(key, raw[vs:ve]); err != nil {
			return err
		}
		if i = skipSpace(raw, ve); i >= len(raw) || raw[i] != ',' {
			return nil
		}
		i = skipSpace(raw, i+1)
	}
	return nil
}

// eachElement bir JSON dizisinin öğelerini sırayla (kopyalanmadan) f'ye verir.
func eachElement(raw []byte, f func(value []byte) error) error {
	i := skipSpace(raw, 1) // '[' sonrası
	for i < len(raw) && raw[i] != ']' {
		end := skipValue(raw, i)
		if end < 0 {
			return nil
		}
		if err := f(raw[i:end]); err != nil {
			return err
		}
		if i = skipSpace(raw, end); i >= len(raw) || raw[i] != ',' {
			return nil
		}
		i = skipSpace(raw, i+1)
	}
	return nil
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	return i
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// skipString b[i]'de ('"') başlayan metnin bittiği konumu (kapanış tırnağından sonrası), metin
// kapanmıyorsa -1 verir.
func skipString(b []byte, i int) int {
	for i++; i < len(b); i++ {
		switch b[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

// skipValue b[i]'de başlayan JSON değerinin bittiği konumu verir (geçersizse -1). Yalnızca
// sınırları bulur; değerin geçerliliğini json.Unmarshal denetler.
func skipValue(b []byte, i int) int {
	depth := 0
	for i < len(b) {
		switch b[i] {
		case '"':
			if i = skipString(b, i); i < 0 || depth == 0 {
				return i
			}
			continue
		case '{', '[':
			depth++
		case '}', ']':
			if depth--; depth < 0 {
				return -1
			} else if depth == 0 {
				return i + 1
			}
		default:
			if depth == 0 { // sayı, true, false, null
				j := i
				for j < len(b) && b[j] != ',' && b[j] != '}' && b[j] != ']' && !isSpace(b[j]) {
					j++
				}
				return j
			}
		}
		i++
	}
	return -1
}

// jsonFields yapının JSON anahtarlarını (etiket ya da alan adı) alan türleriyle verir; gömülü
// etiketsiz yapıların alanları üst yapınınmış gibi eklenir.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" || (!f.IsExported() && !f.Anonymous) {
			continue
		}
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				for k, v := range jsonFields(ft) {
					if _, ok := out[k]; !ok {
						out[k] = v
					}
				}
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}

// hasStruct t'nin içinde (işaretçi, dilim, dizi ve eşlem üzerinden) denetlenecek bir yapı olup
// olmadığını bildirir; olmayan değerler (ör. sözlüğün binlerce girdisi) taranmaz.
func hasStruct(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return t != rawMessageType && hasStruct(t.Elem())
	}
	return t.Kind() == reflect.Struct
}

var rawMessageType = reflect.TypeOf(json.RawMessage(nil))
