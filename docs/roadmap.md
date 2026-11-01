# TR Yol Haritası

TR'nin amacı [README](../README.md)'dedir: yapay zekânın bütün işleri için bir programlama dili.

**Son ölçüm: 2026-11-01, main `aad41d1e`.** Bu belgenin her satırı depoya karşı ölçülerek yazıldı. Eski yol haritasının ([`docs/archive/roadmap-2026-10-27.md`](archive/roadmap-2026-10-27.md)) her maddesinin akıbeti de ölçüldü: bitti, taşındı, bırakıldı ya da buraya alındı. Arşivden gelip bugünkü durumu yeniden ölçülmemiş işler "(arşiv)" diye işaretlidir: iş kayıtlıdır, bugün hâlâ geçerli olduğu ölçülmemiştir.

## Yöntem

- Dil tanımı ([`docs/spec/`](spec/README.md)) dilin nasıl olması gerektiğini yazar; bugünkü uygulamaya göre değil, amaca göre.
- Her bölüm iki denetimden geçer:
  1. **kararlar:** Gemini araştırır, DENETİM doğrular;
  2. **metin:** DENETİM okur.
- Her kuralın bugünkü durumu ve yapılacakları [`status.md`](status.md)'dedir; iş testiyle kapanır ve oradan silinir.
- Bu belge işlerin sırasını, bölümü olmayan işleri ve kilometre taşlarını tutar.
- **Güncel tutma:** bir karar ya da iş değişince bu belge aynı gün, ölçülerek güncellenir. Eksik ya da yanlış bir satır yanlış yöne götürür; fark geç anlaşılınca boşa iş yapılmış olur.
- **Bekçi testler** (`internal/roadmap_test.go`):
  - eski yol haritasının numaralarına gönderme yok;
  - "Son ölçüm"ün commit'i HEAD'in atası;
  - bölümlerin denetim durumu burada kopyalanmaz, yalnız status'ta yazılır.

  Testler ölçümün yerine geçmez.

## Durum

- **Dil tanımı:** 20 bölümün hepsi yazıldı (1–13, 15–20; 14 yok). Her bölüm DENETİM'den ve Gemini'nin karar araştırmasından en az bir kez geçti. Bölüm başına kaydı `status.md`'nin bölüm başlarında.
- **Açık denetimler** (sıradaki dil tanımı turunda):
  - Uygulama sırasında verilen kararlar DENETİM'den geçti, ama Gemini'nin karar araştırmasından geçmedi. Liste yalnız `status.md`'de: "Uygulamada verilen, dil tanımı turunda denetlenecek kararlar" bölümü ve bölüm başları. Burada kopyası tutulmaz.
  - 12, 16, 17 ve 20. bölümlerin kapanış turu: bulguları işleyen commit'in farkını DENETİM'in yeniden okuması. 10, 11, 18 ve 19'da yapıldı.
- **Uygulama fazlar hâlinde ilerliyor.** Her fazın işleri dosya sahipliğine göre ayrılır, paralel kollar aynı dosyaya dokunmaz.
  - **Faz 0, bitti:** toplu ve sıralı işler (kancalar, ad göçleri, ortak sorgular).
  - **Faz 1, sürüyor** (kolların günlük durumu bu belgede tutulmaz; ana oturum depo dışında takip eder):
    - **4a, denetleyici:** çıkarım, tipsiz parametre ve alan, değeri kullanılmayan çağrı, metodun sayısı ve tipi, imzaların öğe tipi, koşulun tipi;
    - **D1, çalışma anı:** hata değerleri, ertele, görev ve iptal, sıralama, JSON;
    - **S1, sistem:** modüller, testler, veri, kodlar, dosya, süreç;
    - **T1, tensör çekirdeği;**
    - **iletiler:** önerinin ve kod örneğinin kimlikle yazılması.
  - **Faz 2:**
    - **4b, akış:** daraltma, kapsama, eksik dönüş;
    - **4c, genel tipler;**
    - **T2:** sinir ağı ve eğitim;
    - **T3:** hız, dosya ve nicemleme;
    - **K:** uzun yerleşiklerin kilidi.
