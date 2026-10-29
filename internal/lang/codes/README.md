# codes

Dilin kullanıcıya görünen her adını (anahtar kelime, yerleşik, tip, üye, adlı argüman) değişmez
bir koda bağlar: `yazdır` → `TR_fn_print`. Adlar dil dosyalarında (`tr.json`, `en.json` ...), dilden
bağımsız tanımlar (tür, imza, dönüş tipi) tanım dosyasındadır.

## Standartlar

**Kod değişmez, ad değişebilir.** Kodun dosyalardaki yazımı kimliğidir: `TR_<sınıf>_<İngilizce ad>` (`TR_kw_if`, `TR_fn_print`, `TR_mem_step`); kimlik kod açılırken bir kez verilir, adlar (İngilizce dahil) değişse de değişmez; iki kimlik yalnızca harf büyüklüğüyle ayrılamaz (çakışırsa sahibin adı eklenir: `TR_arg_G_muon`). Uluslararası gösterimle ya da özel adla yazılan kavramın (`sin`, `exp`, `AdamW`, `json`) **ortak adı** vardır: `definitions.json`'da `common_name` olarak bir kez yazılır, hiçbir dil dosyasında yazılmaz; `Load` onu her dile, dilin kendi adlarıyla aynı denetimlerden geçirerek verir, ortak adlı koda dil dosyasında ad vermek yükleme hatasıdır. Standart kitaplığın (`tr/…`) dışa açık bir adı **kitaplık kodudur**: `definitions.json` satırında yalnız türü ve modülü (`"module": "tr/layers"`) yazılır, imzası ve tipleri kitaplığın kaynağındadır; kitaplık kodu yerleşik değildir (`Code.IsBuiltin` yanlış, `Code.IsLibrary` doğru), adı aktarımla gelir. Modül tablosu `codes_generated.go`'ya üretilir; çalışma anı `definitions.json`'u ayrıştırmaz. Sınıf (`kw`, `ctx`, `val`, `type`, `fn`, `const`, `arg`, `mem`, `cmd`, `sub`, `opt`, `repl`, `dbg`, `env`) kodun ad alanını ve türünü verir; ikilideki sayı yalnızca sıradır, dışarı çıkmaz. `TR_` ile başlayan adlar ayrılmıştır (2.3a). Bir adı değiştirmek yalnızca dil dosyasındaki satırı değiştirir. Dil taslak aşamasındadır: değişen ya da kaldırılan adların kaydı tutulmaz, kullanımlar doğrudan yeni ada çevrilir (bkz. docs/spec/17_kodlar.md). Eski kodlar (`A001`) ve kimlikleri: `docs/code_identities.md`. Denetim: `TestIdentities`, `TestIdentityClassesKnown`, `TestIdentitiesDifferBeyondLetterCase`, `TestCommonNamesGoThroughTheChecks`, `TestCommonNamesAreNotInLanguageFiles`, `TestLibraryCodesAreNotBuiltins`, `TestLibraryCodeHasNoSignatureInDefinitions`, `TestTRSortedOnePerLine`.

**`TR_cmd_index` alt komutları kendi ad alanındadır.** `sub` sınıfının kodları (`TR_sub_definition`, `TR_sub_references`, `TR_sub_tested_by`, `TR_sub_impact`, `TR_sub_rename`) `NamespaceSubcommand` ad alanındadır: `tr tanım` bir komut değildir, `TR_cmd_index TR_sub_definition` çalışır; her gömülü dilin adı alt komut olarak kabul edilir (`FindSubcommand`). Denetim: `TestIndexSubcommandsAreNotCommands`.

**Her işin tek adı vardır.** Bir dilde eş anlamlı ya da ikinci bir yazım kabul edilmez; yabancı
adlar (başka dillerden alışkanlık: `lr`, `axis`) yalnızca hata mesajında doğru adı önermek için
tutulur. Türkçe karakterli adların ASCII biçimi kabul edilmez. Denetim: `TestSingleSpelling`, `TestFindForeign`, `TestASCIINotAcceptedSuggestsTurkish`.

