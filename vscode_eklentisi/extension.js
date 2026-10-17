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
    const panel = vscode.window.createWebviewPanel('trTablo', 'Tablo: ' + ifade, vscode.ViewColumn.Beside, { enableScripts: true });
    panel.webview.html = tabloHtml(panel.webview, veri);
}

function tabloHtml(webview, veri) {
    const nonce = crypto.randomBytes(16).toString('base64');
    // Veri betiğe JSON olarak gömülür; '<' kaçışlanır ki </script> enjeksiyonu olmasın.
    const json = JSON.stringify(veri).replace(/</g, '\\u003c');
    return `<!DOCTYPE html>
<html lang="tr"><head><meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';">
<style>
  body { font-family: var(--vscode-font-family); font-size: var(--vscode-font-size); color: var(--vscode-foreground); background: var(--vscode-editor-background); padding: 8px 12px; }
  h2 { font-size: 1.1em; margin: 4px 0 10px; }
  .arac { display: flex; gap: 10px; align-items: center; margin-bottom: 8px; }
  input { background: var(--vscode-input-background); color: var(--vscode-input-foreground); border: 1px solid var(--vscode-input-border, transparent); padding: 4px 8px; min-width: 240px; }
  .sayac { color: var(--vscode-descriptionForeground); }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid var(--vscode-editorWidget-border, #444); padding: 4px 8px; text-align: left; white-space: nowrap; }
  th { position: sticky; top: 0; background: var(--vscode-editorWidget-background); cursor: pointer; user-select: none; }
  td.sayi { text-align: right; font-variant-numeric: tabular-nums; }
  tr:nth-child(even) td { background: var(--vscode-list-hoverBackground); }
  th .ok { opacity: .6; margin-left: 4px; }
</style></head>
<body>
<h2 id="baslik"></h2>
<div class="arac"><input id="ara" placeholder="Tabloda ara..."><span class="sayac" id="sayac"></span></div>
<table><thead><tr id="basliklar"></tr></thead><tbody id="govde"></tbody></table>
<script nonce="${nonce}">
const veri = ${json};
const sutunlar = ['#'].concat(veri.sutunlar || []);
const satirlar = (veri.satirlar || []).map((s, i) => [String(i)].concat(s));
let sirala = { sutun: -1, azalan: false };
const sayiMi = (x) => x !== '' && !isNaN(Number(x));
document.getElementById('baslik').textContent = veri.baslik + ' — ' + (veri.tip || '') + ' (' + satirlar.length + ' satır)';
function ciz() {
  const q = document.getElementById('ara').value.toLocaleLowerCase('tr');
  let liste = satirlar.filter(s => !q || s.some(h => String(h).toLocaleLowerCase('tr').includes(q)));
  if (sirala.sutun >= 0) {
    const k = sirala.sutun;
    liste = liste.slice().sort((a, b) => {
      const x = a[k], y = b[k];
      const c = sayiMi(x) && sayiMi(y) ? Number(x) - Number(y) : String(x).localeCompare(String(y), 'tr');
      return sirala.azalan ? -c : c;
    });
  }
  const bas = document.getElementById('basliklar');
  bas.replaceChildren();
  sutunlar.forEach((ad, i) => {
    const th = document.createElement('th');
    th.textContent = ad;
    if (sirala.sutun === i) { const o = document.createElement('span'); o.className = 'ok'; o.textContent = sirala.azalan ? '▼' : '▲'; th.appendChild(o); }
    th.addEventListener('click', () => { sirala = { sutun: i, azalan: sirala.sutun === i ? !sirala.azalan : false }; ciz(); });
    bas.appendChild(th);
  });
  const govde = document.getElementById('govde');
  govde.replaceChildren();
  liste.forEach(s => {
    const tr = document.createElement('tr');
    s.forEach(h => { const td = document.createElement('td'); td.textContent = h; if (sayiMi(h)) td.className = 'sayi'; tr.appendChild(td); });
    govde.appendChild(tr);
  });
  document.getElementById('sayac').textContent = liste.length + ' / ' + satirlar.length + ' satır';
}
document.getElementById('ara').addEventListener('input', ciz);
ciz();
</script></body></html>`;
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
            { enableScripts: true, retainContextWhenHidden: true }
        );
        izlemeMasasi.panel.webview.html = izlemeMasasiHtml();
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

