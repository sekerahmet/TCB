# Dil tanımının bekleyen bulguları

Dil tanımı (`docs/spec/NN_*.md`) ve gerekçesi (`docs/spec/rationale/`) projenin ölçütüdür; seyrek ve toplu değişir. Uygulama sırasında dil tanımıyla çelişen ya da onun sustuğu yeri gösteren bulgular burada toplanır (ana oturum ve DENETİM, 2026-11-01). Bu dosya kural değildir; kural yalnız bölüm dosyalarındadır.

**Sınıflar:**
- **Kritik:** sessiz yanlış, güvenlik, CLAUDE.md ilkesine aykırılık, geri alınamaz sonuç (disk biçimi, kullanıcıya görünen ad), ya da bir uygulama adımı ona bağlı ve dil tanımı ne yapılacağını söylemiyor. Ana oturum aynı gün karar verir ve maddeyi yazar; satır burada "geçici, turda denetlenecek" diye durur.
- **Ara:** maddenin örneği ya da gerekçenin emsali. Ana oturum aynı gün düzeltebilir; satır burada görünür.
- **Olağan:** öbürleri. Dil tanımı turuna kadar bekler; tura girmemiş olağan bulguya göre uygulama yapılmaz.

**Dil tanımı turu** faz sonunda, 5 olağan bulgu birikince ya da aynı bölümde 3 bulgu olunca gerekir; etkilenen bölümleri uygulayan açık dal kalmayınca yapılır: ana oturum yazar, Gemini kararı araştırır, DENETİM doğrular. Tur bitince satırlar silinir. *Test:* satırları bu listeye girmez (uygulama günceller), ama bir kuralı gevşetemez.

## Geçici kararlar (dil tanımına yazıldı, turda denetlenecek)

Uygulama sırasında verilip aynı gün dil tanımına yazılan kararlar. DENETİM'den geçtiler; Gemini'nin karar araştırmasından geçmediler. Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar turda bağımsız denetlenir.
Dokuz iş tartısının kararlarından bir kısmı 2026-10-28'de uygulanmamıştı: onaltılık hash sabiti sözdizimi hatasıydı ve iletisi ".0 ekle" diyordu; hash çarpımının taşma iletisi ondalık öneriyordu; yapı dolaşılamıyordu; sözlük üreteci "expected a value" veriyordu [ölçüldü: 2026-10-28]. Bugünkü durumları status'ta.

- **2.4, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** onaltılık yazım bit deseni.
- **3.1, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** sarılan aritmetik.
- **3.6a, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** demet; 6.3'ün birden çok dönüşü demet, H2708 kalkar.
- **3.7, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** demet anahtar.
- **3.7a, geçici (2026-10-28, 2. katman):** JSON'un açılmasında `TR_List(j)`, karşılaştırma, dolaşma.
- **3.7a, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** yinelenen JSON anahtarı hata.
- **3.8, geçici (2026-10-28, 2. katman):** `.TR_reverse()` liste verir.
- **3.10, geçici (2026-10-28, 2. katman):** koşulun geçtiği öbür yerler.
- **3.12, geçici (2026-10-28, 2. katman):** mantık değerinden sayı, metinden liste kurulmaz.
- **3.14, geçici (2026-10-28, 2. katman):** yön karakterleri, kaptaki JSON, bayt dizisinin metni.
- **4.3, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** kesirli üs her makinede aynı.
- **4.5, geçici (2026-10-28, 2. katman):** fonksiyon içeren kap, isteğe bağlı fonksiyon ve `TR_none`.
- **4.9, geçici (2026-10-28, 2. katman):** ifadenin ardındaki `TR_in`.
- **4.12, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** `TR_unsigned_shift_right`.
- **5.7, geçici (2026-10-28, 2. katman):** dolaşılamayan JSON türleri.
- **5.7, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** `TR_next` ile dolaşma.
- **5.11, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** sözlük üreteci.
- **6.6, geçici (2026-10-28, 2. katman):** `_` parametre.
- **7.8, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** `TR_next` ile dolaşma.
- **12.3, geçici (2026-10-28, 2. katman):** koşul `TR_Bool`.
- **13.1, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** sarılan aritmetik.
- **13.1, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** `TR_unsigned_shift_right`.
- **13.8, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** yapının JSON'u iki yönlü.
- **13.3, kritik (2026-11-01, D1):** "Anahtar fonksiyonu her öğe için bir kez çağrılır." Anahtarlı sıralamanın uygulanması bu cümleye bağlıydı; ana oturum aynı gün karar verdi (D1'in dalında, birleşince main'de).
- **10.2, kritik (2026-11-01, D1'in 10.2 ölçümü, `d1-10-2-olcum.md`):** eski söz "kilidi bekleyen bir görev en çok 10 ms içinde kilidi alır" 5 ms'lik devirle üç ve daha çok meşgul görevde tutamazdı ((k − 1) × 5 ms) [hesap]; liste ve sözlük metotlarının istisnası sözde yazılı değildi. Ana oturum aynı gün karar verdi: kilit geliş sırasıyla alınır; tutan görev bekleyen varken en çok 5 ms ve o anki deyimin sonuna kadar tutar; önünde n görev bekleyen görev en çok (n + 1) × 5 ms ve n + 1 deyim bekler; liste, sözlük ve metin yerleşikleri sözün dışında. Geçici, turda denetlenecek.

## Ara

- **11.8'in örneği `m.nokta.x = 1` (2026-11-01, D1):** `nokta` bir kodun adı (`TR_mem_checkpoint`); status'ta aynı örnek `TestReviewedSectionsWriteIdentities`'i düşürdü. **Düzeltildi** (2026-11-01, D1 birleştikten sonra): `m.yer.x = 1`.

- **18.2'nin örneği (2026-11-01, Gemini Faz 1 paralel):** "`.TR_parent` bulunduğu klasörün yoludur (tek bileşenli göreli yolda `.`, `.`'da `..`)". `..`'nın üst klasörü yazılmamış; uygulama ana cümleden `../..` veriyor (eskiden `.`, sessiz yanlış) [kanıtlı: `internal/stdlib/files/path.go:117-125`; test: `tests/dosya_ve_yol_test.tr`]. Öneri: örneğe "`..`'da `../..`" eklenir.

## Olağan

- **8.5, "nedenler zincirinde dıştan içe ilk kod" (2026-11-01, D1):** `TR_error_code`'un uygulaması `messages.Root` ile en içteki kodu veriyor [kanıtlı: `internal/lang/messages/error.go:150`]; madde dıştan içe ilk kodu söylüyor (`docs/spec/08_hata_yonetimi.md:45`). 8.2'nin `TR_cause`'u gelince bakılır.
- **16.5, `int32` sınırı (2026-11-01, Gemini Faz 1 paralel):** madde "işaretsiz 16 ya da 32 bit, işaretli 32 ya da 64 bit" okunur diyor [kanıtlı: `docs/spec/16_veri_parcalayici.md:40`]; `int32`'ye sığmayan `<u4` ve `<i8` değeri okumada H5086 ile reddediliyor [kanıtlı: `internal/compute/dataload/shard.go:278`]. Gürültülü, sessiz yanlış değil; dil tanımı sınırı söylemiyor. Gemini'nin raporu bu sınırı dil tanımına mal etti, metinde yok. Öneri: turda ya sınır maddeye yazılır ya da okuyucu `int64`'e geçer (16.7'nin tensörü zaten `int64`).
