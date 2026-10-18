package denetci

import (
	"sync"

	"tr_dili/internal/dil/kod"
)

// yerlesikOnbellegi etkin dilin yerleşik adlarını bir kez hesaplar.
var yerlesikOnbellegi struct {
	sync.Mutex
	dil   *kod.Dil
	adlar map[string]bool
}

// yerlesikAdlari sözlükteki yerleşiklerin (G/T/V/W) kabul edilen adlarıdır; denetimde bu
// adlar her kapsamda tanımlıdır (çalışma anında genel kapsama kaydedilirler).
func yerlesikAdlari() map[string]bool {
	o := &yerlesikOnbellegi
	o.Lock()
	defer o.Unlock()
	if o.dil != kod.Etkin() {
		m := map[string]bool{}
		for _, k := range kod.Kodlar() {
			if k.Yerlesik() && !kod.Kaldirildi(k) {
				for _, y := range kod.Yazimlar(k) {
					m[y] = true
				}
			}
		}
		o.dil, o.adlar = kod.Etkin(), m
	}
	return o.adlar
}
