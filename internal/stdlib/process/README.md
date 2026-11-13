# process

Programın içinde çalıştığı süreçle ilgili TR yerleşikleri: ortam değişkenleri, betiğin komut
satırı seçenekleri (`TR_fn_command_line`) ve dış programlar (`TR_fn_run_process`, `TR_fn_spawn_process`, bölüm 21).

## Standartlar

**Gizli değer gizli kalır.** Adı gizli bir şey söyleyen ortam değişkenlerinin ve seçeneklerin
(anahtar, belirteç, parola ...) değerleri, insanın göreceği her çıktıda `***` olarak görünür.
Adı ne olursa olsun, ortamın ve komut satırının her değerindeki adresin kimlik kısmı da gizlidir (19.4):
`Register` ortamı, `interp.VM.SetArgs` argümanları program başlarken tarar; program değişkeni okumasa da
dış programın standart hatasında örtülür. `TR_env:` ile verilen değer de taranır (`programEnv`). Kuralın
kendisi tek yerde, `interp`'tedir (`address_secret.go`). Kullanım hatasındaki değer önce maskelenir, sonra
kesilir (`errorValue`, H7207): kesim parolanın ortasına düşse de başı kalmaz.
Denetim: `TestIsSecretName`, `TestCommandLineSecretValue`, `TestCommandLineSecretLeaks`, `TestEnvVar`,
`TestEnvironmentAddressIsMaskedAtStart`, `TestUnreadAddressIsMaskedInChildStderr`, `TestEnvArgumentAddressIsMasked`,
`TestCommandLineAddressInUsageError`; `interp`'te `TestAddressSecrets`; ayrı süreçte `cmd/trlang/e2e`
`TestSecretValuesMaskedInOutput`, `TestAddressUserinfoMaskedInOutput`.

**Parça parça metnin maskesi akıştadır.** Maske her çıktıya ayrı uygulanır, iki parçaya bölünen gizli
değeri görmez. `TR_fn_mask_stream()`'in değeri (`mask_stream.go`) parçaları `interp.MaskStream`'e verir:
algoritma gizli değer listesinin yanında, `interp`'tedir (19.4a). Bittikten sonra `.TR_step` H7634'tür,
ikinci `.TR_finish()` boş metin verir.
Denetim: `TestMaskStreamFollowsTheBatchMask`, `FuzzMaskStream` (interp), `tests/surec_ve_kayit_test.tr`'nin 19.4a testi.

**Tek yazım.** Komut satırı seçeneği tek biçimde yazılır (`--ad değer` ya da `--ad=değer`);
konumlu argüman ya da başka yazımlar kabul edilmez, en yakın doğru ad önerilir.
Denetim: `TestCommandLineUsageErrors`, `TestParseErrorCodes`.

**Sayılar dilin yazımıyla okunur.** Komut satırındaki sayılar TR'nin sayı yazımına uyar;
kısmen sayı olan bir değer sessizce kırpılmaz, hatadır.
Denetim: `TestNumberParsing`, `TestIsNumberOrBool`.

**Dış program kabuksuz çalışır.** Her liste öğesi programa tek argüman olarak, değiştirilmeden gider;
çıktı bayt bayt gelir, beklerken kilit bırakılır. Testler `sh` ve `cmd` varsaymaz: test ikilisi kendi
kendinin dış programıdır (`helperMain`), her işletim sisteminde aynı program koşar.
Denetim: `TestRunProcessArgumentsAreUnchanged`, `TestRunProcessOutputIsBytes`, `TestRunProcessInput`, `TestRunProcessReleasesTheLock`.

**Program yalnız izin verilen yerden bulunur.** Ad, `PATH`'in mutlak girdilerinde aranır; çalışma klasörü ve göreli girdi aranmaz, yol mutlak yapılır. `exec.LookPath` ve `exec.Command` kullanılmaz: ikisi de bu sınırları tutmuyor (DENETİM 217). Windows'ta son yol izin listesiyle sınanır, `.bat` ve `.cmd` hiçbir yazımla çalışmaz.
Denetim: `TestProgramNameIsSearchedInPath`, `TestRelativePathEntriesAreNotSearched`, `TestProgramPathIsRelativeToTheWorkingDirectory`, `TestWindowsBatchFilesAreRefused`.

