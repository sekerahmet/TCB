# gguf

GGUF sürüm 3 dosyasının başlığını çözer: üstveri ve tensör kayıtları (dil tanımı 15.55, 16.1). `ml` tensörleri, `stdlib/tokenizer` parçalayıcıyı buradan okur; iki paket birbirini içe aktarmaz, ayrıştırıcı burada tektir.

## Standartlar

**Dosya güvenilmeyen girdidir.** Anahtar ve tensör sayıları, metin ve dizi boyları, hizalama, kaymalar ve boyutlar taşma denetimli hesaplanır; bildirilen veri dosyada olmalıdır, denetimler ayırmadan önce yapılır. Ayırma dosyayla orantılıdır: Go'nun metin başlığı öğe başına 16 bayttır, dosyadaki en küçük metin 8 bayt; bildirilen boya hiçbir yerde güvenilmez. Denetim: `TestDecodeRejects`, `TestDecodeTruncatedEverywhere`, `TestDecodeAllocatesWithinTheFile`, `FuzzGGUFDecode` (`internal/stdlib/ml`).

**Bütün türlerin boyu bilinir.** TR'nin okumadığı türler de `typeTable`'dadır: üstveriyi ve parçalayıcıyı okumak için bütün dosyanın düzeni denetlenir, `Q5_0` taşıyan dosyanın üstverisi okunur. Hangi türlerin tensöre dönüştüğü `ml`'in işidir. Denetim: `TestDecodeAcceptsEveryKnownType`.

**Veri okunmaz.** `Decode` tensör kayıtlarının sonunda durur; verinin yeri `DataStart` ve `Tensor.Offset`'tedir.

**`gguftest` yalnızca testlerindir.** Depoya model dosyası girmez; testler dosyalarını elle kurar.
