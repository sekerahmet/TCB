# 2. Sözcük yapısı

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "2. Sözcük yapısı".

Her kuralın emsali ve gerekçesi: [`rationale/02_lexical_structure.md`](rationale/02_lexical_structure.md).

Bu dilde her şey kimlik koduyla yazılır (`TR_print`). İnsanların okuyabilmesi için kod Türkçeye, İngilizceye ya da başka bir dile çevrilir; ama hiçbir insan dilinde kod yazılmaz. Çevirinin kuralları ayrı bir bölümde tasarlanacak ([yol haritası](../roadmap.md)).

**2.1** Her `.tr` dosyası UTF-8 metindir; ilk satırı tam olarak `# TR 4` başlığıdır, başka bir başlık ya da başlıksızlık dosyayı çalıştırmaz ve nedenini söyler. Dosyanın dili yoktur: dilin sözcükleri kimlikleriyle yazılır, kullanıcının adları (2.3), metinler ve yorumlar kişinin yazdığı gibidir. İlk satır `#!` ile başlıyorsa (bir yorumdur, 2.2) başlık ikinci satırdır. Başlıktan sonra aktarmalar (1.3), sonra öbür tanımlar ve deyimler gelir. Satır sonu LF ya da CRLF; baştaki BOM yok sayılır, NUL (U+0000) sözdizimi hatasıdır. Konum dosyanın metnindedir: 1'den başlayan satır (başlık 1. satırdır) ve Unicode karakteri (bayt değil) sütunu; hatanın gösterdiği satır parçası da dosyanın metnidir. Bir dilin görünümü (17) başlığa dilini yazar (`# TR 4 view=tr`); görünüm çalışmaz, önce kodlanır.
- *Test:* TR testi, olumsuz fikstür.

