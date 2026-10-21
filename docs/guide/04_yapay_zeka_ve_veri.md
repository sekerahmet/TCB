# 4. Yerleşik Yapay Zeka ("İkisi Bir Arada") ve Veri Bilimi Komutları

## A. Düşük Seviye Matris ve Otomatik Türev
- `tensör([[1, 2], [3, 4]])`, `sıfırlar([satır, sütun])`, `birler([satır, sütun])`, `birim_matris(n)`, `rastgele_normal([satır, sütun], tohum: 42)`: matris 2 boyutlu bir **tensördür** (ayrı bir matris tipi ya da `matris(...)` / `rastgele_matris(...)` oluşturucusu yoktur; ayrıntılar §4.C)
- `A @ B` (Çok çekirdekli matris çarpımı), `A.devrik()`, `A.şekil` (`[satır, sütun]`)
- `tensör(3.0, türev_izle: doğru)` (tek sayı 0 boyutlu tensör verir), `geri_yayılım(y)`, `x.gradyan`, `x.değer` (`x.gradyan` da bir tensördür; sayısını `x.gradyan.değer` verir)
- Aktivasyonlar ve kayıp: `relu(x)`, `sigmoid(x)`, `tanh(x)`, `softmax(x)`, `çapraz_entropi(çıktı, hedef)`; ortalama kare hata `ortalama((tahmin - hedef) ** 2)` ile yazılır

