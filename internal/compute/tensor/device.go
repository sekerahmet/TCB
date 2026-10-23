package tensor

import (
	"fmt"
	"sync/atomic"
)

// ============================================================================
// Cihaz, depolama ve arka uç soyutlaması
// ============================================================================
//
// Tensörün verisi doğrudan bir dilimde değil, bir cihaza ait Storage içinde
// durur. Tüm hesaplar (ileri ve geri yön) Backend arayüzü üzerinden yapılır;
// tensör katmanı ve otomatik türev depolamanın içeriğine hiç dokunmaz, yalnızca
// şekil/adım bilgisi tutar ve arka uç yöntemlerini çağırır. Böylece yeni bir
// cihaz (ör. GPU) yalnızca Backend'i uygulayıp RegisterBackend ile kaydolarak
// eklenebilir; otomatik türev ve üst katman kodu değişmez.
//
// Kurallar (her arka uç için):
//   - Alloc ile ayrılan depolama sıfırlanmış olmalıdır.
//   - "Bitişik" denen çıktı/gradyan depolamaları satır öncelikli ve ofseti 0'dır.
//   - Girdiler View olarak gelir: herhangi bir ofset ve adımla (yayın için adım 0
//     dahil) okunabilmelidir; aksi belirtilmedikçe.
//   - "Grad" ile biten yöntemler hedefin üzerine EKLER (gradyan birikimi).
//   - Konak (host) tarafındaki []int indeksler (Embedding, CrossEntropy) arka
//     uç tarafından gerekirse cihaza aktarılır.
//   - Veri türü: her depolamanın bir eleman türü (DType) vardır. Girdi
//     görünümleri ve hedefler herhangi bir kayan noktalı türde olabilir (ör.
//     bfloat16 girdi, float32 gradyan); arka uç gerekirse türleri dönüştürür.
//     Çıktının türü, çağıranın ayırdığı çıktı depolamasının türüdür.
//   - Karışık hassasiyet: hesap türü (girdi/çıktı depolamalarının türü) ile
//     biriktirme türü ayrıdır. Arka uçlar iç toplamaları (matris çarpımı,
//     indirgemeler, normlar, softmax) en az float32 hassasiyetle yapar ve
//     sonucu çıktı türüne bir kez (en yakına-çifte) yuvarlar.
//   - Yerinde değişiklik: depolamanın içeriğini yerinde değiştiren tensör
//     katmanı işlemleri (Set, iyileştirici adımları) BumpVersion çağırır;
//     otomatik türev bunu ileri hesapta kaydedilen girdilerin bozulduğunu
//     anlamak için kullanır.

// DeviceKind cihaz türü.
type DeviceKind int

const (
	// KindCPU ana işlemci.
	KindCPU DeviceKind = iota
	// KindGPU grafik işlemci (arka ucu internal/compute/gpu; sürücü bulunursa ilk
	// kullanımda kurulur, bkz. RegisterLazyBackend).
	KindGPU
)

// Device bir hesap cihazını tanımlar.
type Device struct {
	Kind  DeviceKind
	Index int
}

// CPU ana işlemci cihazı.
var CPU = Device{Kind: KindCPU}

// GPU verilen sıradaki grafik işlemci cihazı (arka ucu kayıtlı değilse
// kullanılamaz).
func GPU(index int) Device { return Device{Kind: KindGPU, Index: index} }

// String cihazın kullanıcıya görünen adı: "cpu", "gpu" (ilk GPU), "gpu:1", ...
func (d Device) String() string {
	switch d.Kind {
	case KindCPU:
		return "cpu"
	case KindGPU:
		if d.Index == 0 {
			return "gpu"
		}
		return fmt.Sprintf("gpu:%d", d.Index)
	}
	return fmt.Sprintf("cihaz(%d:%d)", d.Kind, d.Index)
}

// DType depolamadaki eleman türü.
type DType uint8

