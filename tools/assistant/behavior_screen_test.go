package assistant_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Ekran tek dosyadır ve derlenmez: bu testler onun sınırlarını ve sözleşmeyle bağını korur. Tarayıcı açmazlar;
// davranış tarayıcıda sahte sahnelerle gezilir (README, "Sınamak").

const (
	screenPath   = "tools/assistant/index.html"
	contractPath = "tools/assistant/contract.md"
	readmePath   = "tools/assistant/README.md"
	languageDir  = "tools/assistant/lang"

	maxScreenBytes = 150 * 1024
	testFileName   = "behavior_screen_test.go"
)

func readFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path(rel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n") // tarayıcı da satır sonunu böyle okur
}

func capture(t *testing.T, pattern, text string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("%q not found", pattern)
	}
	return m[1]
}

func script(t *testing.T) string {
	return capture(t, `(?s)<script>(.*?)</script>`, readFile(t, screenPath))
}

func style(t *testing.T) string {
	return capture(t, `(?s)<style>(.*?)</style>`, readFile(t, screenPath))
}

// markup betik ve stil dışındaki HTML'dir.
func markup(t *testing.T) string {
	t.Helper()
	strip := regexp.MustCompile(`(?s)<script\b.*?</script>|<style>.*?</style>`)
	return strip.ReplaceAllString(readFile(t, screenPath), "")
}

// languageFile bir dil dosyasının tek biçimidir: tek atama, sağında tek nesne.
var languageFile = regexp.MustCompile(`(?s)^assistantLanguages\.([a-z]{2,3}) = (\{.*\});\n$`)

// parseLanguage dil dosyasını JSON olarak okur: yalnız metin taşıyan bir nesneden başkası geçmez, içine kod giremez.
func parseLanguage(text string) (string, map[string]string, error) {
	m := languageFile.FindStringSubmatch(text)
	if m == nil {
		return "", nil, errors.New("a language file is one assignment: assistantLanguages.<code> = {...};")
	}
	var table map[string]string
	if err := json.Unmarshal([]byte(m[2]), &table); err != nil {
		return "", nil, err
	}
	return m[1], table, nil
}

// languageNames dil klasöründeki dosyalardır.
func languageNames(t *testing.T) []string {
	t.Helper()
	entries, err := reporoot.ReadDir(reporoot.Path(languageDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func dictionary(t *testing.T) map[string]map[string]string {
	t.Helper()
	languages := map[string]map[string]string{}
	for _, name := range languageNames(t) {
		code, table, err := parseLanguage(readFile(t, languageDir+"/"+name))
		if err != nil {
			t.Fatalf("%s/%s: %v", languageDir, name, err)
		}
		if code+".js" != name {
			t.Errorf("%s/%s fills assistantLanguages.%s", languageDir, name, code)
		}
		languages[code] = table
	}
	return languages
}

// section sözleşmenin bir "## " bölümüdür.
func section(t *testing.T, title string) string {
	t.Helper()
	contract := readFile(t, contractPath)
	_, rest, ok := strings.Cut(contract, "\n## "+title+"\n")
	if !ok {
		t.Fatalf("%s has no section %q", contractPath, title)
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

func matches(pattern, text string) []string {
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1) {
		out = append(out, m[len(m)-1])
	}
	return out
}

func TestScreenStaysSmall(t *testing.T) {
	n := len(readFile(t, screenPath))
	for _, name := range languageNames(t) {
		n += len(readFile(t, languageDir+"/"+name))
	}
	if n >= maxScreenBytes {
		t.Errorf("the screen and its language files are %d bytes, limit %d", n, maxScreenBytes)
	}
}

// Model ve araç çıktısı güvenilmez girdidir: metin yalnız textContent ile basılır.
func TestScreenNeverParsesTextAsMarkup(t *testing.T) {
	code := script(t)
	for _, banned := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser",
		"createContextualFragment", "srcdoc", "eval(", "new Function"} {
		if strings.Contains(code, banned) {
			t.Errorf("%s uses %s", screenPath, banned)
		}
	}
}

// Ekran çevrimdışı açılır: dış betik, yazı tipi, CDN, ağ isteği yok.
func TestScreenHasNoExternalAddress(t *testing.T) {
	html := readFile(t, screenPath)
	for _, address := range matches(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s"'`+"`"+`<>)]*`, html) {
		if !strings.HasPrefix(address, "http://127.0.0.1:") {
			t.Errorf("%s names an outside address: %s", screenPath, address)
		}
	}
	for _, banned := range []string{"<link", "<img", "<iframe", "<object", "<embed", "<audio", "<video", " src=", " href="} {
		if strings.Contains(markup(t), banned) {
			t.Errorf("%s loads something from outside: %s", screenPath, banned)
		}
	}
	for _, banned := range []string{"url(", "@import", "@font-face"} {
		if strings.Contains(style(t), banned) {
			t.Errorf("%s style loads something from outside: %s", screenPath, banned)
		}
	}
	code := script(t)
	calls := matches(`\bfetch\(([^,)]*)`, code)
	if len(calls) != 2 {
		t.Errorf("want the two contract calls to fetch, got %q", calls)
	}
	for _, call := range calls {
		if !strings.HasPrefix(call, "`${base}/") {
			t.Errorf("fetch must use the checked source: %s", call)
		}
	}
	for _, banned := range []string{"XMLHttpRequest", "WebSocket", "EventSource(", "sendBeacon", "importScripts"} {
		if strings.Contains(code, banned) {
			t.Errorf("%s uses %s", screenPath, banned)
		}
	}
}

