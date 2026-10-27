# TR standart kitaplığı

TR ile yazılmış, `tr` ikilisine gömülü kitaplık. Modüller `TR_kw_import "tr/..."` ile alınır (dil tanımı [11.6](../docs/spec/11_moduller.md)): `tr/` öneki dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez; kitaplık `TR_cmd_build` ile üretilen uygulamaya da girer. Go tarafı tek bir gömme dosyasıdır (`gomulu.go`); kitaplığın kodu bu klasördeki `.tr` dosyalarıdır, okunur ve kopyalanıp değiştirilebilir.

| Modül | Dosya | İçerik |
| :-- | :-- | :-- |
| `tr/katman` | [`katman.tr`](katman.tr) | Transformer katmanları ve yalnızca kod çözücülü `TR_type_GPT` |
| `tr/iyilestirici` | [`iyilestirici.tr`](iyilestirici.tr) | Muon, NorMuon, parametre grupları, öğrenme oranı planı |
| `tr/egitim` | [`egitim.tr`](egitim.tr) | Uzun eğitimin güvenliği: koşunun kayıt işleri (`TR_type_Run`, prova) ve kayıp ile gradyan normu sıçrama bekçisi |
| `tr/hazir` | [`hazir.tr`](hazir.tr) | Hugging Face biçimindeki hazır modelleri (Llama, Qwen3) GPT'ye yüklemek |

Testler her modülün yanındadır (`katman_test.tr`, `iyilestirici_test.tr`, `egitim_test.tr`, `hazir_test.tr`) ve `tr test lib` ile koşar (tam denetimin parçasıdır). Test dosyaları gömülür ama modül olarak aktarılamaz. Kullanım rehberi: [kılavuz §10](../docs/guide/10_hazir_katmanlarla_gpt.md); tam örnek: [`examples/tr/41_gpt.tr`](../examples/tr/41_gpt.tr); GPU'da SimpleStories eğitimi: [`tools/gpu/gpt_training.tr`](../tools/gpu/gpt_training.tr).

```tr
//! tr kodlu 3 dil=tr
TR_kw_import "tr/katman"
TR_kw_import "tr/iyilestirici"

TR_kw_var model = TR_type_GPT(1000, TR_arg_context: 64, TR_arg_embed_dim: 64, TR_arg_num_layers: 2, TR_arg_num_heads: 4)
TR_kw_var op = TR_type_MuonAndAdamW(model) // gizli 2B ağırlıklar Muon, öbürleri AdamW
TR_kw_var giriş = [[1, 2, 3, 4]]
TR_kw_var hedef = [[2, 3, 4, 5]]
TR_kw_for adım TR_kw_in 0..10 {
    op.TR_mem_set_multiplier(TR_fn_lr_schedule(adım, 10, TR_arg_warmup: 2))
    op.TR_mem_zero_grad()
    TR_fn_backward(model.TR_mem_loss(giriş, hedef))
    op.TR_mem_step()
}
```

## `tr/katman`

Her katman sıradan bir `TR_kw_struct`dır: ağırlıklar alanlarındadır, ileri hesap `ileri` metodudur, `TR_fn_parameters(model)` hepsini bulur (adlar kimlik yoludur: `TR_mem_blocks.0.TR_mem_attention.TR_mem_q_proj.TR_mem_weight`, gösterimde `bloklar.0.dikkat.sorgu_katmanı.ağırlık`). Girdiler `[toplu, uzunluk, boyut]`, çarpım `x @ W`, Doğrusal'ın ağırlığı `[giriş, çıkış]` şeklindedir.

