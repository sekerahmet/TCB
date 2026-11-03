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
- **Güncel tutma:** bir karar ya da iş değişince bu belge aynı gün, ölçülerek güncellenir. Sınıfın karar anında yazılması kuralı [status'un "Önem ve öncelik" girişindedir](status.md#önem-ve-öncelik). Eksik ya da yanlış bir satır yanlış yöne götürür; fark geç anlaşılınca boşa iş yapılmış olur.
- **Bekçi testler** (`internal/roadmap_test.go`):
  - eski yol haritasının numaralarına gönderme yok;
  - "Son ölçüm"ün commit'i HEAD'in atası;
  - bölümlerin denetim durumu burada kopyalanmaz, yalnız status'ta yazılır.

  Testler ölçümün yerine geçmez.

## Durum

- **Dil tanımı:** 20 bölümün hepsi yazıldı (1–13, 15–20; 14 yok). Her bölüm DENETİM'den ve Gemini'nin karar araştırmasından en az bir kez geçti. Bölüm başına kaydı `status.md`'nin bölüm başlarında.
- **Açık denetimler** (sıradaki dil tanımı turunda):
  - Uygulama sırasında verilen kararlar DENETİM'den geçti, ama Gemini'nin karar araştırmasından geçmedi. Liste yalnız `status.md`'de: "Uygulamada verilen, dil tanımı turunda denetlenecek kararlar" bölümü ve bölüm başları. Burada kopyası tutulmaz.
  - **Tur 1 bitti (2026-11-01):** 2026-10-28'in 24 geçici kararı ve dört bulgu Gemini'nin araştırmasından ve DENETİM'in kaynak denetiminden geçti; "değiştir" denenlerin dördü yazıldı (3.6a, 3.7a, 3.14, 18.2), 13.8 tur 2'ye kaldı. **Tur 2 bitti (2026-11-01):** o günün beş kritik kararı ve yedi olağan bulgusu Gemini'nin araştırmasından ve DENETİM'in kaynak denetiminden geçti; 20.5/20.17'nin üreteç kaydı iki duruma (paylaşılan ve cihaz) genişledi, gerisi kabul edilip yazıldı. **Tur 3 bitti (2026-11-02):** iki kritik geçici karar (3.7a; 3.14 ve 8.3) kabul edildi; altı olağan bulgudan beşi kapandı (15.46, 15.47a ve 15.55 yazıldı; 3.7a'nın önerisinin sırası ve 15.2 olduğu gibi kaldı); 3.11 koşullu olarak `pending.md`'de kaldı. Sıradaki turun girdisi `docs/spec/pending.md`'dir.
  - 12, 16, 17 ve 20. bölümlerin kapanış turu: bulguları işleyen commit'in farkını DENETİM'in yeniden okuması. 10, 11, 18 ve 19'da yapıldı.
