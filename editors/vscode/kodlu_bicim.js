// Kodlu saklamanın (docs/spec/17_kodlar.md, 17.1-17.6 ve "Düzenleyicide") eklentideki vscode'a bağlı olmayan
// parçaları: kodlu biçim başlığını tanıma ve 'tr' ile çeviri. Eklentide ikinci bir çevirici
// yoktur: Türkçe görünüm 'tr göster -', kodlu kayıt 'tr kodla -' ile üretilir (metin standart
// girişten okunur, sonuç standart çıktıya yazılır). Node ile ayrıca sınanır
// (test/kodlu_gorunum_test.js).
'use strict';

const { spawn } = require('child_process');

// BASLIK kodlu bir dosyanın (varsa BOM'dan sonraki) ilk satırıdır (17.1).
const BASLIK = '//! tr kodlu 1';
const BASLIK_BAYTLARI = Buffer.from(BASLIK, 'utf8');

// CIKIS_SOZDIZIMI 'tr kodla -' ve 'tr göster -' komutlarının "metin ayrıştırılamadı, çevrilmedi"
// çıkış kodudur (cmd/tr/kodlu.go, cikisSozdizimi).
const CIKIS_SOZDIZIMI = 2;

// ZAMAN_ASIMI bir çevirinin en uzun süresidir (milisaniye); aşılırsa süreç durdurulur.
const ZAMAN_ASIMI = 30000;

// kodluMu dosyanın baytları kodlu biçimde mi: (BOM'dan sonraki) ilk satır tam olarak BASLIK
// (satır sonu LF ya da CRLF, ya da dosyanın tek satırı). Kural kaynak.Ayir ile aynıdır.
function kodluMu(veri) {
    const bayt = Buffer.isBuffer(veri) ? veri : Buffer.from(veri.buffer, veri.byteOffset, veri.byteLength);
    let i = 0;
    if (bayt.length >= 3 && bayt[0] === 0xef && bayt[1] === 0xbb && bayt[2] === 0xbf) {
        i = 3;
    }
    if (bayt.length < i + BASLIK_BAYTLARI.length || !bayt.subarray(i, i + BASLIK_BAYTLARI.length).equals(BASLIK_BAYTLARI)) {
        return false;
    }
    const j = i + BASLIK_BAYTLARI.length;
    return j === bayt.length || bayt[j] === 0x0a || (bayt[j] === 0x0d && bayt[j + 1] === 0x0a);
}

// cevir metni 'tr <komut> - --dil <dil>' ile çevirir; komut 'göster' (kodlu → yazım dili) ya da
// 'kodla' (yazım dili → kodlu). Kabuk kullanılmaz. Hiçbir zaman reddetmez:
//   { kod: 0, cikti }                      çevrildi (cikti: Buffer)
//   { kod: CIKIS_SOZDIZIMI, hata }         metin ayrıştırılamadı (hata: tr'nin Türkçe mesajı)
//   { kod: 1 | -1, hata }                  başka bir hata (-1: süreç başlatılamadı ya da durduruldu)
function cevir(yorumlayici, komut, dil, girdi, zamanAsimi) {
    return new Promise((coz) => {
        const bitir = (sonuc) => {
            clearTimeout(zaman);
            coz(sonuc);
        };
        let surec;
        try {
            surec = spawn(yorumlayici, [komut, '-', '--dil', dil], { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
        } catch (e) {
            coz({ kod: -1, cikti: Buffer.alloc(0), hata: 'TR yorumlayıcısı başlatılamadı: ' + (e && e.message ? e.message : e) });
            return;
        }
        const zaman = setTimeout(() => surec.kill(), zamanAsimi || ZAMAN_ASIMI);
        const cikti = [];
        const hata = [];
        surec.stdout.on('data', (d) => cikti.push(d));
        surec.stderr.on('data', (d) => hata.push(d));
        surec.on('error', (e) => bitir({ kod: -1, cikti: Buffer.alloc(0), hata: 'TR yorumlayıcısı başlatılamadı: ' + e.message }));
        surec.on('close', (kod) => {
            const metin = Buffer.concat(hata).toString('utf8');
            bitir({
                kod: kod === null ? -1 : kod,
                cikti: Buffer.concat(cikti),
                hata: kod === null ? 'TR yorumlayıcısı zamanında yanıt vermedi; çeviri durduruldu.' : metin
            });
        });
        surec.stdin.on('error', () => {}); // süreç girdiyi okumadan çıktıysa (EPIPE)
        surec.stdin.end(Buffer.from(girdi));
    });
}

// hataOzeti tr'nin hata çıktısından kullanıcıya gösterilecek ilk hatayı çıkarır (kod
// çerçeveleri ve başlıklar atılır).
function hataOzeti(metin) {
    const satirlar = String(metin || '').split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const madde = satirlar.find((s) => /^[•✗]/.test(s)) || satirlar.find((s) => !/^[❌⚠]/.test(s)) || satirlar[0] || '';
    const ozet = madde.replace(/^[•✗]\s*/, '');
    return ozet.length > 300 ? ozet.slice(0, 297) + '...' : ozet;
}

module.exports = { BASLIK, CIKIS_SOZDIZIMI, kodluMu, cevir, hataOzeti };
