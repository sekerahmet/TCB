# formats

Metin veri biçimlerinin TR yerleşikleri: `TR_fn_json_parse`, `TR_fn_json_string`, `TR_fn_read_csv` ve `TR_fn_parse_csv`.

## Standartlar

**Sıra korunur.** JSON nesnelerinin anahtarları okunduğu sırayla sözlüğe girer, yazılırken aynı
sırayla çıkar.
Denetim: `TestJSONDecodePreservesKeyOrder`, `TestJSONTextOrderNaNAndCycle`.

**CSV hücresi hep metindir (13.8).** Hücre boşlukları kırpılarak metin kalır, sayı program tip
çağrısıyla çevirir; baştaki BOM atılır.
Denetim: `TestCSVCellsAreTextAndBOM`, `FuzzCSVDecode`.

**Yol ile metin karışmaz.** Dosya okuyan ve metin çözen yerleşik ayrıdır: `TR_fn_read_csv` her zaman
dosya okur, `TR_fn_parse_csv` (`TR_fn_json_parse` gibi) her zaman metni ayrıştırır; hiçbiri argümanın ne olduğunu
tahmin etmez. Göreli yollar dilin tek yol kuralıyla çözülür.
Denetim: `TestCSVReadFileAndParseText`, `TestCSVReadRejectsText`, `TestDecodersRequireText`, `TestCSVResolvesRelativeToProgram`; yol kuralı için `TestUserPathsResolvedByLanguageRule` (`internal/stdlib`).
