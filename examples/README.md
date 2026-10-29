# Örnekler

Numaralı örnekler basitten zora bir öğrenme yoludur: her biri az sayıda yeni ad getirir ve öncekilerde öğretilmemiş hiçbir şeyi kullanmaz. Örnekler [`tr/`](tr/) klasöründedir ve kodlu saklanır: aynı dosya her gösterim dilinde okunur (`tr show tr/01_merhaba.tr --lang en` İngilizce gösterir). Yol, dilin bütün adlarını kapsar; kapsamı `go test ./cmd/tr/e2e -run TestExamplesCover` denetler. Tam liste ve ne öğrettikleri aşağıdadır; örneklerin hangi adları getirdiğini sırayla görmek için başlıklara bakın.

Her örnek tek başına çalışır:

```sh
tr examples/tr/01_merhaba.tr        # ya da: tr run examples/tr/01_merhaba.tr
TR_LANG=en tr examples/tr/01_merhaba.tr   # aynı program, iletiler İngilizce
```

## Sıralı liste

Örnekler basitten zora numaralı bir öğrenme yoludur: her örnek tek bir yeni kavram getirir, öncekilerde öğretilmemiş hiçbir şeyi kullanmaz; başında ne öğrettiğini ve önceki örnekten ne aldığını söyler, sonunda "Kendin dene" alıştırmaları vardır. **Örnekler kodlu saklanır** ([`examples/tr/`](tr/)): aynı dosya her gösterim dilinde okunur, `tr show examples/tr/01_merhaba.tr --lang en` İngilizce gösterir (bkz. [dil tanımı 17](../docs/spec/17_kodlar.md)). Proje kökünden çalıştırın: `./bin/tr.exe examples/tr/01_merhaba.tr`.

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

Örneklerin hepsi bir `.tr` dosyasıdır; dilin komut satırı adları (`K` ailesi) dosyada geçmez, burada gösterilir. Komut, alt komut, seçenek, kabuk ve hata ayıklayıcı komutları ile ortam değişkenleri kimlik adıyla yazılır (17.9).

| Komut | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `help` | Komutların ve seçeneklerin kısa yardımı | `tr help` |
| `version` | TR'nin sürümü | `tr version` |
| `run` | Programı çalıştırır (komut adı yazılmadan verilen dosya da çalışır) | `tr run examples/tr/07_donguler.tr` |
| `check` | Çalıştırmadan sözdizimi ve tip denetimi | `tr check examples/tr/13_matematik.tr` |
| `build` | Programı yorumlayıcıyla birlikte tek çalıştırılabilir dosyaya paketler | `tr build examples/tr/18_moduller.tr` |
| `test` | Dosyadaki ya da klasördeki `*_test.tr` testlerini koşar | `tr test examples/tr/17_testler_test.tr` |
| `format` | Dosyaları standart biçime getirir | `tr format examples/tr/09_listeler.tr` |
| `show` | Kodlu dosyayı değiştirmeden bir yazım dilinde yazar | `tr show examples/tr/01_merhaba.tr --lang en` |
| `debug` | Programı terminal hata ayıklayıcısında başlatır | `tr debug examples/tr/42_hata_ayiklama.tr` |
| `watch` | Çalışan bir eğitimin canlı profil tablosunu gösterir | `tr watch examples/tr/40_kontrol_noktasi.tr` |
| `profile` | Biten ya da süren bir eğitimin profil raporunu yazar | `tr profile examples/tr/28_egitim_dongusu.tr` |
| `dap` | Düzenleyicinin hata ayıklama protokolü sunucusu (düzenleyiciler kullanır) | VS Code F5 |
| `index` | TR kodunun dizinini kurar ve özetler; altındaki alt komutlarla ona sorulur | `tr index` |

`index` komutunun alt komutları yalnızca onun altında geçerlidir (`tr definition` bir komut değildir):

| Alt komut | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `definition` | Bir adın tanımı: nerede, imzası, belge yorumu, üyeleri | `tr index definition Doğrusal` |
| `references` | Bir ada başvuranlar: çağrı, okuma, yazma, aktarma, test | `tr index references Doğrusal` |
| `tested_by` | Bir adı sınayan test blokları (doğrudan, sonra çağrı zinciriyle dolaylı) | `tr index tested_by Doğrusal` |
| `impact` | Bir ad ya da dosya değişirse etkilenenler | `tr index impact lib/layers.tr` |
| `rename` | Bir ad değişirse değişecek yerler: kesin ve olası (hiçbir dosyayı değiştirmez) | `tr index rename baş bakış_sayısı` |

Seçenekler (komutlardan sonra, iki tireyle; adın sözcükleri tireyle ayrılır):

