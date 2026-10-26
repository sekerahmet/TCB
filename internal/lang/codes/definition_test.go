package codes

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// definitions.json dil dosyasıyla tutarlıdır: her kodun tanımı var, türler ailelere uygun,
// imzalardaki tip ve adlı argüman kodları geçerli.
func TestDefinitionsConsistent(t *testing.T) {
	for _, err := range CheckDefinitions(Source()) {
		t.Error(err)
	}
}

// Kural (17, dosya biçimi): tanım ve dil dosyalarının alan adları ve "kind" değerleri Go adları
// gibi İngilizce tanımlayıcıdır (küçük ASCII harf, rakam, alt çizgi); bir insan dili yalnız
// değerlerde (adlar, açıklamalar, örnekler) görünür. Kimlik olan anahtarlar (TR_…) veridir.
func TestFileFieldNamesAreEnglishIdentifiers(t *testing.T) {
	ident := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	var walk func(file string, v any)
	walk = func(file string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if !strings.HasPrefix(k, "TR_") && !ident.MatchString(k) {
					t.Errorf("%s: alan adı %q İngilizce tanımlayıcı değil", file, k)
				}
				walk(file, e)
			}
		case []any:
			for _, e := range x {
				walk(file, e)
			}
		}
	}
	for file, data := range map[string][]byte{"definitions.json": definitionsJSON, "tr.json": trJSON, "en.json": enJSON} {
		var v map[string]any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(file, err)
		}
		walk(file, v)
	}
	for _, k := range DefinitionCodes() {
		if d, _ := DefinitionOf(k); !ident.MatchString(string(d.Kind)) {
			t.Errorf("%s: kind %q İngilizce tanımlayıcı değil", k, d.Kind)
		}
	}
}

// Her kodun Türkçe bir açıklaması ve bir örneği vardır.
// Açıklama bir-iki cümledir.
func TestEveryCodeHasDescription(t *testing.T) {
	d := Source()
	for _, k := range d.Codes() {
		if k.IsLibrary() {
			continue // kitaplık kodlarının açıklamaları madde 15 adım 3c'de yazılır; atlama 3c'de kalkar
		}
		g, _ := d.Entry(k)
		if strings.TrimSpace(g.Description) == "" {
			t.Errorf("%s (%s): açıklama yok", k, g.Name)
		}
		if n := strings.Count(g.Description, ". ") - strings.Count(g.Description, "... ") + 1; n > 3 {
			t.Errorf("%s (%s): açıklama %d cümle; en çok 2-3 cümle olmalı", k, g.Name, n)
		}
		if strings.TrimSpace(g.Example) == "" {
			t.Errorf("%s (%s): örnek yok", k, g.Name)
		}
		if strings.Contains(g.Example, "\n") {
			t.Errorf("%s (%s): örnek tek satır olmalı", k, g.Name)
		}
	}
}

// Dosya insanlar tarafından düzenlenir: kodlar artan sırada, her girdi iki boşluk girintili
// tek bir satırdır.
func TestDefinitionsSortedOnePerLine(t *testing.T) {
	data := DefinitionData()
	if !json.Valid(data) {
		t.Fatal("definitions.json geçerli JSON değil")
	}
	checkSortedOnePerLine(t, data, "kind")
}

// checkSortedOnePerLine kod sözlüğü dosyalarının (definitions.json, tr.json) biçimini sınar:
// kodlar sınıf sırasıyla (Classes), sınıf içinde kimliğe göre, her girdi iki boşluk girintili tek
// satır ve satırda requiredField bir kez.
func checkSortedOnePerLine(t *testing.T, data []byte, requiredField string) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	order := map[Class]int{}
	for i, c := range Classes {
		order[c.Class] = i
	}
	prevClass, prevID := -1, ""
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		id := tk.(string)
		cls, ok := order[IdentityClass(id)]
		if !ok {
			t.Fatalf("%q bir kimlik değil (TR_<sınıf>_<ad>)", id)
		}
		if cls < prevClass || cls == prevClass && id <= prevID {
			t.Errorf("%s, %s'den sonra geliyor; kodlar sınıf sırasıyla, sınıf içinde kimliğe göre dizilir", id, prevID)
		}
		prevClass, prevID = cls, id
		var g json.RawMessage
		if err := dec.Decode(&g); err != nil {
			t.Fatal(err)
		}
	}
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		s := strings.TrimSpace(line)
		if s == "{" || s == "}" {
			continue
		}
		if !strings.HasPrefix(line, "  \"") || strings.Count(s, "\""+requiredField+"\":") != 1 {
			t.Errorf("satır %d: her girdi iki boşluk girintili tek satır olmalı: %s", i+1, line)
		}
	}
}

