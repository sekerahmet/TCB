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

**Karışık hassasiyet bloğa bağlıdır.** `TR_fn_mixed_precision(tür) { ... }` görevin durumudur:
başka görevlere sızmaz, blok bitince (hata ile de) önceki tür geri gelir. Ağırlıkların türü
değişmez; dönüşümler türevlenebilir kopyalardır. Kurallar dil tanımının 15.4 maddesindedir.
Denetim: `TestMixedPrecisionBlock`, `TestMixedPrecisionTaskIsolation`, `TestMixedPrecisionGradient`, `TestOptimizerMasterCopy`.

**Tensör dosyası doğrulanır.** Başlık, sürüm, tür, boyutlar, eleman sınırı, uzunluk ve sağlama
toplamı denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir. Biçim değişirse sürüm
artar; deneme döneminde eski sürümler okunmaz, açık bir sürüm hatası verilir (uyumluluk sözü sürüm politikasıyla gelir; yerel ağırlık biçimi de safetensors olacak).
Biçim yolun uzantısından seçilir: `.safetensors` ile biten yol safetensors yazar (dil tanımı 15.47a; tensörler hizaya göre, önce geniş türler, aynı genişlikte sözlüğün sırasıyla), öbürleri TR biçimini.
Denetim: `TestTensorSaveLoad`, `TestTensorSaveVersion2`, `TestSaveZeroAxisLimit`, `FuzzTensorFileDecode`, `TestSaveTensorsChoosesFormatByExtension`, `TestSafetensorsPythonRoundTrip`.

**Hata panik değildir.** Çekirdeğin panikleri (şekil, cihaz belleği) TR hatasına çevrilir;
ileti hangi işlemin neden yapılamadığını ve ne yapılacağını söyler.
Denetim: `TestPanicToError`, `TestWrappedBuiltinPanicIsCatchable`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_gpu_typed_test.go`: GPU'da cinsli yolların (argmax'in `TR_Int` sonucu, taşıma, tam sayı içinde dönüşüm, `TR_Bool` karşılaştırma ve maske, `TR_where`) CPU ile aynı olması; öbür cinsli işlemin hassasiyet hatası (2a adım 8)
- `behavior_device_rules_test.go`: indeksle atamada cihaz kuralları (15.40) ve `TR_recompute` kaydında sayı işleneninin cihazda doldurulması (15.45)
- `behavior_device_args_test.go`: liste argümanlarının girdinin cihazında çevrilmesi (15.38) ve farklı cihaz iletisinin `.TR_to_device(...)` önermesi (15.40)
- `behavior_file_paths_test.go`: tensör dosya işlemlerinde yol çözümü ve Türkçe hata iletileri
- `behavior_scalar_tensor_test.go`: 0 boyutlu tensörün sıradan tensör gibi işlemlere ve türeve girmesi
- `behavior_tensor_test.go`: tensör işlemlerinin uçtan uca davranışı: ileri/geri işlemler, yazdırma, türev, doğrusal bağlanım ve İzleme Masası
- `behavior_grad_composite_specs_test.go`: bileşik gradyan kayıtları (kayıt başına girdi şekli, float64 başvuru, bf16 tolerans)
- `behavior_grad_composite_test.go`: bileşik türevlerin gradyan kapısı (yerleşiğin karışık hassasiyet dönüşümü + tensör işlemi zinciri): float64 sonlu fark ve bf16 tolerans (ölçmek için `TR_MEASURE_BF16=1 go test ./internal/stdlib/ml -run TestMeasureCompositeBF16 -v`)
- `behavior_grad_composite_coverage_test.go`: türevli tensör işlemlerinden ikisini ya da fazlasını birleştiren her fonksiyonun bileşik gradyan kaydı vardır; testsiz yeni bileşik testi düşürür (istisnalar gerekçeleriyle dosyada)
- `behavior_grad_clip_overflow_test.go`: `TR_fn_clip_grad`'ta float32'de taşan normun sonsuz sayılıp gradyanları sıfırlaması
- `behavior_grad_clip_version_test.go`: `TR_fn_clip_grad`'ın gradyan sürümünü CPU ve GPU'da aynı kuralla artırması (katı kipin hatasında artırmaması)
- `behavior_fused_comparison_grad_test.go`: `TR_fuse` içindeki karşılaştırma ve `TR_fn_is_finite`'in türev izlememesi
- `behavior_safetensors_order_test.go`: aynı ofsette boş ve dolu tensörlü geçerli safetensors dosyasının okunması
- `behavior_lock_release_test.go`: uzun tensör işi (`@`) sürerken 5 ms'de bir uyanan görevin kilidi geri alma süresi ve 100 Hz döngünün kaçan turu, tekdüze saatle (10.2; ölçmek için `go test ./internal/stdlib/ml -run TestOtherTasksRunDuringLongTensorWork -v -count=20`)
- `behavior_lock_threshold_test.go`: kilidi bırakma eşiklerinin (öğe öğe 2^23, adım 2^20) dayanağı; her işlem ve boyda doğrudan ve kilit bırakılarak süre, tek görevde ve başka görev canlıyken (10.2; ölçmek için `go test ./internal/stdlib/ml -run TestUnlockThresholdCost -v`)
- `behavior_safetensors_header_test.go`: safetensors başlığının katı okunması (alan adları, yinelenen anahtar, null, vekil kaçışı)
