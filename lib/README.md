# TR standart kitaplığı

TR ile yazılmış, `tr` ikilisine gömülü kitaplık. Modüller `aktar "tr/..."` ile alınır (dil tanımı [11.6](../docs/spec/11_moduller.md)): `tr/` öneki dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez; kitaplık `tr derle` ile üretilen uygulamaya da girer. Go tarafı tek bir gömme dosyasıdır (`gomulu.go`); kitaplığın kodu bu klasördeki `.tr` dosyalarıdır, okunur ve kopyalanıp değiştirilebilir.

| Modül | Dosya | İçerik |
| :-- | :-- | :-- |
| `tr/katman` | [`katman.tr`](katman.tr) | Transformer katmanları ve yalnızca kod çözücülü `GPT` |
| `tr/iyilestirici` | [`iyilestirici.tr`](iyilestirici.tr) | Muon, NorMuon, parametre grupları, öğrenme oranı planı |

Testler her modülün yanındadır (`katman_test.tr`, `iyilestirici_test.tr`) ve `tr test lib` ile koşar (tam denetimin parçasıdır). Test dosyaları gömülür ama modül olarak aktarılamaz. Kullanım rehberi: [kılavuz §10](../docs/guide/10_hazir_katmanlarla_gpt.md); tam örnek: [`examples/tr/28_gpt.tr`](../examples/tr/28_gpt.tr); GPU'da SimpleStories eğitimi: [`tools/gpu/gpt_training.tr`](../tools/gpu/gpt_training.tr).

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken model = GPT(1000, bağlam: 64, boyut: 64, katman: 2, baş: 4)
değişken op = MuonVeAdamW(model) // gizli 2B ağırlıklar Muon, öbürleri AdamW
değişken giriş = [[1, 2, 3, 4]]
değişken hedef = [[2, 3, 4, 5]]
döngü adım içinde 0..10 {
    op.çarpan_ayarla(oran_planı(adım, 10, ısınma: 2))
    op.sıfırla()
    geri_yayılım(model.kayıp(giriş, hedef))
    op.adım()
}
```

## `tr/katman`

Her katman sıradan bir `yapı`dır: ağırlıklar alanlarındadır, ileri hesap `ileri` metodudur, `parametreler(model)` hepsini bulur (adlar: `bloklar.0.dikkat.sorgu_katmanı.ağırlık`). Girdiler `[toplu, uzunluk, boyut]`, çarpım `x @ W`, Doğrusal'ın ağırlığı `[giriş, çıkış]` şeklindedir.

| Ad | Ne yapar | Kurulum |
| :-- | :-- | :-- |
| `Doğrusal` | `x @ ağırlık (+ sapma)`; sapma çarpımın türünde eklenir (karışık hassasiyette çıktı bf16, PyTorch autocast gibi) | `Doğrusal(giriş, çıkış, sapmalı: yanlış, std: 0.02, tohum: 1)` |
| `GömmeKatmanı` | kimlik → tablo satırı (`gömme` yerleşiği) | `GömmeKatmanı(dağarcık, boyut, std: 0.02, tohum: 1)` |
| `RMSNormKatmanı` | RMS normu, öğrenilen ağırlıkla (`rms_normu` yerleşiği) | `RMSNormKatmanı(boyut, epsilon: 0.00001)` |
| `döner_konum_tablosu`, `döner_konum_uygula` | döner konum gömmesi (RoPE): kosinüs/sinüs tabloları ve uygulanması | `döner_konum_tablosu(bağlam, baş_boyutu, taban: 10000.0)` |
| `NedenselDikkat` | RoPE'lu nedensel çok başlı dikkat, gruplu sorgu (`kv_baş`); birleşik `dikkat` yerleşiği | `NedenselDikkat(boyut, baş, kv_baş: yok, sapmalı: yanlış, std:, çıkış_std:, tohum:, pencere: 0, qk_normu: yanlış, birleşik_kv: yanlış, epsilon:)` |
| `swiglu_ara_boyutu` | SwiGLU ara boyutu: 8/3 · boyut, 32'nin katına yukarı | `swiglu_ara_boyutu(boyut)` |
| `SwiGLU` | kapılı ileri besleme `aşağı(silu(kapı(x)) * yukarı(x))` | `SwiGLU(boyut, ara_boyut: yok, sapmalı:, std:, çıkış_std:, tohum:, birleşik: yanlış)` |
| `PLEBağlamı`, `PLEBloğu`, `ple_gelu` | katman başına kelime gömmesi (PLE, Gemma 3n): ortak bağlam ve katman başına tablo/kapı/izdüşüm | `PLEBağlamı(boyut, katman, ple_boyutu, epsilon:, std:, tohum:)`, `PLEBloğu(boyut, dağarcık, ple_boyutu, epsilon:, std:, çıkış_std:, tohum:)` |
| `TransformerBloğu` | ön-norm blok: `x + dikkat(norm(x))`, `+ ileri_besleme(norm(x))`, (PLE varsa) `+ PLE katkısı` | `TransformerBloğu(boyut, baş, kv_baş:, ara_boyut:, std:, çıkış_std:, tohum:, epsilon:, pencere:, qk_normu:, birleşik_kv:, sapmalı:, ileri_sapmalı:, birleşik_kapı:, ple_boyutu:, dağarcık:)` |
| `katman_pencereleri` | `"YYGYGYYG"` düzeninden katman başına pencere listesi | `katman_pencereleri(düzen, pencereler)` |
| `GPT` | gömme → bloklar → son norm → logit; `ileri`, `kayıp`, `üret` | `GPT(dağarcık, bağlam: 256, boyut: 256, katman: 4, baş: 4, kv_baş: yok, ara_boyut: yok, paylaşımlı: yanlış, tohum: 1, pencereler: yok, qk_normu:, birleşik_kv:, birleşik_kapı:, ileri_sapmalı:, epsilon:, ple_boyutu: yok)` |
| `ağırlık_başlat` | N(0, std) ile başlayan, türev izleyen ağırlık | `ağırlık_başlat(şekil, std, tohum)` |

**Başlatma.** Bütün ağırlık matrisleri N(0, 0.02) ile başlar. Artık yola yazan çıkış katmanları (dikkatin `çıkış_katmanı`, ileri beslemenin `aşağı`sı) `0.02 / √(2 · katman)` ile küçültülür: her blok artık yola iki katkı ekler, katkıların varyansı katman sayısıyla büyümesin. Norm ağırlıkları 1, sapmalar 0 ile başlar. Her ağırlığın sabit bir tohumu vardır (blok `i` için `tohum + 100·(i+1)` ve ardışıkları): aynı `tohum` aynı modeli verir. `paylaşımlı: doğru` çıkışta gömme tablosunu kullanır (`logit = h @ tablo.devrik()`).

**Hedef model seçenekleri (RGN-512).** Aşağıdaki seçeneklerin hepsinin varsayılanı eski davranıştır (`katman_test.tr` varsayılanın değişmediğini sınar). `tools/gpu/rgn512.py`'nin ileri hesabı yeni bir model yapısı gerektirmez, `GPT`nin blok seçenekleriyle kurulur:

```tr
aktar "tr/katman"

