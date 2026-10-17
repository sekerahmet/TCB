// İzleme Masası web görünümü betiği (extension.js içindeki izlemeMasasiHtml tarafından yüklenir).
const vscode = acquireVsCodeApi();
// Paneldeki beklenmeyen hatalar sessizce kaybolmasın: eklentiye bildirilir.
window.addEventListener('error', (e) => vscode.postMessage({ tur: 'panelHatasi', mesaj: String(e.message) + ' (' + e.lineno + ':' + e.colno + ')' }));
const IKONLAR = { tablo: '▦', liste: '☰', sozluk: '{}', yapi: '◆', matris: '⊞', sayi: '#', metin: 'Aa', mantik: '◐', yok: '∅', diger: '•' };
const TUR_ADI = { tablo: 'Tablo', liste: 'Liste', sozluk: 'Sözlük', yapi: 'Yapı', matris: 'Matris', sayi: 'Sayı', metin: 'Metin', mantik: 'Mantık', yok: 'Yok', diger: 'Değer' };
const sayiMi = (x) => x !== '' && x !== null && !isNaN(Number(x));
const el = (tag, metin, sinif) => { const e = document.createElement(tag); if (metin !== undefined) e.textContent = metin; if (sinif) e.className = sinif; return e; };

// Durum: değişkenler, açık sekmeler (her biri ayrı tablo), etkin sekme ve sekme başına arama/sıralama.
let durum = null, oncekiDegerler = {}, oturumBitti = false, duraklatildi = false;
function dugmeleriAyarla() {
  document.querySelectorAll('#yonlendirme button').forEach((b) => {
    // Durdur her zaman (oturum sürdükçe) kullanılabilir; adım düğmeleri yalnızca program duraklatılmışken.
    b.disabled = oturumBitti || (b.dataset.komut !== 'stop' && !duraklatildi);
  });
}
document.querySelectorAll('#yonlendirme button').forEach((b) => b.addEventListener('click', () => {
  if (b.disabled) return;
  if (b.dataset.komut !== 'stop') {
    duraklatildi = false;
    document.getElementById('neden').textContent = 'Çalışıyor…';
  }
  dugmeleriAyarla();
  vscode.postMessage({ tur: 'komut', komut: b.dataset.komut });
}));
const sekmeler = [];          // { ad, tur, veri, hata, arama, sirala }
let etkin = null;

window.addEventListener('message', (olay) => {
  const m = olay.data;
  if (m.tur === 'durum') {
    durum = m.veri; oturumBitti = false; duraklatildi = true;
    cizDegiskenler(); dugmeleriAyarla();
    // Açık sekmelerin içeriğini yeni adımın değerleriyle yenile
    sekmeler.forEach((s) => vscode.postMessage({ tur: 'tablo', ifade: s.ad }));
  } else if (m.tur === 'tablo') {
    const s = sekmeler.find((x) => x.ad === m.veri.baslik);
    if (s) { s.veri = m.veri; s.hata = ''; if (s.ad === etkin) cizPencere(); }
  } else if (m.tur === 'tabloHata') {
    const s = sekmeler.find((x) => x.ad === m.ifade);
    if (s) { s.hata = m.mesaj; if (s.ad === etkin) cizPencere(); }
  } else if (m.tur === 'hata') {
    document.getElementById('neden').textContent = m.mesaj;
  } else if (m.tur === 'bitti') {
    oturumBitti = true; duraklatildi = false; dugmeleriAyarla();
    document.getElementById('konum').textContent = 'Program bitti — tablolar son duruştaki değerleri gösteriyor';
    document.getElementById('neden').textContent = '';
  }
});

// tabloAc: tabloyu alt penceredeki sekmesinde açar (yoksa yeni sekme ekler) ve etkin yapar.
function tabloAc(d) {
  let s = sekmeler.find((x) => x.ad === d.ad);
  if (!s) {
    s = { ad: d.ad, tur: d.tur || 'tablo', veri: null, hata: '', arama: '', sirala: { sutun: -1, azalan: false } };
    sekmeler.push(s);
    if (oturumBitti) s.hata = 'Hata ayıklama oturumu bitti; tabloyu görmek için programı yeniden başlatın (F5).';
    else vscode.postMessage({ tur: 'tablo', ifade: d.ad });
  }
  etkin = s.ad;
  cizPencere(); cizDegiskenler();
}

function sekmeKapat(ad) {
  const i = sekmeler.findIndex((x) => x.ad === ad);
  if (i < 0) return;
  sekmeler.splice(i, 1);
  if (etkin === ad) etkin = sekmeler.length ? sekmeler[Math.min(i, sekmeler.length - 1)].ad : null;
  cizPencere(); cizDegiskenler();
}

