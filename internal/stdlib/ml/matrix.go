package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bu dosya matris biçimli yerleşikleri (birim_matris, sıfırlar(satır, sütun), birler(satır,
// sütun), devrik) uygular. Dilde ayrı bir matris türü yoktur: matris 2 boyutlu bir
// tensördür ve tensör([[1, 2], [3, 4]]) ile oluşturulur (rastgele olanı rastgele_normal([2,
// 3]) ile); @, + - * /, relu, softmax ... tensör yolundan geçer.

// toMatrix matris alan işlevlerin girdisini 2
// boyutlu tensöre çevirir: iç içe sayı listesi [satır, sütun], düz sayı listesi [1, n]
// (satır vektörü) olur. 2 boyutlu tensör olduğu gibi, 1 boyutlu tensör [1, n] görünümüyle
// kullanılır.
func toMatrix(v interp.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *TensorValue:
		switch x.t.Dim() {
		case 2:
			return x.t, nil
		case 1:
			return safe(func() *tensor.Tensor { return x.t.Unsqueeze(0) })
		}
		return nil, messages.New(messages.H6000, shapeText(x.t.Shape()))
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			break
		}
		lines := x.Elements
		if _, ok := x.Elements[0].(*interp.ListVal); !ok {
			lines = []interp.Value{x} // düz liste: tek satır
		}
		column := len(lines[0].(*interp.ListVal).Elements)
		data := make([]float32, 0, len(lines)*column)
		for _, s := range lines {
			line, ok := s.(*interp.ListVal)
			if !ok || len(line.Elements) != column {
				return nil, messages.New(messages.H6003)
			}
			for _, e := range line.Elements {
				f, ok := interp.ToFloat(e)
				if !ok {
					return nil, messages.New(messages.H6004)
				}
				data = append(data, float32(f))
			}
		}
		return tensor.New(data, len(lines), column), nil
	}
	return nil, messages.New(messages.H6005, interp.ValList)
}

// checkMatrixShape negatif / sıfır ya da aşırı büyük matris boyutlarını panik yerine
// hata olarak bildirir.
func checkMatrixShape(fn string, line, column int64) error {
	if line <= 0 || column <= 0 {
		return messages.New(messages.H6009, fn, line, column)
	}
	if line > interp.MaxCollectionSize || column > interp.MaxCollectionSize || line*column > interp.MaxCollectionSize {
		return messages.New(messages.H6010, fn, line, column, interp.MaxCollectionSize)
	}
	return nil
}

// intArg i. argümanın Tam sayı olmasını bekler.
func intArg(args []interp.Value, i int, signature string) (int64, error) {
	n, ok := args[i].(*interp.IntVal)
	if !ok {
		return 0, messages.New(messages.H6011, signature, interp.ValInt, typeName(args[i]))
	}
	return n.Val, nil
}

// identity birim_matris(n) yerleşiğidir: n×n birim matris.
func identity(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T111)
	if len(args) != 1 {
		return nil, messages.New(messages.H6014, fn)
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	n, err := intArg(args, 0, fn+"(n)")
	if err != nil {
		return nil, err
	}
	if err := checkMatrixShape(fn, n, n); err != nil {
		return nil, err
	}
	data := make([]float32, n*n)
	for i := int64(0); i < n; i++ {
		data[i*n+i] = 1
	}
	return New(tensor.New(data, int(n), int(n))), nil
}
