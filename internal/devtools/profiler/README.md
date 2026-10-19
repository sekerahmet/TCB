# profiler

`tr izle` ve `tr profil` komutlarının ortak çekirdeği: çalışan bir TR programının canlı
profilini çıkarır. Tensör sayaçlarını (işlem, süre, bayt), isteğe bağlı satır ve fonksiyon
örneklemesini, eğitim adımı sınırlarını, GPU çekirdek sürelerini ve belleği tek bir görüntüde
toplar; terminale canlı yazar, dosyaya rapor olarak kaydeder.

## Standartlar

**Profil programı yavaşlatmaz.** Örnekleme kapalıyken maliyet yalnızca sayaçlardır; adım
kancası yokken tek bir atomik okumadır. Örnekleyicinin paylaşılan değerleri yarış
dedektörüyle derlenen sürümde atomik okunup yazılır.

**Profil programı durdurmaz.** Rapor ya da durum dosyası yazılamazsa uyarı verilir, program
sürer; dosyalar atomik yazılır, okuyan hiçbir zaman yarım dosya görmez.

**Dosya biçimi sözleşmedir.** Rapor ve durum dosyasının JSON anahtarları ve biçim sürümü,
okuyan araçların (VS Code eklentisi, `tr izle`) sözleşmesidir; yazım dilinden bağımsızdır ve
çevrilmez. Biçim değişirse sürüm artar.

**Çıktıya göre davranış.** Terminalde canlı ekran tek yerde yenilenir; terminal olmayan
çıktıya seyrek, düz satırlar yazılır.
