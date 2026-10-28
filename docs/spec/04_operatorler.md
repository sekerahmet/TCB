# 4. İşleçler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "4. İşleçler".

Her kuralın emsali ve gerekçesi: [`rationale/04_operators.md`](rationale/04_operators.md).

**4.1** Öncelik, gevşekten sıkıya; aynı düzeydekiler soldan birleşir, `**` sağdan; karşılaştırmalar 4.4'teki gibi zincirlenir. `**` solundaki tekli işleçten sıkı, sağındakinden gevşek bağlar: `-2 ** 2` ≡ `-(2 ** 2)`, `2.0 ** -1` ≡ `2.0 ** (-1)`.

| Düzey | İşleçler |
| :-: | :-- |
| 1 | `TR_or` |
| 2 | `TR_and` |
| 3 | `TR_not x` |
| 4 | `==` `!=` `<` `>` `<=` `>=` |
| 5 | `??` |
| 6 | `..` `..=` |
| 7 | `\|` |
| 8 | `^` |
| 9 | `&` |
| 10 | `<<` `>>` |
| 11 | `+` `-` |
| 12 | `*` `/` `//` `%` `@` |
| 13 | `-x` `~x` |
| 14 | `**` |
| 15 | `f(x)` `a[i]` `a.b` `a?.b` `f { ... }` |

Örnekler: `TR_not a == b` ≡ `TR_not (a == b)`; `x ?? 0 > 3` ≡ `(x ?? 0) > 3`; `1..n + 1` ≡ `1..(n + 1)`; `x & 1 == 0` ≡ `(x & 1) == 0`; `-2 ** 2` = −4; `2 ** 3 ** 2` = 512.
- *Test:* TR testi.

**4.2** İki `TR_Int`in `+`, `-` ve `*` sonucu `TR_Int`tir (taşma 3.1). `/` her zaman `TR_Float` verir: `7 / 2` = 3.5, `6 / 3` = 2.0. `TR_Int` ile `TR_Float` karışırsa sonuç `TR_Float`tır. Sıfıra bölme çalışma hatasıdır.
- *Test:* TR testi.

**4.2a** `//` tam bölmedir ve aşağı yuvarlar: `7 // 2` = 3, `-7 // 2` = −4. `%` kalanı verir, işareti bölenin işaretidir: `-7 % 3` = 2, `7.5 % 2` = 1.5. İki `TR_Int`te her zaman `a == (a // b) * b + a % b`; ondalıkta yuvarlama yüzünden tutmayabilir. İkisi de iki `TR_Int`te `TR_Int`, ondalıkla `TR_Float` verir; sıfıra bölme çalışma hatasıdır, en küçük `TR_Int`'in `// -1`'i taşmadır (3.1).
- *Test:* TR testi.

**4.3** `**` üs alır. İki `TR_Int`te sonuç `TR_Int`tir (taşma 3.1) ve üs eksi olamaz (çalışma hatası; ileti `2.0 ** -1`'i, eksi tabanda `(-2.0) ** -1`'i önerir); bir taraf ondalıksa sonuç `TR_Float`tır. Sıfırın eksi üssü ve eksi sayının tam olmayan üssü çalışma hatasıdır (`0.0 ** -1`, `(-8.0) ** (1.0 / 3)`); tensörde sonraki bölümlerdedir.
- *Test:* TR testi.

**4.4** `<`, `>`, `<=`, `>=` sayılar ve metinler arasında tanımlıdır: sayılar 4.5'teki gibi kesin, metinler kod noktası sırasıyla karşılaştırılır. Yalnız bu dört işleçten oluşan zincir Python'daki gibi okunur: `0 <= i < n` ≡ `0 <= i TR_and i < n`, ortadaki ifade bir kez hesaplanır. İçinde `==` ya da `!=` olan zincir (`a < b == c`, `a == b == c`) sözdizimi hatasıdır, ileti parantez ya da `TR_and` önerir.
- *Test:* TR testi.

**4.5** `==` ve `!=` aynı tipten iki değeri karşılaştırır; ayrı tipler (`"1" == 1`) denetim hatasıdır. Sayılar tipten bağımsız, kesin karşılaştırılır, `TR_Int` yuvarlanmaz (`1 == 1.0`, `9007199254740993 != 9007199254740992.0`); `TR_nan` hiçbir sayıya eşit değildir. Tipten bağımsızlık yalnız iki sayı arasındadır: öğe tipleri ayrı listeler (`TR_List[TR_Int]` ile `TR_List[TR_Float]`) ayrı tiptir. İsteğe bağlı değer asıl tipten bir değerle ya da `TR_none`'la karşılaştırılabilir. Liste ve sözlük öğe öğe (sözlükte sıra önemsiz), bayt dizisi bayt bayt, `TR_JSON` JSON değeri olarak (sayılar 4.5'in sayı kuralıyla), aralık uçlarıyla karşılaştırılır (uçları ayrı iki boş aralık eşit değildir). Fonksiyonlar karşılaştırılamaz; fonksiyon içeren liste ve sözlük de. İsteğe bağlı bir fonksiyon `TR_none`'la karşılaştırılabilir (3.5). Yapının ve tensörün eşitliği sonraki bölümlerdedir.
- *Test:* TR testi; tip kuralı uygulanınca denetleyicinin testi.

