# 15. Tensörler ve otomatik türev: emsal ve gerekçe

Kurallar: [`15_tensorler.md`](../15_tensorler.md). Bu dosya her kuralın emsalini (başka dillerde ve araçlarda nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 15.1

- *Emsal:*
  - Öğe cinsi tipte [kanıtlı: DENETİM açtı; Burn ikinci kez açıldı]: Burn 0.22.1 (Rust) `pub struct Tensor<const D: usize, K = Float>`: cins (`Float`, `Int`, `Bool`) tipte, hassasiyet arka uçta; TR'ye en yakın emsal. Rust ndarray 0.17.2 `Array<A, D>` ve Julia `Array{T,N}` öğe tipini tipte taşır; Swift for TensorFlow `Tensor<Scalar>` da taşıyordu, proje 2023'te arşivlendi. PyTorch, JAX ve NumPy taşımaz (dinamik). TR yalnız cinsi tipe koyar, hassasiyeti çalışmaya bırakır.
  - Cinsi çalışırken bilinen tensör: Rust'ın `safetensors`'ının `TensorView`'u (cins çalışırken, kullanırken daraltılır) [kanıtsız: bu tur açılmadı].
  - Şekilli tipler: jaxtyping [kanıtsız].
  - float64: JAX'ın 32 bit varsayılanı TR'ninkiyle aynıdır, float64 açıkça istenir; safetensors'ta `F64` vardır.
  - Hassasiyet listesi [kanıtlı: safetensors `Dtype`; `internal/compute/dataload/npy.go`'nun `<u2`, `<u4`, `<i4` desteği]: "int16" ses (PCM16), "int8" nicemleme ham verisi, "uint16" ve "uint32" parçalanmış metin verisi (65 535'ten büyük sözlükte belirteç `uint32`'dir), "uint8" görüntü.
- *Gerekçe:*
  - **Öğe cinsi tipte.** Kimliklerin float32 saklanması 2²⁴'ü aşan kimliği sessizce bozar: `TR_Tensor([16777217.0])` → 16777216 [ölçüldü]. Veri yükleyicinin milyarlık indeksi, uzun bağlamın konumu ve sözlük kimliği tam sayı ister ("Veriyi hazırlamak", "Hazır modelleri çalıştırmak"). Statik cins, indeks isteyen işlemin (`TR_embedding_lookup`, `TR_gather`) ondalık tensörü çalışmadan önce reddetmesini sağlar ("Hata riski az"). `TR_tolist()` ve `TR_item()` ikinci adlar gerekmeden doğru tipi verir (13.8a). 13.10'a göre tip değerden çıkamaz; bu yüzden cins `TR_dtype:` ile değil, tip parametresiyle seçilir. Hassasiyet tipte değildir: bf16 ile float32 arasındaki geçiş ve nicemlenmiş ağırlık tipi değiştirmez.
  - **Bedel** [kanıtlı: DENETİM'in sayımı; süre hipotez]: motorun 45 yönteminin 12–15'i ile 7 GPU çekirdek dosyası. Kararı değiştirmez, uygulamanın sırasını belirler (status).
  - **`TR_AnyTensor` ve `.TR_narrow[E]()`.** Bir dosya birden çok cins taşır (model ve adım sayacı; Hugging Face'in I64 tamponları); cins dosyadan gelir, tipte yazılı olamaz. Üç yol tartıldı (15.46): cinsi silinmiş tensör tipi, bütün dosyayı tek cins okuyan `TR_load_tensors[E]`, cins başına sözlük; cinsi çalışırken bilinen tensör alındı. Daraltma ayrı bir yazımdır, çünkü tip çağrısı hem kopya hem cins dönüşümüdür: bir yazımın üç anlamı olmasın, model yükü kopyalanmasın. Üstünde yalnız okuma vardır: yazma, görünüm ve hesap kısıtsız cinste tiplenemez. Kaplar tip parametresinde değişmezdir: değiştirilebilir sözlükte kovaryans güvensizdir (`TR_Dict[TR_String, TR_Tensor]`'a `TR_AnyTensor` diye tam sayı tensörü yazılabilirdi).
  - **Şekil tipte değil.** Statik şekil, dinamik uzunluklu dizilerde (metin üretimi, değişken toplu iş) ajana yük olur; şekilli tipler ileride açılabilir. Bugün şekil hatası çalışırken, şekilleri söyleyen iletiyle verilir (15.48).
  - **Genel fonksiyonda cins.** Kısıtsız cinste yalnız her cinste anlamı olan yapı işleri tiplenir; aritmetik ve ondalık matematik cinsin açıkça yazılmasını ister. 13.10'un sayı kısıtı tensörün cinsine de uygulanır: ajana ikinci bir kısıt dili öğretilmez.
  - **Nicemlenmiş ağırlık `TR_Float`'un saklama hassasiyetidir.** Değeri ondalıktır; `TR_Int`'e konsa ölçek kaybolur, her işlem açık dönüşüm ister. Adları "int8" gibi `TR_Int` adlarıyla çakışmaz: `.TR_dtype`'in değeri cinsi tek başına söyler. Ham 4 bitlik veriyi elle açmak isteyen `TR_Tensor[TR_Int]` "uint8" ve bit işleçleriyle yazar.
  - **float64 bir hassasiyettir.** safetensors'ta `F64` vardır; float64 olmazsa okumak ya hata ya sessiz yuvarlama olur, 15.49 sessizi yasaklar. TR'nin sayısı 64 bittir (3.2): `TR_item()` ile tensör ile sayı arasında kayıpsız geçiş ölçümde gerekir. Desteklemeyen cihazda hata verilir, sessiz düşürme yoktur; taşınabilirlik hatası açıktır. Reddedilen yol: float64'ü hiç almamak (taşınabilirlik; sayı zaten 64 bit; biriktirme çekirdekte): okunan `F64` ağırlığı sessizce yuvarlanırdı. `.npy` okuyucusu bugün ondalık okumuyor [kanıtlı: `npy.go`]; okursa (16, 18) bu da gerekçe olur.
  - **"uint64" yok:** `TR_Int`'e sığmaz; okurken hata, ileti dönüşümü önerir.
  - **Karmaşık sayı cinsi yok.** Ses modellerinin katmanları gerçek değerlidir; karmaşık sayı yalnız STFT'nin ara adımındadır. `TR_stft` gerçek ve sanal kısmı demetle (3.6a) verir; ses yerleşikleri yol haritasında.

### 15.2

- *Emsal:* PyTorch'un `torch.tensor([1, 2])`'si int64'tür: tam sayı listesinden tam sayı tensörü kurar. PyTorch'un `arange`'i ayrı bir addır. Daraltmada PyTorch ve NumPy sessizce sarar (`uint8`'e 300 → 44).
- *Gerekçe:*
  - **Cins veriden çıkar.** 6.10'un "argümandan çıkar" kuralı tek kuraldır. Cins yazılmazsa `TR_Float` olsaydı `TR_Tensor(kimlikler)` tam sayı listesini sessizce ondalığa çevirirdi: bu hem 3.6'ya (`TR_List[TR_Int]` `TR_List[TR_Float]` yerine verilmez) hem 15.49'a aykırıdır ve 2²⁴ sorununu geri getirir. Beklenen tip yalnız değişmezdeki tam sayı yazımını ondalık yapar (3.13): değişkenle gelen tam sayı listesi açık dönüşüm ister.
  - **Sığmak.** Sessiz sarma bilinen bir sessiz yanlıştır; 15.49 "sessiz kırpma yoktur" der. Kurmak, `.TR_to_dtype` ve atama aynı kurala bağlıdır.
  - **`TR_arange` yok.** Aralık (`0..n`) zaten dilin yazımıdır; ikinci bir ad ajana ikinci bir yol öğretir. İleti PyTorch alışkanlığını doğru yazıma çevirir.
  - **Şekil liste olarak.** `TR_zeros(2, 3)` ile `TR_zeros([2, 3])` iki yazım olurdu; tek yazım liste.

### 15.3

- *Emsal:*
  - Taşma: NumPy 2.5 belgesi: `np.power(100, 9, dtype=np.int32)` "gives -1486618624 (incorrect)" ve Python'un int'inden "differs significantly ... and may confuse users" [kanıtlı: numpy.org/doc/stable/user/basics.types.html, v2.5]. PyTorch ve NumPy sessizce sarar.
  - Tam sayıdan ondalığa [ölçüldü: DENETİM, PyTorch 2.14 ve NumPy 2.1.1]: PyTorch `torch.tensor([16777217]) / 1` → float32 16777216, `.float()` de aynı; NumPy float64 veriyor. Array API 2025.12'nin `divide`'ı: "If one or both of the input arrays have integer data types, the result is implementation-dependent" [kanıtlı: DENETİM]. TR PyTorch'la aynı bitleri verir.
  - Kopya reshape: PyTorch'un `reshape`'i de adımlar izin verirse görünüm, vermezse kopya verir; kopyaya yazılır, asıl tensör değişmez.
  - Yaprak olmayan tensöre yerinde yazma: PyTorch aynı programda `[[2, 2], [4, 4]]` verir [ölçüldü: DENETİM].
  - `.TR_slice`: `torch.narrow` gibi açık sınırlı API kırpmaz.
- *Gerekçe:*
  - **Tam sayı tensöründe taşma hatadır.** Ajan için tek kural: sayıda da tensörde de taşma hatadır (3.1), sarma `TR_wrapping_*` ile açıkça yazılır (13.1); hash gibi sarmanın istendiği veri işi böyle yazılır. "uint8" görüntüde 200 + 100'ün 44 olması bilinen sessiz yanlıştır. Tam sayı tensörü ML'de çoğunlukla indeks ve konumdur; küçük ve seyrek işlemlerdir. Reddedilen yol: PyTorch ve NumPy gibi sarmak.
  - **Taşmanın bedeli** [kanıtlı: DENETİM; süre hipotez]: GPU'da çekirdeklerin hata bildireceği bir cihaz bayrağı yok, yeni düzenek gerekir. Sınamanın kendisi ucuzdur (birkaç bit işlemi; int64 çarpımında yüksek yarı). Asıl bedel okumadır: hatanın kendi satırında ve `TR_try`'ın içinde çıkması için bayrak işlemden hemen sonra okunur, yani bir cihaz–konak beklemesi. Bayrak sonraki beklemede okunsaydı hata başka satırda çıkardı; 8 ve 15.48 bozulurdu. Bedel kararı değiştirmez: tam sayı aritmetiği az ve seyrektir (konum kimliği, indeks kaydırma), çoğu işlem taşamaz, biriktirme int64'te olduğundan görüntü toplamı taşmaz, sarma isteyen sıcak döngü `TR_wrapping_*` yazar.
  - **Mantık ile sayı arasında dönüşüm yok.** 3.12 sayıda bunu zaten yasaklar; tensörde ayrı kural ajana ikinci bir şey öğretirdi. Maskeden sayı `TR_where(m, 1.0, 0.0)`: kaybın maskelenmesinde asıl anlam budur. Sayıdan maske `t != 0`: NaN'ın ne olacağı 15.50'nin karşılaştırma kuralıyla bellidir. Bedeli PyTorch'un `mask.float()` alışkanlığıdır; ileti doğru yazımı verir, sayım ve oran `TR_sum(m)` ve `TR_mean(m)` ile doğrudandır (15.18).
  - **Tam sayıdan ondalığa her geçiş "float32"dir.** Soru doğruluktur: "int64" ya da "int32" tensörde 2²⁴'ten büyük değer float32'de yuvarlanır. Reddedilen yollar: (b) tam sayının bölümünü float64 yapmak: `TR_Int`'in varsayılanı "int64" olduğundan konum ve kimlik bölümlerinin hepsi float64 olur, 15.6c ile model hesabı sessizce float64'e kayar, float64'ü desteklemeyen cihazda program düşer; (c) `/`'yi kaldırıp dönüşümü açık yazdırmak: yuvarlamayı kaldırmaz (`TR_Tensor[TR_Float](i)` de float32'ye çevirir), yalnız her `/`'ye yazım ekler. Yuvarlama bozulma değildir: göreli hata ~6e-8, float32'nin olağanı; kimliğin tam sayı olarak bozulması öğe cinsiyle (15.1) çözüldü. Kesinlik isteyene açık yol: cins dönüşümü hedef hassasiyeti alır.
  - **Sürüm.** Geri yayılım saklanan tensörün sonradan değişmediğine güvenir; değiştiyse gradyan sessizce yanlış olurdu. Sürüm depodadır: görünümden yapılan yazma da yakalanır.
  - **Yaprak olmayan türevli tensöre yerinde yazma yoktur.** Sürüm sayacı bunu yakalamaz: `h = x @ w`'de `@` `h`'yi değil `x`'i ve `w`'yi saklar, ezilen öğelerden gradyan sessizce akar. Hata yazıldığı satırda verilir (8, 15.48). Reddedilen yol: her düğüm çıktısının sürümünü saklamak; hata geri yayılımda, başka satırda çıkardı.
  - **Kopya reshape yazmaya kapalı.** Kopya olmak şekle ve adımlara bağlıdır; yazılan kopya asıl tensörü değiştirmez ve bu sessiz bir yanlıştır. Bedel: depoda bir işaret, yazma girişinde tek dal [kanıtlı: DENETİM]. İleti şekli ve adımları söyler, çünkü kopya olup olmadığı koddan görünmez.
  - **En kısa ondalık.** Aynı değer listede ve tensörde aynı metni verir ("Güvenilir").
  - **Eksi eksen her yerde.** Tek kural: ajan hangi işlemin eksi ekseni aldığını ezberlemez.
  - **Koşul.** Tensör koşul olamaz (3.10). İlk öneri `TR_equal`'dır, çünkü `TR_all(a == b)` yayınlar (15.5).

### 15.4

- *Emsal:* PyTorch'un autocast'i normu float32 verir, dönüşümü `torch.compile` normla kaynaştırır; ama Llama gibi büyük model kodları ve hedef modelin kendi RMSNorm'u (`tools/gpu/rgn512.py`) girdinin türünde verir. Flax `RMSNorm(dtype=...)` çıktının türünü katmanda seçtirir. PyTorch'un `torch.get_autocast_dtype` işlevi `TR_compute_dtype()`'ın karşılığıdır. Görev mirası Python'un `contextvars`'ı gibidir.
- *Gerekçe:*
  - Normun çıktısı çoğunlukla bir çarpıma girer ve çarpım onu zaten blok türünde okur; float32 çıktı yalnız bellek tutar (hedef modelde ~4 GB). TR ikinci yolu (girdinin türünde) seçer; ağırlıklar ve sapmalar float32 kalır.
  - `TR_cumsum` float32'ye açılır: birikimli toplam fp16'nın 65504 sınırını sessizce aşar (15.18); bellek bedeli doğruluktan sonra gelir. PyTorch'un CUDA autocast'i de `cumsum`'ı float32'ye açar [kanıtlı: PyTorch 2.15 AMP belgesi, "CUDA Ops that can autocast to float32"]. Normlar bilerek ayrılır (yukarıda); onların çıktısı bir çarpıma girer, toplam taşmaz.
  - Karışık hassasiyet cinsin içindedir: hassasiyet tipte olmadığından (15.1) blok tipi değiştirmez.
  - **Görev mirası:** yeni görev açıldığı yerin hesap türünü ve türevsiz kipini alır; ajanın beklediği budur (`TR_no_grad { TR_async üret(...) }`). Reddedilen yol: yeni görevlerin float32 ile başlaması.

### 15.5

- *Emsal:* PyTorch da 1 boyutlu tensörde 0 boyutlu tensör verir. PyTorch'un metni `tensor([[1.5000, 2.0000]])` dört basamaklıdır. Uzun tensörün kısaltması NumPy gibidir: kısa eksen bütün yazılır (`[2, 4096]`'nın iki satırı da) [ölçüldü: DENETİM]. Karşılaştırma ve `TR_Bool` maske: PyTorch ve NumPy; `torch.equal` ve `torch.all`. NumPy de dizide karşılaştırma zincirini reddeder [kanıtsız: belge açılmadı].
- *Gerekçe:*
  - **Dolaşmanın öğesi hep tensör.** Boyut sayısı tipte olmadığından 1 boyutlu tensörde sayı, N boyutluda tensör veren bir kuralda `x`'in tipi çalışmadan önce bilinemezdi (1.2'ye aykırı). Şimdi her zaman `TR_Tensor[E]`, sayı için `.TR_tolist()`.
  - **Değerin metni kurma yazımıdır.** 3.14'ün bayt dizisi ve 7.6'nın yapısı gibi: ajan metni okuyunca kodu da görür. Reddedilen yol: "Tensor[2×2, float32] [...]": köşeli parantez tip yazımıyla (`TR_Tensor[TR_Int]`) karışıyordu. Dört basamak yerine 15.3'ün en kısa ondalığı: aynı değer listede ve tensörde aynı metni verir ("Güvenilir").
  - **Öğe öğe karşılaştırma ve `TR_Bool` maske.** 7.6 karşılaştırmayı "öğe öğe" diye söz vermişti. `TR_if t == u` 3.10'a göre denetim hatası olur ve ileti `TR_equal(t, u)`'yu önerir: bütün tensörün eşitliğini öğe öğe eşitlikle karıştırma hatası çalışmadan önce görünür. `TR_all(a == b)`'nin yayın tuzağı yüzünden ilk öneri `TR_equal`'dır. Reddedilen yollar: `a == b`'yi tek bir mantık değerine indirmek; float32 0/1 maske (kimlik ile maske karışır).
  - **İki istisna, anlamları gereği:** `TR_none`'la karşılaştırma öğe karşılaştırması değil, isteğe bağlı değerin var olup olmadığı sorusudur (3.5); cevabı tek bir `TR_Bool`'dur. `TR_assert_equal(a, b)` bir sınamadır ve "bu iki tensör aynı mı" diye sorar; öğe öğe maske bir sınamanın cevabı olamaz.
  - **Zincir:** `TR_and` yalnız `TR_Bool` aldığından (4.6) TR bunu denetimde yakalar ve `&`'li yazımı önerir.

### 15.6

- *Emsal:*
  - Array API standardı 2025.12: "Boolean, integer and floating-point dtypes are not connected, indicating mixed-kind promotion is undefined" [kanıtlı: data-apis.org/array-api/latest/API_specification/type_promotion.html]. PyTorch ve NumPy karışık cinsi yükseltir.
  - Mantık işleçleri: NumPy ve PyTorch maskeyi `&`/`|`/`~` ile birleştirir; NumPy 2.1.1'de `np.array([1, 0]) and np.array([1, 1])` → "ValueError: The truth value of an array with more than one element is ambiguous" [ölçüldü: DENETİM B1].
  - Kaydırma: Go ("a constant of the same type as the left operand") ve Rust (taşma "the number of bits in the type of the left-hand argument") sonucu sol işlenenden alır [kanıtlı: DENETİM, belgeler açıldı]; Array API, PyTorch ve NumPy yükseltir.
  - PyTorch 2.14 ölçümleri cinsli işleç tablosunu destekliyor (`int ** -1` hata) [ölçüldü: DENETİM B1].
- *Gerekçe:*
  - **İşleçler cinse göre.** Ajan için tek tablo; bir işlemin olmadığı yerde ileti doğru yazımı verir. Tam sayıda `/` ondalık verir, 4.2'nin sayıdaki kuralı gibi; hassasiyeti 15.3'ün tam sayıdan ondalığa geçiş kuralıdır.
  - **Cinsler karışmaz.** 3.13 ve 4.2'nin sayıdaki kuralı tensörde de geçerli kalır; kimlik tensörünün yanlışlıkla ondalık bir işleme girmesi görünür. Reddedilen yol: PyTorch'un `int + float` → float yükseltmesi.
  - **Maskeli ortalamanın iletisi.** `TR_sum(TR_where(m, 1.0, 0.0))` ondalık toplamdır, GPU'yu beklemez (bekleme listesi), ondalığa geçişi ajan kendisi yazar; 2²⁴'ün üstü yuvarlanır ama bölende zararsızdır. Reddedilen yol: `TR_Int`'i sessizce float32'ye çevirmek (2²⁴'te yuvarlar; tensörle tensörde karışık cins olurdu).
  - **`@`'de öndeki eksenler yayınlanır.** NumPy'nin `matmul`'u ve PyTorch'un `torch.matmul`'u toplu eksenleri yayınlar [kanıtsız: bu tur açılmadı]; dikkat katmanında `[B, H, T, d] @ [d, T]` gibi ortak ağırlıkla çarpım yayınsız yazılamazdı. Bugünkü motor eşitlik istiyor (status). Sessiz yanlış riski 15.6b'nin yayın kuralıyla aynıdır. (Dikkat, 15.24, kendi şekil kuralında öndeki eksenlerin eşitliğini ister: orada yayın bakış eksenleriyle karışırdı.)
  - **Kaydırmada sol işlenen.** Bit işinde anlam bit genişliğidir; sonucun genişliğini sağ işlenen değiştirmemeli. Reddedilen yol: int64'e yükseltmek (15.6a'ya istisna olurdu).
  - **`TR_and`/`TR_or` tensörde hata.** 4.6 `TR_and`'i `TR_Bool`'a ayırır; maske `&`, `|`, `~` ile birleşir.
  - **Bekleyen işlemler.** Hatası öğeye bağlı olan işlemin hatasını kendi satırında vermek (8, 15.48) GPU'da sonucun beklenmesini ister (15.3'ün bedeli). Bekleme ancak hata mümkünse yapılır: toplamda karar şekil ve hassasiyetten, veri okunmadan verilir. `TR_replay` kaydında bekleme yapılamaz; hata cihaz içi bayrakla kaydedilen satıra bağlanır. Liste tek yerdedir: ajan hangi satırın beklediğini tek yerde okur.

