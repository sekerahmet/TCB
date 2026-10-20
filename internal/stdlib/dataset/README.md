# dataset

TR'nin veri kümesi yerleşikleri: `veri_kümesi(...)` ile açılan kümeler, `.örnekleyici(...)` ve
`.sıralı(...)` okuyucuları. Asıl okuma işini `internal/compute/dataload` yapar; bu paket onu
TR değerlerine ve yol gösterici hata iletilerine bağlar.

## Standartlar

**Go ile aynı veri.** Bir okuyucunun verdiği toplu işler, aynı ayarla kurulmuş Go
örnekleyicisinin verdikleriyle bit bit aynıdır; testler bunu doğrudan karşılaştırır.
Denetim: `TestSamplerSameAsGo`.

**Bekleme kilitsizdir.** `.sonraki()` önden yükleyiciyi beklerken yorumlayıcı kilidini
bırakır; başka görevler bu sırada ilerler.
Denetim: `TestNextReleasesLock`, `TestNextReleasesLockWhileWaiting`.

**Kaynak sızmaz.** Dosya tutamakları, bellek eşlemeleri ve arka plan gorutinleri sıralı
okuyucu bitince, nesne erişilemez olunca ya da program bitince kapanır; TR'de ayrı bir
kapatma çağrısı gerekmez.
Denetim: `TestResourcesClose`, `TestSequentialEndsWithNoneAndCloses`.

**Sürdürme kesintisizle aynıdır.** Okuyucu durumu düz değerli bir sözlüktür; geri
yüklenen okuyucu kesintisiz eğitimle aynı toplu işleri aynı sırayla verir. Durum
sözlüğünün anahtarları kontrol noktası dosya biçiminin parçasıdır ve çevrilmez.
Denetim: `TestReaderStateResumesSameBatches` (durum yükleyen okuyucu aynı toplu işleri verir, anahtarlar sabittir); kontrol noktasıyla birlikte `checkpoint` `TestResumedSameAsUninterrupted`.

**Hata yol gösterir.** Go paketinin hataları kullanıcının anlayacağı TR iletilerine
çevrilir; ileti ne yapılması gerektiğini söyler.
Denetim: `TestErrorMessages` (hata kodları; iletinin yol göstericiliği yargıdır, test edilemez).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_cache_quota_test.go`: önbellek kotası ve önbellek yolunun çözülmesi
- `behavior_lock_test.go`: `.sonraki()` beklerken yorumlayıcı kilidinin bırakılması
- `behavior_sample_data_test.go`: tests/data altındaki sentetik verinin üreteçle birebir aynı olması
