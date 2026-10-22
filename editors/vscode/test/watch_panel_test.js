// İzleme Masası'nın eklenti tarafı (extension.js), sahte vscode modülüyle (fake_vscode.js):
// panelden gelen iletilerin hata ayıklayıcıya hangi DAP isteğiyle gittiği. Bağımlılıksız; depo
// kökünden:
//   node editors/vscode/test/watch_panel_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const path = require('path');
const { setup, wait, createSuite } = require('./fake_vscode');

const suite = createSuite('watch_panel_test', { exit: true });
const test = suite.test;

// openPanel eklentiyi kurar, sahte bir TR oturumuyla paneli açar ve oturumun isteklerini kaydeder.
async function openPanel(replies) {
    const o = setup({ setting: process.execPath, folder: path.resolve(__dirname, '..') });
    const requests = [];
    const session = {
        id: 's1', type: 'tr',
        customRequest: async (command, args) => {
            requests.push({ command, args });
            const reply = replies && replies[command];
            return typeof reply === 'function' ? reply(args) : (reply || {});
        }
    };
    o.vscode.debug.activeDebugSession = session;
    await o.commands['tr.izlemeMasasi']();
    await wait();
    requests.length = 0; // açılışta giden "trDegiskenler" isteği sayılmaz
    return Object.assign(o, { requests, panel: o.log.panels[0] });
}

const SCOPES = { scopes: [{ name: 'Yerel', variablesReference: 3 }, { name: 'Genel', variablesReference: 4 }] };

test('✏️ değer değiştirme: kapsamın başvurusuyla setVariable; evaluate (deyim çalıştırma) yok', async () => {
    const o = await openPanel({ scopes: SCOPES, setVariable: { value: '10' } });
    await o.panel.receive({ tur: 'degistir', kapsam: 'Genel', ad: 'sayi', ifade: '10' });
    const commands = o.requests.map((r) => r.command);
    assert.ok(!commands.includes('evaluate'), 'evaluate kullanılmamalı: ' + commands);
    const set = o.requests.find((r) => r.command === 'setVariable');
    assert.deepStrictEqual(set && set.args, { variablesReference: 4, name: 'sayi', value: '10' });
    assert.deepStrictEqual(o.log.errors, []);

    o.requests.length = 0;
    await o.panel.receive({ tur: 'degistir', kapsam: 'Yerel', ad: 'sayi', ifade: '5' });
    assert.deepStrictEqual(o.requests.find((r) => r.command === 'setVariable').args, { variablesReference: 3, name: 'sayi', value: '5' });
});

test('✏️ değer değiştirme: ifade hiçbir zaman "ad = ifade" deyimine çevrilmez', async () => {
    const o = await openPanel({ scopes: SCOPES, setVariable: {} });
    await o.panel.receive({ tur: 'degistir', kapsam: 'Genel', ad: 'sayi', ifade: '1; yazdır("x")' });
    const set = o.requests.find((r) => r.command === 'setVariable');
    assert.strictEqual(set.args.value, '1; yazdır("x")');
    assert.strictEqual(set.args.name, 'sayi');
});

test('✏️ bilinmeyen kapsam: istek gitmez, hata gösterilir', async () => {
    const o = await openPanel({ scopes: SCOPES });
    await o.panel.receive({ tur: 'degistir', kapsam: 'Başka', ad: 'sayi', ifade: '1' });
    assert.ok(!o.requests.some((r) => r.command === 'setVariable'));
    assert.strictEqual(o.log.errors.length, 1);
});

test('tablo isteği kapsamla gider; yanıt ve hata kapsamıyla panele döner (aynı adlı Yerel/Genel sekmeler çakışmaz)', async () => {
    const o = await openPanel({ trTablo: (args) => (args.kapsam === 'Yok' ? Promise.reject(new Error('yok')) : { baslik: args.ifade, sutunlar: [args.kapsam] }) });
    await o.panel.receive({ tur: 'tablo', kapsam: 'Genel', ifade: 'liste' });
    await o.panel.receive({ tur: 'tablo', kapsam: 'Yerel', ifade: 'liste' });
    await o.panel.receive({ tur: 'tablo', kapsam: 'Yok', ifade: 'liste' });
    assert.deepStrictEqual(o.requests.filter((r) => r.command === 'trTablo').map((r) => r.args), [
        { ifade: 'liste', kapsam: 'Genel' }, { ifade: 'liste', kapsam: 'Yerel' }, { ifade: 'liste', kapsam: 'Yok' }]);
    const replies = o.panel.messages.filter((m) => m.tur === 'tablo' || m.tur === 'tabloHata').map((m) => [m.tur, (m.veri || m).kapsam]);
    assert.deepStrictEqual(replies, [['tablo', 'Genel'], ['tablo', 'Yerel'], ['tabloHata', 'Yok']]);
});

suite.run();
