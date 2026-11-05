# 25. Konuşmayı yazıya çevirme

Bu bölüm yayımlanmış bir Whisper modelini açmayı ve bir sesi yazıya çevirmeyi yazar. Girdi 22.4'ün `TR_Sound`'u, log-Mel 23.6'da yazılıdır. Ölçüt özgün `transcribe`'dır: `temperature=0.0`, `beam_size=None`, `fp16=False`.

Emsal ve gerekçe: [`rationale/25_speech.md`](rationale/25_speech.md).

## Model

**25.1** `TR_pretrained_whisper(yol)` `tr/whisper` kitaplığındadır ve bir `TR_Whisper` verir. Yol Hugging Face biçiminde bir klasördür.
- Dosyaları `model.safetensors`, `config.json`, `tokenizer.json`, `generation_config.json`'dur. Biri ya da gerekli bir alan yoksa hatadır; ileti eksiği söyler.
- `config.json`'un `model_type`'ı `whisper` değilse hatadır; ileti `TR_pretrained_model`'i önerir. `TR_pretrained_model`'e Whisper klasörü vermek de hatadır; ileti bu adı önerir (15.55).
- Çıkış katmanı gömmeye bağlıdır: dosyada ayrı ağırlık aranmaz.
- 16 bitlik ağırlık açılırken `float32`'ye genişletilir; hesap `float32`'dedir. Nicemli ağırlık yoktur.
- `num_mel_bins` `config.json`'dan okunur; 80 ve 128 dışındaki değer hatadır.
- Bilinmeyen ya da eksik tensör hatadır; ileti tensörün adını söyler.
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
- *Test:* gerekmez, henüz uygulanmadı.

**25.3** `TR_language:` zorunludur: varsayılan dil, unutulunca sessizce yanlış dilde metin üretirdi.
- Değer modelin küçük harfli kısa kodudur: `"tr"`, `"en"`, `"yue"`. `<|ad|>` parçası modelin `generation_config.json`'unda yoksa hatadır; `"turkish"` ve `"TR"` de böyledir, ileti küçük harfli kısa kodu önerir.
- Başlangıç dizisi `<|startoftranscript|>`, dil parçası, `<|transcribe|>`'dır.
- Tek dilli modelde yalnız `"en"` geçerlidir; öbür kod hatadır, sessizce yutulmaz. Başlangıç dizisine dil ve görev parçası girmez.
- *Test:* gerekmez, henüz uygulanmadı.

**25.4** Çözme açgözlüdür ve tek turdur: aynı model, aynı ses ve aynı dil aynı metni verir (15.22a). Sıcaklık artırılmaz, örnekleme yoktur; bu yüzden tekrara kilitlenen pencere olduğu gibi kalır.
- *Test:* gerekmez, henüz uygulanmadı.

**25.5** Her adımda logitlere süzgeçler bu sırayla uygulanır, sonra en büyük parça seçilir; eşitlikte küçük kimlik seçilir. Metin parçası burada metin sonu dahil damga dışı her kimliktir.
1. Boş başlangıç: ilk seçilen konumda boşluk ve metin sonu parçası `-sonsuz` olur.
2. Bastırma: `generation_config.json`'un `suppress_tokens` listesi `-sonsuz` olur (görev, başlangıç ve sessizlik parçaları listededir). `config.json`'un listesi kullanılmaz.
3. Zaman damgası kuralları, bastırılmış logitlerden:
   - `<|notimestamps|>` `-sonsuz` olur;
   - son parça damgaysa: dizinin ilk parçasıysa ya da ondan önce damga varsa sıradaki damga olamaz; ondan önce metin varsa metin sonundan küçük kimlikler `-sonsuz` olur;
   - damgalar azalmaz; yalnız çiftin ikincisi birincisine eşit olabilir;
   - ilk seçilen konumda metin parçaları ve `<|1.00|>`'in üstündeki damgalar `-sonsuz` olur;
   - damgaların toplam olasılığı en olası metin parçasını geçerse metin parçaları `-sonsuz` olur.
- *Test:* gerekmez, henüz uygulanmadı.

**25.6** Bir pencerede en çok `max_target_positions // 2` adım koşar. Adımlar dolunca ya da dizi `max_target_positions`'ı geçince dizinin sonuna metin sonu parçası eklenir. Önceki parça varsa en çok `max_target_positions // 2 - 1` tanesi damgalarıyla birlikte `<|startofprev|>` ile başa gelir; bu koşullama kapanmaz.
- *Test:* gerekmez, henüz uygulanmadı.

## Uzun ses

**25.7** Ses pencere pencere çözülür; pencere başı özgün `transcribe`'daki gibi ilerler (23.6). Kaydın kendi çerçeveleri `mel.TR_shape[1] - 3000`'dir; tam 30 saniyelik ses kısa ya da uzun diye ayrılmaz.
- Damga pencere başına göredir; `.TR_start` ve `.TR_end` kaydın başına göredir.
- Ardışık iki damga iki kesitin sınırıdır: dizi orada bölünür, her dilim bir kesittir; başı dilimin ilk damgası, sonu son damgasıdır.
- Ardışık damga varsa ve dizi tek damgayla bitiyorsa pencere boyu kadar ilerlenir; bitmiyorsa son ardışık çiftin ilk damgasına dönülür, sonrası atılır.
- Ardışık damga yoksa tek kesit olur, pencere boyu kadar ilerlenir. Kesitin başı pencere başıdır; sonu `<|0.00|>` dışında son damgadır, damga yoksa pencere sonudur.
- Sıfır süreli ya da yalnız boşluktan oluşan kesitin metni ve parçaları silinir, kesit listede kalır: silinen parça sonraki pencerenin bağlamına girmez.
- *Test:* gerekmez, henüz uygulanmadı.

**25.8** Sessiz pencere atlanır: sessizlik olasılığı 0,6'yı geçiyorsa ve ortalama parça log-olasılığı -1,0'ı geçmiyorsa pencerenin kesiti yoktur, pencere boyu kadar ilerlenir.
- Sessizlik olasılığı süzgeçlerden önce, ilk adımda, başlangıç parçasının konumundaki dağılımdan okunur. Sessizlik parçasının kimliği `no_timestamps_token_id − 1`'dir; adı modele göre değişir.
- Ortalama, seçilen parçaların süzgeçten sonraki log-olasılıklarının toplamının metin sonu hariç parça sayısı artı bire bölümüdür.
- Örneği olan sessiz ses hata değildir; eşiklerle atlanırsa metni boştur.
- *Test:* gerekmez, henüz uygulanmadı.

## Sınır

**25.9** İlk uygulama CPU'dadır; kartta 15.41'in hatasını verir. Yeni yerleşikler yalnız `TR_conv1d` (15.23c) ve `TR_logsumexp`'tir (15.18); öbürü `.tr`'dir.
- *Test:* gerekmez, henüz uygulanmadı.
