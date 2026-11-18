// @ts-check
"use strict";
// Çizim ve düğmeler. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

// ---------- Çizim ----------

/** @param {string} phase @param {Record<string, unknown>} [values] */
function setConn(phase, values) {
  ui.conn = phase;
  ui.connValues = values || {};
  renderStatus();
}

function stateKey() {
  if (!connected()) return "offline";
  return ui.state;
}

/** Yüklemenin yüzdesi dile göre ("%40"); bilinmiyorsa boş. */
function progressText() {
  return ui.progress === null ? "" : new Intl.NumberFormat(language, { style: "percent" }).format(ui.progress);
}

// Halka ve üst satır: kurulumda hiç sohbet modeli inmemişse "Model yok", inmişse "Hazır değil"; hazırlanırken yüzde.
function renderRing() {
  const state = stateKey();
  byId("ring").dataset.state = state;
  byId("right").dataset.state = state;
  const arc = byId("ringProgress");
  arc.classList.toggle("indeterminate", ui.progress === null);
  arc.setAttribute("stroke-dasharray", ui.progress === null ? "25 75" : `${ui.progress * 100} 100`);
  const text = byId("stateText");
  const top = byId("topState");
  const empty = !ui.catalog.some((e) => e.kind === "language_model" && e.downloaded);
  for (const node of [text, top]) {
    if (state === "loading" && ui.verifying) {
      bind(node, "state_loading_verifying");
    } else if (state === "loading" && ui.progress !== null) {
      bind(node, "state_loading_progress", { percent: progressText() });
    } else {
      bind(node, state === "setup" && empty ? "state_setup_empty" : `state_${state}`);
    }
  }
}

function renderConnection() {
  const text = byId("connText");
  const banner = byId("bannerConn");
  if (connected()) {
    bind(text, mock ? "conn_mock" : "conn_open");
    banner.hidden = true;
  } else {
    bind(text, `conn_${ui.conn}`, ui.connValues);
    bind(banner, `conn_${ui.conn}`, ui.connValues);
    banner.hidden = false;
  }
  byId("bannerMock").hidden = !mock;
  byId("tokenBox").hidden = ui.conn !== "auth";
  byId("pairRenew").hidden = mock !== null || !["pair_invalid", "lost"].includes(ui.conn);
}

// Konuş: hazırlanırken "Hazırlanıyor", hazırlanırken basıldıysa "Hazır olunca dinler"; kurulumda sunucu takımı kurulu
// bulmadıysa "Önce modelleri indirin". Basılı görünüm yalnız `state`'in `mic`'inden gelir (contract.md).
function renderControls() {
  const usable = connected();
  composer.disabled = !usable;
  byId("send").disabled = !usable;
  byId("stop").disabled = !usable || (!ui.activeTurn && cards.size === 0);
  const mic = byId("micButton");
  mic.disabled = !usable;
  const chatOnly = ui.settings.chat_only === true;
  byId("chatMode").hidden = Object.keys(ui.settings).length === 0;
  for (const [id, on] of /** @type {[string, boolean][]} */ ([["modeVoice", false], ["modeText", true]])) {
    const option = /** @type {HTMLButtonElement} */ (byId(id));
    option.setAttribute("aria-checked", String(chatOnly === on));
    option.disabled = !usable || ui.activeTurn !== "";
  }
  mic.hidden = chatOnly;
  byId("chatOnlyNote").hidden = !chatOnly;
  mic.setAttribute("aria-pressed", String(ui.mic));
  const value = byId("micValue");
  if (ui.state === "loading") bind(value, ui.micWaiting ? "mic_waits" : "mic_preparing");
  else if (ui.state === "setup" && ui.micHint !== "") bind(value, ui.micHint);
  else bind(value, ui.mic ? "toggle_on" : "toggle_off");
  refreshCards();
}

function renderStatus() {
  renderRing();
  renderConnection();
  renderControls();
  renderTeam();
  renderSetup();
  renderSettings();
  renderModels();
}

