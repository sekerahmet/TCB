# 13. Yerleşikler: emsal ve gerekçe

Kurallar: [`13_yerlesikler.md`](../13_yerlesikler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 13.1

- *Emsal:* Rust ve Go yarımı sıfırdan uzağa yuvarlar; Rust: "If a value is half-way between two integers, round away from 0.0". Python'un `max`'ı iki yazım alır: "If one positional argument is provided, it should be an iterable … If two or more positional arguments are provided, the largest of the positional arguments is returned". *Almadık:* Python'un yarımı çifte yuvarlaması: "rounding is done toward the even choice (so, for example, both round(0.5) and round(-0.5) are 0, and round(1.5) is 2)". Bankacının yuvarlaması istatistikte doğrudur, ama okul kuralından ayrılır ve `TR_round(2.5)`'in 2 vermesi okuyanı şaşırtır. ([Rust 1.99 f64](https://doc.rust-lang.org/std/primitive.f64.html#method.round), [Python 3.15](https://docs.python.org/3/library/functions.html#round))
- *Gerekçe:* sayı yerleşiklerinin tipi işleçlerin tipiyle aynıdır (3.1, 4.2), böylece `TR_sum` ile elle toplama aynı tipi verir. `TR_min`/`TR_max`'ın iki yazımı ayrı iki iştir (bir listenin en büyüğü, birkaç değerin en büyüğü) ve ajanların bildiği yazımdır. Tek tamponlu okuyucu, boruyla gelen satırların okuyucular arasında kaybolmasını önler.

### 13.1a

- *Emsal:* Python ve Go göreli yolu yalnız çalışma klasörüne göre çözer. *Almadık:* yalnız çalışma klasörü: program başka bir klasörden çalıştırılınca yanındaki dosyayı bulamaz ve hata, programın neden çalışmadığını söylemez.
- *Gerekçe:* aynı program hangi klasörden çalıştırılırsa çalıştırılsın kendi dosyasını bulur ve yazdığı yer okuduğu yerdir. Sekmeli yolun dosyaya bakılmadan reddedilmesi, Windows yolundaki kaçış hatasını "dosya yok" gibi yanıltıcı bir hataya dönüştürmez.

### 13.2

- *Emsal:* Go'nun `strings.ToUpper`'ı ve Python'un `str.upper`'ı Unicode'un varsayılan dönüşümünü kullanır, dile göre değişmez. *Almadık:* yerele göre harf dönüşümü (Türkçenin noktalı I'sı): aynı program makinenin diline göre başka sonuç verirdi.
- *Gerekçe:* değerin metni dilden bağımsızdır (3.14); harf dönüşümü de her makinede aynı sonucu verir.

### 13.3

- *Emsal:* Python'un `sorted`'ı yeni bir liste verir ve eşitlerin sırasını korur. *Almadık:* Python'un `list.sort()`'u ve `sorted`'ın iki yazımı; karışık tiplerin sıralanması (Python 3 de `TypeError` verir).
- *Gerekçe:* sıralamanın tek yazımı yeni bir liste verir; yerinde değiştiren tek liste metodu `TR_append`'tir. `TR_first` ve `TR_last`'ın `T?` vermesi boş listeyi çalışmadan önce görünür kılar (4. katman).

### 13.4

- *Emsal:* Python'un `dict.keys()` ve `values()`'ı. *Almadık:* görünüm nesneleri: TR'de sonuç bir listedir.
- *Gerekçe:* sözlüğün anahtarları ve değerleri sıradan bir listeyle dolaşılır.

### 13.5

- *Emsal:* Python bilinmeyen adlı argümanda `TypeError` verir. *Almadık:* JavaScript'in seçenek nesnesinde bilinmeyen alanın sessizce yok sayılması.
- *Gerekçe:* yanlış yazılmış bir seçenek (`TR_lr` yerine "ogrenme_orani") sessizce yok sayılırsa eğitim varsayılan değerle koşar ve sonuç hata vermeden yanlış olur.

### 13.6

- *Emsal:* Rust'ta değeri dönüştüren iş metottur (`iter.map`, `v.len()`). Python'un `functools.reduce`'u başlangıcı isteğe bağlı alır. *Almadık:* Python'un `len(x)` ve `x.__len__()` gibi iki yazımı; `reduce`'un fonksiyonu başa alan sırası: TR'de fonksiyon son argümandır (6.7).
- *Gerekçe:* her işin tek biçimi olunca ajan yazımı tahmin etmez; yanlış biçim tanımsız ad ya da bilinmeyen üyedir ve ileti doğrusunu söyler. Sondaki bloğun parametre almaması (6.7) yüzünden iki parametreli `TR_reduce` ok fonksiyonuyla yazılır.

### 13.7

- *Emsal:* Python'un `math.floor`'u `int` verir; Rust'ın `f64::floor`'u `f64` verir. *Almadık:* Python'un tip değiştiren `floor`'u: TR'de tip girdinin tipidir. Tek harfli `e` sabiti.
- *Gerekçe:* tipin girdiyle aynı kalması 13.10'un kuralıdır. Tek harfli bir yerleşik ad kullanıcı değişkenleriyle çakışırdı.

### 13.8

- *Emsal:* Python'un `json.dumps`'ı JSON'da olmayan değerde `TypeError` verir. *Almadık:* bir metnin dosya adı mı içerik mi olduğunu tahmin eden tek fonksiyon.
- *Gerekçe:* `TR_json_parse`'ın `TR_JSON` vermesi dış verinin tipini 3.7a'daki gibi açmaya zorlar. CSV'de katı sayı biçimi "007" gibi kimlikleri sessizce sayıya çevirmez.

### 13.9

- *Emsal:* TypeScript'in yerleşik kitaplığı imzaları bildirim dosyalarında (`lib.d.ts`) tutar; Python'un yerleşiklerinin imzaları typeshed'dedir. *Almadık:* imzanın yalnız belgede yazılması: belge ile uygulama ayrışır.
- *Gerekçe:* denetleyici (4. katman), düzenleyici, başvuru belgesi ve ajan aynı imzayı okur; imza tek yerde olunca ayrışamaz. TR modüllerinde kaynak ile katalogun aynı olması testle sabitlenir.

### 13.10

- *Emsal:* TypeScript: "we can specify a function that can be called in different ways by writing _overload signatures_"; Python: "The @overload decorator allows describing functions and methods that support multiple different combinations of argument types". Rust'ın `map`'i tip parametreleriyle yazılır: `fn map<B, F>(self, f: F) -> Map<Self, F> where Self: Sized, F: FnMut(Self::Item) -> B`. *Almadık:* birleşim dönüş tipi (bugünkü katalogda 38 yerleşik "Int ya da Float ya da Tensor" döndürüyor): sonucu kullanan her yer tipi yeniden sınamak zorunda kalır. Kullanıcının fonksiyonunda birden çok imza: TR'de bir ad bir tanımdır (6.1). ([TypeScript el kitabı](https://www.typescriptlang.org/docs/handbook/2/functions.html#function-overloads), [Python 3.15 typing](https://docs.python.org/3/library/typing.html#typing.overload), [Rust 1.99 Iterator](https://doc.rust-lang.org/std/iter/trait.Iterator.html#method.map))
- *Gerekçe:* `TR_sum([1, 2])`'nin tipi `TR_Int` ise sonucu `TR_Int` isteyen yere doğrudan verilir; birleşimde her kullanım ya hata verir ya da sessizce genel tipe düşer. Genel tip olmadığı için (3.11) her tipi alan yerleşik tip parametresiyle yazılır ve sonucun tipi kaybolmaz: `TR_map` `TR_List[TR_String]` verince öğeler metindir.

### 13.11

- *Emsal:* Python'un yerleşikleri yanlış argümanda `TypeError` verir ve fonksiyonu adlandırır. *Almadık:* JavaScript'in eksik argümanı `undefined` sayması.
- *Gerekçe:* ileti yerleşiği, sırayı, beklenen ve verilen tipi söyleyince ajan düzeltmeyi tahmin etmez; aynı kodun 4. katmanda çalışmadan önce verilmesi iletinin değişmemesini sağlar.
