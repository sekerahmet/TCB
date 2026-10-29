# 4. Yerleşik Yapay Zeka ("İkisi Bir Arada") ve Veri Bilimi Komutları

## A. Düşük Seviye Matris ve Otomatik Türev
- `TR_Tensor([[1, 2], [3, 4]])`, `TR_zeros([satır, sütun])`, `TR_ones([satır, sütun])`, `TR_eye(n)`, `TR_randn([satır, sütun], TR_seed: 42)`: matris 2 boyutlu bir **tensördür** (ayrı bir matris tipi ya da `matris(...)` / `rastgele_matris(...)` oluşturucusu yoktur; ayrıntılar §4.C)
- `A @ B` (Çok çekirdekli matris çarpımı), `A.TR_transpose()`, `A.TR_shape` (`[satır, sütun]`)
- `TR_Tensor(3.0, TR_requires_grad: TR_true)` (tek sayı 0 boyutlu tensör verir), `TR_backward(y)`, `x.TR_grad`, `x.TR_tolist` (`x.TR_grad` da bir tensördür; sayısını `x.TR_grad.TR_tolist` verir)
- Aktivasyonlar ve kayıp: `TR_relu(x)`, `TR_sigmoid(x)`, `TR_tanh(x)`, `TR_softmax(x)`, `TR_cross_entropy(çıktı, hedef)`; ortalama kare hata `TR_mean((tahmin - hedef) ** 2)` ile yazılır

## B. İstatistik ve Veri Biçimleri
- İstatistik: `TR_mean(liste)`, `TR_std(liste)`, `TR_median(liste)` (vektör işleri tensörlerle yazılır: `TR_sum(a * b)`, `TR_sqrt(TR_sum(a * a))`)
- Veri biçimleri: `TR_read_csv(yol)`, `TR_parse_csv(metin)`, `TR_json_parse(metin)`, `TR_json_string(değer)`
- `TR_read_csv` / `TR_parse_csv`: `TR_read_csv(yol)` bir CSV dosyasını okur, `TR_parse_csv(metin)` CSV metninin kendisini ayrıştırır (`TR_parse_csv("ad,yaş\nAli,30")`). `TR_read_csv`'ya CSV metni verilirse hata `TR_parse_csv`'ü önerir. Göreli yollar önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür. Baştaki bayt sırası işareti (BOM) atılır. Hücreler boşlukları kırpılarak **yalnızca katı sayı biçimindeyse** sayıya çevrilir: `42`, `-7` `TR_Int`; `1.0`, `2.5e3`, `1E-3` `TR_Float`. Öteki her hücre olduğu gibi `TR_String` kalır, böylece hiçbir değer sessizce bozulmaz: baştaki sıfırlı `007` (posta kodu, kimlik), `TR_Int` aralığını aşan `12345678901234567890`, `NaN`, `Inf`, `0x10`, `1_000`, `+5`, `.5`.
- `TR_json_parse` / `TR_json_string`: `TR_json_parse` sözlük anahtarlarının sırasını korur ve baştaki BOM'u atar; değeri bir `TR_JSON`'dur, asıl değere tip çağrısıyla geçilir (`TR_Int(j["a"])`); tam yazılmış ve `TR_Int`'e sığan sayı `TR_Int`, öbür sayılar (`1.0`, `1e2`, `12345678901234567890`) `TR_Float` türündedir. `TR_json_string` anahtarları sözlükteki sırayla yazar (``json_metin(json_çöz(`{"b": 1, "a": 2}`))`` → `{"b":1,"a":2}`; JSON metni ham metinle `` `...` `` yazılınca `{` ve `"` kaçış istemez). JSON'da karşılığı olmayan değerler `dene/yakala` ile yakalanan bir hata verir: Tanımsız ve sonsuz sayılar (`NaN sayısı JSON'a yazılamaz ...`), kendini içeren liste/sözlük (`değer kendini içeriyor ...`), 1000 seviyeden derin iç içe değerler ve `1` ile `"1"` gibi JSON'da aynı anahtara düşen iki anahtar.
## C. Tensörler ve Otomatik Türev
Tensör, n boyutlu bir sayı dizisidir (veri türü varsayılan olarak `float32`; `bf16` ve `fp16` de seçilebilir, bkz. **Veri türleri**); yapay zekâ modellerinin ağırlıkları, girdileri ve ara sonuçları tensördür. Her işlem hesap çizgesine kaydedilir; `TR_backward(kayıp)` tüm gradyanları hesaplar. Çalıştırılabilir örnekler: `examples/tr/23_tensor.tr`, `examples/tr/35_transformer.tr`, `examples/tr/36_karakter_modeli.tr`.

