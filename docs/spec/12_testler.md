# 12. Testler

**12.1** `TR_ctx_test "ad" { ... }` bir test tanımlar. `tr test` her testi sırayla çalıştırır; dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır.

```tr
//! tr kodlu 3 dil=tr
TR_kw_func topla(a, b) { TR_kw_return a + b }

TR_ctx_test "toplama doğru" {
    TR_fn_assert_equal(4, topla(2, 2))
}
```

**12.2** `TR_ctx_test` bağlamsal bir anahtar kelimedir: yalnızca bir deyimin başında ve ardından bir metin geldiğinde test başlatır; başka her yerde sıradan bir addır (`TR_kw_var test = 5`).

**12.3** `TR_fn_assert(koşul, [mesaj])` koşul yanlışsa hata fırlatır: `doğrulama başarısız: mesaj`; mesaj yerine bir `TR_type_Message(H…, …)` verilirse hata o iletinin kendi koduyla fırlar (8.5).

**12.4** `TR_fn_assert_equal(beklenen, gerçek, [mesaj])` iki değer `==` anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır: `eşitlik başarısız: beklenen "5" (Metin), gerçek 5 (Tam)`. Her iki yerleşik test dışında da (ör. sözleşme denetimi için) kullanılabilir.

**12.5** Test dosyasının en dışında yalnız tanım ve `TR_ctx_test` blokları bulunur (1.1); önce bu tanımlar kurulur, sonra her test bu tanımların üzerinde açılan **yeni bir kapsamda** çalışır. Bir testin yerel adları diğerine sızmaz; kalan ya da hata veren test diğerlerini durdurmaz. Testin başlattığı görevler beklenir; testteki `TR_kw_defer` ondan sonra, test bitince çalışır (programın üst düzeyindeki gibi, 9.3).
- *Durum:* en dışta yalnız tanım kuralı karar (Ahmet, 2026-10-27; 1.1). Bugün test dosyasının en dışındaki deyimler de testlerden önce çalışıyor.

**12.6** Test gövdesinde, gövdedeki bir fonksiyonun dışında yazılan `TR_kw_return` derleme hatasıdır: test her zaman sonuna kadar çalışır, bir doğrulamayı atlayarak sessizce "geçmez". (Döngü dışındaki `TR_kw_break`/`TR_kw_continue` da her yerde olduğu gibi hatadır, 5.13.)

`tr test` kullanımı ve çıktı biçimi için `docs/guide/07_testler.md`'ye bakın; kısa bir örnek: `examples/tr/17_testler_test.tr`.
