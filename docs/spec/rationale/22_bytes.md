# 22. Bayt ve sayı: emsal ve gerekçe

Kurallar: [`22_bayt_ve_sayi.md`](../22_bayt_ve_sayi.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

Araştırma: Gemini Jarvis A (2026-11-02, taban `b2892d87`). Raporu DENETİM doğruladı (istek 36). Kaynak etiketleri 21'in gerekçe dosyasındaki gibidir: **[DENETİM açtı]** ve **[rapordan]**.

Karşılaştırma:

| Konu | Python | Go `encoding/binary` | Rust | Başka | TR |
| :-- | :-- | :-- | :-- | :-- | :-- |
| Toplu okuma | yok (`array`) | `binary.Read`, yansımalı | `bytemuck::cast_slice`, hizalama ister | NumPy `frombuffer(b, dtype='<i2')` | `TR_from_bytes[E]` (22.1) |
| Bayt sırası | `struct` varsayılanı makine sırası; `int.from_bytes` 3.11'den beri `'big'` | zorunlu, varsayılan yok | `from_le_bytes`, `from_be_bytes`, `from_ne_bytes`; varsayılan yok | NumPy `'='` makine sırası | zorunlu, makine sırası yok |
| Artık bayt | `struct.error` | kısa dilimde panik | uymazsa hata | NumPy `ValueError` | hata (22.1) |
| Tek sayı | `struct.unpack_from` | `binary.LittleEndian.Uint16(b)` | `i16::from_le_bytes` | | `.TR_read_int`, `.TR_read_float` (22.3) |
| `int16` → ondalık | yok | yok | yok | Whisper, libsndfile, librosa: `/ 32768` | `/ 32768.0` (22.4) |

### 22.1

- *Emsal:* Go'nun `encoding/binary`'si ve Rust'ın `from_le_bytes`/`from_be_bytes`'ı bayt sırasını her zaman yazdırır [rapordan: pkg.go.dev/encoding/binary; doc.rust-lang.org/std/primitive.i16.html]. NumPy'nin `frombuffer`'ı öğe boyunun katı olmayan uzunlukta `ValueError` verir; varsayılan sırası makinenindir [rapordan: numpy.org, numpy.frombuffer]. Rust'ın `from_ne_bytes`'ı makinenin sırasıdır [rapordan]. *Makine sırasını almadık.*
- *Gerekçe:* aynı veriyle aynı sonuç (CLAUDE.md "Güvenilir"): makinenin sırası bir dosyayı bir makinede doğru, başka bir makinede yanlış okutur. Robotta büyük uçlu bir algılayıcı paketini varsayılan sırayla okumak sessiz bir yanlıştır. Python'da `int.from_bytes` ile NumPy'nin varsayılanları birbirinin tersidir [rapordan]; varsayılan olmayınca ajan hangisi olduğunu hatırlamak zorunda kalmaz. Tip parametresi zorunludur, çünkü sonucun tipi argümanın değerinden çıkamaz (13.10); hassasiyet zorunludur, çünkü öğe boyunu o belirler. Artık baytın hata olması 15.49'un "sessiz kırpma yok" kuralıdır. Konum argümanının olmaması bir yazımı (dilim) bırakır.

### 22.2

- *Emsal:* NumPy `tobytes`, PyTorch tensörün bellek düzeni [kanıtsız: bu tur açılmadı].
- *Gerekçe:* 22.1'in tersi aynı kurallarla: bir dosyaya ya da başka bir araca verilen bayt hangi makinede yazılırsa yazılsın aynıdır. Satır sırası tensörün yazılış sırasıdır (iç içe listedeki gibi).

### 22.3

- *Emsal:* Python `struct.unpack_from` ve `int.from_bytes`, Go `binary.LittleEndian.Uint32`, Rust `u32::from_le_bytes` [rapordan].
- *Gerekçe:* dosya başlığı ve algılayıcı paketi birkaç ayrı sayıdan oluşur. Her biri için tek öğeli bir tensör kurup açmak doğru ama dolambaçlıdır, ve ajan için gereksiz bir hata yeridir. Sığmayan değerin hata olması 15.2'nin kuralıdır.

### 22.4

- *Emsal:* Whisper'ın `load_audio`'su `ffmpeg`'den `s16le` alır ve `/ 32768.0` ile böler [DENETİM açtı: github.com/openai/whisper, `whisper/audio.py`, v20250625]. libsndfile okurken 32768'e böler, yazarken 32767 ile çarpar [DENETİM açtı: libsndfile FAQ, S10]. Python'un `wave`'i ham bayt verir ve 32 bit ondalık WAV okumaz; `soundfile` ve `torchaudio.load` [-1, 1) ondalık verir [rapordan]. Şekil `[kanal, örnek]` torchaudio'nunkidir: "If channels_first=True, shape is [channel, time]", varsayılan `channels_first=True` [DENETİM açtı: docs.pytorch.org/audio, torchaudio.load].
- *Gerekçe:* ses okuma Jarvis'in, veriyi hazırlamanın ve konuşmayı yazıya çeviren her modelin girdisidir. Bayttan sayı yerleşikleri onu yazmaya yeter; bu yüzden okuyucu yerleşik değil, TR ile yazılmış bir kitaplıktır (11.6). Ajanın en sık yanlışları [rapordan]:
  - başlığı 44 bayt sanıp `LIST` parçasını ses diye okumak;
  - 32767'ye bölüp -32768'de -1'i aşmak ve Whisper'dan ayrılmak.
  - Okuyucu ikisini de kaldırır.
- İlk sürümün biçimleri Jarvis'in zinciridir (`arecord -f S16_LE`, Whisper'ın `s16le`'si) [DENETİM açtı: whisper `cmd`]. 24 bit reddedilir, çünkü 15.1'de "int24" yoktur ve dönüştürme yolu (`ffmpeg`) zaten bağımlılıktır.
- Yazarken 32767 ile çarpmak, 1.0'ın "int16"ya sığmasını sağlar (15.2'nin "sığmak" kuralı). Aralık dışı örnek hatadır, çünkü taşan ses bir modelin ya da kazancın taştığını gösterir; sessizce kırpmak 15.49'a aykırıdır (DENETİM istek 37, Ş1). Ölçek iki yönde aynı değildir; libsndfile de böyledir. Yuvarlamanın en yakına olması bir seçimdir [hipotez: libsndfile'ın yuvarlama yöntemi açılmadı].
- Yeniden örnekleme kitaplıkta yoktur, çünkü Whisper'la aynı sonuç `ffmpeg`'in kendi yeniden örneklemesiyle alınır [DENETİM açtı: whisper `cmd`'de `-ar`]. Basit bir ara değerleme örtüşmeyle doğruluğu düşürür [rapordan].