function izlemeMasasiHtml() {
    const nonce = crypto.randomBytes(16).toString('base64');
    return `<!DOCTYPE html>
<html lang="tr"><head><meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';">
<style>
  html, body { height: 100%; margin: 0; }
  body { font-family: var(--vscode-font-family); font-size: var(--vscode-font-size); color: var(--vscode-foreground); background: var(--vscode-editor-background); display: flex; flex-direction: column; }
  #ust { flex: 1 1 auto; min-height: 80px; overflow: auto; padding: 6px 12px; }
  .baslik { display: flex; justify-content: space-between; align-items: baseline; border-bottom: 2px solid var(--vscode-focusBorder); padding-bottom: 6px; margin-bottom: 6px; gap: 12px; }
  .baslik b { font-size: 1.05em; }
  .soluk { color: var(--vscode-descriptionForeground); }
  .arac { display: flex; gap: 10px; align-items: center; margin: 6px 0; }
  input { background: var(--vscode-input-background); color: var(--vscode-input-foreground); border: 1px solid var(--vscode-input-border, var(--vscode-focusBorder)); padding: 3px 8px; font-family: var(--vscode-editor-font-family); }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid var(--vscode-editorWidget-border, #555); padding: 3px 8px; text-align: left; vertical-align: top; overflow: hidden; }
  th { background: var(--vscode-editorWidget-background); user-select: none; white-space: nowrap; }
  #degiskenTablosu { table-layout: fixed; }
  col.kapsam { width: 64px; } col.ad { width: 24%; } col.tip { width: 120px; } col.islem { width: 34px; }
  td.kapsam { color: var(--vscode-descriptionForeground); white-space: nowrap; }
  td.ad, td.tip, td.deger { white-space: nowrap; text-overflow: ellipsis; }
  td.tip { color: var(--vscode-symbolIcon-classForeground, var(--vscode-foreground)); }
  td.deger { font-family: var(--vscode-editor-font-family); }
  tr.degisti td.deger, tr.degisti td.ad { color: var(--vscode-debugTokenExpression-number, #d19a66); font-weight: 600; }
  #degiskenler > tr:nth-child(even) > td { background: var(--vscode-list-hoverBackground); }
  tr.acik > td { background: var(--vscode-list-inactiveSelectionBackground) !important; }
  tr.tablo-satiri { cursor: pointer; }
  button.kalem { background: none; border: none; cursor: pointer; padding: 0 2px; }
  .ikon { display: inline-block; width: 1.6em; text-align: center; font-weight: 700; margin-right: 4px; font-size: .9em; }
  .ikon.tablo { color: var(--vscode-charts-blue, #3794ff); } .ikon.liste { color: var(--vscode-charts-purple, #b180d7); }
  .ikon.sozluk { color: var(--vscode-charts-orange, #d18616); } .ikon.yapi { color: var(--vscode-charts-red, #f14c4c); }
  .ikon.matris { color: var(--vscode-charts-green, #89d185); } .ikon.sayi { color: var(--vscode-debugTokenExpression-number, #b5cea8); }
  .ikon.metin { color: var(--vscode-debugTokenExpression-string, #ce9178); } .ikon.mantik, .ikon.yok, .ikon.diger { color: var(--vscode-descriptionForeground); }
  .link { color: var(--vscode-textLink-foreground); text-decoration: underline; cursor: pointer; }
  .link:hover { color: var(--vscode-textLink-activeForeground); }
  .bos { color: var(--vscode-descriptionForeground); padding: 10px; }
  #yonlendirme { display: flex; gap: 6px; flex-wrap: wrap; margin: 4px 0 8px; }
  #yonlendirme button { background: var(--vscode-button-secondaryBackground, var(--vscode-editorWidget-background)); color: var(--vscode-button-secondaryForeground, var(--vscode-foreground)); border: 1px solid var(--vscode-button-border, var(--vscode-editorWidget-border, #555)); border-radius: 3px; padding: 3px 10px; cursor: pointer; font: inherit; white-space: nowrap; }
  #yonlendirme button:hover:not(:disabled) { background: var(--vscode-button-secondaryHoverBackground, var(--vscode-list-hoverBackground)); }
  #yonlendirme button.ana { background: var(--vscode-button-background); color: var(--vscode-button-foreground); border-color: transparent; }
  #yonlendirme button.ana:hover:not(:disabled) { background: var(--vscode-button-hoverBackground); }
  #yonlendirme button:disabled { opacity: .45; cursor: default; }
  #yonlendirme .kisayol { opacity: .7; font-size: .85em; margin-left: 4px; }

  /* Alt tablo penceresi: sekmeler + ızgara; üstteki çubuk sürüklenerek boyutlandırılır */
  #ayirici { flex: 0 0 5px; cursor: row-resize; background: var(--vscode-editorWidget-border, #555); display: none; }
  #pencere { flex: 0 0 auto; height: 45vh; min-height: 90px; display: none; flex-direction: column; border-top: 1px solid var(--vscode-focusBorder); }
  #sekmeler { display: flex; align-items: stretch; background: var(--vscode-editorGroupHeader-tabsBackground, var(--vscode-editorWidget-background)); overflow-x: auto; flex: 0 0 auto; }
  .sekme { display: flex; align-items: center; gap: 6px; padding: 4px 6px 4px 10px; cursor: pointer; border-right: 1px solid var(--vscode-tab-border, #444); white-space: nowrap; color: var(--vscode-tab-inactiveForeground, var(--vscode-descriptionForeground)); background: var(--vscode-tab-inactiveBackground, transparent); }
  .sekme.etkin { color: var(--vscode-tab-activeForeground, var(--vscode-foreground)); background: var(--vscode-tab-activeBackground, var(--vscode-editor-background)); border-top: 2px solid var(--vscode-focusBorder); }
  .sekme .kapat { border: none; background: none; color: inherit; cursor: pointer; padding: 0 4px; border-radius: 3px; font-size: 1em; line-height: 1; }
  .sekme .kapat:hover { background: var(--vscode-toolbar-hoverBackground, rgba(128,128,128,.25)); }
  #sekmeler .bosluk { flex: 1; }
  #hepsiniKapat { border: none; background: none; color: var(--vscode-descriptionForeground); cursor: pointer; padding: 0 10px; }
  #pencereArac { padding: 4px 10px; flex: 0 0 auto; }
  #izgaraKutu { flex: 1 1 auto; overflow: auto; padding: 0 10px 8px; }
  #izgaraKutu th { position: sticky; top: 0; cursor: pointer; }
  #izgaraKutu td { white-space: nowrap; }
  #izgaraKutu td.sayi { text-align: right; font-variant-numeric: tabular-nums; }
  #izgaraKutu tbody tr:nth-child(even) td { background: var(--vscode-list-hoverBackground); }
  #izgaraKutu td:focus { outline: 2px solid var(--vscode-focusBorder); outline-offset: -2px; }
</style></head>
<body>
<div id="ust">
  <div class="baslik"><b id="konum">Hata ayıklama bekleniyor…</b><span class="soluk" id="neden"></span></div>
  <div id="yonlendirme">
    <button data-komut="next" class="ana" title="Sonraki satıra geç; fonksiyon çağrılarının içine girmez">⤼ Sonraki satır<span class="kisayol">F10</span></button>
    <button data-komut="stepIn" title="Bu satırdaki fonksiyonun içine gir">↓ İçine gir<span class="kisayol">F11</span></button>
    <button data-komut="stepOut" title="İçinde bulunulan fonksiyondan çık">↑ Dışarı çık<span class="kisayol">Shift+F11</span></button>
    <button data-komut="continue" title="Bir sonraki kesme noktasına kadar devam et">▶ Devam<span class="kisayol">F5</span></button>
    <button data-komut="stop" title="Hata ayıklamayı durdur">■ Durdur<span class="kisayol">Shift+F5</span></button>
  </div>
  <div class="arac"><input id="degiskenAra" placeholder="Değişken ara…" style="width:220px"><span class="soluk" id="degiskenSayac"></span></div>
  <table id="degiskenTablosu">
    <colgroup><col class="kapsam"><col class="ad"><col class="tip"><col><col class="islem"></colgroup>
    <thead><tr><th>Kapsam</th><th>Değişken</th><th>Tip</th><th>Değer</th><th></th></tr></thead>
    <tbody id="degiskenler"></tbody>
  </table>
</div>
<div id="ayirici" title="Sürükleyerek boyutlandırın"></div>
<div id="pencere">
  <div id="sekmeler"></div>
  <div id="pencereArac" class="arac"><input id="tabloAra" placeholder="Tabloda ara…" style="width:220px"><span class="soluk" id="tabloSayac"></span></div>
  <div id="izgaraKutu"></div>
</div>
<script nonce="${nonce}">
const vscode = acquireVsCodeApi();
// Paneldeki beklenmeyen hatalar sessizce kaybolmasın: eklentiye bildirilir.
window.addEventListener('error', (e) => vscode.postMessage({ tur: 'panelHatasi', mesaj: String(e.message) + ' (' + e.lineno + ':' + e.colno + ')' }));
const IKONLAR = { tablo: '▦', liste: '☰', sozluk: '{}', yapi: '◆', matris: '⊞', sayi: '#', metin: 'Aa', mantik: '◐', yok: '∅', diger: '•' };
const TUR_ADI = { tablo: 'Tablo', liste: 'Liste', sozluk: 'Sözlük', yapi: 'Yapı', matris: 'Matris', sayi: 'Sayı', metin: 'Metin', mantik: 'Mantık', yok: 'Yok', diger: 'Değer' };
const sayiMi = (x) => x !== '' && x !== null && !isNaN(Number(x));
const el = (tag, metin, sinif) => { const e = document.createElement(tag); if (metin !== undefined) e.textContent = metin; if (sinif) e.className = sinif; return e; };

// Durum: değişkenler, açık sekmeler (her biri ayrı tablo), etkin sekme ve sekme başına arama/sıralama.
let durum = null, oncekiDegerler = {}, oturumBitti = false, duraklatildi = false;
function dugmeleriAyarla() {
  document.querySelectorAll('#yonlendirme button').forEach((b) => {
    // Durdur her zaman (oturum sürdükçe) kullanılabilir; adım düğmeleri yalnızca program duraklatılmışken.
    b.disabled = oturumBitti || (b.dataset.komut !== 'stop' && !duraklatildi);
  });
}
document.querySelectorAll('#yonlendirme button').forEach((b) => b.addEventListener('click', () => {
  if (b.disabled) return;
  if (b.dataset.komut !== 'stop') {
    duraklatildi = false;
    document.getElementById('neden').textContent = 'Çalışıyor…';
  }
  dugmeleriAyarla();
  vscode.postMessage({ tur: 'komut', komut: b.dataset.komut });
}));
const sekmeler = [];          // { ad, tur, veri, hata, arama, sirala }
let etkin = null;

window.addEventListener('message', (olay) => {
  const m = olay.data;
  if (m.tur === 'durum') {
    durum = m.veri; oturumBitti = false; duraklatildi = true;
    cizDegiskenler(); dugmeleriAyarla();
    // Açık sekmelerin içeriğini yeni adımın değerleriyle yenile
    sekmeler.forEach((s) => vscode.postMessage({ tur: 'tablo', ifade: s.ad }));
  } else if (m.tur === 'tablo') {
    const s = sekmeler.find((x) => x.ad === m.veri.baslik);
    if (s) { s.veri = m.veri; s.hata = ''; if (s.ad === etkin) cizPencere(); }
  } else if (m.tur === 'tabloHata') {
    const s = sekmeler.find((x) => x.ad === m.ifade);
    if (s) { s.hata = m.mesaj; if (s.ad === etkin) cizPencere(); }
  } else if (m.tur === 'hata') {
    document.getElementById('neden').textContent = m.mesaj;
  } else if (m.tur === 'bitti') {
    oturumBitti = true; duraklatildi = false; dugmeleriAyarla();
    document.getElementById('konum').textContent = 'Program bitti — tablolar son duruştaki değerleri gösteriyor';
    document.getElementById('neden').textContent = '';
  }
});

// tabloAc: tabloyu alt penceredeki sekmesinde açar (yoksa yeni sekme ekler) ve etkin yapar.
function tabloAc(d) {
  let s = sekmeler.find((x) => x.ad === d.ad);
  if (!s) {
    s = { ad: d.ad, tur: d.tur || 'tablo', veri: null, hata: '', arama: '', sirala: { sutun: -1, azalan: false } };
    sekmeler.push(s);
    if (oturumBitti) s.hata = 'Hata ayıklama oturumu bitti; tabloyu görmek için programı yeniden başlatın (F5).';
    else vscode.postMessage({ tur: 'tablo', ifade: d.ad });
  }
  etkin = s.ad;
  cizPencere(); cizDegiskenler();
}

function sekmeKapat(ad) {
  const i = sekmeler.findIndex((x) => x.ad === ad);
  if (i < 0) return;
  sekmeler.splice(i, 1);
  if (etkin === ad) etkin = sekmeler.length ? sekmeler[Math.min(i, sekmeler.length - 1)].ad : null;
  cizPencere(); cizDegiskenler();
}

function cizDegiskenler() {
  if (!durum) return;
  if (!oturumBitti) {
    document.getElementById('konum').textContent = durum.dosya + ' · Satır ' + durum.satir + ' · ' + durum.cerceve;
    document.getElementById('neden').textContent = durum.neden || '';
  }
  const q = document.getElementById('degiskenAra').value.toLocaleLowerCase('tr');
  const govde = document.getElementById('degiskenler');
  govde.replaceChildren();
  const yeniDegerler = {};
  durum.degiskenler.forEach((d) => { yeniDegerler[d.kapsam + ':' + d.ad] = d.deger; });
  const gorunen = durum.degiskenler.filter((d) => !q || d.ad.toLocaleLowerCase('tr').includes(q));
  document.getElementById('degiskenSayac').textContent = gorunen.length + ' / ' + durum.degiskenler.length + ' değişken';
  if (!gorunen.length) {
    const tr = el('tr'); const td = el('td', 'Değişken yok', 'bos'); td.colSpan = 5; tr.appendChild(td); govde.appendChild(tr);
  }
  gorunen.forEach((d) => {
    const anahtar = d.kapsam + ':' + d.ad;
    const tur = d.tur || 'diger';
    const acik = sekmeler.some((s) => s.ad === d.ad);
    const tr = el('tr');
    if (acik) tr.classList.add('acik');
    if (anahtar in oncekiDegerler && oncekiDegerler[anahtar] !== d.deger) tr.classList.add('degisti');
    const adHucre = el('td', undefined, 'ad'); adHucre.title = d.ad + ' (' + d.tip + ')';
    const ikon = el('span', IKONLAR[tur] || '•', 'ikon ' + tur); ikon.title = TUR_ADI[tur] || '';
    adHucre.appendChild(ikon);
    let degerHucre;
    if (d.tablo) {
      // <a href> tıklamalarını VS Code'un web görünümü yakalar; link görünümlü span kullanılır.
      const adLink = el('span', d.ad, 'link'); adLink.setAttribute('role', 'button'); adLink.tabIndex = 0;
      adLink.addEventListener('click', (e) => { e.stopPropagation(); tabloAc(d); });
      adLink.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); tabloAc(d); } });
      adHucre.appendChild(adLink);
      degerHucre = el('td', undefined, 'deger');
      const link = el('span', acik ? '● Açık — sekmeye git' : '▸ Tabloyu aç', 'link'); link.setAttribute('role', 'button'); link.tabIndex = 0;
      link.title = d.tip;
      link.addEventListener('click', (e) => { e.stopPropagation(); tabloAc(d); });
      link.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); tabloAc(d); } });
      degerHucre.appendChild(link);
      tr.classList.add('tablo-satiri');
      tr.title = 'Çift tıklayın: tabloyu alttaki pencerede aç';
      tr.addEventListener('dblclick', (e) => { if (e.target.tagName !== 'INPUT') tabloAc(d); });
    } else {
      adHucre.appendChild(document.createTextNode(d.ad));
      degerHucre = el('td', d.deger, 'deger'); degerHucre.title = d.deger;
    }
    const kalem = el('button', '✏️', 'kalem'); kalem.title = 'Değeri değiştir (TR ifadesi)';
    kalem.addEventListener('click', () => duzenle(degerHucre, d));
    const islem = el('td'); islem.appendChild(kalem);
    tr.append(el('td', d.kapsam, 'kapsam'), adHucre, el('td', d.tip, 'tip'), degerHucre, islem);
    govde.appendChild(tr);
  });
  oncekiDegerler = yeniDegerler;
}

function cizPencere() {
  const pencere = document.getElementById('pencere');
  const ayirici = document.getElementById('ayirici');
  const goster = sekmeler.length > 0;
  pencere.style.display = goster ? 'flex' : 'none';
  ayirici.style.display = goster ? 'block' : 'none';
  if (!goster) return;

  const serit = document.getElementById('sekmeler');
  serit.replaceChildren();
  sekmeler.forEach((s) => {
    const sek = el('div', undefined, 'sekme' + (s.ad === etkin ? ' etkin' : ''));
    sek.title = s.ad;
    sek.append(el('span', IKONLAR[s.tur] || '▦', 'ikon ' + s.tur), el('span', s.ad));
    const kapat = el('button', '✕', 'kapat'); kapat.title = 'Sekmeyi kapat';
    kapat.addEventListener('click', (e) => { e.stopPropagation(); sekmeKapat(s.ad); });
    sek.appendChild(kapat);
    sek.addEventListener('click', () => { etkin = s.ad; cizPencere(); });
    sek.addEventListener('auxclick', (e) => { if (e.button === 1) sekmeKapat(s.ad); });
    serit.appendChild(sek);
  });
  serit.appendChild(el('span', undefined, 'bosluk'));
  const hepsi = el('button', 'Tümünü kapat ✕'); hepsi.id = 'hepsiniKapat';
  hepsi.addEventListener('click', () => { sekmeler.splice(0); etkin = null; cizPencere(); cizDegiskenler(); });
  serit.appendChild(hepsi);

  const s = sekmeler.find((x) => x.ad === etkin);
  const ara = document.getElementById('tabloAra');
  ara.value = s ? s.arama : '';
  const kutu = document.getElementById('izgaraKutu');
  kutu.replaceChildren();
  const sayac = document.getElementById('tabloSayac');
  if (!s) { sayac.textContent = ''; return; }
  if (s.hata) { kutu.appendChild(el('div', 'Tablo gösterilemedi: ' + s.hata, 'bos')); sayac.textContent = ''; return; }
  if (!s.veri) { kutu.appendChild(el('div', 'Yükleniyor…', 'bos')); sayac.textContent = ''; return; }

  const sutunlar = ['#'].concat(s.veri.sutunlar || []);
  const satirlar = (s.veri.satirlar || []).map((r, i) => [String(i)].concat(r));
  const q = s.arama.toLocaleLowerCase('tr');
  let liste = satirlar.filter((r) => !q || r.some((h) => String(h).toLocaleLowerCase('tr').includes(q)));
  if (s.sirala.sutun >= 0) {
    const k = s.sirala.sutun;
    liste = liste.slice().sort((a, b) => {
      const c = sayiMi(a[k]) && sayiMi(b[k]) ? Number(a[k]) - Number(b[k]) : String(a[k]).localeCompare(String(b[k]), 'tr');
      return s.sirala.azalan ? -c : c;
    });
  }
  const t = el('table'); const thead = el('thead'); const bas = el('tr'); const govde = el('tbody');
  sutunlar.forEach((ad, i) => {
    const th = el('th', ad + (s.sirala.sutun === i ? (s.sirala.azalan ? ' ▼' : ' ▲') : ''));
    th.title = 'Sıralamak için tıklayın';
    th.addEventListener('click', () => { s.sirala = { sutun: i, azalan: s.sirala.sutun === i ? !s.sirala.azalan : false }; cizPencere(); });
    bas.appendChild(th);
  });
  liste.forEach((r) => {
    const satir = el('tr');
    r.forEach((h) => { const td = el('td', h, sayiMi(h) ? 'sayi' : ''); td.tabIndex = -1; satir.appendChild(td); });
    govde.appendChild(satir);
  });
  thead.appendChild(bas); t.append(thead, govde); kutu.appendChild(t);
  sayac.textContent = s.ad + ' — ' + liste.length + ' / ' + satirlar.length + ' satır · ' + (s.veri.sutunlar || []).length + ' sütun';
}

// Izgarada ok tuşlarıyla hücreden hücreye gezinme
document.getElementById('izgaraKutu').addEventListener('keydown', (e) => {
  const td = e.target.closest('td');
  if (!td) return;
  const tr = td.parentElement;
  let hedef = null;
  if (e.key === 'ArrowRight') hedef = td.nextElementSibling;
  else if (e.key === 'ArrowLeft') hedef = td.previousElementSibling;
  else if (e.key === 'ArrowDown' && tr.nextElementSibling) hedef = tr.nextElementSibling.children[td.cellIndex];
  else if (e.key === 'ArrowUp' && tr.previousElementSibling) hedef = tr.previousElementSibling.children[td.cellIndex];
  if (hedef) { e.preventDefault(); hedef.focus(); }
});
document.getElementById('izgaraKutu').addEventListener('click', (e) => { const td = e.target.closest('td'); if (td) td.focus(); });

document.getElementById('tabloAra').addEventListener('input', (e) => {
  const s = sekmeler.find((x) => x.ad === etkin);
  if (s) { s.arama = e.target.value; cizPencere(); document.getElementById('tabloAra').focus(); }
});
document.getElementById('degiskenAra').addEventListener('input', cizDegiskenler);
dugmeleriAyarla();

// Ayırıcı çubukla alt pencereyi boyutlandırma
(() => {
  const ayirici = document.getElementById('ayirici');
  const pencere = document.getElementById('pencere');
  let surukle = null;
  ayirici.addEventListener('mousedown', (e) => { surukle = { y: e.clientY, h: pencere.getBoundingClientRect().height }; e.preventDefault(); });
  window.addEventListener('mousemove', (e) => {
    if (!surukle) return;
    const yeni = Math.max(90, Math.min(window.innerHeight - 120, surukle.h - (e.clientY - surukle.y)));
    pencere.style.height = yeni + 'px';
  });
  window.addEventListener('mouseup', () => { surukle = null; });
})();

function duzenle(hucre, d) {
  const giris = el('input'); giris.value = d.tablo ? '' : d.deger; giris.placeholder = 'Yeni değer, ör. 0.20 veya [1, 2]'; giris.style.width = '95%';
  hucre.replaceChildren(giris); giris.focus(); giris.select();
  giris.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && giris.value.trim() !== '') vscode.postMessage({ tur: 'degistir', ad: d.ad, ifade: giris.value });
    if (e.key === 'Escape') cizDegiskenler();
  });
}
</script></body></html>`;
}

function activate(context) {
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
