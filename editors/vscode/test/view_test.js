// Kodlu dosyanın görünümü (view.js, docs/spec/17_kodlar.md 17.7), sahte vscode modülüyle ve GERÇEK 'tr'
// yorumlayıcısıyla: dosya kimliklerle açılır ve düzenlenir (sekme yönlendirilmez); "Görünümü Göster"
// görünümü yanda salt okunur açar, satırları dosyanınkidir; düzenleme ve yazım dili görünümü yeniler;
// görünüm kaydedilmez, koda geri çevrilmez; kısıtlı kipte açılmaz. Depo kökünden:
//   TR_YORUMLAYICI=bin/tr.exe node editors/vscode/test/view_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile, derlediği yorumlayıcıyla çalıştırır.
// Yorumlayıcı yoksa yorumlayıcı gerektiren testler atlanır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { setup, waitUntil, Uri, createSuite } = require('./fake_vscode');
const view = require('../view');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const TR = [process.env.TR_YORUMLAYICI, path.join(repoRoot, 'bin', process.platform === 'win32' ? 'tr.exe' : 'tr')]
    .find((y) => y && fs.existsSync(y));

// Üçüncü argüman false ise test yorumlayıcı gerektirmez; yorumlayıcı yoksa gerekenler atlanır.
const suite = createSuite('view_test', {
    skip: (needsTr) => needsTr !== false && !TR,
    afterAll: async () => {
        // Görünümün yenileme zamanlayıcıları bitsin; sonra geçici klasör silinir.
        await new Promise((resolve) => setTimeout(resolve, 400));
        fs.rmSync(root, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
    },
    summary: ({ passed, total, skipped }) => `view_test: ${passed}/${total} geçti` + (skipped ? ` (${skipped} test atlandı: TR yorumlayıcısı yok; TR_YORUMLAYICI ile verin)` : ''),
    exit: true,
});
const test = suite.test;

const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tr-eklenti-gorunum-'));
let counter = 0;

const CODED = '# TR 4\r\nTR_var sayaç = 0\r\nTR_for i TR_in 1..3 {\r\n    sayaç += i\r\n}\r\nTR_print("sayaç: \\(sayaç)") # yorum\r\n';
const TURKISH_VIEW = '# TR 4 view=tr\r\ndeğişken sayaç = 0\r\ndöngü i içinde 1..3 {\r\n    sayaç += i\r\n}\r\nyazdır("sayaç: \\(sayaç)") # yorum\r\n';
const ENGLISH_VIEW = '# TR 4 view=en\r\nvar sayaç = 0\r\nfor i in 1..3 {\r\n    sayaç += i\r\n}\r\nprint("sayaç: \\(sayaç)") # yorum\r\n';

// makeEnv bir klasörde kodlu bir dosya yazar ve eklentiyi kurar.
function makeEnv(s) {
    const folder = path.join(root, String(++counter));
    fs.mkdirSync(folder);
    const file = path.join(folder, 'program.tr');
    fs.writeFileSync(file, CODED);
    const o = setup(Object.assign({ setting: TR, folder, settings: { writingLanguage: 'tr' } }, s));
    return Object.assign(o, { folder, file, uri: Uri.file(file) });
}

// openView dosyayı açar ve görünüm komutunu çalıştırır; görünüm belgesini verir.
async function openView(o) {
    await o.vscode.window.showTextDocument(o.uri);
    await o.commands['tr.showView']();
    const doc = o.vscode.workspace.textDocuments.find((d) => d.uri.scheme === view.VIEW_SCHEME);
    assert.ok(doc, 'görünüm açılmalı');
    return doc;
}

test('show: tr show - --lang ile görünümü verir; ayrıştırılamayan metin EXIT_SYNTAX', async () => {
    const s = await view.show(TR, 'tr', CODED);
    assert.strictEqual(s.code, 0, s.error);
    assert.strictEqual(s.output.toString('utf8'), TURKISH_VIEW);
    const bad = await view.show(TR, 'tr', '# TR 4\nTR_if {\n');
    assert.strictEqual(bad.code, view.EXIT_SYNTAX);
    assert.ok(view.errorSummary(bad.error).length > 0, bad.error);
});

test('errorSummary: ilk hata satırı, çerçeve ve başlıklar atılır', () => {
    assert.strictEqual(view.errorSummary('❌ Sözdizimi Hataları:\n  • Satır 2, Sütun 1: eksik\n     2 | x\n'), 'Satır 2, Sütun 1: eksik');
}, false);

test('kodlu dosya olduğu gibi açılır ve düzenlenir: sekme yönlendirilmez, dosya sistemi sağlayıcısı yok', async () => {
    const o = makeEnv();
    const editor = await o.vscode.window.showTextDocument(o.uri);
    assert.strictEqual(editor.document.getText(), CODED);
    assert.strictEqual(editor.document.uri.scheme, 'file');
    assert.deepStrictEqual(Object.keys(o.filesystems), [], 'görünümde düzenleme yok: dosya sistemi sağlayıcısı kurulmamalı');
    editor.document.edit(CODED.replace('sayaç: ', 'toplam: '));
    assert.ok(await editor.document.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), CODED.replace('sayaç: ', 'toplam: '), 'dosya yazıldığı gibi kaydedilir');
});

