package vm

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (v *VM) registerBuiltins() {
	// 1. Ekrana Yazdırma: yazdır(...) ve yaz(...)
	v.RegisterBuiltin("yazdır", "yazdir", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.Inspect())
		}
		out := strings.Join(parts, " ")
		fmt.Fprintln(vm.Out, out)
		return &StringVal{Val: out}, nil
	})

	v.RegisterBuiltin("yaz", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.Inspect())
		}
		out := strings.Join(parts, " ")
		fmt.Fprintln(vm.Out, out)
		return &StringVal{Val: out}, nil
	})

	// 2. Kullanıcıdan Girdi Alma: girdi("Adınız: ")
	v.RegisterBuiltin("girdi", "oku", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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
			return nil, fmt.Errorf("girdi okunamadı: %v", readErr)
		}
		line = strings.TrimRight(line, "\r\n")
		return &StringVal{Val: line}, nil
	})

	// 3. Uzunluk / Boyut: uzunluk(x)
	v.RegisterBuiltin("uzunluk", "boy", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("uzunluk() 1 parametre bekler, %d verildi", len(args))
		}
		switch val := args[0].(type) {
		case *StringVal:
			return &IntVal{Val: int64(len([]rune(val.Val)))}, nil
		case *ListVal:
			return &IntVal{Val: int64(len(val.Elements))}, nil
		case *DictVal:
			return &IntVal{Val: int64(len(val.Pairs))}, nil
		default:
			return nil, fmt.Errorf("uzunluk() %s tipi için desteklenmiyor", args[0].Type())
		}
	})

	// 4. Tip Sorgulama: tip(x)
	v.RegisterBuiltin("tip", "tur", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("tip() 1 parametre bekler")
		}
		return &StringVal{Val: string(args[0].Type())}, nil
	})

	// 5. Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
	v.RegisterBuiltin("tam", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("tam() 1 parametre bekler")
		}
		switch val := args[0].(type) {
		case *IntVal:
			return val, nil
		case *FloatVal:
			return &IntVal{Val: int64(val.Val)}, nil
		case *StringVal:
			n, err := strconv.ParseInt(strings.TrimSpace(val.Val), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%q tam sayıya çevrilemedi", val.Val)
			}
			return &IntVal{Val: n}, nil
		case *BoolVal:
			if val.Val {
				return &IntVal{Val: 1}, nil
			}
			return &IntVal{Val: 0}, nil
		default:
			return nil, fmt.Errorf("%s tipi Tam sayıya çevrilemez", args[0].Type())
		}
	})

	v.RegisterBuiltin("ondalık", "ondalik", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("ondalık() 1 parametre bekler")
		}
		switch val := args[0].(type) {
		case *FloatVal:
			return val, nil
		case *IntVal:
			return &FloatVal{Val: float64(val.Val)}, nil
		case *StringVal:
			n, err := strconv.ParseFloat(strings.TrimSpace(val.Val), 64)
			if err != nil {
				return nil, fmt.Errorf("%q ondalık sayıya çevrilemedi", val.Val)
			}
			return &FloatVal{Val: n}, nil
		default:
			return nil, fmt.Errorf("%s tipi Ondalık sayıya çevrilemez", args[0].Type())
		}
	})

	v.RegisterBuiltin("metin", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("metin() 1 parametre bekler")
		}
		return &StringVal{Val: args[0].Inspect()}, nil
	})

	// 6. Liste ve Fonksiyonel Boru Hattı (|>) İşlemleri: filtrele, haritala, indirge, sırala, ekle, sil
	v.RegisterBuiltin("filtrele", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("filtrele(liste, fonksiyon) 2 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("filtrele() ilk parametre olarak Liste bekler, %s verildi", args[0].Type())
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

	v.RegisterBuiltin("haritala", "donustur", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("haritala(liste, fonksiyon) 2 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("haritala() ilk parametre olarak Liste bekler, %s verildi", args[0].Type())
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

	v.RegisterBuiltin("indirge", "ozetle", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("indirge(liste, fonksiyon, [başlangıç]) 2 veya 3 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("indirge() ilk parametre olarak Liste bekler")
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

	v.RegisterBuiltin("sırala", "sirala", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("sırala(liste) en az 1 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("sırala() Liste bekler")
		}
		copied := make([]Value, len(list.Elements))
		copy(copied, list.Elements)
		sort.SliceStable(copied, func(i, j int) bool {
			fi, ok1 := ToFloat(copied[i])
			fj, ok2 := ToFloat(copied[j])
			if ok1 && ok2 {
				return fi < fj
			}
			return copied[i].Inspect() < copied[j].Inspect()
		})
		return &ListVal{Elements: copied}, nil
	})

	v.RegisterBuiltin("ekle", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ekle(liste, eleman...) en az 2 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("ekle() ilk parametre olarak Liste bekler")
		}
		list.Elements = append(list.Elements, args[1:]...)
		return list, nil
	})

	v.RegisterBuiltin("aralık", "aralik", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) == 1 {
			end, ok := args[0].(*IntVal)
			if !ok {
				return nil, fmt.Errorf("aralık() Tam sayı bekler")
			}
			if err := checkRangeSize(0, end.Val); err != nil {
				return nil, err
			}
			elems := []Value{}
			for i := int64(0); i < end.Val; i++ {
				elems = append(elems, &IntVal{Val: i})
			}
			return &ListVal{Elements: elems}, nil
		}
		if len(args) >= 2 {
			start, ok1 := args[0].(*IntVal)
			end, ok2 := args[1].(*IntVal)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("aralık() Tam sayı bekler")
			}
			if err := checkRangeSize(start.Val, end.Val); err != nil {
				return nil, err
			}
			elems := []Value{}
			for i := start.Val; i < end.Val; i++ {
				elems = append(elems, &IntVal{Val: i})
			}
			return &ListVal{Elements: elems}, nil
		}
		return nil, fmt.Errorf("aralık() 1 veya 2 parametre bekler")
	})

	// 7. Temel Matematik Fonksiyonları
	v.RegisterBuiltin("toplam", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("toplam(liste) 1 parametre bekler")
		}
		list, ok := args[0].(*ListVal)
		if !ok {
			return nil, fmt.Errorf("toplam() Liste bekler")
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
		return &IntVal{Val: sumInt}, nil
	})

	v.RegisterBuiltin("min", "en_küçük", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltin("maks", "en_büyük", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
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

	v.RegisterBuiltin("mutlak", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("mutlak(x) 1 parametre bekler")
		}
		if iv, ok := args[0].(*IntVal); ok {
			if iv.Val < 0 {
				return &IntVal{Val: -iv.Val}, nil
			}
			return iv, nil
		}
		if fv, ok := ToFloat(args[0]); ok {
			return &FloatVal{Val: math.Abs(fv)}, nil
		}
		return nil, fmt.Errorf("mutlak() sayısal değer bekler")
	})

	v.RegisterBuiltin("karekök", "karekok", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("karekök(x) 1 parametre bekler")
		}
		if fv, ok := ToFloat(args[0]); ok {
			return &FloatVal{Val: math.Sqrt(fv)}, nil
		}
		return nil, fmt.Errorf("karekök() sayısal değer bekler")
	})

	v.RegisterBuiltin("yuvarla", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("yuvarla(sayı, [basamak]) en az 1 parametre bekler")
		}
		fv, ok := ToFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("yuvarla() sayısal değer bekler")
		}
		digits := 0
		if len(args) >= 2 {
			if iv, ok := args[1].(*IntVal); ok {
				digits = int(iv.Val)
			}
		}
		if digits <= 0 {
			return &IntVal{Val: int64(math.Round(fv))}, nil
		}
		pow := math.Pow(10, float64(digits))
		return &FloatVal{Val: math.Round(fv*pow) / pow}, nil
	})

	// 8. Eşzamanlılık, Zaman ve Dosya İşlemleri
	v.RegisterBuiltin("bekle_hepsi", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		// Görev kendini de beklediği için sonsuza dek kilitlenirdi (deadlock)
		if vm.asyncTask {
			return nil, fmt.Errorf("bekle_hepsi() 'eşzamanlı' bloğu içinden çağrılamaz; ana programda kullanın")
		}
		vm.WaitAsync()
		return TRUE, nil
	})

	v.RegisterBuiltin("zaman_ms", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		return &IntVal{Val: time.Now().UnixMilli()}, nil
	})

	v.RegisterBuiltin("dosya_oku", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("dosya_oku(yol) 1 parametre bekler")
		}
		data, err := os.ReadFile(args[0].Inspect())
		if err != nil {
			return nil, fmt.Errorf("dosya okunamadı: %v", err)
		}
		return &StringVal{Val: string(data)}, nil
	})

	v.RegisterBuiltin("dosya_yaz", "", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("dosya_yaz(yol, içerik) 2 parametre bekler")
		}
		err := os.WriteFile(args[0].Inspect(), []byte(args[1].Inspect()), 0644)
		if err != nil {
			return nil, fmt.Errorf("dosya yazılamadı: %v", err)
		}
		return TRUE, nil
	})

	v.RegisterBuiltin("bekle", "uyut", func(vm *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("bekle(saniye) 1 parametre bekler")
		}
		sec, ok := ToFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("bekle() sayısal saniye değeri bekler")
		}
		vm.Blocking(func() { time.Sleep(time.Duration(sec * float64(time.Second))) })
		return TRUE, nil
	})
}

