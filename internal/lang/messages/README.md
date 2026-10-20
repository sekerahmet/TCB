# messages

Kullanıcıya giden her iletiyi (hata, uyarı, bilgi) değişmez bir koda bağlar: `H7003` →
`{fn}: {path|q} bir klasör; CSV dosyasının yolunu verin`. Adlar için `internal/lang/codes` ne ise
iletiler için bu pakettir.

## Standartlar

**Kod değişmez, metin değişebilir.** Kod `H` ve dört basamaktır; binler basamağı alanı gösterir:
`H0` sözdizimi · `H1` tip denetimi · `H2` yorumlayıcı (ortak `H2000` = `{fn}: {error}`) · `H3`
tensör ve CPU · `H4` GPU · `H5` veri ve parçalayıcı · `H6` stdlib/ml · `H7` standart kitaplığın
geri kalanı · `H8` araçlar ve `tr` komutu · `H9` iç hatalar. Silinen bir kodun numarası bir daha
kullanılmaz: silinenler `retired.json`'dadır (numara → commit ve gerekçe; ilk liste `go run ./tools/codegraph deleted --retired-json` ile üretildi), katalogda ya da tanımlarda yeniden bulunursa üreteç çalışmaz. Denetim: `TestParseCode`, `TestCatalogErrors`, `TestMessageUsage` (tools/gen/msgconsts: kullanılmayan kod), `TestRetiredCodesStayRetired`, `TestGenerateRejectsRetiredCode` (tools/gen/msgconsts).

**Go kodu metin yazmaz, kod yazar.** `messages.New(messages.H7003, fn, path)` bir hata,
`messages.Text(...)` hata olmayan çıktı (uyarı, not) verir. Değerler tanımdaki parametre sırasıyla
verilir; metin hata yazılırken etkin dilde kurulur. Sabitler üretilir
(`go run ./tools/gen/msgconsts`); katalogda olmayan kod derlemeyi düşürür. Denetim: `TestMessageUsage`, `TestGeneratedUpToDate` (tools/gen/msgconsts).

**Dosyalar.** `definitions.json` dilden bağımsızdır: her kodun parametre adları
(`"H7003": { "param": ["fn", "path"] }`; adlar İngilizce tanımlayıcıdır). `tr.json` kaynak dildir
ve her kodun metnini tutar; başka bir dil dosyası (`en.json` ...) çevirisi olmayan kodu Türkçe
gösterir. Etkin dil `TR_DIL` ortam değişkenidir (adlarla aynı: `--dil en`); dosyadan tahmin edilmez.
Her kod bir satırdır, kodlar artan sıradadır. Denetim: `TestEmbeddedCatalogValid`, `TestFilesOneCodePerLine`, `TestDefinitionParamsAreASCIIIdentifiers`, `TestUnknownLanguageFallsBack`.

**Şablon** (ICU MessageFormat'ın alt kümesi; çevirmen araçları tanır):

| Yazım | Anlamı |
| :-- | :-- |
| `{ad}` | değer olduğu gibi |
| `{ad\|q}` | Go tırnağıyla, denetim karakterleri kaçırılarak (kullanıcı verisi için) |
| `{ad\|acc}`, `{ad\|dat}`, `{ad\|loc}`, `{ad\|abl}`, `{ad\|gen}` | hâl eki: dilin çekimcisi ekler (`Satır {line\|dat}` → "Satır 12'ye"); çekimcisi olmayan dilde ad olduğu gibi kalır |
| `{n, plural, one {..} other {..}}` | sayıya göre biçim, dilin CLDR çoğul kuralıyla; dalda (içindeki `select` dallarında da) `#` sayıdır, `=0` gibi tam eşleşme önce gelir (N sayının tek yazımı: `=0.0` reddedilir); `other` zorunlu |
| `{x, select, a {..} other {..}}` | metin değerine göre dal |
| `{{`, `}}` | düz süslü parantez (dal içinde `}` dalı kapatır) |

Her dilin metni kodun **bütün** parametrelerini kullanır (çeviride düşen değer kullanıcıya ulaşmazdı);
bir parametre ya dalların dışında ya `plural`/`select`'in her dalında geçer (yalnız bir dalda
geçen değer öbür dal seçilince kaybolurdu). Sözcük sırası dilden dile değişebilir, parametreler
adla anıldığı için serbesttir. Denetim: `TestRender`, `TestPlural`, `TestSelect`, `TestCaseSuffix`, `TestTemplateErrors`, `TestCatalogErrors`, `TestParamUsedInEveryBranch`, `TestNumberSignInNestedSelect`, `TestPluralNumberTypes`.

