# Whisper log-Mel başvuru dosyaları

`tests/whisper_girdi_test.tr` ve `tests/whisper_model_test.tr` bu dosyaları okur (23.6, 25.1).

- **`sentetik.wav`:** bu depo için üretildi: 16 kHz, tek kanal, 16 bit, 8000 örnek. İki sinüs ve az gürültü, `numpy.random.default_rng(1)`. Kişi sesi değildir, kaydedilmiş bir eserden gelmez.
- **`whisper_log_mel_80.bin`, `whisper_log_mel_128.bin`:** bu sesten OpenAI Whisper `v20250625`'in özgün `log_mel_spectrogram(ses, n_mels=m, padding=N_SAMPLES)` çağrısıyla hesaplandı. Küçük endian float32, `[m, 57]` satır düzeni: ilk 56 çerçeve ve son çerçeve. Ses 50 çerçevedir; 56 çerçeve sesi ve dolguya geçişi, son çerçeve dolgunun sonunu kapsar.
- **`kodlayici_cikti.bin`:** küçük sentetik Whisper'ın kodlayıcı çıktısı, `[1, 6, 8]` `float32`. Ağırlıklar yayımlanmış bir modelden gelmez: `tests/whisper_model_test.tr`'nin `değerler` formülüyle, `ölçek` ölçeğiyle, `adlar` sırasıyla kuruldu. Hugging Face `transformers` 5.17.0'ın `WhisperForConditionalGeneration`'ı, PyTorch 2.14.0 ile `whisper_log_mel_80.bin`'in ilk 12 çerçevesinden hesapladı.
- **`cozucu_logit.bin`:** aynı modelin `[1, 5, 3, 7]` parçaları için logitleri, `[1, 4, 12]` `float32`; aynı yazılım, aynı girdi.

**Lisans:** ses ve değerler bu deponun lisansındadır. Değerleri hesaplayan yazılım Whisper'dır (<https://github.com/openai/whisper>, MIT; metni: `internal/stdlib/ml/testdata/mel_filters.KAYNAK.md`). `kodlayici_cikti.bin` ile `cozucu_logit.bin`'i hesaplayan yazılım `transformers` (<https://github.com/huggingface/transformers>, Apache-2.0) ve PyTorch'tur (<https://github.com/pytorch/pytorch>, BSD-3-Clause).
