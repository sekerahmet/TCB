package interp

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Metin, Liste ve Sözlük üzerinde nokta (.) ile çağrılan yerleşik metotlar. Metot gövdeleri
// nesneyi argüman olarak alan sıradan fonksiyonlardır: derlenmiş kod 'x.metot(...)'
// çağrılarını bağlı bir yerleşik nesnesi ayırmadan doğrudan bunlara yönlendirir;
// 'x.metot' bir değer olarak okunursa (ör. değişkene atanırsa) bağlı yerleşik üretilir.

// metotGovdesi bir yerleşik metodun gövdesidir; line, geri çağrılan fonksiyonların
// hata satırıdır.
type metotGovdesi func(y *VM, inst Value, args []Value, line int) (Value, error)

type yerlesikMetot struct {
	ad    string // bağlı yerleşiğin adı (kod listesindeki ad)
	tamAd string // hata mesajlarındaki tipli adı (metin.içerir)
	f     metotGovdesi
}

// cekirdekMetot bir üye kodunun Metin, Liste ve Sözlük üzerindeki gövdeleridir (tipte yoksa
// nil). Derleyici x.ad'ı derleme anında koda ve bu yapıya çevirir; çalışma anında yalnızca
// nesnenin tipine göre alan seçilir, ad ya da kod araması yapılmaz.
type cekirdekMetot struct {
	metin, liste, sozluk *yerlesikMetot
}

// cekirdekMetotlari üye kodu → Metin/Liste/Sözlük metotları. Adlar kod listesindedir (internal/lang/codes, Y).
var cekirdekMetotlari = map[codes.Kod]*cekirdekMetot{}

const (
	tipMetin = iota
	tipListe
	tipSozluk
)

// argTipi bir metot argümanının beklenen tipidir (hata mesajındaki tip kodu ve denetimi).
type argTipi struct {
	tip  codes.Kod
	uyar func(Value) bool
}

var (
	metinArg = &argTipi{codes.A202, func(v Value) bool { _, ok := v.(*StringVal); return ok }}
	fonkArg  = &argTipi{codes.A208, cagrilabilir}
)

// imza bir metodun argüman sayısı ve tipleridir: en az enAz, en çok enCok (< 0: sınırsız)
// argüman; tipler[i] i. argümanın tipi (nil ya da eksikse her tip). parametreler kullanım
// metnindeki parametre adlarıdır ("alt", "eski, yeni").
type imza struct {
	enAz, enCok  int
	parametreler string
	tipler       []*argTipi
}

// tipKodlari metodun alıcı tipinin kodudur; hata mesajlarında metot tipiyle yazılır
// (metin.içerir(); bkz. kod.AliciAdi).
var tipKodlari = [...]codes.Kod{tipMetin: codes.A202, tipListe: codes.A205, tipSozluk: codes.A206}

// metotEkle metodu sözlük koduyla kaydeder; ad sözlükten gelir. Gövde çağrılmadan önce
// argümanlar imzaya göre denetlenir (yerleşik fonksiyonlarla aynı hata biçimi).
func metotEkle(tip int, kd codes.Kod, im imza, f metotGovdesi) {
	if kd.Alan() != codes.AlanUye {
		panic(fmt.Sprintf("metot kaydı: %s bir üye kodu (Y) değil", kd))
	}
	cm := cekirdekMetotlari[kd]
	if cm == nil {
		cm = &cekirdekMetot{}
		cekirdekMetotlari[kd] = cm
	}
	tamAd := codes.AliciAdi(tipKodlari[tip]) + "." + codes.Ad(kd)
	govde := f
	f = func(y *VM, inst Value, args []Value, line int) (Value, error) {
		if len(args) < im.enAz || (im.enCok >= 0 && len(args) > im.enCok) {
			beklenen := fmt.Sprint(im.enAz)
			switch {
			case im.enCok < 0:
				beklenen = "en az " + beklenen
			case im.enCok != im.enAz:
				beklenen = fmt.Sprintf("%d ya da %d", im.enAz, im.enCok)
			}
			return nil, argumanSayisiHatasiAdla(tamAd, beklenen, len(args), tamAd+"("+im.parametreler+")")
		}
		for i, a := range args {
			if i < len(im.tipler) && im.tipler[i] != nil && !im.tipler[i].uyar(a) {
				return nil, argumanTipHatasiAdla(tamAd, i+1, codes.Ad(im.tipler[i].tip), a)
			}
		}
		return govde(y, inst, args, line)
	}
	m := &yerlesikMetot{ad: codes.Ad(kd), tamAd: tamAd, f: f}
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
	uyeUzunluk = codes.Y001 // uzunluk (Metin, Liste)
	uyeEkle    = codes.Y004 // ekle (Liste)
	uyeIlk     = codes.Y210 // ilk (Liste)
	uyeSon     = codes.Y211 // son (Liste)
)

