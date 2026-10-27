# jsonwrite

TR değerlerini JSON metnine yazan tek yer: `TR_fn_json_string`, web sunucusunun yanıtları, kayıt
dosyaları ve eğitim günlükleri bunu kullanır.

## Standartlar

**Sıra korunur.** Sözlük anahtarları ekleniş sırasıyla yazılır; aynı değer her zaman aynı metni
verir.
Denetim: `TestKeyOrderKept`, `TestSameFormatAsEncodingJSON`.

**Geçersiz JSON üretilmez.** Döngülü ya da çok derin değerler, aynı görünen iki anahtar, NaN ve
sonsuz sayılar açık hatadır; ancak çağıran açıkça isterse sayı dışı değerler TR yazımıyla metin
olarak yazılır (eğitim günlüğünde kayıpta beliren NaN gizlenmesin diye).
Denetim: `TestCycleAndDepthError`, `TestNaNAndInfError`, `TestLookalikeKeyError`, `TestNonFiniteAsTextOnlyWhenAsked`.

**Okuma ile tutarlıdır.** Yazılan metin dilin JSON okuyucusuyla geri okunup yeniden yazıldığında
aynı kalır; bu, bulanıklaştırma testiyle sınanır.
Denetim: `FuzzJSONRoundTrip`.
