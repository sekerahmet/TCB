# TR Yol Haritası

TR'nin amacı [README](../README.md)'dedir: yapay zekânın bütün işleri için bir programlama dili.

## Yöntem

Dil tanımı ([`docs/spec/`](spec/README.md)) baştan, bölüm bölüm yazılıyor. Her bölüm amaca göre yazılır ve iki denetimden geçer: kararlar doğru mu (Gemini araştırır, DENETİM doğrular) ve metin doğru mu (DENETİM). Sonra kararları uygulanır. Bu yol haritası yalnız yeniden yazılmış bölümlerden gelen işleri tutar. Eski yol haritası arşivdedir ([`docs/archive/roadmap-2026-10-27.md`](archive/roadmap-2026-10-27.md)); oradaki bir iş, ilgili bölüm yeniden yazılınca o bölümün kararlarına göre buraya gelir.

## 1. bölüm: Program

Yazıldı; DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27). Her maddenin bugünkü durumu ve yapılacakları [`status.md`](status.md)'dedir; her iş testiyle kapanır.

## Sıradaki

1. ve 2. bölüm DENETİM'den ve Gemini'nin araştırmasından, 3.–7. bölüm DENETİM'den geçti; 1–7 Gemini'nin bağımsız denetiminden de geçti, doğrulanan bulguları işlendi (2026-10-27). Ajan hata oranı deneyinin Evre A'sı bitti (aşağıda).

**Açık tasarım soruları** (Gemini ve DENETİM, 2026-10-27; ilgili bölüm yazılırken karara bağlanır):
- anahtarla sıralama (`sorted(d.items(), key=...)`); demet tipi 2026-10-28'de karara bağlandı (3.6a), anahtarla sıralamanın yazımı açık;
- nesne kimliğinin sınanması (paylaşılan ağırlıklar, 15. bölüm);
- salt okunur görünüm ya da derin değişmezlik (`TR_const` yalnız bağı sabitler);
- sondaki blokta erken çıkış;
- JSON'dan tipli yapıya okuma 2026-10-28'de karara bağlandı (13.8: `TR_json_parse[Ayar](metin)`). Dil tanımı kodlu biçimi, yani kimliklerle yazılmış programı anlatır.

