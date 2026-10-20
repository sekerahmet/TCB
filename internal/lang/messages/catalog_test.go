package messages

import (
	"strings"
	"testing"
)

// Gömülü katalog geçerlidir: her tanımlı kodun Türkçe metni var, metinler yalnızca tanımdaki
// parametreleri kullanır, Türkçe metin her parametreyi kullanır, kodlar artan sırada.
func TestEmbeddedCatalogValid(t *testing.T) {
	d, err := ParseDefinitions(definitionsFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCatalog("tr", trFile, d); err != nil {
		t.Fatal(err)
	}
	if len(d.Codes) == 0 || d.Codes[len(d.Codes)-1] != H9000 {
		t.Fatalf("kodlar: %v", d.Codes)
	}
}

// Şablon hataları dosya yüklenirken yakalanır.
func TestCatalogErrors(t *testing.T) {
	defs := []byte(`{
"H0001": { "param": ["a", "b"] }
}`)
	d, err := ParseDefinitions(defs)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ text, want string }{
		{`{a} {c}`, "böyle bir parametre yok"},
		{`{a} {b`, "kapanmamış"},
		{`{a|x} {b}`, "bilinmeyen biçim"},
		{`{a} }`, "eşi olmayan"},
		{`{a}`, `"b" parametresi metinde kullanılmıyor`},
	} {
		_, err := ParseCatalog("tr", []byte(`{"H0001": `+quoteJSON(c.text)+`}`), d)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %q içeren hata beklenirdi: %v", c.text, c.want, err)
		}
	}
	// Başka dilde parametrenin kullanılmaması serbesttir; tanımsız kod değildir.
	if _, err := ParseCatalog("en", []byte(`{"H0001": "{a}"}`), d); err != nil {
		t.Errorf("öbür dilde eksik parametre serbest olmalı: %v", err)
	}
	if _, err := ParseCatalog("en", []byte(`{"H0002": "x"}`), d); err == nil {
		t.Error("tanımsız kod hata vermeli")
	}
	if _, err := ParseCatalog("tr", []byte(`{}`), d); err == nil || !strings.Contains(err.Error(), "Türkçe metni yok") {
		t.Errorf("Türkçe metni olmayan kod hata vermeli: %v", err)
	}
	for _, bad := range []string{
		"{\n\"H0002\": { \"param\": [] },\n\"H0001\": { \"param\": [] }\n}",
		`{"H0001": { "param": ["a", "a"] }}`,
		`{"H0001": { "param": [], "ek": 1 }}`,
		`{"X0001": { "param": [] }}`,
	} {
		if _, err := ParseDefinitions([]byte(bad)); err == nil {
			t.Errorf("geçersiz tanım kabul edildi: %s", bad)
		}
	}
}

// Metin kurulurken {ad|q} Go tırnağıyla (denetim karakterleri kaçırılarak) yazılır, {{ }} düz
// parantezdir, eksik değer {ad?} olarak görünür.
func TestRender(t *testing.T) {
	d, _ := ParseDefinitions([]byte(`{"H0001": { "param": ["fn", "path"] }}`))
	c, err := ParseCatalog("tr", []byte(`{"H0001": "{fn}: {path|q} {{bir}} klasör"}`), d)
	if err != nil {
		t.Fatal(err)
	}
	old := definitions
	definitions = d
	defer func() { definitions = old }()
	if got := c.render(1, []any{"csv_oku", "a\x1b[2Jb"}); got != `csv_oku: "a\x1b[2Jb" {bir} klasör` {
		t.Errorf("metin: %s", got)
	}
	if got := c.render(1, []any{"csv_oku"}); got != `csv_oku: {path?} {bir} klasör` {
		t.Errorf("eksik değer: %s", got)
	}
}

func quoteJSON(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}
