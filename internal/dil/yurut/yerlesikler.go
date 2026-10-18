package yurut

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
)

// cekirdekYerlesikleri çekirdeğin adlı argüman almayan yerleşikleridir; kayıt sırası
// tablodaki sıradır. Adlar kod listesindedir (internal/dil/kod).
var cekirdekYerlesikleri = [...]struct {
	kd kod.Kod
	fn BuiltinFunc
}{
	{kod.G100, yazdirYerlesik},
	{kod.G101, girdiYerlesik},
	{kod.G201, tipYerlesik},
	{kod.G210, tamYerlesik},
	{kod.G211, ondalikYerlesik},
	{kod.G212, metinYerlesik},
	{kod.G305, aralikYerlesik},
	{kod.G400, toplamYerlesik},
	{kod.G401, minYerlesik},
	{kod.G402, maksYerlesik},
	{kod.G403, mutlakYerlesik},
	{kod.G404, karekokYerlesik},
	{kod.G405, yuvarlaYerlesik},
	{kod.G501, bekleHepsiYerlesik},
	{kod.G502, zamanMsYerlesik},
	{kod.G110, dosyaOkuYerlesik},
	{kod.G111, dosyaYazYerlesik},
	{kod.G500, bekleYerlesik},
}

func (v *VM) registerBuiltins() {
	for _, k := range cekirdekYerlesikleri {
		v.AdlisizKaydet(k.kd, k.fn)
	}
	v.tekArgumanli(kod.G212, metinTek)
}

// yazdirYerlesik yazdır() yerleşiğidir.
// Ekrana Yazdırma: yazdır(...)
func yazdirYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.Inspect())
	}
	out := strings.Join(parts, " ")
	y.CiktiYaz(y.Out, out+"\n")
	return &StringVal{Val: out}, nil
}

// girdiYerlesik girdi() yerleşiğidir.
// Kullanıcıdan Girdi Alma: girdi("Adınız: ")
func girdiYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) > 1 {
		return nil, ArgumanSayisiHatasi(kod.G101, "0 ya da 1", len(args), "", "istem")
	}
	if len(args) > 0 {
		y.CiktiYaz(y.Out, args[0].Inspect())
	}
	line, readErr := y.satirOku()
	if readErr != nil && readErr != io.EOF && line == "" {
		return nil, fmt.Errorf("%s: standart girdi okunamadı: %s", Cagri(kod.G101), turkce.DosyaHatasiNedeni("", readErr))
	}
	line = strings.TrimRight(line, "\r\n")
	return &StringVal{Val: line}, nil
}

// tipYerlesik tip() yerleşiğidir.
// Tip Sorgulama: tip(x)
func tipYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G201, "1", len(args), "değer")
	}
	return DegerinTipi(args[0]), nil
}

// tamYerlesik tam() yerleşiğidir.
// Tip Dönüşümleri: tam(x), ondalık(x), metin(x)
func tamYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G210, "1", len(args), "değer")
	}
	switch val := args[0].(type) {
	case *IntVal:
		return val, nil
	case *FloatVal:
		n, err := ondaliktanTam(Cagri(kod.G210), val.Val)
		if err != nil {
			return nil, err
		}
		return NewInt(n), nil
	case *StringVal:
		n, err := strconv.ParseInt(strings.TrimSpace(val.Val), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q tam sayıya çevrilemedi (yalnızca rakamlardan oluşan bir metin verin, ör. \"42\")", Cagri(kod.G210), val.Val)
		}
		return NewInt(n), nil
	case *BoolVal:
		if val.Val {
			return NewInt(1), nil
		}
		return NewInt(0), nil
	default:
		return nil, ArgumanTipHatasi(kod.G210, 1, args[0], kod.A200, kod.A201, kod.A202, kod.A203)
	}
}