**Dilbilgisi şablondadır, Go'da değil.**
- Go kodu sayıya bakıp sözcük seçmez (`argümanı`/`argümanları`): şablonda `plural`. Denetim: `TestPlural`.
- Go kodu ada ek eklemez: şablonda `{ad|dat}`. `turkish.Suffix` yalnızca `internal/lang/turkish`'te çağrılır (denetim: `TestMessageUsage`); Türkçe çekimciyi o paket kaydeder (`messages.RegisterInflector`), `messages` başka bir TR paketini bilmez.
- Seçenek listesi Go'da birleştirilmez: `messages.OrList(a, b, c)` → "a, b ya da c" (CLDR liste kalıbı: ikili H2010, ayırıcı H2005, son H2006). Denetim: `TestOrList`.
- Cümle parçalardan kurulmaz: bir iletinin anlamı tek şablondadır. Kendi başına anlamlı bir ad öbeği (tür adı, "bir tam sayı", sona eklenen ipucu) değer olabilir; Go kodu bir iletinin metnini düzenlemez (`strings.Replace`, `TrimPrefix`); ayrıntı gerekiyorsa parametresinden okunur (`messages.As(err).Args`). Bir ileti çağrısına başka bir `messages.Text(...)` argüman olmaz; değişen sözcük şablonda `select` ile seçilir (`{place, select, gpu {..} other {..}}`, Go yalnızca anahtarı verir). Denetim: `TestGoCodeDoesNotEditMessageText`, `TestMessageArgsAreNotMessageFragments`.
- Aynı anlam tek koddur: parametre adları farklı olsa da aynı metin iki kodda olamaz (`TestNoDuplicateTexts`; bağlamı farklı etiketler gerekçeyle `sameTextAllowed`'a yazılır).

**Yeni bir dil eklemek.**
1. `internal/lang/messages/<dil>.json` (ISO 639-1); `catalog.go`'da `//go:embed` ve `languageFiles`'a satır.
2. Dilin CLDR çoğul kuralı `template.go`'daki `pluralRules`'a. Kural CLDR işlenenlerini alır (`pluralOperands`: n, i, v, f; tam sayıda i kayıpsızdır, 2^53 üstünde de `i%10` doğru): Türkçe `n = 1`, İngilizce `i = 1 and v = 0` "one"; Rusça, Arapça gibi dillerde daha çok kategori vardır. Denetim: `TestPluralOperands`.
3. Hâl eki olan bir dilse çekimcisini kendi dilbilgisi paketinden `messages.RegisterInflector` ile kaydet.
4. `go test ./internal/lang/messages` her şablonu ve parametreyi denetler; çevrilmemiş kod Türkçeye düşer.

**Bir ileti tek dildedir.** Süreç dili `TR_DIL`'den bir kez okunur (`messages.ActiveLanguage`; `codes` da bunu kullanır, adlar ve iletiler aynı dildedir). Bir iletinin ya da içindeki kodlu hatalardan (`*Error`, `*PosError`) birinin etkin dilde metni yoksa iletinin bütünü kaynak dilde (Türkçe) gösterilir; iç ileti dıştakinin dilinde yazılır. Bu yüzden ileti değerine metin (`messages.Text`) değil kodlu hata (`messages.New`) geçirilir. Denetim: `TestFallbackKeepsOneLanguage`, `TestLanguageReadOnce`.

Pilot: `en.json` 38 farklı türden iletiyi (konum, çoğul, liste, hâl eki, dosya hatası, başlıklar, iç hata) taşır; `TR_DIL=en` ile gösterilir.

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
