package vm

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Metin, Liste ve Sözlük üzerinde nokta (.) ile çağrılan yerleşik metotlar. Metot gövdeleri
// nesneyi argüman olarak alan sıradan fonksiyonlardır: derlenmiş kod 'x.metot(...)'
// çağrılarını bağlı bir yerleşik nesnesi ayırmadan doğrudan bunlara yönlendirir;
// 'x.metot' bir değer olarak okunursa (ör. değişkene atanırsa) bağlı yerleşik üretilir.

// metotGovdesi bir yerleşik metodun gövdesidir; line, geri çağrılan fonksiyonların
// hata satırıdır.
type metotGovdesi func(vm *VM, inst Value, args []Value, line int) (Value, error)

type yerlesikMetot struct {
	ad string // bağlı yerleşiğin adı (eş adlar için asıl ad)
	f  metotGovdesi
}

var (
	metinMetotlari  = map[string]yerlesikMetot{}
	listeMetotlari  = map[string]yerlesikMetot{}
	sozlukMetotlari = map[string]yerlesikMetot{}
)

func metotEkle(tablo map[string]yerlesikMetot, f metotGovdesi, adlar ...string) {
	for _, ad := range adlar {
		tablo[ad] = yerlesikMetot{ad: adlar[0], f: f}
	}
}

// yerlesikMetotAdi ad herhangi bir yerleşik metodun adı mı (derleme anı süzgeci)?
func yerlesikMetotAdi(ad string) bool {
	_, a := metinMetotlari[ad]
	_, b := listeMetotlari[ad]
	_, c := sozlukMetotlari[ad]
	return a || b || c
}

// metotBul nesnenin tipine göre metodu döndürür.
func metotBul(obj Value, prop string) (yerlesikMetot, bool) {
	var m yerlesikMetot
	var ok bool
	switch obj.(type) {
	case *StringVal:
		m, ok = metinMetotlari[prop]
	case *ListVal:
		m, ok = listeMetotlari[prop]
	case *DictVal:
		m, ok = sozlukMetotlari[prop]
	}
	return m, ok
}

// Tablolar init içinde kurulur: gövdeler ApplyFunction'ı çağırdığı için değişken
// başlatıcısında kurulsalardı başlatma döngüsü oluşurdu.
func init() {
	// --- Metin ---
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToUpperSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	}, "büyük_harf", "buyuk_harf")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToLowerSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	}, "küçük_harf", "kucuk_harf")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	}, "kırp", "kirp")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		sep := " "
		if len(args) > 0 {
			sep = args[0].Inspect()
		}
		parts := strings.Split(inst.(*StringVal).Val, sep)
		elems := make([]Value, len(parts))
		for i, p := range parts {
			elems[i] = &StringVal{Val: p}
		}
		return &ListVal{Elements: elems}, nil
	}, "böl", "bol")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("metin.değiştir(eski, yeni) 2 parametre bekler")
		}
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, args[0].Inspect(), args[1].Inspect())}, nil
	}, "değiştir", "degistir")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, args[0].Inspect())), nil
	}, "içerir", "icerir")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, args[0].Inspect())), nil
	}, "başlar", "baslar")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, args[0].Inspect())), nil
	}, "biter")
	metotEkle(metinMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	}, "ters_çevir", "ters_cevir")

	// --- Liste ---
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("liste.filtrele(fonksiyon) 1 parametre bekler")
		}
		var result []Value
		for _, item := range inst.(*ListVal).Elements {
			keep, err := vm.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			if keep.IsTruthy() {
				result = append(result, item)
			}
		}
		return &ListVal{Elements: result}, nil
	}, "filtrele")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("liste.haritala(fonksiyon) 1 parametre bekler")
		}
		elems := inst.(*ListVal).Elements
		result := make([]Value, 0, len(elems))
		for _, item := range elems {
			mapped, err := vm.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		}
		return &ListVal{Elements: result}, nil
	}, "haritala", "dönüştür", "donustur")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		siralaDegerler(copied)
		return &ListVal{Elements: copied}, nil
	}, "sırala", "sirala")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	}, "ters_çevir", "ters_cevir")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		sep := ""
		if len(args) > 0 {
			sep = args[0].Inspect()
		}
		elems := inst.(*ListVal).Elements
		parts := make([]string, len(elems))
		for i, el := range elems {
			parts[i] = el.Inspect()
		}
		return &StringVal{Val: strings.Join(parts, sep)}, nil
	}, "birleştir", "birlestir")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return TRUE, nil
			}
		}
		return FALSE, nil
	}, "içerir", "icerir")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		var unique []Value
		for _, el := range inst.(*ListVal).Elements {
			exists := false
			for _, u := range unique {
				if ValuesEqual(el, u) {
					exists = true
					break
				}
			}
			if !exists {
				unique = append(unique, el)
			}
		}
		return &ListVal{Elements: unique}, nil
	}, "benzersiz")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	}, "ekle")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		sumFloat := 0.0
		hasFloat := false
		var sumInt int64 = 0
		for _, el := range inst.(*ListVal).Elements {
			if iv, ok := el.(*IntVal); ok {
				sumInt += iv.Val
				sumFloat += float64(iv.Val)
			} else if fv, ok := el.(*FloatVal); ok {
				hasFloat = true
				sumFloat += fv.Val
			}
		}
		if hasFloat {
			return &FloatVal{Val: sumFloat}, nil
		}
		return NewInt(sumInt), nil
	}, "toplam")
	metotEkle(listeMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		nums := ExtractNumericSlice([]Value{inst})
		if len(nums) == 0 {
			return &FloatVal{Val: 0}, nil
		}
		s := 0.0
		for _, x := range nums {
			s += x
		}
		return &FloatVal{Val: s / float64(len(nums))}, nil
	}, "ortalama")

	// --- Sözlük ---
	metotEkle(sozlukMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	}, "anahtarlar")
	metotEkle(sozlukMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	}, "değerler", "degerler")
	metotEkle(sozlukMetotlari, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		_, ok := inst.(*DictVal).GetValue(args[0])
		return BoolToVal(ok), nil
	}, "içerir", "icerir")
}

// evalBuiltinMember: Metin, Liste ve Sözlük üzerinde nokta (.) ile özellik okur ya da metodu
// bağlı bir yerleşik olarak döndürür.
func (v *VM) evalBuiltinMember(obj Value, prop string, safe bool, line int) (Value, error) {
	switch inst := obj.(type) {
	case *StringVal:
		if prop == "uzunluk" || prop == "boy" {
			return NewInt(int64(utf8.RuneCountInString(inst.Val))), nil
		}
	case *ListVal:
		switch prop {
		case "uzunluk", "boy":
			return NewInt(int64(len(inst.Elements))), nil
		case "ilk":
			if len(inst.Elements) == 0 {
				return NULL, nil
			}
			return inst.Elements[0], nil
		case "son":
			if len(inst.Elements) == 0 {
				return NULL, nil
			}
			return inst.Elements[len(inst.Elements)-1], nil
		}
	}
	if m, ok := metotBul(obj, prop); ok {
		return &BuiltinVal{Name: m.ad, Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
			return m.f(vm, obj, args, line)
		}}, nil
	}
	if d, ok := obj.(*DictVal); ok {
		if val, ok := d.Pairs[prop]; ok {
			return val, nil
		}
		return NULL, nil
	}

	if safe {
		return NULL, nil
	}
	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği veya metodu bulunamadı", line, obj.Type(), prop)
}
