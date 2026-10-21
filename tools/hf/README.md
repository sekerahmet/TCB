# tools/hf — Hugging Face modelleriyle karşılaştırma

`small_model.py`, `transformers` ile küçük ve rastgele ağırlıklı bir Llama ya da Qwen3 modeli kurar, `save_pretrained` ile (`config.json` + `model.safetensors`) kaydeder ve sabit token kimliklerinin float32 logitlerini `logits.json`'a yazar. Model indirilmez; ağ gerekmez.

```sh
python -I tools/hf/small_model.py --arch qwen3 --out /tmp/qwen3   # --tied: paylaşımlı gömme
```

`cmd/tr/e2e` `TestReadyModelMatchesTransformers` bunu kullanır: `tr/hazir`'ın `hazır_model` ile yüklediği modelin logitleri transformers'ınkilerle float32 yuvarlama düzeyinde aynı olmalıdır (fark ≤ 1·10⁻⁵; ölçülen ~3·10⁻⁷). `torch`, `transformers` ya da `safetensors` yoksa test atlanır.
