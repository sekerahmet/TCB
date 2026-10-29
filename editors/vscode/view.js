// Kodlu dosyanın görünümü (docs/spec/17_kodlar.md, 17.7): .tr dosyası diskte de düzenleyicide de
// kimliklerle (TR_print) yazılır; kişinin yazım dilindeki görünümü yalnız okunur. "Görünümü Göster"
// komutu açık dosyanın görünümünü yanda salt okunur bir belge olarak açar ('tr show - --lang <dil>';
// eklentide ikinci bir çevirici yoktur). Görünüm dosyanın satırlarını tutar: satır numaraları hata
// iletileri ve hata ayıklayıcıyla aynıdır. Dosya düzenlendikçe (kaydedilmemiş metin dahil) ve yazım
// dili değişince görünüm yenilenir. Görünüm düzenlenmez ve koda geri çevrilmez.
//
// Görünümün adresi dosyanın kendi yoludur, yalnızca şeması farklıdır ("tr-view:/c:/proje/a.tr");
// görünüm etkinken çalıştırma ve hata ayıklama komutları dosyanın kendisinde çalışır (localUri).
// Kısıtlı kipte (güvenilmeyen çalışma alanı) yorumlayıcı çalıştırılmaz: görünüm açılmaz, nedeni söylenir.
'use strict';

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const { t } = require('./l10n');

const VIEW_SCHEME = 'tr-view';

// EXIT_SYNTAX 'tr show -' komutunun "metin ayrıştırılamadı, görünüm yazılmadı" çıkış kodudur
// (cmd/tr/show.go, exitSyntax).
const EXIT_SYNTAX = 2;

// TIMEOUT bir görünümün en uzun süresidir (milisaniye); aşılırsa süreç durdurulur.
const TIMEOUT = 30000;

// REFRESH_DELAY düzenlemeden sonra görünümün yenilenmesinden önceki bekleme süresidir (milisaniye).
const REFRESH_DELAY = 300;

