// TR dili VS Code eklentisi: .tr dosyalarını çalıştırma, denetleme ve hata ayıklama komutları.
// Düz JavaScript; derleme adımı ve npm bağımlılığı yoktur.
'use strict';

const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { resolveWatchCommand } = require('./watch_commands');
const { findInterpreter: findInterpreterRule, interpreterName, BIN_FOLDER } = require('./find_interpreter');
const { programArguments } = require('./program_arguments');
const codedFormat = require('./coded_format');
const codedViewModule = require('./coded_view');

const TASK_TYPE = 'tr';
const INTERPRETER_SETTING = 'yorumlayiciYolu';
const INTERPRETER_NAME = interpreterName(process.platform);

// komut kimliği -> yorumlayıcıya verilecek argümanlar ve görev adı. Çalıştırmada dosyadan
// sonraki argümanlar (ek) programa iletilir: 'tr dosya.tr --adım 5'.
const COMMANDS = {
    run: { id: 'tr.calistir', title: 'Çalıştır', args: (file, extra) => [file, ...(extra || [])] },
    check: { id: 'tr.denetle', title: 'Denetle', args: (file) => ['kontrol', file] },
    debug: { id: 'tr.hataAyikla', title: 'Hata Ayıkla', args: (file) => ['hata_ayıkla', file] }
};

function fileExists(filePath) {
    try {
        return fs.statSync(filePath).isFile();
    } catch (e) {
        return false;
    }
}

// Yorumlayıcıyı bulur: ayar -> (yalnızca güvenilen çalışma alanında) çalışma alanı kökü -> PATH.
// Üst klasörler taranmaz. Kural ve gerekçesi find_interpreter.js'tedir.
// Bulunamazsa undefined döner; ayar geçersizse { hata } döner.
function findInterpreter(fileUri) {
    const folder = vscode.workspace.getWorkspaceFolder(fileUri);
    return findInterpreterRule({
        setting: vscode.workspace.getConfiguration('tr').get(INTERPRETER_SETTING),
        folder: folder ? folder.uri.fsPath : undefined,
        trusted: vscode.workspace.isTrusted,
        fileExists,
        pathValue: process.env.PATH,
        platform: process.platform
    });
}

async function interpreterNotFound(message) {
    const openSetting = 'Ayarı Aç';
    const choice = await vscode.window.showErrorMessage(
        `${message} TR yorumlayıcısını proje kökünde 'go build -o ${BIN_FOLDER}/${INTERPRETER_NAME} ./cmd/tr' ile derleyin ` +
        `veya 'tr.${INTERPRETER_SETTING}' ayarına yorumlayıcının tam yolunu yazın.`,
        openSetting
    );
    if (choice === openSetting) {
        vscode.commands.executeCommand('workbench.action.openSettings', `tr.${INTERPRETER_SETTING}`);
    }
}

// codedView, kodlu dosyaların Türkçe görünümüdür (coded_view.js); activate kurar.
let codedView;

// localUri bir belgenin diskteki dosyasıdır (kodlu dosyanın Türkçe görünümünde de dosyanın
// kendisi); yerel olmayan belgede undefined.
function localUri(uri) {
    return codedView ? codedView.localUri(uri) : (uri && uri.scheme === 'file' ? uri : undefined);
}

// Komutun hedef belgesini bulur: başlık çubuğu/bağlam menüsünden gelen URI veya etkin düzenleyici.
// Dosyanın Türkçe görünümü açıksa o seçilir (kaydedilmemiş değişiklikler oradadır).
async function targetDocument(uri) {
    if (uri instanceof vscode.Uri) {
        const existing = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
        return (codedView && codedView.openView(uri)) || existing || vscode.workspace.openTextDocument(uri);
    }
    const editor = vscode.window.activeTextEditor;
    return editor ? editor.document : undefined;
}

function createTask(def, documentUri, interpreter, extra) {
    const file = documentUri.fsPath;
    const folder = vscode.workspace.getWorkspaceFolder(documentUri);
    const cwd = folder ? folder.uri.fsPath : path.dirname(file);

    const task = new vscode.Task(
        { type: TASK_TYPE, komut: def.id, dosya: file },
        folder || vscode.TaskScope.Workspace,
        `${def.title}: ${path.basename(file)}`,
        'TR',
        new vscode.ProcessExecution(interpreter, def.args(file, extra), { cwd }),
        []
    );
    // Görev terminali odaklanır; hata ayıklayıcının komutları buradan yazılır.
    task.presentationOptions = {
        reveal: vscode.TaskRevealKind.Always,
        focus: true,
        panel: vscode.TaskPanelKind.Dedicated,
        clear: true,
        echo: true,
        showReuseMessage: false
    };
    return task;
}

