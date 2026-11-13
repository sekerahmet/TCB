// @ts-check
"use strict";
// Ekranın sabitleri, eşleme kodunun adresten okunması, dil ve sözlük, ekranın durumu. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

/** Ekranda görünen ad: yalnız burada yazılır. */
const ASSISTANT_NAME = "Dost";

/**
 * Eşleme kodu (contract.md, "Kaynak"; dil tanımı 28.2b): sunucu tarayıcıyı `#p=<kod>` ile açar. Kod yalnız bu biçimle
 * okunur ve her şeyden önce adresten silinir; değer DOM'a, metne, günlüğe girmez. Başka parça yok sayılır.
 */
const PAIR_CODE = (() => {
  const match = /^#p=([A-Za-z0-9_-]{43})$/.exec(location.hash);
  if (location.hash !== "") history.replaceState(null, "", location.pathname);
  return match ? match[1] : "";
})();

const PROTOCOL_VERSION = 2;
const MAX_EVENT_BYTES = 1024 * 1024; // contract.md: bir olayın sınırı
const MAX_ROWS = 400; // akışta en çok satır: uzun oturum sekmeyi doldurmaz
const MAX_TEXT_CHARS = 120000; // bir konuşma satırında en çok karakter
const MAX_OUTPUT_CHARS = 24000; // bir araç çıktısından gösterilen en çok karakter
const MAX_RECENT_CALLS = 8;
const MAX_VALUE_CHARS = 200; // hata değerinin ekranda en uzun hali
const RETRY_LIMIT_MS = 10000;
const APPROVE_ARM_MS = 500; // kart açılınca Onayla bu kadar kapalı kalır: kör tıklama onaylamaz
const STATES = ["setup", "idle", "loading", "listening", "thinking", "speaking", "error"];
const RESULTS = ["ok", "error", "denied", "stopped"];
const RESULT_MARKS = { running: "…", ok: "✓", error: "✕", denied: "⊘", stopped: "■" };
const LANGUAGE_KEY = "assistant.language";

/** @param {string} id @returns {any} */
const byId = (id) => document.getElementById(id);
/** @param {unknown} v */
const str = (v) => (typeof v === "string" ? v : "");
/** @param {unknown} v */
const num = (v) => (typeof v === "number" && Number.isFinite(v) ? v : null);

/**
 * Denetim, yön ve görünmez karakterleri kod noktasıyla yazar. Kartın ve araç adının gösterdiği metin okunanla aynı
 * olmalı: U+202E bir dosya adını ters gösterebilir, onay da yanlış şeye verilir.
 * @param {string} text
 */
const visible = (text) => text.replace(
  /[\u0000-\u0008\u000b-\u001f\u007f-\u009f\u00ad\u061c\u200b-\u200f\u2028-\u202e\u2060-\u206f\ufeff]/g,
  (c) => `[U+${c.charCodeAt(0).toString(16).toUpperCase().padStart(4, "0")}]`);

// ---------- Dil ----------

/**
 * Dil dosyaları burayı doldurur (`lang/<kod>.js`). Bu betikten sonra yüklenirler; ekran sayfa hazır olunca başlar.
 * @type {Record<string, Record<string, string>>}
 */
const dictionary = {};
/** @type {any} */ (window).assistantLanguages = dictionary;
let languages = /** @type {string[]} */ ([]);
/** @type {WeakMap<Element, Record<string, unknown>>} */
const boundValues = new WeakMap();

function savedLanguage() {
  try {
    return localStorage.getItem(LANGUAGE_KEY);
  } catch {
    return null;
  }
}

/** @param {string} value */
function saveLanguage(value) {
  try {
    localStorage.setItem(LANGUAGE_KEY, value);
  } catch {
    // Tercih yalnız bu oturumda kalır.
  }
}

/** Seçim hatırlanır; yoksa tarayıcının dili; o da yoksa listedeki ilk dil. Hiç dil yüklenmediyse dil belirsizdir. */
function pickLanguage() {
  const saved = savedLanguage();
  if (saved && dictionary[saved]) return saved;
  for (const tag of navigator.languages || []) {
    const base = tag.toLowerCase().split("-")[0];
    if (dictionary[base]) return base;
  }
  return languages[0] || "und";
}

let language = "und";

/** @param {number} value @param {string} unit @param {number} digits */
function unitFormat(value, unit, digits) {
  return new Intl.NumberFormat(language, { style: "unit", unit, maximumFractionDigits: digits }).format(value);
}

