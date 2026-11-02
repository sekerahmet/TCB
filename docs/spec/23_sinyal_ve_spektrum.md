# 23. Sinyal ve spektrum

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "23. Sinyal ve spektrum".

Her kuralın emsali ve gerekçesi: [`rationale/23_signal.md`](rationale/23_signal.md).

Konuşmayı yazıya çeviren modeller sesi önce frekans gösterimine çevirir; ses üreten modellerin bir kısmı sesi o gösterimden geri kurar. Bu bölüm Fourier dönüşümünü, kısa süreli Fourier dönüşümünü (STFT), pencereyi, Mel süzgeçlerini ve Whisper'ın girdisini söyler. Karmaşık sayı cinsi yoktur (15.1): karmaşık bir sonuç `(gerçek, sanal)` demetidir (3.6a), aynı şekilli iki `TR_Tensor[TR_Float]`.

## Fourier dönüşümü

**23.1** `TR_rfft(x, TR_n: n)` gerçek bir sinyalin Fourier dönüşümünü son eksen boyunca hesaplar ve `(gerçek, sanal)` verir.
- `x` bir `TR_Tensor[TR_Float]`'dur; baştaki eksenler toplu eksendir. `TR_Int` tensör hatadır; ileti `TR_Tensor[TR_Float](x)`'i önerir (15.49).
- `TR_n:` dönüşümün boyudur; verilmezse son eksenin uzunluğudur. Daha kısa sinyalin sonu sıfırla doldurulur. Daha uzun sinyal hatadır, sessizce kesilmez (15.49); ileti dilimi önerir: `x.TR_slice(-1, 0, n)`.
- Sonucun son ekseni `n // 2 + 1`'dir: gerçek sinyalde negatif frekanslar pozitiflerin eşleniğidir, yazılmaz.
- İleri yön ölçeksizdir; `1/n` ters yöndedir (23.2).
- Hesap ve sonuç float32'dedir; float64 girdi float64'te hesaplanır, 16 bitlik girdi float32'ye açılır.
- Sonuç türev izler.

```tr
# TR 4
TR_var x = TR_Tensor([1.0, 0.0, -1.0, 0.0])
TR_var gerçek, sanal = TR_rfft(x)
TR_print(gerçek.TR_tolist(), sanal.TR_tolist()) # [0.0, 2.0, 0.0] [0.0, 0.0, 0.0]
```

- *Test:* gerekmez, henüz uygulanmadı.

**23.2** `TR_irfft(gerçek, sanal, TR_n: n)` 23.1'in tersidir: son eksende `n` uzunlukta gerçek sinyal verir.
- `TR_n:` zorunludur. `m` uzunluklu tek yanlı spektrum `2(m − 1)` ve `2(m − 1) + 1` uzunlukta iki ayrı sinyalden gelebilir; tahmin edilirse tek uzunluklu sinyal sessizce bir örnek kaybeder.
- `gerçek` ile `sanal` aynı şekilde, son eksenleri `n // 2 + 1` olmalıdır; değilse hata, ileti şekilleri söyler.
- Ölçek `1/n`'dir. Sıfır frekansın ve çift `n`'de en yüksek frekansın sanal kısmı gerçek bir sinyalde sıfırdır; verilen değer kullanılmaz.
- Sonuç türev izler. `TR_irfft(gerçek, sanal, TR_n: n)` 23.1'in sonucundan sinyali float32 yuvarlamasıyla geri verir.
- *Test:* gerekmez, henüz uygulanmadı.

## Pencere ve STFT

**23.3** `TR_hann_window(n, TR_periodic: b)` `n` uzunlukta Hann penceresi verir: `k`'inci öğe `0.5 − 0.5 · TR_cos(2πk / N)`'dir; `TR_periodic: TR_true`'da `N = n`, `TR_false`'ta `N = n − 1`.
- `TR_periodic:` zorunludur: PyTorch'un penceresi periyodik, NumPy ve SciPy'ınki simetriktir. STFT'nin ve Whisper'ın penceresi periyodiktir.
- `n` en az 1'dir; `n = 1`'de pencere `[1.0]`'dır.
- Hesap float64'tedir, sonuç float32 bir tensördür ve türev izlemez.
- *Test:* gerekmez, henüz uygulanmadı.

