package vm

import (
	"fmt"
)

// Bu dosyada derlenmiş atamaların değer düzeyindeki yardımcıları vardır. kod opAta ise düz
// atamadır; opTopla/opCikar/opCarp/opBol bileşik atamalardır (+=, -=, *=, /=).

// atamaIslemi bileşik atamada mevcut değerle sağ tarafı birleştirir; düz atamada sağ
// tarafı olduğu gibi döndürür.
func (v *VM) atamaIslemi(kod uint8, cur Value, rhs Value, line int) (Value, error) {
	switch kod {
	case opTopla, opCikar, opCarp, opBol:
		return v.evalInfixKod(kod, atamaIslecMetni(kod), cur, rhs, line)
	}
	return rhs, nil
}

// matrisHucresineAta A[satır][sütun] = değer atamasını yapar.
func (v *VM) matrisHucresineAta(mat *MatrixVal, rIdxVal, cIdxVal Value, kod uint8, rightVal Value, line int) error {
	rInt, ok1 := rIdxVal.(*IntVal)
	cInt, ok2 := cIdxVal.(*IntVal)
	if !ok1 || !ok2 {
		return fmt.Errorf("Satır %d: Matris satır ve sütun indeksleri Tam sayı olmalıdır", line)
	}
	rPos := int(rInt.Val)
	if rPos < 0 {
		rPos = mat.Rows + rPos
	}
	cPos := int(cInt.Val)
	if cPos < 0 {
		cPos = mat.Cols + cPos
	}
	if rPos < 0 || rPos >= mat.Rows || cPos < 0 || cPos >= mat.Cols {
		return fmt.Errorf("Satır %d: Matris indeksi sınır dışı ([%d][%d])", line, rInt.Val, cInt.Val)
	}
	finalVal, err := v.atamaIslemi(kod, &FloatVal{Val: mat.Data[rPos][cPos]}, rightVal, line)
	if err != nil {
		return err
	}
	fv, ok := ToFloat(finalVal)
	if !ok {
		return fmt.Errorf("Satır %d: Matris hücresine sayısal değer atanmalıdır", line)
	}
	mat.Data[rPos][cPos] = fv
	return nil
}

// indekseAta kap[idx] = değer atamasını yapar.
func (v *VM) indekseAta(left Value, idx Value, kod uint8, rightVal Value, line int) error {
	switch container := left.(type) {
	case *ListVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return fmt.Errorf("Satır %d: Liste indeksi Tam sayı olmalıdır", line)
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(container.Elements) + pos
		}
		if pos < 0 || pos >= len(container.Elements) {
			return fmt.Errorf("Satır %d: Liste indeksi sınır dışı (%d)", line, iv.Val)
		}
		finalVal, err := v.atamaIslemi(kod, container.Elements[pos], rightVal, line)
		if err != nil {
			return err
		}
		container.Elements[pos] = finalVal
		return nil
	case *MatrixVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return fmt.Errorf("Satır %d: Matris satır indeksi Tam sayı olmalıdır", line)
		}
		rPos := int(iv.Val)
		if rPos < 0 {
			rPos = container.Rows + rPos
		}
		if rPos < 0 || rPos >= container.Rows {
			return fmt.Errorf("Satır %d: Matris satır indeksi sınır dışı (%d)", line, iv.Val)
		}
		rowList, ok := rightVal.(*ListVal)
		if !ok || len(rowList.Elements) != container.Cols {
			return fmt.Errorf("Satır %d: Matris satır ataması %d elemanlı sayısal Liste bekler", line, container.Cols)
		}
		for j, el := range rowList.Elements {
			fv, ok := ToFloat(el)
			if !ok {
				return fmt.Errorf("Satır %d: Matris elemanları sayısal olmalıdır", line)
			}
			container.Data[rPos][j] = fv
		}
		return nil
	case *DictVal:
		if kod == opAta {
			// Düz atamada mevcut değer gerekmez (atamaIslemi sağ tarafı olduğu gibi verir)
			container.SetValue(idx, rightVal)
			return nil
		}
		cur, _ := container.GetValue(idx)
		if cur == nil {
			cur = NULL
		}
		finalVal, err := v.atamaIslemi(kod, cur, rightVal, line)
		if err != nil {
			return err
		}
		container.SetValue(idx, finalVal)
		return nil
	}
	return fmt.Errorf("Satır %d: Bu tipe indeks ile atama yapılamaz (%s)", line, left.Type())
}

// uyeyeAta nesne.alan = değer atamasını yapar.
func (v *VM) uyeyeAta(obj Value, prop string, kod uint8, rightVal Value, line int) error {
	switch inst := obj.(type) {
	case *StructInstanceVal:
		cur := inst.Fields[prop]
		if cur == nil {
			cur = NULL
		}
		finalVal, err := v.atamaIslemi(kod, cur, rightVal, line)
		if err != nil {
			return err
		}
		inst.Fields[prop] = finalVal
		return nil
	case *DictVal:
		cur := inst.Pairs[prop]
		if cur == nil {
			cur = NULL
		}
		finalVal, err := v.atamaIslemi(kod, cur, rightVal, line)
		if err != nil {
			return err
		}
		inst.Set(prop, finalVal)
		return nil
	}
	return fmt.Errorf("Satır %d: %s tipinin %q alanına atama yapılamaz", line, obj.Type(), prop)
}
