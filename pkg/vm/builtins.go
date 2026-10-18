package vm

import (
	"bufio"
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
	v.RegisterBuiltinKod(sozluk.G+100, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.Inspect())
		}
		out := strings.Join(parts, " ")
		fmt.Fprintln(vm.Out, out)
		return &StringVal{Val: out}, nil
	})

	// 2. Kullanıcıdan Girdi Alma: girdi("Adınız: ")
	v.RegisterBuiltinKod(sozluk.G+101, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) > 1 {
			return nil, ArgumanSayisiHatasi("girdi", "0 ya da 1", len(args), "girdi() ya da girdi(istem)")
		}
		if len(args) > 0 {
			fmt.Fprint(vm.Out, args[0].Inspect())
		}
		var src io.Reader = os.Stdin
		if vm.Stdin != nil {
			src = vm.Stdin
		}
		reader := bufio.NewReader(src)
		var line string
		var readErr error
		vm.Blocking(func() { line, readErr = reader.ReadString('\n') })
		if readErr != nil && readErr != io.EOF && line == "" {
			return nil, fmt.Errorf("girdi(): standart girdi okunamadı: %s", turkce.DosyaHatasiNedeni("", readErr))
		}
		line = strings.TrimRight(line, "\r\n")
		return &StringVal{Val: line}, nil
	})

	// 3. Uzunluk / Boyut: uzunluk(x)
	v.RegisterBuiltinKod(sozluk.G+200, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("uzunluk", "1", len(args), "uzunluk(değer)")
		}
		return uzunlukTek(vm, args[0])
	})
	v.tekArgumanli(sozluk.G+200, uzunlukTek)

	// 4. Tip Sorgulama: tip(x)
	v.RegisterBuiltinKod(sozluk.G+201, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("tip", "1", len(args), "tip(değer)")
		}
		return &StringVal{Val: string(args[0].Type())}, nil
	})

	// 5. Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
	v.RegisterBuiltinKod(sozluk.G+210, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("tam", "1", len(args), "tam(değer)")
		}
		switch val := args[0].(type) {
		case *IntVal:
			return val, nil
		case *FloatVal:
			return NewInt(int64(val.Val)), nil
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

	v.RegisterBuiltinKod(sozluk.G+211, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+212, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("metin", "1", len(args), "metin(değer)")
		}
		return metinTek(vm, args[0])
	})
	v.tekArgumanli(sozluk.G+212, metinTek)

	// 6. Liste ve Fonksiyonel Boru Hattı (|>) İşlemleri: filtrele, haritala, indirge, sırala, ekle, sil
	v.RegisterBuiltinKod(sozluk.G+300, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+301, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+302, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+303, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("sırala", "1", len(args), "sırala(liste)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("sırala", 1, "Liste", args[0])
		}
		copied := make([]Value, len(list.Elements))
		copy(copied, list.Elements)
		siralaDegerler(copied)
		return &ListVal{Elements: copied}, nil
	})

	v.RegisterBuiltinKod(sozluk.G+304, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+305, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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
	v.RegisterBuiltinKod(sozluk.G+400, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("toplam", "1", len(args), "toplam(liste)")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, ArgumanTipHatasi("toplam", 1, "Liste", args[0])
		}
		sumFloat := 0.0
		hasFloat := false
		var sumInt int64 = 0
		for _, el := range list.Elements {
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

	v.RegisterBuiltinKod(sozluk.G+401, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		elems := ExtractNumericSlice(args)
		if len(elems) == 0 {
			return NULL, nil
		}
		m := elems[0]
		for _, x := range elems[1:] {
			if x < m {
				m = x
			}
		}
		return formatNumberVal(m), nil
	})

	v.RegisterBuiltinKod(sozluk.G+402, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		elems := ExtractNumericSlice(args)
		if len(elems) == 0 {
			return NULL, nil
		}
		m := elems[0]
		for _, x := range elems[1:] {
			if x > m {
				m = x
			}
		}
		return formatNumberVal(m), nil
	})

	v.RegisterBuiltinKod(sozluk.G+403, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("mutlak", "1", len(args), "mutlak(sayı)")
		}
		if iv, ok := args[0].(*IntVal); ok {
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

	v.RegisterBuiltinKod(sozluk.G+404, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgumanSayisiHatasi("karekök", "1", len(args), "karekök(sayı)")
		}
		if fv, ok := ToFloat(args[0]); ok {
			return &FloatVal{Val: math.Sqrt(fv)}, nil
		}
		return nil, ArgumanTipHatasi("karekök", 1, sayiTipi, args[0])
	})

	v.RegisterBuiltinKod(sozluk.G+405, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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
		if digits <= 0 {
			return NewInt(int64(math.Round(fv))), nil
		}
		pow := math.Pow(10, float64(digits))
		return &FloatVal{Val: math.Round(fv*pow) / pow}, nil
	})

	// 8. Eşzamanlılık, Zaman ve Dosya İşlemleri
	v.RegisterBuiltinKod(sozluk.G+501, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		// Görev kendini de beklediği için sonsuza dek kilitlenirdi (deadlock)
		if vm.asyncTask {
			return nil, fmt.Errorf("bekle_hepsi() 'eşzamanlı' bloğu içinden çağrılamaz; ana programda kullanın")
		}
		vm.WaitAsync()
		return TRUE, nil
	})

	v.RegisterBuiltinKod(sozluk.G+502, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return NewInt(time.Now().UnixMilli()), nil
	})

	v.RegisterBuiltinKod(sozluk.G+110, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+111, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltinKod(sozluk.G+500, func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

func formatNumberVal(f float64) Value {
	if f == float64(int64(f)) {
		return NewInt(int64(f))
	}
	return &FloatVal{Val: f}
}
