package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// Sinir ağı fonksiyonları
// ----------------------------------------------------------------------------

// tamIndeksler indeks verisini (tam sayı, iç içe liste ya da tensör) düz tam sayı
// dizisine ve şekline çevirir.
func tamIndeksler(fn string, v interp.Value) ([]int, []int, error) {
	if n, ok := tamSayi(v); ok {
		return []int{n}, nil, nil
	}
	var veri []float32
	var sh []int
	switch x := v.(type) {
	case *Deger:
		veri, sh = x.t.Data(), x.t.Shape()
	case *interp.ListVal:
		// Liste değerleri tam sayı olarak okunur (float32'den geçmez).
		return listeTamSayilari(fn, x)
	default:
		return nil, nil, fmt.Errorf("%s: indeksler tam sayı, tam sayı listesi ya da tensör olmalıdır, %s verildi", fn, tipAdi(v))
	}
	out := make([]int, len(veri))
	for i, f := range veri {
		if float32(int(f)) != f {
			return nil, nil, fmt.Errorf("%s: indeksler tam sayı olmalıdır, %v verildi", fn, f)
		}
		out[i] = int(f)
	}
	return out, sh, nil
}

func gomme(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T500)
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(tablo, indeksler) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	tablo, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("%s: tablo bir tensör olmalıdır, %s verildi", fn, tipAdi(args[0]))
	}
	tsh := tablo.t.Shape()
	if len(tsh) != 2 {
		return nil, fmt.Errorf("%s: tablo 2 boyutlu (satır sayısı × boyut) olmalıdır, %s verildi", fn, sekilMetni(tsh))
	}
	idx, ish, err := tamIndeksler(fn, args[1])
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if i < 0 || i >= tsh[0] {
			return nil, fmt.Errorf("%s: indeks %d, %s şeklindeki tablonun dışında; 0 ile %d arasında olmalı", fn, i, sekilMetni(tsh), tsh[0]-1)
		}
	}
	r, err := guvenli(func() *tensor.Tensor {
		if len(ish) == 0 {
			// Tek indeks: sonuç tek bir satırdır, şekli [boyut].
			return tensor.Embedding(tablo.t, idx).Reshape(tsh[1])
		}
		return tensor.Embedding(tablo.t, idx, ish...)
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func caprazEntropi(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T510)
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(çıktı, hedef) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	cikti, err := tensoreCevir(fn, args[0])
	if err != nil {
		return nil, err
	}
	if cikti, err = yuksekHassasiyet(m, cikti); err != nil {
		return nil, err
	}
	sh := cikti.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("%s: çıktı en az 1 boyutlu olmalıdır (son eksen sınıflar)", fn)
	}
	hedef, _, err := tamIndeksler(fn, args[1])
	if err != nil {
		return nil, err
	}
	C := sh[len(sh)-1]
	satir := 0
	if C > 0 {
		satir = cikti.Numel() / C
	}
	if len(hedef) != satir && C > 1 && len(hedef) == cikti.Numel() && sifirBirMi(hedef) {
		return nil, fmt.Errorf("%s: hedef sınıf indeksleri olmalı (ör. [2, 0, 1]); one-hot verdiniz gibi görünüyor (%s şeklindeki çıktıyla aynı sayıda 0/1); her satır için 1'in konumunu verin", fn, sekilMetni(sh))
	}
	if len(hedef) != satir {
		return nil, fmt.Errorf("%s: %s şeklindeki çıktı için %d hedef bekleniyor, %d verildi", fn, sekilMetni(sh), satir, len(hedef))
	}
	gecerli := 0
	for _, h := range hedef {
		if h >= C {
			return nil, fmt.Errorf("%s: hedef sınıf %d, %d sınıflı çıktının dışında; 0 ile %d arasında olmalı (negatif hedefler yok sayılır)", fn, h, C, C-1)
		}
		if h >= 0 {
			gecerli++
		}
	}
	if gecerli == 0 {
		return nil, fmt.Errorf("%s: bütün hedefler yok sayıldı (%d satırın hepsi negatif ya da hiç satır yok); ortalama kayıp tanımsız. Yalnızca dolgudan oluşan partiyi atlayın", fn, len(hedef))
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.CrossEntropy(cikti, hedef) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func sifirBirMi(x []int) bool {
	for _, v := range x {
		if v != 0 && v != 1 {
			return false
		}
	}
	return true
}

// normParam ağırlık/sapma argümanını okur (yok verilebilir) ve şeklini denetler.
func normParam(fn, ad string, v interp.Value, D int) (*tensor.Tensor, error) {
	if v == nil || v == interp.NULL {
		return nil, nil
	}
	t, err := tensoreCevir(fn, v)
	if err != nil {
		return nil, err
	}
	if sh := t.Shape(); len(sh) != 1 || sh[0] != D {
		return nil, fmt.Errorf("%s: %s [%d] şeklinde olmalı (x'in son ekseni %d), %s verildi", fn, ad, D, D, sekilMetni(sh))
	}
	return t, nil
}

// normArgs x, ağırlık, sapma ve epsilon argümanlarını konumlu ya da adlı olarak okur.
func normArgs(fn string, args []interp.Value, named map[string]interp.Value, adlar []bind.Ad) (*tensor.Tensor, []interp.Value, float32, error) {
	if err := bind.AdliDenetle(fn+"()", named, append([]bind.Ad{adEpsilon}, adlar...)...); err != nil {
		return nil, nil, 0, err
	}
	if len(args) < 1 || len(args) > 1+len(adlar) {
		return nil, nil, 0, fmt.Errorf("%s 1 ile %d arasında konumlu parametre alır, %d verildi", fn, 1+len(adlar), len(args))
	}
	x, err := tensoreCevir(fn, args[0])
	if err != nil {
		return nil, nil, 0, err
	}
	if len(x.Shape()) == 0 {
		return nil, nil, 0, fmt.Errorf("%s: x en az 1 boyutlu olmalıdır", fn)
	}
	degerler := make([]interp.Value, len(adlar))
	for i, a := range adlar {
		if 1+i < len(args) {
			degerler[i] = args[1+i]
		}
		if v, ok := bind.AdliAl(named, a); ok {
			if degerler[i] != nil {
				return nil, nil, 0, fmt.Errorf("%s: %s hem konumlu hem adlı verildi", fn, a.TR)
			}
			degerler[i] = v
		}
	}
	eps := float32(1e-5)
	if v, ok := bind.AdliAl(named, adEpsilon); ok {
		f, ok := interp.ToFloat(v)
		if !ok || f <= 0 {
			return nil, nil, 0, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır, %s verildi", fn, adEpsilon.TR, v.Inspect())
		}
		eps = float32(f)
	}
	return x, degerler, eps, nil
}

func katmanNormu(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T501)
	x, p, eps, err := normArgs(fn, args, named, []bind.Ad{adAgirlik, adSapma})
	if err != nil {
		return nil, err
	}
	if x, err = yuksekHassasiyet(m, x); err != nil {
		return nil, err
	}
	sh := x.Shape()
	D := sh[len(sh)-1]
	w, err := normParam(fn, adAgirlik.TR, p[0], D)
	if err != nil {
		return nil, err
	}
	b, err := normParam(fn, adSapma.TR, p[1], D)
	if err != nil {
		return nil, err
	}
	if w, err = yuksekHassasiyet(m, w); err != nil {
		return nil, err
	}
	if b, err = yuksekHassasiyet(m, b); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.LayerNorm(x, w, b, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func rmsNormu(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T502)
	x, p, eps, err := normArgs(fn, args, named, []bind.Ad{adAgirlik})
	if err != nil {
		return nil, err
	}
	sh := x.Shape()
	w, err := normParam(fn, adAgirlik.TR, p[0], sh[len(sh)-1])
	if err != nil {
		return nil, err
	}
	if x, err = yuksekHassasiyet(m, x); err != nil {
		return nil, err
	}
	if w, err = yuksekHassasiyet(m, w); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.RMSNorm(x, w, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}