- **Oluşturma:** `TR_Tensor(veri, TR_requires_grad: TR_true)` (veri: iç içe liste, tek sayı ya da tensör), `TR_zeros([2, 3])`, `TR_ones([2, 3])`, `TR_randn([2, 3], TR_seed: 42)`. Şekil **liste** olarak verilir; `TR_zeros(2, 3)` gibi ayrı tam sayılar hatadır. Matris 2 boyutlu tensördür: `TR_Tensor([[1, 2], [3, 4]])`, `TR_eye(n)` ve `TR_randn([satır, sütun], TR_seed: 7)` ile oluşturulur. Kaldırılan `matris(...)` ve `rastgele_matris(...)` yazılırsa hata karşılığı önerir (`'matris' TR'de yok; 'tensör' yazmak mı istediniz?`). `türev_izle` varsayılanı her zaman (tek sayıda da) `TR_false`tır: `TR_Tensor(3.0)` 0 boyutlu (skaler) bir tensördür (`.TR_shape` `[]`, `.TR_tolist` `3.0`, `.TR_grad` `TR_none`); türevi izlenecekse `TR_Tensor(3.0, TR_requires_grad: TR_true)` yazın.
- **Sayı hassasiyeti:** Tensör hesapları `float32` ile (yaklaşık 7 anlamlı basamak) yapılır; `TR_Float` ise 64 bitliktir. Bu yüzden `(TR_Tensor([1.0]) / 3).TR_tolist[0]` `0.33333334` verir ve `1 / 3`e eşit değildir; `TR_Tensor([16777217.0])` 16777216 olarak saklanır. Kesin karşılaştırma yerine `TR_round(...)` ya da küçük bir tolerans kullanın.
- **Veri türleri:** Her tensörün bir türü vardır: `"float32"` (varsayılan), `"bf16"` (bfloat16: float32 ile aynı aralık, ~3 anlamlı basamak) ya da `"fp16"` (IEEE yarım hassasiyet: en büyük değer 65504, ~3 anlamlı basamak). 16 bitlik türler belleği yarıya indirir. Tür, oluşturmada `TR_dtype:` ile seçilir: `TR_Tensor(veri, TR_dtype: "bf16")`, `TR_zeros([2, 3], TR_dtype: "fp16")`, `TR_ones([4], TR_dtype: "bf16")`, `TR_randn([2, 3], TR_seed: 1, TR_dtype: "bf16")`; değerler en yakın gösterilebilir değere yuvarlanır (`TR_Tensor([1.001], TR_dtype: "bf16").TR_tolist` `[1.0]`). Tensörden oluşturmada (`TR_Tensor(t)`) kaynağın türü korunur; türünü değiştirmek için `t.TR_to_dtype("bf16")` yazılır (`TR_Tensor(t, TR_dtype: ...)` hata verir). `.TR_dtype` türü metin olarak verir (`"bf16"`); `.TR_to_dtype("fp16")` dönüştürülmüş bir kopya verir ve **türevlenebilir**dir (gradyan kaynak tensöre kendi türünde akar; tür aynıysa tensörün kendisi döner). İşlemler içte en az float32 ile biriktirir ve sonucu bir kez yuvarlar; işlenenler aynı türdeyse sonuç o türde, farklıysa `float32`'dir; sayıyla işlemler (`b * 2`, `b / 2`, `2 / b`...) tensörün türünü korur. 16 bitlik parametrelerin (yaprak tensörlerin) gradyanları `float32` tutulur; ara sonuçlarınki kendi türündedir (`.TR_to_dtype` ve görünümler kaynağın gradyan türünü izler: float32 ağırlığın `bf16` kopyasının gradyanı `float32`'dir). Bilinmeyen tür adı yol gösteren bir hatadır: `tensör: bilinmeyen veri türü "bfloat16"; geçerli türler: "float32", "bf16", "fp16". Bunu mu demek istediniz: "bf16"?`
- **Karışık hassasiyet:** `TR_mixed_precision("bf16") { ... }` bloğu (sondaki blok yazımıyla; yeni anahtar kelime değildir) içinde `@` matris çarpımları ve `TR_attention` girdilerini `bf16`'ya çevirir ve float32 biriktirmeyle **bf16** sonuç verir (PyTorch'un autocast'iyle aynı kural); normalizasyon (`TR_layer_norm`, `TR_rms_norm`), `softmax`/`log_softmax` ve `TR_sum`/`TR_mean` 16 bitlik girdiyi float32'ye açarak hesaplar, `TR_cross_entropy` 16 bitlik logitleri float32'de işler ve float32 kayıp verir. Ağırlıkların kendisi değişmez (dönüşüm türevlenebilir bir kopyadır), bu yüzden `TR_backward` ve iyileştirici float32 ağırlıklarla çalışır. Blok son ifadesinin değerini döndürür: `TR_var kayıp = TR_mixed_precision("bf16") { TR_cross_entropy(ileri(x), y) }`. `"fp16"` de verilebilir (65504'ü aşan değerler sonsuza taşar; kayıp ölçekleme yoktur); `"float32"` iç içe bir blokta karışık hassasiyeti kapatır. Bloklar iç içe kurulabilir; blok bitince (hata ile bitse de) önceki tür geri gelir. Durum **göreve bağlıdır**: bloğun içinde kilidi bırakan bir çağrı (`TR_sleep(...)`, ağ, dosya) sırasında çalışan başka bir `TR_async` görev bloktan etkilenmez; blok içinde başlatılan bir `TR_async` görev de float32 ile başlar (gerekirse kendi bloğunu kurar). CPU'da bf16 hız kazandırmaz (dönüşüm maliyeti nedeniyle biraz yavaştır); amacı bellek/bant genişliği tasarrufu ve 16 bitlik donanıma hazırlıktır.
- **Başka dillerden gelenler için:** şekil `.TR_shape` (satır sayısı `.TR_shape[0]`), devrik `.TR_transpose()` (NumPy/PyTorch'un `.T`'si yazılınca hata `.TR_transpose()`i önerir), toplam ve ortalama `TR_sum(A)` / `TR_mean(A)` (0 boyutlu tensör; sayısı `.TR_tolist`), veri `.TR_tolist`.
- **Özellik ve metotlar:** `.TR_shape`, `.TR_tolist` (iç içe liste; skalerde tek sayı; elemanlar Ondalık, tam sayılı tensörlerde — `TR_argmax` sonucu, veri kümesinden gelen `girdi`/`hedef` — `TR_Int`; float32 değerler en kısa gösterimleriyle verilir, `TR_Tensor([0.1]).TR_tolist[0] == 0.1` doğrudur), `.TR_grad` (yalnızca `TR_requires_grad: TR_true` ile oluşturulan yaprak tensörlerde; bir işlemin sonucu olan tensörde açıklayıcı hata verir), `.TR_device`, `.TR_reshape([4, 6])` (bir eksen `-1` olabilir), `.TR_transpose()`, `.TR_permute([0, 2, 1])`, `.TR_slice(eksen, baş, son)` (son hariç, negatifler sondan sayar), `.TR_to_device("cpu")` (GPU için bkz. aşağıda "GPU'da eğitim").
- **İşleçler:** `@` (matris çarpımı; 1 boyutlu işlenen vektör sayılır, 3+ boyutlu toplu çarpım), `+ - * /` eleman bazlı ve **yayınlamalıdır** (eksenler sağdan hizalanır; eşit ya da 1 olan eksenler uyumludur). Sayı iki yanda da durabilir: `2 * a`, `a - 1`, `1 - a`, `a / 2`, `1 / a`. Tekli eksi (`-a`) ve sayı üssü (`a ** 2`) de vardır; hepsi türev izler. İki işlenen de genişletilirse (ör. `[4, 1]` ile `[4]` → `[4, 4]`; çoğunlukla bir şekil hatası) standart hataya her işlem ve şekil çifti için bir kez (en çok 100 farklı uyarı; sonrası tek bir bildirimle susturulur) `Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); ...` yazılır; program durmaz. `%` tensörlerde tanımlı değildir.
- **Karşılaştırma ve maske:** `< > <= >=` bir sayıyla ya da yayınlanabilen bir tensörle eleman bazlı karşılaştırır ve türev izlemeyen 0/1 maskesi verir: `a > 0`, `a < b`. `==` ve `!=` ise tensörleri bütün olarak karşılaştırıp tek bir `TR_Bool` verir: şekilleri ve tüm elemanları aynıysa eşittir (`TR_Tensor([1.0, 2.0]) == TR_Tensor([1.0, 2.0])` doğru, `[2]` ile `[1, 2]` şekilli tensörler eşit değildir). 0 boyutlu tensör bir sayıyla da karşılaştırılabilir: `TR_Tensor(7.0) == 7`, `A[1][0] == 3.0` ve `TR_sum(t) == 10` doğrudur; en az bir ekseni olan tensör (`TR_Tensor([7.0])`) sayıya eşit değildir.
- **İndeksleme:** `t[i]` ilk eksenin i. elemanını verir (bir eksen eksilir; negatif indeks sondan sayar), `t[baş..son]` ilk eksende dilim verir (`son` hariç); ikisi de veriyi kopyalamayan, türev izleyen görünümlerdir ve zincirlenebilir (`t[0][1]`); bu, `.TR_transpose()` gibi bitişik olmayan görünümlerde de geçerlidir (`t.TR_transpose()[0] = 9` `t`'nin ilk sütununu yazar). `.TR_reshape` yalnızca bitişik olmayan bir tensörün eksenlerini depoda düzenli adımla ifade edilemeyecek biçimde birleştirirken (ör. `t.TR_transpose().TR_reshape([-1])`) kopya üretir. Sınır dışı indeks hatadır.
- **İndeksle atama:** `t[i] = d`, `t[i][j] = d`, `t[baş..son] = d` ve `t[i] += d` veriyi yerinde değiştirir. `d` bir sayıysa hedefin tüm elemanlarına yazılır (`t[0] = 0` ilk satırı sıfırlar); liste ya da tensörse şekli hedefin şekliyle aynı olmalıdır (`t[0] = [1, 2, 3]`). Türev izlenen bir tensöre (ör. bir ağırlığa) yalnızca `TR_no_grad { ... }` içinde atanabilir.
- **Döngü:** `TR_for x TR_in t`, `TR_for i, x TR_in t` ve `[ifade TR_for x TR_in t]` tensörü ilk ekseni boyunca gezer: 2 boyutlu tensörde `x` sıradaki satırdır (veriyi paylaşan 1 boyutlu tensör; `x[0] = 9` tensörü değiştirir), 1 boyutlu tensörde sıradaki sayıdır (`TR_Float`; tam sayılı tensörde `TR_Int`). Örnek: `[TR_sum(satır).TR_tolist TR_for satır TR_in A]` satır toplamlarını liste olarak verir. 0 boyutlu tensör üzerinde döngü kurulamaz.
- **Yazdırma:** `TR_print(t)` şekli başlıkta gösterir (`Tensör[2×3]`; float32 dışındaki türlerde tür de: `Tensör[2×3] bf16`, skalerde `Tensör(2.5000) fp16`), değerleri 4 basamakla yazar; 1000'den çok elemanda her eksenin yalnızca ilk ve son 3 elemanı gösterilir (`…`). Sıfır olmayan bir değer 0.0001'den küçükse ya da bir değer 1e8 ve üzerindeyse tüm hücreler bilimsel gösterimle (`1.0000e-05`) yazılır. Sayı olmayan değerler Ondalık'la aynı adlarla görünür: `NaN`, `sonsuz`, `-sonsuz` (İngilizce dosyada `nan`, `inf`, `-inf`; 3.2).
- **Koşul:** tensör koşul olamaz (3.10): `TR_if t { ... }`, tek elemanlı ya da boş tensörde de hatadır. Değeri açıkça karşılaştırın: `TR_if TR_max(t).TR_tolist > 0`.
- **Matematik:** `TR_relu`, `TR_sigmoid`, `TR_tanh`, `TR_gelu` (varsayılan tanh yaklaşımı; `TR_gelu(x, TR_approximate: TR_false)` tam erf tabanlı GELU), `TR_silu`, `TR_sqrt`, `TR_exp`, `TR_log`, `TR_sin`, `TR_cos` (radyan; dönel konum gömmesi gibi açılı hesaplar için, türevli), `TR_softmax(x, TR_dim: -1)`, `TR_log_softmax(x, TR_dim: -1)`; genişletilmiş matematik aşağıdaki tablodadır. **Dönüş tipi kuralı:** tensör yalnızca tensör girdisinden çıkar (`TR_relu(TR_Tensor([[1, -2]]))` ve `TR_relu(TR_eye(2))` tensör verir). Eleman bazlı fonksiyonlar sayıda da çalışır (`TR_relu(-2)` → `0.0`), listede çalışmaz: listeyi `TR_Tensor(...)` ile çevirin (`TR_relu(TR_Tensor([1, -2]))`); `TR_softmax` ve `TR_log_softmax` yalnızca tensör alır (`TR_softmax(TR_Tensor(...), TR_dim: -1)`). Boş tensörün `TR_mean`sı ve bütün hedefleri yok sayılmış (hepsi negatif) bir `TR_cross_entropy` sessizce 0 vermez, açık hata verir. Tamamen maskeli (hepsi `-sonsuz`) bir satırın `TR_softmax`ı 0, `TR_log_softmax`ı `-sonsuz` olur ve gradyan almaz; dolgu satırları kaybı NaN yapmaz. Sayı olmayan değer (`Tanımsız`, NaN) yayılır: `TR_relu`, `TR_max` ve `TR_min` onu yutmaz, `TR_argmax` ilk NaN'ın yerini verir; böylece kayıpta beliren NaN gizlenmez.

  | İş | TR | Not |
  | :-- | :-- | :-- |
  | SiLU etkinleştirme | `TR_silu(x)` | `x · sigmoid(x)`, tek geçiş; SwiGLU: `TR_silu(x @ Wg) * (x @ Wu)` |
  | Trigonometri | `TR_tan`, `TR_asin`, `TR_acos`, `TR_atan`, `TR_atan2(y, x)` | radyan; `TR_atan2` dört bölgeli, yayınlamalı |
  | Hiperbolik | `TR_sinh`, `TR_cosh` (`TR_tanh` zaten var) | |
  | Kararlı logaritma ve üstel | `TR_log1p(x)`, `TR_log2`, `TR_log10`, `TR_expm1(x)` | küçük x'te `TR_log(1 + x)` ve `TR_exp(x) - 1`'den doğru |
  | Yuvarlama | `TR_floor`, `TR_ceil`, `TR_sign` | Tam'da Tam; türev 0 |
  | Mutlak değer | `TR_abs(x)` | Tam'da Tam; tensörde türevli (türev `TR_sign(x)`, 0'da 0); en büyük fark: `TR_max(TR_abs(a - b))` |
  | Sınırlama | `TR_clamp(x, TR_min: 0, TR_max: 1)` | sınırlardan biri yeter; türev aralık içinde 1 |
  | Koşullu seçim | `TR_where(x > 0, x, 0)` | yayınlamalı; gradyan seçilen dala akar |
  | Birikimli toplam | `TR_cumsum(x, TR_dim: -1)` | türevli, belirlenimci |
  | En büyük k | `TR_topk(x, 5)` → `[değerler, indeksler]` | eşitlikte küçük indeks önce |
  | Sıralama | `TR_argsort(x)` | kararlı, küçükten büyüğe |
  | π | `TR_pi` | sabit; `e` yerine `TR_exp(1)` |

  Karşılaştırmalar (`<`, `>`, `<=`, `>=`) tensörlerde yayınlamalıdır ve 0/1 maskesi verir; `==` ise iki tensörün tamamen aynı olup olmadığını söyler. Hepsi GPU'da da çalışır (dil tanımı **15.11**–**15.15**, **15.20**–**15.22**).
