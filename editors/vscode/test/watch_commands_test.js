// İzleme Masası düğme → ileti → VS Code komutu eşlemesinin testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/watch_commands_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const { WATCH_COMMANDS, resolveWatchCommand } = require('../watch_commands');
const { createSuite } = require('./fake_vscode');

const MEDIA = path.join(__dirname, '..', 'media');
const suite = createSuite('watch_commands_test', {
    summary: ({ passed }) => (process.exitCode ? 'watch_commands_test: kalan var' : `watch_commands_test: ${passed} test geçti`),
});
const test = suite.test;

// --- Eklenti tarafı: komut → VS Code komutu / DAP isteği ---

test('her düğme etkin oturumda VS Code komutuna gider', () => {
    assert.deepStrictEqual(resolveWatchCommand('continue', true), { kind: 'vscode', command: 'workbench.action.debug.continue' });
    assert.deepStrictEqual(resolveWatchCommand('next', true), { kind: 'vscode', command: 'workbench.action.debug.stepOver' });
    assert.deepStrictEqual(resolveWatchCommand('stepIn', true), { kind: 'vscode', command: 'workbench.action.debug.stepInto' });
    assert.deepStrictEqual(resolveWatchCommand('stepOut', true), { kind: 'vscode', command: 'workbench.action.debug.stepOut' });
    assert.deepStrictEqual(resolveWatchCommand('stop', true), { kind: 'vscode', command: 'workbench.action.debug.stop' });
});

test('etkin olmayan oturumda istek doğrudan DAP oturumuna gider', () => {
    for (const k of ['continue', 'next', 'stepIn', 'stepOut']) {
        assert.deepStrictEqual(resolveWatchCommand(k, false), { kind: 'dap', request: k, args: { threadId: 1 } });
    }
    assert.deepStrictEqual(resolveWatchCommand('stop', false), { kind: 'durdur' });
});

test('bilinmeyen komut yok sayılır', () => {
    for (const k of ['', 'devam', 'constructor', 'toString', '__proto__', undefined]) {
        assert.strictEqual(resolveWatchCommand(k, true), undefined, String(k));
    }
});

test('webview düğmeleri ile eşleme birebir', () => {
    const html = fs.readFileSync(path.join(MEDIA, 'watch_panel.html'), 'utf8');
    const buttons = [...html.matchAll(/data-komut="([^"]+)"/g)].map((m) => m[1]).sort();
    assert.deepStrictEqual(buttons, Object.keys(WATCH_COMMANDS).sort());
    assert.ok(buttons.includes('continue'), 'Devam düğmesi continue göndermeli');
});

// --- Webview tarafı: düğmeye basış → postMessage ---
// Küçük bir sahte DOM ile media/watch_panel.js çalıştırılır.