// calistir komutu (tanim) dosyada çalıştırır; ek, dosyadan sonra yorumlayıcıya verilen
// program argümanlarıdır (Ctrl+F5'te launch.json'daki "args").
async function run(def, uri, extra) {
    if (!vscode.workspace.isTrusted) {
        vscode.window.showWarningMessage('TR dosyalarını çalıştırmak için bu çalışma alanına güvenmeniz gerekir (Çalışma Alanı Güveni).');
        return;
    }

    const document = await targetDocument(uri);
    if (!document) {
        vscode.window.showErrorMessage('Çalıştırılacak bir .tr dosyası açık değil.');
        return;
    }
    if (document.languageId !== 'tr' && !document.fileName.toLowerCase().endsWith('.tr')) {
        vscode.window.showErrorMessage(`Bu komut yalnızca .tr dosyalarında çalışır: ${path.basename(document.fileName)}`);
        return;
    }
    if (document.isUntitled) {
        vscode.window.showErrorMessage('Dosyayı çalıştırmadan önce .tr uzantısıyla kaydedin.');
        return;
    }
    const local = localUri(document.uri);
    if (!local) {
        vscode.window.showErrorMessage('Yalnızca yerel diskteki .tr dosyaları çalıştırılabilir.');
        return;
    }
    if (document.isDirty && !(await document.save())) {
        vscode.window.showErrorMessage('Dosya kaydedilemedi; çalıştırma iptal edildi.');
        return;
    }

    const found = findInterpreter(local);
    if (!found || found.error) {
        await interpreterNotFound(found ? found.error : 'TR yorumlayıcısı (' + INTERPRETER_NAME + ') bulunamadı.');
        return;
    }

    // Klasör açılmadan tek dosya açıldığında VS Code, görevlerin kullanıcı klasörü dışında
    // çalışmasına izin vermez ("Cannot launch a terminal process in an empty workspace").
    // Bu durumda yorumlayıcı doğrudan bir terminalde, dosyanın klasöründe başlatılır.
    if (!vscode.workspace.getWorkspaceFolder(local)) {
        runInTerminal(def, local, found.filePath, extra);
        return;
    }

    await vscode.tasks.executeTask(createTask(def, local, found.filePath, extra));
}

// runInTerminal yorumlayıcıyı kabuk kullanmadan doğrudan bir terminal sürecinde başlatır
// (tırnak/kaçış sorunu olmaz). Program bitince terminal açık kalır, çıktı okunabilir.
function runInTerminal(def, documentUri, interpreter, extra) {
    const file = documentUri.fsPath;
    const terminal = vscode.window.createTerminal({
        name: `TR ${def.title}: ${path.basename(file)}`,
        shellPath: interpreter,
        shellArgs: def.args(file, extra),
        cwd: path.dirname(file),
        isTransient: true
    });
    terminal.show(false);
}

// tasks.json içinde { "type": "tr", "komut": "tr.calistir" } yazılabilmesi için ("dosya" isteğe bağlı).
const taskProvider = {
    provideTasks() {
        return [];
    },
    resolveTask(task) {
        const d = task.definition || {};
        const def = Object.values(COMMANDS).find((t) => t.id === d.komut);
        if (!def) {
            return undefined;
        }
        // "dosya" verilmemişse veya çözülmemiş bir değişken içeriyorsa etkin .tr dosyası kullanılır.
        let uri;
        if (d.dosya && !String(d.dosya).includes('${')) {
            uri = vscode.Uri.file(d.dosya);
        } else if (vscode.window.activeTextEditor && localUri(vscode.window.activeTextEditor.document.uri)) {
            uri = localUri(vscode.window.activeTextEditor.document.uri);
        } else {
            return undefined;
        }
        const found = findInterpreter(uri);
        if (!found || found.error) {
            return undefined;
        }
        const resolved = createTask(def, uri, found.filePath);
        return new vscode.Task(d, task.scope || vscode.TaskScope.Workspace, task.name, 'TR', resolved.execution, []);
    }
};

// ---------------------------------------------------------------------------
// VS Code hata ayıklayıcısı (F5): 'tr dap' sürecine bağlanır. Kesme noktaları, adımlama,
// değişkenler, değer değiştirme, izleme noktası, satıra zıplama ve Debug Console çalışır.
// Ctrl+F5 ("Run Without Debugging") dosyayı terminalde çalıştırır (girdi() kullanılabilir).
// ---------------------------------------------------------------------------

