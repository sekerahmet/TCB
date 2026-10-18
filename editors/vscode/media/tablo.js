// Tablo görünümü web görünümü betiği. Tablo verisi ('veri') şablondaki satır içi betikten gelir.
// extension.js içindeki tabloHtml tarafından yüklenir.
const sutunlar = ['#'].concat(veri.sutunlar || []);
const satirlar = (veri.satirlar || []).map((s, i) => [String(i)].concat(s));
let sirala = { sutun: -1, azalan: false };
const sayiMi = (x) => x !== '' && !isNaN(Number(x));
document.getElementById('baslik').textContent = veri.baslik + ' — ' + (veri.tip || '') + ' (' + satirlar.length + ' satır)';
function ciz() {
  const q = document.getElementById('ara').value.toLocaleLowerCase('tr');
  let liste = satirlar.filter(s => !q || s.some(h => String(h).toLocaleLowerCase('tr').includes(q)));
  if (sirala.sutun >= 0) {
    const k = sirala.sutun;
    liste = liste.slice().sort((a, b) => {
      const x = a[k], y = b[k];
      const c = sayiMi(x) && sayiMi(y) ? Number(x) - Number(y) : String(x).localeCompare(String(y), 'tr');
      return sirala.azalan ? -c : c;
    });
  }
  const bas = document.getElementById('basliklar');
  bas.replaceChildren();
  sutunlar.forEach((ad, i) => {
    const th = document.createElement('th');
    th.textContent = ad;
    if (sirala.sutun === i) { const o = document.createElement('span'); o.className = 'ok'; o.textContent = sirala.azalan ? '▼' : '▲'; th.appendChild(o); }
    th.addEventListener('click', () => { sirala = { sutun: i, azalan: sirala.sutun === i ? !sirala.azalan : false }; ciz(); });
    bas.appendChild(th);
  });
  const govde = document.getElementById('govde');
  govde.replaceChildren();
  liste.forEach(s => {
    const tr = document.createElement('tr');
    s.forEach(h => { const td = document.createElement('td'); td.textContent = h; if (sayiMi(h)) td.className = 'sayi'; tr.appendChild(td); });
    govde.appendChild(tr);
  });
  document.getElementById('sayac').textContent = liste.length + ' / ' + satirlar.length + ' satır';
}
document.getElementById('ara').addEventListener('input', ciz);
ciz();
