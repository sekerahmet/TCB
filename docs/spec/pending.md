# Dil tanımının bekleyen bulguları

Dil tanımı (`docs/spec/NN_*.md`) ve gerekçesi (`docs/spec/rationale/`) projenin ölçütüdür; seyrek ve toplu değişir. Uygulama sırasında dil tanımıyla çelişen ya da onun sustuğu yeri gösteren bulgular burada toplanır (ana oturum ve DENETİM, 2026-11-01). Bu dosya kural değildir; kural yalnız bölüm dosyalarındadır.

**Sınıflar:**
- **Kritik:** sessiz yanlış, güvenlik, CLAUDE.md ilkesine aykırılık, geri alınamaz sonuç (disk biçimi, kullanıcıya görünen ad), ya da bir uygulama adımı ona bağlı ve dil tanımı ne yapılacağını söylemiyor. Ana oturum aynı gün karar verir ve maddeyi yazar; satır burada "geçici, turda denetlenecek" diye durur.
- **Ara:** maddenin örneği ya da gerekçenin emsali. Ana oturum aynı gün düzeltebilir; satır burada görünür.
- **Olağan:** öbürleri. Dil tanımı turuna kadar bekler; tura girmemiş olağan bulguya göre uygulama yapılmaz.

**Dil tanımı turu** faz sonunda, 5 olağan bulgu birikince ya da aynı bölümde 3 bulgu olunca gerekir; etkilenen bölümleri uygulayan açık dal kalmayınca yapılır: ana oturum yazar, Gemini kararı araştırır, DENETİM doğrular. Tur bitince satırlar silinir. *Test:* satırları bu listeye girmez (uygulama günceller), ama bir kuralı gevşetemez.

## Geçici kararlar (dil tanımına yazıldı, turda denetlenecek)

**Tur 1 bitti (2026-11-01):** 2026-10-28'in 24 geçici kararı ve dört bulgu Gemini'nin araştırmasından (`TUR1_RAPORU.md`, 23 kabul, 5 değiştir) ve DENETİM'in kaynak denetiminden (14/14 alıntı birebir) geçti. Kabul edilenler ve yazılanlar (3.7a karşılaştırması, 18.2'nin örneği, 6.3'ün gerekçesi; 2026-11-01 20:1x'te 3.6a ile 5.3 ve 6.3'ün demet açması, 3.14'ün kaçış ve bayt dizisi şablonu) bu listeden silindi; 8.5 ve 16.5 uygulama işi olarak status'a geçti. Kalan 13.8 aşağıda.

**Tur 2 bitti (2026-11-01):** beş kritik geçici satır (10.2'nin ikisi, 15.47a, 20.5 ve 20.17, 3.14 ve 8.3) ve yedi olağan bulgu Gemini'nin araştırmasından (`TUR2_RAPORU.md`: 4 kabul, 6 öneri kabul, 20.5/20.17 değiştir, 8.5 öneri değiştir) ve DENETİM'in kaynak denetiminden (`kol-denetim-tur2-rapor-sonuc.md`: yerel 13/13, dış 16/16 alıntı birebir; 20.5'i genişletti) geçti; kararlar dil tanımına yazıldı, satırlar silindi, uygulama işleri status'ta. Kalan 13.8 aşağıda.

Uygulama sırasında verilip aynı gün dil tanımına yazılan kararlar. DENETİM'den geçtiler; Gemini'nin karar araştırmasından geçmediler. Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar turda bağımsız denetlenir.
Dokuz iş tartısının kararlarından bir kısmı 2026-10-28'de uygulanmamıştı: onaltılık hash sabiti sözdizimi hatasıydı ve iletisi ".0 ekle" diyordu; hash çarpımının taşma iletisi ondalık öneriyordu; yapı dolaşılamıyordu; sözlük üreteci "expected a value" veriyordu [ölçüldü: 2026-10-28]. Bugünkü durumları status'ta.

- **13.8, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** yapının JSON'u iki yönlü. **Tur 1 (2026-11-01, Gemini `TUR1_RAPORU.md`, DENETİM `kol-denetim-tur1-rapor-sonuc.md`): değiştir; yazılacak.**
- **3.7a, geçici (2026-11-01, 4a'nın üçüncü diliminin yönergesi, `f85743e4`):** "Açmak"a `TR_Dict(j)` → `TR_Dict[TR_String, TR_JSON]`; "Kurmak"ta değişmez olmayan her liste ve sözlük (değişken, alan, çağrı) denetim hatası; `TR_JSON(x)`'e yalnız JSON türleri (`4b5053f8`). Kritik, çünkü H2403'ün önerisi ve 4a3'ün 2c adımı buna bağlıydı ve dil tanımı söylemiyordu. DENETİM'in yönerge incelemesinden geçti (`kol-denetim-4a-dilim3-yonerge-sonuc.md` 3 ve 4); Gemini'nin araştırmasından geçmedi.

## Ara



## Olağan
- **15.47a, `__metadata__` adı (2026-11-01, T1'in ilk dilimi):** safetensors başlığında `__metadata__` ayrılmış bir anahtardır; bu adlı bir tensörü yazmak T1'de çalışma hatası (H6951), ama 15.47a bunu yazmıyor [kanıtlı: T1, `t1-dilim1-devir.md`]. Öneri: 15.47a'ya bir cümle.
- **15.46, uzantının harfi (2026-11-01, T1'in ilk dilimi):** biçim uzantıya göre seçilir, ama `.SAFETENSORS` gibi büyük harfli uzantının ne olacağı yazılı değil [kanıtlı: T1, `t1-dilim1-devir.md`]. Öneri: 15.46'ya harfe duyarlılık için bir cümle.
