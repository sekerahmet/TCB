// Kodlu görünümün dosya sistemi sağlayıcıları (coded_view.js). İki şema vardır:
//   görünüm  (g.GORUNUM_SEMASI)  okuma g.gorunumOku'dan (kodluysa Türkçe yazım), yazma
//                                g.kayitMetni'nden (kodlu biçim) geçer;
//   ham      (g.HAM_SEMASI)      dosyanın baytları, salt okunur (kısıtlı kip ve "kodlu hâlini
//                                göster" komutu).
// Adresin yolu diskteki dosyanın yoludur (g.yerelYol). Tarih ve boyut diskteki dosyanınkidir;
// böylece dosya dışarıda değişince VS Code "dosya diskte değişti" durumunu kendisi tanır.
// Değişiklikler VS Code'un kendi dosya izleyicisiyle izlenir ve görünümün adresiyle bildirilir.
'use strict';

const fs = require('fs');
const path = require('path');

function olustur(vscode, g) {
    function dosyaHatasi(e, uri) {
        switch (e && e.code) {
            case 'ENOENT':
                return vscode.FileSystemError.FileNotFound(uri);
            case 'EEXIST':
                return vscode.FileSystemError.FileExists(uri);
            case 'EISDIR':
                return vscode.FileSystemError.FileIsADirectory(uri);
            case 'ENOTDIR':
                return vscode.FileSystemError.FileNotADirectory(uri);
            case 'EACCES':
            case 'EPERM':
                return vscode.FileSystemError.NoPermissions(uri);
            default:
                return e;
        }
    }
    // sar işi çalıştırır; Node'un dosya hatalarını VS Code'un dosya sistemi hatalarına çevirir.
    async function sar(uri, is) {
        try {
            return await is();
        } catch (e) {
            throw dosyaHatasi(e, uri);
        }
    }

    function stat(uri) {
        return sar(uri, async () => {
            const s = await fs.promises.stat(g.yerelYol(uri));
            return {
                type: s.isDirectory() ? vscode.FileType.Directory : vscode.FileType.File,
                ctime: Math.trunc(s.ctimeMs),
                mtime: Math.trunc(s.mtimeMs),
                size: s.size
            };
        });
    }

    function readDirectory(uri) {
        return sar(uri, async () => (await fs.promises.readdir(g.yerelYol(uri), { withFileTypes: true }))
            .map((d) => [d.name, d.isDirectory() ? vscode.FileType.Directory : vscode.FileType.File]));
    }

    // izle diskteki dosyayı (ya da klasörü) VS Code'un dosya izleyicisiyle izler; olaylar
    // izlenen adresin şemasıyla bildirilir.
    function izle(olay, uri, secenek) {
        const yol = g.yerelYol(uri);
        let klasor = path.dirname(yol);
        let desen = globKacis(path.basename(yol));
        try {
            if (fs.statSync(yol).isDirectory()) {
                klasor = yol;
                desen = secenek && secenek.recursive ? '**' : '*';
            }
        } catch (e) {
            // dosya henüz yok: oluşturulunca bildirilir
        }
        const izleyici = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(vscode.Uri.file(klasor), desen));
        const ilet = (tur) => (u) => olay.fire([{ type: tur, uri: u.with({ scheme: uri.scheme }) }]);
        izleyici.onDidChange(ilet(vscode.FileChangeType.Changed));
        izleyici.onDidCreate(ilet(vscode.FileChangeType.Created));
        izleyici.onDidDelete(ilet(vscode.FileChangeType.Deleted));
        return izleyici;
    }

    const gorunumOlay = new vscode.EventEmitter();
    const gorunum = {
        onDidChangeFile: gorunumOlay.event,
        watch: (uri, secenek) => izle(gorunumOlay, uri, secenek),
        stat,
        readDirectory,
        createDirectory: (uri) => sar(uri, () => fs.promises.mkdir(g.yerelYol(uri))),
        readFile: (uri) => sar(uri, async () => {
            const yol = g.yerelYol(uri);
            return g.gorunumOku(yol, await fs.promises.readFile(yol));
        }),
        writeFile: (uri, icerik, secenek) => sar(uri, async () => {
            const yol = g.yerelYol(uri);
            let disk;
            try {
                disk = await fs.promises.readFile(yol);
            } catch (e) {
                if (e.code !== 'ENOENT') {
                    throw e;
                }
            }
            if (!disk && !secenek.create) {
                throw vscode.FileSystemError.FileNotFound(uri);
            }
            if (disk && !secenek.overwrite) {
                throw vscode.FileSystemError.FileExists(uri);
            }
            await fs.promises.writeFile(yol, await g.kayitMetni(yol, icerik, disk));
        }),
        delete: (uri, secenek) => sar(uri, () => fs.promises.rm(g.yerelYol(uri), { recursive: !!(secenek && secenek.recursive) })),
        rename: (eski, yeni, secenek) => sar(eski, async () => {
            const hedef = g.yerelYol(yeni);
            if (!(secenek && secenek.overwrite) && fs.existsSync(hedef)) {
                throw vscode.FileSystemError.FileExists(yeni);
            }
            await fs.promises.rename(g.yerelYol(eski), hedef);
        })
    };

    const hamOlay = new vscode.EventEmitter();
    const saltOkunur = async (uri) => {
        throw vscode.FileSystemError.NoPermissions(uri);
    };
    const ham = {
        onDidChangeFile: hamOlay.event,
        watch: (uri, secenek) => izle(hamOlay, uri, secenek),
        stat,
        readDirectory,
        readFile: (uri) => sar(uri, () => fs.promises.readFile(g.yerelYol(uri))),
        writeFile: saltOkunur,
        createDirectory: saltOkunur,
        delete: saltOkunur,
        rename: saltOkunur
    };

    return { gorunum, ham };
}

// globKacis bir dosya adını izleme deseninde kendisiyle eşleşecek biçime getirir: desen
// karakterleri tek karakterlik joker olur (en kötü durumda benzer adlı bir dosyanın olayı da
// gelir; zararsızdır).
function globKacis(ad) {
    return ad.replace(/[[\]{}*?!]/g, '?');
}

module.exports = { olustur, globKacis };
