# 3. Tipler ve değerler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "3. Tipler ve değerler".

Her kuralın emsali ve gerekçesi: [`rationale/03_types.md`](rationale/03_types.md).

TR statik tiplidir (1.2): her değerin tipi çalışmadan önce bilinir. Tip denetimi 3.13'te, değerin metni 3.14'tedir.

**3.1** `TR_Int`: 64 bit işaretli tam sayı (−9223372036854775808 … 9223372036854775807). İşlemin ya da yerleşiğin (`TR_sum`, `TR_abs`) sonucu aralığı aşarsa sarılmaz, "tam sayı taşması" çalışma hatası verir; ileti ondalık sonucu ve sarılan işlemi birlikte önerir. Sarılan (modüler) toplama, çıkarma ve çarpma `TR_wrapping_add`, `TR_wrapping_sub` ve `TR_wrapping_mul` ile açıkça yazılır (13.1): hash ve rastgele sayı üreteci gibi bit işleri içindir.
- *Test:* TR testi.

**3.2** `TR_Float`: IEEE 754 64 bit. Metne, geri okununca aynı sayıyı veren en kısa rakamlarla çevrilir: tam değerli ondalık bir sıfırla ("1.0", "1234567.0", "-0.0"); üssü −4'ten küçük ya da 16 ve üstüyse bilimsel, üs her zaman işaretli ve en az iki rakamlı ("1e-05", "1e+16"; "0.0001", "1000000000000000.0" olduğu gibi). Sonsuz ve tanımsız `TR_inf` ve `TR_nan`dır. Daha küçük ondalık ve tam sayı tipleri (32, 16 ve 8 bit, nicemlenmiş) yalnız tensörün öğe tipidir (15.1, 15.51).
- *Test:* TR testi.

**3.3** `TR_String`: değiştirilemez Unicode metni; uzunluk ve indeks kod noktası sayar. `m[i]` tek karakterlik bir `TR_String` verir; negatif indeks sondan sayar; sınır dışı indeks çalışma hatasıdır, `.TR_get(i)` `TR_String?` verir (3.5). `m[a..b]` alt metin verir; dilimde negatif uç sondan sayılır, uçlar sınıra kırpılır, başlangıç bitişten büyükse sonuç boştur (`"şeker"[-3..5]` → "ker").
- *Test:* TR testi.

```tr
# TR 4
TR_print("şeker".TR_length, "şeker"[-1], "şeker"[1..3])   # çıktı: 5 r ek
```

**3.3a** `TR_Bytes`: değiştirilemez bayt dizisi (dosya, ağ, görüntü, ses verisi). `b[i]` 0–255 arası bir `TR_Int`, `b[a..c]` bir `TR_Bytes` verir; indeks ve dilim 3.3'teki gibidir. `TR_Bytes("ab")` metnin UTF-8 baytlarını, `TR_Bytes([0, 255])` bir tam sayı listesini verir; 0–255 dışındaki sayı çalışma hatasıdır. `TR_String(b)` UTF-8 okur; geçersiz UTF-8 çalışma hatasıdır. Metin ile bayt kendiliğinden dönüşmez. Birleştirme ve yineleme 4.10'dadır; bayttan sayı okuyup sayıyı bayta yazma (bayt sırasıyla), tensörden kurma ve parça parça yazma yol haritasındadır.
- *Test:* TR testi.

**3.4** `TR_Bool`: `TR_true` ve `TR_false`; koşulda yalnız bu tip kullanılır (3.10).
- *Test:* TR testi.

**3.5** `TR_none` yokluktur, `TR_NoneType`'ın tek değeridir. Değişken, parametre, alan ya da dönüş ancak tipi isteğe bağlıysa (`TR_Int?`) `TR_none` alabilir; ilk değeri `TR_none` olan değişken tipini yazar. İsteğe bağlı değer asıl tipin yerinde kullanılamaz; `??` ile varsayılan verilerek, `?.` ile ya da `TR_none`'la karşılaştırılarak açılır.
- Daraltma: `x` yerel bir değişkense, o yolda ona atama yapılmıyorsa ve onu değiştiren bir kapanış yoksa `x` asıl tipindedir:
  - `x != TR_none` sınamasının dalında ve `x == TR_none` sınamasının `TR_else` dalında;
  - `TR_and`'in sağ yanında, sol yan `x != TR_none` ise; `TR_or`'un sağ yanında, sol yan `x == TR_none` ise;
  - gövdesi her yolda biten (`TR_return`, `TR_break`, `TR_continue`, `TR_throw`, `TR_exit`) bir `TR_if x == TR_none { ... }`'dan sonra, bloğun sonuna kadar;
  - asıl tipten bir değer atanınca, sonraki atamaya kadar: `TR_if x == TR_none { x = 0 }`'dan sonra.
