package codes

import (
	"bytes"
	"encoding/json"
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

// Her kodun Türkçe bir açıklaması ve bir örneği vardır.
// Açıklama bir-iki cümledir.
func TestEveryCodeHasDescription(t *testing.T) {
	d := Source()
	for _, k := range d.Codes() {
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
	checkSortedOnePerLine(t, data, "tür")
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
	_, _, err = ParseDefinitions([]byte(`{"TR_fn_print": {"tür": "fonksiyon", "bilinmeyen": 1}}`))
	if err == nil || !strings.Contains(err.Error(), "bilinmeyen") {
		t.Errorf("bilinmeyen alan hata vermeli: %v", err)
	}
	_, _, err = ParseDefinitions([]byte(`{"TR_fn_print": {"tür": "fonksiyon", "adlı": [["TR_arg_dim"]]}}`))
	if err == nil {
		t.Error("eksik adlı argüman hata vermeli")
	}
}
