// İzleme Masası web görünümü betiği (extension.js içindeki watchPanelHtml tarafından yüklenir).
const vscode = acquireVsCodeApi();
// Paneldeki beklenmeyen hatalar sessizce kaybolmasın: eklentiye bildirilir.
window.addEventListener('error', (e) => vscode.postMessage({ tur: 'panelHatasi', mesaj: String(e.message) + ' (' + e.lineno + ':' + e.colno + ')' }));
const ICONS = { tablo: '▦', liste: '☰', sozluk: '{}', yapi: '◆', tensor: '▣', sayi: '#', metin: 'Aa', mantik: '◐', yok: '∅', diger: '•' };
const KIND_NAMES = { tablo: 'Tablo', liste: 'Liste', sozluk: 'Sözlük', yapi: 'Yapı', tensor: 'Tensör', sayi: 'Sayı', metin: 'Metin', mantik: 'Mantık', yok: 'Yok', diger: 'Değer' };
const isNumber = (x) => x !== '' && x !== null && !isNaN(Number(x));
const el = (tag, text, cssClass) => { const e = document.createElement(tag); if (text !== undefined) e.textContent = text; if (cssClass) e.className = cssClass; return e; };

// Durum: değişkenler, açık sekmeler (her biri ayrı tablo), etkin sekme ve sekme başına arama/sıralama.
let state = null, previousValues = {}, sessionEnded = false, paused = false;
// stopCount bu oturumdaki duruşları sayar: Devam aynı kesme noktasında yeniden durduğunda
// (döngü) konum metni aynı kalır; sayaç ve kısa vurgu programın ilerlediğini gösterir.
let stopCount = 0;
function updateButtons() {
  document.querySelectorAll('#yonlendirme button').forEach((b) => {
    // Durdur her zaman (oturum sürdükçe) kullanılabilir; adım düğmeleri yalnızca program duraklatılmışken.
    b.disabled = sessionEnded || (b.dataset.komut !== 'stop' && !paused);
  });
}
function buttonPressed(b) {
  if (b.disabled) return;
  if (b.dataset.komut !== 'stop') {
    paused = false;
    document.getElementById('neden').textContent = 'Çalışıyor…';
  }
  updateButtons();
  vscode.postMessage({ tur: 'komut', komut: b.dataset.komut });
}
document.querySelectorAll('#yonlendirme button').forEach((b) => {
  // Fare düğmesi basıldığı anda işlenir (click, basma ile bırakma aynı öğede olmazsa hiç
  // gelmez: panel o anda odak alırken ya da düzen kayarken tıklama kaybolurdu). Klavyeyle
  // (Enter/Boşluk) gelen click'in detail'i 0'dır; fareyle gelen click yok sayılır.
  b.addEventListener('pointerdown', (e) => { if (e.button === 0) buttonPressed(b); });
  b.addEventListener('click', (e) => { if (e.detail === 0) buttonPressed(b); });
});
const tabs = [];          // { ad, tur, veri, hata, arama, sirala }
let activeTab = null;

window.addEventListener('message', (event) => {
  const m = event.data;
  if (m.tur === 'durum') {
    if (sessionEnded) stopCount = 0;
    stopCount++;
    state = m.veri; sessionEnded = false; paused = true;
    const header = document.querySelector('.baslik');
    header.classList.remove('yeni'); void header.offsetWidth; header.classList.add('yeni');
    renderVariables(); updateButtons();
    // Açık sekmelerin içeriğini yeni adımın değerleriyle yenile
    tabs.forEach((s) => vscode.postMessage({ tur: 'tablo', ifade: s.ad }));
  } else if (m.tur === 'tablo') {
    const s = tabs.find((x) => x.ad === m.veri.baslik);
    if (s) { s.veri = m.veri; s.error = ''; if (s.ad === activeTab) renderPane(); }
  } else if (m.tur === 'tabloHata') {
    const s = tabs.find((x) => x.ad === m.ifade);
    if (s) { s.error = m.mesaj; if (s.ad === activeTab) renderPane(); }
  } else if (m.tur === 'hata') {
    document.getElementById('neden').textContent = m.mesaj;
  } else if (m.tur === 'bitti') {
    sessionEnded = true; paused = false; updateButtons();
    document.getElementById('konum').textContent = 'Program bitti — tablolar son duruştaki değerleri gösteriyor';
    document.getElementById('neden').textContent = '';
  }
});

