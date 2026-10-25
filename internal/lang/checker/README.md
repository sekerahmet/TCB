# checker

Programı çalıştırmadan önce okuyup hataları bulan denetçi. Bulduğu şeyler: tip uyuşmazlıkları
(tanım, atama, dönüş değeri, çağrı argümanı), sabite atama, tanımsız adlar, yerleşik adların
gölgelenmesi, ad yazılışı, tip değerinin metinle karşılaştırılması (`tip(x) == "Tam"`) ve Mantık
değerinin metninin başka bir yazım dilinin adıyla karşılaştırılması (İngilizce dosyada `str(b) ==
"doğru"`, uyarı; dosyanın dili `SetFileLanguage` ile gelir).

`tr kontrol` komutunda, her program çalışmadan önce ve her modül yüklenmeden önce çalışır.
Kodlu biçime çevirici de bir sözcüğün dilin mi kullanıcının mı olduğunu buradan öğrenir.

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
bir `aktar` varsa) tanımsız ad hatası verilmez. Çalışacak bir programı haksız yere durdurmak, bir
hatayı çalışma anına bırakmaktan daha kötüdür. Denetim: `TestUndefinedNameNoFalseAlarm`, `TestExamplesCheckWithoutErrors`.

**Çalışma anıyla aynı sözleri kullanır.** Aynı hata denetimde ve çalışırken aynı metinle
anlatılır; ortak metinler `hint` paketinden gelir. Denetim: `TestUndefinedNameIsError`, `TestCheckerAndRuntimeShareWording` (interp).

**Tek hata, tek mesaj.** Bir hata aynı satırda zincirleme ikinci bir hata üretmez (ör. yanlış
yazılmış tip adı ayrıca bir tip uyuşmazlığı olarak bildirilmez). Denetim: `TestTypeNameASCIISuggestion`, `TestTypeNameCaseSpelling`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_read_only_test.go`: denetçinin içe aktarmaları (yorumlayıcı, dosya, süreç, ağ yok): kod çalıştırmaz
