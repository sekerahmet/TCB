// Eklentinin iletileri: anahtar, İngilizce iletinin kendisidir ({0}, {1} yer tutucularıyla); Türkçesi
// l10n/bundle.l10n.tr.json'dadır. VS Code içinde vscode.l10n.t kullanılır (kişinin gösterim diline
// göre seçer); vscode modülü olmayan Node sınamalarında aynı biçimlendirme yapılır, böylece
// vscode'a bağlı olmayan dosyalar (find_interpreter.js, view.js ...) da bunu kullanır.
// Birinci bağımsız değişken tek bir metin sabiti olmalıdır: test/l10n_test.js anahtarları kaynaktan
// okur ve pakete eksiksiz girdiğini denetler.
'use strict';

// override sınamalarda sahte vscode.l10n'dir (use ile verilir).
let override;

function use(api) {
    override = api;
}

function vscodeApi() {
    if (override) {
        return override;
    }
    try {
        return require('vscode').l10n;
    } catch (e) {
        return undefined;
    }
}

function t(message, ...args) {
    const api = vscodeApi();
    if (api) {
        return api.t(message, ...args);
    }
    return message.replace(/\{(\d+)\}/g, (match, index) => (Number(index) < args.length ? String(args[index]) : match));
}

module.exports = { t, use };