- Yalnız değişkenin kendisine atama daraltmayı bozar; alanına atama (`x.alan = ...`) bozmaz. Alan ve indeks daraltılmaz; `??` ya da yerel bir kopyayla açılır.
- İsteğe bağlı olmayan bir değeri `TR_none`'la karşılaştırmak denetim hatasıdır (sonuç hep aynıdır).
- Değer döndürmeyen fonksiyonun çağrısı değer olarak kullanılamaz (denetim hatası); `TR_NoneType` yalnız fonksiyon tipinin dönüşü (6.9) ve tip değeri olarak yazılır.
- *Test:* TR testi; isteğe bağlı tip uygulanınca denetleyicinin testi.

```tr
# TR 4
TR_var l: TR_List[TR_Int] = [10, 20, 30]
TR_var x: TR_Int? = l.TR_get(5)
TR_print(x ?? 0)                 # çıktı: 0
TR_if x != TR_none { TR_print(x + 1) }   # bu dalda x bir TR_Int
```

**3.6** `TR_List[T]`: tek tipli, sıralı, değiştirilebilir dizi; tip parametresi köşeli parantezle yazılır. `[1, 2, 3]` bir `TR_List[TR_Int]`, `[1, 2.5]` bir `TR_List[TR_Float]`dır (değişmezde bir ondalık varsa tam sayı yazımları ondalıktır; `TR_none` varsa öğe tipi isteğe bağlıdır: `[1.0, TR_none]` bir `TR_List[TR_Float?]`); boş liste tipini yazar (`TR_var l: TR_List[TR_Int] = []`). Negatif indeks sondan sayar; sınır dışı okuma ve yazma çalışma hatasıdır, ileti indeksi ve uzunluğu söyler; `.TR_get(i)` `T?` verir. Dilim 3.3'teki gibidir. Liste, sözlük ve yapı nesnesi başvuruyla paylaşılır: `b = a`'dan sonra ikisi aynı listedir. Bu yüzden `TR_List[TR_Int]`, `TR_List[TR_Float]` beklenen yere verilmez. Kaplar tip parametresinde böyle değişmezdir: `TR_Dict[TR_String, TR_Tensor]` de bir `TR_Dict[TR_String, TR_AnyTensor]` değildir (15.1); her cinsten sözlük alan yerleşikler bunu 13.10'un kabul kümesiyle yazar.
- Liste, sözlük, metin ve bayt dizisi her büyümede (`+`, `*`, `.TR_append`, `+=`, sözlüğe ekleme) belleği ayırmadan önce denetler, tensördeki gibi (15.42): makinenin belleğini aşan büyüme çalışma hatasıdır; öğe sayısının elle seçilmiş bir sınırı yoktur.
- *Test:* TR testi; tek tipli liste uygulanınca denetleyicinin testi.

**3.6a** Demet `(A, B, ...)`: sabit sayıda, her biri kendi tipinde değeri bir arada tutan değiştirilemez değer; `(1, "a")` bir `(TR_Int, TR_String)`'dir. En az iki öğesi vardır, `(x)` parantezdir. Tip yerinde de parantez gruplar: `(TR_Int)` `TR_Int`'tir, `((TR_Int) -> TR_Bool)?` isteğe bağlı fonksiyon tipidir; demet tipi en az iki öğelidir. Öğeler `t.0`, `t.1` ile okunur, atanamaz; iç içe demette `t.0.1` `(t.0).1`'dir (sözcük çözümleyici noktadan sonraki `0.1`'i iki indekse böler, ondalık okumaz). Sınır dışı indeks denetim hatasıdır; tipi bilinmediği yerde çalışma hatası. Birden çok değer döndüren fonksiyon (6.3) bir demet döndürür; demet 5.3'teki gibi açılır (`TR_var a, b = t`). `==` öğe öğedir (4.5; tensör içeren demette denetim hatası, 15.5); öğeleri sıralanabilen demet sözlük sırasıyla karşılaştırılır (`(1, 2) < (1, 3)`, 4.4); öğeleri anahtar olabilen demet sözlük anahtarı olabilir (3.7): `TR_Dict[(TR_Int, TR_Int), TR_Int]`. Metni "(1, "a")", tipinin metni "(Int, String)"dir. `TR_json_string` demeti dizi olarak yazar; `TR_json_parse[T]`'de alanın tipi demetse dizi demete kurulur, öğe sayısı ya da öğe tipi tutmazsa hatadır; tipsiz `TR_json_parse` diziyi `TR_JSON` olarak okur (13.8). Kontrol noktasının düz değerlerinde (20.2) demet yoktur: kaydetmek hatadır.
- *Test:* TR testi.

