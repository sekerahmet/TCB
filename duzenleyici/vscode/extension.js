// TR dili VS Code eklentisi: .tr dosyalarını çalıştırma, denetleme ve hata ayıklama komutları.
// Düz JavaScript; derleme adımı ve npm bağımlılığı yoktur.
'use strict';

const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const GOREV_TURU = 'tr';
const AYAR = 'yorumlayiciYolu';
const YORUMLAYICI_ADI = process.platform === 'win32' ? 'tr.exe' : 'tr';
// Depoda yorumlayıcı ikili/ klasörüne derlenir; bir klasörde önce ikili/ altına, sonra klasörün
// kendisine bakılır (kökteki eski derlemeler de bulunsun diye).
const IKILI_KLASORU = 'ikili';

// komut kimliği -> yorumlayıcıya verilecek argümanlar ve görev adı
const KOMUTLAR = {
    calistir: { komut: 'tr.calistir', ad: 'Çalıştır', argumanlar: (dosya) => [dosya] },
    denetle: { komut: 'tr.denetle', ad: 'Denetle', argumanlar: (dosya) => ['kontrol', dosya] },
    hataAyikla: { komut: 'tr.hataAyikla', ad: 'Hata Ayıkla', argumanlar: (dosya) => ['hata_ayıkla', dosya] }
};

function dosyaVarMi(yol) {
    try {
        return fs.statSync(yol).isFile();
    } catch (e) {
        return false;
    }
}

// PATH üzerinde yorumlayıcıyı arar. Unix'te /bin ve /usr/bin atlanır: oradaki 'tr'
// coreutils'in karakter çevirme aracıdır, TR yorumlayıcısı değil.
function pathUzerindeAra() {
    const dizinler = (process.env.PATH || '').split(path.delimiter).filter(Boolean);
    for (const dizin of dizinler) {
        if (process.platform !== 'win32' && (dizin === '/bin' || dizin === '/usr/bin')) {
            continue;
        }
        const aday = path.join(dizin, YORUMLAYICI_ADI);
        if (dosyaVarMi(aday)) {
            return aday;
        }
    }
    return undefined;
}

// klasordeAra yorumlayıcıyı klasörün ikili/ alt klasöründe, sonra klasörün kendisinde arar.
function klasordeAra(klasor) {
    for (const aday of [path.join(klasor, IKILI_KLASORU, YORUMLAYICI_ADI), path.join(klasor, YORUMLAYICI_ADI)]) {
        if (dosyaVarMi(aday)) {
            return aday;
        }
    }
    return undefined;
}

// Yorumlayıcıyı bulur: ayar -> çalışma alanı klasörü -> dosyanın dizininden yukarı -> PATH
// (her klasörde önce ikili/ alt klasörüne bakılır).
// Bulunamazsa undefined döner; ayar geçersizse { hata } döner.
function yorumlayiciyiBul(dosyaUri) {
    const klasor = vscode.workspace.getWorkspaceFolder(dosyaUri);
    const ayar = (vscode.workspace.getConfiguration('tr', dosyaUri).get(AYAR) || '').trim();

    if (ayar) {
        let yol = ayar;
        if (!path.isAbsolute(yol)) {
            yol = path.resolve(klasor ? klasor.uri.fsPath : path.dirname(dosyaUri.fsPath), yol);
        }
        if (dosyaVarMi(yol)) {
            return { yol };
        }
        return { hata: `'tr.${AYAR}' ayarında belirtilen yorumlayıcı bulunamadı: ${yol}` };
    }

    if (klasor) {
        const aday = klasordeAra(klasor.uri.fsPath);
        if (aday) {
            return { yol: aday };
        }
    }

    let dizin = path.dirname(dosyaUri.fsPath);
    for (;;) {
        const aday = klasordeAra(dizin);
        if (aday) {
            return { yol: aday };
        }
        const ust = path.dirname(dizin);
        if (ust === dizin) {
            break;
        }
        dizin = ust;
    }

    const pathYolu = pathUzerindeAra();
    return pathYolu ? { yol: pathYolu } : undefined;
}

