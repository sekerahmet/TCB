# testrunner

`tr test` komutunun çekirdeği: `.tr` dosyalarındaki `TR_ctx_test "ad" { ... }` bloklarını bulur, her
birini yalıtılmış bir kapsamda çalıştırır ve sonucu Türkçe raporlar.

## Standartlar

**Testler birbirinden yalıtılmıştır.** Her test kendi kapsamında çalışır; birinin tanımladığı
ad ya da yarıda kalan bir görev ötekini etkilemez. Denetim: `TestRunFile`, `TestDeferRunsAfterTasks`.

**Yüklenemeyen dosya kaybolmaz.** Sözdizimi, tip ya da üst düzey hata yüzünden testleri
çalıştırılamayan dosya ayrıca sayılır ve raporlanır; başarılı sayılmaz. Denetim: `TestUnloadableFile`.

**Dil tanımının her kuralı neyle sınandığını söyler.** Kapı testleri, dil tanımındaki
(docs/spec) her kuralın kendi bölümünde durduğunu, altındaki Test satırının doğru olduğunu
("TR testi" ise numarasıyla başlayan bir test var, "gerekmez" ise nedeni yazılı), Test satırı
olan bölümde her kuralın bir Test satırı olduğunu, biçime uymayan Test satırı kalmadığını,
Test satırı henüz olmayan bölümde her kuralın bir TR testi olduğunu ve olumsuz örneklerin
kullanıldığını denetler.
Denetim: `TestRuleInItsOwnSection`, `TestEveryRuleHasATest`, `TestRuleTestLineForms`, `TestSectionTestProblems`, `TestNegativeFixturesUsed`.

**Gözden geçirilmiş bölüm dilin sözcüklerini kimlikle yazar.** Test satırı biçimine geçmiş
bölümde madde ve Test satırları ve durum belgesinin (`docs/status.md`) her satırı ters
tırnak içinde bir kodun hiçbir gösterim dilindeki adını (`aktar`, `import`) taşımaz, kimliğini
(`TR_kw_import`) yazar; Emsal ve Gerekçe başka dillerin kodunu bilerek yazdığı için bakılmaz.
Denetim: `TestReviewedSectionsWriteIdentities`.

**Gözden geçirilmiş bölüm yalnız kuralı yazar.** Bugünkü uygulamanın kuraldan farkı ve
yapılacaklar bölümde *Durum:* satırı olarak değil, `docs/status.md`'de bölümün başlığı altında
durur; gözden geçirilmiş her bölümün orada başlığı vardır. Denetim: `TestReviewedSectionsHoldOnlyRules`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_language_spec_test.go`: dil belirtimindeki her kuralın kendi bölümünde olması ve bir testi bulunması; belirtim testlerinin geçmesi