değişken rgn = GPT(40000, bağlam: 1024, boyut: 512, katman: 8, baş: 8, kv_baş: 4, ara_boyut: 672, paylaşımlı: doğru,
    pencereler: katman_pencereleri("YYGYGYYG", [32, 64]), qk_normu: doğru, birleşik_kv: doğru,
    birleşik_kapı: doğru, ileri_sapmalı: doğru, epsilon: 0.000001, ple_boyutu: 512)
```

- `pencere` (dikkat/blok): her konum yalnızca son `pencere` konuma bakar (`dikkat(..., pencere:)`); 0 tam nedensel. Pencere katmanın özelliğidir: `GPT`de `pencereler` katman başına bir listedir (G katman 0, Y katman 32/64...), `katman_pencereleri` düzen metninden kurar (Y katmanları pencere listesini sırayla alır).
- `qk_normu`: q ve k, döner konumdan önce baş boyutunda RMS normundan geçer (`q_norm`, `k_norm`; hedefteki `q_norm/k_norm`).
- `birleşik_kv`: anahtar ve değer tek `kv_katmanı` (`[boyut, 2·kv_boyut]`, `[K | V]`, `.böl(2)`); Muon ikisini birlikte dikleştirir. `birleşik_kapı`: SwiGLU'da `[G | U]` tek `kapı_yukarı` matrisi. Birleşikken ayrı katmanlar `yok`'tur (parametre olarak görünmezler).
- `sapmalı` (dikkat) ve `ileri_sapmalı` (SwiGLU): sapma ekler (hedefte yalnızca MLP'nin sapması vardır). `epsilon`: bloğun bütün RMS normlarının epsilon'u (hedef 1e-6; varsayılan 1e-5).
- **PLE** (`ple_boyutu: P`): `PLEBağlamı` gömme çıktısından `RMSNorm_P(x @ W_ctx / √D)` ile katman başına `[B, T, P]` bağlamlarını üretir (`GPT.ple_bağlamı`); her blok, kendi tablosuyla (`bloklar.i.ple.tablo`, `[dağarcık, P]`) `e = (bağlam_i + √P · tablo_i[kelime]) / √2` kurar ve blok sonunda `H ← H + RMSNorm_D(W_p · (GELU(W_g · H) ⊙ e))` ekler (`PLEBloğu`). GELU çağrısı kitaplıkta yalnızca `ple_gelu`dadır: tam (erf) GELU seçeneği gelince tek satır değişir.

**Üretim.** `model.üret(kimlikler, adet, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok, eos: yok)` önbelleksizdir (her parçada bütün diziyi yeniden hesaplar); örnek metin içindir. `eos` bir parça kimliğidir: o parça örneklenince dizide kalır ve üretim `adet` dolmadan durur. Hızlı üretim (KV önbelleği) yol haritasının 9. maddesidir.

## `tr/iyilestirici`

İyileştiriciler TR ile yazılmıştır: parametreyi türev izlemeden yerinde günceller (`türevsiz { p[0..p.şekil[0]] -= ... }`, dil tanımı [15.7](../docs/spec/15_tensorler.md)). Go tarafına iyileştirici eklenmez; yeni bir yöntem için bu dosya kopyalanıp değiştirilir.

| Ad | Ne yapar |
| :-- | :-- |
| `dikleştir(G, adım_sayısı: 5, tür: "float32")` | Newton–Schulz ile G'nin yaklaşık dik eşi (≈ U Vᵀ); katsayılar (3.4445, −4.7750, 2.0315); tekil değerler ≈ [0.7, 1.2]; G bir matris ya da aynı şekilli matrislerin yığını `[k, satır, sütun]` (her matris ayrı, toplu çarpımla); `tür` hesap türü, sonuç G'nin türünde |
| `Muon(ağırlıklar, öğrenme_oranı: 0.02, momentum: 0.95, nesterov: doğru, ağırlık_çürümesi: 0.0, ns_adımı: 5, ölçek: "oran", ns_türü: "float32")` | Nesterov momentumu + dikleştirme; ölçek `"oran"`: √max(1, çıkış/giriş), `"rms"`: 0.2·√max(giriş, çıkış); aynı şekilli ağırlıklar tek yığında dikleştirilir; `ns_türü` dikleştirmenin hesap türü (`"bf16"`); yalnızca 2B ağırlıklar |
| `NorMuon(ağırlıklar, öğrenme_oranı: 0.003, beta1: 0.95, beta2: 0.95, epsilon: 1e-8, ağırlık_çürümesi: 0.0, ns_adımı: 5, ns_türü: "float32")` | Muon + nöron başına ikinci moment normalleştirmesi; güncellemenin RMS'i 0.2·η |
| `parametre_grupları(model, ayrı: ["gömme", "çıkış"])` | `{gizli: [...], diğer: [...]}`: 2B gizli ağırlıklar ile gömme, çıkış, norm ve sapmalar |
| `parametre_kümeleri(model, ayrı:, seyrek: [])` | `{gizli, sönümlü, sönümsüz, seyrek, diğer}`: ada göre dört grup (aşağıda) |
| `ad_uyar_mı(ad, desen)`, `bir_desen_uyar_mı(ad, desenler)` | yol parçası eşleşmesi: tam (`gömme.tablo`), önek (`gömme`), sonek (`ple.tablo`) |
| `SatırRMS(ağırlıklar, öğrenme_oranı: 0.003, beta2: 0.95, epsilon: 1e-8)` | satır seyrek RMS (yoğun maskeli): yalnızca gradyanı sıfırdan farklı satırlar güncellenir; satır başına ikinci moment ve sayaç |
| `MuonVeAdamW(model, muon_oranı: yok, adamw_oranı: 0.003, ağırlık_çürümesi: 0.0, normuon: yanlış, ayrı:, beta2: 0.95, adamw_çürümesi: 0.0, muon_ölçeği: "oran", seyrek: [], seyrek_oranı: yok, ns_türü: "float32")` | gruplar tek iyileştiricide: `adım()`, `sıfırla()`, `çarpan_ayarla(c)`; alanlar `muon`, `adamw`, `adamw_sönümsüz`, `satır_rms` |
| `oran_planı(adım, toplam, ısınma: 0, azalma: 0.2, en_az: 0.0)` | ısınma–sabit–azalma (WSD) öğrenme oranı çarpanı |
| `gradyanları_sıfırla(ağırlıklar)` | gradyanları yerinde sıfırlar |

**Muon.** Kaynak: K. Jordan ve diğ., *Muon: An optimizer for hidden layers in neural networks* (2024, kellerjordan.github.io/posts/muon). Her W için `m ← β·m + (1−β)·g`, Nesterov'la `u ← β·m + (1−β)·g`, `W ← W·(1 − η·λ) − η·√max(1, çıkış/giriş)·dikleştir(u)`. Ölçek, ağırlığı `[çıkış, giriş]` tutan düzendeki √max(1, satır/sütun)'un bu düzendeki karşılığıdır. Aynı şekilli ağırlıklar (`şekil_grupları`) tek yığında dikleştirilir: hedef modelde 56 ayrı dikleştirme yerine 3 toplu dikleştirme (PyTorch'taki yaygın uygulamalar ve `tools/gpu/rgn512.py` de `torch.stack` ile böyle yapar); sonuç tek tek dikleştirmeyle aynıdır (test korur).

**NorMuon.** Kaynak: Z. Li, L. Liu, C. Liang, W. Chen, T. Zhao, *NorMuon: Making Muon more efficient and scalable* (2025, arXiv:2510.05491, Algoritma 1). `M ← β1·M + (1−β1)·g`, `O ← dikleştir(M)`, `v ← β2·v + (1−β2)·ortalama(O², nöron başına)`, `Ô ← O / (√v + ε)`, `η̂ ← 0.2·η·√(m·n) / ‖Ô‖_F`, `W ← W − η·λ·W − η̂·Ô`. Bu düzende bir nöron W'nin bir **sütunudur** (`v` `[1, çıkış]`). Makaledeki gibi Nesterov'suz ve sapma düzeltmesizdir. **Varsayımlar:** ε = 1e-8 (makalede sayı yok); varsayılan öğrenme oranı 0.003 bu kitaplığın seçimidir (oran AdamW ölçeğindedir; makalede 124M'lik modelde 3.6e-4, 1.1B'de 4e-4).

**Ad desenleri ve gruplar.** `ayrı` ve `seyrek` listeleri `parametreler(model, adlarıyla: doğru)` adlarına yol parçası sınırında eşleşir (`ad_uyar_mı`): `"gömme"` adın başı (`gömme.tablo`), `"ple_bağlamı.ctx_katmanı"` başı, `"ple.tablo"` sonu (`bloklar.3.ple.tablo`), `"gömme.tablo"` tamamı. `parametre_kümeleri` her parametreyi bir gruba koyar (öncelik sırası): `seyrek` (desene uyanlar), `sönümsüz` (2B olmayanlar: normler, sapmalar), `sönümlü` (`ayrı`ya uyan 2B ağırlıklar: gömme, `ple_ctx`), `gizli` (öbür 2B ağırlıklar: Muon). `MuonVeAdamW(..., adamw_çürümesi: λ)` λ > 0 iken AdamW'yi ikiye böler: `op.adamw` sönümlü grubu λ ile, `op.adamw_sönümsüz` normları ve sapmaları çürümesiz günceller; λ = 0 (varsayılan) tek çürümesiz `op.adamw`'dır. `seyrek: ["ple.tablo"]` verilirse bu parametreler `op.satır_rms` (`SatırRMS`) iyileştiricisine gider; oranı `seyrek_oranı`dır (yok: `adamw_oranı`). `ağırlık_çürümesi` Muon'un çürümesidir. RGN-512 tarifi: `MuonVeAdamW(model, muon_oranı: 0.002, adamw_oranı: 0.002, ağırlık_çürümesi: 0.1, adamw_çürümesi: 0.1, muon_ölçeği: "rms", ayrı: ["gömme", "ple_bağlamı.ctx_katmanı"], seyrek: ["ple.tablo"])`. Kontrol noktasına `op.adamw`, (varsa) `op.adamw_sönümsüz`, `op.muon.durum` ve (varsa) `op.satır_rms.durum` verilir.

**SatırRMS.** Tablo `[satır, sütun]` (kelime × P). Adımda gradyanı sıfırdan farklı satırlar için `n ← n + 1`, `v ← β2·v + (1−β2)·ortalama(g²)` (satırın elemanları üzerinden), `W ← W − η·g / (√(v / (1−β2ⁿ)) + ε)`; sapma düzeltmesi satırın kendi sayacıyladır (ilk kez görülen kelime doğru ölçekle başlar), gradyanı olmayan satırın ağırlığı, `v`'si ve sayacı değişmez. Momentum yoktur. Durum `op.durum.ikinci_moment` ve `op.durum.sayaç` (`[satır, 1]`) tensörleridir. **Bu sürüm yoğun maskelidir**: gradyan ve durum tam tablo boyundadır, maliyet O(satır · sütun)'dur; anlam seyrek sürümle aynıdır, gerçek seyrek yol (yalnızca geçen satırlara dokunan) yerleşik olarak gelince arayüz değişmez.

**Öğrenme oranı çarpanı.** `çarpan_ayarla(c)` (c pozitif) sonraki adımda iki grubun oranını birlikte ölçekler: `MuonVeAdamW` her adımda `op.muon.öğrenme_oranı`'nı ve `op.adamw.öğrenme_oranı`'nı (dil tanımı [15.39](../docs/spec/15_tensorler.md)) taban oran × c yapar. Kopya ya da ek bellek yoktur; c = 1 iken AdamW'nin adımı planı olmayan AdamW'ninkiyle bit düzeyinde aynıdır (`iyilestirici_test.tr` sınar).

**Kontrol noktası.** Muon'un durum tensörleri `op.muon.durum` sözlüğündedir ve türev izlemez (parametre değildir, yalnızca türevsiz blokta yerinde güncellenir); `kontrol_noktası` bir nesnenin türev izlemeyen tensörlerini de saklayıp yerinde geri yazar (dil tanımı [20.2](../docs/spec/20_kontrol_noktasi.md)). Kontrol noktasına modelin yanında `op.adamw` ve `op.muon.durum` da verilir (örnek: `examples/tr/28_gpt.tr`); sürdürülen eğitim kesintisizle aynıdır (`iyilestirici_test.tr`).

## Adlar ve açık konular

Kitaplığın adları kullanıcı düzeyi TR adlarıdır; dilin kodu değildir, `internal/lang/codes/tr.json`'a girmez. Yerleşiklerle karışmasın diye katmanlar büyük harfli ve ayırt edicidir (`gömme` yerleşiği ↔ `GömmeKatmanı`, `rms_normu` ↔ `RMSNormKatmanı`, `dikkat` ↔ `NedenselDikkat`).

**Açık konu (Ç3, yazım dilleri):** kodlu saklamada (dil tanımı [17](../docs/spec/17_kodlar.md)) kitaplığın adları kullanıcı adı gibi çevrilmeden kalır: İngilizce yazım dilinde de `GPT`, `Doğrusal`, `MuonVeAdamW` yazılır. Kitaplık adlarının çevirisi (kitaplığın kendi ad listesi mi, ayrı bir kod aralığı mı) İngilizce aşamasında ayrıca ele alınacaktır.
