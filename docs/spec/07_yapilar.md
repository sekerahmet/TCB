# 7. Yapılar

**7.1** `yapı Ad { alanlar ve metotlar }` bir yapı tanımlar; alanlar `ad`, `ad: Tip` ya da `ad = varsayılan` biçimindedir ve virgül/satır sonuyla ayrılır. Gövdede yalnızca alanlar ve `fonksiyon` tanımları bulunabilir; başka bir deyim ya da ifade (`yazdır(...)`, `5 + 3`) ve aynı adın iki kez tanımlanması (iki alan, iki metot ya da aynı adlı alan ve metot) sözdizimi hatasıdır.

```tr
yapı Nokta {
    x: Tam = 0
    y = 0
    fonksiyon uzaklık_kare() { sonuç kendi.x ** 2 + kendi.y ** 2 }
}
```

**7.2** Yapı adı çağrılarak nesne oluşturulur; argümanlar alan sırasıyla ya da adla verilir, verilmeyen alan varsayılanını (yoksa `yok`) alır: `Nokta(3, 4)`, `Nokta(y: 4)`. Alan sayısından fazla konumlu argüman ya da alan olmayan bir adla verilen argüman hatadır (`'Nokta' yapısının 'yy' adında bir alanı yok. Bunu mu demek istediniz: 'y'?`); yapının `kurucu`su varsa argümanlar kurucunun imzasına göre denetlenir (6.3). Varsayılanlar her nesne oluşturulurken, yapının tanımlandığı kapsamda hesaplanır (modüldeki bir yapı modülün adlarını, fonksiyon içindeki bir yapı o fonksiyonun yerel adlarını görür).

**7.3** Metotlar `kendi` ile nesnenin alanlarına erişir ve onları değiştirebilir: `p.uzaklık_kare()`.

**7.4** `kurucu` adlı metot, nesne oluşturulurken aynı argümanlarla çağrılır.

**7.5** Alanlar `p.x = 7` ile atanabilir; olmayan alana erişmek hatadır (`p?.z` ise `yok` verir). Tanımda olmayan bir alana atama da hatadır (`Nokta yapısında 'z' alanı yok; yapı tanımına 'z' alanını ekleyin`), bir metoda değer atanamaz; böylece bir nesnenin alanları her zaman yapı tanımındakilerdir ve yazdırma, eşitlik ve gezinme (15.19) aynı alanları görür.

**7.6** Nesneler içerikleriyle karşılaştırılır (`Nokta(1, 2) == Nokta(1, 2)`) ve `Nokta{x: 1, y: 2}` biçiminde yazdırılır; `tip()` yapının kendisini verir (`tip(p) == Nokta`, 3.11).
