# 10. Eşzamanlılık ve yorumlayıcı kilidi

**10.1** `eşzamanlı { blok }` ya da `eşzamanlı çağrı(...)` işi ayrı bir görev olarak başlatır ve hemen devam eder; `bekle_hepsi()` başlatılmış tüm görevlerin bitmesini bekler. Program da bitmeden önce görevleri bekler. Görevdeki yakalanmamış hata programı durdurmaz, yorumlayıcının hata çıktısına yazılır: komut satırında standart hata, `tr test` altında testin raporu (testin başlattığı görevdeki yakalanmamış hata — başarısız `doğrula` dahil — o testi kaldırır: görevler beklendikten sonra testin hatası olur; Go'da başka bir görevden `t.Error` da testi kaldırır), düzenleyicinin hata ayıklama oturumunda hata ayıklama konsolu (`Eşzamanlı görev hatası (satır N'de başlatılan): ...`).

**10.2** Yorumlayıcı kilidi: aynı anda yalnızca bir görev TR kodu çalıştırır; bu yüzden paylaşılan listeler/sözlükler bozulmaz ve `s += 1` gibi tek bir deyim bölünmez. Kilidi bekleyen bir görev varsa kilidi tutan görev onu deyimler arasında devreder: döngülerde düzenli aralıklarla (her 256 turda bir) ve her kullanıcı fonksiyonu çağrısında. Böylece CPU-yoğun bir döngü sürerken de `eşzamanlı` görevler ilerler.

```tr
değişken toplam = 0
döngü i içinde 1..=200 { eşzamanlı { toplam += i } }
bekle_hepsi()      // toplam == 20100
```

**10.3** Engelleyici işlemler (`bekle(saniye)`, `bekle_hepsi()`, `girdi()`, ağ istekleri, dış süreç ve sunucu beklemeleri) kilidi bırakır; böylece bekleyen görevler iç içe ilerler (4 görevde `bekle(0.2)` toplam ~0.2 sn sürer).

**10.4** Görev, başlatıldığı kapsamı yakalar; döngü içinde başlatılan her görev o turun değişkenlerini görür.
