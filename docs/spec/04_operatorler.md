# 4. İşleçler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "4. İşleçler".

Her kuralın emsali ve gerekçesi: [`rationale/04_operators.md`](rationale/04_operators.md).

**4.1** Öncelik, gevşekten sıkıya; aynı düzeydekiler soldan birleşir, `**` sağdan; karşılaştırmalar birleşmez (4.4).

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
- *Test:* TR testi; yeni öncelik uygulanınca TR testi.

**4.2** İki `TR_Int`in `+`, `-` ve `*` sonucu `TR_Int`tir (taşma 3.1). `/` her zaman `TR_Float` verir: `7 / 2` = 3.5, `6 / 3` = 2.0. `TR_Int` ile `TR_Float` karışırsa sonuç `TR_Float`tır. Sıfıra bölme çalışma hatasıdır.
- *Test:* TR testi; `/`'nin her zaman ondalık vermesi uygulanınca TR testi.

**4.2a** `//` tam bölmedir ve aşağı yuvarlar: `7 // 2` = 3, `-7 // 2` = −4. `%` kalanı verir, işareti bölenin işaretidir: `-7 % 3` = 2, `7.5 % 2` = 1.5. Her zaman `a == (a // b) * b + a % b`. İkisi de iki `TR_Int`te `TR_Int`, ondalıkla `TR_Float` verir; sıfıra bölme çalışma hatasıdır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**4.3** `**` üs alır. İki `TR_Int`te sonuç `TR_Int`tir ve üs eksi olamaz (çalışma hatası; ileti `2.0 ** -1`'i önerir); bir taraf ondalıksa sonuç `TR_Float`tır.
- *Test:* TR testi; tam sayı üssünün kuralı uygulanınca TR testi.

**4.4** `<`, `>`, `<=`, `>=` sayılar ve metinler arasında tanımlıdır: sayılar 4.5'teki gibi kesin, metinler kod noktası sırasıyla karşılaştırılır. Karşılaştırmalar zincirlenmez: `0 <= i < n` ve `a < b == c` sözdizimi hatasıdır, ileti `0 <= i TR_and i < n` ya da parantez önerir.
- *Test:* TR testi; metin karşılaştırması ve zincir hatası uygulanınca TR testi.

**4.5** `==` ve `!=` aynı tipten iki değeri karşılaştırır; ayrı tipler (`"1" == 1`) denetim hatasıdır. Sayılar tipten bağımsız, kesin karşılaştırılır, `TR_Int` yuvarlanmaz (`1 == 1.0`, `9007199254740993 != 9007199254740992.0`); `TR_nan` hiçbir sayıya eşit değildir. İsteğe bağlı değer asıl tipten bir değerle ya da `TR_none`'la karşılaştırılabilir. Liste ve sözlük öğe öğe (sözlükte sıra önemsiz), aralık uçlarıyla karşılaştırılır. Fonksiyonlar karşılaştırılamaz. Yapının ve tensörün eşitliği sonraki bölümlerdedir.
- *Test:* TR testi; tip kuralı uygulanınca denetleyicinin testi.

**4.6** `TR_and`, `TR_or` ve `TR_not`'un işlenenleri `TR_Bool`dur (3.10); `TR_and` ve `TR_or` kısa devre yapar.
- *Test:* TR testi; işlenen kuralı uygulanınca denetleyicinin testi.

**4.7** `a ?? b`: `a` isteğe bağlıdır (`T?`); `a` `TR_none` ise sonuç `b`, değilse `a`'nın değeridir. `b` `T` ise sonuç `T`, `T?` ise `T?`. `a` isteğe bağlı değilse uyarı verilir, çünkü sağ taraf hiç kullanılmaz.
- *Test:* TR testi; uyarı uygulanınca denetleyicinin testi.

**4.8** `a?.b`: `a` `TR_none` ise sonuç `TR_none`, değilse `a.b`; sonucun tipi isteğe bağlıdır. İsteğe bağlı değerin üyesine `.` ile erişmek denetim hatasıdır (3.5).
- *Test:* TR testi.

**4.9** Başka dillerin işleçleri TR'de yoktur; ileti TR'deki yazımı söyler. `&&`, `||` ve tekli `!` sözdizimi hatasıdır (`TR_and`, `TR_or`, `TR_not`); `|>` sözdizimi hatasıdır (`f(x)`). C ailesinin yorumları yorum değildir: satır başındaki `//` ve `/*` sözdizimi hatasıdır; satır sonunda `//`'den sonraki metin bir ifade değilse de aynı hata verilir; ileti `#`'i önerir (2.2).
- *Test:* TR testi; iletiler uygulanınca TR testi.

**4.10** `+` aynı tipten iki metni, iki bayt dizisini ya da iki listeyi uç uca ekler. Başka bir tiple `+` denetim hatasıdır (`"a" + 1`); ileti yerleştirmeyi önerir (`"a\(1)"`).
- *Test:* TR testi; tip kuralı uygulanınca denetleyicinin testi.

**4.11** `@` iki tensörün matris çarpımıdır. Tensörlerde öbür işleçlerin öğe öğe anlamı ve genişletme sonraki bölümlerdedir.
- *Test:* TR testi.

**4.12** Bit işleçleri yalnız `TR_Int`te tanımlıdır: `&`, `|`, `^` (dışlayan veya), `~` (tümleme), `<<`, `>>`. Kaydırma miktarı 0–63 dışındaysa çalışma hatasıdır; dışarı taşan bitler atılır (3.1'in taşma kuralı aritmetik içindir); `>>` işareti korur (`-8 >> 1` = −4).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

```tr
# TR 4
TR_print(7 / 2, 7 // 2, -7 % 3, 2 ** 10, 0x0F & 0b0101, 1 << 4)   # çıktı: 3.5 3 2 1024 5 16
```
