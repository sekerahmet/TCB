# tokenize

`tokenizer.json` biçimindeki parçalayıcıları yükler ve çalıştırır: normalleştiriciler, ön
bölücüler, modeller (BPE, WordPiece, Unigram, WordLevel), son işleyiciler, çözücüler ve
kendi düzenli ifade motoru.

## Standartlar

**Başvuruyla birebir.** Kodlama ve çözme, başvuru uygulamasının ürettiği altın çıktılarla
birebir aynıdır; desteklenmeyen bir bileşen sessizce yok sayılmaz, hata verir. Denetim: `TestSyntheticTokenizers`, `TestComponents`, `TestUnsupportedConfig`.

**Girdi güvenilmez.** Parçalayıcı dosyası da kodlanan metin de dışarıdan gelir. Düzenli ifade
motoru adım bütçesiyle çalışır; üstel ya da karesel büyüyebilecek desenler reddedilir. Hiçbir
girdi çökmeye yol açmaz (bulanıklaştırma testleriyle sınanır). Denetim: `TestRiskyPatternsRejected`, `TestStepLimitTriggers`, `FuzzRegexNoCrash`, `FuzzTokenizer`.

**Önbellek sonucu değiştirmez.** Önbellekli ve önbelleksiz kodlama aynı sonucu verir; toplu ve
paralel kodlama tek tek kodlamayla aynıdır. Denetim: `TestUncachedAndLong`, `TestBatchAndParallel`.

**Geçersiz UTF-8 tanımlı biçimde işlenir.** Kayıplı çözme kuralı bellidir ve sınanır. Denetim: `TestLossyUTF8`.
