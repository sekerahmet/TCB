# testrunner

`tr test` komutunun çekirdeği: `.tr` dosyalarındaki `test "ad" { ... }` bloklarını bulur, her
birini yalıtılmış bir kapsamda çalıştırır ve sonucu Türkçe raporlar.

## Standartlar

**Testler birbirinden yalıtılmıştır.** Her test kendi kapsamında çalışır; birinin tanımladığı
ad ya da yarıda kalan bir görev ötekini etkilemez.

**Yüklenemeyen dosya kaybolmaz.** Sözdizimi, tip ya da üst düzey hata yüzünden testleri
çalıştırılamayan dosya ayrıca sayılır ve raporlanır; başarılı sayılmaz.

**Dil tanımının her kuralının bir testi vardır.** Kapı testleri, dil tanımındaki (docs/spec)
her kuralın kendi bölümündeki bir testte anıldığını ve olumsuz örneklerin kullanıldığını
denetler; testi olmayan kural eklenemez.