async function yorumlayiciBulunamadi(mesaj) {
    const ayarAc = 'Ayarı Aç';
    const secim = await vscode.window.showErrorMessage(
        `${mesaj} TR yorumlayıcısını proje kökünde 'go build -o ${IKILI_KLASORU}/${YORUMLAYICI_ADI} ./cmd/tr' ile derleyin ` +
        `veya 'tr.${AYAR}' ayarına yorumlayıcının tam yolunu yazın.`,
        ayarAc
    );
    if (secim === ayarAc) {
        vscode.commands.executeCommand('workbench.action.openSettings', `tr.${AYAR}`);
    }
}

// Komutun hedef belgesini bulur: başlık çubuğu/bağlam menüsünden gelen URI veya etkin düzenleyici.
async function hedefBelge(uri) {
    if (uri instanceof vscode.Uri) {
        const acik = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
        return acik || vscode.workspace.openTextDocument(uri);
    }
    const editor = vscode.window.activeTextEditor;
    return editor ? editor.document : undefined;
}

function gorevOlustur(tanim, belgeUri, yorumlayici) {
    const dosya = belgeUri.fsPath;
    const klasor = vscode.workspace.getWorkspaceFolder(belgeUri);
    const cwd = klasor ? klasor.uri.fsPath : path.dirname(dosya);

    const gorev = new vscode.Task(
        { type: GOREV_TURU, komut: tanim.komut, dosya },
        klasor || vscode.TaskScope.Workspace,
        `${tanim.ad}: ${path.basename(dosya)}`,
        'TR',
        new vscode.ProcessExecution(yorumlayici, tanim.argumanlar(dosya), { cwd }),
        []
    );
    // Görev terminali odaklanır; hata ayıklayıcının komutları buradan yazılır.
    gorev.presentationOptions = {
        reveal: vscode.TaskRevealKind.Always,
        focus: true,
        panel: vscode.TaskPanelKind.Dedicated,
        clear: true,
        echo: true,
        showReuseMessage: false
    };
    return gorev;
}

async function calistir(tanim, uri) {
    if (!vscode.workspace.isTrusted) {
        vscode.window.showWarningMessage('TR dosyalarını çalıştırmak için bu çalışma alanına güvenmeniz gerekir (Çalışma Alanı Güveni).');
        return;
    }

    const belge = await hedefBelge(uri);
    if (!belge) {
        vscode.window.showErrorMessage('Çalıştırılacak bir .tr dosyası açık değil.');
        return;
    }
    if (belge.languageId !== 'tr' && !belge.fileName.toLowerCase().endsWith('.tr')) {
        vscode.window.showErrorMessage(`Bu komut yalnızca .tr dosyalarında çalışır: ${path.basename(belge.fileName)}`);
        return;
    }
    if (belge.isUntitled) {
        vscode.window.showErrorMessage('Dosyayı çalıştırmadan önce .tr uzantısıyla kaydedin.');
        return;
    }
    if (belge.uri.scheme !== 'file') {
        vscode.window.showErrorMessage('Yalnızca yerel diskteki .tr dosyaları çalıştırılabilir.');
        return;
    }
    if (belge.isDirty && !(await belge.save())) {
        vscode.window.showErrorMessage('Dosya kaydedilemedi; çalıştırma iptal edildi.');
        return;
    }

    const bulunan = yorumlayiciyiBul(belge.uri);
    if (!bulunan || bulunan.hata) {
        await yorumlayiciBulunamadi(bulunan ? bulunan.hata : 'TR yorumlayıcısı (' + YORUMLAYICI_ADI + ') bulunamadı.');
        return;
    }

    // Klasör açılmadan tek dosya açıldığında VS Code, görevlerin kullanıcı klasörü dışında
    // çalışmasına izin vermez ("Cannot launch a terminal process in an empty workspace").
    // Bu durumda yorumlayıcı doğrudan bir terminalde, dosyanın klasöründe başlatılır.
    if (!vscode.workspace.getWorkspaceFolder(belge.uri)) {
        terminaldeCalistir(tanim, belge.uri, bulunan.yol);
        return;
    }

    await vscode.tasks.executeTask(gorevOlustur(tanim, belge.uri, bulunan.yol));
}

