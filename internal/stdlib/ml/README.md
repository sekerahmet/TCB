# ml

TR'nin tensör ve yapay zekâ yerleşikleri: tensör değeri, işleçler (`+ - * / @ **`), sinir ağı
işlemleri (normlar, dikkat, çapraz entropi), indirgemeler, iyileştiriciler (AdamW, SGD, Muon),
özel türevler, karışık hassasiyet ve tensör dosyaları. Asıl hesabı `internal/compute/tensor`
yapar; bu paket onu TR değerlerine, adlarına ve yol gösterici hata iletilerine bağlar.

## Standartlar

**Hesap çekirdektedir.** Bu paket tensör hesabı yazmaz; çekirdeğe ne yapılacağını söyler.
Aynı işlemin CPU ve GPU sonuçları aynıdır (aynı çekirdek sözleşmesi).

**Adlar sözlükten gelir.** Yerleşiklerin, üyelerin ve adlı argümanların adları sabit kodlara
bağlıdır; ad değişikliği yalnızca çeviri dosyasını değiştirir.
Denetim: `TestBuiltinsMatchDictionary`, `TestMembersMatchDictionary` (`internal/stdlib`), `TestNoBareCodes` (`tools/gen/codeconsts`).

**Karışık hassasiyet bloğa bağlıdır.** `karışık_hassasiyet(tür) { ... }` görevin durumudur:
başka görevlere sızmaz, blok bitince (hata ile de) önceki tür geri gelir. Ağırlıkların türü
değişmez; dönüşümler türevlenebilir kopyalardır. Kurallar dil tanımının 15.13 maddesindedir.
Denetim: `TestMixedPrecisionBlock`, `TestMixedPrecisionTaskIsolation`, `TestMixedPrecisionGradient`, `TestOptimizerMasterCopy`.

**Tensör dosyası doğrulanır.** Başlık, sürüm, tür, boyutlar, eleman sınırı, uzunluk ve sağlama
toplamı denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir. Biçim değişirse sürüm
artar; deneme döneminde eski sürümler okunmaz, açık bir sürüm hatası verilir (uyumluluk sözü sürüm politikasıyla gelir, yol haritası Evre 4; yerel ağırlık biçimi de safetensors olacak).
Denetim: `TestTensorSaveLoad`, `TestTensorSaveVersion2`, `TestSaveZeroAxisLimit`, `FuzzTensorFileDecode`.

**Hata panik değildir.** Çekirdeğin panikleri (şekil, cihaz belleği) TR hatasına çevrilir;
ileti hangi işlemin neden yapılamadığını ve ne yapılacağını söyler.
Denetim: `TestPanicToError`, `TestWrappedBuiltinPanicIsCatchable`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_file_paths_test.go`: tensör dosya işlemlerinde yol çözümü ve Türkçe hata iletileri
- `behavior_scalar_tensor_test.go`: 0 boyutlu tensörün sıradan tensör gibi işlemlere ve türeve girmesi
- `behavior_tensor_test.go`: tensör işlemlerinin uçtan uca davranışı: ileri/geri işlemler, yazdırma, türev, doğrusal bağlanım ve İzleme Masası
- `behavior_grad_composite_specs_test.go`: bileşik gradyan kayıtları (kayıt başına girdi şekli, float64 başvuru, bf16 tolerans)
- `behavior_grad_composite_test.go`: bileşik türevlerin gradyan kapısı (yerleşiğin karışık hassasiyet dönüşümü + tensör işlemi zinciri): float64 sonlu fark ve bf16 tolerans (ölçmek için `TR_MEASURE_BF16=1 go test ./internal/stdlib/ml -run TestMeasureCompositeBF16 -v`)
- `behavior_grad_composite_coverage_test.go`: türevli tensör işlemlerinden ikisini ya da fazlasını birleştiren her fonksiyonun bileşik gradyan kaydı vardır; testsiz yeni bileşik testi düşürür (istisnalar gerekçeleriyle dosyada)
