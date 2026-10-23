package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"math"
	"slices"
	"strconv"
)

// ----------------------------------------------------------------------------
// Özellikler ve metotlar
// ----------------------------------------------------------------------------

// Tensör üyeleri (adları kod listesinde, internal/lang/codes Y4xx).
const (
	memberShape     = codes.Y400
	memberDevice    = codes.Y401
	memberDType     = codes.Y402
	memberGrad      = codes.Y403
	memberValue     = codes.Y404
	memberReshape   = codes.Y410
	memberTranspose = codes.Y411
	memberPermute   = codes.Y412
	memberSlice     = codes.Y413
	memberToDevice  = codes.Y414
	memberToDType   = codes.Y415
)

// tensorMemberCodes öneri listesinin sırasıdır.
var tensorMemberCodes = []codes.Code{memberShape, memberDevice, memberDType, memberGrad, memberValue, memberReshape, memberTranspose, memberPermute, memberSlice, memberToDevice, memberToDType, memberSplit}

var tensorMembers = codes.Names(tensorMemberCodes...)

func isTensorMember(k codes.Code) bool { return slices.Contains(tensorMemberCodes, k) }

func (d *TensorValue) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	t := d.t
	switch kd {
	case memberShape:
		return bind.IntList(t.Shape()), nil
	case memberDevice:
		return &interp.StringVal{Val: t.Device().String()}, nil
	case memberDType:
		return &interp.StringVal{Val: t.DType().Name()}, nil
	case memberToDType:
		return d.namedMethod(memberToDType, d.toDType), nil
	case memberGrad:
		if d.intermediate {
			return nil, messages.New(messages.H6700, shapeText(t.Shape()), nameRequiresGrad.TR, codes.DisplayName(codes.A040), codes.DisplayName(codes.T400), codes.DisplayName(codes.T401), codes.DisplayName(codes.T301), gradExample("..."))
		}
		g := t.Grad()
		if g == nil {
			return interp.Null, nil
		}
		return New(g), nil
	case memberValue:
		return d.valueList()
	case memberReshape:
		return d.method(memberReshape, d.reshape), nil
	case memberTranspose:
		return d.method(memberTranspose, func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, messages.New(messages.H6506, codes.DisplayName(memberTranspose))
			}
			return transposed(d)
		}), nil
	case memberPermute:
		return d.method(memberPermute, d.permute), nil
	case memberSlice:
		return d.method(memberSlice, d.slice), nil
	case memberToDevice:
		return d.method(memberToDevice, d.toDevice), nil
	case memberSplit:
		return d.namedMethod(memberSplit, d.split), nil
	}
	if k, _, ok := codes.FindForeign(codes.NamespaceMember, prop, isTensorMember); ok {
		o := codes.Name(k)
		if k == memberTranspose {
			o += "()" // metot
		}
		return nil, messages.New(messages.H7903, interp.ValTensor, prop, o)
	} else if o := turkish.Closest(prop, tensorMembers); o != "" {
		return nil, messages.New(messages.H7903, interp.ValTensor, prop, o)
	}
	return nil, messages.New(messages.H6721, interp.ValTensor, prop)
}

// method adlı argüman almayan bir tensör metodunun bağlı değeridir; namedMethod adlı
// argümanlarını kendisi denetleyen bir metodunkidir.
func (d *TensorValue) method(kd codes.Code, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: codes.Name(kd), Fn: freshResult(fn), NoNamedArgs: true, Pure: true}
}

func (d *TensorValue) namedMethod(kd codes.Code, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: codes.Name(kd), Fn: freshResult(fn), Pure: true}
}

// freshResult tensör metodunun sonucunu (yeni görünüm ya da tensör) taze işaretler.
func freshResult(fn interp.BuiltinFunc) interp.BuiltinFunc {
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		start := tensor.Created()
		res, err := fn(m, args, named)
		if err == nil {
			markFresh(res, start)
		}
		return res, err
	}
}

// shortFloat64 float32 değeri, onu yazan en kısa ondalık gösterime karşılık gelen
// float64'e çevirir: 0.1 (float32) → 0.1, 0.10000000149011612 değil. Böylece
// a.değer[0] == 0.1 doğru olur.
func shortFloat64(x float32) float64 {
	f := float64(x)
	if f == math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	var buffer [32]byte
	s := strconv.AppendFloat(buffer[:0], f, 'g', -1, 32)
	if g, err := strconv.ParseFloat(string(s), 64); err == nil {
		return g
	}
	return f
}

// valueList tensörün değerlerini Ondalık (0 boyutlu) ya da iç içe liste olarak verir.
// Büyük tensörlerde her liste ve sayı için ayrı ayırma yapmamak için tüm listeler,
// eleman dilimleri ve sayılar önceden ayrılmış üç dilimden dağıtılır. Eleman dilimleri
// kapasiteyle sınırlıdır (a[i:j:j]); listeye eleman eklemek komşu listeyi bozmaz.
func (d *TensorValue) valueList() (interp.Value, error) {
	if d.t.Numel() == 0 {
		// Veri yokken liste sayısı veriden bağımsızdır: [10⁹, 0] 10⁹ boş liste isterdi.
		// Dolu tensörde liste sayısı eleman sayısını aşmaz.
		if err := checkShapeLimit(codes.Name(memberValue), d.t.Shape()); err != nil {
			return nil, err
		}
	}
	return d.nestedList(), nil
}

