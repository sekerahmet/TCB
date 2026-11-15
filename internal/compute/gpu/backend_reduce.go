package gpu

import (
	"fmt"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Arka uç: eksen indirgemeleri (Sum, SumAll, Max, MaxGrad, ArgMax).

// noAxis şeklin axis ekseni çıkarılmış hali ve adımları.
func noAxis(sh, st []int, axis int) ([]int, []int) {
	s := append(append([]int{}, sh[:axis]...), sh[axis+1:]...)
	a := append(append([]int{}, st[:axis]...), st[axis+1:]...)
	return s, a
}

func (g *backend) Sum(out tensor.Storage, x tensor.View, axis int, scale float32) {
	g.enter()
	defer g.exit()
	// Çıktı adımları: eksensiz bitişik düzen, indirgenen eksende 0.
	ks, _ := noAxis(x.Shape, x.Strides, axis)
	kst := tensor.ContiguousStrides(ks)
	st := append(append(append([]int{}, kst[:axis]...), 0), kst[axis:]...)
	g.reduce(reduceJob{a: g.storageOf(x.S), aOf: x.Offset, output: g.storageOf(out), shape2: x.Shape, stA2: x.Strides, st: st, term: tA, scaleF: scale, sum: true})
}

func (g *backend) SumAll(out tensor.Storage, x tensor.View, scale float32) {
	g.enter()
	defer g.exit()
	g.reduce(reduceJob{a: g.storageOf(x.S), aOf: x.Offset, output: g.storageOf(out), shape2: x.Shape, stA2: x.Strides,
		st: make([]int, len(x.Shape)), term: tA, scaleF: scale, sum: true})
}

// argmax çekirdeğini kurar ve başlatır: kip 0 değer, 1 indeks, 2 gradyan.
func (g *backend) argmax(mode2 int, out, gd *storage, x tensor.View, axis int, gxSt []int) {
	ks, kx := noAxis(x.Shape, x.Strides, axis)
	var kg []int
	if gxSt != nil {
		ks, kg = noAxis(x.Shape, gxSt, axis)
	} else {
		kg = make([]int, len(ks))
	}
	sh, st := tensor.MergeAxes(ks, kx, kg)
	K, L := tensor.NumelOf(sh), x.Shape[axis]
	if K == 0 || (mode2 == 2 && L == 0) {
		return
	}
	checkSize(K)
	if len(sh) > maxAxis {
		none(messages.H4054, len(sh), maxAxis)
	}
	sg := 0
	if gxSt != nil {
		sg = gxSt[axis]
	}
	meta := append(append(append(append([]int{}, sh...), st[0]...), st[1]...), L, x.Strides[axis], sg)
	p := g.param().readV(x).write(out, 0)
	if gd != nil {
		p.read(gd, 0)
	} else {
		p.empty()
	}
	p.u32(K).arr(len(meta), meta...)
	if B := argmaxBlocks(K, L, sh, st[0]); B > 0 {
		c := g.takeKernel(fmt.Sprintf("argmax_blok:%d:%d:%d", len(sh), mode2, B), argmaxBlockKernel(len(sh), mode2, B))
		g.launch(c, [3]uint32{uint32(K), 1, 1}, [3]uint32{uint32(B), 1, 1}, p)
		return
	}
	c := g.takeKernel(fmt.Sprintf("argmax:%d:%d", len(sh), mode2), argmaxKernel(len(sh), mode2))
	iz, bl := plain(K, sameBlock)
	g.launch(c, iz, bl, p)
}

// argmaxBlockMaxOut blok yolunun (argmaxBlockKernel) en çok çıktı sayısı; çok çıktılı
// argmax'ta çıktı başına iş parçacığı kartı zaten doldurur. Sınamalar eski yolu denemek için
// 0 yapar.
var argmaxBlockMaxOut = 4096

// argmaxBlocks argmax'ın blok yolunu seçer ve blok boyunu verir (0: çıktı başına iş
// parçacığı). Kural toplamanınkidir (reduce): eksen en az 64 elemanlı ve çıktılar çok sayıda
// bitişik sütun değil (korunan iç adım 1 ve K ≥ 256 iken iş parçacığı başına çıktı okumaları
// birleştirir); ek olarak K ≤ argmaxBlockMaxOut.
func argmaxBlocks(K, L int, sh, stX []int) int {
	col := len(sh) > 0 && stX[len(sh)-1] == 1 && K >= 256
	if K > argmaxBlockMaxOut || L < 64 || col {
		return 0
	}
	return powerOfTwo(L, 32, 256)
}

func (g *backend) Max(out tensor.Storage, x tensor.View, axis int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	o, last := gt.write(out, false)
	g.argmax(0, o, nil, gt.readStrided(x), axis, nil)
	last()
}

func (g *backend) MaxGrad(gx, gs tensor.Storage, x tensor.View, axis int) {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	d, last := gt.write(gx, true)
	g.argmax(2, d, gt.readD(gs), gt.readStrided(x), axis, tensor.ContiguousStrides(x.Shape))
	last()
}

func (g *backend) ArgMax(x tensor.View, axis int) []int {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	ks, _ := noAxis(x.Shape, x.Strides, axis)
	n := tensor.NumelOf(ks)
	tmp := gt.buffer(n, false)
	g.argmax(1, tmp, nil, gt.readStrided(x), axis, nil)
	f := make([]uint32, n)
	if n > 0 {
		g.download(unsafe.Pointer(&f[0]), tmp.addr, 4*n)
	}
	r := make([]int, n)
	for i, v := range f {
		r[i] = int(v)
	}
	return r
}
