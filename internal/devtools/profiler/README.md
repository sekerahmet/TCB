# profiler

`TR_cmd_watch` ve `TR_cmd_profile` komutlarının ortak çekirdeği: çalışan bir TR programının canlı
profilini çıkarır. Tensör sayaçlarını (işlem, süre, bayt), isteğe bağlı satır ve fonksiyon
örneklemesini, eğitim adımı sınırlarını, GPU kernel sürelerini ve belleği tek bir görüntüde
toplar; terminale canlı yazar, dosyaya rapor olarak kaydeder.

## Standartlar

**Profil programı yavaşlatmaz.** Örnekleme kapalıyken maliyet yalnızca sayaçlardır; adım
kancası yokken tek bir atomik okumadır. Örnekleyicinin paylaşılan değerleri yarış
dedektörüyle derlenen sürümde atomik okunup yazılır. Denetim: `TestSamplerSharedValuesAtomicUnderRace` (atomik kısım).

**Profil programı durdurmaz.** Rapor ya da durum dosyası yazılamazsa uyarı verilir, program
sürer; dosyalar atomik yazılır, okuyan hiçbir zaman yarım dosya görmez. Denetim: `TestProfileWriteErrorDoesNotStopProgram`, `TestCaptureKeepsDrainingAfterFileError`, `TestStatusFileReaderSeesNoPartialFile`.

**Dosya biçimi sözleşmedir.** Rapor ve durum dosyasının JSON anahtarları ve biçim sürümü,
okuyan araçların (VS Code eklentisi, `TR_cmd_watch`) sözleşmesidir; yazım dilinden bağımsızdır ve
çevrilmez. Biçim değişirse sürüm artar. Denetim: `TestSnapshotKeysFrozenPerVersion`, `TestProfileStatusFile`.

**Çıktıya göre davranış.** Terminalde canlı ekran tek yerde yenilenir; terminal olmayan
çıktıya seyrek, düz satırlar yazılır. Denetim: `TestProfileNonTerminalOutput`, `TestScreenLines`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_sampler_atomics_test.go`: örnekleyicinin paylaşılan değerlerinin yarış denetimli derlemede atomik okunup yazılması (`pos_plain.go` ve `pos_race.go` eşliği)
