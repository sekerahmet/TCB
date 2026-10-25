// Kodlu dosyaların Türkçe görünümü (coded_view.js, docs/spec/17_kodlar.md "Düzenleyicide"), sahte vscode
// modülüyle ve GERÇEK 'tr' yorumlayıcısıyla: kodlu dosyayı aç → Türkçe görünüm (başlık gizli),
// düzenle → kaydet → diskte kodlu ve 'tr göster' ile aynı; geri al; bozuk dosyanın kayıpsız
// kaydı; satır eşlemesi (hata mesajı ve hata ayıklayıcı); kısıtlı kip. Depo kökünden:
//   TR_YORUMLAYICI=bin/tr.exe node editors/vscode/test/coded_view_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile, derlediği yorumlayıcıyla çalıştırır.
// Yorumlayıcı yoksa yorumlayıcı gerektiren testler atlanır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { setup, waitUntil, Uri, FakeState, createSuite } = require('./fake_vscode');
const codedFormat = require('../coded_format');
const { globEscape } = require('../coded_filesystem');
const { PENDING_KEY, PLAIN_PENDING_KEY } = require('../coded_view');

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const TR = [process.env.TR_YORUMLAYICI, path.join(repoRoot, 'bin', process.platform === 'win32' ? 'tr.exe' : 'tr')]
    .find((y) => y && fs.existsSync(y));