// nestedList tensörü iç içe listeye çevirir (sınır denetimsiz; bkz. valueList).
func (d *TensorValue) nestedList() interp.Value {
	data := d.t.Data()
	sh := d.t.Shape()
	if len(sh) == 0 {
		if d.hasInts {
			return interp.NewInt(int64(data[0]))
		}
		return &interp.FloatVal{Val: shortFloat64(data[0])}
	}
	nd := len(sh)
	countList, countValues, p := 0, 0, 1
	for k := 0; k < nd; k++ {
		countList += p
		p *= sh[k]
		countValues += p
	}
	lists := make([]interp.ListVal, countList)
	values := make([]interp.Value, countValues)
	var numbers []interp.FloatVal
	if !d.hasInts {
		numbers = make([]interp.FloatVal, len(data))
		for i, x := range data {
			numbers[i].Val = shortFloat64(x)
		}
	}
	li, di := 0, 0 // bu seviyenin ilk listesi ve ilk eleman yeri
	p = 1          // bu seviyedeki liste sayısı
	for k := 0; k < nd; k++ {
		n := sh[k]
		nextList := li + p
		for j := 0; j < p; j++ {
			head := di + j*n
			el := values[head : head+n : head+n]
			for i := range el {
				m := j*n + i // bu seviyedeki elemanın sırası
				if k == nd-1 && d.hasInts {
					el[i] = interp.NewInt(int64(data[m]))
				} else if k == nd-1 {
					el[i] = &numbers[m]
				} else {
					el[i] = &lists[nextList+m]
				}
			}
			lists[li+j].Elements = el
		}
		li = nextList
		di += p * n
		p *= n
	}
	return &lists[0]
}

// unpackListOrVarargs tek liste argümanının öğelerini, yoksa argümanların kendisini verir
// (t.yeniden_şekillendir([2, 3]) ve t.yeniden_şekillendir(2, 3) aynıdır).
func unpackListOrVarargs(args []interp.Value) []interp.Value {
	if len(args) == 1 {
		if l, ok := args[0].(*interp.ListVal); ok && l.Elements != nil {
			return l.Elements
		}
	}
	return args
}

func (d *TensorValue) reshape(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	// Yeni şekil tek bir liste olarak verilir; [] 0 boyutlu şekildir (15.2). Sayı argümanları
	// ('.şekillendir(2, 3)') şekil sayılmaz.
	var list *interp.ListVal
	if len(args) == 1 {
		list, _ = args[0].(*interp.ListVal)
	}
	if list == nil {
		return nil, messages.New(messages.H6705, codes.DisplayName(memberReshape))
	}
	target := list.Elements
	sh := make([]int, len(target))
	unknown := 0
	for i, v := range target {
		n, ok := intNumber(v)
		if !ok || n < -1 {
			return nil, messages.New(messages.H6706, codes.DisplayName(memberReshape), i+1, v.Inspect())
		}
		if n == -1 {
			unknown++
		}
		sh[i] = n
	}
	if unknown > 1 {
		return nil, messages.New(messages.H6708, codes.DisplayName(memberReshape), shapeText(sh))
	}
	if d.t.Numel() == 0 {
		// Veri yokken eksenler serbesttir ([0] → [10¹², 0]); sınır sıfır eksenler 1 sayılarak
		// uygulanır. Dolu tensörde eksenlerin çarpımı eleman sayısına eşit olmak zorundadır.
		if err := checkShapeLimit(codes.Name(memberReshape), sh); err != nil {
			return nil, err
		}
	}
	n := d.t.Numel()
	product := 1
	for _, x := range sh {
		if x != -1 {
			product *= x
		}
	}
	if (unknown == 0 && product != n) || (unknown == 1 && (product == 0 || n%product != 0)) {
		return nil, messages.New(messages.H6710, shapeText(d.t.Shape()), n, shapeText(sh))
	}
	r, err := safe(func() *tensor.Tensor { return d.t.Reshape(sh...) })
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}

func transposed(d *TensorValue) (interp.Value, error) {
	t := d.t
	sh := t.Shape()
	if len(sh) < 2 {
		return nil, messages.New(messages.H6712, codes.DisplayName(memberTranspose), shapeText(sh))
	}
	r, err := safe(func() *tensor.Tensor { return t.T() })
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}

func (d *TensorValue) permute(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	list := unpackListOrVarargs(args)
	fn := codes.DisplayName(memberPermute)
	sh := d.t.Shape()
	if len(list) != len(sh) {
		return nil, messages.New(messages.H6713, fn, shapeText(sh), len(sh), len(list))
	}
	dims := make([]int, len(list))
	seen := make([]bool, len(sh))
	for i, v := range list {
		n, ok := intNumber(v)
		if !ok {
			return nil, messages.New(messages.H6714, fn, v.Inspect())
		}
		e, err := parseAxis(fn, n, sh)
		if err != nil {
			return nil, err
		}
		if seen[e] {
			return nil, messages.New(messages.H6715, fn, e)
		}
		seen[e] = true
		dims[i] = e
	}
	r, err := safe(func() *tensor.Tensor { return d.t.Permute(dims...) })
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}

func (d *TensorValue) slice(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberSlice)
	if len(args) != 3 {
		return nil, messages.New(messages.H6717, fn, len(args))
	}
	var n [3]int
	for i, a := range args {
		x, ok := intNumber(a)
		if !ok {
			return nil, messages.New(messages.H6718, fn, a.Inspect())
		}
		n[i] = x
	}
	sh := d.t.Shape()
	e, err := parseAxis(fn, n[0], sh)
	if err != nil {
		return nil, err
	}
	head, last := n[1], n[2]
	uz := sh[e]
	if head < 0 {
		head += uz
	}
	if last < 0 {
		last += uz
	}
	if head < 0 || head > uz || last < head || last > uz {
		return nil, messages.New(messages.H6720, fn, e, uz, shapeText(sh), n[1], n[2])
	}
	r, err := safe(func() *tensor.Tensor { return d.t.Slice(e, head, last) })
	if err != nil {
		return nil, err
	}
	return d.view(r), nil
}
