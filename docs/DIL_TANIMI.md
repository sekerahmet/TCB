# TR Dil Tanımı

Bu belge TR dilinin **bugünkü uygulamaya** (`pkg/lexer`, `pkg/parser`, `pkg/checker`, `pkg/vm`) dayanan kısa tanımıdır. Dilek değil, davranış anlatır; bilinen eksikler §14'te açıkça listelenmiştir.

**Kural ve test eşlemesi.** Her kuralın bir numarası vardır (ör. **4.2**). `testler/` klasöründeki testlerin adı bu numarayla başlar (`test "4.2 aritmetik: ..."`). `tr test testler` komutu tanımdaki her kuralı çalıştırarak denetler; `go test ./pkg/testkosucu` de tanımda numarası geçip testi olmayan bir kural kalmadığını doğrular (Faz 3 kapısı: *her kural bir testte*).

İçindekiler: [2. Sözcük yapısı](#2-sözcük-yapısı) · [3. Tipler](#3-tipler-ve-değerler) · [4. Operatörler](#4-operatörler) · [5. Deyimler](#5-deyimler-ve-kontrol-yapıları) · [6. Fonksiyonlar](#6-fonksiyonlar) · [7. Yapılar](#7-yapılar) · [8. Hatalar](#8-hata-yönetimi) · [9. ertele](#9-ertele) · [10. Eşzamanlılık](#10-eşzamanlılık-ve-yorumlayıcı-kilidi) · [11. Modüller](#11-modüller) · [12. Testler](#12-testler) · [13. Yerleşikler](#13-yerleşik-fonksiyonlar-ve-metotlar) · [14. Sınırlamalar](#14-bilinen-sınırlamalar)

---

## 1. Program

Bir TR programı, yukarıdan aşağıya çalışan deyimlerden oluşan tek bir `.tr` dosyasıdır; `main` gibi bir giriş noktası yoktur. Çalıştırmadan önce dosya ayrıştırılır ve tip denetleyiciden geçer; ikisinden birinde hata varsa hiçbir satır çalışmaz.

```tr
yazdır("Merhaba, dünya!")
```

---

## 2. Sözcük yapısı

**2.1** Kaynak UTF-8 metindir; bazı Windows editörlerinin eklediği baştaki BOM (U+FEFF) yok sayılır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir.

**2.2** Yorumlar üç biçimdedir: satır sonuna kadar `#` ve `//`, çok satırlı `/* ... */`. Kapanmayan `/*` bir hatadır.

```tr
değişken x = 1  # Python tarzı
// C tarzı
/* çok
   satırlı */
```

**2.3** Tanımlayıcılar bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; Türkçe harfler (ç ğ ı İ ö ş ü) dahil tüm Unicode harfleri geçerlidir: `öğrenci_sayısı2`.

**2.4** Tam sayı sabitleri ondalık rakamlardan oluşur; okunabilirlik için `_` kullanılabilir: `1_000_000`.

**2.5** Ondalık sabitler nokta içerir ve noktadan sonra rakam gelmelidir: `3.14`. (`1..5` bir aralıktır, ondalık değil.)

**2.6** Metinler çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır; `\n \t \r \" \' \\` kaçış dizileri tanınır, bilinmeyen kaçışta ters bölü atılır. Metin birden çok satıra yayılabilir.

**2.7** Metin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır. Ayrıştırılamayan `{...}` olduğu gibi metinde kalır.

```tr
değişken ad = "Ali"
yazdır("Merhaba {ad}, 1 + 2 = {1 + 2}, kişi: {p['ad']}, düz: \{x\}")
```

**2.8** Deyimler satır sonuyla ya da isteğe bağlı `;` ile ayrılır. Satır sonu iki yerde anlam taşır: yeni satırda başlayan `(` ya da `[` önceki ifadeyi sürdürmez, yeni bir deyim başlatır; sondaki blok `{` (bkz. 6.7) önceki ifadeyle aynı satırda olmalıdır. Satır sonunda kalan ikili operatörden sonra ifade alt satırda sürer.

```tr
değişken f = 5
(1 + 2) |> yazdır        // ayrı deyim; '5(1 + 2)' çağrısı değil
değişken t = 1 +
    2                    // t = 3
```

**2.9** Anahtar kelimeler çift biçimlidir: Türkçe karakterli ve ASCII yazımlar eşdeğerdir.

| Anahtar kelime | Diğer yazımlar | | Anahtar kelime | Diğer yazımlar |
| :-- | :-- | :-- | :-- | :-- |
| `değişken` | `degisken` | | `doğru` / `yanlış` | `dogru` / `yanlis` |
| `sabit` | | | `yok` | |
| `fonksiyon` | `işlev`, `islev` | | `ve` / `veya` / `değil` | `degil` |
| `sonuç` | `sonuc`, `döndür`, `dondur` | | `yapı` / `sınıf` | `yapi` / `sinif` |
| `eğer` / `değilse` | `eger` / `degilse` | | `kendi` | |
| `eşle` / `varsayılan` | `esle` / `varsayilan` | | `dene` / `yakala` | |
| `döngü` / `içinde` | `dongu` / `icinde` | | `fırlat` | `firlat` |
| `iken` | | | `aktar` | |
| `çık` | `cik`, `kır`, `kir` | | `eşzamanlı` | `eszamanli` |
| `devam` | | | `ertele` | |
| `kesme_noktası` | `kesme_noktasi`, `dur_noktası`, `dur_noktasi`, `break_point` | | | |

Bağlamsal sözcükler anahtar kelime değildir ve ad olarak kullanılabilir: `test` (yalnızca deyim başında ardından metin gelince, bkz. 12.2), `olarak`/`as` (yalnızca `aktar` içinde), `_` (yalnızca `eşle` kolunda varsayılan).

---

## 3. Tipler ve değerler

TR dinamik tiplidir; değerler tiplerini taşır. İsteğe bağlı tip açıklamalarını yalnızca çalıştırma öncesi tip denetleyicisi kullanır (3.13).

**3.1** `Tam`: 64 bit işaretli tam sayı. `+ - * **` işlemleri sınırı aşarsa sessizce sarmak yerine "Tam sayı taşması" hatası verir.

**3.2** `Ondalık`: 64 bit kayan noktalı sayı; tam sayılı ondalıklar `1.0` biçiminde yazdırılır.

**3.3** `Metin`: değiştirilemez Unicode metni; uzunluk ve indeks karakter (rune) bazındadır, negatif indeks sondan sayar, sınır dışı indeks `yok` verir, `m[a..b]` alt metin verir.

```tr
uzunluk("şeker")   // 5
"şeker"[-1]        // "r"
"şeker"[1..3]      // "ek"
```

**3.4** `Mantık`: `doğru` ve `yanlış`.

**3.5** `yok`: değerin yokluğu; değer döndürmeyen fonksiyonlar, eksik argümanlar ve bulunamayan indeksler `yok` verir.

**3.6** `Liste`: sıralı, karışık tipli, değiştirilebilir dizi: `[1, "iki", 3.0]`. Negatif indeks sondan sayar, okuma sırasında sınır dışı indeks `yok` verir (yazmada hatadır), `l[a..b]` dilim verir. Listeler, sözlükler ve nesneler **başvuruyla** paylaşılır: `b = a` aynı listeyi gösterir.

**3.7** `Sözlük`: eklenme sırasını koruyan anahtar–değer tablosu. Çıplak ad anahtarı metindir (`{ad: "Ali"}` ≡ `{"ad": "Ali"}`); metin olmayan anahtarlar da geçerlidir (`{1: "bir"}`). Olmayan anahtar `yok` verir; `d.ad` biçimi `d["ad"]` ile aynıdır.

**3.8** `Aralık`: `a..b` (b hariç) ve `a..=b` (b dahil); uçlar `Tam` olmalıdır. `a > b` ise geriye doğru sayar (`3..0` → 3, 2, 1).

**3.9** Fonksiyonlar birinci sınıf değerlerdir: değişkene atanır, listeye konur, argüman olarak geçer, döndürülür.

**3.10** Doğruluk: `yanlış`, `yok`, `0`, `0.0`, boş metin, boş liste ve boş sözlük yanlış; diğer her değer doğru sayılır.

**3.11** `tip(x)` değerin tip adını metin olarak verir: `Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Aralık`, `Fonksiyon`, `YerleşikFonksiyon`, `Matris`, `Tensör`; bir yapı nesnesi için yapının adı (`Nokta`).

**3.12** Dönüşümler: `tam(x)` (ondalığı keser, metni ayrıştırır), `ondalık(x)`, `metin(x)`; çevrilemeyen değer hata verir.

```tr
tam("42") + 1   // 43
tam(3.9)        // 3
metin(12) + "a" // "12a"
```

**3.13** Tip açıklamaları `ad: Tip` biçimindedir (değişken, parametre, yapı alanı; dönüş tipi için 6.8). Tip denetleyicisi bildirilen tiple çıkarılan tip uyuşmazsa ve `sabit` yeniden atanırsa programı **çalıştırmadan** reddeder. Tip adlarının Türkçe/İngilizce eş adları vardır: `Tam`/`tamsayı`/`int`, `Ondalık`/`sayı`/`float`, `Metin`/`yazı`/`str`, `Mantık`/`bool`, `Liste`/`dizi`, `Sözlük`/`harita`/`dict`.

```tr
değişken yaş: Tam = 25
değişken ad: Tam = "Ali"   // denetim hatası: Tip uyuşmazlığı
```

---

## 4. Operatörler

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

**4.4** `< > <= >=` yalnızca sayılar arasında tanımlıdır; karşılaştırmalar zincirlenmez (`1 < 2 < 3` hatadır).

**4.5** Eşitlik: iki sayı tipten bağımsız değerle karşılaştırılır (`1 == 1.0`); diğer değerler aynı tipte ve aynı içerikteyse eşittir (`[1, "a"] == [1, "a"]`, `"1" != 1`, `yok != yanlış`).

**4.6** `ve`/`&&` ile `veya`/`||` kısa devre yapar ve sonucu her zaman `Mantık` olarak verir (`1 && "a"` → `doğru`). `!x` ve `değil x` doğruluğun tersidir.

**4.7** `a ?? b`: `a` `yok` ise `b`, değilse `a` (yalnızca `yok` sayılır: `0 ?? 5` → `0`).

**4.8** `a?.b`: `a` `yok` ise hata yerine `yok` verir; `yok.b` bir hatadır.

**4.9** `x |> f` ≡ `f(x)`; `x |> f(a)` ≡ `f(x, a)`. Sağ taraf bir çağrı ya da fonksiyon değeri olmalıdır.

```tr
[1, 2, 3] |> haritala { o * 2 } |> toplam   // 12
```

**4.10** `+` en az bir tarafı metinse öbürünü metne çevirip birleştirir (`"a" + 1` → `"a1"`); iki listeyi uç uca ekler.

**4.11** `@` matris çarpımıdır (`matris(...) @ matris(...)`); `+ - * /` matrislerde de çalışır.

---

## 5. Deyimler ve kontrol yapıları

**5.1** `değişken ad = ifade` (ya da kısa `ad := ifade`) bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse `yok` olur.

**5.2** `sabit AD = ifade` yeniden atanamayan bir ad tanımlar; ihlali tip denetleyicisi çalıştırmadan önce yakalar.

**5.3** Çoklu tanım ve atama virgülle yapılır; sağdaki tüm ifadeler atamadan önce hesaplanır (takas güvenlidir), sağda tek bir liste varsa elemanlarına açılır.

```tr
değişken a, b = 1, 2
a, b = b, a
değişken ad, soyad = ["Mustafa", "Kemal"]
```

**5.4** Atama `=` ile, bileşik atama `+= -= *= /=` ile yapılır; hedef bir ad, indeks (`l[0]`) ya da alan (`p.x`) olabilir. Tanımlanmamış bir ada atama hatadır (önce `değişken` ile tanımlanmalıdır).

**5.5** `eğer koşul { ... } değilse eğer koşul { ... } değilse { ... }` bir ifadedir: değeri çalışan bloğun son deyimidir; hiçbir dal çalışmazsa `yok`.

```tr
değişken tür = eğer n < 0 { "negatif" } değilse { "pozitif" }
```

**5.6** `eşle konu { desen => sonuç ... }` konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolu çalıştırır. Bir kolda virgülle birden çok desen olabilir; `Aralık` deseni içerme sınar; `varsayılan` ya da `_` her şeyle eşleşir. Kol gövdesi bir ifade, `{ blok }` ya da `sonuç ifade` olabilir; eşleşme yoksa `yok`.

```tr
eşle puan {
    100 => "tam puan"
    90..100 => "pekiyi"
    50, 60, 70 => "yuvarlak"
    varsayılan => "diğer"
}
```

**5.7** `döngü x içinde kaynak { ... }` listede, aralıkta, metinde (karakterler) ve sözlükte (anahtarlar) dolaşır. İki değişkenli biçim `döngü i, x içinde ...` liste/metinde indeks ile elemanı, sözlükte anahtar ile değeri verir.

**5.8** `döngü koşul { ... }` ve `iken koşul { ... }` koşul doğru oldukça tekrarlar.

**5.9** `döngü { ... }` sonsuz döngüdür; `çık` (ya da `kır`) en içteki döngüden çıkar.

**5.10** `devam` en içteki döngünün sonraki turuna geçer.

**5.11** Liste üreteci: `[ifade döngü x içinde kaynak eğer koşul]` (koşul isteğe bağlı).

```tr
[x * x döngü x içinde 1..=5 eğer x % 2 == 1]   // [1, 9, 25]
```

**5.12** Her `{ ... }` bloğu (eğer/döngü gövdesi dahil) yeni bir kapsam açar; içeride tanımlanan ad dışarıda görünmez, dıştaki adlar okunup atanabilir. Döngünün her turu kendi kapsamında çalışır.

---

## 6. Fonksiyonlar

**6.1** `fonksiyon ad(parametreler) { ... }` (ya da `işlev`) bir fonksiyon tanımlar ve adı bulunulan kapsama bağlar. Tanım çalıştırıldığı noktada gerçekleşir; öne alma (hoisting) yoktur, fonksiyon tanımından önce çağrılamaz.

**6.2** `sonuç ifade` (ya da `döndür`) fonksiyondan değer döndürür; yalnız `sonuç` `yok` döndürür. `sonuç` hiç çalışmazsa son deyimin değeri döner.

**6.3** Parametrelere varsayılan değer verilebilir (`ek = "!"`); varsayılanı olmayan eksik argüman `yok` olur, fazla argümanlar yok sayılır.

**6.4** Argümanlar adla da verilebilir: `böl(bölen: 2, bölünen: 10)`. Konumlu argümanlar önce yerleşir.

**6.5** Fonksiyonlar tanımlandıkları ortamı yakalar (kapanış); özyineleme desteklenir.

```tr
fonksiyon sayaç_yap() {
    değişken n = 0
    sonuç fonksiyon() { n += 1; sonuç n }
}
```

**6.6** Ok fonksiyonları: `x => ifade`, `(a, b) => ifade` ya da `(x) => { blok }`; adsız `fonksiyon(x) { ... }` de bir değerdir.

**6.7** Sondaki blok: bir çağrının, üye erişiminin ya da adın hemen ardından **aynı satırda** yazılan `{ ... }`, tek parametresi `o` olan bir fonksiyondur ve çağrının son argümanı olur: `l.filtrele { o > 2 }` ≡ `l.filtrele((o) => o > 2)`. İç içe bloklarda `o` en içteki elemanı gösterir. `eğer`, `iken`, `döngü` ve `eşle` başlıklarında `{` her zaman gövdeyi başlatır; orada sondaki blok için ifade paranteze alınır.

```tr
sayılar.filtrele { o > 2 }.haritala { o * o }
eğer (sayılar.filtrele { o > 2 }).uzunluk > 0 { ... }
```

**6.8** Dönüş tipi `: Tip` ya da `-> Tip` ile bildirilir; tip denetleyicisi `sonuç` değerlerini buna göre denetler.

```tr
fonksiyon topla(a: Tam, b: Tam): Tam { sonuç a + b }
```

---

## 7. Yapılar

**7.1** `yapı Ad { alanlar ve metotlar }` bir yapı tanımlar; alanlar `ad`, `ad: Tip` ya da `ad = varsayılan` biçimindedir ve virgül/satır sonuyla ayrılır.

```tr
yapı Nokta {
    x: Tam = 0
    y = 0
    fonksiyon uzaklık_kare() { sonuç kendi.x ** 2 + kendi.y ** 2 }
}
```

**7.2** Yapı adı çağrılarak nesne oluşturulur; argümanlar alan sırasıyla ya da adla verilir, verilmeyen alan varsayılanını (yoksa `yok`) alır: `Nokta(3, 4)`, `Nokta(y: 4)`.

**7.3** Metotlar `kendi` ile nesnenin alanlarına erişir ve onları değiştirebilir: `p.uzaklık_kare()`.

**7.4** `kurucu` adlı metot, nesne oluşturulurken aynı argümanlarla çağrılır. `sınıf`, `yapı` ile eş anlamlıdır.

**7.5** Alanlar `p.x = 7` ile atanabilir; olmayan alana erişmek hatadır (`p?.z` ise `yok` verir).

**7.6** Nesneler içerikleriyle karşılaştırılır (`Nokta(1, 2) == Nokta(1, 2)`) ve `Nokta{x: 1, y: 2}` biçiminde yazdırılır; `tip()` yapı adını verir.

---

## 8. Hata yönetimi

**8.1** `dene { ... } yakala ad { ... }`: dene bloğunda hata olursa yakala bloğu, hata mesajı `ad` değişkenine bağlanarak çalışır. Ad verilmezse `hata` kullanılır; `yakala (ad)` biçimi de geçerlidir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

**8.2** `fırlat ifade` bir hata başlatır; değer metne çevrilir ve yakalayan değişken her zaman `Metin`dir (`fırlat {"kod": 5}` → `"{\"kod\": 5}"`).

**8.3** Çalışma zamanı hataları (sıfıra bölme, tanımsız ad, tip uyuşmazlığı...) da yakalanabilir; mesajları `Satır N:` ile başlar. Yakalanmayan hata fonksiyon çağrılarından yukarı yayılır ve programı konum ve kod çerçevesiyle durdurur.

---

## 9. ertele

**9.1** `ertele çağrı` ya da `ertele { blok }`, içinde bulunduğu fonksiyon bittiğinde çalışacak işi kaydeder; birden çok ertele ters sırayla (son kaydedilen ilk) çalışır.

**9.2** Ertelenen iş fonksiyon `sonuç` ile erken bittiğinde ve hata ile çıkıldığında da çalışır.

**9.3** Bir blok (ör. `eğer` gövdesi) içindeki `ertele` bloğun değil, onu içeren fonksiyonun (en üst düzeyde programın, `eşzamanlı` içinde görevin, testte testin) sonunu bekler.

---

## 10. Eşzamanlılık ve yorumlayıcı kilidi

**10.1** `eşzamanlı { blok }` ya da `eşzamanlı çağrı(...)` işi ayrı bir görev olarak başlatır ve hemen devam eder; `bekle_hepsi()` başlatılmış tüm görevlerin bitmesini bekler. Program da bitmeden önce görevleri bekler. Görevdeki yakalanmamış hata programı durdurmaz, standart hataya yazılır.

**10.2** Yorumlayıcı kilidi: aynı anda yalnızca bir görev TR kodu çalıştırır; bu yüzden paylaşılan listeler/sözlükler bozulmaz ve `s += 1` gibi tek bir deyim bölünmez.

```tr
değişken toplam = 0
döngü i içinde 1..=200 { eşzamanlı { toplam += i } }
bekle_hepsi()      // toplam == 20100
```

**10.3** Engelleyici işlemler (`bekle(saniye)`, `bekle_hepsi()`, `girdi()`, ağ istekleri, dış süreç ve sunucu beklemeleri) kilidi bırakır; böylece bekleyen görevler iç içe ilerler (4 görevde `bekle(0.2)` toplam ~0.2 sn sürer).

**10.4** Görev, başlatıldığı kapsamı yakalar; döngü içinde başlatılan her görev o turun değişkenlerini görür.

---

## 11. Modüller

**11.1** `aktar "yol"` başka bir `.tr` dosyasını çalıştırır ve üst düzey adlarını bulunulan kapsama getirir. `.tr` uzantısı isteğe bağlıdır; yol önce çalışma klasörüne, bulunamazsa aktaran dosyanın klasörüne göre çözülür.

**11.2** `aktar "yol" olarak ad` (ya da `as`) modülün adlarını tek bir `Sözlük` olarak `ad`a bağlar: `ym.kare(2)`, `ym["PI"]`.

**11.3** Bir modül program boyunca bir kez çalıştırılır; sonraki aktarımlar aynı değerleri paylaşır. Döngüsel aktarım hatadır.

---

## 12. Testler

**12.1** `test "ad" { ... }` bir test tanımlar. `tr test` her testi sırayla çalıştırır; dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır.

```tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama doğru" {
    eşit_mi(4, topla(2, 2))
}
```

**12.2** `test` bağlamsal bir anahtar kelimedir: yalnızca bir deyimin başında ve ardından bir metin geldiğinde test başlatır; başka her yerde sıradan bir addır (`değişken test = 5`).

**12.3** `doğrula(koşul, [mesaj])` (ASCII: `dogrula`) koşul yanlışsa hata fırlatır: `doğrulama başarısız: mesaj`.

**12.4** `eşit_mi(beklenen, gerçek, [mesaj])` (ASCII: `esit_mi`) iki değer `==` anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır: `eşitlik başarısız: beklenen "5" (Metin), gerçek 5 (Tam)`. Her iki yerleşik test dışında da (ör. sözleşme denetimi için) kullanılabilir.

**12.5** Önce dosyanın kendisi (testler hariç) çalışıp üst düzey tanımları oluşturur; sonra her test bu tanımların üzerinde açılan **yeni bir kapsamda** çalışır. Bir testin yerel adları diğerine sızmaz; kalan ya da hata veren test diğerlerini durdurmaz. Testteki `ertele` test bitince çalışır, testin başlattığı görevler beklenir.

`tr test` kullanımı ve çıktı biçimi için `KULLANIM_KILAVUZU.md`'ye bakın.

---

## 13. Yerleşik fonksiyonlar ve metotlar

Tam liste `KULLANIM_KILAVUZU.md`'dedir; burada dilin parçası sayılan çekirdek seçki tanımlanır. Metotlar `değer.metot(...)` ile çağrılır; argümansız olanlar (`.uzunluk`, `.ilk`, `.son`) parantezsiz de yazılır.

**13.1** Genel: `yazdır(...)` (değerleri boşlukla birleştirip yazar), `uzunluk`, `tip`, `toplam`, `min`, `maks`, `mutlak`, `karekök`, `yuvarla`, `aralık(a, b)`, `filtrele`, `haritala`, `indirge`, `sırala`, `ekle`, `bekle`, `bekle_hepsi`, `zaman_ms`, `dosya_oku`, `dosya_yaz`, `girdi`.

**13.2** Metin metotları: `.uzunluk`, `.büyük_harf()` / `.küçük_harf()` (Türkçe `i/İ`, `ı/I` kurallarıyla), `.kırp()`, `.böl(ayırıcı)`, `.değiştir(eski, yeni)`, `.içerir(alt)`, `.başlar(önek)`, `.biter(sonek)`, `.ters_çevir()`.

**13.3** Liste metotları: `.uzunluk`, `.ilk`, `.son`, `.filtrele(f)`, `.haritala(f)`, `.sırala()`, `.ters_çevir()`, `.birleştir(ayırıcı)`, `.benzersiz()`, `.içerir(x)`, `.ekle(x)` (listeyi yerinde değiştirir), `.toplam()`, `.ortalama()`.

**13.4** Sözlük metotları: `.anahtarlar()`, `.değerler()`, `.içerir(anahtar)`.

Hata ayıklayıcıya bağlı `kesme_noktası` deyimi (`kesme_noktası`, `kesme_noktası eğer koşul`) etkileşimli olduğu için bu tanımın testlerine girmez; davranışı Go testleri ve `KULLANIM_KILAVUZU.md` §5 ile belgelenir.

---

## 14. Bilinen sınırlamalar

Bunlar bugünkü uygulamanın gerçek davranışlarıdır; ileride değişebilirler.

- **Metin karşılaştırması yok:** `"a" < "b"` hatadır; `<`, `>`, `<=`, `>=` yalnızca sayılarda çalışır. Zincirleme karşılaştırma (`1 < x < 3`) da yoktur.
- **Tam bölme operatörü yok:** `//` yorum başlatır; tam bölme için `tam(a / b)` kullanılır.
- **Testler arası yalıtım sığdır:** her test yeni bir kapsamda çalışır, ama üst düzey bir değişkene *atama* yapan ya da paylaşılan bir listeyi değiştiren test, sonraki testlerin gördüğü değeri değiştirir (üst düzey durum her testte yeniden kurulmaz).
- **Görev hataları testi düşürmez:** testin başlattığı `eşzamanlı` görevdeki hata standart hataya yazılır, test kalmaz.
- **`sabit` yalnızca ada bağlıdır:** ad yeniden atanamaz (denetleyici ve çalışma zamanı denetler), ama değer bir listeyse içeriği değişebilir; `aktar ... olarak m` ile alınan modül sözlüğünde sabitlik korunmaz (`m["AD"] = 1` çalışır).
- **Tip denetimi sığdır:** tipi çıkarılamayan her değer `Herhangi` sayılır; parametre tipleri yalnızca adıyla doğrudan çağrılan fonksiyonların konumlu argümanlarında denetlenir (adlı argümanlar, metotlar ve fonksiyon değerleri üzerinden yapılan çağrılar denetlenmez).
- **Sınır dışı okuma hata vermez:** `l[10]` ve `m[10]` `yok` döner; bu, hataların geç fark edilmesine yol açabilir.
- **Yapılarda kalıtım yok;** metotlar yapı tanımının içinde yazılmalıdır.
- **Metin içi yerleştirme konumu yaklaşıktır:** `{ifade}` içindeki hataların sütunu, ifadeden önce kaçış dizisi varsa bir-iki karakter kayabilir.
- **Tanımlar öne alınmaz:** fonksiyonlar ve yapılar, tanımlandıkları satır çalıştıktan sonra kullanılabilir.
