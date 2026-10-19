package tensor

import (
	"fmt"
	"sync/atomic"
)

// GradContext bir yürütme bağlamının (ör. bir yorumlayıcı görevinin) türevsiz kip
// durumudur. Her an tek bir bağlam etkindir (SetGradContext); NoGrad ve GradEnabled etkin
// bağlamı kullanır. Bağlam verilmemişse süreç geneli varsayılan bağlam etkindir; tek
// goroutine'li ya da bağlam değiştirmeyen Go programları için davranış süreç genelidir.
//
// Bir yorumlayıcı, görevler arasında yürütmeyi devrederken (ör. yorumlayıcı kilidi el
// değiştirdiğinde) kilidi alan görevin bağlamını etkin yapar; böylece bir görevin türevsiz
// bloğu, o blok sürerken çalışan başka görevin işlemlerini çizge dışında bırakmaz. NoGrad
// girişte etkin olan bağlamı yakalar ve çıkışta aynı bağlamı geri sayar; arada etkin
// bağlam değişip geri gelse de sayaç tutarlı kalır.
type GradContext struct {
	// closed sıfırdan büyükse işlemler hesap çizgesine kaydedilmez. Sayaç olduğu için
	// iç içe NoGrad çağrıları güvenlidir.
	closed atomic.Int32
}

// NewGradContext türevli kipte yeni bir bağlam üretir.
func NewGradContext() *GradContext { return &GradContext{} }

// Enabled bağlamın türevli kipte olup olmadığı.
func (c *GradContext) Enabled() bool { return c.closed.Load() == 0 }

var (
	defaultGrad GradContext
	activeGrad  atomic.Pointer[GradContext]
)

// activeContext etkin türev bağlamını verir.
func activeContext() *GradContext {
	if c := activeGrad.Load(); c != nil {
		return c
	}
	return &defaultGrad
}

// SetGradContext c'yi etkin türev bağlamı yapar (nil: süreç geneli varsayılan) ve önceki
// etkin bağlamı verir.
func SetGradContext(c *GradContext) *GradContext {
	prev := activeContext()
	if c == &defaultGrad {
		c = nil
	}
	activeGrad.Store(c)
	return prev
}

// NoGrad fn'i etkin bağlamda türevsiz kipte çalıştırır: içinde üretilen tensörler
// çizgeye bağlanmaz ve gradyan istemez. Çıkarım ve iyileştirici adımları için.
func NoGrad(fn func()) {
	c := activeContext()
	c.closed.Add(1)
	defer c.closed.Add(-1)
	fn()
}

// GradEnabled etkin bağlamın türevli kipte olup olmadığını bildirir.
func GradEnabled() bool { return activeContext().closed.Load() == 0 }

// savedVersion ileri hesapta kaydedilmiş (geri yayılımın okuyacağı) bir
// tensörün o andaki depolama sürümü.
type savedVersion struct {
	t *Tensor
	v uint64
}

// track, girdilerden en az biri gradyan istiyorsa ve türevli kipteysek sonucu
// çizgeye bağlar. bw sonucun gradyanını girdilere dağıtır; çağrıldığında
// out.grad doludur. kaydet, geri yayılımın verisini okuyacağı tensörlerdir
// (nil olanlar atlanır); sürümleri kaydedilir ve geri yayılımdan önce
// yerinde değiştirilmedikleri denetlenir.
func track(out *Tensor, op string, parents []*Tensor, bw func(), save ...*Tensor) *Tensor {
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
	out.opKind = activeKind() // düğümün profil sayacı türü (bkz. sayac.go)
	for _, k := range save {
		if k != nil {
			out.registered = append(out.registered, savedVersion{k, k.storage.Version()})
		}
	}
	return out
}

// trackPass, gradyanı tek girdisine olduğu gibi aktaran (dönüşüm, görünüm, kopya)
// bir işlemi track ile kaydeder ve çıktıyı işaretler: çıktının gradyanı girdinin
// gradyan türünde tutulur (bkz. gradDType).
func trackPass(out *Tensor, op string, t *Tensor, bw func()) *Tensor {
	out.passGrad = true
	return track(out, op, []*Tensor{t}, bw)
}

