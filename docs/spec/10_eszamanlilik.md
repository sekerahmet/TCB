# 10. Eşzamanlılık ve yorumlayıcı kilidi

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "10. Eşzamanlılık".

Her kuralın emsali ve gerekçesi: [`rationale/10_concurrency.md`](rationale/10_concurrency.md).

Bir model hizmeti aynı anda birden çok isteğe cevap verir, bir ajan birden çok uzak modele birlikte sorar, bir robot algılayıcıyı beklerken başka işi sürdürür. Bu bölüm işi görevlere bölmeyi, sonuçlarını almayı ve görevler arasında veriyi güvenle paylaşmayı söyler.

**10.1** `TR_async f(x)` ve `TR_async { ... }` işi ayrı bir görev olarak başlatır ve hemen devam eder. İkisi de bir ifadedir ve görev değeri verir: `TR_var g = TR_async f(x)`.
- Görevin tipi `TR_Task[T]`'dir; `T` fonksiyonun dönüş tipidir, blokta `TR_NoneType`'tır.
- `g.TR_wait()` görev bitene kadar bekler ve sonucunu verir; görev hatayla bittiyse o hatayı burada fırlatır (8).
- Görev değeri kullanılmayabilir (`TR_async f(x)` bir deyim olarak); o zaman görev yine çalışır ve 10.6'nın kuralı geçerlidir.
- Program bitmeden önce bütün görevleri bekler.
- *Test:* TR testi; görev değeri ve `.TR_wait()` uygulanınca TR testi.

**10.2** Yorumlayıcı kilidi: aynı anda yalnız bir görev TR kodu çalıştırır. Paylaşılan liste ve sözlük bozulmaz, bir deyim (`s += 1`) bölünmez; veri yarışı yoktur.
- Kilidi bekleyen bir görev varsa kilidi tutan görev onu deyimler arasında devreder: döngülerde en geç 256 turda bir ve her kullanıcı fonksiyonu çağrısında.
- Ağır hesap yapan yerleşikler (tensör işlemleri, toplu kodlama, veri okuma) kendi içinde paralel çalışır.
- *Test:* TR testi.

**10.3** Engelleyici işlemler kilidi bırakır: `TR_sleep`, beklemeler (`.TR_wait()`, `TR_wait_all`), girdi, ağ istekleri, dış süreç ve sunucu beklemeleri. Bekleyen görevler böylece iç içe ilerler: dört görevde `TR_sleep(0.2)` toplam yaklaşık 0,2 saniye sürer.
- *Test:* TR testi.

**10.4** Görev başlatıldığı kapsamı yakalar; döngünün içinde başlatılan her görev o turun değişkenlerini görür (5.12). Görevin içinde tanımlanan ad görevin kapsamındadır.
- *Test:* TR testi.

**10.5** `TR_wait_all()` başlatılmış bütün görevlerin bitmesini bekler. `TR_wait_all(görevler)` verilen görevleri bekler ve sonuçlarını **verildikleri sırayla** bir liste olarak verir (`TR_List[T]`); görevlerden biri hatayla bittiyse ilk hatalı görevin hatasını fırlatır (öbürleri yine beklenir).
- *Test:* Go: `TestConcurrentTasks`; liste biçimi uygulanınca TR testi.

**10.6** Görevin hatası kaybolmaz. Beklenen görevin hatası `.TR_wait()`'te ya da `TR_wait_all`'da fırlar. Hiç beklenmeyen görevin yakalanmamış hatası hata çıktısına (komut satırında standart hata, testte testin raporu, hata ayıklamada hata ayıklama konsolu) görevin başladığı satırla yazılır; program sürer ve sonunda hata koduyla biter (1.9).
- Testin başlattığı görevdeki yakalanmamış hata o testi kaldırır (12.5).
- *Test:* Go: `TestDAPTaskError`; `.TR_wait()`'te fırlaması uygulanınca TR testi.

**10.7** `g.TR_wait(TR_timeout: saniye)` en çok verilen süre kadar bekler; süre dolarsa çalışma hatası fırlatır, görev sürer. Sabit hızlı bir döngü ve uzak bir modele sorulan soru böyle sınırlanır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**10.8** Görevlerin birbirine göre sırası belirlenimci değildir: hangi görevin önce ilerlediği beklemelere ve zamana bağlıdır. Sonucu sıraya bağlı olmasın diye her görev sonucunu döndürür ve sonuçlar 10.5'teki gibi sırayla alınır. Paylaşılan bir listeye görevlerden eklemek bozulmaz (10.2) ama ekleme sırası tanımsızdır.
- *Test:* Go: `TestAsyncConcurrentDictAndListWrites`; sırayla toplama uygulanınca TR testi.