- **İndirgemeler:** `TR_sum(x)`, `TR_mean(x)`, `TR_max(x)`, `TR_min(x)` tüm elemanları skaler (0 boyutlu) tensöre indirger; `TR_dim: 1` verilirse o eksen kaldırılır; `TR_keepdim: TR_true` ile 1 uzunlukta korunur (`[B, T, D]` → `TR_sum(x, TR_dim: -1, TR_keepdim: TR_true)` → `[B, T, 1]`, böylece `x / TR_sum(x, TR_dim: -1, TR_keepdim: TR_true)` yayınlanır; eksen verilmezse bütün eksenler 1 olur). `TR_sum(x, 1)` / `TR_mean(x, 1)` konumlu ekseni de kabul eder; `TR_max`/`TR_min` için eksen **yalnızca adlı** verilir (`TR_max(x, 0)` eleman bazlı sınırla karışmasın diye hatadır). `TR_sum([a, b])` / `TR_mean([a, 1, b])` listedeki tensörleri yayınlamalı toplamla katlar (türev korunur); iç içe sayı listesi indirgenemez, önce `TR_Tensor(...)` ile çevirin. `TR_dim:` yalnızca tensörlerde geçerlidir.
- **Sinir ağı:** `TR_embedding_lookup(tablo, indeksler)` (tablo `[V, D]`; sonuç `indeks şekli + [D]`), `TR_cross_entropy(çıktı, hedef)` (çıktı `[..., C]`, hedef sınıf indeksleri; negatif hedefler yok sayılır), `TR_layer_norm(x, ağırlık, sapma, TR_eps: 0.00001)`, `TR_rms_norm(x, ağırlık, TR_eps: 0.00001)` (ağırlık/sapma `TR_none` olabilir).
- **Bitiştirme ve bölme:** `TR_concat([a, b, c], TR_dim: -1)` tensörleri bir eksen boyunca uç uca ekler (diğer eksenler eşit olmalı; türevli). `x.TR_split(3, TR_dim: -1)` tensörü o eksende 3 eşit parçaya böler ve parçaların listesini verir (görünümler, türevli): `TR_var q, k, v = (x @ Wqkv).TR_split(3)`.
- **Dikkat:** `TR_attention(q, k, v, TR_causal: TR_true, TR_window: 0, TR_mask: TR_none)` birleşik ölçekli nokta çarpımı dikkatidir; `softmax(q @ k.devrik() / √dh + maske) @ v` ile aynı sonucu verir ama `T×T` puan matrisini hiç tutmaz (puanlar bloklar hâlinde, çevrimiçi softmax ile hesaplanır; bellek T ile doğrusal) ve tek bir türev düğümüdür (geri yayılım puanları yeniden hesaplar). `q` `[..., T, H, dh]`, `k`/`v` `[..., Tk, Hkv, dh]` şeklindedir: doğrusal katmanın çıktısı `şekillendir([B, T, H, dh])` ile doğrudan verilir, sonuç `[..., T, H, dh]` `şekillendir([B, T, H · dh])` ile birleşir (eksen sıralama gerekmez); `Hkv < H` gruplu sorgu dikkatidir (`H`, `Hkv`'nin katı; k/v kopyalanmaz). Ölçek `1/√dh`'dir. `TR_causal: TR_true` (varsayılan) gelecekteki konumları kapatır; `Tq < Tk` iken (önbellekli üretimde yalnızca yeni parça sorgulanırken) hizalama sondandır: sorgu `i`, `Tk - Tq + i`'ye kadar görür. `TR_window: 128` her sorguyu son 128 konuma sınırlar (yerel dikkat katmanı; pencere dışındaki bloklar hiç hesaplanmaz). `TR_mask:` isteğe bağlı 0/1 tensörüdür (1 = bakılır; `[Tq, Tk]` gibi yayınlanabilir şekiller kabul edilir). `TR_mixed_precision("bf16")` bloğunda girdiler bf16'ya çevrilir, biriktirme float32'de yapılır, sonuç bf16'dır (`@` gibi). Hiçbir konuma bakamayan satırın çıktısı 0'dır.
- **Örnekleme:** `TR_sample(logit, TR_temperature: 1.0, TR_top_k: 0, TR_seed: TR_none)` son eksendeki `TR_softmax(logit / sıcaklık)` dağılımından bir parça kimliği örnekler; sonuç son ekseni düşürülmüş, tam sayılı bir tensördür (`.TR_tolist` `TR_Int`): `[B, V]` logitten `[B]` kimlik. `TR_top_k: 40` yalnızca en olası 40 parça arasından seçer; `TR_temperature: 0` en olasıyı verir (`TR_argmax` ile aynı). Aynı `tohum` aynı sonucu verir; tohumsuz çağrılar her seferinde farklıdır. Türev izlemez.
- **Model parametreleri:** `TR_parameters(model)` bir yapı nesnesini (iç içe yapılar, listeler ve sözlükler dahil) gezip türev izleyen yaprak tensörleri sabit sırayla (alan tanım sırası) bir liste olarak verir; aynı tensör bir kez yer alır: `TR_AdamW(TR_parameters(model), TR_lr: 0.001)`. `TR_parameters(model, TR_named: TR_true)` yol adı → tensör sözlüğü verir (`"bloklar.0.dikkat.Wq"`; liste indeksleri sayı, sözlük anahtarları anahtarın kendisi); bu sözlük doğrudan kaydedilir: `TR_save_tensors("model.trt", TR_parameters(model, TR_named: TR_true))`.
- **Türev:** `TR_backward(kayıp)` (kayıp tek elemanlı olmalı), `x.TR_grad` (gradyanlar **birikir**: her `TR_backward` yeni gradyanı öncekine ekler, bu yüzden eğitim döngüsünde her adımda önce `op.TR_zero_grad()` çağrılır; `x.TR_grad` bir kopya değil gradyanın kendisidir: sonraki `TR_backward`, `op.TR_zero_grad()` ve `TR_clip_grad` onu da değiştirir, saklamak isterseniz `TR_Tensor(x.TR_grad)` ile kopyalayın), `TR_no_grad { ... }` bloğu türev izlemeden çalışır ve bloğun değerini (son ifade ya da `TR_return`) döndürür, `TR_no_grad(x)` tensörü hesap çizgesinden ayırır.
- **Kullanıcı tanımlı türev:** `TR_custom_grad(TR_forward: ..., TR_backward: ...)` yeni bir işlemin ileri ve geri hesabını kendiniz yazmanızı sağlar (yeni bir katman fikrini çizgeyi şişirmeden denemek için). `ileri` türevsiz çalışır ve `[çıktı, saklananlar]` döndürür; geri hesabın ihtiyaç duyacağı her şeyi `saklananlar` listesine **açıkça** koyarsınız. Sonuç çizgeye tek düğüm olarak kaydedilir; geri yayılımda `geri(saklananlar, g)` çağrılır ve her konumlu girdi için girdiyle aynı şekilde bir gradyan ya da `TR_none` döndürür (tensör olmayan girdiler için `TR_none`; yayınlama yapılmaz, yanlış şekil açık hatadır). Adlı argümanlar (`TR_scale: 0.5` gibi) `ileri`ye geçer, gradyan almaz. Çıktı bir tensör listesiyse çağrı liste döndürür ve `g` her çıktının gradyanını içeren listedir (kayba bağlı olmayan çıktı için sıfır tensör). Saklanan bir tensör geri yayılımdan önce yerinde değiştirilirse hata alırsınız. `TR_grad_check(f, [girdiler...], TR_fd_step: 0.001, TR_tol: 0.01)` geri hesabınızı merkezi farkla bulunan sayısal gradyanla karşılaştırır: doğruysa `TR_true` döner, yanlışsa en büyük farkın yerini ve değerlerini yazan bir hata verir; sıradan TR fonksiyonlarında da çalışır. Ayrıntılı örnek (satıra özel kapılı nöron): `tests/ozel_turev_test.tr`.
- **İndeks:** `TR_argmax(x, TR_dim: -1)` en büyük elemanın indeksini verir (eşitlikte ilki). Sonuç her zaman tam sayı değerli, türev izlemeyen bir tensördür (o eksen kaldırılır; 1 boyutlu girdide skaler); doğrudan `TR_embedding_lookup`/`TR_cross_entropy` indeksi olarak kullanılabilir. Tensör tam sayılı işaretlidir: ondalıksız yazdırılır ve `.TR_tolist` `TR_Int` (ya da `TR_Int` listesi) verir, böylece `harfler[TR_argmax(x).TR_tolist[0]]` gibi liste indekslemede doğrudan kullanılır. Bu tensörle yapılan işlemlerin sonucu (ör. `i * 2`) yine Ondalık değerlidir.
- **Gradyan kırpma:** `TR_clip_grad(parametreler, TR_max: 1.0)` gradyanların toplam L2 normunu `en_fazla` ile sınırlar ve kırpma öncesi normu 0 boyutlu tensör olarak döndürür (sayısı `.TR_tolist`); `TR_backward` ile `op.TR_step()` arasında çağrılır. GPU'da norm ve kırpma cihazda yapılır, program beklemez.
- **Kaydetme/yükleme:** `TR_save_tensors("model.trt", {"w": w, "b": b})` ve `TR_var p = TR_load_tensors("model.trt")` (isteğe bağlı `TR_requires_grad: TR_true`). Göreli yollar `TR_read_file`/`TR_write_file` ile aynı kuralla çözülür (önce çalışma klasörü, orada yoksa programın klasörü); dosya varsa üzerine yazılır; yazma atomiktir (önce geçici dosyaya yazılıp yerine konur), eğitim kesilse ya da disk dolsa bile önceki kontrol noktası bozulmaz. Dosya hataları Türkçedir (`tensör_yükle: "model.trt" yüklenemedi: böyle bir dosya ya da klasör yok (yolun yazımını denetleyin); bakılan yer: ...`). Biçim (sürüm 2, küçük sonlu): `TRTENSOR` (8 bayt), sürüm `uint16`, girdi sayısı `uint32`; her girdi için ad uzunluğu `uint32` + UTF-8 ad, veri türü kodu `uint8` (1 = float32, 2 = bf16, 3 = fp16), eksen sayısı `uint8`, her eksen `uint64`, satır öncelikli veri (float32 4 bayt, bf16/fp16 2 baytlık bit deseni; 16 bitlik tensörler 16 bit olarak yazılır ve aynı türde yüklenir); sonda tüm baytların CRC-32 değeri `uint32`. Eski sürüm 1 dosyaları artık okunmaz. Bozuk, eksik, bilinmeyen sürümlü ya da başka türden dosya açık bir Türkçe hatayla reddedilir; indirilen bir model dosyası da güvenle denenebilir: veri taşıyan bir tensör en fazla 2³² eleman ve 32 eksen bildirebilir; verisiz (sıfır uzunluklu eksenli) şekil için sınır 50.000.000'dur (sıfır eksenler 1 sayılır). Hangi girdilerin güvenilmeyen sayıldığı ve açıkların nasıl bildirileceği: [`SECURITY.md`](../../SECURITY.md).
- **İyileştiriciler:** `TR_AdamW(parametreler, TR_lr: 0.001, TR_weight_decay: 0.01, TR_beta1: 0.9, TR_beta2: 0.999, TR_eps: 0.00000001)`, `TR_SGD(parametreler, TR_lr: 0.01, TR_momentum: 0, TR_weight_decay: 0)`; `.TR_step()` parametreleri günceller, `.TR_zero_grad()` gradyanları sıfırlar. Parametreler doğrudan `TR_Tensor(..., TR_requires_grad: TR_true)` ile oluşturulmuş tensörler olmalıdır. İkisi de `TR_master_weights: TR_true` (varsayılan) seçeneğini alır: 16 bitlik (`bf16`/`fp16`) parametrelerin float32 ana kopyası tutulur, güncelleme ana kopyada yapılıp parametreye yuvarlanarak yazılır; böylece 16 bitlik aralıktan küçük adımlar kaybolmaz (`TR_master_weights: TR_false` ile güncelleme doğrudan 16 bitlik parametrede yapılır). float32 parametreleri etkilemez; moment tahminleri her zaman float32'dir.

```tr
# TR 4
# y = 2·x - 1 doğrusunu öğren
TR_var X = TR_Tensor([[0], [1], [2], [3]])
TR_var Y = TR_Tensor([[-1], [1], [3], [5]])
TR_var w = TR_Tensor([[0.0]], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([0.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w, b], TR_lr: 0.1, TR_weight_decay: 0)

TR_for adım TR_in 0..500 {
    op.TR_zero_grad()
    TR_var fark = X @ w + b - Y
    TR_backward(TR_mean(fark * fark))
    op.TR_step()
}
TR_print("w = \(TR_round(w.TR_tolist[0][0], 3)), b = \(TR_round(b.TR_tolist[0], 3))")   # w = 2.0, b = -1.0
TR_print(TR_no_grad { X @ w + b })                                                # Tensör[4×1] [...]
```

Kullanıcı tanımlı türev: kapılı çarpım `a * TR_sigmoid(b)` için elle yazılmış geri hesap ve denetimi.

```tr
# TR 4
TR_var kapılı = TR_custom_grad(
    TR_forward: (a, b) => {
        TR_var s = TR_sigmoid(b)
        TR_return [a * s, [a, s]]                       # [çıktı, saklananlar]
    },
    TR_backward: (k, g) => {                          # k: saklananlar, g: çıktının gradyanı
        TR_var a = k[0]
        TR_var s = k[1]
        TR_return [g * s, g * a * s * (1 - s)]           # a ve b için birer gradyan
    }
)
TR_var a = TR_Tensor([[1.0, -2.0]], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([[0.5, 0.0]], TR_requires_grad: TR_true)
TR_backward(TR_sum(kapılı(a, b)))
TR_print(a.TR_grad, b.TR_grad)
TR_print(TR_grad_check(kapılı, [a, b]))               # true (yanlış bir geri hesabında açık hata)
```

Şekil hataları şekli gösterir ve `dene/yakala` ile yakalanabilir: `[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512`. Fonksiyon, metot, işleç (`@`, `+`, `<`...) ve özellik hatalarının başında `Satır N:` bulunur: `Satır 3: [2, 2] ile [1, 3] matris çarpımı yapılamaz; 2 ≠ 1 (...)`.

Bellek sınırı: bir işlemin sonucu tek başına makinenin fiziksel belleğinden büyükse tensör ayrılmadan önce Türkçe bir hata verilir ve program çökmez. Çoğu zaman sebep beklenmedik bir yayınlamadır: `TR_zeros([185000, 1, 1]) + TR_zeros([4, 46000])` hatası `toplama: sonuç tensörü [185000, 4, 46000] şeklinde, 126,8 GB bellek ister; bu makinede 31,5 GB var. Şekilleri denetleyin (yayınlama beklenmedik bir boyut üretmiş olabilir).` olur. Sınır ayarlanmaz; ayrıntı: dil tanımı **15.42**.

> **Not (türevsiz ve eşzamanlılık):** Türevsiz kip her görevin kendi durumudur. `TR_no_grad { ... }` bloğu fonksiyon çağırsa, döngü kursa ya da `TR_sleep(...)` gibi yorumlayıcı kilidini bırakan bir çağrı yapsa da, aynı anda çalışan başka bir `TR_async` görev (ör. eğitim döngüsü) türev izlemeye devam eder; yani bir görevde türevsiz çıkarım (ör. örnek metin üretme) yaparken arka planda eğitim sürdürebilirsiniz. Yeni bir `TR_async` görev her zaman türevli kipte başlar. Bir geri yayılım sürerken başka bir görev aynı çizgeden geri yayılım başlatırsa açık hata alır.

VS Code İzleme Masası'nda tensörler tip sütununda `Tensör [2×3]` (float32 dışındaki türlerde `Tensör [2×3] bf16`) olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılabilir. Skaler tensörün (ör. kayıp) değeri değer sütununda görünür (`0.6074 (Tensör, skaler)`, bf16'da `2.5000 (Tensör, skaler, bf16)`); 1-8 elemanlı tensörler satır içinde kısaltılmış yazılır (`[[1.4298]] (Tensör, 1×1)`), daha büyükleri tablo olarak açılır. Variables panelinde tensörler ilk ekseni boyunca açılır (satırlar, onların içinde sayılar; 1000'den çok satırda ilk 1000 satır ve bir `…` satırı gösterilir).

### GPU'da eğitim

NVIDIA GPU'su ve sürücüsü olan bir makinede (ör. bulut GPU makinesi) aynı TR programı GPU'da
çalışır; ek kurulum, derleyici ya da kitaplık gerekmez. En sade yol, modeli kurmadan önce
varsayılan cihazı seçmektir: ağırlıklar, iyileştiricinin durumları ve veri kümesinden gelen
toplu işler GPU'da oluşur.

```tr
# TR 4
TR_if TR_devices().TR_contains("gpu") { TR_default_device("gpu") }   # GPU yoksa CPU'da kalır
TR_var W = TR_Tensor(TR_no_grad(TR_randn([16, 4], TR_seed: 1) * 0.1), TR_requires_grad: TR_true)
TR_var x = TR_randn([8, 16], TR_seed: 2)              # varsayılan cihazda
TR_var iyileştirici = TR_AdamW([W], TR_lr: 0.01)
TR_for adım TR_in 0..10 {                                    # döngü CPU'dakiyle aynı
    iyileştirici.TR_zero_grad()
    TR_var kayıp = TR_cross_entropy(x @ W, [0, 1, 2, 3, 0, 1, 2, 3])
    TR_backward(kayıp)
    iyileştirici.TR_step()
}
TR_print(W.TR_device, TR_default_device())
```

Veri kümesinin örnekleyicisi (`TR_Dataset(...).TR_sampler(...)`) de toplu işleri varsayılan
cihazda verir; büyük bir modelin tamamı için örnek: `tools/gpu/compare_gpu.tr`.

- `TR_devices()` kullanılabilir cihazları verir (`["cpu", "gpu"]`); `TR_default_device()` geçerli
  cihazın adını. `t.TR_device` bir tensörün cihazını, `t.TR_to_device("cpu")` taşınmış kopyasını verir.
- Türev izleyen bir ağırlığı tek tek taşımak da olur: `TR_self.W = ağırlık(...).TR_to_device("gpu")`
  GPU'da **yeni bir yaprak** verir; iyileştiriciye bu verilir (gradyanı onun `.TR_grad`ında).
- Farklı cihazlardaki iki tensörle işlem açık bir hatadır; sessizce kopyalanmaz.
- GPU'daki işlemler sıraya eklenip hemen döner; `.TR_tolist` ve yazdırma hesabın bitmesini
  bekler (`TR_clip_grad` beklemez; döndürdüğü norm da okununca bekler). Süre ölçerken ölçümden önce bir `.TR_tolist` okuyun.
- Sonuçlar CPU'dakiyle float32 yuvarlama farkı içinde aynıdır; indirgemeler belirlenimcidir.
  GPU belleği yetmezse Türkçe ve `dene/yakala` ile yakalanabilir bir hata alınır.
- Teşhis: bir GPU hatasının hangi işlemde olduğunu tam bulmak için program
  `TR_GPU_ESZAMANLI=1` ortam değişkeniyle çalıştırılır (her işlemden sonra bekler; yavaştır).
- Hız dökümü: `TR_GPU_PROFIL=1` ile çalışan program sonunda standart hataya GPU profilini
  yazar (adım başına süreler, hangi çekirdeğin GPU zamanının ne kadarını aldığı, bellek).
  `TR_cmd_watch` ve `TR_cmd_profile` aynı ölçümü canlı tabloda ve raporda gösterir (bkz. §11).
- Matris çarpımı, makinede NVIDIA'nın cuBLASLt kitaplığı kuruluysa (CUDA araç takımı ya da
  PyTorch ile gelir) onunla yapılır; yoksa TR'nin kendi çekirdeğiyle (sonuç aynı, hız farklı).
  Kitaplık başka bir yerdeyse yolu `TR_GPU_CUBLASLT=/tam/yol/libcublasLt.so.12` ile verilir
  (çalışma klasörü hiçbir zaman aranmaz); hangi yolun kullanıldığı profil özetinin ilk
  satırında görünür.
