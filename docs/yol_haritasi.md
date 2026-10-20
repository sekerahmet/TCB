# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini kurup eğitebilmesi.

## İlkeler

- **Önce sağlam temel, sonra özellik.** İlk evre, yapılabildiğini görmek için hızlı bir koşuydu. Bundan sonra her özellik, kuralları belli ve testle korunan bir temelin üstüne eklenir. Yama gibi büyüyen kod bir süre sonra yönetilemez.
- **Evreler bitiş ölçütüyle ilerler.** Bir evrenin ölçütü sağlanmadan sonrakine geçilmez. Temel işi sonu belirsiz bir temizliğe dönüşmez.
- **Her kural bir testle korunur.** Yalnızca yazılı kalan kural zamanla aşınır. Örnek: dosya ve fonksiyon boyu, katman sırası, kod listesinden üretilen adlar, ileti kodları.
- **Doğrusu neyse o.** Bir tasarım kararında yerleşik ve kanıtlanmış uygulama (best practice) seçilir; gerekçesi belgeye yazılır.
- **Önce ölç, sonra değiştir.** Hız işinde her değişiklik önce ve sonra ölçülür. Kazandırmayan "iyileştirme" geri alınır.
- **Dış incelemeler doğrulanarak düzeltilir.** Her bulgu önce yeniden üretilir. Doğrulananın düzeltmesine, düzeltmeden önce kalan sonra geçen bir test eklenir.
- **Güçlü yanlar korunur.** Yeni özellik şunları bozmaz:
  - belirlenimcilik: sonuç işçi sayısından bağımsızdır, kesilip devam eden eğitim bit düzeyinde aynıdır;
  - tek dosya ve cgo'suz dağıtım: GPU yalnızca sürücüyle çalışır;
  - güvenilmeyen girdinin sınırlarla denetlenmesi;
  - okunur model kodu.
- **Birlikte çalışma dosya düzeyindedir.** Python köprüsü bilinçli olarak yoktur. Bunun karşılığı, yaygın dosya biçimlerini (safetensors, GGUF, `.npy`, `tokenizer.json`, jsonl) okuyup yazmaktır.

## Evre 1: Kanıt (tamamlandı)

Dil, tensörler, otomatik türev, veri hattı ve GPU üst üste kuruldu; TR ile bir Transformer eğitilebiliyor. GPU'da (116 M parametreli GPT, bf16) adım süresi PyTorch'la başa baş, bellek kullanımı daha az.

## Evre 2: Sağlam temel (şu an)

1. **İleti kodları.** Kullanıcıya giden her ileti, adlar gibi sabit bir koda bağlanır (`H` ve dört basamak, `internal/lang/messages`). Metinler dil dosyasındadır; parametreler adlıdır (`{fn}: {path|q} bir klasör`). Testler metni değil kodu denetler.
   - Tamamlandı: altyapı ve denetim testi; ortak hata yardımcıları; standart kitaplık, `devtools`, `tr` komutu; dil çekirdeği (sözcük çözümleyici, ayrıştırıcı, tip denetleyicisi, yorumlayıcı). Hata konumu artık yapıdan okunur, metinden okunmaz.
   - Sırada:
     - Yapılı hata: ayrıştırıcı ve denetleyici hataları komut satırına, hata ayıklayıcıya, biçimlendiriciye ve test koşucusuna konumlu hata olarak gider. Konumu metinden okuyan son yedek kalkar.
     - En son `compute`.
2. **Açık işler ve bekleyen dil kararları.**
   - `csv_çöz(metin)`: `csv_oku` yalnızca dosya okur, CSV metni `csv_çöz` ile ayrıştırılır (`json_çöz` gibi). Metin var olan bir dosyanın adıysa dosyanın okunması belirsizliği kalkar.
   - Dil çekirdeği incelemesinden kalan konular: kullanıcı metnindeki terminal kaçış dizileri, `VAL_INT` gibi adlar.
   - Testler: eşzamanlı görevdeki başarısız doğrulamanın testi kaldırması; testlerin üst düzey ortamı paylaşması.
   - Yeni satır başındaki `-x` ifadesinin önceki satıra ikili eksi olarak bağlanması.
