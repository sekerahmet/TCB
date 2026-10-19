package tensor

import "fmt"

// Dış düğümler: ileri ve geri hesabı çekirdeğin dışında (ör. TR dilinde, kullanıcı
// tarafından) yazılmış işlemler.
//
// İleri hesap çağıranca türevsiz yapılır; CustomOp yalnızca sonucu çizgeye TEK bir düğüm
// olarak bağlar. Geri yayılımda düğümün geri çağrısı (CustomBackward) çıktıların
// gradyanlarıyla çağrılır ve her girdi için bir gradyan döndürür. Böylece bileşik bir
// işlemin onlarca ara tensörü ve düğümü yerine çizgede tek bir düğüm ve yalnızca geri
// hesabın açıkça istediği kalıntılar (saved) tutulur.
//
// Çok çıktılı işlemlerde çıktılar ortak, gizli bir düğüme bağlanır: her çıktı yalnızca
// kendi gradyanını bu düğüme devreder (hesap yapmaz); kullanıcının geri hesabı gizli
// düğümde bir kez, bütün çıktıların gradyanları toplandıktan sonra çalışır (topolojik
// sıra bunu garanti eder). Gradyan almayan (kayba bağlı olmayan) çıktılar için geri
// çağrıya sıfır tensör verilir.

// CustomBackward bir dış düğümün geri hesabıdır. ctx, geri yayılımı başlatan çağrının
// BackwardCtx'e verdiği değerdir (verilmemişse nil); dış düğümün geri hesabını çağıran
// katmanın bağlamını (ör. yorumlayıcı görevini) taşır. grads her çıktının gradyanıdır
// (çıktıyla aynı şekilde; gradyan almayan çıktı için sıfır tensör). Dönüş, girdi
// sayısı kadar öğeli olmalıdır: her girdi için girdiyle aynı şekilde bir gradyan ya da
// nil (gradyan yok). Gradyan istemeyen girdilerin öğesi yok sayılır. Geri çağrı türevsiz
// kipte çalışır.
type CustomBackward func(ctx any, grads []*Tensor) []*Tensor

// outerNode bir dış işlemin geri yayılım durumudur.
type outerNode struct {
	op       string
	inputs   []*Tensor
	outputs  []*Tensor
	incoming []Storage // çok çıktıda her çıktının devrettiği gradyan (nil: gelmedi)
	bw       CustomBackward
}

// CustomOp outputs'u, ileri hesabı çizgenin dışında yapılmış bir işlemin sonuçları olarak
// çizgeye bağlar. inputs işlemin girdileridir (nil öğeler tensör olmayan girdilerdir);
// saved, geri hesabın okuyacağı tensörlerdir: sürümleri kaydedilir ve geri yayılımdan
// önce yerinde değiştirilmedikleri denetlenir (diğer işlemlerle aynı kural).
//
// Türevli kipte değilsek ya da hiçbir girdi gradyan istemiyorsa outputs olduğu gibi
// döner. Aksi halde her çıktı için veriyi paylaşan yeni bir tensör (başlık) döner; böylece
// ileri hesabın bir girdiyi ya da var olan bir tensörü olduğu gibi döndürmesi o tensörün
// çizge bilgisini bozmaz.
func CustomOp(op string, inputs, outputs, saved []*Tensor, bw CustomBackward) []*Tensor {
	if len(outputs) == 0 {
		panic(fmt.Sprintf("tensor: %q dış işlemi en az bir çıktı vermeli", op))
	}
	if bw == nil {
		panic(fmt.Sprintf("tensor: %q dış işleminin geri hesabı yok", op))
	}
	for i, o := range outputs {
		if o == nil {
			panic(fmt.Sprintf("tensor: %q dış işleminin %d. çıktısı boş", op, i+1))
		}
	}
	if !GradEnabled() {
		return outputs
	}
	need := false
	for _, p := range inputs {
		if p != nil && p.requiresGrad {
			need = true
			break
		}
	}
	if !need {
		return outputs
	}
	outs := make([]*Tensor, len(outputs))
	for i, o := range outputs {
		outs[i] = o.Detach()
	}
	parents := append([]*Tensor(nil), inputs...)
	d := &outerNode{op: op, inputs: parents, outputs: outs, bw: bw}

	if len(outs) == 1 {
		o := outs[0]
		track(o, op, parents, func() { d.back(nil) }, saved...)
		o.outer = d
		return outs
	}

	// Çok çıktı: gizli ortak düğüm (tek elemanlı; gradyanı yalnızca "çıktılardan en az
	// birinin gradyanı geldi" işaretidir).
	be := outs[0].be
	h := fromStorage(be, be.Alloc(1, Float32), nil)
	track(h, op, parents, func() { d.back(nil) }, saved...)
	h.outer = d
	d.incoming = make([]Storage, len(outs))
	for i, o := range outs {
		i, o := i, o
		track(o, fmt.Sprintf("%s[%d]", op, i), []*Tensor{h}, func() {
			d.incoming[i] = o.grad
			o.grad = nil // gradyanın sahipliği dış düğüme geçer
			if h.grad == nil {
				h.grad = h.be.Alloc(1, Float32)
			}
		})
	}
	return outs
}