- Kurallar: `docs/spec/15_tensorler.md` **15.37–15.41**.

## D. Veri ve Parçalayıcı
Büyük metin derlemleriyle model eğitmek için iki yerleşik nesne vardır: `tokenizer.json` dosyalarını okuyan **parçalayıcı** ve `.npy` veri dosyalarını okuyan **veri kümesi**. Uçtan uca örnek: `examples/tr/39_veri_ile_egitim.tr`.

**Parçalayıcı**
- `TR_var p = TR_load_tokenizer("tokenizer.json")` yaygın `tokenizer.json` biçimini okur (BPE, WordPiece, WordLevel, Unigram modelleri; bayt düzeyi, Metaspace, BERT tarzı bileşenler). Desteklenmeyen bir bileşen türü, hangi bileşenin hangi türü olduğunu söyleyen bir hatayla bildirilir. Tip adı `TR_Tokenizer`.
- `p.TR_vocab_size` dağarcıktaki parça sayısıdır; `p.TR_token_id("<|endoftext|>")` parçanın kimliğini verir, dağarcıkta yoksa `TR_none`.
- `p.TR_encode(metin)` bir `TR_Int` listesi, `p.TR_encode([metin1, metin2])` listelerin listesini verir (liste tüm işlemci çekirdeklerinde paralel kodlanır, sıra korunur). Metnin içinde yazılı özel parçalar (`"<|endoftext|>"`) tek bir kimlik olarak tanınır.
  - `TR_special_tokens: TR_false` son işleyicinin eklediği özel parçaları (`[CLS] … [SEP]`, `<s>` gibi) kapatır; varsayılan `TR_true`dur.
  - `TR_pair: TR_String` metin çiftini (soru + bağlam gibi) tek dizi olarak kodlar; liste verilirse `TR_pair:` de aynı uzunlukta bir liste olmalıdır.
