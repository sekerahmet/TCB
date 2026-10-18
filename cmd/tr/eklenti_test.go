package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"tr_dili/internal/depo"
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
	} `json:"contributes"`
}

func eklentiPaketiOku(t *testing.T) eklentiPaketi {
	t.Helper()
	veri, err := os.ReadFile(depo.Yol("duzenleyici/vscode/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p eklentiPaketi
	if err := json.Unmarshal(veri, &p); err != nil {
		t.Fatalf("package.json geçerli JSON değil: %v", err)
	}
	return p
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
	js, err := os.ReadFile(depo.Yol("duzenleyici/vscode/extension.js"))
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
	js, err := os.ReadFile(depo.Yol("duzenleyici/vscode/extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(js), "if (!vscode.workspace.isTrusted)"); n < 2 {
		t.Errorf("extension.js çalıştırmadan ve hata ayıklamadan önce çalışma alanı güvenini denetlemeli (%d denetim bulundu)", n)
	}
}

// Eklentinin vscode modülüne bağlı olmayan kuralları JavaScript testlerindedir
// (duzenleyici/vscode/test/*_test.js): İzleme Masası düğme → VS Code komutu eşlemesi ve
// yorumlayıcıyı bulma kuralı. Node yoksa atlanır.
func TestEklentiJSTestleri(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; eklentinin JavaScript testleri atlandı")
	}
	for _, ad := range []string{"izleme_komutlari_test.js", "yorumlayici_bul_test.js"} {
		cikti, err := exec.Command(node, depo.Yol("duzenleyici/vscode/test/"+ad)).CombinedOutput()
		if err != nil {
			t.Errorf("eklenti JS testi kaldı (%s): %v\n%s", ad, err, cikti)
		}
	}
}
