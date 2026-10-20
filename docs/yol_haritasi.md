# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini kurup eğitebilmesi.

Bugün TR, tek GPU'da sıfırdan model eğitmekte derlenmemiş PyTorch kadar hızlı ve daha az bellek kullanıyor; ama kendi içinde kapalı: hazır bir modeli yükleyemiyor, ince ayar yapamıyor, eğittiği modeli yaygın araçlara veremiyor. Yeni ML dillerinin (Swift for TensorFlow, Julia, Mojo, Dex, Futhark) ve Python dışı çerçevelerin (Gorgonia, GoMLX, Candle, Burn, llama.cpp) ortak dersi: kullanıcıyı dil tasarımı kazanmaz. Hazır modellere erişim, doğru gradyan, kararlı sürümler ve her gün kullanan bir iç kullanıcı kazanır. Bu yüzden sıra şudur: önce ölçüm ve doğruluk kapıları, sonra tek GPU'da bellek ve baş boyu uyumu, sonra hazır ağırlık, ince ayar ve üretim yolu. Çoklu GPU ve genel derleyici ancak bunlar yerine oturunca gelir.

## İlkeler

- **Önce sağlam temel, sonra özellik.** İlk evre, yapılabildiğini görmek için hızlı bir koşuydu. Bundan sonra her özellik, kuralları belli ve testle korunan bir temelin üstüne eklenir. Her eksiği aynı anda kapatmaya çalışmak en pahalı hatadır.
- **Evreler bitiş ölçütüyle ilerler.** Bir evrenin ölçütü sağlanmadan sonrakine geçilmez. Temel işi sonu belirsiz bir temizliğe dönüşmez.
- **Her kural bir testle korunur.** Yalnızca yazılı kalan kural zamanla aşınır. Örnek: dosya ve fonksiyon boyu, katman sırası, kod listesinden üretilen adlar, ileti kodları.
- **Yanlış gradyan en pahalı hatadır.** Tek otomatik türev sistemi vardır; her türevli işlemin sayısal türev testi yayın kapısıdır.
- **Doğrusu neyse o.** Bir tasarım kararında yerleşik ve kanıtlanmış uygulama (best practice) seçilir; gerekçesi belgeye yazılır.
- **Önce ölç, sonra değiştir.** Hız işinde her değişiklik önce ve sonra ölçülür; kazandırmayan "iyileştirme" geri alınır. Genel derleyici ve zorunlu otomatik ayarlama yoktur; ölçümün gösterdiği yer küçük, kurallı ve testli adımlarla kapatılır.
- **Dış incelemeler doğrulanarak düzeltilir.** Her bulgu önce yeniden üretilir. Doğrulananın düzeltmesine, düzeltmeden önce kalan sonra geçen bir test eklenir.
- **Birlikte çalışma dosya ve protokol düzeyindedir.** Python köprüsü bilinçli olarak yoktur. Karşılığı, yaygın ve kararlı biçimleri okuyup yazmaktır: safetensors, Hugging Face `config.json` ve `tokenizer.json`, PEFT bağdaştırıcısı, GGUF, `.npy`, jsonl; sunumda OpenAI uyumlu HTTP protokolü.
- **Tutulamayacak söz verilmez.** Uyumluluk yazılı bir sürüm politikasıyla verilir; her yerleşiğin kararlılık düzeyi bellidir.
- **Hedef model sürekli TR'de eğitilir.** Dilin ilk ve her gün kullanan iç kullanıcısı odur.
- **Güçlü yanlar korunur.** Yeni özellik şunları bozmaz:
  - belirlenimcilik: sonuç işçi sayısından bağımsızdır, kesilip devam eden eğitim bit düzeyinde aynıdır;
  - tek dosya ve cgo'suz dağıtım: GPU yalnızca sürücüyle çalışır;
  - CPU ve GPU arka uçları tek tensör API'sinin arkasında ayrıdır; hiçbir dış derleyiciye kilitlenilmez;
  - güvenilmeyen girdinin sınırlarla denetlenmesi;
  - sabit ad kodları ve okunur model kodu.

