# 22. Bayt ve sayı

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "22. Bayt ve sayı".

Her kuralın emsali ve gerekçesi: [`rationale/22_bytes.md`](rationale/22_bytes.md).

Ses, algılayıcı paketi, ikili veri parçası ve dosya başlığı sayıları bayt olarak taşır. Bu bölüm baytlardan (`TR_Bytes`, 3.3a) sayı ve tensör okumayı, sayıyı ve tensörü bayta yazmayı ve ses dosyasını söyler. Tensörün cinsleri ve hassasiyetleri 15.1'dedir.

**22.1** `TR_from_bytes[E](b, TR_dtype: ad, TR_byte_order: sıra)` baytları okuyup tek boyutlu yeni bir `TR_Tensor[E]` kurar.
- `E` ve `TR_dtype:` zorunludur. Hassasiyet `E`'nin 15.1'deki hassasiyetlerinden biridir. Başka cinsin hassasiyeti hatadır: `TR_from_bytes[TR_Float](b, TR_dtype: "int16", ...)` değişmezle yazılınca denetimde, değişkenle verilince çalışırken. Nicemlenmiş saklama hassasiyetleri (15.51–15.55) buradan kurulmaz.
- Çok baytlı hassasiyette `TR_byte_order:` zorunludur: `"little"` ya da `"big"`. Makinenin kendi sırası diye bir seçenek yoktur. Tek baytlı hassasiyette ("int8", "uint8", "bool") gerekmez.
- `b`'nin uzunluğu öğe boyunun katı değilse hatadır; ileti uzunluğu ve öğe boyunu söyler. Artık bayt sessizce atılmaz (15.49). Akışta parçanın sonunda kalan yarım sayı da böyle görünür.
- "bool"da 0 ve 1 dışındaki bayt hatadır.
- Konum argümanı yoktur: baytın bir kısmı dilimle okunur (`b[44..b.TR_length]`, 3.3a). Konumun hizalı olması gerekmez.
- Sonuç baytların kopyasıdır.

```tr
# TR 4
TR_var b = TR_Bytes([1, 0, 255, 255])
TR_print(TR_from_bytes[TR_Int](b, TR_dtype: "int16", TR_byte_order: "little").TR_tolist()) # [1, -1]
```

- *Test:* TR testi; Go: `FuzzFromBytes`.

**22.2** `t.TR_to_bytes(TR_byte_order: sıra)` tensörün öğelerini kendi hassasiyetinde ve satır sırasıyla (son eksen en hızlı değişir) bir `TR_Bytes`'a yazar. Her şekil yazılır.
- `TR_byte_order:` 22.1'deki gibidir.
- Nicemlenmiş tensör yazılamaz; önce açılır (15.52).
- Sonuç türevlenmez.
- *Test:* TR testi; Go: `FuzzReadNumber`.

**22.3** Tek sayı:
- `b.TR_read_int(konum, TR_dtype: ad, TR_byte_order: sıra)` `konum`'daki bayttan başlayarak bir `TR_Int`, `b.TR_read_float(...)` bir `TR_Float` okur. Hassasiyet ve bayt sırası 22.1'deki gibidir. Eksi konum ya da baytların dışına taşan aralık hatadır.
- `TR_int_to_bytes(n, TR_dtype: ad, TR_byte_order: sıra)` ve `TR_float_to_bytes(x, ...)` tek bir sayıyı `TR_Bytes`'a yazar.
  - Hassasiyete sığmayan tam sayı hatadır (15.2): `TR_int_to_bytes(300, TR_dtype: "uint8")`.
  - Ondalık sayı hassasiyetin en yakın değerine yuvarlanır (15.2).

```tr
# TR 4
TR_var başlık = TR_read_bytes("soru.wav")[0..44]
TR_print(başlık.TR_read_int(24, TR_dtype: "uint32", TR_byte_order: "little")) # örnekleme hızı: 16000
```

- *Test:* TR testi.