// terminaldeCalistir yorumlayıcıyı kabuk kullanmadan doğrudan bir terminal sürecinde başlatır
// (tırnak/kaçış sorunu olmaz). Program bitince terminal açık kalır, çıktı okunabilir.
function terminaldeCalistir(tanim, belgeUri, yorumlayici) {
    const dosya = belgeUri.fsPath;
    const terminal = vscode.window.createTerminal({
        name: `TR ${tanim.ad}: ${path.basename(dosya)}`,
        shellPath: yorumlayici,
        shellArgs: tanim.argumanlar(dosya),
        cwd: path.dirname(dosya),
        isTransient: true
    });
    terminal.show(false);
}

// tasks.json içinde { "type": "tr", "komut": "tr.calistir" } yazılabilmesi için ("dosya" isteğe bağlı).
const gorevSaglayici = {
    provideTasks() {
        return [];
    },
    resolveTask(gorev) {
        const d = gorev.definition || {};
        const tanim = Object.values(KOMUTLAR).find((t) => t.komut === d.komut);
        if (!tanim) {
            return undefined;
        }
        // "dosya" verilmemişse veya çözülmemiş bir değişken içeriyorsa etkin .tr dosyası kullanılır.
        let uri;
        if (d.dosya && !String(d.dosya).includes('${')) {
            uri = vscode.Uri.file(d.dosya);
        } else if (vscode.window.activeTextEditor && vscode.window.activeTextEditor.document.uri.scheme === 'file') {
            uri = vscode.window.activeTextEditor.document.uri;
        } else {
            return undefined;
        }
        const bulunan = yorumlayiciyiBul(uri);
        if (!bulunan || bulunan.hata) {
            return undefined;
        }
        const cozulen = gorevOlustur(tanim, uri, bulunan.yol);
        return new vscode.Task(d, gorev.scope || vscode.TaskScope.Workspace, gorev.name, 'TR', cozulen.execution, []);
    }
};

// ---------------------------------------------------------------------------
// VS Code hata ayıklayıcısı (F5): 'tr dap' sürecine bağlanır. Kesme noktaları, adımlama,
// değişkenler, değer değiştirme, izleme noktası, satıra zıplama ve Debug Console çalışır.
// Ctrl+F5 ("Run Without Debugging") dosyayı terminalde çalıştırır (girdi() kullanılabilir).
// ---------------------------------------------------------------------------

function hataAyiklamaYapilandirmasi(belge) {
    const klasor = vscode.workspace.getWorkspaceFolder(belge.uri);
    return {
        type: GOREV_TURU,
        request: 'launch',
        name: 'TR: Hata Ayıkla',
        program: belge.uri.fsPath,
        cwd: klasor ? klasor.uri.fsPath : path.dirname(belge.uri.fsPath),
        stopOnEntry: false
    };
}

const hataAyiklamaSaglayici = {
    provideDebugConfigurations() {
        return [{ type: GOREV_TURU, request: 'launch', name: 'TR: Hata Ayıkla', program: '${file}', cwd: '${workspaceFolder}', stopOnEntry: false }];
    },
    async resolveDebugConfiguration(klasor, yapilandirma) {
        let y = yapilandirma || {};
        const editor = vscode.window.activeTextEditor;
        // launch.json yokken F5: açık .tr dosyası için yapılandırma üret
        if (!y.type && !y.request && !y.name) {
            if (!editor || editor.document.languageId !== 'tr') {
                vscode.window.showErrorMessage('Hata ayıklamak için bir .tr dosyası açın.');
                return undefined;
            }
            y = hataAyiklamaYapilandirmasi(editor.document);
        }
        if (!vscode.workspace.isTrusted) {
            vscode.window.showWarningMessage('TR dosyalarında hata ayıklamak için bu çalışma alanına güvenmeniz gerekir (Çalışma Alanı Güveni).');
            return undefined;
        }
        // Ctrl+F5: hata ayıklamadan çalıştır → terminalde (girdi() için)
        if (y.noDebug) {
            let uri;
            if (y.program && !String(y.program).includes('${')) {
                uri = vscode.Uri.file(y.program);
            }
            calistir(KOMUTLAR.calistir, uri);
            return undefined;
        }
        return y;
    }
};

