package gpu

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Kaynaşmış eleman bazlı program (tensor.FuseNode, tensor/fuse.go)
// ============================================================================
//
// Programın bir çıktısı tek çekirdektir: iş parçacığı başına bir çıktı elemanı. Çıktının çok
// indeksi doğrusal indeksten (şekiller çekirdeğe gömülüdür) çıkarılır; düğümler bu indeksle
// ve görünümlerin eşlemeleriyle yazmaçlarda hesaplanır, ara sonuç belleğe yazılmaz. Girdiler
// adımlarıyla okunur (adımlar parametredir: aynı program her düzende aynı çekirdektir).
// Eleman işlemleri kaynaşmamış çekirdeklerin ifadeleridir (unaryExpr, unaryGrad,
// binaryExpr) ve her düğüm kendi türüne yuvarlanır: sonuç kaynaşmamış yolla bit düzeyinde
// aynıdır.

var _ tensor.Fuser = (*backend)(nil)

// fusedInputs düğümün okuduğu girdilerin indeksleri (artan, birer kez).
func fusedInputs(n *tensor.FuseNode) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range n.Nodes() {
		if x.Kind == tensor.FuseInput && !seen[x.Index] {
			seen[x.Index] = true
			out = append(out, x.Index)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// fusedName programın çekirdek adı (anahtarın özeti; ad yalnızca profilde görünür).
func fusedName(key string) string {
	h := fnv.New64a()
	h.Write([]byte(key))
	return fmt.Sprintf("kaynasik_%016x", h.Sum64())
}

// RunFused (tensor.Fuser) out = node'un değeri.
func (g *backend) RunFused(node *tensor.FuseNode, out tensor.Storage, ins []tensor.View) {
	g.enter()
	defer g.exit()
	o := g.storageOf(out)
	n := numel(node.Shape)
	if n == 0 {
		return
	}
	checkSize(n)
	used := fusedInputs(node)
	ranks := make([]int, len(used))
	var key strings.Builder
	key.WriteString(node.Key())
	for i, k := range used {
		ranks[i] = len(ins[k].Shape)
		fmt.Fprintf(&key, "|%d:%s:%d", k, g.storageOf(ins[k].S).dtype, ranks[i])
	}
	c := g.takeKernel("kaynasik:"+key.String(), fusedKernel(node, used, ranks))
	p := g.param().write(o, 0)
	for i, k := range used {
		v := ins[k]
		p.readV(v).arr(max(ranks[i], 1), v.Strides...)
	}
	p.u32(n)
	iz, bl := plain(n, sameBlock)
	g.launch(c, iz, bl, p)
}

// fusedKernel düğümün çekirdeği; used okunan girdiler, ranks eksen sayıları.
func fusedKernel(node *tensor.FuseNode, used, ranks []int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fusedName(node.Key()))
		out := k.written("cikti")
		f := &fusedEmit{k: k, memo: map[string]reg{}, base: map[int]reg{}, strides: map[int][]string{}}
		for i, x := range used {
			f.base[x] = k.pointer(fmt.Sprintf("girdi%d", x))
			ad := fmt.Sprintf("adim%d", x)
			k.arr(ad, max(ranks[i], 1))
			f.strides[x] = make([]string, ranks[i])
			for d := range f.strides[x] {
				f.strides[x][d] = ad
			}
			f.ranks = append(f.ranks, ranks[i])
		}
		n := k.u32p("n")
		// Adımlar başta yüklenir: ilk kullanım bir dalın içinde olabilir (birleştir), öbür dal
		// yüklenmemiş yazmacı okumasın.
		for _, x := range used {
			for d := range f.strides[x] {
				f.strideOf(x, d)
			}
		}
		i := k.globalIndex()
		last := k.bounded(i, n)
		v := f.emit(node, f.multiIndex(i, node.Shape))
		k.store(node.DType, k.address(out, i, dtypeSize(node.DType)), v)
		k.put(last)
		return k
	}
}

// fusedEmit bir kaynaşmış çekirdeğin yazıcısı.
type fusedEmit struct {
	k       *emitter
	memo    map[string]reg // düğüm + indeks yazmaçları → değer
	base    map[int]reg    // girdi → gösterici
	strides map[int][]string
	stride  map[string]reg // yüklenmiş adımlar ("adimK:d"); çekirdeğin başında
	ranks   []int
}

// multiIndex doğrusal indeksi shape'in çok indeksine ayırır.
func (f *fusedEmit) multiIndex(i reg, shape []int) []reg {
	k := f.k
	idx := make([]reg, len(shape))
	rest := i
	for d := len(shape) - 1; d >= 0; d-- {
		if d == 0 {
			idx[d] = rest
			break
		}
		r, q := k.r(), k.r()
		k.e("rem.u32", r, rest, shape[d])
		k.e("div.u32", q, rest, shape[d])
		idx[d], rest = r, q
	}
	return idx
}

// round v'yi dt türüne yuvarlar (16 bitlik türlerde gidip gelerek).
func (f *fusedEmit) round(dt tensor.DType, v reg) reg {
	k := f.k
	switch dt {
	case tensor.Float32:
		return v
	case tensor.Float16:
		h, r := k.h(), k.f()
		k.e("cvt.rn.f16.f32", h, v)
		k.e("cvt.f32.f16", r, h)
		return r
	}
	b := k.bf16Bits(v)
	k.e("shl.b32", b, b, 16)
	r := k.f()
	k.e("mov.b32", r, b)
	return r
}

func (f *fusedEmit) strideOf(x, d int) reg {
	key := fmt.Sprintf("%d:%d", x, d)
	if f.stride == nil {
		f.stride = map[string]reg{}
	}
	if r, ok := f.stride[key]; ok {
		return r
	}
	r := f.k.readArr(f.strides[x][d], d)
	f.stride[key] = r
	return r
}

