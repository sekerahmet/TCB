package assistant_test

import (
	"regexp"
	"strings"
	"testing"
)

// Ekranın görünümü (tek tık F1b): ayarlar dişli düğmesinin açtığı panelde, simgeler satır içi SVG, konuş düğmesi ses
// kartında. Bu testler görünümün erişilebilirlik ve yerleşim kurallarını işaretlemede ve stilde sınar; betiği koşmaz
// (Esc'in önceliği, panelin kendiliğinden kapanması ve odağın verilmesi elle ve başsız tarayıcıyla sınanır).

// block işaretlemede kimliği verilen öğenin açılış etiketinden kapanışına kadarki metindir.
func block(t *testing.T, tag, id string) string {
	t.Helper()
	return capture(t, `(?s)<`+tag+`\b[^>]*\bid="`+id+`"[^>]*>(.*?)</`+tag+`>`, markup(t))
}

// openTag işaretlemede kimliği verilen öğenin açılış etiketidir.
func openTag(t *testing.T, id string) string {
	t.Helper()
	return capture(t, `(<[a-z]+\b[^>]*\bid="`+id+`"[^>]*>)`, markup(t))
}

// Ayarın her alanı dişlinin açtığı panelin içindedir; dişli paneli adıyla gösterir ve bir pencere açtığını söyler.
func TestSettingsLiveBehindTheGear(t *testing.T) {
	panel := block(t, "dialog", "settingsDialog")
	for _, id := range []string{"settingsLanguage", "settingsSystemPrompt", "settingsModel", "settingsModelLoad",
		"settingsContext", "settingsMicrophone", "devicesRefresh", "settingsRecording", "settingsClose"} {
		if !strings.Contains(panel, `id="`+id+`"`) {
			t.Errorf("%s is not inside the settings panel", id)
		}
		if n := strings.Count(markup(t), `id="`+id+`"`); n != 1 {
			t.Errorf("%s appears %d times; want once, in the panel", id, n)
		}
	}
	gear := openTag(t, "settingsOpen")
	for _, want := range []string{`aria-controls="settingsDialog"`, `aria-haspopup="dialog"`, `aria-expanded="false"`} {
		if !strings.Contains(gear, want) {
			t.Errorf("the gear has no %s: %s", want, gear)
		}
	}
}

// Simgeler süstür: ekran okuyucu onları okumaz, adı düğmenin yazısından gelir.
func TestIconsAreHiddenFromScreenReaders(t *testing.T) {
	icons := regexp.MustCompile(`<svg\b[^>]*>`).FindAllString(markup(t), -1)
	if len(icons) == 0 {
		t.Fatal("no icons found")
	}
	for _, icon := range icons {
		if !strings.Contains(icon, `aria-hidden="true"`) {
			t.Errorf("icon without aria-hidden=\"true\": %s", icon)
		}
	}
}

// Simgeli her düğmenin sözlükten gelen bir yazısı vardır (görünür ya da yalnız ekran okuyucuya): adsız düğme olmaz.
func TestIconButtonsHaveAName(t *testing.T) {
	seen := 0
	for _, m := range regexp.MustCompile(`(?s)<button\b([^>]*)>(.*?)</button>`).FindAllStringSubmatch(markup(t), -1) {
		if !strings.Contains(m[2], "<svg") {
			continue
		}
		seen++
		if !strings.Contains(m[2], `data-i18n="`) {
			t.Errorf("icon button without a dictionary text: <button%s>", m[1])
		}
	}
	if seen == 0 {
		t.Fatal("no icon buttons found")
	}
}

// Konuş düğmesi ses kartında, durum halkasının yanındadır (odak kipinde kartla birlikte gizlenir, bugünkü gibi).
func TestTalkButtonIsInTheVoiceCard(t *testing.T) {
	if !strings.Contains(block(t, "aside", "right"), `id="micButton"`) {
		t.Error("micButton is not in the voice card (id=\"right\")")
	}
}

// Hareket azaltılmışsa animasyon ve geçiş yoktur.
func TestReducedMotionStopsAnimations(t *testing.T) {
	rule := capture(t, `(?s)@media \(prefers-reduced-motion: reduce\) \{(.*?)\n\}`, style(t))
	for _, want := range []string{"animation: none !important", "transition: none !important"} {
		if !strings.Contains(rule, want) {
			t.Errorf("reduced motion does not set %q", want)
		}
	}
}

// `card` sınıfı onay kartınındır: uyarı çerçevesi başka bir kutuya geçmez, kart model metninden ayrı görünür.
func TestCardClassIsOnlyTheApproval(t *testing.T) {
	if regexp.MustCompile(`class="([^"]* )?card( [^"]*)?"`).MatchString(markup(t)) {
		t.Error("the page markup uses the card class; it belongs to approval cards (js/approvals.js)")
	}
	for _, name := range moduleNames(t) {
		code := readFile(t, moduleDir+"/"+name+".js")
		if name != "approvals" && regexp.MustCompile(`"card( [^"]*)?"`).MatchString(code) {
			t.Errorf("js/%s.js uses the card class; it belongs to approval cards", name)
		}
	}
}

// Panel açıkken ayarın hatası panelde de görünür (Aç36): satır panelin içinde, uyarı rolüyle; gösterdiği her anahtar
// sözlükte vardır.
func TestSettingsErrorLineIsInThePanel(t *testing.T) {
	line := capture(t, `(<p\b[^>]*\bid="settingsError"[^>]*>)`, block(t, "dialog", "settingsDialog"))
	if !strings.Contains(line, `role="alert"`) {
		t.Errorf("the settings error line has no role=\"alert\": %s", line)
	}
	keys := matches(`"([a-z_]+)"`, capture(t, `(?s)const SETTINGS_ERROR_KEYS = new Set\(\[(.*?)\]\)`, script(t)))
	if len(keys) == 0 {
		t.Fatal("SETTINGS_ERROR_KEYS is empty")
	}
	table := dictionary(t)["en"]
	for _, key := range keys {
		if _, ok := table[key]; !ok {
			t.Errorf("SETTINGS_ERROR_KEYS names %q, which is not in the dictionary", key)
		}
	}
}

// Basılı çip renkten başka bir iz de taşır: simgesinin yerine onay işareti gelir (N1).
func TestPressedChipCarriesAMark(t *testing.T) {
	chips := regexp.MustCompile(`(?s)<button\b[^>]*class="[^"]*\bchip\b[^"]*"[^>]*>(.*?)</button>`).FindAllStringSubmatch(markup(t), -1)
	if len(chips) == 0 {
		t.Fatal("no chips found")
	}
	for _, chip := range chips {
		if !strings.Contains(chip[1], "chip-on") || !strings.Contains(chip[1], "chip-off") {
			t.Errorf("chip without both marks (chip-on, chip-off): %s", chip[1])
		}
	}
	css := style(t)
	for _, want := range []string{`.chip[aria-pressed="true"] .chip-off`, `.chip[aria-pressed="true"] .chip-on { display: block; }`} {
		if !strings.Contains(css, want) {
			t.Errorf("style has no %q", want)
		}
	}
}
