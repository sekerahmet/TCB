package interp

import (
	"io"
	"math"
	"os"
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
	{codes.FnSum, sumBuiltin},
	{codes.FnMin, minBuiltin},
	{codes.FnMax, maxBuiltin},
	{codes.FnAbs, absBuiltin},
	{codes.FnSqrt, sqrtBuiltin},
	{codes.FnRound, roundBuiltin},
	{codes.FnWaitAll, waitAllBuiltin},
	{codes.FnTimeMs, timeMsBuiltin},
	{codes.FnReadFile, readFileBuiltin},
	{codes.FnReadBytes, readBytesBuiltin},
	{codes.FnWriteFile, writeFileBuiltin},
	{codes.FnSleep, sleepBuiltin},
	{codes.FnExit, exitBuiltin},
	{codes.FnHandleStop, handleStopBuiltin},
	{codes.FnStopRequested, stopRequestedBuiltin},
}

func (v *VM) registerBuiltins() {
	for _, k := range coreBuiltins {
		v.RegisterNoNamed(k.kd, k.fn)
	}
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
	line, readErr, waitErr := y.readLine()
	if waitErr != nil {
		return nil, waitErr
	}
	if readErr != nil && readErr != io.EOF && line == "" {
		return nil, messages.New(messages.H2507, CallPrefix(codes.FnInput), turkish.FileErrorReason("", readErr))
	}
	line = strings.TrimRight(line, "\r\n")
	return &StringVal{Val: line}, nil
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
// min/max: tek bir liste ya da birden çok sayı alır; Tam ile Ondalık karışırsa sonuç Ondalık'tır (extreme)
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
			m, o := codes.NameRef(codes.FnAbs), codes.NameRef(codes.TypeFloat)
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

// waitAllBuiltin bekle_hepsi() yerleşiğidir: argümansız başlatılmış bütün görevleri bekler; bir görev
// listesiyle o görevleri bekler ve sonuçlarını verilen sırayla verir (10.5, waitTasks).
// Eşzamanlılık, Zaman ve Dosya İşlemleri
func waitAllBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	switch len(args) {
	case 0:
	case 1:
		return waitTasks(y, args[0]) // verilen görevler (10.5)
	default:
		return nil, ArgCountError(codes.FnWaitAll, Counts(0, 1), len(args))
	}
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

// readFileBuiltin dosya_oku() yerleşiğidir: dosyayı metin olarak okur, .gz'i açar (18.12). İçerik
// geçerli UTF-8 değilse hatadır (13.1): ileti aynı yolla dosya_baytları'nı önerir (H2621); .gz'de
// dosya_baytları açılmış içeriği vermediği için önermez (H2622).
func readFileBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return readWhole(y, codes.FnReadFile, args, ReadFileContent, func(data []byte, filePath string) (Value, error) {
		if pos := invalidUTF8(data); pos >= 0 {
			code := messages.H2621
			if IsGzip(filePath) {
				code = messages.H2622
			}
			return nil, messages.New(code, CallPrefix(codes.FnReadFile), filePath, pos)
		}
		return &StringVal{Val: string(data)}, nil
	})
}

// readBytesBuiltin dosya_baytları() yerleşiğidir (13.1): dosyanın baytlarını olduğu gibi okur,
// .gz'i açmaz.
func readBytesBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	return readWhole(y, codes.FnReadBytes, args, os.ReadFile, func(data []byte, _ string) (Value, error) {
		return NewBytes(data), nil
	})
}

// readWhole dosyayı bütün okuyan yerleşiklerin ortak yoludur (13.1a): yolu okuma sırasıyla çözer,
// dosyayı read ile okur, değeri build ile kurar (build'in yolu kullanıcının yazdığıdır). Okuma da
// değerin kurulması da (büyük dosyada kopya) kilit bırakılarak yapılır (10.2).
func readWhole(y *VM, kd codes.Code, args []Value, read func(string) ([]byte, error), build func(data []byte, filePath string) (Value, error)) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(kd, 1, len(args), messages.Text(messages.H2307))
	}
	filePath, ok, err := PathArg(CallPrefix(kd), args[0])
	if !ok {
		return nil, ArgTypeError(kd, 1, args[0], codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return nil, err
	}
	resolved, searched := y.ResolvePath(filePath)
	var v Value
	var buildErr error
	if waitErr := y.Abandonable(func() {
		var data []byte
		if data, err = read(resolved); err == nil {
			v, buildErr = build(data, filePath)
		}
	}); waitErr != nil {
		return nil, waitErr
	}
	if err != nil {
		return nil, ReadError(CallPrefix(kd), filePath, messages.H2100, err, searched)
	}
	return v, buildErr
}

// writeFileBuiltin dosya_yaz() yerleşiğidir (13.1): TR_String'i UTF-8 olarak, TR_Bytes'ı bayt bayt
// yazar; başka değerin metnini yazmaz, H2423 verir (JSON değeri H2623: TR_json_string'i önerir). Yazma
// atomiktir (bkz. AtomikYaz).
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
	var content []byte
	switch c := args[1].(type) {
	case *StringVal:
		content = []byte(c.Val)
	case *BytesVal:
		content = c.Data()
	case *JSONVal:
		// JSON değeri metin olarak açılmaz (TR_String(j) yalnız JSON metnini açar): JSON metni olarak yazılır.
		return nil, messages.New(messages.H2623, CallPrefix(codes.FnWriteFile), TypeAndSummary(c))
	default:
		return nil, ArgTypeError(codes.FnWriteFile, 2, args[1], codes.TypeString, codes.TypeBytes)
	}
	resolved, _ := y.ResolvePath(filePath)
	y.Blocking(func() { err = WriteFileContent(resolved, content) })
	if err != nil {
		return nil, turkish.FileError(CallPrefix(codes.FnWriteFile), filePath, messages.H2101, err)
	}
	return True, nil
}

// sleepBuiltin uyu() yerleşiğidir.
// maxSleepSeconds is the longest uyu(): the largest time.Duration, about 292 years.
const maxSleepSeconds = float64(math.MaxInt64) / float64(time.Second)

func sleepBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.FnSleep, 1, len(args), messages.Text(messages.H2309))
	}
	if _, ok := ToFloat(args[0]); !ok {
		return nil, ArgTypeError(codes.FnSleep, 1, args[0], numberTypes...)
	}
	d, err := seconds(codes.FnSleep, args[0])
	if err != nil {
		return nil, err
	}
	// Uygulanan bir durdurma beklemeyi bırakır; üstlenilmiş programda istek gelince bekleme
	// "durdurma istendi" hatasıyla döner (1.5, 1.6).
	if err := y.sleep(CallPrefix(codes.FnSleep), d); err != nil {
		return nil, err
	}
	return True, nil
}

// seconds is a duration given in seconds (uyu, bekle'nin süre_sınırı): a value that is not a number,
// a negative or NaN one is H2433, one beyond maxSleepSeconds H2434. A negative, NaN or huge
// duration used to return at once without an error (the conversion to time.Duration overflows).
func seconds(fn codes.Code, v Value) (time.Duration, error) {
	pick, ok := ToFloat(v)
	if !ok || math.IsNaN(pick) || pick < 0 {
		return 0, messages.New(messages.H2433, CallPrefix(fn), Shown(v))
	}
	if pick > maxSleepSeconds {
		return 0, messages.New(messages.H2434, CallPrefix(fn), Shown(v), strconv.FormatFloat(maxSleepSeconds, 'f', 0, 64))
	}
	return time.Duration(pick * float64(time.Second)), nil
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
