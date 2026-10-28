# 13. Yerleşikler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "13. Yerleşikler".

Her kuralın emsali ve gerekçesi: [`rationale/13_builtins.md`](rationale/13_builtins.md).

**13.1** Genel yerleşikler: `TR_print`, `TR_sum`, `TR_mean`, `TR_std`, `TR_median`, `TR_min`, `TR_max`, `TR_abs`, `TR_sqrt`, `TR_round`, `TR_sleep`, `TR_wait_all`, `TR_time_ms`, `TR_monotonic_ns`, `TR_read_file`, `TR_read_bytes`, `TR_write_file`, `TR_input`, `TR_wrapping_add`, `TR_wrapping_sub`, `TR_wrapping_mul`, `TR_unsigned_shift_right` ve `TR_error_code` (8.5). `TR_Type`, `TR_Message` ve `TR_String` gibi tip çağrıları fonksiyon değildir (3.12).
- Sayı yerleşiklerinin sonucu işleçlerin tipindedir (3.1, 4.2): `TR_sum(l)` `TR_List[TR_Int]`'te `TR_Int`, `TR_List[TR_Float]`'ta `TR_Float` verir. `TR_mean`, `TR_std` ve `TR_median` `TR_Float` verir. `TR_std` örneklem sapmasıdır, n−1'e böler (PyTorch'un varsayılanı); iki öğeden az listede hatadır. Ondalık toplam soldan sağa, telafili toplamayla (Neumaier) yapılır: sonuç öğelerin sırasından az etkilenir (`TR_sum([1e16, 1.0, -1e16])` → `1.0`); `TR_mean` ve `TR_std` aynı toplamı kullanır.
- Bu bölümün sayı yerleşikleri aynı girdide her makinede (x86, ARM64; Windows, Linux) bit bit aynı sonucu verir: işlemler birleştirilmeden (FMA'sız) yapılır, matematik fonksiyonları (`TR_sqrt` dışında `TR_exp` gibi) işlemciye göre yol seçmeyen tek bir algoritmayla hesaplanır. Tensör işlemlerinin kuralı 15. bölümdedir.
- Toplama yapan yerleşikler (`TR_sum` dışında `TR_mean`, `TR_std`, `TR_median`, `TR_min`, `TR_max`) boş listede hata verir.
- `TR_min` ve `TR_max` bir liste ya da iki ve daha çok değer alır. Değerler sıralanabilir olmalıdır (13.10): sayı ya da metin. `TR_Int` ile `TR_Float` karışırsa sonuç `TR_Float`'tır, 4.2'deki gibi (13.10): `TR_max(2, 1.5)` → `2.0`. Karşılaştırma 4.5'teki gibi kesindir, `TR_Int` yuvarlanmaz; karışık durumda kazanan değer sonra `TR_Float`'a çevrilir (`TR_max(9007199254740993, 1.0)` → `9007199254740992.0`). Bir NaN varsa sonuç NaN'dır.
- `TR_mean`, `TR_std` ve `TR_median` yalnız liste alır; değerleri tek tek almaz.
- `TR_round(x)` bir `TR_Float`'ı en yakın `TR_Int`'e yuvarlar. Yarım sıfırdan uzağa gider: `TR_round(2.5)` → `3`, `TR_round(-2.5)` → `-3`. NaN ve sonsuz hatadır. `TR_round(x, basamak)` girdinin tipini verir: `TR_round(2.567, 2)` → `2.57`, `TR_round(1234, -2)` → `1200`, `TR_round(1234.0, -2)` → `1200.0`. Basamaklı yuvarlama sayının yazılan ondalık metnine göredir (3.2): `TR_round(2.675, 2)` → `2.68`.
- `TR_wrapping_add(a, b)`, `TR_wrapping_sub(a, b)` ve `TR_wrapping_mul(a, b)` iki `TR_Int`'in sonucunu 64 bitte sarar (3.1); `TR_unsigned_shift_right(x, n)` sıfırla doldurarak sağa kaydırır (4.12). Hash ve rastgele sayı üreteci içindir.
- `TR_print` argümanlarının metnini (3.14) birer boşlukla birleştirip standart çıktıya yazar ve satır sonu ekler; her çağrının satırı bütün olarak, çağrı bitmeden yazılır (eşzamanlı görevlerin satırları karışmaz). `TR_print`, `TR_sleep` ve `TR_wait_all` değer döndürmez (6.1, 13.10).
- `TR_read_file(yol)` dosyayı UTF-8 metin olarak okur; geçerli UTF-8 değilse hatadır, ileti `TR_read_bytes`'ı önerir. `TR_read_bytes(yol)` dosyayı `TR_Bytes` olarak okur (görüntü, ses, ikili veri). `TR_write_file(yol, içerik)` `TR_String`'i UTF-8 olarak, `TR_Bytes`'ı bayt bayt yazar; değerin metnini yazmaz.
- `TR_sleep(saniye)` bir `TR_Float` saniye bekler (`TR_sleep(0.001)` bir milisaniye); çözünürlüğü işletim sisteminindir. `TR_time_ms()` duvar saatidir (Unix zamanı, milisaniye): kayıt ve zaman damgası içindir, saat düzeltilince geri gidebilir. Süre ölçmek ve sabit hızlı döngü için `TR_monotonic_ns()` tekdüze saattir (nanosaniye, `TR_Int`): geri gitmez, sistem saatinden etkilenmez; başlangıç noktası belirsizdir, yalnız iki değerin farkı anlamlıdır.
- `TR_input()` ya da `TR_input(istem)` (istem bir `TR_String`) standart girişten bir satır okur ve satır sonunu atar; giriş bittiyse boş metin verir. Süreçteki bütün okumalar (art arda `TR_input`, etkileşimli kabuk, terminal hata ayıklayıcısı) tek bir tamponlu okuyucudan yapılır: boruyla verilen satırlar sırayla gelir, hiçbiri kaybolmaz.
- *Test:* TR testi; karışık sayıların tipi, boş listenin hatası, tek yazım ve değer döndürmeme uygulanınca TR testi.

**13.1a** Yol alan her yerleşik (`TR_read_file`, `TR_write_file`, `TR_read_csv`, `TR_save_tensors`, `TR_load_tensors`, `TR_load_tokenizer`, `TR_Dataset` ve `TR_cache:`, 18. bölümün yerleşikleri) yolu `TR_String` ya da `TR_Path` (18.4) olarak alır ve tek kuralla çözer:
- Göreli yol önce çalışma klasörüne, orada yoksa kodun bulunduğu modülün klasörüne (11.5), sonra programın klasörüne göre çözülür; dosyanın bulunduğu ilk yer kullanılır. Dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir. Kural okuma ve yazma için aynıdır: programın yanındaki dosya okunup güncellenince yine oraya yazılır, yeni dosya çalışma klasöründe oluşur. `TR_import` bu kuralın istisnasıdır (11.1).
- Dosya bulunamazsa hata bakılan yerleri listeler. Yolda sekme ya da satır sonu varsa dosyaya bakılmadan hata verilir: bunlar çoğunlukla Windows yolundaki ters bölünün kaçış sayılmasıdır (`"C:\tokenizer.json"` içindeki `\t`); ileti ham metni (2.6a) önerir.
- `.gz` uzantılı dosyada sıkıştırma saydamdır (18.12); `TR_write_file` atomik yazar (18.13).
- *Test:* TR testi.

**13.2** Metin metotları: `.TR_length`, `.TR_get(i)` (`TR_String?`), `.TR_upper()`, `.TR_lower()`, `.TR_strip()`, `.TR_split()` ve `.TR_split(ayırıcı)`, `.TR_replace(eski, yeni)`, `.TR_contains(alt)`, `.TR_starts_with(önek)`, `.TR_ends_with(sonek)`, `.TR_reverse()`. Argümanları `TR_String`'dir (`TR_get`'inki `TR_Int`). Ayırıcısız `.TR_split()` Unicode boşluk dizilerine böler, baştaki ve sondaki boşluğu atar, boş öğe vermez (`" a 	 b
".TR_split()` → `["a", "b"]`); ayırıcıyla her geçişte böler, boş öğeleri tutar (`"a,,b".TR_split(",")` → `["a", "", "b"]`); boş ayırıcı hatadır. Harf dönüşümü Unicode'un basit eşlemesidir: her karakter tek bir karaktere döner, metnin uzunluğu değişmez, sonuç her dilde aynıdır (`"istanbul".TR_upper()` → "ISTANBUL", `"ß".TR_upper()` → "ß").
- *Test:* TR testi.