- `p.TR_decode(kimlikler)` metin verir; listelerin listesi ya da 2 boyutlu tensör verilirse metin listesi, 1 boyutlu tensör verilirse metin döner (ör. `.TR_next()` ile alınan `girdi` doğrudan çözülebilir). Varsayılan, başvuru kütüphanesindeki gibi **özel parçaları atlamaktır**; `TR_special_tokens: TR_true` onları da yazar. Dağarcıkta olmayan kimlikler sessizce atlanır.
- Desenlerdeki Unicode sınıfları (`\p{Latin}`, `\p{Alphabetic}`, `[[:punct:]]` ...) ve büyük/küçük harf duyarsız eşleşme (`(?i)`), dosyaları üreten başvuru uygulamasının Unicode sürümünü ve sınıf tanımlarını kullanır (ör. `[[:punct:]]` ASCII simgelerini de kapsar, köşeli parantezsiz `(?i)\p{Lu}` katlanmaz); başvurunun tanımadığı bir sınıf adı yüklemede hata verir.
- Güvenlik: parçalayıcı dosyasındaki düzenli ifadeler adım bütçesiyle çalışır. Bütçe bir `kodla` (ya da `.TR_decode`) çağrısının bütün aramalarına birlikte uygulanır ve metnin uzunluğuyla orantılıdır; böylece tek tek zararsız görünen ama toplamda karesel ya da üstel büyüyen aramalar da durur. Kötü niyetli bir metin aşırı geri izlemeye yol açarsa yarım sonuç yerine `dene/yakala` ile yakalanabilen bir "adım sınırını aştı" (ya da "geri izleme belleği sınırını aştı") hatası oluşur; motor çağrı yığınını kullanmadığından uzun girdi süreci çökertmez. Üstel geri izlemeye açık (iç içe sınırsız niceleyicili) ve 1000 düzeyden derin iç içe desenli dosyalar yüklenmez; daha hafif riskler yüklemede standart hataya dosya başına bir kez `Uyarı:` olarak yazılır. Parçalayıcı dosyası güvenilmeyen girdi sayılır ([`SECURITY.md`](../../SECURITY.md)).

