# checkpoint

TR'nin kontrol noktası yerleşiği: eğitimin modelini, iyileştiricisini, veri okuyucularını,
günlüğünü ve düz değerlerini bir koşu klasörüne yazar ve kaldığı yerden sürdürür.

## Standartlar

**Sürdürme kesintisizle aynıdır.** Kesilip sürdürülen bir eğitim, hiç kesilmemiş eğitimle
aynı sonucu verir; testler iki koşuyu doğrudan karşılaştırır.
Denetim: `TestResumedSameAsUninterrupted`; ayrı süreçlerde `cmd/tr/e2e` `TestCheckpointResumeBitExact`.

**Yazım atomiktir.** Bir nokta önce geçici bir klasöre yazılır, sonra yerine konur; yarım
kalan bir yazım eski noktaları bozmaz ve sonraki açılışta temizlenir. Yazılamayan bir nokta
eğitimi durdurmaz, uyarı verir.
Denetim: `TestRenameErrorDoesNotStopTraining`, `TestLatestJSONAndStrayCheckpoint`, `TestStrayTempCheckpointCleaned`.

**Okunan nokta doğrulanır.** Her dosyanın boyu ve SHA-256 özeti denetlenir; JSON dosyaları
boy sınırlıdır ve katı çözülür. Kurcalanmış ya da başka bir koşuya ait bir nokta yol
gösterici bir hatayla reddedilir.
Denetim: `TestTamperedCheckpointRejected`, `FuzzCheckpointFiles`.

**Dosya biçimi sözleşmedir.** Dosya adları ve JSON anahtarları yazım dilinden bağımsızdır ve
çevrilmez. Biçim değişirse sürüm artar; eski sürümler okunmaya devam eder.
Denetim: `TestVersion1CheckpointReads` (eski sürüm okunur), `TestTamperedCheckpointRejected` (daha yeni sürüm ve bilinmeyen alan reddedilir), `cmd/tr/e2e` `TestFileFormatsIndependentOfWritingLanguage` (adlar ve anahtarlar yazım diline göre değişmez).

**Durumlu değerler arayüzle bağlanır.** Bir motor değeri durumunu düz değerli bir sözlük
olarak verip geri yükleyebiliyorsa kontrol noktasına kaydedilir; arayüzün uygulayıcıları
derleme anında denetlenir.
Denetim: `TestStatefulImplementersAreAsserted` (yöntemleri yazan her tipin `var _ Stateful = (*T)(nil)` denetimi vardır; `dataset.Reader` için `helpers_test.go`).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_security_test.go`: kurcalanmış, yarım ve daha yeni biçimli kontrol noktası dosyalarının reddi
- `behavior_stateful_test.go`: durumlu motor tiplerinin (`StateDict` + `LoadState`) `Stateful` arayüzünü derleme anında denetlemesi