// show metni 'tr show - --lang <lang>' ile görünüme çevirir. Kabuk kullanılmaz. Hiçbir zaman reddetmez:
//   { code: 0, output }              gösterildi (output: Buffer)
//   { code: EXIT_SYNTAX, error }     metin ayrıştırılamadı (error: tr'nin iletisi)
//   { code: 1 | -1, error }          başka bir hata (-1: süreç başlatılamadı ya da durduruldu)
function show(interpreter, lang, input, timeoutMs) {
    return new Promise((resolve) => {
        const finish = (result) => {
            clearTimeout(timer);
            resolve(result);
        };
        let proc;
        try {
            proc = spawn(interpreter, ['show', '-', '--lang', lang], { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
        } catch (e) {
            resolve({ code: -1, output: Buffer.alloc(0), error: t('Could not start the TR interpreter: {0}', e && e.message ? e.message : e) });
            return;
        }
        const timer = setTimeout(() => proc.kill(), timeoutMs || TIMEOUT);
        const output = [];
        const error = [];
        proc.stdout.on('data', (d) => output.push(d));
        proc.stderr.on('data', (d) => error.push(d));
        proc.on('error', (e) => finish({ code: -1, output: Buffer.alloc(0), error: t('Could not start the TR interpreter: {0}', e.message) }));
        proc.on('close', (code) => {
            const text = Buffer.concat(error).toString('utf8');
            finish({
                code: code === null ? -1 : code,
                output: Buffer.concat(output),
                error: code === null ? t('The TR interpreter did not respond in time; the view was stopped.') : text
            });
        });
        proc.stdin.on('error', () => {}); // süreç girdiyi okumadan çıktıysa (EPIPE)
        proc.stdin.end(Buffer.from(input));
    });
}

// errorSummary tr'nin hata çıktısından kullanıcıya gösterilecek ilk hatayı çıkarır (kod
// çerçeveleri ve başlıklar atılır).
function errorSummary(text) {
    const lines = String(text || '').split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const item = lines.find((s) => /^[•✗]/.test(s)) || lines.find((s) => !/^[❌⚠]/.test(s)) || lines[0] || '';
    const summary = item.replace(/^[•✗]\s*/, '');
    return summary.length > 300 ? summary.slice(0, 297) + '...' : summary;
}

// create görünümün parçalarını kurar.
//   g.findInterpreter     (dosyaUri) -> { filePath } | { error } | undefined   (find_interpreter.js kuralı)
//   g.writingLanguage     () -> yazım dili kodu ('tr.writingLanguage' ayarı)
//   g.interpreterNotFound (mesaj) -> ayarı açma düğmeli hata mesajı
//   g.show                show ile aynı imza (sınamada değiştirilebilir)
function create(vscode, g) {
    const changed = new vscode.EventEmitter();
    // last her görünümün son yazılabilen metnidir: dosya o an ayrıştırılamıyorsa (yarım kod) görünüm
    // eskisinde kalır.
    const last = new Map();
    const timers = new Map();
    const reported = new Set();

    function viewUri(fileUri) {
        return fileUri.with({ scheme: VIEW_SCHEME });
    }
    // localUri bir belge adresinin diskteki dosyasıdır (görünümde dosyanın kendisi); yerel olmayan
    // adreslerde undefined.
    function localUri(uri) {
        if (!uri) {
            return undefined;
        }
        if (uri.scheme === 'file') {
            return uri;
        }
        return uri.scheme === VIEW_SCHEME ? uri.with({ scheme: 'file' }) : undefined;
    }
    function isView(uri) {
        return !!uri && uri.scheme === VIEW_SCHEME;
    }
    function openViews() {
        return vscode.workspace.textDocuments.filter((d) => isView(d.uri));
    }

    // sourceText dosyanın düzenleyicideki (kaydedilmemiş değişiklikler dahil) ya da diskteki metnidir.
    function sourceText(fileUri) {
        const open = vscode.workspace.textDocuments.find((d) => d.uri.toString() === fileUri.toString());
        return open ? open.getText() : fs.readFileSync(fileUri.fsPath, 'utf8');
    }

    // once mesajı (kind, dosya) için bir kez gösterir; görünüm yazılınca yeniden gösterilebilir.
    function once(kind, filePath, showMessage) {
        const k = kind + '\n' + filePath;
        if (!reported.has(k)) {
            reported.add(k);
            showMessage();
        }
    }

    // provideTextDocumentContent görünümün metnidir. Görünüm yazılamazsa (sözdizimi hatası, yorumlayıcı
    // yok) son yazılan görünüm kalır ve nedeni bir kez söylenir; hiç yazılmadıysa görünüm nedenini taşır.
    async function provideTextDocumentContent(uri) {
        const local = localUri(uri);
        const key = local.toString();
        let reason;
        if (!vscode.workspace.isTrusted) {
            reason = t('The TR interpreter is not run in restricted mode (Workspace Trust).');
        } else {
            const y = g.findInterpreter(local);
            if (!y || y.error) {
                reason = y ? y.error : t('TR interpreter not found.');
            } else {
                let text;
                try {
                    text = sourceText(local);
                } catch (e) {
                    text = undefined;
                    reason = e.message;
                }
                if (text !== undefined) {
                    const s = await g.show(y.filePath, g.writingLanguage(), text);
                    if (s.code === 0) {
                        reported.delete('view\n' + local.fsPath);
                        const output = s.output.toString('utf8');
                        last.set(key, output);
                        return output;
                    }
                    reason = errorSummary(s.error);
                }
            }
        }
        once('view', local.fsPath, () => vscode.window.showWarningMessage(
            t('The view of {0} in your writing language could not be written: {1} The view keeps its last text; it is updated once the file can be read.', path.basename(local.fsPath), reason)));
        return last.has(key) ? last.get(key) : '# ' + reason + '\n';
    }

    // refresh dosyanın açık görünümünü kısa bir beklemeden sonra yeniler (her tuşta değil).
    function refresh(fileUri) {
        const uri = viewUri(fileUri);
        const key = uri.toString();
        if (!openViews().some((d) => d.uri.toString() === key)) {
            return;
        }
        clearTimeout(timers.get(key));
        timers.set(key, setTimeout(() => {
            timers.delete(key);
            changed.fire(uri);
        }, REFRESH_DELAY));
    }

    // showView etkin .tr dosyasının görünümünü yanda salt okunur açar.
    async function showView(uri) {
        const editor = vscode.window.activeTextEditor;
        const local = localUri(uri instanceof vscode.Uri ? uri : editor && editor.document.uri);
        if (!local || path.extname(local.fsPath).toLowerCase() !== '.tr') {
            vscode.window.showErrorMessage(t('Open a .tr file to show its view in your writing language.'));
            return;
        }
        if (!vscode.workspace.isTrusted) {
            vscode.window.showWarningMessage(t('The view in your writing language needs the TR interpreter, which is not run in restricted mode (Workspace Trust).'));
            return;
        }
        const y = g.findInterpreter(local);
        if (!y || y.error) {
            g.interpreterNotFound(t('The view of {0} in your writing language needs the TR interpreter ({1}).', path.basename(local.fsPath), y ? y.error : t('not found')));
            return;
        }
        const document = await vscode.workspace.openTextDocument(viewUri(local));
        await vscode.window.showTextDocument(document, { viewColumn: vscode.ViewColumn.Beside, preview: false, preserveFocus: true });
    }

    function setup(context) {
        context.subscriptions.push(
            vscode.workspace.registerTextDocumentContentProvider(VIEW_SCHEME, { onDidChange: changed.event, provideTextDocumentContent }),
            vscode.workspace.onDidChangeTextDocument((e) => {
                if (e.document.uri.scheme === 'file') {
                    refresh(e.document.uri);
                }
            }),
            vscode.workspace.onDidChangeConfiguration((e) => {
                if (e.affectsConfiguration('tr.writingLanguage')) {
                    for (const d of openViews()) {
                        changed.fire(d.uri);
                    }
                }
            }),
            vscode.commands.registerCommand('tr.showView', showView)
        );
    }

    return { VIEW_SCHEME, setup, viewUri, localUri, isView, showView, provideTextDocumentContent };
}

module.exports = { create, show, errorSummary, VIEW_SCHEME, EXIT_SYNTAX };
