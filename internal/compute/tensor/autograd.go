package tensor

import (
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// node bir tensörün otomatik türev düğümüdür: gradyanı, geri hesabı ve girdilerin
// düğümleri. Tensörün verisini (depolamasını) tutmaz; gradyan ayırmak için yalnızca şeklini,
// türünü ve arka ucunu bilir. Çizge düğümlerden oluşur: bir işlemin sonucu girdilerinin
// tensörlerine değil düğümlerine bağlanır. Geri hesabın okuyacağı veri (kaydedilen
// tensörler, registered) ayrıca ve açıkça tutulur; kaydedilmeyen bir ara sonucun verisi,
// tensörünü kimse tutmuyorsa çöp toplamayla serbest kalır. PyTorch'ta da düğüm (grad_fn)
// ile kaydedilen tensörler (saved tensors) ayrıdır; TR'de eskiden çizge tensörlerin
// kendisiydi ve her ara sonucun verisi geri yayılıma kadar canlı kalıyordu.
type node struct {
	be    Backend
	shape []int
	dtyp  DType
	// opKind, düğümü üreten işlemin profil sayacı türüdür (geri yayılım bu adla sayılır).
	opKind OpKind

	grad         Storage // bitişik gradyan (şekil = shape), gerekince ayrılır
	requiresGrad bool
	// passGrad, gradyanı girdisine olduğu gibi aktaran işlemin (dönüşüm, görünüm, kopya,
	// cihaz değiştirme) düğümünü işaretler: gradyanının türü girdininkidir (bkz. gradDType).
	passGrad bool

	// Hesap çizgesi: düğümü üreten işlemin girdilerinin düğümleri (gradyan istemeyen girdi
	// nil) ve geri hesap. Geri hesap kendi düğümünü alır; girdilerin düğümlerine parents
	// ile ulaşır. Yaprak düğümlerde (kullanıcının oluşturdukları) ikisi de boştur.
	parents  []*node
	backward func(o *node)
	op       string // hata ayıklama için işlem adı
	// registered, geri yayılımın okuyacağı tensörlerin (verinin çizgede tutulan tek yeri)
	// ileri hesap anındaki depolama sürümleri (yerinde değişiklik denetimi).
	registered []savedVersion
	// free, geri yayılımdan sonra çizge bağları bırakılmış bir ara düğümü işaretler; böyle
	// bir düğüm üzerinden ikinci kez geri yayılım yapılamaz.
	free bool
	// outer, dış (çekirdek dışında yazılmış) bir işlemin düğümünde geri hesabı taşır
	// (bkz. custom_op.go).
	outer *outerNode
	// behind, düğümün şu an süren bir geri yayılımın çizgesinde olduğunu işaretler; aynı
	// düğümden ikinci bir geri yayılım (ör. dış düğümün geri hesabı kilidi bıraktığında
	// başka görevin başlattığı) açık hata verir.
	behind atomic.Bool
	// castMemo, bu ara sonucun son paylaşılan dönüşümü (bkz. CastShared); geri yayılım bırakır.
	castMemo atomic.Pointer[castMemo]
}

// newNode t'nin şekli, türü ve arka ucuyla boş bir düğüm kurar.
func newNode(t *Tensor) *node {
	return &node{be: t.be, shape: t.shape, dtyp: t.dtyp}
}

// Numel düğümün tensörünün eleman sayısı.
func (n *node) Numel() int { return numelOf(n.shape) }

// wants düğümün gradyan isteyip istemediği (nil düğüm istemez).
func (n *node) wants() bool { return n != nil && n.requiresGrad }

// savedVersion ileri hesapta kaydedilmiş (geri yayılımın okuyacağı) bir
// tensörün o andaki depolama sürümü.
type savedVersion struct {
	t *Tensor
	v uint64
}

// track, girdilerden en az biri gradyan istiyorsa ve türevli kipteysek sonucu çizgeye
// bağlar ve sonucun düğümünü kurar. bw sonucun gradyanını girdilere dağıtır; çağrıldığında
// kendi düğümünü alır ve o.grad doludur; girdilerin düğümleri o.parents'tadır (parents ile
// aynı sırada; gradyan istemeyen girdi nil). bw tensör yakalamamalıdır: okuyacağı veriyi
// görünüm ya da depolama olarak yakalar ve o tensörleri save ile bildirir. save, geri
// yayılımın verisini okuyacağı tensörlerdir (nil olanlar atlanır); çizge veriyi yalnızca
// bunlar için tutar, sürümleri kaydedilir ve geri yayılımdan önce yerinde
// değiştirilmedikleri denetlenir.
func track(out *Tensor, op string, parents []*Tensor, bw func(o *node), save ...*Tensor) *Tensor {
	if !GradEnabled() {
		return out
	}
	need := false
	for _, p := range parents {
		if p != nil && p.n.wants() {
			need = true
			break
		}
	}
	if !need {
		return out
	}
	o := newNode(out)
	o.requiresGrad = true
	o.parents = make([]*node, len(parents))
	for i, p := range parents {
		if p != nil && p.n.wants() {
			o.parents[i] = p.n
		}
	}
	o.backward = bw
	o.op = op
	o.opKind = activeKind() // düğümün profil sayacı türü (bkz. counter.go)
	for _, k := range save {
		if k != nil {
			o.registered = append(o.registered, savedVersion{k, k.storage.Version()})
		}
	}
	holdSaved(o)
	out.n = o
	return out
}

// trackPass, gradyanı tek girdisine olduğu gibi aktaran (dönüşüm, görünüm, kopya)
// bir işlemi track ile kaydeder ve düğümü işaretler: çıktının gradyanı girdinin
// gradyan türünde tutulur (bkz. gradDType).
func trackPass(out *Tensor, op string, t *Tensor, bw func(o *node)) *Tensor {
	r := track(out, op, []*Tensor{t}, bw)
	if r.n != nil {
		r.n.passGrad = true
	}
	return r
}

// gradDType gradyan depolamasının türü:
//   - float32 tensörde float32;
//   - 16 bitlik yaprakta (parametre) GradDType (varsayılan float32);
//   - gradyanı girdisine olduğu gibi aktaran işlemin çıktısında girdinin gradyan
//     türü (ör. float32 ağırlığın bfloat16 dönüşümünün gradyanı float32'dir; çarpımın
//     geri yönü ağırlık gradyanını yuvarlamadan float32 yazar);
//   - diğer 16 bitlik ara sonuçlarda kendi türü (PyTorch autocast gibi; geri yöndeki
//     çarpımlar gelen gradyanı dönüştürmeden 16 bitlik girdiyle yapılır).
func (n *node) gradDType() DType {
	for {
		switch {
		case n.dtyp == Float32:
			return Float32
		case n.backward == nil:
			return GradDType()
		case !n.passGrad:
			return n.dtyp
		}
		n = n.parents[0]
	}
}

// checkVersion, çizgedeki her düğümün kaydettiği tensörlerin ileri hesaptan bu yana
// yerinde değiştirilmediğini denetler.
func checkVersion(order []*node) {
	for _, n := range order {
		for _, k := range n.registered {
			if v := k.t.storage.Version(); v != k.v {
				panic(messages.New(messages.H3012, n.op, k.t.shape, k.v, v))
			}
		}
	}
}

// freshGrad gradyan henüz yoksa ve destek doğruysa (arka uç üzerine yazan biçimi veriyorsa)
// sıfırlanmamış gradyan depolaması ayırıp doğru döner: çağıran her elemanı yazmalıdır.
// Gradyan varsa (birikim) ya da destek yoksa accGrad'ın depolamasını ve yanlış verir.
func (n *node) freshGrad(support bool) (Storage, bool) {
	if n.grad != nil || !support {
		return n.accGrad(), false
	}
	dt := n.gradDType()
	n.grad = allocUninit(n.be, n.Numel(), dt)
	counterBytes(n.be, dt.StorageBytes(n.Numel()))
	return n.grad, true
}

// accGrad gradyan depolamasını (gerekirse sıfırlanmış olarak ayırarak) verir;
// çağıran arka uç işlemleriyle üzerine ekler.
func (n *node) accGrad() Storage {
	if n.grad == nil {
		dt := n.gradDType()
		n.grad = n.be.Alloc(n.Numel(), dt)
		counterBytes(n.be, dt.StorageBytes(n.Numel()))
	}
	return n.grad
}

// gradView gradyan depolamasını bitişik görünüm olarak verir.
func (n *node) gradView() View {
	return View{S: n.grad, Shape: n.shape, Strides: contiguousStrides(n.shape)}
}

// Backward tek elemanlı (skaler) bir sonuçtan geri yayılım yapar; tohum
// gradyanı 1'dir.
func (t *Tensor) Backward() {
	if t.Numel() != 1 {
		panic(messages.New(messages.H3013, t.shape))
	}
	t.BackwardWith(nil)
}

// BackwardWith verilen tohum gradyanıyla (sonuçla aynı şekilde, nil ise birler)
// geri yayılım yapar. Topolojik sıra yinelemeli (özyinelemesiz) derinlik
// öncelikli arama ile kurulur, böylece çok derin çizgelerde yığın taşmaz.
//
// Yapraklarda gradyanlar birikir (ZeroGrad ile sıfırlanır). Ara düğümlerin
// gradyanları ve çizge bağları kullanıldıktan sonra bellek için bırakılır;
// bu nedenle aynı çizge üzerinden ikinci kez geri yayılım yapılamaz.
func (t *Tensor) BackwardWith(seed *Tensor) { t.BackwardCtx(seed, nil) }

// BackwardCtx BackwardWith gibidir; ctx çizgedeki dış düğümlerin geri çağrılarına
// (CustomOp) iletilir.
func (t *Tensor) BackwardCtx(seed *Tensor, ctx any) {
	if !t.n.wants() {
		panic(messages.New(messages.H3014))
	}
	root := t.n
	// Önce çizgeyi kur: bırakılmış bir ara düğüme rastlanırsa gradyan
	// biriktirmeden hata ver (aksi halde yaprakların gradyanı sessizce eksik kalırdı).
	order := topoOrder(root)
	// Yaprakların gradyan depoları bu geri yayılımda değişecek. Gradyan (.gradyan / Grad)
	// deponun canlı takma adıdır; onu kaydetmiş bir çizge, gradyan değişince açık hata
	// vermeli. Sürüm önce artırılır (bu çizgenin kendisi yaprak gradyanını kaydetmişse
	// aşağıdaki denetim yakalar), birikimden sonra yeniden artırılır.
	var leaves []*node
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
			panic(messages.New(messages.H3015, n.op))
		}
		allocated++
	}
	g := root.accGrad()
	if seed == nil {
		one := t.be.Alloc(t.Numel(), g.DType())
		t.be.Fill(one, 1)
		t.be.AddTo(g, one, 1)
		giveBack(t.be, one)
	} else {
		if !sameShape(seed.shape, t.shape) {
			panic(messages.New(messages.H3016, seed.shape, t.shape))
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
		releaseSaved(n)
		n.backward = nil
		n.parents = nil
		n.registered = nil
		n.outer = nil
		n.castMemo.Store(nil)
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
func topoOrder(root *node) []*node {
	type frame struct {
		n *node
		i int
	}
	secondBackward := func(n *node) {
		panic(messages.New(messages.H3017, n.op))
	}
	if root.free {
		secondBackward(root)
	}
	visited := map[*node]bool{root: true}
	var order []*node
	stack := []frame{{root, 0}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.i < len(top.n.parents) {
			p := top.n.parents[top.i]
			top.i++
			if p.wants() && !visited[p] {
				if p.free {
					secondBackward(p)
				}
				visited[p] = true
				stack = append(stack, frame{p, 0})
			}
			continue
		}
		order = append(order, top.n)
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
