# codes

Dilin kullanıcıya görünen her adını (anahtar kelime, yerleşik, tip, üye, adlı argüman) değişmez
bir koda bağlar: `TR_fn_print` → `yazdır`. Adlar dil dosyalarında (`tr.json`, `en.json` ...), dilden
bağımsız tanımlar (tür, imza, dönüş tipi) tanım dosyasındadır.

## Standartlar

**Kod değişmez, ad değişebilir.** Kodun dosyalardaki yazımı kimliğidir: `TR_<sınıf>_<İngilizce ad>` (`TR_kw_if`, `TR_fn_print`, `TR_mem_step`); kimlik kod açılırken bir kez verilir, adlar (İngilizce dahil) değişse de değişmez. Sınıf (`kw`, `ctx`, `val`, `type`, `fn`, `const`, `arg`, `mem`, `cmd`, `sub`, `opt`, `repl`, `dbg`, `env`) kodun ad alanını ve türünü verir; ikilideki sayı yalnızca sıradır, dışarı çıkmaz. `TR_` ile başlayan adlar ayrılmıştır (2.3a). Bir adı değiştirmek yalnızca dil dosyasındaki satırı değiştirir. Dil taslak aşamasındadır: değişen ya da kaldırılan adların kaydı tutulmaz, kullanımlar doğrudan yeni ada çevrilir (bkz. docs/spec/17_kodlar.md). Eski kodlar (`A001`) ve kimlikleri: `docs/code_identities.md`. Denetim: `TestIdentities`, `TestIdentityClassesKnown`, `TestTRSortedOnePerLine`.

**`tr dizin` alt komutları kendi ad alanındadır.** `sub` sınıfının kodları (`tanım`, `kullananlar`, `sınayanlar`, `etki`, `yeniden_adlandır`) `NamespaceSubcommand` ad alanındadır: `tr tanım` bir komut değildir, `tr dizin tanım` çalışır; her gömülü dilin adı alt komut olarak kabul edilir (`FindSubcommand`). Denetim: `TestIndexSubcommandsAreNotCommands`.

**Her işin tek adı vardır.** Bir dilde eş anlamlı ya da ikinci bir yazım kabul edilmez; yabancı
adlar (başka dillerden alışkanlık: `lr`, `axis`) yalnızca hata mesajında doğru adı önermek için
tutulur. Türkçe karakterli adların ASCII biçimi kabul edilmez. Denetim: `TestSingleSpelling`, `TestFindForeign`, `TestASCIINotAcceptedSuggestsTurkish`.

**Ad, kendi ad alanında benzersizdir.** Genel adlar, tip adları, üyeler ve adlı argümanlar ayrı
ad alanlarıdır; çakışma dil dosyası yüklenirken hata verir. Denetim: `TestNamespacesUnique`, `TestClashesCaught`.

**Dil dosyası dile bağlı olanı içerir.** Ad, yabancı adlar, açıklama ve örnek dil dosyasındadır; imzalar ve türler dilden bağımsızdır ve tanım dosyasında bir
kez yazılır. Her dil dosyası aynı kodlara kendi adlarını verir. Denetim: `TestWritingLanguagesComplete`, `TestParserMatchesJSON`, `TestDefinitionsConsistent`.

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
