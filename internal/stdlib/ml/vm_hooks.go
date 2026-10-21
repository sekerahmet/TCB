package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosya tensör değerinin VM kancalarını (bkz. internal/lang/interp/hooks.go) uygular: eşitlik,
// işleçler (@ + - * / ** ve karşılaştırmalar), tekli eksi, indeksleme, indeksle atama,
// döngüyle gezinme ve dış biçimlere (JSON) dönüşüm.

var (
	_ interp.Equaler          = (*TensorValue)(nil)
	_ interp.OperatorHook     = (*TensorValue)(nil)
	_ interp.TaskOperatorHook = (*TensorValue)(nil)
	_ interp.Negatable        = (*TensorValue)(nil)
	_ interp.Indexable        = (*TensorValue)(nil)
	_ interp.IndexAssignable  = (*TensorValue)(nil)
	_ interp.Iterable         = (*TensorValue)(nil)
	_ interp.PlainValuer      = (*TensorValue)(nil)
)

// PlainValue tensörü JSON gibi dış biçimlere yazmak için .değer'le aynı iç içe Ondalık
// listesine (0 boyutluda Ondalık'a) çevirir. Arayüz hata döndüremez; sıfır eksenli dev
// şekiller oluşturulurken (sıfırlar, şekillendir) zaten engellenir.
func (d *TensorValue) PlainValue() interp.Value { return d.nestedList() }

