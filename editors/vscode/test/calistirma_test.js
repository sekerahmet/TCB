// Eklentinin çalıştırma ve hata ayıklama yolları, sahte vscode modülüyle (sahte_vscode.js):
// F5 / Ctrl+F5 yapılandırmasından yorumlayıcıya giden süreç ve argümanlar. Bağımlılıksız; depo
// kökünden:
//   node editors/vscode/test/calistirma_test.js
// (go test ./cmd/tr/uctan_uca bunu TestEklentiJSTestleri ile çalıştırır.)
'use strict';

const assert = require('assert');
const path = require('path');
const { kur, bekle } = require('./sahte_vscode');
const { programArgumanlari } = require('../program_argumanlari');

const testler = [];
function test(ad, fn) {
    testler.push({ ad, fn });
}

// Yorumlayıcı olarak var olan bir dosya gerekir (yalnızca yolu kullanılır, çalıştırılmaz).
const YORUMLAYICI = process.execPath;
const KLASOR = path.resolve(__dirname, '..');
const DOSYA = path.join(KLASOR, 'deneme.tr');

test('programArgumanlari: yoksa boş, metin dizisiyse kopyası, değilse hata', () => {
    assert.deepStrictEqual(programArgumanlari({}), { argumanlar: [] });
    assert.deepStrictEqual(programArgumanlari({ args: null }), { argumanlar: [] });
    const args = ['--adım', '5', 'İçerik ı.txt'];
    const s = programArgumanlari({ args });
    assert.deepStrictEqual(s, { argumanlar: args });
    assert.notStrictEqual(s.argumanlar, args);
    for (const yanlis of ['--adım 5', [5], ['a', 1], { a: 1 }]) {
        assert.ok(programArgumanlari({ args: yanlis }).hata, JSON.stringify(yanlis));
    }
});

test('Ctrl+F5 launch.json ile: dosya ve "args" görevle yorumlayıcıya gider', async () => {
    const { kayit, saglayici } = kur({ ayar: YORUMLAYICI, klasor: KLASOR });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: DOSYA, args: ['--adım', '5'], noDebug: true };
    const cozulen = await saglayici.resolveDebugConfiguration(undefined, y);
    assert.strictEqual(cozulen, y);
    assert.strictEqual(saglayici.resolveDebugConfigurationWithSubstitutedVariables(undefined, cozulen), undefined, 'hata ayıklama oturumu başlamamalı');
    await bekle();
    assert.strictEqual(kayit.gorevler.length, 1, 'bir görev başlamalı');
    const e = kayit.gorevler[0].execution;
    assert.strictEqual(e.surec, YORUMLAYICI);
    assert.deepStrictEqual(e.argumanlar, [DOSYA, '--adım', '5']);
    assert.strictEqual(e.secenekler.cwd, KLASOR);
});

test('Ctrl+F5 klasörsüz pencerede: argümanlar terminale de gider', async () => {
    const { kayit, saglayici } = kur({ ayar: YORUMLAYICI });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: DOSYA, args: ['a b', 'ç'], noDebug: true };
    saglayici.resolveDebugConfigurationWithSubstitutedVariables(undefined, await saglayici.resolveDebugConfiguration(undefined, y));
    await bekle();
    assert.strictEqual(kayit.terminaller.length, 1);
    assert.strictEqual(kayit.terminaller[0].shellPath, YORUMLAYICI);
    assert.deepStrictEqual(kayit.terminaller[0].shellArgs, [DOSYA, 'a b', 'ç']);
});

test('Ctrl+F5 launch.json olmadan: açık dosya çalışır, hata ayıklama başlamaz', async () => {
    const { kayit, saglayici } = kur({ ayar: YORUMLAYICI, klasor: KLASOR, etkinDosya: DOSYA });
    const cozulen = await saglayici.resolveDebugConfiguration(undefined, { noDebug: true });
    assert.ok(cozulen && cozulen.noDebug, '"hata ayıklamadan" işareti korunmalı');
    assert.strictEqual(saglayici.resolveDebugConfigurationWithSubstitutedVariables(undefined, cozulen), undefined);
    await bekle();
    assert.strictEqual(kayit.gorevler.length, 1);
    assert.deepStrictEqual(kayit.gorevler[0].execution.argumanlar, [DOSYA]);
});

test('F5: yapılandırma olduğu gibi kalır, yorumlayıcı "dap" ile başlar', async () => {
    const { kayit, saglayici, surec } = kur({ ayar: YORUMLAYICI, klasor: KLASOR });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: DOSYA, args: ['--adım', '5'], cwd: KLASOR };
    const cozulen = await saglayici.resolveDebugConfiguration(undefined, y);
    // Kodlu olmayan dosyada görünüm adresi eklenmez (kodlu dosyada: kodlu_gorunum_test.js).
    assert.strictEqual(await saglayici.resolveDebugConfigurationWithSubstitutedVariables(undefined, cozulen), y);
    assert.strictEqual(y.gorunumAdresi, undefined);
    // "args" başlatma isteğiyle (launch) 'tr dap'a gider; süreç argümanı yalnızca 'dap'.
    const d = surec.createDebugAdapterDescriptor({ configuration: cozulen });
    assert.strictEqual(d.komut, YORUMLAYICI);
    assert.deepStrictEqual(d.argumanlar, ['dap']);
    assert.strictEqual(d.secenekler.cwd, KLASOR);
    assert.strictEqual(kayit.gorevler.length + kayit.terminaller.length, 0);
});

test('yanlış "args" Türkçe bir mesajla durdurulur', async () => {
    const { kayit, saglayici } = kur({ ayar: YORUMLAYICI, klasor: KLASOR });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: DOSYA, args: '--adım 5' };
    assert.strictEqual(await saglayici.resolveDebugConfiguration(undefined, y), undefined);
    assert.strictEqual(kayit.hatalar.length, 1);
    assert.ok(kayit.hatalar[0].includes('"args"'), kayit.hatalar[0]);
});

test('güvenilmeyen çalışma alanında Ctrl+F5 yorumlayıcıyı çalıştırmaz', async () => {
    const { kayit, saglayici } = kur({ ayar: YORUMLAYICI, klasor: KLASOR, guvenilir: false });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: DOSYA, noDebug: true };
    assert.strictEqual(await saglayici.resolveDebugConfiguration(undefined, y), undefined);
    await bekle();
    assert.strictEqual(kayit.gorevler.length + kayit.terminaller.length, 0);
    assert.strictEqual(kayit.uyarilar.length, 1);
});

(async () => {
    let gecen = 0;
    for (const { ad, fn } of testler) {
        try {
            await fn();
            gecen++;
        } catch (e) {
            console.error('KALDI: ' + ad + '\n' + (e && e.stack));
            process.exitCode = 1;
        }
    }
    console.log(`calistirma_test: ${gecen}/${testler.length} geçti`);
})();
