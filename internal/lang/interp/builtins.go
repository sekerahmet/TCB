package interp

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// coreBuiltins çekirdeğin adlı argüman almayan yerleşikleridir; kayıt sırası
// tablodaki sıradır. Adlar kod listesindedir (internal/lang/codes).
var coreBuiltins = [...]struct {
	kd codes.Code
	fn BuiltinFunc
}{
	{codes.G100, printBuiltin},
	{codes.G101, inputBuiltin},
	{codes.G201, typeBuiltin},
	{codes.G210, intBuiltin},
	{codes.G211, floatBuiltin},
	{codes.G212, stringBuiltin},
	{codes.G305, rangeBuiltin},
	{codes.G400, sumBuiltin},
	{codes.G401, minBuiltin},
	{codes.G402, maxBuiltin},
	{codes.G403, absBuiltin},
	{codes.G404, sqrtBuiltin},
	{codes.G405, roundBuiltin},
	{codes.G501, waitAllBuiltin},
	{codes.G502, timeMsBuiltin},
	{codes.G110, readFileBuiltin},
	{codes.G111, writeFileBuiltin},
	{codes.G500, sleepBuiltin},
}

func (v *VM) registerBuiltins() {
	for _, k := range coreBuiltins {
		v.RegisterNoNamed(k.kd, k.fn)
	}
	v.singleArg(codes.G212, singleChar)
}

// printBuiltin yazdır() yerleşiğidir.
// Ekrana Yazdırma: yazdır(...)
func printBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.Inspect())
	}
	out := strings.Join(parts, " ")
	y.WriteOutput(y.Out, out+"\n")
	return &StringVal{Val: out}, nil
}

// inputBuiltin girdi() yerleşiğidir.
// Kullanıcıdan Girdi Alma: girdi("Adınız: ")
func inputBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) > 1 {
		return nil, ArgCountError(codes.G101, "0 ya da 1", len(args), "", "istem")
	}
	if len(args) > 0 {
		y.WriteOutput(y.Out, args[0].Inspect())
	}
	line, readErr := y.readLine()
	if readErr != nil && readErr != io.EOF && line == "" {
		return nil, fmt.Errorf("%s: standart girdi okunamadı: %s", CallPrefix(codes.G101), turkish.FileErrorReason("", readErr))
	}
	line = strings.TrimRight(line, "\r\n")
	return &StringVal{Val: line}, nil
}

// typeBuiltin tip() yerleşiğidir.
// Tip Sorgulama: tip(x)
func typeBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G201, "1", len(args), "değer")
	}
	return TypeOfValue(args[0]), nil
}

// intBuiltin tam() yerleşiğidir.
// Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
func intBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G210, "1", len(args), "değer")
	}
	switch val := args[0].(type) {
	case *IntVal:
		return val, nil
	case *FloatVal:
		n, err := floatToInt(CallPrefix(codes.G210), val.Val)
		if err != nil {
			return nil, err
		}
		return NewInt(n), nil
	case *StringVal:
		n, err := strconv.ParseInt(strings.TrimSpace(val.Val), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q tam sayıya çevrilemedi (yalnızca rakamlardan oluşan bir metin verin, ör. \"42\")", CallPrefix(codes.G210), val.Val)
		}
		return NewInt(n), nil
	case *BoolVal:
		if val.Val {
			return NewInt(1), nil
		}
		return NewInt(0), nil
	default:
		return nil, ArgTypeError(codes.G210, 1, args[0], codes.A200, codes.A201, codes.A202, codes.A203)
	}
}

// floatBuiltin ondalık() yerleşiğidir.
func floatBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G211, "1", len(args), "değer")
	}
	switch val := args[0].(type) {
	case *FloatVal:
		return val, nil
	case *IntVal:
		return &FloatVal{Val: float64(val.Val)}, nil
	case *StringVal:
		n, err := strconv.ParseFloat(strings.TrimSpace(val.Val), 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q ondalık sayıya çevrilemedi (ondalık ayırıcı nokta olmalı, ör. \"3.14\")", CallPrefix(codes.G211), val.Val)
		}
		return &FloatVal{Val: n}, nil
	default:
		return nil, ArgTypeError(codes.G211, 1, args[0], codes.A201, codes.A200, codes.A202)
	}
}

