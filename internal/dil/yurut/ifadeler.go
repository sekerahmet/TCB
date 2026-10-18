package yurut

import (
	"fmt"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
)

// Bu dosyada derlenmiş kodun değer düzeyinde kullandığı yardımcılar vardır (indeksleme,
// üye erişimi, liste üreteci kaynakları).

// uretecOgeleri bir liste üretecinin dolaşacağı öğeleri döndürür.
func uretecOgeleri(v *VM, iterVal Value, line int) ([]Value, error) {
	var items []Value
	switch it := iterVal.(type) {
	case *RangeVal:
		if ilk, son, adim, bos := aralikAdimlari(it); !bos {
			for i := ilk; ; i += adim {
				items = append(items, NewInt(i))
				if i == son {
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
	case Yinelenebilir:
		// Dış değerler (ör. tensör: ilk eksen boyunca öğeler)
		n, err := it.YinelemeUzunlugu()
		if err != nil {
			return nil, satirEki(line, err)
		}
		items = make([]Value, n)
		for i := range items {
			if items[i], err = it.YinelemeOgesi(i); err != nil {
				return nil, satirEki(line, err)
			}
		}
	case Akis:
		// Tembel kaynak (ör. sıralı okuyucu): biter ya da liste sınırına ulaşana kadar
		for {
			item, devam, err := it.Sonraki(v)
			if err != nil {
				return nil, satirEki(line, err)
			}
			if !devam {
				break
			}
			if len(items) >= MaxCollectionSize {
				return nil, fmt.Errorf("Satır %d: liste üreteci en fazla %d öğe toplayabilir; akışı döngüyle gezin", line, MaxCollectionSize)
			}
			items = append(items, item)
		}
	default:
		return nil, fmt.Errorf("Satır %d: %s üretecinde %s tipi üzerinde döngü kurulamaz", line, VAL_LIST, iterVal.Type())
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
		// Dilimleme: liste[1..4]; negatif uç sondan sayar (liste[-2..] gibi)
		if rng, ok := idx.(*RangeVal); ok {
			start, end := dilimUclari(rng, len(container.Elements))
			sliced := make([]Value, end-start)
			copy(sliced, container.Elements[start:end])
			return &ListVal{Elements: sliced}, nil
		}
		return nil, fmt.Errorf("Satır %d: %s indeksi %s sayı olmalıdır", line, VAL_LIST, VAL_INT)

	case *DictVal:
		val, exists := container.GetValue(idx)
		if !exists {
			return NULL, nil
		}
		return val, nil

	case *StringVal:
		ascii, runes := v.metinKarakterleri(container)
		if iv, ok := idx.(*IntVal); ok && ascii {
			// ASCII hızlı yolu: karakter = bayt, dönüşüm ve ayırma yok
			n := len(container.Val)
			pos := int(iv.Val)
			if pos < 0 {
				pos = n + pos
			}
			if pos < 0 || pos >= n {
				return NULL, nil
			}
			return asciiKarakterler[container.Val[pos]], nil
		}
		if ascii {
			runes = []rune(container.Val) // ASCII dilimleme: nadir yol
		}
		if rng, ok := idx.(*RangeVal); ok {
			start, end := dilimUclari(rng, len(runes))
			return &StringVal{Val: string(runes[start:end])}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: %s indeksi %s sayı olmalıdır", line, VAL_STRING, VAL_INT)
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(runes) + pos
		}
		if pos < 0 || pos >= len(runes) {
			return NULL, nil
		}
		return &StringVal{Val: string(runes[pos])}, nil

	case Indekslenebilir:
		val, err := container.Indeks(idx)
		return val, satirEki(line, err)
	}

	return nil, fmt.Errorf("Satır %d: %s tipi indekslenemez", line, left.Type())
}

// uyeDegeri nesne.özellik (ya da nesne?.özellik) değerini döndürür.
// u, adın derleme anında çözülmüş kodudur (bkz. uyeCoz).
func (v *VM) uyeDegeri(obj Value, e *agac.MemberExpression, u uyeBilgisi) (Value, error) {
	// Sık türler arayüz denetimlerinden önce denenir (bu türler 'yok' ya da PropertyGetter
	// değildir; adı "Yok" olan bir yapı eski yoldan geçer).
	switch inst := obj.(type) {
	case *StructInstanceVal:
		if ValueType(inst.Def.Name) != VAL_NULL {
			return yapiUyesi(inst, e)
		}
	case *DictVal, *ListVal, *StringVal:
		return v.evalBuiltinMember(obj, e.Property, u, e.Safe, e.Line())
	}

	if obj.Type() == VAL_NULL {
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: '%s' değerin %q özelliğine erişilemez ('?.' güvenli erişim kullanabilirsiniz)", e.Line(), kod.Ad(kod.A042), e.Property)
	}

	// Tensör, iyileştirici, veri kümesi ve parçalayıcı özellikleri
	if pg, ok := obj.(PropertyGetter); ok {
		val, err := pg.GetProperty(u.kod, e.Property)
		if err != nil && u.kod == kod.Yok {
			if h := tasinanUyeHatasi(obj, e.Property); h != "" {
				err = fmt.Errorf("%s", h)
			} else {
				err = uyeOneriHatasi(pg, obj, e.Property, err)
			}
		}
		return val, satirEki(e.Line(), err)
	}

	if inst, ok := obj.(*StructInstanceVal); ok {
		return yapiUyesi(inst, e)
	}
	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği bulunamadı", e.Line(), obj.Type(), e.Property)
}

// yapiUyesi nesnenin alanını ya da (bağlı fonksiyon olarak) metodunu döndürür.
func yapiUyesi(inst *StructInstanceVal, e *agac.MemberExpression) (Value, error) {
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
}

// asciiKarakterler tek karakterlik ASCII metinlerdir (indekslemede yeniden kullanılır;
// metin değerleri değişmezdir).
var asciiKarakterler = func() (t [128]*StringVal) {
	for i := range t {
		t[i] = &StringVal{Val: string(rune(i))}
	}
	return t
}()

// metinKarakterleri metnin ASCII olup olmadığını ve (değilse) karakter dizisini verir. Son
// indekslenen metnin sonucu saklanır: döngüde aynı metne art arda erişim her seferinde
// []rune dönüşümü yapmaz. Yalnızca yorumlayıcı kilidi tutulurken çağrılır.
func (v *VM) metinKarakterleri(s *StringVal) (bool, []rune) {
	o := &v.metinOnbellegi
	if o.metin != s {
		ascii := true
		for i := 0; i < len(s.Val); i++ {
			if s.Val[i] >= 0x80 {
				ascii = false
				break
			}
		}
		o.metin, o.ascii, o.runes = s, ascii, nil
		if !ascii {
			o.runes = []rune(s.Val)
		}
	}
	return o.ascii, o.runes
}