## B. İstatistik ve Veri Biçimleri
- İstatistik: `ortalama(liste)`, `standart_sapma(liste)`, `ortanca(liste)` (vektör işleri tensörlerle yazılır: `toplam(a * b)`, `karekök(toplam(a * a))`)
- Veri biçimleri: `csv_oku(yol)`, `csv_çöz(metin)`, `json_çöz(metin)`, `json_metin(değer)`
- `csv_oku` / `csv_çöz`: `csv_oku(yol)` bir CSV dosyasını okur, `csv_çöz(metin)` CSV metninin kendisini ayrıştırır (`csv_çöz("ad,yaş\nAli,30")`). `csv_oku`'ya CSV metni verilirse hata `csv_çöz`'ü önerir. Göreli yollar önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür. Baştaki bayt sırası işareti (BOM) atılır. Hücreler boşlukları kırpılarak **yalnızca katı sayı biçimindeyse** sayıya çevrilir: `42`, `-7` `Tam`; `1.0`, `2.5e3`, `1E-3` `Ondalık`. Öteki her hücre olduğu gibi `Metin` kalır, böylece hiçbir değer sessizce bozulmaz: baştaki sıfırlı `007` (posta kodu, kimlik), `Tam` aralığını aşan `12345678901234567890`, `NaN`, `Inf`, `0x10`, `1_000`, `+5`, `.5`.
- `json_çöz` / `json_metin`: `json_çöz` sözlük anahtarlarının sırasını korur ve baştaki BOM'u atar; tam sayı değerli ondalıklar (`1.0`, `1e2`) `Tam` olur, `Tam` aralığını aşan tam sayılar (`12345678901234567890`) en yakın `Ondalık`'a yaklaşık çevrilir. `json_metin` anahtarları sözlükteki sırayla yazar (``json_metin(json_çöz(`{"b": 1, "a": 2}`))`` → `{"b":1,"a":2}`; JSON metni ham metinle `` `...` `` yazılınca `{` ve `"` kaçış istemez). JSON'da karşılığı olmayan değerler `dene/yakala` ile yakalanan Türkçe bir hata verir: Tanımsız ve sonsuz sayılar (`NaN sayısı JSON'a yazılamaz ...`), kendini içeren liste/sözlük (`değer kendini içeriyor ...`), 1000 seviyeden derin iç içe değerler ve `1` ile `"1"` gibi JSON'da aynı anahtara düşen iki anahtar.
## C. Tensörler ve Otomatik Türev
Tensör, n boyutlu bir sayı dizisidir (veri türü varsayılan olarak `float32`; `bf16` ve `fp16` de seçilebilir, bkz. **Veri türleri**); yapay zekâ modellerinin ağırlıkları, girdileri ve ara sonuçları tensördür. Her işlem hesap çizgesine kaydedilir; `geri_yayılım(kayıp)` tüm gradyanları hesaplar. Çalıştırılabilir örnekler: `examples/tr/23_tensor.tr`, `examples/tr/35_transformer.tr`, `examples/tr/36_karakter_modeli.tr`.

- **Oluşturma:** `tensör(veri, türev_izle: doğru)` (veri: iç içe liste, tek sayı ya da tensör), `sıfırlar([2, 3])`, `birler([2, 3])`, `rastgele_normal([2, 3], tohum: 42)`. Şekil **liste** olarak verilir; `sıfırlar(2, 3)` gibi iki tam sayı da aynı 2 boyutlu tensörü verir. Matris 2 boyutlu tensördür: `tensör([[1, 2], [3, 4]])`, `birim_matris(n)` ve `rastgele_normal([satır, sütun], tohum: 7)` ile oluşturulur. Kaldırılan `matris(...)` ve `rastgele_matris(...)` yazılırsa hata karşılığı önerir (`'matris' TR'de yok; 'tensör' yazmak mı istediniz?`). `türev_izle` varsayılanı her zaman (tek sayıda da) `yanlış`tır: `tensör(3.0)` 0 boyutlu (skaler) bir tensördür (`.şekil` `[]`, `.değer` `3.0`, `.gradyan` `yok`); türevi izlenecekse `tensör(3.0, türev_izle: doğru)` yazın.
- **Sayı hassasiyeti:** Tensör hesapları `float32` ile (yaklaşık 7 anlamlı basamak) yapılır; `Ondalık` ise 64 bitliktir. Bu yüzden `(tensör([1.0]) / 3).değer[0]` `0.33333334` verir ve `1 / 3`e eşit değildir; `tensör([16777217.0])` 16777216 olarak saklanır. Kesin karşılaştırma yerine `yuvarla(...)` ya da küçük bir tolerans kullanın.
- **Veri türleri:** Her tensörün bir türü vardır: `"float32"` (varsayılan), `"bf16"` (bfloat16: float32 ile aynı aralık, ~3 anlamlı basamak) ya da `"fp16"` (IEEE yarım hassasiyet: en büyük değer 65504, ~3 anlamlı basamak). 16 bitlik türler belleği yarıya indirir. Tür, oluşturmada `tür:` ile seçilir: `tensör(veri, tür: "bf16")`, `sıfırlar([2, 3], tür: "fp16")`, `birler([4], tür: "bf16")`, `rastgele_normal([2, 3], tohum: 1, tür: "bf16")`; değerler en yakın gösterilebilir değere yuvarlanır (`tensör([1.001], tür: "bf16").değer` `[1.0]`). Tensörden oluşturmada (`tensör(t)`) kaynağın türü korunur; türünü değiştirmek için `t.türe("bf16")` yazılır (`tensör(t, tür: ...)` hata verir). `.tür` türü metin olarak verir (`"bf16"`); `.türe("fp16")` dönüştürülmüş bir kopya verir ve **türevlenebilir**dir (gradyan kaynak tensöre kendi türünde akar; tür aynıysa tensörün kendisi döner). İşlemler içte en az float32 ile biriktirir ve sonucu bir kez yuvarlar; işlenenler aynı türdeyse sonuç o türde, farklıysa `float32`'dir; sayıyla işlemler (`b * 2`, `b / 2`, `2 / b`...) tensörün türünü korur. 16 bitlik parametrelerin (yaprak tensörlerin) gradyanları `float32` tutulur; ara sonuçlarınki kendi türündedir (`.türe` ve görünümler kaynağın gradyan türünü izler: float32 ağırlığın `bf16` kopyasının gradyanı `float32`'dir). Bilinmeyen tür adı yol gösteren bir hatadır: `tensör: bilinmeyen veri türü "bfloat16"; geçerli türler: "float32", "bf16", "fp16". Bunu mu demek istediniz: "bf16"?`
- **Karışık hassasiyet:** `karışık_hassasiyet("bf16") { ... }` bloğu (sondaki blok yazımıyla; yeni anahtar kelime değildir) içinde `@` matris çarpımları ve `dikkat` girdilerini `bf16`'ya çevirir ve float32 biriktirmeyle **bf16** sonuç verir (PyTorch'un autocast'iyle aynı kural); normalizasyon (`katman_normu`, `rms_normu`), `softmax`/`log_softmax` ve `toplam`/`ortalama` 16 bitlik girdiyi float32'ye açarak hesaplar, `çapraz_entropi` 16 bitlik logitleri float32'de işler ve float32 kayıp verir. Ağırlıkların kendisi değişmez (dönüşüm türevlenebilir bir kopyadır), bu yüzden `geri_yayılım` ve iyileştirici float32 ağırlıklarla çalışır. Blok son ifadesinin değerini döndürür: `değişken kayıp = karışık_hassasiyet("bf16") { çapraz_entropi(ileri(x), y) }`. `"fp16"` de verilebilir (65504'ü aşan değerler sonsuza taşar; kayıp ölçekleme yoktur); `"float32"` iç içe bir blokta karışık hassasiyeti kapatır. Bloklar iç içe kurulabilir; blok bitince (hata ile bitse de) önceki tür geri gelir. Durum **göreve bağlıdır**: bloğun içinde kilidi bırakan bir çağrı (`bekle(...)`, ağ, dosya) sırasında çalışan başka bir `eşzamanlı` görev bloktan etkilenmez; blok içinde başlatılan bir `eşzamanlı` görev de float32 ile başlar (gerekirse kendi bloğunu kurar). CPU'da bf16 hız kazandırmaz (dönüşüm maliyeti nedeniyle biraz yavaştır); amacı bellek/bant genişliği tasarrufu ve 16 bitlik donanıma hazırlıktır.
- **Eski matris özellikleri:** `.boyut` → `.şekil`, `.satır` / `.sütun` → `.şekil[0]` / `.şekil[1]`, `.T` → `.devrik()`, `.toplam` / `.ortalama` → `toplam(A)` / `ortalama(A)` (0 boyutlu tensör; sayısı `.değer`), `.liste` → `.değer`. Eski adlar kaldırıldı; kullanılırsa hata mesajı karşılığını önerir (`Bunu mu demek istediniz: 'şekil'?`).
- **Özellik ve metotlar:** `.şekil`, `.değer` (iç içe liste; skalerde tek sayı; elemanlar Ondalık, tam sayılı tensörlerde — `maks_indeks` sonucu, veri kümesinden gelen `girdi`/`hedef` — `Tam`; float32 değerler en kısa gösterimleriyle verilir, `tensör([0.1]).değer[0] == 0.1` doğrudur), `.gradyan` (yalnızca `türev_izle: doğru` ile oluşturulan yaprak tensörlerde; bir işlemin sonucu olan tensörde açıklayıcı hata verir), `.cihaz`, `.şekillendir([4, 6])` (bir eksen `-1` olabilir), `.devrik()`, `.eksenleri_sırala([0, 2, 1])`, `.dilim(eksen, baş, son)` (son hariç, negatifler sondan sayar), `.cihaza("cpu")` (GPU için bkz. aşağıda "GPU'da eğitim").
- **İşleçler:** `@` (matris çarpımı; 1 boyutlu işlenen vektör sayılır, 3+ boyutlu toplu çarpım), `+ - * /` eleman bazlı ve **yayınlamalıdır** (eksenler sağdan hizalanır; eşit ya da 1 olan eksenler uyumludur). Sayı iki yanda da durabilir: `2 * a`, `a - 1`, `1 - a`, `a / 2`, `1 / a`. Tekli eksi (`-a`) ve sayı üssü (`a ** 2`) de vardır; hepsi türev izler. İki işlenen de genişletilirse (ör. `[4, 1]` ile `[4]` → `[4, 4]`; çoğunlukla bir şekil hatası) standart hataya her işlem ve şekil çifti için bir kez (en çok 100 farklı uyarı; sonrası tek bir bildirimle susturulur) `Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); ...` yazılır; program durmaz. `%` tensörlerde tanımlı değildir.
- **Karşılaştırma ve maske:** `< > <= >=` bir sayıyla ya da **aynı şekilli** tensörle eleman bazlı karşılaştırır ve türev izlemeyen 0/1 maskesi verir: `a > 0`, `a < b`. `==` ve `!=` ise tensörleri bütün olarak karşılaştırıp tek bir `Mantık` verir: şekilleri ve tüm elemanları aynıysa eşittir (`tensör([1.0, 2.0]) == tensör([1.0, 2.0])` doğru, `[2]` ile `[1, 2]` şekilli tensörler eşit değildir). 0 boyutlu tensör bir sayıyla da karşılaştırılabilir: `tensör(7.0) == 7`, `A[1][0] == 3.0` ve `toplam(t) == 10` doğrudur; en az bir ekseni olan tensör (`tensör([7.0])`) sayıya eşit değildir.
- **İndeksleme:** `t[i]` ilk eksenin i. elemanını verir (bir eksen eksilir; negatif indeks sondan sayar), `t[baş..son]` ilk eksende dilim verir (`son` hariç); ikisi de veriyi kopyalamayan, türev izleyen görünümlerdir ve zincirlenebilir (`t[0][1]`); bu, `.devrik()` gibi bitişik olmayan görünümlerde de geçerlidir (`t.devrik()[0] = 9` `t`'nin ilk sütununu yazar). `.şekillendir` yalnızca bitişik olmayan bir tensörün eksenlerini depoda düzenli adımla ifade edilemeyecek biçimde birleştirirken (ör. `t.devrik().şekillendir([-1])`) kopya üretir. Sınır dışı indeks hatadır.
- **İndeksle atama:** `t[i] = d`, `t[i][j] = d`, `t[baş..son] = d` ve `t[i] += d` veriyi yerinde değiştirir. `d` bir sayıysa hedefin tüm elemanlarına yazılır (`t[0] = 0` ilk satırı sıfırlar); liste ya da tensörse şekli hedefin şekliyle aynı olmalıdır (`t[0] = [1, 2, 3]`). Türev izlenen bir tensöre (ör. bir ağırlığa) yalnızca `türevsiz { ... }` içinde atanabilir.
- **Döngü:** `döngü x içinde t`, `döngü i, x içinde t` ve `[ifade döngü x içinde t]` tensörü ilk ekseni boyunca gezer: 2 boyutlu tensörde `x` sıradaki satırdır (veriyi paylaşan 1 boyutlu tensör; `x[0] = 9` tensörü değiştirir), 1 boyutlu tensörde sıradaki sayıdır (`Ondalık`; tam sayılı tensörde `Tam`). Örnek: `[toplam(satır).değer döngü satır içinde A]` satır toplamlarını liste olarak verir. 0 boyutlu tensör üzerinde döngü kurulamaz.
- **Yazdırma:** `yazdır(t)` şekli başlıkta gösterir (`Tensör[2×3]`; float32 dışındaki türlerde tür de: `Tensör[2×3] bf16`, skalerde `Tensör(2.5000) fp16`), değerleri 4 basamakla yazar; 1000'den çok elemanda her eksenin yalnızca ilk ve son 3 elemanı gösterilir (`…`). Sıfır olmayan bir değer 0.0001'den küçükse ya da bir değer 1e8 ve üzerindeyse tüm hücreler bilimsel gösterimle (`1.0000e-05`) yazılır. Sayı olmayan değerler `Tanımsız`, `Sonsuz`, `-Sonsuz` olarak görünür.
- **Doğruluk değeri:** `eğer t { ... }` tek elemanlı tensörde eleman 0'dan farklıysa doğrudur, boş tensör yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; `eğer maks(t) > 0` gibi bir indirgeme kullanın (şimdilik hata vermez, doğru sayılır).
- **Matematik:** `relu`, `sigmoid`, `tanh`, `gelu` (varsayılan tanh yaklaşımı; `gelu(x, yaklaşık: yanlış)` tam erf tabanlı GELU), `silu`, `karekök`, `üstel`, `log`, `sinüs`, `kosinüs` (radyan; dönel konum gömmesi gibi açılı hesaplar için, türevli), `softmax(x, eksen: -1)`, `log_softmax(x, eksen: -1)`; genişletilmiş matematik aşağıdaki tablodadır. **Dönüş tipi kuralı:** tensör yalnızca tensör girdisinden çıkar (`relu(tensör([[1, -2]]))` ve `relu(birim_matris(2))` tensör verir). Eleman bazlı fonksiyonlar sayıda da çalışır (`relu(-2)` → `0.0`), listede çalışmaz: listeyi `tensör(...)` ile çevirin (`relu(tensör([1, -2]))`); `softmax` ve `log_softmax` yalnızca tensör alır (`softmax(tensör(...), eksen: -1)`). Boş tensörün `ortalama`sı ve bütün hedefleri yok sayılmış (hepsi negatif) bir `çapraz_entropi` sessizce 0 vermez, açık hata verir. Tamamen maskeli (hepsi `-Sonsuz`) bir satırın `softmax`ı 0, `log_softmax`ı `-Sonsuz` olur ve gradyan almaz; dolgu satırları kaybı NaN yapmaz. Sayı olmayan değer (`Tanımsız`, NaN) yayılır: `relu`, `maks` ve `min` onu yutmaz, `maks_indeks` ilk NaN'ın yerini verir; böylece kayıpta beliren NaN gizlenmez.

  | İş | TR | Not |
  | :-- | :-- | :-- |
  | SiLU etkinleştirme | `silu(x)` | `x · sigmoid(x)`, tek geçiş; SwiGLU: `silu(x @ Wg) * (x @ Wu)` |
  | Trigonometri | `tanjant`, `arksinüs`, `arkkosinüs`, `arktanjant`, `arktanjant2(y, x)` | radyan; `arktanjant2` dört bölgeli, yayınlamalı |
  | Hiperbolik | `sinh`, `cosh` (`tanh` zaten var) | |
  | Kararlı logaritma ve üstel | `log1p(x)`, `log2`, `log10`, `üstel_eksi_bir(x)` | küçük x'te `log(1 + x)` ve `üstel(x) - 1`'den doğru |
  | Yuvarlama | `taban`, `tavan`, `işaret` | Tam'da Tam; türev 0 |
  | Mutlak değer | `mutlak(x)` | Tam'da Tam; tensörde türevli (türev `işaret(x)`, 0'da 0); en büyük fark: `maks(mutlak(a - b))` |
  | Sınırlama | `sınırla(x, en_az: 0, en_fazla: 1)` | sınırlardan biri yeter; türev aralık içinde 1 |
  | Koşullu seçim | `koşullu(x > 0, x, 0)` | yayınlamalı; gradyan seçilen dala akar |
  | Birikimli toplam | `birikimli_toplam(x, eksen: -1)` | türevli, belirlenimci |
  | En büyük k | `en_büyük_k(x, 5)` → `[değerler, indeksler]` | eşitlikte küçük indeks önce |
  | Sıralama | `sıralama_indeksi(x)` | kararlı, küçükten büyüğe |
  | π | `pi` | sabit; `e` yerine `üstel(1)` |

  Karşılaştırmalar (`<`, `>`, `<=`, `>=`) tensörlerde yayınlamalıdır ve 0/1 maskesi verir; `==` ise iki tensörün tamamen aynı olup olmadığını söyler. Hepsi GPU'da da çalışır (dil tanımı **15.31**–**15.38**).
