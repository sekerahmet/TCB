// Kodlu dosyaların Türkçe görünümü (docs/spec/17_kodlar.md, "Düzenleyicide"): diskte kodlu saklanan bir
// .tr dosyası düzenleyicide kişinin yazım dilinde açılır, düzenlenir ve kaydedilince diske yine
// kodlu yazılır.
//
// Mekanizma: ayrı bir dosya sistemi şeması (VIEW_SCHEME). Görünümün adresi dosyanın kendi
// yoludur, yalnızca şeması farklıdır ("tr-kodlu:/c:/proje/a.tr"). VS Code bu belgeyi sıradan bir
// metin belgesi olarak yönetir; okuma ve yazma bu sağlayıcıdan geçer:
//   okuma: dosya kodluysa 'tr göster -' ile Türkçe yazım (başlık satırı görünmez; satırlar
//          hata mesajları ve hata ayıklayıcıyla aynı sayılır, 17.5), değilse olduğu gibi;
//   yazma: 'tr kodla -' ile kodlu biçim. Metin ayrıştırılamıyorsa (yarım kod) metin başlıksız
//          Türkçe yazımıyla, KAYIPSIZ yazılır ve dosya "kodlanmayı bekliyor" diye işaretlenir;
//          ilk geçerli kayıtta yeniden kodlu yazılır.
// Geri al/yinele, kirli durum, "dosya diskte değişti" uyarısı, hepsini kaydet ve otomatik kaydet
// VS Code'un kendi belge modelindedir; kayıtta belgenin metnine dokunulmaz.
//
// Dosya gezgininden (ya da bir hata bağlantısından) açılan kodlu dosyanın ham sekmesi görünümle
// değiştirilir; seçim (satır, sütun) olduğu gibi taşınır: hata konumları zaten Türkçe gösterime
// göredir (17.5). Eklenti kendiliğinden kodlamaz.
//
// Kodsuz (başlıksız) dosya Türkçe yazılmıştır (17.7). Yazım dili Türkçeyse olduğu gibi açılır;
// başka bir dilse o da görünümle, kişinin dilinde açılır ('tr göster - --dil en') ve kaydedilince
// diske yine kodsuz Türkçe yazılır ('tr kodla - --dil en', ardından 'tr göster - --dil tr').
// Metin ayrıştırılamıyorsa kayıpsız yazılır ve dosya "Türkçeye çevrilmeyi bekliyor" diye
// işaretlenir; ilk geçerli kayıtta yeniden Türkçe yazılır. Kısıtlı kipte kodsuz dosya olduğu
// gibi (Türkçe, düzenlenebilir) açılır.
//
// Kısıtlı kipte (güvenilmeyen çalışma alanı) hiçbir şey çalıştırılmaz: kodlu dosya salt okunur
// ham hâliyle (RAW_SCHEME) gösterilir ve nedeni söylenir; güven verilince görünüme geçilir.
'use strict';

const fs = require('fs');
const path = require('path');
const codedFormat = require('./coded_format');
const codedFilesystem = require('./coded_filesystem');

const VIEW_SCHEME = 'tr-kodlu';
const RAW_SCHEME = 'tr-ham';
// PENDING_KEY kodlanmayı bekleyen (ayrıştırılamadığı için Türkçe kaydedilmiş) dosyaların
// kalıcı listesinin anahtarıdır; VS Code yeniden açılınca da bilinir.
const PENDING_KEY = 'tr.kodlanmayiBekleyenDosyalar';
// PLAIN_PENDING_KEY Türkçeye çevrilmeyi bekleyen (kişinin dilinde yazılmış ama ayrıştırılamadığı
// için çevrilemeden kaydedilmiş) kodsuz dosyaların kalıcı listesinin anahtarıdır.
const PLAIN_PENDING_KEY = 'tr.turkceyeCevrilmeyiBekleyenDosyalar';
// SOURCE_LANGUAGE kodsuz dosyaların yazıldığı dildir (17.7).
const SOURCE_LANGUAGE = 'tr';
const CASE_INSENSITIVE = process.platform === 'win32' || process.platform === 'darwin';

