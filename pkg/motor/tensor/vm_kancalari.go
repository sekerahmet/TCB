package tensor

import (
	"fmt"

	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Bu dosya tensör değerinin VM kancalarını (bkz. pkg/vm/kancalar.go) uygular: eşitlik,
// sağ işlenen olarak işleçler, '**', karşılaştırmalar, tekli eksi ve indeksleme.

var (
	_ vm.Esitlik          = (*Deger)(nil)
	_ vm.IslecKancasi     = (*Deger)(nil)
	_ vm.Olumsuzlanabilir = (*Deger)(nil)
	_ vm.Indekslenebilir  = (*Deger)(nil)
)

// Esit iki tensörün şekli ve tüm elemanları aynıysa doğrudur ('==' ve '!='). Tensör
// olmayan değerlerle eşit değildir.
func (d *Deger) Esit(diger vm.Value) bool {
	o, ok := diger.(*Deger)
	if !ok {
		return false
	}
	if d == o || d.t == o.t {
		return true
	}
	a, b := d.t.Shape(), o.t.Shape()
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	ad, bd := d.t.Data(), o.t.Data()
	for i := range ad {
		if ad[i] != bd[i] {
			return false
		}
	}
	return true
}

// Olumsuz tekli eksidir (-t); türev izlenir.
func (d *Deger) Olumsuz() (vm.Value, error) {
	r, err := guvenli(func() *ct.Tensor { return d.t.Neg() })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// desteklenenIslecler hata mesajlarında gösterilir.
const desteklenenIslecler = "+ - * / @ ** < > <= >= == !="

// Islec MatrixOperator'ın karşılamadığı işleçleri uygular: sağ işlenenken '-', '/', '@';
// sol işlenenken '**' (sayı üs); her iki yönde '<', '>', '<=', '>=' (0/1 maskesi).
func (d *Deger) Islec(op string, diger vm.Value, sagda bool) (vm.Value, bool, error) {
	switch op {
	case "-", "/":
		if !sagda {
			return nil, false, nil // sol işlenende MatrixOperator (Sub/Div) çalışır
		}
		if s, ok := skalerMi(diger); ok {
			f := float32(s)
			r, err := guvenli(func() *ct.Tensor {
				if op == "-" {
					return d.t.RSubScalar(f)
				}
				return d.t.RDivScalar(f)
			})
			if err != nil {
				return nil, true, err
			}
			return Yeni(r), true, nil
		}
		o, err := tensoreCevir("tensör "+islemAdi[op], diger)
		if err != nil {
			return nil, true, err
		}
		if err := yayinDenetle(op, o, d.t); err != nil {
			return nil, true, err
		}
		if err := cihazDenetle("tensör "+islemAdi[op], o, d.t); err != nil {
			return nil, true, err
		}
		r, err := guvenli(func() *ct.Tensor {
			if op == "-" {
				return ct.Sub(o, d.t)
			}
			return ct.Div(o, d.t)
		})
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "@":
		if !sagda {
			return nil, false, nil
		}
		o, err := tensoreCevir("tensör matris çarpımı", diger)
		if err != nil {
			return nil, true, err
		}
		v, err := Yeni(o).MatMul(d)
		return v, true, err

	case "**":
		s, ok := skalerMi(diger)
		if sagda || !ok {
			return nil, true, fmt.Errorf("tensör '**' işleminde üs bir sayı olmalıdır (t ** 2); eleman bazlı üs için üstel(log(t) * u) kullanın")
		}
		f := float32(s)
		r, err := guvenli(func() *ct.Tensor { return d.t.Pow(f) })
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "<", ">", "<=", ">=":
		v, err := d.karsilastir(op, diger, sagda)
		return v, true, err
	}
	return nil, true, fmt.Errorf("Tensör ile %q işlemi desteklenmiyor (desteklenen işleçler: %s)", op, desteklenenIslecler)
}

// karsilastir eleman bazlı karşılaştırmanın 0/1 maskesini (türev izlemeyen, aynı şekilli
// tensör) verir. Diğer işlenen sayı ya da aynı şekilli tensör/liste olmalıdır.
func (d *Deger) karsilastir(op string, diger vm.Value, sagda bool) (vm.Value, error) {
	a := d.t.Data()
	var b []float32
	if s, ok := skalerMi(diger); ok {
		b = []float32{float32(s)}
	} else {
		o, err := tensoreCevir("tensör karşılaştırması", diger)
		if err != nil {
			return nil, err
		}
		sa, sb := d.t.Shape(), o.Shape()
		ayni := len(sa) == len(sb)
		for i := 0; ayni && i < len(sa); i++ {
			ayni = sa[i] == sb[i]
		}
		if !ayni {
			return nil, fmt.Errorf("tensör karşılaştırması (%s) için şekiller aynı olmalıdır: %s ile %s", op, sekilMetni(sa), sekilMetni(sb))
		}
		b = o.Data()
	}
	sonuc := make([]float32, len(a))
	for i, x := range a {
		y := b[0]
		if len(b) > 1 {
			y = b[i]
		}
		l, r := x, y
		if sagda {
			l, r = y, x
		}
		var dogru bool
		switch op {
		case "<":
			dogru = l < r
		case ">":
			dogru = l > r
		case "<=":
			dogru = l <= r
		default:
			dogru = l >= r
		}
		if dogru {
			sonuc[i] = 1
		}
	}
	r, err := guvenli(func() *ct.Tensor { return ct.New(sonuc, d.t.Shape()...).To(d.t.Device()) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// Indeks t[i] (ilk eksende i. eleman; bir eksen eksilir) ve t[baş..son] (ilk eksende
// dilim) okumasını yapar. Sonuç veriyi kopyalamayan bir görünümdür ve türev izlenir.
func (d *Deger) Indeks(idx vm.Value) (vm.Value, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("skaler (0 boyutlu) tensör indekslenemez")
	}
	n := sh[0]
	if rng, ok := idx.(*vm.RangeVal); ok {
		bas, son := int(rng.Start), int(rng.End)
		if rng.Inclusive {
			son++
		}
		r, err := guvenli(func() *ct.Tensor { return d.t.Slice(0, bas, son) })
		if err != nil {
			return nil, err
		}
		return Yeni(r), nil
	}
	i, ok := tamSayi(idx)
	if !ok {
		return nil, fmt.Errorf("tensör indeksi Tam sayı ya da aralık olmalıdır, %s verildi", idx.Type())
	}
	j := i
	if j < 0 {
		j += n
	}
	if j < 0 || j >= n {
		return nil, fmt.Errorf("tensör indeksi sınır dışı: %d (ilk eksenin uzunluğu %d, şekil %s)", i, n, sekilMetni(sh))
	}
	r, err := guvenli(func() *ct.Tensor { return d.t.Slice(0, j, j+1).Squeeze(0) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}
