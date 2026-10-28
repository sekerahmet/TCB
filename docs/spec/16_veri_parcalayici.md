# 16. Veri ve parçalayıcı

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "16. Veri ve parçalayıcı".

Her kuralın emsali ve gerekçesi: [`rationale/16_data.md`](rationale/16_data.md).

Metin modelinin verisi iki adımdan geçer: metin bir parçalayıcıyla (tokenizer) kimliklere çevrilir, kimlikler eğitim için parçalara (`.npy`) yazılır ve oradan pencereler hâlinde okunur. Bu bölüm parçalayıcıyı yüklemeyi, eğitmeyi ve kullanmayı, veriyi yazmayı ve okumayı söyler. Kullanım rehberi: `docs/guide/04_yapay_zeka_ve_veri.md` §4.D.

## Parçalayıcı

**16.1** `TR_load_tokenizer(yol)` yaygın `tokenizer.json` biçimindeki bir parçalayıcıyı yükler ve `TR_Tokenizer` verir.
- `.TR_vocab_size` dağarcıktaki parça sayısıdır (`TR_Int`).
- `.TR_token_id(parça)` parçanın kimliğini verir, dağarcıkta yoksa `TR_none` (`TR_Int?`).
- `tokenizer.json` dışarıdan gelen bir girdidir: ön ayırıcının düzenli ifadesi bir adım sınırıyla çalışır, sınırı aşan metin çalışma hatası verir.
- *Test:* TR testi.

**16.2** `p.TR_encode(metin)` metni kimliklere çevirir ve `TR_List[TR_Int]` verir. `p.TR_encode(metinler)` (`TR_List[TR_String]`) her metni ayrı çevirir, sırayı korur ve `TR_List[TR_List[TR_Int]]` verir (13.10: iki imza).
- Metnin içinde yazılan özel parça (`<|endoftext|>`) tek bir kimlik olur.
- `TR_special_tokens: TR_false` parçalayıcının kendi eklediği özel parçaları (başa ve sona) kapatır; varsayılan `TR_true`.
- `TR_pair:` ikinci metni (ya da metin listesini) çift olarak kodlar; listeyle verilirse uzunlukları aynı olmalıdır.
- *Test:* TR testi.

**16.3** `p.TR_decode(kimlikler)` tek bir diziyi metne çevirir: `TR_List[TR_Int]` ya da 1 boyutlu `TR_Tensor[TR_Int]`. Birden çok dizi `p.TR_batch_decode(diziler)` ile `TR_List[TR_String]` verir (13.8a).
- Özel parçalar varsayılan olarak yazılmaz; `TR_special_tokens: TR_true` onları da yazar.
- **Dağarcıkta olmayan kimlik çalışma hatasıdır;** ileti kimliği ve dağarcık boyunu söyler. Modelin çıktı boyu dağarcıktan büyükse (dolgu) ajan örneklemeyi dağarcığa sınırlar.
- *Test:* TR testi; dağarcık dışı kimliğin hatası uygulanınca TR testi.

**16.4** Kendi parçalayıcısını eğitmek: `TR_train_tokenizer(metinler, TR_vocab_size: n, TR_reserved_tokens: [...])` metinlerden bayt düzeyinde bir BPE parçalayıcısı eğitir ve `TR_Tokenizer` verir.
- `metinler` bir `TR_List[TR_String]`'dir. Metin veren bir akış (5.7) 13.10'a "dolaşılabilir" kısıtı eklenince alınır.
- `TR_reserved_tokens:` dağarcığın başına konan özel parçalardır (`["<|endoftext|>"]`); sırayla 0'dan kimlik alırlar.
- **Belirlenimci:** aynı metinler ve aynı argümanlar her makinede aynı parçalayıcıyı verir; birleştirmelerin eşitliği sabit bir kuralla bozulur.
- `p.TR_save(yol)` parçalayıcıyı `tokenizer.json` biçiminde yazar; `TR_load_tokenizer` ve başka araçlar onu okur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

## Veri

