package vm

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"tr_dili/pkg/sozluk"
)

// Metin, Liste ve Sözlük üzerinde nokta (.) ile çağrılan yerleşik metotlar. Metot gövdeleri
// nesneyi argüman olarak alan sıradan fonksiyonlardır: derlenmiş kod 'x.metot(...)'
// çağrılarını bağlı bir yerleşik nesnesi ayırmadan doğrudan bunlara yönlendirir;
// 'x.metot' bir değer olarak okunursa (ör. değişkene atanırsa) bağlı yerleşik üretilir.

// metotGovdesi bir yerleşik metodun gövdesidir; line, geri çağrılan fonksiyonların
// hata satırıdır.
type metotGovdesi func(vm *VM, inst Value, args []Value, line int) (Value, error)

type yerlesikMetot struct {
	ad string // bağlı yerleşiğin adı (sözlükteki ad)
	f  metotGovdesi
}

// cekirdekMetot bir üye kodunun Metin, Liste ve Sözlük üzerindeki gövdeleridir (tipte yoksa
// nil). Derleyici x.ad'ı derleme anında koda ve bu yapıya çevirir; çalışma anında yalnızca
// nesnenin tipine göre alan seçilir, ad ya da kod araması yapılmaz.
type cekirdekMetot struct {
	metin, liste, sozluk *yerlesikMetot
}

// cekirdekMetotlari üye kodu → Metin/Liste/Sözlük metotları. Adlar sözlüktedir (pkg/sozluk, Y).
var cekirdekMetotlari = map[sozluk.Kod]*cekirdekMetot{}

const (
	tipMetin = iota
	tipListe
	tipSozluk
)

// metotEkle metodu sözlük koduyla kaydeder; ad sözlükten gelir.
func metotEkle(tip int, kod sozluk.Kod, f metotGovdesi) {
	if kod.Alan() != sozluk.AlanUye {
		panic(fmt.Sprintf("metot kaydı: %s bir üye kodu (Y) değil", kod))
	}
	cm := cekirdekMetotlari[kod]
	if cm == nil {
		cm = &cekirdekMetot{}
		cekirdekMetotlari[kod] = cm
	}
	m := &yerlesikMetot{ad: sozluk.Ad(kod), f: f}
	switch tip {
	case tipMetin:
		cm.metin = m
	case tipListe:
		cm.liste = m
	case tipSozluk:
		cm.sozluk = m
	}
}

// Metin ve Liste özellikleri (metot değil, değer döndürürler).
const (
	uyeUzunluk = sozluk.Y + 1   // uzunluk (Metin, Liste)
	uyeEkle    = sozluk.Y + 4   // ekle (Liste)
	uyeIlk     = sozluk.Y + 210 // ilk (Liste)
	uyeSon     = sozluk.Y + 211 // son (Liste)
)

// uyeBilgisi bir x.ad erişiminin derleme anında çözülmüş biçimidir.
type uyeBilgisi struct {
	kod sozluk.Kod // ad bir üye adı değilse sozluk.Yok
	// cekirdek Metin/Liste/Sözlük metotlarıdır; ad bunların metodu değilse nil.
	cekirdek *cekirdekMetot
}

// uyeCoz adı derleme anında çözer.
func uyeCoz(ad string) uyeBilgisi {
	u := uyeBilgisi{kod: sozluk.UyeKodu(ad)}
	if u.kod == sozluk.Yok {
		return u
	}
	u.cekirdek = cekirdekMetotlari[u.kod]
	return u
}

// cekirdekUyesi kod Metin, Liste ya da Sözlük üyesi mi (kaldırılmış ad ipuçlarının süzgeci)?
func cekirdekUyesi(k sozluk.Kod) bool {
	_, metot := cekirdekMetotlari[k]
	return metot || k == uyeUzunluk || k == uyeIlk || k == uyeSon
}

// metotBul nesnenin tipine göre derleme anında çözülmüş metodu döndürür.
func metotBul(obj Value, cm *cekirdekMetot) *yerlesikMetot {
	if cm == nil {
		return nil
	}
	switch obj.(type) {
	case *StringVal:
		return cm.metin
	case *ListVal:
		return cm.liste
	case *DictVal:
		return cm.sozluk
	}
	return nil
}

// Tablolar init içinde kurulur: gövdeler ApplyFunction'ı çağırdığı için değişken
// başlatıcısında kurulsalardı başlatma döngüsü oluşurdu.
func init() {
	// --- Metin ---
	metotEkle(tipMetin, sozluk.Y+100, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToUpperSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+101, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToLowerSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+102, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+103, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipMetin, sozluk.Y+104, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("metin.değiştir(eski, yeni) 2 parametre bekler")
		}
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, args[0].Inspect(), args[1].Inspect())}, nil
	})
	metotEkle(tipMetin, sozluk.Y+2, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+105, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+106, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+3, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	})

	// --- Liste ---
	metotEkle(tipListe, sozluk.Y+200, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipListe, sozluk.Y+201, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipListe, sozluk.Y+202, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		siralaDegerler(copied)
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, sozluk.Y+3, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, sozluk.Y+203, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipListe, sozluk.Y+2, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return TRUE, nil
			}
		}
		return FALSE, nil
	})
	metotEkle(tipListe, sozluk.Y+204, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipListe, sozluk.Y+4, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	})
	metotEkle(tipListe, sozluk.Y+205, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	})
	metotEkle(tipListe, sozluk.Y+206, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		nums := ExtractNumericSlice([]Value{inst})
		if len(nums) == 0 {
			return &FloatVal{Val: 0}, nil
		}
		s := 0.0
		for _, x := range nums {
			s += x
		}
		return &FloatVal{Val: s / float64(len(nums))}, nil
	})

	// --- Sözlük ---
	metotEkle(tipSozluk, sozluk.Y+300, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, sozluk.Y+301, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, sozluk.Y+2, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < 1 {
			return FALSE, nil
		}
		_, ok := inst.(*DictVal).GetValue(args[0])
		return BoolToVal(ok), nil
	})
}

// evalBuiltinMember: Metin, Liste ve Sözlük üzerinde nokta (.) ile özellik okur ya da metodu
// bağlı bir yerleşik olarak döndürür.
func (v *VM) evalBuiltinMember(obj Value, prop string, u uyeBilgisi, safe bool, line int) (Value, error) {
	switch inst := obj.(type) {
	case *StringVal:
		if u.kod == uyeUzunluk {
			return NewInt(int64(utf8.RuneCountInString(inst.Val))), nil
		}
	case *ListVal:
		switch u.kod {
		case uyeUzunluk:
			return NewInt(int64(len(inst.Elements))), nil
		case uyeIlk:
			if len(inst.Elements) == 0 {
				return NULL, nil
			}
			return inst.Elements[0], nil
		case uyeSon:
			if len(inst.Elements) == 0 {
				return NULL, nil
			}
			return inst.Elements[len(inst.Elements)-1], nil
		}
	}
	if m := metotBul(obj, u.cekirdek); m != nil {
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
	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği veya metodu bulunamadı%s", line, obj.Type(), prop, kaldirilanUyeIpucu(prop))
}
