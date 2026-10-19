# 9. ertele

**9.1** `ertele çağrı` ya da `ertele { blok }`, içinde bulunduğu fonksiyon bittiğinde çalışacak işi kaydeder; birden çok ertele ters sırayla (son kaydedilen ilk) çalışır.

**9.2** Ertelenen iş fonksiyon `sonuç` ile erken bittiğinde ve hata ile çıkıldığında da çalışır; program Ctrl+C ile nazikçe durdurulduğunda da (bkz. 1).

**9.3** Bir blok (ör. `eğer` gövdesi) içindeki `ertele` bloğun değil, onu içeren fonksiyonun (en üst düzeyde programın, `eşzamanlı` içinde görevin, testte testin) sonunu bekler.

**9.4** Ertelenen işteki hata yutulmaz. Fonksiyon başarıyla bittiyse ertelenen işin hatası fonksiyonun hatası olur (`dene/yakala` ile yakalanır); fonksiyon zaten hata verdiyse o ilk hata korunur ve ertelenen işlerin hataları çalışma sırasıyla ona eklenir: `gövde hatası; ertele içinde de hata: ...`. Bir ertelenen iş hata verse de sonraki ertelenen işler çalışır.

**9.5** `ertele f(x)` çağrının tamamını (fonksiyonu ve argümanlarını) fonksiyon biterken hesaplar: `x`'in o anki değeri kullanılır, `ertele` satırındaki değil. Kayıt anındaki değer gerekiyorsa önce ayrı bir değişkene alınır. `eşzamanlı f(x)` de çağrıyı görevin içinde, görev çalışmaya başladığında hesaplar; değişkenler başlatıldığı kapsamdan okunur (10.4: döngüde her tur kendi kapsamındadır).
