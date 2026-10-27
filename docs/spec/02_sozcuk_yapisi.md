# 2. Sözcük yapısı

**2.1** Kaynak UTF-8 metindir; bazı Windows editörlerinin eklediği baştaki BOM (U+FEFF) yok sayılır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir. Kaynağın ortasındaki NUL karakteri (U+0000) sözdizimi hatasıdır (metin dosyasında bulunmaz; ardından gelen kod sessizce yok sayılmaz). Dosya kodlu biçimde de saklanabilir (17.1).

**2.1a** Göründüğünden farklı okunan kaynak kabul edilmez. Yazı yönünü değiştiren görünmez karakterler (U+202A–U+202E, U+2066–U+2069, U+200E, U+200F, U+061C) kaynağın hiçbir yerinde, yorum ve metin içinde de, kullanılamaz: bir satırın gösterim sırasını değiştirip kodun başka bir şey gibi görünmesine yol açarlar. Adlarda görünmez dolgu harfleri (U+115F, U+1160, U+3164, U+FFA0) kullanılamaz. Bir ad birbirine benzeyen yazı sistemlerinden (Latin, Kiril, Yunan) yalnızca birinin harfleriyle yazılır: `yetkі` (son harfi Kiril) hatadır, çünkü `yetki`'den ayırt edilemez; tamamı tek yazı sisteminde olan adlar (`öğrenme`, `привет`, `αβγ`) geçerlidir.

**2.2** Yorumlar iki biçimdedir: satır sonuna kadar `//` ve çok satırlı `/* ... */`. Kapanmayan `/*` bir hatadır. `#` yorum değildir: metin dışında yazılan `#` bir sözdizimi hatasıdır (`'#' TR'de yorum değil; yorum için '//' yazın`); metin içinde (`"#22c55e"`) sıradan bir karakterdir.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var x = 1  // satır sonuna kadar yorum
/* çok
   satırlı */
