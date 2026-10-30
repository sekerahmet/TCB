# messages

Kullanıcıya giden her iletiyi (hata, uyarı, bilgi) değişmez bir koda bağlar: `H7003` →
`{fn}: {path|q} bir klasör; CSV dosyasının yolunu verin`. Adlar için `internal/lang/codes` ne ise
iletiler için bu pakettir.

## Standartlar

**İleti yanlışı söyler ve çalışan doğru yazımı verir.** İletiyi en çok bir yapay zekâ ajanı okur ve önerisini olduğu gibi kopyalar. Bu yüzden ileti yanlış olanı adıyla ve yerinde gösterir. Bir yazım önerirse, o yazım kopyalanınca hata vermeden çalışır ve istenen sonucu verir. Öneri dilin kurallarıyla (öncelik, tip, kapsam) denenir; testi önerilen yazımı da koşar. Önerinin yanlış sonuç vermesi, hiç öneri vermemekten kötüdür: ajan onu kontrol etmeden kullanır. Örnekler (2. katmanın denetimi, 2026-10-28):
- `(-2) ** -2` (H2603): `-2.0 ** -2` önerisi 4.1'e göre `-(2.0 ** -2)` okunur ve −0.25 verir; doğru öneri `(-2.0) ** -2`, 0.25.
- `a == 2 < 3` (H0200): hata zincirde geçerli olan `<`'yi değil, zincire giremeyen `==`'yi gösterir.
- `[1] + 2` (H2614): metin yerleştirmesi değil, listenin yazımı önerilir: `l + [x]` ya da `l.TR_append(x)`.

Denetim: iletiyi yazan paketin testi önerinin metnini, dilin testi önerilen yazımın sonucunu sınar (`TestNegativeIntExponentSuggestsAWorkingSpelling`, `TestListPlusSuggestsTheListSpelling`, `TestChainAndCoalescingErrors`).

**Kod değişmez, metin değişebilir.** Kod `H` ve dört basamaktır; binler basamağı alanı gösterir:
`H0` sözdizimi · `H1` tip denetimi · `H2` yorumlayıcı (ortak `H2000` = `{fn}: {error}`) · `H3`
tensör ve CPU · `H4` GPU · `H5` veri ve parçalayıcı · `H6` stdlib/ml · `H7` standart kitaplığın
geri kalanı · `H8` araçlar ve `tr` komutu · `H9` iç hatalar. Silinen bir kodun numarası bir daha
kullanılmaz: silinenler `retired.json`'dadır (numara → commit ve gerekçe; ilk liste `go run ./tools/codegraph deleted --retired-json` ile üretildi), katalogda ya da tanımlarda yeniden bulunursa üreteç çalışmaz. Denetim: `TestParseCode`, `TestCatalogErrors`, `TestMessageUsage` (tools/gen/msgconsts: kullanılmayan kod), `TestRetiredCodesStayRetired`, `TestGenerateRejectsRetiredCode` (tools/gen/msgconsts).

**Go kodu metin yazmaz, kod yazar.** `messages.New(messages.H7003, fn, path)` bir hata,
`messages.Text(...)` hata olmayan çıktı (uyarı, not) verir. Değerler tanımdaki parametre sırasıyla
verilir; metin hata yazılırken etkin dilde kurulur. İleti ve içindeki dil adları tek dildedir:
ad iletiye kodla verilir (`codes.NameRef`, `interp.CallName`, `bind.Named`; denetleyicide
`typeText`), katalog onu iletinin dilinde yazar; `codes.Name`, `codes.DisplayName`
ya da `.TR` ile kurulmuş ad ne doğrudan, ne bir değişkenle, ne de adı iletiye ileten bir fonksiyonun parametresiyle verilir (`TestNoFixedNamesInMessages`). Sabitler üretilir
(`go run ./tools/gen/msgconsts`); katalogda olmayan kod derlemeyi düşürür. Denetim: `TestMessageUsage`, `TestGeneratedUpToDate` (tools/gen/msgconsts).