// Takım kutusu (sağ panel): sohbet, yazıcı ve ses adlarıyla, altında durum. Yüklü takım yokken (`setup`) yalnız
// "Model yok" der. Adlar ayar ve katalogdan gelir; hazırlanırken sohbet satırı yüklenen modeldir.
function renderTeam() {
  const setup = ui.state === "setup";
  for (const id of ["teamChatLine", "teamTranscriberLine", "teamVoiceLine"]) byId(id).hidden = setup;
  const chat = ui.loadingModel !== "" ? entryById(ui.loadingModel) : teamEntry("language_model");
  const names = { teamChat: chat, teamTranscriber: teamEntry("transcriber"), teamVoice: teamEntry("voice") };
  for (const [id, entry] of Object.entries(names)) byId(id).textContent = entry ? entry.name : "—";
  // Bağlantı yokken durum bilinmez: halka gibi "Bağlantı yok" der, "Hazır" demez.
  const state = stateKey();
  const line = byId("teamState");
  line.className = ["setup", "loading", "offline"].includes(state) ? "team-state" : state === "error" ? "team-state team-bad" : "team-state team-ok";
  if (setup) bind(line, "state_setup_empty");
  else if (state === "loading") bind(line, "team_preparing", { model: chat ? chat.name : "", percent: progressText() });
  else bind(line, state === "error" || state === "offline" ? `state_${state}` : "team_ready");
}

// Kurulum kartı (`setup`): konuşma dili ve Modeller; inerken inen işin adı ve yüzdesi (ortak parça kendi adıyla);
// "bitince kendiliğinden belleğe alınır" yalnız bu iniş ayarın takımını tamamlayacaksa (`completesTeam`, DENETİM 880 N1).
// Yükleme düştüyse "<ad> belleğe alınamadı", Yeniden dene (`model_load`) ve Başka model seç (ayarlar). Yükle yok.
// Ayarın dil modeli inmemiş ama başka bir sohbet modeli kuruluysa (`setupChoice`) kart bunu söyler; Modelleri aç ve
// Başka model seç ikisi de görünür (katalog dilimi K-10).
function renderSetup() {
  const box = byId("setupBox");
  box.hidden = ui.state !== "setup";
  const failed = ui.loadFailed !== "";
  box.dataset.failed = String(failed);
  const note = byId("setupNote");
  const job = failed ? null : runningJob();
  const chosen = str(ui.settings.language_model);
  const choose = !failed && setupChoice(chosen, ui.catalog);
  if (failed) bind(byId("setupHeading"), "setup_load_failed", { model: nameOf(ui.loadFailed) });
  else if (choose) bind(byId("setupHeading"), "setup_chosen_missing", { model: nameOf(chosen) });
  else bind(byId("setupHeading"), "setup_start");
  if (failed) bind(note, "setup_load_failed_note");
  else if (job) bind(note, completesTeam(str(job.entry_id), ui.catalog, ui.settings, ui.downloads) ? "setup_downloading" : "setup_downloading_other", { model: nameOf(str(job.entry_id)), percent: new Intl.NumberFormat(language, { style: "percent" }).format(percentOf(job) / 100) });
  else if (choose) bind(note, "setup_chosen_missing_note");
  note.hidden = !failed && !job && !choose;
  byId("setupModels").hidden = failed;
  byId("setupRetry").hidden = !failed;
  byId("setupOther").hidden = !failed && !choose;
  for (const id of ["setupModels", "setupRetry", "setupOther"]) byId(id).disabled = !connected();
  const speech = /** @type {HTMLSelectElement} */ (byId("speechLanguage"));
  const current = str(ui.settings.speech_language);
  if (current !== "") speech.value = current;
  speech.disabled = !connected();
}

/** @param {HTMLSelectElement} select @param {{value: string, text: string, disabled?: boolean}[]} items @param {string} chosen */
function fillSelect(select, items, chosen) {
  select.replaceChildren(...items.map((item) => {
    const option = /** @type {HTMLOptionElement} */ (el("option", "", item.text));
    option.value = item.value;
    option.disabled = item.disabled === true;
    return option;
  }));
  if (items.some((item) => item.value === chosen)) select.value = chosen;
}

