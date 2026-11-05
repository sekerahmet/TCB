package tensor

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// CPU'da kaynaşmış program (fuse.go): çıktı, son eksen boyunca satır satır hesaplanır. Bir
// düğüm bir "çizgide" (bir başlangıç indeksinden bir eksen boyunca adımla n eleman)
// değerlendirilir; görünümler çizgiyi kaynağın çizgisine çevirir. Eleman işlevleri kaynaşmamış
// yolunkilerdir (unaryLoop, unaryGradLoop, binFlat, narrow/widen): sonuç bit düzeyinde aynıdır.

// fuseLine bir düğümün indeks uzayında base'den axis ekseni boyunca step adımla n eleman.
// axis < 0 tek elemandır (n = 1).
type fuseLine struct {
	base          []int
	axis, step, n int
}

// at çizginin t. elemanının indeksidir; yeri ev'in yığınıdır.
func (l fuseLine) at(ev *fuseEval, t int) []int {
	j := ev.takeInts(len(l.base))
	copy(j, l.base)
	if l.axis >= 0 {
		j[l.axis] += t * l.step
	}
	return j
}

// RunFused (Fuser) out = node'un değeri (add: out += değer); ins[k] k. girdinin görünümü.
func (cpuBackend) RunFused(node *FuseNode, out Storage, ins []View, add bool) {
	total := NumelOf(node.Shape)
	if total == 0 {
		return
	}
	c := cpuStore(out)
	// Çıktı girdilerden biri değilse satırlar doğrudan ona yazılır: sonuç için ikinci tampon, prompt
	// işlemede katman başına onlarca megabayt çöp demektir.
	res := c.d
	if add || c.dtyp != Float32 || aliases(ins, out) {
		res = make([]float32, total)
	}
	fuseParts(res, node, ins)
	if add {
		old := make([]float32, total)
		if c.dtyp == Float32 {
			copy(old, c.d)
		} else {
			widen16(old, c.h, c.dtyp)
		}
		for i := range res {
			res[i] += old[i]
		}
	}
	if c.dtyp != Float32 {
		narrow(c.h, res, c.dtyp)
	} else if &res[0] != &c.d[0] {
		copy(c.d, res)
	}
}

// fuseChunk bir parçanın eleman sayısı, fuseCost bir elemanın göreli maliyetidir. İkisi de tahmindir,
// ölçülmedi. Parça sınırları yalnız boya bağlıdır, işçi sayısına ve zamanlamaya değil.
var fuseChunk = 512

const fuseCost = 8

// fuseParts res = node'un değeri. Parçalar işçilere dağılır; bir satır iki parçaya düşebilir: tek
// satırlı swiglu ancak böyle bölünür. Eleman işlevleri konumdan bağımsızdır, bitler değişmez
// [test: TestFusedChunksKeepBits].
func fuseParts(res []float32, node *FuseNode, ins []View) {
	total := len(res)
	parts := (total + fuseChunk - 1) / fuseChunk
	w := workerCount(total*fuseCost, fuseChunk*fuseCost, elemBigJob)
	distribute(w, parts, func(_, c int) {
		fuseRange(res, node, ins, c*fuseChunk, min((c+1)*fuseChunk, total))
	})
}

// fuseRange res[lo:hi]'yi satır satır hesaplar.
func fuseRange(res []float32, node *FuseNode, ins []View, lo, hi int) {
	ev := fuseEvals.Get().(*fuseEval)
	ev.ins, ev.fTop, ev.iTop = ins, 0, 0
	defer func() {
		ev.ins = nil
		fuseEvals.Put(ev)
	}()
	sh := node.Shape
	nd := len(sh)
	idx := ev.takeInts(nd)
	for d, o := nd-1, lo; d >= 0; d-- {
		idx[d] = o % sh[d]
		o /= sh[d]
	}
	for o := lo; o < hi; {
		n := hi - o
		l := fuseLine{base: idx, axis: -1, n: 1}
		if nd > 0 {
			n = min(n, sh[nd-1]-idx[nd-1])
			l.axis, l.step, l.n = nd-1, 1, n
		}
		ev.evalInto(res[o:o+n], node, l)
		o += n
		if nd > 0 {
			idx[nd-1] = 0
		}
		for d := nd - 2; d >= 0; d-- {
			if idx[d]++; idx[d] < sh[d] {
				break
			}
			idx[d] = 0
		}
	}
}