function cizDegiskenler() {
  if (!durum) return;
  if (!oturumBitti) {
    document.getElementById('konum').textContent = durum.dosya + ' · Satır ' + durum.satir + ' · ' + durum.cerceve;
    document.getElementById('neden').textContent = durum.neden || '';
  }
  const q = document.getElementById('degiskenAra').value.toLocaleLowerCase('tr');
  const govde = document.getElementById('degiskenler');
  govde.replaceChildren();
  const yeniDegerler = {};
  durum.degiskenler.forEach((d) => { yeniDegerler[d.kapsam + ':' + d.ad] = d.deger; });
  const gorunen = durum.degiskenler.filter((d) => !q || d.ad.toLocaleLowerCase('tr').includes(q));
  document.getElementById('degiskenSayac').textContent = gorunen.length + ' / ' + durum.degiskenler.length + ' değişken';
  if (!gorunen.length) {
    const tr = el('tr'); const td = el('td', 'Değişken yok', 'bos'); td.colSpan = 5; tr.appendChild(td); govde.appendChild(tr);
  }
  gorunen.forEach((d) => {
    const anahtar = d.kapsam + ':' + d.ad;
    const tur = d.tur || 'diger';
    const acik = sekmeler.some((s) => s.ad === d.ad);
    const tr = el('tr');
    if (acik) tr.classList.add('acik');
    if (anahtar in oncekiDegerler && oncekiDegerler[anahtar] !== d.deger) tr.classList.add('degisti');
    const adHucre = el('td', undefined, 'ad'); adHucre.title = d.ad + ' (' + d.tip + ')';
    const ikon = el('span', IKONLAR[tur] || '•', 'ikon ' + tur); ikon.title = TUR_ADI[tur] || '';
    adHucre.appendChild(ikon);
    let degerHucre;
    if (d.tablo) {
      // <a href> tıklamalarını VS Code'un web görünümü yakalar; link görünümlü span kullanılır.
      const adLink = el('span', d.ad, 'link'); adLink.setAttribute('role', 'button'); adLink.tabIndex = 0;
      adLink.addEventListener('click', (e) => { e.stopPropagation(); tabloAc(d); });
      adLink.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); tabloAc(d); } });
      adHucre.appendChild(adLink);
      degerHucre = el('td', undefined, 'deger');
      const link = el('span', acik ? '● Açık — sekmeye git' : '▸ Tabloyu aç', 'link'); link.setAttribute('role', 'button'); link.tabIndex = 0;
      link.title = d.tip;
      link.addEventListener('click', (e) => { e.stopPropagation(); tabloAc(d); });
      link.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); tabloAc(d); } });
      degerHucre.appendChild(link);
      tr.classList.add('tablo-satiri');
      tr.title = 'Çift tıklayın: tabloyu alttaki pencerede aç';
      tr.addEventListener('dblclick', (e) => { if (e.target.tagName !== 'INPUT') tabloAc(d); });
    } else {
      adHucre.appendChild(document.createTextNode(d.ad));
      degerHucre = el('td', d.deger, 'deger'); degerHucre.title = d.deger;
    }
    const kalem = el('button', '✏️', 'kalem'); kalem.title = 'Değeri değiştir (TR ifadesi)';
    kalem.addEventListener('click', () => duzenle(degerHucre, d));
    const islem = el('td'); islem.appendChild(kalem);
    tr.append(el('td', d.kapsam, 'kapsam'), adHucre, el('td', d.tip, 'tip'), degerHucre, islem);
    govde.appendChild(tr);
  });
  oncekiDegerler = yeniDegerler;
}

