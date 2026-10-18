package tensor

import (
	"fmt"

	"tr_dili/internal/dil/vm"
	ct "tr_dili/internal/hesap/tensor"
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
func matrisCevir(v vm.Value) (*ct.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		switch x.t.Dim() {
		case 2:
			return x.t, nil
		case 1:
			return guvenli(func() *ct.Tensor { return x.t.Unsqueeze(0) })
		}
		return nil, fmt.Errorf("matris 2 boyutlu olmalıdır, %s şeklinde tensör verildi", sekilMetni(x.t.Shape()))
	case *vm.ListVal:
		if len(x.Elements) == 0 {
			break
		}
		satirlar := x.Elements
		if _, ok := x.Elements[0].(*vm.ListVal); !ok {
			satirlar = []vm.Value{x} // düz liste: tek satır
		}
		sutun := len(satirlar[0].(*vm.ListVal).Elements)
		veri := make([]float32, 0, len(satirlar)*sutun)
		for _, s := range satirlar {
			satir, ok := s.(*vm.ListVal)
			if !ok || len(satir.Elements) != sutun {
				return nil, fmt.Errorf("Matrisin tüm satırları eşit uzunlukta olmalıdır")
			}
			for _, e := range satir.Elements {
				f, ok := vm.ToFloat(e)
				if !ok {
					return nil, fmt.Errorf("Matris elemanları sayısal olmalıdır")
				}
				veri = append(veri, float32(f))
			}
		}
		return ct.New(veri, len(satirlar), sutun), nil
	}
	return nil, fmt.Errorf("Matris oluşturmak için boş olmayan bir Liste gereklidir")
}

// matrisBoyutDenetle negatif / sıfır ya da aşırı büyük matris boyutlarını panik yerine
// hata olarak bildirir.
func matrisBoyutDenetle(fn string, satir, sutun int64) error {
	if satir <= 0 || sutun <= 0 {
		return fmt.Errorf("%s: satır ve sütun sayısı pozitif olmalıdır (%d x %d verildi)", fn, satir, sutun)
	}
	if satir > vm.MaxCollectionSize || sutun > vm.MaxCollectionSize || satir*sutun > vm.MaxCollectionSize {
		return fmt.Errorf("%s: matris çok büyük (%d x %d), en fazla %d eleman", fn, satir, sutun, vm.MaxCollectionSize)
	}
	return nil
}

// tamArg i. argümanın Tam sayı olmasını bekler.
func tamArg(args []vm.Value, i int, imza string) (int64, error) {
	n, ok := args[i].(*vm.IntVal)
	if !ok {
		return 0, fmt.Errorf("%s Tam sayı bekler, %s verildi", imza, tipAdi(args[i]))
	}
	return n.Val, nil
}

// birimMatris birim_matris(n) yerleşiğidir: n×n birim matris.
func birimMatris(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("birim_matris(n) 1 parametre bekler")
	}
	if err := baglama.AdliDenetle("birim_matris()", named); err != nil {
		return nil, err
	}
	n, err := tamArg(args, 0, "birim_matris(n)")
	if err != nil {
		return nil, err
	}
	if err := matrisBoyutDenetle("birim_matris", n, n); err != nil {
		return nil, err
	}
	veri := make([]float32, n*n)
	for i := int64(0); i < n; i++ {
		veri[i*n+i] = 1
	}
	return Yeni(ct.New(veri, int(n), int(n))), nil
}
