package main

import (
	"encoding/json"
	"os"
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
		Menus                 map[string][]struct {
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

// Güvenilmeyen (kısıtlı kipteki) klasörde VS Code, Türkçe harfleri (ş, ğ, ı, ü, ö, ç) ASCII
// dışı oldukları için sarı kutuyla işaretliyordu (editor.unicodeHighlight.nonBasicASCII
// varsayılanı "inUntrustedWorkspace"). .tr dosyalarında bu işaretleme kapalı olmalı;
// görünmez karakterler ise işaretlenmeye devam etmeli (kodda gizli karakter fark edilsin).
func TestEklentiTurkceHarfleriIsaretlemez(t *testing.T) {
	tr := eklentiPaketiOku(t).Contributes.ConfigurationDefaults["[tr]"]
	if tr == nil {
		t.Fatal(`package.json: configurationDefaults içinde "[tr]" yok`)
	}
	for _, ayar := range []string{"editor.unicodeHighlight.nonBasicASCII", "editor.unicodeHighlight.ambiguousCharacters"} {
		if v, ok := tr[ayar]; !ok || v != false {
			t.Errorf("[tr] %s = %v; false olmalı (Türkçe harfler kutulanmasın)", ayar, v)
		}
	}
	if v, ok := tr["editor.unicodeHighlight.invisibleCharacters"]; ok && v != true {
		t.Errorf("[tr] editor.unicodeHighlight.invisibleCharacters = %v; görünmez karakterler işaretlenmeye devam etmeli", v)
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
