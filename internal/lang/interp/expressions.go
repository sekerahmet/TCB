package interp

import (
	"math"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosyada derlenmiş kodun değer düzeyinde kullandığı yardımcılar vardır (indeksleme,
// üye erişimi, liste üreteci kaynakları).

// comprehensionItems bir liste üretecinin dolaşacağı öğeleri döndürür.
func comprehensionItems(v *VM, iterVal Value, line int) ([]Value, error) {
	var items []Value
	switch it := iterVal.(type) {
	case *RangeVal:
		if first, last, step, empty := rangeSteps(it); !empty {
			// The items are built up front, so the size limit of aralık() applies here too.
			if n := rangeCount(first, last, step); n > MaxCollectionSize {
				return nil, messages.At(line, 0, messages.New(messages.H2462, n, MaxCollectionSize))
			}
			for i := first; ; i += step {
				items = append(items, NewInt(i))
				if i == last {
					break
				}
			}
		}
	case *ListVal:
		items = it.Elements
	case *DictVal:
		for _, k := range it.Order {
			items = append(items, it.KeyValue(k))
		}
	case *StringVal:
		for _, ch := range []rune(it.Val) {
			items = append(items, &StringVal{Val: string(ch)})
		}
	case Iterable:
		// Dış değerler (ör. tensör: ilk eksen boyunca öğeler)
		n, err := it.IterLen()
		if err != nil {
			return nil, lineSuffix(line, err)
		}
		items = make([]Value, n)
		for i := range items {
			if items[i], err = it.IterItem(i); err != nil {
				return nil, lineSuffix(line, err)
			}
		}
	case Stream:
		// Tembel kaynak (ör. sıralı okuyucu): biter ya da liste sınırına ulaşana kadar
		for {
			item, cont, err := it.Next(v)
			if err != nil {
				return nil, lineSuffix(line, err)
			}
			if !cont {
				break
			}
			if len(items) >= MaxCollectionSize {
				return nil, messages.At(line, 0, messages.New(messages.H2463, MaxCollectionSize))
			}
			items = append(items, item)
		}
	default:
		return nil, messages.At(line, 0, messages.New(messages.H2464, ValList, iterVal.Type()))
	}
	return items, nil
}

// indexValue kap[idx] değerini döndürür.
func (v *VM) indexValue(left Value, idx Value, line int) (Value, error) {
	switch container := left.(type) {
	case *ListVal:
		if iv, ok := idx.(*IntVal); ok {
			pos := int(iv.Val)
			if pos < 0 {
				pos = len(container.Elements) + pos
			}
			if pos < 0 || pos >= len(container.Elements) {
				return nil, indexRangeError(line, ValList, iv.Val, len(container.Elements))
			}
			return container.Elements[pos], nil
		}
		// Dilimleme: liste[1..4]; negatif uç sondan sayar (liste[-2..] gibi)
		if rng, ok := idx.(*RangeVal); ok {
			start, end := sliceBounds(rng, len(container.Elements))
			sliced := make([]Value, end-start)
			copy(sliced, container.Elements[start:end])
			return &ListVal{Elements: sliced}, nil
		}
		return nil, messages.At(line, 0, messages.New(messages.H2400, ValList, ValInt))

	case *DictVal:
		if err := DictKeyError(idx); err != nil {
			return nil, messages.At(line, 0, err)
		}
		val, exists := container.GetValue(idx)
		if !exists {
			if k, ok := idx.(*StringVal); ok {
				if err := container.privateNameError(k.Val); err != nil {
					return nil, messages.At(line, 0, err)
				}
			}
			return Null, nil
		}
		return val, nil

	case *StringVal:
		ascii, runes := v.textChars(container)
		if iv, ok := idx.(*IntVal); ok && ascii {
			// ASCII hızlı yolu: karakter = bayt, dönüşüm ve ayırma yok
			n := len(container.Val)
			pos := int(iv.Val)
			if pos < 0 {
				pos = n + pos
			}
			if pos < 0 || pos >= n {
				return nil, indexRangeError(line, ValString, iv.Val, n)
			}
			return asciiChars[container.Val[pos]], nil
		}
		if ascii {
			runes = []rune(container.Val) // ASCII dilimleme: nadir yol
		}
		if rng, ok := idx.(*RangeVal); ok {
			start, end := sliceBounds(rng, len(runes))
			return &StringVal{Val: string(runes[start:end])}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, messages.At(line, 0, messages.New(messages.H2400, ValString, ValInt))
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(runes) + pos
		}
		if pos < 0 || pos >= len(runes) {
			return nil, indexRangeError(line, ValString, iv.Val, len(runes))
		}
		return &StringVal{Val: string(runes[pos])}, nil

	case Indexable:
		val, err := container.Index(idx)
		return val, lineSuffix(line, err)
	}

	return nil, messages.At(line, 0, messages.New(messages.H2465, left.Type()))
}

// memberValue nesne.özellik (ya da nesne?.özellik) değerini döndürür.
// u, adın derleme anında çözülmüş kodudur (bkz. uyeCoz).
func (v *VM) memberValue(obj Value, e *ast.MemberExpression, u memberInfo) (Value, error) {
	// Sık türler arayüz denetimlerinden önce denenir (bu türler 'yok' ya da PropertyGetter
	// değildir; adı "Yok" olan bir yapı eski yoldan geçer).
	switch inst := obj.(type) {
	case *StructInstanceVal:
		if ValueType(inst.Def.Name) != ValNull {
			return structMember(inst, e)
		}
	case *DictVal, *ListVal, *StringVal:
		return v.evalBuiltinMember(obj, e.Property, u, e.Safe, e.Line())
	}

	if obj.Type() == ValNull {
		if e.Safe {
			return Null, nil
		}
		return nil, messages.At(e.Line(), 0, messages.New(messages.H2466, codes.DisplayName(codes.A042), e.Property))
	}

	// Tensör, iyileştirici, veri kümesi ve parçalayıcı özellikleri
	if pg, ok := obj.(PropertyGetter); ok {
		var val Value
		var err error
		if g, ok := pg.(TaskMemberHook); ok {
			val, err = g.TaskMember(v, u.codeVal, e.Property)
		} else {
			val, err = pg.GetProperty(u.codeVal, e.Property)
		}
		if err != nil && u.codeVal == codes.None {
			if h := movedMemberError(obj, e.Property); h != "" {
				err = messages.New(messages.H2441, h)
			} else {
				err = memberSuggestionError(pg, obj, e.Property, err)
			}
		}
		return val, lineSuffix(e.Line(), err)
	}

	if inst, ok := obj.(*StructInstanceVal); ok {
		return structMember(inst, e)
	}
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2467, obj.Type(), e.Property))
}

