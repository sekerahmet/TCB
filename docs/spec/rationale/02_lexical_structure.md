# 2. Sözcük yapısı: emsal ve gerekçe

Kurallar: [`02_sozcuk_yapisi.md`](../02_sozcuk_yapisi.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 2.1

- *Emsal:* Go, Rust ve Python kaynağı UTF-8 okur; Go baştaki BOM'u yok sayabilir, Rust siler. Rust ilk satırdaki `#!`'i siler, Python'da o satır bir yorumdur. Python ilk ya da ikinci satırdaki özel bir yorumu kodlama bildirimi sayar ("If a comment in the first or second line of the Python script matches the regular expression"); TR'nin başlığı da ilk satırdaki (ya da `#!`'den sonraki) bir yorumdur. *Almadık:* Python'un dosya başında başka bir kodlama bildirilmesine izin vermesi. ([Go 1.27](https://go.dev/ref/spec#Source_code_representation), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/input-format.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#encoding-declarations))
- *Gerekçe:* program her makinede, dizüstünde de robotta da, aynı baytlardan aynı anlamı çıkarır. Kimliklerle yazılan program hiçbir insan diline bağlı değildir: dosyanın dili olmadığı için programın anlamı ve sonucu onu yazanın ya da okuyanın diline bağlı olamaz. Başlıktaki sürüm, dilin kodlu dosyayı kıran bir değişikliğinden sonra eski bir dosyanın sessizce başka anlamda okunmasını önler. Robotta ve hizmette program dosyası betik gibi doğrudan çalıştırılabilir (`./program.tr`).

### 2.1a

- *Emsal:* Rust yazı yönünü değiştiren karakterleri yorumda ve metinde varsayılan olarak hata sayar (CVE-2021-42574). İskeleti aynı iki adı uyarır (`confusable_idents`); bir yazı sisteminin bütün pakette yalnız karıştırılabilen harflerle kullanılmasını da uyarır (`mixed_script_confusables`). UTS #39 iskeleti, görünüşü aynı iki dizgiyi bulmak için tanımlar. *Almadık:* Go ve Python'un bu karakterlere bir denetimi yok; Rust'ın iskelet çakışmasını yalnız uyarması (TR'de hata); UTS #39'un orta düzeyi ("Moderately Restrictive") ve tek addaki karışık yazı sistemini hata saymak: makine öğrenmesinin `θ` ve `α` gibi harfleri de benzerlik çizelgesindedir. ([Rust lints, hata](https://doc.rust-lang.org/rustc/lints/listing/deny-by-default.html), [Rust lints, uyarı](https://doc.rust-lang.org/rustc/lints/listing/warn-by-default.html), [UTS #39 sürüm 18](https://www.unicode.org/reports/tr39/))
- *Gerekçe:* kod paylaşılır: kitaplık, model kodu, robotun programı. Okunan ile çalışan aynı olmalı; bir ayrıntı yüzünden başka bir şey yapan kod gözden kaçmamalı. Görünüşü aynı iki ad okuyanın birini öbürü sanmasına yol açar; tek bir addaki yabancı harf ise başka bir adla karışmadıkça bir şey gizlemez. Makine öğrenmesinde Yunan harfleri formüldeki gibi adda kullanılır (`θ_eski`, `α_oran`).

### 2.2

- *Emsal:* Python, Ruby, kabuk, YAML ve TOML yorumu `#` ile yazar; Python'da çok satırlı yorum yoktur, PEP 8'e göre "Each line of a block comment starts with a # and a single space". *Almadık:* C ailesinin `//` ve `/* */` yorumu: TR'de `//` Python'daki gibi tam bölmedir (4.2a), iki yorum biçimi de aynı işin ikinci yazımı olurdu; Rust'ın ve Swift'in iç içe blok yorumu. ([Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#comments), [PEP 8](https://peps.python.org/pep-0008/#block-comments), [Go 1.27](https://go.dev/ref/spec#Comments), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/comments.html))
- *Gerekçe:* kodu çoğunlukla kodlama modelleri yazar; yapay zekâ işinde en çok yazdıkları dil Python'dur. Yorum ve tam bölme Python'daki gibi olunca alışkanlık doğru yazar: `a // 2` tam bölmedir, `# not` yorumdur. C alışkanlığıyla yazılan `// not` ya da `/* */` sessizce başka bir anlama kaymaz, sözdizimi hatası verir ve ileti `#`'i önerir. Bir kod parçasını yoruma almak her satırının başına `#` koymaktır; biçimlendirici ve düzenleyici bunu tek adımda yapar.

### 2.3

- *Emsal:* Rust adı XID_Start ya da `_` ile başlatır ve NFC'ye göre karşılaştırır; Python NFKC'ye çevirir. *Almadık:* Go'nun yalnız harf ve rakama izin vermesi (birleşen işaretle yazılan yazı sistemlerinde ad yazılamaz) ve adı hiç normalleştirmemesi (aynı görünen iki ad farklı olur); Python'un NFKC'si (uyumluluk biçimlerini de birleştirir, üst simge `²` ile `2` aynı ad olur). ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/identifiers.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#identifiers), [Go 1.27](https://go.dev/ref/spec#Identifiers))
- *Gerekçe:* aynı harf farklı klavyelerden ya da kopyalanan metinden birleşik ya da ayrık gelebilir; aynı görünen ad aynı ad olmalı.

### 2.3a

- *Emsal:* Python, Go ve Rust'ta da sözcüğün türünü yeri söyler: `x.sum()` üye, `sum(x)` fonksiyondur. Kodun modele gidiş biçimi üzerine ISSTA 2024'te Sun ve arkadaşları: "reducing the tokens in code representation has the potential to reduce the time and energy cost proportionally". Python iki alt çizgiyle başlayıp biten adları (`__init__`) sisteme ayırır; Python 3.10'dan beri bulunamayan ad ve öznitelik iletisi benzer adı önerir (`Did you mean: namedtuple?`); Rust'ın derleyicisi bulamadığı ada yazımca en yakın adı önerir: uzaklık sınırı adın üçte biridir, iki komşu harfin yer değiştirmesi tek değişikliktir ("restricted Damerau-Levenshtein") ve kaynağı önekli adlarda önekin uzaklığı bozduğunu yazar ("would have a larger edit distance because it does not contain the clippy tool prefix"). *Almadık:* sınıflı yazım (`TR_kw_return`): GPT-2 ve SmolLM2 parçalayıcılarında beş parça, `TR_return` üç, `return` bir (o200k'de üç, iki ve bir); `@` işareti (`@return`, iki parça): Python'da süsleyici, Swift'te nitelik, C#'ta anahtar kelimeyi ad yapan işarettir; öneksiz sözcük: kullanıcının adıyla karışır, dile eklenen her yeni sözcük bir programın adını bozabilir; Python'da ayrılmış adın yalnız gelenek olması. ([Sun ve ark., ISSTA 2024](https://arxiv.org/abs/2404.16333), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#reserved-classes-of-identifiers), [Python 3.10 yenilikleri](https://docs.python.org/3/whatsnew/3.10.html#nameerror), [Rust 1.99 derleyici kaynağı](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_span/src/edit_distance.rs), [Python 3.15 süsleyici](https://docs.python.org/3/reference/compound_stmts.html#function-definitions), [Swift 6.4 nitelikler](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/attributes/), [C# 14](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/tokens/verbatim))
- *Gerekçe:* kodu çoğunlukla kodlama modelleri yazar ve okur; her sözcük modele parça parça gider. Sınıf parçası okuyana bir şey kazandırmaz, çünkü yer zaten söyler; kodlama modelinin en sık yanıldığı yer de sınıfı tahmin etmekti (üye mi fonksiyon mu). 48 örnek programın kod satırları sınıflı yazımda İngilizce gösterime göre yüzde 38–40, `TR_` ve adla yüzde 19–20 daha çok parça tutuyor (SmolLM2 ve GPT-2 parçalayıcılarıyla ölçüldü; DENETİM o200k ile %27'den %14'e indiğini ölçtü). İki kodlama modeliyle yapılan küçük bir denemede, kimlik listesi önlerindeyken iki yazımda da kimlik hatası çıkmadı; bu kanıt değil ipucudur. `TR_` öneki kalır: kimlik kullanıcının adıyla hiç karışmaz, dile yeni bir sözcük eklemek hiçbir programın adını bozmaz, uydurulan sözcük çalışmadan önce hata verir ve ileti doğrusunu söyler.

### 2.3b

- *Emsal:* Python (PEP 8) aynı düzeni gelenek olarak koyar; Rust aynı düzeni varsayılan uyarılarla denetler (`non_snake_case`, `non_camel_case_types`, `non_upper_case_globals`); Kotlin sabitlerde aynıdır, değişkende `küçükDeve` yazar. Büyük harf modül düzeyindeki sabit içindir: PEP 8 "Constants are usually defined on a module level and written in all capital letters with underscores separating words"; Kotlin'de büyük harf `const` ve en dıştaki ya da nesnedeki `val` içindir, yerel değişken küçük harfle başlar. *Almadık:* Go'da büyük harfin görünürlüğü belirlemesi (yazılışa anlam yüklenmez). ([PEP 8](https://peps.python.org/pep-0008/#naming-conventions), [Rust lints, uyarı](https://doc.rust-lang.org/rustc/lints/listing/warn-by-default.html), [Kotlin 2.4](https://kotlinlang.org/docs/coding-conventions.html#naming-rules), [Go 1.27](https://go.dev/ref/spec#Exported_identifiers))
- *Gerekçe:* adın yazılışı ne olduğunu söyler; `TR_const` bir anahtar kelime olduğundan yazılış ile tanım birbirini doğrular. Uyarı, çünkü program yine doğrudur. Matematik muafiyeti, makine öğrenmesinde matris ve veri adlarının formüldeki gibi büyük harfle yazılmasındandır. Öneri `'öğrenciSayısı' yerine 'öğrenci_sayısı'` biçimindedir.

### 2.4

- *Emsal:* Go, Rust, Python ve Swift aynı üç ön eki ve `_` ayırıcıyı kabul eder; Rust ön eki yalnız küçük harfle tanır. Rust tipine sığmayan sayı yazımını varsayılan olarak hata sayar. *Almadık:* Python'un sınırsız tam sayısı; Go ve Python'un büyük harfli ön eki (`0X1F`, aynı sayının ikinci yazımı). ([Go 1.27](https://go.dev/ref/spec#Integer_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#integer-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#integer-literals), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Rust lints, hata](https://doc.rust-lang.org/rustc/lints/listing/deny-by-default.html))
- *Gerekçe:* robotun donanımındaki yazmaçlar ve bit maskeleri, paketlenmiş nicemlenmiş ağırlıklar ve dosya biçimlerinin sihirli baytları onaltılık ya da ikilik yazılır.

### 2.5

- *Emsal:* Rust noktadan önce rakam ister; Go, Rust ve Python ondalıkta da `_` ayırıcıyı kabul eder. *Almadık:* Go ve Python'un `.5`'i ve `1.`'i: nokta üye erişimi ve aralıkla karışmasın. ([Go 1.27](https://go.dev/ref/spec#Floating-point_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#floating-point-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#floating-point-literals))
- *Gerekçe:* her sayı tek bir biçimde okunur; `x.5` ya da `1..5` bir sayı sanılmaz.

### 2.6

- *Emsal:* Go, Rust, Swift, Kotlin ve C# metni yalnız çift tırnakla yazar. Rust ve Swift Unicode kaçışını `\u{...}` ile, Go ve Python `\uXXXX` ile yazar; Go ve Rust bilinmeyen kaçışı hata sayar. *Almadık:* Python'un iki tırnak biçimi (aynı metin iki biçimde yazılır); Go ve Python'un sabit uzunluklu `\uXXXX`'i (dört rakamı aşan karakter için ikinci bir biçim gerekir). ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#character-escapes), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Go 1.27](https://go.dev/ref/spec#Rune_literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#escape-sequences))
- *Gerekçe:* modeller her dilin metniyle çalışır; kaynakta yazılamayan karakter (yazı yönü karakteri, 2.1a) metne kaçışla girer. `"C:\veri"` gibi bir yol sessizce bozulmaz. Bir metnin tek yazımı vardır.

### 2.6a

- *Emsal:* Go ham metni ters tırnakla yazar, içinde ters bölünün anlamı yoktur ve satır başı karakteri değerden atılır; Rust ve Python ham metni `r"..."` ön ekiyle yazar. ([Go 1.27](https://go.dev/ref/spec#String_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#raw-string-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#string-and-bytes-literals))
- *Gerekçe:* düzenli ifade, Windows yolu ve JSON metni kaçışsız, okunduğu gibi yazılır.

### 2.7

- *Emsal:* Swift her metinde `\(x)` ile yerleştirir; Kotlin her metinde `${x}` ile, Python yalnız `f` ön ekli metinde `{x}` ile (`f"{x}"`), Rust yalnız biçim makrolarında `{x}` ile (`format!("{x}")`). *Almadık:* `{x}` (Python, Rust): metindeki JSON'un ve şablonun her süslü parantezi kaçışla yazılmak zorunda kalır; Kotlin'in `${x}`'i: aynı yazım kabuk komutlarında ve JavaScript şablonlarında da geçer; Python'un ön eke bağlaması (ön eki unutulan metin `{x}`'i olduğu gibi yazar); Go'nun yerleştirmesizliği. ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Kotlin 2.4](https://kotlinlang.org/docs/strings.html#string-templates), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#f-strings), [Rust 1.99 std::fmt](https://doc.rust-lang.org/std/fmt/))
- *Gerekçe:* günlük satırları ve modele giden istemler değer yerleştirerek yazılır; istemler çoğu zaman JSON, sohbet şablonu (`{{ }}`) ve kabuk komutu içerir. Ters bölü zaten kaçış işareti olduğu için `\(` metinde kendiliğinden geçmez; JSON'un, şablonun ve kabuğun işaretleri olduğu gibi kalır.

### 2.8

- *Emsal:* Go satırın son sözcüğü bir ad, değer, `)`, `]` ya da `}` ise noktalı virgülü kendisi koyar; Python, Kotlin ve Swift'te de satır sonu deyimi bitirir. *Almadık:* JavaScript'in satır başındaki `-`'yi önceki satıra bağlaması, bilinen bir tuzak. ([Go 1.27](https://go.dev/ref/spec#Semicolons), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#line-structure), [Kotlin 2.4](https://kotlinlang.org/docs/basic-syntax.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/))
- *Gerekçe:* satırın nerede bittiği gözle görülür; bir satırın başındaki işaret önceki satırı sessizce değiştirmez.

### 2.9

- *Emsal:* Go, Rust ve Python'un anahtar kelimeleri ayrılmıştır, ad olamaz. Go'nun tek döngü sözcüğü `for`'dur; Python'un ilkesine göre bir işin tek ve açık bir yolu olmalıdır. *Almadık:* Python ve Rust'ın `for` yanında ayrı `while`'ı. ([Go 1.27](https://go.dev/ref/spec#Keywords), [Go 1.27 for](https://go.dev/ref/spec#For_statements), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/keywords.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#keywords), [PEP 20](https://peps.python.org/pep-0020/))
- *Gerekçe:* anahtar kelimeler kimlikle yazıldığı için hiçbir kullanıcı adıyla karışmaz; dile yeni bir anahtar kelime eklemek mevcut adları bozmaz. Aynı iş iki sözcükle yazılınca okuyan ikisini de öğrenmek zorunda kalır. Kodlama modelleri başka dillerin while döngüsünü alışkanlıkla yazar; ileti doğrusunu söyleyince ilk düzeltme tutar.

### 2.9b

- *Emsal:* Python'un yumuşak anahtar kelimeleri (`match`, `case`, `_`) ve C#'ın bağlamsal anahtar kelimeleri yalnız bağlamlarında sözcüktür. ([Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#soft-keywords), [C# 14](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/keywords/#contextual-keywords))
- *Gerekçe:* bağlamsal sözcük bir kavramı yalnız yerinde adlandırır; dilin ayrılmış sözcüklerini çoğaltmaz.

### 2.10

- *Emsal:* Python'un sözcük çözümleyicisi en çok 200 iç içe parantez kabul eder. ([CPython 3.15 lexer](https://github.com/python/cpython/blob/3.15/Parser/lexer/state.h))
- *Gerekçe:* sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi; güvenilmeyen bir kaynak dosya da `tr`'yi düşürememeli.

### 2.13

- *Emsal:* Rust `--error-format=json` ile her iletiyi bir satırda bir JSON nesnesi olarak yazar: `message`, `code`, `level`, ilgili iletiler `children`, başlangıç ve bitiş konumlarıyla (`line_start`, `line_end`, `column_start`, `column_end`) `spans`, konumun içinde `suggested_replacement`; düzeltmenin `suggestion_applicability`'si "MachineApplicable" ise "This suggestion should be automatically applied". Go 1.24'ten beri `go build -json` derleme çıktısını ve hatalarını yapılandırılmış JSON olarak verir. *Almadık:* yalnız metin veren ileti: okuyan program onu metinden ayıklamak zorunda kalır. ([Rust 1.99 rustc kitabı](https://doc.rust-lang.org/rustc/json.html), [Go 1.24 sürüm notları](https://go.dev/doc/go1.24#go-command))
- *Gerekçe:* kodlama modelleri, düzenleyiciler ve sürekli tümleştirme hatayı metni ayrıştırmadan okur; ileti yeniden yazılsa ya da başka bir dilde gösterilse de kod ve konum aynı kalır. Bitiş konumu, düzeltmenin kaynağa tam yerine konmasını; uygulanabilirlik işareti, kesin düzeltmenin bir model turu harcamadan uygulanmasını sağlar. Ajan programı denetleyip çalıştırırken her türlü ön hatayı aynı biçimde alır.
