# Küçük VITS modeli

`tests/vits_model_test.tr` bu klasörü okur.

- **`config.json`, `model.safetensors`, `vocab.json`, `tokenizer_config.json`:** `tools/hf/small_vits.py` ile bu depo için üretildi: Hugging Face `transformers` 5.17.0'ın `VitsModel`'i bir yapılandırmadan kuruldu, ağırlıklar `torch` 2.14.0'ın tohum 0'lı rastgele başlatmasıdır, float16 saklanır. Yayımlanmış bir modelden gelmez; anlamlı ses üretmez. Sözlük bu deponundur.
- **`reference/iz_sessiz_f32.safetensors`, `reference/iz_gurultulu_f32.safetensors`:** `tools/hf/vits_reference.py`'nin bu modelde `"İz"` cümlesi için verdiği parça kimlikleri, iki gürültü tensörü, `ceil`'den önceki log süre, süreler ve dalga biçimi (float32). Gürültülü koşunun süre gürültüsünde bir değer 6,5'tir: spline sınırının dışına düşer.
- Yeniden üretmenin komutları `tools/hf/README.md`'dedir.

**Lisans:** dosyalar bu deponun lisansındadır. Değerleri hesaplayan yazılım `transformers`'tır (Apache 2.0).
