package tensor

import (
	"fmt"

	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Bu dosya tensör değerinin VM kancalarını (bkz. pkg/vm/kancalar.go) uygular: eşitlik,
// işleçler (@ + - * / ** ve karşılaştırmalar), tekli eksi, indeksleme, indeksle atama,
// döngüyle gezinme ve dış biçimlere (JSON) dönüşüm.

var (
	_ vm.Esitlik          = (*Deger)(nil)
	_ vm.IslecKancasi     = (*Deger)(nil)
	_ vm.Olumsuzlanabilir = (*Deger)(nil)
	_ vm.Indekslenebilir  = (*Deger)(nil)
	_ vm.IndeksAtanabilir = (*Deger)(nil)
	_ vm.Yinelenebilir    = (*Deger)(nil)
	_ vm.DuzDegerli       = (*Deger)(nil)
)

// DuzDeger tensörü JSON gibi dış biçimlere yazmak için .değer'le aynı iç içe Ondalık
// listesine (0 boyutluda Ondalık'a) çevirir.
func (d *Deger) DuzDeger() vm.Value { return d.degerListesi() }

// Esit iki tensörün şekli ve tüm elemanları aynıysa doğrudur ('==' ve '!='). 0 boyutlu
// tensör, değeri (.değer ile aynı biçimde okunan) sayıya eşittir: m[1][0] == 7.0. Diğer
// değerlerle eşit değildir.
func (d *Deger) Esit(diger vm.Value) bool {
	o, ok := diger.(*Deger)
	if !ok {
		if f, sayi := vm.ToFloat(diger); sayi && d.t.Dim() == 0 {
			return kisaFloat64(d.t.Item()) == f
		}
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

// Islec tensörün ikili işleçlerini uygular: '+', '-', '*', '/' (eleman bazlı, yayınlamalı;
// diğer işlenen sayı, liste ya da tensör) ve '@' (matris çarpımı) her iki yönde; '**' sol
// işlenenken (sayı üs); '<', '>', '<=', '>=' her iki yönde (0/1 maskesi).
func (d *Deger) Islec(op string, diger vm.Value, sagda bool) (vm.Value, bool, error) {
	switch op {
	case "+", "*":
		// Değişmeli: yön sonucu değiştirmez.
		v, err := d.ikili(op, diger)
		return v, true, err

	case "-", "/":
		if !sagda {
			v, err := d.ikili(op, diger)
			return v, true, err
		}
		if s, ok := vm.ToFloat(diger); ok {
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
		o, err := tensoreCevir("matris çarpımı (@)", diger)
		if err != nil {
			return nil, true, err
		}
		a, b := d.t, o
		if sagda {
			a, b = o, d.t
		}
		r, err := matCarp(a, b)
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "**":
		s, ok := vm.ToFloat(diger)
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
	if s, ok := vm.ToFloat(diger); ok {
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
		return d.gorunum(r), nil
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
	return d.gorunum(r), nil
}

// gorunum t'nin bir görünümünü (indeks, dilim, döngü öğesi) sarar. Görünüm aynı veriyi
// taşıdığından tam sayılı tensörün görünümü de tam sayılıdır (ör. token kimliği
// tensörünün satırları).
func (d *Deger) gorunum(r *ct.Tensor) *Deger {
	g := Yeni(r)
	g.tamSayili = d.tamSayili
	return g
}

// YinelemeUzunlugu 'döngü x içinde t' ve liste üretecinde gezilecek öğe sayısıdır (ilk
// eksenin uzunluğu). 0 boyutlu tensör üzerinde döngü kurulamaz.
func (d *Deger) YinelemeUzunlugu() (int, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return 0, fmt.Errorf("skaler (0 boyutlu) tensör üzerinde döngü kurulamaz; değerini .değer ile okuyun")
	}
	return sh[0], nil
}

// YinelemeOgesi ilk eksen boyunca i. öğedir: 1 boyutlu tensörde sayı (.değer'deki gibi
// Ondalık; tam sayılı tensörde Tam), N boyutluda t[i] ile aynı (N-1) boyutlu görünüm
// (veriyi paylaşır, türev izlenir).
func (d *Deger) YinelemeOgesi(i int) (vm.Value, error) {
	if d.t.Dim() != 1 {
		return d.Indeks(vm.NewInt(int64(i)))
	}
	x, err := guvenli(func() float32 { return d.t.At(i) })
	if err != nil {
		return nil, err
	}
	if d.tamSayili {
		return vm.NewInt(int64(x)), nil
	}
	return &vm.FloatVal{Val: kisaFloat64(x)}, nil
}

// IndekseAta t[i] = değer ve t[baş..son] = değer atamasıdır (t[i][j] = değer, t[i]
// görünümüne atamadır). Değer bir sayıysa hedefin tüm elemanlarına yazılır; liste ya da
// tensörse şekli hedefin şekliyle aynı olmalıdır. Veri yerinde değişir: bu tensörü kullanmış,
// henüz geri yayılmamış bir hesap çizgesi varsa geri yayılım hata verir. Türev izlenen bir
// tensöre yalnızca türevsiz { ... } içinde atanabilir.
func (d *Deger) IndekseAta(idx, deger vm.Value) error {
	if d.t.RequiresGrad() && ct.GradEnabled() {
		return fmt.Errorf("türev izlenen tensöre indeksle atama yapılamaz; değeri türevsiz { ... } içinde değiştirin")
	}
	var hedef *ct.Tensor
	var err error
	ct.NoGrad(func() {
		var v vm.Value
		if v, err = d.Indeks(idx); err == nil {
			hedef = v.(*Deger).t
		}
	})
	if err != nil {
		return err
	}
	sh := hedef.Shape()
	var veri []float32
	if f, ok := vm.ToFloat(deger); ok {
		veri = []float32{float32(f)}
	} else {
		kaynak, err := tensoreCevir("tensör ataması", deger)
		if err != nil {
			return err
		}
		if ks := kaynak.Shape(); sekilMetni(ks) != sekilMetni(sh) {
			return fmt.Errorf("tensör ataması: %s şeklindeki yere %s şeklinde veri atanamaz (şekiller aynı olmalı)", sekilMetni(sh), sekilMetni(ks))
		}
		veri = kaynak.Data()
	}
	_, err = guvenli(func() bool {
		konum := make([]int, len(sh))
		for k := 0; k < hedef.Numel(); k++ {
			hedef.Set(veri[min(k, len(veri)-1)], konum...)
			// Satır öncelikli sıradaki bir sonraki konum.
			for e := len(sh) - 1; e >= 0; e-- {
				if konum[e]++; konum[e] < sh[e] {
					break
				}
				konum[e] = 0
			}
		}
		return true
	})
	return err
}
