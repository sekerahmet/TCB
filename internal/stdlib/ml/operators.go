package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"io"
	"os"
	"sync"
)

// ----------------------------------------------------------------------------
// İşleçler (VM kancaları yurutucu_kancalari.go'dadır)
// ----------------------------------------------------------------------------

var opName = map[string]string{"+": "toplama", "-": "çıkarma", "*": "çarpma", "/": "bölme"}

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
			ad, ok := opName[op]
			if !ok {
				ad = op
			}
			return fmt.Errorf("%s ile %s şekilleri %s için yayınlanamaz; sondan %d. eksende %d ≠ %d (eksenler sağdan hizalanır, eşit ya da 1 olmalı)",
				shapeText(a.Shape()), shapeText(b.Shape()), ad, i, da, db)
		}
	}
	return nil
}

func checkDevice(operation string, a, b *tensor.Tensor) error {
	if a.Device() != b.Device() {
		return fmt.Errorf("%s: tensörler farklı cihazlarda (%s ve %s); önce .%s(...) ile aynı cihaza taşıyın", operation, a.Device(), b.Device(), codes.Name(memberToDevice))
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

var (
	isWarning       sync.Mutex
	warningOutput   io.Writer = os.Stderr
	givenWarning              = map[string]bool{}
	warningSilenced bool      // sınır bildirimi yapıldı mı
)

// maxBroadcastWarnings gösterilecek en çok farklı yayın uyarısı (işlem ve şekil çifti).
const maxBroadcastWarnings = 100

// broadcastWarning iki tarafın da genişletildiği yayınlamayı standart hataya bir kez yazar.
// İşleçler satır bilgisini almadığından (yorumlayıcı işleç çağrısına satır geçirmez)
// tekrar, işlem ve şekil çiftine göre önlenir.
func broadcastWarning(op string, a, b *tensor.Tensor) {
	as, bs := shapeText(a.Shape()), shapeText(b.Shape())
	key := op + as + bs
	isWarning.Lock()
	defer isWarning.Unlock()
	if givenWarning[key] {
		return
	}
	// Harita sınırlıdır: değişken uzunluklu dizilerde her şekil çifti yeni bir anahtar
	// olurdu. Sınıra varılınca bir kez bildirilir, sonraki yeni uyarılar gösterilmez.
	if len(givenWarning) >= maxBroadcastWarnings {
		if !warningSilenced {
			warningSilenced = true
			fmt.Fprintf(warningOutput, "Uyarı: %d farklı yayın uyarısı gösterildi; sonraki yayın uyarıları gösterilmeyecek.\n", maxBroadcastWarnings)
		}
		return
	}
	givenWarning[key] = true
	result := make([]int, max(a.Dim(), b.Dim()))
	for i := 1; i <= len(result); i++ {
		result[len(result)-i] = max(axisFromEnd(a, i), axisFromEnd(b, i))
	}
	fmt.Fprintf(warningOutput, "Uyarı: %s ile %s yayınlanarak %s üretildi (iki taraf da genişletildi); bilerek yapmadıysanız şekilleri kontrol edin.\n",
		as, bs, shapeText(result))
}

func (d *TensorValue) binary(op string, other interp.Value) (interp.Value, error) {
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
		if o, err = toTensorOn("tensör "+opName[op], other, d.t); err != nil {
			return nil, err
		}
	}
	if err := checkBroadcast(op, d.t, o); err != nil {
		return nil, err
	}
	if d.t.Device() != o.Device() {
		return nil, checkDevice("tensör "+opName[op], d.t, o)
	}
	if bothWiden(d.t, o) {
		broadcastWarning(op, d.t, o)
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
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; skaler için '*' kullanın", shapeText(as), shapeText(bs))
	}
	if err := checkDevice("matris çarpımı (@)", a, b); err != nil {
		return nil, err
	}
	K := as[len(as)-1]
	var bK int
	if len(bs) == 1 {
		bK = bs[0]
	} else {
		bK = bs[len(bs)-2]
	}
	if K != bK {
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; %d ≠ %d (soldakinin son ekseni sağdakinin %s eksenine eşit olmalı)",
			shapeText(as), shapeText(bs), K, bK, map[bool]string{true: "tek", false: "sondan ikinci"}[len(bs) == 1])
	}
	if len(as) >= 2 && len(bs) > 2 {
		onA, onB := as[:len(as)-2], bs[:len(bs)-2]
		if len(onA) != len(onB) || shapeText(onA) != shapeText(onB) {
			return nil, fmt.Errorf("%s ile %s toplu matris çarpımı yapılamaz; öndeki eksenler eşit olmalı (%s ≠ %s)",
				shapeText(as), shapeText(bs), shapeText(onA), shapeText(onB))
		}
	}
	if len(as) == 1 && len(bs) > 2 {
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; vektör toplu matrisle çarpılamaz", shapeText(as), shapeText(bs))
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
			r = tensor.MatMulOut(x.Cast(mixed), y.Cast(mixed), mixed)
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
