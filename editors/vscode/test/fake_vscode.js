// Eklentinin (extension.js) Node ile sınanabilmesi için vscode modülünün küçük bir sahtesi:
// yalnızca sınanan yolların kullandığı API'ler vardır ve her çağrı kaydedilir. Gerçek VS Code
// davranışının yerine geçmez; eklentinin VS Code'a NE istediğini (hangi süreç, hangi
// argümanlar, hangi mesaj) sınar. setup() eklentiyi bu sahteyle yükler.
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
    static joinPath(u, ...parts) { return new Uri(path.join(u.fsPath, ...parts), u.scheme); }
    with(d) { return new Uri(this.fsPath, d && d.scheme !== undefined ? d.scheme : this.scheme); }
    // VS Code'un biçimi: "file:///c%3A/a/b.tr", "tr-kodlu:/c%3A/a/b.tr".
    toString() {
        let p = this.fsPath.replace(/\\/g, '/');
        let prefix = '';
        if (/^[a-zA-Z]:/.test(p)) {
            prefix = '/' + p[0].toLowerCase() + '%3A';
            p = p.slice(2);
        }
        return this.scheme + ':' + (this.scheme === 'file' ? '//' : '') + prefix + p.split('/').map(encodeURIComponent).join('/');
    }
}

class EventEmitter {
    constructor() {
        this.listeners = [];
        this.event = (f) => {
            this.listeners.push(f);
            return { dispose: () => { this.listeners = this.listeners.filter((x) => x !== f); } };
        };
    }
    fire(v) { for (const f of [...this.listeners]) { f(v); } }
    dispose() { this.listeners = []; }
}