// ondalikYerlesik ondalık() yerleşiğidir.
func ondalikYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G211, "1", len(args), "değer")
	}
	switch val := args[0].(type) {
	case *FloatVal:
		return val, nil
	case *IntVal:
		return &FloatVal{Val: float64(val.Val)}, nil
	case *StringVal:
		n, err := strconv.ParseFloat(strings.TrimSpace(val.Val), 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q ondalık sayıya çevrilemedi (ondalık ayırıcı nokta olmalı, ör. \"3.14\")", Cagri(kod.G211), val.Val)
		}
		return &FloatVal{Val: n}, nil
	default:
		return nil, ArgumanTipHatasi(kod.G211, 1, args[0], kod.A201, kod.A200, kod.A202)
	}
}

// metinYerlesik metin() yerleşiğidir.
func metinYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G212, "1", len(args), "değer")
	}
	return metinTek(y, args[0])
}

// aralikYerlesik aralık() yerleşiğidir.
// Liste: aralık(bitiş) ya da aralık(başlangıç, bitiş)
func aralikYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) == 1 {
		end, ok := args[0].(*IntVal)
		if !ok {
			return nil, ArgumanTipHatasi(kod.G305, 1, args[0], kod.A200)
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
			return nil, ArgumanTipHatasi(kod.G305, 1, args[0], kod.A200)
		}
		end, ok := args[1].(*IntVal)
		if !ok {
			return nil, ArgumanTipHatasi(kod.G305, 2, args[1], kod.A200)
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
	return nil, ArgumanSayisiHatasi(kod.G305, "1 ya da 2", len(args), "bitiş", "başlangıç, bitiş")
}

// toplamYerlesik toplam() yerleşiğidir.
// Temel Matematik Fonksiyonları
func toplamYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G400, "1", len(args), "liste")
	}
	list, ok := args[0].(*ListVal)
	if !ok {
		return nil, ArgumanTipHatasi(kod.G400, 1, args[0], kod.A205)
	}
	return sayiTopla(Cagri(kod.G400), list.Elements)
}

// minYerlesik min() yerleşiğidir.
// min/maks: tek bir liste ya da birden çok sayı alır; kazanan değerin kendisi döner
func minYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	return enUcArgumanlari(kod.G401, args, false)
}

// maksYerlesik maks() yerleşiğidir.
func maksYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	return enUcArgumanlari(kod.G402, args, true)
}

// mutlakYerlesik mutlak() yerleşiğidir.
func mutlakYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G403, "1", len(args), "sayı")
	}
	if iv, ok := args[0].(*IntVal); ok {
		if iv.Val == math.MinInt64 {
			m, o := kod.Ad(kod.G403), kod.Ad(kod.G211)
			return nil, fmt.Errorf("%s(): %s sayı taşması (%s(%d)); ondalık sonuç için %s(%s(x)) yazın", m, kod.Ad(kod.A200), m, iv.Val, m, o)
		}
		if iv.Val < 0 {
			return NewInt(-iv.Val), nil
		}
		return iv, nil
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Abs(fv)}, nil
	}
	return nil, ArgumanTipHatasi(kod.G403, 1, args[0], sayiTipleri...)
}

// karekokYerlesik karekök() yerleşiğidir.
func karekokYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G404, "1", len(args), "sayı")
	}
	if fv, ok := ToFloat(args[0]); ok {
		return &FloatVal{Val: math.Sqrt(fv)}, nil
	}
	return nil, ArgumanTipHatasi(kod.G404, 1, args[0], sayiTipleri...)
}

// yuvarlaYerlesik yuvarla() yerleşiğidir.
func yuvarlaYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, ArgumanSayisiHatasi(kod.G405, "1 ya da 2", len(args), "sayı", "sayı, basamak")
	}
	fv, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgumanTipHatasi(kod.G405, 1, args[0], sayiTipleri...)
	}
	digits := 0
	if len(args) == 2 {
		iv, ok := args[1].(*IntVal)
		if !ok {
			return nil, ArgumanTipHatasi(kod.G405, 2, args[1], kod.A200)
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
	n, err := ondaliktanTam(Cagri(kod.G405), yuvarlanan)
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
		return nil, fmt.Errorf("%s '%s' bloğu içinden çağrılamaz; ana programda kullanın", Cagri(kod.G501), kod.Ad(kod.A080))
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
		return nil, ArgumanSayisiHatasi(kod.G110, "1", len(args), "yol")
	}
	yol, ok, err := YolArgumani(Cagri(kod.G110), args[0])
	if !ok {
		return nil, ArgumanTipHatasi(kod.G110, 1, args[0], kod.A202)
	}
	if err != nil {
		return nil, err
	}
	cozulen, bakilanlar := y.YolCoz(yol)
	data, err := os.ReadFile(cozulen)
	if err != nil {
		return nil, OkumaHatasi(Cagri(kod.G110), yol, "okunamadı", err, bakilanlar)
	}
	return &StringVal{Val: string(data)}, nil
}

