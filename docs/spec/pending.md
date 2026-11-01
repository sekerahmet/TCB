# Dil tanımının bekleyen bulguları

Dil tanımı (`docs/spec/NN_*.md`) ve gerekçesi (`docs/spec/rationale/`) projenin ölçütüdür; seyrek ve toplu değişir. Uygulama sırasında dil tanımıyla çelişen ya da onun sustuğu yeri gösteren bulgular burada toplanır (ana oturum ve DENETİM, 2026-11-01). Bu dosya kural değildir; kural yalnız bölüm dosyalarındadır.

**Sınıflar:**
- **Kritik:** sessiz yanlış, güvenlik, CLAUDE.md ilkesine aykırılık, geri alınamaz sonuç (disk biçimi, kullanıcıya görünen ad), ya da bir uygulama adımı ona bağlı ve dil tanımı ne yapılacağını söylemiyor. Ana oturum aynı gün karar verir ve maddeyi yazar; satır burada "geçici, turda denetlenecek" diye durur.
- **Ara:** maddenin örneği ya da gerekçenin emsali. Ana oturum aynı gün düzeltebilir; satır burada görünür.
- **Olağan:** öbürleri. Dil tanımı turuna kadar bekler; tura girmemiş olağan bulguya göre uygulama yapılmaz.

**Dil tanımı turu** faz sonunda, 5 olağan bulgu birikince ya da aynı bölümde 3 bulgu olunca gerekir; etkilenen bölümleri uygulayan açık dal kalmayınca yapılır: ana oturum yazar, Gemini kararı araştırır, DENETİM doğrular. Tur bitince satırlar silinir. *Test:* satırları bu listeye girmez (uygulama günceller), ama bir kuralı gevşetemez.

## Geçici kararlar (dil tanımına yazıldı, turda denetlenecek)

Uygulama sırasında çıkan çelişki ya da dil tanımının sustuğu yer için ana oturumun kararı aynı gün dil tanımına yazılır; denetimi sıradaki dil tanımı turunda (Gemini ve DENETİM) yapılır. 2026-10-28, 2. katmandan: 3.7a (açmada `TR_List(j)`, karşılaştırma, dolaşma), 3.8 (`.TR_reverse()` liste verir), 3.10 (koşulun geçtiği öbür yerler), 3.12 (mantık değerinden sayı, metinden liste kurulmaz), 3.14 (yön karakterleri, kaptaki JSON, bayt dizisinin metni), 4.5 (fonksiyon içeren kap, isteğe bağlı fonksiyon ve `TR_none`), 4.9 (ifadenin ardındaki `TR_in`), 5.7 (dolaşılamayan JSON türleri), 6.6 (`_` parametre), 12.3 (koşul `TR_Bool`). Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar o turda bağımsız denetlenir.

2026-10-28, 1–9'un amacın dokuz işine karşı tartısından (ana oturum ve DENETİM, `bolum1-9-dokuz-is`): 2.4 (onaltılık yazım bit deseni), 3.1 ve 13.1 (sarılan aritmetik), 3.6a (demet; 6.3'ün birden çok dönüşü demet, H2708 kalkar), 3.7 (demet anahtar), 3.7a (yinelenen JSON anahtarı hata), 4.3 (kesirli üs her makinede aynı), 4.12 ve 13.1 (`TR_unsigned_shift_right`), 5.7 ve 7.8 (`TR_next` ile dolaşma), 5.11 (sözlük üreteci), 13.8 (yapının JSON'u iki yönlü). Uygulanmadılar; bugün: onaltılık hash sabiti sözdizimi hatası ve iletisi ".0 ekle" diyor, hash çarpımının taşma iletisi ondalık öneriyor, yapı dolaşılamıyor, sözlük üreteci "expected a value" veriyor [ölçüldü]. Hepsi 4a'dan önce ya da 4a ile.

- **13.3, kritik (2026-11-01, D1):** "Anahtar fonksiyonu her öğe için bir kez çağrılır." Anahtarlı sıralamanın uygulanması bu cümleye bağlıydı; ana oturum aynı gün karar verdi (D1'in dalında, birleşince main'de).

## Ara

- **11.8'in örneği `m.nokta.x = 1` (2026-11-01, D1):** `nokta` bir kodun adı (`TR_mem_checkpoint`); status'ta aynı örnek `TestReviewedSectionsWriteIdentities`'i düşürdü. Öneri: `m.yer.x = 1`. D1 birleşince düzeltilir.

## Olağan

- **8.5, "nedenler zincirinde dıştan içe ilk kod" (2026-11-01, D1):** `TR_error_code`'un uygulaması `messages.Root` ile en içteki kodu veriyor [kanıtlı: `internal/lang/messages/error.go:150`]; madde dıştan içe ilk kodu söylüyor (`docs/spec/08_hata_yonetimi.md:45`). 8.2'nin `TR_cause`'u gelince bakılır.
