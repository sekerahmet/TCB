# 20. Kontrol noktası ve kaldığı yerden devam

Uzun bir eğitim kesilebilir: bulut not defterinin oturumu kapanır, Ctrl+C'ye basılır, makine yeniden başlar. `TR_type_Checkpoint` eğitimin bütün durumunu koşu klasörüne atomik kaydeder; program aynı komutla yeniden çalıştırılınca kaldığı adımdan, **kesintisiz eğitimle bit düzeyinde aynı** sonuca gidecek biçimde sürer. Uygulama `internal/stdlib/checkpoint`'dadır; kodlar ve örnekler [`codes.md`](../codes.md)'de (TR_type_Checkpoint, TR_type_Checkpoint, TR_fn_load_parameters, TR_arg_every_minutes, TR_arg_keep, TR_mem_update, TR_mem_state, TR_mem_load_state, TR_mem_resume_step, TR_mem_save). Kullanım için [`docs/guide/09_uzun_egitim.md`](../guide/09_uzun_egitim.md). Kural testleri `tests/kontrol_noktasi_test.tr`'dedir; süreç kesme, ayrı süreçlerle kabul testi, ağ sürücüsü benzetimi ve GPU yolu `tr test` ile sınanamadığından Go testleriyle sınanır (`internal/stdlib/checkpoint`, `cmd/tr/e2e`, `internal/stdlib/gpuprobe`).

```tr
//! tr kodlu 3 dil=tr
TR_kw_var ayar = TR_fn_command_line({TR_mem_step: 20000, koşu: "runs/deneme1"})
TR_kw_var model = {E: TR_type_Tensor(TR_fn_no_grad(TR_fn_randn([50000, 256], TR_arg_seed: 1) * 0.02), TR_arg_requires_grad: TR_kw_true)}
TR_kw_var op = TR_fn_AdamW(TR_fn_parameters(model), TR_arg_lr: 0.0003)
TR_kw_var ö = TR_type_Dataset("veri/train").TR_mem_sampler(TR_arg_batch_size: 16, TR_arg_context: 512, TR_arg_seed: 1)
TR_kw_var g = TR_type_Logger(TR_type_Path(ayar.koşu, "metrics.jsonl"))
TR_kw_var izlenen = {en_iyi_kayıp: 99.0}
TR_kw_var kn = TR_type_Checkpoint(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen})
TR_kw_var ç = TR_type_Progress(ayar.TR_mem_step, TR_arg_log: g)
TR_kw_for adım TR_kw_in kn.TR_mem_resume_step..ayar.TR_mem_step {
    TR_kw_var giriş, hedef = ö.TR_mem_next()
    op.TR_mem_zero_grad()
    TR_kw_var kayıp = TR_fn_cross_entropy(TR_fn_embedding_lookup(model.E, giriş) @ model.E.TR_mem_transpose(), hedef)
    TR_fn_backward(kayıp)
    op.TR_mem_step()
    TR_kw_if kayıp.TR_mem_tolist < izlenen.en_iyi_kayıp { izlenen.en_iyi_kayıp = kayıp.TR_mem_tolist }
    ç.TR_mem_update({TR_mem_loss: kayıp}, TR_arg_step: adım + 1)
    kn.TR_mem_update(adım + 1) // 15 dakikada bir ve Ctrl+C gelince adım sınırında kaydeder
}
kn.TR_mem_save()
```

## Kurulum ve nesneler

**20.1** `TR_type_Checkpoint(koşu, nesneler, TR_arg_every_minutes: 15, TR_arg_keep: 2)` bir `TR_type_Checkpoint` kurar. `koşu` bir `TR_type_String` ya da `TR_type_Path`'dur, 13.1'in kuralıyla çözülür; noktalar her zaman `<koşu>/checkpoints/` altına yazılır (koşu klasöründeki öbür dosyalara dokunulmaz). Klasörde nokta yoksa koşu yenidir, `kn.TR_mem_resume_step` 0'dır ve klasör ilk kayıtta oluşur. `her_dakika` kayıt aralığıdır (0 ile 1 000 000 arası dakika; 0: her `TR_mem_update`'de), `tut` saklanacak en yeni nokta sayısıdır (1 ile 1000 arası `TR_type_Int`). Koşu yolu bir dosyaysa, argüman sayısı ya da tipi yanlışsa, adlı argüman bilinmiyorsa hata verilir.