// dosyaYazYerlesik dosya_yaz() yerleşiğidir.
func dosyaYazYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 2 {
		return nil, ArgumanSayisiHatasi(kod.G111, "2", len(args), "yol, içerik")
	}
	yol, ok, err := YolArgumani(Cagri(kod.G111), args[0])
	if !ok {
		return nil, ArgumanTipHatasi(kod.G111, 1, args[0], kod.A202)
	}
	if err != nil {
		return nil, err
	}
	cozulen, _ := y.YolCoz(yol)
	if err := os.WriteFile(cozulen, []byte(args[1].Inspect()), 0644); err != nil {
		return nil, turkce.DosyaHatasi(Cagri(kod.G111), yol, "yazılamadı", err)
	}
	return TRUE, nil
}

// bekleYerlesik bekle() yerleşiğidir.
func bekleYerlesik(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgumanSayisiHatasi(kod.G500, "1", len(args), "saniye")
	}
	sec, ok := ToFloat(args[0])
	if !ok {
		return nil, ArgumanTipHatasi(kod.G500, 1, args[0], sayiTipleri...)
	}
	// Kesme istenirse (Ctrl+C) bekleme hemen biter; program sonraki güvenli noktada durur.
	y.Blocking(func() { y.kesilebilirUyu(time.Duration(sec * float64(time.Second))) })
	return TRUE, nil
}

// sayiTipleri sayı bekleyen yerleşiklerin kabul ettiği tiplerdir; sayiTipi hata
// mesajlarındaki adlarıdır ("Tam ya da Ondalık").
var (
	sayiTipleri = []kod.Kod{kod.A200, kod.A201}
	sayiTipi    = TipAdlari(sayiTipleri...)
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

// MaxCollectionSize tek bir yerleşik çağrının üretebileceği en fazla eleman sayısıdır (bellek taşmasını önler).
const MaxCollectionSize = 50_000_000

// metinTek metin(x) yerleşiğinin tek argümanlı gövdesidir.
func metinTek(_ *VM, a Value) (Value, error) {
	return &StringVal{Val: a.Inspect()}, nil
}

// tekArgumanli kayıtlı bir yerleşiğe, tek argümanla (adlı argümansız) çağrıldığında
// argüman dilimi ayırmadan çalışan gövdesini bağlar. Gövde, Fn'in tek argümanlı
// durumuyla birebir aynı olmalıdır.
func (v *VM) tekArgumanli(kd kod.Kod, tek func(*VM, Value) (Value, error)) {
	if b := v.Yerlesik(kd); b != nil {
		b.Tek = tek
	}
}

// checkRangeSize aralık(başlangıç, bitiş) için boyut denetimi yapar; boş aralık (bitiş <= başlangıç) geçerlidir.
func checkRangeSize(start, end int64) error {
	if end > start && (end-start > MaxCollectionSize || end-start < 0) {
		return fmt.Errorf("%s çok büyük: en fazla %d eleman üretilebilir", Cagri(kod.G305), MaxCollectionSize)
	}
	return nil
}

// enUcArgumanlari min/maks'ın argümanlarını açar: tek bir liste verilirse elemanları,
// değilse argümanların kendisi karşılaştırılır.
func enUcArgumanlari(kd kod.Kod, args []Value, buyuk bool) (Value, error) {
	if len(args) == 0 {
		return nil, ArgumanSayisiHatasi(kd, "en az 1", 0, "liste", "a, b, ...")
	}
	if len(args) == 1 {
		if list, ok := args[0].(*ListVal); ok {
			return enUc(kd, list.Elements, buyuk, true)
		}
	}
	return enUc(kd, args, buyuk, false)
}
