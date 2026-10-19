package interp

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// siralaDegerler değerleri sırala() kuralıyla kararlı (eşitlerin sırası korunarak) sıralar.
// Liste ya yalnızca sayılardan (Tam ve Ondalık) ya da yalnızca metinlerden oluşmalıdır;
// başka bir tip ya da sayı ile metin karışımı karşılaştırılamaz ve hata verir. Sayılar
// tam olarak karşılaştırılır (iki Tam int64 olarak; Tam ile Ondalık Ondalığa yuvarlanmadan),
// NaN'lar sona konur. Metinler karakter kodu sırasıyla dizilir. ad hata mesajındaki adıdır.
func siralaDegerler(ad string, d []Value) error {
	if len(d) < 2 {
		if len(d) == 1 && !sayiMi(d[0]) {
			if _, metin := d[0].(*StringVal); !metin {
				return siralamaTipHatasi(ad, d[0], nil)
			}
		}
		return nil
	}
	switch d[0].(type) {
	case *IntVal, *FloatVal:
		// Hızlı yol: hepsi Tam (en sık durum: anahtarlar bir kez çıkarılır)
		tamlar := true
		for _, x := range d {
			switch x.(type) {
			case *IntVal:
			case *FloatVal:
				tamlar = false
			default:
				return siralamaTipHatasi(ad, d[0], x)
			}
		}
		if tamlar {
			type anahtarli struct {
				k int64
				v Value
			}
			cift := make([]anahtarli, len(d))
			for i, x := range d {
				cift[i] = anahtarli{x.(*IntVal).Val, x}
			}
			// Eşit Tam'lar ayırt edilemez; kararsız sıralama aynı sonucu verir.
			slices.SortFunc(cift, func(a, b anahtarli) int { return cmp.Compare(a.k, b.k) })
			for i := range cift {
				d[i] = cift[i].v
			}
			return nil
		}
		slices.SortStableFunc(d, func(a, b Value) int {
			c, ok := sayiKarsilastir(a, b)
			if ok {
				return c
			}
			// NaN'lar sona (kendi aralarında eşit)
			an, bn := nanMi(a), nanMi(b)
			switch {
			case an && bn:
				return 0
			case an:
				return 1
			}
			return -1
		})
		return nil
	case *StringVal:
		for _, x := range d {
			if _, ok := x.(*StringVal); !ok {
				return siralamaTipHatasi(ad, d[0], x)
			}
		}
		slices.SortStableFunc(d, func(a, b Value) int { return cmp.Compare(a.(*StringVal).Val, b.(*StringVal).Val) })
		return nil
	}
	return siralamaTipHatasi(ad, d[0], nil)
}

func nanMi(v Value) bool {
	f, ok := v.(*FloatVal)
	return ok && math.IsNaN(f.Val)
}

// siralamaTipHatasi sıralanamayan bir listenin hatasıdır: ilk elemanın tipi sıralanamıyorsa
// (b nil) ya da iki elemanın tipleri birbiriyle karşılaştırılamıyorsa.
func siralamaTipHatasi(ad string, a, b Value) error {
	if b == nil {
		return fmt.Errorf("%s: %s sıralanamaz; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır", ad, TipVeOzet(a))
	}
	return fmt.Errorf("%s: %s ile %s karşılaştırılamaz; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır", ad, TipVeOzet(a), TipVeOzet(b))
}
