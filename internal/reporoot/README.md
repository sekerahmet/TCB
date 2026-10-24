# reporoot

Testlerin depo kökünü bulmasına yarayan küçük yardımcı. Kök, çalışma klasöründen yukarı doğru `go.mod` dosyasının bulunduğu ilk klasördür.

## Kurallar

- Testler depodaki dosyalara (`docs/`, `tests/`, `examples/`, test verileri) `../../` gibi göreli yollarla değil bu yardımcıyla ulaşır; paket taşınınca testler kırılmaz. Denetim: `TestTestsUseRepoRootNotRelativePaths`.
- Yalnızca testler kullanır; hiçbir iç paketi içe aktarmaz, her katmanın testi kullanabilir. Denetim: `internal/layers_test.go` (`TestLayerRule`).
- Testler klasör gezmeyi `reporoot.WalkDir` ile yapar (`filepath.WalkDir`'in imzası): depo içindeki yolda yalnızca git'in izlediği dosyalar ve onları içeren klasörler gezilir, depo testleri geliştiricinin izlenmeyen ve yok sayılan dosyalarına (`scratch/`, kişisel denemeler) bağlı olmaz; depo dışındaki yol (`t.TempDir`) ve git'siz kopya olduğu gibi gezilir. Kod grafiği ve `tr dizin`'in depo testleri aynı kuralı `codeindex.ListFiles(..., TrackedFiles, ...)` ile izler (devtools katmanı bu yardımcıyı içe aktaramaz). Denetim: `TestWalkDirSeesOnlyTrackedFiles`, `TestTestsWalkThroughWalkDir` (bağımsız denetim grup 6).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_usage_test.go`: depodaki testlerin göreli yol kullanmadığı, yardımcının yalnızca testlerden içe aktarıldığı
