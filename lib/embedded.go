// Package lib, TR ile yazılan standart kitaplığın (katmanlar, iyileştiriciler, ileride
// hazır eğitim döngüsü ve TR ile yazılmış çekirdekler) Go tarafıdır. Kitaplığın kendisi bu
// klasördeki .tr dosyalarıdır; bu tek Go dosyası onları go:embed ile ikiliye gömer. go:embed
// yalnızca paketin kendi klasörünü ve alt klasörlerini gömebildiği için kitaplık bu paketin
// altında durur.
//
// Kullanıcı kodunda bir modül `aktar "tr/katman"` ile alınır (dil tanımı 11.6): "tr/" öneki
// dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez. Komut satırı
// (cmd/tr) Files'ı yorumlayıcıya interp.SetLibrary ile bağlar; `tr derle` çıktısı
// yorumlayıcının kopyası olduğundan kitaplık ona da girer. Testler (*_test.tr) de gömülür ama
// modül olarak aktarılamaz; `tr test lib` ile koşar.
//
// Layer: TR library (lib); hiçbir iç paketi içe aktarmaz (internal/layers_test.go).
package lib

import "embed"

// Files kitaplığın gömülü dosyalarıdır (klasöre göre yollarla: "katman.tr").
//
//go:embed README.md *.tr
var Files embed.FS
