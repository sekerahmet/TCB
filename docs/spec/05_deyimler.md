# 5. Deyimler ve kontrol yapıları

**5.1** `değişken ad = ifade` (ya da kısa `ad := ifade`) bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse `yok` olur.

**5.2** `sabit AD = ifade` yeniden atanamayan bir ad tanımlar; ihlali tip denetleyicisi çalıştırmadan önce yakalar.

**5.3** Çoklu tanım ve atama virgülle yapılır; sağdaki tüm ifadeler atamadan önce hesaplanır (takas güvenlidir), sağda tek bir liste varsa elemanlarına açılır.

```tr
değişken a, b = 1, 2
a, b = b, a
değişken ad, soyad = ["Mustafa", "Kemal"]
```

**5.4** Atama `=` ile, bileşik atama `+= -= *= /=` ile yapılır; hedef bir ad, indeks (`l[0]`) ya da alan (`p.x`) olabilir. Tanımlanmamış bir ada atama hatadır (önce `değişken` ile tanımlanmalıdır); hata çalışmadan önce bulunur (5.14).

**5.5** `eğer koşul { ... } değilse eğer koşul { ... } değilse { ... }` bir ifadedir: değeri çalışan bloğun son deyimidir; hiçbir dal çalışmazsa `yok`.

```tr
değişken n = -3
değişken tür = eğer n < 0 { "negatif" } değilse { "pozitif" }
```

**5.6** `eşle konu { desen => sonuç ... }` konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolu çalıştırır. Desen bir ifadedir ve değeri konuyla karşılaştırılır: ad (`sınır => ...`) ve parantezli ifade (`(sınır * 2) => ...`) de desendir, ok fonksiyonu sayılmaz (kolun sonucu ise bir ok fonksiyonu olabilir: `1 => (y) => y + 1`). Bir kolda virgülle birden çok desen olabilir; `Aralık` deseni içerme sınar; `varsayılan` ya da `_` her şeyle eşleşir. Kol gövdesi bir ifade, `{ blok }` ya da `sonuç ifade` olabilir; eşleşme yoksa `yok`.

```tr
değişken puan = 95
eşle puan {
    100 => "tam puan"
    90..100 => "pekiyi"
    50, 60, 70 => "yuvarlak"
    varsayılan => "diğer"
}
```

**5.7** `döngü x içinde kaynak { ... }` listede, aralıkta, metinde (karakterler), sözlükte (anahtarlar) ve tensörde (ilk eksen boyunca, bkz. 15.5) dolaşır; akışlı kaynaklarda (veri kümesinin sıralı okuyucusu, 16.6) öğeler istendikçe okunur ve kaynak bitince döngü biter. İki değişkenli biçim `döngü i, x içinde ...` liste/metin/tensörde indeks ile elemanı, sözlükte anahtar ile değeri verir.

**5.8** `döngü koşul { ... }` ve `iken koşul { ... }` koşul doğru oldukça tekrarlar.

**5.9** `döngü { ... }` sonsuz döngüdür; `çık` en içteki döngüden çıkar.

**5.10** `devam` en içteki döngünün sonraki turuna geçer.

**5.11** Liste üreteci: `[ifade döngü x içinde kaynak eğer koşul]` (koşul isteğe bağlı).

```tr
[x * x döngü x içinde 1..=5 eğer x % 2 == 1]   // [1, 9, 25]
```

**5.12** Her `{ ... }` bloğu (eğer/döngü gövdesi dahil) yeni bir kapsam açar; içeride tanımlanan ad dışarıda görünmez, dıştaki adlar okunup atanabilir. Döngünün her turu kendi kapsamında çalışır.

**5.13** `çık` ve `devam` yalnızca bir döngünün gövdesinde geçerlidir (araya `eğer`, `eşle`, `dene` gibi bloklar girebilir); döngü dışında yazılmaları derleme hatasıdır ve program hiç çalışmaz. Fonksiyon, ok fonksiyonu, sondaki blok, `test`, `ertele` ve `eşzamanlı` gövdeleri sınırdır: içlerinden dıştaki döngüye `çık`/`devam` uygulanamaz (`döngü { fonksiyon f() { çık } }` hatadır; fonksiyondan çıkmak için `sonuç` yazılır).

**5.14** Tanımsız ad çalışmadan önce bulunur: okunan ya da atanan bir ad, bulunduğu yerden görünen hiçbir kapsamda (5.12) tanımlı değilse ve bir yerleşik değilse program (ya da modül) hiç çalışmaz. Hata tip hatalarıyla birlikte, satır sırasıyla bildirilir; yerleşik ad gölgeleme uyarıları (2.9a) da ayrı başlıkla aynı anda yazılır: `Satır 3, Sütun 1: Tanımsız ad — 'yazdir' adında bir değişken ya da fonksiyon yok ('yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?)`. Dil dinamiktir; denetim yalnızca kesin durumları yakalar: bir kapsamda tanımlanan ad o kapsamın her yerinden görünür sayılır (fonksiyon kendisinden sonra tanımlanan bir fonksiyonu çağırabilir; tanımından önce okunan ad ise çalışma anında hata verir); takma adsız `aktar`ın bulunduğu kapsamda ve altında ad aranmaz (adlar çalışma anında gelir); modül, aktaranın genel kapsamındaki adları görür (11.1). Üye adları (`x.ad`), adlı argümanlar ve yapı alanları bu denetimin konusu değildir; parametreler, döngü ve üreteç değişkenleri, sondaki bloğun `o`'su, `yakala`'nın `hata`'sı ve yapı metotlarındaki `kendi` kendi kapsamlarında tanımlıdır.
