# 7. Test Yazma ve Çalıştırma (`tr test`)

Testler kodun yanına `test "ad" { ... }` bloklarıyla yazılır. Dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır; yalnızca `tr test` çalıştırır. Örnek: `tr test examples/tr/17_testler_test.tr`.

```tr
// hesap_test.tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama doğru" {
    doğrula(topla(2, 2) == 4)
}

test "eşitlik" {
    eşit_mi(5, topla(2, 2))   // beklenen önce, gerçek sonra
}
```

- `doğrula(koşul, [mesaj])`: koşul yanlışsa test kalır.
- `eşit_mi(beklenen, gerçek, [mesaj])`: değerler `==` anlamında eşit değilse ikisini de tipiyle gösterir.
- `test` yalnızca deyim başında ve ardından bir metin gelince test başlatır; `değişken test = 5` gibi kullanımlar çalışmaya devam eder.
- Önce dosyanın kendisi çalışıp üst düzey tanımları oluşturur; sonra her test bu tanımların üzerinde **yeni bir kapsamda** çalışır. Kalan ya da hata veren test diğerlerini durdurmaz. (Üst düzey bir değişkene atama yapan test, sonrakilerin gördüğü değeri değiştirir.)

```text
> tr test .
📄 hesap_test.tr
  ✓ toplama doğru
  ✗ eşitlik — Satır 9: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)
       9 |     eşit_mi(5, topla(2, 2))   // beklenen önce, gerçek sonra

❌ 2 test, 1 geçti, 1 kaldı
```

`tr test dosya.tr` yalnızca o dosyayı, `tr test klasör` klasördeki (alt klasörler dahil, gizli klasörler hariç) tüm `*_test.tr` dosyalarını çalıştırır. Sözdizimi ya da tip hatası olan dosya "yüklenemedi" olarak sayılır. Bir test kalırsa ya da bir dosya yüklenemezse çıkış kodu 1'dir; böylece CI'da kapı olarak kullanılabilir.
