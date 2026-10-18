package tensor

import (
	"fmt"
	"math/rand"

	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/sozluk"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Bu dosya matris adlı yerleşikleri (matris, birim_matris, rastgele_matris, sıfırlar(satır,
// sütun), birler(satır, sütun), devrik) uygular. Dilde ayrı bir matris türü yoktur: matris
// 2 boyutlu bir tensördür; @, + - * /, relu, softmax ... tensör yolundan geçer.

// matrisCevir matris alan işlevlerin (matris, devrik, sinir_ağı, kayıp_mse ...) girdisini 2
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

// matrisSekli sıfırlar(satır, sütun) / birler(satır, sütun) biçimini 2 boyutlu şekle
// çevirir. Tek tam sayı (yalnızca sıfırlar(n)) tek satırlı [1, n] matristir.
func matrisSekli(kod sozluk.Kod, args []vm.Value) ([]int, error) {
	ad := sozluk.Ad(kod)
	enAz := 2
	if kod == sozluk.T+101 { // sıfırlar
		enAz = 1
	}
	imza := ad + "(satır, sütun)"
	if len(args) < enAz || len(args) > 2 {
		return nil, fmt.Errorf("%s %d parametre bekler, %d verildi; çok boyutlu tensör için %s([2, 3, 4]) yazın", imza, 2, len(args), ad)
	}
	boy := []int64{1, 0}
	for i := range args {
		n, err := tamArg(args, i, imza)
		if err != nil {
			return nil, err
		}
		boy[2-len(args)+i] = n
	}
	if err := matrisBoyutDenetle(ad, boy[0], boy[1]); err != nil {
		return nil, err
	}
	return []int{int(boy[0]), int(boy[1])}, nil
}

// matrisYerlesik matris(liste) yerleşiğidir: iç içe listeden 2 boyutlu tensör (düz
// listeden tek satırlı matris) oluşturur.
func matrisYerlesik(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("matris(liste) 1 parametre bekler")
	}
	if err := baglama.AdliDenetle("matris()", named); err != nil {
		return nil, err
	}
	t, err := matrisCevir(args[0])
	if err != nil {
		return nil, err
	}
	return Yeni(t), nil
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

// rastgeleMatris rastgele_matris(satır, sütun, [tohum], tohum: 42) yerleşiğidir: elemanları
// ortalaması 0, standart sapması 0.5 olan normal dağılımdan, tohumdan satır öncelikli
// sırayla üretilir (aynı tohum aynı matrisi verir).
func rastgeleMatris(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const imza = "rastgele_matris(satır, sütun)"
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("rastgele_matris(satır, sütun, [tohum]) 2 ya da 3 parametre bekler, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("rastgele_matris()", named, adTohum); err != nil {
		return nil, err
	}
	r, err := tamArg(args, 0, imza)
	if err != nil {
		return nil, err
	}
	c, err := tamArg(args, 1, imza)
	if err != nil {
		return nil, err
	}
	if err := matrisBoyutDenetle("rastgele_matris", r, c); err != nil {
		return nil, err
	}
	tohum := int64(42)
	if len(args) == 3 {
		if tohum, err = tamArg(args, 2, "rastgele_matris(satır, sütun, tohum)"); err != nil {
			return nil, err
		}
	}
	if v, ok := baglama.AdliAl(named, adTohum); ok {
		n, ok := v.(*vm.IntVal)
		if !ok {
			return nil, fmt.Errorf("rastgele_matris: tohum tam sayı olmalıdır, %s verildi", tipAdi(v))
		}
		tohum = n.Val
	}
	rng := rand.New(rand.NewSource(tohum))
	veri := make([]float32, r*c)
	for i := range veri {
		veri[i] = float32(rng.NormFloat64() * 0.5)
	}
	return Yeni(ct.New(veri, int(r), int(c))), nil
}

// devrikYerlesik devrik(x) yerleşiğidir: tensörün son iki eksenini değiştirir (veriyi
// paylaşan görünüm); liste matrise çevrilip devrilir.
func devrikYerlesik(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("devrik(matris) 1 parametre bekler")
	}
	if err := baglama.AdliDenetle("devrik()", named); err != nil {
		return nil, err
	}
	if d, ok := args[0].(*Deger); ok {
		return devrik(d.t)
	}
	t, err := matrisCevir(args[0])
	if err != nil {
		return nil, err
	}
	return devrik(t)
}
