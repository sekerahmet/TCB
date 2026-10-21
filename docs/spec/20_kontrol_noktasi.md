# 20. Kontrol noktası ve kaldığı yerden devam

Uzun bir eğitim kesilebilir: bulut not defterinin oturumu kapanır, Ctrl+C'ye basılır, makine yeniden başlar. `kontrol_noktası` eğitimin bütün durumunu koşu klasörüne atomik kaydeder; program aynı komutla yeniden çalıştırılınca kaldığı adımdan, **kesintisiz eğitimle bit düzeyinde aynı** sonuca gidecek biçimde sürer. Uygulama `internal/stdlib/checkpoint`'dadır; kodlar ve örnekler [`codes.md`](../codes.md)'de (A258, T702, T703, T984, T985, Y010, Y502, Y503, Y520, Y521). Kullanım için [`docs/guide/09_uzun_egitim.md`](../guide/09_uzun_egitim.md). Kural testleri `tests/kontrol_noktasi_test.tr`'dedir; süreç kesme, ayrı süreçlerle kabul testi, ağ sürücüsü benzetimi ve GPU yolu `tr test` ile sınanamadığından Go testleriyle sınanır (`internal/stdlib/checkpoint`, `cmd/tr/e2e`, `internal/stdlib/gpuprobe`).

```tr
değişken ayar = komut_satırı({adım: 20000, koşu: "runs/deneme1"})
değişken model = {E: tensör(türevsiz(rastgele_normal([50000, 256], tohum: 1) * 0.02), türev_izle: doğru)}
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.0003)
değişken ö = veri_kümesi("veri/train").örnekleyici(toplu: 16, bağlam: 512, tohum: 1)
değişken g = günlük(yol(ayar.koşu, "metrics.jsonl"))
değişken izlenen = {en_iyi_kayıp: 99.0}
değişken kn = kontrol_noktası(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen})
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde kn.kaldığı_adım..ayar.adım {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(model.E, giriş) @ model.E.devrik(), hedef)
    geri_yayılım(kayıp)
    op.adım()
    eğer kayıp.değer < izlenen.en_iyi_kayıp { izlenen.en_iyi_kayıp = kayıp.değer }
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1) // 15 dakikada bir ve Ctrl+C gelince adım sınırında kaydeder
}
kn.kaydet()
```

## Kurulum ve nesneler

**20.1** `kontrol_noktası(koşu, nesneler, her_dakika: 15, tut: 2)` bir `KontrolNoktası` kurar. `koşu` bir `Metin` ya da `Yol`'dur, 13.1'in kuralıyla çözülür; noktalar her zaman `<koşu>/kontrol_noktalari/` altına yazılır (koşu klasöründeki öbür dosyalara dokunulmaz). Klasörde nokta yoksa koşu yenidir, `kn.kaldığı_adım` 0'dır ve klasör ilk kayıtta oluşur. `her_dakika` kayıt aralığıdır (0 ile 1 000 000 arası dakika; 0: her `güncelle`'de), `tut` saklanacak en yeni nokta sayısıdır (1 ile 1000 arası `Tam`). Koşu yolu bir dosyaysa, argüman sayısı ya da tipi yanlışsa, adlı argüman bilinmiyorsa hata verilir.

