package interp

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// cekirdekYerlesikleri çekirdeğin adlı argüman almayan yerleşikleridir; kayıt sırası
// tablodaki sıradır. Adlar kod listesindedir (internal/lang/codes).
var cekirdekYerlesikleri = [...]struct {
	kd codes.Code
	fn BuiltinFunc
}{
	{codes.G100, yazdirYerlesik},
	{codes.G101, girdiYerlesik},
	{codes.G201, tipYerlesik},
	{codes.G210, tamYerlesik},
	{codes.G211, ondalikYerlesik},
	{codes.G212, metinYerlesik},
	{codes.G305, aralikYerlesik},
	{codes.G400, toplamYerlesik},
	{codes.G401, minYerlesik},
	{codes.G402, maksYerlesik},
	{codes.G403, mutlakYerlesik},
	{codes.G404, karekokYerlesik},
	{codes.G405, yuvarlaYerlesik},
	{codes.G501, bekleHepsiYerlesik},
	{codes.G502, zamanMsYerlesik},
	{codes.G110, dosyaOkuYerlesik},
	{codes.G111, dosyaYazYerlesik},
	{codes.G500, bekleYerlesik},
}

func (v *VM) registerBuiltins() {
	for _, k := range cekirdekYerlesikleri {
		v.RegisterNoNamed(k.kd, k.fn)
	}
	v.tekArgumanli(codes.G212, metinTek)
}

// yazdirYerlesik yazdır() yerleşiğidir.
// Ekrana Yazdırma: yazdır(...)
func yazdirYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.Inspect())
	}
	out := strings.Join(parts, " ")
	y.WriteOutput(y.Out, out+"\n")
	return &StringVal{Val: out}, nil
}

// girdiYerlesik girdi() yerleşiğidir.
// Kullanıcıdan Girdi Alma: girdi("Adınız: ")
func girdiYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) > 1 {
		return nil, ArgCountError(codes.G101, "0 ya da 1", len(args), "", "istem")
	}
	if len(args) > 0 {
		y.WriteOutput(y.Out, args[0].Inspect())
	}
	line, readErr := y.satirOku()
	if readErr != nil && readErr != io.EOF && line == "" {
		return nil, fmt.Errorf("%s: standart girdi okunamadı: %s", CallPrefix(codes.G101), turkish.FileErrorReason("", readErr))
	}
	line = strings.TrimRight(line, "\r\n")
	return &StringVal{Val: line}, nil
}

// tipYerlesik tip() yerleşiğidir.
// Tip Sorgulama: tip(x)
func tipYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G201, "1", len(args), "değer")
	}
	return TypeOfValue(args[0]), nil
}

// tamYerlesik tam() yerleşiğidir.
// Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
func tamYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G210, "1", len(args), "değer")
	}
	switch val := args[0].(type) {
	case *IntVal:
		return val, nil
	case *FloatVal:
		n, err := ondaliktanTam(CallPrefix(codes.G210), val.Val)
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

// ondalikYerlesik ondalık() yerleşiğidir.
func ondalikYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
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

// metinYerlesik metin() yerleşiğidir.
func metinYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G212, "1", len(args), "değer")
	}
	return metinTek(y, args[0])
}

// aralikYerlesik aralık() yerleşiğidir.
// Liste: aralık(bitiş) ya da aralık(başlangıç, bitiş)
func aralikYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
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

// toplamYerlesik toplam() yerleşiğidir.
// Temel Matematik Fonksiyonları
func toplamYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G400, "1", len(args), "liste")
	}
	list, ok := args[0].(*ListVal)
	if !ok {
		return nil, ArgTypeError(codes.G400, 1, args[0], codes.A205)
	}
	return sayiTopla(CallPrefix(codes.G400), list.Elements)
}

// minYerlesik min() yerleşiğidir.
// min/maks: tek bir liste ya da birden çok sayı alır; kazanan değerin kendisi döner
func minYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	return enUcArgumanlari(codes.G401, args, false)
}

// maksYerlesik maks() yerleşiğidir.
func maksYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	return enUcArgumanlari(codes.G402, args, true)
}

// mutlakYerlesik mutlak() yerleşiğidir.
func mutlakYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
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
	return nil, ArgTypeError(codes.G403, 1, args[0], sayiTipleri...)
}

// karekokYerlesik karekök() yerleşiğidir.
func karekokYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G404, "1", len(args), "sayı")
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Sqrt(fv)}, nil
	}
	return nil, ArgTypeError(codes.G404, 1, args[0], sayiTipleri...)
}

// yuvarlaYerlesik yuvarla() yerleşiğidir.
func yuvarlaYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, ArgCountError(codes.G405, "1 ya da 2", len(args), "sayı", "sayı, basamak")
	}
	fv, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.G405, 1, args[0], sayiTipleri...)
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
	yuvarlanan := math.Round(fv)
	if digits < 0 {
		pow := math.Pow(10, float64(-digits))
		yuvarlanan = math.Round(fv/pow) * pow
	}
	n, err := ondaliktanTam(CallPrefix(codes.G405), yuvarlanan)
	if err != nil {
		return nil, err
	}
	return NewInt(n), nil
}