- **Komut adı:** `tr` yerine `trlang` (Ahmet, 2026-10-30). Unix'in `tr`'si aynı adla sessizce başarılı çıkıyor; `trl` Hugging Face TRL'nin komutu. Takma ad yok. Göç Faz 1'de, iletiler 3'ten sonra ve ajan deneyinin zor-3'ünden önce yapılır; eski adı yasaklayan bir testle.

## Açık dil tasarım soruları

Dilin davranışı: ana oturum karar verir, Gemini araştırır, DENETİM doğrular. İlgili bölümün sıradaki turunda karara bağlanır.

- **Nesne kimliğinin sınanması:** iki ad aynı nesneyi mi gösteriyor (paylaşılan ağırlıklar, 15). 7.6 paylaşmayı yazıyor, sınamayı yazmıyor.
- **Salt okunur görünüm ya da derin değişmezlik:** `TR_const` yalnız bağı sabitler.
- **Yapı alanlarının ve metotlarının gizliliği:** 11.7'nin `_` kuralı yalnız modülün üst düzey adlarını kapsar. Yapının `_x` alanı dışarıdan okunuyor; 7'de karar yok. Kitaplığın iç yardımcıları da buna bağlı.
- **Ağırlıkların kurala göre gruplanması** (iyileştiricinin parametre grupları): elle gruplama 15'te var, otomatik gruplama için karar yok.