// runBackward düğümün geri hesabını yapar; dış düğümlerde ctx geri çağrıya iletilir.
func (t *Tensor) runBackward(ctx any) {
	if t.outer != nil {
		t.outer.back(ctx)
		return
	}
	t.backward()
}

// back dış düğümün geri hesabıdır: çıktı gradyanlarını toplar, geri çağrıyı türevsiz
// kipte çalıştırır ve dönen gradyanları girdilere ekler.
func (d *outerNode) back(ctx any) {
	grads := make([]*Tensor, len(d.outputs))
	if len(d.outputs) == 1 {
		// Tek çıktı: düğüm çıktının kendisidir; gradyanın sahipliği geri çağrıya geçer
		// (çağıran gradyan tensörünü saklasa bile havuza iade edilip bozulmaz).
		o := d.outputs[0]
		grads[0] = fromStorage(o.be, o.grad, o.shape)
		o.grad = nil
	} else {
		for i, o := range d.outputs {
			if g := d.incoming[i]; g != nil {
				grads[i] = fromStorage(o.be, g, o.shape)
			} else {
				grads[i] = zerosOf(isGrad, o.be, o.shape, o.gradDType())
			}
		}
		d.incoming = nil
	}
	var result []*Tensor
	NoGrad(func() { result = d.bw(ctx, grads) })
	if len(result) != len(d.inputs) {
		panic(fmt.Sprintf("tensor: %q dış işleminin geri hesabı %d girdi için %d gradyan döndürdü; her girdi için "+
			"bir gradyan (ya da nil) gerekir", d.op, len(d.inputs), len(result)))
	}
	for i, in := range d.inputs {
		g := result[i]
		if g == nil || in == nil || !in.requiresGrad {
			continue
		}
		if !sameShape(g.shape, in.shape) {
			panic(fmt.Sprintf("tensor: %q dış işleminin geri hesabı %d. girdi için %v şeklinde gradyan döndürdü; "+
				"girdinin şekli %v (gradyan girdiyle aynı şekilde olmalı, yayınlama yapılmaz)", d.op, i+1, g.shape, in.shape))
		}
		sameDevice(d.op, in, g)
		in.be.AddTo(in.accGrad(), g.denseStorage(), 1)
	}
}

// GraphStats t'den ulaşılabilen hesap çizgesinin düğüm sayısını (yapraklar hariç) ve
// çizgenin geri yayılıma dek bellekte tuttuğu ara tensörlerin bayt toplamını verir: düğüm
// sonuçları ile geri hesap için kaydedilmiş tensörler; çizgenin girdisi olan yaprakların
// (parametreler, veri) depolaması sayılmaz. Görünümler depolamayı paylaştığından her
// depolama bir kez sayılır. Tanılama ve ölçüm içindir; geri yayılımı yapılmış bir çizgede
// panikler.
func GraphStats(t *Tensor) (nodes int, bytes int64) {
	if !t.requiresGrad {
		return 0, 0
	}
	order := topoOrder(t)
	leaf := map[Storage]bool{}
	for _, n := range order {
		for _, p := range n.parents {
			if p != nil && p.backward == nil {
				leaf[p.storage] = true
			}
		}
	}
	counted := map[Storage]bool{}
	add := func(s Storage) {
		if s == nil || leaf[s] || counted[s] {
			return
		}
		counted[s] = true
		bytes += int64(s.DType().StorageBytes(s.Len()))
	}
	for _, n := range order {
		if n.backward == nil {
			continue
		}
		nodes++
		add(n.storage)
		for _, k := range n.registered {
			add(k.t.storage)
		}
	}
	return nodes, bytes
}
