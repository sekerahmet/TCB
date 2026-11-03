# checker

Programı çalıştırmadan önce okuyup hataları bulan denetçi. Bulduğu şeyler: tip uyuşmazlıkları
(tanım, atama, dönüş değeri, çağrı argümanı), sabite atama, tanımsız adlar, yerleşik adların
gölgelenmesi, ad yazılışı, tip değerinin metinle karşılaştırılması (`TR_Type(x) == "Tam"`) ve Mantık
değerinin dilden bağımsız metninin (`true`, `false`) bir yazım dilinin adıyla ya da başka bir dilden
tanıdık adla karşılaştırılması (`TR_String(b) == "doğru"`, `TR_String(b) == "True"`; uyarı).

`TR_cmd_check` komutunda, her program çalışmadan önce ve her modül yüklenmeden önce çalışır.
Görünüm (`source.Show`) de bir sözcüğün dilin mi kullanıcının mı olduğunu buradan (`NameBindings`) öğrenir.

## Standartlar

**Kod çalıştırmaz.** Yalnızca okur ve raporlar; programın hiçbir satırı burada çalışmaz. Denetim: `TestCheckerOnlyReads`, `TestLangOrder` (internal).

**Hata ile uyarı ayrıdır.** Hata programın çalışmasını durdurur. Uyarı (ör. yerleşik adın
gölgelenmesi) durdurmaz ve çıkış kodunu etkilemez. Denetim: `TestBuiltinShadowingWarning`.

**Ad yazılışı uyarıdır (2.3b).** Sabitler `BÜYÜK_YILAN`, yapılar `BüyükDeve`, öbür adlar
`küçük_yılan`; matematik adları (en çok üç harf ve rakam: `W`, `dW`, `Wq`) muaftır. Uyarı önerilen
yazımı verir. Depodaki örnekler ve testler uyarısız, kitaplık ve GPU araçları yazılış uyarısızdır.
Denetim: `TestNameStyleWarnings`, `TestNameStyleAcceptsConventionalAndMathNames`,
`TestLibraryFollowsNameStyle`, `TestExamplesCheckWithoutErrors`.

**Yanlış alarm vermez.** Emin olamadığında susar: adların tamamı bilinemiyorsa (ör. takma adsız
bir `TR_kw_import` varsa) tanımsız ad hatası verilmez. Çalışacak bir programı haksız yere durdurmak, bir
hatayı çalışma anına bırakmaktan daha kötüdür. Denetim: `TestUndefinedNameNoFalseAlarm`, `TestExamplesCheckWithoutErrors`.

**Çalışma anıyla aynı sözleri kullanır.** Aynı hata denetimde ve çalışırken aynı metinle
anlatılır; ortak metinler `hint` paketinden gelir. Denetim: `TestUndefinedNameIsError`, `TestCheckerAndRuntimeShareWording` (interp).