**13.3** Liste metotları: `.TR_length`, `.TR_get(i)` (`T?`, 3.6), `.TR_first` ve `.TR_last` (boş listede `TR_none`, tipi `T?`), `.TR_filter(f)`, `.TR_map(f)`, `.TR_reduce(...)` (13.6), `.TR_sort()`, `.TR_reverse()`, `.TR_join(ayırıcı)` (`TR_List[TR_String]`'te), `.TR_unique()`, `.TR_contains(x)` (4.5'in eşitliğiyle), `.TR_append(x, ...)` (bir ya da daha çok öğe ekler, listeyi yerinde değiştirir, değer döndürmez).
- `.TR_sort()` yeni bir liste verir. Öğeler sıralanabilir olmalıdır (13.10): sayılar tam karşılaştırılır, eşitlerin sırası korunur, `TR_nan` sona gelir; metinler karakter kodu sırasıyladır.
- Listenin toplamı ve ortalaması fonksiyondur: `TR_sum(l)`, `TR_mean(l)` (13.6).
- *Test:* TR testi.

**13.4** Sözlük metotları: `.TR_length`, `.TR_get(k)` (`V?`, 3.7), `.TR_keys()` (`TR_List[K]`), `.TR_values()` (`TR_List[V]`), `.TR_contains(anahtar)`.
- *Test:* TR testi.

