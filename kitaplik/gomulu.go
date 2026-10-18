// Package kitaplik, TR ile yazılan standart kitaplığın (katmanlar, hazır modeller, eğitim
// döngüsü, ileride TR ile yazılmış çekirdekler) Go tarafıdır. Kitaplığın kendisi bu klasördeki
// .tr dosyalarıdır; bu tek Go dosyası onları go:embed ile ikiliye gömer. go:embed yalnızca
// paketin kendi klasörünü ve alt klasörlerini gömebildiği için kitaplık bu paketin altında
// durur.
//
// Bugün kitaplık boştur ve paketi hiçbir program içe aktarmaz. İlk .tr dosyası eklendiğinde
// gömme deseni genişletilir (ör. //go:embed README.md katman model egitim) ve kitaplığın
// testleri `tr test kitaplik` ile koşar. Kullanıcı kodunda hangi adla içe aktarılacağı
// (dil tanımı §11, modüller) kitaplık işi başlamadan kararlaştırılır.
//
// Katman kuralı (internal/katman_test.go): bu paket hiçbir iç paketi içe aktarmaz.
package kitaplik

import "embed"

// Dosyalar kitaplığın gömülü dosyalarıdır (klasöre göre yollarla).
//
//go:embed README.md
var Dosyalar embed.FS
