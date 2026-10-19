package interp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Yerleşik fonksiyonların argüman hataları tek biçimdedir; motor paketlerindeki
// yerleşikler de bu işlevleri kullanmalıdır. Dilin adları (yerleşik, metot, tip adları)
// mesaja koddan girer; Go metninde yazılmaz:
//
//	aralık() 1. argüman olarak Tam bekler, Metin ("a") verildi
//	aralık() 1 ya da 2 argüman bekler, 0 verildi (kullanım: aralık(bitiş) ya da aralık(başlangıç, bitiş))

// CagriAdi bir kodun hata mesajlarında yazılan adıdır: yerleşiklerde adın kendisi
// (aralık), üyelerde alıcısıyla (liste.filtrele; bkz. kod.UyeAdi).
func CagriAdi(kd codes.Kod) string {
	if kd.Alan() == codes.AlanUye {
		return codes.UyeAdi(kd)
	}
	return codes.Ad(kd)
}

// Cagri bir kodun hata mesajı önekidir: "aralık()", "liste.filtrele()".
func Cagri(kd codes.Kod) string { return CagriAdi(kd) + "()" }

// Kullanim bir kodun doğru çağrı biçimlerini yazar; her biçim parametre listesidir:
// Kullanim(kod.G305, "bitiş", "başlangıç, bitiş") → "aralık(bitiş) ya da aralık(başlangıç, bitiş)".
func Kullanim(kd codes.Kod, bicimler ...string) string {
	return kullanimAdla(CagriAdi(kd), bicimler)
}

func kullanimAdla(ad string, bicimler []string) string {
	parcalar := make([]string, len(bicimler))
	for i, b := range bicimler {
		parcalar[i] = ad + "(" + b + ")"
	}
	return strings.Join(parcalar, " ya da ")
}

// TipAdlari tip kodlarının adlarını bir seçenek listesi olarak yazar:
// TipAdlari(kod.A200, kod.A201, kod.A202) → "Tam, Ondalık ya da Metin".
func TipAdlari(tipler ...codes.Kod) string {
	switch len(tipler) {
	case 0:
		return ""
	case 1:
		return codes.Ad(tipler[0])
	}
	adlar := codes.Adlar(tipler...)
	return strings.Join(adlar[:len(adlar)-1], ", ") + " ya da " + adlar[len(adlar)-1]
}

// ArgumanTipHatasi kd yerleşiğinin (ya da üyesinin) sira. argümanının beklenen tiplerden
// biri olmadığını bildirir; beklenen tip kodlarıdır (A2xx).
func ArgumanTipHatasi(kd codes.Kod, sira int, verilen Value, beklenen ...codes.Kod) error {
	return argumanTipHatasiAdla(CagriAdi(kd), sira, TipAdlari(beklenen...), verilen)
}

func argumanTipHatasiAdla(ad string, sira int, beklenen string, verilen Value) error {
	return fmt.Errorf("%s() %d. argüman olarak %s bekler, %s verildi", ad, sira, beklenen, TipVeOzet(verilen))
}

// ArgumanSayisiHatasi kd yerleşiğinin (ya da üyesinin) yanlış sayıda argümanla çağrıldığını
// bildirir. beklenen "1", "en az 2", "1 ya da 2" gibi bir sayı ifadesidir; kullanim
// verilirse doğru çağrı biçimleri (parametre listeleri, bkz. Kullanim) eklenir.
func ArgumanSayisiHatasi(kd codes.Kod, beklenen string, verilen int, kullanim ...string) error {
	ad := CagriAdi(kd)
	k := ""
	if len(kullanim) > 0 {
		k = kullanimAdla(ad, kullanim)
	}
	return argumanSayisiHatasiAdla(ad, beklenen, verilen, k)
}

func argumanSayisiHatasiAdla(ad, beklenen string, verilen int, kullanim string) error {
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
		return codes.Ad(codes.A042)
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
