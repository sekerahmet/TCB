# 20. Kontrol noktası ve kaldığı yerden devam

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "20. Kontrol noktası".

Her kuralın emsali ve gerekçesi: [`rationale/20_checkpoint.md`](rationale/20_checkpoint.md).

Uzun bir eğitim kesilebilir: not defterinin oturumu kapanır, Ctrl+C'ye basılır, makine yeniden başlar. `TR_Checkpoint` eğitimin bütün durumunu koşu klasörüne atomik olarak kaydeder; program aynı komutla yeniden çalıştırılınca kaldığı adımdan, kesintisiz eğitimle bit düzeyinde aynı sonuca gidecek biçimde sürer. Kullanım ve koşu klasörünün önerilen düzeni: [`docs/guide/09_uzun_egitim.md`](../guide/09_uzun_egitim.md).

## Kurulum ve nesneler

**20.1** `TR_Checkpoint(koşu, nesneler, TR_every_minutes: 15, TR_keep: 2)` bir kontrol noktası kurar.
- `koşu` bir `TR_String` ya da `TR_Path`'tir; kontrol noktası oraya yazdığı için yol 13'ün **yazma** kuralıyla, çalışma klasörüne göre çözülür (programın klasörüne göre değil). Noktalar her zaman `<koşu>/checkpoints/` altına yazılır; koşu klasöründeki öbür dosyalara dokunulmaz.
- Klasörde nokta yoksa koşu yenidir: `kn.TR_resume_step` 0'dır, klasör ilk kayıtta oluşur.
- `TR_every_minutes` kayıt aralığıdır (0 ile 1 000 000 dakika arası; 0 her `TR_update`'te kaydeder); `TR_keep` saklanacak en yeni nokta sayısıdır (1 ile 1000 arası).
- Koşu yolu bir dosyaysa, argüman yanlışsa hata.
- *Test:* TR testi.

**20.2** `nesneler` kaydedilecek şeylerin sözlüğüdür; anahtarları metindir (3.7) ve olduğu gibi saklanır. Ad boş olmayan, en çok 200 baytlık, `/` ve denetim karakteri içermeyen bir metindir; en çok 1000 nesne. Değerler:
- **Model:** tensör içeren bir yapı, liste ya da sözlük, ya da tek bir tensör. İçindeki bütün yaprak tensörler kaydedilir, her cinste (15.1): türev izleyenler (parametreler) ve izlemeyenler (durum: TR ile yazılmış iyileştiricinin momentumu, sabit tablolar, tam sayı tamponları). Adlar modelin içindeki yol adlarıdır (`m/w`, `m/kos`): parametrelerinki `TR_named_parameters`'ınkiyle (13.8a) aynıdır, durum tensörleri de aynı yolla adlanır. Aynı tensör bir kez kaydedilir. Ara sonuç (türev izleyen bir işlemin sonucu) kaydedilmez, tek başına verilirse hata. Modelin tensör olmayan alanları kaydedilmez.
- **İyileştirici** (`TR_AdamW`, `TR_SGD`): `op.TR_state` (20.12).
- **Okuyucu** (`TR_Sampler`, `TR_SequentialReader`): `ö.TR_state` (20.13).
- **Günlük** (`TR_Logger`): dosyanın o anki bayt uzunluğu.
- **Düz değerler:** yalnız `TR_Int`, `TR_Float`, `TR_String`, `TR_Bool`, `TR_none` ve iç içe liste ve sözlük içeren bir sözlük ya da liste (en iyi kayıp, sayaçlar). Yüklemede yerinde ve iç içe yenilenir: aynı yerde aynı türden kap varsa o kap güncellenir, yenisiyle değiştirilmez; başka değişkenlerden tutulan bağlar kopmaz. Kayıt paylaşımı tutmaz: aynı iç kap birden çok yerden bağlıysa yalnız ilk yerde (anahtar ve sıra düzeniyle) yerinde yenilenir; sonuç belirlenimcidir.

Düz bir sayı, metin ya da mantık değeri doğrudan verilemez (kullanıcının değişkenine geri yazılamaz); bir sözlüğe konur. Fonksiyon, ara tensör ve boş sözlük hatadır.
- *Test:* TR testi.

## Kayıt

**20.3** `kn.TR_update(n)` eğitimin adım sınırıdır: `n` tamamlanan adım sayısıdır (0 ya da pozitif `TR_Int`). Son kayıttan ya da kurulumdan beri `TR_every_minutes` geçtiyse nokta bu adımla kaydedilir ve `TR_true` döner; geçmediyse `TR_false`. Kayıt yalnız adım sınırında yapılır, yarım bir adım hiç kaydedilmez; `TR_update` iyileştiricinin adımından sonra çağrılır. Gradyanlar kaydedilmez.
- *Test:* TR testi.

**20.4** `kn.TR_save()` noktayı hemen, son `TR_update` adımıyla (hiç çağrılmadıysa `TR_resume_step` ile) kaydeder ve noktanın klasörünü `TR_Path` olarak verir: `<koşu>/checkpoints/step_0000001500` (adım 10 basamağa sıfırla doldurulur). Aynı adımın noktası varsa yerine konur. `TR_save`'deki yazma hatası açık hatadır (20.10'dan farklı olarak eğitim sürmez).
- *Test:* TR testi.

