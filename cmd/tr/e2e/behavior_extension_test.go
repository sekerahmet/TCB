package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// extensionPackage VS Code eklentisinin package.json'ının bu testlerin baktığı kısmıdır.
type extensionPackage struct {
	Capabilities struct {
		UntrustedWorkspaces struct {
			Supported                any      `json:"supported"`
			Description              string   `json:"description"`
			RestrictedConfigurations []string `json:"restrictedConfigurations"`
		} `json:"untrustedWorkspaces"`
	} `json:"capabilities"`
	Contributes struct {
		ConfigurationDefaults map[string]map[string]any `json:"configurationDefaults"`
		Configuration         struct {
			Properties map[string]struct {
				Scope string `json:"scope"`
			} `json:"properties"`
		} `json:"configuration"`
		Menus map[string][]struct {
			Command string `json:"command"`
			When    string `json:"when"`
		} `json:"menus"`
		Debuggers []struct {
			ConfigurationAttributes struct {
				Launch struct {
					Properties map[string]struct {
						Type  string `json:"type"`
						Items struct {
							Type string `json:"type"`
						} `json:"items"`
					} `json:"properties"`
				} `json:"launch"`
			} `json:"configurationAttributes"`
		} `json:"debuggers"`
	} `json:"contributes"`
}

// nlsKeyRe package.json'daki çeviri anahtarıdır ("%komut.calistir%"): VS Code değeri
// package.nls.json'dan (ileride package.nls.<dil>.json'dan) alır.
var nlsKeyRe = regexp.MustCompile(`^%([^%\s]+)%$`)

// readExtensionJSON eklenti klasöründeki bir JSON dosyasını genel biçimde okur.
func readExtensionJSON(t *testing.T, ad string) any {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("editors/vscode/" + ad))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%s geçerli JSON değil: %v", ad, err)
	}
	return v
}

// nlsParse v içindeki her "%anahtar%" metnini çeviri dosyasındaki değerle değiştirir; bulunan
// anahtarları kullanilan'a, bulunamayanları eksik'e yazar.
func nlsParse(v any, translation map[string]any, used map[string]bool, missing *[]string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, d := range x {
			x[k] = nlsParse(d, translation, used, missing)
		}
	case []any:
		for i, d := range x {
			x[i] = nlsParse(d, translation, used, missing)
		}
	case string:
		if m := nlsKeyRe.FindStringSubmatch(x); m != nil {
			c, ok := translation[m[1]].(string)
			if !ok {
				*missing = append(*missing, m[1])
				return x
			}
			used[m[1]] = true
			return c
		}
	}
	return v
}

// readExtensionPackage package.json'ı, çeviri anahtarları package.nls.json'dan çözülmüş olarak okur.
func readExtensionPackage(t *testing.T) extensionPackage {
	t.Helper()
	translation, _ := readExtensionJSON(t, "package.nls.json").(map[string]any)
	var missing []string
	resolved := nlsParse(readExtensionJSON(t, "package.json"), translation, map[string]bool{}, &missing)
	if len(missing) > 0 {
		t.Fatalf("package.nls.json'da olmayan çeviri anahtarları: %v", missing)
	}
	data, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	var p extensionPackage
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("package.json beklenen biçimde değil: %v", err)
	}
	return p
}

