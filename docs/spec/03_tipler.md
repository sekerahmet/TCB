# 3. Tipler ve değerler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "3. Tipler ve değerler".

TR statik tiplidir (1.2): her değerin tipi çalışmadan önce bilinir. Bu bölüm dilin tiplerini ve değerlerini anlatır; tip denetiminin kuralı 3.13'te, değerin metni 3.14'tedir.

**3.1** `TR_Int`: 64 bit işaretli tam sayı (−9223372036854775808 … 9223372036854775807). Bir işlemin ya da yerleşiğin (`TR_sum`, `TR_abs`) sonucu bu aralığı aşarsa değer sessizce sarılmaz, "tam sayı taşması" çalışma hatası verilir.
- *Emsal:* Swift: "arithmetic operators in Swift don't overflow by default. Overflow behavior is trapped and reported as an error." Rust hata ayıklama derlemesinde taşmada durur. *Almadık:* Go'nun ve Rust'ın yayım derlemesinin sessizce sarması (Go: "Overflow does not cause a run-time panic"); Python'un sınırsız tam sayısı (2.4). ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/advancedoperators/), [Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch03-02-data-types.html#integer-overflow), [Go 1.27](https://go.dev/ref/spec#Integer_overflow))
- *Gerekçe:* sarılmış bir adım sayacı, bayt konumu ya da indeks sessizce yanlış veri okur; hata o anda görünmeli. 64 bit, adım ve bayt sayıları için yeter.
- *Test:* TR testi.

