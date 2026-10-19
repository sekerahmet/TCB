//go:build race

package profiler

import (
	"sync/atomic"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Örnekleyicinin paylaşılan değerleri, yarış denetimli derleme (-race): atomik yazma ve
// okuma (bkz. pos_plain.go).

func storePos(p *uint64, v uint64)                  { atomic.StoreUint64(p, v) }
func loadPos(p *uint64) uint64                      { return atomic.LoadUint64(p) }
func storeTask(p *unsafe.Pointer, v unsafe.Pointer) { atomic.StorePointer(p, v) }
func loadTask(p *unsafe.Pointer) unsafe.Pointer     { return atomic.LoadPointer(p) }

// readDepth yarış denetimli derlemede çağrı yığını okunmaz (görevin kendi goroutine'ine
// aittir): örnekleyici son deyimin konumunu kullanır. Fonksiyon döndükten sonra çağıran
// satırın kalan işi bu derlemede dönen fonksiyonun son satırına yazılır.
func readDepth(*interp.VM) int { return -1 }

// hasDepth örnekleyicinin çağrı derinliğini okuyup okumadığıdır (testler).
const hasDepth = false
