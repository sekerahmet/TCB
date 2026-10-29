# 15. Tensörler ve otomatik türev

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "15. Tensörler ve otomatik türev".

Her kuralın emsali ve gerekçesi: [`rationale/15_tensors.md`](rationale/15_tensors.md).

Tensör, n boyutlu bir dizidir: öğeleri tek bir cinstendir (ondalık, tam sayı ya da mantık) ve cins tipin parçasıdır (`TR_Tensor[TR_Int]`). İşlemler hesap çizgesine kaydedilir; gradyanlar ters yönlü otomatik türevle hesaplanır. Aynı kurallar CPU'da ve GPU'da geçerlidir. Tensör dilin tek sayısal dizi türüdür; ayrı bir matris tipi yoktur. Ayrıntılı kullanım: [`docs/guide/04_yapay_zeka_ve_veri.md`](../guide/04_yapay_zeka_ve_veri.md).

## Tensör ve özellikleri

**15.1** Tensör tipi `TR_Tensor[E]`'dir: öğe cinsi `E` `TR_Float`, `TR_Int` ya da `TR_Bool`'dur; yalnız `TR_Tensor` yazımı `TR_Tensor[TR_Float]` demektir. Öğe cinsi tipin parçasıdır, hassasiyet değildir. Her cinsin hassasiyetleri `.TR_dtype`'in değeridir ve çalışırken bilinir:
- `TR_Float`:
  - hesap hassasiyetleri: "float32" (varsayılan), "bf16", "fp16", "float64";
  - nicemlenmiş saklama hassasiyetleri (int8 ile ölçek, 4 bit, fp8): değer ondalıktır, hesap float32'de yapılır; adları ve kuralları 15.51–15.55'te. Adları öbür cinslerin hassasiyetleriyle çakışmaz: "int8" yalnız `TR_Int`'indir;
- `TR_Int`: "int64" (varsayılan), "int32", "int16", "int8", "uint32", "uint16", "uint8";
- `TR_Bool`: "bool", öğe başına bir bayt.

"float64"'ü desteklemeyen cihazda float64 tensör kurmak ya da taşımak çalışma hatasıdır; ileti cihazı söyler ve "cpu"yu önerir.

Şekil de çalışırken bilinir, tipte değildir: şekil hatası çalışırken, şekilleri söyleyen iletiyle verilir (15.48).

**Genel fonksiyonda cins.** `TR_func f[E](t: TR_Tensor[E])`'de `E` yalnız üç cinsten biri olabilir. Kısıtsız `E`'de yalnız yapı işleri geçerlidir: özellikler (`.TR_shape`, `.TR_dtype`, `.TR_device`), görünümler, indeks ve dilim, indeksli hedefe atama, `.TR_fill`, `TR_write_slice`, `.TR_split`, `TR_concat`, `TR_where`'in iki dalı, `TR_Tensor(t)` (kopya), `.TR_to_device`, `TR_for` ile dolaşma, `.TR_tolist()`, `.TR_item()`, öğe öğe `==`/`!=` ve `TR_equal`. Aritmetik ve sıralama yalnız cins `TR_Float` ya da `TR_Int` diye açıkça yazılınca yapılır; türev ve ondalık matematik (`TR_relu`, `TR_softmax`) yalnız `TR_Float` alır. Yerleşiklerin imzasında 13.10'un sayı kısıtı tensörün cinsine de uygulanır; programın fonksiyonunda kısıt yazımı sonraki bölümlerdedir (13.10).

**Cinsi çalışırken bilinen tensör.** `TR_AnyTensor` cinsi tipte yazılı olmayan bir tensördür; dosyadan okunan sözlüğün değeri böyledir (15.46).
- Üstünde yalnız **okuma** işleri yapılır: `.TR_shape`, `.TR_dtype`, `.TR_device`, yazdırma, `TR_save_tensors`. Yazma, görünüm, `TR_concat`, `TR_where`, `==` ve hesap tiplenemez: önce daraltılır.
- **Daraltma** `t.TR_narrow[E]()` iledir: kopyasız ve çevirmeyen bir görünüm verir (`d["w"].TR_narrow[TR_Float]()`, `d["step"].TR_narrow[TR_Int]()`); cins uymazsa çalışma hatasıdır, ileti dosyadaki cinsi söyler. Tip çağrısı (`TR_Tensor(t)`) daraltma değildir: o kopya ve cins dönüşümüdür (15.3, 15.27a).
- Her `TR_Tensor[E]` değeri bir `TR_AnyTensor` yerine verilebilir (değerin kendisi; sözlük kurulurken). **Kaplar tip parametresinde değişmezdir:** `TR_Dict[TR_String, TR_Tensor]` bir `TR_Dict[TR_String, TR_AnyTensor]` değildir, çünkü ona tam sayı tensörü yazılabilirdi. Her cinsten sözlük alan yerleşikler bunu 13.10'un kabul kümesiyle yazar (`TR_save_tensors`).
- *Test:* TR testi; öğe cinsli tip, tam sayı tensörü ve float64 uygulanınca TR testi.

**15.2** Kurmak:
- **Veriden:** `TR_Tensor[E](veri)` tensör verisinden (13.10) yeni bir tensör kurar: `TR_Float`'ta ve `TR_Int`'te sayı ya da aralık, `TR_Bool`'da mantık değeri, ya da öğeleri bunlar olan iç içe liste. Aralık `TR_Int` verir: `TR_Tensor(0..n)` `TR_Tensor[TR_Int]`'tir. Tek sayı 0 boyutlu tensör verir: `TR_Tensor(3.0)`'ün şekli `[]`'dir. Düzensiz liste hatadır.
- **Öğe cinsi** sırayla şuradan gelir (6.10):
  - yazılan tip parametresi: `TR_Tensor[TR_Int](veri)`;
  - beklenen tip, ama 3.13'teki gibi yalnız değişmezdeki tam sayı yazımlarını ondalık yapar: `TR_var w: TR_Tensor = TR_Tensor([1, 2])` ondalıktır; `TR_var w: TR_Tensor = TR_Tensor(kimlikler)` denetim hatasıdır, ileti `TR_Tensor[TR_Float](kimlikler)`'i önerir;
  - verinin tipi: `TR_Tensor([1, 2])` ve `TR_Tensor(kimlikler)` (`kimlikler: TR_List[TR_Int]`) `TR_Int`, `TR_Tensor([1, 2.5])` `TR_Float`, `TR_Tensor([TR_true])` `TR_Bool` olur.
  - Veri yoksa (`TR_zeros`, `TR_ones`, `TR_randn`) ya da tipi çıkmıyorsa `TR_Float`'tır: 15.1'deki gibi `TR_Tensor` `TR_Tensor[TR_Float]` demektir. Boş liste `TR_Tensor([])` da `TR_Float`'tır.

  Tam sayı listesinden ondalık tensör sessizce kurulmaz: `TR_Tensor[TR_Float](kimlikler)` çağrısı cins dönüşümüdür (15.3) ve açıkça yazılır. `TR_Tensor[TR_Int]([1.5])` denetim hatasıdır.
