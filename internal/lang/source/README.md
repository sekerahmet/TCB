# source

Kaynaktan programa giriş noktası: bir `.tr` dosyasını bölümlerine ayırır (BOM, kodlu biçim
başlığı, gövde), doğru sözcük çözümleyiciyi seçer, ayrıştırır ve istenirse denetimden geçirir.
Kodlu biçim ile dil yazımları arasındaki çevirici de buradadır.

## Standartlar

**Dosyalar tek yerden okunur.** Komut satırı, etkileşimli kabuk, test koşucusu, hata ayıklayıcı ve
modül yükleyici bir `.tr` dosyasını yalnızca bu paket üzerinden okur; başlık kuralları başka yerde
yinelenmez. Denetim: `TestHeaderRulesLiveOnlyInSourceAndLexer`.

**Başlık kesindir.** İlk satır (ilk satır `#!` ise ikinci satır) tam olarak `# TR 4` ise dosyadır.
Başlığa benzeyen ama tanınmayan satır (başka sürüm) hatadır; başlıksız metin çalışmaz, sessizce bir
dilde yazılmış sayılmaz. Denetim: `TestHeaderRules`.

**Çeviri yeniden yazdırma değildir.** Yalnızca dilin sözcükleri değişir; boşluklar, satır sonları,
yorumlar, kullanıcının adları, metinler ve sayılar bayt bayt korunur. Yorumların ve ham metinlerin
içi hiçbir zaman çevrilmez. Denetim: `TestEncodeEdgeCases`, `TestEncodeBOMAndCRLF`, `FuzzEncodeDecode`.

**Çeviri kayıpsız olmalıdır, değilse dosyaya dokunulmaz.** Her çeviri denetlenir: anlam (sözdizimi
ağacı) aynı kalmalı, çevrilen metin geri çevrilince özgününe dönmelidir. Kodlu metnin bir yazım
diline çevrilmesinde gösterim o dilde geri okunur ve aynı kodlu biçimi vermelidir. Sözdizimi hatası, bozuk
UTF-8 ya da kayıp durumunda hata döner ve dosya yazılmaz. Depodaki bütün `.tr` dosyaları ve
belgelerdeki kod blokları bu denetimden geçmelidir. Denetim: `TestDecodeRefusesAnUnreadableFile`, `TestEscapeInOtherLanguage`, `TestRepoTRFilesEncodeLossless`, `TestDocCodeBlocksEncodeLossless`.

**Dosyanın dili yoktur.** Dosya `# TR 4` başlığıyla başlar ve kimliklerle (`TR_print`) yazılır (dil
tanımı 2.1); program her zaman kimliklerle okunur ve çalışır, başlıksız metin çalışmaz (`NoHeaderError`,
ileti kodlama komutunu söyler). Hiçbir yazım dilinin ayrıcalığı yoktur; değerin metni dilden bağımsızdır
(`codes.ValueNames`). Kişinin yazım dili (`TR_DIL`, `codes.Display`) yalnızca görüntüdür. Bir dilde yazılmış
metin dili açıkça verilerek okunur (`lexer.NewBodyIn`, `EncodeIn`, `Show`): `tr kodla - --dil en`, kişinin
dilinde yazılmış kabuk satırı (`Options.Typed`), başlıksız kodlu parça (`Options.Body`). Denetim:
`TestHeaderRules`, `TestHeaderlessFileDoesNotRun`, `TestUserNamesNeverBecomeLanguageWords`,
`TestDictMemberKeysDoNotDependOnTheLanguage` (cmd/tr/e2e), `TestBodyInLanguage` (lexer), `TestContextWordsFollowLexer` (parser).

**Satırlar dosyanın satırlarıdır.** Başlık ve `#!` satırı dahil, 1. satır dosyanın ilk satırıdır; gösterim
başlığı ve satırları tutar. Denetim: `TestCodedFilePrepares`, `TestEncodeKeepsLineStructure`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_single_reader_test.go`: kodlu biçim başlığı kurallarının yalnızca `source` ve `lexer` paketlerinde olduğu (öbür paketler dosyayı `source` üzerinden okur)
