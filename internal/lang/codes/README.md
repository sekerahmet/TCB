# codes

Dilin kullanıcıya görünen her adını (anahtar kelime, yerleşik, tip, üye, adlı argüman) değişmez
bir koda bağlar: `G100` → `yazdır`. Adlar dil dosyalarında (`tr.json`, `en.json` ...), dilden
bağımsız tanımlar (tür, imza, dönüş tipi) tanım dosyasındadır.

## Standartlar

**Kod değişmez, ad değişebilir.** Kod bir harf ve üç rakamdır (`A001`, `G100`, `Y400`); harf
aileyi, yüzler basamağı grubu gösterir. Bir adı değiştirmek yalnızca dil dosyasındaki satırı
değiştirir. Kaldırılan ya da taşınan bir kodun numarası bir daha kullanılmaz. Denetim: `TestIsCodeForm`, `TestMovedCodes`, `TestTRSortedOnePerLine`.

**`tr dizin` alt komutları kendi ad alanındadır.** K016–K020 (`tanım`, `kullananlar`, `sınayanlar`, `etki`, `yeniden_adlandır`) komut numaralamasındadır ama `NamespaceSubcommand` ad alanındadır: `tr tanım` bir komut değildir, `tr dizin tanım` çalışır; her gömülü dilin adı alt komut olarak kabul edilir (`FindSubcommand`). Denetim: `TestIndexSubcommandsAreNotCommands`.

**Her işin tek adı vardır.** Bir dilde eş anlamlı ya da ikinci bir yazım kabul edilmez; eski ve
yabancı adlar yalnızca hata mesajında doğru adı önermek için tutulur. Türkçe karakterli adların
ASCII biçimi kabul edilmez. Denetim: `TestSingleSpelling`, `TestAltSpellingsRemoved`, `TestASCIINotAcceptedSuggestsTurkish`.

**Ad, kendi ad alanında benzersizdir.** Genel adlar, tip adları, üyeler ve adlı argümanlar ayrı
ad alanlarıdır; çakışma dil dosyası yüklenirken hata verir. Denetim: `TestNamespacesUnique`, `TestClashesCaught`.

**Dil dosyası dile bağlı olanı içerir.** Ad, eski ve yabancı adlar, açıklama ve örnek dil dosyasındadır; imzalar ve türler dilden bağımsızdır ve tanım dosyasında bir
kez yazılır. Her dil dosyası aynı kodlara kendi adlarını verir. Kaynak dil Türkçedir: ölü kodların
adları ipucu için Türkçe dosyada kalır, öbür dillerde isteğe bağlıdır. Denetim: `TestWritingLanguagesComplete`, `TestDeadCodesOptionalOutsideTR`, `TestParserMatchesJSON`, `TestDefinitionsConsistent`.

**Go kodu kodu yazar, adı yazmaz.** Go kodu kodları üretilmiş sabitlerle yazar (`codes.G100`);
sözlükte olmayan bir kod derlemeyi düşürür. Sabitin yanındaki yorum, okuyana yardım için kodun
İngilizce ve Türkçe adını gösterir (`// print · yazdır`); başka diller yoruma girmez. Denetim: `TestNoBareCodes`, `TestNoBareCodes`, `TestConstantsUpToDate` (tools/gen/codeconsts).

**Kod listesi değişince yeniden üretilir.** Kod listesinden üretilen dosyalar (Go sabitleri, kod
belgesi, düzenleyicinin renklendirme listesi) şu durumlarda yeniden üretilir: bir kod eklendiğinde,
kaldırıldığında ya da taşındığında, bir ad değiştiğinde ve yeni bir dil eklendiğinde
(`go generate ./internal/lang/codes`). Üretilmiş bir dosya güncel değilse denetim düşer; unutulan
üretim sessizce eskimez. Denetim: `TestConstantsUpToDate` (tools/gen/codeconsts), `TestDocUpToDate` (tools/gen/codesdoc), `TestGrammarUpToDate` (tools/gen/grammar).

**Dosyalar düzenlidir.** Her girdi tek satırdır, kodlar artan sıradadır; her kodun açıklaması ve
örneği vardır. Denetim: `TestDefinitionsSortedOnePerLine`, `TestTRSortedOnePerLine`, `TestEveryCodeHasDescription`.
