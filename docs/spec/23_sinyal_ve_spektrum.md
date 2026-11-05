# 23. Sinyal ve spektrum

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "23. Sinyal ve spektrum".

Her kuralın emsali ve gerekçesi: [`rationale/23_signal.md`](rationale/23_signal.md).

Konuşmayı yazıya çeviren modeller sesi önce frekans gösterimine çevirir. Bu bölüm Fourier dönüşümünü, kısa süreli Fourier dönüşümünü (STFT), pencereyi, Mel süzgeçlerini ve Whisper'ın girdisini söyler. Karmaşık sayı cinsi yoktur (15.1): karmaşık bir sonuç `(gerçek, sanal)` demetidir (3.6a), aynı şekilli iki `TR_Tensor[TR_Float]`. Ters dönüşümler (sesi frekans gösteriminden geri kurmak) ses üretimiyle gelir.

## Fourier dönüşümü

**23.1** `TR_rfft(x, TR_n: n)` gerçek bir sinyalin Fourier dönüşümünü son eksen boyunca hesaplar ve `(gerçek, sanal)` verir.
- `x` bir `TR_Tensor[TR_Float]`'dur; baştaki eksenler toplu eksendir. `TR_Int` tensör hatadır; ileti `TR_Tensor[TR_Float](x)`'i önerir (15.49).
- `TR_n:` dönüşümün boyudur; verilmezse son eksenin uzunluğudur. Daha kısa sinyalin sonu sıfırla doldurulur. Daha uzun sinyal hatadır, sessizce kesilmez (15.49); ileti dilimi önerir: `x.TR_slice(-1, 0, n)`.
- Sonucun son ekseni `n // 2 + 1`'dir: gerçek sinyalde negatif frekanslar pozitiflerin eşleniğidir, yazılmaz.
- Ölçek yoktur: sonuç `Σ x[k] · e^(−2πi·j·k/n)` toplamının kendisidir. Çeyrek periyottaki katsayılar (`1`, `−1`, `i`, `−i`) tamdır; yuvarlama payı yalnız öbür açılardadır.
- Hesap ve sonuç float32'dedir; float64 girdi float64'te hesaplanır, 16 bitlik girdi float32'ye açılır.
- Sonuç türev izler.

```tr
# TR 4
TR_var x = TR_Tensor([1.0, 0.0, -1.0, 0.0])
TR_var gerçek, sanal = TR_rfft(x)
TR_print(gerçek.TR_tolist(), sanal.TR_tolist()) # [0.0, 2.0, 0.0] [0.0, 0.0, 0.0]
```

- *Test:* TR testi (örnekteki tam sıfırlar dahil); Go: `TestPlanMatchesTheDefinition`.

## Pencere ve STFT

**23.2** `TR_hann_window(n, TR_periodic: b)` `n` uzunlukta Hann penceresi verir: `k`'inci öğe `0.5 − 0.5 · TR_cos(2πk / N)`'dir; `TR_periodic: TR_true`'da `N = n`, `TR_false`'ta `N = n − 1`.
- `TR_periodic:` zorunludur: PyTorch'un penceresi periyodik, NumPy ve SciPy'ınki simetriktir. STFT'nin ve Whisper'ın penceresi periyodiktir.
- `n` en az 1'dir; `n = 1`'de pencere `[1.0]`'dır.
- Hesap float64'tedir, sonuç float32 bir tensördür ve türev izlemez.
- *Test:* TR testi.

**23.3** `TR_stft(x, n, TR_hop_length: h, TR_window: w, TR_center: b, TR_pad_mode: kip)` kısa süreli Fourier dönüşümüdür.
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

- *Test:* TR testi.

## Mel

**23.4** `TR_mel_filterbank(TR_sample_rate: r, TR_n_fft: n, TR_n_mels: m, TR_norm: biçim, TR_mel_scale: ölçek)` `[m, n // 2 + 1]` şekilli üçgen süzgeçler verir. `süzgeç @ güç` güç izgesini Mel izgesine çevirir.
- `TR_norm:` ve `TR_mel_scale:` zorunludur:
  - `TR_norm:` `"slaney"` (her üçgenin alanı 1 olur: `2 / (f[i + 2] − f[i])` ile çarpılır) ya da `TR_none`;
  - `TR_mel_scale:` `"slaney"` ya da `"htk"`.
  - Başka değer hatadır; ileti geçerli değerleri söyler.
- `TR_f_min:` en düşük frekanstır (varsayılan 0.0), `TR_f_max:` en yüksek frekanstır (varsayılan `r / 2`). `0 ≤ f_min < f_max ≤ r / 2` değilse hata.
- Formül librosa'nın `filters.mel`'idir (0.11.0). Hesap float64'tedir; sonuç float32'ye bir kez yuvarlanır ve türev izlemez.
- Hiçbir frekans kutusuna düşmeyen boş süzgeç hatadır; ileti `n`'yi büyütmeyi ya da `m`'yi küçültmeyi önerir.
- *Test:* TR testi; Go: `TestMelFiltersMatchWhisper`: Whisper'ın `mel_filters.npz`'iyle en büyük fark 1e-8'in altında.