**20.2** `nesneler` kaydedilecek şeylerin sözlüğüdür; anahtarları kullanıcının verdiği adlardır (veridir, kodlanmaz; `Metin`, boş değil, en çok 200 bayt, `/` ve denetim karakteri içermez), en çok 1000 nesne. Değerler:
- **model**: tensör içeren bir yapı nesnesi, liste ya da sözlük, ya da tek bir tensör. İçindeki **bütün yaprak tensörler** kaydedilir: türev izleyenler (**parametreler**: iyileştiricinin güncellediği, `parametreler(model)`'in verdiği tensörler) ve türev izlemeyenler (**durum**: TR ile yazılmış bir iyileştiricinin momentumu gibi türevsiz blokta yerinde güncellenen tensörler, sabit tablolar). Adlar `parametreler(model, adlarıyla: doğru)`'nun yol adlarıdır, aynı gezinti sırasıyla; aynı tensör bir kez kaydedilir. Yüklemede ikisi de var olan tensörlere yerinde yazılır (20.6); kayıttaki ile modeldeki tensörlerin adları, şekilleri ve türleri aynı olmalıdır (20.7). Ara sonuçlar (türev izleyen bir işlemin sonucu) kaydedilmez, tek başına verilen ara tensör hatadır; modelin tensör olmayan alanları (ayarlar, sayılar) kaydedilmez — değişen sayılar ayrı bir düz değerli sözlükte verilir;
- **iyileştirici** (`AdamW`, `SGD`): `op.durum` (20.12);
- **okuyucu** (`Örnekleyici`, `SıralıOkuyucu`): `ö.durum` (20.13);
- **günlük** (`Günlük`): dosyanın o anki bayt uzunluğu;
- **düz değerler**: yalnızca `Tam`, `Ondalık`, `Metin`, `Mantık`, `yok` ve iç içe `Liste`/`Sözlük` içeren bir `Sözlük` ya da `Liste` (en iyi kayıp, sayaçlar). Yüklemede (ve `geri_dön`'de) yerinde ve iç içe yenilenir: kullanıcının değerinde aynı anahtarda ya da sırada aynı türden bir `Liste` ya da `Sözlük` varsa o nesne de yerinde güncellenir, yenisiyle değiştirilmez; böylece iç içe kaplara başka değişkenlerden tutulan bağlar kopmaz (`izlenen = {kayıt: kayıt}` ise yüklemeden sonra `kayıt` da kayıttaki listeyi gösterir).

Düz bir `Tam`, `Ondalık`, `Metin` ya da `Mantık` doğrudan verilemez (geri yüklenirken kullanıcının değişkenine yazılamaz; bir sözlüğe konur: `{en_iyi: {değer: 99.0}}`). Fonksiyon, ara tensör ve boş sözlük hatadır; içinde tensör olan bir sözlük düz değer değil modeldir.

## Kayıt

**20.3** `kn.güncelle(adım)` eğitimin adım sınırıdır: `adım` tamamlanan adım sayısıdır (0 ya da pozitif `Tam`). Son kayıttan (ya da kurulumdan) beri `her_dakika` geçtiyse nokta bu adımla kaydedilir ve `doğru` döner; geçmediyse `yanlış`. Kayıt yalnızca adım sınırında yapılır (yarım bir adım hiç kaydedilmez); `güncelle` iyileştiricinin adımından sonra çağrılmalıdır. Gradyanlar kaydedilmez.

**20.4** `kn.kaydet()` noktayı hemen, son `güncelle` adımıyla (hiç çağrılmadıysa `kaldığı_adım` ile) kaydeder ve noktanın klasörünü `Yol` olarak verir: `<koşu>/kontrol_noktalari/adim_0000001500` (adım 10 basamağa sıfırla doldurulur). Aynı adımın noktası varsa yenisi yerine konur. `kaydet`'teki yazma hatası açık hatadır (20.10'dan farklı olarak eğitim sürmez).

**20.5** Bir nokta klasörü üç dosyadır: `tensorler.trt` (modellerin tensörleri — parametreler ve türev izlemeyen durum tensörleri — ve iyileştirici durumları, `tensör_kaydet` biçiminde; adları `<nesne>/<tensör adı>`), `durum.json` (biçim sürümü, adım, zaman, program, işçi sayısı, komut satırı ayarları — gizli değerler maskeli —, nesnelerin kaydı: okuyucu durumları, düz değerler, günlük uzunlukları) ve `ozet.json` (öbür iki dosyanın bayt sayısı ve SHA-256 özeti; en son yazılır). `kontrol_noktalari/son.json` en son kaydedilen noktayı söyler (`{"sürüm":2,"adım":1500,"klasör":"adim_0000001500"}`; araçlar içindir, yükleme klasörlere bakar). Nokta önce `adim_N.tmp-<süreç>` klasörüne yazılıp diske işlenir, sonra tek bir yeniden adlandırmayla yerine konur, `son.json` atomik güncellenir, sonra `tut`'u aşan eski noktalar silinir: hangi aşamada kesilirse kesilsin önceki geçerli nokta bozulmaz.

## Devam

**20.6** Aynı koşu klasörüyle yeniden kurulan kontrol noktası en yeni noktayı yükler ve standart hataya `Kaldığı yerden devam: adım N (...)` yazar: `kn.kaldığı_adım` N olur; modellerin tensörleri (parametreler ve durum tensörleri) **var olan** tensörlere yerinde yazılır (iyileştirici aynı tensörleri güncellemeyi sürdürür; cihazları değişmez); iyileştirici durumu (momentler, adım sayaçları, 16 bitlik parametrelerin ana kopyaları) parametrelerin cihazına yüklenir; okuyucu kaydedilen konumdan sürer (sonraki toplu iş kesintisiz eğitimdekiyle aynıdır); düz değerli sözlük ve listelerin içi yerinde yenilenir (değişken aynı değeri gösterir; `Ondalık` `Ondalık` kalır, `Tam` anahtarlar, NaN ve sonsuz korunur); günlüklere `{"tür":"devam","adım":N}` kaydı yazılır (19, günlük okuyucu kuralı). Kesintisiz eğitim ile N. adımda kesilip sürdürülen eğitim aynı son adımda bit düzeyinde aynı parametreleri, iyileştirici durumunu ve geçerli kayıp dizisini verir.

**20.7** Yüklemeden önce bütün nesnelerin kayda uyduğu denetlenir; uymazsa yol gösterici bir hata verilir ve **hiçbir nesne değişmez**: kayıttaki nesnelerin adları ve türleri, modelin tensörlerinin (parametreler ve durum tensörleri) adları, şekilleri ve veri türleri, iyileştiricinin tipi ve parametre sayısı (ve durum anahtarlarının şekilleri), okuyucunun türü, `toplu`, `bağlam`, `tohum` değerleri ve veri kümesi (veri dosyalarının adları ve parça sayıları) aynı olmalıdır. Yükleme iki aşamalıdır: önce iyileştirici durumları bütün kurallarıyla (adım sayaçları, moment çiftleri, momentli parametrenin sayacı, tensörlerin cihazı ve canlılığı) denetlenir, modelin tensörlerinin yazılabilir olduğu sınanır ve okuyucular yeni konumlarına hazırlanır; hiçbir nesne değişmez. Hepsi geçerse parametreler, okuyucular, iyileştiriciler ve düz değerler bu sırayla yazılır (Go testleri: `internal/stdlib/checkpoint` `TestCorruptOptimizerStateChangesNothingAndSuggestsPrevious`, `TestRollbackStopsBeforeWritingWhenAParamIsUnusable`; `internal/compute/tensor` `TestLoadStateChecksEverythingBeforeWriting`). Yazma aşamasında yalnızca kurtarılamayan hatalar kalır (cihaz belleğinin bitmesi, dosyanın doğrulandıktan sonra değişmesi): parametreler yarıda kalabilir, okuyucular değişmez; bu hatadan sonra eğitim sürdürülmemeli, program yeniden başlatılıp nokta yeniden yüklenmelidir (tam atomiklik modelin ikinci bir kopyasını isterdi, bkz. Biçim ve güvenlik). Dosyaları sağlam ama içeriği kurallara uymayan nokta bozuk sayılır ve **20.9**'daki gibi bir önceki geçerli noktayı önerir: geçersiz iyileştirici durumu ya da şekli bozuk adım sayacı, bozuk düz değer JSON'u, okuyucu durumunda eksik ya da geçersiz alan ya da bu veride geçersiz konum. Kullanıcının programındaki fark (başka okuyucu türü, veri kümesi, `toplu`, `bağlam`, `tohum`, model) bozukluk değil uyumsuzluktur; önceki nokta önerilmez (`TestCorruptContentSuggestsPreviousButMismatchDoesNot`). Baştan başlamak için `<koşu>/kontrol_noktalari` klasörü silinir ya da başka bir koşu klasörü verilir. Komut satırı ayarları kayıttakinden farklıysa (ör. `--adım` artırılarak eğitim uzatılıyorsa) yalnızca bir not yazılır.

**20.8** Her başarılı kayıttan sonra en yeni `tut` nokta (ve az önce yazılan) kalır, daha eskileri silinir.

**20.9** En yeni noktanın bir dosyası eksikse, boyu ya da SHA-256 özeti `ozet.json`'la tutmuyorsa, tensör dosyasının sağlaması tutmuyorsa ya da biçimi bozuksa kurulum açık hata verir: hangi noktanın hangi dosyası, bir önceki geçerli nokta (varsa: `Bir önceki geçerli nokta: adim_0000001000 (adım 1000); ondan sürdürmek için bozuk klasörü silin ...`) ve ne yapılacağı. Bozuk nokta kendiliğinden atlanmaz (kullanıcı neyi kaybettiğini bilerek seçer). Yarıda kalmış bir kaydın geçici klasörü sessizce silinir; yerine yenisi konamadan kesilmiş aynı adımlı bir nokta geri konur.

**20.10** Aralıklı kayıtta (`güncelle`) yazma hatası — disk dolu, izin yok, ağ ya da bulut sürücüsünde yeniden adlandırma hatası — eğitimi durdurmaz: `güncelle` `yanlış` döner, standart hataya bir uyarı yazılır (`Uyarı: ... kaydedilemedi: ... (1/3; eğitim sürüyor, 15 dk sonra yeniden denenecek). Son geçerli nokta: adım 1500 (...)`), sonraki aralıkta yeniden denenir; art arda üçüncü başarısızlık açık bir hatadır. Başarılı bir kayıt sayacı sıfırlar; önceki geçerli nokta hiçbir durumda bozulmaz.

## Durumun parçaları

**20.11** `parametreleri_yükle(model, sözlük)` ad → tensör sözlüğünü (ör. `tensör_yükle` sonucu) modelin var olan parametrelerine, `parametreler(model, adlarıyla: doğru)` ile aynı adlarla yerinde yazar ve yazılan parametre sayısını (`Tam`) verir. Tensörler parametrenin cihazına taşınır, türü farklıysa dönüştürülür; parametrenin sürüm sayacı artar (eski çizgeden geri yayılım hata verir, 15.6). Modelde olup sözlükte olmayan, sözlükte olup modelde olmayan ve şekli farklı ad hatadır (hiçbir şey yazılmaz); türev izleyen parametresi olmayan model ve `Sözlük` olmayan ikinci argüman hatadır. Hazır ağırlıklarla ince ayar için de kullanılır.

**20.12** `op.durum` iyileştiricinin durumunu ad → tensör sözlüğü olarak (kopya) verir: AdamW'de `adim`, `adim.k`, `m.k`, `v.k`; momentumlu SGD'de `hiz.k`; 16 bitlik parametrelerde `ana.k` (k parametrenin listedeki sırasıdır). Öğrenme oranı gibi ayarlar duruma girmez. `op.durumu_yükle(sözlük)` durumu aynı tipte ve aynı parametre listesiyle kurulmuş bir iyileştiriciye yükler ve `doğru` verir; tensörler parametrenin cihazına taşınır. Bilinmeyen anahtar, şekli tutmayan girdi ve tensör olmayan değer hatadır (iyileştirici değişmez).

**20.13** `ö.durum` okuyucunun kaldığı yeri düz değerli bir sözlük olarak verir: `tür` (`örnekleyici`, `sıralı`), `toplu`, `bağlam`, (örnekleyicide) `tohum`, `küme` (veri kümesinin kimliği), (örnekleyicide) `dönem`, `sıra`, `pencere`, `sayaç` (okunan toplu iş sayısı), (sıralıda) `bitti`. Konum, `.sonraki()` ile alınan son toplu işten sonraki konumdur (önden yüklenip bekleyen toplu işler sayılmaz). `ö.durumu_yükle(sözlük)` aynı ayarla kurulmuş bir okuyucuyu o konuma getirir ve `doğru` verir: sonraki `.sonraki()` kaydedilen okuyucunun vereceği toplu işi verir. Türü, `toplu`, `bağlam`, `tohum` ya da veri kümesi farklı kayıt ve eksik alan hatadır (okuyucu değişmez).

**20.14** `kn.geri_dön()` en yeni geçerli noktayı aynı programda yerinde yükler ve noktanın adımını (`Tam`) verir. Yükleme kurulumdakiyle aynı yoldur (**20.6**, **20.7**, **20.9**): parametreler, iyileştirici durumları, okuyucular ve düz değerler noktadaki hallerine döner, günlüklere devam kaydı yazılır (geri dönülen adımdan sonraki kayıtlar geçersiz sayılır, 19). Standart hataya `Son kontrol noktasına dönüldü: adım N (...)` yazılır. `kaldığı_adım` değişmez (kurulumun adımıdır); eğitim döngüsü dönen adımdan sürer. Nokta yoksa açık hatadır; argüman almaz. Bozuk bir adımdan geri dönüp sürdürülen eğitim, hiç bozulmamış eğitimle bit düzeyinde aynı biter (Go testi: `internal/stdlib/checkpoint` `TestRollbackAfterNaNSameAsUninterrupted`). Bozuk adım kaydedilmeden önce dönülmelidir: aralıklı kayıt (`güncelle`) bozulmuş durumu da kaydeder, bu yüzden denetim `güncelle`'den önce yapılır:

```tr
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = AdamW([w])
değişken ö = veri_kümesi("veri").örnekleyici(toplu: 8, bağlam: 32, tohum: 1)
değişken kn = kontrol_noktası("koşu", {w: w, op: op, ö: ö})
değişken adım = kn.kaldığı_adım
döngü adım < 1000 {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = toplam(w * w)
    geri_yayılım(kayıp)
    op.adım()
    eğer değil sonlu_mu(kayıp.değer) {
        adım = kn.geri_dön()
        ö.atla(200)
        devam
    }
    adım += 1
    kn.güncelle(adım)
}
```

**20.15** `ö.atla(n)` okuyucunun (örnekleyici ya da sıralı) sıradaki `n` toplu işini tensör kurmadan atlar ve atladığı sayıyı (`Tam`) verir: sonraki `.sonraki()`, `n` tane `.sonraki()` çağrısından sonra gelecek toplu işin aynısıdır ve durumun `sayaç`ı `n` artar (**20.13**). Sıralı okuyucu biterse kalan kadar atlar, sonra `.sonraki()` `yok` verir. `n = 0` bir şey yapmaz; negatif sayı, `Tam` olmayan değer ve eksik ya da fazla argüman hatadır. Kayıp sıçramasından sonra son noktaya dönen eğitim, sıçramayı tetikleyen toplu işleri böyle atlar (PaLM bunu 200–500 toplu işle yapmıştır; Chowdhery ve ark. 2022, §5.1).

## Durdurma, belirlenimcilik ve sınırlar

**Durdurma isteği** (Go testleri: `cmd/tr/e2e`, `internal/stdlib/checkpoint`). Kontrol noktası kurulduktan sonra gelen ilk Ctrl+C (Unix'te ayrıca SIGTERM, SIGHUP; Windows'ta Ctrl+Break ve konsol kapatma olayları) programı hemen durdurmaz: `Durduruluyor: adım bitince kontrol noktası kaydedilip çıkılacak (hemen çıkmak için Ctrl+C'ye yeniden basın)`. Sıradaki `kn.güncelle` (ya da `kn.kaydet()`) noktayı kaydeder (`Kontrol noktası kaydedildi: adım N (...)`) ve programı nazikçe durdurur: ertelenen işler çalışır, `dene/yakala` durdurmayı yakalamaz, çıkış kodu **3**'tür ("durduruldu, sürdürülebilir", 8.3). Bu kayıt yazılamazsa uyarı verilir ve program yine durur; sürdürülünce son geçerli noktadan devam edilir. İkinci Ctrl+C süreci hemen bitirir (çıkış kodu 130; son düzenli nokta kalır). Kontrol noktası kurulmadan gelen istek bugünkü gibi programı hemen durdurur. Sınır: Windows'ta konsol penceresi kapatılınca işletim sistemi süreci birkaç saniye içinde öldürür, uzun bir adım bitmeyebilir; gerçek güvence düzenli kayıttır (`her_dakika`). Süreç öldürülürse (elektrik kesintisi, not defterinin çalışma makinesi geri alındığında) en son düzenli noktadan sürülür.

**Bit düzeyinde aynılık** (Go kabul testi: `cmd/tr/e2e` `TestKontrolNoktasiBitDuzeyindeDevam`). Ayrı süreçte 40 adım kesintisiz eğitim ile Ctrl+C ve süreç öldürülerek kesilip sürdürülen eğitim (kesme ve öldürme bilinen bir adımın sınırında, kayıtlar adım sayısıyla yapılır; kayıt ortasında öldürmenin bıraktığı yarım geçici klasör ve kenara alınmış aynı adımlı nokta da sınanır; gömme, rms normu, nedensel dikkat, GELU katmanı, çapraz entropi, gradyan kırpma, AdamW, örnekleyici, günlük) bit düzeyinde aynı parametreleri, iyileştirici durumunu, geçerli kayıp dizisini ve düz değerleri verir. CPU kernellerinin indirgemeleri işçi sayısından bağımsızdır (sabit parça sırası): aynı eğitim `GOMAXPROCS=1` ve `=8` ile bit düzeyinde aynıdır ve 8 işçiyle başlayıp 1 işçiyle sürdürülebilir. Koşullar: bütün rastgelelik tohumlu olmalıdır (kontrol noktası kurulduktan sonra `rastgele_normal` ya da `örnekle` tohumsuz kullanılırsa bir kez uyarılır: paylaşılan üretecin durumu kaydedilemez); kayıt ile devam aynı TR ikilisiyle ve aynı cihaz türünde yapılmalıdır (CPU ile GPU, AVX2'li ile AVX2'siz işlemci, ya da farklı işlemci düzeyiyle derlenmiş ikililer — `GOAMD64=v3` ve `v1` — son bitte farklı sonuç verebilir). GPU'da (`internal/stdlib/gpuprobe`) parametreler ve iyileştirici durumları kayıtta konağa indirilir, yüklemede aynı cihaza konur; kesintisiz GPU eğitimiyle bit düzeyinde aynıdır.

**Biçim ve güvenlik** (Go testleri ve bulanıklaştırma: `internal/stdlib/checkpoint`). Kontrol noktası dosyaları güvenilmeyen girdidir (başkasından gelen ya da yarım kopyalanmış koşu klasörü): `ozet.json` en çok 64 KB, `durum.json` en çok 64 MB okunur (`son.json` yüklemede okunmaz); JSON katı çözülür (bilinmeyen alan, fazladan veri hata); biçim sürümü 2'dir (sürüm 1 noktaları da okunur: o sürümde modellerin yalnızca türev izleyen tensörleri kaydedilirdi; böyle bir noktadan sürdürülürken kayıtta olmayan türev izlemeyen tensörler olduğu gibi kalır, kayıttaki tensörler modelde türev izlesin izlemesin yerlerine yazılır), 2'den büyükse `daha yeni bir TR sürümüyle yazılmış` hatası verilir; `ozet.json` ile `durum.json`'un sürümü aynı olmalıdır; adım klasör adı, `ozet.json` ve `durum.json` arasında tutmalıdır; tensör dosyası `tensör_yükle`'nin bütün sınırlarıyla okunur (15.10). Biçimde yalnızca veri vardır: kod çalıştıran, nesne kuran ya da yol çözen alan yoktur (`son.json`'daki klasör adı yüklemede kullanılmaz). Büyük modeller bellekte ikinci bir kopya toplanmadan yazılır ve okunur: tensörler tek tek konağa indirilip yazılır, yüklemede tensör dosyası önce yalnızca doğrulanır, sonra girdi girdi yüklenir. Kayıt yorumlayıcı kilidi tutularak yapılır; eşzamanlı görevler kayıt boyunca bekler.

**Koşu klasörü düzeni (öneri).** Her eğitim kendi koşu klasörüne yazar; kontrol noktaları onun bir alt klasöründedir:

```text
runs/deneme1/
  config.json            ayarlar (komut_satırı sonucu: dosya_yaz(yol(koşu, "config.json"), json_metin(ayar)))
  metrics.jsonl          günlük(yol(koşu, "metrics.jsonl")): her adımın ölçüleri, devam kayıtları
  samples.json           üretilen örnek metinler
  results.json           son değerlendirme
  speed.json             hız ölçümleri
  model.trt              son ağırlıklar: tensör_kaydet(yol(koşu, "model.trt"), parametreler(model, adlarıyla: doğru))
  kontrol_noktalari/     kontrol_noktası(koşu, ...) — son.json, adim_0000001500/, adim_0000001000/
```

**Ağ ve bulut sürücüleri.** Bağlanmış bir bulut sürücüsüne (ör. not defterinde Drive) doğrudan yazmak yavaştır ve yeniden adlandırma (atomik kaydın son adımı) başarısız olabilir ya da gecikmeli yansıyabilir. Öneri: koşu klasörünü çalışma makinesinin yerel diskinde tutun ve kalıcı sürücüye ara ara kopyalayın (`kopyala(koşu, "/content/drive/MyDrive/koşular", üzerine_yaz: doğru)`); oturum kapanınca kopyayı yerel diske geri alıp sürdürün. Doğrudan bulut sürücüsüne yazılıyorsa 20.10 geçerlidir: eğitim durmaz, üç kez art arda başarısız olursa açık hatayla durur.
