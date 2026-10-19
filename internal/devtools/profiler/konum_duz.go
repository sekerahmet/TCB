//go:build !race

package profiler

import (
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Örnekleyicinin paylaşılan değerleri, yarış denetimsiz derleme: düz yazma ve okuma.
// Yorumlayıcı her deyimde yazar, örnekleyici goroutine'i 1 ms'de bir okur; hepsi tek kelime
// (hizalı 64 bitlik değer ya da gösterici) olduğundan bölünmeden yazılır ve okunur. Atomik
// yazma (XCHG) her deyime ~10 ns eklerdi. Yarış denetimli eşi konum_yaris.go'dadır.

func konumYaz(p *uint64, v uint64)                 { *p = v }
func konumOku(p *uint64) uint64                    { return *p }
func gorevYaz(p *unsafe.Pointer, v unsafe.Pointer) { *p = v }
func gorevOku(p *unsafe.Pointer) unsafe.Pointer    { return *p }

// derinlikOku görevin çağrı derinliğidir. Çağrı yığını görevin kendi goroutine'inde
// değişir; burada yalnızca dilim başlığının uzunluk kelimesi okunur (eskimiş olabilir, bir
// sonraki örnekte düzelir; bellek güvenliğini etkilemez: yalnızca bir tam sayıdır).
func derinlikOku(v *interp.VM) int { return len(v.CallStack) }

// derinlikVar örnekleyicinin çağrı derinliğini okuyup okumadığıdır (testler).
const derinlikVar = true
