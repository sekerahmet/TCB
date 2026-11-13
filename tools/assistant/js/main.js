// @ts-check
"use strict";
// Betiklerin kapanması (`sealScripts`), eşleme ve başlangıç: son modül. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

/**
 * Dil dosyaları yüklendi: bundan sonra hiçbir betik yüklenemez. Sayfa yerel dosyadan açılınca ilk politika dosyaları
 * birbirinden ayıramaz; politikalar birlikte geçerli olduğu için bu ikincisi kapıyı kapar, güvenilmez metin de ancak
 * bundan sonra gelir.
 */
function sealScripts() {
  const policy = /** @type {HTMLMetaElement} */ (el("meta"));
  policy.httpEquiv = "Content-Security-Policy";
  policy.content = "script-src 'none'";
  document.head.append(policy);
}

// ---------- Eşleme (contract.md, "Kaynak") ----------

const pairOptions = {
  cache: /** @type {RequestCache} */ ("no-store"),
  credentials: /** @type {RequestCredentials} */ ("omit"),
  referrerPolicy: /** @type {ReferrerPolicy} */ ("no-referrer"),
};

/** Önden işlenen sayfa kodu tüketmez: sayfa gösterilene kadar bekler (DENETİM 494 N3). */
function whenShown() {
  const doc = /** @type {Document & {prerendering?: boolean}} */ (document);
  if (!doc.prerendering) return Promise.resolve();
  return new Promise((resolve) => document.addEventListener("prerenderingchange", () => resolve(undefined), { once: true }));
}

/** Kodla belirteci alır (`GET /pair`); olmazsa nedeni gösterir. Kullanılmış kodda yeniden açma yoktur. @param {string} code */
async function pair(code) {
  await whenShown();
  let response;
  try {
    response = await fetch(`${base}/pair`, { ...pairOptions, headers: { "Dost-Pair-Code": code } });
  } catch {
    setConn("pair_invalid");
    return;
  }
  const json = await response.json().catch(() => null);
  if (response.ok && json && typeof json.token === "string" && json.token !== "") {
    saveToken(json.token);
    client.connect();
    return;
  }
  setConn(json && json.error && json.error.code === "pair_code_used" ? "pair_used" : "pair_invalid");
}

/** Sunucudan yeni kodlu bir sekme ister (`GET /pair/renew`); `202` dışında bir cevapta belirteç kutusu görünür. */
async function renew() {
  let status = 0;
  try {
    status = (await fetch(`${base}/pair/renew`, { ...pairOptions, headers: { "Dost-Pair-Renew": "1" } })).status;
  } catch {
    status = 0;
  }
  if (status !== 202) setConn("auth");
}

function start() {
  sealScripts();
  languages = Object.keys(dictionary);
  language = pickLanguage();
  document.title = ASSISTANT_NAME;
  byId("brand").textContent = ASSISTANT_NAME;
  const select = byId("languageSelect");
  select.replaceChildren(...languages.map((code) => {
    const option = /** @type {HTMLOptionElement} */ (el("option", "", dictionary[code].language_name));
    option.value = code;
    return option;
  }));
  for (const id of ["speechLanguage", "settingsLanguage"]) {
    byId(id).replaceChildren(...languages.map((code) => {
      const option = /** @type {HTMLOptionElement} */ (el("option", "", dictionary[code].language_name));
      option.value = code;
      return option;
    }));
  }
  wire();
  renderToggles();
  renderCalls();
  renderTools();
  applyLanguage();
  if (mock) {
    byId("scenePick").hidden = false;
    byId("scenePlay").addEventListener("click", () => mock.play(byId("sceneSelect").value));
  }
  if (!transport) {
    setConn("rejected", { source: location.origin });
    return;
  }
  renderStatus();
  if (!mock) ui.token = loadToken();
  if (!mock && PAIR_CODE !== "") pair(PAIR_CODE);
  else client.connect();
  if (mock) setTimeout(() => mock.play("conversation"), 500);
}

document.addEventListener("DOMContentLoaded", start);
