# 12. Testler: emsal ve gerekçe

Kurallar: [`12_testler.md`](../12_testler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 12.1

- *Emsal:* Swift Testing'in `@Test("ad")`'ı ve Zig'in `test "ad" { ... }` bloğu testi adıyla birlikte dilin içinde tanımlar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajan kodunu ve testini aynı dilde yazar; test bloğu modülde ya da test dosyasında, program modülü aktarır (Python'un modül ile `__main__` ayrımı gibi). Zig'in "test kipinde en dış deyimler atlanır" yolu alınmadı: aynı dosya iki kipte ayrı anlam taşırdı, sabit olmayan ilk değeri (`TR_var model = kur()`) olan dosyada hangi tanımın kurulacağı belirsiz kalırdı (Gemini Koşu 7).

- *Gerekçe (klasör taraması, 2026-11-05, DENETİM istek 132, 138):* test komutu koşmadığı testi geçmiş göstermemeli. Rehber "adı `_test.tr` ile biten" diyordu, 12.1 `TR_test` içeren dosya diyor: tanım 12.1'dir. Ad yine rol alır: sözcükleri bozuk dosyada sözcük birimi görünmez, ad ve ham `TR_test` araması onu taramada tutar. `testdata` Go'nun bildik kuralı (`go help packages`: "as are directories named "testdata"") [ölçüldü: DENETİM istek 139]; `negative` gibi yalnız bu depoya özgü ad kullanıcının aynı adlı klasörünü sessizce atlardı.

### 12.2

- *Gerekçe (en dış, 2026-11-05, DENETİM istek 138 Ş2):* iç içe test ayrıştırılıyor ama koşmuyordu: `✅ 0 test`, çıkış 0. Hata, sessizce atlamaktan iyidir. Emsal: Rust `unnameable_test_items` uyarısı varsayılan açık [kanıtlı: DENETİM, rustc lint listesi; anlamı bellekten].

- *Emsal:* Python'da aynı adlı iki test metodunun ikincisi birincisini gizler ve ilk test hiç çalışmaz [ölçüldü: DENETİM 12-1, Python 3.12 `unittest`].
- *Gerekçe:* kopyalayıp yapıştırırken test adını değiştirmeyi unutmak ajanın ve insanın sık hatasıdır; sonuç ya bir testin hiç çalışmaması ya da raporda hangisinin kaldığının belirsizliğidir. Sözdizimi hatası bunu yazılırken gösterir. Bugün iki aynı adlı test ikisi de çalışıyor [ölçüldü].

### 12.3

- *Emsal:* Python'un `assert`'i ve PyTorch'ta tensörün koşulda çok öğeliyse hata vermesi [kanıtsız: bu tur açılmadı].
- *Gerekçe:* koşulun `TR_Bool` olması 3.10'un kuralıdır. Bugün `TR_assert(t == u)` tensörde tek mantık değeriyle geçiyor [ölçüldü]; 15'in öğe öğe eşitliğiyle bu bir maske olur ve sınama anlamsızlaşır. Denetim hatası ajanı doğru sınamaya yönlendirir.

### 12.4

- *Emsal:* JUnit'in `assertEquals(expected, actual)`'ı beklenen ile gerçeği ayrı gösterir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tip farkı ("5" ile 5) en sık eşitlik hatasıdır; ikisini tipiyle göstermek okuyan ajana ve insana sorunu doğrudan söyler.
- *Tur 6A (2026-11-10, tensör içeren değer):* PyTorch `torch.testing.assert_close` iç içe kapları yapısıyla ve öğe öğe karşılaştırır: "they are considered close if their structure matches and all their elements are considered close" [PyTorch 2.15, `torch.testing`]. Listedeki tensörün `==`'si yasaktır, çünkü maske verir (15.5); `TR_assert_equal` maske üretmez, ağırlık sözlüğünü ya da kayıp ve logit demetini doğrudan sınar.

### 12.4a