**4.6** `TR_and`, `TR_or` ve `TR_not`'un işlenenleri `TR_Bool`dur (3.10); `TR_and` ve `TR_or` kısa devre yapar.
- *Test:* TR testi; işlenen kuralı uygulanınca denetleyicinin testi.

**4.7** `a ?? b`: `a` isteğe bağlıdır (`T?`); `a` `TR_none` ise sonuç `b`, değilse `a`'nın değeridir. `b` `T` ise sonuç `T`, `T?` ise `T?`. `a` isteğe bağlı değilse denetim hatasıdır, çünkü sağ taraf hiç kullanılmaz (3.5'teki `TR_none` karşılaştırması gibi). Sağ yanda parantezsiz bir ikili işleç varsa (`d.TR_get(k) ?? 0 + 1`, 4.1'e göre `?? (0 + 1)` okunur) sözdizimi hatasıdır; ileti `(d.TR_get(k) ?? 0) + 1`'i ya da sağ yanın parantezini önerir.
- *Test:* TR testi; hata uygulanınca denetleyicinin testi.

**4.8** `a?.b`: `a` `TR_none` ise sonuç `TR_none`, değilse `a.b`; sonucun tipi isteğe bağlıdır, `b` zaten isteğe bağlıysa yine `T?`'dir (iki kat isteğe bağlı olmaz). İsteğe bağlı değerin üyesine `.` ile erişmek denetim hatasıdır (3.5).
- *Test:* TR testi.

**4.9** Başka dillerin işleçleri TR'de yoktur; ileti TR'deki yazımı söyler. `&&`, `||` ve tekli `!` sözdizimi hatasıdır (`TR_and`, `TR_or`, `TR_not`); `|>` sözdizimi hatasıdır (`f(x)`). C ailesinin yorumları yorum değildir: satır başındaki `//` ve `/*` sözdizimi hatasıdır; satır sonunda `//`'den sonraki metin bir ifade değilse de aynı hata verilir; ileti `#`'i önerir (2.2). `//`'nin sağ yanındaki tanımsız adın iletisi de `#`'i önerir. Python'un üyelik sınaması `x TR_in l` yoktur: `TR_in` yalnız döngünün ve liste üretecinin başlığında, değişken adından sonra geçer (sonraki bölümlerde); bir ifadenin ardındaki `TR_in` sözdizimi hatasıdır, ileti `l.TR_contains(x)`'i önerir.
- *Test:* TR testi.

**4.10** `+` aynı tipten iki metni, iki bayt dizisini ya da iki listeyi uç uca ekler. `*` bir metni, bayt dizisini ya da listeyi bir `TR_Int` kadar yineler (`"-" * 40`, `[0] * n`, `3 * "ab"`); sayı sıfır ya da eksiyse sonuç boştur. Sonuç en çok 2³¹ baytlık metin ya da 2²⁷ öğelik liste olabilir; daha büyüğü çalışma hatasıdır. Öğeleri başvuruyla paylaşılan (3.6: liste ve sözlük; yapı ve tensör sonraki bölümlerde) liste yinelenemez: `[[0] * 3] * 3` denetim hatasıdır, çünkü üç satır aynı listeyi paylaşırdı; ileti her satırı ayrı kuran yazımı önerir. Metin, bayt dizisi, sayı ve mantık değeri değiştirilemediği için öğe olarak yinelenebilir. Başka bir tiple `+` ya da `*` denetim hatasıdır (`"a" + 1`); ileti metinde yerleştirmeyi (`"a\(1)"`), listede `l + [x]` ya da `l.TR_append(x)` yazımını önerir.
- *Test:* TR testi; tip kuralı ve yineleme uygulanınca denetleyicinin testi.

**4.11** `@` iki tensörün matris çarpımıdır. Tensörlerde öbür işleçlerin öğe öğe anlamı ve genişletme sonraki bölümlerdedir.
- *Test:* TR testi.

**4.12** Bit işleçleri yalnız `TR_Int`te tanımlıdır: `&`, `|`, `^` (dışlayan veya), `~` (tümleme), `<<`, `>>`. Kaydırma miktarı 0–63 dışındaysa çalışma hatasıdır; dışarı taşan bitler atılır (3.1'in taşma kuralı aritmetik içindir); `>>` işareti korur (`-8 >> 1` = −4).
- *Test:* TR testi.

```tr
# TR 4
TR_print(7 / 2, 7 // 2, -7 % 3, 2 ** 10, 0x0F & 0b0101, 1 << 4)   # çıktı: 3.5 3 2 1024 5 16
```
