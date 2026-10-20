# checkpoint

TR'nin kontrol noktası yerleşiği: eğitimin modelini, iyileştiricisini, veri okuyucularını,
günlüğünü ve düz değerlerini bir koşu klasörüne yazar ve kaldığı yerden sürdürür.

## Standartlar

**Sürdürme kesintisizle aynıdır.** Kesilip sürdürülen bir eğitim, hiç kesilmemiş eğitimle
aynı sonucu verir; testler iki koşuyu doğrudan karşılaştırır.

**Yazım atomiktir.** Bir nokta önce geçici bir klasöre yazılır, sonra yerine konur; yarım
kalan bir yazım eski noktaları bozmaz ve sonraki açılışta temizlenir. Yazılamayan bir nokta
eğitimi durdurmaz, uyarı verir.

**Okunan nokta doğrulanır.** Her dosyanın boyu ve SHA-256 özeti denetlenir; JSON dosyaları
boy sınırlıdır ve katı çözülür. Kurcalanmış ya da başka bir koşuya ait bir nokta yol
gösterici bir hatayla reddedilir.

**Dosya biçimi sözleşmedir.** Dosya adları ve JSON anahtarları yazım dilinden bağımsızdır ve
çevrilmez. Biçim değişirse sürüm artar; eski sürümler okunmaya devam eder.

**Durumlu değerler arayüzle bağlanır.** Bir motor değeri durumunu düz değerli bir sözlük
olarak verip geri yükleyebiliyorsa kontrol noktasına kaydedilir; arayüzün uygulayıcıları
derleme anında denetlenir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_security_test.go`: kurcalanmış, yarım ve daha yeni biçimli kontrol noktası dosyalarının reddi