// create görünümün parçalarını kurar.
//   g.findInterpreter  (dosyaUri) -> { filePath } | { error } | undefined   (find_interpreter.js kuralı)
//   g.translate             codedFormat.translate ile aynı imza
//   g.state             kalıcı durum (ExtensionContext.globalState)
//   g.writingLanguage         () -> yazım dili kodu ('tr.yazimDili' ayarı)
//   g.interpreterNotFound (mesaj) -> ayarı açma düğmeli hata mesajı
function create(vscode, g) {
    const reported = new Set(); // bu oturumda bir kez söylenen durumlar (anahtar + dosya)

    function key(filePath) {
        const full = path.resolve(filePath);
        return CASE_INSENSITIVE ? full.toLowerCase() : full;
    }
    function pendingFiles(listKey) {
        return new Set(g.state.get(listKey, []));
    }
    function isPending(filePath, listKey) {
        return pendingFiles(listKey || PENDING_KEY).has(key(filePath));
    }
    async function setPending(filePath, pending, listKey) {
        listKey = listKey || PENDING_KEY;
        const b = pendingFiles(listKey);
        if (b.has(key(filePath)) === pending) {
            return;
        }
        if (pending) {
            b.add(key(filePath));
        } else {
            b.delete(key(filePath));
        }
        await g.state.update(listKey, [...b]);
    }
    // anyPending dosya kodlanmayı ya da Türkçeye çevrilmeyi bekliyor mu (diskteki metin
    // kişinin yazım dilindedir)?
    function anyPending(filePath) {
        return isPending(filePath) || isPending(filePath, PLAIN_PENDING_KEY);
    }
    // once mesajı bu oturumda (kind, file) için yalnızca bir kez gösterir.
    function once(kind, filePath, show) {
        const k = kind + '\n' + key(filePath);
        if (!reported.has(k)) {
            reported.add(k);
            show();
        }
    }
    function forget(kind, filePath) {
        reported.delete(kind + '\n' + key(filePath));
    }
    // Yazım dili her değiştiğinde kirli belge uyarısı yeniden gösterilebilir.
    function forgetDirtyNotices() {
        for (const k of [...reported]) {
            if (k.startsWith('dil\n')) {
                reported.delete(k);
            }
        }
    }

    function viewUri(fileUri) {
        return fileUri.with({ scheme: VIEW_SCHEME });
    }
    function rawUri(fileUri) {
        return fileUri.with({ scheme: RAW_SCHEME });
    }
    // localUri bir belge adresinin diskteki dosyasıdır (görünümde ve ham gösterimde dosyanın
    // kendisi); yerel olmayan adreslerde undefined.
    function localUri(uri) {
        if (!uri) {
            return undefined;
        }
        if (uri.scheme === 'file') {
            return uri;
        }
        return uri.scheme === VIEW_SCHEME || uri.scheme === RAW_SCHEME ? uri.with({ scheme: 'file' }) : undefined;
    }
    function localPath(uri) {
        return localUri(uri).fsPath;
    }

    // shouldEncode diskteki dosya görünümden kaydedilince kodlu yazılır mı: dosya kodluysa ya
    // da kodlanmayı bekliyorsa (ya da henüz yoksa: görünüm yalnızca kodlu dosyalar için açılır).
    function shouldEncode(filePath, disk) {
        return !disk || codedFormat.isCoded(disk) || isPending(filePath);
    }

    // viewKind dosyanın görünümle açılıp açılmayacağıdır: 'coded' (kodlu ya da bekliyor),
    // 'plain' (kodsuz Türkçe dosya, yazım dili Türkçe değil) ya da undefined (olduğu gibi açılır).
    async function viewKind(filePath) {
        if (anyPending(filePath)) {
            return 'coded';
        }
        let handle;
        try {
            handle = await fs.promises.open(filePath, 'r');
            // Başlık (BOM ve CRLF ile 19 bayt) ilk 64 baytın içindedir.
            const head = Buffer.alloc(64);
            const { bytesRead } = await handle.read(head, 0, head.length, 0);
            if (codedFormat.isCoded(head.subarray(0, bytesRead))) {
                return 'coded';
            }
            return g.writingLanguage() === SOURCE_LANGUAGE ? undefined : 'plain';
        } catch (e) {
            return undefined;
        } finally {
            if (handle) {
                await handle.close();
            }
        }
    }

    // opensInView dosya görünümle açılmalı mı?
    async function opensInView(filePath) {
        return (await viewKind(filePath)) !== undefined;
    }

    // interpreter çeviri için yorumlayıcıyı verir; kısıtlı kipte hiçbir şey çalıştırılmaz.
    function interpreter(filePath) {
        if (!vscode.workspace.isTrusted) {
            return { error: 'Kısıtlı kipte TR yorumlayıcısı çalıştırılmaz (Çalışma Alanı Güveni).' };
        }
        return g.findInterpreter(vscode.Uri.file(filePath)) || { error: 'TR yorumlayıcısı bulunamadı.' };
    }

    // Her açık görünümün Türkçesinin hangi yazım diliyle üretildiği: kayıtta metin AYNI dille
    // kodlanır. Ayar sonradan değişse de (kirli belge yeni dille yenilenemez) metin, yazıldığı
    // dille kodlanınca yerleşik kodlar bozulmaz.
    const resolvedLanguage = new Map();
    // Yazım dili değişince yenilenen belgelerin sürümü (coded_filesystem.js stat).
    const revisions = new Map();
    function revision(filePath) {
        return revisions.get(key(filePath)) || 0;
    }

    const filesystems = codedFilesystem.create(vscode, { VIEW_SCHEME, RAW_SCHEME, localPath, readView, saveText, revision });

    // readView dosyanın görünümde gösterilecek metnidir. Kodlu dosyanın gösterimi üretilemezse
    // (dosya ayrıştırılamıyor, ör. birleştirme çakışması işaretleri; ya da yorumlayıcı yok) ham
    // baytlar gösterilir: kaydedilince olduğu gibi yazılır, hiçbir şey kaybolmaz. Bekleyen
    // dosyanın diskteki metni zaten kişinin yazım dilindedir.
    async function readView(filePath, data) {
        if (!codedFormat.isCoded(data)) {
            return anyPending(filePath) ? data : readPlain(filePath, data);
        }
        const y = interpreter(filePath);
        const lang = g.writingLanguage();
        const s = y.error ? { code: -1, error: y.error } : await g.translate(y.filePath, 'göster', lang, data);
        if (s.code === 0) {
            resolvedLanguage.set(key(filePath), lang);
            forget('ham', filePath);
            return s.output;
        }
        once('ham', filePath, () => vscode.window.showWarningMessage(
            `${path.basename(filePath)} kodlu biçimde saklanıyor ama Türkçe görünümü açılamadı: ${codedFormat.errorSummary(s.error)} ` +
            'Dosya ham (kodlu) hâliyle gösteriliyor; kaydederseniz olduğu gibi yazılır. Hatayı düzeltip kaydettikten sonra dosyayı yeniden açın.'));
        return data;
    }

    // readPlain kodsuz (Türkçe) dosyanın kişinin yazım dilindeki gösterimidir. Üretilemezse
    // (sözdizimi hatası, yorumlayıcı yok) dosya Türkçe gösterilir ve öyle kaydedilir.
    async function readPlain(filePath, data) {
        const lang = g.writingLanguage();
        if (lang === SOURCE_LANGUAGE) {
            resolvedLanguage.set(key(filePath), lang);
            return data;
        }
        const y = interpreter(filePath);
        const s = y.error ? { code: -1, error: y.error } : await g.translate(y.filePath, 'göster', lang, data);
        if (s.code === 0) {
            resolvedLanguage.set(key(filePath), lang);
            forget('turkce', filePath);
            return s.output;
        }
        resolvedLanguage.set(key(filePath), SOURCE_LANGUAGE);
        once('turkce', filePath, () => vscode.window.showWarningMessage(
            `${path.basename(filePath)} Türkçe yazılmış ama "${lang}" yazımı açılamadı: ${codedFormat.errorSummary(s.error)} ` +
            'Dosya Türkçe gösteriliyor ve öyle kaydedilir. Hatayı düzeltip kaydettikten sonra dosyayı yeniden açın.'));
        return data;
    }

    // savePlain kişinin dilindeki metnin kodsuz dosyaya yazılacak Türkçesidir: önce kodlanır,
    // kodlu biçim Türkçe gösterilir. Çevrilemezse metnin kendisi (kayıpsız) yazılır ve dosya
    // Türkçeye çevrilmeyi bekler.
    async function savePlain(filePath, content, lang) {
        if (lang === SOURCE_LANGUAGE) {
            await setPending(filePath, false, PLAIN_PENDING_KEY);
            return content;
        }
        const y = interpreter(filePath);
        let s = y.error ? { code: -1, error: y.error } : await g.translate(y.filePath, 'kodla', lang, content);
        if (s.code === 0) {
            s = await g.translate(y.filePath, 'göster', SOURCE_LANGUAGE, s.output);
        }
        if (s.code === 0) {
            await setPending(filePath, false, PLAIN_PENDING_KEY);
            forget('bozuk', filePath);
            return s.output;
        }
        await setPending(filePath, true, PLAIN_PENDING_KEY);
        const name = path.basename(filePath);
        const reason = codedFormat.errorSummary(s.error);
        once('bozuk', filePath, () => {
            const kept = `Yazdıklarınız kaybolmadı: dosya şimdilik "${lang}" yazımıyla kaydedildi; `;
            if (s.code === codedFormat.EXIT_SYNTAX) {
                vscode.window.showInformationMessage(
                    `${name}: kodda sözdizimi hatası var (${reason}). ${kept}sözdizimi hatası düzelince ilk kayıtta yeniden Türkçe yazılacak.`);
            } else {
                vscode.window.showErrorMessage(
                    `${name} Türkçe yazılamadı: ${reason} ${kept}sorun giderilince ilk kayıtta yeniden Türkçe yazılacak.`);
            }
        });
        return content;
    }

    // saveText görünümdeki metnin diske yazılacak biçimidir. Kodlu dosyada kodlanabiliyorsa
    // kodlu, değilse metnin kendisi (kayıpsız) ve dosya kodlanmayı bekler; kodsuz dosyada
    // Türkçesi (savePlain).
    async function saveText(filePath, content, disk) {
        if (codedFormat.isCoded(content)) {
            // Metin zaten kodlu (ham gösterimde düzenlendi): olduğu gibi yazılır.
            await setPending(filePath, false);
            await setPending(filePath, false, PLAIN_PENDING_KEY);
            return content;
        }
        if (disk && !codedFormat.isCoded(disk) && !isPending(filePath)) {
            return savePlain(filePath, content, resolvedLanguage.get(key(filePath)) || g.writingLanguage());
        }
        if (!shouldEncode(filePath, disk)) {
            return content;
        }
        const y = interpreter(filePath);
        const lang = resolvedLanguage.get(key(filePath)) || g.writingLanguage();
        const s = y.error ? { code: -1, error: y.error } : await g.translate(y.filePath, 'kodla', lang, content);
        if (s.code === 0) {
            await setPending(filePath, false);
            forget('bozuk', filePath);
            return s.output;
        }
        await setPending(filePath, true);
        const name = path.basename(filePath);
        const reason = codedFormat.errorSummary(s.error);
        once('bozuk', filePath, () => {
            if (s.code === codedFormat.EXIT_SYNTAX) {
                vscode.window.showInformationMessage(
                    `${name}: kodda sözdizimi hatası var (${reason}). Yazdıklarınız kaybolmadı: dosya şimdilik Türkçe yazımıyla kaydedildi; ` +
                    'sözdizimi hatası düzelince ilk kayıtta yeniden kodlu yazılacak.');
            } else {
                vscode.window.showErrorMessage(
                    `${name} kodlu biçimde kaydedilemedi: ${reason} Yazdıklarınız kaybolmadı: dosya Türkçe yazımıyla kaydedildi; ` +
                    'sorun giderilince ilk kayıtta yeniden kodlu yazılacak.');
            }
        });
        return content;
    }

    // ---------------------------------------------------------------------------
    // Sekmeler: kodlu dosyanın ham sekmesi görünümle (kısıtlı kipte salt okunur ham gösterimle)
    // değiştirilir.
    // ---------------------------------------------------------------------------

    const inProgress = new Set();

    function sourceUri(tab) {
        const input = tab.input;
        if (!(input instanceof vscode.TabInputText)) {
            return undefined;
        }
        const uri = input.uri;
        const eligible = uri.scheme === 'file' || (uri.scheme === RAW_SCHEME && vscode.workspace.isTrusted);
        return eligible && path.extname(uri.fsPath).toLowerCase() === '.tr' ? uri : undefined;
    }

    function findOpenTab(group, uri) {
        return group.tabs.find((t) => t.input instanceof vscode.TabInputText && t.input.uri.toString() === uri.toString());
    }

    async function redirectTab(tab) {
        const uri = sourceUri(tab);
        if (!uri || !findOpenTab(tab.group, uri)) {
            return;
        }
        const k = uri.toString() + '#' + tab.group.viewColumn;
        if (inProgress.has(k)) {
            return;
        }
        inProgress.add(k);
        try {
            const document = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
            const kind = document && document.isDirty ? undefined : await viewKind(uri.fsPath);
            const trusted = vscode.workspace.isTrusted;
            // Kısıtlı kipte kodsuz dosya Türkçe, düzenlenebilir hâliyle kalır.
            if (!kind || (kind === 'plain' && (!trusted || uri.scheme !== 'file'))) {
                return;
            }
            const local = localUri(uri);
            if (trusted) {
                const y = g.findInterpreter(local);
                if (!y || y.error) {
                    const what = kind === 'plain' ? 'Türkçe yazılmış; yazım dilinizdeki görünümü' : 'kodlu biçimde saklanıyor; Türkçe görünümü';
                    once('yorumlayici', local.fsPath, () => g.interpreterNotFound(
                        `${path.basename(local.fsPath)} ${what} için TR yorumlayıcısı gerekir (${y ? y.error : 'bulunamadı'}). Dosya olduğu gibi açık kaldı.`));
                    return;
                }
            }
            await replaceTab(tab, uri, trusted ? viewUri(local) : rawUri(local));
            if (!trusted) {
                restrictedModeNotice(local.fsPath);
            }
        } finally {
            inProgress.delete(k);
        }
    }

    // waitForSelection düzenleyicinin açılış seçimini verir. VS Code bir dosyayı seçimle açarken
    // (hata bağlantısı "dosya.tr:12:5") düzenleyiciyi önce dosyanın başında gösterir, seçimi
    // hemen ardından uygular: seçim başta ise kısa bir süre seçim değişikliği beklenir.
    function waitForSelection(editor) {
        const s = editor.selection;
        if (s.start.line !== 0 || s.start.character !== 0 || !s.isEmpty) {
            return Promise.resolve(s);
        }
        return new Promise((resolve) => {
            const finish = () => {
                clearTimeout(timer);
                listener.dispose();
                resolve(editor.selection);
            };
            const listener = vscode.window.onDidChangeTextEditorSelection((e) => {
                if (e.textEditor === editor) {
                    finish();
                }
            });
            const timer = setTimeout(finish, 150);
        });
    }

    // replaceTab hedefi aynı grupta (önizleme durumu ve seçim korunarak) açar, eski sekmeyi
    // kapatır. Eski belge kirli değildir; kapatmak hiçbir şey kaybettirmez.
    async function replaceTab(tab, uri, target) {
        const group = tab.group;
        const editor = vscode.window.visibleTextEditors.find((e) => e.document.uri.toString() === uri.toString() && e.viewColumn === group.viewColumn);
        await vscode.window.showTextDocument(target, {
            viewColumn: group.viewColumn,
            preview: tab.isPreview,
            preserveFocus: !(tab.isActive && group.isActive),
            selection: editor ? await waitForSelection(editor) : undefined
        });
        const oldTab = findOpenTab(group, uri);
        if (oldTab) {
            try {
                await vscode.window.tabGroups.close(oldTab, true);
            } catch (e) {
                // sekme bu arada kapanmış olabilir
            }
        }
    }

    function restrictedModeNotice(filePath) {
        once('kisitli', filePath, async () => {
            const trustLabel = 'Çalışma Alanına Güven';
            const choice = await vscode.window.showInformationMessage(
                `${path.basename(filePath)} kodlu biçimde saklanıyor (dilin sözcükleri kodlarla). Kısıtlı kipte TR yorumlayıcısı çalıştırılmadığı için ` +
                'Türkçe görünüm açılamıyor; dosya salt okunur, ham hâliyle gösteriliyor. Çalışma alanına güvenirseniz Türkçe açılır.', trustLabel);
            if (choice === trustLabel) {
                vscode.commands.executeCommand('workbench.trust.manage');
            }
        });
    }

    function allTabs() {
        return vscode.window.tabGroups.all.flatMap((group) => group.tabs);
    }

    function redirectAll() {
        return Promise.all(allTabs().map(redirectTab));
    }

    // openRaw etkin .tr dosyasını salt okunur ham (kodlu) hâliyle yanda açar.
    async function openRaw() {
        const editor = vscode.window.activeTextEditor;
        const local = editor ? localUri(editor.document.uri) : undefined;
        if (!local || path.extname(local.fsPath).toLowerCase() !== '.tr') {
            vscode.window.showErrorMessage('Kodlu hâlini göstermek için bir .tr dosyası açın.');
            return;
        }
        await vscode.window.showTextDocument(rawUri(local), { viewColumn: vscode.ViewColumn.Beside, preview: false });
    }

    // ---------------------------------------------------------------------------
    // Hata ayıklama: program diskteki dosyadır; "viewURI" görünümün adresidir. 'tr dap'
    // çağrı yığınını bu adresle bildirir, kesme noktaları bu adresle gelir; satırlar görünümün
    // satırlarıdır (başlık sayılmaz, 17.5).
    // ---------------------------------------------------------------------------

    // openView bir dosyanın açık Türkçe görünüm belgesidir (yoksa undefined).
    function openView(uri) {
        const local = localUri(uri);
        return local && vscode.workspace.textDocuments.find((b) => b.uri.scheme === VIEW_SCHEME && key(b.uri.fsPath) === key(local.fsPath));
    }

    // withViewURI belge Türkçe görünümse başlatma ayarına görünümün adresini ekler.
    function withViewURI(y, document) {
        if (document.uri.scheme === VIEW_SCHEME && !y.viewURI) {
            y.viewURI = document.uri.toString();
        }
        return y;
    }

    // FILE_VARIABLES VS Code'un etkin dosyadan çözdüğü değişkenlerdir. VS Code bunları
    // yalnızca diskteki dosyalarda çözer; görünüm etkinse burada dosyanın kendisinden çözülür
    // (launch.json'daki "program": "${file}" görünümde de çalışsın).
    const FILE_VARIABLES = {
        file: (p) => p,
        fileDirname: (p) => path.dirname(p),
        fileBasename: (p) => path.basename(p),
        fileBasenameNoExtension: (p) => path.basename(p, path.extname(p)),
        fileExtname: (p) => path.extname(p)
    };

    function resolveFileVariables(y, document) {
        if (document.uri.scheme !== VIEW_SCHEME) {
            return y;
        }
        const file = localPath(document.uri);
        const resolve = (d) => (typeof d === 'string'
            ? d.replace(/\$\{(\w+)\}/g, (all, name) => (FILE_VARIABLES[name] ? FILE_VARIABLES[name](file) : all))
            : d);
        const result = Object.assign({}, y, { program: resolve(y.program), cwd: resolve(y.cwd) });
        if (Array.isArray(y.args)) {
            result.args = y.args.map(resolve);
        }
        return typeof result.program === 'string' && key(result.program) === key(file) ? withViewURI(result, document) : result;
    }

    // programView başlatma ayarındaki program görünümle açılan bir dosyaysa (kodlu, bekleyen ya
    // da başka bir yazım dilinde kodsuz) görünümün adresini ekler: program durunca satır
    // görünümde işaretlenir.
    async function programView(y) {
        if (y && !y.viewURI && typeof y.program === 'string' && y.program && await opensInView(y.program)) {
            y.viewURI = viewUri(vscode.Uri.file(y.program)).toString();
        }
        return y;
    }

    // reloadForLanguage yazım dili değişince açık görünümleri yeni dille yeniler. Kirli belge
    // yenilenmez (yazılanlar kaybolmasın): eski dille kodlanarak kaydedilir ve kişi uyarılır.
    function reloadForLanguage() {
        for (const document of vscode.workspace.textDocuments) {
            if (document.uri.scheme !== VIEW_SCHEME) {
                continue;
            }
            const filePath = localPath(document.uri);
            if (document.isDirty) {
                once('dil', filePath, () => vscode.window.showWarningMessage(
                    `${path.basename(filePath)} kaydedilmemiş değişiklikler içeriyor; yazım dili değişti ama belge ` +
                    `"${resolvedLanguage.get(key(filePath))}" diliyle gösterilmeye devam ediyor ve o dille kaydedilecek. ` +
                    'Yeni dilde görmek için kaydedip dosyayı yeniden açın.'));
                continue;
            }
            revisions.set(key(filePath), revision(filePath) + 1);
            filesystems.notifyChanged(document.uri);
        }
    }

    function setup(context) {
        context.subscriptions.push(
            vscode.workspace.onDidChangeConfiguration((e) => {
                if (e.affectsConfiguration('tr.yazimDili')) {
                    forgetDirtyNotices();
                    reloadForLanguage();
                    // Kodsuz dosyalar başka bir yazım dilinde görünümle açılır.
                    redirectAll();
                }
            }),
            vscode.workspace.registerFileSystemProvider(VIEW_SCHEME, filesystems.view, { isCaseSensitive: !CASE_INSENSITIVE }),
            vscode.workspace.registerFileSystemProvider(RAW_SCHEME, filesystems.raw, { isCaseSensitive: !CASE_INSENSITIVE, isReadonly: true }),
            // Görünen düzenleyiciler hemen (seçimleriyle). Yeni sekmeler kısa bir süre sonra:
            // VS Code sekmeyi düzenleyicisinden ve seçiminden önce bildirir; görünen sekmeyi
            // o arada düzenleyiciler olayı yönlendirmiş olur, bu yalnızca arka plandakiler içindir.
            vscode.window.tabGroups.onDidChangeTabs((e) => {
                for (const tab of e.opened) {
                    setTimeout(() => redirectTab(tab), 150);
                }
            }),
            vscode.window.onDidChangeVisibleTextEditors((editors) => {
                for (const editor of editors) {
                    const group = vscode.window.tabGroups.all.find((g2) => g2.viewColumn === editor.viewColumn);
                    const tab = group && findOpenTab(group, editor.document.uri);
                    if (tab) {
                        redirectTab(tab);
                    }
                }
            }),
            vscode.workspace.onDidGrantWorkspaceTrust(redirectAll),
            vscode.commands.registerCommand('tr.kodluHaliniGoster', openRaw)
        );
        return redirectAll();
    }

    return {
        VIEW_SCHEME, RAW_SCHEME, setup, viewUri, localUri, opensInView, openView,
        withViewURI, resolveFileVariables, programView,
        view: filesystems.view, raw: filesystems.raw
    };
}

module.exports = { create, VIEW_SCHEME, RAW_SCHEME, PENDING_KEY, PLAIN_PENDING_KEY };