**20.5** Bir nokta klasörü üç dosyadır:
- `tensors.trt`: modellerin tensörleri ve iyileştirici durumları, `TR_save_tensors` biçiminde, adları `<nesne>/<tensör adı>`;
- `state.json`: biçim sürümü, adım, zaman, program, işçi sayısı, komut satırı ayarları (gizli değerler maskeli), nesnelerin kaydı (okuyucu durumları, düz değerler, günlük uzunlukları). Düz değerdeki sonlu olmayan sayı `"NaN"`, `"Infinity"`, `"-Infinity"` yazılır ve yalnız bu yazımla okunur (19);
- `digest.json`: öbür iki dosyanın bayt sayısı ve SHA-256 özeti; en son yazılır.

`checkpoints/latest.json` en son noktayı söyler; araçlar içindir, yükleme klasörlere bakar. Dosya ve alan adları dilden bağımsız İngilizce tanımlayıcılardır, iyileştiricinin adı sınıflı kimliğidir (`"optimizer": "TR_fn_AdamW"`, 17.8). **Atomik yazım:** nokta önce geçici bir klasöre yazılıp diske işlenir, tek bir yeniden adlandırmayla yerine konur, `latest.json` atomik güncellenir, sonra `TR_keep`'i aşan eski noktalar silinir. Hangi aşamada kesilirse kesilsin önceki geçerli nokta bozulmaz.
- *Test:* TR testi.

## Devam

**20.6** Aynı koşu klasörüyle yeniden kurulan kontrol noktası en yeni noktayı yükler ve standart hataya kaldığı yerden devam ettiğini adımıyla yazar.
- `kn.TR_resume_step` o adım olur.
- Modellerin tensörleri **var olan** tensörlere yerinde yazılır; iyileştirici aynı tensörleri güncellemeyi sürdürür, cihazlar değişmez.
- İyileştirici durumu (momentler, adım sayaçları, 16 bitlik parametrelerin ana kopyaları) parametrelerin cihazına yüklenir.
- Okuyucu kaydedilen konumdan sürer: sonraki toplu iş kesintisiz eğitimdekiyle aynıdır (16.7).
- Düz değerler yerinde yenilenir; `TR_Float` `TR_Float` kalır, `TR_Int` anahtarlar korunur, NaN ve sonsuz korunur.
- Günlüklere bir devam kaydı yazılır (19).

Kesintisiz eğitim ile kesilip sürdürülen eğitim aynı son adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu ve kayıp dizisini verir (20.17).
- *Test:* TR testi; Go: `TestResumedSameAsUninterrupted`.

**20.7** Yüklemeden önce bütün nesnelerin kayda uyduğu denetlenir; uymazsa yol gösterici bir hata verilir ve **hiçbir nesne değişmez**.
- Uyması gerekenler: nesnelerin adları ve türleri; modelin tensörlerinin adları, şekilleri, cinsleri ve hassasiyetleri; iyileştiricinin tipi, parametre sayısı ve durumunun şekilleri; okuyucunun türü, toplu iş boyu, bağlamı, tohumu ve veri kümesi. Bu, programdaki farktır: önceki noktayı önermez (20.9 bozukluğu ayırır).
- Komut satırı ayarları kayıttakinden farklıysa yalnız bir not yazılır, yükleme sürer (adım sayısını artırıp eğitimi uzatmak gibi).
- **İki aşama:** önce her şey denetlenir ve yazılacak yerlerin yazılabilir olduğu sınanır; hiçbir nesne değişmez. Hepsi geçerse parametreler, okuyucular, iyileştiriciler ve düz değerler bu sırayla yazılır.
- Yazma aşamasında yalnız kurtarılamayan hatalar kalır (cihaz belleğinin bitmesi, dosyanın doğrulandıktan sonra değişmesi): parametreler yarıda kalabilir; bu hatadan sonra program yeniden başlatılıp nokta yeniden yüklenir.
- *Test:* TR testi; Go: `TestCorruptOptimizerStateChangesNothingAndSuggestsPrevious`, `TestRollbackStopsBeforeWritingWhenAParamIsUnusable`, `TestLoadStateChecksEverythingBeforeWriting`, `TestCorruptContentSuggestsPreviousButMismatchDoesNot`.