// Üçüncü argüman false ise test yorumlayıcı gerektirmez; yorumlayıcı yoksa gerekenler atlanır.
const suite = createSuite('coded_view_test', {
    skip: (needsTr) => needsTr !== false && !TR,
    afterAll: async () => {
        // Eklentinin yedek zamanlayıcıları (sekme yönlendirmesi) bitsin; sonra geçici klasör silinir.
        await new Promise((resolve) => setTimeout(resolve, 300));
        fs.rmSync(root, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
    },
    summary: ({ passed, total, skipped }) => `coded_view_test: ${passed}/${total} geçti` + (skipped ? ` (${skipped} test atlandı: TR yorumlayıcısı yok; TR_YORUMLAYICI ile verin)` : ''),
    exit: true, // Eklentinin yedek zamanlayıcıları sınamayı bekletmesin.
});
const test = suite.test;

const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tr-eklenti-kodlu-'));
let counter = 0;
function makeFolder() {
    const k = path.join(root, String(++counter));
    fs.mkdirSync(k);
    return k;
}

function tr(...args) {
    const s = spawnSync(TR, args.slice(0, -1), { input: args[args.length - 1], windowsHide: true });
    return { code: s.status, output: s.stdout.toString('utf8'), error: s.stderr.toString('utf8') };
}
function encode(text) {
    const s = tr('kodla', '-', text);
    assert.strictEqual(s.code, 0, s.error);
    return s.output;
}
function showText(file) {
    const s = spawnSync(TR, ['göster', file], { windowsHide: true });
    assert.strictEqual(s.status, 0, s.stderr.toString());
    return s.stdout.toString('utf8');
}

const TURKISH_TEXT = 'değişken sayaç = 0\r\ndöngü i içinde 1..3 {\r\n    sayaç += i\r\n}\r\nyazdır("sayaç: {sayaç}") // yorum\r\n';

// ortam bir klasörde kodlu bir dosya yazar ve eklentiyi kurar.
function makeEnv(s) {
    const folder = makeFolder();
    const file = path.join(folder, 'program.tr');
    fs.writeFileSync(file, s && s.disk !== undefined ? s.disk : encode(TURKISH_TEXT));
    const o = setup(Object.assign({ setting: TR, folder }, s));
    return Object.assign(o, { folder, file, uri: Uri.file(file) });
}

// ac dosyayı dosya gezgininden açılmış gibi açar ve eklentinin yönlendirmesini bekler.
async function open(o, uri, options, scheme) {
    await o.vscode.window.showTextDocument(uri, Object.assign({ preview: true }, options));
    const expected = scheme || 'tr-kodlu';
    await waitUntil(() => o.vscode.window.activeTextEditor && o.vscode.window.activeTextEditor.document.uri.scheme === expected, expected + ' görünümü');
    return o.vscode.window.activeTextEditor;
}

function tabUris(o) {
    return o.vscode.window.tabGroups.all.flatMap((g) => g.tabs.map((t) => t.input.uri.toString()));
}

test('isCoded: başlık yalnızca ilk satırın tamamıysa (BOM, LF, CRLF, tek satır)', () => {
    const b = (m) => Buffer.from(m, 'utf8');
    const yes = { '//! tr kodlu 3 dil=tr\nTR_kw_var x = 1\n': 'tr', '//! tr kodlu 3 dil=en\r\n': 'en', '﻿//! tr kodlu 3 dil=tr\n': 'tr', '//! tr kodlu 3 dil=en': 'en' };
    for (const [text, language] of Object.entries(yes)) {
        assert.ok(codedFormat.isCoded(b(text)), JSON.stringify(text));
        assert.strictEqual(codedFormat.fileLanguage(b(text)), language, JSON.stringify(text));
    }
    for (const no of ['//! tr kodlu 1\n', '//! tr kodlu 2 dil=en\n', '//! tr kodlu 3\n', '//! tr kodlu 3 dil=\n', '//! tr kodlu 3 dil=en \n', ' //! tr kodlu 3 dil=en\n',
        '//! tr kodlu 3 dil=en\r', 'değişken x = 1\n', '', '\n//! tr kodlu 3 dil=en\n']) {
        assert.ok(!codedFormat.isCoded(b(no)), JSON.stringify(no));
        assert.strictEqual(codedFormat.fileLanguage(b(no)), null, JSON.stringify(no));
    }
    assert.ok(codedFormat.isCoded(new Uint8Array(b('//! tr kodlu 3 dil=tr\n'))), 'Uint8Array da okunur');
    // withLanguage yalnızca başlığın dilini değiştirir: BOM, satır sonu ve gövde aynı kalır.
    assert.strictEqual(codedFormat.withLanguage(b('﻿//! tr kodlu 3 dil=tr\r\nTR_kw_var x = 1\r\n'), 'en').toString('utf8'), '﻿//! tr kodlu 3 dil=en\r\nTR_kw_var x = 1\r\n');
    assert.strictEqual(codedFormat.withLanguage(b('//! tr kodlu 3 dil=en'), 'tr').toString('utf8'), '//! tr kodlu 3 dil=tr');
    assert.strictEqual(codedFormat.withLanguage(b('değişken x = 1\n'), 'en').toString('utf8'), 'değişken x = 1\n');
    assert.strictEqual(codedFormat.errorSummary('❌ Sözdizimi Hataları:\n  • Satır 2, Sütun 1: eksik\n     2 | x\n'), 'Satır 2, Sütun 1: eksik');
    assert.strictEqual(globEscape('a[1]{b}*?.tr'), 'a?1??b???.tr');
}, false);

test('kodlu dosya açılınca ham sekme Türkçe görünümle değişir; başlık görünmez', async () => {
    const o = makeEnv();
    const editor = await open(o, o.uri);
    assert.strictEqual(editor.document.getText(), TURKISH_TEXT, 'görünüm dosyanın Türkçe yazımı olmalı');
    assert.ok(!editor.document.getText().includes(codedFormat.HEADER), 'başlık görünmemeli');
    assert.strictEqual(editor.document.languageId, 'tr');
    assert.strictEqual(editor.document.uri.fsPath, o.file, 'görünümün adresi dosyanın kendi yolu');
    assert.deepStrictEqual(tabUris(o), [o.vscode.Uri.file(o.file).with({ scheme: 'tr-kodlu' }).toString()], 'ham sekme kapanmalı');
    assert.strictEqual(o.log.warnings.length + o.log.errors.length, 0, [...o.log.warnings, ...o.log.errors].join('\n'));
});

test('düzenle ve kaydet: diskte kodlu; tr göster düzenleyicideki metnin aynısı; geri al bozmaz', async () => {
    const o = makeEnv();
    const b = (await open(o, o.uri)).document;
    const firstDisk = fs.readFileSync(o.file);
    b.edit(b.getText().replace('yazdır("sayaç: {sayaç}")', 'yazdır("toplam: {sayaç}", tip(sayaç))'));
    assert.ok(await b.save());
    assert.ok(!b.isDirty);
    const disk = fs.readFileSync(o.file, 'utf8');
    assert.ok(disk.startsWith(codedFormat.HEADER + 'tr\r\n'), 'diskte kodlu (satır sonu korunur): ' + disk);
    assert.ok(disk.includes('TR_fn_print("toplam: {sayaç}", TR_fn_type(sayaç))'), disk);
    assert.strictEqual(showText(o.file), b.getText());
    // Kayıt belgenin metnine dokunmaz: geri al geçmişi aynen durur, geri alınıp kaydedilince
    // disk ilk hâline döner.
    assert.strictEqual(b.history.length, 1);
    b.undo();
    assert.ok(await b.save());
    assert.deepStrictEqual(fs.readFileSync(o.file), firstDisk);
    assert.deepStrictEqual(o.state.get(PENDING_KEY, []), []);
});

test('seçim (hata bağlantısının satırı ve sütunu) görünüme olduğu gibi taşınır', async () => {
    const o = makeEnv();
    const selection = { start: { line: 4, character: 7 }, end: { line: 4, character: 7 } };
    const editor = await open(o, o.uri, { selection: selection });
    assert.deepStrictEqual(editor.selection, selection);
});

test('satır eşlemesi: tr kontrol\'ün hata konumu görünümde aynı satır ve sütun', async () => {
    const o = makeEnv({ disk: encode(TURKISH_TEXT.replace('yazdır("sayaç: {sayaç}")', 'yazdır(sayacc)')) });
    const s = spawnSync(TR, ['kontrol', o.file], { windowsHide: true });
    const m = /Satır (\d+), Sütun (\d+): Tanımsız ad/.exec(s.stderr.toString('utf8'));
    assert.ok(m, s.stderr.toString('utf8'));
    const b = (await open(o, o.uri)).document;
    const line = b.lineAt(Number(m[1]) - 1).text;
    assert.strictEqual(line.slice(Number(m[2]) - 1, Number(m[2]) - 1 + 'sayacc'.length), 'sayacc', line);
});

test('bozuk metin kayıpsız kaydedilir: başlıksız Türkçe, bir kez bilgi, ilk geçerli kayıtta yeniden kodlu', async () => {
    const state = new FakeState();
    const o = makeEnv({ state });
    const b = (await open(o, o.uri)).document;
    const partial = b.getText() + 'eğer sayaç > 2 {\r\n    yazdır("ç';
    b.edit(partial);
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), partial, 'yazılan her şey diskte olmalı (başlıksız Türkçe)');
    assert.strictEqual(o.log.infos.length, 1);
    assert.ok(o.log.infos[0].includes('sözdizimi hatası düzelince') && o.log.infos[0].includes('kaybolmadı'), o.log.infos[0]);
    b.edit(partial + 'o');
    assert.ok(await b.save());
    assert.strictEqual(o.log.infos.length, 1, 'bilgi bir kez verilir');
    assert.strictEqual(o.state.get(PENDING_KEY, []).length, 1, 'dosya kodlanmayı bekler');

    // VS Code yeniden açıldı: bekleyen dosya yine görünümle açılır, ilk geçerli kayıtta kodlanır.
    const o2 = setup({ setting: TR, folder: o.folder, state });
    const b2 = (await open(o2, o.uri)).document;
    assert.strictEqual(b2.getText(), partial + 'o');
    b2.edit(b2.getText() + 'k")\r\n}\r\n');
    assert.ok(await b2.save());
    const disk = fs.readFileSync(o.file, 'utf8');
    assert.ok(disk.startsWith(codedFormat.HEADER + 'tr\r\n') && disk.includes('TR_kw_if sayaç > 2'), disk);
    assert.strictEqual(showText(o.file), b2.getText());
    assert.deepStrictEqual(state.get(PENDING_KEY, []), []);
    assert.strictEqual(o2.log.infos.length, 0);
});

