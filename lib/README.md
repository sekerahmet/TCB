# TR standart kitaplığı

TR ile yazılmış, `tr` ikilisine gömülü kitaplık. Modüller `aktar "tr/..."` ile alınır (dil tanımı [11.6](../docs/spec/11_moduller.md)): `tr/` öneki dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez; kitaplık `tr derle` ile üretilen uygulamaya da girer. Go tarafı tek bir gömme dosyasıdır (`gomulu.go`); kitaplığın kodu bu klasördeki `.tr` dosyalarıdır, okunur ve kopyalanıp değiştirilebilir.

| Modül | Dosya | İçerik |
| :-- | :-- | :-- |
| `tr/katman` | [`katman.tr`](katman.tr) | Transformer katmanları ve yalnızca kod çözücülü `GPT` |
| `tr/iyilestirici` | [`iyilestirici.tr`](iyilestirici.tr) | Muon, NorMuon, parametre grupları, öğrenme oranı planı |
| `tr/egitim` | [`egitim.tr`](egitim.tr) | Uzun eğitimin güvenliği: koşunun kayıt işleri (`Koşu`, prova) ve kayıp ile gradyan normu sıçrama bekçisi |
| `tr/hazir` | [`hazir.tr`](hazir.tr) | Hugging Face biçimindeki hazır modelleri (Llama, Qwen3) GPT'ye yüklemek |

