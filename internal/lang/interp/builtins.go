package interp

import (
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// coreBuiltins çekirdeğin adlı argüman almayan yerleşikleridir; kayıt sırası
// tablodaki sıradır. Adlar kod listesindedir (internal/lang/codes).
var coreBuiltins = [...]struct {
	kd codes.Code
	fn BuiltinFunc
}{
	{codes.FnPrint, printBuiltin},
	{codes.FnInput, inputBuiltin},
	{codes.FnType, typeBuiltin},
	{codes.FnInt, intBuiltin},
	{codes.FnFloat, floatBuiltin},
	{codes.FnStr, stringBuiltin},
	{codes.FnRange, rangeBuiltin},
	{codes.FnSum, sumBuiltin},
	{codes.FnMin, minBuiltin},
	{codes.FnMax, maxBuiltin},
	{codes.FnAbs, absBuiltin},
	{codes.FnSqrt, sqrtBuiltin},
	{codes.FnRound, roundBuiltin},
	{codes.FnWaitAll, waitAllBuiltin},
	{codes.FnTimeMs, timeMsBuiltin},
	{codes.FnReadFile, readFileBuiltin},
	{codes.FnWriteFile, writeFileBuiltin},
	{codes.FnSleep, sleepBuiltin},
}

func (v *VM) registerBuiltins() {
	for _, k := range coreBuiltins {
		v.RegisterNoNamed(k.kd, k.fn)
	}
	v.singleArg(codes.FnStr, singleChar)
}

// printBuiltin yazdır() yerleşiğidir.
// Ekrana Yazdırma: yazdır(...)
func printBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	var parts []string
	lang := y.textLanguage()
	for _, a := range args {
		parts = append(parts, valueText(a, lang))
	}
	out := strings.Join(parts, " ")
	y.WriteOutput(y.Out, out+"\n")
	return &StringVal{Val: out}, nil
}

// inputBuiltin girdi() yerleşiğidir.
// Kullanıcıdan Girdi Alma: girdi("Adınız: ")
func inputBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) > 1 {
		return nil, ArgCountError(codes.FnInput, Counts(0, 1), len(args), "", messages.Text(messages.H2300))
	}
	if len(args) > 0 {
		y.WriteOutput(y.Out, y.textOf(args[0]))
	}
	line, readErr := y.readLine()
	if readErr != nil && readErr != io.EOF && line == "" {
		return nil, messages.New(messages.H2507, CallPrefix(codes.FnInput), turkish.FileErrorReason("", readErr))
	}
	line = strings.TrimRight(line, "\r\n")
	return &StringVal{Val: line}, nil
}

// typeBuiltin tip() yerleşiğidir.
// Tip Sorgulama: tip(x)
func typeBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnType, 1, len(args), messages.Text(messages.H2301))
	}
	return TypeOfValue(args[0]), nil
}

// intBuiltin tam() yerleşiğidir.
// Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
func intBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnInt, 1, len(args), messages.Text(messages.H2301))
	}
	switch val := args[0].(type) {
	case *IntVal:
		return val, nil
	case *FloatVal:
		n, err := floatToInt(CallPrefix(codes.FnInt), val.Val)
		if err != nil {
			return nil, err
		}
		return NewInt(n), nil
	case *StringVal:
		n, err := strconv.ParseInt(strings.TrimSpace(val.Val), 10, 64)
		if err != nil {
			return nil, messages.New(messages.H2429, CallPrefix(codes.FnInt), val.Val)
		}
		return NewInt(n), nil
	case *BoolVal:
		if val.Val {
			return NewInt(1), nil
		}
		return NewInt(0), nil
	default:
		return nil, ArgTypeError(codes.FnInt, 1, args[0], codes.TypeInt, codes.TypeFloat, codes.TypeString, codes.TypeBool)
	}
}

