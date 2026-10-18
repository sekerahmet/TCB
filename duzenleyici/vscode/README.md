# TR Dili — VS Code Eklentisi

`.tr` dosyaları için sözdizimi renklendirme, kod şablonları ve çalıştırma/hata ayıklama komutları.

## Kurulum

1. Önce TR yorumlayıcısını proje kökünde derleyin:

   ```bash
   GOAMD64=v3 go build -o ikili/tr.exe ./cmd/tr
   ```

   PowerShell'de: `$env:GOAMD64="v3"; go build -o ikili/tr.exe ./cmd/tr`. Derlenen yorumlayıcı `ikili/` klasörüne yazılır (bu klasör depoya girmez). 2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez.

2. Bu klasörü (`duzenleyici/vscode`) `%USERPROFILE%\.vscode\extensions\tr-dili-vscode` olarak kopyalayın:

   ```powershell
   Copy-Item -Recurse -Force duzenleyici\vscode "$env:USERPROFILE\.vscode\extensions\tr-dili-vscode"
   ```

3. VS Code'u yeniden başlatın.

## Komutlar

| Komut | Kimlik | Çalıştırılan |
| --- | --- | --- |
| TR: Dosyayı Çalıştır | `tr.calistir` | `tr.exe <dosya>` |
| TR: Hata Ayıkla (VS Code) | `tr.hataAyiklaBaslat` | VS Code hata ayıklayıcısı (`tr.exe dap`) |
| TR: Dosyayı Denetle | `tr.denetle` | `tr.exe kontrol <dosya>` |
| TR: Terminal Hata Ayıklayıcısında Aç | `tr.hataAyikla` | `tr.exe hata_ayıkla <dosya>` |
| TR: Tablo Görünümü | `tr.tabloGoster` | Hata ayıklarken değişkeni tablo olarak gösterir |
| TR: İzleme Masası | `tr.izlemeMasasi` | Hata ayıklarken tüm değişkenleri tablo hâlinde gösteren paneli açar |

- Komutlar dosyayı önce kaydeder, sonra terminal panelinde çalıştırır. Hata ayıklayıcı komutlarını bu terminale yazabilirsiniz.
- Çalışma dizini, dosyanın bulunduğu çalışma alanı klasörüdür; klasör açık değilse dosyanın dizinidir.
- Düzenleyicinin başlık çubuğundaki **▶** düğmesi dosyayı çalıştırır, yanındaki **🐞** düğmesi hata ayıklamayı başlatır. Komutlar sağ tık menüsünde ve Komut Paleti'nde (`Ctrl+Shift+P`, "TR:") de bulunur.

## Hata ayıklama

