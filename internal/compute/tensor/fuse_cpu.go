package tensor

import (
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

func (l fuseLine) at(t int) []int {
	j := copyInts(l.base)
	if l.axis >= 0 {
		j[l.axis] += t * l.step
	}
	return j
}

// RunFused (Fuser) out = node'un değeri (add: out += değer); ins[k] k. girdinin görünümü.
func (cpuBackend) RunFused(node *FuseNode, out Storage, ins []View, add bool) {
	ev := &fuseEval{ins: ins}
	sh := node.Shape
	nd := len(sh)
	total := numelOf(sh)
	if total == 0 {
		return
	}
	row := 1
	if nd > 0 {
		row = sh[nd-1]
	}
	res := make([]float32, total)
	idx := make([]int, nd)
	for o := 0; o < total; o += row {
		l := fuseLine{base: copyInts(idx), axis: nd - 1, step: 1, n: row}
		if nd == 0 {
			l.axis = -1
		}
		copy(res[o:o+row], ev.eval(node, l))
		for d := nd - 2; d >= 0; d-- {
			if idx[d]++; idx[d] < sh[d] {
				break
			}
			idx[d] = 0
		}
	}
	c := cpuStore(out)
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
	if c.dtyp == Float32 {
		copy(c.d, res)
	} else {
		narrow(c.h, res, c.dtyp)
	}
}

type fuseEval struct {
	ins []View
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
	out := make([]float32, l.n)
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
		unaryGradLoop(n.Un, n.P, out, g, x, y)
	case FuseBinary:
		binFlat(n.Bin, out, ev.eval(n.Args[0], l), ev.eval(n.Args[1], l))
		roundTo(out, n.DType)
	case FuseCast:
		copy(out, ev.eval(n.Args[0], l))
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
	src := fuseLine{base: make([]int, len(m.Src)), axis: -1, n: 1}
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
		j := l.at(t)
		off := 0
		for _, part := range n.Args {
			L := part.Shape[k]
			if j[k] < off+L {
				pb := copyInts(j)
				pb[k] -= off
				cnt := 1
				if l.axis == k {
					cnt = min(l.n-t, (off+L-j[k]+l.step-1)/l.step)
				} else if l.axis >= 0 {
					cnt = l.n - t
				}
				pl := fuseLine{base: pb, axis: l.axis, step: l.step, n: cnt}
				if cnt == 1 {
					pl.axis = -1
				}
				copy(out[t:t+cnt], ev.eval(part, pl))
				t += cnt
				break
			}
			off += L
		}
	}
}

// scatter görünümün tersidir: kaynağın her elemanı, görünümde ona düşen eleman varsa onun
// gradyanını, yoksa 0'ı alır. Görünümün kaynağı olmayan eksenlerinin boyu 1'dir (scatterOf).
func (ev *fuseEval) scatter(out []float32, n *FuseNode, l fuseLine) {
	m, vs := n.Map, n.Args[0].Shape
	for t := range out {
		j := l.at(t)
		i := make([]int, len(vs))
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
			out[t] = ev.eval(n.Args[0], fuseLine{base: i, axis: -1, n: 1})[0]
		}
	}
}
