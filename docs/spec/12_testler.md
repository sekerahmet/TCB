# 12. Testler

**12.1** `test "ad" { ... }` bir test tanımlar. `tr test` her testi sırayla çalıştırır; dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır.

```tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama doğru" {
    eşit_mi(4, topla(2, 2))
}
```

**12.2** `test` bağlamsal bir anahtar kelimedir: yalnızca bir deyimin başında ve ardından bir metin geldiğinde test başlatır; başka her yerde sıradan bir addır (`değişken test = 5`).

**12.3** `doğrula(koşul, [mesaj])` koşul yanlışsa hata fırlatır: `doğrulama başarısız: mesaj`.

**12.4** `eşit_mi(beklenen, gerçek, [mesaj])` iki değer `==` anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır: `eşitlik başarısız: beklenen "5" (Metin), gerçek 5 (Tam)`. Her iki yerleşik test dışında da (ör. sözleşme denetimi için) kullanılabilir.

**12.5** Önce dosyanın kendisi (testler hariç) çalışıp üst düzey tanımları oluşturur; sonra her test bu tanımların üzerinde açılan **yeni bir kapsamda** çalışır. Bir testin yerel adları diğerine sızmaz; kalan ya da hata veren test diğerlerini durdurmaz. Testteki `ertele` test bitince çalışır, testin başlattığı görevler beklenir.

**12.6** Test gövdesinde, gövdedeki bir fonksiyonun dışında yazılan `sonuç` derleme hatasıdır: test her zaman sonuna kadar çalışır, bir doğrulamayı atlayarak sessizce "geçmez". (Döngü dışındaki `çık`/`devam` da her yerde olduğu gibi hatadır, 5.13.)

`tr test` kullanımı ve çıktı biçimi için `docs/guide/07_testler.md`'ye bakın; kısa bir örnek: `examples/03_testler_test.tr`.