// Eklentinin kullanıcıya görünen metinleri (komut başlıkları, ayar ve hata ayıklama
// açıklamaları) package.json'a gömülmez, package.nls.json'dadır: başka bir yazım dili
// eklendiğinde yalnızca package.nls.<dil>.json yazılır. Her anahtarın çevirisi vardır,
// kullanılmayan çeviri kalmaz.
func TestExtensionTextsInTranslationFile(t *testing.T) {
	translation, ok := readExtensionJSON(t, "package.nls.json").(map[string]any)
	if !ok || len(translation) == 0 {
		t.Fatal("package.nls.json boş ya da nesne değil")
	}
	used := map[string]bool{}
	var missing []string
	nlsParse(readExtensionJSON(t, "package.json"), translation, used, &missing)
	if len(missing) > 0 {
		t.Errorf("package.nls.json'da olmayan anahtarlar: %v", missing)
	}
	for k := range translation {
		if !used[k] {
			t.Errorf("package.nls.json: %q anahtarı package.json'da kullanılmıyor", k)
		}
	}
	// Her dilin dosyasında aynı anahtarlar vardır (varsayılan dosya İngilizcedir).
	others, _ := reporoot.Glob(reporoot.Path("editors/vscode/package.nls.*.json"))
	if len(others) == 0 {
		t.Error("package.nls.<dil>.json dosyası yok (en az Türkçe olmalı)")
	}
	for _, f := range others {
		other, _ := readExtensionJSON(t, filepath.Base(f)).(map[string]any)
		for k := range translation {
			if _, ok := other[k]; !ok {
				t.Errorf("%s: %q anahtarı yok", filepath.Base(f), k)
			}
		}
		for k := range other {
			if _, ok := translation[k]; !ok {
				t.Errorf("%s: %q anahtarı package.nls.json'da yok", filepath.Base(f), k)
			}
		}
	}
	// Çevrilecek alanlar düz metin olmamalı.
	raw := readExtensionJSON(t, "package.json").(map[string]any)
	contributions := raw["contributes"].(map[string]any)
	check := func(place string, v any) {
		if s, _ := v.(string); !nlsKeyRe.MatchString(s) {
			t.Errorf("%s çeviri anahtarı (%%anahtar%%) olmalı, düz metin: %q", place, s)
		}
	}
	check("displayName", raw["displayName"])
	check("description", raw["description"])
	for _, k := range contributions["commands"].([]any) {
		check("commands[].title", k.(map[string]any)["title"])
	}
	for ad, essence := range contributions["configuration"].(map[string]any)["properties"].(map[string]any) {
		check("configuration "+ad, essence.(map[string]any)["markdownDescription"])
	}
	for _, h := range contributions["debuggers"].([]any) {
		launch := h.(map[string]any)["configurationAttributes"].(map[string]any)["launch"].(map[string]any)
		for ad, essence := range launch["properties"].(map[string]any) {
			check("debuggers launch "+ad, essence.(map[string]any)["description"])
		}
	}
}

// launch.json'daki "args" metin dizisi olarak tanımlıdır (VS Code tamamlar ve denetler);
// F5'te 'tr dap' başlatma isteğinden (internal/devtools/debugger), Ctrl+F5'te komut satırından
// programa gider (test/execution_test.js).
func TestExtensionDebugArguments(t *testing.T) {
	p := readExtensionPackage(t)
	if len(p.Contributes.Debuggers) != 1 {
		t.Fatalf("bir hata ayıklayıcı bekleniyordu: %d", len(p.Contributes.Debuggers))
	}
	args, ok := p.Contributes.Debuggers[0].ConfigurationAttributes.Launch.Properties["args"]
	if !ok || args.Type != "array" || args.Items.Type != "string" {
		t.Fatalf(`launch "args" metin dizisi olmalı: %+v`, args)
	}
}

