// İzleme Masası düğme → ileti → VS Code komutu eşlemesinin testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/izleme_komutlari_test.js
// (go test ./cmd/tr/uctan_uca bunu TestEklentiJSTestleri ile çalıştırır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const { IZLEME_KOMUTLARI, izlemeKomutuCoz } = require('../izleme_komutlari');

const MEDIA = path.join(__dirname, '..', 'media');
let gecen = 0;
function test(ad, fn) {
    try {
        fn();
        gecen++;
    } catch (e) {
        console.error('KALDI: ' + ad + '\n' + (e && e.stack));
        process.exitCode = 1;
    }
}

// --- Eklenti tarafı: komut → VS Code komutu / DAP isteği ---

test('her düğme etkin oturumda VS Code komutuna gider', () => {
    assert.deepStrictEqual(izlemeKomutuCoz('continue', true), { tur: 'vscode', komut: 'workbench.action.debug.continue' });
    assert.deepStrictEqual(izlemeKomutuCoz('next', true), { tur: 'vscode', komut: 'workbench.action.debug.stepOver' });
    assert.deepStrictEqual(izlemeKomutuCoz('stepIn', true), { tur: 'vscode', komut: 'workbench.action.debug.stepInto' });
    assert.deepStrictEqual(izlemeKomutuCoz('stepOut', true), { tur: 'vscode', komut: 'workbench.action.debug.stepOut' });
    assert.deepStrictEqual(izlemeKomutuCoz('stop', true), { tur: 'vscode', komut: 'workbench.action.debug.stop' });
});

test('etkin olmayan oturumda istek doğrudan DAP oturumuna gider', () => {
    for (const k of ['continue', 'next', 'stepIn', 'stepOut']) {
        assert.deepStrictEqual(izlemeKomutuCoz(k, false), { tur: 'dap', istek: k, argumanlar: { threadId: 1 } });
    }
    assert.deepStrictEqual(izlemeKomutuCoz('stop', false), { tur: 'durdur' });
});

test('bilinmeyen komut yok sayılır', () => {
    for (const k of ['', 'devam', 'constructor', 'toString', '__proto__', undefined]) {
        assert.strictEqual(izlemeKomutuCoz(k, true), undefined, String(k));
    }
});

test('webview düğmeleri ile eşleme birebir', () => {
    const html = fs.readFileSync(path.join(MEDIA, 'izleme_masasi.html'), 'utf8');
    const dugmeler = [...html.matchAll(/data-komut="([^"]+)"/g)].map((m) => m[1]).sort();
    assert.deepStrictEqual(dugmeler, Object.keys(IZLEME_KOMUTLARI).sort());
    assert.ok(dugmeler.includes('continue'), 'Devam düğmesi continue göndermeli');
});

// --- Webview tarafı: düğmeye basış → postMessage ---
// Küçük bir sahte DOM ile media/izleme_masasi.js çalıştırılır.

class SahteOge {
    constructor(etiket, nitelikler) {
        this.tagName = (etiket || 'div').toUpperCase();
        this.dataset = Object.assign({}, (nitelikler && nitelikler.dataset) || {});
        this.id = (nitelikler && nitelikler.id) || '';
        this.dinleyiciler = {};
        this.cocuklar = [];
        this.textContent = '';
        this.title = '';
        this.className = '';
        this.disabled = false;
        this.value = '';
        this.style = {};
        this.offsetWidth = 0;
        const siniflar = new Set();
        this.classList = { add: (c) => siniflar.add(c), remove: (c) => siniflar.delete(c), contains: (c) => siniflar.has(c) };
    }
    addEventListener(tur, fn) { (this.dinleyiciler[tur] = this.dinleyiciler[tur] || []).push(fn); }
    gonder(tur, olay) { for (const fn of this.dinleyiciler[tur] || []) fn(Object.assign({ target: this, preventDefault() {}, stopPropagation() {} }, olay)); }
    appendChild(c) { this.cocuklar.push(c); return c; }
    append(...c) { this.cocuklar.push(...c); }
    replaceChildren(...c) { this.cocuklar = c; }
    setAttribute() {}
    focus() {}
    select() {}
    closest() { return null; }
    getBoundingClientRect() { return { height: 100 }; }
}

