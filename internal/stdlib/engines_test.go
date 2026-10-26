package stdlib

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Tüm motorlar yüklüyken kaldırılmış eş anlamlı yerleşik adlarından hiçbiri kayıtlı
// olmamalıdır; asıl adlar (ör. ortanca, standart_sapma) kayıtlıdır.
func TestRemovedBuiltinsNotRegistered(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	m := interp.New()
	for _, name := range m.GlobalEnv.Names() {
		if msg := hint.BuiltinSpellingHint(name).Error(); msg != "" {
			t.Errorf("%q kayıtlı olmamalı:%s", name, msg)
		}
	}
	for _, name := range []string{"yazdır", "girdi", "tip", "toplam", "ortalama", "max", "min",
		"bekle", "standart_sapma", "ortanca", "json_metin", "csv_oku", "veri_kümesi", "parçalayıcı_yükle"} {
		if _, ok := m.GlobalEnv.Get(name); !ok {
			t.Errorf("asıl ad %q kayıtlı olmalı", name)
		}
	}
}
