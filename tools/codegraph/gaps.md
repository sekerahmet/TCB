# codegraph eksikleri

codegraph'ın (ve `tr dizin`'in) yetmeyen ya da yanlış cevaplarının biriktiği yer. Yol haritası
yalnızca aracın gelişimini özetler ([madde 7](../../docs/roadmap.md)); tek tek eksikler buradadır.

**Kim yazar:** dosyayı yalnızca ana oturum günceller. Yan oturumlar ve ajanlar codegraph'ın
yanlış, eksik, belirsiz ya da yavaş cevabını (kancanın gösterdikleri dahil, `tools/edithook`) ana
oturuma bildirir: komut, beklenen, gelen, ikinci yoldan doğrulama (grep, kaynak). Ana oturum
yeniden üretip doğruladığını ya da baş geliştiriciyle birlikte kararlaştırdığını ekler. Ad
tahminiyle aranıp bulunamayan şey eksik değildir (CLAUDE.md, "Kod grafiğinin cevabı nasıl
okunur").

**Sınıf (Ahmet, 2026-10-25):** yanlış bir değişikliğe götüren cevap (kesin grupta yanlış yer,
yanlış "yok", yanlış kenar, ortama bağlı düşen depo testi) **kritik**tir; sıralama, etiket ve
gösterim **ince ayar**dır.

**Akış:** maddeler birikir ve CODEGRAPH oturumuna toplu gider (tek tek gönderilmez); gönderilen
madde "CODEGRAPH'ta (tarih)" diye işaretlenir. Düzeltme ana dala birleştirilince ana oturum
düzeltmeyi aynı komutla doğrular ve maddeyi siler (geçmişi git'tedir). İstenen her zaman örneği yamalayan değil aynı sınıftaki
her durumu kapatan genel çözüm ve sınıfı koruyan bir testtir.

## Kritik

- **Depo geneli kural testleri `tests-for`'da görünmüyor.** CODEGRAPH'ta (2026-10-25).
  - Komut: `go run ./tools/codegraph tests-for tools/edithook/main.go` → yalnızca paketin testleri.
  - Beklenen: o dosyaya uygulanan depo geneli kurallar da: `internal.TestNoTurkishLiterals` (Go metninde Türkçe sabit), düzen testleri (`internal/layout_test.go`: dosya ve fonksiyon boyu, test dosyası adı), `codegraph.TestEveryTrackedDataFileHasATestOrAReason` (izlenen veri dosyası). Doğrulama: aynı gün tam denetim `TestNoTurkishLiterals` ile `tools/edithook/main.go`'da kaldı; cevap bu kuralı göstermemişti.

## İnce ayar

Açık madde yok.

## Kalan işler (eksik değil, yeni yetenek)

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-24'ün; iş açılmadan önce yeniden koşulur): 60 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 116 paket değişkeni; korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
