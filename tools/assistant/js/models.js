// @ts-check
"use strict";
// Modeller penceresi: katalogun her türü kendi grubunda; satırın indirmesi, hızı ve "yavaş" uyarısı. Takımın girdileri
// (ayarın dil modeli, konuşma dilinin yazıcısı ve sesi, VAD) da buradan okunur. Ekranın modülü (index.html; sıra
// contract.md, "Sunulan dosyalar").

/** Pencerenin grupları, sırasıyla; katalogun öbür türlerini ekran yok sayar (events.js'nin catalogEntries'i). */
const MODEL_KINDS = ["language_model", "transcriber", "voice", "vad"];
const ACTIVE_DOWNLOADS = ["queued", "running", "verifying"];
const SLOW_CHECK_MS = 5000; // inen iş varken satırlar bu aralıkla yeniden çizilir: bayt gelmese de "yavaş" görünsün

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

// ---------- Satırın işi ----------
// rowStatus ve slowDownload saf fonksiyondur; Go testleri (TestDownloadRowShowsOnlyItsOwnJob,
// TestDownloadRowShowsSpeedAndSlow) bu iki işaretin arasını node ile koşar.

/**
 * Satırın durumu yalnız kendi girdisinin işinden gelir: sohbetin İndir'inin sıraya koyduğu ortak parça (yazıcı, ses,
 * VAD) kendi satırında iner. Kendi dosyaları inmiş, takımı kurulu olmayan sohbet satırı yüzdeyi kopyalamaz: takımın
 * inen parçasını, yoksa sıradaki ilkini adıyla söyler (`part`).
 * @param {any} entry @param {any[]} catalog @param {Record<string, any>} downloads @param {string} speech
 * @returns {{kind: string, job?: any, part?: string}}
 */
function rowStatus(entry, catalog, downloads, speech) {
  const active = ["queued", "running", "verifying"];
  const job = downloads[entry.id];
  if (job && (active.includes(job.state) || job.state === "failed" || job.state === "cancelled")) return { kind: job.state, job };
  if (entry.kind === "language_model" && entry.downloaded && !entry.installed) {
    const waiting = catalog.filter((e) => e.kind !== "language_model" && (e.kind === "vad" || (e.languages || []).includes(speech)))
      .filter((e) => downloads[e.id] && active.includes(downloads[e.id].state));
    const shown = waiting.find((e) => downloads[e.id].state === "running") || waiting[0];
    if (shown) return { kind: "part", part: shown.name };
  }
  return { kind: entry.downloaded ? "downloaded" : "absent" };
}

/**
 * İş yavaş mı: son 30 sn'nin hızıyla kalan süre 1 saati aşıyor ya da 30 sn'dir bayt gelmiyor (eşikler tahmindir).
 * `samples` işin örnekleridir ({at: ms, done: bayt}, eskiden yeniye); 30 sn'lik pencere dolmadan karar verilmez.
 * @param {{at: number, done: number}[]} samples @param {number} total @param {number} now
 */
function slowDownload(samples, total, now) {
  const span = 30000, limit = 3600000;
  const older = samples.filter((s) => s.at <= now - span);
  if (older.length === 0) return false;
  const base = older[older.length - 1];
  const last = samples[samples.length - 1];
  const gained = last.done - base.done;
  if (gained <= 0) return true;
  return (total - last.done) / (gained / (now - base.at)) > limit;
}
// ---------- Satırın işi sonu ----------

/**
 * İşin örnekleri `slowDownload` içindir: yalnız inerken; 30 sn'den eskilerin en yenisi pencerenin başı olarak kalır.
 * @param {any} job
 */
function noteSample(job) {
  const id = str(job.job_id);
  if (job.state !== "running") {
    delete ui.samples[id];
    return;
  }
  const now = Date.now();
  const list = ui.samples[id] || (ui.samples[id] = []);
  list.push({ at: now, done: jobBytes(job).done });
  while (list.length > 1 && list[1].at <= now - 30000) list.shift();
}

/** @param {any} job */
function jobBytes(job) {
  let done = 0, total = 0;
  for (const f of Array.isArray(job.files) ? job.files : []) { done += num(f.done_bytes) ?? 0; total += num(f.total_bytes) ?? 0; }
  return { done, total };
}

