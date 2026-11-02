package ml

import (
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
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
		if op == "-" && f.B == 0 && f.A != 0 || op == "" {
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
func typedResult(fn any, op string, r *tensor.Tensor, f *tensor.ElemFault) (*TensorValue, error) {
	if f != nil {
		return nil, faultError(fn, op, f)
	}
	return New(r), nil
}