// uyeBilgisi bir x.ad erişiminin derleme anında çözülmüş biçimidir.
type uyeBilgisi struct {
	kod codes.Kod // ad bir üye adı değilse kod.Yok
	// cekirdek Metin/Liste/Sözlük metotlarıdır; ad bunların metodu değilse nil.
	cekirdek *cekirdekMetot
}

// uyeCoz adı derleme anında çözer.
func uyeCoz(ad string) uyeBilgisi {
	u := uyeBilgisi{kod: codes.UyeKodu(ad)}
	if u.kod == codes.Yok {
		return u
	}
	u.cekirdek = cekirdekMetotlari[u.kod]
	return u
}

// cekirdekUyesi kod Metin, Liste ya da Sözlük üyesi mi (kaldırılmış ad ipuçlarının süzgeci)?
func cekirdekUyesi(k codes.Kod) bool {
	_, metot := cekirdekMetotlari[k]
	return metot || k == uyeUzunluk || k == uyeIlk || k == uyeSon
}

// sozlukUyesiMi ad bir sözlük üyesinin (metot ya da .uzunluk) adı mı?
func sozlukUyesiMi(ad string) bool {
	k := codes.UyeKodu(ad)
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
	metinMetotlariniKur()
	listeMetotlariniKur()
	sozlukMetotlariniKur()
}

