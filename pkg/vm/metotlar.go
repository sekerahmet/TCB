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

// argTipi bir metot argümanının beklenen tipidir (hata mesajındaki adı ve denetimi).
type argTipi struct {
	ad   string
	uyar func(Value) bool
}

var (
	metinArg = &argTipi{"Metin", func(v Value) bool { _, ok := v.(*StringVal); return ok }}
	fonkArg  = &argTipi{"Fonksiyon", cagrilabilir}
)

// imza bir metodun argüman sayısı ve tipleridir: en az enAz, en çok enCok (< 0: sınırsız)
// argüman; tipler[i] i. argümanın tipi (nil ya da eksikse her tip). parametreler kullanım
// metnindeki parametre adlarıdır ("alt", "eski, yeni").
type imza struct {
	enAz, enCok  int
	parametreler string
	tipler       []*argTipi
}

// tipOneki metodun hata mesajlarında tipiyle yazılan adının önekidir (metin.içerir()).
var tipOneki = map[int]string{tipMetin: "metin", tipListe: "liste", tipSozluk: "sözlük"}

// metotEkle metodu sözlük koduyla kaydeder; ad sözlükten gelir. Gövde çağrılmadan önce
// argümanlar imzaya göre denetlenir (yerleşik fonksiyonlarla aynı hata biçimi).
func metotEkle(tip int, kod sozluk.Kod, im imza, f metotGovdesi) {
	if kod.Alan() != sozluk.AlanUye {
		panic(fmt.Sprintf("metot kaydı: %s bir üye kodu (Y) değil", kod))
	}
	cm := cekirdekMetotlari[kod]
	if cm == nil {
		cm = &cekirdekMetot{}
		cekirdekMetotlari[kod] = cm
	}
	tamAd := tipOneki[tip] + "." + sozluk.Ad(kod)
	govde := f
	f = func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < im.enAz || (im.enCok >= 0 && len(args) > im.enCok) {
			beklenen := fmt.Sprint(im.enAz)
			switch {
			case im.enCok < 0:
				beklenen = "en az " + beklenen
			case im.enCok != im.enAz:
				beklenen = fmt.Sprintf("%d ya da %d", im.enAz, im.enCok)
			}
			return nil, ArgumanSayisiHatasi(tamAd, beklenen, len(args), tamAd+"("+im.parametreler+")")
		}
		for i, a := range args {
			if i < len(im.tipler) && im.tipler[i] != nil && !im.tipler[i].uyar(a) {
				return nil, ArgumanTipHatasi(tamAd, i+1, im.tipler[i].ad, a)
			}
		}
		return govde(vm, inst, args, line)
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

// sozlukUyesiMi ad bir sözlük üyesinin (metot ya da .uzunluk) adı mı?
func sozlukUyesiMi(ad string) bool {
	k := sozluk.UyeKodu(ad)
	if k == uyeUzunluk {
		return true
	}
	cm := cekirdekMetotlari[k]
	return cm != nil && cm.sozluk != nil
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
	metotEkle(tipMetin, sozluk.Y+100, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToUpperSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+101, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToLowerSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+102, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, sozluk.Y+103, imza{0, 1, "[ayırıcı]", []*argTipi{metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipMetin, sozluk.Y+104, imza{2, 2, "eski, yeni", []*argTipi{metinArg, metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, args[0].Inspect(), args[1].Inspect())}, nil
	})
	metotEkle(tipMetin, sozluk.Y+2, imza{1, 1, "alt", []*argTipi{metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+105, imza{1, 1, "önek", []*argTipi{metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+106, imza{1, 1, "sonek", []*argTipi{metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, sozluk.Y+3, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	})

	// --- Liste ---
	metotEkle(tipListe, sozluk.Y+200, imza{1, 1, "fonksiyon", []*argTipi{fonkArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, sozluk.Y+201, imza{1, 1, "fonksiyon", []*argTipi{fonkArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, sozluk.Y+202, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		if err := siralaDegerler("liste.sırala()", copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, sozluk.Y+3, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, sozluk.Y+203, imza{0, 1, "[ayırıcı]", []*argTipi{metinArg}}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, sozluk.Y+2, imza{1, 1, "değer", nil}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return TRUE, nil
			}
		}
		return FALSE, nil
	})
	metotEkle(tipListe, sozluk.Y+204, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, sozluk.Y+4, imza{1, -1, "eleman, ...", nil}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	})
	metotEkle(tipListe, sozluk.Y+205, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		return sayiTopla("liste.toplam()", inst.(*ListVal).Elements)
	})
	metotEkle(tipListe, sozluk.Y+206, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		if len(elems) == 0 {
			return nil, fmt.Errorf("liste.ortalama(): boş listenin ortalaması yoktur")
		}
		s := 0.0
		for i, el := range elems {
			f, ok := ToFloat(el)
			if !ok {
				return nil, ListeElemaniHatasi("liste.ortalama()", i, sayiTipi, el)
			}
			s += f
		}
		return &FloatVal{Val: s / float64(len(elems))}, nil
	})

	// --- Sözlük ---
	metotEkle(tipSozluk, sozluk.Y+300, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, sozluk.Y+301, imza{}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, sozluk.Y+2, imza{1, 1, "anahtar", nil}, func(vm *VM, inst Value, args []Value, line int) (Value, error) {
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
	case *DictVal:
		if u.kod == uyeUzunluk {
			return NewInt(int64(len(inst.Pairs))), nil
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
