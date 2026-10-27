# formatter

`TR_cmd_format` komutunun çekirdeği: TR kaynağını tek bir standart biçime getirir (girinti,
boşluklar, satır yapısı).

## Standartlar

**Biçimlendirme anlamı değiştirmez.** Sonuç özgün kaynakla aynı sözcük birimi dizisine ve aynı
ağaca (AST) sahip olmalıdır; değilse sonuç reddedilir ve dosyaya dokunulmaz. Denetim: `TestMeaningChangeRejected`, `TestSameMeaning`.

**Hatalı kod biçimlenmez.** Ayrıştırılamayan kaynak olduğu gibi kalır, sözdizimi hatası
bildirilir. Denetim: `TestSyntaxErrorNotFormatted`.

**Korunması gerekenler korunur.** Metin değişmezleri bayt bayt, yorumlar ve satır yapısı ile
dosyanın başındaki BOM olduğu gibi kalır; kodlu dosyalar da biçimlenir. Denetim: `TestStringsKeptByteForByte`, `TestCommentsKept`, `TestLineStructureKept`, `TestBOMKept`, `TestCodedFileFormats`.

**Biçim kararlıdır.** Biçimlenmiş bir dosya yeniden biçimlendirildiğinde değişmez; depodaki
bütün .tr dosyaları standart biçimdedir. Denetim: `TestStandardFormat`, `FuzzFormat`, `TestRepoFiles`.
