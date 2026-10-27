---
name: Sözlük isteği
about: Dile yeni bir ad (yerleşik, metot, anahtar kelime ya da adlı argüman) ve kod isteği
title: "Sözlük isteği: "
labels: sözlük isteği
---

<!--
Dilin kullanıcıya görünen her adı sabit bir koda bağlıdır (internal/lang/codes/tr.json). Kod
numarası ailesinin kuralıyla verilir, son adı baş geliştirici onaylar; bu şablon bir istektir, kodu kendiniz eklemeyin.
Önce docs/codes.md'ye bakın: aynı işi (fonksiyon ya da metot biçiminde) yapan bir kod varsa
onu kullanın. Aynı işi yapan iki kod olamaz.
-->

## İstenen iş

Ne işe yarayacak? Hangi programda, hangi adımda gerekiyor?

## İmza ve örnek

```tr
//! tr kodlu 3 dil=tr
// ör. değişken y = yeni_ad(x, eksen: -1)
```

İmza: girdiler, adlı argümanlar, dönüş tipi. Hata durumları (hangi girdide ne hata verir).

## Önerilen ad

Önerilen Türkçe ad ve neden bu ad. Fonksiyon mu, metot mu (`docs/codes.md`'nin başındaki kural: değer dönüştüren ya da değere soru soran iş metottur, matematik ve tensör işi fonksiyondur)?

## Aynı işi yapan mevcut kod var mı?

`docs/codes.md`'de baktığınız yakın kodlar (ör. `TR_fn_SGD SGD`, `TR_mem_sort sırala`) ve neden yetmedikleri.

## Sınıf önerisi

Hangi sınıfa girmeli (ör. `fn` yerleşik fonksiyon, `mem` metot ya da özellik, `arg` adlı argüman)? Kimlik `TR_<sınıf>_<İngilizce ad>` kuralıyla bir kez verilir (`TR_fn_sum`).
