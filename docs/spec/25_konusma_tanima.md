# 25. Konuşmayı yazıya çevirme

Bu bölüm yayımlanmış bir Whisper modelini açmayı, bir sesi yazıya çevirmeyi ve sürekli gelen sesten konuşma kesitleri çıkarmayı yazar. Girdi 22.4'ün `TR_Sound`'u, log-Mel 23.6'da yazılıdır. Çevirmenin ölçütü özgün `transcribe`'dır: `temperature=0.0`, `beam_size=None`, `fp16=False`. Konuşma algılamanın modeli Silero VAD v6.2.3'tür.

Emsal ve gerekçe: [`rationale/25_speech.md`](rationale/25_speech.md).

## Model

**25.1** `TR_pretrained_whisper(yol)` `tr/whisper` kitaplığındadır ve bir `TR_Whisper` verir. Yol Hugging Face biçiminde bir klasördür.
- Dosyaları `model.safetensors`, `config.json`, `tokenizer.json`, `generation_config.json`'dur. Biri, gerekli bir alan ya da gerekli bir özel parça yoksa açılışta hatadır; ileti eksiği söyler. Uzun bir sesten önce, klasör açılırken:
  - `config.json`: `model_type`, `d_model`, `num_mel_bins`, `vocab_size`, `max_source_positions`, `max_target_positions`, `encoder_layers`, `encoder_attention_heads`, `encoder_ffn_dim`, `decoder_layers`, `decoder_attention_heads`, `decoder_ffn_dim`;
  - `generation_config.json`: `suppress_tokens`, `no_timestamps_token_id`; çok dilli modelde `lang_to_id` (25.3);
  - `tokenizer.json`'un özel parçaları: `<|endoftext|>`, `<|startoftranscript|>`, `<|startofprev|>`, `<|transcribe|>`. Damgalar adla aranmaz: `no_timestamps_token_id`'den sonraki kimliklerdir (25.5).
- `config.json`'un `model_type`'ı `whisper` değilse hatadır; ileti `TR_pretrained_model`'i önerir. `TR_pretrained_model`'e Whisper klasörü vermek de hatadır; ileti bu adı önerir (15.55).
- **Desteklenmeyen mimari hatadır** (model hatasız yüklenip yanlış metin üretirdi): boyutlar sıfırdan büyüktür, iki bakış sayısı `d_model`'i böler; `activation_function` verilmişse `gelu`'dur, `scale_embedding` verilmişse `false`'tur.
- Çıkış katmanı gömmeye bağlıdır: dosyada ayrı ağırlık aranmaz.
- Ağırlık `float32`, `fp16` ya da `bf16`'dır; öbür tür hatadır. 16 bitlik ağırlık açılırken `float32`'ye genişletilir; hesap `float32`'dedir. Nicemli ağırlık yoktur.
- `model.TR_transcribe` dıştaki `TR_mixed_precision` bloğundan etkilenmez: kendi hesabını `"float32"` bloğunda yapar (15.4), 25.4'ün aynı metni bloğa bağlı değildir.
- `num_mel_bins` `config.json`'dan okunur; 80 ve 128 dışındaki değer hatadır.
- Bilinmeyen, eksik ya da şekli `config.json`'a uymayan tensör hatadır; ileti tensörün adını söyler.
- *Test:* TR testi.

```tr
# TR 4
TR_import "tr/audio"
TR_import "tr/whisper"
TR_var model = TR_pretrained_whisper("whisper-tiny")
TR_var yazı = model.TR_transcribe(TR_read_wav("soru.wav"), TR_language: "tr")
TR_print(yazı.TR_text)
TR_for k TR_in yazı.TR_segments { TR_print(k.TR_start, k.TR_end, k.TR_text) }
```

## Çevirme

**25.2** `model.TR_transcribe(ses, TR_language: ad)` bir `TR_Transcription` verir. Dil tespiti, çeviri ve zaman damgasız kip yoktur.
- `TR_Transcription`: `.TR_text: TR_String`, `.TR_segments: TR_List[TR_Segment]`, `.TR_language: TR_String`.
- `TR_Segment`: `.TR_start: TR_Float` ve `.TR_end: TR_Float` saniyedir, `.TR_text: TR_String`.
- Zaman damgaları metne girmez. Kesitin metni metin sonundan küçük kimliklerden çözülür. `TR_Transcription`'ın metni kesit metinlerinin birleşimi değildir: bütün parçaların damgadan küçük kimlikleri `TR_special_tokens: TR_true` ile tek seferde çözülür.
- `ses` 23.6'nın denetimlerinden geçer; log-Mel modelin `num_mel_bins`'iyle kurulur.
- *Test:* TR testi.