// stringBuiltin metin() yerleşiğidir.
func stringBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G212, "1", len(args), "değer")
	}
	return singleChar(y, args[0])
}

// rangeBuiltin aralık() yerleşiğidir.
// Liste: aralık(bitiş) ya da aralık(başlangıç, bitiş)
func rangeBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) == 1 {
		end, ok := args[0].(*IntVal)
		if !ok {
			return nil, ArgTypeError(codes.G305, 1, args[0], codes.A200)
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
			return nil, ArgTypeError(codes.G305, 1, args[0], codes.A200)
		}
		end, ok := args[1].(*IntVal)
		if !ok {
			return nil, ArgTypeError(codes.G305, 2, args[1], codes.A200)
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
	return nil, ArgCountError(codes.G305, "1 ya da 2", len(args), "bitiş", "başlangıç, bitiş")
}

// sumBuiltin toplam() yerleşiğidir.
// Temel Matematik Fonksiyonları
func sumBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G400, "1", len(args), "liste")
	}
	list, ok := args[0].(*ListVal)
	if !ok {
		return nil, ArgTypeError(codes.G400, 1, args[0], codes.A205)
	}
	return sumNumbers(CallPrefix(codes.G400), list.Elements)
}

// minBuiltin min() yerleşiğidir.
// min/maks: tek bir liste ya da birden çok sayı alır; kazanan değerin kendisi döner
func minBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return extremeArgs(codes.G401, args, false)
}

// maxBuiltin maks() yerleşiğidir.
func maxBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return extremeArgs(codes.G402, args, true)
}

// absBuiltin mutlak() yerleşiğidir.
func absBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G403, "1", len(args), "sayı")
	}
	if iv, ok := args[0].(*IntVal); ok {
		if iv.Val == math.MinInt64 {
			m, o := codes.Name(codes.G403), codes.Name(codes.G211)
			return nil, fmt.Errorf("%s(): %s sayı taşması (%s(%d)); ondalık sonuç için %s(%s(x)) yazın", m, codes.Name(codes.A200), m, iv.Val, m, o)
		}
		if iv.Val < 0 {
			return NewInt(-iv.Val), nil
		}
		return iv, nil
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Abs(fv)}, nil
	}
	return nil, ArgTypeError(codes.G403, 1, args[0], numberTypes...)
}

// sqrtBuiltin karekök() yerleşiğidir.
func sqrtBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G404, "1", len(args), "sayı")
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Sqrt(fv)}, nil
	}
	return nil, ArgTypeError(codes.G404, 1, args[0], numberTypes...)
}

// roundBuiltin yuvarla() yerleşiğidir.
func roundBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, ArgCountError(codes.G405, "1 ya da 2", len(args), "sayı", "sayı, basamak")
	}
	fv, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.G405, 1, args[0], numberTypes...)
	}
	digits := 0
	if len(args) == 2 {
		iv, ok := args[1].(*IntVal)
		if !ok {
			return nil, ArgTypeError(codes.G405, 2, args[1], codes.A200)
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
	rounded := math.Round(fv)
	if digits < 0 {
		pow := math.Pow(10, float64(-digits))
		rounded = math.Round(fv/pow) * pow
	}
	n, err := floatToInt(CallPrefix(codes.G405), rounded)
	if err != nil {
		return nil, err
	}
	return NewInt(n), nil
}

// waitAllBuiltin bekle_hepsi() yerleşiğidir.
// Eşzamanlılık, Zaman ve Dosya İşlemleri
func waitAllBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	// Görev kendini de beklediği için sonsuza dek kilitlenirdi (deadlock)
	if y.asyncTask {
		return nil, fmt.Errorf("%s '%s' bloğu içinden çağrılamaz; ana programda kullanın", CallPrefix(codes.G501), codes.Name(codes.A080))
	}
	y.WaitAsync()
	return TRUE, nil
}

// timeMsBuiltin zaman_ms() yerleşiğidir.
func timeMsBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return NewInt(time.Now().UnixMilli()), nil
}

