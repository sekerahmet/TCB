package assistant_test

import (
	"encoding/json"
	"fmt"
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

// page sayfanın bütünüdür: index.html, stili ve modüller.
func page(t *testing.T) string {
	return readFile(t, screenPath) + style(t) + pageCode(t)
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
// tür ekler): dört türü alanlarıyla okur, sayı olmayan boy `null`, bilinmeyen uyum boş, metin olmayan dil düşer. Ayarlar
// yalnız takımı kurulu sohbet modelini seçtirir (kurulu ses seçilmez). Kural js/events.js'in iki saf fonksiyonundadır
// (catalogEntries, loadableModels); test onları işaretlerin arasından alıp node ile koşar, node yoksa atlanır.
func TestPageIgnoresUnknownCatalogFieldsAndKinds(t *testing.T) {
	data := `{"entries":[{"id":"a","kind":"language_model","name":"A","installed":true,"downloaded":true,"bytes":5,` +
		`"memory_bytes":7,"fit":"tight","size_bytes":5,"license":{"name":"x"}},` +
		`{"id":"b","kind":"language_model","name":"B","installed":false,"bytes":"9"},` +
		`{"id":"c","kind":"voice","name":"C","installed":true,"languages":["tr",3]},` +
		`{"id":"d","kind":"image_model","name":"D","installed":true},{"kind":"language_model","name":"E"},"bozuk",null,` +
		`{"id":"f","kind":"language_model","installed":"true","fit":"huge"}],"extra":1}`
	output := nodeBlock(t, eventsModule, "Katalog", "const all = catalogEntries(JSON.parse(process.argv[2]));\n"+
		"process.stdout.write(JSON.stringify({all, loadable: loadableModels(all)}));\n", data)
	var got struct{ All, Loadable []map[string]any }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("node output %q: %v", output, err)
	}
	a := `{"bytes":5,"downloaded":true,"fit":"tight","id":"a","installed":true,"kind":"language_model","languages":[],"memory_bytes":7,"name":"A"}`
	want := `[` + a + `,` +
		`{"bytes":null,"downloaded":false,"fit":"","id":"b","installed":false,"kind":"language_model","languages":[],"memory_bytes":null,"name":"B"},` +
		`{"bytes":null,"downloaded":false,"fit":"","id":"c","installed":true,"kind":"voice","languages":["tr"],"memory_bytes":null,"name":"C"},` +
		`{"bytes":null,"downloaded":false,"fit":"","id":"f","installed":false,"kind":"language_model","languages":[],"memory_bytes":null,"name":"f"}]`
	if all, _ := json.Marshal(got.All); string(all) != want {
		t.Errorf("catalogEntries: %s, want %s", all, want)
	}
	if loadable, _ := json.Marshal(got.Loadable); string(loadable) != `[`+a+`]` {
		t.Errorf("loadableModels: %s, want only the installed chat model", loadable)
	}
}

const modelsModule = "tools/assistant/js/models.js"

// nodeBlock bir modülün "// ---------- <ad> ----------" ile "// ---------- <ad> sonu ----------" arasındaki saf kodunu,
// sonuna sürücüyü ekleyip node ile koşar ve çıktısını verir; node yoksa test atlanır.
func nodeBlock(t *testing.T, module, name, driver string, args ...string) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; " + module + " " + name + " koşulmadı")
	}
	_, rest, ok := strings.Cut(readFile(t, module), "// ---------- "+name+" ----------")
	block, _, found := strings.Cut(rest, "// ---------- "+name+" sonu ----------")
	if !ok || !found {
		t.Fatalf("%s: block %q not found", module, name)
	}
	file := filepath.Join(t.TempDir(), "block.js")
	if err := os.WriteFile(file, []byte(block+"\n"+driver), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, append([]string{file}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, output)
	}
	return output
}

