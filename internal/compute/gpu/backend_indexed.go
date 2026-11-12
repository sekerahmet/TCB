package gpu

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: indeksi cihazdan okuyan işlemler (tensor.IndexReader) ve kayıt durumu
// (tensor.Capturer).

// Capturing (tensor.Capturer) bir kayıt sürüyor mu (graph.go).
func (g *backend) Capturing() bool { return g.capturing != nil }

func (g *backend) EmbeddingAt(out tensor.Storage, w tensor.View, idx tensor.View, D int) *tensor.ElemFault {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	n := idx.Numel()
	checkSize(n * D)
	wv := gt.readNative(w)
	iv, it := g.readIndex(gt, idx, true)
	dt := g.storageOf(wv.S).dtype
	slot := g.takeSlot()
	c := g.takeKernel("gomme_cihaz:"+dt.String()+indexKey(it), embedKernel(dt, true, it))
	g.launch(c, [3]uint32{uint32(n), 1, 1}, [3]uint32{embedBlock, 1, 1},
		g.param().readV(wv).readV(iv).write(o, 0).u32(D).u32(w.Shape[0]).write(slot, 0))
	last()
	return g.indexFault(slot, it, iv, idx.Shape)
}

func (g *backend) UpdateSliceAt(dst tensor.View, src tensor.View, start tensor.View, outer, Td, Ts, inner int) *tensor.ElemFault {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	n := outer * Ts * inner
	checkSize(outer * Td * inner)
	d := g.storageOf(dst.S)
	sv := gt.readNative(src)
	st, it := g.readIndex(gt, start, false)
	sd := g.storageOf(sv.S).dtype
	slot := g.takeSlot()
	c := g.takeKernel("dilim_yaz:"+sd.String()+":"+d.dtype.String()+indexKey(it), updateSliceKernel(sd, d.dtype, it))
	g.launch(c, [3]uint32{blocks(n, 256), 1, 1}, [3]uint32{256, 1, 1},
		g.param().write(d, dst.Offset).readV(sv).readV(st).u32(n).u32(Td).u32(Ts).u32(inner).write(slot, 0))
	return g.indexFault(slot, it, st, start.Shape)
}

// readIndex indeks tensörünü çekirdeğe hazırlar: "int64" olduğu gibi (bitişik), wide64'te
// float64 de (gömme: doğrudan çağrı float64 indeksi kendi türünde denetler); öbürleri float32
// olarak (dilim yazmanın doğrudan çağrısı başlangıcı float32 okur). Öbür tam sayı ve mantık
// türleri hassasiyet hatasıdır (15.48).
func (g *backend) readIndex(gt *temp, v tensor.View, wide64 bool) (tensor.View, tensor.DType) {
	if dt := g.storageOf(v.S).dtype; dt == tensor.Int64 || wide64 && dt == tensor.Float64 {
		return gt.readNative(v), dt
	}
	return gt.read(v), tensor.Float32
}

// indexFault indeks çekirdeğinin yuvasını toplar: kayıtta yuva kayda girer (FaultRecorded),
// kayıt dışında okunur (bekler); hata yoksa nil. iv çekirdeğe verilen indeks, şekil iletinin.
func (g *backend) indexFault(slot *storage, it tensor.DType, iv tensor.View, shape []int) *tensor.ElemFault {
	g.gatherFault(slot, 0, []tensor.DType{it}, nil, []tensor.View{iv})
	site := faultSite{slot: slot, index: true, ins: []tensor.DType{it}, shape: slices.Clone(shape)}
	if g.capturing != nil {
		return g.recordSite(site)
	}
	return g.readSlot(site)
}

// indexKey çekirdek anahtarının indeks türü eki (float32 indekste boş: anahtar eskisiyle aynı).
func indexKey(it tensor.DType) string {
	if it == tensor.Float32 {
		return ""
	}
	return ":" + it.Name()
}