func memoKey(n *tensor.FuseNode, idx []reg) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%p", n)
	for _, r := range idx {
		b.WriteByte(',')
		b.WriteString(string(r))
	}
	return b.String()
}

// emit n'nin idx indeksindeki değerini (n'nin türüne yuvarlanmış float32) verir.
func (f *fusedEmit) emit(n *tensor.FuseNode, idx []reg) reg {
	key := memoKey(n, idx)
	if r, ok := f.memo[key]; ok {
		return r
	}
	r := f.compute(n, idx)
	f.memo[key] = r
	return r
}

func (f *fusedEmit) compute(n *tensor.FuseNode, idx []reg) reg {
	k := f.k
	switch n.Kind {
	case tensor.FuseInput:
		off := k.whole(0)
		for d, j := range idx {
			k.e("mad.lo.s32", off, j, f.strideOf(n.Index, d), off)
		}
		return k.load(n.DType, k.address(f.base[n.Index], off, dtypeSize(n.DType)))
	case tensor.FuseConst:
		return k.constant(n.P)
	case tensor.FuseUnary:
		mode2 := ""
		if n.Un == tensor.OpPow {
			mode2 = powMode(n.P)
		}
		x := f.emit(n.Args[0], idx)
		return f.round(n.DType, k.unaryExpr(n.Un, mode2, x, k.constant(n.P)))
	case tensor.FuseUnaryGrad:
		mode2 := ""
		if n.Un == tensor.OpPow {
			mode2 = powMode(n.P)
		}
		g, x, y := f.emit(n.Args[0], idx), f.emit(n.Args[1], idx), f.emit(n.Args[2], idx)
		return k.unaryGrad(n.Un, mode2, g, x, y, k.constant(n.P))
	case tensor.FuseBinary:
		a, b := f.emit(n.Args[0], idx), f.emit(n.Args[1], idx)
		return f.round(n.DType, k.binaryExpr(n.Bin, a, b))
	case tensor.FuseCast:
		return f.round(n.DType, f.emit(n.Args[0], idx))
	case tensor.FuseView:
		return f.emit(n.Args[0], f.viewIndex(n.Map, idx))
	case tensor.FuseConcat:
		return f.concat(n, idx)
	case tensor.FuseScatter:
		return f.scatter(n, idx)
	}
	panic(internalf("unknown fused node kind %d", n.Kind))
}

// viewIndex görünüm indeksinden kaynağın indeksini kurar: Off + Step·idx[Src].
func (f *fusedEmit) viewIndex(m *tensor.FuseMap, idx []reg) []reg {
	k := f.k
	out := make([]reg, len(m.Src))
	for s, a := range m.Src {
		if a < 0 {
			out[s] = k.whole(m.Off[s])
			continue
		}
		if m.Off[s] == 0 && m.Step[s] == 1 {
			out[s] = idx[a]
			continue
		}
		r := k.r()
		k.e("mad.lo.s32", r, idx[a], m.Step[s], k.whole(m.Off[s]))
		out[s] = r
	}
	return out
}

// branch body'yi kendi ezber kopyasıyla yazar: bir dalda hesaplanan değer diğerinde yoktur.
func (f *fusedEmit) branch(body func() reg) reg {
	saved := f.memo
	f.memo = make(map[string]reg, len(saved))
	for key, r := range saved {
		f.memo[key] = r
	}
	v := body()
	f.memo = saved
	return v
}

// concat indeksin düştüğü parçayı dallanarak okur.
func (f *fusedEmit) concat(n *tensor.FuseNode, idx []reg) reg {
	k := f.k
	res := k.f()
	end := k.label()
	off := 0
	for i, part := range n.Args {
		L := part.Shape[n.Axis]
		next := k.label()
		if i < len(n.Args)-1 {
			p := k.p()
			k.e("setp.ge.u32", p, idx[n.Axis], off+L)
			k.jumpIf(p, next)
		}
		pi := append([]reg{}, idx...)
		if off > 0 {
			r := k.r()
			k.e("sub.s32", r, idx[n.Axis], off)
			pi[n.Axis] = r
		}
		v := f.branch(func() reg { return f.emit(part, pi) })
		k.e("mov.f32", res, v)
		k.jump(end)
		k.put(next)
		off += L
	}
	k.put(end)
	return res
}

// scatter görünümün tersidir: kaynağın indeksine düşen görünüm elemanı varsa onun değeri,
// yoksa 0.
func (f *fusedEmit) scatter(n *tensor.FuseNode, idx []reg) reg {
	k := f.k
	m, vs := n.Map, n.Args[0].Shape
	res := k.constant(0)
	skip := k.label()
	vi := make([]reg, len(vs))
	for a := range vi {
		vi[a] = k.whole(0)
	}
	for s, a := range m.Src {
		r := k.r()
		k.e("sub.s32", r, idx[s], m.Off[s])
		p := k.p()
		if a < 0 {
			k.e("setp.ne.s32", p, r, 0)
			k.jumpIf(p, skip)
			continue
		}
		k.e("setp.lt.s32", p, r, 0)
		k.jumpIf(p, skip)
		q := r
		if m.Step[s] != 1 {
			rm := k.r()
			k.e("rem.u32", rm, r, m.Step[s])
			k.e("setp.ne.u32", p, rm, 0)
			k.jumpIf(p, skip)
			q = k.r()
			k.e("div.u32", q, r, m.Step[s])
		}
		k.e("setp.ge.u32", p, q, vs[a])
		k.jumpIf(p, skip)
		vi[a] = q
	}
	v := f.branch(func() reg { return f.emit(n.Args[0], vi) })
	k.e("mov.f32", res, v)
	k.put(skip)
	return res
}
