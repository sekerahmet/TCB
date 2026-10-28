# 13. Yerleşikler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "13. Yerleşikler".

Her kuralın emsali ve gerekçesi: [`rationale/13_builtins.md`](rationale/13_builtins.md).

**13.1** Genel yerleşikler: `TR_print`, `TR_sum`, `TR_mean`, `TR_std`, `TR_median`, `TR_min`, `TR_max`, `TR_abs`, `TR_sqrt`, `TR_round`, `TR_sleep`, `TR_wait_all`, `TR_time_ms`, `TR_read_file`, `TR_write_file`, `TR_input` ve `TR_error_code` (8.5). `TR_Type`, `TR_Message` ve `TR_String` gibi tip çağrıları fonksiyon değildir (3.12).
- Sayı yerleşiklerinin sonucu işleçlerin tipindedir (3.1, 4.2): `TR_sum(l)` `TR_List[TR_Int]`'te `TR_Int`, `TR_List[TR_Float]`'ta `TR_Float` verir. `TR_mean`, `TR_std` (n'ye bölerek) ve `TR_median` `TR_Float` verir.
- Toplama yapan yerleşikler (`TR_sum` dışında `TR_mean`, `TR_std`, `TR_median`, `TR_min`, `TR_max`) boş listede hata verir.
- `TR_min` ve `TR_max` bir liste ya da iki ve daha çok değer alır. Değerler sıralanabilir olmalıdır (13.10): sayı ya da metin. `TR_Int` ile `TR_Float` karışırsa sonuç `TR_Float`'tır, 4.2'deki gibi: `TR_max(2, 1.5)` → `2.0`. `TR_Int`'leri tam karşılaştırır. Bir NaN varsa sonuç NaN'dır.
- `TR_mean`, `TR_std` ve `TR_median` yalnız liste alır; değerleri tek tek almaz.
- `TR_round(x)` bir `TR_Float`'ı en yakın `TR_Int`'e yuvarlar. Yarım sıfırdan uzağa gider: `TR_round(2.5)` → `3`, `TR_round(-2.5)` → `-3`. NaN ve sonsuz hatadır. `TR_round(x, basamak)` girdinin tipini verir: `TR_round(2.567, 2)` → `2.57`, `TR_round(1234, -2)` → `1200`, `TR_round(1234.0, -2)` → `1200.0`. Basamaklı yuvarlama sayının yazılan ondalık metnine göredir (3.2): `TR_round(2.675, 2)` → `2.68`.
- `TR_print`, `TR_sleep` ve `TR_wait_all` değer döndürmez (6.1).
- `TR_input()` ya da `TR_input(istem)` standart girişten bir satır okur ve satır sonunu atar; giriş bittiyse boş metin verir. Süreçteki bütün okumalar (art arda `TR_input`, etkileşimli kabuk, terminal hata ayıklayıcısı) tek bir tamponlu okuyucudan yapılır: boruyla verilen satırlar sırayla gelir, hiçbiri kaybolmaz.
- *Test:* TR testi; karışık sayıların tipi, boş listenin hatası, tek yazım ve değer döndürmeme uygulanınca TR testi.

**13.1a** Yol alan her yerleşik (`TR_read_file`, `TR_write_file`, `TR_read_csv`, `TR_save_tensors`, `TR_load_tensors`, `TR_load_tokenizer`, `TR_Dataset` ve `TR_cache:`, 18. bölümün yerleşikleri) yolu `TR_String` ya da `TR_Path` (18.4) olarak alır ve tek kuralla çözer:
- Göreli yol önce çalışma klasörüne, orada yoksa kodun bulunduğu modülün klasörüne (11.5), sonra programın klasörüne göre çözülür; dosyanın bulunduğu ilk yer kullanılır. Dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir. Kural okuma ve yazma için aynıdır: programın yanındaki dosya okunup güncellenince yine oraya yazılır, yeni dosya çalışma klasöründe oluşur. `TR_import` bu kuralın istisnasıdır (11.1).
- Dosya bulunamazsa hata bakılan yerleri listeler. Yolda sekme ya da satır sonu varsa dosyaya bakılmadan hata verilir: bunlar çoğunlukla Windows yolundaki ters bölünün kaçış sayılmasıdır (`"C:\tokenizer.json"` içindeki `\t`); ileti ham metni (2.6a) önerir.
- `.gz` uzantılı dosyada sıkıştırma saydamdır (18.12); `TR_write_file` atomik yazar (18.13).
- *Test:* TR testi.

**13.2** Metin metotları: `.TR_length`, `.TR_get(i)` (`TR_String?`), `.TR_upper()`, `.TR_lower()`, `.TR_strip()`, `.TR_split(ayırıcı)` (ayırıcı verilmezse boşluk), `.TR_replace(eski, yeni)`, `.TR_contains(alt)`, `.TR_starts_with(önek)`, `.TR_ends_with(sonek)`, `.TR_reverse()`. Argümanları `TR_String`'dir. Harf dönüşümü Unicode'un basit eşlemesidir: her karakter tek bir karaktere döner, metnin uzunluğu değişmez, sonuç her dilde aynıdır (`"istanbul".TR_upper()` → "ISTANBUL", `"ß".TR_upper()` → "ß").
- *Test:* TR testi.

**13.3** Liste metotları: `.TR_length`, `.TR_get(i)` (`T?`, 3.6), `.TR_first` ve `.TR_last` (boş listede `TR_none`, tipi `T?`), `.TR_filter(f)`, `.TR_map(f)`, `.TR_reduce(...)` (13.6), `.TR_sort()`, `.TR_reverse()`, `.TR_join(ayırıcı)` (`TR_List[TR_String]`'te), `.TR_unique()`, `.TR_contains(x)` (4.5'in eşitliğiyle), `.TR_append(x, ...)` (bir ya da daha çok öğe ekler, listeyi yerinde değiştirir).
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
- `l.TR_reduce(başlangıç, f)` listeyi soldan sağa `f(birikim, öğe)` ile tek değere indirger; boş listede başlangıcı verir: `[1, 2, 3].TR_reduce(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk öğe başlangıçtır ve sonuç `T?`'dir, boş listede `TR_none`. Fonksiyon son argümandır ve iki parametre alır; sondaki blokla yazılamaz, çünkü sondaki blok parametre almaz (6.7). Tek parametreli fonksiyon ya da başa yazılmış fonksiyon hatadır, ileti doğru sırayı söyler.
- *Test:* TR testi; iletilerin ok yazımı 3. katman birleşince TR testi.

**13.7** Sayı matematiği: `TR_floor(x)` ve `TR_ceil(x)` aşağı ve yukarı en yakın tam sayıya yuvarlar; `TR_sign(x)` −1, 0 ya da 1 verir, NaN'ı korur. Üçü de girdinin tipini verir (`TR_floor(3.7)` → `3.0`, `TR_floor(7)` → `7`). `TR_clamp(x, TR_min: a, TR_max: b)` değeri aralığa çeker. `TR_is_finite(x)` NaN, `TR_inf` ve eksi sonsuzda `TR_false`, öbür her sayıda `TR_true` verir. Beşi tensörde de çalışır, öğe öğe (15. bölüm); listede çalışmaz. `TR_pi` bir sabittir (`TR_Float`): çağrılmaz, yeniden atanamaz. `e` sabiti yoktur, `TR_exp(1.0)` yazılır.
- *Test:* TR testi.

**13.8** Veri biçimleri:
- `TR_json_parse(metin)` bir `TR_JSON` verir (3.7a); `TR_json_string(değer)` değeri JSON metnine çevirir. JSON'a yalnız veri yazılır: sayı, metin, mantık değeri, `TR_none`, liste, `TR_JSON`, `TR_Tensor`, `TR_Path` ve anahtarı metin olan sözlük. Fonksiyon, aralık, yapı ve kitaplık nesnesi hatadır. Anahtarı metin olmayan sözlük de hatadır, çünkü JSON'da anahtar metindir ve geri okununca anahtarın tipi değişir; ileti anahtarı `TR_String(k)` ile çevirmeyi önerir.
- `TR_read_csv(yol)` bir CSV dosyasını, `TR_parse_csv(metin)` metnin kendisini `TR_List[TR_List[TR_String]]`'e çevirir; hücre hep metindir, sayı gerekirse tip çağrısıyla çevrilir (`TR_Int(hücre)`, 3.12). İkisi ayrıdır, çünkü bir metnin dosya adı mı içerik mi olduğu tahmin edilemez. `TR_read_csv`'ye `TR_Path` verilirse her zaman dosya okunur. `TR_String` olarak satır sonu içeren bir metin ya da virgül içeren ve öyle bir dosyası olmayan bir metin verilirse ileti `TR_parse_csv`'yi önerir.
- Baştaki bayt sırası işareti atılır. Toplam hücre sayısı koleksiyonların sınırındadır.
- *Test:* TR testi; JSON anahtarının ve metin hücrenin kuralı uygulanınca TR testi.

**13.9** Her yerleşiğin (fonksiyon, metot, kitaplık tipinin çağrısı) imzası tek yerdedir: parametrelerin tipleri, adlı argümanlar (ad, tip, varsayılan) ve dönüş tipi.
- Go ile yazılmış yerleşiğin imzası kod kataloğundadır. TR ile yazılmış kitaplık modülünde (`tr/katman` gibi) imza yalnız kaynaktadır; katalog yalnız kodun türünü ve modülünü tutar. Denetleyici ve başvuru belgesi ([`docs/codes.md`](../codes.md)) imzayı ilgili kaynaktan, okuyanın dilinde gösterir.
- Tipler 3. ve 6. bölümün yazımıyla yazılır: öğe tipli (`TR_List[TR_Int]`, `TR_Dict[TR_String, T]`), `T?`, fonksiyon tipi (`(T) -> U`, 6.9), tip parametresi (6.10).
- Yerleşik bir fonksiyon değeridir; tipi imzasının fonksiyon tipidir (6.9). Birden çok imzalı yerleşik değer olarak verilince imzası beklenen tipten seçilir (`l.TR_map(TR_sqrt)`); beklenen tip yoksa denetim hatasıdır.
- 13.9–13.11 öbür bölümlerin yerleşiklerine ve kitaplık tiplerine de uygulanır (10, 15, 16, 18–20).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: kataloğun her Go yerleşiğinin imzası olması ve her kitaplık kodunun modülde dışa açık bir tanımı olması.

**13.10** Bir yerleşiğin sonucunun tipi argümanlarının tipinden bellidir; argümanın değerinden çıkmaz.
- **Kısıtlı tip parametresi.** Her tipte çalışan yerleşiğin parametresi bir tip parametresidir, genel bir tip değildir (3.11): `TR_List[T].TR_map[U](f: (T) -> U) -> TR_List[U]`. Tip parametresi şu kısıtlardan birini taşıyabilir:
  - **sayı:** `TR_Int`, `TR_Float`;
  - **sıralanabilir:** sayılar ve `TR_String`;
  - **eşitlenebilir:** 4.5'in `==`'sinin hata vermediği tipler;
  - **anahtar olabilen:** 3.7.

  Kısıtlar yalnız yerleşiklerin imzasındadır; programın fonksiyonundaki kısıt sonraki bölümlerdedir (6.10). Örnekler: `TR_abs` için sayı `T` → `T`; `TR_sum` için sayı `T`'lerin listesi → `T`; `TR_List[T].TR_contains(x: T)`, `T` eşitlenebilir.
- **Birden çok imza.** Sonucun tipi kısıtla anlatılamıyorsa her durum ayrı bir imzadır: sayı ve tensör ayrı (`TR_sqrt(TR_Tensor) -> TR_Tensor`), basamaklı ve basamaksız `TR_round` ayrı (13.1). Dönüş tipi birleşim değildir. İmzalar örtüşmez: her çağrıya en çok bir imza uyar. Önce dönüşümsüz uyan imza aranır; tam sayı yazımı `TR_Float`'a (3.13) yalnız dönüşümsüz uyan yoksa döner. Hiçbir imza uymazsa ya da tip parametresi çıkmazsa denetim hatasıdır: ileti imzaları sayar, boş değişmezde tipi yazmayı önerir (`TR_sum[TR_Float]([])`). Birden çok imza yalnız yerleşiklerdedir; programın fonksiyonunda bir ad bir tanımdır (6.1).
- **Parametrede kabul kümesi.** Dönüş tipi bütün parçalar için aynıysa bir parametre birkaç tip kabul edebilir (`yol`: `TR_String` ya da `TR_Path`); değilse ayrı imzadır.
- **Her argüman kendi tipinde.** `TR_print` ve `TR_json_string` gibi her tipten değer alan yerleşikte her argüman kendi tipindedir ve sonucu etkilemez.
- **Değişken sayıda argüman.** Yalnız yerleşiğin son parametresi olabilir ve sıfır ya da daha çok değer alır (`TR_print(a, b, c)`). En az biri gerekiyorsa ayrı bir parametre yazılır (`TR_max(a, b, ...)` iki zorunlu, gerisi isteğe bağlı). Programın fonksiyonunda değişken sayıda parametre yoktur (6.5).
- Fonksiyon alan yerleşik fonksiyonun tipini yazar: `TR_filter`'ın fonksiyonu `(T) -> TR_Bool`'dur (3.10), `TR_reduce`'unki `(U, T) -> U`.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: imzaların birleşim dönüş tipi taşımaması ve örtüşmemesi; denetleyicinin testi 4. katmanda.

**13.11** Yerleşiğe yanlış sayıda ya da yanlış tipte argüman vermek hatadır. İleti yerleşiği, argümanın sırasını, beklenen ve verilen tipi söyler ve kullanımı gösterir: "sqrt() expects Int or Float as argument 1, String ("a") given". Hata bugün çalışma anındadır; 4. katmandan sonra denetleyici aynı kodla çalışmadan önce verir.
- *Test:* TR testi.
