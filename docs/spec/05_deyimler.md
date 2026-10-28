# 5. Deyimler ve kontrol yapıları

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "5. Deyimler ve kontrol yapıları".

Her kuralın emsali ve gerekçesi: [`rationale/05_statements.md`](rationale/05_statements.md).

**5.1** `TR_var x = ifade` bulunulan kapsamda yeni bir değişken tanımlar; ilk değer her zaman yazılır, tip ondan çıkar (3.13). Kısa yazım (`x := ifade`) yoktur: sözdizimi hatasıdır, ileti `TR_var`'ı önerir. Fonksiyonun içinde hiç yeniden atanmayan yerel `TR_var` uyarı verir, ileti `TR_const`'u önerir; programın en dışındaki değişkenler bu uyarıya girmez.
- *Test:* TR testi; uyarı uygulanınca TR testi.

**5.2** `TR_const x = ifade` yeniden atanamayan bir ad tanımlar; ona atama denetim hatasıdır. Sabit olan bağdır, değer değil: `TR_const l = [1]` iken `l.TR_append(2)` geçerli, `l = []` hatadır. Tip açıklaması 3.13'teki gibi yazılır (`TR_const l: TR_List[TR_Int] = []`). Modülün en dışındaki sabitin ilk değeri sabit ifadedir (1.1).
- *Test:* TR testi.

**5.3** Çoklu tanım ve atama virgülle yapılır; sağdaki ifadelerin hepsi atamadan önce hesaplanır (`a, b = b, a` takas eder). Sağda tek bir liste varsa öğelerine açılır; öğe sayısı tutmazsa çalışma hatasıdır. Gerekmeyen değerin yerine `_` yazılır (`TR_var a, _ = b, c`); `_` bir ad değildir, okunamaz.
- *Test:* TR testi.

```tr
# TR 4
TR_var a, b = 1, 2
a, b = b, a
TR_const AD, SOYAD = ["Mustafa", "Kemal"]
```

**5.4** Atama `=` ile, bileşik atama `+=`, `-=`, `*=`, `/=`, `//=`, `%=`, `**=`, `&=`, `|=`, `^=`, `<<=`, `>>=` ile yapılır; `x op= y`, `x = x op y` demektir ve sonucun tipi `x`'in tipi olmalıdır (`TR_Int` bir değişkene `/=` denetim hatasıdır, ileti `//=`'yi önerir). Tek istisna `TR_List`'tir: `+=` ve `*=` listeyi yerinde değiştirir, listeyi paylaşan her ad değişikliği görür (`TR_var m = l` iken `l += [2]` `m`'yi de uzatır); `TR_const` listede de geçerlidir (5.2); `*=` 4.10'un yineleme kuralına uyar. Hedef bir ad, indeks (`l[0]`) ya da alan (`p.x`) olabilir ve bir kez hesaplanır (`l[f()] += 1`'de `f` bir kez çağrılır). Tanımlanmamış bir ada atama denetim hatasıdır; ileti `TR_var`'ı önerir.
- *Test:* TR testi; tip kuralı uygulanınca denetleyicinin testi.

**5.5** `TR_if koşul { ... } TR_else TR_if koşul { ... } TR_else { ... }`: koşul `TR_Bool`'dur (3.10), gövde her zaman süslü parantez içindedir. `TR_if` ifade olarak da kullanılır: değeri çalışan bloğun son ifadesidir; o zaman `TR_else` zorunludur ve bütün dalların tipi aynıdır. `TR_elif` yoktur; ileti `TR_else TR_if`'i önerir. Python'un koşullu ifadesi (`a TR_if c TR_else b`) sözdizimi hatasıdır; ileti `TR_if c { a } TR_else { b }`'yi önerir.
- *Test:* TR testi; dal tipi uygulanınca denetleyicinin testi.

```tr
# TR 4
TR_var n = -3
TR_var tur = TR_if n < 0 { "negatif" } TR_else { "sıfır ya da pozitif" }
```

**5.6** `TR_match x { ... }` x'i kollarla sırayla karşılaştırır, ilk eşleşen kolu çalıştırır; her kol bir desen, `=>` ve bir sonuçtur.
- Desen bir değerdir ve konuyla `==` ile karşılaştırılır: sayı ya da metin yazımı, sabitin (`TR_const`) adı, parantezli ifade. Ad deseni adı bağlamaz (tek istisna, durumlu tipin deseninde alanların adları, sonraki bölümlerde). Değişkenle karşılaştırmak için parantez yazılır (`(x)`); düz adla yazılan değişken denetim hatasıdır, çünkü başka dillerde bu yazım her şeyle eşleşip adı bağlar; ileti `(x)`'i ya da `_`'yi önerir.
- Bir kolda virgülle birden çok desen olabilir; konu `TR_Int` iken `TR_Range` deseni içermeyi sınar; `_` her şeyle eşleşir.
- Eşleme her durumu kapsar: `TR_Bool`'da iki değer, öbür tiplerde bir `_` kolu gerekir; kapsamayan eşleme denetim hatasıdır.
- Kol gövdesi bir ifade, `{ blok }` ya da `TR_return ifade` olabilir. `TR_match` ifade olarak kullanılınca bütün kolların tipi aynıdır.
- Durumlu tiplerin ve yapıların deseni sonraki bölümlerdedir.
- *Test:* TR testi; kapsama denetimi ve ad deseninin kuralı uygulanınca denetleyicinin testi.