/** @param {number} ms */
function formatDuration(ms) {
  return ms < 1000 ? unitFormat(ms, "millisecond", 0) : unitFormat(ms / 1000, "second", 1);
}

/** @param {number} bytes */
function formatBytes(bytes) {
  if (bytes >= 1e9) return unitFormat(bytes / 1e9, "gigabyte", 2);
  if (bytes >= 1e6) return unitFormat(bytes / 1e6, "megabyte", 1);
  if (bytes >= 1e3) return unitFormat(bytes / 1e3, "kilobyte", 1);
  return unitFormat(bytes, "byte", 0);
}

/** Sözleşme: `_ms` süre, `_bytes` boyuttur; öbür sayılar dile göre yazılır. @param {string} name @param {unknown} value */
function formatValue(name, value) {
  if (typeof value === "number") {
    if (name.endsWith("_ms")) return formatDuration(value);
    if (name.endsWith("_bytes")) return formatBytes(value);
    return new Intl.NumberFormat(language).format(value);
  }
  return String(value).slice(0, MAX_VALUE_CHARS);
}

/** @param {string} key @param {Record<string, unknown>} [values] */
function t(key, values) {
  const text = (dictionary[language] || {})[key];
  if (text === undefined) return key;
  return text.replace(/\{(\w+)\}/g, (whole, name) =>
    values && Object.hasOwn(values, name) ? formatValue(name, values[name]) : whole);
}

/** @param {HTMLElement} node */
function paint(node) {
  const key = node.dataset.i18n || "";
  const text = t(key, boundValues.get(node));
  const attr = node.dataset.i18nAttr;
  if (attr) node.setAttribute(attr, text);
  else node.textContent = text;
}

/**
 * Düğüm dile bağlanır: dil değişince kendi kendine yeniden yazılır.
 * @template {HTMLElement} T
 * @param {T} node @param {string} key @param {Record<string, unknown>} [values] @returns {T}
 */
function bind(node, key, values) {
  node.dataset.i18n = key;
  if (values) boundValues.set(node, values);
  paint(node);
  return node;
}

/** @param {string} tag @param {string} [className] @param {string} [text] */
function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

// ---------- Ekranın durumu ----------

const ui = {
  state: "idle",
  mic: false,
  progress: /** @type {number|null} */ (null),
  conn: "connecting", // connecting, open, reconnecting, auth, forbidden, rejected, mismatch, pair_invalid, pair_used, lost
  connValues: /** @type {Record<string, unknown>} */ ({}),
  model: "",
  tools: /** @type {{name: string, irreversible: boolean}[]} */ ([]),
  activeTurn: "",
  lastId: /** @type {string|null} */ (null),
  heartbeatMs: 15000,
  metrics: /** @type {Record<string, unknown>} */ ({}),
  settings: /** @type {Record<string, unknown>} */ ({}),
  catalog: /** @type {{id: string, name: string, installed: boolean}[]} */ ([]),
  microphones: /** @type {string[]|null} */ (null), // `devices`'in listesi; gelmediyse null
  devicesAsked: false,
  recordingNote: false, // kayıt bu sayfadan açıldı, bir sonraki tur başlamadı
  calls: /** @type {{tool: string, result: string, duration_ms: number|null}[]} */ ([]),
  droppedRows: 0,
  filter: "",
  token: "",
};

/**
 * Belirteç sekmenin oturum belleğinde (contract.md): anahtar kapıya bağlı, içinde yalnız belirteç; kalıcı depoya
 * (`localStorage`, çerez) yazılmaz; `401`'de ve sürüm uyuşmazlığında silinir. Depo kapalıysa belirteç bellekte kalır.
 */
const TOKEN_KEY = `dost-token-${location.port}`;
function loadToken() {
  try {
    return sessionStorage.getItem(TOKEN_KEY) || "";
  } catch {
    return "";
  }
}
/** @param {string} token */
function saveToken(token) {
  ui.token = token;
  try {
    sessionStorage.setItem(TOKEN_KEY, token);
  } catch {
    // depo kapalı: belirteç yalnız bellekte
  }
}
function dropToken() {
  ui.token = "";
  try {
    sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    // depo kapalı
  }
}

const stream = byId("stream");
const composer = byId("composer");
const cardsBox = byId("approvals");
/** @type {Map<string, any>} */
const turns = new Map();
/** @type {Map<string, any>} */
const toolRows = new Map();
/** @type {Map<string, any>} */
const cards = new Map();

const connected = () => ui.conn === "open";
