# 8. Hata yönetimi

**8.1** `dene { ... } yakala ad { ... }`: dene bloğunda hata olursa yakala bloğu, hata mesajı `ad` değişkenine bağlanarak çalışır. Ad verilmezse `hata` kullanılır; `yakala (ad)` biçimi de geçerlidir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

**8.2** `fırlat ifade` bir hata başlatır; değer metne çevrilir ve yakalayan değişken her zaman `Metin`dir (`fırlat {"kod": 5}` → `"{\"kod\": 5}"`).

**8.3** Çalışma zamanı hataları (sıfıra bölme, tanımsız ad, tip uyuşmazlığı...) da yakalanabilir; mesajları `Satır N:` ile başlar. Yakalanmayan hata fonksiyon çağrılarından yukarı yayılır ve programı konum ve kod çerçevesiyle durdurur.
