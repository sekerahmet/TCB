# 10. Eşzamanlılık ve yorumlayıcı kilidi

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "10. Eşzamanlılık".

Her kuralın emsali ve gerekçesi: [`rationale/10_concurrency.md`](rationale/10_concurrency.md).

Bir model hizmeti aynı anda birden çok isteğe cevap verir, bir ajan birden çok uzak modele birlikte sorar, bir robot algılayıcıyı beklerken başka işi sürdürür. Bu bölüm işi görevlere bölmeyi, sonuçlarını almayı, beklemeyi sınırlamayı ve görevler arasında veriyi güvenle paylaşmayı söyler.

**10.1** `TR_async f(x)` ve `TR_async { ... }` işi ayrı bir görev olarak başlatır ve hemen devam eder. İkisi de bir ifadedir ve görev değeri verir: `TR_var g = TR_async f(x)`.
- Görevin tipi `TR_Task[T]`'dir (3.11); `T` fonksiyonun dönüş tipidir, blokta `TR_NoneType`'tır.
- `g.TR_wait()` görev bitene kadar bekler ve sonucunu verir; görev hatayla bittiyse o hatayı burada fırlatır (8).
- Görev değeri kullanılmayabilir: `TR_async` ifadesi tek başına bir deyim olarak yazılabilir, 2.8'in değeri kullanılmayan ifade kuralı ona uygulanmaz. O zaman görev yine çalışır ve 10.6'nın kuralı geçerlidir.
- Program bitmeden önce bütün görevleri bekler.
- *Test:* TR testi; görev değeri ve `.TR_wait()` uygulanınca TR testi.

