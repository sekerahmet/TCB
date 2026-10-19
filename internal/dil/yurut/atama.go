package yurut

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

// indekseAta kap[idx] = değer atamasını yapar.
func (v *VM) indekseAta(left Value, idx Value, kod uint8, rightVal Value, line int) error {
	switch container := left.(type) {
	case *ListVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return fmt.Errorf("Satır %d: %s indeksi %s sayı olmalıdır", line, VAL_LIST, VAL_INT)
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(container.Elements) + pos
		}
		if pos < 0 || pos >= len(container.Elements) {
			return fmt.Errorf("Satır %d: %s indeksi sınır dışı (%d)", line, VAL_LIST, iv.Val)
		}
		finalVal, err := v.atamaIslemi(kod, container.Elements[pos], rightVal, line)
		if err != nil {
			return err
		}
		container.Elements[pos] = finalVal
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
	case IndeksAtanabilir:
		// Dış değerler (ör. tensör): bileşik atamada mevcut değer Indeks ile okunur.
		finalVal := rightVal
		if kod != opAta {
			okunan, ok := left.(Indekslenebilir)
			if !ok {
				return fmt.Errorf("Satır %d: %s tipinde indeksli bileşik atama yapılamaz", line, left.Type())
			}
			cur, err := okunan.Indeks(idx)
			if err != nil {
				return satirEki(line, err)
			}
			if finalVal, err = v.atamaIslemi(kod, cur, rightVal, line); err != nil {
				return err
			}
		}
		return satirEki(line, container.IndekseAta(idx, finalVal))
	}
	return fmt.Errorf("Satır %d: Bu tipe indeks ile atama yapılamaz (%s)", line, left.Type())
}

// uyeyeAta nesne.alan = değer atamasını yapar; u adın derleme anında çözülmüş kodudur.
func (v *VM) uyeyeAta(obj Value, prop string, u uyeBilgisi, kod uint8, rightVal Value, line int) error {
	switch inst := obj.(type) {
	case *StructInstanceVal:
		// Nesnenin alanları her zaman yapı tanımındakilerdir (7.5): tanımda olmayan alana ve
		// metoda atama yapılamaz.
		cur, alan := inst.Fields[prop]
		if !alan {
			if _, metot := inst.Def.Methods[prop]; metot {
				return fmt.Errorf("Satır %d: '%s' %s yapısının bir metodu; metoda değer atanamaz", line, prop, inst.Def.Name)
			}
			return fmt.Errorf("Satır %d: %s yapısında '%s' alanı yok; yapı tanımına '%s' alanını ekleyin", line, inst.Def.Name, prop, prop)
		}
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
		// d.ad = ..., d["ad"] = ... ile aynıdır; ancak sözlük üyelerinin adları (d.anahtarlar,
		// d.uzunluk) okumada üyeyi verdiği için atamada da anahtar sayılmaz (3.7).
		if sozlukUyesiMi(prop) {
			return fmt.Errorf("Satır %d: '%s' sözlüğün bir üyesi; anahtar için d[%q] yazın", line, prop, prop)
		}
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
	case UyeAtanabilir:
		// Dış değerler (ör. iyileştirici): bileşik atamada mevcut değer GetProperty ile okunur.
		finalVal := rightVal
		if pg, ok := obj.(PropertyGetter); ok && kod != opAta {
			cur, err := pg.GetProperty(u.kod, prop)
			if err != nil {
				return satirEki(line, err)
			}
			if finalVal, err = v.atamaIslemi(kod, cur, rightVal, line); err != nil {
				return err
			}
		}
		return satirEki(line, inst.UyeyeAta(u.kod, prop, finalVal))
	}
	return fmt.Errorf("Satır %d: %s tipinin %q alanına atama yapılamaz", line, obj.Type(), prop)
}
