# interp

TR'nin derleyicisi ve yorumlayıcısı. Sözdizimi ağacı çalışmadan önce bir kez Go kapanışlarına
derlenir; değerler, çekirdek yerleşikler ve metotlar, modüller, eşzamanlı görevler ve yorumlayıcı
kilidi bu pakettedir.

## Standartlar

**Bir kez derlenir, adla aranmaz.** Değişkenler derleme anında yuvalara, üye adları kodlara
çözülür; çalışma anında ad karşılaştırılmaz, yalnızca kod ve yuva kullanılır.

**Motorlar kancayla bağlanır.** Tensör, veri, parçalayıcı motorları ve hata ayıklayıcı bu paketi
içe aktarıp kendilerini kancalarla bağlar; yorumlayıcı onları bilmez. Yeni bir yetenek önce bir
kanca arayüzü olarak burada tanımlanır, uygulaması motorda yazılır. Denetim: `TestLangOrder` (internal), `TestHooksHaveCompileTimeChecks`.

**TR kodu yalnızca kilidi tutan görevde çalışır.** Eşzamanlı her görev yorumlayıcı kilidini alarak
çalışır. Uzun bekleyen işler (uyku, disk, ağ, dış süreç) kilidi bırakarak bekler; kilidi tutmayan
kod ortamlara ve değerlere dokunmaz. Denetim: `TestAsyncConcurrentDictAndListWrites`, `TestAsyncTasksStillOverlapDuringSleep`, `TestLockHandoffConcurrentTask`, `TestConcurrentCallStackSafety`.

**Hatalar satırıyla ve bir kez sarılır.** Çalışma hatası oluştuğu satırı taşır; zaten satır taşıyan
bir hata yeniden sarılmaz. Kullanıcı hataları `dene`/`yakala` ile yakalanır; kesme isteği (Ctrl+C)
yakalanmaz. Bir yerleşiğin içindeki beklenmedik panik süreci çökertmez, iç hata olarak bildirilir. Denetim: `TestInternalErrorLineOnce`, `TestInterrupt`, `TestRunRecoversFromBuiltinPanic`, `TestPanicInsideAsyncDoesNotCrashProcess`.

**Tek kural.** Göreli dosya yolları için tek kural, modül yolları için tek kural vardır; her
yerleşik yolu aynı yerden çözer. Denetim: `TestResolvePathSingleRule`, `TestFileBuiltinsUseSingleRule`, `TestImportIgnoresWorkingDir`.

**Sıra belirlenimlidir.** Sözlükler ekleme sırasını korur, yapı alanları sabit bir sırayla gezilir;
çıktı harita gezinme sırasına bağlı olmaz. Denetim: `TestOutputOrderIsDeterministic`.

**Değerin metni dosyanın dilindedir.** Programın bir değerden metin yaptığı her yer (`metin()`,
`yazdır`, metin yerleştirmesi, Metin ile `+`, `birleştir`, `dosya_yaz`, `girdi`'nin istemi, metin
metotlarının argümanları) değeri işlemin yazıldığı dosyanın dilinde çevirir (tek yol:
`VM.textOf`/`valueText`; sözcüksel: kitaplık fonksiyonu kendi dilinde, geri çağırma kendi
dosyasının dilinde). Dile bağlı olan yalnızca dilin değerlerinin adlarıdır (`doğru`, `yanlış`,
`yok`, tip adları). `Inspect` kanonik kalır: veri biçimleri (JSON, günlük, kontrol noktası) ve
iletiler onu kullanır. Denetim: `TestValueTextFollowsTheFilesLanguage`, `TestValueTextIsLexical`, `TestEveryTextConversionFollowsTheFilesLanguage`, `TestDataFormatsIndependentOfFileLanguage` (cmd/tr/e2e).

**Sıcak yol ölçülerek değişir.** Yorumlayıcı döngüsündeki ve hızlı yollardaki değişiklikler
önce/sonra hız ölçümüyle yapılır.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_semantics_test.go`: uçtan uca dil anlamı: TR programları bütün yorumlayıcıdan geçerek çalışır
- `behavior_robustness_test.go`: bütün yorumlayıcının çökme, kilitlenme ve eşzamanlılık güvenliği
- `behavior_hooks_test.go`: kanca arayüzlerini uygulayan her tipin derleme anı denetimi (`var _ interp.X = (*T)(nil)`) taşıdığı (depo genelinde)
- `behavior_determinism_test.go`: sözlük, yapı ve döngü çıktısının her çalıştırmada aynı sırada olduğu
- `behavior_checker_parity_test.go`: denetçinin ve çalışma anının tanımsız ad hatasını aynı sözlerle anlattığı
- `behavior_ownership_test.go`: sahiplik kuralları (dil tanımı 15.43): sahte bir kaynak değeriyle her kuralda değerin ulaşılabilirken hiç bırakılmadığı ve beklenen noktada bırakıldığı
- `behavior_value_text_test.go`: değerin metni işlemin yazıldığı dosyanın dilindedir (İngilizce program, Türkçe modül ve kitaplık, geri çağırma); `Inspect` kanonik kalır
- `behavior_scope_walkers_test.go`: ad ve yakalama çözümlemesinin ortak ifade gezgini (`ast.SubExpressions`) eski ayrı gezginlerle `tests/`, `lib/`, `examples/` altındaki her programda aynı adları ve aynı yakalama kararını verir (referans kopya testin içindedir)
