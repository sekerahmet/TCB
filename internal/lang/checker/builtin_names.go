package checker

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// yerlesikOnbellegi etkin dilin yerleşik adlarını bir kez hesaplar.
var yerlesikOnbellegi struct {
	sync.Mutex
	dil   *codes.Language
	adlar map[string]bool
}

// yerlesikAdlari sözlükteki yerleşiklerin (G/T/V/W) ve tip adlarının (A2xx; ifadede tip
// değeridir: tip(x) == Tam) kabul edilen adlarıdır; denetimde bu adlar her kapsamda
// tanımlıdır (çalışma anında genel kapsama kaydedilirler).
func yerlesikAdlari() map[string]bool {
	o := &yerlesikOnbellegi
	o.Lock()
	defer o.Unlock()
	if o.dil != codes.Active() {
		m := map[string]bool{}
		for _, k := range codes.Codes() {
			if (k.IsBuiltin() || k.Namespace() == codes.NamespaceType) && !codes.Removed(k) {
				for _, y := range codes.Spellings(k) {
					m[y] = true
				}
			}
		}
		o.dil, o.adlar = codes.Active(), m
	}
	return o.adlar
}
