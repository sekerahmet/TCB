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
	for _, id := range []string{"settingsLanguage", "settingsSystemPrompt", "settingsModel", "settingsTranscriber",
		"settingsVoice", "settingsVad", "settingsQuantized", "settingsModels", "settingsContext", "settingsMicrophone",
		"devicesRefresh", "settingsRecording", "settingsClose"} {
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

// Yükle düğmesi yoktur (M tasarımı K6, K8; Ahmet'in M1 denemesi: "o butonun orda işi ne anlamadım"): Dost seçili takımı
// kendisi hazırlar. Ayarlarda sohbet seçimi `settings_set language_model`'dir; sayfanın tek `model_load`'u kurulum
// kartının Yeniden dene'sidir (düşen yüklemeden sonra). Sözlükte Yükle ve "Kurulum tamam" kalmaz.
func TestSettingsHaveNoLoadButton(t *testing.T) {
	html := markup(t)
	for _, gone := range []string{`id="loadButton"`, `id="settingsModelLoad"`, `id="settingsDownload"`} {
		if strings.Contains(html, gone) {
			t.Errorf("the page still has %s", gone)
		}
	}
	for code, table := range dictionary(t) {
		for _, key := range []string{"button_load", "settings_model_load", "state_setup_done", "setup_done_note", "download_installed"} {
			if _, ok := table[key]; ok {
				t.Errorf("%s still has %s", code, key)
			}
		}
	}
	code := script(t)
	view := readFile(t, moduleDir+"/view.js")
	if n := strings.Count(view, `command("model_load"`); n != 1 || !strings.Contains(view, `byId("setupRetry").addEventListener("click", () => command("model_load", { entry_id: str(ui.settings.language_model) }));`) {
		t.Errorf("view.js sends model_load %d times; want once, from the setup card's retry", n)
	}
	if !strings.Contains(code, `byId("settingsModel").addEventListener("change", () => setSetting("language_model",`) {
		t.Error("choosing a chat model in the settings does not send settings_set language_model")
	}
	// Ayar ve katalog gelince kart, takım kutusu, durum satırı ve pencere yeniden çizilir.
	for _, event := range []string{"settings", "catalog"} {
		if body := capture(t, `(?s)on\("`+event+`", \(d\) => \{(.*?)\n\}\);`, code); !strings.Contains(body, "renderStatus();") {
			t.Errorf("the %s event does not redraw the setup card and the state line:\n%s", event, body)
		}
	}
	status := capture(t, `(?s)function renderStatus\(\) \{(.*?)\n\}`, code)
	for _, want := range []string{"renderTeam();", "renderSetup();", "renderSettings();", "renderModels();"} {
		if !strings.Contains(status, want) {
			t.Errorf("renderStatus does not call %s", want)
		}
	}
}

// Modeller penceresi (K5): üst çubukta her zaman, dişlinin yanında bir düğme açar; kurulum kartından ve ayarlardan da
// açılır. Gruplar sözleşmenin dört türüdür, her biri adı ve tek cümle açıklamasıyla, her dilde.
func TestModelsDialogNamesEveryKind(t *testing.T) {
	top := capture(t, `(?s)<div class="top-tools">(.*?)</div>`, markup(t))
	open := capture(t, `(<button\b[^>]*\bid="modelsOpen"[^>]*>)`, top)
	for _, want := range []string{`aria-controls="modelsDialog"`, `aria-haspopup="dialog"`} {
		if !strings.Contains(open, want) {
			t.Errorf("the models button has no %s: %s", want, open)
		}
	}
	if strings.Contains(open, "hidden") || strings.Index(top, `id="modelsOpen"`) > strings.Index(top, `id="settingsOpen"`) {
		t.Errorf("the models button is not always shown before the gear: %s", open)
	}
	block(t, "dialog", "modelsDialog")
	kinds := matches("`([a-z_]+)`", capture(t, "`kind` \\(([^)]*)\\)", section(t, "Olaylar")))
	list := capture(t, `const MODEL_KINDS = \[(.*?)\];`, script(t))
	for code, table := range dictionary(t) {
		for _, kind := range kinds {
			if table["kind_"+kind] == "" || table["kind_"+kind+"_note"] == "" {
				t.Errorf("%s has no kind_%s or kind_%s_note", code, kind, kind)
			}
		}
	}
	for _, kind := range kinds {
		if !strings.Contains(list, `"`+kind+`"`) {
			t.Errorf("MODEL_KINDS has no %s", kind)
		}
	}
	if len(kinds) != 4 {
		t.Errorf("want the contract's four kinds, read %v", kinds)
	}
	code := script(t)
	for _, want := range []string{"bind(el(\"h3\", \"label\"), `kind_${kind}`)", "bind(el(\"p\", \"kind-note\"), `kind_${kind}_note`)",
		`byId("setupModels").addEventListener("click", openModels);`, `byId("settingsModels").addEventListener("click", openModels);`} {
		if !strings.Contains(code, want) {
			t.Errorf("the page has no %q", want)
		}
	}
}

// Nicemli etkinleştirmenin seçicisi (N2) ayarların "Kullanılan modeller" bölümündedir: üç değer, `settings_set`.
func TestQuantizedSelectorIsInModels(t *testing.T) {
	group := capture(t, `(?s)<section class="box" aria-labelledby="settingsGroupModels">(.*?)</section>`, markup(t))
	selector := capture(t, `(?s)<select id="settingsQuantized">(.*?)</select>`, group)
	for _, value := range []string{"auto", "on", "off"} {
		if !strings.Contains(selector, `<option value="`+value+`" data-i18n="settings_quantized_`+value+`"></option>`) {
			t.Errorf("the quantized selector has no %s option", value)
		}
	}
	if !strings.Contains(script(t), `setSetting("quantized_activations",`) {
		t.Error("the quantized selector does not send settings_set quantized_activations")
	}
}

// Takım kutusu (K7) sağ panelde MODEL satırının yerindedir: sohbet, sesten yazıya, yazıdan sese ve durum. Yüklü takım
// yokken (`setup`) yalnız "Model yok" der; "bilinmiyor" (`model_unknown`) hiçbir yerde kalmaz (defter 5 (c)); bağlantı
// yokken "Hazır" demez.
func TestTeamBoxNamesTheTeam(t *testing.T) {
	box := capture(t, `(?s)<div class="team" id="teamBox">(.*?)\n    </div>`, block(t, "aside", "right"))
	for _, id := range []string{"teamChat", "teamTranscriber", "teamVoice", "teamState"} {
		if !strings.Contains(box, `id="`+id+`"`) {
			t.Errorf("the team box has no %s", id)
		}
	}
	if strings.Contains(markup(t), `id="modelText"`) || strings.Contains(markup(t), "label_model") {
		t.Error("the panel still has the MODEL line")
	}
	for code, table := range dictionary(t) {
		if _, ok := table["model_unknown"]; ok {
			t.Errorf("%s still has model_unknown", code)
		}
	}
	team := capture(t, `(?s)function renderTeam\(\) \{(.*?)\n\}`, script(t))
	// Bağlantı yokken kutu "Hazır" demez: durum halkanınki gibi stateKey'den (çevrimdışı "Bağlantı yok").
	for _, want := range []string{`if (setup) bind(line, "state_setup_empty");`, `teamEntry("transcriber")`, `teamEntry("voice")`,
		`"team_preparing"`, `"team_ready"`, `const state = stateKey();`, `state === "offline" ? ` + "`state_${state}`"} {
		if !strings.Contains(team, want) {
			t.Errorf("renderTeam has no %q", want)
		}
	}
}

// Sesli / Yalnız sohbet (tasarım dost-sohbet-gpu S4): sağ panelde Konuş'un üstünde iki seçenekli düğme; seçim ayarın
// `chat_only`'sini `settings_set` ile gönderir ve `settings` olayından çizilir.
func TestChatOnlySwitchSendsTheSetting(t *testing.T) {
	panel := block(t, "aside", "right")
	mode := strings.Index(panel, `id="chatMode"`)
	talk := strings.Index(panel, `id="micButton"`)
	if mode < 0 || talk < 0 || mode > talk {
		t.Fatal("the voice/chat-only switch is not above Talk in the right panel")
	}
	for _, want := range []string{`role="radiogroup" data-i18n="chat_mode_label"`, `id="modeVoice" type="button" role="radio"`,
		`id="modeText" type="button" role="radio"`, `data-i18n="chat_mode_voice"`, `data-i18n="chat_mode_text"`} {
		if !strings.Contains(panel, want) {
			t.Errorf("the switch has no %q", want)
		}
	}
	js := script(t)
	choose := capture(t, `(?s)async function chooseChatOnly\(on\) \{(.*?)\n\}`, js)
	if !strings.Contains(choose, `command("settings_set", { key: "chat_only", value: on })`) {
		t.Error("chooseChatOnly does not send settings_set chat_only")
	}
	for _, want := range []string{`byId("modeVoice").addEventListener("click", () => chooseChatOnly(false));`,
		`byId("modeText").addEventListener("click", () => chooseChatOnly(true));`} {
		if !strings.Contains(js, want) {
			t.Errorf("the page has no %q", want)
		}
	}
}

// Yalnız sohbette Konuş gizlenir, yerinde not durur; düğme tur sürerken seçilmez (sunucu `turn_busy` verir).
func TestChatOnlyHidesTalk(t *testing.T) {
	controls := capture(t, `(?s)function renderControls\(\) \{(.*?)\n\}`, script(t))
	for _, want := range []string{`const chatOnly = ui.settings.chat_only === true;`, `mic.hidden = chatOnly;`,
		`byId("chatOnlyNote").hidden = !chatOnly;`, `option.disabled = !usable || ui.activeTurn !== "";`} {
		if !strings.Contains(controls, want) {
			t.Errorf("renderControls has no %q", want)
		}
	}
	if !strings.Contains(block(t, "aside", "right"), `<p class="mode-note" id="chatOnlyNote" data-i18n="chat_only_note" hidden></p>`) {
		t.Error("the right panel has no chat-only note")
	}
}

// Hesap cihazı (tasarım dost-sohbet-gpu T6): Ayarlar'da Kullanılan modeller bölümünde; değerler `session`'ın
// `compute_devices`'ından: kart yoksa "Ekran kartı" seçilemez ve not durur; nicemli "Açık" ile kart birlikte seçilmez;
// ayar kart deyip modeller işlemcideyse not durur. Seçim `settings_set device` gönderir.
func TestDeviceSelectorFollowsTheMachine(t *testing.T) {
	group := capture(t, `(?s)<section class="box" aria-labelledby="settingsGroupModels">(.*?)</section>`, markup(t))
	selector := capture(t, `(?s)<select id="settingsDevice">(.*?)</select>`, group)
	for _, value := range []string{"cpu", "gpu"} {
		if !strings.Contains(selector, `<option value="`+value+`" data-i18n="settings_device_`+value+`"></option>`) {
			t.Errorf("the device selector has no %s option", value)
		}
	}
	for _, id := range []string{`id="settingsDeviceNoGpu" data-i18n="settings_device_no_gpu" hidden`, `id="settingsDeviceMissing" data-i18n="error_compute_device_unavailable" hidden`} {
		if !strings.Contains(group, id) {
			t.Errorf("the device field has no %q", id)
		}
	}
	js := script(t)
	render := capture(t, `(?s)function renderDevice\(usable\) \{(.*?)\n\}`, js)
	for _, want := range []string{`const gpu = ui.computeDevices.includes("gpu");`, `byId("settingsDeviceNoGpu").hidden = gpu;`,
		`.disabled = !gpu || ui.settings.quantized_activations === "on";`, `option[value="on"]')).disabled = saved === "gpu";`,
		`byId("settingsDeviceMissing").hidden = !(saved === "gpu" && ui.computeDeviceInUse !== "gpu");`} {
		if !strings.Contains(render, want) {
			t.Errorf("renderDevice has no %q", want)
		}
	}
	for _, want := range []string{`setSetting("device",`, `ui.computeDevices = Array.isArray(d.compute_devices)`, `ui.computeDeviceInUse = d.compute_device_in_use === "gpu"`} {
		if !strings.Contains(js, want) {
			t.Errorf("the page has no %q", want)
		}
	}
}

// Sonucu değiştiren ayarın işareti (15.22a; tek tık S1-20): hesap cihazının altında "Cevabı değiştirebilir", stilde
// işaretli.
func TestDeviceSelectorMarksResultChange(t *testing.T) {
	field := capture(t, `(?s)<label for="settingsDevice"(.*?)</div>`, markup(t))
	if !strings.Contains(field, `<p class="hint result-mark" data-i18n="settings_changes_answers"></p>`) {
		t.Error("the device field has no result-change mark")
	}
	if !strings.Contains(style(t), `.result-mark::before {`) {
		t.Error("the style has no result-mark")
	}
}