**25.3** `TR_language:` zorunludur: varsayılan dil, unutulunca sessizce yanlış dilde metin üretirdi.
- Değer modelin küçük harfli kısa kodudur: `"tr"`, `"en"`, `"yue"`. `<|ad|>` parçası modelin `generation_config.json`'unda yoksa hatadır; `"turkish"` ve `"TR"` de böyledir, ileti küçük harfli kısa kodu önerir.
- Başlangıç dizisi `<|startoftranscript|>`, dil parçası, `<|transcribe|>`'dır.
- **Tek dilli mi çok dilli mi, `generation_config.json` söyler:** `is_multilingual` varsa o; yoksa `lang_to_id`'nin varlığı. `is_multilingual` `true` iken `lang_to_id` yoksa açılışta hatadır. İkisi de yokken `vocab_size` 51865 ya da büyükse (özgünün çok dilli dağarcığı) de açılışta hatadır, ileti `is_multilingual`'ı ve çok dilliyse `lang_to_id`'yi yazmayı ya da modelin güncel `generation_config.json`'unu almayı söyler: çok dilli bir model sessizce tek dilli sayılırdı.
- Tek dilli modelde yalnız `"en"` geçerlidir; öbür kod hatadır, sessizce yutulmaz. Başlangıç dizisine dil ve görev parçası girmez.
- *Test:* TR testi.

**25.4** Çözme açgözlüdür ve tek turdur: aynı model, aynı ses ve aynı dil aynı metni verir (15.22a). Sıcaklık artırılmaz, örnekleme yoktur; bu yüzden tekrara kilitlenen pencere olduğu gibi kalır.
- *Test:* TR testi.

**25.5** Her adımda logitlere süzgeçler bu sırayla uygulanır, sonra en büyük parça seçilir; eşitlikte küçük kimlik seçilir. Metin parçası burada metin sonu dahil damga dışı her kimliktir.
1. Boş başlangıç: ilk seçilen konumda boşluk ve metin sonu parçası `-sonsuz` olur.
2. Bastırma: `generation_config.json`'un `suppress_tokens` listesi `-sonsuz` olur (görev, başlangıç ve sessizlik parçaları listededir). `config.json`'un listesi kullanılmaz.
3. Zaman damgası kuralları, bastırılmış logitlerden:
   - `<|notimestamps|>` `-sonsuz` olur;
   - son parça damgaysa: dizinin ilk parçasıysa ya da ondan önce damga varsa sıradaki damga olamaz; ondan önce metin varsa metin sonundan küçük kimlikler `-sonsuz` olur;
   - damgalar azalmaz; yalnız çiftin ikincisi birincisine eşit olabilir;
   - ilk seçilen konumda metin parçaları ve `<|1.00|>`'in üstündeki damgalar `-sonsuz` olur;
   - damgaların toplam olasılığı en olası metin parçasını geçerse metin parçaları `-sonsuz` olur.
- *Test:* TR testi.

**25.6** Bir pencerede en çok `max_target_positions // 2` adım koşar. Adımlar dolunca ya da dizi `max_target_positions`'ı geçince dizinin sonuna metin sonu parçası eklenir. Önceki parça varsa en çok `max_target_positions // 2 - 1` tanesi damgalarıyla birlikte `<|startofprev|>` ile başa gelir; bu koşullama kapanmaz.
- *Test:* TR testi.

## Uzun ses

**25.7** Ses pencere pencere çözülür; pencere başı özgün `transcribe`'daki gibi ilerler (23.6). Kaydın kendi çerçeveleri `mel.TR_shape[1] - 3000`'dir; tam 30 saniyelik ses kısa ya da uzun diye ayrılmaz.
- Damga pencere başına göredir; `.TR_start` ve `.TR_end` kaydın başına göredir.
- Ardışık iki damga iki kesitin sınırıdır: dizi orada bölünür, her dilim bir kesittir; başı dilimin ilk damgası, sonu son damgasıdır.
- Ardışık damga varsa ve dizi tek damgayla bitiyorsa pencere boyu kadar ilerlenir; bitmiyorsa son ardışık çiftin ilk damgasına dönülür, sonrası atılır.
- Ardışık damga yoksa tek kesit olur, pencere boyu kadar ilerlenir. Kesitin başı pencere başıdır; sonu `<|0.00|>` dışında son damgadır, damga yoksa kaydın pencereye düşen sonudur (kısa son pencerede kaydın sonu, 30 saniye değil).
- Sıfır süreli ya da yalnız boşluktan oluşan kesitin metni ve parçaları silinir, kesit listede kalır: silinen parça sonraki pencerenin bağlamına girmez.
- *Test:* TR testi.

