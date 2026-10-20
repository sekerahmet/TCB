// Kodlu saklamanın (docs/spec/17_kodlar.md, 17.1-17.6 ve "Düzenleyicide") eklentideki vscode'a bağlı olmayan
// parçaları: kodlu biçim başlığını tanıma ve 'tr' ile çeviri. Eklentide ikinci bir çevirici
// yoktur: Türkçe görünüm 'tr göster -', kodlu kayıt 'tr kodla -' ile üretilir (metin standart
// girişten okunur, sonuç standart çıktıya yazılır). Node ile ayrıca sınanır
// (test/coded_view_test.js).
'use strict';

const { spawn } = require('child_process');

// HEADER kodlu bir dosyanın (varsa BOM'dan sonraki) ilk satırıdır (17.1).
const HEADER = '//! tr kodlu 1';
const HEADER_BYTES = Buffer.from(HEADER, 'utf8');

// EXIT_SYNTAX 'tr kodla -' ve 'tr göster -' komutlarının "metin ayrıştırılamadı, çevrilmedi"
// çıkış kodudur (cmd/tr/coded.go, exitSyntax).
const EXIT_SYNTAX = 2;

// TIMEOUT bir çevirinin en uzun süresidir (milisaniye); aşılırsa süreç durdurulur.
const TIMEOUT = 30000;

// isCoded dosyanın baytları kodlu biçimde mi: (BOM'dan sonraki) ilk satır tam olarak HEADER
// (satır sonu LF ya da CRLF, ya da dosyanın tek satırı). Kural kaynak.Ayir ile aynıdır.
function isCoded(data) {
    const bytes = Buffer.isBuffer(data) ? data : Buffer.from(data.buffer, data.byteOffset, data.byteLength);
    let i = 0;
    if (bytes.length >= 3 && bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf) {
        i = 3;
    }
    if (bytes.length < i + HEADER_BYTES.length || !bytes.subarray(i, i + HEADER_BYTES.length).equals(HEADER_BYTES)) {
        return false;
    }
    const j = i + HEADER_BYTES.length;
    return j === bytes.length || bytes[j] === 0x0a || (bytes[j] === 0x0d && bytes[j + 1] === 0x0a);
}

// translate metni 'tr <command> - --dil <lang>' ile çevirir; komut 'göster' (kodlu → yazım dili) ya da
// 'kodla' (yazım dili → kodlu). Kabuk kullanılmaz. Hiçbir zaman reddetmez:
//   { code: 0, output }                    çevrildi (output: Buffer)
//   { code: EXIT_SYNTAX, error }           metin ayrıştırılamadı (error: tr'nin Türkçe mesajı)
//   { code: 1 | -1, error }                başka bir hata (-1: süreç başlatılamadı ya da durduruldu)
function translate(interpreter, command, lang, input, timeoutMs) {
    return new Promise((resolve) => {
        const finish = (result) => {
            clearTimeout(timer);
            resolve(result);
        };
        let proc;
        try {
            proc = spawn(interpreter, [command, '-', '--dil', lang], { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
        } catch (e) {
            resolve({ code: -1, output: Buffer.alloc(0), error: 'TR yorumlayıcısı başlatılamadı: ' + (e && e.message ? e.message : e) });
            return;
        }
        const timer = setTimeout(() => proc.kill(), timeoutMs || TIMEOUT);
        const output = [];
        const error = [];
        proc.stdout.on('data', (d) => output.push(d));
        proc.stderr.on('data', (d) => error.push(d));
        proc.on('error', (e) => finish({ code: -1, output: Buffer.alloc(0), error: 'TR yorumlayıcısı başlatılamadı: ' + e.message }));
        proc.on('close', (code) => {
            const text = Buffer.concat(error).toString('utf8');
            finish({
                code: code === null ? -1 : code,
                output: Buffer.concat(output),
                error: code === null ? 'TR yorumlayıcısı zamanında yanıt vermedi; çeviri durduruldu.' : text
            });
        });
        proc.stdin.on('error', () => {}); // süreç girdiyi okumadan çıktıysa (EPIPE)
        proc.stdin.end(Buffer.from(input));
    });
}

// errorSummary tr'nin hata çıktısından kullanıcıya gösterilecek ilk hatayı çıkarır (kod
// çerçeveleri ve başlıklar atılır).
function errorSummary(text) {
    const lines = String(text || '').split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const item = lines.find((s) => /^[•✗]/.test(s)) || lines.find((s) => !/^[❌⚠]/.test(s)) || lines[0] || '';
    const summary = item.replace(/^[•✗]\s*/, '');
    return summary.length > 300 ? summary.slice(0, 297) + '...' : summary;
}

module.exports = { HEADER, EXIT_SYNTAX, isCoded, translate, errorSummary };
