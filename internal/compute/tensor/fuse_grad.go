package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaynaşmış programın geri yönü (fuse.go): ters kipte simgesel türev. Çıktı gradyanları
// programın yeni girdileridir (Index = len(Inputs) + çıktı sırası; türleri çıktının
// türüdür). Her düğümün gradyanı float32 bir ifadedir ve düğümün şeklindedir; birden çok
// kullanılan düğümün gradyanları toplanır. Ara değerler saklanmaz: gradyan ifadeleri ileri
// düğümleri yeniden okur (geri geçişte girdilerden yeniden hesaplanır). Tekli işlemlerin
// türevi arka ucun UnaryGrad'ıyla aynı ifadedir (FuseUnaryGrad). Girdinin gradyanı float32
// verilir; dış düğüm onu girdinin gradyan türüne ekler (CustomOp, AddTo).

// fuseGradients p'nin türev isteyen her girdisinin gradyan düğümünü verir.
func fuseGradients(p *FuseProgram) []*FuseNode {
	var order []*FuseNode
	seen := map[*FuseNode]bool{}
	for _, o := range p.Outputs {
		for _, n := range o.Nodes() {
			if !seen[n] {
				seen[n] = true
				order = append(order, n)
			}
		}
	}
	wants := gradReach(order, p.Inputs)
	adj := map[*FuseNode]*FuseNode{}
	add := func(n, g *FuseNode) {
		if !wants[n] || g == nil {
			return
		}
		if old := adj[n]; old != nil {
			g = &FuseNode{Kind: FuseBinary, Bin: OpAdd, Shape: n.Shape, DType: Float32, Args: []*FuseNode{old, g}}
		}
		adj[n] = g
	}
	for i, o := range p.Outputs {
		add(o, fuseInput(len(p.Inputs)+i, o.Shape, o.DType))
	}
	grads := make([]*FuseNode, len(p.Inputs))
	for i := len(order) - 1; i >= 0; i-- {
		n := order[i]
		g := adj[n]
		if g == nil {
			continue
		}
		if n.Kind == FuseInput {
			if n.Index < len(p.Inputs) {
				grads[n.Index] = fuseCast(g, Float32)
			}
			continue
		}
		for k, c := range gradTerms(n, g) {
			add(n.Args[k], c)
		}
	}
	for k, in := range p.Inputs {
		if in.RequiresGrad && grads[k] == nil {
			grads[k] = fuseConst(0, in.Shape) // çıktıya ulaşmayan girdinin gradyanı 0'dır
		}
	}
	return grads
}

// gradReach türev isteyen bir girdiye bağlı düğümleri işaretler.
func gradReach(order []*FuseNode, inputs []FuseInputInfo) map[*FuseNode]bool {
	wants := map[*FuseNode]bool{}
	for _, n := range order {
		switch n.Kind {
		case FuseInput:
			wants[n] = n.Index < len(inputs) && inputs[n.Index].RequiresGrad
		default:
			for _, a := range n.Args {
				wants[n] = wants[n] || wants[a]
			}
		}
	}
	return wants
}

// fuseF32 a ve b'nin float32 ikili işlemidir (shape'te).
func fuseF32(op BinOp, a, b *FuseNode, shape []int) *FuseNode {
	return fuseBinary(op, a, b, shape, Float32)
}

// gradTerms n'nin gradyanı g'den argümanlarına düşen katkılardır (Args sırasıyla; nil:
// katkı yok).
func gradTerms(n, g *FuseNode) []*FuseNode {
	sh := n.Shape
	switch n.Kind {
	case FuseUnary:
		if unaryNoGrad(n.Un) {
			return []*FuseNode{nil}
		}
		return []*FuseNode{{Kind: FuseUnaryGrad, Un: n.Un, P: n.P, Shape: sh, DType: Float32, Args: []*FuseNode{g, n.Args[0], n}}}
	case FuseBinary:
		a, b := n.Args[0], n.Args[1]
		switch n.Bin {
		case OpAdd:
			return []*FuseNode{g, g}
		case OpSub:
			return []*FuseNode{g, fuseUnary(OpMulScalar, -1, g)}
		case OpMul:
			return []*FuseNode{fuseF32(OpMul, g, b, sh), fuseF32(OpMul, g, a, sh)}
		case OpDiv:
			// d(a/b)/db = -(a/b)/b = -y/b
			return []*FuseNode{fuseF32(OpDiv, g, b, sh), fuseUnary(OpMulScalar, -1, fuseF32(OpDiv, fuseF32(OpMul, g, n, sh), b, sh))}
		}
		return []*FuseNode{nil, nil} // karşılaştırmalar: türev 0
	case FuseCast:
		return []*FuseNode{g}
	case FuseView:
		return []*FuseNode{scatterOf(n, g)}
	case FuseConcat:
		out := make([]*FuseNode, len(n.Args))
		off := 0
		for i, part := range n.Args {
			m := identityMap(len(sh))
			m.Off[n.Axis] = off
			out[i] = fuseView(g, m, part.Shape)
			off += part.Shape[n.Axis]
		}
		return out
	}
	panic(messages.Internal("fused node has no derivative"))
}

// scatterOf görünüm düğümü v'nin gradyanını (v'nin şeklinde g) kaynağın şekline götürür.
// Görünümün her ekseni kaynağın en çok bir eksenine karşılık gelmelidir; kaynağı olmayan,
// boyu 1'den büyük bir eksen yayınlamadır ve gradyanı indirgeme ister (kaynaştırılmaz).
func scatterOf(v, g *FuseNode) *FuseNode {
	src := v.Args[0]
	used := make([]int, len(v.Shape))
	for _, a := range v.Map.Src {
		if a >= 0 {
			used[a]++
		}
	}
	for a, n := range used {
		if n > 1 || (n == 0 && v.Shape[a] > 1) {
			panic(messages.New(messages.H3109, src.Shape, v.Shape))
		}
	}
	return &FuseNode{Kind: FuseScatter, Shape: src.Shape, DType: Float32, Args: []*FuseNode{g}, Map: v.Map}
}
