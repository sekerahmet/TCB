# tools/hf — Hugging Face modelleriyle karşılaştırma

`small_model.py`, `transformers` ile küçük ve rastgele ağırlıklı bir Llama ya da Qwen3 modeli kurar, `save_pretrained` ile (`config.json` + `model.safetensors`) kaydeder ve sabit token kimliklerinin float32 logitlerini `logits.json`'a yazar. Model indirilmez; ağ gerekmez.

```sh
python -I tools/hf/small_model.py --arch qwen3 --out /tmp/qwen3   # --tied: paylaşımlı gömme
```

`cmd/trlang/e2e` `TestReadyModelMatchesTransformers` bunu kullanır: `tr/pretrained`'ın `TR_fn_pretrained_model` ile yüklediği modelin logitleri transformers'ınkilerle float32 yuvarlama düzeyinde aynı olmalıdır (fark ≤ 1·10⁻⁵; ölçülen ~3·10⁻⁷). `torch`, `transformers` ya da `safetensors` yoksa test atlanır.

## Örnek modeli (`tests/data/hf_tiny`)

`examples/tr/46_hazir_model_ve_uretim.tr` ve `47_kendi_uretim_dongusu.tr` depodaki küçük bir modeli kullanır: rastgele ağırlıklı (anlamlı metin üretmez), `config.json` + `model.safetensors`, 18 KB. Gömme tablosu `tests/data/tokenizer.json`'ın dağarcığıyla (1005) eşleşir; parçalayıcı o dosyadan açılır, kopyası yoktur. Dosyalar belirlenimcidir (tohum 0, aynı komut aynı baytları verir; `transformers` sürümü ağırlık başlatmasını değiştirirse baytlar değişebilir ve örnek çıktıları yeniden üretilir). Yeniden üretmek için (torch ve transformers gerekir; sonra `generation_config.json` silinir, örnekler onu okumaz):

```sh
python -I tools/hf/small_model.py --arch llama --out tests/data/hf_tiny --tied --vocab 1005 --hidden 8 \
    --intermediate 16 --layers 1 --heads 2 --kv-heads 1 --max-position 32 --dtype float16 --no-logits
rm tests/data/hf_tiny/generation_config.json
```

Gerçek model, kişisel veri ya da indirilmiş ağırlık bu klasöre konmaz.

## VITS (`small_vits.py`, `vits_reference.py`)

`vits_reference.py` bir Hugging Face `VitsModel`'i (MMS-TTS) sabit cümlelerde koşar ve `lib/vits.tr`'nin aynısını vermesi gereken her değeri yazar: parça kimlikleri, `ceil`'den önceki ve sonraki süreler, metin kodlayıcının öncülü, iki gürültü tensörü, akıştan sonraki gizli değişken ve dalga biçimi. Her cümle iki kez koşar: gürültüsüz (`noise_scale=0`, `noise_scale_duration=0`) ve modelin gürültüsüyle. İkisi de float32 ve float64'te, aynı gürültüyle koşar: float32'nin float64'ten sapması kıyasın tabanıdır. `VitsModel.forward`'ın iki `randn`'ı kaydedilir ve float64'te yeniden verilir; TR aynı tensörleri dışarıdan alır.

```sh
python -I tools/hf/vits_reference.py --model facebook/mms-tts-tur --revision SHA --out DIR --wav
```

`small_vits.py` `tests/data/vits_tiny`'deki küçük modeli kurar: rastgele ağırlıklı (anlamlı ses üretmez), float16, tohum 0. Gerçek modelin düzenini taşır: olasılıklı süre öngörücü ve spline akışları, `add_blank`, `weight_g`/`weight_v` çiftleri. Sözlük bu deponundur, yayımlanmış bir modelden kopyalanmaz. U+0307 kendi parçasıdır: Python'un `str.lower()`'ı `İ`'yi iki karaktere ayırır. Yeniden üretmek için:

```sh
python -I tools/hf/small_vits.py --out tests/data/vits_tiny
python -I tools/hf/vits_reference.py --model tests/data/vits_tiny --out tests/data/vits_tiny/kiyas --sentence "iz=İz" \
    --tail-noise 6.5 --keep ids,noise_duration,noise_prior,log_duration,duration,waveform --f32-only
rm tests/data/vits_tiny/kiyas/meta.json
```

`--tail-noise 6.5` gürültülü koşunun süre gürültüsüne spline sınırının (5) dışına düşen bir değer koyar: TR testi aralık dışı dalı da görür.
