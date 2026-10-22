//go:build !race

package tensor

// Profil sayaçlarının bellek erişimleri, yarış denetimsiz derleme: düz okuma ve yazma
// (bkz. sayac.go, "Eşzamanlılık"). Yarış denetimli eşi sayac_yaris.go'dadır.

func add64(p *int64, d int64)       { *p += d }
func read64(p *int64) int64         { return *p }
func add32(p *int32, d int32) int32 { *p += d; return *p }
func read32i(p *int32) int32        { return *p }
func write32i(p *int32, v int32)    { *p = v }
func store64u(p *uint64, v uint64)  { *p = v }
func load64u(p *uint64) uint64      { return *p }
