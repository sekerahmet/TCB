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

// quantGemm c (+)= a·aç(w)ᵀ'dir: a [M, K], b [N, K] deposu (15.53). a float32'ye okunur, hesap
// float32'dedir, 16 bitlik c'ye bir kez yuvarlanır; CPU'nun Gemm'iyle aynı düzen.
func (g *backend) quantGemm(M, N, K int, a, b tensor.View, c tensor.Storage, over bool) {
	gt := g.temp()
	defer gt.finish()
	cd, last := gt.write(c, !over)
	defer last()
	if M == 0 || N == 0 {
		return
	}
	if K == 0 {
		if over {
			g.reset(cd) // boş toplam: float çarpımıyla aynı
		}
		return
	}
	checkSize(M * N)
	checkSize(N * K)
	av := gt.read(a)
	if M == 1 {
		g.quantGemv(gt, N, K, av, b, cd, over)
		return
	}
	g.quantGemmRows(gt, M, N, K, av, b, cd, over)
}

// quantTileFloats çok satırlı çarpımın bir parçasında açılan ağırlık ve çıktı tamponlarının float32
// sayısıdır: 64 MiB. Parça yalnız şekle bağlıdır, toplama sırası da öyle kalır.
const quantTileFloats = 1 << 24

// quantTileRows bir parçada açılan ağırlık satırıdır: en az 64.
func (g *backend) quantTileRows(M, N, K int) int {
	budget := quantTileFloats
	if g.quantTile > 0 {
		budget = g.quantTile
	}
	return min(max(budget/(K+M)/64*64, 64), N)
}

// quantGemmRows M > 1 çarpımıdır: ağırlık satır parçaları sırayla geçici tampona açılır, parça
// float32 çarpımla c'nin sütunlarına gider. Bütün ağırlık hiç açılmaz; CPU'nun quantGemm'iyle aynı yol.
func (g *backend) quantGemmRows(gt *temp, M, N, K int, a, b tensor.View, c *storage, over bool) {
	bd := g.storageOf(b.S)
	rows := g.quantTileRows(M, N, K)
	tile := gt.buffer(rows*K, false)
	if rows == N && over {
		g.openQuant(tile, 0, bd, b.Offset, N*K)
		g.gemm(1, M, N, K, a, false, plainV(tile), true, c, 0, true)
		return
	}
	part := gt.buffer(M*rows, false)
	for n0 := 0; n0 < N; n0 += rows {
		t := min(rows, N-n0)
		g.openQuant(tile, 0, bd, b.Offset+n0*K, t*K)
		g.gemm(1, M, t, K, a, false, plainV(tile), true, part, 0, true)
		band := tensor.View{S: c, Offset: n0, Shape: []int{M, t}, Strides: []int{N, 1}}
		g.copyStrided(band, tensor.View{S: part, Shape: []int{M, t}, Strides: []int{t, 1}}, !over, 1)
	}
}

// quantGemv M = 1 çarpımıdır; çekirdek etkinleştirmeyi 16 baytlık okur, hizasız görünüm kopyalanır.
func (g *backend) quantGemv(gt *temp, N, K int, a, b tensor.View, c *storage, over bool) {
	ad, bd := g.storageOf(a.S), g.storageOf(b.S)
	offset := a.Offset
	if align(g.addressOf(a)) < 16 {
		tmp := gt.buffer(K, false)
		g.dCopy(tmp, 0, ad, offset, K)
		ad, offset = tmp, 0
	}
	elems, _ := bd.dtype.QuantBlock()
	pull := g.takeKernel(fmt.Sprintf("nicem_gemv:%s:%v", bd.dtype, over), quantGemvKernel(bd.dtype, over))
	g.launch(pull, [3]uint32{blocks(N, quantWarps), 1, 1}, [3]uint32{32 * quantWarps, 1, 1},
		g.param().read(ad, offset).read(bd, b.Offset).write(c, 0).u32(N).u32(K/elems))
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