// gradDType gradyan depolamasının türü:
//   - float32 tensörde float32;
//   - 16 bitlik yaprakta (parametre) GradDType (varsayılan float32);
//   - gradyanı girdisine olduğu gibi aktaran işlemin çıktısında girdinin gradyan
//     türü (ör. float32 ağırlığın bfloat16 dönüşümünün gradyanı float32'dir; çarpımın
//     geri yönü ağırlık gradyanını yuvarlamadan float32 yazar);
//   - diğer 16 bitlik ara sonuçlarda kendi türü (PyTorch autocast gibi; geri yöndeki
//     çarpımlar gelen gradyanı dönüştürmeden 16 bitlik girdiyle yapılır).
func (t *Tensor) gradDType() DType {
	for {
		switch {
		case t.dtyp == Float32:
			return Float32
		case t.IsLeaf():
			return GradDType()
		case !t.passGrad:
			return t.dtyp
		}
		t = t.parents[0]
	}
}

// checkVersion, çizgedeki her düğümün kaydettiği girdilerin ileri hesaptan bu
// yana yerinde değiştirilmediğini denetler.
func checkVersion(order []*Tensor) {
	for _, n := range order {
		for _, k := range n.registered {
			if v := k.t.storage.Version(); v != k.v {
				panic(fmt.Sprintf("tensor: %q işleminin ileri hesapta kaydettiği bir girdi (şekil %v) ileri hesaptan "+
					"sonra yerinde değiştirildi; geri yayılım yanlış sonuç verirdi (sürüm %d → %d; ör. iyileştirici "+
					"adımı, Set ya da gradyanın değişmesi: yeni bir geri yayılımın birikimi, gradyanların sıfırlanması ya da kırpılması). Yerinde değişikliği geri yayılımdan sonraya alın ya da girdinin kopyasını "+
					"(Clone) kullanın", n.op, k.t.shape, k.v, v))
			}
		}
	}
}

// freshGrad gradyan henüz yoksa ve destek doğruysa (arka uç üzerine yazan biçimi veriyorsa)
// sıfırlanmamış gradyan depolaması ayırıp doğru döner: çağıran her elemanı yazmalıdır.
// Gradyan varsa (birikim) ya da destek yoksa accGrad'ın depolamasını ve yanlış verir.
func (t *Tensor) freshGrad(support bool) (Storage, bool) {
	if t.grad != nil || !support {
		return t.accGrad(), false
	}
	dt := t.gradDType()
	t.grad = allocUninit(t.be, t.Numel(), dt)
	counterBytes(t.be, dt.StorageBytes(t.Numel()))
	return t.grad, true
}

