# 6. Fonksiyonlar: emsal ve gerekçe

Kurallar: [`06_fonksiyonlar.md`](../06_fonksiyonlar.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 6.1

- *Emsal:* Python'un tip açıklaması (`def f(x: int) -> int`), Rust ve Swift dönüş tipini `->` ile yazar. Kotlin blok gövdeli fonksiyonda dönüş tipini ister: "When you declare a function with a block body (by putting instructions within curly braces `{}`), you must always specify a return type explicitly" ([Kotlin 2.4](https://kotlinlang.org/docs/functions.html)). Go ve Python'da aynı adla aşırı yükleme yoktur ([Go SSS](https://go.dev/doc/faq#overloading)). *Almadık:* TypeScript'in ve Kotlin'in `: R`'si (aynı işin ikinci yazımı olurdu, 2.9); Swift'in, Kotlin'in ve C#'ın parametre tipine göre aşırı yüklemesi; Python'da ikinci tanımın birinciyi sessizce değiştirmesi (bugünkü TR'de de ikincisi kazanıyor [ölçüldü]); Python'un `@` süsleyicisi (TR'de `@` matris çarpımı, 4.11). ([Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#function-definitions), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/items/functions.html), [Go 1.27](https://go.dev/ref/spec#Function_declarations))
- *Gerekçe:* parametre ve dönüş tipi fonksiyonun sözleşmesidir; çağıran ve kodlama modeli onu okuyarak doğru argümanı verir, yanlışı çalışmadan önce görünür (1.2). Python alışkanlığıyla yazılan `->` aynen geçerlidir. Tek ad tek fonksiyon olunca bir çağrının hangi tanıma gittiği aranmaz.

### 6.2

- *Emsal:* Go: "If the function's signature declares result parameters, the function body's statement list must end in a terminating statement"; Go'nun bitiren deyimleri arasında `panic` çağrısı da vardır. Swift eksik dönüşü derlemede reddeder: "missing return in %kindonly1 expected to return %0". *Almadık:* Python'da dönüşü unutulmuş yolun sessizce `None` vermesi (`if x > 0: return 1` için `f(-1)` → `None` [ölçüldü: Python 3.12.6]; bugünkü TR'de de yokluk [ölçüldü]); Rust'ın gövdenin son ifadesini döndürmesi; Kotlin'in ve Swift'in tek ifadeli gövdesi. ([Go 1.27](https://go.dev/ref/spec#Terminating_statements), [swiftlang/swift, DiagnosticsSIL.def](https://github.com/swiftlang/swift/blob/main/include/swift/AST/DiagnosticsSIL.def), [Kotlin 2.4](https://kotlinlang.org/docs/functions.html), [Python 3.15](https://docs.python.org/3/reference/simple_stmts.html#the-return-statement))
- *Gerekçe:* unutulmuş bir dönüş, hiçbir hata vermeden yanlış bir değer olarak yayılırdı. Python alışkanlığı zaten `TR_return` yazar. Son ifadenin örtük dönüşü ise sondaki bir çağrının değerini kazara döndürürdü; değer veren bloklar (`TR_if`, `TR_match`, sondaki blok) ayrı ve açıktır. Hata fırlatan yol bitmiş sayılır, yoksa "eksi olamaz" diye biten bir fonksiyon yersiz bir eksik dönüş hatası alırdı (DENETİM). Kotlin alışkanlığıyla yazılan `(x) => { x * 2 }` de aynı kurala girer ve gürültülü hata verir.

### 6.3

- *Emsal:* Go'da fonksiyon birden çok sonuç döndürür ve sonuç tek bir değer olarak saklanamaz; Python bir demet döndürür (`return loss, acc`) ve açar. *Almadık:* ayrı bir demet tipi: `r[0]` ve `r[1]` anlamı saklar, yeni bir tip adı gerekirdi; Python'un açmadan tek değer olarak tutması. ([Go 1.27](https://go.dev/ref/spec#Return_statements), [Python 3.15](https://docs.python.org/3/reference/simple_stmts.html#assignment-statements))
- *Gerekçe:* ML kodunda kaybı ve doğruluğu birlikte döndürmek çok sıktır; Python alışkanlığıyla yazılan `TR_return a, b` ve `TR_var a, b = f()` doğrudan geçerlidir. Değer sayısı tipte yazılı olduğu için eksik açma çalışmadan önce görünür. `TR_return f()` ile iletme Go'daki gibidir; `g(f())` ise değerlerin hangi parametreye gittiğini saklar, açarak yazılır.

### 6.4

- *Emsal:* Python: "Default parameter values are evaluated from left to right when the function definition is executed. This means that the expression is evaluated once, when the function is defined, and that the same “pre-computed” value is used for each call." Python'un sırası: "If a parameter has a default value, all following parameters up until the “`*`” must also have a default value" (ihlali "parameter without a default follows parameter with a default" hatası verir [ölçüldü]). *Almadık:* Python'un varsayılanı bir kez hesaplaması; Python SSS bu yüzden uyarır: "Why are default values shared between objects?" ([Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#function-definitions), [Python SSS](https://docs.python.org/3/faq/programming.html#why-are-default-values-shared-between-objects))
- *Gerekçe:* `liste: TR_List[TR_Int] = []` her çağrıda boş başlar; Python'un bilinen tuzağı TR'de yoktur, Python alışkanlığıyla yazılan kod beklenen sonucu verir. Varsayılanların sonda durması Python alışkanlığıyla aynıdır.

### 6.5

- *Emsal:* Swift adlı argümanı `ad: değer` diye yazar; TR'de adlı argüman 2.3a'dan beri iki noktayla yazılır (`TR_lr: 0.003`). Go'da ve Rust'ta değişken sayıda kullanıcı parametresi ya yoktur ya dilimle yazılır. *Almadık:* Python'un `ad=değer`'i (`=` TR'de atamadır, iki anlam olurdu); Python'un `*args` ve `**kwargs`'ı (tipleri çalışmadan önce denetlenemez). ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/functions/), [Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#function-definitions))
- *Gerekçe:* adlı argüman, eğitim ayarları gibi uzun argüman listelerinde neyin ne olduğunu söyler. Python alışkanlığıyla yazılan `k=2` gürültülü hata verir ve ileti doğrusunu söyler. Eksik ve fazla argüman bugün ancak çalışırken bulunuyor [ölçüldü]; statik tiple çalışmadan önce bulunur.

### 6.6

- *Emsal:* JavaScript, TypeScript ve C# fonksiyon ifadesini `(a, b) => ifade` diye yazar ([C# lambda](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/lambda-expressions), [TypeScript](https://www.typescriptlang.org/docs/handbook/2/functions.html)). *Almadık:* Python'un `lambda`'sı (yalnız tek ifade alır); bugünkü TR'nin üç yazımı (`x => ...`, `(x) => ...`, adsız `TR_func(x) { ... }`): aynı işin üç yazımı (2.9). ([MDN ok fonksiyonları](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Functions/Arrow_functions), [Python 3.15](https://docs.python.org/3/reference/expressions.html#lambda))
- *Gerekçe:* depodaki fonksiyon ifadelerinin çoğu zaten parantezli oktur (487 kullanım; parantezsiz 21, adsız `TR_func` 52 [ölçüldü]). Tipin beklenen yerden çıkması `l.TR_filter((x) => x > 2)`'yi tip yazmadan bırakır. Fonksiyon ifadesine ad verilemeyince adın hangi kapsama girdiği sorusu da kalkar.

### 6.7

- *Emsal:* Kotlin: "According to Kotlin convention, if the last parameter of a function is a function, then a lambda expression passed as the corresponding argument can be placed outside the parentheses"; Swift'te ve Ruby'de de sondaki blok vardır. Python aynı işi `with` deyimiyle yapar. *Almadık:* Kotlin'in örtük `it` parametresi ve bugünkü TR'nin `TR_it` ve `TR_acc`'si (TR'ye özgü adları model bilmez; parametreli fonksiyonun ikinci yazımı olurdu); Kotlin'de blok içindeki `return`'ün dıştaki fonksiyondan dönmesi ("a `return` inside a lambda expression will return from the enclosing function"). ([Kotlin 2.4](https://kotlinlang.org/docs/lambdas.html#passing-trailing-lambdas), [Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#the-with-statement))
- *Gerekçe:* kapsamlı işler (karışık hassasiyet, zaman ölçümü, hatanın yakalanması) bir blokla okunur; Python'un `with`'iyle yazan model aynı biçimi bulur. Blokta `TR_return`'ün hata olması, Python'un `with` içinden dönme alışkanlığını sessiz bir anlam yerine gürültülü bir hataya çevirir. Parametreli fonksiyon için tek yazım ok olduğundan iki yazım kalmaz. Blokta erken çıkış bugün yoktur; blok içinde `TR_if`/`TR_else` ifadesiyle yazılır. Gemini'nin "bloktaki `TR_return` bloktan dönsün" önerisi alınmadı: Python'un `with` içinden dönme alışkanlığı o zaman sessizce yanlış yere dönerdi (DENETİM, 2026-10-27).

### 6.8

- *Emsal:* Go 1.22'den beri: "Each iteration has its own separate declared variable (or variables)". Swift, Kotlin ve JavaScript kapanışı dıştaki değişkeni başvuruyla yakalar ve ona atar. *Almadık:* Python'un geç bağlaması (SSS: "Why do lambdas defined in a loop with different values all return the same result?", çünkü "it is accessed when the lambda is called — not when it is defined"); Python'un `global` ve `nonlocal`'ı ("If a name is bound in a block, it is a local variable of that block, unless declared as nonlocal or global"). ([Go 1.27](https://go.dev/ref/spec#For_clause), [Python SSS](https://docs.python.org/3/faq/programming.html#why-do-lambdas-defined-in-a-loop-with-different-values-all-return-the-same-result), [Python 3.15](https://docs.python.org/3/reference/executionmodel.html#resolution-of-names))
- *Gerekçe:* her turun kendi değişkeni, döngüde kurulan geri çağırmaların doğru değeri görmesini sağlar (bugünkü TR'de de böyle [ölçüldü]). Python'da atama tanım da olduğu için `global` gerekir; TR'de tanım her zaman `TR_var`'dır, atama ile tanım karışmaz. Kalan risk: Python'da fonksiyonun içindeki `lr = ...` yereldir; `TR_var`'ı unutan model en dıştaki `lr`'yi değiştirir [hipotez]. Dıştaki değişkene okumadan önce atama, "yerel yazmak istedim" işaretidir; uyarı bu durumu gösterir (DENETİM'in önerisi). Kalan sıklık ajan deneyinde ölçülür.

### 6.9

- *Emsal:* Swift ve Kotlin fonksiyon tipini `(Int) -> Bool` diye yazar; Swift'te fonksiyon tipi argüman etiketlerini taşımaz. *Almadık:* Python'un `Callable[[int], bool]`'ı. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/types/#Function-Type), [Kotlin 2.4](https://kotlinlang.org/docs/lambdas.html#function-types))
- *Gerekçe:* fonksiyon tipi tanımın yazımıyla aynı okunur (`->`). `->`'nin sağdan birleşmesi Swift'teki gibidir; bir imzada tek `->` olduğundan `-> (A, B) -> C` başka türlü okunamaz. Fonksiyonun metni bellek adresi taşımaz (Python'un `<function f at 0x...>`'i her koşuda değişir), bu yüzden aynı program aynı metni verir (Güvenilir).

### 6.10

- *Emsal:* Python 3.12'den beri: "A list of type parameters may be given in square brackets between the function’s name and the opening parenthesis for its parameter list" (`def func[T](arg: T): ...`); Go da köşeli parantez kullanır. *Almadık:* Rust'ın, Swift'in ve C#'ın `<T>`'si (`<` karşılaştırmayla karışır). ([Python 3.15](https://docs.python.org/3/reference/compound_stmts.html#type-params), [Go 1.27](https://go.dev/ref/spec#Type_parameter_declarations))
- *Gerekçe:* kitaplık parametrik yazılır, tip sabitlenmez; Python'un yeni yazımı ve `TR_List[T]` ile aynı köşeli parantez. Kısıtsız tip parametresinde yalnız her tipte geçerli işler yapılır; böylece genel bir fonksiyon her tiple çağrılınca çalışmadan önce doğru ya da yanlış olduğu bellidir. Eşitlik isteyen üyeler Swift'teki gibi kısıt ister (`contains` orada `Equatable` ister); sözlük anahtarı 3.7'nin kısıtlı tipleridir.

### 6.11

- *Emsal:* Python'un özyineleme sınırı varsayılan olarak 1000'dir [ölçüldü: Python 3.12.6]. *Almadık:* yığının sessizce taşıp sürecin çökmesi.
- *Gerekçe:* sınırsız özyineleme robotta ve sunucuda süreci düşürür; sınır aşılınca hata yerinde ve okunur biçimde görünür.
