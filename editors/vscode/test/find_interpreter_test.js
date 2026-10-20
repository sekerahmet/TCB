// Yorumlayıcıyı (tr.exe) bulma kuralının testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/find_interpreter_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const { findInterpreter } = require('../find_interpreter');

let passed = 0;
function test(name, fn) {
    try {
        fn();
        passed++;
    } catch (e) {
        console.error('KALDI: ' + name + '\n' + (e && e.stack));
        process.exitCode = 1;
    }
}

// Sahte dosya sistemi: yalnızca listedeki yollar vardır.
function g(files, extra) {
    const set = new Set(files);
    return Object.assign({ platform: 'win32', pathValue: '', trusted: true, fileExists: (y) => set.has(y) }, extra);
}

// İndirilmiş bir klasör: içinde .tr dosyası ve yanında yabancı bir tr.exe; üst klasörde de bir tr.exe.
const DOWNLOADED = 'C:\\Users\\a\\Downloads\\paket';
const FOREIGN = [DOWNLOADED + '\\tr.exe', DOWNLOADED + '\\bin\\tr.exe', 'C:\\Users\\a\\Downloads\\tr.exe', 'C:\\Users\\a\\tr.exe'];

test('klasörsüz pencerede dosyanın yanındaki ve üst klasörlerdeki tr.exe çalıştırılmaz', () => {
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: undefined })), undefined);
});

test('klasörsüz pencerede PATH kullanılır, yandaki tr.exe değil', () => {
    const r = findInterpreter(g([...FOREIGN, 'C:\\araclar\\tr.exe'], { folder: undefined, pathValue: 'C:\\yok;C:\\araclar' }));
    assert.deepStrictEqual(r, { filePath: 'C:\\araclar\\tr.exe' });
});

test('güvenilmeyen çalışma alanının kökündeki tr.exe kullanılmaz', () => {
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: DOWNLOADED, trusted: false })), undefined);
});

test('güvenilen çalışma alanında önce kökteki bin/tr.exe, sonra kökteki tr.exe', () => {
    const root = 'D:\\proje';
    assert.deepStrictEqual(findInterpreter(g([root + '\\bin\\tr.exe', root + '\\tr.exe'], { folder: root })), { filePath: root + '\\bin\\tr.exe' });
    assert.deepStrictEqual(findInterpreter(g([root + '\\tr.exe'], { folder: root })), { filePath: root + '\\tr.exe' });
});

test('güvenilen çalışma alanında da üst klasörler taranmaz', () => {
    // Çalışma alanı C:\Users\a\Downloads\paket\alt; tr.exe yalnızca üst klasörlerde.
    assert.strictEqual(findInterpreter(g(FOREIGN, { folder: DOWNLOADED + '\\alt' })), undefined);
});

test('ayar her şeyden önce gelir; mutlak yol', () => {
    const r = findInterpreter(g([...FOREIGN, 'C:\\tr\\tr.exe'], { folder: DOWNLOADED, setting: '  C:\\tr\\tr.exe ' }));
    assert.deepStrictEqual(r, { filePath: 'C:\\tr\\tr.exe' });
});

test('ayardaki yol yoksa hata; başka yere bakılmaz', () => {
    const r = findInterpreter(g(FOREIGN, { folder: DOWNLOADED, setting: 'C:\\yok\\tr.exe' }));
    assert.ok(r.error && r.error.includes('C:\\yok\\tr.exe'), JSON.stringify(r));
});

test('göreli ayar yalnızca güvenilen çalışma alanı klasörüne göre çözülür', () => {
    const root = 'D:\\proje';
    assert.deepStrictEqual(findInterpreter(g([root + '\\bin\\tr.exe'], { folder: root, setting: 'bin\\tr.exe' })), { filePath: root + '\\bin\\tr.exe' });
    const noFolder = findInterpreter(g(FOREIGN, { folder: undefined, setting: 'tr.exe' }));
    assert.ok(noFolder.error && noFolder.error.includes('göreli'), JSON.stringify(noFolder));
    const untrusted = findInterpreter(g(FOREIGN, { folder: DOWNLOADED, trusted: false, setting: 'tr.exe' }));
    assert.ok(untrusted.error && untrusted.error.includes('göreli'), JSON.stringify(untrusted));
});

test("Unix'te /bin ve /usr/bin'deki 'tr' (karakter çevirme aracı) atlanır", () => {
    const r = findInterpreter(g(['/usr/bin/tr', '/bin/tr', '/home/a/go/bin/tr'], { platform: 'linux', folder: undefined, pathValue: '/usr/bin:/bin:/home/a/go/bin' }));
    assert.deepStrictEqual(r, { filePath: '/home/a/go/bin/tr' });
});

console.log(`find_interpreter_test: ${passed} test geçti`);
