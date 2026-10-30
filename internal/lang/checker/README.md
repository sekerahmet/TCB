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
`Nokta`), tipi bilinen yapı nesnesinin metodunda metodun dönüşüdür. İki kullanıcı tipi ancak ikisi de
programın yapısıysa ve ayrıysa uyumsuzdur; arayüz ya da bilinmeyen tip yapıyı karşılayabilir
(`type_shapes.go`). Denetim: `TestLocalTypeIsPermanent`, `TestLocalTypeUnknownStaysAny`,
`TestOperatorTypes`, `TestStructCallTypes`, `TestStructMeetsInterface`, `TestUserTypeCompatibility`,
`TestRepoFirstValueTypes` (sayaç).

**Tek hata, tek mesaj.** Bir hata aynı satırda zincirleme ikinci bir hata üretmez (ör. yanlış
yazılmış tip adı ayrıca bir tip uyuşmazlığı olarak bildirilmez). Denetim: `TestTypeNameASCIISuggestion`, `TestTypeNameCaseSpelling`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_read_only_test.go`: denetçinin içe aktarmaları (yorumlayıcı, dosya, süreç, ağ yok): kod çalıştırmaz
- `behavior_repo_inference_test.go`: sayaç (1.2, 3.13): depodaki programların `TR_var` ve `TR_const` ilk değerlerinden kaçının tipi çıkarılamıyor (`TR_Any`); düşmez, yazar
- `behavior_repo_modules_test.go`: depodaki modüller ve test dosyaları 1.1'e uyar (en dışta yalnız tanım, sabit ilk değer)
- `behavior_repo_programs_test.go`: depodaki programlar (örnekler, kitaplık, GPU araçları, dil tanımı testleri) diskteki kodlu biçimleriyle okunup hatasız, uyarısız ve yazılış kuralına uygun denetlenir (dış test paketi: `source` denetçiyi içe aktarır)
