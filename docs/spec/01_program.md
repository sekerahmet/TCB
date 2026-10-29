# 1. Program

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "1. Program".

Her kuralın emsali ve gerekçesi: [`rationale/01_program.md`](rationale/01_program.md).

**1.1** Program, `tr`'ye verilen tek bir `.tr` dosyasıdır; en dışındaki deyimler yukarıdan aşağı çalışır, `main` yoktur. Modül (`TR_import` ile alınan dosya) ve test dosyası en dışta yalnız tanım içerir: `TR_func`, `TR_struct`, `TR_enum`, `TR_interface`, `TR_const`, `TR_var`, `TR_import`; test dosyasında `TR_test` de. Oradaki `TR_const` ve `TR_var`'ın ilk değeri sabit ifadedir: sayı; yerleştirdiği ifadeler sabit olan metin (2.7); mantık değeri; `TR_none`; başka bir sabit ve bunların işlemleri; öğeleri sabit liste ve sözlük; alanları sabit olan kurucusuz yapı ve durumlu tip değeri; sabit argümanlı `TR_Bytes(...)` (bayt dizisi değiştirilemez); fonksiyon adı ya da dış değişken yakalamayan fonksiyon ifadesi; sonucu yalnız argümanına bağlı sayı yerleşiği (`TR_sqrt(64.0)`); fonksiyonu çalıştırmadan fonksiyon döndüren yerleşik (`TR_fuse`, `TR_replay`, `TR_custom_grad`). Başka çağrı yoktur; modül aktarmak kod çalıştırmaz.
- *Test:* Go: `TestProgramRunsTopToBottom`. Modül ve test dosyası kısmı uygulanınca denetleyicinin testleri.

```tr
# TR 4
TR_print("Merhaba, dünya!")
```

**1.2** TR statik tiplidir: her değerin tipi çalışmadan önce bilinir. Fonksiyonun parametreleri ve dönüşü tipini yazar; yerel değişkenin tipi ilk değerinden çıkar.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca denetleyicinin testleri.

**1.3** Çalışmadan önce program, aktardığı modüllerle birlikte, bütünüyle ayrıştırılır ve tip denetiminden geçer (tip hataları ve tanımsız adlar); hata varsa hiçbir satır çalışmaz, çıkış kodu 1'dir. Sözdizimi hatası ayrıştırmada, denetim hatası tip denetiminde bulunur; ikisi de çalışmadan önce verilir ve iletinin başlığı türü söyler. `TR_import` yalnız dosyanın başında, başlıktan (2.1) sonra ve öbür tanım ve deyimlerden önce yazılır; koşullu aktarma ve koşullu modül yoktur, her modül her makinede denetlenir. Platform ve cihaz farkı çalışırken bakılan değerlerle çözülür (`TR_default_device()`); makinede olmayan bir donanım onu kullanan satırda çalışma hatası verir.
- *Test:* Go: `TestExampleErrors` (hatadan önceki satırın çalışmadığı da), `TestModulesAreCheckedBeforeAnyLineRuns`, `TestCheckImportedModules`; olumsuz fikstür `tests/negative/aktarma_yeri.tr` (aktarmanın yeri).

