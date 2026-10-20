// Yorumlayıcıyı (tr.exe) bulma kuralı. vscode modülüne bağlı değildir; böylece kural Node ile
// ayrıca sınanır (test/find_interpreter_test.js).
//
// Sıra:
//   1. 'tr.yorumlayiciYolu' ayarı (kapsamı "machine": yalnızca kullanıcı ayarından okunur, bir
//      çalışma alanının .vscode/settings.json dosyası yorumlayıcıyı başka bir programa çeviremez).
//      Göreli yol yalnızca güvenilen bir çalışma alanı klasörüne göre çözülür.
//   2. Yalnızca GÜVENİLEN çalışma alanında, dosyanın çalışma alanı klasörünün KÖKÜNDE
//      bin/tr.exe, sonra tr.exe. Üst klasörler taranmaz: indirilmiş bir klasördeki ya da
//      geçici klasördeki yabancı bir tr.exe kendiliğinden çalıştırılmaz.
//   3. PATH (Unix'te /bin ve /usr/bin atlanır: oradaki 'tr' karakter çevirme aracıdır).
// Klasörsüz pencerede (tek dosya) yalnızca 1 ve 3 kullanılır.
'use strict';

const path = require('path');

const BIN_FOLDER = 'bin';

function interpreterName(platform) {
    return platform === 'win32' ? 'tr.exe' : 'tr';
}

// findInterpreter yorumlayıcının yolunu bulur.
//   g.setting     'tr.yorumlayiciYolu' ayarının değeri ('' ya da undefined: ayar yok)
//   g.folder      dosyanın çalışma alanı klasörünün yolu (klasörsüz pencerede undefined)
//   g.trusted     çalışma alanı güvenilir mi
//   g.fileExists  filePath -> bool
//   g.pathValue   PATH ortam değişkeni
//   g.platform    process.platform
// Döner: { filePath } | { error } | undefined (bulunamadı).
function findInterpreter(g) {
    const platform = g.platform || process.platform;
    const pathMod = platform === 'win32' ? path.win32 : path.posix;
    const name = interpreterName(platform);
    const setting = (g.setting || '').trim();

    if (setting) {
        let filePath = setting;
        if (!pathMod.isAbsolute(filePath)) {
            if (!g.folder || !g.trusted) {
                return { error: `'tr.yorumlayiciYolu' ayarı göreli bir yol (${setting}); göreli yol yalnızca güvenilen bir çalışma alanı klasöründe kullanılabilir. Yorumlayıcının tam yolunu yazın.` };
            }
            filePath = pathMod.resolve(g.folder, filePath);
        }
        if (g.fileExists(filePath)) {
            return { filePath };
        }
        return { error: `'tr.yorumlayiciYolu' ayarında belirtilen yorumlayıcı bulunamadı: ${filePath}` };
    }

    if (g.folder && g.trusted) {
        for (const candidate of [pathMod.join(g.folder, BIN_FOLDER, name), pathMod.join(g.folder, name)]) {
            if (g.fileExists(candidate)) {
                return { filePath: candidate };
            }
        }
    }

    const separator = platform === 'win32' ? ';' : ':';
    for (const dir of (g.pathValue || '').split(separator).filter(Boolean)) {
        if (platform !== 'win32' && (dir === '/bin' || dir === '/usr/bin')) {
            continue;
        }
        const candidate = pathMod.join(dir, name);
        if (g.fileExists(candidate)) {
            return { filePath: candidate };
        }
    }
    return undefined;
}

module.exports = { findInterpreter, interpreterName, BIN_FOLDER };
