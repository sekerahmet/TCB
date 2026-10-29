# Örnekler

Numaralı örnekler basitten zora bir öğrenme yoludur: her biri az sayıda yeni ad getirir ve öncekilerde öğretilmemiş hiçbir şeyi kullanmaz. Örnekler [`tr/`](tr/) klasöründedir ve kodlu saklanır: aynı dosya her gösterim dilinde okunur (`tr göster tr/01_merhaba.tr --dil en` İngilizce gösterir). Yol, dilin bütün adlarını kapsar; kapsamı `go test ./cmd/tr/e2e -run TestExamplesCover` denetler. Tam liste ve ne öğrettikleri: [kök README](../README.md#örnekler); örneklerin hangi adları getirdiğini sırayla görmek için başlıklara bakın.

Her örnek tek başına çalışır:

```sh
tr examples/tr/01_merhaba.tr        # ya da: tr çalıştır examples/tr/01_merhaba.tr
TR_DIL=en tr examples/tr/01_merhaba.tr   # aynı program, iletiler İngilizce
```

## Sıralı liste

Örnekler basitten zora numaralı bir öğrenme yoludur: her örnek tek bir yeni kavram getirir, öncekilerde öğretilmemiş hiçbir şeyi kullanmaz; başında ne öğrettiğini ve önceki örnekten ne aldığını söyler, sonunda "Kendin dene" alıştırmaları vardır. **Örnekler kodlu saklanır** ([`examples/tr/`](tr/)): aynı dosya her gösterim dilinde okunur, `tr göster examples/tr/01_merhaba.tr --dil en` İngilizce gösterir (bkz. [dil tanımı 17](docs/spec/17_kodlar.md)). Proje kökünden çalıştırın: `./bin/tr.exe examples/tr/01_merhaba.tr`.

| No | Örnek | Ne öğretir |
| :-- | :-- | :-- |
| 01 | [`01_merhaba.tr`](tr/01_merhaba.tr) | Merhaba: ilk TR programı |
| 02 | [`02_degiskenler.tr`](tr/02_degiskenler.tr) | Değişkenler: bir değeri adlandırıp saklamak |
| 03 | [`03_tipler.tr`](tr/03_tipler.tr) | Tipler: her değerin bir tipi vardır |
| 04 | [`04_metin.tr`](tr/04_metin.tr) | Metin: metne değer yerleştirmek ve metinle çalışmak |
| 05 | [`05_kosullar.tr`](tr/05_kosullar.tr) | Koşullar: programın karar vermesi |
| 06 | [`06_girdi.tr`](tr/06_girdi.tr) | Girdi: kullanıcıdan veri okumak |
| 07 | [`07_donguler.tr`](tr/07_donguler.tr) | Döngüler: bir işi yinelemek |
| 08 | [`08_fonksiyonlar.tr`](tr/08_fonksiyonlar.tr) | Fonksiyonlar: işi adlandırıp yeniden kullanmak |
| 09 | [`09_listeler.tr`](tr/09_listeler.tr) | Listeler: birçok değeri sıralı tutmak |
| 10 | [`10_ok_fonksiyonlari.tr`](tr/10_ok_fonksiyonlari.tr) | Ok fonksiyonları ve zincirleme: fonksiyonu değer olarak kullanmak |
| 11 | [`11_sozlukler.tr`](tr/11_sozlukler.tr) | Sözlükler: değerleri anahtarla saklamak |
| 12 | [`12_tip_adlari.tr`](tr/12_tip_adlari.tr) | Tip adları: değerlerin tipini sormak ve bildirmek |
| 13 | [`13_matematik.tr`](tr/13_matematik.tr) | Matematik: sayılarla hesap yapan yerleşikler |
| 14 | [`14_yapilar_ve_metotlar.tr`](tr/14_yapilar_ve_metotlar.tr) | Yapılar ve metotlar: ilişkili verileri ve onlara bağlı işleri bir arada tutmak |
| 15 | [`15_hata_yonetimi.tr`](tr/15_hata_yonetimi.tr) | Hata yönetimi: beklenmedik durumda programı düzgün yönetmek |
| 16 | [`16_ertele.tr`](tr/16_ertele.tr) | Ertele: temizliği her durumda yapmak |
| 17 | [`17_testler_test.tr`](tr/17_testler_test.tr) | Testler: kodun yanına test yazmak |
| 18 | [`18_moduller.tr`](tr/18_moduller.tr) | Modüller: kodu dosyalara bölüp aktarmak |
| 19 | [`19_dosya_ve_json.tr`](tr/19_dosya_ve_json.tr) | Dosya ve JSON: veriyi diske yazıp geri okumak |
| 20 | [`20_yollar_ve_klasorler.tr`](tr/20_yollar_ve_klasorler.tr) | Yollar ve klasörler: dosya sistemini yönetmek |
| 21 | [`21_csv_ve_ortam.tr`](tr/21_csv_ve_ortam.tr) | CSV, ortam değişkenleri ve zaman: dış dünyadan veri almak |
| 22 | [`22_eszamanli_gorevler.tr`](tr/22_eszamanli_gorevler.tr) | Eşzamanlı görevler: işleri aynı anda yürütmek |
| 23 | [`23_tensor.tr`](tr/23_tensor.tr) | Tensörler: yapay zekânın sayı dizileri |
| 24 | [`24_tensor_matematigi.tr`](tr/24_tensor_matematigi.tr) | Tensör matematiği: işlemleri bir tensörün her elemanına uygulamak |
| 25 | [`25_secme_ve_siralama.tr`](tr/25_secme_ve_siralama.tr) | Seçme ve sıralama: tensörde koşula göre seçmek, en büyükleri bulmak |
| 26 | [`26_turev.tr`](tr/26_turev.tr) | Otomatik türev: gradyanı bilgisayara hesaplatmak |
| 27 | [`27_ozel_turev.tr`](tr/27_ozel_turev.tr) | Özel türev: kendi türevlenebilir işlemini yazmak ve doğrulamak |
| 28 | [`28_egitim_dongusu.tr`](tr/28_egitim_dongusu.tr) | Eğitim döngüsü: bir modelin veriden öğrenmesi |
| 29 | [`29_iyilestiriciler.tr`](tr/29_iyilestiriciler.tr) | İyileştiriciler: ağırlıkları güncellemenin yolları |
| 30 | [`30_xor_agi.tr`](tr/30_xor_agi.tr) | Küçük sinir ağı: iki katmanlı bir ağ XOR'u öğrenir |
| 31 | [`31_katmanlar.tr`](tr/31_katmanlar.tr) | Katmanlar: sinir ağlarının hazır yapı taşları |
| 32 | [`32_agirliklari_kaydet.tr`](tr/32_agirliklari_kaydet.tr) | Ağırlıkları kaydetmek: eğitilmiş modeli diske yazıp geri yüklemek |
| 33 | [`33_cihaz_ve_hassasiyet.tr`](tr/33_cihaz_ve_hassasiyet.tr) | Cihaz ve hassasiyet: tensörlerin nerede ve hangi sayı türüyle yaşadığı |
| 34 | [`34_metin_uretimi.tr`](tr/34_metin_uretimi.tr) | Metin üretimi: modelin çıktısından örnek seçmek |
| 35 | [`35_transformer.tr`](tr/35_transformer.tr) | Transformer: elle yazılmış küçük bir transformer |
| 36 | [`36_karakter_modeli.tr`](tr/36_karakter_modeli.tr) | Karakter modeli: gerçek metinle eğitilen küçük bir dil modeli (~100 bin parametre) |
| 37 | [`37_komut_satirinda_ayar.tr`](tr/37_komut_satirinda_ayar.tr) | Komut satırından ayar: programı koda dokunmadan ayarlamak |
| 38 | [`38_ilerleme_ve_gunluk.tr`](tr/38_ilerleme_ve_gunluk.tr) | İlerleme çubuğu ve günlük: uzun işi izlemek ve kaydını tutmak |
| 39 | [`39_veri_ile_egitim.tr`](tr/39_veri_ile_egitim.tr) | Veri ile eğitim: parçalayıcı ve veri yükleyiciyle uçtan uca küçük bir dil modeli |
| 40 | [`40_kontrol_noktasi.tr`](tr/40_kontrol_noktasi.tr) | Kontrol noktası: uzun eğitimi kaldığı yerden sürdürmek |
| 41 | [`41_gpt.tr`](tr/41_gpt.tr) | Hazır katmanlarla GPT: dilin kitaplığındaki katmanlar ve iyileştiricilerle küçük bir dil modeli |
| 42 | [`42_hata_ayiklama.tr`](tr/42_hata_ayiklama.tr) | Hata ayıklama: koddaki kesme noktasıyla bir hesap hatasını çalışırken bulup düzeltmek |
| 43 | [`43_hata_ayiklama_alistirmasi.tr`](tr/43_hata_ayiklama_alistirmasi.tr) | Hata ayıklama alıştırması: VS Code hata ayıklayıcısıyla bir programı adım adım izlemek |
| 44 | [`44_egitim_kararliligi.tr`](tr/44_egitim_kararliligi.tr) | Eğitimin kararlılığı: NaN'i ve kayıp sıçramasını yakalamak, son kontrol noktasına dönüp sorunlu toplu işleri atlamak |
| 45 | [`45_tekrar_oynat.tr`](tr/45_tekrar_oynat.tr) | Kaydedip oynatmak: sık tekrarlanan küçük bir GPU adımını bir kez kaydedip tek seferde oynatmak (`TR_fn_replay`, `TR_fn_write_slice`) |
| 46 | [`46_hazir_model_ve_uretim.tr`](tr/46_hazir_model_ve_uretim.tr) | Hazır model ve üretim: Hugging Face biçimindeki bir modeli yüklemek (`TR_fn_pretrained_model`, `ağırlık_türü`) ve `TR_mem_generate` ile parça üretmek (küçük, rastgele ağırlıklı model `tests/data/hf_tiny`) |
| 47 | [`47_kendi_uretim_dongusu.tr`](tr/47_kendi_uretim_dongusu.tr) | Kendi üretim döngün: `TR_mem_build_cache`, önbellekli `ileri`, `TR_mem_strip`; elle yazılan döngü `TR_mem_generate` ile aynı diziyi verir; `gizli_durum` + `TR_mem_single_step` + `TR_fn_replay` |
| 48 | [`48_egitim_kosusu.tr`](tr/48_egitim_kosusu.tr) | Eğitim koşusu: kayıt işlerini kitaplığa bırakmak (`TR_type_Run`: klasör, günlük, çubuk, kontrol noktası, ağırlıklar önce) ve uzun koşudan önce `prova` |
| 49 | [`49_programin_sonu.tr`](tr/49_programin_sonu.tr) | Programın sonu: `TR_exit` ile kendi çıkış kodu, `TR_handle_stop` ve `TR_stop_requested` ile Ctrl+C'yi programın kendisinin karşılaması |

**Hata örnekleri.** [`examples/tr/hatalar/`](tr/hatalar/) sık yapılan hataları tek tek gösterir: her dosya bir hatayı bilerek yapar, yorumunda iletinin kodunu, nedenini ve düzeltilmiş hâlini yazar (çıkış kodu 1).

| No | Örnek | Hata |
| :-- | :-- | :-- |
| 01 | [`01_sozdizimi_kapanmayan_blok.tr`](tr/hatalar/01_sozdizimi_kapanmayan_blok.tr) | Kapatılmayan blok (`H0156`) |
| 02 | [`02_sozdizimi_ad_bekleniyor.tr`](tr/hatalar/02_sozdizimi_ad_bekleniyor.tr) | Ad bekleniyordu (`H0100`) |
| 03 | [`03_sozdizimi_kapanmayan_metin.tr`](tr/hatalar/03_sozdizimi_kapanmayan_metin.tr) | Kapatılmayan metin (`H0010`) |
| 04 | [`04_tanimsiz_ad.tr`](tr/hatalar/04_tanimsiz_ad.tr) | Tanımsız ad (`H1010`) |
| 05 | [`05_tip_uyusmazligi.tr`](tr/hatalar/05_tip_uyusmazligi.tr) | Tip uyuşmazlığı (`H1001`) |
| 06 | [`06_sabit_degistirme.tr`](tr/hatalar/06_sabit_degistirme.tr) | Sabiti değiştirmek (`H1002`) |
| 07 | [`07_yanlis_tip_arguman.tr`](tr/hatalar/07_yanlis_tip_arguman.tr) | Yanlış tipte argüman (`H1004`) |
| 08 | [`08_eksik_arguman.tr`](tr/hatalar/08_eksik_arguman.tr) | Eksik argüman (`H2414`) |
| 09 | [`09_bilinmeyen_adli_arguman.tr`](tr/hatalar/09_bilinmeyen_adli_arguman.tr) | Bilinmeyen adlı argüman (`H2410`) |
| 10 | [`10_sinir_disi_indeks.tr`](tr/hatalar/10_sinir_disi_indeks.tr) | Sınır dışı indeks (`H2515`) |
| 11 | [`11_sifira_bolme.tr`](tr/hatalar/11_sifira_bolme.tr) | Sıfıra bölme (`H2470`) |
| 12 | [`12_yakalanmamis_firlat.tr`](tr/hatalar/12_yakalanmamis_firlat.tr) | Yakalanmayan fırlat (`H2441`) |
| 13 | [`13_yok_uzerinde_ozellik.tr`](tr/hatalar/13_yok_uzerinde_ozellik.tr) | yok değerin özelliği (`H2466`) |
| 14 | [`14_alan_bulunamadi.tr`](tr/hatalar/14_alan_bulunamadi.tr) | Olmayan alan (`H2468`) |
| 15 | [`15_donusum_hatasi.tr`](tr/hatalar/15_donusum_hatasi.tr) | Dönüşüm hatası (`H2640`) |
| 16 | [`16_fonksiyon_degil.tr`](tr/hatalar/16_fonksiyon_degil.tr) | Fonksiyon olmayanı çağırmak (`H2421`) |
| 17 | [`17_dongu_tipi.tr`](tr/hatalar/17_dongu_tipi.tr) | Gezilemeyen değer (`H2443`) |
| 18 | [`18_sonsuz_ozyineleme.tr`](tr/hatalar/18_sonsuz_ozyineleme.tr) | Sonsuz özyineleme (`H2422`) |
| 19 | [`19_dosya_bulunamadi.tr`](tr/hatalar/19_dosya_bulunamadi.tr) | Dosya bulunamadı (`H2100`) |
| 20 | [`20_modul_bulunamadi.tr`](tr/hatalar/20_modul_bulunamadi.tr) | Modül bulunamadı (`H2112`) |
| 21 | [`21_tensor_sekil.tr`](tr/hatalar/21_tensor_sekil.tr) | Tensör şekil uyuşmazlığı (`H6126`) |
| 22 | [`22_tensor_eksen.tr`](tr/hatalar/22_tensor_eksen.tr) | Geçersiz eksen (`H6016`) |
| 23 | [`23_devam_disinda.tr`](tr/hatalar/23_devam_disinda.tr) | Döngü dışında devam (`H0100`) |


Kendi denemeleriniz için kökte bir `scratch/` klasörü açıp örnekleri oraya kopyalayabilirsiniz (ör. `examples/tr/01_merhaba.tr`); bu klasör depoya girmez.

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
| `etki` | Bir ad ya da dosya değişirse etkilenenler | `tr dizin etki lib/layers.tr` |
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
