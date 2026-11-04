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
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Büyük girdide JSON çözme ve yazma yorumlayıcı kilidi bırakılarak yapılır (10.2): girdi
// değişmezdir (metin, TR_JSON), iş yalnız yeni değer kurar, paylaşılan duruma dokunmaz. Eşikler
// ~1 ms'lik iştir (2026-11-01 ölçümü: çözme ~120 ms/MB, yazma kayıt başına ~2 µs).
const (
	longJSONBytes = 8 << 10 // TR_json_parse: metnin bayt sayısı
	longJSONNodes = 500     // TR_json_string: TR_JSON değerinin düğüm sayısı
)

// jsonParse TR_json_parse yerleşiğidir.
func jsonParse(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnJsonParse, 1, len(args), codes.ReceiverRef(codes.TypeString))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, messages.New(messages.H7000, codes.NameRef(codes.FnJsonParse), codes.NameRef(codes.TypeString), args[0].Type())
	}
	var val interp.Value
	var err error
	parse := func() { val, err = interp.JSONToTR([]byte(stripBOM(s.Val))) }
	if len(s.Val) >= longJSONBytes {
		vm.Blocking(parse)
	} else {
		parse()
	}
	if err != nil {
		return nil, messages.New(messages.H7001, err)
	}
	return interp.NewJSON(val), nil
}

// jsonString TR_json_string yerleşiğidir. Girdi büyük bir TR_JSON ise kilit bırakılır; liste,
// sözlük ve yapı değişebilir (başka bir görev yazarken okunamaz), onlarda kilit tutulur: listenin
// ve sözlüğün metotları gibi (10.2'nin istisnası).
func jsonString(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnJsonString, 1, len(args), messages.Text(messages.H2301))
	}
	var b []byte
	var err error
	write := func() { b, err = jsonwrite.Write(args[0]) }
	if j, ok := args[0].(*interp.JSONVal); ok && nodesLeft(j.Unwrap(), longJSONNodes) <= 0 {
		vm.Blocking(write)
	} else {
		write()
	}
	if err != nil {
		return nil, messages.New(messages.H2000, codes.NameRef(codes.FnJsonString), err)
	}
	return &interp.StringVal{Val: string(b)}, nil
}

// nodesLeft değerin düğümlerini sayarak bütçeden düşer ve kalanı verir; bütçe bitince sayım durur
// (sıfır ya da eksi: değer en az bütçe kadar düğümlüdür).
func nodesLeft(v interp.Value, budget int) int {
	budget--
	switch x := v.(type) {
	case *interp.ListVal:
		for _, e := range x.Elements {
			if budget <= 0 {
				break
			}
			budget = nodesLeft(e, budget)
		}
	case *interp.DictVal:
		for _, k := range x.Order {
			if budget <= 0 {
				break
			}
			budget = nodesLeft(x.Pairs[k], budget)
		}
	case *interp.JSONVal:
		return nodesLeft(x.Unwrap(), budget+1)
	}
	return budget
}

// Register JSON ve CSV yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterNoNamed(codes.FnJsonParse, jsonParse)
	v.RegisterNoNamed(codes.FnJsonString, jsonString)

	v.RegisterNoNamed(codes.FnReadCsv, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.FnReadCsv, 1, len(args), messages.Text(messages.H2307))
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
				records, err = parseCSV(codes.FnReadCsv, input)
			}
		})
		if err != nil {
			return nil, err
		}
		return csvValues(records)
	})

	v.RegisterNoNamed(codes.FnParseCsv, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.FnParseCsv, 1, len(args), codes.ReceiverRef(codes.TypeString))
		}
		s, ok := args[0].(*interp.StringVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.FnParseCsv, 1, args[0], codes.TypeString)
		}
		var records [][]string
		var err error
		vm.Blocking(func() { records, err = parseCSV(codes.FnParseCsv, s.Val) })
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
	fn := codes.NameRef(codes.FnReadCsv)
	info, statErr := os.Stat(p.path)
	if statErr != nil {
		err := interp.ReadError(fn, p.arg, messages.H2100, statErr, p.tried)
		if !p.isPath && strings.Contains(p.arg, ",") {
			return "", messages.New(messages.H7007, err, interp.CallPrefix(codes.FnParseCsv))
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
		return csvPath{}, messages.New(messages.H7006, interp.CallPrefix(codes.FnReadCsv), interp.CallPrefix(codes.FnParseCsv))
	}
	arg, ok, err := interp.PathArg(interp.CallPrefix(codes.FnReadCsv), d)
	if !ok {
		return csvPath{}, interp.ArgTypeError(codes.FnReadCsv, 1, d, codes.TypeString, codes.TypePath)
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

// csvValues kayıtları satır listelerine çevirir: hücre hep metindir (13.8), boşlukları kırpılır;
// sayı gerekirse program tip çağrısıyla çevirir (TR_Int(hücre), 3.12). Eskiden katı sayı biçimindeki
// hücre kendiliğinden sayı oluyordu: aynı sütun satırdan satıra tip değiştiriyordu ("007" metin, "7" sayı).
func csvValues(records [][]string) (interp.Value, error) {
	rows := make([]interp.Value, 0, len(records))
	for _, rec := range records {
		cols := make([]interp.Value, 0, len(rec))
		for _, cell := range rec {
			cols = append(cols, &interp.StringVal{Val: strings.TrimSpace(cell)})
		}
		rows = append(rows, &interp.ListVal{Elements: cols})
	}
	return &interp.ListVal{Elements: rows}, nil
}

// decodeCSV CSV metnini satır listelerine çevirir (parseCSV + csvValues).
func decodeCSV(input string) (interp.Value, error) {
	records, err := parseCSV(codes.FnParseCsv, input)
	if err != nil {
		return nil, err
	}
	return csvValues(records)
}
