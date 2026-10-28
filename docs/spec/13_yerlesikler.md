# 13. Yerleşikler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "13. Yerleşikler".

Her kuralın emsali ve gerekçesi: [`rationale/13_builtins.md`](rationale/13_builtins.md).

**13.1** Genel yerleşikler: `TR_print`, `TR_Type`, `TR_sum`, `TR_mean`, `TR_std`, `TR_median`, `TR_min`, `TR_max`, `TR_abs`, `TR_sqrt`, `TR_round`, `TR_sleep`, `TR_wait_all`, `TR_time_ms`, `TR_read_file`, `TR_write_file`, `TR_input`, `TR_Message` ve `TR_error_code` (8.5). Sayı yerleşikleri işleçlerin kurallarına uyar (3.1, 4.2):
- `TR_sum(l)` öğeler `TR_Int` ise `TR_Int`, `TR_Float` ise `TR_Float` verir. `TR_mean`, `TR_std` (n'ye bölerek) ve `TR_median` `TR_Float` verir; boş listede hatadır.
- `TR_min` ve `TR_max` ya tek bir liste ya da iki ve daha çok sayı alır ve kazanan değeri tipiyle verir (`TR_max([2.0, 1.5])` → `2.0`); `TR_Int`'leri tam karşılaştırır; boş listede `TR_none` verir, dönüş tipi `T?`'dir.
- `TR_round(x, basamak)` yarımı sıfırdan uzağa yuvarlar (`TR_round(2.5)` → `3`, `TR_round(-2.5)` → `-3`); basamak pozitifse `TR_Float`, 0 ya da negatifse `TR_Int` verir (`TR_round(1234, -2)` → `1200`).
- `TR_input(istem)` standart girişten bir satır okur, satır sonunu atar; giriş bittiyse boş metin verir. Süreçteki bütün okumalar (art arda `TR_input`, etkileşimli kabuk, terminal hata ayıklayıcısı) tek bir tamponlu okuyucudan yapılır: boruyla verilen satırlar sırayla gelir, hiçbiri kaybolmaz.
- *Test:* TR testi.

**13.1a** Yol alan her yerleşik (`TR_read_file`, `TR_write_file`, `TR_read_csv`, `TR_save_tensors`, `TR_load_tensors`, `TR_load_tokenizer`, `TR_Dataset` ve `TR_cache:`, 18. bölümün yerleşikleri) yolu `TR_String` ya da `TR_Path` (18.4) olarak alır ve tek kuralla çözer:
- Göreli yol önce çalışma klasörüne, orada yoksa kodun bulunduğu modülün klasörüne (11.5), sonra programın klasörüne göre çözülür; dosyanın bulunduğu ilk yer kullanılır. Dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir. Kural okuma ve yazma için aynıdır: programın yanındaki dosya okunup güncellenince yine oraya yazılır, yeni dosya çalışma klasöründe oluşur. `TR_import` bu kuralın istisnasıdır (11.1).
- Dosya bulunamazsa hata bakılan yerleri listeler. Yolda sekme ya da satır sonu varsa dosyaya bakılmadan hata verilir: bunlar çoğunlukla Windows yolundaki ters bölünün kaçış sayılmasıdır (`"C:\tokenizer.json"` içindeki `\t`); ileti ham metni (2.6a) önerir.
- `.gz` uzantılı dosyada sıkıştırma saydamdır (18.12); `TR_write_file` atomik yazar (18.13).
- *Test:* TR testi.

**13.2** Metin metotları: `.TR_length`, `.TR_upper()` ve `.TR_lower()` (Unicode'un varsayılan harf dönüşümüyle, her dilde aynı: `"istanbul".TR_upper()` → "ISTANBUL"), `.TR_strip()`, `.TR_split(ayırıcı)` (ayırıcı verilmezse boşluk), `.TR_replace(eski, yeni)`, `.TR_contains(alt)`, `.TR_starts_with(önek)`, `.TR_ends_with(sonek)`, `.TR_reverse()`. Argümanları `TR_String`'dir.
- *Test:* TR testi.

**13.3** Liste metotları: `.TR_length`, `.TR_first` ve `.TR_last` (boş listede `TR_none`, tipi `T?`), `.TR_filter(f)`, `.TR_map(f)`, `.TR_reduce(...)` (13.6), `.TR_sort()`, `.TR_reverse()`, `.TR_join(ayırıcı)`, `.TR_unique()`, `.TR_contains(x)` (4.5'in eşitliğiyle), `.TR_append(x)` (listeyi yerinde değiştirir).
- `.TR_sort()` yeni bir liste verir. Öğeler sayı ya da metindir; sayılar tam karşılaştırılır, eşitlerin sırası korunur, `TR_nan` sona gelir; metinler karakter kodu sırasıyladır. Başka öğe tipi hatadır.
- Listenin toplamı ve ortalaması fonksiyondur: `TR_sum(l)`, `TR_mean(l)` (13.6).
- *Test:* TR testi.

**13.4** Sözlük metotları: `.TR_keys()`, `.TR_values()`, `.TR_contains(anahtar)`.
- *Test:* TR testi.

**13.5** Bir yerleşik yalnız imzasındaki (13.9) adlı argümanları kabul eder; bilinmeyen adlı argüman sessizce yok sayılmaz, hatadır. İleti geçerli adları sayar ya da yazıma yakın adı önerir; adlı argüman almayan yerleşikte bunu söyler.
- *Test:* TR testi.

**13.6** Metot mu, fonksiyon mu: bir değeri dönüştüren ya da ona soru soran iş metottur (`x.TR_length`, `l.TR_filter(f)`, `l.TR_map(f)`, `l.TR_sort()`, `l.TR_append(x)`, `t.TR_transpose()`); matematik ve tensör işi fonksiyondur (`TR_sum(l)`, `TR_mean(l)`, `TR_min`, `TR_max`, `TR_relu(t)`, `TR_softmax(t)`). Dönüştürmek ve kurmak tipin çağrısıdır (3.12), ayrı bir fonksiyon değildir; `TR_Type(x)` değerin tipini verir (3.11).
- Her işin tek biçimi vardır: öbür biçim yoktur ("uzunluk(l)" tanımsız addır, "l.toplam()" bilinmeyen üyedir). Bu adlar genel kapsamda sıradan adlardır.
- `l.TR_reduce(başlangıç, f)` listeyi soldan sağa `f(birikim, öğe)` ile tek değere indirger; boş listede başlangıcı verir: `[1, 2, 3].TR_reduce(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk öğe başlangıçtır ve sonuç `T?`'dir, boş listede `TR_none`. Fonksiyon son argümandır ve iki parametre alır; sondaki blokla yazılamaz, çünkü sondaki blok parametre almaz (6.7). Tek parametreli fonksiyon ya da başa yazılmış fonksiyon hatadır, ileti doğru sırayı söyler.
- *Test:* TR testi.

**13.7** Sayı matematiği: `TR_floor(x)` ve `TR_ceil(x)` aşağı ve yukarı en yakın tam sayıya yuvarlar; `TR_sign(x)` −1, 0 ya da 1 verir, NaN'ı korur. Üçü de `TR_Int` girdide `TR_Int`, `TR_Float` girdide `TR_Float` verir (`TR_floor(3.7)` → `3.0`). `TR_clamp(x, TR_min: a, TR_max: b)` değeri aralığa çeker. Dördü tensörde de çalışır (15. bölüm), listede çalışmaz. `TR_is_finite(x)` NaN, `TR_inf` ve eksi sonsuzda `TR_false`, öbür her sayıda `TR_true` verir. `TR_pi` bir sabittir (`TR_Float`): çağrılmaz, yeniden atanamaz. `e` sabiti yoktur, `TR_exp(1.0)` yazılır.
- *Test:* TR testi.

**13.8** Veri biçimleri:
- `TR_json_parse(metin)` bir `TR_JSON` verir (3.7a); `TR_json_string(değer)` değeri JSON metnine çevirir. JSON'a yalnız veri yazılır (sayı, metin, mantık değeri, `TR_none`, liste, metin anahtarlı sözlük, `TR_JSON`, `TR_Tensor`, `TR_Path`); fonksiyon, aralık, yapı ve kitaplık nesnesi hatadır.
- `TR_read_csv(yol)` bir CSV dosyasını satır listelerine çevirir; `TR_parse_csv(metin)` metnin kendisini çevirir, dosyaya bakmaz. İkisi ayrıdır, çünkü bir metnin dosya adı mı içerik mi olduğu tahmin edilemez. `TR_read_csv`'ye satır sonu içeren bir metin ya da virgül içeren ve öyle bir dosyası olmayan bir metin verilirse ileti `TR_parse_csv`'yi önerir.
- Baştaki bayt sırası işareti atılır. Hücre, boşlukları kırpılınca katı sayı biçimindeyse sayıdır (`42`, `-7` `TR_Int`; `1.0`, `2.5e3` `TR_Float`), öbür her hücre `TR_String`'dir ("007", "NaN", "+5", ".5", `TR_Int`'e sığmayan sayı).
- *Test:* TR testi.

**13.9** Her yerleşiğin (fonksiyon, metot, kitaplık tipinin çağrısı) imzası kod kataloğundadır: parametrelerin tipleri, adlı argümanlar (ad, tip, varsayılan) ve dönüş tipi. Tipler 3. bölümün yazımıyla yazılır: öğe tipli (`TR_List[TR_Int]`, `TR_Dict[TR_String, T]`), `T?`, fonksiyon tipi (`(T) -> U`, 3.9) ve tip parametresi (6.10). Kitaplığın TR ile yazılmış modüllerinde (`tr/katman` gibi) imza kaynağın kendisidir; katalog onunla aynı olmalıdır. Başvuru belgesi ([`docs/codes.md`](../codes.md)) imzaları katalogdan, okuyanın dilinde gösterir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: kataloğun her yerleşiğinin imzası olması ve TR modülleriyle aynı olması.

**13.10** Bir yerleşiğin sonucunun tipi argümanlarının tipinden bellidir:
- Girdinin tipine göre sonucu değişen yerleşiğin her durumu ayrı bir imzadır: `TR_sum(TR_List[TR_Int]) -> TR_Int`, `TR_sum(TR_List[TR_Float]) -> TR_Float`, `TR_sum(TR_Tensor) -> TR_Tensor`. Dönüş tipi birleşim değildir ("Int ya da Float ya da Tensor" yazılmaz). Çağrıya tek bir imza uyar; hiçbiri uymazsa ileti uyan imzaları sayar. Birden çok imza yalnız yerleşiklerdedir; programın fonksiyonunda bir ad bir tanımdır (6.1).
- Her tipte çalışan yerleşiğin parametresi bir tip parametresidir, genel bir tip değildir (3.11): `TR_List[T].TR_map[U](f: (T) -> U) -> TR_List[U]`, `TR_List[T].TR_first -> T?`, `TR_print` her tipi alır ve değerin metnini (3.14) yazar.
- Fonksiyon alan yerleşik fonksiyonun tipini yazar: `TR_filter`'ın fonksiyonu `(T) -> TR_Bool`'dur (3.10), `TR_reduce`'unki `(U, T) -> U`.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go testi: imzaların birleşim dönüş tipi taşımaması ve çağrıya tek imzanın uyması; denetleyicinin testi 4. katmanda.

**13.11** Yerleşiğe yanlış sayıda ya da yanlış tipte argüman vermek hatadır. İleti yerleşiği, argümanın sırasını, beklenen ve verilen tipi söyler ve kullanımı gösterir: "sqrt() expects Int or Float as argument 1, String ("a") given". Hata bugün çalışma anındadır; 4. katmandan sonra denetleyici aynı kodla çalışmadan önce verir.
- *Test:* TR testi.