const hataAyiklamaSureciFabrikasi = {
    createDebugAdapterDescriptor(oturum) {
        const yapilandirma = oturum.configuration || {};
        const program = yapilandirma.program;
        if (!program) {
            throw new Error("Hata ayıklanacak .tr dosyası belirtilmedi ('program').");
        }
        const bulunan = yorumlayiciyiBul(vscode.Uri.file(program));
        if (!bulunan || bulunan.hata) {
            const mesaj = bulunan ? bulunan.hata : 'TR yorumlayıcısı (' + YORUMLAYICI_ADI + ') bulunamadı.';
            yorumlayiciBulunamadi(mesaj);
            throw new Error(mesaj);
        }
        return new vscode.DebugAdapterExecutable(bulunan.yol, ['dap'], { cwd: yapilandirma.cwd || path.dirname(program) });
    }
};

async function hataAyiklamayiBaslat(uri) {
    const belge = await hedefBelge(uri);
    if (!belge || belge.languageId !== 'tr') {
        vscode.window.showErrorMessage('Hata ayıklamak için bir .tr dosyası açın.');
        return;
    }
    if (belge.isDirty) {
        await belge.save();
    }
    await vscode.debug.startDebugging(vscode.workspace.getWorkspaceFolder(belge.uri), hataAyiklamaYapilandirmasi(belge));
}

// ---------------------------------------------------------------------------
// Web görünümleri: HTML/CSS/JS dosyaları media/ klasöründedir. Şablondaki {{ad}} yer
// tutucuları doldurulur; betikler yalnızca her açılışta üretilen nonce ile çalışır.
// ---------------------------------------------------------------------------

let eklentiUri;

function webviewSecenekleri(ek) {
    return Object.assign({ enableScripts: true, localResourceRoots: [vscode.Uri.joinPath(eklentiUri, 'media')] }, ek);
}

function webviewHtml(webview, ad, degerler) {
    const media = vscode.Uri.joinPath(eklentiUri, 'media');
    const tumu = Object.assign({
        nonce: crypto.randomBytes(16).toString('base64'),
        cspSource: webview.cspSource,
        css: webview.asWebviewUri(vscode.Uri.joinPath(media, ad + '.css')).toString(),
        js: webview.asWebviewUri(vscode.Uri.joinPath(media, ad + '.js')).toString()
    }, degerler);
    const sablon = fs.readFileSync(vscode.Uri.joinPath(media, ad + '.html').fsPath, 'utf8');
    return sablon.replace(/\{\{(\w+)\}\}/g, (_, anahtar) => tumu[anahtar]);
}

// ---------------------------------------------------------------------------
// Tablo görünümü: Variables / Watch panelinde sağ tık → "Tablo Görünümü"
// ---------------------------------------------------------------------------

async function tabloGoster(arg) {
    const oturum = vscode.debug.activeDebugSession;
    if (!oturum || oturum.type !== GOREV_TURU) {
        vscode.window.showErrorMessage('Tablo görünümü için duraklatılmış bir TR hata ayıklama oturumu gerekir.');
        return;
    }
    let ifade = arg && arg.variable ? (arg.variable.evaluateName || arg.variable.name) : undefined;
    if (!ifade) {
        ifade = await vscode.window.showInputBox({ prompt: 'Tablo olarak gösterilecek değişken veya ifade', placeHolder: 'örn. siparisler' });
    }
    if (!ifade) {
        return;
    }
    let veri;
    try {
        veri = await oturum.customRequest('trTablo', { ifade });
    } catch (e) {
        vscode.window.showErrorMessage('Tablo gösterilemedi: ' + (e && e.message ? e.message : e));
        return;
    }
    const panel = vscode.window.createWebviewPanel('trTablo', 'Tablo: ' + ifade, vscode.ViewColumn.Beside, webviewSecenekleri());
    panel.webview.html = tabloHtml(panel.webview, veri);
}

