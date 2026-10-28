# 11. Modüller: emsal ve gerekçe

Kurallar: [`11_moduller.md`](../11_moduller.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 11.1

- *Emsal:* Go'da aktarma dosyanın başındadır; Python'un göreli aktarması paketin konumuna göredir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: başka bir klasörden çalıştırılan program yanındaki modül yerine çalışma klasöründe duran aynı adlı bir dosyayı yüklememelidir; kod, yazıldığı dosyanın yanındadır. Modülün en dışında yalnız tanım olması 1.1'in kuralıdır: aktarmak yan etki doğurmaz.

### 11.2

- *Emsal:* Python'da `import m as x` bir modül nesnesidir, sözlük değildir; Go'nun paket adı da bir ad alanıdır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* TR statik tiplidir (1.2) ve sözlüğün değerleri tek tiplidir (3.7): bir modülün fonksiyonları ve sabitleri bir sözlüğe sığmaz, tipleri kaybolur. Modül değeri her adın tipini çalışmadan önce bilir; yanlış ad denetimde yakalanır. Bugün `TR_as` bir `TR_Dict` bağlıyor ve `m["PI"]` çalışıyor [ölçüldü]: ad bir metindir, yanlış yazılırsa çalışırken hata verir.

### 11.3

- *Emsal:* Python ve Go modülü bir kez yükler [kanıtsız: bu tur açılmadı].
- *Gerekçe:* aynı modülün iki kopyası iki ayrı durum demektir; sonuç hangi kopyanın kullanıldığına bağlı olurdu.

### 11.4

- *Emsal:* derlenen dillerde (Go, Rust) bütün paketler program çalışmadan önce denetlenir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* modüldeki bir hata çalışmanın ortasında çıkarsa program yarım iş yapmış olur. 1.3'ün kararı (baş geliştirici, 2026-10-27): her şey önce denetlenir. Yakalanabilen aktarma hatası bir programın kendi kodunun yarısını çalıştırması demekti.

### 11.5

- *Emsal:* Python'da modülün kodu kendi `__file__`'ını bilir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* bir modülün kodu kendi dosyasına göre yazılmıştır; çağıranın dosyasına göre yol çözmek modülü taşıyınca bozar. Hatanın kendi dosyasıyla yazılması ajanın hatayı doğru dosyada aramasını sağlar.

### 11.6

- *Emsal:* Go'nun standart kitaplığı derleyiciyle gelir ve kullanıcının paketleri onu gölgeleyemez [kanıtsız: bu tur açılmadı].
- *Gerekçe:* `tr/` kitaplığı dilin parçasıdır (17.1): adları koddur, her gösterimde okunur. Modülün yolu ise bir metindir ve çevrilmez; bu yüzden yolun kendisi kimliklerin dilindedir, İngilizce (17.2; baş geliştirici 2026-10-29: "kabul edelim ki dünyadaki tüm yazılım dilleri ingilizce"). Bugün modül yolları Türkçe sözcüklerle (`tr/katman`): Türkçe bilmeyen ajan ve okur yolu tahmin edemez.

### 11.7

- *Emsal:* Dart'ta `_` ile başlayan ad kitaplığa özeldir ve dil bunu uygular; Python'da `_ad` yalnız gelenektir; Go dışa açıklığı büyük harfle yapar.
- *Gerekçe:* kitaplığın dış yüzü bilinçli seçilir, iç yardımcılar kullanıcı koduna sızmaz. TR'de büyük harf sabiti söylediği için (2.3b) Go'nun yolu alınamaz; Dart'ınki alındı.

### 11.8

- *Emsal:* Go'da aynı adı iki paketten getirmek mümkün değildir (paket adıyla erişilir); Python'un `from a import *` ve `from b import *`'ı sonrakini sessizce kazandırır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* bugün iki aktarım aynı adı getirince sonraki kazanıyor, dosyanın kendi tanımı aktarılanı sessizce gölgeliyor [ölçüldü]. Ajan hangi `kare`'nin çağrıldığını bilmez; sonuç sessizce yanlış olur. Hata iki yeri gösterir ve `TR_as`'ı önerir.