| Ad | Ne yapar | Kurulum |
| :-- | :-- | :-- |
| `TR_type_Linear` | `x @ ağırlık (+ sapma)`; sapma çarpımın türünde eklenir (karışık hassasiyette çıktı bf16, PyTorch autocast gibi) | `TR_type_Linear(giriş_boyutu, çıkış_boyutu, TR_arg_use_bias: TR_kw_false, TR_arg_init_std: 0.02, TR_arg_seed: 1)` |
| `TR_type_Embedding` | kimlik → tablo satırı (`TR_fn_embedding` yerleşiği) | `TR_type_Embedding(dağarcık_büyüklüğü, boyut, TR_arg_init_std: 0.02, TR_arg_seed: 1)` |
| `TR_type_RMSNorm` | RMS normu, öğrenilen ağırlıkla (`TR_fn_rms_norm` yerleşiği) | `TR_type_RMSNorm(boyut, TR_arg_eps: 0.00001)` |
| `TR_fn_rotary_table`, `TR_fn_apply_rotary_fused` | döner konum gömmesi (RoPE): kosinüs/sinüs tabloları ve uygulanması | `TR_fn_rotary_table(bağlam, bakış_boyutu, TR_arg_rope_theta: 10000.0)` |
| `TR_type_CausalAttention` | RoPE'lu nedensel çok bakışlı dikkat (İngilizce *multi-head attention*; her bakış diziye kendi alt uzayından bakar), gruplu sorgu (`kv_bakış_sayısı`); birleşik `TR_fn_attention` yerleşiği | `NedenselDikkat(boyut, bakış_sayısı, kv_bakış_sayısı: yok, sapmalı: yanlış, std:, çıkış_std:, tohum:, pencere: 0, qk_normu: yanlış, birleşik_kv: yanlış, epsilon:, bakış_boyutu: yok)` |
| `TR_fn_swiglu_intermediate_dim` | SwiGLU ara boyutu: 8/3 · boyut, 32'nin katına yukarı | `TR_fn_swiglu_intermediate_dim(boyut)` |
| `TR_type_SwiGLU` | kapılı ileri besleme `aşağı(TR_fn_silu(kapı(x)) * yukarı(x))` | `SwiGLU(boyut, ara_boyut: yok, sapmalı:, başlatma_sapması:, çıkış_başlatma_sapması:, tohum:, birleşik: yanlış)` |
| `TR_type_PLEContext`, `TR_type_PLEBlock`, `TR_fn_ple_gelu` | katman başına kelime gömmesi (PLE, Gemma 3n): ortak bağlam ve katman başına tablo/kapı/izdüşüm | `PLEBağlamı(boyut, katman_sayısı, ple_boyutu, epsilon:, başlatma_sapması:, tohum:)`, `PLEBloğu(boyut, dağarcık, ple_boyutu, epsilon:, std:, çıkış_std:, tohum:)` |
| `TR_type_TransformerBlock` | ön-norm blok: `x + TR_fn_attention(norm(x))`, `+ ileri_besleme(norm(x))`, (PLE varsa) `+ PLE katkısı` | `TransformerBloğu(boyut, bakış_sayısı, kv_bakış_sayısı:, ara_boyut:, başlatma_sapması:, çıkış_başlatma_sapması:, tohum:, epsilon:, pencere:, qk_normu:, birleşik_kv:, sapmalı:, ileri_sapmalı:, birleşik_kapı:, ple_boyutu:, dağarcık:, bakış_boyutu:)` |
| `TR_fn_layer_windows` | `"YYGYGYYG"` düzeninden katman başına pencere listesi | `TR_fn_layer_windows(düzen, pencereler)` |
| `TR_type_GPT` | gömme → bloklar → son norm → logit; `ileri`, `kayıp`, `TR_mem_generate` | `GPT(dağarcık_büyüklüğü, bağlam: 256, boyut: 256, katman_sayısı: 4, bakış_sayısı: 4, kv_bakış_sayısı: yok, ara_boyut: yok, paylaşımlı: yanlış, tohum: 1, pencereler: yok, qk_normu:, birleşik_kv:, birleşik_kapı:, ileri_sapmalı:, epsilon:, ple_boyutu: yok, bakış_boyutu: yok, rope_tabanı: 10000.0)` |
| `TR_fn_init_weight` | N(0, std) ile başlayan, türev izleyen ağırlık | `TR_fn_init_weight(şekil, başlatma_sapması, tohum)` |

**Başlatma.** Bütün ağırlık matrisleri N(0, 0.02) ile başlar. Artık yola yazan çıkış katmanları (dikkatin `TR_mem_o_proj`, ileri beslemenin `TR_mem_down_proj`sı) `0.02 / √(2 · katman)` ile küçültülür: her blok artık yola iki katkı ekler, katkıların varyansı katman sayısıyla büyümesin. Norm ağırlıkları 1, sapmalar 0 ile başlar. Her ağırlığın sabit bir tohumu vardır (blok `i` için `tohum + 100·(i+1)` ve ardışıkları): aynı `tohum` aynı modeli verir. `TR_arg_tied: TR_kw_true` çıkışta gömme tablosunu kullanır (`logit = h @ tablo.TR_mem_transpose()`).

**Hedef model seçenekleri (RGN-512).** Aşağıdaki seçeneklerin hepsinin varsayılanı eski davranıştır (`katman_test.tr` varsayılanın değişmediğini sınar). `tools/gpu/rgn512.py`'nin ileri hesabı yeni bir model yapısı gerektirmez, `TR_type_GPT`nin blok seçenekleriyle kurulur:

```tr
//! tr kodlu 3 dil=tr
TR_kw_import "tr/katman"

TR_kw_var rgn = TR_type_GPT(40000, TR_arg_context: 1024, TR_arg_embed_dim: 512, TR_arg_num_layers: 8, TR_arg_num_heads: 8, TR_arg_num_kv_heads: 4, TR_arg_intermediate_dim: 672, TR_arg_tied: TR_kw_true,
    TR_arg_windows: TR_fn_layer_windows("YYGYGYYG", [32, 64]), TR_arg_qk_norm: TR_kw_true, TR_arg_merged_kv: TR_kw_true,
    TR_arg_merged_gate: TR_kw_true, TR_arg_ff_use_bias: TR_kw_true, TR_arg_eps: 0.000001, TR_arg_ple_dim: 512)
```

