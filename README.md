# TR

TR, yapay zekânın bütün işleri için bir programlama dilidir: verinin hazırlanmasından modelin bir robotun içinde çalışmasına kadar her iş tek dille yapılır.

## Amaç

TR her türden model içindir: dil, görüntü, ses, video ve bunları birlikte işleyen çok kipli modeller. Bu modellerle yapılan işlerin hepsi TR'de yazılabilmeli:

- **Veriyi hazırlamak.** Metni, görüntüyü ve sesi toplamak, temizlemek, eğitime hazırlamak; kendi parçalayıcısını (tokenizer) eğitmek.
- **Model yazmak ve eğitmek.** Bir modelin yapısını yazmak ve onu kendi verisiyle sıfırdan eğitmek; tek GPU'dan birçok GPU'ya ve kümeye kadar.
- **Ölçmek ve değerlendirmek.** Modelin ne kadar iyi olduğunu ölçmek, başka modellerle aynı sınavda karşılaştırmak.
- **Hazır modelleri çalıştırmak.** Yayımlanmış bir modeli alıp CPU'da ya da GPU'da hızlı çalıştırmak:
  - dil: metin üretmek, sohbet etmek, çevirmek, özetlemek;
  - görüntü: görüntüyü tanımak, içindeki nesneleri bulmak, görüntü üretmek;
  - ses: konuşmayı yazıya, yazıyı konuşmaya çevirmek;
  - çok kipli: görüntüyü görüp onun hakkında konuşmak.
- **Uyarlamak.** Hazır bir modeli ince ayarla kendi işine uyarlamak; büyük bir modelin bilgisini küçüğüne aktarmak (damıtma); modeli küçültüp hızlandırmak (nicemleme).
- **Sunmak.** Modeli bir hizmet olarak açmak: başka programlar ona API ile soru sorar.
- **Makinenin içinde çalıştırmak.** Modeli robot, telefon ya da gömülü bir cihazın içinde, onun işlemcisinde çalıştırmak; algılayıcıdan gelen veriyle, sabit hızlı bir döngüde, gecikmeye dayanıklı biçimde.
- **Uzaktaki modeli kullanmak.** İnternetteki bir modeli bağlantı ve API ile programın parçası olarak kullanmak; modelin araç kullandığı ajanlar ve belgelerden bilgi bulan sistemler kurmak.
- **Paylaşmak.** Eğitilen modeli yaygın biçimlerde dışarı vermek ve başka araçlarda eğitilmiş modelleri almak.

Her karar bu işlerin hepsine göre verilir.

**Herkes için.** Biraz teknik bilgisi olan biri de kendi modelini kurup eğitebilmeli.

**Güvenilir.** Aynı program aynı veriyle aynı sonucu verir; kesilen bir eğitim kaldığı yerden aynı sonuçla sürer.

**Kimlik kodlu dil.** Bu dilde her şey kimlik koduyla yazılır: dosyalar diskte, program içeride dilden bağımsız kimliklerle (`TR_fn_sum`) çalışır. İnsanların okuyabilmesi için kod Türkçeye, İngilizceye ya da başka bir dile çevrilir; ama hiçbir insan dilinde kod yazılmaz. Bugün Türkçe ve İngilizce var, başka diller sırada; hiçbir dilin ayrıcalığı yoktur.

## Dil tanımı

Dilin kuralları bölüm bölüm [dil tanımında](docs/spec/README.md).