| Seçenek | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `--lang` | Yazım dilini seçer (`tr`, `en`) | `tr show examples/tr/05_kosullar.tr --lang en` |
| `--check` | `format` ile dosyayı değiştirmeden standart biçimde olup olmadığını denetler | `tr format --check examples` |
| `--json` | `profile` raporunu JSON olarak yazar | `tr profile --json rapor.json examples/tr/28_egitim_dongusu.tr` |
| `--go-profile` | `profile` ile yorumlayıcının kendi Go profilini de yazar | `tr profile --go-profile examples/tr/28_egitim_dongusu.tr` |
| `--output` | `watch` tablosunun yazılacağı dosyayı seçer | `tr watch --output canli.txt examples/tr/40_kontrol_noktasi.tr` |
| `--limit` | `index` sorgularında bir listedeki en çok satır (varsayılan 12) | `tr index references Doğrusal --limit 5` |
| `--all` | `index` sorgularında satır sınırını kaldırır | `tr index references Doğrusal --all` |
| `--root` | `index` için proje kökü (verilmezse `.git` bulunan üst klasör) | `tr index definition Doğrusal --root .` |
| `--kind` | `index` sorgu adını yalnızca bu türdeki düğümlere çözer | `tr index definition ileri --kind method` |
| `--no-index` | `index` sorgusunda önbellekteki dizini olduğu gibi kullanır (tazelemez) | `tr index definition Doğrusal --no-index` |
| `--status` | `index` komutunda dizin önbelleğinin durumunu gösterir | `tr index --status` |
| `--clear` | `index` komutunda dizin önbelleğini siler | `tr index --clear` |
| `--no-text` | `rename` sorgusunda belge, yorum ve ileti metni taramasını kapatır | `tr index rename baş bakış --no-text` |
| `--term` | `rename` sorgusunda sembol aramaz, sözcüğü yalnızca metinde arar | `tr index rename çekirdek kernel --term` |
| `--help` | Programın `komut_satırı` ile tanımlanan seçeneklerinin yardımı | `tr examples/tr/39_veri_ile_egitim.tr --help` |

Ortam değişkenleri (kimlik adıyla yazılır ve okunur, 17.9):

| Değişken | Ne yapar | Örnek |
| :-- | :-- | :-- |
| `TR_LANG` | Yazım dilini seçer (`tr`, `en`): iletiler, yardım, kodlu dosyanın görünümü | `TR_LANG=en tr help` |
| `TR_JOB_DIR` | Arka plan işinin klasörü: ilerleme çubuğu oraya `progress.json` yazar | `TR_JOB_DIR=isler/egitim tr examples/tr/28_egitim_dongusu.tr` |
| `TR_GPU_SYNC` | `1` ise her GPU kernelinden sonra beklenir; kernel hatası tam yerinde bildirilir (yavaş) | `TR_GPU_SYNC=1 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |
| `TR_GPU_PROFILE` | `1` ise GPU kernel süreleri ölçülür (`watch` ve `profile` kendileri açar) | `TR_GPU_PROFILE=1 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |
| `TR_CPU_AVX2` | `0` ise CPU'nun AVX2/FMA kernelleri kapanır, taşınabilir Go kernelleri çalışır | `TR_CPU_AVX2=0 tr examples/tr/23_tensor.tr` |
| `TR_GPU_CUBLASLT` | Bulunamazsa cuBLASLt kitaplığının tam yolu | `TR_GPU_CUBLASLT=/usr/local/cuda/lib64/libcublasLt.so.12 tr examples/tr/33_cihaz_ve_hassasiyet.tr` |

Etkileşimli kabuk `tr` ile açılır (dosya verilmeden); kabuktan `exit` ile çıkılır.

Hata ayıklayıcı komutları ([42](tr/42_hata_ayiklama.tr) ve [43](tr/43_hata_ayiklama_alistirmasi.tr)'te kullanılır; VS Code'da aynı işler F5, F10, F11 tuşlarıdır):

| Komut | Ne yapar |
| :-- | :-- |
| `step` | Tek adım ilerler, fonksiyonların içine girer (F11) |
| `next` | Üstten geçer: fonksiyonu tek adımda çalıştırır (F10) |
| `out` | Çalışan fonksiyondan çıkana kadar çalıştırır (Shift+F11) |
| `continue` | Sonraki kesme noktasına kadar devam eder (F5) |
| `break` | Bir satıra kesme noktası koyar ya da kaldırır |
| `jump` | Çalışma sırasını verilen satıra taşır |
| `set` | Çalışma anında bir değişkenin değerini değiştirir (`set vergi_orani = 0.20`) |
| `watch` | Bir ifadeyi izleme listesine ekler |
| `table` | Değişkenlerin tablosunu gösterir (`table siparisler`) |
| `stack` | Çağrı yığınını gösterir |
| `eval` | Bir ifadeyi o anki ortamda hesaplar |
| `screen` | Kaynak kodun duraklanan satır çevresini gösterir |
| `help` | Hata ayıklayıcı komutlarının yardımı |
| `quit` | Hata ayıklayıcıdan ve programdan çıkar |

## Hata örnekleri

[`tr/hatalar/`](tr/hatalar/) sık yapılan hataları tek tek gösterir: her dosya bir hatayı bilerek yapar, yorumunda nedenini ve düzeltilmiş hâlini söyler. Bu dosyalar çıkış koduyla biter; testler iletinin kodunu (`H…`) ve çıkış kodunu sınar.
