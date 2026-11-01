package ml

import (
	"errors"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ----------------------------------------------------------------------------
// TR değerinden tensöre dönüşüm
// ----------------------------------------------------------------------------

// toTensor sayı, iç içe liste ya da tensörü çekirdek tensörüne çevirir.
// Tensör verilirse kendisi döner (kopyalanmaz, hesap çizgesi korunur).
func toTensor(fn any, v interp.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *TensorValue:
		return x.t, nil
	case *interp.IntVal:
		if !exactInFloat32(x.Val) {
			return nil, inexactIntError(fn, "", x.Val)
		}
		return tensor.Scalar(float32(x.Val)), nil
	case *interp.FloatVal:
		return tensor.Scalar(float32(x.Val)), nil
	case *interp.ListVal:
		data, sh, err := listData(fn, x)
		if err != nil {
			return nil, err
		}
		return tensor.New(data, sh...), nil
	}
	if b, ok := v.(*interp.BuiltinVal); ok {
		return nil, messages.New(messages.H6033, fn, typeName(v), b.Name)
	}
	return nil, messages.New(messages.H6032, fn, typeName(v))
}

// toTensorOn toTensor gibidir; ancak sayı ve listeden oluşan tensör, işlemin
// diğer tarafı olan tensörün cihazında kurulur (ör. GPU'daki t için t + [1, 2]). Tensör
// verilirse kendisi döner; cihazı farklıysa işlem açık hata verir (15.40).
func toTensorOn(fn any, v interp.Value, otherVal *tensor.Tensor) (*tensor.Tensor, error) {
	if x, ok := interp.ToFloat(v); ok {
		// Sayı cihazda doldurulur (konaktan yüklenmez; tekrar_oynat kaydında da çalışır).
		return safe(func() *tensor.Tensor { return tensor.FullOn(otherVal.Device(), float32(x)) })
	}
	t, err := toTensor(fn, v)
	if err != nil {
		return nil, err
	}
	if _, isTensor := v.(*TensorValue); isTensor || t.Device() == otherVal.Device() {
		return t, nil
	}
	return safe(func() *tensor.Tensor {
		var r *tensor.Tensor
		tensor.NoGrad(func() { r = t.To(otherVal.Device()) })
		return r
	})
}