// Tip ifadeleri çözülür; geçersiz bir tanım satırı hatayla bildirilir.
func TestParseDefinitions(t *testing.T) {
	ti, err := ParseType("TR_type_Int|TR_type_Float?", true)
	if err != nil || !ti.Optional || len(ti.Types) != 2 || ti.Types[1] != TypeFloat {
		t.Fatalf("TipCoz: %+v %v", ti, err)
	}
	if _, err := ParseType("TR_type_Int?", false); err == nil {
		t.Error("dönüş tipinde sonek hata olmalı")
	}
	_, _, err = ParseDefinitions([]byte(`{"TR_fn_print": {"kind": "function", "bilinmeyen": 1}}`))
	if err == nil || !strings.Contains(err.Error(), "bilinmeyen") {
		t.Errorf("bilinmeyen alan hata vermeli: %v", err)
	}
	_, _, err = ParseDefinitions([]byte(`{"TR_fn_print": {"kind": "function", "named": [["TR_arg_dim"]]}}`))
	if err == nil {
		t.Error("eksik adlı argüman hata vermeli")
	}
}

// Kural (17, kitaplık kodu): definitions.json'da "module" alanı olan kod standart kitaplığın
// (tr/…) kodudur; yerleşik değildir. Alan yalnız kitaplığın dışa açık adlarının sınıflarında
// (tip, fonksiyon, sabit, üye, adlı argüman) yazılır.
func TestLibraryCodesAreNotBuiltins(t *testing.T) {
	defs, _, err := ParseDefinitions([]byte(`{"TR_fn_sin": {"kind": "function", "module": "tr/deneme"}}`))
	if err != nil || defs[FnSin].Module != "tr/deneme" {
		t.Fatalf("modül alanı okunmalı: %+v %v", defs[FnSin], err)
	}
	for _, c := range []struct{ json, want string }{
		{`{"TR_kw_if": {"kind": "keyword", "module": "tr/deneme"}}`, "TR_kw_if: a module is written only on"},
		{`{"TR_fn_sin": {"kind": "function", "module": "deneme"}}`, "TR_fn_sin: module \"deneme\" is not a standard library module"},
	} {
		if _, _, err := ParseDefinitions([]byte(c.json)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu, alınan: %v", c.json, c.want, err)
		}
	}
	t.Cleanup(SetLibraryModulesForTest(map[Code]string{FnSin: "tr/deneme"}))
	if !FnSin.IsLibrary() || FnSin.IsBuiltin() {
		t.Errorf("kitaplık kodu: IsLibrary %v, IsBuiltin %v", FnSin.IsLibrary(), FnSin.IsBuiltin())
	}
	if FnPrint.IsLibrary() || !FnPrint.IsBuiltin() {
		t.Errorf("çekirdek kodu: IsLibrary %v, IsBuiltin %v", FnPrint.IsLibrary(), FnPrint.IsBuiltin())
	}
}

// Kural (17, kitaplık kodu): kitaplık kodunun imzası ve üyenin tipleri yalnız kitaplığın kaynağında
// yazılır; definitions.json'da yalnız türü ve modülü vardır (ikinci kopya yok).
func TestLibraryCodeHasNoSignatureInDefinitions(t *testing.T) {
	t.Cleanup(SetLibraryModulesForTest(map[Code]string{MemStep: "tr/deneme", FnSum: "tr/deneme"}))
	var got []string
	for _, err := range CheckDefinitions(Active()) {
		got = append(got, err.Error())
	}
	for _, k := range []Code{MemStep, FnSum} {
		want := k.String() + ": a library code's signature and types are written only in the library's source"
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("%q bekleniyordu, alınan: %q", want, got)
		}
	}
}