// Ayarlar: ayar dosyası varken (`settings` geldi), kurulum dışında; yükleme düştüyse ya da ayarın dil modeli inmemişken
// başka bir sohbet modeli kuruluysa kurulumda da (kartın "Başka model seç"i). Değerler sunucudan gelir; her değişiklik
// bir `settings_set`'tir, sunucu reddederse değer sunucununkine döner.
// Sohbet modeli: yalnız sohbet seçilir, takımı kurulu girdilerden; seçim hemen geçerlidir (takım yeniden
// hazırlanır), Yükle yok. Yazıcı ve VAD bu dil için tek seçenekli sönük seçicidir; ses `renderVoice`'tadır.
function renderSettings() {
  const chosen = str(ui.settings.language_model);
  const shown = Object.keys(ui.settings).length > 0 && (ui.state !== "setup" || ui.loadFailed !== "" || setupChoice(chosen, ui.catalog));
  byId("settingsOpen").hidden = !shown;
  if (!shown && settingsDialog.open) {
    closeSettings();
    focusAfterSettings();
  }
  const usable = connected();
  const speech = /** @type {HTMLSelectElement} */ (byId("settingsLanguage"));
  const current = str(ui.settings.speech_language);
  if (current !== "") speech.value = current;
  speech.disabled = !usable;
  const installed = loadableModels(ui.catalog);
  const modelSelect = /** @type {HTMLSelectElement} */ (byId("settingsModel"));
  const options = modelOptions(ui.catalog, chosen).map((o) => ({ value: o.value, text: o.missing ? t("settings_model_missing", { model: o.name }) : o.name, disabled: o.missing }));
  fillSelect(modelSelect, options, chosen);
  modelSelect.disabled = !usable || installed.length === 0;
  byId("settingsModelNote").hidden = installed.length === 0;
  byId("settingsModelNone").hidden = installed.length > 0;
  for (const [kind, id] of [["transcriber", "settingsTranscriber"], ["vad", "settingsVad"]]) {
    const entry = teamEntry(kind);
    fillSelect(byId(id), entry ? [{ value: entry.id, text: entryLabel(entry) }] : [], entry ? entry.id : "");
  }
  renderVoice(usable, current);
  const quantized = /** @type {HTMLSelectElement} */ (byId("settingsQuantized"));
  quantized.value = str(ui.settings.quantized_activations) || "auto";
  quantized.disabled = !usable;
  renderDevice(usable);
  const context = /** @type {HTMLInputElement} */ (byId("settingsContext"));
  if (document.activeElement !== context) context.value = String(num(ui.settings.context) ?? "");
  context.disabled = !usable;
  const saved = str(ui.settings.microphone);
  const names = ui.microphones === null ? [] : ui.microphones.slice();
  if (saved !== "" && !names.includes(saved)) names.unshift(saved);
  const micSelect = /** @type {HTMLSelectElement} */ (byId("settingsMicrophone"));
  fillSelect(micSelect, names.map((value) => ({ value, text: value })), saved);
  micSelect.disabled = !usable || names.length === 0;
  byId("devicesRefresh").disabled = !usable;
  byId("devicesEmpty").hidden = !(ui.microphones !== null && ui.microphones.length === 0);
  const recording = /** @type {HTMLInputElement} */ (byId("settingsRecording"));
  recording.checked = ui.settings.recording === true;
  recording.disabled = !usable;
  byId("recordingNote").hidden = !ui.recordingNote;
  const prompt = /** @type {HTMLTextAreaElement} */ (byId("settingsSystemPrompt"));
  const own = /** @type {any} */ (ui.settings.system_prompt);
  if (document.activeElement !== prompt) prompt.value = own && own.source === "user" ? str(own.text) : "";
  prompt.disabled = !usable;
  if (shown && usable && !ui.devicesAsked) {
    ui.devicesAsked = true;
    command("devices_list", {});
  }
}