- *Emsal:* PyTorch'un `torch.testing.assert_close`'u toleransı hassasiyete göre seçer: float32 `1.3e-6 / 1e-5`, bf16 `1.6e-2 / 1e-5`, fp16 `1e-3 / 1e-5`, float64 `1e-7 / 1e-7` [ölçüldü: DENETİM, C'nin 2. turu, `torch.testing._comparison.default_tolerances`].
- *Emsal (formül):* Python'un `math.isclose`'u simetriktir (`max(|a|, |b|)`); NumPy'nin `allclose`'u ve PyTorch'un `assert_close`'u ikinci argümanı başvuru alır [kanıtsız: belgeler bu tur açılmadı]. Ajan bir aracı öbürü sanınca bağıl toleransın ölçeği değişir; simetrik formül sırayı önemsiz kılar.
- *Gerekçe:* ondalık sonuçlar kesin eşitlikle sınanamaz: `TR_assert_equal(0.1 + 0.2, 0.3)` kalır [ölçüldü]. ML testinde sınanan çoğu değer ondalık ve tensördür; ajan bir toleransı her seferinde uydurmak zorunda kalmamalıdır. 15.18a'nın `TR_all_close`'u ile aynı tablo.
- *Gerekçe (sonsuz):* formül sonsuzda IEEE'de tersini verir (sonlu ile sonsuz yakın, aynı işaretli iki sonsuz uzak); NumPy ve PyTorch'un `isclose`'u gibi sonsuz ayrı ele alınır [ölçüldü: DENETİM, `numpy==2.1.1`, `torch==2.14`].
- *Gerekçe (sayı ile tensör):* `.TR_item()` sayısı float64'tür ama hesabı float32'dir; tensörüyle karşılaştırmak toleransı doğru seçer. PyTorch iki toleranstan yalnız biri verilince hata verir; TR varsayılanı kullanır, çünkü ajan için tek argümanlık yazım daha az hatalıdır (Gemini Koşu 7).
- *Tur 6A (2026-11-10, tolerans):* Python `math.isclose`'da "_rel_tol_ must be nonnegative and less than `1.0`", _abs_tol_ de "must be nonnegative" [Python 3.15.2, `math`]. PyTorch eksi toleransı kaynakta reddeder: "rtol must be greater than or equal to zero" [`aten/src/ATen/native/TensorCompare.cpp`, main, `TORCH_CHECK`]. NumPy denetlemez; eksi tolerans sessizce doğru verebilir (`TR_rtol: -1.0, TR_atol: 5.0` → `TR_true` [ölçüldü: TCB, `70b1575b`]). NaN tolerans aynı nedenle hatadır, 13.7'de NaN sınır gibi.

### 12.5

- *Emsal:* pytest her testi ayrı bir fonksiyon çağrısında çalıştırır; ama rastgele üretecin durumunu sıfırlamaz, bunu eklentiler yapar (`pytest-randomly`) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: aynı test aynı sonucu vermeli. Bugün rastgele üretecin durumu testten teste sürüyor [ölçüldü]: bir testi eklemek ya da silmek öbür testlerin sonucunu değiştirir. Her testin aynı durumda başlaması bunu kapatır. En dışta yalnız tanım kuralı 1.1'dendir; bugün en dıştaki deyim de çalışıyor [ölçüldü]. Ortak kurulumu bir kez çalıştırmak (Go'nun `TestMain`'i, pytest'in modül fikstürü) yerine her testin bir fonksiyonu çağırması seçildi: tek kural, ve testler birbirinden bağımsız kalır. Aynı nedenle en dıştaki tanımlar her test için yeniden kurulur: bugün bir testin en dıştaki değişkene atadığı değer sonraki testte görünüyor [ölçüldü: main `e16d39df`]; testlerin sırası sonucu değiştirir.

### 12.6

- *Emsal:* Swift'te ve Rust'ta testten erken dönmek serbesttir [kanıtsız: bu tur açılmadı]. *Almadık.*
- *Gerekçe:* erken dönen test, sonraki doğrulamaları atlar ve "geçti" der; bu sessiz bir yanlıştır. Erken bitmesi gereken kısım bir `TR_if` bloğuna konur.
