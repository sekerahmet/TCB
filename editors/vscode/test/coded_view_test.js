// Kodlu dosyaların Türkçe görünümü (coded_view.js, docs/spec/17_kodlar.md "Düzenleyicide"), sahte vscode
// modülüyle ve GERÇEK 'tr' yorumlayıcısıyla: kodlu dosyayı aç → Türkçe görünüm (başlık gizli),
// düzenle → kaydet → diskte kodlu ve 'tr göster' ile aynı; geri al; bozuk dosyanın kayıpsız
// kaydı; satır eşlemesi (hata mesajı ve hata ayıklayıcı); kısıtlı kip. Depo kökünden:
//   TR_YORUMLAYICI=bin/tr.exe node editors/vscode/test/coded_view_test.js
// (go test ./cmd/tr/e2e bunu TestEklentiJSTestleri ile, derlediği yorumlayıcıyla çalıştırır.
// Yorumlayıcı yoksa yorumlayıcı gerektiren testler atlanır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { kur, olana, Uri, SahteDurum } = require('./fake_vscode');
const kodluBicim = require('../coded_format');
const { globKacis } = require('../coded_filesystem');
const { BEKLEYEN_ANAHTARI } = require('../coded_view');

const depoKoku = path.resolve(__dirname, '..', '..', '..');
const TR = [process.env.TR_YORUMLAYICI, path.join(depoKoku, 'bin', process.platform === 'win32' ? 'tr.exe' : 'tr')]
    .find((y) => y && fs.existsSync(y));

const testler = [];
function test(ad, fn, trGerekir) {
    testler.push({ ad, fn, trGerekir: trGerekir !== false });
}

const kok = fs.mkdtempSync(path.join(os.tmpdir(), 'tr-eklenti-kodlu-'));
let sayac = 0;
function klasorAc() {
    const k = path.join(kok, String(++sayac));
    fs.mkdirSync(k);
    return k;
}

function tr(...args) {
    const s = spawnSync(TR, args.slice(0, -1), { input: args[args.length - 1], windowsHide: true });
    return { kod: s.status, cikti: s.stdout.toString('utf8'), hata: s.stderr.toString('utf8') };
}
function kodla(metin) {
    const s = tr('kodla', '-', metin);
    assert.strictEqual(s.kod, 0, s.hata);
    return s.cikti;
}
function goster(dosya) {
    const s = spawnSync(TR, ['göster', dosya], { windowsHide: true });
    assert.strictEqual(s.status, 0, s.stderr.toString());
    return s.stdout.toString('utf8');
}

const TURKCE = 'değişken sayaç = 0\r\ndöngü i içinde 1..3 {\r\n    sayaç += i\r\n}\r\nyazdır("sayaç: {sayaç}") // yorum\r\n';

// ortam bir klasörde kodlu bir dosya yazar ve eklentiyi kurar.
function ortam(s) {
    const klasor = klasorAc();
    const dosya = path.join(klasor, 'program.tr');
    fs.writeFileSync(dosya, s && s.disk !== undefined ? s.disk : kodla(TURKCE));
    const o = kur(Object.assign({ ayar: TR, klasor }, s));
    return Object.assign(o, { klasor, dosya, uri: Uri.file(dosya) });
}

// ac dosyayı dosya gezgininden açılmış gibi açar ve eklentinin yönlendirmesini bekler.
async function ac(o, uri, secenek, sema) {
    await o.vscode.window.showTextDocument(uri, Object.assign({ preview: true }, secenek));
    const beklenen = sema || 'tr-kodlu';
    await olana(() => o.vscode.window.activeTextEditor && o.vscode.window.activeTextEditor.document.uri.scheme === beklenen, beklenen + ' görünümü');
    return o.vscode.window.activeTextEditor;
}

function sekmeler(o) {
    return o.vscode.window.tabGroups.all.flatMap((g) => g.tabs.map((t) => t.input.uri.toString()));
}