## Evre 1: Kanıt (tamamlandı)

Dil, tensörler, otomatik türev, veri hattı ve GPU üst üste kuruldu; TR ile bir Transformer eğitilebiliyor. GPU'da (116 M parametreli GPT, bf16) adım süresi derlenmemiş PyTorch'la başa baş (~81 ms), bellek kullanımı ~3 GB daha az.

## Evre 2: Sağlam temel (şu an)

1. **İleti kodları.** Kullanıcıya giden her ileti, adlar gibi sabit bir koda bağlanır (`H` ve dört basamak, `internal/lang/messages`). Metinler dil dosyasındadır; parametreler adlıdır (`{fn}: {path|q} bir klasör`). Testler metni değil kodu denetler.
   - Tamamlandı: altyapı ve denetim testi; ortak hata yardımcıları; standart kitaplık, `devtools`, `tr` komutu; dil çekirdeği; yapılı hata (hata konumu her yerde yapıdan okunur, metinden okunmaz); GPU arka ucu.
   - Sürüyor: tensör çekirdeği, veri hattı ve parçalayıcı.
   - Sonra: geçirilmiş paketlerde Türkçe harfli metin sabitinin yalnızca izinli yerlerde (veri biçimi anahtarları, Türkçe ek tabloları) bulunduğunu denetleyen test. Bitiş ölçütünü bu test kanıtlar.
2. **Gradyan doğruluğu kapısı.** Her türevli işlemin (tensör çekirdeğinin işlem listesindeki her işlem) float64 sonlu fark testi ve bf16 tolerans testi vardır. İşlem listesini gezen bir test, türev testi olmayan işlemi yakalar. GPU'da aynı testler her GPU değişikliğinde koşar. Temeli: `türev_denetle` ve tensör testlerindeki sayısal türev denetimi.
3. **Anlam kuralları yazılı ve testli.** Yayınlama (15.3), tür yükseltme (15.12), dilim ve indeksin sınır dışı davranışı, tam sayılı ve mantık değerli işlenenler tek yerde, eksiksiz yazılır; her kural bir `tests/` testiyle korunur. Sessiz kırpma ya da sessiz tür değişimi yoktur; belirsiz durum hatadır.
4. **Açık işler ve bekleyen dil kararları.**
   - Tamamlandı: `csv_çöz(metin)`; `csv_oku` yalnızca dosya okur (13.8). Ortam değişkeni adları ASCII (`TR_IS_KLASORU`).
   - Dil çekirdeği incelemesinden kalan konular: kullanıcı metnindeki terminal kaçış dizileri, `VAL_INT` gibi adlar.
   - Testler: eşzamanlı görevdeki başarısız doğrulamanın testi kaldırması; testlerin üst düzey ortamı paylaşması.
   - Yeni satır başındaki `-x` ifadesinin önceki satıra ikili eksi olarak bağlanması.