// Güvenilmeyen (kısıtlı kipteki) klasörde VS Code, Türkçe harfleri ASCII dışı oldukları için
// sarı kutuyla işaretler (editor.unicodeHighlight.nonBasicASCII varsayılanı
// "inUntrustedWorkspace"); "ı" ise ayrıca "i" ile karıştırılabilir sayılır (ambiguousCharacters).
// .tr dosyalarında yalnızca Türk alfabesinin harfleri serbest olmalı: nonBasicASCII kapalı,
// Türkçe harfler allowedCharacters'ta; Latin harfine benzeyen yabancı harfler (Kiril а, Yunan ο)
// ve görünmez karakterler işaretlenmeye devam etmeli (ambiguousCharacters ve
// invisibleCharacters açık). Gerçek VS Code'da (kısıtlı kip) doğrulandı: yalnızca Kiril/Yunan
// harfleri ve sıfır genişlikli boşluk kutulu.
func TestExtensionDoesNotFlagTurkishLetters(t *testing.T) {
	tr := readExtensionPackage(t).Contributes.ConfigurationDefaults["[tr]"]
	if tr == nil {
		t.Fatal(`package.json: configurationDefaults içinde "[tr]" yok`)
	}
	expected := map[string]bool{
		"editor.unicodeHighlight.nonBasicASCII":       false,
		"editor.unicodeHighlight.ambiguousCharacters": true,
		"editor.unicodeHighlight.invisibleCharacters": true,
	}
	for setting, b := range expected {
		if v, ok := tr[setting]; !ok || v != b {
			t.Errorf("[tr] %s = %v; %v olmalı", setting, v, b)
		}
	}
	allowed, _ := tr["editor.unicodeHighlight.allowedCharacters"].(map[string]any)
	turkish := "çÇğĞıIİiöÖşŞüÜ"
	var letters []string
	for _, h := range turkish {
		if h < 0x80 { // I ve i ASCII: zaten işaretlenmez, listede olmamalı
			continue
		}
		letters = append(letters, string(h))
		if allowed[string(h)] != true {
			t.Errorf("[tr] allowedCharacters %q harfini serbest bırakmalı", h)
		}
	}
	for h := range allowed {
		if !slices.Contains(letters, h) {
			t.Errorf("[tr] allowedCharacters yalnızca Türk alfabesinin harflerini içermeli; fazla: %q", h)
		}
	}
}

// .tr dosyaları UTF-8'dir (2.1): kullanıcının genel files.encoding ayarı başka bir kodlama olsa da
// eklenti .tr dosyalarını UTF-8 açar ve kaydeder; kodlu dosyanın Türkçe görünümü de ('tr göster -'
// UTF-8 yazar) yanlış çözülmez. Yazım dili ayarı her kişinindir (kapsam "application").
func TestExtensionTRFilesUTF8(t *testing.T) {
	p := readExtensionPackage(t)
	if v := p.Contributes.ConfigurationDefaults["[tr]"]["files.encoding"]; v != "utf8" {
		t.Errorf(`[tr] files.encoding = %v; "utf8" olmalı`, v)
	}
	if s := p.Contributes.Configuration.Properties["tr.writingLanguage"].Scope; s != "application" {
		t.Errorf("tr.writingLanguage kapsamı %q; \"application\" olmalı", s)
	}
}

