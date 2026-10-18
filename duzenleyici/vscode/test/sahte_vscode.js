// Eklentinin (extension.js) Node ile sınanabilmesi için vscode modülünün küçük bir sahtesi:
// yalnızca sınanan yolların kullandığı API'ler vardır ve her çağrı kaydedilir. Gerçek VS Code
// davranışının yerine geçmez; eklentinin VS Code'a NE istediğini (hangi süreç, hangi
// argümanlar, hangi mesaj) sınar. kur() eklentiyi bu sahteyle yükler.
'use strict';

const Module = require('module');
const path = require('path');

class Uri {
    constructor(fsPath, scheme) {
        this.fsPath = fsPath;
        this.scheme = scheme || 'file';
    }
    static file(p) { return new Uri(p, 'file'); }
    static joinPath(u, ...parcalar) { return new Uri(path.join(u.fsPath, ...parcalar)); }
    toString() { return this.scheme + '://' + this.fsPath; }
}

// sahte bir VS Code ortamı kurar ve eklentiyi etkinleştirir.
//   s.ayar        tr.yorumlayiciYolu
//   s.klasor      çalışma alanı klasörü (yoksa klasörsüz pencere)
//   s.guvenilir   çalışma alanı güvenilir mi (varsayılan doğru)
//   s.etkinDosya  etkin düzenleyicideki .tr dosyası (yoksa düzenleyici kapalı)
// Döner: { vscode, kayit, saglayici, komutlar }
function kur(s) {
    const kayit = { gorevler: [], terminaller: [], hatalar: [], uyarilar: [] };
    const belge = (p) => ({ uri: Uri.file(p), fileName: p, languageId: 'tr', isUntitled: false, isDirty: false, save: async () => true });
    const klasor = s.klasor ? { uri: Uri.file(s.klasor), name: path.basename(s.klasor), index: 0 } : undefined;
    const kayitlar = { saglayicilar: {}, komutlar: {} };
    const vscode = {
        Uri,
        TaskScope: { Workspace: 'workspace' },
        TaskRevealKind: { Always: 1 },
        TaskPanelKind: { Dedicated: 2 },
        Task: class {
            constructor(tanim, kapsam, ad, kaynak, yurutme) {
                Object.assign(this, { tanim, kapsam, ad, kaynak, execution: yurutme });
            }
        },
        ProcessExecution: class {
            constructor(surec, argumanlar, secenekler) {
                Object.assign(this, { surec, argumanlar, secenekler });
            }
        },
        DebugAdapterExecutable: class {
            constructor(komut, argumanlar, secenekler) {
                Object.assign(this, { komut, argumanlar, secenekler });
            }
        },
        workspace: {
            isTrusted: s.guvenilir !== false,
            textDocuments: [],
            getWorkspaceFolder: () => klasor,
            getConfiguration: () => ({ get: (ad) => (ad === 'yorumlayiciYolu' ? s.ayar : undefined) }),
            openTextDocument: async (uri) => belge(uri.fsPath)
        },
        window: {
            activeTextEditor: s.etkinDosya ? { document: belge(s.etkinDosya) } : undefined,
            showErrorMessage: async (m) => { kayit.hatalar.push(m); },
            showWarningMessage: async (m) => { kayit.uyarilar.push(m); },
            createTerminal: (secenekler) => { kayit.terminaller.push(secenekler); return { show() {} }; }
        },
        tasks: {
            executeTask: async (gorev) => { kayit.gorevler.push(gorev); },
            registerTaskProvider: () => ({ dispose() {} })
        },
        commands: {
            registerCommand: (ad, fn) => { kayitlar.komutlar[ad] = fn; return { dispose() {} }; },
            executeCommand: async () => {}
        },
        debug: {
            registerDebugConfigurationProvider: (tur, p) => { kayitlar.saglayicilar.yapilandirma = p; return { dispose() {} }; },
            registerDebugAdapterDescriptorFactory: (tur, f) => { kayitlar.saglayicilar.surec = f; return { dispose() {} }; },
            registerDebugAdapterTrackerFactory: () => ({ dispose() {} }),
            activeDebugSession: undefined
        }
    };

    // extension.js'nin require('vscode') çağrısı bu sahteye gider; her kurulum eklentiyi yeniden yükler.
    const eklentiYolu = require.resolve('../extension');
    delete require.cache[eklentiYolu];
    const asilCoz = Module._resolveFilename;
    Module._resolveFilename = function (istek, ...geri) {
        return istek === 'vscode' ? 'vscode' : asilCoz.call(this, istek, ...geri);
    };
    require.cache.vscode = { id: 'vscode', filename: 'vscode', loaded: true, exports: vscode };
    let eklenti;
    try {
        eklenti = require(eklentiYolu);
    } finally {
        Module._resolveFilename = asilCoz;
        delete require.cache.vscode;
    }
    eklenti.activate({ subscriptions: [], extensionUri: Uri.file(path.join(__dirname, '..')) });
    return { vscode, kayit, saglayici: kayitlar.saglayicilar.yapilandirma, surec: kayitlar.saglayicilar.surec, komutlar: kayitlar.komutlar };
}

// bekle bekleyen işlerin (calistir'in await zinciri) bitmesine izin verir.
function bekle() {
    return new Promise((coz) => setImmediate(coz));
}

module.exports = { kur, bekle, Uri };
