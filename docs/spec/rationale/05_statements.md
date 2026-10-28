# 5. Deyimler ve kontrol yapıları: emsal ve gerekçe

Kurallar: [`05_deyimler.md`](../05_deyimler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 5.1

- *Emsal:* Kotlin'de ve Swift'te yerel değişken ilk değeriyle tanımlanır; Rust gereksiz değişebilirliği uyarır ("The `unused_mut` lint detects mut variables which don't need to be mutable"). *Almadık:* Go'nun iki yazımı (`var x = 1` ve `x := 1`): aynı işin ikinci yazımı olurdu (2.9); Go'nun değer verilmeyen değişkene sıfır değeri vermesi (unutulmuş bir atama sessizce sıfır olur). ([Rust lints, uyarı](https://doc.rust-lang.org/rustc/lints/listing/warn-by-default.html), [Go 1.27](https://go.dev/ref/spec#Short_variable_declarations), [Kotlin 2.4](https://kotlinlang.org/docs/basic-syntax.html#variables))
- *Gerekçe:* ilk değer tipi verir (3.13) ve unutulmuş bir atamayı imkânsız kılar. Değişmeyen bağı `TR_const` ile yazmak, okuyana ve kodlama modeline o adın bir daha değişmeyeceğini söyler. Uyarı fonksiyonun içiyle sınırlıdır: programın en dışında hesaplanan her değer `TR_const`'a itilse 2.3b onu büyük harfe iterdi, betiğin bütün adları büyük harf olurdu (Gemini ve DENETİM, 2026-10-27).

### 5.2

- *Emsal:* Kotlin'in `val`'ı, Swift'in `let`'i ve JavaScript'in `const`'u bağı sabitler; MDN: "It does not mean the value it holds is immutable". *Almadık:* Rust'ın derin değişmezliği: değiştirilebilir liste ayrı bir sahiplik modeli ister. ([MDN const](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Statements/const), [Kotlin 2.4](https://kotlinlang.org/docs/basic-syntax.html#variables))
- *Gerekçe:* listeler ve sözlükler başvuruyla paylaşılır (3.6); bağın sabitliği okunabilir ve denetlenebilir bir sözdür, içeriğin değişip değişmediği ayrı bir sorudur.

### 5.3

- *Emsal:* Python ve Go virgüllü atamada sağ yanı önce hesaplar (`a, b = b, a`); Python liste açarken öğe sayısı tutmazsa hata verir. ([Python 3.15](https://docs.python.org/3/reference/simple_stmts.html#assignment-statements), [Go 1.27](https://go.dev/ref/spec#Assignment_statements))
- *Gerekçe:* takas ve çoklu ilk değer tek satırda okunur; eksik ya da fazla öğe sessizce kaybolmaz. Gerekmeyen değer için `_` Python alışkanlığıdır (`_, b = f()`, `for _ in range(n)`); `_` okunamadığı için yanlışlıkla kullanılamaz.

### 5.4

- *Emsal:* Python'da her ikili işlecin bileşik ataması vardır (`//=`, `**=`, `&=` ...) ve değiştirilebilir nesnede yerinde çalışır: "when possible, the actual operation is performed in-place, meaning that rather than creating a new object and assigning that to the target, the old object is modified instead" (`m = l; l += [2]` `m`'yi de uzatır [ölçüldü: Python 3.12.6]). *Almadık:* Python'da bileşik atamanın değişkenin tipini değiştirebilmesi (`n /= 2` bir tam sayıyı ondalığa çevirir); bugünkü TR'nin listede `+=`'yi yeni liste kurup atayarak yapması (paylaşan ad eski listede kalıyor [ölçüldü]). ([Python 3.15](https://docs.python.org/3/reference/simple_stmts.html#augmented-assignment-statements))
- *Gerekçe:* Python alışkanlığıyla yazılan `x = 5` tanımsız ada atamadır; gürültülü hata verir ve ileti `TR_var`'ı önerir. Hedefin bir kez hesaplanması Python'daki gibidir ("In the augmented version, `x` is only evaluated once"). `l *= 3` de 4.10'un yineleme kuralına uyar, yoksa iç içe listenin sessiz yanlışı bu yoldan geri gelirdi (DENETİM). Her işlecin bileşik biçimi olunca kodlama modeli alışkanlıkla doğru yazar; tipin değişmemesi 3.13'ün kalıcı tip kuralıdır, ileti doğru işleci söyler. Listeler başvuruyla paylaşıldığı için (3.6) Python'un yerinde `+=`'si TR'de de aynı sonucu verir; ayrı bir anlam, Python alışkanlığıyla yazılmış kodda sessiz bir fark olurdu (DENETİM, 2026-10-27).

### 5.5

- *Emsal:* Rust'ta `if` bir ifadedir ("An `if` expression must have the same type in all situations"); Go, Rust ve Swift gövdeyi her zaman süslü parantezle yazar. *Almadık:* C ailesinde süslü parantezsiz gövde (Apple'ın 2014'teki "goto fail" güvenlik açığı tek satırlık gövdenin yanlış okunmasıydı); Python'un `elif`'i (TR'de `TR_else TR_if`); Python'un `a if c else b`'si (aynı işin ikinci yazımı, 2.9; liste üretecinin `TR_if`'iyle de karışırdı). ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/if-expr.html), [Go 1.27](https://go.dev/ref/spec#If_statements), [CVE-2014-1266](https://nvd.nist.gov/vuln/detail/CVE-2014-1266))
- *Gerekçe:* değer veren `TR_if`'in her dalı bir değer vermek zorundadır; `TR_else`'siz bir ifade sessizce yokluk vermez.

### 5.6

- *Emsal:* Swift: "Every `switch` statement must be *exhaustive*"; Rust'ta `match` da her durumu kapsar. Swift'te düz ad deseni değeri karşılaştırır, bağlamak için `let` yazılır. Python karşılaştırmayı noktalı adla ayırır: "The dotted name in the pattern is looked up using standard Python name resolution rules"; düz ad ise her zaman yakalar. *Almadık:* Python'un ve Rust'ın yakalama deseni: düz ad her şeyle eşleşir ve konuyu o ada bağlar ("Capture patterns always succeed"; `case GREEN:` hiçbir zaman karşılaştırma değildir); Swift'in değişken adını düz yazımla karşılaştırması; Python'da eşleşme yoksa sessizce hiçbir şey olmaması; bugünkü TR'nin `TR_default`'u (2.9). ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/controlflow/#Switch), [Rust 1.99 kitabı](https://doc.rust-lang.org/book/ch06-02-match.html), [Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#capture-patterns))
- *Gerekçe:* unutulmuş bir durum çalışmadan önce görünür; bir sabitle karşılaştırırken yazılan ad sessizce her şeyi yakalamaz. Python ya da Rust alışkanlığıyla `x =>` yazan model bağlama bekler. Ad tanımsızsa 5.14, kol kapsamıyorsa kapsama denetimi hata verir. Ama ad tanımlı bir değişkense ve bir `_` kolu da varsa, kol sessizce karşılaştırma olurdu (DENETİM, 2026-10-27). Düz adın yalnız sabit olabilmesi bu son durumu da gürültülü yapar; değişkenle karşılaştırma `(x)` diye açıkça yazılır. Tek istisna durumlu tipin deseninin alanlarıdır (7.7): `Durum.Çalışıyor(a)`'da desen bir değer değil, bir durumu açar; alanın adı orada bağlanır ve değer karşılaştırması kolun içinde yapılır.

### 5.7

- *Emsal:* Go'nun `for i, x := range` biçimi indeksle öğeyi verir; Python dolaşılırken boyu değişen sözlükte hata verir: "dictionary changed size during iteration" [ölçüldü: Python 3.12.6]; belge yalnız "may raise a RuntimeError or fail to iterate over all entries" der. *Almadık:* Python'da dolaşılırken boyu değişen listenin sessizce öğe atlaması ya da yinelemesi. ([Go 1.27](https://go.dev/ref/spec#For_range), [Python 3.15](https://docs.python.org/3/library/stdtypes.html#dictionary-view-objects))
- *Gerekçe:* dolaşırken listeyi ya da sözlüğü büyütüp küçültmek yaygın bir yanlıştır ve sonucu belirsizdir; hata o anda görünür. Var olan öğeye atama boyu değiştirmez, Python'da da serbesttir (`l[i] = x * 10`, `d[k] = 0` dolaşırken çalışır [ölçüldü: Python 3.12.6]); öğeleri yerinde dönüştürmek sık bir iştir.
- *Gerekçe (dolaşılamayan JSON türleri):* serde_json'da `Value` dolaşılmaz; dizi ve nesne `as_array()` ve `as_object()` ile açılır. TR dizide ve nesnede doğrudan dolaşır, öbür türlerde hata verir: Python'daki gibi bir JSON metninde sessizce karakterleri dolaşmak yanlış olurdu. ([serde_json 1.0.154 Value](https://docs.rs/serde_json/1.0.154/serde_json/enum.Value.html))

### 5.8

- *Emsal:* Go'nun tek döngü sözcüğü `for`'dur. ([Go 1.27](https://go.dev/ref/spec#For_statements))
- *Gerekçe:* 2.9'un tek yazım kuralı.

### 5.9

- *Emsal:* Go'da `for { }`, Rust'ta `loop { }` sonsuz döngüdür. ([Go 1.27](https://go.dev/ref/spec#For_statements), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/loop-expr.html))
- *Gerekçe:* sunucu ve robot döngüsü sonsuzdur; çıkış `TR_break` ya da durdurma isteğiyle olur (1.5, 1.6).

### 5.10

- *Emsal:* Go, Rust ve Python'da `continue`. ([Go 1.27](https://go.dev/ref/spec#Continue_statements))
- *Gerekçe:* turun geri kalanını atlamanın tek yazımı.

### 5.11

- *Emsal:* Python'un liste üreteci (PEP 202). ([PEP 202](https://peps.python.org/pep-0202/))
- *Gerekçe:* kodlama modelleri listeyi en sık bu biçimle kurar; değişkenin üretecin dışına sızmaması Python 3'teki gibidir (`x = 1; [x for x in [5, 6]]` sonrasında `x` 1 kalır [ölçüldü: Python 3.12.6]). Üretecin değişkeni bu yüzden 5.12'nin dışındadır: Python alışkanlığıyla yazılan kod aynı sonucu verir, sessiz bir fark doğmaz.

### 5.12

- *Emsal:* C#: "within a nested declaration space it is not possible to declare a local variable or constant with the same name as a local variable or constant in an enclosing declaration space" (CS0136). *Almadık:* Rust'ın ve Go'nun gölgelemeye izin vermesi. ([C# CS0136](https://learn.microsoft.com/en-us/dotnet/csharp/misc/cs0136))
- *Gerekçe:* iç blokta yanlışlıkla yazılan `TR_var t = 0` atama yerine yeni bir değişken tanımlar ve dıştaki değer sessizce değişmez; bu hatanın çalışmadan önce görünmesi gerekir. Python'da blok kapsamı yoktur, betikler işi en dışta yapar; bu yüzden programın en dışı da kurala girer. Döngü değişkeni dıştaki bir adla aynıysa da hata verir: Python'da döngüden sonra o ad son değeri taşır, TR'de taşımaz. Bu ikinci anlamın sessiz kalmasını hata önler. Fonksiyon sınırında gizleme serbesttir: fonksiyon ifadesinin parametresi (`(x) => x * 2`) dıştaki `x`'le karışmaz. Sondaki blok Python'un `with`'inin karşılığıdır, ama ondan farklı olarak kapsam açar. `with` alışkanlığıyla yazan model içerideki atamanın dıştakini değiştirdiğini bekler; TR'de `TR_var kayıp = ...` ise dıştaki `kayıp`'ı sessizce gizlerdi. Bu yüzden sondaki blok bu kural için sıradan bloktur (DENETİM, 2026-10-27).

### 5.13

- *Emsal:* Go, Rust ve Python'da `break` ve `continue` fonksiyon sınırını geçemez. ([Go 1.27](https://go.dev/ref/spec#Break_statements))
- *Gerekçe:* bir fonksiyonun içindeki `TR_break`'in hangi döngüyü bitirdiği belirsiz olmasın.

### 5.14

- *Emsal:* Go ve Rust adları derlemede çözer; paket düzeyindeki tanımlar sırasından bağımsız görünür. Go paket düzeyindeki değişkenleri bağımlılık sırasıyla başlatır ("a package-level variable is considered ready for initialization if it is not yet initialized and either has no initialization expression or its initialization expression has no dependencies on uninitialized variables"), döngüyü reddeder ("those variables are part of one or more initialization cycles, and the program is not valid"). Rust'ta blokta tanımlanan öğe bütün blokta görünür: "The name of an item declared as a statement has a scope that extends from the start of the block the item statement is in until the end of the block." Swift'in yerel fonksiyonlar için benzer bir denetimi var ("closure captures %0 before it is declared"); kullanım yerine mi tanım sırasına mı baktığı denenmedi [hipotez] ([swiftlang/swift, DiagnosticsSIL.def](https://github.com/swiftlang/swift/blob/main/include/swift/AST/DiagnosticsSIL.def)). Python 3.10'dan beri bulunamayan ad iletisi benzer adı önerir. *Almadık:* Python'da en dıştaki değişkeni okuyan fonksiyonun, değişkenden önce çağrılınca ancak çalışırken hata vermesi; bir modülün kendisini aktaranın adlarını görmesi (modülün anlamı aktarana bağlı olur, tek başına denetlenemez). ([Go 1.27](https://go.dev/ref/spec#Package_initialization), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/names/scopes.html#item-scopes), [Python 3.10 yenilikleri](https://docs.python.org/3/whatsnew/3.10.html#nameerror))
- *Gerekçe:* yazım yanlışı bir ad çalışmadan önce bulunur ve ileti doğrusunu söyler; her modül tek başına denetlenir (1.3). Fonksiyonların sıradan bağımsız görünmesi karşılıklı çağrıyı ve Python'daki "önce fonksiyonlar, sonra ana kod" düzenini korur. Çağrının fonksiyonun kullandığı değişkenlerden sonra gelmesi, bugün çalışırken ortaya çıkan "henüz tanımlanmamış" hatasını çalışmadan önceye taşır (bugün sonradan tanımlanan en dış değişkeni okuyan fonksiyon çalışıyor [ölçüldü, DENETİM]).