function tabloHtml(webview, veri) {
    // Veri betiğe JSON olarak gömülür; '<' kaçışlanır ki </script> enjeksiyonu olmasın.
    const json = JSON.stringify(veri).replace(/</g, '\\u003c');
    return webviewHtml(webview, 'tablo', { veri: json });
}

// ---------------------------------------------------------------------------
// İzleme Masası: program her durduğunda editörün yanında tüm değişkenleri
// tek tabloda (Kapsam | Değişken | Tip | Değer) gösterir. Tabloya tıklanınca altta tablo
// ızgarası açılır ve adım atıldıkça güncellenir; ✏️ ile değer doğrudan değiştirilir.
// ---------------------------------------------------------------------------

const izlemeMasasi = { panel: undefined, oturum: undefined };

function izlemeMasasiAyari() {
    return vscode.workspace.getConfiguration('tr').get('izlemeMasasi', true);
}

function izlemeMasasiAc(oturum, odak) {
    if (!izlemeMasasi.panel) {
        izlemeMasasi.panel = vscode.window.createWebviewPanel(
            'trIzlemeMasasi', 'İzleme Masası',
            { viewColumn: vscode.ViewColumn.Beside, preserveFocus: !odak },
            webviewSecenekleri({ retainContextWhenHidden: true })
        );
        izlemeMasasi.panel.webview.html = izlemeMasasiHtml(izlemeMasasi.panel.webview);
        izlemeMasasi.panel.onDidDispose(() => { izlemeMasasi.panel = undefined; });
        izlemeMasasi.panel.webview.onDidReceiveMessage(izlemeMasasiMesaj);
    } else if (odak) {
        izlemeMasasi.panel.reveal(undefined, false);
    }
    izlemeMasasi.oturum = oturum || izlemeMasasi.oturum;
}

async function izlemeMasasiGuncelle(oturum) {
    izlemeMasasi.oturum = oturum;
    if (!izlemeMasasi.panel) {
        if (!izlemeMasasiAyari()) {
            return;
        }
        izlemeMasasiAc(oturum, false);
    }
    try {
        const veri = await oturum.customRequest('trDegiskenler', {});
        izlemeMasasi.panel.webview.postMessage({ tur: 'durum', veri });
    } catch (e) {
        izlemeMasasi.panel.webview.postMessage({ tur: 'hata', mesaj: String(e && e.message ? e.message : e) });
    }
}

async function izlemeMasasiMesaj(m) {
    const oturum = izlemeMasasi.oturum;
    if (!izlemeMasasi.panel) {
        return;
    }
    if (m.tur === 'panelHatasi') {
        vscode.window.showErrorMessage('İzleme Masası hatası: ' + m.mesaj);
        return;
    }
    if (!oturum) {
        // Hata ayıklama oturumu bittiyse istek göndermek "No debugger available" hatası verir.
        if (m.tur === 'tablo') {
            izlemeMasasi.panel.webview.postMessage({ tur: 'tabloHata', ifade: m.ifade, mesaj: 'Hata ayıklama oturumu bitti; tabloyu görmek için programı yeniden başlatın (F5).' });
        } else {
            vscode.window.showWarningMessage('Hata ayıklama oturumu bitti; değer değiştirilemez.');
        }
        return;
    }
    try {
        if (m.tur === 'tablo') {
            const veri = await oturum.customRequest('trTablo', { ifade: m.ifade });
            izlemeMasasi.panel.webview.postMessage({ tur: 'tablo', veri });
        } else if (m.tur === 'komut') {
            await izlemeMasasiKomut(oturum, m.komut);
        } else if (m.tur === 'degistir') {
            await oturum.customRequest('evaluate', { expression: m.ad + ' = ' + m.ifade, context: 'repl', frameId: 1 });
            await izlemeMasasiGuncelle(oturum);
        }
    } catch (e) {
        const mesaj = String(e && e.message ? e.message : e);
        if (m.tur === 'tablo') {
            izlemeMasasi.panel.webview.postMessage({ tur: 'tabloHata', ifade: m.ifade, mesaj });
        } else {
            vscode.window.showErrorMessage('Değer değiştirilemedi: ' + mesaj);
        }
    }
}

