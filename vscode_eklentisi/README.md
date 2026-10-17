# TR Dili — VS Code Eklentisi

`.tr` dosyaları için sözdizimi renklendirme, kod şablonları ve çalıştırma/hata ayıklama komutları.

## Kurulum

1. Önce TR yorumlayıcısını proje kökünde derleyin:

   ```bash
   go build -o tr.exe ./cmd/tr
   ```

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
| TR: Terminal Hata Ayıklayıcısında Aç | `tr.hataAyikla` | `tr.exe debug <dosya>` |
| TR: Görsel Hata Ayıklayıcı (Tarayıcı) | `tr.gorselHataAyikla` | `tr.exe debug --web <dosya>` |
| TR: Tablo Görünümü (ALV) | `tr.tabloGoster` | Hata ayıklarken değişkeni tablo olarak gösterir |

- Komutlar dosyayı önce kaydeder, sonra terminal panelinde çalıştırır. Hata ayıklayıcı komutlarını bu terminale yazabilirsiniz.
- Çalışma dizini, dosyanın bulunduğu çalışma alanı klasörüdür; klasör açık değilse dosyanın dizinidir.
- Düzenleyicinin başlık çubuğundaki **▶** düğmesi dosyayı çalıştırır, yanındaki **🐞** düğmesi hata ayıklamayı başlatır. Komutlar sağ tık menüsünde ve Komut Paleti'nde (`Ctrl+Shift+P`, "TR:") de bulunur.

## Hata ayıklama (ABAP debugger tarzı)

**F5** VS Code'un hata ayıklayıcısını açık `.tr` dosyasında başlatır; **Ctrl+F5** hata ayıklamadan terminalde çalıştırır (`girdi()` kullanan programlar için).

| ABAP debugger | VS Code'da |
| --- | --- |
| Kesme noktası (BREAK-POINT) | Satır numarasının soluna tıklayın. Sağ tık → koşullu kesme noktası (`bakiye < 0`) veya iz noktası (`bakiye = {bakiye}` — durmadan Debug Console'a yazar). Koddaki `kesme_noktası` da durdurur. |
| F5 / F6 / F7 / F8 | F11 adım içine, F10 üstten geç, Shift+F11 fonksiyondan çık, F5 devam |
| Değişkenler | Variables paneli: Yerel ve Genel kapsamlar; listeler, sözlükler, yapılar ve matrisler açılır |
| ✏️ Değeri değiştir | Değişkene çift tıklayın, TR ifadesi yazın (`0.20`, `[1, 2]`, `fiyat * 2`) |
| İzleme noktası (Watchpoint) | Variables panelinde sağ tık → **Break on Value Change** |
| Satıra zıpla (Shift+F12) | Satırda sağ tık → **Jump to Cursor** |
| Çağrı yığını | Call Stack paneli |
| İç tablo görüntüleyici (ALV) | Variables/Watch panelinde sağ tık → **TR: Tablo Görünümü** (sıralanabilir, aranabilir) |
| Komut satırı | Debug Console: ifade değerlendirin veya deyim çalıştırın (`bakiye = 0`) |

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
- `launch.json` ile özelleştirme: `"program"`, `"cwd"` ve `"stopOnEntry": true` (ilk satırda dur) desteklenir.