// aliases s'nin girdilerden birinin deposu olduğunu söyler.
func aliases(ins []View, s Storage) bool {
	for _, v := range ins {
		if v.S == s {
			return true
		}
	}
	return false
}

// fuseEval değerlendirmenin durumudur. Çizgiler onun iki yığınındadır: küçük ve kısa ömürlü çok dilim
// var, çöp toplayıcıya gitmezler. Düğüm sonucu üstte kalır, çağıran işi bitince yığını geri alır.
// Yığın dolarsa yenisi ayrılır: eski dilimler canlı kaldıkça durur, üstüne yazılmaz.
type fuseEval struct {
	ins    []View
	floats []float32
	fTop   int
	ints   []int
	iTop   int
}

var fuseEvals = sync.Pool{New: func() any { return &fuseEval{} }}

// takeFloats n elemanlık yer verir; içeriği belirsizdir.
func (ev *fuseEval) takeFloats(n int) []float32 {
	if ev.fTop+n > len(ev.floats) {
		ev.floats, ev.fTop = make([]float32, 2*(len(ev.floats)+n)), 0
	}
	s := ev.floats[ev.fTop : ev.fTop+n : ev.fTop+n]
	ev.fTop += n
	return s
}

// takeInts n indekslik yer verir; içeriği belirsizdir.
func (ev *fuseEval) takeInts(n int) []int {
	if ev.iTop+n > len(ev.ints) {
		ev.ints, ev.iTop = make([]int, 2*(len(ev.ints)+n)), 0
	}
	s := ev.ints[ev.iTop : ev.iTop+n : ev.iTop+n]
	ev.iTop += n
	return s
}

// evalInto n'in çizgisini dst'ye yazar, yığınları olduğu gibi bırakır.
func (ev *fuseEval) evalInto(dst []float32, n *FuseNode, l fuseLine) {
	f, i := ev.fTop, ev.iTop
	copy(dst, ev.eval(n, l))
	ev.fTop, ev.iTop = f, i
}

// round değerleri dt türüne yuvarlar (16 bitlik türlerde gidip gelerek).
func roundTo(v []float32, dt DType) {
	if dt == Float32 {
		return
	}
	for i, x := range v {
		h := narrowOne(x, dt)
		if dt == BFloat16 {
			v[i] = bf16ToF32(h)
		} else {
			v[i] = f16ToF32(h)
		}
	}
}

func (ev *fuseEval) eval(n *FuseNode, l fuseLine) []float32 {
	out := ev.takeFloats(l.n)
	f, i := ev.fTop, ev.iTop
	switch n.Kind {
	case FuseInput:
		ev.load(out, ev.ins[n.Index], l)
	case FuseConst:
		for i := range out {
			out[i] = n.P
		}
	case FuseUnary:
		unaryLoop(n.Un, n.P, out, ev.eval(n.Args[0], l))
		roundTo(out, n.DType)
	case FuseUnaryGrad:
		g, x, y := ev.eval(n.Args[0], l), ev.eval(n.Args[1], l), ev.eval(n.Args[2], l)
		clear(out) // gradyan çevrimi ekler
		unaryGradLoop(n.Un, n.P, out, g, x, y)
	case FuseBinary:
		binFlat(n.Bin, out, ev.eval(n.Args[0], l), ev.eval(n.Args[1], l))
		roundTo(out, n.DType)
	case FuseCast:
		ev.evalInto(out, n.Args[0], l)
		roundTo(out, n.DType)
	case FuseView:
		ev.view(out, n, l)
	case FuseConcat:
		ev.concat(out, n, l)
	case FuseScatter:
		ev.scatter(out, n, l)
	default:
		panic(messages.Internal("unknown fused node"))
	}
	ev.fTop, ev.iTop = f, i
	return out
}