// Yönlendirme düğmeleri: oturum etkin oturumsa VS Code'un kendi komutları kullanılır (araç
// çubuğu ve editördeki sarı satır da güncellenir); değilse istek doğrudan oturuma gönderilir.
const IZLEME_KOMUTLARI = {
    next: 'workbench.action.debug.stepOver',
    stepIn: 'workbench.action.debug.stepInto',
    stepOut: 'workbench.action.debug.stepOut',
    continue: 'workbench.action.debug.continue',
    stop: 'workbench.action.debug.stop'
};

async function izlemeMasasiKomut(oturum, komut) {
    if (!IZLEME_KOMUTLARI[komut]) {
        return;
    }
    if (vscode.debug.activeDebugSession && vscode.debug.activeDebugSession.id === oturum.id) {
        await vscode.commands.executeCommand(IZLEME_KOMUTLARI[komut]);
    } else if (komut === 'stop') {
        await vscode.debug.stopDebugging(oturum);
    } else {
        await oturum.customRequest(komut, { threadId: 1 });
    }
}

function izlemeMasasiOturumBitti(oturum) {
    if (izlemeMasasi.oturum !== oturum) {
        return;
    }
    izlemeMasasi.oturum = undefined;
    if (izlemeMasasi.panel) {
        izlemeMasasi.panel.webview.postMessage({ tur: 'bitti' });
    }
}

const izlemeMasasiIzleyici = {
    createDebugAdapterTracker(oturum) {
        return {
            onDidSendMessage(m) {
                if (!m || m.type !== 'event') {
                    return;
                }
                if (m.event === 'stopped') {
                    izlemeMasasiGuncelle(oturum);
                } else if (m.event === 'terminated' || m.event === 'exited') {
                    izlemeMasasiOturumBitti(oturum);
                }
            },
            onWillStopSession() { izlemeMasasiOturumBitti(oturum); },
            onExit() { izlemeMasasiOturumBitti(oturum); }
        };
    }
};

function izlemeMasasiHtml(webview) {
    return webviewHtml(webview, 'izleme_masasi');
}

function activate(context) {
    eklentiUri = context.extensionUri;
    context.subscriptions.push(vscode.debug.registerDebugAdapterTrackerFactory(GOREV_TURU, izlemeMasasiIzleyici));
    context.subscriptions.push(vscode.commands.registerCommand('tr.izlemeMasasi', () => {
        const oturum = vscode.debug.activeDebugSession;
        izlemeMasasiAc(oturum && oturum.type === GOREV_TURU ? oturum : undefined, true);
        if (oturum && oturum.type === GOREV_TURU) {
            izlemeMasasiGuncelle(oturum);
        }
    }));
    for (const tanim of Object.values(KOMUTLAR)) {
        context.subscriptions.push(vscode.commands.registerCommand(tanim.komut, (uri) => calistir(tanim, uri)));
    }
    context.subscriptions.push(vscode.commands.registerCommand('tr.hataAyiklaBaslat', hataAyiklamayiBaslat));
    context.subscriptions.push(vscode.commands.registerCommand('tr.tabloGoster', tabloGoster));
    context.subscriptions.push(vscode.tasks.registerTaskProvider(GOREV_TURU, gorevSaglayici));
    context.subscriptions.push(vscode.debug.registerDebugConfigurationProvider(GOREV_TURU, hataAyiklamaSaglayici));
    context.subscriptions.push(vscode.debug.registerDebugAdapterDescriptorFactory(GOREV_TURU, hataAyiklamaSureciFabrikasi));
}

function deactivate() {}

module.exports = { activate, deactivate };
