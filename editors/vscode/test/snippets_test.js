// Kod şablonlarının yazım diline göre seçilmesinin testi. Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/snippets_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.) Şablonların kendisi
// `go run ./tools/gen/snippets` ile üretilir ve tools/gen/snippets testlerinde sınanır.
'use strict';

const assert = require('assert');
const path = require('path');
const snippets = require('../snippets');
const { setup, createSuite } = require('./fake_vscode');

const suite = createSuite('snippets_test', {
    summary: ({ passed }) => (process.exitCode ? 'snippets_test: kalan var' : `snippets_test: ${passed} test geçti`),
});
const test = suite.test;

const table = snippets.loadTable(path.join(__dirname, '..'));

const prefixesOf = (lang) => snippets.completions(table, lang).map((c) => c.prefix);

test('şablon dosyası Türkçe ve İngilizce şablonları taşır', () => {
    assert.ok(table.tr && table.en);
    assert.strictEqual(table.tr.length, table.en.length, 'her dilde aynı şablonlar vardır');
});

test('şablonlar yazım dilinin sözcükleriyle gelir', () => {
    assert.ok(prefixesOf('tr').includes('fonksiyon') && prefixesOf('tr').includes('eğer'));
    assert.ok(prefixesOf('en').includes('func') && prefixesOf('en').includes('if'));
    assert.ok(!prefixesOf('en').includes('fonksiyon'), 'İngilizce yazana Türkçe sözcük sunulmaz');
    const english = snippets.completions(table, 'en').find((c) => c.prefix === 'if');
    assert.ok(english.snippet.startsWith('if ${1:condition} {'), english.snippet);
    const turkish = snippets.completions(table, 'tr').find((c) => c.prefix === 'eğer');
    assert.ok(turkish.snippet.startsWith('eğer ${1:koşul} {'), turkish.snippet);
});

test('şablonu olmayan dil Türkçeye düşer', () => {
    assert.deepStrictEqual(prefixesOf('xx'), prefixesOf('tr'));
});

// Gerçek VS Code'un sınıflarının, sağlayıcının kullandığı kadarı.
function fakeApi() {
    class Position {
        constructor(line, character) { this.line = line; this.character = character; }
        translate(lines, characters) { return new Position(this.line + lines, this.character + characters); }
    }
    class Range { constructor(start, end) { this.start = start; this.end = end; } }
    const registered = [];
    return {
        registered, Position, Range,
        CompletionItem: class { constructor(label, kind) { this.label = label; this.kind = kind; } },
        CompletionItemKind: { Snippet: 14 },
        SnippetString: class { constructor(value) { this.value = value; } },
        languages: { registerCompletionItemProvider: (selector, provider) => { registered.push({ selector, provider }); return { dispose() {} }; } }
    };
}

// documentWith tek satırlı sahte bir belgedir; imleç satırın sonundadır.
function documentWith(api, line) {
    const wordStart = line.search(/[\p{L}_]*$/u);
    return {
        position: new api.Position(0, line.length),
        document: {
            getWordRangeAtPosition: () => new api.Range(new api.Position(0, wordStart), new api.Position(0, line.length)),
            getText: (range) => line.slice(range.start.character, range.end.character)
        }
    };
}

test('sağlayıcı yazım diline göre sunar; ".ad" kısayolu imlecin solundaki noktayı da değiştirir', () => {
    const api = fakeApi();
    let language = 'tr';
    snippets.register(api, table, () => language);
    assert.deepStrictEqual(api.registered[0].selector, { language: 'tr' });
    const provide = (line) => {
        const { document, position } = documentWith(api, line);
        return api.registered[0].provider.provideCompletionItems(document, position);
    };
    assert.ok(provide('fon').some((item) => item.label === 'fonksiyon' && item.kind === 14));
    language = 'en';
    const items = provide('liste.fil');
    assert.ok(items.some((item) => item.label === 'func'));
    assert.ok(!items.some((item) => item.label === 'fonksiyon'));
    const member = items.find((item) => item.label === '.filter');
    assert.strictEqual(member.range.start.character, 5, 'nokta da değiştirilir');
    assert.strictEqual(member.insertText.value, '.filter { it > ${1:2} }');
    assert.strictEqual(items.find((item) => item.label === 'filter').range, undefined);
    assert.strictEqual(provide('fil').find((item) => item.label === '.filter').range, undefined, 'nokta yoksa aralık değişmez');
});

test('eklenti şablon sağlayıcısını etkinleşirken kaydeder', () => {
    const o = setup({});
    assert.deepStrictEqual(o.completion.selector, { language: 'tr' });
});

suite.run();