**3.7** `TR_Dict[K, V]`: eklenme sırasını koruyan tablo (`{"k": 1}`); anahtar tipi `TR_Int`, `TR_String`, `TR_Bool`, `TR_Bytes`, öğeleri anahtar olabilen demet (3.6a) ya da alanlarının hepsi anahtar olabilen bir durumlu tip (7.7); boş sözlük tipini yazar. Değişmezdeki anahtar bir ifadedir: `{k: 1}`'de `k` değişkendir. Anahtar yalnız köşeli parantezle okunur ve yazılır (`d["k"]`); nokta yalnız üyeleri verir (`.TR_length`, `.TR_keys()`, `.TR_values()`, `.TR_contains(k)`, `.TR_get(k)`). Olmayan anahtarı köşeli parantezle okumak çalışma hatasıdır; `.TR_get(k)` `V?` verir. Tensör içeren sözlüğün `==`'si denetim hatasıdır (15.5).
- *Test:* TR testi.

**3.7a** `TR_JSON`: yapısı önceden bilinmeyen veri; JSON'un her değerini tutar: yokluk, mantık değeri, sayı, metin, dizi, nesne.
- **Kurmak:** `TR_json_parse(m)` bir `TR_JSON` verir. Beklenen tip `TR_JSON` olan değişmez JSON kurar; içinde metin, sayı, mantık değeri, `TR_none`, liste, metin anahtarlı sözlük ve `TR_JSON` bulunabilir (`TR_var g: TR_JSON = {"k": "a", "n": 512}`). JSON'da olmayan `TR_inf` ve `TR_nan` `TR_JSON`'a giremez: çözmede ve kurmada çalışma hatasıdır. Nesnede yinelenen anahtar çözmede çalışma hatasıdır, ileti anahtarı ve iki yerini söyler.
- **Okumak:** `j["k"]` ve `j[i]` yine `TR_JSON` verir; olmayan alan ve sınır dışı indeks çalışma hatasıdır. `.TR_get(k)` `TR_JSON?` verir: eksik alan `TR_none`'dır, var olan JSON yokluğu ise bir `TR_JSON` değeridir. `.TR_length` nesnede ve dizide, `.TR_keys()` yalnız nesnede çalışır; tür tutmazsa çalışma hatasıdır.
- **Türü sormak:** `TR_Type(j)` değerin JSON türünü tip olarak verir: `TR_NoneType`, `TR_Bool`, `TR_Int` (tam yazılmış ve `TR_Int`'e sığan sayı), `TR_Float` (öbür sayılar), `TR_String`, `TR_List[TR_JSON]`, `TR_Dict[TR_String, TR_JSON]`.
- **Açmak:** tip çağrısı JSON değerini açar: `TR_String(j)` metni, `TR_Int(j)` tam yazılmış ve `TR_Int`'e sığan sayıyı, `TR_Float(j)` her sayıyı, `TR_Bool(j)` mantık değerini, `TR_List(j)` diziyi `TR_JSON` değerlerinin listesi olarak verir; tür tutmazsa çalışma hatasıdır, metin sayı olarak okunmaz.
- **Karşılaştırmak:** `==` ve `!=` iki `TR_JSON` değerini JSON değeri olarak karşılaştırır (4.5). JSON değeriyle JSON olmayan bir değerin karşılaştırması çalışma hatasıdır; ileti açan yazımı önerir (`TR_Int(j) == 1`). `TR_none` ile karşılaştırma hata değildir ve her zaman eşit değildir: JSON yokluğu bir `TR_JSON` değeridir, `TR_Type(j) == TR_NoneType` ile sorulur.
- **Dolaşmak:** dizide öğeler, nesnede anahtarlar (5.7); öbür türler dolaşılamaz.
- **Yazmak:** `TR_json_string(j)` her zaman JSON metnini verir (metin tırnaklı: `"\"merhaba\""`). Değerin metni (`TR_print(j)`, yerleştirme, 3.14) JSON metin türünde metnin kendisidir (`"\(j["k"])"` → merhaba), öbür türlerde JSON metnidir.
- Her değeri tutan genel bir tip yoktur.
- Güvenilmeyen JSON'un iç içeliği en çok 1000'dir; daha derini çözülmeden hatadır (kaynak dosyanın sınırı gibi, 2.10).
- *Test:* TR testi.

```tr
# TR 4
TR_var cevap = TR_json_parse(`{"content": [{"text": "merhaba"}], "id": 7}`)
TR_var metin = TR_String(cevap["content"][0]["text"])   # merhaba
TR_if TR_Type(cevap["id"]) == TR_Int { TR_print(TR_Int(cevap["id"]) + 1) }
TR_var istek: TR_JSON = {"soru": metin, "en_fazla": 512, "akış": TR_false}
```

**3.8** `TR_Range`: `a..b` (b hariç), `a..=b` (b dahil); uçlar `TR_Int`. `a >= b` ise `a..b`, `a > b` ise `a..=b` boştur; geriye `.TR_reverse()` sayar ve bir `TR_List` verir (`(0..3).TR_reverse()` → `[2, 1, 0]`). Sınırdaki uçlar taşmadan gezilir.
- *Test:* TR testi.

**3.9** Fonksiyon birinci sınıf değerdir: atanır, listeye konur, argüman olur, döndürülür. Tipi parametrelerinin ve dönüşünün tipleridir; yazımı 6.9'dadır.
- *Test:* TR testi.

**3.10** Koşul `TR_Bool` olmalıdır: `TR_if` ve koşullu `TR_for`'un koşulu, liste üretecinin koşulu, `TR_breakpoint TR_if`'in koşulu, `.TR_filter`'ın yükleminin sonucu, `TR_assert`'in koşulu (12.3), `TR_and`, `TR_or`, `TR_not`'un işlenenleri. Başka tip denetim hatasıdır; boşluk `.TR_length == 0`, yokluk `TR_none`'la karşılaştırma ile sorulur.
- *Test:* TR testi; hata ayıklayıcının koşulu uygulanınca Go testi; mantık dışı koşulun hatası uygulanınca denetleyicinin testi.

**3.11** `TR_Type(x)` değerin tipini verir; tip değerlerinin tipi `TR_Type`'tır. Tip değerleri: dilin tipleri (`TR_Int`, `TR_Float`, `TR_String`, `TR_Bytes`, `TR_Bool`, `TR_NoneType`, `TR_JSON`, `TR_Range`, `TR_Tensor`, `TR_Type`), parametreli tipler (`TR_List[TR_Int]`, `TR_Task[TR_Int]` (10.1), `TR_Int?`), demet tipleri (`(TR_Int, TR_String)`), fonksiyon tipleri (3.9), yapılar ve kitaplığın tipleri (`TR_Optimizer`, `TR_Tokenizer`). Tip, tiple karşılaştırılır (`TR_Type(x) == TR_Int`); metinle karşılaştırma denetim hatasıdır. `TR_JSON` değerinde `TR_Type` JSON türünü verir (3.7a); bu yüzden `TR_Type(j) == TR_JSON` hiç doğru olmaz ve denetim hatasıdır. İsteğe bağlı değerde `TR_Type` içteki değerin tipini, yoklukta `TR_NoneType`'ı verir. `TR_print`, `TR_String(x)` ve `TR_Type(x)` her tipten değer alan yerleşiklerdir; kullanıcının yazabileceği genel bir tip yoktur (3.7a). Matris iki boyutlu tensördür.
- *Test:* TR testi.

**3.12** Tip çağrılınca değerini kurar ya da verileni o tipe çevirir; aynı işi gören ayrı fonksiyon yoktur.
- `TR_Int(x)` ondalığı sıfıra doğru keser; metni 2.4'ün ondalık yazımıyla okur (`_` dahil, önde isteğe bağlı `+` ya da `-`, boşluksuz).
- `TR_Float(x)` tam sayıyı ve metni ondalığa çevirir; metni 2.4'ün ondalık ya da 2.5'in yazımıyla, önde isteğe bağlı `+` ya da `-` ile (`"3"`, `"-1.5"`), bir de "inf", "-inf" ve "nan" olarak okur.
- `TR_Bool(m)` yalnız "true" ve "false" metnini okur; sayıdan ve listeden mantık değeri kurulmaz. Tersi de yoktur: `TR_Int` ve `TR_Float` mantık değerinden sayı kurmaz.
- `TR_List(x)` aralığın ve listenin öğelerini yeni bir listeye koyar; metinden liste kurulmaz.
- `TR_String(x)` değerin metnini verir (3.14). Kitaplığın tipleri de böyle kurulur (`TR_Tensor([1, 2])`, `TR_Checkpoint("kayit")`); tensörde cins dönüşümü de tip çağrısıdır: `TR_Tensor[TR_Int](t)` (15.3).
- İki istisna: `TR_Type(x)` değerin tipini verir (3.11); `TR_JSON` değerinde tip çağrısı değeri açar (3.7a).
- Çevrilemeyen değer (yazıma uymayan metin, `TR_nan`, sonsuz, `TR_Int`'e sığmayan ondalık) çalışma hatasıdır; yerel ayar yoktur (`TR_Float("1,5")` hatadır). Atamada ve argümanda kendiliğinden dönüşüm yoktur; tek istisna 3.13'ün tam sayı yazımıdır.
- *Test:* TR testi.

```tr
# TR 4
TR_print(TR_Int("42") + 1, TR_Int(3.9), TR_Int(-3.9), TR_String(12) + "a")   # çıktı: 43 3 -3 12a
```

**3.13** Tip denetimi çalışmadan önce bütün programa iner (1.3): her ifadenin, atama hedefinin, varsayılanın, alanın ve metot gövdesinin tipi bilinir. Fonksiyon parametrelerinin ve dönüşünün tipini yazar (1.2); yerel değişkenin tipi ilk değerinden çıkar ve değişmez. Tip açıklaması adın ardından iki nokta ve tiple yazılır (`n: TR_Int`); yerel değişkende isteğe bağlıdır, ilk değeri `TR_none` ya da boş değişmez olan değişkende gerekir. Tam sayı yazımı (2.4) `TR_Float` beklenen yerde ondalıktır (`r: TR_Float = 1` → 1.0); `TR_Int` tipli bir değer `TR_Float` yerine verilmez; işleç (4.2) ve sayı kısıtlı yerleşik (13.10) bunun dışındadır; tensörün öğe cinsinin hangi sırayla çıktığı 15.2'dedir. Tip uyuşmazlığı denetim hatasıdır (çıkış kodu 1); ileti doğru yazımı ya da dönüşümü önerir (`0.0`, `TR_Float(n)`).
- *Test:* TR testi; kalıcı tip ve tam sayı yazımı uygulanınca denetleyicinin testleri.

```tr hata
# TR 4
TR_var toplam = 0
toplam += 0.5        # denetim hatası: toplam bir TR_Int; ilk değeri 0.0 yazın
```

**3.14** Değerin metni dilden bağımsızdır: `TR_String(x)`, `TR_print` ve yerleştirme (2.7) her makinede ve her okuyan için aynı metni verir.
- Tam sayı ondalık rakamla, ondalık 3.2'deki gibi yazılır. Dilin değerleri ve tipleri kimliğin adıyla yazılır: "true", "false", "none", "inf", "-inf", "nan"; tip adıyla ("Int", "List[Int]", "Int?").
- Metin kendisi olarak yazılır. Liste "[1, 2]", sözlük "{"k": 1}" biçimindedir; içlerindeki metin çift tırnakla ve kaçışla yazılır: `\n`, `\t`, `\r`, `\"`, `\\`, öbür denetim karakterleri ve 2.1a'nın yön karakterleri `\u{...}` ile (2.6). Kaptaki `TR_JSON` değeri JSON metniyle yazılır. Bayt dizisinin metni onu kuran çağrıdır: `Bytes([104, 105])`; 64 bayttan uzunsa boyu ve ilk 16 baytı.
- İsteğe bağlı değer içindeki değerin ya da "none"ın metnini, `TR_JSON` JSON metin türünde metnin kendisini, öbür türlerde JSON metnini (3.7a), aralık "0..3" ya da "0..=3" metnini verir.
- Fonksiyonun metni 6.9'da, yapı nesnesininki 7.6'da, tensörünki 15.5'tedir; bayt dizisininki yukarıda.
- Program çıktısı çevrilmez; kişinin dilinde görünen yalnız iletiler, kabuk ve hata ayıklayıcıdır.
- *Test:* TR testi.

```tr
# TR 4
TR_print(TR_true, TR_none, 1e16, TR_Type([1]), ["a"], {"k": 1})
# çıktı: true none 1e+16 List[Int] ["a"] {"k": 1}
```
