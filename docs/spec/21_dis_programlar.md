# 21. Dış programlar

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "21. Dış programlar".

Her kuralın emsali ve gerekçesi: [`rationale/21_processes.md`](rationale/21_processes.md).

Bir TR programı bazı işleri başka bir programa yaptırır: `ffmpeg` sesi ve videoyu çözer, `arecord` mikrofonu okur, `aplay` hoparlöre çalar, bir ajan bir aracı çalıştırır. Bu bölüm dış programın nasıl başlatıldığını, girdisinin ve çıktısının nasıl taşındığını, nasıl durdurulduğunu ve hatasının nasıl görüldüğünü söyler.

**21.1** `TR_run_process(program, argümanlar)` bir dış programı çalıştırır, bitmesini bekler ve standart çıktısını `TR_Bytes` olarak verir.
- `program` aranacak bir ad (`TR_String`, 21.3) ya da bir dosya yoludur (`TR_Path`). `argümanlar` bir `TR_List[TR_String]`'dir; her öğe programa tek bir argüman olarak, değiştirilmeden verilir. Yol argümanı `TR_String(yol)` ile yazılır (18.1); listeye `TR_Path` koymak denetim hatasıdır ve ileti `TR_String(yol)`'u önerir.
- **Kabuk yoktur:** `|`, `>`, `*`, `$`, `~` ve boşluk argümanın sıradan karakterleridir. Boru hattı iki süreçle (21.2) yazılır; kabuk gerçekten gerekiyorsa açıkça çağrılır: `TR_run_process("sh", ["-c", komut])`.
- Adlı argümanlar:
  - `TR_input:` bir `TR_Bytes`'tır: programın standart girdisi. Verilmezse girdi boş aygıttır; girdiden okuyan program hemen dosya sonu alır, beklemede kalmaz.
  - `TR_env:` için 21.6, `TR_timeout:` için 21.5, `TR_exit_codes:` için 21.4.
- Bütün çıktı belleğe alınır. Büyük ya da bitmeyen çıktı `TR_spawn_process` ile parça parça okunur.
- Beklerken kilit bırakılır (10.3).

```tr
# TR 4
TR_var ham = TR_run_process("ffmpeg", ["-nostdin", "-i", "soru.mp3", "-f", "s16le", "-ac", "1", "-ar", "16000", "-"])
TR_var ses = TR_Tensor[TR_Float](TR_from_bytes[TR_Int](ham, TR_dtype: "int16", TR_byte_order: "little")) / 32768.0
```

- *Test:* TR testi (olumsuz fikstür `tests/negative/program_argumani_yol.tr`); Go: `TestRunProcessOutputIsBytes`, `TestRunProcessArgumentsAreUnchanged`, `TestRunProcessInput`, `TestRunProcessArgumentListErrors`, `TestRunProcessPathSuggestionRuns`, `TestRunProcessCallErrors`, `TestRunProcessReleasesTheLock`, `TestProcessArgumentListHoldsNoPath`.

**21.2** `TR_spawn_process(program, argümanlar)` programı başlatır ve beklemeden bir `TR_Process` verir. Programın standart girdisi ve standart çıktısı TR programına bağlıdır. `program` ve `argümanlar` 21.1'deki gibidir; adlı argümanlar `TR_env:` ve `TR_exit_codes:`'tır.
- `p.TR_read(n)` tam `n` bayt okur ve `TR_Bytes` verir. Program çıktısını kapattıysa (dosya sonu) kalanı verir, kalan da yoksa boş `TR_Bytes` verir. Okuma `n` bayt gelene kadar bekler, yani `n` gecikmedir: 16 kHz tek kanallı "int16" seste 1024 bayt 32 ms'dir.
- `p.TR_write(b)` bir `TR_Bytes`'ın hepsini programın girdisine yazar; program okumuyorsa bekler. `p.TR_close_stdin()` girdiyi kapatır, program dosya sonu alır.
- `p.TR_wait()` girdiyi kapatır, programın bitmesini bekler ve çıkış kodunu `TR_Int` olarak verir (21.4). `p.TR_wait(TR_timeout: saniye)` 10.7'deki gibidir. `p.TR_done()` programın bitip bitmediğini söyler, beklemez.
- `p.TR_cancel()` programı 21.5'teki gibi durdurur; bitmiş programda etkisi yoktur.
- `TR_read`, `TR_write` ve `TR_wait` beklerken kilidi bırakır (10.3). İptal, durdurma ve süre sınırı bu beklemeleri de bitirir (21.5).
- Program biterken (hatayla ya da durdurmayla da) hâlâ süren bütün süreçler 21.5'teki gibi durdurulur: geride yetim süreç kalmaz (TR'nin kendisi öldürülür ya da çökerse 21.5'in sınırı geçerlidir). Hiç beklenmemiş bir süreç kabul edilmeyen bir kodla kendiliğinden bittiyse hatası program biterken yazılır ve program 1 koduyla biter (10.6 gibi). TR'nin durdurduğu sürecin durması hata olarak yazılmaz (21.4).