**20.2** `nesneler` kaydedilecek şeylerin sözlüğüdür; anahtarları kullanıcının verdiği adlardır: kod taşıyan çıplak anahtar kodun dilden bağımsız adıyla saklanır (17.3; Türkçe programın `{adım: a}`'sı da İngilizce programın `{step: a}`'sı da `"step"`), kodsuz çıplak anahtar ve tırnaklı ad yazıldığı gibi; böylece iki dilde yazılmış programlar aynı noktayı paylaşır. Ad boş olmayan bir `TR_type_String`dir, en çok 200 bayttır, `/` ve denetim karakteri içermez; en çok 1000 nesne. Değerler:
- **model**: tensör içeren bir yapı nesnesi, liste ya da sözlük, ya da tek bir tensör. İçindeki **bütün yaprak tensörler** kaydedilir: türev izleyenler (**parametreler**: iyileştiricinin güncellediği, `TR_fn_parameters(model)`'in verdiği tensörler) ve türev izlemeyenler (**durum**: TR ile yazılmış bir iyileştiricinin momentumu gibi türevsiz blokta yerinde güncellenen tensörler, sabit tablolar). Adlar `TR_fn_parameters(model, TR_arg_named: TR_kw_true)`'nun yol adlarıdır, aynı gezinti sırasıyla; aynı tensör bir kez kaydedilir. Yüklemede ikisi de var olan tensörlere yerinde yazılır (20.6); kayıttaki ile modeldeki tensörlerin adları, şekilleri ve türleri aynı olmalıdır (20.7). Ara sonuçlar (türev izleyen bir işlemin sonucu) kaydedilmez, tek başına verilen ara tensör hatadır; modelin tensör olmayan alanları (ayarlar, sayılar) kaydedilmez — değişen sayılar ayrı bir düz değerli sözlükte verilir;
- **iyileştirici** (`TR_fn_AdamW`, `TR_fn_SGD`): `op.TR_mem_state` (20.12);
- **okuyucu** (`TR_type_Sampler`, `TR_type_SequentialReader`): `ö.TR_mem_state` (20.13);
- **günlük** (`TR_type_Logger`): dosyanın o anki bayt uzunluğu;
- **düz değerler**: yalnızca `TR_type_Int`, `TR_type_Float`, `TR_type_String`, `TR_type_Bool`, `TR_kw_none` ve iç içe `TR_type_List`/`TR_type_Dict` içeren bir `TR_type_Dict` ya da `TR_type_List` (en iyi kayıp, sayaçlar). Yüklemede (ve `TR_mem_rollback`'de) yerinde ve iç içe yenilenir: kullanıcının değerinde aynı anahtarda ya da sırada aynı türden bir `TR_type_List` ya da `TR_type_Dict` varsa o nesne de yerinde güncellenir, yenisiyle değiştirilmez; böylece iç içe kaplara başka değişkenlerden tutulan bağlar kopmaz (`izlenen = {kayıt: kayıt}` ise yüklemeden sonra `kayıt` da kayıttaki listeyi gösterir). Kayıt (JSON) paylaşımı tutmaz: aynı iç kap birden çok yerden bağlıysa (`{a: l, b: l}`) yalnızca ilk yerde (anahtar ve sıra düzeniyle) yerinde yenilenir, öbür yerler kayıttaki kendi değerlerini alır; ikinci yenileme birinciyi ezmez ve sonuç belirlenimcidir.

Düz bir `TR_type_Int`, `TR_type_Float`, `TR_type_String` ya da `TR_type_Bool` doğrudan verilemez (geri yüklenirken kullanıcının değişkenine yazılamaz; bir sözlüğe konur: `{en_iyi: {TR_mem_tolist: 99.0}}`). Fonksiyon, ara tensör ve boş sözlük hatadır; içinde tensör olan bir sözlük düz değer değil modeldir.

## Kayıt

**20.3** `kn.TR_mem_update(adım)` eğitimin adım sınırıdır: `adım` tamamlanan adım sayısıdır (0 ya da pozitif `TR_type_Int`). Son kayıttan (ya da kurulumdan) beri `her_dakika` geçtiyse nokta bu adımla kaydedilir ve `TR_kw_true` döner; geçmediyse `TR_kw_false`. Kayıt yalnızca adım sınırında yapılır (yarım bir adım hiç kaydedilmez); `TR_mem_update` iyileştiricinin adımından sonra çağrılmalıdır. Gradyanlar kaydedilmez.

**20.4** `kn.TR_mem_save()` noktayı hemen, son `TR_mem_update` adımıyla (hiç çağrılmadıysa `TR_mem_resume_step` ile) kaydeder ve noktanın klasörünü `TR_type_Path` olarak verir: `<koşu>/checkpoints/step_0000001500` (adım 10 basamağa sıfırla doldurulur). Aynı adımın noktası varsa yenisi yerine konur. `TR_mem_save`'teki yazma hatası açık hatadır (20.10'dan farklı olarak eğitim sürmez).

**20.5** Bir nokta klasörü üç dosyadır: `tensors.trt` (modellerin tensörleri — parametreler ve türev izlemeyen durum tensörleri — ve iyileştirici durumları, `TR_fn_save_tensors` biçiminde; adları `<nesne>/<tensör adı>`), `state.json` (biçim sürümü, adım, zaman, program, işçi sayısı, komut satırı ayarları — gizli değerler maskeli —, nesnelerin kaydı: okuyucu durumları, düz değerler, günlük uzunlukları; düz değerdeki sonlu olmayan sayı günlükteki gibi `"NaN"`, `"Infinity"`, `"-Infinity"` yazılır ve yalnızca bu yazımla okunur, 19.5) ve `digest.json` (öbür iki dosyanın bayt sayısı ve SHA-256 özeti; en son yazılır). `checkpoints/latest.json` en son kaydedilen noktayı söyler (`{"version":3,"step":1500,"dir":"step_0000001500"}`; araçlar içindir, yükleme klasörlere bakar). Dosya ve klasör adları, JSON anahtarları ve nesne kayıtlarının türleri (`parameters`, `optimizer`, `stateful`, `log`, `values`) dilden bağımsız İngilizce tanımlayıcılardır, iyileştiricinin adı kodudur (`"optimizer": "TR_fn_AdamW"`; 17.3). Nokta önce `step_N.tmp-<süreç>` klasörüne yazılıp diske işlenir, sonra tek bir yeniden adlandırmayla yerine konur, `latest.json` atomik güncellenir, sonra `tut`'u aşan eski noktalar silinir: hangi aşamada kesilirse kesilsin önceki geçerli nokta bozulmaz.

## Devam

**20.6** Aynı koşu klasörüyle yeniden kurulan kontrol noktası en yeni noktayı yükler ve standart hataya `Kaldığı yerden devam: adım N (...)` yazar: `kn.TR_mem_resume_step` N olur; modellerin tensörleri (parametreler ve durum tensörleri) **var olan** tensörlere yerinde yazılır (iyileştirici aynı tensörleri güncellemeyi sürdürür; cihazları değişmez); iyileştirici durumu (momentler, adım sayaçları, 16 bitlik parametrelerin ana kopyaları) parametrelerin cihazına yüklenir; okuyucu kaydedilen konumdan sürer (sonraki toplu iş kesintisiz eğitimdekiyle aynıdır); düz değerli sözlük ve listelerin içi yerinde yenilenir (değişken aynı değeri gösterir; `TR_type_Float` `TR_type_Float` kalır, `TR_type_Int` anahtarlar, NaN ve sonsuz korunur); günlüklere `{"tür":"devam","adım":N}` kaydı yazılır (19, günlük okuyucu kuralı). Kesintisiz eğitim ile N. adımda kesilip sürdürülen eğitim aynı son adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu ve geçerli kayıp dizisini verir.

**20.7** Yüklemeden önce bütün nesnelerin kayda uyduğu denetlenir; uymazsa yol gösterici bir hata verilir ve **hiçbir nesne değişmez**: kayıttaki nesnelerin adları ve türleri, modelin tensörlerinin (parametreler ve durum tensörleri) adları, şekilleri ve veri türleri, iyileştiricinin tipi ve parametre sayısı (ve durum anahtarlarının şekilleri), okuyucunun türü, `toplu`, `bağlam`, `tohum` değerleri ve veri kümesi (veri dosyalarının adları ve parça sayıları) aynı olmalıdır. Yükleme iki aşamalıdır: önce iyileştirici durumları bütün kurallarıyla (adım sayaçları, moment çiftleri, momentli parametrenin sayacı, tensörlerin cihazı ve canlılığı) denetlenir, modelin tensörlerinin yazılabilir olduğu sınanır ve okuyucular yeni konumlarına hazırlanır; hiçbir nesne değişmez. Hepsi geçerse parametreler, okuyucular, iyileştiriciler ve düz değerler bu sırayla yazılır (Go testleri: `internal/stdlib/checkpoint` `TestCorruptOptimizerStateChangesNothingAndSuggestsPrevious`, `TestRollbackStopsBeforeWritingWhenAParamIsUnusable`; `internal/compute/tensor` `TestLoadStateChecksEverythingBeforeWriting`). Yazma aşamasında yalnızca kurtarılamayan hatalar kalır (cihaz belleğinin bitmesi, dosyanın doğrulandıktan sonra değişmesi): parametreler yarıda kalabilir, okuyucular değişmez; bu hatadan sonra eğitim sürdürülmemeli, program yeniden başlatılıp nokta yeniden yüklenmelidir (tam atomiklik modelin ikinci bir kopyasını isterdi, bkz. Biçim ve güvenlik). Dosyaları sağlam ama içeriği kurallara uymayan nokta bozuk sayılır ve **20.9**'daki gibi bir önceki geçerli noktayı önerir: geçersiz iyileştirici durumu (tür ve parametre sayısı tuttuğu halde bilinmeyen bir durum anahtarı) ya da şekli bozuk adım sayacı, bozuk düz değer JSON'u, okuyucu durumunda eksik ya da geçersiz alan (hiçbir okuyucunun yazmadığı tür, pozitif olmayan `toplu` ya da `bağlam`) ya da bu veride geçersiz konum (örnekleyicide grubun pencere sayısını aşan pencere). Kullanıcının programındaki fark (başka okuyucu türü, veri kümesi, `toplu`, `bağlam`, `tohum`, model) bozukluk değil uyumsuzluktur; önceki nokta önerilmez (`TestCorruptContentSuggestsPreviousButMismatchDoesNot`). Baştan başlamak için `<koşu>/checkpoints` klasörü silinir ya da başka bir koşu klasörü verilir. Komut satırı ayarları kayıttakinden farklıysa (ör. `--adım` artırılarak eğitim uzatılıyorsa) yalnızca bir not yazılır.

**20.8** Her başarılı kayıttan sonra en yeni `tut` nokta (ve az önce yazılan) kalır, daha eskileri silinir.

**20.9** En yeni noktanın bir dosyası eksikse, boyu ya da SHA-256 özeti `digest.json`'la tutmuyorsa, tensör dosyasının sağlaması tutmuyorsa ya da biçimi bozuksa kurulum açık hata verir: hangi noktanın hangi dosyası, bir önceki geçerli nokta (varsa: `Bir önceki geçerli nokta: step_0000001000 (adım 1000); ondan sürdürmek için bozuk klasörü silin ...`) ve ne yapılacağı. Bozuk nokta kendiliğinden atlanmaz (kullanıcı neyi kaybettiğini bilerek seçer). Yarıda kalmış bir kaydın geçici klasörü sessizce silinir; yerine yenisi konamadan kesilmiş aynı adımlı bir nokta geri konur.

**20.10** Aralıklı kayıtta (`TR_mem_update`) yazma hatası — disk dolu, izin yok, ağ ya da bulut sürücüsünde yeniden adlandırma hatası — eğitimi durdurmaz: `TR_mem_update` `TR_kw_false` döner, standart hataya bir uyarı yazılır (`Uyarı: ... kaydedilemedi: ... (1/3; eğitim sürüyor, 15 dk sonra yeniden denenecek). Son geçerli nokta: adım 1500 (...)`), sonraki aralıkta yeniden denenir; art arda üçüncü başarısızlık açık bir hatadır. Başarılı bir kayıt sayacı sıfırlar; önceki geçerli nokta hiçbir durumda bozulmaz.

## Durumun parçaları

**20.11** `TR_fn_load_parameters(model, sözlük)` ad → tensör sözlüğünü (ör. `TR_fn_load_tensors` sonucu) modelin var olan parametrelerine, `TR_fn_parameters(model, TR_arg_named: TR_kw_true)` ile aynı adlarla yerinde yazar ve yazılan parametre sayısını (`TR_type_Int`) verir. Tensörler parametrenin cihazına taşınır, türü farklıysa dönüştürülür; parametrenin sürüm sayacı artar (eski çizgeden geri yayılım hata verir, 15.27). Modelde olup sözlükte olmayan, sözlükte olup modelde olmayan ve şekli farklı ad hatadır (hiçbir şey yazılmaz); türev izleyen parametresi olmayan model ve `TR_type_Dict` olmayan ikinci argüman hatadır. Hazır ağırlıklarla ince ayar için de kullanılır.

**20.12** `op.TR_mem_state` iyileştiricinin durumunu ad → tensör sözlüğü olarak (kopya) verir: AdamW'de `step`, `step.k`, `m.k`, `v.k`; momentumlu SGD'de `momentum.k`; 16 bitlik parametrelerde `master.k` (k parametrenin listedeki sırasıdır; adlar dilden bağımsızdır, PyTorch'un `step`, `exp_avg`, `momentum_buffer`'ı gibi). Öğrenme oranı gibi ayarlar duruma girmez. `op.TR_mem_load_state(sözlük)` durumu aynı tipte ve aynı parametre listesiyle kurulmuş bir iyileştiriciye yükler ve `TR_kw_true` verir; tensörler parametrenin cihazına taşınır. Bilinmeyen anahtar, şekli tutmayan girdi ve tensör olmayan değer hatadır (iyileştirici değişmez).

**20.13** `ö.TR_mem_state` okuyucunun kaldığı yeri düz değerli bir sözlük olarak verir; anahtarlar dilden bağımsızdır (17.3), Türkçe programda da `ö.TR_mem_state["counter"]` diye okunur: `kind` (`sampler`, `sequential`), `batch`, `context`, (örnekleyicide) `seed`, `dataset` (veri kümesinin kimliği), (örnekleyicide) `epoch`, `position`, `window`, `counter` (okunan toplu iş sayısı), (sıralıda) `done`. Konum, `.TR_mem_next()` ile alınan son toplu işten sonraki konumdur (önden yüklenip bekleyen toplu işler sayılmaz). `ö.TR_mem_load_state(sözlük)` aynı ayarla kurulmuş bir okuyucuyu o konuma getirir ve `TR_kw_true` verir: sonraki `.TR_mem_next()` kaydedilen okuyucunun vereceği toplu işi verir. Türü, `batch`, `context`, `seed` ya da veri kümesi farklı kayıt ve eksik alan hatadır (okuyucu değişmez).

**20.14** `kn.TR_mem_rollback()` en yeni geçerli noktayı aynı programda yerinde yükler ve noktanın adımını (`TR_type_Int`) verir. Yükleme kurulumdakiyle aynı yoldur (**20.6**, **20.7**, **20.9**): parametreler, iyileştirici durumları, okuyucular ve düz değerler noktadaki hallerine döner, günlüklere devam kaydı yazılır (geri dönülen adımdan sonraki kayıtlar geçersiz sayılır, 19). Standart hataya `Son kontrol noktasına dönüldü: adım N (...)` yazılır. `TR_mem_resume_step` değişmez (kurulumun adımıdır); eğitim döngüsü dönen adımdan sürer. Nokta yoksa açık hatadır; argüman almaz. Bozuk bir adımdan geri dönüp sürdürülen eğitim, hiç bozulmamış eğitimle bit düzeyinde aynı biter (Go testi: `internal/stdlib/checkpoint` `TestRollbackAfterNaNSameAsUninterrupted`). Bozuk adım kaydedilmeden önce dönülmelidir: aralıklı kayıt (`TR_mem_update`) bozulmuş durumu da kaydeder, bu yüzden denetim `TR_mem_update`'den önce yapılır:

```tr
//! tr kodlu 3 dil=tr
TR_kw_var w = TR_type_Tensor([1.0, 2.0], TR_arg_requires_grad: TR_kw_true)
TR_kw_var op = TR_fn_AdamW([w])
TR_kw_var ö = TR_type_Dataset("veri").TR_mem_sampler(TR_arg_batch_size: 8, TR_arg_context: 32, TR_arg_seed: 1)
TR_kw_var kn = TR_type_Checkpoint("koşu", {w: w, op: op, ö: ö})
TR_kw_var adım = kn.TR_mem_resume_step
TR_kw_for adım < 1000 {
    TR_kw_var giriş, hedef = ö.TR_mem_next()
    op.TR_mem_zero_grad()
    TR_kw_var kayıp = TR_fn_sum(w * w)
    TR_fn_backward(kayıp)
    op.TR_mem_step()
    TR_kw_if TR_kw_not TR_fn_is_finite(kayıp.TR_mem_tolist) {
        adım = kn.TR_mem_rollback()
        ö.TR_mem_skip(200)
        TR_kw_continue
    }
    adım += 1
    kn.TR_mem_update(adım)
}
```

**20.15** `ö.TR_mem_skip(n)` okuyucunun (örnekleyici ya da sıralı) sıradaki `n` toplu işini tensör kurmadan atlar ve atladığı sayıyı (`TR_type_Int`) verir: sonraki `.TR_mem_next()`, `n` tane `.TR_mem_next()` çağrısından sonra gelecek toplu işin aynısıdır ve durumun `counter`'ı `n` artar (**20.13**). Sıralı okuyucu biterse kalan kadar atlar, sonra `.TR_mem_next()` `TR_kw_none` verir. `n = 0` bir şey yapmaz; negatif sayı, `TR_type_Int` olmayan değer ve eksik ya da fazla argüman hatadır. Kayıp sıçramasından sonra son noktaya dönen eğitim, sıçramayı tetikleyen toplu işleri böyle atlar (PaLM bunu 200–500 toplu işle yapmıştır; Chowdhery ve ark. 2022, §5.1).

## Durdurma, belirlenimcilik ve sınırlar

**Durdurma isteği** (Go testleri: `cmd/tr/e2e`, `internal/stdlib/checkpoint`). Kontrol noktası kurulduktan sonra gelen ilk Ctrl+C (Unix'te ayrıca SIGTERM, SIGHUP; Windows'ta Ctrl+Break ve konsol penceresinin kapanması; bkz. 1.5) programı hemen durdurmaz: `Durduruluyor: adım bitince kontrol noktası kaydedilip çıkılacak (hemen çıkmak için Ctrl+C'ye yeniden basın)`. Sıradaki `kn.TR_mem_update` (ya da `kn.TR_mem_save()`) noktayı kaydeder (`Kontrol noktası kaydedildi: adım N (...)`) ve programı nazikçe durdurur: ertelenen işler çalışır, `dene/yakala` durdurmayı yakalamaz, süreç isteğin sinyaliyle biter (1.9; bugün 3 koduyla çıkılıyor); sürdürme bu kayıttan olur. Bu kayıt yazılamazsa uyarı verilir ve program yine durur; sürdürülünce son geçerli noktadan devam edilir. İkinci Ctrl+C süreci hemen SIGINT ile bitirir (1.7; bugün 130 koduyla; son düzenli nokta kalır). Kontrol noktası kurulmadan gelen istek programı 1.5'teki gibi durdurur. Sınır: Windows'ta konsol penceresi kapatılınca işletim sistemi süreci varsayılan olarak 5 saniye içinde öldürür (1.8), uzun bir adım bitmeyebilir; gerçek güvence düzenli kayıttır (`her_dakika`). Süreç öldürülürse (elektrik kesintisi, not defterinin çalışma makinesi geri alındığında) en son düzenli noktadan sürülür.

**Bit düzeyinde aynılık** (Go kabul testi: `cmd/tr/e2e` `TestCheckpointResumeBitExact`). Ayrı süreçte 40 adım kesintisiz eğitim ile Ctrl+C ve süreç öldürülerek kesilip sürdürülen eğitim (kesme ve öldürme bilinen bir adımın sınırında, kayıtlar adım sayısıyla yapılır; kayıt ortasında öldürmenin bıraktığı yarım geçici klasör ve kenara alınmış aynı adımlı nokta da sınanır; gömme, rms normu, nedensel dikkat, GELU katmanı, çapraz entropi, gradyan kırpma, AdamW, örnekleyici, günlük) bit düzeyinde aynı parametreleri, iyileştirici durumunu, geçerli kayıp dizisini ve düz değerleri verir. CPU kernellerinin indirgemeleri işçi sayısından bağımsızdır (sabit parça sırası): aynı eğitim `GOMAXPROCS=1` ve `=8` ile bit düzeyinde aynıdır ve 8 işçiyle başlayıp 1 işçiyle sürdürülebilir. Koşullar: bütün rastgelelik tohumlu olmalıdır (kontrol noktası kurulduktan sonra `TR_fn_randn` ya da `TR_fn_sample` tohumsuz kullanılırsa bir kez uyarılır: paylaşılan üretecin durumu kaydedilemez); kayıt ile devam aynı TR ikilisiyle ve aynı cihaz türünde yapılmalıdır (CPU ile GPU, AVX2'li ile AVX2'siz işlemci, ya da farklı işlemci düzeyiyle derlenmiş ikililer — `GOAMD64=v3` ve `v1` — son bitte farklı sonuç verebilir). GPU'da (`internal/stdlib/gpuprobe`) parametreler ve iyileştirici durumları kayıtta konağa indirilir, yüklemede aynı cihaza konur; kesintisiz GPU eğitimiyle bit düzeyinde aynıdır.

**Biçim ve güvenlik** (Go testleri ve bulanıklaştırma: `internal/stdlib/checkpoint`). Kontrol noktası dosyaları güvenilmeyen girdidir (başkasından gelen ya da yarım kopyalanmış koşu klasörü): `digest.json` en çok 64 KB, `state.json` en çok 64 MB okunur (`latest.json` yüklemede okunmaz); JSON katı çözülür (bilinmeyen alan, fazladan veri hata); biçim sürümü 3'tür ve yalnız o okunur (eski sürümler okunmaz: dil taslaktır, 17.6), 3'ten büyükse `daha yeni bir TR sürümüyle yazılmış` hatası verilir; adım klasör adı, `digest.json` ve `state.json` arasında tutmalıdır; tensör dosyası `TR_fn_load_tensors`'nin bütün sınırlarıyla okunur (15.46). Biçimde yalnızca veri vardır: kod çalıştıran, nesne kuran ya da yol çözen alan yoktur (`latest.json`'daki klasör adı yüklemede kullanılmaz). Büyük modeller bellekte ikinci bir kopya toplanmadan yazılır ve okunur: tensörler tek tek konağa indirilip yazılır, yüklemede tensör dosyası önce yalnızca doğrulanır, sonra girdi girdi yüklenir. Kayıt yorumlayıcı kilidi tutularak yapılır; eşzamanlı görevler kayıt boyunca bekler.

