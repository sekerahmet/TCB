// @ts-check
"use strict";
// Akış satırları: konuşma, araç ve not satırları, arama, satır sınırı. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

// ---------- Akış satırları ----------

let stick = true;
let followPending = false;

function follow() {
  if (!stick) {
    byId("jump").hidden = false;
    return;
  }
  if (followPending) return;
  followPending = true;
  requestAnimationFrame(() => {
    followPending = false;
    stream.scrollTop = stream.scrollHeight;
  });
}

/** @param {HTMLElement} row */
function matchesFilter(row) {
  return !ui.filter || (row.textContent || "").toLocaleLowerCase(language).includes(ui.filter);
}

function renderSearchCount() {
  const count = byId("searchCount");
  count.hidden = !ui.filter;
  if (!ui.filter) return;
  const rows = Array.from(stream.children);
  const shown = rows.filter((row) => !(/** @type {HTMLElement} */ (row).hidden)).length;
  bind(count, "search_count", { shown, total: rows.length });
}

function applyFilter() {
  for (const row of stream.children) {
    const item = /** @type {HTMLElement} */ (row);
    item.hidden = !matchesFilter(item);
  }
  renderSearchCount();
}

/** @param {HTMLElement} row */
function addRow(row) {
  stream.append(row);
  row.hidden = !matchesFilter(row);
  while (stream.childElementCount > MAX_ROWS) {
    stream.firstElementChild.remove();
    ui.droppedRows += 1;
  }
  const dropped = byId("dropped");
  dropped.hidden = ui.droppedRows === 0;
  if (ui.droppedRows > 0) bind(dropped, "rows_dropped", { count: ui.droppedRows });
  byId("streamEmpty").hidden = true;
  if (ui.filter) renderSearchCount();
  follow();
}

/** @param {string} key @param {Record<string, unknown>} [values] @param {string} [kind] */
function note(key, values, kind) {
  const row = el("div", "row note");
  if (kind) row.classList.add(`notice-${kind}`);
  addRow(bind(row, key, values));
}

/** @param {any} error */
function errorKey(error) {
  const code = str(error && error.code);
  // Yükleme başarılı ama ayar yazılamadı: model yüklü kaldı, ileti sonucu söyler (contract.md, `settings_write_failed`).
  if (code === "settings_write_failed" && error.values && error.values.command === "model_load") {
    return { key: "settings_write_failed_load_note", values: error.values };
  }
  const key = `error_${code}`;
  return t(key) !== key ? { key, values: (error && error.values) || {} } : { key: "error_unknown", values: { code } };
}

/** @param {any} error */
function noteError(error) {
  const { key, values } = errorKey(error);
  note(key, values, "error");
  settingsErrorNote(key, values);
}

function clearStream() {
  stream.replaceChildren();
  turns.clear();
  toolRows.clear();
  ui.droppedRows = 0;
  ui.calls = [];
  ui.activeTurn = "";
  byId("dropped").hidden = true;
  byId("streamEmpty").hidden = false;
  renderCalls();
  if (ui.filter) renderSearchCount();
}

/** @param {string} id @param {string} [source] */
function turnOf(id, source) {
  let turn = turns.get(id);
  if (!turn) {
    turn = { source: source || "text", user: null, assistant: null };
    turns.set(id, turn);
  }
  return turn;
}

/** @param {any} turn @param {string} role */
function messageOf(turn, role) {
  if (turn[role]) return turn[role];
  const row = el("article", `row msg ${role}`);
  const head = el("div", "msg-head");
  if (role === "user") {
    head.append(bind(el("span"), "speaker_user"));
    if (turn.source === "voice") head.append(" · ", bind(el("span"), "source_voice"));
  } else {
    head.textContent = ASSISTANT_NAME;
  }
  const body = el("div", "msg-text");
  body.dir = "auto";
  const textNode = document.createTextNode("");
  body.append(textNode);
  row.append(head, body);
  if (role === "assistant") row.classList.add("streaming");
  addRow(row);
  turn[role] = { row, textNode, length: 0, cut: 0, cutNote: null };
  return turn[role];
}

/** Ekran sınırını aşan metin kesilir ve söylenir. @param {any} msg @param {string} text @param {boolean} replace */
function writeText(msg, text, replace) {
  if (replace) {
    msg.textNode.data = "";
    msg.length = 0;
    msg.cut = 0;
  }
  const room = MAX_TEXT_CHARS - msg.length;
  const part = text.length > room ? text.slice(0, Math.max(room, 0)) : text;
  msg.textNode.appendData(part);
  msg.length += part.length;
  msg.cut += text.length - part.length;
  if (msg.cut > 0) {
    if (!msg.cutNote) {
      msg.cutNote = el("p", "text-cut");
      msg.row.append(msg.cutNote);
    }
    bind(msg.cutNote, "text_cut", { count: msg.cut });
  } else if (msg.cutNote) {
    msg.cutNote.remove();
    msg.cutNote = null;
  }
  follow();
}

/** @param {any} turn */
function closeAssistant(turn) {
  if (turn.assistant) turn.assistant.row.classList.remove("streaming");
  turn.assistant = null;
}

/** @param {string} tool @param {string} result @param {number|null} ms */
function rememberCall(tool, result, ms) {
  ui.calls.unshift({ tool, result, duration_ms: ms });
  ui.calls.length = Math.min(ui.calls.length, MAX_RECENT_CALLS);
  renderCalls();
}

function renderCalls() {
  const list = byId("calls");
  list.replaceChildren(...ui.calls.map((call) => {
    const item = el("li");
    const info = el("span", "call-info");
    bind(info, `result_${call.result}`);
    item.append(el("span", "call-name", `${RESULT_MARKS[call.result]} ${call.tool}`), info);
    if (call.duration_ms !== null) {
      item.append(" · ", bind(el("span", "call-info"), "duration_value", { duration_ms: call.duration_ms }));
    }
    return item;
  }));
  byId("callsEmpty").hidden = ui.calls.length > 0;
}

/** @param {string} id @param {string} tool */
function toolRowOf(id, tool) {
  let entry = toolRows.get(id);
  if (entry) return entry;
  const row = el("div", "row tool");
  const mark = el("span", "tool-mark", RESULT_MARKS.running);
  mark.setAttribute("aria-hidden", "true");
  const name = el("span", "tool-name", tool);
  const result = bind(el("span", "tool-result"), "result_running");
  const time = el("span", "tool-time");
  row.append(mark, name, result, time);
  row.dataset.result = "running";
  addRow(row);
  entry = { row, mark, result, time, tool };
  toolRows.set(id, entry);
  return entry;
}

/** @param {any} entry @param {string} output @param {any} d */
function addOutput(entry, output, d) {
  const box = /** @type {HTMLDetailsElement} */ (el("details", "out"));
  const summary = bind(el("summary"), "tool_output");
  const pre = el("pre", "", output.slice(0, MAX_OUTPUT_CHARS));
  pre.dir = "auto";
  box.append(summary, pre);
  if (output.length > MAX_OUTPUT_CHARS) {
    box.append(bind(el("p", "notice-warn"), "tool_display_cut"));
  }
  if (d.truncated === true) {
    const total = num(d.output_bytes) ?? output.length;
    box.append(bind(el("p", "notice-warn"), "tool_truncated", { shown_bytes: new TextEncoder().encode(output).length, total_bytes: total }));
  }
  entry.row.append(box);
}
