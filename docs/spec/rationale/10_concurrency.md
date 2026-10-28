# 10. Eşzamanlılık: emsal ve gerekçe

Kurallar: [`10_eszamanlilik.md`](../10_eszamanlilik.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 10.1

- *Emsal:* Python'un `asyncio.create_task`'ı ve JavaScript'in `Promise`'i görev değeri verir, sonuç beklenerek alınır; Go'nun `go` deyimi değer vermez, sonuç kanalla taşınır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç bir ajanın birden çok uzak modele birlikte sormasını ve bir hizmetin birden çok isteğe cevap vermesini ister; ikisi de görevin sonucunu almayı gerektirir. Bugün `TR_async` bir ifade değil ve sonuç ancak paylaşılan bir değişkene yazılarak alınıyor [ölçüldü]; bu, sırayı sonuca karıştırır (10.8). Görev değeri sonucu ve hatayı birlikte taşır.

### 10.2

- *Emsal:* CPython'un küresel yorumlayıcı kilidi (GIL) aynı anda bir iş parçacığının Python kodu çalıştırmasına izin verir; NumPy ve PyTorch gibi kitaplıklar ağır hesapta kilidi bırakır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* veri yarışı ajanın en zor gördüğü hatadır: bazen olur, bazen olmaz. Tek kilit onu dilden kaldırır; ajan kilit yazmaz. Hız kaybı ağır işin yerleşiklerde paralel çalışmasıyla karşılanır: yapay zekâ işinin ağır kısmı tensör ve veri işidir.

### 10.3

- *Emsal:* 10.2'deki gibi.
- *Gerekçe:* bekleyen bir görev kilidi tutarsa öbürleri de bekler ve eşzamanlılık bir işe yaramaz.

### 10.4

- *Emsal:* JavaScript'in `let`'i döngünün her turunda yeni bir bağ kurar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* her turun görevi kendi değişkenini görmezse bütün görevler son turun değerini okur; bilinen ve sessiz bir yanlıştır.

### 10.5

- *Emsal:* Python'un `asyncio.gather`'ı sonuçları verilen sırayla döndürür [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: sonuçlar görevlerin bitme sırasına göre değil verildikleri sıraya göre alınırsa program her çalışmada aynı sonucu verir.

### 10.6

- *Emsal:* Python, hiç alınmayan görev hatası için yalnız bir uyarı yazar ("Task exception was never retrieved") [kanıtsız: bu tur açılmadı].
- *Gerekçe:* görevin hatası sessizce kaybolursa program yanlış sonuçla başarılı görünür. Hata beklenen yerde fırlar; beklenmediyse yazılır ve program hata koduyla biter. Bugün de böyle [ölçüldü: çıkış kodu 1].

### 10.7

- *Emsal:* Python'un `asyncio.wait_for`'ı ve Go'nun `context.WithTimeout`'u beklemeyi süreyle sınırlar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* amaç "sabit hızlı bir döngüde, gecikmeye dayanıklı" çalışmayı ve uzak modeli kullanmayı ister; bir cevabı sınırsız beklemek döngüyü durdurur.

### 10.8

- *Emsal:* bütün eşzamanlı dillerde görevlerin sırası zamanlayıcıya bağlıdır.
- *Gerekçe:* Güvenilir, sırayı belirlenimci yapmayı değil, sonucu sıradan bağımsız yazmayı ister. Bugün görevlerin paylaşılan listeye ekleme sırası beklemelere göre değişiyor: `[4, 3, 2, 1, 0]` [ölçüldü]. Kural bunu açıkça söyler ve doğru yazımı (dönüş değeri, sırayla toplama) verir.