**20.8** Her başarılı kayıttan sonra en yeni `TR_keep` nokta (ve az önce yazılan) kalır, eskileri silinir.
- *Test:* TR testi.

**20.9** En yeni noktanın bir dosyası eksikse, boyu ya da özeti `digest.json`'la tutmuyorsa, biçimi bozuksa ya da dosyaları sağlam ama içeriği kurala uymuyorsa (bilinmeyen durum anahtarı, okuyucuda geçersiz konum) nokta bozuktur ve kurulum açık hata verir. İleti hangi noktanın hangi dosyası olduğunu, bir önceki geçerli noktayı ve ondan sürdürmek için ne yapılacağını söyler. **Bozuk nokta kendiliğinden atlanmaz:** kullanıcı neyi kaybettiğini bilerek seçer. Yarıda kalmış bir kaydın geçici klasörü sessizce silinir; yerine yenisi konamadan kesilmiş aynı adımlı nokta geri konur.
- *Test:* TR testi; Go: `TestTamperedCheckpointRejected`, `TestStrayTempCheckpointCleaned`, `TestCorruptContentSuggestsPreviousButMismatchDoesNot`.

**20.10** Aralıklı kayıtta (`TR_update`) yazma hatası (disk dolu, izin yok, ağ ya da bulut sürücüsünde yeniden adlandırma hatası) eğitimi durdurmaz: `TR_update` `TR_false` döner, standart hataya son geçerli noktayı da söyleyen bir uyarı yazılır, sonraki aralıkta yeniden denenir. Art arda üçüncü başarısızlık açık hatadır. Başarılı bir kayıt sayacı sıfırlar; önceki geçerli nokta hiçbir durumda bozulmaz.
- *Test:* TR testi; Go: `TestRenameErrorDoesNotStopTraining`.

## Durumun parçaları

