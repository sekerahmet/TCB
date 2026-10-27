# 3. Tipler ve değerler

Bugün TR dinamik tiplidir; değerler tiplerini taşır. İsteğe bağlı tip açıklamalarını yalnızca çalıştırma öncesi tip denetleyicisi kullanır (3.13). Karar statik tip ve tip çıkarımıdır (1.2); bu bölüm geçişle birlikte yeniden yazılır.

İlke: **Programın anlamı koda bağlıdır; değerin metni, işlemin yazıldığı dosyanın dilindedir; karşılaştırma değerle yapılır.** Bir değer metne çevrildiğinde (`TR_fn_str`, `TR_fn_print`, yerleştirme `{…}`, `+` ile birleştirme, `TR_fn_type()`), metin, o işlemin yazıldığı dosyanın yazım dilindedir (17.1, 17.7): Türkçe dosyada `TR_kw_true`, `TR_kw_none`, `TR_type_Int`; İngilizce dosyada `true`, `none`, `Int`. Programı çalıştıranın dili (`TR_DIL`) sonucu değiştirmez. Kitaplık fonksiyonu kendi dosyasının dilinde (Türkçe), geri çağrılan fonksiyon kendi dosyasının dilinde çevirir. Görüntüler (etkileşimli kabuğun yankısı, hata ayıklayıcı, İzleme Masası, iletilerin içindeki değerler) kişinin dilindedir; veri biçimleri (JSON, günlük, kontrol noktası) dilden bağımsızdır (19.5). Değerin metni başka bir dilin adıyla karşılaştırılırsa karşılaştırma hiç tutmaz: İngilizce dosyada `TR_fn_str(b) == "doğru"` her zaman `TR_kw_false`tır; değer değerle karşılaştırılır (`b == TR_kw_true`). Denetleyici, tipi bilinen bir `TR_type_Bool` değerinin (sabit, tip açıklamalı değişken, karşılaştırma) `TR_fn_str()`'i başka bir yazım dilinin doğru/yanlış adıyla karşılaştırılınca uyarır, çalışmayı durdurmaz (tip değerinde aynı karşılaştırma hatadır, 3.11); açıklamasız değişkenin tipi denetleyicide bilinmediği için orada uyarı yoktur.

**3.1** `TR_type_Int`: 64 bit işaretli tam sayı. `+ - * / **` işlemleri ve tekli `-` sınırı aşarsa (ör. en küçük değerin `/ -1`'i ya da eksisi) sessizce sarmak yerine "Tam sayı taşması" hatası verir. Yerleşikler de aynı kurala uyar: `TR_fn_sum`, `TR_fn_abs` (en küçük değerde), `TR_fn_int` ve `TR_fn_round` (Ondalık'tan çevirirken `TR_val_nan`, sonsuz ya da Tam aralığı dışındaki değerde) taşma hatası verir.

**3.2** `TR_type_Float`: 64 bit kayan noktalı sayı; en kısa gösterimiyle yazdırılır, tam sayılı ondalıklar `1.0` biçimindedir (`123456.0`). Büyüklüğü 1e6 ve üstü ya da 1e-4'ten küçük olanlar bilimsel gösterimle yazdırılır (`1e+06`, `1.6777216e+07`, `1e-05`; `0.0001` ise olduğu gibi). Sonsuz ve tanımsız değerlerin metni dilin değer adlarıdır: `sonsuz` (TR_val_inf), eksi sonsuz `-sonsuz`, tanımsız `NaN` (TR_val_nan); İngilizcede `inf`, `-inf`, `nan`. Sayı ve tensör aynı adı kullanır, ad işlemin yazıldığı dosyanın dilindedir (iletilerde ve kabukta kişinin dilinde). Değer adları ayrılmış sözcük değildir (`TR_val_inf` adlı bir değişken tanımlanabilir); kodda `TR_fn_float("sonsuz")` ile yazılır: `TR_fn_float()` dosyanın dilindeki adı, her dilde de `"inf"` ve `"nan"`'ı okur. Veri biçimleri (günlük, durum dosyaları) dilden bağımsız kendi yazımını kullanır: `"NaN"`, `"Infinity"`, `"-Infinity"` (19.5).

