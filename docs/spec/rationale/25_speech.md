# 25. Konuşmayı yazıya çevirme: emsal ve gerekçe

Kurallar: [`25_konusma_tanima.md`](../25_konusma_tanima.md). Bu dosya her kuralın emsalini ve seçimin gerekçesini tutar.

Araştırma: Gemini Whisper raporu (2026-11-05). DENETİM alıntıları doğruladı (istek 175), metni özgün koda karşı okudu (istek 177). Sürümler: `openai/whisper` `v20250625`, HF `transformers` v4.51.3, `faster-whisper` v1.2.1, `candle` 0.11.0.

### 25.1

- *Emsal:* HF, `faster-whisper` ve `candle` Whisper'ı ayrı sınıfla açıyor [kanıtlı: DENETİM 175].
- *Gerekçe:* `TR_pretrained_model` `TR_GPT` döndürür, klasörün `model_type`'ı çalışma anı verisidir; aynı çağrı iki tip dönemez (1.2). H6697 ve H6945 metni `TR_pretrained_gguf` ve `TR_pretrained_model` diye sabittir; Whisper yolu kendi iletisini ister [ölçüldü: DENETİM 175].

### 25.2

- *Emsal:* özgün kesitte metin sonundan, üst metinde damgadan küçük kimlikleri süzüyor [kanıtlı: DENETİM 177].
- *Gerekçe:* dönüş yapıdır, demet değil: alanlı değer indeks ve süre karışıklığını kaldırır, alan eklemek kırmaz. "Kesit" denir, çünkü "parça" TR'de token için kullanılıyor. Damgalar `tokenizer.json`'da özel parça değil, `TR_decode` atmaz.

### 25.3

- *Emsal:* `faster-whisper` ve `candle` yalnız kısa kod alıyor [kanıtlı: DENETİM 175]. Tek dilli modelde özgün Python `"tr"`'yi sessizce yutuyor, HF her değeri reddediyor.
- *Gerekçe:* `"turkish"` tablosu İngilizceye ayrıcalık verirdi. TR `"en"`'i kabul eder, öbürünü reddeder: sessiz yanlış olmaz, model de kullanılabilir kalır.

### 25.4

- *Emsal:* özgün başarısız pencerede sıcaklığı artırıyor; `compression_ratio` yalnız bu yedek döngüsünde okunur [kanıtlı: `transcribe.py:205`].
- *Gerekçe:* `temperature > 0` örnekleme ister, 15.22a'yı zorlar. Tek sıcaklıkta `zlib` gerekmez. Bedeli tekrara kilitlenmedir; bilinen sınır.

### 25.5

- *Emsal:* süzgeç sırası `decoding.py:554`–`:566`; `suppress_tokens` özgünün `_get_suppress_tokens`'ıyla aynı, dört modelde fark 0 [ölçüldü: DENETİM 175].
- *Gerekçe:* damga kuralının `logsumexp`'i bastırılmış logitlerden hesaplanır, sıra sonucu değiştirir [kanıtlı: DENETİM 175]. Dizinin ilk damgası çift sayılır (`decoding.py:464`): yazılmazsa `<0><0>` geçer, pencere ilerlemez [hesap: DENETİM 177].

### 25.6

- *Emsal:* `sample_len`, `n_ctx`; pencere sonunda metin sonu eklenir (`decoding.py:295`–`:297`) [kanıtlı: DENETİM 175].
- *Gerekçe:* sınırlar özgünündür; ayrılırsa parçalar özgünle aynı kalmaz.

### 25.7

- *Emsal:* `transcribe.py:375`, `:389`, `:486`.
- *Gerekçe:* son ardışık çiftin ilk damgasına dönülür: son damgaya dönmek arada kalan metni sessizce atardı [hesap: DENETİM 177].

### 25.8

- *Emsal:* `<|nospeech|>` yalnız `large-v3`'te, öbürlerinde `<|nocaptions|>`; `candle` iki adı deniyor [ölçüldü: DENETİM 175].
- *Gerekçe:* kimlik `no_timestamps_token_id − 1` üç modelde de doğru. Ortalamanın böleni `len(t) + 1`'dir; pencere dolunca eklenen metin sonu bölende sayılır, toplama girmez (`decoding.py:287`, `:761`).

### 25.9

