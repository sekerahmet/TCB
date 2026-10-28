// Kodlu saklamanın (dil tanımı 2.1; docs/spec/17_kodlar.md, "Düzenleyicide") eklentideki vscode'a bağlı
// olmayan parçaları: başlığı tanıma ve 'tr' ile çeviri. Eklentide ikinci bir çevirici yoktur: görünüm
// 'tr göster -', kodlu kayıt 'tr kodla -' ile üretilir (metin standart girişten okunur, sonuç standart
// çıktıya yazılır). Node ile ayrıca sınanır (test/coded_view_test.js).
'use strict';

const { spawn } = require('child_process');
const { t } = require('./l10n');

// HEADER bir TR dosyasının (varsa BOM'dan sonraki) ilk satırıdır; ilk satır '#!' ile başlıyorsa
// ikinci satırıdır. Dosyanın dili yoktur: başlık yalnız biçimin sürümünü söyler (2.1).
const HEADER = '# TR 4';

// SHEBANG ilk satırdaki yorumdur: dosya betik gibi doğrudan çalıştırılır (./program.tr).
const SHEBANG = '#!';

// EXIT_SYNTAX 'tr kodla -' ve 'tr göster -' komutlarının "metin ayrıştırılamadı, çevrilmedi"
// çıkış kodudur (cmd/tr/coded.go, exitSyntax).
const EXIT_SYNTAX = 2;

// TIMEOUT bir çevirinin en uzun süresidir (milisaniye); aşılırsa süreç durdurulur.
const TIMEOUT = 30000;

// line bytes'ın start'tan başlayan satırıdır (satır sonu LF ya da CRLF hariç) ve sonraki satırın
// başı.
function line(bytes, start) {
    let end = bytes.indexOf(0x0a, start);
    const next = end < 0 ? bytes.length : end + 1;
    if (end < 0) {
        end = bytes.length;
    } else if (end > start && bytes[end - 1] === 0x0d) {
        end--; // CRLF
    }
    return { text: bytes.subarray(start, end).toString('utf8'), next };
}

// isCoded dosyanın baytları bir TR dosyası mı: (BOM'dan sonraki) ilk satır, '#!' ile başlıyorsa ikinci
// satır, tam olarak HEADER. Kural source.Split ile aynıdır.
function isCoded(data) {
    const bytes = Buffer.isBuffer(data) ? data : Buffer.from(data.buffer, data.byteOffset, data.byteLength);
    let i = 0;
    if (bytes.length >= 3 && bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf) {
        i = 3;
    }
    let first = line(bytes, i);
    if (first.text.startsWith(SHEBANG)) {
        first = line(bytes, first.next);
    }
    return first.text === HEADER;
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
            resolve({ code: -1, output: Buffer.alloc(0), error: t('Could not start the TR interpreter: {0}', e && e.message ? e.message : e) });
            return;
        }
        const timer = setTimeout(() => proc.kill(), timeoutMs || TIMEOUT);
        const output = [];
        const error = [];
        proc.stdout.on('data', (d) => output.push(d));
        proc.stderr.on('data', (d) => error.push(d));
        proc.on('error', (e) => finish({ code: -1, output: Buffer.alloc(0), error: t('Could not start the TR interpreter: {0}', e.message) }));
        proc.on('close', (code) => {
            const text = Buffer.concat(error).toString('utf8');
            finish({
                code: code === null ? -1 : code,
                output: Buffer.concat(output),
                error: code === null ? t('The TR interpreter did not respond in time; the translation was stopped.') : text
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
