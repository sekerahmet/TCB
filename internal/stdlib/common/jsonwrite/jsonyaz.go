// Package jsonwrite TR değerlerini JSON metnine yazan tek yerdir: json_metin, web sunucusunun
// bunu kullanır. Go'nun map tabanlı yazımından farkları:
//   - Sözlük anahtarları sözlükteki sırayla yazılır (json_metin(json_çöz('{"b":1,"a":2}'))
//     '{"b":1,"a":2}' verir).
//   - Döngüsel ya da çok derin değerler yığın taşması yerine Türkçe hata verir.
//   - JSON'da karşılığı olmayan Tanımsız (NaN) ve sonsuz sayılar Türkçe hata verir.
//   - Metin olmayan anahtar görünen hâliyle yazılır (1 → "1"); bu, aynı sözlükteki "1"
//     metin anahtarıyla çakışırsa sessizce biri atılmaz, hata verilir.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package jsonwrite

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Yaz değeri boşluksuz JSON olarak yazar.
func Yaz(val interp.Value) ([]byte, error) { return YazGirintili(val, "") }

// YazGirintili değeri her seviyede girinti kadar içeri alarak yazar; girinti boşsa
// boşluksuz yazar.
func YazGirintili(val interp.Value, girinti string) ([]byte, error) {
	return YazSecenekli(val, Secenekler{Girinti: girinti})
}

// Secenekler yazımı ayarlar; sıfır değeri Yaz ile aynıdır.
type Secenekler struct {
	// Girinti her seviyede eklenen girintidir; boşsa boşluksuz yazılır.
	Girinti string
	// SayiDisiMetin Tanımsız (NaN) ve sonsuz sayıları hata yerine TR'deki yazımlarıyla metin
	// olarak yazar ("NaN", "+Sonsuz", "-Sonsuz"); eğitim günlüğünde kayıpta beliren NaN
	// gizlenmesin diye.
	SayiDisiMetin bool
	// Metin yazılan her metne (sözlük anahtarları dahil) uygulanır, ör. gizli değer süzgeci;
	// nil ise metinler olduğu gibi yazılır.
	Metin func(string) string
}

// YazSecenekli değeri verilen seçeneklerle yazar.
func YazSecenekli(val interp.Value, s Secenekler) ([]byte, error) {
	y := yazici{girinti: s.Girinti, sayiDisiMetin: s.SayiDisiMetin, metinSuzgeci: s.Metin}
	if err := y.deger(val, 0); err != nil {
		return nil, err
	}
	return y.b, nil
}

type yazici struct {
	b             []byte
	girinti       string
	sayiDisiMetin bool
	metinSuzgeci  func(string) string
	gezgin        interp.ContainerWalker
}

func (y *yazici) satir(derinlik int) {
	if y.girinti == "" {
		return
	}
	y.b = append(y.b, '\n')
	for i := 0; i < derinlik; i++ {
		y.b = append(y.b, y.girinti...)
	}
}

func (y *yazici) metin(s string) {
	if y.metinSuzgeci != nil {
		s = y.metinSuzgeci(s)
	}
	b, _ := json.Marshal(s) // metin her zaman yazılabilir
	y.b = append(y.b, b...)
}

func (y *yazici) sayi(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if y.sayiDisiMetin {
			y.metin((&interp.FloatVal{Val: f}).Inspect())
			return nil
		}
		return fmt.Errorf("%s sayısı JSON'a yazılamaz (JSON'da Tanımsız ya da sonsuz sayı yoktur)", (&interp.FloatVal{Val: f}).Inspect())
	}
	// encoding/json ile aynı biçim: küçük ve çok büyük sayılarda bilimsel yazım
	bicim := byte('f')
	if a := math.Abs(f); a != 0 && (a < 1e-6 || a >= 1e21) {
		bicim = 'e'
	}
	y.b = strconv.AppendFloat(y.b, f, bicim, -1, 64)
	if bicim == 'e' {
		// 1e-07 → 1e-7
		n := len(y.b)
		if n >= 4 && y.b[n-4] == 'e' && y.b[n-3] == '-' && y.b[n-2] == '0' {
			y.b[n-2] = y.b[n-1]
			y.b = y.b[:n-1]
		}
	}
	return nil
}

func (y *yazici) deger(val interp.Value, derinlik int) error {
	switch v := val.(type) {
	case *interp.NullVal:
		y.b = append(y.b, "null"...)
	case *interp.BoolVal:
		y.b = strconv.AppendBool(y.b, v.Val)
	case *interp.IntVal:
		y.b = strconv.AppendInt(y.b, v.Val, 10)
	case *interp.FloatVal:
		return y.sayi(v.Val)
	case *interp.StringVal:
		y.metin(v.Val)
	case *interp.ListVal:
		if err := y.gezgin.Enter(v, derinlik); err != nil {
			return err
		}
		defer y.gezgin.Leave(v)
		y.b = append(y.b, '[')
		for i, el := range v.Elements {
			if i > 0 {
				y.b = append(y.b, ',')
			}
			y.satir(derinlik + 1)
			if err := y.deger(el, derinlik+1); err != nil {
				return err
			}
		}
		if len(v.Elements) > 0 {
			y.satir(derinlik)
		}
		y.b = append(y.b, ']')
	case *interp.DictVal:
		if err := y.gezgin.Enter(v, derinlik); err != nil {
			return err
		}
		defer y.gezgin.Leave(v)
		y.b = append(y.b, '{')
		var gorulen map[string]bool
		if len(v.KeyVals) > 0 {
			gorulen = make(map[string]bool, len(v.Order))
		}
		ilk := true
		for _, k := range v.Order {
			el, ok := v.Pairs[k]
			if !ok {
				continue
			}
			ad := v.DisplayKey(k)
			if gorulen != nil {
				if gorulen[ad] {
					return fmt.Errorf("sözlükte görünüşü aynı iki anahtar var (%s); JSON'da anahtarlar metindir, ikisi aynı anahtar olurdu", strconv.Quote(ad))
				}
				gorulen[ad] = true
			}
			if !ilk {
				y.b = append(y.b, ',')
			}
			ilk = false
			y.satir(derinlik + 1)
			y.metin(ad)
			y.b = append(y.b, ':')
			if y.girinti != "" {
				y.b = append(y.b, ' ')
			}
			if err := y.deger(el, derinlik+1); err != nil {
				return err
			}
		}
		if !ilk {
			y.satir(derinlik)
		}
		y.b = append(y.b, '}')
	case interp.PlainValuer:
		// Dış değerler (ör. tensör → iç içe sayı listesi)
		return y.deger(v.PlainValue(), derinlik+1)
	default:
		y.metin(val.Inspect())
	}
	return nil
}
