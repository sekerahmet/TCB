# TR standart kitaplığı

TR ile yazılmış, `tr` ikilisine gömülü kitaplık. Modüller `aktar "tr/..."` ile alınır (dil tanımı [11.6](../docs/dil/11_moduller.md)): `tr/` öneki dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez; kitaplık `tr derle` ile üretilen uygulamaya da girer. Go tarafı tek bir gömme dosyasıdır (`gomulu.go`); kitaplığın kodu bu klasördeki `.tr` dosyalarıdır, okunur ve kopyalanıp değiştirilebilir.

| Modül | Dosya | İçerik |
| :-- | :-- | :-- |
| `tr/katman` | [`katman.tr`](katman.tr) | Transformer katmanları ve yalnızca kod çözücülü `GPT` |
| `tr/iyilestirici` | [`iyilestirici.tr`](iyilestirici.tr) | Muon, NorMuon, parametre grupları, öğrenme oranı planı |

Testler her modülün yanındadır (`katman_test.tr`, `iyilestirici_test.tr`) ve `tr test kitaplik` ile koşar (tam denetimin parçasıdır). Test dosyaları gömülür ama modül olarak aktarılamaz. Kullanım rehberi: [kılavuz §10](../docs/kilavuz/10_hazir_katmanlarla_gpt.md); tam örnek: [`ornekler/11_gpt.tr`](../ornekler/11_gpt.tr); GPU'da SimpleStories eğitimi: [`tools/gpu/gpt_egitim.tr`](../tools/gpu/gpt_egitim.tr).

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
| `Doğrusal` | `x @ ağırlık (+ sapma)` | `Doğrusal(giriş, çıkış, sapmalı: yanlış, std: 0.02, tohum: 1)` |
| `GömmeKatmanı` | kimlik → tablo satırı (`gömme` yerleşiği) | `GömmeKatmanı(dağarcık, boyut, std: 0.02, tohum: 1)` |
| `RMSNormKatmanı` | RMS normu, öğrenilen ağırlıkla (`rms_normu` yerleşiği) | `RMSNormKatmanı(boyut, epsilon: 0.00001)` |
| `döner_konum_tablosu`, `döner_konum_uygula` | döner konum gömmesi (RoPE): kosinüs/sinüs tabloları ve uygulanması | `döner_konum_tablosu(bağlam, baş_boyutu, taban: 10000.0)` |
| `NedenselDikkat` | RoPE'lu nedensel çok başlı dikkat, gruplu sorgu (`kv_baş`); birleşik `dikkat` yerleşiği | `NedenselDikkat(boyut, baş, kv_baş: yok, sapmalı: yanlış, std:, çıkış_std:, tohum:)` |
| `swiglu_ara_boyutu` | SwiGLU ara boyutu: 8/3 · boyut, 32'nin katına yukarı | `swiglu_ara_boyutu(boyut)` |
| `SwiGLU` | kapılı ileri besleme `aşağı(silu(kapı(x)) * yukarı(x))` | `SwiGLU(boyut, ara_boyut: yok, sapmalı:, std:, çıkış_std:, tohum:)` |
| `TransformerBloğu` | ön-norm blok: `x + dikkat(norm(x))`, `+ ileri_besleme(norm(x))` | `TransformerBloğu(boyut, baş, kv_baş:, ara_boyut:, std:, çıkış_std:, tohum:)` |
| `GPT` | gömme → bloklar → son norm → logit; `ileri`, `kayıp`, `üret` | `GPT(dağarcık, bağlam: 256, boyut: 256, katman: 4, baş: 4, kv_baş: yok, ara_boyut: yok, paylaşımlı: yanlış, tohum: 1)` |
| `ağırlık_başlat` | N(0, std) ile başlayan, türev izleyen ağırlık | `ağırlık_başlat(şekil, std, tohum)` |

**Başlatma.** Bütün ağırlık matrisleri N(0, 0.02) ile başlar. Artık yola yazan çıkış katmanları (dikkatin `çıkış_katmanı`, ileri beslemenin `aşağı`sı) `0.02 / √(2 · katman)` ile küçültülür: her blok artık yola iki katkı ekler, katkıların varyansı katman sayısıyla büyümesin. Norm ağırlıkları 1, sapmalar 0 ile başlar. Her ağırlığın sabit bir tohumu vardır (blok `i` için `tohum + 100·(i+1)` ve ardışıkları): aynı `tohum` aynı modeli verir. `paylaşımlı: doğru` çıkışta gömme tablosunu kullanır (`logit = h @ tablo.devrik()`).

**Üretim.** `model.üret(kimlikler, adet, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok)` önbelleksizdir (her parçada bütün diziyi yeniden hesaplar); örnek metin içindir. Hızlı üretim (KV önbelleği) yol haritasının 9. maddesidir.

## `tr/iyilestirici`

İyileştiriciler TR ile yazılmıştır: parametreyi türev izlemeden yerinde günceller (`türevsiz { p[0..p.şekil[0]] -= ... }`, dil tanımı [15.7](../docs/dil/15_tensorler.md)). Go tarafına iyileştirici eklenmez; yeni bir yöntem için bu dosya kopyalanıp değiştirilir.

