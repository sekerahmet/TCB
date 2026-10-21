# 16. Veri ve parçalayıcı

Model eğitimi için `tokenizer.json` parçalayıcıları ve `.npy` veri dosyaları yerleşik nesnelerle okunur. Ayrıntılı kullanım için `docs/guide/04_yapay_zeka_ve_veri.md` §4.D'ye bakın.

**16.1** `parçalayıcı_yükle(yol)` bir `Parçalayıcı` nesnesi döndürür. `.dağarcık_boyutu` dağarcıktaki parça sayısıdır (`Tam`); `.parça_kimliği(parça)` parçanın kimliğini, dağarcıkta yoksa `yok` verir.

**16.2** `.kodla(metin)` bir `Tam` listesi, `.kodla(metin_listesi)` her metin için bir liste (sıra korunur) döndürür. Metindeki özel parça yazımları tek kimlik olarak tanınır. Adlı seçenekler: `özel_parçalar:` (varsayılan `doğru`; `yanlış` son işleyicinin eklediği özel parçaları kapatır) ve `ikinci:` (çift kodlama; listeyle eşit uzunlukta liste).

**16.3** `.çöz(kimlikler)` kimlik listesinden ya da 1 boyutlu tensörden metin, listelerin listesinden ya da 2 boyutlu tensörden metin listesi üretir. Varsayılan olarak özel parçalar çıktıya yazılmaz; `özel_parçalar: doğru` onları da yazar. Dağarcıkta olmayan kimlikler atlanır.

**16.4** `veri_kümesi(dizin)` bir klasördeki `.npy` veri dosyalarını (ad sırasıyla) ya da tek bir `.npy` dosyasını `VeriKümesi` olarak açar; `meta.json` varsa parça sayıları ondan okunur. `önbellek: klasör` veri dosyalarını yerel bir önbelleğe kopyalar; bu durumda `kota_gb:` zorunludur. Aynı önbellek klasörünü kullanan kümeler tek kotayı paylaşır (en son verilen geçerlidir); klasörü aynı anda başka bir program kullanıyorsa hata verilir.

**16.5** `.örnekleyici(toplu:, bağlam:, tohum: 0)` bir `Örnekleyici` döndürür. `.sonraki()` `[girdi, hedef]` listesidir: ikisi de `[toplu, bağlam]` şeklinde tam sayılı işaretli tensördür (`.değer` `Tam` listesi verir; işlem sonuçlarına bu işaret geçmez), `hedef` girdinin bir kaydırılmışıdır ve pencereler veri dosyası sınırını aşmaz. Aynı küme ve tohum aynı toplu işleri verir; örnekleyici sonsuz üretir.

**16.6** `.sıralı(toplu:, bağlam:)` bir `SıralıOkuyucu` döndürür: pencereler veri dosyası sırasıyla ve `bağlam` adımla ardışıktır; son toplu iş daha az satırlı olabilir; veri bitince `.sonraki()` `yok` döner. Sıralı okuyucu `döngü` ile de gezilir (5.7): `döngü iş içinde okuyucu { değişken girdi, hedef = iş ... }` her turda `.sonraki()`'nin verdiği `[girdi, hedef]` listesini verir ve veri bitince biter; döngü okuyucuyu baştan almaz, kaldığı yerden tüketir. Örnekleyici sonsuz olduğundan döngüyle gezilmez (hata adım sayısıyla döngü kurmayı önerir).

**16.7** Hatalar (dosya yok, `.npy` değil, eksik ya da hatalı adlı argüman, bağlam veri dosyalarından uzun, düzenli ifade adım sınırı...) Türkçe bir mesajla bildirilir ve `dene/yakala` ile yakalanabilir.

**16.8** Bu yerleşiklerde göreli yollar `dosya_oku`/`dosya_yaz` ile aynı tek kuralla (bkz. **13.1**) çözülür: önce çalışma klasörü, orada yoksa programın bulunduğu klasör.

Önden yükleme her zaman açıktır ve yorumlayıcı kilidini tutmaz; `.sonraki()` beklerken kilit bırakılır (10.3). Açık bir kapatma adı yoktur: sıralı okuyucu bitince, kullanılmayan nesneler çöp toplanınca ve program bitince dosyalar kapatılır.
