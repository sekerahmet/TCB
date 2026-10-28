# 12. Testler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "12. Testler".

Her kuralın emsali ve gerekçesi: [`rationale/12_tests.md`](rationale/12_tests.md).

Ajan yazdığı kodu testle doğrular. Bu bölüm testin nasıl yazıldığını, nasıl çalıştığını ve neyin sessizce geçemeyeceğini söyler. `TR_cmd_test`'in kullanımı ve çıktısı: `docs/guide/07_testler.md`; örnek: `examples/tr/17_testler_test.tr`.

**12.1** `TR_test "ad" { ... }` bir test tanımlar. `TR_cmd_test` dosyanın testlerini yazıldıkları sırayla çalıştırır; dosya normal çalıştırıldığında test blokları atlanır.
- *Test:* TR testi.

**12.2** `TR_test` yalnız bir deyimin başında ve ardından bir metin gelince test başlatır; başka bir yerde yazılırsa sözdizimi hatasıdır (2.3a).
- **Aynı dosyada aynı adlı iki test** sözdizimi hatasıdır; ileti iki satırı da gösterir.
- *Test:* TR testi; aynı adlı testlerin hatası uygulanınca olumsuz fikstür.

**12.3** `TR_assert(koşul, mesaj)` koşul yanlışsa bir hata fırlatır; `mesaj` isteğe bağlıdır.
- Koşul `TR_Bool`'dur (3.10). Tensör koşul olamaz: `TR_assert(t == u)` denetim hatasıdır, ileti `TR_assert_equal(t, u)`'yu ya da `TR_all(...)`'ı önerir (15.5).
- Mesaj yerine `TR_Message(H…, …)` verilirse hata o iletinin koduyla fırlar (8.5).
- Bir hatanın beklendiğini sınamak `TR_error_code { ... }` ile yazılır (8.5): `TR_assert_equal("H7221", TR_error_code { ... })`.
- *Test:* TR testi.

**12.4** `TR_assert_equal(beklenen, gerçek, mesaj)` iki değer 4.5'in `==`'siyle eşit değilse bir hata fırlatır; `mesaj` isteğe bağlıdır.
- İleti ikisini de tipiyle gösterir: beklenen `"5"` (`String`), gerçek `5` (`Int`).
- İki tensörde anlamı `TR_equal`'ınkidir: şekil ve bütün öğeler (15.5).
- Ondalık kesin karşılaştırılır; toleransla karşılaştırma 12.4a'dadır.
- *Test:* TR testi.

**12.4a** `TR_assert_close(beklenen, gerçek, TR_rtol: r, TR_atol: a)` iki ondalık sayıyı ya da iki `TR_Float` tensörü toleransla karşılaştırır: her öğede `|gerçek − beklenen| ≤ a + r · |beklenen|`.
- Varsayılan toleranslar hassasiyete göredir (15.18a'nın tablosu; sayıda "float64"ün satırı).
- Tensörde şekiller aynı olmalıdır; yayın yoktur. NaN hiçbir şeye yakın değildir.
- İleti en büyük farkı, yerini ve kullanılan toleransları söyler.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**12.5** Test dosyasının en dışında yalnız tanımlar ve `TR_test` blokları bulunur (1.1); en dıştaki başka bir deyim sözdizimi hatasıdır.
- Önce tanımlar kurulur; sonra her test bu tanımların üstünde açılan yeni bir kapsamda çalışır. Bir testin yerel adları öbürüne sızmaz.
- Kalan ya da hata veren test öbürlerini durdurmaz.
- Testin başlattığı görevler beklenir; testteki `TR_defer` test bitince çalışır (9.1).
- **Her test paylaşılan rastgele üreteci aynı durumda başlar:** tohumsuz rastgelelik bir testin sonucunu öbür testlere ve sıraya bağlamaz.
- *Test:* TR testi, olumsuz fikstür; en dıştaki deyimin hatası ve üretecin durumu uygulanınca TR testi.

**12.6** Test gövdesinde, gövdedeki bir fonksiyonun dışında yazılan `TR_return` sözdizimi hatasıdır: test her zaman sonuna kadar çalışır, bir doğrulamayı atlayıp sessizce geçmez. Döngü dışındaki `TR_break` ve `TR_continue` da hatadır (5.13).
- *Test:* TR testi, olumsuz fikstür.