| Ad | Ne yapar |
| :-- | :-- |
| `dikleştir(G, adım_sayısı: 5)` | Newton–Schulz ile G'nin yaklaşık dik eşi (≈ U Vᵀ); katsayılar (3.4445, −4.7750, 2.0315); tekil değerler ≈ [0.7, 1.2] |
| `Muon(ağırlıklar, öğrenme_oranı: 0.02, momentum: 0.95, nesterov: doğru, ağırlık_çürümesi: 0.0, ns_adımı: 5)` | Nesterov momentumu + dikleştirme; ölçek √max(1, çıkış/giriş); yalnızca 2B ağırlıklar |
| `NorMuon(ağırlıklar, öğrenme_oranı: 0.003, beta1: 0.95, beta2: 0.95, epsilon: 1e-8, ağırlık_çürümesi: 0.0, ns_adımı: 5)` | Muon + nöron başına ikinci moment normalleştirmesi; güncellemenin RMS'i 0.2·η |
| `parametre_grupları(model, ayrı: ["gömme", "çıkış"])` | `{gizli: [...], diğer: [...]}`: 2B gizli ağırlıklar ile gömme, çıkış, norm ve sapmalar |
| `MuonVeAdamW(model, muon_oranı: yok, adamw_oranı: 0.003, ağırlık_çürümesi: 0.0, normuon: yanlış, ayrı:, beta2: 0.95)` | iki grup tek iyileştiricide: `adım()`, `sıfırla()`, `çarpan_ayarla(c)`; alanlar `muon`, `adamw` |
| `oran_planı(adım, toplam, ısınma: 0, azalma: 0.2, en_az: 0.0)` | ısınma–sabit–azalma (WSD) öğrenme oranı çarpanı |
| `gradyanları_sıfırla(ağırlıklar)` | gradyanları yerinde sıfırlar |

**Muon.** Kaynak: K. Jordan ve diğ., *Muon: An optimizer for hidden layers in neural networks* (2024, kellerjordan.github.io/posts/muon). Her W için `m ← β·m + (1−β)·g`, Nesterov'la `u ← β·m + (1−β)·g`, `W ← W·(1 − η·λ) − η·√max(1, çıkış/giriş)·dikleştir(u)`. Ölçek, ağırlığı `[çıkış, giriş]` tutan düzendeki √max(1, satır/sütun)'un bu düzendeki karşılığıdır.

**NorMuon.** Kaynak: Z. Li, L. Liu, C. Liang, W. Chen, T. Zhao, *NorMuon: Making Muon more efficient and scalable* (2025, arXiv:2510.05491, Algoritma 1). `M ← β1·M + (1−β1)·g`, `O ← dikleştir(M)`, `v ← β2·v + (1−β2)·ortalama(O², nöron başına)`, `Ô ← O / (√v + ε)`, `η̂ ← 0.2·η·√(m·n) / ‖Ô‖_F`, `W ← W − η·λ·W − η̂·Ô`. Bu düzende bir nöron W'nin bir **sütunudur** (`v` `[1, çıkış]`). Makaledeki gibi Nesterov'suz ve sapma düzeltmesizdir. **Varsayımlar:** ε = 1e-8 (makalede sayı yok); varsayılan öğrenme oranı 0.003 bu kitaplığın seçimidir (oran AdamW ölçeğindedir; makalede 124M'lik modelde 3.6e-4, 1.1B'de 4e-4).

**Öğrenme oranı çarpanı.** `çarpan_ayarla(c)` (c pozitif) sonraki adımda iki grubun oranını birlikte ölçekler: `MuonVeAdamW` her adımda `op.muon.öğrenme_oranı`'nı ve `op.adamw.öğrenme_oranı`'nı (dil tanımı [15.39](../docs/dil/15_tensorler.md)) taban oran × c yapar. Kopya ya da ek bellek yoktur; c = 1 iken AdamW'nin adımı planı olmayan AdamW'ninkiyle bit düzeyinde aynıdır (`iyilestirici_test.tr` sınar).

**Kontrol noktası.** Muon'un durum tensörleri `op.muon.durum` sözlüğündedir ve türev izlemez (parametre değildir, yalnızca türevsiz blokta yerinde güncellenir); `kontrol_noktası` bir nesnenin türev izlemeyen tensörlerini de saklayıp yerinde geri yazar (dil tanımı [20.2](../docs/dil/20_kontrol_noktasi.md)). Kontrol noktasına modelin yanında `op.adamw` ve `op.muon.durum` da verilir (örnek: `ornekler/11_gpt.tr`); sürdürülen eğitim kesintisizle aynıdır (`iyilestirici_test.tr`).

## Adlar ve açık konular

Kitaplığın adları kullanıcı düzeyi TR adlarıdır; dilin kodu değildir, `internal/dil/kod/tr.json`'a girmez. Yerleşiklerle karışmasın diye katmanlar büyük harfli ve ayırt edicidir (`gömme` yerleşiği ↔ `GömmeKatmanı`, `rms_normu` ↔ `RMSNormKatmanı`, `dikkat` ↔ `NedenselDikkat`).

**Açık konu (Ç3, yazım dilleri):** kodlu saklamada (dil tanımı [17](../docs/dil/17_kodlar.md)) kitaplığın adları kullanıcı adı gibi çevrilmeden kalır: İngilizce yazım dilinde de `GPT`, `Doğrusal`, `MuonVeAdamW` yazılır. Kitaplık adlarının çevirisi (kitaplığın kendi ad listesi mi, ayrı bir kod aralığı mı) İngilizce aşamasında ayrıca ele alınacaktır.
