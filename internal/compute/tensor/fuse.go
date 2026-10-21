package tensor

import (
	"fmt"
	"strings"
)

// ============================================================================
// Kaynaştırma: eleman bazlı zincirin tek programı
// ============================================================================
//
// Kaynaştırılan bir fonksiyon (TR'de kaynaştır(f)) iz tensörleriyle bir kez çalıştırılır:
// işlemler hesaplanmaz, bir program (FuseNode düğümlerinden bir DAG) olarak kaydedilir
// (fuse_trace.go). Program her çıktı için tek bir hesaptır: arka uç onu tek geçişte (GPU'da
// tek çekirdekte) yürütür, ara sonuçlar belleğe yazılmaz. Geri yönü programın simgesel
// türevidir (fuse_grad.go): ara değerler saklanmaz, geri geçişte girdilerden yeniden
// hesaplanır.
//
// Anlam: her düğümün değeri float32'de hesaplanır ve düğümün türüne yuvarlanır; kaynaşmamış
// yol da her işlemi float32'de hesaplayıp sonucun türüne yuvarladığından ileri yön bit
// düzeyinde aynıdır (arka uçlar aynı eleman işlevlerini kullanır). Geri yön float32'de
// hesaplanır, yalnızca sonuç girdinin gradyan türüne yuvarlanır.
//
// Kapsam: eleman bazlı tekli ve ikili işlemler (karşılaştırmalar dahil: sonuç float32 0/1,
// türevi 0), sayıyla işlemler, tür dönüşümü, yayınlama,
// dilim (adımlı), eksen sırası, boyu 1 eksen ekleme ve çıkarma, birleştirme. İndirgeme,
// çarpım, genel yeniden şekillendirme kaynaştırılmaz (açık hata).

// FuseKind bir program düğümünün türüdür.
type FuseKind uint8

const (
	FuseInput     FuseKind = iota // Index. girdinin (kendi şeklinde) elemanı
	FuseConst                     // P sabiti
	FuseUnary                     // Un(Args[0], P)
	FuseUnaryGrad                 // Args[0] · Un'(Args[1], Args[2]) (g, x, y; UnaryGrad ile aynı)
	FuseBinary                    // Bin(Args[0], Args[1]); ikisi de düğümün şeklinde
	FuseCast                      // Args[0]'ın DType'a yuvarlanması
	FuseView                      // Args[0]'ın Map ile okunması
	FuseScatter                   // görünümün tersi: Args[0] (görünüm şeklinde) Map'in kaynağına yayılır, dışı 0
	FuseConcat                    // Args'ın Axis boyunca birleşimi
)

// FuseMap bir görünümün indeks eşlemesidir: kaynağın (Args[0]) s ekseninin indeksi
// Off[s] + Step[s] · i[Src[s]]'dir; Src[s] < 0 ise sabit Off[s]'dir (çıkarılan ya da
// yayınlanan eksen). Görünümün Src'de geçmeyen eksenlerinin boyu 1'dir ya da (yayınlamada)
// kaynakta karşılığı yoktur.
type FuseMap struct {
	Src, Off, Step []int
}

// FuseNode programın bir düğümüdür. Shape ve DType düğümün değerinin şekli ve türüdür.
type FuseNode struct {
	Kind  FuseKind
	Un    UnOp
	Bin   BinOp
	P     float32
	DType DType
	Shape []int
	Args  []*FuseNode
	Map   *FuseMap
	Axis  int
	Index int
}

// FuseProgram kaynaştırılmış bir fonksiyonun programıdır: girdilerin tanımı, çıktıların
// düğümleri ve (türev isteyen girdiler için) geri yönün düğümleri.
type FuseProgram struct {
	Inputs  []FuseInputInfo
	Outputs []*FuseNode
	// Single, fonksiyonun liste değil tek bir tensör döndürdüğünü söyler (dil katmanı için).
	Single bool
	// grads[k] k. girdinin gradyanıdır; Index ≥ len(Inputs) olan Input düğümleri çıktı
	// gradyanlarıdır (len(Inputs) + çıktı sırası). Türev istemeyen girdide nil.
	grads []*FuseNode
}

// FuseInputInfo bir girdinin izlendiği şekil, tür ve türev isteği (önbellek anahtarıdır).
type FuseInputInfo struct {
	Shape        []int
	DType        DType
	RequiresGrad bool
}

// Fuser kaynaşmış bir program düğümünü yürüten arka uçtur: out (node'un şeklinde, bitişik)
// = node'un değeri, out'un türüne bir kez yuvarlanmış; add doğruysa out += değer (toplam
// float32'de, out'un türüne bir kez yuvarlanır). ins[k] k. girdinin görünümüdür.
type Fuser interface {
	RunFused(node *FuseNode, out Storage, ins []View, add bool)
}