test('kodluMu: başlık yalnızca ilk satırın tamamıysa (BOM, LF, CRLF, tek satır)', () => {
    const b = (m) => Buffer.from(m, 'utf8');
    for (const evet of ['//! tr kodlu 1\nA030 x = 1\n', '//! tr kodlu 1\r\n', '﻿//! tr kodlu 1\n', '//! tr kodlu 1']) {
        assert.ok(kodluBicim.kodluMu(b(evet)), JSON.stringify(evet));
    }
    for (const hayir of ['//! tr kodlu 2\n', '//! tr kodlu 1 \n', ' //! tr kodlu 1\n', '//! tr kodlu 1\r', 'değişken x = 1\n', '', '\n//! tr kodlu 1\n']) {
        assert.ok(!kodluBicim.kodluMu(b(hayir)), JSON.stringify(hayir));
    }
    assert.ok(kodluBicim.kodluMu(new Uint8Array(b('//! tr kodlu 1\n'))), 'Uint8Array da okunur');
    assert.strictEqual(kodluBicim.hataOzeti('❌ Sözdizimi Hataları:\n  • Satır 2, Sütun 1: eksik\n     2 | x\n'), 'Satır 2, Sütun 1: eksik');
    assert.strictEqual(globKacis('a[1]{b}*?.tr'), 'a?1??b???.tr');
}, false);

test('kodlu dosya açılınca ham sekme Türkçe görünümle değişir; başlık görünmez', async () => {
    const o = ortam();
    const editor = await ac(o, o.uri);
    assert.strictEqual(editor.document.getText(), TURKCE, 'görünüm dosyanın Türkçe yazımı olmalı');
    assert.ok(!editor.document.getText().includes(kodluBicim.BASLIK), 'başlık görünmemeli');
    assert.strictEqual(editor.document.languageId, 'tr');
    assert.strictEqual(editor.document.uri.fsPath, o.dosya, 'görünümün adresi dosyanın kendi yolu');
    assert.deepStrictEqual(sekmeler(o), [o.vscode.Uri.file(o.dosya).with({ scheme: 'tr-kodlu' }).toString()], 'ham sekme kapanmalı');
    assert.strictEqual(o.kayit.uyarilar.length + o.kayit.hatalar.length, 0, [...o.kayit.uyarilar, ...o.kayit.hatalar].join('\n'));
});

test('düzenle ve kaydet: diskte kodlu; tr göster düzenleyicideki metnin aynısı; geri al bozmaz', async () => {
    const o = ortam();
    const b = (await ac(o, o.uri)).document;
    const ilkDisk = fs.readFileSync(o.dosya);
    b.duzenle(b.getText().replace('yazdır("sayaç: {sayaç}")', 'yazdır("toplam: {sayaç}", tip(sayaç))'));
    assert.ok(await b.save());
    assert.ok(!b.isDirty);
    const disk = fs.readFileSync(o.dosya, 'utf8');
    assert.ok(disk.startsWith(kodluBicim.BASLIK + '\r\n'), 'diskte kodlu (satır sonu korunur): ' + disk);
    assert.ok(disk.includes('G100("toplam: {sayaç}", G201(sayaç))'), disk);
    assert.strictEqual(goster(o.dosya), b.getText());
    // Kayıt belgenin metnine dokunmaz: geri al geçmişi aynen durur, geri alınıp kaydedilince
    // disk ilk hâline döner.
    assert.strictEqual(b.gecmis.length, 1);
    b.geriAl();
    assert.ok(await b.save());
    assert.deepStrictEqual(fs.readFileSync(o.dosya), ilkDisk);
    assert.deepStrictEqual(o.durum.get(BEKLEYEN_ANAHTARI, []), []);
});

test('seçim (hata bağlantısının satırı ve sütunu) görünüme olduğu gibi taşınır', async () => {
    const o = ortam();
    const secim = { start: { line: 4, character: 7 }, end: { line: 4, character: 7 } };
    const editor = await ac(o, o.uri, { selection: secim });
    assert.deepStrictEqual(editor.selection, secim);
});

