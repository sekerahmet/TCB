package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// quantActPath çarpımın yolunu sınamada zorlar (bitleri değiştirmez): "" kartın yeteneğinden, "mma" (M > 1'de
// mma.sync), "dp4a" (M > 1'de karolu dp4a), "nodp4a" (dp4a'sız eşler, sm_61 altındaki gibi).
var quantActPath = ""

// quantActGemvLimit tek başlatmalı satır çekirdeğinin paylaşımlı bellek sınırıdır (bayt); sınamada küçültülür.
var quantActGemvLimit = staticSharedMemLimit

// GemmActQuant (tensor.ActQuantGemmer) c = nicem(a) · aç(bv)'dir (15.53a; c ezilir): bv q4, q4_k ya da q6_k
// ağırlığın devrik görünümüdür. a float32'ye okunur. Yol (bitleri değiştirmez):
//   - M = 1: tek başlatma (quantActGemv); K paylaşımlı belleğe sığmazsa karolu çekirdek;
//   - M > 1 (quantActGemm): satırlar ağırlığın bloğuyla int8'e nicemlenir; q4 her kartta karolu çekirdek
//     (quantActTileKernel), q4_k ve q6_k sm_80+ mma.sync s8 (kernel_quant_act_mma.go), altında karolu çekirdek;
//   - sm_61 altında dp4a yerine dört tamsayı çarpma-toplama.
//
// Bitler CPU'nun GemmActQuant'ıyla ve yoldan bağımsız aynıdır.
func (g *backend) GemmActQuant(M, N, K int, a tensor.View, bv tensor.View, c tensor.Storage) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	cd, last := gt.write(c, false)
	defer last()
	if M == 0 || N == 0 {
		return
	}
	if K == 0 {
		g.reset(cd) // boş toplam
		return
	}
	checkSize(M * N)
	checkSize(M * K)
	bd := g.storageOf(bv.S)
	dt := bd.dtype
	if dt != tensor.Q4 && dt != tensor.Q4K && dt != tensor.Q6K {
		panic(internalf("int8 activation product on the GPU takes q4, q4_k or q6_k, got %s", dt.Name()))
	}
	av := gt.read(a)
	if M == 1 && quantActGemvShared(dt, K) <= quantActGemvLimit {
		g.quantActGemv(gt, N, K, av, bd, bv.Offset, cd)
		return
	}
	g.reset(cd)
	g.quantActGemm(gt, M, N, K, av, bd, bv.Offset, cd)
}

// actDp4a kartın sm sürümü ve dp4a'nın kullanılıp kullanılmadığıdır. Zorlanan yol yeteneği de seçer: benzetici sm_52
// bildirir, "dp4a" ve "mma" orada da dp4a'lı eşi sınar.
func (g *backend) actDp4a() (cc int, dp bool) {
	cc, _ = g.sr.capability()
	return cc, (cc >= 61 || quantActPath == "dp4a" || quantActPath == "mma") && quantActPath != "nodp4a"
}

// quantActGemv M = 1 çarpımıdır; çekirdek etkinleştirmeyi 16 baytlık okur, hizasız görünüm kopyalanır.
func (g *backend) quantActGemv(gt *temp, N, K int, a tensor.View, bd *storage, bOffset int, c *storage) {
	ad, offset := g.storageOf(a.S), a.Offset
	if align(g.addressOf(a)) < 16 {
		tmp := gt.buffer(K, false)
		g.dCopy(tmp, 0, ad, offset, K)
		ad, offset = tmp, 0
	}
	dt := bd.dtype
	elems, _ := dt.QuantBlock()
	_, dp := g.actDp4a()
	mv := g.takeKernel(fmt.Sprintf("nicem_etk_gemv:%s:%v", dt, dp), quantActGemvKernel(dt, dp))
	g.launchShared(mv, [3]uint32{blocks(N, actWarpsMV), 1, 1}, [3]uint32{32 * actWarpsMV, 1, 1},
		uint32(quantActGemvShared(dt, K)), g.param().read(ad, offset).read(bd, bOffset).write(c, 0).u32(N).u32(K/elems))
}

// quantActGemm nicemleme, çarpım ve sonlu olmayan satır başlatmalarıdır; c sıfırlanmıştır. Sonlu olmayan değer taşıyan
// satır tamsayı çekirdeklerinde atlanır, float yoluyla yazılır (quantActNonFiniteKernel).
func (g *backend) quantActGemm(gt *temp, M, N, K int, av tensor.View, bd *storage, bOffset int, cd *storage) {
	dt := bd.dtype
	elems, _ := dt.QuantBlock()
	nb := K / elems
	ad := g.storageOf(av.S)
	// Kod tamponunun 256 baytlık dolgusu: q4'ün son diliminin taşan okuması (kernel_quant_act_tile.go).
	q := gt.buffer((M*K+256+3)/4, false)
	d := gt.buffer(M*nb, false)
	bs := gt.buffer(M*K/16, false)
	fin := gt.buffer(M, true)
	qk := g.takeKernel(fmt.Sprintf("nicem_etk_q8:%d", elems), quantActQuantizeKernel(elems))
	g.launch(qk, [3]uint32{blocks(K, actBlk), uint32(M), 1}, [3]uint32{32, 1, 1},
		g.param().read(ad, av.Offset).write(q, 0).write(d, 0).write(bs, 0).write(fin, 0).u32(K))
	args := func() *paramWriter {
		return g.param().read(q, 0).read(d, 0).read(bs, 0).read(bd, bOffset).write(cd, 0).read(fin, 0).
			u32(M).u32(N).u32(nb)
	}
	cc, dp := g.actDp4a()
	if dt == tensor.Q4 || M == 1 || quantActPath == "dp4a" || quantActPath == "nodp4a" || (cc < 80 && quantActPath == "") {
		tk := g.takeKernel(fmt.Sprintf("nicem_etk_karo:%s:%v", dt, dp), quantActTileKernel(dt, dp))
		g.launch(tk, [3]uint32{blocks(N, 64), blocks(M, 64), 1}, [3]uint32{tileThreads, 1, 1}, args())
	} else {
		gen := quantActQ4KKernel
		if dt == tensor.Q6K {
			gen = quantActQ6KKernel
		}
		mm := g.takeKernel(fmt.Sprintf("nicem_etk_mma:%s", dt), gen)
		g.launch(mm, [3]uint32{blocks(N, actTile), blocks(M, actTile), 1}, [3]uint32{32 * actWarpsMM, 1, 1}, args())
	}
	nf := g.takeKernel(fmt.Sprintf("nicem_etk_sonsuz:%s", dt), quantActNonFiniteKernel(dt))
	g.launch(nf, [3]uint32{blocks(N, 128), uint32(M), 1}, [3]uint32{128, 1, 1},
		g.param().read(ad, av.Offset).read(bd, bOffset).read(fin, 0).write(cd, 0).u32(N).u32(K))
}
