package gpu

import (
	"fmt"

	"tr_dili/internal/hesap/tensor"
)

// Arka uç: toplu matris çarpımı.

// carpimSec problem boyuna göre döşeme ayarını seçer: iki boyutu da büyük çarpımlar
// büyük döşemeyle, diğerleri varsayılanla.
func (g *arkaUc) carpimSec(M, N int) carpimAyari {
	if g.carpimSabit != nil {
		return *g.carpimSabit
	}
	if M >= 1024 && N >= 1024 {
		return buyukCarpim
	}
	return g.carpim
}

// gemm C (+)= op(A)·op(B), batch toplu; uzerine doğruysa C'nin eski içeriği okunmaz.
// Görünümler float32 bitişiktir.
func (g *arkaUc) gemm(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c *depo, cOf int, uzerine bool) {
	if batch == 0 || M == 0 || N == 0 {
		return
	}
	for _, n := range []int{M * K, K * N, M * N} {
		boyDenetle(n)
	}
	ay := g.carpimSec(M, N)
	if !ay.gecerli() {
		panic(fmt.Sprintf("tensor: GPU: geçersiz matris çarpımı döşeme ayarı %+v (iç hata)", ay))
	}
	if bloklar(M, ay.BM) > 65535 {
		yok(fmt.Sprintf("%d satırlı matris çarpımı (en çok %d)", M, 65535*ay.BM))
	}
	anahtar := fmt.Sprintf("carpim:%+v:%v:%v:%v", ay, transA, transB, uzerine)
	cek := g.cekirdekAl(anahtar, carpimCekirdegi(ay, transA, transB, uzerine))
	ad, bd := g.depoOf(a.S), g.depoOf(b.S)
	for z0 := 0; z0 < batch; z0 += 65535 {
		nz := min(batch-z0, 65535)
		p := g.param().oku(ad, a.Offset+z0*M*K).oku(bd, b.Offset+z0*K*N).yaz(c, cOf+z0*M*N).u32(M).u32(N).u32(K)
		g.baslat(cek, [3]uint32{bloklar(N, ay.BN), bloklar(M, ay.BM), uint32(nz)}, [3]uint32{uint32(ay.is()), 1, 1}, p)
	}
}

// Gemm (Backend) her toplu eleman için C += op(A)·op(B).
func (g *arkaUc) Gemm(batch, M, N, K int, a tensor.View, transA bool, b tensor.View, transB bool, c tensor.Storage) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	cd, son := gt.yaz(c, true)
	g.gemm(batch, M, N, K, gt.oku(a), transA, gt.oku(b), transB, cd, 0, false)
	son()
}

// carpimDosemesi matris çarpımının döşeme boylarını sabitler (sınama ve ölçüm için; nil
// otomatik seçime döner). Arka ucun ayarıdır, kullanıcıya görünmez.
func (g *arkaUc) carpimDosemesi(a *carpimAyari) {
	g.mu.Lock()
	g.carpimSabit = a
	g.mu.Unlock()
}
