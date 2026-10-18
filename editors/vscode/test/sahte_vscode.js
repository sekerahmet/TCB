// Eklentinin (extension.js) Node ile sınanabilmesi için vscode modülünün küçük bir sahtesi:
// yalnızca sınanan yolların kullandığı API'ler vardır ve her çağrı kaydedilir. Gerçek VS Code
// davranışının yerine geçmez; eklentinin VS Code'a NE istediğini (hangi süreç, hangi
// argümanlar, hangi mesaj) sınar. kur() eklentiyi bu sahteyle yükler.
//
// Belgeler ve sekmeler: kayıtlı bir dosya sistemi sağlayıcısının şemasındaki belge sağlayıcıdan
// okunur ve sağlayıcıya kaydedilir (VS Code gibi: BOM metinden ayrılır, kayıtta geri eklenir;
// kayıt belgenin metnine dokunmaz). Diskteki dosyalar diskten okunur. showTextDocument sekme
// açar (önizleme sekmesi yeni önizlemeyle değişir), tabGroups.close kapatır; ikisi de VS Code'un
// olaylarını (onDidChangeTabs, onDidChangeVisibleTextEditors) yayar. Eklentinin kayıtta belgeyi
// değiştirmesi (onWillSaveTextDocument, applyEdit) yasaktır: çağrılırsa sınama kalır.
'use strict';

const Module = require('module');
const fs = require('fs');
const path = require('path');

class Uri {
    constructor(fsPath, scheme) {
        this.fsPath = fsPath;
        this.scheme = scheme || 'file';
    }
    static file(p) { return new Uri(p, 'file'); }
    static joinPath(u, ...parcalar) { return new Uri(path.join(u.fsPath, ...parcalar), u.scheme); }
    with(d) { return new Uri(this.fsPath, d && d.scheme !== undefined ? d.scheme : this.scheme); }
    // VS Code'un biçimi: "file:///c%3A/a/b.tr", "tr-kodlu:/c%3A/a/b.tr".
    toString() {
        let p = this.fsPath.replace(/\\/g, '/');
        let onek = '';
        if (/^[a-zA-Z]:/.test(p)) {
            onek = '/' + p[0].toLowerCase() + '%3A';
            p = p.slice(2);
        }
        return this.scheme + ':' + (this.scheme === 'file' ? '//' : '') + onek + p.split('/').map(encodeURIComponent).join('/');
    }
}

class EventEmitter {
    constructor() {
        this.dinleyiciler = [];
        this.event = (f) => {
            this.dinleyiciler.push(f);
            return { dispose: () => { this.dinleyiciler = this.dinleyiciler.filter((x) => x !== f); } };
        };
    }
    fire(v) { for (const f of [...this.dinleyiciler]) { f(v); } }
    dispose() { this.dinleyiciler = []; }
}

class FileSystemError extends Error {
    constructor(mesaj, code) { super(mesaj); this.code = code; }
    static FileNotFound(u) { return new FileSystemError('bulunamadı: ' + u, 'FileNotFound'); }
    static FileExists(u) { return new FileSystemError('var: ' + u, 'FileExists'); }
    static FileIsADirectory(u) { return new FileSystemError('klasör: ' + u, 'FileIsADirectory'); }
    static FileNotADirectory(u) { return new FileSystemError('klasör değil: ' + u, 'FileNotADirectory'); }
    static NoPermissions(u) { return new FileSystemError('izin yok: ' + u, 'NoPermissions'); }
}

class TabInputText {
    constructor(uri) { this.uri = uri; }
}

class RelativePattern {
    constructor(base, pattern) { this.base = base; this.pattern = pattern; }
}

