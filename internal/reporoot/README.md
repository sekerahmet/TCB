# reporoot

Testlerin depo kökünü bulmasına yarayan küçük yardımcı. Kök, çalışma klasöründen yukarı doğru `go.mod` dosyasının bulunduğu ilk klasördür.

## Kurallar

- Testler depodaki dosyalara (`docs/`, `tests/`, `examples/`, test verileri) `../../` gibi göreli yollarla değil bu yardımcıyla ulaşır; paket taşınınca testler kırılmaz. Denetim: `TestTestsUseRepoRootNotRelativePaths`.
- Yalnızca testler kullanır; hiçbir iç paketi içe aktarmaz, her katmanın testi kullanabilir. Denetim: `internal/layers_test.go` (`TestLayerRule`).

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_usage_test.go`: depodaki testlerin göreli yol kullanmadığı, yardımcının yalnızca testlerden içe aktarıldığı