- *Emsal:* yok: dilin kendi dilim sınırı.
- *Gerekçe:* `TR_conv2d` görüntü dilimine kalır; kart ayrı iştir, ilk ölçü CPU'da.

## Konuşma algılama

Araştırma: Gemini canlı ses raporu, Soru 3 (2026-11-06). DENETİM yönergeyi okudu ve ileri geçişi `.tr` ile yazdı (istek 240). Sürümler: `snakers4/silero-vad` v6.2.3, `faster-whisper` v1.2.1.

### 25.10

- *Emsal:* Silero VAD v6.2.3, MIT; mimari `tinygrad_model.py`. *Rust:* `silero-vad-rust` 6.2.2 aynı modeli ONNX'ten çalıştırır [ölçüldü: crates.io araması].
- *Gerekçe:* üç adımlı evrişim 640 örneği tek zaman adımına indirir, zaman üstünde dönen LSTM yoktur: yeni yerleşik gerekmez [ölçüldü: DENETİM 240]. Safetensors biçimi 15.47'yle açılır; ONNX gerekmez. Adlar sabit olduğu için 15 tensör adla denetlenir: başka dosya hatasız yüklenip yanlış olasılık vermez.
- *Model dosyası:* v6.2.3'ün iki dosyası aynı ağırlık değildir. `silero_vad_16k.safetensors` ile `silero_vad.jit`'in 16 kHz modelinde 15 ad ve şekil aynıdır, yalnız `stft_conv.weight` aynı değerdedir; öbür 14 tensör farklıdır, en büyük öğe farkı `conv4.weight`'te 18,18 [ölçüldü: T1], korelasyon 0,90–0,997 [ölçüldü: DENETİM 242]. Paketin `load_silero_vad()`'ı JIT açar [kanıtlı: DENETİM 242, `model.py`]; safetensors için yayımlanmış kalite ölçüsü bulunamadı. Bu kural safetensors'u açar: dosya biçimi `.tr`'den okunur, `TR_pretrained_vad` iki ağırlıkta da aynı kalır; hangi modelin dağıtılacağı ayrı iştir.

### 25.11

- *Emsal:* Silero'nun `model(x, 16000)`'i 512 örnek ister, son 64 örneği bağlam tutar, `reset_states` ikisini sıfırlar [kanıtlı: `silero_vad.jit`'in `forward`'ı]. NaN ve `[-1, 1]` denetimi emsalde yok.
- *Gerekçe:* tek NaN çerçeveden sonra sonraki beş temiz çerçevenin hepsi NaN'dır; `nan >= 0.5` ve `nan < 0.35` ikisi de yanlıştır, kesitleyici hatasız sonsuza dek dinlerdi [ölçüldü: DENETİM 240]. `1e20` genlik sonludur ama karesi `float32`'de taşar: durum ancak sonuç sonluysa yazılır. Bir çerçevenin süresi 32 ms olduğu için CPU yeter; ölçü DEVİR'dedir. Aralık denetlenmez: bölü 32768 unutulursa hata yok, olasılık gelir [ölçüldü: DENETİM 245, kanal modeli]; bilinen sınır.

### 25.12

- *Emsal:* `max(threshold - 0.15, 0.01)` `faster-whisper` `vad.py:95` ve `utils_vad.py:476`. 0,5 iki kaynağın varsayılanıdır.
- *Gerekçe:* sabit 0,35 varsayılan, `TR_start_threshold: 0.3` yazan kullanıcıya hata verirdi. Emsalin formülü 0,01'in altındaki başlangıçta bitişi başlangıcın üstüne çıkarır; dış `min` bu geçerli değere de hata verdirmez. Yanlış değer sessiz yanlış değil, gecikme ya da fazla kesit üretir: 25.3'ün zorunlu dili buraya uymaz, varsayılan vardır. 0,32 ve 0,6 görüştür: emsal aralıkları 30–400 ms ve 100–2000 ms'dir (`utils_vad.py:596`–`:597`, `vad.py`). 0,32 aralığın üst yarısında: Silero'nun 30 ms'si tek çerçevedir, `faster-whisper`'ın 400 ms'si çevrimdışıdır; 0,6, 100 ms'den altı kat uzun, cümle içi nefesi bölmez, 2000 ms'den 1,4 saniye kısa, canlı yanıtı geciktirmez. 29 saniye türetilmiştir: kesit Whisper'ın 30 saniyelik penceresine sığar (25.7).
- Üst sınır çerçeveye yuvarlandıktan sonra denetlenir: makine çerçeveyle çalışır; `TR_pre_roll: 0.001, TR_silence: 0.001, TR_max_speech: 0.002` saniyede geçerdi, sınır 0 çerçeve olurdu.

