package gpu

import (
	"fmt"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: nicemlenmiş depo (15.51–15.55). Depo dosyanın bloklarıdır, bayt düzeni CPU'nunkiyle
// aynıdır; n öğe sayısıdır, görünümün Offset'i öğe cinsindendir ve blok sınırındadır. Bloklar kartta
// nicemli durur: hiçbir yol bütün ağırlığı kalıcı açmaz.

// rawElems raw'ın öğe sayısıdır.
func rawElems(raw []byte, dt tensor.DType) int {
	if dt.IsQuantized() {
		elems, size := dt.QuantBlock()
		return len(raw) / size * elems
	}
	return len(raw) / dtypeSize(dt)
}

// quantToRaw görünümün bloklarını bayt bayt konağa kopyalar; bitişik öğe aralığıdır.
func (g *backend) quantToRaw(d *storage, src tensor.View) []byte {
	out := make([]byte, storageBytes(src.Numel(), d.dtype))
	if len(out) > 0 {
		g.download(unsafe.Pointer(&out[0]), d.addr+storageBytes(src.Offset, d.dtype), len(out))
	}
	return out
}

// refuseQuantCopy öğe öğe kopyanın kapısıdır: nicemlenmiş depoda öğe yoktur, blok vardır (15.53).
func (g *backend) refuseQuantCopy(ds ...*storage) {
	for _, d := range ds {
		if d.dtype.IsQuantized() {
			panic(tensor.PrecisionError(d.dtype, g.dev))
		}
	}
}

// OpenQuant (tensor.QuantOpener) bitişik nicemlenmiş görünümü float32 out'a açar.
func (g *backend) OpenQuant(out tensor.Storage, src tensor.View) {
	g.enter()
	defer g.exit()
	n := src.Numel()
	if n == 0 {
		return
	}
	checkSize(n)
	o, d := g.storageOf(out), g.storageOf(src.S)
	g.openQuant(o, 0, d, src.Offset, n)
}

// openQuant d'nin offset'ten başlayan n öğesini o'nun oOf'una açar.
func (g *backend) openQuant(o *storage, oOf int, d *storage, offset, n int) {
	c := g.takeKernel(fmt.Sprintf("nicem_ac:%s", d.dtype), quantOpenKernel(d.dtype))
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, g.param().read(d, offset).write(o, oOf).u32(n))
}
