package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"sync"
)

// ----------------------------------------------------------------------------
// İşleçler (VM kancaları yurutucu_kancalari.go'dadır)
// ----------------------------------------------------------------------------

// opName dört işlemin hata iletilerindeki adıdır (toplama, çıkarma, çarpma, bölme).
var opName = map[string]messages.Code{"+": messages.H6119, "-": messages.H6120, "*": messages.H6121, "/": messages.H6122}

// operationName işlecin adını verir; adı olmayan işleç olduğu gibi yazılır.
func operationName(op string) string {
	if c, ok := opName[op]; ok {
		return messages.Text(c)
	}
	return op
}

// tensorOperation hata iletilerinde tensör işlemini adlandırır ("tensör toplama").
func tensorOperation(op string) string { return messages.Text(messages.H6123, operationName(op)) }

// axisFromEnd tensörün sondan i. (1'den başlayarak) ekseninin uzunluğunu verir; o eksen
// yoksa yayınlama kuralı gereği 1 sayılır. Şekil kopyalanmaz.
func axisFromEnd(t *tensor.Tensor, i int) int {
	if j := t.Dim() - i; j >= 0 {
		return t.Size(j)
	}
	return 1
}

// checkBroadcast iki tensörün şekillerinin yayınlanabilirliğini denetler (eksenler sağdan
// hizalanır; eşit ya da biri 1 olan eksenler uyumludur). Sık çağrıldığı için ayırma
// yapmaz; şekil metni yalnızca hata durumunda üretilir. op bir işleçse (+ - * /) işlemin
// adı, değilse (ör. bir yerleşiğin adı) olduğu gibi yazılır.
func checkBroadcast(op string, a, b *tensor.Tensor) error {
	nd := max(a.Dim(), b.Dim())
	for i := 1; i <= nd; i++ {
		da, db := axisFromEnd(a, i), axisFromEnd(b, i)
		if da != db && da != 1 && db != 1 {
			ad := operationName(op)
			return messages.New(messages.H6100, shapeText(a.Shape()), shapeText(b.Shape()), ad, i, da, db)
		}
	}
	return nil
}

func checkDevice(operation string, a, b *tensor.Tensor) error {
	if a.Device() != b.Device() {
		return messages.New(messages.H6102, operation, a.Device(), b.Device(), codes.DisplayName(memberToDevice))
	}
	return nil
}

// bothWiden yayınlamada iki işlenenin de genişletilip genişletilmediğini söyler
// (ör. [4, 1] ile [4] → [4, 4]). Bu çoğunlukla bir şekil hatasıdır. Ayırma yapmaz.
func bothWiden(a, b *tensor.Tensor) bool {
	nd := max(a.Dim(), b.Dim())
	var aWide, bWide bool
	for i := 1; i <= nd; i++ {
		da, db := axisFromEnd(a, i), axisFromEnd(b, i)
		size := max(da, db)
		if da == 0 || db == 0 {
			size = 0
		}
		// Eksik eksen de (ön tarafa eklenen) genişletme sayılır, ancak uzunluğu 1 ise
		// veri çoğalmaz.
		if da != size || (i > a.Dim() && size != 1) {
			aWide = true
		}
		if db != size || (i > b.Dim() && size != 1) {
			bWide = true
		}
	}
	return aWide && bWide
}

// broadcastWarnings bir programın gösterdiği yayın uyarılarıdır; programa bağlıdır
// (interp.VM.ProgramValue): aynı süreçte çalışan başka bir program (tr test'in öbür dosyaları)
// kendi uyarılarını baştan gösterir.
type broadcastWarnings struct {
	mu       sync.Mutex
	given    map[string]bool
	silenced bool // sınır bildirimi yapıldı mı
}

type warningsKey struct{}

// warningsOf programın yayın uyarıları.
func warningsOf(m *interp.VM) *broadcastWarnings {
	return m.ProgramValue(warningsKey{}, func() any { return &broadcastWarnings{given: map[string]bool{}} }).(*broadcastWarnings)
}

// maxBroadcastWarnings gösterilecek en çok farklı yayın uyarısı (işlem ve şekil çifti).
const maxBroadcastWarnings = 100

// broadcastWarning iki tarafın da genişletildiği yayınlamayı programın hata akışına (m.Err) bir
// kez yazar. İşleçler satır bilgisini almadığından (yorumlayıcı işleç çağrısına satır
// geçirmez) tekrar, işlem ve şekil çiftine göre önlenir.
func broadcastWarning(m *interp.VM, op string, a, b *tensor.Tensor) {
	as, bs := shapeText(a.Shape()), shapeText(b.Shape())
	key := op + as + bs
	w := warningsOf(m)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.given[key] {
		return
	}
	// Harita sınırlıdır: değişken uzunluklu dizilerde her şekil çifti yeni bir anahtar
	// olurdu. Sınıra varılınca bir kez bildirilir, sonraki yeni uyarılar gösterilmez.
	if len(w.given) >= maxBroadcastWarnings {
		if !w.silenced {
			w.silenced = true
			fmt.Fprintln(m.Err, messages.Text(messages.H6127, maxBroadcastWarnings))
		}
		return
	}
	w.given[key] = true
	result := make([]int, max(a.Dim(), b.Dim()))
	for i := 1; i <= len(result); i++ {
		result[len(result)-i] = max(axisFromEnd(a, i), axisFromEnd(b, i))
	}
	fmt.Fprintln(m.Err, messages.Text(messages.H6128, as, bs, shapeText(result)))
}

