package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya matris biçimli yerleşikleri (birim_matris, sıfırlar(satır, sütun), birler(satır,
// sütun), devrik) uygular. Dilde ayrı bir matris türü yoktur: matris 2 boyutlu bir
// tensördür ve tensör([[1, 2], [3, 4]]) ile oluşturulur (rastgele olanı rastgele_normal([2,
// 3]) ile); @, + - * /, relu, softmax ... tensör yolundan geçer.

// matrisCevir matris alan işlevlerin girdisini 2
// boyutlu tensöre çevirir: iç içe sayı listesi [satır, sütun], düz sayı listesi [1, n]
// (satır vektörü) olur. 2 boyutlu tensör olduğu gibi, 1 boyutlu tensör [1, n] görünümüyle
// kullanılır.
func matrisCevir(v interp.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		switch x.t.Dim() {
		case 2:
			return x.t, nil
		case 1:
			return guvenli(func() *tensor.Tensor { return x.t.Unsqueeze(0) })
		}
		return nil, fmt.Errorf("matris 2 boyutlu olmalıdır, %s şeklinde tensör verildi", sekilMetni(x.t.Shape()))
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			break
		}
		satirlar := x.Elements
		if _, ok := x.Elements[0].(*interp.ListVal); !ok {
			satirlar = []interp.Value{x} // düz liste: tek satır
		}
		sutun := len(satirlar[0].(*interp.ListVal).Elements)
		veri := make([]float32, 0, len(satirlar)*sutun)
		for _, s := range satirlar {
			satir, ok := s.(*interp.ListVal)
			if !ok || len(satir.Elements) != sutun {
				return nil, fmt.Errorf("Matrisin tüm satırları eşit uzunlukta olmalıdır")
			}
			for _, e := range satir.Elements {
				f, ok := interp.ToFloat(e)
				if !ok {
					return nil, fmt.Errorf("Matris elemanları sayısal olmalıdır")
				}
				veri = append(veri, float32(f))
			}
		}
		return tensor.New(veri, len(satirlar), sutun), nil
	}
	return nil, fmt.Errorf("matris oluşturmak için boş olmayan bir %s gereklidir", interp.VAL_LIST)
}

// matrisBoyutDenetle negatif / sıfır ya da aşırı büyük matris boyutlarını panik yerine
// hata olarak bildirir.
func matrisBoyutDenetle(fn string, satir, sutun int64) error {
	if satir <= 0 || sutun <= 0 {
		return fmt.Errorf("%s: satır ve sütun sayısı pozitif olmalıdır (%d x %d verildi)", fn, satir, sutun)
	}
	if satir > interp.MaxCollectionSize || sutun > interp.MaxCollectionSize || satir*sutun > interp.MaxCollectionSize {
		return fmt.Errorf("%s: matris çok büyük (%d x %d), en fazla %d eleman", fn, satir, sutun, interp.MaxCollectionSize)
	}
	return nil
}

// tamArg i. argümanın Tam sayı olmasını bekler.
func tamArg(args []interp.Value, i int, imza string) (int64, error) {
	n, ok := args[i].(*interp.IntVal)
	if !ok {
		return 0, fmt.Errorf("%s %s sayı bekler, %s verildi", imza, interp.VAL_INT, tipAdi(args[i]))
	}
	return n.Val, nil
}

// birimMatris birim_matris(n) yerleşiğidir: n×n birim matris.
func birimMatris(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T111)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(n) 1 parametre bekler", fn)
	}
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
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