Tuşlar terminal hata ayıklayıcısında da (`tr hata_ayıkla`) aynı anlamdadır: F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık (terminalde tuş adı yazılıp Enter'a basılır). **F5** hata ayıklayıcıyı açık `.tr` dosyasında başlatır; **Ctrl+F5** hata ayıklamadan terminalde çalıştırır (`girdi()` kullanan programlar için). Denemek için depodaki `ornekler/09_hata_ayiklama_alistirmasi.tr` (adım adım alıştırma) ya da `ornekler/04_hata_ayiklama.tr` (koddaki koşullu `kesme_noktası`) dosyasını açıp F5'e basın.

**İzleme Masası:** Program her durduğunda editörün yanında açılır.

- **Yönlendirme düğmeleri:** ⤼ Sonraki satır (F10), ↓ İçine gir (F11), ↑ Dışarı çık (Shift+F11), ▶ Devam (F5), ■ Durdur. Düğmeler panelin en üstünde sabit durur (konum metni uzayıp kısalınca yer değiştirmez) ve fareye basıldığı anda çalışır. Program çalışırken adım düğmeleri pasif görünür. Altlarındaki satır konumu ve duruş nedenini gösterir; **N. duruş** sayacı, Devam döngüdeki aynı kesme noktasında yeniden durduğunda da programın ilerlediğini gösterir.
- **Değişkenler:** Tüm değişkenler tek tabloda görünür (Kapsam, Değişken, Tip, Değer). Her değişkenin yanında türünü gösteren bir ikon vardır: ▦ tablo, ☰ liste, { } sözlük, ◆ yapı, ▣ tensör, # sayı, Aa metin, ◐ mantık, ∅ yok. Son adımda değişen değerler vurgulanır; ✏️ ile değer doğrudan değiştirilir. Üstteki kutuyla değişkenler aranır.
- **Tablolar:** Tablo değerlerinin içeriği satıra dökülmez. Değişkenin adına ya da **▸ Tabloyu aç** linkine tıklayınca (veya satıra çift tıklayınca) tablo alttaki pencerede bir **sekme** olarak açılır. Birden çok tablo sekmeler hâlinde açık kalabilir; her sekme kendi **✕** düğmesiyle, hepsi **Tümünü kapat** ile kapanır. Pencere, üstündeki çubuk sürüklenerek büyütülüp küçültülür. Sütun başlığına tıklayınca sıralanır, arama kutusuyla süzülür, hücreler arasında ok tuşlarıyla gezilir. Adım atıldıkça açık tablolar güncellenir.
- **Yapay zekâ nesneleri:** Tensörler tip sütununda `Tensör [2×3]` olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılır. Skaler tensörün (ör. kayıp) değeri doğrudan görünür: `0.6074 (Tensör, skaler)`; 1-8 elemanlı küçük tensörler satır içinde kısaltılmış yazılır (`[[1.4298]] (Tensör, 1×1)`), daha büyükleri **▸ Tabloyu aç** ile açılır. float32 dışındaki türlerde tür de yazılır (`2.5000 (Tensör, skaler, bf16)`). Variables panelinde ve Debug Console'da da aynı değer görünür. Parçalayıcı, veri kümesi ve örnekleyici özetleriyle görünür (`Parçalayıcı (dağarcık 40000)`, `Veri kümesi (246 veri dosyası, 56,9 milyar parça)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)`); parçalayıcının eklenmiş/özel parçaları ve veri kümesinin veri dosyaları tablo olarak açılır.
- İstemiyorsanız `tr.izlemeMasasi` ayarını kapatın.

## Kod şablonları

`.tr` dosyasında kısayolu yazıp `Tab`a basın: `fonk`, `eğer`, `eşle`, `döngü`, `liste_üret`, `yapı`, `dene`, `filtrele`, `haritala`, `indirge` ve `tensör` (tensörler, `AdamW`, `geri_yayılım`, `iyileştirici.adım()` ve `iyileştirici.sıfırla()` ile küçük bir eğitim döngüsü).

Renklendirmenin anahtar kelime, tip ve yerleşik listeleri elle yazılmaz; dilin kod listesinden (`internal/dil/kod/tr.json`) üretilir. Kod listesinde bir ad değişince depo kökünde `go run ./bakim/uret/eklenti_listeleri` çalıştırın (`go test ./...` dosyanın güncel olduğunu denetler). Her işin tek yazımı vardır: yalnızca Türkçe biçimler renklendirilir; `#` yorum değildir, yorumlar `//` ve `/* */` ile yazılır.

| Ne yapmak istiyorsunuz? | Nasıl |
| --- | --- |
| Kesme noktası koymak | Satır numarasının soluna tıklayın. Sağ tık → koşullu kesme noktası (`bakiye < 0`) veya iz noktası (`bakiye = {bakiye}` — durmadan Debug Console'a yazar). Koddaki `kesme_noktası` da durdurur. |
| Adım adım ilerlemek | F11 adım içine, F10 üstten geç, Shift+F11 fonksiyondan çık, F5 devam |
| Değişkenleri görmek | İzleme Masası ya da Variables paneli (Yerel ve Genel kapsamlar; listeler, sözlükler, yapılar ve tensörler açılır) |
| Değeri değiştirmek | İzleme Masası'nda ✏️ ya da Variables panelinde çift tıklama; TR ifadesi yazın (`0.20`, `[1, 2]`, `fiyat * 2`) |
| Değer değişince durmak (izleme noktası) | Variables panelinde sağ tık → **Break on Value Change** |
| Bir satıra zıplamak | Satırda sağ tık → **Jump to Cursor** |
| Çağrı yığınını görmek | Call Stack paneli |
| Komut çalıştırmak | Debug Console: ifade değerlendirin veya deyim çalıştırın (`bakiye = 0`) |

Sınırlamalar: kesme noktaları yalnızca hata ayıklanan dosyada çalışır (`aktar` ile yüklenen modüllerde değil); hata ayıklama oturumunda `girdi()` kullanılamaz.

## Ayar

`tr.yorumlayiciYolu`: TR yorumlayıcısının tam yolu (varsayılan: boş). Bu ayar **yalnızca kullanıcı ayarlarından** okunur (kapsamı `machine`): bir klasörün kendi `.vscode/settings.json` dosyası yorumlayıcıyı başka bir programa çeviremez. Göreli bir yol yalnızca güvenilen bir çalışma alanı klasörüne göre çözülür. Ayar boşsa sırasıyla şunlar denenir:

1. Yalnızca **güvenilen** çalışma alanında, dosyanın çalışma alanı klasörünün kökündeki `ikili/tr.exe`, yoksa `tr.exe`
2. PATH üzerindeki `tr`

Üst klasörlere bakılmaz: indirdiğiniz bir klasörün içinde ya da üstünde duran yabancı bir `tr.exe` kendiliğinden çalıştırılmaz. Klasör açmadan tek bir `.tr` dosyası açtığınızda yalnızca bu ayar ve PATH kullanılır. Depoyu değil de bir alt klasörünü (örneğin `ornekler/`) açıyorsanız yorumlayıcının tam yolunu bu ayara yazın.

Komutlar yalnızca güvenilen çalışma alanlarında çalışır. Güvenilmeyen (kısıtlı kipteki) klasörlerde renklendirme, şablonlar ve dosya okuma yine kullanılabilir; çalıştırma, denetleme ve hata ayıklama kapalıdır ve bu düğmeler görünmez.

Unicode işaretleme: VS Code kısıtlı kipte ASCII dışındaki her harfi sarı kutuyla işaretler. Eklenti `.tr` dosyalarında yalnızca Türk alfabesinin harflerini (ç, ğ, ı, İ, ö, ş, ü ve büyükleri) serbest bırakır (`editor.unicodeHighlight.nonBasicASCII: false` ve `allowedCharacters`); Latin harflerine benzeyen yabancı harfler (ör. Kiril `а`, `е`, Yunan `ο`) ve görünmez karakterler (ör. sıfır genişlikli boşluk) güvenilen ve güvenilmeyen klasörde yine kutuyla işaretlenir (`ambiguousCharacters` ve `invisibleCharacters` açık). Türkçe harfler hâlâ kutulu görünüyorsa eklentinin eski bir kopyası (1.7.0'dan önceki) çalışıyordur: Uzantılar görünümünden eski kopyayı kaldırıp güncelini kurun.

## Notlar

- En iyi deneyim için tek dosya yerine proje **klasörünü** açın (Dosya → Klasör Aç). Klasör açılmadan da çalıştırma çalışır: program bu durumda dosyanın bulunduğu klasörde, ayrı bir terminalde başlatılır.
- `.tr` dosyalarında VS Code'un "belirsiz karakter" uyarısı kapalıdır; Türkçe `ı` harfi artık Latin `i` ile karıştırılabilir diye sarı kutuyla işaretlenmez.
- Bu depoyu klasör olarak açtığınızda `.vscode/tasks.json` görevleri de kullanılabilir: `Ctrl+Shift+B` açık dosyayı `ikili/tr.exe` ile çalıştırır; denetleme ve terminal hata ayıklama görevleri de aynı `ikili/tr.exe`'yi kullanır. Bu görevler ve eklenti komutları (▶, F5) yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce "Tasks: Run Task" → **TR: yorumlayıcıyı derle** (`GOAMD64=v3 go build -o ikili/tr.exe ./cmd/tr`) görevini çalıştırın.
- `launch.json` ile özelleştirme: `"program"`, `"cwd"`, `"args"` (programa iletilen komut satırı argümanları, ör. `["--adım", "5"]`) ve `"stopOnEntry": true` (ilk satırda dur) desteklenir.
