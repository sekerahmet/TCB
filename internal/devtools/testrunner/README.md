# testrunner

`tr test` komutunun çekirdeği: `.tr` dosyalarındaki `test "ad" { ... }` bloklarını bulur, her
birini yalıtılmış bir kapsamda çalıştırır ve sonucu Türkçe raporlar.

## Standartlar

**Testler birbirinden yalıtılmıştır.** Her test kendi kapsamında çalışır; birinin tanımladığı
ad ya da yarıda kalan bir görev ötekini etkilemez. Denetim: `TestRunFile`, `TestDeferRunsAfterTasks`.

**Yüklenemeyen dosya kaybolmaz.** Sözdizimi, tip ya da üst düzey hata yüzünden testleri
çalıştırılamayan dosya ayrıca sayılır ve raporlanır; başarılı sayılmaz. Denetim: `TestUnloadableFile`.

**Dil tanımının her kuralının bir testi vardır.** Kapı testleri, dil tanımındaki (docs/spec)
her kuralın kendi bölümündeki bir testte anıldığını ve olumsuz örneklerin kullanıldığını
denetler; testi olmayan kural eklenemez. Denetim: `TestRuleInItsOwnSection`, `TestEveryRuleHasATest`, `TestNegativeFixturesUsed`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_language_spec_test.go`: dil belirtimindeki her kuralın kendi bölümünde olması ve bir testi bulunması; belirtim testlerinin geçmesi
