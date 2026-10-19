// Package ml, n-boyutlu tensör çekirdeğini (github.com/sekerahmet/TCB/internal/compute/tensor) TR diline bağlar:
// tensör değeri, işleçleri (@ + - * /), özellik ve metotları, matematik ve sinir ağı
// fonksiyonları, türevsiz çalışma, iyileştiriciler ve hazır sinir ağı modeli.
//
// Dilin tek sayısal dizi türü tensördür: tek sayıdan tensör 0 boyutlu, matris adlı
// oluşturucuların (sıfırlar([2, 3]), birim_matris ...) sonucu 2 boyutlu tensördür.
//
// Aynı adı taşıyan çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama, min, maks,
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
	// hasInts, değerlerin tam sayı olduğunu (ör. maks_indeks sonucu) gösterir: .değer
	// Tam döndürür, yazdırma ondalıksız yapılır. İşlem sonuçlarına geçmez.
	hasInts bool
}

// New çekirdek tensörünü TR değeri olarak sarar.
func New(t *tensor.Tensor) *TensorValue {
	return &TensorValue{t: t, intermediate: t.RequiresGrad() && !t.IsLeaf()}
}

// NewInt değerleri tam sayı olan (ör. token kimlikleri) bir tensörü sarar: .değer Tam
// verir ve tensör ondalıksız yazdırılır.
func NewInt(t *tensor.Tensor) *TensorValue {
	d := New(t)
	d.hasInts = true
	return d
}

// Tensor sarılı çekirdek tensörünü verir.
func (d *TensorValue) Tensor() *tensor.Tensor { return d.t }

func (d *TensorValue) Type() interp.ValueType { return interp.VAL_TENSOR }

// IsTruthy tek elemanlı tensörde elemanın 0'dan farklı olup olmadığıdır; boş tensör
// yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; yorumlayıcı bu arayüzden
// hata alamadığı için şimdilik "boş değil" (doğru) sayılır.
func (d *TensorValue) IsTruthy() bool {
	switch d.t.Numel() {
	case 0:
		return false
	case 1:
		return d.t.Item() != 0
	}
	return true
}