class FileSystemError extends Error {
    constructor(message, code) { super(message); this.code = code; }
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

// FakeDocument VS Code'un metin belgesinin sahtesidir; edit ve undo düzenleyicide yazmayı ve
// Ctrl+Z'yi taklit eder.
class FakeDocument {
    constructor(uri, bytes, provider) {
        const text = Buffer.from(bytes).toString('utf8');
        this.bom = text.charCodeAt(0) === 0xfeff;
        this.uri = uri;
        this.fileName = uri.fsPath;
        this.languageId = path.extname(uri.fsPath).toLowerCase() === '.tr' ? 'tr' : 'plaintext';
        this.isUntitled = false;
        this.isDirty = false;
        this.text = this.bom ? text.slice(1) : text;
        this.history = [];
        this.provider = provider;
    }
    getText() { return this.text; }
    get lineCount() { return this.text.split(/\r?\n/).length; }
    lineAt(i) { return { text: this.text.split(/\r?\n/)[i] }; }
    edit(fresh) { this.history.push(this.text); this.text = fresh; this.isDirty = true; }
    undo() { this.text = this.history.pop(); this.isDirty = true; }
    // reload dosyanın tarihi (etag) değiştiyse metni sağlayıcıdan yeniden okur.
    async reload() {
        const etag = (await this.provider.p.stat(this.uri)).mtime;
        if (etag === this.etag) {
            return;
        }
        this.etag = etag;
        const text = Buffer.from(await this.provider.p.readFile(this.uri)).toString('utf8');
        this.bom = text.charCodeAt(0) === 0xfeff;
        this.text = this.bom ? text.slice(1) : text;
        this.history = [];
    }
    async save() {
        if (this.provider.readOnly) {
            return false;
        }
        const bytes = Buffer.from((this.bom ? '﻿' : '') + this.text, 'utf8');
        await this.provider.p.writeFile(this.uri, bytes, { create: true, overwrite: true });
        if (this.provider.p.stat) {
            this.etag = (await this.provider.p.stat(this.uri)).mtime;
        }
        this.isDirty = false;
        return true;
    }
}

// diskProvider diskteki dosyaların ("file" şeması) okuma ve yazmasıdır.
const diskProvider = {
    p: {
        readFile: async (uri) => fs.promises.readFile(uri.fsPath),
        writeFile: async (uri, bytes) => fs.promises.writeFile(uri.fsPath, bytes)
    }
};

// FakeState ExtensionContext.globalState'in sahtesidir; aynı nesne iki kuruluma verilirse
// "VS Code yeniden açıldı" taklit edilir.
class FakeState {
    constructor() { this.values = {}; }
    get(name, defaultValue) { return name in this.values ? this.values[name] : defaultValue; }
    async update(name, value) { this.values[name] = value; }
}

// sahte bir VS Code ortamı kurar ve eklentiyi etkinleştirir.
//   s.setting     tr.yorumlayiciYolu
//   s.settings    başka 'tr.' ayarları ({ yazimDili: 'tr' })
//   s.folder      çalışma alanı klasörü (yoksa klasörsüz pencere)
//   s.trusted     çalışma alanı güvenilir mi (varsayılan doğru)
//   s.activeFile  etkin düzenleyicideki .tr dosyası (yoksa düzenleyici kapalı)
//   s.state       kalıcı durum (FakeState; yoksa yenisi)
// Döner: { vscode, log, provider, adapterFactory, commands, filesystems, state, events }
function setup(s) {
    const log = { tasks: [], terminals: [], errors: [], warnings: [], infos: [], commands: [], watchers: [], debugSessions: [], panels: [] };
    const makeDocument = (p) => ({ uri: Uri.file(p), fileName: p, languageId: 'tr', isUntitled: false, isDirty: false, save: async () => true });
    const folder = s.folder ? { uri: Uri.file(s.folder), name: path.basename(s.folder), index: 0 } : undefined;
    const registries = { providers: {}, commands: {}, filesystems: {} };
    const events = { tabs: new EventEmitter(), visibleEditors: new EventEmitter(), selectionChange: new EventEmitter(), trust: new EventEmitter(), setting: new EventEmitter() };
    const forbidden = (name) => () => { throw new Error(name + ' kullanılmamalı: kayıtta belgenin metnine dokunulmaz'); };

    const groups = [];
    const window = {};
    function findGroup(column) {
        let g = groups.find((x) => x.viewColumn === column);
        if (!g) {
            g = { viewColumn: column, isActive: false, tabs: [] };
            groups.push(g);
        }
        return g;
    }
    function activateGroup(g) {
        for (const x of groups) { x.isActive = x === g; }
    }
    async function openDocument(uri) {
        const existing = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
        if (existing) {
            return existing;
        }
        const fsEntry = registries.filesystems[uri.scheme];
        const provider = fsEntry ? { p: fsEntry.p, readOnly: !!(fsEntry.options && fsEntry.options.isReadonly) } : diskProvider;
        if (!fsEntry && (uri.scheme !== 'file' || !fs.existsSync(uri.fsPath))) {
            return makeDocument(uri.fsPath);
        }
        const b = new FakeDocument(uri, await provider.p.readFile(uri), provider);
        b.etag = provider.p.stat ? (await provider.p.stat(uri)).mtime : undefined;
        vscode.workspace.textDocuments.push(b);
        return b;
    }
    function fireVisibleEditors() {
        events.visibleEditors.fire(window.visibleTextEditors);
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
            constructor(definition, scope, name, source, execution) {
                Object.assign(this, { definition, scope, name, source, execution: execution });
            }
        },
        ProcessExecution: class {
            constructor(process, args, options) {
                Object.assign(this, { process, args, options });
            }
        },
        DebugAdapterExecutable: class {
            constructor(command, args, options) {
                Object.assign(this, { command, args, options });
            }
        },
        workspace: {
            isTrusted: s.trusted !== false,
            textDocuments: [],
            // Çalışma alanı klasörü yalnızca diskteki dosyaların klasörüdür (VS Code gibi).
            getWorkspaceFolder: (uri) => (uri && uri.scheme === 'file' ? folder : undefined),
            getConfiguration: () => ({
                get: (name, defaultValue) => {
                    if (name === 'yorumlayiciYolu') {
                        return s.setting;
                    }
                    return s.settings && name in s.settings ? s.settings[name] : defaultValue;
                }
            }),
            openTextDocument: openDocument,
            registerFileSystemProvider: (scheme, p, options) => {
                registries.filesystems[scheme] = { p, options };
                // VS Code gibi: sağlayıcı "değişti" bildirince kirli olmayan açık belge, dosyanın
                // tarihi (etag) değiştiyse yeniden okunur; kirli belgeye dokunulmaz.
                if (p.onDidChangeFile) {
                    p.onDidChangeFile((changes) => {
                        for (const c of changes) {
                            const doc = vscode.workspace.textDocuments.find((b) => b.uri.toString() === c.uri.toString());
                            if (doc && !doc.isDirty) {
                                doc.reload();
                            }
                        }
                    });
                }
                return { dispose() {} };
            },
            createFileSystemWatcher: (pattern) => {
                const watcher = { pattern, changed: new EventEmitter(), created: new EventEmitter(), deleted: new EventEmitter(), dispose() { watcher.disposed = true; } };
                watcher.onDidChange = watcher.changed.event;
                watcher.onDidCreate = watcher.created.event;
                watcher.onDidDelete = watcher.deleted.event;
                log.watchers.push(watcher);
                return watcher;
            },
            onDidGrantWorkspaceTrust: events.trust.event,
            onDidChangeConfiguration: events.setting.event,
            onWillSaveTextDocument: forbidden('onWillSaveTextDocument'),
            applyEdit: forbidden('applyEdit')
        },
        window: Object.assign(window, {
            activeTextEditor: s.activeFile ? { document: makeDocument(s.activeFile) } : undefined,
            visibleTextEditors: [],
            // createWebviewPanel web görünümünü taklit eder: panele giden iletiler (messages) ve
            // sayfadan gelen iletileri işleyen yöntem (receive) kaydedilir.
            createWebviewPanel: (type, title, column, options) => {
                const panel = { type, title, options, messages: [], disposed: false, reveal() {} };
                panel.webview = {
                    html: '',
                    cspSource: 'csp',
                    asWebviewUri: (u) => u,
                    postMessage: async (m) => { panel.messages.push(m); return true; },
                    onDidReceiveMessage: (f) => { panel.receive = f; return { dispose() {} }; }
                };
                panel.onDidDispose = () => ({ dispose() {} });
                log.panels.push(panel);
                return panel;
            },
            showErrorMessage: async (m) => { log.errors.push(m); },
            showWarningMessage: async (m) => { log.warnings.push(m); },
            showInformationMessage: async (m) => { log.infos.push(m); },
            createTerminal: (options) => { log.terminals.push(options); return { show() {} }; },
            onDidChangeVisibleTextEditors: events.visibleEditors.event,
            onDidChangeTextEditorSelection: events.selectionChange.event,
            // showTextDocument belgeyi bir sekmede açar (VS Code gibi: önizleme sekmesi yeni
            // önizlemeyle değişir; açık belgenin sekmesi öne gelir).
            showTextDocument: async (target, option) => {
                const o = option || {};
                const b = target instanceof Uri ? await openDocument(target) : target;
                const column = o.viewColumn === -2 ? Math.max(0, ...groups.map((x) => x.viewColumn)) + 1 : (o.viewColumn || 1);
                const g = findGroup(column);
                let tab = g.tabs.find((t) => t.input.uri.toString() === b.uri.toString());
                const closing = [];
                const fresh = !tab;
                if (fresh) {
                    const preview = o.preview !== false;
                    const old = preview ? g.tabs.find((t) => t.isPreview) : undefined;
                    if (old) {
                        g.tabs.splice(g.tabs.indexOf(old), 1);
                        closing.push(old);
                    }
                    tab = { input: new TabInputText(b.uri), group: g, isPreview: preview, isActive: false };
                    g.tabs.push(tab);
                }
                for (const t of g.tabs) { t.isActive = t === tab; }
                const editor = { document: b, viewColumn: column, selection: o.selection || { start: { line: 0, character: 0 }, end: { line: 0, character: 0 }, isEmpty: true } };
                window.visibleTextEditors = window.visibleTextEditors.filter((e) => e.viewColumn !== column).concat([editor]);
                if (!o.preserveFocus) {
                    window.activeTextEditor = editor;
                    activateGroup(g);
                }
                events.tabs.fire({ opened: fresh ? [tab] : [], closed: closing, changed: fresh ? [] : [tab] });
                fireVisibleEditors();
                return editor;
            },
            tabGroups: {
                get all() { return groups; },
                onDidChangeTabs: events.tabs.event,
                close: async (tab) => {
                    const g = tab.group;
                    const i = g.tabs.indexOf(tab);
                    if (i < 0) {
                        return false;
                    }
                    g.tabs.splice(i, 1);
                    const editor = window.visibleTextEditors.find((e) => e.viewColumn === g.viewColumn);
                    if (editor && editor.document.uri.toString() === tab.input.uri.toString()) {
                        window.visibleTextEditors = window.visibleTextEditors.filter((e) => e !== editor);
                        if (window.activeTextEditor === editor) {
                            window.activeTextEditor = undefined;
                        }
                    }
                    events.tabs.fire({ opened: [], closed: [tab], changed: [] });
                    fireVisibleEditors();
                    return true;
                }
            }
        }),
        tasks: {
            executeTask: async (task) => { log.tasks.push(task); },
            registerTaskProvider: () => ({ dispose() {} })
        },
        commands: {
            registerCommand: (name, fn) => { registries.commands[name] = fn; return { dispose() {} }; },
            executeCommand: async (name) => { log.commands.push(name); }
        },
        debug: {
            registerDebugConfigurationProvider: (type, p) => { registries.providers.configProvider = p; return { dispose() {} }; },
            registerDebugAdapterDescriptorFactory: (type, f) => { registries.providers.adapterFactory = f; return { dispose() {} }; },
            registerDebugAdapterTrackerFactory: () => ({ dispose() {} }),
            startDebugging: async (k, y) => { log.debugSessions.push(y); return true; },
            activeDebugSession: undefined,
            stopDebugging: async () => {}
        }
    };

