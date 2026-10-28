# 7. Yapılar

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "7. Yapılar".

Her kuralın emsali ve gerekçesi: [`rationale/07_structs.md`](rationale/07_structs.md).

**7.1** `TR_struct Ad { ... }` bir yapı tanımlar; gövdede yalnız alanlar ve metotlar (7.3) bulunur.
- Alan `ad: T` ya da `ad: T = varsayılan` diye yazılır; her alanın tipi yazılır. Varsayılanlı bir alandan sonraki alanların da varsayılanı vardır (6.4).
- Aynı adlı iki alan, iki metot ya da aynı adlı alan ve metot denetim hatasıdır.
- Yapı 6.10'daki gibi tip parametresi alabilir: `TR_struct Kutu[T] { değer: T }`.
- Kalıtım yoktur. Başka dillerin yazımları sözdizimi hatasıdır ve ileti TR'deki yazımı söyler: Python'un `class A(B):`'si, `TR_struct A(B)`. Ortak davranış arayüzle (7.8), ortak parça alanla kurulur.
- *Test:* TR testi; tipli alan, alan sırası ve iletiler uygulanınca TR testi.

```tr
# TR 4
TR_struct Nokta {
    x: TR_Float = 0.0
    y: TR_Float = 0.0
    TR_func uzaklık() -> TR_Float {
        TR_return (TR_self.x ** 2 + TR_self.y ** 2) ** 0.5
    }
}
TR_var p = Nokta(3.0, 4.0)
TR_print(p.uzaklık(), p)   # çıktı: 5.0 Nokta(x: 3.0, y: 4.0)
```

**7.2** Yapının adı çağrılınca nesne kurulur: alanlar tanım sırasıyla ya da adla verilir (`Nokta(3.0, 4.0)`, `Nokta(y: 4.0)`); argüman kuralları 6.5'tekidir. Varsayılanı olmayan alan verilmezse denetim hatasıdır. Varsayılan her kurmada yeniden, yapının tanımlandığı kapsamda hesaplanır (6.4). Yapının kurucusu varsa kurma onun imzasıyla yapılır (7.4).
- *Test:* TR testi; zorunlu alanın denetimi uygulanınca denetleyicinin testi.

**7.3** Metot gövdedeki `TR_func`'tır ve `p.m(...)` ile çağrılır. Metot nesneye `TR_self` ile erişir: alan her zaman `TR_self.x` diye okunur ve atanır, başka metot `TR_self.m()` diye çağrılır. Metotta düz `x` ya da `m()` alan ya da metot değildir. Düz ad yapının bir alanının ya da metodunun adıysa ve dıştaki bir ada çözülüyorsa denetim hatasıdır; hiçbir ada çözülmüyorsa 5.14'ün hatasıdır. İkisinde de ileti `TR_self.x`'i önerir. `TR_self` parametre olarak yazılmaz; Python'un self parametresi sözdizimi hatasıdır. Çağrısız `p.m` `p`'ye bağlı bir fonksiyon değeridir (6.9). Nesnesiz metot (yapının adıyla, nesne olmadan çağrılan) yoktur; yapıya bağlı olmayan iş dosyanın fonksiyonu olarak yazılır. Metot `TR_self` kullanmayabilir: alansız bir yapı (`TR_struct Relu { ... }`) bir arayüzü (7.8) böyle karşılar.
- *Test:* TR testi; iletiler uygulanınca TR testi.

**7.4** Kurucu `TR_init` adlı metottur. Yapının kurucusu varsa nesne yalnız onun parametreleriyle kurulur (`Kat(512, 1024)`); alanlarla kurma kalkar. Kurucu değer döndürmez. Varsayılanlı alanlar kurucunun gövdesinden önce, tanım sırasıyla değerini alır; kurucu onları yeniden atayabilir. Kurucunun sonunda, her yolda, varsayılanı olmayan her alana atanmış olmalıdır; atanmadan okunan alan ve bütün alanlar atanmadan metot çağrısı denetim hatasıdır.
- *Test:* TR testi; alanların atanma denetimi uygulanınca denetleyicinin testi.

