# 16. Veri ve parçalayıcı: emsal ve gerekçe

Kurallar: [`16_veri_parcalayici.md`](../16_veri_parcalayici.md). Bu dosya her kuralın emsalini (başka dillerde ve araçlarda nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 16.1

- *Emsal:* Hugging Face `tokenizers`: "To save the tokenizer in one file that contains all its configuration and vocabulary, just use the Tokenizer.save method ... you can reload your tokenizer from that file with the Tokenizer.from_file classmethod". ([tokenizers, Quicktour](https://huggingface.co/docs/tokenizers/quicktour))
- *Gerekçe:* amaç hazır modelleri çalıştırmayı ve başka araçlarda eğitilmiş modelleri almayı ister; yayımlanmış modellerin parçalayıcısı bu tek dosyadadır. Dosya dışarıdan gelir; içindeki düzenli ifade kötü niyetle yavaşlatılabilir, adım sınırı Güvenilir'in gereğidir.

### 16.2

- *Emsal:* `tokenizers`'ın `encode` ve `encode_batch`'i; PyTorch tarafında `add_special_tokens` bayrağı [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tek metin ile metin listesinin dönüş tipi ayrıdır ve argümanın tipinden çalışmadan önce bellidir; 13.10'un iki imzasıdır. Ayrı bir toplu ad gerekmez.
- *Gerekçe (metin içi özel parça):* `TR_special_tokens: TR_false` metin içindeki yazımı açık bırakırsa güvenilmeyen girdideki `<|im_end|>` bir kontrol kimliğine dönüşür, derlemdeki düz `<|endoftext|>` metni de belge sınırı sanılır (Gemini Koşu 7).

### 16.3

- *Emsal:* `tokenizers`'ın `decode` ve `decode_batch`'i [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tek dizi ile birden çok dizinin ayrımı 13.8a'da `TR_batch_decode` ile yapıldı: tensörün boyut sayısı tipte değildir, dönüş tipi değerden çıkamaz. Dağarcık dışı kimliğin sessizce atlanması bir hatayı gizler: dolgu yüzünden dağarcıktan büyük çıktı veren model, örneklemeyi sınırlamayı unutan ajanın metninden harf düşürür ve kimse fark etmez. Hata bunu ilk satırda gösterir. Bu, emsalden bilerek ayrılmaktır: Hugging Face `tokenizers` 0.23.2 ve `transformers` dağarcık dışı kimliği sessizce atlıyor [ölçüldü: DENETİM, 16-1].

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
