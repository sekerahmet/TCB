# 20. Kontrol noktası: emsal ve gerekçe

Kurallar: [`20_kontrol_noktasi.md`](../20_kontrol_noktasi.md). Bu dosya her kuralın emsalini (başka araçlarda nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 20.1

- *Emsal:* 13'ün yol kuralı: okuma çalışma klasörü, modülün ve programın klasörü; yazma yalnız çalışma klasörü. PyTorch'ta kullanıcı modelin, iyileştiricinin ve örnekleyicinin durumunu ayrı ayrı toplayıp `torch.save` ile yazar ve geri yüklerken her birini kendisi geri koyar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* koşu klasörü yazılan bir yerdir; okuma kuralıyla çözülseydi başka klasörden çalışan program programın klasöründeki koşuyu sürerdi ve yazdırılan yol çalışma klasörüne göreli gibi okunurdu [ölçüldü: DENETİM 20-1]. Amaç "biraz teknik bilgisi olan biri de kendi modelini eğitebilmeli" der. Kesilen eğitimi aynı sonuçla sürdürmek için hangi durumun kaydedileceğini kullanıcının bilmesi gerekmemeli: tek nesne, tek klasör, tek çağrı.

### 20.2

- *Emsal:* PyTorch'un `state_dict`'i parametreleri ve tamponları (türev izlemeyen durum) birlikte kaydeder [kanıtsız: bu tur açılmadı].
- *Gerekçe:* kaldığı yerden aynı sonuç, bir tensörü bile unutmamayı ister: momentumu TR ile yazılmış bir iyileştiricinin durumu türev izlemeyen bir tensördür ve unutulursa sürdürülen eğitim ayrılır. Bu yüzden bütün yaprak tensörler, her cinste, kaydedilir. Sözlüğün anahtarı metindir (3.7); bir zamanlar kodlu anahtar vardı, bugün yoktur (17.1).

### 20.3

- *Emsal:* Hugging Face `Trainer` kontrol noktasını adım sonunda (`save_steps`) kaydeder [kanıtsız: bu tur açılmadı].
- *Gerekçe:* yarım adımın durumu (gradyanlar biriktirilmiş, iyileştirici adımını atmamış) kesintisiz eğitimde hiçbir zaman görülmeyen bir durumdur; ondan sürdürmek aynı sonucu vermez. Kayıt yalnız adım sınırında yapılır.

### 20.4

- *Emsal:* 20.3'teki gibi; ayrıca eğitimin sonunda elle kayıt.
- *Gerekçe:* eğitimin sonunda ya da kullanıcının istediği bir anda nokta bırakmak; klasörü vermek sonucu araçlara iletmeyi kolaylaştırır. Donma durumu noktaya girer, çünkü donma bir değere bağlı olabilir (kayıp eşiğinde gövdeyi çözmek): kaydedilmeseydi sürdürülen eğitim başka bir donma durumuyla başlar ve 20.17'nin aynılığını bozardı (15.34).

### 20.5

- *Emsal:* veritabanları ve dosya sistemleri atomik değişikliği geçici dosya ve tek yeniden adlandırmayla yapar (POSIX `rename`'in atomikliği) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* kayıt sırasında kesilen bir eğitim, kaydı yarıda bıraktığı için önceki noktasını da kaybederse kontrol noktası amacına ters işler. Özet dosyası en son yazılır: özetle tutan nokta tamdır.

### 20.6

- *Emsal:* PyTorch'ta `load_state_dict` var olan parametrelere yerinde kopyalar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç, kesilen eğitimin kaldığı yerden aynı sonuçla sürmesidir. Var olan tensörlere yerinde yazmak, iyileştiricinin ve modelin aynı nesneleri tutmaya devam etmesini sağlar: yeni nesne kurulsaydı iyileştirici eski tensörleri güncellerdi.

### 20.7

- *Emsal:* veritabanı işlemlerinin "önce doğrula, sonra yaz" düzeni [kanıtsız: bu tur açılmadı].
- *Gerekçe:* yarıda kalan bir yükleme modeli bir noktanın, iyileştiriciyi ötekinin durumunda bırakır ve eğitim sessizce yanlış sürer. Önce her şeyi denetleyip sonra yazmak bu sessiz yanlışı yüksek sesli hataya çevirir; ajan hatayı okur.

### 20.8

- *Emsal:* Hugging Face `Trainer`'ın `save_total_limit`'i en yeni noktaları tutar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* disk sınırlıdır; en yeni birkaç nokta, bozuk bir noktadan bir öncekine dönmeye yeter.

### 20.9

- *Emsal:* 20.5'in özet dosyası; bozuk dosyayı özetiyle yakalamak paket yöneticilerinin (sha256 doğrulaması) yoludur [kanıtsız: bu tur açılmadı].
- *Gerekçe:* bozuk noktayı kendiliğinden atlamak kullanıcıya haber vermeden adımlar kaybettirir. Hata neyin kaybedileceğini söyler; seçim kullanıcınındır.

### 20.10

- *Emsal:* ağ işlemlerinde yeniden deneme ve sınırlı deneme sayısı [kanıtsız: bu tur açılmadı].
- *Gerekçe:* bulut sürücüsü gibi yerlerde geçici yazma hataları olağandır; saatlerce süren eğitimi bir yazma hatası için durdurmak, kontrol noktasının korumak istediğini kaybettirir. Art arda üç başarısızlık kalıcı bir sorunu gösterir ve durdurur.

### 20.11

- *Emsal:* PyTorch'un `load_state_dict`'i: adlar ve şekiller tutmalı, varsayılan olarak eksik ve fazla anahtar hatadır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç "hazır bir modeli ince ayarla uyarlamak"ı ister; hazır ağırlıklar bu yolla yüklenir. Cins farkı hata, hassasiyet farkı çevrilir: kimlik tamponu ondalığa sessizce dönmesin (15.1).

### 20.12

- *Emsal:* PyTorch'un iyileştirici durum adları (`step`, `exp_avg`, `momentum_buffer`) dilden bağımsızdır [kanıtsız: bu tur açılmadı].
- *Emsal (oran):* PyTorch'un `state_dict`'i oranı `param_groups`'ta kaydeder: "Each parameter group contains metadata specific to the optimizer, such as learning rate" [kanıtlı: docs.pytorch.org/docs/2.15, Gemini Koşu 4, DENETİM].
- *Gerekçe:* durum ayrı da kaydedilip taşınabilmeli; oran durumdan ayrıdır ama kontrol noktası onu ayrıca kaydeder (20.5): kayba bağlı düşürülen oran program adımdan yeniden hesaplayamaz, sürdürme ayrılırdı. Yükleme bir iştir, değer döndürmez (13'ün "anlamsız dönüş yoktur" kuralı).

### 20.13

- *Emsal:* 16.7; PyTorch'ta örnekleyicinin konumu kullanıcı tarafından ayrıca saklanır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* okuyucunun konumu kaydedilmezse sürdürülen eğitim başka toplu işler görür ve sonuç değişir (16.7).

### 20.14

- *Emsal:* büyük modellerin eğitiminde kayıp sıçradığında son noktaya dönülür ve sıçramayı tetikleyen toplu işler atlanır (PaLM, Chowdhery ve ark. 2022, §5.1) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç "tek GPU'dan kümeye" eğitimi ister; uzun eğitimde kayıp sıçraması olağandır. Aynı programda geri dönmek, süreci yeniden başlatmadan sürdürür.

### 20.15

- *Emsal:* 20.14'teki gibi.
- *Gerekçe:* atlanan toplu işler tensör kurmadan atlanır; hızlıdır ve konum durumda kalır.

### 20.16

- *Emsal:* 1. bölümün durdurma kuralları (1.5, 1.6, 1.9); ertelemenin koşulları tek yerde, 1.6'dadır.
- *Gerekçe:* Ctrl+C'ye basan kişi çoğu zaman eğitimi kaybetmek istemez; adım sınırında kaydedip çıkmak "kesilen eğitimin kaldığı yerden sürmesi"nin en sık yoludur. İkinci istek hemen çıkar: kişinin son sözü geçer.

### 20.17

- *Emsal:* PyTorch'ta paylaşılan üretecin durumu `torch.get_rng_state()` ve `set_rng_state()` ile alınıp verilir; Hugging Face `Trainer` kontrol noktasında bunu `_save_rng_state` ile kaydeder [ölçüldü: DENETİM 20-1, transformers'ın bugünkü sürümü]. PyTorch ve NumPy'nin paylaşılan üreteci ise kendiliğinden saatle tohumlanır: aynı program iki kez farklı sonuç verir [ölçüldü: DENETİM 20-1].
- *Gerekçe:* "Aynı program aynı veriyle aynı sonucu verir; kesilen bir eğitim kaldığı yerden aynı sonuçla sürer." Saatle tohumlanan üreteç ilk cümleyi, kaydedilmeyen üreteç ikinci cümleyi bozar. Bu yüzden üreteç her programda aynı sabit durumla başlar ve durumu kontrol noktasına girer; tohum unutulsa da sonuç aynıdır. Bit düzeyinde aynılığın koşulu aynı cihaz türüdür: farklı işlemci yolları son bitte ayrılabilir. Eşzamanlı görevlerin aynı üreteçten çekme sırası belirlenimci değildir; bu sınır yazılır, görev başına tohum çözümdür.

### 20.18

- *Emsal:* `pickle` tabanlı biçimler yüklenirken kod çalıştırabilir; `safetensors` bu yüzden yalnız veri tutar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* koşu klasörü paylaşılır ve kopyalanır; içindeki bir dosyanın kod çalıştırması ya da belleği tüketmesi bir güvenlik açığıdır. Yalnız veri, katı çözüm ve boyut sınırları bunu kapatır.