- **İndirgemeler:** `toplam(x)`, `ortalama(x)`, `maks(x)`, `min(x)` tüm elemanları skaler (0 boyutlu) tensöre indirger; `eksen: 1` verilirse o eksen kaldırılır; `boyutu_koru: doğru` ile 1 uzunlukta korunur (`[B, T, D]` → `toplam(x, eksen: -1, boyutu_koru: doğru)` → `[B, T, 1]`, böylece `x / toplam(x, eksen: -1, boyutu_koru: doğru)` yayınlanır; eksen verilmezse bütün eksenler 1 olur). `toplam(x, 1)` / `ortalama(x, 1)` konumlu ekseni de kabul eder; `maks`/`min` için eksen **yalnızca adlı** verilir (`maks(x, 0)` eleman bazlı sınırla karışmasın diye hatadır). `toplam([a, b])` / `ortalama([a, 1, b])` listedeki tensörleri yayınlamalı toplamla katlar (türev korunur); iç içe sayı listesi indirgenemez, önce `tensör(...)` ile çevirin. `eksen:` yalnızca tensörlerde geçerlidir.
- **Sinir ağı:** `gömme(tablo, indeksler)` (tablo `[V, D]`; sonuç `indeks şekli + [D]`), `çapraz_entropi(çıktı, hedef)` (çıktı `[..., C]`, hedef sınıf indeksleri; negatif hedefler yok sayılır), `katman_normu(x, ağırlık, sapma, epsilon: 0.00001)`, `rms_normu(x, ağırlık, epsilon: 0.00001)` (ağırlık/sapma `yok` olabilir).
- **Birleştirme ve bölme:** `birleştir([a, b, c], eksen: -1)` tensörleri bir eksen boyunca birleştirir (diğer eksenler eşit olmalı; türevli). `x.böl(3, eksen: -1)` tensörü o eksende 3 eşit parçaya böler ve parçaların listesini verir (görünümler, türevli): `değişken q, k, v = (x @ Wqkv).böl(3)`.
- **Dikkat:** `dikkat(q, k, v, nedensel: doğru, pencere: 0, maske: yok)` birleşik ölçekli nokta çarpımı dikkatidir; `softmax(q @ k.devrik() / √dh + maske) @ v` ile aynı sonucu verir ama `T×T` puan matrisini hiç tutmaz (puanlar bloklar hâlinde, çevrimiçi softmax ile hesaplanır; bellek T ile doğrusal) ve tek bir türev düğümüdür (geri yayılım puanları yeniden hesaplar). `q` `[..., H, T, dh]`, `k`/`v` `[..., Hkv, Tk, dh]` şeklindedir; `Hkv < H` gruplu sorgu dikkatidir (`H`, `Hkv`'nin katı; k/v kopyalanmaz). Ölçek `1/√dh`'dir. `nedensel: doğru` (varsayılan) gelecekteki konumları kapatır; `Tq < Tk` iken (önbellekli üretimde yalnızca yeni parça sorgulanırken) hizalama sondandır: sorgu `i`, `Tk - Tq + i`'ye kadar görür. `pencere: 128` her sorguyu son 128 konuma sınırlar (yerel dikkat katmanı; pencere dışındaki bloklar hiç hesaplanmaz). `maske:` isteğe bağlı 0/1 tensörüdür (1 = bakılır; `[Tq, Tk]` gibi yayınlanabilir şekiller kabul edilir). `karışık_hassasiyet("bf16")` bloğunda girdiler bf16'ya çevrilir, sonuç float32'dir. Hiçbir konuma bakamayan satırın çıktısı 0'dır.
- **Örnekleme:** `örnekle(logit, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok)` son eksendeki `softmax(logit / sıcaklık)` dağılımından bir parça kimliği örnekler; sonuç son ekseni düşürülmüş, tam sayılı bir tensördür (`.değer` `Tam`): `[B, V]` logitten `[B]` kimlik. `en_iyi_k: 40` yalnızca en olası 40 parça arasından seçer; `sıcaklık: 0` en olasıyı verir (`maks_indeks` ile aynı). Aynı `tohum` aynı sonucu verir; tohumsuz çağrılar her seferinde farklıdır. Türev izlemez.
- **Model parametreleri:** `parametreler(model)` bir yapı nesnesini (iç içe yapılar, listeler ve sözlükler dahil) gezip türev izleyen yaprak tensörleri sabit sırayla (alan tanım sırası) bir liste olarak verir; aynı tensör bir kez yer alır: `AdamW(parametreler(model), öğrenme_oranı: 0.001)`. `parametreler(model, adlarıyla: doğru)` yol adı → tensör sözlüğü verir (`"bloklar.0.dikkat.Wq"`; liste indeksleri sayı, sözlük anahtarları anahtarın kendisi); bu sözlük doğrudan kaydedilir: `tensör_kaydet("model.trt", parametreler(model, adlarıyla: doğru))`.
- **Türev:** `geri_yayılım(kayıp)` (kayıp tek elemanlı olmalı), `x.gradyan` (gradyanlar **birikir**: her `geri_yayılım` yeni gradyanı öncekine ekler, bu yüzden eğitim döngüsünde her adımda önce `op.sıfırla()` çağrılır; `x.gradyan` bir kopya değil gradyanın kendisidir: sonraki `geri_yayılım`, `op.sıfırla()` ve `gradyan_kırp` onu da değiştirir, saklamak isterseniz `tensör(x.gradyan)` ile kopyalayın), `türevsiz { ... }` bloğu türev izlemeden çalışır ve bloğun değerini (son ifade ya da `sonuç`) döndürür, `türevsiz(x)` tensörü hesap çizgesinden ayırır.
- **Kullanıcı tanımlı türev:** `özel_türev(ileri: ..., geri: ...)` yeni bir işlemin ileri ve geri hesabını kendiniz yazmanızı sağlar (yeni bir katman fikrini çizgeyi şişirmeden denemek için). `ileri` türevsiz çalışır ve `[çıktı, saklananlar]` döndürür; geri hesabın ihtiyaç duyacağı her şeyi `saklananlar` listesine **açıkça** koyarsınız. Sonuç çizgeye tek düğüm olarak kaydedilir; geri yayılımda `geri(saklananlar, g)` çağrılır ve her konumlu girdi için girdiyle aynı şekilde bir gradyan ya da `yok` döndürür (tensör olmayan girdiler için `yok`; yayınlama yapılmaz, yanlış şekil açık hatadır). Adlı argümanlar (`ölçek: 0.5` gibi) `ileri`ye geçer, gradyan almaz. Çıktı bir tensör listesiyse çağrı liste döndürür ve `g` her çıktının gradyanını içeren listedir (kayba bağlı olmayan çıktı için sıfır tensör). Saklanan bir tensör geri yayılımdan önce yerinde değiştirilirse hata alırsınız. `türev_denetle(f, [girdiler...], adım: 0.001, tolerans: 0.01)` geri hesabınızı merkezi farkla bulunan sayısal gradyanla karşılaştırır: doğruysa `doğru` döner, yanlışsa en büyük farkın yerini ve değerlerini yazan bir hata verir; sıradan TR fonksiyonlarında da çalışır. Ayrıntılı örnek (satıra özel kapılı nöron): `tests/ozel_turev_test.tr`.
- **İndeks:** `maks_indeks(x, eksen: -1)` en büyük elemanın indeksini verir (eşitlikte ilki). Sonuç her zaman tam sayı değerli, türev izlemeyen bir tensördür (o eksen kaldırılır; 1 boyutlu girdide skaler); doğrudan `gömme`/`çapraz_entropi` indeksi olarak kullanılabilir. Tensör tam sayılı işaretlidir: ondalıksız yazdırılır ve `.değer` `Tam` (ya da `Tam` listesi) verir, böylece `harfler[maks_indeks(x).değer[0]]` gibi liste indekslemede doğrudan kullanılır. Bu tensörle yapılan işlemlerin sonucu (ör. `i * 2`) yine Ondalık değerlidir.
- **Gradyan kırpma:** `gradyan_kırp(parametreler, en_fazla: 1.0)` gradyanların toplam L2 normunu `en_fazla` ile sınırlar ve kırpma öncesi normu 0 boyutlu tensör olarak döndürür (sayısı `.değer`); `geri_yayılım` ile `op.adım()` arasında çağrılır. GPU'da norm ve kırpma cihazda yapılır, program beklemez.
- **Kaydetme/yükleme:** `tensör_kaydet("model.trt", {"w": w, "b": b})` ve `değişken p = tensör_yükle("model.trt")` (isteğe bağlı `türev_izle: doğru`). Göreli yollar `dosya_oku`/`dosya_yaz` ile aynı kuralla çözülür (önce çalışma klasörü, orada yoksa programın klasörü); dosya varsa üzerine yazılır; yazma atomiktir (önce geçici dosyaya yazılıp yerine konur), eğitim kesilse ya da disk dolsa bile önceki kontrol noktası bozulmaz. Dosya hataları Türkçedir (`tensör_yükle: "model.trt" yüklenemedi: böyle bir dosya ya da klasör yok (yolun yazımını denetleyin); bakılan yer: ...`). Biçim (sürüm 2, küçük sonlu): `TRTENSOR` (8 bayt), sürüm `uint16`, girdi sayısı `uint32`; her girdi için ad uzunluğu `uint32` + UTF-8 ad, veri türü kodu `uint8` (1 = float32, 2 = bf16, 3 = fp16), eksen sayısı `uint8`, her eksen `uint64`, satır öncelikli veri (float32 4 bayt, bf16/fp16 2 baytlık bit deseni; 16 bitlik tensörler 16 bit olarak yazılır ve aynı türde yüklenir); sonda tüm baytların CRC-32 değeri `uint32`. Eski sürüm 1 dosyaları artık okunmaz. Bozuk, eksik, bilinmeyen sürümlü ya da başka türden dosya açık bir Türkçe hatayla reddedilir; indirilen bir model dosyası da güvenle denenebilir: bir tensör en fazla 50.000.000 eleman bildirebilir (sıfır uzunluklu eksenler 1 sayılır). Hangi girdilerin güvenilmeyen sayıldığı ve açıkların nasıl bildirileceği: [`SECURITY.md`](../../SECURITY.md).
- **İyileştiriciler:** `AdamW(parametreler, öğrenme_oranı: 0.001, ağırlık_çürümesi: 0.01, beta1: 0.9, beta2: 0.999, epsilon: 0.00000001)`, `SGD(parametreler, öğrenme_oranı: 0.01, momentum: 0, ağırlık_çürümesi: 0)`; `.adım()` parametreleri günceller, `.sıfırla()` gradyanları sıfırlar. Parametreler doğrudan `tensör(..., türev_izle: doğru)` ile oluşturulmuş tensörler olmalıdır. İkisi de `ana_kopya: doğru` (varsayılan) seçeneğini alır: 16 bitlik (`bf16`/`fp16`) parametrelerin float32 ana kopyası tutulur, güncelleme ana kopyada yapılıp parametreye yuvarlanarak yazılır; böylece 16 bitlik aralıktan küçük adımlar kaybolmaz (`ana_kopya: yanlış` ile güncelleme doğrudan 16 bitlik parametrede yapılır). float32 parametreleri etkilemez; moment tahminleri her zaman float32'dir.

```tr
// y = 2·x - 1 doğrusunu öğren
değişken X = tensör([[0], [1], [2], [3]])
değişken Y = tensör([[-1], [1], [3], [5]])
değişken w = tensör([[0.0]], türev_izle: doğru)
değişken b = tensör([0.0], türev_izle: doğru)
değişken op = AdamW([w, b], öğrenme_oranı: 0.1, ağırlık_çürümesi: 0)

döngü adım içinde 0..500 {
    op.sıfırla()
    değişken fark = X @ w + b - Y
    geri_yayılım(ortalama(fark * fark))
    op.adım()
}
yazdır("w = {yuvarla(w.değer[0][0], 3)}, b = {yuvarla(b.değer[0], 3)}")   // w = 2.0, b = -1.0
yazdır(türevsiz { sonuç X @ w + b })                                       // Tensör[4×1] [...]
```

Kullanıcı tanımlı türev: kapılı çarpım `a * sigmoid(b)` için elle yazılmış geri hesap ve denetimi.

```tr
değişken kapılı = özel_türev(
    ileri: fonksiyon(a, b) {
        değişken s = sigmoid(b)
        sonuç [a * s, [a, s]]                       // [çıktı, saklananlar]
    },
    geri: fonksiyon(k, g) {                          // k: saklananlar, g: çıktının gradyanı
        değişken a = k[0]
        değişken s = k[1]
        sonuç [g * s, g * a * s * (1 - s)]           // a ve b için birer gradyan
    }
)
değişken a = tensör([[1.0, -2.0]], türev_izle: doğru)
değişken b = tensör([[0.5, 0.0]], türev_izle: doğru)
geri_yayılım(toplam(kapılı(a, b)))
yazdır(a.gradyan, b.gradyan)
yazdır(türev_denetle(kapılı, [a, b]))               // doğru (yanlış bir geri hesabında açık hata)
```

Şekil hataları şekli gösterir ve `dene/yakala` ile yakalanabilir: `[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512`. Fonksiyon, metot, işleç (`@`, `+`, `<`...) ve özellik hatalarının başında `Satır N:` bulunur: `Satır 3: [2, 2] ile [1, 3] matris çarpımı yapılamaz; 2 ≠ 1 (...)`.

Bellek sınırı: bir işlemin sonucu tek başına makinenin fiziksel belleğinden büyükse tensör ayrılmadan önce Türkçe bir hata verilir ve program çökmez. Çoğu zaman sebep beklenmedik bir yayınlamadır: `sıfırlar([185000, 1, 1]) + sıfırlar([4, 46000])` hatası `toplama: sonuç tensörü [185000, 4, 46000] şeklinde, 126,8 GB bellek ister; bu makinede 31,5 GB var. Şekilleri denetleyin (yayınlama beklenmedik bir boyut üretmiş olabilir).` olur. Sınır ayarlanmaz; ayrıntı: dil tanımı **15.25**.

> **Not (türevsiz ve eşzamanlılık):** Türevsiz kip her görevin kendi durumudur. `türevsiz { ... }` bloğu fonksiyon çağırsa, döngü kursa ya da `bekle(...)` gibi yorumlayıcı kilidini bırakan bir çağrı yapsa da, aynı anda çalışan başka bir `eşzamanlı` görev (ör. eğitim döngüsü) türev izlemeye devam eder; yani bir görevde türevsiz çıkarım (ör. örnek metin üretme) yaparken arka planda eğitim sürdürebilirsiniz. Yeni bir `eşzamanlı` görev her zaman türevli kipte başlar. Bir geri yayılım sürerken başka bir görev aynı çizgeden geri yayılım başlatırsa açık hata alır.

VS Code İzleme Masası'nda tensörler tip sütununda `Tensör [2×3]` (float32 dışındaki türlerde `Tensör [2×3] bf16`) olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılabilir. Skaler tensörün (ör. kayıp) değeri değer sütununda görünür (`0.6074 (Tensör, skaler)`, bf16'da `2.5000 (Tensör, skaler, bf16)`); 1-8 elemanlı tensörler satır içinde kısaltılmış yazılır (`[[1.4298]] (Tensör, 1×1)`), daha büyükleri tablo olarak açılır. Variables panelinde tensörler ilk ekseni boyunca açılır (satırlar, onların içinde sayılar; 1000'den çok satırda ilk 1000 satır ve bir `…` satırı gösterilir).

### GPU'da eğitim

NVIDIA GPU'su ve sürücüsü olan bir makinede (ör. bulut GPU makinesi) aynı TR programı GPU'da
çalışır; ek kurulum, derleyici ya da kitaplık gerekmez. En sade yol, modeli kurmadan önce
varsayılan cihazı seçmektir: ağırlıklar, iyileştiricinin durumları ve veri kümesinden gelen
toplu işler GPU'da oluşur.

```tr
eğer cihazlar().içerir("gpu") { varsayılan_cihaz("gpu") }   // GPU yoksa CPU'da kalır
değişken W = tensör(türevsiz(rastgele_normal([16, 4], tohum: 1) * 0.1), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)              // varsayılan cihazda
değişken iyileştirici = AdamW([W], öğrenme_oranı: 0.01)
döngü adım içinde 0..10 {                                    // döngü CPU'dakiyle aynı
    iyileştirici.sıfırla()
    değişken kayıp = çapraz_entropi(x @ W, [0, 1, 2, 3, 0, 1, 2, 3])
    geri_yayılım(kayıp)
    iyileştirici.adım()
}
yazdır(W.cihaz, varsayılan_cihaz())
```

Veri kümesinin örnekleyicisi (`veri_kümesi(...).örnekleyici(...)`) de toplu işleri varsayılan
cihazda verir; büyük bir modelin tamamı için örnek: `tools/gpu/compare_gpu.tr`.

- `cihazlar()` kullanılabilir cihazları verir (`["cpu", "gpu"]`); `varsayılan_cihaz()` geçerli
  cihazın adını. `t.cihaz` bir tensörün cihazını, `t.cihaza("cpu")` taşınmış kopyasını verir.
- Türev izleyen bir ağırlığı tek tek taşımak da olur: `kendi.W = ağırlık(...).cihaza("gpu")`
  GPU'da **yeni bir yaprak** verir; iyileştiriciye bu verilir (gradyanı onun `.gradyan`ında).
- Farklı cihazlardaki iki tensörle işlem açık bir hatadır; sessizce kopyalanmaz.
- GPU'daki işlemler sıraya eklenip hemen döner; `.değer` ve yazdırma hesabın bitmesini
  bekler (`gradyan_kırp` beklemez; döndürdüğü norm da okununca bekler). Süre ölçerken ölçümden önce bir `.değer` okuyun.
- Sonuçlar CPU'dakiyle float32 yuvarlama farkı içinde aynıdır; indirgemeler belirlenimcidir.
  GPU belleği yetmezse Türkçe ve `dene/yakala` ile yakalanabilir bir hata alınır.
- Teşhis: bir GPU hatasının hangi işlemde olduğunu tam bulmak için program
  `TR_GPU_ESZAMANLI=1` ortam değişkeniyle çalıştırılır (her işlemden sonra bekler; yavaştır).
- Hız dökümü: `TR_GPU_PROFIL=1` ile çalışan program sonunda standart hataya GPU profilini
  yazar (adım başına süreler, hangi çekirdeğin GPU zamanının ne kadarını aldığı, bellek).
  `tr izle` ve `tr profil` aynı ölçümü canlı tabloda ve raporda gösterir (bkz. §11).
- Matris çarpımı, makinede NVIDIA'nın cuBLASLt kitaplığı kuruluysa (CUDA araç takımı ya da
  PyTorch ile gelir) onunla yapılır; yoksa TR'nin kendi çekirdeğiyle (sonuç aynı, hız farklı).
  Kitaplık başka bir yerdeyse yolu `TR_GPU_CUBLASLT=/tam/yol/libcublasLt.so.12` ile verilir
  (çalışma klasörü hiçbir zaman aranmaz); hangi yolun kullanıldığı profil özetinin ilk
  satırında görünür.
