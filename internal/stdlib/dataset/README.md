# dataset

TR'nin veri kümesi yerleşikleri: `veri_kümesi(...)` ile açılan kümeler, `.örnekleyici(...)` ve
`.sıralı(...)` okuyucuları. Asıl okuma işini `internal/compute/dataload` yapar; bu paket onu
TR değerlerine ve yol gösterici hata iletilerine bağlar.

## Standartlar

**Go ile aynı veri.** Bir okuyucunun verdiği toplu işler, aynı ayarla kurulmuş Go
örnekleyicisinin verdikleriyle bit bit aynıdır; testler bunu doğrudan karşılaştırır.

**Bekleme kilitsizdir.** `.sonraki()` önden yükleyiciyi beklerken yorumlayıcı kilidini
bırakır; başka görevler bu sırada ilerler.

**Kaynak sızmaz.** Dosya tutamakları, bellek eşlemeleri ve arka plan gorutinleri sıralı
okuyucu bitince, nesne erişilemez olunca ya da program bitince kapanır; TR'de ayrı bir
kapatma çağrısı gerekmez.

**Sürdürme kesintisizle aynıdır.** Okuyucu durumu düz değerli bir sözlüktür; geri
yüklenen okuyucu kesintisiz eğitimle aynı toplu işleri aynı sırayla verir. Durum
sözlüğünün anahtarları kontrol noktası dosya biçiminin parçasıdır ve çevrilmez.

**Hata yol gösterir.** Go paketinin hataları kullanıcının anlayacağı TR iletilerine
çevrilir; ileti ne yapılması gerektiğini söyler.