// readFileBuiltin dosya_oku() yerleşiğidir.
func readFileBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G110, "1", len(args), "dosya")
	}
	filePath, ok, err := PathArg(CallPrefix(codes.G110), args[0])
	if !ok {
		return nil, ArgTypeError(codes.G110, 1, args[0], codes.A202, codes.A250)
	}
	if err != nil {
		return nil, err
	}
	resolved, searched := y.ResolvePath(filePath)
	var data []byte
	y.Blocking(func() { data, err = ReadFileContent(resolved) })
	if err != nil {
		return nil, ReadError(CallPrefix(codes.G110), filePath, "okunamadı", err, searched)
	}
	return &StringVal{Val: string(data)}, nil
}

// writeFileBuiltin dosya_yaz() yerleşiğidir. Yazma atomiktir (bkz. AtomikYaz).
func writeFileBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 2 {
		return nil, ArgCountError(codes.G111, "2", len(args), "yol, içerik")
	}
	filePath, ok, err := PathArg(CallPrefix(codes.G111), args[0])
	if !ok {
		return nil, ArgTypeError(codes.G111, 1, args[0], codes.A202, codes.A250)
	}
	if err != nil {
		return nil, err
	}
	resolved, _ := y.ResolvePath(filePath)
	content := []byte(args[1].Inspect())
	y.Blocking(func() { err = WriteFileContent(resolved, content) })
	if err != nil {
		return nil, turkish.FileError(CallPrefix(codes.G111), filePath, "yazılamadı", err)
	}
	return TRUE, nil
}

// sleepBuiltin bekle() yerleşiğidir.
func sleepBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G500, "1", len(args), "saniye")
	}
	pick, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.G500, 1, args[0], numberTypes...)
	}
	// Kesme istenirse (Ctrl+C) bekleme hemen biter; program sonraki güvenli noktada durur.
	y.Blocking(func() { y.interruptibleSleep(time.Duration(pick * float64(time.Second))) })
	return TRUE, nil
}

// sayiTipleri sayı bekleyen yerleşiklerin kabul ettiği tiplerdir; sayiTipi hata
// mesajlarındaki adlarıdır ("Tam ya da Ondalık").
var (
	numberTypes = []codes.Code{codes.A200, codes.A201}
	numberType  = TypeNames(numberTypes...)
)

// callable değerin filtrele/haritala/indirge gibi metotlara fonksiyon olarak
// verilebilir olup olmadığını söyler.
func callable(v Value) bool {
	switch v.(type) {
	case *FuncVal, *BuiltinVal, *StructDefVal:
		return true
	}
	return false
}

// MaxCollectionSize is the largest number of elements a single builtin call may produce (it
// guards against running out of memory).
const MaxCollectionSize = 50_000_000

// singleChar metin(x) yerleşiğinin tek argümanlı gövdesidir.
func singleChar(_ *VM, a Value) (Value, error) {
	return &StringVal{Val: a.Inspect()}, nil
}

// singleArg kayıtlı bir yerleşiğe, tek argümanla (adlı argümansız) çağrıldığında
// argüman dilimi ayırmadan çalışan gövdesini bağlar. Gövde, Fn'in tek argümanlı
// durumuyla birebir aynı olmalıdır.
func (v *VM) singleArg(kd codes.Code, single func(*VM, Value) (Value, error)) {
	if b := v.Builtin(kd); b != nil {
		b.Single = single
	}
}

// checkRangeSize aralık(başlangıç, bitiş) için boyut denetimi yapar; boş aralık (bitiş <= başlangıç) geçerlidir.
func checkRangeSize(start, end int64) error {
	if end > start && (end-start > MaxCollectionSize || end-start < 0) {
		return fmt.Errorf("%s çok büyük: en fazla %d eleman üretilebilir", CallPrefix(codes.G305), MaxCollectionSize)
	}
	return nil
}

// extremeArgs min/maks'ın argümanlarını açar: tek bir liste verilirse elemanları,
// değilse argümanların kendisi karşılaştırılır.
func extremeArgs(kd codes.Code, args []Value, upper bool) (Value, error) {
	if len(args) == 0 {
		return nil, ArgCountError(kd, "en az 1", 0, "liste", "a, b, ...")
	}
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			return extreme(kd, list.Elements, upper, true)
		}
	}
	return extreme(kd, args, upper, false)
}
