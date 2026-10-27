# 3. Tipler ve değerler: emsal ve gerekçe

Kurallar: [`03_tipler.md`](../03_tipler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 3.1

- *Emsal:* Swift: "arithmetic operators in Swift don't overflow by default. Overflow behavior is trapped and reported as an error." Rust hata ayıklama derlemesinde taşmada durur. *Almadık:* Go'nun ve Rust'ın yayım derlemesinin sessizce sarması (Go: "Overflow does not cause a run-time panic"); Python'un sınırsız tam sayısı (2.4). ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/advancedoperators/), [Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch03-02-data-types.html#integer-overflow), [Go 1.27](https://go.dev/ref/spec#Integer_overflow))
- *Gerekçe:* sarılmış bir adım sayacı, bayt konumu ya da indeks sessizce yanlış veri okur; hata o anda görünmeli. 64 bit, adım ve bayt sayıları için yeter.

### 3.2

- *Emsal:* Python, Go ve Rust ondalığı IEEE 754'ün 64 bitlik biçimiyle tutar. Python ondalığı geri okununca aynı sayıyı veren en kısa rakamlarla, noktadan sonra en az bir rakamla yazar; üs −4'ün altındaysa ya da duyarlığın sınırını aşıyorsa bilimsel gösterime geçer. TR'nin eşiği Python'unkiyle aynıdır (Python 3.12'de `repr(1e16)` "1e+16", `repr(1e15)` "1000000000000000.0"). PyTorch'ta tek sayı 64 bittir, küçük tipler tensörün öğe tipidir. *Almadık:* C'nin `%g`'si (altı anlamlı rakama yuvarlar: 1234567.0 → "1.23457e+06"). ([Python 3.15 biçim](https://docs.python.org/3/library/string.html#format-specification-mini-language), [Go 1.27 strconv](https://pkg.go.dev/strconv#FormatFloat), [PyTorch 2.14 öğe tipleri](https://docs.pytorch.org/docs/2.14/tensor_attributes.html), [C printf](https://en.cppreference.com/w/c/io/fprintf))
- *Gerekçe:* aynı sayı her makinede aynı metni verir ve geri okununca aynı sayı olur; günlükteki kayıp değeri yuvarlanıp kaybolmaz, bir milyonluk adım sayısı bilimsel gösterime kaymaz. Küçük tipler tensörün işidir; tek bir sayı tipi programı sade tutar.

### 3.3

- *Emsal:* Python metni değiştirilemez kod noktası dizisi olarak tutar ("Strings are immutable sequences of Unicode code points"), dilimde sınır dışı uçları kırpar, indekste hata verir. Swift'te karakter bir grafem kümesidir; Rust ve Go'da uzunluk bayt sayısıdır. *Almadık:* Swift'in grafem sayımı: grafem kuralları Unicode sürümüyle değişir (UAX #29'un Unicode 18.0 için 49. düzeltmesi GB9c kuralını değiştirdi), aynı metnin uzunluğu sürümden sürüme farklı çıkardı; Rust ve Go'nun bayt indeksi: bir harfin ortasından kesebilir. ([Python 3.15 str](https://docs.python.org/3/library/stdtypes.html#text-sequence-type-str), [Python 3.15 dilim](https://docs.python.org/3/tutorial/introduction.html#text), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/stringsandcharacters/), [UAX #29, Unicode 18.0](https://www.unicode.org/reports/tr29/), [Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch08-02-strings.html), [Go 1.27](https://go.dev/ref/spec#String_types))
- *Gerekçe:* parçalayıcılar ve veri hazırlama kod noktasıyla çalışır; aynı metin her sürümde aynı uzunluğu verir. Kırpan dilim, bir istemi en çok N karakterle sınırlamayı tek adımda yazdırır (`m[0..N]`).

### 3.3a

- *Emsal:* Python'da `bytes` değiştirilemez bayt dizisidir, öğesi bir tam sayıdır ve metinle kendiliğinden karışmaz; Python 3 metinle baytı bu yüzden ayırdı ("Text Vs. Data Instead Of Unicode Vs. 8-bit"). Go'da `[]byte`, Rust'ta `Vec<u8>`, Kotlin'de `ByteArray`, Swift'te `Data` vardır. *Almadık:* Python 2'nin metinle baytı aynı tip sayması; Go'nun ve Kotlin'in değiştirilebilir bayt dizisi (paylaşılan veri sessizce değişebilir; değiştirilecek veri tensörde tutulur). ([Python 3.15 bytes](https://docs.python.org/3/library/stdtypes.html#bytes-objects), [Python 3.0 yenilikleri](https://docs.python.org/3/whatsnew/3.0.html#text-vs-data-instead-of-unicode-vs-8-bit), [Go 1.27](https://go.dev/ref/spec#Slice_types), [Rust 1.99 Vec](https://doc.rust-lang.org/std/vec/struct.Vec.html), [Swift Data](https://developer.apple.com/documentation/foundation/data))
- *Gerekçe:* model dosyaları, görüntü, ses ve ağ verisi bayt olarak gelir. Metinle bayt ayrı tip olunca bir dosyanın kodlaması sessizce bozulmaz.

### 3.4

- *Emsal:* Go, Rust, Swift ve Python'da iki değerli bir mantık tipi vardır. ([Go 1.27](https://go.dev/ref/spec#Boolean_types), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/types/boolean.html))
- *Gerekçe:* karşılaştırmanın ve sınamanın sonucu ayrı bir tiptir; sayıyla ya da metinle karışmaz.

### 3.5

- *Emsal:* Kotlin daraltmayı sınırlar: "Smart casts work only when the compiler can guarantee that the variable won't change between the check and its usage"; değişebilen özellik hiç daraltılmaz. Go değer döndürmeyen çağrıyı değer olarak kullanmayı reddeder ("f() (no value) used as value"). Swift ve Kotlin isteğe bağlı tipi `T?` ile yazar; Kotlin sınamadan sonra değişkeni kendiliğinden asıl tipe daraltır. Rust'ta `Option<T>`, C#'ta boş olabilen başvuru tipleri vardır. *Almadık:* Go'nun `nil`'i ve Python'un `None`'u: her yerde olabilir, hata ancak kullanıldığı satırda çıkar. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Optionals), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html), [Rust 1.99 std::option](https://doc.rust-lang.org/std/option/), [C# 14](https://learn.microsoft.com/en-us/dotnet/csharp/nullable-references), [Kotlin 2.4 daraltma](https://kotlinlang.org/docs/typecasts.html#smart-casts))
- *Gerekçe:* yok gelebilecek değer (bulunamayan dosya, boş dönen arama, API cevabındaki eksik alan) tipinde görünür; denetleyici açılmamış kullanımı çalışmadan önce yakalar. Boş değerin beklenmediği yerde kullanılması hizmette ve robotta çalışırken çıkan yaygın bir hatadır.

### 3.6

- *Emsal:* Swift'te ve Go'da tipsiz sayı yazımı değişmezdeki öbür öğelere göre tip alır (`[1, 2.5]` ondalık listesidir). Rust `Vec<T>`, Swift `[T]`, Go `[]T` ve Kotlin `List<T>` tek tiplidir. Python ve Go tip parametresini köşeli parantezle yazar (`list[int]`; Go'da tip parametresi listesi "enclosed in square brackets rather than parentheses"); Rust'ta ifadenin içinde `<` küçüktür işleciyle karıştığı için `::<>` gerekir ("required before the opening < for generic arguments to avoid ambiguity with the less-than operator"). Python'da ve Go'nun diliminde liste başvuruyla paylaşılır. *Almadık:* Python'un karışık tipli listesi. ([Rust 1.99 Vec](https://doc.rust-lang.org/std/vec/struct.Vec.html), [Rust 1.99 Reference yollar](https://doc.rust-lang.org/reference/paths.html#paths-in-expressions), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/collectiontypes/), [Go 1.27 tip parametreleri](https://go.dev/ref/spec#Type_parameter_declarations), [Kotlin 2.4](https://kotlinlang.org/docs/collections-overview.html), [PEP 585](https://peps.python.org/pep-0585/))
- *Gerekçe:* tek tipli listede her öğenin tipi bilinir; öğe üzerindeki her işlem çalışmadan önce denetlenir. Köşeli parantez karşılaştırma işleçleriyle karışmaz. Büyük bir veri listesi fonksiyona kopyalanmadan geçer.

### 3.7

- *Emsal:* Python 3.7'den beri sözlük eklenme sırasını korur ("Dictionary order is guaranteed to be insertion order"); olmayan anahtar hata verir, `get` `None` döner. Python, Rust, Go ve Swift sözlüğü yalnız köşeli parantezle okur. Rust'ta `f64` anahtar olamaz (`Eq` ve `Hash` yok). *Almadık:* JavaScript nesnesinin aynı anahtarı iki yazımla (`o.k`, `o["k"]`) okuması ve değişmezdeki çıplak adı anahtar sayması; Swift'in her okumayı isteğe bağlı vermesi (her erişim açılmak zorunda); Go'nun ondalık anahtarı (Go ondalık anahtarı kabul eder; `NaN` kendine eşit olmadığından `NaN` anahtar bir daha bulunamaz). ([Python 3.15 dict](https://docs.python.org/3/library/stdtypes.html#mapping-types-dict), [Rust 1.99 HashMap](https://doc.rust-lang.org/std/collections/struct.HashMap.html), [Go 1.27](https://go.dev/ref/spec#Map_types), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/collectiontypes/), [MDN özellik erişimi](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Property_accessors))
- *Gerekçe:* her erişimin tek yazımı vardır ve anahtarla üye hiç karışmaz: köşeli parantezin içi her zaman anahtardır. Kayan noktalı anahtar yuvarlama yüzünden sessizce bulunamazdı. JSON nesnesi ve model ayarları sözlükle taşınır; yazan da okuyan da aynı yazımı kullanır.

### 3.7a

- *Emsal:* serde_json'da `json!` değişmezi karışık değerli JSON kurar, `is_null()` ve `is_object()` gibi sorgular türü söyler; Python'un `json`'u sayının tam ya da ondalık yazımını korur. Rust'ın `serde_json`'u JSON'un her değerini tek bir `Value` tipiyle tutar ve `v["a"][0]` ile okur; Kotlin'de `JsonElement`, C#'ta `JsonNode` vardır. *Almadık:* genel bir "her şey" tipi: TypeScript'in `any`'si denetimi tamamen kapatır ("Using `any` disables all further type checking"); Go'nun `any`'si her kullanımda tip sorgusu ister. ([serde_json Value](https://docs.rs/serde_json/latest/serde_json/enum.Value.html), [Kotlin JsonElement](https://kotlinlang.org/api/kotlinx.serialization/kotlinx-serialization-json/kotlinx.serialization.json/-json-element/), [C# JsonNode](https://learn.microsoft.com/en-us/dotnet/api/system.text.json.nodes.jsonnode), [TypeScript any](https://www.typescriptlang.org/docs/handbook/2/everyday-types.html#any), [serde_json json!](https://docs.rs/serde_json/latest/serde_json/macro.json.html), [Python 3.15 json](https://docs.python.org/3/library/json.html))
- *Gerekçe:* API cevapları, ayar dosyaları ve modelin yapılandırılmış çıktısı JSON'dur. Değer `TR_JSON` kaldıkça yalnız JSON olarak kullanılır; asıl tipe geçiş yazıyla görünür ve yanlış tür o yerde hata verir. Genel bir "her şey" tipi, statik tipin çalışmadan önce yakaladığı hataları geri getirirdi.

### 3.8

- *Emsal:* Rust: "It is empty if start >= end"; Python'da `range(3, 0)` boştur. İkisi de geriye saymayı ayrı yazar (`.rev()`, `reversed()`). *Almadık:* kendiliğinden geriye sayan aralık. ([Rust 1.99 Range](https://doc.rust-lang.org/std/ops/struct.Range.html), [Python 3.15 range](https://docs.python.org/3/library/stdtypes.html#ranges))
- *Gerekçe:* `0..n` her zaman "n kez" demektir; n sıfır ya da eksiyse döngü hiç dönmez. Kendiliğinden geriye sayan aralıkta eksi bir n, beklenmeyen ve geriye doğru giden bir döngü olurdu.

### 3.9

- *Emsal:* Rust, Swift, Go, Kotlin ve Python'da fonksiyon bir değerdir ve tipi imzasıdır. ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/types/function-pointer.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/functions/#Function-Types), [Go 1.27](https://go.dev/ref/spec#Function_types))
- *Gerekçe:* eğitim döngüsü kayıp fonksiyonunu, iyileştirici öğrenme oranı çizelgesini, sunucu isteği karşılayan fonksiyonu argüman olarak alır.

### 3.10

- *Emsal:* Go'da `if` ve `for` koşulu bir mantık ifadesidir; Rust'ta koşul `bool` tipinde olmalıdır; Swift: "Swift's type safety prevents non-Boolean values from being substituted for Bool"; Kotlin'de ve C#'ta da böyledir. *Almadık:* Python'un doğruluk sınaması: sıfır, boş metin ve `None` yanlış sayılır; örtük sınama sayının sıfır olduğunu mu yoksa yokluğunu mu sorduğunu söylemez. ([Go 1.27](https://go.dev/ref/spec#If_statements), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/if-expr.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Booleans), [Python 3.15](https://docs.python.org/3/library/stdtypes.html#truth-value-testing))
- *Gerekçe:* açık koşul ne sorulduğunu söyler: sıfır kayıp yanlış sayılmaz, boş bir cevapla gelmeyen cevap karışmaz. Kodlama modelinin Python alışkanlığıyla yazdığı örtük sınama çalışmadan önce görünür.

### 3.11

- *Emsal:* Python `type(x)`, Swift `type(of:)` ve Go `reflect.TypeOf` bir tip değeri verir. *Almadık:* tipi adının metniyle sınamak. ([Python 3.15](https://docs.python.org/3/library/functions.html#type), [Swift 6.4](https://developer.apple.com/documentation/swift/type(of:)), [Go 1.27 reflect](https://pkg.go.dev/reflect#TypeOf))
- *Gerekçe:* çalışırken tip sormak tanılama ve genel yardımcılar içindir; statik tip (3.13) çoğu soruyu çalışmadan önce cevaplar. Metinle karşılaştırma bir yazım farkında sessizce yanlış verirdi.

### 3.12

- *Emsal:* Swift'te `Bool("true")` yalnız "true" ve "false" metnini okur. Python'da ise `bool("false")` doğru verir: doğruluk sınamasıdır (3.10), TR'de yoktur. Python'da tip çağrılınca kurar ve çevirir (`int("42")`, `list(...)`); `int()` sıfıra doğru keser, `NaN`'da ve sonsuzda hata verir. Swift'te de dönüşüm tipin kurucusudur (`Int("42")`, `String(x)`). Kotlin: "Kotlin does not support implicit conversion for assignments or function arguments." *Almadık:* aynı işi gören iki ad (PyTorch'ta `torch.tensor` da `torch.Tensor` da tensör kurar ve ikisi farklı davranır); Rust'ın `as`'ı (sığmayan değeri sessizce uca yuvarlar, `NaN`'ı 0 yapar); Go: "the conversion succeeds but the result value is implementation-dependent". ([Python 3.15 int](https://docs.python.org/3/library/functions.html#int), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Integer-and-Floating-Point-Conversion), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html), [PyTorch 2.14](https://docs.pytorch.org/docs/2.14/tensors.html), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#numeric-cast), [Go 1.27](https://go.dev/ref/spec#Conversions), [Swift Bool](https://developer.apple.com/documentation/swift/bool))
- *Gerekçe:* her işin tek yazımı vardır (2.9): tip ile aynı adı taşıyıp yalnız büyük-küçük harfle ayrılan ikinci bir fonksiyon kodlama modelini yanıltırdı. Sığmayan ya da tanımsız bir değer sessizce başka bir sayıya dönmez; bozuk bir ayar dosyasındaki sayı olmayan bir metin çalışmanın başında görünür.

### 3.13

- *Emsal:* Swift ilk değeri yalnız `nil` olan değişkende tip ister ("'nil' requires a contextual type"). Rust, Swift, Go ve Kotlin'de yerel değişkenin çıkarılan tipi kalıcıdır. Swift'te tam sayı yazımının kendi tipi yoktur ("The literal value of 3 has no explicit type in and of itself"), ondalık beklenen yerde ondalık olur; Go'nun tipsiz sabitleri de öyledir. *Almadık:* Python'un dinamik tipi (değişken sonra başka tipte değer alır); Kotlin'in ondalık beklenen yerde tam sayı yazımını reddetmesi (`val one: Double = 1` hata). ([Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch03-02-data-types.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Type-Safety-and-Type-Inference), [Go 1.27 sabitler](https://go.dev/ref/spec#Constants), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html))
- *Gerekçe:* değişkenin tipi bildirildiği yerden okunur; kişi de kodlama modeli de bir değişkenin sonra başka tipte değer alacağını hesaba katmaz. Kayıp toplamını `0` ile başlatıp ondalık eklemek gibi bir yanlış çalışmadan önce görünür ve ileti doğrusunu söyler. Öğrenme oranına `1` yazmak doğaldır ve bilgi kaybettirmez.

### 3.14

- *Emsal:* Kotlin isteğe bağlı değeri içindeki değerle yazar; Swift `Optional(5)` yazar, TR Kotlin gibi. Python, Go, Rust ve Java mantık değerini her yerel ayarda aynı sözcükle yazar (Python "True", Go ve Rust "true"); JSON'da "true", "false" ve "null"dır. Rust'ın hata ayıklama gösterimi listedeki metni çift tırnakla yazar. *Almadık:* değerin metnini dosyanın ya da çalıştıranın diline bağlamak: aynı program iki makinede iki çıktı verir, çıktıyı okuyan program bozulur; Python'un listedeki metni tek tırnakla yazması (TR'de metin çift tırnaklıdır, 2.6). ([Python 3.15](https://docs.python.org/3/library/stdtypes.html#boolean-type-bool), [Go 1.27 fmt](https://pkg.go.dev/fmt), [Rust 1.99 std::fmt](https://doc.rust-lang.org/std/fmt/), [RFC 8259 JSON](https://www.rfc-editor.org/rfc/rfc8259))
- *Gerekçe:* programın çıktısı veridir: günlüğe yazılır, başka bir programı ve kodlama modelini besler. Aynı program aynı veriyle aynı sonucu vermelidir; dosyanın dili yoktur (2.1). Kimliğin ad kısmı hiç değişmez.
