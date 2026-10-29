# 10. Hazır katmanlarla GPT

Bir dil modelini her seferinde ağırlık ağırlık yazmak gerekmez ([`examples/tr/35_transformer.tr`](../../examples/tr/35_transformer.tr) bunu öğretmek için yapar). Dilin kitaplığı hazır katmanlar ve iyileştiricilerle gelir; TR ile yazılmıştır, `tr` ikilisine gömülüdür ve `TR_import "tr/..."` ile alınır ([dil tanımı 11.6](../spec/11_moduller.md)). Kitaplığın kodu okunur: [`lib/katman.tr`](../../lib/katman.tr), [`lib/iyilestirici.tr`](../../lib/iyilestirici.tr); bütün adlar [`lib/README.md`](../../lib/README.md)'de. Uçtan uca örnek: [`examples/tr/41_gpt.tr`](../../examples/tr/41_gpt.tr).

## Model

```tr
# TR 4
TR_import "tr/katman"

TR_var model = TR_GPT(4096, TR_context: 256, TR_embed_dim: 256, TR_num_layers: 4, TR_num_heads: 4, TR_tied: TR_true, TR_seed: 1)
TR_var logit = model.TR_forward([[1, 2, 3]])            # [1, 3, 4096]
TR_var kayıp = model.TR_loss([[1, 2, 3]], [[2, 3, 4]]) # ortalama çapraz entropi
TR_print(TR_parameters(model, TR_named: TR_true).TR_keys()[0..3])
```

`TR_GPT` sıradan bir `TR_struct`dır: `model.TR_blocks[0].TR_attention.TR_q_proj.TR_weight` bir tensördür, `TR_parameters(model)` bütün ağırlıkları bulur, `TR_save_tensors(yol, TR_parameters(model, TR_named: TR_true))` kaydeder. İçinde döner konumlu (RoPE) nedensel çok bakışlı dikkat (İngilizce *multi-head attention*; `TR_num_kv_heads:` ile gruplu sorgu), SwiGLU ileri besleme ve ön-norm RMS normu vardır. Ağırlıklar N(0, 0.02) ile, artık yola yazan çıkış katmanları `0.02 / √(2 · katman)` ile başlar.

Modeli değiştirmek isterseniz `lib/katman.tr`'yi kendi klasörünüze kopyalayıp `TR_import "katman"` yazın: kopyanız sizindir, kitaplık değişmez. Tek bir katmanı da kullanabilirsiniz: `TR_Linear(giriş, çıkış)`, `TR_Embedding`, `TR_RMSNorm`, `TR_CausalAttention`, `TR_SwiGLU`, `TR_TransformerBlock`; hepsinin ileri hesabı `.TR_forward` metodudur.

## Yerel dikkat, PLE ve hedef model seçenekleri

`TR_GPT` ve `TR_TransformerBlock` daha büyük tariflere seçeneklerle uyar; hepsinin varsayılanı yukarıdaki sade modeldir. Katman başına yerel pencere (`TR_windows: TR_layer_windows("YYGYGYYG", [32, 64])`: G katmanı tam nedensel, Y katmanları son 32/64 konuma bakar), q/k normu (`TR_qk_norm: TR_true`), Muon'un K ve V'yi (`TR_merged_kv:`) ile SwiGLU'nun kapı ve yukarı matrislerini (`TR_merged_gate:`) birlikte dikleştirmesi için birleşik matrisler, `TR_eps:`, sapmalar ve katman başına kelime gömmesi (`TR_ple_dim:`, Gemma 3n'deki PLE) vardır:

```tr
# TR 4
TR_import "tr/katman"

TR_var model = TR_GPT(4096, TR_context: 64, TR_embed_dim: 64, TR_num_layers: 4, TR_num_heads: 4, TR_num_kv_heads: 2, TR_intermediate_dim: 96, TR_tied: TR_true,
    TR_windows: TR_layer_windows("YGYG", [8, 16]), TR_qk_norm: TR_true, TR_merged_kv: TR_true, TR_merged_gate: TR_true,
    TR_ff_use_bias: TR_true, TR_eps: 0.000001, TR_ple_dim: 32)
TR_print(model.TR_generate([1, 2, 3], 10, TR_temperature: 0, TR_eos: 0)) # eos örneklenirse üretim durur
```

`.TR_generate` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, sonraki her adım yalnızca yeni parçayı hesaplar (anahtar ve değerler katman başına önbellekte tutulur). `TR_cache_kind: "sabit"` (varsayılan; bir kez ayrılır, yerine yazılır), `"büyüyen"` (her adımda uzar) ya da `TR_none` (her adımda bütün dizi) seçilebilir; sonuç üçünde de aynıdır. Kendi üretim döngüsünü yazan `TR_var ö = model.TR_build_cache()` ile `model.TR_forward(kimlikler, TR_cache: ö)` çağırır; `ö.TR_truncate(n)` önbelleği ilk `n` parçaya indirir (sonrakileri geri alır).