function masaYukle() {
    const html = fs.readFileSync(path.join(MEDIA, 'izleme_masasi.html'), 'utf8');
    const dugmeler = [...html.matchAll(/data-komut="([^"]+)"/g)].map((m) => new SahteOge('button', { dataset: { komut: m[1] } }));
    const ogeler = {};
    const oge = (id) => (ogeler[id] = ogeler[id] || new SahteOge('div', { id }));
    const baslik = new SahteOge('div');
    const gonderilen = [];
    const pencere = new SahteOge('window');
    const document = {
        querySelectorAll: (s) => (s === '#yonlendirme button' ? dugmeler : []),
        querySelector: (s) => (s === '.baslik' ? baslik : null),
        getElementById: oge,
        createElement: (t) => new SahteOge(t),
        createTextNode: (t) => ({ textContent: t })
    };
    const baglam = {
        // İleti, gerçek web görünümünde olduğu gibi kopyalanarak geçer (başka bir JS ortamından gelir).
        acquireVsCodeApi: () => ({ postMessage: (m) => gonderilen.push(JSON.parse(JSON.stringify(m))) }),
        document,
        window: pencere,
        console
    };
    pencere.innerHeight = 800;
    vm.createContext(baglam);
    vm.runInContext(fs.readFileSync(path.join(MEDIA, 'izleme_masasi.js'), 'utf8'), baglam, { filename: 'izleme_masasi.js' });
    const dugme = (k) => dugmeler.find((d) => d.dataset.komut === k);
    const durum = (satir, neden) => pencere.gonder('message', {
        data: { tur: 'durum', veri: { dosya: 'a.tr', satir, cerceve: 'ana program', neden, degiskenler: [] } }
    });
    return { dugme, durum, gonderilen, ogeler, pencere };
}

test('Devam düğmesine basış continue iletisi gönderir (fare ve klavye)', () => {
    const m = masaYukle();
    m.durum(25, 'Kesme Noktası (Satır 25)');
    // Fare: basıldığı anda (pointerdown) işlenir; ardından gelen click (detail 1) ikinci kez göndermez.
    m.dugme('continue').gonder('pointerdown', { button: 0 });
    m.dugme('continue').gonder('click', { detail: 1 });
    assert.deepStrictEqual(m.gonderilen, [{ tur: 'komut', komut: 'continue' }]);
    // Program çalışırken adım düğmeleri pasiftir; yeni duruşta yeniden açılır.
    assert.strictEqual(m.dugme('continue').disabled, true);
    assert.strictEqual(m.dugme('stop').disabled, false);
    m.durum(25, 'Kesme Noktası (Satır 25)');
    assert.strictEqual(m.dugme('continue').disabled, false);
    // Klavye (Enter/Boşluk) click'i detail 0 ile gelir.
    m.dugme('continue').gonder('click', { detail: 0 });
    assert.deepStrictEqual(m.gonderilen[1], { tur: 'komut', komut: 'continue' });
    // Sağ tık işlem yapmaz.
    m.durum(25, 'Kesme Noktası (Satır 25)');
    m.dugme('continue').gonder('pointerdown', { button: 2 });
    assert.strictEqual(m.gonderilen.length, 2);
});

test('her düğme kendi komutunu gönderir', () => {
    const m = masaYukle();
    for (const k of ['next', 'stepIn', 'stepOut', 'continue', 'stop']) {
        m.durum(3, 'Adım');
        m.dugme(k).gonder('pointerdown', { button: 0 });
        assert.deepStrictEqual(m.gonderilen[m.gonderilen.length - 1], { tur: 'komut', komut: k });
    }
});

test('aynı satırda yeniden duruş sayaçla görünür', () => {
    const m = masaYukle();
    m.durum(25, 'Kesme Noktası (Satır 25)');
    const ilk = m.ogeler.neden.textContent;
    m.dugme('continue').gonder('pointerdown', { button: 0 });
    assert.strictEqual(m.ogeler.neden.textContent, 'Çalışıyor…');
    m.durum(25, 'Kesme Noktası (Satır 25)');
    assert.notStrictEqual(m.ogeler.neden.textContent, ilk, 'Devam sonrası başlık değişmeli');
    assert.match(m.ogeler.neden.textContent, /2\. duruş/);
    // Oturum bitip yeniden başlayınca sayaç sıfırlanır.
    m.pencere.gonder('message', { data: { tur: 'bitti' } });
    assert.strictEqual(m.dugme('continue').disabled, true);
    m.durum(1, 'Adım');
    assert.match(m.ogeler.neden.textContent, /1\. duruş/);
});

if (process.exitCode) {
    console.error('izleme_komutlari_test: kalan var');
} else {
    console.log('izleme_komutlari_test: ' + gecen + ' test geçti');
}