**2.1a** Göründüğünden farklı okunan kaynak kabul edilmez. Yazı yönünü değiştiren karakterler (U+202A–U+202E, U+2066–U+2069, U+200E, U+200F, U+061C) hiçbir yerde, yorumda ve metinde de, yazılamaz; metinde gerekirse kaçışla (2.6). Adlarda görünmez dolgu harfleri (U+115F, U+1160, U+3164, U+FFA0) yazılamaz. Dosyanın adları ve aktardığı adlar arasında, Unicode benzerlik iskeleti (UTS #39) aynı olan iki ayrı ad hatadır (`yetki` ile son harfi Kiril `і` olan `yetkі`). İskeleti `TR_` ile başlayan ad kullanıcı adı olamaz: Kiril `Т` ile yazılmış `ТR_print` dilin sözcüğü gibi görünür (2.3a). Tek bir adda birden çok yazı sistemi uyarıdır (`θ_eski`); tek yazı sistemindeki ad (`öğrenme`, `привет`, `αβγ`) uyarısızdır.
- *Test:* TR testi, olumsuz fikstür.

**2.2** Yorum, metin dışındaki `#` ile başlar ve satır sonuna kadar sürer. Çok satırlı yorum yoktur; her satır `#` ile başlar. `//` yorum değildir, tam bölme işlecidir (sonraki bölümlerde); satır başındaki `//` ve `/*` sözdizimi hatasıdır, ileti `#`'i önerir.
- *Test:* TR testi, olumsuz fikstür.

```tr
# TR 4
TR_var x = 1  # satır sonuna kadar yorum
# birden çok satırlık açıklama
# her satırı # ile başlar
```

**2.3** Ad UAX #31'e uyar: bir harfle (XID_Start) ya da `_` ile başlar; harf, rakam, birleşen işaret ve `_` (XID_Continue) ile sürer; her yazı sisteminin harfleri geçerlidir (`öğrenci_sayısı2`). Adlar NFC biçimine göre karşılaştırılır: aynı görünen iki yazım aynı addır.
- *Test:* TR testi, olumsuz fikstür.

**2.3a** Dilin sözcükleri `TR_` ve kimliğin adıyla yazılır: `TR_if`, `TR_print(x)`, `x.TR_map(f)`, `TR_lr: 0.003`, `TR_Int`. Sınıfı yeri belirler: noktadan sonra üye, çağrıda iki noktadan önce argüman, öbür yerlerde anahtar kelime, bağlamsal sözcük, fonksiyon, tip ya da değer. Programın dışındaki adlar (komut, seçenek, ortam değişkeni, hata ayıklayıcı komutu) belgelerde sınıflarıyla yazılır (`TR_cmd_test`). `TR_` ile başlayan adlar kimliklere ayrılmıştır: kullanıcının adı `TR_` ile başlayamaz, iskeleti `TR_` ile başlayan ad da olamaz (2.1a); `tr_x`, `TRx` sıradan addır; o yerde geçerli olmayan `TR_` adı hatadır. İleti en yakın kimliği önerir: ad başka bir yerde geçerliyse doğru yeri (`TR_map(x)` → `x.TR_map(...)`), değilse o yerdeki kimliklerden adı en az değişiklikle tutanı (`TR_prnt` → `TR_print`). Değişiklik `TR_`'den sonraki adda sayılır, iki komşu harfin yer değiştirmesi tek değişikliktir (`TR_Flaot` → `TR_Float`); adın uzunluğunun üçte birini (en az bir) aşan değişiklikte öneri yoktur. İleti kimliğin `TR_`'siz yazımını ve başka dillerden tanıdık adlarını da tanır: o yerde tanımsız bir ad ya da geçerli olmayan bir `TR_` adı bir kimliğin adına ya da tanıdık adına uyuyorsa ileti o kimliği önerir (`print(x)` → `TR_print(x)`, `True` → `TR_true`, `None`, `null` ve `nil` → `TR_none`, `def` ve `TR_def` → `TR_func`, `argv` → `TR_command_line()`). Tanıdık adlar dilden bağımsızdır, kimliğin tanımında durur. Kitaplığın kimlikle adlanmış parametresi (`TR_min`) fonksiyonun gövdesinde aynı adlı yerleşiği gölgeler: ad önce çevreleyen fonksiyonun parametrelerinde aranır.
- *Test:* TR testi, olumsuz fikstür; Go testi (`TestSpellingAndSuggestion`, `TestNoTwoIdentitiesShareASpellingInAPlace`).

**2.3b** Ad yazılışı: dosyanın en dışındaki sabit `BÜYÜK_YILAN` (`TR_const EN_BÜYÜK = 10`), tip, yapı ve durumlu tipin durumları `BüyükDeve` (kısaltma bütünüyle büyük: `TR_GPT`), öbür adlar, yerel sabitler dahil, `küçük_yılan` (`öğrenci_sayısı`). Matematik adlarında en çok üç harf ve rakamlık parça büyük olabilir (`W`, `Wq`, `dW`, `W1`, `QKV`, `X_veri`). Baştaki `_` sayılmaz. Uymayan ad uyarıdır; uyarı doğru yazımı verir, büyük harfli ve hiç değişmeyen değişkene `TR_const` önerir.
- *Test:* TR testi, olumsuz fikstür.

**2.4** Tam sayı ondalık ya da ön ekle yazılır: `0x1F`, `0o17`, `0b1010`; ön ek yalnız küçük harftir (`0X1F` hatadır, ileti `0x1F`'yi önerir). Rakamlar arasına ve ön ekten sonra `_` yazılabilir (`1_000_000`, `0xFF_FF`), sonda ve ön ekin içinde yazılamaz. `TR_Int`'e sığmayan yazım sözdizimi hatasıdır; ileti ondalık yazımı önerir.
- *Test:* TR testi, olumsuz fikstür.

**2.5** Ondalık sayıda noktanın iki yanında rakam vardır: `3.14` (`.5` ve `1.` yazılmaz; `1..5` aralıktır). Üslü yazım: `e` ya da `E`, isteğe bağlı işaret ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); ardında rakam olmayan `e` sayıya ait değildir. Rakamlar arasına `_` yazılabilir (`1_000.5`).
- *Test:* TR testi.

**2.6** Metin çift tırnakla yazılır (`"..."`); metin ve yorum dışında tek tırnak sözdizimi hatasıdır, ileti çift tırnağı önerir. Kaçışlar: `\n \t \r \" \\` ve `\u{...}` (bir ile altı onaltılık rakamlı geçerli bir Unicode karakteri); `\(` yerleştirme başlatır (2.7). Bilinmeyen kaçış sözdizimi hatasıdır, ileti ham metni ya da `\\`'yi önerir. Metin satırlara yayılabilir; içindeki satır sonu her zaman `\n`dir. `TR_cmd_format` satır sonlarını LF yazar.
- *Test:* TR testi, olumsuz fikstür.

**2.6a** Ham metin iki ters tırnak (U+0060) arasındadır: kaçış ve yerleştirme yoktur, ters bölü ve tırnaklar olduğu gibi kalır; düzenli ifade, Windows yolu ve JSON için önerilir. Ham metin de `TR_String`'dir, satırlara yayılabilir (satır sonu `\n`), içine ters tırnak yazılamaz; kapanmayan ham metin hatadır. `TR_cmd_format` içeriğine dokunmaz, yalnız satır sonlarını LF yazar.
- *Test:* TR testi.

```tr
# TR 4
TR_var yol = `C:\veri\tokenizer.json`
TR_var tarih = `\d{4}-\d{2}-\d{2}`     # "\\d{4}-\\d{2}-\\d{2}" ile aynı
```

**2.7** Tırnaklı metindeki `\(ifade)` yerleştirmedir: ifade hesaplanıp metne yazılır. Süslü parantez sıradan karakterdir: `n` 3 iken `"{\"n\": \(n)}"` → `{"n": 3}`. Yerleştirme `\(`'in eşi olan parantezle kapanır, içindeki parantez ve metinler onu kapatmaz (`"\((1 + 2) * 3)"` → `9`); aynı satırda kapanmalıdır; boşsa ya da tek bir ifade değilse sözdizimi hatasıdır. İçindeki sözcükler dosyadaki gerçek konumlarıyla bildirilir. Tırnaklı metinde `{konu}` ya da `${konu}`, `konu` o yerde bir değişken, sabit ya da parametreyse uyarıdır: başka dillerin yerleştirme yazımıdır ve metne olduğu gibi geçer; uyarı `\(konu)`'ı önerir. Ham metinde (2.6a) yalnız `${konu}` uyarıdır; istem şablonu gibi `{konu}`'ı olduğu gibi tutan metin ham yazılır. Süslü parantezin içinde tek bir ad olmayan metin (`{\"n\": 1}`, `{{konu}}`) uyarı vermez.
- *Test:* TR testi, olumsuz fikstür.

```tr
# TR 4
TR_var ad = "Ali"
TR_var p = {"ad": "Ayşe"}
TR_print("Merhaba \(ad), 1 + 2 = \(1 + 2), kişi: \(p["ad"]), JSON: {\"adım\": \(1 + 2)}")
```

**2.8** Deyimler satır sonuyla ya da `;` ile ayrılır. Satır sonu ifadeyi bitirir, açık parantezin (`(`, `[`, sözlük `{`) içinde bitirmez. Yeni satırdaki `(` ya da `[` yeni deyim başlatır; sondaki blok `{` önceki ifadeyle aynı satırda olmalıdır. Satır başındaki ikili işleç önceki satırı sürdürmez: `-x` ayrı bir deyimdir. Çağrı içermeyen tek başına bir ifade (`-x`, `a + b`) değeri hiç kullanılmadığı için denetim hatasıdır; tekli biçimi olmayan işleç (`+`, `*`, `TR_and`, `??` ...) satır başında sözdizimi hatasıdır. Bölmek için işleç satır sonuna yazılır (`1 +` ↵ `2`) ya da parantez kullanılır. Zincir işleçleri satır başında sürer: `.` ve `?.`.
- *Test:* TR testi.

```tr hata
# TR 4
TR_var f = TR_sum
(1 + 2)                    # ayrı deyim, kullanılmayan değer hatası; 'TR_sum(1 + 2)' çağrısı değil
TR_var t = 1 +
    2                    # t = 3
```

**2.9** Anahtar kelimeler ayrılmıştır, ad olamaz. Her işin tek yazımı vardır: koşullu döngü `TR_for` ve bir koşulla, eşlemenin varsayılan kolu `_` ile yazılır. Başka dillerden alışkanlıkla yazılan `TR_while` ya da `TR_default` sözdizimi hatasıdır, ileti TR'deki yazımı söyler:

| Görev | Anahtar kelimeler |
| :-- | :-- |
| Bildirim | `TR_var`, `TR_const` |
| Fonksiyon | `TR_func`, `TR_return` |
| Koşul | `TR_if`, `TR_else`, `TR_match` |
| Döngü | `TR_for`, `TR_in`, `TR_break`, `TR_continue` |
| Değer ve mantık | `TR_true`, `TR_false`, `TR_none`, `TR_and`, `TR_or`, `TR_not` |
| Yapı | `TR_struct`, `TR_self` |
| Hata yönetimi | `TR_try`, `TR_catch`, `TR_throw` |
| Modül ve eşzamanlılık | `TR_import`, `TR_async`, `TR_defer` |
| Hata ayıklama | `TR_breakpoint` |

- *Test:* TR testi, olumsuz fikstür.

**2.9b** Bağlamsal sözcükler yalnız kendi yerlerinde sözcüktür: `TR_test` (test tanımının başında, ardından metin), `TR_as` (`TR_import` içinde), `TR_init` (yapının kurucu metodu), `_` (`TR_match`'te varsayılan kol ve durum deseninde atlanan alan; çoklu atamada, döngüde ve üreteçte atılan değer; okunamaz).
- *Test:* gerekmez: her bağlamsal sözcük kendi bağlamının kuralıyla sınanır.

**2.10** İfade ve bloklar en çok 1000 düzey iç içe olabilir (`((((1))))` dört düzey); aşan program sözdizimi hatası verir. Yerleştirmedeki ifade bulunduğu düzeyden sayılır.
- *Test:* TR testi.

**2.13** Çalışmadan önceki hatalar ve uyarılar (sözdizimi ve tip, 1.3; kullanım, 1.4) metin iletisinin yanında JSON olarak da verilebilir. Her kayıt ayrı alanlarda kodu, türü (hata ya da uyarı), dosyayı, başlangıç ve bitiş satır ve sütununu (2.1), iletiyi, varsa ilgili öbür konumları ve önerilen düzeltmeyi (2.3a, 2.3b) taşır; olduğu gibi uygulanabilecek düzeltme bunu bir işaretle söyler (satır başındaki `//` → `#`, `0X1F` → `0x1F`). İleti okuyanın dilindedir; kod, konum ve düzeltme dilden bağımsızdır. Seçenek ve alanların tam listesi sonraki bölümlerdedir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go e2e testi.