test('satır eşlemesi: tr kontrol\'ün hata konumu görünümde aynı satır ve sütun', async () => {
    const o = ortam({ disk: kodla(TURKCE.replace('yazdır("sayaç: {sayaç}")', 'yazdır(sayacc)')) });
    const s = spawnSync(TR, ['kontrol', o.dosya], { windowsHide: true });
    const m = /Satır (\d+), Sütun (\d+): Tanımsız ad/.exec(s.stderr.toString('utf8'));
    assert.ok(m, s.stderr.toString('utf8'));
    const b = (await ac(o, o.uri)).document;
    const satir = b.lineAt(Number(m[1]) - 1).text;
    assert.strictEqual(satir.slice(Number(m[2]) - 1, Number(m[2]) - 1 + 'sayacc'.length), 'sayacc', satir);
});

test('bozuk metin kayıpsız kaydedilir: başlıksız Türkçe, bir kez bilgi, ilk geçerli kayıtta yeniden kodlu', async () => {
    const durum = new SahteDurum();
    const o = ortam({ durum });
    const b = (await ac(o, o.uri)).document;
    const yarim = b.getText() + 'eğer sayaç > 2 {\r\n    yazdır("ç';
    b.duzenle(yarim);
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.dosya, 'utf8'), yarim, 'yazılan her şey diskte olmalı (başlıksız Türkçe)');
    assert.strictEqual(o.kayit.bilgiler.length, 1);
    assert.ok(o.kayit.bilgiler[0].includes('sözdizimi hatası düzelince') && o.kayit.bilgiler[0].includes('kaybolmadı'), o.kayit.bilgiler[0]);
    b.duzenle(yarim + 'o');
    assert.ok(await b.save());
    assert.strictEqual(o.kayit.bilgiler.length, 1, 'bilgi bir kez verilir');
    assert.strictEqual(o.durum.get(BEKLEYEN_ANAHTARI, []).length, 1, 'dosya kodlanmayı bekler');

    // VS Code yeniden açıldı: bekleyen dosya yine görünümle açılır, ilk geçerli kayıtta kodlanır.
    const o2 = kur({ ayar: TR, klasor: o.klasor, durum });
    const b2 = (await ac(o2, o.uri)).document;
    assert.strictEqual(b2.getText(), yarim + 'o');
    b2.duzenle(b2.getText() + 'k")\r\n}\r\n');
    assert.ok(await b2.save());
    const disk = fs.readFileSync(o.dosya, 'utf8');
    assert.ok(disk.startsWith(kodluBicim.BASLIK + '\r\n') && disk.includes('A001 sayaç > 2'), disk);
    assert.strictEqual(goster(o.dosya), b2.getText());
    assert.deepStrictEqual(durum.get(BEKLEYEN_ANAHTARI, []), []);
    assert.strictEqual(o2.kayit.bilgiler.length, 0);
});

test('çeviri hatası (sözdizimi dışı) da kayıpsız: Türkçe mesaj, dosya kodlanmayı bekler', async () => {
    const o = ortam();
    const b = (await ac(o, o.uri)).document;
    const asil = kodluBicim.cevir;
    kodluBicim.cevir = async () => ({ kod: 1, cikti: Buffer.alloc(0), hata: '✗ çeviri kayıpsız olmadı (programın anlamı ya da metni değişecekti), dosyaya dokunulmadı\n' });
    try {
        b.duzenle(b.getText() + 'yazdır(1)\r\n');
        assert.ok(await b.save());
    } finally {
        kodluBicim.cevir = asil;
    }
    assert.strictEqual(fs.readFileSync(o.dosya, 'utf8'), b.getText());
    assert.strictEqual(o.kayit.hatalar.length, 1);
    assert.ok(o.kayit.hatalar[0].includes('çeviri kayıpsız olmadı') && o.kayit.hatalar[0].includes('kaybolmadı'), o.kayit.hatalar[0]);
    assert.strictEqual(o.durum.get(BEKLEYEN_ANAHTARI, []).length, 1);
});

