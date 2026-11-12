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

function renderRing() {
  const state = stateKey();
  byId("ring").dataset.state = state;
  byId("right").dataset.state = state;
  const arc = byId("ringProgress");
  arc.classList.toggle("indeterminate", ui.progress === null);
  arc.setAttribute("stroke-dasharray", ui.progress === null ? "25 75" : `${ui.progress * 100} 100`);
  const text = byId("stateText");
  const top = byId("topState");
  for (const node of [text, top]) {
    if (state === "loading" && ui.progress !== null) {
      const percent = new Intl.NumberFormat(language, { style: "percent" }).format(ui.progress);
      bind(node, "state_loading_progress", { percent });
    } else {
      bind(node, `state_${state}`);
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
  const model = byId("modelText");
  if (ui.model) {
    delete model.dataset.i18n;
    model.textContent = ui.model;
  } else {
    bind(model, "model_unknown");
  }
}

function renderControls() {
  const usable = connected();
  composer.disabled = !usable;
  byId("send").disabled = !usable;
  byId("stop").disabled = !usable || (!ui.activeTurn && cards.size === 0);
  const mic = byId("micButton");
  mic.disabled = !usable;
  mic.setAttribute("aria-pressed", String(ui.mic));
  bind(byId("micValue"), ui.mic ? "toggle_on" : "toggle_off");
  refreshCards();
}

function renderStatus() {
  renderRing();
  renderConnection();
  renderControls();
  renderSetup();
  renderSettings();
}

// Kurulum: `setup` durumunda konuşma dili ve ayarın dil modelini yükleyen düğme (indirme ve katalog sonraki dilimde).
function renderSetup() {
  byId("setupBox").hidden = ui.state !== "setup";
  const model = str(ui.settings.language_model);
  const load = byId("loadButton");
  load.disabled = !connected() || model === "";
  const speech = /** @type {HTMLSelectElement} */ (byId("speechLanguage"));
  const current = str(ui.settings.speech_language);
  if (current !== "") speech.value = current;
  speech.disabled = !connected();
}

/** @param {HTMLSelectElement} select @param {{value: string, text: string}[]} items @param {string} chosen */
function fillSelect(select, items, chosen) {
  select.replaceChildren(...items.map((item) => {
    const option = /** @type {HTMLOptionElement} */ (el("option", "", item.text));
    option.value = item.value;
    return option;
  }));
  if (items.some((item) => item.value === chosen)) select.value = chosen;
}

// Ayarlar: ayar dosyası varken (`settings` geldi), kurulum dışında. Değerler sunucudan gelir; her değişiklik bir
// `settings_set`'tir, sunucu reddederse değer sunucununkine döner. Dil modeli yalnız kurulu girdilerden seçilir.
function renderSettings() {
  const shown = Object.keys(ui.settings).length > 0 && ui.state !== "setup";
  byId("settingsBox").hidden = !shown;
  const usable = connected();
  const speech = /** @type {HTMLSelectElement} */ (byId("settingsLanguage"));
  const current = str(ui.settings.speech_language);
  if (current !== "") speech.value = current;
  speech.disabled = !usable;
  const installed = loadableModels(ui.catalog);
  const modelSelect = /** @type {HTMLSelectElement} */ (byId("settingsModel"));
  fillSelect(modelSelect, installed.map((entry) => ({ value: entry.id, text: entry.name })), modelSelect.value || str(ui.settings.language_model));
  modelSelect.disabled = !usable || installed.length === 0;
  const load = byId("settingsModelLoad");
  load.hidden = installed.length === 0;
  load.disabled = modelSelect.disabled;
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
  bind(byId("toolsValue"), sheet.hidden ? "toggle_off" : "toggle_on");
  bind(byId("focusValue"), focused ? "toggle_on" : "toggle_off");
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
  byId("micButton").addEventListener("click", () => command("mic_set", { on: !ui.mic }));
  byId("loadButton").addEventListener("click", () => command("model_load", { entry_id: str(ui.settings.language_model) }));
  byId("speechLanguage").addEventListener("change", () => {
    const value = /** @type {HTMLSelectElement} */ (byId("speechLanguage")).value;
    command("settings_set", { key: "speech_language", value });
  });
  byId("settingsLanguage").addEventListener("change", () => setSetting("speech_language", /** @type {HTMLSelectElement} */ (byId("settingsLanguage")).value));
  byId("settingsModelLoad").addEventListener("click", () => command("model_load", { entry_id: /** @type {HTMLSelectElement} */ (byId("settingsModel")).value }));
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
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && document.body.classList.contains("focus")) setFocusMode(false);
  });
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