test('çeviri hatası (sözdizimi dışı) da kayıpsız: Türkçe mesaj, dosya kodlanmayı bekler', async () => {
    const o = makeEnv();
    const b = (await open(o, o.uri)).document;
    const original = codedFormat.translate;
    codedFormat.translate = async () => ({ code: 1, output: Buffer.alloc(0), error: '✗ çeviri kayıpsız olmadı (programın anlamı ya da metni değişecekti), dosyaya dokunulmadı\n' });
    try {
        b.edit(b.getText() + 'yazdır(1)\r\n');
        assert.ok(await b.save());
    } finally {
        codedFormat.translate = original;
    }
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), b.getText());
    assert.strictEqual(o.log.errors.length, 1);
    assert.ok(o.log.errors[0].includes('çeviri kayıpsız olmadı') && o.log.errors[0].includes('kaybolmadı'), o.log.errors[0]);
    assert.strictEqual(o.state.get(PENDING_KEY, []).length, 1);
});

test('kodlu olmayan dosya olduğu gibi açılır ve kaydedilir; yorumlayıcı hiç çalışmaz', async () => {
    const o = makeEnv({ disk: TURKISH_TEXT });
    let callCount = 0;
    const original = codedFormat.translate;
    codedFormat.translate = async (...a) => { callCount++; return original(...a); };
    try {
        await o.vscode.window.showTextDocument(o.uri, { preview: true });
        await new Promise((resolve) => setTimeout(resolve, 200));
        assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
        const b = o.vscode.window.activeTextEditor.document;
        b.edit(b.getText() + '// ek\r\n');
        assert.ok(await b.save());
    } finally {
        codedFormat.translate = original;
    }
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), TURKISH_TEXT + '// ek\r\n', 'eklenti kendiliğinden kodlamaz');
    assert.strictEqual(callCount, 0);
});