test('Görünümü Göster: görünüm yanda salt okunur açılır, satırları dosyanınki; kaydedilmez', async () => {
    const o = makeEnv();
    const doc = await openView(o);
    assert.strictEqual(doc.getText(), TURKISH_VIEW);
    assert.strictEqual(doc.getText().split('\n').length, CODED.split('\n').length);
    assert.strictEqual(doc.languageId, 'tr');
    assert.strictEqual(doc.uri.fsPath, o.file, 'görünümün adresi dosyanın kendi yolu');
    assert.strictEqual(await doc.save(), false, 'görünüm kaydedilmez');
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), CODED, 'dosya değişmemeli');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file', 'odak dosyada kalır');
    assert.strictEqual(o.log.warnings.length + o.log.errors.length, 0, [...o.log.warnings, ...o.log.errors].join('\n'));
});

test('dosya düzenlenince görünüm yenilenir (kaydedilmemiş metin); ayrıştırılamayan metinde eski görünüm kalır, bir kez uyarılır', async () => {
    const o = makeEnv();
    const doc = await openView(o);
    const source = o.vscode.workspace.textDocuments.find((d) => d.uri.scheme === 'file');
    source.edit(CODED.replace('"sayaç: ', '"toplam: '));
    await waitUntil(() => doc.getText().includes('yazdır("toplam: '), 'düzenlemeden sonra görünüm');
    source.edit(source.getText() + 'TR_if {\r\n');
    await waitUntil(() => o.log.warnings.length === 1, 'sözdizimi hatası uyarısı');
    assert.ok(doc.getText().includes('yazdır("toplam: '), 'eski görünüm kalır');
    source.edit(source.getText() + '}\r\n');
    source.edit(source.getText() + '\r\n');
    await new Promise((resolve) => setTimeout(resolve, 700));
    assert.strictEqual(o.log.warnings.length, 1, 'uyarı bir kez: ' + o.log.warnings.join('\n'));
});

test('yazım dili değişince açık görünüm yeni dille yenilenir', async () => {
    const o = makeEnv();
    const doc = await openView(o);
    o.settingValues.writingLanguage = 'en';
    o.events.setting.fire({ affectsConfiguration: (name) => name === 'tr.writingLanguage' });
    await waitUntil(() => doc.getText() === ENGLISH_VIEW, 'İngilizce görünüm');
});

test('görünüm etkinken çalıştır dosyanın kendisinde çalışır', async () => {
    const o = makeEnv();
    const doc = await openView(o);
    await o.vscode.window.showTextDocument(doc);
    await o.commands['tr.run']();
    assert.strictEqual(o.log.tasks.length, 1, [...o.log.errors].join('\n'));
    assert.deepStrictEqual(o.log.tasks[0].execution.args, [o.file]);
});

test('kısıtlı kipte görünüm açılmaz, nedeni söylenir; yorumlayıcı çalışmaz', async () => {
    const o = makeEnv({ trusted: false });
    await o.vscode.window.showTextDocument(o.uri);
    await o.commands['tr.showView']();
    assert.ok(!o.vscode.workspace.textDocuments.some((d) => d.uri.scheme === view.VIEW_SCHEME));
    assert.strictEqual(o.log.warnings.length, 1);
}, false);

test('.tr olmayan belgede görünüm komutu hata verir', async () => {
    const o = makeEnv();
    const other = path.join(o.folder, 'not.txt');
    fs.writeFileSync(other, 'x');
    await o.vscode.window.showTextDocument(Uri.file(other));
    await o.commands['tr.showView']();
    assert.strictEqual(o.log.errors.length, 1);
}, false);

suite.run();