// structMember nesnenin alanını ya da (bağlı fonksiyon olarak) metodunu döndürür.
func structMember(inst *StructInstanceVal, e *ast.MemberExpression) (Value, error) {
	if val, ok := inst.Fields[e.Property]; ok {
		return val, nil
	}
	if method, ok := inst.Def.Methods[e.Property]; ok {
		return &FuncVal{
			Name:       method.Name,
			Parameters: method.Parameters,
			Body:       method.Body,
			Env:        method.Env,
			BoundSelf:  inst,
			codeVal:    method.codeVal,
		}, nil
	}
	if e.Safe {
		return Null, nil
	}
	return nil, messages.At(e.Line(), 0, messages.New(messages.H2468, inst.Def.Name, e.Property))
}

// asciiChars tek karakterlik ASCII metinlerdir (indekslemede yeniden kullanılır;
// metin değerleri değişmezdir).
var asciiChars = func() (t [128]*StringVal) {
	for i := range t {
		t[i] = &StringVal{Val: string(rune(i))}
	}
	return t
}()

// textChars metnin ASCII olup olmadığını ve (değilse) karakter dizisini verir. Son
// indekslenen metnin sonucu saklanır: döngüde aynı metne art arda erişim her seferinde
// []rune dönüşümü yapmaz. Yalnızca yorumlayıcı kilidi tutulurken çağrılır.
func (v *VM) textChars(s *StringVal) (bool, []rune) {
	o := &v.stringCache
	if o.text != s {
		ascii := true
		for i := 0; i < len(s.Val); i++ {
			if s.Val[i] >= 0x80 {
				ascii = false
				break
			}
		}
		o.text, o.ascii, o.runes = s, ascii, nil
		if !ascii {
			o.runes = []rune(s.Val)
		}
	}
	return o.ascii, o.runes
}

// rangeCount is the number of items from first to last (inclusive) by step, computed without
// overflow; it saturates at math.MaxInt64.
func rangeCount(first, last, step int64) int64 {
	span, by := uint64(last)-uint64(first), uint64(step)
	if step < 0 {
		span, by = uint64(first)-uint64(last), uint64(-step)
	}
	n := span/by + 1
	if n > math.MaxInt64 || n == 0 {
		return math.MaxInt64
	}
	return int64(n)
}

// indexRangeError köşeli parantezle sınır dışı okumanın hatasıdır (3.6): sessizce yok vermek
// program yanlışını saklardı. Öğenin olmayabileceği bilinçli durum için varsa_al'ı önerir.
func indexRangeError(line int, container ValueType, index int64, length int) error {
	return messages.At(line, 0, messages.New(messages.H2515, container, index, length, "."+codes.DisplayName(codes.Y013)))
}
