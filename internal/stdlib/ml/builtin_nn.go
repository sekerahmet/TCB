package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
		return nil, nil, messages.New(messages.H6300, fn, typeName(v))
	}
	out := make([]int, len(data))
	for i, f := range data {
		if float32(int(f)) != f {
			return nil, nil, messages.New(messages.H6012, fn, f)
		}
		out[i] = int(f)
	}
	return out, sh, nil
}

func embedding(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T500)
	if len(args) != 2 {
		return nil, messages.New(messages.H6301, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	table, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H6302, fn, typeName(args[0]))
	}
	tsh := table.t.Shape()
	if len(tsh) != 2 {
		return nil, messages.New(messages.H6303, fn, shapeText(tsh))
	}
	if it, ok := args[1].(*TensorValue); ok && tensor.Capturing(table.t.Device()) {
		// Kayıt içinde indeks konağa okunamaz: her oynatma onu cihazdan okur (sınır dışı
		// indeks kırpılır; tensor.EmbeddingAt).
		if err := checkDevice(fn, table.t, it.t); err != nil {
			return nil, err
		}
		r, err := safe(func() *tensor.Tensor { return tensor.EmbeddingAt(table.t, it.t) })
		if err != nil {
			return nil, err
		}
		return New(r), nil
	}
	idx, ish, err := intIndices(fn, args[1])
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if i < 0 || i >= tsh[0] {
			return nil, messages.New(messages.H6305, fn, i, shapeText(tsh), tsh[0]-1)
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
	fn := codes.DisplayName(codes.T510)
	if len(args) != 2 {
		return nil, messages.New(messages.H6308, fn, len(args))
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
		return nil, messages.New(messages.H6310, fn)
	}
	target, err := lossTargets(fn, sh, args[1])
	if err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.CrossEntropy(output, target) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// lossTargets çapraz entropinin hedeflerini okur ve sh şekilli logitlere göre denetler: satır
// başına bir sınıf indeksi (one-hot değil), sınıf sayısının altında, en az biri geçerli
// (negatifler yok sayılır).
func lossTargets(fn string, sh []int, v interp.Value) ([]int, error) {
	target, _, err := intIndices(fn, v)
	if err != nil {
		return nil, err
	}
	C := sh[len(sh)-1]
	n := 1
	for _, d := range sh {
		n *= d
	}
	line := 0
	if C > 0 {
		line = n / C
	}
	if len(target) != line && C > 1 && len(target) == n && isZeroOne(target) {
		return nil, messages.New(messages.H6312, fn, shapeText(sh))
	}
	if len(target) != line {
		return nil, messages.New(messages.H6313, fn, shapeText(sh), line, len(target))
	}
	valid := 0
	for _, h := range target {
		if h >= C {
			return nil, messages.New(messages.H6314, fn, h, C, C-1)
		}
		if h >= 0 {
			valid++
		}
	}
	if valid == 0 {
		return nil, messages.New(messages.H6316, fn, len(target))
	}
	return target, nil
}

// linearCrossEntropy doğrusal_çapraz_entropi(x, ağırlık, hedef): çapraz_entropi(x @ ağırlık,
// hedef) ile aynı kayıp ve gradyanlar, ama bütün logitler hiç tutulmaz (tensor.LinearCrossEntropy,
// dil tanımı 15.43). Karışık hassasiyet bloğunda x ve ağırlık `@` gibi blok türüne çevrilir.
func linearCrossEntropy(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T512)
	if len(args) != 3 {
		return nil, messages.New(messages.H6345, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	x, err := toTensor(fn, args[0])
	if err != nil {
		return nil, err
	}
	w, err := toTensorOn(fn, args[1], x)
	if err != nil {
		return nil, err
	}
	xs, ws := x.Shape(), w.Shape()
	if len(xs) == 0 || len(ws) != 2 || xs[len(xs)-1] != ws[0] {
		return nil, messages.New(messages.H6346, fn, shapeText(xs), shapeText(ws))
	}
	logits := append(append([]int(nil), xs[:len(xs)-1]...), ws[1])
	target, err := lossTargets(fn, logits, args[2])
	if err != nil {
		return nil, err
	}
	mixed := mixedDType(m)
	r, err := safe(func() *tensor.Tensor {
		if mixed != tensor.Float32 {
			return tensor.LinearCrossEntropy(x.CastShared(mixed), w.CastShared(mixed), target)
		}
		return tensor.LinearCrossEntropy(x, w, target)
	})
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
	if v == nil || v == interp.Null {
		return nil, nil
	}
	t, err := toTensor(fn, v)
	if err != nil {
		return nil, err
	}
	if sh := t.Shape(); len(sh) != 1 || sh[0] != D {
		return nil, messages.New(messages.H6319, fn, ad, D, shapeText(sh))
	}
	return t, nil
}

// normArgs x, ağırlık, sapma ve epsilon argümanlarını konumlu ya da adlı olarak okur.
func normArgs(fn string, args []interp.Value, named map[string]interp.Value, names []bind.Named) (*tensor.Tensor, []interp.Value, float32, tensor.DType, error) {
	if err := bind.CheckNamed(fn+"()", named, append([]bind.Named{nameEpsilon, nameDType}, names...)...); err != nil {
		return nil, nil, 0, 0, err
	}
	if len(args) < 1 || len(args) > 1+len(names) {
		return nil, nil, 0, 0, messages.New(messages.H6320, fn, 1+len(names), len(args))
	}
	x, err := toTensor(fn, args[0])
	if err != nil {
		return nil, nil, 0, 0, err
	}
	if len(x.Shape()) == 0 {
		return nil, nil, 0, 0, messages.New(messages.H6322, fn)
	}
	values := make([]interp.Value, len(names))
	for i, a := range names {
		if 1+i < len(args) {
			values[i] = args[1+i]
		}
		if v, ok := bind.GetNamed(named, a); ok {
			if values[i] != nil {
				return nil, nil, 0, 0, messages.New(messages.H5528, fn, a.TR)
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
			return nil, nil, 0, 0, messages.New(messages.H6325, fn, nameEpsilon.TR, v.Inspect())
		}
		eps = float32(f)
	}
	// tür: çıktının türü (hesap float32'dir); verilmezse girdinin türü (15.13).
	dt := x.DType()
	if v, ok := bind.GetNamed(named, nameDType); ok {
		if dt, err = parseDTypeValue(fn, v); err != nil {
			return nil, nil, 0, 0, err
		}
	}
	return x, values, eps, dt, nil
}

func layerNorm(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T501)
	x, p, eps, dt, err := normArgs(fn, args, named, []bind.Named{nameWeight, nameBias})
	if err != nil {
		return nil, err
	}
	// 16 bitlik girdi açılmaz: çekirdek onu float32'de normlar, sonuç girdinin türündedir
	// (dil tanımı 15.13). Ağırlık ve sapma float32 kalır.
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
	r, err := safe(func() *tensor.Tensor { return tensor.LayerNormAs(x, w, b, eps, dt) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

func rmsNorm(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T502)
	x, p, eps, dt, err := normArgs(fn, args, named, []bind.Named{nameWeight})
	if err != nil {
		return nil, err
	}
	sh := x.Shape()
	w, err := normParam(fn, nameWeight.TR, p[0], sh[len(sh)-1])
	if err != nil {
		return nil, err
	}
	// 16 bitlik girdi açılmaz (katman_normu gibi; dil tanımı 15.13).
	if w, err = highPrecision(m, w); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.RMSNormAs(x, w, eps, dt) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
