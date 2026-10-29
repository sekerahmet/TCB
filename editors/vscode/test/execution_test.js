// Eklentinin çalıştırma ve hata ayıklama yolları, sahte vscode modülüyle (fake_vscode.js):
// F5 / Ctrl+F5 yapılandırmasından yorumlayıcıya giden süreç ve argümanlar. Bağımlılıksız; depo
// kökünden:
//   node editors/vscode/test/execution_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const path = require('path');
const { setup, wait, createSuite } = require('./fake_vscode');
const { programArguments } = require('../program_arguments');

const suite = createSuite('execution_test');
const test = suite.test;

// Yorumlayıcı olarak var olan bir dosya gerekir (yalnızca yolu kullanılır, çalıştırılmaz).
const INTERPRETER = process.execPath;
const FOLDER = path.resolve(__dirname, '..');
const FILE = path.join(FOLDER, 'deneme.tr');

test('programArguments: yoksa boş, metin dizisiyse kopyası, değilse hata', () => {
    assert.deepStrictEqual(programArguments({}), { args: [] });
    assert.deepStrictEqual(programArguments({ args: null }), { args: [] });
    const args = ['--adım', '5', 'İçerik ı.txt'];
    const s = programArguments({ args });
    assert.deepStrictEqual(s, { args: args });
    assert.notStrictEqual(s.args, args);
    for (const wrong of ['--adım 5', [5], ['a', 1], { a: 1 }]) {
        assert.ok(programArguments({ args: wrong }).error, JSON.stringify(wrong));
    }
});

test('Ctrl+F5 launch.json ile: dosya ve "args" görevle yorumlayıcıya gider', async () => {
    const { log, provider } = setup({ setting: INTERPRETER, folder: FOLDER });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, args: ['--adım', '5'], noDebug: true };
    const resolved = await provider.resolveDebugConfiguration(undefined, y);
    assert.strictEqual(resolved, y);
    assert.strictEqual(provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, resolved), undefined, 'hata ayıklama oturumu başlamamalı');
    await wait();
    assert.strictEqual(log.tasks.length, 1, 'bir görev başlamalı');
    const e = log.tasks[0].execution;
    assert.strictEqual(e.process, INTERPRETER);
    assert.deepStrictEqual(e.args, [FILE, '--adım', '5']);
    assert.strictEqual(e.options.cwd, FOLDER);
});

test('Ctrl+F5 klasörsüz pencerede: argümanlar terminale de gider', async () => {
    const { log, provider } = setup({ setting: INTERPRETER });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, args: ['a b', 'ç'], noDebug: true };
    provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, await provider.resolveDebugConfiguration(undefined, y));
    await wait();
    assert.strictEqual(log.terminals.length, 1);
    // Yorumlayıcı terminalin kabuğu olmaz (bitince terminal kapanırdı); eklentinin terminalinde
    // çalışır (program_terminal_test.js).
    assert.strictEqual(log.terminals[0].shellPath, undefined);
    const p = log.terminals[0].pty;
    assert.ok(p, 'sahte terminal verilmeli');
    assert.strictEqual(p.command, INTERPRETER);
    assert.deepStrictEqual(p.args, [FILE, 'a b', 'ç']);
    assert.strictEqual(p.cwd, FOLDER);
});

test('Ctrl+F5 launch.json olmadan: açık dosya çalışır, hata ayıklama başlamaz', async () => {
    const { log, provider } = setup({ setting: INTERPRETER, folder: FOLDER, activeFile: FILE });
    const resolved = await provider.resolveDebugConfiguration(undefined, { noDebug: true });
    assert.ok(resolved && resolved.noDebug, '"hata ayıklamadan" işareti korunmalı');
    assert.strictEqual(provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, resolved), undefined);
    await wait();
    assert.strictEqual(log.tasks.length, 1);
    assert.deepStrictEqual(log.tasks[0].execution.args, [FILE]);
});