Ayrıntılı anlatım ve formüller [`lib/README.md`](../../lib/README.md)'dedir. İyileştirici tarafında aynı tarif için `TR_MuonAndAdamW(model, TR_muon_scale: "rms", TR_adamw_weight_decay: 0.1, TR_separated: ["gömme", "ple_bağlamı.ctx_katmanı"], TR_sparse: ["ple.tablo"])` yazılır: parametreler ada göre (önek, sonek ya da tam yol) gruplanır, PLE tabloları satır seyrek RMS iyileştiricisine (`TR_RowRMS`, yalnızca o adımda gradyanı olan satırları günceller) gider.

## İyileştirici: Muon + AdamW

```tr
# TR 4
TR_import "tr/katman"
TR_import "tr/iyilestirici"

TR_var model = TR_GPT(4096, TR_context: 256, TR_embed_dim: 256, TR_num_layers: 4, TR_num_heads: 4)
TR_var op = TR_MuonAndAdamW(model, TR_muon_lr: 0.02, TR_adamw_lr: 0.003)
TR_var toplam_adım = 1000
TR_for adım TR_in 0..3 {
    op.TR_set_multiplier(TR_lr_schedule(adım, toplam_adım, TR_warmup: 100, TR_decay_fraction: 0.2))
    op.TR_zero_grad()
    TR_backward(model.TR_loss([[1, 2, 3]], [[2, 3, 4]]))
    op.TR_step()
}
```

`TR_MuonAndAdamW` modelin parametrelerini ikiye ayırır (`TR_parameter_groups`): gizli katmanların 2 boyutlu ağırlıkları **Muon**'a, gömme tablosu, çıkış katmanı, norm ağırlıkları ve sapmalar **AdamW**'ye gider. Muon her ağırlığın momentumunu Newton–Schulz yinelemesiyle dikleştirir (`TR_orthogonalize`): güncelleme bütün yönlerde aynı büyüklükte olur; küçük ve orta modellerde AdamW'den belirgin hızlı öğrenir. `TR_normuon: TR_true` NorMuon'u seçer (dikleştirmeden sonra nöron başına normalleştirme; öğrenme oranı AdamW ölçeğindedir, ör. `muon_oranı: 0.003`).

`TR_lr_schedule` ısınma–sabit–azalma (WSD) planıdır: ilk `TR_warmup:` adımda 0'dan 1'e çıkar, sonra sabit kalır, son `TR_decay_fraction:` kesrinde 0'a iner. Çarpan iki grubun oranını birlikte ölçekler.

## Kendi iyileştiricinizi yazmak

İyileştiriciler TR kodudur; tek yapı taşı parametreyi türev izlemeden **yerinde** güncellemektir ([15.34](../spec/15_tensorler.md)):

```tr
# TR 4
TR_func işaretli_sgd(ağırlıklar, oran) {
    TR_no_grad {
        TR_for p TR_in ağırlıklar {
            TR_if p.TR_grad != TR_none {
                p[0..p.TR_shape[0]] -= oran * TR_sign(p.TR_grad) # ağırlığın kendisine yazar
            }
        }
    }
}
```

Muon'u değiştirmek için `lib/iyilestirici.tr`'yi kopyalayın: `TR_Muon.TR_step()` on satırdır. Durum tensörlerini (momentum gibi) `TR_requires_grad: TR_true` ile kurarsanız `TR_Checkpoint` onları da saklar ([`lib/README.md`](../../lib/README.md), kontrol noktası).

## Eğitim döngüsü, kontrol noktası ve GPU

```tr
# TR 4
TR_import "tr/katman"
TR_import "tr/iyilestirici"

TR_var ayar = TR_command_line({"step": 100, "koşu": "runs/gpt"})
TR_var p = TR_load_tokenizer("../tests/data/tokenizer.json")
TR_var ö = TR_Dataset("../tests/data/text").TR_sampler(TR_batch_size: 8, TR_context: 32, TR_seed: 1)
TR_var model = TR_GPT(p.TR_vocab_size, TR_context: 32, TR_embed_dim: 32, TR_num_layers: 2, TR_num_heads: 4, TR_tied: TR_true)
TR_var op = TR_MuonAndAdamW(model)
TR_var g = TR_Logger(TR_Path(ayar["koşu"], "metrics.jsonl"))
TR_var kn = TR_Checkpoint(ayar["koşu"], {"model": model, "adamw": op.TR_adamw, "muon": op.TR_muon.TR_state, "ö": ö, "g": g})
TR_var ç = TR_Progress(ayar["step"], TR_log: g)
TR_for adım TR_in kn.TR_resume_step..ayar["step"] {
    op.TR_set_multiplier(TR_lr_schedule(adım, ayar["step"], TR_warmup: 10))
    TR_var giriş, hedef = ö.TR_next()
    op.TR_zero_grad()
    TR_var kayıp = model.TR_loss(giriş, hedef)
    TR_backward(kayıp)
    op.TR_step()
    ç.TR_update({"loss": kayıp}, TR_step: adım + 1)
    kn.TR_update(adım + 1)
}
TR_print(p.TR_decode(model.TR_generate(p.TR_encode("Bugün"), 12, TR_temperature: 0.8, TR_seed: 1)))
```