```tr
# TR 4
TR_var mikrofon = TR_spawn_process("arecord", ["-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw"])
TR_defer { mikrofon.TR_cancel() }
TR_for {
    TR_var parça = mikrofon.TR_read(1024) # 512 örnek: 32 ms
    TR_if parça.TR_length < 1024 { TR_break }
    TR_var örnekler = TR_Tensor[TR_Float](TR_from_bytes[TR_Int](parça, TR_dtype: "int16", TR_byte_order: "little")) / 32768.0
    TR_print(örnekler.TR_shape)
}
```

- *Test:* gerekmez, henüz uygulanmadı.

**21.3** Ad olarak verilen program `PATH`'te aranır.
- Ad bir yol ayırıcısı (`/` ya da `\`) içeremez. Dosyanın kendisi bir `TR_Path` ile verilir; göreli yol çalışma klasörüne göredir.
- `PATH`'teki göreli girdiler ve çalışma klasörü aranmaz: indirilen bir veri klasöründeki aynı adlı dosya çalışmaz.
- Windows'ta yalnız `.exe` ve `.com` dosyaları aranır. Ad `.exe`, `.com`, `.bat` ya da `.cmd` ile bitemez: `"ffmpeg.exe"` hatadır ve ileti `"ffmpeg"`'i önerir, böylece aynı kod her işletim sisteminde çalışır. Başka noktalı adlar (`"python3.11"`) program adıdır.
- Windows'ta `.bat` ve `.cmd` dosyaları ad ya da yol olarak çalıştırılamaz: bunların argümanları güvenle kaçırılamaz. İleti açık yazımı verir (`TR_run_process("cmd", ["/c", "npm.cmd", ...])`) ve bu yolda argümanların `cmd.exe`'nin kurallarıyla yorumlandığını, dışarıdan gelen bir argümanın böyle verilmemesi gerektiğini söyler.
- Bulunamayan programın iletisi adı söyler. Ad boşluk içeriyorsa ileti argümanların listeye ayrılmasını önerir: `TR_run_process("ffmpeg", ["-i", "a.wav"])`.
- *Test:* Go: `TestProgramNameIsSearchedInPath`, `TestProgramNameHoldsNoSeparator`, `TestProgramPathIsRelativeToTheWorkingDirectory`, `TestRelativePathEntriesAreNotSearched`, `TestProgramNameCannotEndInAnExtension`, `TestDottedProgramNameIsAName`, `TestWindowsRunsOnlyExeAndCom`, `TestWindowsBatchFilesAreRefused`, `TestBatchSuggestionRuns`, `TestMissingProgramMessages`.

**21.4** Programın çıkış kodu `TR_exit_codes:`'ta değilse (varsayılan `[0]`) çağrı çalışma hatasıyla biter. `TR_spawn_process`'te hata `p.TR_wait()`'ten gelir.
- İleti programın adını, çıkış kodunu ve standart hata çıktısının sonunu taşır. Gizli değerler orada da 19.4'teki gibi görünmez.
- Bir sinyalle ölen program da hatadır; ileti sinyali söyler.
- TR'nin kendisinin durdurduğu programın çıkışı çıkış hatası değildir:
  - iptalde iptal hatası (10.9), süre sınırında zaman aşımı hatası (10.7) gelir;
  - program biterken ve durdurmada (1.5) durdurulan süreç hata olarak yazılmaz, 10.6'da iptal edilen görev gibi.
- Standart hata çıktısı arka planda okunur, iki çıktı birbirini kilitlemez. Program başarıyla biterse standart hata çıktısı atılır.
- Bazı programlar başarıyı başka kodlarla bildirir: `grep` eşleşme bulamayınca 1 döner. Kabul edilen kodlar açıkça yazılır: `TR_exit_codes: [0, 1]`. Hangi kodla bittiğini bilmesi gereken program kabul ettiği kodları `TR_exit_codes:`'a yazar ve hangisi olduğunu `p.TR_wait()`'in sonucundan alır: `TR_spawn_process("grep", ["-q", "hata", "günlük.txt"], TR_exit_codes: [0, 1]).TR_wait() == 0`.
- *Test:* Go: `TestExitCodeIsARunError`, `TestAcceptedExitCodes`, `TestExitCodesAreChecked`, `TestStderrInTheMessageIsMasked`, `TestStderrInTheMessageIsPrintable`, `TestKilledBySignal`, `TestBothOutputsDoNotDeadlock`, `TestStderrTailIsBounded`, `TestStderrTailMasksBeforeCutting`, `TestStderrTailPrintable`, `TestStderrTailKeepsWholeRunes`.

**21.5** İptal (10.9), süre sınırı (10.7), `p.TR_cancel()` ve durdurma (1.5) programı aynı yolla durdurur:
- Önce nazik bir istek gider: Unix'te SIGTERM, Windows'ta Ctrl+Break olayı.
- Program 2 saniye içinde durmazsa öldürülür.
- İstek ve öldürme programın başlattığı süreçlere de gider: program kendi süreç grubunda başlar (Windows'ta bir iş nesnesinde). Kendini gruptan bilerek ayıran bir torun süreç bunun dışında kalır.
- Uçbirimde basılan Ctrl+C önce TR programına gelir, dış programa gelmez. TR programı durursa dış program da bu kurala göre durur (1.5). Durdurmayı üstlenen programda (1.6) dış program, TR programı durana kadar sürer. 1.6'nın sınıflarıyla: `p.TR_read` yeni bir şey bekler, istekten sonra "durdurma istendi" hatasıyla döner; `p.TR_write`, `p.TR_wait` ve `TR_run_process` süren bir işi bitirir, sürer.
- TR hemen biterken (1.7) önce süreç gruplarını öldürür. TR'nin kendisi öldürülür ya da çökerse Unix'te gruptaki süreçler kalabilir; Windows'ta iş nesnesi onları da kapatır.
- Bekleyen `TR_read`, `TR_write` ve `TR_wait` program durunca hemen döner: iptalde iptal hatasıyla, süre sınırında zaman aşımı hatasıyla.
- *Test:* Go: `TestTimeoutStopsTheProgram`, `TestTimeoutIsCheckedLikeTaskWait`, `TestProgramEndingInTimeIsNotATimeout`, `TestNiceRequestComesFirst`, `TestStubbornProgramIsKilled`, `TestNoConsoleUsesTheKillPath`, `TestGrandchildStopsWithTheProgram`, `TestStopUnwindsAfterTheProgramStopped`, `TestHandledStopLetsTheCallFinish`, `TestDeferralExpiringDuringTheCallStopsIt`, `TestCancelStopsTheCallEvenWhenStopIsHandled`, `TestCallInADeferIsNotAbandoned`, `TestTimeoutWorksInADefer`, `TestStoppingDoesNotHoldTheLock`, `TestGrandchildHoldingThePipeDoesNotBlock`, `TestEscapedGrandchildDoesNotBlockTheStop`, `TestHardStopReachesOnlyItsOwnProgram`, `TestStopRequestStopsTheExternalProgram`, `TestSecondRequestKillsTheExternalProgram`, `TestUncaughtErrorStopsTheExternalProgram`, `TestExitStopsTheExternalProgram`, `TestProgramEndWaitsForTheExternalProgram`, `TestExitInATestLeavesTheExternalProgram`.

**21.6** Dış program TR programının ortamını alır, ama 19.4'e göre gizli adlı değişkenler aktarılmaz.
- `TR_env:` bir `TR_Dict[TR_String, TR_String]`'dir. Ortama değişken ekler ya da var olanın değerini değiştirir, ortamın geri kalanını silmez.
- Gizli bir değer gerekiyorsa açıkça verilir: `TR_env: {"HF_TOKEN": anahtar}`.
- Program başarısız biterse ve ortamdan değişken süzüldüyse ileti süzülen adları (değerlerini değil) ve `TR_env:` yazımını söyler.
- *Test:* Go: `TestSecretNamedVariablesAreNotPassed`, `TestSecretFilterLooksAtNameAndValue`, `TestEnvArgumentAddsAndReplaces`, `TestExplicitSecretReachesTheProgramAndIsMasked`, `TestFailureNamesTheFilteredVariables`, `TestEnvArgumentIsChecked`, `TestEnvNamesIgnoreCaseOnWindows`.

**21.7** Dış programın çıktısı bayttır. Metin olarak okumak `TR_String(b)` iledir (UTF-8, 3.3a); UTF-8 olmayan çıktı orada çalışma hatasıdır, sessizce bozulmaz.
- *Test:* Go: `TestRunProcessOutputIsBytes`.

**21.8** TR argümanları ve ortamı 21.1 ve 21.6'da yazılanın dışında değiştirmez. Dış programın sonucu ise kendi sürümüne bağlıdır: aynı sonuç ancak aynı sürümle alınır.
- `-` ile başlayan bir dosya adı programın kendi kuralıyla yazılır: `["--", ad]` ya da `["./" + ad]`. TR argümanı kendisi değiştirmez.
- Kontrol noktası (20) dış süreci kaydetmez: kaldığı yerden süren program dış programı yeniden başlatır.
- *Test:* Go: `TestRunProcessArgumentsAreUnchanged`.
