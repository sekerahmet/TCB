# detmath

Tam yuvarlanmayan 18 matematik fonksiyonunun her makinede (x86, ARM64; Windows, Linux) aynı biti veren karşılığı: `Exp`, `Log`, `Log2`, `Log10`, `Log1p`, `Expm1`, `Sin`, `Cos`, `Tan`, `Asin`, `Acos`, `Atan`, `Atan2`, `Sinh`, `Cosh`, `Tanh`, `Pow`, `Erfc`. Dil tanımının sözü 13.1 ve 4.3'tedir.

## Kurallar

- Kod Go'nun `math` paketinin saf Go kaynağıdır. Dosyaların telif başlıkları (Go, FreeBSD, Sun, Cephes) aynen kalır; depo herkese açıktır.
- Her ondalık çarpım açık `float64()` dönüşümüyle yuvarlanır: dönüşüm çarpmanın toplamayla birleşmesini engeller. Yeni kodda da böyledir.
- Tam yuvarlanan fonksiyonlar (`Sqrt`, `Floor`, `Ceil`, `Abs`, `Mod`, `Frexp`, `Ldexp`) `math`'tan kullanılır.

Denetim: `TestNoFusedInstructions` (`GOAMD64=v3` ve `GOOS=linux GOARCH=arm64` derlemesinde birleşik komut sayısı 0), `TestGoldenBits`, `TestGoldenBitsOnV3`, `TestSameBitsAsGoWithoutFusion`.