// Belge VS Code'un metin belgesinin sahtesidir; duzenle ve geriAl düzenleyicide yazmayı ve
// Ctrl+Z'yi taklit eder.
class Belge {
    constructor(uri, bayt, saglayici) {
        const metin = Buffer.from(bayt).toString('utf8');
        this.bom = metin.charCodeAt(0) === 0xfeff;
        this.uri = uri;
        this.fileName = uri.fsPath;
        this.languageId = path.extname(uri.fsPath).toLowerCase() === '.tr' ? 'tr' : 'plaintext';
        this.isUntitled = false;
        this.isDirty = false;
        this.metin = this.bom ? metin.slice(1) : metin;
        this.gecmis = [];
        this.saglayici = saglayici;
    }
    getText() { return this.metin; }
    get lineCount() { return this.metin.split(/\r?\n/).length; }
    lineAt(i) { return { text: this.metin.split(/\r?\n/)[i] }; }
    duzenle(yeni) { this.gecmis.push(this.metin); this.metin = yeni; this.isDirty = true; }
    geriAl() { this.metin = this.gecmis.pop(); this.isDirty = true; }
    async save() {
        if (this.saglayici.salt) {
            return false;
        }
        const bayt = Buffer.from((this.bom ? '﻿' : '') + this.metin, 'utf8');
        await this.saglayici.p.writeFile(this.uri, bayt, { create: true, overwrite: true });
        this.isDirty = false;
        return true;
    }
}

// diskSaglayici diskteki dosyaların ("file" şeması) okuma ve yazmasıdır.
const diskSaglayici = {
    p: {
        readFile: async (uri) => fs.promises.readFile(uri.fsPath),
        writeFile: async (uri, bayt) => fs.promises.writeFile(uri.fsPath, bayt)
    }
};

// SahteDurum ExtensionContext.globalState'in sahtesidir; aynı nesne iki kuruluma verilirse
// "VS Code yeniden açıldı" taklit edilir.
class SahteDurum {
    constructor() { this.degerler = {}; }
    get(ad, varsayilan) { return ad in this.degerler ? this.degerler[ad] : varsayilan; }
    async update(ad, deger) { this.degerler[ad] = deger; }
}

