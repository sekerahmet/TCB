package vm

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"tr_dili/pkg/sozluk"
	"unicode/utf8"

	"tr_dili/pkg/turkce"
)

func (v *VM) registerBuiltins() {
	// 1. Ekrana Yazdırma: yazdır(...)
	v.RegisterBuiltinKod(sozluk.G100, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.Inspect())
		}
		out := strings.Join(parts, " ")
		fmt.Fprintln(vm.Out, out)
		return &StringVal{Val: out}, nil
	})

	// 2. Kullanıcıdan Girdi Alma: girdi("Adınız: ")
	v.RegisterBuiltinKod(sozluk.G101, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) > 1 {
			return nil, ArgumanSayisiHatasi("girdi", "0 ya da 1", len(args), "girdi() ya da girdi(istem)")
		}
		if len(args) > 0 {
			fmt.Fprint(vm.Out, args[0].Inspect())
		}
		line, readErr := vm.satirOku()
		if readErr != nil && readErr != io.EOF && line == "" {
			return nil, fmt.Errorf("girdi(): standart girdi okunamadı: %s", turkce.DosyaHatasiNedeni("", readErr))
		}
		line = strings.TrimRight(line, "\r\n")
		return &StringVal{Val: line}, nil
	})

	// 3. Uzunluk / Boyut: uzunluk(x)
	v.RegisterBuiltinKod(sozluk.G200, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("uzunluk", "1", len(args), "uzunluk(değer)")
		}
		return uzunlukTek(vm, args[0])
	})
	v.tekArgumanli(sozluk.G200, uzunlukTek)

	// 4. Tip Sorgulama: tip(x)
	v.RegisterBuiltinKod(sozluk.G201, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("tip", "1", len(args), "tip(değer)")
		}
		return &StringVal{Val: string(args[0].Type())}, nil
	})

	// 5. Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
	v.RegisterBuiltinKod(sozluk.G210, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("tam", "1", len(args), "tam(değer)")
		}
		switch val := args[0].(type) {
		case *IntVal:
			return val, nil
		case *FloatVal:
			n, err := ondaliktanTam("tam()", val.Val)
			if err != nil {
				return nil, err
			}
			return NewInt(n), nil
		case *StringVal:
			n, err := strconv.ParseInt(strings.TrimSpace(val.Val), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("tam(): %q tam sayıya çevrilemedi (yalnızca rakamlardan oluşan bir metin verin, ör. \"42\")", val.Val)
			}
			return NewInt(n), nil
		case *BoolVal:
			if val.Val {
				return NewInt(1), nil
			}
			return NewInt(0), nil
		default:
			return nil, ArgumanTipHatasi("tam", 1, "Tam, Ondalık, Metin ya da Mantık", args[0])
		}
	})

	v.RegisterBuiltinKod(sozluk.G211, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("ondalık", "1", len(args), "ondalık(değer)")
		}
		switch val := args[0].(type) {
		case *FloatVal:
			return val, nil
		case *IntVal:
			return &FloatVal{Val: float64(val.Val)}, nil
		case *StringVal:
			n, err := strconv.ParseFloat(strings.TrimSpace(val.Val), 64)
			if err != nil {
				return nil, fmt.Errorf("ondalık(): %q ondalık sayıya çevrilemedi (ondalık ayırıcı nokta olmalı, ör. \"3.14\")", val.Val)
			}
			return &FloatVal{Val: n}, nil
		default:
			return nil, ArgumanTipHatasi("ondalık", 1, "Ondalık, Tam ya da Metin", args[0])
		}
	})

	v.RegisterBuiltinKod(sozluk.G212, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("metin", "1", len(args), "metin(değer)")
		}
		return metinTek(vm, args[0])
	})
	v.tekArgumanli(sozluk.G212, metinTek)

	// 6. Liste ve Fonksiyonel Boru Hattı (|>) İşlemleri: filtrele, haritala, indirge, sırala, ekle, sil
	v.RegisterBuiltinKod(sozluk.G300, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, ArgumanSayisiHatasi("filtrele", "2", len(args), "filtrele(liste, fonksiyon)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("filtrele", 1, "Liste", args[0])
		}
		if !cagrilabilir(args[1]) {
			return nil, ArgumanTipHatasi("filtrele", 2, "Fonksiyon", args[1])
		}
		var result []Value
		for _, item := range list.Elements {
			keep, err := vm.ApplyFunction(args[1], []Value{item}, nil, 0)
			if err != nil {
				return nil, err
			}
			if keep.IsTruthy() {
				result = append(result, item)
			}
		}
		return &ListVal{Elements: result}, nil
	})

	v.RegisterBuiltinKod(sozluk.G301, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, ArgumanSayisiHatasi("haritala", "2", len(args), "haritala(liste, fonksiyon)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("haritala", 1, "Liste", args[0])
		}
		if !cagrilabilir(args[1]) {
			return nil, ArgumanTipHatasi("haritala", 2, "Fonksiyon", args[1])
		}
		result := make([]Value, 0, len(list.Elements))
		for _, item := range list.Elements {
			mapped, err := vm.ApplyFunction(args[1], []Value{item}, nil, 0)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		}
		return &ListVal{Elements: result}, nil
	})

	v.RegisterBuiltinKod(sozluk.G302, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, ArgumanSayisiHatasi("indirge", "2 ya da 3", len(args), "indirge(liste, fonksiyon) ya da indirge(liste, fonksiyon, başlangıç)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("indirge", 1, "Liste", args[0])
		}
		if !cagrilabilir(args[1]) {
			return nil, ArgumanTipHatasi("indirge", 2, "Fonksiyon", args[1])
		}
		if len(list.Elements) == 0 {
			if len(args) == 3 {
				return args[2], nil
			}
			return NULL, nil
		}
		var acc Value
		startIdx := 0
		if len(args) == 3 {
			acc = args[2]
		} else {
			acc = list.Elements[0]
			startIdx = 1
		}
		for i := startIdx; i < len(list.Elements); i++ {
			var err error
			acc, err = vm.ApplyFunction(args[1], []Value{acc, list.Elements[i]}, nil, 0)
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	})

	v.RegisterBuiltinKod(sozluk.G303, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("sırala", "1", len(args), "sırala(liste)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("sırala", 1, "Liste", args[0])
		}
		copied := make([]Value, len(list.Elements))
		copy(copied, list.Elements)
		if err := siralaDegerler("sırala()", copied); err != nil {
			return nil, err
		}
		return &ListVal{Elements: copied}, nil
	})

	v.RegisterBuiltinKod(sozluk.G304, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 {
			return nil, ArgumanSayisiHatasi("ekle", "en az 2", len(args), "ekle(liste, eleman, ...)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("ekle", 1, "Liste", args[0])
		}
		list.Elements = append(list.Elements, args[1:]...)
		return list, nil
	})

	v.RegisterBuiltinKod(sozluk.G305, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) == 1 {
			end, ok := args[0].(*IntVal)
			if !ok {
				return nil, ArgumanTipHatasi("aralık", 1, "Tam", args[0])
			}
			if err := checkRangeSize(0, end.Val); err != nil {
				return nil, err
			}
			elems := []Value{}
			for i := int64(0); i < end.Val; i++ {
				elems = append(elems, NewInt(i))
			}
			return &ListVal{Elements: elems}, nil
		}
		if len(args) == 2 {
			start, ok := args[0].(*IntVal)
			if !ok {
				return nil, ArgumanTipHatasi("aralık", 1, "Tam", args[0])
			}
			end, ok := args[1].(*IntVal)
			if !ok {
				return nil, ArgumanTipHatasi("aralık", 2, "Tam", args[1])
			}
			if err := checkRangeSize(start.Val, end.Val); err != nil {
				return nil, err
			}
			elems := []Value{}
			for i := start.Val; i < end.Val; i++ {
				elems = append(elems, NewInt(i))
			}
			return &ListVal{Elements: elems}, nil
		}
		return nil, ArgumanSayisiHatasi("aralık", "1 ya da 2", len(args), "aralık(bitiş) ya da aralık(başlangıç, bitiş)")
	})

	// 7. Temel Matematik Fonksiyonları
	v.RegisterBuiltinKod(sozluk.G400, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("toplam", "1", len(args), "toplam(liste)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("toplam", 1, "Liste", args[0])
		}
		return sayiTopla("toplam()", list.Elements)
	})

	// min/maks: tek bir liste ya da birden çok sayı alır; kazanan değerin kendisi döner
	v.RegisterBuiltinKod(sozluk.G401, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return enUcArgumanlari("min", args, false)
	})

	v.RegisterBuiltinKod(sozluk.G402, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return enUcArgumanlari("maks", args, true)
	})

	v.RegisterBuiltinKod(sozluk.G403, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("mutlak", "1", len(args), "mutlak(sayı)")
		}
		if iv, ok := args[0].(*IntVal); ok {
			if iv.Val == math.MinInt64 {
				return nil, fmt.Errorf("mutlak(): Tam sayı taşması (mutlak(%d)); ondalık sonuç için mutlak(ondalık(x)) yazın", iv.Val)
			}
			if iv.Val < 0 {
				return NewInt(-iv.Val), nil
			}
			return iv, nil
		}
		if fv, ok := ToFloat(args[0]); ok {
			return &FloatVal{Val: math.Abs(fv)}, nil
		}
		return nil, ArgumanTipHatasi("mutlak", 1, sayiTipi, args[0])
	})

	v.RegisterBuiltinKod(sozluk.G404, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("karekök", "1", len(args), "karekök(sayı)")
		}
		if fv, ok := ToFloat(args[0]); ok {
			return &FloatVal{Val: math.Sqrt(fv)}, nil
		}
		return nil, ArgumanTipHatasi("karekök", 1, sayiTipi, args[0])
	})

	v.RegisterBuiltinKod(sozluk.G405, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, ArgumanSayisiHatasi("yuvarla", "1 ya da 2", len(args), "yuvarla(sayı) ya da yuvarla(sayı, basamak)")
		}
		fv, ok := ToFloat(args[0])
		if !ok {
			return nil, ArgumanTipHatasi("yuvarla", 1, sayiTipi, args[0])
		}
		digits := 0
		if len(args) == 2 {
			iv, ok := args[1].(*IntVal)
			if !ok {
				return nil, ArgumanTipHatasi("yuvarla", 2, "Tam", args[1])
			}
			digits = int(iv.Val)
		}
		if digits > 0 {
			pow := math.Pow(10, float64(digits))
			return &FloatVal{Val: math.Round(fv*pow) / pow}, nil
		}
		// Basamak 0 ya da negatifse sonuç Tam'dır: yuvarla(1234, -2) → 1200
		if iv, ok := args[0].(*IntVal); ok && digits == 0 {
			return iv, nil
		}
		yuvarlanan := math.Round(fv)
		if digits < 0 {
			pow := math.Pow(10, float64(-digits))
			yuvarlanan = math.Round(fv/pow) * pow
		}
		n, err := ondaliktanTam("yuvarla()", yuvarlanan)
		if err != nil {
			return nil, err
		}
		return NewInt(n), nil
	})

	// 8. Eşzamanlılık, Zaman ve Dosya İşlemleri
	v.RegisterBuiltinKod(sozluk.G501, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		// Görev kendini de beklediği için sonsuza dek kilitlenirdi (deadlock)
		if vm.asyncTask {
			return nil, fmt.Errorf("bekle_hepsi() 'eşzamanlı' bloğu içinden çağrılamaz; ana programda kullanın")
		}
		vm.WaitAsync()
		return TRUE, nil
	})

	v.RegisterBuiltinKod(sozluk.G502, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return NewInt(time.Now().UnixMilli()), nil
	})

	v.RegisterBuiltinKod(sozluk.G110, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("dosya_oku", "1", len(args), "dosya_oku(yol)")
		}
		yol, ok := args[0].(*StringVal)
		if !ok {
			return nil, ArgumanTipHatasi("dosya_oku", 1, "Metin (dosya yolu)", args[0])
		}
		cozulen, bakilanlar := vm.YolCoz(yol.Val)
		data, err := os.ReadFile(cozulen)
		if err != nil {
			return nil, OkumaHatasi("dosya_oku()", yol.Val, "okunamadı", err, bakilanlar)
		}
		return &StringVal{Val: string(data)}, nil
	})

	v.RegisterBuiltinKod(sozluk.G111, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, ArgumanSayisiHatasi("dosya_yaz", "2", len(args), "dosya_yaz(yol, içerik)")
		}
		yol, ok := args[0].(*StringVal)
		if !ok {
			return nil, ArgumanTipHatasi("dosya_yaz", 1, "Metin (dosya yolu)", args[0])
		}
		cozulen, _ := vm.YolCoz(yol.Val)
		if err := os.WriteFile(cozulen, []byte(args[1].Inspect()), 0644); err != nil {
			return nil, turkce.DosyaHatasi("dosya_yaz()", yol.Val, "yazılamadı", err)
		}
		return TRUE, nil
	})

	v.RegisterBuiltinKod(sozluk.G500, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("bekle", "1", len(args), "bekle(saniye)")
		}
		sec, ok := ToFloat(args[0])
		if !ok {
			return nil, ArgumanTipHatasi("bekle", 1, sayiTipi+" (saniye)", args[0])
		}
		vm.Blocking(func() { time.Sleep(time.Duration(sec * float64(time.Second))) })
		return TRUE, nil
	})
}

