# 2. Sözcük yapısı

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "2. Sözcük yapısı".

Bu bölüm kodlu biçimi, yani kimliklerle yazılmış programı anlatır. Kodun bir insan dilinde gösterilmesi ve o dilde yazılması (gösterim dilleri) ayrı bir konudur, ayrıca tasarlanacak ([yol haritası](../roadmap.md)).

**2.1** Diskteki her `.tr` dosyası UTF-8 metindir ve kodludur: ilk satırı kodlu biçimi ve sürümünü bildiren başlıktır, dilin sözcükleri kimlikleriyle (`TR_fn_print`) yazılır. Başlıktan sonra aktarmalar gelir (1.3), sonra öbür tanım ve deyimler. Baştaki BOM (U+FEFF) yok sayılır; kaynağın ortasındaki NUL (U+0000) sözdizimi hatasıdır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir.
- *Emsal:* Go, Rust ve Python kaynağı UTF-8 okur; Go baştaki BOM'u yok sayabilir, Rust siler. *Almadık:* Python'un dosya başında başka bir kodlama bildirilmesine izin vermesi. ([Go 1.27](https://go.dev/ref/spec#Source_code_representation), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/input-format.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#encoding-declarations))
- *Gerekçe:* program her makinede, dizüstünde de robotta da, aynı baytlardan aynı anlamı çıkarır. Kimliklerle yazılan program hiçbir insan diline bağlı değildir.
- *Test:* TR testi.

**2.1a** Göründüğünden farklı okunan kaynak kabul edilmez. Yazı yönünü değiştiren görünmez karakterler (U+202A–U+202E, U+2066–U+2069, U+200E, U+200F, U+061C) kaynağın hiçbir yerinde, yorum ve metin içinde de, yazılamaz; metinde gerekiyorsa kaçışla yazılır (2.6). Adlarda görünmez dolgu harfleri (U+115F, U+1160, U+3164, U+FFA0) kullanılamaz. Bir ad birbirine benzeyen yazı sistemlerinden (Latin, Kiril, Yunan) yalnız birinin harfleriyle yazılır: son harfi Kiril olan `yetkі` hatadır, çünkü Latin harfli yazımından ayırt edilemez; tamamı tek yazı sisteminde olan adlar (`öğrenme`, `привет`, `αβγ`) geçerlidir.
- *Emsal:* Rust yazı yönünü değiştiren karakterleri yorumda ve metinde varsayılan olarak hata sayar (CVE-2021-42574); karışık yazı sistemli ve birbirine benzeyen adları Unicode'un güvenlik standardına (UTS #39) göre uyarır. UTS #39 tek yazı sistemli adı "single-script" diye tanımlar. *Almadık:* Go ve Python'un bu karakterlere bir denetimi yok; Rust'ın karışık yazı sistemini yalnız uyarması (TR'de hata). ([Rust lints, hata](https://doc.rust-lang.org/rustc/lints/listing/deny-by-default.html), [Rust lints, uyarı](https://doc.rust-lang.org/rustc/lints/listing/warn-by-default.html), [UTS #39 sürüm 18](https://www.unicode.org/reports/tr39/))
- *Gerekçe:* kod paylaşılır: kitaplık, model kodu, robotun programı. Okunan ile çalışan aynı olmalı; bir ayrıntı yüzünden başka bir şey yapan kod gözden kaçmamalı.
- *Test:* TR testi.

**2.2** Yorumlar iki biçimdedir: satır sonuna kadar `//` ve çok satırlı `/* ... */`. Çok satırlı yorumlar iç içe yazılabilir; kapanmayan `/*` hatadır. `#` yorum değildir: metin dışında yazılan `#` sözdizimi hatasıdır ve ileti `//`'yi önerir; metin içinde (`"#22c55e"`) sıradan bir karakterdir.
- *Emsal:* Go, Rust, Swift ve C# yorumu `//` ve `/* */` ile yazar; Rust ve Swift çok satırlı yorumu iç içe kabul eder. *Almadık:* Python'un `#`'i: yorumun tek yazımı `//`'dir; Go'nun iç içe olmayan yorumu: içinde yorum bulunan bir kod parçası yoruma alınamaz. ([Go 1.27](https://go.dev/ref/spec#Comments), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/comments.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#comments))
- *Gerekçe:* bir kod parçası, içindeki yorumlarla birlikte geçici olarak devre dışı bırakılabilir (`/* dış /* iç */ dış */`); Python alışkanlığıyla yazılan `#` sessizce başka bir anlama kaymaz.
- *Test:* TR testi; iç içe yorum uygulanınca TR testi.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var x = 1  // satır sonuna kadar yorum
/* çok
   satırlı */
```

**2.3** Ad bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; bütün Unicode harfleri geçerlidir: `öğrenci_sayısı2`. Adlar Unicode'un NFC biçimine göre karşılaştırılır: aynı görünen iki yazım (tek karakterli `ş` ya da `s` ile birleşen çengel) aynı addır.
- *Emsal:* Rust adı XID_Start ya da `_` ile başlatır ve NFC'ye göre karşılaştırır; Python NFKC'ye çevirir. *Almadık:* Go'nun adı hiç normalleştirmemesi (aynı görünen iki ad farklı olur); Python'un NFKC'si (uyumluluk biçimlerini de birleştirir, üst simge `²` ile `2` aynı ad olur). ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/identifiers.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#identifiers), [Go 1.27](https://go.dev/ref/spec#Identifiers))
- *Gerekçe:* aynı harf farklı klavyelerden ya da kopyalanan metinden birleşik ya da ayrık gelebilir; aynı görünen ad aynı ad olmalı.
- *Test:* TR testi; NFC karşılaştırması uygulanınca TR testi.

**2.3a** `TR_` ile başlayan adlar kimliklere ayrılmıştır: kodlu dosyada `TR_` ile başlayan her ad dilin bir sözcüğüdür, dilin kimlik listesinde olmayan `TR_` adı hatadır. Kullanıcının adı `TR_` ile başlayamaz; `tr_x` ve `TRx` sıradan addır.
- *Emsal:* Python iki alt çizgiyle başlayıp biten adları (`__init__`) sisteme ayırır; C ve C++ alt çizgi ve büyük harfle başlayan adları gerçekleştirmeye ayırır. *Almadık:* Python'da bu ayrımın yalnız gelenek olması: TR'de ayrılmış ad hatadır. ([Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#reserved-classes-of-identifiers))
- *Gerekçe:* kimlik ile kullanıcının adı hiçbir zaman karışmaz; dile yeni bir kimlik eklemek hiçbir programın adını bozmaz.
- *Test:* TR testi.

**2.3b** Ad yazılışı: sabitler `BÜYÜK_YILAN` (`TR_kw_const EN_BÜYÜK = 10`), yapılar `BüyükDeve` (kısaltma bütünüyle büyük kalır: `TR_type_GPT`), değişken, fonksiyon, parametre, alan ve metot adları `küçük_yılan` (`öğrenci_sayısı`) yazılır. Matematik adları muaftır: en çok üç harf ve rakamdan oluşan parça büyük harfli olabilir (`W`, `Wq`, `dW`, `W1`, `QKV`, `X_veri`). Baştaki `_` yazılışa sayılmaz. Kurala uymayan ad hata değil uyarıdır, program çalışır; uyarı önerilen yazımı verir ve büyük harfle yazılmış, değeri değişmeyen bir değişkene `TR_kw_const` önerir. Depodaki kitaplık, örnek, test ve araç programları uyarısızdır.
- *Emsal:* Python (PEP 8) aynı düzeni gelenek olarak koyar; Rust aynı düzeni varsayılan uyarılarla denetler (`non_snake_case`, `non_camel_case_types`, `non_upper_case_globals`); Kotlin sabitlerde aynıdır, değişkende `küçükDeve` yazar. *Almadık:* Go'da büyük harfin görünürlüğü belirlemesi (yazılışa anlam yüklenmez). ([PEP 8](https://peps.python.org/pep-0008/#naming-conventions), [Rust lints, uyarı](https://doc.rust-lang.org/rustc/lints/listing/warn-by-default.html), [Kotlin 2.4](https://kotlinlang.org/docs/coding-conventions.html#naming-rules), [Go 1.27](https://go.dev/ref/spec#Exported_identifiers))
- *Gerekçe:* adın yazılışı ne olduğunu söyler; `TR_kw_const` bir anahtar kelime olduğundan yazılış ile tanım birbirini doğrular. Uyarı, çünkü program yine doğrudur. Öneri `'öğrenciSayısı' yerine 'öğrenci_sayısı'` biçimindedir.
- *Test:* TR testi.

**2.4** Tam sayı ondalık rakamlarla ya da ön ekle başka tabanda yazılır: onaltılık `0x1F`, sekizlik `0o17`, ikilik `0b1010`. Okunabilirlik için rakamların arasına ve ön ekten sonra `_` yazılabilir (`1_000_000`, `0xFF_FF`); `_` değeri değiştirmez, sayının sonunda ya da ön ekin içinde yazılamaz.
- *Emsal:* Go, Rust, Python ve Swift aynı üç ön eki ve `_` ayırıcıyı kabul eder. ([Go 1.27](https://go.dev/ref/spec#Integer_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#integer-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#integer-literals), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/))
- *Gerekçe:* robotun donanımındaki yazmaçlar ve bit maskeleri, paketlenmiş nicemlenmiş ağırlıklar ve dosya biçimlerinin sihirli baytları onaltılık ya da ikilik yazılır.
- *Test:* TR testi; ön ekler uygulanınca TR testi.

**2.5** Ondalık sayı nokta içerir ve noktanın iki yanında rakam bulunur: `3.14` (`.5` ve `1.` yazılmaz; `1..5` bir aralıktır, ondalık değil). Bilimsel yazım da ondalık verir: üs harfi `e` ya da `E`, isteğe bağlı `+`/`-` ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); üs harfinden sonra rakam yoksa harf sayıya ait değildir, ayrı bir addır. Rakamların arasına `_` yazılabilir (`1_000.5`).
- *Emsal:* Rust noktadan önce rakam ister; Go, Rust ve Python ondalıkta da `_` ayırıcıyı kabul eder. *Almadık:* Go ve Python'un `.5`'i ve `1.`'i: nokta üye erişimi ve aralıkla karışmasın. ([Go 1.27](https://go.dev/ref/spec#Floating-point_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#floating-point-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#floating-point-literals))
- *Gerekçe:* her sayı tek bir biçimde okunur; `x.5` ya da `1..5` bir sayı sanılmaz.
- *Test:* TR testi.

**2.6** Metin çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır. Kaçış dizileri: `\n \t \r \" \' \\ \{ \}` ve Unicode karakteri `\u{...}` (bir ile altı onaltılık rakam, geçerli bir Unicode karakteri: `"\u{202E}"`). Bilinmeyen kaçış dizisi sözdizimi hatasıdır, ters bölü sessizce atılmaz; ileti ham metni (2.6a) ya da `\\`'yi önerir. Metin birden çok satıra yayılabilir; metnin içindeki satır sonu kaynağın satır sonu biçiminden bağımsız olarak `\n`dir (CRLF'li dosyada değere `\r` girmez). `TR_cmd_format` bütün satır sonlarını LF yazar.
- *Emsal:* Rust ve Swift Unicode kaçışını `\u{...}` ile, Go ve Python `\uXXXX` ile yazar; Go ve Rust bilinmeyen kaçışı hata sayar. *Almadık:* Go ve Python'un sabit uzunluklu `\uXXXX`'i (dört rakamı aşan karakter için ikinci bir biçim gerekir). ([Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#character-escapes), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Go 1.27](https://go.dev/ref/spec#Rune_literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#escape-sequences))
- *Gerekçe:* modeller her dilin metniyle çalışır; kaynakta yazılamayan karakter (yazı yönü karakteri, 2.1a) metne kaçışla girer. `"C:\veri"` gibi bir yol sessizce bozulmaz.
- *Test:* TR testi; `\u{...}` uygulanınca TR testi.

**2.6a** Ham metin ters tırnakla (`` `...` ``) yazılır: içinde kaçış dizisi ve `{ifade}` yerleştirmesi (2.7) yoktur, ters bölü, tırnaklar ve süslü parantezler olduğu gibi kalır. Düzenli ifadeler, Windows yolları ve yerleştirme istenmeyen `{` içeren metinler için önerilen yazımdır. Ham metin de `TR_type_String` değeridir, birden çok satıra yayılabilir (satır sonu `\n`dir); içine ters tırnak yazılamaz. Kapanmayan ham metin sözdizimi hatasıdır. `TR_cmd_format` ham metnin içeriğine dokunmaz, yalnız satır sonlarını LF yazar (2.6).
- *Emsal:* Go ham metni ters tırnakla yazar, içinde ters bölünün anlamı yoktur ve satır başı karakteri değerden atılır; Rust ve Python ham metni `r"..."` ön ekiyle yazar. ([Go 1.27](https://go.dev/ref/spec#String_literals), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/tokens.html#raw-string-literals), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#string-and-bytes-literals))
- *Gerekçe:* düzenli ifade, Windows yolu ve JSON metni kaçışsız, okunduğu gibi yazılır.
- *Test:* TR testi.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var yol = `C:\veri\tokenizer.json`
TR_kw_var tarih = `\d{4}-\d{2}-\d{2}`     // "\\d\{4\}-\\d\{2\}-\\d\{2\}" ile aynı
```

**2.7** Tırnaklı metnin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır ya da ham metin kullanılır (2.6a). Kaçışsız her `{` bir yerleştirme başlatır: içi tek bir ifade olarak ayrıştırılamazsa ya da boşsa sözdizimi hatasıdır, düz metne dönülmez. Yerleştirmenin içindeki tırnaklı metinde süslü parantez yerleştirmeyi açıp kapatmaz (`"x {"a}b"} y"` → `x a}b y`). Yerleştirmedeki ifadenin sözcükleri dosyadaki gerçek satır ve sütunlarıyla bildirilir.
- *Emsal:* Swift ve Kotlin her metinde yerleştirme yapar (`\(x)`, `${x}`); Python yalnız `f` ön ekli metinde (`f"{x}"`), Rust yalnız biçim makrolarında (`format!("{x}")`). *Almadık:* Go'nun yerleştirmesizliği; Python'un ön eke bağlaması (ön eki unutulan metin `{x}`'i olduğu gibi yazar). ([Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/), [Kotlin 2.4](https://kotlinlang.org/docs/strings.html#string-templates), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#f-strings), [Rust 1.99 std::fmt](https://doc.rust-lang.org/std/fmt/))
- *Gerekçe:* günlük ve ileti satırları değer yerleştirerek yazılır; bir parçanın kod mu metin mi olduğu dilin sözcüklerine bağlı değildir, JSON gibi süslü parantezli metin ham metinle yazılır.
- *Test:* TR testi.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var ad = "Ali"
TR_kw_var p = {"ad": "Ayşe"}
TR_fn_print("Merhaba {ad}, 1 + 2 = {1 + 2}, kişi: {p['ad']}, düz: \{x\}")
```

**2.8** Deyimler satır sonuyla ya da isteğe bağlı `;` ile ayrılır. Satır sonu ifadeyi bitirir, parantez içinde (`(`, `[` ve sözlük `{`) bitirmez. Yeni satırda başlayan `(` ya da `[` önceki ifadeyi sürdürmez, yeni bir deyim başlatır; sondaki blok `{` önceki ifadeyle aynı satırda olmalıdır. Yeni satırın başındaki ikili işleç de önceki ifadeyi sürdürmez: `-x` satırı ayrı bir deyimdir ve değeri kullanılmadığı için uyarı verir; tekli biçimi olmayan işleç (`+`, `*`, `TR_kw_and`, `??` ...) satır başında sözdizimi hatasıdır ve nedenini söyler. İfadeyi bölmek için işleç satır sonuna yazılır (`1 +` ↵ `2`) ya da ifade parantez içine alınır. Zincir işleçleri satır başında da sürer: üye erişimi `.` ve `?.`, boru `|>`.
- *Emsal:* Go satırın son sözcüğü bir ad, değer, `)`, `]` ya da `}` ise noktalı virgülü kendisi koyar; Python, Kotlin ve Swift'te de satır sonu deyimi bitirir. *Almadık:* JavaScript'in satır başındaki `-`'yi önceki satıra bağlaması, bilinen bir tuzak. ([Go 1.27](https://go.dev/ref/spec#Semicolons), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#line-structure), [Kotlin 2.4](https://kotlinlang.org/docs/basic-syntax.html), [Swift 6.4](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/lexicalstructure/))
- *Gerekçe:* satırın nerede bittiği gözle görülür; bir satırın başındaki işaret önceki satırı sessizce değiştirmez.
- *Test:* TR testi.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var f = 5
(1 + 2) |> TR_fn_print        // ayrı deyim; '5(1 + 2)' çağrısı değil
TR_kw_var t = 1 +
    2                    // t = 3
```

**2.9** Anahtar kelimeler dilin ayrılmış sözcükleridir ve kimlikleriyle yazılır; ad olarak kullanılamazlar:

| Görev | Anahtar kelimeler |
| :-- | :-- |
| Bildirim | `TR_kw_var`, `TR_kw_const` |
| Fonksiyon | `TR_kw_func`, `TR_kw_return` |
| Koşul | `TR_kw_if`, `TR_kw_else`, `TR_kw_match`, `TR_kw_default` |
| Döngü | `TR_kw_for`, `TR_kw_in`, `TR_kw_while`, `TR_kw_break`, `TR_kw_continue` |
| Değer ve mantık | `TR_kw_true`, `TR_kw_false`, `TR_kw_none`, `TR_kw_and`, `TR_kw_or`, `TR_kw_not` |
| Yapı | `TR_kw_struct`, `TR_kw_self` |
| Hata yönetimi | `TR_kw_try`, `TR_kw_catch`, `TR_kw_throw` |
| Modül ve eşzamanlılık | `TR_kw_import`, `TR_kw_async`, `TR_kw_defer` |
| Hata ayıklama | `TR_kw_breakpoint` |

- *Emsal:* Go, Rust ve Python'un anahtar kelimeleri ayrılmıştır, ad olamaz. ([Go 1.27](https://go.dev/ref/spec#Keywords), [Rust 1.99 Reference](https://doc.rust-lang.org/reference/keywords.html), [Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#keywords))
- *Gerekçe:* anahtar kelimeler kimlikle yazıldığı için hiçbir kullanıcı adıyla karışmaz; dile yeni bir anahtar kelime eklemek mevcut adları bozmaz.
- *Test:* TR testi.

**2.9b** Bağlamsal sözcükler (`TR_ctx_...`) anahtar kelime değildir, yalnız kendi bağlamlarında yazılır: `TR_ctx_test` (deyim başında ardından metin gelince), `TR_ctx_as` (`TR_kw_import` içinde), `TR_ctx_init` (yapının kurucu metodu), `TR_ctx_error` (adsız `TR_kw_catch`'te hata değişkeni), `TR_ctx_it` (sondaki bloğun parametresi), `TR_ctx_acc` (`TR_mem_reduce`'nin sondaki bloğunda önceki sonuç), `_` (`TR_kw_match` kolunda varsayılan).
- *Emsal:* Python'un yumuşak anahtar kelimeleri (`match`, `case`, `_`) ve C#'ın bağlamsal anahtar kelimeleri yalnız bağlamlarında sözcüktür. ([Python 3.15](https://docs.python.org/3/reference/lexical_analysis.html#soft-keywords), [C# 14](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/keywords/#contextual-keywords))
- *Gerekçe:* bağlamsal sözcük bir kavramı yalnız yerinde adlandırır; dilin ayrılmış sözcüklerini çoğaltmaz.
- *Test:* gerekmez: her bağlamsal sözcük kendi bağlamının kuralıyla sınanır.

**2.10** İfadeler ve bloklar en çok 1000 düzey iç içe yazılabilir (`((((1))))` dört düzeydir); sınır aşılırsa program çalışmaz, sözdizimi hatası verilir. Metin içindeki yerleştirmenin ifadesi (2.7) bulunduğu yerin düzeyinden sayılır.
- *Emsal:* Python'un sözcük çözümleyicisi en çok 200 iç içe parantez kabul eder. ([CPython 3.15 lexer](https://github.com/python/cpython/blob/3.15/Parser/lexer/state.h))
- *Gerekçe:* sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi; güvenilmeyen bir kaynak dosya da `tr`'yi düşürememeli.
- *Test:* TR testi.

**2.12** İleti kodu (`H6903`: `H` ve tam dört rakam) bir ad değil sözcük birimidir ve ifadede kodun metnidir: `TR_fn_message(H6903, -1)` ile `TR_fn_message("H6903", -1)` aynıdır. Daha uzun, kısa ya da küçük harfli yazım (`H69030`, `h6903`) sıradan bir addır; tam `H` ve dört rakamlık sözcük ad olamaz. İleti kodu dilden bağımsızdır, olduğu gibi yazılır. Yazım yanlışı (`H690`) tanımsız ad hatasıdır; var olmayan kod `TR_fn_message` kurulurken bildirilir.
- *Emsal:* yok. Rust'ın hata kodları (`E0308`) belgede ve iletide geçer, dilin sözcüğü değildir. *Almadık:* iletiyi metniyle yakalamak (metin dile göre değişir).
- *Gerekçe:* program bir hatayı dilden bağımsız koduyla tanır ve kendi iletisini kurar.
- *Test:* TR testi.
