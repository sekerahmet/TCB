# reporoot

Testlerin depo kökünü bulmasına yarayan küçük yardımcı. Kök, çalışma klasöründen yukarı doğru `go.mod` dosyasının bulunduğu ilk klasördür.

## Kurallar

- Testler depodaki dosyalara (`docs/`, `tests/`, `examples/`, test verileri) `../../` gibi göreli yollarla değil bu yardımcıyla ulaşır; paket taşınınca testler kırılmaz.
- Yalnızca testler kullanır; hiçbir iç paketi içe aktarmaz, her katmanın testi kullanabilir.
