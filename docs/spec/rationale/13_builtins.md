# 13. Yerleşikler: emsal ve gerekçe

Kurallar: [`13_yerlesikler.md`](../13_yerlesikler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 13.1

- *Emsal:* Go'da `min` ve `max`'ın tipi işlecin tipidir: "the type of min(x, y) is the type of x + y"; NaN'da "if any argument is a NaN, the result is a NaN". Python'da boş listenin en büyüğü hatadır: "If the iterable is empty and default is not provided, a ValueError is raised". `round`'un tipi Python'daki gibidir: "The return value is an integer if ndigits is omitted or None. Otherwise, the return value has the same type as number". Yarım Go'daki gibi sıfırdan uzağa gider: "Round returns the nearest integer, rounding half away from zero". *Almadık:* Python'un yarımı çifte yuvarlaması ("both round(0.5) and round(-0.5) are 0, and round(1.5) is 2"): istatistikte doğrudur, ama okul kuralından ayrılır. Python'un basamaklı yuvarlamasının ikili değere göre çalışması (`round(2.675, 2)` → 2.67): TR yazılan ondalık metne göre yuvarlar. Kazanan değerin tipini vermek (bugünkü TR): `TR_max(2, 1.5)`'in tipi değere göre değişir ve 1.2'nin "her değerin tipi çalışmadan önce bilinir"iyle yazılamaz. `TR_mean(1, 2)` gibi ikinci bir yazım. ([Go 1.27 spec](https://go.dev/ref/spec#Min_and_max), [Go 1.27 math](https://pkg.go.dev/math#Round), [Python 3.15](https://docs.python.org/3/library/functions.html#round))
- *Gerekçe:* sayı yerleşiklerinin tipi işleçlerin tipiyle aynıdır (3.1, 4.2): `TR_max(a, b)` ile `TR_if a > b { a } TR_else { b }` aynı tipi verir. Boş listenin toplamı (`TR_sum` dışında) bir hatadır; `TR_none` dönseydi çağıran `?? 0` yazar ve boş girdinin hatası sessizce kaybolurdu. `TR_round(x)`'in `TR_Int` vermesi yuvarlamanın asıl işidir (indeks, adım sayısı); `TR_floor` ve `TR_ceil` tipi korur (13.7), çünkü onlar sayıyı tam sayıya değil aşağı ya da yukarı bir değere götürür. Yazılan ondalığa göre yuvarlama okuyanın gördüğü sayıyı yuvarlar. Değer döndürmeyen `TR_print`, değerinin yanlışlıkla kullanılmasını çalışmadan önce hata yapar. Tek tamponlu okuyucu, boruyla gelen satırların okuyucular arasında kaybolmasını önler.

### 13.1a

- *Emsal:* Python ve Go göreli yolu yalnız çalışma klasörüne göre çözer [kanıtsız]. *Almadık:* yalnız çalışma klasörü: program başka bir klasörden çalıştırılınca yanındaki dosyayı bulamaz ve hata programın neden çalışmadığını söylemez.
- *Gerekçe:* aynı program hangi klasörden çalıştırılırsa çalıştırılsın kendi dosyasını bulur ve yazdığı yer okuduğu yerdir. Sekmeli yolun dosyaya bakılmadan reddedilmesi, Windows yolundaki kaçış hatasını "dosya yok" gibi yanıltıcı bir hataya dönüştürmez.

### 13.2

- *Emsal:* Go'nun basit eşlemesi: "ToUpper returns s with all Unicode letters mapped to their upper case". *Almadık:* Python'un tam dönüşümü: "The uppercasing algorithm used is described in section 3.13.2 'Default Case Conversion' of the Unicode Standard"; orada "ß" "SS" olur, metnin uzunluğu değişir. Yerele göre dönüşüm (Türkçenin noktalı I'sı): aynı program makinenin diline göre başka sonuç verirdi. ([Go 1.27 strings](https://pkg.go.dev/strings#ToUpper), [Python 3.15](https://docs.python.org/3/library/stdtypes.html#str.upper))
- *Gerekçe:* değerin metni dilden bağımsızdır (3.14); harf dönüşümü her makinede aynı sonucu verir ve uzunluğu korur, böylece indeksler dönüşümden sonra da geçerlidir.

### 13.3

- *Emsal:* Python'un `sorted`'ı yeni bir liste verir: "The built-in sorted() function is guaranteed to be stable". *Almadık:* Python'un `list.sort()`'u ile `sorted`'ın iki yazımı; karışık tiplerin sıralanması. ([Python 3.15](https://docs.python.org/3/library/functions.html#sorted))
- *Gerekçe:* sıralamanın tek yazımı yeni bir liste verir; yerinde değiştiren tek liste metodu `TR_append`'tir. `TR_first`, `TR_last` ve `TR_get` okumadır, toplama değil: boş liste ya da olmayan indeks bir hata değil olağan bir durumdur, `T?` onu çalışmadan önce görünür kılar (4. katman).

### 13.4

- *Emsal:* Python'un `dict.keys()` ve `values()`'ı. *Almadık:* görünüm nesneleri: TR'de sonuç bir listedir.
- *Gerekçe:* sözlüğün anahtarları ve değerleri sıradan bir listeyle dolaşılır; `TR_get` 3.7'deki gibi `V?` verir.

### 13.5

- *Emsal:* Python: "If any keyword argument does not correspond to a formal parameter name, a TypeError exception is raised". *Almadık:* bilinmeyen seçeneğin sessizce yok sayılması (JavaScript'in seçenek nesnesi) [kanıtsız]. ([Python 3.15 başvuru](https://docs.python.org/3/reference/expressions.html#calls))
- *Gerekçe:* yanlış yazılmış bir seçenek (`TR_lr` yerine "ogrenme_orani") sessizce yok sayılırsa eğitim varsayılan değerle koşar ve sonuç hata vermeden yanlış olur.

### 13.6

- *Emsal:* Rust'ta değeri dönüştüren iş metottur (`iter.map`, `v.len()`). Python'un `functools.reduce`'u başlangıcı isteğe bağlı alır: "If the optional initial is present, it is placed before the items of the iterable in the calculation, and serves as a default when the iterable is empty". Go `max(a, b)` ile `slices.Max(s)`'i ayrı adla, Kotlin `maxOf(a, b)` ile `list.max()`'ı ayrı adla ayırır [kanıtsız]. *Almadık:* Python'un `len(x)` ve `x.__len__()` gibi iki yazımı; `reduce`'un fonksiyonu başa alan sırası: TR'de fonksiyon son argümandır (6.7). ([Python 3.15 functools](https://docs.python.org/3/library/functools.html#functools.reduce))
- *Gerekçe:* her işin tek biçimi olunca ajan yazımı tahmin etmez; yanlış biçim tanımsız ad ya da bilinmeyen üyedir ve ileti doğrusunu söyler. Listenin en büyüğü ile iki değerin büyüğü ayrı iki iştir; başka diller ad, TR imza ile ayırır. Sondaki bloğun parametre almaması (6.7) yüzünden iki parametreli `TR_reduce` ok fonksiyonuyla yazılır.

### 13.7

- *Emsal:* Rust'ın `f64::floor`'u `f64` verir (tip korunur). *Almadık:* Python'un tip değiştiren `math.floor`'u ("Floor of x, the largest integer less than or equal to x", `int`); tek harfli `e` sabiti. ([Rust 1.99 f64](https://doc.rust-lang.org/std/primitive.f64.html#method.floor), [Python 3.15 math](https://docs.python.org/3/library/math.html#math.floor))
- *Gerekçe:* `TR_floor` ve `TR_ceil` bir değeri aşağı ya da yukarı götürür ve NaN'ı korur; tam sayı istenirse `TR_round` ya da tip çağrısı yazılır. Tek harfli bir yerleşik ad kullanıcı değişkenleriyle çakışırdı.

### 13.8

- *Emsal:* Python'un `csv` modülü: "Each row read from the csv file is returned as a list of strings. No automatic data type conversion is performed". Python'un `json`'ı metin olmayan anahtarı sessizce metne çevirir ve kendi belgesi uyarır: "if a dictionary is converted into JSON and then back into a dictionary, the dictionary may not equal the original one". *Almadık:* bu sessiz çevirme; hücreyi sayıya benziyorsa sayı yapan eski TR kuralı: bir satır karışık tipli bir liste olur ve 3.6'nın tek tipli listesiyle yazılamaz. ([Python 3.15 csv](https://docs.python.org/3/library/csv.html#csv.reader), [Python 3.15 json](https://docs.python.org/3/library/json.html))
- *Gerekçe:* `TR_json_parse`'ın `TR_JSON` vermesi dış verinin tipini 3.7a'daki gibi açmaya zorlar. Metin hücre "007" gibi kimlikleri bozmaz ve satırın tipi her zaman `TR_List[TR_String]`'dir; sayıya çevirmek açık ve tek yerdedir. Metin olmayan anahtarın hatası, geri okunan sözlüğün sessizce başka anahtarlar taşımasını önler.

### 13.9

- *Emsal:* TypeScript'in yerleşik kitaplığı imzaları bildirim dosyalarında, Python'un yerleşiklerininkiler typeshed'de tutulur [kanıtsız]. *Almadık:* imzanın yalnız belgede yazılması (belge ile uygulama ayrışır); TR ile yazılmış modülün imzasının katalogda bir kopyası (iki kopya ayrışır, `TestLibraryCodeHasNoSignatureInDefinitions` bunu yasaklar).
- *Gerekçe:* denetleyici (4. katman), düzenleyici, başvuru belgesi ve ajan aynı imzayı okur; imza tek yerde olunca ayrışamaz. Yerleşiğin bir fonksiyon değeri olması, onu argüman olarak veren kodu (`l.TR_map(TR_sqrt)`) programın kendi fonksiyonuyla aynı kurala bağlar.

### 13.10

- *Emsal:* Go'nun kısıtları: "The predeclared interface type comparable denotes the set of all non-interface types that are strictly comparable" ve `cmp.Ordered`: "Ordered is a constraint that permits any ordered type: any type that supports the operators < <= >= >". TypeScript: "we can specify a function that can be called in different ways by writing _overload signatures_"; Python: "The @overload decorator allows describing functions and methods that support multiple different combinations of argument types". Rust'ın `map`'i tip parametreleriyle yazılır: `fn map<B, F>(self, f: F) -> Map<Self, F> where Self: Sized, F: FnMut(Self::Item) -> B`. *Almadık:* birleşim dönüş tipi (bugünkü katalogda 35 yerleşik): sonucu kullanan her yer tipi yeniden sınamak zorunda kalır. Örtüşen imzalar arasında sıra (Python ilkini seçer) ya da belirginlik (Kotlin) kuralı: imzalar örtüşmezse ajan bir kural öğrenmez. Programın fonksiyonunda birden çok imza ve değişken sayıda parametre: TR'de bir ad bir tanımdır (6.1), liste parametresi tek yazımdır (6.5). ([Go 1.27 spec](https://go.dev/ref/spec#Type_constraints), [Go 1.27 cmp](https://pkg.go.dev/cmp#Ordered), [TypeScript el kitabı](https://www.typescriptlang.org/docs/handbook/2/functions.html#function-overloads), [Python 3.15 typing](https://docs.python.org/3/library/typing.html#typing.overload), [Rust 1.99 Iterator](https://doc.rust-lang.org/std/iter/trait.Iterator.html#method.map))
- *Gerekçe:* `TR_sum([1, 2])`'nin tipi `TR_Int` ise sonucu `TR_Int` isteyen yere doğrudan verilir; birleşimde her kullanım ya hata verir ya da sessizce genel tipe düşer. Sayı kısıtı `TR_Int` ve `TR_Float` için tek imza verir; ayrı iki imza boş listede (`TR_sum([])`) ikisine de uyardı. Dönüşümsüz imzanın önce aranması `TR_abs(1)`'i `TR_Int` tutar. Genel tip olmadığı için (3.11) her tipi alan yerleşik tip parametresiyle yazılır ve sonucun tipi kaybolmaz.

### 13.11

- *Emsal:* Python'un yerleşikleri yanlış argümanda `TypeError` verir ve fonksiyonu adlandırır [kanıtsız]. *Almadık:* JavaScript'in eksik argümanı `undefined` sayması [kanıtsız].
- *Gerekçe:* ileti yerleşiği, sırayı, beklenen ve verilen tipi söyleyince ajan düzeltmeyi tahmin etmez; aynı kodun 4. katmanda çalışmadan önce verilmesi iletinin değişmemesini sağlar.