// Satır yalnız kendi girdisinin işini gösterir (M tasarımı, defter 5 (b); Ahmet'in M1 denemesi): sohbetin İndir'i
// eksik İngilizce sesi indirirken iki sohbet satırı birden "iniyor" gösteriyordu. Ses kendi satırında iner; kendi
// dosyaları inmiş, takımı kurulu olmayan sohbet satırları yüzdeyi kopyalamaz, inen parçanın adını söyler. Kendi dosyası
// inen sohbet satırı kendi işini gösterir; başka bir dilin parçası sohbet satırını beklemeye almaz.
func TestDownloadRowShowsOnlyItsOwnJob(t *testing.T) {
	catalog := `[{"id":"qwen","kind":"language_model","name":"Qwen","downloaded":true,"installed":false,"languages":[]},` +
		`{"id":"copy","kind":"language_model","name":"Copy","downloaded":true,"installed":false,"languages":[]},` +
		`{"id":"whisper","kind":"transcriber","name":"Whisper","downloaded":true,"installed":true,"languages":["tr","en"]},` +
		`{"id":"tts-tur","kind":"voice","name":"MMS-TTS tur","downloaded":true,"installed":true,"languages":["tr"]},` +
		`{"id":"tts-eng","kind":"voice","name":"MMS-TTS eng","downloaded":false,"installed":false,"languages":["en"]},` +
		`{"id":"vad","kind":"vad","name":"VAD","downloaded":true,"installed":true,"languages":[]}]`
	cases := []struct{ name, downloads, catalog, want string }{
		{"the English voice downloads for the chat models",
			`{"tts-eng":{"job_id":"d2","entry_id":"tts-eng","state":"running"}}`, catalog,
			`{"copy":"part MMS-TTS eng","qwen":"part MMS-TTS eng","tts-eng":"running","tts-tur":"downloaded","vad":"downloaded","whisper":"downloaded"}`},
		{"the chat model's own file downloads first, the part waits in the queue",
			`{"copy":{"job_id":"d1","entry_id":"copy","state":"running"},"tts-eng":{"job_id":"d2","entry_id":"tts-eng","state":"queued"}}`,
			strings.Replace(catalog, `"name":"Copy","downloaded":true`, `"name":"Copy","downloaded":false`, 1),
			`{"copy":"running","qwen":"part MMS-TTS eng","tts-eng":"queued","tts-tur":"downloaded","vad":"downloaded","whisper":"downloaded"}`},
		{"a part of another language does not hold the chat rows",
			`{"tts-tur":{"job_id":"d3","entry_id":"tts-tur","state":"running"}}`, catalog,
			`{"copy":"downloaded","qwen":"downloaded","tts-eng":"absent","tts-tur":"running","vad":"downloaded","whisper":"downloaded"}`},
	}
	for _, c := range cases {
		output := nodeBlock(t, modelsModule, "Satırın işi", "const catalog = JSON.parse(process.argv[2]);\n"+
			"const downloads = JSON.parse(process.argv[3]);\nconst out = {};\n"+
			"for (const e of catalog) { const s = rowStatus(e, catalog, downloads, \"en\"); out[e.id] = s.part ? `part ${s.part}` : s.kind; }\n"+
			"process.stdout.write(JSON.stringify(out));\n", c.catalog, c.downloads)
		var got map[string]string
		if err := json.Unmarshal(output, &got); err != nil {
			t.Fatalf("%s: node output %q: %v", c.name, output, err)
		}
		if all, _ := json.Marshal(got); string(all) != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, all, c.want)
		}
	}
	// Satırı çizen kod durumu yalnız rowStatus'tan alır; D'nin takımın işlerini toplayan rowJobs'u yoktur.
	code := pageCode(t)
	if !strings.Contains(code, "const status = rowStatus(entry, ui.catalog, ui.downloads, str(ui.settings.speech_language));") || strings.Contains(code, "rowJobs") {
		t.Error("modelRow does not take the row's state from rowStatus alone")
	}
	if !strings.Contains(code, `note("model_part_downloading", { model: status.part })`) {
		t.Error("a waiting chat row does not say model_part_downloading with the part's name")
	}
}

// İnen satır yüzdenin yanında hızı (`download` olayının `bytes_per_second`'i) gösterir; son 30 sn'nin hızıyla kalan süre
// 1 saati aşarsa ya da 30 sn'dir bayt gelmediyse "yavaş" uyarısı çıkar (M tasarımı K11; eşikler tahmindir). Pencere
// 30 sn dolmadan karar verilmez. slowDownload saf fonksiyondur; test onu node ile koşar.
func TestDownloadRowShowsSpeedAndSlow(t *testing.T) {
	const now = 1_000_000
	cases := []struct {
		name, samples string
		total         int
		want          bool
	}{
		{"no samples", `[]`, 100, false},
		{"the window is not full yet", `[{"at":990000,"done":0},{"at":999000,"done":0}]`, 100, false},
		{"no bytes for 30 s", `[{"at":969000,"done":100},{"at":999000,"done":100}]`, 1000, true},
		{"fast enough", `[{"at":969000,"done":0},{"at":999000,"done":30000000}]`, 60000000, false},
		{"more than an hour left", `[{"at":969000,"done":0},{"at":999000,"done":1000000}]`, 10000000000, true},
	}
	for _, c := range cases {
		output := nodeBlock(t, modelsModule, "Satırın işi", fmt.Sprintf("process.stdout.write(String(slowDownload(JSON.parse(process.argv[2]), %d, %d)));\n", c.total, now), c.samples)
		if got := string(output); got != fmt.Sprint(c.want) {
			t.Errorf("%s: slowDownload = %s, want %v", c.name, got, c.want)
		}
	}
	code := pageCode(t)
	for _, want := range []string{
		`bind(el("span"), "download_speed", { speed_bytes: speed })`,
		`const speed = num(status.job.bytes_per_second);`,
		`slow = note("download_slow"`,
		"noteSample(d);",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("the page has no %q: the running row shows its speed and the slow note", want)
		}
	}
}
