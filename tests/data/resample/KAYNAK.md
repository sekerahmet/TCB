# Yeniden örnekleme başvurusu

`lib/audio_test.tr` ("22.4 TR_resample soxr HQ'ya yakındır") bu dosyayı okur (dil tanımı 22.4).

- **`resample.safetensors`:** bu depo için üretildi, `generate.py` ile: `numpy.random.default_rng(2204)` ile 0,1 genlikli gürültü (`noise_…`) ve aynı gürültünün küçük hızın yarısının 0,9 katının altı (`band_…`), 0,25 saniye, 16 bit tam sayı; `soxr_…` aynı girdinin `soxr.resample(quality="HQ")` sonucu (float64 girdi, float32 sonuç). Adların sonu hızlardır: `noise_24000_16000`. Kişi verisi ve yayımlanmış ses içermez.
- **Üretim:** `python generate.py tests/data/resample`; numpy 1.26.4, soxr 1.1.0, safetensors. sha256 `5852f82f6d4ad1a9ac8a41e4f6b4982edaca57311721df7b99b23d9cd3fee310`.
- **Ölçülen fark** (iki ucun 0,05 saniyesi dışında, en büyük mutlak fark; 2026-11-25): gürültüde 24→16 kHz 4,8e-4, 24→32 6,7e-4, 44,1→24 4,2e-4, 16→24 5,6e-4; bant sınırlıda 1,5e-7, 2,1e-7, 3,4e-5, 1,8e-7. Testin eşikleri 1e-3 ve 1e-4.

**Lisans:** değerler bu deponun lisansındadır. Değerleri hesaplayan yazılım soxr'dır (<https://github.com/dofuuz/python-soxr>, LGPL-2.1-or-later); kodu depoda yoktur.
