package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"math"
)

// ----------------------------------------------------------------------------
// Sinir ağı fonksiyonları
// ----------------------------------------------------------------------------

// intIndices indeks verisini (tam sayı, iç içe liste ya da tensör) düz tam sayı
// dizisine ve şekline çevirir.
func intIndices(fn string, v interp.Value) ([]int, []int, error) {
	if n, ok := intNumber(v); ok {
		return []int{n}, nil, nil
	}
	var data []float32
	var sh []int
	switch x := v.(type) {
	case *TensorValue:
		data, sh = x.t.Data(), x.t.Shape()
	case *interp.ListVal:
		// Liste değerleri tam sayı olarak okunur (float32'den geçmez).
		return listInts(fn, x)
	default:
		return nil, nil, fmt.Errorf("%s: indeksler tam sayı, tam sayı listesi ya da tensör olmalıdır, %s verildi", fn, typeName(v))
	}
	out := make([]int, len(data))
	for i, f := range data {
		if float32(int(f)) != f {
			return nil, nil, fmt.Errorf("%s: indeksler tam sayı olmalıdır, %v verildi", fn, f)
		}
		out[i] = int(f)
	}
	return out, sh, nil
}

func embedding(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T500)
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(tablo, indeksler) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	table, ok := args[0].(*TensorValue)
	if !ok {
		return nil, fmt.Errorf("%s: tablo bir tensör olmalıdır, %s verildi", fn, typeName(args[0]))
	}
	tsh := table.t.Shape()
	if len(tsh) != 2 {
		return nil, fmt.Errorf("%s: tablo 2 boyutlu (satır sayısı × boyut) olmalıdır, %s verildi", fn, shapeText(tsh))
	}
	idx, ish, err := intIndices(fn, args[1])
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if i < 0 || i >= tsh[0] {
			return nil, fmt.Errorf("%s: indeks %d, %s şeklindeki tablonun dışında; 0 ile %d arasında olmalı", fn, i, shapeText(tsh), tsh[0]-1)
		}
	}
	r, err := safe(func() *tensor.Tensor {
		if len(ish) == 0 {
			// Tek indeks: sonuç tek bir satırdır, şekli [boyut].
			return tensor.Embedding(table.t, idx).Reshape(tsh[1])
		}
		return tensor.Embedding(table.t, idx, ish...)
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

func crossEntropy(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T510)
	if len(args) != 2 {
		return nil, fmt.Errorf("%s(çıktı, hedef) 2 parametre bekler, %d verildi", fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	output, err := toTensor(fn, args[0])
	if err != nil {
		return nil, err
	}
	// 16 bitlik logitler float32'ye açılmaz: çekirdek onları kendi türünde okur, hesabı ve
	// kaybı float32'de yapar (açılmış kopyayla aynı sonuç, B·T·V'lik kopya olmadan).
	sh := output.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("%s: çıktı en az 1 boyutlu olmalıdır (son eksen sınıflar)", fn)
	}
	target, _, err := intIndices(fn, args[1])
	if err != nil {
		return nil, err
	}
	C := sh[len(sh)-1]
	line := 0
	if C > 0 {
		line = output.Numel() / C
	}
	if len(target) != line && C > 1 && len(target) == output.Numel() && isZeroOne(target) {
		return nil, fmt.Errorf("%s: hedef sınıf indeksleri olmalı (ör. [2, 0, 1]); one-hot verdiniz gibi görünüyor (%s şeklindeki çıktıyla aynı sayıda 0/1); her satır için 1'in konumunu verin", fn, shapeText(sh))
	}
	if len(target) != line {
		return nil, fmt.Errorf("%s: %s şeklindeki çıktı için %d hedef bekleniyor, %d verildi", fn, shapeText(sh), line, len(target))
	}
	valid := 0
	for _, h := range target {
		if h >= C {
			return nil, fmt.Errorf("%s: hedef sınıf %d, %d sınıflı çıktının dışında; 0 ile %d arasında olmalı (negatif hedefler yok sayılır)", fn, h, C, C-1)
		}
		if h >= 0 {
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("%s: bütün hedefler yok sayıldı (%d satırın hepsi negatif ya da hiç satır yok); ortalama kayıp tanımsız. Yalnızca dolgudan oluşan partiyi atlayın", fn, len(target))
	}
	r, err := safe(func() *tensor.Tensor { return tensor.CrossEntropy(output, target) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

func isZeroOne(x []int) bool {
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
	t, err := toTensor(fn, v)
	if err != nil {
		return nil, err
	}
	if sh := t.Shape(); len(sh) != 1 || sh[0] != D {
		return nil, fmt.Errorf("%s: %s [%d] şeklinde olmalı (x'in son ekseni %d), %s verildi", fn, ad, D, D, shapeText(sh))
	}
	return t, nil
}

// normArgs x, ağırlık, sapma ve epsilon argümanlarını konumlu ya da adlı olarak okur.
func normArgs(fn string, args []interp.Value, named map[string]interp.Value, names []bind.Named) (*tensor.Tensor, []interp.Value, float32, error) {
	if err := bind.CheckNamed(fn+"()", named, append([]bind.Named{nameEpsilon}, names...)...); err != nil {
		return nil, nil, 0, err
	}
	if len(args) < 1 || len(args) > 1+len(names) {
		return nil, nil, 0, fmt.Errorf("%s 1 ile %d arasında konumlu parametre alır, %d verildi", fn, 1+len(names), len(args))
	}
	x, err := toTensor(fn, args[0])
	if err != nil {
		return nil, nil, 0, err
	}
	if len(x.Shape()) == 0 {
		return nil, nil, 0, fmt.Errorf("%s: x en az 1 boyutlu olmalıdır", fn)
	}
	values := make([]interp.Value, len(names))
	for i, a := range names {
		if 1+i < len(args) {
			values[i] = args[1+i]
		}
		if v, ok := bind.GetNamed(named, a); ok {
			if values[i] != nil {
				return nil, nil, 0, fmt.Errorf("%s: %s hem konumlu hem adlı verildi", fn, a.TR)
			}
			values[i] = v
		}
	}
	eps := float32(1e-5)
	if v, ok := bind.GetNamed(named, nameEpsilon); ok {
		// float32'de pozitif ve sonlu olmalı: NaN ve sonsuz (f <= 0 ikisini de geçirirdi) ya da
		// float32'de 0'a düşen değer çıktıyı NaN yapardı.
		f, ok := interp.ToFloat(v)
		if e := float32(f); !ok || !(e > 0) || math.IsInf(float64(e), 0) {
			return nil, nil, 0, fmt.Errorf("%s: %s pozitif ve sonlu bir sayı olmalıdır, %s verildi", fn, nameEpsilon.TR, v.Inspect())
		}
		eps = float32(f)
	}
	return x, values, eps, nil
}

func layerNorm(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T501)
	x, p, eps, err := normArgs(fn, args, named, []bind.Named{nameWeight, nameBias})
	if err != nil {
		return nil, err
	}
	if x, err = highPrecision(m, x); err != nil {
		return nil, err
	}
	sh := x.Shape()
	D := sh[len(sh)-1]
	w, err := normParam(fn, nameWeight.TR, p[0], D)
	if err != nil {
		return nil, err
	}
	b, err := normParam(fn, nameBias.TR, p[1], D)
	if err != nil {
		return nil, err
	}
	if w, err = highPrecision(m, w); err != nil {
		return nil, err
	}
	if b, err = highPrecision(m, b); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.LayerNorm(x, w, b, eps) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

func rmsNorm(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T502)
	x, p, eps, err := normArgs(fn, args, named, []bind.Named{nameWeight})
	if err != nil {
		return nil, err
	}
	sh := x.Shape()
	w, err := normParam(fn, nameWeight.TR, p[0], sh[len(sh)-1])
	if err != nil {
		return nil, err
	}
	if x, err = highPrecision(m, x); err != nil {
		return nil, err
	}
	if w, err = highPrecision(m, w); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.RMSNorm(x, w, eps) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
