# 9. Ertele: emsal ve gerekçe

Kurallar: [`09_ertele.md`](../09_ertele.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 9.1

- *Emsal:* Swift'in `defer`'i kapsama bağlıdır: "A `defer` statement is used for executing code just before transferring program control outside of the scope that the `defer` statement appears in"; yazımı yalnız bloktur (`defer { ... }`). *Almadık:* Go'nun fonksiyona bağlı `defer`'i: "invokes a function whose execution is deferred to the moment the surrounding function returns". Döngüde açılan her kaynak fonksiyon bitene kadar açık kalır; bugünkü TR örneği (`examples/tr/16_ertele.tr`) bunu aşmak için her turu ayrı bir fonksiyona koyuyor. Go'nun çağrı yazımı (`defer f(x)`): blokla iki yazım olurdu. ([Swift 6.4 başvuru](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/statements/#Defer-Statement), [Go 1.27](https://go.dev/ref/spec#Defer_statements))
- *Gerekçe:* eğitim döngüsü her turda dosya, bağlantı ya da kamera karesi açar; robotun döngüsü saatlerce döner. Fonksiyona bağlı ertele kaynakları tur tur biriktirir ve bu sessizce olur, ancak kaynak tükenince görünür. Kapsama bağlı ertele kaynağı açıldığı turda kapatır. Tek yazımda ajan hangisini yazacağını seçmez; çağrı yazımı bir kez yazılırsa hata doğru yazımı verir.

### 9.2

- *Emsal:* Swift: "The statements within the `defer` statement are executed no matter how program control is transferred". Go'da da `return`, gövdenin sonu ve panik üç çıkıştır. ([Swift 6.4 başvuru](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/statements/#Defer-Statement), [Go 1.27](https://go.dev/ref/spec#Defer_statements))
- *Gerekçe:* temizlik çıkış yolundan bağımsız olmalı; durdurma (1.5) ve `TR_exit` (1.13) 1. bölümün kurallarıyla aynıdır.

### 9.3

- *Emsal:* Swift'te blok çalıştığı anda okur ve bloktan çıkılamaz: "The statements in the `defer` statement can't transfer program control outside of the `defer` statement". *Almadık:* Go'nun kayıt anında argüman hesaplaması: "the function value and parameters to the call are evaluated as usual and saved anew but the actual function is not invoked". Çağrı yazımı kalkınca bu ayrım da kalkar. Go'da ertelenen fonksiyon adlı sonuçları değiştirebilir; TR'de ertelenen iş sonucu değiştirmez. ([Swift 6.4 başvuru](https://docs.swift.org/swift-book/documentation/the-swift-programming-language/statements/#Defer-Statement), [Go 1.27](https://go.dev/ref/spec#Defer_statements))
- *Gerekçe:* bugünkü TR'de ertelenen bloktaki `TR_return 5` sessizce yok sayılıyor ve fonksiyon 1 döndürüyor [ölçüldü: main `7bda6fab`]: yazan bir şey bekliyor, program başka şey yapıyor. Sözdizimi hatası bunu çalışmadan önce gösterir. Bloğun her zaman çalıştığı andaki değeri okuması tek kuraldır; kayıt anındaki değer açıkça bir sabite alınır.

### 9.4

- *Emsal:* Java'nın kaynaklı `try`'ında ilk hata korunur, kapatmanın hatası ona bastırılmış olarak eklenir: "An exception from the closing of one resource does not prevent the closing of other resources. Such an exception is suppressed if an exception was thrown previously by an initializer, the try block, or the closing of a resource". *Almadık:* Python'un `finally`'sinde yeni hata öncekinin yerine geçer (önceki yalnız bağlam olarak kalır); bugünkü TR ise iki hatayı tek bir metinde birleştiriyor ("govde hatasi; error in defer as well: ertele hatasi") [ölçüldü: main `7bda6fab`]. ([Java SE 21, 14.20.3](https://docs.oracle.com/javase/specs/jls/se21/html/jls-14.html#jls-14.20.3))
- *Gerekçe:* asıl hata, programın neden durduğunu söyleyendir; temizlikteki hata onun sonucudur ve yerini almamalı. Hata 8.3'teki gibi koduyla ve değeriyle ayrılır, metinle değil. Bu yüzden ikinci hatalar metne katılmaz, liste olarak taşınır.
