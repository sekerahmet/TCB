# 16. Veri ve parçalayıcı

Model eğitimi için `tokenizer.json` parçalayıcıları ve `.npy` veri dosyaları yerleşik nesnelerle okunur. Ayrıntılı kullanım için `docs/guide/04_yapay_zeka_ve_veri.md` §4.D'ye bakın.

**16.1** `TR_fn_load_tokenizer(yol)` bir `TR_type_Tokenizer` nesnesi döndürür. `.TR_mem_vocab_size` dağarcıktaki parça sayısıdır (`TR_type_Int`); `.TR_mem_token_id(parça)` parçanın kimliğini, dağarcıkta yoksa `TR_kw_none` verir.

**16.2** `.TR_mem_encode(metin)` bir `TR_type_Int` listesi, `.TR_mem_encode(metin_listesi)` her metin için bir liste (sıra korunur) döndürür. Metindeki özel parça yazımları tek kimlik olarak tanınır. Adlı seçenekler: `TR_arg_special_tokens:` (varsayılan `TR_kw_true`; `TR_kw_false` son işleyicinin eklediği özel parçaları kapatır) ve `TR_arg_pair:` (çift kodlama; listeyle eşit uzunlukta liste).

**16.3** `.TR_mem_decode(kimlikler)` kimlik listesinden ya da 1 boyutlu tensörden metin, listelerin listesinden ya da 2 boyutlu tensörden metin listesi üretir. Varsayılan olarak özel parçalar çıktıya yazılmaz; `TR_arg_special_tokens: TR_kw_true` onları da yazar. Dağarcıkta olmayan kimlikler atlanır.

**16.4** `TR_type_Dataset(dizin)` bir klasördeki `.npy` veri dosyalarını (ad sırasıyla) ya da tek bir `.npy` dosyasını `TR_type_Dataset` olarak açar; `meta.json` varsa parça sayıları ondan okunur. `TR_arg_cache: klasör` veri dosyalarını yerel bir önbelleğe kopyalar; bu durumda `TR_arg_quota_gb:` zorunludur. Aynı önbellek klasörünü kullanan kümeler tek kotayı paylaşır (en son verilen geçerlidir); klasörü aynı anda başka bir program kullanıyorsa hata verilir.

**16.5** `.örnekleyici(toplu:, bağlam:, tohum: 0)` bir `TR_type_Sampler` döndürür. `.TR_mem_next()` `[girdi, hedef]` listesidir: ikisi de `[toplu, bağlam]` şeklinde tam sayılı işaretli tensördür (`.TR_mem_tolist` `TR_type_Int` listesi verir; işlem sonuçlarına bu işaret geçmez), `hedef` girdinin bir kaydırılmışıdır ve pencereler veri dosyası sınırını aşmaz. Aynı küme ve tohum aynı toplu işleri verir; örnekleyici sonsuz üretir.

**16.6** `.sıralı(toplu:, bağlam:)` bir `TR_type_SequentialReader` döndürür: pencereler veri dosyası sırasıyla ve `bağlam` adımla ardışıktır; son toplu iş daha az satırlı olabilir; veri bitince `.TR_mem_next()` `TR_kw_none` döner. Sıralı okuyucu `TR_kw_for` ile de gezilir (5.7): `TR_kw_for iş TR_kw_in okuyucu { TR_kw_var girdi, hedef = iş ... }` her turda `.TR_mem_next()`'nin verdiği `[girdi, hedef]` listesini verir ve veri bitince biter; döngü okuyucuyu baştan almaz, kaldığı yerden tüketir. Örnekleyici sonsuz olduğundan döngüyle gezilmez (hata adım sayısıyla döngü kurmayı önerir).

**16.7** Hatalar (dosya yok, `.npy` değil, eksik ya da hatalı adlı argüman, bağlam veri dosyalarından uzun, düzenli ifade adım sınırı...) kişinin dilinde bir iletiyle bildirilir ve `dene/yakala` ile yakalanabilir.

**16.8** Bu yerleşiklerde göreli yollar `TR_fn_read_file`/`TR_fn_write_file` ile aynı tek kuralla (bkz. **13.1**) çözülür: önce çalışma klasörü, sonra kodun modülünün klasörü (11.5), sonra programın bulunduğu klasör.

Önden yükleme her zaman açıktır ve yorumlayıcı kilidini tutmaz; `.TR_mem_next()` beklerken kilit bırakılır (10.3). Açık bir kapatma adı yoktur: sıralı okuyucu bitince, kullanılmayan nesneler çöp toplanınca ve program bitince dosyalar kapatılır.