**20.11** `TR_load_parameters(model, sözlük)` ad → tensör sözlüğünü (ör. `TR_load_tensors`'ın sonucu) modelin var olan parametrelerine, `TR_named_parameters`'ın adlarıyla yerinde yazar ve yazılan parametre sayısını (`TR_Int`) verir.
- Tensörler parametrenin cihazına taşınır; hassasiyeti farklıysa çevrilir, cinsi farklıysa hata (15). Deponun sürümü artar (15).
- Modelde olup sözlükte olmayan, sözlükte olup modelde olmayan ve şekli farklı ad hatadır; hiçbir şey yazılmaz.
- Hazır ağırlıklarla ince ayar için de kullanılır.
- *Test:* TR testi.

**20.12** `op.TR_state` iyileştiricinin durumunu ad → tensör sözlüğü olarak (kopya) verir: AdamW'de "step", "step.k", "m.k", "v.k"; momentumlu SGD'de "momentum.k"; 16 bitlik parametrelerde "master.k" (k parametrenin listedeki sırası; adlar dilden bağımsızdır). Öğrenme oranı gibi ayarlar duruma girmez. `op.TR_load_state(sözlük)` durumu aynı tipte ve aynı parametre listesiyle kurulmuş bir iyileştiriciye yükler ve değer döndürmez (13). Bilinmeyen anahtar, şekli tutmayan girdi ve tensör olmayan değer hatadır; iyileştirici değişmez.
- *Test:* TR testi.

**20.13** `ö.TR_state` okuyucunun kaldığı yeri düz değerli bir sözlük olarak verir; anahtarlar metindir: "kind" ("sampler", "sequential"), "batch", "context", (örnekleyicide) "seed", "dataset", (örnekleyicide) "epoch", "position", "window", "counter", (sıralıda) "done". Konum, `.TR_next()` ile alınan son toplu işten sonraki konumdur; önden yüklenip bekleyenler sayılmaz. `ö.TR_load_state(sözlük)` aynı ayarla kurulmuş bir okuyucuyu o konuma getirir ve değer döndürmez: sonraki `.TR_next()` kaydedilen okuyucunun vereceği toplu işi verir. Ayarı ya da veri kümesi farklı kayıt ve eksik alan hatadır; okuyucu değişmez.
- *Test:* TR testi.

**20.14** `kn.TR_rollback()` en yeni geçerli noktayı aynı programda yerinde yükler ve noktanın adımını (`TR_Int`) verir. Yükleme kurulumdakiyle aynı yoldur (20.6, 20.7, 20.9); günlüklere devam kaydı yazılır. `TR_resume_step` değişmez. Nokta yoksa açık hatadır. Bozuk bir adımdan geri dönüp sürdürülen eğitim, hiç bozulmamış eğitimle bit düzeyinde aynı biter. Bozuk adım kaydedilmeden önce dönülmelidir: denetim `TR_update`'ten önce yapılır.
- *Test:* TR testi; Go: `TestRollbackAfterNaNSameAsUninterrupted`, `TestRollbackWithoutCheckpointIsError`.

**20.15** `ö.TR_skip(n)` okuyucunun sıradaki `n` toplu işini tensör kurmadan atlar ve atladığı sayıyı (`TR_Int`) verir: sonraki `.TR_next()`, `n` tane `.TR_next()`'ten sonra gelecek toplu işin aynısıdır ve durumun "counter"'ı `n` artar. Sıralı okuyucu biterse kalan kadar atlar. `n = 0` bir şey yapmaz; eksi sayı hatadır.
- *Test:* TR testi.

## Durdurma, belirlenimcilik ve dosyalar

**20.16** Kontrol noktası kuruluyken gelen durdurma isteği (1.5) 1.6'nın kuralıyla ertelenir: istekten sonraki ilk `kn.TR_update` ya da `kn.TR_save()` bütün durumu kaydeder, sonra program 1.5'teki gibi durur ve süreç isteğin sinyaliyle biter (1.9). Ertelemenin koşulları ve süre sınırı 1.6'dadır. Bu bölüm yalnız kaydı söyler:
- Standart hataya adım bitince kaydedilip çıkılacağı ve hemen çıkmak için yeniden istenebileceği yazılır; kayıttan sonra kaydedilen adım yazılır.
- Bu kayıt yazılamazsa uyarı verilir ve program yine durur; sürdürülünce son geçerli noktadan devam edilir.
- Sürdürme bu kayıttan olur. Süreç öldürülürse (elektrik kesintisi) en son düzenli noktadan sürülür; gerçek güvence düzenli kayıttır.
- *Test:* Go: `TestDeferredInterruptSavesAtStepBoundary`, `TestSaveHonoursTheDeferredRequestAndStops`, `TestStopBeforeTheSecondUpdateIsNotDeferred`.

**20.17** Bit düzeyinde aynılık. Kesilip sürdürülen eğitim kesintisiz eğitimle bit düzeyinde aynı parametreleri, iyileştirici durumunu, kayıp dizisini ve düz değerleri verir.
- **Koşul, aynı cihaz türü:** kayıt ile devam aynı TR ikilisiyle ve aynı cihaz türünde (aynı GPU modeli ya da aynı CPU özellik kümesi) yapılır. CPU'nun indirgemeleri iş parçacığı sayısından bağımsızdır: 8 iş parçacığıyla başlayıp 1 ile sürdürülebilir.
- **Rastgelelik:** tohumlu rastgelelik zaten aynıdır. Paylaşılan rastgele üreteç (tohumsuz `TR_randn`, `TR_sample`) her programda aynı sabit durumla başlar ve durumu kontrol noktasına kaydedilip geri yüklenir: tohumsuz rastgelelik de hem yeniden çalıştırmada hem kaldığı yerden aynı sonucu verir. Sınırlar: eşzamanlı görevler aynı üreteçten çekerse çekme sırası, dolayısıyla sonuç, belirsizdir; görev başına tohum verilir. Bir programda birden çok kontrol noktası varsa üretecin durumu her birine yazılır; yüklenen sonuncusu geçerlidir.
- **GPU'da** parametreler ve iyileştirici durumları kayıtta konağa indirilir, yüklemede aynı cihaza konur.
- *Test:* Go: `TestCheckpointResumeBitExact`; paylaşılan üretecin kaydı uygulanınca TR testi.

**20.18** Kontrol noktası dosyaları güvenilmeyen girdidir (başkasından gelen ya da yarım kopyalanmış koşu klasörü).
- `digest.json` en çok 64 KB, `state.json` en çok 64 MB okunur; JSON katı çözülür (bilinmeyen alan ve fazladan veri hata).
- Biçim sürümü 3'tür ve yalnız o okunur (dil taslaktır, 17.10); daha yenisi hata verir.
- Adım, klasör adında, `digest.json`'da ve `state.json`'da aynı olmalıdır; tensör dosyası `TR_load_tensors`'ın bütün sınırlarıyla okunur.
- Biçimde yalnız veri vardır: kod çalıştıran, nesne kuran ya da yol çözen alan yoktur.
- Büyük modeller bellekte ikinci bir kopya toplanmadan yazılır ve okunur. Kayıt sürerken eşzamanlı görevler bekler.
- *Test:* Go: `FuzzCheckpointFiles`, `TestTamperedCheckpointRejected`, `TestCheckpointFormatIsLanguageIndependent`.
