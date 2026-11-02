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
	iv, it := g.readIndex(gt, idx)
	dt := g.storageOf(wv.S).dtype
	c := g.takeKernel("gomme_cihaz:"+dt.String()+indexKey(it), embedKernel(dt, true, it))
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
	st, it := g.readIndex(gt, start)
	sd := g.storageOf(sv.S).dtype
	c := g.takeKernel("dilim_yaz:"+sd.String()+":"+d.dtype.String()+indexKey(it), updateSliceKernel(sd, d.dtype, it))
	g.launch(c, [3]uint32{blocks(n, 256), 1, 1}, [3]uint32{256, 1, 1},
		g.param().write(d, dst.Offset).readV(sv).readV(st).u32(n).u32(Td).u32(Ts).u32(inner))
}

// readIndex indeks tensörünü çekirdeğe hazırlar: "int64" olduğu gibi (bitişik), öbürleri float32
// olarak; öbür tam sayı ve mantık türleri hassasiyet hatasıdır (15.48).
func (g *backend) readIndex(gt *temp, v tensor.View) (tensor.View, tensor.DType) {
	if dt := g.storageOf(v.S).dtype; dt == tensor.Int64 {
		return gt.readNative(v), dt
	}
	return gt.read(v), tensor.Float32
}

// indexKey çekirdek anahtarının indeks türü eki (float32 indekste boş: anahtar eskisiyle aynı).
func indexKey(it tensor.DType) string {
	if it == tensor.Float32 {
		return ""
	}
	return ":" + it.Name()
}