test('ayrıştırılamayan kodlu dosya (ör. birleştirme çakışması) ham hâliyle gösterilir, olduğu gibi kaydedilir', async () => {
    const raw = codedFormat.HEADER + 'tr\n<<<<<<< HEAD\nTR_kw_var x = 1\n=======\nTR_kw_var x = 2\n>>>>>>> dal\n';
    const o = makeEnv({ disk: raw });
    const b = (await open(o, o.uri)).document;
    assert.strictEqual(b.getText(), raw);
    assert.strictEqual(o.log.warnings.length, 1);
    assert.ok(o.log.warnings[0].includes('ham (kodlu) hâliyle'), o.log.warnings[0]);
    const resolved = codedFormat.HEADER + 'tr\nTR_kw_var x = 2\n';
    b.edit(resolved);
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), resolved);
    assert.strictEqual(showText(o.file), 'değişken x = 2\n');
});

test('yazım dili ayarı tr göster ve tr kodla komutlarına --dil ile geçer', async () => {
    const o = makeEnv({ settings: { yazimDili: 'tr' } });
    const calls = [];
    const original = codedFormat.translate;
    codedFormat.translate = async (...a) => { calls.push(a.slice(1, 3)); return original(...a); };
    try {
        const b = (await open(o, o.uri)).document;
        b.edit(b.getText() + '\r\n');
        await b.save();
    } finally {
        codedFormat.translate = original;
    }
    assert.deepStrictEqual(calls, [['göster', 'tr'], ['kodla', 'tr']]);
});

// Kural (17.1): dosyanın dili kayıtta korunur; dosyayı başka bir yazım dilinde düzenleyen kişi
// onun değerlerinin metnini (doğru/true) değiştirmez. Neden: çıktı dosyayı düzenleyene bağlı olmamalı.
test('başka dilde düzenlenen kodlu dosya kaydedilince dil=\'si korunur', async () => {
    const english = tr('kodla', '-', '--dil', 'en', 'var x = true\nprint(x)\n');
    assert.strictEqual(english.code, 0, english.error);
    assert.ok(english.output.startsWith(codedFormat.HEADER + 'en\n'), english.output);
    const o = makeEnv({ disk: english.output, settings: { yazimDili: 'tr' } });
    const b = (await open(o, o.uri)).document;
    assert.strictEqual(b.getText(), 'değişken x = doğru\nyazdır(x)\n');
    b.edit(b.getText() + 'yazdır(yanlış)\n');
    assert.ok(await b.save());
    const disk = fs.readFileSync(o.file, 'utf8');
    assert.strictEqual(disk, codedFormat.HEADER + 'en\nTR_kw_var x = TR_kw_true\nTR_fn_print(x)\nTR_fn_print(TR_kw_false)\n');
    // Yeni (Türkçe yazılmış) kodlu dosya Türkçe kodlanır.
    assert.ok(encode(TURKISH_TEXT).startsWith(codedFormat.HEADER + 'tr\r\n'));
});