test('kodlu olmayan dosya olduğu gibi açılır ve kaydedilir; yorumlayıcı hiç çalışmaz', async () => {
    const o = ortam({ disk: TURKCE });
    let cagri = 0;
    const asil = kodluBicim.cevir;
    kodluBicim.cevir = async (...a) => { cagri++; return asil(...a); };
    try {
        await o.vscode.window.showTextDocument(o.uri, { preview: true });
        await new Promise((coz) => setTimeout(coz, 200));
        assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
        const b = o.vscode.window.activeTextEditor.document;
        b.duzenle(b.getText() + '// ek\r\n');
        assert.ok(await b.save());
    } finally {
        kodluBicim.cevir = asil;
    }
    assert.strictEqual(fs.readFileSync(o.dosya, 'utf8'), TURKCE + '// ek\r\n', 'eklenti kendiliğinden kodlamaz');
    assert.strictEqual(cagri, 0);
});

test('ayrıştırılamayan kodlu dosya (ör. birleştirme çakışması) ham hâliyle gösterilir, olduğu gibi kaydedilir', async () => {
    const ham = kodluBicim.BASLIK + '\n<<<<<<< HEAD\nA030 x = 1\n=======\nA030 x = 2\n>>>>>>> dal\n';
    const o = ortam({ disk: ham });
    const b = (await ac(o, o.uri)).document;
    assert.strictEqual(b.getText(), ham);
    assert.strictEqual(o.kayit.uyarilar.length, 1);
    assert.ok(o.kayit.uyarilar[0].includes('ham (kodlu) hâliyle'), o.kayit.uyarilar[0]);
    const cozulmus = kodluBicim.BASLIK + '\nA030 x = 2\n';
    b.duzenle(cozulmus);
    assert.ok(await b.save());
    assert.strictEqual(fs.readFileSync(o.dosya, 'utf8'), cozulmus);
    assert.strictEqual(goster(o.dosya), 'değişken x = 2\n');
});

test('yazım dili ayarı tr göster ve tr kodla komutlarına --dil ile geçer', async () => {
    const o = ortam({ ayarlar: { yazimDili: 'tr' } });
    const cagrilar = [];
    const asil = kodluBicim.cevir;
    kodluBicim.cevir = async (...a) => { cagrilar.push(a.slice(1, 3)); return asil(...a); };
    try {
        const b = (await ac(o, o.uri)).document;
        b.duzenle(b.getText() + '\r\n');
        await b.save();
    } finally {
        kodluBicim.cevir = asil;
    }
    assert.deepStrictEqual(cagrilar, [['göster', 'tr'], ['kodla', 'tr']]);
});

test('dosya dışarıda değişince görünümün adresiyle bildirilir; tarih diskteki dosyanınki', async () => {
    const o = ortam();
    const gorunum = o.dosyaSistemleri['tr-kodlu'].p;
    const adres = o.uri.with({ scheme: 'tr-kodlu' });
    const olaylar = [];
    gorunum.onDidChangeFile((e) => olaylar.push(...e));
    const izleme = gorunum.watch(adres, { recursive: false, excludes: [] });
    const iz = o.kayit.izleyiciler[o.kayit.izleyiciler.length - 1];
    assert.strictEqual(iz.desen.base.fsPath, o.klasor);
    assert.strictEqual(iz.desen.pattern, 'program.tr');
    const once = await gorunum.stat(adres);
    fs.writeFileSync(o.dosya, kodla('yazdır(1)\n'));
    fs.utimesSync(o.dosya, new Date(), new Date(Date.now() + 5000));
    iz.degisti.fire(o.uri);
    assert.deepStrictEqual(olaylar.map((e) => [e.type, e.uri.toString()]), [[1, adres.toString()]]);
    const sonra = await gorunum.stat(adres);
    assert.notStrictEqual(sonra.mtime, once.mtime);
    assert.strictEqual(sonra.size, fs.statSync(o.dosya).size);
    assert.strictEqual(Buffer.from(await gorunum.readFile(adres)).toString('utf8'), 'yazdır(1)\n');
    izleme.dispose();
    assert.ok(iz.kapandi);
    await assert.rejects(gorunum.stat(Uri.file(path.join(o.klasor, 'yok.tr')).with({ scheme: 'tr-kodlu' })), (e) => e.code === 'FileNotFound');
});

