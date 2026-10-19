// Package formats, metin veri biçimlerinin (JSON ve CSV) TR yerleşiklerini içerir:
// json_çöz, json_metin ve csv_oku. Bu yerleşikler ne matematik/yapay
// zekâ ne de ağ işidir; ayrı bir paket olmaları yz ve web paketlerini kendi işlerine
// odaklı tutar. Sıralı JSON çözme çekirdekte (yurut.JSONToTR), sıralı JSON yazma
// motorların ortak paketinde (internal/jsonyaz) kalır, çünkü web sunucusu ve istemcisi de
// onları kullanır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package formats

import (
	"encoding/csv"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"os"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Kaydet JSON ve CSV yerleşiklerini VM'ye kaydeder.
func Kaydet(v *interp.VM) {
	v.RegisterNoNamed(codes.G600, func(makine *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G600, "1", len(args), codes.ReceiverName(codes.A202))
		}
		val, err := interp.JSONToTR([]byte(bomSoy(args[0].Inspect())))
		if err != nil {
			return nil, fmt.Errorf("JSON çözümleme hatası: %v", err)
		}
		return val, nil
	})

	v.RegisterNoNamed(codes.G601, func(makine *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G601, "1", len(args), "değer")
		}
		b, err := jsonwrite.Yaz(args[0])
		if err != nil {
			return nil, fmt.Errorf("%s: %v", codes.Name(codes.G601), err)
		}
		return &interp.StringVal{Val: string(b)}, nil
	})

	v.RegisterNoNamed(codes.G602, func(makine *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 1 {
			return nil, interp.ArgCountError(codes.G602, "1", len(args), "dosya ya da metin")
		}
		input, err := csvGirdisi(makine, args[0])
		if err != nil {
			return nil, err
		}
		return csvCoz(input)
	})
}

// csvGirdisi csv_oku argümanını ayrıştırılacak CSV metnine çevirir:
//   - Argüman bir Yol (yurut.YolDegerli) ise her zaman dosya okunur.
//   - Argüman var olan bir dosyanın yoluysa dosya okunur. Göreli yollar önce çalışma
//     klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür. .gz uzantılı dosya
//     açılarak okunur (bkz. yurut.DosyaIceriginiOku).
//   - Argüman yol gibi görünüyor ama öyle bir dosya yoksa "dosya yok" hatası
//     verilir (eskiden "yok.csv" metni tek hücreli bir tablo olarak sessizce dönüyordu).
//   - Aksi hâlde argüman CSV içeriğinin kendisi sayılır ve olduğu gibi ayrıştırılır
//     (ör. csv_oku("ad,yaş\nAli,30")); bu eski davranış bilerek korunmuştur.
func csvGirdisi(makine *interp.VM, d interp.Value) (string, error) {
	fn := codes.Name(codes.G602)
	yolDegeri, kesinYol := d.(interp.PathValuer)
	arg := d.Inspect()
	if kesinYol {
		arg = yolDegeri.FilePath()
	}
	yol, bakilanlar := bind.YolCozBakilan(makine, arg)
	bilgi, statErr := os.Stat(yol)
	if statErr == nil {
		if bilgi.IsDir() {
			return "", fmt.Errorf("%s: %q bir klasör; CSV dosyasının yolunu verin", fn, arg)
		}
		data, err := interp.ReadFileContent(yol)
		if err != nil {
			return "", turkish.FileError(fn, arg, "okunamadı", err)
		}
		return string(data), nil
	}
	if kesinYol || yolGibi(arg) {
		return "", interp.ReadError(fn, arg, "okunamadı", statErr, bakilanlar)
	}
	return arg, nil
}

// yolGibi argümanın CSV içeriği değil bir dosya yolu olarak yazıldığını tahmin eder.
// Tek satırlık olmalı ve ya .csv (ya da .csv.gz) uzantısıyla bitmeli ya da yol ayırıcı (/ veya \)
// içerip virgül içermemelidir. Virgül şartı "tarih,1/2/2024" gibi tek satırlık CSV
// içeriklerinin yol sanılmasını önler.
func yolGibi(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "\r\n") {
		return false
	}
	if k := strings.ToLower(s); strings.HasSuffix(k, ".csv") || strings.HasSuffix(k, ".csv.gz") {
		return true
	}
	return strings.ContainsAny(s, `/\`) && !strings.Contains(s, ",")
}

// bomSoy metnin başındaki UTF-8 bayt sırası işaretini (BOM) atar; bazı programlar CSV ve
// JSON dosyalarının başına bunu yazar ve yoksa ilk başlık görünmez bir karakter taşırdı.
func bomSoy(s string) string { return strings.TrimPrefix(s, "\ufeff") }

// csvCoz CSV metnini satır listelerine çevirir; hücreler boşlukları kırpılarak hücreDegeri
// kuralıyla sayıya ya da metne dönüşür.
func csvCoz(input string) (interp.Value, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(bomSoy(input))))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV okuma hatası: %v", err)
	}
	var rows []interp.Value
	for _, rec := range records {
		var cols []interp.Value
		for _, cell := range rec {
			cols = append(cols, hucreDegeri(strings.TrimSpace(cell)))
		}
		rows = append(rows, &interp.ListVal{Elements: cols})
	}
	return &interp.ListVal{Elements: rows}, nil
}

// hucreDegeri bir CSV hücresini yalnızca katı ondalık sayı biçimindeyse sayıya çevirir, yoksa
// metni olduğu gibi bırakır; böylece hiçbir hücre sessizce bozulmaz:
//   - Tam: isteğe bağlı '-' ve rakamlar, baştaki sıfır yok ("0", "42", "-7"), Tam aralığında.
//   - Ondalık: Tam biçimine '.rakamlar' ve/veya 'e±rakamlar' eki ("1.0", "2.5e3", "1E-3"),
//     sonlu sonuç.
//   - Geri kalan her şey Metin kalır: "007" ve "00.5" (posta kodu, kimlik), Tam aralığını
//     aşan "12345678901234567890", "NaN", "Inf", "infinity", "0x10", "1_000", "+5", ".5",
//     "1.", "1e400".
func hucreDegeri(h string) interp.Value {
	tamSon, ondalik := sayiBicimi(h)
	if tamSon < 0 {
		return &interp.StringVal{Val: h}
	}
	if !ondalik {
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

// sayiBicimi h'nin katı sayı biçiminde olup olmadığını söyler: biçim dışıysa -1, değilse
// tam kısmın bittiği konum ve sayının ondalık (nokta ya da üs içeren) olup olmadığı.
func sayiBicimi(h string) (int, bool) {
	i := 0
	if i < len(h) && h[i] == '-' {
		i++
	}
	bas := i
	for i < len(h) && h[i] >= '0' && h[i] <= '9' {
		i++
	}
	if i == bas || (h[bas] == '0' && i-bas > 1) {
		return -1, false
	}
	tamSon := i
	ondalik := false
	if i < len(h) && h[i] == '.' {
		i++
		k := i
		for i < len(h) && h[i] >= '0' && h[i] <= '9' {
			i++
		}
		if i == k {
			return -1, false
		}
		ondalik = true
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
		ondalik = true
	}
	if i != len(h) {
		return -1, false
	}
	return tamSon, ondalik
}
