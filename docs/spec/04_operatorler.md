# 4. Operatörler

**4.1** Öncelik tablosu (gevşekten sıkıya). Aynı satırdakiler soldan birleşir; `**` sağdan birleşir.

| Düzey | Operatörler | Not |
| :-: | :-- | :-- |
| 1 | `\|>` | boru hattı |
| 2 | `??` | yok-birleştirme |
| 3 | `TR_kw_or` `\|\|` | |
| 4 | `TR_kw_and` `&&` | |
| 5 | `TR_kw_not x` | karşılaştırmadan gevşek: `TR_kw_not 1 == 2` ≡ `TR_kw_not (1 == 2)`; `TR_kw_not a TR_kw_and b` ≡ `(TR_kw_not a) TR_kw_and b` |
| 6 | `==` `!=` | |
| 7 | `<` `>` `<=` `>=` | |
| 8 | `..` `..=` | `1..2 + 3` ≡ `1..5` |
| 9 | `+` `-` | |
| 10 | `*` `/` `%` `@` | |
| 11 | `-x` `!x` | tekli |
| 12 | `**` | tekli eksiden sıkı: `-2 ** 2` = `-4`; `2 ** 3 ** 2` = `512` |
| 13 | `f(x)` `a[i]` `a.b` `a?.b` `f { ... }` | çağrı, indeks, üye, sondaki blok |

**4.2** Aritmetik: `TR_type_Int` ile `TR_type_Int` işlemi `TR_type_Int` verir; `/` bölümü tam çıkmazsa `TR_type_Float` verir (`7 / 2` = `3.5`, `6 / 3` = `2`). `TR_type_Int` ile `TR_type_Float` karışırsa sonuç `TR_type_Float`tır. `%` kalanı verir (ondalıklarda da). Sıfıra bölme ve sıfıra göre kalan hatadır. Tam bölme operatörü yoktur (`//` yorumdur).

**4.3** `**` üs alır; negatif üs `TR_type_Float` verir (`2 ** -1` = `0.5`).

**4.4** `< > <= >=` yalnızca sayılar arasında tanımlıdır (tensörlerde eleman bazlı 0/1 maskesi verir, bkz. 15.6); karşılaştırmalar zincirlenmez (`1 < 2 < 3` hatadır).

**4.5** Eşitlik: iki sayı tipten bağımsız değerle karşılaştırılır (`1 == 1.0`); iki `TR_type_Int` tam olarak karşılaştırılır (`9007199254740993 != 9007199254740992`); `TR_type_Int` ile `TR_type_Float` da değerleriyle kesin karşılaştırılır, `TR_type_Int` yuvarlanmaz (`9007199254740993 != 9007199254740992.0`, `9007199254740993 > 9007199254740992.0`); `<`, `>`, `<=`, `>=` aynı kurala uyar ve NaN hiçbir sayıya eşit değildir; diğer değerler aynı tipte ve aynı içerikteyse eşittir (`[1, "a"] == [1, "a"]`, `"1" != 1`, `TR_kw_none != TR_kw_false`). Listeler, sözlükler ve yapı nesneleri eleman eleman bu kuralla karşılaştırılır (`[1, 2] == [1.0, 2]`); sözlüklerde anahtar sırası önemsizdir. Fonksiyonlar, yerleşikler ve yapı tanımları kimlikle karşılaştırılır: aynı değer kendisine eşittir, görünüşleri aynı olsa da ayrı tanımlar eşit değildir (`(x => x) != (x => x)`); bir nesnenin metodu aynı nesne için kendisine eşittir (`k.selamla == k.selamla`). Aralıklar uçları ve dahil olup olmamalarıyla karşılaştırılır. Tensörler şekilleri ve tüm elemanları aynıysa eşittir. 0 boyutlu tensör, değeri (`.TR_mem_tolist`) bir sayıya eşitse o sayıya eşittir (`TR_fn_tensor(7.0) == 7`, `TR_fn_tensor([[1, 2]])[0][1] == 2.0`, `TR_fn_sum(t) == 10`); en az bir ekseni olan tensör sayıya eşit değildir (`TR_fn_tensor([7.0]) != 7`).

**4.6** `TR_kw_and`/`&&` ile `TR_kw_or`/`||` kısa devre yapar ve sonucu her zaman `TR_type_Bool` olarak verir (`1 && "a"` → `TR_kw_true`). `!x` ve `TR_kw_not x` doğruluğun tersidir.

**4.7** `a ?? b`: `a` `TR_kw_none` ise `b`, değilse `a` (yalnızca `TR_kw_none` sayılır: `0 ?? 5` → `0`).

**4.8** `a?.b`: `a` `TR_kw_none` ise hata yerine `TR_kw_none` verir; `TR_kw_none.b` bir hatadır.

**4.9** `x |> f` ≡ `f(x)`; `x |> f(a)` ≡ `f(x, a)`. Sağ taraf bir fonksiyon çağrısı ya da fonksiyon değeri olmalıdır. Boru yalnızca fonksiyonlarla çalışır: kullanıcı fonksiyonları ve ilk parametresi borudaki değeri alan yerleşik fonksiyonlar (`TR_fn_sum`, `TR_fn_mean`, `TR_fn_min`, `TR_fn_max`, `TR_fn_str`, `TR_fn_type`, `TR_fn_relu`, `TR_fn_softmax`, `TR_fn_json_string` ...; tam liste [`codes.md`](../codes.md) "Boru hattıyla kullanılabilen fonksiyonlar"). Metotlar boruyla değil `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 }`); `l |> filtrele { ... }` yazılırsa hata metot biçimini önerir (13.6).

```tr
//! tr kodlu 3 dil=tr
[1, 2, 3].TR_mem_map { TR_ctx_it * 2 } |> TR_fn_sum   // 12
```

**4.10** `+` en az bir tarafı metinse öbürünü metne çevirip birleştirir (`"a" + 1` → `"a1"`); iki listeyi uç uca ekler.

**4.11** `@` matris çarpımıdır (`TR_fn_tensor([[1, 2], [3, 4]]) @ TR_fn_tensor([[1], [1]])`; matris 2 boyutlu tensördür); `+ - * /` tensörlerde eleman bazlıdır (bkz. 15.6).