**Koşu klasörü düzeni (öneri).** Her eğitim kendi koşu klasörüne yazar; kontrol noktaları onun bir alt klasöründedir:

```text
runs/deneme1/
  config.json            ayarlar (komut_satırı sonucu: dosya_yaz(yol(koşu, "config.json"), json_metin(ayar)))
  metrics.jsonl          günlük(yol(koşu, "metrics.jsonl")): her adımın ölçüleri, devam kayıtları
  samples.json           üretilen örnek metinler
  results.json           son değerlendirme
  speed.json             hız ölçümleri
  model.trt              son ağırlıklar: tensör_kaydet(yol(koşu, "model.trt"), parametreler(model, adlarıyla: doğru))
  checkpoints/           kontrol_noktası(koşu, ...) — latest.json, step_0000001500/, step_0000001000/
```

**Ağ ve bulut sürücüleri.** Bağlanmış bir bulut sürücüsüne (ör. not defterinde Drive) doğrudan yazmak yavaştır ve yeniden adlandırma (atomik kaydın son adımı) başarısız olabilir ya da gecikmeli yansıyabilir. Öneri: koşu klasörünü çalışma makinesinin yerel diskinde tutun ve kalıcı sürücüye ara ara kopyalayın (`TR_fn_copy(koşu, "/content/drive/MyDrive/koşular", TR_arg_overwrite: TR_kw_true)`); oturum kapanınca kopyayı yerel diske geri alıp sürdürün. Doğrudan bulut sürücüsüne yazılıyorsa 20.10 geçerlidir: eğitim durmaz, üç kez art arda başarısız olursa açık hatayla durur.
