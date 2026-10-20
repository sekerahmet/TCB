package messages

import (
	"sort"
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

// Aynı Türkçe metin iki koda verilmez (paralel geçişlerde kolayca olur): aynı anlamdaki ileti
// tek koddur. Yalnızca bağlamı farklı, çeviride ayrı karşılık alabilecek kısa etiketler (bir
// tablonun sütun başlığı ile bir paneldeki etiket gibi) gerekçesiyle izinlidir.
var sameTextAllowed = map[string]string{
	"H5505 H5606":       "veri kümesi ve parçalayıcı panellerinin ayrı etiketleri",
	"H6124 H8215":       "ml işlem adı ve profil raporu satırı",
	"H6132 H8274":       "ml hata bağlamı ve profil raporu",
	"H6818 H8142":       "tensör izleme sütunu ve hata ayıklayıcı sütunu",
	"H2301 H7214 H7484": "kullanım tarifindeki parametre adı, komut satırı tipi ve kontrol noktası türü",
	"H2307 H7127":       "kullanım tarifindeki parametre adı ve okuyucu iletisindeki nesne adı",
}

func TestNoDuplicateTexts(t *testing.T) {
	var raw map[string]string
	if err := strictUnmarshal(trFile, &raw); err != nil {
		t.Fatal(err)
	}
	byText := map[string][]string{}
	for k, v := range raw {
		byText[v] = append(byText[v], k)
	}
	seen := map[string]bool{}
	for text, ks := range byText {
		if len(ks) < 2 {
			continue
		}
		sort.Strings(ks)
		key := strings.Join(ks, " ")
		seen[key] = true
		if _, ok := sameTextAllowed[key]; !ok {
			t.Errorf("%s aynı metni taşıyor (%q): tek koda indirin ya da bağlamı farklıysa gerekçesiyle sameTextAllowed'a yazın", key, text)
		}
	}
	for key := range sameTextAllowed {
		if !seen[key] {
			t.Errorf("sameTextAllowed: %s artık yinelenmiyor; kaydı silin", key)
		}
	}
}