// load girdinin çizgisini float32 olarak okur.
func (ev *fuseEval) load(out []float32, v View, l fuseLine) {
	off := v.Offset
	for d, j := range l.base {
		off += j * v.Strides[d]
	}
	st := 0
	if l.axis >= 0 {
		st = l.step * v.Strides[l.axis]
	}
	c := cpuStore(v.S)
	for i := range out {
		p := off + i*st
		switch c.dtyp {
		case Float32:
			out[i] = c.d[p]
		case BFloat16:
			out[i] = bf16ToF32(c.h[p])
		default:
			out[i] = f16ToF32(c.h[p])
		}
	}
}

// view görünümün çizgisini kaynağın çizgisine çevirir; çizginin ekseni kaynakta yoksa
// (yayınlama) kaynağın tek elemanı çoğaltılır.
func (ev *fuseEval) view(out []float32, n *FuseNode, l fuseLine) {
	m := n.Map
	src := fuseLine{base: ev.takeInts(len(m.Src)), axis: -1, n: 1}
	for s, a := range m.Src {
		src.base[s] = m.Off[s]
		if a >= 0 {
			src.base[s] += m.Step[s] * l.base[a]
			if a == l.axis {
				src.axis, src.step, src.n = s, m.Step[s]*l.step, l.n
			}
		}
	}
	v := ev.eval(n.Args[0], src)
	if src.n == l.n {
		copy(out, v)
		return
	}
	for i := range out {
		out[i] = v[0]
	}
}

// concat çizgiyi parçalara böler: çizgi birleştirme ekseni boyuncaysa her parçanın düştüğü
// aralık o parçadan, değilse yalnızca indeksin düştüğü parçadan okunur.
func (ev *fuseEval) concat(out []float32, n *FuseNode, l fuseLine) {
	k := n.Axis
	for t := 0; t < l.n; {
		mark := ev.iTop
		j := l.at(ev, t)
		off := 0
		for _, part := range n.Args {
			L := part.Shape[k]
			if j[k] < off+L {
				cnt := 1
				if l.axis == k {
					cnt = min(l.n-t, (off+L-j[k]+l.step-1)/l.step)
				} else if l.axis >= 0 {
					cnt = l.n - t
				}
				j[k] -= off
				pl := fuseLine{base: j, axis: l.axis, step: l.step, n: cnt}
				if cnt == 1 {
					pl.axis = -1
				}
				ev.evalInto(out[t:t+cnt], part, pl)
				t += cnt
				break
			}
			off += L
		}
		ev.iTop = mark
	}
}

// scatter görünümün tersidir: kaynağın her elemanı, görünümde ona düşen eleman varsa onun
// gradyanını, yoksa 0'ı alır. Görünümün kaynağı olmayan eksenlerinin boyu 1'dir (scatterOf).
func (ev *fuseEval) scatter(out []float32, n *FuseNode, l fuseLine) {
	m, vs := n.Map, n.Args[0].Shape
	clear(out)
	for t := range out {
		mark := ev.iTop
		j := l.at(ev, t)
		i := ev.takeInts(len(vs))
		clear(i)
		ok := true
		for s, a := range m.Src {
			r := j[s] - m.Off[s]
			if a < 0 {
				ok = ok && r == 0
				continue
			}
			if r < 0 || r%m.Step[s] != 0 || r/m.Step[s] >= vs[a] {
				ok = false
				continue
			}
			i[a] = r / m.Step[s]
		}
		if ok {
			ev.evalInto(out[t:t+1], n.Args[0], fuseLine{base: i, axis: -1, n: 1})
		}
		ev.iTop = mark
	}
}