**Veri kümesi**
- `TR_var küme = TR_Dataset("veri/train")` klasördeki `.npy` veri dosyalarını ad sırasıyla tanır (tek bir `.npy` dosyası da verilebilir). Veri dosyaları tek boyutlu, küçük uçlu `uint16`, `uint32` ya da `int32` parça kimliği dizileridir (standart `.npy` dosya biçimi). Klasörde ya da üst klasörde `meta.json` varsa veri dosyalarının parça sayıları oradan okunur; dosya başlıkları açılışta okunmaz. Tip adı `TR_Dataset`.
- **Önbellek:** veri yavaş bir sürücüdeyse (ağ, akışlı bulut sürücüsü) `TR_Dataset(dizin, TR_cache: "onbellek/veri", TR_quota_gb: 20)` veri dosyalarını kullanılmadan önce yerel klasöre kopyalar; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. `TR_cache:` verilince `TR_quota_gb:` **zorunludur** (disk alanını sessizce doldurmamak için); verilmezse hata, bu küme için gereken en küçük kotayı (aynı anda yerelde tutulan 8 veri dosyası × en büyük veri dosyası) söyler. Aynı önbellek klasörünü birden çok küme (ör. eğitim ve doğrulama) paylaşabilir: kota ortaktır (en son verilen geçerlidir) ve kümelerin örnekleyicileri birlikte sığmıyorsa hata gereken toplamı söyler. Bir önbellek klasörünü aynı anda yalnızca bir TR programı kullanabilir; ikinci program açık bir hata alır. Önbellek klasörü de diğer yollar gibi çözülür (çalışma klasörü, yoksa programın klasörü). Önbellekteki kopyalar belleğe eşlenerek okunur; eğitim sürerken bir kopya dışarıdan kısaltılırsa program çökmez, `.TR_next()` açık bir hata verir. Önbelleksiz okumada veri dosyaları belleğe eşlenmez, her pencere ayrı okunur: yalnızca istenen baytlar sürücüden çekilir.
- `küme.TR_sampler(TR_batch_size: 32, TR_context: 1024, TR_seed: 1)` eğitim için rastgele pencereler üretir (tip `TR_Sampler`; `tohum` varsayılanı 0). Aynı tohum ve küme hep aynı toplu işleri verir. Her dönemde veri dosyası sırası karışır, pencereler üst üste binmez (bir dönemde her parça en çok bir kez görülür) ve hiçbir pencere veri dosyası sınırını aşmaz; örnekleyici dönemden döneme sonsuz üretir. Pencereler `bağlam + 1` parçadır ve veri dosyasını rastgele bir başlangıç kaymasıyla böler: her veri dosyasının `bağlam + 1`'e bölünmeyen artığı (en çok `bağlam` parça) o dönem kullanılmaz ve her pencerenin ilk parçası yalnızca girdi olur (hedef olmaz). Kayma her dönem değiştiğinden bunlar dönemden döneme farklı parçalardır; ama veri dosyaları bağlama göre kısaysa kayıp büyüktür (ör. 2000 parçalık veri dosyası ve `TR_context: 1500` ile dönem başına dosyanın %25'i kullanılmaz), bu yüzden veri dosyalarını bağlamdan çok uzun tutun.
- `küme.TR_sequential(TR_batch_size: 32, TR_context: 1024)` doğrulama için veri dosyalarını sırayla okur (tip `TR_SequentialReader`): ardışık pencereler `bağlam` kadar kayar, böylece her parça en çok bir kez hedef olur: her veri dosyasının ilk parçası ve son pencereye sığmayan kuyruğu (en çok `bağlam - 1` parça) hedef olmaz, `bağlam + 1`'den kısa veri dosyaları hiç okunmaz. Son toplu iş daha az satırlı olabilir; veri bitince `.TR_next()` `TR_none` döner.
- `.TR_next()` `[girdi, hedef]` listesi verir: ikisi de `[toplu, bağlam]` şeklinde, tam sayı değerli tensördür ve `hedef` girdinin bir kaydırılmışıdır. Doğrudan `TR_embedding_lookup(E, girdi)` ve `TR_cross_entropy(çıktı, hedef)`'e verilir: `TR_var girdi, hedef = ö.TR_next()`. İkisi de tam sayılı işaretlidir: ondalıksız yazdırılır ve `.TR_tolist` `TR_Int` listesi verir, böylece liste indekslemede doğrudan kullanılır. Bu tensörlerle yapılan işlemlerin sonucu (ör. `TR_input + 1`) Ondalık değerlidir.
- **Önden yükleme her zaman açıktır:** sonraki 4 toplu iş arka planda hazırlanır. Bu iş TR kodu çalıştırmaz, yorumlayıcı kilidini tutmaz; `.TR_next()` beklerken kilit bırakılır, böylece `TR_async` görevler ilerler. Aynı okuyucuyu birden çok görev paylaşabilir (her toplu iş bir göreve gider).
- **Kaynaklar:** açık bir kapatma adı yoktur. Sıralı okuyucu bitince dosyalarını hemen kapatır; artık kullanılmayan (hiçbir değişkenin göstermediği) okuyucu ve kümeler çöp toplayıcı tarafından kapatılır; program bitince açık kalanların hepsi kapatılır (takılan bir ağ okumasında en çok birkaç saniye beklenir).
- Göreli yollar, dosya okuyan ve yazan bütün yerleşiklerde (`TR_read_file`, `TR_write_file`, `TR_import`, `TR_load_tokenizer`, `TR_Dataset`, `TR_read_csv`, `TR_save_tensors`, `TR_load_tensors`) aynı kuralla çözülür: göreli bir yol önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür ve dosyanın bulunduğu ilk yer kullanılır; dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir (düz bir `rapor.txt` için bu çalışma klasörüdür). Kural okuma ve yazma için aynıdır: programın yanındaki bir dosya okunup güncellenince yine oraya yazılır, yeni dosyalar çalışma klasöründe oluşur. Dosya bulunamazsa hata bakılan yerleri listeler.
- Hatalar Türkçe ve yol göstericidir (dosya yok, `.npy` değil, kota yetersiz, bağlam veri dosyalarından uzun, adlı argüman yazım hatası...) ve `dene/yakala` ile yakalanır.

