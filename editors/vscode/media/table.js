// Tablo görünümü web görünümü betiği. Tablo verisi ('veri') şablondaki satır içi betikten gelir.
// extension.js içindeki tableHtml tarafından yüklenir.
const columns = ['#'].concat(veri.sutunlar || []);
const rows = (veri.satirlar || []).map((s, i) => [String(i)].concat(s));
let sortState = { column: -1, descending: false };
document.getElementById('baslik').textContent = veri.baslik + ' — ' + (veri.tip || '') + ' (' + rows.length + ' satır)';
function render() {
  const list = tableRows(rows, document.getElementById('ara').value, sortState);
  const headerRow = document.getElementById('basliklar');
  headerRow.replaceChildren();
  columns.forEach((name, i) => {
    const th = document.createElement('th');
    th.textContent = name;
    if (sortState.column === i) { const o = document.createElement('span'); o.className = 'ok'; o.textContent = sortState.descending ? '▼' : '▲'; th.appendChild(o); }
    th.addEventListener('click', () => { sortState = { column: i, descending: sortState.column === i ? !sortState.descending : false }; render(); });
    headerRow.appendChild(th);
  });
  const body = document.getElementById('govde');
  body.replaceChildren();
  list.forEach(s => {
    const tr = document.createElement('tr');
    s.forEach(h => { const td = document.createElement('td'); td.textContent = h; if (isNumber(h)) td.className = 'sayi'; tr.appendChild(td); });
    body.appendChild(tr);
  });
  document.getElementById('sayac').textContent = list.length + ' / ' + rows.length + ' satır';
}
document.getElementById('ara').addEventListener('input', render);
render();