**3.3** `TR_type_String`: değiştirilemez Unicode metni; uzunluk ve indeks karakter (rune) bazındadır, negatif indeks sondan sayar, sınır dışı indeksle okuma hatadır (öğenin olmayabileceği bilinçli durumda `m.TR_mem_get(i)` öğe yoksa `TR_kw_none` verir, 3.6), `m[a..b]` alt metin verir. Dilimde de negatif uç indeksteki gibi sondan sayar (`"şeker"[-3..5]` → `"ker"`, `"şeker"[1..=-1]` → `"eker"`); uçlar metnin sınırlarına kırpılır, başlangıç bitişten büyükse sonuç boştur.

```tr
//! tr kodlu 3 dil=tr
"şeker".TR_mem_length   // 5
"şeker"[-1]        // "r"
"şeker"[1..3]      // "ek"
```

**3.4** `TR_type_Bool`: `TR_kw_true` ve `TR_kw_false`.

**3.5** `TR_kw_none`: değerin yokluğu; değer döndürmeyen fonksiyonlar, sözlükte olmayan anahtar ve `TR_mem_get` ile istenen olmayan öğe `TR_kw_none` verir. Liste ve metinde köşeli parantezle sınır dışı okuma `TR_kw_none` vermez, hatadır (3.3, 3.6).