### 25.13

- *Emsal:* eşikler `utils_vad.py:666`, `:671`; sıfırlama `:663`. Silero'da sayaç ilk `p < bitiş` çerçevesinden geçen süredir, iki eşik arası çerçeve süreye girer.
- *Gerekçe:* TR yalnız `p < bitiş` çerçevelerini sayar: "sessizlik" o çerçevedir. Sessizlik 19 çerçevede bir kez yazılıdır, belirsiz bölgede kesit erken bitmez. Kesitin kuyruğu iki kuralda da kesittedir: son ünsüz sayaca bağlı değildir. Silero'dan uzaklık en çok 2 çerçevedir, kesit sayısı hiç ayrılmadı [ölçüldü: DENETİM 245; FLEURS tr dev, 338 kayıt, temiz, 20 dB ve 10 dB beyaz gürültü, iki ağırlık, kesitleyicinin Python eşidi]. İki kural yalnız sessiz ve ara çerçeve dönüşümlü gelirse ayrılır: TR'nin bedeli üst sınıra kadar gecikmedir, Silero'nunki zayıf konuşmanın kesilmesidir [hesap: DENETİM 245]; ölçüde ikisi de görülmedi. 600 / 32 = 18,75 ve 29 000 / 32 = 906,25: sessizlik ve tampon "en az", üst sınır "en çok" okunur.

### 25.14

- *Emsal:* `faster-whisper` ve Silero dolguyu kesite ekler (`speech_pad_ms`). En kısa konuşma Silero'da 250 ms, `faster-whisper`'da 0'dır (`utils_vad.py:285`).
- *Gerekçe:* kuyruğun atılması son ünsüzü keserdi. Tek çerçevelik kesit 10 + 1 + 19 = 30 çerçevedir (960 ms); en kısa konuşma kuralı somut ihtiyaç görülmeden eklenmez.

### 25.15

- *Emsal:* 98 ms `vad.py:80`, `utils_vad.py:295`. Bölme yeri "en son" `faster-whisper`'ın, "en uzun" Silero v6.2.3'ün varsayılanıdır (`utils_vad.py:502`). Kalan konuşmasızsa tetik kapanması `utils_vad.py:508`–`:511`, `vad.py:122`. Sınırda kesme `utils_vad.py:527`.
- *Gerekçe:* canlı akışta en son duraklama kesiti en uzun tutar, kalan en kısa olur. Emsaller dolguyu sınırdan düşer (`utils_vad.py:467`, `vad.py:74`–`:78`); TR'de sınır bütün boyu sayar, böylece "30 saniyeye sığar" argüman değerinden okunur. Ortadan bölmek duraklamanın iki yanını iki kesite paylaştırır. Baş tamponundaki duraklama sayılmaz: orada bölmek sessiz kesit verirdi. Emsallerde kalan konuşmasızsa tetik kapanır, kalan kesite girmez; TR'de kalan iki eşik arası çerçeve tutabilir, dışarıda bırakmak 25.14'ü bozar: 3 konuşma, 6 sessiz, 31 ara çerçevede, sınır 40'ta 34 çerçeve kaybolurdu [ölçüldü: DENETİM 245]. Kesit sınırda biter: sınırı geçmez, konuşma içerir, örnek düşmez.

### 25.16

- *Emsal:* Silero'nun `VADIterator`'ında karşılığı yok: akış kapanınca `reset_states` çağrılır.
- *Gerekçe:* kesit ile dedektör ayrı durumdur: akış sürerken kesiti almak LSTM'i sıfırlamamalıdır.

### 25.17

- *Emsal:* 15.22a.
- *Gerekçe:* akış sabit 512 örnekle ilerler; "tamponda ne varsa" okumak çerçeve sınırlarını zamana bağlardı (canlı ses raporu, Soru 5). `GOMAXPROCS` 1 ve 8 aynı sonucu verir [ölçüldü: DENETİM 240].