**Dosyalar.** `definitions.json` dilden bağımsızdır: her kodun parametre adları
(`"H7003": { "param": ["fn", "path"] }`; adlar İngilizce tanımlayıcıdır). Her yazım dilinin bir dosyası vardır
(`tr.json`, `en.json` ...); `tr.json` her kodun metnini tutar ve bugün yedektir: başka bir dil dosyası çevirisi olmayan kodu Türkçe
gösterir (hiçbir yazım dilinin ayrıcalığı olmaması ilkesine aykırı geçici istisna). Etkin dil `TR_LANG` ortam değişkenidir (adlarla aynı: `--lang en`); dosyadan tahmin edilmez.
Her kod bir satırdır, kodlar artan sıradadır. Denetim: `TestEmbeddedCatalogValid`, `TestFilesOneCodePerLine`, `TestDefinitionParamsAreASCIIIdentifiers`, `TestUnknownLanguageFallsBack`.

**Şablon** (ICU MessageFormat'ın alt kümesi; çevirmen araçları tanır):

| Yazım | Anlamı |
| :-- | :-- |
| `{ad}` | değer olduğu gibi |
| `{ad\|q}` | Go tırnağıyla, denetim karakterleri kaçırılarak (kullanıcı verisi için) |
| `{ad\|decimal}` | sayının ondalık ayırıcısı iletinin dilinde (Go noktayla yazar; tr `1,5`, en `1.5`). İleti dışı görüntüde `messages.Decimal` |
| `{ad\|list}` | liste değeri (`messages.Listed`), dilin "ya da" kalıbıyla (`OrList`): tr "a, b ya da c", en "a, b, or c" |
| `{@ad}`, `{@ad\|dat}` | **ad yuvası**: tanımın `slots` tablosundaki kitaplık adı (kimlik, `TR_arg_kind`), iletinin dilindeki adıyla yazılır; hâl eki yuvaya da gelir. Değer değildir, `param` sayısına girmez; TR kodunda kitaplığın iletisi adı geçirmez, katalog anar (parametre ya da alan adı TR'de değer olarak geçirilemez, ad değişince ileti eskimez). `definitions.json`: `"H6900": { "param": [...], "slots": {"num_heads": "TR_arg_num_heads"} }`. Her dil tanımdaki bütün yuvaları kullanır |
| `{#ad}` | **kod yuvası**: tanımın `slots` tablosundaki kitaplık adını programdaki kimlik yazımıyla (`TR_length`, `TR_print`) yazar; kimlik çekimlenmediği için hâl eki almaz. `{` ile başladığından ICU çoğul dalındaki yalın `#` sayısıyla karışmaz |
| `{ad\|acc}`, `{ad\|dat}`, `{ad\|loc}`, `{ad\|abl}`, `{ad\|gen}` | hâl eki: dilin çekimcisi ekler (`Satır {line\|dat}` → "Satır 12'ye"); çekimcisi olmayan dilde ad olduğu gibi kalır |
| `{n, plural, one {..} other {..}}` | sayıya göre biçim, dilin CLDR çoğul kuralıyla; dalda (içindeki `select` dallarında da) `#` sayıdır, `=0` gibi tam eşleşme önce gelir (N sayının tek yazımı: `=0.0` reddedilir); `other` zorunlu |
| `{x, select, a {..} other {..}}` | metin değerine göre dal; boş değer `empty` dalını seçer |
| `{{`, `}}` | düz süslü parantez (dal içinde `}` dalı kapatır) |

Her dilin metni kodun **bütün** parametrelerini kullanır (çeviride düşen değer kullanıcıya ulaşmazdı);
bir parametre ya dalların dışında ya `plural`/`select`'in her dalında geçer (yalnız bir dalda
geçen değer öbür dal seçilince kaybolurdu). Sözcük sırası dilden dile değişebilir, parametreler
adla anıldığı için serbesttir. Denetim: `TestRender`, `TestPlural`, `TestSelect`, `TestCaseSuffix`, `TestTemplateErrors`, `TestCatalogErrors`, `TestParamUsedInEveryBranch`, `TestNumberSignInNestedSelect`, `TestNameSlot`, `TestNameSlotErrors`, `TestListFormat`, `TestPluralNumberTypes`.

Kod örneği ters tırnak içinde yazılır; ters tırnak içinde ad yuvası `{#ad}`'dır, `{@ad}` yazılmaz; ters tırnak dışında `{#ad}` yazılmaz. Kendi satırında başlayan çok satırlı blok da bir kod örneğidir (ör. H1107'nin `{func} kur() { ... }` önerisi): ters tırnak almaz, içindeki adlar kimlikle yazılır.
Denetim: `TestCodeSlot`, `TestCodeSlotsInBackticks`, `TestCodeSlotsResolveToIdentities` (internal/lang/codes).

