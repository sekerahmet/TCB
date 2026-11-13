// @ts-check
"use strict";
// Gerçek kaynak (`fetch` ile akış ve komut), sayfanın kaynağı ve bağlantı. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

// ---------- Taşıyıcılar ----------

/**
 * Gerçek kaynak. `EventSource` üstbilgi taşıyamaz, belirteç URL'ye yazılmaz: akış `fetch` ile okunur.
 * @param {string} base
 */
function fetchTransport(base) {
  const headers = () => {
    /** @type {Record<string, string>} */
    const list = {};
    if (ui.token) list.Authorization = `Bearer ${ui.token}`;
    return list;
  };
  const options = { cache: /** @type {RequestCache} */ ("no-store"), credentials: /** @type {RequestCredentials} */ ("omit"), referrerPolicy: /** @type {ReferrerPolicy} */ ("no-referrer") };
  return {
    /** @param {string|null} lastId @param {any} sink */
    open(lastId, sink) {
      const abort = new AbortController();
      (async () => {
        try {
          const list = { ...headers(), Accept: "text/event-stream" };
          if (lastId !== null) list["Last-Event-ID"] = lastId;
          const response = await fetch(`${base}/events`, { ...options, headers: list, signal: abort.signal });
          if (!response.ok || !response.body) {
            sink.closed(response.status);
            return;
          }
          sink.opened();
          const reader = response.body.getReader();
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            sink.chunk(value);
          }
          sink.closed(0);
        } catch {
          if (!abort.signal.aborted) sink.closed(0);
        }
      })();
      return { cancel: () => abort.abort() };
    },
    /** @param {string} body */
    async post(body) {
      const response = await fetch(`${base}/command`, {
        ...options,
        method: "POST",
        headers: { ...headers(), "Content-Type": "application/json" },
        body,
      });
      let json = null;
      try {
        json = await response.json();
      } catch {
        json = null;
      }
      return { status: response.status, json };
    },
  };
}

// ---------- Bağlantı ----------

/**
 * Kaynak sayfanın kendi köküdür (contract.md, "Kaynak"): belirteç yalnız sayfayı sunan kapıya gider, başka bir
 * yerel süreç onu alamaz. Yalnız `http://127.0.0.1:<kapı>`; `localhost`, başka adres ve `https` reddedilir.
 * @returns {string|null}
 */
function pageSource() {
  const plain = location.protocol === "http:" && location.hostname === "127.0.0.1" && location.port !== "";
  return plain ? location.origin : null;
}

const fromFile = location.protocol === "file:";
const base = fromFile ? null : pageSource();
const mock = fromFile ? createMockTransport() : null;
/** @type {any} */
const transport = mock || (base ? fetchTransport(base) : null);

const client = (() => {
  let attempt = 0;
  let handle = /** @type {{cancel: () => void}|null} */ (null);
  let watchdog = 0;
  let retryTimer = 0;
  let generation = 0;

  function stop() {
    generation += 1;
    clearTimeout(watchdog);
    clearTimeout(retryTimer);
    if (handle) handle.cancel();
    handle = null;
  }

  function retry() {
    stop();
    attempt += 1;
    setConn("reconnecting", { attempt });
    retryTimer = window.setTimeout(connect, Math.min(RETRY_LIMIT_MS, 500 * 2 ** (attempt - 1)));
  }

  function feed() {
    clearTimeout(watchdog);
    watchdog = window.setTimeout(retry, ui.heartbeatMs * 2.5);
  }

  function connect() {
    stop();
    const mine = generation;
    const live = () => mine === generation;
    if (attempt === 0) setConn("connecting");
    // Bırakılan bağlantının tamponda kalan çerçeveleri işlenmez: yeniden bağlanma aynı itişin içinde olabilir.
    const parser = createParser((frame) => {
      if (live()) receive(frame);
    }, () => {
      noteError({ code: "event_too_large", values: { limit_bytes: MAX_EVENT_BYTES } });
      retry();
    });
    handle = transport.open(ui.lastId, {
      opened() {
        if (!live()) return;
        attempt = 0;
        setConn("open");
        feed();
      },
      /** @param {Uint8Array} bytes */
      chunk(bytes) {
        if (!live()) return;
        feed();
        parser.push(bytes);
      },
      /** @param {number} status */
      closed(status) {
        if (!live()) return;
        if (status === 401) unauthorized();
        else if (status === 403) refuse("forbidden");
        else retry();
      },
    });
  }

  /** Yeniden denemenin işe yaramayacağı durumlar. @param {string} phase @param {Record<string, unknown>} [values] */
  function refuse(phase, values) {
    stop();
    setConn(phase, values);
  }

  /** `401`: belirteç vardıysa geçersizdir (sunucu yeniden açıldı), silinir; yoksa kullanıcı girer (kutu). */
  function unauthorized() {
    const had = ui.token !== "";
    dropToken();
    refuse(had ? "lost" : "auth");
  }

  return { connect, refuse, retry, unauthorized };
})();

function newId() {
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

/** @param {string} type @param {any} data @param {string} [commandId] @returns {Promise<{ok: boolean, error?: any}>} */
async function sendCommand(type, data, commandId) {
  const command_id = commandId || newId();
  let reply;
  try {
    reply = await transport.post(JSON.stringify({ v: PROTOCOL_VERSION, command_id, type, data }));
  } catch {
    return { ok: false, error: { code: "connection_lost" } };
  }
  if (reply.status === 401) client.unauthorized();
  const json = reply.json;
  if (json && json.status === "accepted") return { ok: true };
  return { ok: false, error: (json && json.error) || { code: "bad_request" } };
}

/** @param {string} type @param {any} data */
async function command(type, data) {
  const reply = await sendCommand(type, data);
  if (!reply.ok) noteError(reply.error);
  return reply.ok;
}