// percentOf: işin dosyalarının toplam yüzdesi, aşağı yuvarlanmış (`download_running`).
/** @param {any} job */
function percentOf(job) {
  const { done, total } = jobBytes(job);
  return total > 0 ? Math.floor((done * 100) / total) : 0;
}

/** @param {string} id */
function entryById(id) {
  return ui.catalog.find((e) => e.id === id);
}

/** Katalogdaki adı; girdi yoksa kimliği. @param {string} id */
function nameOf(id) {
  const entry = entryById(id);
  return entry ? entry.name : id;
}

/**
 * Ayarın takımındaki girdi: dil modeli ayardan, yazıcı ve ses konuşma dilinden (`languages`), VAD katalogun ilkidir.
 * @param {string} kind
 */
function teamEntry(kind) {
  if (kind === "language_model") return entryById(str(ui.settings.language_model));
  if (kind === "vad") return ui.catalog.find((e) => e.kind === "vad");
  const speech = str(ui.settings.speech_language);
  return ui.catalog.find((e) => e.kind === kind && e.languages.includes(speech));
}

/** Yazıcıda ve seste ad, kullanıldığı dillerle. @param {any} entry */
function entryLabel(entry) {
  if (entry.languages.length === 0) return entry.name;
  const names = entry.languages.map((/** @type {string} */ code) => (dictionary[code] || {}).language_name || code);
  return `${entry.name} · ${names.join(", ")}`;
}

/** İnen iş: önce süren, yoksa sıradaki ya da denetlenen ilk iş (kurulum kartı onu adıyla söyler). */
function runningJob() {
  const active = Object.values(ui.downloads).filter((j) => ACTIVE_DOWNLOADS.includes(str(j.state)));
  return active.find((j) => j.state === "running") || active[0] || null;
}

/** Makinenin belleği ikili birimle (17 179 869 184 bayt "16 GB"); öbür boylar onlu. @param {number} bytes */
function binaryBytes(bytes) {
  return unitFormat(bytes / 2 ** 30, "gigabyte", 1);
}

// Pencere: türlere göre gruplar, her birinde tek cümle açıklama; ilk sohbet modeli konuşma dilinin modellerini de indirir.
function renderModels() {
  const machine = /** @type {any} */ (ui.settings.machine);
  const memory = num(machine && machine.memory_bytes);
  const line = byId("modelsMachine");
  line.hidden = memory === null;
  if (memory !== null) bind(line, "models_machine_memory", { memory_bytes: binaryBytes(memory) });
  byId("modelsGroups").replaceChildren(...MODEL_KINDS.map((kind) => {
    const box = el("section", "box");
    box.append(bind(el("h3", "label"), `kind_${kind}`), bind(el("p", "kind-note"), `kind_${kind}_note`));
    const rows = ui.catalog.filter((e) => e.kind === kind);
    if (kind === "language_model" && !rows.some((e) => e.downloaded)) box.append(bind(el("p", "hint"), "models_first_download_note"));
    box.append(...rows.map(modelRow));
    return box;
  }));
}

