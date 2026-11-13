# 16. Veri ve parçalayıcı

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "16. Veri ve parçalayıcı".

Her kuralın emsali ve gerekçesi: [`rationale/16_data.md`](rationale/16_data.md).

Metin modelinin verisi iki adımdan geçer: metin bir parçalayıcıyla (tokenizer) kimliklere çevrilir, kimlikler eğitim için parçalara (`.npy`) yazılır ve oradan pencereler hâlinde okunur. Bu bölüm parçalayıcıyı yüklemeyi, eğitmeyi ve kullanmayı, veriyi yazmayı ve okumayı söyler. Kullanım rehberi: `docs/guide/04_yapay_zeka_ve_veri.md` §4.D.

## Parçalayıcı

**16.1** `TR_load_tokenizer(yol)` yaygın `tokenizer.json` biçimindeki bir parçalayıcıyı yükler ve `TR_Tokenizer` verir.
- `.TR_vocab_size` kimliklerin sınırıdır: en büyük kimlik + 1 (`TR_Int`). Kimlikler 0'dan aralıksız ve her kimlikte tek parça varsa parça sayısıdır; aradaki boş kimlik `.TR_decode`'da hatadır.
- `.TR_token_id(parça)` parçanın kimliğini verir, dağarcıkta yoksa `TR_none` (`TR_Int?`).
- `.TR_pretokenizer_tag` GGUF'tan yüklenen parçalayıcıda dosyanın ön bölme etiketidir (`tokenizer.ggml.pre`, ör. `"llama-bpe"`; `TR_String?`); GGUF'ta her zaman vardır (etiketsiz dosya yüklenmez, aşağıda), `tokenizer.json`'dan yüklenen parçalayıcıda `TR_none`'dır. Kitaplık sohbet tanımını mimari ile bu etiketten seçer (15.55).
- `.TR_declared_eos_tokens` GGUF'tan yüklenen parçalayıcıda dosyanın bildirdiği durma kimlikleridir: `tokenizer.ggml.eos_token_id`, `.eot_token_id` ve `.eom_token_id`'den dosyada olanlar, bu sırayla ve tekrarsız (`TR_List[TR_Int]?`). Hiçbiri yoksa ve `tokenizer.json`'dan yüklenen parçalayıcıda `TR_none`'dır; boş liste dönmez. `eot` ve `eom` dağarcıkla denetlenmez: dağarcık dışı kimlik hata değildir, hiç örneklenmez. `eot` ya da `eom` kimlik değilse (sayı değil) parçalayıcı yine yüklenir, `.TR_declared_eos_tokens` H6681 verir; `eos` özel kimliktir, yanlış türdeyse parçalayıcı yüklenmez (H6681). Modelin durma listesi ayrıdır: `model.TR_eos_tokens()` ailenin listesidir (15.55).
- Eklenmiş parçanın kimliği dosyadaki kimliktir, değiştirilmez: içerik sözlükte varsa kimlik onunla aynıdır; daha önce eklenmişse kimliği ve bütün bayrakları (`special`, `lstrip`, `rstrip`, `single_word`, `normalized`) aynı olmalıdır (birebir aynı tekrar kayıt geçerlidir); değilse 0 ile `1<<30` arasında ve boştur. `normalized` parçaların, dosyanın kendi normalleştiricisiyle normalleşmiş metinleri birbirinden ve öbür eklenmiş parçalardan ayrı olmalıdır. Tutmayan dosya yükleme hatası verir. Kimlikler dosyadaki sıraya bağlı değildir.
- `tokenizer.json` dışarıdan gelen bir girdidir: ön ayırıcının düzenli ifadesi bir adım sınırıyla çalışır, sınırı aşan metin çalışma hatası verir.
- `TR_load_tokenizer(klasör)` bir model klasörünü de açar: `tokenizer.json`'u okur, aileyi klasörün `config.json`'u ve `tokenizer_config.json`'uyla seçer (15.55); ailesi bulunmayan klasör ailesiz açılır. Klasörde `tokenizer.json` yoksa yükleme hatasıdır (H5610).
- `TR_load_tokenizer` GGUF dosyasını da içerikten tanır (15.55) ve parçalayıcıyı `tokenizer.ggml.` anahtarlarından kurar: GGUF depolarında `tokenizer.json` yoktur. Hiçbir anahtarı `tokenizer.ggml.` ile başlamayan dosyada parçalayıcı yoktur, yükleme hatasıdır (H5650); anahtarlardan bazıları varken biri eksikse hata o anahtarı söyler. Model `gpt2` olmalıdır; `tokenizer.ggml.pre` etiketi ön ayırıcıyı ve ayarlarını seçer: `qwen2`, `llama-bpe`, `smollm`. Etiket normalleştirmeyi de getirir: `qwen2` NFC uygular, çünkü Qwen modellerinin `tokenizer.json`'u uygular ve GGUF'ta yazmaz. Aynı etiketi NFC'siz modeller de taşıdığı için `qwen2` yalnız `general.architecture` `qwen2` ya da `qwen3` iken kabul edilir; başka mimari yükleme hatasıdır. GGUF özel olmayan altı parçayı denetim türüyle yazıyor (`<|fim_prefix|>`, `<|fim_middle|>`, `<|fim_suffix|>`, `<|fim_pad|>`, `<|repo_name|>`, `<|file_sep|>`): `qwen2` onları özel saymaz, `tokenizer.json`'daki gibi. Bilinmeyen etiket ve başka model yükleme hatasıdır, ileti etiketi söyler: başka ön ayırıcıyla sessizce başka parçalar çıkardı. `add_bos_token` ve `add_eos_token` anahtarı varsa o, yoksa etiketin varsayılanı geçerlidir. Tür dizisi zorunludur: yoksa özel parçalar sessizce bayta bölünürdü. Parça, puan ve tür dizilerinin boyu eşit olmalı, özel kimlikler sözlükte olmalıdır.
- *Test:* TR testi; seyrek sözlükte sınır da TR testi ("16.1 dağarcık boyutu kimliklerin sınırıdır: seyrek sözlükte en büyük kimlik + 1"); eklenmiş parçanın kimliği de TR testi ("16.1 eklenmiş parça dosyadaki kimliğini alır, dolu kimlik hatadır"); etiket ve bildirilen durma kimlikleri: Go testi `TestGGUFPretokenizerTag`, `TestGGUFDeclaredEosTokens` (sıra, tekrar, dağarcık dışı `eom`, hiçbiri; metin ve işaretli `eot`'ta parçalayıcı yüklenir, H6681 bildirilen kimliklerde), `TestLoadTokenizerFromGGUF` (TR'den okuma), `tokenizer.json`'da yoklukları TR testi ("16.1 ön_bölme_etiketi ve bildirilen_durma_parçaları tokenizer.json'dan yüklenen parçalayıcıda yok"); parçalayıcısız dosya ve eksik anahtar: `TestGGUFRejects` ("parçalayıcı yok" H5650, "model yok" ve "etiket yok" H6681); klasörle açma TR testi ("16.1 TR_load_tokenizer(klasör) ailesiz klasörde ailesiz açılır; tokenizer.json'u olmayan klasör H5610"): ailesiz klasör, `tokenizer.json`'u olmayan klasör; aileli klasör Go'da (aile seçimi; biçim parçaları test ailesiyle) ve gerçek dosyada Colab.

