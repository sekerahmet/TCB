# gguf

GGUF sürüm 3 dosyasının başlığını çözer: üstveri ve tensör kayıtları (dil tanımı 15.55, 16.1). `ml` tensörleri, `stdlib/tokenizer` parçalayıcıyı buradan okur; iki paket birbirini içe aktarmaz, ayrıştırıcı burada tektir.

## Standartlar

**Dosya güvenilmeyen girdidir.** Anahtar ve tensör sayıları, metin ve dizi boyları, hizalama, kaymalar ve boyutlar taşma denetimli hesaplanır; bildirilen veri dosyada olmalıdır, denetimler ayırmadan önce yapılır. Ayırma dosyayla orantılıdır, dosya boyunun altında değildir: bildirilen boya hiçbir yerde güvenilmez, ama en küçük anahtar kaydı dosyada 13, tensör kaydı 25 bayt tutarken bellekte eşleme ve başlıklarla yüzlerce bayt tutar. Yalnız anahtar ya da yalnız boş tensör taşıyan dosyada toplam ayırma dosyanın 13 ile 16 katıdır (DENETİM 133'ün ölçüsü, test sınırı 24 kat). Gerçek dosyalarda anahtar en çok 40, tensör en çok 398'dir; llama.cpp'de de sayı tavanı yoktur, sabit kat için tavan konmadı. `Decode` `size`'ı zorlar: akış uzun olsa da aşılmaz. Denetim: `TestDecodeRejects`, `TestDecodeTruncatedEverywhere`, `TestDecodeHoldsToSize`, `TestDecodeAllocatesWithinTheFile`, `FuzzGGUFDecode` (`internal/stdlib/ml`).

**Bütün türlerin boyu bilinir.** TR'nin okumadığı türler de `typeTable`'dadır: üstveriyi ve parçalayıcıyı okumak için bütün dosyanın düzeni denetlenir, `Q5_0` taşıyan dosyanın üstverisi okunur. Hangi türlerin tensöre dönüştüğü `ml`'in işidir. Denetim: `TestDecodeAcceptsEveryKnownType`.

**Veri okunmaz.** `Decode` tensör kayıtlarının sonunda durur; verinin yeri `DataStart` ve `Tensor.Offset`'tedir.

**`gguftest` yalnızca testlerindir.** Depoya model dosyası girmez; testler dosyalarını elle kurar.
