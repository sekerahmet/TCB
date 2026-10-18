# 1. Hızlı Başlangıç Komutları

Yorumlayıcıyı proje kökünde derleyin:

```bash
GOAMD64=v3 go build -o ikili/tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o ikili/tr.exe ./cmd/tr   # PowerShell
```

2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez. Eğitim derlemelerinde v3, matris çarpımını v1 derlemeye göre çok hızlandırır (512×512 çarpımda yaklaşık 8–14 kat; bkz. `bakim/kiyas/README.md`).

| Komut | Açıklama |
| :--- | :--- |
| `./ikili/tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile hata ayıklayıcı açılır) |
| `./ikili/tr.exe dosya.tr [argümanlar]` | `.tr` kaynak dosyasını derler ve anında çalıştırır (`./ikili/tr.exe çalıştır dosya.tr` ile aynı). Dosya adından sonrakiler programa iletilir; program okumazsa çıkışta uyarı yazılır |
| `./ikili/tr.exe hata_ayıkla dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `./ikili/tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `./ikili/tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `./ikili/tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §7) |
| `./ikili/tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (bkz. §8) |
| `./ikili/tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |
| `./ikili/tr.exe dap` | VS Code hata ayıklama sunucusu; eklenti kendisi başlatır, elle çalıştırılmaz |
| `./ikili/tr.exe yardım` / `-h`, `./ikili/tr.exe sürüm` / `-v` | Yardımı ve sürümü gösterir |

Çalışan bir programı **Ctrl+C** nazikçe durdurur: program sıradaki döngü turunda ya da çağrıda durur, `ertele` blokları çalışır (dosyalar kapanır, kayıtlar yazılır), çıkış kodu 3'tür; ikinci Ctrl+C hemen çıkar (130). Ayrıntılar: `docs/dil/01_program.md`.

Her komutun tek, Türkçe bir adı vardır; Türkçe karaktersiz ya da İngilizce yazımlar (`calistir`, `bicimle`, `debug`, `--help` ...) çalışmaz, hata mesajı asıl komutu önerir (`'debug' TR'de bir komut değil; 'tr hata_ayıkla' yazmak mı istediniz?`). Kısa bayraklar `-h`, `-v`, `-k` simgedir. Kabuktan (REPL) yalnızca `çıkış` ile çıkılır; `exit` ya da `cikis` yazılınca kabuk `çıkış` yazmanızı söyler.

Öğrenmeye `ornekler/` klasöründeki numaralı örneklerle başlayın (`01_merhaba.tr` … `09_hata_ayiklama_alistirmasi.tr`; liste ve açıklamalar `README.md`'nin "Örnekler" bölümünde).
