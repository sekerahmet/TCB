package interp

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tip argümanlı çağrı (15.2): TR_Tensor[TR_Int](veri), TR_zeros[TR_Int]([3]), TR_eye[TR_Bool](3).
// Bir tip ya da yerleşik, kendi kodu için kaydedilmiş bir işlevle tip argümanı alır
// (RegisterTypeArgs); x[T] o işlevin verdiği çağrılabilir yerleşiği verir. Kayıtlı olmayan
// kodun indekslenmesi bugünkü gibi H2465'tir: genel tip yazımı (TR_List[TR_JSON], 3.11) bu
// kancayla çözülmez. Dönen değer yalnız çağrılmak içindir; '==' yerleşik gibi hatadır.
//
// Kayıt koda bağlıdır ve süreç genelidir: aynı kod için her VM aynı işlevi kaydeder (kitaplık
// motorları, ör. internal/stdlib/ml).

// TypeArgsFunc x[T]'nin çağrılabilir değerini kurar; args tip argümanlarıdır (bugün tek).
type TypeArgsFunc func(v *VM, args []*TypeVal) (*BuiltinVal, error)

var typeArgs struct {
	mu sync.RWMutex
	m  map[codes.Code]TypeArgsFunc
}

// RegisterTypeArgs k kodlu tipin ya da yerleşiğin tip argümanı almasını fn'e bağlar.
func (v *VM) RegisterTypeArgs(k codes.Code, fn TypeArgsFunc) {
	typeArgs.mu.Lock()
	defer typeArgs.mu.Unlock()
	if typeArgs.m == nil {
		typeArgs.m = map[codes.Code]TypeArgsFunc{}
	}
	typeArgs.m[k] = fn
}

// typeArgsCode x'in tip argümanı alan kayıtlı kodu (tip ya da yerleşik); yoksa ok yanlış.
func typeArgsCode(x Value) (codes.Code, TypeArgsFunc, bool) {
	var k codes.Code
	switch t := x.(type) {
	case *TypeVal:
		if t.Struct != nil || t.Elems != nil {
			return codes.None, nil, false
		}
		k = t.Code
	case *BuiltinVal:
		found, ok := codes.Find(codes.NamespaceGlobal, t.Name)
		if !ok {
			return codes.None, nil, false
		}
		k = found
	default:
		return codes.None, nil, false
	}
	typeArgs.mu.RLock()
	fn := typeArgs.m[k]
	typeArgs.mu.RUnlock()
	return k, fn, fn != nil
}

// typeArgsIndex x[idx]'i, x tip argümanı alan kayıtlı bir tip ya da yerleşikse yapar: idx bir
// tip değeri olmalıdır. ok yanlışsa x kayıtlı değildir (çağıran bugünkü yolu izler).
func (v *VM) typeArgsIndex(x, idx Value) (Value, bool, error) {
	k, fn, ok := typeArgsCode(x)
	if !ok {
		return nil, false, nil
	}
	t, isType := idx.(*TypeVal)
	if !isType {
		return nil, true, messages.New(messages.H6970, codes.NameRef(k), Shown(idx))
	}
	b, err := fn(v, []*TypeVal{t})
	if err != nil {
		return nil, true, err
	}
	return b, true, nil
}
