// @ts-check
"use strict";
// Olaylar: türüne göre işleyiciler, olay kimliği kuralı, SSE çerçevesinin ayrıştırılması. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

// ---------- Olaylar ----------

/** @type {Map<string, (data: any) => void>} */
const handlers = new Map();
/** @param {string} type @param {(data: any) => void} fn */
const on = (type, fn) => handlers.set(type, fn);

on("session", (d) => {
  const lost = new Map(cards);
  const snapshot = Array.isArray(d.snapshot) ? d.snapshot : [];
  for (const item of snapshot) {
    if (item && item.type === "approval_open" && item.data) lost.delete(str(item.data.approval_id));
  }
  for (const card of lost.values()) removeCard(card);
  clearStream();
  ui.model = str(d.model);
  ui.tools = (Array.isArray(d.tools) ? d.tools : []).map((tool) => ({ name: str(tool && tool.name), irreversible: Boolean(tool && tool.irreversible) }));
  ui.heartbeatMs = num(d.heartbeat_ms) ?? ui.heartbeatMs;
  ui.metrics = {};
  renderMetrics();
  renderTools();
  for (const item of snapshot) {
    const handler = item && item.type !== "session" ? handlers.get(item.type) : undefined;
    if (handler) handler(item.data || {});
  }
  for (const card of lost.values()) note("approval_lost", { tool: card.tool }, "warn");
  renderStatus();
});

on("settings", (d) => {
  ui.settings = d;
  renderSetup();
  renderSettings();
});

on("catalog", (d) => {
  ui.catalog = catalogEntries(d);
  renderSettings();
});

on("devices", (d) => {
  ui.microphones = (Array.isArray(d.microphones) ? d.microphones : []).filter((name) => typeof name === "string" && name !== "");
  renderSettings();
});

// ---------- Katalog ----------
// catalogEntries: `catalog` olayının dil modelleri (contract.md); bilinmeyen alan ve bilinmeyen `kind` yok sayılır.
/** @param {any} data @returns {{id: string, name: string, installed: boolean}[]} */
function catalogEntries(data) {
  const entries = data && Array.isArray(data.entries) ? data.entries : [];
  return entries
    .filter((entry) => entry && entry.kind === "language_model" && typeof entry.id === "string" && entry.id !== "")
    .map((entry) => ({ id: entry.id, name: typeof entry.name === "string" && entry.name !== "" ? entry.name : entry.id, installed: entry.installed === true }));
}
// loadableModels: ayar sayfasının seçebildiği modeller: yalnız kurulu olanlar ("Yükle" kurulu olmayanı göstermez).
/** @param {{id: string, name: string, installed: boolean}[]} entries */
function loadableModels(entries) {
  return entries.filter((entry) => entry.installed);
}
// ---------- Katalog sonu ----------

on("state", (d) => {
  if (STATES.includes(d.state)) ui.state = d.state;
  ui.mic = d.mic === true;
  ui.progress = num(d.progress);
  renderStatus();
});

on("turn_start", (d) => {
  const id = str(d.turn_id);
  turnOf(id, str(d.source));
  ui.activeTurn = id;
  if (ui.recordingNote) {
    ui.recordingNote = false; // kayıt bu turla başladı
    renderSettings();
  }
  renderControls();
});

on("turn_text", (d) => {
  const role = d.role === "user" ? "user" : "assistant";
  const turn = turnOf(str(d.turn_id));
  const msg = messageOf(turn, role);
  writeText(msg, str(d.text), d.mode === "replace");
  if (role === "user") msg.row.classList.toggle("draft", d.final === false);
});

on("turn_end", (d) => {
  const id = str(d.turn_id);
  const turn = turns.get(id);
  if (turn) {
    closeAssistant(turn);
    turns.delete(id);
  }
  if (ui.activeTurn === id) ui.activeTurn = "";
  if (d.reason === "stopped") note("reason_stopped");
  if (d.reason === "error") {
    if (d.error) noteError(d.error);
    else note("reason_error", undefined, "error");
  }
  renderControls();
});

on("tool_start", (d) => {
  const turn = turns.get(str(d.turn_id));
  if (turn) closeAssistant(turn); // araçtan sonraki cevap satırı aracın altına düşer
  toolRowOf(str(d.call_id), visible(str(d.tool)));
});

on("tool_end", (d) => {
  const id = str(d.call_id);
  const entry = toolRowOf(id, visible(str(d.tool)));
  const result = RESULTS.includes(d.result) ? d.result : "error";
  entry.row.dataset.result = result;
  entry.mark.textContent = RESULT_MARKS[result];
  bind(entry.result, `result_${result}`);
  const ms = num(d.duration_ms);
  if (ms !== null) bind(entry.time, "duration_value", { duration_ms: ms });
  const output = str(d.output);
  if (output || d.truncated === true) addOutput(entry, output, d);
  if (d.error) noteError(d.error);
  toolRows.delete(id);
  rememberCall(entry.tool, result, ms);
  follow();
});

on("approval_open", (d) => {
  const entry = toolRows.get(str(d.call_id));
  if (entry) bind(entry.result, "result_waiting"); // onaylanmayan araç çalışmıyor: satır bunu söyler
  openCard(d);
});