**3.6** `TR_type_List`: sıralı, karışık tipli, değiştirilebilir dizi: `[1, "iki", 3.0]`. Negatif indeks sondan sayar; sınır dışı indeks okumada da yazmada da hatadır: hata indeksi ve uzunluğu söyler. Öğenin olmayabileceği bilinçli durumda `l.TR_mem_get(i)` öğe yoksa `TR_kw_none` verir, varsayılan `??` ile yazılır (`l.TR_mem_get(5) ?? 0`). Gerekçe: sessiz `TR_kw_none` program yanlışını saklar ve hata çok sonra, başka bir yerde çıkar; tensör indeksi (15.6) de sınır dışında hatadır. Sözlükte olmayan anahtar ise `TR_kw_none` verir (bir anahtarın olup olmadığını sormak olağan bir sorudur), `l[a..b]` dilim verir (dilim uçları Metin'deki gibidir, 3.3: `[1, 2, 3, 4][-2..4]` → `[3, 4]`). Listeler, sözlükler ve nesneler **başvuruyla** paylaşılır: `b = a` aynı listeyi gösterir.

**3.7** `TR_type_Dict`: eklenme sırasını koruyan anahtar–değer tablosu. Çıplak ad anahtarı metindir (`{ad: "Ali"}` ≡ `{"ad": "Ali"}`); ad dilin bir üye koduysa anahtar kodu taşır ve metni kodun dilden bağımsız adıdır (Türkçe `{adım: 1}` de İngilizce `{step: 1}` de `"step"` anahtarıdır; yazdırırken dosyanın dilindeki adla görünür, 17.3); metin olmayan anahtarlar da geçerlidir (`{1: "bir"}`). Sayı anahtarları sayı eşitliğine uyar (4.5): `d[1]` ile `d[1.0]` aynı girdidir, anahtar ilk eklendiği biçimiyle saklanır; `d["1"]` ise ayrı bir anahtardır. Anahtarlar genel olarak eşitliğe (4.5) uyar: fonksiyonlar, yerleşikler ve yapı tanımları kimlikle anahtardır (görünüşü aynı iki fonksiyon iki ayrı anahtardır); her metin, içeriğinden bağımsız olarak, yalnızca kendisine eşit metin anahtarıdır. İçeriği sonradan değişebilen değerler (liste, sözlük, yapı nesnesi, tensör gibi motor nesneleri) anahtar olamaz: bunlarla anahtar yazmak, okumak ya da `.TR_mem_contains()` sormak hatadır (değiştirilen bir anahtar sözlükte bir daha bulunamazdı). Olmayan anahtar `TR_kw_none` verir; `d.ad` biçimi `d["ad"]` ile aynıdır (okuma ve atama), ad dilin bir üye koduysa `d.ad` kodlu anahtarı, yoksa dosyanın dilindeki adıyla duran metin anahtarı bulur (17.3), `d["ad"]` ise yalnızca tam metni. Tek istisna sözlüğün kendi üyeleridir (`.TR_mem_length`, `.TR_mem_keys()`, `.TR_mem_values()`, `.TR_mem_contains()`): ad bir üye adıysa üye önceliklidir; `d.TR_mem_keys` aynı adlı anahtarı değil metodu verir, `d.TR_mem_length` eleman sayısıdır. Böyle bir anahtar `d["anahtarlar"]` ile okunur ve yazılır; `d.TR_mem_keys = 5` ataması hatadır.

**3.8** `TR_type_Range`: `a..b` (b hariç) ve `a..=b` (b dahil); uçlar `TR_type_Int` olmalıdır. `a > b` ise geriye doğru sayar (`3..0` → 3, 2, 1); `a..a` boş, `a..=a` tek elemanlıdır. Tam sınırındaki uçlar da taşmadan gezilir (`(üst - 1)..=üst`, üst = 9223372036854775807).

**3.9** Fonksiyonlar birinci sınıf değerlerdir: değişkene atanır, listeye konur, argüman olarak geçer, döndürülür.

**3.10** Doğruluk: `TR_kw_false`, `TR_kw_none`, `0`, `0.0`, boş metin, boş liste ve boş sözlük yanlış; diğer her değer doğru sayılır.

**3.11** `TR_fn_type(x)` değerin tipini bir **tip değeri** olarak verir; tip değerinin kendi tipi `TR_type_Type`'tir. Tip adları (`TR_type_Int`, `TR_type_Float`, `TR_type_String`, `TR_type_Bool`, `TR_type_None`, `TR_type_List`, `TR_type_Dict`, `TR_type_Range`, `TR_type_Function`, `TR_type_BuiltinFunction`, `TR_type_Tensor`, `TR_type_Type` ve motor tipleri: `TR_type_Optimizer`, `TR_type_Dataset`, `TR_type_Sampler`, `TR_type_SequentialReader`, `TR_type_Tokenizer`) ifadede tip değeridir; karşılaştırma bunlarla yapılır: `TR_fn_type(x) == TR_type_Int`. Bir yapı nesnesinin tipi yapının kendisidir: `TR_fn_type(p) == Nokta` (`TR_fn_type(Nokta)` ise `TR_type_StructType`dır). Tip değeri yazdırılınca ve metne çevrilince adı dosyanın dilinde görünür (`TR_fn_print(TR_fn_type(42))` → `TR_type_Int`, İngilizce dosyada `Int`; `TR_fn_str(TR_fn_type(p))` → `"Nokta"`). Tip değeri metinle karşılaştırılamaz: `TR_fn_type(x) == "Tam"` sessizce `TR_kw_false` vermez, hatadır; denetleyici yerleşik `TR_fn_type(...)` ile bir metin sabitinin karşılaştırılmasını çalıştırmadan önce bulur, çalışma anında da aynı hata verilir (ipucu: `tip(x) == Tam yazın`). Ayrı bir matris tipi yoktur: matris 2 boyutlu bir `TR_type_Tensor`dür (`TR_fn_tensor([[1, 2], [3, 4]])`, `TR_fn_eye(n)`; 15.1).

```tr
//! tr kodlu 3 dil=tr
TR_fn_print(TR_fn_type(42), TR_fn_type(42) == TR_type_Int, TR_fn_type([1]) != TR_type_String)   // Tam doğru doğru
TR_kw_struct Nokta { x = 0 }
TR_fn_print(TR_fn_type(Nokta()) == Nokta, TR_fn_type(TR_fn_type(1)))         // doğru Tip
```

```tr hata
//! tr kodlu 3 dil=tr
TR_fn_print(TR_fn_type(42) == "Tam")   // denetim hatası: tip değeri metinle karşılaştırılamaz
```

**3.12** Dönüşümler: `TR_fn_int(x)` (ondalığı keser, metni ayrıştırır), `TR_fn_float(x)`, `TR_fn_str(x)`; çevrilemeyen değer hata verir.

```tr
//! tr kodlu 3 dil=tr
TR_fn_int("42") + 1   // 43
TR_fn_int(3.9)        // 3
TR_fn_str(12) + "a" // "12a"
```

**3.13** Tip açıklamaları isteğe bağlıdır ve `ad: TR_type_Type` biçimindedir (değişken, parametre, yapı alanı; dönüş tipi için 6.8). Tip denetleyicisi bildirilen tiple çıkarılan tip uyuşmazsa ve `TR_kw_const` yeniden atanırsa programı **çalıştırmadan** reddeder. Denetim programın her yerine iner: ifadelerin içine (liste, sözlük, aralık, metin içi ifade, liste üreteci, indeks, üye), atama hedeflerine (`l[i] = ...`, `p.x = ...`), parametre ve yapı alanı varsayılanlarına, yapı metotlarının gövdelerine ve `TR_kw_match` desenlerine. Yalnızca açıklamalı adlar denetlenir: açıklamasız bir değişkenin ilk değerinden çıkarılan tip kalıcı bir bildirim sayılmaz, değişken sonra başka tipte değer alabilir (`TR_kw_var kayıp = 0` ardından `kayıp += 0.5` geçerlidir); `TR_kw_const`in tipi ise değerinin tipidir. Denetleyicinin çıkardığı tip çalışma zamanının sonucuyla aynıdır, ondan farklı bir tip iddia etmez: `TR_type_Int / TR_type_Int` ve negatif üslü `TR_type_Int ** TR_type_Int` "Tam ya da Ondalık" sayılır (4.2, 4.3), bu yüzden `TR_type_Int` açıklamalı bir ada atanamaz (`TR_fn_int(a / b)` ya da `TR_type_Float` açıklaması kullanılır); aralık ifadesinin tipi `TR_type_Range`tır, `TR_type_List` değil. Tip açıklamasında yalnızca dilin tip adları geçerlidir: `TR_type_Int`, `TR_type_Float`, `TR_type_String`, `TR_type_Bool`, `TR_type_None`, `TR_type_List`, `TR_type_Dict`, `TR_type_Range`, `TR_type_Function`, `TR_type_BuiltinFunction`, `TR_type_Tensor`, `TR_type_Type` ve yapı adları. Her tip adının tek yazımı vardır (3.14). Başka dillerin adları (`int`, `str`, `dict` ...) ve Türkçe harfsiz yazımlar kabul edilmez; denetleyici asıl adı önerir (`'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?`, `'int' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`; bkz. 2.9a).

```tr hata
//! tr kodlu 3 dil=tr
TR_kw_var yaş: TR_type_Int = 25
TR_kw_var ad: TR_type_Int = "Ali"   // denetim hatası: Tip uyuşmazlığı
```

**3.14** Tip açıklamasında tip adı yalnızca kendi yazımıyla geçerlidir (2.9a: her işin tek yazımı): `TR_type_Int`, `TR_type_Float`, `TR_type_BuiltinFunction`. Büyük/küçük harfi farklı bir yazım (`tam`, `MANTIK`, `ondalık`) tip adı sayılmaz; denetleyici programı çalıştırmadan reddeder ve asıl yazımı söyler: `'tam' TR'de bir tip adı değil; tip adının tek yazımı vardır: 'Tam' yazın`. Böylece kodlu saklamada (17.3) her tip açıklaması koduna çevrilir. Değer olarak okunan küçük harfli adlar (`TR_fn_int(x)`, `TR_fn_str(x)`) dönüşüm fonksiyonlarıdır (3.12), tip adı değildir.

```tr hata
//! tr kodlu 3 dil=tr
TR_kw_var adet: tam = 3   // denetim hatası: 'tam' TR'de bir tip adı değil; ... 'Tam' yazın
```

**3.15** `TR_type_Message`: `TR_fn_message(H6903, ...)` ile kurulan katalog iletisi (8.5). Kodu ve değerlerinden oluşur, kendi metni yoktur: metin gösterilirken kataloğun metninden, gösterildiği dilde kurulur (programın metni dosyanın dilinde, `TR_DIL` görüntüsü kişinin dilinde). `TR_kw_throw` ve `TR_fn_assert` bir iletiyi kendi koduyla fırlatır; `TR_fn_error_code` o kodu geri verir.