**Başarısız bitiş iletide okunur, gizli değer görünmez.** Kabul edilmeyen çıkış kodu çalışma hatasıdır: ad, kod, standart hatanın sonu. Standart hata sınırlı tamponla okunur (`stderrTailLimit`); tampon sınır artı en uzun gizli değer kadar tutar, önce süzülür sonra kesilir, yazdırılamayan bayt iletiye girmez.
Denetim: `TestExitCodeIsARunError`, `TestStderrTailMasksBeforeCutting`, `TestStderrTailPrintable`, `TestBothOutputsDoNotDeadlock`.

**Dış programa gizli değer kendiliğinden gitmez.** Gizli adlı değişkenler ortamdan süzülür, yalnız sayı ya da mantık olan değer gider (`MAX_TOKENS=4096`, 19.4); `TR_env:` ekler ya da değiştirir, gizli adlı değeri süzgece verir. Başarısız bitişte ileti süzülen adları söyler, değerlerini söylemez.
Denetim: `TestSecretNamedVariablesAreNotPassed`, `TestExplicitSecretReachesTheProgramAndIsMasked`, `TestFailureNamesTheFilteredVariables`, `TestEnvNamesIgnoreCaseOnWindows`.

**Durdurma tek yoldur, kilit tutmaz.** Süre sınırı, iptal ve durdurma programı aynı yolla durdurur (`job.stop`): nazik istek, 2 saniye, öldürme; istek ve öldürme başlattığı süreçlere de gider (Unix'te süreç grubu, Windows'ta `KILL_ON_JOB_CLOSE` iş nesnesi ve Ctrl+Break). Durdurma `interp.AwaitExternal`'ın kilit bırakılmış bölümündedir: yığın ancak program durunca çözülür.
Denetim: `TestStopUnwindsAfterTheProgramStopped`, `TestGrandchildStopsWithTheProgram`, `TestStoppingDoesNotHoldTheLock`, `TestHandledStopLetsTheCallFinish`, `TestNoConsoleUsesTheKillPath`.

**Geride yetim süreç kalmaz.** Normal bitişte görevdeki çağrı beklenir; hata, `TR_exit` ve durdurma görevleri durdurur, çağrı programı durdurur. Hemen bitişte (1.7) `State`'in kaydındaki gruplar öldürülür (`interp.VM.HardStop`); kayıt VM başınadır.
Denetim: `TestHardStopReachesOnlyItsOwnProgram`; ayrı süreçte `cmd/trlang/e2e` `TestSecondRequestKillsTheExternalProgram`, `TestUncaughtErrorStopsTheExternalProgram`, `TestExitStopsTheExternalProgram`, `TestProgramEndWaitsForTheExternalProgram`, `TestExitInATestLeavesTheExternalProgram`.

**Kullanım hatası temiz çıkar.** Yanlış bir seçenek ya da `--help` programı açıklayıcı bir
iletiyle ve tanımlı bir çıkış koduyla bitirir; çökme ya da yığın dökümü olmaz.
Denetim: `TestCommandLineHelp`, `TestCommandLineUsageErrors`, `TestExitRequestNotCaught`; ayrı süreçte `cmd/trlang/e2e` `TestCommandLineOptions`.

**Akışlı çıktıda bayt kaybolmaz.** Çıktıyı kilit dışında tek bir okuyucu alır; `TR_buffer_bytes:` yalnız dolunca ne olacağını seçer: program bekler ya da durdurulur ve hata kurulur. Sessizce veri atılmaz.
Denetim: `TestBackpressureKeepsEveryByte`, `TestOutputOutlivesTheProgram`, `TestOverflowStopsTheProgramAndSticks`, `TestPendingReadOfTheBufferSizeIsNoOverflow`.

## Testler

- `behavior_run_process_stop_test.go`: `TR_run_process`'in durdurulması (21.5) ve torun süreçler.
- `behavior_run_process_exit_test.go`: `TR_run_process`'in çıkış kodu ve standart hata iletisi (21.4).
- `behavior_spawn_process_test.go`: `TR_spawn_process`, `TR_Process` ve okuyucu (21.2).
- `behavior_helper_unix_test.go`: test ikilisinin dış program olarak Unix'te yaptığı iş.
- `behavior_helper_windows_test.go`: aynı iş, Windows'ta.