**Dilbilgisi şablondadır, Go'da değil.**
- Go kodu sayıya bakıp sözcük seçmez (`argümanı`/`argümanları`): şablonda `plural`. Denetim: `TestPlural`.
- Go kodu ada ek eklemez: şablonda `{ad|dat}`. `turkish.Suffix` yalnızca `internal/lang/turkish`'te çağrılır (denetim: `TestMessageUsage`); Türkçe çekimciyi o paket kaydeder (`messages.RegisterInflector`), `messages` başka bir TR paketini bilmez.
- Seçenek listesi Go'da birleştirilmez: `messages.OrList(a, b, c)` → "a, b ya da c" (CLDR liste kalıbı: ikili H2010, ayırıcı H2005, son H2006). Denetim: `TestOrList`.
- Cümle parçalardan kurulmaz: bir iletinin anlamı tek şablondadır. Go kodu bir iletinin metnini düzenlemez (`strings.Replace`, `TrimPrefix`; ayrıntı gerekiyorsa `messages.As(err).Args`), `+` ile birleştirmez ve bir ileti çağrısına başka bir `messages.Text(...)` ya da ondan türeyen değişken argüman vermez; değişen sözcük şablonda `select` ile seçilir (`{place, select, gpu {..} other {..}}`, Go yalnızca anahtarı verir). Üç istisna açık ölçüte bağlıdır:
  - **Ad öbeği:** kendi başına anlamlı, cümlede yalın duran (kendisi hâl/iyelik eki almayan) etiket (tür adı, "bir tam sayı", "3. girdi") değer olabilir. Yüklem, ek alan öbek, sözcük sırasını belirleyen parça olamaz. Kayıtlar testteki `nounPhraseUses`'ta gerekçesiyle durur ve yalnızca küçülür.
  - **İpucu yuvası:** ana iletinin sonuna eklenen, kendi başına tam cümle olan ipucu/öneri ("Bunu mu demek istediniz ..." H1104/H2479, tek kaynak `internal/lang/hint`) ana iletinin `hint` ya da `suggestion` adlı parametresine ileti olarak verilir. Öneri eki için tek desen budur; başka adlı parametreye ileti verilemez.
  - **Sunum:** `+`nın öbür tarafı yalnızca boşluk, simge (`❌`, `✗`), alan ayıracı (`·`, `•`) ya da satır sonu ise birleştirme sunumdur (bağımsız alanları dizer).

  İsteğe bağlı parça Go'da kurulmaz, şablonda yazılır: boş değer `select`'in `empty` dalını seçer (`{exit, select, empty {} other {; çıkmak için '{exit}' yazın}}`; sayı için `{n, plural, =0 {} other {..#..}}`). Yan yana duran bağımsız maddeler (her biri kendi başına tam bir ileti) liste olarak birleştirilebilir (`gemmPath`). Denetim: `TestGoCodeDoesNotEditMessageText`, `TestMessageArgsAreNotMessageFragments` (doğrudan argüman, değişkene alınıp geçirilen metin ve `+` birleştirmesi; `TestFragmentDetector` dedektörün kendisini sınar), `TestSelectEmpty`.
