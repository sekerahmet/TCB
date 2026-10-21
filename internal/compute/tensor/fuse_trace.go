package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İz tensörleri (fuse.go): kaynaştırılan fonksiyon, girdilerinin yerine depolaması olmayan
// iz tensörleriyle çalışır. Desteklenen işlemler girişlerinde iz tensörünü görünce hesap
// yapmadan bir program düğümü kurar (sym* işlevleri); diğer her işlem verinin görünümünü
// (view) isteyince açık hata verir (H3107). İz tensörü ile gerçek tensör karışırsa fonksiyon
// dışarıdan bir tensör kullanıyordur (H3108). İzde çizge kurulmaz (geri yön programdan
// türetilir).

// symTensor n düğümünün iz tensörü.
func symTensor(be Backend, n *FuseNode) *Tensor {
	return newHeader(&Tensor{be: be, shape: copyInts(n.Shape), strides: contiguousStrides(n.Shape), dtyp: n.DType, sym: n})
}

// symOf t'nin düğümü; t iz tensörü değilse fonksiyon dışarıdan bir tensör kullanıyordur.
func symOf(t *Tensor) *FuseNode {
	if t.sym == nil {
		panic(messages.New(messages.H3108, t.shape))
	}
	return t.sym
}

func symBinary(a, b *Tensor, op BinOp, dt DType) *Tensor {
	shape := broadcastShapes(a.shape, b.shape)
	return symTensor(a.be, fuseBinary(op, symOf(a), symOf(b), shape, dt))
}

func symUnary(t *Tensor, op UnOp, p float32) *Tensor {
	return symTensor(t.be, fuseUnary(op, p, t.sym))
}

func symCast(t *Tensor, dt DType) *Tensor {
	return symTensor(t.be, fuseCast(t.sym, dt))
}

// symSlice t'nin axis ekseninde start'tan step adımla m eleman seçer.
func symSlice(t *Tensor, axis, start, m, step int) *Tensor {
	mp := identityMap(len(t.shape))
	mp.Off[axis], mp.Step[axis] = start, step
	sh := copyInts(t.shape)
	sh[axis] = m
	return symTensor(t.be, fuseView(t.sym, mp, sh))
}

// symPermute eksenleri dims sırasına dizer: görünümün i. ekseni kaynağın dims[i]. eksenidir.
func symPermute(t *Tensor, dims []int) *Tensor {
	mp := identityMap(len(t.shape))
	sh := make([]int, len(dims))
	for i, d := range dims {
		mp.Src[d] = i
		sh[i] = t.shape[d]
	}
	return symTensor(t.be, fuseView(t.sym, mp, sh))
}

// symReshape yalnızca boyu 1 eksen ekleyen ya da çıkaran yeniden şekillendirmedir: boyu 1
// olmayan eksenler aynı sırada kalır.
func symReshape(t *Tensor, sh []int) *Tensor {
	mp := identityMap(len(t.shape))
	j := 0
	for s, d := range t.shape {
		if d == 1 {
			mp.Src[s] = -1
			continue
		}
		for j < len(sh) && sh[j] == 1 {
			j++
		}
		if j == len(sh) || sh[j] != d {
			panic(messages.New(messages.H3110, t.shape, sh))
		}
		mp.Src[s] = j
		j++
	}
	for ; j < len(sh); j++ {
		if sh[j] != 1 {
			panic(messages.New(messages.H3110, t.shape, sh))
		}
	}
	return symTensor(t.be, fuseView(t.sym, mp, sh))
}

// symConcat parçaları axis boyunca birleştirir (tür kaynaşmamış yoldaki gibi ortak türdür).
func symConcat(ts []*Tensor, axis int, shape []int, dt DType) *Tensor {
	n := &FuseNode{Kind: FuseConcat, Axis: axis, Shape: copyInts(shape), DType: dt}
	for _, t := range ts {
		n.Args = append(n.Args, fuseCast(symOf(t), dt))
	}
	return symTensor(ts[0].be, n)
}

// anySym ts'den biri iz tensörü mü.
func anySym(ts ...*Tensor) bool {
	for _, t := range ts {
		if t.sym != nil {
			return true
		}
	}
	return false
}

