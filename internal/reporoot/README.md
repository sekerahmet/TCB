# reporoot

Testlerin depo kökünü bulmasına yarayan küçük yardımcı. Kök, çalışma klasöründen yukarı doğru `go.mod` dosyasının bulunduğu ilk klasördür.

## Kurallar

- Testler depodaki dosyalara (`docs/`, `tests/`, `examples/`, test verileri) `../../` gibi göreli yollarla değil bu yardımcıyla ulaşır; paket taşınınca testler kırılmaz. Denetim: `TestTestsUseRepoRootNotRelativePaths`.
- Yalnızca testler kullanır; hiçbir iç paketi içe aktarmaz, her katmanın testi kullanabilir. Denetim: `internal/layers_test.go` (`TestLayerRule`).
- Testler ve ortak test yardımcıları (`internal/testkit`) dosya listelemeyi `reporoot` ile yapar: `WalkDir`, `Glob`, `ReadDir` (standart kitaplıktakilerin imzası). Depo içindeki yolda yalnızca git'in izlediği dosyalar ve onları içeren klasörler görünür; depo testleri geliştiricinin izlenmeyen ve yok sayılan dosyalarına (`scratch/`, kişisel denemeler) bağlı olmaz. Depo dışındaki yol (`t.TempDir`), git'siz kopya ve kökü bulunamayan çalışma klasörü olduğu gibi okunur. Depoda geride dosya bırakılmadığını arayan test `GlobAll`'u adıyla çağırır. Kod grafiği ve `tr dizin`'in depo testleri aynı kuralı `codeindex.ListFiles(..., TrackedFiles, ...)` ile izler (devtools katmanı bu yardımcıyı içe aktaramaz). Denetim: `TestWalkDirSeesOnlyTrackedFiles`, `TestTestsListFilesThroughReporoot` (bağımsız denetim grup 6 ve 7).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_usage_test.go`: depodaki testlerin göreli yol kullanmadığı, yardımcının yalnızca testlerden içe aktarıldığı
