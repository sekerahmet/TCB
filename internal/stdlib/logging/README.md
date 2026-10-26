# logging

TR'nin eğitim günlüğü ve ilerleme çubuğu yerleşikleri: `günlük(yol)` ve `ilerleme(toplam)`.
Araçlar (`tr izle`, `tr profil`) sayı ve süre yazımını buradan alır, böylece her yerde aynı
görünür.

## Standartlar

**Günlük biçimi sözleşmedir.** Günlük JSON satırlarıdır (`.jsonl`); kayıt alanlarının adları, kayıt
türleri (`start`, `end`, `resume`), `progress.json`'un adı ve anahtarları dilden bağımsız İngilizce
tanımlayıcılardır, çevrilmez (dil tanımı 17.3). Programın ölçü adları programındır: kodlu çıplak
anahtar kodun dilden bağımsız adıyla yazılır (`{kayıp: k}` → `"loss"`). Biçim değişirse sürüm artar;
eski sürüm okunmaz.
Denetim: `TestLogFormatNamesAreLanguageIndependent` (adlar ve sürüm), `TestLogRecords`, `TestJobStatus`, `cmd/tr/e2e` `TestFileFormatsIndependentOfWritingLanguage`, `TestDictCodedKeysOnDisk`.

**Kayıt kaybolmaz.** Kayıtlar dosyanın sonuna eklenir; önceki koşu yarım satır bıraktıysa yeni kayıt yeni satırda başlar,
kontrol noktasından sürdürmede "devam" kaydı düşülür. Program bitince açık günlükler kapanır,
bitmemiş çubuklar son satırlarını yazar.
Denetim: `TestLogAppendAndResume`, `TestLogEmptyClose`, `TestCloseAllFinishesBar`.

**Çıktıya göre davranış.** Terminalde çubuk tek satırda canlı çizilir; dosyaya ya da boruya
giden çıktıda seyrek, düz satırlar yazılır. Çubuk programın kendi çıktısını bozmaz.
Denetim: `TestProgressPlainLinesAndLog`, `TestProgressTerminal`, `TestNestedProgressRedrawsOuter`.

**Sayılar Türkçe yazılır.** Ölçüler 3 anlamlı basamakla ve ondalık virgülle, süreler
"3 dk 2 sn" biçiminde gösterilir.
Denetim: `TestFormatNumber`, `TestFormatDurationAndBar`.

**Adım kancası ucuzdur.** Canlı profil adım sınırlarını bir kancayla izler; kanca yokken
maliyet tek bir atomik okumadır.
Denetim: `TestStepHookCostsOneAtomicRead` (kanca yalnızca `atomic.Pointer.Load` ile okunur), `TestStepHook`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_format_names_test.go`: günlüğün ve `progress.json`'un anahtarlarının, kayıt türlerinin ve dosya adının dilden bağımsız İngilizce tanımlayıcı olması (dondurulmuş liste ve sürüm)
