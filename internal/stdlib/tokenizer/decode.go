package tokenizer

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// ----------------------------------------------------------------------------
// Çözme (16.3): çöz tek diziyi metne, toplu_çöz dizilerin listesini metin listesine çevirir
// (13.8a: dönüşü argümanın biçimine bağlı tek ad yok).
// ----------------------------------------------------------------------------

func (n *Object) decode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberDecode)
	special, err := decodeArgs(fn, args, named, messages.H5632)
	if err != nil {
		return nil, err
	}
	ids, err := idSeq(fn, args[0])
	if err != nil {
		return nil, err
	}
	result, err := n.decodeSeqs(m, fn, [][]int{ids}, special, true)
	if err != nil {
		return nil, err
	}
	return &interp.StringVal{Val: result[0]}, nil
}

func (n *Object) batchDecode(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberBatchDecode)
	special, err := decodeArgs(fn, args, named, messages.H5644)
	if err != nil {
		return nil, err
	}
	seqs, err := idBatch(fn, args[0])
	if err != nil {
		return nil, err
	}
	result, err := n.decodeSeqs(m, fn, seqs, special, false)
	if err != nil {
		return nil, err
	}
	elems := make([]interp.Value, len(result))
	for i, s := range result {
		elems[i] = &interp.StringVal{Val: s}
	}
	return &interp.ListVal{Elements: elems}, nil
}

// decodeArgs çöz'ün ve toplu_çöz'ün ortak argüman denetimidir; count argüman sayısının iletisidir.
// Varsayılan, başvuru kütüphanesininki gibi özel parçaları ATLAMAKTIR; özel_parçalar: doğru onları
// da çıktıya yazar.
func decodeArgs(fn codes.Ref, args []interp.Value, named map[string]interp.Value, count messages.Code) (bool, error) {
	if err := bind.CheckNamed(fn.Called(), named, nameSpecial); err != nil {
		return false, err
	}
	if len(args) != 1 {
		return false, messages.New(count, fn, len(args))
	}
	return bind.OptionalBool(fn, named, nameSpecial, false)
}

// decodeSeqs dizileri çözer; single doğruysa tek dizidir (hata iletisi dizinin sırasını anmaz).
// Kural (16.3): dağarcıkta olmayan kimlik çalışma hatasıdır (H5647), sessizce atlanmaz; çekirdeğin
// Decode'u (başvuru kütüphanesi gibi) atlar, denetim burada, çözmeden önce.
func (n *Object) decodeSeqs(m *interp.VM, fn codes.Ref, seqs [][]int, special, single bool) ([]string, error) {
	for i, d := range seqs {
		for _, id := range d {
			if _, ok := n.p.IDToToken(id); ok {
				continue
			}
			if single {
				return nil, messages.New(messages.H5647, fn, id, n.p.VocabSize())
			}
			return nil, messages.New(messages.H5647, messages.New(messages.H5633, fn, i+1), id, n.p.VocabSize())
		}
	}
	var result []string
	var errs []error
	job := func() { result, errs = n.p.SafeDecodeBatch(seqs, !special, 0) }
	total := 0
	for _, d := range seqs {
		total += len(d)
	}
	n.maybeRelease(m, total*4, job)
	for i, h := range errs {
		if h != nil {
			if single {
				return nil, stepError(fn, h)
			}
			return nil, stepError(messages.New(messages.H5633, fn, i+1), h)
		}
	}
	return result, nil
}

// idSeq çöz'ün argümanını tek kimlik dizisine çevirir: kimlik listesi ya da 1 boyutlu tensör.
// Listelerin listesi ve 2 boyutlu tensör toplu_çöz'ü önerir (H5642).
func idSeq(fn codes.Ref, v interp.Value) ([]int, error) {
	switch x := v.(type) {
	case *interp.ListVal:
		if len(x.Elements) > 0 {
			if _, inner := x.Elements[0].(*interp.ListVal); inner {
				return nil, messages.New(messages.H5642, fn, "list")
			}
		}
		return intList(fn, 0, x.Elements)
	case *ml.TensorValue:
		switch d := len(x.Tensor().Shape()); d {
		case 1:
		case 2:
			return nil, messages.New(messages.H5642, fn, "tensor")
		default:
			return nil, messages.New(messages.H5636, fn, d)
		}
		ids, bad, ok := x.IntElements()
		if !ok {
			return nil, messages.New(messages.H5637, fn, bad)
		}
		return ids, nil
	}
	return nil, messages.New(messages.H5638, fn, v.Type())
}

// idBatch toplu_çöz'ün argümanını kimlik dizilerine çevirir: kimlik listelerinin listesi ya da
// 2 boyutlu tensör (satırları). Tek kimlik listesi ve 1 boyutlu tensör çöz'ü önerir (H5643).
func idBatch(fn codes.Ref, v interp.Value) ([][]int, error) {
	switch x := v.(type) {
	case *interp.ListVal:
		seqs := make([][]int, len(x.Elements))
		for i, e := range x.Elements {
			l, ok := e.(*interp.ListVal)
			if !ok {
				if _, id := bind.Int(e); id && i == 0 {
					return nil, messages.New(messages.H5643, fn, "list")
				}
				return nil, messages.New(messages.H5634, fn, i+1, e.Type())
			}
			var err error
			if seqs[i], err = intList(fn, i+1, l.Elements); err != nil {
				return nil, err
			}
		}
		return seqs, nil
	case *ml.TensorValue:
		sh := x.Tensor().Shape()
		switch len(sh) {
		case 2:
		case 1:
			return nil, messages.New(messages.H5643, fn, "tensor")
		default:
			return nil, messages.New(messages.H5646, fn, len(sh))
		}
		ids, bad, ok := x.IntElements()
		if !ok {
			return nil, messages.New(messages.H5637, fn, bad)
		}
		seqs := make([][]int, sh[0])
		for i := range seqs {
			seqs[i] = ids[i*sh[1] : (i+1)*sh[1]]
		}
		return seqs, nil
	}
	return nil, messages.New(messages.H5645, fn, v.Type())
}

// intList kimlik listesini tam sayılara çevirir; row iç içe listenin 1'den başlayan satırıdır, düz
// listede 0 (iletide "listenin" ya da "3. listenin" diye yazılır, H5639).
func intList(fn codes.Ref, row int, elems []interp.Value) ([]int, error) {
	out := make([]int, len(elems))
	for i, e := range elems {
		n, ok := bind.Int(e)
		if !ok {
			return nil, messages.New(messages.H5639, fn, row, i+1, interp.Shown(e))
		}
		out[i] = int(n)
	}
	return out, nil
}