// listData iç içe listeyi düz veri ve şekle çevirir; düzensiz listeleri reddeder. Bütün
// öğeleri tam sayı olan bir listede (tek tam sayıda da, toTensor) float32'de tam yazılamayan
// değer (|n| > 2²⁴) çalışma hatasıdır (15.1'in ara kuralı: tam sayı tensörü bu sürümde yok, yuvarlama sessiz olurdu);
// karışık liste ([1, 2.5]) ondalıktır ve yuvarlar (15.2).
func listData(fn any, l *interp.ListVal) ([]float32, []int, error) {
	var data []float32
	ints, inexact := true, -1
	var inexactVal int64
	sh, err := walkList(fn, l, func(n int) { data = make([]float32, 0, n) }, func(v interp.Value) error {
		if n, integer := v.(*interp.IntVal); !integer {
			ints = false
		} else if inexact < 0 && !exactInFloat32(n.Val) {
			inexact, inexactVal = len(data), n.Val
		}
		f, ok := interp.ToFloat(v)
		if !ok {
			return messages.New(messages.H6007, fn, interp.Shown(v), typeName(v))
		}
		data = append(data, float32(f))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if ints && inexact >= 0 {
		return nil, nil, inexactIntError(fn, positionText(inexact, sh), inexactVal)
	}
	return data, sh, nil
}

// inexactIntError float32'de tam yazılamayan tam sayıdan tensör kurmanın hatasıdır (H6954);
// position listedeki konumdur, tek sayıda boş. rounded float32'nin tam değeridir (en kısa
// float32 yazımı, en kısa float64 yazımı da değil: 2⁵³ + 1 için 9007199254740992, −2⁶³ için
// -9223372036854775808).
func inexactIntError(fn any, position string, n int64) error {
	rounded := strconv.FormatFloat(float64(float32(n)), 'f', 0, 64) // 2²⁴'ün üstünde float32 tam sayıdır
	return messages.New(messages.H6954, fn, position, n, rounded)
}

// exactInFloat32 n float32'de tam yazılabilir mi? 2²⁴'e kadar her tam sayı yazılır; üstünde
// yalnız yuvarlanınca aynı kalanlar.
func exactInFloat32(n int64) bool {
	if n >= -1<<24 && n <= 1<<24 {
		return true
	}
	f := float64(float32(n))
	// 2⁶³'e yuvarlanan değer int64'e sığmaz; n ondan küçük olduğu için tam değildir.
	return f < 1<<63 && f >= -(1<<63) && int64(f) == n
}

// positionText düz konumu iç içe listedeki indeks yazımına çevirir: [1][0].
func positionText(flat int, sh []int) string {
	idx := make([]string, len(sh))
	for a := len(sh) - 1; a >= 0; a-- {
		idx[a] = "[" + strconv.Itoa(flat%sh[a]) + "]"
		flat /= sh[a]
	}
	return strings.Join(idx, "")
}

// listInts iç içe tam sayı listesini düz dizi ve şekle çevirir. Değerler doğrudan
// tam sayı olarak okunur (float32'den geçirilmez; 2^24 üstü indeksler yuvarlanmaz).
func listInts(fn any, l *interp.ListVal) ([]int, []int, error) {
	var data []int
	sh, err := walkList(fn, l, func(n int) { data = make([]int, 0, n) }, func(v interp.Value) error {
		n, ok := intNumber(v)
		if !ok {
			if _, number := interp.ToFloat(v); number {
				return messages.New(messages.H6012, fn, interp.Shown(v))
			}
			return messages.New(messages.H6013, fn, interp.Shown(v), typeName(v))
		}
		data = append(data, n)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return data, sh, nil
}

// walkList iç içe listenin şeklini bulur (ilk elemanlar boyunca inilerek), düzensizliği
// reddeder ve yaprakları sırayla yaprak'a verir; hazirla eleman sayısıyla bir kez çağrılır.
func walkList(fn any, l *interp.ListVal, prepare func(n int), leaf func(v interp.Value) error) ([]int, error) {
	var sh []int
	var cur interp.Value = l
	var walker interp.ContainerWalker // kendini içeren liste ([a] içinde a) sonsuz şekil verirdi
	for {
		ll, ok := cur.(*interp.ListVal)
		if !ok {
			break
		}
		if err := walker.Enter(ll, len(sh)); err != nil {
			if errors.Is(err, interp.ErrCyclic) {
				return nil, messages.New(messages.H6018, fn)
			}
			return nil, messages.New(messages.H6019, fn, interp.MaxExternalDepth)
		}
		sh = append(sh, len(ll.Elements))
		if len(ll.Elements) == 0 {
			break
		}
		cur = ll.Elements[0]
	}
	n := 1
	for _, d := range sh {
		n *= d
		if n > interp.MaxCollectionSize {
			return nil, messages.New(messages.H6022, fn, interp.MaxCollectionSize)
		}
	}
	prepare(n)
	var walk func(v interp.Value, axis int) error
	walk = func(v interp.Value, axis int) error {
		if axis == len(sh) {
			if _, list := v.(*interp.ListVal); list {
				return messages.New(messages.H6025, fn, axis+1)
			}
			return leaf(v)
		}
		ll, ok := v.(*interp.ListVal)
		if !ok {
			return messages.New(messages.H6026, fn, axis+1, interp.Shown(v), tensor.ShapeText(sh))
		}
		if len(ll.Elements) != sh[axis] {
			return messages.New(messages.H6027, fn, axis+1, sh[axis], len(ll.Elements))
		}
		for _, el := range ll.Elements {
			if err := walk(el, axis+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(l, 0); err != nil {
		return nil, err
	}
	return sh, nil
}