on("approval_close", (d) => {
  const card = cards.get(str(d.approval_id));
  if (!card) return;
  removeCard(card);
  const values = { tool: card.tool };
  if (d.reason === "answered") {
    const who = card.answered ? "" : "other_";
    note(`approval_closed_${who}${d.decision === "approve" ? "approved" : "denied"}`, values);
  } else if (d.reason === "expired" || d.reason === "stopped") {
    note(`approval_closed_${d.reason}`, values, "warn");
  }
});

on("metrics", (d) => {
  Object.assign(ui.metrics, d);
  renderMetrics();
});

on("error", (d) => {
  noteError(d);
  if (d.fatal === true) {
    ui.state = "error";
    renderStatus();
  }
});

on("heartbeat", () => {});

/** @param {{id: string, data: string}} frame */
function receive(frame) {
  let event;
  try {
    event = JSON.parse(frame.data);
  } catch {
    noteError({ code: "bad_event" });
    return;
  }
  if (!event || typeof event !== "object") {
    noteError({ code: "bad_event" });
    return;
  }
  if (event.v !== PROTOCOL_VERSION) {
    dropToken();
    client.refuse("mismatch", { got: String(event.v), want: PROTOCOL_VERSION });
    return;
  }
  if (event.type === "heartbeat") return;
  const step = eventStep(ui.lastId, frame.id, event.type);
  ui.lastId = step.lastId;
  if (step.reconnect) client.retry();
  if (!step.apply) return;
  const handler = handlers.get(event.type);
  if (handler) handler(event.data && typeof event.data === "object" ? event.data : {});
}

// ---------- Olay kimliği ----------
// splitId ve eventStep saf fonksiyondur; Go testi (TestEventIdOrder) bu iki işaretin arasını node ile koşar.

/**
 * Kimlik `<ön ek>-<sıra>` metnidir (contract.md, "Kopma ve yeniden bağlanma"): olduğu gibi saklanır ve geri
 * gönderilir, ön ek üretilmez; karşılaştırmada yalnız son tireden sonraki sıra sayıdır.
 * @param {string} id @returns {{prefix: string, seq: bigint}|null}
 */
function splitId(id) {
  const dash = id.lastIndexOf("-");
  const digits = id.slice(dash + 1);
  if (dash <= 0 || !/^[0-9]{1,19}$/.test(digits)) return null;
  return { prefix: id.slice(0, dash), seq: BigInt(digits) };
}

/**
 * Bir olayın kimliğine göre: uygulanır mı, son kimlik ne olur, bağlantı yenilenir mi. `session` hiç atılmaz ve son
 * kimliği kendi kimliğine kurar (ön eki farklı, sırası küçük olsa da). Öbür olay son kimlikle aynı ön eki ve daha
 * büyük sırayı taşımalıdır: sırası büyük değilse atılır; ön eki farklıysa ya da son kimlik yoksa (ilk olay `session`
 * değil) uygulanmaz, son kimlik silinir ve ekran `Last-Event-ID`'siz yeniden bağlanır.
 * @param {string|null} lastId @param {string} id @param {string} type
 * @returns {{apply: boolean, lastId: string|null, reconnect: boolean}}
 */
function eventStep(lastId, id, type) {
  if (type === "session") return { apply: true, lastId: id, reconnect: false };
  const last = lastId === null ? null : splitId(lastId);
  const next = splitId(id);
  if (last === null || next === null || next.prefix !== last.prefix) return { apply: false, lastId: null, reconnect: true };
  if (next.seq <= last.seq) return { apply: false, lastId, reconnect: false };
  return { apply: true, lastId: id, reconnect: false };
}
// ---------- Olay kimliği sonu ----------

// ---------- SSE ----------

/** @param {(frame: {id: string, data: string}) => void} onFrame @param {() => void} onOverflow */
function createParser(onFrame, onOverflow) {
  const decoder = new TextDecoder("utf-8");
  let buffer = "";
  let id = "";
  let data = /** @type {string[]} */ ([]);
  let size = 0;
  /** @param {string} line */
  function field(line) {
    if (line === "") {
      if (data.length > 0) onFrame({ id, data: data.join("\n") });
      id = "";
      data = [];
      size = 0;
      return;
    }
    if (line.startsWith(":")) return;
    const colon = line.indexOf(":");
    const name = colon < 0 ? line : line.slice(0, colon);
    const value = colon < 0 ? "" : line.slice(colon + 1).replace(/^ /, "");
    if (name === "id") id = value;
    if (name === "data") {
      data.push(value);
      size += value.length;
    }
  }
  return {
    /** @param {Uint8Array} bytes */
    push(bytes) {
      buffer += decoder.decode(bytes, { stream: true });
      const breaks = /\r\n|\n|\r(?!$)/g;
      let start = 0;
      let match;
      while ((match = breaks.exec(buffer)) !== null) {
        field(buffer.slice(start, match.index));
        start = breaks.lastIndex;
      }
      buffer = buffer.slice(start);
      if (buffer.length + size > MAX_EVENT_BYTES) onOverflow();
    },
  };
}
