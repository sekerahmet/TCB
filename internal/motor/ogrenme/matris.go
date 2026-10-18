package ogrenme

import (
	"fmt"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// Bu dosya matris biçimli yerleşikleri (birim_matris, sıfırlar(satır, sütun), birler(satır,
// sütun), devrik) uygular. Dilde ayrı bir matris türü yoktur: matris 2 boyutlu bir
// tensördür ve tensör([[1, 2], [3, 4]]) ile oluşturulur (rastgele olanı rastgele_normal([2,
// 3]) ile); @, + - * /, relu, softmax ... tensör yolundan geçer.

// matrisCevir matris alan işlevlerin girdisini 2
// boyutlu tensöre çevirir: iç içe sayı listesi [satır, sütun], düz sayı listesi [1, n]
// (satır vektörü) olur. 2 boyutlu tensör olduğu gibi, 1 boyutlu tensör [1, n] görünümüyle
// kullanılır.
func matrisCevir(v yurut.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		switch x.t.Dim() {
		case 2:
			return x.t, nil
		case 1:
			return guvenli(func() *tensor.Tensor { return x.t.Unsqueeze(0) })
		}
		return nil, fmt.Errorf("matris 2 boyutlu olmalıdır, %s şeklinde tensör verildi", sekilMetni(x.t.Shape()))
	case *yurut.ListVal:
		if len(x.Elements) == 0 {
			break
		}
		satirlar := x.Elements
		if _, ok := x.Elements[0].(*yurut.ListVal); !ok {
			satirlar = []yurut.Value{x} // düz liste: tek satır
		}
		sutun := len(satirlar[0].(*yurut.ListVal).Elements)
		veri := make([]float32, 0, len(satirlar)*sutun)
		for _, s := range satirlar {
			satir, ok := s.(*yurut.ListVal)
			if !ok || len(satir.Elements) != sutun {
				return nil, fmt.Errorf("Matrisin tüm satırları eşit uzunlukta olmalıdır")
			}
			for _, e := range satir.Elements {
				f, ok := yurut.ToFloat(e)
				if !ok {
					return nil, fmt.Errorf("Matris elemanları sayısal olmalıdır")
				}
				veri = append(veri, float32(f))
			}
		}
		return tensor.New(veri, len(satirlar), sutun), nil
	}
	return nil, fmt.Errorf("matris oluşturmak için boş olmayan bir %s gereklidir", yurut.VAL_LIST)
}

// matrisBoyutDenetle negatif / sıfır ya da aşırı büyük matris boyutlarını panik yerine
// hata olarak bildirir.
func matrisBoyutDenetle(fn string, satir, sutun int64) error {
	if satir <= 0 || sutun <= 0 {
		return fmt.Errorf("%s: satır ve sütun sayısı pozitif olmalıdır (%d x %d verildi)", fn, satir, sutun)
	}
	if satir > yurut.MaxCollectionSize || sutun > yurut.MaxCollectionSize || satir*sutun > yurut.MaxCollectionSize {
		return fmt.Errorf("%s: matris çok büyük (%d x %d), en fazla %d eleman", fn, satir, sutun, yurut.MaxCollectionSize)
	}
	return nil
}

// tamArg i. argümanın Tam sayı olmasını bekler.
func tamArg(args []yurut.Value, i int, imza string) (int64, error) {
	n, ok := args[i].(*yurut.IntVal)
	if !ok {
		return 0, fmt.Errorf("%s %s sayı bekler, %s verildi", imza, yurut.VAL_INT, tipAdi(args[i]))
	}
	return n.Val, nil
}

// birimMatris birim_matris(n) yerleşiğidir: n×n birim matris.
func birimMatris(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T111)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(n) 1 parametre bekler", fn)
	}
	if err := baglama.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	n, err := tamArg(args, 0, fn+"(n)")
	if err != nil {
		return nil, err
	}
	if err := matrisBoyutDenetle(fn, n, n); err != nil {
		return nil, err
	}
	veri := make([]float32, n*n)
	for i := int64(0); i < n; i++ {
		veri[i*n+i] = 1
	}
	return Yeni(tensor.New(veri, int(n), int(n))), nil
}