- Kurallar: `docs/spec/15_tensorler.md` **15.26–15.30**.

## D. Veri ve Parçalayıcı
Büyük metin derlemleriyle model eğitmek için iki yerleşik nesne vardır: `tokenizer.json` dosyalarını okuyan **parçalayıcı** ve `.npy` veri dosyalarını okuyan **veri kümesi**. Uçtan uca örnek: `examples/tr/39_veri_ile_egitim.tr`.

**Parçalayıcı**
- `değişken p = parçalayıcı_yükle("tokenizer.json")` yaygın `tokenizer.json` biçimini okur (BPE, WordPiece, WordLevel, Unigram modelleri; bayt düzeyi, Metaspace, BERT tarzı bileşenler). Desteklenmeyen bir bileşen türü, hangi bileşenin hangi türü olduğunu söyleyen bir hatayla bildirilir. Tip adı `Parçalayıcı`.
- `p.dağarcık_boyutu` dağarcıktaki parça sayısıdır; `p.parça_kimliği("<|endoftext|>")` parçanın kimliğini verir, dağarcıkta yoksa `yok`.
- `p.kodla(metin)` bir `Tam` listesi, `p.kodla([metin1, metin2])` listelerin listesini verir (liste tüm işlemci çekirdeklerinde paralel kodlanır, sıra korunur). Metnin içinde yazılı özel parçalar (`"<|endoftext|>"`) tek bir kimlik olarak tanınır.
  - `özel_parçalar: yanlış` son işleyicinin eklediği özel parçaları (`[CLS] … [SEP]`, `<s>` gibi) kapatır; varsayılan `doğru`dur.
  - `ikinci: metin` metin çiftini (soru + bağlam gibi) tek dizi olarak kodlar; liste verilirse `ikinci:` de aynı uzunlukta bir liste olmalıdır.
