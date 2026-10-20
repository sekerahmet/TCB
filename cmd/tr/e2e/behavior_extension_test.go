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

// eklentiPaketi VS Code eklentisinin package.json'ının bu testlerin baktığı kısmıdır.
type eklentiPaketi struct {
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

// nlsAnahtarRe package.json'daki çeviri anahtarıdır ("%komut.calistir%"): VS Code değeri
// package.nls.json'dan (ileride package.nls.<dil>.json'dan) alır.
var nlsAnahtarRe = regexp.MustCompile(`^%([^%\s]+)%$`)

// eklentiJSONOku eklenti klasöründeki bir JSON dosyasını genel biçimde okur.
func eklentiJSONOku(t *testing.T, ad string) any {
	t.Helper()
	veri, err := os.ReadFile(reporoot.Path("editors/vscode/" + ad))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(veri, &v); err != nil {
		t.Fatalf("%s geçerli JSON değil: %v", ad, err)
	}
	return v
}

// nlsCoz v içindeki her "%anahtar%" metnini çeviri dosyasındaki değerle değiştirir; bulunan
// anahtarları kullanilan'a, bulunamayanları eksik'e yazar.
func nlsCoz(v any, ceviri map[string]any, kullanilan map[string]bool, eksik *[]string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, d := range x {
			x[k] = nlsCoz(d, ceviri, kullanilan, eksik)
		}
	case []any:
		for i, d := range x {
			x[i] = nlsCoz(d, ceviri, kullanilan, eksik)
		}
	case string:
		if m := nlsAnahtarRe.FindStringSubmatch(x); m != nil {
			c, ok := ceviri[m[1]].(string)
			if !ok {
				*eksik = append(*eksik, m[1])
				return x
			}
			kullanilan[m[1]] = true
			return c
		}
	}
	return v
}

// eklentiPaketiOku package.json'ı, çeviri anahtarları package.nls.json'dan çözülmüş olarak okur.
func eklentiPaketiOku(t *testing.T) eklentiPaketi {
	t.Helper()
	ceviri, _ := eklentiJSONOku(t, "package.nls.json").(map[string]any)
	var eksik []string
	cozulmus := nlsCoz(eklentiJSONOku(t, "package.json"), ceviri, map[string]bool{}, &eksik)
	if len(eksik) > 0 {
		t.Fatalf("package.nls.json'da olmayan çeviri anahtarları: %v", eksik)
	}
	veri, err := json.Marshal(cozulmus)
	if err != nil {
		t.Fatal(err)
	}
	var p eklentiPaketi
	if err := json.Unmarshal(veri, &p); err != nil {
		t.Fatalf("package.json beklenen biçimde değil: %v", err)
	}
	return p
}

// Eklentinin kullanıcıya görünen metinleri (komut başlıkları, ayar ve hata ayıklama
// açıklamaları) package.json'a gömülmez, package.nls.json'dadır: başka bir yazım dili
// eklendiğinde yalnızca package.nls.<dil>.json yazılır. Her anahtarın çevirisi vardır,
// kullanılmayan çeviri kalmaz.
func TestEklentiMetinleriCeviriDosyasinda(t *testing.T) {
	ceviri, ok := eklentiJSONOku(t, "package.nls.json").(map[string]any)
	if !ok || len(ceviri) == 0 {
		t.Fatal("package.nls.json boş ya da nesne değil")
	}
	kullanilan := map[string]bool{}
	var eksik []string
	nlsCoz(eklentiJSONOku(t, "package.json"), ceviri, kullanilan, &eksik)
	if len(eksik) > 0 {
		t.Errorf("package.nls.json'da olmayan anahtarlar: %v", eksik)
	}
	for k := range ceviri {
		if !kullanilan[k] {
			t.Errorf("package.nls.json: %q anahtarı package.json'da kullanılmıyor", k)
		}
	}
	// Çevrilecek alanlar düz metin olmamalı.
	ham := eklentiJSONOku(t, "package.json").(map[string]any)
	katkilar := ham["contributes"].(map[string]any)
	denetle := func(yer string, v any) {
		if s, _ := v.(string); !nlsAnahtarRe.MatchString(s) {
			t.Errorf("%s çeviri anahtarı (%%anahtar%%) olmalı, düz metin: %q", yer, s)
		}
	}
	denetle("displayName", ham["displayName"])
	denetle("description", ham["description"])
	for _, k := range katkilar["commands"].([]any) {
		denetle("commands[].title", k.(map[string]any)["title"])
	}
	for ad, oz := range katkilar["configuration"].(map[string]any)["properties"].(map[string]any) {
		denetle("configuration "+ad, oz.(map[string]any)["markdownDescription"])
	}
	for _, h := range katkilar["debuggers"].([]any) {
		launch := h.(map[string]any)["configurationAttributes"].(map[string]any)["launch"].(map[string]any)
		for ad, oz := range launch["properties"].(map[string]any) {
			denetle("debuggers launch "+ad, oz.(map[string]any)["description"])
		}
	}
}

