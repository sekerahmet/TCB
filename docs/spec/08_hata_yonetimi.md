# 8. Hata yönetimi

**8.1** `dene { ... } yakala ad { ... }`: dene bloğunda hata olursa yakala bloğu, hata mesajı `ad` değişkenine bağlanarak çalışır. Ad verilmezse `hata` kullanılır; `yakala (ad)` biçimi de geçerlidir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

**8.2** `fırlat ifade` bir hata başlatır; değer metne çevrilir ve yakalayan değişken her zaman `Metin`dir (`fırlat {"kod": 5}` → `` `{"kod": 5}` ``).

**8.3** Çalışma zamanı hataları (sıfıra bölme, tanımsız ad, tip uyuşmazlığı...) da yakalanabilir; mesajları `Satır N:` ile başlar. Yakalanan hatanın metni programın değeridir: onu yakalayan dosyanın dilinde kurulur, içindeki tip adları ve değerlerle (İngilizce dosyada `Line N: ... Bool (true) given`; değer metni kuralı, 3), hata başka bir dosyada oluşmuş olsa da. Yakalanmayan hata fonksiyon çağrılarından yukarı yayılır ve programı konum ve kod çerçevesiyle durdurur; ekrandaki bu ileti görüntüdür, kişinin dilindedir (`TR_DIL`). Kesme isteği (Ctrl+C, bkz. 1) bir hata değildir, yakalanmaz.

**8.4** Hata ve uyarılar terminale yazılırken (komut satırının hata çıktısı, `tr test` raporu, eşzamanlı görev hataları) denetim karakterleri görünür biçimde yazılır: `\x1b`, `\x07`, `\u009b` gibi; satır sonu ve sekme korunur. Böylece iletiye giren dış veri (bir dosya adı, bir JSON değeri) terminali silemez, metni gizleyemez. Yakalanan hatanın metni değişmez (`dene/yakala` ile alınan metin olduğu gibidir); programın kendi çıktısı (`yazdır`) da değişmez, orada renk kullanmak programın seçimidir. Karşılaştırma: Python ve Go hata metnini olduğu gibi yazar; git ve systemd dışarıdan gelen metni terminale yazmadan önce kaçırır.
