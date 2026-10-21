# Örnekler

Numaralı örnekler basitten zora bir öğrenme yoludur: her biri az sayıda yeni ad getirir ve öncekilerde öğretilmemiş hiçbir şeyi kullanmaz. Her örnek iki yazımla bulunur: [`tr/`](tr/) Türkçe, [`en/`](en/) İngilizce (aynı numara aynı program; İngilizce dosyalar kodlu saklanır, `tr göster --dil en` ile okunur). Yol, dilin bütün adlarını kapsar; kapsamı `go test ./cmd/tr/e2e -run TestExamplesCover` denetler. Tam liste ve ne öğrettikleri: [kök README](../README.md#örnekler).

Her örnek tek başına çalışır:

```sh
tr examples/tr/01_merhaba.tr        # ya da: tr çalıştır examples/tr/01_merhaba.tr
tr examples/en/01_hello.tr          # aynı program, İngilizce yazım
```

## Komut satırı

Örneklerin hepsi bir `.tr` dosyasıdır; dilin komut satırı adları (`K` ailesi) dosyada geçmez, burada gösterilir. Komut ve seçenek adlarının her gömülü dilde yazımı kabul edilir (`tr çalıştır` ile `tr run` aynıdır).

| Komut | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `yardım` | Komutların ve seçeneklerin kısa yardımı | `tr yardım` |
| `sürüm` | TR'nin sürümü | `tr sürüm` |
| `çalıştır` | Programı çalıştırır (komut adı yazılmadan verilen dosya da çalışır) | `tr çalıştır examples/tr/07_donguler.tr` |
| `kontrol` | Çalıştırmadan sözdizimi ve tip denetimi | `tr kontrol examples/tr/13_matematik.tr` |
| `derle` | Programı yorumlayıcıyla birlikte tek çalıştırılabilir dosyaya paketler | `tr derle examples/tr/18_moduller.tr` |
| `test` | Dosyadaki ya da klasördeki `*_test.tr` testlerini koşar | `tr test examples/tr/17_testler_test.tr` |
| `biçimle` | Dosyaları standart biçime getirir | `tr biçimle examples/tr/09_listeler.tr` |
| `kodla` | Dosyayı dilden bağımsız kodlu biçime çevirir | `tr kodla scratch/deneme.tr` |
| `çöz` | Kodlu dosyayı etkin yazım dilindeki biçime geri çevirir | `tr çöz scratch/deneme.tr` |
| `göster` | Kodlu dosyayı değiştirmeden bir yazım dilinde yazar | `tr göster examples/en/01_hello.tr --dil tr` |
| `hata_ayıkla` | Programı terminal hata ayıklayıcısında başlatır | `tr hata_ayıkla examples/tr/40_hata_ayiklama.tr` |
| `izle` | Çalışan bir eğitimin canlı profil tablosunu gösterir | `tr izle examples/tr/38_kontrol_noktasi.tr` |
| `profil` | Biten ya da süren bir eğitimin profil raporunu yazar | `tr profil examples/tr/28_egitim_dongusu.tr` |
| `dap` | Düzenleyicinin hata ayıklama protokolü sunucusu (düzenleyiciler kullanır) | VS Code F5 |

Seçenekler (komutlardan sonra, iki tireyle; adın sözcükleri tireyle ayrılır):

| Seçenek | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `--dil` | Yazım dilini seçer (`tr`, `en`) | `tr göster examples/en/05_conditions.tr --dil tr` |
| `--kontrol` | `biçimle` ile dosyayı değiştirmeden standart biçimde olup olmadığını denetler | `tr biçimle --kontrol examples` |
| `--json` | `profil` raporunu JSON olarak yazar | `tr profil --json rapor.json examples/tr/28_egitim_dongusu.tr` |
| `--go-profili` | `profil` ile yorumlayıcının kendi Go profilini de yazar | `tr profil --go-profili examples/tr/28_egitim_dongusu.tr` |
| `--çıktı` | `izle` tablosunun yazılacağı dosyayı seçer | `tr izle --çıktı canli.txt examples/tr/38_kontrol_noktasi.tr` |
| `--yardım` | Programın `komut_satırı` ile tanımlanan seçeneklerinin yardımı | `tr examples/tr/37_veri_ile_egitim.tr --yardım` |

Etkileşimli kabuk `tr` ile açılır (dosya verilmeden); kabuktan `çıkış` ile çıkılır.

Hata ayıklayıcı komutları ([40](tr/40_hata_ayiklama.tr) ve [41](tr/41_hata_ayiklama_alistirmasi.tr)'te kullanılır; VS Code'da aynı işler F5, F10, F11 tuşlarıdır):

| Komut | Ne yapar |
| :-- | :-- |
| `adım` | Tek adım ilerler, fonksiyonların içine girer (F11) |
| `geç` | Üstten geçer: fonksiyonu tek adımda çalıştırır (F10) |
| `dön` | Çalışan fonksiyondan çıkana kadar çalıştırır (Shift+F11) |
| `devam` | Sonraki kesme noktasına kadar devam eder (F5) |
| `kesme` | Bir satıra kesme noktası koyar ya da kaldırır |
| `zıpla` | Çalışma sırasını verilen satıra taşır |
| `değiştir` | Çalışma anında bir değişkenin değerini değiştirir (`değiştir vergi_orani = 0.20`) |
| `izle` | Bir ifadeyi izleme listesine ekler |
| `tablo` | Değişkenlerin tablosunu gösterir (`tablo siparisler`) |
| `yığın` | Çağrı yığınını gösterir |
| `hesapla` | Bir ifadeyi o anki ortamda hesaplar |
| `ekran` | Kaynak kodun duraklanan satır çevresini gösterir |
| `yardım` | Hata ayıklayıcı komutlarının yardımı |
| `çıkış` | Hata ayıklayıcıdan ve programdan çıkar |

## Hata örnekleri

[`tr/hatalar/`](tr/hatalar/) (ve [`en/errors/`](en/errors/)) sık yapılan hataları tek tek gösterir: her dosya bir hatayı bilerek yapar, yorumunda nedenini ve düzeltilmiş hâlini söyler. Bu dosyalar çıkış koduyla biter; testler iletinin kodunu (`H…`) ve çıkış kodunu sınar.
