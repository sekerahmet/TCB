# cmd/tr

`tr` komutunun giriş noktası: komutları ayrıştırır (çalıştır, kontrol, test, biçimle, derle, profil, izle ...) ve her birini ilgili paketin çekirdeğine bağlar. Uçtan uca komut testleri `e2e/` klasöründedir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_repo_tr_files_test.go`: depodaki bütün .tr dosyalarının tanımsız ad denetiminden geçmesi