// sayiTipi sayı bekleyen yerleşiklerin hata mesajlarındaki tip adıdır.
const sayiTipi = "Tam ya da Ondalık"

// cagrilabilir değerin filtrele/haritala/indirge gibi yerleşiklere fonksiyon olarak
// verilebilir olup olmadığını söyler.
func cagrilabilir(v Value) bool {
	switch v.(type) {
	case *FuncVal, *BuiltinVal, *StructDefVal:
		return true
	}
	return false
}

// MaxCollectionSize tek bir yerleşik çağrının üretebileceği en fazla eleman sayısıdır (bellek taşmasını önler).
const MaxCollectionSize = 50_000_000

// checkRangeSize aralık(başlangıç, bitiş) için boyut denetimi yapar; boş aralık (bitiş <= başlangıç) geçerlidir.
// uzunlukTek uzunluk(x) yerleşiğinin tek argümanlı gövdesidir.
func uzunlukTek(_ *VM, a Value) (Value, error) {
	switch val := a.(type) {
	case *StringVal:
		return NewInt(int64(utf8.RuneCountInString(val.Val))), nil
	case *ListVal:
		return NewInt(int64(len(val.Elements))), nil
	case *DictVal:
		return NewInt(int64(len(val.Pairs))), nil
	default:
		return nil, ArgumanTipHatasi("uzunluk", 1, "Metin, Liste ya da Sözlük", a)
	}
}