**1.4** Dosya adından sonraki argümanlar programındır ve her çalıştırma yolunda aynıdır (`TR_cmd_run`, `TR_cmd_debug`, `TR_cmd_build`'in ürettiği uygulama, düzenleyicinin `"args"` ayarı); onları `TR_command_line` okur. Argüman verildiği hâlde programın en dışından ulaşılabilen bir `TR_command_line` çağrısı yoksa program çalışmaz: kullanım hatası, çıkış kodu 2. Ulaşılabilirlik kesin bilinemiyorsa (fonksiyon değeriyle çağrı) çağrı var sayılır. Böyle bir programa yalnız `-h` ya da yardım seçeneği (`TR_opt_help`) verilirse programın seçeneği olmadığı yazılır, çıkış kodu 0. 1.3'ün hatası bundan önce gelir. Başlatıcıların argümanları programın sayılmaz ve görülmez: ROS 2'nin `--ros-args` ile başlayıp `--` ile ya da sonda biten blokları; `--local-rank=N` ve `--local_rank=N` (aynı bilgi `LOCAL_RANK` ortam değişkeninde).
- *Test:* Go: `TestScriptArgumentsPassed`, `TestBuiltAppArguments`, `TestCommandLineOptions`, `TestArgumentsToAProgramThatReadsNoneAreAUsageError`, `TestHelpAndLauncherArgumentsOfAProgramThatReadsNone`, `TestPositionalArgumentsToAProgramThatReadsNoneAreAUsageError`, `TestProgramArgumentsLeaveLaunchersOut`, `TestCheckArguments`, `TestReadsCommandLineReachability`, `TestLibraryModulesDoNotReadTheCommandLine`.

**1.5** Ctrl+C ya da bir sonlandırma isteği (Unix'te SIGTERM, SIGHUP, SIGUSR1; Windows'ta Ctrl+Break ve konsolun kapanması) programı bir sonraki güvenli noktada nazikçe durdurur: kullanıcı fonksiyonu çağrısında, döngünün her turunda ve süren bir yerleşiğin içinde.
- Programın durumunu değiştirmeyen yerleşik bırakılır, sonucu atılır: hesap, dosya okuma, dışarıdan veri ya da bağlantı beklemek (girdi, algılayıcı, ağ), `TR_sleep`, `TR_input`, `TR_wait_all`. Cihazda (GPU) başlamış hesap biter.
- Durum değiştiren yerleşik biter: indeksle atama, `TR_write_slice`, `TR_backward`, iyileştiricinin adımı, dosyaya yazma, yarım gönderilmiş ileti ya da yarım okunmuş kayıt, rastgele üretecin ve veri yükleyicinin ilerlemesi (yükleyicinin bir sonraki parçayı beklemesi dahil).
- Sonra `TR_defer` blokları çalışır; içlerindeki beklemeler bırakılmaz. `TR_try`/`TR_catch` isteği yakalamaz. `TR_async` görevleri de böyle durur.
- Süreç isteğin kendi sinyaliyle biter (1.9). Durdurmayı üstlenen program için 1.6, biterken gelen istek için 1.15 geçerlidir.
- *Test:* Go: `TestInterruptGracefulExit`, `TestInterruptWakesWait`, `TestInterrupt`, `TestInterruptWaitsForBuiltin`, `TestUnhandledStopAbandonsWaitsButNotInDefer`, `TestAbandonableWaitEndsWithTheStop`, `TestAbandonedInputKeepsTheLine`, `TestStateChangesAreNeverAbandoned`, `TestWaitsInDefersAfterAStopAreNotAbandoned`. Hesabın ortasında bırakma, veri okuyucuyu bekleme ve alt süreç uygulanınca Go testi; döngünün her turunda durduğunu sınayan Go testi yazılacak.

**1.6** Program durdurmayı `TR_handle_stop()` ile üstlenebilir: ondan sonra gelen ilk istek programı durdurmaz, kaydedilir; `TR_stop_requested()` istek geldiyse `TR_true` döner ve hiçbir şeyi değiştirmez.
- İstekten sonra yeni bir şey bekleyen yerleşik (`TR_sleep`, girdi, bağlantı, kamera karesi, ileti) "durdurma istendi" hatasıyla döner; süren bir işi bitiren yerleşik (cevap yazmak, görev beklemek) ve hesaplar sürer. `TR_defer` bloklarında, bu hatayı yakalayan `TR_catch` gövdesinde ve oralardan çağrılan fonksiyonlarda beklemeler normal çalışır.
- Program kendi sınırında biter (döngüden çıkar, fonksiyondan döner, `TR_exit`); ertelenen işler çalışır, çıkış kodu 1.9'dadır.
- `TR_Checkpoint` kuruluysa istek yalnız kaydedilir, beklemeler hata vermez: istekten sonraki ilk `.TR_update(...)` ya da `.TR_save()` bütün durumu kaydeder, sonra program 1.5'teki gibi durur. Bu erteleme süreyle sınırlıdır: istek ilk `.TR_update(...)`'ten önce geldiyse (kayıtlı bir nokta yoksa da sürdürülen noktadan bu yana tamamlanmış bir adım yoktur, bir şey kaybolmaz) ya da şimdiye kadarki en uzun ardışık aralığın iki katı içinde kayda varılmazsa istek 1.5'teki gibi uygulanır; ilk aralık kurulumdan ilk `.TR_update(...)`'e kadardır; süre monoton saatle ölçülür ve bir bekleme sürerken dolarsa 1.5 o anda uygulanır. Program `TR_handle_stop` ile de üstlendiyse kontrol noktası yalnız kaydeder. Kayda varmadan biten programda ek kayıt yapılmaz; sürdürme son kayıttan olur.
- *Test:* Go: `TestDeferredInterruptSavesAtStepBoundary`, `TestCheckpointResumeBitExact`, `TestRequestStopDispositions`, `TestDeferredStopExpires`, `TestHandledStopTurnsWaitsIntoErrors`, `TestHandledStopEndsAtTheProgramBoundaryWithTheSignal`, `TestHandledStopThenExitCode`, `TestStopBeforeTheSecondUpdateIsNotDeferred`, `TestSaveHonoursTheDeferredRequestAndStops`, `TestHandledStopCheckpointOnlySaves`, `TestDeferralExpiresWhileWaiting`, `TestNoExtraSaveAtTheEndAndTheRequestIsKept`, `TestDeferralWindowIsTwiceTheLongestInterval`, `TestHandleStopCancelsTheDeferralTimer`.

**1.7** İlk istekten sonra program durmadan gelen ikinci Ctrl+C süreci hemen SIGINT ile bitirir (kabuk 130 görür); durum değişikliği ya da `TR_defer` blokları sürerken, 1.6'da program kendi sınırına varmadan ve kayıt sürerken de.
- *Test:* Go: `TestSecondInterruptExitsImmediately`.

**1.8** Durdurmanın süresini programı başlatan belirler; süre dolunca süreç zorla biter, ertelenen işler ve 1.6'nın kaydı yetişmeyebilir. Bazı ortamlarda istek hiç gelmez (Windows'ta oturum ya da bilgisayar kapanınca). Süre yetmeyecekse istek erken istenir. Ortamların süreleri ve erken isteğin yazımı gerekçededir. Eğitimde asıl güvence kontrol noktasının düzenli kaydıdır.
- *Test:* gerekmez: davranış işletim sistemine ve süreci başlatan denetleyiciye bağlı.

