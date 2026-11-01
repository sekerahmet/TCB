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

## Ara

- **11.8'in örneği `m.nokta.x = 1` (2026-11-01, D1):** `nokta` bir kodun adı (`TR_mem_checkpoint`); status'ta aynı örnek `TestReviewedSectionsWriteIdentities`'i düşürdü. Öneri: `m.yer.x = 1`. D1 birleşince düzeltilir.

## Olağan

- **8.5, "nedenler zincirinde dıştan içe ilk kod" (2026-11-01, D1):** `TR_error_code`'un uygulaması `messages.Root` ile en içteki kodu veriyor [kanıtlı: `internal/lang/messages/error.go:150`]; madde dıştan içe ilk kodu söylüyor (`docs/spec/08_hata_yonetimi.md:45`). 8.2'nin `TR_cause`'u gelince bakılır.
