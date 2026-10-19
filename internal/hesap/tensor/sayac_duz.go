//go:build !race

package tensor

// Profil sayaçlarının bellek erişimleri, yarış denetimsiz derleme: düz okuma ve yazma
// (bkz. sayac.go, "Eşzamanlılık"). Yarış denetimli eşi sayac_yaris.go'dadır.

func ekle64(p *int64, d int64)       { *p += d }
func oku64(p *int64) int64           { return *p }
func ekle32(p *int32, d int32) int32 { *p += d; return *p }
func oku32i(p *int32) int32          { return *p }
func yaz32i(p *int32, v int32)       { *p = v }
