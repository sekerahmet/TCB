package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosyada derlenmiş atamaların değer düzeyindeki yardımcıları vardır. kod opAta ise düz
// atamadır; opTopla/opCikar/opCarp/opBol bileşik atamalardır (+=, -=, *=, /=).

// assignOp bileşik atamada mevcut değerle sağ tarafı birleştirir; düz atamada sağ
// tarafı olduğu gibi döndürür.
func (v *VM) assignOp(codeVal uint8, cur Value, rhs Value, line int) (Value, error) {
	switch codeVal {
	case opAdd, opSub, opMul, opDiv:
		return v.infix(codeVal, assignOpText(codeVal), cur, rhs, line)
	}
	return rhs, nil
}

// setIndex kap[idx] = değer atamasını yapar.
func (v *VM) setIndex(left Value, idx Value, codeVal uint8, rightVal Value, line int) error {
	if _, copies := left.(AssignCopier); !copies {
		// Kaba yazılan değer kaçar; değişen sahip liste de (ownership.go).
		escape(left)
		escape(rightVal)
	}
	switch container := left.(type) {
	case *ListVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return messages.At(line, 0, messages.New(messages.H2400, ValList, ValInt))
		}
		pos, inRange := resolveIndex(iv.Val, len(container.Elements))
		if !inRange {
			return messages.At(line, 0, messages.New(messages.H2401, ValList, iv.Val))
		}
		finalVal, err := v.assignOp(codeVal, container.Elements[pos], rightVal, line)
		if err != nil {
			return err
		}
		escape(finalVal)
		container.Elements[pos] = finalVal
		return nil
	case *DictVal:
		if err := DictKeyError(idx); err != nil {
			return messages.At(line, 0, err)
		}
		if codeVal == opAssign {
			// Düz atamada mevcut değer gerekmez (atamaIslemi sağ tarafı olduğu gibi verir)
			container.SetValue(idx, rightVal)
			return nil
		}
		cur, _ := container.GetValue(idx)
		if cur == nil {
			cur = Null
		}
		finalVal, err := v.assignOp(codeVal, cur, rightVal, line)
		if err != nil {
			return err
		}
		escape(finalVal)
		container.SetValue(idx, finalVal)
		return nil
	case IndexAssignable:
		// Dış değerler (ör. tensör): bileşik atamada mevcut değer Indeks ile okunur.
		finalVal := rightVal
		if codeVal != opAssign {
			readCount, ok := left.(Indexable)
			if !ok {
				return messages.At(line, 0, messages.New(messages.H2402, left.Type()))
			}
			cur, err := readCount.Index(idx)
			if err != nil {
				return lineSuffix(line, err)
			}
			if finalVal, err = v.assignOp(codeVal, cur, rightVal, line); err != nil {
				return err
			}
		}
		return lineSuffix(line, container.SetIndex(idx, finalVal))
	}
	return messages.At(line, 0, messages.New(messages.H2403, left.Type()))
}

// setMember nesne.alan = değer atamasını yapar; u adın derleme anında çözülmüş kodudur.
func (v *VM) setMember(obj Value, prop string, u memberInfo, codeVal uint8, rightVal Value, line int) error {
	escape(rightVal) // alana yazılan değer kaçar (ownership.go)
	switch inst := obj.(type) {
	case *StructInstanceVal:
		// Nesnenin alanları her zaman yapı tanımındakilerdir (7.5): tanımda olmayan alana ve
		// metoda atama yapılamaz.
		cur, field := inst.Fields[prop]
		if !field {
			if _, meth := inst.Def.Methods[prop]; meth {
				return messages.At(line, 0, messages.New(messages.H2404, prop, inst.Def.Name))
			}
			return messages.At(line, 0, messages.New(messages.H2405, inst.Def.Name, prop))
		}
		if cur == nil {
			cur = Null
		}
		finalVal, err := v.assignOp(codeVal, cur, rightVal, line)
		if err != nil {
			return err
		}
		escape(finalVal)
		inst.Fields[prop] = finalVal
		return nil
	case *DictVal:
		// d.ad = ..., d["ad"] = ... ile aynıdır; ancak sözlük üyelerinin adları (d.anahtarlar,
		// d.uzunluk) okumada üyeyi verdiği için atamada da anahtar sayılmaz (3.7).
		if isDictMember(prop) {
			return messages.At(line, 0, messages.New(messages.H2406, prop))
		}
		key, _ := u.dictKey(inst, prop)
		cur := inst.Pairs[key]
		if cur == nil {
			cur = Null
		}
		finalVal, err := v.assignOp(codeVal, cur, rightVal, line)
		if err != nil {
			return err
		}
		escape(finalVal)
		inst.Set(key, finalVal)
		return nil
	case MemberAssignable:
		// Dış değerler (ör. iyileştirici): bileşik atamada mevcut değer GetProperty ile okunur.
		finalVal := rightVal
		if pg, ok := obj.(PropertyGetter); ok && codeVal != opAssign {
			cur, err := pg.GetProperty(u.codeVal, prop)
			if err != nil {
				return lineSuffix(line, err)
			}
			if finalVal, err = v.assignOp(codeVal, cur, rightVal, line); err != nil {
				return err
			}
		}
		return lineSuffix(line, inst.SetMember(u.codeVal, prop, finalVal))
	}
	return messages.At(line, 0, messages.New(messages.H2407, obj.Type(), prop))
}
