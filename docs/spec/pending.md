# Dil tanımının bekleyen bulguları

Dil tanımı (`docs/spec/NN_*.md`) ve gerekçesi (`docs/spec/rationale/`) projenin ölçütüdür; seyrek ve toplu değişir. Uygulama sırasında dil tanımıyla çelişen ya da onun sustuğu yeri gösteren bulgular burada toplanır (ana oturum ve DENETİM, 2026-11-01). Bu dosya kural değildir; kural yalnız bölüm dosyalarındadır.

**Sınıflar:**
- **Kritik:** sessiz yanlış, güvenlik, CLAUDE.md ilkesine aykırılık, geri alınamaz sonuç (disk biçimi, kullanıcıya görünen ad), ya da bir uygulama adımı ona bağlı ve dil tanımı ne yapılacağını söylemiyor. Ana oturum aynı gün karar verir ve maddeyi yazar; satır burada "geçici, turda denetlenecek" diye durur.
- **Ara:** maddenin örneği ya da gerekçenin emsali. Ana oturum aynı gün düzeltebilir; satır burada görünür.
- **Olağan:** öbürleri. Dil tanımı turuna kadar bekler; tura girmemiş olağan bulguya göre uygulama yapılmaz.

**Dil tanımı turu** faz sonunda, 5 olağan bulgu birikince ya da aynı bölümde 3 bulgu olunca gerekir; etkilenen bölümleri uygulayan açık dal kalmayınca yapılır: ana oturum yazar, Gemini kararı araştırır, DENETİM doğrular. Tur bitince satırlar silinir. *Test:* satırları bu listeye girmez (uygulama günceller), ama bir kuralı gevşetemez.

## Geçici kararlar (dil tanımına yazıldı, turda denetlenecek)

**Tur 1 bitti (2026-11-01):** 2026-10-28'in 24 geçici kararı ve dört bulgu Gemini'nin araştırmasından (`TUR1_RAPORU.md`, 23 kabul, 5 değiştir) ve DENETİM'in kaynak denetiminden (14/14 alıntı birebir) geçti. Kabul edilenler ve yazılanlar (3.7a karşılaştırması, 18.2'nin örneği, 6.3'ün gerekçesi) bu listeden silindi; 8.5 ve 16.5 uygulama işi olarak status'a geçti. Kalan üçü aşağıda.

Uygulama sırasında verilip aynı gün dil tanımına yazılan kararlar. DENETİM'den geçtiler; Gemini'nin karar araştırmasından geçmediler. Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar turda bağımsız denetlenir.
Dokuz iş tartısının kararlarından bir kısmı 2026-10-28'de uygulanmamıştı: onaltılık hash sabiti sözdizimi hatasıydı ve iletisi ".0 ekle" diyordu; hash çarpımının taşma iletisi ondalık öneriyordu; yapı dolaşılamıyordu; sözlük üreteci "expected a value" veriyordu [ölçüldü: 2026-10-28]. Bugünkü durumları status'ta.

- **3.6a, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** demet; 6.3'ün birden çok dönüşü demet, H2708 kalkar. **Tur 1 (2026-11-01, Gemini `TUR1_RAPORU.md`, DENETİM `kol-denetim-tur1-rapor-sonuc.md`): değiştir; yazılacak.**
- **3.14, geçici (2026-10-28, 2. katman):** yön karakterleri, kaptaki JSON, bayt dizisinin metni. **Tur 1 (2026-11-01, Gemini `TUR1_RAPORU.md`, DENETİM `kol-denetim-tur1-rapor-sonuc.md`): değiştir; yazılacak.**
- **13.8, geçici (2026-10-28, dokuz iş tartısı, `bolum1-9-dokuz-is`):** yapının JSON'u iki yönlü. **Tur 1 (2026-11-01, Gemini `TUR1_RAPORU.md`, DENETİM `kol-denetim-tur1-rapor-sonuc.md`): değiştir; yazılacak.**
- **10.2, kritik (2026-11-01, D1'in 10.2 ölçümü, `d1-10-2-olcum.md`):** eski söz "kilidi bekleyen bir görev en çok 10 ms içinde kilidi alır" 5 ms'lik devirle üç ve daha çok meşgul görevde tutamazdı ((k − 1) × 5 ms) [hesap]; liste ve sözlük metotlarının istisnası sözde yazılı değildi. Ana oturum aynı gün karar verdi: kilit geliş sırasıyla alınır; tutan görev bekleyen varken en çok 5 ms ve o anki deyimin sonuna kadar tutar; önünde n görev bekleyen görev en çok (n + 1) × 5 ms ve n + 1 deyim bekler; yalnız liste ve sözlük metotları sözün dışında; değişmez girdiyi okuyan yerleşikler (`TR_json_parse`, metin) sözün içinde; 5 ms'den uzun sürebilen her yerleşik kilidi bırakır (DENETİM, `kol-denetim-10-2-kritik-sonuc.md`). Geçici, turda denetlenecek.

## Ara



## Olağan

- **10.2, uzun bölünmez deyim (2026-11-01, DENETİM):** gecikme sözü "o anki deyimin sonuna kadar" der; kilidi bırakan çağrı içermeyen uzun bir deyim (büyük bir üreteç ifadesi) 10.2'ye göre bölünmez ve sözü boşa çıkarabilir. 1.5 ise döngünün her turunu güvenli nokta sayar [kanıtlı: `docs/spec/01_program.md:24`]. Öneri: turda, üreteç ve döngü turunun devir noktası olup olmadığı karara bağlanır.
- **3.10, 4.9, 5.7, 5.11: "liste üreteci" (2026-11-01, Gemini tur 1'in kendi bulgusu):** 5.11 sözlük üretecini ekledi, ama 3.10 (`03_tipler.md:84`), 4.9 (`04_operatorler.md:54`) ve 5.7 (`05_deyimler.md:54`) hâlâ yalnız "liste üreteci" diyor; 5.11 iki değişkenli `TR_for i, x TR_in` biçimini üreteçte yazmıyor, gerekçesi (`rationale/05_statements.md:59`) yazıyor. Öneri: "üreteç"; iki değişkenli biçim 5.11'e.
