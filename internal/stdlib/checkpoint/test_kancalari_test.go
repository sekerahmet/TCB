package checkpoint

import "time"

// Testlerin dosya sistemini ve saati taklit etmesi için (yalnızca testlerde derlenir).

// YenidenAdlandirmaTaklidi yeniden adlandırmayı f'ye yaptırır (ağ sürücüsü benzetimi); geri
// alan fonksiyonu döndürür.
func YenidenAdlandirmaTaklidi(f func(eski, yeni string) error) func() {
	onceki := yenidenAdlandir
	yenidenAdlandir = f
	return func() { yenidenAdlandir = onceki }
}

// SaatTaklidi şimdiki zamanı f'den okutur; geri alan fonksiyonu döndürür.
func SaatTaklidi(f func() time.Time) func() {
	onceki := saat
	saat = f
	return func() { saat = onceki }
}

// İç yardımcılar (bulanıklaştırma ve biçim testleri).
var (
	OzetCoz  = ozetCoz
	DurumCoz = durumCoz
	DuzOku   = duzOku
	DuzYaz   = duzYaz
)
