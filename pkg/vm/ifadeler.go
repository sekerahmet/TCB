package vm

import (
	"fmt"
	"tr_dili/pkg/ast"
)

// Bu dosyada derlenmiş kodun değer düzeyinde kullandığı yardımcılar vardır (indeksleme,
// üye erişimi, liste üreteci kaynakları).

// uretecOgeleri bir liste üretecinin dolaşacağı öğeleri döndürür.
func uretecOgeleri(iterVal Value, line int) ([]Value, error) {
	var items []Value
	switch it := iterVal.(type) {
	case *RangeVal:
		end := it.End
		if it.Inclusive {
			end++
		}
		step := int64(1)
		if it.Start > it.End {
			step = -1
			if it.Inclusive {
				end = it.End - 1
			} else {
				end = it.End
			}
		}
		for i := it.Start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			items = append(items, NewInt(i))
		}
	case *ListVal:
		items = it.Elements
	case *MatrixVal:
		for i := 0; i < it.Rows; i++ {
			row := make([]Value, it.Cols)
			for j := 0; j < it.Cols; j++ {
				row[j] = &FloatVal{Val: it.Data[i][j]}
			}
			items = append(items, &ListVal{Elements: row})
		}
	case *DictVal:
		for _, k := range it.Order {
			items = append(items, it.KeyValue(k))
		}
	case *StringVal:
		for _, ch := range []rune(it.Val) {
			items = append(items, &StringVal{Val: string(ch)})
		}
	default:
		return nil, fmt.Errorf("Satır %d: Liste üretecinde %s tipi üzerinde döngü kurulamaz", line, iterVal.Type())
	}
	return items, nil
}

// indeksDegeri kap[idx] değerini döndürür.
func (v *VM) indeksDegeri(left Value, idx Value, line int) (Value, error) {
	switch container := left.(type) {
	case *ListVal:
		if iv, ok := idx.(*IntVal); ok {
			pos := int(iv.Val)
			if pos < 0 {
				pos = len(container.Elements) + pos
			}
			if pos < 0 || pos >= len(container.Elements) {
				return NULL, nil
			}
			return container.Elements[pos], nil
		}
		// Dilimleme (Slicing): liste[1..4]
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(container.Elements) {
				end = len(container.Elements)
			}
			if start > end {
				return &ListVal{Elements: []Value{}}, nil
			}
			sliced := make([]Value, end-start)
			copy(sliced, container.Elements[start:end])
			return &ListVal{Elements: sliced}, nil
		}
		return nil, fmt.Errorf("Satır %d: Liste indeksi Tam sayı olmalıdır", line)

	case *MatrixVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Matris indeksi Tam sayı olmalıdır", line)
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = container.Rows + pos
		}
		if pos < 0 || pos >= container.Rows {
			return NULL, nil
		}
		rowElems := make([]Value, container.Cols)
		for j := 0; j < container.Cols; j++ {
			rowElems[j] = &FloatVal{Val: container.Data[pos][j]}
		}
		return &ListVal{Elements: rowElems}, nil

	case *DictVal:
		val, exists := container.GetValue(idx)
		if !exists {
			return NULL, nil
		}
		return val, nil

	case *StringVal:
		runes := []rune(container.Val)
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start > end {
				return &StringVal{Val: ""}, nil
			}
			return &StringVal{Val: string(runes[start:end])}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Metin indeksi Tam sayı olmalıdır", line)
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(runes) + pos
		}
		if pos < 0 || pos >= len(runes) {
			return NULL, nil
		}
		return &StringVal{Val: string(runes[pos])}, nil
	}

	return nil, fmt.Errorf("Satır %d: %s tipi indekslenemez", line, left.Type())
}

// uyeDegeri nesne.özellik (ya da nesne?.özellik) değerini döndürür.
func (v *VM) uyeDegeri(obj Value, e *ast.MemberExpression) (Value, error) {
	if obj.Type() == VAL_NULL {
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: 'yok' (null) değerin %q özelliğine erişilemez ('?.' güvenli erişim kullanabilirsiniz)", e.Line(), e.Property)
	}

	// Matris, Tensör, SinirAğı, WebSunucu, GörselArayüz ve Veritabanı özellikleri
	if pg, ok := obj.(PropertyGetter); ok {
		return pg.GetProperty(e.Property)
	}

	switch inst := obj.(type) {
	case *StructInstanceVal:
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
				kod:        method.kod,
			}, nil
		}
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: %s yapısında %q alanı veya metodu bulunamadı", e.Line(), inst.Def.Name, e.Property)

	case *DictVal, *ListVal, *StringVal:
		return v.evalBuiltinMember(obj, e.Property, e.Safe, e.Line())
	}

	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği bulunamadı", e.Line(), obj.Type(), e.Property)
}
