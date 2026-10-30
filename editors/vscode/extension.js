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
const { programTerminal } = require('./program_terminal');
const viewModule = require('./view');
const snippets = require('./snippets');
const { t } = require('./l10n');

const TASK_TYPE = 'tr';
const INTERPRETER_SETTING = 'interpreterPath';
const INTERPRETER_NAME = interpreterName(process.platform);

// komut kimliği -> yorumlayıcıya verilecek argümanlar ve görev adı. Çalıştırmada dosyadan
// sonraki argümanlar (ek) programa iletilir: 'tr dosya.tr --adım 5'.
const COMMANDS = {
    run: { id: 'tr.run', title: t('Run'), args: (file, extra) => [file, ...(extra || [])] },
    check: { id: 'tr.check', title: t('Check'), args: (file) => ['check', file] },
    debug: { id: 'tr.debug', title: t('Debug'), args: (file) => ['debug', file] }
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
    const openSetting = t('Open Settings');
    const choice = await vscode.window.showErrorMessage(
        t("{0} Build the TR interpreter at the project root with 'go build -o {1}/{2} ./cmd/tr' or set 'tr.interpreterPath' to the interpreter's full path.", message, BIN_FOLDER, INTERPRETER_NAME),
        openSetting
    );
    if (choice === openSetting) {
        vscode.commands.executeCommand('workbench.action.openSettings', `tr.${INTERPRETER_SETTING}`);
    }
}

// views, kodlu dosyaların salt okunur görünümüdür (view.js); activate kurar.
let views;

// localUri bir belgenin diskteki dosyasıdır (görünümde de dosyanın kendisi); yerel olmayan belgede
// undefined.
function localUri(uri) {
    return views ? views.localUri(uri) : (uri && uri.scheme === 'file' ? uri : undefined);
}

// Komutun hedef belgesini bulur: başlık çubuğu/bağlam menüsünden gelen URI veya etkin düzenleyici.
// Görünüm etkinse hedef görünümün dosyasıdır (görünüm salt okunurdur; kod dosyadadır).
async function targetDocument(uri) {
    const editor = vscode.window.activeTextEditor;
    const target = uri instanceof vscode.Uri ? uri : editor && editor.document.uri;
    if (!target) {
        return undefined;
    }
    const file = views && views.isView(target) ? views.localUri(target) : target;
    if (editor && editor.document.uri.toString() === file.toString()) {
        return editor.document;
    }
    const existing = vscode.workspace.textDocuments.find((b) => b.uri.toString() === file.toString());
    return existing || vscode.workspace.openTextDocument(file);
}

// writingLanguage kişinin yazım dili ayarıdır (tr.writingLanguage).
function writingLanguage() {
    return vscode.workspace.getConfiguration('tr').get('writingLanguage', 'tr');
}

// languageEnvironment, yorumlayıcıya başlattığımız her süreçte yazım dilini (TR_LANG) ayara göre
// verir; böylece çalışan program ve hata ayıklayıcı iletileri ayarla aynı dilde gösterir.
function languageEnvironment() {
    return { TR_LANG: writingLanguage() };
}