- `p.çöz(kimlikler)` metin verir; listelerin listesi ya da 2 boyutlu tensör verilirse metin listesi, 1 boyutlu tensör verilirse metin döner (ör. `.sonraki()` ile alınan `girdi` doğrudan çözülebilir). Varsayılan, başvuru kütüphanesindeki gibi **özel parçaları atlamaktır**; `özel_parçalar: doğru` onları da yazar. Dağarcıkta olmayan kimlikler sessizce atlanır.
- Desenlerdeki Unicode sınıfları (`\p{Latin}`, `\p{Alphabetic}`, `[[:punct:]]` ...) ve büyük/küçük harf duyarsız eşleşme (`(?i)`), dosyaları üreten başvuru uygulamasının Unicode sürümünü ve sınıf tanımlarını kullanır (ör. `[[:punct:]]` ASCII simgelerini de kapsar, köşeli parantezsiz `(?i)\p{Lu}` katlanmaz); başvurunun tanımadığı bir sınıf adı yüklemede hata verir.
- Güvenlik: parçalayıcı dosyasındaki düzenli ifadeler adım bütçesiyle çalışır. Bütçe bir `kodla` (ya da `çöz`) çağrısının bütün aramalarına birlikte uygulanır ve metnin uzunluğuyla orantılıdır; böylece tek tek zararsız görünen ama toplamda karesel ya da üstel büyüyen aramalar da durur. Kötü niyetli bir metin aşırı geri izlemeye yol açarsa yarım sonuç yerine `dene/yakala` ile yakalanabilen bir "adım sınırını aştı" (ya da "geri izleme belleği sınırını aştı") hatası oluşur; motor çağrı yığınını kullanmadığından uzun girdi süreci çökertmez. Üstel geri izlemeye açık (iç içe sınırsız niceleyicili) ve 1000 düzeyden derin iç içe desenli dosyalar yüklenmez; daha hafif riskler yüklemede standart hataya dosya başına bir kez `Uyarı:` olarak yazılır. Parçalayıcı dosyası güvenilmeyen girdi sayılır ([`SECURITY.md`](../../SECURITY.md)).

