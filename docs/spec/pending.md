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

**Tur 3 bitti (2026-11-02):** iki kritik geçici satır (3.7a; 3.14 ve 8.3) ve altı olağan bulgu Gemini'nin araştırmasından (`TUR3_RAPORU.md`: 2 kabul, 4 öneri değiştir, 2 öneri kabul) ve DENETİM'in doğrulamasından (`kol-denetim-istek25-sonuc.md`: 7 ONAY, 3.11 KOŞULLU; dış alıntılar 6/6 birebir) geçti. İki geçici kabul edildi. 3.7a'nın önerisinin sırası bugünkü gibi kaldı (açma önce; çevirme türü tutmayan JSON'da sessiz `TR_false` verir), 15.2 bulgusu yersizdi (15.2 aralığı zaten söylüyor); 15.46, 15.47a ve 15.55 yazıldı, gerekçedeki iki 18.13 göndermesi 18.12 oldu. Satırlar silindi, uygulama işleri status'ta. Kalan 13.8 aşağıda, 3.11 Olağan'da.

Uygulama sırasında verilip aynı gün dil tanımına yazılan kararlar. DENETİM'den geçtiler; Gemini'nin karar araştırmasından geçmediler. Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar turda bağımsız denetlenir.
Dokuz iş tartısının kararlarından bir kısmı 2026-10-28'de uygulanmamıştı: onaltılık hash sabiti sözdizimi hatasıydı ve iletisi ".0 ekle" diyordu; hash çarpımının taşma iletisi ondalık öneriyordu; yapı dolaşılamıyordu; sözlük üreteci "expected a value" veriyordu [ölçüldü: 2026-10-28]. Bugünkü durumları status'ta.

- **13.8, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** yapının JSON'u iki yönlü. **Tur 1 (2026-11-01, Gemini `TUR1_RAPORU.md`, DENETİM `kol-denetim-tur1-rapor-sonuc.md`): değiştir; yazılacak.**

## Ara



## Olağan
- **13.8a ve 15.3, `TR_tolist`'in Türkçe adı (2026-11-02, T1 dilim 2'nin ad sorusu):** adı "değer" ama iş liste vermek; `TR_item`'a "değer" verilemedi, "tek_değer" oldu. Ad turunda tartılır.
- **3.11, parametresiz `TR_Task` (2026-11-02, D1 dilim 3):** denetleyicide dönüşü bilinmeyen çağrının görevi parametresiz `TR_Task`'tır; yalnız iç tiptir, kullanıcı yazımında (`g: TR_Task`) ve tip değerinde (`TR_Type(g) == TR_Task`) yeni kabul açılmadı. 3.11'in göçü yazılırken `TR_Task` de anılır.
- **10.2, gecikme sözünde işletim sisteminin payı (2026-11-02, DENETİM'in D1 ikinci dilim incelemesi):** 10.2'nin gecikme sözü süreyi koşulsuz veriyor. Uygulama işletim sisteminin zamanlamasına bağlı: devredilen görevin uyanması (testte 32 ms pay) ve yük altında zamanlayıcının gecikmesi; o durumda yedek saat devri en çok 64 deyim sınırı geciktirir [kanıtlı: DENETİM, `kol-denetim-istek24-sonuc.md` 3]. Emsal: Python `sys.setswitchinterval` aralığı "ideal" diye yazıyor, söz değil. Öneri: turda söz "işletim sistemi görevleri zamanında çalıştırdığında" diye koşullanır ya da yükteki en kötü durum yazılır; 64 uygulamanın sabitidir, dil tanımına girmez.
- **3.11, parametresiz kap tipinin eşitliği (2026-11-02, DENETİM'in 4a3 birleştirme incelemesi):** H1134 `TR_Type(j) == TR_Dict`'i öneriyor ve bu çalışıyor (`TR_Type([1]) == TR_List` de `TR_true`); ama 3.7a nesnede `TR_Type`'ın `TR_Dict[TR_String, TR_JSON]` verdiğini, 3.11 tip değerlerini parametreli yazıyor. Parametresiz `TR_Dict`'in parametreli tipe eşit olup olmadığı yazılı değil; dil tanımının yazımı `TR_Type(j["a"]) == TR_List[TR_JSON]` denetimden geçiyor, çalışırken "Tip tipi indekslenemez" veriyor [ölçüldü: DENETİM, `kol-denetim-istek21-sonuc.md` 5]. Öneri: ya 3.11 "parametresiz kap tipi her parametreli hâline eşittir" der (motorun davranışı, `tests/tipler_test.tr`'nin `TR_Type(j) == TR_List`'i), ya da öneri parametreli yazılır ve çalışma anı düzelir. **Tur 3 (2026-11-02): KOŞULLU.** Yön: parametresiz `TR_List`/`TR_Dict`/`TR_Task` tip değeri olarak denetim hatası, yalnız parametreli tip değeri geçerli ("her parametreli hâline eşit" `==`'yi geçişsiz yapardı: `List[Int]` ve `List[String]` ikisi de `TR_List`'e eşit olurdu; 2.9). Koşul: çalışma anı bugün tipin parametresini hiç taşımıyor (`TR_Type([1])` `List`, `TR_Type([1]) == TR_Type(["a"])` `TR_true`; boş kapta tip ancak denetleyiciden bilinir) [ölçüldü: DENETİM, `kol-denetim-istek25-sonuc.md` A2]. Madde, `TR_Type(x)`'in parametreyi nereden alacağı (denetimde katlama ya da tip haritası) seçilince yazılır. Göç: `tests/tipler_test.tr`, `definitions.json`'un `TR_Type` örneği, `examples/tr/12_tip_adlari.tr`, H1134'ün önerisi ve öneri tablosundaki satırı (`internal/suggestion_messages_test.go`), `internal/lang/checker/README.md`'nin 3.7a paragrafı.
