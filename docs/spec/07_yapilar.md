# 7. Yapılar

**7.1** `yapı Ad { alanlar ve metotlar }` bir yapı tanımlar; alanlar `ad`, `ad: TR_type_Type` ya da `ad = varsayılan` biçimindedir ve virgül/satır sonuyla ayrılır. Gövdede yalnızca alanlar ve `TR_kw_func` tanımları bulunabilir; başka bir deyim ya da ifade (`TR_fn_print(...)`, `5 + 3`) ve aynı adın iki kez tanımlanması (iki alan, iki metot ya da aynı adlı alan ve metot) sözdizimi hatasıdır.

```tr
//! tr kodlu 3 dil=tr
TR_kw_struct Nokta {
    x: TR_type_Int = 0
    y = 0
    TR_kw_func uzaklık_kare() { TR_kw_return TR_kw_self.x ** 2 + TR_kw_self.y ** 2 }
}
```

**7.2** Yapı adı çağrılarak nesne oluşturulur; argümanlar alan sırasıyla ya da adla verilir, verilmeyen alan varsayılanını (yoksa `TR_kw_none`) alır: `Nokta(3, 4)`, `Nokta(y: 4)`. Alan sayısından fazla konumlu argüman, alan olmayan bir adla verilen argüman (`'Nokta' yapısının 'yy' adında bir alanı yok. Bunu mu demek istediniz: 'y'?`) ve konumlu argümanın doldurduğu bir alanın adıyla da verilmesi (`Nokta(1, x: 2)` → `'Nokta' yapısının 'x' alanı hem sırayla (1. argüman) hem adıyla verildi`) hatadır; yapının `TR_ctx_init`su varsa argümanlar kurucunun imzasına göre denetlenir (6.3). Varsayılanlar her nesne oluşturulurken, yapının tanımlandığı kapsamda hesaplanır (modüldeki bir yapı modülün adlarını, fonksiyon içindeki bir yapı o fonksiyonun yerel adlarını görür).

**7.3** Metotlar `TR_kw_self` ile nesnenin alanlarına erişir ve onları değiştirebilir: `p.uzaklık_kare()`.

**7.4** `TR_ctx_init` adlı metot, nesne oluşturulurken aynı argümanlarla çağrılır.

**7.5** Alanlar `p.x = 7` ile atanabilir; olmayan alana erişmek hatadır (`p?.z` ise `TR_kw_none` verir). Tanımda olmayan bir alana atama da hatadır (`Nokta yapısında 'z' alanı yok; yapı tanımına 'z' alanını ekleyin`), bir metoda değer atanamaz; böylece bir nesnenin alanları her zaman yapı tanımındakilerdir ve yazdırma, eşitlik ve gezinme (15.33) aynı alanları görür.

**7.6** Nesneler içerikleriyle karşılaştırılır (`Nokta(1, 2) == Nokta(1, 2)`) ve `Nokta{x: 1, y: 2}` biçiminde yazdırılır; `TR_fn_type()` yapının kendisini verir (`TR_fn_type(p) == Nokta`, 3.11).
