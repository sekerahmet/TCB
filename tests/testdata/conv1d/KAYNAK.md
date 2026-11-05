# Evrişim başvuru dosyası

`tests/conv1d_test.tr` bu dosyayı okur (15.23c).

- **`conv1d_pytorch.bin`:** bu depo için üretildi: `numpy.random.default_rng(1505)` ile rastgele girdi, ağırlık, sapma ve çıkış gradyanı; sonuç ve gradyanlar PyTorch 2.14.0'ın `torch.nn.functional.conv1d(x, w, b, stride=2, padding=2, dilation=2, groups=2)` çağrısından. Sıra: `x` `[2, 4, 9]`, `w` `[6, 2, 3]`, `b` `[6]`, çıkış gradyanı, sonuç, `x`'in, `w`'nin ve `b`'nin gradyanı; hepsi `<f4`, arka arkaya. Kişi verisi ve yayımlanmış model ağırlığı içermez.

**Lisans:** değerler bu deponun lisansındadır. Değerleri hesaplayan yazılım PyTorch'tur (<https://github.com/pytorch/pytorch>, BSD-3-Clause).