// debugConfigurationFor açık belgenin hata ayıklama ayarıdır. Kodlu dosyanın Türkçe
// görünümünde program diskteki dosyadır, "viewURI" görünümün adresidir (coded_view.js).
function debugConfigurationFor(document) {
    const local = localUri(document.uri) || document.uri;
    const folder = vscode.workspace.getWorkspaceFolder(local);
    return codedView.withViewURI({
        type: TASK_TYPE,
        request: 'launch',
        name: 'TR: Hata Ayıkla',
        program: local.fsPath,
        cwd: folder ? folder.uri.fsPath : path.dirname(local.fsPath),
        stopOnEntry: false
    }, document);
}

const debugConfigProvider = {
    provideDebugConfigurations() {
        return [{ type: TASK_TYPE, request: 'launch', name: 'TR: Hata Ayıkla', program: '${file}', cwd: '${workspaceFolder}', stopOnEntry: false }];
    },
    async resolveDebugConfiguration(folder, config) {
        let y = config || {};
        const editor = vscode.window.activeTextEditor;
        // launch.json yokken F5 (ya da Ctrl+F5): açık .tr dosyası için yapılandırma üret;
        // Ctrl+F5'in "hata ayıklamadan" işareti korunur.
        if (!y.type && !y.request && !y.name) {
            if (!editor || editor.document.languageId !== 'tr') {
                vscode.window.showErrorMessage('Hata ayıklamak için bir .tr dosyası açın.');
                return undefined;
            }
            y = Object.assign(debugConfigurationFor(editor.document), y.noDebug ? { noDebug: true } : {});
        } else if (editor) {
            y = codedView.resolveFileVariables(y, editor.document);
        }
        if (!vscode.workspace.isTrusted) {
            vscode.window.showWarningMessage('TR dosyalarında hata ayıklamak için bu çalışma alanına güvenmeniz gerekir (Çalışma Alanı Güveni).');
            return undefined;
        }
        const a = programArguments(y);
        if (a.error) {
            vscode.window.showErrorMessage(a.error);
            return undefined;
        }
        return y;
    },
    // Ctrl+F5 ("Run Without Debugging"): hata ayıklamadan terminalde çalıştırır (girdi() için).
    // Burada ${file}, ${workspaceFolder} gibi değişkenler çözülmüştür; "args" programa iletilir.
    resolveDebugConfigurationWithSubstitutedVariables(folder, y) {
        if (!y || !y.noDebug) {
            return codedView.programView(y);
        }
        const uri = y.program ? vscode.Uri.file(String(y.program)) : undefined;
        run(COMMANDS.run, uri, programArguments(y).args || []);
        return undefined;
    }
};

const debugAdapterFactory = {
    createDebugAdapterDescriptor(session) {
        const config = session.configuration || {};
        const program = config.program;
        if (!program) {
            throw new Error("Hata ayıklanacak .tr dosyası belirtilmedi ('program').");
        }
        const found = findInterpreter(vscode.Uri.file(program));
        if (!found || found.error) {
            const message = found ? found.error : 'TR yorumlayıcısı (' + INTERPRETER_NAME + ') bulunamadı.';
            interpreterNotFound(message);
            throw new Error(message);
        }
        return new vscode.DebugAdapterExecutable(found.filePath, ['dap'], { cwd: config.cwd || path.dirname(program) });
    }
};

async function startDebugSession(uri) {
    const document = await targetDocument(uri);
    if (!document || document.languageId !== 'tr') {
        vscode.window.showErrorMessage('Hata ayıklamak için bir .tr dosyası açın.');
        return;
    }
    if (document.isDirty) {
        await document.save();
    }
    await vscode.debug.startDebugging(vscode.workspace.getWorkspaceFolder(localUri(document.uri) || document.uri), debugConfigurationFor(document));
}

// ---------------------------------------------------------------------------
// Web görünümleri: HTML/CSS/JS dosyaları media/ klasöründedir. Şablondaki {{ad}} yer
// tutucuları doldurulur; betikler yalnızca her açılışta üretilen nonce ile çalışır.
// ---------------------------------------------------------------------------

let extensionUri;

function webviewOptions(extra) {
    return Object.assign({ enableScripts: true, localResourceRoots: [vscode.Uri.joinPath(extensionUri, 'media')] }, extra);
}