// Kural (17.1): kodlanamadığı için başlıksız kaydedilen (bekleyen) dosyanın dili de kalıcıdır;
// VS Code yeniden açılıp dosya kodlanınca dil=en korunur. Neden: dil yalnız bellekteydi, yeniden
// açılışta görünümün diliyle kodlanıyordu (dosyanın dili ve değer metinleri sessizce değişiyordu).
test('bekleyen kodlu dosyanın dili VS Code yeniden açılınca da korunur', async () => {
    const state = new FakeState();
    const english = tr('kodla', '-', '--dil', 'en', 'var x = true\nprint(x)\n');
    const o = makeEnv({ disk: english.output, settings: { yazimDili: 'tr' }, state });
    const b = (await open(o, o.uri)).document;
    b.edit(b.getText() + 'eğer x {\n');
    assert.ok(await b.save());
    assert.ok(!codedFormat.isCoded(fs.readFileSync(o.file)), 'bozuk metin başlıksız (bekleyen) kaydedilir');

    const o2 = setup({ setting: TR, folder: o.folder, state, settings: { yazimDili: 'tr' } });
    const b2 = (await open(o2, o.uri)).document;
    b2.edit(b2.getText() + '}\n');
    assert.ok(await b2.save());
    assert.ok(fs.readFileSync(o.file, 'utf8').startsWith(codedFormat.HEADER + 'en\n'), fs.readFileSync(o.file, 'utf8'));
});

test('yazım dili değişince kirli olmayan açık görünüm yeni dille yenilenir', async () => {
    const settings = { yazimDili: 'tr' };
    const o = makeEnv({ settings });
    const b = (await open(o, o.uri)).document;
    assert.ok(b.getText().includes('değişken sayaç'), b.getText());
    settings.yazimDili = 'en';
    o.events.setting.fire({ affectsConfiguration: (k) => k === 'tr.yazimDili' });
    await waitUntil(() => b.getText().includes('var sayaç'), 'İngilizce görünüm');
    assert.ok(!b.isDirty);
});

test('yazım dili değişince kirli görünüm eski dille kodlanır (kodlar bozulmaz), kişi uyarılır', async () => {
    const settings = { yazimDili: 'tr' };
    const o = makeEnv({ settings });
    const b = (await open(o, o.uri)).document;
    const edited = b.getText() + 'yazdır("son")\r\n';
    b.edit(edited);
    settings.yazimDili = 'en';
    o.events.setting.fire({ affectsConfiguration: (k) => k === 'tr.yazimDili' });
    await waitUntil(() => o.log.warnings.length > 0, 'kirli belge uyarısı');
    assert.strictEqual(b.getText(), edited, 'kirli belgenin metnine dokunulmaz');
    await b.save();
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), encode(edited), 'eski dille kodlanmalı');
    assert.strictEqual(showText(o.file), edited);
});

test('dosya dışarıda değişince görünümün adresiyle bildirilir; tarih diskteki dosyanınki', async () => {
    const o = makeEnv();
    const view = o.filesystems['tr-kodlu'].p;
    const address = o.uri.with({ scheme: 'tr-kodlu' });
    const events = [];
    view.onDidChangeFile((e) => events.push(...e));
    const watch = view.watch(address, { recursive: false, excludes: [] });
    const watcher = o.log.watchers[o.log.watchers.length - 1];
    assert.strictEqual(watcher.pattern.base.fsPath, o.folder);
    assert.strictEqual(watcher.pattern.pattern, 'program.tr');
    const before = await view.stat(address);
    fs.writeFileSync(o.file, encode('yazdır(1)\n'));
    fs.utimesSync(o.file, new Date(), new Date(Date.now() + 5000));
    watcher.changed.fire(o.uri);
    assert.deepStrictEqual(events.map((e) => [e.type, e.uri.toString()]), [[1, address.toString()]]);
    const after = await view.stat(address);
    assert.notStrictEqual(after.mtime, before.mtime);
    assert.strictEqual(after.size, fs.statSync(o.file).size);
    assert.strictEqual(Buffer.from(await view.readFile(address)).toString('utf8'), 'yazdır(1)\n');
    watch.dispose();
    assert.ok(watcher.disposed);
    await assert.rejects(view.stat(Uri.file(path.join(o.folder, 'yok.tr')).with({ scheme: 'tr-kodlu' })), (e) => e.code === 'FileNotFound');
});