// tr.interpreterPath yalnızca kullanıcı ayarından okunmalı (kapsam "machine"): güvenilen bir
// klasörün .vscode/settings.json dosyası yorumlayıcıyı keyfi bir programa çeviremesin.
func TestExtensionInterpreterSettingMachineScoped(t *testing.T) {
	feature, ok := readExtensionPackage(t).Contributes.Configuration.Properties["tr.interpreterPath"]
	if !ok {
		t.Fatal("package.json: tr.interpreterPath ayarı yok")
	}
	if feature.Scope != "machine" {
		t.Errorf("tr.interpreterPath kapsamı %q; \"machine\" olmalı", feature.Scope)
	}
	js, err := os.ReadFile(reporoot.Path("editors/vscode/extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	// Yorumlayıcı arama kuralı tek yerde: find_interpreter.js (üst klasör taraması yok).
	if !strings.Contains(string(js), "require('./find_interpreter')") || strings.Contains(string(js), "path.dirname(dizin)") {
		t.Error("extension.js yorumlayıcıyı find_interpreter.js ile bulmalı ve üst klasörleri taramamalı")
	}
}

// Kısıtlı kipte eklenti "limited" çalışır: renklendirme, şablonlar ve okuma açık; yorumlayıcıyı
// başlatan her şey (çalıştırma, denetleme, hata ayıklama) kapalı. Yorumlayıcı yolu ayarı
// güvenilmeyen klasörün ayarından okunmaz; çalıştırma düğmeleri görünmez ve kod da ayrıca
// çalışma alanı güvenini denetler.
func TestExtensionRestrictedMode(t *testing.T) {
	p := readExtensionPackage(t)
	u := p.Capabilities.UntrustedWorkspaces
	if u.Supported != "limited" {
		t.Errorf("untrustedWorkspaces.supported = %v; \"limited\" olmalı", u.Supported)
	}
	if !strings.Contains(u.Description, "debugging") || !strings.Contains(u.Description, "highlighting") {
		t.Errorf("untrustedWorkspaces.description neyin açık neyin kapalı olduğunu söylemeli: %q", u.Description)
	}
	if !slices.Contains(u.RestrictedConfigurations, "tr.interpreterPath") {
		t.Errorf("tr.interpreterPath kısıtlı ayarlarda olmalı: %v", u.RestrictedConfigurations)
	}
	runners := map[string]bool{"tr.run": true, "tr.check": true, "tr.debug": true, "tr.debugStart": true}
	number := 0
	for menu, ogeler := range p.Contributes.Menus {
		for _, o := range ogeler {
			if runners[o.Command] {
				number++
				if !strings.Contains(o.When, "isWorkspaceTrusted") {
					t.Errorf("menus.%s: %s yalnızca güvenilen çalışma alanında görünmeli (when: %q)", menu, o.Command, o.When)
				}
			}
		}
	}
	if number < 6 {
		t.Errorf("çalıştırma menü öğeleri bulunamadı (%d)", number)
	}
	js, err := os.ReadFile(reporoot.Path("editors/vscode/extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(js), "if (!vscode.workspace.isTrusted)"); n < 2 {
		t.Errorf("extension.js çalıştırmadan ve hata ayıklamadan önce çalışma alanı güvenini denetlemeli (%d denetim bulundu)", n)
	}
	// Salt okunur görünüm 'tr göster -' çalıştırır: yalnızca güvenilen çalışma alanında (kısıtlı
	// kipte görünüm açılmaz; view_test.js).
	view, err := os.ReadFile(reporoot.Path("editors/vscode/view.js"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(view), "if (!vscode.workspace.isTrusted)"); n < 2 {
		t.Errorf("view.js yorumlayıcıyı çalıştırmadan önce çalışma alanı güvenini denetlemeli (%d denetim bulundu)", n)
	}
}

// Eklentinin JavaScript testleri (editors/vscode/test/*_test.js): İzleme Masası düğme →
// VS Code komutu eşlemesi, yorumlayıcıyı bulma kuralı, sahte vscode modülüyle çalıştırma ve
// stderr ayıklama yolları (F5, Ctrl+F5 ve "args") ve kodlu dosyanın salt okunur görünümü (yanda
// açılır, satırlar dosyanınki, düzenlemeyle ve yazım diliyle yenilenir, kaydedilmez, kısıtlı kipte
// açılmaz; bu testler TestMain'in derlediği yorumlayıcıyı TR_YORUMLAYICI ile kullanır). Klasördeki her test kendiliğinden koşar.
// Node yoksa atlanır (otomatik sınamada node vardır).
func TestExtensionJSTests(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; eklentinin JavaScript testleri atlandı")
	}
	tests, _ := reporoot.Glob(reporoot.Path("editors/vscode/test/*_test.js"))
	if len(tests) < 4 {
		t.Fatalf("eklenti JS testleri bulunamadı: %v", tests)
	}
	for _, filePath := range tests {
		command := exec.Command(node, filePath)
		command.Env = append(os.Environ(), "TR_YORUMLAYICI="+trBinary)
		stdout, err := command.CombinedOutput()
		if err != nil {
			t.Errorf("eklenti JS testi kaldı (%s): %v\n%s", filepath.Base(filePath), err, stdout)
		} else if strings.Contains(string(stdout), "atlandı") {
			t.Errorf("eklenti JS testi yorumlayıcıyı bulamadı (%s):\n%s", filepath.Base(filePath), stdout)
		}
	}
}