function cizPencere() {
  const pencere = document.getElementById('pencere');
  const ayirici = document.getElementById('ayirici');
  const goster = sekmeler.length > 0;
  pencere.style.display = goster ? 'flex' : 'none';
  ayirici.style.display = goster ? 'block' : 'none';
  if (!goster) return;

  const serit = document.getElementById('sekmeler');
  serit.replaceChildren();
  sekmeler.forEach((s) => {
    const sek = el('div', undefined, 'sekme' + (s.ad === etkin ? ' etkin' : ''));
    sek.title = s.ad;
    sek.append(el('span', IKONLAR[s.tur] || '▦', 'ikon ' + s.tur), el('span', s.ad));
    const kapat = el('button', '✕', 'kapat'); kapat.title = 'Sekmeyi kapat';
    kapat.addEventListener('click', (e) => { e.stopPropagation(); sekmeKapat(s.ad); });
    sek.appendChild(kapat);
    sek.addEventListener('click', () => { etkin = s.ad; cizPencere(); });
    sek.addEventListener('auxclick', (e) => { if (e.button === 1) sekmeKapat(s.ad); });
    serit.appendChild(sek);
  });
  serit.appendChild(el('span', undefined, 'bosluk'));
  const hepsi = el('button', 'Tümünü kapat ✕'); hepsi.id = 'hepsiniKapat';
  hepsi.addEventListener('click', () => { sekmeler.splice(0); etkin = null; cizPencere(); cizDegiskenler(); });
  serit.appendChild(hepsi);

  const s = sekmeler.find((x) => x.ad === etkin);
  const ara = document.getElementById('tabloAra');
  ara.value = s ? s.arama : '';
  const kutu = document.getElementById('izgaraKutu');
  kutu.replaceChildren();
  const sayac = document.getElementById('tabloSayac');
  if (!s) { sayac.textContent = ''; return; }
  if (s.hata) { kutu.appendChild(el('div', 'Tablo gösterilemedi: ' + s.hata, 'bos')); sayac.textContent = ''; return; }
  if (!s.veri) { kutu.appendChild(el('div', 'Yükleniyor…', 'bos')); sayac.textContent = ''; return; }

  const sutunlar = ['#'].concat(s.veri.sutunlar || []);
  const satirlar = (s.veri.satirlar || []).map((r, i) => [String(i)].concat(r));
  const q = s.arama.toLocaleLowerCase('tr');
  let liste = satirlar.filter((r) => !q || r.some((h) => String(h).toLocaleLowerCase('tr').includes(q)));
  if (s.sirala.sutun >= 0) {
    const k = s.sirala.sutun;
    liste = liste.slice().sort((a, b) => {
      const c = sayiMi(a[k]) && sayiMi(b[k]) ? Number(a[k]) - Number(b[k]) : String(a[k]).localeCompare(String(b[k]), 'tr');
      return s.sirala.azalan ? -c : c;
    });
  }
  const t = el('table'); const thead = el('thead'); const bas = el('tr'); const govde = el('tbody');
  sutunlar.forEach((ad, i) => {
    const th = el('th', ad + (s.sirala.sutun === i ? (s.sirala.azalan ? ' ▼' : ' ▲') : ''));
    th.title = 'Sıralamak için tıklayın';
    th.addEventListener('click', () => { s.sirala = { sutun: i, azalan: s.sirala.sutun === i ? !s.sirala.azalan : false }; cizPencere(); });
    bas.appendChild(th);
  });
  liste.forEach((r) => {
    const satir = el('tr');
    r.forEach((h) => { const td = el('td', h, sayiMi(h) ? 'sayi' : ''); td.tabIndex = -1; satir.appendChild(td); });
    govde.appendChild(satir);
  });
  thead.appendChild(bas); t.append(thead, govde); kutu.appendChild(t);
  sayac.textContent = s.ad + ' — ' + liste.length + ' / ' + satirlar.length + ' satır · ' + (s.veri.sutunlar || []).length + ' sütun';
}

// Izgarada ok tuşlarıyla hücreden hücreye gezinme
document.getElementById('izgaraKutu').addEventListener('keydown', (e) => {
  const td = e.target.closest('td');
  if (!td) return;
  const tr = td.parentElement;
  let hedef = null;
  if (e.key === 'ArrowRight') hedef = td.nextElementSibling;
  else if (e.key === 'ArrowLeft') hedef = td.previousElementSibling;
  else if (e.key === 'ArrowDown' && tr.nextElementSibling) hedef = tr.nextElementSibling.children[td.cellIndex];
  else if (e.key === 'ArrowUp' && tr.previousElementSibling) hedef = tr.previousElementSibling.children[td.cellIndex];
  if (hedef) { e.preventDefault(); hedef.focus(); }
});
document.getElementById('izgaraKutu').addEventListener('click', (e) => { const td = e.target.closest('td'); if (td) td.focus(); });

document.getElementById('tabloAra').addEventListener('input', (e) => {
  const s = sekmeler.find((x) => x.ad === etkin);
  if (s) { s.arama = e.target.value; cizPencere(); document.getElementById('tabloAra').focus(); }
});
document.getElementById('degiskenAra').addEventListener('input', cizDegiskenler);
dugmeleriAyarla();

// Ayırıcı çubukla alt pencereyi boyutlandırma
(() => {
  const ayirici = document.getElementById('ayirici');
  const pencere = document.getElementById('pencere');
  let surukle = null;
  ayirici.addEventListener('mousedown', (e) => { surukle = { y: e.clientY, h: pencere.getBoundingClientRect().height }; e.preventDefault(); });
  window.addEventListener('mousemove', (e) => {
    if (!surukle) return;
    const yeni = Math.max(90, Math.min(window.innerHeight - 120, surukle.h - (e.clientY - surukle.y)));
    pencere.style.height = yeni + 'px';
  });
  window.addEventListener('mouseup', () => { surukle = null; });
})();

function duzenle(hucre, d) {
  const giris = el('input'); giris.value = d.tablo ? '' : d.deger; giris.placeholder = 'Yeni değer, ör. 0.20 veya [1, 2]'; giris.style.width = '95%';
  hucre.replaceChildren(giris); giris.focus(); giris.select();
  giris.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && giris.value.trim() !== '') vscode.postMessage({ tur: 'degistir', ad: d.ad, ifade: giris.value });
    if (e.key === 'Escape') cizDegiskenler();
  });
}
