# 4. İşleçler: emsal ve gerekçe

Kurallar: [`04_operatorler.md`](../04_operatorler.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 4.1

- *Emsal:* Python: "Unlike C, all comparison operations in Python have the same priority, which is lower than that of any arithmetic, shifting or bitwise operation." Rust'ta karşılaştırmalar aynı düzeyde ve birleşmez. Swift'te `??` toplamadan gevşek, karşılaştırmadan sıkıdır. Python'un üs kuralı: "The power operator binds more tightly than unary operators on its left; it binds less tightly than unary operators on its right." *Almadık:* C'nin bit işleçlerini `==`'den gevşek tutması (`x & 1 == 0`, `x & (1 == 0)` diye okunur); bugünkü TR'nin `??`'yi en gevşek tutması (`x ?? 0 > 3` yanlış okunurdu). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#comparisons), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#comparison-operators), [Swift işleç bildirimleri](https://developer.apple.com/documentation/swift/operator-declarations))
- *Gerekçe:* bit maskesi sınaması (`x & 1 == 0`) ve varsayılanlı karşılaştırma (`x ?? 0 > 3`) parantezsiz doğru okunur. Python alışkanlığıyla yazan model ve kişi aynı öncelikleri bulur.

### 4.2

- *Emsal:* Python 3: "Division of integers yields a float"; PEP 238 eski bölmenin sorununu yazar: "if the expressions for both arguments happen to have an integral type, it implements floor division rather than true division". Kotlin'de "you can combine different numeric types in arithmetic expressions". *Almadık:* Go'nun, Rust'ın, Kotlin'in ve C'nin tam sayıda `/`'yi kesmesi (Rust: "Integer division rounds towards zero"); bugünkü TR'nin sonucu değere göre tam sayı ya da ondalık vermesi (statik tiple bağdaşmaz); Swift'in aritmetikte de dönüşümü açık istemesi. ([Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-arithmetic-operations), [PEP 238](https://peps.python.org/pep-0238/), [Kotlin 2.4](https://kotlinlang.org/docs/numbers.html), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#arithmetic-and-logical-binary-operators), [Go 1.27](https://go.dev/ref/spec#Integer_operators))
- *Gerekçe:* `/`'nin sonucunun tipi değere bağlı olmaz; ortalama kayıp (`toplam / n`) her zaman doğru çıkar. Öğrenme oranı ile adım sayısı gibi karışık formüller dönüşüm yazmadan okunur.

### 4.2a

- *Emsal:* Python: "floor division of integers results in an integer; the result is that of mathematical division with the 'floor' function applied to the result" ve "The modulo operator always yields a result with the same sign as its second operand"; ondalıkta özdeşlik yaklaşıktır, Python bölmeyi "be very close to `x`" diye tutar (`-1e-100 // 1e100` ile `-1e-100 % 1e100` özdeşliği tutmaz [ölçüldü: Python 3.12.6, DENETİM]). Haskell'in `div` ve `mod`'u da aşağı yuvarlar. *Almadık:* Go'nun ve Rust'ın kesen bölmesi ve kalanı (`-7 % 3` = −1); tam bölme için ayrı bir fonksiyon (Python alışkanlığıyla yazılan `//` yine yazılırdı). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-arithmetic-operations), [Haskell Prelude](https://hackage.haskell.org/package/base/docs/Prelude.html#v:div), [Go 1.27](https://go.dev/ref/spec#Integer_operators))
- *Gerekçe:* kodlama modelleri tam bölmeyi Python'daki gibi `//` ile yazar; aynı yazım TR'de aynı anlamı taşır. Halka arabellek, parti ve adım hesabında eksi sayıyla da doğru indeks çıkar (`(i - 1) % n` hep 0 ile n−1 arasında); bölme ile kalan birbirini tutar.

### 4.3

- *Emsal:* Rust'ta tam sayının üssü `pow(u32)` ile, yalnız eksi olmayan üsle alınır. Python `0.0 ** -1`'de hata verir ("0.0 cannot be raised to a negative power"), `math.pow(-8, 1/3)`'te de ("math domain error") [ölçüldü: Python 3.12.6]. *Almadık:* Python'un eksi üste ondalık vermesi (sonucun tipi değere bağlı olur); Python'un `(-8) ** (1/3)`'e karmaşık sayı vermesi (TR'de karmaşık sayı yok); Go'nun sessiz sonucu ("Pow(x, y) = NaN for finite x < 0 and finite non-integer y", sıfırın eksi üssünde sonsuz; bugünkü TR de sonsuz ve NaN veriyor [ölçüldü]). ([Rust 1.99 i64::pow](https://doc.rust-lang.org/std/primitive.i64.html#method.pow), [Python 3.15](https://docs.python.org/3/reference/expressions.html#the-power-operator), [Go 1.27 math.Pow](https://pkg.go.dev/math#Pow))
- *Gerekçe:* `2 ** n` gibi boyut hesapları tam sayı kalır; tipi çalışmadan önce bellidir. Tanımsız bir üs sessizce `TR_nan` olup eğitimin kaybına karışmaz, yerinde görünür; 4.2'nin sıfıra bölme hatasıyla aynı. Kesirli üssün her makinede aynı olması "aynı program aynı veriyle aynı sonucu verir" ilkesidir (13.1).

### 4.4

- *Emsal:* Python: "Comparisons can be chained arbitrarily, e.g., `x < y <= z` is equivalent to `x < y and y <= z`, except that `y` is evaluated only once". Rust `==`'li zinciri reddeder: "Parentheses are required when chaining comparison operators. For example, the expression `a == b == c` is invalid". Python, Go ve Rust metinleri sözlük sırasıyla karşılaştırır (UTF-8 bayt sırası kod noktası sırasıyla aynıdır). *Almadık:* Python'un `==`'li zinciri; Rust'ın sıralama zincirini de reddetmesi. ([Python 3.15](https://docs.python.org/3/reference/expressions.html#comparisons), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#comparison-operators), [Go 1.27](https://go.dev/ref/spec#Comparison_operators))
- *Gerekçe:* sınır sınaması (`0 <= i < n`) kodlama modellerinin Python'dan getirdiği en sık zincirdir ve ilk denemede doğru çıkar. C ailesi aynı zinciri `(a < b) < c` diye okur; TR'de bu bir `TR_Bool` ile sayının karşılaştırılmasıdır ve denetim hatası verir (4.5), bu yüzden geriye Python'un tek anlamı kalır, sessiz bir yanlış çıkmaz (DENETİM, 2026-10-27). `x == y == TR_true` ise Python'da ve C'de iki ayrı anlama gelir, ikisi de tip denetiminden geçer; bu yüzden hatadır. Sözcükleri sıralamak için metin karşılaştırması gerekir.

### 4.5

- *Emsal:* Go'da fonksiyon tipleri karşılaştırılamaz; Go, Rust ve Swift'te ayrı tipler `==` ile karşılaştırılamaz, derleme hatasıdır. *Almadık:* Python'un ayrı tipleri sessizce eşit saymaması (`"1" == 1`, `False` döner, hata vermez); fonksiyonları kimlikle karşılaştırmak (görünüşü aynı iki fonksiyon eşit sayılmaz, kafa karıştırır). ([Go 1.27](https://go.dev/ref/spec#Comparison_operators), [Python 3.15](https://docs.python.org/3/reference/expressions.html#value-comparisons))
- *Gerekçe:* her zaman yanlış çıkan bir karşılaştırma bir yanlışı saklar; çalışmadan önce hata olur. Tam sayı ile ondalığın kesin karşılaştırılması büyük adım ve bayt sayılarında yanlış eşitlik vermez. Sayıların tipten bağımsız eşitliği öğelere uzanmaz, çünkü öğe tipi ayrı iki liste 3.6'ya göre ayrı tiptir. Aralığın eşitliği uçlarından gelir, Rust'ın `Range`'indeki gibi (`PartialEq` uçlardan); Python'da `range(0) == range(1, 1)` doğrudur [ölçüldü: Python 3.12.6], TR'de iki aralık aynı uçları taşıdığında eşittir, kural tek cümle kalır.
- *Emsal (fonksiyon içeren kap):* Go'da fonksiyon tipi karşılaştırılamaz, yalnız `nil`'le; Rust Reference'a göre kapanış tipleri `Sized`'dır ve yakaladıkları izin verirse `Clone`, `Copy`, `Sync`, `Send` uygular; `PartialEq` bu listede yoktur. Kabın eşitliği öğelerinin eşitliğidir; öğe karşılaştırılamıyorsa kap da karşılaştırılamaz. ([Go 1.27 karşılaştırma](https://go.dev/ref/spec#Comparison_operators), [Rust Reference: closure types](https://doc.rust-lang.org/reference/types/closure.html))
- *Gerekçe:* `[f] == [f]`'in kimlikle doğru vermesi, `f == f`'nin hata vermesiyle çelişirdi.

### 4.6

- *Emsal:* Python mantık işlecini sözcükle yazar (`and`, `or`, `not`); Go ve Rust işlenenin mantık tipinde olmasını ister. *Almadık:* Python'un işlenenin kendisini döndürmesi (`1 and "a"` → `"a"`). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#boolean-operations), [Go 1.27](https://go.dev/ref/spec#Logical_operators))
- *Gerekçe:* mantık işleminin sonucu her zaman mantık değeridir; 3.10'un koşul kuralıyla aynı.

### 4.7

- *Emsal:* Swift'te `??`'nin sol yanı her zaman isteğe bağlıdır, sağ yanı içteki tiple aynıdır; Kotlin'de `?:`. *Almadık:* Swift'in ve Kotlin'in isteğe bağlı olmayan sol yanda yalnız uyarı vermesi (DENETİM, 2026-10-27): kodlama modeli uyarıyı geçebilir, ve 3.5 aynı türden durumu hata sayar. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/basicoperators/#Nil-Coalescing-Operator), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html#elvis-operator))
- *Gerekçe:* varsayılan değer isteğe bağlı değeri açmanın kısa yoludur (3.5); hiç kullanılmayacak bir sağ taraf bir yanlışı gösterir ve çalışmadan önce görünür. Python'un `d.get(k, 0) + 1` alışkanlığı `d.TR_get(k) ?? 0 + 1` diye yazılır ve 4.1'e göre `?? (0 + 1)` okunur: tip denetiminden geçer, sayaç sessizce 1'de kalır (Gemini, 2026-10-27). Swift, Kotlin ve C# da aynı biçimde ayrıştırır [kanıtlı: DENETİM, Policy.swift ve Kotlin dilbilgisi]; önceliği değiştirmek onların alışkanlığını bozardı. Bu yüzden öncelik kalır, parantezsiz karışım hata verir.

### 4.8

- *Emsal:* Swift'in ve Kotlin'in isteğe bağlı zinciri sonucu isteğe bağlı verir; Swift: "If the type you are trying to retrieve is already optional, it will not become more optional because of the chaining." ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/), [Kotlin 2.4](https://kotlinlang.org/docs/null-safety.html#safe-call-operator))
- *Gerekçe:* yok olabilecek bir alanın içine güvenle inilir; yokluk tipte görünmeye devam eder.

### 4.9

- *Emsal:* Python'da `&&`, `||` ve `!` yoktur, sözcükleri vardır; F# ve Elixir `|>` ile çağırır. *Almadık:* C ailesinin sembolleri ve `|>`: aynı işin ikinci yazımı olurdu (2.9); C ailesinin `//` ve `/* */` yorumu (2.2). ([Python 3.15](https://docs.python.org/3/reference/expressions.html#boolean-operations), [F#](https://learn.microsoft.com/en-us/dotnet/fsharp/language-reference/symbol-and-operator-reference/))
- *Gerekçe:* her işin tek yazımı vardır; kodlama modelleri başka dillerin işlecini ve yorumunu alışkanlıkla yazar. Bu yazımların hepsi sessiz bir yanlış değil, sözdizimi hatası olur ve ileti doğrusunu söyler; düzeltme ilk denemede tutar.
  - `x = f(a) // ölçek` satırında `//`'nin sağı bir ifadedir; sözdizimi hatası yerine tanımsız ad hatası çıkar, bu yüzden o ileti de `#`'i önerir. Sağdaki ad tanımlı bir sayıysa (`bs = 32 // batch`) yanlış bir değer sessizce çıkar [hipotez: sıklığı ölçülmedi]. TR'nin yüzeyi Swift'e benzediği için model `//` yorumuna uzanabilir [hipotez, DENETİM]; iletilerin önemi bundandır.
  - Python'un `x in l`'si TR'de işleç olamaz: koşullu döngü `TR_for koşul` olduğundan `TR_for x TR_in l { }` hem dolaşma hem koşul diye okunurdu. Üyelik bu yüzden yalnız `.TR_contains` ile sorulur.
- *Gerekçe (ifadenin ardındaki `TR_in`):* kural `TR_in`'in yerini anlatır: döngünün ve üretecin başlığında değişken adından sonra; başka her yerde, döngü başlığına benzese de (`TR_if x.y TR_in l`), Python'un üyelik sınamasıdır.

### 4.10

- *Emsal:* Python, Go, Rust ve Swift metne başka bir tipi `+` ile eklemez (Python: `TypeError`). Python dizileri `*` ile yineler (`"-" * 3` → "---", `[0] * 3` → `[0, 0, 0]`, eksi sayı boş verir [ölçüldü: Python 3.12.6]). *Almadık:* Java'nın, JavaScript'in ve Kotlin'in metne her değeri kendiliğinden çevirip eklemesi; Python'da iç içe listenin yinelenmesi: "replicating a list with `*` doesn’t create copies, it only creates references to the existing objects" (`[[0] * 2] * 2`'de bir satırı değiştirmek ikisini de değiştirir [ölçüldü]). ([Python 3.15](https://docs.python.org/3/library/stdtypes.html#common-sequence-operations), [Python SSS](https://docs.python.org/3/faq/programming.html#how-do-i-create-a-multidimensional-list), [Go 1.27](https://go.dev/ref/spec#Arithmetic_operators))
- *Gerekçe:* metin kurmanın tek yazımı yerleştirmedir (2.7); kendiliğinden dönüşüm bir yanlış tipi saklar. `[0] * n` ve `"-" * 40` ML kodunda çok sık; C ailesinde zaten hatadır, bu yüzden Python'daki anlamı sessiz bir fark doğurmaz (DENETİM, 2026-10-27). Eksi sayının boş vermesi Python'un dolgu alışkanlığını korur (`" " * (genişlik - n)`). İç içe listenin yinelenmesi Python'un bilinen sessiz yanlışıdır; TR'de listeler de başvuruyla paylaşıldığı için (3.6) aynı yanlış doğardı, bu yüzden denetim hatasıdır.

### 4.11

- *Emsal:* Python, matris çarpımı için `@` işlecini PEP 465 ile ekledi; NumPy ve PyTorch onu kullanır. ([PEP 465](https://peps.python.org/pep-0465/))
- *Gerekçe:* modellerin çekirdek işlemi matris çarpımıdır; formüldeki gibi okunur.

### 4.12

- *Emsal:* Python, Go, Rust ve C aynı bit işleçlerini kullanır. Rust kaydırma miktarının türün bit sayısını aşmasını taşma sayar ("Using `<<` or `>>` where the right-hand argument is greater than or equal to the number of bits in the type of the left-hand argument, or is negative") ve işaretli sayıda "Arithmetic right shift" yapar. *Almadık:* Go'nun aşırı kaydırmayı sıfıra çevirmesi. ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/expressions/operator-expr.html#arithmetic-and-logical-binary-operators), [Python 3.15](https://docs.python.org/3/reference/expressions.html#binary-bitwise-operations), [Go 1.27](https://go.dev/ref/spec#Arithmetic_operators)) Java'nın `>>>`'ı: "The value of n >>> s is n right-shifted s bit positions with zero-extension". *Almadık:* ayrı bir `>>>` işleci: tek karakterle ayrılan iki kaydırma ajan için karışır. ([Java SE 21, 15.19](https://docs.oracle.com/javase/specs/jls/se21/html/jls-15.html#jls-15.19))
- *Gerekçe:* robotun yazmaçları, bit maskeleri, 4 bitlik nicemlenmiş ağırlıkların paketlenmesi ve özetleme işlevleri bit işlemi ister; 2.4'ün onaltılık ve ikilik yazımı bunlar içindir. xorshift ve FNV'nin karıştırması sıfırla dolduran kaydırma ister; açık ad hangisinin istendiğini söyler.
