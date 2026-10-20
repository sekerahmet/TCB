package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Arka uç: toplu matris çarpımı.

// pickGemm problem boyuna göre döşeme ayarını seçer: büyük döşemeyle de bütün SM'lere
// en az bir blok düşüyorsa büyük döşeme, değilse varsayılan.
func (g *backend) pickGemm(batch, M, N int) gemmConfig {
	if g.gemmConst != nil {
		return *g.gemmConst
	}
	_, sm := g.sr.capability()
	b := largeGemm
	if int(blocks(M, b.BM))*int(blocks(N, b.BN))*batch >= max(sm, 1) && M >= 256 && N >= 256 {
		return b
	}
	return g.gemm2
}

// gemm C (+)= op(A)·op(B), batch toplu; uzerine doğruysa C'nin eski içeriği okunmaz.
// Görünümler float32 bitişiktir.
func (g *backend) gemm(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c *storage, cOf int, over bool) {
	if batch == 0 || M == 0 || N == 0 {
		return
	}
	for _, n := range []int{M * K, K * N, M * N} {
		checkSize(n)
	}
	if g.ltMul(batch, M, N, K, g.addressOf(a), transA, g.addressOf(b), transB, c.addr+uint64(4*cOf), over, ltR32F, ltR32F) {
		c.host = nil
		return
	}
	ay := g.pickGemm(batch, M, N)
	if !ay.valid() {
		panic(internalf("invalid matrix multiply tile setting %+v", ay))
	}
	if blocks(M, ay.BM) > 65535 {
		none(messages.H4053, M, 65535*ay.BM)
	}
	key := fmt.Sprintf("carpim:%+v:%v:%v:%v", ay, transA, transB, over)
	pull := g.takeKernel(key, gemmKernel(ay, transA, transB, over))
	ad, bd := g.storageOf(a.S), g.storageOf(b.S)
	for z0 := 0; z0 < batch; z0 += 65535 {
		nz := min(batch-z0, 65535)
		p := g.param().read(ad, a.Offset+z0*M*K).read(bd, b.Offset+z0*K*N).write(c, cOf+z0*M*N).u32(M).u32(N).u32(K)
		g.launch(pull, [3]uint32{blocks(N, ay.BN), blocks(M, ay.BM), uint32(nz)}, [3]uint32{uint32(ay.is()), 1, 1}, p)
	}
}

// Gemm (Backend) her toplu eleman için C += op(A)·op(B).
func (g *backend) Gemm(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c tensor.Storage) {
	g.enter()
	defer g.exit()
	g.gemmOuter(batch, M, N, K, a, transA, b, transB, c, false)
}

// GemmSet (tensor.GemmSetter) C = op(A)·op(B); C'nin eski içeriği okunmaz.
func (g *backend) GemmSet(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c tensor.Storage) {
	g.enter()
	defer g.exit()
	g.gemmOuter(batch, M, N, K, a, transA, b, transB, c, true)
}

func (g *backend) gemmOuter(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c tensor.Storage, over bool) {
	// 16 bitlik C, girdilerle aynı türdeyse kitaplık onu doğrudan yazar (float32 ara tampon
	// ve iki kopya yok).
	if cd := g.storageOf(c); cd.dtype != tensor.Float32 && g.gemm16(batch, M, N, K, a, transA, b, transB, cd, over) {
		return
	}
	gt := g.temp()
	defer gt.finish()
	cd, last := gt.write(c, !over)
	if !g.gemm16(batch, M, N, K, a, transA, b, transB, cd, over) {
		g.gemm(batch, M, N, K, gt.read(a), transA, gt.read(b), transB, cd, 0, over)
	}
	last()
}

// gemm16 bitişik 16 bitlik (bf16 ya da f16) girdileri float32'ye açmadan kitaplığa verir:
// tensör çekirdekleri girdileri 16 bit okur, float32 biriktirir ve C'ye (float32 ya da
// girdilerle aynı 16 bitlik tür) bir kez yuvarlayıp yazar. Kitaplık yoksa, girdilerin türü
// farklıysa, C başka bir türdeyse ya da girdilerden biri adımlıysa yanlış döner; o zaman
// girdiler float32'ye açılıp çarpılır (karışık hassasiyette her çarpım eskiden böyleydi:
// bf16 adım float32'den yavaştı).
func (g *backend) gemm16(batch, M, N, K int, a tensor.View, ta bool, b tensor.View, tb bool, c *storage, over bool) bool {
	if g.lt == nil || batch == 0 || M == 0 || N == 0 || K == 0 || !a.IsContiguous() || !b.IsContiguous() {
		return false
	}
	da, db := g.storageOf(a.S), g.storageOf(b.S)
	if da.dtype != db.dtype {
		return false
	}
	var ab uintptr
	switch da.dtype {
	case tensor.BFloat16:
		ab = ltR16BF
	case tensor.Float16:
		ab = ltR16F
	default:
		return false
	}
	ct := uintptr(ltR32F)
	switch c.dtype {
	case tensor.Float32:
	case da.dtype:
		ct = ab
	default:
		return false
	}
	for _, n := range []int{M * K, K * N, M * N} {
		checkSize(n)
	}
	if !g.ltMul(batch, M, N, K, g.addressOf(a), ta, g.addressOf(b), tb, c.addr, over, ab, ct) {
		return false
	}
	c.host = nil
	return true
}

// gemmTile matris çarpımının döşeme boylarını sabitler (sınama ve ölçüm için; nil
// otomatik seçime döner). Arka ucun ayarıdır, kullanıcıya görünmez.
func (g *backend) gemmTile(a *gemmConfig) {
	g.mu.Lock()
	g.gemmConst = a
	g.mu.Unlock()
}