- `pencere` (dikkat/blok): her konum yalnızca son `pencere` konuma bakar (`dikkat(..., pencere:)`); 0 tam nedensel. Pencere katmanın özelliğidir: `TR_type_GPT`de `pencereler` katman başına bir listedir (G katman 0, Y katman 32/64...), `TR_fn_layer_windows` düzen metninden kurar (Y katmanları pencere listesini sırayla alır).
- `bakış_boyutu` (dikkat/blok/GPT): bir bakışın boyutu; verilmezse `boyut / bakış_sayısı`. Verilirse gizli boyuttan bağımsızdır (Hugging Face `head_dim`; Qwen3-0.6B: gizli 1024, 16 bakış × 128): sorgu katmanı `[boyut, bakış_sayısı · bakış_boyutu]`, çıkış katmanı `[bakış_sayısı · bakış_boyutu, boyut]`, döner konum tabloları bu boydadır; çift olmalıdır.
- `qk_normu`: q ve k, döner konumdan önce bakış boyutunda RMS normundan geçer (`TR_mem_q_norm`, `TR_mem_k_norm`; hedefteki `q_norm/k_norm`).
- `birleşik_kv`: anahtar ve değer tek `TR_mem_kv_proj` (`[boyut, 2·kv_boyut]`, `[K | V]`, `.TR_mem_split(2)`); Muon ikisini birlikte dikleştirir. `birleşik_kapı`: SwiGLU'da `[G | U]` tek `TR_mem_gate_up_proj` matrisi. Birleşikken ayrı katmanlar `TR_kw_none`'tur (parametre olarak görünmezler).
- `sapmalı` (dikkat) ve `ileri_sapmalı` (SwiGLU): sapma ekler (hedefte yalnızca MLP'nin sapması vardır). `epsilon`: bloğun bütün RMS normlarının epsilon'u (hedef 1e-6; varsayılan 1e-5).
- **PLE** (`TR_arg_ple_dim: P`): `TR_type_PLEContext` gömme çıktısından `RMSNorm_P(x @ W_ctx / √D)` ile katman başına `[B, T, P]` bağlamlarını üretir (`TR_type_GPT.TR_mem_ple_context`); her blok, kendi tablosuyla (`bloklar.i.TR_mem_ple.TR_mem_table`, `[dağarcık, P]`) `e = (bağlam_i + √P · tablo_i[kelime]) / √2` kurar ve blok sonunda `H ← H + RMSNorm_D(W_p · (GELU(W_g · H) ⊙ e))` ekler (`TR_type_PLEBlock`). GELU çağrısı kitaplıkta yalnızca `TR_fn_ple_gelu`dadır: tam (erf) GELU seçeneği gelince tek satır değişir.

**Üretim.** `model.TR_mem_generate(kimlikler, en_fazla_yeni_parça, TR_arg_temperature: 1.0, TR_arg_top_k: 0, TR_arg_seed: TR_kw_none, TR_arg_eos: TR_kw_none, TR_arg_cache_kind: "sabit")` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, sonraki her adım yalnızca yeni parçayı hesaplar ve yalnızca son konumun logitini çıkarır (transformers `generate`'in `logits_to_keep`'i gibi). `eos` bir parça kimliğidir: o parça örneklenince dizide kalır ve üretim `adet` dolmadan durur. Dizi bağlamı doldurunca son `bağlam` parça yeniden işlenir. GPU'da sabit önbellekle (`çizge: TR_kw_true`, varsayılan) her adım `TR_type_GPT.TR_mem_single_step`'dır ve `TR_fn_replay` ile kaydedilip oynatılır (dil tanımı 15.45): konum ve parça kimliği cihazda bir tensördür, yeni anahtarlar `TR_fn_write_slice` ile konuma yazılır, dikkat tamponun tamamına maskeyle bakar (transformers sabit önbelleği ve `cache_position` ile aynı yol); yorumlayıcı adımı yeniden çalıştırmaz. Sonuç kayıtsızla aynı parçalardır; `çizge: TR_kw_false` kapatır. Sabit önbellek en çok başlangıç + adet konum tutar.
- **`TR_type_KVCache`** (`model.TR_mem_build_cache(TR_arg_kind: "sabit", TR_arg_capacity: bağlam)`): katman başına döner konumlu anahtar ve değerler, `[B, T, kv_bakış, bakış_boyutu]` düzende (dikkat'in düzeni; GQA'da çoğaltılmaz). `"sabit"` tamponu bir kez ayırır ve yeni anahtarları zaman ekseninde yerine yazar (gpt-fast, FlashAttention `flash_attn_with_kvcache` gibi); `"büyüyen"` her adımda `TR_fn_concat` ile uzar (transformers `DynamicCache` gibi). `model.TR_mem_forward(kimlikler, TR_arg_cache: ö)` önbellekteki konumdan devam eder: döner konum tabloları o konumdan dilimlenir, `TR_fn_attention` önbellekteki bütün konumlara bakar (nedensellik sondan hizalı, dil tanımı 15.24). `ö.TR_mem_strip(n)` önbelleği ilk `n` parçaya indirir, sonrakileri geri alır (reddedilen taslak). Önbellek türev izlemez.
- **Ölçüm (SmolLM2-135M, CPU, 100 parça, açgözlü, 4 tur ortancası, 2026-10-21):** önbelleksiz 1,7; büyüyen 23,6; sabit 24,7 parça/sn; transformers `generate` 23,2. Colab (RTX PRO 6000, 3 tur): GPU'da sabit ~216, büyüyen ~201, transformers ~109; Colab işlemcisinde (24 iş parçacığı) TR ~38, transformers ~37 parça/sn. Önbellekli ve önbelleksiz sonuç aynıdır (`lib/katman_test.tr`).

**`TR_mem_single_step` ve `TR_mem_hidden_states`.** Kendi üretim döngünüz için iki yapı taşı: `model.TR_mem_hidden_states(kimlikler, TR_arg_cache: ö)` kimlikler `[B, T]` (liste) için gizli durumu `[B, T, boyut]` verir ve önbelleğe yazar (`ö.TR_mem_position` T kadar ilerler); `model.TR_mem_logits(gizli)` onu dağarcık logitine çevirir (üretimde yalnızca son konum için: `model.TR_mem_logits(x.TR_mem_slice(1, T - 1, T))[0][0]`). `model.TR_mem_single_step(kimlik, konum, ö)` bir parçalık adımdır: `kimlik` `[1, 1]` ve `konum` `[1]` **tensörleri** (konum cihazdadır, bu yüzden adım `TR_fn_replay` ile kaydedilebilir), `ö` **sabit** önbellek olmalı ve önceki parçalar `gizli_durum` ile işlenmiş olmalıdır; son konumun logitini `[dağarcık]` verir. Konumu çağıran ilerletir: `ö.TR_mem_advance(1)`. CPU'da `TR_fn_replay` adımı doğrudan çağırır. Örnek: `examples/tr/47_kendi_uretim_dongusu.tr` (elle yazılan döngü `model.TR_mem_generate` ile aynı diziyi verir).

## `tr/iyilestirici`

İyileştiriciler TR ile yazılmıştır: parametreyi türev izlemeden yerinde günceller (`TR_fn_no_grad { p[0..p.TR_mem_shape[0]] -= ... }`, dil tanımı [15.34](../docs/spec/15_tensorler.md)). Go tarafına iyileştirici eklenmez; yeni bir yöntem için bu dosya kopyalanıp değiştirilir.

| Ad | Ne yapar |
| :-- | :-- |
| `TR_fn_orthogonalize(G, TR_arg_ns_iterations: 5, TR_arg_ns_dtype: "float32")` | Newton–Schulz ile G'nin yaklaşık dik eşi (≈ U Vᵀ); katsayılar (3.4445, −4.7750, 2.0315); tekil değerler ≈ [0.7, 1.2]; G bir matris ya da aynı şekilli matrislerin yığını `[k, satır, sütun]` (her matris ayrı, toplu çarpımla); `tür` hesap türü, sonuç G'nin türünde |
| `TR_type_Muon(parametreler, TR_arg_lr: 0.02, TR_arg_momentum: 0.95, TR_arg_nesterov: TR_kw_true, TR_arg_weight_decay: 0.0, TR_arg_ns_iterations: 5, TR_arg_scale: "oran", TR_arg_ns_dtype: "float32")` | Nesterov momentumu + dikleştirme; ölçek `"oran"`: √max(1, çıkış/giriş), `"rms"`: 0.2·√max(giriş, çıkış); aynı şekilli ağırlıklar tek yığında dikleştirilir; `ns_türü` dikleştirmenin hesap türü (`"bf16"`); yalnızca 2B ağırlıklar |
| `TR_type_NorMuon(parametreler, TR_arg_lr: 0.003, TR_arg_beta1: 0.95, TR_arg_beta2: 0.95, TR_arg_eps: 1e-8, TR_arg_weight_decay: 0.0, TR_arg_ns_iterations: 5, TR_arg_ns_dtype: "float32")` | Muon + nöron başına ikinci moment normalleştirmesi; güncellemenin RMS'i 0.2·η |
| `TR_fn_parameter_groups(model, TR_arg_separated: ["TR_mem_embedding", "TR_mem_logit_layer"])` | `{gizli: [...], TR_mem_other: [...]}`: 2B gizli ağırlıklar ile gömme, çıkış, norm ve sapmalar |
| `parametre_kümeleri(model, ayrı:, seyrek: [])` | `{gizli, sönümlü, sönümsüz, seyrek, diğer}`: ada göre dört grup (aşağıda) |
| `TR_fn_name_matches(ad, desen)`, `TR_fn_any_pattern_matches(ad, desenler)` | yol parçası eşleşmesi: tam (`TR_fn_embedding.TR_mem_table`), önek (`TR_fn_embedding`), sonek (`ple.TR_mem_table`) |
| `TR_type_RowRMS(parametreler, TR_arg_lr: 0.003, TR_arg_beta2: 0.95, TR_arg_eps: 1e-8)` | satır seyrek RMS (yoğun maskeli): yalnızca gradyanı sıfırdan farklı satırlar güncellenir; satır başına ikinci moment ve sayaç |
| `MuonVeAdamW(model, muon_öğrenme_oranı: yok, adamw_öğrenme_oranı: 0.003, ağırlık_çürümesi: 0.0, normuon: yanlış, ayrılan:, beta2: 0.95, adamw_çürümesi: 0.0, muon_ölçeği: "oran", seyrek: [], seyrek_öğrenme_oranı: yok, ns_türü: "float32")` | gruplar tek iyileştiricide: `TR_mem_step()`, `TR_mem_zero_grad()`, `çarpan_ayarla(c)`; alanlar `TR_mem_muon`, `TR_mem_adamw`, `adamw_sönümsüz`, `TR_mem_row_rms` |
| `TR_fn_lr_schedule(adım, adım_sayısı, TR_arg_warmup: 0, TR_arg_decay_fraction: 0.2, TR_arg_min: 0.0)` | ısınma–sabit–azalma (WSD) öğrenme oranı çarpanı |
| `TR_fn_zero_grads(parametreler)` | gradyanları yerinde sıfırlar |

**Muon.** Kaynak: K. Jordan ve diğ., *Muon: An optimizer for hidden layers in neural networks* (2024, kellerjordan.github.io/posts/muon). Her W için `m ← β·m + (1−β)·g`, Nesterov'la `u ← β·m + (1−β)·g`, `W ← W·(1 − η·λ) − η·√max(1, çıkış/giriş)·dikleştir(u)`. Ölçek, ağırlığı `[çıkış, giriş]` tutan düzendeki √max(1, satır/sütun)'un bu düzendeki karşılığıdır. Aynı şekilli ağırlıklar (`TR_fn_shape_groups`) tek yığında dikleştirilir: hedef modelde 56 ayrı dikleştirme yerine 3 toplu dikleştirme (PyTorch'taki yaygın uygulamalar ve `tools/gpu/rgn512.py` de `torch.stack` ile böyle yapar); sonuç tek tek dikleştirmeyle aynıdır (test korur).

**NorMuon.** Kaynak: Z. Li, L. Liu, C. Liang, W. Chen, T. Zhao, *NorMuon: Making Muon more efficient and scalable* (2025, arXiv:2510.05491, Algoritma 1). `M ← β1·M + (1−β1)·g`, `O ← dikleştir(M)`, `v ← β2·v + (1−β2)·ortalama(O², nöron başına)`, `Ô ← O / (√v + ε)`, `η̂ ← 0.2·η·√(m·n) / ‖Ô‖_F`, `W ← W − η·λ·W − η̂·Ô`. Bu düzende bir nöron W'nin bir **sütunudur** (`v` `[1, çıkış]`). Makaledeki gibi Nesterov'suz ve sapma düzeltmesizdir. **Varsayımlar:** ε = 1e-8 (makalede sayı yok); varsayılan öğrenme oranı 0.003 bu kitaplığın seçimidir (oran AdamW ölçeğindedir; makalede 124M'lik modelde 3.6e-4, 1.1B'de 4e-4).

**Ad desenleri ve gruplar.** `ayrılan` ve `seyrek` listeleri `TR_fn_parameters(model, TR_arg_named: TR_kw_true)` adlarına yol parçası sınırında eşleşir (`TR_fn_name_matches`): adlar ve desenler kimlik yoludur: `"TR_mem_embedding"` adın başı (`TR_mem_embedding.TR_mem_table`), `"TR_mem_ple_context.TR_mem_context_proj"` başı, `"TR_mem_ple.TR_mem_table"` sonu (`TR_mem_blocks.3.TR_mem_ple.TR_mem_table`), `"TR_mem_embedding.TR_mem_table"` tamamı. `parametre_kümeleri` her parametreyi bir gruba koyar (öncelik sırası): `seyrek` (desene uyanlar), `sönümsüz` (2B olmayanlar: normler, sapmalar), `sönümlü` (`ayrılan`a uyan 2B ağırlıklar: gömme, `ple_ctx`), `gizli` (öbür 2B ağırlıklar: Muon). `MuonVeAdamW(..., adamw_çürümesi: λ)` λ > 0 iken AdamW'yi ikiye böler: `op.TR_mem_adamw` sönümlü grubu λ ile, `op.adamw_sönümsüz` normları ve sapmaları çürümesiz günceller; λ = 0 (varsayılan) tek çürümesiz `op.TR_mem_adamw`'dır. `TR_arg_sparse: ["TR_mem_ple.TR_mem_table"]` verilirse bu parametreler `op.TR_mem_row_rms` (`TR_type_RowRMS`) iyileştiricisine gider; oranı `seyrek_öğrenme_oranı`dır (yok: `adamw_öğrenme_oranı`). `ağırlık_çürümesi` Muon'un çürümesidir. RGN-512 tarifi: `TR_type_MuonAndAdamW(model, TR_arg_muon_lr: 0.002, TR_arg_adamw_lr: 0.002, TR_arg_weight_decay: 0.1, TR_arg_adamw_weight_decay: 0.1, TR_arg_muon_scale: "rms", TR_arg_separated: ["TR_mem_embedding", "TR_mem_ple_context.TR_mem_context_proj"], TR_arg_sparse: ["TR_mem_ple.TR_mem_table"])`. Kontrol noktasına `op.TR_mem_adamw`, (varsa) `op.adamw_sönümsüz`, `op.TR_mem_muon.TR_mem_state` ve (varsa) `op.TR_mem_row_rms.TR_mem_state` verilir.

**SatırRMS.** Tablo `[satır, sütun]` (kelime × P). Adımda gradyanı sıfırdan farklı satırlar için `n ← n + 1`, `v ← β2·v + (1−β2)·ortalama(g²)` (satırın elemanları üzerinden), `W ← W − η·g / (√(v / (1−β2ⁿ)) + ε)`; sapma düzeltmesi satırın kendi sayacıyladır (ilk kez görülen kelime doğru ölçekle başlar), gradyanı olmayan satırın ağırlığı, `v`'si ve sayacı değişmez. Momentum yoktur. Durum `op.TR_mem_state.ikinci_moment` ve `op.TR_mem_state.sayaç` (`[satır, 1]`) tensörleridir. **Bu sürüm yoğun maskelidir**: gradyan ve durum tam tablo boyundadır, maliyet O(satır · sütun)'dur; anlam seyrek sürümle aynıdır, gerçek seyrek yol (yalnızca geçen satırlara dokunan) yerleşik olarak gelince arayüz değişmez.

**Öğrenme oranı çarpanı.** `çarpan_ayarla(c)` (c pozitif) sonraki adımda iki grubun oranını birlikte ölçekler: `TR_type_MuonAndAdamW` her adımda `op.TR_mem_muon.TR_mem_lr`'nı ve `op.TR_mem_adamw.TR_mem_lr`'nı (dil tanımı [15.35](../docs/spec/15_tensorler.md)) taban oran × c yapar. Kopya ya da ek bellek yoktur; c = 1 iken AdamW'nin adımı planı olmayan AdamW'ninkiyle bit düzeyinde aynıdır (`iyilestirici_test.tr` sınar).

**Kontrol noktası.** Muon'un durum tensörleri `op.TR_mem_muon.TR_mem_state` sözlüğündedir ve türev izlemez (parametre değildir, yalnızca türevsiz blokta yerinde güncellenir); `TR_fn_checkpoint` bir nesnenin türev izlemeyen tensörlerini de saklayıp yerinde geri yazar (dil tanımı [20.2](../docs/spec/20_kontrol_noktasi.md)). Kontrol noktasına modelin yanında `op.TR_mem_adamw` ve `op.TR_mem_muon.TR_mem_state` da verilir (örnek: `examples/tr/41_gpt.tr`); sürdürülen eğitim kesintisizle aynıdır (`iyilestirici_test.tr`).

## Adlar ve açık konular

Kitaplığın adları kullanıcı düzeyi TR adlarıdır; dilin kodu değildir, `internal/lang/codes/tr.json`'a girmez. Yerleşiklerle karışmasın diye katmanlar büyük harfli ve ayırt edicidir (`TR_fn_embedding` yerleşiği ↔ `TR_type_Embedding`, `TR_fn_rms_norm` ↔ `TR_type_RMSNorm`, `TR_fn_attention` ↔ `TR_type_CausalAttention`).

**Yazım dilleri.** Kitaplığın dili yoktur: dışa açık her adı (yapı, fonksiyon, sabit, alan, metot, parametre) bir koddur (dil tanımı [17](../docs/spec/17_kodlar.md); `definitions.json` dosyasında `modül` alanıyla) ve her yazım dilinde o dilin adıyla görünür: `Doğrusal` / `Linear` (`TR_type_Linear`), `MuonVeAdamW` / `MuonAndAdamW`. Ağırlık adları da kimlik yoludur (`TR_mem_blocks.0.TR_mem_attention.TR_mem_q_proj.TR_mem_weight`). Yerel değişkenler, yorumlar ve metinler çevrilmez.

**Hata iletileri.** Kitaplık hata metni yazmaz: her `TR_kw_throw` ve `TR_fn_assert` bir katalog iletisi kurar (`TR_fn_message(H6903, pencere)`, 27 kod `H6900`–`H6946`, `internal/lang/messages`); metin okuyanın dilinde kurulur ve kitaplığın kendi adları iletide o dilin adıyla yazılır. Testler iletinin metnine değil koduna bakar (`TR_fn_error_code`). Denetim: `TestMessageUsage` (tools/gen/msgconsts), `TestMessageSlotsAreNames` (internal/lang/codes).

## `tr/egitim`

`TR_type_Run(klasör, ayar, nesneler, adım_sayısı, TR_arg_every_minutes: 15.0, TR_arg_dry_run: TR_kw_false)` bir eğitimin kayıt işlerini üstlenir, döngü kullanıcıda kalır (Hugging Face Accelerate'in yolu; Lightning ve Keras döngüyü kendileri sürer). Klasöre `config.json` (`"config"`: ayarlar, `"num_params"`, `"device"`; diske giden anahtarlar dilden bağımsız İngilizce tanımlayıcılardır, dil tanımı 17.3) yazar, `metrics.jsonl` günlüğünü, ilerleme çubuğunu ve kontrol noktasını (dil tanımı 20; `nesneler` ve `"logger"` adıyla günlük) kurar; aynı komut `TR_mem_resume_step`'dan sürer. `adım_bitti(adım, ölçüler)` adım sınırıdır: ölçüler verilirse çubuk ve günlük güncellenir, zamanı geldiyse kontrol noktası yazılır (yazıldıysa `TR_kw_true`). `değerlendirildi(adım, ölçüler)` değerlendirmeyi günlüğe yazar. `bitir(model)` ağırlıkları (`model.trt`) her şeyden önce yazar, sonra son kontrol noktasını: sonrası bir hatayla bitse de eğitimin sonucu kaybolmaz. `TR_arg_dry_run: TR_kw_true` aynı komutun uzun koşudan önceki denemesidir (Lightning `fast_dev_run`): yeni bir geçici klasörde `TR_const_DRY_RUN_STEPS` (2) adım, kontrol noktası her adımda; `TR_mem_finish` bütün dosyaların yazıldığını denetler. Döngü `koşu.TR_mem_resume_step..koşu.TR_mem_last_step` üzerinde döner.

```tr
//! tr kodlu 3 dil=tr
TR_kw_import "tr/egitim"
TR_kw_var model = {w: TR_fn_tensor([1.0, 2.0], TR_arg_requires_grad: TR_kw_true)}
TR_kw_var op = TR_fn_AdamW(TR_fn_parameters(model))
TR_kw_var koşu = TR_type_Run("koşu", {oran: 0.001}, {model: model, op: op}, TR_arg_num_steps: 1000)
TR_kw_for adım TR_kw_in koşu.TR_mem_resume_step..koşu.TR_mem_last_step {
    op.TR_mem_zero_grad()
    TR_kw_var kayıp = TR_fn_sum(model.w * model.w)
    TR_fn_backward(kayıp)
    op.TR_mem_step()
    koşu.TR_mem_step_done(adım + 1, TR_kw_if (adım + 1) % 100 == 0 { {TR_mem_loss: kayıp.TR_mem_tolist} } TR_kw_else { TR_kw_none })
}
koşu.TR_mem_finish(model)
```

GPU'da tam kullanım: [`tools/gpu/gpt_training.tr`](../tools/gpu/gpt_training.tr) (`--prova`).

`TR_type_SpikeGuard(TR_arg_history_limit: 128, TR_arg_threshold: 6.0)` her adımın kaybını ve gradyan normunu izler; `gözle(kayıp, norm)` (sayılar) `TR_const_OK`, `TR_const_NONFINITE` ya da `TR_const_SPIKE` verir. Sıçrama: değerin son `pencere` sağlam adımın ortalamasından `eşik` standart sapma yukarıda olması (OLMo-core `StabilityMonitor`'ın ölçütü; değişmeyen dizide sapma ortalamanın %1'inden küçük alınmaz). Pencere dolana kadar yalnızca sonlu olmayan değer yakalanır; sıçrama ve sonlu olmayan değer geçmişe girmez. `TR_mem_spike_count` sayar, `TR_mem_history_length()` geçmişin boyudur.

Bekçi yalnızca karar verir; ne yapılacağını döngü seçer (dil tanımı 15.36, 20.14, 20.15):

```tr
//! tr kodlu 3 dil=tr
TR_kw_import "tr/egitim"
TR_kw_var w = TR_fn_tensor([1.0, 2.0], TR_arg_requires_grad: TR_kw_true)
TR_kw_var op = TR_fn_AdamW([w])
TR_kw_var ö = TR_fn_dataset("veri").TR_mem_sampler(TR_arg_batch_size: 8, TR_arg_context: 32, TR_arg_seed: 1)
TR_kw_var kn = TR_fn_checkpoint("koşu", {w: w, op: op, ö: ö})
TR_kw_var bekçi = TR_type_SpikeGuard()
TR_kw_var adım = kn.TR_mem_resume_step
TR_kw_for adım < 1000 {
    TR_kw_var giriş, hedef = ö.TR_mem_next()
    op.TR_mem_zero_grad()
    TR_kw_var kayıp = TR_fn_sum(w * w)
    TR_fn_backward(kayıp)
    TR_kw_var norm = TR_fn_clip_grad([w], TR_arg_max: 1.0)
    TR_kw_var karar = bekçi.TR_mem_observe(kayıp.TR_mem_tolist, norm.TR_mem_tolist)
    TR_kw_if karar != TR_const_OK {
        adım = kn.TR_mem_rollback()   // son sağlam kontrol noktası
        ö.TR_mem_skip(200)            // sorunlu toplu işleri atla (PaLM §5.1)
        TR_kw_continue
    }
    op.TR_mem_step()
    adım += 1
    kn.TR_mem_update(adım)
}
```

Tam örnek: [`examples/tr/44_egitim_kararliligi.tr`](../examples/tr/44_egitim_kararliligi.tr); hedef modelde: [`tools/gpu/rgn512.tr`](../tools/gpu/rgn512.tr) (`--koşu`, `--atla`, `--bekçi_pencere`, `--bekçi_eşik`).

## `tr/hazir`

`TR_fn_pretrained_model(klasör, TR_arg_weight_dtype: "float32")` Hugging Face biçiminde kaydedilmiş bir modeli (`config.json` + `model.safetensors`) TR'nin `GPT`'si olarak kurar ve ağırlıklarını yükler; parçalayıcı `TR_fn_load_tokenizer(TR_fn_path(klasör, "tokenizer.json"))` ile ayrıca açılır. `ağırlık_türü` matris ağırlıklarının (gömme ve doğrusal katmanlar) türüdür; normlar ve hesap float32 kalır. Hugging Face modelleri çoğunlukla bf16 saklanır: `TR_arg_weight_dtype: "bf16"` ağırlıkları olduğu gibi tutar (sonuç float32 ağırlıkla aynı matematiktir, bellek yarıya iner; dosyada F32 saklanan ağırlıklar ise bf16'ya yuvarlanır, sonuç float32 yüklemeden biraz ayrılır) ve GPU'da tek parçalık çarpımlar ağırlığı açmadan okur (llama.cpp'nin f16 ağırlık + f32 etkinleşme düzeni). K ve V, kapı ve yukarı tek katman olarak yüklenir (birleşik_kv, birleşik_kapı).

- **Mimariler:** `llama` (Llama, SmolLM2, TinyLlama ...) ve `qwen3`. İkisi de `GPT`'nin iskeletidir (transformers `modeling_llama.py`, `modeling_qwen3.py`): ön-norm blok, RMSNorm, GQA ve yarı döndürmeli RoPE, `aşağı(silu(kapı(x)) · yukarı(x))`; qwen3'te QK-norm.
- **Ayarlar:** `hidden_size` → `boyut`, `num_attention_heads` → `bakış_sayısı`, `num_key_value_heads` → `kv_bakış_sayısı`, `head_dim` → `bakış_boyutu` (yoksa `boyut / bakış_sayısı`), `intermediate_size` → `ara_boyut`, `rms_norm_eps` → `epsilon`, `tie_word_embeddings` → `paylaşımlı`, `rope_theta` (ya da transformers 5.x'te `rope_parameters.rope_theta`) → `TR_fn_rope_theta`. Sapmalı katmanlar, RoPE ölçekleme, kayan pencere ve `silu` dışı etkinleştirme açık hatadır; sessizce yanlış model kurulmaz.
- **Ağırlıklar:** `TR_fn_map_hugging_face` adları TR parametre adlarına çevirir; Hugging Face `nn.Linear` ağırlığı `[çıkış, giriş]`, TR `TR_type_Linear` `[giriş, çıkış]` tuttuğu için doğrusal katmanlar devrilir. Tanınmayan ad hatadır.
- **Doğruluk:** küçük rastgele Llama ve Qwen3 modellerinde transformers'la en büyük logit farkı ~3·10⁻⁷ (`cmd/tr/e2e` `TestReadyModelMatchesTransformers`, `tools/hf/`).

`TR_fn_build_pretrained_model(ayar)` yalnızca kurar (ağırlıksız); `TR_fn_map_hugging_face(ağırlıklar, ayar)` yalnızca eşler.

