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
    const nonce = Math.random().toString(36).slice(2) + Date.now().toString(36);
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
    if (!oturum || !izlemeMasasi.panel) {
        return;
    }
    try {
        if (m.tur === 'tablo') {
            const veri = await oturum.customRequest('trTablo', { ifade: m.ifade });
            izlemeMasasi.panel.webview.postMessage({ tur: 'tablo', veri });
        } else if (m.tur === 'degistir') {
            await oturum.customRequest('evaluate', { expression: m.ad + ' = ' + m.ifade, context: 'repl', frameId: 1 });
            await izlemeMasasiGuncelle(oturum);
        }
    } catch (e) {
        const mesaj = String(e && e.message ? e.message : e);
        if (m.tur === 'tablo') {
            izlemeMasasi.panel.webview.postMessage({ tur: 'tabloHata', mesaj });
        } else {
            vscode.window.showErrorMessage('Değer değiştirilemedi: ' + mesaj);
        }
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
                } else if ((m.event === 'terminated' || m.event === 'exited') && izlemeMasasi.panel && izlemeMasasi.oturum === oturum) {
                    izlemeMasasi.panel.webview.postMessage({ tur: 'bitti' });
                }
            }
        };
    }
};

function izlemeMasasiHtml() {
    const nonce = Math.random().toString(36).slice(2) + Date.now().toString(36);
    return `<!DOCTYPE html>
<html lang="tr"><head><meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';">
<style>
  body { font-family: var(--vscode-font-family); font-size: var(--vscode-font-size); color: var(--vscode-foreground); background: var(--vscode-editor-background); padding: 6px 12px; }
  .ust { display: flex; justify-content: space-between; align-items: baseline; border-bottom: 2px solid var(--vscode-focusBorder); padding-bottom: 6px; margin-bottom: 8px; }
  .ust b { font-size: 1.05em; }
  .neden { color: var(--vscode-descriptionForeground); }
  h3 { font-size: .95em; margin: 12px 0 6px; text-transform: uppercase; letter-spacing: .04em; color: var(--vscode-descriptionForeground); }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid var(--vscode-editorWidget-border, #555); padding: 3px 8px; text-align: left; vertical-align: top; }
  th { background: var(--vscode-editorWidget-background); position: sticky; top: 0; cursor: pointer; user-select: none; white-space: nowrap; }
  td.deger { font-family: var(--vscode-editor-font-family); word-break: break-all; }
  td.kapsam { color: var(--vscode-descriptionForeground); white-space: nowrap; }
  td.tip { white-space: nowrap; color: var(--vscode-symbolIcon-classForeground, var(--vscode-foreground)); }
  td.sayi { text-align: right; font-variant-numeric: tabular-nums; }
  tr.tablo td.ad { cursor: pointer; text-decoration: underline dotted; }
  tr.secili td { background: var(--vscode-list-activeSelectionBackground) !important; color: var(--vscode-list-activeSelectionForeground); }
  tr.degisti td.deger { color: var(--vscode-debugTokenExpression-number, #d19a66); font-weight: 600; }
  tbody tr:nth-child(even) td { background: var(--vscode-list-hoverBackground); }
  button { background: none; border: none; color: var(--vscode-textLink-foreground); cursor: pointer; padding: 0 4px; }
  input { background: var(--vscode-input-background); color: var(--vscode-input-foreground); border: 1px solid var(--vscode-focusBorder); padding: 2px 6px; width: 95%; font-family: var(--vscode-editor-font-family); }
  .arac { display: flex; gap: 10px; align-items: center; margin-bottom: 6px; }
  .bos { color: var(--vscode-descriptionForeground); padding: 12px 0; }
  .izgara { max-height: 55vh; overflow: auto; }
</style></head>
<body>
<div class="ust"><b id="konum">Hata ayıklama bekleniyor…</b><span class="neden" id="neden"></span></div>
<h3>Değişkenler</h3>
<table><thead><tr><th>Kapsam</th><th>Değişken</th><th>Tip</th><th>Değer</th><th></th></tr></thead><tbody id="degiskenler"></tbody></table>
<div id="tabloAlani" style="display:none">
  <h3 id="tabloBaslik"></h3>
  <div class="arac"><input id="ara" placeholder="Tabloda ara…" style="width:240px"><span class="neden" id="sayac"></span></div>
  <div class="izgara"><table><thead><tr id="tabloBasliklar"></tr></thead><tbody id="tabloGovde"></tbody></table></div>
</div>
<script nonce="${nonce}">
const vscode = acquireVsCodeApi();
let durum = null, oncekiDegerler = {}, seciliTablo = null, tablo = null, sirala = { sutun: -1, azalan: false };
const sayiMi = (x) => x !== '' && x !== null && !isNaN(Number(x));
const el = (tag, metin, sinif) => { const e = document.createElement(tag); if (metin !== undefined) e.textContent = metin; if (sinif) e.className = sinif; return e; };

window.addEventListener('message', (olay) => {
  const m = olay.data;
  if (m.tur === 'durum') {
    durum = m.veri;
    cizDegiskenler();
    if (seciliTablo) vscode.postMessage({ tur: 'tablo', ifade: seciliTablo });
  } else if (m.tur === 'tablo') {
    tablo = m.veri; cizTablo();
  } else if (m.tur === 'tabloHata') {
    tablo = null; document.getElementById('tabloAlani').style.display = 'none'; seciliTablo = null;
  } else if (m.tur === 'hata') {
    document.getElementById('neden').textContent = m.mesaj;
  } else if (m.tur === 'bitti') {
    document.getElementById('konum').textContent = 'Program bitti';
    document.getElementById('neden').textContent = '';
  }
});

function cizDegiskenler() {
  document.getElementById('konum').textContent = durum.dosya + ' · Satır ' + durum.satir + ' · ' + durum.cerceve;
  document.getElementById('neden').textContent = durum.neden || '';
  const govde = document.getElementById('degiskenler');
  govde.replaceChildren();
  const yeniDegerler = {};
  if (!durum.degiskenler.length) {
    const tr = el('tr'); const td = el('td', 'Değişken yok', 'bos'); td.colSpan = 5; tr.appendChild(td); govde.appendChild(tr);
  }
  durum.degiskenler.forEach((d) => {
    const anahtar = d.kapsam + ':' + d.ad;
    yeniDegerler[anahtar] = d.deger;
    const tr = el('tr');
    if (d.tablo) tr.classList.add('tablo');
    if (seciliTablo === d.ad) tr.classList.add('secili');
    if (anahtar in oncekiDegerler && oncekiDegerler[anahtar] !== d.deger) tr.classList.add('degisti');
    const adHucre = el('td', d.ad, 'ad');
    if (d.tablo) adHucre.title = 'Tabloyu aşağıda göster';
    adHucre.addEventListener('click', () => { if (d.tablo) { seciliTablo = d.ad; sirala = { sutun: -1, azalan: false }; vscode.postMessage({ tur: 'tablo', ifade: d.ad }); cizDegiskenler(); } });
    const degerHucre = el('td', d.deger, 'deger');
    const kalem = el('button', '✏️'); kalem.title = 'Değeri değiştir (TR ifadesi)';
    kalem.addEventListener('click', () => duzenle(degerHucre, d));
    const islem = el('td'); islem.appendChild(kalem);
    tr.append(el('td', d.kapsam, 'kapsam'), adHucre, el('td', d.tip, 'tip'), degerHucre, islem);
    govde.appendChild(tr);
  });
  oncekiDegerler = yeniDegerler;
}

function duzenle(hucre, d) {
  const giris = el('input'); giris.value = d.tablo ? '' : d.deger; giris.placeholder = 'Yeni değer, ör. 0.20 veya [1, 2]';
  hucre.replaceChildren(giris); giris.focus(); giris.select();
  giris.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && giris.value.trim() !== '') vscode.postMessage({ tur: 'degistir', ad: d.ad, ifade: giris.value });
    if (e.key === 'Escape') cizDegiskenler();
  });
}

function cizTablo() {
  if (!tablo) return;
  document.getElementById('tabloAlani').style.display = '';
  const sutunlar = ['#'].concat(tablo.sutunlar || []);
  const satirlar = (tablo.satirlar || []).map((s, i) => [String(i)].concat(s));
  document.getElementById('tabloBaslik').textContent = 'Tablo: ' + tablo.baslik + ' (' + satirlar.length + ' satır)';
  const q = document.getElementById('ara').value.toLocaleLowerCase('tr');
  let liste = satirlar.filter((s) => !q || s.some((h) => String(h).toLocaleLowerCase('tr').includes(q)));
  if (sirala.sutun >= 0) {
    const k = sirala.sutun;
    liste = liste.slice().sort((a, b) => {
      const c = sayiMi(a[k]) && sayiMi(b[k]) ? Number(a[k]) - Number(b[k]) : String(a[k]).localeCompare(String(b[k]), 'tr');
      return sirala.azalan ? -c : c;
    });
  }
  const bas = document.getElementById('tabloBasliklar'); bas.replaceChildren();
  sutunlar.forEach((ad, i) => {
    const th = el('th', ad + (sirala.sutun === i ? (sirala.azalan ? ' ▼' : ' ▲') : ''));
    th.addEventListener('click', () => { sirala = { sutun: i, azalan: sirala.sutun === i ? !sirala.azalan : false }; cizTablo(); });
    bas.appendChild(th);
  });
  const govde = document.getElementById('tabloGovde'); govde.replaceChildren();
  liste.forEach((s) => {
    const tr = el('tr');
    s.forEach((h) => tr.appendChild(el('td', h, sayiMi(h) ? 'sayi' : '')));
    govde.appendChild(tr);
  });
  document.getElementById('sayac').textContent = liste.length + ' / ' + satirlar.length + ' satır';
}
document.getElementById('ara').addEventListener('input', cizTablo);
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