- Aynı anlam tek koddur: parametre adları farklı olsa da aynı metin iki kodda olamaz (`TestNoDuplicateTexts`; bağlamı farklı etiketler gerekçeyle `sameTextAllowed`'a yazılır).

**Yeni bir dil eklemek.**
1. `internal/lang/messages/<dil>.json` (ISO 639-1); `catalog.go`'da `//go:embed` ve `languageFiles`'a satır.
2. Dilin CLDR çoğul kuralı `template.go`'daki `pluralRules`'a. Kural CLDR işlenenlerini alır (`pluralOperands`: n, i, v, f; tam sayıda i kayıpsızdır, 2^53 üstünde de `i%10` doğru): Türkçe `n = 1`, İngilizce `i = 1 and v = 0` "one"; Rusça, Arapça gibi dillerde daha çok kategori vardır. Denetim: `TestPluralOperands`.
3. Hâl eki olan bir dilse çekimcisini kendi dilbilgisi paketinden `messages.RegisterInflector` ile kaydet.
4. `go test ./internal/lang/messages` her şablonu ve parametreyi denetler; çevrilmemiş kod Türkçeye düşer.

**Bir ileti tek dildedir.** Süreç dili `TR_LANG`'den bir kez okunur (`messages.ActiveLanguage`; `codes` da bunu kullanır, adlar ve iletiler aynı dildedir). Bir iletinin ya da içindeki kodlu hatalardan (`*Error`, `*PosError`) birinin etkin dilde metni yoksa iletinin bütünü yedek dilde (bugün Türkçe) gösterilir; iç ileti dıştakinin dilinde yazılır. Bu yüzden ileti değerine metin (`messages.Text`) değil kodlu hata (`messages.New`) geçirilir. Denetim: `TestFallbackKeepsOneLanguage`, `TestLanguageReadOnce`.

İngilizce: `en.json` her kodun metnini taşır; `TR_LANG=en` ile gösterilir. `en.json`'da olmayan kod hatadır; yeni kod `tr.json`'la birlikte `en.json`'a da yazılır. Denetim: `TestEnglishCoversEveryCode`.

**TR kodu da ileti kurar.** `lib/*.tr` hata metni yazmaz: `TR_throw TR_Message("H6903", pencere)` kodu (bir metin değeri) ve değerleri taşır, metin hata gösterilirken bu katalogdan kurulur (dil tanımı 8.5). `ParamCount` `TR_Message` çağrısını tanımla denetler (tanımsız kod H2528, yanlış değer sayısı H2529). Kitaplığın kendi adları ad yuvasıdır (`{@ad}`, yukarıda). Denetim: `TestParamCount`, `TestMessageUsage` (tools/gen/msgconsts: `.tr` çağrıları), `TestMessageSlotsAreNames` (internal/lang/codes).

**Testler metni değil kodu denetler.** `messages.Has(err, messages.H7003)` hata zincirinde kodu
arar; metin değişince test kırılmaz.

**İç hata.** Yalnızca TR'nin kendi hatasıyla oluşabilen durum `messages.Internal("ayrıntı")` ile
bildirilir: kullanıcı tek ortak iletiyi (`H9000`, "lütfen bildirin") görür, ayrıntı geliştirici
içindir ve İngilizce yazılır. Denetim: `TestInternal`, `TestInternalfAndHasAny`.

**Denetim.** `go test ./tools/gen/msgconsts` her çağrının değer sayısını tanımla karşılaştırır,
kullanılmayan kodu bildirir ve ileti kodlarına geçirilmiş paketlerde (`migrated` listesi)
`fmt.Errorf` ile `errors.New` kullanımını yasaklar.

## Geçiş

Paket paket yapılır; geçirilen paket `tools/gen/msgconsts/main_test.go`'daki `migrated` listesine
eklenir. Önce ortak yardımcılar geçirildi (tamam): dosya hataları (`turkish.FileError` işlemi bir
kod olarak alır: `messages.H2100` "okunamadı"; nedenler kodludur, `turkish.NewReason`), argüman
sayısı, satır öneki (`interp.LineError`, `H2001`), adlı argüman ve üye hataları (`bind`). Sonra:
`formats` → standart kitaplığın geri kalanı → `lang` → `devtools` → `compute` (hepsi tamam; `compute`
yalnızca bu paketi içe aktarır, bkz. `internal/layers_test.go`). Satır önekine
metinden bakan denetim (`interp.hasLine`, test koşucusunun `located`ı) kalktı: konum yalnızca yapıdan
(`messages.Pos`) okunur. Ayrıştırıcı ve denetleyici hataları da yapıyla taşınır (`source.Result.Errors`,
`interp.ModuleError.Errors`, `formatter.SyntaxError` `[]error` tutar; konum `messages.PosError`'dadır);
`turkish.ErrorFrame` konumu yalnızca yapıdan okur, metin yedeği yoktur.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_message_text_test.go`: Go kodunun bir iletinin metnini düzenlemediği (`strings.Replace`, `TrimPrefix` ... doğrudan ileti metniyle çağrılmaz)
- `behavior_message_fragments_test.go`: bir ileti çağrısına başka bir `messages.Text(...)` parça olarak verilmez (cümle tek şablondur)
- `behavior_localized_errors_test.go`: `Error()` uygulayan her hata türü `MessageText(lang)` de uygular (`messages.Localized`); başka dilde (`ErrorIn`, yakalanan hata için `ValueLanguage`) kurulan ileti bütünüyle o dilde yazılır, katalog hiçbir hatayı `TR_LANG` metnine düşürmez
- `behavior_english_complete_test.go`: tr.json'daki her ileti kodunun en.json'da İngilizce metni vardır
