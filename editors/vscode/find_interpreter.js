// Yorumlayıcıyı (tr.exe) bulma kuralı. vscode modülüne bağlı değildir; böylece kural Node ile
// ayrıca sınanır (test/yorumlayici_bul_test.js).
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

const BIN_KLASORU = 'bin';

function yorumlayiciAdi(platform) {
    return platform === 'win32' ? 'tr.exe' : 'tr';
}

// yorumlayiciyiBul yorumlayıcının yolunu bulur.
//   g.ayar        'tr.yorumlayiciYolu' ayarının değeri ('' ya da undefined: ayar yok)
//   g.klasor      dosyanın çalışma alanı klasörünün yolu (klasörsüz pencerede undefined)
//   g.guvenilir   çalışma alanı güvenilir mi
//   g.dosyaVarMi  yol -> bool
//   g.pathDegeri  PATH ortam değişkeni
//   g.platform    process.platform
// Döner: { yol } | { hata } | undefined (bulunamadı).
function yorumlayiciyiBul(g) {
    const platform = g.platform || process.platform;
    const yolMod = platform === 'win32' ? path.win32 : path.posix;
    const ad = yorumlayiciAdi(platform);
    const ayar = (g.ayar || '').trim();

    if (ayar) {
        let yol = ayar;
        if (!yolMod.isAbsolute(yol)) {
            if (!g.klasor || !g.guvenilir) {
                return { hata: `'tr.yorumlayiciYolu' ayarı göreli bir yol (${ayar}); göreli yol yalnızca güvenilen bir çalışma alanı klasöründe kullanılabilir. Yorumlayıcının tam yolunu yazın.` };
            }
            yol = yolMod.resolve(g.klasor, yol);
        }
        if (g.dosyaVarMi(yol)) {
            return { yol };
        }
        return { hata: `'tr.yorumlayiciYolu' ayarında belirtilen yorumlayıcı bulunamadı: ${yol}` };
    }

    if (g.klasor && g.guvenilir) {
        for (const aday of [yolMod.join(g.klasor, BIN_KLASORU, ad), yolMod.join(g.klasor, ad)]) {
            if (g.dosyaVarMi(aday)) {
                return { yol: aday };
            }
        }
    }

    const ayirici = platform === 'win32' ? ';' : ':';
    for (const dizin of (g.pathDegeri || '').split(ayirici).filter(Boolean)) {
        if (platform !== 'win32' && (dizin === '/bin' || dizin === '/usr/bin')) {
            continue;
        }
        const aday = yolMod.join(dizin, ad);
        if (g.dosyaVarMi(aday)) {
            return { yol: aday };
        }
    }
    return undefined;
}

module.exports = { yorumlayiciyiBul, yorumlayiciAdi, BIN_KLASORU };
