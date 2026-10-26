// Kodlu görünümün dosya sistemi sağlayıcıları (coded_view.js). İki şema vardır:
//   görünüm  (g.VIEW_SCHEME)  okuma g.readView'dan (kodluysa Türkçe yazım), yazma
//                                g.saveText'ten (kodlu biçim) geçer;
//   ham      (g.RAW_SCHEME)      dosyanın baytları, salt okunur (kısıtlı kip ve "kodlu hâlini
//                                göster" komutu).
// Adresin yolu diskteki dosyanın yoludur (g.localPath). Tarih ve boyut diskteki dosyanınkidir;
// böylece dosya dışarıda değişince VS Code "dosya diskte değişti" durumunu kendisi tanır.
// Değişiklikler VS Code'un kendi dosya izleyicisiyle izlenir ve görünümün adresiyle bildirilir.
'use strict';

const fs = require('fs');
const path = require('path');

function create(vscode, g) {
    function fileError(e, uri) {
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
    // wrap işi çalıştırır; Node'un dosya hatalarını VS Code'un dosya sistemi hatalarına çevirir.
    async function wrap(uri, job) {
        try {
            return await job();
        } catch (e) {
            throw fileError(e, uri);
        }
    }

    // stat diskteki dosyanın durumudur. Görünümde mtime'a dosyanın "sürüm"ü eklenir
    // (g.revision): VS Code açık belgeyi yeniden okuyup okumayacağına tarih+boyuta bakar;
    // yazım dili değişince metin değişir ama disk aynı kalır, sürüm artınca belge yeniden okunur.
    function stat(uri) {
        return wrap(uri, async () => {
            const filePath = g.localPath(uri);
            const s = await fs.promises.stat(filePath);
            const revision = uri.scheme === g.VIEW_SCHEME && g.revision ? g.revision(filePath) : 0;
            return {
                type: s.isDirectory() ? vscode.FileType.Directory : vscode.FileType.File,
                ctime: Math.trunc(s.ctimeMs),
                mtime: Math.trunc(s.mtimeMs) + revision,
                size: s.size
            };
        });
    }

    function readDirectory(uri) {
        return wrap(uri, async () => (await fs.promises.readdir(g.localPath(uri), { withFileTypes: true }))
            .map((d) => [d.name, d.isDirectory() ? vscode.FileType.Directory : vscode.FileType.File]));
    }

    // watchFile diskteki dosyayı (ya da klasörü) VS Code'un dosya izleyicisiyle izler; olaylar
    // izlenen adresin şemasıyla bildirilir.
    function watchFile(emitter, uri, options) {
        const filePath = g.localPath(uri);
        let folder = path.dirname(filePath);
        let pattern = globEscape(path.basename(filePath));
        try {
            if (fs.statSync(filePath).isDirectory()) {
                folder = filePath;
                pattern = options && options.recursive ? '**' : '*';
            }
        } catch (e) {
            // dosya henüz yok: oluşturulunca bildirilir
        }
        const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(vscode.Uri.file(folder), pattern));
        const forward = (kind) => (u) => emitter.fire([{ type: kind, uri: u.with({ scheme: uri.scheme }) }]);
        watcher.onDidChange(forward(vscode.FileChangeType.Changed));
        watcher.onDidCreate(forward(vscode.FileChangeType.Created));
        watcher.onDidDelete(forward(vscode.FileChangeType.Deleted));
        return watcher;
    }

    // Kısıtlı kipte (güvenilmeyen çalışma alanı) görünüm şeması da salt okunurdur: tr-coded:
    // adresi elle verilse bile yerel diske yazılmaz, silinmez, taşınmaz.
    const writable = (fn) => (uri, ...rest) => {
        if (!vscode.workspace.isTrusted) {
            throw vscode.FileSystemError.NoPermissions(uri);
        }
        return fn(uri, ...rest);
    };

    const viewEmitter = new vscode.EventEmitter();
    const view = {
        onDidChangeFile: viewEmitter.event,
        watch: (uri, options) => watchFile(viewEmitter, uri, options),
        stat,
        readDirectory,
        createDirectory: writable((uri) => wrap(uri, () => fs.promises.mkdir(g.localPath(uri)))),
        readFile: (uri) => wrap(uri, async () => {
            const filePath = g.localPath(uri);
            return g.readView(filePath, await fs.promises.readFile(filePath));
        }),
        writeFile: writable((uri, content, options) => wrap(uri, async () => {
            const filePath = g.localPath(uri);
            let disk;
            try {
                disk = await fs.promises.readFile(filePath);
            } catch (e) {
                if (e.code !== 'ENOENT') {
                    throw e;
                }
            }
            if (!disk && !options.create) {
                throw vscode.FileSystemError.FileNotFound(uri);
            }
            if (disk && !options.overwrite) {
                throw vscode.FileSystemError.FileExists(uri);
            }
            await fs.promises.writeFile(filePath, await g.saveText(filePath, content, disk));
        })),
        delete: writable((uri, options) => wrap(uri, () => fs.promises.rm(g.localPath(uri), { recursive: !!(options && options.recursive) }))),
        rename: writable((oldUri, newUri, options) => wrap(oldUri, async () => {
            const target = g.localPath(newUri);
            if (!(options && options.overwrite) && fs.existsSync(target)) {
                throw vscode.FileSystemError.FileExists(newUri);
            }
            await fs.promises.rename(g.localPath(oldUri), target);
        }))
    };

    const rawEmitter = new vscode.EventEmitter();
    const readOnly = async (uri) => {
        throw vscode.FileSystemError.NoPermissions(uri);
    };
    const raw = {
        onDidChangeFile: rawEmitter.event,
        watch: (uri, options) => watchFile(rawEmitter, uri, options),
        stat,
        readDirectory,
        readFile: (uri) => wrap(uri, () => fs.promises.readFile(g.localPath(uri))),
        writeFile: readOnly,
        createDirectory: readOnly,
        delete: readOnly,
        rename: readOnly
    };

    // notifyChanged görünümün adresini "değişti" diye bildirir (belge yeniden okunur).
    function notifyChanged(uri) {
        viewEmitter.fire([{ type: vscode.FileChangeType.Changed, uri }]);
    }

    return { view, raw, notifyChanged };
}

// globEscape bir dosya adını izleme deseninde kendisiyle eşleşecek biçime getirir: desen
// karakterleri tek karakterlik joker olur (en kötü durumda benzer adlı bir dosyanın olayı da
// gelir; zararsızdır).
function globEscape(name) {
    return name.replace(/[[\]{}*?!]/g, '?');
}

module.exports = { create, globEscape };
