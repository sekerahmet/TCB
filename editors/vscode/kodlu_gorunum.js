// Kodlu dosyaların Türkçe görünümü (docs/dil/17_kodlar.md, "Düzenleyicide"): diskte kodlu saklanan bir
// .tr dosyası düzenleyicide kişinin yazım dilinde açılır, düzenlenir ve kaydedilince diske yine
// kodlu yazılır.
//
// Mekanizma: ayrı bir dosya sistemi şeması (GORUNUM_SEMASI). Görünümün adresi dosyanın kendi
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
// göredir (17.5). Kodlu olmayan dosyalara dokunulmaz; eklenti kendiliğinden kodlamaz.
//
// Kısıtlı kipte (güvenilmeyen çalışma alanı) hiçbir şey çalıştırılmaz: kodlu dosya salt okunur
// ham hâliyle (HAM_SEMASI) gösterilir ve nedeni söylenir; güven verilince görünüme geçilir.
'use strict';

const fs = require('fs');
const path = require('path');
const kodluBicim = require('./kodlu_bicim');
const kodluDosyaSistemi = require('./kodlu_dosya_sistemi');

const GORUNUM_SEMASI = 'tr-kodlu';
const HAM_SEMASI = 'tr-ham';
// BEKLEYEN_ANAHTARI kodlanmayı bekleyen (ayrıştırılamadığı için Türkçe kaydedilmiş) dosyaların
// kalıcı listesinin anahtarıdır; VS Code yeniden açılınca da bilinir.
const BEKLEYEN_ANAHTARI = 'tr.kodlanmayiBekleyenDosyalar';
const BUYUK_KUCUK_DUYARSIZ = process.platform === 'win32' || process.platform === 'darwin';