// accGrad gradyan depolamasını (gerekirse sıfırlanmış olarak ayırarak) verir;
// çağıran arka uç işlemleriyle üzerine ekler.
func (t *Tensor) accGrad() Storage {
	if t.grad == nil {
		dt := t.gradDType()
		t.grad = t.be.Alloc(t.Numel(), dt)
		counterBytes(t.be, dt.StorageBytes(t.Numel()))
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
func (t *Tensor) BackwardWith(seed *Tensor) { t.BackwardCtx(seed, nil) }

// BackwardCtx BackwardWith gibidir; ctx çizgedeki dış düğümlerin geri çağrılarına
// (CustomOp) iletilir.
func (t *Tensor) BackwardCtx(seed *Tensor, ctx any) {
	if !t.requiresGrad {
		panic("tensor: gradyan istemeyen bir tensörden geri yayılım yapılamaz")
	}
	// Önce çizgeyi kur: bırakılmış bir ara tensöre rastlanırsa gradyan
	// biriktirmeden hata ver (aksi halde yaprakların gradyanı sessizce eksik kalırdı).
	order := topoOrder(t)
	// Yaprakların gradyan depoları bu geri yayılımda değişecek. Gradyan (.gradyan / Grad)
	// deponun canlı takma adıdır; onu kaydetmiş bir çizge, gradyan değişince açık hata
	// vermeli. Sürüm önce artırılır (bu çizgenin kendisi yaprak gradyanını kaydetmişse
	// aşağıdaki denetim yakalar), birikimden sonra yeniden artırılır.
	var leaves []*Tensor
	for _, n := range order {
		if n.backward == nil {
			leaves = append(leaves, n)
			if n.grad != nil {
				n.grad.BumpVersion()
			}
		}
	}
	defer func() {
		for _, n := range leaves {
			if n.grad != nil {
				n.grad.BumpVersion()
			}
		}
	}()
	checkVersion(order)
	// Çizgenin ara düğümlerini bu geri yayılıma ayır: başka bir geri yayılım (ör. dış
	// düğümün geri hesabı sırasında başka görevin başlattığı) aynı düğümlere rastlarsa
	// gradyanlar iki kez ya da eksik birikeceği için açık hata verir.
	allocated := 0
	defer func() {
		// Hata (panik) durumunda henüz bırakılmamış düğümlerin işaretini kaldır.
		for _, n := range order[:allocated] {
			if !n.free {
				n.behind.Store(false)
			}
		}
	}()
	for _, n := range order {
		if n.backward == nil {
			allocated++
			continue
		}
		if !n.behind.CompareAndSwap(false, true) {
			panic(fmt.Sprintf("tensor: %q işleminin sonucu başka bir geri yayılımın çizgesinde ve o geri yayılım "+
				"sürüyor (ör. başka bir görevden); aynı çizgeden aynı anda iki geri yayılım yapılamaz", n.op))
		}
		allocated++
	}
	g := t.accGrad()
	if seed == nil {
		one := t.be.Alloc(t.Numel(), g.DType())
		t.be.Fill(one, 1)
		t.be.AddTo(g, one, 1)
		giveBack(t.be, one)
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
			n.backwardCounted(ctx)
		}
		// Hâlâ bu düğümde duran gradyanın tek sahibi düğümdür (görünüm
		// işlemleri devrettikleri gradyanı düğümden alır); arka uca iade edilir.
		if n.grad != nil {
			giveBack(n.be, n.grad)
		}
		n.grad = nil
		n.backward = nil
		n.parents = nil
		n.registered = nil
		n.outer = nil
		n.free = true
		n.behind.Store(false)
	}
	if f, ok := t.be.(BackwardFinisher); ok {
		f.BackwardDone()
	}
}

// giveBack arka uç destekliyorsa depolamayı iade eder.
func giveBack(be Backend, s Storage) {
	if r, ok := be.(Releaser); ok {
		r.Release(s)
	}
}

// topoOrder kökten ulaşılabilen ve gradyan isteyen düğümleri sonradan-sıralı
// (önce girdiler) olarak, açık bir yığınla verir.
func topoOrder(root *Tensor) []*Tensor {
	type frame struct {
		t *Tensor
		i int
	}
	secondBackward := func(n *Tensor) {
		panic(fmt.Sprintf("tensor: %q işleminin sonucu üzerinden geri yayılım zaten yapıldı ve ara sonuçlar "+
			"bellek için bırakıldı; aynı çizgeden ikinci kez geri yayılım yapılamaz. Birden çok kayıp "+
			"varsa önce toplayıp tek seferde geri yayın", n.op))
	}
	if root.free {
		secondBackward(root)
	}
	visited := map[*Tensor]bool{root: true}
	var order []*Tensor
	stack := []frame{{root, 0}}
	for len(stack) > 0 {
		totalSum := &stack[len(stack)-1]
		if totalSum.i < len(totalSum.t.parents) {
			p := totalSum.t.parents[totalSum.i]
			totalSum.i++
			if p != nil && p.requiresGrad && !visited[p] {
				if p.free {
					secondBackward(p)
				}
				visited[p] = true
				stack = append(stack, frame{p, 0})
			}
			continue
		}
		order = append(order, totalSum.t)
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
