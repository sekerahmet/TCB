# jsonwrite

TR değerlerini JSON metnine yazan tek yer: `TR_fn_json_string`, web sunucusunun yanıtları, kayıt
dosyaları ve eğitim günlükleri bunu kullanır.

## Standartlar

**Sıra korunur.** Sözlük anahtarları ekleniş sırasıyla yazılır; aynı değer her zaman aynı metni
verir.
Denetim: `TestKeyOrderKept`, `TestSameFormatAsEncodingJSON`.

**Ondalık ondalık kalır.** Ondalık sayı 3.2'nin en kısa yazımıyla yazılır (`1.0`, `1e-07`, `1e+16`),
geri okununca yine ondalıktır.
Denetim: `TestFloatsInTheLanguagesSpelling`.

**Geçersiz JSON üretilmez.** Döngülü ya da çok derin değerler, anahtarı metin olmayan sözlük (13.8;
geri okununca anahtarın tipi değişirdi), NaN ve sonsuz sayılar açık hatadır; ancak çağıran açıkça isterse sayı dışı değerler TR yazımıyla metin
olarak yazılır (eğitim günlüğünde kayıpta beliren NaN gizlenmesin diye).
Denetim: `TestCycleAndDepthError`, `TestNaNAndInfError`, `TestNonTextKeyIsAnError`, `TestNonFiniteAsTextOnlyWhenAsked`.

**Okuma ile tutarlıdır.** Yazılan metin dilin JSON okuyucusuyla geri okunup yeniden yazıldığında
aynı kalır; bu, bulanıklaştırma testiyle sınanır.
Denetim: `FuzzJSONRoundTrip`.

**Süzgeç TR_JSON'un içine de girer.** `TextFilter` (gizli değer süzgeci, 19.4) yazılan her metne ve sözlük
anahtarına uygulanır, `TR_JSON` değerinin içindekiler dahil; süzgeç yokken `TR_JSON` kendi metniyle yazılır.
Olay akışı (28.7) ve kayıtlar (19.4) bunu kullanır.
Denetim: `TestTextFilterReachesInsideJSON`.
