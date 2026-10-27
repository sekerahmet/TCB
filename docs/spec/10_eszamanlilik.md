# 10. Eşzamanlılık ve yorumlayıcı kilidi

**10.1** `TR_kw_async { blok }` ya da `TR_kw_async çağrı(...)` işi ayrı bir görev olarak başlatır ve hemen devam eder; `TR_fn_wait_all()` başlatılmış tüm görevlerin bitmesini bekler. Program da bitmeden önce görevleri bekler. Görevdeki yakalanmamış hata programı durdurmaz, yorumlayıcının hata çıktısına yazılır: komut satırında standart hata, `tr test` altında testin raporu (testin başlattığı görevdeki yakalanmamış hata — başarısız `TR_fn_assert` dahil — o testi kaldırır: görevler beklendikten sonra testin hatası olur; Go'da başka bir görevden `t.Error` da testi kaldırır), düzenleyicinin hata ayıklama oturumunda hata ayıklama konsolu (`Eşzamanlı görev hatası (satır N'de başlatılan): ...`).

**10.2** Yorumlayıcı kilidi: aynı anda yalnızca bir görev TR kodu çalıştırır; bu yüzden paylaşılan listeler/sözlükler bozulmaz ve `s += 1` gibi tek bir deyim bölünmez. Kilidi bekleyen bir görev varsa kilidi tutan görev onu deyimler arasında devreder: döngülerde düzenli aralıklarla (her 256 turda bir) ve her kullanıcı fonksiyonu çağrısında. Böylece CPU-yoğun bir döngü sürerken de `TR_kw_async` görevler ilerler.

```tr
//! tr kodlu 3 dil=tr
TR_kw_var toplam = 0
TR_kw_for i TR_kw_in 1..=200 { TR_kw_async { toplam += i } }
TR_fn_wait_all()      // toplam == 20100
```

**10.3** Engelleyici işlemler (`TR_fn_sleep(saniye)`, `TR_fn_wait_all()`, `TR_fn_input()`, ağ istekleri, dış süreç ve sunucu beklemeleri) kilidi bırakır; böylece bekleyen görevler iç içe ilerler (4 görevde `TR_fn_sleep(0.2)` toplam ~0.2 sn sürer).

**10.4** Görev, başlatıldığı kapsamı yakalar; döngü içinde başlatılan her görev o turun değişkenlerini görür. `TR_kw_async çağrı(...)` içinde tanımlanan adlı fonksiyon görevin kapsamındadır, çevre kapsama girmez (5.12).
