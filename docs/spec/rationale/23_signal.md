# 23. Sinyal ve spektrum: emsal ve gerekçe

Kurallar: [`23_sinyal_ve_spektrum.md`](../23_sinyal_ve_spektrum.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

Araştırma: Gemini Jarvis B (2026-11-02, taban `b2892d87`). Raporu DENETİM doğruladı (istek 42), kararları ikinci kez okudu (istek 43). Kaynak etiketleri 21'in gerekçe dosyasındaki gibidir: **[DENETİM açtı]** ve **[rapordan]**.

Karşılaştırma:

| Konu | PyTorch 2.6 | librosa 0.11 / SciPy 1.15 | transformers v4.51.3 | whisper.cpp 1.7.4 / candle 0.8.4 | TR |
| :-- | :-- | :-- | :-- | :-- | :-- |
| Karmaşık sonuç | `complex64`; `return_complex=False` kullanımdan kalktı | `complex64`/`complex128` | `complex64` ya da NumPy `complex128` | düz dizide `[re, im]` çiftleri | `(gerçek, sanal)` demeti (23.1) |
| STFT adımı | `n_fft // 4`; torchaudio `MelSpectrogram` `win_length // 2` | `n_fft // 4`; SciPy `nperseg // 2` | Whisper'da açık 160 | açık | zorunlu (23.3) |
| Pencere | yoksa dikdörtgen; `hann_window` periyodik | Hann; `np.hanning` ve SciPy `hann` simetrik | Whisper'da açık | açık | zorunlu; `TR_periodic:` zorunlu (23.2) |
| Dolgu | `center=True`, `"reflect"` | `center=True`, `"constant"`; SciPy `boundary="zeros"` | Whisper'da `"reflect"` | whisper.cpp `reflect`; candle dolgusuz | `TR_center: TR_true`'da `TR_pad_mode:` zorunlu (23.3) |
| Mel ölçeği, normu | torchaudio `"htk"`, `norm=None` | `htk=False` (Slaney), `norm="slaney"` | `mel_filter_bank` `"htk"`, `None`; Whisper'da açık `"slaney"` | dosyadan okunur | ikisi de zorunlu (23.4) |
| GPU'da FFT | cuFFT | yok | PyTorch'unki | yok | kendi çekirdeği (23.5) |

**Ters dönüşümler yok.** `irfft` ve `istft` yalnız ses üretiminde gerekir; Whisper'ın yolu kullanmaz [DENETİM açtı: whisper `audio.py`, v20250625, yalnız `torch.stft`]. Bugün kullanan bir yer olmadığı için ad olarak eklenmedi; yol haritasında ses üretiminin notudur (DENETİM istek 43, Ş4). Emsalleri ve tuzakları o adım için burada kalır:
- NumPy ve PyTorch'ta `irfft`'in `n`'si verilmezse `2(m − 1)`'dir; tek uzunluklu sinyal geri çevrilince bir örnek kısalır. TR'de uzunluk zorunlu olmalı [rapordan].
- `torch.istft` üst üste ekler ve pencere karelerinin toplamına böler; toplam sıfıra yakınsa (NOLA koşulu) hata verir [rapordan].
- StyleTTS2 ve Kokoro (ISTFTNet) sesi `torch.istft` ile genlik ve fazdan kurar; Kokoro'nun ONNX yolu `conv_transpose1d` ile yazılmış ve sıfır ile en yüksek frekansın tekilliğini atlıyor [rapordan: `istftnet.py`, `custom_stft.py`]. Demetle girdi `(genlik · TR_cos(faz), genlik · TR_sin(faz))` olur.

### 23.1

- *Emsal:* PyTorch `torch.fft.rfft` ve NumPy `np.fft.rfft` tek yanlı spektrum verir; varsayılan ölçekte ileri yön ölçeksizdir [rapordan]. NumPy'de uzun girdi `n`'ye sessizce kesilir [rapordan]. PyTorch karmaşık cins kullanır; eski `[..., 2]` gerçek görünümü kullanımdan kalktı, çünkü son eksendeki 2 öbür eksenlerle karışıyordu [rapordan: pytorch v2.6.0, `functional.py`].
- *Gerekçe:* karmaşık cins 15.1'in üç cinsini, bütün işleç tablosunu, türevi, çekirdekleri ve `.safetensors`'ı genişletir; demet bunların hiçbirini istemez ve 15'in gerekçesinde zaten yazılı [kanıtlı: `rationale/15_tensors.md:22`]. Güç, genlik ve faz var olan işlemlerle türevli yazılır. Demet `[..., 2]`'nin eksen karışıklığını da taşımaz: `gerçek` ve `sanal` aynı şekilli iki tensördür. Uzun girdinin sessizce kesilmesi 15.49'a aykırıdır.

### 23.2

- *Emsal:* `torch.hann_window(N)` varsayılanıyla periyodiktir; `np.hanning(N)` ve `scipy.signal.windows.hann(N)` simetriktir [rapordan: transformers v4.51.3 `audio_utils.py`]. Whisper `torch.hann_window(N_FFT)` kullanır [DENETİM açtı: whisper `audio.py`, v20250625].
- *Gerekçe:* iki ekosistemin varsayılanı zıttır; NumPy'den gelen ajan Whisper'ın penceresini sessizce yanlış kurar. Zorunlu argüman bunu denetime çevirir (emsal 15.17'nin `TR_approximate:`'i) [kanıtlı: `rationale/15_tensors.md:193`].

### 23.3

- *Emsal:* `torch.stft` `center=True`, `pad_mode="reflect"` [DENETİM açtı: PyTorch `torch/functional.py`]; `librosa.stft` `pad_mode="constant"` [DENETİM açtı: librosa `core/spectrum.py`, `stft` imzası]; `scipy.signal.stft` `boundary="zeros"` ve pencere toplamına bölen `scaling="spectrum"` [rapordan]. Whisper `torch.stft`'yi `pad_mode`'suz, yani `reflect` ile çağırır [DENETİM açtı: `audio.py`]. Adım PyTorch'ta ve librosa'da `n_fft // 4`, torchaudio'nun `MelSpectrogram`'ında `win_length // 2`'dir [rapordan]. VITS `center=False` verip yansıtmayı elle yapar [rapordan]. candle 0.8.4'ün Whisper'ı yansıtmayı yapmaz ve negatif frekansı pozitife ekler; özgün uygulamadan ayrılır [rapordan, `[görüş]`].
- *Gerekçe:* her varsayılan bir ekosistemden gelen ajana sessiz yanlış bırakır: torchaudio alışkanlığıyla Whisper'ın adımı 200 olur, librosa alışkanlığıyla dolgu sıfır olur. Zorunlu olanlar ekosistemlerin birbirini tutmadığı argümanlardır; `TR_center:`'ın varsayılanı PyTorch ve librosa'da aynıdır, o yüzden varsayılanı vardır. `TR_pad_mode:`'un zorunlu olması DENETİM'in eki (istek 42, Konu 3, öneri 4). Kısa pencere (`win_length < n_fft`) yoktur: Whisper kullanmaz, bugün kullanan bir yer yok. Baştaki eksenlerin toplu eksen olması `TR_Sound`'un `[kanal, örnek]`'ine uyar (DENETİM istek 42, §4).
- Çerçeveye sığmayan son örnekler: bu kesme STFT'nin tanımıdır, PyTorch'ta ve librosa'da aynıdır, ve sonuç şekliyle görünür; 15.49'un yasakladığı sessiz uzunluk ya da tür değişimi değildir. `TR_center: TR_true`'da en çok `h − 1` örnek dışarıda kalır.

### 23.4

- *Emsal:* librosa `filters.mel` `htk=False`, `norm="slaney"`, alan normu `2.0 / (mel_f[2 : n_mels + 2] - mel_f[:n_mels])` [DENETİM açtı: librosa 0.11.0 `filters.py`]. torchaudio ve transformers'ın `mel_filter_bank`'i varsayılanla `norm=None`, `mel_scale="htk"` verir [rapordan]. Whisper süzgeçleri `mel_filters.npz`'den okur; dosya `librosa.filters.mel(sr=16000, n_fft=400, n_mels=80)` ile üretilmiş [DENETİM açtı: whisper `audio.py`]. librosa boş süzgeçte uyarır [hipotez: bu tur açılmadı].
- *Gerekçe:* torchaudio'dan gelen ajan Whisper için HTK'lı ve normsuz süzgeç alır ve çıktı bozulur; iki argümanın zorunlu olması bunu denetime çevirir (emsal 15.17). `.npz` okumak yerine hesaplamak bir zip okuyucusu istemez ve 128 Mel'i de verir; `mel_filters.npz` ile farkın "1 ulp" olduğu ölçülmedi [rapordan, `[görüş]`], test ölçer (DENETİM istek 42, öneri 2, koşul 4). Boş süzgeç sessizce sıfır bir Mel kanalı verirdi (15.49).

### 23.5

- *Emsal:* cuFFT 13.4: "Results produced by the cuFFT library are deterministic (ie, bitwise reproducible)"; koşulları plan parametreleri, cuFFT sürümü ve GPU modelidir. Toplu kipte farklı sinyaller birlikte işlenebilir [DENETİM açtı: docs.nvidia.com/cuda/cufft, v13.4, §2.13]. whisper.cpp çerçeveleri iş parçacıklarına böler, her çerçevenin FFT'si bağımsızdır [rapordan].
- *Gerekçe:* 15.22a'nın sözü kitaplık sürümünü anmıyor [kanıtlı: `15_tensorler.md:312`]. Sistemden yüklenen cuFFT ile aynı karttaki iki makine farklı cuFFT sürümüyle farklı bitler verebilir. Kendi çekirdeğimizle söz TR'nin elinde kalır. 2026-11-02'de TR cuFFT yüklemiyor [ölçüldü: DENETİM istek 42, `git grep -i cufft`]. cuFFT ileride hız için gerekirse sürümü kontrol noktası kaydına (20) yazılır ve 15.22a'ya "aynı kitaplık sürümü" koşulu eklenir; o zamana kadar yoktur. Toplu ile tek sonucun ayrılabilmesi 15.22a'nın "aynı şekil" koşulunun sonucudur; Mel çarpımı (`@`) da şekle bağlıdır. Çekirdek gelene kadar GPU'da hata vermesi 15.41'in "sessizce CPU'ya düşmez" kuralıdır (DENETİM istek 43, §4).

### 23.6

- *Emsal:* OpenAI Whisper `v20250625`: `N_FFT = 400`, `HOP_LENGTH = 160`, `stft[..., :-1].abs() ** 2`, `clamp(min=1e-10).log10()`, `torch.maximum(log_spec, log_spec.max() - 8.0)`, `(log_spec + 4.0) / 4.0` [DENETİM açtı: `audio.py`]. `transcribe` bütün sesin log-Mel'ini `padding=N_SAMPLES` ile bir kez hesaplar [DENETİM açtı: `transcribe.py:139`]. Pencerenin başı (`seek`) çözülen son zaman damgasına, kelime zamanlarına ya da bütün pencereye göre ilerler; son pencere kaydın kendi çerçevelerinin sonunda kesilir ve `pad_or_trim` ile log-Mel uzayında 0'la doldurulur [DENETİM açtı: `transcribe.py:140`, `:283`, `:286`, `:371`, `:377`, `:399`, `:416–435`; istek 43, Ş3]. Hugging Face `WhisperFeatureExtractor` sesi STFT'den önce 30 saniyeye doldurur ya da kırpar ve özgün uygulamayla `1e-5` toleransını belgeler [rapordan: transformers v4.51.3]; toplu yolunda alt sınırı kayıt başına alır [DENETİM açtı: transformers v5.17.0 `feature_extraction_whisper.py:161–162`]. transformers'ın testleri log-Mel'i `rtol=1e-4, atol=1e-4` ile sınar [DENETİM açtı: v5.17.0 `test_feature_extraction_whisper.py:336`, `:399`]; açgözlü çıktıyı birebir sınar [rapordan]. `num_mel_bins` large-v3'te 128'dir [rapordan].
- *Gerekçe:* 2. kilometre taşının ölçütü "özgün uygulamayla aynı sonuç"tur; bu yüzden sıra Hugging Face'inki değil OpenAI'ınkidir. İki uygulama uzun seste ayrılır: HF 30 saniyeyi aşanı keser, özgün uygulama alt sınırı bütün kayıttan alır (DENETİM istek 42, §5a). Ajanın en sık iki yanlışı [rapordan, `[görüş]`]: son çerçeveyi atmamak (şekil 3001 olur) ve kısa sesi STFT'den sonra doldurmak (sessiz bölge `0.0`, yani yüksek enerji olur). Fonksiyon ikisini de içinde yapar.
- Pencereler log-Mel fonksiyonunun işi değildir, çünkü başları çözmenin sonucuna bağlıdır; sabit 3000'lik kesim uzun seste özgün uygulamadan ayrılır.
- 16 kHz ve tek kanal: yeniden örnekleme kitaplıkta yok (22.4); kanalı sessizce seçmek ya da ortalamak bir kararı ajandan saklar. İleti iki yazımı da verir; ikisi `TR_Sound`'un kurulmasıyla çalışır (22.4).
- `TR_n_mels:` zorunludur, çünkü yanlış varsayılan (80) large-v3'te şekil hatasına, başka bir modelde sessiz yanlışa dönüşür.
- Eşitlik ölçütü iki ayrı sözdür. Özgün uygulamayla karşılaştırmada `1e-4` transformers'ın sınavıdır; dış bir uygulamayla eşitliğin ölçüsüdür. 15.41 ise TR'nin kendi cihazları arasındaki farkı söyler; ikisi karıştırılmaz (DENETİM istek 43, K7). Açgözlü parçaların birebir eşitliği yalnız aynı cihaz türünde bir doğruluk testidir; cihazlar arasında logitler eşite yakınken seçilen parça değişebilir (DENETİM istek 42, öneri 5).
