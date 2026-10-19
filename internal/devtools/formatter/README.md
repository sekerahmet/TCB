# formatter

`tr biçimle` komutunun çekirdeği: TR kaynağını tek bir standart biçime getirir (girinti,
boşluklar, satır yapısı).

## Standartlar

**Biçimlendirme anlamı değiştirmez.** Sonuç özgün kaynakla aynı sözcük birimi dizisine ve aynı
ağaca (AST) sahip olmalıdır; değilse sonuç reddedilir ve dosyaya dokunulmaz.

**Hatalı kod biçimlenmez.** Ayrıştırılamayan kaynak olduğu gibi kalır, sözdizimi hatası
bildirilir.

**Korunması gerekenler korunur.** Metin değişmezleri bayt bayt, yorumlar ve satır yapısı ile
dosyanın başındaki BOM olduğu gibi kalır; kodlu dosyalar da biçimlenir.

**Biçim kararlıdır.** Biçimlenmiş bir dosya yeniden biçimlendirildiğinde değişmez; depodaki
bütün .tr dosyaları standart biçimdedir.