func TestScreenPolicy(t *testing.T) {
	html := readFile(t, screenPath)
	policy := capture(t, `<meta http-equiv="Content-Security-Policy" content="([^"]*)">`, html)
	sum := func(body string) string {
		digest := sha256.Sum256([]byte(body))
		return "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
	}
	for _, want := range []string{
		"default-src 'none'",
		"script-src http://127.0.0.1:*/lang/ file: " + sum(script(t)),
		"style-src " + sum(style(t)),
		"connect-src http://127.0.0.1:*",
		"base-uri 'none'",
		"form-action 'none'",
	} {
		if !strings.Contains(policy, want) {
			t.Errorf("policy has no %q (a hash changes with every edit):\n%s", want, policy)
		}
	}
	for _, banned := range []string{"unsafe-inline", "unsafe-eval", "frame-ancestors"} {
		if strings.Contains(policy, banned) {
			t.Errorf("policy must not contain %s (frame-ancestors belongs in the response header)", banned)
		}
	}
	if strings.Contains(markup(t), " style=") {
		t.Errorf("%s has a style attribute: the policy blocks it", screenPath)
	}
	if n := len(matches(`(<script>)`, html)); n != 1 {
		t.Errorf("want one inline script, got %d: only one hash is allowed", n)
	}
	// `file:` yerel dosyaları ayıramaz: ekran başlarken bütün betikler kapanır, güvenilmez metin ancak ondan sonra gelir.
	code := script(t)
	seal := strings.Index(code, "sealScripts();")
	if !strings.Contains(code, `policy.content = "script-src 'none'";`) || seal < 0 || seal > strings.LastIndex(code, "client.connect();") {
		t.Errorf("the screen must close script loading before it connects")
	}
}

// Belirteç sayfanın kendi köküne gider: kaynak yalnız http://127.0.0.1:<kapı> olabilir; başka bir kök kabul edilirse
// bir bağlantı belirteci aynı makinedeki başka bir sürece gönderir.
func TestSourceMustBeThePagesOwnOrigin(t *testing.T) {
	plain := capture(t, `(?s)const plain = (.*?);`, script(t))
	if plain != `location.protocol === "http:" && location.hostname === "127.0.0.1" && location.port !== ""` {
		t.Errorf("pageSource must accept only http://127.0.0.1:<port>, got: %s", plain)
	}
	if !strings.Contains(script(t), "return plain ? location.origin : null;") {
		t.Errorf("pageSource must return only the page's own origin")
	}
	if strings.Contains(script(t), "location.search") || strings.Contains(script(t), `get("source")`) {
		t.Errorf("the screen must not take its source from the address: a query needs the token (28.2)")
	}
	if !strings.Contains(section(t, "Kaynak"), "başka kapı reddedilir") {
		t.Errorf("%s does not say that another port is rejected", contractPath)
	}
}

