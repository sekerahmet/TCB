# 1. Hızlı Başlangıç Komutları

Yorumlayıcıyı proje kökünde derleyin:

```bash
GOAMD64=v3 go build -o bin/tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o bin/tr.exe ./cmd/tr   # PowerShell
```

2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez. Eğitim derlemelerinde v3, matris çarpımını v1 derlemeye göre çok hızlandırır (512×512 çarpımda yaklaşık 8–14 kat; bkz. `tools/kiyas/README.md`). İki derleme aynı tohumla son basamaklarda farklı sonuç verebilir (v3 kaynaştırılmış çarpma-toplama kullanır): aynı tohum aynı sonuç sözü aynı program dosyası için geçerlidir.

| Komut | Açıklama |
| :--- | :--- |
| `./bin/tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile hata ayıklayıcı açılır) |
| `./bin/tr.exe dosya.tr [argümanlar]` | `.tr` kaynak dosyasını derler ve anında çalıştırır (`./bin/tr.exe çalıştır dosya.tr` ile aynı). Dosya adından sonrakiler programa iletilir; program okumazsa çıkışta uyarı yazılır |
| `./bin/tr.exe hata_ayıkla dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `./bin/tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `./bin/tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `./bin/tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §7) |
| `./bin/tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (bkz. §8) |
| `./bin/tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |
| `./bin/tr.exe kodla dosya.tr` | Dosyayı yerinde **kodlu biçime** çevirir: dilin sözcükleri dilden bağımsız kodlarla (`A001`, `G100`) saklanır, gerisi bayt bayt aynı kalır; klasör verilirse içindeki tüm `.tr` dosyalarını çevirir, zaten kodlu dosyaya dokunmaz. Kodlu dosya çevrilmeden çalışır, test edilir ve aktarılır (bkz. `docs/dil/17_kodlar.md`, Kodlu saklama) |
| `./bin/tr.exe çöz dosya.tr` | Kodlu dosyayı yerinde Türkçe yazıma geri çevirir (klasör de verilebilir) |
| `./bin/tr.exe göster dosya.tr [--dil tr]` | Dosyayı bir yazım dilinde (bugün yalnızca `tr`) standart çıktıya basar; kodlu dosyanın hata mesajlarındaki satır ve sütunlar bu gösterime göredir |
| `./bin/tr.exe göster -`, `./bin/tr.exe kodla -` | Metni standart girişten okur, gösterimini ya da kodlu biçimini standart çıktıya yazar (düzenleyiciler için; çıkış kodu 2: metin ayrıştırılamadı, çevrilmedi) |
| `./bin/tr.exe dap` | VS Code hata ayıklama sunucusu; eklenti kendisi başlatır, elle çalıştırılmaz |
| `./bin/tr.exe yardım` / `-h`, `./bin/tr.exe sürüm` / `-v` | Yardımı ve sürümü gösterir |

Çalışan bir programı **Ctrl+C** nazikçe durdurur: program sıradaki döngü turunda ya da çağrıda durur, `ertele` blokları çalışır (dosyalar kapanır, kayıtlar yazılır), çıkış kodu 3'tür; ikinci Ctrl+C hemen çıkar (130). Ayrıntılar: `docs/dil/01_program.md`.

Her komutun tek, Türkçe bir adı vardır; Türkçe karaktersiz ya da İngilizce yazımlar (`calistir`, `bicimle`, `debug`, `--help` ...) çalışmaz, hata mesajı asıl komutu önerir (`'debug' TR'de bir komut değil; 'tr hata_ayıkla' yazmak mı istediniz?`). Kısa bayraklar `-h`, `-v`, `-k` simgedir. Kabuktan (REPL) yalnızca `çıkış` ile çıkılır; `exit` ya da `cikis` yazılınca kabuk `çıkış` yazmanızı söyler.

Öğrenmeye `ornekler/` klasöründeki numaralı örneklerle başlayın (`01_merhaba.tr` … `09_hata_ayiklama_alistirmasi.tr`; liste ve açıklamalar `README.md`'nin "Örnekler" bölümünde).

## Kodlu dosyalar: düzenleyici ve git farkları

Kodlu bir dosyayı (ilk satırı `//! tr kodlu 1`) VS Code'da açtığınızda Türkçe yazımıyla görürsünüz; kaydedince diske yine kodlu yazılır, satır numaraları hata mesajlarıyla ve hata ayıklayıcıyla aynıdır (ayrıntılar: [eklenti belgesi](../../editors/vscode/README.md#kodlu-dosyalar); kurallar: [17_kodlar.md, Düzenleyicide](../dil/17_kodlar.md#düzenleyicide)).

`git diff` kodlu dosyada kodları gösterir (`A030 x = 1`). Farkı Türkçe görmek için git'e `tr göster`'i bir "textconv" (farktan önce metni çeviren komut) olarak tanıtın. Depodaki `.gitattributes` `.tr` dosyalarını `tr` adlı fark sürücüsüne bağlar (`*.tr diff=tr`); sürücüyü siz, kendi git ayarınızda bir kez tanımlarsınız:

```bash
git config --global diff.tr.textconv "tr göster"                      # tr PATH üzerindeyse
git config --global diff.tr.textconv "'C:/Projeler/TR/bin/tr.exe' göster"   # ya da tam yolla (boşluklu yol tek tırnakta)
```

Sonra `git diff`, `git log -p` ve `git show` kodlu dosyalarda Türkçe fark gösterir (Türkçe yazılmış dosyalar olduğu gibi gösterilir); ham farkı görmek için `git diff --no-textconv`. Sürücü tanımlı değilse git olağan farkı gösterir, hiçbir şey bozulmaz. Textconv yalnızca gösterimi değiştirir: depoya kodlu metin girer, birleştirme de kodlu metin üzerinde yapılır. Bir birleştirme çakışması dosyayı ayrıştırılamaz bıraktıysa `tr göster` hata verir (git "textconv failed" der); çakışmayı düzeltin ya da `--no-textconv` kullanın. VS Code'un kendi kaynak denetimi görünümündeki farklar ham (kodlu) metni gösterir.