**Bölümü olmayan işler** (amacın dokuz işine karşı tartı, ana oturum ve DENETİM, 2026-10-28; Ahmet: "Evet, ekle"). Dil tanımında hiç bölümü olmayan, amacın bir işinin yazılabilmesi için gereken konular. Her biri kendi bölümü olarak, öbür bölümlerin yöntemiyle (dokuz iş tartısı, kaynaklı emsal, iki denetim) yazılır:
- **Ağ:** HTTP sunucusu ve istemcisi, akışlı cevap (metin üretiminin parça parça gönderilmesi), zaman aşımı ve yeniden deneme. Olmadan "sunmak" ve "uzaktaki modeli kullanmak" yazılamıyor.
- **Görüntü, ses ve video çözme:** PNG, JPEG, WAV ve benzerlerinin baytlarından tensöre ve geri; büyük ses ve video dosyalarını akışla (parça parça bayt) okumak ve dosyanın değiştirilme zamanı (18'in tartısı). Veriyi hazırlamak ve görüntü ile ses modelleri bunu ister; 2. kilometre taşının görüntü ve ses modelleri de.
- **Standart hataya yazmak:** programın kendi kullanım hatasını ve hizmetin hata çıktısını standart hataya yazacak bir yol yok (19.10 kullanım hatasını standart hataya ister); yeni ad ya da adlı argüman, karar gerekir. `TR_command_line[Ayar]()` ile zorunlu seçenek (çıkış 2) de 4c'den sonra (Gemini Koşu 5).
- **Unicode normalleştirme ve kod noktası:** metni NFC ya da NFKC'ye getirmek (parçalayıcının eğitim derlemi, 16.4) ve kod noktası ile karakter arasında dönüşüm. Yeni adlar; karar ve onay gerekir (Gemini Koşu 6).
- **Hash:** sha256 ve hızlı hash'ler (xxhash gibi); yinelenen veriyi ayıklamak, veri kümesinin doğrulanması, önbellek anahtarı. Sarılan aritmetik (3.1) TR'de yazmayı mümkün kılar, hazır yerleşik hızı verir.
- **Düzenli ifade:** metin temizleme ve parçalayıcının ön ayırması (GPT tarzı ön bölme).
- **Unicode normalleştirmesi:** NFC ve NFKC; parçalayıcı eğitimi ve metin hazırlama.
- **Bellek ve gerçek zaman:** çöp toplayıcının duraklamasının robotun sabit hızlı döngüsüne etkisi; ölçüm ve gerekirse sınır (tasarım yönteminin açık konusu). Yorumlayıcı kilidinin gecikmesi 10.2'nin kuralıdır (süreyle devir); ölçümü de buraya girer.
- **Alt süreç** (2026-10-29, 19'un tartısı): veri hazırlamada dış bir programı (ffmpeg gibi) çalıştırmak ve çıktısını beklemek; 10.3'ün "dış süreç beklemeleri" ona bağlıdır. TR bugün alt süreç açmaz. Güvenlik tartısı gerekir (komutun metinle kurulması); "Görüntü, ses ve video çözme" gelirse ihtiyaç küçülür.
- **Hizmet günlüğü** (2026-10-29, 19'un tartısı): sunulan modelin istek kayıtları (düzeyler, istek kimliği) ve cihazda uzun çalışan programın günlüğünün boyut sınırı (eski dosyaların döndürülmesi). 19'un `TR_Logger`'ı eğitim günlüğüdür.
- **zstd sıkıştırması** (2026-10-29, 18'in tartısı): büyük derlemlerin yaygın biçimi (`.jsonl.zst`); 18.12'nin `.gz` saydamlığı gibi okuma ve yazmada.
- **Paket, bağımlılık ve sürüm sabitleme** (2026-10-29, 11'in tartısı): başkasının modülünü sürümüyle almak ve sürümü bir kilit dosyasında sabitlemek; amacın "Paylaşmak"ı ve "Güvenilir"i (aynı program aynı sonucu) ister. `tr/` kitaplığı ikiliyle gelir, ona gerekmez (11.6).
- **Süreçlerle paralellik** (2026-10-29, 10'un tartısı): TR koduyla yazılmış veri temizleme ve süzme yorumlayıcı kilidi yüzünden tek çekirdekte kalır; işi süreçlere bölmek (Python'un `multiprocessing`'i gibi). Amacın 1. işi ister.
- **Bayttan sayı okuma ve yazma** (bayt sırasıyla; 3.3a) ve CSV yazmak, yüzdelik gibi küçük yerleşikler; ilgili bölümler (16, 18, 13) yazılırken.
- **Veriyi toplamak ve temizlemek** (2026-10-29, 16'nın tartısı): yinelenen veriyi ayıklamak (hash ile), süzmek, Unicode normalleştirmesi, metni dosyadan ve ağdan toplamak. 16 parçalayıcıyı ve veri parçalarını yazar; toplama ve temizleme 13'ün metin ve liste işleriyle, yukarıdaki hash, düzenli ifade ve normalleştirmeyle yazılır.
- **Dağıtık eğitim:** birçok GPU ve kümede eğitim, verinin süreçler arasında bölünmesi (örnekleyicinin sıra ve sayı argümanı). Amacın 2. işi ister.
- **Hazır modelin sohbet biçimi:** sohbet şablonu (`tokenizer_config.json`'un şablonu) ve özel parçalar; amacın 4. işinin "sohbet etmek"i ister. Hazır modelleri yükleyen bölümle birlikte.

**15. bölümün sonraya bıraktıkları** (2026-10-29, 15'in yeniden yazımı ve DENETİM'i). Her biri gelince 15'in yöntemiyle (dokuz iş tartısı, kaynaklı emsal, iki denetim) yazılır:
- **"Paylaşmak"ın öbür biçimleri:** ONNX ve GGUF'a dışa verme; GGUF okuma ve K-nicemleme ailesi (`Q4_K` ...) (15.47a, 15.55).
- **Başka hızlandırıcılar:** Apple, gömülü NPU; cihaz adları 15.37'nin kuralıyla.
- **Türev:** etkinleştirme kaydı (`TR_recompute`) ve ikinci türev.
- **Sinir ağı işlemleri:** havuzlama ve büyütme, ters evrişim (15.23c), CTC ve STFT (ses modelleri).
- **Nicemleme:** sıra değiştiren gruplu GPTQ (`desc_act`, 15.52); NF4, 2 ve 3 bitlik biçimler, nicemlemeye duyarlı eğitim (QAT), ARM NEON çekirdekleri (15.51–15.53).
- **Bayt dizisi:** bayttan sayı okuma ve sayıyı bayta yazma (bayt sırasıyla: WAV, `.npy`, algılayıcı paketi), tensörden kurma, parça parça yazma (3.3a).
- **Örnekleme:** `TR_sample`'a `TR_top_p:` (bugün top-p, 15.22'nin `TR_argsort`'u ve `TR_cumsum` ile yazılır; 15.26).

**7. bölümden sonra: ajan hata oranı deneyi** (Ahmet, 2026-10-27, plan onaylı). Kodlama modeli TR'yi alışkanlıkla doğru yazıyor mu, yanlış yazınca hata gürültülü mü; aynı görevlerde Python'la karşılaştırılır.
- **Evre A** (1–7 yazılınca, uygulamadan önce): üç Claude modeli, 20 görev, iki bağlam (yalnız tek sayfalık başvuru; ya da 1–7'nin kuralları ve kimlik listesi). Programlar kural numarasıyla denetlenir; sık çiğnenen kural ya da başvuru yeniden tartılır. Başvuru (Araç işleri) Evre A'dan önce yazılır.
- **Evre B** (1–7 uygulanınca): aynı görevler yeniden yazdırılır, denetlenir ve çalıştırılır; en çok 3 düzeltme turu, yalnız TR'nin iletileriyle. Python ve Gemini'yle karşılaştırılır.
- **Önceden konan eşikler:**
  - A'da yalnız başvuruyla yazılan programların en az %70'i kural ihlalsiz;
  - B'de TR'de ilk deneme başarısı Python'unkinin en az %85'i;
  - B'de 3 turda düzelme en az %95;
  - B'de TR kuralından doğan sessiz yanlış 0.

**En sonda: gösterim dilleri** (Ahmet, 2026-10-27: "bu ayrı yeni bir spec olur en sonda"). Kod her zaman kimliklerle yazılır, hiçbir insan dilinde yazılmaz. Kodun okunabilmesi için bir insan diline çevrilmesi, diller arası ad çakışması, değer metninin dili ve çok dilli ekip ayrı bir dil tanımı bölümü olarak, öbür bölümler bittikten sonra tasarlanır.

## Araç işleri

Dilin kuralı değil, kodlama modellerinin dili rahat kullanması için araçlar (Gemini'nin 1. ve 2. bölüm araştırması, DENETİM doğruladı, 2026-10-27):
- Dilin makinece okunur dilbilgisi dosyası: model kod üretirken sözdizimine uymayan sözcüğü baştan dışarıda bırakabilsin.
- Kodlama modeline verilecek tek sayfalık başvuru: kimliklerin yazımı, sık hatalar ve doğruları.
- **Claude Code kancalarının gözden geçirilmesi** (Ahmet, 2026-10-28; [`tools/edithook`](../tools/edithook/README.md)). Kancalar ajanın yanlış yapmasını önlemek için var; her biri ajanı gerçekten doğru şeye yönlendiriyor mu, bedeli ne, diye tartılır. 2026-10-28'de bulunan ve o gün giderilenler: düzenleme kancası ilk düzenlemeyi durdurup yineletiyordu (her dosyada bir tur, karar değişmiyordu; artık düzenlemeden sonra bilgi verir); test kancası bütün komutu durdurduğunu söylemiyordu (aynı komuttaki bir düzeltme çalışmamış, ajan onu uygulanmış sanmıştı) ve dar alt ağaçları da durduruyordu. Açık kalan: düzenleme kancası kabuktan (Python, sed) yapılan düzenlemeyi görmez, büyük işlerde düzenlemelerin çoğu oradan yapılıyor; bilgi listesinin hangi testleri öne alacağı; kancaların kendi bedeli (her çağrıda derleme).

## Kilometre taşları

Kilometre taşı, dili gerçek bir işte sınayan ve bitiş ölçütü ölçülebilen bir hedeftir. Sıradakiler bir öncekinden sonra onayla eklenir.

### 1. kilometre taşı: RGN-512'yi TR'de eğitmek (büyük ölçüde yapıldı)

PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş bir dil modelini (`tools/gpu/rgn512.py`) TR'de yazıp (`tools/gpu/rgn512.tr`) aynı veriyle eğitmek. 2026-10-23'te bitiş ölçütü sağlandı: 0,8 milyar parçalık iki tohumlu koşuda PyTorch'tan daha düşük doğrulama kaybı, bellekte derlenmiş PyTorch'la başa baş, hızda derlenmiş PyTorch'un 1,07 katı; üç kart türünde (RTX PRO 6000, A100, L4) GPU testleri geçiyor. Ayrıntı [arşivde](archive/roadmap-2026-10-27.md). Ahmet, 2026-10-28: büyük ölçüde yapıldı; RGN-512 artık dilin ölçü modelidir, hedef değildir.

### 2. kilometre taşı: hazır modelleri CPU'da ve GPU'da hızlı çalıştırmak

Ahmet, 2026-10-27. Hazır modeli çalıştırmak amacın en yaygın işidir; modeli sunmak, robotun içinde çalıştırmak ve ince ayar ona dayanır.

**Bitiş ölçütü:**
- Bir dil modeli (Llama ya da Qwen3 ailesi), bir görüntü modeli ve bir ses modeli (Whisper gibi konuşma tanıma) hazır ağırlıklarıyla çalışır ve özgün uygulamayla aynı sonucu verir.
- Hız aynı model, aynı hassasiyet ve aynı makinede ölçülür: GPU'da PyTorch ve llama.cpp'ye, CPU'da PyTorch CPU ve llama.cpp'ye karşı. Hedef oran ilk ölçümden sonra buraya yazılır.
- CPU'da hem x86 hem ARM64 işlemcide çalışır.
- Ağırlık 16 bit ve nicemlenmiş (8 ve 4 bit) olarak yüklenir.

**Bugün** (ölçüm 2026-10-22, ayrıntı arşivde): GPU'da üretim transformers'ın en hızlı yolunu geçti, llama.cpp f32'nin %63–83'ü; CPU'da llama.cpp'nin yaklaşık 1/5'i. Yalnız dil modelleri çalışıyor; evrişim ve ses işlemleri, nicemleme ve ARM vektör kernelleri yok.