// Dil listesi sayfadaki <script src> satırlarıdır: dil dosyaları ve yalnız onlar. Ana betik önce gelir, sözlüğü o kurar.
func TestLanguageFilesAreTheOnlyOtherScripts(t *testing.T) {
	html := readFile(t, screenPath)
	listed := map[string]bool{}
	for _, tag := range matches(`(<script[^>]+>)`, html) {
		name, ok := strings.CutPrefix(tag, `<script src="lang/`)
		name, closed := strings.CutSuffix(name, `">`)
		if !ok || !closed {
			t.Errorf("only language files may be loaded: %s", tag)
		}
		listed[name] = true
		if strings.Index(html, tag) < strings.Index(html, "<script>") {
			t.Errorf("%s comes before the inline script that sets up the dictionary", tag)
		}
	}
	names := languageNames(t)
	if len(names) != len(listed) {
		t.Errorf("%s has %v, the page lists %v", languageDir, names, listed)
	}
	for _, name := range names {
		if !listed[name] {
			t.Errorf("%s/%s is not in the language list of %s", languageDir, name, screenPath)
		}
	}
}

// Dil dosyası kod taşımaz: JSON olarak okunmayan her şey düşer.
func TestLanguageFilesHoldNoCode(t *testing.T) {
	for _, text := range []string{
		"assistantLanguages.tr = {\"a\": alert(1)};\n",
		"assistantLanguages.tr = {\"a\": `b`};\n",
		"assistantLanguages.tr = {\"a\": \"b\"};\nalert(1);\n",
		"alert(1);\nassistantLanguages.tr = {\"a\": \"b\"};\n",
		"assistantLanguages.tr = {\"a\": {\"b\": \"c\"}};\n",
		"assistantLanguages.tr = {\"a\": 1};\n",
		"assistantLanguages.tr = (() => ({}))();\n",
		"window.other = {\"a\": \"b\"};\n",
	} {
		if _, _, err := parseLanguage(text); err == nil {
			t.Errorf("accepted as a language file: %s", text)
		}
	}
	if len(dictionary(t)) < 2 {
		t.Errorf("want at least two language files")
	}
}

// Görünmez karakter koda yazılmaz, kaçışıyla yazılır: okuyan onu göremez, ekran da onu kötü metinde yakalamaya çalışıyor.
func TestScreenHasNoHiddenCharacters(t *testing.T) {
	hidden := regexp.MustCompile("[\u0000-\u0008\u000b-\u001f\u007f-\u009f\u00ad\u061c\u200b-\u200f\u2028-\u202e\u2060-\u206f\ufeff]")
	files := []string{screenPath}
	for _, name := range languageNames(t) {
		files = append(files, languageDir+"/"+name)
	}
	for _, file := range files {
		if m := hidden.FindString(readFile(t, file)); m != "" {
			t.Errorf("%s contains U+%04X; write it as an escape", file, []rune(m)[0])
		}
	}
}

// Ad tek sabittedir: değiştirmek için tek yer vardır.
func TestScreenNameIsOneConstant(t *testing.T) {
	html := readFile(t, screenPath)
	name := capture(t, `const ASSISTANT_NAME = "([^"]+)";`, html)
	if n := strings.Count(html, name); n != 1 {
		t.Errorf("the name %q appears %d times; it lives only in ASSISTANT_NAME", name, n)
	}
	for _, language := range languageNames(t) {
		if strings.Contains(readFile(t, languageDir+"/"+language), name) {
			t.Errorf("the name %q is in %s; it lives only in ASSISTANT_NAME", name, language)
		}
	}
}

// Ekrandaki her yazı sözlükten gelir: HTML'in kendisinde insan dilinde metin yok, hiçbir dil ayrıcalıklı değil.
func TestScreenMarkupHoldsNoText(t *testing.T) {
	body := capture(t, `(?s)<body>(.*)</body>`, markup(t))
	text := regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(body, "")
	if text = strings.TrimSpace(text); text != "" {
		t.Errorf("text outside the dictionary: %q", text)
	}
	if title := capture(t, `<title>(.*)</title>`, readFile(t, screenPath)); title != "" {
		t.Errorf("title must come from ASSISTANT_NAME, got %q", title)
	}
}