**23.4** `TR_stft(x, n, TR_hop_length: h, TR_window: w, TR_center: b, TR_pad_mode: kip)` kısa süreli Fourier dönüşümüdür.
- Son eksendeki sinyal `n` uzunluklu, `h` adımlı çerçevelere bölünür; her çerçeve pencereyle çarpılıp 23.1 ile dönüştürülür. Sonuç `(gerçek, sanal)`'dır; ikisinin de şekli `[..., n // 2 + 1, çerçeve]`'dir. Baştaki eksenler toplu eksendir: `TR_Sound`'un `[kanal, örnek]`'i (22.4) `[kanal, n // 2 + 1, çerçeve]` olur.
- `TR_hop_length:` ve `TR_window:` zorunludur; ekosistemlerin varsayılanları birbirini tutmaz.
- `w` `n` uzunlukta tek boyutlu bir `TR_Tensor[TR_Float]`'dur; başka uzunluk hatadır.
- **Ortalama:** `TR_center: TR_true` (varsayılan) sinyalin iki ucuna `n // 2` örnek dolgu ekler; `t`'inci çerçeve sinyalin `t·h`'inci örneğine ortalanır.
  - `TR_pad_mode:` o zaman zorunludur: `"reflect"` (yansıtma; PyTorch, Whisper) ya da `"constant"` (sıfır; librosa). Başka metin hatadır, ileti ikisini söyler.
  - `"reflect"`'te sinyal `n // 2`'den uzun olmalıdır; değilse hata, ileti iki uzunluğu söyler.
  - `TR_center: TR_false` dolgu eklemez; `TR_pad_mode:` o zaman verilirse hatadır.
- **Çerçeve sayısı** `1 + (L − n) // h`'dir, `L` dolgulu uzunluktur. `L < n` hatadır. Çerçeveye sığmayan son `(L − n) % h` örnek hiçbir çerçeveye girmez; tanımı budur, sonuç bunu şekliyle gösterir.
- Ölçek yoktur. Sonuç türev izler.

```tr
# TR 4
TR_import "tr/audio"
TR_var x = TR_read_wav("soru.wav").TR_samples[0] # şekli [örnek]
TR_var pencere = TR_hann_window(400, TR_periodic: TR_true)
TR_var gerçek, sanal = TR_stft(x, 400, TR_hop_length: 160, TR_window: pencere, TR_center: TR_true, TR_pad_mode: "reflect")
TR_var güç = gerçek ** 2 + sanal ** 2 # şekli [201, çerçeve]
```

- *Test:* gerekmez, henüz uygulanmadı.

