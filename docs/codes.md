# TR kod listesi

<!-- Bu dosya internal/lang/codes/definitions.json ve internal/lang/codes/tr.json'dan üretilir; elle düzenlemeyin: go run ./tools/gen/codesdoc -->

Dilin kullanıcıya görünen her adı sabit bir **koda** bağlıdır (bkz. `docs/spec/17_kodlar.md` §17). Bu belge her kodun ne işe yaradığını, imzasını ve örneğini verir; "hangi kod ne işe yarar" sorusunun tek başvurusudur. Yeni bir ad gerekmeden önce burada aynı işi yapan bir kod olup olmadığına bakın.

İmzalarda parametrelerin yalnızca tipleri yazılır: `[Tip]` isteğe bağlı, `[Tip...]` sıfır ya da çok, `Tip...` bir ya da çok argümandır; `ad: Tip = varsayılan` adlı argümandır, `→` dönüş tipidir. Metot ve özellikler `Tip.ad` biçiminde, geçerli oldukları her tip için ayrı satırda yazılır.

**Değer dönüştüren ya da değere soru soran iş metottur** (`liste.sırala()`, `metin.uzunluk`); **matematik ve tensör işi fonksiyondur** (`toplam(liste)`, `relu(x)`).

Toplam 564 kod: 28 anahtar kelime, 4 bağlamsal sözcük, 2 değer adı, 44 tip, 127 fonksiyon, 11 sabit, 61 metot, 97 özellik, 136 adlı argüman.

## İçindekiler

- Dilin çekirdeği: Anahtar kelimeler · Bağlamsal sözcükler · Değer adları · Tip adları
- Yerleşikler: Yerleşik fonksiyonlar · Yerleşik sabitler · Adlı argümanlar
- Metot ve özellikler (x.ad): Metot ve özellikler
- Komutlar (tr, kabuk, hata ayıklayıcı): tr komutları · tr index alt komutları · Komut satırı seçenekleri · Kabuk komutları · Hata ayıklayıcı komutları

## Dilin çekirdeği

### Anahtar kelimeler

#### `TR_and` (`TR_kw_and`, `ve`)

Mantıksal ve; işlenenleri Mantık olmalıdır, kısa devre yapar. TR'de && yoktur: yazılırsa sözdizimi hatasıdır, ileti ve'yi önerir.

```tr
TR_print(1 < 2 TR_and 2 < 3, TR_false TR_and TR_true)
```

Görünümü:

```text
en: print(1 < 2 and 2 < 3, false and true)
tr: yazdır(1 < 2 ve 2 < 3, yanlış ve doğru)
```

İlişkili: `TR_or`, `TR_not`

#### `TR_async` (`TR_kw_async`, `eşzamanlı`)

Bloğu ya da çağrıyı ayrı bir görev olarak başlatıp hemen devam eder; aynı anda yalnızca bir görev TR kodu çalıştırır.

```tr
TR_async { TR_print("görevde") }; TR_wait_all(); TR_print("bitti")
```

Görünümü:

```text
en: async { print("görevde") }; wait_all(); print("bitti")
tr: eşzamanlı { yazdır("görevde") }; bekle_hepsi(); yazdır("bitti")
```

İlişkili: `TR_wait_all(...)`

#### `TR_break` (`TR_kw_break`, `çık`)

En içteki döngüden çıkar; döngü dışında yazılması derleme hatasıdır (fonksiyondan çıkmak için sonuç yazılır).

```tr
TR_for x TR_in 1..10 { TR_if x == 3 { TR_break }; TR_print(x) }
```

Görünümü:

```text
en: for x in 1..10 { if x == 3 { break }; print(x) }
tr: döngü x içinde 1..10 { eğer x == 3 { çık }; yazdır(x) }
```

İlişkili: `TR_continue`

#### `TR_breakpoint` (`TR_kw_breakpoint`, `kesme_noktası`)

Programı bu satırda durdurup hata ayıklayıcıyı açar; kesme_noktası eğer koşul biçiminde yalnızca koşul doğruyken durur.

```tr
TR_var x = 2; TR_breakpoint TR_if x > 5; TR_print(x)
```

Görünümü:

```text
en: var x = 2; breakpoint if x > 5; print(x)
tr: değişken x = 2; kesme_noktası eğer x > 5; yazdır(x)
```

#### `TR_catch` (`TR_kw_catch`, `yakala`)

dene bloğunda hata olursa çalışır ve hata mesajını verilen ada (ad verilmezse hata adına) Metin olarak bağlar.

```tr
TR_try { TR_var z = 1 / 0 } TR_catch h { TR_print(h) }
```

Görünümü:

```text
en: try { var z = 1 / 0 } catch h { print(h) }
tr: dene { değişken z = 1 / 0 } yakala h { yazdır(h) }
```

İlişkili: `TR_try`, `TR_error`

#### `TR_const` (`TR_kw_const`, `sabit`)

Yeniden atanamayan bir ad tanımlar; ihlal çalıştırmadan önce yakalanır, ama değer bir listeyse içeriği değişebilir.

```tr
TR_const PI = 3.14; TR_print(PI * 2)
```

Görünümü:

```text
en: const PI = 3.14; print(PI * 2)
tr: sabit PI = 3.14; yazdır(PI * 2)
```

İlişkili: `TR_var`

#### `TR_continue` (`TR_kw_continue`, `devam`)

En içteki döngünün sonraki turuna geçer; yalnızca bir döngü gövdesinde geçerlidir.

```tr
TR_for x TR_in 1..=5 { TR_if x % 2 == 0 { TR_continue }; TR_print(x) }
```

Görünümü:

```text
en: for x in 1..=5 { if x % 2 == 0 { continue }; print(x) }
tr: döngü x içinde 1..=5 { eğer x % 2 == 0 { devam }; yazdır(x) }
```

İlişkili: `TR_break`

#### `TR_defer` (`TR_kw_defer`, `ertele`)

Çağrıyı ya da bloğu içinde bulunduğu fonksiyon bitince çalıştırır; birden çok ertele ters sırayla çalışır, erken sonuç ve hata durumunda da çalışır.

```tr
TR_func iş() { TR_defer TR_print("temizlik"); TR_print("iş") }; iş()
```

Görünümü:

```text
en: func iş() { defer print("temizlik"); print("iş") }; iş()
tr: fonksiyon iş() { ertele yazdır("temizlik"); yazdır("iş") }; iş()
```

#### `TR_else` (`TR_kw_else`, `değilse`)

eğer koşulu tutmadığında çalışacak dalı başlatır; değilse eğer ile yeni bir koşul zincirlenir.

```tr
TR_var n = -3; TR_var tür = TR_if n < 0 { "negatif" } TR_else TR_if n == 0 { "sıfır" } TR_else { "pozitif" }; TR_print(tür)
```

Görünümü:

```text
en: var n = -3; var tür = if n < 0 { "negatif" } else if n == 0 { "sıfır" } else { "pozitif" }; print(tür)
tr: değişken n = -3; değişken tür = eğer n < 0 { "negatif" } değilse eğer n == 0 { "sıfır" } değilse { "pozitif" }; yazdır(tür)
```

İlişkili: `TR_if`

#### `TR_enum` (`TR_kw_enum`, `durumlu`)

Durumlu bir tip tanımlar: değer sayılı durumlardan biridir, durum alan taşıyabilir; alanlar eşlemeyle okunur.

```tr
TR_enum D { Bekliyor; Bitti(n: TR_Int) }; TR_print(D.Bitti(3), D.Bekliyor != D.Bitti(1))
```

Görünümü:

```text
en: enum D { Bekliyor; Bitti(n: Int) }; print(D.Bitti(3), D.Bekliyor != D.Bitti(1))
tr: durumlu D { Bekliyor; Bitti(n: Tam) }; yazdır(D.Bitti(3), D.Bekliyor != D.Bitti(1))
```

İlişkili: `TR_struct`, `TR_match`

#### `TR_false` (`TR_kw_false`, `yanlış`)

Mantık tipinin yanlış değeridir. Koşul yalnız Mantık değeri alır: yok, 0, boş metin ve boş liste koşul olamaz, açık sınama yazılır (l.uzunluk == 0, x != yok).

```tr
TR_print(TR_false TR_or TR_true, TR_not TR_false)
```

Görünümü:

```text
en: print(false or true, not false)
tr: yazdır(yanlış veya doğru, değil yanlış)
```

İlişkili: `TR_true`

#### `TR_for` (`TR_kw_for`, `döngü`)

Bir kaynağı gezer (döngü x içinde ...), koşul doğru oldukça tekrarlar (döngü koşul { }) ya da sonsuz döner (döngü { }); her tur kendi kapsamında çalışır.

```tr
TR_for i, x TR_in ["a", "b"] { TR_print(i, x) }
```

Görünümü:

```text
en: for i, x in ["a", "b"] { print(i, x) }
tr: döngü i, x içinde ["a", "b"] { yazdır(i, x) }
```

İlişkili: `TR_in`

#### `TR_func` (`TR_kw_func`, `fonksiyon`)

Fonksiyon tanımlar; parametrelere varsayılan değer verilebilir, tanım öne alınmaz (tanımdan önce çağrılamaz).

```tr
TR_func topla(a, b = 1) { TR_return a + b }; TR_print(topla(2, 3), topla(5))
```

Görünümü:

```text
en: func topla(a, b = 1) { return a + b }; print(topla(2, 3), topla(5))
tr: fonksiyon topla(a, b = 1) { sonuç a + b }; yazdır(topla(2, 3), topla(5))
```

İlişkili: `TR_return`

#### `TR_if` (`TR_kw_if`, `eğer`)

Koşul doğruysa bloğu çalıştırır; eğer bir ifadedir, değeri çalışan bloğun son deyimidir (hiçbir dal çalışmazsa yok).

```tr
TR_var n = 7; TR_if n % 2 == 0 { TR_print("çift") } TR_else { TR_print("tek") }
```

Görünümü:

```text
en: var n = 7; if n % 2 == 0 { print("çift") } else { print("tek") }
tr: değişken n = 7; eğer n % 2 == 0 { yazdır("çift") } değilse { yazdır("tek") }
```

İlişkili: `TR_else`

#### `TR_import` (`TR_kw_import`, `aktar`)

Başka bir .tr dosyasını program boyunca bir kez çalıştırır ve üst düzey adlarını getirir; yol metin sabiti olmalıdır, .tr uzantısı isteğe bağlıdır.

```tr
TR_import "tests/yardimci_modul"; TR_print(kare(PI))
```

Görünümü:

```text
en: import "tests/yardimci_modul"; print(kare(PI))
tr: aktar "tests/yardimci_modul"; yazdır(kare(PI))
```

İlişkili: `TR_as`

#### `TR_in` (`TR_kw_in`, `içinde`)

döngüde ve liste üretecinde gezilecek kaynağı belirtir; iki değişkenli biçimde indeks ile eleman (sözlükte anahtar ile değer) verir.

```tr
TR_print([x * x TR_for x TR_in 1..=5 TR_if x % 2 == 1])
```

Görünümü:

```text
en: print([x * x for x in 1..=5 if x % 2 == 1])
tr: yazdır([x * x döngü x içinde 1..=5 eğer x % 2 == 1])
```

İlişkili: `TR_for`

#### `TR_interface` (`TR_kw_interface`, `arayüz`)

Gövdesiz metot imzalarından oluşan bir arayüz tanımlar; metotları aynı imzayla taşıyan yapı onu karşılar.

```tr
TR_interface Ölçen { TR_func ölç(x: TR_Float) -> TR_Float }; TR_struct Kare { TR_func ölç(x: TR_Float) -> TR_Float { TR_return x * x } }; TR_print(Ölçen, Kare().ölç(3.0))
```

Görünümü:

```text
en: interface Ölçen { func ölç(x: Float) -> Float }; struct Kare { func ölç(x: Float) -> Float { return x * x } }; print(Ölçen, Kare().ölç(3.0))
tr: arayüz Ölçen { fonksiyon ölç(x: Ondalık) -> Ondalık }; yapı Kare { fonksiyon ölç(x: Ondalık) -> Ondalık { sonuç x * x } }; yazdır(Ölçen, Kare().ölç(3.0))
```

İlişkili: `TR_struct`

#### `TR_match` (`TR_kw_match`, `eşle`)

Konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolun sonucunu verir; Aralık deseni içerme sınar, eşleşme yoksa yok döner.

```tr
TR_var puan = 95; TR_print(TR_match puan { 100 => "tam puan"; 90..100 => "pekiyi"; _ => "diğer" })
```

Görünümü:

```text
en: var puan = 95; print(match puan { 100 => "tam puan"; 90..100 => "pekiyi"; _ => "diğer" })
tr: değişken puan = 95; yazdır(eşle puan { 100 => "tam puan"; 90..100 => "pekiyi"; _ => "diğer" })
```

#### `TR_none` (`TR_kw_none`, `yok`)

Değerin yokluğudur; değer döndürmeyen fonksiyonlar ve varsa_al ile istenen olmayan öğe ya da anahtar yok verir. Sözlükte olmayan anahtarı köşeli parantezle okumak hatadır.

```tr
TR_var l = [1, 2]; TR_print(l.TR_get(5) == TR_none, l.TR_get(5) ?? "boş", {"a": 1}.TR_get("b"))
```

Görünümü:

```text
en: var l = [1, 2]; print(l.get(5) == none, l.get(5) ?? "boş", {"a": 1}.get("b"))
tr: değişken l = [1, 2]; yazdır(l.varsa_al(5) == yok, l.varsa_al(5) ?? "boş", {"a": 1}.varsa_al("b"))
```

İlişkili: `TR_NoneType`

#### `TR_not` (`TR_kw_not`, `değil`)

Mantık değerinin tersini verir; işleneni Mantık olmalıdır. Karşılaştırmadan gevşek bağlanır: değil 1 == 2, değil (1 == 2) demektir.

```tr
TR_print(TR_not 1 == 2)
```

Görünümü:

```text
en: print(not 1 == 2)
tr: yazdır(değil 1 == 2)
```

İlişkili: `TR_and`, `TR_or`

#### `TR_or` (`TR_kw_or`, `veya`)

Mantıksal veya; işlenenleri Mantık olmalıdır, kısa devre yapar. TR'de || yoktur: yazılırsa sözdizimi hatasıdır, ileti veya'yı önerir.

```tr
TR_print(TR_false TR_or TR_true, 1 > 2 TR_or 3 > 2)
```

Görünümü:

```text
en: print(false or true, 1 > 2 or 3 > 2)
tr: yazdır(yanlış veya doğru, 1 > 2 veya 3 > 2)
```

İlişkili: `TR_and`, `TR_not`

#### `TR_return` (`TR_kw_return`, `sonuç`)

Fonksiyondan değer döndürür; gövdenin son ifadesi dönüş değeri değildir, sonuç'suz biten fonksiyon yok döndürür; yalnız sonuç yok döndürür; programın en dışında yazılmaz (1.10), test gövdesinde de derleme hatasıdır.

```tr
TR_func işaret(n) { TR_if n < 0 { TR_return "negatif" }; TR_return "pozitif" }; TR_print(işaret(-2))
```

Görünümü:

```text
en: func işaret(n) { if n < 0 { return "negatif" }; return "pozitif" }; print(işaret(-2))
tr: fonksiyon işaret(n) { eğer n < 0 { sonuç "negatif" }; sonuç "pozitif" }; yazdır(işaret(-2))
```

İlişkili: `TR_func`

#### `TR_self` (`TR_kw_self`, `kendi`)

Yapı metodunun içinde metodun çağrıldığı nesneyi gösterir; alanlar kendi ile okunur ve değiştirilir.

```tr
TR_struct Sayaç { n = 0; TR_func artır() { TR_self.n += 1 } }; TR_var s = Sayaç(); s.artır(); s.artır(); TR_print(s.n)
```

Görünümü:

```text
en: struct Sayaç { n = 0; func artır() { self.n += 1 } }; var s = Sayaç(); s.artır(); s.artır(); print(s.n)
tr: yapı Sayaç { n = 0; fonksiyon artır() { kendi.n += 1 } }; değişken s = Sayaç(); s.artır(); s.artır(); yazdır(s.n)
```

İlişkili: `TR_struct`

#### `TR_struct` (`TR_kw_struct`, `yapı`)

Alanları ve metotları olan bir yapı tanımlar; nesne yapının adıyla, alan sırasıyla ya da adlı argümanlarla oluşturulur. Kalıtım yoktur.

```tr
TR_struct Nokta { x = 0, y = 0 }; TR_print(Nokta(3, 4), Nokta(y: 1).x)
```

Görünümü:

```text
en: struct Nokta { x = 0, y = 0 }; print(Nokta(3, 4), Nokta(y: 1).x)
tr: yapı Nokta { x = 0, y = 0 }; yazdır(Nokta(3, 4), Nokta(y: 1).x)
```

İlişkili: `TR_self`, `TR_init`

#### `TR_throw` (`TR_kw_throw`, `fırlat`)

Bir hata başlatır; verilen değer metne çevrilir, yakalayan değişken her zaman Metin'dir.

```tr
TR_try { TR_throw {"kod": 5} } TR_catch h { TR_print(TR_Type(h), h) }
```

Görünümü:

```text
en: try { throw {"kod": 5} } catch h { print(Type(h), h) }
tr: dene { fırlat {"kod": 5} } yakala h { yazdır(Tip(h), h) }
```

İlişkili: `TR_try`

#### `TR_true` (`TR_kw_true`, `doğru`)

Mantık tipinin doğru değeridir.

```tr
TR_var hazır = TR_true; TR_if hazır { TR_print("hazır") }
```

Görünümü:

```text
en: var hazır = true; if hazır { print("hazır") }
tr: değişken hazır = doğru; eğer hazır { yazdır("hazır") }
```

İlişkili: `TR_false`

#### `TR_try` (`TR_kw_try`, `dene`)

Bloktaki hatayı yakala bloğuna yönlendirir; çalışma zamanı hataları da yakalanır.

```tr
TR_try { TR_throw "patladı" } TR_catch h { TR_print("yakalandı:", h) }
```

Görünümü:

```text
en: try { throw "patladı" } catch h { print("yakalandı:", h) }
tr: dene { fırlat "patladı" } yakala h { yazdır("yakalandı:", h) }
```

İlişkili: `TR_catch`, `TR_throw`

#### `TR_var` (`TR_kw_var`, `değişken`)

Bulunulan kapsamda ilk değeriyle yeni bir ad tanımlar, tipi ilk değerden çıkar (ilk değersiz tanım ve := sözdizimi hatasıdır); virgülle birden çok ad tanımlanabilir.

```tr
TR_var a, b = 1, 2; a, b = b, a; TR_print(a, b)
```

Görünümü:

```text
en: var a, b = 1, 2; a, b = b, a; print(a, b)
tr: değişken a, b = 1, 2; a, b = b, a; yazdır(a, b)
```

İlişkili: `TR_const`

### Bağlamsal sözcükler

#### `TR_as` (`TR_ctx_as`, `olarak`)

aktar ile alınan modülün adlarını tek bir Sözlük olarak verilen ada bağlar.

```tr
TR_import "tests/yardimci_modul" TR_as ym; TR_print(ym.kare(2), ym["PI"])
```

Görünümü:

```text
en: import "tests/yardimci_modul" as ym; print(ym.kare(2), ym["PI"])
tr: aktar "tests/yardimci_modul" olarak ym; yazdır(ym.kare(2), ym["PI"])
```

İlişkili: `TR_import`

#### `TR_error` (`TR_ctx_error`, `hata`)

yakala bloğunda ad verilmediğinde hata mesajının bağlandığı değişkendir.

```tr
TR_try { TR_throw "patladı" } TR_catch { TR_print(TR_error) }
```

Görünümü:

```text
en: try { throw "patladı" } catch { print(error) }
tr: dene { fırlat "patladı" } yakala { yazdır(hata) }
```

İlişkili: `TR_catch`

#### `TR_init` (`TR_ctx_init`, `kurucu`)

Bu adlı metot nesne oluşturulurken aynı argümanlarla çağrılır; argümanlar kurucunun imzasına göre denetlenir.

```tr
TR_struct Hesap { bakiye; TR_func TR_init(b) { TR_self.bakiye = b * 2 } }; TR_print(Hesap(5).bakiye)
```

Görünümü:

```text
en: struct Hesap { bakiye; func init(b) { self.bakiye = b * 2 } }; print(Hesap(5).bakiye)
tr: yapı Hesap { bakiye; fonksiyon kurucu(b) { kendi.bakiye = b * 2 } }; yazdır(Hesap(5).bakiye)
```

İlişkili: `TR_struct`

#### `TR_test` (`TR_ctx_test`, `test`)

test "ad" { ... } bir test tanımlar; testler yalnızca tr test ile çalışır, normal çalıştırmada atlanır. Deyim başı dışında sıradan bir addır.

```tr
TR_test "toplama" { TR_assert_equal(4, 2 + 2) }
```

Görünümü:

```text
en: test "toplama" { assert_equal(4, 2 + 2) }
tr: test "toplama" { eşit_olmalı(4, 2 + 2) }
```

İlişkili: `TR_assert(...)`, `TR_assert_equal(...)`

### Değer adları

#### `TR_inf` (`TR_val_inf`, `inf`)

Ondalık sonsuz değerdir; metni her dilde inf, eksi sonsuzunki -inf'tir (3.2). Ondalık("inf") metinden okur.

```tr
TR_print(TR_inf, -TR_inf, TR_inf > 1e308)
```

Görünümü:

```text
en: print(inf, -inf, inf > 1e308)
tr: yazdır(inf, -inf, inf > 1e308)
```

İlişkili: `TR_nan`, `TR_Float`

#### `TR_nan` (`TR_val_nan`, `nan`)

Ondalık tanımsız değerdir (sayı olmayan); metni her dilde nan'dır (3.2) ve hiçbir sayıya, kendine de eşit değildir. Ondalık("nan") metinden okur.

```tr
TR_print(TR_nan, TR_nan == TR_nan)
```

Görünümü:

```text
en: print(nan, nan == nan)
tr: yazdır(nan, nan == nan)
```

İlişkili: `TR_inf`, `TR_Float`

### Tip adları

#### `TR_Any` (`TR_type_Any`, `Herhangi`)

Tip denetleyicisinin tipi bilinmeyen değerler için kullandığı tiptir; Herhangi açıklamalı bir ada her değer verilebilir.

```tr
TR_func kimlik(x: TR_Any) { TR_return x }; TR_print(kimlik(5), kimlik("beş"))
```

Görünümü:

```text
en: func kimlik(x: Any) { return x }; print(kimlik(5), kimlik("beş"))
tr: fonksiyon kimlik(x: Herhangi) { sonuç x }; yazdır(kimlik(5), kimlik("beş"))
```

#### `TR_Bool` (`TR_type_Bool`, `Mantık`)

doğru ve yanlış değerlerinin tipidir.

```tr
TR_var hazır: TR_Bool = TR_true; TR_print(TR_Type(hazır))
```

Görünümü:

```text
en: var hazır: Bool = true; print(Type(hazır))
tr: değişken hazır: Mantık = doğru; yazdır(Tip(hazır))
```

İlişkili: `TR_true`, `TR_false`

#### `TR_BuiltinFunction` (`TR_type_BuiltinFunction`, `YerleşikFonksiyon`)

Dilin yerleşik fonksiyonlarının tipidir.

```tr
TR_var f: TR_BuiltinFunction = TR_print; f(TR_Type(f))
```

Görünümü:

```text
en: var f: BuiltinFunction = print; f(Type(f))
tr: değişken f: YerleşikFonksiyon = yazdır; f(Tip(f))
```

İlişkili: `TR_Function`

#### `TR_Bytes` (`TR_type_Bytes`, `Baytlar`)

Değiştirilemez bayt dizisinin tipidir (3.3a): dosya, ağ, görüntü ve ses verisi. b[i] 0–255 arası bir Tam, b[a..c] yine Baytlar verir; Baytlar("ab") metnin UTF-8 baytlarını, Baytlar([0, 255]) bir tam sayı listesini kurar; Metin(b) UTF-8 okur. Metin ile bayt kendiliğinden dönüşmez.

```tr
TR_var b = TR_Bytes("şe"); TR_print(b, b.TR_length, b[0], TR_String(b + TR_Bytes([107])))
```

Görünümü:

```text
en: var b = Bytes("şe"); print(b, b.length, b[0], String(b + Bytes([107])))
tr: değişken b = Baytlar("şe"); yazdır(b, b.uzunluk, b[0], Metin(b + Baytlar([107])))
```

İlişkili: `TR_String`

#### `TR_CausalAttention` (`TR_type_CausalAttention`, `NedenselDikkat`)



#### `TR_Checkpoint` (`TR_type_Checkpoint`, `KontrolNoktası`)

Eğitimin bütün durumunu (modeller, iyileştiriciler, okuyucular, günlükler, düz değerli sözlük ve listeler; anahtarlar veridir) atomik kaydeden ve yeniden başlatılan programı kaldığı adımdan sürdüren nesnenin tipidir. KontrolNoktası(koşu, {...}) koşu klasöründe (noktalar <koşu>/checkpoints/ altına) kurulur; geçerli bir nokta varsa kurulurken yükler ve kaldığı adımı yazar, yoksa boş başlar. .güncelle(adım) adım sınırında aralıkla, .kaydet() hemen kaydeder.

```tr
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true); TR_var op = TR_SGD([w], TR_lr: 0.1); TR_var kn = TR_Checkpoint("koşu_t702", {"model": [w], "op": op}, TR_every_minutes: 15, TR_keep: 2); TR_print(kn.TR_resume_step, TR_Type(kn) == TR_Checkpoint)
```

Görünümü:

```text
en: var w = Tensor([1.0, 2.0], requires_grad: true); var op = SGD([w], lr: 0.1); var kn = Checkpoint("koşu_t702", {"model": [w], "op": op}, every_minutes: 15, keep: 2); print(kn.resume_step, Type(kn) == Checkpoint)
tr: değişken w = Tensör([1.0, 2.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.1); değişken kn = KontrolNoktası("koşu_t702", {"model": [w], "op": op}, her_dakika: 15, tut: 2); yazdır(kn.kaldığı_adım, Tip(kn) == KontrolNoktası)
```

Üyeleri: `.TR_resume_step`, `.TR_rollback(...)`, `.TR_save(...)`, `.TR_update(...)` · İlişkili: `.TR_update(...)`, `.TR_resume_step`, `.TR_save(...)`, `.TR_rollback(...)`, `TR_load_parameters(...)`, `.TR_state`

#### `TR_Dataset` (`TR_type_Dataset`, `VeriKümesi`)

Bir klasördeki .npy veri dosyalarından (ad sırasıyla) ya da tek bir .npy veri dosyasından oluşan veri kümesinin tipidir; VeriKümesi(yol) açar. Okuyucular .örnekleyici ve .sıralı ile alınır. önbellek: verilirse kota_gb: de verilmelidir.

```tr
TR_var küme = TR_Dataset("tests/data/numbers"); TR_print(küme, TR_Type(küme) == TR_Dataset)
```

Görünümü:

```text
en: var küme = Dataset("tests/data/numbers"); print(küme, Type(küme) == Dataset)
tr: değişken küme = VeriKümesi("tests/data/numbers"); yazdır(küme, Tip(küme) == VeriKümesi)
```

Üyeleri: `.TR_sampler(...)`, `.TR_sequential(...)` · İlişkili: `.TR_sampler(...)`, `.TR_sequential(...)`

#### `TR_Dict` (`TR_type_Dict`, `Sözlük`)

Eklenme sırasını koruyan anahtar–değer tablosu tipidir; anahtar köşeli parantezle okunur, olmayan anahtar hatadır, .varsa_al(k) yok verir (3.7).

```tr
TR_var s: TR_Dict = {"ad": "Ali"}; TR_print(TR_Type(s), s["ad"])
```

Görünümü:

```text
en: var s: Dict = {"ad": "Ali"}; print(Type(s), s["ad"])
tr: değişken s: Sözlük = {"ad": "Ali"}; yazdır(Tip(s), s["ad"])
```

Üyeleri: `.TR_contains(...)`, `.TR_get(...)`, `.TR_keys(...)`, `.TR_length`, `.TR_values(...)`

#### `TR_Embedding` (`TR_type_Embedding`, `GömmeKatmanı`)



#### `TR_Float` (`TR_type_Float`, `Ondalık`)

64 bit kayan noktalı sayı tipidir; Tam bir değer Ondalık açıklamalı bir ada da atanabilir. Ondalık(x) Tam'ı ya da metni Ondalık'a çevirir; metin programdaki yazımla okunur ("3.14", "2e-3", "inf"), yerel ayar yoktur.

```tr
TR_var oran: TR_Float = 0.75; TR_print(TR_Type(oran), TR_Float("3.14"), TR_Float(2))
```

Görünümü:

```text
en: var oran: Float = 0.75; print(Type(oran), Float("3.14"), Float(2))
tr: değişken oran: Ondalık = 0.75; yazdır(Tip(oran), Ondalık("3.14"), Ondalık(2))
```

İlişkili: `TR_Int`, `TR_String`

#### `TR_Function` (`TR_type_Function`, `Fonksiyon`)

Kullanıcı tanımlı fonksiyonların ve ok fonksiyonlarının tipidir; fonksiyonlar birinci sınıf değerdir.

```tr
TR_var f: TR_Function = (x) => x * 2; TR_print(TR_Type(f), f(3))
```

Görünümü:

```text
en: var f: Function = (x) => x * 2; print(Type(f), f(3))
tr: değişken f: Fonksiyon = (x) => x * 2; yazdır(Tip(f), f(3))
```

İlişkili: `TR_func`, `TR_BuiltinFunction`

#### `TR_GPT` (`TR_type_GPT`, `GPT`)