const (
	// Float32 32 bitlik kayan nokta (varsayılan).
	Float32 DType = iota
	// BFloat16 16 bitlik beyin kayan noktası: float32 ile aynı üs aralığı,
	// 8 bitlik anlamlı kısım (7 bit saklanır).
	BFloat16
	// Float16 IEEE 754 yarım hassasiyet: 5 bit üs, 10 bit anlamlı kısım.
	Float16
	// Int8 nicemlenmiş çıkarım için ayrılmıştır (ölçek ayrı tutulur); henüz
	// hiçbir arka uç hesap desteği sunmaz.
	Int8
	// Int4 4 bitlik paketli tamsayı (bir baytta iki eleman), nicemlenmiş
	// çıkarım için ayrılmıştır; henüz hiçbir arka uç hesap desteği sunmaz.
	Int4
)

func (d DType) String() string {
	switch d {
	case Float32:
		return "float32"
	case BFloat16:
		return "bfloat16"
	case Float16:
		return "float16"
	case Int8:
		return "int8"
	case Int4:
		return "int4"
	}
	return fmt.Sprintf("dtype(%d)", uint8(d))
}

// Name türün dildeki adıdır (`.tür`, `türe(...)`: "float32", "bf16", "fp16"); dilde adı
// olmayan türlerde String.
func (d DType) Name() string {
	switch d {
	case BFloat16:
		return "bf16"
	case Float16:
		return "fp16"
	}
	return d.String()
}

// Bits bir elemanın bit sayısı.
func (d DType) Bits() int {
	switch d {
	case Float32:
		return 32
	case BFloat16, Float16:
		return 16
	case Int8:
		return 8
	case Int4:
		return 4
	}
	return 0
}

// IsFloat kayan noktalı (türevlenebilir) bir tür mü.
func (d DType) IsFloat() bool { return d == Float32 || d == BFloat16 || d == Float16 }

// StorageBytes n elemanın bu türde kapladığı bayt (paketli türler yukarı yuvarlanır).
func (d DType) StorageBytes(n int) int { return (n*d.Bits() + 7) / 8 }

// Storage bir cihazdaki düz bellek bloğu (Len eleman, DType türünde). Her
// depolamanın, içeriği yerinde her değiştirildiğinde artan bir sürüm sayacı ve
// sahip sayacı vardır (bkz. VersionCounter, lifetime.go).
type Storage interface {
	Device() Device
	Len() int
	DType() DType
	// Version depolamanın sürüm sayacı.
	Version() uint64
	// BumpVersion sürüm sayacını artırır (yerinde değişiklikten sonra).
	BumpVersion()
	// Hold depolamanın sahip sayacını artırır; Unhold azaltır ve kalan sahip sayısını verir
	// (lifetime.go).
	Hold()
	Unhold() int64
}

// VersionCounter depolama uygulamalarına gömülebilen, eşzamanlı kullanıma
// güvenli sürüm ve sahip sayacı (Storage arayüzünün Version/BumpVersion ve
// Hold/Unhold yöntemleri).
type VersionCounter struct {
	v       atomic.Uint64
	holders atomic.Int64
}

// Hold sahip sayacını bir artırır.
func (c *VersionCounter) Hold() { c.holders.Add(1) }

// Unhold sahip sayacını bir azaltır ve kalanı verir.
func (c *VersionCounter) Unhold() int64 { return c.holders.Add(-1) }

// Version sayacın değeri.
func (c *VersionCounter) Version() uint64 { return c.v.Load() }

// BumpVersion sayacı bir artırır.
func (c *VersionCounter) BumpVersion() { c.v.Add(1) }

// View bir depolamaya ofset, şekil ve adımlarla bakan salt okunur tanım.
type View struct {
	S       Storage
	Offset  int
	Shape   []int
	Strides []int
}

// DType görünümün eleman türü.
func (v View) DType() DType { return v.S.DType() }

// Numel görünümdeki eleman sayısı.
func (v View) Numel() int { return numelOf(v.Shape) }

// DenseView s'nin baştan, satır öncelikli ve boşluksuz okunan shape şekilli görünümüdür.
func DenseView(s Storage, shape []int) View {
	return View{S: s, Shape: shape, Strides: contiguousStrides(shape)}
}

// IsContiguous görünümün satır öncelikli ve boşluksuz olup olmadığı.
func (v View) IsContiguous() bool { return isContiguous(v.Shape, v.Strides) }