// bekleHepsiYerlesik bekle_hepsi() yerleşiğidir.
// Eşzamanlılık, Zaman ve Dosya İşlemleri
func bekleHepsiYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	// Görev kendini de beklediği için sonsuza dek kilitlenirdi (deadlock)
	if y.asyncTask {
		return nil, fmt.Errorf("%s '%s' bloğu içinden çağrılamaz; ana programda kullanın", CallPrefix(codes.G501), codes.Name(codes.A080))
	}
	y.WaitAsync()
	return TRUE, nil
}

// zamanMsYerlesik zaman_ms() yerleşiğidir.
func zamanMsYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	return NewInt(time.Now().UnixMilli()), nil
}

// dosyaOkuYerlesik dosya_oku() yerleşiğidir.
func dosyaOkuYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G110, "1", len(args), "dosya")
	}
	yol, ok, err := PathArg(CallPrefix(codes.G110), args[0])
	if !ok {
		return nil, ArgTypeError(codes.G110, 1, args[0], codes.A202, codes.A250)
	}
	if err != nil {
		return nil, err
	}
	cozulen, bakilanlar := y.ResolvePath(yol)
	var data []byte
	y.Blocking(func() { data, err = ReadFileContent(cozulen) })
	if err != nil {
		return nil, ReadError(CallPrefix(codes.G110), yol, "okunamadı", err, bakilanlar)
	}
	return &StringVal{Val: string(data)}, nil
}

// dosyaYazYerlesik dosya_yaz() yerleşiğidir. Yazma atomiktir (bkz. AtomikYaz).
func dosyaYazYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 2 {
		return nil, ArgCountError(codes.G111, "2", len(args), "yol, içerik")
	}
	yol, ok, err := PathArg(CallPrefix(codes.G111), args[0])
	if !ok {
		return nil, ArgTypeError(codes.G111, 1, args[0], codes.A202, codes.A250)
	}
	if err != nil {
		return nil, err
	}
	cozulen, _ := y.ResolvePath(yol)
	icerik := []byte(args[1].Inspect())
	y.Blocking(func() { err = WriteFileContent(cozulen, icerik) })
	if err != nil {
		return nil, turkish.FileError(CallPrefix(codes.G111), yol, "yazılamadı", err)
	}
	return TRUE, nil
}

// bekleYerlesik bekle() yerleşiğidir.
func bekleYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G500, "1", len(args), "saniye")
	}
	sec, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgTypeError(codes.G500, 1, args[0], sayiTipleri...)
	}
	// Kesme istenirse (Ctrl+C) bekleme hemen biter; program sonraki güvenli noktada durur.
	y.Blocking(func() { y.kesilebilirUyu(time.Duration(sec * float64(time.Second))) })
	return TRUE, nil
}

// sayiTipleri sayı bekleyen yerleşiklerin kabul ettiği tiplerdir; sayiTipi hata
// mesajlarındaki adlarıdır ("Tam ya da Ondalık").
var (
	sayiTipleri = []codes.Code{codes.A200, codes.A201}
	sayiTipi    = TypeNames(sayiTipleri...)
)

// cagrilabilir değerin filtrele/haritala/indirge gibi metotlara fonksiyon olarak
// verilebilir olup olmadığını söyler.
func cagrilabilir(v Value) bool {
	switch v.(type) {
	case *FuncVal, *BuiltinVal, *StructDefVal:
		return true
	}
	return false
}

// MaxCollectionSize is the largest number of elements a single builtin call may produce (it
// guards against running out of memory).
const MaxCollectionSize = 50_000_000

// metinTek metin(x) yerleşiğinin tek argümanlı gövdesidir.
func metinTek(_ *VM, a Value) (Value, error) {
	return &StringVal{Val: a.Inspect()}, nil
}

// tekArgumanli kayıtlı bir yerleşiğe, tek argümanla (adlı argümansız) çağrıldığında
// argüman dilimi ayırmadan çalışan gövdesini bağlar. Gövde, Fn'in tek argümanlı
// durumuyla birebir aynı olmalıdır.
func (v *VM) tekArgumanli(kd codes.Code, tek func(*VM, Value) (Value, error)) {
	if b := v.Builtin(kd); b != nil {
		b.Single = tek
	}
}

// checkRangeSize aralık(başlangıç, bitiş) için boyut denetimi yapar; boş aralık (bitiş <= başlangıç) geçerlidir.
func checkRangeSize(start, end int64) error {
	if end > start && (end-start > MaxCollectionSize || end-start < 0) {
		return fmt.Errorf("%s çok büyük: en fazla %d eleman üretilebilir", CallPrefix(codes.G305), MaxCollectionSize)
	}
	return nil
}

// enUcArgumanlari min/maks'ın argümanlarını açar: tek bir liste verilirse elemanları,
// değilse argümanların kendisi karşılaştırılır.
func enUcArgumanlari(kd codes.Code, args []Value, buyuk bool) (Value, error) {
	if len(args) == 0 {
		return nil, ArgCountError(kd, "en az 1", 0, "liste", "a, b, ...")
	}
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			return enUc(kd, list.Elements, buyuk, true)
		}
	}
	return enUc(kd, args, buyuk, false)
}
