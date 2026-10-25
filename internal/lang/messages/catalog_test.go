package messages

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Gömülü kataloglar geçerlidir: her tanımlı kodun Türkçe metni var; her dilin metni yalnızca
// tanımdaki parametreleri ve hepsini kullanır; kodlar artan sırada.
func TestEmbeddedCatalogValid(t *testing.T) {
	d, err := ParseDefinitions(definitionsFile)
	if err != nil {
		t.Fatal(err)
	}
	for lang, data := range languageFiles {
		if _, err := ParseCatalog(lang, data, d); err != nil {
			t.Fatalf("%s.json: %v", lang, err)
		}
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
		{`{a} {c}`, "no such parameter"},
		{`{a} {b`, "unclosed"},
		{`{a|x} {b}`, "unknown format"},
		{`{a} }`, "unmatched"},
		{`{a}`, `parameter "b" is not used`},
	} {
		_, err := ParseCatalog("tr", []byte(`{"H0001": `+quoteJSON(c.text)+`}`), d)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %q içeren hata beklenirdi: %v", c.text, c.want, err)
		}
	}
	// Her dil kodun bütün parametrelerini kullanır: çeviride düşen değer kullanıcıya ulaşmazdı.
	// Kodun çevirisi olmaması ise serbesttir (Türkçeye düşer).
	if _, err := ParseCatalog("en", []byte(`{"H0001": "{a}"}`), d); err == nil || !strings.Contains(err.Error(), "is not used") {
		t.Errorf("öbür dilde eksik parametre hata vermeli: %v", err)
	}
	if _, err := ParseCatalog("en", []byte(`{}`), d); err != nil {
		t.Errorf("çevirisi olmayan kod serbest olmalı: %v", err)
	}
	if _, err := ParseCatalog("en", []byte(`{"H0002": "x"}`), d); err == nil {
		t.Error("tanımsız kod hata vermeli")
	}
	if _, err := ParseCatalog("tr", []byte(`{}`), d); err == nil || !strings.Contains(err.Error(), "no Turkish text") {
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
	"H2006 H2010":       "seçenek listesinin son ve ikili kalıbı (İngilizcede 'a, b, or c' ve 'a or b' ayrıdır)",
	"H6123 H8247":       "ml hata bağlamındaki tensör etiketi ve profil raporu satırı",
	"H7310 H8280":       "ilerleme çubuğu etiketi ve profil raporu satırı",
}

// paramName bir parametrenin adını yakalar; biçim (|q, |dat) karşılaştırmada kalır.
var paramName = regexp.MustCompile(`\{[A-Za-z]+(\|[a-z]+)?\}`)

func TestNoDuplicateTexts(t *testing.T) {
	var raw map[string]string
	if err := strictUnmarshal(trFile, &raw); err != nil {
		t.Fatal(err)
	}
	// Parametre adları karşılaştırmaya girmez: "{closest}" ile "{name}" aynı iletidir.
	byText := map[string][]string{}
	for k, v := range raw {
		shape := paramName.ReplaceAllString(v, "{$1}")
		byText[shape] = append(byText[shape], k)
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

// Dosyalar insanlar tarafından düzenlenir: her kod tek bir satırdır ({ ve } ayrı satırlarda,
// ara satırlar virgülle biter); kodların artan sırası dosya yüklenirken denetlenir
// (TestEmbeddedCatalogValid, TestCatalogErrors).
func TestFilesOneCodePerLine(t *testing.T) {
	files := map[string][]byte{"definitions.json": definitionsFile}
	for lang, data := range languageFiles {
		files[lang+".json"] = data
	}
	for name, data := range files {
		lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
		if len(lines) < 3 || strings.TrimSpace(lines[0]) != "{" || strings.TrimSpace(lines[len(lines)-1]) != "}" {
			t.Errorf("%s: dosya { ve } satırlarıyla çevrili olmalı", name)
			continue
		}
		body := lines[1 : len(lines)-1]
		for i, l := range body {
			l = strings.TrimRight(l, "\r")
			last := i == len(body)-1
			if !regexp.MustCompile(`^"H\d{4}": .+$`).MatchString(l) || strings.HasSuffix(l, ",") == last {
				t.Errorf("%s:%d: her kod tek satır olmalı (ve sonuncu dışında virgülle bitmeli): %.60s", name, i+2, l)
			}
		}
	}
}

// Parametre adları İngilizce tanımlayıcıdır: ASCII, küçük harfle başlar, harf, rakam ve `_` içerir
// (`fn`, `path`, `closest`). Ad şablonda `{ad}` olarak geçer; Türkçe harf ya da ayraç taşımaz.
func TestDefinitionParamsAreASCIIIdentifiers(t *testing.T) {
	d, err := ParseDefinitions(definitionsFile)
	if err != nil {
		t.Fatal(err)
	}
	ident := regexp.MustCompile(`^[a-z][A-Za-z0-9_]*$`)
	for _, c := range d.Codes {
		for _, p := range d.Params[c] {
			if !ident.MatchString(p) {
				t.Errorf("%s: parametre adı %q ASCII bir tanımlayıcı olmalı", c, p)
			}
		}
	}
}