func (d *TensorValue) binary(m *interp.VM, op string, other interp.Value) (interp.Value, error) {
	if s, ok := interp.ToFloat(other); ok {
		f := float32(s)
		r, err := safe(func() *tensor.Tensor {
			switch op {
			case "+":
				return d.t.AddScalar(f)
			case "-":
				return d.t.SubScalar(f)
			case "*":
				return d.t.MulScalar(f)
			default:
				// Bölme, 1/s ile çarpma yerine gerçek bölmeyle yapılır (yuvarlama farkı olmasın);
				// sonuç tensörün türündedir (* + - gibi).
				return d.t.DivScalar(f)
			}
		})
		if err != nil {
			return nil, err
		}
		return New(r), nil
	}
	// Sık yol (iki tensör) metin üretmez; işlem adı yalnızca hata mesajında birleştirilir.
	var o *tensor.Tensor
	if od, ok := other.(*TensorValue); ok {
		o = od.t
	} else {
		var err error
		if o, err = toTensorOn(tensorOperation(op), other, d.t); err != nil {
			return nil, err
		}
	}
	if err := checkBroadcast(op, d.t, o); err != nil {
		return nil, err
	}
	if d.t.Device() != o.Device() {
		return nil, checkDevice(tensorOperation(op), d.t, o)
	}
	if m != nil && bothWiden(d.t, o) {
		broadcastWarning(m, op, d.t, o)
	}
	r, err := safe(func() *tensor.Tensor {
		switch op {
		case "+":
			return tensor.Add(d.t, o)
		case "-":
			return tensor.Sub(d.t, o)
		case "*":
			return tensor.Mul(d.t, o)
		default:
			return tensor.Div(d.t, o)
		}
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// matmulDType '@' işlecidir: matris çarpımı. 1 boyutlu işlenenler vektör sayılır.
// mixed Float32 değilse (karışık_hassasiyet bloğu) girdiler o türe çevrilir (türevlenebilir
// kopya; ağırlıklar değişmez) ve çarpım float32 biriktirmeyle o türde sonuç verir (bir kez
// yuvarlanır; PyTorch autocast'iyle aynı kural). Aksi halde sonuç girdilerin türündedir
// (türler farklıysa float32).
func matmulDType(a, b *tensor.Tensor, mixed tensor.DType) (*tensor.Tensor, error) {
	as, bs := a.Shape(), b.Shape()
	if len(as) == 0 || len(bs) == 0 {
		return nil, messages.New(messages.H6114, shapeText(as), shapeText(bs))
	}
	if err := checkDevice(messages.Text(messages.H6124), a, b); err != nil {
		return nil, err
	}
	K := as[len(as)-1]
	var bK int
	if len(bs) == 1 {
		bK = bs[0]
	} else {
		bK = bs[len(bs)-2]
	}
	if K != bK && len(bs) == 1 {
		return nil, messages.New(messages.H6125, shapeText(as), shapeText(bs), K, bK)
	}
	if K != bK {
		return nil, messages.New(messages.H6126, shapeText(as), shapeText(bs), K, bK)
	}
	if len(as) >= 2 && len(bs) > 2 {
		onA, onB := as[:len(as)-2], bs[:len(bs)-2]
		if len(onA) != len(onB) || shapeText(onA) != shapeText(onB) {
			return nil, messages.New(messages.H6115, shapeText(as), shapeText(bs), shapeText(onA), shapeText(onB))
		}
	}
	if len(as) == 1 && len(bs) > 2 {
		return nil, messages.New(messages.H6117, shapeText(as), shapeText(bs))
	}
	return safe(func() *tensor.Tensor {
		x, y := a, b
		if len(as) == 1 {
			x = x.Unsqueeze(0)
		}
		if len(bs) == 1 {
			y = y.Unsqueeze(1)
		}
		var r *tensor.Tensor
		if mixed != tensor.Float32 {
			r = tensor.MatMulOut(x.CastShared(mixed), y.CastShared(mixed), mixed)
		} else {
			r = tensor.MatMul(x, y)
		}
		if len(bs) == 1 {
			r = r.Squeeze(-1)
		}
		if len(as) == 1 {
			r = r.Squeeze(0)
		}
		return r
	})
}