class FakeElement {
    constructor(tag, attributes) {
        this.tagName = (tag || 'div').toUpperCase();
        this.dataset = Object.assign({}, (attributes && attributes.dataset) || {});
        this.id = (attributes && attributes.id) || '';
        this.listeners = {};
        this.children = [];
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
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    dispatch(type, event) { for (const fn of this.listeners[type] || []) fn(Object.assign({ target: this, preventDefault() {}, stopPropagation() {} }, event)); }
    appendChild(c) { this.children.push(c); return c; }
    append(...c) { this.children.push(...c); }
    replaceChildren(...c) { this.children = c; }
    setAttribute() {}
    focus() {}
    select() {}
    closest() { return null; }
    getBoundingClientRect() { return { height: 100 }; }
}

function loadPanel() {
    const html = fs.readFileSync(path.join(MEDIA, 'watch_panel.html'), 'utf8');
    const buttons = [...html.matchAll(/data-komut="([^"]+)"/g)].map((m) => new FakeElement('button', { dataset: { komut: m[1] } }));
    const elements = {};
    const element = (id) => (elements[id] = elements[id] || new FakeElement('div', { id }));
    const header = new FakeElement('div');
    const sent = [];
    const windowObj = new FakeElement('window');
    const document = {
        querySelectorAll: (s) => (s === '#yonlendirme button' ? buttons : []),
        querySelector: (s) => (s === '.baslik' ? header : null),
        getElementById: element,
        createElement: (t) => new FakeElement(t),
        createTextNode: (t) => ({ textContent: t })
    };
    const context = {
        // İleti, gerçek web görünümünde olduğu gibi kopyalanarak geçer (başka bir JS ortamından gelir).
        acquireVsCodeApi: () => ({ postMessage: (m) => sent.push(JSON.parse(JSON.stringify(m))) }),
        document,
        window: windowObj,
        console
    };
    windowObj.innerHeight = 800;
    vm.createContext(context);
    vm.runInContext(fs.readFileSync(path.join(MEDIA, 'table_common.js'), 'utf8'), context, { filename: 'table_common.js' });
    vm.runInContext(fs.readFileSync(path.join(MEDIA, 'watch_panel.js'), 'utf8'), context, { filename: 'watch_panel.js' });
    const button = (k) => buttons.find((d) => d.dataset.komut === k);
    const sendState = (line, reason) => windowObj.dispatch('message', {
        data: { tur: 'durum', veri: { file: 'a.tr', line, frame: 'ana program', reason, variables: [] } }
    });
    return { button, sendState, sent, elements, windowObj };
}

test('Devam düğmesine basış continue iletisi gönderir (fare ve klavye)', () => {
    const m = loadPanel();
    m.sendState(25, 'Kesme Noktası (Satır 25)');
    // Fare: basıldığı anda (pointerdown) işlenir; ardından gelen click (detail 1) ikinci kez göndermez.
    m.button('continue').dispatch('pointerdown', { button: 0 });
    m.button('continue').dispatch('click', { detail: 1 });
    assert.deepStrictEqual(m.sent, [{ tur: 'komut', komut: 'continue' }]);
    // Program çalışırken adım düğmeleri pasiftir; yeni duruşta yeniden açılır.
    assert.strictEqual(m.button('continue').disabled, true);
    assert.strictEqual(m.button('stop').disabled, false);
    m.sendState(25, 'Kesme Noktası (Satır 25)');
    assert.strictEqual(m.button('continue').disabled, false);
    // Klavye (Enter/Boşluk) click'i detail 0 ile gelir.
    m.button('continue').dispatch('click', { detail: 0 });
    assert.deepStrictEqual(m.sent[1], { tur: 'komut', komut: 'continue' });
    // Sağ tık işlem yapmaz.
    m.sendState(25, 'Kesme Noktası (Satır 25)');
    m.button('continue').dispatch('pointerdown', { button: 2 });
    assert.strictEqual(m.sent.length, 2);
});

test('her düğme kendi komutunu gönderir', () => {
    const m = loadPanel();
    for (const k of ['next', 'stepIn', 'stepOut', 'continue', 'stop']) {
        m.sendState(3, 'Adım');
        m.button(k).dispatch('pointerdown', { button: 0 });
        assert.deepStrictEqual(m.sent[m.sent.length - 1], { tur: 'komut', komut: k });
    }
});

test('aynı satırda yeniden duruş sayaçla görünür', () => {
    const m = loadPanel();
    m.sendState(25, 'Kesme Noktası (Satır 25)');
    const first = m.elements.neden.textContent;
    m.button('continue').dispatch('pointerdown', { button: 0 });
    assert.strictEqual(m.elements.neden.textContent, 'Çalışıyor…');
    m.sendState(25, 'Kesme Noktası (Satır 25)');
    assert.notStrictEqual(m.elements.neden.textContent, first, 'Devam sonrası başlık değişmeli');
    assert.match(m.elements.neden.textContent, /2\. duruş/);
    // Oturum bitip yeniden başlayınca sayaç sıfırlanır.
    m.windowObj.dispatch('message', { data: { tur: 'bitti' } });
    assert.strictEqual(m.button('continue').disabled, true);
    m.sendState(1, 'Adım');
    assert.match(m.elements.neden.textContent, /1\. duruş/);
});

suite.run();
