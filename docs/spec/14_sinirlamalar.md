# 14. Bilinen sınırlamalar

Bunlar bugünkü uygulamanın gerçek davranışlarıdır; ileride değişebilirler.

- **Metin karşılaştırması yok:** `"a" < "b"` hatadır; `<`, `>`, `<=`, `>=` yalnızca sayılarda çalışır. Zincirleme karşılaştırma (`1 < x < 3`) da yoktur.
- **Tam bölme operatörü yok:** `//` yorum başlatır; tam bölme için `tam(a / b)` kullanılır.
- **Testler arası yalıtım sığdır:** her test yeni bir kapsamda çalışır, ama üst düzey bir değişkene *atama* yapan ya da paylaşılan bir listeyi değiştiren test, sonraki testlerin gördüğü değeri değiştirir (üst düzey durum her testte yeniden kurulmaz).
- **Görev hataları testi düşürmez:** testin başlattığı `eşzamanlı` görevdeki hata test raporuna yazılır, test kalmaz.
- **`sabit` yalnızca ada bağlıdır:** ad yeniden atanamaz (denetleyici ve çalışma zamanı denetler), ama değer bir listeyse içeriği değişebilir; `aktar ... olarak m` ile alınan modül sözlüğünde sabitlik korunmaz (`m["AD"] = 1` çalışır).
- **Tip denetimi sığdır:** tipi çıkarılamayan her değer `Herhangi` sayılır; parametre tipleri yalnızca adıyla doğrudan çağrılan fonksiyonların konumlu argümanlarında denetlenir (adlı argümanlar, metotlar ve fonksiyon değerleri üzerinden yapılan çağrılar denetlenmez).
- **Sınır dışı okuma hata vermez:** `l[10]` ve `m[10]` `yok` döner; bu, hataların geç fark edilmesine yol açabilir.
- **Yapılarda kalıtım yok;** metotlar yapı tanımının içinde yazılmalıdır.
- **Tanımlar öne alınmaz:** fonksiyonlar ve yapılar, tanımlandıkları satır çalıştıktan sonra kullanılabilir.
- **Tensörde kalan işleci yoktur:** `%` tensörlerde tanımlı değildir.
- **Kodlu saklama yalnızca Türkçe yazımdan kodlar:** `tr kodla` Türkçe yazılmış dosyayı kodlar; gösterim (`tr göster --dil`) ve çözme bugün yalnızca Türkçedir (başka yazım dilleri sonra gelecek). Kapsamı çalışma anında belli olan adlar yaklaşık kodlanır: takma adsız `aktar`ın altında yerleşik adı yerleşik sayılır (modül aynı adı tanımlıyorsa yanlış kodlanır), bir modül aktaranın genel adlarını bilmeden kodlanır, bir yerleşiği taşıyan değişkene verilen adlı argüman (`değişken t = toplam` ve `t(x, eksen: 0)`) kullanıcının adı sayılır. `$` kaçışı gerektiren bir ad (yalnızca başka dilde yazılmış koddan gelir) gösterimde sütunu bir kaydırır (17.5).
