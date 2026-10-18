// Package veribicim, metin veri biçimlerinin (JSON ve CSV) TR yerleşiklerini içerir:
// json_çöz, json_metin ve csv_oku (eş yazımlarıyla). Bu yerleşikler ne matematik/yapay
// zekâ ne de ağ işidir; ayrı bir paket olmaları yz ve web paketlerini kendi işlerine
// odaklı tutar. Sıralı JSON çözme ve TR→Go dönüşümü çekirdekte (vm.JSONToTR,
// vm.TRToGoValue) kalır, çünkü web sunucusu ve veritabanı da onları kullanır.
package veribicim

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"tr_dili/pkg/sozluk"

	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Kaydet JSON ve CSV yerleşiklerini VM'ye kaydeder.
func Kaydet(v *vm.VM) {
	v.RegisterBuiltinKod(sozluk.G+600, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_çöz(metin) 1 parametre bekler")
		}
		val, err := vm.JSONToTR([]byte(args[0].Inspect()))
		if err != nil {
			return nil, fmt.Errorf("JSON çözümleme hatası: %v", err)
		}
		return val, nil
	})

	v.RegisterBuiltinKod(sozluk.G+601, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_metin(değer) 1 parametre bekler")
		}
		b, err := json.Marshal(vm.TRToGoValue(args[0]))
		if err != nil {
			return nil, err
		}
		return &vm.StringVal{Val: string(b)}, nil
	})

	v.RegisterBuiltinKod(sozluk.G+602, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("csv_oku(dosya_veya_metin) 1 parametre bekler")
		}
		input, err := csvGirdisi(makine, args[0].Inspect())
		if err != nil {
			return nil, err
		}
		return csvCoz(input)
	})
}

// csvGirdisi csv_oku argümanını ayrıştırılacak CSV metnine çevirir:
//   - Argüman var olan bir dosyanın yoluysa dosya okunur. Göreli yollar önce çalışma
//     klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür.
//   - Argüman yol gibi görünüyor ama öyle bir dosya yoksa "dosya yok" hatası
//     verilir (eskiden "yok.csv" metni tek hücreli bir tablo olarak sessizce dönüyordu).
//   - Aksi hâlde argüman CSV içeriğinin kendisi sayılır ve olduğu gibi ayrıştırılır
//     (ör. csv_oku("ad,yaş\nAli,30")); bu eski davranış bilerek korunmuştur.
func csvGirdisi(makine *vm.VM, arg string) (string, error) {
	yol, bakilanlar := baglama.YolCozBakilan(makine, arg)
	bilgi, statErr := os.Stat(yol)
	if statErr == nil {
		if bilgi.IsDir() {
			return "", fmt.Errorf("csv_oku: %q bir klasör; CSV dosyasının yolunu verin", arg)
		}
		data, err := os.ReadFile(yol)
		if err != nil {
			return "", turkce.DosyaHatasi("csv_oku", arg, "okunamadı", err)
		}
		return string(data), nil
	}
	if yolGibi(arg) {
		return "", vm.OkumaHatasi("csv_oku", arg, "okunamadı", statErr, bakilanlar)
	}
	return arg, nil
}

// yolGibi argümanın CSV içeriği değil bir dosya yolu olarak yazıldığını tahmin eder.
// Tek satırlık olmalı ve ya .csv uzantısıyla bitmeli ya da yol ayırıcı (/ veya \)
// içerip virgül içermemelidir. Virgül şartı "tarih,1/2/2024" gibi tek satırlık CSV
// içeriklerinin yol sanılmasını önler.
func yolGibi(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "\r\n") {
		return false
	}
	if strings.HasSuffix(strings.ToLower(s), ".csv") {
		return true
	}
	return strings.ContainsAny(s, `/\`) && !strings.Contains(s, ",")
}

// csvCoz CSV metnini satır listelerine çevirir; tam sayı ve ondalık görünen hücreler
// sayıya, diğerleri (boşlukları kırpılmış) metne dönüşür.
func csvCoz(input string) (vm.Value, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(input)))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV okuma hatası: %v", err)
	}
	var rows []vm.Value
	for _, rec := range records {
		var cols []vm.Value
		for _, cell := range rec {
			cell = strings.TrimSpace(cell)
			if iv, err := strconv.ParseInt(cell, 10, 64); err == nil {
				cols = append(cols, &vm.IntVal{Val: iv})
			} else if fv, err := strconv.ParseFloat(cell, 64); err == nil {
				cols = append(cols, &vm.FloatVal{Val: fv})
			} else {
				cols = append(cols, &vm.StringVal{Val: cell})
			}
		}
		rows = append(rows, &vm.ListVal{Elements: cols})
	}
	return &vm.ListVal{Elements: rows}, nil
}