**25.8** Sessiz pencere atlanır: sessizlik olasılığı 0,6'yı geçiyorsa ve ortalama parça log-olasılığı -1,0'ı geçmiyorsa pencerenin kesiti yoktur, pencere boyu kadar ilerlenir.
- Sessizlik olasılığı süzgeçlerden önce, ilk adımda, başlangıç parçasının konumundaki dağılımdan okunur. Sessizlik parçasının kimliği `no_timestamps_token_id − 1`'dir; adı modele göre değişir.
- Ortalama, seçilen parçaların süzgeçten sonraki log-olasılıklarının toplamının metin sonu hariç parça sayısı artı bire bölümüdür.
- Örneği olan sessiz ses hata değildir; eşiklerle atlanırsa metni boştur.
- *Test:* TR testi.

## Sınır

**25.9** Yeni yerleşikler yalnız `TR_conv1d` (15.23c) ve `TR_logsumexp`'tir (15.18); öbürü `.tr`'dir. `TR_conv1d` ve `TR_logsumexp` kartta da çalışır (15.41).
- *Test:* Go: `TestConv1dUnfoldAndFoldMatchCPUBits`, `TestConv1dMatchesCPUWithinMatmulStep`, `TestConv1dRunsOnTheCard`: `TR_conv1d`'in kartta işlemciyle 15.41'in basamaklarıyla aynı sonucu vermesi (gpu ve ml paketleri); `TR_logsumexp`'inkini `TestLogSumExpMatchesCPU` ve `TestLogSumExpRunsOnTheCard` sınar, öbürü 25.1–25.8'in testleridir.

## Konuşma algılama

Örnekler 16 kHz, tek kanal ve `[-1, 1]` aralığındadır; aralık denetlenmez: ölçeklenmemiş örnek hata vermez, olasılık yine gelir. Çerçeve 512 örnektir: 32 ms. Kesit konuşma çerçevelerini ve çevresini tutan 1 boyutlu `TR_Tensor`'dür; Whisper'a `TR_Sound` olarak verilir.

**25.10** `TR_pretrained_vad(yol)` `tr/vad` kitaplığındadır ve bir `TR_VoiceDetector` verir. Yol tek bir safetensors dosyasıdır: `silero_vad_16k.safetensors`. Yeni yerleşik yoktur, model `.tr`'dir.
- Dosyada 15 tensör vardır, adları ve şekilleri sabittir, türleri `float32`'dir.
- Eksik ya da bilinmeyen tensör, şekli ya da türü uymayan tensör hatadır; ileti tensörün adını söyler.
- *Test:* TR testi.

```tr
# TR 4
TR_import "tr/audio"
TR_import "tr/vad"
TR_import "tr/whisper"
TR_var model = TR_pretrained_whisper("whisper-tiny")
TR_var kesici = TR_SpeechSplitter(TR_pretrained_vad("silero_vad_16k.safetensors"))
TR_var ses = TR_read_wav("kayıt.wav").TR_samples
TR_for i TR_in 0..ses.TR_shape[1] // 512 {
    TR_var kesit = kesici.TR_push(ses.TR_slice(1, i * 512, (i + 1) * 512).TR_reshape([512]))
    TR_if kesit != TR_none {
        TR_var parça = TR_Sound(kesit.TR_reshape([1, kesit.TR_shape[0]]), 16000)
        TR_print(model.TR_transcribe(parça, TR_language: "tr").TR_text)
    }
}
```

**25.11** `d.TR_probability(x)` 1 boyutlu, `float32`, 512 öğeli, sonlu bir `TR_Tensor` alır ve konuşma olasılığını `TR_Float` verir.
- Başka girdi hatadır ve durum değişmez: tek NaN çerçeve LSTM'i kalıcı bozar, sonraki bütün olasılıklar NaN olurdu. Örnekler sonlu olup hesap taşarsa da böyledir.
- Yalnız CPU'da çalışır: varsayılan cihaz ne olursa olsun model CPU'dadır, kartta duran örnekler hatadır.
- Durum (LSTM ve son 64 örnek) çağrılar arasında kalır. `d.TR_reset_state()` ikisini sıfırlar: yeni bir akış başlar.
- Girdi türev izliyorsa durum türeve bağlanmaz.
- *Test:* TR testi.