function webviewHtml(webview, name, values) {
    const media = vscode.Uri.joinPath(extensionUri, 'media');
    const all = Object.assign({
        nonce: crypto.randomBytes(16).toString('base64'),
        cspSource: webview.cspSource,
        css: webview.asWebviewUri(vscode.Uri.joinPath(media, name + '.css')).toString(),
        js: webview.asWebviewUri(vscode.Uri.joinPath(media, name + '.js')).toString()
    }, values);
    const template = fs.readFileSync(vscode.Uri.joinPath(media, name + '.html').fsPath, 'utf8');
    return template.replace(/\{\{(\w+)\}\}/g, (_, key) => all[key]);
}

// ---------------------------------------------------------------------------
// Tablo görünümü: Variables / Watch panelinde sağ tık → "Tablo Görünümü"
// ---------------------------------------------------------------------------

async function showTable(arg) {
    const session = vscode.debug.activeDebugSession;
    if (!session || session.type !== TASK_TYPE) {
        vscode.window.showErrorMessage('Tablo görünümü için duraklatılmış bir TR hata ayıklama oturumu gerekir.');
        return;
    }
    let expression = arg && arg.variable ? (arg.variable.evaluateName || arg.variable.name) : undefined;
    if (!expression) {
        expression = await vscode.window.showInputBox({ prompt: 'Tablo olarak gösterilecek değişken veya ifade', placeHolder: 'örn. siparisler' });
    }
    if (!expression) {
        return;
    }
    let data;
    try {
        data = await session.customRequest('trTablo', { ifade: expression });
    } catch (e) {
        vscode.window.showErrorMessage('Tablo gösterilemedi: ' + (e && e.message ? e.message : e));
        return;
    }
    const panel = vscode.window.createWebviewPanel('trTablo', 'Tablo: ' + expression, vscode.ViewColumn.Beside, webviewOptions());
    panel.webview.html = tableHtml(panel.webview, data);
}

function tableHtml(webview, data) {
    // Veri betiğe JSON olarak gömülür; '<' kaçışlanır ki </script> enjeksiyonu olmasın.
    const json = JSON.stringify(data).replace(/</g, '\\u003c');
    return webviewHtml(webview, 'table', { veri: json });
}

// ---------------------------------------------------------------------------
// İzleme Masası: program her durduğunda editörün yanında tüm değişkenleri
// tek tabloda (Kapsam | Değişken | Tip | Değer) gösterir. Tabloya tıklanınca altta tablo
// ızgarası açılır ve adım atıldıkça güncellenir; ✏️ ile değer doğrudan değiştirilir.
// ---------------------------------------------------------------------------

const watchPanelState = { panel: undefined, session: undefined };

function watchPanelEnabled() {
    return vscode.workspace.getConfiguration('tr').get('izlemeMasasi', true);
}

function openWatchPanel(session, focus) {
    if (!watchPanelState.panel) {
        watchPanelState.panel = vscode.window.createWebviewPanel(
            'trIzlemeMasasi', 'İzleme Masası',
            { viewColumn: vscode.ViewColumn.Beside, preserveFocus: !focus },
            webviewOptions({ retainContextWhenHidden: true })
        );
        watchPanelState.panel.webview.html = watchPanelHtml(watchPanelState.panel.webview);
        watchPanelState.panel.onDidDispose(() => { watchPanelState.panel = undefined; });
        watchPanelState.panel.webview.onDidReceiveMessage(onWatchPanelMessage);
    } else if (focus) {
        watchPanelState.panel.reveal(undefined, false);
    }
    watchPanelState.session = session || watchPanelState.session;
}

async function updateWatchPanel(session) {
    watchPanelState.session = session;
    if (!watchPanelState.panel) {
        if (!watchPanelEnabled()) {
            return;
        }
        openWatchPanel(session, false);
    }
    try {
        const data = await session.customRequest('trDegiskenler', {});
        watchPanelState.panel.webview.postMessage({ tur: 'durum', veri: data });
    } catch (e) {
        watchPanelState.panel.webview.postMessage({ tur: 'hata', mesaj: String(e && e.message ? e.message : e) });
    }
}

