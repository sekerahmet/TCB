// Package veribicim, metin veri biçimlerinin (JSON ve CSV) TR yerleşiklerini içerir:
// json_çöz, json_metin ve csv_oku. Bu yerleşikler ne matematik/yapay
// zekâ ne de ağ işidir; ayrı bir paket olmaları yz ve web paketlerini kendi işlerine
// odaklı tutar. Sıralı JSON çözme çekirdekte (vm.JSONToTR), sıralı JSON yazma
// motorların ortak paketinde (internal/jsonyaz) kalır, çünkü web sunucusu ve istemcisi de
// onları kullanır.
package veribicim

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/vm"
	"tr_dili/internal/motor/ortak/baglama"
	"tr_dili/internal/motor/ortak/jsonyaz"
)

// Kaydet JSON ve CSV yerleşiklerini VM'ye kaydeder.
func Kaydet(v *vm.VM) {
	v.AdlisizKaydet(kod.G600, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_çöz(metin) 1 parametre bekler")
		}
		val, err := vm.JSONToTR([]byte(bomSoy(args[0].Inspect())))
		if err != nil {
			return nil, fmt.Errorf("JSON çözümleme hatası: %v", err)
		}
		return val, nil
	})

	v.AdlisizKaydet(kod.G601, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_metin(değer) 1 parametre bekler")
		}
		b, err := jsonyaz.Yaz(args[0])
		if err != nil {
			return nil, fmt.Errorf("json_metin: %v", err)
		}
		return &vm.StringVal{Val: string(b)}, nil
	})

	v.AdlisizKaydet(kod.G602, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

// bomSoy metnin başındaki UTF-8 bayt sırası işaretini (BOM) atar; bazı programlar CSV ve
// JSON dosyalarının başına bunu yazar ve yoksa ilk başlık görünmez bir karakter taşırdı.
func bomSoy(s string) string { return strings.TrimPrefix(s, "\ufeff") }

// csvCoz CSV metnini satır listelerine çevirir; hücreler boşlukları kırpılarak hücreDegeri
// kuralıyla sayıya ya da metne dönüşür.
func csvCoz(input string) (vm.Value, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(bomSoy(input))))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV okuma hatası: %v", err)
	}
	var rows []vm.Value
	for _, rec := range records {
		var cols []vm.Value
		for _, cell := range rec {
			cols = append(cols, hucreDegeri(strings.TrimSpace(cell)))
		}
		rows = append(rows, &vm.ListVal{Elements: cols})
	}
	return &vm.ListVal{Elements: rows}, nil
}

// hucreDegeri bir CSV hücresini yalnızca katı ondalık sayı biçimindeyse sayıya çevirir, yoksa
// metni olduğu gibi bırakır; böylece hiçbir hücre sessizce bozulmaz:
//   - Tam: isteğe bağlı '-' ve rakamlar, baştaki sıfır yok ("0", "42", "-7"), Tam aralığında.
//   - Ondalık: Tam biçimine '.rakamlar' ve/veya 'e±rakamlar' eki ("1.0", "2.5e3", "1E-3"),
//     sonlu sonuç.
//   - Geri kalan her şey Metin kalır: "007" ve "00.5" (posta kodu, kimlik), Tam aralığını
//     aşan "12345678901234567890", "NaN", "Inf", "infinity", "0x10", "1_000", "+5", ".5",
//     "1.", "1e400".
func hucreDegeri(h string) vm.Value {
	tamSon, ondalik := sayiBicimi(h)
	if tamSon < 0 {
		return &vm.StringVal{Val: h}
	}
	if !ondalik {
		if iv, err := strconv.ParseInt(h, 10, 64); err == nil {
			return &vm.IntVal{Val: iv}
		}
		return &vm.StringVal{Val: h}
	}
	if fv, err := strconv.ParseFloat(h, 64); err == nil {
		return &vm.FloatVal{Val: fv}
	}
	return &vm.StringVal{Val: h}
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
