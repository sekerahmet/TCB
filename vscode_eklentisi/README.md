# TR Dili — VS Code Eklentisi

`.tr` dosyaları için sözdizimi renklendirme, kod şablonları ve çalıştırma/hata ayıklama komutları.

## Kurulum

1. Önce TR yorumlayıcısını proje kökünde derleyin:

   ```bash
   GOAMD64=v3 go build -o tr.exe ./cmd/tr
   ```

   PowerShell'de: `$env:GOAMD64="v3"; go build -o tr.exe ./cmd/tr`. 2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez.

2. Bu klasörü (`vscode_eklentisi`) `%USERPROFILE%\.vscode\extensions\tr-dili-vscode` olarak kopyalayın:

   ```powershell
   Copy-Item -Recurse -Force vscode_eklentisi "$env:USERPROFILE\.vscode\extensions\tr-dili-vscode"
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

Tuşlar terminal hata ayıklayıcısında da (`tr hata_ayıkla`) aynı anlamdadır: F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık (terminalde tuş adı yazılıp Enter'a basılır). **F5** hata ayıklayıcıyı açık `.tr` dosyasında başlatır; **Ctrl+F5** hata ayıklamadan terminalde çalıştırır (`girdi()` kullanan programlar için). Denemek için depodaki `calisma/hata_ayiklama.tr` (adım adım alıştırma) ya da `ornekler/04_hata_ayiklama.tr` (koddaki koşullu `kesme_noktası`) dosyasını açıp F5'e basın.

**İzleme Masası:** Program her durduğunda editörün yanında açılır.

- **Yönlendirme düğmeleri:** ⤼ Sonraki satır (F10), ↓ İçine gir (F11), ↑ Dışarı çık (Shift+F11), ▶ Devam (F5), ■ Durdur. Program çalışırken adım düğmeleri pasif görünür.
- **Değişkenler:** Tüm değişkenler tek tabloda görünür (Kapsam, Değişken, Tip, Değer). Her değişkenin yanında türünü gösteren bir ikon vardır: ▦ tablo, ☰ liste, { } sözlük, ◆ yapı, ▣ tensör, # sayı, Aa metin, ◐ mantık, ∅ yok. Son adımda değişen değerler vurgulanır; ✏️ ile değer doğrudan değiştirilir. Üstteki kutuyla değişkenler aranır.
- **Tablolar:** Tablo değerlerinin içeriği satıra dökülmez. Değişkenin adına ya da **▸ Tabloyu aç** linkine tıklayınca (veya satıra çift tıklayınca) tablo alttaki pencerede bir **sekme** olarak açılır. Birden çok tablo sekmeler hâlinde açık kalabilir; her sekme kendi **✕** düğmesiyle, hepsi **Tümünü kapat** ile kapanır. Pencere, üstündeki çubuk sürüklenerek büyütülüp küçültülür. Sütun başlığına tıklayınca sıralanır, arama kutusuyla süzülür, hücreler arasında ok tuşlarıyla gezilir. Adım atıldıkça açık tablolar güncellenir.
- **Yapay zekâ nesneleri:** Tensörler tip sütununda `Tensör [2×3]` olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılır. Parçalayıcı, veri kümesi ve örnekleyici özetleriyle görünür (`Parçalayıcı (40000 sözcük)`, `Veri kümesi (246 parça, 56,9 milyar token)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)`); parçalayıcının eklenmiş/özel tokenları ve veri kümesinin parçaları tablo olarak açılır.
- İstemiyorsanız `tr.izlemeMasasi` ayarını kapatın.

## Kod şablonları

`.tr` dosyasında kısayolu yazıp `Tab`a basın: `fonk`, `eğer`, `eşle`, `döngü`, `liste_üret`, `yapı`, `dene`, `filtrele`, `haritala`, `sinir` ve `tensör` (tensörler, `AdamW`, `geri_yayılım`, `iyileştirici.adım()` ve `iyileştirici.sıfırla()` ile küçük bir eğitim döngüsü).

Renklendirmenin anahtar kelime, tip ve yerleşik listeleri elle yazılmaz; dilin sözlüğünden (`pkg/sozluk/tr.json`) üretilir. Sözlükte bir ad değişince depo kökünde `go run ./araclar/eklenti_listeleri` çalıştırın (`go test ./...` dosyanın güncel olduğunu denetler). Her işin tek yazımı vardır: yalnızca Türkçe biçimler renklendirilir; `#` yorum değildir, yorumlar `//` ve `/* */` ile yazılır.

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

`tr.yorumlayiciYolu`: TR yorumlayıcısının yolu (varsayılan: boş). Göreli yollar çalışma alanı klasörüne göre çözülür. Boş bırakılırsa sırasıyla şunlar denenir:

1. Çalışma alanı klasöründeki `tr.exe`
2. Dosyanın dizininden yukarı doğru bulunan ilk `tr.exe`
3. PATH üzerindeki `tr`

Komutlar yalnızca güvenilen çalışma alanlarında çalışır. Güvenilmeyen klasörlerde renklendirme ve şablonlar yine kullanılabilir.

## Notlar

- En iyi deneyim için tek dosya yerine proje **klasörünü** açın (Dosya → Klasör Aç). Klasör açılmadan da çalıştırma çalışır: program bu durumda dosyanın bulunduğu klasörde, ayrı bir terminalde başlatılır.
- `.tr` dosyalarında VS Code'un "belirsiz karakter" uyarısı kapalıdır; Türkçe `ı` harfi artık Latin `i` ile karıştırılabilir diye sarı kutuyla işaretlenmez.
- Bu depoyu klasör olarak açtığınızda `.vscode/tasks.json` görevleri de kullanılabilir: `Ctrl+Shift+B` açık dosyayı kökteki `tr.exe` ile çalıştırır; denetleme ve terminal hata ayıklama görevleri de aynı `tr.exe`'yi kullanır. Bu görevler ve eklenti komutları (▶, F5) yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce "Tasks: Run Task" → **TR: yorumlayıcıyı derle** (`GOAMD64=v3 go build -o tr.exe ./cmd/tr`) görevini çalıştırın.
- `launch.json` ile özelleştirme: `"program"`, `"cwd"` ve `"stopOnEntry": true` (ilk satırda dur) desteklenir.
