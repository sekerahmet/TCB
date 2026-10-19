# formats

Metin veri biçimlerinin TR yerleşikleri: `json_çöz`, `json_metin` ve `csv_oku`.

## Standartlar

**Sıra korunur.** JSON nesnelerinin anahtarları okunduğu sırayla sözlüğe girer, yazılırken aynı
sırayla çıkar.

**Sayılar katı okunur.** CSV hücresi yalnızca bütünüyle bir sayıysa sayı olur; baştaki BOM
atılır, gerisi metin olarak kalır.

**Yol ile metin karışmaz.** Bir `Yol` değeri her zaman dosya olarak okunur; düz metin yalnızca
açıkça bir dosyaya benziyorsa dosya sayılır. Göreli yollar dilin tek yol kuralıyla çözülür.