// TraceFused fn'i girdilerin şekil ve türündeki iz tensörleriyle çalıştırıp programını
// kurar; türev isteyen girdi varsa geri yönü de türetir. fn'in döndürdüğü her tensör bir
// iz tensörü olmalıdır.
func TraceFused(be Backend, inputs []FuseInputInfo, fn func(args []*Tensor) []*Tensor) *FuseProgram {
	args := make([]*Tensor, len(inputs))
	for k, in := range inputs {
		args[k] = symTensor(be, fuseInput(k, in.Shape, in.DType))
	}
	var outs []*Tensor
	NoGrad(func() { outs = fn(args) })
	p := &FuseProgram{Inputs: inputs}
	for _, o := range outs {
		p.Outputs = append(p.Outputs, symOf(o))
	}
	for _, in := range inputs {
		if in.RequiresGrad {
			p.grads = fuseGradients(p)
			break
		}
	}
	return p
}

// runFused düğümü arka uçta yürütür (add: out'a ekler).
func runFused(be Backend, n *FuseNode, out Storage, ins []View, add bool) {
	f, ok := be.(Fuser)
	if !ok {
		panic(messages.Internal("backend cannot run fused programs"))
	}
	f.RunFused(n, out, ins, add)
}

// Run programı gerçek girdilerle yürütür: her çıktı tek geçişte hesaplanır. Türevli kipte ve
// türev isteyen girdi varsa çıktılar tek bir dış düğümle çizgeye bağlanır; geri yön ara
// değerleri saklamaz, gradyanları girdilerden yeniden hesaplar.
func (p *FuseProgram) Run(op string, inputs []*Tensor) []*Tensor {
	if len(inputs) != len(p.Inputs) {
		panic(messages.Internal("fused program called with a different input count"))
	}
	be := sameDevice(op, inputs...)
	views := make([]View, len(inputs))
	for k, t := range inputs {
		views[k] = t.view()
	}
	outs := make([]*Tensor, len(p.Outputs))
	for i, n := range p.Outputs {
		out := emptyOf(isElementwise, be, n.Shape, n.DType)
		runFused(be, n, out.storage, views, false)
		outs[i] = out
	}
	if p.grads == nil || !GradEnabled() {
		return outs
	}
	if len(outs) == 1 {
		return []*Tensor{p.trackSingle(inputs, outs[0], views, be)}
	}
	return CustomOp(op, inputs, outs, inputs, func(_ any, grads []*Tensor) []*Tensor {
		return p.backward(be, views, grads)
	})
}

// trackSingle tek çıktılı programın sonucunu kendi düğümüyle çizgeye bağlar: geri hesap her
// girdinin gradyanını, onun gradyan türünde doğrudan yazar (ilk gradyansa üzerine, değilse
// ekleyerek); ara tampon ve ayrı ekleme yoktur. Girdiler görünümleriyle tutulur (kayıt, save).
func (p *FuseProgram) trackSingle(inputs []*Tensor, out *Tensor, views []View, be Backend) *Tensor {
	return track(out, "fused", inputs, func(o *node) {
		ins := append(append([]View{}, views...), o.gradView())
		for k, g := range p.grads {
			in := o.parents[k]
			if g == nil || !in.wants() {
				continue
			}
			dst, fresh := in.freshGrad(true)
			runFused(be, g, dst, ins, !fresh)
		}
	}, inputs...)
}

// backward çıktı gradyanlarından girdilerin gradyanlarını hesaplar (girdiler görünümleriyle
// tutulur: geri hesap tensör başlığı yakalamaz).
func (p *FuseProgram) backward(be Backend, views []View, grads []*Tensor) []*Tensor {
	ins := append([]View{}, views...)
	for _, g := range grads {
		ins = append(ins, g.view())
	}
	out := make([]*Tensor, len(p.Inputs))
	for k, n := range p.grads {
		if n == nil {
			continue
		}
		gt := emptyOf(isElementwise, be, n.Shape, n.DType)
		runFused(be, n, gt.storage, ins, false)
		out[k] = gt
	}
	return out
}