// Yazıdan sese (P dilimi, P-7): konuşma dilinin sesleri sırayla; inmemiş ses seçilemez ve "(indirilmedi)" der. Dilin tek
// sesi varsa seçici sönük ve "Bu dil için tek seçenek" görünür; birden çoksa seçici açılır, ipucu gizlenir.
/** @param {boolean} usable @param {string} speech */
function renderVoice(usable, speech) {
  const voices = voiceOptions(ui.catalog, speech);
  const chosen = voices.find((v) => v.selected);
  const items = voices.map((o) => ({ value: o.value, text: o.missing ? t("settings_model_missing", { model: o.name }) : o.name, disabled: o.missing }));
  const select = /** @type {HTMLSelectElement} */ (byId("settingsVoice"));
  fillSelect(select, items, chosen ? chosen.value : "");
  select.disabled = !usable || voices.length < 2;
  byId("settingsVoiceOnly").hidden = voices.length > 1;
}

// Hesap cihazı (tasarım dost-sohbet-gpu T6): kart yoksa "Ekran kartı" seçilemez ve not durur; kartta nicemlinin
// "Açık"ı, nicemli açıkken kart seçilemez (sunucu `invalid_value` verir). Kullanılan cihaz ayarınkinden farklıysa
// (ayar kart diyor, kart yok) not durur.
/** @param {boolean} usable */
function renderDevice(usable) {
  const device = /** @type {HTMLSelectElement} */ (byId("settingsDevice"));
  const saved = str(ui.settings.device) || "cpu";
  const gpu = ui.computeDevices.includes("gpu");
  device.value = saved;
  device.disabled = !usable;
  /** @type {HTMLOptionElement} */ (device.querySelector('option[value="gpu"]')).disabled = !gpu || ui.settings.quantized_activations === "on";
  /** @type {HTMLOptionElement} */ (byId("settingsQuantized").querySelector('option[value="on"]')).disabled = saved === "gpu";
  byId("settingsDeviceNoGpu").hidden = gpu;
  byId("settingsDeviceMissing").hidden = !(saved === "gpu" && ui.computeDeviceInUse !== "gpu");
}

/** @param {string} key @param {unknown} value */
async function setSetting(key, value) {
  if (!(await command("settings_set", { key, value }))) renderSettings();
}

function renderMetrics() {
  const memory = num(ui.metrics.memory_bytes);
  const tps = num(ui.metrics.tokens_per_second);
  const first = num(ui.metrics.first_token_ms);
  byId("metricMemory").textContent = memory === null ? "—" : formatBytes(memory);
  const tpsNode = byId("metricTps");
  if (tps === null) {
    delete tpsNode.dataset.i18n;
    tpsNode.textContent = "—";
  } else {
    const value = new Intl.NumberFormat(language, { maximumFractionDigits: 1 }).format(tps);
    bind(tpsNode, "unit_tokens_per_second", { value });
  }
  byId("metricFirstToken").textContent = first === null ? "—" : formatDuration(first);
}

function renderTools() {
  byId("toolList").replaceChildren(...ui.tools.map((tool) => {
    const item = el("li");
    item.append(el("code", "", tool.name));
    if (tool.irreversible) item.append(bind(el("span", "warn-text"), "tool_irreversible"));
    return item;
  }));
  byId("toolsEmpty").hidden = ui.tools.length > 0;
}

function renderToggles() {
  const sheet = byId("toolSheet");
  const focused = document.body.classList.contains("focus");
  byId("toolsButton").setAttribute("aria-pressed", String(!sheet.hidden));
  byId("focusButton").setAttribute("aria-pressed", String(focused));
  byId("focusExit").hidden = !focused;
  byId("topState").hidden = !focused;
}

function fillScenes() {
  const select = byId("sceneSelect");
  const chosen = select.value;
  select.replaceChildren(...mock.scenes.map((/** @type {string} */ name) => {
    const option = /** @type {HTMLOptionElement} */ (el("option", "", t(`scene_${name}`)));
    option.value = name;
    return option;
  }));
  if (chosen) select.value = chosen;
}