// floatBuiltin ondalık() yerleşiğidir.
func floatBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnFloat, 1, len(args), messages.Text(messages.H2301))
	}
	switch val := args[0].(type) {
	case *FloatVal:
		return val, nil
	case *IntVal:
		return &FloatVal{Val: float64(val.Val)}, nil
	case *StringVal:
		s := strings.TrimSpace(val.Val)
		n, err := strconv.ParseFloat(s, 64) // "inf", "nan" her dilde okunur
		if err != nil {
			var ok bool
			if n, ok = ParseNonFinite(s, y.textLanguage()); !ok {
				return nil, messages.New(messages.H2430, CallPrefix(codes.FnFloat), val.Val)
			}
		}
		return &FloatVal{Val: n}, nil
	default:
		return nil, ArgTypeError(codes.FnFloat, 1, args[0], codes.TypeFloat, codes.TypeInt, codes.TypeString)
	}
}

// stringBuiltin metin() yerleşiğidir.
func stringBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnStr, 1, len(args), messages.Text(messages.H2301))
	}
	return singleChar(y, args[0])
}

// rangeBuiltin aralık() yerleşiğidir.
// Liste: aralık(bitiş) ya da aralık(başlangıç, bitiş)
func rangeBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, ArgCountError(codes.FnRange, Counts(1, 2), len(args), messages.Text(messages.H2302), messages.Text(messages.H2303))
	}
	var bounds [2]int64 // tek argümanda başlangıç 0
	first := 2 - len(args)
	for i, arg := range args {
		n, ok := arg.(*IntVal)
		if !ok {
			return nil, ArgTypeError(codes.FnRange, i+1, arg, codes.TypeInt)
		}
		bounds[first+i] = n.Val
	}
	start, end := bounds[0], bounds[1]
	if err := checkRangeSize(start, end); err != nil {
		return nil, err
	}
	elems := []Value{}
	for i := start; i < end; i++ {
		elems = append(elems, NewInt(i))
	}
	return &ListVal{Elements: elems}, nil
}

// sumBuiltin toplam() yerleşiğidir.
// Temel Matematik Fonksiyonları
func sumBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnSum, 1, len(args), messages.Text(messages.H2304))
	}
	list, ok := args[0].(*ListVal)
	if !ok {
		return nil, ArgTypeError(codes.FnSum, 1, args[0], codes.TypeList)
	}
	return sumNumbers(CallPrefix(codes.FnSum), list.Elements)
}

// minBuiltin min() yerleşiğidir.
// min/max: tek bir liste ya da birden çok sayı alır; kazanan değerin kendisi döner
func minBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return extremeArgs(codes.FnMin, args, false)
}

// maxBuiltin max() yerleşiğidir.
func maxBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return extremeArgs(codes.FnMax, args, true)
}

// absBuiltin mutlak() yerleşiğidir.
func absBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnAbs, 1, len(args), messages.Text(messages.H2305))
	}
	if iv, ok := args[0].(*IntVal); ok {
		if iv.Val == math.MinInt64 {
			m, o := codes.NameRef(codes.FnAbs), codes.NameRef(codes.FnFloat)
			return nil, messages.New(messages.H2431, m, codes.NameRef(codes.TypeInt), iv.Val, o)
		}
		if iv.Val < 0 {
			return NewInt(-iv.Val), nil
		}
		return iv, nil
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Abs(fv)}, nil
	}
	return nil, ArgTypeError(codes.FnAbs, 1, args[0], numberTypes...)
}

// sqrtBuiltin karekök() yerleşiğidir.
func sqrtBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnSqrt, 1, len(args), messages.Text(messages.H2305))
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Sqrt(fv)}, nil
	}
	return nil, ArgTypeError(codes.FnSqrt, 1, args[0], numberTypes...)
}

// roundBuiltin yuvarla() yerleşiğidir.
func roundBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, ArgCountError(codes.FnRound, Counts(1, 2), len(args), messages.Text(messages.H2305), messages.Text(messages.H2306))
	}
	fv, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.FnRound, 1, args[0], numberTypes...)
	}
	digits := 0
	if len(args) == 2 {
		iv, ok := args[1].(*IntVal)
		if !ok {
			return nil, ArgTypeError(codes.FnRound, 2, args[1], codes.TypeInt)
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
	n, err := floatToInt(CallPrefix(codes.FnRound), rounded)
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
		return nil, messages.New(messages.H2432, CallPrefix(codes.FnWaitAll), codes.NameRef(codes.KwAsync))
	}
	if y.exclusive != nil {
		// Kilit devredilmediğinden görevler hiç ilerlemez: beklemek kilitlenirdi.
		return nil, messages.New(messages.H2522, CallPrefix(codes.FnWaitAll), y.exclusive)
	}
	y.WaitAsync()
	return True, nil
}

