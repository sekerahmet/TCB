package gpu

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: bölünmüş dikkat (kernel_flash_split.go). Hangi şeklin bu yolda olduğu ve ayarları
// yalnız şekilden gelir (15.22a): aşağıdaki sabitler toplama sırasını belirler, değişirlerse bitler
// TR sürümleri arasında değişir (docs/spec/rationale/15_tensors.md, 15.22a).

const (
	splitMaxD       = 128      // bakış boyu 32'nin katı ve en çok bu
	splitMaxRows    = 256      // G·Tq: satır sınırı
	splitMinKeys    = 16       // (a) az sorguda en az anahtar
	splitLongKeys   = 256      // (b) uzun anahtarda en az anahtar
	splitLongD      = 64       // (b) en küçük bakış boyu
	splitTempMax    = 64 << 20 // kısmi sonuçların geçici belleğinin sınırı (bayt)
	splitGridDimMax = 65535    // ızgaranın y ve z boyunun sınırı
	splitFewRows    = 4        // (a) Tq ≤ Br / splitFewRows
)

// splitFits bölünmüş çekirdeğin bu tanım için kullanılabildiği (bakış boyu, ek maske, ızgara ve
// geçici bellek) ve ayarları. attnPath "bolme" seçim kuralını (splitPreferred) geçer.
func (g *backend) splitFits(mask2 bool, p tensor.AttentionParams) (flashSplit, bool) {
	if mask2 || g.attnPath != "" && g.attnPath != "bolme" {
		return flashSplit{}, false
	}
	if p.Dh != p.Dv || p.Dh == 0 || p.Dh%32 != 0 || p.Dh > splitMaxD || p.Hkv > splitGridDimMax || p.Batch > splitGridDimMax {
		return flashSplit{}, false
	}
	a := pickSplit(p)
	if splitTemp(a, p) > splitTempMax || g.attnPath == "" && !g.splitPreferred(p) {
		return flashSplit{}, false
	}
	checkAttnSize(p, true)
	return a, true
}

// splitPreferred seçim kuralı: (a) sorgu satırları birleşik yolun karosunu çok boş bırakıyor, ya da
// (b) anahtar uzun ve birleşik yolun karoları karta az blok veriyor. Br birleşik yolun karo satırıdır.
func (g *backend) splitPreferred(p tensor.AttentionParams) bool {
	fused, ok := pickFlash(p.Dh, p.Dv, false, false, g.sr.sharedMemLimit())
	if !ok || p.Tq*(p.H/p.Hkv) > splitMaxRows {
		return false
	}
	few := p.Tq <= fused.Br/splitFewRows && p.Tk >= splitMinKeys
	long := p.Dh >= splitLongD && p.Tk >= splitLongKeys && int(blocks(p.Tq, fused.Br))*p.H*p.Batch <= p.Dh/2
	return few || long
}

// pickSplit ayarlar: R, G·Tq satıra göre 4, 8 ya da 16; warp sayısı R ile ters (paylaşımlı bellek
// 4·W·R·D ≤ 32 KB), warp başına anahtar 32 ya da (R = 16) 64. R = 4'te tek parçaya iki katı
// sığıyorsa KC iki katıdır: ikinci başlatmanın konak payı çekirdekteki kaybından büyüktür.
func pickSplit(p tensor.AttentionParams) flashSplit {
	a := flashSplit{R: 16, W: 4, KC: 64, D: p.Dh}
	switch rows := p.Tq * (p.H / p.Hkv); {
	case rows <= 4:
		a.R, a.W, a.KC = 4, 16, 32
		if p.Tk > a.part() && p.Tk <= 2*a.part() {
			a.KC = 64
		}
	case rows <= 8:
		a.R, a.W, a.KC = 8, 8, 32
	}
	a.single = p.Tk <= a.part()
	return a
}

// splitTemp birden çok parçada kısmi sonuçların (O, m, l) baytı; tek parçada 0.
func splitTemp(a flashSplit, p tensor.AttentionParams) int {
	if a.single {
		return 0
	}
	return 4 * p.Batch * p.H * p.Tq * int(blocks(p.Tk, a.part())) * (a.D + 2)
}

// attnSplit (q, k, v) → (çıktı, lse): tek parçada tek başlatma, yoksa parçalar ve birleştirme. Girdiler
// float32'dir; biri 16 bayta hizalı değilse yüklemeler skalerdir.
func (g *backend) attnSplit(gt *temp, a flashSplit, od, lse *storage, q, k, v tensor.View, md, mo *storage, p tensor.AttentionParams) {
	for _, x := range []tensor.View{q, k, v} {
		a.loose = a.loose || align(g.addressOf(x)) < 16
	}
	rows := p.Tq * (p.H / p.Hkv)
	RG, parts := int(blocks(rows, a.R)), int(blocks(p.Tk, a.part()))
	outD, stD := od, lse
	if !a.single {
		n := p.Batch * p.Hkv * rows * parts
		outD, stD = gt.buffer(n*a.D, false), gt.buffer(2*n, false)
	}
	c := g.takeKernel(a.key(), flashSplitKernel(a))
	pr := g.param().readV(q).readV(k).readV(v).write(outD, 0).write(stD, 0)
	g.attnShared(pr, md, mo, p)
	g.launch(c, [3]uint32{uint32(RG * parts), uint32(p.Hkv), uint32(p.Batch)}, [3]uint32{uint32(32 * a.W), 1, 1}, pr)
	if a.single {
		return
	}
	m := g.takeKernel(flashSplitMergeKey(a.D), flashSplitMergeKernel(a.D))
	g.launch(m, [3]uint32{uint32(p.Batch * p.Hkv * rows), 1, 1}, [3]uint32{32, 1, 1},
		g.param().read(outD, 0).read(stD, 0).write(od, 0).write(lse, 0).u32(p.Tq).u32(p.H).u32(p.H/p.Hkv).u32(parts))
}
