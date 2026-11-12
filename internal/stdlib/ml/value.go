// Package ml, n-boyutlu tensör çekirdeğini (github.com/sekerahmet/TCB/internal/compute/tensor) TR diline bağlar:
// tensör değeri, işleçleri (@ + - * /), özellik ve metotları, matematik ve sinir ağı
// fonksiyonları, türevsiz çalışma, iyileştiriciler ve hazır sinir ağı modeli.
//
// Dilin tek sayısal dizi türü tensördür: tek sayıdan tensör 0 boyutlu, matris adlı
// oluşturucuların (sıfırlar([2, 3]), birim_matris ...) sonucu 2 boyutlu tensördür.
//
// Aynı adı taşıyan çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama, min, max,
// karekök) silinmez; kayıt sırasında sarılır: argüman tensörse yeni yol, değilse eski
// yerleşik çalışır. Bu yüzden bu motor onlardan SONRA kaydedilmelidir (bkz. internal/stdlib).
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// TensorValue, TR'de bir tensörü taşıyan değerdir. Tip adı "Tensör"dür.
type TensorValue struct {
	t *tensor.Tensor
	// intermediate, tensörün türev izlenen bir işlemin sonucu (yaprak olmayan) olduğunu gösterir.
	// Çekirdek, geri yayılımdan sonra ara düğümlerin çizge bağlarını bıraktığı için bu
	// bilgi oluşturma anında saklanır; böylece ara tensörün .gradyan'ı sessizce 'yok'
	// dönmez ve ara tensör iyileştirici parametresi olarak kabul edilmez.
	intermediate bool
	// own yorumlayıcının sahiplik durumudur: değer bırakılınca başlığın bu sarıcısı düşer
	// (dil tanımı 15.43; internal/lang/interp/ownership.go).
	own interp.Ownership
}

// New çekirdek tensörünü TR değeri olarak sarar. Aynı başlığı birden çok değer sarabilir;
// başlık hepsi bırakılınca düşer (tensor.Wrap).
func New(t *tensor.Tensor) *TensorValue {
	t.Wrap()
	return &TensorValue{t: t, intermediate: t.RequiresGrad() && !t.IsLeaf()}
}

var (
	_ interp.Owned        = (*TensorValue)(nil)
	_ interp.AssignCopier = (*TensorValue)(nil)
)

// Ownership (interp.Owned) değerin sahiplik durumu.
func (d *TensorValue) Ownership() *interp.Ownership { return &d.own }

// Release (interp.Owned) yorumlayıcı değere artık ulaşılamayacağını bildiğinde çağırır:
// başlığın bu sarıcısı düşer, son sarıcıysa depolamanın sahibi azalır.
func (d *TensorValue) Release() { d.t.Unwrap() }

// CopiesAssigned (interp.AssignCopier) t[i] = değer atanan değeri kopyalar, saklamaz.
func (d *TensorValue) CopiesAssigned() {}

// markFresh v'yi (tensör ya da tensör listesi) start'tan sonra oluşturulmuş başlıkları
// taşıyorsa taze işaretler: yorumlayıcı onu çalışan çağrının sahipliğine alır.
func markFresh(v interp.Value, start uint64) {
	switch x := v.(type) {
	case *TensorValue:
		if x.t.Serial() > start {
			interp.MarkFresh(x)
		}
	case *interp.ListVal:
		fresh := false
		for _, e := range x.Elements {
			if d, ok := e.(*TensorValue); ok && d.t.Serial() > start {
				interp.MarkFresh(d)
				fresh = true
			}
		}
		if fresh {
			interp.MarkFresh(x)
		}
	}
}

// NewInt değerleri tam sayı olan bir tensörü (veri kümesinin kimlikleri, sıralama indeksleri)
// TR_Int tensörü ("int64", 15.1) olarak sarar; ondalık tensör aynı cihazda çevrilir.
func NewInt(t *tensor.Tensor) *TensorValue { return New(intTensor(t)) }

// intTensor tam sayı değerli ondalık tensörü "int64" tensörüne çevirir (konakta; değerler zaten
// tam sayıdır, kesmek bir şey değiştirmez); tam sayı tensörü aynen döner.
func intTensor(t *tensor.Tensor) *tensor.Tensor {
	if t.Elem() == tensor.ElemInt {
		return t
	}
	data := t.Data()
	vals := make([]int64, len(data))
	for i, x := range data {
		vals[i] = int64(x)
	}
	return tensor.Int64sOn(t.Device(), tensor.Int64, vals, t.Shape()...)
}

// sortIndex sıralamanın (TR_topk, TR_argsort) float32 indeksini TR_Int tensörü olarak sarar.
// Kartta indeks konağa okunmaz: değerleri tam sayı olduğu sıralamanın sözleşmesinden bilinen
// kaynak için yuvasız iç dönüşümle kartta çevrilir, TR_replay kaydında da çalışır (T1 dilim 2b,
// A15; DENETİM 501 Ş1). Yuvasız giriş yalnız buradadır, NewInt'in genel yolu değişmez (DENETİM
// 503 N1). İşlemcide NewInt'in yoludur.
func sortIndex(t *tensor.Tensor) *TensorValue {
	if t.Device() == tensor.CPU {
		return NewInt(t)
	}
	return New(tensor.ConvertSortIndex(t, tensor.Int64))
}

// Tensor sarılı çekirdek tensörünü verir.
func (d *TensorValue) Tensor() *tensor.Tensor { return d.t }

func (d *TensorValue) Type() interp.ValueType { return interp.ValTensor }

// IsTruthy tek elemanlı tensörde elemanın 0'dan farklı olup olmadığıdır; boş tensör
// yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; yorumlayıcı bu arayüzden
// hata alamadığı için şimdilik "boş değil" (doğru) sayılır.
func (d *TensorValue) IsTruthy() bool {
	switch d.t.Numel() {
	case 0:
		return false
	case 1:
		if d.t.DType().IsTyped() {
			return d.scalarValue().IsTruthy() || isNonzeroFloat(d.scalarValue())
		}
		return d.t.Item() != 0
	}
	return true
}

// isNonzeroFloat Ondalık değerin 0'dan farklı olduğu (float64 tensörün doğruluk değeri).
func isNonzeroFloat(v interp.Value) bool {
	f, ok := v.(*interp.FloatVal)
	return ok && f.Val != 0
}

// IntElements tensörün bütün elemanlarını düz tam sayı dizisi olarak verir. Tam sayı olmayan bir
// eleman varsa ok yanlıştır ve bad o ilk elemandır (çağıran kendi iletisini kurar).
func (d *TensorValue) IntElements() (ids []int, bad float32, ok bool) {
	if d.t.Elem() == tensor.ElemInt {
		vals := d.t.Int64s()
		ids = make([]int, len(vals))
		for i, v := range vals {
			ids[i] = int(v)
		}
		return ids, 0, true
	}
	if d.t.Elem() == tensor.ElemBool {
		return nil, 0, false
	}
	if d.t.DType() == tensor.Float64 {
		vals := d.t.Float64s()
		ids = make([]int, len(vals))
		for i, f := range vals {
			if float64(int(f)) != f {
				return nil, float32(f), false
			}
			ids[i] = int(f)
		}
		return ids, 0, true
	}
	data := d.t.Data()
	ids = make([]int, len(data))
	for i, f := range data {
		if float32(int(f)) != f {
			return nil, f, false
		}
		ids[i] = int(f)
	}
	return ids, 0, true
}
