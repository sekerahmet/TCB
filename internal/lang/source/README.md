# source

Kaynaktan programa giriş noktası: bir `.tr` dosyasını bölümlerine ayırır (BOM, kodlu biçim
başlığı, gövde), kimlik okuyucusunu kurar, ayrıştırır ve istenirse denetimden geçirir. Kodlu
dosyanın bir yazım dilindeki görünümünü (`Show`) yazan yol da buradadır; görünüm yalnız okunur.

## Standartlar

**Dosyalar tek yerden okunur.** Komut satırı, etkileşimli kabuk, test koşucusu, hata ayıklayıcı ve
modül yükleyici bir `.tr` dosyasını yalnızca bu paket üzerinden okur; başlık kuralları başka yerde
yinelenmez. Denetim: `TestHeaderRulesLiveOnlyInSourceAndLexer`.

**Başlık kesindir.** İlk satır (ilk satır `#!` ise ikinci satır) tam olarak `# TR 4` ise dosyadır.
Başlığa benzeyen ama tanınmayan satır (başka sürüm) hatadır; başlıksız metin çalışmaz, sessizce bir
dilde yazılmış sayılmaz. Denetim: `TestHeaderRules`.

**Kod yalnız kimliklerle yazılır, görünüm yalnız okunur (17.7).** Dosya, kabuğa yazılan satır
(`Options.Body`) ve hata ayıklayıcının ifadesi (`TypedLexer`) kimliklerle (`TR_print`) okunur; bir yazım
dilinin sözcüğü (`yazdır`) kullanıcının adıdır. Görünüm (`# TR 4 view=tr`, `TR_cmd_show`'un çıktısı)
çalışmaz ve koda geri çevrilmez; ileti kodlu dosyayı çalıştırmayı söyler (`ViewError`). Başlıksız gövde
de kimlik yazımıyla okunur (`Parts.Lexer`). Denetim: `TestViewHeader`, `TestTypedLineIsReadWithIdentities`,
`TestLexerReadsIdentitiesAtDiskPositions`, `TestShowCommand` ve `TestHeaderlessFileDoesNotRun` (cmd/tr/e2e),
`TestCLIExpressionIsWrittenWithIdentities` (debugger).

**Görünüm yalnız dilin sözcüklerini çevirir.** Boşluklar, satır sonları, yorumlar, kullanıcının adları,
metinler ve sayılar korunur; yorumların ve ham metinlerin içi çevrilmez, metin içi yerleştirmedeki
sözcükler çevrilir. Görünüm dilin bir sözcüğüyle aynı yazılan kullanıcı adını `$ad` diye işaretleyebilir;
bu bir kural değildir, okuyanı uyarır. Sözdizimi hatası ya da bozuk UTF-8 varsa görünüm yazılmaz.
Depodaki bütün kodlu `.tr` dosyaları ve belgelerdeki kod blokları her gömülü dilde gösterilir. Denetim:
`TestShowTranslatesOnlyTheLanguageWords`, `TestShowKeepsBOMAndCRLF`, `TestShowMarksUserNamesThatReadAsWords`,
`TestShowRefusesAnUnreadableFile`, `TestRepoTRFilesShow`, `TestDocCodeBlocksShow`, `FuzzShow`.

**Dosyanın dili yoktur.** Dosya `# TR 4` başlığıyla başlar ve kimliklerle (`TR_print`) yazılır (dil
tanımı 2.1); program her zaman kimliklerle okunur ve çalışır, başlıksız metin çalışmaz (`NoHeaderError`,
ileti başlığı ve kimlik yazımını söyler). Hiçbir yazım dilinin ayrıcalığı yoktur; değerin metni dilden
bağımsızdır (`codes.ValueNames`). Kişinin yazım dili (`TR_DIL`, `codes.Display`) yalnızca görüntüdür.
Denetim: `TestHeaderRules`, `TestHeaderlessFileDoesNotRun`, `TestUserNamesNeverBecomeLanguageWords`,
`TestDictMemberKeysDoNotDependOnTheLanguage` (cmd/tr/e2e).

**Satırlar dosyanın satırlarıdır.** Başlık ve `#!` satırı dahil, 1. satır dosyanın ilk satırıdır; görünüm
başlığı ve satırları tutar. Denetim: `TestCodedFilePrepares`, `TestShowTranslatesOnlyTheLanguageWords`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_single_reader_test.go`: kodlu biçim başlığı kurallarının yalnızca `source` ve `lexer` paketlerinde olduğu (öbür paketler dosyayı `source` üzerinden okur)
