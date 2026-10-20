# formats

Metin veri biçimlerinin TR yerleşikleri: `json_çöz`, `json_metin`, `csv_oku` ve `csv_çöz`.

## Standartlar

**Sıra korunur.** JSON nesnelerinin anahtarları okunduğu sırayla sözlüğe girer, yazılırken aynı
sırayla çıkar.

**Sayılar katı okunur.** CSV hücresi yalnızca bütünüyle bir sayıysa sayı olur; baştaki BOM
atılır, gerisi metin olarak kalır.

**Yol ile metin karışmaz.** Dosya okuyan ve metin çözen yerleşik ayrıdır: `csv_oku` her zaman
dosya okur, `csv_çöz` (`json_çöz` gibi) her zaman metni ayrıştırır; hiçbiri argümanın ne olduğunu
tahmin etmez. Göreli yollar dilin tek yol kuralıyla çözülür.
