package yurut

import (
	"strings"
	"testing"
)

func TestArgumanHatalariBicimi(t *testing.T) {
	if got := ArgumanTipHatasi("sıfırlar", 1, "Tam", &StringVal{Val: "a"}).Error(); got != `sıfırlar() 1. argüman olarak Tam bekler, Metin ("a") verildi` {
		t.Errorf("tip hatası: %s", got)
	}
	if got := ArgumanSayisiHatasi("aralık", "1 ya da 2", 3, "aralık(bitiş)").Error(); got != "aralık() 1 ya da 2 argüman bekler, 3 verildi (kullanım: aralık(bitiş))" {
		t.Errorf("sayı hatası: %s", got)
	}
	// Büyük değerler özetlenir: hata mesajı binlerce elemanı yazdırmaz
	buyuk := &ListVal{}
	for i := 0; i < 10000; i++ {
		buyuk.Elements = append(buyuk.Elements, NewInt(int64(i)))
	}
	if got := DegerOzeti(buyuk); got != "[0, 1, 2, …], 10000 eleman" {
		t.Errorf("liste özeti: %s", got)
	}
	uzun := DegerOzeti(&StringVal{Val: strings.Repeat("ğ", 100)})
	if !strings.HasSuffix(uzun, `…"`) || len([]rune(uzun)) != ozetSiniri+2 {
		t.Errorf("metin özeti: %s", uzun)
	}
	if got := TipVeOzet(nil); got != "hiçbir değer" {
		t.Errorf("nil: %s", got)
	}
}