#### `TR_Int` (`TR_type_Int`, `Tam`)

64 bit işaretli tam sayı tipidir; sınırı aşan işlem sessizce sarmak yerine taşma hatası verir. Tam(x) değeri Tam'a çevirir (3.12): Ondalık'ın kesirli kısmını atar, metni programdaki yazımla okur ("42", "-1_000"); çevrilemeyen değer hatadır.

```tr
TR_var x: TR_Int = 5; TR_print(TR_Type(x), x * 2, TR_Int("42") + 1, TR_Int(3.9))
```

Görünümü:

```text
en: var x: Int = 5; print(Type(x), x * 2, Int("42") + 1, Int(3.9))
tr: değişken x: Tam = 5; yazdır(Tip(x), x * 2, Tam("42") + 1, Tam(3.9))
```

İlişkili: `TR_Float`, `TR_String`

#### `TR_JSON` (`TR_type_JSON`, `JSON`)

Yapısı önceden bilinmeyen verinin tipidir (3.7a): JSON'un her değerini tutar. j["k"] ve j[i] yine JSON verir, olmayan alan hatadır, .varsa_al(k) yok verir; Tip(j) değerin JSON türünü söyler, asıl değere tip çağrısıyla geçilir (Tam(j), Metin(j)).

```tr
TR_var j = TR_json_parse(`{"ad": "Ali", "yaş": 30}`); TR_print(TR_Type(j["yaş"]), TR_Int(j["yaş"]) + 1, j["ad"])
```

Görünümü:

```text
en: var j = json_parse(`{"ad": "Ali", "yaş": 30}`); print(Type(j["yaş"]), Int(j["yaş"]) + 1, j["ad"])
tr: değişken j = json_çöz(`{"ad": "Ali", "yaş": 30}`); yazdır(Tip(j["yaş"]), Tam(j["yaş"]) + 1, j["ad"])
```

İlişkili: `TR_json_parse(...)`, `TR_json_string(...)`

#### `TR_KVCache` (`TR_type_KVCache`, `KVÖnbellek`)



#### `TR_LineReader` (`TR_type_LineReader`, `SatırOkuyucu`)

dosya_satırları(...) ile açılan, dosyayı akışla satır satır okuyan okuyucunun tipidir; döngü ile gezilir ya da .sonraki() ile sıradaki satır alınır, dosya bitince yok verir; .kapat() dosyayı sonuna kadar okumadan bırakır.

```tr
TR_write_file("okuyucu.txt", "a\nb"); TR_var o = TR_read_lines("okuyucu.txt"); TR_print(TR_Type(o) == TR_LineReader, o.TR_next(), o.TR_next(), o.TR_next())
```

Görünümü:

```text
en: write_file("okuyucu.txt", "a\nb"); var o = read_lines("okuyucu.txt"); print(Type(o) == LineReader, o.next(), o.next(), o.next())
tr: dosya_yaz("okuyucu.txt", "a\nb"); değişken o = dosya_satırları("okuyucu.txt"); yazdır(Tip(o) == SatırOkuyucu, o.sonraki(), o.sonraki(), o.sonraki())
```

Üyeleri: `.TR_close(...)`, `.TR_next(...)` · İlişkili: `TR_read_lines(...)`, `.TR_next(...)`, `.TR_close(...)`

#### `TR_Linear` (`TR_type_Linear`, `Doğrusal`)



#### `TR_List` (`TR_type_List`, `Liste`)

Sıralı, karışık tipli, değiştirilebilir dizi tipidir; listeler başvuruyla paylaşılır.

```tr
TR_var l: TR_List = [1, "iki", 3.0]; TR_print(TR_Type(l), l[-1])
```

Görünümü:

```text
en: var l: List = [1, "iki", 3.0]; print(Type(l), l[-1])
tr: değişken l: Liste = [1, "iki", 3.0]; yazdır(Tip(l), l[-1])
```

Üyeleri: `.TR_append(...)`, `.TR_contains(...)`, `.TR_filter(...)`, `.TR_first`, `.TR_get(...)`, `.TR_join(...)`, `.TR_last`, `.TR_length`, `.TR_map(...)`, `.TR_reduce(...)`, `.TR_reverse(...)`, `.TR_sort(...)`, `.TR_unique(...)`

#### `TR_Logger` (`TR_type_Logger`, `Günlük`)

Her satırı bir JSON kaydı olan eğitim günlüğü dosyasının (.jsonl) tipidir; Günlük(yol) açar: dosya varsa sonuna eklenir, klasörü yoksa oluşturulur. İlk kayıttan önce program adı ve komut satırı ayarlarıyla bir başlangıç kaydı yazılır; .ekle(...) bir kayıt yazar.

```tr
TR_var g = TR_Logger("kayit/egitim.jsonl"); g.TR_append({"kayıp": 2.31, "doğruluk": 0.5}); TR_print(g, TR_Type(g) == TR_Logger)
```

Görünümü:

```text
en: var g = Logger("kayit/egitim.jsonl"); g.append({"kayıp": 2.31, "doğruluk": 0.5}); print(g, Type(g) == Logger)
tr: değişken g = Günlük("kayit/egitim.jsonl"); g.ekle({"kayıp": 2.31, "doğruluk": 0.5}); yazdır(g, Tip(g) == Günlük)
```

Üyeleri: `.TR_append(...)` · İlişkili: `.TR_append(...)`, `TR_Progress`

#### `TR_Message` (`TR_type_Message`, `İleti`)

Katalog iletisinin tipidir: İleti("H6903", ...) iletiyi kodu ve değerleriyle (katalogdaki parametre sırasıyla) kurar; metni gösterilirken iletinin dilinde kurulur. fırlat ve doğrula ona verilince hata o kodla fırlar. Kitaplık hata metnini böyle yazar.

```tr
TR_var i = TR_Message("H6903", -1); TR_print(TR_Type(i), i)
```

Görünümü:

```text
en: var i = Message("H6903", -1); print(Type(i), i)
tr: değişken i = İleti("H6903", -1); yazdır(Tip(i), i)
```

İlişkili: `TR_error_code(...)`, `TR_assert(...)`, `TR_throw`

#### `TR_Muon` (`TR_type_Muon`, `Muon`)



#### `TR_MuonAndAdamW` (`TR_type_MuonAndAdamW`, `MuonVeAdamW`)



#### `TR_NoneType` (`TR_type_NoneType`, `YokTipi`)

yok değerinin tipidir; Tip(yok) == YokTipi. Tip adı yalnız tip değeri olarak yazılır.

```tr
TR_print(TR_Type(TR_none), TR_Type(TR_none) == TR_NoneType)
```

Görünümü:

```text
en: print(Type(none), Type(none) == NoneType)
tr: yazdır(Tip(yok), Tip(yok) == YokTipi)
```

İlişkili: `TR_none`

#### `TR_NorMuon` (`TR_type_NorMuon`, `NorMuon`)



#### `TR_Object` (`TR_type_Object`, `Nesne`)

Yapıdan oluşturulan nesnelerin genel adıdır; Tip() bir nesne için Nesne değil yapının kendisini verir (Tip(p) == Nokta).

```tr
TR_struct Nokta { x, y }; TR_var p = Nokta(1, 2); TR_print(TR_Type(p) == Nokta, p)
```

Görünümü:

```text
en: struct Nokta { x, y }; var p = Nokta(1, 2); print(Type(p) == Nokta, p)
tr: yapı Nokta { x, y }; değişken p = Nokta(1, 2); yazdır(Tip(p) == Nokta, p)
```

İlişkili: `TR_StructType`

#### `TR_Optimizer` (`TR_type_Optimizer`, `İyileştirici`)

AdamW(...) ve SGD(...) ile oluşturulan iyileştiricilerin tipidir; .adım() parametreleri günceller, .sıfırla() gradyanları sıfırlar.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var op = TR_AdamW([w], TR_lr: 0.1); TR_print(TR_Type(op), TR_Type(TR_SGD([w])))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var op = AdamW([w], lr: 0.1); print(Type(op), Type(SGD([w])))
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken op = AdamW([w], öğrenme_oranı: 0.1); yazdır(Tip(op), Tip(SGD([w])))
```

Üyeleri: `.TR_load_state(...)`, `.TR_lr`, `.TR_state`, `.TR_step(...)`, `.TR_zero_grad(...)` · İlişkili: `TR_AdamW(...)`, `TR_SGD(...)`

#### `TR_PLEBlock` (`TR_type_PLEBlock`, `PLEBloğu`)



#### `TR_PLEContext` (`TR_type_PLEContext`, `PLEBağlamı`)



#### `TR_Path` (`TR_type_Path`, `Yol`)

Dosya ya da klasör yolunun tipidir; değiştirilemez, ayırıcıyı / ile saklar ve gösterir. Yol(...) parçaları / ile birleştirip temizlenmiş bir yol kurar (a/./b/../c → a/c); ilk parçadan sonra mutlak bir parça, boş parça ya da sekme/satır sonu içeren parça hatadır; göreli yol, kullanıldığı anda dosya_oku ile aynı kuralla çözülür. Özellikleri yola soru sorar (dosya_adı, uzantı, var_mı, bayt ...); yol alan her yerleşik Metin yerine Yol da kabul eder.

```tr
TR_var y = TR_Path("veri", "./eğitim/../derlem.txt"); TR_print(TR_Type(y) == TR_Path, y)
```

Görünümü:

```text
en: var y = Path("veri", "./eğitim/../derlem.txt"); print(Type(y) == Path, y)
tr: değişken y = Yol("veri", "./eğitim/../derlem.txt"); yazdır(Tip(y) == Yol, y)
```

Üyeleri: `.TR_absolute`, `.TR_exists`, `.TR_extension`, `.TR_is_dir`, `.TR_is_file`, `.TR_name`, `.TR_parent`, `.TR_size`, `.TR_stem` · İlişkili: `TR_list_dir(...)`

#### `TR_Progress` (`TR_type_Progress`, `İlerleme`)

İlerleme çubuğunun tipidir; İlerleme(toplam) toplam adımla açar (toplam verilmezse kalan süresiz sayaç), .güncelle(...) bir adım ilerletir, .durdur() çubuğu bitirir. Çubuk standart hataya yazılır: terminalde tek satırda yenilenir, dosyaya ya da boruya giderken 10 saniyede bir düz satır yazılır; günlük: verilirse her güncelleme o günlüğe de yazılır.

```tr
TR_var ç = TR_Progress(3); TR_for i TR_in 0..3 { ç.TR_update({"kayıp": 1.0 / (i + 1)}) }; TR_print(ç, TR_Type(ç) == TR_Progress)
```

Görünümü:

```text
en: var ç = Progress(3); for i in 0..3 { ç.update({"kayıp": 1.0 / (i + 1)}) }; print(ç, Type(ç) == Progress)
tr: değişken ç = İlerleme(3); döngü i içinde 0..3 { ç.güncelle({"kayıp": 1.0 / (i + 1)}) }; yazdır(ç, Tip(ç) == İlerleme)
```

Üyeleri: `.TR_stop(...)`, `.TR_update(...)` · İlişkili: `.TR_update(...)`, `.TR_stop(...)`, `TR_Logger`

#### `TR_RMSNorm` (`TR_type_RMSNorm`, `RMSNormKatmanı`)



#### `TR_Range` (`TR_type_Range`, `Aralık`)

a..b (b hariç) ve a..=b (b dahil) aralıklarının tipidir; uçlar Tam olmalıdır. Aralığın sayılarının listesi Liste(a..b) ile kurulur.

```tr
TR_var a: TR_Range = 1..5; TR_print(TR_Type(a), TR_List(0..5))
```

Görünümü:

```text
en: var a: Range = 1..5; print(Type(a), List(0..5))
tr: değişken a: Aralık = 1..5; yazdır(Tip(a), Liste(0..5))
```

Üyeleri: `.TR_reverse(...)`

#### `TR_RowRMS` (`TR_type_RowRMS`, `SatırRMS`)



#### `TR_Run` (`TR_type_Run`, `Koşu`)



#### `TR_Sampler` (`TR_type_Sampler`, `Örnekleyici`)

Veri kümesinden tohuma göre rastgele [girdi, hedef] tensör çiftleri üreten okuyucunun tipidir; dönemler boyunca sonsuz üretir.

```tr
TR_var ö = TR_Dataset("tests/data/numbers").TR_sampler(TR_batch_size: 2, TR_context: 4, TR_seed: 1); TR_print(TR_Type(ö) == TR_Sampler)
```

Görünümü:

```text
en: var ö = Dataset("tests/data/numbers").sampler(batch_size: 2, context: 4, seed: 1); print(Type(ö) == Sampler)
tr: değişken ö = VeriKümesi("tests/data/numbers").örnekleyici(toplu: 2, bağlam: 4, tohum: 1); yazdır(Tip(ö) == Örnekleyici)
```

Üyeleri: `.TR_load_state(...)`, `.TR_next(...)`, `.TR_skip(...)`, `.TR_state` · İlişkili: `.TR_sampler(...)`, `TR_SequentialReader`

#### `TR_SequentialReader` (`TR_type_SequentialReader`, `SıralıOkuyucu`)

Veri kümesini pencereler hâlinde sırayla okuyan okuyucunun tipidir; veri bitince .sonraki() yok verir.

```tr
TR_var s = TR_Dataset("tests/data/numbers").TR_sequential(TR_batch_size: 2, TR_context: 4); TR_print(TR_Type(s) == TR_SequentialReader)
```

Görünümü:

```text
en: var s = Dataset("tests/data/numbers").sequential(batch_size: 2, context: 4); print(Type(s) == SequentialReader)
tr: değişken s = VeriKümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4); yazdır(Tip(s) == SıralıOkuyucu)
```

Üyeleri: `.TR_load_state(...)`, `.TR_next(...)`, `.TR_skip(...)`, `.TR_state` · İlişkili: `.TR_sequential(...)`, `TR_Sampler`

#### `TR_SpikeGuard` (`TR_type_SpikeGuard`, `SıçramaBekçisi`)



#### `TR_String` (`TR_type_String`, `Metin`)

Değiştirilemez Unicode metin tipidir; uzunluk ve indeks karakter bazındadır. Metin(x) her değeri yazdırıldığı biçimiyle Metin'e çevirir; bayt dizisini UTF-8 olarak okur.

```tr
TR_var ad: TR_String = "şeker"; TR_print(TR_Type(ad), ad[-1], TR_String(12) + "a", TR_String([1, 2]))
```

Görünümü:

```text
en: var ad: String = "şeker"; print(Type(ad), ad[-1], String(12) + "a", String([1, 2]))
tr: değişken ad: Metin = "şeker"; yazdır(Tip(ad), ad[-1], Metin(12) + "a", Metin([1, 2]))
```

Üyeleri: `.TR_contains(...)`, `.TR_ends_with(...)`, `.TR_get(...)`, `.TR_length`, `.TR_lower(...)`, `.TR_replace(...)`, `.TR_reverse(...)`, `.TR_split(...)`, `.TR_starts_with(...)`, `.TR_strip(...)`, `.TR_upper(...)` · İlişkili: `TR_Int`, `TR_Float`

#### `TR_StructType` (`TR_type_StructType`, `YapıTanımı`)

yapı ile tanımlanan yapının kendisinin tipidir; tip açıklamasında kullanılamaz.

```tr
TR_struct Nokta { x, y }; TR_print(TR_Type(Nokta) == TR_StructType)
```

Görünümü:

```text
en: struct Nokta { x, y }; print(Type(Nokta) == StructType)
tr: yapı Nokta { x, y }; yazdır(Tip(Nokta) == YapıTanımı)
```

İlişkili: `TR_struct`, `TR_Object`

#### `TR_SwiGLU` (`TR_type_SwiGLU`, `SwiGLU`)



#### `TR_Tensor` (`TR_type_Tensor`, `Tensör`)

N boyutlu sayı dizisi tipidir (matris 2 boyutlu tensördür); elemanlar varsayılan olarak float32'dir. Tensör(x) iç içe sayı listesinden, tek sayıdan ya da başka bir tensörden yeni bir yaprak tensör kurar; türev_izle varsayılan olarak yanlıştır; tür verilmezse listeden float32 olur, tensörden kaynağın türü korunur (tensörün türünü değiştirmek için tür: değil .türe kullanılır).

```tr
TR_var t: TR_Tensor = TR_Tensor([[1, 2], [3, 4]], TR_requires_grad: TR_true); TR_print(TR_Type(t), t.TR_shape, t.TR_dtype)
```

Görünümü:

```text
en: var t: Tensor = Tensor([[1, 2], [3, 4]], requires_grad: true); print(Type(t), t.shape, t.dtype)
tr: değişken t: Tensör = Tensör([[1, 2], [3, 4]], türev_izle: doğru); yazdır(Tip(t), t.şekil, t.tür)
```

Üyeleri: `.TR_device`, `.TR_dtype`, `.TR_grad`, `.TR_permute(...)`, `.TR_reshape(...)`, `.TR_shape`, `.TR_slice(...)`, `.TR_split(...)`, `.TR_to_device(...)`, `.TR_to_dtype(...)`, `.TR_tolist`, `.TR_transpose(...)` · İlişkili: `TR_zeros(...)`, `TR_ones(...)`, `TR_randn(...)`

#### `TR_Tokenizer` (`TR_type_Tokenizer`, `Parçalayıcı`)

parçalayıcı_yükle(...) ile yüklenen, metni parçalara ayırıp parça kimliklerine çeviren nesnenin tipidir.

```tr
TR_var p = TR_load_tokenizer("tests/data/tokenizer.json"); TR_print(TR_Type(p) == TR_Tokenizer)
```

Görünümü:

```text
en: var p = load_tokenizer("tests/data/tokenizer.json"); print(Type(p) == Tokenizer)
tr: değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(Tip(p) == Parçalayıcı)
```

Üyeleri: `.TR_decode(...)`, `.TR_encode(...)`, `.TR_token_id(...)`, `.TR_vocab_size` · İlişkili: `TR_load_tokenizer(...)`

#### `TR_TransformerBlock` (`TR_type_TransformerBlock`, `TransformerBloğu`)



#### `TR_Type` (`TR_type_Type`, `Tip`)

Tip değerlerinin tipidir: Tip(x) değerin tipini verir, tip adları (Tam, Liste, Tensör ...) ifadede tip değeridir. Karşılaştırma değerle yapılır: Tip(x) == Tam; tip değeri metinle karşılaştırılamaz. Bir yapı nesnesi için yapının kendisini verir (Tip(p) == Nokta).

```tr
TR_var t: TR_Type = TR_Type(42); TR_print(t == TR_Int, t, TR_Type(t), TR_Type([1, 2]) == TR_List)
```

Görünümü:

```text
en: var t: Type = Type(42); print(t == Int, t, Type(t), Type([1, 2]) == List)
tr: değişken t: Tip = Tip(42); yazdır(t == Tam, t, Tip(t), Tip([1, 2]) == Liste)
```

İlişkili: `TR_Int`, `TR_Float`, `TR_String`

## Yerleşikler

### Yerleşik fonksiyonlar

#### `TR_AdamW(...)` (`TR_fn_AdamW`, `AdamW`)

    AdamW(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.001, ağırlık_çürümesi: Tam | Ondalık = 0.01, beta1: Tam | Ondalık = 0.9, beta2: Tam | Ondalık = 0.999, epsilon: Tam | Ondalık = 1e-08, ana_kopya: Mantık = doğru) → İyileştirici

Türev izleyen yaprak tensörler için ağırlık çürümesi ayrık uygulanan Adam iyileştiricisini kurar; .adım() parametreleri günceller, .sıfırla() gradyanları sıfırlar. Varsayılanlar: öğrenme_oranı 0.001, ağırlık_çürümesi 0.01, beta1 0.9, beta2 0.999, epsilon 1e-8.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var opt = TR_AdamW([w], TR_lr: 0.1); TR_backward(TR_sum(w * w)); opt.TR_step(); TR_print(w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var opt = AdamW([w], lr: 0.1); backward(sum(w * w)); opt.step(); print(w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken opt = AdamW([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: `TR_SGD(...)`, `TR_clip_grad(...)`, `.TR_step(...)`, `.TR_zero_grad(...)`, `TR_Optimizer`

#### `TR_SGD(...)` (`TR_fn_SGD`, `SGD`)

    SGD(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.01, momentum: Tam | Ondalık = 0, ağırlık_çürümesi: Tam | Ondalık = 0, ana_kopya: Mantık = doğru) → İyileştirici

Türev izleyen yaprak tensörler için olasılıksal gradyan inişi iyileştiricisini kurar; .adım() ve .sıfırla() metotları vardır. Varsayılanlar: öğrenme_oranı 0.01, momentum 0, ağırlık_çürümesi 0.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var opt = TR_SGD([w], TR_lr: 0.1, TR_momentum: 0.9); TR_backward(TR_sum(w * w)); opt.TR_step(); TR_print(w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var opt = SGD([w], lr: 0.1, momentum: 0.9); backward(sum(w * w)); opt.step(); print(w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.1, momentum: 0.9); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: `TR_AdamW(...)`, `.TR_step(...)`, `.TR_zero_grad(...)`, `TR_Optimizer`

#### `TR_abs(...)` (`TR_fn_abs`, `mutlak`)

    mutlak(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayının mutlak değerini verir; Tam Tam, Ondalık Ondalık kalır, en küçük Tam değerde taşma hatası verir. Tensörde eleman bazlı çalışır (−0 sıfır olur, NaN korunur, 16 bitlik tensör kendi türünde döner); türevi işaret(x)'tir, 0'da 0. Listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_abs(-5), TR_abs(-2.5), TR_abs(TR_Tensor([-2.0, 0.0, 3.0])).TR_tolist)
```

Görünümü:

```text
en: print(abs(-5), abs(-2.5), abs(Tensor([-2.0, 0.0, 3.0])).tolist)
tr: yazdır(mutlak(-5), mutlak(-2.5), mutlak(Tensör([-2.0, 0.0, 3.0])).değer)
```

İlişkili: `TR_round(...)`, `TR_sign(...)`