```tr
# TR 4
TR_var p = TR_load_tokenizer("tokenizer.json")
TR_print(p.TR_encode("Merhaba dünya"), p.TR_vocab_size)
TR_var küme = TR_Dataset("veri/train")
TR_var ö = küme.TR_sampler(TR_batch_size: 8, TR_context: 256, TR_seed: 1)
TR_var E = TR_Tensor(TR_no_grad(TR_randn([p.TR_vocab_size, 64], TR_seed: 1) * 0.1), TR_requires_grad: TR_true)
TR_var W = TR_Tensor(TR_no_grad(TR_randn([64, p.TR_vocab_size], TR_seed: 2) * 0.1), TR_requires_grad: TR_true)
TR_var op = TR_AdamW([E, W], TR_lr: 0.01)
TR_for adım TR_in 0..100 {
    TR_var girdi, hedef = ö.TR_next()
    op.TR_zero_grad()
    TR_var kayıp = TR_cross_entropy(TR_embedding_lookup(E, girdi) @ W, hedef)
    TR_backward(kayıp)
    op.TR_step()
}
TR_var doğrulama = TR_Dataset("veri/_valid").TR_sequential(TR_batch_size: 8, TR_context: 256)
TR_for doğrulama.TR_next() != TR_none { }   # bitince yok döner
```

VS Code İzleme Masası'nda bu nesneler tip sütununda `Parçalayıcı (dağarcık 40000)`, `Veri kümesi (246 veri dosyası, 56,9 milyar parça)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)` gibi görünür; parçalayıcı (eklenmiş/özel parçalar) ve veri kümesi (veri dosyaları) tablo olarak açılabilir.

## E. Dosya ve Yol