Testler her modülün yanındadır (`katman_test.tr`, `iyilestirici_test.tr`, `egitim_test.tr`, `hazir_test.tr`) ve `tr test lib` ile koşar (tam denetimin parçasıdır). Test dosyaları gömülür ama modül olarak aktarılamaz. Kullanım rehberi: [kılavuz §10](../docs/guide/10_hazir_katmanlarla_gpt.md); tam örnek: [`examples/tr/41_gpt.tr`](../examples/tr/41_gpt.tr); GPU'da SimpleStories eğitimi: [`tools/gpu/gpt_training.tr`](../tools/gpu/gpt_training.tr).

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken model = GPT(1000, bağlam: 64, boyut: 64, katman_sayısı: 2, bakış_sayısı: 4)
değişken op = MuonVeAdamW(model) // gizli 2B ağırlıklar Muon, öbürleri AdamW
değişken giriş = [[1, 2, 3, 4]]
değişken hedef = [[2, 3, 4, 5]]
döngü adım içinde 0..10 {
    op.çarpan_ayarla(öğrenme_oranı_planı(adım, 10, ısınma: 2))
    op.sıfırla()
    geri_yayılım(model.kayıp(giriş, hedef))
    op.adım()
}
```

## `tr/katman`

Her katman sıradan bir `yapı`dır: ağırlıklar alanlarındadır, ileri hesap `ileri` metodudur, `parametreler(model)` hepsini bulur (adlar kimlik yoludur: `TR_mem_blocks.0.TR_mem_attention.TR_mem_q_proj.TR_mem_weight`, gösterimde `bloklar.0.dikkat.sorgu_katmanı.ağırlık`). Girdiler `[toplu, uzunluk, boyut]`, çarpım `x @ W`, Doğrusal'ın ağırlığı `[giriş, çıkış]` şeklindedir.

| Ad | Ne yapar | Kurulum |
| :-- | :-- | :-- |
| `Doğrusal` | `x @ ağırlık (+ sapma)`; sapma çarpımın türünde eklenir (karışık hassasiyette çıktı bf16, PyTorch autocast gibi) | `Doğrusal(giriş_boyutu, çıkış_boyutu, sapmalı: yanlış, başlatma_sapması: 0.02, tohum: 1)` |
| `GömmeKatmanı` | kimlik → tablo satırı (`gömme` yerleşiği) | `GömmeKatmanı(dağarcık_büyüklüğü, boyut, başlatma_sapması: 0.02, tohum: 1)` |
| `RMSNormKatmanı` | RMS normu, öğrenilen ağırlıkla (`rms_normu` yerleşiği) | `RMSNormKatmanı(boyut, epsilon: 0.00001)` |
| `döner_konum_tablosu`, `döner_konum_uygula_kaynaşık` | döner konum gömmesi (RoPE): kosinüs/sinüs tabloları ve uygulanması | `döner_konum_tablosu(bağlam, bakış_boyutu, rope_tabanı: 10000.0)` |
| `NedenselDikkat` | RoPE'lu nedensel çok bakışlı dikkat (İngilizce *multi-head attention*; her bakış diziye kendi alt uzayından bakar), gruplu sorgu (`kv_bakış_sayısı`); birleşik `dikkat` yerleşiği | `NedenselDikkat(boyut, bakış_sayısı, kv_bakış_sayısı: yok, sapmalı: yanlış, std:, çıkış_std:, tohum:, pencere: 0, qk_normu: yanlış, birleşik_kv: yanlış, epsilon:, bakış_boyutu: yok)` |
| `swiglu_ara_boyutu` | SwiGLU ara boyutu: 8/3 · boyut, 32'nin katına yukarı | `swiglu_ara_boyutu(boyut)` |
| `SwiGLU` | kapılı ileri besleme `aşağı(silu(kapı(x)) * yukarı(x))` | `SwiGLU(boyut, ara_boyut: yok, sapmalı:, başlatma_sapması:, çıkış_başlatma_sapması:, tohum:, birleşik: yanlış)` |
| `PLEBağlamı`, `PLEBloğu`, `ple_gelu` | katman başına kelime gömmesi (PLE, Gemma 3n): ortak bağlam ve katman başına tablo/kapı/izdüşüm | `PLEBağlamı(boyut, katman_sayısı, ple_boyutu, epsilon:, başlatma_sapması:, tohum:)`, `PLEBloğu(boyut, dağarcık, ple_boyutu, epsilon:, std:, çıkış_std:, tohum:)` |
| `TransformerBloğu` | ön-norm blok: `x + dikkat(norm(x))`, `+ ileri_besleme(norm(x))`, (PLE varsa) `+ PLE katkısı` | `TransformerBloğu(boyut, bakış_sayısı, kv_bakış_sayısı:, ara_boyut:, başlatma_sapması:, çıkış_başlatma_sapması:, tohum:, epsilon:, pencere:, qk_normu:, birleşik_kv:, sapmalı:, ileri_sapmalı:, birleşik_kapı:, ple_boyutu:, dağarcık:, bakış_boyutu:)` |
| `katman_pencereleri` | `"YYGYGYYG"` düzeninden katman başına pencere listesi | `katman_pencereleri(düzen, pencereler)` |
| `GPT` | gömme → bloklar → son norm → logit; `ileri`, `kayıp`, `üret` | `GPT(dağarcık_büyüklüğü, bağlam: 256, boyut: 256, katman_sayısı: 4, bakış_sayısı: 4, kv_bakış_sayısı: yok, ara_boyut: yok, paylaşımlı: yanlış, tohum: 1, pencereler: yok, qk_normu:, birleşik_kv:, birleşik_kapı:, ileri_sapmalı:, epsilon:, ple_boyutu: yok, bakış_boyutu: yok, rope_tabanı: 10000.0)` |
| `ağırlık_başlat` | N(0, std) ile başlayan, türev izleyen ağırlık | `ağırlık_başlat(şekil, başlatma_sapması, tohum)` |

**Başlatma.** Bütün ağırlık matrisleri N(0, 0.02) ile başlar. Artık yola yazan çıkış katmanları (dikkatin `çıkış_katmanı`, ileri beslemenin `aşağı`sı) `0.02 / √(2 · katman)` ile küçültülür: her blok artık yola iki katkı ekler, katkıların varyansı katman sayısıyla büyümesin. Norm ağırlıkları 1, sapmalar 0 ile başlar. Her ağırlığın sabit bir tohumu vardır (blok `i` için `tohum + 100·(i+1)` ve ardışıkları): aynı `tohum` aynı modeli verir. `paylaşımlı: doğru` çıkışta gömme tablosunu kullanır (`logit = h @ tablo.devrik()`).

**Hedef model seçenekleri (RGN-512).** Aşağıdaki seçeneklerin hepsinin varsayılanı eski davranıştır (`katman_test.tr` varsayılanın değişmediğini sınar). `tools/gpu/rgn512.py`'nin ileri hesabı yeni bir model yapısı gerektirmez, `GPT`nin blok seçenekleriyle kurulur:

```tr
aktar "tr/katman"

