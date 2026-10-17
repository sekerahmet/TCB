package tensor

import (
	"fmt"
	"sync/atomic"
)

// gradKapali sıfırdan büyükse işlemler hesap çizgesine kaydedilmez
// (türevsiz çalışma). Sayaç olduğu için iç içe NoGrad çağrıları güvenlidir.
// Durum süreç genelidir; eşzamanlı eğitim ve çıkarım aynı süreçte karışık
// yürütülecekse bu göz önünde tutulmalıdır.
var gradKapali atomic.Int32

// NoGrad fn'i türevsiz kipte çalıştırır: içinde üretilen tensörler çizgeye
// bağlanmaz ve gradyan istemez. Çıkarım ve iyileştirici adımları için.
func NoGrad(fn func()) {
	gradKapali.Add(1)
	defer gradKapali.Add(-1)
	fn()
}

// GradEnabled türevli kipte olup olmadığımızı bildirir.
func GradEnabled() bool { return gradKapali.Load() == 0 }

// track, girdilerden en az biri gradyan istiyorsa ve türevli kipteysek sonucu
// çizgeye bağlar. bw sonucun gradyanını girdilere dağıtır; çağrıldığında
// out.grad doludur.
func track(out *Tensor, op string, parents []*Tensor, bw func()) *Tensor {
	if !GradEnabled() {
		return out
	}
	need := false
	for _, p := range parents {
		if p != nil && p.requiresGrad {
			need = true
			break
		}
	}
	if !need {
		return out
	}
	out.requiresGrad = true
	out.parents = parents
	out.backward = bw
	out.op = op
	return out
}

// accGrad gradyan depolamasını (gerekirse sıfırlanmış olarak ayırarak) verir;
// çağıran arka uç işlemleriyle üzerine ekler.
func (t *Tensor) accGrad() Storage {
	if t.grad == nil {
		t.grad = t.be.Alloc(t.Numel())
	}
	return t.grad
}

// Backward tek elemanlı (skaler) bir sonuçtan geri yayılım yapar; tohum
// gradyanı 1'dir.
func (t *Tensor) Backward() {
	if t.Numel() != 1 {
		panic(fmt.Sprintf("tensor: Backward skaler sonuç ister, şekil %v; BackwardWith kullanın", t.shape))
	}
	t.BackwardWith(nil)
}

// BackwardWith verilen tohum gradyanıyla (sonuçla aynı şekilde, nil ise birler)
// geri yayılım yapar. Topolojik sıra yinelemeli (özyinelemesiz) derinlik
// öncelikli arama ile kurulur, böylece çok derin çizgelerde yığın taşmaz.
//
// Yapraklarda gradyanlar birikir (ZeroGrad ile sıfırlanır). Ara tensörlerin
// gradyanları ve çizge bağları kullanıldıktan sonra bellek için bırakılır;
// bu nedenle aynı çizge üzerinden ikinci kez geri yayılım yapılamaz.
func (t *Tensor) BackwardWith(seed *Tensor) {
	if !t.requiresGrad {
		panic("tensor: gradyan istemeyen bir tensörden geri yayılım yapılamaz")
	}
	// Önce çizgeyi kur: bırakılmış bir ara tensöre rastlanırsa gradyan
	// biriktirmeden hata ver (aksi halde yaprakların gradyanı sessizce eksik kalırdı).
	order := topoOrder(t)
	g := t.accGrad()
	if seed == nil {
		bir := t.be.Alloc(t.Numel())
		t.be.Fill(bir, 1)
		t.be.AddTo(g, bir, 1)
	} else {
		if !sameShape(seed.shape, t.shape) {
			panic(fmt.Sprintf("tensor: tohum gradyan şekli %v, sonuç şekli %v", seed.shape, t.shape))
		}
		sameDevice("BackwardWith", t, seed)
		t.be.AddTo(g, seed.denseStorage(), 1)
	}

	// order sonradan-sıralıdır (kök en sonda); tersten gezmek her düğümü,
	// ona bağlı tüm düğümler işlendikten sonra ziyaret eder.
	for i := len(order) - 1; i >= 0; i-- {
		n := order[i]
		if n.backward == nil {
			continue // yaprak
		}
		if n.grad != nil {
			n.backward()
		}
		n.grad = nil
		n.backward = nil
		n.parents = nil
		n.serbest = true
	}
}

// topoOrder kökten ulaşılabilen ve gradyan isteyen düğümleri sonradan-sıralı
// (önce girdiler) olarak, açık bir yığınla verir.
func topoOrder(root *Tensor) []*Tensor {
	type cerceve struct {
		t *Tensor
		i int
	}
	ikinciGeri := func(n *Tensor) {
		panic(fmt.Sprintf("tensor: %q işleminin sonucu üzerinden geri yayılım zaten yapıldı ve ara sonuçlar "+
			"bellek için bırakıldı; aynı çizgeden ikinci kez geri yayılım yapılamaz. Birden çok kayıp "+
			"varsa önce toplayıp tek seferde geri yayın", n.op))
	}
	if root.serbest {
		ikinciGeri(root)
	}
	visited := map[*Tensor]bool{root: true}
	var order []*Tensor
	stack := []cerceve{{root, 0}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.i < len(top.t.parents) {
			p := top.t.parents[top.i]
			top.i++
			if p != nil && p.requiresGrad && !visited[p] {
				if p.serbest {
					ikinciGeri(p)
				}
				visited[p] = true
				stack = append(stack, cerceve{p, 0})
			}
			continue
		}
		order = append(order, top.t)
		stack = stack[:len(stack)-1]
	}
	return order
}

// ZeroGrads verilen tensörlerin gradyanlarını sıfırlar.
func ZeroGrads(params []*Tensor) {
	for _, p := range params {
		p.ZeroGrad()
	}
}