**16.5** `TR_Dataset(yol)` kimlik parçalarını açar: bir klasördeki `.npy` dosyalarını ad sırasıyla ya da tek bir `.npy` dosyasını.
- Parçaların öğeleri tam sayıdır: işaretsiz 16 ya da 32 bit, işaretli 32 ya da 64 bit (NumPy'nin varsayılanı). Başka türdeki dosya hatadır; ileti dosyanın türünü söyler.
- `meta.json` varsa parça uzunlukları ondan okunur.
- `TR_cache: klasör` parçaları yerel bir önbelleğe kopyalar; o zaman `TR_quota_gb:` zorunludur. Aynı önbellek klasörünü kullanan kümeler tek kotayı paylaşır; klasörü aynı anda başka bir program kullanıyorsa hata.
- Dosyalar açıkça kapatılmaz: sıralı okuyucu bitince, kullanılmayan nesneler toplanınca ve program bitince kapanır.
- *Test:* TR testi.

**16.6** Veriyi eğitime hazırlamak: `TR_write_dataset(yol, diziler, TR_shard_tokens: n)` kimlik dizilerini `yol` klasörüne `.npy` parçaları olarak yazar.
- `diziler` bir `TR_List[TR_List[TR_Int]]`'dir; diziler sırayla uç uca eklenir. Akış 16.4'teki gibi sonra.
- Her parça en çok `TR_shard_tokens` kimlik tutar; parçalar ad sırasıyla okunacak biçimde adlandırılır ve `meta.json` yazılır.
- Öğe türü kimliklerin en büyüğüne sığan en dar işaretsiz türdür: 65 536'dan küçükse 16 bit, değilse 32 bit. Eksi ya da 2³²'ye sığmayan kimlik hatadır.
- Yazma yolu 13'ün yazma kuralına uyar (çalışma klasörü).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**16.7** `küme.TR_sampler(TR_batch_size: b, TR_context: c, TR_seed: s)` rastgele pencereler veren bir `TR_Sampler` kurar.
- `.TR_next()` bir demet verir (3.6a): `TR_var girdi, hedef = ö.TR_next()`. İkisi de `[b, c]` şekilli `TR_Tensor[TR_Int]` "int64"tür; `hedef`, `girdi`nin bir kaydırılmışıdır. Pencere bir parçanın sınırını aşmaz.
- **Belirlenimci:** aynı küme, aynı argümanlar ve aynı tohum aynı toplu işleri verir. Örnekleyici sonsuzdur.
- **Kaldığı yerden:** örnekleyici kontrol noktasına kaydedilir (20); geri yüklenince kaldığı yerden aynı toplu işleri verir.
- Sonsuz olduğu için döngüyle gezilmez: `.TR_next()`'i isteğe bağlı değer vermez, 5.7'ye göre dolaşılamaz; `TR_for x TR_in ö` denetim hatasıdır, ileti adım sayısıyla döngüyü önerir.
- Sonraki toplu işler arka planda hazırlanır; `.TR_next()` beklerken öbür görevler çalışır (10.3).
- *Test:* TR testi; demet dönüşü ve tam sayı tensörü uygulanınca TR testi.

**16.8** `küme.TR_sequential(TR_batch_size: b, TR_context: c)` pencereleri parça sırasıyla ve `c` adımla ardışık veren bir `TR_SequentialReader` kurar; ölçmek ve değerlendirmek içindir.
- `.TR_next()` 16.7'deki gibi bir demet verir; son toplu iş daha az satırlı olabilir; veri bitince `TR_none`.
- Döngüyle gezilir (5.7): `TR_for iş TR_in okuyucu { TR_var girdi, hedef = iş ... }` (5.3'ün açması). Döngünün iki adlı biçimi listede indeks ve öğedir, burada kullanılmaz. Döngü baştan almaz, kaldığı yerden tüketir.
- *Test:* TR testi.

**16.9** Bu bölümün hataları (dosya yok, `.npy` değil, bağlam veri parçasından uzun, dağarcık dışı kimlik ...) çalışma hatasıdır, ileti koduyla verilir ve `TR_try`/`TR_catch` ile yakalanır (8).
- *Test:* TR testi.

**16.10** Göreli yollar 13'ün yol kuralıyla çözülür: okumada çalışma klasörü, kodun modülünün klasörü, sonra programın klasörü; yazmada yalnız çalışma klasörü.
- *Test:* TR testi.