değişken rgn = GPT(40000, bağlam: 1024, boyut: 512, katman_sayısı: 8, bakış_sayısı: 8, kv_bakış_sayısı: 4, ara_boyut: 672, paylaşımlı: doğru,
    pencereler: katman_pencereleri("YYGYGYYG", [32, 64]), qk_normu: doğru, birleşik_kv: doğru,
    birleşik_kapı: doğru, ileri_sapmalı: doğru, epsilon: 0.000001, ple_boyutu: 512)
```

- `pencere` (dikkat/blok): her konum yalnızca son `pencere` konuma bakar (`dikkat(..., pencere:)`); 0 tam nedensel. Pencere katmanın özelliğidir: `GPT`de `pencereler` katman başına bir listedir (G katman 0, Y katman 32/64...), `katman_pencereleri` düzen metninden kurar (Y katmanları pencere listesini sırayla alır).
- `bakış_boyutu` (dikkat/blok/GPT): bir bakışın boyutu; verilmezse `boyut / bakış_sayısı`. Verilirse gizli boyuttan bağımsızdır (Hugging Face `head_dim`; Qwen3-0.6B: gizli 1024, 16 bakış × 128): sorgu katmanı `[boyut, bakış_sayısı · bakış_boyutu]`, çıkış katmanı `[bakış_sayısı · bakış_boyutu, boyut]`, döner konum tabloları bu boydadır; çift olmalıdır.
- `qk_normu`: q ve k, döner konumdan önce bakış boyutunda RMS normundan geçer (`q_norm`, `k_norm`; hedefteki `q_norm/k_norm`).
- `birleşik_kv`: anahtar ve değer tek `kv_katmanı` (`[boyut, 2·kv_boyut]`, `[K | V]`, `.böl(2)`); Muon ikisini birlikte dikleştirir. `birleşik_kapı`: SwiGLU'da `[G | U]` tek `kapı_yukarı` matrisi. Birleşikken ayrı katmanlar `yok`'tur (parametre olarak görünmezler).
- `sapmalı` (dikkat) ve `ileri_sapmalı` (SwiGLU): sapma ekler (hedefte yalnızca MLP'nin sapması vardır). `epsilon`: bloğun bütün RMS normlarının epsilon'u (hedef 1e-6; varsayılan 1e-5).
- **PLE** (`ple_boyutu: P`): `PLEBağlamı` gömme çıktısından `RMSNorm_P(x @ W_ctx / √D)` ile katman başına `[B, T, P]` bağlamlarını üretir (`GPT.ple_bağlamı`); her blok, kendi tablosuyla (`bloklar.i.ple.tablo`, `[dağarcık, P]`) `e = (bağlam_i + √P · tablo_i[kelime]) / √2` kurar ve blok sonunda `H ← H + RMSNorm_D(W_p · (GELU(W_g · H) ⊙ e))` ekler (`PLEBloğu`). GELU çağrısı kitaplıkta yalnızca `ple_gelu`dadır: tam (erf) GELU seçeneği gelince tek satır değişir.

**Üretim.** `model.üret(kimlikler, en_fazla_yeni_parça, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok, eos: yok, önbellek_çeşidi: "sabit")` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, sonraki her adım yalnızca yeni parçayı hesaplar ve yalnızca son konumun logitini çıkarır (transformers `generate`'in `logits_to_keep`'i gibi). `eos` bir parça kimliğidir: o parça örneklenince dizide kalır ve üretim `adet` dolmadan durur. Dizi bağlamı doldurunca son `bağlam` parça yeniden işlenir. GPU'da sabit önbellekle (`çizge: doğru`, varsayılan) her adım `GPT.tek_adım`'dır ve `tekrar_oynat` ile kaydedilip oynatılır (dil tanımı 15.45): konum ve parça kimliği cihazda bir tensördür, yeni anahtarlar `yerine_yaz` ile konuma yazılır, dikkat tamponun tamamına maskeyle bakar (transformers sabit önbelleği ve `cache_position` ile aynı yol); yorumlayıcı adımı yeniden çalıştırmaz. Sonuç kayıtsızla aynı parçalardır; `çizge: yanlış` kapatır. Sabit önbellek en çok başlangıç + adet konum tutar.
- **`KVÖnbellek`** (`model.önbellek_kur(çeşit: "sabit", kapasite: bağlam)`): katman başına döner konumlu anahtar ve değerler, `[B, T, kv_bakış, bakış_boyutu]` düzende (dikkat'in düzeni; GQA'da çoğaltılmaz). `"sabit"` tamponu bir kez ayırır ve yeni anahtarları zaman ekseninde yerine yazar (gpt-fast, FlashAttention `flash_attn_with_kvcache` gibi); `"büyüyen"` her adımda `bitiştir` ile uzar (transformers `DynamicCache` gibi). `model.ileri(kimlikler, önbellek: ö)` önbellekteki konumdan devam eder: döner konum tabloları o konumdan dilimlenir, `dikkat` önbellekteki bütün konumlara bakar (nedensellik sondan hizalı, dil tanımı 15.24). `ö.kırp(n)` önbelleği ilk `n` parçaya indirir, sonrakileri geri alır (reddedilen taslak). Önbellek türev izlemez.
- **Ölçüm (SmolLM2-135M, CPU, 100 parça, açgözlü, 4 tur ortancası, 2026-10-21):** önbelleksiz 1,7; büyüyen 23,6; sabit 24,7 parça/sn; transformers `generate` 23,2. Colab (RTX PRO 6000, 3 tur): GPU'da sabit ~216, büyüyen ~201, transformers ~109; Colab işlemcisinde (24 iş parçacığı) TR ~38, transformers ~37 parça/sn. Önbellekli ve önbelleksiz sonuç aynıdır (`lib/katman_test.tr`).

**`tek_adım` ve `gizli_temsil`.** Kendi üretim döngünüz için iki yapı taşı: `model.gizli_temsil(kimlikler, önbellek: ö)` kimlikler `[B, T]` (liste) için gizli durumu `[B, T, boyut]` verir ve önbelleğe yazar (`ö.konum` T kadar ilerler); `model.logit(gizli)` onu dağarcık logitine çevirir (üretimde yalnızca son konum için: `model.logit(x.dilim(1, T - 1, T))[0][0]`). `model.tek_adım(kimlik, konum, ö)` bir parçalık adımdır: `kimlik` `[1, 1]` ve `konum` `[1]` **tensörleri** (konum cihazdadır, bu yüzden adım `tekrar_oynat` ile kaydedilebilir), `ö` **sabit** önbellek olmalı ve önceki parçalar `gizli_durum` ile işlenmiş olmalıdır; son konumun logitini `[dağarcık]` verir. Konumu çağıran ilerletir: `ö.ilerle(1)`. CPU'da `tekrar_oynat` adımı doğrudan çağırır. Örnek: `examples/tr/47_kendi_uretim_dongusu.tr` (elle yazılan döngü `model.üret` ile aynı diziyi verir).

## `tr/iyilestirici`

İyileştiriciler TR ile yazılmıştır: parametreyi türev izlemeden yerinde günceller (`türevsiz { p[0..p.şekil[0]] -= ... }`, dil tanımı [15.34](../docs/spec/15_tensorler.md)). Go tarafına iyileştirici eklenmez; yeni bir yöntem için bu dosya kopyalanıp değiştirilir.

| Ad | Ne yapar |
| :-- | :-- |
| `dikleştir(G, ns_yineleme_sayısı: 5, ns_türü: "float32")` | Newton–Schulz ile G'nin yaklaşık dik eşi (≈ U Vᵀ); katsayılar (3.4445, −4.7750, 2.0315); tekil değerler ≈ [0.7, 1.2]; G bir matris ya da aynı şekilli matrislerin yığını `[k, satır, sütun]` (her matris ayrı, toplu çarpımla); `tür` hesap türü, sonuç G'nin türünde |
| `Muon(parametreler, öğrenme_oranı: 0.02, momentum: 0.95, nesterov: doğru, ağırlık_çürümesi: 0.0, ns_yineleme_sayısı: 5, ölçek: "oran", ns_türü: "float32")` | Nesterov momentumu + dikleştirme; ölçek `"oran"`: √max(1, çıkış/giriş), `"rms"`: 0.2·√max(giriş, çıkış); aynı şekilli ağırlıklar tek yığında dikleştirilir; `ns_türü` dikleştirmenin hesap türü (`"bf16"`); yalnızca 2B ağırlıklar |
| `NorMuon(parametreler, öğrenme_oranı: 0.003, beta1: 0.95, beta2: 0.95, epsilon: 1e-8, ağırlık_çürümesi: 0.0, ns_yineleme_sayısı: 5, ns_türü: "float32")` | Muon + nöron başına ikinci moment normalleştirmesi; güncellemenin RMS'i 0.2·η |
| `parametre_grupları(model, ayrılan: ["TR_mem_embedding", "TR_mem_logit_layer"])` | `{gizli: [...], diğer: [...]}`: 2B gizli ağırlıklar ile gömme, çıkış, norm ve sapmalar |
| `parametre_kümeleri(model, ayrı:, seyrek: [])` | `{gizli, sönümlü, sönümsüz, seyrek, diğer}`: ada göre dört grup (aşağıda) |
| `ad_uyar_mı(ad, desen)`, `bir_desen_uyar_mı(ad, desenler)` | yol parçası eşleşmesi: tam (`gömme.tablo`), önek (`gömme`), sonek (`ple.tablo`) |
| `SatırRMS(parametreler, öğrenme_oranı: 0.003, beta2: 0.95, epsilon: 1e-8)` | satır seyrek RMS (yoğun maskeli): yalnızca gradyanı sıfırdan farklı satırlar güncellenir; satır başına ikinci moment ve sayaç |
| `MuonVeAdamW(model, muon_öğrenme_oranı: yok, adamw_öğrenme_oranı: 0.003, ağırlık_çürümesi: 0.0, normuon: yanlış, ayrılan:, beta2: 0.95, adamw_çürümesi: 0.0, muon_ölçeği: "oran", seyrek: [], seyrek_öğrenme_oranı: yok, ns_türü: "float32")` | gruplar tek iyileştiricide: `adım()`, `sıfırla()`, `çarpan_ayarla(c)`; alanlar `muon`, `adamw`, `adamw_sönümsüz`, `satır_rms` |
| `öğrenme_oranı_planı(adım, adım_sayısı, ısınma: 0, azalma: 0.2, en_az: 0.0)` | ısınma–sabit–azalma (WSD) öğrenme oranı çarpanı |
| `gradyanları_sıfırla(parametreler)` | gradyanları yerinde sıfırlar |

**Muon.** Kaynak: K. Jordan ve diğ., *Muon: An optimizer for hidden layers in neural networks* (2024, kellerjordan.github.io/posts/muon). Her W için `m ← β·m + (1−β)·g`, Nesterov'la `u ← β·m + (1−β)·g`, `W ← W·(1 − η·λ) − η·√max(1, çıkış/giriş)·dikleştir(u)`. Ölçek, ağırlığı `[çıkış, giriş]` tutan düzendeki √max(1, satır/sütun)'un bu düzendeki karşılığıdır. Aynı şekilli ağırlıklar (`şekil_grupları`) tek yığında dikleştirilir: hedef modelde 56 ayrı dikleştirme yerine 3 toplu dikleştirme (PyTorch'taki yaygın uygulamalar ve `tools/gpu/rgn512.py` de `torch.stack` ile böyle yapar); sonuç tek tek dikleştirmeyle aynıdır (test korur).

**NorMuon.** Kaynak: Z. Li, L. Liu, C. Liang, W. Chen, T. Zhao, *NorMuon: Making Muon more efficient and scalable* (2025, arXiv:2510.05491, Algoritma 1). `M ← β1·M + (1−β1)·g`, `O ← dikleştir(M)`, `v ← β2·v + (1−β2)·ortalama(O², nöron başına)`, `Ô ← O / (√v + ε)`, `η̂ ← 0.2·η·√(m·n) / ‖Ô‖_F`, `W ← W − η·λ·W − η̂·Ô`. Bu düzende bir nöron W'nin bir **sütunudur** (`v` `[1, çıkış]`). Makaledeki gibi Nesterov'suz ve sapma düzeltmesizdir. **Varsayımlar:** ε = 1e-8 (makalede sayı yok); varsayılan öğrenme oranı 0.003 bu kitaplığın seçimidir (oran AdamW ölçeğindedir; makalede 124M'lik modelde 3.6e-4, 1.1B'de 4e-4).

**Ad desenleri ve gruplar.** `ayrılan` ve `seyrek` listeleri `parametreler(model, adlarıyla: doğru)` adlarına yol parçası sınırında eşleşir (`ad_uyar_mı`): adlar ve desenler kimlik yoludur: `"TR_mem_embedding"` adın başı (`TR_mem_embedding.TR_mem_table`), `"TR_mem_ple_context.TR_mem_context_proj"` başı, `"TR_mem_ple.TR_mem_table"` sonu (`TR_mem_blocks.3.TR_mem_ple.TR_mem_table`), `"TR_mem_embedding.TR_mem_table"` tamamı. `parametre_kümeleri` her parametreyi bir gruba koyar (öncelik sırası): `seyrek` (desene uyanlar), `sönümsüz` (2B olmayanlar: normler, sapmalar), `sönümlü` (`ayrılan`a uyan 2B ağırlıklar: gömme, `ple_ctx`), `gizli` (öbür 2B ağırlıklar: Muon). `MuonVeAdamW(..., adamw_çürümesi: λ)` λ > 0 iken AdamW'yi ikiye böler: `op.adamw` sönümlü grubu λ ile, `op.adamw_sönümsüz` normları ve sapmaları çürümesiz günceller; λ = 0 (varsayılan) tek çürümesiz `op.adamw`'dır. `seyrek: ["TR_mem_ple.TR_mem_table"]` verilirse bu parametreler `op.satır_rms` (`SatırRMS`) iyileştiricisine gider; oranı `seyrek_öğrenme_oranı`dır (yok: `adamw_öğrenme_oranı`). `ağırlık_çürümesi` Muon'un çürümesidir. RGN-512 tarifi: `MuonVeAdamW(model, muon_öğrenme_oranı: 0.002, adamw_öğrenme_oranı: 0.002, ağırlık_çürümesi: 0.1, adamw_çürümesi: 0.1, muon_ölçeği: "rms", ayrılan: ["TR_mem_embedding", "TR_mem_ple_context.TR_mem_context_proj"], seyrek: ["TR_mem_ple.TR_mem_table"])`. Kontrol noktasına `op.adamw`, (varsa) `op.adamw_sönümsüz`, `op.muon.durum` ve (varsa) `op.satır_rms.durum` verilir.

**SatırRMS.** Tablo `[satır, sütun]` (kelime × P). Adımda gradyanı sıfırdan farklı satırlar için `n ← n + 1`, `v ← β2·v + (1−β2)·ortalama(g²)` (satırın elemanları üzerinden), `W ← W − η·g / (√(v / (1−β2ⁿ)) + ε)`; sapma düzeltmesi satırın kendi sayacıyladır (ilk kez görülen kelime doğru ölçekle başlar), gradyanı olmayan satırın ağırlığı, `v`'si ve sayacı değişmez. Momentum yoktur. Durum `op.durum.ikinci_moment` ve `op.durum.sayaç` (`[satır, 1]`) tensörleridir. **Bu sürüm yoğun maskelidir**: gradyan ve durum tam tablo boyundadır, maliyet O(satır · sütun)'dur; anlam seyrek sürümle aynıdır, gerçek seyrek yol (yalnızca geçen satırlara dokunan) yerleşik olarak gelince arayüz değişmez.

**Öğrenme oranı çarpanı.** `çarpan_ayarla(c)` (c pozitif) sonraki adımda iki grubun oranını birlikte ölçekler: `MuonVeAdamW` her adımda `op.muon.öğrenme_oranı`'nı ve `op.adamw.öğrenme_oranı`'nı (dil tanımı [15.35](../docs/spec/15_tensorler.md)) taban oran × c yapar. Kopya ya da ek bellek yoktur; c = 1 iken AdamW'nin adımı planı olmayan AdamW'ninkiyle bit düzeyinde aynıdır (`iyilestirici_test.tr` sınar).

**Kontrol noktası.** Muon'un durum tensörleri `op.muon.durum` sözlüğündedir ve türev izlemez (parametre değildir, yalnızca türevsiz blokta yerinde güncellenir); `kontrol_noktası` bir nesnenin türev izlemeyen tensörlerini de saklayıp yerinde geri yazar (dil tanımı [20.2](../docs/spec/20_kontrol_noktasi.md)). Kontrol noktasına modelin yanında `op.adamw` ve `op.muon.durum` da verilir (örnek: `examples/tr/41_gpt.tr`); sürdürülen eğitim kesintisizle aynıdır (`iyilestirici_test.tr`).

## Adlar ve açık konular

Kitaplığın adları kullanıcı düzeyi TR adlarıdır; dilin kodu değildir, `internal/lang/codes/tr.json`'a girmez. Yerleşiklerle karışmasın diye katmanlar büyük harfli ve ayırt edicidir (`gömme` yerleşiği ↔ `GömmeKatmanı`, `rms_normu` ↔ `RMSNormKatmanı`, `dikkat` ↔ `NedenselDikkat`).

**Yazım dilleri.** Kitaplığın dili yoktur: dışa açık her adı (yapı, fonksiyon, sabit, alan, metot, parametre) bir koddur (dil tanımı [17](../docs/spec/17_kodlar.md); `definitions.json` dosyasında `modül` alanıyla) ve her yazım dilinde o dilin adıyla görünür: `Doğrusal` / `Linear` (`TR_type_Linear`), `MuonVeAdamW` / `MuonAndAdamW`. Ağırlık adları da kimlik yoludur (`TR_mem_blocks.0.TR_mem_attention.TR_mem_q_proj.TR_mem_weight`). Yerel değişkenler, yorumlar ve metinler çevrilmez.

## `tr/egitim`

`Koşu(klasör, ayar, nesneler, adım_sayısı, her_dakika: 15.0, prova: yanlış)` bir eğitimin kayıt işlerini üstlenir, döngü kullanıcıda kalır (Hugging Face Accelerate'in yolu; Lightning ve Keras döngüyü kendileri sürer). Klasöre `config.json` (ayarlar, parametre sayısı, cihaz) yazar, `metrics.jsonl` günlüğünü, ilerleme çubuğunu ve kontrol noktasını (dil tanımı 20; `nesneler` ve günlük) kurar; aynı komut `kaldığı_adım`'dan sürer. `adım_bitti(adım, ölçüler)` adım sınırıdır: ölçüler verilirse çubuk ve günlük güncellenir, zamanı geldiyse kontrol noktası yazılır (yazıldıysa `doğru`). `değerlendirildi(adım, ölçüler)` değerlendirmeyi günlüğe yazar. `bitir(model)` ağırlıkları (`model.trt`) her şeyden önce yazar, sonra son kontrol noktasını: sonrası bir hatayla bitse de eğitimin sonucu kaybolmaz. `prova: doğru` aynı komutun uzun koşudan önceki denemesidir (Lightning `fast_dev_run`): yeni bir geçici klasörde `PROVA_ADIMI` (2) adım, kontrol noktası her adımda; `bitir` bütün dosyaların yazıldığını denetler. Döngü `koşu.kaldığı_adım..koşu.son_adım` üzerinde döner.

```tr
aktar "tr/egitim"
değişken model = {w: tensör([1.0, 2.0], türev_izle: doğru)}
değişken op = AdamW(parametreler(model))
değişken koşu = Koşu("koşu", {oran: 0.001}, {model: model, op: op}, adım_sayısı: 1000)
döngü adım içinde koşu.kaldığı_adım..koşu.son_adım {
    op.sıfırla()
    değişken kayıp = toplam(model.w * model.w)
    geri_yayılım(kayıp)
    op.adım()
    koşu.adım_bitti(adım + 1, eğer (adım + 1) % 100 == 0 { {kayıp: kayıp.değer} } değilse { yok })
}
koşu.bitir(model)
```

GPU'da tam kullanım: [`tools/gpu/gpt_training.tr`](../tools/gpu/gpt_training.tr) (`--prova`).

`SıçramaBekçisi(geçmiş_sınırı: 128, eşik: 6.0)` her adımın kaybını ve gradyan normunu izler; `gözle(kayıp, norm)` (sayılar) `TAMAM`, `SONLU_DEĞİL` ya da `SIÇRAMA` verir. Sıçrama: değerin son `pencere` sağlam adımın ortalamasından `eşik` standart sapma yukarıda olması (OLMo-core `StabilityMonitor`'ın ölçütü; değişmeyen dizide sapma ortalamanın %1'inden küçük alınmaz). Pencere dolana kadar yalnızca sonlu olmayan değer yakalanır; sıçrama ve sonlu olmayan değer geçmişe girmez. `sıçrama_sayısı` sayar, `geçmiş_uzunluğu()` geçmişin boyudur.

Bekçi yalnızca karar verir; ne yapılacağını döngü seçer (dil tanımı 15.36, 20.14, 20.15):

```tr
aktar "tr/egitim"
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = AdamW([w])
değişken ö = veri_kümesi("veri").örnekleyici(toplu: 8, bağlam: 32, tohum: 1)
değişken kn = kontrol_noktası("koşu", {w: w, op: op, ö: ö})
değişken bekçi = SıçramaBekçisi()
değişken adım = kn.kaldığı_adım
döngü adım < 1000 {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = toplam(w * w)
    geri_yayılım(kayıp)
    değişken norm = gradyan_kırp([w], en_fazla: 1.0)
    değişken karar = bekçi.gözle(kayıp.değer, norm.değer)
    eğer karar != TAMAM {
        adım = kn.geri_dön()   // son sağlam kontrol noktası
        ö.atla(200)            // sorunlu toplu işleri atla (PaLM §5.1)
        devam
    }
    op.adım()
    adım += 1
    kn.güncelle(adım)
}
```

Tam örnek: [`examples/tr/44_egitim_kararliligi.tr`](../examples/tr/44_egitim_kararliligi.tr); hedef modelde: [`tools/gpu/rgn512.tr`](../tools/gpu/rgn512.tr) (`--koşu`, `--atla`, `--bekçi_pencere`, `--bekçi_eşik`).

## `tr/hazir`

`hazır_model(klasör, ağırlık_türü: "float32")` Hugging Face biçiminde kaydedilmiş bir modeli (`config.json` + `model.safetensors`) TR'nin `GPT`'si olarak kurar ve ağırlıklarını yükler; parçalayıcı `parçalayıcı_yükle(yol(klasör, "tokenizer.json"))` ile ayrıca açılır. `ağırlık_türü` matris ağırlıklarının (gömme ve doğrusal katmanlar) türüdür; normlar ve hesap float32 kalır. Hugging Face modelleri çoğunlukla bf16 saklanır: `ağırlık_türü: "bf16"` ağırlıkları olduğu gibi tutar (sonuç float32 ağırlıkla aynı matematiktir, bellek yarıya iner; dosyada F32 saklanan ağırlıklar ise bf16'ya yuvarlanır, sonuç float32 yüklemeden biraz ayrılır) ve GPU'da tek parçalık çarpımlar ağırlığı açmadan okur (llama.cpp'nin f16 ağırlık + f32 etkinleşme düzeni). K ve V, kapı ve yukarı tek katman olarak yüklenir (birleşik_kv, birleşik_kapı).

- **Mimariler:** `llama` (Llama, SmolLM2, TinyLlama ...) ve `qwen3`. İkisi de `GPT`'nin iskeletidir (transformers `modeling_llama.py`, `modeling_qwen3.py`): ön-norm blok, RMSNorm, GQA ve yarı döndürmeli RoPE, `aşağı(silu(kapı(x)) · yukarı(x))`; qwen3'te QK-norm.
- **Ayarlar:** `hidden_size` → `boyut`, `num_attention_heads` → `bakış_sayısı`, `num_key_value_heads` → `kv_bakış_sayısı`, `head_dim` → `bakış_boyutu` (yoksa `boyut / bakış_sayısı`), `intermediate_size` → `ara_boyut`, `rms_norm_eps` → `epsilon`, `tie_word_embeddings` → `paylaşımlı`, `rope_theta` (ya da transformers 5.x'te `rope_parameters.rope_theta`) → `rope_tabanı`. Sapmalı katmanlar, RoPE ölçekleme, kayan pencere ve `silu` dışı etkinleştirme açık hatadır; sessizce yanlış model kurulmaz.
- **Ağırlıklar:** `hugging_face_eşle` adları TR parametre adlarına çevirir; Hugging Face `nn.Linear` ağırlığı `[çıkış, giriş]`, TR `Doğrusal` `[giriş, çıkış]` tuttuğu için doğrusal katmanlar devrilir. Tanınmayan ad hatadır.
- **Doğruluk:** küçük rastgele Llama ve Qwen3 modellerinde transformers'la en büyük logit farkı ~3·10⁻⁷ (`cmd/tr/e2e` `TestReadyModelMatchesTransformers`, `tools/hf/`).

`hazır_model_kur(ayar)` yalnızca kurar (ağırlıksız); `hugging_face_eşle(ağırlıklar, ayar)` yalnızca eşler.