function applyLanguage() {
  document.documentElement.lang = language;
  document.documentElement.dir = (dictionary[language] || {}).direction || "ltr";
  for (const node of document.querySelectorAll("[data-i18n]")) paint(/** @type {HTMLElement} */ (node));
  renderMetrics();
  if (mock) fillScenes();
  byId("languageSelect").value = language;
  refreshCards();
}

// ---------- Düğmeler ----------

async function submit() {
  const text = composer.value.trim();
  if (!text || !connected()) return;
  if (await command("send_text", { text })) composer.value = "";
}

// Ayarlar dişli düğmesinin açtığı kalıcı pencerededir (`<dialog>`): Esc, kapat düğmesi ve dışına tık kapar,
// odak kapanınca dişli düğmesine döner. Pencere ayarın yalnız yeridir; komutlar ve olaylar aynı.
const settingsDialog = /** @type {HTMLDialogElement} */ (byId("settingsDialog"));

// ---------- Ayar grupları ----------
/**
 * Ayarlar penceresinin kategorileri, sırasıyla. Her kimliğin index.html'de `data-category`'li bir paneli ve iki dilde
 * `settings_group_<kimlik>` adı vardır; yeni kategori buraya bir satırdır (README, "Değiştirmek").
 */
const SETTINGS_CATEGORIES = [
  "speech",
  "chat_model",
  "audio",
  "performance",
];

/** Açılacak kategori: verilmeyen ya da bilinmeyen kimlik ilk kategoriye düşer. @param {unknown} category */
function settingsCategory(category) {
  return typeof category === "string" && SETTINGS_CATEGORIES.includes(category) ? category : SETTINGS_CATEGORIES[0];
}

/**
 * Listede tuşla gidilen kategori: oklar bir adım (uçta öbür uca döner), Home ve End uçlar; öbür tuş `null`.
 * @param {string} current @param {string} key
 */
function categoryAfterKey(current, key) {
  const at = SETTINGS_CATEGORIES.indexOf(current);
  const last = SETTINGS_CATEGORIES.length - 1;
  if (key === "ArrowDown") return SETTINGS_CATEGORIES[at >= last ? 0 : at + 1];
  if (key === "ArrowUp") return SETTINGS_CATEGORIES[at <= 0 ? last : at - 1];
  if (key === "Home") return SETTINGS_CATEGORIES[0];
  if (key === "End") return SETTINGS_CATEGORIES[last];
  return null;
}
// ---------- Ayar grupları sonu ----------

/** Seçili kategori: sekmesi seçili ve odaklanabilir, paneli açık; öbürleri kapalı. @param {string} category */
function showCategory(category) {
  for (const id of SETTINGS_CATEGORIES) {
    const tab = byId(`settingsTab_${id}`);
    tab.setAttribute("aria-selected", String(id === category));
    tab.tabIndex = id === category ? 0 : -1;
    byId(`settingsPanel_${id}`).hidden = id !== category;
  }
}

/** Sekmeleri listeden çizer; adları panellerin başlığıyla aynı anahtardır (applyLanguage boyar). */
function wireSettingsTabs() {
  const list = byId("settingsTabs");
  for (const id of SETTINGS_CATEGORIES) {
    const tab = el("button", "settings-tab");
    tab.type = "button";
    tab.id = `settingsTab_${id}`;
    tab.setAttribute("role", "tab");
    tab.setAttribute("aria-controls", `settingsPanel_${id}`);
    tab.dataset.i18n = `settings_group_${id}`;
    tab.addEventListener("click", () => {
      showCategory(id);
      settingsDialog.dataset.view = "panel";
    });
    tab.addEventListener("keydown", (event) => {
      const next = categoryAfterKey(id, event.key);
      if (next === null) return;
      event.preventDefault();
      showCategory(next);
      byId(`settingsTab_${next}`).focus();
    });
    list.append(tab);
  }
  byId("settingsBack").addEventListener("click", () => {
    settingsDialog.dataset.view = "list";
    const selected = list.querySelector('[aria-selected="true"]');
    if (selected) /** @type {HTMLElement} */ (selected).focus();
  });
  showCategory(SETTINGS_CATEGORIES[0]);
}