// openTable: tabloyu alt penceredeki sekmesinde açar (yoksa yeni sekme ekler) ve etkin yapar.
function openTable(d) {
  let s = tabs.find((x) => x.ad === d.ad);
  if (!s) {
    s = { ad: d.ad, tur: d.tur || 'tablo', veri: null, error: '', search: '', sortState: { column: -1, descending: false } };
    tabs.push(s);
    if (sessionEnded) s.error = 'Hata ayıklama oturumu bitti; tabloyu görmek için programı yeniden başlatın (F5).';
    else vscode.postMessage({ tur: 'tablo', ifade: d.ad });
  }
  activeTab = s.ad;
  renderPane(); renderVariables();
}

function closeTab(name) {
  const i = tabs.findIndex((x) => x.ad === name);
  if (i < 0) return;
  tabs.splice(i, 1);
  if (activeTab === name) activeTab = tabs.length ? tabs[Math.min(i, tabs.length - 1)].ad : null;
  renderPane(); renderVariables();
}

function renderVariables() {
  if (!state) return;
  if (!sessionEnded) {
    const location = state.dosya + ' · Satır ' + state.satir + ' · ' + state.cerceve;
    const reason = stopCount + '. duruş' + (state.neden ? ' · ' + state.neden : '');
    document.getElementById('konum').textContent = location;
    document.getElementById('konum').title = location;
    document.getElementById('neden').textContent = reason;
    document.getElementById('neden').title = reason;
  }
  const q = document.getElementById('degiskenAra').value.toLocaleLowerCase('tr');
  const body = document.getElementById('degiskenler');
  body.replaceChildren();
  const newValues = {};
  state.degiskenler.forEach((d) => { newValues[d.kapsam + ':' + d.ad] = d.deger; });
  const visible = state.degiskenler.filter((d) => !q || d.ad.toLocaleLowerCase('tr').includes(q));
  document.getElementById('degiskenSayac').textContent = visible.length + ' / ' + state.degiskenler.length + ' değişken';
  if (!visible.length) {
    const tr = el('tr'); const td = el('td', 'Değişken yok', 'bos'); td.colSpan = 5; tr.appendChild(td); body.appendChild(tr);
  }
  visible.forEach((d) => {
    const key = d.kapsam + ':' + d.ad;
    const kind = d.tur || 'diger';
    const isOpen = tabs.some((s) => s.ad === d.ad);
    const tr = el('tr');
    if (isOpen) tr.classList.add('acik');
    if (key in previousValues && previousValues[key] !== d.deger) tr.classList.add('degisti');
    const nameCell = el('td', undefined, 'ad'); nameCell.title = d.ad + ' (' + d.tip + ')';
    const icon = el('span', ICONS[kind] || '•', 'ikon ' + kind); icon.title = KIND_NAMES[kind] || '';
    nameCell.appendChild(icon);
    let valueCell;
    if (d.tablo) {
      // <a href> tıklamalarını VS Code'un web görünümü yakalar; link görünümlü span kullanılır.
      const nameLink = el('span', d.ad, 'link'); nameLink.setAttribute('role', 'button'); nameLink.tabIndex = 0;
      nameLink.addEventListener('click', (e) => { e.stopPropagation(); openTable(d); });
      nameLink.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openTable(d); } });
      nameCell.appendChild(nameLink);
      valueCell = el('td', undefined, 'deger');
      // Küçük değerler (ör. 1-8 elemanlı tensör) satır içinde de görünür; tablo bağlantısı yanında kalır.
      if (d.ozet) { valueCell.appendChild(el('span', d.ozet + '  ', 'ozet')); valueCell.title = d.ozet; }
      const link = el('span', isOpen ? '● Açık — sekmeye git' : (d.ozet ? '▸ Tablo' : '▸ Tabloyu aç'), 'link'); link.setAttribute('role', 'button'); link.tabIndex = 0;
      link.title = d.tip;
      link.addEventListener('click', (e) => { e.stopPropagation(); openTable(d); });
      link.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openTable(d); } });
      valueCell.appendChild(link);
      tr.classList.add('tablo-satiri');
      tr.title = 'Çift tıklayın: tabloyu alttaki pencerede aç';
      tr.addEventListener('dblclick', (e) => { if (e.target.tagName !== 'INPUT') openTable(d); });
    } else {
      nameCell.appendChild(document.createTextNode(d.ad));
      valueCell = el('td', d.deger, 'deger'); valueCell.title = d.deger;
    }
    const pencil = el('button', '✏️', 'kalem'); pencil.title = 'Değeri değiştir (TR ifadesi)';
    pencil.addEventListener('click', () => editValue(valueCell, d));
    const actionCell = el('td'); actionCell.appendChild(pencil);
    tr.append(el('td', d.kapsam, 'kapsam'), nameCell, el('td', d.tip, 'tip'), valueCell, actionCell);
    body.appendChild(tr);
  });
  previousValues = newValues;
}

