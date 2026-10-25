# 2. Sözcük yapısı

**2.1** Kaynak UTF-8 metindir; bazı Windows editörlerinin eklediği baştaki BOM (U+FEFF) yok sayılır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir. Kaynağın ortasındaki NUL karakteri (U+0000) sözdizimi hatasıdır (metin dosyasında bulunmaz; ardından gelen kod sessizce yok sayılmaz). Dosya kodlu biçimde de saklanabilir (17.1).

**2.1a** Göründüğünden farklı okunan kaynak kabul edilmez. Yazı yönünü değiştiren görünmez karakterler (U+202A–U+202E, U+2066–U+2069, U+200E, U+200F, U+061C) kaynağın hiçbir yerinde, yorum ve metin içinde de, kullanılamaz: bir satırın gösterim sırasını değiştirip kodun başka bir şey gibi görünmesine yol açarlar. Adlarda görünmez dolgu harfleri (U+115F, U+1160, U+3164, U+FFA0) kullanılamaz. Bir ad birbirine benzeyen yazı sistemlerinden (Latin, Kiril, Yunan) yalnızca birinin harfleriyle yazılır: `yetkі` (son harfi Kiril) hatadır, çünkü `yetki`'den ayırt edilemez; tamamı tek yazı sisteminde olan adlar (`öğrenme`, `привет`, `αβγ`) geçerlidir.

**2.2** Yorumlar iki biçimdedir: satır sonuna kadar `//` ve çok satırlı `/* ... */`. Kapanmayan `/*` bir hatadır. `#` yorum değildir: metin dışında yazılan `#` bir sözdizimi hatasıdır (`'#' TR'de yorum değil; yorum için '//' yazın`); metin içinde (`"#22c55e"`) sıradan bir karakterdir.

```tr
değişken x = 1  // satır sonuna kadar yorum
/* çok
   satırlı */
```

**2.3** Tanımlayıcılar bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; Türkçe harfler (ç ğ ı İ ö ş ü) dahil tüm Unicode harfleri geçerlidir: `öğrenci_sayısı2`.

**2.3a** Kod biçimindeki adlar ayrılmıştır: bir büyük harf ve üç rakamdan oluşan ad (`G100`, `A001`, `K001`) dilin kodlarının yazımıdır (17) ve kullanıcı adı olarak kullanılamaz; Türkçe yazılmış dosyada sözdizimi hatası verir: `'G100' kod biçiminde bir ad (büyük harf ve üç rakam); bu biçimdeki adlar dilin kodları için ayrılmıştır, başka bir ad seçin (ör. G100_ ya da g100). Kodlu dosyalar ilk satırdaki '//! tr kodlu 2 dil=tr' başlığıyla başlar`. Kodlu dosyada (17.1) bu adlar dilin sözcükleridir; `g100`, `G1000`, `G100_` sıradan adlardır.

**2.3b** Ad yazılışı (PEP 8 düzeni): sabitler `BÜYÜK_YILAN` (`sabit EN_BÜYÜK = 10`), yapılar `BüyükDeve` (`yapı SıçramaBekçisi`; kısaltma bütünüyle büyük kalır: `GPT`), değişken, fonksiyon, parametre, alan ve metot adları `küçük_yılan` (`öğrenci_sayısı`, `hazır_model`) yazılır. Matematik adları muaftır: en çok üç harf ve rakamdan oluşan parça büyük harfli olabilir (`W`, `X`, `Wq`, `dW`, `W1`, `QKV`, `X_veri`); formüllerin yazımıdır. Baştaki `_` ve `$` (2.11) yazılışa sayılmaz. Kurala uymayan ad hata değil uyarıdır (Rust'ın `non_snake_case`, `non_camel_case_types`, `non_upper_case_globals` uyarıları gibi; program çalışır): uyarı önerilen yazımı verir (`'öğrenciSayısı' yerine 'öğrenci_sayısı'`); büyük harfle yazılmış bir değişkene, değeri değişmiyorsa `sabit` ile tanımlanması önerilir. Öneri adda Türkçeye özgü harf varsa ya da gösterim dili Türkçeyse Türkçe büyük/küçük harf kuralıyla (`ADIM` → `adım`) kurulur. Depodaki kitaplık, örnek, test ve araç programları uyarısızdır. Karşılaştırma: Python (PEP 8) aynı düzeni gelenek olarak koyar, Kotlin sabitlerde aynıdır ama değişkenlerde `küçükDeve` kullanır, Swift sabitleri ayırmaz; TR'de `sabit` bir anahtar kelime olduğundan yazılış ile tanım birbirini doğrular.

**2.4** Tam sayı sabitleri ondalık rakamlardan oluşur; okunabilirlik için `_` kullanılabilir: `1_000_000`.

**2.5** Ondalık sabitler nokta içerir ve noktadan sonra rakam gelmelidir: `3.14`. (`1..5` bir aralıktır, ondalık değil.) Bilimsel yazım da Ondalık verir: üs harfi `e`/`E`, isteğe bağlı `+`/`-` ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); üsten sonra rakam yoksa `e` sayıya ait değildir (`2e` → `2` ve `e` adı).