### 15.6a

- *Emsal:* Array API: sayı "a Python int or float for real-valued floating-point array data types", "a Python int within the bounds of the given data type for integer array" ve "Convert the scalar to a zero-dimensional array with the same data type as that of the array used in the expression" [kanıtlı: aynı sayfa]. TR aynıdır. PyTorch aritmetikte sayıyı daraltmaz, karşılaştırmada daraltır: `bf16(1.0) > 0.999` yanlış verir [ölçüldü: DENETİM]. NumPy ve PyTorch `True`'yu 1 sayar. Rust da dar işaretli tipte sığmayan onaltılık sabiti reddeder [hipotez].
- *Gerekçe:*
  - **Sayı tensörün hassasiyetini değiştirmez:** int8/uint8 ve bf16 hassasiyeti korunur ("Makinenin içinde çalıştırmak").
  - **Hesap en az float32.** Sayı daraltılmaz, karşılaştırma yuvarlanmaz: eşik maskesi doğru sonucu verir. Örnek: float32'de `TR_Tensor([0.1]) <= 0.1` doğru, bf16'da yanlıştır, çünkü bf16'nın 0.1'i 0.10009765625'tir [ölçüldü: DENETİM]: kuralın doğru sonucu. fp16'da 50000 / 1e5 → 0.49975586 [ölçüldü: DENETİM].
  - **Liste işlenen yok.** Listenin cinsi değerden çıkar (`[10, 20]` tam sayı, tensör ondalık): cinsler karışmaz (15.49) ve 13.10 ile çelişirdi. Listeyi tensör sanmak bir hata kaynağıdır; tek yazım `TR_Tensor(...)`.
  - **Mantık değeri sayı değildir** (3.2): `t + TR_true` sessiz dönüşüm olurdu.
  - **Onaltılık sabit:** `0x80` "int8"e sığmaz; hash işaretsiz hassasiyette yazılır, sessiz işaret değişimi olmaz.

### 15.6b

- *Emsal:* PyTorch ve NumPy aynı yayın kuralını uygular; iki taraflı genişleme sessizdir.
- *Gerekçe:* eksen sayıları farklıyken iki taraflı genişleme çoğu zaman yanlışlıkla olur (bir eksenin unutulması); sonuç verilir ama uyarı yazılır, aynı yerde bir kez. Eksen sayıları eşitken iki yanlı genişleme (nedensel maske `[T, 1] >= [1, T]`) `.TR_unsqueeze` ile bilerek yazılır; uyarı orada gürültü olurdu.

### 15.6c

- *Emsal:* Array API'nin tabloları: "u1 i1 → i2", "u4 ile i1/i2/i4/i8 → i8" [kanıtlı: aynı sayfa, tablolar]. PyTorch 2.14'te `uint8 + int8` → `int16` [ölçüldü: DENETİM B1]; `bf16 + fp16` → float32.
- *Gerekçe:* birleşme yalnız hassasiyetlere bağlıdır, değerlere değil ("Güvenilir"; 13.10: sonucun tipi değerden çıkmaz). "uint64" TR'de yok (15.1); tablonun tanımsız köşesi de yok.

### 15.6d

- *Emsal:*
  - PyTorch ve NumPy tensörle ve maskeyle indeksi verir. PyTorch'un `torch.select`'i ve `unsqueeze`'i; NumPy'de `expand_dims`.
  - Yinelenen indeks [ölçüldü ve kanıtlı: DENETİM, `torch==2.14`]: PyTorch'ta `c[idx] += 1` → `[1, 0, 0]`, `index_add_` → `[3, 0, 0]`; `index_put_`'un belgesi: "If accumulate is False, the behavior is undefined if indices contain duplicate elements". PyTorch'un `x[idx] += 1`'i yinelenenleri kaybeder; `index_add_` ve `index_put_(accumulate=True)` biriktirir.
  - Örtüşme: NumPy'ın kuralı; kaynak önce kopyalanır [ölçüldü: DENETİM, `[1, 1, 2, 3]`].
  - Bileşik atama: PyTorch'ta `TR_var q = p; p -= ...` karşılığında `q` de değişir [ölçüldü: DENETİM, 2.14]; paylaşımın tuzağı (`kalıntı`) PyTorch'ta da aynıdır.
  - Yaprak: PyTorch'un `is_leaf` kuralıyla aynıdır.
  - Türevsiz hedefe türevli değer: PyTorch hedefi çizgeye katar.
