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

**Dosya biçimi sözleşmedir.** Dosya ve klasör adları, JSON anahtarları ve kayıt türleri dilden
bağımsız İngilizce tanımlayıcılardır (`checkpoints/step_N/state.json`, `"kind": "optimizer"`);
değeri bir dil öğesi olan alan kodunu taşır (`"optimizer": "TR_fn_AdamW"`; dil tanımı 17.3).
Biçim değişirse sürüm artar; eski sürümler okunmaz (dil taslaktır, 17.6).
Denetim: `TestCheckpointFormatIsLanguageIndependent` (adlar, anahtarlar, türler, iyileştiricinin kodu ve durum tensörlerinin adları), `TestTamperedCheckpointRejected` (eski ve daha yeni sürüm, bilinmeyen alan reddedilir), `cmd/tr/e2e` `TestFileFormatsIndependentOfWritingLanguage` (adlar ve anahtarlar yazım diline göre değişmez).

**Durumlu değerler arayüzle bağlanır.** Bir motor değeri durumunu düz değerli bir sözlük
olarak verip geri yükleyebiliyorsa kontrol noktasına kaydedilir; arayüzün uygulayıcıları
derleme anında denetlenir.
Denetim: `TestStatefulImplementersAreAsserted` (yöntemleri yazan her tipin `var _ Stateful = (*T)(nil)` denetimi vardır; `dataset.Reader` için `helpers_test.go`).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_rollback_test.go`: `TR_mem_rollback` ile NaN'la bozulan adımdan geri dönülen eğitimin kesintisizle bit düzeyinde aynı bitmesi; nokta yokken açık hata
- `behavior_security_test.go`: kurcalanmış, yarım ve daha yeni biçimli kontrol noktası dosyalarının reddi; içeriği bozuk noktanın hiçbir nesneyi değiştirmeden reddedilip önceki noktayı önermesi; düz değerde fazladan JSON
- `behavior_stateful_test.go`: durumlu motor tiplerinin (`StateDict` + `PrepareState`) `Stateful` arayüzünü derleme anında denetlemesi
- `behavior_shared_containers_test.go`: düz değerde aynı iç kaba birden çok yerden bağ varken yüklemenin ve `TR_mem_rollback`'ün kayıttaki değerleri ezmeden ve belirlenimci yazması
- `behavior_corrupt_states_test.go`: bozuk iyileştirici ve okuyucu durumlarının uyumsuzluk değil bozukluk sayılıp önceki noktayı önermesi
- `behavior_sampler_window_test.go`: örnekleyici durumunda grubun pencere sayısını aşan konumun reddi
- `behavior_format_names_test.go`: noktanın dosya ve klasör adlarının, JSON anahtarlarının, nesne türlerinin ve iyileştirici durumu tensör adlarının dilden bağımsız İngilizce tanımlayıcı, iyileştiricinin adının kod olması