Karara bağlananlar (2026-10-27'deki listeden):
- anahtarla sıralama: 13.3, `.TR_sort(f)`;
- sondaki bloktan erken çıkış: 6.7, içinde `TR_return` sözdizimi hatası;
- JSON'dan tipli yapıya okuma: 13.8, `TR_json_parse[Ayar](metin)`;
- iletilerdeki örnek kod: kimlikle yazılır (`{#ad}`, `internal/lang/messages/README.md`).

## Ahmet'in kararını bekleyenler

Kapsam ve kaynak kararları.

- **Hazır nicemli modeller:** GGUF okumak mı (15.55 "Sonraya" diyor), yoksa yalnız kendi `q4`/`q8` biçimi mi (15.51–15.53)? 2. kilometre taşının nicemli ağırlık ölçütü buna bağlı.
- **`.pt` dosyası:** yazmak ve kısıtlı okumak (arşiv). Bırakıldığı yazılı değil; güvenlik tartısı ister.
- **Python köprüsü:** eski yol haritası "bilinçli olarak yok" diyordu (arşiv, İlkeler). Bu karar bugün hiçbir belgede yazılı değil.
- **1. kilometre taşının kalanları:** RGN-512 hedef olmaktan çıktı; eğitim hızı adaylarının yapılıp yapılmayacağı (aşağıda).
- **Kilometre taşı adayları** (en sonda).

## Bölümü olmayan işler

Dil tanımında hiç bölümü olmayan, ama amacın bir işinin yazılabilmesi için gereken konular. Her biri kendi bölümü olarak, öbür bölümlerin yöntemiyle yazılır: dokuz iş tartısı, kaynaklı emsal, iki denetim.

**Sunmak ve uzaktaki model:**
- **Ağ:** HTTP sunucusu ve istemcisi, akışlı cevap (metin üretiminin parça parça gönderilmesi), zaman aşımı ve yeniden deneme. Olmadan "sunmak" ve "uzaktaki modeli kullanmak" yazılamaz.
- **Toplu sunum:** sürekli toplu işleme ve sayfalı KV önbelleği (arşiv).
- **Hizmet günlüğü:** istek kayıtları (düzeyler, istek kimliği), uzun çalışan programın günlüğünün döndürülmesi. 19'un `TR_Logger`'ı eğitim günlüğüdür.
- **Belgelerden bilgi bulma:** gömme modeli ve benzerlik araması. CLAUDE.md "belgelerden bilgi bulan sistemler" der; Jarvis araştırması da eksik buldu.

**Veriyi hazırlamak:**
- **Görüntü, ses ve video çözme:** PNG, JPEG, WAV ve benzerlerinin baytlarından tensöre ve geri; büyük dosyayı akışla okumak; dosyanın değiştirilme zamanı. Görüntü ve ses modelleri bunu ister.
- **Canlı ses giriş ve çıkışı:** mikrofon ve hoparlör. Jarvis ve robot ister; yukarıdaki madde yalnız dosyayı kapsar.
- **Veriyi toplamak ve temizlemek:** yinelenen veriyi ayıklamak, süzmek, metni dosyadan ve ağdan toplamak. Bunun için gerekenler:
  - **hash:** sha256 ve xxhash gibi hızlı hash'ler; yineleneni ayıklamak, veri kümesini doğrulamak, önbellek anahtarı;
  - **düzenli ifade:** metin temizleme ve parçalayıcının ön ayırması;
  - **Unicode:** NFC ve NFKC normalleştirme, kod noktası ile karakter arasında dönüşüm.
- **Dosya biçimleri:**
  - zstd ve zip (`.jsonl.zst`; 18.12'nin `.gz` saydamlığı gibi);
  - Parquet;
  - CSV yazmak;
  - bayttan sayı okuma ve sayıyı bayta yazma (bayt sırasıyla: WAV, `.npy`, algılayıcı paketi; 3.3a).
- **Uzak depolama:** S3 ve GCS'ten akışla okuma (arşiv).
- **Alt süreç:** dış bir programı (ffmpeg gibi) çalıştırıp beklemek; 10.3'ün "dış süreç beklemeleri" ona bağlıdır. Güvenlik tartısı gerekir.
- **Süreçlerle paralellik:** TR koduyla yazılmış veri işi yorumlayıcı kilidi yüzünden tek çekirdekte kalır; işi süreçlere bölmek.

**Eğitmek ve ölçmek:**
- **Dağıtık eğitim:** birçok GPU ve kümede eğitim, verinin süreçler arasında bölünmesi, dizi paralelliği.
- **Değerlendirme:** amacın "Ölçmek ve değerlendirmek" işinin bölümü yok. Ortak sınavlar (lm-evaluation-harness ile aynı sınav), değerlendirme kitaplığı (arşiv).
- **Deney takibi ve hiperparametre taraması** (arşiv).

**Dil ve çalışma ortamı:**
- **Standart hataya yazmak:** programın kendi kullanım hatasını ve hizmetin hata çıktısını standart hataya yazacak bir yol yok. 19.10 bunu ister; yeni ad ya da adlı argüman gerekir. `TR_command_line[Ayar]()` ile zorunlu seçenek de 4c'den sonra gelir.
- **Tarih ve saat** (arşiv).
- **Bellek ve gerçek zaman:** çöp toplayıcının duraklamasının robotun sabit hızlı döngüsüne etkisi; ölçüm ve gerekirse sınır. Yorumlayıcı kilidinin gecikmesi 10.2'nin kuralıdır; ölçümü buraya girer.
- **Yorumlayıcı hızı:** arşiv yorumlayıcının Python'dan yaklaşık 1,5 kat yavaş olduğunu yazıyor [kanıtsız: arşivin sözü, bugün ölçülmedi].
- **Paket, bağımlılık ve sürüm sabitleme:** başkasının modülünü sürümüyle almak, kilit dosyası. `tr/` kitaplığı ikiliyle gelir, ona gerekmez (11.6).
- **Sürüm politikası:** 0.y.z numarası; kararlı, beta ve prototip etiketleri; dil ve dosya biçimini dondurma kuralları; yayın ikilileri (arşiv).
- **Hazır modelin sohbet biçimi:** sohbet şablonu (`tokenizer_config.json`'un şablonu) ve özel parçalar; "sohbet etmek" ister.
- **Karo dili:** GPU çekirdeklerini TR'de yazmak (arşiv).
- **Güvenlik denetimi:** saldırı yüzeyinin baştan sona denetimi (dosya biçimleri, ağ, komut satırı) (arşiv).

## 15. bölümün sonraya bıraktıkları

Her biri gelince 15'in yöntemiyle yazılır.

- **"Paylaşmak"ın öbür biçimleri:**
  - ONNX ve GGUF'a dışa verme; GGUF okuma ve K-nicemleme ailesi (15.47a, 15.55);
  - PEFT bağdaştırıcı dosyasını okuma ve yazma (LoRA'nın hazır araçlarla paylaşılması).
- **Başka hızlandırıcılar:** AMD (ROCm), Apple, gömülü NPU; cihaz adları 15.37'nin kuralıyla.
- **Türev:**
  - etkinleştirme kaydı (`TR_recompute`);
  - ikinci türev ve `retain_graph` (15'in gerekçesi buraya gönderiyor);
  - ileri yönlü türev.
- **Tensör işlemleri:**
  - çok eksenli indeks;
  - `TR_stack`, `TR_pad`, `.TR_repeat` (15'in gerekçesi `TR_pad` ve çok eksenli indeks için buraya gönderiyor);
  - doğrusal cebir ve FFT.
- **Sinir ağı işlemleri:**
  - havuzlama ve büyütme, ters evrişim (15.23c);
  - CTC ve STFT (ses modelleri);
  - BatchNorm;
  - uzman karışımı (MoE): yönlendirici ve koşullu uzman çarpımı. Qwen3-30B-A3B ve gpt-oss bunu ister.
- **Kayıp ve başlatma:** BCE, karşıtsal kayıp, z-loss; Xavier ve Kaiming başlatma.
- **İyileştirme:** μP, Schedule-Free, düşük bitli iyileştirici durumu, katman başına güncelleme/ağırlık oranı.
- **Gözlemlenebilirlik:** parametre normu, etkinleştirme RMS'i, log Z, MFU (15.36'nın yanında; arşiv).
- **Nicemleme ve hassasiyet:**
  - sıra değiştiren gruplu GPTQ (`desc_act`, 15.52);
  - NF4, 2 ve 3 bitlik biçimler;
  - nicemlemeye duyarlı eğitim (QAT);
  - ARM NEON çekirdekleri (15.51–15.53);
  - FP8 eğitimi ve kazancının ölçümü;
  - T4 için fp16'da dinamik kayıp ölçekleme.
- **GPU belleği:** programın GPU bellek kullanımını okuyup günlüğe yazması.
- **Tensör tipi:** bölme (shard) bilgisinin tensörün tipinde taşınması (arşiv).
- **Kontrol noktası** (20'nin dışında kalanlar, arşiv): eğitimi durdurmadan eşzamansız kayıt; sürdürmede öğrenme oranının ölçeklenmesi.
- **Örnekleme:** `TR_sample`'a `TR_top_p:`. Bugün top-p 15.22'nin `TR_argsort`'u ve `TR_cumsum` ile yazılır (15.26).

## Ajan hata oranı deneyi

Kodlama modelleri TR'yi doğru yazıyor mu, yanlış yazınca hata gürültülü mü; aynı görevler Python'la karşılaştırılır (Ahmet, 2026-10-27).

- **Önceden konan eşikler:**
  - A'da yalnız başvuruyla yazılan programların en az %70'i kural ihlalsiz;
  - B'de TR'de ilk deneme başarısı Python'unkinin en az %85'i;
  - B'de 3 turda düzelme en az %95;
  - B'de TR kuralından doğan sessiz yanlış 0.
- **Evre A, yazım (2026-10-27):** üç Claude modeli, 20 görev, iki bağlam. 120 programın 120'si ihlalsiz, eşik geçti.
- **Evre B, çalıştırma (2026-10-30, Faz 0'dan sonra):**
  - **1. koşu (Gemini, 20 görev):** üç koşul da eşikleri geçti. Ama Python da 20/20 aldı; görevler TR ile Python'u ayırt etmiyor.
  - **Zor görevler** (zor-1, zor-2; `sekerahmet/tcb-research`):
    - Aracı çalıştırıp `tr` ile deneyen modellerde (Claude, Gemini, Haiku, GLM, Grok, Kimi, arena ajanı) sessiz yanlış 0.
    - Araçsız tek denemede iki sessiz yanlış TR kuralından çıktı (grok-4.5-search, Qwen 3.8 Max), ikisi de değeri atılan `.TR_sort()`. Bu, "sessiz yanlış 0" eşiğini bozdu. Aynı alışkanlık DeepSeek'te de var, orada çıktıyı etkilemiyor.
    - 4a2 bu kalıbı denetim hatası yaptı (13.10, H1115).
    - Öbür iki sessiz yanlış programın mantığından (Python'da da aynı).
  - **Sıradaki koşu, zor-3:** TR'nin yeni yerlerinden görevler (demet, `T?`, durumlu tip, `TR_match`, JSON, hata anları), İngilizce koşul, `trlang` adıyla.
- **Önekin bedeli:** kimlikle yazılan kod, İngilizce görünümden %22 fazla parça tutuyor; Türkçe görünümden %8 az [ölçüldü: DENETİM, 2026-11-01, Qwen3'ün `tokenizer.json`'u; 40 örnek ve üç kitaplık dosyası, yalnız program satırları, 1870 satır]. Dil kararını değiştirmez; ajanın bağlam bedeli olarak izlenir.

## Araç işleri

Dilin kuralı değil; kodlama modellerinin ve insanların dili rahat kullanması için.

- **Makinece okunur dilbilgisi dosyası:** model kod üretirken sözdizimine uymayan sözcüğü baştan dışarıda bırakabilsin.
- **Kodlama modeline tek sayfalık başvuru:** var (`sekerahmet/tcb-research`, Evre A ve B). Zor-3'ten önce düzeltilir: anahtarlı sıralama (dilde var: `.TR_sort((x) => anahtar)`, 13.3, 2026-11-01), `TR_defer` yalnız blokla (9.1), sık hatalar.
- **Claude Code kancaları** ([`tools/edithook`](../tools/edithook/README.md)): her kanca ajanı gerçekten doğru şeye yönlendiriyor mu, bedeli ne, diye tartılır. Açık kalanlar:
  - düzenleme kancası kabuktan (Python, sed) yapılan düzenlemeyi görmez;
  - bilgi listesinin hangi testleri öne alacağı;
  - kancaların kendi bedeli (her çağrıda derleme).
- **Geliştirici araçları** (arşiv):
  - **dil sunucusu:** eklentide bugün yalnız tamamlama var; tanıma gitme, kullanımlar, yeniden adlandırma ve öbürleri;
  - **hata ayıklayıcı:**
    - DAP bitiş ölçütü: hata ayıklayıcının uçtan uca sürüşü;
    - kesme noktası bir sonraki deyimde duruyor;
    - F5'te `TR_input()` çalışmıyor;
    - isabet sayılı kesme noktası ve iz noktası, hata fırlatılınca durma;
    - tensör özeti, "kayıp NaN olunca dur";
  - **İzleme Masası:** gradyan ve tensör özeti;
  - not defteri;
  - Model Sihirbazı; süre, bellek ve maliyet tahmini;
  - `TR_print(model)`'in özet görünümü;
  - dizinde ve eklentide modüle özel adların gizlenmesi.
- **Depo bakımı** (arşiv):
  - kararsız testler, ölü metotlar;
  - `.claude/agents`'teki eski yollar;
  - belge testinin araç README'lerini ve `gaps.md`'yi almaması;
  - `tools/gpu` betiklerinin karışık anahtarları;
  - `rgn512.tr`'nin `TR_Run`'a geçmesi ve `benchstat` ile hız karşılaştırması;
  - `recipes.md`'ye "bir adı kaldırmak" tarifi;
  - Mac'te otomatik sınama, GPU testlerinin zamanlanmış sınaması;
  - Colab oturumlarının kesilme sıklığının ölçümü (kontrol noktası aralığının varsayılanı için).
- **Status'a girecekler, önce ölçülecek** (arşivden; bugünkü durumları ölçülmedi):
  - 17.7: Go'nun Türkçeye düşen yerleri (`codes.Source()` çağrıları, `Inspect` metni, Türkçe ad önerileri);
  - 17.3: `MemoryPlace`'in metin yerine kod taşıması;
  - 17.8: metot kullanım metinlerindeki parametre adları;
  - 20.1: kontrol noktasına adım sayısıyla kayıt.

## En sonda: gösterim dilleri

Ahmet, 2026-10-27: "bu ayrı yeni bir spec olur en sonda". Kod her zaman kimliklerle yazılır, hiçbir insan dilinde yazılmaz. Ayrı bir dil tanımı bölümü olarak, öbür bölümler bittikten sonra tasarlanır:
- kodun okunabilmesi için bir insan diline çevrilmesi;
- diller arası ad çakışması;
- değer metninin dili;
- çok dilli ekip.

## Kilometre taşları

Kilometre taşı, dili gerçek bir işte sınayan ve bitiş ölçütü ölçülebilen bir hedeftir. Sıradakiler bir öncekinden sonra onayla eklenir.

### 1. kilometre taşı: RGN-512'yi TR'de eğitmek (büyük ölçüde yapıldı)

- **İş:** PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş bir dil modelini (`tools/gpu/rgn512.py`) TR'de yazıp (`tools/gpu/rgn512.tr`) aynı veriyle eğitmek.
- **Sonuç, 2026-10-23'te bitiş ölçütü sağlandı:**
  - 0,8 milyar parçalık iki tohumlu koşuda PyTorch'tan daha düşük doğrulama kaybı;
  - bellekte derlenmiş PyTorch'la başa baş;
  - adım süresi derlenmiş PyTorch'un 1,07 katı, yani TR %7 yavaş (RTX PRO 6000; A100'de 1,15) [kanıtlı: `docs/archive/roadmap-2026-10-27.md:17,133,232`];
  - üç kart türünde (RTX PRO 6000, A100, L4) GPU testleri geçiyor.

  Ayrıntı [arşivde](archive/roadmap-2026-10-27.md).
- **Ahmet, 2026-10-28:** büyük ölçüde yapıldı; RGN-512 artık dilin ölçü modelidir, hedef değildir.
- **Kalanlar** (arşiv; hedef olmaktan çıktığı için karar bekliyor):
  - eğitim hızının dokuz adayı (çekirdek kaynaştırma, cuBLASLt epilogları, eğitimde CUDA Graph …); artan adım süresi önce, sızıntı olabilir;
  - A100'deki çekirdek farkı;
  - "aynı tarifle TR neden daha düşük kayıp veriyor" araştırması.

### 2. kilometre taşı: hazır modelleri CPU'da ve GPU'da hızlı çalıştırmak (şimdiki)

Ahmet, 2026-10-27. Hazır modeli çalıştırmak amacın en yaygın işidir; modeli sunmak, robotun içinde çalıştırmak ve ince ayar ona dayanır.

**Bitiş ölçütü:**
- Bir dil modeli (Llama ya da Qwen3 ailesi), bir görüntü modeli ve bir ses modeli (Whisper gibi konuşma tanıma) hazır ağırlıklarıyla çalışır ve özgün uygulamayla aynı sonucu verir.
- Hız aynı model, aynı hassasiyet ve aynı makinede ölçülür: GPU'da PyTorch ve llama.cpp'ye, CPU'da PyTorch CPU ve llama.cpp'ye karşı. Hedef oran ilk ölçümden sonra buraya yazılır.
- CPU'da hem x86 hem ARM64 işlemcide çalışır.
- Ağırlık 16 bit ve nicemlenmiş (8 ve 4 bit) olarak yüklenir.

**Bugün** (ölçüm 2026-10-22, Colab, ayrıntı arşivde):
- **GPU'da üretim:** transformers'ın en hızlı yolunu geçti, llama.cpp f32'nin %63–83'ü [kanıtlı: `docs/archive/roadmap-2026-10-27.md:373`].
- **CPU'da üretim:** llama.cpp'nin yaklaşık 1/5'i. Bu ölçüm Colab'ın EPYC sunucusunda, 48 iş parçacığıyla ve float32'ye float32 yapıldı (Qwen3-0.6B: TR 12,1, llama.cpp 47,5 parça/sn).
  - Geliştirme dizüstünde (i7-13700H, iki 16 GB DDR5-4800 modülü; çift kanalsa teorik 76,8 GB/sn, kanal sayısı ölçülmedi [ölçüldü: `Win32_PhysicalMemory`, 2026-11-01]) henüz ölçülmedi.
  - Farkın nedeni profille bulunmadı.
- Yalnız dil modelleri çalışıyor. Evrişim ve ses işlemleri, nicemleme ve ARM vektör çekirdekleri yok.

**Ölçüte varmak için eksikler** (dil tanımında olanlar status'ta; olmayanlar burada):
- **Yükleme:**
  - safetensors'u bellek eşlemeyle (mmap) kopyasız yüklemek; bugün dosya belleğe kopyalanıyor;
  - birden çok dosyaya bölünmüş modeli okumak (`model.safetensors.index.json`); 8 milyar ve üstü modeller böyle gelir;
  - Mistral mimarisi;
  - açılış süresi.
- **Üretim:**
  - KV önbelleği için halka tampon, 8 bitlik KV önbelleği;
  - hazır istemin önbelleğini saklamak (önek önbelleği);
  - tahminli çözme.
- **CPU:**
  - 16 bitlik ağırlığı çarpımın içinde açan çekirdek (bugün her çarpımda bütün matris geçici float32 tampona açılıyor);
  - `relu`, `sigmoid`, `silu`'nun vektörlenmesi;
  - üretimde işçi sayısının ölçümü;
  - katman başına çekirdeklerin kaynaştırılması.
- **Kartlar:** T4 ve H100'de sınama; SM120'nin tensör çekirdeği komutları.
- **Model seçimi:** bu dizüstü için en uygun açık modeller Gemini'nin araştırmasında (2026-11-01; DENETİM okudu, düzeltmeleriyle). Uzman karışımlı Qwen3-30B-A3B ve yoğun Qwen3-8B öne çıkıyor; uzman karışımı 15. bölümün sonraya bıraktıklarında.

### Kilometre taşı adayları (Ahmet'in kararı bekleniyor)

- **Yerel sesli asistan, "Jarvis"** (Ahmet, 2026-10-31: "robottan önce bir Jarvis", kabaca eklendi):
  - TR ile yazılmış, yerelde çalışan bir sesli asistan. Robotun "beyin ve ses" kısmı; yayımlanmış modeller kullanılır, tutkal, çalıştırma ve ajan döngüsü TR'de yazılır.
  - Adımlar (Gemini araştırması ve DENETİM, 2026-10-31):
    1. yerel metin sohbeti;
    2. dosyadan ses ve Whisper;
    3. ses üretimi ve canlı ses;
    4. söz kesme (görev iptali, kilidin hızlı devri);
    5. belgeler ve hafıza;
    6. yerli ses giriş ve çıkışı ile CPU nicemleme.
  - Bitiş ölçütü ve sıra Ahmet'in kararı.
- **Uyarlamak:** hazır bir modeli ince ayarla kendi işine uyarlamak, uçtan uca. LoRA'yı dil yazabiliyor; ama PEFT düzeninde kayıt, ince ayar örneği ve bitiş ölçütü yok (arşiv).
- **Herkes için:** biraz teknik bilgisi olan birinin kendi modelini kurup eğitmesi. Model Sihirbazı, süre ve maliyet tahmini, küçük model tarifleri (arşiv).
- **Masaüstünde çalışan bir TR modeli** (Ahmet, 2026-11-01: "önce TR'yi bitirelim, tüm fazları; ondan sonra"): açık ağırlıklı, Türkçe ve İngilizce bilen bir model, bu dizüstünde en hızlı çalışacak biçimde; doğrulanmış TR programlarıyla ince ayar. Fazlardan sonra.