/** @param {string} [category] verilmezse liste açılır (dar ekranda önce liste); verilirse o kategori */
function openSettings(category) {
  if (settingsDialog.open) return;
  byId("settingsError").hidden = true;
  showCategory(settingsCategory(category));
  settingsDialog.dataset.view = category === undefined ? "list" : "panel";
  settingsDialog.showModal();
  byId("settingsOpen").setAttribute("aria-expanded", "true");
}

function closeSettings() {
  if (settingsDialog.open) settingsDialog.close();
}

/** Panel kendiliğinden kapandı (ayar kalktı, kurulum): dişli gizli, odak görünür ve etkin ilk öğeye gider. */
function focusAfterSettings() {
  const target = [composer, byId("micButton"), stream].find(
    (node) => !(/** @type {HTMLButtonElement} */ (node)).disabled && node.offsetParent !== null);
  if (target) target.focus();
}

// Panel açıkken ayarın reddi ya da yazılamaması akışın notuyla aynı metinle panelde de görünür (akış panelin arkasında).
const SETTINGS_ERROR_KEYS = new Set(["error_setting_invalid", "error_unknown_language", "error_settings_write_failed",
  "settings_write_failed_load_note", "error_devices_unavailable", "error_not_installed", "error_unknown_entry",
  "error_model_load_failed"]);

/** @param {string} key @param {Record<string, unknown>} values */
function settingsErrorNote(key, values) {
  if (!settingsDialog.open || !SETTINGS_ERROR_KEYS.has(key)) return;
  const line = byId("settingsError");
  bind(line, key, values);
  line.hidden = false;
}

// Konuş: açıkken ya da beklerken basılırsa kapatır (bekleyen dinleme iptal). Hazırlanırken ya da kurulumda kabul
// edilen açma bekler; sunucu takımı kurulu bulmazsa (`not_installed`) düğme "Önce modelleri indirin" der.
async function micPressed() {
  const on = !(ui.mic || ui.micWaiting);
  if (!on) ui.micWaiting = false;
  const reply = await sendCommand("mic_set", { on });
  if (!reply.ok) {
    noteError(reply.error);
    ui.micHint = reply.error && reply.error.code === "not_installed" ? "mic_download_first" : "";
  } else if (on && !ui.mic && (ui.state === "loading" || ui.state === "setup")) {
    ui.micWaiting = true;
  }
  renderControls();
}

// Sesli / Yalnız sohbet: ayarın `chat_only`'si (contract.md); seçim `settings` olayıyla çizilir. Yalnız sohbete
// geçince yazı kutusu odak alır.
/** @param {boolean} on */
async function chooseChatOnly(on) {
  if ((ui.settings.chat_only === true) === on) return;
  if ((await command("settings_set", { key: "chat_only", value: on })) && on) composer.focus();
}

/** @param {boolean} on */
function setFocusMode(on) {
  document.body.classList.toggle("focus", on);
  renderToggles();
  if (!on) byId("focusButton").focus();
}