async function onWatchPanelMessage(m) {
    const session = watchPanelState.session;
    if (!watchPanelState.panel) {
        return;
    }
    if (m.tur === 'panelHatasi') {
        vscode.window.showErrorMessage('İzleme Masası hatası: ' + m.mesaj);
        return;
    }
    if (!session) {
        // Hata ayıklama oturumu bittiyse istek göndermek "No debugger available" hatası verir.
        if (m.tur === 'tablo') {
            watchPanelState.panel.webview.postMessage({ tur: 'tabloHata', ifade: m.ifade, mesaj: 'Hata ayıklama oturumu bitti; tabloyu görmek için programı yeniden başlatın (F5).' });
        } else {
            vscode.window.showWarningMessage('Hata ayıklama oturumu bitti; değer değiştirilemez.');
        }
        return;
    }
    try {
        if (m.tur === 'tablo') {
            const data = await session.customRequest('trTablo', { ifade: m.ifade });
            watchPanelState.panel.webview.postMessage({ tur: 'tablo', veri: data });
        } else if (m.tur === 'komut') {
            await runWatchCommand(session, m.komut);
        } else if (m.tur === 'degistir') {
            await session.customRequest('evaluate', { expression: m.ad + ' = ' + m.ifade, context: 'repl', frameId: 1 });
            await updateWatchPanel(session);
        }
    } catch (e) {
        const message = String(e && e.message ? e.message : e);
        if (m.tur === 'tablo') {
            watchPanelState.panel.webview.postMessage({ tur: 'tabloHata', ifade: m.ifade, mesaj: message });
        } else {
            vscode.window.showErrorMessage('Değer değiştirilemedi: ' + message);
        }
    }
}

// Yönlendirme düğmeleri: oturum etkin oturumsa VS Code'un kendi komutları kullanılır (araç
// çubuğu ve editördeki sarı satır da güncellenir); değilse istek doğrudan oturuma gönderilir.
// Eşleme watch_commands.js'tedir (Node ile ayrıca sınanır).
async function runWatchCommand(session, command) {
    const active = !!vscode.debug.activeDebugSession && vscode.debug.activeDebugSession.id === session.id;
    const action = resolveWatchCommand(command, active);
    if (!action) {
        return;
    }
    if (action.kind === 'vscode') {
        await vscode.commands.executeCommand(action.command);
    } else if (action.kind === 'durdur') {
        await vscode.debug.stopDebugging(session);
    } else {
        await session.customRequest(action.request, action.args);
    }
}

function watchPanelSessionEnded(session) {
    if (watchPanelState.session !== session) {
        return;
    }
    watchPanelState.session = undefined;
    if (watchPanelState.panel) {
        watchPanelState.panel.webview.postMessage({ tur: 'bitti' });
    }
}

const watchPanelTracker = {
    createDebugAdapterTracker(session) {
        return {
            onDidSendMessage(m) {
                if (!m || m.type !== 'event') {
                    return;
                }
                if (m.event === 'stopped') {
                    updateWatchPanel(session);
                } else if (m.event === 'terminated' || m.event === 'exited') {
                    watchPanelSessionEnded(session);
                }
            },
            onWillStopSession() { watchPanelSessionEnded(session); },
            onExit() { watchPanelSessionEnded(session); }
        };
    }
};

function watchPanelHtml(webview) {
    return webviewHtml(webview, 'watch_panel');
}

function activate(context) {
    extensionUri = context.extensionUri;
    codedView = codedViewModule.create(vscode, {
        findInterpreter,
        translate: (...a) => codedFormat.translate(...a),
        state: context.globalState,
        writingLanguage: () => vscode.workspace.getConfiguration('tr').get('yazimDili', 'tr'),
        interpreterNotFound
    });
    codedView.setup(context);
    context.subscriptions.push(vscode.debug.registerDebugAdapterTrackerFactory(TASK_TYPE, watchPanelTracker));
    context.subscriptions.push(vscode.commands.registerCommand('tr.izlemeMasasi', () => {
        const session = vscode.debug.activeDebugSession;
        openWatchPanel(session && session.type === TASK_TYPE ? session : undefined, true);
        if (session && session.type === TASK_TYPE) {
            updateWatchPanel(session);
        }
    }));
    for (const def of Object.values(COMMANDS)) {
        context.subscriptions.push(vscode.commands.registerCommand(def.id, (uri) => run(def, uri)));
    }
    context.subscriptions.push(vscode.commands.registerCommand('tr.hataAyiklaBaslat', startDebugSession));
    context.subscriptions.push(vscode.commands.registerCommand('tr.tabloGoster', showTable));
    context.subscriptions.push(vscode.tasks.registerTaskProvider(TASK_TYPE, taskProvider));
    context.subscriptions.push(vscode.debug.registerDebugConfigurationProvider(TASK_TYPE, debugConfigProvider));
    context.subscriptions.push(vscode.debug.registerDebugAdapterDescriptorFactory(TASK_TYPE, debugAdapterFactory));
}

function deactivate() {}

module.exports = { activate, deactivate };