**1.9** Program sona gelince önce görevlerini bekler, sonra en dıştaki ertelenen işler çalışır. Program yakalanmayan bir hatayla biterse öbür görevler 1.5'teki gibi durur ve beklenir, sonra ertelenen işler çalışır. Çıkış kodu: 0 başarı; 1 hata (1.3'ün denetimi, yakalanmayan hata, görevdeki yakalanmayan hata, ertelenen işin hatası); 2 kullanım hatası (1.4 ve `tr`'nin kendi kullanım hatası); başka bir kod `TR_exit(kod)` ile (1.10).
- Durdurma isteği geldiyse süreç o isteğin sinyaliyle biter: kabuk Ctrl+C'de 130, SIGTERM'de 143, SIGHUP'ta 129 görür; SIGUSR1'de süreç sinyalle ölmez, 128 ile sinyalin numarasıyla (Linux'ta 138) çıkar, kabuk aynı kodu görür; Windows'ta Ctrl+C ve Ctrl+Break sonunda kod `STATUS_CONTROL_C_EXIT`'tir.
- Durdurmayı üstlenen program (1.6) sona gelirse yine sinyalle, `TR_exit(kod)` ile biterse o kodla, ertelenen işi hata verirse 1 ile biter.
- Görevdeki yakalanmayan hata ve ertelenen işin hatası `TR_exit`'in kodundan önce gelir: çıkış 1. Ertelenen işten çağrılan `TR_exit` yalnız programın kendi yakalanmayan hatasının 1'ini değiştirir (1.13).
- "Kaldığı yerden sürdürülebilir" bilgisi çıkış kodunda değil, kontrol noktasındadır.
- TR'nin seçmediği çakışan kodlar: uygulamanın iç çöküşü (status), Windows'ta yerel bir bileşenin `abort`'u 3, sinyalle ölen süreç 128+N.
- *Test:* Go: `TestExampleErrors`, `TestCommandLineOptions`, `TestInterruptGracefulExit`, `TestSecondInterruptExitsImmediately`, `TestExitCodes`, `TestToolUsageErrorsExitTwo`, `TestHandledProgramExitDecisions`, `TestUnhandledStopKeepsTheSignal`, `TestDeferredAndTaskErrorsMakeTheExitOne`, `TestUncaughtErrorStopsTasksBeforeTheOutermostDefers`, `TestErrorsComeBeforeTheExitCode`.

**1.10** Program erken ya da kendi koduyla yalnız `TR_exit(kod)` ile biter; `TR_return` yalnız fonksiyonun içinde yazılır, programın en dışında sözdizimi hatasıdır. Bu çıkış naziktir: `TR_defer` blokları çalışır, `TR_try`/`TR_catch` onu yakalamaz; bir `TR_async` görevinden çağrılsa da bütün program biter, öbür görevler 1.5'teki gibi durur.
- *Test:* Go: `TestExitCodes`, `TestExitSkipsCatchRunsDefersAndLastCallWins`, `TestExitFromTaskEndsTheProgram`, `TestProgramTopLevelReturnIsError`.

**1.11** `TR_exit`'in kodu 0–125 arasında bir `TR_Int`'tir; dışındaki değer çalışma hatasıdır. 1 ve 2, 1.9'daki anlamlarıyla kullanılır; programın kendi anlamları 4'ten başlar (3, Windows'ta `abort` ile karışır).
- *Test:* TR testi (`tests/program_test.tr`); Go: `TestExitCodeAndRange`.

**1.12** `TR_cmd_test` altında `TR_exit` süreci bitirmez: bir testin içinden (çağırdığı fonksiyon ya da başlattığı `TR_async` görevi dahil) çağrılırsa o test başarısız sayılır, öbür testler sürer.
- *Test:* Go: `TestExitInATestFailsOnlyThatTest` (interp ve testrunner).

**1.13** Bir `TR_defer` bloğunun içinden çağrılan `TR_exit(kod)` süren çıkışın kodunu değiştirir; kalan ertelenen işler yine çalışır, program askıda kalmaz. Durdurmayı üstlenmemiş programa istek geldiyse kod değişmez, süreç isteğin sinyaliyle biter.
- *Test:* Go: `TestExitSkipsCatchRunsDefersAndLastCallWins`, `TestUnhandledStopKeepsTheSignal`.

**1.14** Program biterken (sona gelince, `TR_exit` ile, komut satırı çözümleyicisinin yardım ya da kullanım hatası çıkışında) bir ertelenen iş hata verirse hata yazılır ve çıkış kodu 1 olur. Durdurmayı üstlenmemiş programa istek geldiyse hata yine yazılır, süreç isteğin sinyaliyle biter.
- *Test:* Go: `TestDeferredAndTaskErrorsMakeTheExitOne`.

**1.15** Program biterken gelen ilk Ctrl+C ertelenen işleri kesmez; işler bitince durdurmayı üstlenmemiş programın süreci SIGINT ile, üstlenen program 1.9'daki koduyla biter. İkinci Ctrl+C 1.7'deki gibi hemen çıkarır.
- *Test:* Go: `TestFirstInterruptDoesNotCutTheOutermostDefers`.
