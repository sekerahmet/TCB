// Klasörsüz pencerenin çalıştırma terminali (program_terminal.js), gerçek bir süreçle (Node):
// çıktı terminalde kalır, girdi satır satır gider, program bitince terminal açık kalır ve bir
// tuşla kapanır. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/program_terminal_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const { createSuite } = require('./fake_vscode');
const { programTerminal } = require('../program_terminal');

const suite = createSuite('program_terminal_test');
const test = suite.test;

// start, Node betiğini sahte terminalde açar; yazılanlar ve kapanış kaydedilir.
function start(script, env) {
    const p = programTerminal({ command: process.execPath, args: ['-e', script], cwd: __dirname, env });
    const t = { p, out: '', closed: false };
    p.onDidWrite((s) => { t.out += s; });
    p.onDidClose(() => { t.closed = true; });
    p.open();
    return t;
}

// until, koşul sağlanana kadar bekler (en çok 10 sn).
async function until(t, cond) {
    for (let i = 0; i < 1000 && !cond(); i++) {
        await new Promise((r) => setTimeout(r, 10));
    }
    assert.ok(cond(), 'beklenen olmadı; terminal:\n' + JSON.stringify(t.out));
}

test('çıktı terminalde kalır, satır sonları \\r\\n olur, bitince çıkış kodu yazılır', async () => {
    const t = start('console.log("Merhaba, dünya!"); console.error("uyarı"); process.exitCode = 3', { TR_DIL: 'tr' });
    await until(t, () => t.out.includes('exit code'));
    assert.ok(t.out.includes('Merhaba, dünya!\r\n'), t.out);
    assert.ok(t.out.includes('uyarı\r\n'), t.out);
    assert.ok(t.out.includes('The program finished (exit code 3).'), t.out);
    assert.ok(!t.out.replace(/\r\n/g, '').includes('\n'), 'yalın \\n kalmamalı');
    assert.strictEqual(t.closed, false, 'program bitince terminal açık kalmalı');
    t.p.handleInput('x');
    assert.strictEqual(t.closed, true, 'bir tuşla kapanmalı');
});

test('ortam değişkeni sürece geçer', async () => {
    const t = start('console.log("dil=" + process.env.TR_DIL)', { TR_DIL: 'tr' });
    await until(t, () => t.out.includes('exit code'));
    assert.ok(t.out.includes('dil=tr'), t.out);
});

test('girdi: harfler gösterilir, geri silme ve ok tuşları satırı düzenler, Enter yollar', async () => {
    const t = start('process.stdin.once("data", (d) => { console.log("okunan=" + JSON.stringify(String(d))); process.exit(0); })');
    t.p.handleInput('ağx');
    t.p.handleInput('\x7f');
    t.p.handleInput('\x1b[Dç\r');
    await until(t, () => t.out.includes('exit code'));
    assert.ok(t.out.includes('ağx\b \bç\r\n'), JSON.stringify(t.out));
    assert.ok(t.out.includes('okunan="ağç\\n"'), t.out);
});

test('Ctrl+D girdiyi kapatır, Ctrl+C programı durdurur', async () => {
    const eof = start('process.stdin.resume(); process.stdin.on("end", () => console.log("girdi bitti"))');
    eof.p.handleInput('\x04');
    await until(eof, () => eof.out.includes('exit code 0'));
    assert.ok(eof.out.includes('girdi bitti'), eof.out);

    const t = start('setInterval(() => {}, 1000)');
    t.p.handleInput('\x03');
    await until(t, () => t.out.includes('The program was stopped') || t.out.includes('exit code'));
    assert.ok(t.out.includes('^C'), t.out);
    assert.strictEqual(t.closed, false);
});

test('terminal kapatılınca süreç durdurulur', async () => {
    const t = start('setInterval(() => {}, 1000)');
    t.p.close();
    await new Promise((r) => setTimeout(r, 300));
    assert.ok(!t.out.includes('The program finished'), 'kapatılmış terminale yazılmamalı');
});

test('başlatılamayan program terminalde bildirilir', async () => {
    const p = programTerminal({ command: 'tr-olmayan-yorumlayici-xyz', args: [], cwd: __dirname, env: {} });
    let out = '';
    p.onDidWrite((s) => { out += s; });
    p.open();
    await until({ out }, () => out.includes('Could not start the program'));
});

suite.run();
