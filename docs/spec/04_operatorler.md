# 4. Operatörler

**4.1** Öncelik tablosu (gevşekten sıkıya). Aynı satırdakiler soldan birleşir; `**` sağdan birleşir.

| Düzey | Operatörler | Not |
| :-: | :-- | :-- |
| 1 | `\|>` | boru hattı |
| 2 | `??` | yok-birleştirme |
| 3 | `veya` `\|\|` | |
| 4 | `ve` `&&` | |
| 5 | `değil x` | karşılaştırmadan gevşek: `değil 1 == 2` ≡ `değil (1 == 2)`; `değil a ve b` ≡ `(değil a) ve b` |
| 6 | `==` `!=` | |
| 7 | `<` `>` `<=` `>=` | |
| 8 | `..` `..=` | `1..2 + 3` ≡ `1..5` |
| 9 | `+` `-` | |
| 10 | `*` `/` `%` `@` | |
| 11 | `-x` `!x` | tekli |
| 12 | `**` | tekli eksiden sıkı: `-2 ** 2` = `-4`; `2 ** 3 ** 2` = `512` |
| 13 | `f(x)` `a[i]` `a.b` `a?.b` `f { ... }` | çağrı, indeks, üye, sondaki blok |

**4.2** Aritmetik: `Tam` ile `Tam` işlemi `Tam` verir; `/` bölümü tam çıkmazsa `Ondalık` verir (`7 / 2` = `3.5`, `6 / 3` = `2`). `Tam` ile `Ondalık` karışırsa sonuç `Ondalık`tır. `%` kalanı verir (ondalıklarda da). Sıfıra bölme ve sıfıra göre kalan hatadır. Tam bölme operatörü yoktur (`//` yorumdur).

**4.3** `**` üs alır; negatif üs `Ondalık` verir (`2 ** -1` = `0.5`).

**4.4** `< > <= >=` yalnızca sayılar arasında tanımlıdır (tensörlerde eleman bazlı 0/1 maskesi verir, bkz. 15.3); karşılaştırmalar zincirlenmez (`1 < 2 < 3` hatadır).

**4.5** Eşitlik: iki sayı tipten bağımsız değerle karşılaştırılır (`1 == 1.0`); iki `Tam` tam olarak karşılaştırılır (`9007199254740993 != 9007199254740992`); `Tam` ile `Ondalık` da değerleriyle kesin karşılaştırılır, `Tam` yuvarlanmaz (`9007199254740993 != 9007199254740992.0`, `9007199254740993 > 9007199254740992.0`); `<`, `>`, `<=`, `>=` aynı kurala uyar ve NaN hiçbir sayıya eşit değildir; diğer değerler aynı tipte ve aynı içerikteyse eşittir (`[1, "a"] == [1, "a"]`, `"1" != 1`, `yok != yanlış`). Listeler, sözlükler ve yapı nesneleri eleman eleman bu kuralla karşılaştırılır (`[1, 2] == [1.0, 2]`); sözlüklerde anahtar sırası önemsizdir. Fonksiyonlar, yerleşikler ve yapı tanımları kimlikle karşılaştırılır: aynı değer kendisine eşittir, görünüşleri aynı olsa da ayrı tanımlar eşit değildir (`(x => x) != (x => x)`); bir nesnenin metodu aynı nesne için kendisine eşittir (`k.selamla == k.selamla`). Aralıklar uçları ve dahil olup olmamalarıyla karşılaştırılır. Tensörler şekilleri ve tüm elemanları aynıysa eşittir. 0 boyutlu tensör, değeri (`.değer`) bir sayıya eşitse o sayıya eşittir (`tensör(7.0) == 7`, `tensör([[1, 2]])[0][1] == 2.0`, `toplam(t) == 10`); en az bir ekseni olan tensör sayıya eşit değildir (`tensör([7.0]) != 7`).

**4.6** `ve`/`&&` ile `veya`/`||` kısa devre yapar ve sonucu her zaman `Mantık` olarak verir (`1 && "a"` → `doğru`). `!x` ve `değil x` doğruluğun tersidir.

**4.7** `a ?? b`: `a` `yok` ise `b`, değilse `a` (yalnızca `yok` sayılır: `0 ?? 5` → `0`).

**4.8** `a?.b`: `a` `yok` ise hata yerine `yok` verir; `yok.b` bir hatadır.

**4.9** `x |> f` ≡ `f(x)`; `x |> f(a)` ≡ `f(x, a)`. Sağ taraf bir fonksiyon çağrısı ya da fonksiyon değeri olmalıdır. Boru yalnızca fonksiyonlarla çalışır: kullanıcı fonksiyonları ve ilk parametresi borudaki değeri alan yerleşik fonksiyonlar (`toplam`, `ortalama`, `min`, `maks`, `metin`, `tip`, `relu`, `softmax`, `json_metin` ...; tam liste [`kodlar.md`](../kodlar.md) "Boru hattıyla kullanılabilen fonksiyonlar"). Metotlar boruyla değil `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 }`); `l |> filtrele { ... }` yazılırsa hata metot biçimini önerir (13.6).

```tr
[1, 2, 3].haritala { o * 2 } |> toplam   // 12
```

**4.10** `+` en az bir tarafı metinse öbürünü metne çevirip birleştirir (`"a" + 1` → `"a1"`); iki listeyi uç uca ekler.

**4.11** `@` matris çarpımıdır (`tensör([[1, 2], [3, 4]]) @ tensör([[1], [1]])`; matris 2 boyutlu tensördür); `+ - * /` tensörlerde eleman bazlıdır (bkz. 15.3).