- **Hassasiyet:** `TR_dtype:` argümanı cinsin içinde seçer (`TR_Tensor([1.5], TR_dtype: "bf16")`; `TR_zeros`, `TR_ones` ve `TR_randn`'da da). Hassasiyetin adı bir metindir; bilinmeyen ad (`"bfloat16"`) en yakın geçerli adı öneren bir hatadır. Değerler en yakına, eşitlikte çifte yuvarlanır. Başka cinsin hassasiyeti hatadır; ileti tip parametresini önerir: `TR_Tensor([1.5], TR_dtype: "int64")` → `TR_Tensor[TR_Int](...)`, `TR_zeros([3], TR_dtype: "int64")` → `TR_zeros[TR_Int]([3])`. `TR_Tensor([1, 2], TR_dtype: "uint8")` geçerlidir (cins veriden `TR_Int`).
- **Sığmak:** tam sayı hassasiyetine yazılan her değer o hassasiyete sığmalıdır: kurmakta (`TR_Tensor([300], TR_dtype: "uint8")`), `.TR_to_dtype`'ta, atamada ve bileşik atamada (15.6d). Sığmayan değer çalışma hatasıdır; ileti ilk öğenin indeksini ve değerini söyler. Sessiz sarma yoktur (15.49).
- **Tensörden:** `TR_Tensor(t)` kendi verisine sahip yeni bir yaprak kurar, cinsini ve hassasiyetini korur. Burada `TR_dtype:` verilmez (hassasiyetin tek yazımı `.TR_to_dtype`'tır); ileti `t.TR_to_dtype(...)`'yi önerir.
- **Hazır kurucular:**
  - `TR_zeros(şekil)`, `TR_ones(şekil)` ve `TR_randn(şekil, TR_seed: n)` şekli liste olarak alır (`[2, 3]`); `TR_zeros(2, 3)` hatadır, ileti `TR_zeros([2, 3])`'ü önerir.
  - `TR_zeros[TR_Int]([3])` gibi öğe cinsi yazılabilir; `TR_randn` yalnız `TR_Float`'tır. `TR_zeros[TR_Bool]` `TR_false`, `TR_ones[TR_Bool]` `TR_true`, `TR_eye[TR_Bool]` köşegende `TR_true` kurar. `TR_zeros(şekil, TR_dtype: "q8")` (ve "q4", "q4z128", "fp8") float bellek ayırmadan sıfır nicemlenmiş tensör kurar (ölçekler 0, kodlar sıfırın kodu): yükleme hedefi içindir; `TR_ones` ve `TR_randn` nicemlenmiş hassasiyette hatadır.
  - Ayrı bir `TR_arange` yoktur: PyTorch'un `arange`'ini yazan ajana ileti `TR_Tensor(0..n)`'yi önerir.
  - `TR_eye(n)` n×n birim matrisi kurar (`TR_Float`, `TR_eye[TR_Int](n)` de yazılabilir).
- **Dosyadan:** dosyadan okunan tensör `TR_AnyTensor`'dür (15.1, 15.46): cinsi dosyadan gelir, tipte yazılı değildir; hesap için `.TR_narrow[E]()` ile daraltılır. Hassasiyet dosyadakidir ("bf16" "bf16" kalır). Dosyadaki tür TR'de yoksa çalışma hatasıdır; ileti anahtarı ve dosyadaki türü söyler.
- **Türev:** `TR_requires_grad: TR_true` yalnız `TR_Float` tensörde geçerlidir (varsayılan `TR_false`).
- *Test:* TR testi.

**15.3** Özellikler ve dönüşümler:
- **Özellikler:** `.TR_shape` (`TR_List[TR_Int]`), `.TR_dtype` (`TR_String`), `.TR_device` (15.37), `.TR_grad` (`TR_Float` yaprakta `TR_Tensor?`; yaprak olmayan türevli tensörde okumak hatadır, 15.6d), `.TR_requires_grad` (`TR_Bool`; yaprakta atanabilir, 15.34: dondurma).
- **Liste ve sayı:** `t.TR_tolist()` ve `t.TR_item()` 13.8a'daki gibidir; öğe tipi tensörün cinsidir: `TR_Tensor[TR_Int]`'in `TR_tolist()`'i `TR_List[TR_Int]` verir. Ondalık öğe, geri okununca aynı değeri veren en kısa ondalık sayıya çevrilir: `TR_Tensor([0.1]).TR_item()` 0.1'dir, 0.10000000149011612 değil.
- **Hassasiyet:** `.TR_to_dtype(ad)` aynı cinsin içinde hassasiyeti değiştirir ve türevlenebilirdir; tensör zaten o hassasiyetteyse kendisini verir (15.39'un `.TR_to_device`'ı gibi). 16 bitlik yaprakların (parametrelerin) gradyanı float32'dir; 16 bitlik ara sonuçlarınki kendi türündedir. `.TR_to_dtype` ve görünümler kaynağın gradyan türünü izler: float32 bir ağırlığın "bf16" kopyasının gradyanı float32 biriktirilir ve ağırlığa yuvarlanmadan ulaşır. Başka cinsin hassasiyeti hatadır; ileti tip çağrısını önerir. Tam sayıda dar hassasiyete geçişte değer sığmalıdır (15.2'nin "Sığmak"ı).
- **Cins dönüşümü:** cinsler arası dönüşüm tip çağrısıdır (3.12):
  - `TR_Tensor[TR_Int](t)` ondalığı sıfıra doğru keser, 3.12'nin `TR_Int`'i gibi. Bir öğe bile NaN, sonsuz ya da hassasiyete sığmayan değerse bütün dönüşüm çalışma hatasıdır; ileti ilk böyle öğenin indeksini söyler.
  - `TR_Tensor[TR_Float](i)` tam sayıyı ondalığa çevirir. Tam sayıdan ondalığa her geçiş (bu dönüşüm ve tam sayı tensöründe `/`, 15.6) varsayılan olarak "float32"ye gider; 2²⁴'ü aşan değerler en yakın float32'ye yuvarlanır. Cins dönüşümü hedef hassasiyeti alır: `TR_Tensor[TR_Float](i, TR_dtype: "float64") / n`.
  - 3.12'deki gibi mantık değeri ile sayı arasında dönüşüm yoktur. Maskeden sayı `TR_where(m, 1.0, 0.0)` ile, sayıdan maske karşılaştırmayla (`t != 0`) kurulur. PyTorch'un `.float()`/`.long()`/`.bool()`/`.to(...)` alışkanlığına ileti doğru yazımı önerir.

  Kendiliğinden cins dönüşümü yoktur.
- **Tam sayı tensöründe taşma:** 3.1'deki gibidir: aritmetiğin sonucu hassasiyete sığmazsa çalışma hatasıdır ("uint8"de 200 + 100). İleti genişletmeyi (`.TR_to_dtype("int32")`) ya da kasıtlı sarmayı önerir: `TR_wrapping_add`, `TR_wrapping_sub` ve `TR_wrapping_mul` (13.1) tam sayı tensöründe hassasiyetin genişliğinde sarar. Sıfıra bölme `//` ve `%`'de hatadır (4.2a).
  - Tam sayı ve mantık tensörünün biriktirmesi (`TR_sum`, `TR_cumsum`) "int64"'te yapılır ve "int64" verir: "uint8" görüntünün toplamı taşmaz (15.18, 15.20).
  - Hata işlemin satırında verilir ve `TR_try` ile yakalanır (15.48). GPU'da bu, taşabilen tam sayı işleminden sonra cihazın beklenmesi demektir; hangi işlemlerin beklediği ve biriktirmenin ancak taşabilirse beklemesi 15.6'dadır. Sıcak döngüde sarmanın istendiği yer `TR_wrapping_*` ile yazılır; o beklemez.
- **Paylaşım:** tensör başvuruyla paylaşılır (3.6): `TR_var u = t` aynı tensördür, `u`'ya yerinde yazmak `t`'yi de değiştirir. Bu yüzden tensör içeren liste 4.10'a göre yinelenemez: `[t] * 3` denetim hatasıdır.
- **Sürüm:** her veri deposunun bir sürümü vardır; yerinde her yazma (atama, bileşik atama, `.TR_fill`, `TR_write_slice`, 15.6d) onu artırır. Geri yayılım için saklanmış bir tensör, girdi ya da çıktı (türev izlemeyen girdi dahil: `y = x @ w`'nin `x`'i; `TR_sigmoid`, `TR_tanh`, `TR_softmax`, `TR_sqrt` çıktılarını saklar), sonradan yerinde değiştiyse `TR_backward` çalışma hatası verir; ileti tensörün girdi mi çıktı mı olduğunu, işlemi ve sürümleri söyler. Sürüm tensörde değil depodadır: depoyu paylaşan görünümler de yakalanır.
- **Eksi eksen:** eksen alan her yerde (konumlu eksen argümanları ve `TR_dim:`; metotlar, `TR_concat`, `TR_split`, `TR_write_slice` ve 15.18–15.25'in işlemleri) eksi eksen sondan sayar (`-1` son eksen); sınır dışı eksen çalışma hatasıdır, ileti eksen sayısını söyler.
- **Yaprak olmayan türevli tensöre yerinde yazma yoktur:** türev izleyen bir işlemin sonucuna ya da onun görünümüne (`h = x @ w`; `h.TR_select(1, -1)`) yerinde yazmak türevsiz kipte de (`TR_no_grad` bloğu ve ondan çağrılanlar, 15.6d), yazıldığı satırda çalışma hatasıdır. İleti `TR_where` ile yerinde olmayan yazımı önerir. KV önbelleği, `TR_write_slice` hedefleri, iyileştiricinin ağırlıkları, EMA ve normun istatistikleri yaprak ya da türev izlemeyen tampondur ve türevsiz kipte yazılır; etkilenmez.
- **Görünümler:** `.TR_transpose()` (son iki eksen; PyTorch'un `.T`'si yazılınca ileti `.TR_transpose()`'u önerir), `.TR_permute(sıra)` ve `.TR_slice(eksen, baş, son)` veriyi kopyalamaz; görünüm de aynı cinstedir.
  - `.TR_slice(eksen, baş, son)` yarı açık `[baş, son)` aralığıdır; negatif uç sondan sayar; `son` verilmezse eksenin sonudur: `x[:, 1:]` → `x.TR_slice(1, 1)`, `x[:, :-1]` → `x.TR_slice(1, 0, -1)`. Sınır dışı uç hatadır.
  - `.TR_reshape(şekil)` (bir eksen −1 olabilir) adımlar izin veriyorsa görünümdür, vermiyorsa kopyadır (`t.TR_transpose().TR_reshape([-1])`). Yalnız boyu 1 eksen ekleyen ya da silen `.TR_reshape` her zaman görünümdür. **Kopya olan sonucun deposu yerinde yazmaya kapalıdır:** ona ya da görünümüne (`r[0][1] = d`, `r.TR_slice(...)`, dolaşmanın öğesi) atama, bileşik atama, `.TR_fill` ve `TR_write_slice` çalışma hatasıdır; yasak depodaki bir bayraktır; ileti şekli ve adımları söyler, `TR_Tensor(...)` ile açık kopyayı ya da asıl tensöre yazmayı önerir. Okumak serbesttir.
- **Koşul:** tensör koşul olamaz (3.10). İleti koşula göre önerir: `a == b` ise önce `TR_equal(a, b)`'yi; başka öğe öğe koşulda `TR_all(...)`'ı ya da `TR_any(...)`'ı (15.18a); 0 boyutlu mantık tensöründe `.TR_item()`'ı.
- *Test:* TR testi; cins dönüşümü, taşma ve öğe tipli liste uygulanınca TR testi.

**15.4** Karışık hassasiyet: `TR_mixed_precision(h) { ... }` sondaki blokla (6.7) yazılır. Blokta `@` ve `TR_attention` girdileri `h` hassasiyetine çevrilir ve float32 biriktirmeyle `h`'de sonuç verir; sonucun gradyanı da `h`'dedir. `TR_softmax`, `TR_log_softmax`, `TR_logsumexp`, `TR_sum`, `TR_mean`, `TR_std` ve `TR_cumsum` 16 bitlik girdiyi float32'ye açar. `TR_layer_norm` ve `TR_rms_norm` ortalamayı ve varyansı float32'de hesaplar, sonucu girdinin türünde verir. Ağırlıklar ve sapmalar float32 kalır. Blokta `@`'nin iki gradyan çarpımı 16 bitlik girdiyle yapılır; float32 ağırlığın gradyanı float32 yazılır. Normlar 16 bitlik girdiyi açmadan okur. Normun çıktısı `TR_dtype:` ile doğrudan blok türünde yazılabilir: `TR_rms_norm(x, w, TR_dtype: TR_compute_dtype())` (`TR_layer_norm` da); hesap yine float32'dedir, değer float32 çıktının o türe bir kez yuvarlanmışıdır; daraltılmış çıktının gradyanı float32 tutulur, ileri ve geri yön çarpımın kendi dönüşümüyle bit bit aynıdır. Çapraz entropi, normlar, evrişim ve dikkat blokta kendi maddelerindeki gibidir (15.23a–15.25). Türev izleyen bir ara sonucun aynı türe dönüşümü, ara sonuç değişmediği sürece bir kez yapılır ve paylaşılır. Karışık hassasiyet cinsin içindedir; tipi değiştirmez. Bloğun içinde `TR_compute_dtype()` bloğun hesap türünü (`"bf16"`, `"fp16"`), dışında `"float32"` verir. Bloğun değeri son ifadesidir. `"float32"` bloğu karışık hassasiyeti kapatır; iç içe bloklarda en içteki geçerlidir ve blok bitince (hatayla da) önceki tür geri gelir. Hesap türü göreve bağlıdır; yeni bir `TR_async` görevi açıldığı yerin hesap türünü ve türevsiz kipini alır (15.27).
- *Test:* TR testi.

**15.5** Dolaşmak, görmek ve karşılaştırmak:
- **Dolaşmak:** `TR_for x TR_in t` ve liste üreteci tensörü ilk ekseni boyunca dolaşır. Her boyutta öğe `t[i]` ile aynı, (N−1) boyutlu ve aynı cinsten bir görünümdür; 1 boyutlu tensörde 0 boyutlu tensördür. Boyut sayısı tipte olmadığından öğenin tipi böylece her zaman bellidir: `TR_Tensor[E]`. Görünüm veriyi paylaşır ve türev izler; `x`'e atama yalnız döngü değişkenini değiştirir (5.12), görünüme yerinde yazmak tensörü değiştirir. Sayılarla dolaşmak `TR_for x TR_in t.TR_tolist()` ile yazılır. 0 boyutlu tensörde dolaşmak hatadır.
- **Metin:** tensörün metni (3.14) onu kuran yazımdır, bayt dizisi (3.14) ve yapı (7.6) gibi: "Tensor([[1.5, 2.0], [3.0, 4.0]])".
  - Cins `TR_Float` değilse tip parametresi yazılır: "Tensor[Int]([1, 2])". Yazdırma ve eklentinin gösterimi cinsinin varsayılanı dışındaki hassasiyeti de yazar.
  - Hassasiyet varsayılan değilse "dtype: "bf16"" argümanı, türev izliyorsa "requires_grad: true" eklenir; cihaz CPU değilse sonda ".to_device("gpu:0")" gelir (cihazın uzun adı, 15.37).
  - Ondalık öğe 15.3'teki en kısa ondalıkla yazılır.
  - 1000'den çok öğeli tensörde uzunluğu 6'yı aşan her eksenin ilk ve son üçü yazılır, arada "..."; kısa eksen bütün yazılır (`[2, 4096]`'nın iki satırı da); başta şekil "shape: [2, 4096]" diye verilir.
- **Karşılaştırma:**
  - `==`, `!=`, `<`, `>`, `<=`, `>=` tensörde öğe öğedir (yayınla, 15.6b) ve `TR_Tensor[TR_Bool]` verir, türev izlemez. 0 boyutlu tensörün sayıyla karşılaştırması da 0 boyutlu `TR_Tensor[TR_Bool]`'dur; sayı isteyen yerde `t.TR_item() == 3.0` yazılır.
  - İki tensörün eşitliği (şekil dahil) `TR_equal(a, b)`'dir (15.18a). `TR_all(a == b)` yayınlı öğe eşitliğidir: `[1, 2]` ile `[2, 2]` şekilli iki tensörü uyarısız yayınlar ve doğru verebilir; bütün eşitlik için kullanılmaz. `TR_assert_equal(a, b)` iki tensörde `TR_equal`'ın anlamıyla sınar (13).
  - **İstisna:** her isteğe bağlı değerin (`T?`) `TR_none` değişmeziyle `==` ve `!=`'si geçerlidir ve `TR_Bool` verir (3.5), `T` eşitlenebilir olmasa da: `TR_Tensor?`, `TR_AnyTensor?` (`d.TR_get(ad)`), tensör içeren yapı ve demet. `TR_if g != TR_none { ... }`.
  - Karşılaştırma zinciri (4.4) tensörde denetim hatasıdır, çünkü `TR_and` yalnız `TR_Bool` alır (4.6). Maskeler `&`, `|`, `^` ve `~` ile birleşir (15.6): `(0.0 <= t) & (t < 1.0)`; ileti bu yazımı önerir.
  - Tensör ve tensör içeren her değer eşitlenebilir değildir (13.10): liste, sözlük, demet (3.6a), yapı (7.6) ve durumlu tip (7.7). Bunların `==`'si, `==` isteyen değer deseni (5.6) ve `.TR_contains`/`.TR_unique`/`.TR_index_of` (13.3) denetim hatasıdır; ileti `TR_equal`'ı önerir. Durum açan desen (7.7, `Cevap.Tamam(t) =>`) geçerlidir: alanı bağlar, karşılaştırmaz. İki isteğe bağlı tensör birbiriyle karşılaştırılamaz; yalnız `TR_none` değişmeziyle.
- *Test:* TR testi; metin, öğe öğe karşılaştırma ve `TR_Bool` maskesi uygulanınca TR testi.

## İşleçler, indeks ve atama

**15.6** İşleçler cinse göre tanımlıdır. Her işleç öğe öğedir ve yayınlar (15.6b); `@` dışında.

| İşleç | `TR_Float` | `TR_Int` | `TR_Bool` |
| :-- | :-- | :-- | :-- |
| `+ - *`, tekli `-` | aynı cins | aynı cins, taşma hata (15.3) | denetim hatası |
| `/` | aynı cins | `TR_Tensor[TR_Float]` "float32": tam sayıdan ondalığa geçiş kuralı (15.3); 4.2 gibi | denetim hatası |
| `//`, `%` | aynı cins, 4.2a'nın anlamı | aynı cins, sıfıra bölme hata (4.2a) | denetim hatası |
| `**` | aynı cins (15.6e) | aynı cins, eksi üs hata (4.3) | denetim hatası |
| `@` | aynı cins | denetim hatası | denetim hatası |
| `& \| ^ ~` | denetim hatası | bit bit (4.12) | mantık (ve, veya, dışlayan veya, değil) |
| `<< >>`, `TR_unsigned_shift_right` | denetim hatası | 4.12; sonucun hassasiyeti ve bit genişliği **sol işlenenden** gelir, sağ işlenen herhangi bir `TR_Int` hassasiyetinde yalnız miktardır (15.6c uygulanmaz); miktar 0 ile sol işlenenin bit genişliği eksi 1 arasında ("uint8"de 0–7); sol işlenen sayıysa 15.6a'ya uyar ve tensörün hassasiyetini alır (`1 << uint8` 8 bittir, miktar 8 hata; int64 isteyen önce tensörü çevirir, ileti bunu önerir) | denetim hatası |
| `< > <= >=` | `TR_Tensor[TR_Bool]` | `TR_Tensor[TR_Bool]` | denetim hatası |
| `== !=` | `TR_Tensor[TR_Bool]` (15.5) | `TR_Tensor[TR_Bool]` | `TR_Tensor[TR_Bool]` |

- İki tensör işlenen aynı cinsten olmalıdır (15.49); hassasiyetleri 15.6c'ye göre birleşir. `TR_Tensor[TR_Float]` ile `TR_Tensor[TR_Int]`'in işlemi (maskeli ortalama: `TR_sum(TR_where(m, x, 0.0)) / TR_sum(m)`) denetim hatasıdır; ileti `TR_sum(TR_where(m, 1.0, 0.0))`'ı önerir.
- `@` matris çarpımıdır: `[..., M, K] @ [K, N]`; toplu çarpımda öndeki eksenler yayınlanır; 1 boyutlu işlenen vektör sayılır (`[K] @ [K]` 0 boyutlu tensör verir). Nicemlenmiş saklama hassasiyetindeki ağırlık (15.1) `@`'ye sağ işlenen olarak girer, hesap float32'dedir (15.53).
- `TR_and`, `TR_or`, `TR_not` tensörde denetim hatasıdır (4.6); ileti `&`, `|`, `~`'yi önerir.
- **GPU'da bekleyen işlemler.** Hatası öğeye bağlı olan işlem hatayı kendi satırında vermek için GPU'da sonucu bekler: `TR_Int`'in `+ - *`'ı ve tekli `-`'si (taşma), `**` (eksi üs), `//` ve `%` (sıfıra bölme), `TR_abs` (en küçük değer), miktarı tensör olan kaydırma (aralık), `t[idx]` (sınır dışı indeks; atamada yinelenen indeks), `t[m]` (sonucun boyu), tam sayıya cins dönüşümü ve daraltan `.TR_to_dtype` (15.2, 15.3: sığmama, NaN), dar hassasiyete atama, tensör sınırlı `TR_clamp` (`a > b`), tensör başlangıçlı `TR_write_slice` (sınır), tam sayı toplamı ile `TR_cumsum` (taşma, 15.18, 15.20), çapraz entropinin hedef denetimi (15.23a, 15.25), `TR_sample`'ın dağılım denetimi (15.26), nicemlemenin NaN denetimi ve `TR_quantized`'ın aralık denetimi (15.52), `TR_clip_grad(…, TR_error_if_nonfinite: TR_true)` (15.36), `TR_embedding_lookup`'ın ve `TR_gather`'ın tensör indeksi (sınır dışı; 15.23, 15.21a). Eksensiz `TR_all`, `TR_any`, `TR_equal` ve `TR_all_close` `TR_Bool` verdiği için sonucu konağa okur (15.18a). **Bekleyen işlemlerin tek listesi budur;** öbür maddeler buraya gönderir. Toplam ve `TR_cumsum` ancak taşabilirse bekler: karar çağrı anında konakta, veri okunmadan verilir; indirgenen öğe sayısı (`TR_cumsum`'da eksenin boyu) çarpı hassasiyetin en büyük mutlak değeri int64'e sığıyorsa beklenmez (mantık ve uint8 toplamı pratikte hiç beklemez; int32 2³² ve üstü öğede, uint32 2³¹ ve üstü öğede bekler; int64 her zaman bekler). `TR_replay` kaydında bekleyen işlemler cihaz içi bir hata bayrağına yazar; bayrak oynatmanın sonunda bir kez okunur ve ileti kaydedilen satırı söyler (15.45). Ondalık aritmetik, karşılaştırma, mantık işleçleri, `@` ve `TR_wrapping_*` beklemez.
- Desteklenmeyen her birleşimin iletisi doğru yazımı önerir: `TR_Tensor[TR_Int]` ile `@` için `TR_Tensor[TR_Float](i) @ w`.
- *Test:* TR testi; cinsli işleçler uygulanınca TR testi.

**15.6a** Sayı işlenen. İşlecin bir yanı sayı olabilir, iki yanda da durabilir (`1 - t`, `2 / t`). Sonucun hassasiyeti tensörünkidir (`bf16 * 2.5` → bf16). **Hesap en az float32'dedir:** sayı "float32"ye (tensör "float64"se "float64"e) çevrilir, daraltılmaz; 16 bitlik tensör de float32'ye açılır, aritmetiğin sonucu tensörün hassasiyetine bir kez yuvarlanır. Karşılaştırmanın sonucu mantıktır, yuvarlanmaz: `TR_Tensor([1.0], TR_dtype: "bf16") > 0.999` doğrudur. fp16'da `x / 100000.0` sayı sonsuza taşmadan hesaplanır. Sayı işlenen kuralı `TR_wrapping_add`, `TR_wrapping_sub`, `TR_wrapping_mul` ve `TR_unsigned_shift_right`'a da uygulanır (biri tensör olmak üzere); yayın 15.6b'ye, hassasiyet 15.6c'ye göredir. Dar işaretli tensörde sığmayan onaltılık sabit ("int8" ile `0x80`) hatadır: hash işaretsiz hassasiyette yazılır. Nicemlenmiş saklama hassasiyeti öğe öğe işleme girmez: 15.53'teki sayılı işlemler dışında çalışma hatasıdır, açmak açıktır (`.TR_to_dtype("float32")`).
- `TR_Float` tensörde `TR_Int` ve `TR_Float` sayı geçerlidir (4.2'nin karışık sayı kuralı gibi);
- `TR_Int` tensörde yalnız `TR_Int` sayı geçerlidir ve hassasiyete sığmalıdır: `TR_Tensor[TR_Int]` ile `2.5` denetim hatası, "uint8" tensörle `300` ya da `-1` çalışma hatası;
- `TR_Bool` tensörde yalnız `TR_Bool` değer, yalnız `& | ^ == !=` ile.

Sayı tensörün cinsinden 0 boyutlu ve yukarıdaki hesap hassasiyetinde bir tensör gibi işlenir.

Liste işlenen değildir: `t + [10, 20]` denetim hatasıdır, ileti `t + TR_Tensor([10.0, 20.0])`'yi önerir. `t + TR_true` denetim hatasıdır.
- *Test:* TR testi; Go: `TestDivScalarKeepsDType`.

**15.6b** Yayın: şekiller sağdan hizalanır, her eksen eşit ya da 1 olmalıdır; eksik eksen 1 sayılır. Uymazsa çalışma hatasıdır, ileti şekilleri ve uymayan ekseni söyler. Eksen sayıları farklıyken iki taraf birden genişlerse (`[5]` ile `[2, 1]` → `[2, 5]`) sonuç verilir ve bir uyarı yazılır; uyarı aynı yerde bir kez yazılır. Eksen sayıları eşitken iki yanlı genişleme (`[T, 1] >= [1, T]`, nedensel maske; `.TR_unsqueeze` ile bilerek yazılır) uyarı vermez.
- *Test:* TR testi; Go: `TestBroadcastWarning`, `TestBroadcastWarningLimited`, `TestElementwiseAndBroadcast`.

**15.6c** Hassasiyet birleşmesi (iki tensör işlenende, aynı cins içinde). Yalnız hassasiyetler önemlidir, değerler değil:
- `TR_Float`: aynı hassasiyet o hassasiyette kalır; "bf16" ile "fp16", ve ikisinden biri ile "float32", "float32" verir; biri "float64" ise "float64"; nicemlenmiş saklama hassasiyeti öğe öğe işleme girmez (15.53). Hesap 15.6a'daki gibi en az float32'dedir.
- `TR_Int`: iki işaretli ya da iki işaretsiz hassasiyetin genişi; işaretsiz ile işaretlide ikisini de tutan en dar işaretli hassasiyet: "uint8" ile "int8" "int16", "uint16" ile "int16" "int32", "uint32" ile herhangi bir işaretli "int64".
- *Test:* TR testi; Go: `TestDTypeChoice`; tam sayı hassasiyetleri uygulanınca TR testi.

**15.6d** İndeks, dilim ve atama:
- `t[i]` (`TR_Int`) ilk eksenin i. öğesidir; bir eksen eksilir, negatif indeks sondan sayar, sınır dışı çalışma hatasıdır. `t[a..b]` ilk eksende dilimdir; uçlar 3.3'teki gibi kırpılır. İkisi de veriyi paylaşan, türev izleyen görünümlerdir (`t[i][j]`).
- **Tensörle indeks:** `t[idx]` (`idx: TR_Tensor[TR_Int]`) ilk eksenden öğeleri seçer; sonucun şekli `idx`'in şekli artı kalan eksenlerdir, veri kopyalanır, türev izlenir. Çok boyutlu tensörde bir eksen boyunca seçim (`TR_topk`'un indeksi, hedef dizgeciğin olasılığı) `TR_gather` iledir (15.21a). Sınır dışı bir indeks bütün işlemi çalışma hatası yapar; ileti ilk böyle indeksi söyler.
- **Maskeyle seçim:** `t[m]` (`m: TR_Tensor[TR_Bool]`, `t` ile aynı şekil) maskenin doğru olduğu öğeleri satır sırasıyla 1 boyutlu yeni bir tensörde verir; veri kopyalanır, türev izlenir. Şekil uymazsa çalışma hatası.
- **Başka eksen:** üç görünüm, üçü de veriyi kopyalamaz ve türev izler:
  - `.TR_slice(eksen, baş, son)` (15.3): `x[..., :d]` → `x.TR_slice(-1, 0, d)`;
  - `.TR_select(eksen, i)` o eksenin i. öğesini alıp ekseni düşürür: `x[:, -1]` → `x.TR_select(1, -1)`;
  - `.TR_unsqueeze(eksen)` o konuma 1 uzunluklu bir eksen ekler: `mask[:, None, :]` → `mask.TR_unsqueeze(1)`; negatif eksen sondan sayar (`-1` sona ekler).

  Çok eksenli indeks yazımı (`x[:, -1]`) yoktur; sözdizimi hatasının iletisi bu üçünü önerir.
- **Atama:** `t[i] = d`, `t[a..b] = d`, `t[idx] = d`, `t[m] = d` ve `x.TR_slice(...)[i] = d` (bir indeks hedefi, 5.4) veriyi yerinde değiştirir. `d` sayıysa (15.6a) hedefin bütün öğelerine yazılır; tensörse hedefe yayınlanabilmelidir ve aynı cinsten olmalıdır; liste değer hatadır, ileti `TR_Tensor(...)`'ı önerir; tam sayıda hedef hassasiyete sığmalıdır (15.2). `t[idx] = d`'de `idx`'te yinelenen indeks çalışma hatasıdır (hangi değerin kalacağı tanımsız olurdu). **Örtüşme:** kaynak hedefle aynı depoda örtüşüyorsa (`t[1..4] = t[0..3]`) önce kopyalanır; atama, bileşik atama ve `.TR_fill`'de, 15.8 gibi.
- **Bütününe yazmak:** `v.TR_fill(d)` tensörün ya da görünümün bütün öğelerine yerinde yazar ve değer döndürmez (13.10); `d` sayı ya da `v`'ye yayınlanan aynı cinsten tensördür. `TR_var son = x.TR_select(1, -1); son.TR_fill(0.0)` `x`'in son konumunu sıfırlar. `son = d` ise 5.4'teki gibi yalnız adı yeniden bağlar, `x`'e dokunmaz. `son *= 0.0` NaN ve sonsuzda NaN verir (15.50); sıfırlamak için `.TR_fill`.
- **Bileşik atama tensörü yerinde değiştirir:** `t op= u` (`+=`, `-=`, `*=`, `/=`, `//=`, `%=`, `**=`, `&=`, `|=`, `^=`, `<<=`, `>>=`) 5.4'ün listedeki istisnası gibidir; tensörü paylaşan her ad değişikliği görür (`TR_var q = p` iken `p -= g` `q`'yu da değiştirir). Sonucun cinsi ve hassasiyeti hedefinkidir ("bf16" `+=` "float32" "bf16" kalır); cinsi değiştirecek bileşik atama denetim hatasıdır: `TR_Tensor[TR_Int]`'te `/=` (`/` ondalık verir), ileti `//=`'yi önerir (5.4 gibi). `t[m] op= d` maskenin seçtiği öğeleri yerinde değiştirir (maskede yinelenen öğe olmaz); `t[a..b] op= d` görünüm kuralıyla yerindedir; sonucun şekli hedefin şekli olmalıdır, yayın yalnız `u` tarafındadır. `t[i] += d` ve `TR_var v = t.TR_slice(...); v += d` görünümü, dolayısıyla tensörü yerinde değiştirir; `t.TR_slice(...) += d` 5.4'e göre hedef değildir (çağrı), denetim hatası. Bileşik atama deponun sürümünü artırır (15.3).
- **Tensörle indekste bileşik atama biriktirir:** `t[idx] += d` ve `t[idx] -= d`, `idx`'te yinelenen indeksin her geçişini sayar: `sayaç[TR_Tensor([0, 0, 0])] += 1` → `[3, 0, 0]`. Bu, 5.4'ün "`x op= y`, `x = x op y` demektir" kuralının ikinci istisnasıdır (liste ve tensörden sonra); `t[idx] = t[idx] + d` yinelenenleri kaybederdi. Sıra indeks sırasıdır, sonuç belirlenimcidir (ondalıkta toplama sırası sabit). 16 bitlik ondalık hedefte aynı indekse düşen güncellemeler float32'de toplanır ve hedefin hassasiyetine bir kez yuvarlanır (15.6a'nın "hesap en az float32" kuralı). Öbür bileşik atamalar (`*=`, `/=`, `//=`, `%=`, `**=`, bit işleçleri) yinelenen indekste çalışma hatasıdır.
- **Paylaşımın tuzağı:** `TR_var kalıntı = h; h += f(h)` `kalıntı`'yı da değiştirir; başvuru anlamıdır (5.4'teki liste gibi). Artık bağlantıda `h = h + f(h)` yazılır.
- **Yaprak**, türev çizgesinde kaynağı olmayan tensördür. Türev izlemeyen her tensör yapraktır (kurucunun ve dosyanın tensörü, türevsiz kipte ya da türev izlemeyen girdilerle yapılan işlemin sonucu, `.TR_detach()`'ın sonucu). Türev izleyen tensör ancak `TR_requires_grad: TR_true` ile kurulduysa, `.TR_requires_grad = TR_true` ile açıldıysa ya da cihaza taşındıysa (15.39) yapraktır; türev izleyen bir işlemin sonucu ve onun görünümü yaprak değildir. Doğru ilkleme bu yüzden `TR_var w = TR_randn([D, D]) * 0.02` ve `w.TR_requires_grad = TR_true`'dur (15.34); `TR_randn([D, D], TR_requires_grad: TR_true) * 0.02` bir ara sonuçtur ve yapının alanında hata verir (15.33).
- **Türev.** Yerinde yazma (atama, bileşik atama, `.TR_fill`, `TR_write_slice`) türevde yoktur. **Türevsiz kip** `TR_no_grad { ... }` bloğunun çalıştığı süredir: bloktan çağrılan her fonksiyon (`TR_step`'in bloğundan çağrılan iyileştirici yardımcısı gibi) ve `TR_custom_grad`'ın ileri ve geri hesabı da bu kiptedir; kural sözcüksel değil, kipe bağlıdır. Üç kural:
  - türev izleyen **yaprağa** (ve görünümüne) yalnız türevsiz kipte yazılır (iyileştiricinin adımı, gradyan kırpma); dışında çalışma hatası, ileti `TR_no_grad`'ı önerir (`w = w - ...` ağırlığı sessizce yaprak olmaktan çıkarırdı);
  - **yaprak olmayan** türevli tensöre (türev izleyen bir işlemin sonucu ya da görünümü) yerinde yazmak türevsiz kipte de hatadır (15.3); ileti bütün hedefte `h = h + ...`'yı, indeksli hedefte `TR_where` ile yerinde olmayan yazımı önerir;
  - türev izlemeyen bir hedefe, türevsiz kipin dışında, **türev izleyen bir değer** yazmak hatadır: gradyan bağını sessizce koparırdı (`havuz[idx] += d`). EMA'nın gölge ağırlığı ve KV önbelleği bu yüzden türevsiz kipte güncellenir.
  - Türev izlemeyen ama geri yayılım için saklanmış bir tensöre yazmak serbesttir; `TR_backward` onu sürümden yakalar (15.3).
- *Test:* TR testi; Go: `TestMatrixIndexAndAssign`, `TestIndexAssignSingleCopy`; tensörle indeks, maske ve `.TR_select` uygulanınca TR testi.

**15.6e** Üs: `t ** u`'da `u` sayı ya da tensördür; `2 ** t` de geçerlidir. `TR_Float`'ta IEEE 754'ün `pow`'u uygulanır: eksi tabanın tam olmayan üssü NaN, sıfırın eksi üssü sonsuzdur (15.50; sayıdaki 4.3 bunları hata sayar, tensörde öğe başına hata yoktur, 15.50'deki `TR_sqrt` gibi). `TR_Int`'te üs eksi olamaz; bir öğe bile eksiyse çalışma hatası. Türev `TR_Float`'ta iki işlenene de akar.
- *Test:* Go: `TestGeluAndZeroPowerGrad`, `TestSpecialValuesTable`; tensör üs ve tam sayı tensörü uygulanınca TR testi.

**15.7** `TR_concat([a, b, ...], TR_dim: -1)` tensörleri verilen eksen boyunca uç uca ekler. Hepsi aynı cinsten (15.49) ve aynı eksen sayısından olmalı, birleşen eksen dışındaki eksenleri eşit olmalıdır; değilse şekilleri söyleyen çalışma hatası. Hassasiyetler 15.6c'ye göre birleşir. Sonuç türev izler. `t.TR_split(parça_sayısı, TR_dim: -1)` tensörü o eksende eşit parçalara böler, parçalar veriyi paylaşan, türev izleyen görünümlerdir; eksen uzunluğu parça sayısına bölünmüyorsa hata.
- *Test:* TR testi.

**15.8** `TR_write_slice(hedef, kaynak, TR_dim: -1, TR_start: 0)` kaynağı hedefin `TR_dim` eksenindeki `[TR_start, TR_start + kaynak.TR_shape[TR_dim])` dilimine yerinde yazar ve değer döndürmez (13.10).
- Hedef ile kaynak aynı cinsten olmalıdır.
- Hedef bitişik bir tensördür; görünüme yazmak için indeksle atama kullanılır (15.6d), ileti bunu söyler.
- Kaynağın eksen sayısı ve öbür eksenleri hedefinkiyle aynıdır; yayın yapılmaz. Uymazsa ileti kaynağın ve hedefin şeklini ve ekseni söyler.
- `TR_start` `TR_Int` ya da 0 boyutlu `TR_Tensor[TR_Int]`'tir. Dilim eksene sığmalıdır; sığmazsa ileti başlangıcı, dilimin ve eksenin uzunluğunu söyler.
- Kaynak hedefle örtüşüyorsa (hedefin bir dilimiyse) önce kopyalanır.
- Türevle ilişkisi 15.6d'nin üç kuralıdır (türevsiz kip).
- `hedef.TR_slice(...)[...] = kaynak` ile aynı işi yapar; farkı `TR_replay` kaydındadır: tensörle verilen başlangıç her oynatmada okunur ve sınır dışı değer kırpılmaz, cihaz içi bayrakla hata verir (15.45). Bu denetimler kayıt içinde ve dışında aynıdır.
- *Test:* TR testi; Go: `TestWriteSliceChecks`, `TestWriteSliceSameRulesInsideAndOutsideRecording`.

## Öğe öğe matematik

**15.9** Öğe öğe matematik: `TR_relu`, `TR_sigmoid`, `TR_tanh`, `TR_gelu` (15.17), `TR_silu`, `TR_sqrt`, `TR_exp`, `TR_log`, `TR_sin`, `TR_cos` ve 15.11'dekiler:
- yalnız `TR_Float` tensörde ve sayıda çalışır; tensörde türev izleyen aynı hassasiyette tensör, sayıda `TR_Float` verir (13.1);
- `TR_Tensor[TR_Int]` ya da `TR_Tensor[TR_Bool]` denetim hatasıdır, ileti cins dönüşümünü önerir (`TR_exp(TR_Tensor[TR_Float](i))`);
- liste denetim hatasıdır, ileti `TR_Tensor(...)`'ı önerir;
- 16 bitlik tensör float32'de hesaplanıp kendi hassasiyetinde döner.
- NaN girdide sonuç türev izlenen ve izlenmeyen yolda aynıdır (15.50).

`TR_softmax(x, TR_dim: -1)` ve `TR_log_softmax(x, TR_dim: -1)` verilen eksende çalışır, yalnız `TR_Float` tensörde. Bütün öğeleri −sonsuz olan (tamamen maskeli) satırın softmax'ı 0, log_softmax'ı −sonsuzdur, gradyanı 0'dır (15.24). NaN içeren satırın softmax'ı, log_softmax'ı ve gradyanı NaN'dır: NaN gizlenmez. İndirgemeler (`TR_sum`, `TR_mean`, `TR_max`, `TR_min`) 15.18'dedir.
- *Test:* TR testi.

**15.10** `TR_sin(x)` ve `TR_cos(x)` (radyan) 15.9'daki gibidir; türevleri `TR_cos(x)` ve `-TR_sin(x)`. Tensör sonuçları doğru yuvarlanmış float32'den en çok 1 ulp uzaktır.
- *Test:* TR testi.

**15.11** Genişletilmiş öğe öğe fonksiyonlar: `TR_silu` (`x · TR_sigmoid(x)`), `TR_tan`, `TR_asin`, `TR_acos`, `TR_atan` (radyan), `TR_sinh`, `TR_cosh`, `TR_log1p` (`TR_log(1 + x)`), `TR_log2`, `TR_log10` ve `TR_expm1` (`e^x − 1`) 15.9'daki gibidir.
- **Türevler:** `TR_silu` s + x·s·(1 − s) (s = `TR_sigmoid(x)`), `TR_tan` 1 + tan(x)², `TR_asin` 1/√(1 − x²), `TR_acos` −1/√(1 − x²), `TR_atan` 1/(1 + x²), `TR_sinh` `TR_cosh(x)`, `TR_cosh` `TR_sinh(x)`, `TR_log1p` 1/(1 + x), `TR_log2` 1/(x · ln 2), `TR_log10` 1/(x · ln 10), `TR_expm1` eˣ.
- **Tanım dışı değer** NaN verir (`TR_asin(2)`, `TR_log2(-1)`, `TR_log1p(-2)`); `TR_log1p(-1)` ve `TR_log2(0)` −sonsuz, `TR_silu(-sonsuz)` 0'dır.
- **Küçük x'te kararlılık:** `TR_log1p` ve `TR_expm1` küçük x'te kararlıdır: float32 tensörde `TR_log1p(1e-10)` ve `TR_expm1(1e-10)` 1e-10 verir, `TR_log(1 + x)` ve `TR_exp(x) - 1` ise 0. Sayıda (`TR_Float`) sonuçlar doğru yuvarlanmış float64 değerlerdir (9.999999999500001e-11 ve 1.00000000005e-10); `TR_log(1 + x)`'in göreli hatası ise ~8e-8'dir.
- **Doğruluk sınırları:** CPU'daki tensör sonuçları doğru yuvarlanmış float32 değerden en çok 1 ulp (`TR_silu` 3 ulp; x < −87'de sonuç normal altı bölgededir), GPU'dakiler birkaç ulp uzaktır. GPU'da `TR_tan`, `TR_sin` ve `TR_cos` gibi, |x| ≤ 10⁵ radyanda doğrudur.
- *Test:* TR testi; Go: `TestUnaryWideRef`, `TestUnaryWideEdges`, `TestUnaryWideGrad`, `TestUnaryWide`, `TestUnaryWideSpecialValues`.

**15.12** `TR_floor`, `TR_ceil` ve `TR_sign` sayıda (13.7) ve tensörde çalışır. `TR_Float` tensörde aynı cins verir; `TR_Int` tensörde `TR_sign` aynı cinsi verir, `TR_floor` ve `TR_ceil` girdinin kendisini verir (bir şey yapmaz, 13.7'deki gibi). `TR_sign` NaN'ı ve sıfırın işaretini korur (13.7: `TR_sign(-0.0)` → −0.0). Türevleri her yerde 0'dır. Listede denetim hatası.
- *Test:* TR testi.

**15.13** `TR_clamp(x, TR_min: a, TR_max: b)` x'i [a, b] aralığına sınırlar; NaN öğe NaN kalır. Sınırlamak için yazılan `TR_min(x, 0)` ya da `TR_max(x, 0)` (bir tensör ve bir sayı) hatadır; ileti `TR_clamp(x, TR_max: 0)`'ı ya da `TR_relu`'yu önerir. Sınırlardan biri de verilebilir; ikisi de yoksa, `a > b` ise ya da sayı sınır NaN ise hata. Sınırlar sayı ya da `x`'e yayınlanan aynı cinsten tensördür; sayı 15.6a'ya uyar: `TR_Int` tensörde `TR_Int` sınır. Tensör sınırda yalnız `a > b` öğe öğe sınanır (GPU'da bekleme, 15.6); sınırdaki NaN öğe hata değildir, yayılır (`TR_maximum` gibi). Hassasiyetler 15.6c'ye göre birleşir. Cins korunur. Sayı sınırda `x`'in türevi sınırlar dahil aralıkta 1, dışında 0, NaN'da NaN'dır. Tensör sınırda `TR_clamp(x, TR_min: a)` `TR_maximum(x, a)` ile, `TR_max: b` `TR_minimum(x, b)` ile aynıdır, türevi dahil (15.16a: seçilen tarafa, tam eşitlikte yarı yarıya). `TR_max(x, 0)` öğe öğe sınır değildir; ileti `TR_clamp(x, TR_min: 0)`'ı önerir.
- *Test:* TR testi.

**15.14** `TR_atan2(y, x)` (x, y) noktasının açısını (−π, π] aralığında radyan olarak verir. İşaretli sıfırlar ve sonsuzlar IEEE kuralına uyar; `TR_atan2(0, 0)` 0'dır. İki sayıda `TR_Float`; en az biri tensörse yayınlar ve türev izleyen `TR_Float` tensör verir. Türevler ∂/∂y = x/(x² + y²), ∂/∂x = −y/(x² + y²)'dir; (0, 0)'da ikisi de 0 alınır.
- *Test:* TR testi.

**15.15** `TR_where(koşul, a, b)` öğe öğe seçimdir: `koşul` `TR_Tensor[TR_Bool]`'dur; doğru olduğu yerde `a`'yı, öbür yerlerde `b`'yi verir. Üçü yayınlanır. `a` ve `b` aynı cinsten tensör ya da 15.6a'ya uyan sayıdır; hassasiyet 15.6c'ye göre birleşir. `a` ve `b`'nin ikisi de sayıysa cins ve hassasiyet onlardan çıkar: iki `TR_Int` → `TR_Int` "int64"; `TR_Int` ile `TR_Float` → `TR_Float` "float32"; iki `TR_Bool` → `TR_Bool`. Gradyan `a`'ya koşulun doğru, `b`'ye yanlış olduğu yerlerden akar; seçilmeyen dala yukarıdan gelen gradyan tam 0'dır, yukarıdan gelen NaN seçilmeyen dala geçmez. **Ama** dalın kendi türevi o yerde sonsuz ya da NaN ise 0 · sonsuz = NaN girdiye ulaşır: `TR_where(x > 0.0, TR_log(x), 0.0)`'ın `x = 0`'daki gradyanı NaN'dır. Doğru yazım girdiyi içeride maskelemektir: `TR_where(x > 0.0, TR_log(TR_where(x > 0.0, x, 1.0)), 0.0)`. Koşul türev almaz. Hiçbiri tensör değilse denetim hatası, ileti `TR_if`'i önerir. Ondalık ya da tam sayı koşul denetim hatasıdır; ileti `TR_where(t != 0, ...)`'ı önerir.
- *Test:* TR testi; mantık koşulu uygulanınca TR testi.

**15.16** `TR_abs(x)` (13.1) tensörde de çalışır: `TR_Float` ve `TR_Int`'te aynı cins. İşaret biti silinir (`TR_abs(-0.0)` +0), NaN korunur, CPU ve GPU aynı bitleri verir. `TR_Int`'te en küçük değerin mutlak değeri taşmadır (15.3). Türev `TR_sign(x)`'tir; 0'da 0.
- *Test:* TR testi.

**15.16a** Öğe öğe iki tensör ve sınama:
- `TR_maximum(a, b)` ve `TR_minimum(a, b)` iki tensörün (ya da tensörle sayının, 15.6a) öğe öğe en büyüğünü ve en küçüğünü verir; yayınlar, cins aynı olmalıdır. NaN yayılır (15.50). ±0'da sıralama sırası (15.22b) kullanılır: `TR_maximum(-0.0, 0.0)` argüman sırası ne olursa olsun `0.0`'dır. Türev seçilen tarafa gider; tam eşitlikte yarı yarıya; ±0 eşit sayılmadığı için seçilen tarafa. Bir konumda işlenenlerden biri NaN ise o konumda iki işlenene de NaN türev gider.
- `TR_round(t)` 13'ün yarımı çifte kuralıyla öğe öğe yuvarlar. Tensörde sonuç `TR_Float`'tır, `TR_floor` gibi (sayıdaki `TR_round` ise 13.1'e göre `TR_Int` verir). NaN ve sonsuz korunur, öğe başına hata ve bekleme olmaz. `TR_Int` tensörde girdiyi verir. Basamaklı `TR_round(x, basamak)` yalnız sayıdadır. Nicemleme `TR_Tensor[TR_Int](TR_round(t))` yazar. Türevi 0'dır.
- `TR_is_nan(t)` ve `TR_is_finite(t)` (13.7) `TR_Float` tensörde öğe öğe `TR_Tensor[TR_Bool]` verir. `t != t` de NaN'ı bulur ama okunmaz; ileti yok, ikisi de geçerli.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.17** `TR_gelu(x, TR_approximate: a)`: `TR_approximate: TR_true` tanh yaklaşımlı GELU, `TR_approximate: TR_false` tam (erf tabanlı) GELU'dur.
- **`TR_approximate:` zorunludur, varsayılanı yoktur ve yalnız adla verilir**: `TR_gelu(x, TR_true)` denetim hatasıdır. Eksikse denetim hatası; ileti iki biçimi de gösterir ve PyTorch'un `F.gelu(x)`'inin tam biçim olduğunu söyler.
- **Formüller:** tanh biçimi 0.5·x·(1 + tanh(√(2/π)·(x + 0.044715·x³))); tam biçim x·Φ(x) = 0.5·x·(1 + erf(x/√2)) (Φ standart normal dağılım işlevi). İki biçim en çok ~5e-4 farklıdır.
- **Türevler ve uç değerler:** tam biçimin türevi Φ(x) + x·φ(x)'tir (φ standart normal yoğunluğu). `TR_gelu(-sonsuz)` 0, `TR_gelu(sonsuz)` sonsuzdur, türevleri 0 ve 1; NaN yayılır.
- **Hesap yolu:** CPU'da tam biçim float64 erfc ile hesaplanıp float32'ye bir kez yuvarlanır (doğru yuvarlanmış değerden en çok 1 ulp); 1 + erf değil erfc kullanıldığı için x çok negatifken (Φ(x) ≈ 0) göreli doğruluk korunur. GPU'da Φ(x) iki bölgeli bir yaklaşımla hesaplanır: |x|/√2 < 0.5'te erf'in 7 terimli Maclaurin serisi, ötesinde erfc(t) = e^(−t²)·g(t)/(t·√π) (g, 1/(1+t) değişkeninde 12. dereceden Chebyshev serisi; üs x²/2'nin hi+lo bölünmesiyle alınır). PTX benzetiminde ölçülen en büyük hata (|x| ≤ 14): ileri değerde göreli 2.7e-7, türevde mutlak 9e-8.
- *Test:* TR testi; Go: `TestGeluExactErfFormula`, `TestGeluExactGradEdges`, `TestGeluExactAccuracy`.

## İndirgeme, arama ve sıralama

**15.18** İndirgemeler: `TR_sum`, `TR_mean`, `TR_std`, `TR_max`, `TR_min` ve `TR_logsumexp`. Aynı şekilli tensörlerin listesinde `TR_sum` ve `TR_mean` tensörleri öğe öğe katlar (kayıpları toplamak); iç içe liste ya da listeyle `TR_dim:` hatadır. Boş tensör listesinde `TR_sum` ve `TR_mean` çalışma hatasıdır: şekil, hassasiyet ve cihaz bilinmez.
- **Yazım:** `TR_sum(t, TR_dim: k, TR_keepdim: b)`. Eksen yalnız adla verilir; `TR_sum(t, 1)` denetim hatasıdır, ileti `TR_sum(t, TR_dim: 1)`'i önerir. Negatif eksen sondan sayar; olmayan eksen çalışma hatası.
- **Eksen verilmezse** bütün öğeler indirgenir ve 0 boyutlu tensör çıkar.
- **Birden çok eksen:** `TR_dim:` bir `TR_Int` ya da `TR_List[TR_Int]` alır (13.10: dönüş aynı, parametrede kabul kümesi): `TR_mean(x, TR_dim: [1, 2])`. Bunu `TR_sum`, `TR_mean`, `TR_std`, `TR_max`, `TR_min`, `TR_logsumexp`, `TR_all` ve `TR_any` kabul eder. Aynı eksen iki kez (sondan sayılınca da: 2 boyutta `[1, -1]`) ya da olmayan eksen çalışma hatası. Boş liste değişmezi `[]` denetim hatası, değişkenle gelen boş liste çalışma hatası; ileti eksensiz yazımı önerir.
- **`TR_keepdim: TR_true`** indirgenen ekseni 1 uzunlukta korur (`[B, T, D]` → `[B, T, 1]`), sonuç girdiye yayınlanabilir. Eksen yoksa bütün eksenler 1 olarak korunur.
- **Sonucun cinsi ve hassasiyeti:**

| | `TR_Float` | `TR_Int` | `TR_Bool` |
| :-- | :-- | :-- | :-- |
| `TR_sum` | aynı hassasiyet | "int64" | `TR_Int` "int64" (sayım) |
| `TR_mean` | aynı hassasiyet | `TR_Float` "float32" (15.3) | `TR_Float` "float32" (oran) |
| `TR_std` | aynı hassasiyet | denetim hatası | denetim hatası |
| `TR_max`, `TR_min` | aynı hassasiyet | aynı hassasiyet | denetim hatası |
| `TR_logsumexp` | aynı hassasiyet | denetim hatası | denetim hatası |

  - `TR_Int` ve `TR_Bool` toplamı "int64"'te kesindir. **Matematiksel toplam** "int64"e sığmazsa çalışma hatasıdır (GPU'da bekleme, 15.6); ara toplamın taşması sayılmaz, böylece sonuç toplama sırasından ve cihazdan bağımsızdır (`[MAX, 1, -1]`'in toplamı MAX). Kasıtlı sarmanın indirgemesi yoktur; çok büyük toplam için önce `TR_Float`'a çevrilir.
  - `TR_mean` `TR_Int` ve `TR_Bool`'da "int64"'te kesin toplar, sonucu "float32"ye bir kez yuvarlar (15.3'ün tek kuralı): doğruluk oranı kesin sayımdan tek yuvarlamayla çıkar.
  - 16 bitlik girdi en az float32'de biriktirilir; sonuç girdinin hassasiyetine bir kez yuvarlanır.
  - 13.1'in telafili (Neumaier) toplaması listeye özgüdür; tensör toplamının doğruluğu 15.22a'dadır. `TR_sum([1e16, 1.0, -1e16])` 1.0 verir, tensörü 0.0 verebilir. `TR_mixed_precision` bloğunda `TR_sum`, `TR_mean`, `TR_std`, `TR_logsumexp` ve `TR_cumsum` 16 bitlik girdiyi float32 verir (15.4).
  - 16 bitlik toplam ve `TR_cumsum` sonucun hassasiyetine sığmazsa sonsuzdur (fp16'da 65504'ün üstü; IEEE 754 gibi). Sonucu okumadan hata verilemez, ondalık işlem beklemez (15.6). Büyük toplam float32'ye çevrilerek ya da `TR_mean` ile alınır.
- **Anlamlar:**
  - `TR_std` örneklem sapmasıdır, n−1'e böler ve iki geçişlidir: önce ortalama, sonra farkların kareleri (13.1 gibi). İndirgenen eksende iki öğeden azsa çalışma hatası. Sapması 0 olan eksende türevi 0'dır (`TR_abs`'ın 0'daki ve `TR_atan2(0, 0)`'ın türevi gibi).
  - `TR_logsumexp(t, TR_dim: k)` `TR_log(Σ TR_exp(t))`'yi taşmadan hesaplar (en büyük öğe çıkarılarak). Özel değerler: bütün öğeleri −sonsuz olan satır ve boş satır −sonsuz, türevi 0 (15.9'un maskeli satırı gibi; boş toplamın logaritması); +sonsuz öğe varsa +sonsuz, türevi limitteki gibi o öğeye 1, öbürlerine 0 (15.50'nin "Limitler"i); birden çok +sonsuzda türev ilk +sonsuza 1, öbürlerine 0 (`TR_max` ve `TR_argmax` ile aynı öğe, belirlenimci); NaN varsa (+sonsuzla birlikte de) NaN.
  - `TR_max` ve `TR_min` NaN dışındaki sayılarda sıralama sırasını (15.22b) kullanır: −0.0 0.0'dan küçüktür. **NaN varsa sonuç NaN'dır:** indirgeme NaN'ı yayar, sıralama ise onu en büyük sayar; ikisi bilerek ayrıdır (15.22b). Eşitlerde ilk öğe alınır; türev o öğeye gider (`TR_argmax` ile aynı öğe).
  - `TR_max` ve `TR_min` eksenle de **yalnız değerleri** verir; indeks `TR_argmax`/`TR_argmin`'dir. `TR_max(x, TR_dim: 1)[0]` değerlerin ilk satırıdır.
- **Boş:** indirgenen öğe sayısı sıfırsa (eksensiz biçimde boş tensör, eksenli biçimde uzunluğu 0 olan indirgenen eksen) `TR_sum` 0, `TR_logsumexp` −sonsuzdur; `TR_mean`, `TR_std`, `TR_max` ve `TR_min` çalışma hatasıdır (13.1'deki gibi; değerleri tanımsız). İndirgenen eksen boş değilse sonuç boş bir tensördür: `[0, 4]`'te `TR_dim: 1` → `[0]`.
- **Özel değerler** 15.50'deki gibidir. Yalnız −0'lardan oluşan toplam +0'dır.
- **Türev** `TR_Float`'ta izlenir.
- **İki tensörün öğe öğe en büyüğü** `TR_maximum`'dur (15.16a). `TR_max(t, u)` ya da `TR_max(t, 0)` denetim hatasıdır; ileti `TR_maximum`'u ya da `TR_clamp`'i önerir.
- *Test:* TR testi; cinsli sonuçlar, `TR_std` ve `TR_logsumexp` uygulanınca TR testi.

**15.18a** Bütün tensör hakkında soru:
- `TR_all(m)` ve `TR_any(m)` bir `TR_Tensor[TR_Bool]`'un bütün öğelerinin doğru olup olmadığını ya da birinin doğru olup olmadığını sorar ve `TR_Bool` verir; koşulda kullanılır: `TR_if TR_all(t >= 0.0) { ... }`. İki tensörün eşitliği `TR_all(a == b)` değil `TR_equal(a, b)`'dir: `==` yayınlar, şekilleri farklı iki tensör doğru çıkabilir (15.5).
  - `TR_dim: k` verilirse o eksende (ya da eksenlerde) indirger ve `TR_Tensor[TR_Bool]` verir; `TR_keepdim: TR_true` indirgenen ekseni 1 uzunlukta korur (`TR_any(TR_is_nan(b), TR_dim: -1, TR_keepdim: TR_true)` → `[B, T, 1]`, girdiye yayınlanır). Eksensiz ve eksenli biçim ayrı imzadır (13.10, dönüşleri ayrı); eksensiz biçim `TR_keepdim:` almaz.
  - Boş tensörde `TR_all` `TR_true`, `TR_any` `TR_false`'tur.
  - Ondalık ya da tam sayı tensör denetim hatasıdır; ileti `TR_all(t != 0)`'ı önerir.
- `TR_equal(a, b)` iki tensörün şekli ve bütün öğeleri aynıysa `TR_true` verir.
  - Cinsler aynı olmalıdır (denetim hatası); hassasiyet farkı değeri karşılaştırmaya engel değildir.
  - NaN hiçbir şeye eşit değildir, −0.0 0.0'a eşittir (4.5).
  - `TR_assert_equal(a, b)` tensörde bunu sınar (15.5).
- `TR_all_close(a, b)` şekiller aynıysa ve her öğede `|a − b| ≤ TR_atol + TR_rtol · TR_max(|a|, |b|)` ise `TR_true` verir (simetrik; 12.4a ile aynı formül); yalnız `TR_Float`. Yakınlık öğe öğedir: iki öğe de sonluysa formül uygulanır; sonsuz bir öğe yalnız aynı işaretli sonsuza yakındır; sonlu ile sonsuz ve zıt işaretli iki sonsuz yakın değildir; NaN hiçbir şeye yakın değildir. Şekiller aynı değilse `TR_false`; yayın yoktur.
  - Varsayılan toleranslar iki işlenenin daha az hassas olanına göredir; `TR_rtol:` ve `TR_atol:` üstüne yazar:

    | hassasiyet | `TR_rtol` | `TR_atol` |
    | :-- | :-- | :-- |
    | "float64" | 1e-7 | 1e-7 |
    | "float32" | 1.3e-6 | 1e-5 |
    | "fp16" | 1e-3 | 1e-5 |
    | "bf16" | 1.6e-2 | 1e-5 |

- Bu dördünün `TR_Bool` veren eksensiz biçimleri GPU'da sonucu konağa okur; bir beklemedir. Eksenli `TR_all`/`TR_any` tensör verir, beklemez.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.19** `TR_argmax(t, TR_dim: k)` ve `TR_argmin(t, TR_dim: k)` en büyük ve en küçük öğenin indeksini `TR_Tensor[TR_Int]` "int64" olarak verir, türev izlemez.
- **Eksen verilmezse** bütün öğeler üstünde çalışır ve düzleştirilmiş tensördeki indeksi 0 boyutlu tensörde verir (15.18'in "eksensiz bütün öğeler" kuralı). Eksen verilirse o eksen düşer; `TR_keepdim: TR_true` onu 1 uzunlukta korur (`[B, 1]`). Tek eksen alır; eksen yalnız adla (`TR_dim:`) verilir. `TR_keepdim:` yalnız eksenli imzadadır; eksensiz biçimde denetim hatasıdır.
- Sıra 15.18'in `TR_max`'ı gibidir: eşitlerde ilk, −0.0 0.0'dan küçük (`TR_argmax(TR_Tensor([-0.0, 0.0]))` 1), NaN varsa ilk NaN'ın indeksi (`TR_argmin` için de: NaN indirgemede yayılır, 15.18). Bu yüzden NaN'lı tensörde `TR_argmin(t)` ile `TR_argsort(t)[0]` ayrılır.
- İndirgenen öğe sayısı sıfırsa çalışma hatası (15.18'in `TR_max`'ı gibi).
- `TR_Bool` tensörde denetim hatası.
- *Test:* TR testi; eksensiz biçim ve `TR_argmin` uygulanınca TR testi.

**15.20** `TR_cumsum(t, TR_dim: k)` her konumda o konuma kadarki (kendisi dahil) öğelerin toplamını verir; şekil korunur. **`TR_dim:` zorunludur.**
- Cins ve hassasiyet 15.18'in `TR_sum`'ı gibidir: `TR_Float` aynı hassasiyet (16 bitlik girdi float32'de toplanır), `TR_Int` ve `TR_Bool` "int64", taşma çalışma hatası.
- Türev `TR_Float`'ta izler; gradyan gelen gradyanın sondan birikimli toplamıdır.
- Toplama sırası yalnız şekle bağlıdır, sonuç belirlenimcidir. 0 boyutlu tensörde çalışma hatası.
- *Test:* TR testi.

**15.21** `TR_topk(t, k, TR_dim: -1)` verilen eksende en büyük `k` öğeyi büyükten küçüğe bir **demet** olarak verir (3.6a): `TR_var v, ix = TR_topk(t, 5)`.
- `v` `t`'nin cinsinde ve hassasiyetindedir, türev izler (gradyan seçilen konumlara gider). `ix` `TR_Tensor[TR_Int]` "int64"'tür.
- İkisinin şekli `t`'ninkidir, yalnız eksenin uzunluğu `k`'dır.
- Sıra: sıralama sırası (15.22b), büyükten küçüğe; NaN en büyüktür, −0.0 0.0'dan küçüktür, eşitlerde küçük indeks önce gelir; sonuç belirlenimcidir. `TR_topk(t, 1)` `TR_max(t)` ile aynı öğeyi verir.
- `k` `TR_Int`'tir, 0 ile eksen uzunluğu arasında olmalıdır. `TR_Bool` tensörde denetim hatası.
- *Test:* TR testi; demet dönüşü uygulanınca TR testi.

**15.21a** `TR_gather(t, idx, TR_dim: k)` `t`'den `k` ekseni boyunca `idx`'in gösterdiği öğeleri seçer: `idx` `TR_Tensor[TR_Int]`'tir ve `t` ile aynı eksen sayısındadır, öbür eksenlerinde `t`'ye eşit ya da 1'dir (yayın). Sonucun şekli `idx`'inkidir.
- `TR_dim:` zorunludur, yalnız adla verilir.
- Sınır dışı indeks çalışma hatasıdır; ileti ilk böyle indeksi söyler (15.6'nın bekleyen işlemleri gibi).
- Türev `t`'ye gider; geri yönde aynı öğeye düşen gradyanlar belirlenimci biriktirilir (15.22a).
- İki kullanım: `TR_topk`'un indeksini sözlüğe çevirmek (`TR_gather(üst_indeksler, örnek.TR_unsqueeze(-1), TR_dim: -1)`) ve kayıpta hedef dizgeciğin log-olasılığı (`TR_gather(log_olasılık, hedef.TR_unsqueeze(-1), TR_dim: -1)`).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.22** `TR_argsort(t, TR_dim: -1, TR_descending: TR_false)` eksendeki öğeleri dizen indeksleri `TR_Tensor[TR_Int]` "int64" olarak verir; şekil `t`'ninkidir, türev izlemez.
- Sıralama kararlıdır: eşitlerde küçük indeks önce gelir, artan sırada da azalan sırada da.
- Sıralama sırası 15.22b'dir: artan sırada NaN'lar sondadır, azalan sırada başta (15.21 ile tutarlı); −0.0 0.0'dan küçüktür.
- `TR_descending: TR_true` olasılıkları büyükten küçüğe dizer (top-p örnekleme).
- GPU'nun sonucu CPU'nunkiyle aynı indekslerdir (sıralama kararlı, sıra 15.22b). Bir cihazın sıralayamadığı eksen çalışma hatası verir; sessizce yanlış sıralanmaz.
- `TR_Bool` tensörde denetim hatası.
- *Test:* TR testi; `TR_descending:` uygulanınca TR testi.

**15.22a** Belirlenimcilik (Güvenilir). **Aynı cihaz türünde** (aynı GPU modeli, ya da aynı CPU özellik kümesi: mimari ve FMA gibi komut kümeleri) aynı girdi ve şekil, her indirgemede ve sıralamada aynı bitleri verir. CPU'nun iş parçacığı sayısı sonucu değiştirmez: toplama sırası yalnız şekle bağlıdır.
- **Cihaz türleri arasında** bit eşitliği söz verilmez: ondalık toplamın biriktirme yolu CPU'da ve GPU'da ayrıdır (15.50'nin sınır cümlesi) ve son bitlerde ayrılabilir. GPU'nun SM sayısına göre ayarlanan yolları (normun geri yönü, GEMM) farklı kart modellerinde ayrılabilir.
- Tam sayı ve mantık indirgemeleri, `TR_max`/`TR_min`/`TR_argmax`/`TR_argmin`, `TR_topk` ve `TR_argsort` cihazdan bağımsız aynı sonucu verir.
- Cihaz türleri arasındaki farkın sınırı 15.41'dedir.
- *Test:* Go: `TestSumParallelPaths`, `TestSumAllParallel`, `TestParallelKernelsMatchSerial`, `TestDeterministic`.

**15.22b** Sıralama sırası. Tanımı 13.10'dadır ("sıralanabilir" kısıtının yanında: −0.0 < 0.0, NaN her sayıdan büyük; işleçler, 4.4 ve 4.5, ise IEEE 754 karşılaştırmasıdır). Tensörde bu sırayı **bütünüyle** kullananlar: `TR_topk` ve `TR_argsort`. `TR_max`, `TR_min`, `TR_argmax`, `TR_argmin`, `TR_maximum` ve `TR_minimum` (15.16a) onu NaN dışındaki sayılarda kullanır (−0.0 < 0.0); NaN'ı ise yayar ya da ilk NaN'ı seçer (15.18, 15.19).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

## Sinir ağı işlemleri

**15.23** `TR_embedding_lookup(gömme_tablosu, indeksler)` `[V, D]` tablosundan satır seçer; `indeksler` `TR_Tensor[TR_Int]`'tir, sonucun şekli indekslerin şekli artı `[D]`'dir.
- Türev tabloya gider; aynı satıra düşen gradyanlar belirlenimci biriktirilir (15.22a). Sınır dışı indeks çalışma hatasıdır; ileti ilk böyle indeksi söyler (15.6'nın bekleyen işlemleri).
- *Test:* TR testi.

**15.23a** `TR_cross_entropy(z, hedef)` son eksen sınıf olmak üzere ortalama kaybı 0 boyutlu bir tensör olarak verir. Hedefin cinsi iki biçimi ayırır:
- **Sınıf hedefi** `TR_Tensor[TR_Int]` (logitin son ekseni düşmüş şekilde): her satır için bir sınıf indeksi. **`-100` olan hedef yok sayılır.** Başka eksi hedef ve sınıf sayısına eşit ya da büyük hedef çalışma hatasıdır. Kayıp geçerli hedefler üstünde ortalamadır; hepsi yok sayılırsa ortalama tanımsızdır ve hatadır. Bu iki denetim öğeye bağlıdır: 15.6'nın bekleyen işlemlerindendir (`TR_replay`'de bayrak).
- **Olasılık hedefi** `TR_Tensor[TR_Float]` (logitle aynı şekil): her satır bir dağılımdır, kayıp `−Σ hedef · TR_log_softmax(z)`'in satır ortalamasıdır. Damıtma böyle yazılır: `TR_cross_entropy(öğrenci / T, TR_softmax(öğretmen / T, TR_dim: -1))`. Satırın toplamı ve öğelerin işareti denetlenmez: ondalık işlem beklemez (15.6); olasılık hedefi çoğunlukla `TR_softmax`'tan gelir.
- Hesap en az float32'dedir; `TR_mixed_precision` bloğunda da kayıp float32'dir (15.4).
- *Test:* TR testi; Go: `TestCrossEntropyForward`, `TestCrossEntropy16Bit`; `-100` kuralı, öbür negatiflerin hatası ve olasılık hedefi uygulanınca TR testi.

**15.23b** Normlar son eksende (grup normu kanal ekseninde) normalleştirir; ortalama ve varyans float32'de hesaplanır, sonuç girdinin türündedir (15.4).
- `TR_layer_norm(x, ağırlık, sapma)` ve `TR_rms_norm(x, ağırlık)`: `ağırlık` ve `sapma` `[D]` ya da `TR_none`.
- `TR_group_norm(x, ağırlık, sapma, TR_num_groups: g)`: `TR_num_groups:` zorunludur, yalnız adla verilir; `x` `[B, C, ...]`, `C` `g`'nin katı olmalıdır; her grup kendi kanallarında ve uzay eksenlerinde normalleştirilir; `ağırlık` ve `sapma` `[C]` ya da `TR_none`. Yayılım modellerinin ve evrişimli ağların normudur.
- `TR_eps:` varsayılanı `0.00001`; float32'de pozitif ve sonlu olmalıdır (float32'de 0'a düşen `1e-50` hatadır).
- *Test:* Go: `TestNorms16Bit`, `TestNormsOutputDType`, `TestNormEpsilonMustBeFinite`; `TR_group_norm` uygulanınca TR testi.

**15.23c** Evrişim: `TR_conv1d(x, w, b)` ve `TR_conv2d(x, w, b)`.
- Şekiller kanal önde: `x` `[B, Cin, L]` ya da `[B, Cin, H, W]`; `w` `[Cout, Cin / g, K]` ya da `[Cout, Cin / g, Kh, Kw]` (`g` grup sayısı); `b` `[Cout]` ya da `TR_none`. PyTorch'un ve ONNX'in düzenidir: hazır modellerin ağırlıkları dönüştürülmeden alınır (15.47, 20).
- Adlı argümanlar: `TR_stride: 1`, `TR_padding: 0`, `TR_dilation: 1`, `TR_num_groups: 1`. 2 boyutta ilk üçü bir `TR_Int` ya da iki öğeli demet (`(sh, sw)`); dolgu sıfırdır. Grup sayısının adı iyileştiricinin parametre gruplarından (`.TR_groups`) ayrıdır: aynı ad iki anlam taşımaz (17.5).
- Hesap `@` gibidir: float32 biriktirme, `TR_mixed_precision` bloğunda blok türü (15.4); türev girdiye, ağırlığa ve sapmaya gider.
- Şekil uymazsa (kanal sayısı, gruplar, çekirdek girdiden büyük) çalışma hatasıdır; ileti şekilleri söyler.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.23d** `TR_dropout(x, oran)` her öğeyi `oran` olasılıkla sıfırlar, kalanları `1 / (1 − oran)` ile ölçekler; türev aynı maskeden geçer.
- `oran` 0 ile 1 arasındadır (1 hariç); 0'da sonuç `x`'in kendisidir (kopyasız).
- **Tohum yoktur:** maske paylaşılan üreteçten (20.17) çekilir; kesilen eğitim kaldığı yerden aynı maskeyle sürer. Görevler aynı üreteçten çekerse 10.8'in sınırı geçerlidir.
- Eğitim ile çıkarımın ayrımı programındır: dropout çıkarımda çağrılmaz. Bir kip bayrağı yoktur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.24** `TR_attention(q, k, v, TR_causal: TR_true, TR_window: 0, TR_mask: TR_none, TR_bias: TR_none)` birleşik ölçekli nokta çarpımı dikkatidir: `TR_softmax(q @ kᵀ / √dh + TR_bias) @ v`'yi, bakılmayan yerler (maske, nedensellik, pencere) dışarıda kalarak, aynı sonuçla (float32 yuvarlama farkıyla) ve aynı gradyanlarla hesaplar.
- **Şekiller:** `q` `[..., Tq, H, dh]`, `k` `[..., Tk, Hkv, dh]`, `v` `[..., Tk, Hkv, dv]`; öndeki eksenler eşit olmalı, sonuç `[..., Tq, H, dv]`. 2 boyutlu girdiler tek bakışlıdır (`[T, dh]`). Bu, doğrusal katmanın çıktısının şekillendirilmiş hâlidir: `(x @ Wq).TR_reshape([B, T, H, dh])` doğrudan verilir, sonuç `.TR_reshape([B, T, H * dv])` ile birleşir.
- `Hkv < H` gruplu sorgu dikkatidir: `H`, `Hkv`'nin katı olmalı; ardışık `H / Hkv` bakış aynı `k`/`v` bakışını kullanır (kopyasız). Ölçek her zaman `1/√dh`'dir.
- **Hassasiyet:** `q`, `k`, `v` ve `TR_bias` hassasiyetleri birlikte 15.6c'deki gibi birleşir; blokta 15.4.
- `TR_causal: TR_true` sorgu `i`'yi `Tk − Tq + i`'ye kadarki anahtarlarla sınırlar (sondan hizalı). Sondan hizalama `Tk` geçerli anahtar sayısıyken doğrudur: büyüyen önbellek ya da geçerli kısmın görünümü (`k.TR_slice(1, 0, n)`). **Sabit kapasiteli önbellekte** (`TR_replay`'in sabit şekli, 15.45) `Tk` kapasitedir: yazılmamış yuvalar maskeyle dışarıda bırakılır, `TR_causal: TR_false` ve `TR_mask` (kitaplığın önbelleği maskeyi verir, `lib/katman.tr`). Yazılmamış yuvaya bakmak hata vermez, sonucu değiştirir. `TR_window: w > 0` sorguyu son `w` konuma, **kendisi dahil**, sınırlar; `TR_causal: TR_false` ile pencere iki yönlüdür: `|Tk − Tq + i − j| < w`.
- `TR_mask` `[..., H, Tq, Tk]`'ye yayınlanabilen bir `TR_Tensor[TR_Bool]`'dur: `TR_true` olan yere bakılır; türev izlemez. 0/1 sayı tensörü denetim hatasıdır, ileti karşılaştırmayı önerir.
- `TR_bias` `[..., H, Tq, Tk]`'ye yayınlanabilen bir `TR_Tensor[TR_Float]`'tur ve puanlara eklenir (T5'in göreli konum sapması, ALiBi); türev izler. Maske ve sapma ayrı argümandır, karışmaz. **Önce maske:** bakılmayan yerdeki sapma okunmaz (NaN ve ±∞ dahil). Bakılabilen anahtarların hepsinde puanı −∞ olan satır (sapmadan ya da taşmadan) 0 verir, gradyanı 0'dır: 15.9'un softmax'ıyla ve bu maddenin denkliğiyle aynı. Sapmanın kendisi ve gradyanı `Tq × Tk` boyludur; `TR_mixed_precision` bloğunda float32'de eklenir.
- Hiçbir anahtara bakamayan satırın çıktısı 0'dır. Puanları NaN olan satırın çıktısı ve gradyanı NaN'dır; maskeli satırın 0'ıyla karışmaz. Maskelenen anahtardaki NaN satıra sızmaz.
- Hesap tek bir türev düğümüdür; `Tq × Tk` matris tutulmaz (bellek uzunlukla doğrusaldır), pencere ve nedensellik dışındaki bloklar hesaplanmaz, geri yayılım puanları yeniden hesaplar. Bir cihaz bu düzeni doğrudan hesaplayamıyorsa girdiler içeride görünümle çevrilir; sonuç aynıdır. `TR_mixed_precision` bloğunda `@` gibidir (15.4).
- Bilinmeyen adlı argüman (`TR_scale:`) hatadır; şekil hatalarının iletisi şekilleri söyler.
- *Test:* TR testi.

**15.25** `TR_linear_cross_entropy(x, w, hedef)` `TR_cross_entropy(x @ w, hedef)` ile aynı kaybı ve gradyanları verir (float32 yuvarlama farkıyla), ama logit matrisini bütün olarak hiç tutmaz: büyük sözlüklü dil modelinin çıkış katmanı içindir.
- `x` `[..., D]`, `w` `[D, sınıf]` (`x @ W` düzeni); paylaşımlı gömmede `tablo.TR_transpose()` (görünüm) verilir. Şekiller uymazsa ileti iki şekli söyler.
- Hedef ve hata kuralları 15.23a'nın sınıf hedefinkidir (`-100` yok sayılır; denetimler bekleyen işlemlerdir).
- `TR_mixed_precision` bloğunda `x` ve `w` `@` gibi blok türüne çevrilir (15.4); logitler blok türünde, kayıp float32'de hesaplanır.
- Bellek bir dilimin logitleri ve gradyanı kadardır; dilimin boyu kullanıcı seçeneği değildir. Logitlerin gradyanı ileri geçişte dilim dilim hesaplanır, geri yayılımda logitler yeniden hesaplanmaz. Türevsiz kipte yalnız kayıp hesaplanır.
- *Test:* TR testi; Go: `TestLinearCrossEntropy`, `TestLinearCrossEntropyMatchesCrossEntropyOfMatMul`, `TestLinearCrossEntropyNoGradAndShapes`, `TestLinearCEIgnoredChunkHasZeroXGrad`.

**15.26** `TR_sample(z, TR_temperature: 1.0, TR_top_k: 0, TR_seed: TR_none)` son eksendeki `TR_softmax(z / TR_temperature)` dağılımından bir indeks örnekler; sonuç son ekseni düşmüş bir `TR_Tensor[TR_Int]` "int64"'tür (1 boyutlu girdide 0 boyutlu), türev izlemez.
- `TR_top_k: k > 0` en olası `k` sınıf arasından seçer (eşitlikte küçük indeks önce, 15.21); `0` hepsi.
- `TR_temperature: 0` en olasıyı verir (`TR_argmax(z, TR_dim: -1)` ile aynı; eksensiz argmax düzleştirirdi, 15.19). Eksi sıcaklık ve eksi `k` hatadır.
- Dağılım kurulamayan satır (NaN, `+∞`, hepsi `−∞`) hatadır; sessizce bir indeks seçilmez.
- Aynı tohum aynı sonucu verir; tohumsuz çağrı paylaşılan üreteçten çeker (20.17). Söz 15.22a'nın "aynı cihaz türü" sınırıyla geçerlidir: aynı GPU modeli ya da aynı CPU özellik kümesi. FMA'lı ve FMA'sız x86 işlemci (çekirdek çalışırken seçilir) ya da `GOAMD64=v3` ile `v1` derlenmiş iki `tr` ayrı türdür: son basamaktaki fark örneklenen indeksi değiştirebilir.
- Top-p `TR_gather` ile yazılır (15.21a).
- *Test:* TR testi.

## Türev

**15.27** `TR_backward(L)` 0 boyutlu ya da tek öğeli bir kayıptan geri yayılım yapar; yaprak tensörlerin `.TR_grad`'ı birikir: her geri yayılım yeni gradyanı öncekine ekler. Adımlar arasında gradyanlar iyileştiricinin `.TR_zero_grad()`'ıyla sıfırlanır (15.34).
- **Çizge bir kez geri yayılır:** geri yayılım çizgenin ara sonuçlarını bırakır; aynı çizgeden ikinci geri yayılım hatadır, ileti çizgeyi yeniden kurmayı söyler.
- `.TR_grad` yalnız türev izleyen yaprakta vardır; ara sonucun `.TR_grad`'ı hatadır, ileti nedenini söyler. Hiç gradyan almamış yaprakta `TR_none`'dır (tipi `TR_Tensor?`); `.TR_zero_grad()`'ın sıfırladığı gradyan sıfır tensörüdür, `TR_none` değildir.
- `.TR_grad` gradyan deposunun **canlı takma adıdır**: sonraki geri yayılım, `.TR_zero_grad()` ve `TR_clip_grad` onu değiştirir ve deponun sürümünü artırır (15.3). Saklamak için `TR_Tensor(x.TR_grad)` ile kopyası alınır.
- **Türevsiz kip** (15.6d'nin tanımı): `TR_no_grad { ... }` bloğu türev izlemeden çalışır ve bloğun değerini döndürür; bloktan çağrılan her fonksiyon da bu kiptedir. Kip **göreve bağlıdır** (10): blok kilidi devrettiğinde öbür görevler türev izlemeye devam eder. Yeni bir `TR_async` görevi açıldığı yerin kipini ve `TR_mixed_precision` türünü alır (15.4): `TR_no_grad { TR_async üret(...) }` türevsiz üretir. Görev açıldıktan sonra açanın kipinin değişmesi ona geçmez (10.4'ün "görev başlatıldığı kapsamı yakalar"ı gibi). `TR_no_grad` bloğundaki `TR_defer` (9.1) kip kapanmadan, kipin içinde çalışır.
- Türev izlemeyen bir kayıptan geri yayılım hatadır. Bir geri yayılım sürerken başka bir görev aynı çizgeden geri yayılım başlatırsa hatadır.
- *Test:* TR testi.

**15.27a** `x.TR_detach()` tensörü çizgeden ayırır: verisi `x` ile paylaşılan, türev izlemeyen bir görünümdür. Yazma kuralları depoya bağlıdır (15.3, 15.6d): ayrılmış görünüm üstünden türev izleyen bir yaprak olmayan tensörün deposuna yazmak da hatadır. Yaprağın ayrılmış görünümüne türevsiz kipte yazmak serbesttir (iyileştirici, 15.6d'nin birinci kuralı). Bağımsız kopya `TR_Tensor(x)`'tir.
- `TR_no_grad` yalnız blok biçimindedir; `TR_no_grad(x)` biçimi yoktur: bir ad iki iş yapmaz (kip ve ayırma; 2.9). Bu biçim yazılınca ileti `x.TR_detach()`'i önerir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.28** `TR_custom_grad(TR_forward: f_ileri, TR_backward: f_geri)` yeni bir işlemin ileri ve geri hesabını programın yazmasını sağlar ve bir fonksiyon değeri döndürür: parametreleri `f_ileri`'ninkidir, dönüşü `çıktı`'nın tipidir (13.10).
- Türevsiz kipin dışında çağrılan `f_ileri` kapanıştan türev izleyen bir tensör okursa çalışma hatasıdır; ileti onu konumlu argüman yapmayı söyler (15.44'ün `TR_fuse`'u gibi). Türev izlemeyen tablo ve sabit okunabilir.
- Çağrıldığında `f_ileri` verilen argümanlarla **türevsiz kipte** çalışır ve iki öğeli bir **demet** (3.6a) döndürür: `(çıktı, saklananlar)`. `çıktı` bir tensör ya da tensör demetidir (15.30). `saklananlar` geri hesabın ihtiyaç duyduğu değerdir (bir tensör, sıradan bir değer ya da bunların demeti); açılmaz, `f_geri`'ye olduğu gibi verilir. Liste yerine demet: öğelerin tipleri ayrıdır ve çalışmadan önce bilinir (1.2).
- Konumlu tensör girdilerinden en az biri türev izliyorsa ve türevsiz kipte değilsek çıktı çizgeye **tek bir düğüm** olarak kaydedilir; `f_ileri`'nin ara sonuçları tutulmaz, yalnız `saklananlar` yaşar. Hiçbir girdi türev izlemiyorsa çıktı olduğu gibi döner.
- Türev yalnız konumlu tensör girdileri için hesaplanır; `f_ileri`'nin dışarıdan yakaladığı tensörlere gradyan akmaz.

```tr
# TR 4
TR_var kare = TR_custom_grad(
    TR_forward: (x) => (x * x, x),
    TR_backward: (saklanan, g) => g * 2.0 * saklanan
)
TR_var x = TR_Tensor([3.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(kare(x)))
TR_print(x.TR_grad)
```

- *Test:* TR testi; demet biçimi uygulanınca TR testi.

**15.29** `f_geri(saklananlar, g)` geri yayılımda türevsiz kipte çalışır; `g` çıktının gradyanıdır (çıktıyla aynı şekil; 16 bitlik çıktıda çıktının türünde).
- `f_geri` her konumlu girdi için girdiyle **aynı şekilde** bir gradyan ya da `TR_none` döndürür: **tek girdide kendisi, birden çokta girdi sayısı kadar öğeli bir demet**. Girdi sayısı `f_ileri`'nin konumlu parametre sayısıdır: demet tipli tek bir parametre tek girdidir. Fonksiyon ifadesi verildiğinde öğe sayısının uyuşmazlığı denetim hatasıdır (tipler çalışmadan önce bilinir); değişkenle gelen fonksiyonda çalışma hatasıdır.
- `g` `f_geri`'nindir: ona yerinde yazmak başka bir gradyanı değiştirmez.
- Adlı argümanlar (sayılar, metinler) `f_ileri`'ye olduğu gibi geçer ve gradyan almaz. `TR_mixed_precision` bloğunda çağrılan `f_ileri` bloğun türüyle hesaplar. Tensör olmayan girdinin öğesi `TR_none`'dır; türev izlemeyen tensör girdisine verilen gradyan yok sayılır. Yayın yapılmaz.
- `f_geri` `TR_backward`'ı çağıran görevle, ama `f_ileri`'nin çağrıldığı andaki `TR_mixed_precision` türüyle çalışır (yerleşiklerin geri yönü gibi, 15.4). Gradyanlar girdilerin gradyan türünde birikir (16 bitlik yaprakta float32).
- *Test:* TR testi.

**15.30** Çok çıktı: `f_ileri`'nin çıktısı bir tensör demetiyse çağrı aynı sırada bir demet döndürür ve `f_geri`'nin `g`'si her çıktı için bir gradyan taşıyan demettir. Kayba bağlı olmayan çıktının gradyanı çıktıyla aynı şekilde **sıfır tensördür**; `f_geri` denetim yapmadan aritmetik kullanabilir. Çıktılar tek bir düğüme bağlıdır: `f_geri` bütün çıktıların gradyanları toplandıktan sonra bir kez çalışır; aynı çizgeden ikinci geri yayılım, başka bir çıktı üzerinden de, hatadır (15.27).
- *Test:* TR testi.

**15.31** Özel türevin hataları `TR_try`/`TR_catch` ile yakalanır ve iletileri yanlışı ve doğru biçimi söyler:
- `f_ileri` iki öğeli demet döndürmezse: fonksiyon ifadesi verildiğinde denetim hatası, değişkenle gelende çağrı satırında çalışma hatası (15.29 gibi);
- `f_geri`'nin demeti girdi sayısından farklıysa, tensör olmayan girdiye gradyan verirse ya da gradyanın şekli girdininkinden farklıysa `TR_backward` satırında (ileti iki şekli söyler);
- saklanan bir tensör ileri hesaptan sonra yerinde değiştiyse geri yayılım 15.27'deki gibi sürüm hatası verir (15.3);
- `f_geri`'nin içindeki hata kendi satırıyla gelir;
- `TR_forward:`/`TR_backward:` eksikse ya da fonksiyon değilse, `f_geri` iki parametre almıyorsa denetim hatasıdır; bilinmeyen adlı argümanda en yakın ad önerilir.
- *Test:* TR testi.

**15.32** `TR_grad_check(f, girdiler, TR_fd_step: 0.001, TR_tol: 0.01)` `f`'nin geri yayılımla bulunan gradyanını merkezi farkla bulunan sayısal gradyanla karşılaştırır ve **değer döndürmez** (13.10): bir doğrulamadır, uyuşmazsa hata fırlatır (12.3 gibi).
- `girdiler` `f`'nin konumlu argümanlarıdır (tek girdide kendisi, birden çokta demet; sayı `f`'nin konumlu parametre sayısıdır); yalnız türev izleyen tensör girdileri denetlenir: 16 bitlik girdiler float32 kopyalarıyla, float32 ve float64 girdiler kendi hassasiyetindeki kopyalarıyla (float64 desteklemeyen cihazda kopya CPU'dadır); programın tensörleri ve gradyanları değişmez.
- Fark adımı float32'de etkisizse (girdiye eklenince değer değişmiyorsa) hata verilir; ileti daha büyük `TR_fd_step:`'i önerir. `f` her girdi öğesi için iki kez çalıştığından küçük girdilerle kullanılır.
- Çıktı tek tensör ya da tensör demeti olabilir. Skaler olmayan çıktı sabit tohumlu rastgele bir yönle iç çarpımla skalere indirgenir; kayıp ve farklar float64'te hesaplanır.
- Her öğede `|analitik − sayısal| ≤ TR_tol · TR_max(1, |analitik|, |sayısal|)` aranır; sağlanmazsa ileti en büyük farkın girdisini, öğesini ve iki değeri söyler.
- Hiçbir girdi türev izlemiyorsa, adım ya da tolerans pozitif değilse, türevsiz kipte çağrılırsa ya da adım bir öğede float32'de etkisizse (`x + TR_fd_step == x − TR_fd_step`) hatadır.
- *Test:* TR testi.

## Eğitim

**15.33** `TR_parameters(m)` bir yapı değerini (13.10'un "yapı" kısıtı), yapıların listesini ya da tensör listesini iç içe yapılar, listeler ve sözlüklerle gezip **parametreleri** bir liste (`TR_List[TR_Tensor]`) olarak verir: iyileştiriciye verilen budur. **Parametre**, türev izlemek için kurulmuş ya da açılmış (`TR_requires_grad: TR_true`, `.TR_requires_grad = TR_true`, 15.39'un taşıması) yapraktır; dondurmak onu parametre olmaktan çıkarmaz (15.34), hiç türev izlememiş tensör (tablo, tampon) parametre değildir. `TR_named_parameters(m)` (yalnız yapıda) **bütün** tensör yapraklarını, dondurulmuş olanlar da (15.34), yol adıyla bir sözlük (`TR_Dict[TR_String, TR_AnyTensor]`, çünkü tam sayı tamponları da girer) olarak verir: kaydetmek ve yüklemek (`TR_save_tensors`, 20.11) budur; donmuş gövde de kaydedilir ve yüklenir. İkisi ayrı adlardır (13.1); `TR_named:` argümanı yoktur.
- Sıra sabittir: yapı alanları tanım sırasıyla, listeler sırayla, sözlükler ekleme sırasıyla. Aynı tensör birden çok yerde geçerse ilk yerinde bir kez yer alır; tensör olmayan değerler atlanır; döngülü yapı bir kez gezilir.
- `TR_parameters` parametre olmayan yaprakları (tablolar, tamponlar, nicemlenmişler) atlar, donmuş parametreleri verir; `TR_named_parameters` bütün yaprakları verir (türev izlemeyenler, donmuşlar, tam sayı tamponları ve nicemlenmişler dahil).
- **Türev izleyen bir ara sonuç** (yaprak olmayan, 15.6d) bir alandaysa ikisi de çalışma hatası verir: ileti alanın yolunu söyler ve ilklemeyi `.TR_requires_grad`'dan önce yapmayı önerir. Sessizce atlansaydı o ağırlık hiç eğitilmez ve kaydedilmezdi.
- `TR_named_parameters`'ın adları `.` ile birleşir, liste indeksi sayı, sözlük anahtarı kendisidir (`bloklar.0.dikkat.Wq`). Adlar programın alan adlarıdır, çevrilmez (17.1). İki tensör aynı ada düşerse hatadır. Sözlük doğrudan `TR_save_tensors`'a verilir (15.46).
- *Test:* TR testi.

**15.34** İyileştiriciler: `TR_AdamW(ps, TR_lr:, TR_weight_decay:, TR_beta1:, TR_beta2:, TR_eps:)` ve `TR_SGD(ps, TR_lr:, TR_momentum:, TR_weight_decay:)` (`ps` parametrelerin listesi); ikisinin de tipi `TR_Optimizer`'dir.
- Parametreler 15.33'ün parametreleridir; donmuş olanlar adımda atlanır. Her parametrenin bir "gradyan eklendi" bilgisi vardır: geri yayılım gradyan ekleyince kurulur, `.TR_step()` ve geri dönüş (20.14) onu siler; `.TR_zero_grad()`'ın sıfırladığı gradyan "eklenmiş" sayılmaz. `.TR_step()` bu bilgisi kurulmamış parametreyi atlar: ağırlığı, momentleri, adım sayacı ve ağırlık çürümesi değişmez. Bilgi parametrenindir: aynı geri yayılımın gradyanını iki iyileştirici adımlayamaz; bir parametre iki iyileştiricideyse ikincinin `.TR_step()`'i o parametrede çalışma hatası verir (sessizce atlamak yerine), ileti parametreyi tek iyileştiriciye vermeyi söyler. `.TR_step()` türevsiz kipte günceller; `.TR_zero_grad()` gradyanları sıfırlar (`.TR_grad`'ın canlı takma adı korunur, 15.27). İkisi de değer döndürmez (13.10).
- Sayısal ayarlar sonlu ve eksi olmayan sayılardır; NaN ve sonsuz hatadır. AdamW'nin `TR_eps`'i pozitiftir (float32'de 0'a düşen de hata). Sapma düzeltmesi her parametrenin kendi güncelleme sayısını kullanır (gradyan almadığı adımlar sayılmaz).
- **Son `.TR_step()`'ten bu yana hiçbir parametresine geri yayılım gradyan eklememişse `.TR_step()` çalışma hatasıdır:** iyileştirici kurulduktan sonra taşınan ya da değiştirilen yaprağı tutan iyileştirici (15.39) modeli sessizce eğitmezdi; ileti bunu söyler. `.TR_zero_grad()`'ın sıfırladığı gradyan "eklenmiş" sayılmaz (canlı takma ad hep vardır).
- `TR_master_weights:` varsayılanı `TR_true`'dur: 16 bitlik parametrelerin float32 ana kopyası tutulur, güncelleme orada yapılır, parametreye bir kez yuvarlanır. `TR_false`'ta güncelleme doğrudan 16 bitlik parametreye yuvarlanır. float32 parametrede etkisizdir. İyileştirici bir parametrenin deposu son adımından bu yana başka bir yazımla değiştiyse (sürüm, 15.3: `TR_load_parameters`, 20.11; kullanıcının yerinde yazması), 16 bitlik parametrenin ana kopyasını adımdan önce parametreden yeniler. Bir parametrenin momentleri ve sayacı ilk adımında kurulur: donmuş parametre bellek tutmaz.
- İyileştiricinin durumu (momentler, ana kopya) parametrenin cihazındadır. `TR_master_weights:` (`TR_Bool`) ikisinde de vardır. İyileştiricinin metni kuruluşudur: "AdamW(parametre: 2, lr: 0.001, ...)".
- **Parametre grupları** (katmana göre oran, sapmaya ağırlık çürümesi uygulamamak) birden çok iyileştiriciyle yazılır: her grup kendi `TR_AdamW`'si.
- **Kendi iyileştiricisi** türevsiz kipte, bileşik atamayla yerinde günceller (15.6d'nin birinci kuralı); sayaçları da kontrol noktasına girer (20.2, düz alanlar). `.TR_grad` `TR_Tensor?`'dir ve alan daraltılmaz (3.5), bu yüzden yerel bir kopya sınanır: `TR_no_grad { TR_var g = p.TR_grad; TR_if g != TR_none { p -= oran * g; g.TR_fill(0.0) } }`. Her şekilde, 0 boyutta da çalışır. Kitaplığın Muon ve NorMuon'u (`tr/optimizers`, 11.6) böyle yazılır.
  - **16 bitlik parametrede** doğrudan `p -= ...` ana ağırlık olmadan çalışır: küçük güncellemeler bf16'nın bir ulp'inin altında kalıp sessizce yutulur. Türevsiz kipte 16 bitlik bir parametreye (15.33) yapılan bileşik atama bu yüzden bir uyarı yazar (aynı yerde bir kez): ileti ana kopyayı (float32 bir tensör, güncelleme orada, sonra `p.TR_fill(ana)`) tutmayı söyler.
- **İnce ayarda dondurma:** yaprağın `.TR_requires_grad`'ı (`TR_requires_grad:` argümanıyla aynı ad ve anlam) okunur ve atanabilir: `TR_for p TR_in TR_parameters(model.gövde) { p.TR_requires_grad = TR_false }`.
  - Atama yalnız yaprakta geçerlidir (ara sonuçta hata). `TR_false` gradyanı da siler (`.TR_grad` `TR_none` olur); daha önce alınmış bir `.TR_grad` değeri parametreden kopar, kendi deposunu tutar. `TR_true` yalnız kendi deposuna sahip, görünüm olmayan, yazılabilir, nicemlenmemiş bir `TR_Float` yaprakta geçerlidir (15.52); görünümde, `.TR_detach()`'in sonucunda ve kopya `.TR_reshape`'in sonucunda çalışma hatasıdır, ileti `TR_Tensor(...)` ile bağımsız kopyayı önerir.
  - Dondurulmuş yaprak parametre olarak kalır: `TR_parameters`'ın ve `TR_named_parameters`'ın listesindedir (kaydedilir, yüklenir), iyileştirici onu atlar. Dondurmak gradyanı sildiği için donmuş parametre hiç güncellenmez.
  - İyileştirici türev izlemeyen yaprağı (donmuş, tam sayı, nicemlenmiş) kabul eder ve adımda atlar. Gradyansız adım kuralı aynı bilgiden çıkar: türev izleyen en az bir parametresi varken hiçbirinde bilgi kurulmamışsa hata verir; bütün parametreleri donmuşsa `.TR_step()` bir şey yapmaz (aşamalı çözmede ayrı oranlı gövde iyileştiricisi böyle kurulur); yaprak olmayan tensör kurulumda hatadır.
  - **Aşamalı çözme** kendiliğinden çalışır: `TR_AdamW(TR_parameters(m))` donmuş gövdeyi de tutar; `TR_for p TR_in TR_parameters(m.gövde) { p.TR_requires_grad = TR_true }` onu çözer ve sonraki adımda güncellenir.
  - `TR_load_tensors`'ın varsayılanıyla okunan tensör türev izlemez, parametre değildir: modeli doğrudan bu sözlükten kuran `TR_requires_grad: TR_true` ile okur (olağan yol `TR_load_parameters`'tır, 20.11; o var olan parametrelere yazar).
  - Donma durumu kontrol noktasına girer (20.4): sürdürülen eğitim aynı parametreler donmuş olarak başlar.
- *Test:* TR testi.

**15.35** `op.TR_lr` iyileştiricinin şimdiki öğrenme oranıdır (`TR_Float`) ve atanabilir: `op.TR_lr = 0.0005`, `op.TR_lr *= 0.5`.
- Yeni oran **sonraki** `.TR_step()`'ten başlayarak kullanılır; momentler ve adım sayaçları değişmez. Isınma, sabit bölüm ve düşürme planları her adımda oranı atayarak yazılır.
- Kurulumdaki `TR_lr: x` ile sonradan atanan aynı oran aynı adımı verir, CPU'da da GPU'da da (çok tensörlü tek başlatmalı yolda da). Oran 32 bit saklanır ve en kısa gösterimiyle okunur (`0.003`).
- Atanan değer sonlu ve eksi olmayan bir sayıdır (0 geçerlidir); eksi, NaN, 32 bite sığmayan ya da 32 bitte 0'a yuvarlanan sıfır olmayan değer hatadır ve oran değişmez.
- Oran bir ayardır, `op.TR_state`'e (20.12) girmez; kontrol noktası onu ayrıca kaydeder ve geri yükler (20.5, 20.6). Adım sayısından plan kuran program her adımda yeniden atar. İyileştiricinin öbür üyelerine atanamaz.
- *Test:* TR testi.

**15.36** Eğitimin kararlılığı. Uzun eğitim bir NaN ya da sonsuz değerle sessizce bozulmamalıdır; dil denetlemenin araçlarını verir, ne yapılacağını (adımı atlamak, durmak, geri dönmek) eğitim döngüsü seçer.
- `TR_is_finite(t)` öğe öğedir ve `TR_Tensor[TR_Bool]` verir; türev izlemez; `TR_fuse` içinde kullanılır.
- `TR_clip_grad(parametreler, TR_max: 1.0, TR_error_if_nonfinite: TR_false)` gradyanların toplam L2 normu `TR_max`'ı aşıyorsa hepsini aynı oranda küçültür ve kırpmadan önceki normu 0 boyutlu bir tensör olarak verir (gradyanlar GPU'daysa norm da orada). `.TR_grad`'ı `TR_none` olan parametre atlanır; hiçbirinin gradyanı yoksa norm 0'dır (varsayılan cihazda). `TR_max:` pozitif ve sonlu olmalıdır, değilse hata.
  - `TR_error_if_nonfinite: TR_true` normu kırpmadan önce okur (GPU'yu bekler) ve sonlu değilse gradyanlara ve sürümlerine dokunmadan hata verir. Varsayılanda bekleme ve denetim yoktur.
  - Sonlu olmayan normda kırpma: NaN normda gradyanlar değişmez; sonsuz normda ölçek 0'dır, sonlu öğeler 0, sonsuz öğeler NaN olur.
  - Norm float32'de taşarsa (`[3e38, 3e38]`) sonsuzdur: ölçek 0'dır, sonlu gradyanlar 0 olur. Sonlu gradyanların sessizce sıfırlandığı tek durum budur; `TR_error_if_nonfinite: TR_true` onu yakalar.
  - Kırpma gradyanları yerinde yazar ve sürümünü artırır (15.3), ölçek 1'de de. CPU ve GPU normu float32'ye yuvarlayıp böler: ölçek iki cihazda aynıdır. Kırpmadan önce gradyanı saklamış bir hesabın geri yayılımı iki cihazda da aynı sürüm hatasını verir.
- Adımı atlamak isteyen döngü kaybı ve normu kendisi denetler; atlanan adımda iyileştirici çalışmaz, gradyanlar sıfırlanır. Gradyan normu yalnız gradyanı sınar: türevi değerden bağımsız bir yol (`TR_sum`, `TR_mean`, toplama) kaybın NaN'ını gradyana taşımaz. Kalıp `&` iledir, `TR_and` değil (tensörde `TR_and` denetim hatası, 15.5):

```tr
# TR 4
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var L = TR_sum(w * w)
TR_backward(L)
TR_var norm = TR_clip_grad([w], TR_max: 1.0)
TR_if (TR_is_finite(L) & TR_is_finite(norm)).TR_item() { op.TR_step() }
op.TR_zero_grad()
```

- *Test:* TR testi; `TR_Bool` dönüşü uygulanınca TR testi.

## Cihaz, bellek ve hız

**15.37** Cihazlar: her tensör bir cihazdadır. Cihazın adı bir metindir ve **tek yazımlıdır**: `"cpu"`, `"gpu:0"`, `"gpu:1"`, ...; `"gpu"` ilk GPU'nun kısa adıdır ve `"gpu:0"` ile aynı cihazdır, `.TR_device` her zaman uzun adı verir.
- Büyük harf ve boşluk kabul edilmez; başka ad (`"tpu"`, `"cuda"`) hatadır, ileti geçerli adları söyler.
- `TR_devices()` bu makinedeki cihazları verir: ilki her zaman `"cpu"`, desteklenen GPU ve sürücüsü varsa ardından GPU'lar. Sürücü ilk soruda yüklenir; sürücüsüz makinede liste `["cpu"]`'dir ve her şey CPU'da çalışır.
- Adlar donanım üreticisine bağlı değildir: başka hızlandırıcılar (Apple, gömülü NPU) da aynı adları alır.
- *Test:* TR testi; tek yazım uygulanınca TR testi.

**15.38** `TR_default_device(c)` (`c` cihazın adı) bundan sonra kurulan tensörlerin cihazını seçer (`TR_Tensor`, `TR_zeros`, `TR_ones`, `TR_randn`, `TR_eye`, `TR_load_tensors` ve safetensors okuması (15.46, 15.47), veri kümesinin toplu işleri, 16); var olan tensörler yerinde kalır. Argümanlı biçim değer döndürmez (13.10); argümansız biçim şimdiki adı verir; program `"cpu"` ile başlar.
- Varsayılan cihaz programındır, göreve bağlı değildir: bir kez, model kurulmadan önce seçilir.
- Bir tensörle işleme giren sayı o tensörün cihazında kullanılır.
- Bu makinede olmayan cihaz hatadır; ileti makinedeki cihazları ve GPU sürücüsünün neden yüklenemediğini söyler, varsayılan değişmez.
- *Test:* TR testi.

**15.39** `.TR_to_device(c)` (`c` cihazın adı) tensör o cihazdaysa kendisini verir, değilse kopyalar.
- Türev izleyen bir **yaprak** taşınınca sonuç hedefte türev izleyen **yeni bir yapraktır**: gradyanı kendisinde birikir, kaynağa akmaz; iyileştiriciye ve `TR_parameters`'a yeni yaprak verilir. Parametre taşınınca sonuç da parametredir, donmuşsa donmuştur (15.33); yoksa donmuş gövde taşınınca çözme döngüsü onu bulmazdı.
- Ara sonucun kopyası türevlenebilirdir: gradyan kaynağın cihazına geri akar. Cins ve hassasiyet korunur.
- *Test:* TR testi.

**15.40** Farklı cihazlardaki iki tensörle işlem çalışma hatasıdır; ileti iki cihazı söyler ve `.TR_to_device(...)`'ı önerir. Sessizce kopyalanmaz.
- *Test:* TR testi.

**15.41** GPU'da hesap: tensör işlemlerinin ve gradyanlarının hepsi GPU'da çalışır. 15.22a'nın "aynı cihaz türü"nde (aynı GPU modeli; CPU'da aynı özellik kümesi, FMA dahil) bit bit aynıdır.
- **Cihaz türleri arasında** (CPU ile GPU, x86 ile ARM64, FMA'lı ile FMA'sız x86, iki GPU modeli) söz üç basamaklıdır:
  - **temel aritmetik** (`+ − * /`, `TR_sqrt`, karşılaştırma) IEEE 754'ün doğru yuvarlanmış işlemidir ve her cihazda **bit bit aynıdır**; tam sayı, mantık, sıralama ve arama da aynıdır (15.22a);
  - **matematik işlevleri** (15.9–15.11) her cihazda kendi ölçülmüş ulp sınırındadır; iki cihaz en çok iki sınırın toplamı kadar ayrılır. Sınırlar 15.10 ve 15.11'dedir;
  - **ondalık indirgemeler, `@`, dikkat ve normlar** toplama sırası değişebildiği için float32 yuvarlamasının birikmiş farkı kadar ayrılır; sayısal sınır söz verilmez, testler `TR_all_close`'un hassasiyete göre varsayılanlarıyla karşılaştırır (15.18a).
- GPU'da henüz olmayan işlem sessizce CPU'ya düşmez: hata verir, ileti işlemi söyler.
- İşlemler sıraya eklenip hemen döner; `.TR_tolist()`, `.TR_item()`, yazdırma ve 15.6'nın bekleyen işlemleri hesabın bitmesini bekler.
- GPU belleği işlem sırasında biterse yakalanabilir bir çalışma hatasıdır (15.42).
- *Test:* TR testi; Go: `TestGPUTrainingLanguageLevel`, `TestGPULibraryGPT`; cihazlar arası sınır uygulanınca Go testi.

**15.42** Bellek sınırı: tensör verisi ayıran her işlem sonucu ayırmadan önce denetler. Şeklin öğe sayısı taşma denetimli hesaplanır; bir tensör tek başına makinenin fiziksel belleğinden (okunamazsa 64 GB; GPU'da cihaz belleğinden) büyük olamaz.
- Sınır aşılınca ileti işlemi, şekli, istenen belleği ve var olan belleği söyler ve yayının beklenmedik bir boyut üretmiş olabileceğini anar; hata yakalanabilir, program çökmez. Birimler 1024 tabanlıdır.
- Sınır ayarlanamaz ve meşru büyük tensörleri kesmez; aynı anda yaşayan tensörlerin toplamı denetlenmez.
- Öğe sayısının elle seçilmiş bir sınırı yoktur; tek ölçü bellektir. Sıfır uzunluklu eksenli tensör veri tutmaz; şeklinin öğe sayısı sıfır eksenler 1 sayılarak da taşma denetimli hesaplanır (int64'e sığmayan şekil hatadır); şeklinden doğan dev bir sonuç (`TR_zeros([1000000000, 0]) @ TR_zeros([0, 1000000000])`) ayrılırken bu maddenin denetimine takılır.
- *Test:* TR testi.

**15.43** Tensör belleğinin ömrü: bir tensörün belleği (GPU'da cihaz belleği), ona artık hiçbir yoldan ulaşılamayacağı kesinleşince havuza döner. Çöp toplamayı beklemez; program bir şey yazmaz (kapatma, `dispose`, kapsam bloğu yoktur).
- İşlemin ürettiği tensör işlemi çalıştıran çağrının sahipliğindedir (en dış da bir çağrıdır). İfadedeki ara sonuç, onu kullanan işlem bitince bırakılır.
- Değişkene bağlı tensör, son değişkeni başka değer alınca ya da kapsamı bitince en geç o deyimin sonunda bırakılır. Argüman çağıranındır; dönen tensör çağırana geçer.
- Döngü gövdesinde tanımlanan değişken blok yerelidir (5.12), en dışta da.
- **Kaçış:** bir alana, sözlüğe, en dış ya da modül değişkenine, bir kapanışın gördüğü değişkene ya da onu saklayabilecek bir yerleşiğe (iyileştirici, veri kümesi) yazılan tensör belirlenimci bırakılmaz; çöp toplayıcı bırakır.
- Bir işlemin ürettiği ya da ifadede kurulan liste ve demet (`[a, b]`, `(a, b)`; `TR_topk`'un, özel türevin, `TR_next`'in demeti; 3.6a) içindeki tensörlerin sahibidir: o bırakılınca içindekiler de bırakılır, o kaçarsa içindekiler de kaçar; yalnız bir değişkene bağlı listeye sonradan (`.TR_append`, indeksle atama) yazılan tensör kaçar.
- Görünümler ve geri yayılımın sakladıkları depoyu ortak tutar; depo son sahibiyle ve onu saklayan geri hesap çalışınca döner. Geri yayılım her işlemin sakladıklarını o işlemin geri hesabı biter bitmez bırakır.
- Bırakılmış bir tensör kodda görünmez; uygulama hatası buna yol açarsa sessiz bozulma yerine iç hata verilir.
- *Test:* TR testi; Go: `TestOwnershipEngineListOwnsElements`, `TestOwnershipEscapesAreNeverReleased`, `TestOwnershipDeferAndAsyncCaptureEscape`, `TestSavedTensorReleasedAfterItsBackward`, `TestViewKeepsStorage`, `TestDroppedHeaderUseIsInternalError`.

**15.44** `TR_fuse(f)` öğe öğe tensör işlemlerinden oluşan `f`'yi tek geçişte hesaplayan bir fonksiyon döndürür (GPU'da her çıktı tek çekirdek): ara sonuçlar belleğe yazılmaz, geri yayılım ara değerleri saklamaz, girdilerden yeniden hesaplar.
- Argümanlar tensör ya da sayıdır; sayı argümanları önbellek anahtarına girer. Önbellek anahtarı `TR_compute_dtype()`'ı da içerir (15.4).
- **Kapanıştan okunan sayı ilk çağrıda kaydedilir ve her çağrıda, her cihazda denetlenir:** değişmişse çalışma hatasıdır, ileti onu argüman olarak vermeyi söyler. Denetim yalnız kapanıştan gelen değere uygulanır ve adın yolunu izler: değişken, alan, tam sayı liste indeksi ve metin sözlük anahtarı zinciri (`TR_self.oran`, `model.katmanlar[0].Wq`, `d["wq"]`); indeks ya da anahtar bir değişkense değeri yolun parçası olarak kaydedilir ve denetlenir; kapanıştan gelen ama yolu olmayan bir sayı (kapanıştaki bir çağrının sonucu) kayıtta hatadır, argüman olarak verilir. Argümandan türeyen sayılar (`x.TR_shape[0]`, `1 / TR_sqrt(dh)`) denetlenmez: argüman bileşimi önbellek anahtarındadır. Kapanıştan tensör okumak hatadır; tensör argüman olarak verilir.
- `f` ilk çağrıda (ve her yeni şekil, tür, türev isteği ve sabit bileşiminde) bir kez çalışır ve kaydedilir; yan etkileri (yazdırma, dış değişkene yazma) yalnız kayıtta olur.
- `f` şunları kullanabilir: öğe öğe tekli ve ikili işlemler (15.9–15.16a), `& | ^ ~`, karşılaştırmalar, sayıyla işlemler, `.TR_to_dtype`, yayın, dilim (`t[a..b]`, `.TR_slice`, `.TR_split`), `.TR_permute`, `.TR_transpose()`, `.TR_unsqueeze`, boyu 1 eksende `.TR_select(eksen, 0)` ve `TR_concat`. İndirgeme, `@`, genel `.TR_reshape`, veriyi isteyen işlem (`.TR_tolist()`, yazdırma, dolaşma) ve kaynaştırılamayan işlem hatadır; sessizce kaynaşmamış yola dönülmez.
- Sonuç `f`'nin döndürdüğü gibi bir tensör ya da tensör demetidir. Tensör ve sayı dışındaki argüman hatadır. Türev isteyen bir girdiye türevlenebilir yoldan bağlı olmayan çıktı türev izlemez.
- Karşılaştırmalar ve `TR_is_finite` türev izlemez.
- Sonuç kaynaşmamış hesapla bit bit aynıdır: 16 bitlik ve float32 işlemler float32'de, float64 işlemler float64'te, tam sayı ve mantık işlemleri kendi türünde hesaplanır ve kendi türüne yuvarlanır; tam sayı taşması 15.6'nın bekleyen işlemi gibi cihaz bayrağına yazar. Geri yön float32'dedir (float64 girdide float64) ve gradyana bir kez yuvarlanır. Türev izleyen bir değer (girdi ya da ara sonuç) yayınlanıyorsa ya da aynı türevli girdinin örtüşen görünümleri birleşiyorsa (gradyanı indirgeme ister) hatadır.
- *Test:* TR testi; Go: `TestFusedComparisonsMatchUnfusedBits`, `TestFusedMatchesUnfusedBits`, `TestFusedMatchesCPU`, `TestFuseCacheKeyIncludesComputeDType`; kapanıştaki sayının denetimi uygulanınca TR testi.

**15.45** `TR_replay(f)` GPU'da `f`'nin cihaz işini bir kez kaydeder ve sonraki çağrılarda tek seferde oynatır: yorumlayıcı `f`'yi yeniden çalıştırmaz, çekirdek başlatmalarının konak yükü kalkar (üretimin tek dizgecik adımı gibi küçük, sık işler).
- **Girdi bileşimi başına** (tensörlerin şekli, cinsi, hassasiyeti, cihazı, aynı tensörün kaçıncı argümanda tekrarlandığı; sayı argümanlarının değeri; `TR_compute_dtype()`) ilk çağrı `f`'yi doğrudan çalıştırır (ısınma), ikincisi kaydeder, sonrakiler oynatır; bileşim değişince yeniden kaydedilir.
- `f`'nin yerinde değiştirdiği argüman (`TR_write_slice`, indeksle atama) her oynatmadan sonra çağıranın tensörüne yazılır; aynı tensör iki argümanda verilirse kayıtta da tek tensördür. Argümanlar farklı cihazlardaysa, ayrı argümanlar aynı tensörün farklı görünümleriyse ya da cihazda zaten bir kayıt sürüyorsa (iç içe `TR_replay`, iç işin kaydı dıştakine girer) `f` doğrudan çağrılır; bir argüman kapanıştan okunan bir tensörle aynı depoyu paylaşıyorsa da.
- Çıktı her çağrıda kopyalanarak verilir. Sonuç doğrudan çağrıyla bit bit aynıdır (aynı çekirdekler, aynı sıra). İşlemlerin sayı işlenenleri cihazda doldurulur, kayıtta da çalışır.
- Oynatma aynı işi aynı bellekte yapar: `f`'nin içindeki konak hesabı (`TR_if`, döngü sayıları) kayıtta sabitlenir. **Kapanıştan okunan sayılar 15.44'teki gibi denetlenir;** kapanıştan okunan tensör (15.44'ün aksine) kullanılabilir, kayıt onun deposunu tutar ve depo da yoluyla denetlenir: aynı yol başka bir depoya bağlanmışsa çalışma hatasıdır, ileti yerinde güncellemeyi ya da tensörü argüman yapmayı söyler (yoksa kayıt eski bellekle oynatırdı). Her çağrıda değişmesi gereken değer tensör argümanıdır; kalıcı durum (KV önbelleği) kapanıştan verilir ve yerinde güncellenir (argüman olarak verilen tensör her oynatmada kopyalanıp geri yazılır).
- **Rastgele üreteç:** kayıtta paylaşılan üreteçten çeken işlemler (`TR_sample`, `TR_dropout`, tohumsuz `TR_randn`) cihazın sayaçlı üretecini kullanır; her oynatma üreteci kayıtta çektiği kadar ilerletir. Oynatma doğrudan çağrıyla aynı sayıları verir; 20.17'nin kaydı oynatmadan sonra da doğrudur.
- Yalnız türevsiz kipte çağrılır. Kayıt sırasında cihazdan konağa okuma (boş tensörde de), konaktan yeni veri ve eşzamanlama hatadır; kayıt bırakılır, sonraki çağrılar yeniden kaydetmeyi dener.
- **Hatalar kırpılmaz:** kayıtta `TR_embedding_lookup`'ın tensör indeksi, `TR_write_slice`'ın tensör başlangıcı ve 15.6'nın bekleyen işlemleri cihaz içi hata bayrağına yazar; bayrak ilk hatayı tutar, sınır dışı erişim belleğe dokunmadan bayrağı yazar; bayrak oynatmanın sonunda bir kez okunur ve ileti kaydedilen satırı söyler. Hata verilen oynatmada yerinde değiştirilen argümanların ve kapanış tensörlerinin (KV önbelleği) içeriği tanımsızdır: hata o ana kadar yapılmış yazmaları geri almaz, program durumu yeniden kurar.
- Kayıt sırasında kilit başka göreve geçmez; kayıtta görev beklemek hatadır. Kayda giren tensörlerin belleğini kayıt tutar; `TR_replay`'in döndürdüğü fonksiyona ulaşılamayınca kayıt ve bellek bırakılır.
- Kaydı desteklemeyen cihazda (CPU) `f` doğrudan çağrılır.
- *Test:* TR testi; Go: `TestGraphReplayMatchesEager`, `TestReplayMatchesDirectCalls`, `TestReplayArgumentsKeepDirectCallMeaning`, `TestReplayNestedRunsInsideOuterRecording`, `TestReplayErrors`; bayrak ve kapanış denetimi uygulanınca Go testi.

## Dosyalar

**15.46** `TR_save_tensors(yol, sözlük)` adları metin, değerleri tensör olan bir sözlüğü yazar; `TR_load_tensors(yol, TR_requires_grad: TR_false)` aynı sırayla bir sözlük döndürür. `TR_requires_grad: TR_true` dosyadaki nicemlenmemiş `TR_Float` tensörlerde türev izlemeyi açar; `TR_Int`, `TR_Bool` ve nicemlenmiş tensörler türevsiz kalır.
- Yol 18.16'ya göre çözülür (yazma: çalışma klasörü; okuma: okuma sırası). Yazma atomiktir (18.13).
- **Biçim uzantıya göre seçilir:** `.safetensors` ile biten yol safetensors yazar (15.47a), öbürleri TR biçimini. TR biçimi sürüm başlıklıdır (`TRTENSOR`, sürüm 2) ve sonunda CRC-32 taşır: kontrol noktası (20) bozulmayı böyle görür. Her cinsin ve hassasiyetin kendi tür kodu vardır (1 = float32, 2 = bf16, 3 = fp16 korunur; öbürleri yeni kodlar); düzen bütün cinslerde aynıdır, bu yüzden sürüm 2'nin bugünkü dosyaları okunmaya devam eder.
- **Her cins ve hassasiyet saklanır:** float32, bf16, fp16, float64; int64, int32, int16, int8, uint8 ve öbürleri; mantık. Nicemlenmiş saklama hassasiyetleri de (15.55). Yüklenen tensör kaydedildiği cins ve hassasiyettedir.
- **Tip:** bir dosya birden çok cins taşır (model ve adım sayacı, 20.12; Hugging Face'in I64 tamponları). `TR_load_tensors` `TR_Dict[TR_String, TR_AnyTensor]` verir: `TR_AnyTensor` cinsi çalışırken bilinen tensördür (15.1). Üstünde yalnız okuma işleri yapılır (15.1: `.TR_shape`, `.TR_dtype`, `.TR_device`, yazdırma, `TR_save_tensors`); görünüm, yazma ve hesap için `.TR_narrow[E]()` ile daraltılır (15.1; kopyasız): `d["w"].TR_narrow[TR_Float]()`, `d["step"].TR_narrow[TR_Int]()`. Cins uymazsa çalışma hatasıdır, ileti dosyadaki cinsi söyler. `TR_save_tensors` her cinsten tensör sözlüğünü alır (13.10'un kabul kümesi; kaplar değişmezdir).
- Dosya güvenilmeyen girdidir: bildirilen veri dosyada olmalı (eksik veri ayırmadan önce hata), bir tensör en çok 32 eksen bildirebilir; öğe sayısı taşma denetimli hesaplanır (sıfır eksenler 1 sayılarak da, 15.42), verisi dosyada olmalıdır ve ayırma 15.42'nin bellek denetiminden geçer. Eski sürüm, bilinmeyen tür, eksik, bozuk ya da başka türden dosya açık bir hatayla reddedilir.
- *Test:* TR testi; her cins uygulanınca TR testi.

**15.47** `TR_load_tensors(yol)` safetensors dosyasını da okur (Hugging Face ve PyTorch ağırlıklarının biçimi). Okumada biçim uzantıdan değil içerikten anlaşılır: `TRTENSOR` ile başlayan dosya TR biçimidir, 9. baytı `{` olan safetensors'tur; ikisi de değilse ileti iki biçimi de anar.
- Desteklenen türler 15.1'in bütün cinsleri ve hassasiyetleridir (F32, F64, BF16, F16, I64, I32, I16, I8, U8, BOOL ...); `F8_E4M3` ölçeği 1 olan "fp8" olarak okunur (15.55); karşılığı olmayan tür (`F8_E5M2`, U64) açık hatadır.
- Dosya 8 baytlık başlık boyuyla, ardından JSON başlıkla başlar. Tensörler dosyadaki veri sırasıyla gelir; `__metadata__` tensör değildir.
- Okuma kuralları (dosya güvenilmeyen girdidir; denetimler veri okunmadan yapılır, dosya boyunu aşan ayırma olmaz): başlık en çok 100 000 000 bayt ve dosyadan kısa; başlık geçerli UTF-8 JSON, sonunda yalnız boşluk; eşi olmayan UTF-16 vekil kaçışı (`\ud800`) geçersiz UTF-8 sayılır; alanlar harfine duyarlı (`DTYPE` bilinmeyen alandır), `shape` ve `data_offsets` eksi olmayan tam sayı dizileri (`null`, `1e0`, `1.0`, `-0` geçersiz), `data_offsets` tam iki sayı; her nesnede yinelenen anahtar, bilinmeyen alan ve eksik alan hatadır; `__metadata__` yalnız metin tutar; veri aralıkları `[başlangıç, bitiş)` sıralanınca ilki 0'da başlar, her biri öncekinin bittiği yerde başlar (boşluk ve örtüşme yok), sonuncusu veri alanının sonunda biter ve her birinin boyu şeklin öğe sayısı çarpı tür boyudur; 15.46'nın sınırları geçerlidir; boş ve 0 boyutlu tensör geçerlidir.
- *Test:* TR testi; Go: `TestSafetensorsLoad`, `TestSafetensorsRejects`, `FuzzSafetensorsDecode`.

**15.47a** `.safetensors` yoluna `TR_save_tensors` safetensors yazar: Hugging Face, PyTorch ve öbür araçlar modeli doğrudan okur.
- Başlık safetensors belirtimine uyar ve Python'un `safetensors`'ının yaptığı gibi yazılır: başlık boşlukla 8 bayta tamamlanır, tensörler hizaya göre sıralanır (önce geniş türler). `__metadata__`'ya `{"format": "pt"}` yazılır.
- CRC yoktur (biçimde yeri yok); kontrol noktası TR biçimini kullanır (20).
- Yazılan dosya 15.47'nin okuyucusundan ve Python'un `safetensors`'ından geçer: gidiş dönüşte her tensörün adı, cinsi, şekli ve verisi bit bit aynıdır; sıra yazılan sıradır, sözlüğün sırası değil.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi (Python `safetensors` ile gidiş dönüş).

## Hatalar ve özel değerler

**15.48** Şekil ve eksen hatası bir çalışma hatasıdır, `TR_try`/`TR_catch` ile yakalanır (8). İleti şekilleri ve ekseni söyler: "[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512". Hassasiyet hatası (cihazın desteklemediği hassasiyet, dosyadaki uymayan tür, cins dönüşümünde NaN) ve bellek sınırı hatası (15.42) da böyledir. Cins uyuşmazlığı ise tiptedir, denetim hatasıdır (15.49).
- *Test:* TR testi.

**15.49** Sessiz kırpma ya da sessiz tür değişimi yoktur; belirsiz durum hatadır ya da uyarıdır. Ayrıntılar kendi maddelerindedir.

| Kural | TR | Madde |
| :-- | :-- | :-- |
| Yayın | Şekiller sağdan hizalanır, her eksen eşit ya da 1 olmalıdır; uymazsa ekseni söyleyen hata. Eksen sayıları farklıyken iki taraf birden genişlerse (`[5]` ile `[2, 1]` → `[2, 5]`) uyarı. | 15.6b |
| Nicemlenmiş tensör | Yalnız sayılı işlemlerde; başka işlem hata, açmak açık. | 15.53 |
| Tür yükseltme | Aynı hassasiyet o hassasiyette, farklı ondalık hassasiyetler float32'de (float64 varsa float64'te) sonuç verir; sayıyla işlem tensörün hassasiyetini korur (`bf16 * 2.5` → bf16); hesap en az float32'dedir, sayı daraltılmaz (15.6a). | 15.6a, 15.6c |
| Daraltma | Sığmayan tam sayı hata ("uint8"e 300). | 15.2 |
| Kopya reshape | Kopya olan `.TR_reshape` sonucu yazmaya kapalı. | 15.3 |
| Cinsler | Karışmaz: `TR_Tensor[TR_Int] + TR_Tensor[TR_Float]` ve `TR_Tensor[TR_Int] * 2.5` denetim hatasıdır, ileti cins dönüşümünü önerir. | 15.3, 15.6, 15.6a |
| Tam sayı taşması | Çalışma hatası; kasıtlı sarma `TR_wrapping_*` ile. | 15.3 |
| Karşılaştırma | `TR_Tensor[TR_Bool]` verir, türev izlemez. | 15.5 |
| Tensör indeksi | `t[i]` sınır dışında hata, negatif indeks sondan sayar, indeks `TR_Int` ya da `TR_Tensor[TR_Int]`. | 15.6d |
| Tensör dilimi | `t[a..b]` sınır dışı uçları kırpar (3.3'ün dilimi gibi). | 15.6d |
| `.TR_slice` | Sınır dışında hata. | 15.3 |
| Sayı olmayan işlenen | `t + "a"` ve `t + TR_true` hata. | 15.6a |
| NaN | Karşılaştırma NaN'da `TR_false`; `TR_max`/`TR_min` NaN'ı yayar; bütün özel değerler 15.50'de. | 15.50 |

- *Test:* gerekmez: özet madde; her satırın testi kendi maddesindedir (15.6a–15.6d, 15.13, 15.18, 15.22b).

**15.50** Özel değerler (±0, ±sonsuz, NaN): her tensör işlemi bu değerlerde CPU'da ve GPU'da aynı sonucu verir: NaN'lar NaN, sonsuzlar aynı işaretli sonsuz, sıfırlar aynı işaretli sıfırdır; sonlu sonuçlar yaklaşık hesabın yuvarlaması içindedir. Türevlerde sıfırın işareti belirtilmez (gradyan biriktirilirken +0'a döner). Temel kural IEEE 754 aritmetiğidir; matematik işlevlerinin özel değerleri C99 Ek F'deki gibidir. Bunun dışında kalan ya da seçim gerektiren durumlar aşağıdaki tablodadır.

| Kural | TR |
| :-- | :-- |
| NaN yayılır | NaN girdili aritmetik işlem, indirgeme (toplam, ortalama, `TR_max`), softmax ve norm NaN verir. İstisnalar: karşılaştırma NaN'da `TR_false` verir (15.5), bu yüzden NaN `TR_where`'in koşuluna `TR_false` olarak girer (koşul mantık tensörüdür, 15.15); x⁰ = 1 ve 1ᵖ = 1; sıralamada NaN her sayıdan büyüktür (15.21, 15.22, 15.22b). |
| Tek işlevler sıfırın işaretini korur | f(±0) = ±0: `TR_sin`, `TR_tan`, `TR_tanh`, `TR_asin`, `TR_atan`, `TR_sinh`, `TR_expm1`, `TR_log1p`, `TR_sqrt`, `TR_silu`, `TR_gelu` (iki biçimi de). |
| Toplam | Yalnız −0'lardan oluşan toplam, ortalama ve birikimli toplam +0'dır; +∞ ile −∞'un toplamı NaN. |
| Üs (`x ** p`) | p tek tam sayıysa −0 ve −∞ işaretini korur ((−0)³ = −0, (−0)⁻¹ = −∞, (−∞)⁻¹ = −0); (−∞)^0,5 = +∞; (±1)^(±∞) = 1; (±0)^(−∞) = +∞; sonlu negatif taban ve tam sayı olmayan üs NaN (15.6e). |
| Doyma ve tanım dışı | `TR_tanh`(±∞) = ±1 (float32'de \|x\| > 9,02'de ±1), `TR_sigmoid`(+∞) = 1, `TR_sigmoid`(−∞) = 0, `TR_exp`(−∞) = 0, `TR_log`(±0) = −∞; `TR_log` ve `TR_sqrt` negatif sayıda NaN. |
| Limitler | `TR_silu` ve `TR_gelu` (iki biçimi de) −∞'da 0, +∞'da +∞; türevleri −∞'da 0, +∞'da 1. |
| Tamamen −∞ satır | `TR_softmax` 0, `TR_log_softmax` −∞, türevleri 0 (15.9; dikkatte de böyle, 15.24); +∞ ya da NaN içeren satır NaN'dır. |
| `TR_logsumexp` | Bütün öğeleri −∞ olan satır ve boş satır −∞, türevi 0; +∞ öğe varsa +∞, türevi o öğeye 1, öbürlerine 0; birden çok +∞'da türev ilk +∞'a 1, öbürlerine 0; NaN varsa (+∞ ile birlikte de) NaN (15.18). |
| `TR_sign` | `TR_sign`(−0) = −0, `TR_sign`(NaN) = NaN (15.12). |
| `TR_relu`, `TR_abs` | `TR_relu`(−0) = +0, `TR_relu`(NaN) = NaN; `TR_relu`'nun türevi x > 0'da 1, 0'da 0, NaN'da NaN. `TR_abs`'ın türevi ±0'da 0, NaN'da NaN (15.16). |
| `TR_max`, `TR_min` | NaN dışındaki sayılarda sıralama sırası (15.22b): −0 +0'dan küçüktür. Eşitlerde ilk öğe alınır, türev o öğeye gider (`TR_argmax` ile aynı öğe); NaN yayılır (15.18). |
| `TR_maximum`, `TR_minimum` | ±0'da sıralama sırası (15.22b): `TR_maximum(-0.0, 0.0)` argüman sırası ne olursa olsun +0'dır; NaN yayılır; türev seçilen tarafa, tam eşitlikte yarı yarıya, NaN olan konumda iki işlenene de NaN (15.16a). |
| `TR_clamp` | NaN korunur; sayı sınırda türevi sınırlar dahil aralıkta 1, dışında 0, NaN'da NaN; tensör sınırda `TR_maximum` ve `TR_minimum` gibi (15.13). |
| `TR_atan2` türevi | (0, 0)'da 0, bir girdi sonsuzken 0 (limit), NaN girdide NaN (15.14). |

Sınır: tablo özel değerler içindir. float32'nin ucundaki büyük sonlu sayılarda ara sonuç taşabilir (normlarda x², yani \|x\| ≳ 1,8e19; toplamın kendisi 3,4e38'i aşınca); bu durumda bir cihaz sonlu, öbürü sonsuz sonuç verebilir (15.22a). Trigonometrik işlevler GPU'da \|x\| ≤ 1e5'te doğrudur (15.11). 16 bitlik tensörler float32'de hesaplanıp kendi hassasiyetine yuvarlanır (15.6a); dönüşüm ±0, ±∞ ve NaN'ı korur.

İki ek:
- Sayıda `TR_sqrt` eksi girdide hata verir (13.1), tensörde NaN verir. Tensörün tek bir öğesi bütün işlemi durdurmaz; NaN, eğitimin kararlılık kuralıyla (15.36: kayıp ve gradyan normu) yakalanır.
- Tam sayı ve mantık tensöründe özel değer yoktur.
- *Test:* TR testi; Go: `TestSpecialValuesSameOnBothDevices`, `TestSpecialValuesTable`, `TestSumsKeepSpecialValues`.

## Nicemleme

**15.51** Nicemlenmiş saklama hassasiyetleri `TR_Float`'undur (15.1): her öğenin değeri ölçek × kod'dur (sıfır noktalı biçimde ölçek × (kod − sıfır)); hesap ondalıktır. Hassasiyetin adı düzeni söyler:

| Ad | Kod | Blok | Ölçek | Karşılığı |
| :-- | :-- | :-- | :-- | :-- |
| "q8" | 8 bit işaretli (−127..127), sıfırsız | son eksende 32 öğe | blok başına fp16 | GGUF `Q8_0`, bit bit aynı |
| "q4" | 4 bit, değer `d · (kod − 8)`, kod 0..15 | son eksende 32 öğe | blok başına fp16 | GGUF `Q4_0`, bit bit aynı |
| "q4z128" | 4 bit işaretsiz (0..15), grup başına sıfır | son eksende 128 öğe | grup başına fp16 ölçek ve 1 baytlık sıfır (0..16) | GPTQ ve AWQ (grup 128, `desc_act` yok) |
| "fp8" | E4M3 (8 bit ondalık) | 128 × 128 (son iki eksen) | blok başına float32 | Hugging Face'in FP8 modelleri (`weight_block_size: [128, 128]`) |

- Adlar öbür cinslerin hassasiyetleriyle çakışmaz: "int8" `TR_Int`'indir, "q8" `TR_Float`'un (15.1).
- Son eksenin uzunluğu bloğun katı olmalıdır; değilse nicemleme çalışma hatasıdır, ileti eksenin uzunluğunu ve bloğu söyler. fp8'de son iki eksen 128'in katı olmayabilir: son blok kısmidir, ölçek `[⌈N/128⌉, ⌈K/128⌉]`.
- Grup boyu adın parçasıdır ("q4z128"): başka grup boyu başka bir hassasiyettir; tablodakiler dışında nicemlenmiş hassasiyet yoktur.
- `.TR_dtype` adı verir; `.TR_shape` mantıksal şekli verir (`[N, K]`), deponun bayt düzeni görünmez.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.52** Nicemlemek ve açmak `.TR_to_dtype` iledir (15.3):
- `t.TR_to_dtype("q8")` float32, bf16, fp16 ya da float64 tensörü nicemler.
  - **"q8" ve "q4" llama.cpp'nin `Q8_0` ve `Q4_0`'ıyla bit bit aynı kodları verir:** ölçek `d` float32'de hesaplanır, kod `x · (1 / d)` ile bulunur, `d` yalnız saklanırken fp16'ya yuvarlanır.
  - "q8"de `d` bloğun mutlak en büyüğü / 127, kod sıfırdan uzağa yuvarlanır.
  - "q4"te `d` bloğun işaretli en büyük öğesi / −8; en büyük öğe mutlak değeri kesin büyük olanla seçilir (başlangıç +0.0; kesin büyük olmayan öğe seçilmez, işaretiyle); kod, `x · (1 / d) + 8.5`'in 0 ile 15 arasına kırpılmış değerinin tam kısmıdır.
  - Bu iki yuvarlama 13'ün yarımı çifte kuralından bilerek ayrılır: aynı adlı biçim, ekosistemdeki karşılığıyla aynı bitleri vermelidir; TR'de nicemlenen model llama.cpp'ninkiyle aynıdır.
  - "q4z128"de `.TR_to_dtype` en yakına yuvarlamadır (RTN): ölçek ve sıfır grubun en küçüğü ile en büyüğünden çıkar: `lo = min(grup_min, 0)`, `hi = max(grup_max, 0)` (float32); `d = (hi − lo) / 15` (`hi == lo` ise `d = 1`, sıfır 0, kodlar 0); `sıfır = round(−lo / d)` (yarımı çifte, 0..15); `kod = clamp(round(x / d) + sıfır, 0, 15)`; `d` kod hesabından sonra fp16'ya yuvarlanır. GPTQ ve AWQ'nun kodları kendi algoritmalarından gelir; onlar `TR_quantized` ile kurulur, `.TR_to_dtype` GPTQ'yu yeniden üretmez.
  - "fp8"de ölçek bloğun mutlak en büyüğü / 448; E4M3'e dönüşüm doyurur (sonsuz ve 448'den büyük değer 448) ve yarımı çifte yuvarlar.
  - Sonuç belirlenimcidir: aynı girdi her cihazda aynı kodları verir.
- Bütün öğeleri 0 olan blokta (−0.0 dahil): "q8"de ölçek +0.0, kodlar 0; "q4"te ölçek −0.0 (fp16'da `0x8000`: `max` +0.0'dan başlar, `d = max / −8`), kodlar 8 (`0x88`). Böylece llama.cpp ile bit bit aynıdır. NaN ya da sonsuz öğe nicemlemeyi çalışma hatası yapar; ileti ilk böyle öğenin indeksini söyler (sessiz NaN ölçek yok). Bu denetim öğeye bağlıdır: 15.6'nın bekleyen işlemlerindendir.
- `q.TR_to_dtype("float32")` (ya da "float64", "bf16", "fp16") açar. Açmak `ölçek × kod`'un hesabıdır: float64'te kesindir, float32'de bir kez yuvarlanır (fp8'de de), bf16 ve fp16 hedefte ayrıca hedefe yuvarlanır.
- **Kendi katmanını nicemlemek:** TR'nin `x @ W` düzeninde `W` `[K, N]`'dir; bloklar `K` boyunca olsun diye `W.TR_transpose().TR_to_dtype("q8")` yazılır (`[N, K]` deposu), çarpım `x @ q.TR_transpose()`'tur.
- **Parçalardan kurmak:** `TR_quantized(kodlar, TR_dtype: "q4z128", TR_scale: ölçek, TR_zero_point: sıfır)` hazır bir modelin ayrı saklanan parçalarından (GPTQ'nun `qweight`, `scales`, `qzeros`'u; FP8'in ağırlığı ve `weight_scale_inv`'i) yeniden nicemlemeden nicemlenmiş bir tensör kurar: model yayımlandığı bitlerle çalışır. Parçaların şekilleri:
  - "q8", "q4", "q4z128": `kodlar` açık kodlardır, `[N, K]` bir `TR_Tensor[TR_Int]` ("q4"te 0..15, "q4z128"de 0..15, "q8"de −127..127); `TR_scale` `[N, K / blok]` ondalık; `TR_zero_point` yalnız "q4z128"de, `[N, K / 128]` tam sayı, **gerçek sıfır** (0..16; değer `(kod − sıfır) · ölçek`). AutoGPTQ'nun dosyası sıfırı bir eksiğiyle saklar (`zeros -= 1`), açarken `& 15` ve `+ 1` uygular; gerçek sıfırı 0 olan grup böylece 16 olur. Kitaplık bu dosyayı `+ 1` ile açar; sıfır 16'yı taşımak için bir bayttır ve sonuç yayımlanan bitlerle aynıdır. GPTQ ve AWQ'nun int32'ye paketli kodlarını açmak kitaplığın işidir (`>>` ve `&` ile).
  - "fp8": `kodlar` ölçeği 1 olan bir "fp8" tensördür (safetensors'un `F8_E4M3`'ü, 15.55), `TR_scale` `[⌈N/128⌉, ⌈K/128⌉]` float32. Hugging Face'in `weight_scale_inv`'i adına rağmen çarpılan ölçektir: `TR_quantized(d["weight"].TR_narrow[TR_Float](), TR_dtype: "fp8", TR_scale: d["weight_scale_inv"].TR_narrow[TR_Float]())`.
  - Aralık dışı kod ya da sıfır, NaN ya da sonsuz ölçek çalışma hatasıdır (paketli depoda aralık dışı kod komşu öğeyi bozardı); ileti ilk böyle öğenin indeksini söyler. Denetim öğeye bağlıdır: 15.6'nın bekleyen işlemlerindendir.
  - Şekiller uymazsa çalışma hatası, ileti beklenen şekilleri söyler. Sıra değiştiren gruplar (GPTQ'nun `g_idx`'i, `desc_act`) desteklenmez: ileti bunu söyler.
- Nicemlemeye geçiş türevlenmez: türev izleyen tensörü nicemlemek çalışma hatasıdır, ileti `.TR_detach()`'ı ya da türevsiz kipi önerir. Nicemlenmiş tensör hiçbir zaman türev izlemez (`TR_requires_grad: TR_true` hata).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.53** Nicemlenmiş tensörün girdiği işlemler **açıkça sayılıdır**:
- `x @ w.TR_transpose()` (`x @ w` yazana ileti doğru yazımı verir): sağ işlenen, `[N, K]` deposu nicemlenmiş bir ağırlığın `.TR_transpose()` görünümüdür; bloklar çarpımın indirgeme ekseni `K` boyuncadır. `x` float32, bf16 ya da fp16'dır; açma çekirdeğin içinde, blok blok yapılır, biriktirme float32'dedir, sonuç `x`'in hassasiyetindedir (`TR_mixed_precision` bloğunda blok türü, 15.4).
- `TR_embedding_lookup(tablo, i)`: tablo `[V, D]` nicemlenmiş olabilir (bloklar `D` boyunca); seçilen satırlar `TR_compute_dtype()`'a açılır: `TR_mixed_precision` bloğunda blok türü, dışında float32 (15.4); `@`'nin blok kuralıyla aynı.
- `TR_linear_cross_entropy(girdi, w.TR_transpose(), hedef)`: `@`'nin kuralıyla.
- `.TR_to_dtype`, `.TR_shape`, `.TR_dtype`, `.TR_device`, `.TR_to_device`, ilk eksende indeks ve dilim (satırlar; bloklar bölünmez; fp8'de ilk eksende indeks hata, dilimin iki ucu 128'in katı), `.TR_transpose()`, yazdırma (açılmış değerlerle), `TR_save_tensors`. Bu görünümler bitişik nicemlenmiş tensöre uygulanır (bloklar son eksende). `.TR_transpose()` görünümü yalnız `@`'nin ve `TR_linear_cross_entropy`'nin sağ işleneni olur ve okunur (`.TR_shape`, `.TR_dtype`, `.TR_device`); üstünde indeks, dilim, `TR_embedding_lookup` ve `TR_save_tensors` çalışma hatasıdır, ileti `.TR_transpose()`'u geri almayı söyler. Başka her görünüm (son eksende dilim, `.TR_reshape`, `.TR_permute`) hatadır.
- **Başka her işlem çalışma hatasıdır** (öğe öğe aritmetik, indirgeme, `TR_concat`, son eksende dilim, sol işlenen olarak `@`): ileti `.TR_to_dtype("float32")` ile açıkça açmayı önerir. Sessizce açılmaz: büyük bir ağırlığın her adımda gizlice float32'ye açılması belleği sekiz katına çıkarır ve hızı yok eder (15.49 "sessiz tür değişimi yoktur").
- **Türev:** `x @ w.TR_transpose()`'da gradyan `x`'e akar, nicemlenmiş ağırlığa akmaz (ağırlık donmuştur); geri yön `g @ w`'yi ikinci bir çekirdekle hesaplar. Nicemlenmiş tabanın üstüne eğitilen düşük sıralı bağdaştırıcı böyle yazılır: `x @ w.TR_transpose() + (x @ A) @ B`.
- **Belirlenimcilik:** 15.22a. Cihazlar arasında açılmış değerler kesin aynıdır; çarpımın toplamı 15.41'in sınırındadır.
- "fp8" ağırlıkla TR etkinleştirmeyi nicemlemeden, açılmış ağırlıkla çarpar. Hugging Face'in FP8 modelleri etkinleştirmeyi de çalışırken nicemler; sonuç onlarınkinden yuvarlama farkıyla ayrılır, özgün uygulamayla bit bit aynı değildir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi ve Go testi (CPU ve GPU, açılmış hesapla karşılaştırma).

**15.54** Nicemlenmiş tensöre yerinde yazılmaz: atama, bileşik atama, `.TR_fill` ve `TR_write_slice` çalışma hatasıdır (yazma bir bloğun ölçeğini değiştirirdi); ileti açıp yazmayı ve yeniden nicemlemeyi söyler. Kopyası `TR_Tensor(q)` aynı hassasiyette yeni bir depodur. Yükleme (20.6, 20.11, 20.14) yerinde yazma değildir: nicemlenmiş yaprağın deposuna, aynı belleğe, bütün olarak yazar (kodlar, ölçekler ve sıfırlar birlikte, aynı hassasiyet ve şekille; görünümler ve `TR_replay` kaydı geçerli kalır); hassasiyet ya da şekil farklıysa hatadır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**15.55** Dosyalar:
- TR biçimi (15.46) nicemlenmiş tensörü kendi hassasiyetinde, kodları ve ölçekleriyle saklar; yüklenen tensör aynı hassasiyettedir. Her nicemlenmiş hassasiyetin kendi tür kodu vardır; sürüm 2'dir (15.46).
- safetensors'ta nicemlenmiş tür yoktur: hazır modeller kodu, ölçeği ve sıfırı ayrı tam sayı ve ondalık tensörler olarak saklar (GPTQ'nun `qweight`, `qzeros`, `scales`'i). `TR_load_tensors` onları olduğu gibi okur (`TR_Int` ve `TR_Float` tensörler, `TR_AnyTensor`); kitaplık (`tr/pretrained`) onları `TR_quantized` ile birleştirir.
- safetensors'un `F8_E4M3` türü ölçeği 1 olan bir "fp8" tensör olarak okunur; ölçek ayrı tensörden `TR_quantized` ile verilir.
- `TR_save_tensors` `.safetensors`'a nicemlenmiş tensör yazmaz: çalışma hatası, ileti açmayı ya da TR biçimini önerir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.