**22.4** Ses dosyası `tr/audio` kitaplığındadır (11.6). `TR_read_wav(yol)` bir WAV dosyasını okuyup `TR_Sound` verir:
- `.TR_samples`: `TR_Tensor[TR_Float]`, şekli `[kanal, örnek]`; 16 bit tam sayıda değerler [-1, 1) aralığındadır, 32 bit ondalıkta dosyadaki değerdir;
- `.TR_sample_rate`: `TR_Int`, saniyedeki örnek sayısı.
- `TR_Sound` bir yapıdır (7.1); `TR_Sound(TR_samples: t, TR_sample_rate: r)` ile de kurulur (7.2). `t` iki boyutlu değilse, kanal sayısı 0 ise ya da `r` pozitif değilse hatadır.
- Dosyanın parçaları sırayla okunur (`fmt `, `data`; `LIST` ve öbürleri atlanır). Başlığın 44 bayt olduğu varsayılmaz. Tek boylu parçanın sonundaki dolgu baytı atlanır; son parçada yoksa hata değildir. `fmt ` ve `data` aranırken `RIFF` boyuna bakılmaz. `data`'dan sonra tarama `RIFF` zarfının sonuna ya da dosya sonuna, önce gelenine varınca kalana bakar: kalan yoksa, 8 bayttan kısaysa, hepsi sıfırsa ya da `TAG` ile başlayan 128 baytsa tarama biter; öbür her kuyrukta bir parça okunur ve kalana yine bakılır, bozuksa hatadır. `data`'dan sonraki parçanın adı yazdırılabilir ASCII değilse parça değildir: kalan hepsi sıfırsa tarama biter, değilse hatadır.
- Biçim önce denetlenir: desteklenmeyen biçim `ffmpeg` iletisini verir, aşağıdaki sebeplerden birini değil. Bozuk dosya hatadır, sessizce kısa ses olmaz (15.49); ileti sebebi söyler: `RIFF` ya da `WAVE` yok; `fmt ` ya da `data` yok, `data` `fmt `'ten önce ya da ikisinden biri iki kez; `fmt ` 16 bayttan, `WAVE_FORMAT_EXTENSIBLE`'da 40 bayttan kısa; kanal sayısı ya da örnekleme hızı 0; blok hizası kanal ve örnek boyunu tutmuyor; bir parçanın boyu dosyada kalandan büyük; `data` boyu 0 ya da kare boyunun katı değil. `data`'dan sonraki kesik parça da hatadır. Boruya yazılmış dosyanın boyu bilinmez: çoğu araçta boy dosyadan büyük görünür, Python `wave`'de ilk yazılan boy kalır ve sesin arkası kuyruk gibi görünür. Parça boyu dosyada kalandan büyükse ya da parça adı geçersizse ileti dosyaya yazmayı önerir.
- 32 bit ondalıkta NaN ve sonsuz hatadır; ileti ilk böyle örneğin indeksini söyler. Aralık dışı sonlu değer olduğu gibi gelir.
- Okunan biçimler: 16 bit tam sayı PCM, 32 bit ondalık ve bunların `WAVE_FORMAT_EXTENSIBLE` biçimi; orada alt biçim bu ikisinden biridir, geçerli bit sayısı örnek boyuna eşit ya da 0'dır. Öbürleri (8, 24 ve 32 bit tam sayı PCM, sıkıştırılmış ses, `RIFX`, `RF64`) hatadır. İleti `ffmpeg` ile 16 bite çevirmenin kabuk yazımını verir: `ffmpeg -nostdin -i girdi -c:a pcm_s16le çıktı.wav`.
- 16 bit tam sayı 32768'e bölünür: tam sayı ses ondalığa `TR_Tensor[TR_Float](ham) / 32768.0` ile çevrilir.
- `TR_write_wav(yol, ses)` 16 bit tam sayı PCM yazar. Ondalık değer 32767 ile çarpılır ve en yakın tam sayıya yuvarlanır; yarım çifte gider (13.1, 15.16a). Boş ses hatadır: `TR_read_wav` onu geri okuyamaz.
  - [-1, 1] dışındaki ve NaN örnek hatadır: taşan bir ses sessizce kırpılmaz (15.49). İleti ilk böyle örneğin indeksini ve değerini söyler. Aralık dışında kırpmanın açık yazımını önerir: `TR_clamp(x, TR_min: -1.0, TR_max: 1.0)`; NaN ve sonsuzda önermez, çünkü `TR_clamp` NaN'ı değiştirmez, sonsuzu sessizce sınıra indirir (15.13); yukarıdaki bir hesabın bozulduğunu söyler.
  - Okuma 32768'e böler, yazma 32767 ile çarpar: okunup yeniden yazılan dosyanın bazı örnekleri bir birim değişir, kopya kayıpsız değildir.
- Yeniden örnekleme kitaplıkta yoktur; `ffmpeg` ile yapılır (21.1'in örneğindeki `-ar 16000`).

```tr
# TR 4
TR_import "tr/audio"
TR_var ses = TR_read_wav("soru.wav")
TR_print(ses.TR_sample_rate, ses.TR_samples.TR_shape)
TR_var tek_kanal = ses.TR_samples[0] # şekli [örnek]
```

- *Test:* Go: `TestWavMessagesGiveTheFix`, `FuzzReadWav`; kitaplık testi `lib/audio_test.tr`.
