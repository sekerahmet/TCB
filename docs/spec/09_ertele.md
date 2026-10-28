# 9. Ertele

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "9. Ertele".

Her kuralın emsali ve gerekçesi: [`rationale/09_defer.md`](rationale/09_defer.md).

**9.1** `TR_defer { ... }` bloğun içinde bulunduğu kapsam bitince çalışacak işi kaydeder. Kapsam onu içeren en yakın bloktur: fonksiyonun gövdesi, `TR_if` ya da `TR_match` kolu, döngünün bir turu, `TR_try` bloğu; en dışta program, `TR_async` içinde görev, testte test. Birden çok ertele ters sırayla çalışır (son kaydedilen ilk). Tek yazım bloktur: `TR_defer f(x)` sözdizimi hatasıdır, ileti `TR_defer { f(x) }`'i önerir.
- *Test:* TR testi; kapsamın blok oluşu ve tek yazımın iletisi uygulanınca TR testi.

```tr
# TR 4
TR_func işle(adlar: TR_List[TR_String]) {
    TR_for ad TR_in adlar {
        TR_defer { TR_print("\(ad) bitti") }
        TR_print("\(ad) başladı")
    }
}
```

Her turun ertelenen işi o turun sonunda çalışır, fonksiyonun sonunu beklemez: "a başladı", "a bitti", "b başladı", "b bitti". Bir dosyayı ya da bağlantıyı turda açan döngü onu turda kapatır.

**9.2** Ertelenen iş kapsamdan her çıkışta çalışır: blok sona gelince, `TR_return`, `TR_break` ya da `TR_continue` ile çıkılınca, bir hata ile çıkılınca, program durdurulunca (1.5) ve `TR_exit` ile bitince (1.13).
- *Test:* TR testi.

**9.3** Ertelenen işin bloğu çalıştığı anda okur: içindeki adlar kapsam biterkenki değerlerini verir, kayıt anındakileri değil. Kayıt anındaki değer gerekiyorsa önce bir sabite alınır: `TR_const eski_x = x`, sonra `TR_defer { TR_print(eski_x) }`. Ertelenen işin bloğundan `TR_return`, `TR_break` ya da `TR_continue` ile çıkılamaz, sözdizimi hatasıdır: ertelenen iş kapsamın sonucunu değiştirmez. Bloğun içindeki bir döngüde `TR_break` ve `TR_continue` geçerlidir.
- *Test:* TR testi; bloktan çıkışın hatası uygulanınca TR testi.

**9.4** Ertelenen işteki hata yutulmaz. Bir ertelenen iş hata verse de sonraki ertelenen işler çalışır. Kapsam hatasız bittiyse ilk ertelenen işin hatası kapsamın hatası olur. Kapsam bir hatayla bittiyse o hata korunur: kodu, değeri ve nedeni değişmez. Ertelenen işlerin öbür hataları çalışma sırasıyla korunan hatanın `e.TR_suppressed` listesine (`TR_List[TR_Error]`, 8.3) eklenir.
- *Test:* TR testi; `TR_suppressed` uygulanınca TR testi.