test('hata ayıklama: görünümde F5 → program diskteki dosya, görünümün adresi tr dap\'a gider', async () => {
    const o = makeEnv();
    const editor = await open(o, o.uri);
    const address = editor.document.uri.toString();
    const { provider, adapterFactory } = o;
    // launch.json olmadan
    const y = await provider.resolveDebugConfiguration(undefined, {});
    assert.strictEqual(y.program, o.file);
    assert.strictEqual(y.cwd, o.folder);
    assert.strictEqual(y.viewURI, address);
    // launch.json'daki ${file}: VS Code görünümde çözemez, eklenti dosyanın kendisinden çözer.
    const lj = await provider.resolveDebugConfiguration(undefined, { type: 'tr', request: 'launch', name: 'TR', program: '${file}', cwd: '${fileDirname}', args: ['${fileBasename}'] });
    assert.deepStrictEqual([lj.program, lj.cwd, lj.args, lj.viewURI], [o.file, o.folder, ['program.tr'], address]);
    // Programı yoluyla veren ayar: kodlu dosyaysa görünümün adresi eklenir.
    const filePath = await provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, { type: 'tr', request: 'launch', name: 'TR', program: o.file });
    assert.strictEqual(filePath.viewURI, address);
    const d = adapterFactory.createDebugAdapterDescriptor({ configuration: y });
    assert.deepStrictEqual([d.command, d.args], [TR, ['dap']]);
    // Başlık çubuğundaki "Hata Ayıkla" düğmesi
    await o.commands['tr.hataAyiklaBaslat']();
    assert.strictEqual(o.log.debugSessions[0].viewURI, address);
    assert.strictEqual(o.log.debugSessions[0].program, o.file);
});

test('görünümde çalıştır: önce kodlu kaydedilir, görev diskteki dosyayla başlar', async () => {
    const o = makeEnv();
    const b = (await open(o, o.uri)).document;
    b.edit(b.getText() + 'yazdır("son")\r\n');
    await o.commands['tr.calistir']();
    await waitUntil(() => o.log.tasks.length === 1, 'çalıştırma görevi');
    assert.deepStrictEqual(o.log.tasks[0].execution.args, [o.file]);
    assert.ok(!b.isDirty);
    assert.ok(fs.readFileSync(o.file, 'utf8').includes('TR_fn_print("son")'));
});

test('kısıtlı kip: hiçbir şey çalıştırılmaz, salt okunur ham gösterim ve açıklama; güven verilince Türkçe', async () => {
    const o = makeEnv({ trusted: false });
    let callCount = 0;
    const original = codedFormat.translate;
    codedFormat.translate = async (...a) => { callCount++; return original(...a); };
    try {
        const b = (await open(o, o.uri, {}, 'tr-ham')).document;
        assert.ok(b.getText().startsWith(codedFormat.HEADER), 'ham hâl');
        assert.strictEqual(await b.save(), false, 'salt okunur');
        await assert.rejects(o.filesystems['tr-ham'].p.writeFile(b.uri, Buffer.from('x'), { create: true, overwrite: true }), (e) => e.code === 'NoPermissions');
        assert.ok(o.filesystems['tr-ham'].options.isReadonly);
        assert.strictEqual(o.log.infos.length, 1);
        assert.ok(o.log.infos[0].includes('Kısıtlı kipte') && o.log.infos[0].includes('salt okunur'), o.log.infos[0]);
        assert.strictEqual(callCount, 0, 'kısıtlı kipte yorumlayıcı çalıştırılmaz');
        // Kısıtlı kipte görünüm adresi de (ör. geri yüklenen sekme) çeviri yapmaz.
        const view = o.filesystems['tr-kodlu'].p;
        assert.ok(codedFormat.isCoded(await view.readFile(o.uri.with({ scheme: 'tr-kodlu' }))));
        assert.strictEqual(callCount, 0);
    } finally {
        codedFormat.translate = original;
    }
    o.vscode.workspace.isTrusted = true;
    o.events.trust.fire();
    await waitUntil(() => o.vscode.window.activeTextEditor && o.vscode.window.activeTextEditor.document.uri.scheme === 'tr-kodlu', 'güvenden sonra görünüm');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.getText(), TURKISH_TEXT);
});

