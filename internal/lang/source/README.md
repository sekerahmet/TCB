# source

Kaynaktan programa giriş noktası: bir `.tr` dosyasını bölümlerine ayırır (BOM, kodlu biçim
başlığı, gövde), doğru sözcük çözümleyiciyi seçer, ayrıştırır ve istenirse denetimden geçirir.
Kodlu biçim ile dil yazımları arasındaki çevirici de buradadır.

## Standartlar

**Dosyalar tek yerden okunur.** Komut satırı, etkileşimli kabuk, test koşucusu, hata ayıklayıcı ve
modül yükleyici bir `.tr` dosyasını yalnızca bu paket üzerinden okur; başlık kuralları başka yerde
yinelenmez. Denetim: `TestHeaderRulesLiveOnlyInSourceAndLexer`.

**Başlık kesindir.** İlk satır tam olarak kodlu biçim başlığıysa dosya kodludur. Başlığa benzeyen
ama tanınmayan bir ilk satır hatadır; sessizce Türkçe kaynak sayılmaz. Denetim: `TestHeaderRules`.

**Çeviri yeniden yazdırma değildir.** Yalnızca dilin sözcükleri değişir; boşluklar, satır sonları,
yorumlar, kullanıcının adları, metinler ve sayılar bayt bayt korunur. Yorumların ve ham metinlerin
içi hiçbir zaman çevrilmez. Denetim: `TestEncodeEdgeCases`, `TestEncodeBOMAndCRLF`, `FuzzEncodeDecode`.

**Çeviri kayıpsız olmalıdır, değilse dosyaya dokunulmaz.** Her çeviri denetlenir: anlam (sözdizimi
ağacı) aynı kalmalı, çevrilen metin geri çevrilince özgününe dönmelidir. Kodlu metnin bir yazım
diline çevrilmesinde bu denetim, sözcük çözümleyicinin okuyabildiği dil olan etkin dilde yapılır. Sözdizimi hatası, bozuk
UTF-8 ya da kayıp durumunda hata döner ve dosya yazılmaz. Depodaki bütün `.tr` dosyaları ve
belgelerdeki kod blokları bu denetimden geçmelidir. Denetim: `TestDecodeRefusesLossyView`, `TestEscapeInOtherLanguage`, `TestRepoTRFilesEncodeLossless`, `TestDocCodeBlocksEncodeLossless`.

**Kodsuz dosya Türkçedir.** Başlıksız dosya etkin dil (`TR_DIL`) ne olursa olsun Türkçe okunur
(17.7): sözcük çözümleyici ve ayrıştırıcının bağlamsal sözcükleri Türkçedir (`lexer.NewBodyIn`).
Etkin dil Türkçe değilse dosya önce kodlu biçime, oradan etkin dildeki görünümüne çevrilip
çalışır (kodlu dosya gibi). Kabuğa yazılan satır ise etkin dildedir (`Options.Typed`). Dosyaya
yazan `tr kodla` ve `tr çöz` Türkçeyle çalışır. Denetim: `TestUncodedFileIsTurkishInEveryLanguage`
(cmd/tr/e2e), `TestBodyInLanguage` (lexer), `TestContextWordsFollowLexer` (parser).

**Satırlar her gösterimde aynıdır.** Satır numaraları başlıktan sonra sayılır: kodlu dosyanın 1.
satırı, Türkçe gösterimin 1. satırıdır. Denetim: `TestCodedFilePrepares`, `TestEncodeKeepsLineStructure`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_single_reader_test.go`: kodlu biçim başlığı kurallarının yalnızca `source` ve `lexer` paketlerinde olduğu (öbür paketler dosyayı `source` üzerinden okur)