5. **Test düzeni (tamamlandı).** Test dosyası adlandırma kuralı (`x_test.go` yalnızca `x.go`'yu sınar; süreçleri sınayan testler `behavior_` önekli dosyalarda) ve her paket README'sinde bir "Testler" bölümü; ikisi de testle denetlenir.
6. **Kalan adlar.**
   - `cmd/tr` dosya ve Go adları ile `tools` betik adları İngilizceye geçer.
   - `tr` komutunun alt komutları (`çalıştır`, `biçimle` ...), seçenekleri (`--yardım`) ve hata ayıklayıcının komutları bir ad ailesine bağlanır; adlar değişmez, Go metninden çıkar.
7. **Kurallar testle korunur.** Her paket README'sindeki "nasıl eklenir" kuralları gözden geçirilir; testi olmayan kurala test eklenir. Belgede (`docs/mimari.md`) adı geçen Go adlarının var olduğu da testle denetlenir.
8. **Belgeler yeni düzeni anlatır (tamamlandı).** Kurallar (`internal/README.md`), ekleme tarifleri (`docs/tarifler.md`), mimari ve katkı rehberi yeni düzene göre yazıldı.

**Bitiş ölçütü:** kodda Türkçe ileti metni kalmaz (testle); her türevli işlemin gradyan testi vardır; anlam kuralları yazılı ve testlidir; her kural bir testle korunur; bekleyen dil kararı kalmaz.

## Evre 3: Tek GPU'da gerçek model

Dilin asıl sınavı, Python/PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş, yalnızca kod çözücülü (decoder-only) bir Transformer ve küçük sürümleridir (hedef model). Tek GPU'da sınır hız değil, bellek ve baş boyu uyumudur; sıra buna göredir. Eksik parçalar Evre 2'nin kurallarına uyarak eklenir.

1. **Önce ölçüm.** Aynı kartta ve aynı yapılandırmada `torch.compile` (varsayılan ve `max-autotune`) ile TR karşılaştırılır; TR adımının Nsight Systems zaman çizelgesi çıkarılır (çekirdek sayısı, çekirdekler arası boşluk, en pahalı 10 çekirdek). Fark kaynaştırmada mı, matris çarpımı seçiminde mi, başlatma boşluklarında mı, bu ölçüm söyler. Tahmini fark 1,1–1,4 kattır; ölçülmeden hız işi seçilmez.
2. **Bellek:**
   - parçalı ya da birleşik doğrusal katman + çapraz entropi: logitler bütünüyle bellekte tutulmaz (büyük sözlükte en büyük tek tampon logitlerdir);
   - blok düzeyinde, isteğe bağlı aktivasyon yeniden hesaplama (yaklaşık %33 süreye etkinleşme belleğinde 10 kattan fazla azalma);
   - dikkatin adımlı girdi okuması (başlara ayırma kopyasız).
3. **Rastgelelik ve dropout.** Açık, bölünebilir ve kontrol noktasına yazılan bir rastgelelik anahtarı (JAX emsali); üstüne dropout. Yeniden hesaplanan blok aynı rastgeleliği yeniden üretir, böylece bit düzeyinde aynılık korunur. Yeniden hesaplamanın önkoşuludur.
4. **Baş boyu uyumu.**
   - tensör çekirdekli dikkatin geri yönü baş boyu 128'e (bugün en çok 64; büyüğünde SIMT yola düşer), sonra 256'ya;
   - gizli boyuttan bağımsız baş boyu (`tr/katman`'da bugün `baş_boyutu = boyut / baş`).

   Bu bir hız değil uyumluluk konusudur: açık modellerin neredeyse hepsi 128 kullanır ve hazır ağırlık yüklemenin önkoşuludur.
5. **Hız (ölçümden sonra, ölçümün gösterdiği sırayla):**
   - birleşik RMSNorm, RoPE ve SwiGLU çekirdekleri;
   - cuBLASLt epilogları (bias, GELU, bias gradyanı);
   - eleman bazlı zincirleri ve tekli indirgemeleri tek GPU çekirdeğine kaynaştıran küçük, kural tabanlı bir birleştirici; her kaynaşmanın kaynaşmamış yola karşı sayısal testi vardır. Genel derleyici değildir.
6. **Eğitim gözlemlenebilirliği.**
   - Günlüğe yazılanlar: kırpma öncesi ve sonrası gradyan normu, parametre normu, katman başına etkinleşme RMS, en büyük logit ya da log Z, fp16'da kayıp ölçeği, saniyede token, MFU.
   - NaN/Inf'te ya da kayıp hareketli ortalamasının k katını aşınca durma ya da son kontrol noktasına dönme.
   - Seçenekler: z-loss, "N toplu işi atla", devamda öğrenme oranı ölçeği.
   - Eşzamansız kontrol noktası: Colab'da asıl arıza oturumun kesilmesidir, sık ve ucuz kontrol noktası ile sağlam otomatik devam bu yüzden gerekir.
7. **Belirlenimci veri sırası.** Örnek sırası (tohum, dönem, küresel örnek indeksi) üçlüsünün saf bir fonksiyonudur; GPU ve işçi sayısından bağımsızdır. Çoklu GPU geldiğinde belirlenimcilik bozulmaz; geri dönüp toplu iş atlama bunun üstüne kurulur.
8. **T4 yolu.** bf16 tensör çekirdeği olmayan kartlar için fp16 ve dinamik kayıp ölçekleme; ölçek değeri günlüğe yazılır.
9. **Model parçaları:** katman başına gömme tabloları (PLE) için seyrek satır güncellemesi; Muon'da aynı şekilli matrislerin toplu Newton–Schulz adımı.
10. **Karşılaştırma ve kartlar.** Hedef model TR'de yazılır; PyTorch sürümüyle hız, bellek ve kayıp eğrisi karşılaştırılır. Ölçümler ve GPU testleri birden çok kartta tekrarlanır (T4, L4, A100). CPU'da küçük bir modelle eğitim denemesi ve işlem başına sabit maliyetin profili.

**Bitiş ölçütü:**
- Hedef model TR'de PyTorch sürümüyle aynı kalitede eğitilir.
- Hız ölçütü derlenmiş (`torch.compile`) PyTorch'tur; ilk ölçümden sonra hedef oran buraya yazılır ve sağlanır.
- Baş boyu 128 tensör çekirdeğiyle ileri ve geri çalışır.
- Eğitim bir NaN'de ya da kayıp sıçramasında kendiliğinden durur ya da geri döner.

## Evre 4: Hazır modeller ve üretim yolu

Modeli kullanan geliştiricinin ilk sorusu: "Hugging Face'teki modeli yükleyip ince ayar yapabilir miyim?" Bu evre o yolu açar. Sıra bağımlılığa göredir.

1. **Tensör dili tamamlanır.**
   - Genel indeksleme: çok eksenli `t[:, 0]`, bool maskeyle seçme, topla/dağıt (gather/scatter).
   - Yığ (stack), doldur (pad), tekrarla (repeat).
   - Kullanıcıya açık tam sayı ve bool tensör türleri: token kimlikleri ve maskeler tensörle taşınır.
   - Kayıplar: BCE, KL (damıtma ve tercih eğitimi için), etiket yumuşatma (MSE var).
   - Ağırlık başlatma şemaları (Xavier, Kaiming), EMA ağırlıkları.

   KV önbellek, örnekleme, maske ve LoRA yükleme bunlara dayanır.
2. **safetensors yerel ağırlık biçimi olur.**
   - `.trt`'nin kullanıcıya safetensors'un vermediği bir şey vermediği için TR'nin kendi biçimi bırakılır; TR'ye özgü bilgi `__metadata__`'da durur.
   - Okuma kuralları ileti kodu, dil kuralı ve bulanıklaştırma testi olur:
     - başlık sınırı (100 MB);
     - sıralı ve çakışmasız bayt aralıkları;
     - `(son − baş) = şeklin çarpımı × eleman boyu`;
     - bilinen veri türü;
     - yinelenen anahtar yok.
   - Bölünmüş kontrol noktası dizini (`model.safetensors.index.json`) okunur.
3. **Hugging Face modelleri.**
   - `config.json`'dan TR modeli kurulur; ilk aile Llama (sonra Qwen ve Mistral; Qwen'deki bağımsız `head_dim` dahil).
   - Tensör adları (`model.layers.N.self_attn.q_proj.weight`) bir eşleme tablosuyla TR modelinin adlarına bağlanır.
   - `tokenizer.json` zaten okunuyor.
4. **İnce ayar.**
   - LoRA; bağdaştırıcı PEFT düzeninde kaydedilir (`adapter_config.json` + `adapter_model.safetensors`), böylece PEFT, vLLM ve llama.cpp doğrudan okur.
   - Bir ince ayar örneği.
5. **Çıkarım ve sunum.**
   - KV önbellekli `üret`; örnekleme (sıcaklık, en iyi k, en iyi p).
   - 127.0.0.1'e bağlı OpenAI uyumlu HTTP sunucusu: completions ve chat, `logprobs` ile. Bu tek adım lm-evaluation-harness ile değerlendirmeyi, arayüzleri ve ajan çerçevelerini getirir.
   - Yorumlayıcı kilidi çekirdek çağrılarında ve G/Ç'de bırakılır; hangi yerleşiklerin bıraktığı belgelenir. Böylece sunucu, parçalayıcı ve GPU eşzamanlı çalışır.
   - CUDA Graphs önce üretim için: statik şekil kipinde, isteğe bağlı yakalama. Yakalanan bölgede ana belleğe okuma dil düzeyinde hatadır. Eğitimde beklenen kazanç küçüktür (%0–5).
6. **Sürüm politikası.**
   - 0.y.z sürüm numaralama.
   - Her yerleşiğe ad kodu üzerinden kararlı, beta ya da prototip etiketi; kararlı davranış prototip ya da beta bir alternatif uğruna kaldırılmaz.
   - İki adımlı kaldırma: önce uyarı (bir süre ya da iki sürüm), sonra hata. Ad kodları eski adı tutabildiği için bu TR'de ucuzdur.
   - Dil ve dosya biçimi dondurma kuralları; yayın ikilileri.

   Bugünkü "deneme dönemi, geriye uyum yok" durumu bu politikayla sınırlanır ve 1.0'da biter.

**Bitiş ölçütü:**
- Hugging Face'ten Llama ailesinden küçük bir model safetensors ile yüklenir, TR'de LoRA ile ince ayar yapılır.
- Bağdaştırıcı PEFT ile okunur.
- Model OpenAI uyumlu sunucudan sunulur ve lm-evaluation-harness ile değerlendirilir.
- Sürüm politikası yazılıdır; ilk 0.y.z sürümü yayımlanır.

## Evre 5: Geliştirici deneyimi

- **Not defteri:** Jupyter çekirdeği ya da VS Code defteri; TR, Colab'da ve not defterinde hücre hücre çalışır. Yapay zekâ geliştiricileri not defterinde çalışır.
- **VS Code dil sunucusu** `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır):
  - yazarken hata gösterimi, tek tıkla düzeltme;
  - üzerine gelince belge, adlı argüman ipuçları, tamamlama;
  - tanıma git, kullanımları bul, yeniden adlandır;
  - anlamsal renklendirme;
  - test gezgini ve satır üstünde çalıştır ya da hata ayıkla düğmeleri;
  - tensör şekillerinin satır içinde gösterimi, eğitimi canlı izleyen panel.
- **İzleme Masası:** Devam düğmesi; gradyan ve tensör özeti.
- **Transformer kitaplığı:** kitaplık adlarının yazım dillerinde çevirisi, hazır eğitim döngüsü, iyileştiricide sonradan değiştirilebilen öğrenme oranı.
- **Kodlu saklama:** otomatik sınamanın kodlu biçimi denetlemesi; depodaki örneklerin ve testlerin kodlu biçime çevrilmesi.
- **Standart kitaplık:** sıkıştırma (zip), özet (hash), tarih ve saat, düzenli ifadeler; her ad önce tasarlanıp onaylanır.

## Evre 6: Ölçek ve yerel çıkarım

- **Yerel çıkarım:**
  - GGUF dışa aktarma: önce F16/BF16 ve Q8_0, sonra Q4_K ailesi;
  - yalnızca ağırlık int8/int4 nicemleme ve NF4: matris çarpımı içinde anında geri açan tek bir çekirdek ailesi hem QLoRA eğitimine hem sunuma hizmet eder.
- **Çoklu GPU:**
  - NCCL ile veri paralelliği (DDP): kova sırası sabit (belirlenimcilik için), fp32 biriktirme, hesapla örtüşen iletişim, gradyan biriktirirken eşitlemeyi atlama;
  - her GPU'ya kendi veri parçası (Evre 3'teki belirlenimci sıra üstünde);
  - ardından ZeRO-1 (ek iletişim hacmi yok). Bölme parametre başınadır, kontrol noktasındaki adlar değişmez.
- **GPU otomatik sınaması:** gradyan kapısı ve GPU testleri zamanlanmış olarak, en az iki kart türünde (T4 ve L4).
- **Veri:** Parquet ve jsonl akışı.

**Bitiş ölçütü:**
- Aynı eğitim iki GPU'da veri paralel koşar ve tek GPU'yla aynı sonucu verir.
- Model GGUF olarak dışa aktarılır ve llama.cpp'de çalışır.
- GPU testleri otomatik koşar.

## Evre 7: Yaygınlaşma

- **Model Sihirbazı:** kod bilmeyenin bile model kurmasını sağlayan, okunur TR kodu üreten sihirbaz.
- **Az teknik kullanıcı yolu:**
  - ham metinden veri kümesi hazırlama;
  - parçalayıcı eğitme;
  - çalıştırmadan önce süre, bellek ve maliyet tahmini;
  - küçük model tarifleri;
  - eğitilen modeli tek dosya uygulama ya da web hizmeti olarak paylaşma.
- **Sunum ölçeği:** sürekli (iterasyon düzeyinde) toplu işleme ve sayfalı KV önbelleği, eşzamanlı kullanıcı olunca.
- **Model kapsamı:**
  - görüntü ve ses: evrişim (1B/2B), havuzlama, yukarı örnekleme, BatchNorm/GroupNorm, görüntü çözme (PNG/JPEG);
  - karşıtsal kayıp;
  - MoE: en iyi k yönlendirme ve gruplu çarpım.
- **Büyük ölçek:** FSDP/ZeRO-3, tensör, boru hattı ve bağlam paralelliği; bulut depolamadan (S3/GCS) veri akışı.
- **Ekosistem:**
  - TR kitaplıkları için paket yöneticisi (kitaplık ekosistemi oluşunca);
  - deney takibi aktarımı (TensorBoard, MLflow; `.jsonl` günlük taban olarak yeterli);
  - değerlendirme kitaplığı (OpenAI uyumlu sunucuyla lm-evaluation-harness bunu büyük ölçüde karşılar).
- **Yazım dilleri:** adların İngilizcesi var (`en.json`, `tr göster --dil en`); iletilerin İngilizcesi (`internal/lang/messages/en.json`) eklenir.
- **Platformlar:** Mac otomatik sınaması, Apple M ve ARM için vektör çekirdekleri, Metal arka ucu, AMD (ROCm).

## Sonra

- **FP8 eğitimi** (Hopper/Blackwell): 100 M–1 B ölçeğinde kazanç küçüktür. Başladığında yalnızca doğrusal katmanlarda başlar; dikkat, norm, gömme ve çıkış başı bf16'da kalır.
- **TR ile çekirdek yazma (karo dili):**
  - Triton/TileLang uzlaşısını izler:
    - ızgara üzerinde bir program;
    - derleme anında şekli belli karolar;
    - maskeli yükleme ve saklama, mma, indirgeme;
    - türlendirilmiş zamanlama düğmeleri;
    - satır içi PTX kaçış kapısı.
  - Otomatik ayarlama zorunlu değildir.
  - Bitiş: elle yazılmış norm, softmax, çapraz entropi ve AdamW çekirdekleri karo dilinde aynı hızda yeniden yazılır.
- **Genel otomatik kaynaştırma:** ancak küçük birleştirici ölçümle yetersiz kalırsa.
- **Bölme bilgisi tensör tipinde:** tip denetleyicisi belirsiz bölmeyi hata sayar (JAX'ın açık kipi emsali).
- **ONNX:** büyük işlem yüzeyi, kendi yürütme motoru olan bir dil için düşük getiri.
- **Statik şekil denetimi:** tensör şekil hatalarının çalıştırmadan önce yakalanması.
- **İleri yönlü ve yüksek mertebe türev**, uzun bağlam için dizi paralelliği, hiperparametre taraması.
- **Matematik:** doğrusal sistem çözme, ters matris, ayrışımlar (QR, Cholesky, tekil değer), hızlı Fourier dönüşümü; türevli, CPU ve GPU çekirdekli.

## Doğrulanacaklar

Kararlar bu soruların yanıtıyla gözden geçirilir; ilk ikisi TR'nin kendi ölçümüyle kapanır:

- 100 M ölçekli GPT'de Blackwell üzerinde `torch.compile`'a karşı adım süresi.
- Küçük modellerde L4 ya da RTX PRO 6000'de FP8 kazancı.
- RTX PRO 6000'in (SM120) hangi tensör çekirdeği komutlarını desteklediği (SM100'ün tcgen05/TMEM'i dahil mi): PTX ISA belgesinden.
- Colab oturumlarının kesilme sıklığı (kontrol noktası aralığını belirler).

## Platform durumu

- **Windows, Linux (Intel/AMD):** tam destek, her değişiklikte otomatik sınama.
- **Linux ve Mac (ARM):** derleniyor ve sınanıyor; vektör çekirdekleri yok (düz Go yolu).
- **GPU:** NVIDIA (sürücü ve isteğe bağlı cuBLASLt çalışma anında yüklenir); ölçümler RTX PRO 6000 Blackwell üzerinde, GPU testleri elle ve Colab'da.

## Tamamlananlar

- Yorumlayıcı: kapanış derlemesi, yuva tabanlı ortamlar; Python ile başa baş hız.
- Tensör çekirdeği: AVX2/FMA matris çarpımı (çalışma anında seçilir), ters yönlü otomatik türev, kalıcı işçi havuzu.
- Veri hattı: `.npy` parçaları, önden yükleme, önbellek; genel `tokenizer.json` parçalayıcısı.
- bf16/fp16 veri türleri ve `karışık_hassasiyet` bloğu; 16 bitlik ara gradyanlar kendi türünde, float32 ağırlığın gradyanı yuvarlanmadan.
- GPU arka ucu: Go'nun ürettiği GPU kodu, bütün tensör işlemleri ve gradyanları, tensör çekirdekli dikkat (ileri ve geri, baş boyu 64'e kadar), cuBLASLt ile bf16 matris çarpımı, çok tensörlü iyileştirici adımları, kontrol noktası ve gradyan kırpma cihazda.
- Kodlar: dilin bütün adları sabit kodlara bağlı (`internal/lang/codes`); açıklamalı başvuru `docs/kodlar.md`; kodlu saklama (`tr kodla`, `tr çöz`, `tr göster`) ve VS Code izdüşümü; İngilizce yazım dili.
- Tek yazım: Türkçe harfsiz biçimler, eş yazımlar ve İngilizce eşler kaldırıldı; yerleşik ad gölgeleme uyarısı.
- Matematik ve model yapı taşları: genişletilmiş eleman bazlı matematik, birleşik `dikkat`, `örnekle`, `parametreler(model)`, `türev_denetle`; Transformer kitaplığı (`tr/katman`: Doğrusal, RMSNorm, RoPE, gruplu sorgu dikkati, SwiGLU, GPT).
- Standart kitaplık: dosya ve yol işlemleri, komut satırı, günlük ve ilerleme çubuğu, kontrol noktası, profil ve canlı izleme.
- Kod tabanı İngilizceye geçti (klasörler, dosyalar, Go adları); her paketin README'si standartlarını yazar.
- Dış incelemeler: dil çekirdeği, hesap çekirdeği, standart kitaplık ve geliştirici araçları raporları doğrulanarak düzeltildi.
- Güvenlik: güvenilmeyen girdiler (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`) sınırlarla denetlenir ve bulanıklaştırma testleriyle sınanır; VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır; otomatik sınamada sabitlenmiş eylemler.