function createTask(def, documentUri, interpreter, extra) {
    const file = documentUri.fsPath;
    const folder = vscode.workspace.getWorkspaceFolder(documentUri);
    const cwd = folder ? folder.uri.fsPath : path.dirname(file);

    const task = new vscode.Task(
        { type: TASK_TYPE, command: def.id, file: file },
        folder || vscode.TaskScope.Workspace,
        `${def.title}: ${path.basename(file)}`,
        'TR',
        new vscode.ProcessExecution(interpreter, def.args(file, extra), { cwd, env: languageEnvironment() }),
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

// run komutu (tanim) dosyada çalıştırır; ek, dosyadan sonra yorumlayıcıya verilen
// program argümanlarıdır (Ctrl+F5'te launch.json'daki "args").
async function run(def, uri, extra) {
    if (!vscode.workspace.isTrusted) {
        vscode.window.showWarningMessage(t('Running TR files requires trusting this workspace (Workspace Trust).'));
        return;
    }

    const document = await targetDocument(uri);
    if (!document) {
        vscode.window.showErrorMessage(t('No .tr file is open to run.'));
        return;
    }
    if (document.languageId !== 'tr' && !document.fileName.toLowerCase().endsWith('.tr')) {
        vscode.window.showErrorMessage(t('This command works only on .tr files: {0}', path.basename(document.fileName)));
        return;
    }
    if (document.isUntitled) {
        vscode.window.showErrorMessage(t('Save the file with a .tr extension before running it.'));
        return;
    }
    const local = localUri(document.uri);
    if (!local) {
        vscode.window.showErrorMessage(t('Only .tr files on the local disk can be run.'));
        return;
    }
    if (document.isDirty && !(await document.save())) {
        vscode.window.showErrorMessage(t('The file could not be saved; the run was cancelled.'));
        return;
    }

    const found = findInterpreter(local);
    if (!found || found.error) {
        await interpreterNotFound(found ? found.error : t('TR interpreter ({0}) not found.', INTERPRETER_NAME));
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

// runInTerminal yorumlayıcıyı kabuk kullanmadan eklentinin terminalinde başlatır (tırnak/kaçış
// sorunu olmaz). Program bitince terminal açık kalır, çıktı okunabilir (program_terminal.js).
function runInTerminal(def, documentUri, interpreter, extra) {
    const file = documentUri.fsPath;
    const terminal = vscode.window.createTerminal({
        name: `TR ${def.title}: ${path.basename(file)}`,
        pty: programTerminal({ command: interpreter, args: def.args(file, extra), cwd: path.dirname(file), env: languageEnvironment() }),
        isTransient: true
    });
    terminal.show(false);
}

// tasks.json içinde { "type": "tr", "command": "tr.run" } yazılabilmesi için ("file" isteğe bağlı).
const taskProvider = {
    provideTasks() {
        return [];
    },
    resolveTask(task) {
        // Görev yorumlayıcıyı çalıştırır: güvenilmeyen çalışma alanında çözülmez (run() ile aynı kural).
        if (!vscode.workspace.isTrusted) {
            return undefined;
        }
        const d = task.definition || {};
        const def = Object.values(COMMANDS).find((t) => t.id === d.command);
        if (!def) {
            return undefined;
        }
        // "file" verilmemişse veya çözülmemiş bir değişken içeriyorsa etkin .tr dosyası kullanılır.
        let uri;
        if (d.file && !String(d.file).includes('${')) {
            uri = vscode.Uri.file(d.file);
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

// debugConfigurationFor açık belgenin hata ayıklama ayarıdır; görünümde de program diskteki
// dosyadır (view.js).
function debugConfigurationFor(document) {
    const local = localUri(document.uri) || document.uri;
    const folder = vscode.workspace.getWorkspaceFolder(local);
    return {
        type: TASK_TYPE,
        request: 'launch',
        name: 'TR: Debug',
        program: local.fsPath,
        cwd: folder ? folder.uri.fsPath : path.dirname(local.fsPath),
        stopOnEntry: false
    };
}

const debugConfigProvider = {
    provideDebugConfigurations() {
        return [{ type: TASK_TYPE, request: 'launch', name: 'TR: Debug', program: '${file}', cwd: '${workspaceFolder}', stopOnEntry: false }];
    },
    async resolveDebugConfiguration(folder, config) {
        let y = config || {};
        const editor = vscode.window.activeTextEditor;
        // launch.json yokken F5 (ya da Ctrl+F5): açık .tr dosyası için yapılandırma üret;
        // Ctrl+F5'in "hata ayıklamadan" işareti korunur.
        if (!y.type && !y.request && !y.name) {
            if (!editor || editor.document.languageId !== 'tr') {
                vscode.window.showErrorMessage(t('Open a .tr file to debug.'));
                return undefined;
            }
            y = Object.assign(debugConfigurationFor(editor.document), y.noDebug ? { noDebug: true } : {});
        }
        if (!vscode.workspace.isTrusted) {
            vscode.window.showWarningMessage(t('Debugging TR files requires trusting this workspace (Workspace Trust).'));
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
            return y;
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
            throw new Error(t("No .tr file to debug was given ('program')."));
        }
        const found = findInterpreter(vscode.Uri.file(program));
        if (!found || found.error) {
            const message = found ? found.error : t('TR interpreter ({0}) not found.', INTERPRETER_NAME);
            interpreterNotFound(message);
            throw new Error(message);
        }
        return new vscode.DebugAdapterExecutable(found.filePath, ['dap'], { cwd: config.cwd || path.dirname(program), env: languageEnvironment() });
    }
};

async function startDebugSession(uri) {
    const document = await targetDocument(uri);
    if (!document || document.languageId !== 'tr') {
        vscode.window.showErrorMessage(t('Open a .tr file to debug.'));
        return;
    }
    if (!vscode.workspace.isTrusted) {
        vscode.window.showWarningMessage(t('Debugging TR files requires trusting this workspace (Workspace Trust).'));
        return;
    }
    if (document.isDirty && !(await document.save())) {
        vscode.window.showErrorMessage(t('The file could not be saved; debugging was cancelled.'));
        return;
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
        js: webview.asWebviewUri(vscode.Uri.joinPath(media, name + '.js')).toString(),
        common: webview.asWebviewUri(vscode.Uri.joinPath(media, 'table_common.js')).toString()
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
        vscode.window.showErrorMessage(t('The table view needs a paused TR debug session.'));
        return;
    }
    let expression = arg && arg.variable ? (arg.variable.evaluateName || arg.variable.name) : undefined;
    if (!expression) {
        expression = await vscode.window.showInputBox({ prompt: t('Variable or expression to show as a table'), placeHolder: t('e.g. orders') });
    }
    if (!expression) {
        return;
    }
    let data;
    try {
        data = await session.customRequest('trTable', { expression });
    } catch (e) {
        vscode.window.showErrorMessage(t('Could not show the table: {0}', e && e.message ? e.message : e));
        return;
    }
    const panel = vscode.window.createWebviewPanel('trTable', t('Table: {0}', expression), vscode.ViewColumn.Beside, webviewOptions());
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
    return vscode.workspace.getConfiguration('tr').get('autoOpenWatchPanel', true);
}

function openWatchPanel(session, focus) {
    if (!watchPanelState.panel) {
        watchPanelState.panel = vscode.window.createWebviewPanel(
            'trWatchPanel', t('Watch Panel'),
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
        const data = await session.customRequest('trVariables', {});
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
        vscode.window.showErrorMessage(t('Watch Panel error: {0}', m.mesaj));
        return;
    }
    if (!session) {
        // Hata ayıklama oturumu bittiyse istek göndermek "No debugger available" hatası verir.
        if (m.tur === 'tablo') {
            watchPanelState.panel.webview.postMessage({ tur: 'tabloHata', kapsam: m.kapsam, ifade: m.ifade, mesaj: t('The debug session has ended; restart the program (F5) to see the table.') });
        } else {
            vscode.window.showWarningMessage(t('The debug session has ended; the value cannot be changed.'));
        }
        return;
    }
    try {
        if (m.tur === 'tablo') {
            const data = await session.customRequest('trTable', { expression: m.ifade, scope: m.kapsam });
            watchPanelState.panel.webview.postMessage({ tur: 'tablo', veri: Object.assign({ scope: m.kapsam }, data) });
        } else if (m.tur === 'komut') {
            await runWatchCommand(session, m.komut);
        } else if (m.tur === 'degistir') {
            await setWatchValue(session, m.kapsam, m.ad, m.ifade);
            await updateWatchPanel(session);
        }
    } catch (e) {
        const message = String(e && e.message ? e.message : e);
        if (m.tur === 'tablo') {
            watchPanelState.panel.webview.postMessage({ tur: 'tabloHata', kapsam: m.kapsam, ifade: m.ifade, mesaj: message });
        } else {
            vscode.window.showErrorMessage(t('Could not change the value: {0}', message));
        }
    }
}

// setWatchValue ✏️ ile değeri değiştirir: DAP setVariable, değişkenin kapsamının
// variablesReference'ıyla gönderilir (aynı adlı genel/yerel değişkenden doğrusu değişir);
// hata ayıklayıcı yeni değeri yalnızca TEK ifade olarak hesaplar, deyim çalıştırılmaz.
async function setWatchValue(session, scope, name, expression) {
    const { scopes } = await session.customRequest('scopes', { frameId: 1 });
    const found = (scopes || []).find((k) => k.name === scope);
    if (!found) {
        throw new Error(t("'{0}' scope not found (the program may have stopped in another frame).", scope));
    }
    await session.customRequest('setVariable', { variablesReference: found.variablesReference, name, value: expression });
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
    views = viewModule.create(vscode, {
        findInterpreter,
        show: (...a) => viewModule.show(...a),
        writingLanguage,
        interpreterNotFound
    });
    views.setup(context);
    context.subscriptions.push(snippets.register(vscode, snippets.loadTable(context.extensionUri.fsPath), writingLanguage));
    context.subscriptions.push(vscode.debug.registerDebugAdapterTrackerFactory(TASK_TYPE, watchPanelTracker));
    context.subscriptions.push(vscode.commands.registerCommand('tr.watchPanel', () => {
        const session = vscode.debug.activeDebugSession;
        openWatchPanel(session && session.type === TASK_TYPE ? session : undefined, true);
        if (session && session.type === TASK_TYPE) {
            updateWatchPanel(session);
        }
    }));
    for (const def of Object.values(COMMANDS)) {
        context.subscriptions.push(vscode.commands.registerCommand(def.id, (uri) => run(def, uri)));
    }
    context.subscriptions.push(vscode.commands.registerCommand('tr.debugStart', startDebugSession));
    context.subscriptions.push(vscode.commands.registerCommand('tr.showTable', showTable));
    context.subscriptions.push(vscode.tasks.registerTaskProvider(TASK_TYPE, taskProvider));
    context.subscriptions.push(vscode.debug.registerDebugConfigurationProvider(TASK_TYPE, debugConfigProvider));
    context.subscriptions.push(vscode.debug.registerDebugAdapterDescriptorFactory(TASK_TYPE, debugAdapterFactory));
}

function deactivate() {}

module.exports = { activate, deactivate };