**Modülün ve test dosyasının en dışı yalnız tanımdır (1.1).** Aynı dosya çalıştırılınca program,
aktarılınca modüldür; `TR_test` içeren dosya test dosyasıdır (12.1). Denetleyici dosyanın türünü
`SetFile`'dan öğrenir (`source.Prepare`: `Module` ya da `HasTests`). Modülde ve test dosyasında en
dışta tanım dışındaki deyim H1107, `TR_const`/`TR_var`'ın sabit olmayan ilk değeri H1108'dir
(`module_rules.go`, sabit ifadenin listesi `const_values.go`'da). Hata modülün satırında verilir;
aktaran dosyayı modül hatasının başlığı anar (`interp.ModuleError`). İleti deyimi bir fonksiyona
alan, ilk değeri bir fonksiyonun sonucu yapan yazımı dosyadaki metinden kurar. Dosyanın tanımlamadığı
(aktarılan ya da genel) ad bilinemez, sabit sayılır. Denetim: `TestModuleTopLevelHoldsOnlyDefinitions`,
`TestModuleFirstValuesAreConstant`, `TestModuleRuleMessages`, `TestRepoModulesHoldOnlyDefinitions`,
`TestModuleRuleSuggestionRuns` (interp).

**Yerel değişkenin tipi ilk değerinden çıkar ve değişmez (1.2, 3.13).** Açıklamasız `TR_var`'ın
tipi ilk değerinin çıkarılan tipidir; sonraki atama ona uyar (H1005: `TR_var toplam = 0` sonra
`toplam += 0.5` hatadır, ondalık değişkene tam sayı atamak değildir). Tipi çıkarılamayan ilk değer,
`TR_none` ve denetleyicinin tanımadığı tip (`local_types.go`, `knownType`) değişkeni `TR_Any` bırakır.
İşlecin sonucu çalışma anının kuralıyla çıkar (önek `-`, metnin ve listenin `+` ve `*`'ı, tensörün
aritmetiği). Çağrının tipi tipli fonksiyonun dönüşü, yapı kurucusunda yapının kendisi (`Nokta(1, 2)` →
`Nokta`), tipi bilinen yapı nesnesinin metodunda metodun dönüşü, yerleşikte tanım dosyasının tek tipli dönüşüdür
(`builtin_returns.go`; motorla uyuştuğu `TestSingleReturnsMatchEngine` ile sınanır, uyuşmayan `ReturnsNotVerified`'dadır). İki kullanıcı tipi ancak ikisi de
programın yapısıysa ve ayrıysa uyumsuzdur; arayüz ya da bilinmeyen tip yapıyı karşılayabilir
(`type_shapes.go`). Liste ve sözlüğün tipi öğe tipini taşır (`[1, 2]` → `TR_List[TR_Int]`, yazılan
`TR_List[TR_Int]` de); boş ya da karışık değişmez öğe tipsizdir (`TR_List`) ve asıl adla karşılaştırılır.
Kap tip argümanında değişmezdir (3.6): `TR_List[TR_Int]` `TR_List[TR_Float]` yerine verilmez, değişmezdeki
tam sayı yazımı ise ondalık beklenen yerde ondalıktır. Demetin tipi öğelerinin tipleridir (`(1, "a")` →
`(TR_Int, TR_String)`, yazılan demet tipi de); `t.0` öğenin tipini verir. Denetim: `TestLocalTypeIsPermanent`,
`TestLocalTypeUnknownStaysAny`, `TestOperatorTypes`, `TestStructCallTypes`, `TestStructMeetsInterface`,
`TestUserTypeCompatibility`, `TestCollectionLiteralTypes`, `TestCollectionElementTypes`,
`TestContainerCompatibility`, `TestTypeShapeText`, `TestTupleTypes`, `TestRepoFirstValueTypes` (sayaç).

**Ad olmayan `T?` ifadesi açılmadan `T` beklenen yere verilmez (3.5).** `l.TR_first`, `l.TR_get(5)`, `TR_List[T?]`'nin indeksi gibi isteğe bağlı tipli bir ifade isteğe bağlı olmayan, anlamı bilinen tip beklenen yere (tanım H1001, atama H1005, dönüş H1003, kullanıcı fonksiyonunun argümanı H1004, kurucu H1125, alan H1124, yerleşik metodun argümanı H2423, yerleşik fonksiyonun argümanı fonksiyonun çalışma anı kodu, `.TR_append` H1123) ve liste, sözlük, demet değişmezinin öğesine verilmez; öğesi isteğe bağlı olmayan kabın indeksine yazılmaz (H1135). H1001, H1003, H1005 ve H1135'in ipucu H1130: ifadenin dosyadaki yazımıyla `l.TR_first ?? 0`. Ad (daraltma, 4b), `TR_Any` ve bilinmeyen ad (tip parametresi) beklenen yer geçer (`optional_values.go`). Denetim: `TestUnopenedOptional`, `TestUnopenedOptionalHint`, `tests/negative/istege_bagli_acilmadan.tr`.

**Kabın öğe tipi tektir, paylaşılan öğe yinelenmez (3.6, 3.7, 4.10).** Öğe tipi bilinen kabın indeksine başka tipte değer yazmak H1136'dır (`m[0] = "s"`, `d["b"] = "s"`; bileşik atamada işlemin sonucu: `m[0] += 1.5`); öğe tipleri ayrı iki listenin `+`'sı ve `+=`'sı H1138 (`[1] + ["a"]`, `[1] + [2.5]`). Kabın tip argümanında isteğe bağlı öğe isteğe bağlı olmayan yere verilmez: `[1, TR_none]` bir `TR_List[TR_Int]` değildir (H1001). Tipi bilinmeyen değer, öğe tipsiz kap, dilim ve fonksiyon geçer (`element_types.go`). Öğeleri başvuruyla paylaşılan (liste, sözlük, tensör, yapı ya da onları tutan demet) listenin `*` ile yinelenmesi H1139 (`[[0] * 3] * 3`, `*=`'de de); ileti tek öğeli değişmezde her satırı ayrı kuran üreteci önerir: `[[0] * 3 TR_for _ TR_in 0..3]`. Tensörün karşılaştırması bir tensördür (15.5). Denetim: `TestElementAssignType`, `TestOptionalElementListIsNotARequiredList`, `TestListPlusElementTypes`, `TestSharedRowsAreNotRepeated`, `TestTensorComparisonIsATensor`, `tests/negative/oge_tipi_atama.tr`, `tests/negative/paylasilan_satirlar.tr`.

**Sözlüğün anahtar tipi tektir (3.7).** Anahtar tipi bilinen sözlüğü başka tipte anahtarla okumak ya da ona yazmak H1140'tır (`d[1]`, `d[2] = 3`, `.TR_get(5)`, `.TR_contains(5)`); ondalık anahtar (`{1.5: 1}`, `e[1.5]`, `TR_Dict[TR_Float, V]`) çalışma anının koduyla H2646. Tipi bilinmeyen ya da isteğe bağlı anahtar ve öğe tipsiz sözlük geçer (`dict_keys.go`). Denetim: `TestDictKeyType`, `tests/negative/sozluk_anahtar_tipi.tr`.

**`==` ayrı tipleri karşılaştırmaz (4.5).** Tipleri bilinen ve ayrı iki değerin `==`/`!=`'si H1141'dir (`"1" == 1`, `[1] == [1.0]`: kaplar tip argümanında değişmez); sayılar, isteğe bağlı ile asıl tipi ve `TR_none`, JSON, fonksiyon, tip değeri ve tensör kendi kurallarındadır, tipi bilinmeyen değer ve öğe tipsiz kap geçer (`equality_types.go`). `TR_JSON?` (`j.TR_get(k)`) ile JSON olmayanın karşılaştırması H1131'dir; öneri öbür yanı çevirir. Denetim: `TestDistinctTypesAreNotCompared`, `TestOptionalJSONComparison`, `tests/negative/ayri_tip_esitligi.tr`.

**`TR_JSON` çekirdek bir tiptir (3.7a).** `TR_json_parse(m)`, `TR_JSON(x)`, `j["k"]` ve `j[i]` `TR_JSON`; `TR_List(j)` `TR_List[TR_JSON]`; `.TR_get(k)` `TR_JSON?`, `.TR_keys()` `TR_List[TR_String]`, `.TR_length` `TR_Int` (`json_values.go`). `TR_JSON` veren yerleşikler (`JSONReturns`) motorla `TestSingleReturnsMatchEngine`'de değil `TestJSONReturnsAreJSONValues`'ta (stdlib) doğrulanır: motorun `TR_Type`'ı JSON'un türünü verir. `TR_JSON(x)`'te tipi bilinen ve JSON'a giremeyen değer ya da değişmezin öğesi (demet, tensör, yapı, fonksiyon, aralık, metin olmayan anahtarlı sözlük) çalışma anının kodudur (H2654). Denetim: `TestJSONTypes`, `TestJSONConversionKinds`, `tests/negative/json_cevirme.tr`.

**JSON değeri yalnız JSON değeriyle karşılaştırılır (3.7a, 4.5).** `==` ve `!=`'de `TR_JSON` ile tipi bilinen JSON olmayan değer H1131, `TR_none` H1132; kabın içinde (liste, sözlük, demet öğe öğe), `.TR_contains`'te ve eşleme deseninde de. `TR_Type(j) == TR_JSON` hep yanlıştır: H1134. `TR_JSON`'a indeksle atama çalışma anının H2403'üdür. Öneriler programın yazımıyla kurulur (`splice`): `TR_Int(j["k"]) == 3`, `j["k"] == TR_JSON(3)`, `TR_Type(j["n"]) == TR_NoneType`, `j.TR_get("n") == TR_none`, `TR_Type(j) == TR_Dict` (`json_comparison.go`). Denetim: `TestJSONComparison`, `tests/negative/json_karsilastirma.tr`.

**Demetin uzunluğu ve öğeleri tipindedir (3.6a, 5.3, 4.4, 15.5).** Tipi bilinen demeti (değişken, tipli parametre, alan) yanlış sayıda ada açmak H1026 (`unpack.go`, önerisi `ast.UnpackSuggestion`); sınır dışı indeks (`t.5`) H2774; tensör tutan demetin `==`/`!=`'si H2777; sıralanamayan öğe çiftli demetin `<`, `>`, `<=`, `>=`'si H2772 (`tuple_comparison.go`; çalışma anının `tuple_ops.go`'su gibi ortak öğelerde ve iç içe demette). Tipi bilinmeyen ve isteğe bağlı demet çalışırken denetlenir. Denetim: `TestUnpackCountOfAKnownTuple`, `TestTupleTypeRules`, `tests/negative/demet_acma.tr`, `tests/negative/demet_tipi.tr`.

**Yerleşik çağrısı imzasıyla denetlenir (13.9–13.11).** Tanım dosyasında `pure` işaretli yerleşiğin (yalnız yeni değer veren) çağrısı tek başına deyimse H1115: değer atılır; ileti atamayı önerir, alıcı atanabilir adsa uygulanabilir düzeltmeyle (`l = l.TR_sort()`; `pure_calls.go`). Tipi bilinen alıcıda (Metin, Liste, Sözlük, Aralık) yerleşik metodun konumlu argüman sayısı imzaya uymuyorsa çalışma anındaki hata (H2011, `hint.MemberArgCount`): ad ve kullanım kimlikle (`.TR_sort()`), `.TR_sort()`'a anahtar verilirse bugünkü yol (demet sıralaması; `builtin_calls.go`). Sayının tanımla aynı olduğu `TestMethodCountsMatchDefinitions` (interp), üyenin tiplerinin `TestMemberTypesMatchDefinitions` (stdlib) ile sınanır; uyuşmayan üye `MembersBeyondDefinitions`'tadır. Denetim: `TestPureCallValueUnused`, `TestMemberArgCount`, `TestMessageSuggestionsRunCleanly` (internal).

**Yerleşik fonksiyonun argümanı seçeneklerinden birine uyar (13.9–13.11).** Tanımın `TR_Int|TR_Float|TR_Tensor` parametresi birden çok imzadır (13.10); konumlu argüman hiçbir seçeneğe uymazsa denetim hatası, çalışma anının fonksiyona ve verilen tipe göre verdiği kodla: `TR_abs("s")` H2423, `TR_sin("s")` H6103, `TR_zeros("a")` H6021 (`builtin_args.go`). Kodlar `FunctionArgCodes`'ta ölçülmüş olarak durur (39 fonksiyon); tabloda olmayan fonksiyon denetlenmez. Liste ve Tam verilen argüman, çalışma anının önerili kodu (H6104, H6020) kaldığı fonksiyonlarda karar vermez. Tam sayı ondalık seçeneğe uyar; ad olmayan `T?` ancak bir seçenek `TR_NoneType` ise geçer, değilse yokluğun koduyla (`TR_abs(l.TR_first)` H2423). Tipi bilinmeyen değer, ad, JSON ve fonksiyon değeri geçer. Seçeneklerin ve kodların motorla aynı olduğu `TestFunctionParamOptionsMatchEngine` (stdlib) ile sınanır. Denetim: `TestBuiltinArgOptions`, `tests/negative/yerlesik_arguman_tipi.tr`.

**Koşul `TR_Bool`'dur (3.10).** Tipi bilinen koşul (`TR_if`, koşullu `TR_for`, liste üreteci, `TR_breakpoint TR_if`; H2600), `TR_and`/`TR_or`/`TR_not` işleneni (H2601) ve `TR_assert`'in koşulu (H2612) `TR_Bool` değilse çalışma anının koduyla denetim hatası (`conditions.go`); tipi bilinmeyen ve isteğe bağlı koşul susar. Denetim: `TestConditionMustBeBool`.

**Tek hata, tek mesaj.** Bir hata aynı satırda zincirleme ikinci bir hata üretmez (ör. yanlış
yazılmış tip adı ayrıca bir tip uyuşmazlığı olarak bildirilmez). Denetim: `TestTypeNameASCIISuggestion`, `TestTypeNameCaseSpelling`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_read_only_test.go`: denetçinin içe aktarmaları (yorumlayıcı, dosya, süreç, ağ yok): kod çalıştırmaz
- `behavior_repo_inference_test.go`: sayaç (1.2, 3.13): depodaki programların `TR_var` ve `TR_const` ilk değerlerinden kaçının tipi çıkarılamıyor (`TR_Any`); düşmez, yazar
- `behavior_repo_modules_test.go`: depodaki modüller ve test dosyaları 1.1'e uyar (en dışta yalnız tanım, sabit ilk değer)
- `behavior_repo_programs_test.go`: depodaki programlar (örnekler, kitaplık, GPU araçları, dil tanımı testleri) diskteki kodlu biçimleriyle okunup hatasız, uyarısız ve yazılış kuralına uygun denetlenir (dış test paketi: `source` denetçiyi içe aktarır)