// metinTek metin(x) yerleşiğinin tek argümanlı gövdesidir.
func metinTek(_ *VM, a Value) (Value, error) {
	return &StringVal{Val: a.Inspect()}, nil
}

// tekArgumanli kayıtlı bir yerleşiğe, tek argümanla (adlı argümansız) çağrıldığında
// argüman dilimi ayırmadan çalışan gövdesini bağlar. Gövde, Fn'in tek argümanlı
// durumuyla birebir aynı olmalıdır.
func (v *VM) tekArgumanli(kod sozluk.Kod, tek func(*VM, Value) (Value, error)) {
	if b := v.Yerlesik(kod); b != nil {
		b.Tek = tek
	}
}

func checkRangeSize(start, end int64) error {
	if end > start && (end-start > MaxCollectionSize || end-start < 0) {
		return fmt.Errorf("aralık() çok büyük: en fazla %d eleman üretilebilir", MaxCollectionSize)
	}
	return nil
}

// enUcArgumanlari min/maks'ın argümanlarını açar: tek bir liste verilirse elemanları,
// değilse argümanların kendisi karşılaştırılır.
func enUcArgumanlari(ad string, args []Value, buyuk bool) (Value, error) {
	if len(args) == 0 {
		return nil, ArgumanSayisiHatasi(ad, "en az 1", 0, ad+"(liste) ya da "+ad+"(a, b, ...)")
	}
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			return enUc(ad+"()", list.Elements, buyuk, true)
		}
	}
	return enUc(ad, args, buyuk, false)
}

// ExtractNumericSlice argümanlardaki (ya da tek liste argümanının elemanlarındaki) sayıları
// Ondalık olarak toplar; sayı olmayanları atlar. Motorların istatistik yerleşikleri kullanır.
func ExtractNumericSlice(args []Value) []float64 {
	var res []float64
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			for _, el := range list.Elements {
				if f, ok := ToFloat(el); ok {
					res = append(res, f)
				}
			}
			return res
		}
	}
	for _, a := range args {
		if f, ok := ToFloat(a); ok {
			res = append(res, f)
		}
	}
	return res
}
