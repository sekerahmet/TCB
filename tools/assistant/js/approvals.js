// @ts-check
"use strict";
// Onay kartları: süre, odak, cevap. Ekranın modülü (index.html; sıra contract.md, "Sunulan dosyalar").

// ---------- Onay kartları ----------

/** @param {any} card */
function refreshCard(card) {
  const left = Math.max(0, Math.ceil((card.expiresAt - Date.now()) / 1000));
  const timed = Number.isFinite(card.expiresAt);
  card.time.hidden = !timed;
  if (timed) {
    if (left > 0) bind(card.time, "approval_time_left", { seconds: left });
    else bind(card.time, "approval_time_up");
  }
  const usable = connected() && !card.busy;
  card.deny.disabled = !usable;
  card.approve.disabled = !usable || !card.armed || (timed && left === 0);
  card.status.hidden = connected() && !card.busy && !card.error;
  if (!connected()) bind(card.status, "approval_offline");
  else if (card.busy) bind(card.status, "approval_sent");
  else if (card.error) bind(card.status, card.error.key, card.error.values);
}

function refreshCards() {
  for (const card of cards.values()) refreshCard(card);
}

/** @param {any} card @param {string} decision */
async function answerCard(card, decision) {
  if (card.busy || !connected()) return;
  // Düğme kapanınca tarayıcı odağı bırakır: kart kalkınca odağın nereye döneceği şimdiden bilinmeli.
  card.hadFocus = card.root.contains(document.activeElement);
  card.busy = true;
  card.error = null;
  card.answered = true;
  refreshCard(card);
  // Aynı karar aynı kimlikle yinelenir: kopan bağlantıda iş iki kez yapılmaz.
  card.commands[decision] = card.commands[decision] || newId();
  const reply = await sendCommand("approval_answer", { approval_id: card.id, decision }, card.commands[decision]);
  if (reply.ok || !cards.has(card.id)) return;
  card.busy = false;
  card.answered = false;
  card.error = errorKey(reply.error);
  refreshCard(card);
  if (card.hadFocus) card.deny.focus();
}

/** @param {any} d */
function openCard(d) {
  const id = str(d.approval_id);
  if (!id || cards.has(id)) return;
  const root = el("section", "card");
  root.setAttribute("role", "group");
  const title = bind(el("h2", "card-title"), "approval_title");
  title.id = `approval-title-${cards.size}-${Date.now()}`;
  root.setAttribute("aria-labelledby", title.id);
  const time = el("span", "card-time");
  const head = el("div", "card-head");
  head.append(title, time);
  const args = el("pre", "card-args", visible(str(d.args)));
  args.dir = "ltr";
  args.tabIndex = 0;
  const tool = visible(str(d.tool));
  root.append(head, bind(el("p", "card-field"), "approval_tool"), el("div", "card-tool", tool),
    bind(el("p", "card-field"), "approval_args"), args);
  if (d.args_truncated === true) {
    const shown = new TextEncoder().encode(str(d.args)).length;
    root.append(bind(el("p", "card-note"), "approval_truncated", { shown_bytes: shown, total_bytes: num(d.args_bytes) ?? shown }));
  }
  const status = el("p", "card-note");
  const deny = /** @type {HTMLButtonElement} */ (bind(el("button", "btn btn-deny"), "approval_deny"));
  const approve = /** @type {HTMLButtonElement} */ (bind(el("button", "btn btn-approve"), "approval_approve"));
  deny.type = "button";
  approve.type = "button";
  const actions = el("div", "card-actions");
  actions.append(deny, approve);
  root.append(status, actions);

  const card = {
    id, tool, root, time, status, deny, approve,
    expiresAt: Date.parse(str(d.expires_at)),
    armed: false, busy: false, answered: false, hadFocus: false, error: null,
    commands: /** @type {Record<string, string>} */ ({}),
  };
  cards.set(id, card);
  cardsBox.append(root);
  deny.addEventListener("click", () => answerCard(card, "deny"));
  approve.addEventListener("click", () => answerCard(card, "approve"));
  root.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      answerCard(card, "deny");
    }
  });
  setTimeout(() => {
    card.armed = true;
    refreshCard(card);
  }, APPROVE_ARM_MS);
  refreshCard(card);
  // Yazı yazarken odak çalınmaz: boşluk tuşu istemeden kartı kapatmasın.
  const active = document.activeElement;
  const typing = (active === composer || active === byId("search")) && /** @type {HTMLInputElement} */ (active).value !== "";
  if (!typing) deny.focus();
}

/** @param {any} card */
function removeCard(card) {
  const hadFocus = card.hadFocus || card.root.contains(document.activeElement);
  cards.delete(card.id);
  card.root.remove();
  if (!hadFocus) return;
  const next = cards.values().next().value;
  if (next) next.deny.focus();
  else composer.focus();
}
