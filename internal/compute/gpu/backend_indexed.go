package gpu

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: indeksi cihazdan okuyan işlemler (tensor.IndexReader) ve kayıt durumu
// (tensor.Capturer).

// Capturing (tensor.Capturer) bir kayıt sürüyor mu (graph.go).
func (g *backend) Capturing() bool { return g.capturing != nil }

func (g *backend) EmbeddingAt(out tensor.Storage, w tensor.View, idx tensor.View, D int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	n := idx.Numel()
	checkSize(n * D)
	wv := gt.readNative(w)
	iv := gt.read(idx)
	dt := g.storageOf(wv.S).dtype
	c := g.takeKernel("gomme_cihaz:"+dt.String(), embedKernel(dt, true))
	g.launch(c, [3]uint32{uint32(n), 1, 1}, [3]uint32{embedBlock, 1, 1},
		g.param().readV(wv).readV(iv).write(o, 0).u32(D).u32(w.Shape[0]))
	last()
}

func (g *backend) UpdateSliceAt(dst tensor.View, src tensor.View, start tensor.View, outer, Td, Ts, inner int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	n := outer * Ts * inner
	checkSize(outer * Td * inner)
	d := g.storageOf(dst.S)
	sv := gt.readNative(src)
	st := gt.read(start)
	sd := g.storageOf(sv.S).dtype
	c := g.takeKernel("dilim_yaz:"+sd.String()+":"+d.dtype.String(), updateSliceKernel(sd, d.dtype))
	g.launch(c, [3]uint32{blocks(n, 256), 1, 1}, [3]uint32{256, 1, 1},
		g.param().write(d, dst.Offset).readV(sv).readV(st).u32(n).u32(Td).u32(Ts).u32(inner))
}
