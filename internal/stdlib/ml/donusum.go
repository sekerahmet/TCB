package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// ----------------------------------------------------------------------------
// TR değerinden tensöre dönüşüm
// ----------------------------------------------------------------------------

// tensoreCevir sayı, iç içe liste ya da tensörü çekirdek tensörüne çevirir.
// Tensör verilirse kendisi döner (kopyalanmaz, hesap çizgesi korunur).
func tensoreCevir(fn string, v interp.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		return x.t, nil
	case *interp.IntVal:
		return tensor.Scalar(float32(x.Val)), nil
	case *interp.FloatVal:
		return tensor.Scalar(float32(x.Val)), nil
	case *interp.ListVal:
		veri, sh, err := listeVerisi(fn, x)
		if err != nil {
			return nil, err
		}
		return tensor.New(veri, sh...), nil
	}
	ipucu := ""
	if b, ok := v.(*interp.BuiltinVal); ok {
		ipucu = fmt.Sprintf("; metodu çağırmak için parantez ekleyin: %s()", b.Name)
	}
	return nil, fmt.Errorf("%s: %s tipi tensöre dönüştürülemez (sayı, iç içe liste ya da tensör bekleniyor)%s", fn, tipAdi(v), ipucu)
}

// tensoreCevirCihaz tensoreCevir gibidir; ancak sayı ve listeden oluşan tensör, işlemin
// diğer tarafı olan tensörün cihazında kurulur (ör. GPU'daki t için t + [1, 2]). Tensör
// verilirse kendisi döner; cihazı farklıysa işlem açık hata verir (15.29).
func tensoreCevirCihaz(fn string, v interp.Value, diger *tensor.Tensor) (*tensor.Tensor, error) {
	t, err := tensoreCevir(fn, v)
	if err != nil {
		return nil, err
	}
	if _, tensorMu := v.(*Deger); tensorMu || t.Device() == diger.Device() {
		return t, nil
	}
	return guvenli(func() *tensor.Tensor {
		var r *tensor.Tensor
		tensor.NoGrad(func() { r = t.To(diger.Device()) })
		return r
	})
}

// listeVerisi iç içe listeyi düz veri ve şekle çevirir; düzensiz listeleri reddeder.
func listeVerisi(fn string, l *interp.ListVal) ([]float32, []int, error) {
	var veri []float32
	sh, err := listeGez(fn, l, func(n int) { veri = make([]float32, 0, n) }, func(v interp.Value) error {
		f, ok := interp.ToFloat(v)
		if !ok {
			return fmt.Errorf("%s: tensör elemanları sayı olmalıdır, %s (%s) verildi", fn, v.Inspect(), tipAdi(v))
		}
		veri = append(veri, float32(f))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return veri, sh, nil
}

// listeTamSayilari iç içe tam sayı listesini düz dizi ve şekle çevirir. Değerler doğrudan
// tam sayı olarak okunur (float32'den geçirilmez; 2^24 üstü indeksler yuvarlanmaz).
func listeTamSayilari(fn string, l *interp.ListVal) ([]int, []int, error) {
	var veri []int
	sh, err := listeGez(fn, l, func(n int) { veri = make([]int, 0, n) }, func(v interp.Value) error {
		n, ok := tamSayi(v)
		if !ok {
			if _, sayi := interp.ToFloat(v); sayi {
				return fmt.Errorf("%s: indeksler tam sayı olmalıdır, %s verildi", fn, v.Inspect())
			}
			return fmt.Errorf("%s: indeksler tam sayı olmalıdır, %s (%s) verildi", fn, v.Inspect(), tipAdi(v))
		}
		veri = append(veri, n)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return veri, sh, nil
}

// listeGez iç içe listenin şeklini bulur (ilk elemanlar boyunca inilerek), düzensizliği
// reddeder ve yaprakları sırayla yaprak'a verir; hazirla eleman sayısıyla bir kez çağrılır.
func listeGez(fn string, l *interp.ListVal, hazirla func(n int), yaprak func(v interp.Value) error) ([]int, error) {
	var sh []int
	var cur interp.Value = l
	for {
		ll, ok := cur.(*interp.ListVal)
		if !ok {
			break
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
			return nil, fmt.Errorf("%s: liste çok büyük (en fazla %d eleman)", fn, interp.MaxCollectionSize)
		}
	}
	hazirla(n)
	var gez func(v interp.Value, eksen int) error
	gez = func(v interp.Value, eksen int) error {
		if eksen == len(sh) {
			if _, liste := v.(*interp.ListVal); liste {
				return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte sayı yerine liste var", fn, eksen+1)
			}
			return yaprak(v)
		}
		ll, ok := v.(*interp.ListVal)
		if !ok {
			return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte liste yerine %s var (beklenen şekil %s)", fn, eksen+1, v.Inspect(), sekilMetni(sh))
		}
		if len(ll.Elements) != sh[eksen] {
			return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte %d ve %d uzunlukta listeler var (tüm satırlar eşit uzunlukta olmalı)", fn, eksen+1, sh[eksen], len(ll.Elements))
		}
		for _, el := range ll.Elements {
			if err := gez(el, eksen+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := gez(l, 0); err != nil {
		return nil, err
	}
	return sh, nil
}