function renderPane() {
  const pane = document.getElementById('pencere');
  const divider = document.getElementById('ayirici');
  const show = tabs.length > 0;
  pane.style.display = show ? 'flex' : 'none';
  divider.style.display = show ? 'block' : 'none';
  if (!show) return;

  const strip = document.getElementById('sekmeler');
  strip.replaceChildren();
  tabs.forEach((s) => {
    const tabEl = el('div', undefined, 'sekme' + (s.ad === activeTab ? ' etkin' : ''));
    tabEl.title = s.ad;
    tabEl.append(el('span', ICONS[s.tur] || '▦', 'ikon ' + s.tur), el('span', s.ad));
    const closeBtn = el('button', '✕', 'kapat'); closeBtn.title = 'Sekmeyi kapat';
    closeBtn.addEventListener('click', (e) => { e.stopPropagation(); closeTab(s.ad); });
    tabEl.appendChild(closeBtn);
    tabEl.addEventListener('click', () => { activeTab = s.ad; renderPane(); });
    tabEl.addEventListener('auxclick', (e) => { if (e.button === 1) closeTab(s.ad); });
    strip.appendChild(tabEl);
  });
  strip.appendChild(el('span', undefined, 'bosluk'));
  const closeAll = el('button', 'Tümünü kapat ✕'); closeAll.id = 'hepsiniKapat';
  closeAll.addEventListener('click', () => { tabs.splice(0); activeTab = null; renderPane(); renderVariables(); });
  strip.appendChild(closeAll);

  const s = tabs.find((x) => x.ad === activeTab);
  const search = document.getElementById('tabloAra');
  search.value = s ? s.search : '';
  const box = document.getElementById('izgaraKutu');
  box.replaceChildren();
  const counter = document.getElementById('tabloSayac');
  if (!s) { counter.textContent = ''; return; }
  if (s.error) { box.appendChild(el('div', 'Tablo gösterilemedi: ' + s.error, 'bos')); counter.textContent = ''; return; }
  if (!s.veri) { box.appendChild(el('div', 'Yükleniyor…', 'bos')); counter.textContent = ''; return; }

  const columns = ['#'].concat(s.veri.sutunlar || []);
  const rows = (s.veri.satirlar || []).map((r, i) => [String(i)].concat(r));
  const q = s.search.toLocaleLowerCase('tr');
  let list = rows.filter((r) => !q || r.some((h) => String(h).toLocaleLowerCase('tr').includes(q)));
  if (s.sortState.column >= 0) {
    const k = s.sortState.column;
    list = list.slice().sort((a, b) => {
      const c = isNumber(a[k]) && isNumber(b[k]) ? Number(a[k]) - Number(b[k]) : String(a[k]).localeCompare(String(b[k]), 'tr');
      return s.sortState.descending ? -c : c;
    });
  }
  const t = el('table'); const thead = el('thead'); const headerRow = el('tr'); const body = el('tbody');
  columns.forEach((name, i) => {
    const th = el('th', name + (s.sortState.column === i ? (s.sortState.descending ? ' ▼' : ' ▲') : ''));
    th.title = 'Sıralamak için tıklayın';
    th.addEventListener('click', () => { s.sortState = { column: i, descending: s.sortState.column === i ? !s.sortState.descending : false }; renderPane(); });
    headerRow.appendChild(th);
  });
  list.forEach((r) => {
    const row = el('tr');
    r.forEach((h) => { const td = el('td', h, isNumber(h) ? 'sayi' : ''); td.tabIndex = -1; row.appendChild(td); });
    body.appendChild(row);
  });
  thead.appendChild(headerRow); t.append(thead, body); box.appendChild(t);
  counter.textContent = s.ad + ' — ' + list.length + ' / ' + rows.length + ' satır · ' + (s.veri.sutunlar || []).length + ' sütun';
}

