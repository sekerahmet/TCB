---
name: Sözlük isteği
about: Dile yeni bir ad (yerleşik, metot, anahtar kelime ya da adlı argüman) ve kod isteği
title: "Sözlük isteği: "
labels: sözlük isteği
---

<!--
Dilin kullanıcıya görünen her adı sabit bir koda bağlıdır (internal/lang/codes/tr.json). Kodu ve
son adı YALNIZCA baş geliştirici verir; bu şablon bir istektir, kodu kendiniz eklemeyin.
Önce docs/codes.md'ye bakın: aynı işi (fonksiyon ya da metot biçiminde) yapan bir kod varsa
onu kullanın. Aynı işi yapan iki kod olamaz.
-->

## İstenen iş

Ne işe yarayacak? Hangi programda, hangi adımda gerekiyor?

## İmza ve örnek

```tr
// ör. değişken y = yeni_ad(x, eksen: -1)
```

İmza: girdiler, adlı argümanlar, dönüş tipi. Hata durumları (hangi girdide ne hata verir).

## Önerilen ad

Önerilen Türkçe ad ve neden bu ad. Fonksiyon mu, metot mu (`docs/codes.md`'nin başındaki kural: değer dönüştüren ya da değere soru soran iş metottur, matematik ve tensör işi fonksiyondur)?

## Aynı işi yapan mevcut kod var mı?

`docs/codes.md`'de baktığınız yakın kodlar (ör. `T401 SGD`, `Y202 sırala`) ve neden yetmedikleri.

## Aile önerisi

Hangi aileye ve gruba girmeli (ör. `T2xx` eleman bazlı matematik, `Y2xx` Liste, `V9xx` adlı argüman)? Kod numarasını baş geliştirici verir.