func TestDictionaryLanguagesMatch(t *testing.T) {
	languages := dictionary(t)
	for _, want := range []string{"en", "tr"} {
		if languages[want] == nil {
			t.Fatalf("dictionary has no %s", want)
		}
	}
	placeholder := regexp.MustCompile(`\{\w+\}`)
	holes := func(text string) string {
		found := placeholder.FindAllString(text, -1)
		sort.Strings(found)
		return strings.Join(found, " ")
	}
	base := languages["en"]
	for code, table := range languages {
		if table["language_name"] == "" {
			t.Errorf("%s has no language_name", code)
		}
		if d := table["direction"]; d != "ltr" && d != "rtl" {
			t.Errorf("%s has direction %q", code, d)
		}
		for key, text := range base {
			other, ok := table[key]
			if !ok {
				t.Errorf("%s has no %s", code, key)
				continue
			}
			if holes(other) != holes(text) {
				t.Errorf("%s/%s has placeholders %q, en has %q", code, key, holes(other), holes(text))
			}
		}
		for key := range table {
			if _, ok := base[key]; !ok {
				t.Errorf("%s has %s, en does not", code, key)
			}
		}
	}
}

// dynamicKeys koddaki anahtarların kurulduğu öneklerdir (`state_${state}`).
var dynamicKeys = []string{"conn_", "state_", "result_", "error_", "scene_", "approval_closed_"}

func TestDictionaryKeysAreUsed(t *testing.T) {
	code := script(t)
	table := dictionary(t)["en"]
	used := map[string]bool{}
	for _, key := range matches(`data-i18n="([a-z0-9_]+)"`, markup(t)) {
		used[key] = true
	}
	for _, pattern := range []string{`\bt\("([a-z0-9_]+)"`, `\bnote\("([a-z0-9_]+)"`, `\bbind\(.*?, "([a-z][a-z0-9]*_[a-z0-9_]+)"`} {
		for _, key := range matches(pattern, code) {
			used[key] = true
		}
	}
	for key := range used {
		if _, ok := table[key]; !ok {
			t.Errorf("%q is used but not in the dictionary", key)
		}
	}
	for key := range table {
		named := strings.Contains(code, `"`+key+`"`) || used[key] || key == "language_name" || key == "direction"
		for _, prefix := range dynamicKeys {
			named = named || strings.HasPrefix(key, prefix)
		}
		if !named {
			t.Errorf("dictionary key %q is not used", key)
		}
	}
	_, scenes, _ := strings.Cut(code, "const scenes = {")
	scenes, _, _ = strings.Cut(scenes, "function play(")
	for _, scene := range matches(`(?m)^      async ([a-z_]+)\(`, scenes) {
		if _, ok := table["scene_"+scene]; !ok && scene != "reply" {
			t.Errorf("mock scene %s has no scene_%s text", scene, scene)
		}
	}
}

// Sözleşmedeki her olayı ekran işler ve sahte akış üretir: sahte akışta görülmeyen olay ekranda sınanmamıştır.
func TestEveryContractEventIsInTheMock(t *testing.T) {
	code := script(t)
	events := matches("(?m)^### `([a-z_]+)`$", section(t, "Olaylar"))
	if len(events) < 12 {
		t.Fatalf("only %d events read from %s: %v", len(events), contractPath, events)
	}
	for _, event := range events {
		if !strings.Contains(code, `emit("`+event+`"`) {
			t.Errorf("event %s is in %s but the mock never emits it", event, contractPath)
		}
		if !strings.Contains(code, `on("`+event+`"`) {
			t.Errorf("event %s is in %s but the screen does not handle it", event, contractPath)
		}
	}
	for _, event := range matches(`\bon\("([a-z_]+)"`, code) {
		if !strings.Contains(section(t, "Olaylar"), "### `"+event+"`") {
			t.Errorf("the screen handles %s, which is not in %s", event, contractPath)
		}
	}
}

