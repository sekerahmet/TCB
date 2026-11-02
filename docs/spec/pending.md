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
- **3.7a, geçici (2026-11-01, 4a'nın üçüncü diliminin yönergesi, `f85743e4`):** "Açmak"a `TR_Dict(j)` → `TR_Dict[TR_String, TR_JSON]`; "Kurmak"ta değişmez olmayan her liste ve sözlük (değişken, alan, çağrı) denetim hatası; `TR_JSON(x)`'e yalnız JSON türleri, kapalı küme, tipi bilinen öğede denetim hatası (`4b5053f8`, DENETİM'in iki ekiyle), tek değer de (`TR_JSON(3)`; çalışma anı zaten kabul ediyor, H2705 öneriyor [ölçüldü: `d93648ab`'nin ikilisi]). Kritik, çünkü H2403'ün önerisi ve 4a3'ün 2c adımı buna bağlıydı ve dil tanımı söylemiyordu. DENETİM'in yönerge incelemesinden çıktı (`kol-denetim-4a-dilim3-yonerge-sonuc.md` 3 ve 4; metni DENETİM örnekledi, istek 17 B ve C); Gemini'nin araştırmasından geçmedi.
- **3.14 ve 8.3, geçici (2026-11-01, D1'in ikinci dilimi):** iletinin değerindeki dil adı programdaki yazımıyla (`"TR_requires_grad"`), sınıflı iç yazım girmez; sarılmış çalışma hatasında `e.TR_value` en içteki nedenin iletisi. Kritik, çünkü D1'in uygulaması bu iki yerde dil tanımının söylemediğini seçmek zorundaydı; DENETİM 3.14'ü erken düzeltilecek ara bulgu, 8.3'ü doğru saydı (`kol-denetim-d1-dilim2-kod-sonuc.md`). Gemini'nin araştırmasından geçmedi.

## Ara



## Olağan
- **3.11, parametresiz kap tipinin eşitliği (2026-11-02, DENETİM'in 4a3 birleştirme incelemesi):** H1134 `TR_Type(j) == TR_Dict`'i öneriyor ve bu çalışıyor (`TR_Type([1]) == TR_List` de `TR_true`); ama 3.7a nesnede `TR_Type`'ın `TR_Dict[TR_String, TR_JSON]` verdiğini, 3.11 tip değerlerini parametreli yazıyor. Parametresiz `TR_Dict`'in parametreli tipe eşit olup olmadığı yazılı değil; dil tanımının yazımı `TR_Type(j["a"]) == TR_List[TR_JSON]` denetimden geçiyor, çalışırken "Tip tipi indekslenemez" veriyor [ölçüldü: DENETİM, `kol-denetim-istek21-sonuc.md` 5]. Öneri: ya 3.11 "parametresiz kap tipi her parametreli hâline eşittir" der (motorun davranışı, `tests/tipler_test.tr`'nin `TR_Type(j) == TR_List`'i), ya da öneri parametreli yazılır ve çalışma anı düzelir.
- **3.7a "Karşılaştırmak", önerinin sırası (2026-11-02, aynı inceleme):** H1131 önce açmayı (`TR_Int(j["k"]) == 3`) önerir; JSON'da sayı `3.0` yazılmışsa bu yazım çalışırken düşer, çevirme (`j["k"] == TR_JSON(3)`) 4.5'in sayı kuralıyla `TR_true` verir [ölçüldü: DENETİM, aynı not 3]. Düşüş gürültülü, sessiz değil. Öneri: dil tanımı çevirmeyi ilk öneri yapar.
- **15.55, nicemlenmiş tensörü açmak (2026-11-01, T1'in ilk dilimi):** 15.55 "ileti açmayı ya da TR biçimini önerir" diyor, ama açma yerleşiği dil tanımında ve sözlükte yok; H6950 yalnız TR biçimini öneriyor [kanıtlı: T1, `t1-dilim1-devir.md`; `grep -n quant internal/lang/codes/definitions.json` boş]. Öneri: açma yerleşiği ya adlandırılır ya da 15.55'ten "açmayı" çıkar.
- **15.2, aralıktan tensör (2026-11-01, T1'in ilk dilimi):** `TR_Tensor(0..3)` bugün H6032; 15.2 aralıktan kurmayı söylemiyor, 15.1'in ara kuralı ise "tam sayı listesi ya da aralık" diyor [kanıtlı: T1, `t1-dilim1-devir.md`, ölçüm]. Öneri: 15.2 aralığı kabul eder ya da 15.1'in cümlesinden "aralık" çıkar.
- **15.47a, `__metadata__` adı (2026-11-01, T1'in ilk dilimi):** safetensors başlığında `__metadata__` ayrılmış bir anahtardır; bu adlı bir tensörü yazmak T1'de çalışma hatası (H6951), ama 15.47a bunu yazmıyor [kanıtlı: T1, `t1-dilim1-devir.md`]. Öneri: 15.47a'ya bir cümle.
- **15.46, uzantının harfi (2026-11-01, T1'in ilk dilimi):** biçim uzantıya göre seçilir, ama `.SAFETENSORS` gibi büyük harfli uzantının ne olacağı yazılı değil [kanıtlı: T1, `t1-dilim1-devir.md`]. Öneri: 15.46'ya harfe duyarlılık için bir cümle.
