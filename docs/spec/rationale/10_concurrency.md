# 10. Eşzamanlılık: emsal ve gerekçe

Kurallar: [`10_eszamanlilik.md`](../10_eszamanlilik.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 10.1

- *Emsal:* Python'un `asyncio.create_task`'ı ve JavaScript'in `Promise`'i görev değeri verir, sonuç beklenerek alınır; Go'nun `go` deyimi değer vermez, sonuç kanalla taşınır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç bir ajanın birden çok uzak modele birlikte sormasını ve bir hizmetin birden çok isteğe cevap vermesini ister; ikisi de görevin sonucunu almayı gerektirir. Bugün `TR_async` bir ifade değil ve sonuç ancak paylaşılan bir değişkene yazılarak alınıyor [ölçüldü]; bu, sırayı sonuca karıştırır (10.8). Görev değeri sonucu ve hatayı birlikte taşır.

### 10.2

- *Emsal:* CPython'un küresel yorumlayıcı kilidi (GIL) aynı anda bir iş parçacığının Python kodu çalıştırmasına izin verir ve kilidi süreyle devreder (`sys.getswitchinterval()` 0,005 s); NumPy matris çarpımında kilidi bırakır: 60 çarpım sürerken 5 ms'de bir uyanan iş parçacığının en büyük boşluğu 6,9 ms [ölçüldü: DENETİM 10-1, CPython 3.12.6]. NumPy kilidi bırakınca okunan dizinin başka iş parçacığından yazılmasını engellemez; *bunu almadık.*
- *Gerekçe:* veri yarışı ajanın en zor gördüğü hatadır: bazen olur, bazen olmaz. Tek kilit onu dilden kaldırır; ajan kilit yazmaz. Saf TR döngüsü görevlerle hızlanmaz, CPython da hızlanmaz [ölçüldü: DENETİM 10-1]; yapay zekâ işinin ağır kısmı tensör ve veri işidir, o yerleşiklerde paralel çalışır.
- *Gerekçe (devir ve bırakma):* amaç "Sunmak" ve "sabit hızlı bir döngüde, gecikmeye dayanıklı" çalışmayı ister. Kilit tur sayısıyla devredilir ve uzun yerleşikler kilidi tutarsa model çalıştıran görev öbür görevleri yüzlerce milisaniye bekletir: bir istek ya da algılayıcı okuması modelin bütün ileri geçişi kadar gecikir (60 çarpımda 301 ms) [ölçüldü: DENETİM 10-1]. Süreyle devir ve uzun yerleşiklerin kilidi bırakması bunu kapatır. Bölünmezlik kuralı veri yarışı yokluğunu tensörde de korur: bir yerleşiğin okuduğu ağırlığa başka görev o sırada yazamaz, yazan bekler.
- *Gerekçe (söz ve izinler):* kural hangi yerleşiğin kilidi bıraktığını değil, görevin gözlediği gecikmeyi söyler: kilidi bırakmanın bedeli (yarışmasızken ~25 ns, devir mikrosaniyeler) küçük bir tensör işlemini (1,14 µs) katlar, bu yüzden küçük işlem kilidi tutar ve bölünmezliğini kilitten alır [ölçüldü: DENETİM 10-2]. İzinlerin hepsini birden almak ve geri çağırmadan önce bırakmak döngüsel beklemeyi kapatır: adım adım alınan izinlerde A'yı tutup B'yi bekleyen ile B'yi tutup A'yı bekleyen birbirini sonsuza dek bekler. İzin depoda durur, çünkü görünümler aynı depoya yazar; bugünkü sürüm sayacı da depodadır.

### 10.3

- *Emsal:* 10.2'deki gibi.
- *Gerekçe:* bekleyen bir görev kilidi tutarsa öbürleri de bekler ve eşzamanlılık bir işe yaramaz.

### 10.4

- *Emsal:* JavaScript'in `let`'i döngünün her turunda yeni bir bağ kurar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* her turun görevi kendi değişkenini görmezse bütün görevler son turun değerini okur; bilinen ve sessiz bir yanlıştır.

### 10.5

- *Emsal:* Python'un `asyncio.gather`'ı sonuçları verilen sırayla döndürür; hatalı bir görev olunca ilk hatayı hemen fırlatır, öbürlerini beklemez [ölçüldü: DENETİM 10-1, CPython 3.12.6]. *İkincisini almadık.*
- *Gerekçe:* Güvenilir: sonuçlar görevlerin bitme sırasına göre değil verildikleri sıraya göre alınırsa program her çalışmada aynı sonucu verir. Hata için de aynısı: hangi hatanın fırladığı bitme zamanına bağlı olursa aynı program iki çalışmada iki ayrı hata verir; bütün görevleri bekleyip verilen sıradaki ilk hatayı fırlatmak belirlenimcidir.

### 10.6

- *Emsal:* Python, hiç alınmayan görev hatası için yalnız bir uyarı yazar ("Task exception was never retrieved") ve program 0 koduyla biter [ölçüldü: DENETİM 10-1]. *Almadık.*
- *Gerekçe:* görevin hatası sessizce kaybolursa program yanlış sonuçla başarılı görünür. Hata beklenen yerde fırlar; beklenmediyse yazılır ve program hata koduyla biter. İptal bir hata değil, istenen bir durmadır; yazılsaydı her zaman aşımı yanlış bir hata kaydı bırakırdı.

### 10.7

- *Emsal:* Python'un `asyncio.wait_for`'ı süre dolunca `TimeoutError` verir ve görevi iptal eder [ölçüldü: DENETİM 10-1]; Go'nun `context.WithTimeout`'u da işi iptal ettirir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç "sabit hızlı bir döngüde, gecikmeye dayanıklı" çalışmayı ve uzak modeli kullanmayı ister; bir cevabı sınırsız beklemek döngüyü durdurur. Süresi dolan görev sürseydi program bitmeden önce onu yine beklerdi (10.1) ve program uzak cevaba bağlı kalırdı. Görevi sürdürmek isteyen döngü beklemez, `TR_done` ile sorar.

### 10.8

- *Emsal:* bütün eşzamanlı dillerde görevlerin sırası zamanlayıcıya bağlıdır.
- *Gerekçe:* Güvenilir, sırayı belirlenimci yapmayı değil, sonucu sıradan bağımsız yazmayı ister. Bugün görevlerin paylaşılan listeye ekleme sırası beklemelere göre değişiyor: `[4, 3, 2, 1, 0]` [ölçüldü]. Kural bunu açıkça söyler ve doğru yazımı (dönüş değeri, sırayla toplama) verir.

### 10.9

- *Emsal:* Python'un `Task.cancel()`'ı görevi bir sonraki beklemesinde `CancelledError` ile durdurur. `except Exception` iptali yakalamaz (`CancelledError` `Exception`'dan türemez), `except BaseException` yakalar ve görev sürer; `asyncio.TaskGroup` bir alt görev hata verince kardeşlerini iptal eder [ölçüldü: DENETİM 10-2, CPython 3.12.6]. Trio ve Kotlin'in yapılı eşzamanlılığı da iptali alt görevlere geçirir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç uzak modeli kullanmayı, sunmayı ve gecikmeye dayanıklı çalışmayı ister; üçü de artık gerekmeyen bir işi durdurabilmeyi gerektirir. Yakalanabilen iptal, ajanın geniş bir `TR_catch` ile iptali farkında olmadan yutmasına yol açar; Python bunu hata hiyerarşisiyle önler; TR'de hiyerarşi yoktur, `TR_catch` her hatayı yakalar. Bu yüzden iptal yakalanmaz, yalnız ertelenen işler çalışır (kaynaklar kapanır). İptalin 1.5'in durdurmasıyla aynı noktaları ve aynı yerleşik ayrımını kullanması ajanın öğrendiği tek bir durma kuralı bırakır; saf hesabın bırakılması iptali gerçekten hızlı yapar. Alt görevlere geçmesi, iptal edilen görevin bıraktığı ve kimsenin beklemediği işlerin programı açık tutmasını önler.