## Belirlenimcilik

**23.5** Bu bölümün işlemleri 15.22a'ya uyar: aynı cihaz türünde aynı girdi ve şekil aynı bitleri verir. Cihaz türleri arasında 15.41'in üçüncü basamağındadır (ondalık indirgeme).
- GPU'da Fourier dönüşümü TR'nin kendi çekirdeğiyle yapılır; sistemin FFT kitaplığı yüklenmez. Sonuç sistemde kurulu bir kitaplığın sürümüne bağlı olmaz. Çekirdek gelene kadar GPU'daki tensörde 23.1 ve 23.3 hata verir, CPU'ya düşmez (15.41).
- Toplu verilen seslerin sonucu tek tek hesaplananla aynı bitleri vermeyebilir: şekil değişir (15.22a "aynı şekil").
- *Test:* Go: `TestStftSameBitsForAnyWorkerCount`, `TestRfftSameBitsForAnyWorkerCount`.

## Whisper'ın girdisi

**23.6** `TR_whisper_log_mel(ses, TR_n_mels: m)` `tr/audio` kitaplığındadır (22.4, 11.6). OpenAI Whisper'ın `transcribe`'ının kullandığı log-Mel'i bir kaydın tamamı için verir: `[m, örnek // 160 + 3000]` şekilli bir `TR_Tensor[TR_Float]`. Kaydın kendi çerçeveleri ilk `örnek // 160`'tır; gerisi 30 saniyelik dolgununkilerdir.
- `ses` bir `TR_Sound`'dur (22.4). Denetim sırası: örnekler iki boyutlu değilse hata (alanlar kurulduktan sonra atanmış olabilir, 7.5); sonra hız; sonra kanal; sonra boş ses.
  - `.TR_sample_rate` 16000 değilse hatadır; ileti yeniden örneklemenin kabuk yazımını önerir: `ffmpeg -nostdin -i girdi -ar 16000 -c:a pcm_s16le çıktı.wav`.
  - Örneği olmayan ses hatadır: dolgu onu sessizce sessizliğe çevirirdi.
  - Birden çok kanal hatadır; sessizce tek kanala indirilmez. İleti iki açık yazımı önerir: ilk kanal `TR_Sound(TR_samples: ses.TR_samples.TR_slice(0, 0, 1), TR_sample_rate: ses.TR_sample_rate)`, ortalama `TR_Sound(TR_samples: TR_mean(ses.TR_samples, TR_dim: 0, TR_keepdim: TR_true), TR_sample_rate: ses.TR_sample_rate)`.
- `TR_n_mels:` zorunludur: modelin `config.json`'undaki `num_mel_bins` (large-v3'te 128, öncekilerde 80).
- **Sıra** özgün uygulamanınkidir:
  1. sinyalin sonuna 480 000 sıfır örnek (30 saniye) eklenir;
  2. `TR_stft(x, 400, TR_hop_length: 160, TR_window: TR_hann_window(400, TR_periodic: TR_true), TR_center: TR_true, TR_pad_mode: "reflect")`; son çerçeve atılır; güç `gerçek ** 2 + sanal ** 2`'dir;
  3. `TR_mel_filterbank(TR_sample_rate: 16000, TR_n_fft: 400, TR_n_mels: m, TR_norm: "slaney", TR_mel_scale: "slaney") @ güç`;
  4. `TR_log10(TR_clamp(mel, TR_min: 1e-10))`; sonra alt sınır: `TR_maximum(x, TR_max(x) − 8.0)`; sonra `(x + 4.0) / 4.0`.
- **Alt sınır** kaydın tamamından alınır, kayıt başınadır. Uzun ses parçalara bölünüp ayrı ayrı verilmez; birden çok kayıt her biri için ayrı çağrılır.
- **Pencereler** ayrı bir iştir, Whisper'ın çeviri döngüsünündür; özgün `transcribe`'ı izler:
  - pencerenin başı sabit 3000'er ilerlemez; çözülen son zaman damgasına göre ilerler;
  - pencere kaydın kendi çerçevelerinin sonunda kesilir; 3000'e eksik kalanı log-Mel'in içinde 0 ile doldurulur (sesin değil).

```tr
# TR 4
TR_import "tr/audio"
TR_var ses = TR_read_wav("soru.wav")
TR_var mel = TR_whisper_log_mel(ses, TR_n_mels: 80)
TR_print(mel.TR_shape) # [80, örnek // 160 + 3000]
```

- *Test:* TR testi (`tests/whisper_girdi_test.tr`): sentetik sınav sesinde özgünden en çok 1e-4 fark.
