// Klasörsüz pencerede çalıştırma terminali: yorumlayıcıyı eklentinin kendisi başlatır ve
// çıktısını VS Code'un sahte terminaline (Pseudoterminal) yazar. Program bitince terminal
// açık kalır, çıkış kodu yazılır; bir tuşa basınca kapanır (görev terminalinin davranışı).
//
// Neden: klasör açılmadan VS Code görev çalıştırmaz; yorumlayıcıyı terminalin kabuğu yapmak
// (shellPath) ise süreç bitince terminali hemen kapatır, çıktı bir an görünüp kaybolur.
// Kabuk terminaline komut yazmak (sendText; Python eklentisi böyle yapar) her kabuğun kendi
// tırnak/kaçış kuralını ister. Sahte terminal ikisinden de kurtulur: süreç kabuksuz başlar,
// terminalin ömrü sürece bağlı değildir.
//
// Girdi (girdi()) satır satır gider: yazılan harfler terminalde gösterilir, Geri silme son
// harfi siler, Enter satırı programa yollar, Ctrl+C programı durdurur, boş satırda Ctrl+D
// girdiyi kapatır. vscode modülüne bağlı değildir (test/program_terminal_test.js).
'use strict';

const { spawn } = require('child_process');
const { StringDecoder } = require('string_decoder');
const { t } = require('./l10n');

// emitter VS Code'un Event biçiminde (dinleyici -> { dispose }) basit bir olay kaynağıdır.
function emitter() {
    const listeners = new Set();
    return {
        event: (fn) => {
            listeners.add(fn);
            return { dispose: () => listeners.delete(fn) };
        },
        fire: (value) => {
            for (const fn of [...listeners]) {
                fn(value);
            }
        }
    };
}

// Ok tuşları gibi kaçış dizileri satır düzenlemesinde yok sayılır.
const ESCAPES = /\x1b\[[0-9;]*[A-Za-z~]|\x1bO[A-Za-z]/g;

// programTerminal yorumlayıcıyı (command, args) cwd'de çalıştıran sahte terminali döner;
// env süreç ortamına eklenir.
function programTerminal({ command, args, cwd, env }) {
    const write = emitter();
    const close = emitter();
    let child;
    let running = false;
    let line = '';

    const show = (text) => write.fire(text.replace(/\r?\n/g, '\r\n'));
    const pipe = (stream) => {
        const decoder = new StringDecoder('utf8');
        stream.on('data', (chunk) => show(decoder.write(chunk)));
        stream.on('end', () => show(decoder.end()));
    };

    function finish(message) {
        if (!running) {
            return;
        }
        running = false;
        show(`\n${message} ${t('Press any key to close.')}\n`);
    }

    function type(data) {
        for (const ch of data.replace(ESCAPES, '')) {
            if (ch === '\r') {
                write.fire('\r\n');
                child.stdin.write(line + '\n');
                line = '';
            } else if (ch === '\x7f' || ch === '\b') {
                if (line) {
                    line = [...line].slice(0, -1).join('');
                    write.fire('\b \b');
                }
            } else if (ch === '\x03') {
                write.fire('^C\r\n');
                child.kill();
            } else if (ch === '\x04') {
                if (!line) {
                    child.stdin.end();
                }
            } else if (ch >= ' ') {
                line += ch;
                write.fire(ch);
            }
        }
    }

    return {
        // Başlatılacak süreç (yalnızca okunur; sınamada argümanlar buradan görülür).
        command, args, cwd, env,
        onDidWrite: write.event,
        onDidClose: close.event,
        open() {
            running = true;
            child = spawn(command, args, { cwd, env: Object.assign({}, process.env, env), windowsHide: true });
            pipe(child.stdout);
            pipe(child.stderr);
            // Program girdiyi okumadan biterse yazılan satır kapalı boruya gider; yok sayılır.
            child.stdin.on('error', () => {});
            child.on('error', (e) => finish(t('Could not start the program: {0}.', e.message)));
            child.on('close', (code, signal) => finish(signal ? t('The program was stopped ({0}).', signal) : t('The program finished (exit code {0}).', code)));
        },
        close() {
            if (running) {
                running = false;
                child.kill();
            }
        },
        handleInput(data) {
            if (running) {
                type(data);
            } else {
                close.fire();
            }
        }
    };
}

module.exports = { programTerminal };