// Equal iki tensörün şekli ve tüm elemanları aynıysa doğrudur ('==' ve '!='). 0 boyutlu
// tensör, değeri (.değer ile aynı biçimde okunan) sayıya eşittir: m[1][0] == 7.0. Diğer
// değerlerle eşit değildir.
func (d *TensorValue) Equal(otherVal interp.Value) bool {
	o, ok := otherVal.(*TensorValue)
	if !ok {
		if f, number := interp.ToFloat(otherVal); number && d.t.Dim() == 0 {
			return shortFloat64(d.t.Item()) == f
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

// Negate tekli eksidir (-t); türev izlenir. Sonuç tazedir (yorumlayıcı sahiplenir).
func (d *TensorValue) Negate() (interp.Value, error) {
	start := tensor.Created()
	res, err := d.negate()
	if err == nil {
		markFresh(res, start)
	}
	return res, err
}

func (d *TensorValue) negate() (interp.Value, error) {
	r, err := safe(func() *tensor.Tensor { return d.t.Neg() })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// supportedOps hata mesajlarında gösterilir.
const supportedOps = "+ - * / @ ** < > <= >= == !="

// TaskOperator işleci yapan görevle birlikte çağrılır: '@' görevin karışık hassasiyet
// türüne göre (bkz. karışık_hassasiyet) hesaplanır; diğer işleçler Islec'e gider.
func (d *TensorValue) TaskOperator(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	start := tensor.Created()
	res, ok, err := d.taskOperator(m, op, otherVal, onRight)
	if err == nil {
		markFresh(res, start)
	}
	return res, ok, err
}

func (d *TensorValue) taskOperator(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	if op != "@" {
		return d.Operator(op, otherVal, onRight)
	}
	o, err := toTensorOn(messages.Text(messages.H6124), otherVal, d.t)
	if err != nil {
		return nil, true, err
	}
	a, b := d.t, o
	if onRight {
		a, b = o, d.t
	}
	r, err := matmulDType(a, b, mixedDType(m))
	if err != nil {
		return nil, true, err
	}
	return New(r), true, nil
}

// Operator tensörün ikili işleçlerini uygular: '+', '-', '*', '/' (eleman bazlı, yayınlamalı;
// diğer işlenen sayı, liste ya da tensör) ve '@' (matris çarpımı) her iki yönde; '**' sol
// işlenenken (sayı üs); '<', '>', '<=', '>=' her iki yönde (0/1 maskesi).
func (d *TensorValue) Operator(op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	start := tensor.Created()
	res, ok, err := d.operator(op, otherVal, onRight)
	if err == nil {
		markFresh(res, start)
	}
	return res, ok, err
}

func (d *TensorValue) operator(op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	switch op {
	case "+", "*":
		// Değişmeli: yön sonucu değiştirmez.
		v, err := d.binary(op, otherVal)
		return v, true, err

	case "-", "/":
		if !onRight {
			v, err := d.binary(op, otherVal)
			return v, true, err
		}
		if s, ok := interp.ToFloat(otherVal); ok {
			f := float32(s)
			r, err := safe(func() *tensor.Tensor {
				if op == "-" {
					return d.t.RSubScalar(f)
				}
				return d.t.RDivScalar(f)
			})
			if err != nil {
				return nil, true, err
			}
			return New(r), true, nil
		}
		o, err := toTensorOn(tensorOperation(op), otherVal, d.t)
		if err != nil {
			return nil, true, err
		}
		if err := checkBroadcast(op, o, d.t); err != nil {
			return nil, true, err
		}
		if err := checkDevice(tensorOperation(op), o, d.t); err != nil {
			return nil, true, err
		}
		r, err := safe(func() *tensor.Tensor {
			if op == "-" {
				return tensor.Sub(o, d.t)
			}
			return tensor.Div(o, d.t)
		})
		if err != nil {
			return nil, true, err
		}
		return New(r), true, nil

	case "@":
		o, err := toTensorOn(messages.Text(messages.H6124), otherVal, d.t)
		if err != nil {
			return nil, true, err
		}
		a, b := d.t, o
		if onRight {
			a, b = o, d.t
		}
		r, err := matmulDType(a, b, tensor.Float32)
		if err != nil {
			return nil, true, err
		}
		return New(r), true, nil

	case "**":
		s, ok := interp.ToFloat(otherVal)
		if onRight || !ok {
			return nil, true, messages.New(messages.H6702, codes.DisplayName(codes.T220), codes.DisplayName(codes.T221))
		}
		f := float32(s)
		r, err := safe(func() *tensor.Tensor { return d.t.Pow(f) })
		if err != nil {
			return nil, true, err
		}
		return New(r), true, nil

	case "<", ">", "<=", ">=":
		v, err := d.compare(op, otherVal, onRight)
		return v, true, err
	}
	return nil, true, messages.New(messages.H6703, interp.ValTensor, op, supportedOps)
}

// comparisonOp karşılaştırma işleçlerinin çekirdek işlemi.
var comparisonOp = map[string]tensor.BinOp{"<": tensor.OpLt, ">": tensor.OpGt, "<=": tensor.OpLe, ">=": tensor.OpGe}

// compare eleman bazlı karşılaştırmanın 0/1 maskesini (türev izlemeyen float32 tensör)
// tensörün cihazında verir. Diğer işlenen sayı, liste ya da tensör olabilir; şekiller
// yayınlanır (+ - * / gibi). NaN içeren karşılaştırma 0'dır.
func (d *TensorValue) compare(op string, otherVal interp.Value, onRight bool) (interp.Value, error) {
	bop := comparisonOp[op]
	if s, ok := interp.ToFloat(otherVal); ok {
		r, err := safe(func() *tensor.Tensor { return d.t.CompareScalar(float32(s), bop, onRight) })
		if err != nil {
			return nil, err
		}
		return New(r), nil
	}
	ad := messages.Text(messages.H6129, op)
	o, err := toTensorOn(ad, otherVal, d.t)
	if err != nil {
		return nil, err
	}
	a, b := d.t, o
	if onRight {
		a, b = o, d.t
	}
	if err := checkBroadcast(ad, a, b); err != nil {
		return nil, err
	}
	if err := checkDevice(ad, a, b); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.Compare(a, b, bop) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// Index t[i] (ilk eksende i. eleman; bir eksen eksilir) ve t[baş..son] (ilk eksende
// dilim) okumasını yapar. Sonuç veriyi kopyalamayan bir görünümdür ve türev izlenir.
func (d *TensorValue) Index(idx interp.Value) (interp.Value, error) {
	start := tensor.Created()
	res, err := d.index(idx)
	if err == nil {
		markFresh(res, start)
	}
	return res, err
}

func (d *TensorValue) index(idx interp.Value) (interp.Value, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, messages.New(messages.H6704)
	}
	n := sh[0]
	if rng, ok := idx.(*interp.RangeVal); ok {
		// Listelerdeki gibi: negatif uç sondan sayılır, dahil uç (..=) bundan sonra eklenir
		// (t[0..=-1] bütün tensör); sınır dışı uçlar kırpılır.
		head, last := int(rng.Start), int(rng.End)
		if head < 0 {
			head += n
		}
		if last < 0 {
			last += n
		}
		if rng.Inclusive {
			last++
		}
		head = min(max(head, 0), n)
		last = min(max(last, head), n)
		r, err := safe(func() *tensor.Tensor { return d.t.Slice(0, head, last) })
		if err != nil {
			return nil, err
		}
		return d.view(r), nil
	}
	i, ok := intNumber(idx)
	if !ok {
		return nil, messages.New(messages.H6707, interp.ValInt, idx.Type())
	}
	j := i
	if j < 0 {
		j += n
	}
	if j < 0 || j >= n {
		return nil, messages.New(messages.H6709, i, n, shapeText(sh))
	}
	r, err := safe(func() *tensor.Tensor { return d.t.Slice(0, j, j+1).Squeeze(0) })
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}

// view t'nin bir görünümünü (indeks, dilim, döngü öğesi) sarar. Görünüm aynı veriyi
// taşıdığından tam sayılı tensörün görünümü de tam sayılıdır (ör. token kimliği
// tensörünün satırları).
func (d *TensorValue) view(r *tensor.Tensor) *TensorValue {
	g := New(r)
	g.hasInts = d.hasInts
	return g
}

// IterLen 'döngü x içinde t' ve liste üretecinde gezilecek öğe sayısıdır (ilk
// eksenin uzunluğu). 0 boyutlu tensör üzerinde döngü kurulamaz.
func (d *TensorValue) IterLen() (int, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return 0, messages.New(messages.H6711, codes.DisplayName(memberValue))
	}
	return sh[0], nil
}

// IterItem ilk eksen boyunca i. öğedir: 1 boyutlu tensörde sayı (.değer'deki gibi
// Ondalık; tam sayılı tensörde Tam), N boyutluda t[i] ile aynı (N-1) boyutlu görünüm
// (veriyi paylaşır, türev izlenir).
func (d *TensorValue) IterItem(i int) (interp.Value, error) {
	if d.t.Dim() != 1 {
		return d.Index(interp.NewInt(int64(i)))
	}
	x, err := safe(func() float32 { return d.t.At(i) })
	if err != nil {
		return nil, err
	}
	if d.hasInts {
		return interp.NewInt(int64(x)), nil
	}
	return &interp.FloatVal{Val: shortFloat64(x)}, nil
}

// SetIndex t[i] = değer ve t[baş..son] = değer atamasıdır (t[i][j] = değer, t[i]
// görünümüne atamadır). Değer bir sayıysa hedefin tüm elemanlarına yazılır; liste ya da
// tensörse şekli hedefin şekliyle aynı olmalıdır. Veri yerinde değişir: bu tensörü kullanmış,
// henüz geri yayılmamış bir hesap çizgesi varsa geri yayılım hata verir. Türev izlenen bir
// tensöre yalnızca türevsiz { ... } içinde atanabilir.
func (d *TensorValue) SetIndex(idx, value interp.Value) error {
	if d.t.RequiresGrad() && tensor.GradEnabled() {
		return messages.New(messages.H6716)
	}
	var target *tensor.Tensor
	var err error
	tensor.NoGrad(func() {
		var v interp.Value
		if v, err = d.Index(idx); err == nil {
			target = v.(*TensorValue).t
		}
	})
	if err != nil {
		return err
	}
	sh := target.Shape()
	var src *tensor.Tensor
	if f, ok := interp.ToFloat(value); ok {
		src = tensor.Scalar(float32(f)) // hedefin tüm elemanlarına yayınlanır
	} else {
		if src, err = toTensor(messages.Text(messages.H6130), value); err != nil {
			return err
		}
		if ks := src.Shape(); shapeText(ks) != shapeText(sh) {
			return messages.New(messages.H6719, shapeText(sh), shapeText(ks))
		}
	}
	// Tek arka uç kopyası, tek sürüm artışı; hedef bitişik olmayan bir görünüm olabilir.
	_, err = safe(func() bool {
		if src.Device() != target.Device() {
			tensor.NoGrad(func() { src = src.To(target.Device()) })
		}
		target.CopyFrom(src)
		return true
	})
	return err
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter   = (*TensorValue)(nil)
	_ interp.Equaler          = (*TensorValue)(nil)
	_ interp.Indexable        = (*TensorValue)(nil)
	_ interp.IndexAssignable  = (*TensorValue)(nil)
	_ interp.Iterable         = (*TensorValue)(nil)
	_ interp.Negatable        = (*TensorValue)(nil)
	_ interp.OperatorHook     = (*TensorValue)(nil)
	_ interp.TaskOperatorHook = (*TensorValue)(nil)
	_ interp.PlainValuer      = (*TensorValue)(nil)
)
