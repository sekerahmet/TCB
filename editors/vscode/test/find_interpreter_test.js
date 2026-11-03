// Yorumlayıcıyı (trlang.exe) bulma kuralının testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/find_interpreter_test.js
// (go test ./cmd/trlang/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const { findInterpreter } = require('../find_interpreter');
const { createSuite } = require('./fake_vscode');

const suite = createSuite('find_interpreter_test', {
    summary: ({ passed }) => `find_interpreter_test: ${passed} test geçti`,
});
const test = suite.test;

// Sahte dosya sistemi: yalnızca listedeki yollar vardır.
function g(files, extra) {
    const set = new Set(files);
    return Object.assign({ platform: 'win32', pathValue: '', trusted: true, fileExists: (y) => set.has(y) }, extra);
}

// İndirilmiş bir klasör: içinde .tr dosyası ve yanında yabancı bir trlang.exe; üst klasörde de bir trlang.exe.
const DOWNLOADED = 'C:\\Users\\a\\Downloads\\paket';
const FOREIGN = [DOWNLOADED + '\\trlang.exe', DOWNLOADED + '\\bin\\trlang.exe', 'C:\\Users\\a\\Downloads\\trlang.exe', 'C:\\Users\\a\\trlang.exe'];

test('klasörsüz pencerede dosyanın yanındaki ve üst klasörlerdeki trlang.exe çalıştırılmaz', () => {
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: undefined })), undefined);
});

test('klasörsüz pencerede PATH kullanılır, yandaki trlang.exe değil', () => {
    const r = findInterpreter(g([...FOREIGN, 'C:\\araclar\\trlang.exe'], { folder: undefined, pathValue: 'C:\\yok;C:\\araclar' }));
    assert.deepStrictEqual(r, { filePath: 'C:\\araclar\\trlang.exe' });
});

test('güvenilmeyen çalışma alanının kökündeki trlang.exe kullanılmaz', () => {
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: DOWNLOADED, trusted: false })), undefined);
});

test('güvenilen çalışma alanında önce kökteki bin/trlang.exe, sonra kökteki trlang.exe', () => {
    const root = 'D:\\proje';
    assert.deepStrictEqual(findInterpreter(g([root + '\\bin\\trlang.exe', root + '\\trlang.exe'], { folder: root })), { filePath: root + '\\bin\\trlang.exe' });
    assert.deepStrictEqual(findInterpreter(g([root + '\\trlang.exe'], { folder: root })), { filePath: root + '\\trlang.exe' });
});

test('güvenilen çalışma alanında da üst klasörler taranmaz', () => {
    // Çalışma alanı C:\Users\a\Downloads\paket\alt; trlang.exe yalnızca üst klasörlerde.
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: DOWNLOADED + '\\alt' })), undefined);
});

test('ayar her şeyden önce gelir; mutlak yol', () => {
    const r = findInterpreter(g([...FOREIGN, 'C:\\tr\\trlang.exe'], { folder: DOWNLOADED, setting: '  C:\\tr\\trlang.exe ' }));
    assert.deepStrictEqual(r, { filePath: 'C:\\tr\\trlang.exe' });
});

test('ayardaki yol yoksa hata; başka yere bakılmaz', () => {
    const r = findInterpreter(g(FOREIGN, { folder: DOWNLOADED, setting: 'C:\\yok\\trlang.exe' }));
    assert.ok(r.error && r.error.includes('C:\\yok\\trlang.exe'), JSON.stringify(r));
});

test('göreli ayar yalnızca güvenilen çalışma alanı klasörüne göre çözülür', () => {
    const root = 'D:\\proje';
    assert.deepStrictEqual(findInterpreter(g([root + '\\bin\\trlang.exe'], { folder: root, setting: 'bin\\trlang.exe' })), { filePath: root + '\\bin\\trlang.exe' });
    const noFolder = findInterpreter(g(FOREIGN, { folder: undefined, setting: 'trlang.exe' }));
    assert.ok(noFolder.error && noFolder.error.includes('relative'), JSON.stringify(noFolder));
    const untrusted = findInterpreter(g(FOREIGN, { folder: DOWNLOADED, trusted: false, setting: 'trlang.exe' }));
    assert.ok(untrusted.error && untrusted.error.includes('relative'), JSON.stringify(untrusted));
});

// Kural: PATH'teki göreli girdiler ('.', 'bin') atlanır, yalnızca mutlak yollar aranır (SECURITY.md:
// güvenilmeyen klasörde yalnızca PATH ve ayar kullanılır). Neden: göreli girdi çalışma klasörüne
// göre çözülür ve çalıştırma, açılan dosyanın (indirilmiş) klasöründe olur.
test('PATH içindeki göreli girdiler atlanır, yalnızca mutlak yol bulunur', () => {
    const r = findInterpreter(g(['trlang.exe', 'bin\\trlang.exe', 'C:\\araclar\\trlang.exe'], { folder: undefined, trusted: false, pathValue: '.;bin;C:\\araclar' }));
    assert.deepStrictEqual(r, { filePath: 'C:\\araclar\\trlang.exe' });
    assert.strictEqual(findInterpreter(g(['trlang.exe', 'bin\\trlang.exe'], { folder: undefined, trusted: false, pathValue: '.;bin' })), undefined);
});

suite.run();
