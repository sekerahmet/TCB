# 12. Testler: emsal ve gerekçe

Kurallar: [`12_testler.md`](../12_testler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 12.1

- *Emsal:* Swift Testing'in `@Test("ad")`'ı ve Zig'in `test "ad" { ... }` bloğu testi adıyla birlikte dilin içinde tanımlar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajan kodunu ve testini aynı dilde, aynı dosyada yazar; test bloğu normal çalıştırmada atlandığı için dosya hem program hem test olur.

### 12.2

- *Emsal:* Python'da aynı adlı iki test metodunun ikincisi birincisini gizler ve ilk test hiç çalışmaz [ölçüldü: DENETİM 12-1, Python 3.12 `unittest`].
- *Gerekçe:* kopyalayıp yapıştırırken test adını değiştirmeyi unutmak ajanın ve insanın sık hatasıdır; sonuç ya bir testin hiç çalışmaması ya da raporda hangisinin kaldığının belirsizliğidir. Sözdizimi hatası bunu yazılırken gösterir. Bugün iki aynı adlı test ikisi de çalışıyor [ölçüldü].

### 12.3

- *Emsal:* Python'un `assert`'i ve PyTorch'ta tensörün koşulda çok öğeliyse hata vermesi [kanıtsız: bu tur açılmadı].
- *Gerekçe:* koşulun `TR_Bool` olması 3.10'un kuralıdır. Bugün `TR_assert(t == u)` tensörde tek mantık değeriyle geçiyor [ölçüldü]; 15'in öğe öğe eşitliğiyle bu bir maske olur ve sınama anlamsızlaşır. Denetim hatası ajanı doğru sınamaya yönlendirir.

### 12.4

- *Emsal:* JUnit'in `assertEquals(expected, actual)`'ı beklenen ile gerçeği ayrı gösterir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* tip farkı ("5" ile 5) en sık eşitlik hatasıdır; ikisini tipiyle göstermek okuyan ajana ve insana sorunu doğrudan söyler.

### 12.4a

- *Emsal:* PyTorch'un `torch.testing.assert_close`'u toleransı hassasiyete göre seçer: float32 `1.3e-6 / 1e-5`, bf16 `1.6e-2 / 1e-5`, fp16 `1e-3 / 1e-5`, float64 `1e-7 / 1e-7` [ölçüldü: DENETİM, C'nin 2. turu, `torch.testing._comparison.default_tolerances`].
- *Emsal (formül):* Python'un `math.isclose`'u simetriktir (`max(|a|, |b|)`); NumPy'nin `allclose`'u ve PyTorch'un `assert_close`'u ikinci argümanı başvuru alır [kanıtsız: belgeler bu tur açılmadı]. Ajan bir aracı öbürü sanınca bağıl toleransın ölçeği değişir; simetrik formül sırayı önemsiz kılar.
- *Gerekçe:* ondalık sonuçlar kesin eşitlikle sınanamaz: `TR_assert_equal(0.1 + 0.2, 0.3)` kalır [ölçüldü]. ML testinde sınanan çoğu değer ondalık ve tensördür; ajan bir toleransı her seferinde uydurmak zorunda kalmamalıdır. 15.18a'nın `TR_all_close`'u ile aynı tablo.

### 12.5

- *Emsal:* pytest her testi ayrı bir fonksiyon çağrısında çalıştırır; ama rastgele üretecin durumunu sıfırlamaz, bunu eklentiler yapar (`pytest-randomly`) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: aynı test aynı sonucu vermeli. Bugün rastgele üretecin durumu testten teste sürüyor [ölçüldü]: bir testi eklemek ya da silmek öbür testlerin sonucunu değiştirir. Her testin aynı durumda başlaması bunu kapatır. En dışta yalnız tanım kuralı 1.1'dendir; bugün en dıştaki deyim de çalışıyor [ölçüldü]. Ortak kurulumu bir kez çalıştırmak (Go'nun `TestMain`'i, pytest'in modül fikstürü) yerine her testin bir fonksiyonu çağırması seçildi: tek kural, ve testler birbirinden bağımsız kalır.

### 12.6

- *Emsal:* Swift'te ve Rust'ta testten erken dönmek serbesttir [kanıtsız: bu tur açılmadı]. *Almadık.*
- *Gerekçe:* erken dönen test, sonraki doğrulamaları atlar ve "geçti" der; bu sessiz bir yanlıştır. Erken bitmesi gereken kısım bir `TR_if` bloğuna konur.