**3.2** `TR_Float`: IEEE 754'ün 64 bitlik kayan noktalı sayısı. Metne, geri okununca aynı sayıyı veren en kısa rakamlarla çevrilir (3.14): tam sayı değerindeki ondalık noktadan sonra bir sıfırla yazılır ("1.0", "1234567.0"); üssü −4'ten küçük ya da 16 ve üstü olan sayı bilimsel gösterimle yazılır ("1e-05", "1e+16"; "0.0001" ve "1000000000000000.0" olduğu gibi). Sonsuz ve tanımsız değerler `TR_inf` ve `TR_nan`dır. Daha küçük ondalık ve tam sayı tipleri (32 ve 16 bit, nicemlenmiş) tensörün öğe tipleridir; kuralları sonraki bölümlerdedir.
- *Emsal:* Python, Go ve Rust ondalığı IEEE 754'ün 64 bitlik biçimiyle tutar. Python ondalığı geri okununca aynı sayıyı veren en kısa rakamlarla, noktadan sonra en az bir rakamla yazar; üs −4'ün altındaysa ya da duyarlığın sınırını aşıyorsa bilimsel gösterime geçer. TR'nin eşiği Python'unkiyle aynıdır (Python 3.12'de `repr(1e16)` "1e+16", `repr(1e15)` "1000000000000000.0"). PyTorch'ta tek sayı 64 bittir, küçük tipler tensörün öğe tipidir. *Almadık:* C'nin `%g`'si (altı anlamlı rakama yuvarlar: 1234567.0 → "1.23457e+06"). ([Python 3.15 biçim](https://docs.python.org/3/library/string.html#format-specification-mini-language), [Go 1.27 strconv](https://pkg.go.dev/strconv#FormatFloat), [PyTorch 2.14 öğe tipleri](https://docs.pytorch.org/docs/2.14/tensor_attributes.html), [C printf](https://en.cppreference.com/w/c/io/fprintf))
- *Gerekçe:* aynı sayı her makinede aynı metni verir ve geri okununca aynı sayı olur; günlükteki kayıp değeri yuvarlanıp kaybolmaz, bir milyonluk adım sayısı bilimsel gösterime kaymaz. Küçük tipler tensörün işidir; tek bir sayı tipi programı sade tutar.
- *Test:* TR testi; yazım eşiği uygulanınca TR testi.

**3.3** `TR_String`: değiştirilemez Unicode metni. Uzunluk ve indeks Unicode karakteri (kod noktası) sayar; negatif indeks sondan sayar; sınır dışı indeksle okuma çalışma hatasıdır, öğenin olmayabileceği yerde `.TR_get(i)` isteğe bağlı bir metin verir (3.5). `m[a..b]` alt metin verir: uçlar negatifse sondan sayılır, metnin sınırlarına kırpılır, başlangıç bitişten büyükse sonuç boştur (`"şeker"[-3..5]` → "ker").
- *Emsal:* Python metni değiştirilemez kod noktası dizisi olarak tutar ("Strings are immutable sequences of Unicode code points"), dilimde sınır dışı uçları kırpar, indekste hata verir. Swift'te karakter bir grafem kümesidir; Rust ve Go'da uzunluk bayt sayısıdır. *Almadık:* Swift'in grafem sayımı: grafem kuralları Unicode sürümüyle değişir (UAX #29'un Unicode 18.0 için 49. düzeltmesi GB9c kuralını değiştirdi), aynı metnin uzunluğu sürümden sürüme farklı çıkardı; Rust ve Go'nun bayt indeksi: bir harfin ortasından kesebilir. ([Python 3.15 str](https://docs.python.org/3/library/stdtypes.html#text-sequence-type-str), [Python 3.15 dilim](https://docs.python.org/3/tutorial/introduction.html#text), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/stringsandcharacters/), [UAX #29, Unicode 18.0](https://www.unicode.org/reports/tr29/), [Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch08-02-strings.html), [Go 1.27](https://go.dev/ref/spec#String_types))
- *Gerekçe:* parçalayıcılar ve veri hazırlama kod noktasıyla çalışır; aynı metin her sürümde aynı uzunluğu verir. Kırpan dilim, bir istemi en çok N karakterle sınırlamayı tek adımda yazdırır (`m[0..N]`).
- *Test:* TR testi.

```tr
//! TR 4
TR_print("şeker".TR_length, "şeker"[-1], "şeker"[1..3])   // 5 r ek
```

**3.4** `TR_Bool`: `TR_true` ve `TR_false`. Koşul yerinde yalnız bu tip kullanılır (3.10).
- *Emsal:* Go, Rust, Swift ve Python'da iki değerli bir mantık tipi vardır. ([Go 1.27](https://go.dev/ref/spec#Boolean_types), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/types/boolean.html))
- *Gerekçe:* karşılaştırmanın ve sınamanın sonucu ayrı bir tiptir; sayıyla ya da metinle karışmaz.
- *Test:* TR testi.

**3.5** `TR_none` değerin yokluğudur ve `TR_NoneType` tipinin tek değeridir. Bir değişken, parametre, alan ya da dönüş ancak tipi isteğe bağlıysa `TR_none` alabilir: isteğe bağlı tip, asıl tipin ardına `?` yazılarak gösterilir (`TR_Int?`). İsteğe bağlı değer asıl tipinin beklendiği yerde kullanılamaz; önce `??` ile varsayılan verilerek, `?.` zinciriyle ya da `TR_none` ile karşılaştırılarak açılır: `x != TR_none` sınamasından sonra o dalda `x` asıl tipindedir. Değer döndürmeyen fonksiyonun dönüş tipi `TR_NoneType`dır.
- *Emsal:* Swift ve Kotlin isteğe bağlı tipi `T?` ile yazar; Kotlin sınamadan sonra değişkeni kendiliğinden asıl tipe daraltır. Rust'ta `Option<T>`, C#'ta boş olabilen başvuru tipleri vardır. *Almadık:* Go'nun `nil`'i ve Python'un `None`'u: her yerde olabilir, hata ancak kullanıldığı satırda çıkar. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Optionals), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html), [Rust 1.99 std::option](https://doc.rust-lang.org/std/option/), [C# 14](https://learn.microsoft.com/en-us/dotnet/csharp/nullable-references))
- *Gerekçe:* yok gelebilecek değer (bulunamayan dosya, boş dönen arama, API cevabındaki eksik alan) tipinde görünür; denetleyici açılmamış kullanımı çalışmadan önce yakalar. Boş değerin beklenmediği yerde kullanılması hizmette ve robotta çalışırken çıkan yaygın bir hatadır.
- *Test:* TR testi; isteğe bağlı tip uygulanınca denetleyicinin testi.

```tr
//! TR 4
TR_var l: TR_List[TR_Int] = [10, 20, 30]
TR_var x: TR_Int? = l.TR_get(5)
TR_print(x ?? 0)                 // 0
TR_if x != TR_none { TR_print(x + 1) }   // bu dalda x bir TR_Int
```

**3.6** `TR_List[T]`: tek tipli, sıralı, değiştirilebilir dizi; `[1, 2, 3]` bir `TR_List[TR_Int]`dir. Öğelerin tipi aynıdır; boş liste tipini yazar (`TR_var l: TR_List[TR_Int] = []`). Negatif indeks sondan sayar; sınır dışı indeks okumada da yazmada da çalışma hatasıdır ve indeksi ile uzunluğu söyler; `.TR_get(i)` `T?` verir (3.5). Dilim metindeki gibidir (3.3). Listeler, sözlükler ve yapı nesneleri başvuruyla paylaşılır: `b = a`'dan sonra ikisi aynı listedir. Tip parametresi köşeli parantezle yazılır.
- *Emsal:* Rust `Vec<T>`, Swift `[T]`, Go `[]T` ve Kotlin `List<T>` tek tiplidir. Python ve Go tip parametresini köşeli parantezle yazar (`list[int]`; Go: "the type parameter list uses square brackets"); Rust'ta ifadenin içinde `<` küçüktür işleciyle karıştığı için `::<>` gerekir ("required before the opening < for generic arguments to avoid ambiguity with the less-than operator"). Python'da ve Go'nun diliminde liste başvuruyla paylaşılır. *Almadık:* Python'un karışık tipli listesi. ([Rust 1.99 Vec](https://doc.rust-lang.org/std/vec/struct.Vec.html), [Rust 1.99 Reference yollar](https://doc.rust-lang.org/reference/paths.html#paths-in-expressions), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/collectiontypes/), [Go 1.27 tip parametreleri](https://go.dev/ref/spec#Type_parameter_declarations), [Kotlin 2.4](https://kotlinlang.org/docs/collections-overview.html), [PEP 585](https://peps.python.org/pep-0585/))
- *Gerekçe:* tek tipli listede her öğenin tipi bilinir; öğe üzerindeki her işlem çalışmadan önce denetlenir. Köşeli parantez karşılaştırma işleçleriyle karışmaz. Büyük bir veri listesi fonksiyona kopyalanmadan geçer.
- *Test:* TR testi; tek tipli liste uygulanınca denetleyicinin testi.

**3.7** `TR_Dict[K, V]`: eklenme sırasını koruyan anahtar–değer tablosu (`{"k": 1}`). Anahtar tipi `TR_Int`, `TR_String` ya da `TR_Bool`dur. Sözlük değişmezindeki anahtar bir ifadedir: `{k: 1}`'de `k` bir değişkendir, anahtar onun değeridir. Anahtar yalnız köşeli parantezle okunur ve yazılır (`d["k"]`); nokta yalnız sözlüğün üyelerini verir (`.TR_length`, `.TR_keys()`, `.TR_values()`, `.TR_contains(k)`, `.TR_get(k)`). Olmayan anahtarı köşeli parantezle okumak çalışma hatasıdır; `.TR_get(k)` `V?` verir (3.5).
- *Emsal:* Python 3.7'den beri sözlük eklenme sırasını korur ("Dictionary order is guaranteed to be insertion order"); olmayan anahtar hata verir, `get` `None` döner. Python, Rust, Go ve Swift sözlüğü yalnız köşeli parantezle okur. Rust'ta `f64` anahtar olamaz (`Eq` ve `Hash` yok). *Almadık:* JavaScript nesnesinin aynı anahtarı iki yazımla okuması ve değişmezdeki çıplak adı anahtar sayması; Swift'in her okumayı isteğe bağlı vermesi (her erişim açılmak zorunda); Go'nun ondalık anahtarı ("comparison operators == and != must be fully defined" kuralı ondalıkta `NaN` yüzünden tutmaz). ([Python 3.15 dict](https://docs.python.org/3/library/stdtypes.html#mapping-types-dict), [Rust 1.99 HashMap](https://doc.rust-lang.org/std/collections/struct.HashMap.html), [Go 1.27](https://go.dev/ref/spec#Map_types), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/collectiontypes/))
- *Gerekçe:* her erişimin tek yazımı vardır ve anahtarla üye hiç karışmaz: köşeli parantezin içi her zaman anahtardır. Kayan noktalı anahtar yuvarlama yüzünden sessizce bulunamazdı. JSON nesnesi ve model ayarları sözlükle taşınır; yazan da okuyan da aynı yazımı kullanır.
- *Test:* TR testi; anahtar ve erişim kuralları uygulanınca TR testi.

**3.8** `TR_Range`: `a..b` (b hariç) ve `a..=b` (b dahil); uçlar `TR_Int`tir. `a >= b` ise `a..b`, `a > b` ise `a..=b` boştur; geriye saymak `.TR_reverse()` ile yazılır (`(0..3).TR_reverse()` → 2, 1, 0). Tam sayının sınırındaki uçlar da taşmadan gezilir.
- *Emsal:* Rust: "It is empty if start >= end"; Python'da `range(3, 0)` boştur. İkisi de geriye saymayı ayrı yazar (`.rev()`, `reversed()`). *Almadık:* kendiliğinden geriye sayan aralık. ([Rust 1.99 Range](https://doc.rust-lang.org/std/ops/struct.Range.html), [Python 3.15 range](https://docs.python.org/3/library/stdtypes.html#ranges))
- *Gerekçe:* `0..n` her zaman "n kez" demektir; n sıfır ya da eksiyse döngü hiç dönmez. Kendiliğinden geriye sayan aralıkta eksi bir n, beklenmeyen ve geriye doğru giden bir döngü olurdu.
- *Test:* TR testi; boş aralık ve geriye sayma uygulanınca TR testi.

**3.9** Fonksiyonlar birinci sınıf değerlerdir: değişkene atanır, listeye konur, argüman olarak geçer, döndürülür. Fonksiyon değerinin tipi parametrelerinin ve dönüşünün tipleridir; yazımı sonraki bölümlerdedir.
- *Emsal:* Rust, Swift, Go, Kotlin ve Python'da fonksiyon bir değerdir ve tipi imzasıdır. ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/types/function-pointer.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/functions/#Function-Types), [Go 1.27](https://go.dev/ref/spec#Function_types))
- *Gerekçe:* eğitim döngüsü kayıp fonksiyonunu, iyileştirici öğrenme oranı çizelgesini, sunucu isteği karşılayan fonksiyonu argüman olarak alır.
- *Test:* TR testi.

**3.10** Koşul `TR_Bool` olmalıdır: `TR_if`'in ve koşullu `TR_for`'un koşulu, `TR_and`, `TR_or` ve `TR_not`'un işlenenleri. Başka tipte bir değer (sayı, metin, liste, isteğe bağlı değer) koşul yerinde denetim hatasıdır; boşluk `.TR_length == 0` ile, yokluk `TR_none` ile karşılaştırılarak açıkça sorulur.
- *Emsal:* Go'da `if` ve `for` koşulu bir mantık ifadesidir; Rust'ta koşul `bool` tipinde olmalıdır; Swift: "Swift's type safety prevents non-Boolean values from being substituted for Bool"; Kotlin'de ve C#'ta da böyledir. *Almadık:* Python'un doğruluk sınaması: sıfır, boş metin ve `None` yanlış sayılır; örtük sınama sayının sıfır olduğunu mu yoksa yokluğunu mu sorduğunu söylemez. ([Go 1.27](https://go.dev/ref/spec#If_statements), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/if-expr.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Booleans), [Python 3.15](https://docs.python.org/3/library/stdtypes.html#truth-value-testing))
- *Gerekçe:* açık koşul ne sorulduğunu söyler: sıfır kayıp yanlış sayılmaz, boş bir cevapla gelmeyen cevap karışmaz. Kodlama modelinin Python alışkanlığıyla yazdığı örtük sınama çalışmadan önce görünür.
- *Test:* TR testi; mantık dışı koşulun hatası uygulanınca denetleyicinin testi.

**3.11** `TR_Type(x)` değerin tipini bir tip değeri olarak verir; tip değerlerinin tipi `TR_Type`dir. Tip değerleri dilin tipleri (`TR_Int`, `TR_Float`, `TR_String`, `TR_Bool`, `TR_NoneType`, `TR_Range`, `TR_Function`, `TR_Tensor`, `TR_Type`), parametreli tipler (`TR_List[TR_Int]`, `TR_Int?`), yapı tanımları ve kitaplığın tipleridir (`TR_Optimizer`, `TR_Tokenizer` gibi). Tip değeri tip değeriyle karşılaştırılır (`TR_Type(x) == TR_Int`); metinle karşılaştırılması denetim hatasıdır. Matris ayrı bir tip değildir, iki boyutlu bir tensördür.
- *Emsal:* Python `type(x)`, Swift `type(of:)` ve Go `reflect.TypeOf` bir tip değeri verir. *Almadık:* tipi adının metniyle sınamak. ([Python 3.15](https://docs.python.org/3/library/functions.html#type), [Swift 6.4](https://developer.apple.com/documentation/swift/type(of:)), [Go 1.27 reflect](https://pkg.go.dev/reflect#TypeOf))
- *Gerekçe:* çalışırken tip sormak tanılama ve genel yardımcılar içindir; statik tip (3.13) çoğu soruyu çalışmadan önce cevaplar. Metinle karşılaştırma bir yazım farkında sessizce yanlış verirdi.
- *Test:* TR testi.

**3.12** Bir tip çağrılınca o tipin değerini kurar ya da verilen değeri o tipe çevirir; aynı işi gören ayrı bir fonksiyon yoktur. `TR_Int(x)` ondalığı sıfıra doğru keser ve metni ondalık tam sayı olarak okur; `TR_Float(x)` tam sayıyı ve metni ondalığa çevirir; `TR_String(x)` değerin metnini verir (3.14); kitaplığın tipleri de böyle kurulur (`TR_Tensor([1, 2])`, `TR_Checkpoint("kayit")`). Çevrilemeyen değer çalışma hatasıdır: sayı olmayan metin, `TR_nan`, sonsuz ve `TR_Int`'in aralığına sığmayan ondalık. Atamada ve argümanda tipler arasında kendiliğinden dönüşüm yoktur; tek istisna 3.13'ün tam sayı yazımıdır.
- *Emsal:* Python'da tip çağrılınca kurar ve çevirir (`int("42")`, `list(...)`); `int()` sıfıra doğru keser, `NaN`'da ve sonsuzda hata verir. Swift'te de dönüşüm tipin kurucusudur (`Int("42")`, `String(x)`). Kotlin: "Kotlin does not support implicit conversion for assignments or function arguments." *Almadık:* aynı işi gören iki ad (PyTorch'ta `torch.tensor` da `torch.Tensor` da tensör kurar ve ikisi farklı davranır); Rust'ın `as`'ı (sığmayan değeri sessizce uca yuvarlar, `NaN`'ı 0 yapar); Go: "the conversion succeeds but the result value is implementation-dependent". ([Python 3.15 int](https://docs.python.org/3/library/functions.html#int), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Integer-and-Floating-Point-Conversion), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html), [PyTorch 2.14](https://docs.pytorch.org/docs/2.14/tensors.html), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#numeric-cast), [Go 1.27](https://go.dev/ref/spec#Conversions))
- *Gerekçe:* her işin tek yazımı vardır (2.9): tip ile aynı adı taşıyıp yalnız büyük-küçük harfle ayrılan ikinci bir fonksiyon kodlama modelini yanıltırdı. Sığmayan ya da tanımsız bir değer sessizce başka bir sayıya dönmez; bozuk bir ayar dosyasındaki sayı olmayan bir metin çalışmanın başında görünür.
- *Test:* TR testi.

```tr
//! TR 4
TR_print(TR_Int("42") + 1, TR_Int(3.9), TR_Int(-3.9), TR_String(12) + "a")   // 43 3 -3 12a
```

**3.13** Tip denetimi çalışmadan önce bütün programa iner (1.3): her ifadenin, atama hedefinin, parametre varsayılanının, yapı alanının ve metot gövdesinin tipi bilinir. Fonksiyonun parametreleri ve dönüşü tipini yazar (1.2); yerel değişkenin tipi ilk değerinden çıkarılır ve değişmez, açıklaması isteğe bağlıdır. Tip açıklaması adın ardından iki nokta ve tiple yazılır (`n: TR_Int`). Tam sayı yazımı (2.4) `TR_Float` beklenen yerde ondalık değerdir (`r: TR_Float = 1` → 1.0); bu yalnız yazım içindir, `TR_Int` tipli bir değer `TR_Float` yerine verilmez. Tip uyuşmazlığı denetim hatasıdır (çıkış kodu 1, 1.9); ileti doğru yazımı ya da dönüşümü önerir (`0.0`, `TR_Float(n)`).
- *Emsal:* Rust, Swift, Go ve Kotlin'de yerel değişkenin çıkarılan tipi kalıcıdır. Swift'te tam sayı yazımının kendi tipi yoktur ("The literal value of 3 has no explicit type in and of itself"), ondalık beklenen yerde ondalık olur; Go'nun tipsiz sabitleri de öyledir. *Almadık:* Python'un dinamik tipi (değişken sonra başka tipte değer alır); Kotlin'in ondalık beklenen yerde tam sayı yazımını reddetmesi (`val one: Double = 1` hata). ([Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch03-02-data-types.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/#Type-Safety-and-Type-Inference), [Go 1.27 sabitler](https://go.dev/ref/spec#Constants), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html))
- *Gerekçe:* değişkenin tipi bildirildiği yerden okunur; kişi de kodlama modeli de bir değişkenin sonra başka tipte değer alacağını hesaba katmaz. Kayıp toplamını `0` ile başlatıp ondalık eklemek gibi bir yanlış çalışmadan önce görünür ve ileti doğrusunu söyler. Öğrenme oranına `1` yazmak doğaldır ve bilgi kaybettirmez.
- *Test:* TR testi; kalıcı tip ve tam sayı yazımı uygulanınca denetleyicinin testleri.

```tr hata
//! TR 4
TR_var toplam = 0
toplam += 0.5        // denetim hatası: toplam bir TR_Int; ilk değeri 0.0 yazın
```

**3.14** Değerin metni hiçbir dile bağlı değildir: `TR_String(x)`, `TR_print` ve yerleştirme (2.7) aynı değer için her makinede ve her okuyan için aynı metni verir. Tam sayı ondalık rakamlarla, ondalık 3.2'deki gibi yazılır. Dilin değerleri ve tipleri kimliklerinin ad kısmıyla yazılır: `TR_true` ve `TR_false` "true" ve "false", `TR_none` "none", `TR_inf` "inf" (eksisi "-inf"), `TR_nan` "nan", tip değeri tipin adıyla ("Int", "List[Int]", "Int?"). Liste "[1, 2]", sözlük "{"k": 1}" biçimindedir; liste ve sözlüğün içindeki metin çift tırnakla ve 2.6'nın kaçışlarıyla yazılır. Yapı nesnesinin ve tensörün metni sonraki bölümlerdedir. Programın çıktısı çevrilmez; kişinin dilinde görünen yalnız iletiler ve kabuğun, hata ayıklayıcının görüntüsüdür.
- *Emsal:* Python, Go, Rust ve Java mantık değerini her yerel ayarda aynı sözcükle yazar (Python "True", Go ve Rust "true"); JSON'da "true", "false" ve "null"dır. Rust'ın hata ayıklama gösterimi listedeki metni çift tırnakla yazar. *Almadık:* değerin metnini dosyanın ya da çalıştıranın diline bağlamak: aynı program iki makinede iki çıktı verir, çıktıyı okuyan program bozulur; Python'un listedeki metni tek tırnakla yazması (TR'de metin çift tırnaklıdır, 2.6). ([Python 3.15](https://docs.python.org/3/library/stdtypes.html#boolean-type-bool), [Go 1.27 fmt](https://pkg.go.dev/fmt), [Rust 1.99 std::fmt](https://doc.rust-lang.org/std/fmt/), [RFC 8259 JSON](https://www.rfc-editor.org/rfc/rfc8259))
- *Gerekçe:* programın çıktısı veridir: günlüğe yazılır, başka bir programı ve kodlama modelini besler. Aynı program aynı veriyle aynı sonucu vermelidir; dosyanın dili yoktur (2.1). Kimliğin ad kısmı hiç değişmez.
- *Test:* TR testi; dilden bağımsız metin uygulanınca TR testi.

```tr
//! TR 4
TR_print(TR_true, TR_none, 1e16, TR_Type([1]), ["a"], {"k": 1})
// true none 1e+16 List[Int] ["a"] {"k": 1}
```