// olustur görünümün parçalarını kurar.
//   g.yorumlayiciyiBul  (dosyaUri) -> { yol } | { hata } | undefined   (yorumlayici_bul.js kuralı)
//   g.cevir             kodluBicim.cevir ile aynı imza
//   g.durum             kalıcı durum (ExtensionContext.globalState)
//   g.yazimDili         () -> yazım dili kodu ('tr.yazimDili' ayarı)
//   g.yorumlayiciBulunamadi (mesaj) -> ayarı açma düğmeli hata mesajı
function olustur(vscode, g) {
    const bildirilen = new Set(); // bu oturumda bir kez söylenen durumlar (anahtar + dosya)

    function anahtar(yol) {
        const tam = path.resolve(yol);
        return BUYUK_KUCUK_DUYARSIZ ? tam.toLowerCase() : tam;
    }
    function bekleyenler() {
        return new Set(g.durum.get(BEKLEYEN_ANAHTARI, []));
    }
    function bekliyor(yol) {
        return bekleyenler().has(anahtar(yol));
    }
    async function bekleyenYap(yol, bekleyen) {
        const b = bekleyenler();
        if (b.has(anahtar(yol)) === bekleyen) {
            return;
        }
        if (bekleyen) {
            b.add(anahtar(yol));
        } else {
            b.delete(anahtar(yol));
        }
        await g.durum.update(BEKLEYEN_ANAHTARI, [...b]);
    }
    // birKez mesajı bu oturumda (tur, dosya) için yalnızca bir kez gösterir.
    function birKez(tur, yol, goster) {
        const k = tur + '\n' + anahtar(yol);
        if (!bildirilen.has(k)) {
            bildirilen.add(k);
            goster();
        }
    }
    function unut(tur, yol) {
        bildirilen.delete(tur + '\n' + anahtar(yol));
    }

    function gorunumUri(dosyaUri) {
        return dosyaUri.with({ scheme: GORUNUM_SEMASI });
    }
    function hamUri(dosyaUri) {
        return dosyaUri.with({ scheme: HAM_SEMASI });
    }
    // yerelUri bir belge adresinin diskteki dosyasıdır (görünümde ve ham gösterimde dosyanın
    // kendisi); yerel olmayan adreslerde undefined.
    function yerelUri(uri) {
        if (!uri) {
            return undefined;
        }
        if (uri.scheme === 'file') {
            return uri;
        }
        return uri.scheme === GORUNUM_SEMASI || uri.scheme === HAM_SEMASI ? uri.with({ scheme: 'file' }) : undefined;
    }
    function yerelYol(uri) {
        return yerelUri(uri).fsPath;
    }

    // kodlanacakMi diskteki dosya görünümden kaydedilince kodlu yazılır mı: dosya kodluysa ya
    // da kodlanmayı bekliyorsa (ya da henüz yoksa: görünüm yalnızca kodlu dosyalar için açılır).
    function kodlanacakMi(yol, disk) {
        return !disk || kodluBicim.kodluMu(disk) || bekliyor(yol);
    }

    // kodluDosyaMi dosya görünümle açılmalı mı (kodlu ya da kodlanmayı bekliyor)?
    async function kodluDosyaMi(yol) {
        if (bekliyor(yol)) {
            return true;
        }
        let tutamak;
        try {
            tutamak = await fs.promises.open(yol, 'r');
            // Başlık (BOM ve CRLF ile 19 bayt) ilk 64 baytın içindedir.
            const bas = Buffer.alloc(64);
            const { bytesRead } = await tutamak.read(bas, 0, bas.length, 0);
            return kodluBicim.kodluMu(bas.subarray(0, bytesRead));
        } catch (e) {
            return false;
        } finally {
            if (tutamak) {
                await tutamak.close();
            }
        }
    }

    // yorumlayici çeviri için yorumlayıcıyı verir; kısıtlı kipte hiçbir şey çalıştırılmaz.
    function yorumlayici(yol) {
        if (!vscode.workspace.isTrusted) {
            return { hata: 'Kısıtlı kipte TR yorumlayıcısı çalıştırılmaz (Çalışma Alanı Güveni).' };
        }
        return g.yorumlayiciyiBul(vscode.Uri.file(yol)) || { hata: 'TR yorumlayıcısı bulunamadı.' };
    }

    const sistem = kodluDosyaSistemi.olustur(vscode, { GORUNUM_SEMASI, HAM_SEMASI, yerelYol, gorunumOku, kayitMetni });

    // gorunumOku kodlu dosyanın görünümde gösterilecek metnidir. Türkçesi üretilemezse (dosya
    // ayrıştırılamıyor, ör. birleştirme çakışması işaretleri; ya da yorumlayıcı yok) ham baytlar
    // gösterilir: kaydedilince olduğu gibi yazılır, hiçbir şey kaybolmaz.
    async function gorunumOku(yol, veri) {
        if (!kodluBicim.kodluMu(veri)) {
            return veri;
        }
        const y = yorumlayici(yol);
        const s = y.hata ? { kod: -1, hata: y.hata } : await g.cevir(y.yol, 'göster', g.yazimDili(), veri);
        if (s.kod === 0) {
            unut('ham', yol);
            return s.cikti;
        }
        birKez('ham', yol, () => vscode.window.showWarningMessage(
            `${path.basename(yol)} kodlu biçimde saklanıyor ama Türkçe görünümü açılamadı: ${kodluBicim.hataOzeti(s.hata)} ` +
            'Dosya ham (kodlu) hâliyle gösteriliyor; kaydederseniz olduğu gibi yazılır. Hatayı düzeltip kaydettikten sonra dosyayı yeniden açın.'));
        return veri;
    }

    // kayitMetni görünümdeki metnin diske yazılacak biçimidir: kodlanabiliyorsa kodlu,
    // değilse metnin kendisi (kayıpsız) ve dosya kodlanmayı bekler.
    async function kayitMetni(yol, icerik, disk) {
        if (kodluBicim.kodluMu(icerik)) {
            // Metin zaten kodlu (ham gösterimde düzenlendi): olduğu gibi yazılır.
            await bekleyenYap(yol, false);
            return icerik;
        }
        if (!kodlanacakMi(yol, disk)) {
            return icerik;
        }
        const y = yorumlayici(yol);
        const s = y.hata ? { kod: -1, hata: y.hata } : await g.cevir(y.yol, 'kodla', g.yazimDili(), icerik);
        if (s.kod === 0) {
            await bekleyenYap(yol, false);
            unut('bozuk', yol);
            return s.cikti;
        }
        await bekleyenYap(yol, true);
        const ad = path.basename(yol);
        const neden = kodluBicim.hataOzeti(s.hata);
        birKez('bozuk', yol, () => {
            if (s.kod === kodluBicim.CIKIS_SOZDIZIMI) {
                vscode.window.showInformationMessage(
                    `${ad}: kodda sözdizimi hatası var (${neden}). Yazdıklarınız kaybolmadı: dosya şimdilik Türkçe yazımıyla kaydedildi; ` +
                    'sözdizimi hatası düzelince ilk kayıtta yeniden kodlu yazılacak.');
            } else {
                vscode.window.showErrorMessage(
                    `${ad} kodlu biçimde kaydedilemedi: ${neden} Yazdıklarınız kaybolmadı: dosya Türkçe yazımıyla kaydedildi; ` +
                    'sorun giderilince ilk kayıtta yeniden kodlu yazılacak.');
            }
        });
        return icerik;
    }

    // ---------------------------------------------------------------------------
    // Sekmeler: kodlu dosyanın ham sekmesi görünümle (kısıtlı kipte salt okunur ham gösterimle)
    // değiştirilir.
    // ---------------------------------------------------------------------------

    const isleniyor = new Set();

    function kaynakAdresi(sekme) {
        const giris = sekme.input;
        if (!(giris instanceof vscode.TabInputText)) {
            return undefined;
        }
        const uri = giris.uri;
        const uygun = uri.scheme === 'file' || (uri.scheme === HAM_SEMASI && vscode.workspace.isTrusted);
        return uygun && path.extname(uri.fsPath).toLowerCase() === '.tr' ? uri : undefined;
    }

    function sekmeAcik(grup, uri) {
        return grup.tabs.find((t) => t.input instanceof vscode.TabInputText && t.input.uri.toString() === uri.toString());
    }

    async function sekmeyiYonlendir(sekme) {
        const uri = kaynakAdresi(sekme);
        if (!uri || !sekmeAcik(sekme.group, uri)) {
            return;
        }
        const k = uri.toString() + '#' + sekme.group.viewColumn;
        if (isleniyor.has(k)) {
            return;
        }
        isleniyor.add(k);
        try {
            const belge = vscode.workspace.textDocuments.find((b) => b.uri.toString() === uri.toString());
            if ((belge && belge.isDirty) || !(await kodluDosyaMi(uri.fsPath))) {
                return;
            }
            const yerel = yerelUri(uri);
            const guvenilir = vscode.workspace.isTrusted;
            if (guvenilir) {
                const y = g.yorumlayiciyiBul(yerel);
                if (!y || y.hata) {
                    birKez('yorumlayici', yerel.fsPath, () => g.yorumlayiciBulunamadi(
                        `${path.basename(yerel.fsPath)} kodlu biçimde saklanıyor; Türkçe görünümü için TR yorumlayıcısı gerekir (${y ? y.hata : 'bulunamadı'}). Dosya ham hâliyle açık kaldı.`));
                    return;
                }
            }
            await sekmeyiDegistir(sekme, uri, guvenilir ? gorunumUri(yerel) : hamUri(yerel));
            if (!guvenilir) {
                kisitliKipBilgisi(yerel.fsPath);
            }
        } finally {
            isleniyor.delete(k);
        }
    }

    // secimBekle düzenleyicinin açılış seçimini verir. VS Code bir dosyayı seçimle açarken
    // (hata bağlantısı "dosya.tr:12:5") düzenleyiciyi önce dosyanın başında gösterir, seçimi
    // hemen ardından uygular: seçim başta ise kısa bir süre seçim değişikliği beklenir.
    function secimBekle(editor) {
        const s = editor.selection;
        if (s.start.line !== 0 || s.start.character !== 0 || !s.isEmpty) {
            return Promise.resolve(s);
        }
        return new Promise((coz) => {
            const bitir = () => {
                clearTimeout(zaman);
                dinleyici.dispose();
                coz(editor.selection);
            };
            const dinleyici = vscode.window.onDidChangeTextEditorSelection((e) => {
                if (e.textEditor === editor) {
                    bitir();
                }
            });
            const zaman = setTimeout(bitir, 150);
        });
    }

    // sekmeyiDegistir hedefi aynı grupta (önizleme durumu ve seçim korunarak) açar, eski sekmeyi
    // kapatır. Eski belge kirli değildir; kapatmak hiçbir şey kaybettirmez.
    async function sekmeyiDegistir(sekme, uri, hedef) {
        const grup = sekme.group;
        const editor = vscode.window.visibleTextEditors.find((e) => e.document.uri.toString() === uri.toString() && e.viewColumn === grup.viewColumn);
        await vscode.window.showTextDocument(hedef, {
            viewColumn: grup.viewColumn,
            preview: sekme.isPreview,
            preserveFocus: !(sekme.isActive && grup.isActive),
            selection: editor ? await secimBekle(editor) : undefined
        });
        const eski = sekmeAcik(grup, uri);
        if (eski) {
            try {
                await vscode.window.tabGroups.close(eski, true);
            } catch (e) {
                // sekme bu arada kapanmış olabilir
            }
        }
    }

    function kisitliKipBilgisi(yol) {
        birKez('kisitli', yol, async () => {
            const guven = 'Çalışma Alanına Güven';
            const secim = await vscode.window.showInformationMessage(
                `${path.basename(yol)} kodlu biçimde saklanıyor (dilin sözcükleri kodlarla). Kısıtlı kipte TR yorumlayıcısı çalıştırılmadığı için ` +
                'Türkçe görünüm açılamıyor; dosya salt okunur, ham hâliyle gösteriliyor. Çalışma alanına güvenirseniz Türkçe açılır.', guven);
            if (secim === guven) {
                vscode.commands.executeCommand('workbench.trust.manage');
            }
        });
    }

    function tumSekmeler() {
        return vscode.window.tabGroups.all.flatMap((grup) => grup.tabs);
    }

    function hepsiniYonlendir() {
        return Promise.all(tumSekmeler().map(sekmeyiYonlendir));
    }

    // hamAc etkin .tr dosyasını salt okunur ham (kodlu) hâliyle yanda açar.
    async function hamAc() {
        const editor = vscode.window.activeTextEditor;
        const yerel = editor ? yerelUri(editor.document.uri) : undefined;
        if (!yerel || path.extname(yerel.fsPath).toLowerCase() !== '.tr') {
            vscode.window.showErrorMessage('Kodlu hâlini göstermek için bir .tr dosyası açın.');
            return;
        }
        await vscode.window.showTextDocument(hamUri(yerel), { viewColumn: vscode.ViewColumn.Beside, preview: false });
    }

    // ---------------------------------------------------------------------------
    // Hata ayıklama: program diskteki dosyadır; "gorunumAdresi" görünümün adresidir. 'tr dap'
    // çağrı yığınını bu adresle bildirir, kesme noktaları bu adresle gelir; satırlar görünümün
    // satırlarıdır (başlık sayılmaz, 17.5).
    // ---------------------------------------------------------------------------

    // acikGorunum bir dosyanın açık Türkçe görünüm belgesidir (yoksa undefined).
    function acikGorunum(uri) {
        const yerel = yerelUri(uri);
        return yerel && vscode.workspace.textDocuments.find((b) => b.uri.scheme === GORUNUM_SEMASI && anahtar(b.uri.fsPath) === anahtar(yerel.fsPath));
    }

    // gorunumAdresiyle belge Türkçe görünümse başlatma ayarına görünümün adresini ekler.
    function gorunumAdresiyle(y, belge) {
        if (belge.uri.scheme === GORUNUM_SEMASI && !y.gorunumAdresi) {
            y.gorunumAdresi = belge.uri.toString();
        }
        return y;
    }

    // DOSYA_DEGISKENLERI VS Code'un etkin dosyadan çözdüğü değişkenlerdir. VS Code bunları
    // yalnızca diskteki dosyalarda çözer; görünüm etkinse burada dosyanın kendisinden çözülür
    // (launch.json'daki "program": "${file}" görünümde de çalışsın).
    const DOSYA_DEGISKENLERI = {
        file: (p) => p,
        fileDirname: (p) => path.dirname(p),
        fileBasename: (p) => path.basename(p),
        fileBasenameNoExtension: (p) => path.basename(p, path.extname(p)),
        fileExtname: (p) => path.extname(p)
    };

    function dosyaDegiskenleriniCoz(y, belge) {
        if (belge.uri.scheme !== GORUNUM_SEMASI) {
            return y;
        }
        const dosya = yerelYol(belge.uri);
        const coz = (d) => (typeof d === 'string'
            ? d.replace(/\$\{(\w+)\}/g, (tum, ad) => (DOSYA_DEGISKENLERI[ad] ? DOSYA_DEGISKENLERI[ad](dosya) : tum))
            : d);
        const sonuc = Object.assign({}, y, { program: coz(y.program), cwd: coz(y.cwd) });
        if (Array.isArray(y.args)) {
            sonuc.args = y.args.map(coz);
        }
        return typeof sonuc.program === 'string' && anahtar(sonuc.program) === anahtar(dosya) ? gorunumAdresiyle(sonuc, belge) : sonuc;
    }

    // programinGorunumu başlatma ayarındaki program kodlu bir dosyaysa (ya da kodlanmayı
    // bekliyorsa) görünümün adresini ekler: program durunca satır görünümde işaretlenir.
    async function programinGorunumu(y) {
        if (y && !y.gorunumAdresi && typeof y.program === 'string' && y.program && await kodluDosyaMi(y.program)) {
            y.gorunumAdresi = gorunumUri(vscode.Uri.file(y.program)).toString();
        }
        return y;
    }

    function kur(context) {
        context.subscriptions.push(
            vscode.workspace.registerFileSystemProvider(GORUNUM_SEMASI, sistem.gorunum, { isCaseSensitive: !BUYUK_KUCUK_DUYARSIZ }),
            vscode.workspace.registerFileSystemProvider(HAM_SEMASI, sistem.ham, { isCaseSensitive: !BUYUK_KUCUK_DUYARSIZ, isReadonly: true }),
            // Görünen düzenleyiciler hemen (seçimleriyle). Yeni sekmeler kısa bir süre sonra:
            // VS Code sekmeyi düzenleyicisinden ve seçiminden önce bildirir; görünen sekmeyi
            // o arada düzenleyiciler olayı yönlendirmiş olur, bu yalnızca arka plandakiler içindir.
            vscode.window.tabGroups.onDidChangeTabs((e) => {
                for (const sekme of e.opened) {
                    setTimeout(() => sekmeyiYonlendir(sekme), 150);
                }
            }),
            vscode.window.onDidChangeVisibleTextEditors((editorler) => {
                for (const editor of editorler) {
                    const grup = vscode.window.tabGroups.all.find((g2) => g2.viewColumn === editor.viewColumn);
                    const sekme = grup && sekmeAcik(grup, editor.document.uri);
                    if (sekme) {
                        sekmeyiYonlendir(sekme);
                    }
                }
            }),
            vscode.workspace.onDidGrantWorkspaceTrust(hepsiniYonlendir),
            vscode.commands.registerCommand('tr.kodluHaliniGoster', hamAc)
        );
        return hepsiniYonlendir();
    }

    return {
        GORUNUM_SEMASI, HAM_SEMASI, kur, gorunumUri, yerelUri, kodluDosyaMi, acikGorunum,
        gorunumAdresiyle, dosyaDegiskenleriniCoz, programinGorunumu,
        gorunum: sistem.gorunum, ham: sistem.ham
    };
}

module.exports = { olustur, GORUNUM_SEMASI, HAM_SEMASI, BEKLEYEN_ANAHTARI };
