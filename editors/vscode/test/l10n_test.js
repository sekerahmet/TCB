// Eklentinin yerelleştirmesinin testi: kaynaktaki her ileti Türkçe pakette vardır, pakette artık
// (kaynakta olmayan) ileti kalmaz. package.json'ın %anahtar%ları için bkz. behavior_extension_test.go. Bağımlılıksız;
// depo kökünden: node editors/vscode/test/l10n_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { createSuite, setup } = require('./fake_vscode');

const ROOT = path.join(__dirname, '..');
const suite = createSuite('l10n_test', {
    summary: ({ passed }) => (process.exitCode ? 'l10n_test: kalan var' : `l10n_test: ${passed} test geçti`),
});
const test = suite.test;

const readJSON = (...parts) => JSON.parse(fs.readFileSync(path.join(ROOT, ...parts), 'utf8'));

// sourceMessages eklenti kaynaklarındaki t('...') çağrılarının ilk bağımsız değişkenleridir.
function sourceMessages() {
    const found = new Map(); // ileti -> ilk geçtiği dosya
    const call = /(?<![\w.])t\(\s*('(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*")/g;
    for (const file of fs.readdirSync(ROOT).filter((f) => f.endsWith('.js'))) {
        const text = fs.readFileSync(path.join(ROOT, file), 'utf8');
        for (const m of text.matchAll(call)) {
            const message = new Function('return ' + m[1])();
            if (!found.has(message)) {
                found.set(message, file);
            }
        }
    }
    return found;
}

const placeholders = (text) => (text.match(/\{\d+\}/g) || []).sort().join(',');

test('kaynaktaki her ileti Türkçe pakette vardır ve yer tutucuları aynıdır', () => {
    const bundle = readJSON('l10n', 'bundle.l10n.tr.json');
    const missing = [];
    const mismatched = [];
    for (const [message, file] of sourceMessages()) {
        if (typeof bundle[message] !== 'string' || bundle[message] === '') {
            missing.push(`${file}: ${message}`);
        } else if (placeholders(bundle[message]) !== placeholders(message)) {
            mismatched.push(`${file}: ${message}`);
        }
    }
    assert.deepStrictEqual(missing, [], 'Türkçe paketi olmayan iletiler');
    assert.deepStrictEqual(mismatched, [], 'yer tutucuları çevirisinden farklı iletiler');
});

test('Türkçe pakette kaynakta olmayan ileti kalmaz', () => {
    const bundle = readJSON('l10n', 'bundle.l10n.tr.json');
    const messages = sourceMessages();
    assert.deepStrictEqual(Object.keys(bundle).filter((key) => !messages.has(key)), []);
});

test('package.json yerelleştirme klasörünü bildirir ve l10n API\'sini taşıyan sürümü ister', () => {
    const p = readJSON('package.json');
    assert.strictEqual(p.l10n, './l10n');
    assert.ok(fs.existsSync(path.join(ROOT, 'l10n', 'bundle.l10n.tr.json')));
    const [major, minor] = p.engines.vscode.replace(/^\^/, '').split('.').map(Number);
    assert.ok(major > 1 || minor >= 73, `vscode.l10n 1.73'te geldi: ${p.engines.vscode}`);
});

test('eklenti iletileri çeviri paketiyle Türkçe gösterilir', async () => {
    const bundle = readJSON('l10n', 'bundle.l10n.tr.json');
    // l10n.js tek bir modüldür: kurulumlar sırayla kullanılır.
    const english = setup({});
    await english.commands['tr.run']();
    assert.strictEqual(english.log.errors[0], 'No .tr file is open to run.');
    const turkish = setup({ bundle });
    await turkish.commands['tr.run']();
    assert.strictEqual(turkish.log.errors[0], bundle['No .tr file is open to run.']);
});

suite.run();