**7.5** Alan `p.x` ile okunur, `p.x = ifade` ile atanır; atanan değerin tipi alanın tipidir (3.13). Olmayan alana erişmek ve metoda değer atamak denetim hatasıdır; ileti en yakın alanı önerir (2.3a'nın uzaklığıyla), yakın alan yoksa alanı yapıya `k: T` diye eklemeyi. Bağı sabit bir nesnenin (`TR_const p`) alanları atanabilir (5.2).
- *Test:* TR testi; alan tipinin ve olmayan alanın denetimi uygulanınca denetleyicinin testi.

**7.6** Yapı nesnesi listeler gibi başvuruyla paylaşılır (3.6): `TR_var q = p` aynı nesnedir, `q.x = 9` `p.x`'i de değiştirir; bu yüzden 4.10'un yineleme kuralına girer.
- `==` aynı yapıdan iki nesneyi alanlarıyla, tanım sırasıyla karşılaştırır (4.5). Alanlarından biri karşılaştırılamayan yapı karşılaştırılamaz, denetim hatasıdır: fonksiyon alanı ve tensör alanı (tensörün `==`'si öğe öğedir, sonraki bölümlerde) böyledir.
- Metni kurma yazımıdır: "Nokta(x: 3.0, y: 4.0)"; alanlar 3.14'teki gibi yazılır.
- Nesne kendine bağlanabilir (ağaçta ebeveyn, çift yönlü bağlantı). Metin, yazılmakta olan bir nesneye yeniden varınca orada "..." yazar. `==` karşılaştırılmakta olan bir nesne çiftine yeniden varırsa çalışma hatasıdır.
- `TR_Type(p)` yapının kendisidir (`TR_Type(p) == Nokta`); yapının metni adıdır.
- *Test:* TR testi; metin uygulanınca TR testi.

**7.7** `TR_enum Ad { ... }` durumlu bir tip tanımlar: değer, sayılı durumlardan biridir; durum alan taşıyabilir.
- Durum `BüyükDeve` bir addır (2.3b), alanları parantez içinde 7.1'deki gibi yazılır: `Bekliyor`, `Çalışıyor(adım: TR_Int)`. Durumlardan sonra metotlar yazılabilir (7.3). Durumlu tip 6.10'daki gibi tip parametresi alabilir: `TR_enum Cevap[T] { ... }`, durumları `Tamam(v: T)` ve `Hata(m: TR_String)`.
- Değer `Durum.Bekliyor` ya da `Durum.Çalışıyor(5)` ile kurulur (argümanlar 6.5'teki gibi). Alanlı durum çağrısız (`Durum.Çalışıyor`), alansız durum çağrıyla (`Durum.Bekliyor()`) yazılamaz: çalışma hatasıdır, ileti doğru yazımı verir. Durum bir fonksiyon değeri değildir (tip çağrısı gibi, 13.6); fonksiyon gerekirse ok yazılır: `(x) => Durum.Çalışıyor(x)`. Durumlu tipin kurucusu (`TR_init`) yoktur, sözdizimi hatasıdır: değer durumun kendi alanlarıyla kurulur. Değer değiştirilemez; alanları yalnız eşlemeyle okunur. Alanlarından biri başvuruyla paylaşılan tipteyse değer 4.10'un yineleme kuralına girer.
- `TR_match`'te desen: `Durum.Bekliyor`; ya da `Durum.Çalışıyor(a)`, burada `a` alanın değerini bağlar ve yalnız o kolda görünür. Desen konumludur: her alana bir ad ya da `_`. Ad sayısı alan sayısını tutmazsa eşleme o kolu denerken çalışma hatasıdır, kol eşleşmez sayılmaz. Desende adlı alan (`Durum.Çalışıyor(adım: a)`) sözdizimi hatasıdır, ileti konumlu yazımı verir. Bağlanan ad dıştaki bir adla aynıysa 5.12'nin hatasıdır. `_` bir alanı atlar. Alanın içinde değer karşılaştırılmaz ve koşullu kol (`... TR_if koşul =>`) yoktur, sözdizimi hatasıdır; karşılaştırma kolun içinde yapılır.
- Eşleme her durumu ya da `_`'yi kapsar (5.6).
- `==` aynı durumu ve eşit alanları sınar. Metni kurma yazımıdır: "Durum.Çalışıyor(adım: 5)".
- Yapının deseni yoktur; yapıyla karşılaştırma parantezli ifadeyle yapılır (5.6).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

```tr
# TR 4
TR_enum Durum {
    Bekliyor
    Çalışıyor(adım: TR_Int)
    Bitti(kayıp: TR_Float)
}
TR_func anlat(d: Durum) -> TR_String {
    TR_return TR_match d {
        Durum.Bekliyor => "bekliyor"
        Durum.Çalışıyor(a) => "adım \(a)"
        Durum.Bitti(k) => "bitti, kayıp \(k)"
    }
}
TR_print(anlat(Durum.Çalışıyor(5)))   # çıktı: adım 5
```

**7.8** `TR_interface Ad { ... }` bir arayüz tanımlar: gövdesiz metot imzalarından oluşur (`TR_func f(x: TR_Tensor) -> TR_Tensor`). En az bir metot yazılır; boş arayüz denetim hatasıdır, çünkü her tip onu karşılar ve genel bir tip olurdu (3.11).
- Bir yapı ya da durumlu tip arayüzün bütün metotlarını aynı imzayla taşıyorsa onu karşılar; ayrıca bildirilmez. İmza parametre tipleri ve dönüş tipidir; parametre adları sayılmaz, arayüz üzerinden çağrı sırayla yapılır (6.9).
- Arayüz bir tiptir: parametrede, alanda ve listede kullanılır (`TR_List[Katman]`). Çağrı nesnenin kendi metoduna gider. Arayüz değeri karşılaştırılamaz (4.5); metni asıl değerin metnidir.
- Asıl tipe `TR_Type` sınamasıyla inilir: `TR_if TR_Type(k) == Kat { ... }` dalında `k` `Kat` tipindedir, 3.5'in daraltma koşullarıyla (yerel, dalda atanmayan, onu değiştiren kapanış olmayan değişken).
- Arayüzü karşılamayan değer verilirse denetim hatasıdır; ileti eksik ya da imzası tutmayan metotları söyler.
- Arayüz tip parametresine kısıt olur: `TR_func f[T: Katman](k: T)`; kısıtlı `T`'nin değerinde arayüzün metotları çağrılır (6.10).
- Yerleşik tiplerin karşıladığı hazır arayüzler (eşitlik, sıralama, sayı) sonraki bölümlerdedir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.
