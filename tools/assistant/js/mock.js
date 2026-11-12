// @ts-check
"use strict";
// Sahte kaynak (sahte kip, `file:`): gerçek sunucuyla aynı baytları üretir, sahneler. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

/**
 * Sahte kaynak: gerçek sunucuyla aynı baytları üretir, aynı ayrıştırıcıdan geçer. Ekran kodu ikisini ayırmaz;
 * yalnız üstteki sahne seçicisi `scenes` alanına bakar.
 */
function createMockTransport() {
  const HEARTBEAT_MS = 2000;
  const CANCELLED = Symbol("cancelled");
  const encoder = new TextEncoder();
  let sessionNumber = 1;
  let prefix = newId(); // yayıncının ön eki: sunucu her açılışta yeniden üretir
  let seq = 0;
  let frames = /** @type {{seq: number, text: string}[]} */ ([]);
  let log = /** @type {{type: string, data: any}[]} */ ([]);
  let sink = /** @type {any} */ (null);
  let heartbeatTimer = 0;
  let muted = false;
  let down = false;
  let runToken = 0;
  let turnCount = 0;
  let callCount = 0;
  let approvalCount = 0;
  let activeTurn = "";
  let runningCall = "";
  let current = { state: "idle", mic: false };
  let lastMetrics = /** @type {any} */ (null);
  /** @type {Map<string, {resolve: (value: string) => void}>} */
  const open = new Map();
  const closedApprovals = new Set();
  /** @type {Map<string, any>} */
  const answers = new Map();

  /** Çerçeve iki parçada gelir: ayrıştırıcı bölünmüş UTF-8'i de görsün. @param {string} text */
  function deliver(text) {
    if (!sink) return;
    const bytes = encoder.encode(text);
    const half = Math.floor(bytes.length / 2);
    sink.chunk(bytes.subarray(0, half));
    if (sink) sink.chunk(bytes.subarray(half));
  }

  /** @param {string} type @param {any} data */
  function remember(type, data) {
    if (type === "state") current = { state: data.state, mic: data.mic };
    if (type === "metrics") lastMetrics = data;
    if (["turn_start", "turn_text", "turn_end", "tool_start", "tool_end", "approval_open"].includes(type)) {
      log.push({ type, data });
      if (log.length > 1500) log.shift();
    }
    if (type === "approval_close") {
      log = log.filter((item) => !(item.type === "approval_open" && item.data.approval_id === data.approval_id));
    }
  }

  /** @param {string} type @param {any} [data] */
  function emit(type, data) {
    if (type === "heartbeat") {
      if (!muted) deliver(`data: ${JSON.stringify({ v: PROTOCOL_VERSION, type })}\n\n`);
      return;
    }
    if (type === "session") {
      // Görüntü saklanan bir olay değildir: kimliği son olayınkidir (olay yoksa `<ön ek>-0`).
      deliver(`id: ${prefix}-${seq}\ndata: ${JSON.stringify({ v: PROTOCOL_VERSION, type, data })}\n\n`);
      return;
    }
    seq += 1;
    const text = `id: ${prefix}-${seq}\ndata: ${JSON.stringify({ v: PROTOCOL_VERSION, type, data })}\n\n`;
    frames.push({ seq, text });
    if (frames.length > 4096) frames.shift();
    remember(type, data);
    deliver(text);
  }

  function sendSession() {
    const snapshot = [{ type: "state", data: current }, ...log];
    if (lastMetrics) snapshot.push({ type: "metrics", data: lastMetrics });
    emit("session", {
      session_id: `mock-${sessionNumber}`,
      server_version: "0.0.0-mock",
      model: t("mock_model"),
      tools: [
        { name: "example_read_note", irreversible: false },
        { name: "example_delete_file", irreversible: true },
      ],
      heartbeat_ms: HEARTBEAT_MS,
      snapshot,
    });
  }

  /** @param {any} s */
  function closeStream(s) {
    if (sink === s) sink = null;
    clearInterval(heartbeatTimer);
  }

  function drop() {
    const s = sink;
    closeStream(s);
    if (s) s.closed(0);
  }

  /** @param {string|null} lastId @param {any} s */
  function attach(lastId, s) {
    if (down) {
      s.closed(0);
      return;
    }
    muted = false;
    sink = s;
    s.opened();
    const asked = lastId === null ? null : splitId(lastId);
    const last = asked === null || asked.prefix !== prefix ? -1 : Number(asked.seq);
    const oldest = frames.length > 0 ? frames[0].seq - 1 : seq;
    if (last > 0 && last >= oldest && last <= seq) {
      for (const frame of frames) {
        if (frame.seq > last) deliver(frame.text);
      }
    } else {
      sendSession();
    }
    clearInterval(heartbeatTimer);
    heartbeatTimer = window.setInterval(() => emit("heartbeat"), HEARTBEAT_MS);
  }

  /** @param {string} state @param {any} [extra] */
  const setState = (state, extra) => emit("state", { state, mic: current.mic, ...extra });
  /** @type {Record<string, unknown>} */
  const mockSettings = { language_model: "mock-model", speech_language: "tr", system_prompt: { source: "catalog" }, context: 8192, device: "cpu", port: 47863, machine: { memory_bytes: 17179869184, memory_source: "option" } };

  /** @param {string} source */
  function newTurn(source) {
    turnCount += 1;
    activeTurn = `t-${turnCount}`;
    emit("turn_start", { turn_id: activeTurn, source });
  }

  /** @param {string} text */
  function userSays(text) {
    emit("turn_text", { turn_id: activeTurn, role: "user", mode: "replace", text, final: true });
  }

  /** @param {string} reason @param {any} [error] */
  function endTurn(reason, error) {
    if (!activeTurn) return;
    emit("turn_end", error ? { turn_id: activeTurn, reason, error } : { turn_id: activeTurn, reason });
    activeTurn = "";
  }

  /** @param {(ms: number) => Promise<void>} wait @param {string} text @param {number} [step] @param {number} [pause] */
  async function say(wait, text, step = 2, pause = 60) {
    const words = text.split(/(?<=\s)/);
    for (let i = 0; i < words.length; i += step) {
      emit("turn_text", { turn_id: activeTurn, role: "assistant", mode: "append", text: words.slice(i, i + step).join("") });
      await wait(pause);
    }
  }

  /** @param {string} tool */
  function startTool(tool) {
    callCount += 1;
    runningCall = `c-${callCount}`;
    emit("tool_start", { call_id: runningCall, turn_id: activeTurn, tool });
    return runningCall;
  }

  /** @param {string} result @param {number} ms @param {string} output @param {any} [extra] */
  function endTool(result, ms, output, extra) {
    const output_bytes = encoder.encode(output).length;
    emit("tool_end", { call_id: runningCall, result, duration_ms: ms, output, output_bytes, truncated: false, ...extra });
    runningCall = "";
  }

  /** @param {string} tool @param {string} args @param {number} seconds @param {any} [extra] @returns {Promise<string>} */
  function ask(tool, args, seconds, extra) {
    approvalCount += 1;
    const approval_id = `a-${approvalCount}`;
    startTool(tool);
    emit("approval_open", {
      approval_id,
      turn_id: activeTurn,
      call_id: runningCall,
      tool,
      args,
      args_bytes: encoder.encode(args).length,
      args_truncated: false,
      expires_at: new Date(Date.now() + seconds * 1000).toISOString(),
      ...extra,
    });
    setTimeout(() => closeApproval(approval_id, "expired"), seconds * 1000);
    return new Promise((resolve) => open.set(approval_id, { resolve }));
  }

  /** @param {string} id @param {string} reason @param {string} [decision] */
  function closeApproval(id, reason, decision) {
    const entry = open.get(id);
    if (!entry) return;
    open.delete(id);
    closedApprovals.add(id);
    emit("approval_close", decision ? { approval_id: id, reason, decision } : { approval_id: id, reason });
    entry.resolve(decision || reason);
  }

  /** Durdur: açık onaylar ve sürmekte olan iş kapanır. */
  function stopAll() {
    runToken += 1;
    for (const id of Array.from(open.keys())) closeApproval(id, "stopped");
    if (runningCall) endTool("stopped", 0, "");
    endTurn("stopped");
    setState("idle");
  }

  function publishMetrics() {
    emit("metrics", { memory_bytes: 2147483648 + (turnCount % 5) * 67108864, tokens_per_second: 9.5 + (turnCount % 4), first_token_ms: 640 + (turnCount % 3) * 90 });
  }

  /** @param {(ms: number) => Promise<void>} wait @param {string} question @param {number} seconds @param {any} [extra] */
  async function deleteTurn(wait, question, seconds, extra) {
    newTurn("text");
    userSays(question);
    setState("thinking");
    await wait(500);
    setState("speaking");
    await say(wait, t("mock_delete_intro"));
    setState("thinking");
    const args = JSON.stringify({ path: "/example/made-up-folder/made-up-file.txt" });
    return ask("example_delete_file", args, seconds, extra);
  }

  /** @param {(ms: number) => Promise<void>} wait @param {string} answer */
  async function finishDelete(wait, answer) {
    await wait(10);
    if (answer === "approve") {
      endTool("ok", 212, "");
      setState("speaking");
      await say(wait, t("mock_delete_done"));
    } else if (answer === "deny") {
      endTool("denied", 0, "");
      setState("speaking");
      await say(wait, t("mock_delete_denied"));
    } else {
      endTool(answer === "stopped" ? "stopped" : "denied", 0, "");
      setState("speaking");
      await say(wait, t("mock_delete_closed"));
    }
    endTurn("done");
    publishMetrics();
    setState("idle");
  }

  /** @type {Record<string, (wait: (ms: number) => Promise<void>, text?: string) => Promise<void>>} */
  const scenes = {
    async setup(wait) {
      current.mic = false;
      emit("settings", mockSettings);
      setState("setup");
      await wait(100);
    },
    async conversation(wait) {
      current.mic = true;
      setState("listening");
      await wait(500);
      newTurn("voice");
      for (const key of ["mock_draft_1", "mock_draft_2"]) {
        emit("turn_text", { turn_id: activeTurn, role: "user", mode: "replace", text: t(key), final: false });
        await wait(650);
      }
      userSays(t("mock_draft_3"));
      current.mic = false;
      setState("thinking");
      await wait(500);
      startTool("example_read_note");
      await wait(700);
      endTool("ok", 124, t("mock_tool_output"));
      setState("speaking");
      await say(wait, t("mock_answer"));
      endTurn("done");
      publishMetrics();
      setState("idle");
    },
    async loading(wait) {
      for (let step = 0; step <= 10; step += 1) {
        setState("loading", { progress: step / 10 });
        await wait(280);
      }
      setState("loading");
      await wait(1200);
      setState("idle");
    },
    async approval(wait) {
      await finishDelete(wait, await deleteTurn(wait, t("mock_question"), 30));
    },
    async approval_truncated(wait) {
      const args = JSON.stringify({ path: "/example/made-up-folder/", names: Array.from({ length: 300 }, (_, i) => `made-up-${i}.txt`) }).slice(0, 4096);
      await finishDelete(wait, await deleteTurn(wait, t("mock_question"), 30, { args, args_bytes: 1250000, args_truncated: true }));
    },
    async approval_hidden(wait) {
      const args = `{"path":"/example/made-up-\u202etxt.exe"}`;
      await finishDelete(wait, await deleteTurn(wait, t("mock_question"), 30, { args }));
    },
    async approval_expired(wait) {
      await finishDelete(wait, await deleteTurn(wait, t("mock_question"), 6));
    },
    async approval_stopped(wait) {
      deleteTurn(wait, t("mock_question"), 30).catch(() => {});
      await wait(4500);
      stopAll();
    },
    async approval_other_tab(wait) {
      const answer = deleteTurn(wait, t("mock_question"), 30);
      await wait(4500);
      for (const id of Array.from(open.keys())) closeApproval(id, "answered", "deny");
      await finishDelete(wait, await answer);
    },
    async stop(wait) {
      newTurn("text");
      userSays(t("mock_question"));
      setState("speaking");
      for (let i = 0; i < 400; i += 1) await say(wait, `${t("mock_stop_sentence")} `, 3, 70);
      endTurn("done");
      setState("idle");
    },
    async errors(wait) {
      newTurn("text");
      userSays(t("mock_question"));
      setState("thinking");
      await wait(400);
      startTool("example_delete_file");
      await wait(500);
      endTool("error", 37, "", { error: { code: "tool_failed", values: { tool: "example_delete_file" } } });
      await wait(300);
      endTurn("error", { code: "tool_timeout", values: { tool: "example_delete_file", limit_ms: 30000 } });
      emit("error", { code: "mic_unavailable", values: {}, fatal: false });
      emit("error", { code: "code_from_the_future", values: {}, fatal: false });
      await wait(500);
      emit("error", { code: "out_of_memory", values: { needed_bytes: 6442450944 }, fatal: true });
      setState("error");
      await wait(3500);
      setState("idle");
    },
    async disconnect(wait) {
      newTurn("text");
      userSays(t("mock_question"));
      setState("speaking");
      await say(wait, t("mock_drop_intro"));
      const answer = ask("example_delete_file", JSON.stringify({ path: "/example/made-up-file.txt" }), 40);
      await wait(1200);
      down = true;
      drop();
      await wait(1000);
      emit("turn_text", { turn_id: activeTurn, role: "assistant", mode: "append", text: t("mock_drop_rest") });
      await wait(2500);
      down = false;
      await finishDelete(wait, await answer);
    },
    async silent(wait) {
      muted = true;
      await wait(1);
    },
    async restart(wait) {
      deleteTurn(wait, t("mock_question"), 40).catch(() => {});
      await wait(3500);
      down = true;
      drop();
      await wait(1000);
      runToken += 1;
      open.clear();
      sessionNumber += 1;
      prefix = newId();
      seq = 0;
      frames = [];
      log = [];
      activeTurn = "";
      runningCall = "";
      lastMetrics = null;
      current = { state: "idle", mic: false };
      down = false;
    },
    async long_answer(wait) {
      newTurn("text");
      userSays(t("mock_question"));
      setState("speaking");
      const parts = [];
      for (let n = 1; n <= 2200; n += 1) parts.push(t("mock_long_sentence", { n }));
      await say(wait, parts.join(""), 300, 16);
      endTurn("done");
      publishMetrics();
      setState("idle");
    },
    async long_output(wait) {
      newTurn("text");
      userSays(t("mock_question"));
      setState("thinking");
      startTool("example_read_note");
      await wait(600);
      const lines = [];
      for (let n = 1; n <= 3000; n += 1) lines.push(t("mock_long_output", { n }));
      const output = lines.join("\n").slice(0, 40000); // her dilde 64 KiB altında
      endTool("ok", 2840, output, { output_bytes: 2500000, truncated: true });
      setState("speaking");
      await say(wait, t("mock_tool_output"));
      endTurn("done");
      setState("idle");
    },
    async hostile(wait) {
      newTurn("text");
      userSays("<b>made-up</b>");
      setState("speaking");
      const tag = "<" + "img src=x onerror=\"document.title='broken'\">";
      const script = "<" + "script>document.title='broken'<" + "/script>";
      const mixed = "\u202e" + "made-up" + "\u202c שלום مرحبا";
      await say(wait, `${tag}\n${script}\n${mixed}\n${t("mock_hostile_fake_card")}\n${"x".repeat(600)}\n`, 8, 30);
      startTool("<b>example_read_note</b>");
      await wait(300);
      endTool("ok", 1, `${tag}\n${script}\n`);
      setState("speaking");
      await say(wait, t("mock_hostile_note"));
      endTurn("done");
      setState("idle");
    },
    async bad_events(wait) {
      deliver("id: \ndata: {this is not json\n\n");
      await wait(300);
      emit("future_event", { anything: true });
      await wait(300);
      deliver(": a comment line\r\n\r\n");
      await wait(300);
      deliver(`data: ${"x".repeat(MAX_EVENT_BYTES + 16)}`);
    },
    async version() {
      deliver(`data: ${JSON.stringify({ v: PROTOCOL_VERSION + 1, type: "state", data: {} })}\n\n`);
    },
    /** Ekrandan gönderilen metin bu sahneye düşer; seçicide görünmez. */
    async reply(wait, text) {
      newTurn("text");
      userSays(text || "");
      setState("thinking");
      await wait(400);
      setState("speaking");
      await say(wait, t("mock_reply", { count: (text || "").length }));
      endTurn("done");
      publishMetrics();
      setState("idle");
    },
  };

  /** @param {string} name @param {string} [text] */
  function play(name, text) {
    if (activeTurn || open.size > 0) stopAll();
    runToken += 1;
    const token = runToken;
    down = false;
    /** @param {number} ms @returns {Promise<void>} */
    const wait = (ms) => new Promise((resolve, reject) => {
      setTimeout(() => (token === runToken ? resolve() : reject(CANCELLED)), ms);
    });
    scenes[name](wait, text).catch((reason) => {
      if (reason !== CANCELLED) throw reason;
    });
  }

  /** @param {any} command */
  function answer(command) {
    const accepted = { v: PROTOCOL_VERSION, command_id: command.command_id, status: "accepted" };
    /** @param {string} code @param {any} [values] */
    const rejected = (code, values) => ({ v: PROTOCOL_VERSION, command_id: command.command_id, status: "rejected", error: { code, values: values || {} } });
    const data = command.data || {};
    switch (command.type) {
      case "send_text":
        if (activeTurn) return rejected("turn_busy", { turn_id: activeTurn });
        if (encoder.encode(str(data.text)).length > 32768) return rejected("text_too_long", { limit_bytes: 32768 });
        play("reply", str(data.text));
        return accepted;
      case "mic_set":
        current.mic = data.on === true;
        setState(current.mic ? "listening" : "idle");
        return accepted;
      case "approval_answer":
        if (closedApprovals.has(data.approval_id)) return rejected("already_answered", { approval_id: data.approval_id });
        if (!open.has(data.approval_id)) return rejected("unknown_approval", { approval_id: data.approval_id });
        closeApproval(data.approval_id, "answered", data.decision);
        return accepted;
      case "stop":
        stopAll();
        return accepted;
      case "model_load":
        if (activeTurn) return rejected("turn_busy", { turn_id: activeTurn });
        if (str(data.entry_id) !== mockSettings.language_model) return rejected("unknown_entry", { entry_id: str(data.entry_id) });
        setState("loading", { model: str(data.entry_id) });
        setTimeout(() => setState("idle"), 400);
        return accepted;
      case "settings_set":
        if (!["speech_language", "system_prompt"].includes(str(data.key))) return rejected("setting_invalid", { key: str(data.key), reason: "unknown_key" });
        if (data.key === "speech_language" && typeof data.value !== "string") return rejected("setting_invalid", { key: "speech_language", reason: "invalid_value" });
        mockSettings[str(data.key)] = data.value;
        emit("settings", mockSettings);
        return accepted;
      default:
        return rejected("unknown_command", { type: command.type });
    }
  }

  return {
    scenes: Object.keys(scenes).filter((name) => name !== "reply"),
    play,
    /** @param {string|null} lastId @param {any} s */
    open(lastId, s) {
      const timer = setTimeout(() => attach(lastId, s), 120);
      return {
        cancel() {
          clearTimeout(timer);
          closeStream(s);
        },
      };
    },
    /** @param {string} body */
    async post(body) {
      await new Promise((resolve) => setTimeout(resolve, 60));
      if (down) throw new Error("connection lost");
      const command = JSON.parse(body);
      if (!answers.has(command.command_id)) answers.set(command.command_id, answer(command));
      return { status: 200, json: answers.get(command.command_id) };
    },
  };
}
