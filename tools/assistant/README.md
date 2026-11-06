# Asistan ekranı

TR'de yazılacak yerel asistanın ekranı ve TR'yle konuşacağı sözleşme. Ekran incedir: tek HTML dosyası, düz JavaScript, derleme yok, dışarıdan hiçbir şey yüklemez. TR'ye HTTP ile bağlanır; pencere çizmek TR'nin işi değildir.

TR'de HTTP sunucusu henüz yok, bu yüzden ekran bugün **sahte olay akışıyla** çalışır ve bunu üstte sürekli yazar. Ekrandaki her şey uydurmadır. Yer geçicidir: asistanın TR kodu gelince taşınır.

## Dosyalar

- `index.html`: ekran. Biçim, sözlük, kod ve sahte akış bu tek dosyada.
- `contract.md`: ekranın TR'den beklediği her şey. Asıl değer budur; HTTP bölümü yazılırken tartılır.
- `behavior_screen_test.go`: aşağıdaki kuralları korur.

## Açmak

`index.html`'i tarayıcıda açın. Sahte akış kendi başlar. Üstteki **Sahte sahne** seçicisi sözleşmenin bütün durumlarını oynatır: onay, süre dolması, bağlantı kopması, sunucunun yeniden başlaması, uzun çıktı, kötü niyetli metin.

Gerçek kaynağa bağlanmak için `?source=http://127.0.0.1:<kapı>` verilir. Yalnız bu biçim kabul edilir ve sayfayı TR sunmalıdır; `file://` yalnız sahte kipte çalışır (`contract.md`, "Kaynak").

## Yapı

- **Tek yol:** sahte akış da gerçek SSE baytları üretir ve aynı ayrıştırıcıdan geçer; çerçeveler bölünmüş gelir ki UTF-8 sınırı da sınansın. Ekran kodu iki kaynağı ayırmaz; yalnız sahne seçicisi sahte kaynağa bakar. İkisinin farkı `fetchTransport` ve `createMockTransport`'tur, ikisi de `open` ve `post` verir.
- **Onay kartı akışın dışındadır:** kendi yuvasında durur, yalnız `approval_open` olayından gelir. Model metni o yuvaya ulaşamaz, en fazla ona benzeyen düz yazı olur. Kart `approval_close` gelmeden kalkmaz. Onayla, kart açıldıktan yarım saniye sonra etkinleşir; varsayılan odak Vazgeç'tedir ama yazı yazılırken odak çalınmaz.
- **Görünmez karakter:** kart ve araç adı yön ve denetim karakterlerini `[U+202E]` gibi yazar.
- **Güvenilmez girdi:** model metni, araç adı ve çıktısı yalnız `textContent` ile basılır.
- **Sınırlar:** dosyanın başındaki sabitler. Akış 400 satırda, bir metin 120 000 karakterde, gösterilen araç çıktısı 24 000 karakterde durur ve bunu söyler.

## Dil eklemek

Ekranda dil ayrıcalığı yok; varsayılan tarayıcının dilidir, eşleşme yoksa sözlükteki ilk dil, seçim hatırlanır. Dil eklemek yalnız veridir:

1. `index.html`'deki `dictionary` bloğuna dil koduyla yeni bir nesne ekleyin. Anahtarlar öbür dillerle aynı olmalı, `{ad}` boşlukları da.
2. `language_name` dilin kendi adıdır, `direction` `ltr` ya da `rtl`'dir.

Dil seçicisi kendini sözlükten kurar. Sayılar, boyutlar ve süreler `Intl` ile dile göre yazılır. Biçim mantıksal özelliklerle yazıldığı için sağdan sola yazılan dil düzeni kendiliğinden çevirir; metin uzunluğu sabit genişliğe bağlı değildir. HTML'in içine yazı yazılmaz, `<title>` dahil.

Sözlüğe eklenen dil CSP özetini değiştirmez: JSON bloğu çalıştırılmaz.

## Değiştirmek

Betik ya da stil değişince CSP etiketindeki özet de değişmelidir, yoksa tarayıcı sayfayı çalıştırmaz. `TestScreenPolicy` düşerken beklenen özeti yazar: onu etikete kopyalayın. Özet, `<script>` ile `</script>` arasındaki baytların `sha256`'sının base64 yazımıdır.

Ekranda görünen ad yalnız `ASSISTANT_NAME` sabitindedir.

## Sınamak

Testler tarayıcı açmaz; dosyaları okuyup kuralları denetler:

```bash
go test ./tools/assistant
```

Davranış tarayıcıda sahnelerle gezilir: konsolda hata olmamalı, 1280 ve 800 genişlikte düzen bozulmamalı.

## Testler

- `behavior_screen_test.go`: `innerHTML` yok, dış adres yok, CSP var ve özetleri tutuyor, boyut 150 KB altı, görünmez karakter yok; sözleşmedeki her olay, komut ve durum ekranda ve sahte akışta; dillerde aynı anahtarlar ve boşluklar var, her hata kodunun her dilde karşılığı var.
