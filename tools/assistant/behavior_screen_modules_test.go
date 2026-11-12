package assistant_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Ekranın modülleri (contract.md, "Sunulan dosyalar"; tek tık F1): sayfa satır içi betik taşımaz, betiklerini
// sözleşmenin listesindeki sırayla `js/` altından yükler, dil dosyaları en sondadır. Modüller klasik betiktir (sahte
// kip sayfayı `file:` ile açar, orada ES modülü yüklenmez): üst düzey adları bütün betiklerin ortak küresel
// kapsamındadır. İki betikte aynı ad ya da `window`'un bir adı sessiz bir ezilmedir; burada sınanır.

const moduleDir = "tools/assistant/js"

// moduleNames sözleşmenin modül listesidir, yükleme sırasıyla.
func moduleNames(t *testing.T) []string {
	t.Helper()
	line := capture(t, `Modüller, sayfanın yüklediği sırayla: ([^\n]*?)\. `, section(t, "Güvenlik"))
	names := matches("`js/([a-z_]+)\\.js`", line)
	if len(names) == 0 {
		t.Fatalf("%s lists no modules: %q", contractPath, line)
	}
	return names
}

// moduleFiles modüllerin depodaki yollarıdır, sözleşmenin sırasıyla.
func moduleFiles(t *testing.T) []string {
	var files []string
	for _, name := range moduleNames(t) {
		files = append(files, moduleDir+"/"+name+".js")
	}
	return files
}

// pageCode sayfanın betiğidir: modüllerin sözleşmenin sırasıyla birleşimi.
func pageCode(t *testing.T) string {
	var b strings.Builder
	for _, f := range moduleFiles(t) {
		b.WriteString(readFile(t, f))
	}
	return b.String()
}

// page sayfanın bütünüdür: index.html ve modüller.
func page(t *testing.T) string {
	return readFile(t, screenPath) + pageCode(t)
}

