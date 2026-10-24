# 3. Tipler ve değerler

TR dinamik tiplidir; değerler tiplerini taşır. İsteğe bağlı tip açıklamalarını yalnızca çalıştırma öncesi tip denetleyicisi kullanır (3.13).

İlke: **Değerlerin metne çevrilmesi çalıştırma dilini izler; karşılaştırma değerle yapılır.** Bir program kodlu saklanıp başka bir yazım dilinde gösterildiğinde de aynı sonucu verir: `doğru`, `yok` ve tip değerleri yazdırılınca adları çalıştırma dilinde görünür, ama programın anlamı bu metne bağlı değildir.

**3.1** `Tam`: 64 bit işaretli tam sayı. `+ - * / **` işlemleri ve tekli `-` sınırı aşarsa (ör. en küçük değerin `/ -1`'i ya da eksisi) sessizce sarmak yerine "Tam sayı taşması" hatası verir. Yerleşikler de aynı kurala uyar: `toplam`, `mutlak` (en küçük değerde), `tam` ve `yuvarla` (Ondalık'tan çevirirken `NaN`, sonsuz ya da Tam aralığı dışındaki değerde) taşma hatası verir.

**3.2** `Ondalık`: 64 bit kayan noktalı sayı; en kısa gösterimiyle yazdırılır, tam sayılı ondalıklar `1.0` biçimindedir (`123456.0`). Büyüklüğü 1e6 ve üstü ya da 1e-4'ten küçük olanlar bilimsel gösterimle yazdırılır (`1e+06`, `1.6777216e+07`, `1e-05`; `0.0001` ise olduğu gibi). Sonsuz ve tanımsız değerlerin metni dilin değer adlarıdır: `sonsuz` (A110), eksi sonsuz `-sonsuz`, tanımsız `NaN` (A111); İngilizcede `inf`, `-inf`, `nan`. Sayı ve tensör aynı adı kullanır, ad işlemin yazıldığı dosyanın dilindedir (iletilerde ve kabukta kişinin dilinde). Değer adları ayrılmış sözcük değildir (`sonsuz` adlı bir değişken tanımlanabilir); kodda `ondalık("sonsuz")` ile yazılır: `ondalık()` dosyanın dilindeki adı, her dilde de `"inf"` ve `"nan"`'ı okur. Veri biçimleri (günlük, durum dosyaları) dilden bağımsız kendi yazımını kullanır: `"NaN"`, `"Infinity"`, `"-Infinity"` (19.5).

**3.3** `Metin`: değiştirilemez Unicode metni; uzunluk ve indeks karakter (rune) bazındadır, negatif indeks sondan sayar, sınır dışı indeksle okuma hatadır (öğenin olmayabileceği bilinçli durumda `m.varsa_al(i)` öğe yoksa `yok` verir, 3.6), `m[a..b]` alt metin verir. Dilimde de negatif uç indeksteki gibi sondan sayar (`"şeker"[-3..5]` → `"ker"`, `"şeker"[1..=-1]` → `"eker"`); uçlar metnin sınırlarına kırpılır, başlangıç bitişten büyükse sonuç boştur.

```tr
"şeker".uzunluk   // 5
"şeker"[-1]        // "r"
"şeker"[1..3]      // "ek"
```

**3.4** `Mantık`: `doğru` ve `yanlış`.

**3.5** `yok`: değerin yokluğu; değer döndürmeyen fonksiyonlar, sözlükte olmayan anahtar ve `varsa_al` ile istenen olmayan öğe `yok` verir. Liste ve metinde köşeli parantezle sınır dışı okuma `yok` vermez, hatadır (3.3, 3.6).