**2.6** Metinler çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır; `\n \t \r \" \' \\ \{ \}` kaçış dizileri tanınır. Bilinmeyen bir kaçış dizisi sözdizimi hatasıdır, ters bölü sessizce atılmaz (`"\d+"`, `"C:\veri"` → `bilinmeyen kaçış dizisi '\d' ...; ters bölüyü olduğu gibi yazmak için ham metin kullanın ya da ters bölüyü ikileyin`): düzenli ifadeler ve Windows yolları için ham metin (2.6a) ya da `\\` yazılır. Metin birden çok satıra yayılabilir; metnin içindeki satır sonu kaynağın satır sonu biçiminden bağımsız olarak `\n`dir (CRLF'li bir dosyada metnin değerine `\r` girmez; `\r` yalnızca kaçışla yazılır). `tr biçimle` metinlerin içindekiler dahil bütün satır sonlarını LF yazar.

**2.6a** Ham metin ters tırnakla (`` `...` ``) yazılır: içinde kaçış dizisi ve `{ifade}` yerleştirmesi (2.7) yoktur, ters bölü, tırnaklar ve süslü parantezler olduğu gibi kalır. Düzenli ifadeler, Windows yolları ve yerleştirme istenmeyen `{` içeren metinler için önerilen yazımdır. Ham metin de `Metin` değeridir ve birden çok satıra yayılabilir (satır sonu yine `\n`dir); içine ters tırnak yazılamaz (o zaman tırnaklı metin kullanılır). Kapanmayan ham metin sözdizimi hatasıdır. `tr biçimle` ham metni olduğu gibi korur.

```tr
değişken yol = `C:\veri\tokenizer.json`
değişken tarih = `\d{4}-\d{2}-\d{2}`     // "\\d\{4\}-\\d\{2\}-\\d\{2\}" ile aynı
```

**2.7** Metin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır (ya da ham metin, 2.6a). Tırnaklı metinde kaçışsız her `{` bir yerleştirme başlatır: içi tek bir ifade olarak ayrıştırılamazsa (ya da boşsa) sözdizimi hatasıdır, düz metne dönülmez (`"{değil}"` → `metindeki her '{' bir yerleştirme başlatır, ama '{değil}' bir ifade değil ... Düz süslü parantez için '\{' yazın ya da ham metin kullanın`). Böylece bir parçanın kod mu metin mi olduğu dilin anahtar kelimelerine bağlı değildir; JSON gibi süslü parantezli metinler ham metinle yazılır: `` `{"ad": "Ali"}` ``. Yerleştirmenin içindeki tırnaklı bir metinde süslü parantez yerleştirmeyi açıp kapatmaz (`"x {"a}b"} y"` → `x a}b y`). Yerleştirmedeki ifadenin sözcükleri dosyadaki gerçek satır ve sütunlarıyla bildirilir (önündeki kaçış dizileri konumu kaydırmaz).

```tr
değişken ad = "Ali"
değişken p = {"ad": "Ayşe"}
yazdır("Merhaba {ad}, 1 + 2 = {1 + 2}, kişi: {p['ad']}, düz: \{x\}")
```

**2.8** Deyimler satır sonuyla ya da isteğe bağlı `;` ile ayrılır. Satır sonu ifadeyi bitirir, parantez içinde (`(`, `[` ve sözlük `{`) bitirmez. Yeni satırda başlayan `(` ya da `[` önceki ifadeyi sürdürmez, yeni bir deyim başlatır; sondaki blok `{` (bkz. 6.7) önceki ifadeyle aynı satırda olmalıdır. Yeni satırın başındaki ikili işleç de önceki ifadeyi sürdürmez: `değişken a = 1` ardından gelen `-x` satırı `a = 1 - x` okunmaz, ayrı bir deyimdir ve değeri kullanılmadığı için uyarı verir; tekli biçimi olmayan işleç (`+`, `*`, `ve`, `??` ...) satır başında sözdizimi hatasıdır ve hata nedenini söyler. İfadeyi bölmek için işleç satır sonuna yazılır (`1 +` ↵ `2`) ya da ifade parantez içine alınır. Zincir işleçleri satır başında da sürer: üye erişimi `.` ve `?.`, boru `|>`. Gerekçe: Go, Python, Kotlin ve Swift'te satır sonu ifadeyi bitirir; satır başındaki `-`'nin önceki satıra bağlandığı JavaScript davranışı bilinen bir tuzaktır.

```tr
değişken f = 5
(1 + 2) |> yazdır        // ayrı deyim; '5(1 + 2)' çağrısı değil
değişken t = 1 +
    2                    // t = 3
```

**2.9** Kodsuz (Türkçe) dosyada dilin kelimelerinin yalnızca Türkçe yazımı geçerlidir; başka bir yazım dilinde (bugün İngilizce) yazılan kod o dilin adlarıyla okunur ve kodlanarak saklanır (17.1, 17.7). Anahtar kelimeler şunlardır:

| Görev | Anahtar kelimeler |
| :-- | :-- |
| Bildirim | `değişken`, `sabit` |
| Fonksiyon | `fonksiyon`, `sonuç` |
| Koşul | `eğer`, `değilse`, `eşle`, `varsayılan` |
| Döngü | `döngü`, `içinde`, `iken`, `çık`, `devam` |
| Değer ve mantık | `doğru`, `yanlış`, `yok`, `ve`, `veya`, `değil` |
| Yapı | `yapı`, `kendi` |
| Hata yönetimi | `dene`, `yakala`, `fırlat` |
| Modül ve eşzamanlılık | `aktar`, `eşzamanlı`, `ertele` |
| Hata ayıklama | `kesme_noktası` |

Bu kural anahtar kelimelerle sınırlı değildir: yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar ve tip adları da (bkz. 3.13) yalnızca Türkçe harfleriyle yazıldıkları biçimde geçerlidir. Türkçe harfleri `c g i o s u` ile yazılmış bir biçim (`degisken`, `sonuc`, `dogru`, `yazdir`, `buyuk_harf`) dilin kelimesi değil, sıradan bir addır: `değişken degisken = 1` geçerlidir. Böyle bir ad tanımsızken kullanılırsa hata mesajı asıl Türkçe adı önerir:

- anahtar kelime: `'degisken' TR'de anahtar kelime değil; 'değişken' yazmak mı istediniz?`
- yerleşik fonksiyon ya da metot: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`, `'buyuk_harf' TR'de yok; 'büyük_harf' yazmak mı istediniz?`
- adlı argüman: `AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`

**2.9a** Her işin tek yazımı vardır: her anahtar kelimenin, yerleşik fonksiyonun (bkz. 13), metodun, özelliğin, adlı argümanın ve tip adının yalnızca bir adı vardır; eş anlamlı ya da başka yazımlı ikinci bir ad kabul edilmez, öyle bir ad sıradan bir addır (`değişken sınıf = "9-A"`, `değişken boy = 180`). Başka dillerden alışılmış adlar (`int`, `str`, `dict`, `lr`, `axis`, `.T` ...) da Türkçe yazımda dilin adı değildir (İngilizce yazımda `Int`, `lr` gibi adlar o dilin adlarıdır, 17.7); bir hatada (tanımsız ad, bilinmeyen metot ya da adlı argüman, tip açıklaması) hata mesajı asıl adı önerir: `'int' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`, `AdamW 'lr' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`. Bu adlar kod listesinde kodun `yabancı` alanındadır (17). Yorum `#` ile değil `//` ile yazılır (2.2). Dil taslak aşamasındadır: bir ad değişince ya da kaldırılınca eski adı için ipucu tutulmaz, kullanımlar yeni ada çevrilir (17.6).

**2.10** İfadeler ve bloklar en fazla 1000 düzey iç içe yazılabilir (`((((1))))` dört düzeydir); sınır aşılırsa program çalışmaz, `ifade ya da blok çok derin iç içe (en fazla 1000 düzey)` sözdizimi hatası verilir. Metin içindeki yerleştirmenin ifadesi (2.7) de bulunduğu yerin düzeyinden sayılır. (Sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi.)

**2.11** `$ad` kaçışı: `$` ve ardından gelen ad her zaman kullanıcının adıdır, dilin bir sözcüğüyle aynı olsa bile (`değişken $eğer = 3`, `yazdır($eğer)`; ad `eğer`'dir). Kaçış, kodlu bir dosya (17) bir yazım dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir: gösterimde yalnızca gerektiği yerde yazılır (bir anahtar kelimeyle aynı ad, yazılsaydı dilin bir öğesine bağlanacak ad ya da kodla yazılmış bir sözcüğü kendi kapsamında yakalayacak ad), kodlama `$`'ı atar. Türkçe yazımda kullanıcı adları dilin sözcükleriyle çakışamadığı için kaçış Türkçe gösterimde ancak başka bir dilde yazılmış kodda görünür. `$` yalnızca bu kaçış içindir; bir adın ya da sayının hemen ardından yazılamaz (`a$b`, `1$x` sözdizimi hatasıdır: `$` atılınca iki sözcük birleşirdi). Kod biçimli adlar kaçışla da ayrılmıştır (2.3a).

Bağlamsal sözcükler anahtar kelime değildir ve ad olarak kullanılabilir: `test` (yalnızca deyim başında ardından metin gelince, bkz. 12.2), `olarak` (yalnızca `aktar` içinde), `kurucu` (yapının oluşturulurken çağrılan metodunun adı, 7.4), `hata` (`yakala`da ad verilmeyince hata değişkeninin adı, 8.1), `o` (sondaki bloğun parametresi, 6.7), `birikim` (yalnızca `indirge`'nin sondaki bloğunda önceki sonuç, 6.7), `_` (yalnızca `eşle` kolunda varsayılan). Bunların adları da kod listesindedir (`A1xx`, bkz. 17).
