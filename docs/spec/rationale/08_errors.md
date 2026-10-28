# 8. Hata yönetimi: emsal ve gerekçe

Kurallar: [`08_hata_yonetimi.md`](../08_hata_yonetimi.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 8.1

- *Emsal:* Kotlin'de `try` bir ifadedir: "It's a common approach to use `try-catch` as an expression, so it can return a value from either the `try` block or the `catch` block". Go'da temizlik `defer` ile yazılır, `finally` yoktur. *Almadık:* Kotlin'in ve Python'un `finally`'si: Kotlin aynı iş için ayrıca `.use()`'u önerir, yani temizliğin iki yazımı olur; TR'de `TR_defer` (9) zaten fonksiyondan her çıkışta çalışır. ([Kotlin exceptions](https://kotlinlang.org/docs/exceptions.html), [Go 1.27 defer](https://go.dev/ref/spec#Defer_statements))
- *Gerekçe:* `TR_if` ve `TR_match` gibi `TR_try` da değer verince "değeri ya da varsayılanı" tek satırda yazılır; ajan geçici bir değişkene atama yapmak zorunda kalmaz. Temizliğin tek yazımı olunca hangi yolun kullanılacağı seçimi kalkar. Kesme ve `TR_exit`'in yakalanmaması 1.5 ve 1.10'un sonucudur.

### 8.2

- *Emsal:* Python yeni hatayı eskisinin sonucu olarak işaretler: "To indicate that an exception is a direct consequence of another, the raise statement allows an optional `from` clause". Go'da bir hata başkasını sarar: "If e.Unwrap() returns a non-nil error w ... then we say that e wraps w". Rust'ın hata türleri çoğunlukla bir enum'dur (`io::ErrorKind`). *Almadık:* her değerin fırlatılıp metne çevrilmesi (bugünkü TR): fırlatılan sözlük yakalayanda bir metne dönüşür, yapısı kaybolur. ([Python 3.15 errors](https://docs.python.org/3/tutorial/errors.html), [Go 1.27 errors](https://pkg.go.dev/errors), [Rust kitabı 9.2](https://doc.rust-lang.org/book/ch09-02-recoverable-errors-with-result.html))
- *Gerekçe:* fırlatılabilen dört tür, yakalayanın ayırabildiği dört türdür: metin (kodsuz), ileti (kodlu), durumlu tip değeri (programın kendi türü), hata (neden zinciri). Neden taşımak, katmanlı işte (dosya okuma → ayar okuma → eğitim) asıl kökün kaybolmamasını sağlar.

### 8.3

- *Emsal:* Rust hatanın türünü değerden okur ve ona göre dallanır: "`File::open` returns inside the `Err` variant ... `io::Error` ... has a method, `kind` ... we match on ... `error.kind()`". Go hata ağacını `errors.Is` ile arar: "It should be used in preference to simple equality checks". *Almadık:* Python'un çoklu `except` kolu: ilk eşleşen kol çalışır, sıra yanlışsa üst sınıf alt sınıfı yutar (belgenin kendi örneği "B, B, B" yazdırır). ([Rust kitabı 9.2](https://doc.rust-lang.org/book/ch09-02-recoverable-errors-with-result.html), [Go 1.27 errors](https://pkg.go.dev/errors), [Python 3.15 errors](https://docs.python.org/3/tutorial/errors.html))
- *Gerekçe:* bugünkü TR'de yakalanan hata yalnız metindir ve metin okuyanın dilindedir [ölçüldü: main `cd38d7bf`, `TR_throw TR_Message("H6903", -1)` yakalanınca yalnız "window cannot be negative, got -1"]: türe göre ayırmanın tek yolu metni karşılaştırmaktır ve bu, Türkçe makinede doğru çalışan programı İngilizce makinede sessizce yanlış dala götürür. Kod ve değer dilden bağımsızdır. Tek `TR_catch` ve içinde `TR_match`, sıranın yanlış olabileceği çoklu kol yerine tek yerde ayırır; 4. katmandan sonra eşlemenin kapsaması da denetlenir.

### 8.4

- *Emsal:* git ve systemd dışarıdan gelen metni terminale yazmadan önce kaçırır. *Almadık:* Python ve Go hata metnini olduğu gibi yazar.
- *Gerekçe:* iletiye giren dış veri (dosya adı, JSON değeri) terminali silemez ya da metni gizleyemez; yakalanan metin değişmediği için program aldığı veriyi olduğu gibi işler.

### 8.5

- *Emsal:* Fluent (`get_message("kimlik")`) ve Java ResourceBundle iletiyi açık bir kimlikle anar; TR'nin H kodları da açık kimliktir. *Almadık:* Python gettext, Swift `String(localized:)` ve Go `x/text` kaynak metni anahtar yapar: metin değişince çeviri düşer, geliştiricinin dili ayrıcalıklı olur (karşılaştırma 2026-10-26).
- *Gerekçe:* kod dilden bağımsızdır, metin okuyanın dilindedir; testler kodu sınadığı için ileti yeniden yazılınca kırılmaz.

### 8.6

- *Emsal:* Python yakalanmayan hatada çağrı yığınını yazar: "The preceding part of the error message shows the context where the exception occurred, in the form of a stack traceback". ([Python 3.15 errors](https://docs.python.org/3/tutorial/errors.html))
- *Gerekçe:* bugün yalnız hatanın fırlatıldığı satır yazılıyor [ölçüldü: `h()` → `g()` → `TR_throw` yalnız `g`'nin satırını gösteriyor]; hatayı düzeltecek ajan çağrının nereden geldiğini görmeden yanlış yeri düzeltir.
