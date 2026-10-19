package checker

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// builtinCache etkin dilin yerleşik adlarını bir kez hesaplar.
var builtinCache struct {
	sync.Mutex
	lang  *codes.Language
	names map[string]bool
}

// builtinNames sözlükteki yerleşiklerin (G/T/V/W) ve tip adlarının (A2xx; ifadede tip
// değeridir: tip(x) == Tam) kabul edilen adlarıdır; denetimde bu adlar her kapsamda
// tanımlıdır (çalışma anında genel kapsama kaydedilirler).
func builtinNames() map[string]bool {
	o := &builtinCache
	o.Lock()
	defer o.Unlock()
	if o.lang != codes.Active() {
		m := map[string]bool{}
		for _, k := range codes.Codes() {
			if (k.IsBuiltin() || k.Namespace() == codes.NamespaceType) && !codes.Removed(k) {
				for _, y := range codes.Spellings(k) {
					m[y] = true
				}
			}
		}
		o.lang, o.names = codes.Active(), m
	}
	return o.names
}
