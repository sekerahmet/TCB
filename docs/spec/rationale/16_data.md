# 16. Veri ve parçalayıcı: emsal ve gerekçe

Kurallar: [`16_veri_parcalayici.md`](../16_veri_parcalayici.md). Bu dosya her kuralın emsalini (başka dillerde ve araçlarda nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 16.1

- *Emsal:* Hugging Face `tokenizers`: "To save the tokenizer in one file that contains all its configuration and vocabulary, just use the Tokenizer.save method ... you can reload your tokenizer from that file with the Tokenizer.from_file classmethod". ([tokenizers, Quicktour](https://huggingface.co/docs/tokenizers/quicktour))
- *Gerekçe:* amaç hazır modelleri çalıştırmayı ve başka araçlarda eğitilmiş modelleri almayı ister; yayımlanmış modellerin parçalayıcısı bu tek dosyadadır. Dosya dışarıdan gelir; içindeki düzenli ifade kötü niyetle yavaşlatılabilir, adım sınırı Güvenilir'in gereğidir.

- *Gerekçe (`.TR_vocab_size`, 2026-11-04, DENETİM istek 79 Ş2):* adın her kullanımı bir kimlik sınırıdır: gömme tablosu, örnekleme sınırı. Parça sayısı seyrek sözlükte geçerli kimliği sınırın dışında bırakır ve sessizdir; en büyük kimlik + 1 ise boş kimliği içeri alır ama `.TR_decode` onu hatayla söyler. Yeni ad gerekmez: aralıksız sözlükte iki sayı aynıdır.
- *Emsal, karşı örnek:* Hugging Face `tokenizers` 0.23.2'nin `get_vocab_size()`'ı `{"[UNK]":0,"a":1,"b":5}` sözlüğünde 3, `id_to_token(3)` `None` verir [ölçüldü: TCB]; o kütüphanede sınır çağıranın işidir.
- *Gerekçe (eklenmiş parçanın kimliği, 2026-11-04):* kimlik modelin satırını seçer; TR onu değiştirmez, tutmayan dosyayı reddeder. Kural tek cümledir ve dosyadaki sıraya bağlı değildir. Önceki sıra hesabı `{"[UNK]":0,"a":1,"b":5}` üstünde 3, 4, 5 kimlikli parçaları kabul edip `b`'yi eziyordu: `a b z` metni `a z z` olarak dönüyordu, hatasız.
- *Emsal, karşı örnek:* Hugging Face `tokenizers` 0.23.2 sözlükte olmayan eklenmiş parçanın dosyadaki kimliğini okumaz, sıradakini verir: aynı sözlükte 9 yazan parça 3 olur; sıra dolu kimliğe gelirse sözlükteki parçayı ezer [ölçüldü: T1].
- *Risk, iki yönlü [hipotez: gerçek dosyada görülmedi]:* budanmış sözlükte modelin satırları dosyadaki kimliklere göre durur; `tokenizers`'ın 3'ü orada sessiz yanlıştır. Tersi de olabilir: başka araçta `tokenizers` ile eğitilen bir modelin satırları sıradaki kimliğe göre durur, dosyadaki kimlik yanlıştır ve TR'nin 9'u yanlış satırı seçer. Qwen3 ve SmolLM2'de iki yol aynı kimliği verir [ölçüldü: T1].

### 16.2

- *Emsal:* `tokenizers`'ın `encode` ve `encode_batch`'i; PyTorch tarafında `add_special_tokens` bayrağı [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tek metin ile metin listesinin dönüş tipi ayrıdır ve argümanın tipinden çalışmadan önce bellidir; 13.10'un iki imzasıdır. Ayrı bir toplu ad gerekmez.
- *Gerekçe (metin içi özel parça):* `TR_special_tokens: TR_false` metin içindeki yazımı açık bırakırsa güvenilmeyen girdideki `<|im_end|>` bir kontrol kimliğine dönüşür, derlemdeki düz `<|endoftext|>` metni de belge sınırı sanılır (Gemini Koşu 7).
- *Emsal (özel olmayan eklenmiş parça):* Hugging Face `<think>` gibi `special: false` parçaları her yolda eşler; `split_special_tokens=True` yalnız özelleri böler [ölçüldü: DENETİM istek 43 ve TCB, 2026-11-02, tokenizers 0.23.2, transformers 5.17.0]. llama.cpp bunları `USER_DEFINED` sayar ve `parse_special=false`'ta da eşler [DENETİM açtı: v0.8.0 `src/llama-vocab.cpp`]. vLLM şablon metnini HF parçalayıcısıyla kodlar, içerik için ayrı koruma yoktur [DENETİM açtı: v0.33.0rc3 `vllm/renderers/hf.py`]. tiktoken'da özel olmayan eklenmiş parça yoktur, eklenmiş her parça özeldir; varsayılan metinde geçeni hata sayar [DENETİM açtı: tiktoken 0.14.0 `core.py`]. *TR `TR_false`'ta tiktoken'ı izler, HF'den ayrılır.*
- *Gerekçe (özel olmayan eklenmiş parça):* Qwen3'ün `<tool_call>`, `<tool_response>`, `<think>` parçaları özel değildir ve şablon araç sonucunu `<tool_response>` ile sarar [DENETİM açtı: Qwen3-8B `tokenizer_config.json`]. HF gibi eşlemek, bir araç sonucundaki düz `</tool_response>` metnini kimliğe çevirir ve sarmalı erkenden kapatır: sahte bir araç sonucu. Güvenilmeyen girdiyi kapatan bayrak bütün eklenmiş parçaları kapatmalıdır. Varsayılan (`TR_true`) tiktoken gibi hata değildir, HF gibi eşlemedir: biçimlenmiş güvenilir metin özgün uygulamayla aynı kimlikleri verir (24.4). Ham metin derlemi `TR_false` ile kodlanır; biçimlenmiş sohbet derleminin yolu 24.4'tedir (DENETİM istek 43, Ş2).

### 16.3

- *Emsal:* `tokenizers`'ın `decode` ve `decode_batch`'i [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tek dizi ile birden çok dizinin ayrımı 13.8a'da `TR_batch_decode` ile yapıldı: tensörün boyut sayısı tipte değildir, dönüş tipi değerden çıkamaz. Dağarcık dışı kimliğin sessizce atlanması bir hatayı gizler: dolgu yüzünden dağarcıktan büyük çıktı veren model, örneklemeyi sınırlamayı unutan ajanın metninden harf düşürür ve kimse fark etmez. Hata bunu ilk satırda gösterir. Bu, emsalden bilerek ayrılmaktır: Hugging Face `tokenizers` 0.23.2 ve `transformers` dağarcık dışı kimliği sessizce atlıyor [ölçüldü: DENETİM, 16-1].
- *Gerekçe (yarım karakter):* kesilen bir üretim (en çok parça sınırı) karakterin ortasında biter; hata o metni okunmaz kılardı. U+FFFD görünür bir işarettir. Hugging Face ile beş dizide bayt bayt aynı, her yarım dizi bir U+FFFD (Unicode'un en uzun geçerli alt parça kuralı) [ölçüldü: TCB ve DENETİM istek 43, 2026-11-02, tokenizers 0.23.2]; tiktoken'ın `decode`'u da varsayılanla `errors="replace"`'tir [DENETİM açtı: tiktoken 0.14.0 `core.py`].

### 16.3a

- *Emsal:* `tokenizers`'ın `DecodeStream`'i çıktı U+FFFD ile bitiyorsa parçayı tamponda tutar [DENETİM açtı: tokenizers v0.21.1 `tokenizer/mod.rs`, `step_decode_stream`]. llama.cpp `common_utf8_is_complete`, Ollama `IncompleteUnicode` ile son baytların tamamlanmasını bekler [rapordan]. transformers'ın `TextStreamer`'ı bütün kimlikleri yeniden çözer ve son boşluğa kadar basar [rapordan].
- *Gerekçe:* Jarvis üretilen metni parça parça sese ve ekrana verir. Türkçe harfler (`ğ`, `ş`) bayt düzeyindeki parçalayıcıda iki parçaya bölünebilir; parçayı tek başına çözen ajan her harfte U+FFFD yazar ve bu harfi kalıcı olarak bozar [rapordan, `[görüş]`]. Birleşimin toplu çözümle aynı olması sınanabilir bir sözdür ve parçalayıcının komşu parçaya bağlı çözücülerini (baştaki boşluk) de kapsar. `.TR_finish` kalan yarımı sessizce atmaz.

### 16.4

- *Emsal:* `tokenizers` metinlerden eğitir ve tek dosyaya yazar: `tokenizer.train(files, trainer)`, `tokenizer.save(...)` ([tokenizers, Quicktour](https://huggingface.co/docs/tokenizers/quicktour)).
- *Gerekçe:* amaç "kendi parçalayıcısını eğitmek"i veri hazırlamanın parçası sayar; bugün TR'de bu yok, Python'a gidilir. Bayt düzeyi BPE her dilde ve her baytta çalışır, bilinmeyen parça yoktur. Belirlenimcilik Güvenilir'in gereğidir: aynı veriyle eğitilen parçalayıcı aynı olmazsa aynı model iki kez kurulamaz. Emsal bunu sağlıyor: HF'nin BPE eğitimi 1 ve 8 iş parçacığında, metin sırası ters verildiğinde de aynı parçalayıcıyı verdi [ölçüldü: DENETİM, 16-1]. Biçim yaygın `tokenizer.json`'dur, böylece eğitilen parçalayıcı paylaşılır.
- *Gerekçe (eşitlik kuralı ve dosya listesi):* "sabit bir kural" yazılmadan belirlenimcilik sınanamaz. Hugging Face `tokenizers` ile eşitlik bozmanın aynı olup olmadığı bir kez sınanır [hipotez]; ayrıysa fark buraya yazılır. Dosya listesinden eğitim Hugging Face'in `tokenizer.train(files, ...)`'i gibidir (Gemini Koşu 7).

### 16.5

- *Emsal:* nanoGPT veriyi işaretsiz 16 bit kimlik dizisi olarak yazar ve okur: "dtype = np.uint16 # (can do since enc.max_token_value == 50256 is < 2**16)". ([nanoGPT, data/openwebtext/prepare.py](https://github.com/karpathy/nanoGPT/blob/master/data/openwebtext/prepare.py))
- *Gerekçe:* kimlik verisi büyüktür; 16 bit, 32 bitin yarı belleğidir. 65 536'yı aşan sözlükler (128 bin parçalı modeller) 32 bit ister. Dosyadaki tür ne olursa olsun okunan kimlik `TR_Int`'tir (15'in cinsi). NumPy'nin varsayılan tam sayısı 64 bittir; NumPy ile hazırlanan veri yolda kalmasın diye işaretli 64 bit de okunur [ölçüldü: DENETİM, NumPy 2.1.1].

### 16.6

- *Emsal:* nanoGPT'nin `prepare.py`'si metni kodlayıp kimlikleri tek bir 16 bitlik dosyaya yazar (aynı kaynak).
- *Gerekçe:* amaç "eğitime hazırlamak"ı ister; okunan biçimi yazan bir yerleşik olmadan veri hazırlamak Python'a gider. Türün en büyük kimliğe göre seçilmesi ajanın yanlış türü seçip kimliği sessizce kesmesini önler; sığmayan kimlik hatadır.

### 16.7

- *Emsal:* nanoGPT rastgele pencereler alır ve hedef bir kaydırılmıştır: `get_batch` aynı belleğe eşlenmiş diziden okur ([nanoGPT, train.py](https://github.com/karpathy/nanoGPT/blob/master/train.py)).
- *Gerekçe:* Güvenilir: "kesilen bir eğitim kaldığı yerden aynı sonuçla sürer". Örnekleyicinin durumu kontrol noktasına girmezse geri yüklenen eğitim başka toplu işler görür ve sonuç değişir. Demet, girdi ile hedefi açma yazımıyla verir; liste iki öğeli olsa da tipi bunu söylemez. Kimlikler tam sayı tensörüdür (15'in A'sı): 2²⁴'ü aşan kimlik ondalıkta bozulurdu. Bugün örnekleyici kontrol noktasından yeni bir süreçte aynı toplu işleri veriyor [ölçüldü: DENETİM, 16-1, 3–5. adımlar birebir].

### 16.8

- *Emsal:* değerlendirmede veri sırayla ve bir kez gezilir (PyTorch'un `DataLoader`'ı karıştırmasız) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ölçmek, aynı verinin her seferinde aynı sırayla görülmesini ister; sonlu olduğu için döngüyle gezilir.

### 16.9

- *Emsal:* 8. bölümün hata kuralı.
- *Gerekçe:* hata kodla ayrılır, metinle değil (8.3); ajan kodu yakalar.

### 16.10

- *Emsal:* 13'ün yol kuralı.
- *Gerekçe:* her yerleşikte tek yol kuralı; ajan ikinci bir kural öğrenmez.
