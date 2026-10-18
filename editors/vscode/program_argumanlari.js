// Hata ayıklama yapılandırmasındaki (launch.json) "args" alanı: programa iletilen komut satırı
// argümanları. F5'te 'tr dap' bunları başlatma isteğinden okur; Ctrl+F5'te (hata ayıklamadan
// çalıştırma) yorumlayıcıya dosyanın ardından verilir ('tr dosya.tr --adım 5'). vscode
// modülüne bağlı değildir; böylece Node ile ayrıca sınanır (test/program_argumanlari_test.js).
'use strict';

// programArgumanlari yapılandırmanın "args" alanını denetler.
// Döner: { argumanlar } (alan yoksa boş liste) | { hata } (alan bir metin dizisi değilse).
function programArgumanlari(yapilandirma) {
    const a = yapilandirma ? yapilandirma.args : undefined;
    if (a === undefined || a === null) {
        return { argumanlar: [] };
    }
    if (!Array.isArray(a) || a.some((x) => typeof x !== 'string')) {
        return { hata: 'Hata ayıklama ayarındaki "args" bir metin dizisi olmalı (ör. "args": ["--adım", "5"]).' };
    }
    return { argumanlar: a.slice() };
}

module.exports = { programArgumanlari };
