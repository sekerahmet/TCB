//go:build race

package tensor

import "sync/atomic"

// Profil sayaçlarının bellek erişimleri, yarış denetimli derleme (-race): atomik işlemler.
// Davranış sayac_duz.go ile aynıdır; yalnızca yarış dedektörü kilitsiz erişimleri hata
// saymasın diye atomiktir.

func add64(p *int64, d int64)       { atomic.AddInt64(p, d) }
func read64(p *int64) int64         { return atomic.LoadInt64(p) }
func add32(p *int32, d int32) int32 { return atomic.AddInt32(p, d) }
func read32i(p *int32) int32        { return atomic.LoadInt32(p) }
func write32i(p *int32, v int32)    { atomic.StoreInt32(p, v) }
