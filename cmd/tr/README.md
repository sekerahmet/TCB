# cmd/tr

`tr` komutunun giriş noktası: komutları ayrıştırır (çalıştır, kontrol, test, biçimle, derle, profil, izle, dizin ...) ve her birini ilgili paketin çekirdeğine bağlar. Uçtan uca komut testleri `e2e/` klasöründedir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_repo_tr_files_test.go`: depodaki bütün .tr dosyalarının tanımsız ad denetiminden geçmesi

## Standartlar

**`tr dizin` alt komutları yalnızca dizinin altındadır.** `tr dizin tanım|kullananlar|sınayanlar|etki ad` ve `tr dizin yeniden_adlandır ad yeni` (listeler, hiçbir dosyayı değiştirmez) çalışır; `tr tanım` bir komut değildir (dosya adı sanılır). Çıkış kodu sonucun durumudur: 0 tamam, 3 belirsiz ad (adaylar listelenir), 4 bulunamayan ad (yakın adlar önerilir), 1 kullanım hatası. Çıktı biçimleri ve şema: [`internal/devtools/codeindex`](../../internal/devtools/codeindex/README.md). Denetim: `TestIndexCommandAndSubcommands`, `TestIndexSubcommandsInEveryLanguage`, `TestIndexExitCodesAndJSON`, `TestIndexUsageErrors`, `TestIndexRootDiscovery`, `TestIndexRename` (cmd/tr/e2e).
