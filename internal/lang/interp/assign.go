package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosyada derlenmiş atamaların değer düzeyindeki yardımcıları vardır. kod opAssign ise düz
// atamadır; öbür kodlar bileşik atamanın ikili işlecidir (+=, //=, <<= ...; 5.4).

// assignOp bileşik atamada mevcut değerle sağ tarafı birleştirir; düz atamada sağ
// tarafı olduğu gibi döndürür.
func (v *VM) assignOp(codeVal uint8, cur Value, rhs Value, line int) (Value, error) {
	if codeVal == opAssign {
		return rhs, nil
	}
	if done, err := v.inPlaceHook(codeVal, cur, rhs, line); done {
		if err != nil {
			return nil, err
		}
		return cur, nil
	}
	if res, done, err := v.listInPlace(codeVal, cur, rhs, line); done {
		return res, err
	}
	return v.infix(codeVal, assignOpText(codeVal), cur, rhs, line)
}

// inPlaceHook cur InPlaceOperator ise bileşik atamayı ona verir (ör. tensörde yerinde t += d);
// done, değerin atamayı kendisinin yaptığıdır: cur aynı nesne kalır, bağ değişmez. Hem assignOp
// hem yerel ve genel yuvaların hızlı yolu (compiler_assign.go) bunu çağırır.
func (v *VM) inPlaceHook(codeVal uint8, cur Value, rhs Value, line int) (bool, error) {
	h, ok := cur.(InPlaceOperator)
	if !ok {
		return false, nil
	}
	done, err := h.InPlaceOp(assignOpText(codeVal), rhs)
	if !done {
		return false, nil
	}
	if err != nil {
		return true, lineSuffix(line, err)
	}
	v.releaseTemp(rhs, cur)
	return true, nil
}

// assignInPlace bileşik atamayı yerinde yapmayı dener: önce değerin kendi kancası
// (InPlaceOperator), sonra listede += ve *=. done, atamanın yapıldığı (ya da hata verdiği)dır.
func (v *VM) assignInPlace(codeVal uint8, cur Value, rhs Value, line int) (bool, error) {
	if done, err := v.inPlaceHook(codeVal, cur, rhs, line); done {
		return true, err
	}
	_, done, err := v.listInPlace(codeVal, cur, rhs, line)
	return done, err
}

// listInPlace listede += ve *= atamasını yerinde yapar (5.4): sonuç '+' ve '*'nin kurallarıyla (4.10:
// yinelenemeyen öğe, boy sınırı) kurulur, öğeleri aynı listeye yazılır; listeyi paylaşan her ad
// değişikliği görür. Bağ değişmediği için sabit listede de geçerlidir. done, atamanın bir liste
// ataması olduğudur (yapıldı ya da hata verdi); değilse çağıran genel yolla sürer.
func (v *VM) listInPlace(codeVal uint8, cur, rhs Value, line int) (res Value, done bool, err error) {
	l, ok := cur.(*ListVal)
	if !ok || codeVal != opAdd && codeVal != opMul {
		return nil, false, nil
	}
	fresh, err := v.infix(codeVal, assignOpText(codeVal), cur, rhs, line)
	if err != nil {
		return nil, true, err
	}
	nl, ok := fresh.(*ListVal)
	if !ok {
		return fresh, true, nil
	}
	// Değişen liste ve ona giren öğeler kaçar (ownership.go; setIndex gibi).
	escape(l)
	for _, e := range nl.Elements {
		escape(e)
	}
	l.Elements = nl.Elements
	return l, true, nil
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
		cur, ok := container.GetValue(idx)
		if !ok {
			return messages.At(line, 0, missingKeyError(idx))
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
			if acc, ok := left.(IndexAccumulator); ok {
				if done, err := acc.AccumulateIndex(assignOpText(codeVal), idx, rightVal); done {
					return lineSuffix(line, err)
				}
			}
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
		return v.setStructField(inst, prop, codeVal, rightVal, line)
	case *DictVal:
		// d.ad = ..., d["ad"] = ... ile aynıdır; ancak sözlük üyelerinin adları (d.anahtarlar,
		// d.uzunluk) okumada üyeyi verdiği için atamada da anahtar sayılmaz (3.7).
		if isDictMember(prop) {
			return messages.At(line, 0, messages.New(messages.H2406, prop))
		}
		if inst.module == nil {
			return messages.At(line, 0, messages.New(messages.H2648, u.keyName(prop)))
		}
		key, _ := u.moduleKey(inst, prop)
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
