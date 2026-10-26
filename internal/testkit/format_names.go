package testkit

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// formatName dosya biçiminin bir adıdır (JSON anahtarı, kayıt türü, dosya adının parçası):
// dilden bağımsız İngilizce tanımlayıcı, küçük ASCII harf, rakam ve alt çizgi (dil tanımı 17.3).
var formatName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FormatNames her adın dosya biçimine uygun bir tanımlayıcı olduğunu denetler; noktalı yollar
// ("gpu.memory") ve dosya adları ("state.json", "step_") parça parça sınanır.
func FormatNames(t testing.TB, names ...string) {
	t.Helper()
	for _, name := range names {
		parts := strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '-' })
		if len(parts) == 0 {
			t.Errorf("format name %q is empty", name)
		}
		for _, p := range parts {
			if !formatName.MatchString(strings.TrimSuffix(p, "_")) {
				t.Errorf("format name %q: %q is not a language-independent identifier (lowercase ASCII, digits, _)", name, p)
			}
		}
	}
}

// FormatJSONKeys v'nin tipinin (iç içe yapılar, dilimler ve haritaların değerleri dahil) bütün
// JSON anahtarlarını FormatNames ile denetler; etiketsiz dışa açık alan da hatadır (anahtar Go
// adına bağlı kalırdı).
func FormatJSONKeys(t testing.TB, v any) {
	t.Helper()
	seen := map[reflect.Type]bool{}
	var walk func(typ reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			switch name {
			case "-":
				continue
			case "":
				t.Errorf("%s%s has no json key", path, f.Name)
				continue
			}
			FormatNames(t, name)
			walk(f.Type, path+name+".")
		}
	}
	walk(reflect.TypeOf(v), "")
}