function wire() {
  byId("send").addEventListener("click", submit);
  composer.addEventListener("keydown", (/** @type {KeyboardEvent} */ event) => {
    if (event.key === "Enter" && !event.isComposing) submit();
  });
  byId("stop").addEventListener("click", () => command("stop", {}));
  byId("micButton").addEventListener("click", micPressed);
  byId("modeVoice").addEventListener("click", () => chooseChatOnly(false));
  byId("modeText").addEventListener("click", () => chooseChatOnly(true));
  byId("setupModels").addEventListener("click", openModels);
  byId("setupRetry").addEventListener("click", () => command("model_load", { entry_id: str(ui.settings.language_model) }));
  byId("setupOther").addEventListener("click", () => openSettings("chat_model"));
  byId("settingsModels").addEventListener("click", openModels);
  byId("speechLanguage").addEventListener("change", () => {
    const value = /** @type {HTMLSelectElement} */ (byId("speechLanguage")).value;
    command("settings_set", { key: "speech_language", value });
  });
  byId("settingsLanguage").addEventListener("change", () => setSetting("speech_language", /** @type {HTMLSelectElement} */ (byId("settingsLanguage")).value));
  byId("settingsModel").addEventListener("change", () => setSetting("language_model", /** @type {HTMLSelectElement} */ (byId("settingsModel")).value));
  byId("settingsVoice").addEventListener("change", () => setSetting("voice", /** @type {HTMLSelectElement} */ (byId("settingsVoice")).value));
  byId("settingsDevice").addEventListener("change", () => setSetting("device", /** @type {HTMLSelectElement} */ (byId("settingsDevice")).value));
  byId("settingsQuantized").addEventListener("change", () => setSetting("quantized_activations", /** @type {HTMLSelectElement} */ (byId("settingsQuantized")).value));
  byId("settingsContext").addEventListener("change", () => setSetting("context", Number(/** @type {HTMLInputElement} */ (byId("settingsContext")).value)));
  byId("settingsMicrophone").addEventListener("change", () => setSetting("microphone", /** @type {HTMLSelectElement} */ (byId("settingsMicrophone")).value));
  byId("devicesRefresh").addEventListener("click", () => command("devices_list", {}));
  byId("settingsRecording").addEventListener("change", async () => {
    const value = /** @type {HTMLInputElement} */ (byId("settingsRecording")).checked;
    const ok = await command("settings_set", { key: "recording", value });
    ui.recordingNote = ok && value;
    renderSettings();
  });
  byId("settingsSystemPrompt").addEventListener("change", () => {
    const text = /** @type {HTMLTextAreaElement} */ (byId("settingsSystemPrompt")).value.trim();
    setSetting("system_prompt", text === "" ? { source: "catalog" } : { source: "user", text, language: str(ui.settings.speech_language) });
  });
  byId("toolsButton").addEventListener("click", () => {
    const sheet = byId("toolSheet");
    sheet.hidden = !sheet.hidden;
    renderToggles();
  });
  byId("focusButton").addEventListener("click", () => setFocusMode(true));
  byId("focusExit").addEventListener("click", () => setFocusMode(false));
  byId("settingsOpen").addEventListener("click", () => openSettings());
  wireSettingsTabs();
  byId("settingsClose").addEventListener("click", closeSettings);
  settingsDialog.addEventListener("click", (event) => {
    if (event.target === settingsDialog) closeSettings();
  });
  settingsDialog.addEventListener("close", () => {
    byId("settingsOpen").setAttribute("aria-expanded", "false");
    byId("settingsError").hidden = true;
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !settingsDialog.open && !modelsDialog.open && document.body.classList.contains("focus")) setFocusMode(false);
  });
  wireModels();
  const search = byId("search");
  search.addEventListener("input", () => {
    ui.filter = search.value.trim().toLocaleLowerCase(language);
    applyFilter();
  });
  stream.addEventListener("scroll", () => {
    stick = stream.scrollHeight - stream.scrollTop - stream.clientHeight < 48;
    if (stick) byId("jump").hidden = true;
  });
  byId("jump").addEventListener("click", () => {
    stick = true;
    byId("jump").hidden = true;
    stream.scrollTop = stream.scrollHeight;
  });
  byId("languageSelect").addEventListener("change", () => {
    language = byId("languageSelect").value;
    saveLanguage(language);
    applyLanguage();
  });
  byId("tokenBox").addEventListener("submit", (event) => {
    event.preventDefault();
    const input = byId("tokenInput");
    saveToken(input.value);
    input.value = "";
    client.connect();
  });
  byId("pairRenew").addEventListener("click", () => {
    renew();
  });
  setInterval(refreshCards, 500);
}
