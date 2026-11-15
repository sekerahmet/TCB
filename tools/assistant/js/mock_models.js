// @ts-check
"use strict";
// Sahte kaynağın modelleri (sahte kip): katalogun her türü, ayar, indirme (ortak parça kendi işiyle), otomatik yükleme
// ve düşen yükleme; komutların bu kısmının cevabı. js/mock.js kurar. Ekranın modülü (index.html; sıra contract.md,
// "Sunulan dosyalar").

/**
 * @param {{emit: (type: string, data?: any) => void, setState: (state: string, extra?: any) => void,
 *   state: () => string, setMic: (on: boolean) => void, turn: () => string}} host
 */
function createMockModels(host) {
  const { emit, setState } = host;
  const MAX_CONTEXT = 40960;
  const settings = /** @type {Record<string, any>} */ ({ language_model: "mock-model", speech_language: "tr", system_prompt: { source: "catalog" }, context: 8192, device: "cpu", port: 47863, quantized_activations: "auto", machine: { memory_bytes: 17179869184, memory_source: "option" } });
  // Uydurma katalog: dört tür, iki konuşma dili ve bilinmeyen bir tür (ekran onu yok sayar). `installed` hesaplanır.
  /** @param {string} id @param {string} kind @param {number} bytes @param {number} memory @param {any} [extra] */
  const entry = (id, kind, bytes, memory, extra) => ({ id, kind, name: id, installed: false, downloaded: true, bytes, memory_bytes: memory, ...extra });
  const catalog = { entries: [
    entry("mock-model", "language_model", 2497280256, 13632032768, { fit: "tight" }),
    entry("mock-model-large", "language_model", 5027783488, 16900000000, { fit: "too_large", downloaded: false }),
    entry("mock-transcriber", "transcriber", 969481381, 990814208, { languages: ["tr", "en"] }),
    entry("mock-voice-tr", "voice", 145233756, 253636608, { languages: ["tr"] }),
    entry("mock-voice-en", "voice", 145229853, 252952576, { languages: ["en"], downloaded: false }),
    entry("mock-vad", "vad", 1239748, 1257472),
    { id: "mock-image", kind: "image_model", name: "mock-image", installed: true },
  ] };
  /** @param {string} id */
  const find = (id) => catalog.entries.find((e) => e.id === id);
  /** Ayarın konuşma dilinin parçaları: VAD ve dilin yazıcısı ile sesi. */
  const parts = () => catalog.entries.filter((e) => e.kind === "vad" || ((e.kind === "transcriber" || e.kind === "voice") && e.languages.includes(settings.speech_language)));
  function refresh() {
    const team = parts().every((e) => e.downloaded);
    for (const e of catalog.entries) if (e.kind !== "image_model") e.installed = e.downloaded && (e.kind !== "language_model" || team);
    emit("catalog", catalog);
  }
  let pendingMic = false;
  let loadTimer = 0;
  /** Yükleme: 0,3 sn'de bir %25; düşerse `model_load_failed` ve kurulum. @param {string} id @param {boolean} [fail] */
  function load(id, fail) {
    clearTimeout(loadTimer);
    let step = 0;
    const tick = () => {
      if (step < 4) {
        setState("loading", { model: id, progress: step / 4 });
        step += 1;
        loadTimer = window.setTimeout(tick, 300);
        return;
      }
      if (fail) {
        pendingMic = false;
        emit("error", { code: "model_load_failed", values: { model: id }, fatal: false });
        setState("setup");
        return;
      }
      settings.language_model = id;
      emit("settings", settings);
      host.setMic(pendingMic);
      setState(pendingMic ? "listening" : "idle");
      pendingMic = false;
    };
    tick();
  }
  /** Ayarın takımı kuruluysa kurulumda kendisi yükler (contract.md, "Yükleme"). */
  function autoLoad() {
    const model = find(settings.language_model);
    if (host.state() === "setup" && model && model.installed) load(model.id);
  }
  // Sahte indirme: her girdi bir iş, sırayla; sırada, 0,3 sn'de bir %25, denetleme, bitince girdi inmiş. Sürdürme
  // kalan yüzdeden. Dil modelinin İndir'i konuşma dilinin eksik parçalarını da sıraya koyar.
  /** @type {any[]} */
  const queue = [];
  /** @type {Record<string, any>} */
  const last = {};
  let jobCount = 0;
  const started = new Set(); // adımları süren işler: Vazgeç'ten sonra iş ikinci kez başlamasın
  /** @param {any} e */
  function enqueue(e) {
    const from = last[e.id] ? last[e.id].files[0].done_bytes : 0;
    const job = last[e.id] = { job_id: `d${++jobCount}`, entry_id: e.id, state: "queued", files: [{ name: `${e.id}.bin`, done_bytes: from, total_bytes: e.bytes }], bytes_per_second: 0, error: null };
    queue.push(job);
    emit("download", job);
  }
  function work() {
    const job = queue.find((j) => j.state === "queued" || j.state === "running" || j.state === "verifying");
    if (!job || started.has(job.job_id)) return;
    started.add(job.job_id);
    const step = () => {
      if (job.state === "cancelled") return void work();
      const f = job.files[0];
      job.state = f.done_bytes < f.total_bytes ? "running" : job.state === "verifying" ? "done" : "verifying";
      if (job.state === "running") f.done_bytes = Math.min(f.total_bytes, f.done_bytes + f.total_bytes / 4);
      job.bytes_per_second = job.state === "running" ? 5242880 : 0;
      emit("download", job);
      if (job.state !== "done") return void setTimeout(step, 300);
      /** @type {any} */ (find(job.entry_id)).downloaded = true;
      refresh();
      autoLoad();
      work();
    };
    setTimeout(step, 300);
  }
  const active = (/** @type {string} */ id) => queue.some((j) => j.entry_id === id && ["queued", "running", "verifying"].includes(j.state));
  /** @type {Record<string, (value: any) => boolean>} */
  const types = {
    speech_language: (value) => typeof value === "string",
    system_prompt: (value) => Boolean(value) && typeof value === "object" && (value.source === "catalog" || (value.source === "user" && typeof value.text === "string")),
    context: (value) => Number.isInteger(value) && value > 0 && value <= MAX_CONTEXT,
    microphone: (value) => typeof value === "string" && value !== "",
    recording: (value) => typeof value === "boolean",
    quantized_activations: (value) => ["auto", "on", "off"].includes(value),
    language_model: (value) => Boolean(find(str(value))),
  };

  return {
    settings,
    publish() {
      refresh();
      emit("settings", settings);
    },
    /** Kurulum: hiçbir model inmemiş (`empty`) ya da bütün Türkçe takım inmiş. @param {boolean} empty */
    reset(empty) {
      for (const e of catalog.entries) if (e.kind !== "image_model") e.downloaded = !empty && !["mock-model-large", "mock-voice-en"].includes(e.id);
      settings.language_model = "mock-model";
      settings.speech_language = "tr";
      this.publish();
    },
    /** Düşen yükleme: büyük model inmiş ama yüklenemiyor. */
    failLoad() {
      this.reset(false);
      /** @type {any} */ (find("mock-model-large")).downloaded = true;
      refresh();
      load("mock-model-large", true);
    },
    /** @param {any} command @param {any} accepted @param {(code: string, values?: any) => any} rejected */
    answer(command, accepted, rejected) {
      const data = command.data || {};
      const id = str(data.entry_id);
      switch (command.type) {
        case "mic_set": {
          if (data.on !== true) {
            pendingMic = false;
            host.setMic(false);
            if (host.state() === "listening") setState("idle");
            return accepted;
          }
          if (host.state() === "loading") {
            pendingMic = true;
            return accepted;
          }
          if (host.state() === "setup") {
            const model = find(settings.language_model);
            if (!model || !model.installed) return rejected("not_installed", { entry_id: model && model.downloaded ? (parts().find((e) => !e.downloaded) || model).id : settings.language_model });
            pendingMic = true;
            load(model.id);
            return accepted;
          }
          host.setMic(true);
          setState("listening");
          return accepted;
        }
        case "model_load": {
          const model = find(id);
          if (host.turn()) return rejected("turn_busy", { turn_id: host.turn() });
          if (!model || model.kind !== "language_model") return rejected("unknown_entry", { entry_id: id });
          if (!model.installed) return rejected("not_installed", { entry_id: id });
          load(id, id === "mock-model-large");
          return accepted;
        }
        case "settings_set": {
          const key = str(data.key);
          const check = types[key];
          if (!check) return rejected("setting_invalid", { key, reason: "unknown_key" });
          if (!check(data.value)) return rejected("setting_invalid", { key, reason: "invalid_value" });
          if (key === "language_model" && !(/** @type {any} */ (find(data.value))).installed) return rejected("setting_invalid", { key, reason: "not_installed" });
          if ((key === "language_model" || key === "speech_language") && host.turn()) return rejected("turn_busy", { turn_id: host.turn() });
          settings[key] = data.value;
          if (key === "speech_language") refresh();
          emit("settings", settings);
          const model = find(settings.language_model);
          if (key === "speech_language" && host.state() !== "setup" && !(model && model.installed)) setState("setup");
          if (key === "language_model" && host.state() !== "setup") load(data.value, data.value === "mock-model-large");
          if (key === "language_model" || key === "speech_language") autoLoad();
          return accepted;
        }
        case "model_download": {
          const e = find(id);
          if (!e || e.kind === "image_model") return rejected("unknown_entry", { entry_id: id });
          if (active(id)) return rejected("download_busy", { entry_id: id });
          if (!e.downloaded) enqueue(e);
          if (e.kind === "language_model") for (const p of parts()) if (!p.downloaded && !active(p.id)) enqueue(p);
          work();
          return accepted;
        }
        case "model_delete": {
          const e = find(id);
          if (!e || e.kind === "image_model") return rejected("unknown_entry", { entry_id: id });
          if (active(id)) return rejected("download_busy", { entry_id: id });
          const inTeam = e.id === settings.language_model || parts().includes(e);
          if (host.turn() && inTeam) return rejected("turn_busy", { turn_id: host.turn() });
          e.downloaded = false;
          delete last[e.id];
          refresh();
          if (inTeam) setState("setup");
          return accepted;
        }
        case "download_cancel": {
          const job = queue.find((j) => j.job_id === str(data.job_id));
          if (!job) return rejected("unknown_job", { job_id: str(data.job_id) });
          if (job.state === "queued" || job.state === "running") emit("download", Object.assign(job, { state: "cancelled", bytes_per_second: 0 }));
          work();
          return accepted;
        }
        default:
          return null;
      }
    },
  };
}
