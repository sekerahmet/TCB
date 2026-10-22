# tools/hf — Hugging Face modelleriyle karşılaştırma

`small_model.py`, `transformers` ile küçük ve rastgele ağırlıklı bir Llama ya da Qwen3 modeli kurar, `save_pretrained` ile (`config.json` + `model.safetensors`) kaydeder ve sabit token kimliklerinin float32 logitlerini `logits.json`'a yazar. Model indirilmez; ağ gerekmez.

```sh
python -I tools/hf/small_model.py --arch qwen3 --out /tmp/qwen3   # --tied: paylaşımlı gömme
```

`cmd/tr/e2e` `TestReadyModelMatchesTransformers` bunu kullanır: `tr/hazir`'ın `hazır_model` ile yüklediği modelin logitleri transformers'ınkilerle float32 yuvarlama düzeyinde aynı olmalıdır (fark ≤ 1·10⁻⁵; ölçülen ~3·10⁻⁷). `torch`, `transformers` ya da `safetensors` yoksa test atlanır.

## Örnek modeli (`tests/data/hf_tiny`)

`examples/tr/46_hazir_model_ve_uretim.tr` ve `47_kendi_uretim_dongusu.tr` depodaki küçük bir modeli kullanır: rastgele ağırlıklı (anlamlı metin üretmez), `config.json` + `model.safetensors`, 18 KB. Gömme tablosu `tests/data/tokenizer.json`'ın dağarcığıyla (1005) eşleşir; parçalayıcı o dosyadan açılır, kopyası yoktur. Dosyalar belirlenimcidir (tohum 0, aynı komut aynı baytları verir; `transformers` sürümü ağırlık başlatmasını değiştirirse baytlar değişebilir ve örnek çıktıları yeniden üretilir). Yeniden üretmek için (torch ve transformers gerekir; sonra `generation_config.json` silinir, örnekler onu okumaz):

```sh
python -I tools/hf/small_model.py --arch llama --out tests/data/hf_tiny --tied --vocab 1005 --hidden 8 \
    --intermediate 16 --layers 1 --heads 2 --kv-heads 1 --max-position 32 --dtype float16 --no-logits
rm tests/data/hf_tiny/generation_config.json
```

Gerçek model, kişisel veri ya da indirilmiş ağırlık bu klasöre konmaz.
