// Package formats, metin veri biçimlerinin (JSON ve CSV) TR yerleşiklerini içerir:
// json_çöz, json_metin, csv_oku ve csv_çöz. Bu yerleşikler ne matematik/yapay
// zekâ ne de ağ işidir; ayrı bir paket olmaları yz ve web paketlerini kendi işlerine
// odaklı tutar. Sıralı JSON çözme çekirdekte (interp.JSONToTR), sıralı JSON yazma
// motorların ortak paketinde (internal/stdlib/common/jsonwrite) kalır, çünkü web sunucusu ve istemcisi de
// onları kullanır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package formats

import (
	"encoding/csv"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Register JSON ve CSV yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterNoNamed(codes.G600, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G600, 1, len(args), codes.ReceiverName(codes.A202))
		}
		s, ok := args[0].(*interp.StringVal)
		if !ok {
			return nil, messages.New(messages.H7000, codes.NameRef(codes.G600), codes.NameRef(codes.A202), args[0].Type())
		}
		val, err := interp.JSONToTR([]byte(stripBOM(s.Val)))
		if err != nil {
			return nil, messages.New(messages.H7001, err)
		}
		return val, nil
	})

	v.RegisterNoNamed(codes.G601, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G601, 1, len(args), messages.Text(messages.H2301))
		}
		b, err := jsonwrite.Write(args[0])
		if err != nil {
			return nil, messages.New(messages.H2000, codes.NameRef(codes.G601), err)
		}
		return &interp.StringVal{Val: string(b)}, nil
	})

	v.RegisterNoNamed(codes.G602, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G602, 1, len(args), messages.Text(messages.H2307))
		}
		resolved, err := resolveCSVPath(vm, args[0])
		if err != nil {
			return nil, err
		}
		// Dosya okuma ve ayrıştırma yorumlayıcı kilidi bırakılarak yapılır (TR değerlerine
		// dokunmaz); değerler kilit geri alınınca kurulur.
		var records [][]string
		vm.Blocking(func() {
			var input string
			if input, err = csvInput(resolved); err == nil {
				records, err = parseCSV(codes.G602, input)
			}
		})
		if err != nil {
			return nil, err
		}
		return csvValues(records)
	})

	v.RegisterNoNamed(codes.G603, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G603, 1, len(args), codes.ReceiverName(codes.A202))
		}
		s, ok := args[0].(*interp.StringVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.G603, 1, args[0], codes.A202)
		}
		var records [][]string
		var err error
		vm.Blocking(func() { records, err = parseCSV(codes.G603, s.Val) })
		if err != nil {
			return nil, err
		}
		return csvValues(records)
	})
}

// csvInput csv_oku'nun dosyasını okur. Argüman her zaman bir dosya yoludur; göreli yollar
// önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür, .gz uzantılı
// dosya açılarak okunur (bkz. interp.ReadFileContent). CSV metninin kendisi csv_çöz ile
// ayrıştırılır: dosya yoksa ve argüman virgül içeriyorsa hata csv_çöz'ü önerir.
func csvInput(p csvPath) (string, error) {
	fn := codes.NameRef(codes.G602)
	info, statErr := os.Stat(p.path)
	if statErr != nil {
		err := interp.ReadError(fn, p.arg, messages.H2100, statErr, p.tried)
		if !p.isPath && strings.Contains(p.arg, ",") {
			return "", messages.New(messages.H7007, err, interp.CallPrefix(codes.G603))
		}
		return "", err
	}
	if info.IsDir() {
		return "", messages.New(messages.H7003, fn, p.arg)
	}
	data, err := interp.ReadFileContent(p.path)
	if err != nil {
		return "", turkish.FileError(fn, p.arg, messages.H2100, err)
	}
	return string(data), nil
}

// csvPath csv_oku argümanının dosya yolu olarak çözümüdür (VM'ye bakılarak, kilit altında).
type csvPath struct {
	arg, path string
	tried     []string
	isPath    bool // argüman bir Yol
}

// resolveCSVPath argümanı öteki dosya yerleşikleri gibi okur (Metin ya da Yol, interp.PathArg).
// Satır sonu içeren metin bir yol değil CSV metnidir: sekme denetiminden önce csv_çöz önerilir.
func resolveCSVPath(vm *interp.VM, d interp.Value) (csvPath, error) {
	if s, ok := d.(*interp.StringVal); ok && strings.ContainsAny(s.Val, "\r\n") {
		return csvPath{}, messages.New(messages.H7006, interp.CallPrefix(codes.G602), interp.CallPrefix(codes.G603))
	}
	arg, ok, err := interp.PathArg(interp.CallPrefix(codes.G602), d)
	if !ok {
		return csvPath{}, interp.ArgTypeError(codes.G602, 1, d, codes.A202, codes.A250)
	}
	if err != nil {
		return csvPath{}, err
	}
	_, isPath := d.(interp.PathValuer)
	path, tried := bind.ResolvePathTried(vm, arg)
	return csvPath{arg: arg, path: path, tried: tried, isPath: isPath}, nil
}

