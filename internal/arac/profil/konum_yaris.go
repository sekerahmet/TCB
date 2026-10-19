//go:build race

package profil

import (
	"sync/atomic"
	"unsafe"

	"tr_dili/internal/dil/yurut"
)

// Örnekleyicinin paylaşılan değerleri, yarış denetimli derleme (-race): atomik yazma ve
// okuma (bkz. konum_duz.go).

func konumYaz(p *uint64, v uint64)                 { atomic.StoreUint64(p, v) }
func konumOku(p *uint64) uint64                    { return atomic.LoadUint64(p) }
func gorevYaz(p *unsafe.Pointer, v unsafe.Pointer) { atomic.StorePointer(p, v) }
func gorevOku(p *unsafe.Pointer) unsafe.Pointer    { return atomic.LoadPointer(p) }

// derinlikOku yarış denetimli derlemede çağrı yığını okunmaz (görevin kendi goroutine'ine
// aittir): örnekleyici son deyimin konumunu kullanır. Fonksiyon döndükten sonra çağıran
// satırın kalan işi bu derlemede dönen fonksiyonun son satırına yazılır.
func derinlikOku(*yurut.VM) int { return -1 }

// derinlikVar örnekleyicinin çağrı derinliğini okuyup okumadığıdır (testler).
const derinlikVar = false
