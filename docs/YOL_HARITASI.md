# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini eğitebilmesi. Bu belge yapılacak işlerin sırasını tutar; tamamlanan işler kısa bir satırla en altta kalır.

## Şu an yapılıyor

- **Tek yazım temizliği** — Türkçe harfsiz biçimler, eş yazımlar, `matris`, `#` yorum ve İngilizce eşler kaldırılıyor; komut satırı tek Türkçe ada iniyor; yerleşik ad gölgelenince uyarı.
- **Tensör eksikleri** — `sinüs`/`kosinüs`, `birleştir`, `.böl`, `boyutu_koru`, birleşik `dikkat` (nedensel, pencereli, gruplu sorgu; döşemeli, T×T ara matrissiz), `örnekle`, `parametreler(model)`.
- **Kullanıcı tanımlı türev** — `özel_türev(ileri:, geri:)` ve `türev_denetle`.

## Sırada

1. **CPU hızı: profil ve iyileştirme** — küçük modellerde işlem başına sabit maliyet (yorumlayıcı, çizge düğümü, bellek ayırma); hedef PyTorch'u CPU'da da geçmek.
2. **Transformer kitaplığı (TR ile yazılı)** — katmanlar (Doğrusal, Gömme, RMSNormu, Dikkat, SwiGLU, Blok) ve yalnızca kod çözücülü `GPT`; model sıradan bir `yapı`, her model tek dosyada okunur.
3. **GPU arka ucu** — Go'nun ürettiği GPU kodu (PTX) sürücüye verilir; tek dosya dağıtım korunur, ek derleyici gerekmez.
   - Önce **G4** ve **L4** (bf16, fp8); **T4** sonra (fp16 yolu).
   - Programa gömülü birleşik çekirdekler: dikkat, doğrusal katman + çapraz entropi, RMSNorm/RoPE/SwiGLU, AdamW.
   - Blok boyutları arka ucun ayarı; ilk çalıştırmada ölçülerek seçilir ve saklanır.
   - Sınama Colab'da.
4. **TR ile çekirdek yazma** — eleman bazlı ve satır işlemleri için küçük, blok düzeyinde bir yazım; aynı kod CPU'da vektör komutlarına, GPU'da GPU koduna derlenir. Ardından tensör çekirdekli matris çarpımı ve dikkat.
5. **Yazım dilleri** — EN ilk ikinci dil; dosyanın başında yazım dili; TR ⇄ EN kayıpsız çeviri aracı (çakışan kullanıcı adları `$ad` ile); bütün testlerin ve örneklerin iki dilde de koşması. Sonra hata mesajları (H kodları) ve PT gibi başka diller; yalnızca resmî, programa gömülü diller.
6. **Üretim (modeli kullanan geliştirici için)** — KV önbellekli hızlı üretim, toplu istek işleme, nicemleme (int8/int4), model dosyaları, ağır hesapta yorumlayıcı kilidinin bırakılması.
7. **Model Sihirbazı** — VS Code eklentisinde, kod bilmeyenin bile model kurmasını sağlayan sihirbaz; okunur TR kodu üretir; hazır eğitim döngüsü bununla gelir.
8. **Az teknik kullanıcı yolu** — ham metinden modele kadar her adım TR içinde:
   - ham metin klasöründen veri kümesi hazırlama;
   - kendi parçalayıcısını eğitme (bugün yalnızca hazır `tokenizer.json` okunuyor);
   - eğitim sağlığı uyarıları: kayıp patlıyor ya da düşmüyor, öğrenme oranı yüksek görünüyor, bellek yetmeyecek;
   - çalıştırmadan önce süre, bellek ve maliyet tahmini;
   - dizüstü bilgisayarda ya da tek bir Colab GPU'sunda çalışan küçük model tarifleri;
   - eğitilen modeli tek dosya uygulama ya da web hizmeti olarak paylaşma.
9. **Statik şekil denetimi** — tensör şekil hatalarının çalıştırmadan önce yakalanması.
10. **Eklenti ve araçlar sözlükten** — VS Code renklendirme, tamamlama ve belge listelerinin `pkg/sozluk`'tan üretilmesi.

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
- Sözlük: dilin bütün adları `pkg/sozluk/tr.json`'da sabit kodlara bağlı.
