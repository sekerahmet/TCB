# 14. Bilinen sınırlamalar

Bunlar bugünkü uygulamanın gerçek davranışlarıdır; ileride değişebilirler.

- **Metin karşılaştırması yok:** `"a" < "b"` hatadır; `<`, `>`, `<=`, `>=` yalnızca sayılarda çalışır. Zincirleme karşılaştırma (`1 < x < 3`) da yoktur.
- **Tam bölme operatörü yok:** `//` yorum başlatır; tam bölme için `TR_fn_int(a / b)` kullanılır.
- **Testler arası yalıtım sığdır:** her test yeni bir kapsamda çalışır, ama üst düzey bir değişkene *atama* yapan ya da paylaşılan bir listeyi değiştiren test, sonraki testlerin gördüğü değeri değiştirir (üst düzey durum her testte yeniden kurulmaz).
- **`TR_kw_const` yalnızca ada bağlıdır:** ad yeniden atanamaz (denetleyici ve çalışma zamanı denetler), ama değer bir listeyse içeriği değişebilir; `aktar ... olarak m` ile alınan modül sözlüğünde sabitlik korunmaz (`m["AD"] = 1` çalışır).
- **Tip denetimi sığdır:** tipi çıkarılamayan her değer `TR_type_Any` sayılır; parametre tipleri yalnızca adıyla doğrudan çağrılan fonksiyonların konumlu argümanlarında denetlenir (adlı argümanlar, metotlar ve fonksiyon değerleri üzerinden yapılan çağrılar denetlenmez).
- **Yapılarda kalıtım yok;** metotlar yapı tanımının içinde yazılmalıdır.
- **Tanımlar öne alınmaz:** fonksiyonlar ve yapılar, tanımlandıkları satır çalıştıktan sonra kullanılabilir.
- **Tensörde kalan işleci yoktur:** `%` tensörlerde tanımlı değildir.
- **Kodlu saklamada kapsamı çalışma anında belli olan adlar yaklaşık kodlanır:** takma adsız `TR_kw_import`ın altında yerleşik adı yerleşik sayılır (modül aynı adı tanımlıyorsa yanlış kodlanır), bir modül aktaranın genel adlarını bilmeden kodlanır, bir yerleşiği taşıyan değişkene verilen adlı argüman (`TR_kw_var t = TR_fn_sum` ve `t(x, eksen: 0)`) kullanıcının adı sayılır. `$` kaçışı gerektiren bir ad (yalnızca başka dilde yazılmış koddan gelir) gösterimde sütunu bir kaydırır (17.5).
