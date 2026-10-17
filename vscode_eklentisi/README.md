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
| TR: Dosyayı Denetle | `tr.denetle` | `tr.exe kontrol <dosya>` |
| TR: Hata Ayıklayıcıda Aç | `tr.hataAyikla` | `tr.exe debug <dosya>` |
| TR: Görsel Hata Ayıklayıcı (Tarayıcı) | `tr.gorselHataAyikla` | `tr.exe debug --web <dosya>` |

- Komutlar dosyayı önce kaydeder, sonra terminal panelinde çalıştırır. Hata ayıklayıcı komutlarını bu terminale yazabilirsiniz.
- Çalışma dizini, dosyanın bulunduğu çalışma alanı klasörüdür; klasör açık değilse dosyanın dizinidir.
- **F5** veya **Ctrl+F5**: Dosyayı Çalıştır (hata ayıklama oturumu açık değilken).
- Düzenleyicinin başlık çubuğundaki **▶** düğmesi dosyayı çalıştırır. Dört komutun hepsi sağ tık menüsünde ve Komut Paleti'nde (`Ctrl+Shift+P`, "TR:") de bulunur.

## Ayar

`tr.yorumlayiciYolu`: TR yorumlayıcısının yolu (varsayılan: boş). Göreli yollar çalışma alanı klasörüne göre çözülür. Boş bırakılırsa sırasıyla şunlar denenir:

1. Çalışma alanı klasöründeki `tr.exe`
2. Dosyanın dizininden yukarı doğru bulunan ilk `tr.exe`
3. PATH üzerindeki `tr`

Komutlar yalnızca güvenilen çalışma alanlarında çalışır. Güvenilmeyen klasörlerde renklendirme ve şablonlar yine kullanılabilir.

## Notlar

- En iyi deneyim için tek dosya yerine proje **klasörünü** açın (Dosya → Klasör Aç). Klasör açılmadan da çalıştırma çalışır: program bu durumda dosyanın bulunduğu klasörde, ayrı bir terminalde başlatılır.
- `.tr` dosyalarında VS Code'un "belirsiz karakter" uyarısı kapalıdır; Türkçe `ı` harfi artık Latin `i` ile karıştırılabilir diye sarı kutuyla işaretlenmez.
- **F5 her yerden çalışır:** TR, VS Code'a hata ayıklanabilir bir dil olarak tanıtılır. F5, Ctrl+F5 ya da "Çalıştır" menüsü, imleç kodun içinde olmasa bile açık `.tr` dosyasını çalıştırır; "You don't have an extension for debugging" uyarısı çıkmaz. `launch.json` içinde `"hataAyikla": true` verilirse dosya terminal hata ayıklayıcısında (`tr debug`) açılır.
