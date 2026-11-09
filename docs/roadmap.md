# TR Yol Haritası

TR'nin amacı [README](../README.md)'dedir: yapay zekânın bütün işleri için bir programlama dili.

**Son ölçüm: 2026-11-08, main `cf43b3c0`.** O gün belge yön belgesine çevrildi; değişen satırlar depoya, status'a ve `measurements.md`'ye karşı ölçüldü (DENETİM 322), öbürleri 2026-11-01'in ölçümünden. Eski yol haritasının ([`docs/archive/roadmap-2026-10-27.md`](archive/roadmap-2026-10-27.md)) her maddesinin akıbeti de ölçüldü: bitti, taşındı, bırakıldı ya da buraya alındı. Arşivden gelip bugünkü durumu yeniden ölçülmemiş işler "(arşiv)" diye işaretlidir: iş kayıtlıdır, bugün hâlâ geçerli olduğu ölçülmemiştir.

## Yöntem

- Dil tanımı ([`docs/spec/`](spec/README.md)) dilin nasıl olması gerektiğini yazar; bugünkü uygulamaya göre değil, amaca göre.
- Her bölüm iki denetimden geçer:
  1. **kararlar:** Gemini araştırır, DENETİM doğrular;
  2. **metin:** DENETİM okur.
- Her kuralın bugünkü durumu ve yapılacakları [`status.md`](status.md)'dedir; iş testiyle kapanır ve oradan silinir.
- Bu belge işlerin sırasını, bölümü olmayan işleri ve kilometre taşlarını tutar.
- **Güncel tutma:** bir karar ya da iş değişince bu belge aynı gün, ölçülerek güncellenir. Sınıfın karar anında yazılması kuralı [status'un "Önem ve öncelik" girişindedir](status.md#önem-ve-öncelik). Eksik ya da yanlış bir satır yanlış yöne götürür; fark geç anlaşılınca boşa iş yapılmış olur.
- **Yön burada, ayrıntı status'ta** (Ahmet, 2026-11-08: "roadmap genel olur yön gösterir. status ise detay olur."): bu belge ne yapılacağını, nedenini, sırasını ve kimin karar verdiğini yazar. Ölçü, commit, birleşme tarihçesi ve bir işin bugünkü durumu burada yazılmaz: durum ve yapılacaklar [`status.md`](status.md)'de, ölçüler [`measurements.md`](measurements.md)'de; burada yalnız bağlantısı. Bitmiş bir iş tek sözcükle ve status bağlantısıyla anılabilir ("bitti, status 10.2"); ayrıntısı yazılmaz. Neden: aynı bilgi iki yerde durunca biri eskiyordu (2026-11-08: sızıntılı `relu` satırı kaynaştırmadan sonra da eski profili gösteriyordu).
- **Bekçi testler** (`internal/roadmap_test.go`):
  - eski yol haritasının numaralarına gönderme yok;
  - "Son ölçüm"ün commit'i HEAD'in atası;
  - bölümlerin denetim durumu burada kopyalanmaz, yalnız status'ta yazılır;
  - ölçü etiketleri (ölçüldü, hesap, kanıtlı, hipotez, tahmin, görüş, test) bu belgede kullanılmaz.

  Testler ölçümün yerine geçmez.

## Durum

- **Dil tanımı:** bölümler [`docs/spec/`](spec/README.md)'de; her bölümün denetim durumu `status.md`'nin bölüm başlarında.
- **Açık denetimler** (sıradaki dil tanımı turunda):
  - Uygulama sırasında verilen kararlar DENETİM'den geçti, ama Gemini'nin karar araştırmasından geçmedi. Liste yalnız `status.md`'de: "Uygulamada verilen, dil tanımı turunda denetlenecek kararlar" bölümü ve bölüm başları. Burada kopyası tutulmaz.
  - Dil tanımı turları: sıradaki turun girdisi `docs/spec/pending.md`'dir; biten turlar status'un bölüm başlarında.
  - Bölümlerin kapanış turu: bulguları işleyen commit'in farkını DENETİM'in yeniden okuması; hangi bölümde yapıldığı status'un bölüm başlarında.
- **Uygulama fazlar hâlinde ilerliyor.** Her fazın işleri dosya sahipliğine göre ayrılır, paralel kollar aynı dosyaya dokunmaz.
  - **Faz 0:** toplu ve sıralı işler (kancalar, ad göçleri, ortak sorgular); bitti, status'ta "Faz 0" diye geçen satırlar.
  - **Faz 1, sürüyor.** Kolların durumu ve kalanları status'ta, işin bölümünün başlığında; kolların günlük durumu bu belgede tutulmaz.
    - **4a, denetleyici:** çıkarım, tipsiz parametre ve alan, değeri kullanılmayan çağrı, metodun sayısı ve tipi, imzaların öğe tipi, koşulun tipi; tanım dosyasının biçimini isteyenler (kısıtlar, çoklu imza) ve karışık liste değişmezi beşinci dilimde;
    - **D1, çalışma anı:** hata değerleri, ertele, görev ve iptal, sıralama, JSON, genel yerleşiklerin sessiz yanlışları (13.1);
    - **S1, sistem:** modüller, testler, veri, kodlar, dosya, süreç;
    - **T1, tensör çekirdeği:** sessiz yanlışlar, cinsli tensör ve Kol B; kalan GPU'da cinsli genel çekirdekler (2b) ve denetleyicinin cins kuralları (4c);
    - **13'ün yerleşikleri:** değer döndürmeyen yerleşikler ve sayı fonksiyonlarında tek algoritma sıradaki iş;
    - **parçalayıcı;**
    - **iletiler:** önerinin ve kod örneğinin kimlikle yazılması.
  - **Faz 2:**
    - **4b, akış:** daraltma, kapsama, eksik dönüş;
    - **4c, genel tipler;**
    - **T2:** sinir ağı ve eğitim;
    - **T3:** hız, dosya ve nicemleme;
    - **K:** uzun yerleşiklerin kilidi; durumu ve kalanı status 10.2'de.
- **Komut adı:** `tr` yerine `trlang` (Ahmet, 2026-10-30): Unix'in `tr`'si aynı adla sessizce başarılı çıkıyor; `trl` Hugging Face TRL'nin komutu. Takma ad yok. Göç bitti, eski ad bir testle yasak (status "Komut adı").

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
- **Fonemleştirme:** metni sese çevirmeden önce seslerine (fonemlere) ayırmak. Metinden sese modelleri (Piper) ister.
- **Veriyi toplamak ve temizlemek:** yinelenen veriyi ayıklamak, süzmek, metni dosyadan ve ağdan toplamak. Bunun için gerekenler:
  - **hash:** sha256 ve xxhash gibi hızlı hash'ler; yineleneni ayıklamak, veri kümesini doğrulamak, önbellek anahtarı;
  - **düzenli ifade:** metin temizleme ve parçalayıcının ön ayırması;
  - **Unicode:** NFC ve NFKC normalleştirme, kod noktası ile karakter arasında dönüşüm.
- **Dosya biçimleri:**
  - zstd ve zip (`.jsonl.zst`; 18.12'nin `.gz` saydamlığı gibi);
  - Parquet;
  - CSV yazmak.
- **Uzak depolama:** S3 ve GCS'ten akışla okuma (arşiv).
- **Süreçlerle paralellik:** TR koduyla yazılmış veri işi yorumlayıcı kilidi yüzünden tek çekirdekte kalır; işi süreçlere bölmek.

**Eğitmek ve ölçmek:**
- **Dağıtık eğitim:** birçok GPU ve kümede eğitim, verinin süreçler arasında bölünmesi, dizi paralelliği.
- **Değerlendirme:** amacın "Ölçmek ve değerlendirmek" işinin bölümü yok. Ortak sınavlar (lm-evaluation-harness ile aynı sınav), değerlendirme kitaplığı (arşiv).
- **Deney takibi ve hiperparametre taraması** (arşiv).

**Dil ve çalışma ortamı:**
- **Standart hataya yazmak:** programın kendi kullanım hatasını ve hizmetin hata çıktısını standart hataya yazacak bir yol yok. 19.10 bunu ister; yeni ad ya da adlı argüman gerekir. `TR_command_line[Ayar]()` ile zorunlu seçenek de 4c'den sonra gelir.
- **Tarih ve saat** (arşiv).
- **Bellek ve gerçek zaman:** çöp toplayıcı kalır, sahiplik yazdırılmaz; duraklama ölçüsü `measurements.md`'nin eski listesinde. İki iş açık:
  - Tensör yerleşiklerinin kilidi bırakması, "5 ms'den uzun sürebilen her yerleşik çalışırken kilidi bırakır" sözüne göre (durumu status 10.2'de); tensör çekirdeğinin işi, yerel Jarvis'ten önce.
  - Küçük işaretçili yığınla dönen döngüde çöp toplama turunu tik arasına almak ölçülecek.
- **Hız** (hız tasarımı, 2026-11-02; hedefleri Ahmet onayladı):
  - **Durum:** ölçüleri `measurements.md`'nin eski listesinde (yeniden ölçülecek); ağır işin tabanı [`tools/bench/README.md`](../tools/bench/README.md)'de.
  - **Yol:** TR, denetleyicinin tipleriyle Go kaynağına derlenir. Yorumlayıcı kalır: kabuk, hata ayıklayıcı ve derleyici bütün dili kapsayana kadar varsayılan koşu yolu. Yorumlanan ve derlenen program aynı sonucu verir: `tests/`, `lib/` ve `examples/` iki yolda koşar, ARM64'te de.
  - **Sıra:** yorumlayıcıda ucuz yerel kazançlar (ilk dilim S1, 2026-11-02, main'de); denetleyicinin tip haritası ve tip uyuşması kipi (her ifadenin değeri denetleyicinin tipinde mi); derleyicinin ilk dilimi; bütün dil. Derleyiciden önce Go araç zincirinin kullanıcıya nasıl gideceği Ahmet'e sorulur.
  - **Hedefler:** saf hesapta (`dongu`, `fib`, `matris`) taşma denetimli ve aynı veri düzenli (matriste iç içe liste) Go'nun ≤ 1,5 katı; ağır işte PyTorch'un ≤ 1,0 katı; tensör işi yapan bir görev varken de 100 Hz'lik döngüde kaçan tur 0. Yorumlayıcının hedefi yerel kazançlar ölçülünce konur; eski "TR/CPython ≤ 1,0" hedefi kalktı, o hız derleyiciden gelir.
- **Paket, bağımlılık ve sürüm sabitleme:** başkasının modülünü sürümüyle almak, kilit dosyası. `tr/` kitaplığı ikiliyle gelir, ona gerekmez (11.6).
  - Başkasının modülü gelince izin modeli de yazılır: dosya, ağ ve dış program (21) için tek tasarım. 2026-11-02'de dilde izin modeli yok; yalnız dış programa izin koymak yarım bir model olurdu (TCB'nin kararı, DENETİM'in kanıtıyla).
- **Sürüm politikası:** 0.y.z numarası; kararlı, beta ve prototip etiketleri; dil ve dosya biçimini dondurma kuralları; yayın ikilileri (arşiv).
- **Karo dili:** GPU çekirdeklerini TR'de yazmak (arşiv).
- **Güvenlik denetimi:** saldırı yüzeyinin baştan sona denetimi (dosya biçimleri, ağ, komut satırı) (arşiv).

## 15. bölümün sonraya bıraktıkları

Her biri gelince 15'in yöntemiyle yazılır.

- **"Paylaşmak"ın öbür biçimleri:**
  - ONNX ve GGUF'a dışa verme (15.47a); GGUF okuma 2. kilometre taşına geçti;
  - ONNX okuma (Piper ve Silero ağırlıkları);
  - PEFT bağdaştırıcı dosyasını okuma ve yazma (LoRA'nın hazır araçlarla paylaşılması).
- **Başka hızlandırıcılar:** AMD (ROCm), Apple, gömülü NPU; cihaz adları 15.37'nin kuralıyla.
- **Türev:**
  - etkinleştirme kaydı (`TR_recompute`);
  - ikinci türev ve `retain_graph` (15'in gerekçesi buraya gönderiyor);
  - ileri yönlü türev.
- **Tensör işlemleri:**
  - çok eksenli indeks;
  - `TR_stack`, `TR_pad`, `.TR_repeat` (15'in gerekçesi `TR_pad` ve çok eksenli indeks için buraya gönderiyor);
  - doğrusal cebir. FFT 23. bölümde (2026-11-02).
- **Sinir ağı işlemleri:**
  - havuzlama ve büyütme, ters evrişim (15.23c);
  - CTC (ses modelleri). STFT ve Mel 23. bölümde (2026-11-02);
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
  - **Gerçek kullanım provası (2026-11-06):** zor-3 yerine yabancı ajan tek insan cümlesiyle `AGENTS.md`'den başlıyor; hazır ikili `v0.2.0` yayımlandı. Ajanlar ve kayıt: [`agent_trials.md`](agent_trials.md). Türkçe kök bulucu `tr/turkish` olarak kitaplığa girdi (2026-11-06). Sıradaki istem: okuma hazırlığı. Zor-3'ün konuları (demet, `T?`, durumlu tip, `TR_match`, JSON, hata anları) ve sessiz yanlış eşiği provada henüz ölçülmedi: sonraki istemlerde aranır.
- **Önekin bedeli:** kimlikle yazılan kod, insan dilindeki görünümden farklı sayıda parça tutuyor; ölçüsü `measurements.md`'nin motordan bağımsız listesinde. Dil kararını değiştirmez; ajanın bağlam bedeli olarak izlenir.

## Araç işleri

Dilin kuralı değil; kodlama modellerinin ve insanların dili rahat kullanması için.

- **Makinece okunur dilbilgisi dosyası:** model kod üretirken sözdizimine uymayan sözcüğü baştan dışarıda bırakabilsin.
- **Kodlama modeline tek sayfalık başvuru:** var ([`agent_reference.md`](agent_reference.md), giriş [`AGENTS.md`](../AGENTS.md)); `sekerahmet/tcb-research`'teki Evre A ve B başvuruları kayıttır.
- **Claude Code kancaları** ([`tools/edithook`](../tools/edithook/README.md)): her kanca ajanı gerçekten doğru şeye yönlendiriyor mu, bedeli ne, diye tartılır. Açık kalanlar:
  - düzenleme kancası kabuktan (Python, sed) yapılan düzenlemeyi görmez;
  - bilgi listesinin hangi testleri öne alacağı;
  - kancaların kendi bedeli (her çağrıda derleme).
- **Geliştirici araçları** (arşiv):
  - **dil sunucusu:** eklentide bugün yalnız tamamlama var; tanıma gitme, kullanımlar, yeniden adlandırma ve öbürleri;
  - **hata ayıklayıcı:**
    - DAP bitiş ölçütü: hata ayıklayıcının uçtan uca sürüşü;
    - kesme noktası bir sonraki deyimde duruyor: komut satırının hata ayıklayıcısında doğru duruyor (DENETİM istek 76, düz programda ve fonksiyon gövdesinde); DAP'ta (eklenti) denenmedi;
    - F5'te `TR_input()` çalışmıyor;
    - isabet sayılı kesme noktası ve iz noktası, hata fırlatılınca durma;
    - tensör özeti, "kayıp NaN olunca dur";
  - **İzleme Masası:** gradyan ve tensör özeti;
  - not defteri;
  - Model Sihirbazı; süre, bellek ve maliyet tahmini;
  - `TR_print(model)`'in özet görünümü;
  - dizinde ve eklentide modüle özel adların gizlenmesi.
- **Test denetimi** (plan 2026-11-04, DENETİM istek 90): testlerin tekrarı, hızı ve eksikleri altı adımda ölçülür ve düzeltilir; durum status "Denetim katmanlarının gözden geçirmesi"nde.
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
  - adım süresi derlenmiş PyTorch'a yakın; sayılar [`tools/bench/README.md`](../tools/bench/README.md)'nin "Güncel taban"ında ve arşivde;
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
- Hız aynı model, aynı hassasiyet ve aynı makinede ölçülür: GPU'da PyTorch ve llama.cpp'ye, CPU'da PyTorch CPU ve llama.cpp'ye karşı. Kartta llama.cpp'ye karşı ilk prototipin hedefi üretimde ve prompt'ta 0,5 (Ahmet, 2026-11-05); öbür hedefler ilk ölçümden sonra yazılır.
- CPU'da hem x86 hem ARM64 işlemcide çalışır.
- Ağırlık 16 bit ve nicemlenmiş (8 ve 4 bit) olarak yüklenir.
- Nicemli ağırlık hazır dosyadan gelir: yayımlanmış GGUF okunur; Jarvis için ön koşul budur, kendimiz nicemlemek sonraki fazlarda (Ahmet, 2026-11-04). Türler (araştırma ve DENETİM istek 118): `Q8_0`, `Q4_K`, `Q6_K`; `Q4_0` sonra, `Q4_1` eklenmez. GGUF okuyucu, üstveri ve parçalayıcı aynı adımda (15.55, 16.1).

**Durum:** ölçüler [`measurements.md`](measurements.md)'de; dil modelinin, GGUF'un ve nicem çarpımının kalanları status 15'te.

**Ölçüte varmak için eksikler** (dil tanımında olanlar status'ta; olmayanlar burada):
- **Yükleme:**
  - safetensors'u bellek eşlemeyle (mmap) kopyasız yüklemek; bugün dosya belleğe kopyalanıyor;
  - birden çok dosyaya bölünmüş modeli okumak (`model.safetensors.index.json`); 8 milyar ve üstü modeller böyle gelir;
  - Mistral mimarisi;
  - açılış süresi.
- **Üretim:**
  - KV önbelleği için halka tampon, 8 bitlik KV önbelleği;
  - hazır istemin önbelleğini saklamak (önek önbelleği): DOST'un her turu önbelleği korumalı; ölçüsü `measurements.md`'nin dizüstü satırında;
  - tahminli çözme.
- **CPU:**
  - birden çok satırlık çarpımda (`M > 1`) 16 bitlik ağırlığı açmadan kullanmak: ön doldurma ve eğitimde bütün matris hâlâ geçici float32 tampona açılıyor (tek satırlık çarpım 2026-11-04'te kapandı);
  - `relu`, `sigmoid`, `silu`'nun vektörlenmesi;
  - ses üretiminde evrişimin hızı: sızıntılı `relu` `TR_fuse` ile kaynaştırıldıktan sonra kalan pay orada (status "Yazıyı konuşmaya çevirme");
  - üretimde işçi sayısının ölçümü;
  - dizüstünde istem okumanın iş parçacığıyla ölçeklenmemesi (`measurements.md`, dizüstü satırı); sıradaki ölçü sınırı ayırır: P ve E çekirdeklerine ayrı bağlı koşu, saat ve güç sayacı, bellek bandı, Windows'un güç modu ("Dengeli" ve "En İyi Performans", fişte, sessiz pencerede, sıra dönüşümlü; Ahmet'in önerisi, 2026-11-08);
  - çok satırlı nicem çarpımının sabit bedeli (`measurements.md`, dizüstü satırı): yalnız 21 parçalık ileri geçişin profili bedelin içini gösterir. Küçük M için tek satırlık yol bitleri değiştirir, ayrı karar;
  - dizüstü hız işlerinin sırası (H1'in önerisi, TCB kararı, 2026-11-08), **asistanın mimari kararından sonra** (Ahmet, 2026-11-08 22:07): önce istem okumanın sınırı ve sabit bedelin profili; sonra Whisper'ın kodlayıcı ve çözücü adımı (DOST zincirinde sözün yazıya çevrilmesi dil modelinin ilk parçasıyla aynı mertebede); sonra yüklemedeki float32 kopya (bellek); kartta istem okuma kartsız dizüstüne katkı vermediği için sona; etkinleştirme nicemleme denemesi en sonda;
  - katman başına çekirdeklerin kaynaştırılması.
- **Kartlar:** T4 ve H100'de sınama; SM120'nin tensör çekirdeği komutları.
- **Model seçimi:** bu dizüstü için en uygun açık modeller Gemini'nin araştırmasında (2026-11-01; DENETİM okudu, düzeltmeleriyle). Uzman karışımlı Qwen3-30B-A3B ve yoğun Qwen3-8B öne çıkıyor; uzman karışımı 15. bölümün sonraya bıraktıklarında.

### Kilometre taşı adayları (Ahmet'in kararı bekleniyor)

- **Yerel sesli asistan, "Jarvis"** (Ahmet, 2026-10-31: "robottan önce bir Jarvis", kabaca eklendi):
  - TR ile yazılmış, yerelde çalışan bir sesli asistan. Robotun "beyin ve ses" kısmı; yayımlanmış modeller kullanılır, tutkal, çalıştırma ve ajan döngüsü TR'de yazılır.
  - Öncelik (Ahmet, 2026-11-07): önce Jarvis çalışır hale gelir, yeni fikirler de kullanılır. Sıra (TCB): önce uçtan uca zincir (dinler, anlar, cevap verir, konuşur); Jarvis'in yolunu kısaltmayan fikir zincir çalıştıktan sonraya kalır.
  - Fikir, katmanlı uzman yerleşimi (Ahmet, 2026-11-07; github.com/JustVugg/colibri, C, Apache-2.0): uzman karışımlı modelde sık kullanılan uzmanlar hızlı bellekte kalır, kalanı gerektikçe diskten okunur. Yerleşim yalnız hızı değiştirmeli, sonucu değiştirmemeli (colibri'nin tasarım amacı); bu, aynı programın aynı sonucu vermesi ilkesine uyar. Uzman GPU'ya geçerse bire bir aynılık ayrıca sınanır. TR'de önce uzman karışımı gerekir (15. bölümün sonraya bıraktıkları). Sıra (TCB), zincir çalıştıktan sonra: uzman karışımı ve RAM'e sığan 30B-A3B sınıfı model; aynı makinede colibri'yle hız kıyası; disk akışı ancak RAM yetmezse ya da daha büyük model istenirse. colibri'nin hız sayıları bizde ölçülmedi.
  - Adımlar (Gemini araştırması ve DENETİM, 2026-10-31):
    1. yerel metin sohbeti;
    2. dosyadan ses ve Whisper: bitti, status 25;
    3. ses üretimi ve canlı ses (Ahmet, 2026-11-05: Jarvis ilk aşamada kişisel kullanım, ticari olmayan lisanslı Türkçe ses kabul; ilk doğrulama modeli MMS-TTS, ağırlık depoya girmez): MMS-TTS TR'de çalışıyor (status "Yazıyı konuşmaya çevirme"); gerekenlerin listesini 27 çıkarır;
       Ses klonlama (Ahmet, 2026-11-06 ve 2026-11-08): aday Chatterbox Multilingual (MIT, Türkçe, kısa örnekle klon, çıktıda PerTh filigranı). Klon yalnız kendi ya da izinli sesle; Ahmet'in sesi depoya girmez.
       Kalite: Ahmet MMS-TTS'yi dinledi (2026-11-08): "benzer, kötü değil ama süper de değil"; kalite modelin sınırı, TR'nin değil. Chatterbox'ı dinledi: "aşırı kaliteli ve iyi". Kaliteli ses için aday Chatterbox; Piper'ın Türkçe sesleri yedek aday, denenmedi.
       Karar (TCB): DOST'un canlı sesi MMS'le kalır. Chatterbox dizüstü CPU'sunda gerçek zamandan çok yavaş (ölçüler `measurements.md`'de); TR'ye taşımak ancak iki yarısını (konuşma parçası üreten dil modeli ve dalga çözücüsü) birlikte en az gerçek zamana ulaştıracak bir yol ölçülürse açılır. **Aday (TCB, Ahmet onayladı 2026-11-08):** çözücüyü (S3Gen) hızlandırmak, önce çözücünün içinin profili; ayrıntı status'un "Yazıyı konuşmaya çevirme" başlığında. Canlı olmayan kullanım (önceden seslendirme) ayrı tartılır. Canlı ses için ölçüt en az gerçek zaman, gecikme eşiği gelince yeniden tartılır.
       Sıra (2026-11-05, DENETİM 196): birbirine bağlı olmayan üç zincir: dış süreç (`TR_run_process`, `TR_spawn_process`), konuşma algılama (Silero VAD, `tr/vad`), ses üretimi (bölüm 27, MMS-TTS); üçü de yapıldı (status 21, "Bölümü olmayan işler", "Yazıyı konuşmaya çevirme"). Hız zincirlerin önüne geçmez: üç zincir olmadan ürün yok, hız onların yerini tutmaz. Söz kesme ilk sürümün önkoşulu değil. İlk sürüm Windows'ta, NVIDIA kartsız bir dizüstünde (Ahmet, 2026-11-05): hız CPU'da aranır.
       Uçtan uca zincir (S1, birleşik 58): dinler, yazıya çevirir, Qwen3-4B cevaplar, MMS konuşur; `tr/chat` (24.1–24.3) Qwen3'ün biçimini yazıp okuyor. Zincir çalışıyor ama asistan olarak kullanılır değil; durumu ve ölçüleri status'un "Bölümü olmayan işler"inde (asistan zinciri). Whisper'ın ve dil modelinin kalitesi hata değil, model seçimidir. **Mimari önce, hız sonra (Ahmet, 2026-11-08 22:07):** "bence mimari karar sonrası hıza bakmak daha mantıklı. mimari değişirse H1 tekrar bakması gerekmez mi ?". **Mimari kararı (TCB, tasarım turu 2026-11-09; Gemini'nin araştırması, DENETİM 323 ve 324):** zincir sıralı kalmaz: mikrofon tur boyunca açık kalır, cevap cümle cümle konuşulur, sistem iletisi program açılırken okunur. Whisper'ın penceresi ve sessizlik eşiği değişmez; cevap konuşma bitmeden üretilmeye başlanmaz (kazancı küçük, bedeli büyük). Ayrıntı status'un "Bölümü olmayan işler"inde (asistan zinciri). Ölçü sade (Ahmet, 2026-11-08 23:56): tanımlar bir kez, başta taban ve sonda son ölçü, ara ölçü yalnız bir değer seçilecek yerde; S1 kendi denemelerini kendi ölçüleriyle yapar. Mimari dilimi ve tekdüze saat bitti (status, asistan zinciri ve 13.1). Sıra (TCB, DENETİM 335'in önem sırasıyla, 2026-11-09): sabit bedel ve bellek dökümü bitti (status, asistan zinciri). Sonra: (1) okumanın kalan sabit bedeli (panelin açılması, H1); (2) yalnız önbelleği öbek dışına almak (TCB'nin tasarımı, sonra H1; bitler aynı); (3) üretimin profili ve dizüstünde llama.cpp ile üretim kıyası; (4) Gemma 3: mimari ve kayan pencere (E: bugün açık hata); (5) konuşmalı üretim, Qwen3-0.6B taslağıyla (bitler için karar). Whisper'ın profili ve evrişimin `unfold1d`'si (zincirin ayırmasının çoğu) (3) ile birlikte. Bit değiştiren iki seçenek (16 bit önbellek, 8 bit etkinleştirme) Ahmet'in kararı, önce deneme kipiyle ölçü. Çalıcının gecikmesi S1'in sonraki diliminde. Akışlı çözme (16.3a) S1'in cümle cümle konuşmasından sonra: o dilim baştan çözmeyle çalışır, 16.3a gelince ona geçer. Oturumlar Ahmet'in sözüyle başlar. TCB'nin notu: istem okumanın sabit bedeli ve modelin Türkçe cevap kalitesi mimariden bağımsız. Sonra S1'in kalite dilimi (S1'in önerisi, TCB kabul, 2026-11-08): (1) dil modeli yalnız yazıyla, aynı altı cümle birkaç adaya, Whisper karışmaz, Ahmet gerekmez; (2) duyma: daha büyük Whisper (aday `large-v3-turbo`), önce sentetik sesle; (3) Ahmet'le aynı altı cümleyle canlı test (Ahmet 2026-11-08 19:36: "sonuçlar iyi değil"; "düzeltmeler ve kazançlar olunca tekrar bakarız"). Rakam yazıya çevrilmeden MMS onu okuyamıyor (dil tanımı 27).
    4. söz kesme (görev iptali, kilidin hızlı devri);
    5. belgeler ve hafıza; ilk ipucu: beş ajanın arama programı yarışında kök bulan program öne geçti ([`agent_trials.md`](agent_trials.md));
    6. yerli ses giriş ve çıkışı ile CPU nicemleme; dış süreçte kaynağın kendi kaybını TR görmez (21.2), yerli giriş bunu kapatır.
  - Asistan araçları: modelin çağırdığı dosya ve süreç araçları. Modelin verdiği dosya adı dış girdidir: `TR_Path`'in denetiminden geçmeyen yollar (status 18.1) bu dilimde kapanır, dilim onlarsız bitmez (DENETİM 230).
  - Ekran (Ahmet, 2026-11-05): Jarvis TR'de; ekran ince kalır, tek dosya HTML ve düz JavaScript, derleme yok, TR'ye HTTP ile bağlanır. Önce ekransız sürüm (komut satırı, sonra Whisper); HTTP sunucusu dile girince ekran gerçek akışa bağlanır. Ahmet ekranı öne aldı (2026-11-06): `tools/assistant/` sahte akışla ve `contract.md` sözleşmesiyle hazır, TR'nin sunucusu gelince entegrasyon noktası orası. Pencere çizmek TR'nin işi değildir.
  - **Taban ölçümü (2026-11-01, Colab CPU, hazır parçalarla: llama.cpp, whisper.cpp, Piper):** sayılar `measurements.md`'nin motordan bağımsız listesinde. Sıradaki: dizüstünde aynı ölçüm, sonra bitiş ölçütünün önerisi.
  - **Yol (2026-11-02):**
    - Küçük bir modelin profili (DENETİM, 2026-11-02) yükün çoğunu tensör yerleşiklerinde, azını yorumlayıcıda gösterdi; sayılar `measurements.md`'nin eski listesinde, gerçek modelin profili yok. Derleyici bu yüzden Jarvis'in yolunda değil; asıl kazanç CPU'da nicemli üretim, çekirdeklerin kaynaştırılması ve ses için sinir ağı işlemleri.
    - Dil tanımında henüz yazılmamış işler Jarvis'in en uzun yolu:
      - bölümü olmayanlar: canlı ses (ilk sürüm dış programla), gömme; ses üretimi için fonemleştirme. Dış programlar ve bayttan sayı 21 ve 22'de, sohbet biçimi 24'te;
      - 15'in sonraya bıraktıkları: ses üretimi için ters evrişim, ONNX okuma ve ters Fourier dönüşümleri (`irfft`, `istft`; emsali 23'ün gerekçesinde). STFT ve Mel artık 23'te;
    - Jarvis'in istediği bölümler (21–25) yazıldı; denetim durumları status'un bölüm başlarında. Yeni bölüm yazımı, sıradaki dil işi uygulama olana dek durur (Ahmet, 2026-11-02: "mevcutlar bitsin"); önce status'un önem taramasındaki öncelikli işler (status "Önem ve öncelik").
    - Bunların araştırması ve dil tanımı yazımı Faz 2'nin koduyla aynı anda başlar; ses üretiminin üç işi de bu karara girer (Ahmet, 2026-11-02). Uygulaması fazların sırasıyla gelir.
  - Bitiş ölçütü ve sıra Ahmet'in kararı. Ölçüt adayları: zincir gecikmesi, dizgecik/s, söz kesme süresi, Türkçe kelime hata oranı, tepe bellek; sayılar dizüstündeki tabandan sonra.
- **Uyarlamak:** hazır bir modeli ince ayarla kendi işine uyarlamak, uçtan uca. LoRA'yı dil yazabiliyor; ama PEFT düzeninde kayıt, ince ayar örneği ve bitiş ölçütü yok (arşiv).
- **Herkes için:** biraz teknik bilgisi olan birinin kendi modelini kurup eğitmesi. Model Sihirbazı, süre ve maliyet tahmini, küçük model tarifleri (arşiv).
- **Masaüstünde çalışan bir TR modeli** (Ahmet, 2026-11-01: "önce TR'yi bitirelim, tüm fazları; ondan sonra"): açık ağırlıklı, Türkçe ve İngilizce bilen bir model, bu dizüstünde en hızlı çalışacak biçimde; doğrulanmış TR programlarıyla ince ayar. Fazlardan sonra.
