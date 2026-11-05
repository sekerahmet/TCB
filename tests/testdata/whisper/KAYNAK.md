# Whisper log-Mel başvuru dosyaları

`tests/whisper_girdi_test.tr` bu dosyaları okur (23.6).

- **`sentetik.wav`:** bu depo için üretildi: 16 kHz, tek kanal, 16 bit, 8000 örnek. İki sinüs ve az gürültü, `numpy.random.default_rng(1)`. Kişi sesi değildir, kaydedilmiş bir eserden gelmez.
- **`whisper_log_mel_80.bin`, `whisper_log_mel_128.bin`:** bu sesten OpenAI Whisper `v20250625`'in özgün `log_mel_spectrogram(ses, n_mels=m, padding=N_SAMPLES)` çağrısıyla hesaplandı. Küçük endian float32, `[m, 57]` satır düzeni: ilk 56 çerçeve ve son çerçeve. Ses 50 çerçevedir; 56 çerçeve sesi ve dolguya geçişi, son çerçeve dolgunun sonunu kapsar.

**Lisans:** ses ve değerler bu deponun lisansındadır. Değerleri hesaplayan yazılım Whisper'dır (<https://github.com/openai/whisper>, MIT; metni: `internal/stdlib/ml/testdata/mel_filters.KAYNAK.md`).
