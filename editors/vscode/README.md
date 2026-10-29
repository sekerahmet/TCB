# TR Stüdyo — VS Code Eklentisi

`.tr` dosyaları için sözdizimi renklendirme, kod şablonları ve çalıştırma/hata ayıklama komutları.

## Kurulum

1. Önce TR yorumlayıcısını proje kökünde derleyin:

   ```bash
   go build -o bin/tr.exe ./cmd/tr
   ```

   Derlenen yorumlayıcı `bin/` klasörüne yazılır (bu klasör depoya girmez). Her x86-64 işlemcide çalışır; AVX2/FMA çekirdekleri çalışma anında seçilir.

2. Bu klasörü (`editors/vscode`) `%USERPROFILE%\.vscode\extensions\tr-dili-vscode` olarak kopyalayın:

   ```powershell
   Copy-Item -Recurse -Force editors\vscode "$env:USERPROFILE\.vscode\extensions\tr-dili-vscode"
   ```

3. VS Code'u yeniden başlatın.

## Komutlar

| Komut | Kimlik | Çalıştırılan |
| --- | --- | --- |
| TR: Dosyayı Çalıştır | `tr.run` | `tr.exe <dosya>` |
| TR: Hata Ayıkla (VS Code) | `tr.debugStart` | VS Code hata ayıklayıcısı (`tr.exe dap`) |
| TR: Dosyayı Denetle | `tr.check` | `tr.exe kontrol <dosya>` |
| TR: Terminal Hata Ayıklayıcısında Aç | `tr.debug` | `tr.exe hata_ayıkla <dosya>` |
| TR: Tablo Görünümü | `tr.showTable` | Hata ayıklarken değişkeni tablo olarak gösterir |
| TR: İzleme Masası | `tr.watchPanel` | Hata ayıklarken tüm değişkenleri tablo hâlinde gösteren paneli açar |
| TR: Görünümü Göster (salt okunur) | `tr.showView` | Açık dosyanın yazım dilinizdeki görünümünü yanda salt okunur açar (`tr.exe göster - --dil <dil>`) |

- Komut başlıkları VS Code'un gösterim dilini izler (`Configure Display Language`): Türkçe dil paketiyle yukarıdaki Türkçe başlıklar, aksi hâlde İngilizceleri görünür (`TR: Run File`, `TR: Check File` ...). Komut, ayar ve görev kimlikleri her dilde aynıdır ve İngilizcedir; `tasks.json`'da `{ "type": "tr", "command": "tr.run", "file": "${file}" }` yazılır.
- Komutlar dosyayı önce kaydeder, sonra terminal panelinde çalıştırır. Hata ayıklayıcı komutlarını bu terminale yazabilirsiniz.
- Çalışma dizini, dosyanın bulunduğu çalışma alanı klasörüdür; klasör açık değilse dosyanın dizinidir.
- Düzenleyicinin başlık çubuğundaki **▶** düğmesi dosyayı çalıştırır, yanındaki **🐞** düğmesi hata ayıklamayı başlatır. Komutlar sağ tık menüsünde ve Komut Paleti'nde (`Ctrl+Shift+P`, "TR:") de bulunur.

## Hata ayıklama

