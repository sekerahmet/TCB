// Package detmath, tam yuvarlanmayan matematik fonksiyonlarının her makinede aynı biti veren
// karşılığıdır (13.1, 4.3). Kod Go'nun math paketinin saf Go kaynağıdır, telif başlıklarıyla;
// tek fark her ondalık çarpımın açık float64() dönüşümüyle yuvarlanmasıdır: dönüşüm derleyicinin
// çarpmayı toplamayla birleştirmesini (FMA) engeller.
//
// Go'nun math paketi bu sözü vermez: amd64'te Exp çalışma anında işlemciye göre yol seçer, arm64
// assembly'si FMA'lıdır, derleyici de saf kodu arm64'te her zaman, amd64'te GOAMD64=v3'te birleştirir.
// Tam yuvarlanan fonksiyonlar (Sqrt, Floor, Ceil, Abs, Mod, Frexp, Ldexp) math'tan kullanılır.
//
// Layer: deterministic math (internal/detmath)
package detmath
