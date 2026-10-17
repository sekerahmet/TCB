package vm

import (
	"errors"
	"fmt"
	"tr_dili/pkg/ast"
)

func (v *VM) execAssign(target ast.Expression, op string, rightVal Value, line int, env *Environment) error {
	return v.execAssignKod(target, islecKodu(op), rightVal, line, env)
}

// execAssignKod atamayı işleç koduyla yapar: opAta düz atamadır, opTopla/opCikar/opCarp/opBol
// bileşik atamalardır (+=, -=, *=, /=).
func (v *VM) execAssignKod(target ast.Expression, kod uint8, rightVal Value, line int, env *Environment) error {
	bilesik := kod == opTopla || kod == opCikar || kod == opCarp || kod == opBol
	applyOp := func(cur Value, rhs Value) (Value, error) {
		if !bilesik {
			return rhs, nil
		}
		return v.evalInfixKod(kod, atamaIslecMetni(kod), cur, rhs, line)
	}

	switch t := target.(type) {
	case *ast.Identifier:
		var finalVal Value = rightVal
		if kod != opAta {
			cur, ok := env.Get(t.Value)
			if !ok {
				return fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken yok%s",
					line, t.Token.Column, t.Value, suggestName(t.Value, env))
			}
			var err error
			finalVal, err = applyOp(cur, rightVal)
			if err != nil {
				return err
			}
		}
		if err := env.Assign(t.Value, finalVal); err != nil {
			var tanimsiz *TanimsizDegiskenHatasi
			if errors.As(err, &tanimsiz) {
				return fmt.Errorf("Satır %d, Sütun %d: %v%s", line, t.Token.Column, err, suggestName(t.Value, env))
			}
			return fmt.Errorf("Satır %d: %v", line, err)
		}
		return nil

	case *ast.IndexExpression:
		// 2 boyutlu Matris hücre ataması kontrolü: A[satır][sütun] = değer
		if innerIdx, is2D := t.Left.(*ast.IndexExpression); is2D {
			baseObj, err := v.evalExpression(innerIdx.Left, env)
			if err != nil {
				return err
			}
			if mat, isMat := baseObj.(*MatrixVal); isMat {
				rIdxVal, err := v.evalExpression(innerIdx.Index, env)
				if err != nil {
					return err
				}
				cIdxVal, err := v.evalExpression(t.Index, env)
				if err != nil {
					return err
				}
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
				finalVal, err := applyOp(&FloatVal{Val: mat.Data[rPos][cPos]}, rightVal)
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
		}

		left, err := v.evalExpression(t.Left, env)
		if err != nil {
			return err
		}
		idx, err := v.evalExpression(t.Index, env)
		if err != nil {
			return err
		}
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
			finalVal, err := applyOp(container.Elements[pos], rightVal)
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
			cur, _ := container.GetValue(idx)
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			container.SetValue(idx, finalVal)
			return nil
		}
		return fmt.Errorf("Satır %d: Bu tipe indeks ile atama yapılamaz (%s)", line, left.Type())

	case *ast.MemberExpression:
		obj, err := v.evalExpression(t.Object, env)
		if err != nil {
			return err
		}
		switch inst := obj.(type) {
		case *StructInstanceVal:
			cur := inst.Fields[t.Property]
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			inst.Fields[t.Property] = finalVal
			return nil
		case *DictVal:
			cur := inst.Pairs[t.Property]
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			inst.Set(t.Property, finalVal)
			return nil
		}
		return fmt.Errorf("Satır %d: %s tipinin %q alanına atama yapılamaz", line, obj.Type(), t.Property)
	}
	return fmt.Errorf("Satır %d: Geçersiz atama hedefi", line)
}