// launch.json'daki "args" metin dizisi olarak tanımlıdır (VS Code tamamlar ve denetler);
// F5'te 'tr dap' başlatma isteğinden (internal/devtools/debugger), Ctrl+F5'te komut satırından
// programa gider (test/calistirma_test.js).
func TestEklentiHataAyiklamaArgumanlari(t *testing.T) {
	p := eklentiPaketiOku(t)
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
func TestEklentiTurkceHarfleriIsaretlemez(t *testing.T) {
	tr := eklentiPaketiOku(t).Contributes.ConfigurationDefaults["[tr]"]
	if tr == nil {
		t.Fatal(`package.json: configurationDefaults içinde "[tr]" yok`)
	}
	beklenen := map[string]bool{
		"editor.unicodeHighlight.nonBasicASCII":       false,
		"editor.unicodeHighlight.ambiguousCharacters": true,
		"editor.unicodeHighlight.invisibleCharacters": true,
	}
	for ayar, b := range beklenen {
		if v, ok := tr[ayar]; !ok || v != b {
			t.Errorf("[tr] %s = %v; %v olmalı", ayar, v, b)
		}
	}
	izinli, _ := tr["editor.unicodeHighlight.allowedCharacters"].(map[string]any)
	turkce := []rune("çÇğĞıIİiöÖşŞüÜ")
	var harfler []string
	for _, h := range turkce {
		if h < 0x80 { // I ve i ASCII: zaten işaretlenmez, listede olmamalı
			continue
		}
		harfler = append(harfler, string(h))
		if izinli[string(h)] != true {
			t.Errorf("[tr] allowedCharacters %q harfini serbest bırakmalı", h)
		}
	}
	for h := range izinli {
		if !slices.Contains(harfler, h) {
			t.Errorf("[tr] allowedCharacters yalnızca Türk alfabesinin harflerini içermeli; fazla: %q", h)
		}
	}
}

// .tr dosyaları UTF-8'dir (2.1): kullanıcının genel files.encoding ayarı başka bir kodlama olsa da
// eklenti .tr dosyalarını UTF-8 açar ve kaydeder; kodlu dosyanın Türkçe görünümü de ('tr göster -'
// UTF-8 yazar) yanlış çözülmez. Yazım dili ayarı her kişinindir (kapsam "application").
func TestEklentiTRDosyalariUTF8(t *testing.T) {
	p := eklentiPaketiOku(t)
	if v := p.Contributes.ConfigurationDefaults["[tr]"]["files.encoding"]; v != "utf8" {
		t.Errorf(`[tr] files.encoding = %v; "utf8" olmalı`, v)
	}
	if s := p.Contributes.Configuration.Properties["tr.yazimDili"].Scope; s != "application" {
		t.Errorf("tr.yazimDili kapsamı %q; \"application\" olmalı", s)
	}
}

// tr.yorumlayiciYolu yalnızca kullanıcı ayarından okunmalı (kapsam "machine"): güvenilen bir
// klasörün .vscode/settings.json dosyası yorumlayıcıyı keyfi bir programa çeviremesin.
func TestEklentiYorumlayiciAyariMakineKapsamli(t *testing.T) {
	ozellik, ok := eklentiPaketiOku(t).Contributes.Configuration.Properties["tr.yorumlayiciYolu"]
	if !ok {
		t.Fatal("package.json: tr.yorumlayiciYolu ayarı yok")
	}
	if ozellik.Scope != "machine" {
		t.Errorf("tr.yorumlayiciYolu kapsamı %q; \"machine\" olmalı", ozellik.Scope)
	}
	js, err := os.ReadFile(reporoot.Path("editors/vscode/extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	// Yorumlayıcı arama kuralı tek yerde: yorumlayici_bul.js (üst klasör taraması yok).
	if !strings.Contains(string(js), "require('./yorumlayici_bul')") || strings.Contains(string(js), "path.dirname(dizin)") {
		t.Error("extension.js yorumlayıcıyı yorumlayici_bul.js ile bulmalı ve üst klasörleri taramamalı")
	}
}

// Kısıtlı kipte eklenti "limited" çalışır: renklendirme, şablonlar ve okuma açık; yorumlayıcıyı
// başlatan her şey (çalıştırma, denetleme, hata ayıklama) kapalı. Yorumlayıcı yolu ayarı
// güvenilmeyen klasörün ayarından okunmaz; çalıştırma düğmeleri görünmez ve kod da ayrıca
// çalışma alanı güvenini denetler.
func TestEklentiKisitliKip(t *testing.T) {
	p := eklentiPaketiOku(t)
	u := p.Capabilities.UntrustedWorkspaces
	if u.Supported != "limited" {
		t.Errorf("untrustedWorkspaces.supported = %v; \"limited\" olmalı", u.Supported)
	}
	if !strings.Contains(u.Description, "hata ayıklama") || !strings.Contains(u.Description, "renklendirme") {
		t.Errorf("untrustedWorkspaces.description neyin açık neyin kapalı olduğunu söylemeli: %q", u.Description)
	}
	if !slices.Contains(u.RestrictedConfigurations, "tr.yorumlayiciYolu") {
		t.Errorf("tr.yorumlayiciYolu kısıtlı ayarlarda olmalı: %v", u.RestrictedConfigurations)
	}
	calistiranlar := map[string]bool{"tr.calistir": true, "tr.denetle": true, "tr.hataAyikla": true, "tr.hataAyiklaBaslat": true}
	sayi := 0
	for menu, ogeler := range p.Contributes.Menus {
		for _, o := range ogeler {
			if calistiranlar[o.Command] {
				sayi++
				if !strings.Contains(o.When, "isWorkspaceTrusted") {
					t.Errorf("menus.%s: %s yalnızca güvenilen çalışma alanında görünmeli (when: %q)", menu, o.Command, o.When)
				}
			}
		}
	}
	if sayi < 6 {
		t.Errorf("çalıştırma menü öğeleri bulunamadı (%d)", sayi)
	}
	js, err := os.ReadFile(reporoot.Path("editors/vscode/extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(js), "if (!vscode.workspace.isTrusted)"); n < 2 {
		t.Errorf("extension.js çalıştırmadan ve hata ayıklamadan önce çalışma alanı güvenini denetlemeli (%d denetim bulundu)", n)
	}
	// Kodlu dosyaların Türkçe görünümü 'tr göster -' ve 'tr kodla -' çalıştırır: yalnızca
	// güvenilen çalışma alanında (kısıtlı kipte salt okunur ham gösterim; kodlu_gorunum_test.js).
	gorunum, err := os.ReadFile(reporoot.Path("editors/vscode/kodlu_gorunum.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gorunum), "if (!vscode.workspace.isTrusted)") {
		t.Error("kodlu_gorunum.js yorumlayıcıyı çalıştırmadan önce çalışma alanı güvenini denetlemeli")
	}
}

// Eklentinin JavaScript testleri (editors/vscode/test/*_test.js): İzleme Masası düğme →
// VS Code komutu eşlemesi, yorumlayıcıyı bulma kuralı, sahte vscode modülüyle çalıştırma ve
// hata ayıklama yolları (F5, Ctrl+F5 ve "args") ve kodlu dosyaların Türkçe görünümü (aç,
// düzenle, kaydet, geri al, bozuk dosya, satır eşlemesi, kısıtlı kip; bu testler TestMain'in
// derlediği yorumlayıcıyı TR_YORUMLAYICI ile kullanır). Klasördeki her test kendiliğinden koşar.
// Node yoksa atlanır (otomatik sınamada node vardır).
func TestEklentiJSTestleri(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; eklentinin JavaScript testleri atlandı")
	}
	testler, _ := filepath.Glob(reporoot.Path("editors/vscode/test/*_test.js"))
	if len(testler) < 4 {
		t.Fatalf("eklenti JS testleri bulunamadı: %v", testler)
	}
	for _, yol := range testler {
		komut := exec.Command(node, yol)
		komut.Env = append(os.Environ(), "TR_YORUMLAYICI="+trIkilisi)
		cikti, err := komut.CombinedOutput()
		if err != nil {
			t.Errorf("eklenti JS testi kaldı (%s): %v\n%s", filepath.Base(yol), err, cikti)
		} else if strings.Contains(string(cikti), "atlandı") {
			t.Errorf("eklenti JS testi yorumlayıcıyı bulamadı (%s):\n%s", filepath.Base(yol), cikti)
		}
	}
}
