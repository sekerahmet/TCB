package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Veriden tensör kurmak (15.2): öğe cinsi veriden çıkar. Bütün öğeleri tam sayı (ya da aralık)
// olan veri TR_Int ("int64"), bütün öğeleri mantık değeri olan veri TR_Bool ("bool"), en az bir
// ondalık sayı içeren sayı verisi TR_Float'tır ("float32"; tam sayılar en yakına yuvarlanır).
// Boş liste TR_Float'tır. Mantık değeri sayıyla karışmaz. Tam sayılar float32'den geçmez.

// dataLeaves düzleştirilmiş veridir: yapraklar sırayla ve şekil (tek değerde boş).
type dataLeaves struct {
	leaves []interp.Value
	shape  []int
}

// readData veriyi (sayı, mantık değeri, aralık ya da iç içe liste) yapraklarına açar; düzensiz
// liste hatadır.
func readData(fn any, v interp.Value) (dataLeaves, error) {
	v, err := expandRanges(fn, v)
	if err != nil {
		return dataLeaves{}, err
	}
	l, isList := v.(*interp.ListVal)
	if !isList {
		switch v.(type) {
		case *interp.IntVal, *interp.FloatVal, *interp.BoolVal:
			return dataLeaves{leaves: []interp.Value{v}}, nil
		}
		_, err := toTensor(fn, v) // tensöre çevrilmeyen değerin iletisi (H6032, H6033)
		return dataLeaves{}, err
	}
	var d dataLeaves
	d.shape, err = walkList(fn, l, func(n int) { d.leaves = make([]interp.Value, 0, n) }, func(x interp.Value) error {
		d.leaves = append(d.leaves, x)
		return nil
	})
	return d, err
}

// position i. yaprağın iletideki konumu ([1][0]); tek değerde boş.
func (d dataLeaves) position(i int) string {
	if len(d.shape) == 0 {
		return ""
	}
	return positionText(i, d.shape)
}

// kind verinin öğe cinsi (TR_Float, TR_Int ya da TR_Bool'un kodu). Sayı ya da mantık değeri
// olmayan öğe ve sayıyla karışan mantık değeri hatadır; ileti ilk böyle öğeyi söyler.
func (d dataLeaves) kind(fn any) (codes.Code, error) {
	kind := codes.TypeFloat
	if len(d.leaves) == 0 {
		return kind, nil
	}
	_, firstBool := d.leaves[0].(*interp.BoolVal)
	ints := true
	for i, x := range d.leaves {
		switch x.(type) {
		case *interp.IntVal:
		case *interp.FloatVal:
			ints = false
		case *interp.BoolVal:
		default:
			return 0, messages.New(messages.H6007, fn, interp.Shown(x), typeName(x))
		}
		if _, b := x.(*interp.BoolVal); b != firstBool {
			return 0, messages.New(messages.H6975, fn, d.position(i), interp.Shown(x), interp.Shown(d.leaves[0]))
		}
	}
	switch {
	case firstBool:
		kind = codes.TypeBool
	case ints:
		kind = codes.TypeInt
	}
	return kind, nil
}

// exact TR_Int ya da TR_Bool verisini tensöre çevirir (kind'ın varsayılan hassasiyetinde);
// öğeler doğrudan okunur. Başka cinsten öğe hatadır (H6974).
func (d dataLeaves) exact(fn any, kind codes.Code) (*tensor.Tensor, error) {
	dt := kindDTypes[kind]
	data := make([]int64, len(d.leaves))
	for i, x := range d.leaves {
		switch e := x.(type) {
		case *interp.IntVal:
			if kind == codes.TypeInt {
				data[i] = e.Val
				continue
			}
		case *interp.BoolVal:
			if kind == codes.TypeBool {
				data[i] = b2i(e.Val)
				continue
			}
		}
		return nil, messages.New(messages.H6974, fn, d.position(i), interp.Shown(x), codes.NameRef(kind))
	}
	return safe(func() *tensor.Tensor { return tensor.Int64sOn(tensor.DefaultDevice(), dt, data, d.shape...) })
}

// dataTensor veriden tensör kurar; cins veriden çıkar (kind döner). Ondalık veri float32'dir.
func dataTensor(fn any, v interp.Value) (*tensor.Tensor, codes.Code, error) {
	d, err := readData(fn, v)
	if err != nil {
		return nil, 0, err
	}
	kind, err := d.kind(fn)
	if err != nil {
		return nil, 0, err
	}
	if kind != codes.TypeFloat {
		t, err := d.exact(fn, kind)
		return t, kind, err
	}
	t, err := d.float()
	return t, kind, err
}

// float sayı verisini float32 tensöre çevirir (tam sayılar en yakına yuvarlanır).
func (d dataLeaves) float() (*tensor.Tensor, error) {
	data := make([]float32, len(d.leaves))
	for i, x := range d.leaves {
		f, _ := interp.ToFloat(x)
		data[i] = float32(f)
	}
	return safe(func() *tensor.Tensor { return tensor.New(data, d.shape...) })
}

// expandRanges verideki aralıkları tam sayı listesine açar (TR_Tensor(0..n), [0..2, 2..4]); aralık
// yoksa v'yi olduğu gibi verir.
func expandRanges(fn any, v interp.Value) (interp.Value, error) { return expandRangesAt(fn, v, 0) }

// expandRangesAt expandRanges'tir; derinlik sınırının altındaki liste olduğu gibi kalır (kendini
// içeren ya da çok derin listenin iletisini walkList verir).
func expandRangesAt(fn any, v interp.Value, depth int) (interp.Value, error) {
	switch x := v.(type) {
	case *interp.RangeVal:
		end := x.End
		if x.Inclusive {
			end++
		}
		n := max(end-x.Start, 0)
		if n > interp.MaxCollectionSize {
			return nil, messages.New(messages.H6022, fn, interp.MaxCollectionSize)
		}
		out := make([]interp.Value, n)
		for i := range out {
			out[i] = interp.NewInt(x.Start + int64(i))
		}
		return &interp.ListVal{Elements: out}, nil
	case *interp.ListVal:
		if depth > interp.MaxExternalDepth {
			return v, nil
		}
		var out []interp.Value
		for i, e := range x.Elements {
			r, err := expandRangesAt(fn, e, depth+1)
			if err != nil {
				return nil, err
			}
			if r != e && out == nil {
				out = append(make([]interp.Value, 0, len(x.Elements)), x.Elements[:i]...)
			}
			if out != nil {
				out = append(out, r)
			}
		}
		if out != nil {
			return &interp.ListVal{Elements: out}, nil
		}
	}
	return v, nil
}