- *Gerekçe:*
  - **Tensörle ve maskeyle indeks.** Maskeyle seçim veri temizlemede (geçersiz örnekleri atmak) ve değerlendirmede (yalnız maskeli belirteçlerin kaybı), tensörle indeks örneklemede ve veri karıştırmada gerekir. Maskeyle seçimin sonucunun şekli veriye bağlıdır; şekil tipte olmadığından (15.1) 1.2'ye aykırı değildir.
  - **Çok eksenli indeks yazımı yok, `.TR_select` ve `.TR_unsqueeze` var.** PyTorch alışkanlığı `x[:, -1]`, `mask[:, None, :]`. TR'de açık uçlu aralık (`:`, `..`) yok (3.3); yazımı getirmek 2, 4 ve 5'e dokunan bir dilbilgisi kararıdır. `.TR_slice`, `.TR_select` ve `.TR_unsqueeze` yaygın çok eksenli yazımları karşılar ve ileti `x[:, -1]`'i bunlara çevirir; `.TR_unsqueeze` olmadan `mask[:, None, :]` bütün şekli bilerek (`mask.TR_reshape([B, 1, T])`) yazılıyordu, ajan `B`'yi ve `T`'yi ayrıca okumak zorundaydı. Çok eksenli indeks yol haritasında açık soru kalır.
  - **Bileşik atama yerinde.** Adı yeniden bağlayan `p -= g` sessiz bir yanlıştır: ajan `TR_no_grad { w -= lr * w.TR_grad }` yazar ve parametrenin yalnız bir adı güncellenir [ölçüldü: `[0.0, 1.0] [1.0, 2.0]`]. 5.4'ün liste istisnası tensöre de uygulanır: liste ile aynı kural, ajana ikinci bir şey öğretmez. Dilimle yazılan `r[0..2] -= ...` zaten yerindeydi; kitaplık bu yüzden dilimle yazıyordu (`lib/iyilestirici.tr:344`).
  - **Tensörle indekste biriktirme.** `t[idx] = t[idx] + d` yinelenenleri kaybederdi; gömmenin gradyanı ve sayaç bu biriktirmeyi ister. Sonuç belirlenimcidir ("Güvenilir"); öbür bileşik atamalarda yinelenenin sonucu tanımsız olacağı için hatadır. Bedel: GPU'daki gömme gradyanı tam bu işi belirlenimci yapıyor, yinelenen sınaması gruplamadan bedava [kanıtlı: DENETİM, `kernel_embed.go`].
  - **`.TR_fill`.** Görünüme `son = d` yazmak 5.4'e göre adı yeniden bağlar; `son *= 0.0` NaN ve sonsuzda NaN verir. Bütün öğelere yazmanın açık yolu gerekir.
  - **Paylaşımın tuzağı** başvuru anlamıdır; alınmadı, ileti ve rehber `h = h + f(h)`'yi gösterir.
  - **Yaprağın tanımı** PyTorch'unkiyle aynıdır: türev izlemeyen her tensör yapraktır. Kitaplığın `TR_Tensor(TR_no_grad(...), TR_requires_grad: TR_true)` dolambacı (`lib/katman.tr:20`) tanımın eksikliğini gösteriyordu.
  - **Türevin üç kuralı.** Yerinde yazma türevde yoktur; bu yüzden yazma ya türevsiz kipte ve yaprağa yapılır (iyileştirici), ya hatadır. Türevsiz hedefe türevli değer yazmak gradyan bağını sessizce koparırdı; PyTorch hedefi çizgeye katar, TR'de yerinde türev olmadığı için hata verilir. Kip sözcüksel değildir: iyileştirici yardımcısı ve `TR_custom_grad` bloktan çağrılır.

### 15.6e

- *Emsal:* C99 Ek F'nin `pow`'u (IEEE 754). PyTorch 2.14'te tam sayı tensöründe `int ** -1` hatadır [ölçüldü: DENETİM B1]. TR'nin tensöründe `(-8.0) ** 0.5` NaN'dır [ölçüldü].
- *Gerekçe:* 15.50'nin `TR_sqrt` kuralıyla aynı: tensörde öğe başına hata yok, NaN; sayıda hata (4.3). Tam sayı tensöründe eksi üs hatadır, çünkü sonucu tam sayı değildir; öğeye bakmak gerektiği için GPU'da bekler (15.6).

### 15.7

- *Emsal:* PyTorch `torch.cat`.
- *Gerekçe:* cinsler karışmaz (15.49); hassasiyet birleşmesi 15.6c'nin tek kuralıdır, ayrı bir "türler farklıysa float32" kuralı gerekmez.

### 15.8

- *Emsal:* JAX `lax.dynamic_update_slice(operand, update, start_indices)` (sınır dışını kırpar), PyTorch `Tensor.index_copy_` (transformers sabit önbelleği bununla yazar), FlashAttention `flash_attn_with_kvcache` (`cache_seqlens` konumundan yerinde ekler).
- *Gerekçe:* KV önbelleği üretimin her adımında bir dilime yerinde yazar ("Hazır modelleri çalıştırmak"); `TR_replay` kaydında başlangıç her oynatmada değişir, bu yüzden tensörle verilir ve her oynatmada okunur. JAX'tan farklı olarak sınır dışı kırpılmaz: 15.49 sessiz kırpmayı yasaklar. Türevle ilişkisi 15.6d'nin kurallarıdır: yerinde yazmanın tek kural kümesi vardır.

### 15.9

- *Emsal:* PyTorch tamamen −∞ satırın softmax'ında NaN verir.
- *Gerekçe:* öğe öğe matematik yalnız ondalıkta anlamlıdır; tam sayıyı sessizce ondalığa çevirmek cinslerin karışması olurdu (15.6). Tamamen maskeli satırın kuralı dikkatteki boş satırla tutarlıdır (15.24); NaN içeren satır NaN kalır: NaN gizlenmez, maskeli satırla karışmaz.

### 15.10

- *Emsal:* yok: doğruluk sınırı TR'nin sözüdür.
- *Gerekçe:* "Güvenilir": sonucun doğruluğu yazılıdır; 1 ulp sınırı CPU ile GPU'nun karşılaştırılmasına ölçüt verir.

### 15.11

- *Emsal:* yok: işlevler C'nin matematik kitaplığındaki işlevlerdir (`log1p`, `expm1`); `TR_silu` kapılı ileri beslemeli katmanların (SwiGLU) temelidir.
- *Gerekçe:* `TR_log1p` ve `TR_expm1` küçük x'te kayıpsız sonuç verir: `TR_log(1 + x)` ve `TR_exp(x) - 1` float32'de 0 verir. Doğruluk sınırları yazılıdır ("Güvenilir").

### 15.12

- *Emsal:* PyTorch `sign(−0)` +0, `sign(NaN)` 0 verir; NumPy +0 ve NaN.
- *Gerekçe:* her x için (−0 dahil) `TR_sign(x)·TR_abs(x) = x`; NaN gizlenmez. Tam sayı tensöründe `TR_floor` ve `TR_ceil` bir şey yapmaz (13.7 ile aynı).

### 15.13

- *Emsal:* PyTorch sınırı tensör olarak da alır; PyTorch'ta sınırdaki türev 0'dır.
- *Gerekçe:* sayı sınırda sınırdaki değer aralığın içindedir, türevi 1'dir. Tensör sınırda `TR_clamp(x, TR_min: a) ≡ TR_maximum(x, a)` özdeşliği türevde de korunur; PyTorch da tensör sınırda `maximum` gibi davranır [ölçüldü: DENETİM, `torch==2.14`, eşitlikte 0,5/0,5]. Tensör sınırdaki NaN öğe yayılır: sınırları öğe öğe NaN için sınamak her çağrıda bekleme olurdu (15.6). `TR_max(x, 0)` indirgemedir; öğe öğe sınırla karışmasın diye ileti `TR_clamp`'i önerir.

### 15.14

- *Emsal:* PyTorch `atan2`'nin türevi bir girdi sonsuzken NaN verir.
- *Gerekçe:* matematiksel limit: sonsuzda türev 0'dır; (0, 0)'da tanımsız türev 0 alınır.

### 15.15

- *Emsal:* PyTorch 2.14'te `torch.where` ondalık koşulda hatadır [ölçüldü: DENETİM B1]. Seçilmeyen dalın sonsuz türevinin NaN vermesi PyTorch'ta da böyledir; çift `where` ile iç maskeleme `[0.0, 0.5]` verir [ölçüldü: DENETİM].
- *Gerekçe:* koşul mantıktır, çünkü mantık ile sayı arasında dönüşüm yoktur (15.3): "sıfırdan farklı" koşulu NaN'ı da doğru sayıyordu [ölçüldü]. Hiçbiri tensör değilse işin adı `TR_if`'tir; sayı imzası ikinci bir yol olurdu. Türev kuralı açıkça yazılır, çünkü `TR_where(x > 0.0, TR_log(x), 0.0)` sessizce NaN gradyan verir; doğru yazım iç maskelemedir.

### 15.16

- *Emsal:* PyTorch'ta `relu`(−0) = −0; `abs`'ın türevi NaN'da 0'dır.
- *Gerekçe:* işaret bitinin silinmesi IEEE'dir ve CPU ile GPU aynı bitleri verir ("Güvenilir"). Tam sayıda en küçük değerin mutlak değeri sığmaz; 15.3'ün taşma kuralı uygulanır.

### 15.16a

- *Emsal:* PyTorch'un `maximum`'u ±0'da ilk argümanı verir, sonuç argüman sırasına bağlı kalır [ölçüldü: DENETİM]; tam eşitlikte türevi yarı yarıya böler [ölçüldü: DENETİM, 0.5/0.5]. Python'un `round`'u tam sayı, PyTorch'un `torch.round`'u tensörün türünü verir.
- *Gerekçe:* `TR_maximum`'un sonucu argüman sırasından bağımsızdır ("Güvenilir"; sıralama sırası 15.22b). `TR_round` tensörde cinsi korur (`TR_floor` gibi); tam sayıya geçiş açık cins dönüşümüdür. `TR_is_nan` ve `TR_is_finite` okunur; `t != t` de geçerli kalır.

### 15.17

- *Emsal:* PyTorch 2.14: "approximate (str, optional) ... Default: 'none'" (tam) [kanıtlı: docs.pytorch.org/docs/2.14/generated/torch.nn.GELU.html]; JAX `jax.nn.gelu(x, approximate=True)` (tanh) [kanıtlı: docs.jax.dev, latest]; TensorFlow/Keras `gelu(approximate=False)` (tam). İki ekosistem ters. GPU'da PTX'te hazır erf komutu olmadığından Φ(x) yaklaşımla hesaplanır.
- *Gerekçe:*
  - Hangi varsayılan seçilse bir ekosistemden gelen ajan sessizce yanlış biçimi alır (fark ~5e-4, model taşırken çıktı kayar; "Paylaşmak", "Hazır modelleri çalıştırmak"). Zorunlu argüman bu sessiz yanlışı denetime çevirir. Reddedilen yol: tanh varsayılanı; gerekçesi eğitilmiş kontrol noktalarının davranışını korumaktı, geçmişe dayanan bir gerekçedir.
  - Mantık değerli adlı argüman (JAX ve TensorFlow gibi): PyTorch'un `"none"`/`"tanh"` metni yazım hatasına açıktır; TR'de `TR_causal:`, `TR_keepdim:` gibi seçenekler de mantıktır. Yalnız adla verilir: `TR_gelu(x, TR_true)` okunduğunda anlamı görünmez.
  - Tam biçim `1 + erf` yerine `erfc` ile hesaplanır: x çok negatifken göreli doğruluk korunur.

### 15.18

