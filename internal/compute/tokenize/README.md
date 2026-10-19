# tokenize

`tokenizer.json` biçimindeki parçalayıcıları yükler ve çalıştırır: normalleştiriciler, ön
bölücüler, modeller (BPE, WordPiece, Unigram, WordLevel), son işleyiciler, çözücüler ve
kendi düzenli ifade motoru.

## Standartlar

**Başvuruyla birebir.** Kodlama ve çözme, başvuru uygulamasının ürettiği altın çıktılarla
birebir aynıdır; desteklenmeyen bir bileşen sessizce yok sayılmaz, hata verir.

**Girdi güvenilmez.** Parçalayıcı dosyası da kodlanan metin de dışarıdan gelir. Düzenli ifade
motoru adım bütçesiyle çalışır; üstel ya da karesel büyüyebilecek desenler reddedilir. Hiçbir
girdi çökmeye yol açmaz (bulanıklaştırma testleriyle sınanır).

**Önbellek sonucu değiştirmez.** Önbellekli ve önbelleksiz kodlama aynı sonucu verir; toplu ve
paralel kodlama tek tek kodlamayla aynıdır.

**Geçersiz UTF-8 tanımlı biçimde işlenir.** Kayıplı çözme kuralı bellidir ve sınanır.