Veri klasörünü gezmek, çıktı klasörü açmak, kontrol noktalarını yedeklemek ve büyük metin derlemlerini satır satır okumak için. Kurallar ve ayrıntılar: dil tanımı [§18](../spec/18_dosya_ve_yol.md).

- **Yol:** `TR_Path("çıktı", "model.trt")` parçaları `/` ile birleştirip temizler; `TR_Path` her platformda `/` ile yazılır. Yola sorulur: `.TR_name`, `.TR_extension`, `.TR_stem`, `.TR_parent`, `.TR_absolute`, `.TR_exists`, `.TR_is_file`, `.TR_is_dir`, `.TR_size`. İlk parçadan sonra mutlak bir parça hatadır (dışarıdan gelen bir ad taban klasörü değiştiremez). Yol alan her yerleşik (`TR_read_file`, `TR_write_file`, `TR_read_csv`, `TR_save_tensors`, `TR_Dataset` ...) `TR_String` yerine `TR_Path` da alır.
- **Listeleme:** `TR_list_dir("veri", "**/*.npy")` kalıba uyan dosya ve klasörleri her çalıştırmada aynı sırayla verir; `*`, `?`, `[a-z]`, `**` (alt klasörler). Gizli adlar (`.git`) kalıp nokta ile başlamadıkça listelenmez.
- **Klasör, silme, kopyalama, taşıma:** `TR_make_dir("çıktı/kontrol_noktaları")` (varsa sorun değil), `TR_copy("çıktı/model.trt", "yedek")` (→ `yedek/model.trt`), `TR_move(a, b)`, `TR_remove(yol)`. Güvenli varsayılanlar: var olan hedefin yerine koymak `TR_overwrite: TR_true`, dolu klasörü silmek `TR_recursive: TR_true` ister; `TR_remove` kökü, ev ve sistem klasörlerini, çalışma ve program klasörünü ve bunların üstlerini silmez, salt okunur dosyayı zorla silmez; bağlantılar (Windows dizin bağlantıları dahil) izlenmez. Kopya önce geçici bir ada yazılır, sonra yerine konur.
- **Geçici klasör:** `TR_temp_dir()` işletim sisteminin geçici klasöründe bu program için yeni, boş bir klasör verir; program bitince içindekilerle silinir (ara dosyalar, testler).
- **Büyük dosyalar:** `TR_for satır TR_in TR_read_lines("derlem.jsonl.gz") { ... }` dosyayı akışla okur (bellek dosya boyutundan bağımsız; tek satır en çok 64 MB). Sonuna kadar okumadan bırakılacak okuyucu `o.TR_close()` ile kapatılır (dosya böylece silinip taşınabilir). `.gz` uzantılı dosyalar `TR_read_file`, `TR_read_lines` ve `TR_read_csv`'da açılarak okunur, `TR_write_file`'da sıkıştırılarak yazılır.
- **Atomik yazma:** `TR_write_file` de `TR_save_tensors` gibi önce geçici dosyaya yazıp yerine koyar: yarıda kalan yazma eski dosyayı bozmaz. Bedeli: her yazma diske işlenir (çok sayıda küçük yazma eskisinden yavaştır) ve dosyanın sabit bağlantıları kopar.

```tr
# TR 4
TR_var çıktı = TR_make_dir(TR_Path("çıktı", "deneme_1"))
TR_for p TR_in TR_list_dir("veri", "**/*.npy") { TR_print(p.TR_name, p.TR_size) }
TR_var model = TR_Path(çıktı, "model.trt")
TR_if model.TR_exists { TR_copy(model, TR_make_dir("yedek"), TR_overwrite: TR_true) }
TR_var n = 0
TR_for satır TR_in TR_read_lines("veri/derlem.jsonl.gz") { n += 1 }
TR_print("\(n) satır")
```

## F. Eğitimde ayarlar, günlük ve ilerleme
Uzun bir eğitimi izlemek için üç yerleşik yeter: betiğin ayarlarını komut satırından alan `TR_command_line`, her adımı gösteren `TR_Progress` çubuğu ve sonradan bakılacak `TR_Logger` dosyası. Kurallar: [`docs/spec/19_surec_ve_kayit.md`](../spec/19_surec_ve_kayit.md). Uçtan uca örnek: `examples/tr/39_veri_ile_egitim.tr`.

```tr
# TR 4
TR_var ayar = TR_command_line({"step": 2000, "lr": 0.001, "günlük_dosyası": TR_none},
    TR_description: "Küçük dil modelini eğitir",
    TR_descriptions: {"step": "eğitim adımı sayısı", "günlük_dosyası": "ölçülerin yazılacağı .jsonl dosyası"})
TR_var g = TR_if ayar["günlük_dosyası"] != TR_none { TR_Logger(ayar["günlük_dosyası"]) } TR_else { TR_none }
TR_var ç = TR_Progress(ayar["step"], TR_log: g)
TR_for adım TR_in 0..ayar["step"] {
    TR_var kayıp = 1.0 / (adım + 1)   # burada modelin bir eğitim adımı çalışır
    ç.TR_update({"loss": kayıp, "lr": ayar["lr"]})
}
```

**Ayarlar (`TR_command_line`).** Sözlüğün anahtarları seçenek adları, değerleri varsayılanlardır; tip varsayılandan çıkar. `tr egit.tr --adım 5000 --öğrenme_oranı=3e-4` seçenekleri değiştirir, `tr egit.tr --yardım` hepsini açıklamalarıyla listeler. Yanlış yazılan seçenek (`--ogrenme_orani`, `--adim`) ya da tipine uymayan değer (`--öğrenme_oranı 0,001`) eğitim başlamadan, doğru adı öneren bir mesajla durdurur. Argüman vermeden çalıştırınca varsayılanlar kullanılır.

**İlerleme çubuğu.** Her `ç.TR_update(...)` bir adım ilerletir; çubuk terminalde tek satırda yenilenir, geçen adım sayısını, ölçüleri, adım hızını ve kalan süreyi gösterir. Çıktı bir dosyaya ya da not defteri hücresine gidiyorsa 10 saniyede bir düz satır yazılır. `TR_print` çubuğu bozmaz. Kaldığı yerden sürdürülen bir eğitimde `ç.TR_update(..., TR_step: n)` sayacı n'ye getirir.

```text
adım 1200/2000 ▕█████████▌░░░░░░▏ %60 · kayıp 2,31 · öğrenme_oranı 0,001 · 85,2 adım/sn · kalan 9 sn
adım 2000/2000 · kayıp 1,84 · 24 sn sürdü
```

**Günlük.** `TR_Logger("kayit/egitim.jsonl")` her satırı bir JSON kaydı olan bir dosya açar; `TR_Progress(..., TR_log: g)` her adımın ölçülerini oraya da yazar, `g.TR_append({"doğrulama_kaybı": d})` ayrı kayıt ekler. Dosyanın başında programın adı ve komut satırı ayarları durur: hangi ayarla hangi kaybın alındığı sonradan da okunur. Dosya varsa sonuna eklenir; kayıpta NaN belirirse gizlenmez, `"NaN"` olarak yazılır. Kesilen bir eğitimi kaldığı yerden sürdürmek için: [§9, uzun eğitim](09_uzun_egitim.md).

**Gizli değerler.** Erişim anahtarını ortam değişkeniyle verin: `TR_var belirteç = TR_env("HF_TOKEN")`. Adı `TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `ŞİFRE`, `ANAHTAR` gibi bir sözcük içeren değişken ve seçeneklerin değeri ekranda, hata mesajlarında, günlükte ve hata ayıklayıcıda `***` olarak görünür (`TOKENIZER_PATH` ya da `max_tokens` gibi adlar etkilenmez).