// metinMetotlariniKur Metin tipinin metotlarını kaydeder.
func metinMetotlariniKur() {
	metotEkle(tipMetin, codes.Y100, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToUpperSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, codes.Y101, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ToLowerSpecial(unicode.TurkishCase, inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, codes.Y102, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.TrimSpace(inst.(*StringVal).Val)}, nil
	})
	metotEkle(tipMetin, codes.Y103, imza{0, 1, "[ayırıcı]", []*argTipi{metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipMetin, codes.Y104, imza{2, 2, "eski, yeni", []*argTipi{metinArg, metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return &StringVal{Val: strings.ReplaceAll(inst.(*StringVal).Val, args[0].Inspect(), args[1].Inspect())}, nil
	})
	metotEkle(tipMetin, codes.Y002, imza{1, 1, "alt", []*argTipi{metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.Contains(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, codes.Y105, imza{1, 1, "önek", []*argTipi{metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasPrefix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, codes.Y106, imza{1, 1, "sonek", []*argTipi{metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return BoolToVal(strings.HasSuffix(inst.(*StringVal).Val, args[0].Inspect())), nil
	})
	metotEkle(tipMetin, codes.Y003, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		runes := []rune(inst.(*StringVal).Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringVal{Val: string(runes)}, nil
	})
}

// listeMetotlariniKur Liste tipinin metotlarını kaydeder.
func listeMetotlariniKur() {
	metotEkle(tipListe, codes.Y200, imza{1, 1, codes.AliciAdi(codes.A208), []*argTipi{fonkArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		var result []Value
		for _, item := range inst.(*ListVal).Elements {
			keep, err := y.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			if keep.IsTruthy() {
				result = append(result, item)
			}
		}
		return &ListVal{Elements: result}, nil
	})
	metotEkle(tipListe, codes.Y201, imza{1, 1, codes.AliciAdi(codes.A208), []*argTipi{fonkArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		result := make([]Value, 0, len(elems))
		for _, item := range elems {
			mapped, err := y.ApplyFunction(args[0], []Value{item}, nil, line)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		}
		return &ListVal{Elements: result}, nil
	})
	metotEkle(tipListe, codes.Y202, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		copy(copied, elems)
		if err := siralaDegerler(Cagri(codes.Y202), copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, codes.Y003, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		elems := inst.(*ListVal).Elements
		copied := make([]Value, len(elems))
		for i, el := range elems {
			copied[len(elems)-1-i] = el
		}
		return &ListVal{Elements: copied}, nil
	})
	metotEkle(tipListe, codes.Y203, imza{0, 1, "[ayırıcı]", []*argTipi{metinArg}}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, codes.Y002, imza{1, 1, "değer", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		for _, el := range inst.(*ListVal).Elements {
			if ValuesEqual(el, args[0]) {
				return TRUE, nil
			}
		}
		return FALSE, nil
	})
	metotEkle(tipListe, codes.Y204, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
	metotEkle(tipListe, codes.Y004, imza{1, -1, "eleman, ...", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		l := inst.(*ListVal)
		l.Elements = append(l.Elements, args...)
		return l, nil
	})
	metotEkle(tipListe, codes.Y207, imza{1, 2, "[başlangıç], " + codes.AliciAdi(codes.A208), nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		return listeIndirge(y, inst.(*ListVal).Elements, args, line)
	})
}

// sozlukMetotlariniKur Sözlük tipinin metotlarını kaydeder.
func sozlukMetotlariniKur() {
	metotEkle(tipSozluk, codes.Y300, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.KeyValue(k)
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, codes.Y301, imza{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
		d := inst.(*DictVal)
		elems := make([]Value, len(d.Order))
		for i, k := range d.Order {
			elems[i] = d.Pairs[k]
		}
		return &ListVal{Elements: elems}, nil
	})
	metotEkle(tipSozluk, codes.Y002, imza{1, 1, "anahtar", nil}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
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
		return &BuiltinVal{Name: m.ad, Fn: func(y *VM, args []Value, named map[string]Value) (Value, error) {
			if len(named) > 0 { // çekirdek metotlar adlı argüman almaz
				return nil, AdliAlmazHatasi(m.tamAd, named)
			}
			return m.f(y, obj, args, line)
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
	if h := tasinanUyeHatasi(obj, prop); h != "" {
		return nil, fmt.Errorf("Satır %d: %s", line, h)
	}
	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği veya metodu bulunamadı%s", line, obj.Type(), prop, kaldirilanUyeIpucu(prop))
}

// listeIndirge liste.indirge([başlangıç], fonksiyon) metodunun gövdesidir. Fonksiyon son
// argümandır (sondaki blokla da yazılabilsin diye: liste.indirge(0) { birikim + o }) ve
// (birikim, eleman) ile çağrılır; kullanıcı fonksiyonu iki parametreli olmalıdır.
// Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (verilmemişse yok)
// döner.
func listeIndirge(y *VM, elems []Value, args []Value, line int) (Value, error) {
	ad, fonk := CagriAdi(codes.Y207), codes.AliciAdi(codes.A208)
	f := args[len(args)-1]
	if !cagrilabilir(f) {
		if len(args) == 2 && cagrilabilir(args[0]) {
			return nil, fmt.Errorf("%s(): %s son argüman olmalı; %s(başlangıç, %s) ya da %s(başlangıç) { ... } yazın", ad, fonk, ad, fonk, ad)
		}
		return nil, ArgumanTipHatasi(codes.Y207, len(args), f, codes.A208)
	}
	if fn, ok := f.(*FuncVal); ok && len(fn.Parameters) != 2 {
		b := codes.Ad(codes.A105)
		return nil, fmt.Errorf("%s(): %s iki parametre almalı (%s, eleman), %d parametreli verildi; ör. %s(0, (%s, x) => %s + x) ya da %s(0) { %s + %s }",
			ad, fonk, b, len(fn.Parameters), ad, b, b, ad, b, codes.Ad(codes.A104))
	}
	var acc Value
	bas := 0
	switch {
	case len(args) == 2:
		acc = args[0]
	case len(elems) == 0:
		return NULL, nil
	default:
		acc, bas = elems[0], 1
	}
	for _, el := range elems[bas:] {
		var err error
		acc, err = y.ApplyFunction(f, []Value{acc, el}, nil, line)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// SayiOrtalamasi sayıların aritmetik ortalamasını Ondalık olarak verir; boş girdi ve
// sayı olmayan eleman hatadır (ortalama(liste) yerleşiği kullanır).
func SayiOrtalamasi(ad string, elems []Value) (Value, error) {
	if len(elems) == 0 {
		return nil, fmt.Errorf("%s: boş listenin ortalaması yoktur", ad)
	}
	s := 0.0
	for i, el := range elems {
		f, ok := ToFloat(el)
		if !ok {
			return nil, ListeElemaniHatasi(ad, i, sayiTipi, el)
		}
		s += f
	}
	return &FloatVal{Val: s / float64(len(elems))}, nil
}