test('kirli ham belge ve yorumlayıcısız pencere yönlendirilmez (hiçbir şey kaybolmaz)', async () => {
    const o = makeEnv({ setting: path.join(root, 'olmayan', 'tr.exe') });
    await o.vscode.window.showTextDocument(o.uri, { preview: true });
    await waitUntil(() => o.log.errors.length === 1, 'yorumlayıcı bulunamadı mesajı');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
    assert.ok(o.log.errors[0].includes('kodlu biçimde saklanıyor'), o.log.errors[0]);

    // Kaydedilmemiş değişikliği olan ham belge (ör. önceki oturumdan geri yüklenen) yerinde kalır.
    const o2 = makeEnv();
    const raw = await o2.vscode.workspace.openTextDocument(o2.uri);
    raw.edit(raw.getText() + '// kaydedilmedi\n');
    await o2.vscode.window.showTextDocument(o2.uri, { preview: true });
    await new Promise((resolve) => setTimeout(resolve, 300));
    assert.strictEqual(o2.vscode.window.activeTextEditor.document, raw);
    assert.deepStrictEqual(tabUris(o2), [o2.uri.toString()]);
});

test('Kodlu Hâlini Göster komutu ham hâli yanda salt okunur açar', async () => {
    const o = makeEnv();
    await open(o, o.uri);
    await o.commands['tr.kodluHaliniGoster']();
    const e = o.vscode.window.activeTextEditor;
    assert.strictEqual(e.document.uri.scheme, 'tr-ham');
    assert.strictEqual(e.viewColumn, 2);
    assert.deepStrictEqual(Buffer.from(e.document.getText(), 'utf8'), fs.readFileSync(o.file));
});

// Kodsuz (Türkçe) dosya başka bir yazım dilinde görünümle açılır, diske yine Türkçe yazılır (17.7).
function inEnglish(text) {
    const s = tr('göster', '-', '--dil', 'en', text);
    assert.strictEqual(s.code, 0, s.error);
    return s.output;
}

test('yazım dili en: kodsuz Türkçe dosya İngilizce görünür, kaydedilince diske Türkçe yazılır', async () => {
    const o = makeEnv({ disk: TURKISH_TEXT, settings: { yazimDili: 'en' } });
    const b = (await open(o, o.uri)).document;
    const english = inEnglish(TURKISH_TEXT);
    assert.ok(english.includes('var sayaç') && english.includes('print('), english);
    assert.strictEqual(b.getText(), english);
    b.edit(b.getText() + 'print(sayaç * 2)\r\n');
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), TURKISH_TEXT + 'yazdır(sayaç * 2)\r\n', 'diskte kodsuz Türkçe');
    assert.strictEqual(o.log.warnings.length + o.log.errors.length + o.log.infos.length, 0, [...o.log.warnings, ...o.log.errors, ...o.log.infos].join('\n'));
    // F5: program diskteki dosya, satırlar görünümünkiler.
    const y = await o.provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, { type: 'tr', request: 'launch', name: 'TR', program: o.file });
    assert.strictEqual(y.viewURI, b.uri.toString());
});

