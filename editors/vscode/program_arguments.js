// Hata ayıklama yapılandırmasındaki (launch.json) "args" alanı: programa iletilen komut satırı
// argümanları. F5'te 'tr dap' bunları başlatma isteğinden okur; Ctrl+F5'te (hata ayıklamadan
// çalıştırma) yorumlayıcıya dosyanın ardından verilir ('tr dosya.tr --adım 5'). vscode
// modülüne bağlı değildir; böylece Node ile ayrıca sınanır (test/program_arguments_test.js).
'use strict';

const { t } = require('./l10n');

// programArguments yapılandırmanın "args" alanını denetler.
// Döner: { args } (alan yoksa boş liste) | { error } (alan bir metin dizisi değilse).
function programArguments(config) {
    const a = config ? config.args : undefined;
    if (a === undefined || a === null) {
        return { args: [] };
    }
    if (!Array.isArray(a) || a.some((x) => typeof x !== 'string')) {
        return { error: t('The "args" in the debug configuration must be an array of strings (e.g. "args": ["--step", "5"]).') };
    }
    return { args: a.slice() };
}

module.exports = { programArguments };
