# dataload

Eğitim verisinin okunması: `.npy` parçaları, `meta.json`, yerel önbellek, örnekleyici, sıralı
okuyucu ve önden yükleyici.

## Standartlar

**Dosya biçimi sabittir.** `.npy` başlığının desteklenen sürümleri ve `meta.json` alanları
değişmez; bozuk ya da kesik bir dosya çökmeye değil açıklayıcı bir hataya yol açar.

**Örnekleme belirlenimcidir.** Aynı tohum aynı sırayı verir; kaydedilen durumdan sürdürülen
eğitim, kesintisiz eğitimle aynı veriyi görür.

**Önbellek kotasına uyar.** Kota yetmezse açıklayıcı bir hata verir; sessizce kaynaktan
okumaya dönmez. Aynı önbellek klasörünü paylaşan süreçler birbirini bozmaz.

**Kapatma temizdir.** Kapatılan okuyucu, önbellek ya da önden yükleyici arkada çalışan iş, açık
dosya ya da bellek eşlemesi bırakmaz.
