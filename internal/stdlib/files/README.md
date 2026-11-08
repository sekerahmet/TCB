# files

TR'nin dosya ve klasör yerleşikleri: `TR_type_Path(...)` değeri, satır okuyucu (`TR_fn_read_lines`),
listeleme ve kalıplar, kopyalama, taşıma, silme ve geçici klasörler.

## Standartlar

**Yollar dilin kuralıyla çözülür (18.16).** Okuyan ve soran işlerde göreli bir yol önce çalışma
klasöründe, yoksa kodun modülünün ve programın klasöründe aranır; değiştiren işler (`TR_make_dir`,
`TR_remove`, `TR_move`'un kaynağı ve hedefi, `TR_copy`'nin hedefi) yalnız çalışma klasörüne bakar ve
başka klasördeki aynı adlı dosyaya dokunmaz.
Denetim: `TestUserPathsResolvedByLanguageRule` (`internal/stdlib`; yorumlayıcı alan her işlev yolu `VM.ResolvePath` ya da `VM.ResolveWritePath` ile çözer), `TestPathTakingBuiltins`, `TestWritersResolveOnlyInTheWorkingDirectory` (`internal/lang/interp`).

**Silme korumalıdır.** Kök, ev ve sistem klasörleri, çalışma klasörü ve programın klasörü
silinmez; dolu bir klasör yalnızca açıkça istenince silinir. Bağlantılar izlenmez: silme,
kopyalama ve listeleme bir bağlantının gösterdiği yere dokunmaz ve klasörün dışına çıkmaz.
Denetim: `TestDeleteProtections`, `TestSystemDirProtection`, `TestDeleteDoesNotFollowLinks`, `TestDeleteWindowsJunction`, `TestListDoesNotEnterLinks`, `TestCopyDir`, `TestTempDirDoesNotFollowLinks`.

**Yarıda kalan iş hedefi bozmaz.** Kopya önce geçici bir ada yazılır, doğrulanınca yerine
konur; başka diske taşımada kaynak ancak kopya doğrulandıktan sonra silinir.
Denetim: `TestCopyFile`, `TestMoveToOtherDisk`, `TestOverwriteDirWithReadOnlyFile`.

**Açık dosya yol gösterir.** Açık bir okuyucunun ya da günlüğün (`TR_Logger`, kaydı `LogOpened`)
dosyası silinmek ya da taşınmak istenirse işlem başlamadan, ne yapılacağını söyleyen bir hata
verilir (18.7).
Denetim: `TestDeleteOpenReader`, `TestMoveOpenReader`, `TestLineReaderClose`.

**Okuyucu sızmaz.** Satır okuyucular bitince, kapatılınca, erişilemez olunca ya da program
bitince kapanır; çok uzun satırlar belleği tüketmeden hatayla durur.
Denetim: `TestLineReaderClose`, `TestLineReaderClosesAtEOFWithoutNewline`, `TestLineReaderClosedWhenUnreachable`, `TestTempDirDeletedByProgram`, `TestFileLinesGzipAndLongLine`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_write_paths_test.go`: değiştiren işlerin (`TR_remove`, `TR_move`, `TR_copy`'nin hedefi, `TR_write_file`, `TR_save_tensors`, `TR_make_dir`) göreli yolu yalnız çalışma klasörüne göre çözmesi, programın klasöründeki aynı adlı dosyaya dokunmaması; H2630'un verdiği açık yolun çalışması