**13.5** Bir yerleşik yalnız imzasındaki (13.9) adlı argümanları kabul eder; bilinmeyen adlı argüman sessizce yok sayılmaz, hatadır. İleti geçerli adları sayar ya da yazıma yakın adı önerir; adlı argüman almayan yerleşikte bunu söyler.
- *Test:* TR testi.

**13.6** Metot mu, fonksiyon mu: bir değeri dönüştüren ya da ona soru soran iş metottur (`x.TR_length`, `l.TR_filter(f)`, `l.TR_map(f)`, `l.TR_sort()`, `l.TR_append(x)`, `t.TR_transpose()`); matematik ve tensör işi fonksiyondur (`TR_sum(l)`, `TR_mean(l)`, `TR_min`, `TR_max`, `TR_relu(t)`, `TR_softmax(t)`). Dönüştürmek ve kurmak tipin çağrısıdır (3.12), ayrı bir fonksiyon değildir.
- Her işin tek biçimi vardır: öbür biçim yoktur ("uzunluk(l)" tanımsız addır, "l.toplam()" bilinmeyen üyedir). Bu adlar genel kapsamda sıradan adlardır.
- Aynı adın ayrık imzaları (13.10) tek yazımı bozmaz, çünkü ayrı işlerdir: `TR_max(l)` listenin en büyüğü, `TR_max(a, b)` iki değerin büyüğü. Aynı işin iki yazımı bozar; bu yüzden `TR_mean` yalnız liste alır.
- `l.TR_reduce(başlangıç, f)` listeyi soldan sağa `f(birikim, öğe)` ile tek değere indirger; boş listede başlangıcı verir: `[1, 2, 3].TR_reduce(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk öğe başlangıçtır ve boş listede hatadır (13.1'deki toplama gibi). Fonksiyon son argümandır ve iki parametre alır; sondaki blokla yazılamaz, çünkü sondaki blok parametre almaz (6.7). Tek parametreli fonksiyon ya da başa yazılmış fonksiyon hatadır, ileti doğru sırayı söyler.
- *Test:* TR testi; iletilerin ok yazımı 3. katman birleşince TR testi.

**13.7** Sayı matematiği: `TR_floor(x)` ve `TR_ceil(x)` aşağı ve yukarı en yakın tam sayıya yuvarlar; `TR_sign(x)` −1, 0 ya da 1 verir, NaN'ı korur. Üçü de girdinin tipini verir (`TR_floor(3.7)` → `3.0`, `TR_floor(7)` → `7`). `TR_clamp(x, TR_min: a, TR_max: b)` değeri aralığa çeker. `TR_is_finite(x)` NaN, `TR_inf` ve eksi sonsuzda `TR_false`, öbür her sayıda `TR_true` verir. Beşi tensörde de çalışır, öğe öğe (15. bölüm); listede çalışmaz. `TR_pi` bir sabittir (`TR_Float`): çağrılmaz, yeniden atanamaz. `e` sabiti yoktur, `TR_exp(1.0)` yazılır.
- *Test:* TR testi.

**13.8** Veri biçimleri:
- `TR_json_parse(metin)` bir `TR_JSON` verir (3.7a); `TR_json_string(değer)` değeri JSON metnine çevirir: boşluksuz, anahtarlar eklenme sırasıyla, ASCII dışı karakterler kaçırılmadan UTF-8 (`{"a":1,"ç":"ş
"}`); ondalık 3.2'nin en kısa yazımıyla. JSON'a yalnız veri yazılır: sonlu sayı (NaN ve sonsuz hatadır, JSON'da yoktur), metin, mantık değeri, `TR_none`, liste, `TR_JSON`, `TR_Tensor`, `TR_Path` ve anahtarı metin olan sözlük. Alanları JSON'a yazılabilen bir yapı JSON nesnesi olarak yazılır: alanlar tanım sırasıyla, alanın adı anahtar; isteğe bağlı alanın yokluğu `null`'dır. Fonksiyon, aralık, kitaplık nesnesi ve durumlu tip değeri hatadır. Anahtarı metin olmayan sözlük de hatadır, çünkü JSON'da anahtar metindir ve geri okununca anahtarın tipi değişir; ileti anahtarı `TR_String(k)` ile çevirmeyi önerir.
- `TR_json_parse[Ayar](metin)` JSON nesnesini doğrudan bir yapıya okur (tip parametresi 6.10'daki gibi yazılır): anahtar alanın adıdır; eksik alan varsayılanını alır; varsayılanı olmayan eksik alan, bilinmeyen alan ve tipi tutmayan alan çalışma hatasıdır, ileti alanın yolunu söyler (`ayar.model.katman`). Nesnede yinelenen anahtar her iki biçimde de hatadır (3.7a).
- `TR_read_csv(yol)` bir CSV dosyasını, `TR_parse_csv(metin)` metnin kendisini `TR_List[TR_List[TR_String]]`'e çevirir; hücre hep metindir, sayı gerekirse tip çağrısıyla çevrilir (`TR_Int(hücre)`, 3.12). İkisi ayrıdır, çünkü bir metnin dosya adı mı içerik mi olduğu tahmin edilemez. `TR_read_csv`'ye `TR_Path` verilirse her zaman dosya okunur. `TR_String` olarak satır sonu içeren bir metin ya da virgül içeren ve öyle bir dosyası olmayan bir metin verilirse ileti `TR_parse_csv`'yi önerir.
- Baştaki bayt sırası işareti atılır. Toplam hücre sayısı koleksiyonların sınırındadır; belleğe sığmayan dosya satır satır okunur (18).
- *Test:* TR testi; JSON anahtarının ve metin hücrenin kuralı uygulanınca TR testi.

**13.8a** Dönüşü argümanın değerine bağlı olan üç iş ayrı adlarla yazılır (13.10):
- `t.TR_tolist` 1 boyutlu tensörü `TR_List[TR_Float]`'a çevirir; daha çok boyutlu tensörde hatadır, ileti `t.TR_reshape([-1]).TR_tolist`'i ya da satırları dolaşmayı önerir. Tek öğeli tensörün sayısı `t.TR_item`'dır (`TR_Float`), başka tensörde hatadır. Öğe her zaman `TR_Float`'tır; indeks olarak kullanılacaksa `TR_Int(...)` ile çevrilir (3.12).
- `k.TR_decode(kimlikler)` bir dizi kimliği metne çevirir; birden çok dizi `k.TR_batch_decode(diziler)` ile `TR_List[TR_String]`'e çevrilir.
- `TR_parameters(model)` bir yapının (13.10) ya da bir tensör listesinin parametrelerini `TR_List[TR_Tensor]` olarak verir; adlarıyla birlikte `TR_named_parameters(model)` yalnız yapıda `TR_Dict[TR_String, TR_Tensor]` verir. `TR_named:` argümanı kalkar.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**13.9** Her yerleşiğin (fonksiyon, metot, kitaplık tipinin çağrısı) imzası tek yerdedir: parametrelerin tipleri, adlı argümanlar (ad, tip, varsayılan) ve dönüş tipi.
- Go ile yazılmış yerleşiğin imzası kod kataloğundadır. TR ile yazılmış kitaplık modülünde (`tr/katman` gibi) imza yalnız kaynaktadır; katalog yalnız kodun türünü ve modülünü tutar. Denetleyici ve başvuru belgesi ([`docs/codes.md`](../codes.md)) imzayı ilgili kaynaktan, okuyanın dilinde gösterir.
- Tipler 3. ve 6. bölümün yazımıyla yazılır: öğe tipli (`TR_List[TR_Int]`, `TR_Dict[TR_String, T]`), `T?`, fonksiyon tipi (`(T) -> U`, 6.9), tip parametresi (6.10).
- Yerleşik bir fonksiyon değeridir; tipi imzasının fonksiyon tipidir (6.9). Birden çok imzalı yerleşik değer olarak verilince imzası beklenen tipten seçilir (`l.TR_map(TR_sqrt)`); beklenen tip yoksa denetim hatasıdır.
- 13.9–13.11 öbür bölümlerin yerleşiklerine ve kitaplık tiplerine de uygulanır (10, 15, 16, 18–20).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: kataloğun her Go yerleşiğinin imzası olması ve her kitaplık kodunun modülde dışa açık bir tanımı olması.

**13.10** Bir yerleşiğin sonucunun tipi argümanlarının tipinden bellidir; argümanın değerinden çıkmaz.
- **Kısıtlı tip parametresi.** Her tipte çalışan yerleşiğin parametresi bir tip parametresidir, genel bir tip değildir (3.11): `TR_List[T].TR_map[U](f: (T) -> U) -> TR_List[U]`. Tip parametresi şu kısıtlardan birini taşıyabilir:
  - **sayı:** `TR_Int`, `TR_Float`. Tip parametresi `TR_Int` ve `TR_Float` argümanlardan çıkarsa `TR_Float`'tır ve `TR_Int` değerler `TR_Float`'a çevrilir (4.2): `TR_max(n, 1.5)`, `TR_clamp(15, TR_max: 10.0)`, `TR_atan2(1, 2.0)`. Bu, 3.13'ün "`TR_Int` değer `TR_Float` yerine verilmez" kuralının işleçteki gibi tek istisnasıdır.
  - **sıralanabilir:** sayılar ve `TR_String`; `TR_sort`'ta `TR_nan` sona gelir (13.3), `TR_min` ve `TR_max` ise NaN'ı yayar (13.1);
  - **eşitlenebilir:** 4.5'in `==`'sinin hata vermediği tipler;
  - **anahtar olabilen:** 3.7;
  - **JSON'a yazılabilir:** 13.8'in saydığı tipler, öğeleri de yazılabilen liste ve anahtarı metin olan sözlük;
  - **yapı:** programın ya da kitaplığın herhangi bir yapısı ve arayüz tipli bir değer (7.1, 7.8); durumlu tip değil, çünkü alanları duruma göre değişir: `TR_parameters`, `TR_Checkpoint`'in durumu ve `TR_command_line`'ın seçenekleri bir yapıdır;
  - **fonksiyon:** herhangi bir fonksiyon tipi (6.9); `TR_fuse` bir fonksiyonu alıp aynı tipte bir fonksiyon döndürür;
  - **tensör verisi:** bir sayı ya da öğeleri tensör verisi olan liste, her derinlikte (`TR_Tensor([[1.0, 2.0], [3.0, 4.0]])`).

  Kısıtlar yalnız yerleşiklerin imzasındadır; katalogda İngilizce biçim değeriyle yazılır ("number", "ordered", "equatable", "key", "json", "struct", "function", "tensor_data"), iletide kelimeyle anlatılır. Programın fonksiyonundaki kısıt ve onun kimliği sonraki bölümlerdedir (6.10). Örnekler: `TR_abs` için sayı `T` → `T`; `TR_sum` için sayı `T`'lerin listesi → `T`; `TR_List[T].TR_contains(x: T)`, `T` eşitlenebilir.
- **Birden çok imza.** Sonucun tipi kısıtla anlatılamıyorsa her durum ayrı bir imzadır: sayı ve tensör ayrı (`TR_sqrt(TR_Tensor) -> TR_Tensor`), basamaklı ve basamaksız `TR_round` ayrı (13.1). Dönüş tipi birleşim değildir. İmzalar örtüşmez: her çağrıya en çok bir imza uyar. Önce dönüşümsüz uyan imza aranır; tam sayı yazımı `TR_Float`'a (3.13) yalnız dönüşümsüz uyan yoksa döner. Hiçbir imza uymazsa ya da tip parametresi çıkmazsa denetim hatasıdır: ileti imzaları sayar, boş değişmezde tipi yazmayı önerir (`TR_sum[TR_Float]([])`). Metodun alıcısı da imzanın bir parçasıdır ve örtüşmeme onu da kapsar: `TR_List[TR_String].TR_join(...)` yalnız metin listesinde, `TR_List[T].TR_sort()` sıralanabilir `T`'de. Birden çok imza yalnız yerleşiklerdedir; programın fonksiyonunda bir ad bir tanımdır (6.1).
- **Parametrede kabul kümesi.** Dönüş tipi bütün parçalar için aynıysa bir parametre birkaç tip kabul edebilir (`yol`: `TR_String` ya da `TR_Path`); değilse ayrı imzadır.
- **Her argüman kendi tipinde.** `TR_print` gibi her tipten değer alan yerleşikte her argüman kendi tipindedir ve sonucu etkilemez. `TR_json_string` her tipi değil, JSON'a yazılabileni alır.
- **Anlamsız dönüş yoktur.** İşini yapıp değer vermeyen yerleşik değer döndürmez (6.1): `TR_print`, `TR_write_file`, `TR_assert`, `TR_assert_equal`, `.TR_append`, `.TR_load_state`. `TR_true` ya da alıcının kendisi "başarılı" anlamında döndürülmez. Döndürülen her değerin anlamı kendi bölümünde yazılır; yazılmamışsa yerleşik değer döndürmez.
- **Değişken sayıda argüman.** Yalnız yerleşiğin son parametresi olabilir ve sıfır ya da daha çok değer alır (`TR_print(a, b, c)`). En az biri gerekiyorsa ayrı bir parametre yazılır (`TR_max(a, b, ...)` iki zorunlu, gerisi isteğe bağlı). Programın fonksiyonunda değişken sayıda parametre yoktur (6.5).
- Fonksiyon alan yerleşik fonksiyonun tipini yazar: `TR_filter`'ın fonksiyonu `(T) -> TR_Bool`'dur (3.10), `TR_reduce`'unki `(U, T) -> U`.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: imzaların birleşim dönüş tipi taşımaması ve örtüşmemesi; denetleyicinin testi 4. katmanda.

**13.11** Yerleşiğe yanlış sayıda ya da yanlış tipte argüman vermek hatadır. İleti yerleşiği, argümanın sırasını, beklenen ve verilen tipi söyler ve kullanımı gösterir: "sqrt() expects Int or Float as argument 1, String ("a") given". Hata bugün çalışma anındadır; 4. katmandan sonra denetleyici aynı kodla çalışmadan önce verir.
- *Test:* TR testi.
