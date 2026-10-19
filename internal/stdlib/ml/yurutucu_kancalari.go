package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Bu dosya tensör değerinin VM kancalarını (bkz. internal/lang/interp/kancalar.go) uygular: eşitlik,
// işleçler (@ + - * / ** ve karşılaştırmalar), tekli eksi, indeksleme, indeksle atama,
// döngüyle gezinme ve dış biçimlere (JSON) dönüşüm.

var (
	_ interp.Esitlik             = (*Deger)(nil)
	_ interp.IslecKancasi        = (*Deger)(nil)
	_ interp.GorevliIslecKancasi = (*Deger)(nil)
	_ interp.Olumsuzlanabilir    = (*Deger)(nil)
	_ interp.Indekslenebilir     = (*Deger)(nil)
	_ interp.IndeksAtanabilir    = (*Deger)(nil)
	_ interp.Yinelenebilir       = (*Deger)(nil)
	_ interp.DuzDegerli          = (*Deger)(nil)
)

// DuzDeger tensörü JSON gibi dış biçimlere yazmak için .değer'le aynı iç içe Ondalık
// listesine (0 boyutluda Ondalık'a) çevirir.
func (d *Deger) DuzDeger() interp.Value { return d.degerListesi() }

// Esit iki tensörün şekli ve tüm elemanları aynıysa doğrudur ('==' ve '!='). 0 boyutlu
// tensör, değeri (.değer ile aynı biçimde okunan) sayıya eşittir: m[1][0] == 7.0. Diğer
// değerlerle eşit değildir.
func (d *Deger) Esit(diger interp.Value) bool {
	o, ok := diger.(*Deger)
	if !ok {
		if f, sayi := interp.ToFloat(diger); sayi && d.t.Dim() == 0 {
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
func (d *Deger) Olumsuz() (interp.Value, error) {
	r, err := guvenli(func() *tensor.Tensor { return d.t.Neg() })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// desteklenenIslecler hata mesajlarında gösterilir.
const desteklenenIslecler = "+ - * / @ ** < > <= >= == !="

// GorevliIslec işleci yapan görevle birlikte çağrılır: '@' görevin karışık hassasiyet
// türüne göre (bkz. karışık_hassasiyet) hesaplanır; diğer işleçler Islec'e gider.
func (d *Deger) GorevliIslec(m *interp.VM, op string, diger interp.Value, sagda bool) (interp.Value, bool, error) {
	if op != "@" {
		return d.Islec(op, diger, sagda)
	}
	o, err := tensoreCevirCihaz("matris çarpımı (@)", diger, d.t)
	if err != nil {
		return nil, true, err
	}
	a, b := d.t, o
	if sagda {
		a, b = o, d.t
	}
	r, err := matCarpTur(a, b, karisikTuru(m))
	if err != nil {
		return nil, true, err
	}
	return Yeni(r), true, nil
}

// Islec tensörün ikili işleçlerini uygular: '+', '-', '*', '/' (eleman bazlı, yayınlamalı;
// diğer işlenen sayı, liste ya da tensör) ve '@' (matris çarpımı) her iki yönde; '**' sol
// işlenenken (sayı üs); '<', '>', '<=', '>=' her iki yönde (0/1 maskesi).
func (d *Deger) Islec(op string, diger interp.Value, sagda bool) (interp.Value, bool, error) {
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
		if s, ok := interp.ToFloat(diger); ok {
			f := float32(s)
			r, err := guvenli(func() *tensor.Tensor {
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
		o, err := tensoreCevirCihaz("tensör "+islemAdi[op], diger, d.t)
		if err != nil {
			return nil, true, err
		}
		if err := yayinDenetle(op, o, d.t); err != nil {
			return nil, true, err
		}
		if err := cihazDenetle("tensör "+islemAdi[op], o, d.t); err != nil {
			return nil, true, err
		}
		r, err := guvenli(func() *tensor.Tensor {
			if op == "-" {
				return tensor.Sub(o, d.t)
			}
			return tensor.Div(o, d.t)
		})
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "@":
		o, err := tensoreCevirCihaz("matris çarpımı (@)", diger, d.t)
		if err != nil {
			return nil, true, err
		}
		a, b := d.t, o
		if sagda {
			a, b = o, d.t
		}
		r, err := matCarpTur(a, b, tensor.Float32)
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "**":
		s, ok := interp.ToFloat(diger)
		if sagda || !ok {
			return nil, true, fmt.Errorf("tensör '**' işleminde üs bir sayı olmalıdır (t ** 2); eleman bazlı üs için %s(%s(t) * u) kullanın", codes.Ad(codes.T220), codes.Ad(codes.T221))
		}
		f := float32(s)
		r, err := guvenli(func() *tensor.Tensor { return d.t.Pow(f) })
		if err != nil {
			return nil, true, err
		}
		return Yeni(r), true, nil

	case "<", ">", "<=", ">=":
		v, err := d.karsilastir(op, diger, sagda)
		return v, true, err
	}
	return nil, true, fmt.Errorf("%s ile %q işlemi desteklenmiyor (desteklenen işleçler: %s)", interp.VAL_TENSOR, op, desteklenenIslecler)
}

// karsilastirmaIslemi karşılaştırma işleçlerinin çekirdek işlemi.
var karsilastirmaIslemi = map[string]tensor.BinOp{"<": tensor.OpLt, ">": tensor.OpGt, "<=": tensor.OpLe, ">=": tensor.OpGe}

// karsilastir eleman bazlı karşılaştırmanın 0/1 maskesini (türev izlemeyen float32 tensör)
// tensörün cihazında verir. Diğer işlenen sayı, liste ya da tensör olabilir; şekiller
// yayınlanır (+ - * / gibi). NaN içeren karşılaştırma 0'dır.
func (d *Deger) karsilastir(op string, diger interp.Value, sagda bool) (interp.Value, error) {
	bop := karsilastirmaIslemi[op]
	if s, ok := interp.ToFloat(diger); ok {
		r, err := guvenli(func() *tensor.Tensor { return d.t.CompareScalar(float32(s), bop, sagda) })
		if err != nil {
			return nil, err
		}
		return Yeni(r), nil
	}
	ad := "tensör karşılaştırması (" + op + ")"
	o, err := tensoreCevirCihaz(ad, diger, d.t)
	if err != nil {
		return nil, err
	}
	a, b := d.t, o
	if sagda {
		a, b = o, d.t
	}
	if err := yayinDenetle(ad, a, b); err != nil {
		return nil, err
	}
	if err := cihazDenetle(ad, a, b); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.Compare(a, b, bop) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// Indeks t[i] (ilk eksende i. eleman; bir eksen eksilir) ve t[baş..son] (ilk eksende
// dilim) okumasını yapar. Sonuç veriyi kopyalamayan bir görünümdür ve türev izlenir.
func (d *Deger) Indeks(idx interp.Value) (interp.Value, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("skaler (0 boyutlu) tensör indekslenemez")
	}
	n := sh[0]
	if rng, ok := idx.(*interp.RangeVal); ok {
		bas, son := int(rng.Start), int(rng.End)
		if rng.Inclusive {
			son++
		}
		r, err := guvenli(func() *tensor.Tensor { return d.t.Slice(0, bas, son) })
		if err != nil {
			return nil, err
		}
		return d.gorunum(r), nil
	}
	i, ok := tamSayi(idx)
	if !ok {
		return nil, fmt.Errorf("tensör indeksi %s sayı ya da aralık olmalıdır, %s verildi", interp.VAL_INT, idx.Type())
	}
	j := i
	if j < 0 {
		j += n
	}
	if j < 0 || j >= n {
		return nil, fmt.Errorf("tensör indeksi sınır dışı: %d (ilk eksenin uzunluğu %d, şekil %s)", i, n, sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Slice(0, j, j+1).Squeeze(0) })
	if err != nil {
		return nil, err
	}
	return d.gorunum(r), nil
}

// gorunum t'nin bir görünümünü (indeks, dilim, döngü öğesi) sarar. Görünüm aynı veriyi
// taşıdığından tam sayılı tensörün görünümü de tam sayılıdır (ör. token kimliği
// tensörünün satırları).
func (d *Deger) gorunum(r *tensor.Tensor) *Deger {
	g := Yeni(r)
	g.tamSayili = d.tamSayili
	return g
}

// YinelemeUzunlugu 'döngü x içinde t' ve liste üretecinde gezilecek öğe sayısıdır (ilk
// eksenin uzunluğu). 0 boyutlu tensör üzerinde döngü kurulamaz.
func (d *Deger) YinelemeUzunlugu() (int, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return 0, fmt.Errorf("skaler (0 boyutlu) tensör üzerinde döngü kurulamaz; değerini .%s ile okuyun", codes.Ad(uyeDeger))
	}
	return sh[0], nil
}

// YinelemeOgesi ilk eksen boyunca i. öğedir: 1 boyutlu tensörde sayı (.değer'deki gibi
// Ondalık; tam sayılı tensörde Tam), N boyutluda t[i] ile aynı (N-1) boyutlu görünüm
// (veriyi paylaşır, türev izlenir).
func (d *Deger) YinelemeOgesi(i int) (interp.Value, error) {
	if d.t.Dim() != 1 {
		return d.Indeks(interp.NewInt(int64(i)))
	}
	x, err := guvenli(func() float32 { return d.t.At(i) })
	if err != nil {
		return nil, err
	}
	if d.tamSayili {
		return interp.NewInt(int64(x)), nil
	}
	return &interp.FloatVal{Val: kisaFloat64(x)}, nil
}

// IndekseAta t[i] = değer ve t[baş..son] = değer atamasıdır (t[i][j] = değer, t[i]
// görünümüne atamadır). Değer bir sayıysa hedefin tüm elemanlarına yazılır; liste ya da
// tensörse şekli hedefin şekliyle aynı olmalıdır. Veri yerinde değişir: bu tensörü kullanmış,
// henüz geri yayılmamış bir hesap çizgesi varsa geri yayılım hata verir. Türev izlenen bir
// tensöre yalnızca türevsiz { ... } içinde atanabilir.
func (d *Deger) IndekseAta(idx, deger interp.Value) error {
	if d.t.RequiresGrad() && tensor.GradEnabled() {
		return fmt.Errorf("türev izlenen tensöre indeksle atama yapılamaz; değeri türevsiz { ... } içinde değiştirin")
	}
	var hedef *tensor.Tensor
	var err error
	tensor.NoGrad(func() {
		var v interp.Value
		if v, err = d.Indeks(idx); err == nil {
			hedef = v.(*Deger).t
		}
	})
	if err != nil {
		return err
	}
	sh := hedef.Shape()
	var kaynak *tensor.Tensor
	if f, ok := interp.ToFloat(deger); ok {
		kaynak = tensor.Scalar(float32(f)) // hedefin tüm elemanlarına yayınlanır
	} else {
		if kaynak, err = tensoreCevir("tensör ataması", deger); err != nil {
			return err
		}
		if ks := kaynak.Shape(); sekilMetni(ks) != sekilMetni(sh) {
			return fmt.Errorf("tensör ataması: %s şeklindeki yere %s şeklinde veri atanamaz (şekiller aynı olmalı)", sekilMetni(sh), sekilMetni(ks))
		}
	}
	// Tek arka uç kopyası, tek sürüm artışı; hedef bitişik olmayan bir görünüm olabilir.
	_, err = guvenli(func() bool {
		if kaynak.Device() != hedef.Device() {
			tensor.NoGrad(func() { kaynak = kaynak.To(hedef.Device()) })
		}
		hedef.CopyFrom(kaynak)
		return true
	})
	return err
}
