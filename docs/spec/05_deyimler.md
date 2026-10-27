# 5. Deyimler ve kontrol yapıları

**5.1** `TR_kw_var ad = ifade` (ya da kısa `ad := ifade`) bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse `TR_kw_none` olur.

**5.2** `TR_kw_const AD = ifade` yeniden atanamayan bir ad tanımlar; ihlali tip denetleyicisi çalıştırmadan önce yakalar.

**5.3** Çoklu tanım ve atama virgülle yapılır; sağdaki tüm ifadeler atamadan önce hesaplanır (takas güvenlidir), sağda tek bir liste varsa elemanlarına açılır.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var a, b = 1, 2
a, b = b, a
TR_kw_var ad, soyad = ["Mustafa", "Kemal"]
```

**5.4** Atama `=` ile, bileşik atama `+= -= *= /=` ile yapılır; hedef bir ad, indeks (`l[0]`) ya da alan (`p.x`) olabilir. Tanımlanmamış bir ada atama hatadır (önce `TR_kw_var` ile tanımlanmalıdır); hata çalışmadan önce bulunur (5.14).

**5.5** `TR_kw_if koşul { ... } TR_kw_else TR_kw_if koşul { ... } TR_kw_else { ... }` bir ifadedir: değeri çalışan bloğun son deyimidir; hiçbir dal çalışmazsa `TR_kw_none`.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var n = -3
TR_kw_var tür = TR_kw_if n < 0 { "negatif" } TR_kw_else { "pozitif" }
```

**5.6** `eşle konu { desen => sonuç ... }` konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolu çalıştırır. Desen bir ifadedir ve değeri konuyla karşılaştırılır: ad (`sınır => ...`) ve parantezli ifade (`(sınır * 2) => ...`) de desendir, ok fonksiyonu sayılmaz (kolun sonucu ise bir ok fonksiyonu olabilir: `1 => (y) => y + 1`). Bir kolda virgülle birden çok desen olabilir; `TR_type_Range` deseni içerme sınar; `TR_kw_default` ya da `_` her şeyle eşleşir. Kol gövdesi bir ifade, `{ blok }` ya da `TR_kw_return ifade` olabilir; eşleşme yoksa `TR_kw_none`.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var puan = 95
TR_kw_match puan {
    100 => "tam puan"
    90..100 => "pekiyi"
    50, 60, 70 => "yuvarlak"
    TR_kw_default => "diğer"
}
```

**5.7** `TR_kw_for x TR_kw_in kaynak { ... }` listede, aralıkta, metinde (karakterler), sözlükte (anahtarlar) ve tensörde (ilk eksen boyunca, bkz. 15.5) dolaşır; akışlı kaynaklarda (veri kümesinin sıralı okuyucusu, 16.6) öğeler istendikçe okunur ve kaynak bitince döngü biter. İki değişkenli biçim `TR_kw_for i, x TR_kw_in ...` liste/metin/tensörde indeks ile elemanı, sözlükte anahtar ile değeri verir.

**5.8** `TR_kw_for koşul { ... }` ve `TR_kw_while koşul { ... }` koşul doğru oldukça tekrarlar.

**5.9** `TR_kw_for { ... }` sonsuz döngüdür; `TR_kw_break` en içteki döngüden çıkar.

**5.10** `TR_kw_continue` en içteki döngünün sonraki turuna geçer.

**5.11** Liste üreteci: `[ifade TR_kw_for x TR_kw_in kaynak TR_kw_if koşul]` (koşul isteğe bağlı).

```tr
//! tr kodlu 3 dil=tr
[x * x TR_kw_for x TR_kw_in 1..=5 TR_kw_if x % 2 == 1]   // [1, 9, 25]
```

**5.12** Her `{ ... }` bloğu (eğer/döngü gövdesi dahil) yeni bir kapsam açar; içeride tanımlanan ad dışarıda görünmez, dıştaki adlar okunup atanabilir. Döngünün her turu kendi kapsamında çalışır. İfade içindeki bloklar da (değer veren `TR_kw_if`, `TR_kw_match` kolları — tek satırlık `desen => ifade` kolu dahil — ve sondaki blok) kapsam açar: kolda ya da blokta tanımlanan adlı fonksiyon orada kalır, dıştaki aynı adı ezmez; koşul, `TR_kw_match` konusu ve desenler blok değildir, bulundukları kapsamda değerlendirilir.

**5.13** `TR_kw_break` ve `TR_kw_continue` yalnızca bir döngünün gövdesinde geçerlidir (araya `TR_kw_if`, `TR_kw_match`, `TR_kw_try` gibi bloklar girebilir); döngü dışında yazılmaları derleme hatasıdır ve program hiç çalışmaz. Fonksiyon, ok fonksiyonu, sondaki blok, `TR_ctx_test`, `TR_kw_defer` ve `TR_kw_async` gövdeleri sınırdır: içlerinden dıştaki döngüye `TR_kw_break`/`TR_kw_continue` uygulanamaz (`döngü { fonksiyon f() { çık } }` hatadır; fonksiyondan çıkmak için `TR_kw_return` yazılır).

**5.14** Tanımsız ad çalışmadan önce bulunur: okunan ya da atanan bir ad, bulunduğu yerden görünen hiçbir kapsamda (5.12) tanımlı değilse ve bir yerleşik değilse program (ya da modül) hiç çalışmaz. Hata tip hatalarıyla birlikte, satır sırasıyla bildirilir; yerleşik ad gölgeleme uyarıları (2.9a) da ayrı başlıkla aynı anda yazılır: `Satır 3, Sütun 1: Tanımsız ad — 'yazdir' adında bir değişken ya da fonksiyon yok ('yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?)`. Bugün dil dinamiktir (karar statik tiptir, 1.2); denetim yalnızca kesin durumları yakalar: bir kapsamda tanımlanan ad o kapsamın her yerinden görünür sayılır (fonksiyon kendisinden sonra tanımlanan bir fonksiyonu çağırabilir; tanımından önce okunan ad ise çalışma anında hata verir); takma adsız `TR_kw_import`ın bulunduğu kapsamda ve altında ad aranmaz (adlar çalışma anında gelir); modül, aktaranın genel kapsamındaki adları görür (11.1). Üye adları (`x.ad`), adlı argümanlar ve yapı alanları bu denetimin konusu değildir; parametreler, döngü ve üreteç değişkenleri, sondaki bloğun `TR_ctx_it`'su, `TR_kw_catch`'nın `TR_ctx_error`'sı ve yapı metotlarındaki `TR_kw_self` kendi kapsamlarında tanımlıdır.
