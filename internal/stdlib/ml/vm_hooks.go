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

// PlainValue tensörü JSON gibi dış biçimlere yazmak için .değer'le aynı iç içe Ondalık
// listesine (0 boyutluda Ondalık'a) çevirir. Arayüz hata döndüremez; sıfır eksenli dev
// şekiller oluşturulurken (sıfırlar, şekillendir) zaten engellenir.
func (d *TensorValue) PlainValue() interp.Value { return d.nestedList() }

// Equal iki tensörün şekli ve tüm elemanları aynıysa doğrudur (TR_assert_equal, 12.4). 0 boyutlu
// tensör, değeri (.değer ile aynı biçimde okunan) sayıya eşittir: m[1][0] == 7.0. Diğer
// değerlerle eşit değildir.
func (d *TensorValue) Equal(otherVal interp.Value) bool {
	o, ok := otherVal.(*TensorValue)
	if !ok {
		if f, number := interp.ToFloat(otherVal); number && d.t.Dim() == 0 {
			if d.t.DType().IsTyped() {
				return interp.ValuesEqual(d.scalarValue(), otherVal)
			}
			return shortFloat64(d.t.Item()) == f
		}
		return false
	}
	// Aynı nesne de öğe öğe: NaN tutan tensör kendisine eşit değildir, TR_equal'daki gibi (4.5, 12.4).
	a, b := d.t.Shape(), o.t.Shape()
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	if d.t.DType().IsTyped() || o.t.DType().IsTyped() {
		return d.t.Elem() == o.t.Elem() && interp.ValuesEqual(d.nestedList(), o.nestedList())
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
	if err := float64GradError(d); err != nil {
		return nil, err // tekli eksi çizgeye kenar ekler (15.46)
	}
	if d.t.DType().IsTyped() {
		var f *tensor.ElemFault
		r, err := safe(func() (r *tensor.Tensor) { r, f = tensor.Neg(d.t); return })
		if err != nil {
			return nil, err
		}
		return typedResult(codes.NameRef(codes.TypeTensor), "-", r, f)
	}
	r, err := safe(func() *tensor.Tensor { return d.t.Neg() })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// supportedOps hata mesajlarında gösterilir.
const supportedOps = "+ - * / @ ** < > <= >= == !="

// TaskOperator tensörün ikili işleçlerini uygular: '+', '-', '*', '/' (eleman bazlı,
// yayınlamalı; diğer işlenen sayı, liste ya da tensör) ve '@' (matris çarpımı) her iki yönde;
// '**' sol işlenenken (sayı üs); '<', '>', '<=', '>=' her iki yönde (TR_Bool maske, 15.5). İşleci yapan
// görevle çağrılır: '@' görevin karışık hassasiyet türüne göre hesaplanır (bkz.
// karışık_hassasiyet), yayın uyarısı programın durumuna ve hata akışına yazılır. Tensörün
// görevsiz işleç kancası (interp.OperatorHook) yoktur: yorumlayıcı önce bu kancayı dener.
func (d *TensorValue) TaskOperator(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	start := tensor.Created()
	res, ok, err := d.taskOperator(m, op, otherVal, onRight)
	if err == nil {
		markFresh(res, start)
	}
	return res, ok, err
}

// comparisonOps türev çizgesine kenar eklemeyen ikili işleçlerdir (sonuç mantık tensörü).
var comparisonOps = map[string]bool{"<": true, ">": true, "<=": true, ">=": true, "==": true, "!=": true}

func (d *TensorValue) taskOperator(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	// Aritmetik ve '@' türevi izlenen float64'te H6040 verir, H6960'tan önce (15.46);
	// karşılaştırma çizge kurmaz.
	if !comparisonOps[op] {
		if err := float64GradError(d, otherVal); err != nil {
			return nil, true, err
		}
	}
	if op != "@" {
		return d.operator(m, op, otherVal, onRight) // TaskOperator zaten markFresh çağırır
	}
	return d.matmulOp(m, otherVal, onRight, mixedDType(m))
}

// matmulOp '@' işlecidir: işlenen tensöre çevrilir, matris çarpımı dt türünde yapılır.
func (d *TensorValue) matmulOp(m *interp.VM, otherVal interp.Value, onRight bool, dt tensor.DType) (interp.Value, bool, error) {
	o, err := toTensorOn(messages.New(messages.H6124), otherVal, d.t)
	if err != nil {
		return nil, true, err
	}
	a, b := d.t, o
	if onRight {
		a, b = o, d.t
	}
	r, err := matmulDType(m, a, b, dt)
	if err != nil {
		return nil, true, err
	}
	return New(r), true, nil
}

// operator ikili işleci uygular; m işlemi yapan görevdir (nil: program yok, uyarı yazılmaz).
func (d *TensorValue) operator(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, bool, error) {
	if d.t.DType().IsTyped() || isTypedOperand(otherVal) {
		v, err := d.typedOperator(m, op, otherVal, onRight)
		return v, true, err
	}
	switch op {
	case "+", "*":
		// Değişmeli: yön sonucu değiştirmez.
		v, err := d.binary(m, op, otherVal)
		return v, true, err

	case "-", "/":
		if !onRight {
			v, err := d.binary(m, op, otherVal)
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
		if err := checkBroadcast(m, op, o, d.t); err != nil {
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
		return d.matmulOp(m, otherVal, onRight, tensor.Float32)

	case "**":
		s, ok := interp.ToFloat(otherVal)
		if onRight || !ok {
			return nil, true, messages.New(messages.H6702, codes.NameRef(codes.FnExp), codes.NameRef(codes.FnLog))
		}
		f := float32(s)
		r, err := safe(func() *tensor.Tensor { return d.t.Pow(f) })
		if err != nil {
			return nil, true, err
		}
		return New(r), true, nil

	case "<", ">", "<=", ">=", "==", "!=":
		v, err := d.compare(m, op, otherVal, onRight)
		return v, true, err
	}
	return nil, true, messages.New(messages.H6703, interp.ValTensor, op, supportedOps)
}

// comparisonOp kaynaştır kaydındaki karşılaştırmanın çekirdek işlemi.
var comparisonOp = map[string]tensor.BinOp{"<": tensor.OpLt, ">": tensor.OpGt, "<=": tensor.OpLe, ">=": tensor.OpGe}

// compare hesap ondalığı tensörün öğe öğe karşılaştırmasıdır (15.5): sonuç TR_Tensor[TR_Bool],
// türev izlemez. Diğer işlenen sayı, liste ya da tensör olabilir; şekiller yayınlanır. Sayı
// float32'ye çevrilir, 16 bitlik tensör float32'de karşılaştırılır (15.6a). NaN'lı
// karşılaştırma yanlış, != doğrudur (IEEE 754). Kaynaştır kaydında karşılaştırma bugün 0/1
// ondalık maskedir (kaynaşmış çekirdekte mantık cinsi 2b'de).
func (d *TensorValue) compare(m *interp.VM, op string, otherVal interp.Value, onRight bool) (interp.Value, error) {
	ad := messages.New(messages.H6129, op)
	var o *tensor.Tensor
	var err error
	if s, ok := interp.ToFloat(otherVal); ok && d.t.IsSymbolic() {
		// Kaynaştır kaydında sayı sabit olarak kaydedilir (dışarıdan tensör değildir).
		bop, cmp := comparisonOp[op]
		if !cmp {
			return nil, messages.New(messages.H3107)
		}
		return safe(func() interp.Value { return New(d.t.CompareScalar(float32(s), bop, onRight)) })
	} else if ok {
		o, err = safe(func() *tensor.Tensor { return tensor.FullOn(d.t.Device(), float32(s)) }) // cihazda doldurulur: TR_replay kaydında da
	} else {
		o, err = toTensorOn(ad, otherVal, d.t)
	}
	if err != nil {
		return nil, err
	}
	a, b := d.t, o
	if onRight {
		a, b = o, d.t
	}
	if err := checkBroadcast(m, ad, a, b); err != nil {
		return nil, err
	}
	if err := checkDevice(ad, a, b); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor {
		if a.IsSymbolic() || b.IsSymbolic() {
			bop, ok := comparisonOp[op]
			if !ok {
				panic(messages.New(messages.H3107)) // kaynaştır kaydında == ve != yok (2b)
			}
			return tensor.Compare(a, b, bop)
		}
		return tensor.CompareTyped(a, b, cmpOps[op])
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

var _ interp.ValueEqualer = (*TensorValue)(nil)

// ValueEqual tensörün == ve != işleçleridir: öğe öğe, TR_Tensor[TR_Bool] verir (15.5). İki
// tensörün bütün eşitliği TR_equal'dır. TR_none ile karşılaştırma (3.5) ve öbür değerler
// yorumlayıcının olağan yolundan gider (ok false). Yayın uyarısı yazılmaz: kancanın görevi yok.
func (d *TensorValue) ValueEqual(op string, other interp.Value) (interp.Value, bool, error) {
	switch other.(type) {
	case *TensorValue, *interp.IntVal, *interp.FloatVal, *interp.BoolVal, *interp.ListVal:
	default:
		return nil, false, nil
	}
	start := tensor.Created()
	res, _, err := d.operator(nil, op, other, false)
	if err == nil {
		markFresh(res, start)
	}
	return res, true, err
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
	if err := float64GradError(d); err != nil {
		return nil, err // indeks görünümü çizgeye kenar ekler (15.46); dolaşma da buradan geçer
	}
	sh := d.t.Shape()
	if len(sh) == 0 {
		return nil, messages.New(messages.H6704)
	}
	n := sh[0]
	if rng, ok := idx.(*interp.RangeVal); ok {
		// Listelerle aynı sınır hesabı: negatif uç sondan sayılır, dahil uç (..=) bundan sonra
		// eklenir (t[0..=-1] bütün tensör); sınır dışı uçlar kırpılır.
		head, last := interp.SliceBounds(rng, n)
		r, err := safe(func() *tensor.Tensor { return d.t.Slice(0, head, last) })
		if err != nil {
			return nil, err
		}
		return d.view(r), nil
	}
	if it, ok := idx.(*TensorValue); ok && it.t.Elem() == tensor.ElemInt {
		return d.indexByTensor(it)
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
		return nil, messages.New(messages.H6709, i, n, tensor.ShapeText(sh))
	}
	r, err := safe(func() *tensor.Tensor {
		s := d.t.Slice(0, j, j+1)
		r := s.Squeeze(0)
		dropTemp(d.t, s)
		return r
	})
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}

// indexByTensor t[idx]'tir (15.6d): idx ilk eksenden satırları seçer, veri kopyalanır, türev izlenir.
// Kırpma yoktur: eksi indeks de sınır dışıdır.
func (d *TensorValue) indexByTensor(idx *TensorValue) (interp.Value, error) {
	ints := idx.t.Int64s()
	rows := make([]int, len(ints))
	for i, v := range ints {
		rows[i] = int(v)
	}
	r, err := safe(func() *tensor.Tensor { return tensor.TakeRows(d.t, rows, idx.t.Shape()) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

// view t'nin bir görünümünü (indeks, dilim, döngü öğesi) sarar; görünüm cinsi korur.
func (d *TensorValue) view(r *tensor.Tensor) *TensorValue { return New(r) }

// IterLen 'döngü x içinde t' ve liste üretecinde gezilecek öğe sayısıdır (ilk
// eksenin uzunluğu). 0 boyutlu tensör üzerinde döngü kurulamaz.
func (d *TensorValue) IterLen() (int, error) {
	sh := d.t.Shape()
	if len(sh) == 0 {
		return 0, messages.New(messages.H6711, codes.NameRef(memberValue))
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
	if d.t.DType().IsTyped() {
		v, err := d.Index(interp.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}
		return v.(*TensorValue).scalarValue(), nil
	}
	x, err := safe(func() float32 { return d.t.At(i) })
	if err != nil {
		return nil, err
	}
	return &interp.FloatVal{Val: shortFloat64(x)}, nil
}

// SetIndex t[i] = değer ve t[baş..son] = değer atamasıdır (t[i][j] = değer, t[i]
// görünümüne atamadır). Değer bir sayıysa hedefin tüm elemanlarına yazılır; liste ya da
// tensörse şekli hedefin şekliyle aynı olmalıdır. Veri yerinde değişir: bu tensörü kullanmış,
// henüz geri yayılmamış bir hesap çizgesi varsa geri yayılım hata verir. Türevin yazma
// kuralları checkWrite'tadır. Tensörle indeks kopya verir, ona yazılmaz (tensorIndexWrite).
func (d *TensorValue) SetIndex(idx, value interp.Value) error {
	if err := checkWrite(d.t, valueTensor(value), false); err != nil {
		return err
	}
	var target *tensor.Tensor
	var view *TensorValue
	var err error
	defer func() {
		if view != nil {
			view.Release() // görünüm başlığı bu işlemin ara değeridir
		}
	}()
	tensor.NoGrad(func() {
		var v interp.Value
		if v, err = d.Index(idx); err == nil {
			view = v.(*TensorValue)
			target = view.t
		}
	})
	if err != nil {
		return err
	}
	if _, ok := idx.(*TensorValue); ok {
		return tensorIndexWrite("", value, target)
	}
	sh := target.Shape()
	var src *tensor.Tensor
	_, isTensor := value.(*TensorValue)
	_, isList := value.(*interp.ListVal)
	if target.DType().IsTyped() && !isTensor && !isList {
		// Cinsli hedefe sayı ya da mantık değeri: hedefin cinsinden, hassasiyetine sığarak (15.6a).
		if src, err = view.typedOperandFor("=", "=", value, true); err != nil {
			return err
		}
	} else if f, ok := interp.ToFloat(value); ok {
		// Hedefin tüm elemanlarına yayınlanır; sayı cihazda doldurulur (konaktan yüklenmez:
		// tekrar_oynat kaydında da çalışır).
		if src, err = safe(func() *tensor.Tensor { return tensor.FullOn(target.Device(), float32(f)) }); err != nil {
			return err
		}
	} else {
		// Liste hedefin cihazında tensöre çevrilir (15.38); başka cihazdaki tensör sessizce
		// kopyalanmaz, açık hatadır (15.40).
		if src, err = toTensorOn(messages.New(messages.H6130), value, target); err != nil {
			return err
		}
		if err = checkDevice(messages.New(messages.H6130), src, target); err != nil {
			return err
		}
		if ks := src.Shape(); tensor.ShapeText(ks) != tensor.ShapeText(sh) {
			return messages.New(messages.H6719, tensor.ShapeText(sh), tensor.ShapeText(ks))
		}
	}
	if src, err = assignSource(target, src); err != nil {
		return err
	}
	// Tek arka uç kopyası, tek sürüm artışı; hedef bitişik olmayan bir görünüm olabilir.
	_, err = safe(func() bool {
		target.CopyFrom(src)
		return true
	})
	return err
}

// tensorIndexWrite t[idx] = d ve t[idx] op= d'nin hatasıdır (H7042; atamada op boştur). t[idx] kopya
// verir (15.6d): yazı kopyaya gider, t değişmezdi. İndeks denetimi önce geçer, öneri döngüsünün
// sondan saydığı eksi indeks H6694 kalır. target t[idx]'in kopyasıdır: d onun şeklindeyse her
// indekse kendi parçası, değilse (sayı, bir satır) aynı d yazılır.
func tensorIndexWrite(op string, value interp.Value, target *tensor.Tensor) error {
	form := "whole"
	sh := tensor.ShapeText(target.Shape())
	switch v := value.(type) {
	case *TensorValue:
		if tensor.ShapeText(v.t.Shape()) == sh {
			form = "each"
		}
	case *interp.ListVal:
		src, err := toTensorOn(messages.New(messages.H6130), value, target)
		if err == nil && tensor.ShapeText(src.Shape()) == sh {
			form = "list"
		}
	}
	return messages.New(messages.H7042, op, form)
}

// assignSource atanan tensörü hedefin hassasiyetine getirir (15.6d): cinsli hedefte ya da
// kaynakta iki yan aynı cinsten olmalıdır; tam sayıda değer hedefin hassasiyetine sığmalıdır.
func assignSource(target, src *tensor.Tensor) (*tensor.Tensor, error) {
	if src.DType() == target.DType() || !src.DType().IsTyped() && !target.DType().IsTyped() {
		return src, nil
	}
	if src.Elem() != target.Elem() {
		return nil, kindMismatch("=", target.DType(), src.DType())
	}
	return convertKind("=", src, target.DType())
}

// checkWrite yerinde yazmanın türev kurallarını (15.6d "Türev") uygular. dst yazılan tensördür
// (görünümse görünümün kendisi), src yazılan değer (tensör değilse nil); whole bütün hedefe
// yazmaktır (bileşik atama), değilse indeksli hedeftir. Kip sözcüksel değildir: türevsiz bloktan
// çağrılan fonksiyon da kiptedir (tensor.GradEnabled bloğun çalıştığı süreyi söyler).
//   - türev izleyen yaprağa (ve görünümüne) yalnız türevsiz kipte yazılır (H6716);
//   - türev izleyen ara sonuca yerinde yazmak türevsiz kipte de hatadır (H6955);
//   - türev izlemeyen hedefe kipin dışında türev izleyen değer yazmak hatadır (H6956).
func checkWrite(dst, src *tensor.Tensor, whole bool) error {
	if dst.DType().IsQuantized() {
		return messages.New(messages.H6957, dst.DType().Name())
	}
	if l := dst.WriteLock(); l != nil {
		// Kopya olan .TR_reshape'in deposu (ya da görünümü), türevsiz kipte de (15.3).
		return messages.New(messages.H6041, tensor.ShapeText(dst.Shape()), tensor.ShapeText(l.Shape), tensor.ShapeText(l.Strides))
	}
	tracked, leaf := dst.GradTarget()
	switch {
	case tracked && leaf:
		if tensor.GradEnabled() {
			return messages.New(messages.H6716, codes.NameRef(codes.FnNoGrad))
		}
	case tracked:
		return messages.New(messages.H6955, map[bool]string{true: "whole", false: "index"}[whole])
	case src != nil && src.RequiresGrad() && tensor.GradEnabled():
		return messages.New(messages.H6956)
	}
	return nil
}

// valueTensor yazılan değerin tensörüdür; tensör değilse (sayı, liste) nil: türev izlemez.
func valueTensor(v interp.Value) *tensor.Tensor {
	if t, ok := v.(*TensorValue); ok {
		return t.t
	}
	return nil
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
	_ interp.TaskOperatorHook = (*TensorValue)(nil)
	_ interp.PlainValuer      = (*TensorValue)(nil)
)
