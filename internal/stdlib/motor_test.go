package stdlib

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Tüm motorlar yüklüyken kaldırılmış eş anlamlı yerleşik adlarından hiçbiri kayıtlı
// olmamalıdır; asıl adlar (ör. ortanca, standart_sapma) kayıtlıdır.
func TestKaldirilanYerlesiklerKayitliDegil(t *testing.T) {
	m := interp.New()
	for _, ad := range m.GlobalEnv.Names() {
		if ipucu := hint.BuiltinSpellingHint(ad); ipucu != "" {
			t.Errorf("%q kayıtlı olmamalı:%s", ad, ipucu)
		}
	}
	for _, ad := range []string{"yazdır", "girdi", "tip", "toplam", "ortalama", "maks", "min",
		"bekle", "standart_sapma", "ortanca", "json_metin", "csv_oku", "veri_kümesi", "parçalayıcı_yükle"} {
		if _, ok := m.GlobalEnv.Get(ad); !ok {
			t.Errorf("asıl ad %q kayıtlı olmalı", ad)
		}
	}
}