**Ad, kendi ad alanında benzersizdir.** Genel adlar, tip adları, üyeler ve adlı argümanlar ayrı
ad alanlarıdır; çakışma dil dosyası yüklenirken hata verir. Denetim: `TestNamespacesUnique`, `TestClashesCaught`.

**Dil dosyası dile bağlı olanı içerir.** Ad, yabancı adlar ve açıklama dil dosyasındadır; imzalar, türler ve örnek (kimlik yazımıyla, 17.4) dilden bağımsızdır ve tanım dosyasında bir
kez yazılır. Her dil dosyası aynı kodlara kendi adlarını verir. Denetim: `TestWritingLanguagesComplete`, `TestParserMatchesJSON`, `TestDefinitionsConsistent`.

**Dosya biçiminin anahtarları İngilizce tanımlayıcıdır.** Tanım ve dil dosyalarının alan adları (`kind`, `params`, `named`, `returns`, `related`, `common_name`, `module`, `types`, `signatures`, `example`; `name`, `foreign`, `family`, `description`) ve `kind` değerleri (`keyword`, `function`, `method` ...) Go adları gibi İngilizce yazılır; bir insan dili yalnız değerlerde (adlar, açıklamalar) görünür; örnek kimlik yazımıyla tanım dosyasındadır. Denetim: `TestFileFieldNamesAreEnglishIdentifiers`.

**Kurulamayan dil listesi süreci kapatmaz.** Listeler paket başlatılırken kurulur (sözcük çözümleyicinin ve denetleyicinin tabloları onlardan kurulur). Geçersiz bir `TR_DIL` Türkçeye düşer; gömülü Türkçe listenin kendisi okunamazsa ya da bir kodu eksik taşırsa (bozuk ya da yarım değişmiş `tr.json`) adlar kimliklere düşer. Hata `LanguageError` ile, Türkçeyi isteyene `LoadLanguage` ile verilir: `tr` başlarken iç hatayla çıkar, codegraph Go grafiğini kurar ve TR cevaplarının eksik olduğunu söyler. Denetim: `TestABrokenTurkishListFallsBackToIdentities`, `TestAnInvalidDisplayLanguageKeepsTheTurkishList`, `TestInvalidLanguageFallsBack`, `TestABrokenLanguageFileIsAnInternalError` (cmd/tr/e2e), `TestABrokenLanguageFileLeavesTheGoAnswers` (tools/codegraph).

**Go kodu kodu yazar, adı yazmaz.** Go kodu kodları kimlikten türeyen üretilmiş sabitlerle yazar (`TR_fn_print` → `codes.FnPrint`);
sözlükte olmayan bir kod derlemeyi düşürür, kodun sayısı (`codes.Code(57)`) yazılmaz. Sabitin yorumu kimlikle başlar, okuyana yardım için kodun
İngilizce ve Türkçe adını gösterir (`// TR_fn_print: print · yazdır`); başka diller yoruma girmez. Denetim: `TestNoBareCodes`, `TestConstantsUpToDate` (tools/gen/codeconsts).

**Kod listesi değişince yeniden üretilir.** Kod listesinden üretilen dosyalar (Go sabitleri, kod
belgesi, düzenleyicinin renklendirme listesi) şu durumlarda yeniden üretilir: bir kod eklendiğinde ya da
silindiğinde, bir ad değiştiğinde ve yeni bir dil eklendiğinde
(`go generate ./internal/lang/codes`). Üretilmiş bir dosya güncel değilse denetim düşer; unutulan
üretim sessizce eskimez. Denetim: `TestConstantsUpToDate` (tools/gen/codeconsts), `TestDocUpToDate` (tools/gen/codesdoc), `TestGrammarUpToDate` (tools/gen/grammar).

**Dosyalar düzenlidir.** Her girdi tek satırdır; kodlar sınıf sırasıyla, sınıf içinde kimliğe göre dizilir; her kodun açıklaması ve
örneği vardır. Denetim: `TestDefinitionsSortedOnePerLine`, `TestTRSortedOnePerLine`, `TestEveryCodeHasDescription`.