// Sayfanın <script src>'leri sözleşmenin listesidir: modüller o sırayla, sonra dil klasörünün dosyaları ve başka betik
// yok. `js/` klasöründe de listede olmayan dosya yoktur: sunucu onu sunmaz, sayfada da durmaz.
func TestPageLoadsTheContractModulesThenTheLanguageFiles(t *testing.T) {
	html := readFile(t, screenPath)
	var want []string
	for _, name := range moduleNames(t) {
		want = append(want, `<script src="js/`+name+`.js"></script>`)
	}
	langs := languageNames(t)
	sort.Strings(langs)
	var got, gotLangs []string
	for _, tag := range matches(`(<script[^>]*>(?:</script>)?)`, html) {
		if name, ok := strings.CutPrefix(tag, `<script src="lang/`); ok {
			gotLangs = append(gotLangs, strings.TrimSuffix(name, `"></script>`))
			continue
		}
		if len(gotLangs) > 0 {
			t.Errorf("%s comes after a language file: modules come first", tag)
		}
		got = append(got, tag)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s loads\n%s\nthe contract lists\n%s", screenPath, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	sort.Strings(gotLangs)
	if strings.Join(gotLangs, " ") != strings.Join(langs, " ") {
		t.Errorf("%s loads the language files %v, %s has %v", screenPath, gotLangs, languageDir, langs)
	}
	entries, err := reporoot.ReadDir(reporoot.Path(moduleDir))
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, name := range moduleNames(t) {
		listed[name+".js"] = true
	}
	for _, entry := range entries {
		if !listed[entry.Name()] {
			t.Errorf("%s/%s is not in the module list of %s", moduleDir, entry.Name(), contractPath)
		}
	}
}

// topLevel bir klasik betiğin en dıştaki bildirimleridir (girintisiz satırlar).
var topLevel = regexp.MustCompile(`(?m)^(?:async +)?(?:function\*?|const|let|var|class) +([A-Za-z_$][\w$]*)`)

// windowNames `window`'un adlarıdır: üst düzeyde aynı adı bildirmek ya hata verir (`top`, `location`, `document`,
// `window`) ya da tarayıcının adını sessizce gölgeler. Liste kapsamlı değil, ekranın değebileceği adlardır.
var windowNames = []string{"window", "self", "document", "location", "top", "parent", "frames", "name", "status", "length",
	"origin", "opener", "closed", "history", "navigator", "screen", "event", "external", "performance", "crypto",
	"localStorage", "sessionStorage", "indexedDB", "caches", "close", "open", "stop", "focus", "blur", "print", "alert",
	"confirm", "prompt", "postMessage", "fetch", "find", "scroll", "scrollTo", "scrollBy", "getComputedStyle",
	"setTimeout", "setInterval", "clearTimeout", "clearInterval", "requestAnimationFrame", "customElements",
	"visualViewport", "devicePixelRatio", "innerWidth", "innerHeight", "isSecureContext", "onload", "onerror",
	"onmessage", "toolbar", "menubar", "statusbar", "scrollbars", "locationbar", "personalbar"}

// Betikler arasında üst düzey ad tekrarı yok: `function` bildirimi öbürünü sessizce ezer, `const` sayfayı durdurur.
// `window`'un adıyla çakışma da yok. Üst düzey yapı bozma (`const { a } = …`) yazılmaz: adı tek tek görülsün.
func TestModulesShareNoTopLevelName(t *testing.T) {
	files := moduleFiles(t)
	for _, name := range languageNames(t) {
		files = append(files, languageDir+"/"+name)
	}
	window := map[string]bool{}
	for _, name := range windowNames {
		window[name] = true
	}
	where := map[string]string{}
	for _, f := range files {
		text := readFile(t, f)
		if m := regexp.MustCompile(`(?m)^(?:const|let|var) *[{[]`).FindString(text); m != "" {
			t.Errorf("%s destructures at the top level (%q): declare each name on its own", f, m)
		}
		for _, name := range matches(topLevel.String(), text) {
			if other, ok := where[name]; ok {
				t.Errorf("%s is declared at the top level of both %s and %s", name, other, f)
			}
			where[name] = f
			if window[name] {
				t.Errorf("%s declares %s at the top level, a name of window", f, name)
			}
		}
	}
	if len(where) < 50 {
		t.Errorf("only %d top-level names read from the modules: the pattern no longer sees them", len(where))
	}
}

// Ekran `catalog` olayında bilmediği alanı ve bilmediği `kind`'ı yok sayar (sözleşme, `catalog`; sonraki dilim alan ve
// tür ekler); ayar sayfası yalnız kurulu modeli seçtirir. Kural js/events.js'in iki saf fonksiyonundadır
// (catalogEntries, loadableModels); test onları işaretlerin arasından alıp node ile koşar, node yoksa atlanır.
func TestPageIgnoresUnknownCatalogFieldsAndKinds(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; ekranın katalog kuralı koşulmadı")
	}
	_, rest, ok := strings.Cut(readFile(t, eventsModule), "// ---------- Katalog ----------")
	block, _, found := strings.Cut(rest, "// ---------- Katalog sonu ----------")
	if !ok || !found {
		t.Fatalf("%s: catalog block not found", eventsModule)
	}
	data := `{"entries":[{"id":"a","kind":"language_model","name":"A","installed":true,"size_bytes":5,"license":{"name":"x"}},` +
		`{"id":"b","kind":"language_model","name":"B","installed":false},{"id":"c","kind":"voice","name":"C","installed":true},` +
		`{"id":"d","kind":"image_model","name":"D","installed":true},{"kind":"language_model","name":"E"},"bozuk",null,` +
		`{"id":"f","kind":"language_model","installed":"true"}],"extra":1}`
	driver := block + "\nconst all = catalogEntries(JSON.parse(process.argv[2]));\n" +
		"process.stdout.write(JSON.stringify({all, loadable: loadableModels(all)}));\n"
	file := filepath.Join(t.TempDir(), "catalog.js")
	if err := os.WriteFile(file, []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, file, data).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, output)
	}
	var got struct{ All, Loadable []map[string]any }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("node output %q: %v", output, err)
	}
	want := `[{"id":"a","installed":true,"name":"A"},{"id":"b","installed":false,"name":"B"},{"id":"f","installed":false,"name":"f"}]`
	if all, _ := json.Marshal(got.All); string(all) != want {
		t.Errorf("catalogEntries: %s, want %s", all, want)
	}
	if loadable, _ := json.Marshal(got.Loadable); string(loadable) != `[{"id":"a","installed":true,"name":"A"}]` {
		t.Errorf("loadableModels: %s, want only the installed model", loadable)
	}
}