// Izgarada ok tuşlarıyla hücreden hücreye gezinme
document.getElementById('izgaraKutu').addEventListener('keydown', (e) => {
  const td = e.target.closest('td');
  if (!td) return;
  const tr = td.parentElement;
  let target = null;
  if (e.key === 'ArrowRight') target = td.nextElementSibling;
  else if (e.key === 'ArrowLeft') target = td.previousElementSibling;
  else if (e.key === 'ArrowDown' && tr.nextElementSibling) target = tr.nextElementSibling.children[td.cellIndex];
  else if (e.key === 'ArrowUp' && tr.previousElementSibling) target = tr.previousElementSibling.children[td.cellIndex];
  if (target) { e.preventDefault(); target.focus(); }
});
document.getElementById('izgaraKutu').addEventListener('click', (e) => { const td = e.target.closest('td'); if (td) td.focus(); });

document.getElementById('tabloAra').addEventListener('input', (e) => {
  const s = tabs.find((x) => x.ad === activeTab);
  if (s) { s.search = e.target.value; renderPane(); document.getElementById('tabloAra').focus(); }
});
document.getElementById('degiskenAra').addEventListener('input', renderVariables);
updateButtons();

// Ayırıcı çubukla alt pencereyi boyutlandırma
(() => {
  const divider = document.getElementById('ayirici');
  const pane = document.getElementById('pencere');
  let drag = null;
  divider.addEventListener('mousedown', (e) => { drag = { y: e.clientY, h: pane.getBoundingClientRect().height }; e.preventDefault(); });
  window.addEventListener('mousemove', (e) => {
    if (!drag) return;
    const newHeight = Math.max(90, Math.min(window.innerHeight - 120, drag.h - (e.clientY - drag.y)));
    pane.style.height = newHeight + 'px';
  });
  window.addEventListener('mouseup', () => { drag = null; });
})();

function editValue(cell, d) {
  const input = el('input'); input.value = d.tablo ? '' : d.deger; input.placeholder = 'Yeni değer, ör. 0.20 veya [1, 2]'; input.style.width = '95%';
  cell.replaceChildren(input); input.focus(); input.select();
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && input.value.trim() !== '') vscode.postMessage({ tur: 'degistir', ad: d.ad, ifade: input.value });
    if (e.key === 'Escape') renderVariables();
  });
}
