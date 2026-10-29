# DURUM — status 5, 6, 7'nin yeniden ölçülmesi (Faz 0.1)

- Başlama: 2026-10-29 14:41 UTC (konteynerin saati)
- Taban: main `1073aea`; ölçümden sonra main `59efe46`'ya ilerledi (yalnız 8, 12, 16'nın belgeleri ve status'un başka bölümleri, motor aynı), dala birleştirildi.
- Ölçüm: depodan kurulan `tr` ikilisiyle, her kural için geçici klasörde küçük programlar; her biri hem denetimden (TR_cmd_check) hem çalıştırmadan geçirildi. Üç test dosyası da koşuldu: `deyimler_test.tr` 23/23, `fonksiyonlar_test.tr` 20/20, `yapilar_test.tr` 24/24 geçiyor.

## Biten bölümler

### 5 (commit: status 5)

- Silinen: 0. Değişen: 8 (5.1, 5.3, 5.4, 5.5, 5.6, 5.7, 5.12, 5.14; hepsi yeniden ölçüldü, eskimiş olanlar düzeltildi). Eklenen: 1 (5.11, sözlük üreteci yok).
- Uyuyor: 5.2, 5.8, 5.9, 5.10, 5.13.
- Eskimiş olanlar (status'un dediği, bugün öyle değil): `:=` ve ilk değersiz `TR_var` artık sözdizimi hatası (5.1); öğe sayısı tutmazsa H2700/H2701, `_` okunamıyor (5.3); on iki bileşik atama ve yerinde `+=` çalışıyor (5.4); `TR_else`'siz değer veren `TR_if` hata (5.5); eşleşmeyen eşleme H2702, `TR_default` iletisi (5.6); dolaşırken ekleme H2703/H2704 (5.7); `TR_while` iletisi (5.8). Girişteki "testler eski kuralları sınıyor" listesi de eskimişti, çıkarıldı.
- Yeni bulgu (status'ta yoktu): aktarılan modülün fonksiyonu aktaran dosyanın en dış adını okuyor (5.14); `TR_elif`'ten sonra ikinci, yanıltıcı sözdizimi hatası (5.5).

## Kalanlar

6, 7.

## Kararsız kaldıklarım

- 5.2 "uyuyor" sayıldı, ama kuralın örneği `TR_const l: TR_List[TR_Int] = []` bugün ayrışmıyor ("'=' bekleniyordu"). Bu parametreli tipin yazımıdır, 3.6'nın maddesinde (`TR_List[T]`) zaten var; 5.2'ye ayrıca madde açmadım.
- 5.8 "uyuyor" sayıldı: koşulun `TR_Bool` olması yalnız çalışırken denetleniyor, o 3.10'un maddesinde.

## Sorular

(henüz yok)