// stripBOM metnin başındaki UTF-8 bayt sırası işaretini (BOM) atar; bazı programlar CSV ve
// JSON dosyalarının başına bunu yazar ve yoksa ilk başlık görünmez bir karakter taşırdı.
func stripBOM(s string) string { return strings.TrimPrefix(s, "\ufeff") }

// parseCSV CSV metnini kayıtlara ayırır (TR değerlerine dokunmaz; kilit bırakılmışken
// çalışabilir). Satır sayısı ve toplam hücre sayısı koleksiyon sınırındadır.
func parseCSV(kd codes.Code, input string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(stripBOM(input))))
	r.ReuseRecord = false
	var records [][]string
	cells := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return records, nil
		}
		if err != nil {
			return nil, messages.New(messages.H7004, err)
		}
		if cells += len(rec); cells > interp.MaxCollectionSize {
			return nil, messages.New(messages.H7005, codes.NameRef(kd), interp.MaxCollectionSize)
		}
		records = append(records, rec)
	}
}

// csvValues kayıtları satır listelerine çevirir; hücreler boşlukları kırpılarak cellValue
// kuralıyla sayıya ya da metne dönüşür.
func csvValues(records [][]string) (interp.Value, error) {
	rows := make([]interp.Value, 0, len(records))
	for _, rec := range records {
		cols := make([]interp.Value, 0, len(rec))
		for _, cell := range rec {
			cols = append(cols, cellValue(strings.TrimSpace(cell)))
		}
		rows = append(rows, &interp.ListVal{Elements: cols})
	}
	return &interp.ListVal{Elements: rows}, nil
}

// decodeCSV CSV metnini satır listelerine çevirir (parseCSV + csvValues).
func decodeCSV(input string) (interp.Value, error) {
	records, err := parseCSV(codes.G603, input)
	if err != nil {
		return nil, err
	}
	return csvValues(records)
}

// cellValue bir CSV hücresini yalnızca katı ondalık sayı biçimindeyse sayıya çevirir, yoksa
// metni olduğu gibi bırakır; böylece hiçbir hücre sessizce bozulmaz:
//   - Tam: isteğe bağlı '-' ve rakamlar, baştaki sıfır yok ("0", "42", "-7"), Tam aralığında.
//   - Ondalık: Tam biçimine '.rakamlar' ve/veya 'e±rakamlar' eki ("1.0", "2.5e3", "1E-3"),
//     sonlu sonuç.
//   - Geri kalan her şey Metin kalır: "007" ve "00.5" (posta kodu, kimlik), Tam aralığını
//     aşan "12345678901234567890", "NaN", "Inf", "infinity", "0x10", "1_000", "+5", ".5",
//     "1.", "1e400".
func cellValue(h string) interp.Value {
	intEnd, float := numberFormat(h)
	if intEnd < 0 {
		return &interp.StringVal{Val: h}
	}
	if !float {
		if iv, err := strconv.ParseInt(h, 10, 64); err == nil {
			return &interp.IntVal{Val: iv}
		}
		return &interp.StringVal{Val: h}
	}
	if fv, err := strconv.ParseFloat(h, 64); err == nil {
		return &interp.FloatVal{Val: fv}
	}
	return &interp.StringVal{Val: h}
}

// numberFormat h'nin katı sayı biçiminde olup olmadığını söyler: biçim dışıysa -1, değilse
// tam kısmın bittiği konum ve sayının ondalık (nokta ya da üs içeren) olup olmadığı.
func numberFormat(h string) (int, bool) {
	i := 0
	if i < len(h) && h[i] == '-' {
		i++
	}
	start := i
	for i < len(h) && h[i] >= '0' && h[i] <= '9' {
		i++
	}
	if i == start || (h[start] == '0' && i-start > 1) {
		return -1, false
	}
	intEnd := i
	float := false
	if i < len(h) && h[i] == '.' {
		i++
		k := i
		for i < len(h) && h[i] >= '0' && h[i] <= '9' {
			i++
		}
		if i == k {
			return -1, false
		}
		float = true
	}
	if i < len(h) && (h[i] == 'e' || h[i] == 'E') {
		i++
		if i < len(h) && (h[i] == '+' || h[i] == '-') {
			i++
		}
		k := i
		for i < len(h) && h[i] >= '0' && h[i] <= '9' {
			i++
		}
		if i == k {
			return -1, false
		}
		float = true
	}
	if i != len(h) {
		return -1, false
	}
	return intEnd, float
}
