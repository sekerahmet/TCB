# 4. İşleçler: emsal ve gerekçe

Kurallar: [`04_operatorler.md`](../04_operatorler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 4.1

- *Emsal:* Python: "Unlike C, all comparison operations in Python have the same priority, which is lower than that of any arithmetic, shifting or bitwise operation." Rust'ta karşılaştırmalar aynı düzeyde ve birleşmez. Swift'te `??` toplamadan gevşek, karşılaştırmadan sıkıdır. *Almadık:* C'nin bit işleçlerini `==`'den gevşek tutması (`x & 1 == 0`, `x & (1 == 0)` diye okunur); bugünkü TR'nin `??`'yi en gevşek tutması (`x ?? 0 > 3` yanlış okunurdu). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#comparisons), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#comparison-operators), [Swift işleç bildirimleri](https://developer.apple.com/documentation/swift/operator-declarations))
- *Gerekçe:* bit maskesi sınaması (`x & 1 == 0`) ve varsayılanlı karşılaştırma (`x ?? 0 > 3`) parantezsiz doğru okunur. Python alışkanlığıyla yazan model ve kişi aynı öncelikleri bulur.

### 4.2

- *Emsal:* Python 3: "Division of integers yields a float"; PEP 238 eski bölmenin sorununu yazar: "if the expressions for both arguments happen to have an integral type, it implements floor division rather than true division". Kotlin'de "you can combine different numeric types in arithmetic expressions". *Almadık:* Go'nun, Rust'ın ve C'nin tam sayıda `/`'yi kesmesi (Rust: "Integer division rounds towards zero"); bugünkü TR'nin sonucu değere göre tam sayı ya da ondalık vermesi (statik tiple bağdaşmaz); Swift'in aritmetikte de dönüşümü açık istemesi. ([Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-arithmetic-operations), [PEP 238](https://peps.python.org/pep-0238/), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#arithmetic-and-logical-binary-operators), [Go 1.27](https://go.dev/ref/spec#Integer_operators))
- *Gerekçe:* `/`'nin sonucunun tipi değere bağlı olmaz; ortalama kayıp (`toplam / n`) her zaman doğru çıkar. Öğrenme oranı ile adım sayısı gibi karışık formüller dönüşüm yazmadan okunur.

### 4.2a

- *Emsal:* Python: "floor division of integers results in an integer; the result is that of mathematical division with the 'floor' function applied to the result" ve "The modulo operator always yields a result with the same sign as its second operand". Haskell'in `div` ve `mod`'u da aşağı yuvarlar. *Almadık:* Go'nun ve Rust'ın kesen bölmesi ve kalanı (`-7 % 3` = −1); tam bölme için ayrı bir fonksiyon (Python alışkanlığıyla yazılan `//` yine yazılırdı). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-arithmetic-operations), [Haskell Prelude](https://hackage.haskell.org/package/base/docs/Prelude.html#v:div), [Go 1.27](https://go.dev/ref/spec#Integer_operators))
- *Gerekçe:* kodlama modelleri tam bölmeyi Python'daki gibi `//` ile yazar; aynı yazım TR'de aynı anlamı taşır. Halka arabellek, parti ve adım hesabında eksi sayıyla da doğru indeks çıkar (`(i - 1) % n` hep 0 ile n−1 arasında); bölme ile kalan birbirini tutar.

### 4.3

- *Emsal:* Rust'ta tam sayının üssü `pow(u32)` ile, yalnız eksi olmayan üsle alınır. *Almadık:* Python'un eksi üste ondalık vermesi (sonucun tipi değere bağlı olur). ([Rust 1.99 i64::pow](https://doc.rust-lang.org/std/primitive.i64.html#method.pow), [Python 3.15](https://docs.python.org/3/reference/expressions.html#the-power-operator))
- *Gerekçe:* `2 ** n` gibi boyut hesapları tam sayı kalır; tipi çalışmadan önce bellidir.

### 4.4

- *Emsal:* Rust: "Parentheses are required when chaining comparison operators. For example, the expression `a == b == c` is invalid". Python, Go ve Rust metinleri sözlük sırasıyla karşılaştırır (UTF-8 bayt sırası kod noktası sırasıyla aynıdır). *Almadık:* Python'un zinciri ("`x < y <= z` is equivalent to `x < y and y <= z`"): TR'de tek anlam, ileti doğru yazımı söyler. ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#comparison-operators), [Python 3.15](https://docs.python.org/3/reference/expressions.html#comparisons), [Go 1.27](https://go.dev/ref/spec#Comparison_operators))
- *Gerekçe:* `a < b == c` gibi bir ifade iki türlü okunamaz. Sözcükleri sıralamak için metin karşılaştırması gerekir.

### 4.5

- *Emsal:* Go'da fonksiyon tipleri karşılaştırılamaz; Go, Rust ve Swift'te ayrı tipler `==` ile karşılaştırılamaz, derleme hatasıdır. *Almadık:* Python'un ayrı tipleri sessizce eşit saymaması (`"1" == 1`, `False` döner, hata vermez); fonksiyonları kimlikle karşılaştırmak (görünüşü aynı iki fonksiyon eşit sayılmaz, kafa karıştırır). ([Go 1.27](https://go.dev/ref/spec#Comparison_operators), [Python 3.15](https://docs.python.org/3/reference/expressions.html#value-comparisons))
- *Gerekçe:* her zaman yanlış çıkan bir karşılaştırma bir yanlışı saklar; çalışmadan önce hata olur. Tam sayı ile ondalığın kesin karşılaştırılması büyük adım ve bayt sayılarında yanlış eşitlik vermez.

### 4.6

- *Emsal:* Python mantık işlecini sözcükle yazar (`and`, `or`, `not`); Go ve Rust işlenenin mantık tipinde olmasını ister. *Almadık:* Python'un işlenenin kendisini döndürmesi (`1 and "a"` → `"a"`). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#boolean-operations), [Go 1.27](https://go.dev/ref/spec#Logical_operators))
- *Gerekçe:* mantık işleminin sonucu her zaman mantık değeridir; 3.10'un koşul kuralıyla aynı.

### 4.7

- *Emsal:* Swift'te `??`'nin sol yanı her zaman isteğe bağlıdır, sağ yanı içteki tiple aynıdır; Kotlin'de `?:`. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/basicoperators/#Nil-Coalescing-Operator), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html#elvis-operator))
- *Gerekçe:* varsayılan değer isteğe bağlı değeri açmanın kısa yoludur (3.5); hiç kullanılmayacak bir sağ taraf bir yanlışı gösterir.

### 4.8

- *Emsal:* Swift'in ve Kotlin'in isteğe bağlı zinciri sonucu isteğe bağlı verir. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html#safe-call-operator))
- *Gerekçe:* yok olabilecek bir alanın içine güvenle inilir; yokluk tipte görünmeye devam eder.

### 4.9

- *Emsal:* Python'da `&&`, `||` ve `!` yoktur, sözcükleri vardır; F# ve Elixir `|>` ile çağırır. *Almadık:* C ailesinin sembolleri ve `|>`: aynı işin ikinci yazımı olurdu (2.9); C ailesinin `//` ve `/* */` yorumu (2.2). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#boolean-operations), [F#](https://learn.microsoft.com/en-us/dotnet/fsharp/language-reference/symbol-and-operator-reference/))
- *Gerekçe:* her işin tek yazımı vardır; kodlama modelleri başka dillerin işlecini ve yorumunu alışkanlıkla yazar. Bu yazımların hepsi sessiz bir yanlış değil, sözdizimi hatası olur ve ileti doğrusunu söyler; düzeltme ilk denemede tutar.

### 4.10

- *Emsal:* Python, Go, Rust ve Swift metne başka bir tipi `+` ile eklemez (Python: `TypeError`). *Almadık:* Java'nın, JavaScript'in ve Kotlin'in metne her değeri kendiliğinden çevirip eklemesi. ([Python 3.15](https://docs.python.org/3/library/stdtypes.html#common-sequence-operations), [Go 1.27](https://go.dev/ref/spec#Arithmetic_operators))
- *Gerekçe:* metin kurmanın tek yazımı yerleştirmedir (2.7); kendiliğinden dönüşüm bir yanlış tipi saklar.

### 4.11

- *Emsal:* Python, matris çarpımı için `@` işlecini PEP 465 ile ekledi; NumPy ve PyTorch onu kullanır. ([PEP 465](https://peps.python.org/pep-0465/))
- *Gerekçe:* modellerin çekirdek işlemi matris çarpımıdır; formüldeki gibi okunur.

### 4.12

- *Emsal:* Python, Go, Rust ve C aynı bit işleçlerini kullanır. Rust kaydırma miktarının türün bit sayısını aşmasını taşma sayar ("Using `<<` or `>>` where the right-hand argument is greater than or equal to the number of bits in the type of the left-hand argument, or is negative") ve işaretli sayıda "Arithmetic right shift" yapar. *Almadık:* Go'nun aşırı kaydırmayı sıfıra çevirmesi. ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#arithmetic-and-logical-binary-operators), [Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-bitwise-operations), [Go 1.27](https://go.dev/ref/spec#Arithmetic_operators))
- *Gerekçe:* robotun yazmaçları, bit maskeleri, 4 bitlik nicemlenmiş ağırlıkların paketlenmesi ve özetleme işlevleri bit işlemi ister; 2.4'ün onaltılık ve ikilik yazımı bunlar içindir.