    // extension.js'nin require('vscode') çağrısı bu sahteye gider; her kurulum eklentiyi yeniden yükler.
    const extensionPath = require.resolve('../extension');
    delete require.cache[extensionPath];
    const originalResolve = Module._resolveFilename;
    Module._resolveFilename = function (request, ...rest) {
        return request === 'vscode' ? 'vscode' : originalResolve.call(this, request, ...rest);
    };
    require.cache.vscode = { id: 'vscode', filename: 'vscode', loaded: true, exports: vscode };
    let extension;
    try {
        extension = require(extensionPath);
    } finally {
        Module._resolveFilename = originalResolve;
        delete require.cache.vscode;
    }
    const state = s.state || new FakeState();
    const context = { subscriptions: [], extensionUri: Uri.file(path.join(__dirname, '..')), globalState: state };
    extension.activate(context);
    return {
        vscode, log, state, events,
        provider: registries.providers.configProvider, adapterFactory: registries.providers.adapterFactory,
        commands: registries.commands, filesystems: registries.filesystems
    };
}

// wait bekleyen işlerin (run'ın await zinciri) bitmesine izin verir.
function wait() {
    return new Promise((resolve) => setImmediate(resolve));
}

// waitUntil koşul doğru olana kadar bekler (süreç ve dosya işleri gerçek zamanlıdır); süre
// dolarsa hata verir.
async function waitUntil(condition, what, duration) {
    const deadline = Date.now() + (duration || 15000);
    while (!(await condition())) {
        if (Date.now() > deadline) {
            throw new Error('beklenen durum oluşmadı: ' + what);
        }
        await new Promise((resolve) => setTimeout(resolve, 10));
    }
}

module.exports = { setup, wait, waitUntil, Uri, FakeState };
