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
- Koşul `TR_Bool`'dur (3.10). Tensör koşul olamaz, 0 boyutlu tensör de: `TR_assert(t == u)` denetim hatasıdır; ileti `TR_assert_equal(t, u)`'yu, `TR_all(...)`'ı, 0 boyutluda `.TR_item()`'ı ya da `TR_assert_close`'u önerir (15.5).
- Mesaj yerine `TR_Message(H…, …)` verilirse hata o iletinin koduyla fırlar (8.5).
- Bir hatanın beklendiğini sınamak `TR_error_code { ... }` ile yazılır (8.5): `TR_assert_equal("H7221", TR_error_code { ... })`.
- Değer döndürmez (13).
- `TR_grad_check` (15.32) de bir doğrulamadır: değer döndürmez, gradyanlar uyuşmazsa hata fırlatır.
- *Test:* TR testi.

**12.4** `TR_assert_equal(beklenen, gerçek, mesaj)` iki değer 4.5'in `==`'siyle (iki tensörde `TR_equal`'ın anlamıyla, aşağıda) eşit değilse bir hata fırlatır; `mesaj` isteğe bağlıdır.
- İki argüman aynı tipten olmalıdır; ayrı tipler 4.5'teki gibi denetim hatasıdır, ileti iki tipi söyler.
- İleti ikisini de gösterir: beklenen `[1, 2, 3]`, gerçek `[1, 2, 4]`; ilk farklı öğenin yerini de söyler.
- Değer döndürmez (13).
- İki tensörde anlamı `TR_equal`'ınkidir: şekil ve bütün öğeler (15.5).
- Ondalık kesin karşılaştırılır; toleransla karşılaştırma 12.4a'dadır.
- *Test:* TR testi.

**12.4a** `TR_assert_close(beklenen, gerçek, TR_rtol: r, TR_atol: a)` iki ondalık sayıyı ya da iki `TR_Float` tensörü toleransla karşılaştırır: her öğede fark, `a` ile `r` kere iki değerin mutlak değerinin büyüğünün toplamını aşmamalıdır. Formül simetriktir, argümanların sırası sonucu değiştirmez; 15.18a'nın aynı işi gören yerleşiği de bu formülü kullanır.
- Formül: |x − y| ≤ a + r · en_büyük(|x|, |y|).
- Varsayılan toleranslar hassasiyete göredir (15.18a'nın tablosu; sayıda "float64"ün satırı).
- Tensörde şekiller aynı olmalıdır; yayın yoktur. NaN hiçbir şeye yakın değildir.
- İleti en büyük farkı, yerini ve kullanılan toleransları söyler.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**12.5** Test dosyasının en dışında yalnız tanımlar ve `TR_test` blokları bulunur (1.1); en dıştaki başka bir deyim ve sabit olmayan bir ilk değer sözdizimi hatasıdır. Testlerin ortak kurulumu (geçici klasör, model) en dışta tanımlanan bir fonksiyondur ve her test onu kendi çağırır; böylece her test öbürlerinden bağımsızdır.
- **Her test kendi tanımlarıyla çalışır:** test dosyasının ve aktardığı modüllerin en dıştaki tanımları her test için yeniden kurulur; test bunların üstünde açılan yeni bir kapsamda çalışır. Bir testin en dıştaki bir değişkene atadığı değer, en dıştaki bir listeye eklediği öğe, bir modülün durumuna yaptığı değişiklik ve yerel adları sonraki teste geçmez. En dışta yalnız sabit ilk değer olduğu için (1.1) yeniden kurmanın bedeli küçüktür.
- Kalan ya da hata veren test öbürlerini durdurmaz.
- Testin başlattığı görevler beklenir; testteki `TR_defer` test bitince çalışır (9.1).
- **Her test paylaşılan rastgele üreteci programın başlangıç durumunda (20.17) başlar:** tohumsuz rastgelelik bir testin sonucunu öbür testlere ve sıraya bağlamaz. Testin görevleri aynı üreteçten çekerse 10.8'in sınırı geçerlidir.
- *Test:* TR testi, olumsuz fikstür; en dıştaki deyimin hatası, tanımların yeniden kurulması ve üretecin durumu uygulanınca TR testi.

**12.6** Test gövdesinde, gövdedeki bir fonksiyonun dışında yazılan `TR_return` sözdizimi hatasıdır: test her zaman sonuna kadar çalışır, bir doğrulamayı atlayıp sessizce geçmez. Döngü dışındaki `TR_break` ve `TR_continue` da hatadır (5.13).
- *Test:* TR testi, olumsuz fikstür.
