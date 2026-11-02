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

**10.2** Yorumlayıcı kilidi: aynı anda yalnız bir görev TR kodu çalıştırır. Paylaşılan liste ve sözlük bozulmaz. Kilidi bırakan bir çağrı, döngü ya da üreteç (5.11) içermeyen deyim (`s += 1`) bölünmez; kullanıcı fonksiyonu çağrısı, 10.3'ün engelleyici işleri ve uzun yerleşikler deyimin içinde kilidi devredebilir. Bileşik atamada hedefin değeri sağ taraftan sonra, kilit devredilmeden okunur (5.4): `t += yavaş()` eşzamanlı görevlerde de güncelleme kaybetmez.
- **Devir süreyledir:** kilidi bekleyen bir görev varsa kilidi tutan görev, kilidi aldığından bu yana 5 ms geçtikten sonraki ilk devir noktasında onu devreder. Devir noktaları deyim sınırı, döngünün her turu (5.7–5.9) ve üretecin her turudur (5.11): gövdesi boş bir döngü ya da büyük bir üreteç de bölünür. Tur sayısı ya da çağrı sayısı devri belirlemez; 1.5'in durdurma noktası ayrı bir kuraldır.
- **Sıra:** kilidi bekleyen görevler onu geldikleri sırayla alır.
- **Gecikme sözü:** bekleyen bir görev varken kilidi tutan görev onu en çok 5 ms ve ondan sonraki ilk devir noktasına kadar (o anki deyimin ya da turun sonu) tutar; tuttuğu sırada TR kodu, tensör ve veri yerleşikleri (tensör işlemleri, ileri ve geri yayılım, iyileştiricinin adımı, örnekleme, toplu kodlama, veri ve dosya okuma, cihaz beklemeleri) çalıştırıyor olabilir. 5 ms'den uzun sürebilen her yerleşik çalışırken kilidi bırakır. Önünde n görev bekleyen bir görev kilidi en çok (n + 1) × 5 ms ve n + 1 deyim ya da tur içinde alır: model çalıştıran bir görevle algılayıcı okuyan bir görevde 5 ms ve bir deyim ya da tur. Yalnız listenin ve sözlüğün metotları ve liste, sözlük, yapı ya da bunları içeren demet okuyan yerleşikler sözün dışındadır (aşağıda).
- Listenin ve sözlüğün metotları ve liste, sözlük, yapı ya da bunları içeren demet okuyan yerleşikler (`TR_json_string(liste)`) kilidi tutar; süreleri büyüklükle artar. Yalnız değişmez girdiyi (metin, `TR_JSON`, 3.7a) okuyup yeni değer kuran yerleşikler (`TR_json_parse`, metne ya da `TR_JSON`'a uygulanan `TR_json_string`, metnin metotları) paylaşılan durumu bozamaz; sözün içindedir ve uzun sürünce kilidi bırakır. Değişebilir bir girdiyi okurken kilidi bırakmak, başka görev ona yazarken yarış olurdu. Tensör okuyan yerleşik (`TR_json_string(t)` dahil) sözün içindedir: deponun iznini alır ve kilidi bırakır (aşağıda). Paylaşılan bir listeyi dolaşırken başka görevin onu değiştirmesi 5.7'nin hatasıdır. Büyük veri tensörde ve veri kümesinde tutulur (15, 16).
- **Her yerleşik bölünmez görünür:** bir yerleşiğin okuduğu tensöre o bitene kadar başka bir görev yazamaz; yazdığı tensörü başka bir görev okuyamaz da yazamaz da. Sonuç, yerleşik başka hiçbir görev araya girmeden çalışmış gibidir.
  - Okuma ve yazma izni tensörün **deposundadır**: aynı depoyu paylaşan görünümler (15) birbirini korur.
  - Yerleşik bütün izinlerini başlarken **hepsini birden** alır; biri alınamıyorsa hiçbirini tutmadan, kilidi bırakarak bekler. Böylece kilitlenme olmaz.
  - TR kodunu geri çağıran yerleşik (kullanıcının yazdığı türev, `TR_fuse`'un fonksiyonu, anahtarlı sıralama) geri çağırmadan önce izinlerini bırakır: bölünmezlik geri çağırmalar arasındaki parçalar içindir.
- Ağır hesap yapan yerleşikler ayrıca kendi içinde paralel çalışır.
- *Test:* TR testi; Go: `TestAsyncConcurrentDictAndListWrites`; sıra, gecikme sözü ve bölünmezlik uygulanınca Go testi (bekleyen görevin kilidi alma süresi tekdüze saatle ölçülür; iki ve üç meşgul görevle).

**10.3** Engelleyici işlemler kilidi bırakır: `TR_sleep`, beklemeler (`.TR_wait()`, `TR_wait_all`), girdi, dosya işleri, ağ istekleri, dış süreç ve sunucu beklemeleri. Bekleyen görevler böylece iç içe ilerler: dört görevde `TR_sleep(0.2)` toplam yaklaşık 0,2 saniye sürer.
- *Test:* TR testi.

**10.4** Görev başlatıldığı kapsamı yakalar; döngünün içinde başlatılan her görev o turun değişkenlerini görür (5.12). Görevin içinde tanımlanan ad görevin kapsamındadır. Görev açıldığı yerin türevsiz kipini ve `TR_mixed_precision` türünü de alır (15.4, 15.27); açıldıktan sonra açanın kipinin değişmesi ona geçmez.
- *Test:* TR testi; kipin ve hesap türünün mirası uygulanınca TR testi.

**10.5** `TR_wait_all()` başlatılmış bütün görevlerin bitmesini bekler. `TR_wait_all(görevler)` verilen görevleri bekler ve sonuçlarını **verildikleri sırayla** bir liste olarak verir (`TR_List[T]`).
- Görevlerden biri hatayla bittiyse öbürleri de beklenir ve **verilen sırada ilk hatalı görevin** hatası fırlar; bitme zamanı hangi hatanın fırladığını değiştirmez. Öbür hatalı görevlerin hataları, verilen sırayla, fırlayan hatanın `e.TR_suppressed` listesine eklenir (9.4 gibi); hiçbir hata kaybolmaz.
- Argümansız `TR_wait_all()` sonuç vermez; iptal edilmiş bir görev onu hataya düşürmez (iptal hata değildir, 10.6), hatayla biten görevlerin hatası 10.6'ya göre yazılır.
- *Test:* TR testi; liste biçimi uygulanınca TR testi.

**10.6** Görevin hatası kaybolmaz. Beklenen görevin hatası `.TR_wait()`'te ya da `TR_wait_all`'da fırlar. Değeri bir ada bağlanmamış görevin (deyim olarak yazılan `TR_async`, 10.1) hatası görev hatayla bitince hemen; değeri tutulan ama hiç beklenmeyen görevin hatası program ya da test biterken hata çıktısına (komut satırında standart hata, testte testin raporu, hata ayıklamada hata ayıklama konsolu) görevin başladığı satırla yazılır; program sürer ve sonunda hata koduyla biter (1.9). Sonradan `TR_try { g.TR_wait() }` ile yakalanan hata çıkış kodunu bozmaz.
- İptal edilen görevin (10.9) iptali bir hata olarak yazılmaz; iptalden önce görevin kendi hatası olduysa o yazılır.
- Testin başlattığı görevdeki yakalanmamış hata o testi kaldırır (12.5).
- *Test:* TR testi: `tests/eszamanli_test.tr` (tutulan görevin hatasının `.TR_wait()`'te fırlaması, beklenip yakalanan hatanın testi düşürmemesi); Go: `TestDAPTaskError`, `TestDAPUnwaitedTaskError`, `TestUnwaitedTaskErrorShownInReport`, `TestExitCodes` (beklenmeyen görevin hatasının program ya da test biterken başladığı satırla yazılması, çıkış kodu), `TestACancelledTaskStopsAtItsSafePoint` (iptalin yazılmaması), `TestAnAbandonedWaitAllKeepsTheErrors` (yarıda kalan `TR_wait_all(görevler)`'in görevlerinin hatasının program sonunda yazılması); iptalden önce olan kendi hatanın yazılmasının testi yok.

**10.7** `g.TR_wait(TR_timeout: saniye)` en çok verilen süre kadar bekler. Görev o sürede biterse sonucunu verir ya da hatasını fırlatır; süre dolarsa **görev iptal edilir** (10.9), durması beklenir ve zaman aşımı hatası görev durunca fırlar; toplam bekleme bu yüzden süreyi aşabilir. Sabit hızlı bir döngü ve uzak bir modele sorulan soru böyle sınırlanır; program uzak cevabı beklemeye bağlı kalmaz.
- İptal görevi onu bekleyen herkes için durdurur: aynı görevi başka bir yer de bekliyorsa o da iptal hatası alır.
- Zaman aşımı hatasının (H2921) ve iptal hatasının (H2920) kendi ileti kodları vardır (8.5); `TR_error_code { g.TR_wait(TR_timeout: 1) }` ile sınanır.
- Görevi iptal etmeden sonucunu sormak için `g.TR_done()` kullanılır: görev bittiyse (sonuçla, hatayla ya da iptalle) `TR_true` verir, beklemez. Sabit hızlı bir döngü her turda böyle sorar, bitmemişse önceki sonucu kullanır.
- *Test:* TR testi: `tests/eszamanli_test.tr` (süresinde biten görev, süre dolan görevin iptali, ertelenen işi ve H2921, aynı görevi sonra bekleyenin H2920'si, görevin kendi hatasının ve ertelenen işin hatasının H2921'in bastırılanlarına girmesi); Go: `TestAWaitWithATimeout` (süre denetimi), `TestATimeoutStopsABusyLoopSoon` (ağır turlu döngüde iptalin gecikmesi, tekdüze saat); aynı görevi aynı anda bekleyen ikinci yerin ve sabit hızlı döngünün testi yok.

**10.8** Görevlerin birbirine göre sırası belirlenimci değildir: hangi görevin önce ilerlediği beklemelere ve zamana bağlıdır. Sonucu sıraya bağlı olmasın diye her görev sonucunu döndürür ve sonuçlar 10.5'teki gibi sırayla alınır. Paylaşılan bir listeye görevlerden eklemek bozulmaz (10.2) ama ekleme sırası tanımsızdır.
- *Test:* gerekmez: sıranın tanımsızlığı sınanmaz; doğru yazım (sırayla toplama) 10.5'in testidir.

**10.9** `g.TR_cancel()` görevi iptal eder. İptal, 1.5'in durdurmasının tek bir göreve uygulanmasıdır: görev 1.5'in güvenli noktalarında durur (kullanıcı fonksiyonu çağrısı, döngünün ve üretecin her turu, süren bir yerleşiğin içi).
- Programın durumunu değiştirmeyen yerleşik (hesap, okuma, bekleme) bırakılır, sonucu atılır; durum değiştiren yerleşik (indeksle atama, `TR_backward`, iyileştiricinin adımı, dosyaya yazma) biter. 1.5'in listesi geçerlidir; bölünmezlik (10.2) bozulmaz.
- Görev program gibi biter (1.9): gövdesi bitince önce başlattığı ve henüz bitmemiş görevleri bekler, sonra kendi ertelenen işleri (`TR_defer`, 9.1) çalışır. Yakalanmayan bir hatayla ya da iptalle bittiyse bu alt görevler önce iptal edilir, durmaları beklenir, sonra ertelenenler çalışır. İptal görevin içinde `TR_catch` ile yakalanmaz, görev her zaman durur.
- İptal, görevin başlattığı ve henüz bitmemiş görevlere de geçer: iptal edilen görevden kimsenin beklemediği bir görev kalmaz. O alt görevin görev değerini başka bir görev bekliyorsa o da iptal hatası alır.
- İptal edilen görevi bekleyen `.TR_wait()` ve `TR_wait_all`, görev durana kadar bekler (süren durum değiştiren yerleşik, ertelenen işler ve iptal geçen alt görevler biter), sonra iptal hatası fırlatır (10.7'deki kodla); `g.TR_done()` ancak o zaman `TR_true` verir. `TR_wait_all(görevler)` için iptal de bir hatadır (10.5).
- Bitmiş görevi iptal etmenin etkisi yoktur. `TR_handle_stop` (1.6) iptali etkilemez: o program durdurma isteğini üstlenir, iptal edilen görev yine durur.
- *Test:* TR testi: `tests/eszamanli_test.tr` (beklemede ve döngü turunda durma, görevin içinde `TR_catch`'in yakalamaması, ertelenenlerin çalışması, gövdesi sürerken iptal edilen görevin alt görevinin de ana görevin ertelenenlerinden önce durması, bitmiş görevde etkisizlik); Go: `TestACancelledTaskStopsAtItsSafePoint`, `TestCancelReachesTheTasksChildren`, `TestDefersKeepTheirErrorsWhileUnwinding` (iptalde ertelenen işin hatası), `TestCancelReachesAFinishedBodysChildren` (gövdesi bitmiş, alt görevini bekleyen görevde iptalin ve süre sınırının alt göreve geçmesi, tekdüze saat), `TestAFinishedChildIsNotKept` (bitmiş alt görevlerin listeden budanması); durum değiştiren yerleşiğin bitmesinin, alt görevi bekleyen başka görevin ve `TR_handle_stop`'un testi yok.