```tr
# TR 4
TR_var puan = 95
TR_var not_ = TR_match puan {
    100 => "tam puan"
    90..100 => "pekiyi"
    50, 60, 70 => "yuvarlak"
    _ => "diğer"
}
```

**5.7** `TR_for x TR_in kaynak { ... }` dolaşır: listede öğeler, aralıkta sayılar, metinde karakterler, bayt dizisinde baytlar (`TR_Int`), sözlükte anahtarlar, `TR_JSON` dizisinde öğeler ve nesnesinde anahtarlar; akışlı kaynakta (veri okuyucu) öğeler istendikçe okunur, kaynak bitince döngü biter. Metin, sayı, mantık ve yokluk türündeki `TR_JSON` değeri dolaşılamaz (çalışma hatası). `TR_for i, x TR_in ...` listede, metinde ve bayt dizisinde indeksle öğeyi, sözlükte ve `TR_JSON` nesnesinde anahtarla değeri verir. Değer gerekmiyorsa değişkenin yerine `_` yazılır (`TR_for _ TR_in 0..3`); liste üretecinde de. Dolaşılan liste ya da sözlüğe öğe eklemek ya da ondan öğe çıkarmak çalışma hatasıdır; var olan öğeye ya da anahtara atama (`l[i] = ...`, `d[k] = ...`) serbesttir, dolaşma o anki değeri görür. Tensörde dolaşmak sonraki bölümlerdedir.
- *Test:* TR testi.

**5.8** `TR_for koşul { ... }` koşul doğru oldukça döner; koşul `TR_Bool`'dur. Ayrı bir `TR_while` yoktur (2.9).
- *Test:* TR testi; `TR_while`'ın kalkması uygulanınca TR testi.

**5.9** `TR_for { ... }` sonsuz döngüdür; `TR_break` en içteki döngüden çıkar.
- *Test:* TR testi.

**5.10** `TR_continue` en içteki döngünün sonraki turuna geçer.
- *Test:* TR testi.

**5.11** Liste üreteci `[ifade TR_for x TR_in kaynak TR_if koşul]` bir `TR_List` kurar; koşul isteğe bağlıdır. Değişken üretecin içinde kalır: dıştaki aynı adlı değişkeni gizler ve onu değiştirmez (5.12'nin istisnası).
- *Test:* TR testi.

```tr
# TR 4
TR_print([x * x TR_for x TR_in 1..=5 TR_if x % 2 == 1])   # çıktı: [1, 9, 25]
```

**5.12** Her `{ ... }` bloğu yeni bir kapsam açar: `TR_if` ve döngü gövdesi, `TR_match` kolu, sondaki blok. İçeride tanımlanan ad dışarıda görünmez; dıştaki adlar okunur ve atanır. Döngünün her turu kendi kapsamındadır. Bir yerel ad, aynı fonksiyonda kendisini kapsayan ya da aynı kapsamda tanımlı bir yerel adla yeniden tanımlanamaz (denetim hatası): iç bloktaki `TR_var t = 0` dıştaki `t`'yi gizlemez, hata verir. Bu kural için programın en dışı da bir fonksiyon gövdesidir; parametreler gövdenin en dış kapsamındadır, gövdede aynı adla tanım hatadır. Fonksiyon ve fonksiyon ifadesi yeni bir fonksiyondur: parametreleri ve yerelleri dıştaki adları gizleyebilir. Sondaki blok bu kural için sıradan bir bloktur. Koşul, `TR_match`'in konusu ve desenler blok değildir.
- *Test:* TR testi; yeniden tanımlama hatası uygulanınca denetleyicinin testi.

**5.13** `TR_break` ve `TR_continue` yalnız bir döngünün gövdesinde geçerlidir; araya `TR_if`, `TR_match`, `TR_try` blokları girebilir. Fonksiyon, fonksiyon ifadesi, sondaki blok, `TR_test`, `TR_defer` ve `TR_async` gövdeleri sınırdır: içlerinden dıştaki döngüye uygulanamaz. Aksi sözdizimi hatasıdır: dosya okunurken bulunur.
- *Test:* TR testi.

**5.14** Her ad çalışmadan önce çözülür (1.3): okunan ya da atanan ad görünen bir kapsamda tanımlı değilse denetim hatasıdır ve ileti görünen adlardan en yakınını önerir (2.3a'nın uzaklığıyla). Değişken tanımından önce kullanılamaz. Adlı fonksiyon ve tip tanımları (yapı ve sonraki bölümlerdeki öbür tipler) bulundukları bloğun (dosyanın en dışı dahil) her yerinden görünür, birbirini çağırabilir. Ama bir fonksiyonu çağıran ya da değer olarak alan ifade, o fonksiyonun doğrudan ya da çağırdığı fonksiyonlar yoluyla kullandığı değişkenlerin tanımından sonra gelmelidir; aksi denetim hatasıdır, ileti tanımı yukarı almayı önerir. Modülün en dışındaki tanımlar (1.1) sıradan bağımsızdır; sabitlerin ilk değerleri birbirine döngüyle bağlıysa denetim hatasıdır. Bir dosya yalnız kendi adlarını, yerleşikleri ve aktardığı adları görür; kendisini aktaranın adlarını görmez. Üye adları, adlı argümanlar ve alanlar tipten çözülür (3.13).
- *Test:* TR testi; yakın ad önerisi ve görünürlük kuralı uygulanınca denetleyicinin testi.