**Veri kümesi**
- `değişken küme = veri_kümesi("veri/train")` klasördeki `.npy` veri dosyalarını ad sırasıyla tanır (tek bir `.npy` dosyası da verilebilir). Veri dosyaları tek boyutlu, küçük uçlu `uint16`, `uint32` ya da `int32` parça kimliği dizileridir (standart `.npy` dosya biçimi). Klasörde ya da üst klasörde `meta.json` varsa veri dosyalarının parça sayıları oradan okunur; dosya başlıkları açılışta okunmaz. Tip adı `VeriKümesi`.
- **Önbellek:** veri yavaş bir sürücüdeyse (ağ, akışlı bulut sürücüsü) `veri_kümesi(dizin, önbellek: "onbellek/veri", kota_gb: 20)` veri dosyalarını kullanılmadan önce yerel klasöre kopyalar; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. `önbellek:` verilince `kota_gb:` **zorunludur** (disk alanını sessizce doldurmamak için); verilmezse hata, bu küme için gereken en küçük kotayı (aynı anda yerelde tutulan 8 veri dosyası × en büyük veri dosyası) söyler. Aynı önbellek klasörünü birden çok küme (ör. eğitim ve doğrulama) paylaşabilir: kota ortaktır (en son verilen geçerlidir) ve kümelerin örnekleyicileri birlikte sığmıyorsa hata gereken toplamı söyler. Bir önbellek klasörünü aynı anda yalnızca bir TR programı kullanabilir; ikinci program açık bir hata alır. Önbellek klasörü de diğer yollar gibi çözülür (çalışma klasörü, yoksa programın klasörü). Önbellekteki kopyalar belleğe eşlenerek okunur; eğitim sürerken bir kopya dışarıdan kısaltılırsa program çökmez, `.sonraki()` açık bir hata verir. Önbelleksiz okumada veri dosyaları belleğe eşlenmez, her pencere ayrı okunur: yalnızca istenen baytlar sürücüden çekilir.
- `küme.örnekleyici(toplu: 32, bağlam: 1024, tohum: 1)` eğitim için rastgele pencereler üretir (tip `Örnekleyici`; `tohum` varsayılanı 0). Aynı tohum ve küme hep aynı toplu işleri verir. Her dönemde veri dosyası sırası karışır, pencereler üst üste binmez (bir dönemde her parça en çok bir kez görülür) ve hiçbir pencere veri dosyası sınırını aşmaz; örnekleyici dönemden döneme sonsuz üretir. Pencereler `bağlam + 1` parçadır ve veri dosyasını rastgele bir başlangıç kaymasıyla böler: her veri dosyasının `bağlam + 1`'e bölünmeyen artığı (en çok `bağlam` parça) o dönem kullanılmaz ve her pencerenin ilk parçası yalnızca girdi olur (hedef olmaz). Kayma her dönem değiştiğinden bunlar dönemden döneme farklı parçalardır; ama veri dosyaları bağlama göre kısaysa kayıp büyüktür (ör. 2000 parçalık veri dosyası ve `bağlam: 1500` ile dönem başına dosyanın %25'i kullanılmaz), bu yüzden veri dosyalarını bağlamdan çok uzun tutun.
- `küme.sıralı(toplu: 32, bağlam: 1024)` doğrulama için veri dosyalarını sırayla okur (tip `SıralıOkuyucu`): ardışık pencereler `bağlam` kadar kayar, böylece her parça en çok bir kez hedef olur: her veri dosyasının ilk parçası ve son pencereye sığmayan kuyruğu (en çok `bağlam - 1` parça) hedef olmaz, `bağlam + 1`'den kısa veri dosyaları hiç okunmaz. Son toplu iş daha az satırlı olabilir; veri bitince `.sonraki()` `yok` döner.
- `.sonraki()` `[girdi, hedef]` listesi verir: ikisi de `[toplu, bağlam]` şeklinde, tam sayı değerli tensördür ve `hedef` girdinin bir kaydırılmışıdır. Doğrudan `gömme(E, girdi)` ve `çapraz_entropi(çıktı, hedef)`'e verilir: `değişken girdi, hedef = ö.sonraki()`. İkisi de tam sayılı işaretlidir: ondalıksız yazdırılır ve `.değer` `Tam` listesi verir, böylece liste indekslemede doğrudan kullanılır. Bu tensörlerle yapılan işlemlerin sonucu (ör. `girdi + 1`) Ondalık değerlidir.
- **Önden yükleme her zaman açıktır:** sonraki 4 toplu iş arka planda hazırlanır. Bu iş TR kodu çalıştırmaz, yorumlayıcı kilidini tutmaz; `.sonraki()` beklerken kilit bırakılır, böylece `eşzamanlı` görevler ilerler. Aynı okuyucuyu birden çok görev paylaşabilir (her toplu iş bir göreve gider).
- **Kaynaklar:** açık bir kapatma adı yoktur. Sıralı okuyucu bitince dosyalarını hemen kapatır; artık kullanılmayan (hiçbir değişkenin göstermediği) okuyucu ve kümeler çöp toplayıcı tarafından kapatılır; program bitince açık kalanların hepsi kapatılır (takılan bir ağ okumasında en çok birkaç saniye beklenir).
- Göreli yollar, dosya okuyan ve yazan bütün yerleşiklerde (`dosya_oku`, `dosya_yaz`, `aktar`, `parçalayıcı_yükle`, `veri_kümesi`, `csv_oku`, `tensör_kaydet`, `tensör_yükle`) aynı kuralla çözülür: göreli bir yol önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür ve dosyanın bulunduğu ilk yer kullanılır; dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir (düz bir `rapor.txt` için bu çalışma klasörüdür). Kural okuma ve yazma için aynıdır: programın yanındaki bir dosya okunup güncellenince yine oraya yazılır, yeni dosyalar çalışma klasöründe oluşur. Dosya bulunamazsa hata bakılan yerleri listeler.
- Hatalar Türkçe ve yol göstericidir (dosya yok, `.npy` değil, kota yetersiz, bağlam veri dosyalarından uzun, adlı argüman yazım hatası...) ve `dene/yakala` ile yakalanır.

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.dağarcık_boyutu)
değişken küme = veri_kümesi("veri/train")
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken E = tensör(türevsiz(rastgele_normal([p.dağarcık_boyutu, 64], tohum: 1) * 0.1), türev_izle: doğru)
değişken W = tensör(türevsiz(rastgele_normal([64, p.dağarcık_boyutu], tohum: 2) * 0.1), türev_izle: doğru)
değişken op = AdamW([E, W], öğrenme_oranı: 0.01)
döngü adım içinde 0..100 {
    değişken girdi, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(E, girdi) @ W, hedef)
    geri_yayılım(kayıp)
    op.adım()
}
değişken doğrulama = veri_kümesi("veri/_valid").sıralı(toplu: 8, bağlam: 256)
döngü doğrulama.sonraki() != yok { }   // bitince yok döner
```

VS Code İzleme Masası'nda bu nesneler tip sütununda `Parçalayıcı (dağarcık 40000)`, `Veri kümesi (246 veri dosyası, 56,9 milyar parça)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)` gibi görünür; parçalayıcı (eklenmiş/özel parçalar) ve veri kümesi (veri dosyaları) tablo olarak açılabilir.

## E. Dosya ve Yol

Veri klasörünü gezmek, çıktı klasörü açmak, kontrol noktalarını yedeklemek ve büyük metin derlemlerini satır satır okumak için. Kurallar ve ayrıntılar: dil tanımı [§18](../spec/18_dosya_ve_yol.md).

- **Yol:** `yol("çıktı", "model.trt")` parçaları `/` ile birleştirip temizler; `Yol` her platformda `/` ile yazılır. Yola sorulur: `.dosya_adı`, `.uzantı`, `.uzantısız_ad`, `.üst_klasör`, `.mutlak`, `.var_mı`, `.dosya_mı`, `.klasör_mü`, `.bayt`. İlk parçadan sonra mutlak bir parça hatadır (dışarıdan gelen bir ad taban klasörü değiştiremez). Yol alan her yerleşik (`dosya_oku`, `dosya_yaz`, `csv_oku`, `tensör_kaydet`, `veri_kümesi` ...) `Metin` yerine `Yol` da alır.
- **Listeleme:** `klasörü_listele("veri", "**/*.npy")` kalıba uyan dosya ve klasörleri her çalıştırmada aynı sırayla verir; `*`, `?`, `[a-z]`, `**` (alt klasörler). Gizli adlar (`.git`) kalıp nokta ile başlamadıkça listelenmez.
- **Klasör, silme, kopyalama, taşıma:** `klasör_oluştur("çıktı/kontrol_noktaları")` (varsa sorun değil), `kopyala("çıktı/model.trt", "yedek")` (→ `yedek/model.trt`), `taşı(a, b)`, `sil(yol)`. Güvenli varsayılanlar: var olan hedefin yerine koymak `üzerine_yaz: doğru`, dolu klasörü silmek `içindekilerle: doğru` ister; `sil` kökü, ev ve sistem klasörlerini, çalışma ve program klasörünü ve bunların üstlerini silmez, salt okunur dosyayı zorla silmez; bağlantılar (Windows dizin bağlantıları dahil) izlenmez. Kopya önce geçici bir ada yazılır, sonra yerine konur.
- **Geçici klasör:** `geçici_klasör()` işletim sisteminin geçici klasöründe bu program için yeni, boş bir klasör verir; program bitince içindekilerle silinir (ara dosyalar, testler).
- **Büyük dosyalar:** `döngü satır içinde dosya_satırları("derlem.jsonl.gz") { ... }` dosyayı akışla okur (bellek dosya boyutundan bağımsız; tek satır en çok 64 MB). Sonuna kadar okumadan bırakılacak okuyucu `o.kapat()` ile kapatılır (dosya böylece silinip taşınabilir). `.gz` uzantılı dosyalar `dosya_oku`, `dosya_satırları` ve `csv_oku`'da açılarak okunur, `dosya_yaz`'da sıkıştırılarak yazılır.
- **Atomik yazma:** `dosya_yaz` de `tensör_kaydet` gibi önce geçici dosyaya yazıp yerine koyar: yarıda kalan yazma eski dosyayı bozmaz. Bedeli: her yazma diske işlenir (çok sayıda küçük yazma eskisinden yavaştır) ve dosyanın sabit bağlantıları kopar.

```tr
değişken çıktı = klasör_oluştur(yol("çıktı", "deneme_1"))
döngü p içinde klasörü_listele("veri", "**/*.npy") { yazdır(p.dosya_adı, p.bayt) }
değişken model = yol(çıktı, "model.trt")
eğer model.var_mı { kopyala(model, klasör_oluştur("yedek"), üzerine_yaz: doğru) }
değişken n = 0
döngü satır içinde dosya_satırları("veri/derlem.jsonl.gz") { n += 1 }
yazdır("{n} satır")
```

## F. Eğitimde ayarlar, günlük ve ilerleme
Uzun bir eğitimi izlemek için üç yerleşik yeter: betiğin ayarlarını komut satırından alan `komut_satırı`, her adımı gösteren `ilerleme` çubuğu ve sonradan bakılacak `günlük` dosyası. Kurallar: [`docs/spec/19_surec_ve_kayit.md`](../spec/19_surec_ve_kayit.md). Uçtan uca örnek: `examples/tr/39_veri_ile_egitim.tr`.

```tr
değişken ayar = komut_satırı({adım: 2000, öğrenme_oranı: 0.001, günlük_dosyası: yok},
    açıklama: "Küçük dil modelini eğitir",
    açıklamalar: {adım: "eğitim adımı sayısı", günlük_dosyası: "ölçülerin yazılacağı .jsonl dosyası"})