test('hata ayıklama: görünümde F5 → program diskteki dosya, görünümün adresi tr dap\'a gider', async () => {
    const o = ortam();
    const editor = await ac(o, o.uri);
    const adres = editor.document.uri.toString();
    const { saglayici, surec } = o;
    // launch.json olmadan
    const y = await saglayici.resolveDebugConfiguration(undefined, {});
    assert.strictEqual(y.program, o.dosya);
    assert.strictEqual(y.cwd, o.klasor);
    assert.strictEqual(y.gorunumAdresi, adres);
    // launch.json'daki ${file}: VS Code görünümde çözemez, eklenti dosyanın kendisinden çözer.
    const lj = await saglayici.resolveDebugConfiguration(undefined, { type: 'tr', request: 'launch', name: 'TR', program: '${file}', cwd: '${fileDirname}', args: ['${fileBasename}'] });
    assert.deepStrictEqual([lj.program, lj.cwd, lj.args, lj.gorunumAdresi], [o.dosya, o.klasor, ['program.tr'], adres]);
    // Programı yoluyla veren ayar: kodlu dosyaysa görünümün adresi eklenir.
    const yol = await saglayici.resolveDebugConfigurationWithSubstitutedVariables(undefined, { type: 'tr', request: 'launch', name: 'TR', program: o.dosya });
    assert.strictEqual(yol.gorunumAdresi, adres);
    const d = surec.createDebugAdapterDescriptor({ configuration: y });
    assert.deepStrictEqual([d.komut, d.argumanlar], [TR, ['dap']]);
    // Başlık çubuğundaki "Hata Ayıkla" düğmesi
    await o.komutlar['tr.hataAyiklaBaslat']();
    assert.strictEqual(o.kayit.hataAyiklamalar[0].gorunumAdresi, adres);
    assert.strictEqual(o.kayit.hataAyiklamalar[0].program, o.dosya);
});

test('görünümde çalıştır: önce kodlu kaydedilir, görev diskteki dosyayla başlar', async () => {
    const o = ortam();
    const b = (await ac(o, o.uri)).document;
    b.duzenle(b.getText() + 'yazdır("son")\r\n');
    await o.komutlar['tr.calistir']();
    await olana(() => o.kayit.gorevler.length === 1, 'çalıştırma görevi');
    assert.deepStrictEqual(o.kayit.gorevler[0].execution.argumanlar, [o.dosya]);
    assert.ok(!b.isDirty);
    assert.ok(fs.readFileSync(o.dosya, 'utf8').includes('G100("son")'));
});