**10.2** Yorumlayıcı kilidi: aynı anda yalnız bir görev TR kodu çalıştırır. Paylaşılan liste ve sözlük bozulmaz, bir deyim (`s += 1`) bölünmez; veri yarışı yoktur.
- **Devir süreyledir:** kilidi bekleyen bir görev varsa kilidi tutan görev, kilidi aldığından bu yana 5 ms geçtikten sonraki ilk deyim sınırında onu devreder. Tur sayısı ya da çağrı sayısı devri belirlemez; 1.5'in durdurma noktası ayrı bir kuraldır.
- **Gecikme sözü:** kilidi bekleyen bir görev en çok 10 ms içinde kilidi alır; öbür görevler TR kodu, tensör ve veri yerleşikleri (tensör işlemleri, ileri ve geri yayılım, iyileştiricinin adımı, örnekleme, toplu kodlama, veri ve dosya okuma, cihaz beklemeleri) çalıştırıyor olabilir. Bu yerleşikler uzun sürünce çalışırken kilidi bırakır; hangisinin hangi büyüklükten sonra bıraktığı uygulamanın işidir.
- TR'nin kendi değerleri üstünde çalışan yerleşikler (listenin, sözlüğün ve metnin metotları, `TR_json_string`, `TR_json_parse`) kilidi tutar; süreleri büyüklükle artar. Büyük veri tensörde ve veri kümesinde tutulur (15, 16).
- **Her yerleşik bölünmez görünür:** bir yerleşiğin okuduğu tensöre o bitene kadar başka bir görev yazamaz; yazdığı tensörü başka bir görev okuyamaz da yazamaz da. Sonuç, yerleşik başka hiçbir görev araya girmeden çalışmış gibidir.
  - Okuma ve yazma izni tensörün **deposundadır**: aynı depoyu paylaşan görünümler (15) birbirini korur.
  - Yerleşik bütün izinlerini başlarken **hepsini birden** alır; biri alınamıyorsa hiçbirini tutmadan, kilidi bırakarak bekler. Böylece kilitlenme olmaz.
  - TR kodunu geri çağıran yerleşik (kullanıcının yazdığı türev, `TR_fuse`'un fonksiyonu, anahtarlı sıralama) geri çağırmadan önce izinlerini bırakır: bölünmezlik geri çağırmalar arasındaki parçalar içindir.
- Ağır hesap yapan yerleşikler ayrıca kendi içinde paralel çalışır.
- *Test:* TR testi; Go: `TestAsyncConcurrentDictAndListWrites`; gecikme sözü ve bölünmezlik uygulanınca Go testi (bekleyen görevin kilidi alma süresi ölçülür).

**10.3** Engelleyici işlemler kilidi bırakır: `TR_sleep`, beklemeler (`.TR_wait()`, `TR_wait_all`), girdi, dosya işleri, ağ istekleri, dış süreç ve sunucu beklemeleri. Bekleyen görevler böylece iç içe ilerler: dört görevde `TR_sleep(0.2)` toplam yaklaşık 0,2 saniye sürer.
- *Test:* TR testi.

**10.4** Görev başlatıldığı kapsamı yakalar; döngünün içinde başlatılan her görev o turun değişkenlerini görür (5.12). Görevin içinde tanımlanan ad görevin kapsamındadır. Görev açıldığı yerin türevsiz kipini ve `TR_mixed_precision` türünü de alır (15.4, 15.27); açıldıktan sonra açanın kipinin değişmesi ona geçmez.
- *Test:* TR testi; kipin ve hesap türünün mirası uygulanınca TR testi.

**10.5** `TR_wait_all()` başlatılmış bütün görevlerin bitmesini bekler. `TR_wait_all(görevler)` verilen görevleri bekler ve sonuçlarını **verildikleri sırayla** bir liste olarak verir (`TR_List[T]`).
- Görevlerden biri hatayla bittiyse öbürleri de beklenir ve **verilen sırada ilk hatalı görevin** hatası fırlar; bitme zamanı hangi hatanın fırladığını değiştirmez. Öbür görevlerin hataları alınmış sayılır, 10.6'ya göre yazılmaz.
- Argümansız `TR_wait_all()` sonuç vermez; iptal edilmiş bir görev onu hataya düşürmez (iptal hata değildir, 10.6), hatayla biten görevlerin hatası 10.6'ya göre yazılır.
- *Test:* TR testi; liste biçimi uygulanınca TR testi.

**10.6** Görevin hatası kaybolmaz. Beklenen görevin hatası `.TR_wait()`'te ya da `TR_wait_all`'da fırlar. Hiç beklenmeyen görevin yakalanmamış hatası hata çıktısına (komut satırında standart hata, testte testin raporu, hata ayıklamada hata ayıklama konsolu) görevin başladığı satırla yazılır; program sürer ve sonunda hata koduyla biter (1.9).
- İptal edilen görevin (10.9) iptali bir hata olarak yazılmaz; iptalden önce görevin kendi hatası olduysa o yazılır.
- Testin başlattığı görevdeki yakalanmamış hata o testi kaldırır (12.5).
- *Test:* Go: `TestDAPTaskError`; `.TR_wait()`'te fırlaması uygulanınca TR testi.

**10.7** `g.TR_wait(TR_timeout: saniye)` en çok verilen süre kadar bekler. Görev o sürede biterse sonucunu verir ya da hatasını fırlatır; süre dolarsa **görev iptal edilir** (10.9) ve zaman aşımı hatası fırlar. Sabit hızlı bir döngü ve uzak bir modele sorulan soru böyle sınırlanır; program uzak cevabı beklemeye bağlı kalmaz.
- İptal görevi onu bekleyen herkes için durdurur: aynı görevi başka bir yer de bekliyorsa o da iptal hatası alır.
- Zaman aşımı hatasının ve iptal hatasının kendi ileti kodları vardır (8.5); `TR_error_code { g.TR_wait(TR_timeout: 1) }` ile sınanır.
- Görevi iptal etmeden sonucunu sormak için `g.TR_done()` kullanılır: görev bittiyse (sonuçla, hatayla ya da iptalle) `TR_true` verir, beklemez. Sabit hızlı bir döngü her turda böyle sorar, bitmemişse önceki sonucu kullanır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**10.8** Görevlerin birbirine göre sırası belirlenimci değildir: hangi görevin önce ilerlediği beklemelere ve zamana bağlıdır. Sonucu sıraya bağlı olmasın diye her görev sonucunu döndürür ve sonuçlar 10.5'teki gibi sırayla alınır. Paylaşılan bir listeye görevlerden eklemek bozulmaz (10.2) ama ekleme sırası tanımsızdır.
- *Test:* gerekmez: sıranın tanımsızlığı sınanmaz; doğru yazım (sırayla toplama) 10.5'in testidir.

**10.9** `g.TR_cancel()` görevi iptal eder. İptal, 1.5'in durdurmasının tek bir göreve uygulanmasıdır: görev 1.5'in güvenli noktalarında durur (kullanıcı fonksiyonu çağrısı, döngüde en geç 256 turda bir, süren bir yerleşiğin içi).
- Programın durumunu değiştirmeyen yerleşik (hesap, okuma, bekleme) bırakılır, sonucu atılır; durum değiştiren yerleşik (indeksle atama, `TR_backward`, iyileştiricinin adımı, dosyaya yazma) biter. 1.5'in listesi geçerlidir; bölünmezlik (10.2) bozulmaz.
- Görevin ertelenen işleri (`TR_defer`, 9.1) çalışır; iptal görevin içinde `TR_catch` ile yakalanmaz, görev her zaman durur.
- İptal, görevin başlattığı ve henüz bitmemiş görevlere de geçer: iptal edilen görevden kimsenin beklemediği bir görev kalmaz. O alt görevin görev değerini başka bir görev bekliyorsa o da iptal hatası alır.
- İptal edilen görevi bekleyen `.TR_wait()` iptal hatası fırlatır (10.7'deki kodla); `TR_wait_all(görevler)` için iptal de bir hatadır (10.5).
- Bitmiş görevi iptal etmenin etkisi yoktur. `TR_handle_stop` (1.6) iptali etkilemez: o program durdurma isteğini üstlenir, iptal edilen görev yine durur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.