değişken g = eğer ayar.günlük_dosyası != yok { günlük(ayar.günlük_dosyası) } değilse { yok }
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde 0..ayar.adım {
    değişken kayıp = 1.0 / (adım + 1)   // burada modelin bir eğitim adımı çalışır
    ç.güncelle({kayıp: kayıp, öğrenme_oranı: ayar.öğrenme_oranı})
}
```

**Ayarlar (`komut_satırı`).** Sözlüğün anahtarları seçenek adları, değerleri varsayılanlardır; tip varsayılandan çıkar. `tr egit.tr --adım 5000 --öğrenme_oranı=3e-4` seçenekleri değiştirir, `tr egit.tr --yardım` hepsini açıklamalarıyla listeler. Yanlış yazılan seçenek (`--ogrenme_orani`, `--adim`) ya da tipine uymayan değer (`--öğrenme_oranı 0,001`) eğitim başlamadan, doğru adı öneren bir mesajla durdurur. Argüman vermeden çalıştırınca varsayılanlar kullanılır.

**İlerleme çubuğu.** Her `ç.güncelle(...)` bir adım ilerletir; çubuk terminalde tek satırda yenilenir, geçen adım sayısını, ölçüleri, adım hızını ve kalan süreyi gösterir. Çıktı bir dosyaya ya da not defteri hücresine gidiyorsa 10 saniyede bir düz satır yazılır. `yazdır` çubuğu bozmaz. Kaldığı yerden sürdürülen bir eğitimde `ç.güncelle(..., adım: n)` sayacı n'ye getirir.

```text
adım 1200/2000 ▕█████████▌░░░░░░▏ %60 · kayıp 2,31 · öğrenme_oranı 0,001 · 85,2 adım/sn · kalan 9 sn
adım 2000/2000 · kayıp 1,84 · 24 sn sürdü
```

**Günlük.** `günlük("kayit/egitim.jsonl")` her satırı bir JSON kaydı olan bir dosya açar; `ilerleme(..., günlük: g)` her adımın ölçülerini oraya da yazar, `g.ekle({doğrulama_kaybı: d})` ayrı kayıt ekler. Dosyanın başında programın adı ve komut satırı ayarları durur: hangi ayarla hangi kaybın alındığı sonradan da okunur. Dosya varsa sonuna eklenir; kayıpta NaN belirirse gizlenmez, `"NaN"` olarak yazılır. Kesilen bir eğitimi kaldığı yerden sürdürmek için: [§9, uzun eğitim](09_uzun_egitim.md).

**Gizli değerler.** Erişim anahtarını ortam değişkeniyle verin: `değişken belirteç = ortam_değişkeni("HF_TOKEN")`. Adı `TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `ŞİFRE`, `ANAHTAR` gibi bir sözcük içeren değişken ve seçeneklerin değeri ekranda, hata mesajlarında, günlükte ve hata ayıklayıcıda `***` olarak görünür (`TOKENIZER_PATH` ya da `max_tokens` gibi adlar etkilenmez).