func TestEveryContractCommandIsSentAndAnswered(t *testing.T) {
	code := script(t)
	commands := matches("(?m)^\\| `([a-z_]+)` \\| `\\{", section(t, "Komutlar"))
	if len(commands) < 4 {
		t.Fatalf("only %d commands read from %s: %v", len(commands), contractPath, commands)
	}
	for _, command := range commands {
		if !regexp.MustCompile(`(command|sendCommand)\("` + command + `"`).MatchString(code) {
			t.Errorf("the screen never sends %s", command)
		}
		if !strings.Contains(code, `case "`+command+`":`) {
			t.Errorf("the mock does not answer %s", command)
		}
	}
}

func TestEveryContractStateHasText(t *testing.T) {
	block := section(t, "Olaylar")
	_, rest, _ := strings.Cut(block, "### `state`")
	line := capture(t, "(?m)^- `state`: (.*)$", rest)
	states := matches("`([a-z]+)`", line)
	if len(states) != 6 {
		t.Fatalf("want six states, read %v", states)
	}
	list := capture(t, `const STATES = \[(.*?)\];`, script(t))
	for code, table := range dictionary(t) {
		for _, state := range states {
			if table["state_"+state] == "" {
				t.Errorf("%s has no state_%s", code, state)
			}
			if !strings.Contains(list, `"`+state+`"`) {
				t.Errorf("STATES has no %s", state)
			}
		}
	}
}

// Hata metin değil kod taşır: her kodun her dilde karşılığı olmalı, yoksa ekran kullanıcıya kod gösterir.
func TestEveryErrorCodeHasText(t *testing.T) {
	codes := matches("(?m)^\\| `([a-z_]+)` \\|", section(t, "Hata kodları"))
	if len(codes) < 20 {
		t.Fatalf("only %d error codes read from %s: %v", len(codes), contractPath, codes)
	}
	languages := dictionary(t)
	for language, table := range languages {
		for _, code := range append(codes, "unknown") {
			if table["error_"+code] == "" {
				t.Errorf("%s has no error_%s", language, code)
			}
		}
	}
	known := map[string]bool{"unknown": true}
	for _, code := range codes {
		known[code] = true
	}
	for key := range languages["en"] {
		if code, ok := strings.CutPrefix(key, "error_"); ok && !known[code] {
			t.Errorf("error_%s is not in %s", code, contractPath)
		}
	}
}

// Her hata kodunun tablodaki değerleri metindeki boşluklarla aynı olmalı: `limit_bytes` geliyorsa ekran onu gösterir.
func TestErrorTextsUseOnlyContractValues(t *testing.T) {
	rows := regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\| ([^|]*) \\|").FindAllStringSubmatch(section(t, "Hata kodları"), -1)
	table := dictionary(t)["en"]
	for _, row := range rows {
		values := map[string]bool{}
		for _, value := range matches("`([a-z_]+)`", row[2]) {
			values[value] = true
		}
		for _, hole := range matches(`\{(\w+)\}`, table["error_"+row[1]]) {
			if !values[hole] {
				t.Errorf("error_%s shows {%s}, which %s does not list", row[1], hole, contractPath)
			}
		}
	}
}

func TestContractSaysTheSecurityRules(t *testing.T) {
	body := section(t, "Güvenlik")
	for _, want := range []string{
		"`Origin`", "`Host`", "`Origin: null`", "`Authorization: Bearer", "`Content-Type`", "`frame-ancestors`", "19.4", "`127.0.0.1`",
		"`Content-Security-Policy: frame-ancestors 'none'; connect-src 'self'`",
		"`error.values`", "`snapshot`", "kullanıcının sözü", "`TR_mask_stream()`", "19.4a", "`.TR_finish()`", "süzme kesmeden önce",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the security section of %s does not mention %s", contractPath, want)
		}
	}
}

func TestReadmeListsThisTest(t *testing.T) {
	if !strings.Contains(readFile(t, readmePath), "- `"+testFileName+"`: ") {
		t.Errorf("%s does not list %s", readmePath, testFileName)
	}
}
