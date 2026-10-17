// TR dili VS Code eklentisi: .tr dosyalarını çalıştırma, denetleme ve hata ayıklama komutları.
// Düz JavaScript; derleme adımı ve npm bağımlılığı yoktur.
'use strict';

const vscode = require('vscode');
const fs = require('fs');
const path = require('path');

const GOREV_TURU = 'tr';
const AYAR = 'yorumlayiciYolu';
const YORUMLAYICI_ADI = process.platform === 'win32' ? 'tr.exe' : 'tr';

// komut kimliği -> yorumlayıcıya verilecek argümanlar ve görev adı
const KOMUTLAR = {
    calistir: { komut: 'tr.calistir', ad: 'Çalıştır', argumanlar: (dosya) => [dosya] },
    denetle: { komut: 'tr.denetle', ad: 'Denetle', argumanlar: (dosya) => ['kontrol', dosya] },
    hataAyikla: { komut: 'tr.hataAyikla', ad: 'Hata Ayıkla', argumanlar: (dosya) => ['debug', dosya] },
    gorselHataAyikla: { komut: 'tr.gorselHataAyikla', ad: 'Görsel Hata Ayıkla', argumanlar: (dosya) => ['debug', '--web', dosya] }
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

// Yorumlayıcıyı bulur: ayar -> çalışma alanı klasörü -> dosyanın dizininden yukarı -> PATH.
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
        const aday = path.join(klasor.uri.fsPath, YORUMLAYICI_ADI);
        if (dosyaVarMi(aday)) {
            return { yol: aday };
        }
    }

    let dizin = path.dirname(dosyaUri.fsPath);
    for (;;) {
        const aday = path.join(dizin, YORUMLAYICI_ADI);
        if (dosyaVarMi(aday)) {
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
        `${mesaj} TR yorumlayıcısını proje kökünde 'go build -o ${YORUMLAYICI_ADI} ./cmd/tr' ile derleyin ` +
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

// F5 / Ctrl+F5 ve "Çalıştır" menüsü VS Code'un hata ayıklama altyapısından geçer. TR bir
// hata ayıklama türü olarak tanıtılır; ama ayrı bir hata ayıklama oturumu açmak yerine dosya
// yorumlayıcıyla çalıştırılır ve oturum başlatılmaz (undefined döndürmek oturumu iptal eder).
// Yapılandırmada "hataAyikla": true ise terminal hata ayıklayıcısı (tr debug) açılır.
const hataAyiklamaSaglayici = {
    provideDebugConfigurations() {
        return [{ type: GOREV_TURU, request: 'launch', name: 'TR: Dosyayı Çalıştır', program: '${file}' }];
    },
    resolveDebugConfiguration(klasor, yapilandirma) {
        const y = yapilandirma || {};
        let uri;
        if (y.program && !String(y.program).includes('${')) {
            uri = vscode.Uri.file(path.isAbsolute(y.program) || !klasor ? y.program : path.join(klasor.uri.fsPath, y.program));
        }
        const tanim = y.hataAyikla ? KOMUTLAR.hataAyikla : KOMUTLAR.calistir;
        calistir(tanim, uri);
        return undefined;
    }
};

function activate(context) {
    for (const tanim of Object.values(KOMUTLAR)) {
        context.subscriptions.push(vscode.commands.registerCommand(tanim.komut, (uri) => calistir(tanim, uri)));
    }
    context.subscriptions.push(vscode.tasks.registerTaskProvider(GOREV_TURU, gorevSaglayici));
    context.subscriptions.push(vscode.debug.registerDebugConfigurationProvider(GOREV_TURU, hataAyiklamaSaglayici));
}

function deactivate() {}

module.exports = { activate, deactivate };
