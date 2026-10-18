package tensor

import (
	"testing"
)

// Sert hakem denetiminin motor tarafı bulgularının regresyon testleri.

// Bulgu 7: indeksle atama tek kopya ve tek sürüm artışıdır (eleman eleman yazılmaz);
// bitişik olmayan hedefte de.
func TestIndekseAtamaTekKopya(t *testing.T) {
	v := deger(t, `değişken cikti = sıfırlar([300, 200])
cikti[0..300] = birler([300, 200])
cikti.devrik()[5] = 2
cikti[1][2] = 3`)
	d := v.(*Deger)
	if s := d.t.Version(); s != 3 {
		t.Fatalf("3 atama için sürüm %d; her atama bir kez artırmalı", s)
	}
	veri := d.t.Data()
	if veri[0] != 1 || veri[5] != 2 || veri[200+5] != 2 || veri[299*200+5] != 2 || veri[200+2] != 3 || veri[6] != 1 {
		t.Fatalf("atama sonucu yanlış: %v", veri[:8])
	}
}
