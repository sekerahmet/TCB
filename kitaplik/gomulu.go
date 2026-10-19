// Package kitaplik, TR ile yazılan standart kitaplığın (katmanlar, iyileştiriciler, ileride
// hazır eğitim döngüsü ve TR ile yazılmış çekirdekler) Go tarafıdır. Kitaplığın kendisi bu
// klasördeki .tr dosyalarıdır; bu tek Go dosyası onları go:embed ile ikiliye gömer. go:embed
// yalnızca paketin kendi klasörünü ve alt klasörlerini gömebildiği için kitaplık bu paketin
// altında durur.
//
// Kullanıcı kodunda bir modül `aktar "tr/katman"` ile alınır (dil tanımı 11.6): "tr/" öneki
// dilin kendi kitaplığına ayrılmıştır, diskteki dosyalar onu gölgeleyemez. Komut satırı
// (cmd/tr) Dosyalar'ı yorumlayıcıya yurut.KitaplikBagla ile bağlar; `tr derle` çıktısı
// yorumlayıcının kopyası olduğundan kitaplık ona da girer. Testler (*_test.tr) de gömülür ama
// modül olarak aktarılamaz; `tr test kitaplik` ile koşar.
//
// Katman: TR kitaplığı (kitaplik); hiçbir iç paketi içe aktarmaz (internal/katman_test.go).
package kitaplik

import "embed"

// Dosyalar kitaplığın gömülü dosyalarıdır (klasöre göre yollarla: "katman.tr").
//
//go:embed README.md *.tr
var Dosyalar embed.FS
