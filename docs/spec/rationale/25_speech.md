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