**16.2** `p.TR_encode(metin)` metni kimliklere çevirir ve `TR_List[TR_Int]` verir. `p.TR_encode(metinler)` (`TR_List[TR_String]`) her metni ayrı çevirir, sırayı korur ve `TR_List[TR_List[TR_Int]]` verir (13.10: iki imza).
- Metnin içinde yazılan eklenmiş parça (`tokenizer.json`'un `added_tokens`'ı: `<|endoftext|>` gibi özel ya da `<think>` gibi özel olmayan) tek bir kimlik ya da sıradan baytlar olur; seçim `TR_special_tokens:`'ındır (aşağıda).
- **İki argüman, iki iş.** `TR_boundary_tokens:` (`TR_Bool`, varsayılan `TR_true`) parçalayıcının şablonunun eklediği parçaları yönetir: başa ve sona, çiftte aradaki ayırıcı da (`TR_pair:`). `TR_special_tokens:` (`TR_Bool?`, varsayılan `TR_none`) yalnız metnin içini yönetir.
- `TR_special_tokens: TR_true` metindeki eklenmiş parça yazımını tek kimliğe eşler. `TR_special_tokens: TR_false` metnin içindeki eklenmiş parça yazımlarını, özel olsun olmasın, ve modelin kendi sözlüğündeki özel parçaları kapatır: metindeki `<|endoftext|>`, `<think>` ve unigram'ın `</s>`'si sıradan baytlar olarak kodlanır; parçayı isteyen kimliğini açıkça ekler (`p.TR_token_id(...)`).
- **Ailenin biçim parçaları.** Parçalayıcı bir model ailesinin dosyasından açılmışsa (GGUF'ta mimari ve ön bölme etiketiyle, klasörde 15.55'in sohbet tanımının seçimiyle) ailenin tanımı `TR_false`'ta da eşlenecek özel olmayan eklenmiş parçaları sayar. Bunlar yalnız biçim metnidir: ailenin protokol etiketleri (sohbet, araç, görüntü, düşünce) ve özel parçalar giremez; öbür eklenmiş parçaların kapatılması değişmez. Aileler: **`gemma3`**, iki küme: içeriği yalnız boşluk karakterlerinden (boşluk, `▁` U+2581, sekme, satır sonu, satır başı) oluşan parçalar ve içeriği `^</?[a-z]+>$` desenine uyan parçalar (yalnız küçük ASCII harfli HTML öğe adı, ör. `<b>`, `</b>`); **`llama3`** ve **`qwen3`**: boşluk kümesi boş, desen kümesi yok (bu ailelerde desene uyan `<think>` protokol etiketidir); boşluk parçası taşıyan dosya bu ailelere uymaz.
- **Kapalı küme.** Ailenin tanımı tanımlı her kümesinin, ailenin resmî dosyasındaki parça sayısını ve karmasını taşır; tanımlı olmayan küme denetlenmez. Yüklenen dosyanın kümesi bunlardan farklıysa (ör. özel olmayan yeni bir etiket ekleyen ince ayar) parçalayıcı ailesiz açılır. Ailenin kümesi bir protokol etiketiyle kesişirse yükleme hatasıdır (H7185).
- **Ailesiz parçalayıcı**, ailesi seçilemeyen her parçalayıcıdır (tek başına açılan bir `tokenizer.json`, ailesi bulunmayan bir klasör ya da GGUF, kümesi ailesininkine uymayan dosya): dosyada yalnız boşluk karakterlerinden oluşan özel olmayan bir eklenmiş parça varsa (dosyadaki içerik, normalleştirmeden önce) `TR_false` ile her kodlama çalışma hatasıdır; ileti çalışan yolu yazar: tek başına açılan dosyada, ailesi bulunmayan klasörde ya da GGUF'ta (H7178), dosyanın klasörü 15.55'in seçimiyle bir aile seçiyor ve dosyanın kümesi o ailenin kümesine uyuyorsa parçalayıcıyı o klasörle açmayı, her durumda güvenilir metinde `TR_true`'yu; kümesi uymayan dosyada (H7184) hangi kümenin nasıl ayrıldığını ve güvenilir metinde `TR_true`'yu. Böyle bir parça yoksa kapatma bugünkü gibidir.
- `TR_special_tokens:` verilmezse metnin içinde `TR_false`'un sıradan bayta çevireceği bir yazım geçince çağrı çalışma hatasıdır; ileti iki yolu yazar: dış veri `TR_special_tokens: TR_false`, biçimlenmiş güvenilir metin `TR_special_tokens: TR_true`. Çok metinli imzada ve `TR_pair:`'ın ikinci metninde her metin denetlenir, ileti metnin sırasını söyler. Modelin çıktısı yeniden kodlanırken de (16.3'ün varsayılanı `<think>`'i yazar) seçim açık yazılır.
- Sınır parçası eklenirken metnin eşlenen ilk ya da son parçası aynı kimlikse (biçim BOS'u kendisi taşıyor) çağrı çalışma hatasıdır; ileti `TR_boundary_tokens: TR_false`'u önerir. Çift sınır parçası sessizce oluşmaz.
- Güvenilmeyen girdi (sunmak) ve ham metin derlemi (ön eğitim verisi) `TR_special_tokens: TR_false` ile kodlanır; HTML'li derlemde `<s>` ve `</s>` sıradan metindir. Dizinin ortasındaki parça `TR_boundary_tokens: TR_false` da yazar. Sohbet derlemi 24.4'tedir.
- `TR_pair:` ikinci metni (ya da metin listesini) çift olarak kodlar; listeyle verilirse uzunlukları aynı olmalıdır.
- *Test:* TR testi; metin içi eklenmiş parçaların `TR_false` ile kapanması da TR testi ("16.2 TR_special_tokens: TR_false ile metin eklenmiş parçanın kimliğini üretmez"); `TR_boundary_tokens:`, verilmeyen `TR_special_tokens:`'ın hatası ve çift sınır parçası gerekmez, henüz uygulanmadı; uygulanınca TR testi; ailesiz yolun hatası TR testi ("16.2 ailesiz boşluk parçalı dosya ve kümesi uymayan dosya TR_false'ta hatadır, TR_true'da kodlanır"); Go: ailenin biçim parçaları test ailesiyle TR_false'ta tek kimlik, öbür eklenmiş parçalar ve protokol etiketleri kapalı; iki kümenin kuralı ve karması, kümesi uymayan dosya ailesiz açılır ve H7184'ün her kolu, kesişme H7185, H7178'in üç kolu (alone, gguf, folder); ailenin biçim parçaları kendi fikstüründe TR testi (`tests/data/gemma3_format_pieces/`: klasörden aileyle açılır, `\n\n` ve `<b>` `TR_false`'ta tek kimlik); gerçek dosyada ailenin kümeleri özel parçalarla ve protokol etiketleriyle kesişmez, biçim parçaları tek kimlik olur (Colab).

**16.3** `p.TR_decode(kimlikler)` tek bir diziyi metne çevirir: `TR_List[TR_Int]` ya da 1 boyutlu `TR_Tensor[TR_Int]`. Birden çok dizi `p.TR_batch_decode(diziler)` ile `TR_List[TR_String]` verir (13.8a).
- Özel parçalar varsayılan olarak yazılmaz; `TR_special_tokens: TR_true` onları da yazar.
- **Dağarcıkta olmayan kimlik çalışma hatasıdır;** ileti kimliği ve dağarcık boyunu söyler. Modelin çıktı boyu dağarcıktan büyükse (dolgu) ajan örneklemeyi dağarcığa sınırlar: `tr/layers`'ın üretiminde `TR_vocab_size: p.TR_vocab_size` verilir, kendi döngüsünde logitler `z.TR_slice(-1, 0, p.TR_vocab_size)` ile dilimlenir. Parçalayıcı modelin çıktısından büyükse (`p.TR_vocab_size` çıktının boyunu aşıyorsa) `tr/layers`'ın üretimi hata verir (H5648): ileti önce parçalayıcının o modelin parçalayıcısı olduğunun denetimini söyler, aynı modelin fazla kimliğinde (ör. çok kipli modelin görüntü yer tutucusu) örneklemeyi çıktıyla sınırlayan yazımı verir; kendi döngüsünde aynı durumda dilim sınır dışı uç hatası verir (15.3), denetim aynıdır ve aynı modelin fazla kimliğinde dilim gerekmez (çıktı dağarcıktan küçüktür). Seyrek sözlükte bu sınır en büyük kimlik + 1'dir, aradaki boş kimlik yine hatadır.
- Bayt düzeyindeki bir parçalayıcıda dizi bir karakterin ortasında biterse yarım kalan bayt dizisi U+FFFD (yerine koyma karakteri) olur, hata değildir: kesilen bir üretimin metni okunabilir kalır. Her yarım dizi bir U+FFFD'dir.
- *Test:* TR testi ("16.3 çöz: tek dizi; varsayılan özel parçaları atlar", "16.3 toplu_çöz: her dizi çöz ile aynı metin", "16.3 dağarcıkta olmayan kimlik çalışma hatasıdır").

**16.3a** Akışlı çözme: `p.TR_decode_stream(TR_special_tokens: b)` bir `TR_DecodeStream` verir; `akış.TR_step(kimlik)` o kimlikle tamamlanan yeni metni verir.
- Yarım kalan karakter beklenir: son kimlik bir karakterin ortasında bitiyorsa `.TR_step` `""` verir, karakter sonraki kimlikle tamamlanınca birlikte verilir. Kimlikleri tek tek `p.TR_decode([k])` ile çözmek her yarımda U+FFFD yazar (16.3).
- Dağarcıkta olmayan kimlik 16.3'teki gibi hatadır, o kimliğin `.TR_step`'inde.
- `akış.TR_finish()` kalan metni verir; yarım kalan dizi 16.3'teki gibi U+FFFD olur. Sonra `.TR_step` hatadır.
- **Söz:** bir akıştaki bütün `.TR_step` ve `.TR_finish` sonuçlarının birleşimi, aynı kimliklerin `p.TR_decode(kimlikler, TR_special_tokens: b)`'siyle aynı metindir.
- `.TR_step`'in sonucu veridir, maskelenmez (19.4). İnsana parça parça gösterilecek metin 19.4a'nın `TR_mask_stream()`'inden geçer: her `TR_print` ayrı maskelenir, iki parçaya bölünen gizli değeri görmez.

```tr
# TR 4
TR_var p = TR_load_tokenizer("data/tokenizer.json")
TR_var akış = p.TR_decode_stream(TR_special_tokens: TR_false)
TR_for k TR_in p.TR_encode("güneş") {
    TR_print(akış.TR_step(k)) # yarım karakterde ""
}
TR_print(akış.TR_finish())
```

- *Test:* gerekmez, henüz uygulanmadı.

**16.4** Kendi parçalayıcısını eğitmek: `TR_train_tokenizer(metinler, TR_vocab_size: n, TR_reserved_tokens: [...])` metinlerden bayt düzeyinde bir BPE parçalayıcısı eğitir ve `TR_Tokenizer` verir.
- `metinler` bir `TR_List[TR_String]` ya da dosya yollarının `TR_List[TR_Path]`'idir: her dosya bir metindir, satır satır okunur (`.gz` 18.12 gibi); dosya listesiyle eğitim derlemi belleğe almaz. Metin veren bir akış (5.7) 13.10'a "dolaşılabilir" kısıtı eklenince alınır.
- `TR_reserved_tokens:` dağarcığın başına konan özel parçalardır (`["<|endoftext|>"]`); sırayla 0'dan kimlik alırlar.
- `TR_vocab_size:` en az 256 artı ayrılmış parça sayısıdır (bayt düzeyinde BPE'nin tabanı 256 bayttır); daha küçüğü hatadır, ileti en küçük değeri söyler.
- **Belirlenimci:** aynı metinler ve aynı argümanlar her makinede aynı parçalayıcıyı verir. Ön ayırıcı ve çözücü bayt düzeyindedir (GPT-2'nin düzenli ifadesi ve 256 baytın Unicode eşlemesi; `tokenizer.json`'da `ByteLevel`, `add_prefix_space: false`). Taban 256 bayt, ayrılmış parçalardan sonra bayt sırasıyla numaralanır. Sıklığı eşit çiftlerden önce birleşen, iki parçanın bayt dizilerinin sözlük sırasıyla küçüğüdür.
- Eğitilen parçalayıcı başa ya da sona kendiliğinden özel parça eklemez.
- `p.TR_save(yol)` parçalayıcıyı `tokenizer.json` biçiminde yazar; `TR_load_tokenizer` ve başka araçlar onu okur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

## Veri

**16.5** `TR_Dataset(yol)` kimlik parçalarını açar: bir klasördeki `.npy` dosyalarını ad sırasıyla ya da tek bir `.npy` dosyasını.
- Parçaların öğeleri tam sayıdır: işaretsiz 16 ya da 32 bit, işaretli 32 ya da 64 bit (NumPy'nin varsayılanı). Başka türdeki dosya hatadır; ileti dosyanın türünü söyler.
- `meta.json` varsa parça uzunlukları ondan okunur.
- `TR_cache: klasör` parçaları yerel bir önbelleğe kopyalar; o zaman `TR_quota_gb:` zorunludur. Önbellek her parçayı kaynak kümenin adı, parçanın adı, bayt boyu ve değiştirilme zamanıyla anahtarlar; kaynak değişince kopya yenilenir. Aynı önbellek klasörünü kullanan kümeler tek kotayı paylaşır, aynı adlı parçaları ayrı tutulur; klasörü aynı anda başka bir program kullanıyorsa hata.
- Dosyalar açıkça kapatılmaz: sıralı okuyucu bitince, kullanılmayan nesneler toplanınca ve program bitince kapanır.
- *Test:* TR testi.

**16.6** Veriyi eğitime hazırlamak: `TR_write_dataset(yol, diziler, TR_shard_tokens: n)` kimlik dizilerini `yol` klasörüne `.npy` parçaları olarak yazar.
- `diziler` bir `TR_List[TR_List[TR_Int]]`'dir; diziler sırayla uç uca eklenir. Akış 16.4'teki gibi sonra.
- Her parça en çok `TR_shard_tokens` kimlik tutar; parçalar ad sırasıyla okunacak biçimde adlandırılır ve `meta.json` yazılır. Hedef klasörde var olan kümenin yerine yazılır: eski parçalar ve `meta.json` atomik olarak değişir (18.13).
- Öğe türü kimliklerin en büyüğüne sığan en dar işaretsiz türdür: 65 536'dan küçükse 16 bit, değilse 32 bit. Eksi ya da 2³²'ye sığmayan kimlik hatadır.
- Yazma yolu 13'ün yazma kuralına uyar (çalışma klasörü).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**16.7** `küme.TR_sampler(TR_batch_size: b, TR_context: c, TR_seed: s)` rastgele pencereler veren bir `TR_Sampler` kurar.
- `.TR_next()` bir demet verir (3.6a): `TR_var girdi, hedef = ö.TR_next()`. İkisi de `[b, c]` şekilli `TR_Tensor[TR_Int]` "int64"tür, parçanın diskteki türü ne olursa olsun (16.5): çapraz entropinin yok sayılan hedefi `-100` (15.23a) işaretsiz türde tutulamaz. `hedef`, `girdi`nin bir kaydırılmışıdır: bir pencere `TR_context + 1` kimlik ister. Daha kısa parçalar atlanır; hiçbir parça yetmiyorsa 16.9'un "bağlam veri parçasından uzun" hatası. Toplu işler varsayılan cihazda kurulur (15.38). Pencere bir parçanın sınırını aşmaz.
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
