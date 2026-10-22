// İki web görünümünün (media/table.js ve media/watch_panel.js) ortak tablo mantığının testi
// (media/table_common.js: isNumber, tableRows). Bağımlılıksız; depo kökünden:
//   node editors/vscode/test/table_common_test.js
// (go test ./cmd/tr/e2e bunu TestExtensionJSTests ile çalıştırır.)
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const { createSuite } = require('./fake_vscode');

const context = {};
vm.createContext(context);
vm.runInContext(fs.readFileSync(path.join(__dirname, '..', 'media', 'table_common.js'), 'utf8'), context, { filename: 'table_common.js' });
const { isNumber, tableRows } = vm.runInContext('({ isNumber, tableRows })', context);

const suite = createSuite('table_common_test');
const test = suite.test;

test('isNumber boş metni ve null\'ı sayı saymaz', () => {
    assert.strictEqual(isNumber('12'), true);
    assert.strictEqual(isNumber('1.5'), true);
    assert.strictEqual(isNumber(''), false);
    assert.strictEqual(isNumber(null), false);
    assert.strictEqual(isNumber('abc'), false);
});

test('süzme Türkçe büyük/küçük harfe duyarsızdır (I/ı, İ/i)', () => {
    const rows = [['0', 'IŞIK'], ['1', 'İşler'], ['2', 'kapı']];
    assert.deepStrictEqual(tableRows(rows, 'ışık', { column: -1, descending: false }).map((r) => r[0]), ['0']);
    assert.deepStrictEqual(tableRows(rows, 'işler', { column: -1, descending: false }).map((r) => r[0]), ['1']);
    assert.strictEqual(tableRows(rows, '', { column: -1, descending: false }).length, 3);
});

test('sıralama sayısal hücrelerde sayı, öbürlerinde Türkçe sıra; azalan tersine çevirir', () => {
    const rows = [['10'], ['9'], ['100']];
    assert.deepStrictEqual(tableRows(rows, '', { column: 0, descending: false }).map((r) => r[0]), ['9', '10', '100']);
    assert.deepStrictEqual(tableRows(rows, '', { column: 0, descending: true }).map((r) => r[0]), ['100', '10', '9']);
    const words = [['çay'], ['cam'], ['dal']];
    assert.deepStrictEqual(tableRows(words, '', { column: 0, descending: false }).map((r) => r[0]), ['cam', 'çay', 'dal']);
    assert.deepStrictEqual(words.map((r) => r[0]), ['çay', 'cam', 'dal'], 'girdi dizisi değişmemeli');
});

suite.run();
