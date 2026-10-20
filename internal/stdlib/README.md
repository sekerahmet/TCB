# stdlib

TR'nin standart kitaplığı: çekirdek yorumlayıcının bilmediği motorları (tensör ve yapay zekâ,
veri, parçalayıcı, dosya biçimleri, dosyalar, süreç, günlük, kontrol noktası) yorumlayıcıya
bağlar. Tam dili çalıştıran her program bu paketi içe aktarır.

## Standartlar

**Katman sınırı.** stdlib dil ve hesap çekirdeklerini içe aktarır, araçları (hata ayıklayıcı,
profil aracı) bilmez.

**Sözlük tek kaynaktır.** Her yerleşik ve üye sabit bir koda bağlıdır; kayıtlı adlar, adlı
argümanlar ve üye tipleri kod sözlüğüyle aynıdır. Testler sözlükte olmayan bir yerleşiğin
kaydedilmesini ve sözlükteki bir kodun kayıtsız kalmasını engeller.

**Her örnek çalışır.** Sözlükteki her örnek tip denetiminden geçer ve hatasız çalışır.

**Kaynaklar program sonunda kapanır.** Açık okuyucular, günlükler, önden yükleyiciler ve
geçici klasörler program bitince kapatılır ve temizlenir; çağrı birden çok kez yapılabilir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_builtin_codes_test.go`: yerleşiklerin ve üyelerin kod sözlüğüyle birebir eşleşmesi
- `behavior_definitions_test.go`: kod listesindeki örneklerin, adlı argümanların ve üye tiplerinin gerçek davranışla aynı olması