// BinOp ikili eleman bazlı işlem.
type BinOp int

const (
	OpAdd BinOp = iota
	OpSub
	OpMul
	OpDiv
	OpAtan2 // atan2(a, b): a/b'nin açısı (dört bölge)
	// Karşılaştırmalar: sonuç 0/1 (NaN içeren karşılaştırma 0); türevsizdir.
	OpLt // a < b
	OpGt // a > b
	OpLe // a <= b
	OpGe // a >= b

	numBinOps // işlem sayısı; işlem değildir (testler üyeleri sayar)
)

// UnOp tekli eleman bazlı işlem; bazıları bir skaler parametre (p) alır.
type UnOp int

const (
	OpExp        UnOp = iota
	OpLog             // doğal logaritma
	OpSqrt            // karekök
	OpPow             // x^p
	OpTanh            // tanh
	OpRelu            // max(0,x)
	OpGelu            // tanh yaklaşımlı GELU
	OpSigmoid         // lojistik
	OpAddScalar       // x+p
	OpMulScalar       // x·p
	OpRSubScalar      // p-x
	OpRDivScalar      // p/x
	OpSin             // sinüs
	OpCos             // kosinüs
	OpSilu            // x·σ(x)
	OpTan             // tanjant
	OpAsin            // arksinüs
	OpAcos            // arkkosinüs
	OpAtan            // arktanjant
	OpSinh            // hiperbolik sinüs
	OpCosh            // hiperbolik kosinüs
	OpLog1p           // log(1+x), küçük x'te kararlı
	OpLog2            // 2 tabanında logaritma
	OpLog10           // 10 tabanında logaritma
	OpExpm1           // e^x − 1, küçük x'te kararlı
	OpFloor           // taban (türevi 0)
	OpCeil            // tavan (türevi 0)
	OpSign            // işaret: −1, 0, 1 (NaN korunur; türevi 0)
	OpAbs             // mutlak değer |x| (işaret biti silinir, NaN korunur; türevi işaret(x), 0'da 0)
	OpGeluExact       // tam (erf tabanlı) GELU: x·Φ(x), Φ standart normal dağılım işlevi; türevi Φ(x) + x·φ(x)

	numUnOps // işlem sayısı; işlem değildir (testler üyeleri sayar)
)

// GradMode ikili işlemlerin geri yayılımında hedefe eklenecek terim:
// g çıktı gradyanı, o diğer işlenen, y çıktı.
type GradMode int

const (
	GradPlus  GradMode = iota // g
	GradMinus                 // -g
	GradMul                   // g·o
	GradDiv                   // g/o
	GradDivB                  // -g·y/o (bölmenin paydaya göre türevi)
)