// sahte bir VS Code ortamı kurar ve eklentiyi etkinleştirir.
//   s.ayar        tr.yorumlayiciYolu
//   s.ayarlar     başka 'tr.' ayarları ({ yazimDili: 'tr' })
//   s.klasor      çalışma alanı klasörü (yoksa klasörsüz pencere)
//   s.guvenilir   çalışma alanı güvenilir mi (varsayılan doğru)
//   s.etkinDosya  etkin düzenleyicideki .tr dosyası (yoksa düzenleyici kapalı)
//   s.durum       kalıcı durum (SahteDurum; yoksa yenisi)
// Döner: { vscode, kayit, saglayici, surec, komutlar, dosyaSistemleri, durum, etkinlesme }
function kur(s) {
    const kayit = { gorevler: [], terminaller: [], hatalar: [], uyarilar: [], bilgiler: [], komutlar: [], izleyiciler: [], hataAyiklamalar: [] };
    const belge = (p) => ({ uri: Uri.file(p), fileName: p, languageId: 'tr', isUntitled: false, isDirty: false, save: async () => true });
    const klasor = s.klasor ? { uri: Uri.file(s.klasor), name: path.basename(s.klasor), index: 0 } : undefined;
    const kayitlar = { saglayicilar: {}, komutlar: {}, dosyaSistemleri: {} };
    const olaylar = { sekmeler: new EventEmitter(), gorunenler: new EventEmitter(), secim: new EventEmitter(), guven: new EventEmitter(), ayar: new EventEmitter() };
    const yasak = (ad) => () => { throw new Error(ad + ' kullanılmamalı: kayıtta belgenin metnine dokunulmaz'); };

    const gruplar = [];
    const window = {};
    function grupBul(sutun) {
        let g = gruplar.find((x) => x.viewColumn === sutun);
        if (!g) {
            g = { viewColumn: sutun, isActive: false, tabs: [] };
            gruplar.push(g);
        }
        return g;
    }
    function etkinGrup(g) {
        for (const x of gruplar) { x.isActive = x === g; }
    }
    async function belgeAc(uri) {
        const acik = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
        if (acik) {
            return acik;
        }
        const ds = kayitlar.dosyaSistemleri[uri.scheme];
        const saglayici = ds ? { p: ds.p, salt: !!(ds.secenekler && ds.secenekler.isReadonly) } : diskSaglayici;
        if (!ds && (uri.scheme !== 'file' || !fs.existsSync(uri.fsPath))) {
            return belge(uri.fsPath);
        }
        const b = new Belge(uri, await saglayici.p.readFile(uri), saglayici);
        vscode.workspace.textDocuments.push(b);
        return b;
    }
    function gorunenleriYay() {
        olaylar.gorunenler.fire(window.visibleTextEditors);
    }

    const vscode = {
        Uri, EventEmitter, FileSystemError, TabInputText, RelativePattern,
        Disposable: class { constructor(f) { this.dispose = f; } },
        FileType: { File: 1, Directory: 2 },
        FileChangeType: { Changed: 1, Created: 2, Deleted: 3 },
        ViewColumn: { One: 1, Beside: -2 },
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
            // Çalışma alanı klasörü yalnızca diskteki dosyaların klasörüdür (VS Code gibi).
            getWorkspaceFolder: (uri) => (uri && uri.scheme === 'file' ? klasor : undefined),
            getConfiguration: () => ({
                get: (ad, varsayilan) => {
                    if (ad === 'yorumlayiciYolu') {
                        return s.ayar;
                    }
                    return s.ayarlar && ad in s.ayarlar ? s.ayarlar[ad] : varsayilan;
                }
            }),
            openTextDocument: belgeAc,
            registerFileSystemProvider: (sema, p, secenekler) => {
                kayitlar.dosyaSistemleri[sema] = { p, secenekler };
                return { dispose() {} };
            },
            createFileSystemWatcher: (desen) => {
                const iz = { desen, degisti: new EventEmitter(), olustu: new EventEmitter(), silindi: new EventEmitter(), dispose() { iz.kapandi = true; } };
                iz.onDidChange = iz.degisti.event;
                iz.onDidCreate = iz.olustu.event;
                iz.onDidDelete = iz.silindi.event;
                kayit.izleyiciler.push(iz);
                return iz;
            },
            onDidGrantWorkspaceTrust: olaylar.guven.event,
            onDidChangeConfiguration: olaylar.ayar.event,
            onWillSaveTextDocument: yasak('onWillSaveTextDocument'),
            applyEdit: yasak('applyEdit')
        },
        window: Object.assign(window, {
            activeTextEditor: s.etkinDosya ? { document: belge(s.etkinDosya) } : undefined,
            visibleTextEditors: [],
            showErrorMessage: async (m) => { kayit.hatalar.push(m); },
            showWarningMessage: async (m) => { kayit.uyarilar.push(m); },
            showInformationMessage: async (m) => { kayit.bilgiler.push(m); },
            createTerminal: (secenekler) => { kayit.terminaller.push(secenekler); return { show() {} }; },
            onDidChangeVisibleTextEditors: olaylar.gorunenler.event,
            onDidChangeTextEditorSelection: olaylar.secim.event,
            // showTextDocument belgeyi bir sekmede açar (VS Code gibi: önizleme sekmesi yeni
            // önizlemeyle değişir; açık belgenin sekmesi öne gelir).
            showTextDocument: async (hedef, secenek) => {
                const o = secenek || {};
                const b = hedef instanceof Uri ? await belgeAc(hedef) : hedef;
                const sutun = o.viewColumn === -2 ? Math.max(0, ...gruplar.map((x) => x.viewColumn)) + 1 : (o.viewColumn || 1);
                const g = grupBul(sutun);
                let sekme = g.tabs.find((t) => t.input.uri.toString() === b.uri.toString());
                const kapanan = [];
                const yeni = !sekme;
                if (yeni) {
                    const onizleme = o.preview !== false;
                    const eski = onizleme ? g.tabs.find((t) => t.isPreview) : undefined;
                    if (eski) {
                        g.tabs.splice(g.tabs.indexOf(eski), 1);
                        kapanan.push(eski);
                    }
                    sekme = { input: new TabInputText(b.uri), group: g, isPreview: onizleme, isActive: false };
                    g.tabs.push(sekme);
                }
                for (const t of g.tabs) { t.isActive = t === sekme; }
                const editor = { document: b, viewColumn: sutun, selection: o.selection || { start: { line: 0, character: 0 }, end: { line: 0, character: 0 }, isEmpty: true } };
                window.visibleTextEditors = window.visibleTextEditors.filter((e) => e.viewColumn !== sutun).concat([editor]);
                if (!o.preserveFocus) {
                    window.activeTextEditor = editor;
                    etkinGrup(g);
                }
                olaylar.sekmeler.fire({ opened: yeni ? [sekme] : [], closed: kapanan, changed: yeni ? [] : [sekme] });
                gorunenleriYay();
                return editor;
            },
            tabGroups: {
                get all() { return gruplar; },
                onDidChangeTabs: olaylar.sekmeler.event,
                close: async (sekme) => {
                    const g = sekme.group;
                    const i = g.tabs.indexOf(sekme);
                    if (i < 0) {
                        return false;
                    }
                    g.tabs.splice(i, 1);
                    const editor = window.visibleTextEditors.find((e) => e.viewColumn === g.viewColumn);
                    if (editor && editor.document.uri.toString() === sekme.input.uri.toString()) {
                        window.visibleTextEditors = window.visibleTextEditors.filter((e) => e !== editor);
                        if (window.activeTextEditor === editor) {
                            window.activeTextEditor = undefined;
                        }
                    }
                    olaylar.sekmeler.fire({ opened: [], closed: [sekme], changed: [] });
                    gorunenleriYay();
                    return true;
                }
            }
        }),
        tasks: {
            executeTask: async (gorev) => { kayit.gorevler.push(gorev); },
            registerTaskProvider: () => ({ dispose() {} })
        },
        commands: {
            registerCommand: (ad, fn) => { kayitlar.komutlar[ad] = fn; return { dispose() {} }; },
            executeCommand: async (ad) => { kayit.komutlar.push(ad); }
        },
        debug: {
            registerDebugConfigurationProvider: (tur, p) => { kayitlar.saglayicilar.yapilandirma = p; return { dispose() {} }; },
            registerDebugAdapterDescriptorFactory: (tur, f) => { kayitlar.saglayicilar.surec = f; return { dispose() {} }; },
            registerDebugAdapterTrackerFactory: () => ({ dispose() {} }),
            startDebugging: async (k, y) => { kayit.hataAyiklamalar.push(y); return true; },
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
    const durum = s.durum || new SahteDurum();
    const context = { subscriptions: [], extensionUri: Uri.file(path.join(__dirname, '..')), globalState: durum };
    eklenti.activate(context);
    return {
        vscode, kayit, durum, olaylar,
        saglayici: kayitlar.saglayicilar.yapilandirma, surec: kayitlar.saglayicilar.surec,
        komutlar: kayitlar.komutlar, dosyaSistemleri: kayitlar.dosyaSistemleri
    };
}

// bekle bekleyen işlerin (calistir'in await zinciri) bitmesine izin verir.
function bekle() {
    return new Promise((coz) => setImmediate(coz));
}

// olana kadar koşul doğru olana kadar bekler (süreç ve dosya işleri gerçek zamanlıdır); süre
// dolarsa hata verir.
async function olana(kosul, ne, sure) {
    const son = Date.now() + (sure || 15000);
    while (!(await kosul())) {
        if (Date.now() > son) {
            throw new Error('beklenen durum oluşmadı: ' + ne);
        }
        await new Promise((coz) => setTimeout(coz, 10));
    }
}

module.exports = { kur, bekle, olana, Uri, SahteDurum };
