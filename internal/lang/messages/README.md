# messages

Kullanıcıya giden her iletiyi (hata, uyarı, bilgi) değişmez bir koda bağlar: `H7003` →
`{fn}: {path|q} bir klasör; CSV dosyasının yolunu verin`. Adlar için `internal/lang/codes` ne ise
iletiler için bu pakettir.

## Standartlar

**Kod değişmez, metin değişebilir.** Kod `H` ve dört basamaktır; binler basamağı alanı gösterir:
`H0` sözdizimi · `H1` tip denetimi · `H2` yorumlayıcı (ortak `H2000` = `{fn}: {error}`) · `H3`
tensör ve CPU · `H4` GPU · `H5` veri ve parçalayıcı · `H6` stdlib/ml · `H7` standart kitaplığın
geri kalanı · `H8` araçlar ve `tr` komutu · `H9` iç hatalar. Silinen bir kodun numarası bir daha
kullanılmaz.

**Go kodu metin yazmaz, kod yazar.** `messages.New(messages.H7003, fn, path)` bir hata,
`messages.Text(...)` hata olmayan çıktı (uyarı, not) verir. Değerler tanımdaki parametre sırasıyla
verilir; metin hata yazılırken etkin dilde kurulur. Sabitler üretilir
(`go run ./tools/gen/msgconsts`); katalogda olmayan kod derlemeyi düşürür.

**Dosyalar.** `definitions.json` dilden bağımsızdır: her kodun parametre adları
(`"H7003": { "param": ["fn", "path"] }`; adlar İngilizce tanımlayıcıdır). `tr.json` kaynak dildir
ve her kodun metnini tutar; başka bir dil dosyası (`en.json` ...) eksik kodu Türkçe gösterir. Her
kod bir satırdır, kodlar artan sıradadır.

**Şablon.** `{ad}` değeri olduğu gibi, `{ad|q}` Go tırnağıyla (denetim karakterleri kaçırılarak;
kullanıcı verisi için) yazar; `{{` ve `}}` düz paranteztir. Türkçe metin her parametreyi kullanır;
öbür diller kullanmayabilir ve sırayı değiştirebilir.

**Testler metni değil kodu denetler.** `messages.Has(err, messages.H7003)` hata zincirinde kodu
arar; metin değişince test kırılmaz.

**İç hata.** Yalnızca TR'nin kendi hatasıyla oluşabilen durum `messages.Internal("ayrıntı")` ile
bildirilir: kullanıcı tek ortak iletiyi (`H9000`, "lütfen bildirin") görür, ayrıntı geliştirici
içindir ve İngilizce yazılır.

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
