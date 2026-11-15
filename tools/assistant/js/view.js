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
    if (state === "loading" && ui.verifying) {
      bind(node, "state_loading_verifying");
    } else if (state === "loading" && ui.progress !== null) {
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

// Kurulum: `setup` durumunda konuşma dili, ayarın dil modelini yükleyen düğme ve dil modellerinin indirme satırları.
function renderSetup() {
  byId("setupBox").hidden = ui.state !== "setup";
  const model = str(ui.settings.language_model);
  const load = byId("loadButton");
  load.disabled = !connected() || model === "";
  const speech = /** @type {HTMLSelectElement} */ (byId("speechLanguage"));
  const current = str(ui.settings.speech_language);
  if (current !== "") speech.value = current;
  speech.disabled = !connected();
  renderDownloads();
}

// İndirme düğmelerinin simgeleri: renkten başka iz (F1b): İndir aşağı ok, Sürdür dönüş oku, Sil çöp kutusu.
const DOWNLOAD_ICONS = {
  download: "M12 4v11M7 10l5 5 5-5M5 20h14",
  resume: "M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5",
  delete: "M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3",
};

/** @param {string} d */
function svgIcon(d) {
  const ns = /** @type {Element} */ (document.querySelector("svg")).namespaceURI; // sayfanın SVG'lerinin ad alanı
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("class", "icon");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  const path = document.createElementNS(ns, "path");
  path.setAttribute("d", d);
  svg.append(path);
  return svg;
}

// renderDownloads: katalogun her dil modeli için bir satır (tek tık D): ad, durum yazısı, düğmeler. Düğmeler her satırda
// sağa yaslı, adla dikey ortalı; kısa durum ("İndiriliyor %43") adın satırında, uzun yazı (hata, silme onayı) altında
// tam genişlikte (`dl-long`). İndir ve Sürdür olumlu (`btn-go`), Sil tehlike (`btn-danger`) düğmesidir; Vazgeç sıradan.
// Satır takımın işlerini gösterir: dil modelinin kendi işi ve konuşma modellerinin işleri (`rowJobs`); yüzde onların
// toplamı, Vazgeç hepsini durdurur. Ayarlar paneli `setup`'ta gizli olduğundan kap orada kurulum kutusuna geçer: hiç
// modeli olmayan kullanıcı ilk modeli oradan indirir.
function renderDownloads() {
  const box = byId("settingsDownload");
  const anchor = ui.state === "setup" ? byId("loadButton") : /** @type {HTMLElement} */ (byId("settingsModel").closest(".field"));
  if (box.previousElementSibling !== anchor) anchor.after(box);
  const usable = connected();
  /** @param {string} key @param {string} kind @param {() => void} act */
  const button = (key, kind, act) => {
    const b = /** @type {HTMLButtonElement} */ (el("button", kind === "delete" ? "btn btn-small btn-danger" : kind ? "btn btn-small btn-go" : "btn btn-small"));
    b.type = "button";
    b.disabled = !usable;
    if (kind) b.append(svgIcon(DOWNLOAD_ICONS[/** @type {"download"|"resume"|"delete"} */ (kind)]));
    b.append(bind(el("span"), key));
    b.addEventListener("click", act);
    return b;
  };
  /** @param {string} key @param {Record<string, unknown>} [values] @param {boolean} [long] */
  const note = (key, values, long) => bind(el("span", long ? "card-note dl-long" : "card-note"), key, values);
  // İndir ve Sürdür: biten ve duran işler unutulur, yüzde yeni işlerden hesaplanır.
  const download = (/** @type {string} */ id) => () => {
    for (const [key, job] of Object.entries(ui.downloads)) if (!ACTIVE_DOWNLOADS.includes(str(job.state))) delete ui.downloads[key];
    command("model_download", { entry_id: id });
  };
  box.replaceChildren(...ui.catalog.map((entry) => {
    const jobs = rowJobs(entry.id);
    const active = jobs.filter((j) => ACTIVE_DOWNLOADS.includes(str(j.state)));
    const stopped = jobs.find((j) => j.state === "failed" || j.state === "cancelled");
    let status = el("span");
    /** @type {HTMLElement[]} */
    let actions = [];
    if (ui.confirmDelete === entry.id) {
      status = note("settings_model_delete_confirm", undefined, true);
      actions = [button("settings_model_delete", "delete", () => { ui.confirmDelete = ""; command("model_delete", { entry_id: entry.id }); renderDownloads(); }),
        button("download_cancel", "", () => { ui.confirmDelete = ""; renderDownloads(); })];
    } else if (entry.installed) {
      actions = [button("settings_model_delete", "delete", () => { ui.confirmDelete = entry.id; renderDownloads(); })];
    } else if (active.length > 0) {
      const states = active.map((j) => str(j.state));
      const counted = jobs.filter((j) => j.state !== "failed" && j.state !== "cancelled");
      status = states.includes("running") ? note("download_running", { percent: percentOf(counted) })
        : states.includes("verifying") ? note("download_verifying") : note("download_queued");
      const stoppable = active.filter((j) => j.state !== "verifying");
      if (stoppable.length > 0) {
        actions = [button("download_cancel", "", () => { for (const j of stoppable) command("download_cancel", { job_id: str(j.job_id) }); })];
      }
    } else if (stopped) {
      const error = stopped.error || {};
      status = stopped.state === "failed" ? note(`error_${str(error.code)}`, error.values || {}, true) : note("download_cancelled");
      actions = [button("download_resume", "resume", download(entry.id))];
    } else {
      actions = [button("settings_model_download", "download", download(entry.id))];
    }
    const row = el("div", "dl-row");
    const buttons = el("span", "dl-actions");
    buttons.append(...actions);
    row.append(el("span", "dl-name", entry.name), status, buttons);
    return row;
  }));
}

const ACTIVE_DOWNLOADS = ["queued", "running", "verifying"];

// rowJobs: bir dil modelinin satırının işleri: kendi işi ve katalogda dil modeli olmayan girdilerin (konuşma modelleri)
// işleri; `model_download` onları dil modeliyle birlikte sıraya koyar (contract.md).
/** @param {string} id @returns {any[]} */
function rowJobs(id) {
  const models = new Set(ui.catalog.map((e) => e.id));
  return Object.values(ui.downloads).filter((j) => j.entry_id === id || !models.has(str(j.entry_id)));
}

// percentOf: işlerin dosyalarının toplam yüzdesi, aşağı yuvarlanmış (`download_running`).
/** @param {any[]} jobs */
function percentOf(jobs) {
  let done = 0, total = 0;
  for (const job of jobs) {
    for (const f of Array.isArray(job.files) ? job.files : []) { done += num(f.done_bytes) ?? 0; total += num(f.total_bytes) ?? 0; }
  }
  return total > 0 ? Math.floor((done * 100) / total) : 0;
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
  fillSelect(modelSelect, installed.map((entry) => ({ value: entry.id, text: entry.name })), modelSelect.value || str(ui.settings.language_model));
  modelSelect.disabled = !usable || installed.length === 0;
  const load = byId("settingsModelLoad");
  load.hidden = installed.length === 0;
  load.disabled = modelSelect.disabled;
  renderDownloads();
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

function openSettings() {
  if (settingsDialog.open) return;
  byId("settingsError").hidden = true;
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
  byId("settingsOpen").addEventListener("click", openSettings);
  byId("settingsClose").addEventListener("click", closeSettings);
  settingsDialog.addEventListener("click", (event) => {
    if (event.target === settingsDialog) closeSettings();
  });
  settingsDialog.addEventListener("close", () => {
    byId("settingsOpen").setAttribute("aria-expanded", "false");
    byId("settingsError").hidden = true;
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !settingsDialog.open && document.body.classList.contains("focus")) setFocusMode(false);
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