test('yazım dili en: ayrıştırılamayan metin kayıpsız kaydedilir, ilk geçerli kayıtta yeniden Türkçe', async () => {
    const state = new FakeState();
    const o = makeEnv({ disk: TURKISH_TEXT, settings: { yazimDili: 'en' }, state });
    const b = (await open(o, o.uri)).document;
    const partial = b.getText() + 'if sayaç > 2 {\r\n';
    b.edit(partial);
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), partial, 'yazılan her şey diskte');
    assert.strictEqual(o.log.infos.length, 1);
    assert.ok(o.log.infos[0].includes('yeniden Türkçe yazılacak'), o.log.infos[0]);
    assert.strictEqual(state.get(PLAIN_PENDING_KEY, []).length, 1);
    // VS Code yeniden açıldı: bekleyen dosya olduğu gibi (İngilizce) görünür.
    const o2 = setup({ setting: TR, folder: o.folder, state, settings: { yazimDili: 'en' } });
    const b2 = (await open(o2, o.uri)).document;
    assert.strictEqual(b2.getText(), partial);
    b2.edit(partial + '    print(1)\r\n}\r\n');
    assert.ok(await b2.save());
    assert.strictEqual(fs.readFileSync(o.file, 'utf8'), TURKISH_TEXT + 'eğer sayaç > 2 {\r\n    yazdır(1)\r\n}\r\n');
    assert.deepStrictEqual(state.get(PLAIN_PENDING_KEY, []), []);
});

test('yazım dili tr\'den en\'e geçince açık kodsuz dosya görünümle açılır', async () => {
    const settings = { yazimDili: 'tr' };
    const o = makeEnv({ disk: TURKISH_TEXT, settings });
    await o.vscode.window.showTextDocument(o.uri, { preview: true });
    await new Promise((resolve) => setTimeout(resolve, 200));
    assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
    settings.yazimDili = 'en';
    o.events.setting.fire({ affectsConfiguration: (k) => k === 'tr.yazimDili' });
    await waitUntil(() => o.vscode.window.activeTextEditor.document.uri.scheme === 'tr-kodlu', 'İngilizce görünüm');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.getText(), inEnglish(TURKISH_TEXT));
});

test('yazım dili en, kısıtlı kip: kodsuz dosya Türkçe ve düzenlenebilir kalır', async () => {
    const o = makeEnv({ disk: TURKISH_TEXT, settings: { yazimDili: 'en' }, trusted: false });
    await o.vscode.window.showTextDocument(o.uri, { preview: true });
    await new Promise((resolve) => setTimeout(resolve, 200));
    assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
    assert.strictEqual(o.log.infos.length + o.log.warnings.length, 0);
});

// Kural: güvenilmeyen çalışma alanında tr-kodlu: şeması da salt okunurdur; adresi elle verilse
// bile yazma, silme, taşıma ve klasör oluşturma yerel diske dokunmaz (SECURITY.md: kısıtlı
// kipte yerel diske yazılmaz). Neden: görünüm sağlayıcısı kısıtlı kipte yönlendirmeye güveniyordu,
// kendisi güven denetimi yapmıyordu.
test('kısıtlı kipte tr-kodlu: yazma, silme, taşıma ve klasör oluşturma reddedilir', async () => {
    const dir = makeFolder();
    const filePath = path.join(dir, 'program.tr');
    const original = '//! tr kodlu 3 dil=tr\nTR_kw_var x = 1\n';
    fs.writeFileSync(filePath, original, 'utf8');
    const o = setup({ folder: dir, trusted: false });
    const codedUri = Uri.file(filePath).with({ scheme: 'tr-kodlu' });
    const provider = o.filesystems['tr-kodlu'].p;
    const attempts = {
        writeFile: () => provider.writeFile(codedUri, Buffer.from('ÜZERİNE_YAZILDI\n', 'utf8'), { create: true, overwrite: true }),
        delete: () => provider.delete(codedUri, { recursive: false }),
        rename: () => provider.rename(codedUri, Uri.file(path.join(dir, 'baska.tr')).with({ scheme: 'tr-kodlu' }), { overwrite: true }),
        createDirectory: () => provider.createDirectory(Uri.file(path.join(dir, 'yeni')).with({ scheme: 'tr-kodlu' })),
    };
    for (const [name, attempt] of Object.entries(attempts)) {
        await assert.rejects(async () => attempt(), undefined, `kısıtlı kipte ${name} reddedilmeli`);
    }
    assert.strictEqual(fs.readFileSync(filePath, 'utf8'), original);
    assert.deepStrictEqual(fs.readdirSync(dir), ['program.tr']);
}, false);

suite.run();
