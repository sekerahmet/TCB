package gpu

import (
	"fmt"
	"math"
	"slices"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Cihaz içi hata bayrağı (15.6, 15.45)
// ============================================================================
//
// Hatası öğeye bağlı çekirdek (bugün tam sayıya dönüşüm) hatalı öğenin anahtarını
// (düz konum << 4 | tensor.FaultKind) bir yuvaya tek atom.global.min.u64 ile yazar: konum ve
// çeşit birlikte yazılır, en küçük düz konum kazanır (15.6'nın "ilk hatalı öğe"si), iki
// cihazda aynıdır. Yuva 32 bayttır: anahtar (boşken bütün bitler bir) ve kazanan konumun en
// çok üç girdisinin ham bitleri. Hatalı çekirdekten sonra tek iş parçacıklı toplama çekirdeği
// (faultGatherKernel) anahtarı okur ve o konumun girdilerini yuvaya kopyalar: konak ayrıntıyı
// (ElemFault'un değerleri) işlemcinin aynı işlemiyle tek öğede yeniden hesaplar
// (tensor.FaultFromSlot). Kopya girdinin işlem anındaki değeridir: kayıtta konak yuvayı
// oynatmanın sonunda okur, girdi o ana kadar kaydın sonraki bir işiyle değişmiş olabilir.
//
// Kayıt dışında akışın tek yuvası her işlemde boşaltılır, işlemden sonra okunur (15.6'nın
// beklemesi). Kayıtta her işlem kaydın havuzundan kendi yuvasını alır; boşaltma da kayda girer
// ve her oynatmada yinelenir. Oynatmanın sonunda Graph.FirstFault bütün yuvaları okur; kayıt
// sırasındaki ilk dolu yuva hatadır (15.45).

var _ tensor.FaultRecording = (*Graph)(nil)

// faultSlotWords yuvanın 64 bitlik sözcük sayısı: anahtar ve üç girdi.
const faultSlotWords = 4

// emptyKey boş yuvanın anahtarı.
const emptyKey = math.MaxUint64

// faultSite bir hata yuvası: yuvanın kendisi, ayrıntıyı yeniden hesaplamak için işlem ve türler,
// iletinin konumu için çıktının şekli (oynatmada çağıranın şekli yoktur). index doğruysa yuva
// bir indeks çekirdeğinindir (gömme, dilim yazma; backend_indexed.go): işlemi yoktur, ayrıntı
// indeksin değeridir (ins[0] türünde). sum doğruysa yuva int64 toplamınındır (typed_reduce.go):
// ayrıntı 128 bitlik toplamdır (1. sözcük alt, 2. sözcük üst yarı).
type faultSite struct {
	slot  *storage
	op    tensor.TypedOp
	out   tensor.DType
	ins   []tensor.DType
	shape []int
	index bool
	sum   bool
}

// takeSlot işlemin yuvasını verir ve boşaltır: kayıtta kaydın havuzundan yeni bir yuva, kayıt
// dışında akışın tek yuvası.
func (g *backend) takeSlot() *storage {
	var d *storage
	if g.capturing != nil {
		d = g.allocate(faultSlotWords, tensor.Int64, false)
	} else {
		if g.faultSlot == nil {
			g.faultSlot = g.allocate(faultSlotWords, tensor.Int64, false)
		}
		d = g.faultSlot
	}
	g.try_("cuMemsetD32Async", g.sr.fill32(d.addr, math.MaxUint32, 2))
	return d
}

// gatherFault toplama çekirdeğini başlatır: anahtar doluysa kazanan konumun girdilerinin ham
// bitleri yuvanın 1., 2. ve 3. sözcüğüne.
func (g *backend) gatherFault(slot *storage, nd int, dts []tensor.DType, meta []int, ins []tensor.View) {
	key := fmt.Sprintf("hata_topla:%d:%v", nd, dts)
	k := g.takeKernel(key, faultGatherKernel(nd, dts))
	p := g.param()
	for _, v := range ins {
		p = p.readV(v)
	}
	g.launch(k, [3]uint32{1, 1, 1}, [3]uint32{1, 1, 1}, p.write(slot, 0).arr(len(meta), meta...))
}

// recordSite kayıtta yuvayı kayda ekler; sırası tensor.FaultRecorded'ın Index'idir.
func (g *backend) recordSite(s faultSite) *tensor.ElemFault {
	c := g.capturing
	c.sites = append(c.sites, s)
	return &tensor.ElemFault{Kind: tensor.FaultRecorded, Index: len(c.sites) - 1}
}

// readSlot yuvayı konağa okur (eşzamanlar); boşsa nil.
func (g *backend) readSlot(s faultSite) *tensor.ElemFault {
	var w [faultSlotWords]uint64
	g.download(unsafe.Pointer(&w[0]), s.slot.addr, 8*faultSlotWords)
	if w[0] == emptyKey {
		return nil
	}
	var f *tensor.ElemFault
	switch {
	case s.sum:
		f = tensor.SumFaultFromSlot(w[0], w[1], w[2])
	case s.index:
		f = tensor.IndexFaultFromSlot(s.ins[0], w[0], w[1])
	default:
		f = tensor.FaultFromSlot(s.op, s.out, s.ins, w[0], w[1:1+len(s.ins)])
	}
	f.Shape = slices.Clone(s.shape)
	return f
}

// FirstFault oynatmadan sonra kaydın yuvalarını okur (eşzamanlar): kayıt sırasındaki ilk dolu
// yuvanın sırası ve hatası; yuva yoksa ya da hepsi boşsa -1, nil.
func (c *Graph) FirstFault() (int, *tensor.ElemFault) {
	if len(c.sites) == 0 {
		return -1, nil
	}
	g := c.g
	g.enter()
	defer g.exit()
	for i, s := range c.sites {
		if f := g.readSlot(s); f != nil {
			return i, f
		}
	}
	return -1, nil
}

// writeFault hatanın anahtarını (at << 4 | kind; at ve kind u32) bayrağa tek
// atom.global.min.u64 ile yazar: en küçük konum kazanır (15.6).
func (k *emitter) writeFault(flag, at, kind reg) {
	key, kd := k.rd(), k.rd()
	k.e("cvt.u64.u32", key, at)
	k.e("shl.b64", key, key, 4)
	k.e("cvt.u64.u32", kd, kind)
	k.e("or.b64", key, key, kd)
	old := k.rd()
	k.e("atom.global.min.u64", old, mem(flag, 0), key)
}

// leaderFault writeFault'u yalnız lane 0 olan iş parçacığında yapar (aynı hatayı gören iş
// parçacıklarından biri: gömmenin satır bloğu, dilim yazmanın öğeleri).
func (k *emitter) leaderFault(flag, lane, at, kind reg) {
	skip, p := k.label(), k.p()
	k.e("setp.ne.u32", p, lane, 0)
	k.jumpIf(p, skip)
	k.writeFault(flag, at, kind)
	k.put(skip)
}

// faultGatherKernel tek iş parçacığı: yuvanın anahtarı doluysa konumun girdilerini (yayınlamalı;
// meta işlemin metasıdır) ham bitleriyle yuvaya yazar.
func faultGatherKernel(nd int, ins []tensor.DType) func() *emitter {
	return func() *emitter {
		name := fmt.Sprintf("hata_topla_%d", nd)
		for _, dt := range ins {
			name += "_" + dt.String()
		}
		k := newEmitter(name)
		ptrs := make([]reg, len(ins))
		for j := range ins {
			ptrs[j] = k.pointer(fmt.Sprintf("g%d", j))
		}
		slot := k.written("yuva")
		k.arr("meta", (len(ins)+1)*nd)
		key := k.rd()
		k.e("ld.global.u64", key, mem(slot, 0))
		empty := k.p()
		k.e("setp.eq.u64", empty, key, int64(-1))
		end := k.label()
		k.jumpIf(empty, end)
		k.e("shr.u64", key, key, 4)
		i := k.r()
		k.e("cvt.u32.u64", i, key)
		ofs := k.tripleOffsets(i, nd, len(ins))
		for j, dt := range ins {
			raw := k.rawLoad(dt, k.addressS(ptrs[j], ofs[j], dtypeSize(dt)))
			w := raw
			switch dtypeSize(dt) {
			case 2:
				w = k.rd()
				k.e("cvt.u64.u16", w, raw)
			case 1, 4:
				w = k.rd()
				k.e("cvt.u64.u32", w, raw)
			}
			k.e("st.global.u64", mem(slot, 8*(1+j)), w)
		}
		k.put(end)
		return k
	}
}