#### `TR_acos(...)` (`TR_fn_acos`, `acos`)

    acos(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arkkosinüsünü radyan olarak [0, π] aralığında verir; [-1, 1] dışında NaN döner. Türevi -1 / karekök(1 - x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_acos(0.0), TR_acos(TR_Tensor([1.0, -1.0])).TR_tolist)
```

Görünümü:

```text
en: print(acos(0.0), acos(Tensor([1.0, -1.0])).tolist)
tr: yazdır(acos(0.0), acos(Tensör([1.0, -1.0])).değer)
```

İlişkili: `TR_asin(...)`, `TR_cos(...)`

#### `TR_any_pattern_matches(...)` (`TR_fn_any_pattern_matches`, `bir_desen_uyar_mı`)

    bir_desen_uyar_mı()



#### `TR_apply_rotary(...)` (`TR_fn_apply_rotary`, `döner_konum_uygula`)

    döner_konum_uygula()



#### `TR_apply_rotary_fused(...)` (`TR_fn_apply_rotary_fused`, `döner_konum_uygula_kaynaşık`)

    döner_konum_uygula_kaynaşık()



#### `TR_argmax(...)` (`TR_fn_argmax`, `argmax`)

    argmax(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) en büyük elemanın indeksini verir; eşitlikte ilki seçilir. Sonuç tam sayılı ve türev izlemeyen bir tensördür; eksen yalnızca adlı verilir.

```tr
TR_print(TR_argmax(TR_Tensor([[0.1, 2.5, 0.3], [1.7, 0.2, 0.9]]), TR_dim: -1).TR_tolist)
```

Görünümü:

```text
en: print(argmax(Tensor([[0.1, 2.5, 0.3], [1.7, 0.2, 0.9]]), dim: -1).tolist)
tr: yazdır(argmax(Tensör([[0.1, 2.5, 0.3], [1.7, 0.2, 0.9]]), eksen: -1).değer)
```

İlişkili: `TR_sample(...)`

#### `TR_argsort(...)` (`TR_fn_argsort`, `sıralama_indeksi`)

    sıralama_indeksi(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) elemanları küçükten büyüğe sıralayan indeksleri verir; sıralama kararlıdır (eşitlerde küçük indeks önce), NaN'lar sondadır. Sonuç tam sayılı ve türevsiz bir tensördür.

```tr
TR_print(TR_argsort(TR_Tensor([3.0, 1.0, 2.0])).TR_tolist)
```

Görünümü:

```text
en: print(argsort(Tensor([3.0, 1.0, 2.0])).tolist)
tr: yazdır(sıralama_indeksi(Tensör([3.0, 1.0, 2.0])).değer)
```

İlişkili: `TR_topk(...)`, `.TR_sort(...)`

#### `TR_asin(...)` (`TR_fn_asin`, `asin`)

    asin(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arksinüsünü radyan olarak [-π/2, π/2] aralığında verir; [-1, 1] dışında NaN döner. Türevi 1 / karekök(1 - x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_asin(1.0), TR_asin(TR_Tensor([0.0, 0.5])).TR_tolist)
```

Görünümü:

```text
en: print(asin(1.0), asin(Tensor([0.0, 0.5])).tolist)
tr: yazdır(asin(1.0), asin(Tensör([0.0, 0.5])).değer)
```

İlişkili: `TR_acos(...)`, `TR_sin(...)`

#### `TR_assert(...)` (`TR_fn_assert`, `doğrula`)

    doğrula(Herhangi, [Herhangi]) → Mantık

Koşul yanlışsa "doğrulama başarısız" hatası fırlatır, doğruysa doğru döndürür; isteğe bağlı mesaj hataya eklenir. Test dışında da çalışır.

```tr
TR_assert(2 > 1, "iki birden büyük olmalı"); TR_print("doğrulandı")
```

Görünümü:

```text
en: assert(2 > 1, "iki birden büyük olmalı"); print("doğrulandı")
tr: doğrula(2 > 1, "iki birden büyük olmalı"); yazdır("doğrulandı")
```

İlişkili: `TR_assert_equal(...)`, `TR_test`

#### `TR_assert_equal(...)` (`TR_fn_assert_equal`, `eşit_olmalı`)

    eşit_olmalı(Herhangi, Herhangi, [Herhangi]) → Mantık

İki değer == anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır, eşitse doğru döndürür. Test dışında da çalışır.

```tr
TR_assert_equal(4, 2 + 2); TR_print("eşit")
```

Görünümü:

```text
en: assert_equal(4, 2 + 2); print("eşit")
tr: eşit_olmalı(4, 2 + 2); yazdır("eşit")
```

İlişkili: `TR_assert(...)`, `TR_test`

#### `TR_atan(...)` (`TR_fn_atan`, `atan`)

    atan(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arktanjantını radyan olarak (-π/2, π/2) aralığında verir; türevi 1 / (1 + x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_atan(1.0), TR_atan(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(atan(1.0), atan(Tensor([0.0, 1.0])).tolist)
tr: yazdır(atan(1.0), atan(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_atan2(...)`, `TR_tan(...)`

#### `TR_atan2(...)` (`TR_fn_atan2`, `atan2`)

    atan2(Tensör | Tam | Ondalık, Tensör | Tam | Ondalık) → Tensör | Ondalık

atan2(y, x), (x, y) noktasının açısını (-π, π] aralığında radyan olarak verir; dört bölgeyi ayırır, (0, 0)'da 0 döner. Sayıda Ondalık, tensörde yayınlamalı ve türev izleyen tensör verir; (0, 0)'da türev 0 alınır. Listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_atan2(1.0, -1.0), TR_atan2(TR_Tensor([1.0, -1.0]), TR_Tensor([0.0])).TR_tolist)
```

Görünümü:

```text
en: print(atan2(1.0, -1.0), atan2(Tensor([1.0, -1.0]), Tensor([0.0])).tolist)
tr: yazdır(atan2(1.0, -1.0), atan2(Tensör([1.0, -1.0]), Tensör([0.0])).değer)
```

İlişkili: `TR_atan(...)`

#### `TR_attention(...)` (`TR_fn_attention`, `dikkat`)

    dikkat(Tensör, Tensör, Tensör, nedensel: Mantık = doğru, pencere: Tam = 0, maske: Tensör | Liste | YokTipi = yok) → Tensör

Birleşik ölçekli nokta çarpımı dikkatini (softmax(q @ kᵀ / √dh + maske) @ v) bellek dostu, tek türev düğümüyle hesaplar. Varsayılan olarak nedenseldir; k/v bakış sayısı q'nunkinin böleni olabilir (gruplu sorgu dikkati; bakış, İngilizce head).

```tr
TR_var q = TR_randn([2, 4, 8], TR_seed: 1); TR_print(TR_attention(q, q, q, TR_causal: TR_true).TR_shape)
```

Görünümü:

```text
en: var q = randn([2, 4, 8], seed: 1); print(attention(q, q, q, causal: true).shape)
tr: değişken q = rastgele_normal([2, 4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: doğru).şekil)
```

İlişkili: `TR_softmax(...)`

#### `TR_backward(...)` (`TR_fn_backward`, `geri_yayılım`)

    geri_yayılım(Tensör) → Mantık

Tek elemanlı (skaler) bir kayıptan geri yayılım yaparak yaprak tensörlerin .gradyan değerlerini hesaplar. Gradyanlar birikir; adımlar arasında iyileştiricinin .sıfırla() metoduyla sıfırlanmalıdır.

```tr
TR_var x = TR_Tensor(3.0, TR_requires_grad: TR_true); TR_backward(x * x); TR_print(x.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var x = Tensor(3.0, requires_grad: true); backward(x * x); print(x.grad.tolist)
tr: değişken x = Tensör(3.0, türev_izle: doğru); geri_yayılım(x * x); yazdır(x.gradyan.değer)
```

İlişkili: `TR_no_grad(...)`, `.TR_grad`

#### `TR_build_pretrained_model(...)` (`TR_fn_build_pretrained_model`, `hazır_model_kur`)

    hazır_model_kur()



#### `TR_ceil(...)` (`TR_fn_ceil`, `tavan`)

    tavan(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayıyı yukarı (artı sonsuza doğru) en yakın tam sayıya yuvarlar; Tam girdide Tam, Ondalık girdide Ondalık döner. Tensörde eleman bazlı çalışır (tam sayılı tensör tam sayılı kalır), türevi 0'dır; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_ceil(3.2), TR_ceil(7), TR_ceil(TR_Tensor([-1.5, 2.5])).TR_tolist)
```

Görünümü:

```text
en: print(ceil(3.2), ceil(7), ceil(Tensor([-1.5, 2.5])).tolist)
tr: yazdır(tavan(3.2), tavan(7), tavan(Tensör([-1.5, 2.5])).değer)
```

İlişkili: `TR_floor(...)`, `TR_round(...)`

#### `TR_clamp(...)` (`TR_fn_clamp`, `sınırla`)

    sınırla(Tam | Ondalık | Tensör, en_az: Tam | Ondalık = yok, en_fazla: Tam | Ondalık = yok) → Tam | Ondalık | Tensör

Değeri en_az ile en_fazla arasına sınırlar; ikisinden en az biri verilmeli, en_az en_fazla'dan büyük olamaz. Sayıda tip korunur (sınırlardan biri Ondalık ise Ondalık), NaN korunur. Tensörde eleman bazlıdır; türev sınırların içinde (sınırlar dahil) 1, dışında 0'dır; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_clamp(15, TR_max: 10), TR_clamp(TR_Tensor([-2.0, 0.5, 3.0]), TR_min: 0, TR_max: 1).TR_tolist)
```

Görünümü:

```text
en: print(clamp(15, max: 10), clamp(Tensor([-2.0, 0.5, 3.0]), min: 0, max: 1).tolist)
tr: yazdır(sınırla(15, en_fazla: 10), sınırla(Tensör([-2.0, 0.5, 3.0]), en_az: 0, en_fazla: 1).değer)
```

İlişkili: `TR_min(...)`, `TR_max(...)`

#### `TR_clip_grad(...)` (`TR_fn_clip_grad`, `gradyan_kırp`)

    gradyan_kırp(Liste | Tensör, en_fazla: Tam | Ondalık = 1, sonlu_değilse_hata: Mantık = yanlış) → Tensör

Parametre gradyanlarının toplam L2 normu en_fazla değerini (varsayılan 1.0) aşıyorsa hepsini aynı oranda küçültür. Kırpma öncesi normu 0 boyutlu tensör olarak döndürür (kayıp gibi): GPU'da norm ve kırpma cihazda yapılır, program norm okununca (.değer, yazdırma) bekler.

```tr
TR_var w = TR_Tensor([3.0, 4.0], TR_requires_grad: TR_true); TR_backward(TR_sum(w * w)); TR_print(TR_clip_grad([w], TR_max: 1.0))
```

Görünümü:

```text
en: var w = Tensor([3.0, 4.0], requires_grad: true); backward(sum(w * w)); print(clip_grad([w], max: 1.0))
tr: değişken w = Tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 1.0))
```

İlişkili: `TR_AdamW(...)`, `TR_SGD(...)`

#### `TR_command_line(...)` (`TR_fn_command_line`, `komut_satırı`)

    komut_satırı(Sözlük, açıklama: Metin = "", açıklamalar: Sözlük) → Sözlük

Betiğin seçeneklerini varsayılanlarıyla tanımlar ve komut satırında verilen --ad değer argümanlarıyla doldurulmuş yeni bir Sözlük döndürür; seçeneğin tipi varsayılanından çıkar. --help seçenekleri listeler; hatalı argüman programı 2 koduyla bitirir. Programda bir kez çağrılır; argüman yoksa varsayılanlar döner.

```tr
TR_var ayar = TR_command_line({"adım": 100, "oran": 0.01, "hızlı": TR_false}, TR_description: "Küçük modeli eğitir"); TR_print(ayar["adım"], ayar["oran"], ayar["hızlı"])
```

Görünümü:

```text
en: var ayar = command_line({"adım": 100, "oran": 0.01, "hızlı": false}, description: "Küçük modeli eğitir"); print(ayar["adım"], ayar["oran"], ayar["hızlı"])
tr: değişken ayar = komut_satırı({"adım": 100, "oran": 0.01, "hızlı": yanlış}, açıklama: "Küçük modeli eğitir"); yazdır(ayar["adım"], ayar["oran"], ayar["hızlı"])
```

İlişkili: `TR_env(...)`

#### `TR_compute_dtype(...)` (`TR_fn_compute_dtype`, `hesap_türü`)

    hesap_türü() → Metin

Etkin karışık_hassasiyet bloğunun hesap türünün adını verir ("bf16", "fp16"); blok dışında "float32". Çıktısını yalnızca çarpımların okuyacağı bir işlem sonucunu doğrudan o türde yazmak için kullanılır (ör. rms_normu(x, w, tür: hesap_türü())).

```tr
TR_print(TR_compute_dtype(), TR_mixed_precision("bf16") { TR_compute_dtype() })
```

Görünümü:

```text
en: print(compute_dtype(), mixed_precision("bf16") { compute_dtype() })
tr: yazdır(hesap_türü(), karışık_hassasiyet("bf16") { hesap_türü() })
```

İlişkili: `TR_mixed_precision(...)`, `TR_rms_norm(...)`, `TR_dtype:`

#### `TR_concat(...)` (`TR_fn_concat`, `bitiştir`)

    bitiştir(Liste, [Tam], eksen: Tam = -1) → Tensör

Bir tensör listesini verilen eksen boyunca (varsayılan son eksen) uç uca ekleyip tek tensör yapar. Birleşen eksen dışındaki eksenler eşit olmalıdır; sonuç türev izler.

```tr
TR_print(TR_concat([TR_ones([2, 1]), TR_zeros([2, 2])], TR_dim: -1).TR_shape)
```

Görünümü:

```text
en: print(concat([ones([2, 1]), zeros([2, 2])], dim: -1).shape)
tr: yazdır(bitiştir([birler([2, 1]), sıfırlar([2, 2])], eksen: -1).şekil)
```

İlişkili: `.TR_split(...)`

#### `TR_config_value(...)` (`TR_fn_config_value`, `ayar_değeri`)

    ayar_değeri()



#### `TR_copy(...)` (`TR_fn_copy`, `kopyala`)

    kopyala(Metin | Yol, Metin | Yol, üzerine_yaz: Mantık = yanlış) → Yol

Dosyayı ya da klasörü (içindekilerle) kopyalar ve hedefin yolunu döndürür. Hedef var olan bir klasörse kaynak onun içine aynı adla gider; hedef zaten varsa üzerine_yaz: doğru gerekir. Değiştirilme zamanı korunur, bağlantılar kopyalanmaz (atlanır, uyarı yazılır); kopya önce geçici bir ada yazılıp yerine konur.

```tr
TR_write_file("ağırlık.txt", "w"); TR_make_dir("yedek"); TR_print(TR_copy("ağırlık.txt", "yedek"))
```

Görünümü:

```text
en: write_file("ağırlık.txt", "w"); make_dir("yedek"); print(copy("ağırlık.txt", "yedek"))
tr: dosya_yaz("ağırlık.txt", "w"); klasör_oluştur("yedek"); yazdır(kopyala("ağırlık.txt", "yedek"))
```

İlişkili: `TR_move(...)`

#### `TR_cos(...)` (`TR_fn_cos`, `cos`)

    cos(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın kosinüsünü (radyan) hesaplar; türevi -sin(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_cos(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(cos(Tensor([0.0, 1.0])).tolist)
tr: yazdır(cos(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_sin(...)`

#### `TR_cosh(...)` (`TR_fn_cosh`, `cosh`)

    cosh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik kosinüstür: (e^x + e^(-x)) / 2; türevi sinh(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_cosh(0.0), TR_cosh(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(cosh(0.0), cosh(Tensor([0.0, 1.0])).tolist)
tr: yazdır(cosh(0.0), cosh(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_sinh(...)`, `TR_tanh(...)`

#### `TR_cross_entropy(...)` (`TR_fn_cross_entropy`, `çapraz_entropi`)

    çapraz_entropi(Tensör | Liste, Tam | Liste | Tensör) → Tensör

Son ekseni sınıf olan çıktıyla satır başına bir sınıf indeksi arasındaki ortalama çapraz entropi kaybını skaler tensör olarak verir. Negatif hedefler yok sayılır; hepsi yok sayılırsa hata verir.

```tr
TR_print(TR_cross_entropy(TR_Tensor([[2.0, 0.5, 0.1], [0.2, 1.5, 0.3]]), [0, 1]).TR_tolist)
```

Görünümü:

```text
en: print(cross_entropy(Tensor([[2.0, 0.5, 0.1], [0.2, 1.5, 0.3]]), [0, 1]).tolist)
tr: yazdır(çapraz_entropi(Tensör([[2.0, 0.5, 0.1], [0.2, 1.5, 0.3]]), [0, 1]).değer)
```

İlişkili: `TR_log_softmax(...)`

#### `TR_cumsum(...)` (`TR_fn_cumsum`, `birikimli_toplam`)

    birikimli_toplam(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) her konuma kadar olan elemanların toplamını verir; şekil korunur, türev izler (gradyan sondan birikimli toplamdır), toplama sırası sabittir. Yalnızca tensörde çalışır, listede çalışmaz (Tensör(...) ile çevrilir); eksen yalnızca adlı verilir.

```tr
TR_print(TR_cumsum(TR_Tensor([1, 2, 3, 4])).TR_tolist, TR_cumsum(TR_Tensor([[1, 2], [3, 4]]), TR_dim: 0).TR_tolist)
```

Görünümü:

```text
en: print(cumsum(Tensor([1, 2, 3, 4])).tolist, cumsum(Tensor([[1, 2], [3, 4]]), dim: 0).tolist)
tr: yazdır(birikimli_toplam(Tensör([1, 2, 3, 4])).değer, birikimli_toplam(Tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

İlişkili: `TR_sum(...)`

#### `TR_custom_grad(...)` (`TR_fn_custom_grad`, `özel_türev`)

    özel_türev(ileri: Fonksiyon | YerleşikFonksiyon, geri: Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

İleri ve geri hesabı kullanıcının yazdığı yeni bir türevlenebilir işlem kurar ve çağrılabilir bir fonksiyon döndürür. Yalnızca ileri: ve geri: adlı argümanlarını alır; ileri [çıktı, saklananlar], geri her konumlu girdi için bir gradyan (ya da yok) döndürür.

```tr
TR_var kare = TR_custom_grad(TR_forward: (x) => [x * x, [x]], TR_backward: (s, g) => [g * 2 * s[0]]); TR_var x = TR_Tensor([3.0], TR_requires_grad: TR_true); TR_backward(TR_sum(kare(x))); TR_print(x.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var kare = custom_grad(forward: (x) => [x * x, [x]], backward: (s, g) => [g * 2 * s[0]]); var x = Tensor([3.0], requires_grad: true); backward(sum(kare(x))); print(x.grad.tolist)
tr: değişken kare = özel_türev(ileri: (x) => [x * x, [x]], geri: (s, g) => [g * 2 * s[0]]); değişken x = Tensör([3.0], türev_izle: doğru); geri_yayılım(toplam(kare(x))); yazdır(x.gradyan.değer)
```

İlişkili: `TR_grad_check(...)`, `TR_backward(...)`

#### `TR_default_device(...)` (`TR_fn_default_device`, `varsayılan_cihaz`)

    varsayılan_cihaz([Metin]) → Metin

Argümanla çağrılınca ("cpu", "gpu", "gpu:1") bundan sonra oluşturulan bütün tensörlerin cihazını seçer: Tensör(...), sıfırlar, birler, rastgele_normal ve veri kümesi örnekleyicisinin toplu işleri o cihazda kurulur. Argümansız çağrı geçerli cihazın adını verir. Model GPU'da eğitilecekse bu çağrı model kurulmadan önce yapılır; olmayan cihaz hatadır.

```tr
TR_print(TR_default_device()); TR_default_device("cpu"); TR_print(TR_Tensor([1.0]).TR_device)
```

Görünümü:

```text
en: print(default_device()); default_device("cpu"); print(Tensor([1.0]).device)
tr: yazdır(varsayılan_cihaz()); varsayılan_cihaz("cpu"); yazdır(Tensör([1.0]).cihaz)
```

İlişkili: `TR_devices(...)`, `.TR_to_device(...)`, `.TR_device`

#### `TR_devices(...)` (`TR_fn_devices`, `cihazlar`)

    cihazlar() → Liste

Bu makinede kullanılabilir cihazların adlarını liste olarak verir; ilk eleman her zaman "cpu"dur, NVIDIA GPU'su ve sürücüsü varsa "gpu" (birden çoksa "gpu:1", ...) gelir.

```tr
TR_print(TR_devices()[0], TR_devices().TR_contains("gpu"))
```

Görünümü:

```text
en: print(devices()[0], devices().contains("gpu"))
tr: yazdır(cihazlar()[0], cihazlar().içerir("gpu"))
```

İlişkili: `TR_default_device(...)`, `.TR_to_device(...)`

#### `TR_embedding_lookup(...)` (`TR_fn_embedding_lookup`, `gömme_seç`)

    gömme_seç(Tensör, Tam | Liste | Tensör) → Tensör

[V, D] şeklindeki tablodan verilen indekslerin satırlarını seçer; sonuç şekli indeks şekli + [D]'dir. Tek tam sayı indeks [D] şeklinde tek satır verir; gradyan tabloya akar.

```tr
TR_var tablo = TR_Tensor([[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]); TR_print(TR_embedding_lookup(tablo, [2, 0]).TR_tolist)
```

Görünümü:

```text
en: var tablo = Tensor([[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]); print(embedding_lookup(tablo, [2, 0]).tolist)
tr: değişken tablo = Tensör([[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]); yazdır(gömme_seç(tablo, [2, 0]).değer)
```

İlişkili: `TR_cross_entropy(...)`

#### `TR_env(...)` (`TR_fn_env`, `ortam_değişkeni`)

    ortam_değişkeni(Metin) → Metin | YokTipi

Ortam değişkeninin değerini Metin olarak verir; tanımsızsa yok döner (varsayılan için ?? yazılır). Adının bir sözcüğü TOKEN, KEY, SECRET, PASSWORD, ŞİFRE, PAROLA, ANAHTAR gibi bir gizli sözcükse (HF_TOKEN, api_anahtarı; TOKENIZER_PATH değil) sayı ve mantık olmayan değeri gizli sayılır ve insanın göreceği çıktılarda *** olarak görünür.

```tr
TR_var klasör = TR_env("VERİ_KLASÖRÜ") ?? "veri"; TR_print(klasör)
```

Görünümü:

```text
en: var klasör = env("VERİ_KLASÖRÜ") ?? "veri"; print(klasör)
tr: değişken klasör = ortam_değişkeni("VERİ_KLASÖRÜ") ?? "veri"; yazdır(klasör)
```

İlişkili: `TR_command_line(...)`

#### `TR_error_code(...)` (`TR_fn_error_code`, `hata_kodu`)

    hata_kodu(Fonksiyon | YerleşikFonksiyon) → Metin | YokTipi

Verilen fonksiyonu çalıştırır; fonksiyon bir ileti (ileti) fırlatırsa onun kodunu Metin olarak ("H6903") döndürür, hata fırlatmazsa ya da hata kodlu değilse yok döndürür. İletinin metnini değil kodunu sınamak içindir.

```tr
TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) })
```

Görünümü:

```text
en: print(error_code { throw Message("H6903", -1) })
tr: yazdır(hata_kodu { fırlat İleti("H6903", -1) })
```

İlişkili: `TR_Message`, `TR_assert(...)`

#### `TR_exit(...)` (`TR_fn_exit`, `sonlandır`)

    sonlandır(Tam) → YokTipi

Programı verilen çıkış koduyla nazikçe bitirir: ertele blokları çalışır, dene/yakala bunu yakalamaz; eşzamanlı bir görevden çağrılsa da bütün program biter. Kod 0–125 arasında bir Tam olmalıdır, dışındaki değer çalışma hatasıdır; programın kendi kodları 4'ten başlar (1 hata, 2 kullanım hatası).

```tr
TR_exit(4)
```

Görünümü:

```text
en: exit(4)
tr: sonlandır(4)
```

İlişkili: `TR_handle_stop(...)`

#### `TR_exp(...)` (`TR_fn_exp`, `exp`)

    exp(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın e tabanında üstelini (e^x) hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_exp(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(exp(Tensor([0.0, 1.0])).tolist)
tr: yazdır(exp(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_log(...)`

#### `TR_expm1(...)` (`TR_fn_expm1`, `expm1`)

    expm1(Tensör | Tam | Ondalık) → Tensör | Ondalık

e^x - 1 hesaplar; küçük x'te çıkarmadaki basamak kaybı olmaz (expm1(1e-10) = 1e-10). Türevi exp(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_expm1(0.000001), TR_expm1(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(expm1(0.000001), expm1(Tensor([0.0, 1.0])).tolist)
tr: yazdır(expm1(0.000001), expm1(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_exp(...)`, `TR_log1p(...)`

#### `TR_eye(...)` (`TR_fn_eye`, `birim_matris`)

    birim_matris(Tam) → Tensör

n×n birim matrisi (köşegeni 1, geri kalanı 0 olan 2 boyutlu tensör) oluşturur. n pozitif bir Tam olmalıdır.

```tr
TR_print(TR_eye(3).TR_tolist)
```

Görünümü:

```text
en: print(eye(3).tolist)
tr: yazdır(birim_matris(3).değer)
```

İlişkili: `TR_Tensor`

#### `TR_floor(...)` (`TR_fn_floor`, `taban`)

    taban(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayıyı aşağı (eksi sonsuza doğru) en yakın tam sayıya yuvarlar; Tam girdide Tam, Ondalık girdide Ondalık döner. Tensörde eleman bazlı çalışır (tam sayılı tensör tam sayılı kalır), türevi 0'dır; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_floor(3.7), TR_floor(-2), TR_floor(TR_Tensor([-1.5, 2.5])).TR_tolist)
```

Görünümü:

```text
en: print(floor(3.7), floor(-2), floor(Tensor([-1.5, 2.5])).tolist)
tr: yazdır(taban(3.7), taban(-2), taban(Tensör([-1.5, 2.5])).değer)
```

İlişkili: `TR_ceil(...)`, `TR_round(...)`

#### `TR_fuse(...)` (`TR_fn_fuse`, `kaynaştır`)

    kaynaştır(Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

Eleman bazlı tensör işlemlerinden oluşan bir fonksiyonu kaynaştırır ve aynı şeyi hesaplayan bir fonksiyon döndürür: her çağrıda zincirin tamamı tek geçişte (GPU'da tek çekirdekte) hesaplanır, ara sonuçlar belleğe yazılmaz; geri yayılım ara değerleri saklamadan girdilerden yeniden hesaplar. Sonuç kaynaşmamış hesapla aynıdır. Fonksiyon eleman bazlı işlemleri, dilimi, .böl'ü, bitiştir'i, boyu 1 eksen ekleyip çıkarmayı ve tür dönüşümünü kullanabilir; argümanları tensör ya da sayıdır (sayı programa sabit olarak girer).

```tr
TR_var f = TR_fuse((a, b) => a * b + a); TR_print(f(TR_Tensor([1.0, 2.0]), TR_Tensor([3.0, 4.0])).TR_tolist)
```

Görünümü:

```text
en: var f = fuse((a, b) => a * b + a); print(f(Tensor([1.0, 2.0]), Tensor([3.0, 4.0])).tolist)
tr: değişken f = kaynaştır((a, b) => a * b + a); yazdır(f(Tensör([1.0, 2.0]), Tensör([3.0, 4.0])).değer)
```

İlişkili: `TR_custom_grad(...)`

#### `TR_gelu(...)` (`TR_fn_gelu`, `gelu`)

    gelu(Tensör | Tam | Ondalık, yaklaşık: Mantık = doğru) → Tensör | Ondalık

Eleman bazlı GELU etkinleştirmesidir; varsayılan tanh yaklaşımıyla hesaplanır, yaklaşık: yanlış tam (erf tabanlı) GELU verir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_gelu(TR_Tensor([-1.0, 0.0, 1.0])).TR_tolist, TR_gelu(1.0, TR_approximate: TR_false))
```

Görünümü:

```text
en: print(gelu(Tensor([-1.0, 0.0, 1.0])).tolist, gelu(1.0, approximate: false))
tr: yazdır(gelu(Tensör([-1.0, 0.0, 1.0])).değer, gelu(1.0, yaklaşık: yanlış))
```

İlişkili: `TR_relu(...)`

#### `TR_grad_check(...)` (`TR_fn_grad_check`, `türev_denetle`)

    türev_denetle(Fonksiyon | YerleşikFonksiyon, Liste, fark_adımı: Tam | Ondalık = 0.001, tolerans: Tam | Ondalık = 0.01) → Mantık

Bir fonksiyonun geri yayılımla bulunan gradyanını merkezi farkla bulunan sayısal gradyanla karşılaştırır; uyuşursa doğru döndürür, uyuşmazsa en büyük farkı gösteren bir hata verir. Yalnızca türev izleyen tensör girdileri, kopyaları üzerinde denetlenir.

```tr
TR_print(TR_grad_check((x) => x * x, [TR_Tensor([0.5, -1.0], TR_requires_grad: TR_true)], TR_fd_step: 0.001, TR_tol: 0.01))
```

Görünümü:

```text
en: print(grad_check((x) => x * x, [Tensor([0.5, -1.0], requires_grad: true)], fd_step: 0.001, tol: 0.01))
tr: yazdır(türev_denetle((x) => x * x, [Tensör([0.5, -1.0], türev_izle: doğru)], fark_adımı: 0.001, tolerans: 0.01))
```

İlişkili: `TR_custom_grad(...)`

#### `TR_handle_stop(...)` (`TR_fn_handle_stop`, `durdurmayı_üstlen`)

    durdurmayı_üstlen() → YokTipi

Ctrl+C ve sonlandırma isteklerinin karşılanmasını programa bırakır: ondan sonra gelen ilk istek programı durdurmaz, kaydedilir (durdurma_istendi() doğru döner). Program kendi sınırında biter; ertele blokları çalışır.

```tr
TR_handle_stop()
```

Görünümü:

```text
en: handle_stop()
tr: durdurmayı_üstlen()
```

İlişkili: `TR_stop_requested(...)`, `TR_exit(...)`

#### `TR_has_rope_scaling(...)` (`TR_fn_has_rope_scaling`, `rope_ölçekleme_var_mı`)

    rope_ölçekleme_var_mı()



#### `TR_init_weight(...)` (`TR_fn_init_weight`, `ağırlık_başlat`)

    ağırlık_başlat()



#### `TR_input(...)` (`TR_fn_input`, `girdi`)

    girdi([Herhangi]) → Metin

Standart girişten bir satır okur, varsa önce istemi yazar; satır sonu atılır, giriş bittiyse boş metin döner.

```tr
TR_var ad = TR_input("Adınız: "); TR_print("Merhaba " + ad)
```

Görünümü:

```text
en: var ad = input("Adınız: "); print("Merhaba " + ad)
tr: değişken ad = girdi("Adınız: "); yazdır("Merhaba " + ad)
```

İlişkili: `TR_print(...)`

#### `TR_is_finite(...)` (`TR_fn_is_finite`, `sonlu_mu`)

    sonlu_mu(Tam | Ondalık | Tensör) → Mantık | Tensör

Sayının sonlu olup olmadığını verir: NaN, sonsuz ve eksi sonsuzda yanlış, öbür her sayıda (her Tam dahil) doğru. Tensörde eleman bazlı çalışır: sonlu elemanda 1, öbürlerinde 0 olan float32 bir tensör verir (karşılaştırmalar gibi), türev izlemez; listede çalışmaz (Tensör(...) ile çevrilir). Eğitimde gradyan normunu denetlemek için kullanılır: eğer değil sonlu_mu(norm.değer) { ... }.

```tr
TR_print(TR_is_finite(1.5), TR_is_finite(TR_Float("nan")), TR_is_finite(TR_Tensor([1.0, TR_Float("inf")])).TR_tolist)
```

Görünümü:

```text
en: print(is_finite(1.5), is_finite(Float("nan")), is_finite(Tensor([1.0, Float("inf")])).tolist)
tr: yazdır(sonlu_mu(1.5), sonlu_mu(Ondalık("nan")), sonlu_mu(Tensör([1.0, Ondalık("inf")])).değer)
```

İlişkili: `TR_sign(...)`, `TR_clip_grad(...)`

#### `TR_json_parse(...)` (`TR_fn_json_parse`, `json_çöz`)

    json_çöz(Metin) → JSON

JSON metnini bir JSON değerine çevirir (3.7a); anahtar sırası ve sayının tam ya da ondalık yazımı korunur. Asıl değere tip çağrısıyla geçilir.

```tr
TR_var v = TR_json_parse(`{"ad": "Ali", "yaş": 30}`); TR_print(v["ad"], TR_Int(v["yaş"]) + 1)
```

Görünümü:

```text
en: var v = json_parse(`{"ad": "Ali", "yaş": 30}`); print(v["ad"], Int(v["yaş"]) + 1)
tr: değişken v = json_çöz(`{"ad": "Ali", "yaş": 30}`); yazdır(v["ad"], Tam(v["yaş"]) + 1)
```

İlişkili: `TR_json_string(...)`

#### `TR_json_string(...)` (`TR_fn_json_string`, `json_metin`)

    json_metin(Herhangi) → Metin

Değeri boşluksuz JSON metnine çevirir; anahtarlar sözlükteki sırayla yazılır, NaN, sonsuz ya da kendini içeren değer hatadır.

```tr
TR_print(TR_json_string({"ad": "Ali", "puan": [90, 85]}))
```

Görünümü:

```text
en: print(json_string({"ad": "Ali", "puan": [90, 85]}))
tr: yazdır(json_metin({"ad": "Ali", "puan": [90, 85]}))
```

İlişkili: `TR_json_parse(...)`

#### `TR_layer_norm(...)` (`TR_fn_layer_norm`, `katman_normu`)

    katman_normu(Tensör | Liste, [Tensör | Liste | YokTipi], [Tensör | Liste | YokTipi], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | YokTipi = yok, sapma: Tensör | Liste | YokTipi = yok, tür: Metin = yok) → Tensör

Girdiyi son eksende ortalaması 0, varyansı 1 olacak biçimde normalleştirir; isteğe bağlı ağırlık ve sapma [D] şeklinde olmalıdır. Konumlu ya da adlı verilebilirler; epsilon varsayılanı 0.00001'dir.

```tr
TR_print(TR_layer_norm(TR_Tensor([[1.0, 2.0, 3.0]]), TR_weight: TR_ones([3]), TR_bias: TR_zeros([3])).TR_tolist)
```

Görünümü:

```text
en: print(layer_norm(Tensor([[1.0, 2.0, 3.0]]), weight: ones([3]), bias: zeros([3])).tolist)
tr: yazdır(katman_normu(Tensör([[1.0, 2.0, 3.0]]), ağırlık: birler([3]), sapma: sıfırlar([3])).değer)
```

İlişkili: `TR_rms_norm(...)`

#### `TR_layer_windows(...)` (`TR_fn_layer_windows`, `katman_pencereleri`)

    katman_pencereleri()



#### `TR_linear_cross_entropy(...)` (`TR_fn_linear_cross_entropy`, `doğrusal_çapraz_entropi`)

    doğrusal_çapraz_entropi(Tensör, Tensör, Tam | Liste | Tensör) → Tensör

çapraz_entropi(girdi @ ağırlık, hedef) ile aynı kaybı ve gradyanları verir, ama bütün logitleri bellekte tutmaz: satırları dilim dilim işler (büyük sözlüklü dil modellerinin çıkış katmanı için). Ağırlık [girdinin son ekseni, sınıf sayısı] şeklindedir; paylaşımlı gömmede tablo.devrik() verilir.

```tr
TR_print(TR_linear_cross_entropy(TR_Tensor([[1.0, 2.0]]), TR_Tensor([[0.5, -1.0, 0.2], [0.1, 0.3, -0.4]]), [2]).TR_tolist)
```

Görünümü:

```text
en: print(linear_cross_entropy(Tensor([[1.0, 2.0]]), Tensor([[0.5, -1.0, 0.2], [0.1, 0.3, -0.4]]), [2]).tolist)
tr: yazdır(doğrusal_çapraz_entropi(Tensör([[1.0, 2.0]]), Tensör([[0.5, -1.0, 0.2], [0.1, 0.3, -0.4]]), [2]).değer)
```

İlişkili: `TR_cross_entropy(...)`

#### `TR_list_dir(...)` (`TR_fn_list_dir`, `klasörü_listele`)

    klasörü_listele([Metin | Yol], [Metin]) → Liste

Klasörde kalıba uyan dosya ve klasörleri sıralı bir Yol listesi olarak verir (varsayılan: "." ve "*"). Kalıpta * ve ? bir bileşenin içinde, [a-z] bir karakter kümesi, ** tam bileşen olarak sıfır ya da çok klasör eşler; nokta ile başlayan adlar, kalıp da nokta ile başlamadıkça listelenmez, klasör bağlantılarının içine girilmez.

```tr
TR_make_dir("liste/alt"); TR_write_file("liste/a.npy", "1"); TR_write_file("liste/alt/b.npy", "2"); TR_print(TR_list_dir("liste", "**/*.npy"))
```

Görünümü:

```text
en: make_dir("liste/alt"); write_file("liste/a.npy", "1"); write_file("liste/alt/b.npy", "2"); print(list_dir("liste", "**/*.npy"))
tr: klasör_oluştur("liste/alt"); dosya_yaz("liste/a.npy", "1"); dosya_yaz("liste/alt/b.npy", "2"); yazdır(klasörü_listele("liste", "**/*.npy"))
```

İlişkili: `TR_Path`, `TR_make_dir(...)`

#### `TR_load_parameters(...)` (`TR_fn_load_parameters`, `parametreleri_yükle`)

    parametreleri_yükle(Nesne | Liste | Sözlük | Tensör, Sözlük) → Tam

Ad → tensör sözlüğünü (tensör_yükle sonucu) modelin var olan parametrelerine, parametreler(model, adlarıyla: doğru) ile aynı adlarla yerinde yazar ve yazılan parametre sayısını verir; tensörler parametrenin cihazına taşınır, türü farklıysa dönüştürülür. Eksik, fazla ya da şekli farklı ad hatadır; iyileştiricinin tuttuğu tensörler aynı kalır.

```tr
TR_var m = {"w": TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)}; TR_save_tensors("m.trt", {"w": TR_Tensor([5.0, 6.0])}); TR_print(TR_load_parameters(m, TR_load_tensors("m.trt")), m["w"].TR_tolist)
```

Görünümü:

```text
en: var m = {"w": Tensor([1.0, 2.0], requires_grad: true)}; save_tensors("m.trt", {"w": Tensor([5.0, 6.0])}); print(load_parameters(m, load_tensors("m.trt")), m["w"].tolist)
tr: değişken m = {"w": Tensör([1.0, 2.0], türev_izle: doğru)}; tensör_kaydet("m.trt", {"w": Tensör([5.0, 6.0])}); yazdır(parametreleri_yükle(m, tensör_yükle("m.trt")), m["w"].değer)
```

İlişkili: `TR_parameters(...)`, `TR_load_tensors(...)`, `TR_Checkpoint`

#### `TR_load_tensors(...)` (`TR_fn_load_tensors`, `tensör_yükle`)

    tensör_yükle(Metin, türev_izle: Mantık = yanlış) → Sözlük

tensör_kaydet ile yazılmış dosyayı ya da safetensors dosyasını (Hugging Face, PyTorch; biçim içerikten anlaşılır) ad → tensör sözlüğü olarak, dosyadaki sırayla okur; tensörler kaydedildikleri türdedir (F32, BF16, F16). Bozuk, eksik ya da başka türden dosya açık bir hatayla reddedilir.

```tr
TR_save_tensors("a.trt", {"w": TR_Tensor([1.0])}); TR_print(TR_load_tensors("a.trt", TR_requires_grad: TR_true)["w"].TR_grad)
```

Görünümü:

```text
en: save_tensors("a.trt", {"w": Tensor([1.0])}); print(load_tensors("a.trt", requires_grad: true)["w"].grad)
tr: tensör_kaydet("a.trt", {"w": Tensör([1.0])}); yazdır(tensör_yükle("a.trt", türev_izle: doğru)["w"].gradyan)
```

İlişkili: `TR_save_tensors(...)`

#### `TR_load_tokenizer(...)` (`TR_fn_load_tokenizer`, `parçalayıcı_yükle`)

    parçalayıcı_yükle(Metin) → Parçalayıcı

tokenizer.json biçimindeki bir parçalayıcı dosyasını yükler ve Parçalayıcı nesnesi döndürür. Adlı argüman almaz; klasör yolu verilirse hata verir.

```tr
TR_var p = TR_load_tokenizer("tests/data/tokenizer.json"); TR_print(p)
```

Görünümü:

```text
en: var p = load_tokenizer("tests/data/tokenizer.json"); print(p)
tr: değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(p)
```

İlişkili: `.TR_encode(...)`, `.TR_decode(...)`, `.TR_vocab_size`, `.TR_token_id(...)`

#### `TR_log(...)` (`TR_fn_log`, `log`)

    log(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın doğal logaritmasını hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_log(TR_Tensor([1.0, 10.0])).TR_tolist)
```

Görünümü:

```text
en: print(log(Tensor([1.0, 10.0])).tolist)
tr: yazdır(log(Tensör([1.0, 10.0])).değer)
```

İlişkili: `TR_exp(...)`

#### `TR_log10(...)` (`TR_fn_log10`, `log10`)

    log10(Tensör | Tam | Ondalık) → Tensör | Ondalık

10 tabanında logaritmadır; 0'da -sonsuz, negatifte NaN. Türevi 1 / (x · ln 10)'dur. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_log10(1000), TR_log10(TR_Tensor([1.0, 100.0])).TR_tolist)
```

Görünümü:

```text
en: print(log10(1000), log10(Tensor([1.0, 100.0])).tolist)
tr: yazdır(log10(1000), log10(Tensör([1.0, 100.0])).değer)
```

İlişkili: `TR_log(...)`, `TR_log2(...)`

#### `TR_log1p(...)` (`TR_fn_log1p`, `log1p`)

    log1p(Tensör | Tam | Ondalık) → Tensör | Ondalık

log(1 + x) hesaplar; küçük x'te 1 + x yuvarlamasından etkilenmez (log1p(1e-10) = 1e-10). x = -1'de -sonsuz, x < -1'de NaN; türevi 1 / (1 + x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_log1p(0.000001), TR_log1p(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(log1p(0.000001), log1p(Tensor([0.0, 1.0])).tolist)
tr: yazdır(log1p(0.000001), log1p(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_log(...)`, `TR_expm1(...)`

#### `TR_log2(...)` (`TR_fn_log2`, `log2`)

    log2(Tensör | Tam | Ondalık) → Tensör | Ondalık

2 tabanında logaritmadır; ikinin kuvvetlerinde tam sonuç verir, 0'da -sonsuz, negatifte NaN. Türevi 1 / (x · ln 2)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_log2(8), TR_log2(TR_Tensor([1.0, 1024.0])).TR_tolist)
```

Görünümü:

```text
en: print(log2(8), log2(Tensor([1.0, 1024.0])).tolist)
tr: yazdır(log2(8), log2(Tensör([1.0, 1024.0])).değer)
```

İlişkili: `TR_log(...)`, `TR_log10(...)`

#### `TR_log_softmax(...)` (`TR_fn_log_softmax`, `log_softmax`)

    log_softmax(Tensör, [Tam], eksen: Tam = -1) → Tensör

softmax'ın logaritmasını verilen eksende (varsayılan son eksen) sayısal olarak kararlı biçimde hesaplar. Yalnızca tensörde çalışır, listede çalışmaz (Tensör(...) ile çevrilir); tamamen -sonsuz bir satırın sonucu -sonsuz, gradyanı 0'dır.

```tr
TR_print(TR_log_softmax(TR_Tensor([1.0, 2.0, 3.0])).TR_tolist)
```

Görünümü:

```text
en: print(log_softmax(Tensor([1.0, 2.0, 3.0])).tolist)
tr: yazdır(log_softmax(Tensör([1.0, 2.0, 3.0])).değer)
```

İlişkili: `TR_softmax(...)`, `TR_cross_entropy(...)`

#### `TR_lr_schedule(...)` (`TR_fn_lr_schedule`, `öğrenme_oranı_planı`)

    öğrenme_oranı_planı()



#### `TR_make_dir(...)` (`TR_fn_make_dir`, `klasör_oluştur`)

    klasör_oluştur(Metin | Yol) → Yol

Klasörü üst klasörleriyle birlikte oluşturur ve Yol olarak döndürür; klasör zaten varsa sorun değildir, aynı adda bir dosya varsa hatadır.

```tr
TR_var k = TR_make_dir("çıktı/kontrol_noktaları"); TR_print(k, k.TR_is_dir)
```

Görünümü:

```text
en: var k = make_dir("çıktı/kontrol_noktaları"); print(k, k.is_dir)
tr: değişken k = klasör_oluştur("çıktı/kontrol_noktaları"); yazdır(k, k.klasör_mü)
```

İlişkili: `TR_list_dir(...)`, `TR_remove(...)`

#### `TR_map_hugging_face(...)` (`TR_fn_map_hugging_face`, `hugging_face_eşle`)

    hugging_face_eşle()



#### `TR_max(...)` (`TR_fn_max`, `max`)

    max(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | YokTipi | Tensör

Tek bir listenin ya da birden çok sayının en büyüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
TR_print(TR_max([2.0, 1.5]), TR_max(3, 7, 5), TR_max(TR_Tensor([[1, 5], [3, 2]]), TR_dim: 0).TR_tolist)
```

Görünümü:

```text
en: print(max([2.0, 1.5]), max(3, 7, 5), max(Tensor([[1, 5], [3, 2]]), dim: 0).tolist)
tr: yazdır(max([2.0, 1.5]), max(3, 7, 5), max(Tensör([[1, 5], [3, 2]]), eksen: 0).değer)
```

İlişkili: `TR_min(...)`, `TR_argmax(...)`

#### `TR_mean(...)` (`TR_fn_mean`, `ortalama`)

    ortalama(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Ondalık | Tensör

Sayıların aritmetik ortalamasını Ondalık olarak verir; tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır. Tensörde eksen: alır, boş tensörün ortalaması hatadır.

```tr
TR_print(TR_mean([1, 2, 3, 4]), TR_mean(TR_Tensor([[1, 2], [3, 4]]), TR_dim: 1).TR_tolist)
```

Görünümü:

```text
en: print(mean([1, 2, 3, 4]), mean(Tensor([[1, 2], [3, 4]]), dim: 1).tolist)
tr: yazdır(ortalama([1, 2, 3, 4]), ortalama(Tensör([[1, 2], [3, 4]]), eksen: 1).değer)
```

İlişkili: `TR_sum(...)`, `TR_median(...)`

#### `TR_median(...)` (`TR_fn_median`, `ortanca`)

    ortanca(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların ortancasını Ondalık olarak verir; eleman sayısı çiftse ortadaki ikisinin ortalamasıdır; ortalama gibi boş girdi ve sayı olmayan eleman hatadır.

```tr
TR_print(TR_median([3, 1, 4, 2]))
```

Görünümü:

```text
en: print(median([3, 1, 4, 2]))
tr: yazdır(ortanca([3, 1, 4, 2]))
```

İlişkili: `TR_mean(...)`

#### `TR_min(...)` (`TR_fn_min`, `min`)

    min(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | YokTipi | Tensör

Tek bir listenin ya da birden çok sayının en küçüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
TR_print(TR_min([3, 1, 2]), TR_min(4, 2.5), TR_min([]))
```

Görünümü:

```text
en: print(min([3, 1, 2]), min(4, 2.5), min([]))
tr: yazdır(min([3, 1, 2]), min(4, 2.5), min([]))
```

İlişkili: `TR_max(...)`

#### `TR_mixed_precision(...)` (`TR_fn_mixed_precision`, `karışık_hassasiyet`)

    karışık_hassasiyet(Metin, Fonksiyon | YerleşikFonksiyon) → Herhangi

Bloğu verilen hesap türüyle ("bf16", "fp16" ya da kapatmak için "float32") çalıştırır ve bloğun son ifadesinin değerini döndürür. Blok içinde @ ve dikkat girdileri bu türe çevrilir; biriktirme float32'dir, sonuç bu türdedir (dil tanımı 15.4); ağırlıkların türü değişmez.

```tr
TR_var y = TR_mixed_precision("bf16") { TR_Tensor([[1.0, 2.0]]) @ TR_Tensor([[3.0], [4.0]]) }; TR_print(y.TR_tolist, y.TR_dtype)
```

Görünümü:

```text
en: var y = mixed_precision("bf16") { Tensor([[1.0, 2.0]]) @ Tensor([[3.0], [4.0]]) }; print(y.tolist, y.dtype)
tr: değişken y = karışık_hassasiyet("bf16") { Tensör([[1.0, 2.0]]) @ Tensör([[3.0], [4.0]]) }; yazdır(y.değer, y.tür)
```

İlişkili: `TR_dtype:`, `TR_compute_dtype(...)`

#### `TR_move(...)` (`TR_fn_move`, `taşı`)

    taşı(Metin | Yol, Metin | Yol, üzerine_yaz: Mantık = yanlış) → Yol

Dosyayı ya da klasörü taşır ve hedefin yolunu döndürür; hedef kuralları kopyala ile aynıdır. Aynı diskte yeniden adlandırır; başka bir diske kopyalar, boyutu doğrular, sonra kaynağı siler (kaynağa sil'in korumaları uygulanır).

```tr
TR_write_file("taşınacak.txt", "x"); TR_print(TR_move("taşınacak.txt", "taşındı.txt"))
```

Görünümü:

```text
en: write_file("taşınacak.txt", "x"); print(move("taşınacak.txt", "taşındı.txt"))
tr: dosya_yaz("taşınacak.txt", "x"); yazdır(taşı("taşınacak.txt", "taşındı.txt"))
```

İlişkili: `TR_copy(...)`, `TR_remove(...)`

#### `TR_name_matches(...)` (`TR_fn_name_matches`, `ad_uyar_mı`)

    ad_uyar_mı()



#### `TR_no_grad(...)` (`TR_fn_no_grad`, `türevsiz`)

    türevsiz(Tensör | Fonksiyon | YerleşikFonksiyon) → Herhangi

türevsiz { ... } bloğu türev izlemeden çalışır ve bloğun değerini döndürür; türevsiz(x) ise tensörü hesap çizgesinden ayırır. Türevsiz kip görev başınadır, başka eşzamanlı görevlere sızmaz.

```tr
TR_var x = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var y = TR_no_grad { x * 2 }; TR_print(y.TR_tolist)
```

Görünümü:

```text
en: var x = Tensor([1.0], requires_grad: true); var y = no_grad { x * 2 }; print(y.tolist)
tr: değişken x = Tensör([1.0], türev_izle: doğru); değişken y = türevsiz { x * 2 }; yazdır(y.değer)
```

İlişkili: `TR_backward(...)`

#### `TR_ones(...)` (`TR_fn_ones`, `birler`)

    birler(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 1 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir ([2, 3]).

```tr
TR_print(TR_ones([2, 2], TR_dtype: "bf16").TR_tolist)
```

Görünümü:

```text
en: print(ones([2, 2], dtype: "bf16").tolist)
tr: yazdır(birler([2, 2], tür: "bf16").değer)
```

İlişkili: `TR_zeros(...)`, `TR_Tensor`

#### `TR_only_2d(...)` (`TR_fn_only_2d`, `yalnızca_2b`)

    yalnızca_2b()



#### `TR_orthogonalize(...)` (`TR_fn_orthogonalize`, `dikleştir`)

    dikleştir()



#### `TR_orthogonalize_stack(...)` (`TR_fn_orthogonalize_stack`, `yığın_dikleştir`)

    yığın_dikleştir()



#### `TR_parameter_groups(...)` (`TR_fn_parameter_groups`, `parametre_grupları`)

    parametre_grupları()



#### `TR_parameters(...)` (`TR_fn_parameters`, `parametreler`)

    parametreler(Nesne | Liste | Sözlük | Tensör, adlarıyla: Mantık = yanlış) → Liste | Sözlük

Bir yapı nesnesini (iç içe yapılar, listeler ve sözlüklerle) gezip türev izleyen yaprak tensörleri sabit sırayla ve her birini bir kez toplar. adlarıyla: doğru ile yol adı → tensör sözlüğü döndürür; bu sözlük tensör_kaydet ile kaydedilebilir.

```tr
TR_var model = {"w": TR_Tensor([1.0], TR_requires_grad: TR_true), "b": TR_Tensor([0.0], TR_requires_grad: TR_true)}; TR_print(TR_parameters(model, TR_named: TR_true))
```

Görünümü:

```text
en: var model = {"w": Tensor([1.0], requires_grad: true), "b": Tensor([0.0], requires_grad: true)}; print(parameters(model, named: true))
tr: değişken model = {"w": Tensör([1.0], türev_izle: doğru), "b": Tensör([0.0], türev_izle: doğru)}; yazdır(parametreler(model, adlarıyla: doğru))
```

İlişkili: `TR_AdamW(...)`, `TR_save_tensors(...)`

#### `TR_parse_csv(...)` (`TR_fn_parse_csv`, `csv_çöz`)

    csv_çöz(Metin) → Liste

CSV metnini satır listelerine çevirir (json_çöz gibi; dosya okumaz, dosya için csv_oku). Baştaki bayt sırası işareti atılır; hücreler yalnızca katı sayı biçimindeyse sayı olur ("007" Metin kalır).

```tr
TR_var tablo = TR_parse_csv("ad,yaş\nAli,30"); TR_print(tablo[1][0], tablo[1][1] + 1)
```

Görünümü:

```text
en: var tablo = parse_csv("ad,yaş\nAli,30"); print(tablo[1][0], tablo[1][1] + 1)
tr: değişken tablo = csv_çöz("ad,yaş\nAli,30"); yazdır(tablo[1][0], tablo[1][1] + 1)
```

İlişkili: `TR_read_csv(...)`, `TR_json_parse(...)`

#### `TR_ple_gelu(...)` (`TR_fn_ple_gelu`, `ple_gelu`)

    ple_gelu()



#### `TR_ple_mix(...)` (`TR_fn_ple_mix`, `ple_karışımı`)

    ple_karışımı()



#### `TR_ple_mix_fused(...)` (`TR_fn_ple_mix_fused`, `ple_karışımı_kaynaşık`)

    ple_karışımı_kaynaşık()



#### `TR_pretrained_model(...)` (`TR_fn_pretrained_model`, `hazır_model`)

    hazır_model()



#### `TR_print(...)` (`TR_fn_print`, `yazdır`)

    yazdır([Herhangi...]) → Metin

Değerleri aralarına boşluk koyarak tek satırda yazar ve yazdığı metni döndürür; argümansız çağrı boş satır yazar.

```tr
TR_print("Toplam:", 2 + 3)
```

Görünümü:

```text
en: print("Toplam:", 2 + 3)
tr: yazdır("Toplam:", 2 + 3)
```

İlişkili: `TR_input(...)`

#### `TR_randn(...)` (`TR_fn_randn`, `rastgele_normal`)

    rastgele_normal(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32", tohum: Tam) → Tensör

Ortalaması 0, standart sapması 1 olan normal dağılımdan rastgele sayılarla dolu bir tensör oluşturur. Aynı tam sayı tohum her çalıştırmada aynı değerleri verir; şekil liste olarak verilir.

```tr
TR_print(TR_randn([2, 3], TR_seed: 42).TR_shape)
```

Görünümü:

```text
en: print(randn([2, 3], seed: 42).shape)
tr: yazdır(rastgele_normal([2, 3], tohum: 42).şekil)
```

İlişkili: `TR_zeros(...)`

#### `TR_read_csv(...)` (`TR_fn_read_csv`, `csv_oku`)

    csv_oku(Metin | Yol) → Liste

Bir CSV dosyasını (Metin ya da Yol olarak verilen yol) satır listelerine çevirir; .gz uzantılı dosya açılarak okunur. CSV metninin kendisi csv_çöz ile ayrıştırılır. Hücreler yalnızca katı sayı biçimindeyse sayı olur ("007" Metin kalır).

```tr
TR_write_file("tablo.csv", "ad,yaş\nAli,30"); TR_var tablo = TR_read_csv("tablo.csv"); TR_print(tablo[1][0], tablo[1][1] + 1)
```

Görünümü:

```text
en: write_file("tablo.csv", "ad,yaş\nAli,30"); var tablo = read_csv("tablo.csv"); print(tablo[1][0], tablo[1][1] + 1)
tr: dosya_yaz("tablo.csv", "ad,yaş\nAli,30"); değişken tablo = csv_oku("tablo.csv"); yazdır(tablo[1][0], tablo[1][1] + 1)
```

İlişkili: `TR_read_file(...)`, `TR_parse_csv(...)`

#### `TR_read_file(...)` (`TR_fn_read_file`, `dosya_oku`)

    dosya_oku(Metin | Yol) → Metin

Dosyanın tüm içeriğini Metin olarak okur; yol Metin ya da Yol olabilir. Göreli yol önce çalışma klasörüne, sonra modülün ve programın klasörüne göre çözülür, dosya yoksa hata bakılan yerleri listeler. .gz uzantılı dosya açılarak okunur (açılmış içerik en çok 1 GB; daha büyüğü için dosya_satırları).

```tr
TR_write_file("notlar.txt", "merhaba"); TR_print(TR_read_file("notlar.txt"))
```

Görünümü:

```text
en: write_file("notlar.txt", "merhaba"); print(read_file("notlar.txt"))
tr: dosya_yaz("notlar.txt", "merhaba"); yazdır(dosya_oku("notlar.txt"))
```

İlişkili: `TR_write_file(...)`, `TR_read_lines(...)`, `TR_read_csv(...)`

#### `TR_read_lines(...)` (`TR_fn_read_lines`, `dosya_satırları`)

    dosya_satırları(Metin | Yol) → SatırOkuyucu

Dosyayı akışla satır satır okuyan bir SatırOkuyucu açar; bellek dosyanın boyutundan bağımsızdır. Satır sonu (\n, \r\n) ve baştaki BOM atılır, .gz uzantılı dosya açılarak okunur; tek satır en çok 64 MB olabilir.

```tr
TR_write_file("satırlar.txt", "bir\niki\n"); TR_for satır TR_in TR_read_lines("satırlar.txt") { TR_print(satır) }
```

Görünümü:

```text
en: write_file("satırlar.txt", "bir\niki\n"); for satır in read_lines("satırlar.txt") { print(satır) }
tr: dosya_yaz("satırlar.txt", "bir\niki\n"); döngü satır içinde dosya_satırları("satırlar.txt") { yazdır(satır) }
```

İlişkili: `TR_read_file(...)`, `.TR_next(...)`

#### `TR_relu(...)` (`TR_fn_relu`, `relu`)

    relu(Tensör | Tam | Ondalık) → Tensör | Ondalık

Negatif elemanları 0 yapan, pozitifleri koruyan eleman bazlı etkinleştirme fonksiyonudur. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir); NaN değerini korur.

```tr
TR_print(TR_relu(TR_Tensor([-1.0, 0.5, 2.0])).TR_tolist)
```

Görünümü:

```text
en: print(relu(Tensor([-1.0, 0.5, 2.0])).tolist)
tr: yazdır(relu(Tensör([-1.0, 0.5, 2.0])).değer)
```

İlişkili: `TR_gelu(...)`, `TR_sigmoid(...)`

#### `TR_remove(...)` (`TR_fn_remove`, `sil`)

    sil(Metin | Yol, içindekilerle: Mantık = yanlış) → Tam

Dosyayı, bağlantıyı ya da boş klasörü siler ve silinen dosya sayısını verir; dolu klasör için içindekilerle: doğru gerekir. Olmayan yol, salt okunur dosya, kök, ev ve sistem klasörleri, çalışma ve program klasörü ile onların üstleri hatadır; bağlantıların gösterdiği yere dokunulmaz, joker karakter genişletilmez.

```tr
TR_write_file("silinecek.txt", "x"); TR_print(TR_remove("silinecek.txt"))
```

Görünümü:

```text
en: write_file("silinecek.txt", "x"); print(remove("silinecek.txt"))
tr: dosya_yaz("silinecek.txt", "x"); yazdır(sil("silinecek.txt"))
```

İlişkili: `TR_make_dir(...)`, `TR_move(...)`

#### `TR_replay(...)` (`TR_fn_replay`, `tekrar_oynat`)

    tekrar_oynat(Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

Tensör alıp tensör veren bir fonksiyonu sarar: GPU'da ikinci çağrı f'in işini kaydeder (CUDA Graph), sonraki çağrılar girdileri kopyalayıp kaydı tek seferde oynatır, yorumlayıcı f'i yeniden çalıştırmaz. Yalnızca türevsiz içinde; şekil ya da sayı argümanları değişince yeniden kaydeder, CPU'da f doğrudan çağrılır.

```tr
TR_var f = TR_replay((x) => x * 2); TR_no_grad { TR_print(f(TR_Tensor([1.0, 2.0])).TR_tolist) }
```

Görünümü:

```text
en: var f = replay((x) => x * 2); no_grad { print(f(Tensor([1.0, 2.0])).tolist) }
tr: değişken f = tekrar_oynat((x) => x * 2); türevsiz { yazdır(f(Tensör([1.0, 2.0])).değer) }
```

İlişkili: `TR_fuse(...)`

#### `TR_rms_norm(...)` (`TR_fn_rms_norm`, `rms_normu`)

    rms_normu(Tensör | Liste, [Tensör | Liste | YokTipi], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | YokTipi = yok, tür: Metin = yok) → Tensör

Girdiyi son eksende karelerinin ortalamasının köküne bölerek normalleştirir (ortalama çıkarılmaz). İsteğe bağlı ağırlık [D] şeklinde olmalıdır; epsilon varsayılanı 0.00001'dir.

```tr
TR_print(TR_rms_norm(TR_Tensor([[3.0, 4.0]]), TR_weight: TR_ones([2])).TR_tolist)
```

Görünümü:

```text
en: print(rms_norm(Tensor([[3.0, 4.0]]), weight: ones([2])).tolist)
tr: yazdır(rms_normu(Tensör([[3.0, 4.0]]), ağırlık: birler([2])).değer)
```

İlişkili: `TR_layer_norm(...)`, `TR_compute_dtype(...)`

#### `TR_rope_theta(...)` (`TR_fn_rope_theta`, `rope_tabanı`)

    rope_tabanı()



#### `TR_rotary_table(...)` (`TR_fn_rotary_table`, `döner_konum_tablosu`)

    döner_konum_tablosu()



#### `TR_round(...)` (`TR_fn_round`, `yuvarla`)

    yuvarla(Tam | Ondalık, [Tam]) → Tam | Ondalık

Sayıyı en yakına yuvarlar (yarımlar sıfırdan uzağa); basamak pozitifse Ondalık, 0 ya da negatifse Tam verir.

```tr
TR_print(TR_round(3.14159, 2), TR_round(2.5), TR_round(1234, -2))
```

Görünümü:

```text
en: print(round(3.14159, 2), round(2.5), round(1234, -2))
tr: yazdır(yuvarla(3.14159, 2), yuvarla(2.5), yuvarla(1234, -2))
```

İlişkili: `TR_Int`

#### `TR_sample(...)` (`TR_fn_sample`, `örnekle`)

    örnekle(Tensör, sıcaklık: Tam | Ondalık = 1, en_iyi_k: Tam = 0, tohum: Tam | YokTipi = yok) → Tensör

Son eksendeki softmax(logit / sıcaklık) dağılımından bir indeks örnekler; sonuç tam sayılı ve türev izlemeyen bir tensördür. sıcaklık 0 en olasıyı verir, en_iyi_k > 0 seçimi en olası k sınıfla sınırlar, aynı tohum aynı sonucu verir.

```tr
TR_print(TR_sample(TR_Tensor([1.0, 3.0, 0.5]), TR_temperature: 0.8, TR_top_k: 2, TR_seed: 7).TR_tolist)
```

Görünümü:

```text
en: print(sample(Tensor([1.0, 3.0, 0.5]), temperature: 0.8, top_k: 2, seed: 7).tolist)
tr: yazdır(örnekle(Tensör([1.0, 3.0, 0.5]), sıcaklık: 0.8, en_iyi_k: 2, tohum: 7).değer)
```

İlişkili: `TR_argmax(...)`, `TR_softmax(...)`

#### `TR_save_tensors(...)` (`TR_fn_save_tensors`, `tensör_kaydet`)

    tensör_kaydet(Metin, Sözlük) → YokTipi

Anahtarları metin, değerleri tensör olan bir sözlüğü sürüm başlıklı ikili TR tensör dosyasına yazar. Yazma atomiktir; dosya varsa üzerine yazılır ve tensörler kendi türlerinde saklanır.

```tr
TR_save_tensors("agirliklar.trt", {"w": TR_Tensor([[1.0, 2.0]]), "b": TR_Tensor([0.5])}); TR_print(TR_load_tensors("agirliklar.trt"))
```

Görünümü:

```text
en: save_tensors("agirliklar.trt", {"w": Tensor([[1.0, 2.0]]), "b": Tensor([0.5])}); print(load_tensors("agirliklar.trt"))
tr: tensör_kaydet("agirliklar.trt", {"w": Tensör([[1.0, 2.0]]), "b": Tensör([0.5])}); yazdır(tensör_yükle("agirliklar.trt"))
```

İlişkili: `TR_load_tensors(...)`, `TR_parameters(...)`

#### `TR_shape_groups(...)` (`TR_fn_shape_groups`, `şekil_grupları`)

    şekil_grupları()



#### `TR_sigmoid(...)` (`TR_fn_sigmoid`, `sigmoid`)

    sigmoid(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemana 1 / (1 + e^(-x)) uygulayarak değerleri 0 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_sigmoid(TR_Tensor([-2.0, 0.0, 2.0])).TR_tolist)
```

Görünümü:

```text
en: print(sigmoid(Tensor([-2.0, 0.0, 2.0])).tolist)
tr: yazdır(sigmoid(Tensör([-2.0, 0.0, 2.0])).değer)
```

İlişkili: `TR_tanh(...)`, `TR_relu(...)`

#### `TR_sign(...)` (`TR_fn_sign`, `işaret`)

    işaret(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Pozitif sayıda 1, negatifte -1, sıfırda 0 verir; Tam girdide Tam, Ondalık girdide Ondalık döner, NaN korunur. Tensörde eleman bazlı çalışır, türevi her yerde (0'da da) 0'dır; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_sign(-4), TR_sign(0.0), TR_sign(TR_Tensor([-2.0, 0.0, 3.0])).TR_tolist)
```

Görünümü:

```text
en: print(sign(-4), sign(0.0), sign(Tensor([-2.0, 0.0, 3.0])).tolist)
tr: yazdır(işaret(-4), işaret(0.0), işaret(Tensör([-2.0, 0.0, 3.0])).değer)
```

İlişkili: `TR_abs(...)`

#### `TR_silu(...)` (`TR_fn_silu`, `silu`)

    silu(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı SiLU etkinleştirmesidir: x · sigmoid(x); tek geçişte hesaplanır, kapılı ileri beslemeli katmanların (SwiGLU) temelidir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_silu(TR_Tensor([-1.0, 0.0, 2.0])).TR_tolist)
```

Görünümü:

```text
en: print(silu(Tensor([-1.0, 0.0, 2.0])).tolist)
tr: yazdır(silu(Tensör([-1.0, 0.0, 2.0])).değer)
```

İlişkili: `TR_sigmoid(...)`, `TR_gelu(...)`

#### `TR_sin(...)` (`TR_fn_sin`, `sin`)

    sin(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın sinüsünü (radyan) hesaplar; türevi cos(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_sin(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(sin(Tensor([0.0, 1.0])).tolist)
tr: yazdır(sin(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_cos(...)`

#### `TR_sinh(...)` (`TR_fn_sinh`, `sinh`)

    sinh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik sinüstür: (e^x - e^(-x)) / 2; türevi cosh(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_sinh(1.0), TR_sinh(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(sinh(1.0), sinh(Tensor([0.0, 1.0])).tolist)
tr: yazdır(sinh(1.0), sinh(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_cosh(...)`, `TR_tanh(...)`

#### `TR_sleep(...)` (`TR_fn_sleep`, `uyu`)

    uyu(Tam | Ondalık) → Mantık

Verilen saniye kadar bekler ve doğru döndürür; beklerken yorumlayıcı kilidini bırakır, öbür görevler ilerler.

```tr
TR_sleep(0.1); TR_print("0.1 saniye geçti")
```

Görünümü:

```text
en: sleep(0.1); print("0.1 saniye geçti")
tr: uyu(0.1); yazdır("0.1 saniye geçti")
```

İlişkili: `TR_wait_all(...)`, `TR_time_ms(...)`

#### `TR_softmax(...)` (`TR_fn_softmax`, `softmax`)

    softmax(Tensör, [Tam], eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) değerleri toplamı 1 olan olasılıklara çevirir. Yalnızca tensörde çalışır, listede çalışmaz (Tensör(...) ile çevrilir); tamamen -sonsuz bir satırın sonucu 0'dır.

```tr
TR_print(TR_softmax(TR_Tensor([[1.0, 2.0, 3.0]]), TR_dim: -1).TR_tolist)
```

Görünümü:

```text
en: print(softmax(Tensor([[1.0, 2.0, 3.0]]), dim: -1).tolist)
tr: yazdır(softmax(Tensör([[1.0, 2.0, 3.0]]), eksen: -1).değer)
```

İlişkili: `TR_log_softmax(...)`, `TR_sample(...)`

#### `TR_sqrt(...)` (`TR_fn_sqrt`, `karekök`)

    karekök(Tam | Ondalık | Tensör) → Ondalık | Tensör

Sayının karekökünü Ondalık olarak verir (negatif sayıda NaN); tensörde eleman bazlı ve türev izleyerek çalışır, liste kabul etmez.

```tr
TR_print(TR_sqrt(16), TR_sqrt(2.25))
```

Görünümü:

```text
en: print(sqrt(16), sqrt(2.25))
tr: yazdır(karekök(16), karekök(2.25))
```

İlişkili: `TR_abs(...)`

#### `TR_state_tensor(...)` (`TR_fn_state_tensor`, `durum_tensörü`)

    durum_tensörü()



#### `TR_std(...)` (`TR_fn_std`, `standart_sapma`)

    standart_sapma(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların popülasyon standart sapmasını (n'ye bölerek) Ondalık olarak verir; ortalama gibi tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır.

```tr
TR_print(TR_std([2, 4, 4, 4, 5, 5, 7, 9]))
```

Görünümü:

```text
en: print(std([2, 4, 4, 4, 5, 5, 7, 9]))
tr: yazdır(standart_sapma([2, 4, 4, 4, 5, 5, 7, 9]))
```

İlişkili: `TR_mean(...)`

#### `TR_stop_requested(...)` (`TR_fn_stop_requested`, `durdurma_istendi`)

    durdurma_istendi() → Mantık

Bir durdurma isteği geldiyse doğru döner ve hiçbir şeyi değiştirmez; isteği durdurmayı_üstlen() ya da kontrol noktası ertelemişse sorgulanır.

```tr
TR_stop_requested()
```

Görünümü:

```text
en: stop_requested()
tr: durdurma_istendi()
```

İlişkili: `TR_handle_stop(...)`

#### `TR_sum(...)` (`TR_fn_sum`, `toplam`)

    toplam(Liste | Tensör, [Tam], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Tensör

Sayı listesini toplar (hepsi Tam ise Tam, bir Ondalık varsa Ondalık); sayı olmayan eleman ve iç içe liste hatadır. Tensörde bütün elemanları ya da eksen: ile verilen ekseni indirger.

```tr
TR_print(TR_sum([1, 2, 3]), TR_sum([1, 2.5]), TR_sum(TR_Tensor([[1, 2], [3, 4]]), TR_dim: 0).TR_tolist)
```

Görünümü:

```text
en: print(sum([1, 2, 3]), sum([1, 2.5]), sum(Tensor([[1, 2], [3, 4]]), dim: 0).tolist)
tr: yazdır(toplam([1, 2, 3]), toplam([1, 2.5]), toplam(Tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

İlişkili: `TR_mean(...)`

#### `TR_swiglu_gate(...)` (`TR_fn_swiglu_gate`, `swiglu_kapısı`)

    swiglu_kapısı()



#### `TR_swiglu_gate_fused(...)` (`TR_fn_swiglu_gate_fused`, `swiglu_kapısı_kaynaşık`)

    swiglu_kapısı_kaynaşık()



#### `TR_swiglu_intermediate_dim(...)` (`TR_fn_swiglu_intermediate_dim`, `swiglu_ara_boyutu`)

    swiglu_ara_boyutu()



#### `TR_swiglu_merged_gate(...)` (`TR_fn_swiglu_merged_gate`, `swiglu_birleşik_kapısı`)

    swiglu_birleşik_kapısı()



#### `TR_swiglu_merged_gate_fused(...)` (`TR_fn_swiglu_merged_gate_fused`, `swiglu_birleşik_kapısı_kaynaşık`)

    swiglu_birleşik_kapısı_kaynaşık()



#### `TR_tan(...)` (`TR_fn_tan`, `tan`)

    tan(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın tanjantını (radyan) hesaplar; türevi 1 + tan(x)²'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_tan(0.5), TR_tan(TR_Tensor([0.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(tan(0.5), tan(Tensor([0.0, 1.0])).tolist)
tr: yazdır(tan(0.5), tan(Tensör([0.0, 1.0])).değer)
```

İlişkili: `TR_sin(...)`, `TR_cos(...)`, `TR_atan(...)`

#### `TR_tanh(...)` (`TR_fn_tanh`, `tanh`)

    tanh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik tanjanttır; değerleri -1 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (Tensör(...) ile çevrilir).

```tr
TR_print(TR_tanh(0.5), TR_tanh(TR_Tensor([-1.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(tanh(0.5), tanh(Tensor([-1.0, 1.0])).tolist)
tr: yazdır(tanh(0.5), tanh(Tensör([-1.0, 1.0])).değer)
```

İlişkili: `TR_sigmoid(...)`

#### `TR_temp_dir(...)` (`TR_fn_temp_dir`, `geçici_klasör`)

    geçici_klasör() → Yol

İşletim sisteminin geçici klasörünün altında bu program için yeni, boş ve yalnızca kullanıcıya açık bir klasör oluşturur ve Yol olarak döndürür; her çağrı ayrı bir klasör verir. Program bitince klasör içindekilerle birlikte silinir (sil'in korumaları uygulanır, bağlantıların gösterdiği yere dokunulmaz).

```tr
TR_var k = TR_temp_dir(); TR_write_file(TR_Path(k, "not.txt"), "x"); TR_print(TR_Path(k, "not.txt").TR_exists, TR_list_dir(k).TR_length)
```

Görünümü:

```text
en: var k = temp_dir(); write_file(Path(k, "not.txt"), "x"); print(Path(k, "not.txt").exists, list_dir(k).length)
tr: değişken k = geçici_klasör(); dosya_yaz(Yol(k, "not.txt"), "x"); yazdır(Yol(k, "not.txt").var_mı, klasörü_listele(k).uzunluk)
```

İlişkili: `TR_make_dir(...)`, `TR_remove(...)`

#### `TR_time_ms(...)` (`TR_fn_time_ms`, `zaman_ms`)

    zaman_ms() → Tam

Şimdiki zamanı 1970'ten bu yana geçen milisaniye olarak Tam verir; süre ölçmek için kullanılır.

```tr
TR_var baş = TR_time_ms(); TR_sleep(0.05); TR_print(TR_time_ms() - baş >= 50)
```

Görünümü:

```text
en: var baş = time_ms(); sleep(0.05); print(time_ms() - baş >= 50)
tr: değişken baş = zaman_ms(); uyu(0.05); yazdır(zaman_ms() - baş >= 50)
```

İlişkili: `TR_sleep(...)`

#### `TR_topk(...)` (`TR_fn_topk`, `en_büyük_k`)

    en_büyük_k(Tensör, Tam, eksen: Tam = -1) → Liste

Verilen eksende (varsayılan son eksen) en büyük k elemanı büyükten küçüğe [değerler, indeksler] listesi olarak verir: değerler türev izler, indeksler tam sayılı ve türevsizdir. Eşit değerlerde küçük indeks önce gelir, NaN her sayıdan büyüktür; k 0 ile eksen uzunluğu arasında olmalıdır.

```tr
TR_var s = TR_topk(TR_Tensor([3.0, 1.0, 4.0, 1.5]), 2); TR_print(s[0].TR_tolist, s[1].TR_tolist)
```

Görünümü:

```text
en: var s = topk(Tensor([3.0, 1.0, 4.0, 1.5]), 2); print(s[0].tolist, s[1].tolist)
tr: değişken s = en_büyük_k(Tensör([3.0, 1.0, 4.0, 1.5]), 2); yazdır(s[0].değer, s[1].değer)
```

İlişkili: `TR_argsort(...)`, `TR_argmax(...)`

#### `TR_wait_all(...)` (`TR_fn_wait_all`, `bekle_hepsi`)

    bekle_hepsi() → Mantık

Başlatılmış bütün eşzamanlı görevlerin bitmesini bekler ve doğru döndürür; eşzamanlı bloğunun içinden çağrılması hatadır.

```tr
TR_var s = 0; TR_for i TR_in 1..=3 { TR_async { s += i } }; TR_wait_all(); TR_print(s)
```

Görünümü:

```text
en: var s = 0; for i in 1..=3 { async { s += i } }; wait_all(); print(s)
tr: değişken s = 0; döngü i içinde 1..=3 { eşzamanlı { s += i } }; bekle_hepsi(); yazdır(s)
```

İlişkili: `TR_async`, `TR_sleep(...)`

#### `TR_where(...)` (`TR_fn_where`, `koşullu`)

    koşullu(Tensör | Mantık | Tam | Ondalık, Tensör | Tam | Ondalık, Tensör | Tam | Ondalık) → Tensör | Tam | Ondalık

koşullu(koşul, a, b), koşulun doğru (sıfırdan farklı) olduğu elemanlarda a'yı, diğerlerinde b'yi seçer; üçü de yayınlamalıdır, koşul 0/1 maskesi (ör. x > 0), sayı ya da Mantık olabilir. Gradyan a'ya koşulun doğru, b'ye yanlış olduğu yerlerden akar, koşul türev almaz; hiçbiri tensör değilse a ya da b olduğu gibi döner.

```tr
TR_var x = TR_Tensor([-1.0, 2.0, -3.0]); TR_print(TR_where(x > 0, x, 0).TR_tolist)
```

Görünümü:

```text
en: var x = Tensor([-1.0, 2.0, -3.0]); print(where(x > 0, x, 0).tolist)
tr: değişken x = Tensör([-1.0, 2.0, -3.0]); yazdır(koşullu(x > 0, x, 0).değer)
```

İlişkili: `TR_if`

#### `TR_write_file(...)` (`TR_fn_write_file`, `dosya_yaz`)

    dosya_yaz(Metin | Yol, Herhangi) → Mantık

Değeri metne çevirip dosyaya yazar ve doğru döndürür; var olan dosyanın yerine konur. Yazma atomiktir: içerik önce aynı klasörde geçici bir dosyaya yazılıp diske işlenir, sonra eski dosyanın yerine konur (yarıda kalan yazma eski dosyayı bozmaz). .gz uzantılı yola sıkıştırarak yazar.

```tr
TR_write_file("not.txt", "merhaba"); TR_print(TR_read_file("not.txt"))
```

Görünümü:

```text
en: write_file("not.txt", "merhaba"); print(read_file("not.txt"))
tr: dosya_yaz("not.txt", "merhaba"); yazdır(dosya_oku("not.txt"))
```

İlişkili: `TR_read_file(...)`

#### `TR_write_slice(...)` (`TR_fn_write_slice`, `yerine_yaz`)

    yerine_yaz(Tensör, Tensör, eksen: Tam = -1, başlangıç: Tam | Tensör = 0) → YokTipi

Kaynağı hedefin verilen eksenindeki dilime, başlangıç konumundan itibaren yerinde yazar (hedefin öbür eksenleri kaynakla aynı olmalı). Başlangıç tam sayı ya da 0 boyutlu tam sayı tensörüdür; tensör verilirse tekrar_oynat kaydında konum her oynatmada cihazdan okunur (KV önbelleği). Türev izlenen hedefe yalnızca türevsiz içinde yazılır; sonuç yok.

```tr
TR_var t = TR_zeros([2, 4]); TR_write_slice(t, TR_ones([2, 2]), TR_dim: 1, TR_start: 1); TR_print(t.TR_tolist)
```

Görünümü:

```text
en: var t = zeros([2, 4]); write_slice(t, ones([2, 2]), dim: 1, start: 1); print(t.tolist)
tr: değişken t = sıfırlar([2, 4]); yerine_yaz(t, birler([2, 2]), eksen: 1, başlangıç: 1); yazdır(t.değer)
```

İlişkili: `TR_concat(...)`

#### `TR_zero_grads(...)` (`TR_fn_zero_grads`, `gradyanları_sıfırla`)

    gradyanları_sıfırla()



#### `TR_zeros(...)` (`TR_fn_zeros`, `sıfırlar`)

    sıfırlar(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 0 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir; sıfırlar(2, 3) yazımı hatadır ve liste biçimini önerir.

```tr
TR_print(TR_zeros([2, 3]).TR_shape)
```

Görünümü:

```text
en: print(zeros([2, 3]).shape)
tr: yazdır(sıfırlar([2, 3]).şekil)
```

İlişkili: `TR_ones(...)`, `TR_Tensor`

### Yerleşik sabitler

#### `TR_BLOCK_NAMES` (`TR_const_BLOCK_NAMES`, `BLOK_ADLARI`)

    BLOK_ADLARI



#### `TR_DRY_RUN_STEPS` (`TR_const_DRY_RUN_STEPS`, `PROVA_ADIMI`)

    PROVA_ADIMI



#### `TR_MIN_RELATIVE_STD` (`TR_const_MIN_RELATIVE_STD`, `EN_AZ_GÖRELİ_SAPMA`)

    EN_AZ_GÖRELİ_SAPMA



#### `TR_NONFINITE` (`TR_const_NONFINITE`, `SONLU_DEĞİL`)

    SONLU_DEĞİL



#### `TR_NS_A` (`TR_const_NS_A`, `NS_A`)

    NS_A



#### `TR_NS_B` (`TR_const_NS_B`, `NS_B`)

    NS_B



#### `TR_NS_C` (`TR_const_NS_C`, `NS_C`)

    NS_C



#### `TR_OK` (`TR_const_OK`, `TAMAM`)

    TAMAM



#### `TR_SPIKE` (`TR_const_SPIKE`, `SIÇRAMA`)

    SIÇRAMA



#### `TR_SUPPORTED_ARCHITECTURES` (`TR_const_SUPPORTED_ARCHITECTURES`, `DESTEKLENEN_MİMARİLER`)

    DESTEKLENEN_MİMARİLER



#### `TR_pi` (`TR_const_pi`, `pi`)

    pi → Ondalık

Bir çemberin çevresinin çapına oranı olan π sayısıdır (Ondalık, 3.141592653589793); çağrılmaz, değer olarak yazılır. sin, cos ve tan radyanla çalışır: 180 derece pi radyandır.

```tr
TR_print(TR_pi, TR_sin(TR_pi / 2), TR_cos(TR_pi))
```

Görünümü:

```text
en: print(pi, sin(pi / 2), cos(pi))
tr: yazdır(pi, sin(pi / 2), cos(pi))
```

İlişkili: `TR_sin(...)`, `TR_tan(...)`

### Adlı argümanlar

#### `TR_G_muon:` (`TR_arg_G_muon`, `G`)



#### `TR_T:` (`TR_arg_T`, `T`)



#### `TR_adamw_lr:` (`TR_arg_adamw_lr`, `adamw_öğrenme_oranı`)



#### `TR_adamw_weight_decay:` (`TR_arg_adamw_weight_decay`, `adamw_çürümesi`)



#### `TR_approximate:` (`TR_arg_approximate`, `yaklaşık`)

gelu'nun hangi biçimde hesaplanacağını seçer: doğru (varsayılan) tanh yaklaşımı, yanlış tam (erf tabanlı) GELU. Mantık olmalıdır.

```tr
TR_print(TR_gelu(TR_Tensor([-1.0, 0.0, 1.0]), TR_approximate: TR_false).TR_tolist)
```

Görünümü:

```text
en: print(gelu(Tensor([-1.0, 0.0, 1.0]), approximate: false).tolist)
tr: yazdır(gelu(Tensör([-1.0, 0.0, 1.0]), yaklaşık: yanlış).değer)
```

Kullananlar: `TR_gelu(...)` · İlişkili: `TR_gelu(...)`

#### `TR_backward:` (`TR_arg_backward`, `geri`)

özel_türev'in geri hesabıdır: (saklananlar, g) => ... biçiminde iki parametre alır ve her konumlu girdi için girdiyle aynı şekilde bir gradyan ya da yok döndürür.

```tr
TR_var f = TR_custom_grad(TR_forward: (x) => [x * 3, []], TR_backward: (s, g) => [g * 3]); TR_var x = TR_Tensor([2.0], TR_requires_grad: TR_true); TR_backward(TR_sum(f(x))); TR_print(x.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var f = custom_grad(forward: (x) => [x * 3, []], backward: (s, g) => [g * 3]); var x = Tensor([2.0], requires_grad: true); backward(sum(f(x))); print(x.grad.tolist)
tr: değişken f = özel_türev(ileri: (x) => [x * 3, []], geri: (s, g) => [g * 3]); değişken x = Tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(f(x))); yazdır(x.gradyan.değer)
```

Kullananlar: `TR_custom_grad(...)` · İlişkili: `TR_forward:`

#### `TR_batch_size:` (`TR_arg_batch_size`, `toplu`)

Okuyucunun her .sonraki() çağrısında verdiği satır (toplu iş) sayısıdır. Pozitif bir tam sayı olmalıdır ve verilmesi zorunludur.

```tr
TR_var ö = TR_Dataset("tests/data/numbers").TR_sampler(TR_batch_size: 8, TR_context: 16)
```

Görünümü:

```text
en: var ö = Dataset("tests/data/numbers").sampler(batch_size: 8, context: 16)
tr: değişken ö = VeriKümesi("tests/data/numbers").örnekleyici(toplu: 8, bağlam: 16)
```

Kullananlar: `.TR_sampler(...)`, `.TR_sequential(...)` · İlişkili: `TR_context:`

#### `TR_beta1:` (`TR_arg_beta1`, `beta1`)

AdamW'nin gradyan ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.9'dur.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_print(TR_AdamW([w], TR_beta1: 0.8))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); print(AdamW([w], beta1: 0.8))
tr: değişken w = Tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta1: 0.8))
```

Kullananlar: `TR_AdamW(...)` · İlişkili: `TR_beta2:`

#### `TR_beta2:` (`TR_arg_beta2`, `beta2`)

AdamW'nin gradyan karelerinin ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.999'dur.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_print(TR_AdamW([w], TR_beta2: 0.99))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); print(AdamW([w], beta2: 0.99))
tr: değişken w = Tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta2: 0.99))
```

Kullananlar: `TR_AdamW(...)` · İlişkili: `TR_beta1:`

#### `TR_bias:` (`TR_arg_bias`, `sapma`)

katman_normu sonucuna son eksende eklenen [D] şeklindeki kayma tensörüdür; yok verilirse eklenmez. Konumlu da verilebilir.

```tr
TR_print(TR_layer_norm(TR_Tensor([[1.0, 3.0]]), TR_bias: TR_Tensor([0.5, 0.5])).TR_tolist)
```

Görünümü:

```text
en: print(layer_norm(Tensor([[1.0, 3.0]]), bias: Tensor([0.5, 0.5])).tolist)
tr: yazdır(katman_normu(Tensör([[1.0, 3.0]]), sapma: Tensör([0.5, 0.5])).değer)
```

Kullananlar: `TR_layer_norm(...)` · İlişkili: `TR_weight:`

#### `TR_buffers:` (`TR_arg_buffers`, `tamponlar`)



#### `TR_cache:` (`TR_arg_cache`, `önbellek`)

veri_kümesi'nin veri dosyalarını kullanmadan önce kopyalayacağı yerel önbellek klasörüdür; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. Tek başına verilemez: kota_gb: zorunludur.

```tr
TR_var küme = TR_Dataset("tests/data/numbers", TR_cache: "onbellek/veri", TR_quota_gb: 1)
```

Görünümü:

```text
en: var küme = Dataset("tests/data/numbers", cache: "onbellek/veri", quota_gb: 1)
tr: değişken küme = VeriKümesi("tests/data/numbers", önbellek: "onbellek/veri", kota_gb: 1)
```

Kullananlar: `TR_Dataset` · İlişkili: `TR_quota_gb:`

#### `TR_cache_kind:` (`TR_arg_cache_kind`, `önbellek_çeşidi`)



#### `TR_capacity:` (`TR_arg_capacity`, `kapasite`)



#### `TR_causal:` (`TR_arg_causal`, `nedensel`)

doğru iken (varsayılan) her sorgu yalnızca kendisine kadarki anahtarlara bakar; Tq < Tk iken hizalama sondandır. yanlış ile dikkat iki yönlüdür.

```tr
TR_var q = TR_randn([4, 8], TR_seed: 1); TR_print(TR_attention(q, q, q, TR_causal: TR_false).TR_shape)
```

Görünümü:

```text
en: var q = randn([4, 8], seed: 1); print(attention(q, q, q, causal: false).shape)
tr: değişken q = rastgele_normal([4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış).şekil)
```

Kullananlar: `TR_attention(...)` · İlişkili: `TR_window:`, `TR_mask:`

#### `TR_config:` (`TR_arg_config`, `ayar`)



#### `TR_context:` (`TR_arg_context`, `bağlam`)

Okuyucunun her satırdaki pencere uzunluğudur; girdi ve hedef [toplu, bağlam] şeklinde olur. Zorunludur ve en uzun veri dosyasından kısa olmalıdır.

```tr
TR_var s = TR_Dataset("tests/data/numbers").TR_sequential(TR_batch_size: 4, TR_context: 16)
```

Görünümü:

```text
en: var s = Dataset("tests/data/numbers").sequential(batch_size: 4, context: 16)
tr: değişken s = VeriKümesi("tests/data/numbers").sıralı(toplu: 4, bağlam: 16)
```

Kullananlar: `.TR_sampler(...)`, `.TR_sequential(...)` · İlişkili: `TR_batch_size:`

#### `TR_cos_table:` (`TR_arg_cos_table`, `cos_tablosu`)



#### `TR_cuda_graph:` (`TR_arg_cuda_graph`, `cuda_çizgesi`)



#### `TR_decay_fraction:` (`TR_arg_decay_fraction`, `azalma`)



#### `TR_description:` (`TR_arg_description`, `açıklama`)

komut_satırı'nın --help çıktısının başında gösterilen, programın ne yaptığını anlatan metindir; varsayılan boştur.

```tr
TR_var ayar = TR_command_line({"adım": 10}, TR_description: "Modeli eğitir"); TR_print(ayar["adım"])
```

Görünümü:

```text
en: var ayar = command_line({"adım": 10}, description: "Modeli eğitir"); print(ayar["adım"])
tr: değişken ayar = komut_satırı({"adım": 10}, açıklama: "Modeli eğitir"); yazdır(ayar["adım"])
```

Kullananlar: `TR_command_line(...)` · İlişkili: `TR_descriptions:`

#### `TR_descriptions:` (`TR_arg_descriptions`, `açıklamalar`)

komut_satırı seçeneklerinin --help çıktısındaki açıklamalarıdır: anahtarlar seçenek adları, değerler Metin olmalıdır; seçeneklerde olmayan bir ad hatadır.

```tr
TR_var ayar = TR_command_line({"adım": 10}, TR_descriptions: {"adım": "eğitim adımı sayısı"}); TR_print(ayar["adım"])
```

Görünümü:

```text
en: var ayar = command_line({"adım": 10}, descriptions: {"adım": "eğitim adımı sayısı"}); print(ayar["adım"])
tr: değişken ayar = komut_satırı({"adım": 10}, açıklamalar: {"adım": "eğitim adımı sayısı"}); yazdır(ayar["adım"])
```

Kullananlar: `TR_command_line(...)` · İlişkili: `TR_description:`

#### `TR_device:` (`TR_arg_device`, `cihaz`)



#### `TR_dim:` (`TR_arg_dim`, `eksen`)

İşlemin uygulanacağı ekseni seçer; negatif değer sondan sayar (-1 son eksen). İndirgemelerde verilmezse bütün elemanlar, softmax, bitiştir ve argmax'te son eksen kullanılır.

```tr
TR_print(TR_sum(TR_Tensor([[1, 2], [3, 4]]), TR_dim: 0).TR_tolist)
```

Görünümü:

```text
en: print(sum(Tensor([[1, 2], [3, 4]]), dim: 0).tolist)
tr: yazdır(toplam(Tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

Kullananlar: `TR_argmax(...)`, `TR_argsort(...)`, `TR_concat(...)`, `TR_cumsum(...)`, `TR_log_softmax(...)`, `TR_max(...)`, `TR_mean(...)`, `TR_min(...)`, `TR_softmax(...)`, `TR_sum(...)`, `TR_topk(...)`, `TR_write_slice(...)`, `.TR_split(...)` · İlişkili: `TR_keepdim:`

#### `TR_dry_run:` (`TR_arg_dry_run`, `prova`)



#### `TR_dtype:` (`TR_arg_dtype`, `tür`)

Oluşturulan tensörün veri türünü seçer: "float32" (varsayılan), "bf16" ya da "fp16". Normlarda (katman_normu, rms_normu) çıktının türüdür; hesap yine float32'dir, verilmezse çıktı girdinin türündedir. Başka bir ad en yakın geçerli adı öneren bir hatadır.

```tr
TR_print(TR_zeros([2, 2], TR_dtype: "fp16").TR_dtype)
```

Görünümü:

```text
en: print(zeros([2, 2], dtype: "fp16").dtype)
tr: yazdır(sıfırlar([2, 2], tür: "fp16").tür)
```

Kullananlar: `TR_Tensor`, `TR_layer_norm(...)`, `TR_ones(...)`, `TR_randn(...)`, `TR_rms_norm(...)`, `TR_zeros(...)`

#### `TR_embed_dim:` (`TR_arg_embed_dim`, `boyut`)



#### `TR_end:` (`TR_arg_end`, `bitiş`)



#### `TR_eos:` (`TR_arg_eos`, `eos`)



#### `TR_eps:` (`TR_arg_eps`, `epsilon`)

Sıfıra bölmeyi önlemek için paydaya eklenen küçük pozitif sayıdır. Normlarda varsayılan 0.00001, AdamW'de 1e-8'dir.

```tr
TR_print(TR_layer_norm(TR_Tensor([[1.0, 2.0, 3.0]]), TR_eps: 0.001).TR_tolist)
```

Görünümü:

```text
en: print(layer_norm(Tensor([[1.0, 2.0, 3.0]]), eps: 0.001).tolist)
tr: yazdır(katman_normu(Tensör([[1.0, 2.0, 3.0]]), epsilon: 0.001).değer)
```

Kullananlar: `TR_AdamW(...)`, `TR_layer_norm(...)`, `TR_rms_norm(...)`

#### `TR_error_if_nonfinite:` (`TR_arg_error_if_nonfinite`, `sonlu_değilse_hata`)

gradyan_kırp'ın seçeneğidir (Mantık, varsayılan yanlış): doğruysa gradyanların toplam normu NaN ya da sonsuzsa hata verir (PyTorch clip_grad_norm_(error_if_nonfinite=True)). Normu okumak için GPU'yu bekler; varsayılanda bekleme yoktur. Adımı atlamak isteyen eğitim bunun yerine sonlu_mu(norm.değer) ile denetler.

```tr
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true); TR_backward(TR_sum(w * TR_Float("inf"))); TR_try { TR_clip_grad([w], TR_error_if_nonfinite: TR_true) } TR_catch h { TR_print("sonlu değil") }
```

Görünümü:

```text
en: var w = Tensor([1.0, 2.0], requires_grad: true); backward(sum(w * Float("inf"))); try { clip_grad([w], error_if_nonfinite: true) } catch h { print("sonlu değil") }
tr: değişken w = Tensör([1.0, 2.0], türev_izle: doğru); geri_yayılım(toplam(w * Ondalık("inf"))); dene { gradyan_kırp([w], sonlu_değilse_hata: doğru) } yakala h { yazdır("sonlu değil") }
```

Kullananlar: `TR_clip_grad(...)` · İlişkili: `TR_clip_grad(...)`, `TR_is_finite(...)`

#### `TR_every_minutes:` (`TR_arg_every_minutes`, `her_dakika`)

kontrol_noktası'nın kayıt aralığıdır (dakika, varsayılan 15; 0: her güncelle'de). Süre dolunca sıradaki .güncelle(adım) çağrısı adım sınırında kaydeder; durdurma isteğinde (Ctrl+C) aralık beklenmez.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var kn = TR_Checkpoint("koşu_t984", {"w": w}, TR_every_minutes: 0.5); TR_print(kn)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var kn = Checkpoint("koşu_t984", {"w": w}, every_minutes: 0.5); print(kn)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken kn = KontrolNoktası("koşu_t984", {"w": w}, her_dakika: 0.5); yazdır(kn)
```

Kullananlar: `TR_Checkpoint` · İlişkili: `TR_keep:`

#### `TR_fallback:` (`TR_arg_fallback`, `yedek`)



#### `TR_fd_step:` (`TR_arg_fd_step`, `fark_adımı`)

türev_denetle'de sayısal gradyanı veren merkezi farkın adım büyüklüğüdür (h): (f(x + h) - f(x - h)) / 2h. Pozitif olmalıdır; varsayılan 0.001'dir.

```tr
TR_print(TR_grad_check((x) => x * x, [TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)], TR_fd_step: 0.01))
```

Görünümü:

```text
en: print(grad_check((x) => x * x, [Tensor([1.0, 2.0], requires_grad: true)], fd_step: 0.01))
tr: yazdır(türev_denetle((x) => x * x, [Tensör([1.0, 2.0], türev_izle: doğru)], fark_adımı: 0.01))
```

Kullananlar: `TR_grad_check(...)` · İlişkili: `TR_tol:`

#### `TR_ff_use_bias:` (`TR_arg_ff_use_bias`, `ileri_sapmalı`)



#### `TR_folder:` (`TR_arg_folder`, `klasör`)



#### `TR_forward:` (`TR_arg_forward`, `ileri`)

özel_türev'in ileri hesabıdır: türevsiz çalışır ve [çıktı, saklananlar] döndürmelidir. Çıktı bir tensör ya da tensör listesidir.

```tr
TR_var f = TR_custom_grad(TR_forward: (x) => [x * 3, []], TR_backward: (s, g) => [g * 3]); TR_print(f(TR_Tensor([2.0])).TR_tolist)
```

Görünümü:

```text
en: var f = custom_grad(forward: (x) => [x * 3, []], backward: (s, g) => [g * 3]); print(f(Tensor([2.0])).tolist)
tr: değişken f = özel_türev(ileri: (x) => [x * 3, []], geri: (s, g) => [g * 3]); yazdır(f(Tensör([2.0])).değer)
```

Kullananlar: `TR_custom_grad(...)` · İlişkili: `TR_backward:`

#### `TR_g_swiglu:` (`TR_arg_g_swiglu`, `g`)



#### `TR_gate:` (`TR_arg_gate`, `kapı`)



#### `TR_gu:` (`TR_arg_gu`, `gu`)



#### `TR_h:` (`TR_arg_h`, `h`)



#### `TR_head_dim:` (`TR_arg_head_dim`, `bakış_boyutu`)



#### `TR_history:` (`TR_arg_history`, `geçmiş`)



#### `TR_history_limit:` (`TR_arg_history_limit`, `geçmiş_sınırı`)



#### `TR_in_features:` (`TR_arg_in_features`, `giriş_boyutu`)



#### `TR_init_std:` (`TR_arg_init_std`, `başlatma_sapması`)



#### `TR_intermediate_dim:` (`TR_arg_intermediate_dim`, `ara_boyut`)



#### `TR_k:` (`TR_arg_k`, `k`)



#### `TR_keep:` (`TR_arg_keep`, `tut`)

kontrol_noktası'nın saklayacağı en yeni nokta sayısıdır (pozitif Tam, varsayılan 2); daha eskileri her başarılı kayıttan sonra silinir.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var kn = TR_Checkpoint("koşu_t985", {"w": w}, TR_keep: 3); TR_for i TR_in 1..5 { kn.TR_update(i); kn.TR_save() }; TR_print(TR_list_dir("koşu_t985/checkpoints", "step_*").TR_length)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var kn = Checkpoint("koşu_t985", {"w": w}, keep: 3); for i in 1..5 { kn.update(i); kn.save() }; print(list_dir("koşu_t985/checkpoints", "step_*").length)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken kn = KontrolNoktası("koşu_t985", {"w": w}, tut: 3); döngü i içinde 1..5 { kn.güncelle(i); kn.kaydet() }; yazdır(klasörü_listele("koşu_t985/checkpoints", "step_*").uzunluk)
```

Kullananlar: `TR_Checkpoint` · İlişkili: `TR_every_minutes:`

#### `TR_keepdim:` (`TR_arg_keepdim`, `boyutu_koru`)

doğru verilince indirgenen eksen 1 uzunlukta korunur, böylece sonuç girdiye yayınlanabilir; eksen verilmezse bütün eksenler 1 olur. Mantık olmalıdır ve yalnızca tensörlerde geçerlidir.

```tr
TR_print(TR_sum(TR_Tensor([[1, 2], [3, 4]]), TR_dim: -1, TR_keepdim: TR_true).TR_shape)
```

Görünümü:

```text
en: print(sum(Tensor([[1, 2], [3, 4]]), dim: -1, keepdim: true).shape)
tr: yazdır(toplam(Tensör([[1, 2], [3, 4]]), eksen: -1, boyutu_koru: doğru).şekil)
```

Kullananlar: `TR_max(...)`, `TR_mean(...)`, `TR_min(...)`, `TR_sum(...)` · İlişkili: `TR_dim:`

#### `TR_key:` (`TR_arg_key`, `anahtar`)



#### `TR_kind:` (`TR_arg_kind`, `çeşit`)



#### `TR_layer_idx:` (`TR_arg_layer_idx`, `katman_no`)



#### `TR_layout:` (`TR_arg_layout`, `düzen`)



#### `TR_log:` (`TR_arg_log`, `günlük`)

ilerleme çubuğunun her güncellemesini (adım, ölçüler, hız) ve bitişini verilen Günlük'e de yazar; varsayılan yok.

```tr
TR_var g = TR_Logger("kayit/ilerleme.jsonl"); TR_var ç = TR_Progress(2, TR_log: g); ç.TR_update({"kayıp": 0.9}); ç.TR_update({"kayıp": 0.7})
```

Görünümü:

```text
en: var g = Logger("kayit/ilerleme.jsonl"); var ç = Progress(2, log: g); ç.update({"kayıp": 0.9}); ç.update({"kayıp": 0.7})
tr: değişken g = Günlük("kayit/ilerleme.jsonl"); değişken ç = İlerleme(2, günlük: g); ç.güncelle({"kayıp": 0.9}); ç.güncelle({"kayıp": 0.7})
```

Kullananlar: `TR_Progress` · İlişkili: `TR_Logger`

#### `TR_loss:` (`TR_arg_loss`, `kayıp`)



#### `TR_lr:` (`TR_arg_lr`, `öğrenme_oranı`)

İyileştiricinin her adımda parametreleri ne kadar değiştireceğini belirleyen orandır. Sonlu ve negatif olmayan bir sayı olmalıdır; varsayılan AdamW'de 0.001, SGD'de 0.01'dir.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var opt = TR_SGD([w], TR_lr: 0.5); TR_backward(TR_sum(w * w)); opt.TR_step(); TR_print(w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var opt = SGD([w], lr: 0.5); backward(sum(w * w)); opt.step(); print(w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.5); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: `TR_AdamW(...)`, `TR_SGD(...)` · İlişkili: `TR_weight_decay:`, `.TR_lr`

#### `TR_mask:` (`TR_arg_mask`, `maske`)

[..., H, Tq, Tk] şekline yayınlanabilen 0/1 tensörüdür; sıfırdan farklı yerlere bakılır. Türev izlemez; varsayılan yok'tur.

```tr
TR_var q = TR_randn([3, 4], TR_seed: 1); TR_print(TR_attention(q, q, q, TR_causal: TR_false, TR_mask: TR_Tensor([[1, 1, 0], [1, 1, 0], [1, 1, 1]])).TR_shape)
```

Görünümü:

```text
en: var q = randn([3, 4], seed: 1); print(attention(q, q, q, causal: false, mask: Tensor([[1, 1, 0], [1, 1, 0], [1, 1, 1]])).shape)
tr: değişken q = rastgele_normal([3, 4], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış, maske: Tensör([[1, 1, 0], [1, 1, 0], [1, 1, 1]])).şekil)
```

Kullananlar: `TR_attention(...)` · İlişkili: `TR_causal:`

#### `TR_master_weights:` (`TR_arg_master_weights`, `ana_kopya`)

doğru iken (varsayılan) 16 bitlik parametrelerin float32 ana kopyası tutulur; güncelleme orada yapılıp parametreye yuvarlanır, böylece küçük adımlar kaybolmaz. float32 parametrelerde etkisizdir.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true, TR_dtype: "bf16"); TR_var opt = TR_AdamW([w], TR_master_weights: TR_true); TR_backward(TR_sum(w * w)); opt.TR_step(); TR_print(w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true, dtype: "bf16"); var opt = AdamW([w], master_weights: true); backward(sum(w * w)); opt.step(); print(w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru, tür: "bf16"); değişken opt = AdamW([w], ana_kopya: doğru); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: `TR_AdamW(...)`, `TR_SGD(...)`

#### `TR_max:` (`TR_arg_max`, `en_fazla`)

gradyan_kırp için izin verilen en büyük toplam gradyan normudur (pozitif bir sayı, varsayılan 1.0); sınırla için üst sınırdır (sayı; en_az'dan küçük olamaz).

```tr
TR_var w = TR_Tensor([3.0, 4.0], TR_requires_grad: TR_true); TR_backward(TR_sum(w * w)); TR_print(TR_clip_grad([w], TR_max: 0.5))
```

Görünümü:

```text
en: var w = Tensor([3.0, 4.0], requires_grad: true); backward(sum(w * w)); print(clip_grad([w], max: 0.5))
tr: değişken w = Tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 0.5))
```

Kullananlar: `TR_clamp(...)`, `TR_clip_grad(...)` · İlişkili: `TR_min:`

#### `TR_max_new_tokens:` (`TR_arg_max_new_tokens`, `en_fazla_yeni_parça`)



#### `TR_merged:` (`TR_arg_merged`, `birleşik`)



#### `TR_merged_gate:` (`TR_arg_merged_gate`, `birleşik_kapı`)



#### `TR_merged_kv:` (`TR_arg_merged_kv`, `birleşik_kv`)



#### `TR_metrics:` (`TR_arg_metrics`, `ölçüler`)



#### `TR_min:` (`TR_arg_min`, `en_az`)

sınırla için alt sınırdır: değerden küçük elemanlar bu değere çekilir. Bir sayı olmalıdır ve en_fazla'dan büyük olamaz; en_az ile en_fazla'dan en az biri verilmelidir.

```tr
TR_print(TR_clamp(-3.5, TR_min: 0.0), TR_clamp(TR_Tensor([-1.0, 2.0]), TR_min: 0).TR_tolist)
```

Görünümü:

```text
en: print(clamp(-3.5, min: 0.0), clamp(Tensor([-1.0, 2.0]), min: 0).tolist)
tr: yazdır(sınırla(-3.5, en_az: 0.0), sınırla(Tensör([-1.0, 2.0]), en_az: 0).değer)
```

Kullananlar: `TR_clamp(...)` · İlişkili: `TR_max:`

#### `TR_model:` (`TR_arg_model`, `model`)



#### `TR_momentum:` (`TR_arg_momentum`, `momentum`)

SGD'nin önceki adımların yönünü sürdürme katsayısıdır; varsayılan 0'dır (momentum yok).

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_print(TR_SGD([w], TR_momentum: 0.9))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); print(SGD([w], momentum: 0.9))
tr: değişken w = Tensör([1.0], türev_izle: doğru); yazdır(SGD([w], momentum: 0.9))
```

Kullananlar: `TR_SGD(...)`

#### `TR_multiplier:` (`TR_arg_multiplier`, `çarpan`)



#### `TR_muon_lr:` (`TR_arg_muon_lr`, `muon_öğrenme_oranı`)



#### `TR_muon_scale:` (`TR_arg_muon_scale`, `muon_ölçeği`)



#### `TR_n:` (`TR_arg_n`, `n`)



#### `TR_name:` (`TR_arg_name`, `ad`)



#### `TR_named:` (`TR_arg_named`, `adlarıyla`)

doğru verilince parametreler liste yerine "bloklar.0.Wq" gibi yol adı → tensör sözlüğü döndürür. Mantık olmalıdır; varsayılan yanlış'tır.

```tr
TR_print(TR_parameters([TR_Tensor([1.0], TR_requires_grad: TR_true)], TR_named: TR_true))
```

Görünümü:

```text
en: print(parameters([Tensor([1.0], requires_grad: true)], named: true))
tr: yazdır(parametreler([Tensör([1.0], türev_izle: doğru)], adlarıyla: doğru))
```

Kullananlar: `TR_parameters(...)`

#### `TR_nesterov:` (`TR_arg_nesterov`, `nesterov`)



#### `TR_new_chunk:` (`TR_arg_new_chunk`, `yeni`)



#### `TR_norm:` (`TR_arg_norm`, `norm`)



#### `TR_normuon:` (`TR_arg_normuon`, `normuon`)



#### `TR_ns_dtype:` (`TR_arg_ns_dtype`, `ns_türü`)



#### `TR_ns_iterations:` (`TR_arg_ns_iterations`, `ns_yineleme_sayısı`)



#### `TR_num_heads:` (`TR_arg_num_heads`, `bakış_sayısı`)



#### `TR_num_kv_heads:` (`TR_arg_num_kv_heads`, `kv_bakış_sayısı`)



#### `TR_num_layers:` (`TR_arg_num_layers`, `katman_sayısı`)



#### `TR_num_steps:` (`TR_arg_num_steps`, `adım_sayısı`)



#### `TR_objects:` (`TR_arg_objects`, `nesneler`)



#### `TR_out_features:` (`TR_arg_out_features`, `çıkış_boyutu`)



#### `TR_output_dtype:` (`TR_arg_output_dtype`, `çıktı_türü`)



#### `TR_output_init_std:` (`TR_arg_output_init_std`, `çıkış_başlatma_sapması`)



#### `TR_overwrite:` (`TR_arg_overwrite`, `üzerine_yaz`)

kopyala ve taşı'da hedef zaten varsa kaynağın onun yerine konmasına izin verir; varsayılan yanlıştır (var olan hedef hatadır). Bir dosya bir klasörün, bir klasör bir dosyanın yerine konmaz.

```tr
TR_write_file("kaynak.txt", "yeni"); TR_write_file("hedef.txt", "eski"); TR_copy("kaynak.txt", "hedef.txt", TR_overwrite: TR_true); TR_print(TR_read_file("hedef.txt"))
```

Görünümü:

```text
en: write_file("kaynak.txt", "yeni"); write_file("hedef.txt", "eski"); copy("kaynak.txt", "hedef.txt", overwrite: true); print(read_file("hedef.txt"))
tr: dosya_yaz("kaynak.txt", "yeni"); dosya_yaz("hedef.txt", "eski"); kopyala("kaynak.txt", "hedef.txt", üzerine_yaz: doğru); yazdır(dosya_oku("hedef.txt"))
```

Kullananlar: `TR_copy(...)`, `TR_move(...)` · İlişkili: `TR_copy(...)`, `TR_move(...)`

#### `TR_pair:` (`TR_arg_pair`, `ikinci`)

kodla'ya ikinci bir metin vererek çift kodlama yapar. Liste kodlanırken aynı uzunlukta bir metin listesi olmalıdır.

```tr
TR_print(TR_load_tokenizer("tests/data/tokenizer.json").TR_encode("soru", TR_pair: "cevap"))
```

Görünümü:

```text
en: print(load_tokenizer("tests/data/tokenizer.json").encode("soru", pair: "cevap"))
tr: yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("soru", ikinci: "cevap"))
```

Kullananlar: `.TR_encode(...)`

#### `TR_parameters:` (`TR_arg_parameters`, `parametreler`)



#### `TR_pattern:` (`TR_arg_pattern`, `desen`)



#### `TR_patterns:` (`TR_arg_patterns`, `desenler`)



#### `TR_ple_context:` (`TR_arg_ple_context`, `ple_bağlamı`)



#### `TR_ple_contexts:` (`TR_arg_ple_contexts`, `ple_bağlamları`)



#### `TR_ple_dim:` (`TR_arg_ple_dim`, `ple_boyutu`)



#### `TR_position:` (`TR_arg_position`, `konum`)



#### `TR_prompt:` (`TR_arg_prompt`, `istem`)



#### `TR_qk_norm:` (`TR_arg_qk_norm`, `qk_normu`)



#### `TR_quota_gb:` (`TR_arg_quota_gb`, `kota_gb`)

Önbellek klasörünün kullanabileceği en çok disk alanıdır (GB). Pozitif bir sayı olmalıdır ve yalnızca önbellek: ile birlikte geçerlidir.

```tr
TR_var küme = TR_Dataset("tests/data/numbers", TR_cache: "onbellek/veri", TR_quota_gb: 0.5)
```

Görünümü:

```text
en: var küme = Dataset("tests/data/numbers", cache: "onbellek/veri", quota_gb: 0.5)
tr: değişken küme = VeriKümesi("tests/data/numbers", önbellek: "onbellek/veri", kota_gb: 0.5)
```

Kullananlar: `TR_Dataset` · İlişkili: `TR_cache:`

#### `TR_recursive:` (`TR_arg_recursive`, `içindekilerle`)

sil'de dolu bir klasörün içindekilerle birlikte silinmesine izin verir; varsayılan yanlıştır (yalnızca boş klasör silinir). İçerideki bağlantıların gösterdiği yerlere dokunulmaz; salt okunur bir dosya varsa hiçbir şey silinmez.

```tr
TR_make_dir("çöp/alt"); TR_write_file("çöp/alt/a.txt", "x"); TR_print(TR_remove("çöp", TR_recursive: TR_true))
```

Görünümü:

```text
en: make_dir("çöp/alt"); write_file("çöp/alt/a.txt", "x"); print(remove("çöp", recursive: true))
tr: klasör_oluştur("çöp/alt"); dosya_yaz("çöp/alt/a.txt", "x"); yazdır(sil("çöp", içindekilerle: doğru))
```

Kullananlar: `TR_remove(...)` · İlişkili: `TR_remove(...)`

#### `TR_requires_grad:` (`TR_arg_requires_grad`, `türev_izle`)

Oluşturulan tensörün türevinin izlenip izlenmeyeceğini belirler; varsayılan yanlıştır. Parametreler ve gradyanı istenen girdiler türev_izle: doğru ile oluşturulmalıdır.

```tr
TR_var x = TR_Tensor([2.0], TR_requires_grad: TR_true); TR_backward(TR_sum(x * x)); TR_print(x.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var x = Tensor([2.0], requires_grad: true); backward(sum(x * x)); print(x.grad.tolist)
tr: değişken x = Tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(x * x)); yazdır(x.gradyan.değer)
```

Kullananlar: `TR_Tensor`, `TR_load_tensors(...)`, `TR_ones(...)`, `TR_randn(...)`, `TR_zeros(...)`

#### `TR_rope_theta:` (`TR_arg_rope_theta`, `rope_tabanı`)



#### `TR_row:` (`TR_arg_row`, `satır`)



#### `TR_scale:` (`TR_arg_scale`, `ölçek`)



#### `TR_seed:` (`TR_arg_seed`, `tohum`)

Rastgele sayı üretecinin başlangıç değeridir (tam sayı); aynı tohum her çalıştırmada aynı sonucu verir. Verilmezse paylaşılan bir üreteç kullanılır.

```tr
TR_print(TR_randn([3], TR_seed: 7).TR_tolist)
```

Görünümü:

```text
en: print(randn([3], seed: 7).tolist)
tr: yazdır(rastgele_normal([3], tohum: 7).değer)
```

Kullananlar: `TR_randn(...)`, `TR_sample(...)`, `.TR_sampler(...)`

#### `TR_separated:` (`TR_arg_separated`, `ayrılan`)



#### `TR_shape:` (`TR_arg_shape`, `şekil`)



#### `TR_sin_table:` (`TR_arg_sin_table`, `sin_tablosu`)



#### `TR_sparse:` (`TR_arg_sparse`, `seyrek`)



#### `TR_sparse_lr:` (`TR_arg_sparse_lr`, `seyrek_öğrenme_oranı`)



#### `TR_special_tokens:` (`TR_arg_special_tokens`, `özel_parçalar`)

Özel parçaların kodlamaya eklenip eklenmeyeceğini (kodla, varsayılan doğru) ya da çözülen metne yazılıp yazılmayacağını (çöz, varsayılan yanlış) belirler. Yalnızca doğru ya da yanlış olabilir.

```tr
TR_print(TR_load_tokenizer("tests/data/tokenizer.json").TR_encode("merhaba dünya", TR_special_tokens: TR_false))
```

Görünümü:

```text
en: print(load_tokenizer("tests/data/tokenizer.json").encode("merhaba dünya", special_tokens: false))
tr: yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("merhaba dünya", özel_parçalar: yanlış))
```

Kullananlar: `.TR_decode(...)`, `.TR_encode(...)`

#### `TR_start:` (`TR_arg_start`, `başlangıç`)

yerine_yaz'ın hedef eksende yazmaya başladığı konumdur: tam sayı ya da 0 boyutlu tam sayı tensörü. Kaynak hedefe sığmalıdır; tensörle verilen konum tekrar_oynat kaydında her oynatmada cihazdan okunur.

```tr
TR_var t = TR_zeros([3]); TR_write_slice(t, TR_ones([1]), TR_dim: 0, TR_start: 2); TR_print(t.TR_tolist)
```

Görünümü:

```text
en: var t = zeros([3]); write_slice(t, ones([1]), dim: 0, start: 2); print(t.tolist)
tr: değişken t = sıfırlar([3]); yerine_yaz(t, birler([1]), eksen: 0, başlangıç: 2); yazdır(t.değer)
```

Kullananlar: `TR_write_slice(...)` · İlişkili: `TR_write_slice(...)`

#### `TR_step:` (`TR_arg_step`, `adım`)

İlerleme.güncelle'de tamamlanan eğitim adımı sayısıdır (kaldığı yerden sürdürmek için; 0 ya da pozitif Tam).

```tr
TR_var ç = TR_Progress(100); ç.TR_update(TR_step: 40); ç.TR_stop()
```

Görünümü:

```text
en: var ç = Progress(100); ç.update(step: 40); ç.stop()
tr: değişken ç = İlerleme(100); ç.güncelle(adım: 40); ç.durdur()
```

Kullananlar: `.TR_update(...)` · İlişkili: `.TR_update(...)`

#### `TR_targets:` (`TR_arg_targets`, `hedef`)



#### `TR_temperature:` (`TR_arg_temperature`, `sıcaklık`)

örnekle'de logitlerin bölündüğü sıcaklıktır: küçük değer en olasıya yaklaştırır, 0 her zaman en olasıyı seçer. Negatif olmamalıdır; varsayılan 1.0'dır.

```tr
TR_print(TR_sample(TR_Tensor([1.0, 3.0, 0.5]), TR_temperature: 0).TR_tolist)
```

Görünümü:

```text
en: print(sample(Tensor([1.0, 3.0, 0.5]), temperature: 0).tolist)
tr: yazdır(örnekle(Tensör([1.0, 3.0, 0.5]), sıcaklık: 0).değer)
```

Kullananlar: `TR_sample(...)` · İlişkili: `TR_top_k:`

#### `TR_threshold:` (`TR_arg_threshold`, `eşik`)



#### `TR_tied:` (`TR_arg_tied`, `paylaşımlı`)



#### `TR_token_id:` (`TR_arg_token_id`, `kimlik`)



#### `TR_token_ids:` (`TR_arg_token_ids`, `kimlikler`)



#### `TR_tol:` (`TR_arg_tol`, `tolerans`)

türev_denetle'de izin verilen göreli farktır: |analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|). Pozitif olmalıdır; varsayılan 0.01'dir.

```tr
TR_print(TR_grad_check((x) => TR_sigmoid(x), [TR_Tensor([0.3], TR_requires_grad: TR_true)], TR_tol: 0.001))
```

Görünümü:

```text
en: print(grad_check((x) => sigmoid(x), [Tensor([0.3], requires_grad: true)], tol: 0.001))
tr: yazdır(türev_denetle((x) => sigmoid(x), [Tensör([0.3], türev_izle: doğru)], tolerans: 0.001))
```

Kullananlar: `TR_grad_check(...)` · İlişkili: `TR_fd_step:`

#### `TR_top_k:` (`TR_arg_top_k`, `en_iyi_k`)

örnekle'de seçimi en olası k sınıfla sınırlar; 0 (varsayılan) bütün sınıflar demektir. Negatif olmayan bir tam sayı olmalıdır.

```tr
TR_print(TR_sample(TR_Tensor([1.0, 3.0, 0.5]), TR_top_k: 1).TR_tolist)
```

Görünümü:

```text
en: print(sample(Tensor([1.0, 3.0, 0.5]), top_k: 1).tolist)
tr: yazdır(örnekle(Tensör([1.0, 3.0, 0.5]), en_iyi_k: 1).değer)
```

Kullananlar: `TR_sample(...)` · İlişkili: `TR_temperature:`

#### `TR_u:` (`TR_arg_u`, `u`)



#### `TR_updates:` (`TR_arg_updates`, `güncellemeler`)



#### `TR_use_bias:` (`TR_arg_use_bias`, `sapmalı`)



#### `TR_v:` (`TR_arg_v`, `v`)



#### `TR_value:` (`TR_arg_value`, `değer`)



#### `TR_vocab_size:` (`TR_arg_vocab_size`, `dağarcık_boyutu`)



#### `TR_warmup:` (`TR_arg_warmup`, `ısınma`)



#### `TR_weight:` (`TR_arg_weight`, `ağırlık`)

Normalleştirme sonucunu son eksende eleman bazlı ölçekleyen [D] şeklindeki tensördür; yok verilirse ölçekleme yapılmaz. Konumlu da verilebilir.

```tr
TR_print(TR_rms_norm(TR_Tensor([[3.0, 4.0]]), TR_weight: TR_Tensor([2.0, 1.0])).TR_tolist)
```

Görünümü:

```text
en: print(rms_norm(Tensor([[3.0, 4.0]]), weight: Tensor([2.0, 1.0])).tolist)
tr: yazdır(rms_normu(Tensör([[3.0, 4.0]]), ağırlık: Tensör([2.0, 1.0])).değer)
```

Kullananlar: `TR_layer_norm(...)`, `TR_rms_norm(...)` · İlişkili: `TR_bias:`

#### `TR_weight_decay:` (`TR_arg_weight_decay`, `ağırlık_çürümesi`)

Her adımda parametreleri sıfıra doğru küçülten düzenlileştirme katsayısıdır. Varsayılan AdamW'de 0.01, SGD'de 0'dır.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_print(TR_AdamW([w], TR_weight_decay: 0.1))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); print(AdamW([w], weight_decay: 0.1))
tr: değişken w = Tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], ağırlık_çürümesi: 0.1))
```

Kullananlar: `TR_AdamW(...)`, `TR_SGD(...)` · İlişkili: `TR_lr:`

#### `TR_weight_dtype:` (`TR_arg_weight_dtype`, `ağırlık_türü`)



#### `TR_weights:` (`TR_arg_weights`, `ağırlıklar`)



#### `TR_window:` (`TR_arg_window`, `pencere`)

Her sorguyu son w konuma (kendisi dahil) sınırlayan yerel dikkat penceresidir; 0 (varsayılan) pencere yok demektir. nedensel: yanlış ile pencere iki yönlüdür.

```tr
TR_var q = TR_randn([6, 4], TR_seed: 2); TR_print(TR_attention(q, q, q, TR_window: 2).TR_shape)
```

Görünümü:

```text
en: var q = randn([6, 4], seed: 2); print(attention(q, q, q, window: 2).shape)
tr: değişken q = rastgele_normal([6, 4], tohum: 2); yazdır(dikkat(q, q, q, pencere: 2).şekil)
```

Kullananlar: `TR_attention(...)` · İlişkili: `TR_causal:`

#### `TR_windows:` (`TR_arg_windows`, `pencereler`)



#### `TR_x:` (`TR_arg_x`, `x`)



## Metot ve özellikler (x.ad)

### Metot ve özellikler

#### `.TR_absolute` (`TR_mem_absolute`, `mutlak`)

    Yol.mutlak → Yol

Yolu dosya_oku ile aynı kuralla (önce çalışma klasörü, sonra modülün ve programın klasörü) çözüp mutlak bir Yol olarak verir; günlük ve hata ayıklama için.

```tr
TR_print(TR_Path("a.txt").TR_absolute.TR_name)
```

Görünümü:

```text
en: print(Path("a.txt").absolute.name)
tr: yazdır(Yol("a.txt").mutlak.dosya_adı)
```

#### `.TR_adamw` (`TR_mem_adamw`, `adamw`)



#### `.TR_adamw_lr` (`TR_mem_adamw_lr`, `adamw_öğrenme_oranı`)



#### `.TR_adamw_no_decay` (`TR_mem_adamw_no_decay`, `adamw_çürümesiz`)



#### `.TR_advance(...)` (`TR_mem_advance`, `ilerle`)



#### `.TR_append(...)` (`TR_mem_append`, `ekle`)

    Liste.ekle(Herhangi...) → Liste
    Günlük.ekle(Sözlük | Metin) → Mantık

Listenin sonuna verilen elemanları ekler; listeyi yerinde değiştirir ve listenin kendisini verir. Günlük'te bir kayıt (Sözlük ya da Metin) yazar ve doğru verir.

```tr
TR_var l = [1]; l.TR_append(2, 3); TR_print(l)
```

Görünümü:

```text
en: var l = [1]; l.append(2, 3); print(l)
tr: değişken l = [1]; l.ekle(2, 3); yazdır(l)
```

İlişkili: `.TR_first`, `.TR_last`

#### `.TR_append_buffer(...)` (`TR_mem_append_buffer`, `tampon_ekle`)



#### `.TR_attention` (`TR_mem_attention`, `dikkat`)



#### `.TR_bar` (`TR_mem_bar`, `çubuk`)



#### `.TR_beta1` (`TR_mem_beta1`, `beta1`)



#### `.TR_beta2` (`TR_mem_beta2`, `beta2`)



#### `.TR_bias` (`TR_mem_bias`, `sapma`)



#### `.TR_blocks` (`TR_mem_blocks`, `bloklar`)



#### `.TR_build_cache(...)` (`TR_mem_build_cache`, `önbellek_kur`)



#### `.TR_capacity` (`TR_mem_capacity`, `kapasite`)



#### `.TR_checkpoint` (`TR_mem_checkpoint`, `nokta`)



#### `.TR_close(...)` (`TR_mem_close`, `kapat`)

    SatırOkuyucu.kapat() → Mantık

Okuyucunun dosyasını bırakır (sonuna kadar okunmamış dosya böylece silinebilir ve taşınabilir); kapatılmış okuyucudan satır istemek hatadır. Dosyayı bu çağrı kapattıysa doğru, zaten kapalıysa (sonuna kadar okunduysa ya da önceden kapatıldıysa) yanlış verir; iki kez kapatmak hata değildir.

```tr
TR_write_file("kapat.txt", "a\nb"); TR_var o = TR_read_lines("kapat.txt"); TR_print(o.TR_next(), o.TR_close(), o.TR_close(), TR_remove("kapat.txt"))
```

Görünümü:

```text
en: write_file("kapat.txt", "a\nb"); var o = read_lines("kapat.txt"); print(o.next(), o.close(), o.close(), remove("kapat.txt"))
tr: dosya_yaz("kapat.txt", "a\nb"); değişken o = dosya_satırları("kapat.txt"); yazdır(o.sonraki(), o.kapat(), o.kapat(), sil("kapat.txt"))
```

İlişkili: `.TR_next(...)`, `TR_read_lines(...)`

#### `.TR_config` (`TR_mem_config`, `ayar`)



#### `.TR_contains(...)` (`TR_mem_contains`, `içerir`)

    Sözlük.içerir(Herhangi) → Mantık
    Liste.içerir(Herhangi) → Mantık
    Metin.içerir(Metin) → Mantık

Metinde alt metni, listede değeri (== ile), sözlükte anahtarı arar ve doğru ya da yanlış verir. Metinde argüman Metin olmalıdır.

```tr
TR_print("merhaba".TR_contains("er"), [1, 2].TR_contains(2), {"a": 1}.TR_contains("a"))
```

Görünümü:

```text
en: print("merhaba".contains("er"), [1, 2].contains(2), {"a": 1}.contains("a"))
tr: yazdır("merhaba".içerir("er"), [1, 2].içerir(2), {"a": 1}.içerir("a"))
```

İlişkili: `.TR_starts_with(...)`, `.TR_ends_with(...)`

#### `.TR_context_proj` (`TR_mem_context_proj`, `bağlam_katmanı`)



#### `.TR_cos_table` (`TR_mem_cos_table`, `cos_tablosu`)



#### `.TR_decode(...)` (`TR_mem_decode`, `çöz`)

    Parçalayıcı.çöz(Liste | Tensör, özel_parçalar: Mantık = yanlış) → Metin | Liste

Parça kimliklerini (liste, listelerin listesi ya da tensör) metne geri çevirir. Varsayılan olarak özel parçaları atlar; özel_parçalar: doğru onları da yazar.

```tr
TR_var p = TR_load_tokenizer("tests/data/tokenizer.json"); TR_print(p.TR_decode(p.TR_encode("Merhaba dünya!")))
```

Görünümü:

```text
en: var p = load_tokenizer("tests/data/tokenizer.json"); print(p.decode(p.encode("Merhaba dünya!")))
tr: değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(p.çöz(p.kodla("Merhaba dünya!")))
```

İlişkili: `.TR_encode(...)`

#### `.TR_device` (`TR_mem_device`, `cihaz`)

    Tensör.cihaz → Metin

Tensörün bulunduğu cihazın adını verir: "cpu", "gpu" (ilk GPU) ya da "gpu:1" gibi.

```tr
TR_print(TR_Tensor([1, 2]).TR_device)
```

Görünümü:

```text
en: print(Tensor([1, 2]).device)
tr: yazdır(Tensör([1, 2]).cihaz)
```

İlişkili: `.TR_to_device(...)`, `TR_default_device(...)`

#### `.TR_device_step(...)` (`TR_mem_device_step`, `cihaz_adımı`)



#### `.TR_down_proj` (`TR_mem_down_proj`, `aşağı`)



#### `.TR_dry_run` (`TR_mem_dry_run`, `prova`)



#### `.TR_dtype` (`TR_mem_dtype`, `tür`)

    Tensör.tür → Metin

Tensörün sayı türünün adını verir: "float32", "bf16" ya da "fp16".

```tr
TR_print(TR_Tensor([1.0, 2.0]).TR_dtype)
```

Görünümü:

```text
en: print(Tensor([1.0, 2.0]).dtype)
tr: yazdır(Tensör([1.0, 2.0]).tür)
```

İlişkili: `.TR_to_dtype(...)`, `TR_dtype:`

#### `.TR_embed(...)` (`TR_mem_embed`, `göm`)



#### `.TR_embed_dim` (`TR_mem_embed_dim`, `boyut`)



#### `.TR_embedding` (`TR_mem_embedding`, `gömme`)



#### `.TR_encode(...)` (`TR_mem_encode`, `kodla`)

    Parçalayıcı.kodla(Metin | Liste, özel_parçalar: Mantık = doğru, ikinci: Metin | Liste) → Liste

Metni parça kimliklerinin listesine çevirir; metin listesi verilirse her metni ayrı kodlar. özel_parçalar: yanlış verilmezse parçalayıcının özel parçaları eklenir; ikinci: ile metin çifti kodlanır.

```tr
TR_print(TR_load_tokenizer("tests/data/tokenizer.json").TR_encode("Merhaba dünya!"))
```

Görünümü:

```text
en: print(load_tokenizer("tests/data/tokenizer.json").encode("Merhaba dünya!"))
tr: yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("Merhaba dünya!"))
```

İlişkili: `.TR_decode(...)`

#### `.TR_ends_with(...)` (`TR_mem_ends_with`, `biter`)

    Metin.biter(Metin) → Mantık

Metin verilen sonekle bitiyorsa doğru verir. Argüman Metin olmalıdır.

```tr
TR_print("rapor.tr".TR_ends_with(".tr"))
```

Görünümü:

```text
en: print("rapor.tr".ends_with(".tr"))
tr: yazdır("rapor.tr".biter(".tr"))
```

İlişkili: `.TR_starts_with(...)`, `.TR_contains(...)`

#### `.TR_eps` (`TR_mem_eps`, `epsilon`)



#### `.TR_evaluated(...)` (`TR_mem_evaluated`, `değerlendirildi`)



#### `.TR_exceeds(...)` (`TR_mem_exceeds`, `aşıyor`)



#### `.TR_exists` (`TR_mem_exists`, `var_mı`)

    Yol.var_mı → Mantık

Yolda bir dosya ya da klasör var mı söyler; bağlantıyı izler, kırık bağlantıda yanlış verir.

```tr
TR_write_file("var.txt", "x"); TR_print(TR_Path("var.txt").TR_exists, TR_Path("olmayan.txt").TR_exists)
```

Görünümü:

```text
en: write_file("var.txt", "x"); print(Path("var.txt").exists, Path("olmayan.txt").exists)
tr: dosya_yaz("var.txt", "x"); yazdır(Yol("var.txt").var_mı, Yol("olmayan.txt").var_mı)
```

İlişkili: `.TR_is_file`, `.TR_is_dir`

#### `.TR_extension` (`TR_mem_extension`, `uzantı`)

    Yol.uzantı → Metin

Dosya adının noktasıyla birlikte son uzantısını verir ("derlem.tar.gz" → ".gz"); uzantı yoksa ya da ad nokta ile başlıyorsa (".gitignore") boş metindir.

```tr
TR_print(TR_Path("veri/derlem.tar.gz").TR_extension)
```

Görünümü:

```text
en: print(Path("veri/derlem.tar.gz").extension)
tr: yazdır(Yol("veri/derlem.tar.gz").uzantı)
```

İlişkili: `.TR_stem`

#### `.TR_feed_forward` (`TR_mem_feed_forward`, `ileri_besleme`)



#### `.TR_filter(...)` (`TR_mem_filter`, `filtrele`)

    Liste.filtrele(Fonksiyon) → Liste

Fonksiyonun doğru döndüğü elemanlardan oluşan yeni bir liste verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
TR_print([1, 2, 3, 4].TR_filter((x) => x > 2))
```

Görünümü:

```text
en: print([1, 2, 3, 4].filter((x) => x > 2))
tr: yazdır([1, 2, 3, 4].filtrele((x) => x > 2))
```

İlişkili: `.TR_map(...)`, `.TR_reduce(...)`

#### `.TR_final_norm` (`TR_mem_final_norm`, `son_norm`)



#### `.TR_finish(...)` (`TR_mem_finish`, `bitir`)



#### `.TR_first` (`TR_mem_first`, `ilk`)

    Liste.ilk → Herhangi

Listenin ilk elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
TR_print([5, 6, 7].TR_first)
```

Görünümü:

```text
en: print([5, 6, 7].first)
tr: yazdır([5, 6, 7].ilk)
```

İlişkili: `.TR_last`

#### `.TR_folder` (`TR_mem_folder`, `klasör`)



#### `.TR_forward(...)` (`TR_mem_forward`, `ileri`)



#### `.TR_gate_proj` (`TR_mem_gate_proj`, `kapı`)



#### `.TR_gate_up_proj` (`TR_mem_gate_up_proj`, `kapı_yukarı`)



#### `.TR_generate(...)` (`TR_mem_generate`, `üret`)



#### `.TR_get(...)` (`TR_mem_get`, `varsa_al`)

    Sözlük.varsa_al(Herhangi) → Herhangi
    Liste.varsa_al(Tam) → Herhangi
    Metin.varsa_al(Tam) → Metin | YokTipi

i. öğeyi (negatif indeks sondan sayar) ya da sözlükte ve JSON nesnesinde k anahtarının değerini verir; yoksa yok döner. Köşeli parantezle okuma (l[i], d["k"]) olmayan öğede hata verir; öğenin olmayabileceği bilinçli durumda varsa_al ile ?? birlikte kullanılır.

```tr
TR_var l = [10, 20, 30]; TR_print(l.TR_get(1), l.TR_get(5) ?? 0, "ab".TR_get(-1), {"a": 1}.TR_get("b") ?? 0)
```

Görünümü:

```text
en: var l = [10, 20, 30]; print(l.get(1), l.get(5) ?? 0, "ab".get(-1), {"a": 1}.get("b") ?? 0)
tr: değişken l = [10, 20, 30]; yazdır(l.varsa_al(1), l.varsa_al(5) ?? 0, "ab".varsa_al(-1), {"a": 1}.varsa_al("b") ?? 0)
```

İlişkili: `.TR_first`, `.TR_last`, `.TR_contains(...)`

#### `.TR_grad` (`TR_mem_grad`, `gradyan`)

    Tensör.gradyan → Tensör | YokTipi

geri_yayılım sonrası biriken gradyanı tensör olarak verir; henüz yoksa yok döner. Yalnızca yaprak tensörlerde okunabilir, bir işlemin sonucunda hata verir.

```tr
TR_var x = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true); TR_backward(TR_sum(x * 3)); TR_print(x.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var x = Tensor([1.0, 2.0], requires_grad: true); backward(sum(x * 3)); print(x.grad.tolist)
tr: değişken x = Tensör([1.0, 2.0], türev_izle: doğru); geri_yayılım(toplam(x * 3)); yazdır(x.gradyan.değer)
```

İlişkili: `TR_backward(...)`, `TR_requires_grad:`

#### `.TR_groups` (`TR_mem_groups`, `gruplar`)



#### `.TR_head_dim` (`TR_mem_head_dim`, `bakış_boyutu`)



#### `.TR_hidden_states(...)` (`TR_mem_hidden_states`, `gizli_temsil`)



#### `.TR_history_length(...)` (`TR_mem_history_length`, `geçmiş_uzunluğu`)



#### `.TR_history_limit` (`TR_mem_history_limit`, `geçmiş_sınırı`)



#### `.TR_input_norm` (`TR_mem_input_norm`, `norm1`)



#### `.TR_is_dir` (`TR_mem_is_dir`, `klasör_mü`)

    Yol.klasör_mü → Mantık

Yol bir klasör mü söyler (bağlantıyı izler); yol yoksa yanlış verir.

```tr
TR_make_dir("bir_klasör"); TR_print(TR_Path("bir_klasör").TR_is_dir)
```

Görünümü:

```text
en: make_dir("bir_klasör"); print(Path("bir_klasör").is_dir)
tr: klasör_oluştur("bir_klasör"); yazdır(Yol("bir_klasör").klasör_mü)
```

İlişkili: `.TR_exists`, `.TR_is_file`

#### `.TR_is_file` (`TR_mem_is_file`, `dosya_mı`)

    Yol.dosya_mı → Mantık

Yol sıradan bir dosya mı söyler (bağlantıyı izler); yol yoksa yanlış verir.

```tr
TR_make_dir("bir_klasör"); TR_print(TR_Path("bir_klasör").TR_is_file)
```

Görünümü:

```text
en: make_dir("bir_klasör"); print(Path("bir_klasör").is_file)
tr: klasör_oluştur("bir_klasör"); yazdır(Yol("bir_klasör").dosya_mı)
```

İlişkili: `.TR_exists`, `.TR_is_dir`

#### `.TR_join(...)` (`TR_mem_join`, `birleştir`)

    Liste.birleştir([Metin]) → Metin

Liste elemanlarını metne çevirip aralarına ayırıcıyı koyarak birleştirir (ayırıcı verilmezse boş metin). Sonuç bir metindir.

```tr
TR_print(["a", "b", "c"].TR_join(", "))
```

Görünümü:

```text
en: print(["a", "b", "c"].join(", "))
tr: yazdır(["a", "b", "c"].birleştir(", "))
```

İlişkili: `.TR_split(...)`

#### `.TR_k_buffers` (`TR_mem_k_buffers`, `k_tamponları`)



#### `.TR_k_norm` (`TR_mem_k_norm`, `k_norm`)



#### `.TR_k_proj` (`TR_mem_k_proj`, `anahtar_katmanı`)



#### `.TR_keys(...)` (`TR_mem_keys`, `anahtarlar`)

    Sözlük.anahtarlar() → Liste

Sözlüğün anahtarlarını ekleniş sırasıyla bir liste olarak verir.

```tr
TR_print({"ad": "Ali", "puan": 30}.TR_keys())
```

Görünümü:

```text
en: print({"ad": "Ali", "puan": 30}.keys())
tr: yazdır({"ad": "Ali", "puan": 30}.anahtarlar())
```

İlişkili: `.TR_values(...)`

#### `.TR_kind` (`TR_mem_kind`, `çeşit`)



#### `.TR_kv_proj` (`TR_mem_kv_proj`, `kv_katmanı`)



#### `.TR_last` (`TR_mem_last`, `son`)

    Liste.son → Herhangi

Listenin son elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
TR_print([5, 6, 7].TR_last)
```

Görünümü:

```text
en: print([5, 6, 7].last)
tr: yazdır([5, 6, 7].son)
```

İlişkili: `.TR_first`

#### `.TR_last_step` (`TR_mem_last_step`, `son_adım`)



#### `.TR_length` (`TR_mem_length`, `uzunluk`)

    Metin.uzunluk → Tam
    Liste.uzunluk → Tam
    Sözlük.uzunluk → Tam

Metnin karakter sayısını, listenin ya da sözlüğün eleman sayısını verir. Parantezsiz yazılan bir özelliktir.

```tr
TR_print("çay".TR_length, [1, 2, 3].TR_length, {"a": 1}.TR_length)
```

Görünümü:

```text
en: print("çay".length, [1, 2, 3].length, {"a": 1}.length)
tr: yazdır("çay".uzunluk, [1, 2, 3].uzunluk, {"a": 1}.uzunluk)
```

#### `.TR_load_state(...)` (`TR_mem_load_state`, `durumu_yükle`)

    İyileştirici.durumu_yükle(Sözlük) → Mantık
    Örnekleyici.durumu_yükle(Sözlük) → Mantık
    SıralıOkuyucu.durumu_yükle(Sözlük) → Mantık

durum ile alınmış sözlüğü aynı ayarla kurulmuş bir iyileştiriciye ya da okuyucuya yükler ve doğru verir; tensörler parametrenin cihazına taşınır. Parametre sayısı ve şekiller, okuyucunun türü, toplu, bağlam, tohum ve veri kümesi aynı olmalıdır, değilse hata verir.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var op = TR_AdamW([w]); TR_backward(TR_sum(w * 2)); op.TR_step(); TR_var yeni = TR_AdamW([w]); TR_print(yeni.TR_load_state(op.TR_state))
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var op = AdamW([w]); backward(sum(w * 2)); op.step(); var yeni = AdamW([w]); print(yeni.load_state(op.state))
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken op = AdamW([w]); geri_yayılım(toplam(w * 2)); op.adım(); değişken yeni = AdamW([w]); yazdır(yeni.durumu_yükle(op.durum))
```

İlişkili: `.TR_state`

#### `.TR_logger` (`TR_mem_logger`, `günlük`)



#### `.TR_logit_layer` (`TR_mem_logit_layer`, `logit_katmanı`)



#### `.TR_logits(...)` (`TR_mem_logits`, `logit`)



#### `.TR_loss(...)` (`TR_mem_loss`, `kayıp`)



#### `.TR_losses` (`TR_mem_losses`, `kayıplar`)



#### `.TR_lower(...)` (`TR_mem_lower`, `küçük_harf`)

    Metin.küçük_harf() → Metin

Metni Unicode'un varsayılan kuralıyla küçük harfe çevirir; sonuç programın dilinden bağımsızdır (I → i). Yeni bir metin verir.

```tr
TR_print("MERHABA".TR_lower())
```

Görünümü:

```text
en: print("MERHABA".lower())
tr: yazdır("MERHABA".küçük_harf())
```

İlişkili: `.TR_upper(...)`

#### `.TR_lr` (`TR_mem_lr`, `öğrenme_oranı`)

    İyileştirici.öğrenme_oranı → Ondalık

İyileştiricinin (AdamW, SGD) şimdiki öğrenme oranıdır (Ondalık); okunur ve atanabilir: op.öğrenme_oranı = 0.0005 ataması sonraki adımdan başlayarak yeni oranı kullanır (ısınma, düşürme gibi öğrenme oranı planları). Atanan değer sonlu ve negatif olmayan bir Tam ya da Ondalık olmalıdır; oran 32 bit saklanır. Ayar olduğu için duruma (durum) girmez.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var op = TR_SGD([w], TR_lr: 0.5); op.TR_lr = 0.25; TR_backward(TR_sum(w * w)); op.TR_step(); TR_print(op.TR_lr, w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var op = SGD([w], lr: 0.5); op.lr = 0.25; backward(sum(w * w)); op.step(); print(op.lr, w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.5); op.öğrenme_oranı = 0.25; geri_yayılım(toplam(w * w)); op.adım(); yazdır(op.öğrenme_oranı, w.değer)
```

İlişkili: `TR_lr:`, `.TR_step(...)`

#### `.TR_lr_multiplier` (`TR_mem_lr_multiplier`, `öğrenme_oranı_çarpanı`)



#### `.TR_map(...)` (`TR_mem_map`, `haritala`)

    Liste.haritala(Fonksiyon) → Liste

Her elemana fonksiyonu uygular ve sonuçların yeni listesini verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
TR_print([1, 2, 3].TR_map((x) => x * 10))
```

Görünümü:

```text
en: print([1, 2, 3].map((x) => x * 10))
tr: yazdır([1, 2, 3].haritala((x) => x * 10))
```

İlişkili: `.TR_filter(...)`, `.TR_reduce(...)`

#### `.TR_mask(...)` (`TR_mem_mask`, `maske`)



#### `.TR_masks` (`TR_mem_masks`, `maskeler`)



#### `.TR_momentum` (`TR_mem_momentum`, `momentum`)



#### `.TR_muon` (`TR_mem_muon`, `muon`)



#### `.TR_muon_lr` (`TR_mem_muon_lr`, `muon_öğrenme_oranı`)



#### `.TR_name` (`TR_mem_name`, `dosya_adı`)

    Yol.dosya_adı → Metin

Yolun son bileşenini verir ("veri/derlem.tar.gz" → "derlem.tar.gz"); . ve kök için boş metindir.

```tr
TR_print(TR_Path("veri/derlem.tar.gz").TR_name)
```

Görünümü:

```text
en: print(Path("veri/derlem.tar.gz").name)
tr: yazdır(Yol("veri/derlem.tar.gz").dosya_adı)
```

İlişkili: `.TR_extension`, `.TR_stem`

#### `.TR_nesterov` (`TR_mem_nesterov`, `nesterov`)



#### `.TR_next(...)` (`TR_mem_next`, `sonraki`)

    SatırOkuyucu.sonraki() → Metin | YokTipi
    Örnekleyici.sonraki() → Liste
    SıralıOkuyucu.sonraki() → Liste | YokTipi

Örnekleyici ve sıralı okuyucuda bir sonraki toplu işi [girdi, hedef] tensörleri olarak (ikisi de [toplu, bağlam] şeklinde, hedef bir parça kaydırılmış), SatırOkuyucu'da sıradaki satırı Metin olarak verir. Sıralı okuyucu veri, SatırOkuyucu dosya bitince yok verir.

```tr
TR_var girdi, hedef = TR_Dataset("tests/data/numbers").TR_sequential(TR_batch_size: 2, TR_context: 4).TR_next(); TR_print(girdi.TR_tolist, hedef.TR_tolist)
```

Görünümü:

```text
en: var girdi, hedef = Dataset("tests/data/numbers").sequential(batch_size: 2, context: 4).next(); print(girdi.tolist, hedef.tolist)
tr: değişken girdi, hedef = VeriKümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4).sonraki(); yazdır(girdi.değer, hedef.değer)
```

İlişkili: `.TR_sampler(...)`, `.TR_sequential(...)`, `TR_read_lines(...)`

#### `.TR_norm` (`TR_mem_norm`, `norm`)



#### `.TR_norms` (`TR_mem_norms`, `normlar`)



#### `.TR_ns_dtype` (`TR_mem_ns_dtype`, `ns_türü`)



#### `.TR_ns_iterations` (`TR_mem_ns_iterations`, `ns_yineleme_sayısı`)



#### `.TR_num_heads` (`TR_mem_num_heads`, `bakış_sayısı`)



#### `.TR_num_kv_heads` (`TR_mem_num_kv_heads`, `kv_bakış_sayısı`)



#### `.TR_num_layers` (`TR_mem_num_layers`, `katman_sayısı`)



#### `.TR_o_proj` (`TR_mem_o_proj`, `çıkış_katmanı`)



#### `.TR_observe(...)` (`TR_mem_observe`, `gözle`)



#### `.TR_other` (`TR_mem_other`, `diğer`)



#### `.TR_parameters` (`TR_mem_parameters`, `parametreler`)



#### `.TR_parent` (`TR_mem_parent`, `üst_klasör`)

    Yol.üst_klasör → Yol

Yolun bulunduğu klasörü Yol olarak verir; tek bileşenli göreli yolda . olur.

```tr
TR_print(TR_Path("veri/eğitim/a.npy").TR_parent)
```

Görünümü:

```text
en: print(Path("veri/eğitim/a.npy").parent)
tr: yazdır(Yol("veri/eğitim/a.npy").üst_klasör)
```

İlişkili: `.TR_name`

#### `.TR_permute(...)` (`TR_mem_permute`, `eksenleri_sırala`)

    Tensör.eksenleri_sırala(Liste | Tam, [Tam...]) → Tensör

Tensörün eksenlerini verilen sıraya dizer. Her eksen tam olarak bir kez yer almalıdır.

```tr
TR_print(TR_zeros([2, 3, 4]).TR_permute([2, 0, 1]).TR_shape)
```

Görünümü:

```text
en: print(zeros([2, 3, 4]).permute([2, 0, 1]).shape)
tr: yazdır(sıfırlar([2, 3, 4]).eksenleri_sırala([2, 0, 1]).şekil)
```

İlişkili: `.TR_transpose(...)`

#### `.TR_ple` (`TR_mem_ple`, `ple`)



#### `.TR_ple_context` (`TR_mem_ple_context`, `ple_bağlamı`)



#### `.TR_ple_dim` (`TR_mem_ple_dim`, `ple_boyutu`)



#### `.TR_position` (`TR_mem_position`, `konum`)



#### `.TR_positions` (`TR_mem_positions`, `konumlar`)



#### `.TR_post_attention_norm` (`TR_mem_post_attention_norm`, `norm2`)



#### `.TR_proj` (`TR_mem_proj`, `izdüşüm_katmanı`)



#### `.TR_q_norm` (`TR_mem_q_norm`, `q_norm`)



#### `.TR_q_proj` (`TR_mem_q_proj`, `sorgu_katmanı`)



#### `.TR_reduce(...)` (`TR_mem_reduce`, `indirge`)

    Liste.indirge([Herhangi], Fonksiyon) → Herhangi

Listeyi soldan sağa (birikim, eleman) alan bir fonksiyonla tek değere indirger; başlangıç verilmezse ilk eleman başlangıçtır, boş listede başlangıç (yoksa yok) döner. Fonksiyon son argümandır ve iki parametre alır; sondaki blokla yazılınca birikim önceki sonuç, o eleman olur.

```tr
TR_print([1, 2, 3, 4].TR_reduce(0, (a, x) => a + x), [1, 2, 3, 4].TR_reduce(1, (a, x) => a * x))
```

Görünümü:

```text
en: print([1, 2, 3, 4].reduce(0, (a, x) => a + x), [1, 2, 3, 4].reduce(1, (a, x) => a * x))
tr: yazdır([1, 2, 3, 4].indirge(0, (a, x) => a + x), [1, 2, 3, 4].indirge(1, (a, x) => a * x))
```

İlişkili: `.TR_filter(...)`, `.TR_map(...)`

#### `.TR_replace(...)` (`TR_mem_replace`, `değiştir`)

    Metin.değiştir(Metin, Metin) → Metin

Metindeki bütün eski alt metinleri yenisiyle değiştirir. İki argüman da Metin olmalıdır; yeni bir metin döner.

```tr
TR_print("elma armut elma".TR_replace("elma", "erik"))
```

Görünümü:

```text
en: print("elma armut elma".replace("elma", "erik"))
tr: yazdır("elma armut elma".değiştir("elma", "erik"))
```

#### `.TR_reshape(...)` (`TR_mem_reshape`, `şekillendir`)

    Tensör.şekillendir(Liste) → Tensör

Tensörü aynı eleman sayısıyla yeni bir şekle sokar; bir eksen -1 verilirse uzunluğu hesaplanır. Sonuç veriyi paylaşan bir görünümdür.

```tr
TR_print(TR_Tensor([1, 2, 3, 4, 5, 6]).TR_reshape([2, 3]).TR_shape)
```

Görünümü:

```text
en: print(Tensor([1, 2, 3, 4, 5, 6]).reshape([2, 3]).shape)
tr: yazdır(Tensör([1, 2, 3, 4, 5, 6]).şekillendir([2, 3]).şekil)
```

İlişkili: `.TR_shape`

#### `.TR_resume_step` (`TR_mem_resume_step`, `kaldığı_adım`)

    KontrolNoktası.kaldığı_adım → Tam

Kurulurken yüklenen kontrol noktasının adımıdır (tamamlanan adım sayısı); yeni bir koşuda 0. Eğitim döngüsü buradan başlar: döngü adım içinde kn.kaldığı_adım..toplam.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var kn = TR_Checkpoint("koşu_y520", {"w": w}); TR_print(kn.TR_resume_step)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var kn = Checkpoint("koşu_y520", {"w": w}); print(kn.resume_step)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken kn = KontrolNoktası("koşu_y520", {"w": w}); yazdır(kn.kaldığı_adım)
```

İlişkili: `.TR_save(...)`, `.TR_update(...)`

#### `.TR_reverse(...)` (`TR_mem_reverse`, `ters_çevir`)

    Liste.ters_çevir() → Liste
    Aralık.ters_çevir() → Liste
    Metin.ters_çevir() → Metin

Metnin karakterlerini, listenin elemanlarını ya da aralığın sayılarını ters sırayla verir; aralıkta sonuç bir listedir (geriye sayan döngü için). Asıl değeri değiştirmez.

```tr
TR_print("kitap".TR_reverse(), [1, 2, 3].TR_reverse(), (0..3).TR_reverse())
```

Görünümü:

```text
en: print("kitap".reverse(), [1, 2, 3].reverse(), (0..3).reverse())
tr: yazdır("kitap".ters_çevir(), [1, 2, 3].ters_çevir(), (0..3).ters_çevir())
```

İlişkili: `.TR_sort(...)`

#### `.TR_rollback(...)` (`TR_mem_rollback`, `geri_dön`)

    KontrolNoktası.geri_dön() → Tam

En yeni geçerli kontrol noktasını aynı programda yerinde yükler (parametreler, iyileştiriciler, okuyucular, düz değerler; günlüklere devam kaydı yazılır) ve noktanın adımını Tam olarak verir; eğitim döngüsü o adımdan sürer. Kayıp NaN olunca ya da sıçrayınca son sağlam noktaya dönmek içindir. Nokta yoksa hatadır.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var kn = TR_Checkpoint("koşu_y522", {"w": w}); kn.TR_update(1); kn.TR_save(); TR_print(kn.TR_rollback())
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var kn = Checkpoint("koşu_y522", {"w": w}); kn.update(1); kn.save(); print(kn.rollback())
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken kn = KontrolNoktası("koşu_y522", {"w": w}); kn.güncelle(1); kn.kaydet(); yazdır(kn.geri_dön())
```

İlişkili: `.TR_save(...)`, `.TR_resume_step`, `TR_is_finite(...)`

#### `.TR_row_rms` (`TR_mem_row_rms`, `satır_rms`)



#### `.TR_run_blocks(...)` (`TR_mem_run_blocks`, `blokları_işle`)



#### `.TR_sampler(...)` (`TR_mem_sampler`, `örnekleyici`)

    VeriKümesi.örnekleyici([Tam], [Tam], [Tam], toplu: Tam, bağlam: Tam, tohum: Tam = 0) → Örnekleyici

Veri kümesinden rastgele pencereler alan bir örnekleyici kurar; toplu ve bağlam zorunludur, tohum isteğe bağlıdır. Örnekleyici sonsuzdur, dönemler bitince baştan sürer.

```tr
TR_print(TR_Dataset("tests/data/numbers").TR_sampler(TR_batch_size: 2, TR_context: 4, TR_seed: 1))
```

Görünümü:

```text
en: print(Dataset("tests/data/numbers").sampler(batch_size: 2, context: 4, seed: 1))
tr: yazdır(VeriKümesi("tests/data/numbers").örnekleyici(toplu: 2, bağlam: 4, tohum: 1))
```

İlişkili: `.TR_sequential(...)`, `.TR_next(...)`

#### `.TR_save(...)` (`TR_mem_save`, `kaydet`)

    KontrolNoktası.kaydet() → Yol

Kontrol noktasını hemen, son güncelle(adım) adımıyla kaydeder ve noktanın klasörünü Yol olarak verir (ör. eğitim sonunda). Yazma hatası açık hatadır; aralıkla kayıttaki gibi eğitim sürmez.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var kn = TR_Checkpoint("koşu_y521", {"w": w}); kn.TR_update(1); TR_print(kn.TR_save())
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var kn = Checkpoint("koşu_y521", {"w": w}); kn.update(1); print(kn.save())
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken kn = KontrolNoktası("koşu_y521", {"w": w}); kn.güncelle(1); yazdır(kn.kaydet())
```

İlişkili: `.TR_resume_step`, `.TR_update(...)`

#### `.TR_scales` (`TR_mem_scales`, `ölçekler`)



#### `.TR_sequential(...)` (`TR_mem_sequential`, `sıralı`)

    VeriKümesi.sıralı([Tam], [Tam], toplu: Tam, bağlam: Tam) → SıralıOkuyucu

Veri kümesini baştan sona sırayla okuyan bir okuyucu kurar; toplu ve bağlam zorunludur. Veri bitince sonraki() yok verir.

```tr
TR_print(TR_Dataset("tests/data/numbers").TR_sequential(TR_batch_size: 2, TR_context: 4).TR_next()[0].TR_shape)
```

Görünümü:

```text
en: print(Dataset("tests/data/numbers").sequential(batch_size: 2, context: 4).next()[0].shape)
tr: yazdır(VeriKümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4).sonraki()[0].şekil)
```

İlişkili: `.TR_sampler(...)`, `.TR_next(...)`

#### `.TR_set_multiplier(...)` (`TR_mem_set_multiplier`, `çarpan_ayarla`)



#### `.TR_shape` (`TR_mem_shape`, `şekil`)

    Tensör.şekil → Liste

Tensörün her eksendeki uzunluğunu Tam listesi olarak verir; 0 boyutlu tensörde boş listedir.

```tr
TR_print(TR_Tensor([[1, 2], [3, 4]]).TR_shape)
```

Görünümü:

```text
en: print(Tensor([[1, 2], [3, 4]]).shape)
tr: yazdır(Tensör([[1, 2], [3, 4]]).şekil)
```

İlişkili: `.TR_reshape(...)`

#### `.TR_sin_table` (`TR_mem_sin_table`, `sin_tablosu`)



#### `.TR_single_step(...)` (`TR_mem_single_step`, `tek_adım`)



#### `.TR_size` (`TR_mem_size`, `bayt`)

    Yol.bayt → Tam

Dosyanın boyutunu bayt olarak Tam verir (.gz dosyada sıkıştırılmış boyut); klasörde ve olmayan dosyada hata verir.

```tr
TR_write_file("boyut.txt", "merhaba"); TR_print(TR_Path("boyut.txt").TR_size)
```

Görünümü:

```text
en: write_file("boyut.txt", "merhaba"); print(Path("boyut.txt").size)
tr: dosya_yaz("boyut.txt", "merhaba"); yazdır(Yol("boyut.txt").bayt)
```

#### `.TR_skip(...)` (`TR_mem_skip`, `atla`)

    Örnekleyici.atla(Tam) → Tam
    SıralıOkuyucu.atla(Tam) → Tam

Sıradaki n toplu işi tensör kurmadan atlar ve atladığı sayıyı Tam olarak verir; sonraki .sonraki() n tane sonraki() çağrısından sonra gelecek toplu işi verir, okuyucunun durumu (sayaç) n artar. Sıralı okuyucu biterse kalan kadar atlar. Kayıp sıçramasından sonra son kontrol noktasına dönen eğitim, sıçramayı tetikleyen toplu işleri böyle atlar.

```tr
TR_var ö = TR_Dataset("tests/data/text").TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1); TR_print(ö.TR_skip(3), ö.TR_state["counter"])
```

Görünümü:

```text
en: var ö = Dataset("tests/data/text").sampler(batch_size: 2, context: 8, seed: 1); print(ö.skip(3), ö.state["counter"])
tr: değişken ö = VeriKümesi("tests/data/text").örnekleyici(toplu: 2, bağlam: 8, tohum: 1); yazdır(ö.atla(3), ö.durum["counter"])
```

İlişkili: `.TR_next(...)`, `.TR_state`, `.TR_rollback(...)`

#### `.TR_slice(...)` (`TR_mem_slice`, `dilim`)

    Tensör.dilim(Tam, Tam, Tam) → Tensör

Verilen eksende [baş, son) aralığını keser; negatif konumlar sondan sayılır. Sonuç veriyi paylaşan bir görünümdür.

```tr
TR_print(TR_Tensor([[1, 2, 3], [4, 5, 6]]).TR_slice(1, 0, 2).TR_tolist)
```

Görünümü:

```text
en: print(Tensor([[1, 2, 3], [4, 5, 6]]).slice(1, 0, 2).tolist)
tr: yazdır(Tensör([[1, 2, 3], [4, 5, 6]]).dilim(1, 0, 2).değer)
```

#### `.TR_sort(...)` (`TR_mem_sort`, `sırala`)

    Liste.sırala() → Liste

Listenin küçükten büyüğe sıralanmış yeni bir kopyasını verir. Liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır.

```tr
TR_print([3, 1, 2].TR_sort())
```

Görünümü:

```text
en: print([3, 1, 2].sort())
tr: yazdır([3, 1, 2].sırala())
```

İlişkili: `.TR_reverse(...)`

#### `.TR_sparse_lr` (`TR_mem_sparse_lr`, `seyrek_öğrenme_oranı`)



#### `.TR_spike_count` (`TR_mem_spike_count`, `sıçrama_sayısı`)



#### `.TR_split(...)` (`TR_mem_split`, `böl`)

    Metin.böl([Metin]) → Liste
    Tensör.böl(Tam, [Tam], eksen: Tam = -1) → Liste

Metni ayırıcıdan böler ve parçaların listesini verir (ayırıcı verilmezse tek boşluk). Tensörü bir eksende (varsayılan son eksen) eşit sayıda parçaya böler; eksen uzunluğu parça sayısına tam bölünmelidir.

```tr
TR_print("a,b,c".TR_split(","), TR_Tensor([1, 2, 3, 4]).TR_split(2)[0].TR_tolist)
```

Görünümü:

```text
en: print("a,b,c".split(","), Tensor([1, 2, 3, 4]).split(2)[0].tolist)
tr: yazdır("a,b,c".böl(","), Tensör([1, 2, 3, 4]).böl(2)[0].değer)
```

İlişkili: `.TR_join(...)`, `TR_concat(...)`

#### `.TR_starts_with(...)` (`TR_mem_starts_with`, `başlar`)

    Metin.başlar(Metin) → Mantık

Metin verilen önekle başlıyorsa doğru verir. Argüman Metin olmalıdır.

```tr
TR_print("merhaba".TR_starts_with("mer"))
```

Görünümü:

```text
en: print("merhaba".starts_with("mer"))
tr: yazdır("merhaba".başlar("mer"))
```

İlişkili: `.TR_ends_with(...)`, `.TR_contains(...)`

#### `.TR_state` (`TR_mem_state`, `durum`)

    İyileştirici.durum → Sözlük
    Örnekleyici.durum → Sözlük
    SıralıOkuyucu.durum → Sözlük

Kaldığı yerden sürdürmek için gereken durumdur: iyileştiricide ad → tensör sözlüğü (momentler, adım sayaçları, ana kopyalar; kopyadır, tensör_kaydet ile yazılabilir), okuyucularda düz değerli bir sözlük (tür, toplu, bağlam, tohum, küme, dönem, sıra, pencere, sayaç). Öğrenme oranı gibi ayarlar duruma girmez.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var op = TR_AdamW([w]); TR_backward(TR_sum(w * 2)); op.TR_step(); TR_print(op.TR_state.TR_keys())
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var op = AdamW([w]); backward(sum(w * 2)); op.step(); print(op.state.keys())
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken op = AdamW([w]); geri_yayılım(toplam(w * 2)); op.adım(); yazdır(op.durum.anahtarlar())
```

İlişkili: `.TR_load_state(...)`, `TR_Checkpoint`

#### `.TR_stem` (`TR_mem_stem`, `uzantısız_ad`)

    Yol.uzantısız_ad → Metin

Dosya adının son uzantısı atılmış hâlini verir ("derlem.tar.gz" → "derlem.tar"); uzantısız_ad + uzantı her zaman dosya_adı'dır.

```tr
TR_print(TR_Path("veri/derlem.tar.gz").TR_stem)
```

Görünümü:

```text
en: print(Path("veri/derlem.tar.gz").stem)
tr: yazdır(Yol("veri/derlem.tar.gz").uzantısız_ad)
```

İlişkili: `.TR_extension`

#### `.TR_step(...)` (`TR_mem_step`, `adım`)

    İyileştirici.adım() → YokTipi

İyileştiricinin (AdamW, SGD) bir güncelleme adımını yapar: parametreleri gradyanlarına göre yerinde günceller. Argüman almaz, yok döner.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_var op = TR_SGD([w], TR_lr: 0.1); TR_backward(TR_sum(w * 2)); op.TR_step(); TR_print(w.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); var op = SGD([w], lr: 0.1); backward(sum(w * 2)); op.step(); print(w.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * 2)); op.adım(); yazdır(w.değer)
```

İlişkili: `.TR_zero_grad(...)`, `TR_backward(...)`

#### `.TR_step_done(...)` (`TR_mem_step_done`, `adım_bitti`)



#### `.TR_step_position` (`TR_mem_step_position`, `adım_konumu`)



#### `.TR_stop(...)` (`TR_mem_stop`, `durdur`)

    İlerleme.durdur() → Mantık

Çubuğu bitirir ve geçen süreyle son satırı yazar; ilk çağrıda doğru, sonrakilerde yanlış verir. Toplama ulaşınca ve program bitince kendiliğinden çağrılır.

```tr
TR_var ç = TR_Progress(); ç.TR_update(); TR_print(ç.TR_stop())
```

Görünümü:

```text
en: var ç = Progress(); ç.update(); print(ç.stop())
tr: değişken ç = İlerleme(); ç.güncelle(); yazdır(ç.durdur())
```

İlişkili: `.TR_update(...)`

#### `.TR_strip(...)` (`TR_mem_strip`, `kırp`)

    Metin.kırp() → Metin

Metnin başındaki ve sonundaki boşlukları (satır sonları dahil) atar. Yeni bir metin verir.

```tr
TR_print("  merhaba  ".TR_strip())
```

Görünümü:

```text
en: print("  merhaba  ".strip())
tr: yazdır("  merhaba  ".kırp())
```

#### `.TR_table` (`TR_mem_table`, `tablo`)



#### `.TR_threshold` (`TR_mem_threshold`, `eşik`)



#### `.TR_to_device(...)` (`TR_mem_to_device`, `cihaza`)

    Tensör.cihaza(Metin) → Tensör

Tensörü verilen cihaza ("cpu", "gpu", "gpu:1") taşınmış olarak verir; kullanılabilir cihazlar cihazlar() ile görülür, olmayan cihaz hatadır. Türev izleyen bir yaprak taşınınca sonuç, hedef cihazda türev izleyen yeni bir yapraktır (gradyanı kendi .gradyan'ında birikir); bir işlemin sonucu taşınınca türev kaynağa geri akar.

```tr
TR_print(TR_Tensor([1, 2]).TR_to_device("cpu").TR_device)
```

Görünümü:

```text
en: print(Tensor([1, 2]).to_device("cpu").device)
tr: yazdır(Tensör([1, 2]).cihaza("cpu").cihaz)
```

İlişkili: `.TR_device`, `TR_default_device(...)`, `TR_devices(...)`

#### `.TR_to_dtype(...)` (`TR_mem_to_dtype`, `türe`)

    Tensör.türe(Metin) → Tensör

Tensörün "float32", "bf16" ya da "fp16" türüne çevrilmiş kopyasını verir; gradyan kaynak tensöre akar. Tür zaten aynıysa tensörün kendisi döner.

```tr
TR_print(TR_Tensor([1.0, 2.0]).TR_to_dtype("bf16").TR_dtype)
```

Görünümü:

```text
en: print(Tensor([1.0, 2.0]).to_dtype("bf16").dtype)
tr: yazdır(Tensör([1.0, 2.0]).türe("bf16").tür)
```

İlişkili: `.TR_dtype`, `TR_mixed_precision(...)`

#### `.TR_token_id(...)` (`TR_mem_token_id`, `parça_kimliği`)

    Parçalayıcı.parça_kimliği(Metin) → Tam | YokTipi

Verilen parçanın dağarcıktaki kimliğini verir; parça dağarcıkta yoksa yok döner. Argüman Metin olmalıdır.

```tr
TR_print(TR_load_tokenizer("tests/data/tokenizer.json").TR_token_id("<|endoftext|>"))
```

Görünümü:

```text
en: print(load_tokenizer("tests/data/tokenizer.json").token_id("<|endoftext|>"))
tr: yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").parça_kimliği("<|endoftext|>"))
```

İlişkili: `.TR_vocab_size`

#### `.TR_tolist` (`TR_mem_tolist`, `değer`)

    Tensör.değer → Liste | Ondalık | Tam

Tensörün değerlerini iç içe liste olarak verir; 0 boyutlu tensörde tek bir sayı döner. Sonuç kopyadır, değiştirilmesi tensörü etkilemez.

```tr
TR_print(TR_Tensor([[1, 2], [3, 4]]).TR_tolist)
```

Görünümü:

```text
en: print(Tensor([[1, 2], [3, 4]]).tolist)
tr: yazdır(Tensör([[1, 2], [3, 4]]).değer)
```

#### `.TR_transpose(...)` (`TR_mem_transpose`, `devrik`)

    Tensör.devrik() → Tensör

Tensörün son iki eksenini yer değiştirir. Argüman almaz ve en az 2 boyutlu tensör ister.

```tr
TR_print(TR_Tensor([[1, 2], [3, 4]]).TR_transpose().TR_tolist)
```

Görünümü:

```text
en: print(Tensor([[1, 2], [3, 4]]).transpose().tolist)
tr: yazdır(Tensör([[1, 2], [3, 4]]).devrik().değer)
```

İlişkili: `.TR_permute(...)`

#### `.TR_truncate(...)` (`TR_mem_truncate`, `kısalt`)



#### `.TR_unique(...)` (`TR_mem_unique`, `benzersiz`)

    Liste.benzersiz() → Liste

Tekrar eden elemanları atar, her değerin ilk geçtiği sırayı korur. Yeni bir liste verir.

```tr
TR_print([1, 2, 2, 3, 1].TR_unique())
```

Görünümü:

```text
en: print([1, 2, 2, 3, 1].unique())
tr: yazdır([1, 2, 2, 3, 1].benzersiz())
```

#### `.TR_up_proj` (`TR_mem_up_proj`, `yukarı`)



#### `.TR_update(...)` (`TR_mem_update`, `güncelle`)

    KontrolNoktası.güncelle(Tam) → Mantık
    İlerleme.güncelle([Sözlük], adım: Tam = yok) → Mantık

Her çağrı eğitimin bir adım sınırıdır. İlerleme'de çubuğu bir adım ilerletir, verilen ölçüleri (kayıp, öğrenme oranı ...) gösterir, adım: ile tamamlanan adım sayısını o değere getirir ve toplama ulaşınca çubuğu bitirir. KontrolNoktası'nda güncelle(adım) tamamlanan adım sayısını bildirir; her_dakika süresi dolduysa ya da durdurma istendiyse (Ctrl+C) burada kaydeder, kaydettiyse doğru verir.

```tr
TR_var ç = TR_Progress(10); ç.TR_update({"kayıp": 0.5}, TR_step: 5); TR_print(ç)
```

Görünümü:

```text
en: var ç = Progress(10); ç.update({"kayıp": 0.5}, step: 5); print(ç)
tr: değişken ç = İlerleme(10); ç.güncelle({"kayıp": 0.5}, adım: 5); yazdır(ç)
```

İlişkili: `.TR_stop(...)`, `.TR_save(...)`

#### `.TR_upper(...)` (`TR_mem_upper`, `büyük_harf`)

    Metin.büyük_harf() → Metin

Metni Unicode'un varsayılan kuralıyla büyük harfe çevirir; sonuç programın dilinden bağımsızdır (i → I). Yeni bir metin verir.

```tr
TR_print("merhaba".TR_upper())
```

Görünümü:

```text
en: print("merhaba".upper())
tr: yazdır("merhaba".büyük_harf())
```

İlişkili: `.TR_lower(...)`

#### `.TR_v_buffers` (`TR_mem_v_buffers`, `v_tamponları`)



#### `.TR_v_proj` (`TR_mem_v_proj`, `değer_katmanı`)



#### `.TR_values(...)` (`TR_mem_values`, `değerler`)

    Sözlük.değerler() → Liste

Sözlüğün değerlerini anahtarların ekleniş sırasıyla bir liste olarak verir.

```tr
TR_print({"ad": "Ali", "puan": 30}.TR_values())
```

Görünümü:

```text
en: print({"ad": "Ali", "puan": 30}.values())
tr: yazdır({"ad": "Ali", "puan": 30}.değerler())
```

İlişkili: `.TR_keys(...)`

#### `.TR_vocab_size` (`TR_mem_vocab_size`, `dağarcık_boyutu`)

    Parçalayıcı.dağarcık_boyutu → Tam

Parçalayıcının dağarcığındaki parça sayısını (eklenmiş özel parçalar dahil) Tam olarak verir. Parantezsiz yazılan bir özelliktir.

```tr
TR_print(TR_load_tokenizer("tests/data/tokenizer.json").TR_vocab_size)
```

Görünümü:

```text
en: print(load_tokenizer("tests/data/tokenizer.json").vocab_size)
tr: yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").dağarcık_boyutu)
```

İlişkili: `.TR_token_id(...)`

#### `.TR_weight` (`TR_mem_weight`, `ağırlık`)



#### `.TR_weight_decay` (`TR_mem_weight_decay`, `ağırlık_çürümesi`)



#### `.TR_window` (`TR_mem_window`, `pencere`)



#### `.TR_zero_grad(...)` (`TR_mem_zero_grad`, `sıfırla`)

    İyileştirici.sıfırla() → YokTipi

İyileştiricinin parametrelerindeki biriken gradyanları sıfırlar. Bir sonraki geri_yayılım'dan önce çağrılır; argüman almaz.

```tr
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true); TR_backward(TR_sum(w * 2)); TR_AdamW([w]).TR_zero_grad(); TR_print(w.TR_grad.TR_tolist)
```

Görünümü:

```text
en: var w = Tensor([1.0], requires_grad: true); backward(sum(w * 2)); AdamW([w]).zero_grad(); print(w.grad.tolist)
tr: değişken w = Tensör([1.0], türev_izle: doğru); geri_yayılım(toplam(w * 2)); AdamW([w]).sıfırla(); yazdır(w.gradyan.değer)
```

İlişkili: `.TR_step(...)`

## Komutlar (tr, kabuk, hata ayıklayıcı)

### tr komutları

#### TR_cmd_build `derle`

Programı yorumlayıcıyla birlikte tek bir çalıştırılabilir dosyaya paketler.

```tr
tr build eğit.tr
```

#### TR_cmd_check `kontrol`

Programı çalıştırmadan ayrıştırır ve tip denetiminden geçirir.

```tr
tr check eğit.tr
```

#### TR_cmd_dap `dap`

Düzenleyicinin hata ayıklama protokolü (DAP) sunucusunu başlatır; düzenleyiciler kullanır.

```tr
tr dap
```

#### TR_cmd_debug `hata_ayıkla`

Programı terminal hata ayıklayıcısında başlatır.

```tr
tr debug eğit.tr
```

#### TR_cmd_format `biçimle`

TR dosyalarını standart biçime getirir.

```tr
tr format eğit.tr
```

#### TR_cmd_help `yardım`

Komutların ve seçeneklerin kısa yardımını yazar.

```tr
tr help
```

#### TR_cmd_index `dizin`

TR kodunun dizinini kurar ve ona sorar. Tek başına dizini kurar ve özetler; altında tanım, kullananlar, sınayanlar ve etki alt komutları vardır.

```tr
tr index
```

#### TR_cmd_profile `profil`

Biten ya da süren bir eğitimin profil raporunu yazar.

```tr
tr profile
```

#### TR_cmd_run `çalıştır`

Bir TR programını çalıştırır; komut adı yazılmadan verilen dosya da çalıştırılır.

```tr
tr run eğit.tr
```

#### TR_cmd_show `göster`

Kodlu dosyayı değiştirmeden etkin yazım dilinde yazar.

```tr
tr show eğit.tr
```

#### TR_cmd_test `test`

Bir dosyadaki ya da klasördeki *_test.tr testlerini koşar.

```tr
tr test tests
```

#### TR_cmd_version `sürüm`

TR'nin sürümünü yazar.

```tr
tr version
```

#### TR_cmd_watch `izle`

Çalışan bir eğitimin canlı profil tablosunu gösterir.

```tr
tr watch
```

### tr index alt komutları

#### TR_sub_definition `tanım`

Bir adın tanımını gösterir: nerede, imzası, belge yorumu, üyeleri (tr index altında).

```tr
tr index definition Muon
```

#### TR_sub_impact `etki`

Bir ad ya da dosya değişirse etkilenenleri gösterir: dışarıdan kullananlar, onu aktaran modüller, sınayan testler (tr index altında).

```tr
tr index impact lib/layers.tr
```

#### TR_sub_references `kullananlar`

Bir ada başvuranları gösterir: çağrı, okuma, yazma, aktarma, test; dosya:satır:sütun ve içeren bildirimle (tr index altında).

```tr
tr index references Doğrusal
```

#### TR_sub_rename `yeniden_adlandır`

Bir ad değişirse değişecek yerleri listeler, hiçbir dosyayı değiştirmez: kesin (tanım, parametre, adlı argüman, kendi.x ve yapıcıdan bilinen yerelin üye erişimi, komut_satırı anahtarı) ve olası (alıcısı bilinmeyen üye erişimi, belge, yorum, ileti metni) olarak ikiye ayrılır (tr index altında).

```tr
tr index rename baş bakış_sayısı
```

#### TR_sub_tested_by `sınayanlar`

Bir adı sınayan test bloklarını gösterir: önce doğrudan, sonra çağrı zinciriyle dolaylı olanlar (tr index altında).

```tr
tr index tested_by çevre_hesapla
```

### Komut satırı seçenekleri

#### TR_opt_all `tümü`

Dizin sorgularında satır sınırını kaldırır.

```tr
tr index references Muon --all
```

#### TR_opt_check `kontrol`

Dosyayı değiştirmeden biçiminin standart olup olmadığını denetler (tr format).

```tr
tr format --check eğit.tr
```

#### TR_opt_clear `temizle`

Dizin komutunda dizin önbelleğini siler.

```tr
tr index --clear
```

#### TR_opt_go_profile `go_profili`

Yorumlayıcının kendi Go profilini de yazar (tr profile).

```tr
tr profile --go-profile go.prof eğit.tr
```

#### TR_opt_help `yardım`

Programın komut satırı seçeneklerinin yardımını yazar (komut_satırı ile tanımlanan seçenekler).

```tr
tr eğit.tr --help
```

#### TR_opt_json `json`

Profil raporunu JSON olarak yazar (tr profile).

```tr
tr profile --json
```

#### TR_opt_kind `tür`

Sorgu adını yalnızca bu türdeki düğümlere çözer (function, struct, method, field, constant, variable, test, module, builtin).

```tr
tr index definition ileri --kind method
```

#### TR_opt_lang `dil`

Yazım dilini seçer (tr, en); --lang biçiminde yazılır.

```tr
tr show --lang en eğit.tr
```

#### TR_opt_limit `sınır`

Dizin sorgularında bir listedeki en çok satır sayısını verir (varsayılan 12).

```tr
tr index references Muon --limit 5
```

#### TR_opt_no_index `dizin_yok`

Dizini kurma ya da tazeleme, önbellekteki dizini olduğu gibi kullan (dizin sorgularında).

```tr
tr index definition Muon --no-index
```

#### TR_opt_no_text `metin_yok`

Yeniden adlandırma etkisinde belge, yorum ve ileti metni taramasını kapatır; yalnızca kodun anlamsal yerleri listelenir.

```tr
tr index rename baş bakış_sayısı --no-text
```

#### TR_opt_output `çıktı`

Canlı tablonun yazılacağı dosyayı seçer (tr watch).

```tr
tr watch --output tablo.txt
```

#### TR_opt_root `kök`

Proje kökünü verir; verilmezse .git bulunan üst klasör, o da yoksa çalışılan klasör alınır.

```tr
tr index definition Muon --root .
```

#### TR_opt_status `durum`

Dizin komutunda dizini kurmadan durumunu gösterir.

```tr
tr index --status
```

#### TR_opt_term `terim`

Yeniden adlandırma etkisinde sembol aramaz, verilen sözcüğü (ve Türkçe ekli biçimlerini) yalnızca metinde arar: bir terimin çevirisi değişince belge, yorum ve ileti metinlerini listelemek için. Sözcüğün birden çok anlamı varsa araç ayıramaz, her satırın bağlamı gösterilir.

```tr
tr index rename çekirdek kernel --term
```

### Kabuk komutları

#### TR_repl_exit `çıkış`

Etkileşimli kabuktan çıkar.

```tr
exit
```

### Hata ayıklayıcı komutları

#### TR_dbg_break `kesme`

Bir satıra kesme noktası koyar ya da kaldırır.

```tr
break 12
```

#### TR_dbg_continue `devam`

Sonraki kesme ya da izleme noktasına kadar devam eder (F5).

```tr
continue
```

#### TR_dbg_eval `hesapla`

Bir ifadeyi o anki ortamda hesaplar ve yazar.

```tr
eval x * 2
```

#### TR_dbg_help `yardım`

Hata ayıklayıcı komutlarının yardımını yazar.

```tr
help
```

#### TR_dbg_jump `zıpla`

Çalışma sırasını verilen satıra taşır.

```tr
jump 15
```

#### TR_dbg_next `geç`

Üstten geçer: fonksiyonu tek adımda çalıştırır (F10).

```tr
next
```

#### TR_dbg_out `dön`

Çalışan fonksiyondan çıkana kadar çalıştırır (Shift+F11).

```tr
out
```

#### TR_dbg_quit `çıkış`

Hata ayıklayıcıdan ve programdan çıkar.

```tr
quit
```

#### TR_dbg_screen `ekran`

Kaynak kodun duraklanan satır çevresini gösterir.

```tr
screen
```

#### TR_dbg_set `değiştir`

Çalışma anında bir değişkenin değerini değiştirir.

```tr
set x = 100
```

#### TR_dbg_stack `yığın`

Çağrı yığınını gösterir.

```tr
stack
```

#### TR_dbg_step `adım`

Tek adım ilerler; fonksiyonların içine girer (F11).

```tr
step
```

#### TR_dbg_table `tablo`

Değişkenlerin tablosunu gösterir.

```tr
table
```

#### TR_dbg_watch `izle`

Bir ifadeyi izleme listesine ekler; değeri her durakta gösterilir.

```tr
watch kayıp
```
