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
	if M == 1 && g.gemv(batch, N, K, a, b, transB, c, cOf, over) {
		return
	}
	if g.ltMul(batch, M, N, K, g.addressOf(a), transA, g.addressOf(b), transB, c.addr+uint64(4*cOf), over, ltR32F, ltR32F) {
		c.dropShadow()
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

// gemv M = 1 çarpımını tek satırlık çekirdekle yapar (kernel_gemv.go); kapalıysa ya da K = 0
// ise yanlış döner. M = 1'de transA önemsizdir: A [1, K] de [K, 1] de bitişik K elemandır.
// Toplama sırası her zaman dört bitişik öğeyle tanımlıdır; vektörel okuma yalnız yükleme
// biçimidir ve satırlar dört elemana hizalıysa seçilir (devrik değilken B'nin satırları: N 4'ün
// katı; devrikken A ile B'nin satırları: K 4'ün katı). Hiza bitleri değiştirmez (15.22a). A ve C
// float32'dir; B float32 ya da 16 bitlik olabilir (ağırlık açılmadan okunur, kernel_gemv.go).
func (g *backend) gemv(batch, N, K int, a tensor.View, b tensor.View, transB bool, c *storage, cOf int, over bool) bool {
	if g.gemvOff || K == 0 {
		return false
	}
	for _, n := range []int{K, K * N, N} {
		checkSize(n) // gemm'in sınırı (H4016): 16 bitlik B de float32'deki gibi reddedilir
	}
	if batch*K*N > maxN {
		return false // toplu ofsetler 32 bite sığmaz: gemm'e kalır
	}
	ad, bd := g.storageOf(a.S), g.storageOf(b.S)
	bt := bd.dtype
	alignA, alignB := align(g.addressOf(a)) >= 16, align(g.addressOf(b)) >= uint32(4*dtypeSize(bt))
	var pull *kernel
	var cfg gemvConfig
	if transB {
		vec := K%4 == 0 && alignA && alignB
		pull = g.takeKernel(fmt.Sprintf("gemv_t:%v:%v:%s", vec, over, bt), gemvTKernel(vec, over, bt))
	} else {
		vec := N%4 == 0 && alignB
		cfg = g.pickGemv(batch, N)
		pull = g.takeKernel(fmt.Sprintf("gemv:%+v:%v:%v:%s", cfg, vec, over, bt), gemvKernel(cfg, vec, over, bt))
	}
	for z0 := 0; z0 < batch; z0 += 65535 {
		nz := min(batch-z0, 65535)
		p := g.param().read(ad, a.Offset+z0*K).read(bd, b.Offset+z0*K*N).write(c, cOf+z0*N).u32(N).u32(K)
		if transB {
			g.launch(pull, [3]uint32{blocks(N, gemvTWarps), uint32(nz), 1}, [3]uint32{32 * gemvTWarps, 1, 1}, p)
		} else {
			g.launch(pull, [3]uint32{blocks(N, cfg.L*gemvCols), uint32(nz), 1}, [3]uint32{uint32(cfg.T), 1, 1}, p)
		}
	}
	return true
}

// fixedSMs toplama sırasını belirleyen ayarlarda kartın SM sayısının yerine geçen sabittir
// (15.22a: ayar yalnız boydan seçilir; SM sayısı başlatma ızgarasını değiştirebilir, toplama
// sırasını değiştiremez): gemv'nin şerit sayısı (gemvMinBlocks) ve norm geri yönünün parça
// sayısı (backend_row.go). Değeri 0b'nin önce-sonra ölçüsünde L4'te gözden geçirilir.
const fixedSMs = 64

// gemvMinBlocks devrik olmayan tek satırlık çarpımın şeritlerini daraltma eşiğidir (eski 2·SM).
const gemvMinBlocks = 2 * fixedSMs

// pickGemv devrik olmayan tek satırlık çarpımın ayarı: blok önce 32 sütun (satır başına 128
// bayt) okur; bloklar gemvMinBlocks'u doldurmuyorsa sütun sayısı 8'e kadar yarılanır (satır
// başına en az bir 32 baytlık kesim). Seçim yalnız (N, batch) iledir. Blokta 512 iş parçacığı:
// ölçüm (RTX PRO 6000, SmolLM2 ve Qwen3 boyları) 256 ve 128'den hızlı ya da eşit (1536×576:
// 4,1 / 6,2 / 12,3 µs), kitaplıktan hızlı ya da eşit.
func (g *backend) pickGemv(batch, N int) gemvConfig {
	if a := g.gemvConst; a != nil {
		if a.L < 1 || a.L > 32 || a.L&(a.L-1) != 0 || a.T < 32 || a.T > 1024 || a.T%32 != 0 {
			panic(internalf("invalid matrix-vector setting %+v", *a))
		}
		return *a
	}
	L := 32 / gemvCols
	for L*gemvCols > 8 && int(blocks(N, L*gemvCols))*batch < gemvMinBlocks {
		L /= 2
	}
	return gemvConfig{L: L, T: 512}
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
	if g.storageOf(b.S).dtype.IsQuantized() {
		g.quantGemm(M, N, K, a, b, c, over)
		return
	}
	// 16 bitlik C, girdilerle aynı türdeyse kitaplık onu doğrudan yazar (float32 ara tampon
	// ve iki kopya yok).
	if cd := g.storageOf(c); cd.dtype != tensor.Float32 && g.gemm16(batch, M, N, K, a, transA, b, transB, cd, over) {
		return
	}
	gt := g.temp()
	defer gt.finish()
	cd, last := gt.write(c, !over)
	// Tek satır ve 16 bitlik bitişik B (ör. float32 etkinleşme @ bf16 ağırlık): ağırlık float32'ye
	// açılmadan gemv ile okunur.
	if M == 1 && b.IsContiguous() && g.storageOf(b.S).dtype != tensor.Float32 &&
		g.gemv(batch, N, K, gt.read(a), b, transB, cd, 0, over) {
		last()
		return
	}
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
	c.dropShadow()
	return true
}

// gemmTile matris çarpımının döşeme boylarını sabitler (sınama ve ölçüm için; nil
// otomatik seçime döner). Arka ucun ayarıdır, kullanıcıya görünmez.
func (g *backend) gemmTile(a *gemmConfig) {
	g.mu.Lock()
	g.gemmConst = a
	g.mu.Unlock()
}
