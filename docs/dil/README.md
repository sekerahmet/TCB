# TR Dil Tanımı

Bu belge TR dilinin **bugünkü uygulamaya** (`internal/dil/sozcuk`, `internal/dil/ayristir`, `internal/dil/denetci`, `internal/dil/yurut`) dayanan kısa tanımıdır. Dilek değil, davranış anlatır; bilinen eksikler [§14](14_sinirlamalar.md)'te açıkça listelenmiştir. Dili örneklerle tanımak için önce `ornekler/02_dil_turu.tr`'yi çalıştırın (sıralı örnek listesi: [`README.md`](../../README.md#örnekler), "Örnekler").

**Kural ve test eşlemesi.** Her kuralın bir numarası vardır (ör. **4.2**). Kural, numarasının ilk kısmıyla başlayan bölüm dosyasında yazılıdır (**4.2** → [`04_operatorler.md`](04_operatorler.md)). `testler/` klasöründeki testlerin adı bu numarayla başlar (`test "4.2 aritmetik: ..."`). `tr test testler` komutu tanımdaki her kuralı çalıştırarak denetler; `go test ./internal/arac/testkosucu` de tanımda numarası geçip testi olmayan bir kural kalmadığını ve her kuralın kendi bölüm dosyasında durduğunu doğrular (*her kural bir testte*). Numara harfli olabilir (**2.9a**); testin numarası kuralınkiyle harfi dahil aynıdır, bir kuralın birden çok testi aynı numarayı taşır. Ayrıştırıcının ve tip denetleyicisinin reddettiği kodlar (programın hiç çalışmaması beklenen durumlar) `testler/olumsuz/` altındaki bilerek hatalı modüllerle sınanır: test modülü `aktar` ile yüklemeyi dener ve beklenen hatayı doğrular (11.4). Bu fikstürler bilerek hatalıdır; `tr kontrol` ve `tr biçimle` onları hatalı bulur.

## Bölümler

1. [Program](01_program.md)
2. [Sözcük yapısı](02_sozcuk_yapisi.md)
3. [Tipler ve değerler](03_tipler.md)
4. [Operatörler](04_operatorler.md)
5. [Deyimler ve kontrol yapıları](05_deyimler.md)
6. [Fonksiyonlar](06_fonksiyonlar.md)
7. [Yapılar](07_yapilar.md)
8. [Hata yönetimi](08_hata_yonetimi.md)
9. [ertele](09_ertele.md)
10. [Eşzamanlılık ve yorumlayıcı kilidi](10_eszamanlilik.md)
11. [Modüller](11_moduller.md)
12. [Testler](12_testler.md)
13. [Yerleşik fonksiyonlar ve metotlar](13_yerlesikler.md)
14. [Bilinen sınırlamalar](14_sinirlamalar.md)
15. [Tensörler ve otomatik türev](15_tensorler.md)
16. [Veri ve parçalayıcı](16_veri_parcalayici.md)
17. [Kodlar](17_kodlar.md)
18. [Dosya ve yol](18_dosya_ve_yol.md)
19. [Süreç ve kayıt: ortam değişkenleri, komut satırı, günlük, ilerleme çubuğu](19_surec_ve_kayit.md)