**25.12** `TR_SpeechSplitter(d)` bir kesitleyici verir. Adlı argümanlar, süreler saniye `TR_Float`:
- `TR_start_threshold: 0.5`;
- `TR_end_threshold:` verilmezse `min(başlangıç, max(başlangıç - 0.15, 0.01))`;
- `TR_pre_roll: 0.32`, `TR_silence: 0.6`, `TR_max_speech: 29.0`.

Eşik 0–1 dışında, bitiş eşiği başlangıçtan büyük, süre sıfır, eksi ya da sonlu değilse hatadır. Çerçeveye yuvarlanmış üst sınır baş tamponu artı sessizlikten büyük olmalıdır (25.13): eşit ya da küçükse hatadır, yoksa tetiği açan çerçeve kesiti aynı çağrıda bitirebilirdi.
- *Test:* TR testi.

**25.13** `k.TR_push(x)` 512 örnek alır; bir kesit bittiyse onu `TR_Tensor` olarak, bitmediyse `TR_none` verir. Örnekler 25.11'den geçer; hata varsa kesitleyici de değişmez.
- **Tetik:** `p >= başlangıç` tetiği açar.
- **Konuşmada:** `k.TR_in_speech` salt okunur bir `TR_Bool` özelliktir: tetik açıkken `TR_true`. `p >= başlangıç` çerçevesini alan `k.TR_push(x)`'ten sonra `TR_true` olur, `k.TR_push(x)` o sırada `TR_none` verse de; kesit bitince (25.13, 25.15) ve `k.TR_flush()`'tan sonra `TR_false`'tur. Bölünen kesit kalanla sürerse (25.15) `TR_true` kalır. Sözün başladığı an değerin `TR_false`'tan `TR_true`'ya geçişidir: `TR_var önce = k.TR_in_speech`, `k.TR_push(x)`, sonra `k.TR_in_speech TR_and TR_not önce`. Söz kesme (asistan konuşurken kullanıcının başlaması) kesitleyicinin eşiğini ve baş tamponunu dışarıda yeniden yazmadan böyle bulunur.
- **Sessizlik sayacı:** `p < bitiş` çerçevelerini sayar; iki eşik arası çerçeve saymaz, sıfırlamaz da. Yalnız `p >= başlangıç` sıfırlar. Sayaç sessizliğe varınca kesit biter.
- **Yuvarlama:** sessizlik ve baş tamponu çerçeveye yukarı, üst sınır aşağı yuvarlanır: 0,32 sn 10, 0,6 sn 19, 29 sn 906 çerçeve.
- *Test:* TR testi.

**25.14** Kesitin başına baş tamponu eklenir, bitiş sessizliği kesitte kalır.
- Tetik kapalıyken baş tamponundan eskisi düşer; tetik açıkken örnek düşmez.
- Kesit bitince baş tamponu boşalır: bir örnek iki kesite girmez.
- En kısa konuşma kuralı yoktur: tek çerçevelik konuşma da kesittir.
- Çerçeve kopyalanır: çağıran aynı tensörü yeniden doldurabilir.
- *Test:* TR testi.

**25.15** Üst sınır kesitin bütün boyunu sayar, baş tamponu dahil. `TR_max_speech` 30 saniyeyi geçmezse kesit tek Whisper penceresine sığar.
- Sınıra varınca kesit, ilk konuşma çerçevesinden sonraki en son duraklamanın ortasından bölünür. Duraklama en az 98 ms, yani en az 4 ardışık `p < bitiş` çerçevesidir; ortası baştan `boy // 2` çerçeve sonrasıdır.
- Duraklama yoksa kesit sınırda kesilir.
- Kalanda `p >= başlangıç` çerçevesi varsa kesit kalanla sürer. Yoksa kesit bölünmez: sınırda biter, tetik kapanır.
- Sessizlik sayacının dolduğu çerçeve sınıra denk gelirse kesit bölünmeden biter.
- *Test:* TR testi.

**25.16** `k.TR_flush()` süren kesiti verir; süren kesit yoksa ya da kesitte konuşma çerçevesi yoksa `TR_none`. Sonra kesitleyici boştur, baş tamponu dahil. Dedektör durumu sıfırlanmaz: o `d.TR_reset_state()`'tir.
- *Test:* TR testi.

**25.17** Aynı model ve aynı örnekler aynı kesitleri verir (15.22a): işçi sayısına ve zamana bağlı değildir.
- *Test:* TR testi.
