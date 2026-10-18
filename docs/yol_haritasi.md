# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini eğitebilmesi. Bu belge yapılacak işlerin sırasını tutar; tamamlanan işler kısa bir satırla en altta kalır.

## Şu an yapılıyor

- **İkinci tur hakem denetimi** — yeni klasör ve kod düzeninin, sabit kodların ve çeviriye hazırlığın denetimi.
- **Güvenlik denetiminin kalanları** — modül çözümleme sırası (önce içe aktaran dosyanın klasörü), düşük önemli bulgular.

## Sırada

1. **CPU hızı: profil ve iyileştirme** — küçük modellerde işlem başına sabit maliyet (yorumlayıcı, çizge düğümü, bellek ayırma); hedef bugünkü yaygın Python kitaplıklarını CPU'da da geçmek.
2. **Standart kitaplık (yapay zekâ iş akışı için)** — Python'un "pil dahil" kitaplığının tamamı değil, model eğiten ve kullanan birinin gerçekten ihtiyaç duyduğu kısmı:
   - klasör ve dosya yolu işlemleri (listeleme, birleştirme, var mı), ortam değişkenleri, komut satırı argümanları;
   - sıkıştırma (zip, gzip), özet (hash) ile veri bütünlüğü denetimi;
   - tarih ve saat, genel rastgele sayı, kullanıcıya açık düzenli ifadeler;
   - eğitim için günlük kaydı ve ilerleme çubuğu (adım, kayıp, kalan süre), profil çıkarma.
   Her ad önce tasarlanıp onaylanır; aynı işi yapan iki kod olmaz.
3. **Kodlu saklama** — dosyalar diskte dilden bağımsız sabit kodlarla saklanır (`A001`, `G100`); kişi kodu VS Code'da ya da başka bir düzenleyicide kendi yazım dilinde (`eğer` / `if`) görür ve düzenler. Böylece her dilde yazılan kod diskte aynıdır; farklı dillerde çalışan kişiler aynı dosyalar üzerinde birlikte çalışabilir.
   - Go kodunda kodlar adlı sabitler olarak (`kod.G100`), kod listesinden üretilir.
   - Sözcük çözümleyici kodları her zaman kabul eder; `tr biçimle` ve VS Code kaydı kodlu biçimi yazar; otomatik sınama kodlu biçimi denetler.
   - Kullanıcının adları, metinler, yorumlar ve sayılar olduğu gibi kalır; metin içi `{ifade}` yerleştirmelerindeki dil kelimeleri de kodlanır; ham metnin (`` `...` ``) içi hiç çevrilmez.
   - `tr göster dosya.tr --dil tr` ve hata mesajları kodu okunur dilde gösterir.
   - Depodaki örnekler ve testler, VS Code izdüşümü hazır olunca kodlu biçime çevrilir.