// Backend bir cihazdaki tüm tensör işlemlerini uygular.
type Backend interface {
	Device() Device

	// --- Bellek ---

	// Alloc n elemanlı, dt türünde sıfırlanmış depolama ayırır. Arka ucun
	// desteklemediği türlerde panik verir.
	Alloc(n int, dt DType) Storage
	// FromHost konak verisini dt türüne (en yakına-çifte yuvarlayarak)
	// dönüştürüp kopyalayarak depolama oluşturur.
	FromHost(data []float32, dt DType) Storage
	// ToHost görünümün elemanlarını mantıksal sırayla float32 olarak konağa kopyalar.
	ToHost(src View) []float32
	// Copy src'yi dst'ye dstOff'tan başlayarak bitişik yazar; türler farklıysa
	// dönüştürür (en yakına-çifte).
	Copy(dst Storage, dstOff int, src View)
	// Fill tüm elemanları v yapar.
	Fill(dst Storage, v float32)
	// CopyStrided src'nin elemanlarını dst görünümüne (herhangi ofset ve adımla)
	// mantıksal sırayla yazar; şekiller aynıdır (src yayınlanmış olabilir: adım 0).
	// Türler farklıysa dönüştürür (en yakına-çifte). dst ile src örtüşmemelidir.
	CopyStrided(dst, src View)

	// --- Gradyan biriktirme ---

	// AddTo dst += scale·src (eşit uzunluklu bitişik depolamalar).
	AddTo(dst, src Storage, scale float32)
	// ScatterAdd bitişik src'yi (şekil shape) dst'ye dstOff/dstStrides
	// eşlemesiyle ekler (görünümlerin geri yayılımı).
	ScatterAdd(dst Storage, dstOff int, dstStrides []int, src Storage, shape []int)
	// ExpandAdd dst (dstShape, bitişik) += scale·yayın(src (srcShape, bitişik)).
	ExpandAdd(dst Storage, dstShape []int, src Storage, srcShape []int, scale float32)

	// --- Eleman bazlı ---

	// Binary out (outShape, bitişik) = a op b (yayınlamalı).
	Binary(op BinOp, out Storage, outShape []int, a, b View)
	// BinaryGrad g'den (outShape) türetilen terimi dstShape'e indirgeyerek dst'ye ekler.
	// o (diğer işlenen) ve y (çıktı) yalnızca gereken kiplerde kullanılır.
	BinaryGrad(mode GradMode, dst Storage, dstShape, outShape []int, g Storage, o View, y Storage)
	// Unary out (bitişik) = f(x).
	Unary(op UnOp, p float32, out Storage, x View)
	// UnaryGrad gx += g·f'(x, y).
	UnaryGrad(op UnOp, p float32, gx, g Storage, x View, y Storage)

	// --- Genişletilmiş eleman bazlı işlemler ---

	// Clamp out (bitişik) = x'in [lo, hi] aralığına sınırlanmışı; NaN korunur (sınır
	// verilmeyen yan ±sonsuzdur).
	Clamp(out Storage, x View, lo, hi float32)
	// ClampGrad gx += g·[lo ≤ x ≤ hi].
	ClampGrad(gx, g Storage, x View, lo, hi float32)
	// Where out (outShape, bitişik) = c ≠ 0 ise a, değilse b (üçü de yayınlamalı; NaN
	// koşul doğru sayılır).
	Where(out Storage, outShape []int, c, a, b View)
	// WhereGrad g'nin (outShape) koşulu sec olan (sec doğruysa c ≠ 0, değilse c = 0)
	// elemanlarını dstShape'e indirgeyerek dst'ye ekler; diğer elemanlar katkı vermez.
	WhereGrad(dst Storage, dstShape, outShape []int, g Storage, c View, pick bool)
	// Atan2Grad atan2(a, b)'nin gradyanları: ga += indirge(g·b/(a²+b²)), gb +=
	// indirge(−g·a/(a²+b²)); a = b = 0'da ve bir girdi sonsuzken (limit) terim 0'dır, NaN
	// girdide NaN. ga ya da gb nil olabilir.
	Atan2Grad(ga, gb Storage, aShape, bShape, outShape []int, g Storage, a, b View)

	// --- Eksen boyunca birikim ve sıralama ---

	// CumSum eksen boyunca birikimli toplam; out bitişik, x ile aynı şekil.
	CumSum(out Storage, x View, axis int)
	// CumSumGrad gx (shape, bitişik) += g'nin eksen boyunca sondan birikimli toplamı.
	CumSumGrad(gx, g Storage, shape []int, axis int)
	// Sort eksen boyunca kararlı sıralamanın (desc: büyükten küçüğe) ilk k elemanını
	// yazar: idx eksen indeksleri (float32 tam sayı), vals (nil olabilir) değerler; ikisi
	// de bitişik, x'in şekli eksende k ile. NaN her sayıdan büyüktür; eşitlerde küçük
	// indeks önce gelir (-0 ile +0 eşittir).
	Sort(vals, idx Storage, x View, axis, k int, desc bool)
	// GatherGrad gx (shape, bitişik) += eksen boyunca idx (k uzunluklu eksen; bir satırda
	// her indeks en çok bir kez) konumlarına g.
	GatherGrad(gx, g, idx Storage, shape []int, axis, k int)

	// --- İndirgemeler ---

	// Sum eksen boyunca toplam·scale; out bitişik ve eksensiz şekilde.
	Sum(out Storage, x View, axis int, scale float32)
	// SumAll tüm elemanların toplamı·scale; out tek elemanlı.
	SumAll(out Storage, x View, scale float32)
	// Max eksen boyunca en büyük değer.
	Max(out Storage, x View, axis int)
	// MaxGrad g'yi her satırın (ilk) en büyük elemanına ekler.
	MaxGrad(gx, g Storage, x View, axis int)
	// ArgMax eksen boyunca en büyük elemanın indeksleri (konakta).
	ArgMax(x View, axis int) []int
	// Softmax eksen boyunca (log) softmax; out bitişik.
	Softmax(out Storage, x View, axis int, log bool)
	// SoftmaxGrad gx += softmax geri yayılımı (y: ileri çıktı, bitişik).
	SoftmaxGrad(gx, g, y Storage, shape []int, axis int, log bool)

	// --- Matris çarpımı ---

	// Gemm her toplu eleman için C += op(A)·op(B). C (batch, M, N) bitişik;
	// A bitişik (batch, M, K) ya da transA ise (batch, K, M); B bitişik
	// (batch, K, N) ya da transB ise (batch, N, K). Ofsetler serbesttir.
	Gemm(batch, M, N, K int, a View, transA bool, b View, transB bool, c Storage)

	// --- Sinir ağı işlemleri ---

	Embedding(out Storage, w View, idx []int, D int)
	EmbeddingGrad(gw, g Storage, idx []int, D int)
	// CrossEntropy satır başına log-sum-exp (lse) ve geçerli hedefler üzerinden
	// ortalama kaybı (loss, tek eleman) üretir; negatif hedefler yok sayılır.
	CrossEntropy(x View, targets []int, C int) (loss, lse Storage)
	CrossEntropyGrad(gx, g Storage, x View, lse Storage, targets []int, C int)
	// LayerNorm son eksen (D) boyunca; gamma/beta nil olabilir. Satır başına
	// ortalama ve ters standart sapmayı geri yayılım için döndürür.
	LayerNorm(out Storage, x View, gamma, beta *View, D int, eps float32) (mean, rstd Storage)
	LayerNormGrad(gx, ggamma, gbeta, g Storage, x View, gamma *View, mean, rstd Storage, D int)
	// RMSNorm son eksen boyunca; satır başına 1/rms döndürür.
	RMSNorm(out Storage, x View, gamma *View, D int, eps float32) (rinv Storage)
	RMSNormGrad(gx, ggamma, g Storage, x View, gamma *View, rinv Storage, D int)
	// Attention döşemeli, çevrimiçi softmax'lı ölçekli nokta çarpımı dikkati
	// (bkz. AttentionParams; q, k, v bitişik, mask nil olabilir). out bitişik
	// [Batch, H, Tq, Dv]; satır başına logsumexp'i (float32, Batch·H·Tq) döndürür.
	Attention(out Storage, q, k, v View, mask Storage, p AttentionParams) (lse Storage)
	// AttentionGrad puanları yeniden hesaplayarak gq, gk, gv'ye (nil olanlar
	// atlanır) gradyan ekler; o ileri çıktı, lse ileri geçişin logsumexp'idir.
	AttentionGrad(gq, gk, gv, g Storage, q, k, v View, mask Storage, o, lse Storage, p AttentionParams)

	// --- İyileştirici ---

	// SGDStep p -= lr·(momentumlu) (g + wd·p); vel nil ise momentum yok.
	SGDStep(p, g, vel Storage, lr, momentum, wd float32)
	// AdamWStep ayrık ağırlık çürümeli Adam adımı; bc1, bc2 sapma düzeltmeleri.
	AdamWStep(p, g, m, v Storage, lr, b1, b2, eps, wd, bc1, bc2 float32)
	// SumSquares Σ s² (konakta, float64).
	SumSquares(s Storage) float64
	// Scale s *= k.
	Scale(s Storage, k float32)
}

// Releaser arka ucun isteğe bağlı olarak uyguladığı arayüzdür: artık hiçbir
// görünümün bakmadığı bir depolamayı geri alır ve sonraki Alloc çağrılarında
// yeniden kullanabilir. Otomatik türev yalnızca geri yayılımda işi biten ARA
// gradyan depolamalarını iade eder; ileri yön tamponları (kullanıcının elinde
// görünümleri olabilir) iade edilmez.
type Releaser interface {
	Release(s Storage)
}
