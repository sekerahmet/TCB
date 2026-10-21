package tensor

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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

// outerNode bir dış işlemin geri yayılım durumudur. Girdileri ve çıktıları düğümleriyle
// tutar (veri tutmaz; geri hesabın okuyacakları saved ile ayrıca kaydedilir). inputs'ta
// gradyan istemeyen ya da tensör olmayan girdinin öğesi nil'dir; inShapes girdilerin
// şeklidir (dönen gradyanın şekli denetlenir).
type outerNode struct {
	op       string
	inputs   []*node
	inShapes [][]int
	outputs  []*node
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
		panic(messages.New(messages.H3028, op))
	}
	if bw == nil {
		panic(messages.New(messages.H3029, op))
	}
	for i, o := range outputs {
		if o == nil {
			panic(messages.New(messages.H3030, op, i+1))
		}
	}
	if !GradEnabled() {
		return outputs
	}
	need := false
	for _, p := range inputs {
		if p != nil && p.RequiresGrad() {
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
	d := &outerNode{op: op, bw: bw, inputs: make([]*node, len(parents)), inShapes: make([][]int, len(parents))}
	for i, p := range parents {
		if p != nil {
			d.inputs[i], d.inShapes[i] = p.n, p.shape
		}
	}
	// Geri hesap dış düğümün kendisidir (backwardCounted outer'ı çağırır).
	outerBack := func(*node) { d.back(nil) }

	if len(outs) == 1 {
		o := outs[0]
		track(o, op, parents, outerBack, saved...)
		o.n.outer = d
		d.outputs = []*node{o.n}
		return outs
	}

	// Çok çıktı: gizli ortak düğüm (tek elemanlı; gradyanı yalnızca "çıktılardan en az
	// birinin gradyanı geldi" işaretidir).
	be := outs[0].be
	h := fromStorage(be, be.Alloc(1, Float32), nil)
	track(h, op, parents, outerBack, saved...)
	hn := h.n
	hn.outer = d
	d.incoming = make([]Storage, len(outs))
	d.outputs = make([]*node, len(outs))
	for i, o := range outs {
		i := i
		track(o, fmt.Sprintf("%s[%d]", op, i), []*Tensor{h}, func(on *node) {
			d.incoming[i] = on.grad
			on.grad = nil // gradyanın sahipliği dış düğüme geçer
			if hn.grad == nil {
				hn.grad = hn.be.Alloc(1, Float32)
			}
		})
		d.outputs[i] = o.n
	}
	return outs
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
	start := Created()
	NoGrad(func() { result = d.bw(ctx, grads) })
	if len(result) != len(d.inputs) {
		panic(messages.New(messages.H3031, d.op, len(d.inputs), len(result)))
	}
	for i, in := range d.inputs {
		g := result[i]
		if g == nil || !in.wants() {
			continue
		}
		if !sameShape(g.shape, d.inShapes[i]) {
			panic(messages.New(messages.H3032, d.op, i+1, g.shape, d.inShapes[i]))
		}
		if g.be.Device() != in.be.Device() {
			panic(messages.New(messages.H3072, d.op, in.be.Device(), g.Device()))
		}
		in.be.AddTo(in.accGrad(), g.denseStorage(), 1)
	}
	// Geri hesabın gradyanları ve bu çağrıda ürettiği tensörler burada biter. Bir yorumlayıcı
	// değerinin sardığı başlık (wraps > 0) sahibi tarafından bırakılır; önceden var olan bir
	// başlık (kaydedilmiş bir tensör) çağıranındır, dokunulmaz.
	for _, g := range grads {
		g.dropTemp(0)
	}
	for _, g := range result {
		if g != nil {
			g.dropTemp(start)
		}
	}
}

// GraphStats t'den ulaşılabilen hesap çizgesinin düğüm sayısını (yapraklar hariç) ve
// çizgenin geri yayılıma dek bellekte tuttuğu ara tensörlerin bayt toplamını verir. Çizge
// veriyi yalnızca geri hesabın okuyacağı (kaydedilmiş) tensörler için tutar; düğüm sonuçları
// kaydedilmedikçe sayılmaz (onları tutan, varsa kullanıcının kendisidir). Yaprakların
// (parametreler, veri) depolaması sayılmaz. Görünümler depolamayı paylaştığından her
// depolama bir kez sayılır. Tanılama ve ölçüm içindir; geri yayılımı yapılmış bir çizgede
// panikler.
func GraphStats(t *Tensor) (nodes int, bytes int64) {
	if !t.RequiresGrad() {
		return 0, 0
	}
	order := topoOrder(t.n)
	leaf := map[Storage]bool{}
	for _, n := range order {
		for _, k := range n.registered {
			if k.t.IsLeaf() {
				leaf[k.t.storage] = true
			}
		}
	}
	counted := map[Storage]bool{}
	for _, n := range order {
		if n.backward == nil {
			continue
		}
		nodes++
		for _, k := range n.registered {
			s := k.t.storage
			if s == nil || leaf[s] || counted[s] {
				continue
			}
			counted[s] = true
			bytes += int64(s.DType().StorageBytes(s.Len()))
		}
	}
	return nodes, bytes
}
