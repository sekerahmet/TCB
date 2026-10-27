# TR Dil Tanımı

Bu belge TR dilinin kısa tanımıdır. Dilin nasıl olması gerektiğini anlatır; bugünkü uygulama (`internal/lang/lexer`, `internal/lang/parser`, `internal/lang/checker`, `internal/lang/interp`) bir kuraldan farklıysa kuralın altındaki *Durum:* satırı bunu söyler (Ahmet, 2026-10-27: "bugünkü koda göre değil doğru olana göre tasarım yapıyoruz"). Bilinen eksikler [§14](14_sinirlamalar.md)'te listelenmiştir. Dili örneklerle tanımak için önce `examples/tr/01_merhaba.tr`'yi çalıştırıp numaralı örnekleri sırayla izleyin (sıralı örnek listesi: [`README.md`](../../README.md#örnekler), "Örnekler").

**Madde biçimi** (Ahmet, 2026-10-27). Madde kısa bir kuraldır; dilin sözcüklerini kimlikle yazar (`TR_kw_import`; madde, Durum ve Test satırlarında testle korunur). Hemen altında: *Emsal:* (hangi dillerden alındı, neyi neden almadık; sürümlü kaynak bağlantısıyla, her karşılaştırmada Rust da), *Gerekçe:*, uygulanmamış bir karar için *Durum:* ve *Test:*. Test satırı satır başında `- *Test:*` ile yazılır ve üç biçimden biriyle başlar: `TR testi` (`tests/` altında numarasıyla başlayan test), `` Go: `TestX` `` (adı geçen Go testi) ya da `gerekmez` ve nedeni. Bir bölümde Test satırı varsa o bölümün her kuralı bir Test satırı ister. Bölüm 1 bu biçimdedir; öbür bölümler gözden geçirildikçe geçer.

**Kural ve test eşlemesi.** Her kuralın bir numarası vardır (ör. **4.2**). Kural, numarasının ilk kısmıyla başlayan bölüm dosyasında yazılıdır (**4.2** → [`04_operatorler.md`](04_operatorler.md)). `tests/` klasöründeki testlerin adı bu numarayla başlar (`test "4.2 aritmetik: ..."`). `tr test tests` komutu tanımdaki her kuralı çalıştırarak denetler; `go test ./internal/devtools/testrunner` de her kuralın Test satırının doğru olduğunu (Test satırı henüz olmayan kuralın bir TR testi olduğunu) ve her kuralın kendi bölüm dosyasında durduğunu doğrular (*her kural bir testte*); Test satırının andığı Go testlerinin varlığını `go test ./internal` (`TestRuleTestsExist`) denetler. Numara harfli olabilir (**2.9a**); testin numarası kuralınkiyle harfi dahil aynıdır, bir kuralın birden çok testi aynı numarayı taşır. Uzun bölümler konulara göre ara başlıklara ayrılır (**15**); maddeler başlık sırasıyla numaralanır. Yeni kural konusunun son maddesinin numarasına harf eklenerek yazılır (15.17'den sonra `15.17a`), böylece var olan numaralar ve onlara bağlı testler kaymaz (Java'nın dil tanımında araya giren bölüm sonrakileri kaydırır: SE 13'te 15.28 olan bölüm SE 21'de 15.29'dur; Rust ve Kotlin kuralları bu yüzden numarayla değil adla anar). Ayrıştırıcının ve tip denetleyicisinin reddettiği kodlar (programın hiç çalışmaması beklenen durumlar) `tests/negative/` altındaki bilerek hatalı modüllerle sınanır: test modülü `TR_kw_import` ile yüklemeyi dener ve beklenen hatayı doğrular (11.4). Bu fikstürler bilerek hatalıdır; `TR_cmd_check` ve `TR_cmd_format` onları hatalı bulur.

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
20. [Kontrol noktası ve kaldığı yerden devam](20_kontrol_noktasi.md)