```

**2.3** Tanımlayıcılar bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; Türkçe harfler (ç ğ ı İ ö ş ü) dahil tüm Unicode harfleri geçerlidir: `öğrenci_sayısı2`.

**2.3a** `TR_` ile başlayan adlar ayrılmıştır: dilin ve kitaplığının kimliklerinin yazımıdır (`TR_fn_print`, `TR_kw_if`; 17) ve kimlik kod listesinde olsun olmasın kullanıcı adı olarak kullanılamaz; Türkçe yazılmış dosyada sözdizimi hatası verir: `'TR_x' ayrılmış bir ad: 'TR_' ile başlayan adlar dilin ve kitaplığının kimlikleri içindir; başka bir ad seçin (ör. tr_x)`. Ad bilinen bir kimlikse (`TR_fn_print`: büyük olasılıkla başlıksız yapıştırılmış kodlu metin) iletiye okunan metnin dilindeki başlık eklenir: `Kodlu dosyalar ilk satırdaki '//! tr kodlu 3 dil=tr' başlığıyla başlar` (İngilizce okunan metinde `dil=en`). Kodlu dosyada (17.1) bu adlar dilin sözcükleridir; `tr_x`, `TRx` sıradan adlardır. Kodların eski numaralı biçimi (`G100`, `A001`) artık ayrılmış değildir, sıradan addır.

**2.3b** Ad yazılışı (PEP 8 düzeni): sabitler `BÜYÜK_YILAN` (`TR_kw_const EN_BÜYÜK = 10`), yapılar `BüyükDeve` (`TR_kw_struct TR_type_SpikeGuard`; kısaltma bütünüyle büyük kalır: `GPT`), değişken, fonksiyon, parametre, alan ve metot adları `küçük_yılan` (`öğrenci_sayısı`, `TR_fn_pretrained_model`) yazılır. Matematik adları muaftır: en çok üç harf ve rakamdan oluşan parça büyük harfli olabilir (`W`, `X`, `Wq`, `dW`, `W1`, `QKV`, `X_veri`); formüllerin yazımıdır. Baştaki `_` ve `$` (2.11) yazılışa sayılmaz. Kurala uymayan ad hata değil uyarıdır (Rust'ın `non_snake_case`, `non_camel_case_types`, `non_upper_case_globals` uyarıları gibi; program çalışır): uyarı önerilen yazımı verir (`'öğrenciSayısı' yerine 'öğrenci_sayısı'`); büyük harfle yazılmış bir değişkene, değeri değişmiyorsa `TR_kw_const` ile tanımlanması önerilir. Öneri adda Türkçeye özgü harf varsa ya da gösterim dili Türkçeyse Türkçe büyük/küçük harf kuralıyla (`ADIM` → `adım`) kurulur. Depodaki kitaplık, örnek, test ve araç programları uyarısızdır. Karşılaştırma: Python (PEP 8) aynı düzeni gelenek olarak koyar, Kotlin sabitlerde aynıdır ama değişkenlerde `küçükDeve` kullanır, Swift sabitleri ayırmaz; TR'de `TR_kw_const` bir anahtar kelime olduğundan yazılış ile tanım birbirini doğrular.

**2.4** Tam sayı sabitleri ondalık rakamlardan oluşur; okunabilirlik için `_` kullanılabilir: `1_000_000`.

**2.5** Ondalık sabitler nokta içerir ve noktadan sonra rakam gelmelidir: `3.14`. (`1..5` bir aralıktır, ondalık değil.) Bilimsel yazım da Ondalık verir: üs harfi `e`/`E`, isteğe bağlı `+`/`-` ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); üsten sonra rakam yoksa `e` sayıya ait değildir (`2e` → `2` ve `e` adı).

**2.6** Metinler çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır; `\n \t \r \" \' \\ \{ \}` kaçış dizileri tanınır. Bilinmeyen bir kaçış dizisi sözdizimi hatasıdır, ters bölü sessizce atılmaz (`"\d+"`, `"C:\veri"` → `bilinmeyen kaçış dizisi '\d' ...; ters bölüyü olduğu gibi yazmak için ham metin kullanın ya da ters bölüyü ikileyin`): düzenli ifadeler ve Windows yolları için ham metin (2.6a) ya da `\\` yazılır. Metin birden çok satıra yayılabilir; metnin içindeki satır sonu kaynağın satır sonu biçiminden bağımsız olarak `\n`dir (CRLF'li bir dosyada metnin değerine `\r` girmez; `\r` yalnızca kaçışla yazılır). `TR_cmd_format` metinlerin içindekiler dahil bütün satır sonlarını LF yazar.