// MaxCollectionSize tek bir yerleşik çağrının üretebileceği en fazla eleman sayısıdır (bellek taşmasını önler).
const MaxCollectionSize = 50_000_000

// checkRangeSize aralık(başlangıç, bitiş) için boyut denetimi yapar; boş aralık (bitiş <= başlangıç) geçerlidir.
func checkRangeSize(start, end int64) error {
	if end > start && (end-start > MaxCollectionSize || end-start < 0) {
		return fmt.Errorf("aralık() çok büyük: en fazla %d eleman üretilebilir", MaxCollectionSize)
	}
	return nil
}

// evalBuiltinMember: Metin, Liste ve Sözlük üzerinde nokta (.) ile zincirleme metodları çalıştırır.
func (v *VM) evalBuiltinMember(obj Value, prop string, safe bool, line int) (Value, error) {
	switch inst := obj.(type) {
	case *StringVal:
		switch prop {
		case "uzunluk", "boy":
			return &IntVal{Val: int64(len([]rune(inst.Val)))}, nil
		case "büyük_harf", "buyuk_harf":
			return &BuiltinVal{Name: "büyük_harf", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				return &StringVal{Val: strings.ToUpperSpecial(unicode.TurkishCase, inst.Val)}, nil
			}}, nil
		case "küçük_harf", "kucuk_harf":
			return &BuiltinVal{Name: "küçük_harf", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				return &StringVal{Val: strings.ToLowerSpecial(unicode.TurkishCase, inst.Val)}, nil
			}}, nil
		case "kırp", "kirp":
			return &BuiltinVal{Name: "kırp", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				return &StringVal{Val: strings.TrimSpace(inst.Val)}, nil
			}}, nil
		case "böl", "bol":
			return &BuiltinVal{Name: "böl", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				sep := " "
				if len(args) > 0 {
					sep = args[0].Inspect()
				}
				parts := strings.Split(inst.Val, sep)
				elems := make([]Value, len(parts))
				for i, p := range parts {
					elems[i] = &StringVal{Val: p}
				}
				return &ListVal{Elements: elems}, nil
			}}, nil
		case "değiştir", "degistir":
			return &BuiltinVal{Name: "değiştir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("metin.değiştir(eski, yeni) 2 parametre bekler")
				}
				return &StringVal{Val: strings.ReplaceAll(inst.Val, args[0].Inspect(), args[1].Inspect())}, nil
			}}, nil
		case "içerir", "icerir":
			return &BuiltinVal{Name: "içerir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 1 {
					return FALSE, nil
				}
				return BoolToVal(strings.Contains(inst.Val, args[0].Inspect())), nil
			}}, nil
		case "başlar", "baslar":
			return &BuiltinVal{Name: "başlar", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 1 {
					return FALSE, nil
				}
				return BoolToVal(strings.HasPrefix(inst.Val, args[0].Inspect())), nil
			}}, nil
		case "biter":
			return &BuiltinVal{Name: "biter", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 1 {
					return FALSE, nil
				}
				return BoolToVal(strings.HasSuffix(inst.Val, args[0].Inspect())), nil
			}}, nil
		case "ters_çevir", "ters_cevir":
			return &BuiltinVal{Name: "ters_çevir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				runes := []rune(inst.Val)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return &StringVal{Val: string(runes)}, nil
			}}, nil
		}

	case *ListVal:
		switch prop {
		case "uzunluk", "boy":
			return &IntVal{Val: int64(len(inst.Elements))}, nil
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
		case "filtrele":
			return &BuiltinVal{Name: "filtrele", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("liste.filtrele(fonksiyon) 1 parametre bekler")
				}
				var result []Value
				for _, item := range inst.Elements {
					keep, err := vm.ApplyFunction(args[0], []Value{item}, nil, line)
					if err != nil {
						return nil, err
					}
					if keep.IsTruthy() {
						result = append(result, item)
					}
				}
				return &ListVal{Elements: result}, nil
			}}, nil
		case "haritala", "dönüştür", "donustur":
			return &BuiltinVal{Name: "haritala", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("liste.haritala(fonksiyon) 1 parametre bekler")
				}
				result := make([]Value, 0, len(inst.Elements))
				for _, item := range inst.Elements {
					mapped, err := vm.ApplyFunction(args[0], []Value{item}, nil, line)
					if err != nil {
						return nil, err
					}
					result = append(result, mapped)
				}
				return &ListVal{Elements: result}, nil
			}}, nil
		case "sırala", "sirala":
			return &BuiltinVal{Name: "sırala", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				copied := make([]Value, len(inst.Elements))
				copy(copied, inst.Elements)
				sort.SliceStable(copied, func(i, j int) bool {
					fi, ok1 := ToFloat(copied[i])
					fj, ok2 := ToFloat(copied[j])
					if ok1 && ok2 {
						return fi < fj
					}
					return copied[i].Inspect() < copied[j].Inspect()
				})
				return &ListVal{Elements: copied}, nil
			}}, nil
		case "ters_çevir", "ters_cevir":
			return &BuiltinVal{Name: "ters_çevir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				copied := make([]Value, len(inst.Elements))
				for i, el := range inst.Elements {
					copied[len(inst.Elements)-1-i] = el
				}
				return &ListVal{Elements: copied}, nil
			}}, nil
		case "birleştir", "birlestir":
			return &BuiltinVal{Name: "birleştir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				sep := ""
				if len(args) > 0 {
					sep = args[0].Inspect()
				}
				parts := make([]string, len(inst.Elements))
				for i, el := range inst.Elements {
					parts[i] = el.Inspect()
				}
				return &StringVal{Val: strings.Join(parts, sep)}, nil
			}}, nil
		case "içerir", "icerir":
			return &BuiltinVal{Name: "içerir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 1 {
					return FALSE, nil
				}
				for _, el := range inst.Elements {
					if ValuesEqual(el, args[0]) {
						return TRUE, nil
					}
				}
				return FALSE, nil
			}}, nil
		case "benzersiz":
			return &BuiltinVal{Name: "benzersiz", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				var unique []Value
				for _, el := range inst.Elements {
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
			}}, nil
		case "ekle":
			return &BuiltinVal{Name: "ekle", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				inst.Elements = append(inst.Elements, args...)
				return inst, nil
			}}, nil
		case "toplam":
			return &BuiltinVal{Name: "toplam", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				sumFloat := 0.0
				hasFloat := false
				var sumInt int64 = 0
				for _, el := range inst.Elements {
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
				return &IntVal{Val: sumInt}, nil
			}}, nil
		case "ortalama":
			return &BuiltinVal{Name: "ortalama", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				nums := ExtractNumericSlice([]Value{inst})
				if len(nums) == 0 {
					return &FloatVal{Val: 0}, nil
				}
				s := 0.0
				for _, x := range nums {
					s += x
				}
				return &FloatVal{Val: s / float64(len(nums))}, nil
			}}, nil
		}

	case *DictVal:
		switch prop {
		case "anahtarlar":
			return &BuiltinVal{Name: "anahtarlar", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				elems := make([]Value, len(inst.Order))
				for i, k := range inst.Order {
					elems[i] = inst.KeyValue(k)
				}
				return &ListVal{Elements: elems}, nil
			}}, nil
		case "değerler", "degerler":
			return &BuiltinVal{Name: "değerler", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				elems := make([]Value, len(inst.Order))
				for i, k := range inst.Order {
					elems[i] = inst.Pairs[k]
				}
				return &ListVal{Elements: elems}, nil
			}}, nil
		case "içerir", "icerir":
			return &BuiltinVal{Name: "içerir", Fn: func(vm *VM, args []Value, named map[string]Value) (Value, error) {
				if len(args) < 1 {
					return FALSE, nil
				}
				_, ok := inst.GetValue(args[0])
				return BoolToVal(ok), nil
			}}, nil
		}
		if val, ok := inst.Pairs[prop]; ok {
			return val, nil
		}
		return NULL, nil
	}

	if safe {
		return NULL, nil
	}
	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği veya metodu bulunamadı", line, obj.Type(), prop)
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
		return &IntVal{Val: int64(f)}
	}
	return &FloatVal{Val: f}
}

