package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Hata yardımcıları dilin adlarını koddan alır: yerleşik ve tip adları Go metninde yazılmaz.
func TestArgumanHatalariBicimi(t *testing.T) {
	if got := ArgumanTipHatasi(codes.T101, 1, &StringVal{Val: "a"}, codes.A200).Error(); got != `sıfırlar() 1. argüman olarak Tam bekler, Metin ("a") verildi` {
		t.Errorf("tip hatası: %s", got)
	}
	if got := ArgumanTipHatasi(codes.G210, 1, NULL, codes.A200, codes.A201, codes.A202).Error(); got != `tam() 1. argüman olarak Tam, Ondalık ya da Metin bekler, Yok (yok) verildi` {
		t.Errorf("çok tipli tip hatası: %s", got)
	}
	if got := ArgumanSayisiHatasi(codes.G305, "1 ya da 2", 3, "bitiş", "başlangıç, bitiş").Error(); got != "aralık() 1 ya da 2 argüman bekler, 3 verildi (kullanım: aralık(bitiş) ya da aralık(başlangıç, bitiş))" {
		t.Errorf("sayı hatası: %s", got)
	}
	if got := ArgumanSayisiHatasi(codes.Y200, "1", 0).Error(); got != "liste.filtrele() 1 argüman bekler, 0 verildi" {
		t.Errorf("üye sayı hatası: %s", got)
	}
	if got := Kullanim(codes.G101, "", "istem"); got != "girdi() ya da girdi(istem)" {
		t.Errorf("kullanım: %s", got)
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