**23.5** `TR_istft(gerçek, sanal, n, TR_hop_length: h, TR_window: w, TR_center: b, TR_length: L)` 23.4'ün tersidir.
- Her çerçeve 23.2 ile geri çevrilir, pencereyle çarpılır ve üst üste eklenir. Her örnek, pencere karelerinin o örnekteki toplamına bölünür. `TR_center: TR_true`'da iki uçtaki `n // 2` örnek atılır.
- Argümanlar 23.4'teki gibidir; `TR_pad_mode:` yoktur.
- `TR_length:` zorunludur: çıktının örnek sayısıdır. Çerçeve sayısından tahmin edilemez (23.2'deki gibi). Çerçevelerin kurduğu örnek `L`'den fazlaysa sonu atılır: 23.4'ün sığmayan son örnekleri oradadır. Azsa hatadır; eksik örnek sıfırla uydurulmaz, ileti iki sayıyı söyler.
- Bir örnekte pencere karelerinin toplamı 1e-11'den küçükse hatadır: o örnek geri kurulamaz. İleti örneğin indeksini söyler; bölme sessizce sonsuz vermez.
- Sonuç türev izler.
- Ses üreten modeller sesi genlik ve fazdan kurar:

```tr
# TR 4
TR_var genlik = TR_ones([11, 9])
TR_var faz = TR_zeros([11, 9])
TR_var pencere = TR_hann_window(20, TR_periodic: TR_true)
TR_var ses = TR_istft(genlik * TR_cos(faz), genlik * TR_sin(faz), 20, TR_hop_length: 5, TR_window: pencere, TR_center: TR_true, TR_length: 40)
```

- *Test:* gerekmez, henüz uygulanmadı; uygulaması ses üretimiyle gelir.

## Mel

**23.6** `TR_mel_filterbank(TR_sample_rate: r, TR_n_fft: n, TR_n_mels: m, TR_norm: biçim, TR_mel_scale: ölçek)` `[m, n // 2 + 1]` şekilli üçgen süzgeçler verir. `süzgeç @ güç` güç izgesini Mel izgesine çevirir.
- `TR_norm:` ve `TR_mel_scale:` zorunludur:
  - `TR_norm:` `"slaney"` (her üçgenin alanı 1 olur: `2 / (f[i + 2] − f[i])` ile çarpılır) ya da `TR_none`;
  - `TR_mel_scale:` `"slaney"` ya da `"htk"`.
  - Başka değer hatadır; ileti geçerli değerleri söyler.
- `TR_f_min:` en düşük frekanstır (varsayılan 0.0), `TR_f_max:` en yüksek frekanstır (varsayılan `r / 2`). `0 ≤ f_min < f_max ≤ r / 2` değilse hata.
- Formül librosa'nın `filters.mel`'idir (0.11.0). Hesap float64'tedir; sonuç float32'ye bir kez yuvarlanır ve türev izlemez.
- Hiçbir frekans kutusuna düşmeyen boş süzgeç hatadır; ileti `n`'yi büyütmeyi ya da `m`'yi küçültmeyi önerir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: Whisper'ın `mel_filters.npz`'iyle en büyük farkı ölçülür ve sınırı testte yazılır.

## Belirlenimcilik

**23.7** Bu bölümün işlemleri 15.22a'ya uyar: aynı cihaz türünde aynı girdi ve şekil aynı bitleri verir. Cihaz türleri arasında 15.41'in üçüncü basamağındadır (ondalık indirgeme).
- GPU'da Fourier dönüşümü TR'nin kendi çekirdeğiyle yapılır; sistemin FFT kitaplığı yüklenmez. Sonuç sistemde kurulu bir kitaplığın sürümüne bağlı olmaz.
- Toplu verilen seslerin sonucu tek tek hesaplananla aynı bitleri vermeyebilir: şekil değişir (15.22a "aynı şekil").
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: paralel ve GPU çekirdeği seri hesapla karşılaştırılır.

## Whisper'ın girdisi

**23.8** `TR_whisper_log_mel(ses, TR_n_mels: m)` `tr/audio` kitaplığındadır (22.4, 11.6). OpenAI Whisper'ın `transcribe`'ının kullandığı log-Mel'i verir: `[m, örnek // 160 + 3000]` şekilli bir `TR_Tensor[TR_Float]`.
- `ses` bir `TR_Sound`'dur (22.4).
  - `.TR_sample_rate` 16000 değilse hatadır; ileti yeniden örneklemeyi önerir: `TR_run_process("ffmpeg", ["-nostdin", "-i", girdi, "-ar", "16000", "-c:a", "pcm_s16le", çıktı])` (21.1).
  - Birden çok kanal hatadır; sessizce tek kanala indirilmez. İleti iki açık yazımı önerir: ilk kanal `TR_Sound(TR_samples: ses.TR_samples.TR_slice(0, 0, 1), TR_sample_rate: 16000)`, ortalama `TR_Sound(TR_samples: TR_mean(ses.TR_samples, TR_dim: 0, TR_keepdim: TR_true), TR_sample_rate: 16000)`.
- `TR_n_mels:` zorunludur: modelin `config.json`'undaki `num_mel_bins` (large-v3'te 128, öncekilerde 80).
- **Sıra** özgün uygulamanınkidir:
  1. sinyalin sonuna 480 000 sıfır örnek (30 saniye) eklenir;
  2. `TR_stft(x, 400, TR_hop_length: 160, TR_window: TR_hann_window(400, TR_periodic: TR_true), TR_center: TR_true, TR_pad_mode: "reflect")`; son çerçeve atılır; güç `gerçek ** 2 + sanal ** 2`'dir;
  3. `TR_mel_filterbank(TR_sample_rate: 16000, TR_n_fft: 400, TR_n_mels: m, TR_norm: "slaney", TR_mel_scale: "slaney") @ güç`;
  4. `TR_log10(TR_clamp(mel, TR_min: 1e-10))`; sonra alt sınır: `TR_maximum(x, TR_max(x) − 8.0)`; sonra `(x + 4.0) / 4.0`.
- **Uzun ses:** alt sınır bütün kayıttan alınır. Uzun ses parçalara bölünmeden bir kez verilir; çeviri 3000 çerçevelik pencereleri sonuçtan keser (`.TR_slice(1, baş, baş + 3000)`). Parçaları ayrı ayrı vermek özgün uygulamadan ayrılır.

```tr
# TR 4
TR_import "tr/audio"
TR_var ses = TR_read_wav("soru.wav")
TR_var mel = TR_whisper_log_mel(ses, TR_n_mels: 80)
TR_var ilk_pencere = mel.TR_slice(1, 0, 3000) # modele giden [80, 3000]
```

- *Test:* gerekmez, henüz uygulanmadı. Uygulanınca TR testi: sabit bir sınav sesinde (`jfk.wav`) özgün uygulamanın log-Mel'iyle `TR_all_close(..., TR_rtol: 1e-4, TR_atol: 1e-4)`. Aynı cihaz türünde açgözlü çözmenin parçaları özgün uygulamanınkiyle birebir aynıdır; bu bir doğruluk testidir, cihazlar arası bir söz değildir (15.22a).
