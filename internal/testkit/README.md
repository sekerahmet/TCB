# testkit

Birden çok paketin testlerinde aynı yazılan küçük yardımcıların tek yeri: `WriteFile`/`WritePath` (dosya yazma), `MustContain` (çıktıda parça arama), `TRSeeds` (bulanıklaştırma tohumları), `GlobFiles` (depo köküne göre dosya kalıbı), `HasJSONName` (yapı etiketi denetimi), `RunBinary`/`Wait` (derlenmiş `tr` ikilisini çalıştırıp sonucunu denetleme). `trrun/` altpaketi TR programı çalıştıran yardımcılardır (`Parse`, `Exec`, `MustExec`, `MustExecChecked`).

## Kurallar

- Yalnızca testler (`_test.go`) içe aktarır; üretim koduna sızmaz. Denetim: `TestHelpersAreImportedOnlyByTests`.
- `testkit` yalnızca `reporoot`'u bilir; `trrun` yalnızca dil çekirdeğini (`internal/lang/`) bilir. Bu yüzden dil çekirdeğinin kendi testleri `trrun`'u kullanamaz (içe aktarma döngüsü); motor, araç ve komut testleri kullanır. Denetim: `TestHelpersAreImportedOnlyByTests`, `internal/layers_test.go`.
- Paketler arası gerçek farklar (tip denetimi, VM kurulumu, günlük/denetim noktası yazıcıları, motor kaydı) paketin kendi ince sarmalayıcısında kalır; ortak çekirdek burada durur.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`): `testkit_test.go`, `trrun/trrun_test.go`.