test('F5: yapılandırma olduğu gibi kalır, yorumlayıcı "dap" ile başlar', async () => {
    const { log, provider, adapterFactory } = setup({ setting: INTERPRETER, folder: FOLDER });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, args: ['--adım', '5'], cwd: FOLDER };
    const resolved = await provider.resolveDebugConfiguration(undefined, y);
    // Görünüm adresi eklenmez: hata ayıklanan program diskteki dosyadır (görünüm salt okunurdur, view.js).
    assert.strictEqual(await provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, resolved), y);
    assert.strictEqual(y.viewURI, undefined);
    // "args" başlatma isteğiyle (launch) 'tr dap'a gider; süreç argümanı yalnızca 'dap'.
    const d = adapterFactory.createDebugAdapterDescriptor({ configuration: resolved });
    assert.strictEqual(d.command, INTERPRETER);
    assert.deepStrictEqual(d.args, ['dap']);
    assert.strictEqual(d.options.cwd, FOLDER);
    assert.strictEqual(log.tasks.length + log.terminals.length, 0);
});

test('yazım dili ayarı çalıştırma, terminal ve hata ayıklama süreçlerine TR_LANG olarak geçer', async () => {
    for (const lang of ['tr', 'en']) {
        const settings = { writingLanguage: lang };
        const task = setup({ setting: INTERPRETER, folder: FOLDER, settings });
        const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, noDebug: true };
        task.provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, await task.provider.resolveDebugConfiguration(undefined, y));
        await wait();
        assert.strictEqual(task.log.tasks[0].execution.options.env.TR_LANG, lang, 'görev');

        const terminal = setup({ setting: INTERPRETER, settings });
        terminal.provider.resolveDebugConfigurationWithSubstitutedVariables(undefined, await terminal.provider.resolveDebugConfiguration(undefined, y));
        await wait();
        assert.strictEqual(terminal.log.terminals[0].pty.env.TR_LANG, lang, 'terminal');

        const debug = setup({ setting: INTERPRETER, folder: FOLDER, settings });
        const d = debug.adapterFactory.createDebugAdapterDescriptor({ configuration: { program: FILE, cwd: FOLDER } });
        assert.strictEqual(d.options.env.TR_LANG, lang, 'hata ayıklayıcı');
    }
});

test('güvenilmeyen çalışma alanında "Hata Ayıkla" komutu ve görev sağlayıcısı yorumlayıcıyı çalıştırmaz', async () => {
    const o = setup({ setting: INTERPRETER, folder: FOLDER, trusted: false, activeFile: FILE });
    await o.commands['tr.debugStart']();
    assert.strictEqual(o.log.debugSessions.length, 0);
    assert.strictEqual(o.log.warnings.length, 1);
    const task = o.taskProvider.resolveTask({ definition: { type: 'tr', command: 'tr.run', file: FILE }, name: 'TR' });
    assert.strictEqual(task, undefined);
});

test('güvenilen çalışma alanında görev sağlayıcısı çözer', async () => {
    const o = setup({ setting: INTERPRETER, folder: FOLDER });
    const task = o.taskProvider.resolveTask({ definition: { type: 'tr', command: 'tr.run', file: FILE }, name: 'TR' });
    assert.ok(task && task.execution.process === INTERPRETER);
});

test('"Hata Ayıkla" komutu kaydedilemeyen dosyada oturum başlatmaz', async () => {
    const o = setup({ setting: INTERPRETER, folder: FOLDER, activeFile: FILE });
    const document = o.vscode.window.activeTextEditor.document;
    document.isDirty = true;
    document.save = async () => false;
    await o.commands['tr.debugStart']();
    assert.strictEqual(o.log.debugSessions.length, 0);
    assert.strictEqual(o.log.errors.length, 1);
    document.save = async () => true;
    await o.commands['tr.debugStart']();
    assert.strictEqual(o.log.debugSessions.length, 1);
});

test('yanlış "args" Türkçe bir mesajla durdurulur', async () => {
    const { log, provider } = setup({ setting: INTERPRETER, folder: FOLDER });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, args: '--adım 5' };
    assert.strictEqual(await provider.resolveDebugConfiguration(undefined, y), undefined);
    assert.strictEqual(log.errors.length, 1);
    assert.ok(log.errors[0].includes('"args"'), log.errors[0]);
});

test('güvenilmeyen çalışma alanında Ctrl+F5 yorumlayıcıyı çalıştırmaz', async () => {
    const { log, provider } = setup({ setting: INTERPRETER, folder: FOLDER, trusted: false });
    const y = { type: 'tr', request: 'launch', name: 'TR', program: FILE, noDebug: true };
    assert.strictEqual(await provider.resolveDebugConfiguration(undefined, y), undefined);
    await wait();
    assert.strictEqual(log.tasks.length + log.terminals.length, 0);
    assert.strictEqual(log.warnings.length, 1);
});

suite.run();