// Satır (D'nin tek tık satırı): ad ve "Kullanılıyor", durum yazısı, düğmeler sağa yaslı; altında boy, bellek, uyum ve
// inerken hız. İndir ve Sürdür olumlu (`btn-go`), Sil tehlike (`btn-danger`); Sil ve "İndirildi" `downloaded`'a bağlı.
/** @param {any} entry */
function modelRow(entry) {
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
  /** @param {string} key @param {Record<string, unknown>} [values] @param {string} [extra] */
  const note = (key, values, extra) => bind(el("span", extra ? `card-note ${extra}` : "card-note"), key, values);
  // İndir ve Sürdür: biten ve duran işler unutulur.
  const download = () => {
    for (const [key, job] of Object.entries(ui.downloads)) if (!ACTIVE_DOWNLOADS.includes(str(job.state))) delete ui.downloads[key];
    command("model_download", { entry_id: entry.id });
  };
  const remove = button("settings_model_delete", "delete", () => { ui.confirmDelete = entry.id; renderModels(); });
  const cancel = (/** @type {any} */ job) => button("download_cancel", "", () => command("download_cancel", { job_id: str(job.job_id) }));
  const status = rowStatus(entry, ui.catalog, ui.downloads, str(ui.settings.speech_language));
  const meta = el("span", "m-meta");
  let text = el("span");
  let slow = null;
  /** @type {HTMLElement[]} */
  let actions = [];
  if (ui.confirmDelete === entry.id) {
    text = note("settings_model_delete_confirm", undefined, "dl-long");
    actions = [button("settings_model_delete", "delete", () => { ui.confirmDelete = ""; command("model_delete", { entry_id: entry.id }); renderModels(); }),
      button("download_cancel", "", () => { ui.confirmDelete = ""; renderModels(); })];
  } else if (status.kind === "running") {
    text = note("download_running", { percent: percentOf(status.job) });
    const speed = num(status.job.bytes_per_second);
    if (speed !== null) meta.append(bind(el("span"), "download_speed", { speed_bytes: speed }));
    if (slowDownload(ui.samples[str(status.job.job_id)] || [], jobBytes(status.job).total, Date.now())) slow = note("download_slow", undefined, "dl-long m-slow");
    actions = [cancel(status.job)];
  } else if (status.kind === "queued" || status.kind === "verifying") {
    text = note(status.kind === "queued" ? "download_queued" : "download_verifying");
    if (status.kind === "queued") actions = [cancel(status.job)];
  } else if (status.kind === "failed" || status.kind === "cancelled") {
    const error = status.job.error || {};
    text = status.kind === "failed" ? note(`error_${str(error.code)}`, error.values || {}, "dl-long") : note("download_cancelled");
    actions = [button("download_resume", "resume", download)];
  } else if (status.kind === "part") {
    text = note("model_part_downloading", { model: status.part });
    actions = [remove];
  } else if (status.kind === "downloaded") {
    text = note("model_downloaded", undefined, "dl-done");
    actions = [remove];
  } else {
    text = note("model_not_downloaded", undefined, "m-off");
    actions = [button("settings_model_download", "download", download)];
  }
  // `{bytes}` `_bytes` ile bitmez (formatValue onu sayı yazardı): boy burada birimle yazılır.
  if (entry.bytes !== null) meta.append(bind(el("span"), "model_size", { bytes: formatBytes(entry.bytes) }));
  if (entry.memory_bytes !== null) meta.append(bind(el("span"), "model_memory", { memory_bytes: entry.memory_bytes }));
  if (entry.kind === "language_model" && entry.fit !== "") meta.append(bind(el("span", `fit fit-${entry.fit}`), `fit_${entry.fit}`));
  const name = el("span", "dl-name");
  name.append(el("span", "m-name", entryLabel(entry)));
  if (entry.downloaded && teamEntry(entry.kind) === entry) name.append(bind(el("span", "tag tag-use"), "model_in_use"));
  const buttons = el("span", "dl-actions");
  buttons.append(...actions);
  const row = el("div", "dl-row m-row");
  row.append(name, text, buttons, meta);
  if (slow) row.append(slow);
  return row;
}

function openModels() {
  if (settingsDialog.open) closeSettings();
  if (modelsDialog.open) return;
  renderModels();
  modelsDialog.showModal();
  byId("modelsOpen").setAttribute("aria-expanded", "true");
}

// Modeller, ayarlar gibi kalıcı pencerededir (`<dialog>`): Esc, kapat düğmesi ve dışına tık kapar.
const modelsDialog = /** @type {HTMLDialogElement} */ (byId("modelsDialog"));

function wireModels() {
  byId("modelsOpen").addEventListener("click", openModels);
  byId("modelsClose").addEventListener("click", () => modelsDialog.close());
  modelsDialog.addEventListener("click", (event) => {
    if (event.target === modelsDialog) modelsDialog.close();
  });
  modelsDialog.addEventListener("close", () => {
    byId("modelsOpen").setAttribute("aria-expanded", "false");
    ui.confirmDelete = "";
  });
  setInterval(() => {
    if (Object.values(ui.downloads).some((j) => j.state === "running")) renderModels();
  }, SLOW_CHECK_MS);
}
