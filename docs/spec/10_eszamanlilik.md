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
- **Uzun yerleşikler kilidi bırakır:** tensör işlemleri, ileri ve geri yayılım, iyileştiricinin adımı, örnekleme, toplu kodlama, veri okuma ve cihaz beklemeleri çalışırken öbür görevler TR kodu çalıştırabilir.
- **Her yerleşik bölünmez görünür:** bir yerleşiğin okuduğu tensöre o bitene kadar başka bir görev yazamaz; yazdığı tensörü başka bir görev okuyamaz da yazamaz da. Bekleyen görev kilidi bırakarak bekler. Sonuç, yerleşik başka hiçbir görev araya girmeden çalışmış gibidir; kilitlenme olmaz.
- Ağır hesap yapan yerleşikler ayrıca kendi içinde paralel çalışır.
- *Test:* TR testi; Go: `TestAsyncConcurrentDictAndListWrites`; süreyle devir ve kilidi bırakan uzun yerleşikler uygulanınca TR testi.

**10.3** Engelleyici işlemler kilidi bırakır: `TR_sleep`, beklemeler (`.TR_wait()`, `TR_wait_all`), girdi, dosya işleri, ağ istekleri, dış süreç ve sunucu beklemeleri. Bekleyen görevler böylece iç içe ilerler: dört görevde `TR_sleep(0.2)` toplam yaklaşık 0,2 saniye sürer.
- *Test:* TR testi.

**10.4** Görev başlatıldığı kapsamı yakalar; döngünün içinde başlatılan her görev o turun değişkenlerini görür (5.12). Görevin içinde tanımlanan ad görevin kapsamındadır.
- *Test:* TR testi.

**10.5** `TR_wait_all()` başlatılmış bütün görevlerin bitmesini bekler. `TR_wait_all(görevler)` verilen görevleri bekler ve sonuçlarını **verildikleri sırayla** bir liste olarak verir (`TR_List[T]`).
- Görevlerden biri hatayla bittiyse öbürleri de beklenir ve **verilen sırada ilk hatalı görevin** hatası fırlar; bitme zamanı hangi hatanın fırladığını değiştirmez. Öbür görevlerin hataları alınmış sayılır, 10.6'ya göre yazılmaz.
- *Test:* TR testi; liste biçimi uygulanınca TR testi.

**10.6** Görevin hatası kaybolmaz. Beklenen görevin hatası `.TR_wait()`'te ya da `TR_wait_all`'da fırlar. Hiç beklenmeyen görevin yakalanmamış hatası hata çıktısına (komut satırında standart hata, testte testin raporu, hata ayıklamada hata ayıklama konsolu) görevin başladığı satırla yazılır; program sürer ve sonunda hata koduyla biter (1.9).
- İptal edilen görevin (10.9) iptali bir hata olarak yazılmaz; iptalden önce görevin kendi hatası olduysa o yazılır.
- Testin başlattığı görevdeki yakalanmamış hata o testi kaldırır (12.5).
- *Test:* Go: `TestDAPTaskError`; `.TR_wait()`'te fırlaması uygulanınca TR testi.

**10.7** `g.TR_wait(TR_timeout: saniye)` en çok verilen süre kadar bekler. Görev o sürede biterse sonucunu verir ya da hatasını fırlatır; süre dolarsa **görev iptal edilir** (10.9) ve zaman aşımı hatası fırlar. Sabit hızlı bir döngü ve uzak bir modele sorulan soru böyle sınırlanır; program uzak cevabı beklemeye bağlı kalmaz.
- Görevi iptal etmeden sonucunu sormak için `g.TR_done()` kullanılır: görev bittiyse (sonuçla, hatayla ya da iptalle) `TR_true` verir, beklemez. Sabit hızlı bir döngü her turda böyle sorar, bitmemişse önceki sonucu kullanır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**10.8** Görevlerin birbirine göre sırası belirlenimci değildir: hangi görevin önce ilerlediği beklemelere ve zamana bağlıdır. Sonucu sıraya bağlı olmasın diye her görev sonucunu döndürür ve sonuçlar 10.5'teki gibi sırayla alınır. Paylaşılan bir listeye görevlerden eklemek bozulmaz (10.2) ama ekleme sırası tanımsızdır.
- *Test:* gerekmez: sıranın tanımsızlığı sınanmaz; doğru yazım (sırayla toplama) 10.5'in testidir.

**10.9** `g.TR_cancel()` görevi iptal eder: görev bir sonraki deyim sınırında ya da beklemesinde durur.
- O sırada çalışan bir yerleşik önce biter (10.2'nin bölünmezliği).
- Görevin ertelenen işleri (`TR_defer`, 9.1) çalışır; iptal görevin içinde `TR_catch` ile yakalanmaz, görev her zaman durur.
- İptal, görevin başlattığı ve henüz bitmemiş görevlere de geçer: iptal edilen görevden kimsenin beklemediği bir görev kalmaz.
- İptal edilen görevi bekleyen `.TR_wait()` iptal hatası fırlatır; `TR_wait_all` için iptal de bir hatadır (10.5).
- Bitmiş görevi iptal etmenin etkisi yoktur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.
