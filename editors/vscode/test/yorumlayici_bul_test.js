// Yorumlayıcıyı (tr.exe) bulma kuralının testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/yorumlayici_bul_test.js
// (go test ./cmd/tr/uctan_uca bunu TestEklentiJSTestleri ile çalıştırır.)
'use strict';

const assert = require('assert');
const { yorumlayiciyiBul } = require('../yorumlayici_bul');

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

// Sahte dosya sistemi: yalnızca listedeki yollar vardır.
function g(dosyalar, ek) {
    const kume = new Set(dosyalar);
    return Object.assign({ platform: 'win32', pathDegeri: '', guvenilir: true, dosyaVarMi: (y) => kume.has(y) }, ek);
}

// İndirilmiş bir klasör: içinde .tr dosyası ve yanında yabancı bir tr.exe; üst klasörde de bir tr.exe.
const INDIRILEN = 'C:\\Users\\a\\Downloads\\paket';
const YABANCI = [INDIRILEN + '\\tr.exe', INDIRILEN + '\\bin\\tr.exe', 'C:\\Users\\a\\Downloads\\tr.exe', 'C:\\Users\\a\\tr.exe'];

test('klasörsüz pencerede dosyanın yanındaki ve üst klasörlerdeki tr.exe çalıştırılmaz', () => {
    assert.strictEqual(yorumlayiciyiBul(g(YABANCI, { klasor: undefined })), undefined);
});

test('klasörsüz pencerede PATH kullanılır, yandaki tr.exe değil', () => {
    const r = yorumlayiciyiBul(g([...YABANCI, 'C:\\araclar\\tr.exe'], { klasor: undefined, pathDegeri: 'C:\\yok;C:\\araclar' }));
    assert.deepStrictEqual(r, { yol: 'C:\\araclar\\tr.exe' });
});

test('güvenilmeyen çalışma alanının kökündeki tr.exe kullanılmaz', () => {
    assert.strictEqual(yorumlayiciyiBul(g(YABANCI, { klasor: INDIRILEN, guvenilir: false })), undefined);
});

test('güvenilen çalışma alanında önce kökteki bin/tr.exe, sonra kökteki tr.exe', () => {
    const kok = 'D:\\proje';
    assert.deepStrictEqual(yorumlayiciyiBul(g([kok + '\\bin\\tr.exe', kok + '\\tr.exe'], { klasor: kok })), { yol: kok + '\\bin\\tr.exe' });
    assert.deepStrictEqual(yorumlayiciyiBul(g([kok + '\\tr.exe'], { klasor: kok })), { yol: kok + '\\tr.exe' });
});

test('güvenilen çalışma alanında da üst klasörler taranmaz', () => {
    // Çalışma alanı C:\Users\a\Downloads\paket\alt; tr.exe yalnızca üst klasörlerde.
    assert.strictEqual(yorumlayiciyiBul(g(YABANCI, { klasor: INDIRILEN + '\\alt' })), undefined);
});

test('ayar her şeyden önce gelir; mutlak yol', () => {
    const r = yorumlayiciyiBul(g([...YABANCI, 'C:\\tr\\tr.exe'], { klasor: INDIRILEN, ayar: '  C:\\tr\\tr.exe ' }));
    assert.deepStrictEqual(r, { yol: 'C:\\tr\\tr.exe' });
});

test('ayardaki yol yoksa hata; başka yere bakılmaz', () => {
    const r = yorumlayiciyiBul(g(YABANCI, { klasor: INDIRILEN, ayar: 'C:\\yok\\tr.exe' }));
    assert.ok(r.hata && r.hata.includes('C:\\yok\\tr.exe'), JSON.stringify(r));
});

test('göreli ayar yalnızca güvenilen çalışma alanı klasörüne göre çözülür', () => {
    const kok = 'D:\\proje';
    assert.deepStrictEqual(yorumlayiciyiBul(g([kok + '\\bin\\tr.exe'], { klasor: kok, ayar: 'bin\\tr.exe' })), { yol: kok + '\\bin\\tr.exe' });
    const klasorsuz = yorumlayiciyiBul(g(YABANCI, { klasor: undefined, ayar: 'tr.exe' }));
    assert.ok(klasorsuz.hata && klasorsuz.hata.includes('göreli'), JSON.stringify(klasorsuz));
    const guvensiz = yorumlayiciyiBul(g(YABANCI, { klasor: INDIRILEN, guvenilir: false, ayar: 'tr.exe' }));
    assert.ok(guvensiz.hata && guvensiz.hata.includes('göreli'), JSON.stringify(guvensiz));
});

test("Unix'te /bin ve /usr/bin'deki 'tr' (karakter çevirme aracı) atlanır", () => {
    const r = yorumlayiciyiBul(g(['/usr/bin/tr', '/bin/tr', '/home/a/go/bin/tr'], { platform: 'linux', klasor: undefined, pathDegeri: '/usr/bin:/bin:/home/a/go/bin' }));
    assert.deepStrictEqual(r, { yol: '/home/a/go/bin/tr' });
});

console.log(`yorumlayıcı bulma: ${gecen} test geçti`);