- **Uygulama fazlar hâlinde ilerliyor.** Her fazın işleri dosya sahipliğine göre ayrılır, paralel kollar aynı dosyaya dokunmaz.
  - **Faz 0, bitti:** toplu ve sıralı işler (kancalar, ad göçleri, ortak sorgular).
  - **Faz 1, sürüyor** (kolların günlük durumu bu belgede tutulmaz; ana oturum depo dışında takip eder):
    - **4a, denetleyici:** çıkarım, tipsiz parametre ve alan, değeri kullanılmayan çağrı, metodun sayısı ve tipi, imzaların öğe tipi, koşulun tipi. Üçüncü dilim birleşti (2026-11-02): açılmamış isteğe bağlı değer (3.5), demetin tipi (3.6a, 5.3) ve `TR_JSON` (3.7a) denetim hatası; `TR_JSON` tipli değişmezin denetimi D1'in çalışma anı diliminden sonra gelir. Dördüncü dilim birleşti (2026-11-03): çalışırken sessizce yanlış sonuç veren yazımlar denetim hatası oldu (indeksle ve anahtarla atama, sözlük anahtarının tipi, ayrı tiplerin `==`'si, satırları paylaşılan liste, yerleşik fonksiyonun argümanı); tanım dosyasının biçimini isteyenler (kısıtlar, çoklu imza) ve karışık liste değişmezi beşinci dilime;
    - **D1, çalışma anı:** hata değerleri, ertele, görev ve iptal, sıralama, JSON. Üç dilim birleşti (ikincisi ve üçüncüsü 2026-11-02: hata değeri `TR_Error`, 10.2'nin süreyle devreden kilidi; görev değeri, görev listesini beklemek, iptal ve süre sınırı); sıralama ve JSON kaldı. Genel yerleşiklerin sessiz yanlışları (13.1) birleşti (2026-11-03): örneklem sapması, telafili ondalık toplam, `TR_read_bytes`, `TR_write_file`'ın metin ve baytı, geçersiz UTF-8 hatası; `TR_max`/`TR_round`'un kalanı da birleşti (2026-11-03, 13.1c: karışık sayıda `TR_Float`, boş listede hata, `TR_round` girdinin tipi ve yarımı çifte); eşitlikte özdeşlik kısa devresi ve dosya hatasında sistemin metni de birleşti (2026-11-03: aynı kap da öğeleriyle karşılaştırılır, kendine varan değer H2771; dosya hatası yalnız sistemin numarasını taşır);
    - **S1, sistem:** modüller, testler, veri, kodlar, dosya, süreç;
    - **T1, tensör çekirdeği:** ilk dilim yeni cins istemeyen sessiz yanlışlar (safetensors yazımı, paylaşılan üretecin kaydı, −0.0 sırası, türevin yazma kuralları, yerinde bileşik atama); ikinci dilim cinsli tensör (`TR_Int`, `TR_Bool`, float64). İki dilim de birleşti (2026-11-02; ikincisi `d24b95ea`, gerçek kartta iki hata yakalanıp düzeldi: PTX adı ve kimlik okumasının +%6'sı). Kalan: GPU'da cinsli genel çekirdekler (2b) ve denetleyicinin cins kuralları (4c);
    - **iletiler:** önerinin ve kod örneğinin kimlikle yazılması.
  - **Faz 2:**
    - **4b, akış:** daraltma, kapsama, eksik dönüş;
    - **4c, genel tipler;**
    - **T2:** sinir ağı ve eğitim;
    - **T3:** hız, dosya ve nicemleme;
    - **K:** uzun yerleşiklerin kilidi. Birinci dilim birleşti (2026-11-03, T1): tensörün deposuna okuma ve yazma izni; CPU'da uzun `@`, büyük öğe öğe işlemler, indirgemeler, iyileştirici adımı ve örnekleme kilidi bırakıyor. Bitiş ölçütü CPU'da Linux'ta geçti: uzun tensör işi sürerken başka görev en çok ~4 ms bekliyor, 100 Hz döngü tur kaçırmıyor. GPU yolu ve geri yayılım kilidi tutuyor: sıradaki dilim GPU, ana daldan.
- **Komut adı:** `tr` yerine `trlang` (Ahmet, 2026-10-30). Unix'in `tr`'si aynı adla sessizce başarılı çıkıyor; `trl` Hugging Face TRL'nin komutu. Takma ad yok. Göç ajan deneyinin zor-3'ünden önce yapılır, eski adı yasaklayan bir testle; sınıfı A (ayrıca C, F1), Ö0, kolu Gemini (DENETİM istek 76; status "Bölümü olmayan işler"). İletiler 3'e bağlı değil.

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
- **Bellek ve gerçek zaman:** çöp toplayıcı kalır, sahiplik yazdırılmaz. Duraklama en çok 0,23 ms; büyük veri tensördeyken 100 Hz'lik döngüde kaçan tur 0 [ölçüldü: hizdeney2, Colab CPU, `69c943eb`, kayıt `20261102094913.log`]. İki iş açık:
  - Tensör yerleşiklerinin kilidi bırakması, "5 ms'den uzun sürebilen her yerleşik çalışırken kilidi bırakır" sözüne göre (durumu status 10.2'de); tensör çekirdeğinin işi, yerel Jarvis'ten önce.
  - Küçük işaretçili yığınla dönen döngüde çöp toplama turunu tik arasına almak ölçülecek.
- **Hız** (hız tasarımı, 2026-11-02; hedefleri Ahmet onayladı):
  - **Durum:** saf hesapta TR, Rust'ın 100–270 katı. Beş testin (saf hesap ve kitaplık) geometrik ortalamasında TR Rust'ın ≈ 44 katı, elle yazılmış kutusuz Go ≈ 1,17 katı [ölçüldü: hizdeney2, kayıt `20261102094913.log`]. Ağır işte TR PyTorch'un 0,82–1,07 katı [ölçüldü: [`tools/bench/README.md`](../tools/bench/README.md)].
  - **Yol:** TR, denetleyicinin tipleriyle Go kaynağına derlenir. Yorumlayıcı kalır: kabuk, hata ayıklayıcı ve derleyici bütün dili kapsayana kadar varsayılan koşu yolu. Yorumlanan ve derlenen program aynı sonucu verir: `tests/`, `lib/` ve `examples/` iki yolda koşar, ARM64'te de.
  - **Sıra:** yorumlayıcıda ucuz yerel kazançlar (ilk dilim S1, 2026-11-02, main'de: dokuz kısa yol, `tools/bench` programlarında geomean −%14,75, yedi programın altısında p=0,000, `sirala` değişmedi [ölçüldü: Colab CPU, `BenchmarkPrograms`, `-count 10`, benchstat, `08d8e019` → `ce6cc28c`]); denetleyicinin tip haritası ve tip uyuşması kipi (her ifadenin değeri denetleyicinin tipinde mi); derleyicinin ilk dilimi; bütün dil. Derleyiciden önce Go araç zincirinin kullanıcıya nasıl gideceği Ahmet'e sorulur.
  - **Hedefler:** saf hesapta (`dongu`, `fib`, `matris`) taşma denetimli ve aynı veri düzenli (matriste iç içe liste) Go'nun ≤ 1,5 katı; ağır işte PyTorch'un ≤ 1,0 katı; tensör işi yapan bir görev varken de 100 Hz'lik döngüde kaçan tur 0. Yorumlayıcının hedefi yerel kazançlar ölçülünce konur; eski "TR/CPython ≤ 1,0" hedefi kalktı, o hız derleyiciden gelir.
- **Paket, bağımlılık ve sürüm sabitleme:** başkasının modülünü sürümüyle almak, kilit dosyası. `tr/` kitaplığı ikiliyle gelir, ona gerekmez (11.6).
  - Başkasının modülü gelince izin modeli de yazılır: dosya, ağ ve dış program (21) için tek tasarım. 2026-11-02'de dilde izin modeli yok; yalnız dış programa izin koymak yarım bir model olurdu (TCB'nin kararı, DENETİM'in kanıtıyla).
- **Sürüm politikası:** 0.y.z numarası; kararlı, beta ve prototip etiketleri; dil ve dosya biçimini dondurma kuralları; yayın ikilileri (arşiv).
- **Karo dili:** GPU çekirdeklerini TR'de yazmak (arşiv).
- **Güvenlik denetimi:** saldırı yüzeyinin baştan sona denetimi (dosya biçimleri, ağ, komut satırı) (arşiv).

## 15. bölümün sonraya bıraktıkları

Her biri gelince 15'in yöntemiyle yazılır.

- **"Paylaşmak"ın öbür biçimleri:**
  - ONNX ve GGUF'a dışa verme; GGUF okuma ve K-nicemleme ailesi (15.47a, 15.55);
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
    - kesme noktası bir sonraki deyimde duruyor: komut satırının hata ayıklayıcısında doğru duruyor [ölçüldü: DENETİM istek 76, düz programda ve fonksiyon gövdesinde]; DAP'ta (eklenti) ölçülmedi;
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
  - adım süresi derlenmiş PyTorch'un 1,07 katı, yani TR %7 yavaş (RTX PRO 6000; A100'de 1,15) [kanıtlı: `docs/archive/roadmap-2026-10-27.md:17,133,232`]; 2026-11-01'de Faz 0'dan sonra aynı oran, 145 / 136 ms/adım, TR'nin belleği 10,57 / 10,83 GiB [ölçüldü: Colab G4, `e9c64a45`, [`tools/bench/README.md`](../tools/bench/README.md) "Güncel taban"];
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

**Son ölçüm** (2026-11-03, main `51b75f0a`, Colab G4 ve CPU High-RAM; ayrıntı ve kaynak [`measurements.md`](measurements.md)):
- **GPU'da üretim:** transformers'ın en hızlı yolunun 1,18 katı; llama.cpp f32'nin 0,58–0,75, f16'nın 0,5 katı. 2026-10-22'ye göre %7–10 yavaşladı, llama.cpp aynı kaldı: gerilemenin yeri ölçülecek.
- **CPU'da üretim:** 48 iş parçacığında transformers'ın 2,2 katı, llama.cpp f32'nin 0,39–0,49 katı (Qwen3-0.6B: TR 25,1, llama.cpp 50,7 parça/sn; 2026-10-22'de 12,1). 8 iş parçacıklı Xeon'da transformers'tan yavaş (0,68–0,88). 16 bitlik ağırlık float32'nin 0,4 katı hızda.
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
  - **Taban ölçümü (2026-11-01, Colab CPU 8 vCPU; yalnız iç oranlar, dizüstüne aktarılmaz):** hazır parçalarla (llama.cpp, whisper.cpp, Piper) Qwen3-4B Q4 üretim en çok ~11 dizgecik/s, ilk dizgecik ortanca 0,9 s; konuşmayı yazıya çevirmede base ve small kısa bağlamla 4 sn sese 0,4 ve 1,1 s, large-v3-turbo 13–29 s (CPU'da kullanılamaz); Piper ilk ses parçası 92 ms; tepe bellek dil modeli ~4,3 GB, whisper small ~0,75 GB [ölçüldü: Drive `olcumler/jarvis-taban`, `jarvis-ek`, `jarvis-whisper`]. Sıradaki: dizüstünde aynı ölçüm, sonra bitiş ölçütünün önerisi.
  - **Yol (2026-11-02):**
    - Küçük bir modelde dizgecik başına yükün ≈ %7'si yorumlayıcıda, ≈ %56'sı tensör yerleşiklerinde (çağrı, küçük tensör ayırma ve işlemin kendisi birlikte) [ölçüldü: DENETİM, tensör işi ≈ 0 olan 28 katmanlı küçük bir GPT (gömme 16), `tr profile --go-profile`; gerçek modelin profili yok]. Gerçek modelde işlemin payı büyür, yorumlayıcınınki küçülür [hipotez]. Derleyici bu yüzden Jarvis'in yolunda değil; asıl kazanç CPU'da nicemli üretim, çekirdeklerin kaynaştırılması ve ses için sinir ağı işlemleri.
    - Dil tanımında henüz yazılmamış işler Jarvis'in en uzun yolu:
      - bölümü olmayanlar: canlı ses (ilk sürüm dış programla), gömme; ses üretimi için fonemleştirme. Dış programlar ve bayttan sayı 21 ve 22'de, sohbet biçimi 24'te;
      - 15'in sonraya bıraktıkları: ses üretimi için ters evrişim, ONNX okuma ve ters Fourier dönüşümleri (`irfft`, `istft`; emsali 23'ün gerekçesinde). STFT ve Mel artık 23'te;
    - 21–24 yazıldı ve DENETİM'den geçti (2026-11-02: istek 36–39 ve 42–46). Yeni bölüm yazımı, sıradaki dil işi uygulama olana dek durur (Ahmet, 2026-11-02: "mevcutlar bitsin"); önce 16.2'nin kapatması (güvenlik), sonra status'un önem taraması. Önem taraması yapıldı (2026-11-03): status'ta sınıflar (sessiz yanlış, güvenlik, güvenilirlik ...) ve öncelikler; 16.2 öncelikli işlerin ilkiydi: birleşti (2026-11-03, T1; `TR_false` metinden hiçbir eklenmiş parça kimliği üretmez), 16.3 de birleşti (2026-11-03, T1: `.TR_decode` tek dizi, `TR_batch_decode` birden çok dizi, dağarcık dışı kimlik hata, `TR_generate`'in `TR_vocab_size:`'ı dolgulu modelde örneklemeyi dağarcığa sınırlar). Gemini'nin bağımsız taraması bölüm bölüm sürüyor, DENETİM her bulguyu yeniden koşuyor.
    - Bunların araştırması ve dil tanımı yazımı Faz 2'nin koduyla aynı anda başlar; ses üretiminin üç işi de bu karara girer (Ahmet, 2026-11-02). Uygulaması fazların sırasıyla gelir.
  - Bitiş ölçütü ve sıra Ahmet'in kararı. Ölçüt adayları: zincir gecikmesi, dizgecik/s, söz kesme süresi, Türkçe kelime hata oranı, tepe bellek; sayılar dizüstündeki tabandan sonra.
- **Uyarlamak:** hazır bir modeli ince ayarla kendi işine uyarlamak, uçtan uca. LoRA'yı dil yazabiliyor; ama PEFT düzeninde kayıt, ince ayar örneği ve bitiş ölçütü yok (arşiv).
- **Herkes için:** biraz teknik bilgisi olan birinin kendi modelini kurup eğitmesi. Model Sihirbazı, süre ve maliyet tahmini, küçük model tarifleri (arşiv).
- **Masaüstünde çalışan bir TR modeli** (Ahmet, 2026-11-01: "önce TR'yi bitirelim, tüm fazları; ondan sonra"): açık ağırlıklı, Türkçe ve İngilizce bilen bir model, bu dizüstünde en hızlı çalışacak biçimde; doğrulanmış TR programlarıyla ince ayar. Fazlardan sonra.