Tuşlar terminal hata ayıklayıcısında da (`TR_cmd_debug`) aynı anlamdadır: F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık (terminalde tuş adı yazılıp Enter'a basılır). **F5** hata ayıklayıcıyı açık `.tr` dosyasında başlatır; **Ctrl+F5** hata ayıklamadan terminalde çalıştırır (`TR_fn_input()` kullanan programlar için). Denemek için depodaki `examples/tr/43_hata_ayiklama_alistirmasi.tr` (adım adım alıştırma) ya da `examples/tr/42_hata_ayiklama.tr` (koddaki koşullu `TR_kw_breakpoint`) dosyasını açıp F5'e basın.

**İzleme Masası:** Program her durduğunda editörün yanında açılır.

- **Yönlendirme düğmeleri:** ⤼ Sonraki satır (F10), ↓ İçine gir (F11), ↑ Dışarı çık (Shift+F11), ▶ Devam (F5), ■ Durdur. Düğmeler panelin en üstünde sabit durur (konum metni uzayıp kısalınca yer değiştirmez) ve fareye basıldığı anda çalışır. Program çalışırken adım düğmeleri pasif görünür. Altlarındaki satır konumu ve duruş nedenini gösterir; **N. duruş** sayacı, Devam döngüdeki aynı kesme noktasında yeniden durduğunda da programın ilerlediğini gösterir.
- **Değişkenler:** Tüm değişkenler tek tabloda görünür (Kapsam, Değişken, Tip, Değer). Her değişkenin yanında türünü gösteren bir ikon vardır: ▦ tablo, ☰ liste, { } sözlük, ◆ yapı, ▣ tensör, # sayı, Aa metin, ◐ mantık, ∅ yok. Son adımda değişen değerler vurgulanır; ✏️ ile değer doğrudan değiştirilir. Üstteki kutuyla değişkenler aranır.
- **Tablolar:** Tablo değerlerinin içeriği satıra dökülmez. Değişkenin adına ya da **▸ Tabloyu aç** linkine tıklayınca (veya satıra çift tıklayınca) tablo alttaki pencerede bir **sekme** olarak açılır. Birden çok tablo sekmeler hâlinde açık kalabilir; her sekme kendi **✕** düğmesiyle, hepsi **Tümünü kapat** ile kapanır. Pencere, üstündeki çubuk sürüklenerek büyütülüp küçültülür. Sütun başlığına tıklayınca sıralanır, arama kutusuyla süzülür, hücreler arasında ok tuşlarıyla gezilir. Adım atıldıkça açık tablolar güncellenir.
- **Yapay zekâ nesneleri:** Tensörler tip sütununda `Tensör [2×3]` olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılır. Skaler tensörün (ör. kayıp) değeri doğrudan görünür: `0.6074 (Tensör, skaler)`; 1-8 elemanlı küçük tensörler satır içinde kısaltılmış yazılır (`[[1.4298]] (Tensör, 1×1)`), daha büyükleri **▸ Tabloyu aç** ile açılır. float32 dışındaki türlerde tür de yazılır (`2.5000 (Tensör, skaler, bf16)`). Variables panelinde ve Debug Console'da da aynı değer görünür. Parçalayıcı, veri kümesi ve örnekleyici özetleriyle görünür (`Parçalayıcı (dağarcık 40000)`, `Veri kümesi (246 veri dosyası, 56,9 milyar parça)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)`); parçalayıcının eklenmiş/özel parçaları ve veri kümesinin veri dosyaları tablo olarak açılır.
- İstemiyorsanız `tr.autoOpenWatchPanel` ayarını kapatın.

## Kodlu dosyalar

TR dosyaları diskte de düzenleyicide de dilden bağımsız kimliklerle yazılır: ilk satır `# TR 4`, dilin sözcükleri `TR_if`, `TR_print` gibi kimlikler; dosyanın dili yoktur (dil tanımı 17.7). Dosya olduğu gibi açılır, düzenlenir ve kaydedilir; eklenti dosyayı çevirmez.

- **Görünüm:** Komut Paleti → "TR: Görünümü Göster" (ya da düzenleyicinin başlık çubuğundaki düğme) dosyanın **yazım dilinizdeki** (`tr.writingLanguage`) görünümünü yanda **salt okunur** açar. Görünümü `tr göster -` üretir; eklentide ikinci bir çevirici yoktur. Başlık satırı görünümde `# TR 4 view=tr` olur; satır numaraları dosyanınkiyle, hata iletileri ve hata ayıklayıcıyla birebir aynıdır. Görünümün adresi dosyanın kendi yoludur (şeması `tr-view:`).
- **Görünüm yalnız okunur:** düzenlenmez, kaydedilmez ve koda geri çevrilmez. Dosyayı düzenledikçe (kaydetmeden de) görünüm kısa bir beklemeyle yenilenir; yazım dilini değiştirince açık görünümler yeni dille yenilenir. Dosya o an ayrıştırılamıyorsa (yarım kod) görünüm son metninde kalır ve nedeni bir kez söylenir.
- **Görünümden çalıştırma ve hata ayıklama:** görünüm etkinken ▶, F5 ve öbür komutlar dosyanın kendisinde çalışır.
- **Görünüm dosyası çalışmaz:** `tr göster`'in çıktısını bir `.tr` dosyasına kaydedip çalıştırırsanız ileti kodlu dosyayı çalıştırmayı söyler.
- **Kısıtlı kip:** Güvenilmeyen çalışma alanında yorumlayıcı çalıştırılmadığından görünüm açılmaz ve nedeni söylenir; dosya yine okunup düzenlenebilir.
- **Git farkları:** `git diff`'in Türkçe fark göstermesi için `TR_cmd_show`'u textconv olarak tanıtın ([kılavuz](../../docs/guide/01_baslangic.md#kodlu-dosyalar-düzenleyici-ve-git-farkları)). VS Code'un kaynak denetimi görünümündeki farklar kodlu metni gösterir.

## Kod şablonları

`.tr` dosyasında kısayolu yazın, öneri listesinden şablonu seçip `Tab` ile durak durak doldurun. Kısayollar ve durakların varsayılan sözcükleri **yazım dilinizdedir** (`tr.writingLanguage`): Türkçede `fonksiyon`, `eğer`, `eşle`, `döngü`, `liste_üret`, `yapı`, `dene`, `filtrele`, `haritala`, `indirge` ve `tensör` (tensörler, `TR_AdamW`, `TR_backward`, `.TR_step()` ve `.TR_zero_grad()` ile küçük bir eğitim döngüsü); İngilizcede `func`, `if`, `match`, `for`, `list_comp`, `struct`, `try`, `filter`, `map`, `reduce` ve `tensor`. Şablonun yazdığı kod her dilde **kimlik yazımıdır** (`TR_if ${1:koşul} {`): dosya kimliklerle yazılır. Metot şablonları (`filtrele`) hem `filtrele` hem `.filtrele` olarak sunulur; `liste.fil` yazıp seçince nokta da değiştirilir (`liste.TR_filter(...)`). Şablonlar VS Code'un sabit şablon katkısıyla değil tamamlama önerisiyle verilir (katkı yazım dilini seçemez), bu yüzden "Insert Snippet" komutunun listesinde görünmezler.

Şablonlar elle iki kopya yazılmaz: gövde bir kez, kodlu biçimde (`TR_func` ...) `tools/gen/snippets/data.json`da durur (Türkçe sözcük yalnız `text/<dil>.json`da); her dilin hâli aynı gövdedir, kısayol kodun o dildeki adıdır. Bir şablonu ya da kod listesindeki bir adı değiştirince depo kökünde `go run ./tools/gen/snippets` çalıştırın (`go test ./...` dosyanın güncel olduğunu ve her dilin şablonunun kimlik yazımıyla ayrıştırıldığını denetler). Yeni bir yazım dili eklenince şablon dosyası kendiliğinden o dili de taşır; kullanıcıya önerilen ad ve metinler (`isim`, `koşul` ...) `wordTable`dadır, satırı olmayan dil İngilizceyi kullanır.

Renklendirmenin anahtar kelime, tip ve yerleşik listeleri de elle yazılmaz; dilin kod listesinden (`internal/lang/codes/tr.json`, ve İngilizce görünüm için `en.json`) üretilir. Kod listesinde bir ad değişince depo kökünde `go run ./tools/gen/grammar` çalıştırın (`go test ./...` dosyanın güncel olduğunu denetler). Her işin tek yazımı vardır: yalnızca Türkçe biçimler renklendirilir; `#` yorum değildir, yorumlar `//` ve `/* */` ile yazılır.

| Ne yapmak istiyorsunuz? | Nasıl |
| --- | --- |
| Kesme noktası koymak | Satır numarasının soluna tıklayın. Sağ tık → koşullu kesme noktası (`bakiye < 0`) veya iz noktası (`bakiye = {bakiye}` — durmadan Debug Console'a yazar). Koddaki `TR_kw_breakpoint` da durdurur. |
| Adım adım ilerlemek | F11 adım içine, F10 üstten geç, Shift+F11 fonksiyondan çık, F5 devam |
| Değişkenleri görmek | İzleme Masası ya da Variables paneli (Yerel ve Genel kapsamlar; listeler, sözlükler, yapılar ve tensörler açılır) |
| Değeri değiştirmek | İzleme Masası'nda ✏️ ya da Variables panelinde çift tıklama; TR ifadesi yazın (`0.20`, `[1, 2]`, `fiyat * 2`) |
| Değer değişince durmak (izleme noktası) | Variables panelinde sağ tık → **Break on Value Change** |
| Bir satıra zıplamak | Satırda sağ tık → **Jump to Cursor** |
| Çağrı yığınını görmek | Call Stack paneli |
| Komut çalıştırmak | Debug Console: ifade değerlendirin veya deyim çalıştırın (`bakiye = 0`) |

Sınırlamalar: kesme noktaları yalnızca hata ayıklanan dosyada çalışır (`TR_kw_import` ile yüklenen modüllerde değil); hata ayıklama oturumunda `TR_fn_input()` kullanılamaz.

## Ayar

`tr.interpreterPath`: TR yorumlayıcısının tam yolu (varsayılan: boş). Bu ayar **yalnızca kullanıcı ayarlarından** okunur (kapsamı `machine`): bir klasörün kendi `.vscode/settings.json` dosyası yorumlayıcıyı başka bir programa çeviremez. Göreli bir yol yalnızca güvenilen bir çalışma alanı klasörüne göre çözülür. Ayar boşsa sırasıyla şunlar denenir:

1. Yalnızca **güvenilen** çalışma alanında, dosyanın çalışma alanı klasörünün kökündeki `bin/tr.exe`, yoksa `tr.exe`
2. PATH üzerindeki `tr`

Üst klasörlere bakılmaz: indirdiğiniz bir klasörün içinde ya da üstünde duran yabancı bir `tr.exe` kendiliğinden çalıştırılmaz. Klasör açmadan tek bir `.tr` dosyası açtığınızda yalnızca bu ayar ve PATH kullanılır. Depoyu değil de bir alt klasörünü (örneğin `examples/`) açıyorsanız yorumlayıcının tam yolunu bu ayara yazın.

Komutlar yalnızca güvenilen çalışma alanlarında çalışır. Güvenilmeyen (kısıtlı kipteki) klasörlerde renklendirme, şablonlar ve dosya okuma yine kullanılabilir; çalıştırma, denetleme ve hata ayıklama kapalıdır ve bu düğmeler görünmez. Görünüm bu kipte açılmaz.

`tr.writingLanguage`: salt okunur görünümün ve kod şablonlarının kısayollarının yazım dili (`tr`, Türkçe, varsayılan; `en`, İngilizce). Yalnızca kullanıcı ayarlarından okunur; her kişi kendisi seçer, dosyada saklanmaz. Değişiklik açık görünümleri yeniler. Ayar çalıştırma, terminal ve hata ayıklama süreçlerine `TR_DIL` olarak da geçer.

Unicode işaretleme: VS Code kısıtlı kipte ASCII dışındaki her harfi sarı kutuyla işaretler. Eklenti `.tr` dosyalarında yalnızca Türk alfabesinin harflerini (ç, ğ, ı, İ, ö, ş, ü ve büyükleri) serbest bırakır (`editor.unicodeHighlight.nonBasicASCII: false` ve `allowedCharacters`); Latin harflerine benzeyen yabancı harfler (ör. Kiril `а`, `е`, Yunan `ο`) ve görünmez karakterler (ör. sıfır genişlikli boşluk) güvenilen ve güvenilmeyen klasörde yine kutuyla işaretlenir (`ambiguousCharacters` ve `invisibleCharacters` açık). Türkçe harfler hâlâ kutulu görünüyorsa eklentinin eski bir kopyası (1.7.0'dan önceki) çalışıyordur: Uzantılar görünümünden eski kopyayı kaldırıp güncelini kurun.

## Notlar

- En iyi deneyim için tek dosya yerine proje **klasörünü** açın (Dosya → Klasör Aç). Klasör açılmadan da çalıştırma çalışır: program bu durumda dosyanın bulunduğu klasörde, ayrı bir terminalde başlatılır; program bitince terminal açık kalır, çıkış kodu yazılır, bir tuşa basınca kapanır. `TR_fn_input()` bu terminalde de çalışır (Enter satırı yollar, Ctrl+C programı durdurur).
- `.tr` dosyalarında VS Code'un "belirsiz karakter" uyarısı kapalıdır; Türkçe `ı` harfi artık Latin `i` ile karıştırılabilir diye sarı kutuyla işaretlenmez.
- Bu depoyu klasör olarak açtığınızda `.vscode/tasks.json` görevleri de kullanılabilir: `Ctrl+Shift+B` açık dosyayı `bin/tr.exe` ile çalıştırır; denetleme ve terminal hata ayıklama görevleri de aynı `bin/tr.exe`'yi kullanır. Bu görevler ve eklenti komutları (▶, F5) yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce "Tasks: Run Task" → **TR: yorumlayıcıyı derle** (`go build -o bin/tr.exe ./cmd/tr`) görevini çalıştırın.
- `launch.json` ile özelleştirme: `"program"`, `"cwd"`, `"args"` (programa iletilen komut satırı argümanları, ör. `["--adım", "5"]`; F5'te de Ctrl+F5'te de iletilir) ve `"stopOnEntry": true` (ilk satırda dur) desteklenir.
- Eklentinin arayüz metinleri VS Code'un gösterim dilini izler (yazım dili ayarından bağımsızdır). Varsayılan dil İngilizcedir: komut başlıkları ve ayar açıklamaları `package.nls.json`'da, kodun gösterdiği iletiler (`showErrorMessage` ...) iletinin kendisi anahtar olacak biçimde `vscode.l10n.t` çağrılarındadır (`l10n.js`). Türkçeleri `package.nls.tr.json` ve `l10n/bundle.l10n.tr.json`'dadır; başka bir gösterim dili için `package.nls.<dil>.json` ve `l10n/bundle.l10n.<dil>.json` eklenir (VS Code 1.73'ten beri: [`vscode.l10n`](https://code.visualstudio.com/api/references/vscode-api#l10n)). Bir ileti eklerken ya da değiştirirken Türkçe paketi de güncelleyin: `test/l10n_test.js` eksik ve artık iletileri yakalar. Webview'lerdeki (İzleme Masası, Tablo Görünümü) metinler henüz Türkçedir. Eklentinin testleri `test/` klasöründedir (`node test/<ad>_test.js`; `go test ./cmd/tr/e2e` hepsini koşar); çalıştırma ve hata ayıklama yolları sahte bir `vscode` modülüyle sınanır.