4. **Modern VS Code deneyimi** — dil sunucusu `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır; eklenti ince bir istemci olur):
   - yazarken hata gösterimi ve hızlı düzeltmeler (`yazdir` → `yazdır` gibi ipuçları tek tıkla);
   - üzerine gelince belge (kod listesinden ve dil tanımından), adlı argüman ipuçları, bağlama göre tamamlama;
   - tanıma git, bütün kullanımları bul, yeniden adlandır, kaydederken biçimlendirme;
   - anlamsal renklendirme: dilin kelimeleri ile kullanıcının adları ayrı renkte;
   - testlerin VS Code test gezgininde koşması, satır üstünde "çalıştır" ve "hata ayıkla" düğmeleri;
   - yapay zekâya özel: tensör şekillerinin satır içinde gösterimi, eğitimi canlı izleyen panel (kayıp eğrisi, gradyan normu, öğrenme oranı), Model Sihirbazı.
5. **Transformer kitaplığı (TR ile yazılı)** — katmanlar (Doğrusal, Gömme, RMSNormu, Dikkat, SwiGLU, Blok) ve yalnızca kod çözücülü `GPT`; model sıradan bir `yapı`, her model tek dosyada okunur.
6. **GPU arka ucu** — Go'nun ürettiği GPU kodu (PTX) sürücüye verilir; tek dosya dağıtım korunur, ek derleyici gerekmez.
   - Önce **G4** ve **L4** (bf16, fp8); **T4** sonra (fp16 yolu).
   - Programa gömülü birleşik çekirdekler: dikkat, doğrusal katman + çapraz entropi, RMSNorm/RoPE/SwiGLU, AdamW.
   - Blok boyutları arka ucun ayarı; ilk çalıştırmada ölçülerek seçilir ve saklanır.
   - Sınama bulut GPU makinelerinde.
7. **TR ile çekirdek yazma** — eleman bazlı ve satır işlemleri için küçük, blok düzeyinde bir yazım; aynı kod CPU'da vektör komutlarına, GPU'da GPU koduna derlenir. Ardından tensör çekirdekli matris çarpımı ve dikkat.
8. **Yazım dilleri** — EN ilk ikinci dil; dosyanın başında yazım dili; TR ⇄ EN kayıpsız çeviri aracı (çakışan kullanıcı adları `$ad` ile); bütün testlerin ve örneklerin iki dilde de koşması. Sonra hata mesajları (H kodları) ve PT gibi başka diller; yalnızca resmî, programa gömülü diller.
9. **Üretim (modeli kullanan geliştirici için)** — KV önbellekli hızlı üretim, toplu istek işleme, nicemleme (int8/int4), model dosyaları, ağır hesapta yorumlayıcı kilidinin bırakılması.
10. **Model Sihirbazı** — VS Code eklentisinde, kod bilmeyenin bile model kurmasını sağlayan sihirbaz; okunur TR kodu üretir; hazır eğitim döngüsü bununla gelir.
11. **Az teknik kullanıcı yolu** — ham metinden modele kadar her adım TR içinde:
   - ham metin klasöründen veri kümesi hazırlama;
   - kendi parçalayıcısını eğitme (bugün yalnızca hazır `tokenizer.json` okunuyor);
   - eğitim sağlığı uyarıları: kayıp patlıyor ya da düşmüyor, öğrenme oranı yüksek görünüyor, bellek yetmeyecek;
   - çalıştırmadan önce süre, bellek ve maliyet tahmini;
   - dizüstü bilgisayarda ya da tek bir bulut GPU'sunda çalışan küçük model tarifleri;
   - eğitilen modeli tek dosya uygulama ya da web hizmeti olarak paylaşma.
12. **Statik şekil denetimi** — tensör şekil hatalarının çalıştırmadan önce yakalanması.

## Platformlar

- **Windows, Linux (Intel/AMD):** tam destek, her değişiklikte otomatik sınama.
- **Mac:**
  1. Otomatik sınamaya Mac makinesi eklemek.
  2. Apple M çipleri (ARM) için vektör komutlu matris çarpımı ve vektör işlemleri (bugün düz Go yoluna düşüyor).
  3. Mac GPU'su için Metal arka ucu.
- **Linux (ARM):** ARM vektör komutlu çekirdek Mac ile ortak.

## Tamamlananlar

- Yorumlayıcı: kapanış derlemesi, yuva tabanlı ortamlar; Python ile başa baş hız.
- Tensör çekirdeği: AVX2/FMA matris çarpımı, ters yönlü otomatik türev, kalıcı işçi havuzu; `matris` tensöre katıldı.
- Veri hattı: `.npy` parçaları, önden yükleme, önbellek; genel `tokenizer.json` parçalayıcısı.
- bf16/fp16 veri türleri ve `karışık_hassasiyet` bloğu.
- Örnekler 01–09 öğrenme sırasıyla.
- Kodlar: dilin bütün adları `internal/dil/kod/tr.json`'da sabit kodlara bağlı; açıklamalı başvuru `docs/kodlar.md`.
- Tek yazım: Türkçe harfsiz biçimler, eş yazımlar, `matris`, `#` yorum ve İngilizce eşler kaldırıldı; komut satırı tek Türkçe ad; yerleşik ad gölgeleme uyarısı; eklenti listeleri kod listesinden üretiliyor.
- Model yapı taşları: `sinüs`/`kosinüs`, `birleştir`, `.böl`, `boyutu_koru`, birleşik `dikkat` (nedensel, pencereli, gruplu sorgu; T×T ara matrissiz, uzun bağlamda 2–10 kat hızlı), `örnekle`, `parametreler(model)`.
- Standart kitaplığın temeli (Aşama 0): ham metin `` `...` `` ve bilinmeyen kaçış hatası, yol ve tembel yineleme kancaları, betik argümanları, Ctrl+C ile nazik çıkış (ertelenen işler çalışır, çıkış kodu 3), canlı satır ve gizli değer süzgeci; `İyileştirici` tipi, `indirge` bloğunda `birikim`.
- Güvenlik denetimi: tensör dosyası ve tensör ayırmaları bellek sınırıyla denetlenir, metin içi yerleştirmeye derinlik sınırı, güvenilmeyen girdiler için bulanıklaştırma testleri, VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır, otomatik sınamada sabitlenmiş eylemler, Go 1.25.