- *Emsal:* Array API 2025.12 `sum`: işaretli tam sayıda "the returned array must have the default integer data type" [kanıtlı: data-apis.org/array-api/latest/API_specification/generated/array_api.sum.html]; işaretsizde "uint64" der, TR'de işaretsiz yok. PyTorch'un `torch.all`'u uint8 dışındaki her türde bool, uint8'de uint8 verir [kanıtlı: docs.pytorch.org/docs/2.14/generated/torch.all.html]. PyTorch'ta `torch.max(x, 1)` eksen boyunca (değerler, indeksler) demeti verir; yalnız değer veren biçim `amax`'tır. PyTorch `x.mean(dim=(1, 2))` yazar (görüntüde yükseklik ve genişlik, havuzlama). Boş eksen listesinde ekosistemler terstir: PyTorch `dim=()` hepsini, NumPy `axis=()` hiçbirini indirger [ölçüldü: DENETİM C1]. 16 bitlik toplam taştığında PyTorch ve NumPy da `inf` verir [ölçüldü: DENETİM].
- *Gerekçe:*
  - **Eksen yalnız adla.** `TR_max`'ın iki konumlu biçimi 13.1'de "iki değerin en büyüğü"dür (`TR_max(2, 1.5)`); PyTorch'ta `torch.max(x, 1)` eksen boyunca demet verir. Ajan `TR_max(x, 1)`'i üç anlamda okuyabilir. Aynı ailede iki kural da olmaz: tek kural, eksen hep `TR_dim:` ("Hata riski az").
  - **Sonucun cinsi tabloda.** Dinamik türün istisnaları (PyTorch'un uint8'i) ajanın hata kaynağıdır; TR'nin tablosunda istisna yok. Maskenin toplamı sayımdır (`TR_Int`), ortalaması orandır (`TR_Float`): doğruluk oranı tek çağrıyla, kesin sayımdan tek yuvarlamayla çıkar (15.3'ün tek kuralı).
  - **Tam sayı toplamının taşması matematiksel toplamla.** Ara toplam sayılmaz, böylece sonuç sıradan ve cihazdan bağımsızdır (Güvenilir); taşma sessiz sarma değil hatadır (15.3'ün tam sayı taşma kuralı).
  - **`TR_std` iki geçişli.** Tek geçişli `Σx² − n·x̄²` float32'de eksi varyans ve NaN verir.
  - **Eksik indirgemeler.** `TR_std` veri hazırlamada normalleştirme ve ölçmede yayılım için; `TR_logsumexp` ölçmede şaşkınlık (perplexity), elle yazılan kayıp ve uzmanlar karışımında yönlendirme için gerekir.
  - **Birden çok eksen.** 13.10: "Dönüş tipi bütün parçalar için aynıysa bir parametre birkaç tip kabul edebilir ... değilse ayrı imzadır" [kanıtlı: `13_yerlesikler.md:86-87`, DENETİM C1]. Dönüş aynı olduğu için tek imza; `TR_dim:`'in kabul kümesi `TR_Int` ya da `TR_List[TR_Int]`. İç içe çağrı ortalamada doğrudur ama `TR_std`'de yanlıştır; ajanı ona itmemek gerekir. Boş liste ekosistemlerde ters anlamlı olduğu için hatadır. Reddedilen yol: iki imza (1. tur; yanlış araçtı).
  - **`TR_max` eksenle yalnız değer.** Demet dönüşü ile değer dönüşü aynı adda olursa ajan karıştırır; indeks `TR_argmax`'tır.
  - **Dokuz iş:** veri hazırlamada normalleştirme ve `TR_sum(maske)` sayımı; eğitimde kayıp ortalaması ve elle LayerNorm (`TR_mean`, `TR_keepdim`); ölçmede doğruluk oranı ve şaşkınlık; uyarlamada nicemleme ölçeği (`TR_max(TR_abs(w))`).
  - **Boş eksen ile boş tensör ayrı.** Sıfır öğeli indirgeme tanımsızdır; `[0, 4]`'te `TR_dim: 1` her satırda dört öğe indirger ve boş sonuç verir. PyTorch da `mean(dim=1)`'i `[]` verir, boş eksende `amax`'ı ve `argmax`'ı hata verir [ölçüldü: DENETİM, `torch==2.14`]. Boş tensör listesinde sonucun şekli bilinmez. `TR_std`'nin sıfır sapmadaki türevi 0'dır, PyTorch'ta da [ölçüldü].

### 15.18a

- *Emsal:* `torch.equal`: "True if two tensors have the same size and elements ... Tensors containing NaNs are never equal to each other" [kanıtlı: docs.pytorch.org/docs/2.14/generated/torch.equal.html]; TR aynı, yalnız cinsler aynı olmalı. PyTorch'un `if torch.all(x):`'i örtük çeviriyle çalışır. Python'un `math.isclose`'u simetriktir; `torch.allclose` ve `np.allclose` yayınlar ve sabit tolerans kullanır (1e-5, 1e-8); `torch.testing.assert_close` hassasiyete göre (float32 1.3e-6 / 1e-5, bf16 1.6e-2 / 1e-5) [ölçüldü: DENETİM C1]. fp16 (1e-3 / 1e-5) ve float64 (1e-7 / 1e-7) `torch.testing._comparison.default_tolerances` ile birebir aynıdır [ölçüldü: DENETİM, AB-gemini sonucu].
- *Gerekçe:*
  - 3.10 tensörü koşulda kabul etmez; bütün tensör sorusu koşula girecekse `TR_Bool` vermelidir. TR'de örtük çeviri yok.
  - Örnek bilerek `TR_all(t >= 0.0)`'dır: `TR_all(a == b)` yayınlı öğe eşitliğidir ve şekli farklı iki tensörde doğru çıkabilir; iki tensörün eşitliği `TR_equal`'dır (15.5).
  - Mantık ile sayı arasında dönüşüm yoktur (15.3): ondalık ya da tam sayı tensör `TR_all`'a verilmez, ileti `TR_all(t != 0)`'ı önerir.
  - **`TR_all_close`'un varsayılanı hassasiyete göre.** Sabit `rtol: 1e-5` bf16'nın bir ulp'inden (0.0078) çok küçüktür; bf16'da `1.0` ile `1.004` hep "değil" çıkar [ölçüldü: DENETİM C1, PyTorch `allclose` aynısını yapıyor]. Ekosistemler ayrıldığında yazımı açık tutma ilkesi burada zorunlu yazım isterdi; ama zorunlu yazım ajanı her çağrıda sayı uydurmaya iter. Hassasiyete göre varsayılan doğru sonucu verir ve yazım gerektirmez; tablo maddede yazılı.
  - Şekil farkında yayın yok: sınamanın sorusu "aynı mı"dır ("Hata riski az").
  - Bedel: GPU'da konağa okuma. Sıcak döngüde ajanın bilmesi gerekir; madde bunu yazar. Eksenli `TR_all`/`TR_any` beklemez.
  - **Sonsuzda yakınlık.** Formül IEEE'de tersini verir: |0 − ∞| = ∞ ≤ ∞ yakın, |−∞ − (−∞)| = NaN uzak. NumPy ve PyTorch'un `isclose`'u sonsuzu ayrı ele alır, aynı işaretli sonsuzu yakın, sonlu ile sonsuzu uzak sayar [ölçüldü: DENETİM, `numpy==2.1.1`, `torch==2.14`]; TR de.

### 15.19

- *Emsal:* PyTorch ve NumPy eksensiz argmax'te düzleştirir [ölçüldü: DENETİM C1]. Eksensiz biçimde `keepdim` verildiğinde PyTorch ve NumPy ikisi de `(1, 1)` şekli verir [ölçüldü: DENETİM]. PyTorch ve NumPy `argmax([-0.0, 0.0])` için 0 verir [ölçüldü: DENETİM C1]. NaN'lı tensörde `argmin(t)` ile `argsort(t)[0]` PyTorch'ta da ayrılır [ölçüldü: DENETİM].
- *Gerekçe:*
  - **Eksensiz bütün öğeler.** Eski kuralda eksensiz argmax son eksendeydi; PyTorch'tan gelen ajan 2 boyutlu tensörde sessizce yanlış indeks alırdı. 15.18'in "eksensiz bütün öğeler" kuralıyla da aynı: tek kural.
  - **Eksensizde `TR_keepdim:` yok:** düzleştirilmiş indeks hiçbir eksenin indeksi değildir.
  - **−0.0 0.0'dan küçük:** sıralama sırası tektir (15.22b); TR burada ekosistemden bilerek ayrılır.
  - `TR_argmin` eksikti: en yakın komşu ve nicemlemede kod kitabı için gerekir. Açgözlü kod çözme `TR_argmax`'tır (Hazır modelleri çalıştırmak, Sunmak).

### 15.20

- *Emsal:* PyTorch `cumsum`'da `dim` zorunludur, NumPy eksensizde düzleştirir [ölçüldü: DENETİM C1].
- *Gerekçe:* ekosistemler ayrıldığında yazım açık olur: varsayılan eksen PyTorch'tan ya da NumPy'den gelen ajanlardan birini sessizce yanıltır. `TR_topk` ve `TR_argsort`'un −1 varsayılanı ekosistemlerde aynı olduğu için kalır [ölçüldü: DENETİM C1]. Cins ve taşma kuralı `TR_sum`'ınkidir: tek kural. Top-p örneklemede olasılıkların birikimli toplamı için gerekir (Hazır modelleri çalıştırmak).

### 15.21

- *Emsal:* PyTorch `topk`: "A namedtuple of (values, indices)" [kanıtlı: docs.pytorch.org/docs/2.14/generated/torch.topk.html].
- *Gerekçe:* değerler ve indeksler farklı cinstendir (cins tipte, 15.1); liste tek tiplidir (3.6), demet tipleri çalışmadan önce taşır (3.6a). Sıra sıralama sırasıdır (15.22b); böylece `TR_topk(t, 1)` ile `TR_max(t)` aynı öğeyi verir. Metin üretmede top-k örnekleme ve sunucunun örneklemesi için gerekir.

### 15.21a

- *Emsal:* biçim PyTorch'un `take_along_dim`'i ve NumPy'ın `take_along_axis`'idir (ikisi de yayınlar). PyTorch'un `gather`'ı yayınlamaz ve kendisinden kısa indeksi sessizce kabul eder (`[1, 2]` indeks ilk satırı verir) [ölçüldü: DENETİM]; o biçim alınmadı.
- *Gerekçe:* iki gerçek kullanım: `TR_topk`'un indeksini sözlüğe çevirmek ve kayıpta hedef dizgeciğin log-olasılığı. Top-p örnekleme `TR_gather` ile birkaç satırda yazılır; ayrı bir `TR_top_p:` argümanı tek kullanımlı olurdu (yol haritası). Sessiz kabul yerine yayın ve sınır dışı hatası ("Hata riski az"). `TR_dim:` zorunlu ve yalnız adla: 13.10'un varsayılansız adlı argüman kuralı.

### 15.22

- *Emsal:* kararlı azalan sıra PyTorch'un `stable=True`'su gibidir. NumPy'de `argsort(x)[::-1]` eşitleri de çevirir: `[3, 1, 2, 0]`, kararlı azalan ise `[1, 3, 0, 2]` [ölçüldü: DENETİM C1]. Motorda azalan sıralama hazırdır (`Backend.Sort(..., desc)`) [kanıtlı: DENETİM C1].
- *Gerekçe:* metin üretmede top-p örnekleme olasılıkları büyükten küçüğe dizer. `TR_topk(t, n)` de dizer ama eşitlerde ve NaN'da kuralı ayrı yazmak gerekirdi; tek fonksiyon daha açık. `.TR_reverse()` eşitlerin sırasını da çevirir; kararlı azalan sıra `TR_descending: TR_true`'dur. Kural eksen uzunluğuna sınır koymaz; GPU'nun bugünkü 64 bitlik sıralama anahtarının koyduğu 2³² − 1 uygulamanın sınırıdır (status), dilin değil. `TR_descending` yeni kimlik adıdır (kimlik adları dil kararıdır).

### 15.22a

- *Emsal:* yok: taslakta başka araçla karşılaştırılmadı.
- *Gerekçe:* CLAUDE.md: "**Güvenilir.** Aynı program aynı veriyle aynı sonucu verir; kesilen bir eğitim kaldığı yerden aynı sonuçla sürer." Söz verilebilen yerde söz verilir: aynı cihaz türünde aynı bit, toplama sırası yalnız şekle bağlı. Söz verilemeyen yerde sınır açıkça yazılır: CPU ile GPU'nun ondalık biriktirmesi ayrıdır, SM'ye bağlı yollar kart modeline göre ayrılır. Robotta (Makinenin içinde çalıştırmak) aynı cihazda yeniden üretilebilir davranış için gerekir; kesilen eğitimin aynı sonuçla sürmesi de buna dayanır.

### 15.22b

- *Emsal:* Java'nın `Double.compare`'ı: −0 < +0, NaN en büyük; `<` işleci ise IEEE [kanıtlı: DENETİM C1, belge açıldı]. Array API 2025.12 `max`: "The order of signed zeros is unspecified and thus implementation-defined" [kanıtlı: data-apis.org/.../array_api.max.html]. PyTorch ve NumPy'nin sıralaması −0 ile +0'ı eşit tutar, `argmax([-0.0, 0.0])` 0 verir [ölçüldü: DENETİM C1]; ekosistemlerin `max`'ı −0'da girdinin sırasına ve yola göre tutarsızdır [ölçüldü: DENETİM C1].
- *Gerekçe:*
  - Eskiden üç kural vardı: 13.1 listede "−0.0 küçük", tensörde `TR_max` "eşitlerde ilk", 13.3'ün sıralaması "eşit, sıra korunur". Tek kural: sıralayan her işte −0.0 < 0.0, NaN en büyük; adı "sıralama sırası". Ekosistemden açıkça ayrılır; tek kural Güvenilir'e uyar.
  - `==` (4.5) ve `TR_equal` −0.0'ı 0.0'a eşit sayar; sıralama ile eşitlik ayrı sorulardır.
  - **NaN'da sıralama ile indirgeme bilerek ayrı:** `TR_max`/`TR_min` NaN'ı yayar (NaN gizlenmez), `TR_argmax`/`TR_argmin` ilk NaN'ı seçer, `TR_topk`/`TR_argsort` sıralama sırasını bütünüyle kullanır.
  - Tanım 13.10'dadır: 13, 15'ten önce gelir; liste kuralı tensör bölümünde tanımlanmamalı. 15.22b yalnız gönderir ve tensördeki kullanıcılarını sayar.

### 15.23

- *Emsal:* yok: madde taslakta ayrıca tartılmadı (eski kuralı yeni cinslere taşır).
- *Gerekçe:* indeks cinsi `TR_Int`'tir (cins tipte, 15.1); ondalık indeks olmaz. Aynı satıra düşen gradyanlar belirlenimci biriktirilir (Güvenilir, 15.22a). Sınır dışı indeks sessizce kırpılmaz, hatadır ("Hata riski az").

### 15.23a

- *Emsal:* `-100` PyTorch'un `ignore_index` varsayılanı ve Hugging Face'in etiket dolgusudur [kanıtsız: bu tur açılmadı]; PyTorch'ta da `-5` hatadır [ölçüldü: DENETİM]. Bütün hedefler yok sayılınca PyTorch NaN verir [ölçüldü: DENETİM]; TR daha sıkıdır. PyTorch'un `cross_entropy`'si de olasılık hedefi alır [kanıtsız: bu tur açılmadı].
- *Gerekçe:*
  - **`-100` ve yok sayılmayan negatifler.** Eski kural her negatif hedefi sessizce yok sayıyordu. Bir veri hatası (yanlış kaydırma, işaret hatası) kaybı sessizce küçültür. Tek bir değer yok sayılır, ötekiler hatadır. `-100` dışarıdan gelen etiketli verinin ve ajanın bildiği kodun değeridir.
  - **Olasılık hedefi.** Damıtma ("büyük bir modelin bilgisini küçüğüne aktarmak") ve etiket yumuşatma olasılık hedefi ister; ayrı bir yerleşik yerine hedefin cinsi biçimi seçer (cins tipte, imzalar ayrık, 13.10). Olasılık hedefinde satır toplamı ve işaret denetlenmez: ondalık işlem beklemez (15.6).
  - Hedef denetimleri öğeye bağlıdır, bu yüzden bekleyen işlemlerdir (15.6).

### 15.23b

- *Emsal:* yok: grup normu için taslakta karşılaştırma yapılmadı. Normun çıktı türünün emsali 15.4'tedir.
- *Gerekçe:* CLAUDE.md görüntü, ses ve video modellerini ve "görüntü üretmek"i ister; grup normu yayılım modellerinin ve evrişimli ağların normudur. `TR_num_groups:` varsayılansızdır, bu yüzden zorunludur ve yalnız adla verilir (13.10).

### 15.23c

- *Emsal:* PyTorch `conv1d`/`conv2d` (`stride`, `padding`, `dilation`, `groups`), JAX `lax.conv_general_dilated` [kanıtsız: bu tur açılmadı].
- *Gerekçe:* CLAUDE.md görüntü, ses ve video modellerini ve "görüntüyü tanımak, içindeki nesneleri bulmak, görüntü üretmek"i ister. ViT'nin parça gömmesi, Whisper'ın girişi, yayılım modelinin U-Net'i ve nesne bulma evrişim ister. Kanal önde düzen PyTorch'un ve ONNX'in düzenidir: hazır ağırlıklar dönüştürülmeden alınır (Paylaşmak). Grup sayısının adı (`TR_num_groups`) iyileştiricinin `.TR_groups`'undan ayrıdır: bir ad bir anlam (17.5, 2.9). Havuzlama ve büyütme, ters evrişim ve `TR_pad` yol haritasındadır; örtüşmeyen havuzlama bugün `.TR_reshape` ve `TR_max`/`TR_mean` ile yazılır.

### 15.23d

- *Emsal:* PyTorch'un `dropout`'u tohum almaz; oran 0'da sonuç kopyasız girdinin kendisidir [ölçüldü: DENETİM]. PyTorch'ta eğitim ile çıkarım `model.train()`/`model.eval()` bayrağıyla ayrılır.
- *Gerekçe:* dropout'un maskesi TR koduyla yazılabilir, ama paylaşılan üreteç kuralı (Güvenilir: kesilen eğitim kaldığı yerden aynı maskeyle sürer, 20.17) ve türevin maskeyi paylaşması tek yerde doğru yazılmalıdır. Sabit tohum her adımda aynı maskeyi verip düzenlemeyi sessizce bozardı. Kip bayrağı ajanın unuttuğu bir durumdur; TR'de dropout'u çağırmak ya da çağırmamak programın açık kararıdır.

### 15.24

- *Emsal:* JAX `jax.nn.dot_product_attention` query'yi `(B, T, N, H)`, maskeyi `(B, N, T, S)` alır; FlashAttention `flash_attn_func` `(batch_size, seqlen, nheads, headdim)` ve KV önbelleği `(batch_size_cache, seqlen_cache, nheads_k, headdim)` düzenini kullanır, son eksen dışındaki adımları okur ve gradyanları girdinin düzeninde ayırır. PyTorch `scaled_dot_product_attention` `(N, ..., Hq, L, E)` alır ve flash yolunda içeride `query.transpose(1, 2)` ile çevirir; transformers çıktıda `transpose(1, 2).contiguous()` ile kopyalar. Kaynaklar: flash-attention `flash_attn_interface.py` (main); JAX belgesi `dot_product_attention`; PyTorch 2.14 belgesi ve `aten/src/ATen/native/transformers/cuda/attention.cu` (eski 15.24'ün metninden). PyTorch'un `scaled_dot_product_attention`'ı ondalık maskeyi puanlara ekler [ölçüldü: DENETİM].
- *Gerekçe:*
  - **Düzen.** TR'de gradyan depolaması mantıksal şekilde bitişik olduğundan bakış önde düzen her katmanda ileri yönde iki, geri yönde dört kopya isterdi; JAX ve FlashAttention'ın düzeni hepsini kaldırır (hedef modelde kopyalar GPU süresinin %3,5'iydi; eski 15.24'ün metninden, etiketsiz).
  - **Maske mantık tensörü.** Cins tipte; mantık ile sayı arasında dönüşüm yok (15.3). 0/1 sayı maskesi denetim hatasıdır.
  - **Maske ve sapma ayrı argüman.** PyTorch'un ondalık maskesi iki işi tek argümanda karıştırır. TR'de `TR_bias` eklenen terimdir (T5'in göreli konum sapması, ALiBi; var olan ad, aynı anlam); 0 çıktı yalnız maskeden gelir. Önce maske: bakılmayan yerdeki sapma okunmaz.
  - **Sabit kapasiteli önbellek.** `TR_replay`'in sabit şeklinde `Tk` kapasitedir; sondan hizalı nedensellik orada yanlış konuma bakar. Doğru yazım maskeyle yazılır; yazılmamış yuvaya bakmak hata vermediği için madde bunu açıkça söyler.
  - **Hassasiyet** 15.6c'nin birleşme kuralıyladır: dört girdi için ayrı bir kural yok.
  - Etkinleştirme kaydı (`TR_recompute`) ve ikinci türev yol haritasındadır.

### 15.25

- *Emsal:* PyTorch'ta hazır işlem yoktur; hedef modelin PyTorch kodu (`tools/gpu/rgn512.py`, `lm_loss`) dilimleri `checkpoint` ile geri yayılımda yeniden hesaplar (dilim başına bir ileri çarpım fazla). Liger Kernel'in `LigerFusedLinearCrossEntropyLoss`'u gradyanı ileri hesapta dilim dilim çıkarır; Apple'ın Cut Cross-Entropy'si (`linear_cross_entropy`) logitleri hiç yazmayan özel kerneller kullanır; JAX/Optax'ta hazır karşılık yoktur (eski 15.25'in metninden).
- *Gerekçe:* 32 bin parça × 40 bin sınıfta logitler ve gradyanları birkaç GB eder. TR Liger'in yöntemini seçer: ek hesap yoktur ve var olan çarpım ve çapraz entropi kernelleriyle yazıldığından her cihazda çalışır. Ad, Apple'ın adının ve TR'deki `TR_Linear` + `TR_cross_entropy` adlarının birleşimidir. Dilim boyu kullanıcı seçeneği değildir (gerekirse sonra eklenir). Hedef kuralları 15.23a'nınkidir: tek kural.

### 15.26

- *Emsal:* yok: taslakta karşılaştırma yapılmadı.
- *Gerekçe:* "Hazır modelleri çalıştırmak ... metin üretmek" ve Güvenilir: aynı tohum aynı sonucu verir, tohumsuz çağrı paylaşılan üreteçten çeker (20.17), kesilen iş aynı sonuçla sürer. Sözün sınırı (aynı cihaz türü, FMA) açıkça yazılır; son basamaktaki fark örneklenen indeksi değiştirebilir. Dağılım kurulamayan satırda sessizce bir indeks seçmek yanlışı gizlerdi ("Hata riski az"). Sıcaklık 0 eksenli argmax'tir; eksensiz argmax düzleştirir (15.19). Top-p için ayrı argüman yerine `TR_gather` (15.21a).

### 15.27

- *Emsal:* PyTorch'un varsayılanı da çizgeyi bir kez geri yaymaktır (`retain_graph=False`) [kanıtsız: taslakta etiketsiz]. Görevin açıldığı yerin kipini alması Python'un `contextvars`'ı gibidir.
- *Gerekçe:*
  - Çizge bir kez geri yayılır: ara sonuçlar bırakılır (bellek). İkinci türev ve `retain_graph` yol haritasındadır.
  - **Yeni görev açıldığı yerin kipini ve hesap türünü alır.** `TR_no_grad { TR_async üret(...) }` yazan ajanın beklediği budur. Eski metin "yeni görev türevli kipte başlar" diyordu; bu karar bilerek bir davranış değişikliğidir. Hesap türü için aynı miras 15.4'tedir.
  - `TR_no_grad` bloğundaki `TR_defer` kipin içinde çalışır: blok kapanırken yapılan temizlik de türevsizdir.
  - `.TR_grad` canlı takma addır, sürüm artışı eski değerle kurulmuş hesabı yakalar (15.3).

### 15.27a

- *Emsal:* PyTorch'un `detach()`'ı aynı adı ve aynı veri paylaşımını taşır [kanıtsız: taslakta etiketsiz].
- *Gerekçe:* `TR_no_grad` hem kipi hem ayırmayı anlatıyordu; bir ad iki iş yapmaz (2.9). Ayrılmış görünümün depo paylaşımı sessiz bir yanlış açıyordu: görünüme yazmak türev izleyen tensörü değiştiriyordu [ölçüldü]. Yazma kuralları depoya bağlanınca kapanır.

### 15.28

- *Emsal:* JAX'ın `custom_vjp`'si de ileri hesapta `(çıktı, artıklar)` demetini döndürür [kanıtsız: bu tur açılmadı].
- *Gerekçe:* eski biçimde `ileri` `[çıktı, [saklananlar]]` listesini döndürüyordu: farklı tipli öğeler bir listede (3.6'nın tek tipli listesine aykırı). Demet (3.6a) tipleri çalışmadan önce taşır (1.2). Dönüşün tipi `TR_BuiltinFunction` değil, `ileri`'nin parametreleri ve `çıktı`'nın tipiyle bir fonksiyondur: tip denetimi çağrıyı görür (13.10). "Model yazmak ve eğitmek": yeni bir işlemin geri hesabını programın kendisi yazar.

### 15.29

- *Emsal:* 15.28'deki gibi (JAX `custom_vjp`).
- *Emsal (geri yönün hesap türü):* PyTorch'un `custom_bwd`'i geri yönü ileri geçişin autocast durumuyla çalıştırır: "same autocast state as forward" [kanıtlı: PyTorch 2.15 belgesi, Gemini Koşu 1, DENETİM].
- *Gerekçe:* tek girdide kendisi, birden çokta demet: 3.6a'da tek öğeli demet ve demet indeksi yoktur. Girdi sayısı `ileri`'nin parametre sayısıdır; fonksiyon ifadesinde öğe sayısı uyuşmazlığı denetim hatasıdır (tipler çalışmadan önce bilinir). `g` `geri`'nindir: yerinde yazmak başka bir gradyanı bozmaz, ajanın kopya alması gerekmez. Yayın yapılmaz: yanlış şekilli gradyan sessizce genişletilmez ("Hata riski az").

### 15.30

- *Emsal:* yok: taslakta karşılaştırma yapılmadı.
- *Gerekçe:* kayba bağlı olmayan çıktının gradyanı `TR_none` değil sıfır tensördür; `geri` denetim yapmadan aritmetik kullanabilir, ajanın unutacağı bir dal yoktur. Çıktılar tek düğüme bağlıdır: `geri` bir kez çalışır, çizge bir kez geri yayılır (15.27).

### 15.31

- *Emsal:* yok: taslakta karşılaştırma yapılmadı.
- *Gerekçe:* "Hata riski az": ileti yanlışı ve doğru biçimi söyler. Fonksiyon ifadesi verildiğinde tipler çalışmadan önce bilindiği için hata denetimde yakalanır; değişkenle gelen fonksiyonda çalışırken.

### 15.32

- *Emsal:* yok: taslakta karşılaştırma yapılmadı.
- *Gerekçe:* `TR_grad_check` bir doğrulamadır: uyuşmazlıkta hata verir, uyuşunca söyleyecek bir şeyi yoktur. 13.10'un "anlamsız dönüş yoktur" kuralı; eski biçimde `TR_true` döndürüyordu. Denetim girdilerin float32 kopyalarıyla yapılır: programın tensörleri ve gradyanları değişmez. Adım float32'de etkisizse sayısal gradyan 0/0 olurdu; sessiz yanlış yerine hata. "Ölçmek ve değerlendirmek" ve özel türev (15.28) için gerekir.

### 15.33

- *Emsal:* PyTorch'un `parameters()`'ı donmuş (`requires_grad=False`) parametreleri de verir; iyileştiriciye bütün parametreler verilir, gradyanı olmayan atlanır [kanıtlı: DENETİM TUM-tur]. `named_parameters()` yol adlı sözlüğün karşılığıdır.
- *Gerekçe:* "Model yazmak ve eğitmek", "Uyarlamak" (ince ayarda dondurma) ve "Güvenilir" (kaydedilen her şey geri yüklenir). İki ad 13.1'in kararıdır: eğitilecekler (`TR_parameters`) ile kaydedilecekler (`TR_named_parameters`) ayrı işlerdir; bir `TR_named:` argümanı tek adın iki dönüş tipi demekti. `TR_parameters` donmuş parametreleri de verir: yalnız eğitilecekleri verseydi donmuşken kurulan iyileştirici sonradan çözülen gövdeyi tutmaz, `TR_for p TR_in TR_parameters(m.gövde) { p.TR_requires_grad = TR_true }` de hiçbir şeyi çözmezdi [kanıtlı: DENETİM TUM-tur]. Parametre "türev izlemek için kurulmuş ya da açılmış yaprak" diye tanımlandı: hiç türev izlememiş tablo ve tampon parametre değildir, böylece çözme döngüsü bir RoPE tablosunu eğitime açmaz. `TR_named_parameters` bütün yaprakları verir: donmuş gövde de kaydedilip yüklenmeli, yoksa ince ayar sürdürülemez. Yapıdaki türevli ara sonuç sessizce atlanmaz, hata verir (Gemini'nin bütün 15 raporu, 1. bulgu; DENETİM doğruladı): atlansaydı o ağırlık hiç eğitilmez ve kaydedilmezdi, "Hata riski az"ın karşıtı.

### 15.34

- *Emsal:* PyTorch'un `requires_grad_(False)`'ı `.grad`'ı silmez ve AdamW eski gradyanla yine günceller [ölçüldü: DENETİM]; TR dondururken gradyanı siler. PyTorch'ta parametre grupları `param_groups` ile tek iyileştiricide yazılır; TR'nin kitaplığı grupları birden çok iyileştiriciyle yazıyor [kanıtlı: DENETİM E1].
- *Gerekçe:* "Model yazmak ve eğitmek" ve "Uyarlamak".
  - **Kendi iyileştiricisi bileşik atamayla:** 15.6d bileşik atamayı tensörde yerinde yaptı; `p[0..n] -= ...` dolambacı ve 0 boyutun dışarıda kalması kalkar. Yerel kopya, alan daraltılmadığı için gerekir (3.5).
  - **16 bitlik uyarı:** bf16'nın bir ulp'inin altındaki güncelleme sessizce yutulur; ana kopyasız kendi iyileştiricisi bunu fark etmeden eğitimi durdurur. Uyarı motorun verebileceği biçimdedir: türevsiz kipte 16 bitlik yaprağa bileşik atama.
  - **Gradyansız adım hatası:** iyileştirici kurulduktan sonra taşınan ya da değiştirilen yaprak (15.39) iyileştiricide eski yaprak olarak kalır; adım hiçbir şey yapmadan geçer, model sessizce eğitilmez. "Son adımdan bu yana" ve sıfırlamanın sayılmaması, canlı takma adın (15.27) her zaman var olmasındandır.
  - **İyileştirici donmuş yaprağı kabul eder ve atlar:** ince ayarda ajanın en sık yanlışı dondurmak istediği ağırlığı güncellemektir; TR dondururken gradyanı sildiği için donmuş yaprak iyileştiricide hiç güncellenmez. Kurulumda hata vermek (önceki karar) aşamalı çözmeyi bozuyordu: donmuşken kurulan iyileştirici sonradan çözülen gövdeyi tutmaz, gövdenin gradyanı birikir, gövde sessizce eğitilmez [kanıtlı: Gemini TUM 3, DENETİM]. PyTorch'ta da bütün parametreler verilir, gradyanı olmayan atlanır.
  - **Parametre grupları birden çok iyileştiriciyle** (DENETİM E1'in hükmü): yeni özellik gerekmez; PyTorch'un `param_groups`'u reddedildi.
  - **İyileştirici durumu parametrenin cihazında** (DENETİM E1'in hükmü): 20.6'nın yüklemesiyle aynı yer.
  - **Donma durumu kontrol noktasına girer:** donma bir değere bağlıysa (kayıp eşiği), sürdürme başka durumla başlayıp 20.17'yi bozardı ("Güvenilir").
  - `TR_load_tensors`'ın varsayılanı parametre değildir: dosyadan okunan tablo sessizce eğitime açılmasın.

### 15.35

- *Emsal:* PyTorch'ta oran `optimizer.param_groups[i]["lr"]`'ye yazılarak değiştirilir; öğrenme oranı planları (`torch.optim.lr_scheduler`) bunu her adımda yapar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* "Model yazmak ve eğitmek": ısınma ve düşürme planları büyük modelin olağan işidir; plan ayrı bir nesne değil, her adımda bir atamadır (tek yazım). Oran 32 bit saklanır ki CPU'da ve GPU'da (çok tensörlü tek başlatmalı yolda da) aynı adımı versin. Oran `op.TR_state`'e girmez, plan adım sayısından yeniden hesaplanır: sürdürülen eğitim kesintisizle aynıdır ("Güvenilir", 20.17). 0 geçerlidir: planlar eğitimin sonunda oranı 0'a indirir.

### 15.36

- *Emsal:* Keras `TerminateOnNaN` ve Lightning `EarlyStopping(check_finite=True)` NaN'da eğitimi durdurur; PyTorch `GradScaler` inf/NaN gradyanlı adımı atlar (`optimizer.step()`'i çağırmaz); Megatron-LM NaN'ı ve kaybın ilk değerlerin en büyüğünün 10 katını aşmasını yakalayıp adımı yeniden koşar; OLMo-core kayıp ve gradyan normunun son 128 adıma göre 6 standart sapmalık sıçramasını yalnız kaydeder; PaLM kayıp sıçramasında yaklaşık 100 adım önceki kontrol noktasına dönüp 200–500 toplu işi atlamıştır (Chowdhery ve ark. 2022, §5.1) [kanıtlı: bugünkü 15.36'nın "Karşılaştırma ve karar"ı]. `TR_error_if_nonfinite` PyTorch'un `clip_grad_norm_(error_if_nonfinite=True)`'sudur. Sonlu olmayan normda kırpma ve float32'de taşan norm PyTorch'unkiyle aynıdır (PyTorch 2.14'te ölçüldü; PyTorch ölçek 1'de de yerinde çarpar) [ölçüldü: bugünkü 15.36'nın metni]. `TR_is_finite` `torch.isfinite` ve `numpy.isfinite`'in karşılığıdır.
- *Gerekçe:* "Güvenilir" ve "Hata riski az": uzun eğitim sessizce bozulmamalı. Hiçbir emsal tek doğru politika değildir; TR politikayı dile gömmez, parçalarını verir: denetim (`TR_is_finite`, `TR_error_if_nonfinite`), geri dönme ve toplu iş atlama (20.14, 20.15), sıçrama bekçisi (`tr/training`). `TR_is_finite`'in `TR_Bool` vermesi 15.3'ün kararıdır (mantık ile sayı arasında dönüşüm yok; maske mantıktır). Kırpmanın ölçek 1'de de sürümü artırması, kırpmadan önce gradyanı kaydetmiş bir hesabın sonucunun norma bağlı olmamasını sağlar.
  - **Kayıp da sınanır.** `TR_sum` ve `TR_mean`'in türevi değerden bağımsızdır: NaN'lı kayıpta gradyan sonlu çıkabilir, normun denetimi adımı geçirir [ölçüldü: DENETİM, `torch==2.14`'te de]. Kalıp kaybı ve normu birlikte sınar (Gemini Koşu 3).

### 15.37

- *Emsal:* PyTorch'ta cihaz adı üreticiye bağlıdır (`"cuda"`, `"mps"`) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* "Makinenin içinde çalıştırmak ... onun işlemcisinde": adlar NVIDIA'ya bağlı olmamalı, başka hızlandırıcılar aynı adlarla gelir. Bugün `" CPU "` kabul ediliyor [ölçüldü]; 17.9'un tek yazımı girdinin her yerindedir, harf ve boşluk serbestliği iki yazım demektir. `"gpu"` kısa adı kalır (tek GPU'lu programların olağan yazımı), ama `.TR_device` uzun adı verir: okunan ad tektir.

### 15.38

- *Emsal:* PyTorch'un `torch.set_default_device`'ı [kanıtsız: bu tur açılmadı].
- *Gerekçe:* "Herkes için": bir modeli GPU'da eğitmenin en sade yolu, model kurulmadan önce varsayılan cihazı seçmektir; ağırlıklar, iyileştiricinin durumu ve toplu işler orada olur. Varsayılan programındır, göreve bağlı değildir: model bir kez, kurulmadan önce seçilen cihazda kurulur. Sayının tensörün cihazında kullanılması 15.40'ın "sessiz kopya yok"uyla tutarlıdır.

### 15.39

- *Emsal:* PyTorch'ta türev izleyen bir yaprağın `.to(...)` kopyası yaprak değildir, gradyan kaynağa akar [kanıtsız: bu tur açılmadı]; TR kopyayı yeni yaprak yapar.
- *Gerekçe:* `TR_self.W = ağırlık(...).TR_to_device("gpu")` olağan yazımdır; sonuç yeni yaprak olmazsa iyileştirici GPU'daki ağırlığı tutamaz. Parametre taşınınca parametre, donmuşsa donmuş kalır (DENETİM TUM-tur2): yoksa donmuş gövde taşınınca çözme döngüsü (15.34) onu bulmazdı.

### 15.40

- *Emsal:* PyTorch da farklı cihazlardaki tensörlerle işlemi hatayla reddeder [kanıtsız: bu tur açılmadı].
- *Gerekçe:* "Hata riski az": sessiz kopya gizli bir maliyettir ve hangi cihazda hesaplandığını belirsiz yapar; ileti doğru yazımı (`.TR_to_device(...)`) verir.

### 15.41

- *Emsal:* GPU'da temel aritmetik PTX'in doğru yuvarlanmış `.rn` komutlarıyla yapılıyor [ölçüldü: DENETİM, PTX]. Belirlenimciliğin tanımı 15.22a'dadır.
- *Gerekçe:* "Güvenilir": aynı program aynı veriyle aynı sonucu verir. Aynı cihaz türünde bit bit aynılık söz verilir; cihaz türleri arasında söz, matematiğin verebildiği kadardır ve üç basamakta açıkça yazılır: IEEE 754'ün doğru yuvarlanmış işlemleri her yerde aynıdır; matematik işlevlerinin ölçülmüş ulp sınırı vardır; toplama sırası değişen ondalık indirgemelerde sayısal sınır söz verilmez, testler `TR_all_close` ile karşılaştırır. GPU'da olmayan işlemin sessizce CPU'ya düşmesi hızı gizlice yok ederdi ("Hata riski az").

### 15.42

- *Emsal:* yok: taslakta ve bugünkü metinde karşılaştırma yapılmadı.
- *Gerekçe:* "Hata riski az": yanlış bir yayın (`TR_zeros([185000, 1, 1]) + TR_zeros([4, 46000])` → 126,8 GB) süreci çökertmek yerine işlemi, şekli ve belleği söyleyen yakalanabilir bir hata verir [kanıtlı: bugünkü 15.42'nin örneği]. Öğe sayısında elle seçilmiş sınır yoktur: önceki 50 milyon (verisiz şekil, liste) ve 2³² (dosya) dayanaksızdı ve 4.10'un 2²⁷'siyle çelişiyordu; sınırın işi yanlış yayının ya da güvenilmeyen dosyanın belleği bitirmesini ayırmadan önce yakalamaktır, bu da bellekle ölçülür (Gemini'nin bütün 15 raporu; DENETİM doğruladı). DENETİM GPU'nun 32 bitlik indeksinden 2³¹ − 1'i önerdi; o sayı motorun bugünkü çekirdeğinden gelir, dil kuralı değildir (status'ta). Sıfır eksenlerin 1 sayılarak taşma denetimi (DENETİM TUM-tur), verisiz bir şeklin sonraki işlemlerde dev bir sonuç doğurmasını int64 düzeyinde yakalar.

### 15.43

- *Emsal:* çöp toplayıcılı dillerin hepsi aynı sonuca varmıştır: toplayıcı cihaz belleğini görmez ve GPU eğitimi için yavaştır. Çözümleri yükü kullanıcıya bırakır: TorchSharp (C#) `DisposeScope` içinde kalacak tensörü `MoveToOuterDisposeScope` ile dışarı taşıtır (unutulan dönüş değeri silinir; yine de her toplu işten sonra `GC.Collect()` önerir); DJL (Java) `NDManager` kapsamları açtırır; TensorFlow.js `tf.tidy` içinde döneni korur, gerisini siler, kalacakları `tf.keep` ile işaretletir; CUDA.jl (Julia) toplayıcıya ek olarak elle `unsafe_free!` ister ve bırakılmış diziye erişimi hata sayar. Python/PyTorch referans sayacıyla tensörü son referans gidince bırakır (görünümler ve kaydedilen tensörler depoyu tutar); Mojo ve Rust derleme anında son kullanımı bulur, ifade içindeki ara değerleri de hemen yok eder [kanıtlı: bugünkü 15.43'ün "Karşılaştırma ve karar"ı].
- *Gerekçe:* "Hazır modelleri çalıştırmak ... hızlı" ve "Herkes için": kullanıcı bellek yönetimi yazmamalı. TR kapsamı fonksiyon çağrısı yapar (`tf.tidy` gibi), ama dönen değeri kendiliğinden çağırana geçirir, kaçışı kendisi algılar (`tf.keep` gerekmez), ara sonuçları Mojo gibi işlemden hemen sonra, depoyu PyTorch gibi son sahibi gidince bırakır; emin olamadığı her şeyi çöp toplayıcıya bırakır. Ölçüm (hedef model, RTX PRO 6000): yalnız çöp toplayıcıyla bellek ancak toplama sıklaşınca iner (eşik 1536 MB'de 12,38 GiB / 195 ms/adım, 512 MB'de 11,85 / 277, 128 MB'de 11,61 / 696): toplama pahalıdır ve sayısı modelle büyür [ölçüldü: bugünkü 15.43'ün metni].

### 15.44

- *Emsal:* PyTorch `torch.compile` ve JAX `jit` bütün programı izleyip derler (genel derleyici; izlenemeyen yerde `torch.compile` sessizce kaynaşmamış yola döner); CuPy `cupy.fuse` öğe öğe bir Python fonksiyonunu tek çekirdeğe çevirir ama türev almaz; Julia'nın `@.` yayını sözdizimiyle kaynaştırır; Triton ve Liger elle yazılmış birleşik çekirdeklerdir [kanıtlı: bugünkü 15.44'ün "Karşılaştırma ve karar"ı]. Kapanıştaki sayıyı JAX'ın `jit`'i iz anında sessizce sabitler, `torch.compile` değişen değerde korumayla yeniden derler [kanıtlı: bugünkü 15.44'ün metni]. Bugün TR'de CPU'da `TR_fuse` eski değeri, `TR_replay` yeni değeri veriyor, GPU'da `TR_replay` de sabitliyor [ölçüldü: DENETİM].
- *Gerekçe:* "Hazır modelleri çalıştırmak ... hızlı" ve "Hata riski az". TR `cupy.fuse`'un açık ve dar kapsamını alır: kullanıcı neyin kaynaştığını yazarak seçer, kaynaştırılamayan işlem sessizce geçilmez, hata verir; geri yönü `torch.compile` gibi ara değerleri yeniden hesaplayarak üretir ve ileri yön bit düzeyinde aynıdır (derleyicilerin aksine sonuç değişmez). **Kapanıştaki sayının denetimi:** bugün kapanıştan okunan sayı değişince sonuç sessizce eski değerle hesaplanıyor (`sabit` 2'den 5'e çıktı, sonuç `[2, 4]` kaldı [ölçüldü]); öğrenme oranı ya da adım gibi değişen değerler tam bu biçimde yazılır, sessiz eskime "Hata riski az"ın karşıtıdır. Yeniden kaydetmek (`torch.compile`'ın korumaları) doğru sonucu verirdi ama gizli bir maliyet taşır; hata, ajana değeri argüman yapmasını söyler. Aynı program cihaza göre ayrı sonuç vermesin diye denetim her cihazdadır.

### 15.45

- *Emsal:* PyTorch `torch.cuda.graph` ve `make_graphed_callables` aynı kuralları koyar (sabit adresler, kayıtta CPU ile eşzamanlama yok, CPU işi oynatmada atlanır, kayda özel bellek havuzu, önce ısınma), ama girdileri statik tensörlere kopyalamayı ve bellek ömrünü kullanıcıya bırakır ("girdiler canlı tutulmalıdır"); TR ikisini de kendisi yapar. transformers `cache_implementation="static"` ile `torch.compile(mode="reduce-overhead")` üretimde aynı yolu izler (sabit boylu önbellek, konum cihazda); llama.cpp her parçada çizgesini yeniden kurup kaydın KV önbelleğine bağlı çekirdek parametrelerini yamar. İç içe kayıt JAX'ta iç içe `jit` gibidir. Kaynaklar: PyTorch 2.14 CUDA notları ("CUDA Graphs"); NVIDIA sürücü API'si `cuStreamBeginCapture`, `cuGraphLaunch`; NVIDIA "Optimizing llama.cpp AI Inference with CUDA Graphs" [kanıtlı: bugünkü 15.45'in "Karşılaştırma"sı]. JAX `lax.dynamic_update_slice` sınır dışını kırpar (15.8'in emsali).
- *Gerekçe:* "Hazır modelleri çalıştırmak ... hızlı" ve "Sunmak": küçük, sık işlerde (tek dizgecik üretimi) asıl maliyet çekirdek başlatmasının konak yüküdür. **Hatalar kırpılmaz:** bugün kayıtta sınır dışı gömme indeksi sessizce kırpılıyor [kanıtlı: bugünkü 15.45'in metni]; 15.6'nın cihaz içi bayrağı hatayı oynatmanın sonunda verir: gecikmeli ama sessiz değil. **Kapanıştaki tensörün deposu yoluyla denetlenir** (DENETİM TUM-tur): aynı yol başka depoya bağlanırsa kayıt eski bellekle sessizce oynatırdı. **Rastgele üreteç oynatmada ilerler, KV önbelleği kapanıştan verilir** (Gemini'nin bütün 15 raporu, 6. bulgu; DENETİM doğruladı): oynatma doğrudan çağrıyla aynı sayıları vermeli, kontrol noktasının kaydı (20.17) oynatmadan sonra da doğru olmalı ("Güvenilir"). Hata verilen oynatmada yerinde argümanların tanımsız olduğu açıkça yazılır (DENETİM E1): hata o ana kadarki yazmaları geri almaz, program durumu yeniden kurar.

### 15.46

- *Emsal:* cinsi çalışırken bilinen tensör tipi Rust'ın `safetensors`'ının `TensorView`'una benzer: cins çalışırken, kullanırken daraltılır [kanıtsız: bu tur açılmadı]. Yazarken uzantıyla biçim seçmek 18.13'ün `.gz`'si gibidir.
- *Gerekçe:* "Güvenilir" ve "Paylaşmak". 15.1 cinsleri tipe koydu; dosya bunları saklayamazsa tam sayı tensörü (dizgecik kimlikleri, maskeler, adım sayacı) kaydedilemez ya da float'a sessizce çevrilir. Biçim yalnız tür kodu kazanır, sürüm 2 kalır. Tip için üç yol tartıldı: cinsi silinmiş tensör tipi, `TR_load_tensors[E]` (bütün dosya tek cins: model ile adım sayacı birlikte okunamaz), cins başına sözlük (her okuyucu üç sözlüğü birleştirir). Birincisi alındı (DENETİM'in önerisi); daraltma tip çağrısı değil `.TR_narrow[E]()`'dir (15.1), cins uymazsa hata. TR biçimi CRC taşıdığı için kontrol noktası onu kullanır; safetensors paylaşmak içindir. Dosya güvenilmeyen girdidir: bildirilen veri ayırmadan önce dosyayla denetlenir; öğe sayısında elle seçilmiş sınır yoktur (15.42'nin gerekçesi), 2³² kalktı.

### 15.47

- *Emsal:* kurallar safetensors'un kendi okuyucusundan alındı (safetensors README "Format"; `safetensors/src/tensor.rs`: `MAX_HEADER_SIZE: usize = 100_000_000`, `validate` içinde `if s != start || e < s` ve `if e - s != size`, `MetadataIncompleteBuffer`). README yinelenen anahtarı "her ayrıştırıcı uymayabilir" diye bırakır; Python `safetensors` 0.8 `__metadata__`'da yineleneni kabul eder [kanıtlı: bugünkü 15.47'nin "Karşılaştırma ve karar"ı].
- *Gerekçe:* "Paylaşmak ... başka araçlarda eğitilmiş modelleri almak": Hugging Face ve PyTorch ağırlıkları safetensors'tadır. Dosya güvenilmeyen girdidir: denetimler veri okunmadan yapılır, dosya boyunu aşan ayırma olmaz. Yinelenen anahtarı TR reddeder, çünkü hangisinin geçerli olduğu belirsizdir ve Go'nun JSON çözücüsü sessizce sonuncuyu alırdı. Okumada biçim içerikten anlaşılır (15.47a'nın gerekçesi). `F8_E4M3`'ün "fp8" olarak okunması 15.55'in kararıdır.

### 15.47a

- *Emsal:* Python'un `safetensors`'ı başlığı boşlukla 8 bayta tamamlar ve tensörleri hizaya göre sıralar (önce geniş türler) [ölçüldü: DENETİM]. transformers `__metadata__`'ya `{"format": "pt"}` yazar, okumada denetlemez [ölçüldü: DENETİM]. I64, U8 ve BOOL Python'da okunuyor [ölçüldü: DENETİM].
- *Gerekçe:* CLAUDE.md "eğitilen modeli yaygın biçimlerde dışarı vermek"i ister ("Paylaşmak"); bugün yalnız okunuyor. İki biçim kalır: TR biçimi CRC taşıdığı için kontrol noktası onu kullanır ("Güvenilir"), safetensors paylaşmak içindir. Yazarken uzantıyla seçmek 18.13'ün `.gz`'si gibidir; okumada içerikten anlaşılır. Python'unkiyle aynı düzen, başka araçların dosyayı hiçbir ayar olmadan okumasını sağlar. ONNX ve GGUF dışa verme yol haritasındadır (DENETİM E1'in hükmü).

### 15.48

- *Emsal:* yok: hatanın yakalanması dilin genel kuralıdır (8); başka araçların şekil iletileri bu tur karşılaştırılmadı.
- *Gerekçe:* ajan iletiyi okur ve düzeltir: ileti uymayan şekilleri ve ekseni söyler. Hata işlemin kendi satırında verilir ve yakalanır; program çökmez (15.3'ün "hata kendi satırında" ilkesi). Cins uyuşmazlığı tipte olduğu için (15.1) çalışmadan önce, denetimde yakalanır.

### 15.49

- *Emsal:* her satır PyTorch ve NumPy ile karşılaştırılmıştır:

  | Kural | PyTorch / NumPy |
  | :-- | :-- |
  | Yayın | Aynı kural, iki taraflı genişleme sessiz. |
  | Nicemlenmiş tensör | PyTorch'un nicemlenmiş tensörü de çoğu işlemde hata verir [ölçüldü: DENETİM F1]; bitsandbytes katmanın içinde açar. |
  | Tür yükseltme | Aynı; yalnız PyTorch karşılaştırmada sayıyı daraltır (`bf16(1.0) > 0.999` yanlış) [ölçüldü: DENETİM]. |
  | Daraltma | Sessizce sarar (300 → 44). |
  | Kopya reshape | Yazılır, asıl tensör değişmez. |
  | Cinsler | PyTorch ve NumPy tam sayıyı ondalığa sessizce yükseltir. |
  | Tam sayı taşması | Sessizce sarar; NumPy "(incorrect)" der. |
  | Karşılaştırma | PyTorch'ta `bool` tensör. |
  | Tensör indeksi | Aynı. |
  | Tensör dilimi | Aynı. |
  | `.TR_slice` | `torch.narrow` gibi. |
  | Sayı olmayan işlenen | NumPy ve PyTorch `True`'yu 1 sayar. |
  | NaN | Aynı. |

- *Not:* eski tablonun "liste ve metin indeksi" satırı tensörün değil listenin kuralıdır; 3.3 ve 3.6'da durur, bu tabloya alınmadı.
- *Gerekçe:* sessiz yanlış, ajanın göremediği hatadır ("Hata riski az"); belirsiz durum hata ya da uyarı olunca ajan iletiyi okuyup düzeltir. Tablo ayrıntıları kendi maddelerinde olan, yanlış kullanıldığında sessiz hataya yol açabilecek kuralları tek yerde toplar. Cinsler karışmaz: 3.13 ve 4.2'nin sayıdaki kuralı tensörde de geçerli kalır; sayı ile tensörde sayı tensörün cinsine uymalıdır (`TR_Tensor[TR_Int] * 2.5` hata). Reddedilen yol: PyTorch'un `int + float` → float yükseltmesi.

### 15.50

- *Emsal:* üs ve matematik işlevlerinin özel değerleri C99 Ek F'den ([cppreference `pow`](https://en.cppreference.com/w/c/numeric/math/pow)), toplamda sıfırın işareti IEEE 754-2019 §6.3'ten; PyTorch 2.14.0 ve NumPy 2.1.1 davranışı yerelde ölçüldü [ölçüldü: 2026-10-23]; Python `sum`ının −0 kuralı [CPython #100426](https://github.com/python/cpython/pull/100426)'dan. Satır satır:

  | Kural | PyTorch / NumPy |
  | :-- | :-- |
  | NaN yayılır | Aynı. |
  | Tek işlevler sıfırın işaretini korur | Aynı (PyTorch'un tam `gelu`su uzun tensörde −0'da +0 verdi). |
  | Toplam | Aynı (PyTorch ve NumPy `sum`, Python `sum` ve `math.fsum`). |
  | Üs | Aynı. |
  | Doyma ve tanım dışı | Aynı. |
  | Limitler | PyTorch NaN verir (∞·0). |
  | Tamamen −∞ satır | PyTorch NaN verir. |
  | `TR_logsumexp` | bu tur karşılaştırılmadı. |
  | `TR_sign` | PyTorch +0 ve 0; NumPy +0 ve NaN. |
  | `TR_relu`, `TR_abs` | PyTorch'ta `relu`(−0) = −0, `relu`'nun türevi NaN'da 1, `abs`'ınki NaN'da 0. |
  | `TR_max`, `TR_min` | PyTorch `amax`'ta türev eşitlere bölünür. NumPy `max` ve tam indirgemeli `torch.max` işaretli sıfırda +0 verebilir. |
  | `TR_maximum`, `TR_minimum` | PyTorch ±0'da ilk argümanı verir, sonuç argüman sırasına bağlı [ölçüldü: DENETİM]; tam eşitlikte türev 0.5/0.5 [ölçüldü: DENETİM]. |
  | `TR_clamp` | PyTorch'ta sınırda türev 0. |
  | `TR_atan2` türevi | PyTorch sonsuzda NaN verir. |

- *Gerekçe:* "Güvenilir": aynı program aynı veriyle her cihazda aynı sonucu verir; özel değerler de buna dahildir. Seçim gerektiren satırlar:
  - **NaN yayılır:** IEEE 754; üs için C99 Ek F.
  - **Tek işlevler ve doyma:** C99 Ek F.
  - **Toplam:** IEEE'nin sıralı toplaması −0 verirdi (−0 + −0 = −0); ekosistemle aynı bitleri vermek seçildi. Python `sum`ı yalnız `start=-0.0` ile −0 verir.
  - **Limitler:** matematiksel limit; taşmış bir etkinlik eğitimi NaN'a götürmez.
  - **Tamamen −∞ satır:** maskeli satır kuralı (15.9, 15.24).
  - **`TR_logsumexp`:** boş toplamın logaritması −∞'dur; +∞'da türev limitteki gibidir; birden çok +∞'da ilk öğe `TR_max` ve `TR_argmax` ile aynı öğedir (belirlenimci).
  - **`TR_sign`:** her x için (−0 dahil) `TR_sign(x)·TR_abs(x) = x`; NaN gizlenmez.
  - **`TR_relu`:** max(0, x); IEEE 754-2019 `maximum`'da +0 > −0. NaN girdinin türevi NaN'dır (`TR_clamp` ve `TR_maximum` da): NaN geri yönde de görünür kalır. PyTorch'ta `relu`'(NaN) 1, `clamp`'(NaN) 0, `maximum(NaN, 1)` iki yana 1 [ölçüldü: DENETİM, `torch==2.14`]; bunlar uygulamanın sınamasının izidir (`x <= 0`, `x > 0`), TR bilerek ayrılır.
  - **`TR_max`, `TR_min`:** tek ve belirlenimci seçim (`TR_argmax` ile aynı öğe); ±0 sıralama sırasıyla ayrılır (15.22b).
  - **`TR_maximum`, `TR_minimum`:** sonuç argüman sırasından bağımsızdır.
  - **`TR_clamp`:** sayı sınırda sınırdaki değer aralığın içindedir; tensör sınırda `TR_maximum` ve `TR_minimum` ile aynıdır (15.13).
  - **`TR_atan2` türevi:** matematiksel limit.
  - **Sayıda hata, tensörde NaN:** tensörün tek bir öğesi bütün işlemi durdurmamalı; NaN eğitimin kararlılık araçlarıyla (15.36) yakalanır.
  - Kural konurken bulunup düzeltilenler [ölçüldü: 2026-10-23]: GPU'da `tanh`, `tan`, `expm1` ve `sin` (`sin` CPU'da da) −0'ı +0'a çeviriyordu; GPU üssü C99'dan ayrılıyordu ((−0)³ = +0, (−∞)^0,5 = NaN, (±1)^∞ = NaN); CPU `tanh`(∞) 0,99999994 veriyordu; GPU −0'ların toplamını −0 veriyordu; GPU `atan2` türevi NaN'ı 0'a çeviriyor, CPU sonsuzda NaN veriyordu; tanh yaklaşımlı `gelu` −∞'da NaN veriyordu.

### 15.51

- *Emsal:* "q8" ve "q4" llama.cpp'nin en yaygın iki blok biçimidir (`Q8_0`, `Q4_0`: blok 32, fp16 ölçek); "q4z128" Hugging Face'teki GPTQ ve AWQ modellerinin olağan düzenidir (grup 128, ölçek ve sıfır); "fp8" yeni GPU'ların donanım türüdür [kanıtsız: bu tur açılmadı; DENETİM'e]. Tensör başına ölçek bugünkü FP8 modellerine yetmez: Qwen3-8B-FP8 `weight_block_size: [128, 128]` kullanıyor [kanıtlı: DENETİM F1, `config.json`]. "q4z128"in sıfırı gerçek sıfırdır, 0..16, bir bayt (Gemini'nin bütün 15 raporu, 11.1 bulgusu, kaynakla; DENETİM doğruladı).
- *Gerekçe:* "Uyarlamak ... modeli küçültüp hızlandırmak (nicemleme)", "Hazır modelleri çalıştırmak ... hızlı" (4 bitlik ağırlık float32'nin sekizde biri), "Makinenin içinde çalıştırmak" (küçük bellek), "Paylaşmak ... başka araçlarda eğitilmiş modelleri almak". Nicemlenmiş ağırlık `TR_Float`'un saklama hassasiyetidir (15.1): değeri ondalıktır, `TR_Int`'e konsa ölçek kaybolur ve her işlem açık dönüşüm ister; adları "int8" gibi `TR_Int` adlarıyla çakışmaz, `.TR_dtype`'in değeri cinsi tek başına söyler. Az sayıda, adı düzeni söyleyen hassasiyet: ajan bir adı okuyunca bayt düzenini bilir. Grup boyunun adda olması da bu yüzdendir. K-nicemleme ailesi (GGUF `Q4_K` ...), NF4 (bitsandbytes) ve 2–3 bit Sonraya.

### 15.52

- *Emsal:* llama.cpp `ggml-quants.c`: ölçek float32'de hesaplanır, kod `x · (1 / d)` ile bulunur, `d` saklanırken fp16'ya yuvarlanır; "q4"te `d` = işaretli en büyük / −8, `+8.5` ile kesme; "q8"de sıfırdan uzağa yuvarlama [kanıtlı: DENETİM F1, `ggml-quants.c`]. E4M3'e dönüşüm PyTorch'unki gibi doyurur ve yarımı çifte yuvarlar [ölçüldü: DENETİM]. AutoGPTQ'nun dosyası sıfırı bir eksiğiyle saklar (`zeros -= 1`), açarken `& 15` ve `+ 1` uygular [kanıtlı: DENETİM, AutoGPTQ `qlinear_cuda.py`]. Hugging Face'in `weight_scale_inv`'inin çarpılan ölçek olduğu [hipotez: DENETİM F2].
- *Gerekçe:* "Uyarlamak" ve "Paylaşmak".
  - **`.TR_to_dtype` ile:** nicemlenmiş ağırlık `TR_Float`'un hassasiyetidir (15.1); hassasiyet değiştirmenin yazımı zaten `.TR_to_dtype`'tır, yeni ad gerekmez. Tek istisna türevdir: nicemleme türevlenmez (yuvarlama).
  - **Parçalardan kurmak `TR_quantized` ile:** hazır modelin parçalarını açıp yeniden nicemlemek yayımlanan bitlerden ayrılır [kanıtlı: DENETİM F1]; bu yüzden parçalardan kuran bir yerleşik gerekir (yeni adlar `TR_quantized`, `TR_zero_point`; `TR_scale` var olan ad, aynı anlam: çarpan). 1. turdaki "yeni ad yok" tutmadı.
  - **llama.cpp ile bit bit:** "q8" ve "q4" adları `Q8_0` ve `Q4_0`'ın karşılığıdır; aynı ad başka bit verirse TR'de nicemlenen model llama.cpp'ninkinden sessizce ayrılır. 1. turdaki "mutlak / 8" kuralı pozitif en büyüğü 7'ye kırpıyordu [kanıtlı: DENETİM F1]. Yuvarlama bu yüzden 13'ün yarımı çifte kuralından ayrılır.
  - **NaN'lı blok hata:** NaN bir bloğun ölçeğini NaN yapar ve 32 öğeyi birden bozar; sessizce yapılırsa model bozulmuş görünmeden kalite kaybeder ("Hata riski az").
  - **Aralık dışı kod, sıfır ve ölçek hata** (Gemini'nin bütün 15 raporu, 11.3; DENETİM doğruladı): paketli depoda aralık dışı kod komşu öğeyi bozar.
  - **Sıra değiştiren gruplar** (`desc_act`) desteklenmez: Sonraya (DENETİM F2'de kapandı).

### 15.53

- *Emsal:* PyTorch'un nicemlenmiş tensörü de çoğu işlemde hata verir [ölçüldü: DENETİM F1]; bitsandbytes ağırlığı katmanın içinde açar [kanıtsız]. Hazır modellerin ağırlıkları `[çıkış, giriş]` düzenindedir (PyTorch'un `Linear`'ı, GGUF, GPTQ böyle saklar). QLoRA nicemlenmiş tabanı dondurup bağdaştırıcıyı eğitir; TR'deki yazım "QLoRA benzeri"dir, QLoRA'nın NF4'ü yok. Hugging Face'in FP8 modelleri etkinleştirmeyi de çalışırken nicemler.
- *Gerekçe:* "Hazır modelleri çalıştırmak ... hızlı" ve "Uyarlamak".
  - **Sayılı işlemler, sessiz açma yok:** nicemlemenin değeri bellekte ve hızdadır, ikisi de yalnız çarpımda ve gömmede kazanılır; başka bir işlemde gizli açma bu değeri sessizce yok eder. Hata ajana açmayı açıkça yazdırır (15.49).
  - **Sağ işlenen `[N, K]`'nin görünümü:** bloklar indirgeme ekseni boyunca olmalıdır ki çekirdek bir bloğu bir kez açıp K boyunca toplasın. Hazır modellerin `[çıkış, giriş]` düzeni TR'nin `x @ W` düzeninde `w.TR_transpose()`'tur; tek depo kuralı ("bloklar son eksende") böylece iki düzende de geçer.
  - **Türev yalnız girdiye:** QLoRA'da gradyan girdiye akmalı, ağırlığa akmamalıdır; bu "Uyarlamak"ın en ucuz yoludur.
  - **Nicemlenmiş gömme `TR_compute_dtype()`'a açılır** (Gemini'nin bütün 15 raporu, 5b; DENETİM doğruladı): `@`'nin blok kuralıyla aynı olsun.
  - **FP8'de etkinleştirme nicemlenmez:** yalnız ağırlık nicemlenir; özgün uygulamayla bit bit aynılık söz verilmez, açıkça yazılır.

### 15.54

- *Emsal:* yok: taslakta karşılaştırma yapılmadı.
- *Gerekçe:* bir öğeye yazmak bloğun ölçeğini değiştirirdi; açık açma ve yeniden nicemleme ajana neyin değiştiğini gösterir ("Hata riski az"). Yükleme aynı belleğe bütün olarak yazar (Gemini'nin bütün 15 raporu, 2b; DENETİM TUM-tur): yeni bellek görünümleri ve `TR_replay` kaydını eskitirdi; kontrol noktasından sürdürme (20.6) ve geri dönme (20.14) var olan tensörlere yazmalıdır ("Güvenilir").

### 15.55

- *Emsal:* safetensors'ta nicemlenmiş tür yoktur; GPTQ modelleri `qweight`, `qzeros`, `scales`'i ayrı tam sayı ve ondalık tensörler olarak saklar; FP8 modelleri ağırlığı safetensors'un `F8_E4M3` türünde, ölçeği (`weight_scale_inv`) ayrı tensörde saklar [kanıtsız: taslakta etiketsiz].
- *Gerekçe:* "Güvenilir": TR biçimi nicemlenmiş tensörü kendi hassasiyetinde saklar, yeniden yükleme aynı bitleri verir; biçim yalnız tür kodu kazanır, sürüm 2 kalır (15.46'nın gerekçesi). "Paylaşmak": hazır modellerin parçaları olduğu gibi okunur, birleştirmek kitaplığın işidir. `.safetensors`'a nicemlenmiş tensör yazmak, başka araçların okuyamayacağı bir tür uydurmak olurdu; hata açmayı ya da TR biçimini önerir. GGUF okuma Sonraya: Hugging Face'in en çok indirilen 15 nicemlenmiş deposunun hepsi GGUF, yaygın tür K ailesi [ölçüldü: DENETİM, API]; en yaygın dağıtım bu yüzden bugün kapsam dışıdır (yol haritası).
