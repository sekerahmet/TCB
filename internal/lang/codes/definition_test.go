package codes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Taşınan kodlar canlı kodlardan ayrıdır: adları kabul edilmez, yeni koda yol gösterir;
// Go'da sabitleri yoktur, numaraları bir daha kullanılmaz.
func TestMovedCodes(t *testing.T) {
	expected := map[string]string{"G200": "Y001", "G300": "Y200", "G301": "Y201", "G302": "Y207", "G303": "Y202",
		"G304": "Y004", "T120": "Y411", "Y205": "G400", "Y206": "G410"}
	if len(MovedCodes()) != len(expected) {
		t.Errorf("taşınan kodlar %v, %d bekleniyordu", MovedCodes(), len(expected))
	}
	for e, y := range expected {
		extra, _ := Parse(e)
		yk, _ := Parse(y)
		if got, ok := MovedTo(extra); !ok || got != yk {
			t.Errorf("Tasindi(%s) = %s %v; %s bekleniyordu", e, got, ok, y)
		}
		if slices.Contains(Codes(), extra) {
			t.Errorf("%s taşındı; canlı kodlar arasında olmamalı", e)
		}
		name := Name(extra)
		if k, ok := Find(extra.Namespace(), name); ok {
			t.Errorf("taşınan %s'in adı %q kabul edilmemeli (%s)", e, name, k)
		}
		if k, ok := FindDead(extra.Namespace(), name); !ok || k != extra {
			t.Errorf("OluBul(%s, %q) = %s %v", extra.Namespace(), name, k, ok)
		}
	}
	if k, ok := FindDead(NamespaceGlobal, "sirala"); !ok || k.String() != "G303" {
		t.Errorf("ASCII biçim de taşınan koda gitmeli: %s %v", k, ok)
	}
	if k, ok := FindDead(NamespaceGlobal, "boy"); !ok || k.String() != "G200" {
		t.Errorf("taşınan kodun eski adı da taşınan koda gitmeli: %s %v", k, ok)
	}
	// Taşınan bir ad aynı ad alanında canlı bir adla çakışırsa dil dosyası reddedilir.
	_, err := Load("dene", []byte(`{"G100": {"ad": "yazdır"}, "G200": {"ad": "yazdır"}, "Y001": {"ad": "uzunluk"}}`))
	if err == nil || !strings.Contains(err.Error(), "the name \"yazdır\" of taşınmış G200") {
		t.Errorf("çakışma yakalanmalı: %v", err)
	}
}

// Ölü (kaldırılmış ya da taşınmış) kodlar yalnızca kaynak dil dosyasında (TR) zorunludur;
// başka bir dil dosyası onları yazmayabilir. Canlı bir kodun eksikliği her dilde hatadır.
func TestDeadCodesOptionalOutsideTR(t *testing.T) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(TRData(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range DeadCodes() {
		delete(raw, k.String())
	}
	build := func(langCode string, m map[string]json.RawMessage) []error {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Load(langCode, data)
		if err != nil {
			t.Fatalf("%s yüklenmedi: %v", langCode, err)
		}
		return CheckDefinitions(d)
	}
	if errs := build("en", raw); len(errs) != 0 {
		t.Errorf("ölü kodları olmayan başka bir dil dosyası geçmeli: %v", errs)
	}
	if errs := build(SourceLanguage, raw); len(errs) == 0 {
		t.Error("kaynak dil dosyasında ölü kodlar zorunlu olmalı")
	}
	// Ölü bir kodun adı yeniden kullanılırsa ölü kodun adı kodun kendisi yazılır: kod biçimli
	// adlar ayrılmış olduğundan çakışmaz, ipucu düşer.
	if _, err := Load("dene", []byte(`{"G100": {"ad": "boy"}, "G200": {"ad": "G200"}, "Y001": {"ad": "uzunluk"}}`)); err != nil {
		t.Errorf("adı yeniden kullanılan ölü kod yüklenmeli: %v", err)
	}
	if _, err := Load("dene", []byte(`{"G100": {"ad": "G100"}}`)); err == nil || !strings.Contains(err.Error(), "has the code form") {
		t.Errorf("canlı bir kodun adı kod biçiminde olamaz: %v", err)
	}
	delete(raw, "G100")
	if errs := build("en", raw); len(errs) == 0 || !strings.Contains(fmt.Sprint(errs), "G100 is in definitions.json but not in the language file") {
		t.Errorf("canlı kodun eksikliği her dilde hata olmalı: %v", errs)
	}
}

// definitions.json dil dosyasıyla tutarlıdır: her kodun tanımı var, türler ailelere uygun,
// imzalardaki tip ve adlı argüman kodları geçerli, taşınan kodlar canlı bir koda gider.
func TestDefinitionsConsistent(t *testing.T) {
	for _, err := range CheckDefinitions(Active()) {
		t.Error(err)
	}
}

// Her kodun (taşınmışlar dahil) Türkçe bir açıklaması, her canlı kodun bir örneği vardır.
// Açıklama bir-iki cümledir.
func TestEveryCodeHasDescription(t *testing.T) {
	d := Active()
	for _, k := range append(d.Codes(), d.MovedCodes()...) {
		g, _ := d.Entry(k)
		if strings.TrimSpace(g.Description) == "" {
			t.Errorf("%s (%s): açıklama yok", k, g.Name)
		}
		if n := strings.Count(g.Description, ". ") - strings.Count(g.Description, "... ") + 1; n > 3 {
			t.Errorf("%s (%s): açıklama %d cümle; en çok 2-3 cümle olmalı", k, g.Name, n)
		}
		if g.MovedTo == None && strings.TrimSpace(g.Example) == "" {
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
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	var prev Code
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		k, err := Parse(tk.(string))
		if err != nil {
			t.Fatal(err)
		}
		if k <= prev {
			t.Errorf("%s, %s'den sonra geliyor; kodlar artan sırada olmalı", k, prev)
		}
		prev = k
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
		if !strings.HasPrefix(line, "  \"") || strings.Count(s, "\"tür\"") != 1 {
			t.Errorf("satır %d: her girdi iki boşluk girintili tek satır olmalı: %s", i+1, line)
		}
	}
}

// Tip ifadeleri çözülür; geçersiz bir tanım satırı hatayla bildirilir.
func TestParseDefinitions(t *testing.T) {
	ti, err := ParseType("A200|A201?", true)
	if err != nil || !ti.Optional || len(ti.Types) != 2 || ti.Types[1] != A201 {
		t.Fatalf("TipCoz: %+v %v", ti, err)
	}
	if _, err := ParseType("A200?", false); err == nil {
		t.Error("dönüş tipinde sonek hata olmalı")
	}
	_, _, err = ParseDefinitions([]byte(`{"G100": {"tür": "fonksiyon", "bilinmeyen": 1}}`))
	if err == nil || !strings.Contains(err.Error(), "bilinmeyen") {
		t.Errorf("bilinmeyen alan hata vermeli: %v", err)
	}
	_, _, err = ParseDefinitions([]byte(`{"G100": {"tür": "fonksiyon", "adlı": [["T902"]]}}`))
	if err == nil {
		t.Error("eksik adlı argüman hata vermeli")
	}
}