3. **Test düzeni (tamamlandı).** Test dosyası adlandırma kuralı (`x_test.go` yalnızca `x.go`'yu sınar; süreçleri sınayan testler `behavior_` önekli dosyalarda) ve her paket README'sinde bir "Testler" bölümü; ikisi de testle denetlenir.
4. **Kalan adlar.** `cmd/tr` dosya ve Go adları ile `tools` betik adları İngilizceye geçer. Kullanıcıya görünen komut adları değişmez.
5. **Kurallar testle korunur.** Her paket README'sindeki "nasıl eklenir" kuralları gözden geçirilir; testi olmayan kurala test eklenir. Belgede (`docs/mimari.md`) adı geçen Go adlarının var olduğu da testle denetlenir.
6. **Belgeler yeni düzeni anlatır (tamamlandı).** Kurallar (`internal/README.md`), ekleme tarifleri (`docs/tarifler.md`), mimari ve katkı rehberi yeni düzene göre yazıldı.

**Bitiş ölçütü:** kodda Türkçe ileti metni kalmaz, her kural bir testle korunur, bekleyen dil kararı kalmaz.

## Evre 3: Gerçek model

Hız hedefi ve dilin asıl sınavı, Python/PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş, yalnızca kod çözücülü (decoder-only) bir Transformer ve küçük sürümleridir (hedef model). Eksik parçalar Evre 2'nin kurallarına uyarak eklenir:

- **Bellek:** parçalı logit + çapraz entropi (logitler bütünüyle bellekte tutulmaz); dikkatin adımlı girdi okuması (başlara ayırma kopyasız).
- **Hız:**
  - birleşik RMSNorm, RoPE ve SwiGLU çekirdekleri;
  - tensör çekirdekli dikkatin geri yönünün baş boyu 128'e genişletilmesi (bugün en çok 64; büyüğünde SIMT yola düşer);
  - eleman bazlı işlem zincirlerinin kaynaştırılması ve CUDA Graphs adayları, önce ölçülerek.
- **Model parçaları:** katman başına gömme tabloları (PLE) için seyrek satır güncellemesi; Muon'da aynı şekilli matrislerin toplu Newton–Schulz adımı.
- **Karşılaştırma:** hedef model TR'de yazılır; PyTorch sürümüyle hız, bellek ve kayıp eğrisi karşılaştırılır. Kıyas hem derlenmemiş hem derlenmiş (`torch.compile`) PyTorch'la, bf16'da yapılır.
- **Kartlar:** ölçümler ve GPU testleri birden çok kartta tekrarlanır (tensör çekirdeksiz yol için T4, L4/A100).
- **CPU:** CPU'da küçük bir modelle eğitim denemesi; işlem başına sabit maliyetin profili.

**Bitiş ölçütü:** hedef model TR'de PyTorch sürümüyle aynı kalitede eğitilir. Hızı en az derlenmemiş PyTorch kadardır; derlenmiş PyTorch'la fark ölçülüp yazılır.

## Evre 4: Üretim asgarisi

Bugün TR, sıfırdan ve tek GPU'da model eğiten bir dil. Üretimdeki eğitimin çoğuysa hazır bir modelden ince ayar yapmak, birden çok GPU'da eğitmek ve sonucu yaygın sunum araçlarına vermek. Bu evre bu üç yolu açar. Sıra bağımlılığa göredir: önce dilin tensör yüzü tamamlanır, sonra dosya biçimleri, en son ölçek.

1. **Tensör dili tamamlanır:**
   - genel indeksleme: çok eksenli `t[:, 0]`, bool maskeyle seçme, topla/dağıt (gather/scatter);
   - yığ (stack), doldur (pad), tekrarla (repeat);
   - kullanıcıya açık tam sayı ve bool tensör türleri: token kimlikleri ve maskeler tensörle taşınır;
   - dropout;
   - kayıplar: BCE, KL, etiket yumuşatma, z-loss (MSE var);
   - rastgelelik: düzgün dağılım, Bernoulli, rastgele tam sayı, permütasyon; kontrol noktasına yazılan rastgelelik üreteci nesnesi;
   - ağırlık başlatma şemaları (Xavier, Kaiming), EMA ağırlıkları.
2. **Bellek:** aktivasyon yeniden hesaplama (gradient checkpointing).
3. **Hazır modeller:**
   - safetensors okuma ve yazma;
   - yaygın bir açık mimarinin (ör. Llama ailesi) yapılandırmasını okuyup TR modeline kurma;
   - LoRA ile ince ayar ve bir ince ayar örneği.
4. **Çıkarım:**
   - KV önbellekli `üret`;
   - int8/int4 nicemleme;
   - GGUF dışa aktarma (llama.cpp ve onu okuyan araçlar için);
   - toplu istek sunumu: yorumlayıcı kilidi tek görev çalıştırdığı için ağır hesap kilidi bırakır ya da sunum Go tarafında yapılır.
5. **Çoklu GPU:**
   - veri paralelliği, gradyanlar NCCL ile toplanır;
   - her GPU'ya kendi veri parçasını veren örnekleyici;
   - kontrol noktası çoklu GPU'da da belirlenimci kalır.
6. **GPU otomatik sınaması:** zamanlanmış GPU testleri, en az iki kart türünde (T4 ve L4).
7. **Sürüm politikası:**
   - sürüm numaralama;
   - dil ve dosya biçimi dondurma kuralları;
   - yayın ikilileri.

   Bugünkü "deneme dönemi, geriye uyum yok" durumu bu evrenin sonunda biter.

**Bitiş ölçütü:**
- Hazır bir açık model safetensors'tan yüklenir ve TR'de LoRA ile ince ayar yapılır.
- Aynı eğitim iki GPU'da veri paralel koşar.
- Sonuç GGUF olarak dışa aktarılıp llama.cpp'de çalışır.
- GPU testleri otomatik koşar.
- İlk kararlı sürüm yayımlanır.

## Evre 5: Geliştirici deneyimi

- **VS Code dil sunucusu** `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır):
  - yazarken hata gösterimi, tek tıkla düzeltme;
  - üzerine gelince belge, adlı argüman ipuçları, tamamlama;
  - tanıma git, kullanımları bul, yeniden adlandır;
  - anlamsal renklendirme;
  - test gezgini ve satır üstünde çalıştır ya da hata ayıkla düğmeleri;
  - tensör şekillerinin satır içinde gösterimi, eğitimi canlı izleyen panel.
- **Jupyter/Colab çekirdeği:** TR not defterinde hücre hücre çalışır. Yapay zekâ geliştiricileri not defterinde çalışır.
- **İzleme Masası:** Devam düğmesi; gradyan ve tensör özeti.
- **Deney takibi ve değerlendirme:**
  - günlüğün TensorBoard'a ya da MLflow'a aktarımı;
  - perplexity, doğruluk, F1;
  - eğitim sağlığı uyarıları (NaN/Inf, gradyan normu patlaması).
- **Transformer kitaplığı:** kitaplık adlarının yazım dillerinde çevirisi, hazır eğitim döngüsü, iyileştiricide sonradan değiştirilebilen öğrenme oranı.
- **Kodlu saklama:** otomatik sınamanın kodlu biçimi denetlemesi; depodaki örneklerin ve testlerin kodlu biçime çevrilmesi.
- **Standart kitaplık:** sıkıştırma (zip), özet (hash), tarih ve saat, düzenli ifadeler; her ad önce tasarlanıp onaylanır.

## Evre 6: Yaygınlaşma

- **Model Sihirbazı:** kod bilmeyenin bile model kurmasını sağlayan, okunur TR kodu üreten sihirbaz.
- **Az teknik kullanıcı yolu:**
  - ham metinden veri kümesi hazırlama;
  - parçalayıcı eğitme;
  - çalıştırmadan önce süre, bellek ve maliyet tahmini;
  - küçük model tarifleri;
  - eğitilen modeli tek dosya uygulama ya da web hizmeti olarak paylaşma.
- **Model kapsamı:**
  - görüntü ve ses: evrişim (1B/2B), havuzlama, yukarı örnekleme, BatchNorm/GroupNorm;
  - karşıtsal kayıp;
  - MoE: en iyi k yönlendirme ve gruplu çarpım.
- **Veri biçimleri:**
  - jsonl ve parquet akışı;
  - görüntü çözme (PNG/JPEG);
  - bulut depolamadan (S3/GCS) akış.
- **Büyük ölçek:** ZeRO/FSDP ile iyileştirici durumunun ve parametrelerin GPU'lara bölünmesi; parçalı ve arka planda yazılan kontrol noktası.
- **Paket yöneticisi:** TR kitaplıklarının paylaşılması (bugün yalnızca yerel ve gömülü modüller var).
- **Yazım dilleri:** adların İngilizcesi var (`en.json`, `tr göster --dil en`); iletilerin İngilizcesi (`internal/lang/messages/en.json`) eklenir.
- **Platformlar:** Mac otomatik sınaması, Apple M ve ARM için vektör çekirdekleri, Metal arka ucu, AMD (ROCm).

## Sonra

- FP8 eğitimi (Hopper/Blackwell); CPU'da AVX-512.
- Uzun bağlam için dizi paralelliği; hiperparametre taraması.
- İleri yönlü ve yüksek mertebe türev.
- Statik şekil denetimi: tensör şekil hatalarının çalıştırmadan önce yakalanması.
- Matematik: doğrusal sistem çözme, ters matris, ayrışımlar (QR, Cholesky, tekil değer), hızlı Fourier dönüşümü; türevli, CPU ve GPU çekirdekli.
- TR ile çekirdek yazma: aynı kodun CPU'da vektör komutlarına, GPU'da GPU koduna derlendiği küçük bir yazım.

## Platform durumu

- **Windows, Linux (Intel/AMD):** tam destek, her değişiklikte otomatik sınama.
- **Linux ve Mac (ARM):** derleniyor ve sınanıyor; vektör çekirdekleri yok (düz Go yolu).
- **GPU:** NVIDIA (sürücü ve isteğe bağlı cuBLASLt çalışma anında yüklenir); ölçümler RTX PRO 6000 Blackwell üzerinde.

## Tamamlananlar

- Yorumlayıcı: kapanış derlemesi, yuva tabanlı ortamlar; Python ile başa baş hız.
- Tensör çekirdeği: AVX2/FMA matris çarpımı (çalışma anında seçilir), ters yönlü otomatik türev, kalıcı işçi havuzu.
- Veri hattı: `.npy` parçaları, önden yükleme, önbellek; genel `tokenizer.json` parçalayıcısı.
- bf16/fp16 veri türleri ve `karışık_hassasiyet` bloğu; 16 bitlik ara gradyanlar kendi türünde, float32 ağırlığın gradyanı yuvarlanmadan.
- GPU arka ucu: Go'nun ürettiği GPU kodu, bütün tensör işlemleri ve gradyanları, tensör çekirdekli dikkat (ileri ve geri), cuBLASLt ile bf16 matris çarpımı, çok tensörlü iyileştirici adımları, kontrol noktası ve gradyan kırpma cihazda.
- Kodlar: dilin bütün adları sabit kodlara bağlı (`internal/lang/codes`); açıklamalı başvuru `docs/kodlar.md`; kodlu saklama (`tr kodla`, `tr çöz`, `tr göster`) ve VS Code izdüşümü; İngilizce yazım dili.
- Tek yazım: Türkçe harfsiz biçimler, eş yazımlar ve İngilizce eşler kaldırıldı; yerleşik ad gölgeleme uyarısı.
- Matematik ve model yapı taşları: genişletilmiş eleman bazlı matematik, birleşik `dikkat`, `örnekle`, `parametreler(model)`; Transformer kitaplığı (`tr/katman`: Doğrusal, RMSNorm, RoPE, gruplu sorgu dikkati, SwiGLU, GPT).
- Standart kitaplık: dosya ve yol işlemleri, komut satırı, günlük ve ilerleme çubuğu, kontrol noktası, profil ve canlı izleme.
- Kod tabanı İngilizceye geçti (klasörler, dosyalar, Go adları); her paketin README'si standartlarını yazar.
- Dış incelemeler: dil çekirdeği, hesap çekirdeği, standart kitaplık ve geliştirici araçları raporları doğrulanarak düzeltildi.
- Güvenlik: güvenilmeyen girdiler (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`) sınırlarla denetlenir ve bulanıklaştırma testleriyle sınanır; VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır; otomatik sınamada sabitlenmiş eylemler.
