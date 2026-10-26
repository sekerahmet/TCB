# Örnekler

Numaralı örnekler basitten zora bir öğrenme yoludur: her biri az sayıda yeni ad getirir ve öncekilerde öğretilmemiş hiçbir şeyi kullanmaz. Örnekler [`tr/`](tr/) klasöründedir ve kodlu saklanır: aynı dosya her gösterim dilinde okunur (`tr göster tr/01_merhaba.tr --dil en` İngilizce gösterir). Yol, dilin bütün adlarını kapsar; kapsamı `go test ./cmd/tr/e2e -run TestExamplesCover` denetler. Tam liste ve ne öğrettikleri: [kök README](../README.md#örnekler); örneklerin hangi adları getirdiğini sırayla görmek için başlıklara bakın.

Her örnek tek başına çalışır:

```sh
tr examples/tr/01_merhaba.tr        # ya da: tr çalıştır examples/tr/01_merhaba.tr
TR_DIL=en tr examples/tr/01_merhaba.tr   # aynı program, iletiler İngilizce
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
| `göster` | Kodlu dosyayı değiştirmeden bir yazım dilinde yazar | `tr göster examples/tr/01_merhaba.tr --dil en` |
| `hata_ayıkla` | Programı terminal hata ayıklayıcısında başlatır | `tr hata_ayıkla examples/tr/42_hata_ayiklama.tr` |
| `izle` | Çalışan bir eğitimin canlı profil tablosunu gösterir | `tr izle examples/tr/40_kontrol_noktasi.tr` |
| `profil` | Biten ya da süren bir eğitimin profil raporunu yazar | `tr profil examples/tr/28_egitim_dongusu.tr` |
| `dap` | Düzenleyicinin hata ayıklama protokolü sunucusu (düzenleyiciler kullanır) | VS Code F5 |
| `dizin` | TR kodunun dizinini kurar ve özetler; altındaki alt komutlarla ona sorulur | `tr dizin` |

`dizin` komutunun alt komutları yalnızca onun altında geçerlidir (`tr tanım` bir komut değildir):

| Alt komut | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `tanım` | Bir adın tanımı: nerede, imzası, belge yorumu, üyeleri | `tr dizin tanım Doğrusal` |
| `kullananlar` | Bir ada başvuranlar: çağrı, okuma, yazma, aktarma, test | `tr dizin kullananlar Doğrusal` |
| `sınayanlar` | Bir adı sınayan test blokları (doğrudan, sonra çağrı zinciriyle dolaylı) | `tr dizin sınayanlar Doğrusal` |
| `etki` | Bir ad ya da dosya değişirse etkilenenler | `tr dizin etki lib/katman.tr` |
| `yeniden_adlandır` | Bir ad değişirse değişecek yerler: kesin ve olası (hiçbir dosyayı değiştirmez) | `tr dizin yeniden_adlandır baş bakış_sayısı` |

Seçenekler (komutlardan sonra, iki tireyle; adın sözcükleri tireyle ayrılır):

| Seçenek | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `--dil` | Yazım dilini seçer (`tr`, `en`) | `tr göster examples/tr/05_kosullar.tr --dil en` |
| `--kontrol` | `biçimle` ile dosyayı değiştirmeden standart biçimde olup olmadığını denetler | `tr biçimle --kontrol examples` |
| `--json` | `profil` raporunu JSON olarak yazar | `tr profil --json rapor.json examples/tr/28_egitim_dongusu.tr` |
| `--go-profili` | `profil` ile yorumlayıcının kendi Go profilini de yazar | `tr profil --go-profili examples/tr/28_egitim_dongusu.tr` |
| `--çıktı` | `izle` tablosunun yazılacağı dosyayı seçer | `tr izle --çıktı canli.txt examples/tr/40_kontrol_noktasi.tr` |
| `--sınır` | `dizin` sorgularında bir listedeki en çok satır (varsayılan 12) | `tr dizin kullananlar Doğrusal --sınır 5` |
| `--tümü` | `dizin` sorgularında satır sınırını kaldırır | `tr dizin kullananlar Doğrusal --tümü` |
| `--kök` | `dizin` için proje kökü (verilmezse `.git` bulunan üst klasör) | `tr dizin tanım Doğrusal --kök .` |
| `--tür` | `dizin` sorgu adını yalnızca bu türdeki düğümlere çözer | `tr dizin tanım ileri --tür method` |
| `--dizin-yok` | `dizin` sorgusunda önbellekteki dizini olduğu gibi kullanır (tazelemez) | `tr dizin tanım Doğrusal --dizin-yok` |
| `--durum` | `dizin` komutunda dizin önbelleğinin durumunu gösterir | `tr dizin --durum` |
| `--temizle` | `dizin` komutunda dizin önbelleğini siler | `tr dizin --temizle` |
| `--metin-yok` | `yeniden_adlandır` sorgusunda belge, yorum ve ileti metni taramasını kapatır | `tr dizin yeniden_adlandır baş bakış --metin-yok` |
| `--terim` | `yeniden_adlandır` sorgusunda sembol aramaz, sözcüğü yalnızca metinde arar | `tr dizin yeniden_adlandır çekirdek kernel --terim` |
| `--yardım` | Programın `komut_satırı` ile tanımlanan seçeneklerinin yardımı | `tr examples/tr/39_veri_ile_egitim.tr --yardım` |

Ortam değişkenleri (komut ve seçenekler gibi her gömülü dilin adıyla okunur: `TR_GPU_ESZAMANLI` ile `TR_GPU_SYNC` aynıdır):

| Değişken | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `TR_DIL` | Yazım dilini seçer (`tr`, `en`): iletiler, yardım, kodlu dosyanın görünümü | `TR_DIL=en tr yardım` |
| `TR_IS_KLASORU` | Arka plan işinin klasörü: ilerleme çubuğu oraya `progress.json` yazar | `TR_IS_KLASORU=isler/egitim tr examples/tr/28_egitim_dongusu.tr` |
| `TR_GPU_ESZAMANLI` | `1` ise her GPU kernelinden sonra beklenir; kernel hatası tam yerinde bildirilir (yavaş) | `TR_GPU_ESZAMANLI=1 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |
| `TR_GPU_PROFIL` | `1` ise GPU kernel süreleri ölçülür (`izle` ve `profil` kendileri açar) | `TR_GPU_PROFIL=1 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |
| `TR_CPU_AVX2` | `0` ise CPU'nun AVX2/FMA kernelleri kapanır, taşınabilir Go kernelleri çalışır | `TR_CPU_AVX2=0 tr examples/tr/23_tensor.tr` |
| `TR_GPU_CUBLASLT` | Bulunamazsa cuBLASLt kitaplığının tam yolu | `TR_GPU_CUBLASLT=/usr/local/cuda/lib64/libcublasLt.so.12 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |

Etkileşimli kabuk `tr` ile açılır (dosya verilmeden); kabuktan `çıkış` ile çıkılır.

Hata ayıklayıcı komutları ([42](tr/42_hata_ayiklama.tr) ve [43](tr/43_hata_ayiklama_alistirmasi.tr)'te kullanılır; VS Code'da aynı işler F5, F10, F11 tuşlarıdır):

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