Kontrol noktasına modelin yanında iki iyileştiricinin durumu da verilir: `op.TR_adamw` ve `op.TR_muon.TR_state`. GPU'da eğitmek için model kurulmadan önce `TR_default_device("gpu")` çağrılır; ağırlıklar, Muon'un durumu ve toplu işler GPU'da olur ([15.38](../spec/15_tensorler.md)). Bulut not defterinde büyük bir eğitim için [`tools/gpu/gpt_training.tr`](../../tools/gpu/gpt_training.tr) hazır betiğine bakın (SimpleStories, d512, 8 katman; [`tools/gpu/COLAB.md`](../../tools/gpu/COLAB.md) 9. hücre).

## Hazır model, üretim ve kayıtlı oynatma

Hugging Face biçiminde kaydedilmiş bir Llama ya da Qwen3 modelini (`config.json` + `model.safetensors`) eğitmeden kullanmak için `tr/hazir` yeter. Ağ gerekmez: model önceden indirilmiş bir klasördür.

```tr
# TR 4
TR_import "tr/hazir"

TR_var model = TR_pretrained_model("modeller/SmolLM2-135M", TR_weight_dtype: "bf16")
TR_var p = TR_load_tokenizer("modeller/SmolLM2-135M/tokenizer.json")
TR_var kimlikler = model.TR_generate(p.TR_encode("Bugün hava"), 20, TR_temperature: 0)
TR_print(p.TR_decode(kimlikler))
```

- **`TR_weight_dtype:`:** matris ağırlıklarının (gömme ve doğrusal katmanlar) türüdür (`"float32"`, `"bf16"`, `"fp16"`); normlar ve hesap float32 kalır. Hugging Face modelleri çoğunlukla bf16 saklanır, `TR_weight_dtype: "bf16"` ağırlıkları yarı bellekle tutar. Ayrıntı: [`lib/README.md`](../../lib/README.md).
- **`.TR_generate` ve önbellek:** `model.TR_generate(kimlikler, TR_max_new_tokens:, TR_temperature:, TR_top_k:, TR_seed:, eos:, TR_cache_kind:, TR_cuda_graph:)` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, her sonraki adım yalnızca yeni parçayı hesaplar. `TR_cache_kind:` `"sabit"` (varsayılan; bir kez ayrılır, yerine yazılır), `"büyüyen"` ya da `TR_none` (her adımda bütün dizi) olabilir; sonuç üçünde de aynıdır. `TR_temperature: 0` her adımda en olası parçayı seçer (açgözlü); aynı `TR_seed:` aynı örneği verir. Ayrıntı: [`lib/README.md`](../../lib/README.md) "Üretim".
- **`TR_cuda_graph:` ve `TR_replay`:** GPU'da sabit önbellekle (`TR_cuda_graph: TR_true`, varsayılan) her üretim adımı bir kez kaydedilir (CUDA Graph) ve sonraki adımlarda tek seferde oynatılır; yorumlayıcı adımı yeniden çalıştırmaz ve yüzlerce kernelin tek tek başlatılması ortadan kalkar. Aynı mekanizmayı kendi adımınız için `TR_replay(f)` ile, durumu yerinde güncellemek için `TR_write_slice(hedef, kaynak, TR_dim:, TR_start:)` ile kullanırsınız; değişen her değer (konum, parça kimliği) tensör olarak verilir. CPU'da `TR_replay(f)` `f`'i doğrudan çağırır, sonuç aynıdır. Kurallar: [dil tanımı 15.45 ve 15.8](../spec/15_tensorler.md). Örnekler: [`46_hazir_model_ve_uretim.tr`](../../examples/tr/46_hazir_model_ve_uretim.tr), [`47_kendi_uretim_dongusu.tr`](../../examples/tr/47_kendi_uretim_dongusu.tr), [`45_tekrar_oynat.tr`](../../examples/tr/45_tekrar_oynat.tr).
