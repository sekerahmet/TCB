package yurut

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Yerleşik fonksiyonların argüman hataları tek biçimdedir; motor paketlerindeki
// yerleşikler de bu işlevleri kullanmalıdır:
//
//	aralık() 1. argüman olarak Tam bekler, Metin ("a") verildi
//	indirge() 2 ya da 3 argüman bekler, 1 verildi (kullanım: indirge(liste, fonksiyon, [başlangıç]))

// ArgumanTipHatasi yerleşiğin sira. argümanının beklenen tipte olmadığını bildirir.
// ad yerleşiğin adıdır ("aralık"), beklenen kullanıcıya görünen tip adıdır ("Tam",
// "Tam ya da Ondalık").
func ArgumanTipHatasi(ad string, sira int, beklenen string, verilen Value) error {
	return fmt.Errorf("%s() %d. argüman olarak %s bekler, %s verildi", ad, sira, beklenen, TipVeOzet(verilen))
}

// ArgumanSayisiHatasi yerleşiğin yanlış sayıda argümanla çağrıldığını bildirir.
// beklenen "1", "en az 2", "1 ya da 2" gibi bir sayı ifadesidir; kullanim boş değilse
// doğru çağrı biçimi olarak eklenir (ör. "aralık(bitiş) ya da aralık(başlangıç, bitiş)").
func ArgumanSayisiHatasi(ad, beklenen string, verilen int, kullanim string) error {
	if kullanim != "" {
		return fmt.Errorf("%s() %s argüman bekler, %d verildi (kullanım: %s)", ad, beklenen, verilen, kullanim)
	}
	return fmt.Errorf("%s() %s argüman bekler, %d verildi", ad, beklenen, verilen)
}

// TipVeOzet bir değeri hata mesajı için "Metin ("a")" biçiminde yazar.
func TipVeOzet(v Value) string {
	if v == nil {
		return "hiçbir değer"
	}
	return fmt.Sprintf("%s (%s)", v.Type(), DegerOzeti(v))
}

// ozetSiniri hata mesajlarındaki değer özetinin en fazla karakter sayısıdır.
const ozetSiniri = 40

// DegerOzeti bir değeri hata mesajına sığacak kısalıkta yazar: metinler tırnak içinde,
// uzun değerler "…" ile kısaltılmış. Büyük listelerin tamamı yazdırılmaz.
func DegerOzeti(v Value) string {
	var s string
	switch x := v.(type) {
	case nil:
		return "yok"
	case *StringVal:
		return strconv.Quote(kisalt(x.Val, ozetSiniri))
	case *ListVal:
		if len(x.Elements) > 5 {
			parcalar := make([]string, 0, 4)
			for _, e := range x.Elements[:3] {
				parcalar = append(parcalar, DegerOzeti(e))
			}
			return kisalt("["+strings.Join(parcalar, ", ")+", …]", ozetSiniri) +
				fmt.Sprintf(", %d eleman", len(x.Elements))
		}
		s = x.Inspect()
	case *DictVal:
		if len(x.Pairs) > 5 {
			return fmt.Sprintf("%d anahtarlı sözlük", len(x.Pairs))
		}
		s = x.Inspect()
	default:
		s = v.Inspect()
	}
	return kisalt(s, ozetSiniri)
}

// kisalt metni en fazla n karaktere (rune) indirir; kısaltılınca sonuna "…" ekler.
func kisalt(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