// timeMsBuiltin zaman_ms() yerleşiğidir.
func timeMsBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return NewInt(time.Now().UnixMilli()), nil
}

// readFileBuiltin dosya_oku() yerleşiğidir.
func readFileBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnReadFile, 1, len(args), messages.Text(messages.H2307))
	}
	filePath, ok, err := PathArg(CallPrefix(codes.FnReadFile), args[0])
	if !ok {
		return nil, ArgTypeError(codes.FnReadFile, 1, args[0], codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return nil, err
	}
	resolved, searched := y.ResolvePath(filePath)
	var data []byte
	y.Blocking(func() { data, err = ReadFileContent(resolved) })
	if err != nil {
		return nil, ReadError(CallPrefix(codes.FnReadFile), filePath, messages.H2100, err, searched)
	}
	return &StringVal{Val: string(data)}, nil
}

// writeFileBuiltin dosya_yaz() yerleşiğidir. Yazma atomiktir (bkz. AtomikYaz).
func writeFileBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 2 {
		return nil, ArgCountError(codes.FnWriteFile, 2, len(args), messages.Text(messages.H2308))
	}
	filePath, ok, err := PathArg(CallPrefix(codes.FnWriteFile), args[0])
	if !ok {
		return nil, ArgTypeError(codes.FnWriteFile, 1, args[0], codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return nil, err
	}
	resolved, _ := y.ResolvePath(filePath)
	content := []byte(y.textOf(args[1]))
	y.Blocking(func() { err = WriteFileContent(resolved, content) })
	if err != nil {
		return nil, turkish.FileError(CallPrefix(codes.FnWriteFile), filePath, messages.H2101, err)
	}
	return True, nil
}

// sleepBuiltin bekle() yerleşiğidir.
// maxSleepSeconds is the longest bekle(): the largest time.Duration, about 292 years.
const maxSleepSeconds = float64(math.MaxInt64) / float64(time.Second)

func sleepBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnSleep, 1, len(args), messages.Text(messages.H2309))
	}
	pick, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.FnSleep, 1, args[0], numberTypes...)
	}
	// A negative, NaN or huge duration used to return at once without an error (the
	// conversion to time.Duration overflows).
	if math.IsNaN(pick) || pick < 0 {
		return nil, messages.New(messages.H2433, CallPrefix(codes.FnSleep), Shown(args[0]))
	}
	if pick > maxSleepSeconds {
		return nil, messages.New(messages.H2434, CallPrefix(codes.FnSleep), Shown(args[0]), strconv.FormatFloat(maxSleepSeconds, 'f', 0, 64))
	}
	// Kesme istenirse (Ctrl+C) bekleme hemen biter; program sonraki güvenli noktada durur.
	y.Blocking(func() { y.interruptibleSleep(time.Duration(pick * float64(time.Second))) })
	return True, nil
}

// sayiTipleri sayı bekleyen yerleşiklerin kabul ettiği tiplerdir; sayiTipi hata
// mesajlarındaki adlarıdır ("Tam ya da Ondalık").
var (
	numberTypes = []codes.Code{codes.TypeInt, codes.TypeFloat}
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
func singleChar(y *VM, a Value) (Value, error) {
	return &StringVal{Val: y.textOf(a)}, nil
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
		return messages.New(messages.H2435, CallPrefix(codes.FnRange), MaxCollectionSize)
	}
	return nil
}

// extremeArgs min/max'ın argümanlarını açar: tek bir liste verilirse elemanları,
// değilse argümanların kendisi karşılaştırılır.
func extremeArgs(kd codes.Code, args []Value, upper bool) (Value, error) {
	if len(args) == 0 {
		return nil, ArgCountError(kd, AtLeast(1), 0, messages.Text(messages.H2304), messages.Text(messages.H2310))
	}
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			return extreme(kd, list.Elements, upper, true)
		}
	}
	return extreme(kd, args, upper, false)
}