test('kısıtlı kip: hiçbir şey çalıştırılmaz, salt okunur ham gösterim ve açıklama; güven verilince Türkçe', async () => {
    const o = ortam({ guvenilir: false });
    let cagri = 0;
    const asil = kodluBicim.cevir;
    kodluBicim.cevir = async (...a) => { cagri++; return asil(...a); };
    try {
        const b = (await ac(o, o.uri, {}, 'tr-ham')).document;
        assert.ok(b.getText().startsWith(kodluBicim.BASLIK), 'ham hâl');
        assert.strictEqual(await b.save(), false, 'salt okunur');
        await assert.rejects(o.dosyaSistemleri['tr-ham'].p.writeFile(b.uri, Buffer.from('x'), { create: true, overwrite: true }), (e) => e.code === 'NoPermissions');
        assert.ok(o.dosyaSistemleri['tr-ham'].secenekler.isReadonly);
        assert.strictEqual(o.kayit.bilgiler.length, 1);
        assert.ok(o.kayit.bilgiler[0].includes('Kısıtlı kipte') && o.kayit.bilgiler[0].includes('salt okunur'), o.kayit.bilgiler[0]);
        assert.strictEqual(cagri, 0, 'kısıtlı kipte yorumlayıcı çalıştırılmaz');
        // Kısıtlı kipte görünüm adresi de (ör. geri yüklenen sekme) çeviri yapmaz.
        const gorunum = o.dosyaSistemleri['tr-kodlu'].p;
        assert.ok(kodluBicim.kodluMu(await gorunum.readFile(o.uri.with({ scheme: 'tr-kodlu' }))));
        assert.strictEqual(cagri, 0);
    } finally {
        kodluBicim.cevir = asil;
    }
    o.vscode.workspace.isTrusted = true;
    o.olaylar.guven.fire();
    await olana(() => o.vscode.window.activeTextEditor && o.vscode.window.activeTextEditor.document.uri.scheme === 'tr-kodlu', 'güvenden sonra görünüm');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.getText(), TURKCE);
});

test('kirli ham belge ve yorumlayıcısız pencere yönlendirilmez (hiçbir şey kaybolmaz)', async () => {
    const o = ortam({ ayar: path.join(kok, 'olmayan', 'tr.exe') });
    await o.vscode.window.showTextDocument(o.uri, { preview: true });
    await olana(() => o.kayit.hatalar.length === 1, 'yorumlayıcı bulunamadı mesajı');
    assert.strictEqual(o.vscode.window.activeTextEditor.document.uri.scheme, 'file');
    assert.ok(o.kayit.hatalar[0].includes('kodlu biçimde saklanıyor'), o.kayit.hatalar[0]);

    // Kaydedilmemiş değişikliği olan ham belge (ör. önceki oturumdan geri yüklenen) yerinde kalır.
    const o2 = ortam();
    const ham = await o2.vscode.workspace.openTextDocument(o2.uri);
    ham.duzenle(ham.getText() + '// kaydedilmedi\n');
    await o2.vscode.window.showTextDocument(o2.uri, { preview: true });
    await new Promise((coz) => setTimeout(coz, 300));
    assert.strictEqual(o2.vscode.window.activeTextEditor.document, ham);
    assert.deepStrictEqual(sekmeler(o2), [o2.uri.toString()]);
});

test('Kodlu Hâlini Göster komutu ham hâli yanda salt okunur açar', async () => {
    const o = ortam();
    await ac(o, o.uri);
    await o.komutlar['tr.kodluHaliniGoster']();
    const e = o.vscode.window.activeTextEditor;
    assert.strictEqual(e.document.uri.scheme, 'tr-ham');
    assert.strictEqual(e.viewColumn, 2);
    assert.deepStrictEqual(Buffer.from(e.document.getText(), 'utf8'), fs.readFileSync(o.dosya));
});

(async () => {
    let gecen = 0;
    let atlanan = 0;
    for (const { ad, fn, trGerekir } of testler) {
        if (trGerekir && !TR) {
            atlanan++;
            continue;
        }
        try {
            await fn();
            gecen++;
        } catch (e) {
            console.error('KALDI: ' + ad + '\n' + (e && e.stack));
            process.exitCode = 1;
        }
    }
    // Eklentinin yedek zamanlayıcıları (sekme yönlendirmesi) bitsin; sonra geçici klasör silinir.
    await new Promise((coz) => setTimeout(coz, 300));
    fs.rmSync(kok, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
    console.log(`coded_view_test: ${gecen}/${testler.length} geçti` + (atlanan ? ` (${atlanan} test atlandı: TR yorumlayıcısı yok; TR_YORUMLAYICI ile verin)` : ''));
    // Eklentinin yedek zamanlayıcıları sınamayı bekletmesin.
    process.exit(process.exitCode || 0);
})();