**2.6a** Ham metin ters tırnakla (`` `...` ``) yazılır: içinde kaçış dizisi ve `{ifade}` yerleştirmesi (2.7) yoktur, ters bölü, tırnaklar ve süslü parantezler olduğu gibi kalır. Düzenli ifadeler, Windows yolları ve yerleştirme istenmeyen `{` içeren metinler için önerilen yazımdır. Ham metin de `TR_type_String` değeridir ve birden çok satıra yayılabilir (satır sonu yine `\n`dir); içine ters tırnak yazılamaz (o zaman tırnaklı metin kullanılır). Kapanmayan ham metin sözdizimi hatasıdır. `TR_cmd_format` ham metni olduğu gibi korur.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var yol = `C:\veri\tokenizer.json`
TR_kw_var tarih = `\d{4}-\d{2}-\d{2}`     // "\\d\{4\}-\\d\{2\}-\\d\{2\}" ile aynı
```

**2.7** Metin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır (ya da ham metin, 2.6a). Tırnaklı metinde kaçışsız her `{` bir yerleştirme başlatır: içi tek bir ifade olarak ayrıştırılamazsa (ya da boşsa) sözdizimi hatasıdır, düz metne dönülmez (`"{değil}"` → `metindeki her '{' bir yerleştirme başlatır, ama '{değil}' bir ifade değil ... Düz süslü parantez için '\{' yazın ya da ham metin kullanın`). Böylece bir parçanın kod mu metin mi olduğu dilin anahtar kelimelerine bağlı değildir; JSON gibi süslü parantezli metinler ham metinle yazılır: `` `{"ad": "Ali"}` ``. Yerleştirmenin içindeki tırnaklı bir metinde süslü parantez yerleştirmeyi açıp kapatmaz (`"x {"a}b"} y"` → `x a}b y`). Yerleştirmedeki ifadenin sözcükleri dosyadaki gerçek satır ve sütunlarıyla bildirilir (önündeki kaçış dizileri konumu kaydırmaz).

```tr
//! tr kodlu 3 dil=tr
TR_kw_var ad = "Ali"
TR_kw_var p = {"ad": "Ayşe"}
TR_fn_print("Merhaba {ad}, 1 + 2 = {1 + 2}, kişi: {p['ad']}, düz: \{x\}")
```

**2.8** Deyimler satır sonuyla ya da isteğe bağlı `;` ile ayrılır. Satır sonu ifadeyi bitirir, parantez içinde (`(`, `[` ve sözlük `{`) bitirmez. Yeni satırda başlayan `(` ya da `[` önceki ifadeyi sürdürmez, yeni bir deyim başlatır; sondaki blok `{` (bkz. 6.7) önceki ifadeyle aynı satırda olmalıdır. Yeni satırın başındaki ikili işleç de önceki ifadeyi sürdürmez: `TR_kw_var a = 1` ardından gelen `-x` satırı `a = 1 - x` okunmaz, ayrı bir deyimdir ve değeri kullanılmadığı için uyarı verir (değeri kullanılan bir bloğun son ifadesi bloğun değeridir, uyarı vermez: `TR_kw_var g = TR_kw_if a { 0.01 } TR_kw_else { -0.01 }`); tekli biçimi olmayan işleç (`+`, `*`, `TR_kw_and`, `??` ...) satır başında sözdizimi hatasıdır ve hata nedenini söyler. İfadeyi bölmek için işleç satır sonuna yazılır (`1 +` ↵ `2`) ya da ifade parantez içine alınır. Zincir işleçleri satır başında da sürer: üye erişimi `.` ve `?.`, boru `|>`. Gerekçe: Go, Python, Kotlin ve Swift'te satır sonu ifadeyi bitirir; satır başındaki `-`'nin önceki satıra bağlandığı JavaScript davranışı bilinen bir tuzaktır.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var f = 5
(1 + 2) |> TR_fn_print        // ayrı deyim; '5(1 + 2)' çağrısı değil
TR_kw_var t = 1 +
    2                    // t = 3
```

**2.9** Kodsuz (Türkçe) dosyada dilin kelimelerinin yalnızca Türkçe yazımı geçerlidir; başka bir yazım dilinde (bugün İngilizce) yazılan kod o dilin adlarıyla okunur ve kodlanarak saklanır (17.1, 17.7). Anahtar kelimeler şunlardır:

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

Bu kural anahtar kelimelerle sınırlı değildir: yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar ve tip adları da (bkz. 3.13) yalnızca Türkçe harfleriyle yazıldıkları biçimde geçerlidir. Türkçe harfleri `c g i o s u` ile yazılmış bir biçim (`degisken`, `sonuc`, `dogru`, `yazdir`, `buyuk_harf`) dilin kelimesi değil, sıradan bir addır: `TR_kw_var degisken = 1` geçerlidir. Böyle bir ad tanımsızken kullanılırsa hata mesajı asıl Türkçe adı önerir:

- anahtar kelime: `'degisken' TR'de anahtar kelime değil; 'değişken' yazmak mı istediniz?`
- yerleşik fonksiyon ya da metot: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`, `'buyuk_harf' TR'de yok; 'büyük_harf' yazmak mı istediniz?`
- adlı argüman: `AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`

**2.9a** Her işin tek yazımı vardır: her anahtar kelimenin, yerleşik fonksiyonun (bkz. 13), metodun, özelliğin, adlı argümanın ve tip adının yalnızca bir adı vardır; eş anlamlı ya da başka yazımlı ikinci bir ad kabul edilmez, öyle bir ad sıradan bir addır (`TR_kw_var sınıf = "9-A"`, `TR_kw_var boy = 180`). Başka dillerden alışılmış adlar (`int`, `str`, `dict`, `lr`, `axis`, `.T` ...) da Türkçe yazımda dilin adı değildir (İngilizce yazımda `Int`, `lr` gibi adlar o dilin adlarıdır, 17.7); bir hatada (tanımsız ad, bilinmeyen metot ya da adlı argüman, tip açıklaması) hata mesajı asıl adı önerir: `'int' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`, `AdamW 'lr' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`. Bu adlar kod listesinde kodun `yabancı` alanındadır (17). Yorum `#` ile değil `//` ile yazılır (2.2). Dil taslak aşamasındadır: bir ad değişince ya da kaldırılınca eski adı için ipucu tutulmaz, kullanımlar yeni ada çevrilir (17.6).

**2.10** İfadeler ve bloklar en fazla 1000 düzey iç içe yazılabilir (`((((1))))` dört düzeydir); sınır aşılırsa program çalışmaz, `ifade ya da blok çok derin iç içe (en fazla 1000 düzey)` sözdizimi hatası verilir. Metin içindeki yerleştirmenin ifadesi (2.7) de bulunduğu yerin düzeyinden sayılır. (Sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi.)

**2.11** `$ad` kaçışı: `$` ve ardından gelen ad her zaman kullanıcının adıdır, dilin bir sözcüğüyle aynı olsa bile (`TR_kw_var $eğer = 3`, `TR_fn_print($eğer)`; ad `TR_kw_if`'dir). Kaçış, kodlu bir dosya (17) bir yazım dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir: gösterimde yalnızca gerektiği yerde yazılır (bir anahtar kelimeyle aynı ad, yazılsaydı dilin bir öğesine bağlanacak ad ya da kodla yazılmış bir sözcüğü kendi kapsamında yakalayacak ad), kodlama `$`'ı atar. Türkçe yazımda kullanıcı adları dilin sözcükleriyle çakışamadığı için kaçış Türkçe gösterimde ancak başka bir dilde yazılmış kodda görünür. `$` yalnızca bu kaçış içindir; bir adın ya da sayının hemen ardından yazılamaz (`a$b`, `1$x` sözdizimi hatasıdır: `$` atılınca iki sözcük birleşirdi). Kod biçimli adlar kaçışla da ayrılmıştır (2.3a).

Bağlamsal sözcükler anahtar kelime değildir ve ad olarak kullanılabilir: `TR_ctx_test` (yalnızca deyim başında ardından metin gelince, bkz. 12.2), `TR_ctx_as` (yalnızca `TR_kw_import` içinde), `TR_ctx_init` (yapının oluşturulurken çağrılan metodunun adı, 7.4), `TR_ctx_error` (`TR_kw_catch`da ad verilmeyince hata değişkeninin adı, 8.1), `TR_ctx_it` (sondaki bloğun parametresi, 6.7), `TR_ctx_acc` (yalnızca `TR_mem_reduce`'nin sondaki bloğunda önceki sonuç, 6.7), `_` (yalnızca `TR_kw_match` kolunda varsayılan). Bunların adları da kod listesindedir (`ctx` sınıfı, bkz. 17).

**2.12** İleti kodu (`H6903`: `H` ve tam dört rakam) bir ad değil sözcük birimidir ve ifadede kodun metnidir: `TR_fn_message(H6903, -1)` ile `TR_fn_message("H6903", -1)` aynıdır (8.5). Daha uzun, kısa ya da küçük harfli yazım (`H69030`, `h6903`) sıradan bir addır; tam `H` ve dört rakamlık sözcük ad olarak kullanılamaz. İleti kodu hiçbir yazım dilinde çevrilmez, kodlu dosyada olduğu gibi durur. Yazım yanlışı (`H690`) tanımsız ad hatasıdır; var olmayan kod `TR_fn_message` kurulurken bildirilir.
