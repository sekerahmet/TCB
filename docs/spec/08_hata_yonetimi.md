# 8. Hata yönetimi

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "8. Hata yönetimi".

Her kuralın emsali ve gerekçesi: [`rationale/08_errors.md`](rationale/08_errors.md).

**8.1** `TR_try { ... } TR_catch e { ... }`: blok bir hatayla biterse yakalama bloğu çalışır; `e` hatayı tutan bir `TR_Error`'dır (8.3). Ad yazılmazsa hata `TR_error`'dadır (2.9b). `TR_catch` her zaman yazılır. `TR_try` ifade olarak da kullanılır: değeri çalışan bloğun son ifadesidir (`TR_var n = TR_try { TR_Int(m) } TR_catch e { 0 }`); dalların tipi aynıdır (5.5 gibi). Temizlik bloğu yoktur: fonksiyondan nasıl çıkılırsa çıkılsın çalışacak iş `TR_defer` ile yazılır (9). Başka dillerin yazımları sözdizimi hatasıdır ve ileti TR'deki yazımı söyler: Python'un `except` ve `finally`'si (`TR_catch`, `TR_defer`), parantez içinde ad (`TR_catch (e)`). Kesme isteği (1.5) ve `TR_exit` (1.10) hata değildir, yakalanmaz.
- *Test:* TR testi; ifade olarak `TR_try`, parantezli adın ve Python yazımlarının iletileri uygulanınca TR testi.

**8.2** `TR_throw ifade` bir hata başlatır. Fırlatılan değer bir metin, bir ileti (`TR_Message`, 8.5), bir durumlu tip değeri (7.7) ya da bir `TR_Error` olabilir; başka tip çalışma hatasıdır. Python'un `raise`'i sözdizimi hatasıdır, ileti `TR_throw`'u önerir. Yakalanan hata `TR_throw e` ile olduğu gibi yeniden fırlatılır: kodu, değeri ve nedeni korunur. Yeni bir hata eskisini neden olarak taşıyabilir: `TR_throw TR_Error("okunamadı", TR_cause: e)` (tip çağrısı, 3.12).
- *Test:* TR testi; metin dışı değerlerin fırlatılması, neden ve `raise`'in iletisi uygulanınca TR testi.

**8.3** Hata değeri `TR_Error` şunları taşır:
- `e.TR_code`: iletinin kodu (`"H6903"`, metin; 8.5) ya da kodu olmayan hatada `TR_none`;
- `e.TR_message`: iletinin metni, okuyanın dilinde (`TR_DIL`);
- `e.TR_value`: fırlatılan değer (metin, ileti ya da durumlu tip değeri); çalışma hatasında iletisi;
- `e.TR_cause`: nedeni olan hata (`TR_Error?`).

Hatanın türüne göre ayırma koda ya da değere göre yapılır, metne göre değil: metin okuyanın diline göre değişir. Tek `TR_catch` vardır; ayırma onun içinde `TR_match` ile yapılır, yakalanmayacak hata yeniden fırlatılır. Çalışma hataları (sıfıra bölme, olmayan anahtar ...) ve `TR_assert`'in başarısızlığı da yakalanır, kodlarıyla. Hata değerinin metni iletisidir (3.14); hata değerinin kendisi karşılaştırılamaz, kodu ya da değeri karşılaştırılır. Yakalanmayan hata fonksiyon çağrılarından yukarı yayılır.
- *Test:* TR testi; `TR_Error` değeri ve üyeleri uygulanınca TR testi.

```tr
# TR 4
TR_enum AyarHatası {
    Yok(yol: TR_String)
    Bozuk(satır: TR_Int)
}

TR_func ayar_oku(yol: TR_String) -> TR_JSON {
    TR_throw AyarHatası.Yok(yol)
}

TR_var ayar = TR_try { ayar_oku("ayar.json") } TR_catch e {
    TR_match e.TR_value {
        AyarHatası.Yok(y) => TR_JSON({})
        _ => { TR_throw e }
    }
}
```

**8.4** Hata ve uyarılar terminale yazılırken (komut satırının hata çıktısı, `tr test` raporu, eşzamanlı görev hataları) denetim karakterleri görünür biçimde yazılır: `\x1b`, `\x07`, `\u009b` gibi; satır sonu ve sekme korunur. Böylece iletiye giren dış veri (bir dosya adı, bir JSON değeri) terminali silemez, metni gizleyemez. Yakalanan hatanın metni değişmez; programın kendi çıktısı (`TR_print`) da değişmez, orada renk kullanmak programın seçimidir.
- *Test:* TR testi.

**8.5** Katalog iletisiyle hata: `TR_throw TR_Message("H6903", -1)` ya da `TR_assert(koşul, TR_Message("H6903", -1))` hatayı iletinin kodu ve değerleriyle fırlatır. `TR_Message` kodu (metin değeri) ve kataloğun tanımındaki sırayla değerleri alır; kod tanımlı olmalı, değer sayısı tanımla aynı olmalı, aksi `TR_Message` kurulurken hatadır. İletinin metni katalogdan, okuyanın dilinde kurulur; kitaplığın kendi adları iletide o dilin adıyla yazılır, bu yüzden kitaplığın hatası okuyanın dilinde çıkar. Düz metin `TR_throw "..."` programın kendi hatası içindir ve olduğu gibi kalır. `TR_error_code(f)` `f`'yi çalıştırır ve fırlayan hatanın kodunu (`"H6903"`) verir, hata yoksa `TR_none`; testler iletinin metnini değil kodunu sınar (`TR_assert_equal("H6903", TR_error_code { ... })`), ileti yeniden yazılınca kırılmaz. `TR_error_code` nedenler zincirinde en içteki kodu verir.
- *Test:* TR testi.

**8.6** Yakalanmayan hata programı durdurur ve çağrı yığınını yazar: her çerçeve için dosya, satır ve satırın metni, en son çağrı en altta; en sonda hatanın kodu ve iletisi okuyanın dilinde. Çıkış kodu 1'dir (1.9).
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca Go e2e testi.