// Key düğümün ve alt ağacının biçimini tek metinle verir (çekirdek önbelleğinin anahtarı;
// aynı metin aynı hesaptır). Paylaşılan alt düğümler bir kez yazılır.
func (n *FuseNode) Key() string {
	var b strings.Builder
	ids := map[*FuseNode]int{}
	var walk func(x *FuseNode) int
	walk = func(x *FuseNode) int {
		if id, ok := ids[x]; ok {
			return id
		}
		args := make([]int, len(x.Args))
		for i, a := range x.Args {
			args[i] = walk(a)
		}
		id := len(ids)
		ids[x] = id
		fmt.Fprintf(&b, "%d=%d:%d:%d:%v:%s:%v:%v:%d:%d", id, x.Kind, x.Un, x.Bin, x.P, x.DType, x.Shape, args, x.Axis, x.Index)
		if x.Map != nil {
			fmt.Fprintf(&b, ":%v/%v/%v", x.Map.Src, x.Map.Off, x.Map.Step)
		}
		b.WriteByte(';')
		return id
	}
	walk(n)
	return b.String()
}

// Nodes düğümü ve alt ağacını, her düğüm argümanlarından sonra gelecek sırayla (bir kez)
// verir.
func (n *FuseNode) Nodes() []*FuseNode {
	var out []*FuseNode
	seen := map[*FuseNode]bool{}
	var walk func(x *FuseNode)
	walk = func(x *FuseNode) {
		if seen[x] {
			return
		}
		seen[x] = true
		for _, a := range x.Args {
			walk(a)
		}
		out = append(out, x)
	}
	walk(n)
	return out
}

// ----------------------------------------------------------------------------
// Düğüm kurucuları
// ----------------------------------------------------------------------------

func fuseInput(k int, shape []int, dt DType) *FuseNode {
	return &FuseNode{Kind: FuseInput, Index: k, Shape: copyInts(shape), DType: dt}
}

func fuseConst(v float32, shape []int) *FuseNode {
	return &FuseNode{Kind: FuseConst, P: v, Shape: copyInts(shape), DType: Float32}
}

func fuseUnary(op UnOp, p float32, x *FuseNode) *FuseNode {
	return &FuseNode{Kind: FuseUnary, Un: op, P: p, Shape: x.Shape, DType: x.DType, Args: []*FuseNode{x}}
}

// fuseBinary a op b'dir; a ve b önce sonucun şekline yayınlanır.
func fuseBinary(op BinOp, a, b *FuseNode, shape []int, dt DType) *FuseNode {
	return &FuseNode{Kind: FuseBinary, Bin: op, Shape: copyInts(shape), DType: dt,
		Args: []*FuseNode{fuseBroadcast(a, shape), fuseBroadcast(b, shape)}}
}

func fuseCast(x *FuseNode, dt DType) *FuseNode {
	if x.DType == dt {
		return x
	}
	return &FuseNode{Kind: FuseCast, Shape: x.Shape, DType: dt, Args: []*FuseNode{x}}
}

func fuseView(x *FuseNode, m *FuseMap, shape []int) *FuseNode {
	return &FuseNode{Kind: FuseView, Shape: copyInts(shape), DType: x.DType, Args: []*FuseNode{x}, Map: m}
}

// fuseBroadcast x'i shape'e (sağdan hizalı) yayınlar; şekli zaten shape ise x'in kendisidir.
func fuseBroadcast(x *FuseNode, shape []int) *FuseNode {
	if sameShape(x.Shape, shape) {
		return x
	}
	lead := len(shape) - len(x.Shape)
	m := &FuseMap{Src: make([]int, len(x.Shape)), Off: make([]int, len(x.Shape)), Step: make([]int, len(x.Shape))}
	for s, d := range x.Shape {
		m.Step[s] = 1
		if d == shape[s+lead] {
			m.Src[s] = s + lead
		} else {
			m.Src[s] = -1 // boyu 1: hep 0. eleman
		}
	}
	return fuseView(x, m, shape)
}

// identityMap aynı şekilde bir görünümün eşlemesi (dilim ve eksen eşlemeleri bundan kurulur).
func identityMap(rank int) *FuseMap {
	m := &FuseMap{Src: make([]int, rank), Off: make([]int, rank), Step: make([]int, rank)}
	for s := range m.Src {
		m.Src[s], m.Step[s] = s, 1
	}
	return m
}