**3.6** `Liste`: sıralı, karışık tipli, değiştirilebilir dizi: `[1, "iki", 3.0]`. Negatif indeks sondan sayar; sınır dışı indeks okumada da yazmada da hatadır: hata indeksi ve uzunluğu söyler. Öğenin olmayabileceği bilinçli durumda `l.varsa_al(i)` öğe yoksa `yok` verir, varsayılan `??` ile yazılır (`l.varsa_al(5) ?? 0`). Gerekçe: sessiz `yok` program yanlışını saklar ve hata çok sonra, başka bir yerde çıkar; tensör indeksi (15.6) de sınır dışında hatadır. Sözlükte olmayan anahtar ise `yok` verir (bir anahtarın olup olmadığını sormak olağan bir sorudur), `l[a..b]` dilim verir (dilim uçları Metin'deki gibidir, 3.3: `[1, 2, 3, 4][-2..4]` → `[3, 4]`). Listeler, sözlükler ve nesneler **başvuruyla** paylaşılır: `b = a` aynı listeyi gösterir.

**3.7** `Sözlük`: eklenme sırasını koruyan anahtar–değer tablosu. Çıplak ad anahtarı metindir (`{ad: "Ali"}` ≡ `{"ad": "Ali"}`); metin olmayan anahtarlar da geçerlidir (`{1: "bir"}`). Sayı anahtarları sayı eşitliğine uyar (4.5): `d[1]` ile `d[1.0]` aynı girdidir, anahtar ilk eklendiği biçimiyle saklanır; `d["1"]` ise ayrı bir anahtardır. Anahtarlar genel olarak eşitliğe (4.5) uyar: fonksiyonlar, yerleşikler ve yapı tanımları kimlikle anahtardır (görünüşü aynı iki fonksiyon iki ayrı anahtardır); her metin, içeriğinden bağımsız olarak, yalnızca kendisine eşit metin anahtarıdır. İçeriği sonradan değişebilen değerler (liste, sözlük, yapı nesnesi, tensör gibi motor nesneleri) anahtar olamaz: bunlarla anahtar yazmak, okumak ya da `.içerir()` sormak hatadır (değiştirilen bir anahtar sözlükte bir daha bulunamazdı). Olmayan anahtar `yok` verir; `d.ad` biçimi `d["ad"]` ile aynıdır (okuma ve atama). Tek istisna sözlüğün kendi üyeleridir (`.uzunluk`, `.anahtarlar()`, `.değerler()`, `.içerir()`): ad bir üye adıysa üye önceliklidir; `d.anahtarlar` aynı adlı anahtarı değil metodu verir, `d.uzunluk` eleman sayısıdır. Böyle bir anahtar `d["anahtarlar"]` ile okunur ve yazılır; `d.anahtarlar = 5` ataması hatadır.

**3.8** `Aralık`: `a..b` (b hariç) ve `a..=b` (b dahil); uçlar `Tam` olmalıdır. `a > b` ise geriye doğru sayar (`3..0` → 3, 2, 1); `a..a` boş, `a..=a` tek elemanlıdır. Tam sınırındaki uçlar da taşmadan gezilir (`(üst - 1)..=üst`, üst = 9223372036854775807).

**3.9** Fonksiyonlar birinci sınıf değerlerdir: değişkene atanır, listeye konur, argüman olarak geçer, döndürülür.

**3.10** Doğruluk: `yanlış`, `yok`, `0`, `0.0`, boş metin, boş liste ve boş sözlük yanlış; diğer her değer doğru sayılır.

**3.11** `tip(x)` değerin tipini bir **tip değeri** olarak verir; tip değerinin kendi tipi `Tip`'tir. Tip adları (`Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Aralık`, `Fonksiyon`, `YerleşikFonksiyon`, `Tensör`, `Tip` ve motor tipleri: `İyileştirici`, `VeriKümesi`, `Örnekleyici`, `SıralıOkuyucu`, `Parçalayıcı`) ifadede tip değeridir; karşılaştırma bunlarla yapılır: `tip(x) == Tam`. Bir yapı nesnesinin tipi yapının kendisidir: `tip(p) == Nokta` (`tip(Nokta)` ise `YapıTanımı`dır). Tip değeri yazdırılınca ve metne çevrilince adı görünür (`yazdır(tip(42))` → `Tam`, `metin(tip(p))` → `"Nokta"`). Tip değeri metinle karşılaştırılamaz: `tip(x) == "Tam"` sessizce `yanlış` vermez, hatadır; denetleyici yerleşik `tip(...)` ile bir metin sabitinin karşılaştırılmasını çalıştırmadan önce bulur, çalışma anında da aynı hata verilir (ipucu: `tip(x) == Tam yazın`). Ayrı bir matris tipi yoktur: matris 2 boyutlu bir `Tensör`dür (`tensör([[1, 2], [3, 4]])`, `birim_matris(n)`; 15.1).

```tr
yazdır(tip(42), tip(42) == Tam, tip([1]) != Metin)   // Tam doğru doğru
yapı Nokta { x = 0 }
yazdır(tip(Nokta()) == Nokta, tip(tip(1)))         // doğru Tip
```

```tr hata
yazdır(tip(42) == "Tam")   // denetim hatası: tip değeri metinle karşılaştırılamaz
```

**3.12** Dönüşümler: `tam(x)` (ondalığı keser, metni ayrıştırır), `ondalık(x)`, `metin(x)`; çevrilemeyen değer hata verir.

```tr
tam("42") + 1   // 43
tam(3.9)        // 3
metin(12) + "a" // "12a"
```

**3.13** Tip açıklamaları isteğe bağlıdır ve `ad: Tip` biçimindedir (değişken, parametre, yapı alanı; dönüş tipi için 6.8). Tip denetleyicisi bildirilen tiple çıkarılan tip uyuşmazsa ve `sabit` yeniden atanırsa programı **çalıştırmadan** reddeder. Denetim programın her yerine iner: ifadelerin içine (liste, sözlük, aralık, metin içi ifade, liste üreteci, indeks, üye), atama hedeflerine (`l[i] = ...`, `p.x = ...`), parametre ve yapı alanı varsayılanlarına, yapı metotlarının gövdelerine ve `eşle` desenlerine. Yalnızca açıklamalı adlar denetlenir: açıklamasız bir değişkenin ilk değerinden çıkarılan tip kalıcı bir bildirim sayılmaz, değişken sonra başka tipte değer alabilir (`değişken kayıp = 0` ardından `kayıp += 0.5` geçerlidir); `sabit`in tipi ise değerinin tipidir. Denetleyicinin çıkardığı tip çalışma zamanının sonucuyla aynıdır, ondan farklı bir tip iddia etmez: `Tam / Tam` ve negatif üslü `Tam ** Tam` "Tam ya da Ondalık" sayılır (4.2, 4.3), bu yüzden `Tam` açıklamalı bir ada atanamaz (`tam(a / b)` ya da `Ondalık` açıklaması kullanılır); aralık ifadesinin tipi `Aralık`tır, `Liste` değil. Tip açıklamasında yalnızca dilin tip adları geçerlidir: `Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Aralık`, `Fonksiyon`, `YerleşikFonksiyon`, `Tensör`, `Tip` ve yapı adları. Her tip adının tek yazımı vardır (3.14). Başka dillerin adları (`int`, `str`, `dict` ...) ve Türkçe harfsiz yazımlar kabul edilmez; denetleyici asıl adı önerir (`'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?`, `'int' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`; bkz. 2.9a).

```tr hata
değişken yaş: Tam = 25
değişken ad: Tam = "Ali"   // denetim hatası: Tip uyuşmazlığı
```

**3.14** Tip açıklamasında tip adı yalnızca kendi yazımıyla geçerlidir (2.9a: her işin tek yazımı): `Tam`, `Ondalık`, `YerleşikFonksiyon`. Büyük/küçük harfi farklı bir yazım (`tam`, `MANTIK`, `ondalık`) tip adı sayılmaz; denetleyici programı çalıştırmadan reddeder ve asıl yazımı söyler: `'tam' TR'de bir tip adı değil; tip adının tek yazımı vardır: 'Tam' yazın`. Böylece kodlu saklamada (17.3) her tip açıklaması koduna çevrilir. Değer olarak okunan küçük harfli adlar (`tam(x)`, `metin(x)`) dönüşüm fonksiyonlarıdır (3.12), tip adı değildir.

```tr hata
değişken adet: tam = 3   // denetim hatası: 'tam' TR'de bir tip adı değil; ... 'Tam' yazın
```
