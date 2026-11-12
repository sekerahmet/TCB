package ml

import (
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Cinsli tensör işleminin öğeye bağlı hatası (tensor.ElemFault) → ileti (15.3, 15.6). İleti
// ilk hatalı öğenin konumunu ([i][j]) ve değerini söyler; 0 boyutlu sonuçta konum yazılmaz.

// faultError f'yi fn'in iletisine çevirir; op aritmetik işlecin yazımıdır ("+", "//", tekli
// eksi için "-", mutlak değer için fn'in kendisi).
func faultError(fn any, op string, f *tensor.ElemFault) error {
	index := ""
	if len(f.Shape) > 0 {
		index = positionText(f.Index, f.Shape)
	}
	expr := strconv.FormatInt(f.A, 10) + " " + op + " " + strconv.FormatInt(f.B, 10)
	switch f.Kind {
	case tensor.FaultOverflow:
		switch {
		case f.Sum != "":
			expr = f.Sum // toplamın işlenenleri yok: kesin değeri yazılır
		case op == "-" && f.B == 0 && f.A != 0 || op == "":
			expr = op + strconv.FormatInt(f.A, 10)
		}
		wide := "no"
		if f.DType == tensor.Int64 {
			wide = "yes"
		}
		return messages.New(messages.H6961, fn, index, expr, f.DType.Name(), wide)
	case tensor.FaultDivZero:
		return messages.New(messages.H6962, fn, index, expr)
	case tensor.FaultNegExp:
		return messages.New(messages.H6963, fn, index, expr)
	case tensor.FaultShift:
		return messages.New(messages.H6965, fn, index, f.Int, f.DType.Name(), f.DType.Bits()-1)
	}
	kind, value := "other", strconv.FormatInt(f.Int, 10)
	switch f.Kind {
	case tensor.FaultNaN:
		kind, value = "notnumber", "NaN"
	case tensor.FaultInf:
		kind, value = "infinite", strconv.FormatFloat(f.Float, 'g', -1, 64)
	default:
		if f.Float != float64(f.Int) {
			value = strconv.FormatFloat(f.Float, 'g', -1, 64)
		}
	}
	return messages.New(messages.H6964, fn, index, value, f.DType.Name(), kind)
}

// typedResult cinsli işlemin sonucunu sarar ya da hatasını iletiye çevirir.
func typedResult(m *interp.VM, fn any, op string, r *tensor.Tensor, f *tensor.ElemFault) (*TensorValue, error) {
	switch {
	case f == nil:
		return New(r), nil
	case f.Kind == tensor.FaultRecorded:
		// Kayıtta kartın hatası beklenmez: yuvanın yeri (satır, ileti) yazılır, ileti
		// oynatmanın sonunda (replay_faults.go); m nil ise yer yazılamaz (H4065).
		if err := recordFault(m, f, func(f *tensor.ElemFault) error { return faultError(fn, op, f) }); err != nil {
			r.Drop()
			return nil, err
		}
		return New(r), nil
	}
	return nil, faultError(fn, op, f)
}

// kindMismatch iki yanı başka cinsten olan işlemin iletisidir (H6966). İleti mantıktan sayı
// kurmayı önerir; öbür yan tam sayıysa öneri tam sayı verir (`TR_where(m, 1, 0)`), ondalık
// öneri aynı hatayı yeniden verirdi.
func kindMismatch(op any, left, right tensor.DType) error {
	branch := "other"
	if left.Elem() == tensor.ElemBool && right.Elem() == tensor.ElemInt ||
		left.Elem() == tensor.ElemInt && right.Elem() == tensor.ElemBool {
		branch = "int"
	}
	return messages.New(messages.H6966, op, left.Name(), right.Name(), branch)
}
